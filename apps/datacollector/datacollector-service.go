package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/andrej220/HAM/pkg/config"
	gp "github.com/andrej220/HAM/pkg/graphproc"
	ku "github.com/andrej220/HAM/pkg/kafkautil"
	"github.com/andrej220/HAM/pkg/lg"
	dm "github.com/andrej220/HAM/pkg/shared-models"
	boff "github.com/andrej220/HAM/pkg/backoff"
	"github.com/andrej220/HAM/pkg/workerpool"

	"github.com/andrej220/HAM/pkg/serverutil"
	"github.com/segmentio/kafka-go"
)

const (

	DATASERVICEURL = "http://localhost:8082/dataservice"
)

var ready int32 // 0/1 for readiness

type datacollectorHandler struct {
	pool        *workerpool.Pool[SSHJob]
	cancelFuncs sync.Map
	httpClient  *http.Client
	logger      lg.Logger
}

type consumerDeps struct {
	logger  lg.Logger
	handler *datacollectorHandler
	build   func(*DataCollectorConfig) (*ku.Consumer[dm.Request], error)
}

func setupLogger() lg.Logger {
	cfg := lg.NewConfigFromFlags(SERVICENAME)
	return lg.New(cfg)
}

func loadAndValidateConfig(path string) (*DataCollectorConfig, error) {
	store, err := config.NewStore(config.FileStore, &config.FileConfig{Path: path})
	if err != nil {
		return nil, err
	}
	var cfg DataCollectorConfig
	if err := store.Load(&cfg); err != nil {
		return nil, err
	}
	if len(cfg.Kafka.Brokers) == 0 || cfg.Kafka.Topic == "" || cfg.Kafka.GroupID == "" {
		return nil, errors.New("invalid config: kafka brokers/topic/groupID must be set")
	}
	//DONE: Check if kafka is reachable and repeat config reading in a loop... 
	return &cfg, nil
}

func newHandler(logger lg.Logger) *datacollectorHandler {
	return &datacollectorHandler{
		pool: workerpool.NewPool[SSHJob](workerpool.TotalMaxWorkers),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func buildConsumer(cfg *DataCollectorConfig) (*ku.Consumer[dm.Request], error) {
	return ku.NewConsumer[dm.Request](ku.Config{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Kafka.Topic,
		GroupID: cfg.Kafka.GroupID,
	}), nil
}

func SendToDataservice(gr *gp.Graph, httpClient *http.Client) error {
	b, err := json.Marshal(gr)
	if err != nil {
		return fmt.Errorf("marshal graph: %w", err)
	}
	req, err := http.NewRequest("POST", DATASERVICEURL, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("new req: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post dataservice: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dataservice status %d", resp.StatusCode)
	}
	return nil
}

func EnqueueJob(data dm.Request, h *datacollectorHandler, ctx context.Context) {
	sshJob := SSHJob{
		HostID:   data.HostID,
		ScriptID: data.ScriptID,
		UUID:     data.ExecutionUID,
		Ctx:      ctx,
	}
	jb := workerpool.Job[SSHJob]{
		Payload: sshJob,
		Fn: func(j SSHJob) error {
			graph, err := RunJob(j)
			if err != nil {
				return err
			}
			return SendToDataservice(graph, h.httpClient)
		},
		Ctx: ctx,
		CleanupFunc: func() {
			if cancel, ok := h.cancelFuncs.Load(data.ExecutionUID); ok {
				cancel.(context.CancelFunc)()
				h.cancelFuncs.Delete(data.ExecutionUID)
			}
		},
	}
	h.pool.Submit(jb)
}

func readinessMux() *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if atomic.LoadInt32(&ready) == 1 {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	})
	return m
}

func startProbeServerAsync(port string, logger lg.Logger) {
	go func() {
		_ = serverutil.RunServer(
			readinessMux(),
			serverutil.ServerConfig{
				Port:            port, // e.g., from cfg.Server.Port
				ReadTimeout:     5 * time.Second,
				WriteTimeout:    5 * time.Second,
				IdleTimeout:     30 * time.Second,
				ShutdownTimeout: 3 * time.Second,
				Logger:          logger,
			},
		)
	}()
}
func (d *consumerDeps) runConsumer(ctx context.Context, cfg *DataCollectorConfig) error {
	cons, err := d.build(cfg)
	if err != nil {
		return fmt.Errorf("build consumer: %w", err)
	}
	defer cons.Close()

	const retryBudget = 5 * time.Minute // was MaxElapsed
	bo := boff.New(1*time.Second, 30*time.Second, time.Now().UnixNano())
	retryStart := time.Now()

	for {
		if err := ctx.Err(); err != nil {
			d.logger.Info("consumer: context canceled")
			return nil
		}

		order, raw, err := cons.ReadWithMeta(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}

			// transient?
			msg := strings.ToLower(err.Error())
			if errors.Is(err, kafka.LeaderNotAvailable) ||
				strings.Contains(msg, "not leader for partition") ||
				strings.Contains(msg, "connection refused") ||
				strings.Contains(msg, "dial tcp") {
				if time.Since(retryStart) >= retryBudget {
					return fmt.Errorf("retry budget exceeded after %s: %w", time.Since(retryStart).Round(time.Second), err)
				}
				sleep := bo.Next()
				d.logger.Warn("Consumer transient error; backing off",
					lg.String("err", err.Error()), lg.String("sleep", sleep.String()))
				select {
				case <-time.After(sleep):
				case <-ctx.Done():
					return nil
				}
				continue
			}

			// poison: unmarshal error -> commit and move on
			if strings.Contains(msg, "unmarshal") {
				d.logger.Warn("dropping malformed message (committing)", lg.String("err", msg))
				_ = cons.Commit(ctx, raw)
				// reset retry window because we *did* make progress
				bo.Reset(boff.InitialBackoff)
				retryStart = time.Now()
				continue
			}

			// non-transient
			return fmt.Errorf("consumer unexpected error: %w", err)
		}

		// success: mark ready on first message consumed
		if atomic.LoadInt32(&ready) == 0 {
			atomic.StoreInt32(&ready, 1)
			d.logger.Info("Service is ready (first message consumed)")
		}

		bo.Reset(boff.InitialBackoff)
		retryStart = time.Now()

		EnqueueJob(order, d.handler, ctx)
		d.logger.Debug("Received msg", lg.Any("order", order))

		if err := cons.Commit(ctx, raw); err != nil {
			d.logger.Warn("Commit failed", lg.Any("err", err))
		}
	}
}

func waitKafkaReachable( ctx context.Context, filename string, logger lg.Logger) (*DataCollectorConfig, error){
	const (
		initial    = 30 * time.Second
		maxBackoff = 3  * initial
		maxElapsed = 10 * time.Minute
	)

	start := time.Now()
	bo := boff.New(initial, maxBackoff, time.Now().UnixNano())

	var lastErr error

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if elapsed := time.Since(start); elapsed >= maxElapsed {
			if lastErr == nil {
				lastErr = errors.New("brokers unreachable")
			}
			return nil, fmt.Errorf("kafka unreachable after %s: %w", elapsed.Round(time.Second), lastErr)
		}

		cfg, err := loadAndValidateConfig(filename)
		if err != nil {
			lastErr = fmt.Errorf("read/validate config %q: %w", filename, err)
			logger.Warn("Failed to read config", lg.String("file", filename), lg.Any("Error", err))
		} else if ku.CanReachAnyBroker(cfg.Kafka.Brokers, 2*ku.DialTimeout) {
			return cfg, nil
		} else {
			lastErr = fmt.Errorf("no brokers reachable (brokers=%s)", strings.Join(cfg.Kafka.Brokers, ", "))
			logger.Warn("Kafka brokers not reachable; retrying",lg.String("brokers", strings.Join(cfg.Kafka.Brokers, ", ")))
		}

		sleep := bo.Next()
		logger.Debug("Backoff before next attempt",	lg.String("sleep", sleep.String()))

		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func app() int {

	logger := setupLogger()
	handler := newHandler(logger)
	cfgPath := config.GetConfigPath(PROJECTNAME, SERVICENAME, CONFIGFILENAME)

	runCtx, cancel := context.WithCancel(context.Background())
	cfg, err := waitKafkaReachable( runCtx, cfgPath, logger)
	if err != nil{
		logger.Error("Kafka unreachable / config load/validate faile", lg.Any("err", err))
		return 1
	}	

	logger.Info("starting "+SERVICENAME,
		lg.Int("port", cfg.Server.Port),
		lg.String("brokers", strings.Join(cfg.Kafka.Brokers, ", ")))

	startProbeServerAsync(fmt.Sprintf("%d", cfg.Server.Port), logger)

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	reloadCh := make(chan struct{}, 1)

	stopCh := make(chan struct{})
	go func() {
		for {
			s := <-sigCh
			switch s {
			case syscall.SIGHUP:
				select { case reloadCh <- struct{}{}: default: }
			default:
				close(stopCh)
				return
			}
		}
	}()

	deps := &consumerDeps{logger: logger, handler: handler, build: buildConsumer}

	for {
		runCtx, cancel = context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() { errCh <- deps.runConsumer(runCtx, cfg) }()

		select {
		case err := <-errCh:
			cancel()
			handler.pool.Stop()
			if err != nil {
				logger.Error("consumer stopped with error", lg.Any("err", err))
				return 1
			}
			logger.Info("consumer finished normally")
			return 0

		case <-reloadCh:
			logger.Info("reload requested (SIGHUP)")
			cancel()
			<-errCh

			newCfg, err := loadAndValidateConfig(cfgPath)
			if err != nil {
				logger.Error("reload failed (config)", lg.Any("err", err))
				logger.Info("Keeping old config")
			}else{
				if !ku.CanReachAnyBroker(newCfg.Kafka.Brokers, 2*boff.DialTimeout) {
					logger.Error("Reload failed (brokers unreachable)",
					lg.String("brokers", strings.Join(newCfg.Kafka.Brokers, ", ")))
  				    logger.Info("Keeping old config")
				} else{
					cfg = newCfg
					atomic.StoreInt32(&ready, 0) // will flip after next successful consume
					logger.Info("Reload config succeded.")
				}
			}

		case <-stopCh:
			logger.Info("shutdown requested")
			cancel()
			<-errCh
			handler.pool.Stop()
			return 0
		}
	}
}

func main() { 
	os.Exit(app()) 
}

//{"HostID":"1","ScriptID":"1","ExecutionUID":"1001"}
//{"HostID":1,"ScriptID":1,"ExecutionUID":"1001"}
//
//./kafka-console-producer.sh   --bootstrap-server kafka-0.kafka-headless.kafka.svc.cluster.local:9092   --topic orders
//>{"HostID":1,"ScriptID":1,"ExecutionUID":"1001"}
//>{"HostID":1,"ScriptID":1,"ExecutionUID":"00000000-0000-0000-0000-000000000000"}
//>{"HostID":1,"ScriptID":1,"ExecutionUID":"00000000-0000-0000-0000-000000000000"}
//
//kubectl exec -ti -n kafka kafka-0 -- bash
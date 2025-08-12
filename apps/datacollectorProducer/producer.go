// recives API requests and put in Kafka queue
//root@test-pod:/# curl -X GET https://10.42.0.160:8083/datacollectorProducer -d '{"hostid": 2,"scriptid": 2}'

package main

import(
	"net/http"
	"github.com/andrej220/HAM/pkg/lg"
	"github.com/andrej220/HAM/pkg/serverutil"
	"github.com/andrej220/HAM/pkg/config"
	dm "github.com/andrej220/HAM/pkg/shared-models"
	"os"
	"fmt"
	//"github.com/caarlos0/env/v6"
	"context"
	"time"
	"github.com/segmentio/kafka-go"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"math/rand"
)

const (
	MAXTIMEOUT     time.Duration = 2 * time.Minute
	maxAttempts   = 3
	baseBackoff   = 100 * time.Millisecond
	maxBackoff    = 800 * time.Millisecond
)


type messageWriter interface {
    WriteMessages(context.Context, ...kafka.Message) error
    Close() error
}

type  Producer struct{
	writer  messageWriter
	lg 		lg.Logger
}


type Handler struct{
	producer 	*Producer
	lg 			lg.Logger
}

func newKafkaProducer(lg lg.Logger, cfg DatacollectorProducerConfig) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(cfg.Kafka.Brokers),
			Topic:    cfg.Kafka.Topic,
			Balancer: &kafka.LeastBytes{},
			Async:    false, 
			AllowAutoTopicCreation: true,
		},
		lg: lg,
	}
}

func newProducerHandler(cfg  DatacollectorProducerConfig, lg lg.Logger) http.Handler {
	producer := newKafkaProducer(lg, cfg)
	handler := &Handler{
		producer: producer,
		lg:       lg,
	}
	lg.Info("Created handler with Kafka producer")
	return handler
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request){
	request, ok := r.Context().Value("request").(dm.Request)
	if !ok {
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(lg.Attach(context.Background(), h.lg), MAXTIMEOUT)

	defer cancel()
	// set new UUID to the request
	request.ExecutionUID = uuid.New()
	h.lg.Info("Started new execution, %v", lg.Any("UUID", request.ExecutionUID))
	message, err := json.Marshal(request)
	if err != nil {
		h.lg.Error("Failed to marshal request:", lg.Any("err", err))
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	//h.lg.Info("preparing to send message")
	
	msg := kafka.Message{
		Key:   request.ExecutionUID[:],
		Value: message,
		Time:  time.Now(),
	}

	var lastErr error
	start := time.Now()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := h.producer.writer.WriteMessages(ctx, msg); err != nil {
			lastErr = err
			if !isTransientKafkaErr(err) || attempt == maxAttempts || ctx.Err() != nil {
				break
			}
			// backoff + jitter
			backoff := baseBackoff << (attempt - 1)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			jitter := time.Duration(rand.Intn(75)) * time.Millisecond
			time.Sleep(backoff + jitter)
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		if errors.Is(lastErr, kafka.UnknownTopicOrPartition) {
			h.lg.Error("kafka topic does not exist",
				lg.String("action", "create the topic or enable auto-creation"))
			http.Error(rw, "Failed to process request", http.StatusServiceUnavailable)
			return
		}
		// other broker/timeout errors as transient (503)
		if isTransientKafkaErr(lastErr) || errors.Is(lastErr, context.DeadlineExceeded) || errors.Is(lastErr, context.Canceled) {
			h.lg.Info("transient kafka/write error",
				lg.Any("err", lastErr), lg.Any("latency", time.Since(start)))
			http.Error(rw, "Service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}

		h.lg.Error("permanent write error",
			lg.Any("err", lastErr), lg.Any("latency", time.Since(start)))
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusAccepted)
	_, _ = rw.Write([]byte("Request accepted and queued\n"))
}

func isTransientKafkaErr(err error) bool {
	switch {
	case errors.Is(err, kafka.LeaderNotAvailable),
		errors.Is(err, kafka.NotEnoughReplicas),
		errors.Is(err, kafka.RequestTimedOut),
		errors.Is(err, kafka.NetworkException),
		errors.Is(err, kafka.ReplicaNotAvailable):
		return true
	}
	// deadline/ctx errors are transient from caller perspective
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	return false
}

func initConfig(path string)(*DatacollectorProducerConfig, error){
	store, err := config.NewStore(config.FileStore, &config.FileConfig{Path: path})
    if err != nil {
        return nil, err
    }
    var cfg DatacollectorProducerConfig
    if err := store.Load(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

func main(){
	
	cfg, err := initConfig(config.GetConfigPath(PROJECTNAME, SERVICENAME, CONFIGFILENAME))
	if err != nil {
		fmt.Printf("Init config error: %v",err)
		return
	}

	loggerCfg    := lg.NewConfigFromFlags(cfg.Service.Name)
	logger := lg.New(loggerCfg)

	logger.Info("starting service ", lg.String("str",cfg.Service.Name), lg.String("port", cfg.Service.Port))

	mux := http.NewServeMux()
	handler := newProducerHandler(*cfg, logger)
	mux.Handle(cfg.Service.HTTPpath, serverutil.NewValidationHandler[dm.Request](handler))

	serverConfig := serverutil.DefaultServerConfig()
	serverConfig.Logger = logger
	serverConfig.Port = cfg.Service.Port 
	if err := serverutil.RunServer(mux, serverConfig); err != nil {
		logger.Error("Fatal error. Failed to run server: %v", lg.Any("err",err))
		os.Exit(1)
	}
}

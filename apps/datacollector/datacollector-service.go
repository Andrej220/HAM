package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"sync"
	"time"
	"github.com/andrej220/HAM/pkg/lg"
	//"github.com/andrej220/HAM/pkg/serverutil"
	ku "github.com/andrej220/HAM/pkg/kafkautil"
	//"github.com/segmentio/kafka-go"
	"github.com/andrej220/HAM/pkg/workerpool"

	gp "github.com/andrej220/HAM/pkg/graphproc"
	dm "github.com/andrej220/HAM/pkg/shared-models"
	//"go.mongodb.org/mongo-driver/pkg/logger"
)

const MAXTIMEOUT time.Duration = 1 * time.Minute
const DATASERVICEURL = "http://localhost:8082/dataservice"
const SERVICENAME = "HAM-datacollector"
const SERVICEPORT = "8081" 

type datacollectorHandler struct {
	pool        *workerpool.Pool[SSHJob]
	cancelFuncs  sync.Map
	httpClient  *http.Client
	logger		 lg.Logger
}

func newDatacollectorHandler(lg lg.Logger) *datacollectorHandler {
	h := &datacollectorHandler{
		pool: workerpool.NewPool[SSHJob](workerpool.TotalMaxWorkers),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: lg,
	}
	return h
}

func SendToDataservice(gr *gp.Graph, httpClient *http.Client) error {
	// TODO: log information about the request
	graphBytes, err := json.Marshal(gr)

	if err != nil {
		return fmt.Errorf("failed to marshal graph: %v", err)
	}

	// Create HTTP POST request
	req, err := http.NewRequest("POST", DATASERVICEURL, bytes.NewBuffer(graphBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to dataservice: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dataservice returned status %d", resp.StatusCode)
	}
	return nil
}

func Serve(data dm.Request, h *datacollectorHandler, ctx context.Context ) {

	sshJob := SSHJob{
		HostID:   data.HostID,
		ScriptID: data.ScriptID,
		UUID:     data.ExecutionUID,
		Ctx:      ctx,
	}

	jb := workerpool.Job[SSHJob]{
		Payload: sshJob,
		Fn:     func(j SSHJob) error {
					graph, err := RunJob(j)
					if err != nil{
						return err
					}
					//logger.Info("Request to dataservice")
					SendToDataservice(graph,h.httpClient)
					return nil
				},
		Ctx:     ctx,
		CleanupFunc: func() {
			if cancel, ok := h.cancelFuncs.Load(data.ExecutionUID); ok {
				cancel.(context.CancelFunc)()
				h.cancelFuncs.Delete(data.ExecutionUID)
			}
		},
	}
	h.pool.Submit(jb)
}

func main() {
	cfg := lg.NewConfigFromFlags(SERVICENAME)
	logger := lg.New(cfg)
	handler := newDatacollectorHandler(logger)

	consumerCfg := ku.Config{
		Brokers: []string{
			"hev095wvtq2.sn.mynetname.net:31990",
			"hev095wvtq2.sn.mynetname.net:31991",
			"hev095wvtq2.sn.mynetname.net:31992",
		},
		Topic:   "orders",
		GroupID: "order-service",
	}
	fmt.Println("Kafka brokers:", consumerCfg.Brokers)
	cons := ku.NewConsumer[dm.Request](consumerCfg)
	defer cons.Close()

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up channel for receiving signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the consumer in a goroutine so we can wait for signals
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				logger.Info("Shutting down consumer loop...")
				return
			default:
				order, err := cons.Read(ctx)
				if err != nil {
					logger.Error("error", lg.Any("err", err))
					time.Sleep(time.Second)
					continue
				}
				logger.Debug("Received msg", lg.Any("order", order))
				fmt.Println("Received msg:", order)
				Serve(order, handler, ctx)
			}
		}
	}()

	// Wait for signal or completion
	select {
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down...", lg.String("signal", sig.String()))
		cancel() // Cancel the context to stop the consumer
	case <-done:
		logger.Info("Consumer finished normally")
	}

	// Wait for the consumer to finish
	<-done

	// Stop the handler pool
	handler.pool.Stop()
	logger.Info("Service shutdown completed")
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

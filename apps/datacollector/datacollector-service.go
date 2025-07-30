package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	//"os"
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

//func (h *datacollectorHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
//	request, ok := r.Context().Value("request").(dm.Request)
//	if !ok {
//		http.Error(rw, "Internal server error", http.StatusInternalServerError)
//		return
//	}
//	ctx, cancel := context.WithTimeout(lg.Attach(context.Background(), h.logger), MAXTIMEOUT)
//	newUUID := uuid.New()
//
//	sshJob := SSHJob{
//		HostID:   request.HostID,
//		ScriptID: request.ScriptID,
//		UUID:     newUUID,
//		Ctx:      ctx,
//	}
//
//	jb := workerpool.Job[SSHJob]{
//		Payload: sshJob,
//		Fn:     func(j SSHJob) error {
//					graph, err := RunJob(j)
//					if err != nil{
//						return err
//					}
//					h.logger.Info("Request to dataservice")
//					SendToDataservice(graph,h.httpClient)
//					return nil
//				},
//		Ctx:     ctx,
//		CleanupFunc: func() {
//			if cancel, ok := h.cancelFuncs.Load(newUUID); ok {
//				cancel.(context.CancelFunc)()
//				h.cancelFuncs.Delete(newUUID)
//			}
//		},
//	}

//	h.cancelFuncs.Store(newUUID, cancel)
//	h.pool.Submit(jb)
//	response := dm.Response{ExecutionUID: newUUID}
//
//	// TODO: think about the response
//	rw.Header().Set("Content-Type", "application/json")
//	rw.WriteHeader(http.StatusOK)
//	encoder := json.NewEncoder(rw)
//	if err := encoder.Encode(response); err != nil {
//		h.logger.Error("Failed to encode response: %v", lg.Any("err",err))
//	}
//}

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
    cfg    := lg.NewConfigFromFlags(SERVICENAME)
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

	// TODO: add cancel function to shutdown the service gracefully
	// TODO: catch ctrl+c signal 
	ctx := context.Background()
	for {
		order, err := cons.Read(ctx)
		if (err != nil){
			logger.Error("error", lg.Any("err",err))
			time.Sleep(time.Second)
			continue
		}
		logger.Debug("Recieved msg",lg.Any("order",order))
		fmt.Println("Recieved msg:", order)
		Serve(order, handler, ctx)
	}
	//handler := newDatacollectorHandler(logger)
	//exh := handler.(*datacollectorHandler)
	//exh.pool.Stop()
}
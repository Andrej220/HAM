package main

import (
	"context"
	"fmt"
	"encoding/json"
	"github.com/andrej220/HAM/internal/serverutil"
	"github.com/andrej220/HAM/internal/workerpool"
 gp "github.com/andrej220/HAM/internal/graphproc"
	"github.com/google/uuid"
	"log"
	"net/http"
	"sync"
	"time"
	"bytes"
)

const MAXTIMEOUT time.Duration = 1 * time.Minute
const DATACOLLECTORPORT = "8081" 
const DATASERVICEURL = "http://localhost:8082/dataservice"

type datacollectorRequest struct {
	HostID   int `json:"hostid"`
	ScriptID int `json:"scriptid"`
}

type datacollectorResponse struct {
	ExecutionUID uuid.UUID `json:"exuid"`
}

type datacollectorHandler struct {
	pool        *workerpool.Pool[SSHJob]
	cancelFuncs  sync.Map
	httpClient  *http.Client
}

func newDatacollectorHandler() http.Handler {
	h := &datacollectorHandler{
		pool: workerpool.NewPool[SSHJob](workerpool.TotalMaxWorkers),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	return h
}

func SendToDataservice(gr *gp.Graph, httpClient *http.Client) error {
	// TODO: log information about the request
	graphBytes, err := json.Marshal(gr)
	log.Println("Request to dataservice")
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

func (h *datacollectorHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	request, ok := r.Context().Value("request").(datacollectorRequest)
	if !ok {
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), MAXTIMEOUT)
	newUUID := uuid.New()

	sshJob := SSHJob{
		HostID:   request.HostID,
		ScriptID: request.ScriptID,
		UUID:     newUUID,
		Ctx:      ctx,
	}

	jb := workerpool.Job[SSHJob]{
		Payload: sshJob,
		Fn:     func(j SSHJob) error {
					graph, err := RunJob(j)
					if err != nil{
						return err
					}
					SendToDataservice(graph,h.httpClient)
					return nil
				},
		Ctx:     ctx,
		CleanupFunc: func() {
			if cancel, ok := h.cancelFuncs.Load(newUUID); ok {
				cancel.(context.CancelFunc)()
				h.cancelFuncs.Delete(newUUID)
			}
		},
	}

	h.cancelFuncs.Store(newUUID, cancel)
	h.pool.Submit(jb)
	response := datacollectorResponse{ExecutionUID: newUUID}

	// TODO: think about the response
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	if err := encoder.Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func main() {
	mux := http.NewServeMux()
	handler := newDatacollectorHandler()
	mux.Handle("/executor", serverutil.NewValidationHandler[datacollectorRequest](handler))

	config := serverutil.DefaultServerConfig()
	config.Port = DATACOLLECTORPORT 
	if err := serverutil.RunServer(mux, config); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

	exh := handler.(*datacollectorHandler)
	exh.pool.Stop()
}
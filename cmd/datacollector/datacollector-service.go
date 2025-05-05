package main

import (
	"context"
	"encoding/json"
	//"fmt"
	"github.com/andrej220/HAM/internal/serverutil"
	"github.com/andrej220/HAM/internal/workerpool"
	"github.com/google/uuid"
	"log"
	"net/http"
	"sync"
	"time"
)

const MAXTIMEOUT time.Duration = 1 * time.Minute
const DATACOLLECTORPORT = "8081" 

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
		Fn:      sshrunner.RunJob,
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
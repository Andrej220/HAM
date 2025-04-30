package main

import (
	"context"
	"encoding/json"
	//"fmt"
	"github.com/andrej220/HAM/internal/serverutil"
	"github.com/andrej220/HAM/internal/sshrunner"
	"github.com/andrej220/HAM/internal/workerpool"
	"github.com/google/uuid"
	"log"
	"net/http"
	"sync"
	"time"
)

const MAXTIMEOUT time.Duration = 1 * time.Minute

type datacollectorRequest struct {
	HostID   int `json:"hostid"`
	ScriptID int `json:"scriptid"`
}

type datacollectorResponse struct {
	ExecutionUID uuid.UUID `json:"exuid"`
}

type datacollectorHandler struct {
	pool        *workerpool.Pool[sshrunner.SSHJob]
	cancelFuncs sync.Map
}

func newDatacollectorHandler() http.Handler {
	h := &datacollectorHandler{
		pool: workerpool.NewPool[sshrunner.SSHJob](workerpool.TotalMaxWorkers),
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

	sshJob := sshrunner.SSHJob{
		HostID:   request.HostID,
		ScriptID: request.ScriptID,
		UUID:     newUUID,
		Ctx:      ctx,
	}

	jb := workerpool.Job[sshrunner.SSHJob]{
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

	// Configure server using serverutil
	config := serverutil.DefaultServerConfig()
	if err := serverutil.RunServer(mux, config); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

	// Stop the worker pool on shutdown
	exh := handler.(*datacollectorHandler)
	exh.pool.Stop()
}
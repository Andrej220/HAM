package main

import (
	//"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"context"
	"errors"
	"time"
	"os/signal"
	"syscall"
	"os"
	"github.com/google/uuid"
	"executor/pkg/workerpool"
	"sync"
)

const MAXTIMEOUT time.Duration = 5*time.Minute

type executorResponse struct{
	ExecutionUID uuid.UUID `json:"exuid"`
}

type executorRequest struct{
	HostID 		int `json:"hostid"`
	ScriptID 	int  `json:"scriptid"`
}

type validationHandler struct{
	next http.Handler
}

func newValidationHandler(next http.Handler) http.Handler {
	return &validationHandler{next: next}
}

func (h validationHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request){
	var request executorRequest
	decoder := json.NewDecoder(r.Body)
	
	err := decoder.Decode(&request)
	defer r.Body.Close()

	if err != nil{
		http.Error(rw, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if request.HostID < 0 || request.ScriptID < 0 {
		http.Error(rw, "Invalid hostID or scriptID", http.StatusBadRequest)
		return
	}
	
	ctx := context.WithValue(r.Context(), "request", request)
	h.next.ServeHTTP(rw, r.WithContext(ctx))
}

type executorHandler struct{
	pool *workerpool.Pool[SSHJobStruct]
	dspool *workerpool.Pool[DSjobStruct]
	cancelFuncs sync.Map
}

func newExecutorHandler() http.Handler {
	h := executorHandler{}
	h.pool = workerpool.NewPool[SSHJobStruct](workerpool.MAXWORKERS)
	h.dspool = workerpool.NewPool[DSjobStruct](workerpool.MAXWORKERS)
	return &h
}

func (h *executorHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request){
	
	request, ok := r.Context().Value("request").(executorRequest)
	if !ok {
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	ctx, cancel := context.WithTimeout(r.Context(), MAXTIMEOUT )
	//TODO: get results and store them in DB
	//Connect ot a remote host and fetch data
	newUUID := uuid.New()

	dataChan := make(chan string, 100)  // pipe to save data in a file

	sshJob := SSHJobStruct{
		HostID: request.HostID, 
		ScriptID: request.ScriptID, 
		UUID: newUUID,
		dataChan: dataChan,
	}

	dsJob := DSjobStruct{
		HostID: request.HostID, 
		ScriptID: request.ScriptID, 
		UUID: newUUID,
		dataChan: dataChan,
	}

	jb := workerpool.Job[SSHJobStruct]{ 
		Payload: sshJob,
		Fn: GetRemoteConfig,
		Ctx: ctx,
		CleanupFunc: func() {
			if cancel, ok := h.cancelFuncs.Load(newUUID); ok {
				cancel.(context.CancelFunc)()
				h.cancelFuncs.Delete(newUUID)
			}
		},
	}

	dsWPJob := workerpool.Job[DSjobStruct]{
		Payload: dsJob,
		Fn: WriteFile,
		Ctx: ctx,
	}

	h.cancelFuncs.Store(newUUID,cancel)

	h.pool.Submit(jb)
	h.dspool.Submit(dsWPJob)
	
	response := executorResponse{ ExecutionUID: newUUID}
	//
	//TODO: in goroutine save data to a file or DB

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)
	if err:=encoder.Encode(response); err != nil{
		log.Printf("Failed to encode response: %v", err)
	}
}

func main(){
	port := os.Getenv("EXECUTORPORT")
	if port == "" {
		port = "8081"
	}
	
	mux := http.NewServeMux()
	handler:=newExecutorHandler()
	mux.Handle("/executor", newValidationHandler(handler))
	// Configure server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %s\n", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	
	<-done
	log.Print("Server stopping...")

	exh := handler.(*executorHandler)  //assert type
	exh.pool.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	
	log.Print("Server stopped gracefully")
}
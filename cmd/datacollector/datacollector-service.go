package main

import (
	"github.com/andrej220/HAM/internal/workerpool"
	//"executor/pkg/dataservice"
	sshr "github.com/andrej220/HAM/internal/sshrunner"
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
	"sync"
)

const MAXTIMEOUT time.Duration = 1*time.Minute

type datacollectorResponse struct{
	ExecutionUID uuid.UUID `json:"exuid"`
}

type datacollectorRequest struct{
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
	var request datacollectorRequest
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

type datacollectoHandler struct{
	pool *workerpool.Pool[sshr.SSHJob]
	cancelFuncs sync.Map
}

func newDatacollectorHandler() http.Handler {
	h := datacollectoHandler{}
	h.pool = workerpool.NewPool[sshr.SSHJob](workerpool.TotalMaxWorkers)
	return &h
}

func (h *datacollectoHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request){
	
	request, ok := r.Context().Value("request").(datacollectorRequest)
	if !ok {
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), MAXTIMEOUT )
	newUUID := uuid.New()

	sshJob := sshr.SSHJob{
		HostID: request.HostID, 
		ScriptID: request.ScriptID, 
		UUID: newUUID,
		Ctx: ctx,
	}

	jb := workerpool.Job[sshr.SSHJob]{ 
		Payload: sshJob,
		Fn: sshr.RunJob,
		Ctx: ctx,
		CleanupFunc: func() {
			if cancel, ok := h.cancelFuncs.Load(newUUID); ok {
				cancel.(context.CancelFunc)()
				h.cancelFuncs.Delete(newUUID)
			}
		},
	}

	h.cancelFuncs.Store(newUUID,cancel)
	h.pool.Submit(jb)
	response := datacollectorResponse{ ExecutionUID: newUUID}

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
	handler:=newDatacollectorHandler()
	mux.Handle("/executor", newValidationHandler(handler))
	// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!1
	// Configure server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,  // define constants or env vars
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

	exh := handler.(*datacollectoHandler)  //assert type
	exh.pool.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	
	log.Print("Server stopped gracefully")
}
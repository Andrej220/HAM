package serverutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/andrej220/HAM/internal/lg"
)

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	ShutdownTimeout time.Duration
	Logger 	lg.Logger
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:            "8081",
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 30 * time.Second,
		Logger: nil,
	}
}

func RunServer(handler http.Handler, config ServerConfig) error {
	// TODO: PASS LISTENING PORT
	// TODO: pass listening port with environment variable, for different services...
	logger := config.Logger

	if config.Port == "" {
		config.Port = os.Getenv("EXECUTORPORT")
		if config.Port == "" {
			config.Port = DefaultServerConfig().Port
		}
	}
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", config.Port),
		Handler:      handler,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		IdleTimeout:  config.IdleTimeout,
	}
	// Channel to listen for interrupt signals
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		logger.Info("Server starting", lg.String("Port",config.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", lg.Any("error",err))
		}
	}()

	// Wait for interrupt signal
	<-done
	logger.Info("Server stopping...")

	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	// Attempt to gracefully shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", lg.Any("error",err))
		return err
	}

	logger.Info("Server stopped gracefully")
	return nil
}

type ValidationHandler[T any] struct {
	next http.Handler
	validator func(*T) error
}

func NewValidationHandler[T any](next http.Handler, validator ...func(*T) error) http.Handler {
	// TODO: implement a default validator
	var validateFunc func(*T) error
	if len(validator) > 0 {
		validateFunc = validator[0]
	} else {
		validateFunc = defaultValidator[T]
	}

	return &ValidationHandler[T]{
		next:      next,
		validator: validateFunc,
	}
}

func (h *ValidationHandler[T]) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	var request T
	decoder := json.NewDecoder(r.Body)
	
	err := decoder.Decode(&request)
	defer r.Body.Close()

	if err != nil {
		http.Error(rw, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}
	
	if err := h.validator(&request); err != nil {
		respondWithValidationError(rw, err)
		return
	}
	// Pass the decoded request to the next handler via context
	ctx := context.WithValue(r.Context(), "request", request)
	h.next.ServeHTTP(rw, r.WithContext(ctx))
}

// respondWithValidationError sends standardized validation error responses
func respondWithValidationError(rw http.ResponseWriter, err error) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusBadRequest)
	
	// TODO: customize error response
	json.NewEncoder(rw).Encode(map[string]interface{}{
		"error":   "Validation failed",
		"details": err.Error(),
	})
}


func defaultValidator[T any](req *T) error {
	return nil
}
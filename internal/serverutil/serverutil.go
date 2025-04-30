package serverutil

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	ShutdownTimeout time.Duration
}

// DefaultServerConfig provides default server configuration values.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:            "8081",
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}
}

// RunServer starts an HTTP server with the provided handler and configuration.
// It handles graceful shutdown on interrupt signals (SIGINT, SIGTERM).
func RunServer(handler http.Handler, config ServerConfig) error {
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
		log.Printf("Server starting on port %s\n", config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	<-done
	log.Print("Server stopping...")

	// Create a context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	// Attempt to gracefully shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Print("Server stopped gracefully")
	return nil
}

// ValidationHandler is a middleware that validates incoming JSON requests.
type ValidationHandler[T any] struct {
	next http.Handler
}

// NewValidationHandler creates a new validation handler for the given request type.
func NewValidationHandler[T any](next http.Handler) http.Handler {
	return &ValidationHandler[T]{next: next}
}

// ServeHTTP decodes and validates the JSON request, passing it to the next handler via context.
func (h *ValidationHandler[T]) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	var request T
	decoder := json.NewDecoder(r.Body)
	
	err := decoder.Decode(&request)
	defer r.Body.Close()

	if err != nil {
		http.Error(rw, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request (implement validation logic in a separate method if needed)
	if err := validateRequest(request); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	// Pass the decoded request to the next handler via context
	ctx := context.WithValue(r.Context(), "request", request)
	h.next.ServeHTTP(rw, r.WithContext(ctx))
}

// validateRequest is a placeholder for request-specific validation logic.
// Replace with actual validation logic for type T or pass a validator function.
func validateRequest[T any](req T) error {
	// Example: Add specific validation logic here or make it configurable
	return nil
}
package producer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	dm "github.com/andrej220/HAM/pkg/shared-models"
	lg "github.com/andrej220/HAM/pkg/lg"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

type fakeWriter struct {
	messages []kafka.Message
	err      error
}

func (f *fakeWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if f.err != nil {
		return f.err
	}
	f.messages = append(f.messages, msgs...)
	return nil
}

func (f *fakeWriter) Close() error {
	return nil
}

func getLogger() lg.Logger{
	cfg := lg.Config{ServiceName: serviceName, Debug: true, Format: "json"}
    return lg.New(&cfg)
}

func TestHandler_ServeHTTP(t *testing.T){
	stubLog:= getLogger()
	stubLog.Info("test")	
}

func TestHandler_ServeHTTP_Success(t *testing.T) {
	fWriter := &fakeWriter{}
    stubLog := getLogger()
	producer := &Producer{writer: fWriter, lg: stubLog}
	handler := &Handler{producer: producer, lg: stubLog}

	reqBody := dm.Request{}
	req := httptest.NewRequest(http.MethodPost, HTTPpath, nil)
	ctx := context.WithValue(req.Context(), "request", reqBody)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusAccepted {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusAccepted)
	}
	if got := rr.Body.String(); got != "Request accepted and queued\n" {
		t.Errorf("handler returned unexpected body: got %v", got)
	}

	if len(fWriter.messages) != 1 {
		t.Fatalf("expected 1 message written, got %d", len(fWriter.messages))
	}
	msg := fWriter.messages[0]

	var gotReq dm.Request
	if err := json.Unmarshal(msg.Value, &gotReq); err != nil {
		t.Fatalf("failed to unmarshal message value: %v", err)
	}
	if gotReq.ExecutionUID == uuid.Nil {
		t.Error("expected non-nil ExecutionUID in message")
	}

	keyUUID, err := uuid.FromBytes(msg.Key)
	if err != nil {
		t.Fatalf("failed to parse key as UUID: %v", err)
	}
	if keyUUID != gotReq.ExecutionUID {
		t.Errorf("message key UUID %v does not match ExecutionUID %v", keyUUID, gotReq.ExecutionUID)
	}
}

func TestHandler_ServeHTTP_WriteError(t *testing.T) {
	errWrite := errors.New("write failure")
	fWriter := &fakeWriter{err: errWrite}

    stubLog := getLogger()

	producer := &Producer{writer: fWriter, lg: stubLog}
	handler := &Handler{producer: producer, lg: stubLog}

	req := httptest.NewRequest(http.MethodPost, HTTPpath, nil)
	ctx := context.WithValue(req.Context(), "request", dm.Request{})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code on write error: got %v want %v", status, http.StatusInternalServerError)
	}
	if got := rr.Body.String(); got != "Failed to process request\n" {
		t.Errorf("handler returned unexpected body on write error: got %v", got)
	}
}

func TestNewKafkaProducer(t *testing.T) {
    stubLog := getLogger()
	prod := newKafkaProducer(stubLog)
  
	assert.NotNil(t, prod)
	assert.NotNil(t, prod.writer)
  
	kw, ok := prod.writer.(*kafka.Writer)
	if !assert.True(t, ok, "should be *kafka.Writer") {
	  return
	}
  
	assert.Equal(t, kafkaTopic, kw.Topic)
	assert.Equal(t, kafkaBrokers, kw.Addr.String())
	assert.IsType(t, &kafka.LeastBytes{}, kw.Balancer)
	assert.False(t, kw.Async)
	assert.True(t, kw.AllowAutoTopicCreation)
  }

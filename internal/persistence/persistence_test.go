// test module for package persistance

package persistence_test

import (
	"fmt"
	"path/filepath"
	"log"
	"testing"
	"github.com/andrej220/HAM/internal/persistence"
	"github.com/stretchr/testify/assert"
)

const (
    indent = "    "  // Default indentation for JSON output (4 spaces)
    prefix = ""     // Default prefix for JSON output
	sampleJSON = "{\n    \"key\": \"value\"\n}"
)
type MockSerializer struct {
	Bytes []byte
	Err   error
}

func (s MockSerializer) Marshal(data any) ([]byte, error) {
	return s.Bytes, s.Err
}

type MockWriter struct {
	Data map[string][]byte
	Err  error
}

func (w *MockWriter) Write(filename string, data []byte) error {
	if w.Data == nil {
		w.Data = make(map[string][]byte)
	}
	w.Data[filename] = data
	return w.Err
}

func TestWriteJSONToFile(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		data        any
		serializer  persistence.Serializer
		writer      persistence.Writer
		expectedErr bool
	}{
		{
			name:       "valid input",
			filename:   filepath.Join(t.TempDir(), "output.json"),
			data:       map[string]string{"key": "value"},
			serializer: MockSerializer{Bytes: []byte(sampleJSON)},
			writer:     &MockWriter{},
			expectedErr: false,
		},
		{
			name:        "empty filename",
			filename:    "",
			data:        map[string]string{"key": "value"},
			serializer:  MockSerializer{Bytes: []byte(sampleJSON)},
			writer:      &MockWriter{},
			expectedErr: true,
		},
		{
			name:        "serializer error",
			filename:    "test.json",
			data:        map[string]string{"key": "value"},
			serializer:  MockSerializer{Err: fmt.Errorf("serialization failed")},
			writer:      &MockWriter{},
			expectedErr: true,
		},
		{
			name:       "writer error",
			filename:   "test.json",
			data:       map[string]string{"key": "value"},
			serializer: MockSerializer{Bytes: []byte(sampleJSON)},
			writer:     &MockWriter{Err: fmt.Errorf("write failed")},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := persistence.WriteJSONToFile(tt.data, tt.filename, tt.serializer, tt.writer)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if writer, ok := tt.writer.(*MockWriter); ok {
					assert.Equal(t, sampleJSON, string(writer.Data[tt.filename]))
				}
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	data := map[string]string{"key": "value"}

	dir := t.TempDir()
	filename := filepath.Join(dir, "output.json")

	writer := &MockWriter{}

	err := persistence.WriteJSONToFile(data, filename, persistence.JSONSerializer{Prefix: prefix, Indent: indent}, writer)
	assert.NoError(t, err)
	assert.Equal(t, sampleJSON, string(writer.Data[filename]))
}

func ExampleWriteJSONToFile() {
	data := map[string]string{"key": "value"}
	serializer := persistence.JSONSerializer{Prefix: prefix, Indent: indent}
	writer := persistence.FileWriter{Overwrite: true}
	
	filename := "output.json"
	err := persistence.WriteJSONToFile(data, filename, serializer, writer)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Println("Data written successfully")
}

func ExampleWriteJSON() {
	data := map[string]string{"key": "value"}

	filename := "output.json"
	
	err := persistence.WriteJSON(data, filename)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Println("Data written successfully")
}
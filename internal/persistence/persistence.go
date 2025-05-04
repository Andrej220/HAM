// Package persistence provides functionality for persisting data to various destinations,
// with support for different serialization formats.

package persistence 

import (
	"encoding/json"
	"os"
	"path/filepath"
    "fmt"
)

const (
    indent = "    "  // Default indentation for JSON output (4 spaces)
    prefix = ""     // Default prefix for JSON output
)

type Serializer interface {
    Marshal(data any) ([]byte, error)
}

type Writer interface {
    Write(filename string, data []byte) error
}

type JSONSerializer struct {
	Prefix, Indent string
}

func (s JSONSerializer) Marshal(data any) ([]byte, error) {
	return json.MarshalIndent(data, s.Prefix, s.Indent)
}

type FileWriter struct {
	Overwrite bool
}

func (w FileWriter) Write(filename string, data []byte) error {
	if filename == "" {
		return os.ErrInvalid
	}
	if _, err := os.Stat(filename); !os.IsNotExist(err) && !w.Overwrite {
		return os.ErrExist
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// WriteJSONToFile persists data as JSON to a destination using the provided Serializer and Writer.
func WriteJSONToFile(data any, filename string, serializer Serializer, writer Writer) error{
	
    if filename == "" {
		return fmt.Errorf("invalid filename: %w", os.ErrInvalid)
	}

    bytes, err := serializer.Marshal(data)
    if err != nil {
        return fmt.Errorf("failed to marshal data: %w", err)
    }

    if err := writer.Write(filename, bytes); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
    return nil
}

// WriteJSON persists data as JSON to a file with default settings (overwrite enabled, 4-space indent).
func WriteJSON(data any, filename string) error {
	serializer := JSONSerializer{Prefix: prefix, Indent: indent}
	writer := FileWriter{Overwrite: true}
	return WriteJSONToFile(data, filename, serializer, writer)
}

// Usage example
//
//    package persistence_test
//    
//    import (
//    	"fmt"
//    	"log"
//    	".../persistence"
//    )
//    
//    func ExampleWriteJSONToFile() {
//    	data := map[string]string{"key": "value"}
//    	serializer := persistence.JSONSerializer{Prefix: persistence.Prefix, Indent: persistence.Indent}
//    	writer := persistence.FileWriter{Overwrite: true}
//    
//    	err := persistence.WriteJSONToFile(data, "output.json", serializer, writer)
//    	if err != nil {
//    		log.Fatalf("Error: %v", err)
//    	}
//    	fmt.Println("Data written successfully")
//    	// Output: Data written successfully
//    }
//    
//    func ExampleWriteJSON() {
//    	data := map[string]string{"key": "value"}
//    	err := persistence.WriteJSON(data, "output.json")
//    	if err != nil {
//    		log.Fatalf("Error: %v", err)
//    	}
//    	fmt.Println("Data written successfully")
//    	// Output: Data written successfully
//    }
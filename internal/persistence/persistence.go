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

//   Example usage:
//  data := map[string]string{"key": "value"}
//  opts := persistence.Options{
//      Overwrite: true,
//      Prefix:    "",
//      Indent:    "    ",
//  }
//  serializer := persistence.JSONSerializer{Prefix: opts.Prefix, Indent: opts.Indent}
//  writer := persistence.FileWriter{Overwrite: opts.Overwrite}
//  
//  err := persistence.WriteJSONToFile(data, "output.json", serializer, writer, opts)
//  if err != nil {
//      log.Fatalf("Error: %v", err)
//  }
//  fmt.Println("Data written successfully")

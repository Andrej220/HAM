package processor

import (
    "reflect"
    "testing"
)

func TestTrimProcessor(t *testing.T) {
    p := &TrimProcessor{}
    input := []string{"  hello    ", " world "}
    expected := []string{"hello", "world"}
    result, err := p.Process(input, NodeTypeString)
    if err != nil {
        t.Fatalf("TrimProcessor failed: %v", err)
    }
    if !reflect.DeepEqual(result, expected) {
        t.Errorf("TrimProcessor: got %v, want %v", result, expected)
    }
}

func TestKeyValueProcessor(t *testing.T) {
    p := &KeyValueProcessor{}
    input := []string{" key1: value1 ", " key2: value2 "}
    expected := []string{"key1: value1", "key2: value2"}
    result, err := p.Process(input, NodeTypeString)
    if err != nil {
        t.Fatalf("KeyValueProcessor failed: %v", err)
    }
    if !reflect.DeepEqual(result, expected) {
        t.Errorf("KeyValueProcessor: got %v, want %v", result, expected)
    }
}

func TestProcessorChain(t *testing.T) {
    pc := NewProcessorChain()
    input := []string{"key1: value1 ", "key2: value2 "}
    expected := []string{"key1: value1", "key2: value2"}
    result, err := pc.Process(input, NodeTypeString, ProcessorTypeTrim, ProcessorTypeKeyValue)
    if err != nil {
        t.Fatalf("ProcessorChain failed: %v", err)
    }
    if !reflect.DeepEqual(result, expected) {
        t.Errorf("ProcessorChain: got %v, want %v", result, expected)
    }
}

func TestProcessorChainEdgeCases(t *testing.T) {
    pc := NewProcessorChain()
    tests := []struct {
        name     string
        input    []string
        expected []string
    }{
        {
            name:     "single string with newlines",
            input:    []string{"  key1: value1\n    key2: value2  "},
            expected: []string{"key1: value1", "key2: value2"},
        },
        {
            name:     "empty input",
            input:    []string{},
            expected: []string{},
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := pc.Process(tt.input, NodeTypeString, ProcessorTypeTrim, ProcessorTypeKeyValue)
            if err != nil {
                t.Fatalf("ProcessorChain failed: %v", err)
            }
            if !reflect.DeepEqual(result, tt.expected) {
                t.Errorf("ProcessorChain: got %v, want %v", result, tt.expected)
            }
        })
    }
}

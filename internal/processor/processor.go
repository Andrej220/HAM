// Package processor provides a modular framework for processing string slices
// with configurable processor chains.
package processor

import (
	"encoding/json"
	"fmt"
	"strings"
)

type NodeType string

const (
	NodeTypeObject NodeType = "object"
	NodeTypeString NodeType = "string"
	NodeTypeArray  NodeType = "array"
)

const (
	ProcessorTypeTrim 			string = "trim"
	ProcessorTypeKeyValue 		string = "key_value"
	ProcessorJSONTypeKeyValue	string = "key_value_json"
	ProcessorTypeSplitLines		string = "split_lines"
)

// Processor defines the interface for processing string slices.
type Processor interface {
	// Process applies the processor's logic to the input lines.
	Process([]string, NodeType) ([]string, error)
	Name() string
}

// ProcessorChain manages a collection of processors and applies them in sequence.
type ProcessorChain struct {
	processors 			map[string]Processor
	allowEmptyResults 	bool
}
// Process applies the specified processors to the input lines in order.
func NewProcessorChain() *ProcessorChain {
	pc := &ProcessorChain{
		processors: make(map[string]Processor),
	}
	pc.registerDefaults()
	pc.allowEmptyResults = true
	return pc
}

func (pc *ProcessorChain) registerDefaults() {
	pc.Register(&TrimProcessor{})
	pc.Register(&SplitLinesProcessor{})
	pc.Register(&KeyValueProcessor{})
}

// Register adds a processor to the chain.
func (pc *ProcessorChain) Register(p Processor) {
	pc.processors[p.Name()] = p
}

func isValidNodeType(nt NodeType) bool {
    return nt == NodeTypeObject || nt == NodeTypeString || nt == NodeTypeArray
}

func (pc *ProcessorChain) Process(lines []string,nodeType NodeType,processorNames ...string,) ([]string, error) {
	if !isValidNodeType(nodeType) {
        return nil, fmt.Errorf("invalid nodeType: %v", nodeType)
    }
	for _, name := range processorNames {
        if _, exists := pc.processors[name]; !exists {
            return nil, fmt.Errorf("processor %q not registered", name)
        }
    }
    if len(lines) == 0 {
        return lines, nil
    }
	result := lines
    // Apply processors in specified order
    for _, name := range processorNames {
		var err error
        processor, exists := pc.processors[name]
        if !exists {
            return nil, fmt.Errorf("processor %q not registered", name)
        }
        result, err = processor.Process(result, nodeType)
        if err != nil {
            return nil, fmt.Errorf("%s processor failed: %w", name, err)
        }
        if len(result) == 0 && !pc.allowEmptyResults{
            break
        }
    }
    return result, nil
}

//Processor Implementations

// TrimProcessor trims whitespace from each line in the input.
type TrimProcessor struct{}

func (p *TrimProcessor) Name() string { return ProcessorTypeTrim }
func (p *TrimProcessor) Process(lines []string, _ NodeType) ([]string, error) {
	trimmed := make([]string, len(lines))
	for i, line := range lines {
		trimmed[i] = strings.TrimSpace(line)
	}
	return trimmed, nil
}

// KeyValueProcessor handles string nodes with key:value format
func parseKeyValueLines(lines []string) (map[string]string, error) {
    kv := make(map[string]string)
    
    // Handle case where input is a single string with embedded newlines
    if len(lines) == 1 {
        inputLines := strings.Split(strings.TrimSpace(lines[0]), "\n")
        if len(inputLines) > 1 {
            lines = inputLines
        }
    }

    for _, line := range lines {
        parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
        if len(parts) != 2 {
            continue
        }
        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])
        if key == "" {
            return nil, fmt.Errorf("empty key in line: %q", line)
        }
        kv[key] = value
    }
    return kv, nil
}

type KeyValueProcessor struct{}

func (p *KeyValueProcessor) Name() string { return ProcessorTypeKeyValue }

func (p *KeyValueProcessor) Process(lines []string, nodeType NodeType) ([]string, error) {
    if nodeType != NodeTypeString || len(lines) == 0 {
        return lines, nil
    }
    
    kv, err := parseKeyValueLines(lines)
    if err != nil {
        return nil, err
    }

    result := make([]string, 0, len(kv))
    for k, v := range kv {
        result = append(result, fmt.Sprintf("%s: %s", k, v))
    }
    return result, nil
}

// KeyValueProcessor handles string nodes with key:value format
type KeyValueJSONProcessor struct{}

func (p *KeyValueJSONProcessor) Name() string { return ProcessorTypeKeyValue }

func (p *KeyValueJSONProcessor) Process(lines []string, nodeType NodeType) ([]string, error) {
    if nodeType != NodeTypeString || len(lines) == 0 {
        return lines, nil
    }
    
    kv, err := parseKeyValueLines(lines)
    if err != nil {
        return nil, err
    }

    result, err := json.Marshal(kv)
    if err != nil {
        return nil, fmt.Errorf("key_value marshal error: %w", err)
    }
    return []string{string(result)}, nil
}

// SplitLinesProcessor splits each line into fields for array node types.
type SplitLinesProcessor struct{}

func (p *SplitLinesProcessor) Name() string { return ProcessorTypeSplitLines }

func (p *SplitLinesProcessor) Process(lines []string, nodeType NodeType) ([]string, error) {
    if nodeType != NodeTypeArray {
        return lines, nil
    }
	result := make([]string, 0, len(lines)*3)
    for _, line := range lines {
        result = append(result, strings.Fields(line)...)
    }
    return result, nil
}
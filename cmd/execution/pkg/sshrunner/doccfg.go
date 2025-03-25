package sshrunner

import (
	"encoding/json"
	"os"
)


// Node represents a single element in the linked list
type Node struct {
	Key         string  `json:"key,omitempty"`      // Field name (e.g., "cpu", "system")
	Type        string  `json:"type"`               // "string", "array", "object"
	Script      string  `json:"script,omitempty"`   // Script to execute (empty for objects)
	PostProcess string  `json:"post_process,omitempty"` // Processing rule
    Fields      map[string]*Node  `json:"fields,omitempty"` // Add this field	
	Next        *Node   `json:"-"`                  // Next node in the list (not serialized)
}

// Config holds the documentation configuration
type SshConfig struct {
	Version    string `json:"version"`
	RemoteHost string `json:"remote_host"`
	Login	   string `json:"-"`
	Password   string `json:"-"`
	IP 		   string `json:"ip"`
	Head       *Node  `json:"-"` // Head of the linked list (built from structure)
}

// Load reads and parses the config file, building a linked list
func LoadCfg(filename string) (*SshConfig, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }

    var temp struct {
        Version    string            `json:"version"`
        RemoteHost string            `json:"remote_host"`
        Structure  map[string]*Node  `json:"structure"`
    }
    if err := json.Unmarshal(data, &temp); err != nil {
        return nil, err
    }

    config := &SshConfig{
        Version:    temp.Version,
        RemoteHost: temp.RemoteHost,
        Head:       buildLinkedList(temp.Structure),
    }
    return config, nil
}

// buildLinkedList flattens the structure into a linked list
func buildLinkedList(structure map[string]*Node) *Node {
	var head, tail *Node
	for key, node := range structure {
		node.Key = key
		if head == nil {
			head = node
			tail = node
		} else {
			tail.Next = node
			tail = node
		}

		// Recursively handle nested objects
		if node.Type == "object" && len(node.Fields) > 0 {
			nestedHead := buildLinkedList(node.Fields)
			tail.Next = nestedHead
			for tail.Next != nil {
				tail = tail.Next
			}
		}
	}
	return head
}
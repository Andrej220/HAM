package sshrunner

import (
	"encoding/json"
	"os"
	"log"
)

type Node struct {
	Key         string  `json:"key,omitempty"`      
	Type        string  `json:"type"`               
	Script      string  `json:"script,omitempty"`   
	PostProcess string  `json:"post_process,omitempty"` 
    Fields      map[string]*Node  `json:"fields,omitempty"` 
	Next        *Node   `json:"-"`                  
	Stdout		[]string  `json:"stdout,omitempty"`
	Stderr      []string  `json:"stderr,omitempty"`
	Result      any `json:"result"`
}


type NodeData struct {
    Type        string            `json:"type,omitempty"`
    Script      string            `json:"script,omitempty"`
    PostProcess string            `json:"post_process,omitempty"`
    Fields      map[string]*NodeData `json:"fields,omitempty"`
    Stdout      []string          `json:"stdout,omitempty"`
    Stderr      []string          `json:"stderr,omitempty"`
    Result      any               `json:"result,omitempty"`
}
// Config holds the documentation configuration
type DocConfig struct {
	Version    string `json:"version"`
	RemoteHost string `json:"remote_host"`
	Login	   string `json:"login"`
	Password   string `json:"password"`
	Head       *Node  `json:"result,omitempty"` 
}

type TempConfig struct {
    Version    string            `json:"version"`
    RemoteHost string            `json:"remote_host"`
    Structure  map[string]*NodeData  `json:"structure"`
}

func LoadCfg(filename string) (*DocConfig, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }

    var temp struct {
        Version    string            `json:"version"`
        RemoteHost string            `json:"remote_host"`
		Login	   string			 `json:"login"`
		Password   string			 `json:"password"`
        Structure  map[string]*Node  `json:"structure"`
    }
    if err := json.Unmarshal(data, &temp); err != nil {
        return nil, err
    }

    config := &DocConfig{
        Version:    temp.Version,
        RemoteHost: temp.RemoteHost,
		Login:		temp.Login,
		Password:   temp.Password,
        Head:       buildLinkedList(temp.Structure),
    }
    return config, nil
}

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

func (n *Node) MarshalJSON() ([]byte, error) {
    structure := make(map[string]any)
    for current := n; current != nil; current = current.Next {
        if current.Key == "" {
            continue 
        }
        nodeData := map[string]any{
            "type": current.Type,
        }
		if current.Stderr != nil {
            nodeData["stderr"] = current.Stderr
        }
        if current.Result != nil {
            nodeData["result"] = current.Result
        }
        if current.Type == "object" && len(current.Fields) > 0 {
            nodeData["fields"] = current.Fields
        }
        structure[current.Key] = nodeData
    }
    return json.Marshal(structure)
}


func BuildStructureFromList(head *Node) map[string]*NodeData {
    structure := make(map[string]*NodeData)
    current := head

    for current != nil {
        if current.Key == "" {
            current = current.Next
            continue
        }

        key := current.Key
        nodeData := &NodeData{
            Stderr:      current.Stderr,
            Result:      current.Result,
        }

        if current.Type == "object" && len(current.Fields) > 0 {
            nodeData.Fields = make(map[string]*NodeData)
            nextNode := current.Next
            for fieldKey := range current.Fields {
                for nextNode != nil && nextNode.Key == fieldKey {
                    nodeData.Fields[fieldKey] = &NodeData{
                        Stderr:      nextNode.Stderr,
                        Result:      nextNode.Result,
                    }
                    nextNode = nextNode.Next
                    break
                }
            }
            current = nextNode
        } else {
            current = current.Next
        }
        structure[key] = nodeData
    }
    return structure
}


//TODO: split this function - make json and send to data service to store results
func WriteJson(n *DocConfig, filename string) error{
    file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        log.Printf("Failed to open file %s: %v", filename, err)
        return err
    }
    defer file.Close()

    temp := TempConfig{
        Version:    n.Version,
        RemoteHost: n.RemoteHost,
        Structure:  BuildStructureFromList(n.Head),
    }

    data, err := json.MarshalIndent(temp, "", "  ") 
    if err != nil {
        log.Printf("Failed to marshal node to JSON: %v", err)
        return err
    }

    if _, err := file.Write(data); err != nil {
        log.Printf("Failed to write JSON to file %s: %v", filename, err)
        return err
    }
    return nil
}
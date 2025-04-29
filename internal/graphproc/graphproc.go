package graphproc

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Node struct {
	ID          string   `json:"id"`                    
	Type        string   `json:"type,omitempty"`        
	Script      string   `json:"script,omitempty"`      
	PostProcess string   `json:"post_process,omitempty"` 
	Children    []*Node  `json:"children,omitempty"`    
	Result      []string `json:"result,omitempty"`      
	Stderr 		[]string `json:"error,omitempy"`
}

type Config struct {
	Version    string    `json:"version"`
	RemoteHost string    `json:"remote_host"`
	Password   string    `json:"password"`
	Login      string    `json:"login"`
	Structure  *Node     `json:"structure"`
}

type alias struct {
	ID          string   `json:"id"`
	Type        string   `json:"type,omitempty"`
	Children    []*Node  `json:"fields,omitempty"`
	Result      []string `json:"result,omitempty"`
	Error		[]string `json:"error,omitempty"`
}

type Graph struct {
	Config *Config
	Root   *Node
}

func (n *Node) MarshalJSON() ([]byte, error) {

	alias := &alias{
		ID:          n.ID,
		Type:        n.Type,
		Children:    n.Children,
		Result:      n.Result,
		Error:		 n.Stderr,
	}

	return json.Marshal(alias)
}

func NewGraphFromJSON(filePath string) (*Graph, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	return &Graph{
		Config: &config,
		Root:   config.Structure,
	}, nil
}

func (g *Graph) NodeGenerator() <-chan *Node {
	ch := make(chan *Node, 100)
	go func() {
		defer close(ch)
		g.traverseDFS(g.Root, ch)
	}()
	return ch
}

func (g *Graph) traverseDFS(node *Node, ch chan<- *Node) {
	if node == nil {
		return
	}

	ch <- node
	for _, child := range node.Children {
		g.traverseDFS(child, ch)
	}
}

func (g *Graph) ProcessNodes() error {

	nodeChan := g.NodeGenerator()
	var wg sync.WaitGroup

	for node := range nodeChan {
		wg.Add(1)
		go func(n *Node) {
			defer wg.Done()
		}(node)
	}
	wg.Wait()
	return nil
}

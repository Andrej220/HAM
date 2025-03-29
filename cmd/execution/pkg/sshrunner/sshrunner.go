package sshrunner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

const MAXLINES = 50

type SSHJob struct {
    HostID      int
    ScriptID    int
    UUID        uuid.UUID
    Ctx         context.Context
}

type task struct{
    node    *Node
    client  *ssh.Client
    ctx     context.Context
}

func newSSHClient(remote string, login string, password string ) (*ssh.Client,  error){
    log.Println("Connecting to SSH server")
    
    // TODO: just for the test 
    // change it for production
    config := &ssh.ClientConfig{
        User: login,
        Auth: []ssh.AuthMethod{ssh.Password(password)},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:          10 * time.Second,
    }
    
    client, err := ssh.Dial("tcp", remote, config)
    if err != nil {
        return nil,  fmt.Errorf("failed to dial  %w", err)
    }
    
    log.Printf("SSH connection established to %s. ",remote)

    return client,  nil
}

func RunJob(jb SSHJob) error{

    graph, err := NewGraphFromJSON("docconfig.json")

    if err != nil {
        log.Printf("Error reading configuration %+v", err)
        return err
    }

    client, err := newSSHClient(graph.Config.RemoteHost, graph.Config.Login, graph.Config.Password)
    if err != nil{
        log.Printf("Error connection remote host: %+v", err)
        return err
    }
    defer client.Close()
	
    nodeChan := graph.NodeGenerator()
    var wg sync.WaitGroup	
    for node := range nodeChan {
        t := task{node:node,client: client, ctx:jb.Ctx}
		wg.Add(1)
		go func(n *Node) {
			defer wg.Done()
            err = runTask(&t)
            if err != nil{
                log.Printf("Task execution failed: %+v", err)
            }
		}(node)
	}
    wg.Wait()
    WriteJson(graph, "/tmp/test.json")
    return nil
}

// Add error propagation 
func runTask(t *task) error {

    // skip if object (no script to execute)
    if t.node.Type == "object"{
        return nil
    }

    select {
    case <-t.ctx.Done():
        log.Printf("Task canceled before start: %v", t.ctx.Err())
        return t.ctx.Err()
    default:
    }

    session, err := t.client.NewSession()
    if err != nil {
        log.Printf("failed to create session: %+v",err)
        return err
    }
    defer session.Close()

    log.Printf("Session is created: %s",t.client.RemoteAddr())

    stdout, err := session.StdoutPipe()
    if err != nil {
        log.Printf("Failed to get stdout pipe: %+v", err)
        return err
    }

    stderr, err := session.StderrPipe()
    if err != nil {
        log.Printf("Failed to get stderr pipe: %+v", err)
        return err
    }

    if len(t.node.Script) == 0 {
        return nil
    }

    log.Println("Executing script.")
    err = session.Start( t.node.Script )
    if err != nil {
        log.Printf("Failed to start script: %v", err)
        return err
    }

    log.Println("Start reading stdout.")

    // run them as goroutines
    var res []string
    res = readOutput(stdout, t.ctx)
    t.node.Stderr = readOutput(stderr, t.ctx)
    t.node.Result = processOutput(res, t.node.PostProcess, t.node.Type)

    select {
    case <-t.ctx.Done():
        log.Printf("Task canceled before wait: %v", t.ctx.Err())
        return t.ctx.Err()
    default:
        if err := session.Wait(); err != nil {
            log.Printf("Script execution error: %v", err)
        }
    }
    return nil
}

func readOutput(reader io.Reader, ctx  context.Context) []string {
    var lines []string
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            log.Printf("Output reading canceled: %v", ctx.Err())
            return nil
        default:
            lines = append(lines, scanner.Text())
        }
    }
    if err := scanner.Err(); err != nil {
        log.Printf("Scan error: %v", err)
    }
    return lines
}

func processOutput(lines []string, postProcess string, nodeType string) []string {
    if len(lines) == 0 {
        return nil 
    }
    switch postProcess {
    case "trim":
        if nodeType == "string" {
            return lines
        }
        trimmed := make([]string, 0, len(lines))
        for _, line := range lines {
            trimmed = append(trimmed, strings.TrimSpace(line))
        }
        return trimmed 
    case "split_lines":
        fmt.Println(nodeType, postProcess)
        if nodeType == "array" {
            var result []string
            for _, line := range lines {
                fields := strings.Fields(line) 
                result = append(result, fields...)
            }
            return result
        }
        return lines
    default:
        return lines 
    }
}

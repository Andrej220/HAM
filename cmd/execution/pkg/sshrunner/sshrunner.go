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
    ds "executor/pkg/dataservice"
)

const (
        MAXLINES = 50
        maxConcurrent = 7
    )

type SSHJob struct {
    HostID      int
    ScriptID    int
    UUID        uuid.UUID
    Ctx         context.Context
}

type task struct{
    node    *Node
    client  *ssh.Client
    session *ssh.Session
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
        BannerCallback: func(message string) error {return nil},  //ignore banner
    }
    
    client, err := ssh.Dial("tcp", remote, config)
    if err != nil {
        return nil,  fmt.Errorf("failed to dial  %w", err)
    }
    
    log.Printf("SSH connection established to %s. ",remote)

    return client,  nil
}

func newSSHSession(client *ssh.Client) (*ssh.Session, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("session creation failed: %w", err)
	}
	return session, nil
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

    taskChan := make(chan struct{}, maxConcurrent) // Semaphore, limits concurent workers
	var mu sync.Mutex
	var errors []error

    for node := range nodeChan {
        taskChan <- struct{}{}      //take a slot for the task
        session, err := newSSHSession(client)
        if err != nil {
			log.Printf("Failed to create session for node %v: %v", node, err)
            <- taskChan //release slot dues to failure
			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
			continue 
		}

        t := task{node:node,client: client, ctx:jb.Ctx, session: session}
		wg.Add(1)
		go func(n *Node) {
			defer wg.Done()
            defer t.session.Close()
            defer func(){ <-taskChan}() //release task slot

            err = runTask(&t)
            if err != nil{
                mu.Lock()
                errors = append(errors, err)
                mu.Unlock()
                log.Printf("Task execution failed: %+v", err)
            }
		}(node)
	}
    wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("one or more tasks failed, first error: %v", errors[0])
	}
    //test only 
    ds.WriteFile(graph.Root, "/tmp/test.json")
    return nil
}

// Add error propagation 
func runTask(t *task) error {

    // skip if object (no script to execute)
    if t.node.Type == "object" || len(t.node.Script) == 0{
        return nil
    }

    if t.session == nil {
        return fmt.Errorf("Error, session has not been created: %s", t.client.RemoteAddr())
    }

    select {
    case <-t.ctx.Done():
        log.Printf("Task canceled before start: %v", t.ctx.Err())
        return t.ctx.Err()
    default:
    }

    log.Printf("Session is created for %s", t.client.RemoteAddr())

    stdout, err := t.session.StdoutPipe()
    if err != nil {
        log.Printf("Failed to get stdout pipe: %+v", err)
        return err
    }

    stderr, err := t.session.StderrPipe()
    if err != nil {
        log.Printf("Failed to get stderr pipe: %+v", err)
        return err
    }

    log.Println("Executing script.")
    err = t.session.Start( t.node.Script )
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
        if err := t.session.Wait(); err != nil {
            log.Printf("Script error: %v", err)
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
            return lines
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

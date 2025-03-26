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

func newSSHClient(execConfig *DocConfig) (*ssh.Client,  error){
    log.Println("Connecting to SSH server")
    
    // TODO: just for the test 
    // change it for production
    config := &ssh.ClientConfig{
        User: execConfig.Login,
        Auth: []ssh.AuthMethod{ssh.Password(execConfig.Password)},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:          10 * time.Second,
    }
    
    client, err := ssh.Dial("tcp", execConfig.RemoteHost, config)
    if err != nil {
        return nil,  fmt.Errorf("failed to dial  %w", err)
    }
    
    log.Printf("SSH connection established to %s. ",execConfig.RemoteHost)

    return client,  nil
}

func RunJob(jb SSHJob) error{

    jobcfg,err := LoadCfg("docconfig.json")
    if err != nil{
        log.Printf("Error reading configuration %v:", err)
        return err
    }
    // Create connection, return 
    client, err := newSSHClient(jobcfg)
    if err != nil{
        log.Printf("Error connection remote host: %+v", err)
        return err
    }
    defer client.Close()

    var wg sync.WaitGroup
    for node :=jobcfg.Head; node != nil; node = node.Next{
        t := task{node:node,client: client, ctx:jb.Ctx}
        wg.Add(1)
        go runTask(&t, &wg)
    }
    wg.Wait()
    // for the test only
    WriteJson(jobcfg, "/tmp/test.json")
    return nil
}

// Add error propagation 
func runTask(t *task, wg *sync.WaitGroup) {
    defer wg.Done()

    select {
    case <-t.ctx.Done():
        log.Printf("Task canceled before start: %v", t.ctx.Err())
        return
    default:
    }

    session, err := t.client.NewSession()
    if err != nil {
        log.Printf("failed to create session: %+v",err)
        return 
    }
    defer session.Close()

    log.Printf("Session is created: %s",t.client.RemoteAddr())

    stdout, err := session.StdoutPipe()
    if err != nil {
        log.Printf("Failed to get stdout pipe: %+v", err)
        return
    }

    stderr, err := session.StderrPipe()
    if err != nil {
        log.Printf("Failed to get stderr pipe: %+v", err)
        return
    }

    log.Println("Executing script.")
    err = session.Start( t.node.Script)
    if err != nil {
        log.Printf("Failed to start script: %v", err)
        return
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
        return
    default:
        if err := session.Wait(); err != nil {
            log.Printf("Script execution error: %v", err)
        }
    }
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

func processOutput(lines []string, postProcess string, nodeType string) any {
    if len(lines) == 0 {
        return nil 
    }
    switch postProcess {
    case "trim":
        if nodeType == "string" {
            return strings.TrimSpace(strings.Join(lines, "\n"))
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
        return strings.Join(lines,  "\n")
    default:
        return lines 
    }
}

//func readOutput(reader io.Reader, t *task) {
//    var lines []string
//    scanner := bufio.NewScanner(reader)
//    for scanner.Scan() {
//        select {
//        case <-t.ctx.Done():
//            log.Printf("Output reading canceled: %v", t.ctx.Err())
//            return
//        default:
//            lines = append(lines, scanner.Text())
//        }
//    }
//    if err := scanner.Err(); err != nil {
//        log.Printf("Scan error: %v", err)
//    }
//    t.node.Result = processOutput(lines, t.node.PostProcess)
//}

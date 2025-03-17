package main

import (
	"bufio"
	"fmt"
	"time"
    "log"
	"golang.org/x/crypto/ssh"
    "github.com/google/uuid"
    "io"
)

const MAXLINES = 50

type SSHJobStruct struct {
    HostID      int
    ScriptID    int
    UUID        uuid.UUID
    dataChan    chan string
}

type SSHExecConfig struct {
	IP          string
	Username    string
	Password    string
	Script      string
}

type OutputHandler interface {
    Process(line string) error
    Source() string
}

type ConfigData struct {
    StdoutLines []string
    StderrLines []string
    Errors      []string
}

type StdoutHandler struct {
    data *ConfigData
}

func (h *StdoutHandler) Process(line string) error {
    h.data.StdoutLines = append(h.data.StdoutLines, line)
    return nil
}

func (h *StdoutHandler) Source() string {
    return "stdout"
}

type StderrHandler struct {
    data *ConfigData
}

func (h *StderrHandler) Process(line string) error {
    h.data.StderrLines = append(h.data.StderrLines, line)
    return nil
}

func (h *StderrHandler) Source() string {
    return "stderr"
}

type ErrorHandler struct {
    data *ConfigData
}

func (h *ErrorHandler) Process(line string) error {
    h.data.Errors = append(h.data.Errors, line)
    return nil
}

func (h *ErrorHandler) Source() string {
    return "error"
}

type Output struct {
    Handler OutputHandler
    Line    string
}

func scanPipe(reader io.Reader, outputChan chan<- Output, handler OutputHandler, scanDone chan<- struct{}) {
    defer func() { scanDone <- struct{}{} }()
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        outputChan <- Output{Handler: handler, Line: scanner.Text()}
    }
    if err := scanner.Err(); err != nil {
        outputChan <- Output{Handler: handler, Line: fmt.Sprintf("%s scan error: %v", handler.Source(), err)}
    }
}

func createSSHSession(execConfig SSHExecConfig) (*ssh.Client, *ssh.Session, error){
    log.Println("Connecting to SSH server")
    
    config := &ssh.ClientConfig{
        User: execConfig.Username,
        Auth: []ssh.AuthMethod{ssh.Password(execConfig.Password)},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:          10 * time.Second,
    }
    
    client, err := ssh.Dial("tcp", execConfig.IP, config)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to dial  %w", err)
    }
    
    log.Printf("SSH connection established to %s. Creating session...",execConfig.IP)

    session, err := client.NewSession()
    if err != nil {
        return nil, nil, fmt.Errorf("failed to create session: %w",err)
    }

    log.Printf("Session is created: %s",execConfig.IP)

    return client, session, nil
}

func executeScript(execConfig SSHExecConfig, outputChan chan<- Output, doneChan chan<- struct{}, data *ConfigData) {
    defer close(outputChan)
    defer func() { doneChan <- struct{}{} }()
    
    handlers := struct {
        Stdout OutputHandler
        Stderr OutputHandler
        Error  OutputHandler
    }{
        Stdout: &StdoutHandler{data},
        Stderr: &StderrHandler{data},
        Error:  &ErrorHandler{data},
    }

    client, session, err := createSSHSession(execConfig)
    if err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("failed to establish connection: %+v", err)}
        return 
    }
    
    defer client.Close()
    defer session.Close()

    stdout, err := session.StdoutPipe()
    if err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("Failed to get stdout pipe: %+v", err)}
        return
    }

    stderr, err := session.StderrPipe()
    if err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("Failed to get stderr pipe: %+v", err)}
        return
    }

    log.Println("Executing script.")
    err = session.Start( execConfig.Script)
    if err != nil {
        fmt.Println(execConfig.Script)
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("Failed to start script: %v", err)}
        return
    }

    scanDone := make(chan struct{}, 2)

    log.Println("Start reading stdout.")
    go scanPipe(stdout, outputChan, handlers.Stdout, scanDone)
    go scanPipe(stderr, outputChan, handlers.Stderr, scanDone)

    if err := session.Wait(); err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("Script execution error: %v", err)}
    }

    <-scanDone
    <-scanDone
    log.Println("Script execution and stdout reading completed.")
}

func collectResults(inputChan chan Output, outChan chan string ,doneChan chan<- struct{},data *ConfigData){
    defer func() { doneChan <- struct{}{} }()
    log.Println("Collecting data...")
    for output := range inputChan {
        // pass data only if stdout and outChan is set
        // TODO: case for type assertion
        //switch v := i.(type) {
        //case int:
        //    fmt.Println("i is an int:", v)
        //case string:
        //    fmt.Println("i is a string:", v)
        //case float64:
        //    fmt.Println("i is a float64:", v)
        //default:
        //    fmt.Printf("i is of an unknown type: %T\n", v)
        //}
        if _,ok := output.Handler.(*StdoutHandler); ok && outChan != nil{
            outChan <- output.Line + "\n"
        }
        if err := output.Handler.Process(output.Line); err!=nil{
            data.Errors = append(data.Errors, fmt.Sprintf("Processing %s: %v", output.Handler.Source(), err))
        }
    }
    log.Println("End of data collection.")
}

func GetRemoteConfig(jb SSHJobStruct) error {
    outputChan := make(chan Output, MAXLINES)
    doneChan := make(chan struct{}, 2)

    data := ConfigData{}
    
// TEST config, should be fetched from DB
    configs,err := LoadConfigs()
    if err != nil{
        log.Printf("Error reading configuration %v:", err)
        return err
    }
//!!!!!!!!!!!!!!!

    sshExecConfig := configs[jb.HostID]
    
    go executeScript(sshExecConfig, outputChan, doneChan, &data)
    go collectResults(outputChan, jb.dataChan, doneChan, &data)
    
    <-doneChan
    <-doneChan

    log.Println("Script execution and data collection completed.")
    //TODO: send data to pareser, return
    fmt.Println("Stdout:", data.StdoutLines)
    fmt.Println("Stderr:", data.StderrLines)
    fmt.Println("Errors:", data.Errors)
    return nil
}
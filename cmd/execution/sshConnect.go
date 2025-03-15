package main

import (
	"bufio"
	"fmt"
	"time"
    "log"
	"golang.org/x/crypto/ssh"
    "github.com/google/uuid"
)

const MAXLINES = 50

type SSHJobStruct struct {
    HostID      int
    ScriptID    int
    UUID        uuid.UUID
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
    log.Println("Connecting to SSH server")

    config := &ssh.ClientConfig{
        User: execConfig.Username,
        Auth: []ssh.AuthMethod{ssh.Password(execConfig.Password)},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         10 * time.Second,
    }

    client, err := ssh.Dial("tcp", execConfig.IP, config)
    if err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("failed to dial %s: %v", execConfig.IP, err)}
        return
    }
    defer client.Close()

    log.Println("SSH connection established. Creating session...")
    session, err := client.NewSession()
    if err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("failed to create session: %v", err)}
        return
    }
    defer session.Close()

    stdout, err := session.StdoutPipe()
    if err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("Failed to get stdout pipe: %v", err)}
        return
    }

    stderr, err := session.StderrPipe()
    if err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("Failed to get stderr pipe: %v", err)}
        return
    }

    log.Println("Executing script.")
    err = session.Start( execConfig.Script)
    if err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("Failed to start script: %v", err)}
        return
    }

    scanDone := make(chan struct{}, 2)

    log.Println("Start reading stdout.")
    go func() {
        defer func() { scanDone <- struct{}{} }()
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            outputChan <- Output{Handler: handlers.Stdout, Line: scanner.Text()}
        }
        if err := scanner.Err(); err != nil {
            outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("stdout scan error: %v", err)}
        }
    }()

    go func() {
        defer func() { scanDone <- struct{}{} }()
        scanner := bufio.NewScanner(stderr)
        for scanner.Scan() {
            outputChan <- Output{Handler: handlers.Stderr, Line: scanner.Text()}
        }
        if err := scanner.Err(); err != nil {
            outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("stderr scan error: %v", err)}
        }
    }()

    if err := session.Wait(); err != nil {
        outputChan <- Output{Handler: handlers.Error, Line: fmt.Sprintf("Script execution error: %v", err)}
    }

    <-scanDone
    <-scanDone
    log.Println("cript execution and stdout reading completed.")
}

func collectResults(outputChan chan Output, doneChan chan<- struct{},data *ConfigData){
    defer func() { doneChan <- struct{}{} }()
    log.Println("Collecting data...")
    for output := range outputChan {
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
    go collectResults(outputChan, doneChan, &data)
    
    <-doneChan
    <-doneChan

    log.Println("Script execution and data collection completed.")
    //TODO: send data to pareser, return
    fmt.Println("Stdout:", data.StdoutLines)
    fmt.Println("Stderr:", data.StderrLines)
    fmt.Println("Errors:", data.Errors)
    return nil
}
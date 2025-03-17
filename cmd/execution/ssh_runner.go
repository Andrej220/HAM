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

type SSHJob struct {
    HostID      int
    ScriptID    int
    UUID        uuid.UUID
    dataCh    chan string
}

type SSHConfig struct {
	IP          string
	Username    string
	Password    string
	Script      string
}

type OutputHandler interface {
    Process(line string) error
    Source() string
}

type ExecResults struct {
    StdoutLines []string
    StderrLines []string
    Errors      []string
}

type StdoutHandler struct {
    data *ExecResults
}

func (h *StdoutHandler) Process(line string) error {
    h.data.StdoutLines = append(h.data.StdoutLines, line)
    return nil
}

func (h *StdoutHandler) Source() string {
    return "stdout"
}

type StderrHandler struct {
    data *ExecResults
}

func (h *StderrHandler) Process(line string) error {
    h.data.StderrLines = append(h.data.StderrLines, line)
    return nil
}

func (h *StderrHandler) Source() string {
    return "stderr"
}

type ErrorHandler struct {
    data *ExecResults
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

func readOutput(reader io.Reader, outputCh chan<- Output, handler OutputHandler, scanDoneCh chan<- struct{}) {
    defer func() { scanDoneCh <- struct{}{} }()
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        outputCh <- Output{Handler: handler, Line: scanner.Text()}
    }
    if err := scanner.Err(); err != nil {
        outputCh <- Output{Handler: handler, Line: fmt.Sprintf("%s scan error: %v", handler.Source(), err)}
    }
}

func newSSHSession(execConfig SSHConfig) (*ssh.Client, *ssh.Session, error){
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

func runRemoteScript(execConfig SSHConfig, outputCh chan<- Output, doneCh chan<- struct{}, data *ExecResults) {
    defer close(outputCh)
    defer func() { doneCh <- struct{}{} }()
    
    streams := struct {
        Stdout OutputHandler
        Stderr OutputHandler
        Error  OutputHandler
    }{
        Stdout: &StdoutHandler{data},
        Stderr: &StderrHandler{data},
        Error:  &ErrorHandler{data},
    }

    client, session, err := newSSHSession(execConfig)
    if err != nil {
        outputCh <- Output{Handler: streams.Error, Line: fmt.Sprintf("failed to establish connection: %+v", err)}
        return 
    }
    
    defer client.Close()
    defer session.Close()

    stdout, err := session.StdoutPipe()
    if err != nil {
        outputCh <- Output{Handler: streams.Error, Line: fmt.Sprintf("Failed to get stdout pipe: %+v", err)}
        return
    }

    stderr, err := session.StderrPipe()
    if err != nil {
        outputCh <- Output{Handler: streams.Error, Line: fmt.Sprintf("Failed to get stderr pipe: %+v", err)}
        return
    }

    log.Println("Executing script.")
    err = session.Start( execConfig.Script)
    if err != nil {
        fmt.Println(execConfig.Script)
        outputCh <- Output{Handler: streams.Error, Line: fmt.Sprintf("Failed to start script: %v", err)}
        return
    }

    scanDoneCh := make(chan struct{}, 2)

    log.Println("Start reading stdout.")
    go readOutput(stdout, outputCh, streams.Stdout, scanDoneCh)
    go readOutput(stderr, outputCh, streams.Stderr, scanDoneCh)

    if err := session.Wait(); err != nil {
        outputCh <- Output{Handler: streams.Error, Line: fmt.Sprintf("Script execution error: %v", err)}
    }

    <-scanDoneCh
    <-scanDoneCh
    log.Println("Script execution and stdout reading completed.")
}

func handleOutput(inputCh chan Output, outChan chan string ,doneCh chan<- struct{},data *ExecResults){
    defer func() { doneCh <- struct{}{} }()
    log.Println("Collecting data...")
    for output := range inputCh {
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

func FetchRemoteData(jb SSHJob) error {
    outputCh := make(chan Output, MAXLINES)
    doneCh := make(chan struct{}, 2)

    result := ExecResults{}
    
// TEST config, should be fetched from DB
    configs,err := LoadConfigs()
    if err != nil{
        log.Printf("Error reading configuration %v:", err)
        return err
    }
//!!!!!!!!!!!!!!!

    sshExecCfg := configs[jb.HostID]
    
    go runRemoteScript(sshExecCfg, outputCh, doneCh, &result)
    go handleOutput(outputCh, jb.dataCh, doneCh, &result)
    
    <-doneCh
    <-doneCh

    log.Println("Script execution and data collection completed.")
    //TODO: send data to pareser, return
    fmt.Println("Stdout:", result.StdoutLines)
    fmt.Println("Stderr:", result.StderrLines)
    fmt.Println("Errors:", result.Errors)
    return nil
}
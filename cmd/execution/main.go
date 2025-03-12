package main

import (
	"bufio"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// Output represents a line of output with its source
type Output struct {
	Source int
	Line   string
}

const (
    Stdout = iota 
    Stderr        
    Error         
)

func executeScript(ip, username, password, script string, outputChan chan<- Output, doneChan chan<- struct{}) {
	defer close(outputChan)
	defer func() { doneChan <- struct{}{} }() 

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Replace in production
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", ip, config)
	if err != nil {
		outputChan <- Output{Source: Error, Line: fmt.Sprintf("failed to dial: %v", err)}
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		outputChan <- Output{Source: Error, Line: fmt.Sprintf("failed to create session: %v", err)}
		return
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		outputChan <- Output{Source: Error, Line: fmt.Sprintf("Failed to get stdout pipe: %v", err)}
		return
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		outputChan <- Output{Source: Error, Line: fmt.Sprintf("Failed to get stderr pipe: %v", err)}
		return
	}

	err = session.Start(script)
	if err != nil {
		outputChan <- Output{Source: Error, Line: fmt.Sprintf("Failed to start script: %v", err)}
		return
	}

	// Collect output
	scanDone := make(chan struct{}, 2) // Wait for 2 goroutines 
	
	// Collect stdout
	go func() {
		defer func() { scanDone <- struct{}{} }()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			outputChan <- Output{Source: Stdout, Line: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			outputChan <- Output{Source: Error, Line: fmt.Sprintf("stdout scan error: %v", err)}
		}
	}()


	// Collect stderr 
	go func() {
		defer func() { scanDone <- struct{}{} }()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			outputChan <- Output{Source: Stderr, Line: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			outputChan <- Output{Source: Error, Line: fmt.Sprintf("stderr scan error: %v", err)}
		}
	}()


	if err := session.Wait(); err != nil {
		outputChan <- Output{Source: Error, Line: fmt.Sprintf("Script execution error: %v", err)}
	}
	
	// Wait for both scanners to finish
	<-scanDone
	<-scanDone
	close(scanDone) 
}

func collectResults(outputChan <-chan Output, doneChan chan<- struct{}) {
	defer func() { doneChan <- struct{}{} }() 

	for output := range outputChan {
		switch output.Source {
		case Stdout:
			fmt.Println("STDOUT:", output.Line)
		case Stderr:
			fmt.Println("STDERR:", output.Line)
		case Error:
			fmt.Println("ERROR:", output.Line)
		}
	}
}

func main() {
	outputChan := make(chan Output, 10)
	doneChan := make(chan struct{}, 2) // Buffer for 2 goroutines

	go executeScript("192.168.1.105:22", "master", "nekochegar", "ip a", outputChan, doneChan)
	go collectResults(outputChan, doneChan)

	// Wait for both goroutines to signal completion
	<-doneChan
	<-doneChan

	fmt.Println("Execution and collection completed.")
}
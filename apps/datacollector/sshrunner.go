package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"golang.org/x/sync/errgroup"
	gp "github.com/andrej220/HAM/pkg/graphproc"
	"github.com/andrej220/HAM/pkg/executor"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"time"
	"os"
)

const (
	MAXLINES      = 50
	maxConcurrent = 7
)

type Executor interface{
	Run(script string, ctx context.Context) ( stdout *io.Reader, stderr *io.Reader, err error)
}

type SSHJob struct {
	HostID   int
	ScriptID int
	UUID     uuid.UUID
	Ctx      context.Context
}

type task struct {
	node    *gp.Node
	client  *ssh.Client
	session *ssh.Session
}

func (t * task)Run(script string, ctx context.Context)(stdout io.Reader, stderr io.Reader, err error){
	if t.session == nil {
		return nil, nil, fmt.Errorf("Error, session has not been created: %s", t.client.RemoteAddr())
	}

	select {
	case <-ctx.Done():
		log.Printf("Task canceled before start: %v", ctx.Err())
		return nil, nil, ctx.Err()
	default:
	}

	log.Printf("Session is created for %s", t.client.RemoteAddr())

	stdout, err = t.session.StdoutPipe()
	if err != nil {
		log.Printf("Failed to get stdout pipe: %+v", err)
		return nil, nil, err
	}

	stderr, err = t.session.StderrPipe()
	if err != nil {
		log.Printf("Failed to get stderr pipe: %+v", err)
		return nil, nil, err
	}
	
	log.Println("Executing script.")
	err = t.session.Start(t.node.Script)
	if err != nil {
		log.Printf("Failed to start script: %v", err)
		return nil, nil, err
	}
	// Wait for script completion to ensure output is ready
	go func() {
		if err := t.session.Wait(); err != nil {
			log.Printf("Script execution failed: %v", err)
		}
	}()
	return stdout, stderr, nil
}

func loadGraphConfig(jb SSHJob)(*gp.Graph, error){

	//TODO: load task from the database
	graph, err := gp.NewGraphFromJSON("/etc/ham/docconfig.json")
	if err != nil {
		log.Printf("Error reading configuration %+v", err)
		return nil, err
	}
	graph.UUID = jb.UUID

	// TODO: delete it, just for the test. Should be populated from the database
	graph.HostCfg = &gp.HostConfig{
		CustomerID: 	1,
		HostID:     	jb.HostID,
		ScriptID: 		jb.ScriptID,
	}
	return graph, nil
}

func RunJob(jb SSHJob) (*gp.Graph, error) {
	log.Printf("Starting job for host %d, script %d, UUID %s", jb.HostID, jb.ScriptID, jb.UUID)
    graph, err := loadGraphConfig(jb)
    if err != nil {
        return nil, err
    }
	var sshkeyAuth ssh.AuthMethod
	sshkeyAuth, err = publicKeyAuth(graph.Config.SSHKeyPath) 
	if err != nil {
		log.Printf("Failed parsing ssh keys, %v", err)
		return graph, err
	}
	auth := []ssh.AuthMethod{ssh.Password(graph.Config.Password), sshkeyAuth}
	clientConfig := &ssh.ClientConfig{
		User: graph.Config.Login,
		Auth:            auth, 
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
		BannerCallback:  func(message string) error { return nil }, //ignore banner
	}

    rclient, err := executor.NewResilientClient( graph.Config.RemoteHost, clientConfig )

    if err != nil {
        return nil, fmt.Errorf("ssh dial: %w", err)
    }
    defer rclient.Close()

    exec := executor.NewSSHExecutor(rclient)
    g, ctx := errgroup.WithContext(jb.Ctx)
    for i := 0; i < maxConcurrent; i++ {
        g.Go(func() error {
            for node := range graph.NodeGenerator() {
                task := executor.NewNodeTask(node, exec)  
                if err := task.Execute(ctx); err != nil {
                    return err
                }
            }
            return nil
        })
    }
    return graph, g.Wait()
}

func publicKeyAuth(privateKeyPath string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %v", err)
	}
	return ssh.PublicKeys(signer), nil
}

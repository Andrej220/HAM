package main

//TODO: deeper focus on resilience (retries, circuit breakers)
//TODO: Parameterize back-off and breaker settings	

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"golang.org/x/sync/errgroup"
	gp "github.com/andrej220/HAM/internal/graphproc"
	pc "github.com/andrej220/HAM/internal/processor"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"github.com/cenkalti/backoff/v4"
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
	graph, err := gp.NewGraphFromJSON("docconfig.json")
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
		return nil, fmt.Errorf("config: %w", err)
	}
	rClient, err := NewResilientClient(graph.Config.RemoteHost, graph.Config.Login, graph.Config.Password)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}
	defer rClient.SSHClient.Close()

	g, ctx := errgroup.WithContext(jb.Ctx)
	nodes := graph.NodeGenerator()
	// TODO: uncommect if partial data is needed
	//var mu sync.Mutex  
	//var errors []error

	for i := 0; i < maxConcurrent; i++ {
		g.Go(func() error {
			sess, err := newSSHSession(rClient.SSHClient, rClient.ResConf.CircuitBreaker)
			if err != nil {
				return fmt.Errorf("new session: %w", err) 
			}
			defer sess.Close()

			for node := range nodes {
				if err := processNode(ctx, rClient, sess, node); err != nil {
						//errors = append(errors, err)   //collect errors
						//mu.Unlock()
						//continue
					return err
				}
			}
			return nil
		})
	}
			//g.Wait() // Wait for all workers, ignoring context cancellation
			//if len(errors) > 0 {
			//	return graph, fmt.Errorf("some tasks failed: %v errors", len(errors))
			//}
	if err := g.Wait(); err != nil {
		return graph, fmt.Errorf("job failed: %w", err)
	}
	return graph, nil
}

func processNode(ctx context.Context, rclient * ResilientSSHClient ,session *ssh.Session, node *gp.Node) error {
    if node.Type == "object" || len(node.Script) == 0 {
        return nil
    }
	pChain := pc.NewProcessorChain()
	
    operation := func() error {
        select {
        case <-ctx.Done():
            return backoff.Permanent(ctx.Err()) // Mark as permanent to stop retries
        default:
        }

        t := &task{node: node,client: rclient.SSHClient ,session: session}
		stdout, stderr, err := t.Run(node.Script,ctx)
        if err != nil {
            log.Printf("node %v attempt failed: %v", node, err) 
            return err
        }
		log.Println("Start reading stdout.")
		var res []string
		res = readOutput(stdout, ctx)
		node.Stderr = readOutput(stderr, ctx)
		node.Result, _ = pChain.Process(res,pc.NodeType(node.Type),node.PostProcess)
        return nil
    }
    b := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
    return backoff.Retry(operation, b)
}

func readOutput(reader io.Reader, ctx context.Context) []string {
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
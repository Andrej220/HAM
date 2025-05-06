package main

//TODO: deeper focus on resilience (retries, circuit breakers)

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	//"sync"
	"time"

	"golang.org/x/sync/errgroup"
	gp "github.com/andrej220/HAM/internal/graphproc"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
	"github.com/cenkalti/backoff/v4"
	"github.com/sony/gobreaker"
)

const (
	MAXLINES      = 50
	maxConcurrent = 7
)

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
	ctx     context.Context
}

type ResilientSSHClient struct {
    sshclient  *ssh.Client
    cb         *gobreaker.CircuitBreaker
    backoff    backoff.BackOff
}

func NewResilientClient(ctx context.Context,remote, login, password string) (*ResilientSSHClient, error) {
	config := &ssh.ClientConfig{
		User: login,
		//Auth: []ssh.AuthMethod{ssh.Password(password)},
		Auth:            []ssh.AuthMethod{publicKeyAuth("/home/andrey/.ssh/myadminvps.ru")}, // TODO: move to main
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
		BannerCallback:  func(message string) error { return nil }, //ignore banner
	}

	client, err := ssh.Dial("tcp", remote, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial  %w", err)
	}

    defaultBackoff := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		MaxInterval:         5 * time.Second,
		Multiplier:          1.5,
		RandomizationFactor: 0.5,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}

	cbs := gobreaker.Settings{
		Name:        "ssh-connection",
		MaxRequests: 5,         
		Interval:    1 * time.Minute,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	}

    return &ResilientSSHClient{
        sshclient: client,
        cb: gobreaker.NewCircuitBreaker(cbs),
        backoff: backoff.WithContext(defaultBackoff, ctx),
    }, nil
}

func publicKeyAuth(privateKeyPath string) ssh.AuthMethod {
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}
	return ssh.PublicKeys(signer)
}

func newSSHSession(client *ssh.Client,cb *gobreaker.CircuitBreaker) (*ssh.Session, error) {
	res, err := cb.Execute(func() (any, error) {
        return client.NewSession()
    })
    if err != nil {
        return nil, err
    }
    return res.(*ssh.Session), nil
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
	rClient, err := NewResilientClient(jb.Ctx,graph.Config.RemoteHost, graph.Config.Login, graph.Config.Password)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}
	defer rClient.sshclient.Close()

	g, ctx := errgroup.WithContext(jb.Ctx)
	nodes := graph.NodeGenerator()
	// TODO: uncommect if partial data is needed
	//var mu sync.Mutex  
	//var errors []error

	for i := 0; i < maxConcurrent; i++ {
		g.Go(func() error {
			sess, err := newSSHSession(rClient.sshclient, rClient.cb)
			if err != nil {
				return fmt.Errorf("new session: %w", err) 
			}
			defer sess.Close()

			for node := range nodes {
				if err := processNode(ctx, rClient, sess, node); err != nil {
						//mu.Lock()
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
    operation := func() error {
        select {
        case <-ctx.Done():
            return backoff.Permanent(ctx.Err()) // Mark as permanent to stop retries
        default:
        }

        t := &task{node: node,client: rclient.sshclient ,session: session, ctx: ctx}
        if err := runTask(t); err != nil {
            log.Printf("node %v attempt failed: %v", node, err) 
            return err
        }
        return nil
    }

    b := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
    return backoff.Retry(operation, b)
}

func runTask(t *task) error {
	if t.node.Type == "object" || len(t.node.Script) == 0 {
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
	err = t.session.Start(t.node.Script)
	if err != nil {
		log.Printf("Failed to start script: %v", err)
		return err
	}
	log.Println("Start reading stdout.")

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
			return fmt.Errorf("script execution failed: %w", err)
		}
	}
	return nil
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

func processOutput(lines []string, postProcess string, nodeType string) []string {
	if len(lines) == 0 {
		return nil
	}
	switch postProcess {
	case "trim":
		trimmed := make([]string, 0, len(lines))
		for _, line := range lines {
			trimmed = append(trimmed, strings.TrimSpace(line))
		}
		return trimmed
	case "split_lines":
		if nodeType == "array" {
			var result []string
			for _, line := range lines {
				fields := strings.Fields(line)
				result = append(result, fields...)
			}
			return result
		}
		return lines
	case "key_value":
		if nodeType == "string" {
			kv := make(map[string]string)
			for _, line := range lines {
				parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
				if len(parts) == 2 {
					kv[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
				}
			}
			result := make([]string, 0, len(kv))
			for k, v := range kv {
				result = append(result, fmt.Sprintf("%s: %s", k, v))
			}
			return result
		}
		return lines
	default:
		return lines
	}
}

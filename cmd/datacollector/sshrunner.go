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

func newSSHClient(remote string, login string, password string) (*ssh.Client, error) {
	log.Println("Connecting to SSH server")

	// TODO: just for the test
	// change it for production
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

	log.Printf("SSH connection established to %s. ", remote)

	return client, nil
}

func newSSHSession(client *ssh.Client) (*ssh.Session, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("session creation failed: %w", err)
	}
	return session, nil
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
	client, err := newSSHClient(graph.Config.RemoteHost, graph.Config.Login, graph.Config.Password)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()

	g, ctx := errgroup.WithContext(jb.Ctx)
	nodes := graph.NodeGenerator()
	// TODO: uncommect if partial data is needed
	//var mu sync.Mutex  
	//var errors []error

	for i := 0; i < maxConcurrent; i++ {
		g.Go(func() error {
			for node := range nodes {
				if err := processNode(ctx, client, node); err != nil {
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

func processNode(ctx context.Context, client *ssh.Client, node *gp.Node) error {
    if node.Type == "object" || len(node.Script) == 0 {
        return nil
    }

    operation := func() error {
        select {
        case <-ctx.Done():
            return backoff.Permanent(ctx.Err()) // Mark as permanent to stop retries
        default:
        }

        sess, err := newSSHSession(client)
        if err != nil {
            return fmt.Errorf("new session: %w", err) 
        }
        defer sess.Close()

        t := &task{node: node, client: client, session: sess, ctx: ctx}
        if err := runTask(t); err != nil {
            log.Printf("node %v attempt failed: %v", node, err) 
            return err
        }
        return nil
    }

    b := backoff.WithContext(backoff.NewExponentialBackOff(), ctx)
    return backoff.Retry(operation, b)
}

// TODO: Add error propagation
func runTask(t *task) error {

	// skip if object (no script to execute)
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
			// Convert to []string for consistency, e.g., ["processor: 0", "vendor_id: GenuineIntel"]
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

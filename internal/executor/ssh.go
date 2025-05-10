package executor

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "log"

    "github.com/cenkalti/backoff/v4"
    "golang.org/x/crypto/ssh"
)

// Executor runs scripts remotely with resilience baked in.

type SSHExecutor struct {
    client *ResilientSSHClient
}

func NewSSHExecutor(client *ResilientSSHClient) *SSHExecutor {
    return &SSHExecutor{client: client}
}

func (e *SSHExecutor) Run(ctx context.Context, script string) ([]string, []string, error) {
    var outLines, errLines []string

    operation := func() error {
        // open session via circuit-breaker
        res, err := e.client.ResConf.CircuitBreaker.Execute(func() (any, error) {
            return e.client.SSHClient.NewSession()
        })
        if err != nil {
            return fmt.Errorf("new session: %w", err)
        }
        sess := res.(*ssh.Session)
        defer sess.Close()

        // pipes
        stdout, err := sess.StdoutPipe()
        if err != nil {
            return fmt.Errorf("stdout pipe: %w", err)
        }
        stderr, err := sess.StderrPipe()
        if err != nil {
            return fmt.Errorf("stderr pipe: %w", err)
        }

        // start + scan
        if err := sess.Start(script); err != nil {
            return fmt.Errorf("start script: %w", err)
        }
        outLines = scanLines(ctx, stdout)
        errLines = scanLines(ctx, stderr)

        return sess.Wait()
    }

    b := backoff.WithContext(e.client.ResConf.BackoffSettings, ctx)
    if err := backoff.Retry(operation, b); err != nil {
        return nil, nil, err
    }
    return outLines, errLines, nil
}

func scanLines(ctx context.Context, r io.Reader) []string {
    scanner := bufio.NewScanner(r)
    var lines []string
    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return lines
        default:
            lines = append(lines, scanner.Text())
        }
    }
    if err := scanner.Err(); err != nil {
        log.Printf("scan error: %v", err)
    }
    return lines
}

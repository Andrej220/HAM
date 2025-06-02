package executor

import (
    "context"
)

// Executor knows how to run a script over SSH (or any transport),
// apply retries/backoff, and return the output as string slices.
type Executor interface {
    Run(ctx context.Context, script string) (stdoutLines, stderrLines []string, err error)
}

// Task is responsible for taking a node + executor + processor chain,
// invoking the executor, and feeding the output into the processor.
type Task interface {
    Execute(ctx context.Context) error
}

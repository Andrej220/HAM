package executor

import (
    "context"
    pc "github.com/andrej220/HAM/internal/processor"
    gp "github.com/andrej220/HAM/internal/graphproc"
)

type NodeTask struct {
    Node *gp.Node
    Exec Executor
}

func NewNodeTask(node *gp.Node, exec Executor) *NodeTask {
    return &NodeTask{Node: node, Exec: exec}
}

func (t *NodeTask) Execute(ctx context.Context) error {
    // skip objects or empty scripts
    if t.Node.Type == "object" || len(t.Node.Script) == 0 {
        return nil
    }

    out, errOut, err := t.Exec.Run(ctx, t.Node.Script)
    if err != nil {
        return err
    }

    // post process
    chain := pc.NewProcessorChain()
    t.Node.Stderr = errOut
    t.Node.Result, _ = chain.Process(out, pc.NodeType(t.Node.Type), t.Node.PostProcess)
    return nil
}

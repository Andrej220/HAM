``` mermaid

%%{init: {"config": {"look": "handDrawn", "theme": "forest"}}}%%
classDiagram
    direction TB

    class Node {
        - ID: string
        - Type: string
        - Script: string
        - PostProcess: string
        - Children: []*Node
        - Result: []string
        - Stderr: []string
    }

    class HostConfig {
        - CustomerID: int
        - HostID: int
        - ScriptID: int
        - HostName: string
        - HostIP: string
        - HostPort: int
        - HostUser: string
        - HostPass: string
        - HostKey: string
        - HostType: string
    }

    class Config {
        - Version: string
        - RemoteHost: string
        - Password: string
        - Login: string
        - CustomerID: string
        - HostID: string
        - Structure: *Node
    }

    class Graph {
        - Config: *Config
        - HostCfg: *HostConfig
        - UUID: uuid.UUID
        - Root: *Node
        + NodeGenerator() chan *Node
        + ProcessNodes() error
    }

    Node <|-- alias
    Graph --> Config
    Graph --> HostConfig
    Graph --> Node
    Config --> Node
    Node --> Node : Children

```

``` mermaid
%%{init: {"config": {"look": "handDrawn", "theme": "forest"}}}%%
flowchart TD
    A[Load JSON File] --> B[Unmarshal into Config]
    B --> C[Create Graph]
    C --> D[Assign Root Node]
    D --> E[Start NodeGenerator]
    E --> F[Traverse DFS]
    F --> G[Emit Each Node to Channel]
    G --> H[ProcessNodes - goroutines]
    H --> I[Run goroutines with WaitGroup]
    I --> J[Processing complete]

```
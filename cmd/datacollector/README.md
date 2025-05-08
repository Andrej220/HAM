## Architecture Overview


### Class Diagram
The core components and their relationships:

``` mermaid
classDiagram
    direction TB

    class DataCollector {
        -pool: WorkerPool
        -cancelFuncs: sync.Map
        -httpClient: HttpClient
        +ServeHTTP()
    }

    class SSHJob {
        -HostID: int
        -ScriptID: int
        -UUID: UUID
        -Ctx: Context
    }

    class ResilientSSHClient {
        -SSHClient: SSH.Client
        -ResConf: ResilienceConfig
        +Close()
    }

    class Task {
        -node: GraphNode
        -client: SSH.Client
        -session: SSH.Session
        +Run()
    }

    class WorkerPool {
        -Jobs: Channel
        -activeWorkers: int
        +Submit()
        +Stop()
    }

    class GraphProcessor {
        -Config: Config
        -Root: Node
        +NodeGenerator()
    }

    class OutputProcessor {
        -processors: Map
        +Process()
    }

    %% Relationships
    DataCollector --> WorkerPool
    DataCollector --> SSHJob
    WorkerPool --> SSHJob
    SSHJob --> ResilientSSHClient
    SSHJob --> GraphProcessor
    SSHJob --> Task
    Task --> SSH.Client
    Task --> OutputProcessor
    GraphProcessor --> OutputProcessor
```


## Flowchart

``` mermaid
flowchart TD
    A[HTTP Request] --> B[DataCollector]
    B --> C[Create SSHJob]
    C --> D[WorkerPool]
    D --> E[RunJob]
    E --> F[Load Config]
    F --> G[Create SSH Client]
    G --> H[Process Nodes]
    H --> I[Execute Script]
    I --> J[Process Output]
    J --> K[Send to DataService]
    K --> L[MongoDB]

    subgraph DataCollector
        B
        C
        D
    end

    subgraph JobExecution
        E
        F
        G
        H
        I
        J
    end

    subgraph Dependencies
        F -.-> M[GraphProcessor]
        J -.-> N[OutputProcessor]
        G -.-> O[ResilientClient]
    end
```
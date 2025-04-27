``` mermaid
sequenceDiagram
    participant DataCollector
    participant DataService
    participant Postgres
    participant MongoDB
    
    DataCollector->>DataService: SaveConfig(REST)
    DataService->>Postgres: BEGIN TRANSACTION
    DataService->>Postgres: Store metadata
    DataService->>MongoDB: Store stdout/structured output
    DataService->>Postgres: COMMIT
    DataService-->>DataCollector: JobResponse{id, status}
```


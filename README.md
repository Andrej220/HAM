# HAM – Healthcare Asset Map

## Overview
HAM (Healthcare Asset Map) is a microservice-based system for collecting, processing, and storing configuration and operational data from medical equipment across distributed environments.  

Key ideas:
- **Message-driven** architecture with Kafka.  
- **Worker pool** concurrency in the DataCollector.  
- **PostgreSQL** for structured metadata and **MongoDB** for unstructured artifacts.  
- **Prometheus/Grafana** for observability.  

## Architecture Diagram
The diagram below shows the high-level architecture.  
- **API Gateway** is the entry point for users.  
- **DataCollectorProducer** accepts API requests and pushes them into the **Message Queue** (Kafka).  
- **DataCollector** consumes tasks, executes them on remote hosts via SSH, and returns results.  
- **DataService** stores results in **PostgreSQL** (metadata) and **MongoDB** (artifacts/output).  
- **MetadataService** manages metadata and interacts with PostgreSQL.  
- **Prometheus/Grafana** provide observability for all components.


```mermaid
graph TD
  %% Layout
  %%linkStyle default interpolate basis

  %% Clients / API
  subgraph API[API Layer]
    Gateway[API Gateway]
  end

  subgraph Clients[Clients]
    User
  end

  %% Services
  subgraph SVC[Microservices]
    Producer[DataCollectorProducer]
    DC[Data Collector]
    DS[Data Service]
    MS[Metadata Service]
  end

  %% Infra
  subgraph INFRA[Infrastructure]
    Q[(Message Queue)]
    Hosts[(Remote Hosts)]
    PG[(PostgreSQL)]
    MG[(MongoDB)]
  end

  %% Observability
  subgraph OBS[Observability]
    Mon[Prometheus / Grafana]
  end

  %% Flows
  User -->|REST| Gateway
  Gateway --> Producer
  Producer -->|enqueue| Q
  Q -->|consume| DC
  DC -->|SSH exec| Hosts
  DC -->|results| DS
  DS -->|metadata| MS
  MS --> PG
  DS --> MG
  %% MS -->|events| Q
  %% DS -->|subscribe| Q
  DS -->|read| Gateway

  %% Monitoring taps
  Gateway --> Mon
  Producer --> Mon
  DC --> Mon
  DS --> Mon
  MS --> Mon

```


## Data Flow
1. This sequence shows how a request moves through the system:
2. A user sends a request via the API Gateway.
3. DataCollectorProducer pushes the request into Kafka.
4. DataCollector consumes from Kafka, executes tasks on remote hosts, and forwards results to DataService.
5. DataService stores metadata in PostgreSQL and artifacts in MongoDB.
6. The final job response is returned back through the API Gateway.


```mermaid
sequenceDiagram
    participant APIGateway
    participant DataCollectorProducer
    participant Kafka
    participant DataCollector
    participant DataService
    participant Postgres
    participant MongoDB

    APIGateway->>DataCollectorProducer:REST request
    DataCollectorProducer->>Kafka:Push message in a queue
    Kafka->>DataCollector:Read the queue
    DataCollector->>DataService: Save(REST)
    DataService->>Postgres: BEGIN TRANSACTION
    DataService->>Postgres: Store metadata
    DataService->>MongoDB: Store stdout/structured output
    DataService->>Postgres: COMMIT
    DataService-->>DataCollector: JobResponse{id, status}
    DataCollector-->>APIGateway:job completed
```


## Worker Pool Concept

The DataCollector uses a worker pool pattern to handle multiple tasks concurrently.
When a task arrives, it is immediately enqueued in the worker pool, and DataCollector is free to accept the next task.
Workers run in parallel, execute jobs on remote hosts, return results to the DataCollector, which then persists them through DataService.


```mermaid

sequenceDiagram
    participant K as Kafka (tasks)
    participant DC as DataCollector
    participant WP as Worker Pool
    participant DS as DataService

    K->>DC: Task #1
    DC->>WP: Enqueue Job #1
    note over DC: Ready for next task
    K->>DC: Task #2
    DC->>WP: Enqueue Job #2

    par Workers in parallel
        WP-->>DC: Result #1
        DC->>DS: Save #1
    and
        WP-->>DC: Result #2
        DC->>DS: Save #2
    end

    DC-->>K: Commit after saves
```

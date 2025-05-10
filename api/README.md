API diagram

```mermaid
flowchart LR
  subgraph API
    A[API Gateway]
  end

  subgraph Services
    B[DataCollectionService]
    C[DataService]
    D[MetadataService]
  end

  subgraph Database
    MongoDB[(MongoDB)]
    PostgreSQL[(PostgreSQL)]
  end

  subgraph External
    RemoteDevice[Remote Device]
  end

  A -->|POST /datacollection| B
  A -->|GET /dataservice/:customerId/:deviceId| C
  A -->|GET /dataservice/:customerId/:deviceId/history| C
  A -->|DELETE /dataservice/:customerId/:deviceId/history| C
  A -->|DELETE /dataservice/:customerId/:deviceId/history/:configUUID| C
  A -->|GET /dataservice/uuid/:configUUID| C
  A -->|GET /dataservice-results/:customerId/:deviceId| C

  A -->|POST /customers| D
  A -->|GET /customers/:customerId| D
  A -->|PUT /customers/:customerId| D
  A -->|DELETE /customers/:customerId| D

  A -->|POST /devices| D
  A -->|GET /devices/:deviceId| D
  A -->|PUT /devices/:deviceId| D
  A -->|DELETE /devices/:deviceId| D
  A -->|POST /devices/:deviceId/scripts| D
  A -->|DELETE /devices/:deviceId/scripts/:scriptId| D

  A -->|POST /scripts| C
  A -->|GET /scripts| C
  A -->|GET /scripts/:scriptId| C
  A -->|PUT /scripts/:scriptId| C
  A -->|DELETE /scripts/:scriptId| C

  B -->|Triggers remote script execution| RemoteDevice
  RemoteDevice -->|Returns execution output| B

  B -->|Fetches linked scripts incl. createdBy| C
  B -->|Stores output to database| C

  C -->|Manages scripts, DataCollection, Archive, DataCollectionResult| MongoDB
  C -->|Requests metadata for validation| D

  D -->|Manages Customer, Device| PostgreSQL
  D -->|Validates script compatibility| C

```

 Resource Hierarchy Diagram

 ```mermaid
graph LR
  A[API Root]

  subgraph Data Collection
    B[datacollection]
  end

  subgraph Data Service
    C[dataservice]
    C --> G[dataservice/:customerId/:deviceId]
    G --> H[dataservice/:customerId/:deviceId/history]
    H --> I[dataservice/:customerId/:deviceId/history/:configUUID]
    C --> J[dataservice/uuid/:configUUID]
    C --> K[dataservice-results/:customerId/:deviceId]
  end

  subgraph Customers
    D[customers]
    D --> L[customers/:customerId]
  end

  subgraph Devices
    E[devices]
    E --> M[devices/:deviceId]
    M --> N[devices/:deviceId/scripts]
    N --> O[devices/:deviceId/scripts/:scriptId]
  end

  subgraph Scripts
    F[scripts]
    F --> P[scripts/:scriptId]
  end

  A --> B
  A --> C
  A --> D
  A --> E
  A --> F

```
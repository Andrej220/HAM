#HAM
project-root/
├── cmd/                    # Entry points for each microservice
│   ├── auth/               # Authentication Service
│   │   └── main.go
│   ├── customer/           # Customer Management Service
│   │   └── main.go
│   ├── device/             # Device Management Service
│   │   └── main.go
│   ├── script/             # Script Management Service
│   │   └── main.go
│   ├── execution/          # Script Execution Service
│   │   └── main.go
│   └── config/             # Configuration Service
│       └── main.go
├── internal/               # Shared code across services
│   ├── db/                 # Database connectors (PostgreSQL, MongoDB)
│   │   ├── postgres.go
│   │   └── mongo.go
│   ├── ssh/                # SSH client for script execution
│   │   └── client.go
│   ├── auth/               # JWT utilities
│   │   └── jwt.go
│   └── models/             # Shared data models (e.g., Device, Customer)
│       ├── customer.go
│       ├── device.go
│       └── script.go
├── pkg/                    # Optional: Public packages (if you extract reusable code later)
├── api/                    # API definitions (e.g., OpenAPI specs, if used)
│   ├── auth.yaml
│   └── device.yaml
├── deploy/                 # k3s deployment files
│   ├── auth.yaml           # Kubernetes manifests
│   ├── customer.yaml
│   └── ...
├── go.mod                  # Single Go module file
├── go.sum                  # Dependency checksums
└── README.md               # Project overview

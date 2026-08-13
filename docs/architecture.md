# System Architecture & Codebase Design

This document details the high-level architecture, module organization, directory structure, CQRS design patterns, and dependency injection model used across the **Master Go CQRS** repository.

---

## 1. Monorepo Organization

The project is structured as a single Go module (`github/master_go_cqrs`) containing decoupled microservices, shared contract definitions, reusable infrastructure packages, and deployment configurations.

```text
.
├── cmd/                        # Application entry points (main package binaries per service)
│   ├── affiliate/
│   ├── agent/
│   ├── orchestrator/
│   ├── payment/
│   ├── settlement/
│   └── user/
├── contracts/                  # Shared cross-service definitions
│   ├── genproto/               # Generated gRPC code (protoc output)
│   ├── messagebus/             # Shared Kafka event structs and constants
│   └── proto/                  # Protobuf contract definitions (.proto)
├── dockers/                    # Envoy, Docker, and Kubernetes deployment manifests
│   ├── envoy/                  # Envoy configuration and profile templates
│   ├── docker-compose.yaml     # k3d fallback runner
│   └── k3d/                    # Local Kubernetes manifests (Istio, Postgres, Kafka, Keycloak, etc.)
├── docs/                       # Project documentation and API specifications
├── internal/                   # Shared platform & framework packages
│   └── platform/               # EventStore, Kafka, Outbox/Inbox, Postgres, Auth, Telemetry
├── services/                   # Independent microservice domains
│   ├── affiliate/
│   ├── agent/
│   ├── orchestrator/
│   ├── payment/
│   ├── settlement/
│   └── user/
├── tasks/                      # Taskfile task modules (k3d, dev, gateway, docs, etc.)
└── Taskfile.yml                # Main task execution runner
```

---

## 2. Microservice Inner Architecture (Clean Architecture + CQRS)

Each service under `services/<service>` is an autonomous domain boundary. Access to service internal packages is enforced by Go's `internal` keyword rules.

```text
services/<service>/
├── module.go                   # Uber Fx module definition and wiring facade
├── config/                     # Service-specific YAML config files
│   └── <service>.yaml
├── migrations/                 # Independent PostgreSQL migration scripts
└── internal/
    ├── domain/                 # Core entities, value objects, and domain errors
    ├── application/            # Business use cases
    │   ├── command/            # CQRS State-modifying command handlers & transactional UoW
    │   ├── query/              # CQRS Read-only query handlers
    │   ├── dto/                # Request and Response Data Transfer Objects
    │   └── port/               # Repository and Gateway interfaces owned by application layer
    ├── transport/              # Delivery mechanism adapters
    │   ├── http/               # Echo HTTP REST route handlers
    │   ├── grpc/               # gRPC service implementation adapters
    │   └── consumer/           # Kafka event consumer subscription handlers
    └── adapter/                # Implementation of application ports
        ├── persistence/        # GORM / PostgreSQL repository and unit of work implementations
        └── gateway/            # External service client adapters & simulators
```

### Architectural Layering Rules:
1. **Domain (`domain/`)**: Pure domain logic. Standard Go structs, custom domain types, and domain error constants. Contains zero dependencies on infrastructure frameworks (no SQL, HTTP, or gRPC tags).
2. **Application (`application/`)**: Implements application use cases split into **Commands** and **Queries**.
   - **`command/`**: Contains write handlers. Mutates aggregate state, utilizes Unit of Work (`port.UnitOfWork`), saves entities, and records transactional Outbox events.
   - **`query/`**: Contains read handlers. Direct read-optimized repository calls without side-effects.
   - **`port/`**: Interfaces defining contract requirements for persistence repositories, outbox messaging, and external integrations.
3. **Adapter (`adapter/`)**: Fulfills interfaces defined in `application/port`. Includes GORM/PostgreSQL persistence repositories, SQL outbox writers, and payment simulator gateways.
4. **Transport (`transport/`)**: Converts external inputs (HTTP JSON requests, gRPC Proto calls, Kafka messages) into Application DTOs, executes handlers, and converts output results back to HTTP/gRPC/Kafka responses.

---

## 3. Dependency Injection with Uber Fx

Services utilize [Uber Fx](https://github.com/uber-go/fx) for dependency injection, lifecycle management, and clean component isolation.

### Service Facade (`module.go`)
Each service exposes a top-level `module.go` containing two primary module blocks:
- **`Module`**: Registers service-specific dependencies (repositories, command handlers, query handlers, HTTP routes, gRPC servers).
- **`RuntimeModule`**: Combines service `Module` with shared platform modules (Config, Logger, OpenTelemetry, Postgres, Outbox, Kafka Producer, Gateway Auth).

```go
// Example Fx Module snippet from services/payment/module.go
var Module = fx.Module("payment",
    fx.Provide(
        paymentpostgres.NewTransactionRepository,
        paymentpostgres.NewUnitOfWork,
        simulator.NewSimulatorGateway,

        command.NewInitiatePaymentHandler,
        command.NewProcessPaymentHandler,

        query.NewGetTransactionHandler,
        query.NewGetTransactionHistoryHandler,

        route.AsRegistrar(http.NewHandler),
        grpc.NewServerHandler,
    ),
    fx.Invoke(grpc.RequireServerHandler),
)
```

### Main Application Entry Point (`cmd/<service>/main.go`)
The service entry point initializes the Fx application, loading configuration overrides from environment variables:

```go
func main() {
    fx.New(payment.Options()).Run()
}
```

---

## 4. Database-per-Service Isolation

To maintain strict service decoupling, **each service owns its own database schema**. Cross-database queries or direct SQL joins between services are strictly prohibited.

| Service | Database Name | Default DB User | Primary Owned Tables |
| --- | --- | --- | --- |
| `user` | `master_go_cqrs_user` | `master_go_cqrs_user_user` | `user_identities` |
| `payment` | `master_go_cqrs_payment` | `master_go_cqrs_payment_user` | `transactions`, `outbox_events`, `domain_events` |
| `affiliate` | `master_go_cqrs_affiliate` | `master_go_cqrs_affiliate_user` | `affiliate_volumes`, `inbox_events`, `outbox_events` |
| `settlement` | `master_go_cqrs_settlement` | `master_go_cqrs_settlement_user` | `agent_balances`, `inbox_events`, `outbox_events` |
| `agent` | `master_go_cqrs_agent` | `master_go_cqrs_agent_user` | `agent_callbacks`, `inbox_events`, `outbox_events` |
| `orchestrator` | `master_go_cqrs_orchestrator` | `master_go_cqrs_orchestrator_user` | `payment_sagas`, `inbox_events`, `outbox_events` |

Data synchronization between services occurs exclusively via asynchronous Kafka events or gRPC RPC calls.

---

## 5. Gateway & Network Ingress Architecture

```text
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │ Client Request (Browser / App / API Client)                                 │
 └──────────────────────────────────────┬──────────────────────────────────────┘
                                        │ HTTP / JSON (:8888)
                                        ▼
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │ Istio Ingress Gateway / Envoy API Gateway                                   │
 │ - Public TLS termination & CORS                                             │
 │ - Keycloak JWT Authentication Verification (`jwt_authn`)                    │
 │ - External Authorization Policy Enforcement (`ext_authz`)                  │
 │ - gRPC-JSON Transcoding (using dockers/envoy/proto.pb descriptor)           │
 └──────────┬───────────────────────────┬───────────────────────────┬──────────┘
            │                           │                           │
            │ HTTP (8889)               │ gRPC (9081)               │ gRPC (9082/9083/9084)
            ▼                           ▼                           ▼
 ┌──────────────────────┐    ┌──────────────────────┐    ┌──────────────────────┐
 │ User Service         │    │ Payment Service      │    │ Affiliate /          │
 │ - Keycloak Auth      │    │ - Payment Process    │    │ Settlement / Agent   │
 │ - Profile Management │    │ - Outbox Relay       │    │ Services             │
 └──────────────────────┘    └──────────┬───────────┘    └──────────────────────┘
                                        │
                                        ▼ Kafka Event Topics
                             ┌──────────────────────┐
                             │ Orchestrator Service │
                             │ - Saga Engine        │
                             │ - Temporal Engine    │
                             └──────────────────────┘
```

- **Envoy API Gateway**: Exposed at port `8888`. Performs JWT validation via Keycloak JWKS, transforms incoming REST/JSON requests into internal gRPC calls, and forwards authenticated claims via `x-gateway-jwt-payload`.
- **Telepresence Integration**: For local development (`task dev-run`), Telepresence intercepts cluster traffic and redirects Envoy/gRPC requests to local processes while routing outbound host connections to Kubernetes in-cluster PostgreSQL, Kafka, and Keycloak services.

# Master Go CQRS

Go microservices learning project organized around CQRS, Echo HTTP APIs, gRPC services, Fx dependency injection, GORM/PostgreSQL persistence, Temporal workflows, an outbox publisher, Envoy API Gateway, Docker, and Kubernetes.

## Architecture

The project is split by feature module under `internal/`:

- `payment`
- `affiliate`
- `agent`
- `settlement`
- `authentication`
- `accesscontrol`
- `orchestrator`

The service modules use this internal CQRS layout:

```text
internal/<feature>/
  application/
    command/    # state-changing handlers, transactions, side effects
    query/      # read-only handlers
    dto/        # command/query request DTOs
  delivery/
    http/       # Echo route adapters
    grpc/       # gRPC service adapters
  domain/       # entities, value types, repository/gateway ports
  repository/   # persistence adapters
```

Commands mutate state and own transactional work such as saving aggregates, writing audit logs, crediting commissions, charging payment gateways, and writing outbox events. Queries are read-only and delegate to repository ports without side effects.

## Prerequisites

- Go `1.24.4`
- Task
- Docker
- Docker Compose
- PostgreSQL
- Optional: `kubectl`
- Optional local Kubernetes runtime: Docker Desktop Kubernetes, Kind, Minikube, or K3s

## Install and Setup

Install Go dependencies:

```bash
go mod download
```

Each service has its own config file under `config/`. The app reads configuration from `CONFIG_PATH`. If `CONFIG_PATH` is not set, it checks `config/<SERVICE_NAME>.yaml`, then falls back to `config.yaml` and `config.yml` from the current working directory.

```bash
SERVICE_NAME=payment CONFIG_PATH=./config/payment.yaml go run ./cmd/payment
```

Before running services, start the Docker dependencies from `dockers/docker-compose.yaml` or verify your local PostgreSQL and Temporal instances match the target service config values.

## Configuration

The main config sections are:

- `postgres`: database host, port, user, password, and database name.
- `logger`: log level, runtime mode, and encoding.
- `server`: HTTP server port.
- `keycloak`: endpoint, client credentials, public key, auth header, and realm.
- `gateway`: public API gateway values used by services, such as generated payment checkout links.
- `otel`: OpenTelemetry OTLP HTTP endpoint and service name. Local configs send traces to the OpenTelemetry Collector at `localhost:4318`; the collector forwards them to Jaeger. Override with `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, or `OTEL_SERVICE_NAME` when needed.

## Run Locally

Run the app service:

```bash
task run SERVICE=app
```

Run dependencies, migrations, and all services with per-service logs:

```bash
task dev-up
```

After `task dev-up` completes, use these local URLs:

| Surface | URL | What it shows |
| --- | --- | --- |
| Envoy API gateway | http://localhost:8888 | Public HTTP/JSON gateway. `/` redirects to Swagger UI. |
| Gateway health through Envoy | http://localhost:8888/api/v1/health | Health check routed through Envoy to the app service. |
| App service direct health | http://localhost:8889/api/v1/health | Direct app service endpoint, bypassing Envoy. |
| Swagger UI for proto/gateway APIs | http://localhost:8888/api/v1/swagger/ | Swagger UI served from `docs/envoy.swagger.json`, generated from `proto/*.proto`. |
| Raw proto/gateway OpenAPI JSON | http://localhost:8888/api/v1/swagger/swagger.json | The same gateway OpenAPI document used by Swagger UI. |
| Keycloak admin console | http://localhost:8080/admin | Local Keycloak admin UI. Default local admin credentials are `admin` / `admin`. |
| Keycloak backend realm discovery | http://localhost:8080/realms/backend/.well-known/openid-configuration | OIDC discovery document for the `backend` realm. |
| OpenTelemetry collector health | http://localhost:13133 | Local collector readiness endpoint. Apps export OTLP HTTP traces to `localhost:4318`. |
| Jaeger tracing UI | http://localhost:16686 | Trace viewer for spans forwarded by the OpenTelemetry Collector. |
| Envoy admin | http://localhost:9901 | Envoy admin interface for local gateway diagnostics. |

The app-service Swagger docs generated from Go handler annotations are written to `docs/swagger.json` and `docs/swagger.yaml` by `task gen-swagger`; they are generated files, not a runtime URL started by `task dev-up`.

Check status and follow logs:

```bash
task dev-status
task dev-logs SERVICES=app
task infra-logs INFRA_SERVICE=keycloak
task logs-ui
```

`task logs-ui` opens Dozzle at http://localhost:9999 and forwards `.dev/logs/*.log`
into Docker log containers named `master_go_cqrs_log_<service>`.
Run `task logs-ui` after `task dev-up` when you want the browser log UI.

Run individual microservices:

```bash
task run-app
task run-payment
task run-affiliate
task run-agent
task run-settlement
task run-orchestrator
```

HTTP APIs are registered under `/api/v1`.

### Envoy API Gateway

Envoy is the public HTTP/JSON gateway for service gRPC APIs:

```text
client -> HTTP/JSON -> Envoy :8888 -> gRPC -> payment/affiliate/settlement/agent
```

Gateway routes are generated from `google.api.http` proto annotations. The local gateway uses Envoy's gRPC-JSON transcoder and the generated descriptor at `dockers/envoy/proto.pb`; Swagger/OpenAPI is generated at `docs/envoy.swagger.json`.

```bash
task gen-proto
task gateway-up
task gateway-validate
```

The app service Swagger docs are generated from Go handler annotations and DTO schemas under `internal/*/application/dto`:

```bash
task gen-swagger
task gen-docs
```

`task dev-up` starts infra, migrations, app services, Swagger UI, the OpenTelemetry Collector, Jaeger, and Envoy. Envoy is reachable at http://localhost:8888, Swagger UI is reachable at http://localhost:8888/api/v1/swagger/, and Jaeger is reachable at http://localhost:16686.

Keycloak is the identity provider for login, refresh, users, roles, and client credentials. Clients can obtain tokens from the realm token endpoint:

```text
http://localhost:8080/realms/backend/protocol/openid-connect/token
```

Envoy is the public authentication and route-authorization boundary. Its `jwt_authn` filter verifies bearer tokens offline from the Keycloak JWKS endpoint, caches the keys, forwards verified user claims in `x-gateway-jwt-payload`, and then calls the app service `ext_authz` check for protected routes:

```text
http://localhost:8080/realms/backend/protocol/openid-connect/certs
```

Payment checkout routes are public. Payment initiation, affiliate, settlement, agent, `/api/v1/auth/info`, `/api/v1/auth/verify-token`, and `/api/v1/author/*` require a verified Keycloak bearer token through Envoy. Services trust only Envoy-forwarded verified claims for request identity and keep business authorization checks in service code.

Taskfile wraps local commands:

```bash
task run SERVICE=app
task run-payment
task build SERVICE=payment
task build-all
```

## Test

Run all tests:

```bash
go test ./...
```

Run focused service checks:

```bash
go test ./internal/payment/...
go test ./internal/affiliate/...
go test ./internal/agent/...
go test ./internal/settlement/...
```

## Docker

Build service images from the root Dockerfile:

```bash
docker build --target payment -t payment-service:latest .
docker build --target affiliate -t affiliate-service:latest .
docker build --target settlement -t settlement-service:latest .
docker build --target agent -t agent-service:latest .
docker build --target orchestrator -t orchestrator-service:latest .
docker build --target app -t app-service:latest .
```

Or use Task targets:

```bash
task image SERVICE=app
task image-all IMAGE_TAG=local
```

## Kubernetes Deployment

Deployment manifests and the local Kubernetes guide live in `deploy/`.

See [deploy/README.md](deploy/README.md) for K3s, Kustomize, Temporal, and service access instructions.

## CQRS Development Guide

When adding a new operation:

- Put write behavior in `internal/<feature>/application/command`.
- Put read-only behavior in `internal/<feature>/application/query`.
- Put shared command/query input structs in `internal/<feature>/application/dto`.
- Keep business entities and repository interfaces in `domain`.
- Keep database access in `repository/postgres`.
- Keep HTTP and gRPC packages as transport adapters that translate requests into command/query DTOs.
- Register new handlers in the feature `fx.go`.

Do not add read-model tables, projections, generated proto changes, or migrations unless the feature explicitly requires them.

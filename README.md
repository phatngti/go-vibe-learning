# Master Go CQRS

Go microservices learning project organized around CQRS, Echo HTTP APIs, gRPC services, Fx dependency injection, GORM/PostgreSQL persistence, Kafka-backed Saga or Temporal orchestration, transactional inbox/outbox messaging, Envoy API Gateway, Docker, and Kubernetes.

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
- Docker Compose, used only as a Dockerized k3d CLI fallback
- `kubectl`
- `curl` and `tar`, used to install pinned local development tools when needed
- Optional: host `k3d` CLI. If it is missing, Taskfile runs k3d through `dockers/docker-compose.yaml`.

## Install and Setup

Install Go dependencies:

```bash
go mod download
```

Each service has its own config file under `config/`. The app reads configuration from `CONFIG_PATH`. If `CONFIG_PATH` is not set, it uses `config/<SERVICE_NAME>.yaml`; an empty `SERVICE_NAME` selects `config/app.yaml`.

```bash
SERVICE_NAME=payment CONFIG_PATH=./config/payment.yaml go run ./cmd/payment
```

For the local integrated runtime, use `task dev-up`. Docker Compose no longer runs the service stack directly; it only provides a Dockerized k3d CLI helper when the host `k3d` binary is not installed.
The Taskfile caches pinned `istioctl`, Telepresence, and Air binaries under `.dev/tools`.

## Configuration

The main config sections are:

- `postgres`: database host, port, user, password, and database name.
- `logger`: log level, runtime mode, and encoding.
- `server`: HTTP server port.
- `keycloak`: endpoint, client credentials, public key, auth header, and realm.
- `gateway`: public API gateway values used by services, such as generated payment checkout links.
- `otel`: OpenTelemetry OTLP HTTP endpoint and service name. Local configs send traces to the OpenTelemetry Collector at `localhost:4318`; the collector forwards them to Jaeger. Override with `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, or `OTEL_SERVICE_NAME` when needed.
- `orchestrator`: selects the process-wide payment orchestration engine. `saga` is the default; `temporal` uses the same Kafka contracts and shared payment process definition. Override with `ORCHESTRATOR_ENGINE`, `TEMPORAL_ADDRESS`, `TEMPORAL_NAMESPACE`, and `TEMPORAL_TASK_QUEUE`.

The payment process is defined once under `internal/orchestrator/features/payment`. Persistence-oriented Saga and Temporal workflow implementations are runtime adapters around that definition; feature logic does not inspect the selected engine.

## Run Locally

Run the app service:

```bash
task run SERVICE=app
```

Deploy the Docker-backed k3d cluster, Istio, infrastructure, migrations, and app services:

```bash
task dev-up
```

`task dev-up` keeps the existing Kubernetes deployment workflow, then installs the Telepresence traffic manager and connects the host to the `master-go-cqrs` namespace. It returns after confirming that Postgres, Kafka, Keycloak, and OTel are reachable through cluster DNS. Telepresence may request elevated permission when starting its local networking daemon.

`task dev-up` also makes internal ClusterIP services reachable from the host through Telepresence DNS and routes. Use this to check and print the browser UI URLs:

```bash
task dev-ui
```

No `kubectl port-forward` is needed for these UI services. Keep Telepresence connected with `task dev-connect`; stop only the host bridge with `task dev-disconnect`.

If Chrome shows `ERR_NAME_NOT_RESOLVED` for `*.svc.cluster.local`, update `/etc/hosts` from the current k3d service ClusterIPs:

```bash
task dev-hosts
```

This manages only the `master_go_cqrs k3d service hosts` block in `/etc/hosts`. It is useful when Chrome Secure DNS, a proxy, or an extension bypasses the Telepresence DNS resolver.

Run one service locally with Air hot reload while its dependencies continue running in k3d:

```bash
task dev-run SERVICE=payment
```

The local process uses its existing `config/<service>.yaml`, with Postgres, Kafka, Keycloak, and OTel endpoints overridden to their Kubernetes service names. `task dev-run` also installs or upgrades Telepresence, connects the host to the k3d namespace, and verifies dependency access before Air starts. This does not intercept or replace the Kubernetes workload, so Envoy continues routing to the in-cluster service. Run different local services in separate terminals when needed.

After `task dev-up` completes, use these local URLs:

| Surface | URL | What it shows |
| --- | --- | --- |
| Envoy API gateway through Istio | http://localhost:8888 | Public HTTP/JSON gateway exposed by the k3d load balancer. `/` redirects to Swagger UI. |
| Gateway health through Envoy | http://localhost:8888/api/v1/health | Health check routed through Envoy to the user service. |
| Swagger UI for proto/gateway APIs | http://localhost:8888/api/v1/swagger/ | Swagger UI served from `docs/envoy.swagger.json`, generated from `proto/*.proto`. |
| Raw proto/gateway OpenAPI JSON | http://localhost:8888/api/v1/swagger/swagger.json | The same gateway OpenAPI document used by Swagger UI. |
| Keycloak admin console | http://keycloak.master-go-cqrs.svc.cluster.local:8080/admin | Reached through Telepresence service DNS. Default local admin credentials are `admin` / `admin`. |
| Keycloak backend realm discovery | http://keycloak.master-go-cqrs.svc.cluster.local:8080/realms/backend/.well-known/openid-configuration | Reached through Telepresence service DNS. |
| Jaeger tracing UI | http://jaeger.master-go-cqrs.svc.cluster.local:16686 | Reached through Telepresence service DNS. |
| Temporal UI | http://temporal-ui.master-go-cqrs.svc.cluster.local:8080 | Optional. Run `task k3d-temporal-up` first, then access through Telepresence service DNS. |

Connection mechanisms:

- `localhost:8888` is a k3d host-port mapping to the Istio ingress gateway. It is the only browser surface that `task dev-up` exposes directly.
- Telepresence from `task dev-up` lets local processes and browsers resolve and dial Kubernetes service DNS names such as `keycloak.master-go-cqrs.svc.cluster.local:8080`, `postgres.master-go-cqrs.svc.cluster.local:5432`, and `kafka.master-go-cqrs.svc.cluster.local:9092`.
- `/etc/hosts` is normally not needed because Telepresence owns DNS for the mapped namespace. If Chrome bypasses system DNS or uses an HTTP proxy, run `task dev-hosts`, bypass the proxy for `*.svc.cluster.local`, or use `--noproxy '*'` with `curl`.

The app-service Swagger docs generated from Go handler annotations are written to `docs/swagger.json` and `docs/swagger.yaml` by `task gen-swagger`; they are generated files, not a runtime URL started by `task dev-up`.

Check status and follow logs:

```bash
task dev-status
task dev-connect
task dev-disconnect
task dev-logs SERVICES=app
task infra-logs INFRA_SERVICE=keycloak
```

Switch the orchestrator only after the current engine is idle:

```bash
task orchestrator-mode MODE=temporal
task orchestrator-mode MODE=saga
```

The switch is restart-based. The task refuses to proceed while Saga records, outbox messages, or Temporal payment workflows are active. Temporal Server `1.31.1` and UI `2.51.0` are optional and are deployed only when requested.

Run individual microservices:

```bash
task run SERVICE=payment
task run SERVICE=orchestrator
```

`task run` remains the standalone local command using the endpoints in the YAML config. Use `task dev-run` when the process should connect to the k3d environment and reload after code changes.

HTTP APIs are registered under `/api/v1`.

### Envoy API Gateway

Envoy is the public HTTP/JSON gateway for service gRPC APIs:

```text
client -> HTTP/JSON -> Envoy :8888 -> gRPC -> payment/affiliate/settlement/agent
```

Gateway routes are generated from `google.api.http` proto annotations. The local gateway uses Envoy's gRPC-JSON transcoder and the generated descriptor at `dockers/envoy/proto.pb`; Swagger/OpenAPI is generated at `docs/envoy.swagger.json`.

Envoy separates shared gateway behavior from environment-specific values:

- Shared route policy: `dockers/envoy/shared/routes.yaml`
- Local k3d profile: `dockers/envoy/profiles/local/envoy.env`
- Production profile contract: `dockers/envoy/profiles/prod/envoy.env`

Local tasks default to the `local` profile. Render production config explicitly with `ENVOY_PROFILE=prod task gen-envoy-config` after overriding the production profile values from deployment secrets/config.

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

`task dev-up` starts the Docker-backed k3d cluster, installs Istio, deploys infra, migrations, app services, Swagger UI, the OpenTelemetry Collector, Jaeger, and Envoy. Envoy is reachable through Istio at http://localhost:8888, and Swagger UI is reachable at http://localhost:8888/api/v1/swagger/. Internal UIs are reachable through Telepresence service DNS; run `task dev-ui` to verify and print their URLs.

Keycloak is the identity provider for login, refresh, users, roles, and client credentials. After `task dev-up` or `task dev-connect`, clients can obtain tokens from the realm token endpoint:

```text
http://keycloak.master-go-cqrs.svc.cluster.local:8080/realms/backend/protocol/openid-connect/token
```

Envoy is the public authentication and route-authorization boundary. Its `jwt_authn` filter verifies bearer tokens offline from the Keycloak JWKS endpoint, caches the keys, forwards verified user claims in `x-gateway-jwt-payload`, and then calls the app service `ext_authz` check for protected routes:

```text
http://keycloak.master-go-cqrs.svc.cluster.local:8080/realms/backend/protocol/openid-connect/certs
```

Payment checkout routes are public. Payment initiation, affiliate, settlement, agent, `/api/v1/auth/info`, `/api/v1/auth/verify-token`, and `/api/v1/author/*` require a verified Keycloak bearer token through Envoy. Services trust only Envoy-forwarded verified claims for request identity and keep business authorization checks in service code.

Taskfile wraps local commands. The root `Taskfile.yml` keeps shared vars and flattened includes; grouped task definitions live under `tasks/*.yml`.

```bash
task run SERVICE=app
task run SERVICE=payment
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

## Docker and k3d

Build service images from the root Dockerfile:

```bash
docker build --target payment -t payment-service:latest .
docker build --target affiliate -t affiliate-service:latest .
docker build --target settlement -t settlement-service:latest .
docker build --target agent -t agent-service:latest .
docker build --target orchestrator -t master-go-cqrs/orchestrator:local .
docker build --target app -t app-service:latest .
```

Or use Task targets:

```bash
task image SERVICE=app
task image-all IMAGE_TAG=local
```

`dockers/docker-compose.yaml` does not run application or infrastructure services. It contains only a `k3d` helper service that mounts the host Docker socket, so Taskfile can initialize the k3d cluster in Docker even when the host does not have a `k3d` binary installed.
`task dev-up` removes obsolete local Compose containers named `master_go_cqrs_*` before creating k3d so old gateway or infra containers cannot keep ports like `8888` allocated.
It also configures Telepresence's route controller for the local k3d cluster. `task k3d-cluster-delete` disconnects Telepresence before removing the cluster.

## K3d and Istio Deployment

Local Kubernetes assets live under `dockers/k3d/`. The k3d cluster runs K3s in Docker, disables K3s Traefik so Istio owns ingress, and the `master-go-cqrs` namespace is labeled for automatic sidecar injection.

```bash
task k3d-deploy
```

The deployment builds local service images, imports them into k3d, installs Istio, applies infra and app manifests, runs SQL migrations in the Postgres pod, and exposes the Envoy API gateway through Istio at http://localhost:8888.

Useful k3d tasks:

```bash
task k3d-status
task k3d-logs SERVICE=app
task dev-ui
task dev-hosts
task k3d-kafka-url
task k3d-cluster-delete
```

See [dockers/k3d/README.md](dockers/k3d/README.md) for the local cluster workflow and manifest layout.

## Local Feature Development with Telepresence

This section walks through developing a feature on your local machine while connecting to the infrastructure (Postgres, Kafka, Keycloak, OTel/Jaeger) running inside the k3d Kubernetes cluster. Telepresence bridges your host network into the cluster so local processes can reach services by their Kubernetes DNS names without port-forwarding.

### How It Works

```text
┌──────────────────────────────────────────────────────────────────────┐
│  Your machine (macOS / Linux)                                        │
│                                                                      │
│  ┌──────────────┐    Telepresence DNS + routes    ┌───────────────┐  │
│  │ Local Go      │  ─────────────────────────────► │ k3d cluster   │  │
│  │ process (Air) │                                 │               │  │
│  │               │  postgres.master-go-cqrs...     │  Postgres     │  │
│  │ config/       │  kafka.master-go-cqrs...        │  Kafka        │  │
│  │  payment.yaml │  keycloak.master-go-cqrs...     │  Keycloak     │  │
│  │               │  otel.master-go-cqrs...         │  OTel/Jaeger  │  │
│  └──────────────┘                                  └───────────────┘  │
│                                                                      │
│  Envoy (in-cluster) still routes external traffic to the in-cluster  │
│  deployment. The local process is for development only.              │
└──────────────────────────────────────────────────────────────────────┘
```

Service config files under `config/` already use Kubernetes DNS names (e.g. `postgres.master-go-cqrs.svc.cluster.local`). Telepresence makes those names resolvable from the host, so the local process connects to the same infrastructure as the in-cluster pods.

### Step 1 — Bootstrap the Cluster

Start the full k3d environment, including Telepresence connectivity:

```bash
task dev-up
```

This performs the following in order:

1. Creates the k3d cluster (K3s in Docker) with Istio.
2. Builds and imports all service images.
3. Deploys infrastructure (Postgres, Kafka, Keycloak, OTel, Jaeger).
4. Runs SQL migrations.
5. Deploys all application services and the Envoy gateway.
6. Installs the Telepresence traffic manager.
7. Connects your host to the `master-go-cqrs` namespace.
8. Verifies that Postgres, Kafka, Keycloak, and OTel are reachable through Telepresence DNS.

Telepresence may prompt for your macOS password (via `sudo`) to start its local network daemon. This is a one-time prompt per session.

After `task dev-up` completes, verify the environment:

```bash
task dev-status     # k3d pods, Istio, and Telepresence state
task dev-ui         # browser UI URLs through Telepresence DNS
```

### Step 2 — Run Your Service Locally

Pick the service you are developing and run it locally with Air hot reload:

```bash
task dev-run SERVICE=payment
```

Supported services: `user`, `payment`, `affiliate`, `settlement`, `agent`, `orchestrator`.

What happens under the hood:

1. Generates an Air config at `.dev/air/<service>/air.toml` that watches `cmd/<service>/`, `internal/`, `go.mod`, `go.sum`, and `config/<service>.yaml`.
2. Installs or verifies the pinned Air binary (`.dev/tools/air`).
3. Ensures Telepresence is connected (installs traffic manager if needed).
4. Runs a TCP smoke test against Postgres, Kafka, Keycloak, and OTel through cluster DNS.
5. Starts Air, which builds and runs `./cmd/<service>` with `SERVICE_NAME` and `CONFIG_PATH` set.

The local process reads `config/<service>.yaml` directly. Because Telepresence DNS is active, all endpoints in that config resolve to the k3d infrastructure:

```yaml
# config/payment.yaml — these all resolve through Telepresence
postgres:
  host: "postgres.master-go-cqrs.svc.cluster.local"
  port: "5432"
keycloak:
  endpoint: "http://keycloak.master-go-cqrs.svc.cluster.local:8080"
kafka:
  brokers:
    - "kafka.master-go-cqrs.svc.cluster.local:9092"
otel:
  endpoint: "otel.master-go-cqrs.svc.cluster.local:4318"
```

Save any Go file in `cmd/<service>/` or `internal/` and Air will automatically rebuild and restart the process.

> **Note:** `task dev-run` does **not** intercept or replace the in-cluster deployment. Envoy continues routing external HTTP traffic to the Kubernetes pod. The local process is for development and testing only. To test through the full Envoy gateway, use the in-cluster deployment or call the local process directly on its configured server port.

### Step 3 — Develop and Test Your Feature

With the local service running and connected to k3d infrastructure, you can:

**Call the local HTTP API directly:**

```bash
# The payment service listens on port 8081 (from config/payment.yaml → server.port)
curl http://localhost:8081/api/v1/health
```

**Call the Envoy gateway (routes to in-cluster pods):**

```bash
curl http://localhost:8888/api/v1/health
```

**Obtain a Keycloak token (through Telepresence DNS):**

```bash
curl -X POST \
  http://keycloak.master-go-cqrs.svc.cluster.local:8080/realms/backend/protocol/openid-connect/token \
  -d "grant_type=client_credentials" \
  -d "client_id=master-go-cqrs" \
  -d "client_secret=d718f4676030aa7bc997227b38b13831ae2bbfea464bf276f97fdc602c82a417"
```

**Connect to Postgres for debugging:**

```bash
psql -h postgres.master-go-cqrs.svc.cluster.local \
     -U master_go_cqrs_payment_user \
     -d master_go_cqrs_payment
# password: master_go_cqrs_payment_password
```

**View traces in Jaeger:**

Open http://jaeger.master-go-cqrs.svc.cluster.local:16686 in your browser. If Chrome cannot resolve the hostname, run `task dev-hosts` to add `/etc/hosts` entries.

### Step 4 — Multi-Service Development

Run different services in separate terminals. Each terminal gets its own Air watcher:

```bash
# Terminal 1
task dev-run SERVICE=payment

# Terminal 2
task dev-run SERVICE=affiliate

# Terminal 3
task dev-run SERVICE=orchestrator
```

Each local process connects to the shared k3d Postgres, Kafka, Keycloak, and OTel. Services that remain in k3d (not running locally) continue to serve their Kubernetes deployments. Kafka consumers and producers work across the local/cluster boundary through Telepresence.

### Step 5 — Run Migrations

If your feature adds new database tables or columns:

1. Create migration files under `migrations/<service>/`:

```bash
task migration-create SERVICE=payment NAME=add_refund_table
```

2. Apply migrations to the k3d Postgres:

```bash
task k3d-migrate
```

This runs all `*.up.sql` files for every service against the Postgres pod.

### Step 6 — Rebuild and Redeploy to k3d

When you are ready to test your changes inside the cluster (e.g. through Envoy routing):

```bash
# Rebuild images, import into k3d, and restart deployments
task dev-start SERVICES=payment

# Or rebuild everything
task dev-start SERVICES=all
```

### Managing the Telepresence Connection

```bash
# Check Telepresence and k3d status
task dev-status

# Reconnect to the cluster (if you rebooted or disconnected)
task dev-connect

# Disconnect Telepresence without stopping k3d
task dev-disconnect

# Tear down everything (Telepresence + k3d cluster)
task dev-down
```

### Troubleshooting

**Chrome shows `ERR_NAME_NOT_RESOLVED` for `*.svc.cluster.local`**

Chrome Secure DNS or proxy extensions can bypass the Telepresence DNS resolver. Fix:

```bash
task dev-hosts
```

This writes ClusterIP entries to `/etc/hosts` under a managed block. Alternatively, disable Secure DNS in Chrome settings or bypass the proxy for `*.svc.cluster.local`.

**`task dev-run` fails at the smoke test**

The TCP probe checks Postgres `:5432`, Kafka `:9092`, Keycloak `:8080`, and OTel `:4318`. If one fails:

```bash
# Check if infra pods are running
task k3d-status

# Check infra pod logs
task infra-logs INFRA_SERVICE=postgres
task infra-logs INFRA_SERVICE=kafka

# Reconnect Telepresence
task dev-connect
```

**Telepresence asks for a password every time**

Telepresence needs `sudo` for its root daemon that manages the host network/DNS. This prompt appears once per macOS login session. If it appears repeatedly, confirm that the daemon is running:

```bash
.dev/tools/telepresence status
```

**Local service cannot reach Kafka or Postgres after a cluster restart**

k3d cluster restarts may change ClusterIP addresses. Reconnect Telepresence to pick up the new routes:

```bash
task dev-disconnect
task dev-connect
```

**Port conflict on `:8888`**

Port `8888` is the k3d host-port mapping for the Istio ingress. `task dev-up` removes stale Docker Compose containers that may hold this port. If the port is still occupied, find and stop the process:

```bash
lsof -i :8888
```

**Cleaning up the entire environment**

```bash
task dev-down          # disconnects Telepresence and deletes the k3d cluster
```

### Quick Reference

| Task | What it does |
| --- | --- |
| `task dev-up` | Full bootstrap: k3d + Istio + infra + apps + Telepresence |
| `task dev-down` | Tear down Telepresence and delete the k3d cluster |
| `task dev-run SERVICE=<name>` | Run one service locally with Air hot reload |
| `task dev-connect` | (Re)connect Telepresence to the k3d namespace |
| `task dev-disconnect` | Stop Telepresence without stopping k3d |
| `task dev-status` | Show k3d pods, Istio, and Telepresence state |
| `task dev-ui` | Verify and print browser UI URLs |
| `task dev-hosts` | Update `/etc/hosts` for browser DNS fallback |
| `task dev-logs SERVICES=<name>` | Follow in-cluster app logs |
| `task dev-start SERVICES=<name>` | Rebuild, import, and restart k3d deployments |
| `task dev-stop SERVICES=<name>` | Scale k3d app deployments to zero |
| `task k3d-migrate` | Run SQL migrations against k3d Postgres |
| `task infra-logs INFRA_SERVICE=<name>` | Follow infrastructure service logs |

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

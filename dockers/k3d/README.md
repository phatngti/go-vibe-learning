# K3d + Istio Local Cluster

This folder contains the local Kubernetes surface for the project. It keeps the cluster setup next to the existing Docker assets because the manifests reuse the same Postgres init SQL, Keycloak realm import, OTel collector config, Envoy config, descriptor, and Swagger document.

## Prerequisites

- Docker
- Task
- Docker Compose
- kubectl
- curl and tar, used to install pinned local development tools when needed
- Optional: host k3d CLI

## Deploy

```bash
task k3d-deploy
```

The task sequence is:

1. Create or select the `master-go-cqrs` k3d cluster from `dockers/k3d/cluster.yaml`.
2. Install Istio and label the `master-go-cqrs` namespace with `istio-injection=enabled`.
3. Build local service images and import them into the k3d nodes.
4. Sync ConfigMaps from existing repo artifacts.
5. Apply Postgres, Kafka, Jaeger, OTel, and Keycloak manifests.
6. Run service SQL migrations inside the Postgres pod.
7. Apply app, Envoy, Swagger UI, Istio Gateway, and VirtualService manifests.
8. Smoke check `http://localhost:8888/api/v1/health`.

If the host does not have `k3d`, the Taskfile uses the `k3d` service from `dockers/docker-compose.yaml`. That compose file is only a CLI helper mounted to the host Docker socket; all runtime services run as Kubernetes workloads in k3d.
If the host does not have `istioctl`, the Taskfile downloads the configured Istio version into `.dev/tools/istioctl`. `task dev-up` also removes obsolete `master_go_cqrs_*` Compose containers before k3d starts, because the old gateway container can hold port `8888`.

`task dev-up` additionally installs the pinned Telepresence traffic manager with the local-cluster route controller enabled, connects the host to this namespace, and verifies access to the core infrastructure services. `task dev-run SERVICE=payment` runs the same Telepresence readiness path before starting a local Air-reloaded service against k3d, without intercepting or replacing the Kubernetes deployment. Use `task dev-disconnect` to stop only the Telepresence connection.

The k3d gateway uses the local Envoy profile at `dockers/envoy/profiles/local/envoy.env`. Shared route policy lives in `dockers/envoy/shared/routes.yaml`; production-only gateway values belong in `dockers/envoy/profiles/prod/envoy.env` and are not consumed by `task dev-up`.

`task dev-up` exposes Envoy through the k3d load balancer on `localhost:8888`. Internal ClusterIP services are reached from the host through Telepresence service DNS, so Keycloak, Jaeger, Kafka, and optional Temporal do not need `kubectl port-forward`. Print and verify the browser UI URLs with:

```bash
task dev-ui
```

Telepresence works as the local network bridge for service DNS, for example `postgres.master-go-cqrs.svc.cluster.local:5432`, `kafka.master-go-cqrs.svc.cluster.local:9092`, or `keycloak.master-go-cqrs.svc.cluster.local:8080`. `/etc/hosts` is normally unnecessary because Telepresence installs a local DNS resolver for the mapped namespace.

If Chrome cannot resolve `*.svc.cluster.local`, update `/etc/hosts` with current k3d service ClusterIPs:

```bash
task dev-hosts
```

The task manages only the `master_go_cqrs k3d service hosts` block in `/etc/hosts`; it leaves unrelated entries untouched.

## Local URLs

| Surface | Command | URL |
| --- | --- | --- |
| Envoy through Istio | `task k3d-deploy` | http://localhost:8888 |
| Swagger UI | `task k3d-deploy` | http://localhost:8888/api/v1/swagger/ |
| Keycloak | `task dev-ui` | http://keycloak.master-go-cqrs.svc.cluster.local:8080/admin |
| Jaeger | `task dev-ui` | http://jaeger.master-go-cqrs.svc.cluster.local:16686 |
| Temporal UI | `task k3d-temporal-up`, then `task dev-ui` | http://temporal-ui.master-go-cqrs.svc.cluster.local:8080 |

## Files

- `cluster.yaml`: k3d cluster config. Traefik is disabled so Istio owns ingress.
- `manifests/00-namespace.yaml`: namespace with Istio sidecar injection.
- `manifests/10-secrets.yaml`: local development credentials for the k3d workloads.
- `manifests/20-postgres.yaml` through `24-kafka.yaml`: required infrastructure services, including a single-node Kafka KRaft broker.
- `manifests/26-temporal.yaml`: optional pinned Temporal Server and UI workloads. Temporal uses dedicated databases in the shared PostgreSQL instance.
- `manifests/30-apps.yaml`: service deployments and ClusterIP services.
- `manifests/40-gateway.yaml`: Envoy, Swagger UI, Istio Gateway, and VirtualService.

## Operations

```bash
task k3d-status
task k3d-logs SERVICE=payment
task k3d-smoke
task dev-ui
task dev-hosts
task k3d-kafka-url
task k3d-temporal-up
task orchestrator-mode MODE=temporal
task orchestrator-mode MODE=saga
task dev-connect
task dev-run SERVICE=payment
task dev-disconnect
task k3d-cluster-delete
```

Use `task k3d-apply` after manifest-only edits. Use `task k3d-image-import && task k3d-apply` after code changes that require fresh service images.

# Master Go CQRS documentation

This directory separates architecture decisions, operational guidance, engineering standards, and generated API specifications. Start with the overview, then follow the links for the area you are changing.

## Architecture

| Document | Purpose |
| --- | --- |
| [System overview](architecture/overview.md) | Repository boundaries, service layering, dependency injection, database ownership, and ingress. |
| [CQRS and service integration handbook](architecture/cqrs-service-integration-handbook.md) | Learning-oriented guide to CQRS, service boundaries, messaging, orchestration, payments, security, scaling, and the production upgrade status. |
| [System flows](architecture/system-flows.md) | Authentication, payment orchestration, transactional messaging, CQRS, and tracing flows. |
| [Distributed transactions](architecture/distributed-transactions.md) | Saga/Temporal decisions, Outbox/Inbox invariants, payment idempotency, state machines, and failure semantics. |

## Operations

| Document | Purpose |
| --- | --- |
| [Production operations](operations/production.md) | Production topology, HA profiles, security, observability, rollout, and acceptance gates. |
| [Incident runbooks](operations/runbooks.md) | Response procedures for Outbox backlog, uncertain payments, Temporal workflows, and DLQ records. |
| [Database migrations](operations/migrations.md) | Service-owned migration locations and database ownership. |

## Standards

| Document | Purpose |
| --- | --- |
| [REST API standards](standards/rest-api.md) | HTTP conventions, DTOs, errors, authentication, and OpenAPI annotations. |
| [Protobuf and gRPC standards](standards/protobuf-grpc.md) | Contract layout, compatibility rules, Envoy transcoding, and generation. |

## Generated API specifications

Generated OpenAPI JSON is stored under [`api/`](api/README.md). These files are runtime inputs and must be regenerated from source rather than edited by hand:

```bash
task gen-docs
```

Use `task swagger-refresh` when the local k3d Swagger deployments also need to be updated. When k3d is running, the documentation landing page is available at <http://localhost:8888/api/v1/docs/>.

# Master Go CQRS Documentation Hub

Welcome to the documentation portal for **Master Go CQRS** — a enterprise-grade Go microservices repository demonstrating CQRS architecture, gRPC and REST APIs, transactional messaging (Outbox/Inbox patterns), distributed Saga and Temporal orchestration, Envoy API Gateway, and local Kubernetes development with Istio and Telepresence.

---

## 📚 Documentation Index

| Topic | Document | Summary |
| --- | --- | --- |
| 🏗️ **Architecture & Structure** | [docs/architecture.md](file:///Volumes/WD850X/Users/workspace/roadmap/master_go_cqrs/docs/architecture.md) | Service boundaries, CQRS layering, Uber Fx dependency injection, database isolation, and infra architecture. |
| 🔄 **System Design & Flow** | [docs/system_design_flow.md](file:///Volumes/WD850X/Users/workspace/roadmap/master_go_cqrs/docs/system_design_flow.md) | End-to-end execution flows for Authentication, Saga/Temporal Orchestration, Outbox/Inbox events, and CQRS read/write. |
| 🌐 **API Standards & Guide** | [docs/api_standards.md](file:///Volumes/WD850X/Users/workspace/roadmap/master_go_cqrs/docs/api_standards.md) | REST API design rules, Echo routing, DTO conventions, unified error formats, Keycloak auth headers, and Swagger. |
| ⚡ **Proto & gRPC Standards** | [docs/proto_standards.md](file:///Volumes/WD850X/Users/workspace/roadmap/master_go_cqrs/docs/proto_standards.md) | Protobuf file layout, gRPC service definitions, `google.api.http` gateway mapping, generation workflows, and versioning. |
| 🗄️ **Database Migrations** | [docs/migrations.md](file:///Volumes/WD850X/Users/workspace/roadmap/master_go_cqrs/docs/migrations.md) | Database per service layout, migration file structure, and ownership mapping. |

---

## 🛠️ Quick CLI Reference

```bash
# Generate REST Swagger documentation (from Go handler annotations)
task gen-swagger

# Generate gRPC Proto stubs & Envoy descriptor
task gen-proto

# Generate both REST and gRPC API documentation
task gen-docs

# Refresh docs and deploy ConfigMaps to k3d cluster
task swagger-refresh
```

---

## 🌐 Local API & Documentation Portals (when k3d is running)

- **API Docs Landing Page**: [http://localhost:8888/api/v1/docs/](http://localhost:8888/api/v1/docs/)
- **REST API Swagger UI**: [http://localhost:8888/api/v1/docs/rest/](http://localhost:8888/api/v1/docs/rest/)
- **gRPC Gateway Swagger UI**: [http://localhost:8888/api/v1/docs/grpc/](http://localhost:8888/api/v1/docs/grpc/)
- **Envoy API Gateway**: [http://localhost:8888](http://localhost:8888)

# Protobuf & gRPC Service Standards

This document defines standards for defining Protobuf contracts (`.proto`), gRPC service definitions, Envoy HTTP transcoding annotations, and contract generation workflows across **Master Go CQRS**.

---

## 1. Proto File Structure & Location

All Protobuf contract definitions live centrally under `contracts/proto/`:

```text
contracts/proto/
├── affiliate.proto
├── agent.proto
├── payment.proto
├── settlement.proto
└── user.proto
```

Generated Go code is output exclusively to `contracts/genproto/<service>/`:

```text
contracts/genproto/
├── affiliate/
├── agent/
├── payment/
├── settlement/
└── user/
```

### Standard Header Options

Every `.proto` file MUST include:
- `syntax = "proto3";`
- Domain package declaration (`package <domain>.v1;`)
- Import of `google/api/annotations.proto` when mapping REST endpoints
- `go_package` option pointing to `github/master_go_cqrs/contracts/genproto/<domain>;<domain>v1`

```protobuf
syntax = "proto3";

package payment.v1;

import "google/api/annotations.proto";

option go_package = "github/master_go_cqrs/contracts/genproto/payment;paymentv1";
```

---

## 2. Naming & Style Conventions

### Messages & Enums
- **Message Names**: Use `PascalCase` (e.g. `InitiatePaymentRequest`, `GetTransactionStatusResponse`).
- **RPC Request/Response Structure**: Every RPC method MUST have dedicated Request and Response message types named `<RPCMethod>Request` and `<RPCMethod>Response`.
- **Field Names**: Use `snake_case` (e.g. `transaction_id`, `customer_id`, `callback_url`).

```protobuf
message ProcessPaymentRequest {
  string transaction_id = 1;
  string payment_method = 2;
  string card_number = 3;
  string cvv = 4;
}

message ProcessPaymentResponse {
  string transaction_id = 1;
  string status = 2;
  string message = 3;
}
```

### Field Tag Assignments
- Field numbers 1 through 15 take 1 byte to encode; reserve them for frequently used fields.
- Field numbers must never be renumbered or reassigned once committed.

---

## 3. Envoy Transcoding & `google.api.http` Annotations

Services expose gRPC APIs internally while Envoy API Gateway transcodes external HTTP/JSON requests into gRPC calls. This transcoding behavior is driven by `google.api.http` annotations embedded in the service RPC definition.

### Annotation Examples

#### POST Command Endpoint (with request body):
```protobuf
rpc InitiatePayment(InitiatePaymentRequest) returns (InitiatePaymentResponse) {
  option (google.api.http) = {
    post: "/api/v1/payment/initiate"
    body: "*"
  };
}
```

#### GET Query Endpoint (with URL path parameters):
```protobuf
rpc GetTransactionStatus(GetTransactionStatusRequest) returns (GetTransactionStatusResponse) {
  option (google.api.http) = {
    get: "/api/v1/payment/checkout/{transaction_id}"
  };
}
```

---

## 4. Code Generation & Envoy Descriptor Workflow

The repository provides automated Task commands to compile `.proto` files into Go stubs, OpenAPI Swagger JSON files, and Envoy binary descriptors (`dockers/envoy/proto.pb`).

### Code Generation Workflow

```bash
# 1. Compile proto files into Go packages & Envoy descriptor
task gen-proto

# 2. Build full documentation set (Swagger + Proto)
task gen-docs

# 3. Validate Envoy gateway rules against descriptor
task gateway-validate
```

### What `task gen-proto` Executes:
1. Invokes `protoc` (or `buf`) compiler over all `contracts/proto/*.proto` files.
2. Generates Go structs and gRPC service interfaces into `contracts/genproto/<service>/`.
3. Compiles a merged protobuf descriptor set (`dockers/envoy/proto.pb`) for Envoy gRPC-JSON transcoder.
4. Generates the gRPC gateway OpenAPI specification at `docs/api/envoy.swagger.json`.

---

## 5. Backward Compatibility & Contract Evolution

To ensure continuous deployment without breaking running clients or orchestrator workflows:

- **Never delete or rename existing field numbers**.
- **Never change a field's scalar data type** (e.g., `int32` to `string`).
- **Addition of new fields**: Always add new fields with a new, unused tag number. Proto3 handles missing optional fields with standard zero-values.
- **Deprecation**: If a field is deprecated, mark it with `[deprecated = true]` option instead of deleting it immediately.

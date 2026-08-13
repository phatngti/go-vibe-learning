# System Design & Core Execution Flows

This document details the core design flows, sequence diagrams, event lifecycle workflows, and interaction protocols implemented across the **Master Go CQRS** platform.

---

## 1. Authentication & Authorization Gateway Flow

Authentication is delegated to **Keycloak**. Microservices do not handle password validation directly; instead, **Envoy API Gateway** enforces JWT verification at the edge.

### Sequence Flow: Client Authentication & Protected Route Request

```text
 Client                Keycloak               Envoy Gateway              User/App Service
   │                      │                         │                           │
   │ 1. POST /token       │                         │                           │
   ├─────────────────────►│                         │                           │
   │    Credentials       │                         │                           │
   │ 2. Return JWT Token  │                         │                           │
   │◄─────────────────────┤                         │                           │
   │                      │                         │                           │
   │ 3. HTTP Request with Bearer Header             │                           │
   ├───────────────────────────────────────────────►│                           │
   │    Authorization: Bearer <JWT>                 │                           │
   │                      │                         │                           │
   │                      │ 4. Fetch/Cache JWKS     │                           │
   │                      │◄────────────────────────┤                           │
   │                      │    Public Keys          │                           │
   │                      │                         │ 5. Verify Token Signature │
   │                      │                         │    & Claims               │
   │                      │                         │ 6. Inject Header:         │
   │                      │                         │    x-gateway-jwt-payload  │
   │                      │                         │ 7. Forward (HTTP or gRPC) │
   │                      │                         ├──────────────────────────►│
   │                      │                         │                           │ 8. Extract claims from
   │                      │                         │                           │    x-gateway-jwt-payload
   │                      │                         │ 9. Return Response        │    context
   │◄───────────────────────────────────────────────┼───────────────────────────┤
```

### Key Gateway Components:
- **`jwt_authn` Filter**: Verifies token signature offline against Keycloak JWKS endpoint (`http://keycloak.../certs`) and caches the keys.
- **`ext_authz` Filter**: Enforces endpoint protection. Public routes (such as `/api/v1/payment/checkout/*` and `/api/v1/docs/*`) bypass auth checks, while protected routes require valid credentials.
- **Claim Forwarding**: Envoy parses claims from the verified JWT and encodes them into the `x-gateway-jwt-payload` header. Services parse this header via `gatewayauth` middleware into the Request Context.

---

## 2. Distributed Payment Orchestration Flow (CQRS + Saga / Temporal)

When a payment is initiated and processed, multiple services (`payment`, `orchestrator`, `affiliate`, `settlement`, `agent`) coordinate asynchronously through domain events.

### Orchestration Architecture Comparison

The platform supports two payment orchestration engines:
1. **Saga Engine (Default)**: Event-driven state machine managed by `services/orchestrator/internal/payment/saga`. Tracks step progress in `payment_sagas` DB table and emits step request commands to Kafka.
2. **Temporal Engine**: Workflow-based execution using Temporal Server. Executes the payment workflow steps using durable activity task queues.

### End-to-End Payment Flow Diagram

```text
 Client          Payment Service          Kafka Bus          Orchestrator        Affiliate / Settlement / Agent
   │                    │                     │                   │                                │
   │ 1. InitiatePayment │                     │                   │                                │
   ├───────────────────►│                     │                   │                                │
   │                    │ 2. DB Transaction:  │                   │                                │
   │                    │    Save Transaction │                   │                                │
   │                    │    Save Outbox      │                   │                                │
   │ 3. Return Pending  │                     │                   │                                │
   │◄───────────────────┤                     │                   │                                │
   │                    │ 4. Outbox Publisher │                   │                                │
   │                    ├────────────────────►│                   │                                │
   │                    │    PaymentInitiated │                   │                                │
   │                    │                     │ 5. Consume        │                                │
   │                    │                     ├──────────────────►│                                │
   │                    │                     │    PaymentInitiated│ 6. Start Saga / Temporal Workflow│
   │                    │                     │                   │ 7. AccumulateVolumeRequested   │
   │                    │                     │◄──────────────────┤                                │
   │                    │                     │                   │                                │
   │                    │                     │ 8. Consume AccumulateVolume                        │
   │                    │                     ├───────────────────────────────────────────────────►│
   │                    │                     │                   │                                │ 9. Accumulate Volume
   │                    │                     │ 10. VolumeAccumulated                              │    & Save Outbox
   │                    │                     │◄───────────────────────────────────────────────────┤
   │                    │                     │                   │                                │
   │                    │                     │ 11. Consume       │                                │
   │                    │                     ├──────────────────►│                                │
   │                    │                     │    VolumeAccumulate│ 12. CreditCommissionRequested │
   │                    │                     │                   ├───────────────────────────────►│
   │                    │                     │                   │                                │ 13. Credit Balance
   │                    │                     │                   │ 14. CommissionCredited         │     & Save Outbox
   │                    │                     │                   │◄───────────────────────────────┤
   │                    │                     │                   │                                │
   │                    │                     │                   │ 15. DispatchCallbackRequested  │
   │                    │                     │                   ├───────────────────────────────►│
   │                    │                     │                   │                                │ 16. Dispatch HTTP
   │                    │                     │                   │ 17. CallbackDispatched         │     Callback
   │                    │                     │                   │◄───────────────────────────────┤
   │                    │                     │                   │                                │
   │                    │                     │ 18. PaymentCompleted                               │
   │                    │                     │◄──────────────────┤                                │
   │ 19. Check Status   │                     │                   │                                │
   ├───────────────────►│ 20. Update DB State │                   │                                │
   │◄───────────────────┴─────────────────────┤                   │                                │
   │  Return Completed                        │                   │                                │
```

### Orchestration Step Sequence:
1. **Initiate Payment**: Client initiates transaction. `payment` service creates transaction aggregate in `PENDING` state and writes `PaymentInitiated` event to outbox within a single DB transaction.
2. **Saga Trigger**: `orchestrator` consumes `PaymentInitiated` event and starts a payment saga record.
3. **Step 1 - Affiliate Volume**: `orchestrator` emits `AccumulateVolumeRequested`. `affiliate` service updates agent volume metrics and responds with `VolumeAccumulated`.
4. **Step 2 - Settlement Commission**: `orchestrator` receives volume acknowledgment and emits `CreditCommissionRequested`. `settlement` service computes commission based on rank and updates `agent_balances`, responding with `CommissionCredited`.
5. **Step 3 - Agent Callback**: `orchestrator` emits `DispatchCallbackRequested`. `agent` service executes webhook callback to external agent URL and responds with `CallbackDispatched`.
6. **Completion**: `orchestrator` emits `PaymentCompleted`. `payment` service updates transaction status to `SUCCESS`.

---

## 3. Transactional Outbox & Inbox Event Processing Flow

To prevent partial failures and dual-write issues between database mutations and Kafka event publication, services implement the **Outbox Pattern** (for publishing) and **Inbox Pattern** (for consumption).

### Outbox Pattern (Event Publishing)

```text
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │ Command Handler                                                             │
 │ ┌─────────────────────────────────────────────────────────────────────────┐ │
 │ │ PostgreSQL Transaction (db.Tx)                                           │ │
 │ │ 1. UPDATE / INSERT business table (e.g. transactions)                  │ │
 │ │ 2. INSERT into outbox_events (id, topic, payload, status="PENDING")     │ │
 │ └────────────────────────────────────┬────────────────────────────────────┘ │
 └──────────────────────────────────────┼──────────────────────────────────────┘
                                        │ Commit Transaction
                                        ▼
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │ Outbox Relay Background Worker (internal/platform/outbox)                   │
 │ 1. Poll outbox_events WHERE status = "PENDING" FOR UPDATE SKIP LOCKED      │
 │ 2. Publish event payload to Kafka broker                                    │
 │ 3. UPDATE outbox_events SET status = "PROCESSED", processed_at = NOW()      │
 └──────────────────────────────────────┬──────────────────────────────────────┘
                                        │ Kafka Publish
                                        ▼
                                 ┌──────────────┐
                                 │ Kafka Topic  │
                                 └──────────────┘
```

### Inbox Pattern (Idempotent Event Consumer)

```text
                                 ┌──────────────┐
                                 │ Kafka Topic  │
                                 └──────┬───────┘
                                        │ Consume Event Message
                                        ▼
 ┌─────────────────────────────────────────────────────────────────────────────┐
 │ Inbox Consumer Middleware (internal/platform/inbox)                         │
 │ ┌─────────────────────────────────────────────────────────────────────────┐ │
 │ │ PostgreSQL Transaction (db.Tx)                                           │ │
 │ │ 1. INSERT INTO inbox_events (event_id, consumer_name) ON CONFLICT DO    │ │
 │ │    NOTHING                                                              │ │
 │ │ 2. If Duplicate Event ID: SKIP processing (return idempotent success)    │ │
 │ │ 3. Execute business logic                                               │ │
 │ │ 4. Mark inbox event state as PROCESSED                                  │ │
 │ └─────────────────────────────────────────────────────────────────────────┘ │
 └─────────────────────────────────────────────────────────────────────────────┘
```

---

## 4. CQRS Command & Query Execution Paths

```text
                                     CLIENT REQUEST
                                           │
                    ┌──────────────────────┴──────────────────────┐
                    │                                             │
             WRITE OPERATION                               READ OPERATION
                    │                                             │
                    ▼                                             ▼
        [ COMMAND TRANSPORT ADAPTER ]                 [ QUERY TRANSPORT ADAPTER ]
        (HTTP / gRPC Handler)                         (HTTP / gRPC Handler)
                    │                                             │
                    ▼                                             ▼
            [ COMMAND DTO ]                               [ QUERY DTO ]
                    │                                             │
                    ▼                                             ▼
           [ COMMAND HANDLER ]                           [ QUERY HANDLER ]
                    │                                             │
                    ▼                                             ▼
         [ DOMAIN AGGREGATE ]                        [ READ REPOSITORY PORT ]
         (Enforce Invariants)                         (Optimized SELECT Query)
                    │                                             │
                    ▼                                             ▼
         [ UNIT OF WORK / PORT ]                     [ READ DTO RESULT ]
         - Save Aggregate State                                   │
         - Save Outbox Event                                      │
                    │                                             │
                    ▼                                             ▼
         [ POSTGRES DATABASE ] ══════════════════════► [ RETURN JSON / PROTO ]
         (State Mutated)                               (Zero Side Effects)
```

---

## 5. Distributed Tracing & Observability Flow

All incoming HTTP and gRPC requests, as well as Kafka message consumption steps, are automatically instrumented with **OpenTelemetry (OTel)** context propagation.

```text
 Client -> HTTP / gRPC
   │
   ▼
[ Envoy Gateway ] (Injects traceparent HTTP Header)
   │
   ▼
[ Transport Middleware ] (Extracts TraceContext into Go Context)
   │
   ├─► [ GORM DB Query ] (Spans for SQL execution)
   │
   ├─► [ Outbox Relay ] (Injects TraceContext into Kafka Header)
   │
   └─► [ OTel Collector ] (localhost:4318 / otel.master-go-cqrs...)
            │
            ▼
      [ Jaeger UI ] (View distributed trace spans)
```

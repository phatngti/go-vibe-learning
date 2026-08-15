# System Flows

> **Source snapshot:** 2026-08-15
> **Scope:** the principal synchronous, asynchronous, and recovery paths implemented by the current source.
> **Evidence boundary:** diagrams describe code-level behavior and the development topology; they do not prove a live production deployment.

This document is the interaction map for the runtime. It shows where trust changes, where a transaction begins and ends, which component owns an external side effect, and how an at-least-once delivery becomes an idempotent business effect.

Detailed invariants and state-machine rules belong in [Distributed Transactions](distributed-transactions.md). Component boundaries belong in [System Architecture](overview.md). Production tuning, SLOs, and deployment gates belong in [Production Operations](../operations/production.md).

## 1. Reading the diagrams

The flows use these terms consistently:

| Term | Meaning |
|---|---|
| **Local transaction** | one transaction in one service-owned PostgreSQL database |
| **Domain event** | an immutable fact appended to the service EventStore or emitted by a domain handler |
| **Outbox intent** | a durable request to publish a Kafka message after the local transaction commits |
| **Inbox claim** | consumer-scoped deduplication for one `(consumer_name, message_id)` |
| **Business idempotency** | a stable API/provider/ledger identity that prevents a repeated business effect |
| **At least once** | publication or delivery may repeat after a crash or lost acknowledgement |
| **Authoritative outcome** | a result proven by provider/downstream state, not inferred from a timeout |

The core delivery equation is:

```text
local atomic commit + at-least-once delivery + idempotent consumer = one business effect
```

---

## 2. Authentication and protected request flow

### 2.1 Login and token issuance

```mermaid
sequenceDiagram
    actor Client
    participant Envoy
    participant User as User service
    participant Keycloak

    Client->>Envoy: POST /api/v1/auth/login
    Envoy->>User: Public HTTP route
    User->>Keycloak: Resource-owner authentication request
    Keycloak-->>User: Access token + refresh token
    User-->>Envoy: Wrapped login response
    Envoy-->>Client: Tokens
```

Login, refresh, registration, documentation, health endpoints, and the Stripe webhook are public at the gateway JWT layer. “Public” means the route does not require a caller JWT; it does not remove provider-signature validation, input validation, rate limiting, or other route-specific controls.

### 2.2 Protected request

```mermaid
sequenceDiagram
    actor Client
    participant Envoy
    participant Keycloak
    participant ExtAuthz as Service ext_authz check
    participant Service

    Client->>Envoy: Request + Authorization: Bearer JWT
    Envoy->>Envoy: Strip caller x-gateway-jwt-payload
    Envoy->>Keycloak: Fetch/cache JWKS when needed
    Envoy->>Envoy: Verify signature and registered claims
    Envoy->>ExtAuthz: Check method/path/resource access
    alt authentication or route authorization fails
        ExtAuthz-->>Envoy: 401 or 403
        Envoy-->>Client: Reject
    else gateway permits request
        ExtAuthz-->>Envoy: Allow
        Envoy->>Service: Route/transcode + bearer token + verified claims
        Service->>Service: Re-verify JWT when AUTH_JWT_REQUIRED=true
        Service->>Service: Derive tenant/agent owner from principal
        Service->>Service: Apply command/query authorization
        Service-->>Client: Response
    end
```

The client cannot establish ownership by sending `agent_id` or `tenant_id` in the request body. Protected handlers use the verified principal. A lookup for another owner's payment returns not-found rather than revealing that the resource exists.

---

## 3. User registration and durable Keycloak synchronization

Registration creates the identity in Keycloak first, then attempts to record the event that feeds durable synchronization. The durable path begins only after `UserRegistered` exists in the User database.

```mermaid
sequenceDiagram
    actor Client
    participant User as User HTTP handler
    participant Keycloak
    participant UserDB as User PostgreSQL
    participant EventWorker as EventStore consumer
    participant SyncWorker as Sync-job worker

    Client->>User: POST /api/v1/auth/register
    User->>Keycloak: Create identity
    Keycloak-->>User: Token containing provider subject
    User->>UserDB: Append UserRegistered and attempt relay Outbox enqueue
    opt local event emission fails
        User->>User: Log failure; current handler does not fail registration
    end
    User-->>Client: Registration response

    EventWorker->>UserDB: Claim UserRegistered through consumer checkpoint
    EventWorker->>UserDB: Same transaction: insert deduplicated sync job + advance checkpoint

    SyncWorker->>UserDB: Lease one job/page
    SyncWorker->>Keycloak: Fetch user or bounded page outside DB transaction
    Keycloak-->>SyncWorker: Identity data
    SyncWorker->>UserDB: Local transaction: upsert identity + append UserSynced + advance cursor

```

Key failure properties:

- Keycloak creation and local event emission cannot share a transaction. In the current source, registration still succeeds when `EmitUserRegistered` fails; an explicit retry/reconciliation trigger is therefore still required to guarantee repair of that handoff.
- `EmitUserRegistered` is not wrapped in a Unit of Work. The EventStore append may persist before the matching Outbox enqueue fails. Local EventStore polling can still create the sync job, but `user.events` publication is not guaranteed for that failure window.
- Event consumption and sync-job creation commit together; a crash cannot advance the checkpoint without preserving work.
- Keycloak I/O is outside the database transaction and bounded by `providerTimeout`.
- A full sync advances one page and cursor per transaction, so it is restart-safe and fairly schedulable.
- The job uses a lease token, bounded attempts, and exponential backoff.
- The local `user_identities` table is a projection; Keycloak remains the external identity authority.
- Full-sync enqueue and execution handlers exist, but this source snapshot does not expose a transport or scheduled trigger for them; wiring that trigger remains an operational integration gap.

---

## 4. Payment initiation and provider outcome

The source contains a legacy learning flow and a modern v2 PaymentIntent flow. Both produce the same orchestration facts, but their provider interactions differ.

### 4.1 Modern v2 PaymentIntent creation

```mermaid
sequenceDiagram
    actor Client
    participant Envoy
    participant Payment
    participant PaymentDB as Payment PostgreSQL
    participant Provider as Stripe or simulator
    participant Publisher as Outbox publisher
    participant Kafka

    Client->>Envoy: POST /api/v2/payments + Idempotency-Key
    Envoy->>Payment: gRPC-transcoded create command + principal
    Payment->>Payment: Validate Money, owner, metadata, request fingerprint
    Payment->>PaymentDB: Local transaction: create-or-get PENDING + PaymentInitiated + Outbox
    alt matching request exists and provider reference is known
        Payment->>Provider: Retrieve existing provider intent when reference is known
    else new request or reference not yet persisted
        Payment->>Provider: Create intent with stable provider key
    end
    Provider-->>Payment: Provider reference, status, client secret
    Payment->>PaymentDB: Persist provider intent metadata
    Payment-->>Client: Transaction + provider/client data

    Publisher->>PaymentDB: Claim committed immutable Outbox message
    Publisher->>Kafka: payments.initiated keyed by transaction_id
```

The API idempotency key identifies the create request; a request fingerprint rejects reuse with conflicting money, owner, callback endpoint, or metadata. The provider intent uses a separate stable key derived from the internal transaction ID. Money v2 uses integer minor units, currency, and exponent; the legacy floating-point field exists only for compatibility.

### 4.2 Provider webhook

```mermaid
sequenceDiagram
    participant Provider as Stripe
    participant Envoy
    participant PaymentHTTP as Payment raw HTTP route
    participant PaymentDB as Payment PostgreSQL
    participant Publisher as Outbox publisher
    participant Kafka

    Provider->>Envoy: POST /api/v2/payments/webhooks/stripe
    Envoy->>PaymentHTTP: Preserve exact raw body
    PaymentHTTP->>PaymentHTTP: Verify signature, timestamp tolerance, body size
    PaymentHTTP->>PaymentDB: Begin local transaction
    PaymentDB->>PaymentDB: Insert provider event by unique provider event ID
    alt duplicate webhook
        PaymentDB-->>PaymentHTTP: Already processed; no duplicate effect
    else valid new event
        PaymentDB->>PaymentDB: Apply monotonic provider outcome
        PaymentDB->>PaymentDB: Append terminal event + matching Outbox intent
        PaymentDB->>PaymentDB: Mark provider event processed/ignored
    end
    PaymentHTTP-->>Provider: Acknowledge
    Publisher->>PaymentDB: Claim committed terminal fact
    Publisher->>Kafka: payments.events keyed by transaction_id
```

The webhook is public only because it authenticates the provider through the signature rather than a user JWT. A terminal payment cannot be overwritten by a conflicting late provider event. Duplicate provider events are deduplicated before applying state.

### 4.3 Legacy charge and ambiguous timeout

```mermaid
sequenceDiagram
    actor Client
    participant Payment
    participant PaymentDB as Payment PostgreSQL
    participant Provider
    participant Reconciler
    participant Kafka

    Client->>Payment: Process transaction with stable key
    Payment->>PaymentDB: CAS PENDING/UNKNOWN/stale PROCESSING → PROCESSING
    alt claim not acquired
        Payment-->>Client: Existing terminal result or in-progress conflict
    else claim acquired
        Payment->>Provider: Charge using the same idempotency key
        alt authoritative success or decline
            Provider-->>Payment: Definitive result
            Payment->>PaymentDB: Local transaction: terminal state + domain event
            PaymentDB->>Kafka: EventStore relay → Outbox → payment event
        else timeout / transport error
            Payment->>PaymentDB: Local transaction: UNKNOWN + PaymentProcessingUncertain
            PaymentDB->>Kafka: Relay uncertain fact
            Reconciler->>PaymentDB: Select UNKNOWN or stale PROCESSING
            Reconciler->>Provider: LookupCharge(original key, reference)
            alt provider has no authoritative result yet
                Reconciler-->>Reconciler: Leave pending for a later pass
            else authoritative outcome
                Reconciler->>PaymentDB: CAS terminal state + domain event
                PaymentDB->>Kafka: Relay definitive result
            end
        end
    end
```

The reconciler never calls `Charge`. A timeout cannot distinguish “not received,” “received but not charged,” and “charged but response lost,” so it must not be converted into a decline. See [Payment idempotency and unknown outcomes](distributed-transactions.md#chapter-6--payment-idempotency-and-unknown-outcomes).

---

## 5. End-to-end payment orchestration

Exactly one engine owns this process in an environment. `ORCHESTRATOR_ENGINE=temporal` selects Temporal; `saga` selects the PostgreSQL-backed fallback. Consumer groups isolate the selected engine and generation, while message envelopes carry the orchestration owner and generation. A cutover must still ensure that two engines do not produce commands concurrently.

```mermaid
sequenceDiagram
    participant Payment
    participant Kafka
    participant Orchestrator
    participant Affiliate
    participant Settlement
    participant Agent
    participant Merchant as Callback endpoint

    Payment->>Kafka: PaymentInitiated → payments.initiated
    Kafka->>Orchestrator: Start payment:<transaction_id>
    Orchestrator->>Orchestrator: State = WAITING_PAYMENT

    Payment->>Kafka: PaymentCompleted → payments.events
    Kafka->>Orchestrator: Apply authoritative outcome
    Orchestrator->>Kafka: AccumulateVolumeRequested → affiliate.commands
    Orchestrator->>Orchestrator: State = WAITING_AFFILIATE

    Kafka->>Affiliate: Consume command
    Affiliate->>Affiliate: Inbox + volume/rank + event + Outbox in one transaction
    Affiliate->>Kafka: VolumeAccumulated → affiliate.events
    Kafka->>Orchestrator: Advance process
    Orchestrator->>Kafka: CreditCommissionRequested → settlement.commands
    Orchestrator->>Orchestrator: State = WAITING_SETTLEMENT

    Kafka->>Settlement: Consume command
    Settlement->>Settlement: Inbox + balance/ledger + event + Outbox in one transaction
    Settlement->>Kafka: CommissionCredited → settlement.events
    Kafka->>Orchestrator: Advance process
    Orchestrator->>Kafka: DispatchCallbackRequested → agent.commands
    Orchestrator->>Orchestrator: State = WAITING_CALLBACK

    Kafka->>Agent: Persist Inbox + durable callback intent
    Agent->>Merchant: Signed callback with stable event identity
    Merchant-->>Agent: 2xx or error
    Agent->>Kafka: Callback result → agent.events
    Kafka->>Orchestrator: Complete or record callback failure
```

### 5.1 State transitions

```mermaid
stateDiagram-v2
    [*] --> WAITING_PAYMENT: PaymentInitiated
    WAITING_PAYMENT --> WAITING_AFFILIATE: PaymentCompleted
    WAITING_PAYMENT --> FAILED: definitive PaymentFailed
    WAITING_PAYMENT --> NEEDS_MANUAL_REVIEW: uncertain / retry exhausted
    WAITING_AFFILIATE --> WAITING_SETTLEMENT: VolumeAccumulated
    WAITING_SETTLEMENT --> WAITING_CALLBACK: CommissionCredited
    WAITING_CALLBACK --> COMPLETED: delivered or skipped
    WAITING_CALLBACK --> COMPLETED_WITH_CALLBACK_FAILURE: terminal delivery failure
    WAITING_AFFILIATE --> NEEDS_MANUAL_REVIEW: retry exhausted
    WAITING_SETTLEMENT --> NEEDS_MANUAL_REVIEW: retry exhausted
    WAITING_CALLBACK --> NEEDS_MANUAL_REVIEW: retry exhausted
```

`PaymentFailed` stops the financial process only when it is authoritative and arrives before downstream financial progress. A conflicting failure after volume or settlement progress moves to manual review rather than rewriting history.

### 5.2 Temporal and Saga behavior

| Concern | Temporal engine | PostgreSQL Saga engine |
|---|---|---|
| Process identity | Workflow ID `payment:<transaction_id>` | `payment_sagas.transaction_id` |
| Durable state | Workflow history | Saga snapshot row |
| Start race | Signal-With-Start | row lock/create in event transaction |
| Timer/retry | durable Workflow timers and Activities | reconciler using `next_retry_at` |
| Duplicate event | bounded Workflow message-ID set + state semantics | Inbox + state semantics |
| Out-of-order event | Workflow buffers/re-evaluates not-ready facts | retryable error through Kafka policy |
| Code upgrade | `GetVersion`, replay tests, optional pinned Worker Versioning | application/schema compatibility |

Both engines use the same pure payment process definition for valid transitions and commands.

---

## 6. Transactional Outbox publication

```mermaid
sequenceDiagram
    participant Handler
    participant DB as Service PostgreSQL
    participant Publisher as Outbox publisher
    participant Kafka

    Handler->>DB: Begin local transaction
    Handler->>DB: Change business state
    Handler->>DB: Append domain event
    Handler->>DB: Insert immutable Outbox row
    Handler->>DB: Commit

    loop bounded batches
        Publisher->>DB: Claim per-key head rows with SKIP LOCKED + lease
        Publisher->>Kafka: Synchronous publish, key = aggregate/transaction ID
        alt acknowledgement received
            Publisher->>DB: Mark PROCESSED using locked_by fencing
        else publish error or acknowledgement lost
            Publisher->>DB: Schedule retry with backoff
        end
    end
```

Crash reasoning:

| Crash point | Durable state | Recovery |
|---|---|---|
| before database commit | neither business change nor message intent exists | retry the command using business/API idempotency |
| after database commit, before publish | business change and `PENDING` Outbox row exist | publisher or CDC sends later |
| after Kafka accepted, before `PROCESSED` | Kafka may contain the record while Outbox is retryable | duplicate publication is expected; consumer deduplicates |
| after lease expiry while old worker runs | another worker may claim | `locked_by` fencing prevents stale finalization |

A `DEAD` row quarantines later rows with the same `message_key`; causal order is not bypassed automatically.

---

## 7. Transactional Inbox consumption

```mermaid
sequenceDiagram
    participant Kafka
    participant Consumer
    participant DB as Consumer PostgreSQL

    Kafka->>Consumer: Deliver message (possibly a duplicate)
    Consumer->>DB: Begin local transaction
    Consumer->>DB: INSERT Inbox(consumer_name, message_id)
    alt Inbox identity already exists
        DB-->>Consumer: Duplicate
        Consumer->>DB: End without business mutation
    else first delivery
        Consumer->>DB: Apply domain/business effect
        Consumer->>DB: Append event and outgoing Outbox intent
        Consumer->>DB: Mark Inbox PROCESSED
        Consumer->>DB: Commit
    end
    Consumer->>Kafka: Commit source offset
```

If the process dies after the database commit but before the offset commit, redelivery finds the Inbox identity and does not apply the effect again. Inbox deduplication is necessary but not sufficient: affiliate volume and settlement commission also use stable transaction identities and database uniqueness to protect the business invariant.

Consumer failure classification:

- malformed envelope, unsupported event type, invalid money, or impossible identity → permanent failure and DLQ;
- transient database/downstream failure → bounded retry with exponential backoff;
- exhausted retry budget → `<source-topic>.dlq`;
- the source offset commits only after successful handling or successful DLQ publication;
- a fetch, offset-commit, or DLQ-infrastructure failure causes the subscription supervisor to restart the reader.

---

## 8. Callback onboarding and durable delivery

### 8.1 Endpoint registration and ownership proof

```mermaid
sequenceDiagram
    actor AgentUser as Authenticated agent
    participant Agent as Agent service
    participant AgentDB as Agent PostgreSQL
    participant Endpoint as Candidate callback host

    AgentUser->>Agent: Configure callback URL
    Agent->>Agent: Validate scheme, host, and network policy
    Agent->>Agent: Generate one-time challenge and digest
    Agent->>AgentDB: Save PENDING config + expiry + audit log
    Agent-->>AgentUser: Challenge token + well-known proof URL

    AgentUser->>Endpoint: Publish challenge proof
    AgentUser->>Agent: Verify challenge
    Agent->>Endpoint: Fetch proof outside DB transaction
    Endpoint-->>Agent: Matching token
    Agent->>AgentDB: Recheck unused/unexpired challenge, mark VERIFIED, audit
    Agent-->>AgentUser: Verified callback configuration
```

Production configuration requires HTTPS and a verified-domain allowlist. The proof fetch happens outside the short commit transaction. The stored challenge is a digest, and verification rejects expired, reused, mismatched-tenant, or mismatched-agent challenges.

### 8.2 Callback command and delivery worker

```mermaid
sequenceDiagram
    participant Kafka
    participant AgentConsumer as Agent command consumer
    participant AgentDB as Agent PostgreSQL
    participant Worker as Callback worker
    participant Endpoint as Verified endpoint

    Kafka->>AgentConsumer: DispatchCallbackRequested
    AgentConsumer->>AgentDB: Atomically checkpoint Inbox + delivery intent
    alt no callback URL
        AgentConsumer->>AgentDB: Persist SKIPPED result Outbox
    else verification required but missing
        AgentConsumer->>AgentDB: Persist terminal failure result Outbox
    else deliverable
        Worker->>AgentDB: Lease next due delivery
        Worker->>Worker: Revalidate endpoint and sign body
        Worker->>Endpoint: POST payload + event ID + timestamp + key version + HMAC
        alt 2xx
            Worker->>AgentDB: Mark delivered + enqueue CallbackDispatched
        else retryable transport/408/425/429/5xx
            Worker->>AgentDB: Schedule 30s, 2m, 10m, 1h, then 6h retry
        else permanent or retries exhausted
            Worker->>AgentDB: Mark terminal + enqueue CallbackDispatchFailed
        end
    end
    AgentDB->>Kafka: Outbox publishes callback result
```

The callback network call never occurs while the Kafka handler holds the Inbox transaction. Delivery uses a stable `event_id` as both application identity and callback idempotency key. A platform operator may explicitly replay a terminal delivery with actor and reason; normal callers cannot silently revive it.

Callback failure is a notification failure. It does not reverse payment or commission state.

---

## 9. CQRS request paths

```mermaid
flowchart TB
    request["HTTP, gRPC, Kafka, or worker input"] --> adapter["Transport / worker adapter"]
    adapter --> decision{"Command or query?"}
    decision -->|"Command"| command["Application command handler"]
    command --> domain["Validate domain invariants"]
    domain --> tx["Unit of Work"]
    tx --> state["Business state"]
    tx --> event["Domain event"]
    tx --> intent["Outbox / Inbox / durable job"]
    decision -->|"Query"| query["Application query handler"]
    query --> read["Read repository"]
    state --> db[("Service-owned PostgreSQL")]
    event --> db
    intent --> db
    read --> db
    db --> dto["Application DTO / contract response"]
```

Commands own state transitions and transactional side effects. Queries are read-only and do not enqueue messages. The current CQRS split is logical; read and write handlers normally use the same owning database.

---

## 10. Trace and identity propagation

```mermaid
flowchart LR
    client["Client trace"] --> envoy["Envoy"]
    envoy --> service["Service handler"]
    service --> postgres["PostgreSQL span"]
    service --> outbox["Outbox row<br/>traceparent + tracestate"]
    outbox --> kafka["Kafka headers"]
    kafka --> consumer["Consumer span"]
    consumer --> downstream["Downstream command / provider / callback"]
    service -.-> otel["OTel collector"]
    consumer -.-> otel
    downstream -.-> otel
```

The message envelope and Outbox persist correlation, causation, tenant, orchestration owner/generation, and W3C trace context. A consumer extracts that context before starting its span and propagates it into new outgoing messages.

Use logs/traces for high-cardinality fields such as `transaction_id`, `message_id`, provider reference, tenant ID, and callback event ID. Metrics use bounded labels such as service, topic, event type, and outcome.

---

## 11. Contract evolution flow

```mermaid
flowchart LR
    proto["Edit contracts/proto/*.proto"] --> generate["task gen-proto / task gen-docs"]
    handlers["Edit annotated HTTP handlers"] --> swagger["task gen-swagger / task gen-docs"]
    generate --> go["contracts/genproto/*"]
    generate --> descriptor["dockers/envoy/proto.pb"]
    generate --> grpcspec["docs/api/envoy.swagger.json"]
    swagger --> restspec["docs/api/swagger.json"]
    routes["dockers/envoy/shared/routes.yaml"] --> envoygen["Generate Envoy config"]
    descriptor --> envoygen
```

Compatibility rules:

- add fields with new numbers; never reuse a removed Protobuf field number;
- reserve removed field names/numbers;
- regenerate descriptors and OpenAPI from source instead of editing artifacts;
- keep legacy JSON topics while migrating dedicated v2 topics to Protobuf/Schema Registry;
- fail closed when Protobuf/Schema Registry is configured incorrectly—do not publish unexpected JSON to the same topic;
- use immutable event identity across version adapters so a format migration cannot duplicate the business effect.

See [Protobuf and gRPC Standards](../standards/protobuf-grpc.md) and [REST API Standards](../standards/rest-api.md) for detailed compatibility policy.

---

## 12. Failure-to-recovery index

| Failure | Flow that absorbs it | Result |
|---|---|---|
| repeated API create request | request idempotency key + fingerprint + create-or-get | same transaction or explicit conflict |
| provider charge timeout | payment `UNKNOWN` + lookup-only reconciler | no blind second charge |
| duplicate provider webhook | unique provider event ID | one state transition/event |
| process dies after local commit | EventStore relay/Outbox | message is published later |
| Kafka acknowledgement lost | immutable Outbox retry + Inbox/business identity | duplicate transport, one business effect |
| consumer dies before offset commit | Inbox record in business transaction | safe redelivery |
| out-of-order orchestration event | Temporal buffering or Saga retry policy | wait/retry rather than invalid transition |
| retry budget exhausted | `NEEDS_MANUAL_REVIEW` with `resume_state` | late authoritative event can resume |
| callback endpoint transient failure | durable schedule with bounded backoff | retry without blocking Kafka partition |
| callback endpoint permanent failure | terminal callback event | financial state remains intact |
| malformed/poison Kafka record | permanent classification + DLQ | evidence preserved, partition released after DLQ publish |
| Workflow code changes | replay test + `GetVersion` + optional Worker Versioning | deterministic history compatibility |

Operator actions for these states are documented in [Incident Runbooks](../operations/runbooks.md); alert thresholds and rollout gates are documented in [Production Operations](../operations/production.md).

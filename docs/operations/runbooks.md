# Incident runbooks

These procedures cover the failure modes implemented by the payment orchestration core. Use them together with the dashboards, ownership, and rollout requirements in [Production Operations](production.md).

## Safety rules

- Establish the authoritative business and provider state before mutating workflow data.
- Preserve immutable message IDs, source coordinates, correlation IDs, actor, and reason during replay or manual resolution.
- Do not convert an uncertain payment to `FAILED` only because a timeout elapsed.
- Do not make a later event bypass a `DEAD` event for the same message key.
- Use audited administrative tooling for replay and resolution; do not bulk-edit queue tables.

## Outbox backlog

1. Check Kafka health, ACLs, TLS, and producer errors.
2. Inspect pending depth, oldest age, `DEAD` rows, and the first blocked message per key.
3. Confirm that polling or CDC is the only active relay owner.
4. Scale publishers only when the backlog contains many independent message keys.
5. Fix the root cause before replaying a `DEAD` row with its immutable message identity.
6. Verify that the backlog and oldest age fall after recovery.

Never reset processed rows to pending. A `DEAD` row quarantines later rows for the same key because allowing them to pass can violate causal order.

## Payment in `UNKNOWN`

1. Check provider latency, error rate, webhooks, and status-lookup availability.
2. Group uncertain payments by provider, error, and age.
3. Run reconciliation with the original idempotency key and provider reference.
4. Do not call `Charge` again from reconciliation.
5. Apply only an authoritative success or decline result.
6. If the provider cannot establish the outcome, escalate to manual review using provider settlement and webhook evidence.
7. Emit the corresponding domain event for every manual resolution.

## Temporal workflow stuck or awaiting review

1. Query `payment-orchestration-state` and record `state`, `resume_state`, `last_error`, and attempt count.
2. Compare workflow history with Kafka records, provider state, and the business ledger.
3. Signal an authoritative event; do not edit Temporal history.
4. For a code regression, stop the ramp or route work back to the previous Worker Deployment Version.
5. Export the affected history and add it to replay regression tests before redeploying.

## Dead-letter queue

1. Classify the record as malformed contract, permanent business conflict, or exhausted infrastructure retry.
2. Use the stored consumer group, source topic, partition, offset, source message ID, and checksum to identify the original delivery.
3. Fix the producer, consumer, or dependency before replay.
4. Confirm downstream Inbox and business idempotency constraints are active.
5. Replay with original correlation and causation context plus an audited reason.
6. Monitor the source coordinate and message ID to prevent a replay loop.

## Callback delivery terminal failure

1. Locate the callback DLQ row by tenant, transaction, and immutable event ID;
   record the current outcome generation and attempt history.
2. Confirm the endpoint is still active, ownership-verified, HTTPS, and allowed
   by the production egress policy. Do not temporarily permit a private or
   unverified destination.
3. Check DNS answers, controlled-egress health, response status, and HMAC key
   version. Never copy the callback secret into a ticket or replay payload.
4. Correct the endpoint or dependency before replay. A callback failure does
   not roll back a completed payment, settlement, or commission.
5. Use the operator replay endpoint with a verified `platform_operator` token
   and an audit reason. Replay preserves the original event identity and starts
   a new outcome generation.
6. Confirm exactly one worker owns the delivery lease, then monitor attempts at
   30 seconds, 2 minutes, 10 minutes, 1 hour, and 6 hours until delivery or a
   new terminal record.

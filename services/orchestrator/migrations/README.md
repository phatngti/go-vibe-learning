# Orchestrator Migrations

These migrations own the fallback PostgreSQL Saga runtime and its transactional
inbox/outbox tables. Temporal mode stores Workflow history in Temporal; it does
not use `payment_sagas`, but it still exchanges commands and events through the
same Kafka contracts.

Apply them when `ORCHESTRATOR_ENGINE=saga`. Keeping the schema available during
a staged migration also permits rollback from Temporal to the fallback engine;
never run both engines against the same production event stream at once.

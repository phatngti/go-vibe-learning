# Migrations

Migrations are grouped by service so each service can manage its own database schema independently.

```text
services/
  user/migrations/
  payment/migrations/
  affiliate/migrations/
  settlement/migrations/
  agent/migrations/
  orchestrator/migrations/
```

Use the folder that matches the service database you are migrating. For example, payment migrations should run only against the payment database, and affiliate migrations should run only against the affiliate database.

Current database ownership:

| Service | Database | Owner user | Tables |
| --- | --- | --- | --- |
| user | `master_go_cqrs_user` | `master_go_cqrs_user_user` | `user_identities` synced from Keycloak |
| payment | `master_go_cqrs_payment` | `master_go_cqrs_payment_user` | `transactions`, `outbox_events`, `domain_events` |
| affiliate | `master_go_cqrs_affiliate` | `master_go_cqrs_affiliate_user` | affiliate state, inbox, outbox, domain events |
| settlement | `master_go_cqrs_settlement` | `master_go_cqrs_settlement_user` | balances, commission ledger, inbox, outbox, domain events |
| agent | `master_go_cqrs_agent` | `master_go_cqrs_agent_user` | configuration, callback delivery, inbox, outbox, domain events |
| orchestrator | `master_go_cqrs_orchestrator` | `master_go_cqrs_orchestrator_user` | payment sagas, inbox, outbox |

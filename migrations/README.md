# Migrations

Migrations are grouped by service so each service can manage its own database schema independently.

```text
migrations/
  app/
  payment/
  affiliate/
  settlement/
  agent/
  orchestrator/
```

Use the folder that matches the service database you are migrating. For example, payment migrations should run only against the payment database, and affiliate migrations should run only against the affiliate database.

Current database ownership:

| Service | Database | Owner user | Tables |
| --- | --- | --- | --- |
| app | `master_go_cqrs_app` | `master_go_cqrs_app_user` | `users` |
| payment | `master_go_cqrs_payment` | `master_go_cqrs_payment_user` | `transactions`, `outbox_events` |
| affiliate | `master_go_cqrs_affiliate` | `master_go_cqrs_affiliate_user` | `referrals`, `agent_metrics`, `affiliate_levels` |
| settlement | `master_go_cqrs_settlement` | `master_go_cqrs_settlement_user` | `agent_balances`, `commission_ledgers` |
| agent | `master_go_cqrs_agent` | `master_go_cqrs_agent_user` | `agent_configs`, `agent_audit_logs` |
| orchestrator | `master_go_cqrs_orchestrator` | `master_go_cqrs_orchestrator_user` | None currently |

The orchestrator folder is reserved for future workflow-owned tables.

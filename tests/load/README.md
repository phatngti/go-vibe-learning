# Payment v2 load profiles

This k6 harness encodes the agreed request-side capacity gates. It is not
production evidence until run against the production-like multi-AZ topology with
the accompanying Kafka, Outbox, Temporal, PostgreSQL, and callback dashboards.

```bash
BASE_URL=https://staging.example.com \
ACCESS_TOKEN=<short-lived-load-test-token> \
CALLBACK_ENDPOINT_ID=<verified-sink> \
LOAD_PROFILE=smoke \
k6 run tests/load/payment-v2.js
```

Profiles:

- `smoke`: 5 requests/second for 30 seconds.
- `steady`: 500 requests/second for 24 hours.
- `peak`: 2,000 requests/second for 1 hour.

Use `LOAD_DURATION` to shorten a staging rehearsal. The full acceptance run must
not override the default steady or peak durations.

The API thresholds are p95 below 200 ms and technical failure rate below 0.1%.
The asynchronous p95 below 5 seconds, backlog drain below 10 minutes, duplicate
business effects, money drift, RPO, and RTO are validated from backend metrics
and disaster-recovery tooling rather than from the HTTP client alone.


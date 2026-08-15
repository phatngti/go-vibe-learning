# Master Go CQRS application chart

This chart packages application workloads only. A separate GitOps repository is
responsible for namespaces, managed PostgreSQL, Kafka and Schema Registry,
Temporal, ingress, certificate management, External Secrets, observability,
backups, and environment-specific values.

Production values must replace every placeholder image digest and provide an
externally managed Secret referenced by `global.externalSecretName`. The chart
does not render secret values.

```bash
helm lint deploy/helm/master-go-cqrs
helm template cqrs deploy/helm/master-go-cqrs --namespace payments
```

The default NetworkPolicy denies all traffic, then permits same-namespace
application traffic, DNS, and explicitly configured external CIDRs. A GitOps
environment must add the exact ingress, service-mesh control-plane, database,
Kafka, Temporal, identity-provider, PSP, and callback-egress policies required by
its topology.


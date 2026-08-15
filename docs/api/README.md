# Generated OpenAPI specifications

This directory contains generated runtime artifacts. Do not edit them by hand.

| File | Source | Generator | Consumer |
| --- | --- | --- | --- |
| [`swagger.json`](swagger.json) | Go HTTP handler annotations and DTOs | `task gen-swagger` | REST Swagger UI |
| [`envoy.swagger.json`](envoy.swagger.json) | `contracts/proto/*.proto` HTTP annotations | `task gen-proto` | gRPC gateway Swagger UI |

Regenerate both specifications with:

```bash
task gen-docs
```

Only JSON is committed because the local Swagger deployments and k3d ConfigMaps consume JSON. A second YAML serialization would duplicate the same generated contract without adding a runtime consumer.

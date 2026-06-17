# Envoy Gateway Configuration

Envoy configuration is split into shared gateway behavior and environment
profiles.

## Layout

- `shared/routes.yaml`: route policy shared by local and production.
- `profiles/local/envoy.env`: local k3d defaults. This is the default profile.
- `profiles/prod/envoy.env`: production contract and placeholders. Deployment
  automation should override secrets and externally visible URLs.
- `envoy.tmpl.yaml`: static config template with `__PLACEHOLDER__` tokens.
- `envoy.yaml`: generated output mounted by k3d. Do not edit it directly.

## Usage

Generate local config:

```bash
task gen-envoy-config
```

Generate production config:

```bash
ENVOY_PROFILE=prod task gen-envoy-config
```

The local profile must keep `http://localhost:8888` usable without local TLS
certificates. Production must not inherit local Keycloak issuer, service DNS, or
TLS defaults.

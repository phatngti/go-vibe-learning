# ==========================================
# 1. Build Stage
# ==========================================
FROM golang:1.24.4-alpine AS builder

WORKDIR /app

# Install git and system certs (in case dependencies need it)
RUN apk add --no-cache git ca-certificates

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY docs/ ./docs/

# Build targets
FROM builder AS builder-payment
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/payment ./cmd/payment

FROM builder AS builder-affiliate
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/affiliate ./cmd/affiliate

FROM builder AS builder-settlement
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/settlement ./cmd/settlement

FROM builder AS builder-agent
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/agent ./cmd/agent

FROM builder AS builder-orchestrator
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/orchestrator ./cmd/orchestrator

FROM builder AS builder-user
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/user ./cmd/user

# ==========================================
# 2. Runtime Stage
# ==========================================
FROM alpine:latest AS runtime
WORKDIR /root/
RUN apk add --no-cache ca-certificates tzdata

# Copy service configs
COPY config/ ./config/

# Default environments
ENV CONFIG_PATH=/root/config/user.yaml

# Payment Service Target
FROM runtime AS payment
COPY --from=builder-payment /bin/payment /usr/local/bin/payment
ENV SERVICE_NAME=payment
ENV CONFIG_PATH=/root/config/payment.yaml
EXPOSE 8081 9081
ENTRYPOINT ["/usr/local/bin/payment"]

# Affiliate Service Target
FROM runtime AS affiliate
COPY --from=builder-affiliate /bin/affiliate /usr/local/bin/affiliate
ENV SERVICE_NAME=affiliate
ENV CONFIG_PATH=/root/config/affiliate.yaml
EXPOSE 8082 9082
ENTRYPOINT ["/usr/local/bin/affiliate"]

# Settlement Service Target
FROM runtime AS settlement
COPY --from=builder-settlement /bin/settlement /usr/local/bin/settlement
ENV SERVICE_NAME=settlement
ENV CONFIG_PATH=/root/config/settlement.yaml
EXPOSE 8083 9083
ENTRYPOINT ["/usr/local/bin/settlement"]

# Agent Service Target
FROM runtime AS agent
COPY --from=builder-agent /bin/agent /usr/local/bin/agent
ENV SERVICE_NAME=agent
ENV CONFIG_PATH=/root/config/agent.yaml
EXPOSE 8084 9084
ENTRYPOINT ["/usr/local/bin/agent"]

# Orchestrator Worker Target
FROM runtime AS orchestrator
COPY --from=builder-orchestrator /bin/orchestrator /usr/local/bin/orchestrator
ENV SERVICE_NAME=orchestrator
ENV CONFIG_PATH=/root/config/orchestrator.yaml
ENTRYPOINT ["/usr/local/bin/orchestrator"]

# User / HTTP API Target
FROM runtime AS user
COPY --from=builder-user /bin/user /usr/local/bin/user
ENV SERVICE_NAME=user
ENV CONFIG_PATH=/root/config/user.yaml
EXPOSE 8889
ENTRYPOINT ["/usr/local/bin/user"]

# k8s-pod-event-collector

Send Slack alerts when Kubernetes Pods **restart**, **complete**, or are **evicted**.

## Features
- Watches Pods with shared informers (client-go)
- Detects:
  - Restart (any container's RestartCount increases)
  - Completion (Pod phase transitions to Succeeded)
  - Eviction (Pod status reason == Evicted)
- Collects recent Events and previous container logs (when available)
- Sends a concise Slack message with context

## Quick start (local)
```bash
export SLACK_WEBHOOK_URL=https://hooks.slack.com/services/XXXXX/XXXXX
export CLUSTER_NAME=dev-cluster
go run ./cmd/main.go
```

## Build & containerize
```bash
go mod tidy
go build ./cmd/main.go

# Example Dockerfile (multi-stage)
# docker build -t ghcr.io/yourorg/k8s-pod-event-collector:v0.1.0 .
```

## Helm install (cluster)
```bash
helm upgrade --install k8s-pod-event-collector ./helm/k8s-pod-event-collector \
  --set env.clusterName="prod-cluster" \
  --set env.slackWebhookUrl="https://hooks.slack.com/services/CHANGE/ME"
```

## Configuration (env vars)
- `SLACK_WEBHOOK_URL` (required)
- `CLUSTER_NAME` (optional; default: `unknown-cluster`)
- `ALERT_ON_RESTART` (true/false; default true)
- `ALERT_ON_COMPLETION` (true/false; default true)
- `ALERT_ON_EVICTION` (true/false; default true)
- `MUTE_SECONDS` (int; default 120)
- `IGNORE_EXIT_CODE_ZERO` (true/false; default true)

## RBAC
The chart grants read access to Pods, Events, Nodes and Pod logs.

## Notes
- Previous logs are fetched with `Previous=true` when available.
- Mute window prevents alert storms for noisy pods.

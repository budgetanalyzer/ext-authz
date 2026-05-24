# ext-authz

Envoy/Istio HTTP external authorization service for Budget Analyzer.

The service validates browser sessions at the ingress authorization layer. It
reads the shared Redis session hash written by Session Gateway, authorizes
requests based on the session state, and returns identity headers for the
downstream API path.

## Local Commands

```bash
go test ./...
docker build -t ext-authz:local .
```

Run `gofmt -w *.go` before committing Go source changes.

## Configuration

| Variable | Purpose | Default |
|----------|---------|---------|
| `HTTP_PORT` | HTTP ext_authz listener port. | `9002` |
| `HEALTH_PORT` | Health listener port. | `8090` |
| `REDIS_ADDR` | Redis host and port. | `redis.infrastructure:6379` |
| `REDIS_USERNAME` | Redis ACL username. | empty |
| `REDIS_EXT_AUTHZ_PASSWORD` | Redis ACL password for this service. | empty |
| `REDIS_TLS` | Enables TLS for Redis when set to `true`. | `false` |
| `REDIS_CA_CERT` | CA certificate path for Redis TLS. | empty |
| `SESSION_COOKIE_NAME` | Browser session cookie name. | `BA_SESSION` |
| `SESSION_KEY_PREFIX` | Redis key prefix for sessions. | `session:` |
| `LOG_FORMAT` | Logger format, `json` or `text`. | `json` |

`LOG_LEVEL` is also supported for local diagnostics.

## Ownership

This repository owns the Go source, Go module metadata, Dockerfile, tests, and
service-local documentation for ext-authz.

The sibling `../orchestration` repository owns Kubernetes manifests, Istio
`ext_authz` wiring, Redis ACL bootstrap, network policies, production image
pinning, release metadata, and deployment documentation.

The shared browser session contract is documented in
`../orchestration/docs/architecture/session-edge-authorization-pattern.md`.

## Release

Release images publish from
`.github/workflows/publish-release.yml` in this repository to
`ghcr.io/budgetanalyzer/ext-authz`.

The orchestration repository consumes released image digests in its production
desired state and builds the local Tilt image from `../ext-authz`. Deployment
and production runbooks live in `../orchestration/deploy/README.md` and
`../orchestration/kubernetes/production/README.md`.

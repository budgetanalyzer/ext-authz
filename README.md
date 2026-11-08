# ext-authz

Envoy/Istio HTTP external authorization service for Budget Analyzer.

The service validates browser sessions at the ingress authorization layer. It
reads the shared Redis session hash written by Session Gateway, authorizes
requests based on the session state, and returns identity headers for the
downstream API path.

## Session Key Derivation

The Redis key is `SESSION_KEY_PREFIX` followed by the lowercase hex SHA-256 of
the UTF-8 session cookie value, matching Session Gateway. The raw cookie value
never appears in a Redis key. For example, the token
`3f2504e0-4f89-41d3-9a0c-0305e82c3301` resolves to
`session:16362f566387b3cf5a6e92fb0a986c76ca20eb3a0c12cbdfbd0b29501e0c18df`.
There is no fallback to raw-token keys; after switching to hashed keys, flush
existing sessions with `../orchestration/scripts/ops/flush-redis.sh`.

## Authorization Responses

Valid sessions receive the existing `200` response with `X-User-Id`, `X-Roles`,
and `X-Permissions` identity headers. Denials use the minimal shared
`ApiErrorResponse` JSON wire shape:

| Condition | Status | Response |
|-----------|--------|----------|
| Missing, blank, unknown, or expired session | `401` | `{"type":"UNAUTHORIZED","message":"Authentication required"}` |
| Session lookup, parsing, Redis, or other infrastructure failure | `503` | `{"type":"SERVICE_UNAVAILABLE","message":"Service Unavailable"}` |

Authentication failures intentionally share one generic response so callers
cannot distinguish session state. Operational failures remain fail-closed but
are not misclassified as invalid credentials.

The JSON bodies above are what this service returns to Envoy. A `503` from this
service reaches clients as an empty `503` with no `content-type` from the
ingress. The Istio `ext_authz` provider treats authorizer 5xx responses,
timeouts, and unreachable authorizers as errors, answers with
`statusOnError: "503"` while failing closed, and discards the authorizer body.
The provider configuration is owned by
`../orchestration/kubernetes/istio/istiod-values.yaml`.

The Go response representation is local to this service and does not depend on
service-common binaries. Its `type` and `message` fields must remain aligned
with the shared `ApiErrorResponse` contract documented in
[`../service-common/docs/error-handling.md`](../service-common/docs/error-handling.md).
The session storage and edge authorization contract remains owned by
[`../orchestration/docs/architecture/session-edge-authorization-pattern.md`](../orchestration/docs/architecture/session-edge-authorization-pattern.md).

## Runtime Health

The health listener exposes two distinct checks:

| Endpoint | Meaning | Redis dependency |
|----------|---------|------------------|
| `/livez` | The ext-authz process is running. | None |
| `/readyz` | The service can reach Redis and authorize requests. | Redis `PING` |

The health listener starts before the first Redis connection attempt. If Redis
is unavailable, `/livez` remains healthy, `/readyz` returns `503`, and session
lookups continue to fail closed with the authorization response documented
above. Redis connection attempts continue in the background with capped
backoff; exhausting a fixed startup retry count cannot terminate the process.

Both the health and authorization HTTP servers bound header reads, request
reads, response writes, and idle connections. Their write timeout exceeds the
Redis readiness call budget.

## Local Commands

The application supports the Go 1.27 release line. Build CI reads this version
from `go.mod`, and the Docker builder uses the same release line.

```bash
go test ./...
docker build -t ext-authz:local .
```

Run `gofmt -w *.go` before committing Go source changes.

## Dependency Automation

Production Renovate discovery and the reachable Go vulnerability check on
`main` are described in
[`docs/dependency-automation.md`](docs/dependency-automation.md). Dependency
upgrades remain review-owned and are never automerged.

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
| `SESSION_COOKIE_NAME` | Browser session cookie name. Must match Session Gateway. | `__Host-BA_SESSION` |
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

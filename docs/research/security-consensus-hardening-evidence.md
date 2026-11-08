# Security Consensus Hardening Evidence

## 2026-11-07 — Service runtime Phase 1

The ext-authz health contract now separates process liveness from Redis
readiness. Unit tests establish that `/livez` returns `200` without pinging
Redis, `/readyz` returns `503` when Redis is unavailable, and the removed
`/healthz` path returns `404`. The health socket is bound before the first Redis
ping can run.

Redis client construction no longer requires a successful connection. The
background connection monitor uses a 500 ms linear backoff capped at 5 seconds
and has no retry limit; a test observes recovery on attempt 12, beyond the
former ten-attempt limit. Existing authorization tests continue to establish a
fail-closed `503` response for an unexpected session-store failure.

Both HTTP servers use these finite connection bounds:

| Timeout | Value |
|---------|-------|
| Read header | 5 seconds |
| Read | 10 seconds |
| Write | 30 seconds |
| Idle | 60 seconds |

The 30-second write timeout exceeds the two-second Redis readiness-call budget.

Validation from the ext-authz repository:

```text
go test ./...       PASS (github.com/budgetanalyzer/ext-authz, 0.004s)
go test -race ./... PASS (github.com/budgetanalyzer/ext-authz, 1.013s)
git diff --check    PASS
```

Deployment probe changes are intentionally deferred to service-runtime Phase 2
in the orchestration repository.

## 2026-11-07 — Service runtime Phase 3

The application toolchain now uses the supported Go 1.27 release line.
`go.mod` declares Go 1.27, Build CI continues to read that declaration through
`actions/setup-go`'s `go-version-file`, and the pinned Docker builder remains
`golang:1.27-alpine`. The Build workflow's action references remain full-SHA
pinned. The separate vulnerability-analysis toolchain remains independently
managed as documented in `docs/dependency-automation.md`.

Validation from the ext-authz repository, using local Go 1.27.3:

```text
gofmt -w *.go                  PASS (no additional formatting changes)
go mod tidy                    PASS (removed stale go-redis v9.7.0 checksums;
                                     no dependency versions changed)
go test ./...                  PASS (github.com/budgetanalyzer/ext-authz, 0.004s)
go test -race ./...            PASS (github.com/budgetanalyzer/ext-authz, 1.013s)
docker build -t ext-authz:local .
                                PASS (image built with pinned Go 1.27 builder)
gofmt -l .                     PASS (no output)
git diff --check               PASS
toolchain and action-pin checks PASS
```

This phase changes no service behavior or deployment wiring.

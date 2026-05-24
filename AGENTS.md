# Budget Analyzer - ext-authz

## Tree Position

**Archetype**: service
**Scope**: ext-authz service implementation
**Role**: Envoy/Istio HTTP external authorization service for Budget Analyzer

### Relationships

- **Coordinated by**: sibling `../orchestration`
- **Consumes**: Redis browser session hashes written by `../session-gateway`
- **Peers with**: sibling Budget Analyzer service repositories

### Permissions

- **Read**: sibling documentation in `../orchestration/docs/`
- **Write**: this repository only
- **Do not write**: orchestration manifests from this service repo except docs
  or configuration when explicitly coordinating deployment wiring

### Discovery

```bash
# Service source
find . -maxdepth 1 -type f | sort

# Deployment source of truth
find ../orchestration/kubernetes/services/ext-authz -type f | sort

# Shared browser session contract
sed -n '1,220p' ../orchestration/docs/architecture/session-edge-authorization-pattern.md
```

## Source Of Truth

- Deployment, Istio wiring, Redis ACLs, network policies, and production release
  integration: `../orchestration`
- Shared browser session contract:
  `../orchestration/docs/architecture/session-edge-authorization-pattern.md`
- Release image workflow: `.github/workflows/publish-release.yml`
- Local service implementation: this repository

## Development Workflow

Use standard Go tooling:

```bash
gofmt -w *.go
go mod tidy
go test ./...
docker build -t ext-authz:local .
```

Do not change the Redis session schema, cookie name, identity headers, ports,
probes, or Istio fail-closed behavior without coordinating the orchestration
repository documentation and manifests.

## Code Exploration

Use direct shell discovery commands such as `rg`, `find`, `sed`, and `go test`.
Do not use Agent/subagent tools for code exploration.

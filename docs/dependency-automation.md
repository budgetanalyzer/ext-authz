# Dependency Automation

**Status:** Local configuration is prepared; automation is not active.

The Budget Analyzer
[dependency automation policy](https://github.com/budgetanalyzer/orchestration/blob/main/docs/dependency-automation.md)
owns activation, scheduling, review, and cost boundaries. Renovate is the only
update-PR owner. Dependabot remains alert-only, and neither routine nor security
updates are automerged.

## Update discovery

`renovate.json` extends the shared orchestration preset. Native Renovate managers
cover the Go directive and direct and indirect modules in `go.mod`, both pinned
Dockerfile images, and GitHub Actions. A repository-specific regex manager covers
the `GOVULNCHECK_VERSION` value in
`.github/workflows/go-vulnerability-check.yml` as the Go module
`golang.org/x/vuln`.

The existing `build.yml` workflow remains the pull-request check for formatting,
tests, and the service image build. Dependency automation does not bypass those
checks. The shared preset must be published before this consumer configuration is
activated. This onboarding does not update `go.mod`, `go.sum`, the Go directive,
container images, Actions, or application code.

A credential-free local run with Renovate 44.65.5 under Node 24 extracted 21
occurrences from six files: two Dockerfile dependencies, 14 GitHub
Actions/runner/Go inputs, four `go.mod` entries, and one regex-managed scanner
pin. The module entries were `go`, `github.com/redis/go-redis/v9`,
`github.com/cespare/xxhash/v2`, and `github.com/dgryski/go-rendezvous`. Renovate
recognizes the latter two as indirect and leaves their standalone updates
disabled; they remain represented in the module graph and can change through a
reviewed direct-module update.

The local lookup independently found go-redis 9.7.3 as the maintained-line patch
and exposed later v9 release lines through 9.22.0. It also resolved the
distroless runtime image and the current scanner module. The Go builder image was
extracted, but Renovate's anonymous Docker Hub client returned `no-result` and an
anonymous Docker-token warning. GitHub Actions and the workflow's Go 1.26.x input
were extracted but skipped because no GitHub token was supplied. These incomplete
lookups are retained for the hosted Phase 12 retry. Extraction completed without
an exception or fatal log; its only error-level entry was the documented
missing-GitHub-token condition.

## Reachable vulnerability evidence

`go-vulnerability-check.yml` runs after changes reach `main`, every Monday, and
on manual dispatch. It installs the pinned official `govulncheck` CLI and runs
both `govulncheck ./...` and `govulncheck -json ./...`. The workflow keeps the
readable report, JSON event stream, stderr, scanner version, and exit-code summary
for seven days.

Reachable findings are reported and uploaded without failing unrelated work.
Scanner installation, package loading, vulnerability database access, analysis,
JSON generation or validation, empty required evidence, and artifact upload
failures still fail the job. In particular, the text command's exit code 3 means
findings, while any other unexpected text status or any nonzero JSON status is an
operational error.

The application still declares Go 1.24. The current pinned scanner,
`golang.org/x/vuln` v1.7.0, requires Go 1.25 or newer, so this workflow uses Go
1.26.x solely as its analysis toolchain and sets `GOTOOLCHAIN=local` to prevent an
implicit toolchain change. That does not upgrade the application toolchain or its
builder image.

### Local baseline evidence

On 2026-09-06, govulncheck v1.7.0 running with Go 1.26.8 reproduced
GO-2025-3540 as a reachable vulnerability in
`github.com/redis/go-redis/v9` 9.7.0. The fixed version reported by the scanner is
9.7.3. A representative trace is:

```text
session.go:81:24: ext.NewSessionStore calls redis.cmdable.Ping,
which eventually calls redis.baseClient.initConn
```

The scanner reported one reachable vulnerability from one module and no other
vulnerabilities in imported packages or required modules. This call-path result
is stronger evidence than module presence alone and reproduces the preserved
September 6 review. No remediation upgrade is part of this onboarding phase.

Run the same local check with an explicit analysis toolchain and pinned scanner:

```bash
scanner_bin="$(mktemp -d)"
scan_output="$(mktemp -d)"
GOBIN="${scanner_bin}" GOTOOLCHAIN=go1.26.8 \
  go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
PATH="${scanner_bin}:${PATH}" GOTOOLCHAIN=go1.26.8 \
  ./scripts/run-govulncheck.sh "${scan_output}"
printf 'Evidence: %s\n' "${scan_output}"
```

The explicit local analysis toolchain may download that public Go release when
it is not cached. It does not alter `go.mod` or the Dockerfile builder version.

## Phase 12 operator handoff

The following evidence remains pending because this phase neither publishes
configuration nor uses credentials:

- Repository: `budgetanalyzer/ext-authz`. Check: a hosted Renovate full dry run
  after the shared preset is published. Scope: `go.mod`, both Dockerfile images,
  all workflow Actions and Go inputs, and the govulncheck pin. Reason: published
  preset resolution and GitHub lookups require the Phase 12 operator context;
  the local anonymous lookup also returned `Failed to look up docker package
  golang: no-result`. Action: run the trusted read-only hosted validation and
  then onboard the free Renovate App, retrying the Go builder lookup through the
  hosted network. Proof: preserve the run URL and logs showing resolved policy,
  extracted package identities, successful lookups including `golang:1.24-alpine`,
  and no fatal/error output. If the hosted Docker lookup still fails, retain it
  as a coverage gap instead of activating with an assumed result.
- Repository: `budgetanalyzer/ext-authz`. Check: manually dispatch
  `go-vulnerability-check.yml` from the trusted default branch. Scope:
  `github.com/redis/go-redis/v9` 9.7.0, the Go module graph, scanner module, and
  vulnerability database. Reason: the workflow is not published or activated
  during local preparation. Action: dispatch it without exposing its ephemeral
  GitHub token. Proof: preserve the run URL, source revision, and seven-day
  artifact containing complete readable and JSON output; until remediation, it
  should retain the GO-2025-3540 call path above.
- Repository: `budgetanalyzer/ext-authz`. Check: GitHub dependency graph and
  Dependabot alert status. Scope: the Go module graph and vulnerability alerts.
  Reason: repository settings are administrator-owned. Action: enable the graph
  and alerts, keep Dependabot version updates and overlapping security-update PRs
  disabled, and grant Renovate alert-read access. Proof: sanitized settings and
  alert evidence tied to this repository.

Renovate discovery, Dependabot alerts, and govulncheck do not prove support
lifecycle status. Go support remains part of the quarterly upstream lifecycle
review, and the application Go 1.24 migration requires separate coordination.

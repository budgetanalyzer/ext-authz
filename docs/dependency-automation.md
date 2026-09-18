# Dependency Automation

**Status:** Production configuration targets `main`.

The Budget Analyzer
[dependency automation policy](https://github.com/budgetanalyzer/orchestration/blob/main/docs/dependency-automation.md)
owns scheduling, review, and cost boundaries. Renovate is the only update-PR
owner. Dependabot remains alert-only, and neither routine nor security updates
are automerged.

## Update discovery and review

`renovate.json` extends the shared orchestration production preset. Native
Renovate managers cover the Go directive and direct and indirect modules in
`go.mod`, both pinned Dockerfile images, and GitHub Actions. A repository-specific
regex manager covers the `GOVULNCHECK_VERSION` value in
`.github/workflows/go-vulnerability-check.yml` as the Go module
`golang.org/x/vuln`.

Renovate proposals target `main` and must pass the existing Build workflow. That
workflow runs for pushes and pull requests to `main` and for manual dispatches,
using the normal Go module cache before checking formatting, running tests, and
building the service image. Review each proposal for release notes, compatibility,
and expected build and scanner behavior; a successful bot lookup or build is not
approval to merge.

This configuration does not itself update `go.mod`, `go.sum`, the Go directive,
container images, Actions, or application code. The shared preset and repository
configuration determine discovery and grouping; dependency changes remain
separate reviewed pull requests.

## Reachable vulnerability evidence

`go-vulnerability-check.yml` runs on pushes to `main`, on its Monday schedule,
and by manual dispatch. It installs the pinned official `govulncheck` CLI and
uses `scripts/run-govulncheck.sh` to run both `govulncheck ./...` and
`govulncheck -json ./...`. The wrapper records the readable report, JSON event
stream, stderr, scanner version, and exit-code summary.

Reachable findings are reported and retained without becoming a merge gate.
Scanner installation, package loading, vulnerability database access, analysis,
JSON generation or validation, empty required evidence, archive preparation, and
artifact upload failures still fail the job. In particular, the text command's
exit code 3 means findings, while any other unexpected text status or any nonzero
JSON status is an operational error.

Every run passes the complete `govulncheck-results` directory to
`.github/scripts/prepare-dependency-evidence.sh`. The helper creates one gzip
archive from only the allowlisted path and rejects a compressed payload above
25,165,824 bytes (24 MiB). The workflow uploads that already-compressed archive
with action compression disabled and retains it for seven days. Do not trim
scanner output to fit the cap; investigate unexpected growth instead.

The application declares its own Go version in `go.mod`. The scanner workflow
uses the separate analysis toolchain declared in the workflow and sets
`GOTOOLCHAIN=local`, preventing an implicit application toolchain change. Updating
the analysis toolchain or scanner pin does not by itself update the application's
Go directive or Docker builder image.

### Reachability limits

`govulncheck` analyzes reachable Go call paths in the checked-out source and
resolved module graph using the vulnerability database available at run time. A
clean report does not cover operating-system or container packages, code paths
outside the analyzed packages, future revisions, or vulnerabilities absent from
the database. Network or database failures are operational errors, not clean
results. Keep dependency graph and Dependabot alerts enabled as complementary
signals, with Dependabot version and security-update pull requests disabled to
avoid competing with Renovate.

### Local reproduction

Install the workflow's pinned scanner with an explicit analysis toolchain, then
run the same durable wrapper:

```bash
scanner_bin="$(mktemp -d)"
scan_output="$(mktemp -d)"
GOBIN="${scanner_bin}" GOTOOLCHAIN=go1.26.8 \
  go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
PATH="${scanner_bin}:${PATH}" GOTOOLCHAIN=go1.26.8 \
  ./scripts/run-govulncheck.sh "${scan_output}"
printf 'Evidence: %s\n' "${scan_output}"
```

The explicit analysis toolchain may download that public Go release when it is
not cached. It does not alter `go.mod` or the Dockerfile builder version. Review
the text call paths together with `scan-result.json`; findings are actionable
review evidence, while an `operational-error` result means the scan must be
repeated after correcting the scanner or environment failure.

Renovate discovery, Dependabot alerts, and `govulncheck` do not prove support
lifecycle status. Go support remains part of the quarterly upstream lifecycle
review, and an application toolchain migration requires separate coordination.

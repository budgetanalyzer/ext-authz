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

### Image build check

`build.yml` still builds the service image with a native amd64 `docker build`
on every pull request. That build does not prove the release platform.

`.github/workflows/image-build-check.yml` (`Image Build Check`) runs on pull
requests to `main` that change `Dockerfile`, `go.mod`, `go.sum`, or the
workflow itself, and on manual dispatch. Go source is excluded because the
`Build` workflow already compiles, tests, and builds it on every pull request.
The check builds the same `./Dockerfile` and repository-root context as
`publish-release.yml` for `linux/arm64`, proving the arm64 builder and
distroless base manifests and the emulated module download and `go build`.
The builder stage does not use `$BUILDPLATFORM`, so the build runs under QEMU
emulation and has a 30-minute timeout.

The check does not log in to a registry, tag, push, or cache images, and passes
no secrets. Neither the amd64 nor the arm64 build proves runtime behavior,
Istio external-authorization wiring, or Redis session handling, and neither
scans for vulnerabilities.

Do not make this check a required status check while it is path-filtered:
pull requests outside the filter never run it, so a required check would wait
indefinitely.

This configuration does not itself update `go.mod`, `go.sum`, the Go directive,
container images, Actions, or application code. The shared preset and repository
configuration determine discovery and grouping; dependency changes remain
separate reviewed pull requests.

## Reachable vulnerability evidence

`go-vulnerability-check.yml` runs on pushes to `main`, on every pull request
targeting `main`, on its Monday schedule, and by manual dispatch. The
pull-request trigger has no path filter because reachability depends on
application source as well as modules. Each pull request has its own concurrency
group, so a newer push cancels that pull request's in-progress scan; pushes to
`main`, scheduled runs, and manual runs each get a unique group and are never
cancelled. The workflow uses no secrets, so fork pull requests run it too. It
installs the pinned official `govulncheck` CLI and
uses `scripts/run-govulncheck.sh` to run both `govulncheck ./...` and
`govulncheck -json ./...`. The wrapper records the readable report, JSON event
stream, stderr, scanner version, and exit-code summary.

Reachable findings are reported in the job summary and retained in the evidence
artifact without becoming a merge gate, including on pull requests: a pull
request with reachable findings still passes this workflow. Scanner installation, package loading, vulnerability database access, analysis,
JSON generation or validation, empty required evidence, and artifact upload
failures still fail the job. In particular, the text command's
exit code 3 means findings, while any other unexpected text status or any nonzero
JSON status is an operational error.

Every run uploads the `govulncheck-results` directory directly as the single
`govulncheck-<sha>` artifact and retains it for seven days. `<sha>` is
`github.sha`: the pushed commit on `main`, and the pull request's temporary
merge commit, not its head commit, on pull-request runs. A successful run
must include the scanner metadata and complete human-readable and JSON reports;
incomplete successful-run evidence fails under the shared orchestration policy.
The always-run upload step retains diagnostics produced before a failed scan,
while the original error keeps the workflow failed.

Because this workflow is not path-filtered, every pull request to `main` reports
it, so it could later be made a required status check without leaving pull
requests waiting. Whether to require it is a separate branch-protection
decision.

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

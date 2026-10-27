# Remediation verification

## Phase 14 — security baseline and compatibility safeguards

Date: 2026-10-27 (Asia/Bangkok). Baseline: develop at 1a75cef.

### Changes

- Go module/development/CI baseline: 1.27.3. Local base installation was not
  overwritten; Go downloaded the required version into its toolchain cache.
- x/text v0.41.0, with x/sync v0.22.0 required by its module graph. Other direct
  runtime dependencies and Argon2id parameters are unchanged.
- Both builder images: official public.ecr.aws/docker/library/golang:1.27.3-alpine
  at index sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb.
- govulncheck pinned at v1.9.0; CI and scripts/check-security.ps1 scan source
  and the two built binaries. No .env loading or application DB access is
  needed for the security script.
- Production/fulfillment response tests freeze legacy keys, empty arrays,
  exact decimal strings, Worker redaction and full-state command responses.
- Real repository tests compare complete serialized original/replayed states
  after later production completion or payment review, not just version fields.

### Checks actually executed

| Check | Result |
| --- | --- |
| go mod tidy; go build ./... | Pass with Go 1.27.3 |
| go vet ./... | Pass |
| go test ./internal/production ./internal/delivery | Pass |
| go test -race -cover ./... with integration DSNs cleared | Pass; DB tests correctly skipped in this unit-only run |
| scripts/check-security.ps1 | Pass: source and Windows API/worker, zero reachable advisories |
| Dockerfile.api and Dockerfile.worker builds | Pass |
| go version -m on actual Linux image binaries | Both Go 1.27.3; x/text v0.41.0 |
| govulncheck -mode=binary on both extracted Linux binaries | Pass: zero reachable advisories |
| scripts/verify-uc9.ps1, isolated sa-lathe-uc9-b06047c3 | Exit 0; real repository tests pass, including new replay equality assertions |
| Migration 1-12, empty 12/11/10 down and up, legacy Worker contacts | Pass in disposable databases |
| Docker authenticated UC1-UC9 flow, inbox worker, graceful stop | UC9_DOCKER_FLOW_PASSED, PHASE12_VERIFICATION_PASSED, WORKER_CONTACTS_VERIFICATION_PASSED |

The scanner also reports advisories in required modules whose vulnerable
packages/symbols are not imported/reachable (20 in the Windows/source scans,
19 in the Linux binary scans at verification time). These are not hidden or
described as confirmed remote exploits. A clean reachability scan does not
prove the absence of all vulnerabilities. Re-run against the current public
vulnerability database before release.

### Safety and limitations

All DB/S3 mutations occurred in freshly created verification resources. The
verifier removed its own containers, network and two volumes after success;
existing data/settings were preserved. Temporary image-inspection containers
never started and were removed. Ignored binaries and verification image tags
remain available locally. No real LINE message, main push or deployment.

No application query optimization or shop API latency claim is part of this
phase. Phase 15 measurement/integration-CI work and phases 16-21 remain pending.
Commit groups: dc035d8 (security baseline), 53fa00d (contracts/replay),
1b83205 (scanner automation), followed by this verification/checklist checkpoint.
The phase checkpoint must be pushed and its remote tip verified before claiming
publication; the agent handoff records the actual verified commit.

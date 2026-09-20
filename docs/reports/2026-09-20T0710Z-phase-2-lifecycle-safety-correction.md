# Phase 2 — lifecycle safety correction

## Header

- Roadmap phase: Phase 2, correction before first real lifecycle execution.
- Branch: `phase-2/upstream-lifecycle`.
- Start HEAD: `5a7e149d30eb4c638e9068f118eb5fdfc5f75565`.
- Prior Source CI: `35495510862` PASS; its coverage did not establish the properties below.

## Audit findings

| Finding | Severity | Required correction |
|---|---|---|
| P2-PROC-TOCTOU-001 | HIGH | Stop must verify and terminate through one process handle. |
| P2-PROC-UNVERIFIABLE-001 | HIGH | Only genuine process absence may recover stale state. |
| P2-LIFECYCLE-PROBES-001 | HIGH | Add owner-aware listener inspection and L0/L1/L2 probes. |
| P2-CONFIG-PREFLIGHT-001 | HIGH | Revalidate semantic current config immediately before launch. |
| P2-PROC-CONTEXT-001 | MEDIUM | Successful child lifetime must not inherit the startup context. |
| P2-PROC-RECORD-001 | MEDIUM | Use a strict non-secret record codec without absolute image paths. |
| P2-BINARY-RECOVERY-001 | MEDIUM | Add strict install transaction marker and bounded recovery. |

## Scope and gate

This correction changes no OAuth, provider credential or request, IDE configuration, or real product runtime state. `DUALPOOL_RUN_REAL_LIFECYCLE=1` remains prohibited until this correction is pushed and its Source CI passes. Real evidence and checklist movement remain open.

## Local verification

`go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/poolbridge`, `go mod verify`, `node --test scripts/*.test.cjs`, and `node scripts/phase0-upstream-lock.cjs` passed. Race and hosted Source CI remain pending for this correction. No real lifecycle gate ran.

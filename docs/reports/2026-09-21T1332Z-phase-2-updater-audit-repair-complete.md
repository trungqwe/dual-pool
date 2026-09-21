# Phase 2 updater audit repair

## Header

- Run ID: `20260921T1332Z-phase-2-updater-audit-repair`.
- Date/time UTC: `2026-09-21T13:32Z`.
- Roadmap phase: Phase 2.
- Repository: `trungqwe/dual-pool`.
- Branch: `phase-2/updater-audit-repair`.
- Start HEAD: `33e74f1ae006deecd44ae9d805b121e490fe71d0`.
- Implementation HEAD: `20ef4d6f78f3f0747c4983a89f791a617e539859`.
- Delivery HEAD/Source CI: pending documentation commit and exact-SHA CI.

## Assigned objective

Close UPD-AUDIT-001 lifecycle convergence and UPD-AUDIT-002 Windows marker
publication/load/remove object-identity repair using synthetic TEMP fixtures.

## Non-goals

No installed-slot registry, `instance.Manager` active-slot integration, real
updater, product-root mutation, provider traffic, OAuth, credential access,
listener, IDE configuration, binary replacement, or `upstream.lock` change.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| Native rename uses the correct NT namespace and no-replace semantics | VERIFIED | `NtSetInformationFile`, `FileRenameInformation=10`; `TestP2UPDMarkerNTRenamePrimitive*` | Same open handle retains identity; existing canonical marker is not replaced. |
| Created marker ACL is checked on the returned object | VERIFIED | `winacl.Manager.InspectHandle`; `internal/winacl` count=20 | No pathname reinspection between `CREATE_NEW` and ACL validation. |
| Sharing contention is bounded and fail closed | VERIFIED | transient/permanent sharing tests count=20 | Sharing errors never become marker absence. |
| Removal targets the inspected object | VERIFIED | disposition and replacement tests count=20 | No validate-path/delete-replacement TOCTOU. |
| File content and buffered file information are flushed | VERIFIED | pre-rename and post-rename `os.File.Sync` | Publication completes both flush boundaries before lifecycle mutation. |
| Physical persistence under sudden power loss | UNKNOWN | Microsoft documents `FlushFileBuffers`; no physical power-cut test | No absolute power-loss claim is made. |

Primary references:

- [Microsoft FILE_DISPOSITION_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_disposition_info)
- [Microsoft SetFileInformationByHandle](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfileinformationbyhandle)
- [Microsoft FlushFileBuffers](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers)
- [Microsoft MoveFileExW](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexw)

## Changes

| Path | Change | Reason |
|---|---|---|
| `internal/update/marker_windows.go` | Native no-replace rename, bounded contention, exact disposition ABI, post-rename flush, unambiguous close ownership | Preserve exact object identity and fail-closed recovery. |
| `internal/update/correction_test.go` | Rename, identity, sharing, deletion and lifecycle regressions | Deterministic TEMP proof without antivirus timing. |
| `internal/winacl/winacl.go` | Inspect newly created file by handle | Remove creation-time pathname TOCTOU. |
| `internal/winacl/winacl_test.go` | Exact handle and replacement-blocking checks | Prove protected creation behavior. |

## Verification

| Test/command | Result |
|---|---|
| Native rename primitive `-count=100` | PASS |
| Marker/fault focused `-count=20` | PASS |
| `go test -count=10 ./internal/update` | PASS |
| `go test -count=100 ./internal/update` | PASS, 126.404s |
| `go test -race -count=10 ./internal/update` | PASS, 14.609s |
| `go mod verify`; `go vet ./...`; `go test -count=1 ./...` | PASS |
| `go test -race -count=1 ./...` | PASS |
| Windows executable build | PASS |
| `node --test scripts/*.test.cjs` | PASS, 55/55 |
| `node scripts/phase0-upstream-lock.cjs` | PASS |

## Security/privacy review

Evidence contains no absolute local path, SID, marker bytes, credential target,
provider/auth/session data, or secret. All marker/state/version inputs were
synthetic and all filesystem mutations were under TEMP roots.

## Acceptance evaluation

UPD-AUDIT-001 and UPD-AUDIT-002 are E3 synthetic component PASS locally.
Production updater acceptance, FR-025, INV-PROC-05 production acceptance,
installed-slot registry, production multi-version update, and the real updater
gate remain OPEN. Final delivery additionally requires exact-SHA Source CI PASS
for the documentation/evidence head.

## Rollback

Revert the scoped repair commits in reverse order. No product or credential
state requires restoration.

## Next run

After exact-SHA Source CI PASS, the sole next objective is Phase 2 trusted
installed-slot registry plus `instance.Manager` active-selection integration in
a separate worktree/run.

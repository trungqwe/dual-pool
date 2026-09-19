# Phase 1 lock handle race correction

## Header

- Finding: `LOCK-HANDLE-001` (HIGH)
- Roadmap phase: Phase 1 — Foundation and state safety
- Branch/worktree: `phase-1/state-foundation`, dedicated clean worktree
- Starting HEAD: `1b17811154d30858477dfe62223a8799f9561e34`
- Objective: eliminate pathname deletion races during canonical stale-lock reclamation and release.

## Vulnerable sequence

The prior implementation reread canonical bytes and owner identity before calling `os.Remove(canonicalPath)`. A competing reclaimer could delete the stale object, install a new live lock, and then have the first reclaimer's pathname delete remove that new object.

## Planned handle ownership protocol

- Install a fully written and verified sibling candidate with `MoveFileExW` without replacement.
- Open the canonical lock with `CreateFileW`, `GENERIC_READ | DELETE`, share mode `0`, `OPEN_EXISTING`, and normal attributes.
- Verify exact record bytes through that retained handle before returning a Guard.
- Claim an existing canonical object with the same exclusive handle, read and classify through that handle, repeat owner inspection while retaining it, and delete only that object through `SetFileInformationByHandle(FileDispositionInfo)`.
- Release through the Guard's retained ownership handle using the same delete disposition; never delete a canonical lock by pathname.
- Do not set delete-on-close during ordinary ownership. Process exit closes the handle and leaves stale metadata for verified recovery.

## Test strategy

Add failing regression tests first for retained-handle contention, handle-backed release, simultaneous global and per-file stale reclaim, bounded stress iterations, and the invariant that exactly one returned Guard is authoritative. Preserve PID reuse, image mismatch, live/expired live owner, unverifiable/invalid record, Store CAS-window, recovery race, fault matrix and crash subprocess coverage.

## Non-goals

No secret store, DPAPI, Credential Manager, config mutation, process supervisor, ports, OAuth, accounts, provider/IDE integration, or product-root initialization. Filesystem and process tests remain TEMP-only and test-owned.

## Stop conditions

Stop on any canonical pathname deletion in stale reclaim/release, more than one simultaneous Guard, handle leak, weakened PID/reparse checks, changed Store unsafe-artifact contract, failed existing Store recovery gate, privacy leakage, or failed remote CI.

## Verification

- `LOCK-HANDLE-001`: fixed by retaining an exclusive canonical handle in every successful Guard and deleting stale/owned objects through that handle.
- Win32 open: `CreateFileW`, `GENERIC_READ | DELETE`, share mode `0`, `OPEN_EXISTING`, `FILE_ATTRIBUTE_NORMAL | FILE_FLAG_OPEN_REPARSE_POINT`.
- Exact-object deletion: `SetFileInformationByHandle(FileDispositionInfo)` with `DeleteFile = TRUE`; the handle closes only after disposition is set.
- Candidate installation: `MoveFileExW(..., MOVEFILE_WRITE_THROUGH)` without replacement remains the canonical install primitive. Success is returned only after exclusive-open and exact through-handle verification.
- Bounded contention resolution: at most four install/claim cycles, with a 250 ms bounded handoff window only after this attempt installed the candidate.
- Simultaneous stale reclaim: 50 global plus 50 per-file subprocess iterations PASS; exactly one Guard per iteration and zero multiple-owner iterations.
- Handle contention, handle-backed release, live/expired live, dead, PID-reuse, image-mismatch, unverifiable and invalid-record behavior PASS.
- 64 Go test functions, vet, build and module verification PASS. Race instrumentation PASS in short mode (5 iterations per lock class); the ordinary filesystem stress gate separately ran 50 iterations per class.
- Phase 0 53/53 and `UPSTREAM_LOCK_VALID`: PASS.
- Static deletion assertion: PASS. `os.Remove` is limited to the current attempt candidate helper; stale reclaim and Guard release contain no canonical pathname deletion.
- `ErrLockOwnershipLost` is an abnormal fail-closed guard: exclusive read-only ownership normally prevents record replacement or mutation; any through-handle mismatch closes ownership without setting delete disposition.
- Remote Source CI: PENDING implementation push.

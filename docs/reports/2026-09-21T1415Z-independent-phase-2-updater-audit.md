# Independent Phase 2 updater audit

## Verdict

`PASS` — repair head `2f5dd7b134ad19b9390af9c174095ef94ed09abc` is suitable for
controlled fast-forward consideration from base
`4eb550e731f21b6a05347cd84df2fdb3b349a6b4`.

This report was produced on the separate branch
`codex/phase-2-independent-audit`. The reviewed branch was not modified. No
installed-slot registry, real updater, product/provider mutation, registry
write, credential access, listener start, or upstream branch update occurred.

## Scope and ancestry

- Repository: `trungqwe/dual-pool`
- Base: `4eb550e731f21b6a05347cd84df2fdb3b349a6b4`
- Repair head: `2f5dd7b134ad19b9390af9c174095ef94ed09abc`
- Branch reviewed: `phase-2/updater-audit-repair`
- `git merge-base` equals the requested base.
- `git rev-list --count base..head` is `11`.
- The 11 commits form one single-parent linear chain with no merge or
  divergence. GitHub compare reports `ahead_by=11`, `behind_by=0`.

The changed range contains only updater/Windows ACL source and tests, three
existing documentation files, two reports and three sanitized evidence JSON
files. It does not change `go.mod`, `go.sum`, workflows, vendor content,
binaries or supply-chain pins.

## Findings

### Lifecycle convergence and rollback

The updater acquires the global lock, recovers state, refuses an existing
pending marker, verifies both slots, runs disposable smoke, captures and
validates the running pool set, publishes the durable marker, stops the exact
captured set, saves active selection, starts the same set, runs production
smoke, and removes the marker. Failure injection and subprocess-crash tests
retain the marker on unresolved rollback. Recovery starts pools only when the
observed set is exactly the marker set or fully stopped; partial/unexpected
sets remain unresolved and are not guessed into mutation.

### Windows marker identity and security

Publication writes and syncs the candidate, validates ACL and readback on the
same handle, renames that handle with `NtSetInformationFile` using
`FileRenameInformation` and zero `ReplaceIfExists`, validates the post-rename
handle, syncs again, then closes it. Load opens with `OPEN_REPARSE_POINT`,
retries only sharing/lock violations for a bounded 250 ms, and treats only
file/path-not-found as absence. Removal compares the marker read through the
opened handle and applies `FILE_DISPOSITION_INFO` to that exact handle.

`FILE_DISPOSITION_INFO` is represented by a one-byte `BOOLEAN` field. Rename
layout offsets and no-replace behavior are tested on Windows, including
existing-target preservation and volume/file-index identity preservation.
ACL validation uses `GetFileInformationByHandle` and `GetSecurityInfo` on the
returned object; creation uses `CREATE_NEW` and share mode zero.

### Durability boundary

Pre-rename and post-rename `Sync` ordering is verified, and synthetic
process-crash recovery passes. The evidence correctly limits OS persistence to
the documented API boundary and explicitly records sudden power loss as
`NOT_PHYSICALLY_TESTED`. No absolute power-loss claim is accepted.

### Tests and evidence

Focused tests, repeated updater tests, race tests, full Go tests, build,
Node tests and upstream-lock validation were rerun or independently checked.
Assertions inspect external behavior: exact object identity, unchanged
existing target bytes, unrelated replacement preservation, errno
classification, marker retention and state convergence. No material false
positive or ignored-error path was found.

Two low-severity robustness observations remain non-blocking: the transient
contention test releases a handle after a 25 ms sleep, and permanent
contention asserts a 200 ms–1 s elapsed window. A very overloaded runner could
increase timing noise, but the tests also assert the errno, fail-closed load
result and object integrity, and repeated local/CI runs pass.

### Diagnostic gap decision

`markerStore.remove()` collapses a `loadHandle()` diagnostic wrapper to bare
`ErrRecoveryUnresolved` when opening, ACL validation, reading, decoding or
identity comparison fails. This is acceptable non-blocking diagnostic loss:
the public recovery contract is preserved with `errors.Is`, the operation
fails closed, the marker is not deleted, and no specification or test requires
an operation label for a pre-delete load failure. Actual disposition failure
still retains the `marker_delete` label and has a regression assertion. No
repair is required before controlled fast-forward consideration.

## CI receipts

The following exact-SHA Source CI receipts were independently verified as
completed success:

| Commit | Run |
|---|---:|
| `8dd5d5b76cf18a4e4f3584a2b979e44a9d7a81ab` | `35604796580` |
| `e6676fb2b9d83a7b7d0f0f15b4781fbe47c7cd94` | `35605070974` |
| `f3c1045a7cee2561c0cfda1ce8e7719745ff5ce1` | `35605208065` |
| `20ef4d6f78f3f0747c4983a89f791a617e539859` | `35605443117` |
| `2f5dd7b134ad19b9390af9c174095ef94ed09abc` | `35606700703` |

The historical “pending” wording in the repair-head report is superseded by
run `35606700703` and is not a failed gate.

## Next permitted objective

Only after this audit is accepted, a separate run may fast-forward
`phase-2/upstream-lifecycle` from the requested base to the repair head,
verify the ref and Source CI again, and then create a new worktree/branch for
the trusted installed-slot registry plus `instance.Manager` active-selection
integration. That registry work is outside this audit.


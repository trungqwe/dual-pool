# Phase 1 locks and PID identity

## Header

- Run ID: `phase-1-locks-pid-identity`
- Date/time UTC: 2026-09-20T00:15:00Z
- Agent/tool version: Codex / Go 1.26.0 windows/amd64
- Roadmap phase: Phase 1 — Foundation and state safety
- Branch/worktree: `phase-1/state-foundation`, dedicated clean worktree
- Start HEAD: `c985ef154c0af0f35c7a60eba2d214575431e82d`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Implement global and canonical-target per-file inter-process locks with strict metadata, Windows PID/start-time/image identity, safe bounded stale recovery, and Store per-document integration using TEMP-only fixtures.

## Non-goals

No product-root initialization or ACL policy, CLI lock commands, child supervisor/PID files, ports, config patching, secret storage, OAuth, provider/IDE integration, listeners, doctor, or update lifecycle. Tests never terminate a foreign/real process.

## Protocol decisions

- Canonical lock installation: synced exclusive sibling candidate followed by same-directory `MoveFileExW` without replacement.
- Owner identity: positive PID, raw 64-bit process creation FILETIME, and canonical full executable image.
- Stale policy: expiry is diagnostic only; an exact live identity always remains held. Dead, reused-PID, or image-mismatched records are removed only after byte-for-byte record recheck and one bounded retry. Unverifiable or invalid records fail closed.
- Root: existing injected non-reparse directory, canonicalized only after validating the supplied entry.
- Resource IDs: fixed `global`; per-file SHA-256 of canonical local Windows target identity. Raw paths never enter filenames/evidence.
- Release: idempotent for the owning guard; any changed canonical record returns ownership-lost and is preserved.
- Hierarchy: future coordinators acquire GLOBAL then PER-FILE; multiple files use ascending resource ID.
- Store: Save and Recover acquire the corresponding per-document lock before any read or orphan cleanup and hold it through cleanup/release. Load remains read-only.

## Planned subprocess tests

Real Windows process identity stability/exit; global and per-file contention; owner crash and stale acquisition; Store writer contention; crash recovery with stale per-document lock; recover-versus-writer candidate protection.

## Stop conditions

Stop on identity ambiguity, unsafe artifact/reparse behavior, any deletion before full safety inspection, lock-order bypass, real product-root access, real process termination, failed crash regression, secret/path leakage, remote divergence, or CI failure.

## Verification

- 61 Go test functions PASS; 9 are in `internal/lockfile` and 39 cover lock plus Store behavior.
- `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build`, and `go mod verify` PASS.
- Existing 22-scenario atomic Store fault matrix and two abrupt subprocess crash cases remain PASS with stale per-document locks.
- Global and Store subprocess contention, stale-owner recovery, CAS-window exclusion, recover-versus-writer protection, strict codec and reparse regressions PASS.
- Phase 0 regression: 53/53 Node tests and `UPSTREAM_LOCK_VALID` PASS.
- Real product root touched: false. Foreign/real processes killed: false. Only test-owned child processes were terminated by parent fixtures.
- Remote Source CI: PENDING until implementation push.

## Next run

Windows secret-store interface and implementation decision/proof using synthetic secrets only.

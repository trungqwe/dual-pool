# Báo cáo Phase 2 — pinned upstream downloader/stager

## Run identity

- Start HEAD: `64a4d86947af96a5ee71baeb4671fa6932614511`.
- Branch: `phase-2/upstream-lifecycle`.
- Toolchain: `go version go1.26.0 windows/amd64`.
- Objective: triển khai downloader/stager fail-closed, không credential, chỉ tiêu thụ đúng artifact Windows amd64 được khóa trong `upstream.lock`.
- Non-goals: product root, bốn product key, config generator, process lifecycle, listener, health, provider behavior, update promotion/rollback.

## Lock and network contract

- Schema: strict typed JSON v1 with bounded input, UTF-8 validation, duplicate-key rejection at every nesting level, unknown-field rejection and exactly one document.
- Runtime validation must retain parity with `scripts/phase0-upstream-lock.cjs` and add exact GitHub release/metadata path checks.
- Initial origin: exact HTTPS URL under `github.com/router-for-me/CLIProxyAPI/releases/download/<tag>/<artifact>` derived from the validated lock.
- Observed pinned redirect on 2026-09-20 UTC: one HTTPS `302` from `github.com` to `release-assets.githubusercontent.com`, followed by `200`; only these two exact hosts are allowed. Signed redirect URLs are not persisted.
- Request metadata: bounded `User-Agent` and `Accept` only. `Authorization`, `Cookie`, `Proxy-Authorization` and other credentials are forbidden and stripped on redirects.
- HTTP policy: explicit request deadline, response-header timeout, bounded idle connections, at most three redirects, HTTPS only, no custom CA and no TLS bypass.
- Download size policy: fixed 64 MiB archive ceiling, checked against `Content-Length` and during streaming. The observed pinned archive is 22,671,469 bytes, leaving ample bounded headroom without making size part of artifact identity.

## Archive and staging contract

- Download once to a unique attempt-owned file while streaming SHA-256 and byte count; `Sync` and `Close` precede comparison with the lock digest.
- ZIP is opened only after the archive digest matches. Entry count, per-entry expanded size and total inspected expanded size are bounded; unsafe Windows paths, traversal, reparse/symlink modes and device names fail closed.
- Extract only the unique regular entry whose streamed SHA-256 equals `executable_sha256`; filename guesses are not identity.
- Rehash the extracted file, then inspect it as PE AMD64 without execution.
- Source-proven identity command: the pinned `cmd/server/main.go` prints `CLIProxyAPI Version`, `Commit` and `BuiltAt` before Go's standard flag parser; `-h` invokes the standard help path and exits without entering server startup. The probe uses only `-h`, bounded time/output, no stdin and a minimal environment. Version must equal the lock and the reported short commit must prefix-match the lock's full commit.
- Stage root is an existing injected local directory with no reparse ancestor. Tests and pinned integration use disposable TEMP roots only.
- A unique sibling attempt directory is finalized under the existing global mutation lock. The strict manifest is written last inside the attempt. After rename, the manifest, binary digest and PE identity are reopened and verified.
- Existing complete matching stage is returned without download. Existing incomplete, mismatched or reparse-backed final state returns `ErrStageConflict` and is never overwritten.

## Real pinned integration plan

- CI sets `DUALPOOL_RUN_PINNED_UPSTREAM_INTEGRATION=1` on Windows.
- The test reads the repository's exact `upstream.lock`, stages into `t.TempDir()`, checks the observed redirect hosts, archive byte bound/hash, unique executable hash, PE AMD64, normalized version/commit identity, final manifest and cleanup.
- Only normalized booleans and safe artifact metadata may enter committed evidence. Archive, executable, TEMP path, signed URL, headers, environment and raw process output remain outside Git.

## Stop conditions

- Stop on any lock/schema disagreement, non-allowlisted origin/redirect, size overflow, hash mismatch, unsafe ZIP, executable ambiguity, non-AMD64 PE, binary identity mismatch, unsafe stage hierarchy, conflicting final stage, test regression or unavailable mandatory pinned integration.
- Do not weaken a gate, discover a newer release, use a mirror, execute before both hashes pass, or mark this slice PASS without observed Source CI integration success.

## Verification and delivery

- Local implementation result: PASS. The repository lock digest is `ae88ec7b0ca92aef29fdbbf0d1dbe31e3218d9861a556fca1e4b04a49b232b51`.
- Real pinned integration: PASS. Downloaded 22,671,469 bytes; archive/executable hashes, PE AMD64, version `7.3.7`, commit prefix `b773607e`, final manifest, idempotence and TEMP cleanup passed. No listener was started.
- Verification: 112 Go test functions, full race suite, vet, build, module verification, 54/54 Node tests, `UPSTREAM_LOCK_VALID`, JSON/docs/privacy/secret/diff gates PASS.
- Product root touched: false. Product secrets created: false. Provider credentials touched: false. `P2-ENTRY-ACL-001` and `P2-ENTRY-KEYS-001` remain OPEN and untriggered.
- Implementation commit/CI: PENDING.
- Delivery receipt: appended to `docs/18-HANDOFF.md` only after implementation CI PASS.

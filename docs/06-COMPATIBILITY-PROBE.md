# Phase 0 Compatibility Probe

Phase 0 is read-only except for writing sanitized evidence inside the repository's ignored evidence workspace. It must run before production code assumes any path, port, setting, endpoint or schema.

## Outputs

Create `evidence/<run-id>/` containing:

- `environment.json`: OS build, architecture, locale/time zone, non-sensitive executable paths.
- `versions.json`: Antigravity, Codex extension, Codex CLI/app-server, CLIProxyAPI candidate version.
- `repo.json`: branch, HEAD, remotes, dirty-state summary and active worktree path.
- `ports.json`: proposed ports and current owning process metadata.
- `codex-catalog.json`: matching installed/bundled model metadata, redacted only if necessary.
- `cliproxy-schema.json`: recognized config fields, endpoints and response shapes for candidate release.
- `antigravity-discovery.json`: settings candidates and route/schema fingerprints without payloads.
- `probe-results.json`: machine-readable test records.
- `SUMMARY.md`: decisions, blockers and next permitted phase.

Evidence committed to Git must pass redaction and secret scanning. Raw captures, if temporarily necessary, remain outside the repository and are deleted after sanitized derivatives are produced.

## Repository preflight

Record, without modifying:

```powershell
git rev-parse --show-toplevel
git branch --show-current
git rev-parse HEAD
git status --porcelain=v2
git remote -v
git submodule status
```

If the worktree is dirty, classify every path as user-owned, agent-owned or unknown. Do not overwrite unknown changes. If the remote is not exactly the intended repository, block push.

## Environment discovery

Collect:

- `Get-ComputerInfo` subset: Windows product/version/build and OS architecture.
- Current user-scoped `%LOCALAPPDATA%`, `%APPDATA%`, `%USERPROFILE%` paths without copying unrelated directory contents.
- Executable discovery via `Get-Command`, running process image paths and installed extension inventory.
- File versions and SHA-256 for candidate executables.
- Effective Codex home/config path and Antigravity user settings path.

Never recursively search the entire system drive. Search known product roots first; expand only with a documented reason.

## Port preflight

For 51074, 8317, 8318 and any discovered OAuth callback port:

1. Check TCP listeners.
2. Resolve owning PID.
3. Record image path, start time and command line if accessible.
4. Mark `free`, `owned_expected`, `occupied_foreign`, or `unknown`.
5. Never kill an occupying process during the probe.

## CLIProxyAPI candidate probe

Use a temporary, user-scoped test root and no real credentials initially.

1. Record release version, artifact URL, published checksum/signature metadata and local SHA-256.
2. Run `--help` and configuration validation if available.
3. Start one disposable loopback instance with a generated management key and client key.
4. Prove wildcard interfaces are not listening.
5. Verify unauthenticated management access fails.
6. Verify authenticated health/model/management calls and record schemas.
7. Confirm exact support for:
   - separate `auth-dir`;
   - `remote-management.allow-remote=false`;
   - non-empty `secret-key` behavior;
   - `routing.strategy`;
   - `routing.session-affinity` and TTL;
   - `routing.session-affinity-subagents`;
   - `ws-auth` and streaming fields if used;
   - OAuth/status/model/status-management endpoints.
8. Stop and remove only the disposable root.

Do not copy the current `main` example into production. Generate config from a versioned compatibility adapter for the pinned release.

## Codex probe

Record exact executable and extension versions. Then:

1. Locate the effective user-level `config.toml` using supported application behavior, not a guessed path.
2. Determine whether the extension and CLI share that config or have distinct launch environments.
3. Obtain the installed build's model catalog using a supported debug command if present; otherwise document a matching-artifact extraction method.
4. Validate that `gpt-6-astra` exists, is visible, supports required reasoning/tool behavior, and meets the installed client minimum version.
5. Start a disposable mock Responses endpoint on loopback.
6. Apply a temporary, reversible custom provider in a controlled profile/config copy.
7. Prove the client sends a Responses request and uses an environment-provided key.
8. In the extension UI, select Astra and prove the resulting request still reaches the custom provider.
9. Restore the original configuration and hash-compare unrelated content.

Required result for U-006: UI screenshot or structured UI observation plus loopback request metadata (`path`, method, model, content type, selected safe field names), with bodies removed.

## Antigravity probe

This is the highest-risk compatibility spike.

1. Record Antigravity version and settings file candidates.
2. Prove which setting controls the Cloud Code base URL using official settings metadata or reversible controlled experiment.
3. Run a loopback recorder that returns no fabricated application response and captures only route/method/header-name/body-field-name fingerprints.
4. Redirect the candidate setting temporarily after backup.
5. Trigger: startup, model-list refresh, one simple generation, one streaming generation, cancellation, tool action, long session resume.
6. Determine the original upstream destination from supported configuration/observed behavior without TLS interception.
7. Implement temporary transparent forwarding and compare direct vs bridged behavior.
8. Capture exact model ID location and identify a generation endpoint allowlist.
9. Verify the inner request can be forwarded to the proven Google CLIProxyAPI endpoint and the response can be rewrapped losslessly.
10. Restore the setting and verify hash/semantic equivalence.

No production donor routing may begin until transparent mode passes all native scenarios.

## Go/no-go table

| Gate | Pass condition | Failure action |
|---|---|---|
| G0-REPO | Correct repo/branch/remote; changes understood | Stop and request repository access/decision |
| G0-CPA | Candidate release supports required isolated configs/API | Test another pinned candidate or block |
| G0-CODEX-TRANSPORT | Responses custom provider reaches loopback | Block Codex integration |
| G0-CODEX-PICKER | Selecting Astra keeps desired provider route | Supported profile workaround or block UI goal |
| G0-AG-SETTING | Exact reversible base URL setting proven | Block Antigravity integration |
| G0-AG-PASS | Native behavior passes through transparently | Fix bridge only; no override |
| G0-AG-ENVELOPE | Donor unwrap/rewrap is lossless | Block donor override |
| G0-SECURITY | No forbidden listener/secret/evidence leakage | Fix before Phase 1 |

## Phase 0 exit

Update `03-DECISIONS-AND-EVIDENCE.md` unknowns with evidence paths. Set a pin candidate but do not call it supported until smoke tests pass. Update handoff and report; push only sanitized evidence and docs.

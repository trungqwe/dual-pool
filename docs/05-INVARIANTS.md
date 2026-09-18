# Trust Boundaries and Invariants

These invariants are release blockers. Tests must prove them; comments and intentions do not.

## Network

- INV-NET-01: Every listener binds to IPv4 loopback only.
- INV-NET-02: Remote Management is disabled on both CLIProxyAPI instances.
- INV-NET-03: Every Management API request is authenticated, including localhost.
- INV-NET-04: Google and Codex instances use different client keys and management keys.
- INV-NET-05: Poolbridge never disables TLS verification for external upstreams.
- INV-NET-06: No CA certificate, hosts-file entry, firewall opening, tunnel or LAN listener is created.

## Credentials and pools

- INV-POOL-01: Google and Codex auth files reside under disjoint directories.
- INV-POOL-02: Each child process is launched with exactly one provider pool's config/auth root.
- INV-POOL-03: Poolbridge never reads, parses, copies or commits token values.
- INV-POOL-04: OAuth `state` is treated as sensitive ephemeral data and never reused.
- INV-POOL-05: An account is not eligible merely because OAuth succeeded.
- INV-POOL-06: Disable/enable operations address an exact opaque auth identifier.
- INV-POOL-07: No automatic cross-provider fallback exists.

## Protocol and routing

- INV-ROUTE-01: Antigravity model discovery is never rewritten in V1.
- INV-ROUTE-02: Only exact donor ID equality can activate the override path.
- INV-ROUTE-03: Endpoint type must also be generation-capable; model equality alone is insufficient.
- INV-ROUTE-04: Unknown routes and malformed bodies fail safely; they are never guessed into the pool.
- INV-ROUTE-05: Non-donor behavior remains native passthrough.
- INV-ROUTE-06: Donor payload transformation is limited to proven envelope unwrap/rewrap.
- INV-ROUTE-07: Codex arrives at CLIProxyAPI as Responses; Chat Completions is forbidden.
- INV-ROUTE-08: Canonical `gpt-6-astra` identity and matching model descriptor are preserved.

## Configuration

- INV-CFG-01: No mutation occurs during compatibility probe.
- INV-CFG-02: A backup and content hash exist before the first mutation.
- INV-CFG-03: Only registered owned keys can change.
- INV-CFG-04: Writes use temp file, flush, atomic replace and post-parse verification.
- INV-CFG-05: Concurrent external edits trigger a compare-and-swap conflict, never silent overwrite.
- INV-CFG-06: Rollback restores original presence/value semantics, including deleting a key that was originally absent.
- INV-CFG-07: Repeated setup/rollback operations are idempotent.

## Processes and updates

- INV-PROC-01: PID files are not trusted without executable path, start time and instance identity verification.
- INV-PROC-02: A port occupied by an unknown process causes refusal, not termination.
- INV-PROC-03: Poolbridge never auto-upgrades CLIProxyAPI or Codex.
- INV-PROC-04: Downloads require an allowlisted release origin, pinned version and cryptographic checksum.
- INV-PROC-05: Promotion happens only after compatibility smoke tests; the previous binary remains rollback-capable.

## Privacy and evidence

- INV-LOG-01: Logs contain no prompt, output, tool arguments, source content or raw headers.
- INV-LOG-02: Tokens and API/management keys are redacted structurally, not by best-effort substring only.
- INV-LOG-03: Session identity is an HMAC or salted hash truncated for correlation; raw values are forbidden.
- INV-LOG-04: Email/account names are removed from committed evidence.
- INV-LOG-05: Test fixtures are synthetic and contain sentinel secrets that scanners can detect.

## Enforcement

Each invariant must map to one or more automated test IDs in `23-TRACEABILITY-MATRIX.md`. An invariant without a test is open work. A failing invariant test blocks commit promotion and push.

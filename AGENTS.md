# Agent Rules

These rules apply to every agent and every repository subdirectory.

1. Read `docs/00-README.md`, `docs/03-DECISIONS-AND-EVIDENCE.md`, `docs/05-INVARIANTS.md`, `docs/14-ROADMAP.md`, `docs/15-MASTER-CHECKLIST.md`, and `docs/16-AGENT-OPERATING-PROTOCOL.md` before implementation.
2. Work on exactly one roadmap phase per run unless the user explicitly authorizes more.
3. Do not fork, patch, or embed CLIProxyAPI. Use a pinned upstream release binary. A fork requires the seven-part exception test in ADR-001.
4. Do not implement OAuth, read refresh tokens, print secrets, copy auth JSON into project artifacts, or commit credentials.
5. Do not patch Antigravity or Codex binaries/bundles, install a local CA, edit `hosts`, or intercept TLS.
6. Do not invent model identifiers, settings paths, request schemas, callback behavior, or CLIProxyAPI configuration fields. Probe and attach sanitized evidence.
7. Keep the Google and Codex pools physically isolated: separate CLIProxyAPI processes, ports, auth directories, client keys, management keys, logs, and state.
8. Preserve Gemini-native payloads on the Antigravity path and Responses-native payloads on the Codex path. No Chat Completions intermediary is allowed.
9. Persistent edits must be minimal, atomic, backed up, attributed to Dual Pool, and reversible. Never overwrite an entire user config.
10. Stop at any failed phase gate. Do not hide, weaken, skip, or relabel a failing acceptance criterion.
11. Never use real prompts, source code, tokens, email addresses, or raw session identifiers as test evidence. Redact before committing.
12. Before every commit: run the phase verification commands, secret scan, documentation link check, and `git diff --check`.
13. End every run by updating `docs/18-HANDOFF.md` and creating a report from `docs/17-REPORT-TEMPLATE.md` under `docs/reports/`.
14. Commit only scoped files. Push only after all checks pass and the configured remote/branch is verified. Never force-push.
15. When a claim is not proven, label it `UNKNOWN`, record the probe needed, and leave its gate open.

# Dual Pool

Dual Pool is a Windows-first local integration controller for two strictly isolated AI credential pools:

- Google/Antigravity credentials serving one explicitly selected donor slot in Antigravity.
- Codex OAuth credentials serving the canonical `gpt-6-astra` model through the native Codex Responses harness.

The project does not implement OAuth, model protocols, or credential rotation itself. It supervises two isolated instances of the upstream CLIProxyAPI binary and owns only configuration, lifecycle, compatibility probes, reversible IDE integration, health checks, and evidence collection.

Start with [`docs/00-README.md`](docs/00-README.md). Coding agents must read [`AGENTS.md`](AGENTS.md) and [`docs/16-AGENT-OPERATING-PROTOCOL.md`](docs/16-AGENT-OPERATING-PROTOCOL.md) before changing the repository.

Current status: Phase 0 compatibility audit and Phase 1 foundation are closed. Phase 2 is in progress; its first bounded slice implements credential-free pinned upstream download and TEMP-only staging. Live provider/IDE integration is not enabled, and the project is not release-ready.

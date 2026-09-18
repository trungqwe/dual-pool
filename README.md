# Dual Pool

Dual Pool is a Windows-first local integration controller for two strictly isolated AI credential pools:

- Google/Antigravity credentials serving one explicitly selected donor slot in Antigravity.
- Codex OAuth credentials serving the canonical `gpt-6-astra` model through the native Codex Responses harness.

The project does not implement OAuth, model protocols, or credential rotation itself. It supervises two isolated instances of the upstream CLIProxyAPI binary and owns only configuration, lifecycle, compatibility probes, reversible IDE integration, health checks, and evidence collection.

Start with [`docs/00-README.md`](docs/00-README.md). Coding agents must read [`AGENTS.md`](AGENTS.md) and [`docs/16-AGENT-OPERATING-PROTOCOL.md`](docs/16-AGENT-OPERATING-PROTOCOL.md) before changing the repository.

Current status: specification package only. No runtime compatibility claim is valid until Phase 0 evidence has been captured on the target machine.

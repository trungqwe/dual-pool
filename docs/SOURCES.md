# Technical Sources

Retrieval baseline: 2026-09-18 UTC. These sources establish available interfaces, not target-machine compatibility.

- CLIProxyAPI repository and capability overview: https://github.com/router-for-me/CLIProxyAPI
- CLIProxyAPI version-current example configuration: https://github.com/router-for-me/CLIProxyAPI/blob/main/config.example.yaml
- CLIProxyAPI Management API: https://help.router-for.me/management/api
- CLIProxyAPI SDK boundary (reviewed but not selected for V1): https://github.com/router-for-me/CLIProxyAPI/blob/main/docs/sdk-usage.md
- CLIProxyAPI Management Center: https://github.com/router-for-me/Cli-Proxy-API-Management-Center
- Official Codex configuration reference: https://learn.chatgpt.com/docs/config-file/config-reference
- OpenAI Codex repository: https://github.com/openai/codex
- Upstream Codex model catalog source (informational only; never substitute for a mismatched installed build): https://github.com/openai/codex/blob/main/codex-rs/models-manager/models.json

When a release is pinned, add immutable tag/commit URLs and artifact hashes to `upstream.lock` and Phase 0 evidence. Do not treat moving `main` URLs as release evidence.

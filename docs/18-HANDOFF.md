# Current Handoff

Last updated: 2026-09-18 UTC.

## Current status

Phase hiện tại: **Phase 0B**. Gate `P0B-CX-TRANSPORT-001` v5: **PASS**; recorder và Codex tự thoát với code 0, không timeout/forced kill. Phase 1 chưa bắt đầu. Gate CLI không đóng U-005/U-006.

Nguồn trạng thái hiện hành: [status v5](../evidence/phase-0b-v5/phase-0b-status-v5.json). Bằng chứng: [result](../evidence/phase-0b-v5/codex-cli-transport-v5-20260918T212522665Z.json), [index](../evidence/phase-0b-v5/probe-results-v5.json). Run này không sửa cấu hình Codex/Antigravity và không thực hiện UI/OAuth.

Hash trước/sau của hai cấu hình bằng nhau; thứ tự trong result là Codex, Antigravity. Cleanup v5 còn 0 process/listener, temp root đã xóa và child environment không làm thay đổi environment cha. Bốn recorder v4 bị sót đã được dừng theo identity check; xem [correction](../evidence/phase-0b-v5/legacy-cleanup-correction.json).

Branch hiện hành: `phase-0/reversible-compatibility`; start HEAD `aa420da331c4dadab1443c2547f645e13f8eb1b6`. Delivery SHA và kết quả push được ghi bằng receipt sau commit. GitHub trả danh sách PR mở rỗng cho branch; run này không tạo PR.

## Delivery receipt — Gate A v5

- Commit implementation/evidence: `d36da495c5a8c68e4b644deaf5694c531b083546`.
- Push `git push origin phase-0/reversible-compatibility`: PASS, remote tiến từ `aa420da` tới `d36da49`.
- [Compare để review](https://github.com/trungqwe/dual-pool/compare/aa420da...d36da49); không phải PR đã tạo. `gh pr list --head phase-0/reversible-compatibility` trả danh sách rỗng.
- Worktree sạch sau push implementation; receipt này là commit docs tiếp theo, không sửa report/evidence đã push.
- Verifier sau commit: PASS, 32 JSON, 10 link nội bộ, 0 security match, hash và cleanup PASS.
- Gate A PASS; phase tổng thể còn BLOCKED bởi các U-gate. Nhiệm vụ kế tiếp: user-assisted Codex extension picker/config-layer trong run riêng.

## Repository and delivery — lịch sử Phase 0A

- Target remote: `https://github.com/trungqwe/dual-pool.git`.
- Start HEAD: `5e91df5f31531452a74444d171a7edb7db405573`.
- Branch: `phase-0/compatibility-inventory`.
- Phase 0A commit: `3f70ed84cbc52772ed1e448f3ee891c6f3343bbe`.
- Push result: PASS — new remote branch created normally with upstream tracking; no force push.
- Compare/PR URL: `https://github.com/trungqwe/dual-pool/compare/main...phase-0/compatibility-inventory`.
- Delivery receipt: this mutable handoff update records the pushed run commit; its own commit identity is intentionally left to Git history to avoid self-reference.
- Run report: `docs/reports/2026-09-18T1930Z-phase-0a-compatibility-inventory.md`.
- Evidence: `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/`.

## Phase 0A results

- Governance now separates non-destructive Phase 0A from reversible mutation-dependent Phase 0B.
- Root `.gitignore` and `docs/06-COMPATIBILITY-PROBE.md` define raw → sanitize → scan → commit; sanitized evidence remains trackable.
- Repository risk R-13 is resolved by authenticated inventory.
- Observed platform is Microsoft Windows 10 Pro `10.0.19045` x64, not the Windows 11 target in the charter. Results are valid for this observed machine but do not satisfy a Windows 11 acceptance claim.
- Proposed ports `51074`, `8317`, `8318` and candidate callback ports `1455`, `51121` were free at capture time.

## Codex discovery

- CLI: `0.154.0`; exact command path/hash are sanitized in `versions.json`.
- VS Code extension: `openai.chatgpt` `26.311.21342`.
- Default Codex home: `%USERPROFILE%\.codex`; config candidate exists, but contents were not read or changed.
- `codex debug models --bundled` is supported. Exact bundled/effective catalogs contain canonical `gpt-6-astra` with list visibility, API support, unified shell, code-only tool mode, and multi-agent v2 metadata.
- Extension config sharing, outbound Responses transport, and picker/provider retention remain UNKNOWN for Phase 0B.

## Antigravity discovery

- Product version `2.5.5`, core `1.107.0`, bundled extension `0.2.0`.
- Existing settings candidate: `%APPDATA%\Antigravity IDE\User\settings.json`; contents were not read or changed.
- Installed static bundle contains `jetski.cloudCodeUrl`, but the contributed settings schema does not declare it. The key and loopback behavior remain unproven until Phase 0B.

## CLIProxyAPI candidate

- Latest official release rechecked: `v7.3.7`, commit `b773607e3e7756dc6020a291825e4eb08899595a`.
- Windows amd64 archive SHA-256: `da5466b81beb7c769b99e26a5f6f41d9999a07be7c36be170167f10a2a6ecfc7`; matches both published `checksums.txt` and GitHub asset digest.
- Executable SHA-256: `bb44c6fa6a30214a294bdf64cf7385aa7ff65982aebd501ae37dc6a62a227072`.
- Tagged config/source contains required isolation, management, routing/affinity, streaming, WebSocket, OAuth/inventory and Responses/Gemini route surfaces recorded in `cliproxy-schema.json`.
- Credential-free disposable probe bound only `127.0.0.1`, enforced management/client keys, created no auth files, and stopped cleanly.
- Provider-specific response shapes and dedicated health semantics remain UNKNOWN without later authorized credentialed probes.

## Unknown status

- U-001: BLOCKED.
- U-002: BLOCKED.
- U-003: BLOCKED.
- U-004: BLOCKED pending separately authorized credential-specific probe.
- U-005: PARTIAL / UNKNOWN.
- U-006: BLOCKED pending user-assisted picker action.
- U-007: PROBED.
- U-008: PARTIAL / UNKNOWN.

Exact evidence and required next proof are in `docs/03-DECISIONS-AND-EVIDENCE.md`.

## Cleanup and security

- Disposable CLIProxyAPI process and listeners are absent.
- `%TEMP%` probe root containing archive, binary, source checkout, full catalogs, synthetic keys/configs and logs was removed after sanitization.
- No application integration was enabled, no account connected, and no foreign process terminated.
- JSON/link/privacy/secret/whitespace and final diff checks are recorded in the run report and `probe-results.json`.

## Latest Phase 0B run

- Run report: `docs/reports/2026-09-18T1953Z-phase-0b-reversible-compatibility.md`.
- Evidence: `evidence/2026-09-18T1953Z-phase-0b-reversible-compatibility/`.
- Codex CLI transport: `PASS` trong lịch sử, provisional và đã được thay thế bởi Gate A v5.
- Codex picker retention: `BLOCKED` — user-assisted picker action is required; automatic Codex CLI/extension UI automation is outside the active Windows automation safety boundary.
- Antigravity U-001/U-002/U-003: `BLOCKED` pending user-assisted settings/reload/discovery actions. U-004 is separately `BLOCKED` pending authorized credential-specific probing.
- Codex CLI transport v2: `PASS` trong lịch sử, provisional và đã được thay thế bởi Gate A v5.
- Cleanup: synthetic process and listener were removed; no application integration was enabled.
- Phase 1 remains forbidden. Repeat final acceptance on Windows 11 because this machine is Windows 10.

## Final assisted-gates continuation — lịch sử, đã được v5 thay thế

- Starting commit: `a22e70bb913b6a547cbdf23d792c4280a33f219d`.
- v3 Gate A: `FAIL`; Codex did not terminate naturally after the synthetic response, so picker and Antigravity UI checkpoints were not started.
- Current authoritative status: `evidence/2026-09-18T1953Z-phase-0b-reversible-compatibility/phase-0b-status-v3.json`.
- Security evidence: `evidence/2026-09-18T1953Z-phase-0b-reversible-compatibility/security-gate-v3.json` (`PASS`).
- Cleanup: owned probe process/listener stopped; temporary v3 home removed; no user configuration or account/OAuth file changed.
- Exact next objective: repair v3 bounded process lifecycle/response handling, rerun Gate A, and only then request the user-assisted Codex picker checkpoint.

## Gate A harness repair continuation — lịch sử, đã được v5 thay thế

- Start commit: `a6a04adbd468b9aeeab6e90ac358e9b4019377a4`.
- Current Gate A: `FAIL` — v4 reached the Responses request and passed the real SSE byte self-test, but the final harness result-persistence path failed; lifecycle result is explicitly `UNRECORDED`.
- Current authoritative gate status remains `evidence/2026-09-18T1953Z-phase-0b-reversible-compatibility/phase-0b-status-v3.json`; v4 diagnostics are append-only and do not close U-005.
- Historical/provisional transport PASS wording above is superseded by the current Gate A failure.
- U-001..U-008 remain: `BLOCKED`, `BLOCKED`, `BLOCKED`, `BLOCKED`, `PARTIAL / UNKNOWN`, `BLOCKED`, `PROBED`, `PARTIAL / UNKNOWN`.
- No picker or Antigravity probe ran. Exact next objective: repair the v4 finalization runtime exception, then rerun Gate A with a persisted natural-exit result.

## Exact next bounded objective

Phase 0B — user-assisted Codex extension picker/config-layer probe, trong một run độc lập với backup/hash/restore. Windows 10 hiện tại chỉ chứng minh compatibility thăm dò; acceptance Windows 11 vẫn mở. U-004 cần ủy quyền credential-specific riêng.

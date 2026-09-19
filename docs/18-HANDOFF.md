# Current Handoff

## Latest authoritative Phase 0B status — 2026-09-19

Delivery receipt: implementation/evidence commit `e31fd58` (`probe(phase-0): verify parallel Codex picker routing`) đã push thành công từ `c55cd44` lên remote branch `phase-0/reversible-compatibility`. Chưa tạo PR.

Gate `P0B-CX-TRANSPORT-001` remains PASS. The two-request parallel extension probe is **BLOCKED: EXTENSION_PAYLOAD_NOT_PARSEABLE**: the temporary extension reached loopback `POST /v1/responses` with `application/json` and an Authorization header, but the request body could not be safely parsed without retaining raw content. No model or prompt sentinel was inferred. U-005 remains `PARTIAL / UNKNOWN`; U-006 remains `BLOCKED`; U-001..U-004 remain `BLOCKED`; U-007 is `PROBED`; U-008 is `PARTIAL / UNKNOWN`.

The original Antigravity instance survived every probe. Temporary extension silo, user-data, `CODEX_HOME`, recorder and listener cleanup passed; real Codex and Antigravity settings hashes were unchanged. Evidence: [status](../evidence/phase-0b-codex-extension/phase-0b-status-extension.json), [result](../evidence/phase-0b-codex-extension/codex-extension-two-request-20260919T031615744Z.json), [security gate](../evidence/phase-0b-codex-extension/security-gate.json), [manifest](../evidence/phase-0b-codex-extension/evidence-manifest.json).

No Phase 1 or Cloud Code work started. Exact next task: analyze the extension's Responses body framing/encoding using a sanitized structural parser, then repeat the two-request experiment only if the parser can prove model and sentinel fields without persisting raw content.

## Cập nhật probe song song — 2026-09-19

Lượt hai-request `20260919T031615744Z` đã nhận `POST /v1/responses` từ extension tới loopback với `application/json`, body 58.422 bytes và Authorization header. Body không parse được bằng JSON parser/frame parser an toàn; không lưu body và không suy ra model/sentinel. Kết quả là `BLOCKED: EXTENSION_PAYLOAD_NOT_PARSEABLE`; [evidence](../evidence/phase-0b-codex-extension/codex-extension-two-request-20260919T031615744Z.json). Probe đã cleanup, instance gốc còn sống, hash cấu hình thật không đổi.

Cập nhật cleanup lúc 02:51:37 UTC: người dùng đã xóa cả hai profile tạm. Kiểm tra OS xác nhận hai thư mục vắng mặt, không còn process probe/recorder tương ứng, instance gốc vẫn sống và hai hash cấu hình thật không đổi. [Bằng chứng bổ sung](../evidence/phase-0b-codex-extension/cleanup-confirmation-20260919T025137Z.json) thay thế trạng thái cleanup chưa hoàn tất bên dưới; không sửa kết quả lịch sử. Delivery gate vẫn PENDING.

- Run `20260919T024618673Z`: **BLOCKED: ASTRA_NOT_VISIBLE**. Người dùng xác nhận model ban đầu GPT-5.6-Sol, nhưng không thấy Astra trong picker.
- [Kết quả probe](../evidence/phase-0b-codex-extension/codex-extension-picker-20260919T024618673Z.json) giữ nguyên trạng thái tại thời điểm recorder dừng. Sau đó người dùng đã đóng Probe; kiểm tra OS xác nhận PID 27420 không còn, PID gốc 30392 vẫn sống, hash Codex config và Antigravity settings không đổi.
- Không có request capture. U-005 PARTIAL / UNKNOWN; U-006 BLOCKED; U-001..U-004 BLOCKED; U-007 PROBED theo bằng chứng CLI v5; U-008 PARTIAL / UNKNOWN.
- Cleanup chưa hoàn tất: còn hai thư mục probe tạm của lượt timeout và lượt picker. Lệnh xóa bị automatic approval review từ chối (`blocked by policy`); không thử cơ chế khác để vượt chặn.
- Chưa commit/push thay đổi của probe; chưa hoàn tất security scan, manifest và delivery gate. Không chuyển Phase 1.
- Bước tiếp theo: hoàn tất cleanup hai profile tạm bằng thao tác người dùng, rồi kiểm tra catalog của runtime thực sự do extension sử dụng để điều tra Astra không hiển thị. Không suy ra catalog CLI là catalog của extension.

Last updated: 2026-09-18 UTC.

## Incident correction — 2026-09-19

The previous primary watchdog session is **ABORTED / DEAD**. Do not resume its session or write an `agent-approved` checkpoint. It reached `PROBE_CONFIG_ACTIVE`, `PRIMARY_RELAUNCHED_FOR_PROBE`, and `WAIT_AUTH_CONFIRMATION`; auth was lost, then the watchdog was stopped/rebooted before verified rollback. The real config consequently required the temporary `DUALPOOL_CODEX_KEY` on the next normal launch.

The owner/Gemini recovery report says the verified backup was restored and normal Codex became usable again. This is `OWNER/GEMINI_REPORTED`, not independently verified in the repository correction. The current execution host is now available, but the current real config hash is different from the historical `1AE4...5470D` baseline; this is `UNKNOWN: REAL_CONFIG_BASELINE_DRIFT`. No automatic restore is permitted.

Current policy: `REAL_USER_CONFIG_LIVE_MUTATION_DISABLED`. The ordinary `scripts/phase0b-u006-primary-watchdog.ps1` entrypoint now fails before any session, recorder, IDE, or config side effect with `PRIMARY_REAL_CONFIG_MUTATION_DISABLED`. The incident report is [here](reports/2026-09-19T0905Z-phase-0b-primary-watchdog-incident-audit.md), and the no-side-effect test is `scripts/phase0b-primary-watchdog-disabled.test.ps1`.

U-006 remains **BLOCKED**. The next permitted design is a new shadow `CODEX_HOME` probe under `%TEMP%`; the real user Codex directory must never be modified. A future shadow watchdog must set `real_config_mutation_capability=false` and `crash_requires_restore=false`, delete only the shadow, print `SHADOW_CLEANUP_PASS`, and wait for the owner to open Antigravity normally. It must not auto-launch the normal IDE. Do not start that design in this incident correction commit.

## Shadow-home design — pending live gate

The shadow-only entrypoint is now `scripts/phase0b-u006-shadow-home-watchdog.cjs`. It copies the complete `.codex` tree opaquely into a private TEMP root, modifies only the shadow `config.toml`, passes `CODEX_HOME` and the synthetic key only to the probe child, and never starts the normal IDE after cleanup. The real config is never a restore target. Fixture proof: 5/5 tests PASS. Report: `docs/reports/2026-09-19T0930Z-phase-0b-shadow-home-design.md`.

The current real config hash was observed as different from the historical baseline, so the shadow entrypoint will stop with `REAL_CONFIG_BASELINE_DRIFT` before copying or changing anything. This is `UNKNOWN`, not an authorization to overwrite the config. U-006 remains BLOCKED. A live shadow run requires a fresh read-only baseline decision and then owner interaction at auth/Astra checkpoints.

### Delivery receipt — incident correction and shadow design

- Incident correction: `a563d512be00e37a30b37a37dd387a7cb0cf003f`, pushed fast-forward from remote `37d568a`.
- Shadow design: `e79f3ed53e6cec9eee7cb6c34281cb0338c236ce`, pushed fast-forward from `a563d51`.
- Verification: disabled real-config entrypoint test PASS; shadow fixture tests 5/5 PASS; Node syntax, privacy scan, local links and `git diff --check` PASS.
- Live U-006: not run. Current real-config baseline drift blocks it before shadow copy. No config/auth/session/history mutation was performed by these commits.

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
## Latest parser-repair status — 2026-09-19

The v2 recorder self-tests PASS for valid/BOM JSON, invalid UTF-8, invalid JSON, gzip, identity decoding, hostile structure keys, and serializer redaction. The isolated baseline reached `POST /v1/responses` once with transport, auth, identity encoding, strict UTF-8, JSON parse, prompt sentinel, sanitizer, and response lifecycle all PASS. The gate is **BLOCKED: BASELINE_MODEL_MISMATCH** because the observed model is another `gpt-5.6-*` family member; its raw identifier was not persisted. This corrects the historical `EXTENSION_PAYLOAD_NOT_PARSEABLE` classification to **NOT PROVEN**. Evidence: [v2 result](../evidence/phase-0b-codex-extension-v2/extension-baseline-20260919T040521877Z.json), [audit correction](../evidence/phase-0b-codex-extension-v2/audit-correction.json).

U-005 remains **PARTIAL / UNKNOWN**; U-006 remains **BLOCKED**. The original Antigravity instance survived, real Codex/Antigravity hashes were unchanged, and the temporary Probe/recorder/session were cleaned up. No picker, Astra, Cloud Code, Phase 1, OAuth, credential, binary, TLS, or hosts-file work was performed. Exact next task: inspect the installed extension/runtime model catalog read-only and explain the model-selection mismatch; do not persist raw request values or add a catalog fallback.

## Delivery receipt

- Implementation/evidence commit: `9c3709ffa1c606ad1e4e1c0c35850b69927a2ace` (`probe(phase-0): repair extension parser stages`).
- Push: PASS to `origin/phase-0/reversible-compatibility`.
- PR: not created; branch remains the configured Phase 0 branch.
- Verification: `scripts/phase0b-verify-extension-v2.ps1` PASS; parser syntax, JSON validation, forbidden-metadata scan, cleanup, and `git diff --check` PASS.
- Residual worktree changes are pre-existing probe-generated modifications in the historical Phase 0B evidence directory and were not staged or overwritten.
## Latest authenticated-picker status — 2026-09-19

The historical unauthenticated `ASTRA_NOT_VISIBLE` observation is corrected prospectively to `NON_DIAGNOSTIC_FOR_ASTRA_ENTITLEMENT`. Existing evidence proves the extension consumed temporary `CODEX_HOME`, `dualpool_probe`, a random loopback Responses endpoint, and the synthetic child environment key; U-005 is therefore **PROBED** for Windows 10, `openai.chatgpt 26.5730.61309`, and the observed installed Antigravity/Codex runtime. The separate baseline model mismatch does not reopen U-005.

U-006 is **BLOCKED: AUTHENTICATED_PROBE_UNAVAILABLE**. The isolated custom-provider Probe had no Codex login UI, so the user could not authenticate through the disposable profile without reusing the primary session. No auth file, cookie, database, token, email, account ID, or billing data was inspected, copied, hashed, logged, or committed. The opaque temporary root was deleted after the user closed the Probe; the original IDE survived and real configuration hashes remained unchanged. Evidence: [result](../evidence/phase-0b-codex-extension-u006/u006-result.json), [security gate](../evidence/phase-0b-codex-extension-u006/security-gate.json), [manifest](../evidence/phase-0b-codex-extension-u006/evidence-manifest.json).

Exact next task: obtain an official login-capable disposable Codex/Antigravity profile or a supported user-assisted authentication path, then rerun only the U-006 picker retention probe. Do not start Phase 1 or Cloud Code.

## Delivery receipt — authenticated-picker boundary

- Implementation/evidence commit: `128cef7b0dbc8e8c6375d7eb11705b94a16562a0` (`probe(phase-0): audit authenticated picker boundary`).
- Push: PASS to `origin/phase-0/reversible-compatibility`.
- PR: not created.
- Verification: U-006 JSON, Node/PowerShell syntax, generic absolute-path scan, secret/privacy scan, cleanup controls, integrity manifest hash verification, and `git diff --check` PASS.
- Historical probe residue under `evidence/phase-0b-codex-extension/` remains unstaged and untouched.

## Latest U-006 auth-reuse boundary — 2026-09-19

- Owner-authorized opaque auth-copy probe: temporary `codex login status` classified `AUTHENTICATED_CHATGPT`.
- The isolated Codex extension exposed no login UI and did not consume the copied CLI auth state. Owner result: `AUTH_NOT_RECOGNIZED`.
- U-006: **BLOCKED: CLI_AUTH_NOT_CONSUMED_BY_EXTENSION**. Picker, GPT-6 Astra selection, and wire verification were not run.
- Evidence: `docs/reports/2026-09-19T0515Z-phase-0b-u006-auth-reuse.md`, `evidence/phase-0b-codex-u006-auth-reuse/`.
- Cleanup: PASS. Temporary auth/probe roots removed; original IDE survived; real configuration and extension hashes were unchanged; auth contents were never inspected, logged, hashed, committed, or retained.
- Exact next task: use an officially supported login-capable disposable Codex/Antigravity profile or user-assisted authentication path, then rerun only U-006. Do not start Phase 1 or Cloud Code.

## Latest opaque profile-clone outcome — 2026-09-19

- HOT clone was incomplete; COLD clone succeeded and the owner manually reopened the original IDE.
- Probe dropdown displayed `GPT-6 Astra`, but the owner confirmed no Codex Plus account was present in extension settings and the UI remained reconnecting. Authenticated clone recognition therefore failed: `CLONED_AUTH_STATE_NOT_RECOGNIZED`.
- U-006 remains **BLOCKED**. The observed `gpt-5.6-luna` request is not accepted as Astra proof because Astra was not actually selected for that request.
- Evidence: `docs/reports/2026-09-19T0553Z-phase-0b-u006-profile-clone.md`, `evidence/phase-0b-u006-profile-clone/`.
- Exact next task: use a supported login-capable disposable Codex/Antigravity profile or explicit user-assisted authentication path that exposes the real account state. Do not start Phase 1 or Cloud Code.

## Latest primary-profile probe outcome — 2026-09-19

- External watchdog was detached and the real user-level Codex config was backed up and temporarily mutated.
- Authoritative retry restored the config byte-for-byte: before/after SHA-256 `1AE4E3BC2C1185EA4C9C863BA481D66F5C160B02B63F4C04F1D885754257470D`.
- Antigravity relaunch failed within the bounded watchdog window, so no auth/Astra/wire checkpoint ran. U-006 remains **BLOCKED: NORMAL_RELAUNCH_FAILED**.
- Evidence: `docs/reports/2026-09-19T0715Z-phase-0b-u006-primary-profile.md`, `evidence/phase-0b-u006-primary-profile/`.
- Exact next task: resolve the supported Antigravity relaunch path or perform a user-assisted primary-profile relaunch, then rerun only U-006. Do not start Phase 1 or Cloud Code.

## Delivery receipt — opaque profile-clone boundary

- Implementation/evidence commit: `4518d4a` (`probe(phase-0): verify cloned profile authentication boundary`).
- Push: PASS to `origin/phase-0/reversible-compatibility`.
- PR: not created; branch remains the configured Phase 0 branch.
- Verification: COLD clone evidence, profile classification correction, PowerShell/Node syntax, JSON validation, privacy/absolute-path scan, cleanup controls, integrity manifest, and `git diff --check` PASS.
- Historical probe residue remains unstaged and untouched.

## Delivery receipt — U-006 auth-reuse boundary

- Implementation/evidence commit: `b7944df` (`probe(phase-0): record authenticated extension boundary`).
- Push: PASS to `origin/phase-0/reversible-compatibility`.
- PR: not created; branch remains the configured Phase 0 branch.
- Verification: U-006 result, auth bootstrap boundary, PowerShell/Node syntax, JSON validation, absolute-path/secret scan, cleanup controls, integrity manifest, and `git diff --check` PASS.
- Residual historical probe files remain unstaged and untouched.
### Delivery receipt — primary profile audit

- Implementation commit: `e877409c4416ff2d0cfa0b5c3079b7c8337588bf` (`probe(phase-0): audit primary authenticated profile boundary`).
- Remote push: PASS to `origin/phase-0/reversible-compatibility` as a fast-forward from `4f57f8a402be282cbefff802de6473c167e6735a`.
- Local tracking-ref update: `UNKNOWN` because the Windows filesystem rejected Git's local ref lock/unlink operation; the remote branch was verified by the successful push output.
- Next run remains U-006 only after a supported normal Antigravity relaunch path is available.

## Correction sau audit — 2026-09-19T0814Z

Verifier primary cũ ghi sẵn cleanup/restore PASS và bỏ qua một số exit code. Hai trường settings/extension unchanged đã sửa thủ công nhưng không có after hash nên là UNKNOWN. Config before/after nhất quán trong record, không phải kiểm tra live mới.

Verifier mới trả exit 1 và liệt kê bằng chứng thiếu. Watchdog chặn trước side effect với `PRIMARY_PROBE_DISABLED_PENDING_SAFETY_REPAIR`. Tests: 10/10 PASS; U-006 BLOCKED. Xem [audit](reports/2026-09-19T0814Z-phase-0b-primary-audit.md).

Bước tiếp: sửa compare-and-swap, kiểm tra backup trước restore, checkpoint ngoài IDE, recorder readiness và SSE; kiểm thử fixture trước khi bỏ chặn. Người dùng tự mở IDE sau restore. Probe launcher phải truyền synthetic key riêng cho process tree, không phụ thuộc agent chat.

Local HEAD vẫn `4f57f8a`, remote đã xác nhận `54f9744`; không stage toàn bộ các file hiện unstaged/untracked. Audit chưa commit/push vì safety gate fail.

## Tiếp tục đã được cho phép — primary repair

Owner cho phép tự sửa và kiểm thử đến PASS; chỉ gọi hỗ trợ khi cần thao tác tài khoản/IDE. Safety gate mới PASS: 25 Node tests, 12 assertion transaction. Xem [report sửa](reports/2026-09-19T0840Z-phase-0b-primary-repair.md). Guard cũ đã được thay bằng kiểm tra safety gate + hash manifest trước khi mở session. Gate này chỉ chứng minh fixture safety; U-006 chưa PASS.

Session dự kiến dưới TEMP: `dual-pool-u006-primary-729d99527ecc4b049ae56e53944418f0`. Main watchdog là `scripts/phase0b-primary-watchdog.cjs`; wrapper PowerShell gọi nó. Console nhận checkpoint độc lập; agent chỉ ghi `agent-approved` sau khi xác minh PID/heartbeat. Không gửi ASTRA_SENT qua chat: recorder tự phát hiện prompt sau khi chọn Astra. Sau restore PASS, người dùng tự mở IDE bằng shortcut; watchdog không tự mở normal IDE.

Khi quay lại: đọc result mới tại `evidence/phase-0b-u006-primary-repair/result-*.json`; nếu chưa có, đọc status/heartbeat của đúng session TEMP. Không mở lại probe thứ hai. Nếu restore chưa PASS, giữ backup và dùng recovery helper khi IDE đã đóng. Không đọc auth.

### Delivery sửa primary watchdog

- Implementation: `10305afd80fdfa7a3d62f61c65a2657df9cc5088`; push PASS tới `origin/phase-0/reversible-compatibility` bằng checkout delivery riêng, commit/push Git thông thường.
- Safety fixture gate PASS; hash artifact đã kiểm tra cả working tree và staged Git blob trước commit. U-006 live vẫn chưa được xác nhận.
- Workspace chính giữ nguyên residue và local ref cũ; không pull/reset trên workspace chính khi chưa đối chiếu các thay đổi đã push. Checkout delivery nằm dưới TEMP với basename `dual-pool-delivery-729d99527ecc4b049ae56e53944418f0`.

### Startup correction và checkpoint đang chờ

Lần startup đầu dừng ở ACL thư mục TEMP, trước backup/mutation. Config thật vẫn có hash `1AE4E3BC2C1185EA4C9C863BA481D66F5C160B02B63F4C04F1D885754257470D`. Đã sửa cấp quyền theo Windows SID và bổ sung test đọc/ghi thật sau ACL: gate hiện 26 Node tests + 12 assertion transaction PASS. Thư mục startup thất bại rỗng đã được dọn; không có backup/auth trong đó.

Watchdog mới đã được quan sát `WATCHDOG_READY`, Node PID 24088, parent console PID 84276 đúng process sở hữu, heartbeat cập nhật; transaction chưa tồn tại. Sau agent handshake, trạng thái tiếp theo là WAIT_PRIMARY_CLOSE. User cần giữ console này mở, đóng IDE tự nguyện, làm auth/Astra checkpoint tại console; không gửi checkpoint vào chat trong thời gian provider probe hoạt động. Sau restore PASS, user tự mở IDE bình thường. Dùng đúng session đã ghi ở trên, không mở watchdog trùng.
### Shadow harness repair — 2026-09-19

The initial shadow implementation was audited and SHADOW-001..008 were repaired. The watchdog now creates the required three-directory topology before checkpoints, enforces a cold source, uses the real Antigravity launch path with a sanitized child environment, waits for `armed.json`, displays the generated prompt, waits for `capture.json`, distinguishes auth loss from Astra absence, and uses a dynamic run-scoped real-config hash. The old primary watchdog remains hard-disabled.

Read-only assessment: current config SHA-256 `C15720CC37B0670D4EC1D0294DA5E067DD7CF65437D2E5DF4C209A9411F5CD58`; TOML `PASS`; all probe markers `false`; historical equivalence not claimed. Owner-reported current Codex chat supports `CURRENT_WORKING_CONFIG_CANDIDATE`, but this run deliberately did not launch a live probe. Real config mutation: `false`. Tests: 15/15 PASS. Report: `reports/2026-09-19T1128Z-phase-0b-shadow-harness-repair.md`.
### Delivery receipt — shadow U-006 harness repair

- Implementation/evidence commit: `cafd9a3cf72be06dec1d7d457858048d4bf83ee9` (`fix(phase-0): repair shadow U-006 orchestration`).
- Push: PASS to `origin/phase-0/reversible-compatibility`, fast-forward from `383c38b1b65da08ac4b3e87bfee841c1f411cd8b`.
- Verification: 15/15 shadow tests PASS; Node syntax PASS; primary real-config mutation guard PASS; JSON/privacy/staged-secret/diff checks PASS.
- Current real config was read-only assessed; SHA-256 remained `C15720CC37B0670D4EC1D0294DA5E067DD7CF65437D2E5DF4C209A9411F5CD58`; no probe marker was found; real config mutation was `false`.
- Live U-006 was intentionally not run. No Antigravity process was launched, no auth was inspected, and no normal IDE was auto-launched.
- Next action: only a separately authorized owner-assisted live shadow run. On auth loss, record `FULL_CODEX_HOME_SHADOW_AUTH_NOT_RECOGNIZED`; never return to real-config mutation.
### Shadow lifecycle safety repair — 2026-09-19

At starting HEAD `7db2454`, SHADOW-009..013 were repaired. The watchdog now waits for voluntary primary closure, fails closed on process-query errors, applies a restricted TEMP ACL before copying, retains the shadow on probe-close timeout, and requires manual normal reopen plus `NORMAL_IDE_OK` and a final unchanged real-config hash before PASS. The external launcher and bounded cleanup helper are included. Safety tests: 20/20 PASS; live probe not yet run. Report: `reports/2026-09-19T1254Z-phase-0b-shadow-lifecycle-repair.md`.

### Shadow watchdog stall correction — 2026-09-19

The first live shadow run exposed an observability defect: after terminal checkpoint input, the watchdog entered its bounded probe-close wait but stopped heartbeat updates and left `status.json` at `WAIT_AUTH_CONFIRMATION`. Unknown or empty checkpoint input was also treated as a terminal failure. The repair tracks the current lifecycle stage, keeps heartbeat active through `CLOSE_PROBE`, and accepts only exact checkpoint tokens. Tests: 22/22 watchdog and 48/48 Node PASS. The old live session remains fail-closed with its shadow retained while the probe is alive; no second probe was launched. Report: `reports/2026-09-19T1320Z-phase-0b-shadow-watchdog-stall-repair.md`.

### Delivery receipt — shadow watchdog stall correction

- Implementation/evidence commit: `04e989d2569358a53b17be092b81d610842c46d1` (`fix(phase-0): repair shadow watchdog stall state`).
- Push: PASS to `origin/phase-0/reversible-compatibility`; remote SHA verified directly with `git ls-remote`.
- Verification: 22/22 watchdog tests and 48/48 Node tests PASS; Node syntax, PowerShell parser, JSON parse, docs links, staged privacy/absolute-path scan, manifest, and `git diff --check` PASS.
- Primary watchdog remains disabled; real-config mutation capability remains `false`; no second live probe was launched.

### Live shadow run cleanup outcome — 2026-09-19

- The owner reported all probe windows closed; all 11 remaining background Antigravity processes were verified as descendants of the single watchdog-owned probe root before scoped termination.
- The old watchdog did not advance after the probe tree reached zero. Its owned recorder and watchdog processes were stopped, then the bounded cleanup helper removed only the owned shadow.
- Cleanup PASS: probe, watchdog, recorder, listener, and shadow absent. Real config remained marker-free and byte-identical through cleanup with SHA-256 `F8D0FC86632716BC202660E0A544DB1BB26622175C2865B53D4B01B7A227B43F`.
- U-006 remains **BLOCKED: WATCHDOG_STALL_REQUIRES_FORCED_OWNED_PROCESS_CLEANUP**. Auth recognition, Astra visibility, and target wire acceptance remain `UNKNOWN`; no second live probe was launched.
- Exact next task: record safe-isolation exhaustion for U-006; do not return to real-config mutation. Report: `reports/2026-09-19T1335Z-phase-0b-shadow-live-cleanup.md`.

### Delivery receipt — blocked shadow cleanup

- Live evidence commit: `463fbeeabbda5759ddd9135d6291ab862cf4cf7e` (`probe(phase-0): record blocked shadow cleanup`).
- Push: PASS to `origin/phase-0/reversible-compatibility`; remote SHA verified directly with `git ls-remote`.
- Security gate and manifest: PASS. No raw secret, user path, PID, port, SID, auth state, session content, or prompt value was committed.
- PR: not created; direct delivery to the configured Phase 0 branch was requested.

## Phase 0 closure — 2026-09-19

- Phase 0 status: **PASS_WITH_BLOCKERS**. The roadmap permits exit when unresolved compatibility gates are explicitly documented.
- GO: Phase 1 fixture-only foundation and later credential-free lifecycle scaffolding after Phase 1.
- NO-GO: live Antigravity integration, Codex picker integration, provider promotion, and release acceptance.
- Candidate upstream: CLIProxyAPI `v7.3.7`, commit `b773607e3e7756dc6020a291825e4eb08899595a`; deterministic metadata and hashes are in `upstream.lock`. `config_adapter_version` remains `UNIMPLEMENTED`.
- Preserved blockers: U-001..004 and U-006 `BLOCKED`; U-008 `PARTIAL_UNKNOWN`; U-005 and U-007 `PROBED`.
- Exact next task: Phase 1 — create the Go CLI skeleton, version output, stable error envelope, and initial unit-test/CI foundation. No live IDE/provider integration.

# Requirements Traceability Matrix

This initial matrix defines minimum test identities. Implementation must add exact test file/evidence links.

| Requirement/invariant | Tests | Evidence level | Status |
|---|---|---|---|
| FR-001/002, INV-POOL-01/02 | ISO-001..004, CPA-INT-001 | E3 | Open |
| FR-003, INV-NET-01/02 | NET-001, NET-INT-001 | E3 | Open |
| FR-004..009 | AUTH-001..004, AUTH-LIVE-001 | E3/E4 | Open |
| FR-010, INV-ROUTE-05 | AG-DIFF-001 | E4 | Open |
| FR-011/012, INV-ROUTE-01..04 | ROUTE-001..003, AG-E2E-001 | E3/E4 | Open |
| FR-013/014, INV-ROUTE-06 | AG-CONTRACT-001, AG-DIFF-002 | E3/E4 | Open |
| FR-015..017, INV-ROUTE-07/08 | CX-001..003, CX-E2E-001 | E4 | Open |
| FR-018/019 | CX-CATALOG-001, CX-PICKER-001 | E3/E4 | Open |
| FR-020 | AFF-001/002, AFF-LIVE-001 | E4 | Open |
| FR-021..023, INV-CFG-01..07 | CFG-001..004, CFG-PROP-001 | E3 | Open |
| FR-024 | DOC-001, DOC-BUNDLE-001 | E3 | Open |
| FR-025, INV-PROC-03..05 | UPD-001..003, SUP-001 | E3 | Open |
| NFR-001, INV-LOG-01..05 | LOG-001, DEL-001, SECRET-001 | E3 | Open |
| NFR-002 | ISO-001..004, ISO-LIVE-001 | E4 | Open |
| NFR-003 | CFG-ROLL-001, AG-ROLL-001 | E4 | Open |
| NFR-004 | AG-LONG-001, CX-LONG-001 | E4 | Open |
| NFR-005 | PROC-001..003 | E3 | Open |
| NFR-006 | PERF-AG-001, PERF-CX-001 | E4 | Open |
| NFR-007/008 | BUILD-001, ARCH-001 | E3/review | Open |
| NFR-009 | EVIDENCE-SCHEMA-001 | E3 | Open |
| NFR-010 | PRIV-001 | E3/review | Open |
| Phase 0A repository/environment/port discovery | P0A-REPO-001, P0A-ENV-001, P0A-PORT-001 | E2 | Probed — `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/probe-results.json` |
| ADR-006 installed Astra identity and safe export | P0A-CODEX-001 | E2 | Probed — `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/codex-catalog.json` |
| INV-NET-01/02/03 candidate behavior | P0A-CPA-001 | E2 | Candidate probed; production E3 gate remains open |
| INV-PROC-04 candidate supply chain | P0A-SUP-001 | E2 | Candidate probed; production pin/promotion gate remains open |
| INV-LOG-06 evidence boundary | P0A-SEC-001, P0A-CLEANUP-001 | E2/review | Probed — `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/probe-results.json` |

## Update rule

No requirement may be marked DONE until every mapped mandatory test passes for the pinned release/target environment. Add new rows when behavior is discovered; never remove a mapping solely because implementation is difficult.
## Phase 0B evidence-repair addendum

| Requirement | Evidence | Status |
|---|---|---|
| P0B-CX-TRANSPORT-001: CLI Responses, lifecycle, sentinel, cleanup | [v5 script](../scripts/phase0b-codex-cli-transport-v5.ps1), [v5 result](../evidence/phase-0b-v5/codex-cli-transport-v5-20260918T212522665Z.json) | PASS — Windows 10 exploratory; U-005 remains PARTIAL / UNKNOWN |
| Extension picker retention | [two-request probe](../evidence/phase-0b-codex-extension/codex-extension-two-request-20260919T031615744Z.json) | BLOCKED — baseline loopback request arrived, but payload parsing failed before model/sentinel proof; Astra request not run |
| Antigravity setting/redirect | User-assisted reload and loopback discovery | BLOCKED |
| Antigravity route/envelope | Depends on U-001 and U-002 | BLOCKED |
| Credential-specific Gemini eligibility | Separately authorized provider probe | BLOCKED |
## Phase 0B parser-repair addendum

| P0B-CX-EXTENSION-PARSER-V2 | [v2 recorder](../scripts/phase0b-extension-recorder-v2.cjs), [v2 runner](../scripts/phase0b-codex-extension-baseline-v2.ps1), [v2 result](../evidence/phase-0b-codex-extension-v2/extension-baseline-20260919T040521877Z.json) | Parser/sanitizer and ingress controls PASS; overall gate BLOCKED by `BASELINE_MODEL_MISMATCH`; U-005 PARTIAL / UNKNOWN; U-006 BLOCKED |
## Phase 0B authenticated-picker addendum

| P0B-CX-CONFIG-LAYER-001 | [config-resolution](../evidence/phase-0b-codex-extension-u006/config-resolution.json), [U-005 correction](../evidence/phase-0b-codex-extension-u006/u005-audit-correction.json) | U-005 PROBED for observed Windows 10/runtime scope |
| P0B-CX-MODEL-RESOLUTION-001 | [U-006 result](../evidence/phase-0b-codex-extension-u006/u006-result.json) | BLOCKED: AUTHENTICATED_PROBE_UNAVAILABLE; no picker or wire request run |
| P0B-CX-AUTH-REUSE-001 | [auth bootstrap](../evidence/phase-0b-codex-u006-auth-reuse/auth-bootstrap-20260919T053402170Z.json), [U-006 result](../evidence/phase-0b-codex-u006-auth-reuse/u006-20260919T053402492Z.json) | Opaque CLI auth copy authenticated, but extension did not consume it; BLOCKED: CLI_AUTH_NOT_CONSUMED_BY_EXTENSION |
| P0B-CX-PROFILE-CLONE-001 | [profile-clone report](../docs/reports/2026-09-19T0553Z-phase-0b-u006-profile-clone.md), [profile-clone evidence](../evidence/phase-0b-u006-profile-clone/) | COLD copy reached Probe; dropdown Astra was visible but authenticated account was not recognized; BLOCKED: CLONED_AUTH_STATE_NOT_RECOGNIZED |
| P0B-CX-PRIMARY-PROFILE-001 | [primary-profile report](../docs/reports/2026-09-19T0715Z-phase-0b-u006-primary-profile.md), [primary-profile evidence](../evidence/phase-0b-u006-primary-profile/) | Config restored byte-for-byte, but watchdog could not relaunch Antigravity; BLOCKED: NORMAL_RELAUNCH_FAILED |
| P0-CLOSURE-001 | [closure result](../evidence/phase-0-closure.json), [candidate lock](../upstream.lock), [validator](../scripts/phase0-upstream-lock.cjs) | PASS_WITH_BLOCKERS: Phase 1 fixture foundation GO; live IDE/provider integration and release NO-GO |
| P0-LOCK-ALLOWLIST-001 | [validator](../scripts/phase0-upstream-lock.cjs), [negative tests](../scripts/phase0-upstream-lock.test.cjs), [repair report](reports/2026-09-19T1523Z-phase-0-lock-allowlist.md) | PASS: unknown, duplicate and non-string verified capability claims fail closed; candidate lock remains valid |

## Phase 1 foundation bootstrap

| Requirement | Test and evidence | Status |
|---|---|---|
| P1-CLI-001: supported commands and usage failure | [app tests](../internal/app/app_test.go), [result](../evidence/phase-1-foundation-bootstrap/test-result.json) | PASS locally; remote CI pending |
| P1-VERSION-001: deterministic and safe build output | [buildinfo tests](../internal/buildinfo/buildinfo_test.go), [result](../evidence/phase-1-foundation-bootstrap/test-result.json) | PASS locally; remote CI pending |
| P1-ERROR-001: exact categories and stable code registry | [error tests](../internal/apperr/apperr_test.go), [result](../evidence/phase-1-foundation-bootstrap/test-result.json) | PASS locally; remote CI pending |
| P1-ERROR-REDACTION-001: wrapped cause excluded from human and JSON output | [error tests](../internal/apperr/apperr_test.go), [security gate](../evidence/phase-1-foundation-bootstrap/security-gate.json) | PASS locally; remote CI pending |
| P1-CI-001: source-only Windows workflow | [workflow](../.github/workflows/ci.yml), [result](../evidence/phase-1-foundation-bootstrap/test-result.json) | Workflow defined and parsed locally; remote CI pending |

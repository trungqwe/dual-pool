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

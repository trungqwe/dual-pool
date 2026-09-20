# Documentation Control Center

## Purpose

This directory is the executable specification for Dual Pool. It is designed so a new coding agent can determine scope, architecture, risks, current state, required evidence, and the next permitted action without relying on chat history.

## Reading order

| Order | File | Contract |
|---:|---|---|
| 1 | `01-PROJECT-CHARTER.md` | Outcome, users, scope, definition of done |
| 2 | `02-REQUIREMENTS.md` | Functional and non-functional requirements |
| 3 | `03-DECISIONS-AND-EVIDENCE.md` | Accepted decisions, verified facts, unknowns |
| 4 | `04-SYSTEM-ARCHITECTURE.md` | Components, flows, process and port topology |
| 5 | `05-INVARIANTS.md` | Rules that implementation may never violate |
| 6 | `06-COMPATIBILITY-PROBE.md` | Phase 0 discovery and go/no-go experiments |
| 7 | `07-CLIPROXYAPI-INTEGRATION.md` | Upstream ownership, configuration and lifecycle |
| 8 | `08-ANTIGRAVITY-INTEGRATION.md` | Native passthrough and donor override design |
| 9 | `09-CODEX-INTEGRATION.md` | Responses provider, catalog and dropdown behavior |
| 10 | `10-STATE-CONFIG-ROLLBACK.md` | State machine, transactional edits, recovery |
| 11 | `11-SECURITY-THREAT-MODEL.md` | Assets, threats, controls, secret-handling |
| 12 | `12-OBSERVABILITY-ERRORS.md` | Logs, metrics, health and stable errors |
| 13 | `13-TEST-STRATEGY.md` | Test layers, fixtures, failure injection |
| 14 | `14-ROADMAP.md` | Phase sequence, entry/exit gates, deliverables |
| 15 | `15-MASTER-CHECKLIST.md` | Single progress ledger |
| 16 | `16-AGENT-OPERATING-PROTOCOL.md` | Mandatory per-run workflow |
| 17 | `17-REPORT-TEMPLATE.md` | Report schema for each implementation run |
| 18 | `18-HANDOFF.md` | Mutable current-state handoff |
| 19 | `19-MASTER-IMPLEMENTATION-PROMPT.md` | Copy/paste prompt for the next coding agent |
| 20 | `20-VERIFICATION-CONTRACT.md` | Evidence rules and command/result contract |
| 21 | `21-NEGATIVE-TEST-MATRIX.md` | Required fail-closed tests |
| 22 | `22-RISK-REGISTER.md` | Ranked delivery and operational risks |
| 23 | `23-TRACEABILITY-MATRIX.md` | Requirement-to-test-to-evidence mapping |
| 24 | `24-GITHUB-DELIVERY.md` | Branch, commit, push and review workflow |

## Authority and change control

The order of authority is:

1. The user's latest explicit instruction.
2. `AGENTS.md` safety and process rules.
3. Accepted ADRs in `03-DECISIONS-AND-EVIDENCE.md`.
4. Requirements and acceptance criteria.
5. Architecture and component detail.
6. Roadmap estimates and examples.

If two documents conflict, stop and reconcile them in the same commit. Do not silently choose one.

## Status vocabulary

- `UNKNOWN`: no reliable evidence exists.
- `PROBED`: observed once on a named version and environment.
- `VERIFIED`: repeatable test exists and passes.
- `BLOCKED`: required external capability or evidence is unavailable.
- `ACCEPTED`: owner-approved decision or risk.
- `DONE`: acceptance criteria are satisfied with linked evidence.

No feature is `DONE` because code exists. It is `DONE` only when the verification contract is met.

## Scope boundary

Version 1 is Windows 11 x64, local-only, single-user, and limited to two pools. It is not a generic gateway, account reseller, cloud service, GUI platform, telemetry product, or cross-platform package.

## Canonical repository

Target remote: `https://github.com/trungqwe/dual-pool`

An agent must verify the actual remote with `git remote -v` before any push. Repository access and remote identity were established during Phase 0. Current committed handoff, reports and evidence on the named authoritative branches are the source of truth; the unauthenticated package-bootstrap state is historical only.

# Negative Test Matrix

| ID | Fault/attack | Expected behavior | Required evidence |
|---|---|---|---|
| NET-001 | Config requests `0.0.0.0`/`::` | Validation refuses startup | Listener/config test |
| SEC-001 | Missing/wrong management key | 401/403; no data returned | Contract test |
| SEC-002 | Client key used as management key | Rejected | Contract test |
| SEC-003 | OAuth URL has unexpected scheme/host | Browser not opened; stable error | Unit test |
| SEC-004 | Sentinel token appears in upstream error | Redacted before log/report | Redaction test |
| ISO-001 | Codex auth file placed in Google root | Inventory validation quarantines/disables; no request | Integration test |
| ISO-002 | Google auth file placed in Codex root | Same fail-closed behavior | Integration test |
| ISO-003 | Codex request sent to Google port | Unsupported/unauthorized; no Codex credential | Integration trace |
| ISO-004 | Gemini donor request sent to Codex port | Unsupported/unauthorized; no Google credential | Integration trace |
| AUTH-001 | OAuth never completes | Deadline, cancel if supported, no eligible account | Fake API test |
| AUTH-002 | Two OAuth flows started concurrently | Second waits/refuses; states never mixed | Concurrency test |
| P1-LOCK-HANDLE-001 | Two processes reclaim the same stale global/per-file lock simultaneously | Exactly one Guard; loser is held; successor cannot be deleted by pathname | Repeated Windows subprocess test |
| AUTH-003 | Multiple new auth files appear | Ambiguous association; user decision required | Contract test |
| AUTH-004 | OAuth succeeds but model missing | Account marked ineligible | Live/mock test |
| ROUTE-001 | Model display name resembles donor | Passthrough; no reroute | Table test |
| ROUTE-002 | Donor ID on non-generation endpoint | Passthrough/reject per allowlist, never pool | Table test |
| ROUTE-003 | Unknown schema with donor string | `AG_SCHEMA_UNSUPPORTED` | Fuzz/contract test |
| ROUTE-004 | Request attempts arbitrary upstream host | Rejected; no outbound SSRF | Security integration test |
| STREAM-001 | Upstream fails before first byte | Bounded safe retry only | Mock stream test |
| STREAM-002 | Upstream fails after output/tool event | No automatic replay | Mock stream test |
| STREAM-003 | Client cancels | Cancellation reaches upstream; resources close | Integration test |
| CFG-001 | Target changes after backup | `CONFIG_CONFLICT`; no overwrite | Integration test |
| CFG-002 | Disk/permission failure before replace | Original intact | Fault-injection test |
| CFG-003 | Crash between replace and journal | Startup recovery identifies/repairs state | Crash test |
| CFG-004 | Rollback sees user-changed owned value | `ROLLBACK_CONFLICT`; preserve user value | Integration test |
| PROC-001 | PID file points to reused foreign PID | Refuse kill/status as owned | Unit/integration test |
| PROC-002 | Required port occupied | Refuse startup and name safe owner info | Integration test |
| PROC-003 | Child executable hash differs | Refuse startup | Integration test |
| UPD-001 | Artifact checksum mismatch | Quarantine/delete staged candidate; no execute | Unit/integration test |
| UPD-002 | Candidate starts but health schema differs | No promotion | Disposable smoke |
| UPD-003 | Promoted version fails production smoke | Automatic binary pointer rollback | Integration test |
| CX-001 | Catalog from different Codex version | Reject `CATALOG_VERSION_MISMATCH` | Unit/contract test |
| CX-002 | Picker sends Astra to OpenAI/default route | Fail acceptance with `CODEX_PICKER_ROUTE_MISMATCH` | Target UI evidence |
| CX-003 | Request arrives as Chat Completions | Fail protocol invariant | Ingress shape test |
| AFF-001 | Bound credential disabled | Controlled switch, then sticky new binding | Live/integration test |
| AFF-002 | Subagent lacks parent marker | Document fallback; no false affinity claim | Shape + routing test |
| LOG-001 | Prompt contains token-like sentinel | Prompt never logged, even debug/error | End-to-end scan |
| DEL-001 | ZIP/report contains auth JSON/key | Packaging fails and artifact quarantined | Artifact scan |

All rows are mandatory when their component enters scope. `BLOCKED` or `SKIPPED` rows remain open in the checklist.

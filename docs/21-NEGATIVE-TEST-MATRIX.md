# Negative Test Matrix

## Phase 2 pinned upstream downloader/stager

| ID | Failure | Expected result |
|---|---|---|
| P2-LOCK-CONSUME-001 | Unknown/duplicate key, invalid UTF-8/type, extra JSON, missing field or oversized `upstream.lock` | Reject before network access |
| P2-DOWNLOAD-ORIGIN-001 | Wrong origin, HTTP downgrade, unexpected/looping redirect or credential header propagation | Reject without staging |
| P2-DOWNLOAD-HASH-001 | Declared/streamed oversize, truncated body or archive digest mismatch | Delete attempt archive; ZIP and binary verifier remain unopened |
| P2-ARCHIVE-SAFETY-001 | Traversal, absolute/drive/backslash path, symlink, device name, entry/expanded-size overflow | Reject and clean only the current attempt |
| P2-EXEC-HASH-001 | Zero or multiple executable digest matches | Reject before binary execution |
| P2-BINARY-IDENTITY-001 | Invalid/non-AMD64 PE, probe timeout/output overflow/nonzero exit, version or commit mismatch | Reject completed stage |
| P2-STAGE-IDEMPOTENCE-001 | Existing incomplete, modified, extra-entry or reparse-backed final stage | Return conflict; never overwrite or delete it |

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

## Phase 1 config transaction component proof

CFG-001 through CFG-004 pass against disposable Windows TOML fixtures in `internal/configtxn`. CFG-001 injects external drift immediately before CAS and proves those bytes survive. CFG-002 covers pre-commit fault boundaries and replacement failure with the original target intact. CFG-003 uses both deterministic fault points and abrupt subprocess exits before and after `ReplaceFileW`, followed by a fresh engine recovery. CFG-004 changes an owned value and proves rollback returns `ErrRollbackConflict` without replacing the target. These are E3 component results; installed Codex acceptance remains open.

Phase 1 exit repair adds `CFG-PATH-001` ancestor reparse rejection, `CFG-RECOVERY-ARTIFACT-001` marker/backup/candidate substitution and cleanup retry, and `CFG-ROLLBACK-STATUS-001` persistent/idempotent conflict classification. All pass on disposable Windows fixtures; no real config is used.

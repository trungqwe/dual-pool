# Risk Register

Scale: likelihood and impact are Low/Medium/High. Owner is the role responsible for resolution.

| ID | Risk | Likelihood | Impact | Mitigation/trigger | Owner | Status |
|---|---|---|---|---|---|---|
| R-01 | Antigravity base URL or envelope is unsupported/changes | High | High | Phase 0 proof; version fingerprint; disable override on mismatch | Integration | Open |
| R-02 | Codex picker resets custom provider | Medium | High | Mandatory UI+network proof; supported profile alternative; no patch | Codex | Open |
| R-03 | CLIProxyAPI upstream schema changes rapidly | High | High | Pin version; versioned adapter; staged upgrades | Core | Open |
| R-04 | OAuth callback ports conflict | Medium | Medium | Discover/serialize flows; port diagnostics | Accounts | Open |
| R-05 | Environment key unavailable to extension host | Medium | High | Probe launch environment; evaluate command auth/secure user mechanism | Codex/Security | Open |
| R-06 | Two processes increase operational complexity | Medium | Low | Shared supervisor and generated configs; explicit identity | Core | Accepted |
| R-07 | Session affinity unavailable for some request shapes | Medium | Medium | Shape evidence; documented fallback; version gate | Routing | Open |
| R-08 | Multiplicative retries duplicate work/tools | Medium | High | Coordinate retry layers; no post-first-byte replay | Reliability | Open |
| R-09 | Auth metadata exposes account identity | Medium | Medium | Opaque IDs, local nickname separation, evidence redaction | Security | Open |
| R-10 | Config parser rewrites user files | Medium | High | Golden preservation tests; conflict-aware patch/rollback | Config | Open |
| R-11 | Updater executes tampered artifact | Low | Critical | Allowlist, checksum, optional signature, stage before execute | Security | Open |
| R-12 | Subscription/provider policy changes | Medium | High | Surface terms/risk; pin behavior; no bypass promises | Owner | Open |
| R-13 | Remote repo already has conflicting docs/code | High until inspected | High | Authenticated inventory and conflict-aware integration first | Delivery | Open |
| R-14 | Real live tests leak content/secrets | Medium | Critical | Synthetic prompts, metadata-only evidence, scanners | QA/Security | Open |
| R-15 | Windows process/ACL behavior differs across installs | Medium | High | Target build matrix and explicit ACL/process tests | Platform | Open |
| R-16 | Catalog fallback overrides desired Codex catalog updates | Medium | Medium | Use only when necessary; exact-version full catalog; invalidate on update | Codex | Open |
| R-17 | Provider model entitlement differs across accounts | High | Medium | Per-credential eligibility; do not infer by plan/account | Accounts | Open |
| R-18 | Bridge becomes single point of failure for Antigravity | Medium | High | Native passthrough in degraded mode; tested emergency rollback | Integration | Open |

## Escalation

Critical/high residual risks require an owner decision before the affected phase exits. Closing a risk requires evidence or an ADR, not a prose assertion. Reopen on relevant upstream/IDE version changes.

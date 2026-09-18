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

## Update rule

No requirement may be marked DONE until every mapped mandatory test passes for the pinned release/target environment. Add new rows when behavior is discovered; never remove a mapping solely because implementation is difficult.

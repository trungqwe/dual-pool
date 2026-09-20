# Phase 2 — format and recovery correction

- Start HEAD: `124c5e5183c858b39f3fe88817cf85e44c09e1dd`.
- Historical CI `35500189251`: **FAIL — Go format**. Later CI gates were skipped.
- Real lifecycle: NOT RUN and unauthorized.

The correction applies Go formatting and restores fail-closed handling for an impossible manifest-only candidate. A marker-owned candidate is removable only when it is empty, contains an executable alone, or contains an executable plus an optional manifest; manifest-only, unknown, nested and reparse artifacts fail closed.

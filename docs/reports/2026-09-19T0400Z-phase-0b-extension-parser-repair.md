# Phase 0B extension parser repair — baseline-only probe

## Header

- Run ID: `phase-0b-extension-parser-repair-20260919T0400Z`
- Start HEAD: `c252db6946aadd221b9b86c92ca5aef481be36c4`
- Branch: `phase-0/reversible-compatibility`
- Repository: `https://github.com/trungqwe/dual-pool.git`
- Worktree: pre-existing probe-generated modifications are present in earlier Phase 0B evidence files; they will be preserved and are outside this v2 evidence set.

## Objective

Repair recorder stage attribution and prove one baseline Codex extension request through a second isolated Antigravity instance. This run will not open the model picker, send an Astra request, test Cloud Code, or begin Phase 1.

## Audit finding

The historical recorder wrapped decode, UTF-8 conversion, JSON parsing, model extraction, and structural sanitization in one broad catch. A sanitizer/shape exception could therefore be reported as an invalid JSON or encoding failure. The previous `EXTENSION_PAYLOAD_NOT_PARSEABLE` classification is not proven.

The audit correction is: the earlier observation showed `POST /v1/responses`, `application/json`, identity content encoding, decoded length 58,422, and an Authorization header; its classification is now treated prospectively as `RECORDER_PARSE_STAGE_AMBIGUOUS`.

## Planned baseline experiment

Use a new recorder v2 with independent content decode, fatal UTF-8 decode, JSON parse, allowlisted structural summary, authorization, model, and prompt checks. Run parser and serializer self-tests first. Then use the proven parallel topology: protected original IDE, temporary user-data, temporary extension silo, temporary `CODEX_HOME`, empty workspace, and loopback recorder.

## Security boundary and stop conditions

No auth state, cookies, sessions, raw request body, body fragments, user prompts, or real provider account may enter evidence. Stop on parser self-test failure, missing isolation flags, protected PID loss, real-state hash drift, isolated authentication requirement, recorder stage failure, or cleanup failure.

## Acceptance

U-005 becomes `PROBED` only when one baseline request proves loopback Responses route, synthetic Authorization, fatal UTF-8 success, JSON parse success, baseline model, prompt sentinel, safe structure summary, valid SSE response, and cleanup. U-006 remains `BLOCKED` by design for this run.

## Delivery

Create a new append-only evidence directory `evidence/phase-0b-codex-extension-v2/`, update current mutable status docs only after the baseline result, run security/link/hash/cleanup checks, then commit and push normally with a separate handoff receipt.

# Phase 2 — tightly bounded pre-live closure

- Start HEAD: `8710e5f02a9e9259103e8ff4de04edc2f29e059e`.
- Historical Source CI: `35499318383` PASS.
- Real lifecycle is NOT RUN and unauthorized from this starting commit.

This run corrects marker transaction validation, partial candidate recovery and listener proof before any real lifecycle mutation. It preserves all previous process-ownership and fail-closed behavior.

## Local verification

`P2-INSTALL-MARKER-TXN-001`, synthetic TCP decoder tests, `P2-TCP4-WINDOWS-ABI-001`, and `P2-TCP6-WINDOWS-ABI-001` passed locally. The IPv6 test may skip only when IPv6 loopback is unavailable. Full Go tests, vet, build, module verification, Node tests and `UPSTREAM_LOCK_VALID` passed. Race and Source CI are pending. Real lifecycle is NOT RUN.

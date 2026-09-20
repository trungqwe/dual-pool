# Phase 2 — gated real lifecycle harness

- Start HEAD: `f06bab56615c576ed5774e9fe34c6a7d760ebb2e`.
- Historical Source CI: `35500953921` PASS.
- Real lifecycle execution: NOT RUN; this harness requires its own Source CI PASS.

`TestRealLifecycle` now skips before mutation unless `DUALPOOL_RUN_REAL_LIFECYCLE=1`. With the gate set it performs read-only product/config/port preflight, stages the pinned release, installs or reuses the protected binary, starts both instances, verifies idempotent starts and status, restarts, stops twice, and checks configuration bytes plus listener cleanup. It uses only the lifecycle's health, model-list and management debug probes; it performs no OAuth or provider request.

Local full Go test, vet, build, module, Node and upstream-lock verification passed. Hosted CI and the real gate remain pending.

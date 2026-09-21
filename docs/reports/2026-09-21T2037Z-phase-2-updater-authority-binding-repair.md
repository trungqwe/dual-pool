# Phase 2 updater authority-binding repair

## Header

- Run ID: `phase-2-updater-authority-binding-repair`
- Date/time UTC: 2026-09-21T20:37Z
- Roadmap phase: Phase 2
- Branch: `phase-2/updater-manager-composition`
- Reviewed start HEAD: `87375acffc8c20a16d22d3f52e14da9578c7d365`
- Authoritative upstream: `phase-2/upstream-lifecycle` at `a3060848f7b23edfbfb6c9b780841e7d5d6879eb` (unchanged)

## Repair

The prior exported `ComposeUpdater(manager, update.Config)` could create an updater using a different lock, state repository, verifier, marker directory or marker security from the supplied Manager. The public constructor now accepts only `(*Manager, update.Smoke)`. It derives every transaction authority from the validated Manager and creates trusted Windows marker security itself.

`lockfile.Manager.MatchesRoot` validates a supplied root using the same canonicalization, directory and reparse-safe checks as construction. Both `instance.New` and `installedslot.New` require every selected lock manager, including an injected one, to match `layout.Locks`; a valid but distinct lock root fails closed.

## Evidence

| Claim | Status | Test |
|---|---|---|
| Public composition cannot accept caller-selected transaction authority | PASS_COMPONENT | `TestComposeUpdaterDoesNotAcceptCallerSelectedAuthority`, `TestComposeUpdaterCannotSplitManagerAuthority` |
| Updater uses the exact Manager lock, state, verifier and State marker directory | PASS_COMPONENT | `TestComposeUpdaterUsesExactManagerAuthorities` |
| Manager rejects a valid but different lock root | PASS_COMPONENT | `TestInjectedLockManagerMustMatchLayoutLocks` |
| Registry rejects a valid but different lock root and explicit nil | PASS_COMPONENT | `TestRegistryInjectedLockManagerMustMatchLayoutLocks`, `TestRegistryExplicitNilLockManagerFailsClosed` |
| Runtime still shares exact lock, Store, Registry and Manager identities | PASS_COMPONENT | `TestRuntimeCompositionSharesExactObjects`, `TestRuntimePrivateLifecycleUsesExactManager` |

The regression matrix, four targeted 20-run suites, four 50-run stress suites, four ten-run race suites, repository tests and repository race suite passed. Node validation, pinned-lock validation, JSON, documentation links, privacy, secret scan, scanner positive control and `git diff --check` passed.

## Claim boundary

Proof remains `PASS_COMPONENT`: protected TEMP-only fixtures, synthetic process records and deterministic hooks. Recovery remains limited to same-process injected fault boundaries. No subprocess crash, OS crash or power-loss claim is made. No updater, registry, child process, listener, provider, OAuth, credential, product or user mutation occurred.

## Delivery

The scoped code and documentation/evidence commits are pushed normally to `phase-2/updater-manager-composition`; exact final Source CI is the delivery receipt. No upstream ref is moved. The next objective is independent review of the delivered authority-binding diff before any controlled upstream delivery.

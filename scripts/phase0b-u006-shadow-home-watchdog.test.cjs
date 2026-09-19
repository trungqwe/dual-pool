'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { POLICY, sha256, safeTempRoot, copyOpaqueCodexHome, writeShadowConfig, probeEnvironment, cleanupShadow } = require('./phase0b-u006-shadow-home-watchdog.cjs');

test('policy proves real config cannot be part of crash recovery', () => {
  assert.equal(POLICY.real_config_mutation_capability, false);
  assert.equal(POLICY.crash_requires_restore, false);
});

test('opaque full-home fixture copy leaves source bytes unchanged', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'dual-pool-shadow-fixture-'));
  const source = path.join(root, 'source'), shadow = path.join(os.tmpdir(), 'dual-pool-u006-shadow-' + 'a'.repeat(32));
  fs.mkdirSync(source, { recursive: true }); fs.writeFileSync(path.join(source, 'config.toml'), 'original');
  fs.writeFileSync(path.join(source, 'opaque-state.bin'), Buffer.from([0, 1, 2, 3]));
  try {
    const before = sha256(path.join(source, 'config.toml')); copyOpaqueCodexHome(source, shadow);
    writeShadowConfig(shadow, 12345);
    assert.equal(sha256(path.join(source, 'config.toml')), before);
    assert.match(fs.readFileSync(path.join(shadow, 'config.toml'), 'utf8'), /dualpool_probe/);
    assert.equal(fs.readFileSync(path.join(shadow, 'opaque-state.bin'))[2], 2);
  } finally { cleanupShadow(shadow); fs.rmSync(root, { recursive: true, force: true }); }
});

test('probe environment isolates shadow and removes stale probe variables', () => {
  const env = probeEnvironment({ CODEX_HOME: 'real', DUALPOOL_CODEX_KEY: 'old', ELECTRON_RUN_AS_NODE: '1', PATH: 'keep' }, 'shadow', 'synthetic');
  assert.equal(env.CODEX_HOME, 'shadow'); assert.equal(env.DUALPOOL_CODEX_KEY, 'synthetic');
  assert.equal(env.ELECTRON_RUN_AS_NODE, undefined); assert.equal(env.PATH, 'keep');
});

test('unsafe roots cannot be copied or cleaned', () => {
  assert.equal(safeTempRoot(path.join(os.tmpdir(), 'wrong-root')), false);
  assert.throws(() => cleanupShadow(path.join(os.tmpdir(), 'wrong-root')), /UNSAFE_SHADOW_ROOT/);
});

test('script has no primary transaction or normal relaunch path', () => {
  const text = fs.readFileSync(path.join(__dirname, 'phase0b-u006-shadow-home-watchdog.cjs'), 'utf8');
  assert.equal(text.includes('phase0b-primary-transaction'), false);
  assert.equal(text.includes('manualNormal'), false);
  assert.match(text, /SHADOW_CLEANUP_PASS/);
});

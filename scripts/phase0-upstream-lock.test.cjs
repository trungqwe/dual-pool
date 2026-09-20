'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { validate, loadAndValidate } = require('./phase0-upstream-lock.cjs');

const lockPath = path.resolve(__dirname, '..', 'upstream.lock');

test('candidate lock pins the independently verified Windows artifact', () => {
  const lock = loadAndValidate(lockPath);
  assert.equal(lock.status, 'candidate');
  assert.equal(lock.product, 'CLIProxyAPI');
  assert.equal(lock.version, '7.3.7');
  assert.equal(lock.commit, 'b773607e3e7756dc6020a291825e4eb08899595a');
  assert.equal(lock.platforms.windows_amd64.archive_sha256, 'da5466b81beb7c769b99e26a5f6f41d9999a07be7c36be170167f10a2a6ecfc7');
  assert.equal(lock.platforms.windows_amd64.executable_sha256, 'bb44c6fa6a30214a294bdf64cf7385aa7ff65982aebd501ae37dc6a62a227072');
});

test('candidate lock cannot claim credentialed provider support', () => {
  const lock = loadAndValidate(lockPath);
  assert.equal(lock.config_adapter_version, 'dualpool-cpa-v7.3.7-config-v1');
  assert.ok(lock.unverified_capabilities.includes('credential_specific_model_inventory'));
  assert.ok(lock.unverified_capabilities.includes('provider_specific_response_shapes'));
  assert.ok(lock.unverified_capabilities.includes('dedicated_health_endpoint'));
  assert.doesNotMatch(JSON.stringify(lock), /\/latest(?:\/|"|$)/i);
});

test('unknown verified capability is rejected', () => {
  const lock = structuredClone(loadAndValidate(lockPath));
  lock.verified_capabilities.push('invented_provider_support');
  assert.throws(() => validate(lock), /LOCK_VERIFIED_CAPABILITY_UNKNOWN/);
});

test('duplicate verified capability is rejected', () => {
  const lock = structuredClone(loadAndValidate(lockPath));
  lock.verified_capabilities.push(lock.verified_capabilities[0]);
  assert.throws(() => validate(lock), /LOCK_VERIFIED_CAPABILITY_DUPLICATE/);
});

test('non-string verified capability is rejected', () => {
  const lock = structuredClone(loadAndValidate(lockPath));
  lock.verified_capabilities.push({ name: 'loopback_ipv4_bind' });
  assert.throws(() => validate(lock), /LOCK_VERIFIED_CAPABILITY_UNKNOWN/);
});

test('pinned config adapter rejects unimplemented and mismatched claims', () => {
  const base = loadAndValidate(lockPath);
  for (const value of ['UNIMPLEMENTED', '', 'dualpool-cpa-v7.3.8-config-v1', 'other']) {
    const lock = structuredClone(base);
    lock.config_adapter_version = value;
    assert.throws(() => validate(lock), /LOCK_ADAPTER_CLAIM_INVALID/);
  }
});

test('download URL must exactly bind tag and artifact without URL metadata', () => {
  const base = loadAndValidate(lockPath);
  for (const suffix of ['?download=1', '#fragment', '.extra']) {
    const lock = structuredClone(base);
    lock.platforms.windows_amd64.download_url += suffix;
    assert.throws(() => validate(lock), /LOCK_URL_INVALID/);
  }
});

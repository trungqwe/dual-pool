'use strict';

const fs = require('node:fs');
const path = require('node:path');

const SHA256 = /^[a-f0-9]{64}$/;
const COMMIT = /^[a-f0-9]{40}$/;
const VERIFIED_CAPABILITIES = new Set([
  'published_checksum_matches_archive',
  'github_asset_digest_matches_archive',
  'binary_version_matches_tag',
  'loopback_ipv4_bind',
  'management_key_required',
  'client_key_required',
  'credential_free_start_stop_cleanup',
]);

function fail(code) {
  throw new Error(code);
}

function validate(lock) {
  if (!lock || lock.schema_version !== 1) fail('LOCK_SCHEMA_INVALID');
  if (lock.product !== 'CLIProxyAPI') fail('LOCK_PRODUCT_INVALID');
  if (lock.status !== 'candidate') fail('LOCK_STATUS_INVALID');
  if (!/^\d+\.\d+\.\d+$/.test(lock.version) || lock.tag !== `v${lock.version}`) fail('LOCK_VERSION_INVALID');
  if (!COMMIT.test(lock.commit)) fail('LOCK_COMMIT_INVALID');
  if (lock.version !== '7.3.7' || lock.commit !== 'b773607e3e7756dc6020a291825e4eb08899595a' || lock.config_adapter_version !== 'dualpool-cpa-v7.3.7-config-v1') fail('LOCK_ADAPTER_CLAIM_INVALID');

  const platform = lock.platforms?.windows_amd64;
  if (!platform || !SHA256.test(platform.archive_sha256) || !SHA256.test(platform.executable_sha256)) fail('LOCK_HASH_INVALID');
  const expectedDownloadURL = `https://github.com/router-for-me/CLIProxyAPI/releases/download/${lock.tag}/${platform.artifact}`;
  if (platform.download_url !== expectedDownloadURL || /\/latest(?:\/|$)/i.test(platform.download_url)) fail('LOCK_URL_INVALID');
  if (lock.release_metadata_url !== `https://github.com/router-for-me/CLIProxyAPI/releases/tag/${lock.tag}`) fail('LOCK_METADATA_URL_INVALID');

  if (!Array.isArray(lock.verified_capabilities) || !Array.isArray(lock.unverified_capabilities)) fail('LOCK_CAPABILITIES_INVALID');
  const seenVerified = new Set();
  for (const capability of lock.verified_capabilities) {
    if (!VERIFIED_CAPABILITIES.has(capability)) fail('LOCK_VERIFIED_CAPABILITY_UNKNOWN');
    if (seenVerified.has(capability)) fail('LOCK_VERIFIED_CAPABILITY_DUPLICATE');
    seenVerified.add(capability);
  }
  const overlap = lock.verified_capabilities.filter(value => lock.unverified_capabilities.includes(value));
  if (overlap.length) fail('LOCK_CAPABILITY_CONFLICT');
  for (const required of ['credential_specific_model_inventory', 'provider_specific_response_shapes', 'dedicated_health_endpoint']) {
    if (!lock.unverified_capabilities.includes(required)) fail('LOCK_UNVERIFIED_CAPABILITY_MISSING');
  }
  return lock;
}

function loadAndValidate(file) {
  const resolved = path.resolve(file);
  return validate(JSON.parse(fs.readFileSync(resolved, 'utf8')));
}

if (require.main === module) {
  const file = process.argv[2] || path.resolve(__dirname, '..', 'upstream.lock');
  loadAndValidate(file);
  console.log('UPSTREAM_LOCK_VALID');
}

module.exports = { validate, loadAndValidate };

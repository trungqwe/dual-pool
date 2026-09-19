'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { evaluate } = require('./phase0b-primary-evidence.cjs');
const digest = 'a'.repeat(64);
function valid() { return {
  config: { original_state: 'PRESENT', config_before_sha256: digest, config_after_sha256: digest, byte_for_byte_restore: true },
  controls: { settings_before_sha256: digest, settings_after_sha256: digest, settings_unchanged: true,
    extension_before_sha256: digest, extension_after_sha256: digest, extension_unchanged: true,
    normal_relaunch: true, listener_absent: true },
  cleanup: { temp_root_removed: true }, authentication_confirmed: true, astra_selected: true, astra_visible: true,
  target_request_captured: true, target_model: 'gpt-6-astra', wire_method: 'POST', wire_route: '/v1/responses',
  custom_provider_retained: true, json_parse_status: 'PASS', synthetic_auth: 'PASS', prompt_sentinel: 'PASS',
  response_lifecycle: true, emitted_events: ['response.created', 'response.completed']
}; }
test('complete synthetic record proves consistency only', () => {
  assert.equal(evaluate(valid()).status, 'PASS'); assert.equal(evaluate(valid()).live_state_verified, false);
});
test('missing record fails closed', () => assert.equal(evaluate({}).status, 'BLOCKED'));
test('unchanged requires after hash', () => {
  const r = valid(); delete r.controls.extension_after_sha256;
  assert.ok(evaluate(r).failures.includes('EXTENSION_UNCHANGED_UNPROVEN'));
});
test('hash string is not a boolean', () => {
  const r = valid(); r.controls.settings_unchanged = digest;
  assert.ok(evaluate(r).failures.includes('SETTINGS_UNCHANGED_UNPROVEN'));
});
test('restore claim cannot hide differing bytes', () => {
  const r = valid(); r.config.config_after_sha256 = 'b'.repeat(64);
  assert.ok(evaluate(r).failures.includes('CONFIG_RESTORE_UNPROVEN'));
});
for (const key of ['wire_method', 'json_parse_status', 'synthetic_auth', 'target_model']) {
  test(`missing ${key} blocks target`, () => {
    const r = valid(); delete r[key]; assert.ok(evaluate(r).failures.includes('TARGET_WIRE_UNPROVEN'));
  });
}
test('response closure requires completed event', () => {
  const r = valid(); r.emitted_events = ['response.created'];
  assert.ok(evaluate(r).failures.includes('LIFECYCLE_UNPROVEN'));
});

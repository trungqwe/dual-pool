'use strict';
const fs = require('node:fs');
const path = require('node:path');
function evaluate(r) {
  const failures = [];
  const need = (ok, code) => { if (!ok) failures.push(code); };
  const c = r.config || {}, controls = r.controls || {};
  const hash = v => typeof v === 'string' && /^[a-f0-9]{64}$/i.test(v);
  const restored = c.original_state === 'ABSENT'
    ? c.config_before_sha256 === 'ABSENT' && c.config_after_sha256 === 'ABSENT'
    : c.original_state === 'PRESENT' && hash(c.config_before_sha256) && c.config_before_sha256 === c.config_after_sha256;
  need(restored && c.byte_for_byte_restore === true, 'CONFIG_RESTORE_UNPROVEN');
  for (const k of ['settings', 'extension']) need(hash(controls[`${k}_before_sha256`]) &&
    controls[`${k}_before_sha256`] === controls[`${k}_after_sha256`] && controls[`${k}_unchanged`] === true,
    `${k.toUpperCase()}_UNCHANGED_UNPROVEN`);
  need(controls.normal_relaunch === true, 'NORMAL_RELAUNCH_UNPROVEN');
  need(controls.listener_absent === true && r.cleanup?.temp_root_removed === true, 'CLEANUP_UNPROVEN');
  need(r.authentication_confirmed === true, 'AUTH_UNPROVEN');
  need(r.astra_selected === true && r.astra_visible === true, 'PICKER_UNPROVEN');
  need(r.target_request_captured === true && r.target_model === 'gpt-6-astra' &&
    r.wire_method === 'POST' && r.wire_route === '/v1/responses' && r.custom_provider_retained === true &&
    r.json_parse_status === 'PASS' && r.synthetic_auth === 'PASS' && r.prompt_sentinel === 'PASS', 'TARGET_WIRE_UNPROVEN');
  need(r.response_lifecycle === true && r.emitted_events?.includes('response.created') &&
    r.emitted_events?.includes('response.completed'), 'LIFECYCLE_UNPROVEN');
  return { status: failures.length ? 'BLOCKED' : 'PASS', failures,
    config_restore_record_consistent: restored && c.byte_for_byte_restore === true, live_state_verified: false };
}
if (require.main === module) {
  try {
    const root = path.resolve(__dirname, '..');
    const supplied = process.argv[2];
    const filename = path.resolve(root, supplied ||
      'evidence/phase-0b-u006-primary-profile/watchdog-result-20260919T075105857Z.json');
    if (!filename.startsWith(path.join(root, 'evidence') + path.sep)) throw new Error('UNSAFE_RESULT_PATH');
    const r = JSON.parse(fs.readFileSync(filename, 'utf8'));
    const assessment = evaluate(r);
    console.log(JSON.stringify({ schema_version: '2', ...assessment, historical_gate_superseded: !supplied,
      primary_mutation_allowed: false, scope: 'RECORDED_EVIDENCE_ONLY' }, null, 2));
    process.exitCode = supplied && assessment.status === 'PASS' && r.status === 'PASS' ? 0 : 1;
  } catch { console.error('PRIMARY_EVIDENCE_INVALID'); process.exitCode = 1; }
}
module.exports = { evaluate };

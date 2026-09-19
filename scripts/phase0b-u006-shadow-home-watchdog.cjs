'use strict';
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const crypto = require('node:crypto');
const { spawn } = require('node:child_process');
const readline = require('node:readline');

const EXPECTED_REAL_CONFIG_SHA256 = '1AE4E3BC2C1185EA4C9C863BA481D66F5C160B02B63F4C04F1D885754257470D';
const POLICY = Object.freeze({ real_config_mutation_capability: false, crash_requires_restore: false });
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

function sha256(file) {
  return fs.existsSync(file) ? crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex').toUpperCase() : 'ABSENT';
}

function safeTempRoot(root) {
  const temp = path.resolve(os.tmpdir());
  const resolved = path.resolve(root);
  return path.dirname(resolved).toLowerCase() === temp.toLowerCase() &&
    /^dual-pool-u006-shadow-[a-f0-9]{32}$/.test(path.basename(resolved));
}

function copyOpaqueCodexHome(source, shadow) {
  if (!fs.existsSync(source) || !fs.statSync(source).isDirectory()) throw new Error('CODEX_HOME_SOURCE_ABSENT');
  if (!safeTempRoot(shadow)) throw new Error('UNSAFE_SHADOW_ROOT');
  if (fs.existsSync(shadow)) throw new Error('SHADOW_ROOT_ALREADY_EXISTS');
  fs.mkdirSync(shadow, { recursive: true });
  fs.cpSync(source, shadow, { recursive: true, force: false, errorOnExist: false });
  if (!fs.existsSync(path.join(shadow, 'config.toml'))) throw new Error('SHADOW_CONFIG_ABSENT');
  return { source_untouched: true, shadow_created: true };
}

function writeShadowConfig(shadow, port) {
  if (!safeTempRoot(shadow)) throw new Error('UNSAFE_SHADOW_ROOT');
  if (!Number.isInteger(port) || port < 1024 || port > 65535) throw new Error('INVALID_RECORDER_PORT');
  const config = path.join(shadow, 'config.toml');
  const staged = config + '.shadow-stage-' + crypto.randomBytes(8).toString('hex');
  const text = '# Dual Pool shadow-only U-006 probe\nmodel_provider = "dualpool_probe"\n\n' +
    '[model_providers.dualpool_probe]\nname = "Dual Pool U-006 shadow"\n' +
    `base_url = "http://127.0.0.1:${port}/v1"\nwire_api = "responses"\nenv_key = "DUALPOOL_CODEX_KEY"\n`;
  fs.writeFileSync(staged, text, { encoding: 'utf8', flag: 'wx' });
  fs.renameSync(staged, config);
  return { shadow_config_modified: true, real_config_mutation_capability: false };
}

function probeEnvironment(base, shadow, secret) {
  const env = { ...base };
  for (const name of ['ELECTRON_RUN_AS_NODE', 'CODEX_HOME', 'DUALPOOL_CODEX_KEY', 'NODE_OPTIONS']) delete env[name];
  env.CODEX_HOME = shadow;
  env.DUALPOOL_CODEX_KEY = secret;
  return env;
}

function writeSafe(file, value, secret, prompt) {
  const text = JSON.stringify(value, null, 2) + '\n';
  if (text.includes(secret) || text.includes(prompt) || /\b[A-Z]:[\\/]/i.test(text)) throw new Error('UNSAFE_EVIDENCE');
  const staged = file + '.stage';
  fs.writeFileSync(staged, text, { encoding: 'utf8', flag: 'wx' });
  fs.renameSync(staged, file);
}

async function waitFor(predicate, timeout, code) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) { if (await predicate()) return; await sleep(500); }
  throw new Error(code);
}

function cleanupShadow(shadow) {
  if (!safeTempRoot(shadow)) throw new Error('UNSAFE_SHADOW_ROOT');
  if (fs.existsSync(shadow)) fs.rmSync(shadow, { recursive: true, force: false });
  return !fs.existsSync(shadow);
}

async function main() {
  const repo = path.resolve(__dirname, '..');
  const realHome = path.join(os.homedir(), '.codex');
  const realConfig = path.join(realHome, 'config.toml');
  const observedHash = sha256(realConfig);
  if (observedHash !== EXPECTED_REAL_CONFIG_SHA256) throw new Error('REAL_CONFIG_BASELINE_DRIFT');
  if (!fs.existsSync(realHome)) throw new Error('REAL_CODEX_HOME_ABSENT');
  const ownedShadow = path.join(os.tmpdir(), 'dual-pool-u006-shadow-' + crypto.randomBytes(16).toString('hex'));
  const secret = 'SECRET_SENTINEL_' + crypto.randomBytes(24).toString('hex');
  const prompt = 'ASTRA_PROMPT_SENTINEL_' + crypto.randomBytes(24).toString('hex');
  let recorder = null, probe = null, port = null, capture = null, classification = 'RUNNING';
  const result = { schema_version: 1, test_id: 'P0B-CX-SHADOW-HOME-001', primary_profile_used: true,
    policy: POLICY, real_config_before_sha256: observedHash, real_config_after_sha256: null,
    real_config_mutation_capability: false, crash_requires_restore: false, auth_confirmed: false,
    astra_selected: false, target_request_captured: false, target_model: null, status: 'BLOCKED' };
  const stateDir = path.join(ownedShadow, '.dualpool-state');
  const writeState = stage => writeSafe(path.join(stateDir, 'status.json'), { stage, policy: POLICY }, secret, prompt);
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  const answer = async (message, allowed) => {
    console.log(message + '\n' + allowed.join(' / '));
    return new Promise(resolve => rl.once('line', line => resolve(line.trim().toUpperCase())));
  };
  try {
    writeState('PRECHECK'); copyOpaqueCodexHome(realHome, ownedShadow); fs.mkdirSync(stateDir, { recursive: true }); writeState('PRECHECK_COPY_COMPLETE');
    writeState('SHADOW_HOME_READY');
    const recorderScript = path.join(repo, 'scripts', 'phase0b-primary-recorder.cjs');
    const recorderRoot = path.join(ownedShadow, '.dualpool-recorder'); fs.mkdirSync(recorderRoot);
    recorder = spawn(process.execPath, [recorderScript], { windowsHide: true, stdio: 'ignore',
      env: { ...process.env, P0B_PRIMARY_ROOT: recorderRoot, P0B_PRIMARY_SECRET: secret, P0B_PRIMARY_PROMPT: prompt } });
    await waitFor(() => fs.existsSync(path.join(recorderRoot, 'ready.json')), 15000, 'RECORDER_NOT_READY');
    const ready = JSON.parse(fs.readFileSync(path.join(recorderRoot, 'ready.json'), 'utf8')); port = ready.port;
    writeShadowConfig(ownedShadow, port); writeState('SHADOW_CONFIG_ACTIVE');
    const authChoice = await answer('Mở Codex trong probe và xác nhận tài khoản thường dùng.', ['AUTH_OK', 'AUTH_LOST']);
    if (authChoice !== 'AUTH_OK') throw new Error('FULL_CODEX_HOME_SHADOW_AUTH_NOT_RECOGNIZED');
    result.auth_confirmed = true; result.auth_checkpoint = 'OWNER_CONFIRMED';
    const astraChoice = await answer('Chọn GPT-6 Astra, chưa gửi prompt.', ['ASTRA_SELECTED', 'ASTRA_NOT_VISIBLE']);
    if (astraChoice !== 'ASTRA_SELECTED') throw new Error('FULL_CODEX_HOME_SHADOW_AUTH_NOT_RECOGNIZED');
    result.astra_selected = true;
    fs.writeFileSync(path.join(recorderRoot, 'arm'), ''); writeState('WAIT_ASTRA_SENTINEL');
    console.log('Gửi đúng một prompt có sentinel hiển thị trong console.');
    await waitFor(() => fs.existsSync(path.join(recorderRoot, 'capture-u006.json')), 10 * 60 * 1000, 'TARGET_REQUEST_TIMEOUT');
    capture = JSON.parse(fs.readFileSync(path.join(recorderRoot, 'capture-u006.json'), 'utf8'));
    result.target_request_captured = true; result.target_model = capture.model;
    if (capture.model_exact_astra !== true || capture.auth_match_status !== 'PASS' ||
      capture.prompt_match_status !== 'PASS' || capture.response_closed !== true) throw new Error('SHADOW_WIRE_ACCEPTANCE_FAILED');
    classification = 'PASS';
  } catch (error) { classification = error.message; }
  finally {
    try { rl.close(); } catch {}
    console.log('Đóng probe-mode Antigravity trước khi dọn shadow.');
    if (probe) await waitFor(() => probe.exitCode !== null, 20 * 60 * 1000, 'PROBE_CLOSE_TIMEOUT');
    if (recorder && recorder.exitCode === null) { try { recorder.kill(); } catch {} }
    result.shadow_cleanup_pass = cleanupShadow(ownedShadow);
    result.real_config_after_sha256 = sha256(realConfig);
    result.classification = classification; result.status = classification === 'PASS' && result.shadow_cleanup_pass ? 'PASS' : 'BLOCKED';
    const evidence = path.join(repo, 'evidence', 'phase-0b-u006-shadow-home'); fs.mkdirSync(evidence, { recursive: true });
    writeSafe(path.join(evidence, 'shadow-result.json'), result, secret, prompt);
    console.log(result.shadow_cleanup_pass ? 'SHADOW_CLEANUP_PASS' : 'SHADOW_CLEANUP_FAILED');
    console.log('Hãy tự mở Antigravity bằng shortcut/Menu Start bình thường.');
  }
  process.exitCode = result.status === 'PASS' ? 0 : 1;
}

if (require.main === module) main().catch(error => { console.error(error.message); process.exitCode = 1; });
module.exports = { POLICY, sha256, safeTempRoot, copyOpaqueCodexHome, writeShadowConfig, probeEnvironment, cleanupShadow };

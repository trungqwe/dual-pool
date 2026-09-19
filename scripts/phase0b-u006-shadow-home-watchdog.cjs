'use strict';

const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const crypto = require('node:crypto');
const { spawn, spawnSync } = require('node:child_process');
const readline = require('node:readline');

const POLICY = Object.freeze({ real_config_mutation_capability: false, crash_requires_restore: false });
const PREFIX = 'dual-pool-u006-shadow-';
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));

function sha256(file) {
  return fs.existsSync(file) ? crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex').toUpperCase() : 'ABSENT';
}

function safeSessionRoot(root) {
  const resolved = path.resolve(root);
  return path.dirname(resolved).toLowerCase() === path.resolve(os.tmpdir()).toLowerCase() &&
    new RegExp('^' + PREFIX + '[a-f0-9]{32}$').test(path.basename(resolved));
}

function safeChild(parent, child) {
  if (!safeSessionRoot(parent)) return false;
  const root = path.resolve(parent); const resolved = path.resolve(child);
  return resolved !== root && resolved.startsWith(root + path.sep) &&
    (!fs.existsSync(resolved) || !fs.lstatSync(resolved).isSymbolicLink());
}

function assertNoReparse(root) {
  if (fs.existsSync(root) && fs.lstatSync(root).isSymbolicLink()) throw new Error('UNSAFE_REPARSE_POINT');
}

function validateTopology(root, home, state, recorder) {
  if (!safeSessionRoot(root) || ![home, state, recorder].every(child => safeChild(root, child))) throw new Error('UNSAFE_SHADOW_TOPOLOGY');
  assertNoReparse(root);
}

function parseTomlStructure(text) {
  const script = ['import json,sys,tomllib,re', 'data=tomllib.loads(sys.stdin.buffer.read().decode("utf-8"))',
    'providers=data.get("model_providers",{})', 'allow={"gpt-6-astra","gpt-5.6-sol","gpt-5.6-luna"}', 'models=[]',
    'def walk(v):', '  if isinstance(v,dict):', '    for k,x in v.items():', '      if isinstance(x,str) and x in allow: models.append(x)', '      walk(x)',
    '  elif isinstance(v,list):', '    for x in v: walk(x)', 'walk(data)', 'names=[k for k in providers if re.fullmatch(r"[A-Za-z0-9_.-]{1,80}",k)]',
    'print(json.dumps({"top_level_keys":sorted(k for k in data if re.fullmatch(r"[A-Za-z0-9_.-]{1,80}",k)),"provider_table_names":sorted(names),"allowlisted_model_slugs":sorted(set(models))},separators=(",",":")))'].join('\n');
  const run = spawnSync(process.env.PYTHON || 'python', ['-c', script], { input: text, encoding: 'utf8', windowsHide: true });
  if (run.status !== 0) return { toml_parse: 'FAIL', parse_error: 'TOML_PARSE_FAILED' };
  try { return { toml_parse: 'PASS', ...JSON.parse(run.stdout) }; } catch { return { toml_parse: 'FAIL', parse_error: 'TOML_STRUCTURE_OUTPUT_INVALID' }; }
}

function assessRealConfig(file) {
  if (!fs.existsSync(file)) throw new Error('REAL_CONFIG_ABSENT');
  const text = fs.readFileSync(file, 'utf8'); const structure = parseTomlStructure(text);
  const forbidden = { dualpool_probe_present: /dualpool_probe/i.test(text), DUALPOOL_CODEX_KEY_present: /DUALPOOL_CODEX_KEY/.test(text),
    known_old_probe_loopback_provider_present: /dualpool_probe/i.test(text) || (/127\.0\.0\.1|localhost/i.test(text) && /wire_api\s*=\s*["']responses["']/i.test(text)) };
  return { current_sha256: sha256(file), ...structure, forbidden_probe_markers: forbidden, raw_values_persisted: false,
    current_working_config_candidate: structure.toml_parse === 'PASS' && !Object.values(forbidden).some(Boolean) };
}

function runPowerShell(command, runner = spawnSync) {
  const result = runner('powershell.exe', ['-NoProfile', '-Command', command], { encoding: 'utf8', windowsHide: true, stdio: ['ignore', 'pipe', 'ignore'] });
  if (!result || result.error || result.status !== 0) throw new Error('PROCESS_OBSERVATION_FAILED');
  return result.stdout || '';
}

function getAntigravityProcessSnapshot({ runner = spawnSync } = {}) {
  const output = runPowerShell("Write-Output ((Get-CimInstance Win32_Process -Filter \"Name = 'Antigravity IDE.exe'\").ProcessId -join ',')", runner).trim();
  if (!output) return { pids: [], executable: null };
  if (!/^\d+(,\d+)*$/.test(output)) throw new Error('PROCESS_OBSERVATION_FAILED');
  return { pids: output.split(',').map(Number), executable: null };
}

function getAntigravityProcessCount(options = {}) {
  return getAntigravityProcessSnapshot(options).pids.length;
}

function discoverAntigravityExecutable({ runner = spawnSync, fallback = path.join(process.env.LOCALAPPDATA || '', 'Programs', 'Antigravity IDE', 'Antigravity IDE.exe') } = {}) {
  const output = runPowerShell("Write-Output ((Get-CimInstance Win32_Process -Filter \"Name = 'Antigravity IDE.exe'\" | Select-Object -First 1 -ExpandProperty ExecutablePath))", runner).trim();
  const candidate = output || fallback;
  if (!candidate || path.basename(candidate).toLowerCase() !== 'antigravity ide.exe' || !fs.existsSync(candidate)) throw new Error('ANTIGRAVITY_EXECUTABLE_NOT_FOUND');
  return candidate;
}

function applySessionAcl(root, { runner = spawnSync, fixture = true } = {}) {
  fs.mkdirSync(root, { recursive: true });
  const sid = runPowerShell('[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value', runner).trim();
  if (!/^S-\d-\d+(-\d+)+$/.test(sid)) throw new Error('SHADOW_SESSION_ACL_FAILED');
  const acl = runner('icacls.exe', [root, '/inheritance:r', '/grant:r', `*${sid}:(OI)(CI)F`, '*S-1-5-18:(OI)(CI)F'], { encoding: 'utf8', windowsHide: true, stdio: 'ignore' });
  if (!acl || acl.error || acl.status !== 0) throw new Error('SHADOW_SESSION_ACL_FAILED');
  if (fixture) {
    const file = path.join(root, '.acl-fixture-' + crypto.randomBytes(8).toString('hex'));
    try { fs.writeFileSync(file, 'fixture', { flag: 'wx' }); if (fs.readFileSync(file, 'utf8') !== 'fixture') throw new Error(); }
    catch { throw new Error('SHADOW_SESSION_ACL_FAILED'); }
    finally { try { fs.rmSync(file, { force: true }); } catch { throw new Error('SHADOW_SESSION_ACL_FAILED'); } }
  }
  return { acl_applied: true, inheritance_removed: true, system_retained: true, fixture_pass: fixture };
}

function copyOpaqueCodexHome(source, shadow, copyImpl = null) {
  const root = path.dirname(shadow);
  if (!fs.existsSync(source) || !fs.statSync(source).isDirectory()) throw new Error('CODEX_HOME_SOURCE_ABSENT');
  if (!safeSessionRoot(root) || !safeChild(root, shadow)) throw new Error('UNSAFE_SHADOW_ROOT');
  if (fs.existsSync(shadow)) throw new Error('SHADOW_ROOT_ALREADY_EXISTS');
  if (copyImpl) copyImpl(source, shadow);
  else { const run = spawnSync('robocopy', [source, shadow, '/E', '/COPY:DAT', '/DCOPY:DAT', '/XJ', '/R:0', '/W:0', '/NFL', '/NDL', '/NJH', '/NJS', '/NP'], { stdio: 'ignore', windowsHide: true }); if (run.error || run.status === null || run.status > 7) throw new Error('SHADOW_CODEX_HOME_COPY_FAILED'); }
  if (!fs.existsSync(path.join(shadow, 'config.toml'))) throw new Error('SHADOW_CONFIG_ABSENT');
  return { source_untouched: true, shadow_created: true, copy_complete: true };
}

function writeShadowConfig(shadow, port) {
  if (!fs.existsSync(shadow)) throw new Error('UNSAFE_SHADOW_HOME'); if (!Number.isInteger(port) || port < 1024 || port > 65535) throw new Error('INVALID_RECORDER_PORT');
  const file = path.join(shadow, 'config.toml'); const stage = file + '.shadow-stage-' + crypto.randomBytes(8).toString('hex');
  const text = '# Dual Pool shadow-only U-006 probe\nmodel_provider = "dualpool_probe"\n\n[model_providers.dualpool_probe]\nname = "Dual Pool U-006 shadow"\n' + `base_url = "http://127.0.0.1:${port}/v1"\nwire_api = "responses"\nenv_key = "DUALPOOL_CODEX_KEY"\n`;
  fs.writeFileSync(stage, text, { encoding: 'utf8', flag: 'wx' }); fs.renameSync(stage, file); if (parseTomlStructure(text).toml_parse !== 'PASS') throw new Error('SHADOW_CONFIG_TOML_INVALID');
  return { shadow_config_modified: true, real_config_mutation_capability: false };
}

function probeEnvironment(base, shadow, secret) { const env = { ...base }; for (const n of ['ELECTRON_RUN_AS_NODE', 'CODEX_HOME', 'DUALPOOL_CODEX_KEY', 'NODE_OPTIONS', 'VSCODE_IPC_HOOK_CLI']) delete env[n]; env.CODEX_HOME = shadow; env.DUALPOOL_CODEX_KEY = secret; return env; }
function launchProbe(executable, shadow, secret, launcher = spawn) { if (!executable || !fs.existsSync(executable)) throw new Error('ANTIGRAVITY_EXECUTABLE_NOT_FOUND'); const child = launcher(executable, ['--new-window'], { env: probeEnvironment(process.env, shadow, secret), detached: true, stdio: 'ignore', windowsHide: true }); child.unref?.(); return child; }
function renderTargetPrompt(prompt, output = console) { output.log(`Phase 0B shadow Astra synthetic transport check.\nReply briefly.\n${prompt}`); }
function classifyAuth(choice) { return choice === 'AUTH_OK' ? 'AUTH_OK' : 'FULL_CODEX_HOME_SHADOW_AUTH_NOT_RECOGNIZED'; }
function classifyAstra(choice) { return choice === 'ASTRA_SELECTED' ? 'ASTRA_SELECTED' : 'AUTHENTICATED_ASTRA_NOT_VISIBLE'; }
function isAllowedAnswer(choice, allowed) { return allowed.includes(choice); }
function writeSafe(file, value, secret = '', prompt = '') { const text = JSON.stringify(value, null, 2) + '\n'; if (text.includes(secret) || text.includes(prompt) || /\b[A-Z]:[\\/]/i.test(text)) throw new Error('UNSAFE_EVIDENCE'); fs.mkdirSync(path.dirname(file), { recursive: true }); const stage = file + '.stage'; fs.writeFileSync(stage, text, { encoding: 'utf8', flag: 'wx' }); fs.renameSync(stage, file); }
async function waitFor(predicate, timeout, code, interval = 250) { const deadline = Date.now() + timeout; while (Date.now() < deadline) { if (await predicate()) return; await sleep(interval); } throw new Error(code); }
function cleanupShadow(root) { if (!safeSessionRoot(root)) throw new Error('UNSAFE_SHADOW_ROOT'); assertNoReparse(root); if (fs.existsSync(root)) fs.rmSync(root, { recursive: true, force: false }); return !fs.existsSync(root); }
function stopRecorder(root) { try { fs.writeFileSync(path.join(root, 'stop'), ''); } catch {} }

async function main({ processCount = getAntigravityProcessCount, discoverExecutable = discoverAntigravityExecutable, launcher = spawn, acl = applySessionAcl } = {}) {
  const repo = path.resolve(__dirname, '..'); const realHome = path.join(os.homedir(), '.codex'); const realConfig = path.join(realHome, 'config.toml');
  const baseline = assessRealConfig(realConfig); if (!baseline.current_working_config_candidate) throw new Error('CURRENT_CONFIG_PROBE_CONTAMINATION'); const runBaselineHash = baseline.current_sha256;
  const sessionRoot = path.join(os.tmpdir(), PREFIX + crypto.randomBytes(16).toString('hex')); const shadowHome = path.join(sessionRoot, 'codex-home'); const stateRoot = path.join(sessionRoot, 'state'); const recorderRoot = path.join(sessionRoot, 'recorder');
  const secret = 'SECRET_SENTINEL_' + crypto.randomBytes(24).toString('hex'); const prompt = 'ASTRA_PROMPT_SENTINEL_' + crypto.randomBytes(24).toString('hex');
  const result = { schema_version: 3, test_id: 'P0B-CX-SHADOW-HOME-001', policy: POLICY, real_config_before_sha256: runBaselineHash, real_config_after_sha256: null, real_config_mutation_capability: false, crash_requires_restore: false, auth_confirmed: false, astra_selected: false, target_request_captured: false, shadow_retained_for_safe_cleanup: false, normal_process_observed: false, status: 'BLOCKED' };
  let recorder = null; let probeStarted = false; let probeClosed = false; let cleanupPass = false; let classification = 'RUNNING'; let heartbeatTimer = null; let currentStage = 'STARTING';
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  const answer = async (message, allowed) => {
    while (true) {
      console.log(message + '\n' + allowed.join(' / '));
      const choice = await new Promise(resolve => rl.once('line', line => resolve(line.trim().toUpperCase())));
      if (isAllowedAnswer(choice, allowed)) return choice;
      console.log('INVALID_CHECKPOINT_INPUT');
    }
  };
  const writeState = stage => { currentStage = stage; writeSafe(path.join(stateRoot, 'status.json'), { stage, policy: POLICY }, secret, prompt); };
  const heartbeat = stage => { try { writeSafe(path.join(stateRoot, 'heartbeat.json'), { stage, watchdog_ready: true, heartbeat_unix_ms: Date.now() }, secret, prompt); } catch {} };
  try {
    fs.mkdirSync(sessionRoot, { recursive: true }); acl(sessionRoot); fs.mkdirSync(stateRoot); fs.mkdirSync(recorderRoot); fs.mkdirSync(shadowHome); validateTopology(sessionRoot, shadowHome, stateRoot, recorderRoot); writeState('WATCHDOG_READY'); writeState('WAIT_PRIMARY_CLOSE'); heartbeatTimer = setInterval(() => heartbeat(currentStage), 1000); heartbeat(currentStage);
    result.initial_process_count = processCount(); result.executable_discovered = Boolean(discoverExecutable());
    console.log('[WATCHDOG_READY]'); console.log('[WAIT_PRIMARY_CLOSE]'); console.log('Shadow U-006 probe is ready.'); console.log('The REAL ~/.codex config will not be changed.'); console.log('Save your work and close all Antigravity windows normally. Keep this watchdog console open.');
    await waitFor(() => processCount() === 0, 20 * 60 * 1000, 'PRIMARY_CLOSE_TIMEOUT'); result.cold_source_quiescent = true; writeState('COLD_SOURCE_QUIESCENT');
    if (sha256(realConfig) !== runBaselineHash) throw new Error('REAL_CONFIG_CONCURRENT_DRIFT'); fs.rmSync(shadowHome, { recursive: true, force: true }); copyOpaqueCodexHome(realHome, shadowHome); writeState('COLD_COPY_COMPLETE');
    const recorderScript = path.join(repo, 'scripts', 'phase0b-primary-recorder.cjs'); recorder = spawn(process.execPath, [recorderScript], { windowsHide: true, stdio: 'ignore', env: { ...process.env, P0B_PRIMARY_ROOT: recorderRoot, P0B_PRIMARY_SECRET: secret, P0B_PRIMARY_PROMPT: prompt } });
    await waitFor(() => fs.existsSync(path.join(recorderRoot, 'ready.json')), 15000, 'RECORDER_NOT_READY'); const ready = JSON.parse(fs.readFileSync(path.join(recorderRoot, 'ready.json'), 'utf8')); writeShadowConfig(shadowHome, ready.port); writeState('SHADOW_CONFIG_ACTIVE'); if (sha256(realConfig) !== runBaselineHash) throw new Error('REAL_CONFIG_CONCURRENT_DRIFT');
    const probe = launchProbe(discoverExecutable(), shadowHome, secret, launcher); probeStarted = true; await waitFor(() => processCount() > 0, 30000, 'PROBE_IDE_NOT_ALIVE'); writeState('WAIT_AUTH_CONFIRMATION');
    const auth = await answer('[WAIT_AUTH_CONFIRMATION]\nMở Codex trong cửa sổ Antigravity vừa mở.', ['AUTH_OK', 'AUTH_LOST']); classification = classifyAuth(auth); if (classification !== 'AUTH_OK') throw new Error(classification); result.auth_confirmed = true;
    const astra = await answer('[WAIT_ASTRA_SELECTION]\nMở model picker, chọn GPT-6 Astra, chưa gửi prompt.', ['ASTRA_SELECTED', 'ASTRA_NOT_VISIBLE']); classification = classifyAstra(astra); if (classification !== 'ASTRA_SELECTED') throw new Error(classification); result.astra_selected = true;
    fs.writeFileSync(path.join(recorderRoot, 'arm'), ''); writeState('WAIT_RECORDER_ARM'); await waitFor(() => fs.existsSync(path.join(recorderRoot, 'armed.json')) && JSON.parse(fs.readFileSync(path.join(recorderRoot, 'armed.json'), 'utf8')).armed === true, 15000, 'RECORDER_ARM_TIMEOUT'); renderTargetPrompt(prompt); writeState('WAIT_TARGET_CAPTURE');
    await waitFor(() => fs.existsSync(path.join(recorderRoot, 'capture.json')), 10 * 60 * 1000, 'TARGET_REQUEST_TIMEOUT'); const capture = JSON.parse(fs.readFileSync(path.join(recorderRoot, 'capture.json'), 'utf8')); result.target_request_captured = true; result.target_model = capture.model; result.recorder_acceptance = { method: capture.method, route: capture.route, model_exact_astra: capture.model_exact_astra, auth_match_status: capture.auth_match_status, prompt_match_status: capture.prompt_match_status, json_parse_status: capture.json_parse_status, response_closed: capture.response_closed, emitted_events: capture.emitted_events };
    if (capture.method !== 'POST' || capture.route !== '/v1/responses' || capture.model !== 'gpt-6-astra' || capture.model_exact_astra !== true || capture.auth_match_status !== 'PASS' || capture.prompt_match_status !== 'PASS' || capture.json_parse_status !== 'PASS' || capture.response_closed !== true || !capture.emitted_events.includes('response.created') || !capture.emitted_events.includes('response.completed')) throw new Error('SHADOW_WIRE_ACCEPTANCE_FAILED'); classification = 'PASS';
  } catch (error) { classification = error.message || 'SHADOW_RUN_FAILED'; }
  finally {
    writeState('CLOSE_PROBE'); heartbeat(currentStage); console.log('[CLOSE_PROBE]'); console.log('Đóng tất cả cửa sổ Antigravity probe bình thường. Không mở Antigravity lại cho đến khi thấy SHADOW_CLEANUP_PASS.');
    try { probeClosed = !probeStarted ? processCount() === 0 : await waitFor(() => processCount() === 0, 20 * 60 * 1000, 'PROBE_CLOSE_TIMEOUT').then(() => true); } catch (error) { classification = error.message || 'PROBE_CLOSE_TIMEOUT'; probeClosed = false; result.shadow_retained_for_safe_cleanup = true; }
    if (heartbeatTimer) clearInterval(heartbeatTimer);
    if (probeClosed) { stopRecorder(recorderRoot); try { await waitFor(() => !recorder || recorder.exitCode !== null, 5000, 'RECORDER_STOP_TIMEOUT'); } catch { classification = classification === 'PASS' ? 'RECORDER_STOP_TIMEOUT' : classification; } try { cleanupPass = cleanupShadow(sessionRoot); } catch { cleanupPass = false; } }
    result.shadow_cleanup_pass = cleanupPass; result.probe_closed = probeClosed; result.real_config_after_sha256 = sha256(realConfig); result.run_baseline_unchanged = result.real_config_after_sha256 === runBaselineHash; if (!result.run_baseline_unchanged) classification = 'REAL_CONFIG_CONCURRENT_DRIFT';
    if (cleanupPass && result.run_baseline_unchanged && probeClosed) { if (sha256(realConfig) !== runBaselineHash) classification = 'REAL_CONFIG_CONCURRENT_DRIFT'; console.log('[NORMAL_REOPEN]'); console.log('Hãy tự mở Antigravity bằng shortcut/Menu Start bình thường. Không mở từ watchdog.'); try { await waitFor(() => processCount() > 0, 5 * 60 * 1000, 'NORMAL_IDE_REOPEN_TIMEOUT'); result.normal_process_observed = true; } catch (error) { classification = error.message || 'NORMAL_IDE_REOPEN_TIMEOUT'; } if (result.normal_process_observed) { const normal = await answer('Codex bình thường đã mở lại và tài khoản/chat hoạt động bình thường?', ['NORMAL_IDE_OK', 'NORMAL_IDE_FAILED']); result.normal_ide_confirmation = normal; if (normal !== 'NORMAL_IDE_OK') classification = 'NORMAL_IDE_OWNER_REPORTED_FAILED'; if (sha256(realConfig) !== runBaselineHash) classification = 'REAL_CONFIG_CONCURRENT_DRIFT'; result.normal_hash_unchanged = sha256(realConfig) === runBaselineHash; } }
    result.classification = classification; result.status = classification === 'PASS' && probeClosed && cleanupPass && result.run_baseline_unchanged && result.normal_process_observed && result.normal_ide_confirmation === 'NORMAL_IDE_OK' && result.normal_hash_unchanged ? 'PASS' : 'BLOCKED';
    const evidence = path.join(repo, 'evidence', 'phase-0b-u006-shadow-home'); writeSafe(path.join(evidence, 'shadow-result.json'), result, secret, prompt); if (cleanupPass) console.log('SHADOW_CLEANUP_PASS'); else if (result.shadow_retained_for_safe_cleanup) console.log('Shadow retained because Probe is still running. Close all Probe windows before cleanup.'); rl.close();
  }
  process.exitCode = result.status === 'PASS' ? 0 : 1; return result;
}

if (require.main === module) main().catch(error => { console.error(error.message); process.exitCode = 1; });
module.exports = { POLICY, sha256, parseTomlStructure, assessRealConfig, safeSessionRoot, safeChild, validateTopology, runPowerShell, getAntigravityProcessSnapshot, getAntigravityProcessCount, discoverAntigravityExecutable, applySessionAcl, copyOpaqueCodexHome, writeShadowConfig, probeEnvironment, launchProbe, renderTargetPrompt, classifyAuth, classifyAstra, isAllowedAnswer, writeSafe, waitFor, cleanupShadow, main };

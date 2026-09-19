'use strict';
const fs=require('node:fs'),path=require('node:path'),os=require('node:os');
const crypto=require('node:crypto'),readline=require('node:readline');
const {spawn,spawnSync}=require('node:child_process');
const {evaluate}=require('./phase0b-primary-evidence.cjs');

function ideEnvironment(source,key) {
  const env={...source};
  for(const name of Object.keys(env)) if(['ELECTRON_RUN_AS_NODE','CODEX_HOME','DUALPOOL_CODEX_KEY',
    'VSCODE_IPC_HOOK_CLI','NODE_OPTIONS'].includes(name.toUpperCase())) delete env[name];
  if(key) env.DUALPOOL_CODEX_KEY=key;
  return env;
}
const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
function secureSession(session) {
  const command="$sid=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value; & icacls.exe $env:P0B_SESSION /inheritance:r /grant:r ('*'+$sid+':(OI)(CI)F') '*S-1-5-18:(OI)(CI)F'; if($LASTEXITCODE){exit $LASTEXITCODE}";
  const acl=spawnSync('powershell.exe',['-NoProfile','-NonInteractive','-Command',command],
    {windowsHide:true,encoding:'utf8',env:{...process.env,P0B_SESSION:session}});
  if(acl.status!==0)throw new Error('SESSION_ACL_FAILED');
  const check=path.join(session,'.acl-check');fs.writeFileSync(check,'',{flag:'wx'});fs.unlinkSync(check);
}
async function waitFor(predicate,timeout,code,interval=1000) {
  const end=Date.now()+timeout;
  do {if(await predicate())return;await sleep(interval);} while(Date.now()<end);
  throw new Error(code);
}
async function runProbe(io) {
  let prepared=false,capture=null,error=null,restore=null,normal=false;
  let auth=false,astra=false;
  try {
    await io.ready(); // Agent handshake must precede any backup/config mutation.
    await io.closePrimary();
    await io.prepare(); prepared=true;
    await io.startRecorder();
    await io.activate();
    await io.launchProbe();
    auth=await io.confirmAuth();
    if(!auth)throw new Error('PRIMARY_AUTH_NOT_RECOGNIZED_AFTER_RELAUNCH');
    astra=await io.selectAstra();
    if(!astra)throw new Error('AUTHENTICATED_ASTRA_NOT_VISIBLE');
    await io.arm();
    capture=await io.capture();
  } catch(e) {error=/^[A-Z][A-Z0-9_]+$/.test(e.message)?e.message:'WATCHDOG_OPERATION_FAILED';}
  finally {
    // Restore is attempted even if Activate replaced config then threw.
    if(prepared) {
      try {await io.closeProbe();restore=await io.restore();}
      catch(e){error=/^[A-Z][A-Z0-9_]+$/.test(e.message)?e.message:'RESTORE_OPERATION_FAILED';}
    }
    try {await io.stopRecorder();} catch {error='RECORDER_CLEANUP_FAILED';}
    if(restore?.restore_verified) {
      try {await io.manualNormal();normal=true;} catch {error=error||'NORMAL_RELAUNCH_FAILED';}
    }
  }
  return {capture,error,restore,normal,auth,astra,prepared};
}

async function main(session) {
  const repo=path.resolve(__dirname,'..');
  const safety=path.join(repo,'evidence','phase-0b-u006-primary-repair');
  const gate=JSON.parse(fs.readFileSync(path.join(safety,'safety-gate.json'),'utf8'));
  const manifest=JSON.parse(fs.readFileSync(path.join(safety,'safety-manifest.json'),'utf8'));
  if(gate.status!=='PASS'||gate.scope!=='FIXTURE_SAFETY_ONLY')throw new Error('SAFETY_GATE_REQUIRED');
  for(const item of manifest.artifacts){
    const file=path.resolve(repo,item.path);
    if(!file.startsWith(repo+path.sep)||crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')!==item.sha256)
      throw new Error('SAFETY_ARTIFACT_CHANGED');
  }
  const temp=path.resolve(os.tmpdir());
  session=path.resolve(session||'');
  if(path.dirname(session).toLowerCase()!==temp.toLowerCase() ||
    !/^dual-pool-u006-primary-[a-f0-9]{32}$/.test(path.basename(session)))throw new Error('UNSAFE_SESSION_ROOT');
  if(fs.existsSync(session))throw new Error('SESSION_ALREADY_EXISTS');
  fs.mkdirSync(session);
  // Restrict local-only backup access before any backup can exist.
  secureSession(session);
  const run=path.basename(session).replace('dual-pool-u006-primary-','');
  const runId=new Date().toISOString().replace(/[-:.]/g,'');
  const config=path.join(os.homedir(),'.codex','config.toml');
  const settings=path.join(process.env.APPDATA,'Antigravity IDE','User','settings.json');
  const exe=path.join(process.env.LOCALAPPDATA,'Programs','Antigravity IDE','Antigravity IDE.exe');
  const extRoot=path.join(os.homedir(),'.antigravity-ide','extensions');
  const exts=fs.readdirSync(extRoot).filter(n=>n.startsWith('openai.chatgpt-'));
  if(exts.length!==1||!fs.existsSync(exe))throw new Error('PRIMARY_DISCOVERY_FAILED');
  const extension=path.join(extRoot,exts[0],'package.json');
  const hash=p=>fs.existsSync(p)?crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex').toUpperCase():'ABSENT';
  const secret='SECRET_SENTINEL_'+crypto.randomBytes(24).toString('hex');
  const prompt='ASTRA_PROMPT_SENTINEL_'+crypto.randomBytes(24).toString('hex');
  let state='PRECHECK',recorder=null,port=null,baseline=null;
  const rl=readline.createInterface({input:process.stdin,output:process.stdout});
  const consoleLines=[];rl.on('line',line=>consoleLines.push(line.trim().toUpperCase()));
  function write(file,value) {
    const text=JSON.stringify(value,null,2)+'\n';
    if(text.includes(secret)||text.includes(prompt)||/\b[A-Z]:\\/i.test(text))throw new Error('UNSAFE_EVIDENCE');
    const stage=file+'.tmp';fs.writeFileSync(stage,text);fs.renameSync(stage,file);
  }
  function status(next) {
    state=next;console.log('\n['+state+']');
    write(path.join(session,'status.json'),{stage:state,run_id:runId,pid:process.pid});
  }
  const heartbeat=setInterval(()=>write(path.join(session,'heartbeat.json'),{
    status:'WATCHDOG_READY',stage:state,pid:process.pid,at:new Date().toISOString()}),1000);
  async function choice(message,allowed,timeout=20*60*1000) {
    consoleLines.length=0;console.log(message+'\n'+allowed.join(' / '));
    let answer;
    await waitFor(()=>{while(consoleLines.length){const line=consoleLines.shift();if(allowed.includes(line)){answer=line;return true;}}return false;},
      timeout,'USER_CHECKPOINT_TIMEOUT',100);
    return answer;
  }
  function powershell(command,extra={}) {
    const r=spawnSync('powershell.exe',['-NoProfile','-NonInteractive','-Command',command],{
      windowsHide:true,encoding:'utf8',timeout:15000,env:{...process.env,...extra}});
    if(r.status!==0||r.error)throw new Error('PROCESS_OBSERVATION_FAILED');
    return r.stdout.trim();
  }
  function ideCount() {
    return Number(powershell("@(Get-Process -Name 'Antigravity IDE' -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $env:P0B_IDE_EXE }).Count",{P0B_IDE_EXE:exe}));
  }
  function transaction(action) {
    const args=['-NoProfile','-NonInteractive','-File',path.join(__dirname,'phase0b-primary-transaction.ps1'),
      '-Action',action,'-SessionRoot',session];
    if(port)args.push('-Port',String(port));
    const r=spawnSync('powershell.exe',args,{windowsHide:true,encoding:'utf8',timeout:30000});
    let value;try{value=JSON.parse(r.stdout);}catch{throw new Error('TRANSACTION_RESULT_INVALID');}
    if(r.status!==0)throw new Error(typeof value==='string'?value:'TRANSACTION_FAILED');
    return value;
  }
  async function gone() {await waitFor(()=>ideCount()===0,20*60*1000,'PRIMARY_CLOSE_TIMEOUT',2000);}
  const recRoot=path.join(session,'recorder');fs.mkdirSync(recRoot);
  const io={
    async ready(){
      if(!process.stdin.isTTY)throw new Error('INTERACTIVE_CONSOLE_REQUIRED');
      if(!ideCount())throw new Error('ORIGINAL_IDE_NOT_RUNNING');
      status('WATCHDOG_READY');console.log('Chờ agent xác nhận watchdog độc lập. Chưa thay đổi cấu hình.');
      await waitFor(()=>fs.existsSync(path.join(session,'agent-approved')),5*60*1000,'WATCHDOG_NOT_DETACHED');
    },
    async closePrimary(){
      status('WAIT_PRIMARY_CLOSE');
      console.log('Lưu công việc, đóng tất cả cửa sổ Antigravity. Giữ cửa sổ watchdog này mở.');await gone();
    },
    async prepare(){
      baseline={config:hash(config),settings:hash(settings),extension:hash(extension)};
      status('BACKUP_COMPLETE');transaction('Prepare');
    },
    async startRecorder(){
      recorder=spawn(process.execPath,[path.join(__dirname,'phase0b-primary-recorder.cjs')],{
        windowsHide:true,stdio:'ignore',env:{...process.env,P0B_PRIMARY_ROOT:recRoot,P0B_PRIMARY_SECRET:secret,P0B_PRIMARY_PROMPT:prompt}});
      recorder.on('error',()=>{});
      await waitFor(()=>fs.existsSync(path.join(recRoot,'ready.json')),15000,'RECORDER_NOT_READY',100);
      const ready=JSON.parse(fs.readFileSync(path.join(recRoot,'ready.json'),'utf8'));port=ready.port;
      if(ready.pid!==recorder.pid||ready.address!=='127.0.0.1'||!Number.isInteger(port))throw new Error('RECORDER_OWNERSHIP_FAILED');
      const verified=powershell("$l=@(Get-NetTCPConnection -State Listen -LocalPort ([int]$env:P0B_PORT) -ErrorAction Stop); [bool]($l.Count -eq 1 -and $l[0].LocalAddress -eq '127.0.0.1' -and $l[0].OwningProcess -eq [int]$env:P0B_PID)",
        {P0B_PORT:String(port),P0B_PID:String(recorder.pid)});
      if(verified!=='True')throw new Error('RECORDER_OWNERSHIP_FAILED');status('RECORDER_READY');
    },
    async activate(){transaction('Activate');status('PROBE_CONFIG_ACTIVE');},
    async launchProbe(){
      const child=spawn(exe,['--new-window',repo],{cwd:path.dirname(exe),detached:true,stdio:'ignore',env:ideEnvironment(process.env,secret)});
      let launchFailed=false;child.on('error',()=>{launchFailed=true;});child.unref();
      await waitFor(()=>!launchFailed&&ideCount()>0,90000,'PROBE_RELAUNCH_FAILED',1000);
      status('PRIMARY_RELAUNCHED_FOR_PROBE');
    },
    async confirmAuth(){status('WAIT_AUTH_CONFIRMATION');return await choice(
      'Mở Codex. Kiểm tra tài khoản thường dùng có được nhận diện không. Không đăng nhập lại hoặc sao chép auth.',
      ['AUTH_OK','AUTH_LOST'])==='AUTH_OK';},
    async selectAstra(){status('WAIT_ASTRA_SELECTION');return await choice(
      'Chọn GPT-6 Astra trong picker, chưa gửi tin nhắn. Xác nhận tại cửa sổ này.',
      ['ASTRA_SELECTED','ASTRA_NOT_VISIBLE'])==='ASTRA_SELECTED';},
    async arm(){fs.writeFileSync(path.join(recRoot,'arm'),'');
      await waitFor(()=>fs.existsSync(path.join(recRoot,'armed.json')),5000,'RECORDER_ARM_TIMEOUT',100);status('WAIT_ASTRA_SENTINEL');
      console.log('Gửi đúng một prompt sau trong chat Codex mới, rồi quay lại cửa sổ này.\n\nPhase 0B synthetic transport check. Reply briefly.\n'+prompt+'\n');},
    async capture(){
      await waitFor(()=>fs.existsSync(path.join(recRoot,'capture.json')),10*60*1000,'TARGET_REQUEST_TIMEOUT',250);
      const capture=JSON.parse(fs.readFileSync(path.join(recRoot,'capture.json'),'utf8'));
      status('ASTRA_REQUEST_CAPTURED');return capture;
    },
    async closeProbe(){status('WAIT_PROBE_CLOSE');console.log('Đóng các cửa sổ Antigravity để khôi phục cấu hình gốc.');await gone();},
    async restore(){status('RESTORE_CONFIG');return transaction('Restore');},
    async stopRecorder(){if(recorder && recorder.exitCode===null){fs.writeFileSync(path.join(recRoot,'stop'),'');
      await waitFor(()=>recorder.exitCode!==null,10000,'RECORDER_STOP_TIMEOUT',100);}},
    async manualNormal(){status('WAIT_MANUAL_NORMAL_REOPEN');console.log('Khôi phục cấu hình đã PASS. Bây giờ bạn tự mở Antigravity bằng shortcut thường dùng.');
      await waitFor(()=>ideCount()>0,20*60*1000,'NORMAL_RELAUNCH_FAILED',2000);}
  };
  let outcome;
  try {outcome=await runProbe(io);} finally {clearInterval(heartbeat);rl.close();}
  const capture=outcome.capture,restore=outcome.restore;
  const controls={settings_before_sha256:baseline?.settings,settings_after_sha256:hash(settings),
    extension_before_sha256:baseline?.extension,extension_after_sha256:hash(extension),
    normal_relaunch:outcome.normal,normal_relaunch_mode:'USER_MANUAL',
    normal_secret_environment:'UNKNOWN', // External process environment is not inspected.
    listener_absent:port===null||powershell("@(Get-NetTCPConnection -State Listen -LocalPort ([int]$env:P0B_PORT) -ErrorAction SilentlyContinue).Count",{P0B_PORT:String(port)})==='0'};
  controls.settings_unchanged=controls.settings_before_sha256===controls.settings_after_sha256;
  controls.extension_unchanged=controls.extension_before_sha256===controls.extension_after_sha256;
  const result={schema_version:2,test_id:'P0B-CX-PRIMARY-MODEL-002',run_id:runId,primary_profile_used:true,
    authentication_confirmed:outcome.auth,astra_visible:outcome.astra,astra_selected:outcome.astra,
    target_request_captured:!!capture,target_model:capture?.model,wire_method:capture?.method,wire_route:capture?.route,
    custom_provider_retained:!!capture,synthetic_auth:capture?.auth_match_status,prompt_sentinel:capture?.prompt_match_status,
    json_parse_status:capture?.json_parse_status,response_lifecycle:capture?.response_closed===true,emitted_events:capture?.emitted_events,
    config:{original_state:baseline?.config==='ABSENT'?'ABSENT':'PRESENT',config_before_sha256:baseline?.config,
      config_after_sha256:hash(config),byte_for_byte_restore:restore?.restore_verified===true},controls,
    cleanup:{temp_root_removed:false,auth_material_inspected:false,auth_material_committed:false},
    classification:outcome.error||(!capture?.model_exact_astra?'WIRE_MODEL_NOT_GPT_6_ASTRA':'PASS')};
  const evidence=path.join(repo,'evidence','phase-0b-u006-primary-repair');fs.mkdirSync(evidence,{recursive:true});
  const resultPath=path.join(evidence,`result-${runId}.json`);
  // First durable evidence before cleanup; failed cleanup retains recoverable state.
  result.status='BLOCKED';write(resultPath,result);
  if(restore?.restore_verified && outcome.normal && controls.listener_absent){
    const sibling=config+'.dualpool-backup-'+path.basename(session);
    if(fs.existsSync(sibling))fs.unlinkSync(sibling);
    if(fs.realpathSync(session).toLowerCase()!==session.toLowerCase())throw new Error('UNSAFE_SESSION_CLEANUP');
    fs.rmSync(session,{recursive:true});result.cleanup.temp_root_removed=!fs.existsSync(session);
  }
  const assessment=evaluate(result);result.status=assessment.status;result.failed_assertions=assessment.failures;
  if(outcome.error)result.status='BLOCKED';write(resultPath,result);
  console.log('Kết quả: '+result.status+' / '+result.classification+'. Bạn có thể quay lại chat để agent kiểm tra.');
  process.exitCode=result.status==='PASS'?0:1;
}
if(require.main===module)main(process.argv[2]).catch(e=>{
  console.error(/^[A-Z][A-Z0-9_]+$/.test(e.message)?e.message:'WATCHDOG_FATAL_BACKUP_RETAINED');process.exitCode=1;
});
module.exports={ideEnvironment,waitFor,runProbe,secureSession};

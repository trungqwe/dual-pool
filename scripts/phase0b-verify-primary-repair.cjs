'use strict';
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const {spawnSync}=require('node:child_process');
const root=path.resolve(__dirname,'..');
const out=path.join(root,'evidence/phase-0b-u006-primary-repair');
const files=[
  'scripts/phase0b-primary-evidence.cjs','scripts/phase0b-primary-evidence.test.cjs',
  'scripts/phase0b-primary-recorder.cjs','scripts/phase0b-primary-recorder.test.cjs',
  'scripts/phase0b-primary-watchdog.cjs','scripts/phase0b-primary-watchdog.test.cjs',
  'scripts/phase0b-primary-transaction.ps1','scripts/phase0b-primary-transaction.test.ps1',
  'scripts/phase0b-u006-primary-watchdog.ps1','scripts/phase0b-u006-primary-restore.ps1',
  'scripts/phase0b-verify-primary-profile.ps1','scripts/phase0b-verify-primary-repair.cjs',
  'docs/reports/2026-09-19T0814Z-phase-0b-primary-audit.md',
  'docs/reports/2026-09-19T0840Z-phase-0b-primary-repair.md',
  'docs/18-HANDOFF.md','docs/15-MASTER-CHECKLIST.md'
];
const patterns=[/SECRET_SENTINEL_[a-f0-9]{16,}/i,/ASTRA_PROMPT_SENTINEL_[a-f0-9]{16,}/i,
  /Bearer\s+[a-z0-9._~-]{12,}/i,/-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----/,
  /\b[A-Z]:[\\/]/i,/[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}/i];
function violations(text){return patterns.filter(p=>p.test(text)).length;}
function run(exe,args){
  const child=spawnSync(exe,args,{cwd:root,windowsHide:true,encoding:'utf8',timeout:60000});
  if(child.error||child.status!==0)throw new Error('REQUIRED_CHECK_FAILED:'+path.basename(args.at(-1)||exe));
  return child.stdout;
}
function digest(bytes){return crypto.createHash('sha256').update(bytes).digest('hex');}
function main(){
  fs.mkdirSync(out,{recursive:true});
  const nodeOutput=run(process.execPath,['--test','scripts/phase0b-primary-evidence.test.cjs',
    'scripts/phase0b-primary-recorder.test.cjs','scripts/phase0b-primary-watchdog.test.cjs']);
  const testCount=Number(nodeOutput.match(/# tests (\d+)/)?.[1]);
  if(!testCount)throw new Error('TEST_COUNT_MISSING');
  const transaction=JSON.parse(run('powershell.exe',['-NoProfile','-NonInteractive','-File','scripts/phase0b-primary-transaction.test.ps1']));
  for(const f of files.filter(f=>f.endsWith('.ps1'))){
    run('powershell.exe',['-NoProfile','-NonInteractive','-Command',
      `$e=$null;$t=$null;[System.Management.Automation.Language.Parser]::ParseFile('${f}',[ref]$t,[ref]$e)|Out-Null;if($e.Count){exit 1}`]);
  }
  for(const f of files.filter(f=>f.endsWith('.cjs')))run(process.execPath,['--check',f]);
  run('git',['diff','--check']);
  // Positive controls exercise each privacy class without printing the fixture.
  const controls=['SECRET_SENTINEL_'+'a'.repeat(32),'ASTRA_PROMPT_SENTINEL_'+'b'.repeat(32),
    'Bearer '+'x'.repeat(24),'-----BEGIN '+'PRIVATE KEY-----','X'+':'+String.fromCharCode(92)+'fixture',
    'fixture'+'@'+'example.invalid'];
  if(controls.some(c=>!violations(c)))throw new Error('SCANNER_POSITIVE_CONTROL_FAILED');
  let linkCount=0;
  for(const f of files){
    const text=fs.readFileSync(path.join(root,f),'utf8');
    if(violations(text))throw new Error('PRIVACY_SCAN_FAILED:'+f);
    if(f.endsWith('.md'))for(const m of text.matchAll(/\]\(([^)]+)\)/g)){
      const target=m[1].split('#')[0];if(!target||/^[a-z]+:/i.test(target))continue;
      linkCount++;if(!fs.existsSync(path.resolve(root,path.dirname(f),target)))throw new Error('DOC_LINK_FAILED:'+f);
    }
  }
  const gate={schema_version:2,test_id:'P0B-PRIMARY-SAFETY-002',status:'PASS',scope:'FIXTURE_SAFETY_ONLY',
    u006:'BLOCKED_PENDING_LIVE_PROBE',node_tests:testCount,transaction_assertions:transaction.assertions,
    syntax:'PASS',privacy:'PASS',privacy_positive_controls:controls.length,match_count:0,
    document_links_checked:linkCount,diff_check:'PASS',paths_actually_scanned:files,real_config_touched:false};
  const artifacts=files.map(f=>{const bytes=fs.readFileSync(path.join(root,f));return {
    path:f,sha256:digest(bytes),size:bytes.length,git_lf_sha256:digest(Buffer.from(bytes.toString('utf8').replace(/\r\n/g,'\n')))};});
  if(process.argv.includes('--generate')){
    const gateBytes=Buffer.from(JSON.stringify(gate,null,2)+'\n');
    fs.writeFileSync(path.join(out,'safety-gate.json'),gateBytes);
    artifacts.push({path:'evidence/phase-0b-u006-primary-repair/safety-gate.json',sha256:digest(gateBytes),size:gateBytes.length});
    fs.writeFileSync(path.join(out,'safety-manifest.json'),JSON.stringify({schema_version:2,artifacts},null,2)+'\n');
    for(const a of artifacts)if(digest(fs.readFileSync(path.join(root,a.path)))!==a.sha256)throw new Error('MANIFEST_VERIFY_FAILED');
  }
  console.log(JSON.stringify(gate,null,2));
}
try{main();}catch(e){console.error(e.message);process.exitCode=1;}

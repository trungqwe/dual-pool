'use strict';
const test=require('node:test'),assert=require('node:assert/strict');
const {runProbe,ideEnvironment,waitFor,secureSession}=require('./phase0b-primary-watchdog.cjs');
test('actual private session ACL permits owner file operations',()=>{
  const fs=require('node:fs'),os=require('node:os'),path=require('node:path');
  const root=fs.mkdtempSync(path.join(os.tmpdir(),'dual-pool-acl-test-'));
  try{secureSession(root);const file=path.join(root,'check');fs.writeFileSync(file,'fixture');
    assert.equal(fs.readFileSync(file,'utf8'),'fixture');fs.unlinkSync(file);
  }finally{fs.rmdirSync(root);}
});
function fixture(fault) {
  const calls=[];const io={};
  for(const name of ['ready','closePrimary','prepare','startRecorder','activate','launchProbe','confirmAuth',
    'selectAstra','arm','capture','closeProbe','restore','stopRecorder','manualNormal']) {
    io[name]=async()=>{calls.push(name);if(name===fault)throw new Error('INJECTED_FAILURE');
      if(name==='confirmAuth'||name==='selectAstra')return true;
      if(name==='capture')return {model_exact_astra:true};
      if(name==='restore')return {restore_verified:true};};
  }
  return {io,calls};
}
test('successful sequence restores before manual reopening',async()=>{
  const {io,calls}=fixture();const r=await runProbe(io);
  assert.equal(r.error,null);assert.equal(r.normal,true);
  assert.ok(calls.indexOf('ready')<calls.indexOf('prepare'));
  assert.ok(calls.indexOf('selectAstra')<calls.indexOf('arm'));
  assert.ok(calls.indexOf('restore')<calls.indexOf('manualNormal'));
});
for(const fault of ['startRecorder','activate','launchProbe','confirmAuth','selectAstra','arm','capture']) {
  test(`failure at ${fault} always attempts restore`,async()=>{
    const {io,calls}=fixture(fault);const r=await runProbe(io);
    assert.equal(r.error,'INJECTED_FAILURE');assert.ok(calls.includes('restore'));
  });
}
test('failed independence handshake cannot prepare config',async()=>{
  const {io,calls}=fixture('ready');await runProbe(io);assert.equal(calls.includes('prepare'),false);
});
test('restore failure retains recovery and prevents normal reopening',async()=>{
  const {io,calls}=fixture('restore');const r=await runProbe(io);
  assert.equal(r.normal,false);assert.equal(calls.includes('manualNormal'),false);
});
test('voluntary close timeout never overwrites running user config',async()=>{
  const {io,calls}=fixture('closeProbe');await runProbe(io);assert.equal(calls.includes('restore'),false);
});
test('probe environment removes Electron Node mode and user overrides',()=>{
  const original={ELECTRON_RUN_AS_NODE:'1',CODEX_HOME:'fixture',DUALPOOL_CODEX_KEY:'old',NODE_OPTIONS:'old',PATH:'keep'};
  const probe=ideEnvironment(original,'synthetic');
  assert.equal(probe.ELECTRON_RUN_AS_NODE,undefined);assert.equal(probe.CODEX_HOME,undefined);
  assert.equal(probe.NODE_OPTIONS,undefined);assert.equal(probe.DUALPOOL_CODEX_KEY,'synthetic');
  assert.equal(ideEnvironment(probe).DUALPOOL_CODEX_KEY,undefined);assert.equal(original.ELECTRON_RUN_AS_NODE,'1');
});
test('actual child receives only the intended synthetic environment',()=>{
  const {spawnSync}=require('node:child_process');
  const env=ideEnvironment({...process.env,ELECTRON_RUN_AS_NODE:'1',CODEX_HOME:'fixture'},'fixture-key');
  const child=spawnSync(process.execPath,['-e',
    "process.exit(process.env.DUALPOOL_CODEX_KEY==='fixture-key'&&!process.env.ELECTRON_RUN_AS_NODE&&!process.env.CODEX_HOME?0:1)"],
    {env,windowsHide:true});
  assert.equal(child.status,0);
});
test('wait is bounded and classifies timeout',async()=>{
  await assert.rejects(waitFor(()=>false,5,'EXPECTED_TIMEOUT',1),/EXPECTED_TIMEOUT/);
});

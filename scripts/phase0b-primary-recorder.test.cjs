'use strict';
const test=require('node:test'), assert=require('node:assert/strict');
const fs=require('node:fs'),os=require('node:os'),path=require('node:path'),zlib=require('node:zlib');
const {createRecorder}=require('./phase0b-primary-recorder.cjs');
test('loopback contract, arming, background, gzip, SSE and privacy',async()=>{
  const root=fs.mkdtempSync(path.join(os.tmpdir(),'dual-pool-recorder-test-'));
  const secret='test-secret-'+Date.now(),prompt='test-prompt-'+Date.now();
  const recorder=createRecorder({root,secret,prompt});
  await new Promise(resolve=>recorder.server.listen(0,'127.0.0.1',resolve));
  const base=`http://127.0.0.1:${recorder.server.address().port}`;
  async function request(input,extra={}) {
    const r=await fetch(base+'/v1/responses',{method:'POST',headers:{authorization:`Bearer ${secret}`,...extra.headers},
      body:extra.body||JSON.stringify({model:'gpt-6-astra',input})});
    return {status:r.status,text:await r.text()};
  }
  try {
    assert.equal(recorder.server.address().address,'127.0.0.1');
    assert.equal((await fetch(base+'/v1/models')).status,404);
    assert.equal((await request(prompt,{headers:{authorization:'bad'}})).status,401);
    assert.equal((await request(prompt)).status,409);
    assert.equal((await request('background')).status,200);
    assert.equal(fs.existsSync(path.join(root,'capture.json')),false);
    assert.equal((await request('',{body:'{malformed'})).status,400);
    assert.equal((await request('',{body:Buffer.from([0xc3,0x28])})).status,400);
    assert.equal((await request('',{headers:{'content-encoding':'unknown'}})).status,415);
    const bomb=zlib.gzipSync(Buffer.alloc(8*1024*1024+1,65));
    assert.equal((await request('',{body:bomb,headers:{'content-encoding':'gzip'}})).status,400);
    recorder.arm();
    const raw=JSON.stringify({model:'gpt-6-astra',input:[{role:'user',content:[{type:'input_text',text:prompt}]}]});
    const response=await request('',{body:zlib.gzipSync(raw),headers:{'content-encoding':'gzip'}});
    assert.equal(response.status,200);
    const ev=response.text.split('\n').filter(l=>l.startsWith('data: ')).map(l=>JSON.parse(l.slice(6)));
    assert.equal(ev[0].type,'response.created');assert.equal(ev.at(-1).type,'response.completed');
    assert.equal(ev.at(-1).response.status,'completed');
    assert.equal(ev.at(-1).response.output[0].content[0].text,ev.find(e=>e.type==='response.output_text.delta').delta);
    assert.deepEqual(ev.map(e=>e.sequence_number),ev.map((_,i)=>i));
    const persisted=fs.readFileSync(path.join(root,'capture.json'),'utf8');
    assert.equal(persisted.includes(secret)||persisted.includes(prompt),false);
    const record=JSON.parse(persisted);
    assert.equal(record.model_exact_astra,true);assert.equal(record.response_closed,true);
    assert.equal(record.background_request_count,1);
    assert.equal((await request(prompt)).status,409);
  } finally { recorder.server.closeAllConnections();await new Promise(resolve=>recorder.server.close(resolve));fs.rmSync(root,{recursive:true}); }
});

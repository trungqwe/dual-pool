'use strict';
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const zlib = require('node:zlib');
const { randomUUID } = require('node:crypto');
const limit = 8 * 1024 * 1024;

function events(model = 'gpt-6-astra') {
  const id = `resp_${randomUUID().replaceAll('-', '')}`;
  const itemId = `msg_${randomUUID().replaceAll('-', '')}`;
  const text = 'Dual Pool synthetic transport check complete.';
  const part = { type: 'output_text', text, annotations: [], logprobs: [] };
  const item = { id: itemId, type: 'message', role: 'assistant', status: 'completed', content: [part] };
  const response = { id, object: 'response', created_at: Math.floor(Date.now()/1000), status: 'in_progress',
    error: null, incomplete_details: null, output: [], metadata: {}, parallel_tool_calls: false,
    tool_choice: 'auto', tools: [], temperature: 1, top_p: 1, usage: null,
    model, completed_at:null, instructions:null, max_output_tokens:null, previous_response_id:null,
    reasoning:{effort:null,summary:null},store:false,text:{format:{type:'text'}},truncation:'disabled',user:null };
  return [
    { type: 'response.created', response },
    { type: 'response.in_progress', response },
    { type: 'response.output_item.added', output_index: 0, item: {...item, status:'in_progress', content:[]} },
    { type: 'response.content_part.added', item_id:itemId, output_index:0, content_index:0, part:{...part,text:''} },
    { type: 'response.output_text.delta', item_id:itemId, output_index:0, content_index:0, delta:text, logprobs:[] },
    { type: 'response.output_text.done', item_id:itemId, output_index:0, content_index:0, text, logprobs:[] },
    { type: 'response.content_part.done', item_id:itemId, output_index:0, content_index:0, part },
    { type: 'response.output_item.done', output_index:0, item },
    { type: 'response.completed', response:{...response,status:'completed',completed_at:Math.floor(Date.now()/1000),output:[item],
      usage:{input_tokens:0,output_tokens:0,total_tokens:0,input_tokens_details:{cached_tokens:0},output_tokens_details:{reasoning_tokens:0}}} }
  ].map((event, sequence_number) => ({ ...event, sequence_number }));
}

function createRecorder({ root, secret, prompt }) {
  if (!root || !secret || !prompt) throw new Error('RECORDER_INPUT_REQUIRED');
  let background = 0, captured = false, armed = false;
  function save(name, record) {
    const text = JSON.stringify(record, null, 2) + '\n';
    if (text.includes(secret) || text.includes(prompt)) throw new Error('SENTINEL_LEAK');
    const stage = path.join(root, name + '.tmp');
    fs.writeFileSync(stage, text, {encoding:'utf8',flag:'wx'});
    fs.renameSync(stage, path.join(root, name));
  }
  const server = http.createServer((req, res) => {
    // No model catalog injection or proxy forwarding.
    if (req.method !== 'POST' || req.url !== '/v1/responses') { res.writeHead(404).end(); return; }
    if (req.headers.authorization !== `Bearer ${secret}`) { res.writeHead(401).end(); return; }
    let total=0; const chunks=[];
    req.on('error', () => {});
    req.on('data', chunk => {
      total += chunk.length;
      if (total > limit) { res.writeHead(413).end(); req.destroy(); } else chunks.push(chunk);
    });
    req.on('end', () => {
      if (res.writableEnded) return;
      let body;
      try {
        let raw = Buffer.concat(chunks);
        const encoding = req.headers['content-encoding'] || 'identity';
        const decoder = {gzip:zlib.gunzipSync,deflate:zlib.inflateSync,br:zlib.brotliDecompressSync}[encoding];
        if (decoder) raw = decoder(raw,{maxOutputLength:limit});
        else if (encoding !== 'identity') { res.writeHead(415).end(); return; }
        body = JSON.parse(new TextDecoder('utf-8',{fatal:true}).decode(raw));
        if (!body || Array.isArray(body) || typeof body !== 'object') throw new Error();
      } catch { res.writeHead(400).end(); return; }
      // Sentinel must be in input, not an arbitrary request metadata field.
      const target = JSON.stringify(body.input || '').includes(prompt);
      if (target && (!armed || captured)) { res.writeHead(409).end(); return; }
      if (!target) background++;
      const emitted = events(typeof body.model === 'string' ? body.model : 'gpt-6-astra');
      if (target) captured = true;
      res.on('finish', () => {
        if (!target) return;
        save('capture.json', {schema_version:2,method:'POST',route:'/v1/responses',
          model:body.model === 'gpt-6-astra' ? 'gpt-6-astra' : 'OTHER',
          model_exact_astra:body.model === 'gpt-6-astra',auth_match_status:'PASS',prompt_match_status:'PASS',
          json_parse_status:'PASS',response_closed:res.writableFinished,
          emitted_events:emitted.map(e=>e.type),background_request_count:background,armed:true});
      });
      res.writeHead(200, {'Content-Type':'text/event-stream; charset=utf-8','Connection':'close'});
      res.end(emitted.map(e=>`event: ${e.type}\ndata: ${JSON.stringify(e)}\n\n`).join(''));
    });
  });
  server.requestTimeout=15000;
  server.headersTimeout=10000;
  return {server,arm(){armed=true;},save};
}

if (require.main === module) {
  const root=process.env.P0B_PRIMARY_ROOT;
  const recorder=createRecorder({root,secret:process.env.P0B_PRIMARY_SECRET,prompt:process.env.P0B_PRIMARY_PROMPT});
  let acknowledged=false;
  const timer=setInterval(()=>{
    if(!acknowledged && fs.existsSync(path.join(root,'arm'))) {
      recorder.arm();recorder.save('armed.json',{armed:true});acknowledged=true;
    }
    if(fs.existsSync(path.join(root,'stop'))) {clearInterval(timer);recorder.server.close();recorder.server.closeAllConnections();}
  },100);
  recorder.server.on('error',()=>{clearInterval(timer);process.exitCode=1;});
  recorder.server.listen(0,'127.0.0.1',()=>recorder.save('ready.json',{
    address:'127.0.0.1',port:recorder.server.address().port,pid:process.pid}));
}
module.exports={createRecorder,events};

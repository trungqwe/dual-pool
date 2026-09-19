'use strict';
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const root = process.env.P0B_PRIMARY_ROOT;
const secret = process.env.P0B_PRIMARY_SECRET || '';
const prompt = process.env.P0B_PRIMARY_PROMPT || '';
const modelPattern = /^(?:openai\.|global\.openai\.|us\.openai\.)?gpt-[a-z0-9][a-z0-9.-]{0,63}$/;
if (!root) throw new Error('P0B_PRIMARY_ROOT_REQUIRED');
fs.mkdirSync(root, { recursive: true });
function save(name, value) {
  const text = JSON.stringify(value, null, 2) + '\n';
  if (text.includes(secret) || text.includes(prompt) || /\b[A-Z]:\\/i.test(text)) throw new Error('UNSAFE_EVIDENCE');
  fs.writeFileSync(path.join(root, name), text, 'utf8');
}
function parse(text) { try { return { status: 'PASS', value: JSON.parse(text.replace(/^\uFEFF/, '')) }; } catch { return { status: 'FAIL' }; } }
function sse() { return Buffer.from(['response.created', 'response.output_text.delta', 'response.completed'].map(type => `data: ${JSON.stringify({ type, response: { id: 'resp_phase0b_u006' } })}`).join('\n\n') + '\n\n', 'utf8'); }
let background = 0; let captured = false;
save('ready.json', { address: '127.0.0.1', background_requests_supported: true, target_by_prompt_sentinel: true });
const server = http.createServer((req, res) => {
  const route = (req.url || '').split('?')[0];
  if (req.method === 'GET' && route === '/v1/models') {
    res.writeHead(200, { 'Content-Type': 'application/json', Connection: 'close' });
    res.end(JSON.stringify({ object: 'list', data: [{ id: 'gpt-5.6-sol', object: 'model' }, { id: 'gpt-6-astra', object: 'model' }] }));
    return;
  }
  if (req.method !== 'POST' || route !== '/v1/responses' || captured) { res.writeHead(404, { Connection: 'close' }); res.end(); return; }
  const chunks = []; req.on('data', chunk => chunks.push(chunk)); req.on('end', () => {
    const body = Buffer.concat(chunks).toString('utf8'); const target = body.includes(prompt);
    const auth = req.headers.authorization || '';
    if (!target) background += 1;
    res.writeHead(200, { 'Content-Type': 'text/event-stream', Connection: 'close' }); res.end(sse());
    if (!target) return;
    captured = true;
    res.on('finish', () => {
      const parsed = parse(body); const model = parsed.status === 'PASS' && parsed.value && typeof parsed.value.model === 'string' ? parsed.value.model : null;
      const allowed = typeof model === 'string' && modelPattern.test(model);
      save('capture-u006.json', { schema_version: '5', test_id: 'P0B-CX-PRIMARY-MODEL-001', method: 'POST', route, background_request_count: background, target_request_captured: true, model_extract_status: model ? 'PASS' : 'FAIL', model: allowed ? model : undefined, model_allowlist_status: allowed ? 'PASS' : 'FAIL', model_exact_astra: model === 'gpt-6-astra', auth_match_status: auth === `Bearer ${secret}` ? 'PASS' : 'FAIL', prompt_match_status: 'PASS', json_parse_status: parsed.status, response_content_type: 'text/event-stream', emitted_events: ['response.created', 'response.output_text.delta', 'response.completed'], response_closed: Boolean(res.writableFinished) });
      server.close(); server.closeIdleConnections();
    });
  });
});
server.listen(0, '127.0.0.1', () => save('ready.json', { address: '127.0.0.1', port: server.address().port, pid: process.pid, background_requests_supported: true, target_by_prompt_sentinel: true }));

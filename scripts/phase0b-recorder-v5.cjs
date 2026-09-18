'use strict';
// Disposable probe helper. No payload/header values are written to disk.
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const root = process.env.P0B_ROOT;
const events = [
  'data: {"type":"response.created","response":{"id":"resp_phase0b"}}',
  'data: {"type":"response.completed","response":{"id":"resp_phase0b"}}',
];
const bytes = Buffer.from(events.join('\n\n') + '\n\n', 'utf8');
const old = Buffer.from(events.join('`n`n') + '`n`n');
function shape(value, prefix = 'body', out = new Set()) {
  const type = value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value;
  out.add(`${prefix}:${type}`);
  if (type === 'array') for (const item of value) shape(item, `${prefix}[]`, out);
  if (type === 'object') for (const [key, item] of Object.entries(value)) {
    if (!/^[A-Za-z_][A-Za-z0-9_-]{0,80}$/.test(key)) throw new Error('UNSAFE_SCHEMA_KEY');
    shape(item, `${prefix}.${key}`, out);
  }
  return [...out].sort();
}
function save(name, obj) {
  const json = JSON.stringify(obj, null, 2) + '\n';
  for (const sentinel of [process.env.P0B_SECRET, process.env.P0B_PROMPT]) {
    if (sentinel && json.includes(sentinel)) throw new Error('SENTINEL_IN_METADATA');
  }
  fs.writeFileSync(path.join(root, name), json, {encoding: 'utf8', flag: 'wx'});
}
assert.equal(old.includes(Buffer.from([10, 10])), false);
assert.equal(bytes.includes(Buffer.from([10, 10])), true);
assert.equal(JSON.parse(events[0].slice(6)).type, 'response.created');
assert.equal(JSON.parse(events[1].slice(6)).type, 'response.completed');
let targetReceived = false;
const server = http.createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/selftest') {
    res.writeHead(200, {'Content-Type': 'text/event-stream', 'Connection': 'close'});
    res.end(bytes);
    return;
  }
  if (req.method === 'GET' && req.url.split('?')[0] === '/v1/models') {
    res.writeHead(200, {'Content-Type': 'application/json', 'Connection': 'close'});
    res.end(JSON.stringify({data: [{id: 'gpt-6-astra', object: 'model'}]}));
    return;
  }
  if (req.method !== 'POST' || req.url !== '/v1/responses' || targetReceived) {
    res.writeHead(404); res.end(); return;
  }
  targetReceived = true;
  let length = 0;
  const chunks = [];
  req.on('data', chunk => {
    length += chunk.length;
    if (length > 8 * 1024 * 1024) { req.destroy(); server.close(); return; }
    chunks.push(chunk);
  });
  req.on('end', () => {
    try {
      const body = Buffer.concat(chunks).toString('utf8');
      const parsed = JSON.parse(body);
      const auth = req.headers.authorization || '';
      const record = {
        method: req.method, route: '/v1/responses',
        content_type: req.headers['content-type'] || '',
        header_names: Object.keys(req.headers).sort(),
        model: parsed.model === 'gpt-6-astra' ? parsed.model : 'UNEXPECTED',
        request_shape: shape(parsed),
        authorization_header_present: auth.length > 0,
        secret_seen_at_expected_ingress: auth === `Bearer ${process.env.P0B_SECRET}`,
        prompt_seen_at_expected_ingress: body.includes(process.env.P0B_PROMPT),
        response_content_type: 'text/event-stream',
        emitted_events: events.map(line => JSON.parse(line.slice(6)).type),
        actual_lf_lf: bytes.includes(Buffer.from([10, 10])),
      };
      res.on('finish', () => {
        record.response_closed = res.writableFinished;
        save('capture.json', record);
        server.close();
        server.closeIdleConnections();
      });
      res.writeHead(200, {'Content-Type': 'text/event-stream', 'Connection': 'close'});
      res.end(bytes);
    } catch {
      process.exitCode = 1;
      res.destroy(); server.close(); server.closeAllConnections();
    }
  });
});
server.on('error', () => { process.exitCode = 1; });
server.listen(0, '127.0.0.1', () => save('ready.json', {
  port: server.address().port, address: server.address().address,
  old_has_lf_lf: old.includes(Buffer.from([10, 10])),
  new_has_lf_lf: bytes.includes(Buffer.from([10, 10])),
}));

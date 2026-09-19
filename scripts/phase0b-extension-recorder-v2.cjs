'use strict';

// Phase 0B one-request recorder. It records only typed stage outcomes and
// allowlisted structure metadata; request bytes, values, hashes and fragments
// never leave memory.
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const zlib = require('node:zlib');

const root = process.env.P0B_V2_ROOT;
const secret = process.env.P0B_V2_SECRET || '';
const prompt = process.env.P0B_V2_PROMPT || '';
const expectedModel = process.env.P0B_V2_BASELINE_MODEL || 'gpt-5.6-sol';
const maxBody = 8 * 1024 * 1024;
const events = [
  'response.created',
  'response.output_text.delta',
  'response.completed',
];
const sse = Buffer.from(events.map((type) => `data: ${JSON.stringify({ type, response: { id: 'resp_phase0b_baseline' } })}`).join('\n\n') + '\n\n', 'utf8');
const allowed = new Set(['$schema', '$defs', 'model', 'input', 'type', 'role', 'content', 'tools', 'tool_choice', 'parallel_tool_calls', 'reasoning', 'stream', 'store', 'include', 'text', 'client_metadata', 'prompt_cache_key', 'instructions', 'max_output_tokens', 'temperature', 'top_p', 'truncation', 'metadata', 'user', 'service_tier']);

function writeJson(name, value) {
  const text = JSON.stringify(value, null, 2) + '\n';
  if (secret.length > 8 && text.includes(secret)) throw new Error('SENTINEL_IN_EVIDENCE');
  if (prompt.length > 8 && text.includes(prompt)) throw new Error('SENTINEL_IN_EVIDENCE');
  fs.writeFileSync(path.join(root, name), text, { encoding: 'utf8', flag: 'w' });
}

function decodeStage(raw, encoding) {
  const value = (encoding || 'identity').toLowerCase();
  try {
    if (value === 'identity') return { status: 'PASS', buffer: raw };
    if (value === 'gzip') return { status: 'PASS', buffer: zlib.gunzipSync(raw) };
    if (value === 'deflate') return { status: 'PASS', buffer: zlib.inflateSync(raw) };
    if (value === 'br') return { status: 'PASS', buffer: zlib.brotliDecompressSync(raw) };
    return { status: 'FAIL', reason: 'CONTENT_ENCODING_UNSUPPORTED' };
  } catch {
    return { status: 'FAIL', reason: 'CONTENT_ENCODING_DECODE_FAILED' };
  }
}

function utf8Stage(buffer) {
  try {
    const text = new TextDecoder('utf-8', { fatal: true }).decode(buffer);
    return { status: 'PASS', text, bom: buffer.length >= 3 && buffer[0] === 0xef && buffer[1] === 0xbb && buffer[2] === 0xbf };
  } catch {
    return { status: 'FAIL', reason: 'UTF8_DECODE_FAILED' };
  }
}

function jsonStage(text) {
  try { return { status: 'PASS', value: JSON.parse(text.replace(/^\uFEFF/, '')) }; }
  catch { return { status: 'FAIL', reason: 'JSON_PARSE_FAILED' }; }
}

function structureStage(value) {
  const summary = { status: 'PASS', root_type: value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value, allowed_paths: [], redacted_key_count: 0, redacted_object_count: 0, max_depth: 0, node_count: 0 };
  const paths = new Set();
  function walk(node, depth, pathName) {
    summary.node_count += 1; summary.max_depth = Math.max(summary.max_depth, depth);
    if (Array.isArray(node)) { for (const item of node) walk(item, depth + 1, pathName + '[]'); return; }
    if (!node || typeof node !== 'object') return;
    summary.redacted_object_count += 1;
    for (const [key, item] of Object.entries(node)) {
      const safePath = pathName ? `${pathName}.${key}` : key;
      if (!allowed.has(key)) { summary.redacted_key_count += 1; walk(item, depth + 1, pathName + '.*'); continue; }
      paths.add(safePath); walk(item, depth + 1, safePath);
    }
  }
  try { walk(value, 0, ''); summary.allowed_paths = [...paths].sort(); return summary; }
  catch { return { status: 'FAIL', reason: 'STRUCTURE_SANITIZER_FAILED' }; }
}

function selfTests() {
  const hostile = '{"$schema":"x","$defs":{"d":{"type":"object"}},"model":"gpt-5.6-sol","a.b":{"nested":["{}","\\\"quoted\\\""],"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":"x"}}';
  const json = jsonStage('\uFEFF' + hostile);
  const structure = json.status === 'PASS' ? structureStage(json.value) : { status: 'FAIL' };
  const invalidUtf8 = utf8Stage(Buffer.from([0xc3, 0x28]));
  const invalidJson = jsonStage('{not-json');
  const gzip = decodeStage(zlib.gzipSync(Buffer.from(hostile, 'utf8')), 'gzip');
  const identity = decodeStage(Buffer.from(hostile, 'utf8'), 'identity');
  const serializer = JSON.stringify(structure);
  return { status: json.status === 'PASS' && structure.status === 'PASS' && invalidUtf8.status === 'FAIL' && invalidJson.status === 'FAIL' && gzip.status === 'PASS' && identity.status === 'PASS' && !serializer.includes('aaaaaaaa') ? 'PASS' : 'FAIL', json_parse: json.status, safe_structure: structure.status, invalid_utf8_rejected: invalidUtf8.status === 'FAIL', invalid_json_rejected: invalidJson.status === 'FAIL', gzip_decode: gzip.status, identity_decode: identity.status, unsafe_names_absent: !serializer.includes('aaaaaaaa'), raw_values_absent: !serializer.includes('gpt-5.6-sol') };
}

if (!root) throw new Error('P0B_V2_ROOT_REQUIRED');
fs.mkdirSync(root, { recursive: true });
writeJson('self-test.json', selfTests());

let handled = false;
const server = http.createServer((req, res) => {
  const route = (req.url || '').split('?')[0];
  if (req.method === 'GET' && route === '/v1/models') {
    res.writeHead(200, { 'Content-Type': 'application/json', Connection: 'close' });
    res.end(JSON.stringify({ object: 'list', data: [{ id: expectedModel, object: 'model' }] })); return;
  }
  if (handled || req.method !== 'POST' || route !== '/v1/responses') { res.writeHead(404, { Connection: 'close' }); res.end(); return; }
  handled = true;
  const authorization = req.headers.authorization || '';
  const record = { schema_version: '3', ordinal: 1, method: req.method, route, transport_body_received: false, raw_length: 0, content_encoding: req.headers['content-encoding'] || 'identity', content_decode_status: 'NOT_RUN', utf8_status: 'NOT_RUN', body_root_type: 'UNKNOWN', model_extract_status: 'NOT_RUN', model_observed: 'REDACTED', auth_match_status: authorization === `Bearer ${secret}` ? 'PASS' : 'FAIL', prompt_match_status: 'NOT_RUN', shape_summary_status: 'NOT_RUN', response_content_type: 'text/event-stream', emitted_events: events, response_closed: false };
  const chunks = []; let total = 0;
  req.on('data', (chunk) => { total += chunk.length; if (total <= maxBody) chunks.push(chunk); });
  req.on('end', () => {
    record.transport_body_received = true; record.raw_length = total;
    const decoded = decodeStage(Buffer.concat(chunks), record.content_encoding); record.content_decode_status = decoded.status;
    let bodyText = ''; let body;
    if (decoded.status === 'PASS') {
      const utf8 = utf8Stage(decoded.buffer); record.utf8_status = utf8.status;
      if (utf8.status === 'PASS') { bodyText = utf8.text; const parsed = jsonStage(bodyText); record.json_parse_status = parsed.status;
        if (parsed.status === 'PASS') { body = parsed.value; record.body_root_type = body === null ? 'null' : Array.isArray(body) ? 'array' : typeof body; }
        if (parsed.status === 'PASS' && body && typeof body === 'object' && !Array.isArray(body)) { record.model_extract_status = typeof body.model === 'string' ? 'PASS' : 'FAIL'; if (record.model_extract_status === 'PASS') { record.model_match_status = body.model === expectedModel ? 'PASS' : 'FAIL'; record.model_identity_class = body.model === expectedModel ? 'EXPECTED_BASELINE' : (body.model.startsWith('gpt-5.6-') ? 'KNOWN_FAMILY_OTHER' : 'OTHER_STRING'); } }
        else record.model_extract_status = 'FAIL';
        record.prompt_match_status = bodyText.includes(prompt) ? 'PASS' : 'FAIL';
        if (parsed.status === 'PASS') { const shape = structureStage(body); record.shape_summary_status = shape.status; record.shape_summary = shape; }
      }
    }
    res.writeHead(200, { 'Content-Type': 'text/event-stream', Connection: 'close' }); res.end(sse);
    res.on('finish', () => { record.response_closed = Boolean(res.writableFinished); writeJson('capture-1.json', record); writeJson('result.json', record); server.close(); server.closeIdleConnections(); });
  });
});
server.on('error', () => process.exitCode = 1);
server.listen(0, '127.0.0.1', () => writeJson('ready.json', { address: '127.0.0.1', port: server.address().port, pid: process.pid, one_request_only: true }));

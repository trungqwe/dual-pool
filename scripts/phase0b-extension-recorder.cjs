'use strict';

// Disposable two-request Responses recorder. Bodies and authorization values
// are checked in memory and never persisted.
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const zlib = require('node:zlib');
const crypto = require('node:crypto');

const root = process.env.P0B_ROOT;
const secret = process.env.P0B_SECRET;
const baselinePrompt = process.env.P0B_BASELINE_PROMPT;
const astraPrompt = process.env.P0B_ASTRA_PROMPT;
const initialModel = process.env.P0B_INITIAL_MODEL;
const astraModel = 'gpt-6-astra';
const frames = [
  'data: {"type":"response.created","response":{"id":"resp_phase0b_extension"}}',
  'data: {"type":"response.completed","response":{"id":"resp_phase0b_extension"}}',
];
const sse = Buffer.from(frames.join('\n\n') + '\n\n', 'utf8');

function shape(value, prefix = 'body', output = new Set()) {
  const type = value === null ? 'null' : Array.isArray(value) ? 'array' : typeof value;
  output.add(`${prefix}:${type}`);
  if (type === 'array') for (const item of value) shape(item, `${prefix}[]`, output);
  if (type === 'object') for (const [key, item] of Object.entries(value)) {
    if (!/^[A-Za-z_][A-Za-z0-9_-]{0,80}$/.test(key)) throw new Error('UNSAFE_SCHEMA_KEY');
    shape(item, `${prefix}.${key}`, output);
  }
  return [...output].sort();
}

function save(name, value) {
  const json = JSON.stringify(value, null, 2) + '\n';
  for (const sentinel of [secret, baselinePrompt, astraPrompt]) {
    if (sentinel && json.includes(sentinel)) throw new Error('SENTINEL_IN_METADATA');
  }
  fs.writeFileSync(path.join(root, name), json, { encoding: 'utf8', flag: 'wx' });
}

function decodeBody(buffer, encoding) {
  if (!encoding || encoding === 'identity') return buffer;
  if (encoding === 'gzip') return zlib.gunzipSync(buffer);
  if (encoding === 'deflate') return zlib.inflateSync(buffer);
  if (encoding === 'br') return zlib.brotliDecompressSync(buffer);
  throw new Error('UNSUPPORTED_CONTENT_ENCODING');
}

function parseFramedJson(text) {
  const normalized = text.replace(/^\uFEFF/, '').replace(/\u0000/g, '').trim();
  try { return JSON.parse(normalized); } catch {}
  const start = normalized.indexOf('{');
  if (start < 0) throw new Error('INVALID_JSON_NO_OBJECT');
  let depth = 0;
  let quoted = false;
  let escaped = false;
  for (let index = start; index < normalized.length; index += 1) {
    const character = normalized[index];
    if (quoted) {
      if (escaped) escaped = false;
      else if (character === '\\') escaped = true;
      else if (character === '"') quoted = false;
      continue;
    }
    if (character === '"') { quoted = true; continue; }
    if (character === '{') depth += 1;
    if (character === '}') {
      depth -= 1;
      if (depth === 0) return JSON.parse(normalized.slice(start, index + 1));
    }
  }
  throw new Error('INVALID_JSON_UNCLOSED_OBJECT');
}

let ordinal = 0;
const requests = [];
const server = http.createServer((request, response) => {
  const route = request.url.split('?')[0];
  if (request.method === 'GET' && route === '/v1/models') {
    response.writeHead(200, { 'Content-Type': 'application/json', Connection: 'close' });
    response.end(JSON.stringify({ object: 'list', data: [initialModel, astraModel].map(id => ({ id, object: 'model' })) }));
    return;
  }
  if (request.method !== 'POST' || route !== '/v1/responses' || ordinal >= 2) {
    response.writeHead(404, { Connection: 'close' });
    response.end();
    return;
  }
  let bytes = 0;
  const chunks = [];
  request.on('data', chunk => {
    bytes += chunk.length;
    if (bytes > 8 * 1024 * 1024) { request.destroy(); server.close(); }
    else chunks.push(chunk);
  });
  request.on('end', () => {
    let decodedBody = Buffer.alloc(0);
    let bodyText = '';
    try {
      const rawBody = Buffer.concat(chunks);
      decodedBody = decodeBody(rawBody, request.headers['content-encoding']);
      bodyText = decodedBody.toString('utf8');
      const body = parseFramedJson(bodyText);
      const authorization = request.headers.authorization || '';
      const expectedPrompt = body.model === astraModel ? astraPrompt : baselinePrompt;
      const record = {
        ordinal: ordinal + 1,
        method: request.method,
        route: '/v1/responses',
        model: body.model === initialModel || body.model === astraModel ? body.model : 'UNEXPECTED',
        content_type: request.headers['content-type'] || '',
        header_names: Object.keys(request.headers).sort(),
        authorization_header_present: authorization.length > 0,
        secret_seen_at_expected_ingress: authorization === `Bearer ${secret}`,
        baseline_sentinel_seen_at_expected_ingress: bodyText.includes(baselinePrompt),
        astra_sentinel_seen_at_expected_ingress: bodyText.includes(astraPrompt),
        expected_prompt_seen_at_expected_ingress: bodyText.includes(expectedPrompt),
        request_shape: shape(body),
        emitted_events: frames.map(line => JSON.parse(line.slice(6)).type),
        response_content_type: 'text/event-stream',
      };
      ordinal += 1;
      requests.push(record);
      response.on('finish', () => {
        record.response_closed = response.writableFinished;
        save(`capture-${record.ordinal}.json`, record);
        if (ordinal === 2) {
          save('captures.json', { request_count: 2, ordinals: requests.map(item => item.ordinal), models_observed: requests.map(item => item.model) });
          server.close();
          server.closeIdleConnections();
        }
      });
      response.writeHead(200, { 'Content-Type': 'text/event-stream', Connection: 'close' });
      response.end(sse);
    } catch {
      const errorRecord = {
        ordinal: ordinal + 1,
        method: request.method,
        route: '/v1/responses',
        content_type: request.headers['content-type'] || '',
        content_encoding: request.headers['content-encoding'] || 'identity',
        header_names: Object.keys(request.headers).sort(),
        authorization_header_present: Boolean(request.headers.authorization),
        request_parse_error: decodedBody.length === 0 ? 'EMPTY_BODY' : 'INVALID_JSON_OR_ENCODING',
        body_length: decodedBody.length,
        body_sha256: crypto.createHash('sha256').update(decodedBody).digest('hex'),
        body_prefix_hex: decodedBody.subarray(0, 16).toString('hex'),
      };
      ordinal += 1;
      requests.push(errorRecord);
      response.on('finish', () => {
        errorRecord.response_closed = response.writableFinished;
        try { save(`capture-${errorRecord.ordinal}.json`, errorRecord); } catch { process.exitCode = 1; }
        if (ordinal === 2) {
          try { save('captures.json', { request_count: 2, ordinals: requests.map(item => item.ordinal), models_observed: requests.map(item => item.model || 'UNREADABLE') }); } catch { process.exitCode = 1; }
          server.close();
          server.closeIdleConnections();
        }
      });
      response.writeHead(200, { 'Content-Type': 'text/event-stream', Connection: 'close' });
      response.end(sse);
    }
  });
});
server.on('error', () => { process.exitCode = 1; });
server.listen(0, '127.0.0.1', () => save('ready.json', {
  address: server.address().address,
  port: server.address().port,
  pid: process.pid,
  two_request_target: true,
  sse_lf_lf: sse.includes(Buffer.from([10, 10])),
}));

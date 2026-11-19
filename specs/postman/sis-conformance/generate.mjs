#!/usr/bin/env node
// Sinh bộ conformance test EduWorld SIS Contract v1 (F017 T049/T050, FR-SIS-009) từ
// specs/sis/eduworld-sis-contract.v1.yaml: mỗi request kiểm status, JSON Schema của response (schema nhúng
// được dereference từ components.schemas — contract là nguồn duy nhất), phân trang cursor, lỗi
// application/problem+json và 401 khi thiếu credential. Capability tùy chọn (delta, student_lookup, webhook)
// không khai báo trong getHealth → request được bỏ qua (skipped), không fail.
//
// Chạy:  npm ci --prefix scripts && node specs/postman/sis-conformance/generate.mjs [--check]
//   --check: không ghi file, exit 1 nếu collection đang commit khác bản sinh ra (CI).
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import crypto from 'node:crypto';

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '..', '..', '..');
const require = createRequire(path.join(repo, 'scripts', 'package.json'));
const yaml = require('js-yaml');

const CONTRACT = path.join(repo, 'specs/sis/eduworld-sis-contract.v1.yaml');
const OUT = path.join(here, 'sis-conformance.postman_collection.json');
const contract = yaml.load(fs.readFileSync(CONTRACT, 'utf8'));
const schemas = contract.components.schemas;
const maxLimit = contract.components.parameters.Limit.schema.maximum;

// Dereference a schema; drop annotations and `format` (the Postman validator does not know every format).
function deref(node) {
  if (Array.isArray(node)) return node.map(deref);
  if (!node || typeof node !== 'object') return node;
  if (node.$ref) return deref(schemas[node.$ref.split('/').pop()]);
  const out = {};
  for (const [k, v] of Object.entries(node)) {
    if (['description', 'examples', 'example', 'format', 'summary'].includes(k)) continue;
    out[k] = deref(v);
  }
  return out;
}
const schemaOf = (name) => deref(schemas[name]);
const yearsSchema = deref(contract.paths['/schools/{school_external_id}/academic-years'].get.responses['200'].content['application/json'].schema);
const problemSchema = schemaOf('Problem');

const js = (lines) => lines.join('\n').split('\n');

// Shared assertions installed once at collection level.
const collectionTest = js([
  `pm.globals.set('ew_sis_contract', '${contract.info.version}');`,
]);
const lib = `
const ew = {
  json: function () { try { return pm.response.json(); } catch (e) { return null; } },
  problem: function (status) {
    pm.test('HTTP ' + status + ' application/problem+json', function () {
      pm.response.to.have.status(status);
      pm.expect(pm.response.headers.get('Content-Type') || '').to.include('application/problem+json');
      pm.response.to.have.jsonSchema(${JSON.stringify(problemSchema)});
    });
  },
  page: function (schema, limit) {
    pm.test('HTTP 200 + JSON Schema của contract', function () {
      pm.response.to.have.status(200);
      pm.response.to.have.jsonSchema(schema);
    });
    const b = ew.json() || {};
    pm.test('Phân trang cursor: has_more ⇒ next_cursor mới, số bản ghi ≤ limit', function () {
      pm.expect(b.page).to.be.an('object');
      if (b.page.has_more) { pm.expect(b.page.next_cursor).to.be.a('string').and.not.empty; pm.expect(b.page.next_cursor).to.not.equal(pm.request.url.query.get('cursor')); }
      pm.expect((b.data || []).length).to.be.at.most(limit);
    });
    return b;
  },
  needs: function (cap) {
    const caps = JSON.parse(pm.collectionVariables.get('capabilities') || '[]');
    if (caps.indexOf(cap) < 0) { console.log('capability ' + cap + ' không khai báo: bỏ qua'); pm.execution.skipRequest(); }
  },
  mock: function (code) { if (pm.environment.get('mock_prefer') === 'true') { pm.request.headers.upsert({ key: 'Prefer', value: 'code=' + code }); } }
};`;

function auth() {
  // X-Api-Key or Bearer from the environment; mTLS is configured in the Postman CLI (--ssl-client-cert).
  return [
    { key: 'X-Api-Key', value: '{{api_key}}', disabled: false, type: 'text' },
  ];
}
const preAuth = js([
  `if (pm.environment.get('auth_type') === 'oauth2_client_credentials') {`,
  `  pm.request.headers.remove('X-Api-Key');`,
  `  pm.request.headers.upsert({ key: 'Authorization', value: 'Bearer ' + pm.environment.get('access_token') });`,
  `} else if (pm.environment.get('auth_type') === 'mtls') { pm.request.headers.remove('X-Api-Key'); }`,
]);

function request(name, method, p, query, tests, pre = [], headers = auth()) {
  return {
    name,
    event: [
      { listen: 'prerequest', script: { type: 'text/javascript', exec: [...js([lib]), ...preAuth, ...pre] } },
      { listen: 'test', script: { type: 'text/javascript', exec: [...js([lib]), ...tests] } },
    ],
    request: {
      method,
      header: headers,
      url: { raw: `{{sis_base_url}}${p}${query.length ? '?' + query.map(([k, v]) => `${k}=${v}`).join('&') : ''}`,
        host: ['{{sis_base_url}}'], path: p.split('/').filter(Boolean), query: query.map(([key, value]) => ({ key, value })) },
    },
  };
}

const S = '/schools/{{school_external_id}}';
const required = [
  request('getHealth — contract_version major 1, capabilities', 'GET', '/health', [], js([
    `pm.test('HTTP 200 + JSON Schema Health', function () { pm.response.to.have.status(200); pm.response.to.have.jsonSchema(${JSON.stringify(schemaOf('Health'))}); });`,
    `const h = ew.json() || {};`,
    `pm.collectionVariables.set('capabilities', JSON.stringify(h.capabilities || []));`,
    `pm.test('contract_version cùng major với EduWorld (1.x)', function () { pm.expect(String(h.contract_version)).to.match(/^1\\./); });`,
  ])),
  request('getHealth thiếu credential → 401 problem+json', 'GET', '/health', [], js([`ew.problem(401);`]),
    js([`ew.mock(401);`, `pm.request.headers.remove('X-Api-Key'); pm.request.headers.remove('Authorization');`,
      `if (pm.environment.get('auth_type') === 'mtls') { pm.execution.skipRequest(); }`])),
  request('listSchools — trang đầu', 'GET', '/schools', [['limit', String(maxLimit)]], js([
    `ew.page(${JSON.stringify(schemaOf('SchoolPage'))}, ${maxLimit});`,
  ])),
  request(`listSchools limit > ${maxLimit} → 400 problem+json`, 'GET', '/schools', [['limit', String(maxLimit + 1)]], js([`ew.problem(400);`]),
    js([`ew.mock(400);`])),
  request('listAcademicYears', 'GET', `${S}/academic-years`, [], js([
    `pm.test('HTTP 200 + JSON Schema', function () { pm.response.to.have.status(200); pm.response.to.have.jsonSchema(${JSON.stringify(yearsSchema)}); });`,
    `pm.test('Có đúng một năm học hiện hành', function () { pm.expect(((ew.json() || {}).data || []).filter(function (y) { return y.is_current; }).length).to.equal(1); });`,
  ])),
  request('listClasses', 'GET', `${S}/classes`, [['academic_year', '{{academic_year}}'], ['limit', '{{page_limit}}']], js([
    `ew.page(${JSON.stringify(schemaOf('ClassPage'))}, Number(pm.environment.get('page_limit')));`,
  ])),
  request('listStudents — trang đầu và trang kế theo next_cursor', 'GET', `${S}/students`, [['limit', '{{page_limit}}']], js([
    `const b = ew.page(${JSON.stringify(schemaOf('StudentPage'))}, Number(pm.environment.get('page_limit')));`,
    `if (b.page && b.page.has_more) {`,
    `  const url = pm.request.url.toString().replace(/([?&])cursor=[^&]*/, '$1') + '&cursor=' + encodeURIComponent(b.page.next_cursor);`,
    `  pm.sendRequest({ url: url, method: 'GET', header: pm.request.headers.toObject() }, function (err, res) {`,
    `    pm.test('Trang kế theo next_cursor: HTTP 200', function () { pm.expect(err).to.equal(null); pm.expect(res.code).to.equal(200); });`,
    `  });`,
    `}`,
  ])),
  request('listEnrollments', 'GET', `${S}/enrollments`, [['academic_year', '{{academic_year}}'], ['limit', '{{page_limit}}']], js([
    `ew.page(${JSON.stringify(schemaOf('EnrollmentPage'))}, Number(pm.environment.get('page_limit')));`,
  ])),
];

// Known HMAC vector: the signature EduWorld verifies (X-EduWorld-Signature, SIS_INTEGRATION §5).
const vector = { secret: 'whsec-conformance', timestamp: '1800000000',
  body: JSON.stringify({ delivery_id: '0193a5f4-6c1e-7b2a-9d1f-0000000000d1', contract_version: contract.info.version, changes: [
    { sequence: 1, entity_kind: 'enrollment', operation: 'delete', external_id: 'HS-DEMO-0002|C-3A-2026', occurred_at: '2026-11-01T08:00:00Z' }] }) };
vector.signature = 'sha256=' + crypto.createHmac('sha256', vector.secret).update(vector.timestamp + '.' + vector.body).digest('hex');

const optional = [
  request('listChanges (delta) — thứ tự sequence, watermark', 'GET', '/changes', [['limit', '{{page_limit}}']], js([
    `const b = ew.page(${JSON.stringify(schemaOf('ChangePage'))}, Number(pm.environment.get('page_limit')));`,
    `pm.test('sequence tăng dần, record khi upsert', function () {`,
    `  const d = b.data || [];`,
    `  for (let i = 1; i < d.length; i++) { pm.expect(d[i].sequence).to.be.above(d[i - 1].sequence); }`,
    `  d.forEach(function (c) { if (c.operation === 'upsert') { pm.expect(c.record).to.be.an('object'); } });`,
    `});`,
  ]), js([`ew.needs('delta');`])),
  request('listChanges watermark hết hạn → 410 problem+json', 'GET', '/changes', [['since', '{{expired_watermark}}']], js([`ew.problem(410);`]),
    js([`ew.needs('delta');`, `ew.mock(410);`])),
  request('lookupStudent — tìm thấy', 'GET', `${S}/students/lookup`, [['login_username', '{{lookup_username}}']], js([
    `pm.test('HTTP 200 + JSON Schema Student', function () { pm.response.to.have.status(200); pm.response.to.have.jsonSchema(${JSON.stringify(schemaOf('Student'))}); });`,
    `pm.test('Trả lời trong 3 giây (sis_login_sync_timeout_ms)', function () { pm.expect(pm.response.responseTime).to.be.below(3000); });`,
  ]), js([`ew.needs('student_lookup');`])),
  request('lookupStudent — không tìm thấy → 404 problem+json', 'GET', `${S}/students/lookup`, [['login_username', '__eduworld_conformance_unknown__']],
    js([`ew.problem(404);`]), js([`ew.needs('student_lookup');`, `ew.mock(404);`])),
  {
    name: 'Webhook mẫu ký HMAC (receiveSisChanges) — kiểm thuật toán chữ ký',
    event: [
      { listen: 'prerequest', script: { type: 'text/javascript', exec: [...js([lib]), ...js([
        `ew.needs('webhook');`,
        `const v = ${JSON.stringify(vector)};`,
        `const CryptoJS = require('crypto-js');`,
        `const sig = 'sha256=' + CryptoJS.HmacSHA256(v.timestamp + '.' + v.body, v.secret).toString(CryptoJS.enc.Hex);`,
        `pm.collectionVariables.set('webhook_signature_ok', String(sig === v.signature));`,
        `if (!pm.environment.get('eduworld_webhook_url')) { pm.request.url = pm.environment.get('sis_base_url') + '/health'; pm.request.method = 'GET'; }`,
        `pm.request.headers.upsert({ key: 'X-EduWorld-Timestamp', value: v.timestamp });`,
        `pm.request.headers.upsert({ key: 'X-EduWorld-Signature', value: sig });`,
      ])] } },
      { listen: 'test', script: { type: 'text/javascript', exec: js([
        `pm.test('HMAC-SHA256(secret, timestamp + "." + raw_body) khớp vector của EduWorld', function () { pm.expect(pm.collectionVariables.get('webhook_signature_ok')).to.equal('true'); });`,
        `if (pm.environment.get('eduworld_webhook_url')) { pm.test('EduWorld nhận lô (202) hoặc báo replay (409)', function () { pm.expect([202, 409]).to.include(pm.response.code); }); }`,
      ]) } },
    ],
    request: { method: 'POST', header: [{ key: 'Content-Type', value: 'application/json' }],
      body: { mode: 'raw', raw: vector.body }, url: { raw: '{{eduworld_webhook_url}}', host: ['{{eduworld_webhook_url}}'] } },
  },
];

const collection = {
  info: {
    name: 'EduWorld SIS Contract v1 — conformance',
    _postman_id: '6f3c7a3e-17f0-4c2b-9e5a-0f017c0f0f17',
    description: `Bộ conformance test sinh từ specs/sis/eduworld-sis-contract.v1.yaml (contract ${contract.info.version}). KHÔNG SỬA TAY: chạy node specs/postman/sis-conformance/generate.mjs. Hướng dẫn: README.md cùng thư mục.`,
    schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json',
  },
  event: [{ listen: 'prerequest', script: { type: 'text/javascript', exec: collectionTest } }],
  variable: [{ key: 'capabilities', value: '[]' }, { key: 'webhook_signature_ok', value: 'false' }],
  item: [
    { name: 'Bắt buộc (full_sync)', item: required },
    { name: 'Tùy chọn theo capability (delta, student_lookup, webhook)', item: optional },
  ],
};

const text = JSON.stringify(collection, null, 2) + '\n';
if (process.argv.includes('--check')) {
  const cur = fs.existsSync(OUT) ? fs.readFileSync(OUT, 'utf8').replace(/\r\n/g, '\n') : '';
  if (cur !== text) { console.error('sis-conformance: collection lỗi thời — chạy node specs/postman/sis-conformance/generate.mjs'); process.exit(1); }
  console.log('sis-conformance: collection khớp contract');
} else {
  fs.writeFileSync(OUT, text);
  console.log(`sis-conformance: ${required.length} request bắt buộc, ${optional.length} request tùy chọn → ${path.relative(repo, OUT)}`);
}

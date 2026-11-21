// Unit tests of the breaking-change allowlist (node --test scripts/ci/breaking-allowlist.test.mjs).
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { classify, matches, normalizeFor30, validateAllowlist } from './breaking-allowlist.mjs';

const change = (o) => ({ id: 'request-parameter-enum-value-removed', level: 3, operation: 'GET', path: '/admin/trash',
  text: "removed the enum value 'course' from the 'query' request parameter 'resource_type'", ...o });
const entry = (o) => ({ adr: 'ADR-0095', reason: 'no course', file: 'specs/openapi/modules/identity.v1.yaml',
  id: 'request-parameter-enum-value-removed', operation: 'GET', path: '/admin/trash', text: "'course'", ...o });
const F = 'specs/openapi/modules/identity.v1.yaml';

test('an exact entry accepts its change only', () => {
  assert.ok(matches(entry(), F, change()));
  assert.ok(!matches(entry(), 'specs/openapi/modules/learning.v1.yaml', change()), 'other file');
  assert.ok(!matches(entry(), F, change({ path: '/admin/trash/x' })), 'other path');
  assert.ok(!matches(entry(), F, change({ operation: 'DELETE' })), 'other method');
  assert.ok(!matches(entry(), F, change({ id: 'api-path-removed-without-deprecation' })), 'other check');
  assert.ok(!matches(entry(), F, change({ text: "removed the enum value 'quest'" })), 'other text');
  assert.ok(matches(entry({ operation: undefined, text: undefined }), F, change({ operation: 'POST' })), 'method and text optional');
  assert.ok(matches(entry({ file: ['specs/openapi/eduworld-api.v1.yaml', F] }), F, change()), 'list of files');
});

test('classify keeps every unlisted error and ignores warnings', () => {
  const { accepted, rejected } = classify([entry()], F, [change(), change({ path: '/other' }), change({ path: '/warn', level: 2 })]);
  assert.equal(accepted.length, 1);
  assert.equal(accepted[0].entry.adr, 'ADR-0095');
  assert.deepEqual(rejected.map((c) => c.path), ['/other']);
  assert.equal(classify([], F, [change()]).rejected.length, 1, 'an empty allowlist rejects everything');
});

test('the allowlist must be explicit and cite an existing ADR', () => {
  const exists = (a) => a === 'ADR-0095';
  assert.deepEqual(validateAllowlist([entry()], exists), []);
  assert.match(validateAllowlist([entry({ adr: 'ADR-9999' })], exists).join(), /no file in docs\/adr/);
  assert.match(validateAllowlist([entry({ adr: 'adr95' })], exists).join(), /must look like/);
  assert.match(validateAllowlist([entry({ reason: '' })], exists).join(), /missing reason/);
  assert.match(validateAllowlist([entry({ path: '/admin/*' })], exists).join(), /no wildcard/);
  assert.match(validateAllowlist([entry({ operation: 'get' })], exists).join(), /HTTP method/);
  assert.match(validateAllowlist([entry({ file: 'docs/x.yaml' })], exists).join(), /under specs\/openapi/);
  assert.match(validateAllowlist({}, exists).join(), /must be a list/);
});

test('numeric exclusive bounds of OpenAPI 3.1 become the 3.0 form for the oasdiff loader', () => {
  const doc = normalizeFor30({ a: { type: 'number', exclusiveMinimum: 0, exclusiveMaximum: 1 }, b: [{ exclusiveMinimum: true, minimum: 2 }] });
  assert.deepEqual(doc.a, { type: 'number', minimum: 0, exclusiveMinimum: true, maximum: 1, exclusiveMaximum: true });
  assert.deepEqual(doc.b[0], { exclusiveMinimum: true, minimum: 2 });
  assert.deepEqual(normalizeFor30({ type: 'array', prefixItems: [{ type: 'string' }], items: false }), { type: 'array', prefixItems: [{ type: 'string' }] });
});

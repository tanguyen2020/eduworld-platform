// F132-AC-21 (T030): rule no-client-authoritative-fields — a student request declaring score/xp/is_correct/rating
// fails at any depth; the same field in a response, a portal action or an internal-gateway route does not.
// Run: node --test specs/spectral-functions/
import test from 'node:test';
import assert from 'node:assert/strict';
import fn, { forbiddenRequestFields, isStudentOperation } from './noClientAuthoritativeFields.js';
import { checkDocument } from '../../scripts/specs/check-client-authoritative.mjs';

const body = (schema) => ({ requestBody: { content: { 'application/json': { schema } } } });

test('score in a student request body is an error (Spectral entry point)', () => {
  const op = body({ type: 'object', properties: { response: { type: 'string', maxLength: 10 }, score: { type: 'number' } } });
  const res = fn(op, null, { path: ['paths', '/learning/activity-attempts/{attempt_id}/answers', 'post'] });
  assert.equal(res.length, 1);
  assert.match(res[0].message, /"score"/);
  assert.deepEqual(res[0].path.slice(-2), ['properties', 'score']);
});

test('every forbidden name is found at any depth (items, allOf, oneOf, additionalProperties)', () => {
  const op = body({
    allOf: [
      { type: 'object', properties: { answers: { type: 'array', items: { type: 'object', properties: { is_correct: { type: 'boolean' } } } } } },
      { oneOf: [{ type: 'object', properties: { result: { type: 'object', properties: { xp: { type: 'integer' } } } } }] },
      { type: 'object', additionalProperties: { type: 'object', properties: { rating: { type: 'integer' } } } },
    ],
  });
  assert.deepEqual(forbiddenRequestFields(op).map((f) => f.field).sort(), ['is_correct', 'rating', 'xp']);
});

test('the same field in a response is not checked', () => {
  const op = { responses: { 200: { content: { 'application/json': { schema: { type: 'object', properties: { score: { type: 'number' } } } } } } } };
  assert.deepEqual(fn(op, null, { path: ['paths', '/learning/activity-attempts/{attempt_id}:submit', 'post'] }), []);
});

test('portal actions, internal-gateway routes, staff paths and GET are not student requests', () => {
  const op = body({ type: 'object', properties: { score: { type: 'number' } } });
  assert.equal(isStudentOperation('/game/quests', { ...op, 'x-portal-action': 'game.quest.write' }), false);
  assert.equal(isStudentOperation('/competition/x', { ...op, 'x-internal': true }), false);
  assert.equal(isStudentOperation('/game-admin/quests', op), false);
  assert.equal(isStudentOperation('/cms/learning/questions', op), false);
  assert.deepEqual(fn(op, null, { path: ['paths', '/game/quests', 'get'] }), []);
  assert.deepEqual(fn(op, null, { path: ['paths', '/lms/assignments', 'post'] }), []);
});

test('the node checker resolves local $ref like Spectral does', () => {
  const doc = {
    paths: {
      '/competition/matches/{match_id}:result': { post: { operationId: 'postResult', requestBody: { $ref: '#/components/requestBodies/R' } } },
      '/exams/attempts/{id}:submit': { post: { operationId: 'ok', ...body({ $ref: '#/components/schemas/Ok' }) } },
    },
    components: {
      requestBodies: { R: { content: { 'application/json': { schema: { $ref: '#/components/schemas/Bad' } } } } },
      schemas: { Bad: { type: 'object', properties: { rating: { type: 'integer' } } }, Ok: { type: 'object', properties: { answers: { type: 'array' } } } },
    },
  };
  const v = checkDocument(doc, 'fixture.yaml');
  assert.equal(v.length, 1);
  assert.equal(v[0].operation, 'postResult');
  assert.equal(v[0].field, 'rating');
});

test('a circular schema terminates', () => {
  const s = { type: 'object', properties: {} };
  s.properties.child = s;
  assert.deepEqual(forbiddenRequestFields(body(s)), []);
});

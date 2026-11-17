// F004 T032 — F004-AC-22: check-no-hardcode.mjs.
//   node --test deploy/apisix/scripts/check-no-hardcode.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { scan, FILES } from './check-no-hardcode.mjs';

test('literals are reported', () => {
  for (const line of ['  core-api: http://core-api:8100', '  host: portal.eduworld.vn', '  whitelist: [10.0.0.0/8]',
    '  core-api: core-api:8100', '"rootUrl": "https://auth.example.com"', '  discovery: keycloak.svc']) {
    assert.equal(scan('f', line).length > 0, true, line);
  }
});

test('variables, comments and plain values are accepted', () => {
  for (const line of ['  core-api: ${EW_UPSTREAM_CORE_API}', '"secret": "$(env:KC_PORTAL_CLIENT_SECRET)"',
    '# e.g. http://localhost:9080', '  uris: [/api/v1/game/*]', '  priority: 300', '"accessTokenLifespan": 900',
    '"alias": "ew-portal-ref-otp"', '"name": "eduworld-api-audience"']) {
    assert.deepEqual(scan('f', line), [], line);
  }
});

test('the repository files have no hardcoded endpoint', () => {
  const root = new URL('../../../', import.meta.url);
  for (const f of FILES) assert.deepEqual(scan(f, readFileSync(new URL(f, root), 'utf8')), [], f);
});

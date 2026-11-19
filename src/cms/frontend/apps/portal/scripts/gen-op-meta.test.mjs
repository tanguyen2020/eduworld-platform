// @vitest-environment node
// F050-AC-10: metadata from the merged spec (gateway by prefix, tenantHeader, idempotency, ifMatch) and drift
// detection used by `pnpm gen:check` (CI job of web.yml).
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import yaml from 'js-yaml';
import { describe, expect, it } from 'vitest';
import { buildOperationMeta, findDrift, forTypes, formatOperationMeta, generateTypes, OUT_META, SPEC } from './gen-op-meta.mjs';

const spec = yaml.load(fs.readFileSync(SPEC, 'utf8'));
const meta = buildOperationMeta(spec);
const byId = (id) => meta.find((o) => o.operationId === id);

describe('operation metadata', () => {
  it('covers every operation of the merged spec', () => {
    let n = 0;
    for (const item of Object.values(spec.paths)) for (const m of ['get', 'put', 'post', 'delete', 'patch']) if (item[m]) n++;
    expect(meta).toHaveLength(n);
    expect(new Set(meta.map((o) => o.operationId)).size).toBe(n);
  });

  it('reference operations', () => {
    expect(byId('getMyPermissions')).toMatchObject({ method: 'GET', path: '/me/permissions', tenantHeader: 'optional', gateway: 'public' });
    expect(byId('listMyTenants')).toMatchObject({ tenantHeader: 'none', idempotency: false });
    expect(byId('revokeAllSessions')).toMatchObject({ idempotency: true, tenantHeader: 'required' });
    expect(byId('updateCountry')).toMatchObject({ ifMatch: true, gateway: 'internal' });
  });

  it('header parameters at path-item level and inline are resolved', () => {
    const fixture = {
      components: { parameters: { TenantIdHeader: { name: 'X-Tenant-Id', in: 'header', required: true }, IdempotencyKey: { name: 'Idempotency-Key', in: 'header', required: true } } },
      paths: {
        '/sis/things': {
          parameters: [{ $ref: '#/components/parameters/TenantIdHeader' }],
          post: { operationId: 'createThing', parameters: [{ $ref: '#/components/parameters/IdempotencyKey' }, { name: 'If-Match', in: 'header', required: true }] },
        },
      },
    };
    expect(buildOperationMeta(fixture)).toEqual([
      { operationId: 'createThing', method: 'POST', path: '/sis/things', service: null, gateway: 'internal', tenantHeader: 'required', idempotency: true, ifMatch: true },
    ]);
  });

  it('client-managed headers are optional in the generated types only', { timeout: 120_000 }, async () => {
    const relaxed = forTypes(spec);
    expect(relaxed.components.parameters.TenantIdHeader.required).toBe(false);
    expect(spec.components.parameters.TenantIdHeader.required).toBe(true);
    const tiny = {
      openapi: '3.1.0',
      info: { title: 't', version: '1' },
      components: { parameters: { TenantIdHeader: { name: 'X-Tenant-Id', in: 'header', required: true, schema: { type: 'string' } } } },
      paths: { '/x': { get: { operationId: 'x', parameters: [{ $ref: '#/components/parameters/TenantIdHeader' }], responses: { 200: { description: 'ok' } } } } },
    };
    expect(await generateTypes(tiny)).toMatch(/"X-Tenant-Id"\?:/);
  });
});

describe('gen:check drift', () => {
  it('committed operation-meta.gen.ts matches the spec', () => {
    expect(findDrift(new Map([[OUT_META, formatOperationMeta(meta)]]))).toEqual([]);
  });

  it('a spec change not regenerated is reported', () => {
    const changed = structuredClone(spec);
    changed.paths['/admin/new-thing'] = { get: { operationId: 'newThing' } };
    const tmp = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'ew-gen-')), 'operation-meta.gen.ts');
    fs.writeFileSync(tmp, formatOperationMeta(meta));
    expect(findDrift(new Map([[tmp, formatOperationMeta(buildOperationMeta(changed))]]))).toHaveLength(1);
  });
});

// F050-AC-03 (AGENTS R3): config from /config.json, fallback VITE_* in dev; environment URL literals fail lint.
import { ESLint } from 'eslint';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { loadRuntimeConfig, RuntimeConfigError } from './runtime-config';

const FILE_CONFIG = {
  apiBaseUrl: 'https://api.stg.example/api/v1/',
  apiInternalBaseUrl: 'https://ops.stg.example/api/v1',
  oidcIssuer: 'https://id.stg.example/realms/eduworld',
  oidcClientId: 'eduworld-portal',
  env: 'staging',
};
const ENV = {
  DEV: true,
  VITE_API_BASE_URL: 'http://dev.local/api/v1',
  VITE_API_INTERNAL_BASE_URL: 'http://dev-internal.local/api/v1',
  VITE_OIDC_ISSUER: 'http://dev-idp.local/realms/eduworld',
  VITE_OIDC_CLIENT_ID: 'eduworld-portal',
  VITE_APP_ENV: 'local',
};

// MOCK fetch (test only)
const okFetch = (body: unknown) => (async () => new Response(JSON.stringify(body), { status: 200 })) as unknown as typeof fetch;
const notFound = (async () => new Response('', { status: 404 })) as unknown as typeof fetch;

describe('loadRuntimeConfig', () => {
  it('reads /config.json (trailing slash trimmed)', async () => {
    const c = await loadRuntimeConfig(okFetch(FILE_CONFIG), {}, false);
    expect(c).toEqual({ ...FILE_CONFIG, apiBaseUrl: 'https://api.stg.example/api/v1' });
  });

  it('falls back to VITE_* in development when /config.json is missing', async () => {
    const c = await loadRuntimeConfig(notFound, ENV);
    expect(c.apiBaseUrl).toBe(ENV.VITE_API_BASE_URL);
    expect(c.env).toBe('local');
  });

  it('in production there is no env fallback', async () => {
    await expect(loadRuntimeConfig(notFound, { ...ENV, DEV: false })).rejects.toBeInstanceOf(RuntimeConfigError);
  });

  it('reports missing or invalid keys', async () => {
    const err = await loadRuntimeConfig(okFetch({ ...FILE_CONFIG, apiInternalBaseUrl: 'not a url', oidcClientId: '' }), {}, false).catch((e) => e);
    expect(err).toBeInstanceOf(RuntimeConfigError);
    expect(err.missing).toEqual(['apiInternalBaseUrl', 'oidcClientId']);
  });
});

describe('lint: no environment URL in source', () => {
  const root = path.resolve(__dirname, '../../../..');
  const eslint = new ESLint({ cwd: root });
  const lint = async (code: string) =>
    (await eslint.lintText(code, { filePath: path.join(root, 'apps/portal/src/app/Probe.ts') }))[0]!.messages.map((m) => m.ruleId);

  it.each([
    'export const u = "http://localhost:9080/api/v1";',
    'export const u = `https://api.eduworld.vn/api/v1`;',
    'export const u = fetch("http://127.0.0.1:9180/x");',
  ])('%s → ew/no-env-url-literal', async (code) => {
    expect(await lint(code)).toContain('ew/no-env-url-literal');
  });

  it('config read from RuntimeConfig passes', async () => {
    expect(await lint('export const u = (c: { apiBaseUrl: string }) => `${c.apiBaseUrl}/me/tenants`;')).not.toContain('ew/no-env-url-literal');
  });
});

// F050-AC-03, F051 T002 (AGENTS R3, R33): config from /config.json, fallback VITE_* in dev; environment URL
// literals fail lint; no OIDC keys any more (ADR-0091); durations are positive integers read from the config.
import { ESLint } from 'eslint';
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { loadRuntimeConfig, RuntimeConfigError, validateRuntimeConfig } from './runtime-config';

const FILE_CONFIG = {
  apiBaseUrl: 'https://api.stg.example/api/v1/',
  apiInternalBaseUrl: 'https://ops.stg.example/api/v1',
  env: 'staging',
  principalCacheTtlSeconds: 60,
  sessionIdleTimeoutSeconds: 1800,
};
const ENV = {
  DEV: true,
  VITE_API_BASE_URL: 'http://dev.local/api/v1',
  VITE_API_INTERNAL_BASE_URL: 'http://dev-internal.local/api/v1',
  VITE_APP_ENV: 'local',
  VITE_PRINCIPAL_CACHE_TTL_SECONDS: '60',
  VITE_SESSION_IDLE_TIMEOUT_SECONDS: '1800',
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
    expect(c.principalCacheTtlSeconds).toBe(60);
    expect(c.sessionIdleTimeoutSeconds).toBe(1800);
  });

  it('in production there is no env fallback', async () => {
    await expect(loadRuntimeConfig(notFound, { ...ENV, DEV: false })).rejects.toBeInstanceOf(RuntimeConfigError);
  });

  it('reports missing or invalid keys', async () => {
    const err = await loadRuntimeConfig(okFetch({ ...FILE_CONFIG, apiInternalBaseUrl: 'not a url', principalCacheTtlSeconds: 0 }), {}, false).catch((e) => e);
    expect(err).toBeInstanceOf(RuntimeConfigError);
    expect(err.missing).toEqual(['apiInternalBaseUrl', 'principalCacheTtlSeconds']);
  });

  it('the example config of the repository is complete and has no OIDC key (ADR-0091)', () => {
    const example = JSON.parse(fs.readFileSync(path.resolve(__dirname, '../../public/config.example.json'), 'utf8'));
    expect(Array.isArray(validateRuntimeConfig(example))).toBe(false);
    expect(Object.keys(example).some((k) => k.toLowerCase().includes('oidc'))).toBe(false);
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

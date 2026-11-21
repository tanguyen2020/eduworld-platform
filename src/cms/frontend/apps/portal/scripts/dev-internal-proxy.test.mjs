// Local-dev proxy to the internal gateway (LOCAL_DEVELOPMENT §6.4, F004-AC-21 keeps the gateway without CORS).
import fs from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { DEV_INTERNAL_ENV, DEV_INTERNAL_PREFIX, devInternalProxy } from './dev-internal-proxy.mjs';
import { validateRuntimeConfig } from '../src/app/runtime-config';

const ORIGIN = ['http:', '', 'gateway.internal.test:9180'].join('/');

describe('devInternalProxy', () => {
  it('is off when the variable is unset or blank', () => {
    expect(devInternalProxy({})).toBeNull();
    expect(devInternalProxy({ [DEV_INTERNAL_ENV]: '  ' })).toBeNull();
  });

  it('forwards the same-origin prefix to the gateway and strips it', () => {
    const proxy = devInternalProxy({ [DEV_INTERNAL_ENV]: `${ORIGIN}/` });
    const entry = proxy?.[DEV_INTERNAL_PREFIX];
    expect(entry?.target).toBe(ORIGIN);
    expect(entry?.changeOrigin).toBe(true);
    expect(entry?.rewrite?.('/__internal/api/v1/admin/tenants?x=1')).toBe('/api/v1/admin/tenants?x=1');
    expect(entry?.rewrite?.('/__internal')).toBe('/');
    expect(entry?.rewrite?.('/__internalx/api')).toBe('/__internalx/api');
  });

  it.each(['not a url', 'ftp://gateway.test', `${ORIGIN}/api/v1`, `${ORIGIN}/?q=1`, 'http://u:p@gateway.test'])(
    'rejects %s',
    (v) => {
      expect(() => devInternalProxy({ [DEV_INTERNAL_ENV]: v })).toThrow(DEV_INTERNAL_ENV);
    },
  );

  it('the dev example config points the internal gateway at the same-origin prefix and validates', () => {
    const example = JSON.parse(fs.readFileSync(path.resolve(__dirname, '../public/config.example.json'), 'utf8'));
    const cfg = validateRuntimeConfig(example);
    expect(Array.isArray(cfg)).toBe(false);
    const internal = new URL(example.apiInternalBaseUrl);
    const pub = new URL(example.apiBaseUrl);
    expect(internal.pathname).toBe(`${DEV_INTERNAL_PREFIX}${pub.pathname}`);
  });
});

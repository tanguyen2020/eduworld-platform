// @vitest-environment node
// F050-AC-14: Problem Details mapped by `code` (not `title`) to `error.<CODE>`; VALIDATION_FAILED errors[] →
// field errors; unknown code → `error.generic`; trace_id kept. MSW gateway (MOCK).
import { http } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import vi from '../../../i18n/vi.json';
import en from '../../../i18n/en.json';
import { ApiError } from './problem';
import { PUBLIC, problem, testClient } from '../test-utils';

const server = setupServer(
  http.get(`${PUBLIC}/catalog/schools`, () =>
    problem(422, 'VALIDATION_FAILED', {
      errors: [
        { field: 'name', code: 'MAX_LENGTH', message: 'too long' },
        { field: 'code', code: 'PATTERN' },
      ],
    }),
  ),
  http.get(`${PUBLIC}/me/tenants`, () => problem(409, 'SOMETHING_NEW_FROM_SERVER')),
  http.get(`${PUBLIC}/me/permissions`, () => problem(404, 'NOT_FOUND', { title: 'VALIDATION_FAILED' })),
);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterAll(() => server.close());

describe('Problem → ApiError', () => {
  it('VALIDATION_FAILED → error.VALIDATION_FAILED + per-field errors + trace_id', async () => {
    const { api } = testClient();
    const err = (await api.call(api.client.GET('/catalog/schools', {})).catch((e) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(422);
    expect(err.i18nKey).toBe('error.VALIDATION_FAILED');
    expect(err.fieldErrors).toEqual({ name: { code: 'MAX_LENGTH', message: 'too long' }, code: { code: 'PATTERN' } });
    expect(err.traceId).toBe('trace-123');
  });

  it('unknown code → error.generic, code kept for display', async () => {
    const { api } = testClient();
    const err = (await api.call(api.client.GET('/me/tenants', {})).catch((e) => e)) as ApiError;
    expect(err.code).toBe('SOMETHING_NEW_FROM_SERVER');
    expect(err.i18nKey).toBe('error.generic');
    expect(err.traceId).toBe('trace-123');
  });

  it('maps by code, never by title', async () => {
    const { api } = testClient();
    const err = (await api.call(api.client.GET('/me/permissions', {})).catch((e) => e)) as ApiError;
    expect(err.i18nKey).toBe('error.NOT_FOUND');
  });

  it('every error.* key exists in vi and en; error.generic and VPN_REQUIRED are present', () => {
    const viKeys = Object.keys(vi.error).sort();
    const enKeys = Object.keys(en.error).sort();
    expect(viKeys).toEqual(enKeys);
    expect(viKeys).toEqual(expect.arrayContaining(['generic', 'VPN_REQUIRED', 'NETWORK_ERROR', 'TOKEN_EXPIRED', 'TENANT_MISMATCH', 'VALIDATION_FAILED']));
  });
});

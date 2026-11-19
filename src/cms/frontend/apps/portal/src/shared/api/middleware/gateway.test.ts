// @vitest-environment node
// F050-AC-13 (TC-CMS-WS-03): /admin/, /sis/, /game-admin/, /game-media → internal gateway; others → public;
// internal gateway unreachable → vpn_required + VPN_REQUIRED. MSW gateways (MOCK).
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { operations } from '../operation-meta.gen';
import { gatewayForPath } from '../operations';
import { INTERNAL, PUBLIC, testClient } from '../test-utils';

const hits: string[] = [];
const server = setupServer(
  http.all(`${PUBLIC}/*`, ({ request }) => {
    hits.push(request.url);
    return HttpResponse.json({});
  }),
  http.all(`${INTERNAL}/*`, ({ request }) => {
    hits.push(request.url);
    return HttpResponse.json({});
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => {
  hits.length = 0;
  server.resetHandlers();
});
afterAll(() => server.close());

describe('gateway selection', () => {
  it('prefix table', () => {
    expect(gatewayForPath('/admin/catalog/countries')).toBe('internal');
    expect(gatewayForPath('/sis/connections')).toBe('internal');
    expect(gatewayForPath('/game-admin/quests')).toBe('internal');
    expect(gatewayForPath('/game-media')).toBe('internal');
    expect(gatewayForPath('/game-media/uploads')).toBe('internal');
    expect(gatewayForPath('/game/client-config')).toBe('public');
    expect(gatewayForPath('/administrators')).toBe('public');
    expect(gatewayForPath('/me/tenants')).toBe('public');
  });

  it('generated metadata agrees with the prefix rule for every operation', () => {
    for (const op of operations) expect(op.gateway, op.operationId).toBe(gatewayForPath(op.path));
    expect(operations.some((o) => o.gateway === 'internal')).toBe(true);
  });

  it('calls the internal base URL for /admin/* and the public one otherwise', async () => {
    const { api } = testClient();
    await api.fetch(new Request(`${PUBLIC}/admin/analytics/game/metrics`));
    await api.client.GET('/me/tenants', {});
    expect(hits[0]).toBe(`${INTERNAL}/admin/analytics/game/metrics`);
    expect(hits[1]).toBe(`${PUBLIC}/me/tenants`);
  });

  it('internal gateway unreachable → vpn_required event and VPN_REQUIRED error', async () => {
    server.use(http.all(`${INTERNAL}/*`, () => HttpResponse.error()));
    const { api, sleeps } = testClient();
    const seen: unknown[] = [];
    api.events.on('vpn_required', (d) => seen.push(d));
    const err = await api.fetch(new Request(`${PUBLIC}/admin/analytics/game/metrics`)).catch((e) => e);
    expect(err).toMatchObject({ code: 'VPN_REQUIRED', i18nKey: 'error.VPN_REQUIRED', status: 0 });
    expect(seen).toEqual([{ operationId: 'getGameMetrics' }]);
    expect(sleeps).toHaveLength(2); // GET retried twice before giving up
  });

  it('public gateway unreachable → NETWORK_ERROR, no VPN event', async () => {
    server.use(http.all(`${PUBLIC}/*`, () => HttpResponse.error()));
    const { api } = testClient();
    const seen: unknown[] = [];
    api.events.on('vpn_required', (d) => seen.push(d));
    await expect(api.client.GET('/me/tenants', {})).rejects.toMatchObject({ code: 'NETWORK_ERROR' });
    expect(seen).toEqual([]);
  });
});

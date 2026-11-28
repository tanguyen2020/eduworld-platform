// @F066 T052 — F066-AC-29 (TC-CMS-MENU-02) on the REAL stack: `sa.admin` turns `menu.content_studio.simulations` off for
// th-demo-a (tenant override, Feature flag screen F059); the portal of `ad.truong.a` (TH Demo A), still holding the
// permissions read before, calls listSimulationTemplates → `403 FEATURE_DISABLED` (feature_key = the menu) → notice
// "Tính năng đang tắt" and getMyPermissions reloaded → the "Mô phỏng" link disappears and /content-studio/simulations
// shows "Tính năng đang tạm tắt". The override is put back to its first value at the end.
// Needs EW_E2E_PORTAL_URL, EW_E2E_PASSWORD; skipped otherwise (OQ F051-9). No secret is written anywhere (AGENTS R4).
import { test } from '@playwright/test';
import { e2eReady } from '../support/auth';
import { runMenuToggle } from '../support/featureFlags';
import { openSession } from '../support/session';

test.describe('@F066 Mô phỏng — tắt menu cho tenant (F066-AC-29)', () => {
  test.skip(!e2eReady, 'real stack not configured (EW_E2E_PORTAL_URL, EW_E2E_PASSWORD) — OQ F051-9');

  test('menu.content_studio.simulations off for th-demo-a: 403 FEATURE_DISABLED, permissions reload, menu hidden', async ({ browser }) => {
    test.setTimeout(10 * 60_000);
    await runMenuToggle((user, onContext) => openSession(browser, user, onContext), {
      menuKey: 'menu.content_studio.simulations',
      path: '/content-studio/simulations',
      api: /\/api\/v1\/cms\/learning\/simulation-templates(\?|$)/,
      user: 'ad.truong.a',
      tenant: /TH Demo A/,
      reason: 'Kiểm thử E2E F066 T052 tắt menu mô phỏng',
    });
  });
});

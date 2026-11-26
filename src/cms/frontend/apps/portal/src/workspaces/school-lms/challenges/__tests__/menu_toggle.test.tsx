// F108 T023, T032, T033 (US3, workspace C part) — F108-AC-13 (TC-CMS-AUTHZ-06): the menu "Thử thách khối, giải trường"
// shows only with lms.challenge.write (school_admin); academic_manager / tenant_admin do not see it and the direct URL
// shows the portal 403 page. F108-AC-17 (TC-CMS-MENU-02): a turned-off menu is hidden (its URL shows "Tính năng đang
// tắt"), a turned-off action hides its button, and 403 FEATURE_DISABLED shows the notice and reloads the permissions.
// Menus come from the generated catalog (ADR-0090): the screen is attached to the catalog key, never registered by hand.
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { WORKSPACE_CATALOG } from '../../../../generated/menu-catalog';
import { screens } from '../../index';
import { academicManager, CH_IDS, problem, renderChallenges, stopPortals, tenantAdmin } from './challenges-mock';
import { fillWizard } from './wizard-helpers';

afterEach(() => {
  stopPortals();
  cleanup();
});

const MENU = 'Thử thách khối, giải trường';

describe('Menu và quyền (F108-AC-13)', () => {
  it('catalog: menu.school_lms.challenges requires lms.challenge.write and is attached to the challenges screen', () => {
    const menu = WORKSPACE_CATALOG.find((w) => w.key === 'school_lms')!.menus.find((m) => m.key === 'menu.school_lms.challenges')!;
    expect(menu.requiredPermissions).toEqual(['lms.challenge.write']);
    expect(menu.actions.map((a) => a.key)).toEqual(['action.school_lms.challenges.view', 'action.school_lms.challenges.create', 'action.school_lms.challenges.manage']);
    expect(screens['menu.school_lms.challenges'].name).toBe('ChallengesScreen');
  });

  it('school_admin sees the menu and the screen', async () => {
    await renderChallenges('/school-lms/assignments');
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    fireEvent.click(await within(nav).findByRole('link', { name: MENU }));
    expect(await screen.findByTestId('challenge-table')).toBeInTheDocument();
  });

  for (const [role, account] of [
    ['academic_manager', academicManager],
    ['tenant_admin', tenantAdmin],
  ] as const) {
    it(`${role}: no menu; direct URL → portal 403 page, no challenge API call`, async () => {
      const { backend } = await renderChallenges('/school-lms/assignments', { account: account() });
      const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
      await waitFor(() => expect(within(nav).getAllByRole('link').length).toBeGreaterThan(0));
      expect(within(nav).queryByRole('link', { name: MENU })).toBeNull();
      stopPortals();
      cleanup();
      const r2 = await renderChallenges('/school-lms/challenges', { account: account() });
      expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
      expect(r2.backend.requests.some((r) => /^\/lms\/(challenges|catalog\/challenge-templates)/.test(r.path))).toBe(false);
      expect(backend.requests.some((r) => /^\/lms\/challenges/.test(r.path))).toBe(false);
    });
  }
});

describe('Tắt menu/action (F108-AC-17)', () => {
  it('menu turned off: hidden; its URL shows "Tính năng đang tắt"', async () => {
    await renderChallenges('/school-lms/assignments', { disabledMenus: ['menu.school_lms.challenges'] });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await within(nav).findByRole('link', { name: 'Bài tập, đề, bài chẩn đoán' });
    expect(within(nav).queryByRole('link', { name: MENU })).toBeNull();
    stopPortals();
    cleanup();
    const r2 = await renderChallenges('/school-lms/challenges', { disabledMenus: ['menu.school_lms.challenges'] });
    expect(await screen.findByText('Tính năng đang tắt')).toBeInTheDocument();
    expect(r2.backend.requests.some((r) => /^\/lms\/challenges/.test(r.path))).toBe(false);
  });

  it('create / manage actions turned off: their buttons are hidden', async () => {
    await renderChallenges('/school-lms/challenges', { disabledActions: ['action.school_lms.challenges.create'] });
    expect(await screen.findByTestId('challenge-table')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Tạo thử thách' })).toBeNull();
    stopPortals();
    cleanup();
    await renderChallenges(`/school-lms/challenges/${CH_IDS.challengeTournament}`, { disabledActions: ['action.school_lms.challenges.manage'] });
    expect(await screen.findByTestId('tournament-status')).toBeInTheDocument();
    for (const name of ['Công bố giải', 'Hủy giải', 'Hủy thử thách', 'Đăng ký hộ theo đối tượng']) expect(screen.queryByRole('button', { name }), name).toBeNull();
  });

  it('403 FEATURE_DISABLED on createChallenge → "tính năng tạm tắt" notice and permissions reloaded', async () => {
    const { backend } = await renderChallenges('/school-lms/challenges/new');
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/lms/challenges' ? problem(403, 'FEATURE_DISABLED', { feature_key: 'menu.school_lms.challenges' }) : undefined));
    await fillWizard({ include: false, exclude: false });
    const before = backend.calls('GET', '/me/permissions').length;
    fireEvent.click(screen.getByRole('button', { name: 'Tạo thử thách' }));
    expect(await screen.findByText(/Tính năng đang tạm tắt/)).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });
});

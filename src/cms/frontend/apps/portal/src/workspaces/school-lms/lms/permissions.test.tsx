// F055 T038, T041, T049 (US6) — F055-AC-19…AC-22, AC-24 (TC-TEN-002, TC-CMS-WS-01, TC-CMS-AUTHZ-08, TC-CMS-AUTHZ-05,
// TC-CMS-MENU-02): write buttons only with their permission (tenant_admin read-only), 403 keeps the data; a user
// without lms.* gets the portal 403 page without loading the workspace C chunk; every LMS request carries the session
// X-Tenant-Id and no tenant_ids, another tenant's assignment is "Không tìm thấy"; no quest/mini-game configuration
// screen; a turned-off game-activity menu is hidden, 403 FEATURE_DISABLED shows the notice; the course, schedule and
// study-group screens are gone (ADR-0095).
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { WORKSPACE_CATALOG } from '../../../generated/menu-catalog';
import { accountWith, adTruongA, lmsAssignment, problem, renderLms, stopPortals, TENANT_ADMIN_PERMS, TENANTS } from '../../../test/lms-harness';
import { screens } from '../index';

afterEach(() => {
  stopPortals();
  cleanup();
});

describe('Phân quyền và cô lập tenant', () => {
  it('tenant_admin: lists readable, no write button anywhere (F055-AC-19, SC-003)', async () => {
    const a = lmsAssignment({ status: 'closed', result_release: 'manual' });
    await renderLms(`/school-lms/assignments/${a.assignment_id}`, {
      account: accountWith('ad.donvi', TENANTS.thDemoA, TENANT_ADMIN_PERMS),
      setup: (s) => s.assignments.set(String(a.assignment_id), a),
    });
    expect(await screen.findByTestId('submission-table')).toBeInTheDocument();
    expect(await screen.findByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Sửa|Công bố kết quả|Hủy bài|Lưu trữ|Nhắc học sinh|Gia hạn|Miễn/ })).toBeNull();
    fireEvent.click(screen.getByText('← Quay lại'));
    expect(await screen.findByText(a.title as string)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Giao bài' })).toBeNull();
  });

  it('403 PERMISSION_DENIED on a write keeps the data and says why (F055-AC-19)', async () => {
    const a = lmsAssignment({});
    const { backend } = await renderLms(`/school-lms/assignments/${a.assignment_id}`, { setup: (s) => s.assignments.set(String(a.assignment_id), a) });
    backend.overrides.unshift((r) => (r.path.endsWith(':remind') ? problem(403, 'PERMISSION_DENIED') : undefined));
    fireEvent.click(await screen.findByRole('button', { name: 'Nhắc học sinh chưa làm' }));
    expect(await screen.findByText(/Bạn không có quyền thực hiện thao tác này trong phạm vi được giao/)).toBeInTheDocument();
    expect(screen.getByText('Nguyễn An')).toBeInTheDocument();
    expect(screen.getByText(a.title as string)).toBeInTheDocument();
  });

  it('no lms.* permission (Safety Moderator) → portal 403 page, workspace C chunk not loaded (F055-AC-20, TC-CMS-WS-01)', async () => {
    const sm = { ...accountWith('sm.antoan', TENANTS.thDemoA, []), tenants: [], platform: true, otp: '111111', perms: { platform: { workspaces: ['live_ops' as const], permissions: ['moderation.case.read'] } } };
    const { loaders } = await renderLms('/school-lms/assignments', { account: sm });
    expect(await screen.findByText('Không có quyền truy cập')).toBeInTheDocument();
    expect(loaders.school_lms).not.toHaveBeenCalled();
  });

  it('X-Tenant-Id = session tenant, never tenant_ids; another tenant’s assignment → Không tìm thấy (F055-AC-21)', async () => {
    const a = lmsAssignment({});
    const { backend } = await renderLms(`/school-lms/assignments/${a.assignment_id}`, {
      account: accountWith('ad.truong.b', TENANTS.thcsDemoB, adTruongA().perms[TENANTS.thDemoA.tenant_id]!.permissions),
      setup: (s) => s.assignments.set(String(a.assignment_id), a),
    });
    expect(await screen.findByTestId('school-lms-not-found')).toBeInTheDocument();
    const lms = backend.requests.filter((r) => /^\/(lms|exams)\//.test(r.path));
    expect(lms.length).toBeGreaterThan(0);
    expect(lms.every((r) => r.headers['x-tenant-id'] === TENANTS.thcsDemoB.tenant_id && !r.search.includes('tenant_ids'))).toBe(true);
  });

  it('workspace C has no quest / mini-game configuration screen (F055-AC-22)', () => {
    const lmsMenus = WORKSPACE_CATALOG.find((w) => w.key === 'school_lms')!.menus;
    expect(lmsMenus.some((m) => /quest|minigame|game_config/.test(m.key) && m.key !== 'menu.school_lms.game_activities')).toBe(false);
    const games = lmsMenus.find((m) => m.key === 'menu.school_lms.game_activities')!;
    expect(games.actions.map((x) => x.key)).toEqual(['action.school_lms.game_activities.view']);
    expect(screens['menu.school_lms.game_activities'].name).toBe('GameActivitiesScreen');
  });

  it('menu turned off: hidden; 403 FEATURE_DISABLED → notice + permissions reloaded (F055-AC-24)', async () => {
    await renderLms('/school-lms/assignments', { disabledMenus: ['menu.school_lms.game_activities'] });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    await within(nav).findByRole('link', { name: 'Bài tập, đề, bài chẩn đoán' });
    expect(within(nav).queryByRole('link', { name: 'Hoạt động game được gán' })).toBeNull();
    stopPortals();
    cleanup();
    const r2 = await renderLms('/school-lms/assignments');
    const before = r2.backend.calls('GET', '/me/permissions').length;
    r2.backend.overrides.unshift((r) => (r.path === '/lms/catalog/game-activities' ? problem(403, 'FEATURE_DISABLED', { feature_key: 'menu.school_lms.game_activities' }) : undefined));
    fireEvent.click(await screen.findByRole('link', { name: 'Hoạt động game được gán' }));
    expect(await screen.findByText(/Tính năng đang tạm tắt|Khóa tính năng/)).toBeInTheDocument();
    await waitFor(() => expect(r2.backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });

  it('ADR-0095: no course, schedule, study-group or class route or menu in workspace C', async () => {
    const lmsMenus = WORKSPACE_CATALOG.find((w) => w.key === 'school_lms')!.menus.map((m) => m.key);
    for (const k of ['menu.school_lms.courses', 'menu.school_lms.schedules', 'menu.school_lms.study_groups']) expect(lmsMenus).not.toContain(k);
    expect(Object.keys(screens).some((k) => /courses|schedules|study_groups/.test(k))).toBe(false);
    const { backend } = await renderLms('/school-lms/courses');
    expect(await screen.findByText(/Không tìm thấy trang|không tồn tại/i)).toBeInTheDocument();
    expect(backend.requests.some((r) => /\/lms\/(courses|schedules)|study-groups/.test(r.path))).toBe(false);
  });
});

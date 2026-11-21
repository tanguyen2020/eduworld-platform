// F052 T044, T045, T053 — F052-AC-23, F052-AC-28 (TC-CMS-MENU-02): audit list with filters (action, 7-day range),
// `changes` and `trace_id`, no moderation entry; a turned-off menu disappears and its URL shows "Tính năng đang tắt"
// without calling the API; the protected access-control menu has no switch (ADR-0090 mục 2); 403 FEATURE_DISABLED
// from a screen API → notice + permissions reloaded; read-only user without the audit permission has no menu.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import dayjs from 'dayjs';
import yaml from 'js-yaml';
import fs from 'node:fs';
import path from 'node:path';
import { afterEach, describe, expect, it } from 'vitest';
import { WORKSPACE_CATALOG } from '../../../generated/menu-catalog';
import { accountWith, problem, renderSchoolLms, SCHOOL_PERMS, stopPortals, TENANTS } from '../../../test/school-lms-harness';
import { defaultRange } from './AuditLogPage';
import { isModeration } from './useAuditLogs';

afterEach(() => stopPortals());

describe('audit helpers (pure)', () => {
  it('default range is the last 7 days; moderation rows are recognised', () => {
    const [from, to] = defaultRange(dayjs('2026-11-19T10:00:00'));
    expect(to.diff(from, 'day')).toBe(7);
    expect(isModeration({ action: 'moderation.case.opened', target_type: 'x' })).toBe(true);
    expect(isModeration({ action: 'user_role.granted', target_type: 'user' })).toBe(false);
  });
});

describe('Nhật ký (F052-AC-23)', () => {
  it('filters by action and range, shows changes and trace_id, never a moderation entry', async () => {
    const { backend } = await renderSchoolLms('/school-lms/audit-logs');
    expect(await screen.findByText('trace-audit-1')).toBeInTheDocument();
    expect(screen.getByTestId('audit-changes')).toHaveTextContent('academic_manager');
    expect(screen.queryByText('moderation.case.opened')).toBeNull();
    const first = new URLSearchParams(backend.calls('GET', '/tenant/audit-logs')[0]!.search);
    expect(first.get('from')).toBeTruthy();
    expect(first.get('to')).toBeTruthy();
    const action = screen.getByPlaceholderText('Ví dụ user_role.granted');
    fireEvent.change(action, { target: { value: 'user_role.granted' } });
    fireEvent.keyDown(action, { key: 'Enter', code: 'Enter', keyCode: 13 });
    await waitFor(() => expect(backend.calls('GET', '/tenant/audit-logs').some((r) => r.search.includes('action=user_role.granted'))).toBe(true));
  });

  it('no audit menu without tenant.audit.read', async () => {
    await renderSchoolLms('/school-lms/students', { account: accountWith('qlcm.truong.a', TENANTS.thDemoA, SCHOOL_PERMS.filter((p) => p !== 'tenant.audit.read')) });
    const nav = await screen.findByRole('navigation', { name: 'Menu điều hướng' });
    expect(within(nav).getByText('Học sinh theo khối')).toBeInTheDocument();
    expect(within(nav).queryByText('Nhật ký')).toBeNull();
  });
});

describe('menu/action turned off (F052-AC-28, TC-CMS-MENU-02)', () => {
  it('audit menu off: hidden, its URL shows "Tính năng đang tắt" and listTenantAuditLogs is not called', async () => {
    const { backend } = await renderSchoolLms('/school-lms/audit-logs', { disabledMenus: ['menu.school_lms.audit_logs'] });
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Menu điều hướng' });
    expect(within(nav).queryByText('Nhật ký')).toBeNull();
    expect(backend.calls('GET', '/tenant/audit-logs')).toHaveLength(0);
  });

  it('the access-control menu is protected (no switch) in the catalog', () => {
    const yml = fs.readFileSync(path.resolve(__dirname, '../../../../../../../../../specs/portal/menu-catalog.yaml'), 'utf8');
    const catalog = yaml.load(yml) as { workspaces: { key: string; menus: { key: string; can_disable: boolean }[] }[] };
    const menu = catalog.workspaces.find((w) => w.key === 'school_lms')!.menus.find((m) => m.key === 'menu.school_lms.access_control')!;
    expect(menu.can_disable).toBe(false);
    expect(WORKSPACE_CATALOG.some((w) => w.menus.some((m) => m.key === menu.key))).toBe(true);
  });

  it('403 FEATURE_DISABLED from a screen API → notice and permissions reloaded', async () => {
    const { backend } = await renderSchoolLms('/school-lms/settings');
    const before = backend.calls('GET', '/me/permissions').length;
    backend.overrides.unshift((r) => (r.path === '/tenant/settings' ? problem(403, 'FEATURE_DISABLED', { feature_key: 'menu.school_lms.settings' }) : undefined));
    fireEvent.click(await screen.findByRole('tab', { name: 'Cài đặt đơn vị' }));
    expect(await screen.findByText(/Khóa tính năng: menu\.school_lms\.settings/)).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/me/permissions').length).toBeGreaterThan(before));
  });
});

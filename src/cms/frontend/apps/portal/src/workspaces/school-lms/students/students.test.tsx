// F052 T011, T013, T014 — F052-AC-06, F052-AC-07 (SC-004): the login-event view model keeps only whitelisted fields
// (no IP, ip_hash, raw device id even if the API returned them), days without the game; the screen shows enter/leave,
// platform, short device label, end reason; another tenant's student → "Không tìm thấy". Backend = MOCK.
import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { accountWith, IDS, renderSchoolLms, SCHOOL_PERMS, stopPortals, TENANTS } from '../../../test/school-lms-harness';
import { daysSinceLastLogin, LOGIN_EVENT_FIELDS, toLoginEventView, type LoginEvent } from './loginEventView';

afterEach(() => stopPortals());

describe('login event view model (pure)', () => {
  const leaky = {
    login_event_id: 'e1',
    tenant_id: 't',
    student_id: 's',
    platform: 'android',
    device_label: 'Android · Tablet',
    client_version: '1.0.0',
    login_at: '2026-11-10T08:00:00Z',
    logout_at: null,
    end_reason: 'curfew',
    ip: '203.0.113.7',
    ip_hash: 'deadbeef',
    device_id: 'raw-device',
  } as unknown as LoginEvent;

  it('keeps only the whitelisted fields (ADR-0031)', () => {
    const v = toLoginEventView(leaky);
    expect(Object.values(v)).not.toContain('203.0.113.7');
    expect(JSON.stringify(v)).not.toMatch(/deadbeef|raw-device|203\.0\.113/);
    expect(v).toEqual({ id: 'e1', loginAt: '2026-11-10T08:00:00Z', logoutAt: null, platform: 'android', deviceLabel: 'Android · Tablet', clientVersion: '1.0.0', endReason: 'curfew' });
    expect(LOGIN_EVENT_FIELDS).not.toContain('ip' as never);
  });

  it('days since the last login (null when never)', () => {
    const now = Date.parse('2026-11-19T09:00:00Z');
    expect(daysSinceLastLogin([{ loginAt: '2026-11-10T08:00:00Z' }, { loginAt: '2026-11-17T08:00:00Z' }], now)).toBe(2);
    expect(daysSinceLastLogin([], now)).toBeNull();
  });
});

describe('Hoạt động học sinh', () => {
  it('shows login history without IP or raw device id (F052-AC-06, SC-004)', async () => {
    await renderSchoolLms(`/school-lms/student-activity/${IDS.studentAn}`);
    expect(await screen.findByText('Đăng nhập thiết bị khác')).toBeInTheDocument();
    expect(screen.getByText('Windows · PC')).toBeInTheDocument();
    expect(screen.getByText('Số ngày chưa vào game', { selector: '.ant-statistic-title' })).toBeInTheDocument();
    const html = document.body.innerHTML;
    expect(html).not.toContain('203.0.113.7');
    expect(html).not.toContain('deadbeefcafe');
    expect(html).not.toContain('raw-device-123');
  });

  it("another tenant's student → Không tìm thấy (F052-AC-07)", async () => {
    await renderSchoolLms(`/school-lms/student-activity/${IDS.studentAn}`, { account: accountWith('ad.truong.b', TENANTS.thcsDemoB, SCHOOL_PERMS) });
    expect(await screen.findByTestId('school-lms-not-found')).toBeInTheDocument();
  });
});

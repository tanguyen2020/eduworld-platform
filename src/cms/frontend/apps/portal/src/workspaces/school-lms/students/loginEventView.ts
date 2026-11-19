// View model of game login events (F052 T019, F052-AC-06, ADR-0031, data-model §3): ONLY the whitelisted fields are
// kept, so an IP, `ip_hash` or raw device id returned by mistake can never reach the screen (SC-004). Pure functions.
import type { Schemas } from '../common/api';

export type LoginEvent = Schemas['LoginEvent'];

export interface LoginEventView {
  id: string;
  loginAt: string;
  logoutAt: string | null;
  platform: LoginEvent['platform'];
  deviceLabel: string | null;
  clientVersion: string | null;
  endReason: NonNullable<LoginEvent['end_reason']> | null;
}

export const LOGIN_EVENT_FIELDS = ['login_at', 'logout_at', 'platform', 'device_label', 'client_version', 'end_reason'] as const;

export function toLoginEventView(e: LoginEvent): LoginEventView {
  return {
    id: e.login_event_id,
    loginAt: e.login_at,
    logoutAt: e.logout_at ?? null,
    platform: e.platform,
    deviceLabel: e.device_label ?? null,
    clientVersion: e.client_version ?? null,
    endReason: e.end_reason ?? null,
  };
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** Whole days since the most recent login (null when the student never entered the game). */
export function daysSinceLastLogin(events: readonly Pick<LoginEventView, 'loginAt'>[], now: number = Date.now()): number | null {
  const last = events.reduce<number | null>((max, e) => {
    const t = Date.parse(e.loginAt);
    return Number.isNaN(t) ? max : max === null || t > max ? t : max;
  }, null);
  return last === null ? null : Math.max(0, Math.floor((now - last) / DAY_MS));
}

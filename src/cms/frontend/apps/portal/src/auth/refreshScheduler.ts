// refreshScheduler (F051 T059, F051-AC-28, F051-AC-33, ADR-0091 mục 4): the portal TokenProvider of the apiClient.
// - One `refreshPortalToken` call shared by every caller in this tab (single-flight promise);
// - across tabs, refreshes are serialized with the Web Locks API when available, so a tab never sends a refresh
//   cookie another tab is rotating; if a refresh still answers `401 REFRESH_TOKEN_INVALID`/`403 CSRF_TOKEN_INVALID`
//   while the CSRF cookie changed meanwhile (another tab rotated), it re-reads the cookie and retries once;
// - proactive refresh shortly before `expires_in`, only while the user is active (IdleGuard); an idle user is
//   signed out by the IdleGuard instead (F051-AC-29).
// The access token stays in memory (TokenStore); nothing is logged.
import type { TokenProvider } from '../shared/auth/TokenProvider';
import type { AuthApi } from './authApi';
import { readCsrfToken } from './csrf';
import type { TokenStore } from './tokenStore';

export const REFRESH_LOCK = 'ew.portal.refresh';
const RETRY_CODES = new Set(['REFRESH_TOKEN_INVALID', 'CSRF_TOKEN_INVALID']);

export type RefreshOutcome = { ok: true } | { ok: false; status: number; code: string };

interface LockManagerLike {
  request<T>(name: string, callback: () => Promise<T>): Promise<T>;
}

export interface RefreshSchedulerOptions {
  authApi: Pick<AuthApi, 'refresh'>;
  store: TokenStore;
  readCsrf?: () => string | null;
  locks?: LockManagerLike | null;
  /** IdleGuard: true while the user interacted within the idle limit. */
  isActive?: () => boolean;
  /** A proactive refresh failed: the session is over (→ "Phiên làm việc đã hết", F051-AC-07). */
  onExpired?: (outcome: RefreshOutcome) => void;
  now?: () => number;
  setTimer?: (fn: () => void, ms: number) => unknown;
  clearTimer?: (handle: unknown) => void;
}

/** Refresh lead time: 20% of the token lifetime, between 5 and 60 seconds. */
export function refreshLeadMs(lifetimeMs: number): number {
  return Math.min(60_000, Math.max(5_000, lifetimeMs * 0.2));
}

const defaultLocks = (): LockManagerLike | null =>
  typeof navigator !== 'undefined' && 'locks' in navigator && navigator.locks ? (navigator.locks as unknown as LockManagerLike) : null;

export class RefreshScheduler implements TokenProvider {
  private inFlight: Promise<RefreshOutcome> | null = null;
  private timer: unknown = null;
  private readonly readCsrf: () => string | null;
  private readonly locks: LockManagerLike | null;
  private readonly now: () => number;
  private readonly setTimer: (fn: () => void, ms: number) => unknown;
  private readonly clearTimer: (handle: unknown) => void;
  private onExpired: (outcome: RefreshOutcome) => void;

  constructor(private readonly opts: RefreshSchedulerOptions) {
    this.readCsrf = opts.readCsrf ?? (() => readCsrfToken());
    this.locks = opts.locks === undefined ? defaultLocks() : opts.locks;
    this.now = opts.now ?? Date.now;
    this.setTimer = opts.setTimer ?? ((fn, ms) => setTimeout(fn, ms));
    this.clearTimer = opts.clearTimer ?? ((h) => clearTimeout(h as ReturnType<typeof setTimeout>));
    this.onExpired = opts.onExpired ?? (() => undefined);
  }

  setOnExpired(fn: (outcome: RefreshOutcome) => void): void {
    this.onExpired = fn;
  }

  async getAccessToken(): Promise<string | null> {
    return this.opts.store.get()?.token ?? null;
  }

  async refresh(): Promise<boolean> {
    return (await this.refreshDetailed()).ok;
  }

  /** Single-flight refresh; on success the new token is stored and the next proactive refresh scheduled. */
  refreshDetailed(): Promise<RefreshOutcome> {
    this.inFlight ??= this.runLocked()
      .catch((): RefreshOutcome => ({ ok: false, status: 0, code: 'NETWORK_ERROR' }))
      .finally(() => {
        this.inFlight = null;
      });
    return this.inFlight;
  }

  /** Schedules the proactive refresh of the current token (call after login or refresh). */
  schedule(): void {
    this.stop();
    const t = this.opts.store.get();
    if (!t) return;
    const lifetime = t.expiresAt - this.now();
    const wait = Math.max(0, lifetime - refreshLeadMs(lifetime));
    this.timer = this.setTimer(() => {
      this.timer = null;
      if (this.opts.isActive && !this.opts.isActive()) return; // idle: the IdleGuard ends the session
      void this.refreshDetailed().then((r) => {
        if (!r.ok) this.onExpired(r);
      });
    }, wait);
  }

  stop(): void {
    if (this.timer !== null) this.clearTimer(this.timer);
    this.timer = null;
  }

  private runLocked(): Promise<RefreshOutcome> {
    return this.locks ? this.locks.request(REFRESH_LOCK, () => this.run()) : this.run();
  }

  private async run(): Promise<RefreshOutcome> {
    const sent = this.readCsrf();
    let r = await this.opts.authApi.refresh(sent);
    if (!r.ok && RETRY_CODES.has(r.error.code)) {
      const current = this.readCsrf();
      if (current && current !== sent) r = await this.opts.authApi.refresh(current); // another tab rotated
    }
    if (r.ok) {
      this.opts.store.setFromResponse(r.data);
      this.schedule();
      return { ok: true };
    }
    this.stop();
    this.opts.store.clear();
    return { ok: false, status: r.status, code: r.error.code };
  }
}

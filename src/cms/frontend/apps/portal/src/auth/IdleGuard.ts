// IdleGuard (F051 T051, F051-AC-29, ADR-0086 mục 7, `portal_session_sso`): records the last user interaction in
// localStorage `ew.portal.lastActivityAt` (a timestamp, not a secret; shared by tabs) and ends the session when there
// was no interaction for longer than the idle limit — also when a closed tab is reopened later. Storage blocked →
// the timestamp is kept in memory only and the Keycloak client session idle/max still applies (spec edge case).
import { readItem, STORAGE_KEYS, writeItem } from '../shared/storage';

export const ACTIVITY_EVENTS = ['pointerdown', 'keydown', 'wheel', 'touchstart'] as const;
const WRITE_THROTTLE_MS = 5_000;

export interface IdleGuardOptions {
  idleMs: number;
  now?: () => number;
  /** Called once when the idle limit is passed while the guard is running (also settable with setOnIdle). */
  onIdle?: () => void;
  checkEveryMs?: number;
  target?: Pick<Window, 'addEventListener' | 'removeEventListener'>;
}

export class IdleGuard {
  private memory: number | null = null;
  private lastWrite = 0;
  private interval: ReturnType<typeof setInterval> | null = null;
  private readonly now: () => number;
  private readonly onActivity = () => this.touch();
  private onIdle: () => void;

  constructor(private readonly opts: IdleGuardOptions) {
    this.now = opts.now ?? Date.now;
    this.onIdle = opts.onIdle ?? (() => undefined);
  }

  setOnIdle(fn: () => void): void {
    this.onIdle = fn;
  }

  /** Last activity (epoch ms) from storage or memory; null when never recorded. */
  lastActivity(): number | null {
    const raw = readItem('local', STORAGE_KEYS.lastActivityAt);
    const stored = raw ? Date.parse(raw) : NaN;
    const values = [Number.isNaN(stored) ? null : stored, this.memory].filter((v): v is number => v !== null);
    return values.length ? Math.max(...values) : null;
  }

  /** True when an interaction was recorded and it is older than the idle limit (restore must not refresh). */
  isExpired(): boolean {
    const last = this.lastActivity();
    return last !== null && this.now() - last > this.opts.idleMs;
  }

  /** True while the user interacted within the idle limit (or nothing recorded yet). */
  isActive(): boolean {
    return !this.isExpired();
  }

  touch(): void {
    const at = this.now();
    this.memory = at;
    if (at - this.lastWrite >= WRITE_THROTTLE_MS) {
      this.lastWrite = at;
      writeItem('local', STORAGE_KEYS.lastActivityAt, new Date(at).toISOString());
    }
  }

  start(): void {
    if (this.interval) return;
    this.touch();
    const target = this.opts.target ?? window;
    for (const e of ACTIVITY_EVENTS) target.addEventListener(e, this.onActivity, { passive: true });
    this.interval = setInterval(() => {
      if (this.isExpired()) {
        this.stop();
        this.onIdle();
      }
    }, this.opts.checkEveryMs ?? 30_000);
  }

  stop(): void {
    const target = this.opts.target ?? window;
    for (const e of ACTIVITY_EVENTS) target.removeEventListener(e, this.onActivity);
    if (this.interval) clearInterval(this.interval);
    this.interval = null;
  }
}

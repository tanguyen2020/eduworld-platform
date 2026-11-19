// useAutosave (F053-AC-06, F053-AC-07, ADMIN_PORTAL §1.4, research R3): saves a draft when the author pauses
// typing (debounce) and at the latest `intervalMs` (= CONFIG_DEFAULTS `content_autosave_interval`) after the first
// unsaved change; one request at a time; every save carries the last ETag (sent as `If-Match` by the caller);
// `412` / `409 CONTENT_VERSION_CONFLICT` stops autosave for good (state `conflict`) and keeps the local value — the
// author reloads explicitly. A network error keeps the value in memory, shows "Lỗi lưu, thử lại" and retries on the
// next change, when the browser is back online, or on `saveNow()`. No storage: the draft may contain answer keys.
import { useEffect, useMemo, useRef, useSyncExternalStore } from 'react';

export type SaveState = 'idle' | 'saving' | 'saved' | 'error' | 'conflict';

export interface AutosaveSnapshot {
  state: SaveState;
  lastSavedAt: Date | null;
  etag: string | null;
  /** Local changes not yet acknowledged by the server. */
  dirty: boolean;
}

export interface AutosaveSaveResult {
  etag: string | null;
}

export interface AutosaveOptions<T> {
  /** Persists the value with `If-Match: etag`; resolves with the new ETag. */
  save: (value: T, etag: string | null) => Promise<AutosaveSaveResult>;
  /** True for `412` and `409 CONTENT_VERSION_CONFLICT`. */
  isConflict: (err: unknown) => boolean;
  /** Longest wait after the first unsaved change (ms). */
  intervalMs: number;
  /** Pause in typing that triggers a save (ms); default min(2000, intervalMs). */
  debounceMs?: number;
  initialEtag: string | null;
  now?: () => Date;
  setTimer?: (fn: () => void, ms: number) => unknown;
  clearTimer?: (handle: unknown) => void;
}

/** Framework-free controller (unit-tested with fake timers). */
export class AutosaveController<T> {
  private snapshot: AutosaveSnapshot;
  private pending: { value: T } | null = null;
  private inFlight: Promise<void> | null = null;
  private debounceTimer: unknown = null;
  private intervalTimer: unknown = null;
  private enabled = true;
  private readonly listeners = new Set<() => void>();
  private readonly opts: Required<Omit<AutosaveOptions<T>, 'initialEtag'>>;

  constructor(options: AutosaveOptions<T>) {
    this.opts = {
      save: options.save,
      isConflict: options.isConflict,
      intervalMs: options.intervalMs,
      debounceMs: options.debounceMs ?? Math.min(2000, options.intervalMs),
      now: options.now ?? (() => new Date()),
      setTimer: options.setTimer ?? ((fn, ms) => setTimeout(fn, ms)),
      clearTimer: options.clearTimer ?? ((h) => clearTimeout(h as ReturnType<typeof setTimeout>)),
    };
    this.snapshot = { state: 'idle', lastSavedAt: null, etag: options.initialEtag, dirty: false };
  }

  getSnapshot = (): AutosaveSnapshot => this.snapshot;

  subscribe = (listener: () => void): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  private set(patch: Partial<AutosaveSnapshot>): void {
    this.snapshot = { ...this.snapshot, ...patch };
    for (const l of this.listeners) l();
  }

  /** Read-only editor, published version…: changes are kept but never saved. */
  setEnabled(enabled: boolean): void {
    const was = this.enabled;
    this.enabled = enabled;
    if (!enabled) this.clearTimers();
    // re-enabled with unsaved changes (e.g. a blocking issue was fixed): schedule their save
    else if (!was && this.pending) this.change(this.pending.value);
  }

  /** A local change of the draft. */
  change(value: T): void {
    this.pending = { value };
    this.set({ dirty: true });
    if (!this.enabled || this.snapshot.state === 'conflict') return;
    if (this.debounceTimer !== null) this.opts.clearTimer(this.debounceTimer);
    this.debounceTimer = this.opts.setTimer(() => {
      this.debounceTimer = null;
      void this.flush();
    }, this.opts.debounceMs);
    if (this.intervalTimer === null) {
      this.intervalTimer = this.opts.setTimer(() => {
        this.intervalTimer = null;
        void this.flush();
      }, this.opts.intervalMs);
    }
  }

  /** Save now (button "Lưu", retry after an error). Resolves when nothing is left to save or saving failed. */
  saveNow(): Promise<void> {
    return this.flush();
  }

  /** After "Tải bản mới": the server copy replaces the local one; autosave resumes. */
  reset(etag: string | null): void {
    this.clearTimers();
    this.pending = null;
    this.set({ state: 'idle', etag, dirty: false });
  }

  dispose(): void {
    this.clearTimers();
    this.listeners.clear();
  }

  private clearTimers(): void {
    if (this.debounceTimer !== null) this.opts.clearTimer(this.debounceTimer);
    if (this.intervalTimer !== null) this.opts.clearTimer(this.intervalTimer);
    this.debounceTimer = null;
    this.intervalTimer = null;
  }

  private async flush(): Promise<void> {
    if (!this.enabled || this.snapshot.state === 'conflict') return;
    if (this.inFlight) {
      // one request at a time: the change made meanwhile is saved right after
      await this.inFlight;
      return this.flush();
    }
    if (!this.pending) return;
    this.clearTimers();
    const { value } = this.pending;
    this.pending = null;
    this.set({ state: 'saving' });
    this.inFlight = (async () => {
      try {
        const res = await this.opts.save(value, this.snapshot.etag);
        this.set({ state: 'saved', etag: res.etag ?? this.snapshot.etag, lastSavedAt: this.opts.now(), dirty: this.pending !== null });
      } catch (err) {
        this.pending ??= { value }; // keep the local value
        this.set(this.opts.isConflict(err) ? { state: 'conflict', dirty: true } : { state: 'error', dirty: true });
      } finally {
        this.inFlight = null;
      }
    })();
    await this.inFlight;
    if (this.pending && this.snapshot.state === 'saved') return this.flush();
  }
}

export interface UseAutosaveResult extends AutosaveSnapshot {
  saveNow: () => Promise<void>;
  /** Latest snapshot (after an awaited saveNow the rendered values may be stale). */
  current: () => AutosaveSnapshot;
  reset: (etag: string | null) => void;
}

/**
 * React binding: feeds every new `value` (after the first render) to the controller; `enabled = false` for read-only
 * versions. Retries a failed save when the browser comes back online.
 */
export function useAutosave<T>(value: T, enabled: boolean, options: AutosaveOptions<T>): UseAutosaveResult {
  const optsRef = useRef(options);
  optsRef.current = options;
  const controller = useMemo(
    () =>
      new AutosaveController<T>({
        ...options,
        save: (v, etag) => optsRef.current.save(v, etag),
        isConflict: (e) => optsRef.current.isConflict(e),
      }),
    // a new controller only for another document (initial ETag identifies the loaded version)
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );
  const snapshot = useSyncExternalStore(controller.subscribe, controller.getSnapshot);
  const first = useRef(true);

  useEffect(() => controller.setEnabled(enabled), [controller, enabled]);
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    controller.change(value);
  }, [controller, value]);
  useEffect(() => {
    const online = () => {
      if (controller.getSnapshot().state === 'error') void controller.saveNow();
    };
    window.addEventListener('online', online);
    return () => window.removeEventListener('online', online);
  }, [controller]);
  useEffect(() => () => controller.dispose(), [controller]);

  return { ...snapshot, saveNow: controller.saveNow.bind(controller), reset: controller.reset.bind(controller), current: controller.getSnapshot };
}

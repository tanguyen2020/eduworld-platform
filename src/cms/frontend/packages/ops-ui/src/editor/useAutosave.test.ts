// F053 T011 — F053-AC-06, F053-AC-07 (TC-CNT-003): debounce + forced save every content_autosave_interval, one
// request at a time, the last ETag is passed (If-Match), 412/409 CONTENT_VERSION_CONFLICT stops autosave and keeps
// the local value, network errors retry. Timers are fake.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AutosaveController } from './useAutosave';

const conflict = Object.assign(new Error('409'), { code: 'CONTENT_VERSION_CONFLICT' });
const isConflict = (e: unknown) => (e as { code?: string }).code === 'CONTENT_VERSION_CONFLICT' || (e as { status?: number }).status === 412;

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

function make(save: (v: string, etag: string | null) => Promise<{ etag: string | null }>) {
  return new AutosaveController<string>({ save, isConflict, intervalMs: 10_000, debounceMs: 2_000, initialEtag: 'W/"1"' });
}

describe('AutosaveController', () => {
  it('saves after a pause in typing, with the last ETag, then reports "saved"', async () => {
    const save = vi.fn(async (_v: string, etag: string | null) => ({ etag: etag === 'W/"1"' ? 'W/"2"' : 'W/"3"' }));
    const c = make(save);
    c.change('a');
    await vi.advanceTimersByTimeAsync(1_999);
    expect(save).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(save).toHaveBeenCalledWith('a', 'W/"1"');
    expect(c.getSnapshot()).toMatchObject({ state: 'saved', etag: 'W/"2"', dirty: false });
    expect(c.getSnapshot().lastSavedAt).toBeInstanceOf(Date);
    c.change('ab');
    await vi.advanceTimersByTimeAsync(2_000);
    expect(save).toHaveBeenLastCalledWith('ab', 'W/"2"');
  });

  it('typing without pause is still saved every interval (forced save)', async () => {
    const save = vi.fn(async (v: string) => ({ etag: v ? null : null }));
    const c = make(save);
    for (let i = 0; i < 10; i++) {
      c.change(`v${i}`);
      await vi.advanceTimersByTimeAsync(1_000);
    }
    expect(save).toHaveBeenCalledTimes(1);
    expect(save.mock.calls[0]![0]).toBe('v9');
  });

  it('one request at a time: a change during a save is saved right after it', async () => {
    let release!: () => void;
    const save = vi.fn((v: string) => (v === 'first' ? new Promise<{ etag: string }>((r) => (release = () => r({ etag: 'e2' }))) : Promise.resolve({ etag: 'e3' })));
    const c = make(save);
    c.change('first');
    await vi.advanceTimersByTimeAsync(2_000);
    expect(c.getSnapshot().state).toBe('saving');
    c.change('second');
    const flushing = c.saveNow();
    expect(save).toHaveBeenCalledTimes(1);
    release();
    await flushing;
    expect(save).toHaveBeenCalledTimes(2);
    expect(save.mock.calls[1]).toEqual(['second', 'e2']);
    expect(c.getSnapshot()).toMatchObject({ state: 'saved', etag: 'e3', dirty: false });
  });

  it('409 CONTENT_VERSION_CONFLICT / 412: autosave stops, the local value is kept, nothing is overwritten', async () => {
    const save = vi.fn(async () => {
      throw conflict;
    });
    const c = make(save);
    c.change('mine');
    await vi.advanceTimersByTimeAsync(2_000);
    expect(c.getSnapshot()).toMatchObject({ state: 'conflict', dirty: true });
    c.change('mine 2');
    await vi.advanceTimersByTimeAsync(20_000);
    await c.saveNow();
    expect(save).toHaveBeenCalledTimes(1);
    // explicit reload of the server copy resumes autosave with the new ETag
    c.reset('W/"9"');
    expect(c.getSnapshot()).toMatchObject({ state: 'idle', etag: 'W/"9"', dirty: false });

    const c412 = make(vi.fn(async () => Promise.reject(Object.assign(new Error('412'), { status: 412 }))));
    c412.change('x');
    await vi.advanceTimersByTimeAsync(2_000);
    expect(c412.getSnapshot().state).toBe('conflict');
  });

  it('network error: "error" state, value kept and retried on the next attempt', async () => {
    let fail = true;
    const save = vi.fn(async (v: string) => {
      if (fail) throw new TypeError('network');
      return { etag: `e-${v}` };
    });
    const c = make(save);
    c.change('draft');
    await vi.advanceTimersByTimeAsync(2_000);
    expect(c.getSnapshot()).toMatchObject({ state: 'error', dirty: true });
    fail = false;
    await c.saveNow();
    expect(save).toHaveBeenLastCalledWith('draft', 'W/"1"');
    expect(c.getSnapshot()).toMatchObject({ state: 'saved', dirty: false, etag: 'e-draft' });
  });

  it('disabled (read-only version): changes are never saved', async () => {
    const save = vi.fn(async () => ({ etag: null }));
    const c = make(save);
    c.setEnabled(false);
    c.change('x');
    await vi.advanceTimersByTimeAsync(30_000);
    await c.saveNow();
    expect(save).not.toHaveBeenCalled();
  });
});

// F056 T007 — F056-AC-04 (TC-DASH-006): one dashboard page never runs more than `dashboard_max_concurrent_per_user`
// queries at once; the others wait (FIFO) and every widget still gets its own result or error.
import { describe, expect, it } from 'vitest';
import { createDashboardQueue } from './dashboardQueue';

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((a, b) => {
    resolve = a;
    reject = b;
  });
  return { promise, resolve, reject };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe('createDashboardQueue (F056-AC-04)', () => {
  it('runs at most `limit` tasks at once, the rest in FIFO order', async () => {
    const q = createDashboardQueue(4);
    const gates = Array.from({ length: 6 }, () => deferred<number>());
    const started: number[] = [];
    let peak = 0;
    const results = gates.map((g, i) =>
      q.run(() => {
        started.push(i);
        peak = Math.max(peak, q.active);
        return g.promise;
      }),
    );
    expect(started).toEqual([0, 1, 2, 3]);
    expect(q.waiting).toBe(2);
    gates[2]!.resolve(2);
    await results[2];
    await flush();
    expect(started).toEqual([0, 1, 2, 3, 4]);
    gates[0]!.reject(new Error('DASHBOARD_QUERY_TIMEOUT'));
    await expect(results[0]).rejects.toThrow('DASHBOARD_QUERY_TIMEOUT');
    await flush();
    expect(started).toEqual([0, 1, 2, 3, 4, 5]);
    for (const [i, g] of gates.entries()) g.resolve(i);
    expect(await Promise.all(results.slice(1))).toEqual([1, 2, 3, 4, 5]);
    expect(peak).toBe(4);
    await flush();
    expect(q.active).toBe(0);
  });

  it('a task that throws synchronously frees its slot', async () => {
    const q = createDashboardQueue(1);
    await expect(
      q.run(() => {
        throw new Error('boom');
      }),
    ).rejects.toThrow('boom');
    expect(await q.run(async () => 7)).toBe(7);
  });
});

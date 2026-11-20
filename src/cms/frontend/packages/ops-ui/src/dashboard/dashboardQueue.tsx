// Dashboard request queue (F056 T007/T008, F056-AC-04, ADMIN_PORTAL §3.14.1): every widget of a page loads on its own,
// but one page never runs more than `dashboard_max_concurrent_per_user` queries at once — extra queries wait in FIFO
// order. Retrying `429 DASHBOARD_BUSY` belongs to the API client of the app (one retry layer only, 2–5 s, at most twice);
// this queue only limits concurrency. No API knowledge here (ops-ui makes no network call).
import { createContext, useContext, useMemo, type ReactNode } from 'react';

export interface DashboardQueue {
  /** Runs `task` when a slot is free; resolves/rejects with the task. */
  run<T>(task: () => Promise<T>): Promise<T>;
  /** Tasks running now (diagnostics and tests). */
  readonly active: number;
  /** Tasks waiting for a slot. */
  readonly waiting: number;
}

/** A FIFO semaphore of `limit` slots (limit ≥ 1). */
export function createDashboardQueue(limit: number): DashboardQueue {
  const max = Math.max(1, Math.floor(limit));
  let active = 0;
  const waiting: (() => void)[] = [];
  const release = () => {
    active--;
    const next = waiting.shift();
    if (next) next();
  };
  return {
    get active() {
      return active;
    },
    get waiting() {
      return waiting.length;
    },
    run<T>(task: () => Promise<T>): Promise<T> {
      return new Promise<T>((resolve, reject) => {
        const start = () => {
          active++;
          let p: Promise<T>;
          try {
            p = task();
          } catch (e) {
            p = Promise.reject(e);
          }
          p.then(resolve, reject).finally(release);
        };
        if (active < max) start();
        else waiting.push(start);
      });
    },
  };
}

const passThrough: DashboardQueue = { run: (task) => task(), active: 0, waiting: 0 };
const QueueContext = createContext<DashboardQueue>(passThrough);

/** One queue per dashboard page (all widgets of the page share it). */
export const DashboardQueueProvider = ({ limit, children }: { limit: number; children: ReactNode }) => {
  const queue = useMemo(() => createDashboardQueue(limit), [limit]);
  return <QueueContext.Provider value={queue}>{children}</QueueContext.Provider>;
};

/** The queue of the enclosing page; without a provider, tasks run immediately. */
export function useDashboardQueue(): DashboardQueue {
  return useContext(QueueContext);
}

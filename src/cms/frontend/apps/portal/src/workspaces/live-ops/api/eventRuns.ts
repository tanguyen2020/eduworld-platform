// Giám sát sự kiện (F080 T040, F080-AC-19, ADMIN_PORTAL §3.15): listEventRuns (platform runs, newest first, cursor
// pages — "Tải thêm", never the whole list), getEventRunStats (participants, community goal, claims, rewards) and the
// emergency operations of F077 suspendEventRun / resumeEventRun / cancelEventRun with a mandatory reason and one
// Idempotency-Key per user intent (useIdempotentMutation, AGENTS R10). The server decides whether a transition is
// allowed (409 → the screen reloads the run); nothing is retried automatically.
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { liveOpsKey, useLiveOpsApi, type Page, type Schemas } from './common';

export type EventRunSummary = Schemas['EventRunSummary'];
export type EventRunStatus = Schemas['EventRunStatus'];
export type EventRunStats = Schemas['EventRunStats'];

/** Rows per page of the run table (≤ Limit.maximum). */
export const RUN_PAGE_SIZE = 20;
/** Statuses the monitor lists by default (F080 plan: live + suspended). */
export const DEFAULT_RUN_STATUSES: EventRunStatus[] = ['live', 'suspended'];
export const RUN_STATUSES: EventRunStatus[] = ['scheduled', 'live', 'suspended', 'ended', 'cancelled', 'archived'];

export type RunOperation = 'suspend' | 'resume' | 'cancel';

/** Operations the run status allows to offer (display only — the server answers 409 otherwise). */
export function operationsFor(status: EventRunStatus): RunOperation[] {
  if (status === 'live') return ['suspend', 'cancel'];
  if (status === 'suspended') return ['resume', 'cancel'];
  if (status === 'scheduled') return ['cancel'];
  return [];
}

export function useEventRuns(statuses: EventRunStatus[]) {
  const api = useLiveOpsApi();
  const status = statuses.length ? statuses : DEFAULT_RUN_STATUSES;
  return useCursorPages<EventRunSummary, Page<EventRunSummary>>(liveOpsKey('event-runs', status), (cursor) =>
    api.call<Page<EventRunSummary>>(
      api.client.GET('/game-admin/event-runs', { params: { query: { status, limit: RUN_PAGE_SIZE, ...(cursor ? { cursor } : {}) } } }),
    ),
  );
}

export function useEventRunStats(run: Pick<EventRunSummary, 'event_id' | 'event_run_id'> | null) {
  const api = useLiveOpsApi();
  return useQuery({
    queryKey: liveOpsKey('event-run-stats', run?.event_id, run?.event_run_id),
    queryFn: () =>
      api.call<EventRunStats>(
        api.client.GET('/game-admin/events/{event_id}/runs/{run_id}/stats', { params: { path: { event_id: run!.event_id, run_id: run!.event_run_id } } }),
      ),
    enabled: !!run,
  });
}

const PATHS = {
  suspend: '/game-admin/events/{event_id}/runs/{run_id}:suspend',
  resume: '/game-admin/events/{event_id}/runs/{run_id}:resume',
  cancel: '/game-admin/events/{event_id}/runs/{run_id}:cancel',
} as const;

export interface RunOperationInput {
  op: RunOperation;
  reason: string;
}

/** One emergency operation on a run (Idempotency-Key per intent); the runs and the stats reload when it settles (409 too). */
export function useRunOperation(run: Pick<EventRunSummary, 'event_id' | 'event_run_id'>) {
  const api = useLiveOpsApi();
  const qc = useQueryClient();
  return useIdempotentMutation<unknown, RunOperationInput>(
    ({ op, reason }, intent) =>
      api.call<unknown>(
        api.client.POST(PATHS[op], { params: { path: { event_id: run.event_id, run_id: run.event_run_id } }, body: { reason }, headers: { ...intent } } as never),
      ),
    {
      onSettled: () => {
        void qc.invalidateQueries({ queryKey: ['live-ops', 'event-runs'] });
        void qc.invalidateQueries({ queryKey: liveOpsKey('event-run-stats', run.event_id, run.event_run_id) });
      },
    },
  );
}

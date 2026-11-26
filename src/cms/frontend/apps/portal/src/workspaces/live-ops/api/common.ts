// Shared plumbing of the workspace E screens of F080 (Live dashboard, Giám sát sự kiện, Game Analytics): the typed
// apiClient of the portal runtime, query keys `['live-ops', <name>, …]` and the generated contract types. No business
// rule here: every number comes from module game (AGENTS R1, ADR-0062).
import { usePortalRuntime } from '../../../app/runtime';
import type { components } from '../../../shared/api/schema.gen';

export type Schemas = components['schemas'];
export type PageInfo = Schemas['PageInfo'];

export interface Page<T> {
  data: T[];
  page: PageInfo;
}

export const liveOpsKey = (name: string, ...rest: unknown[]) => ['live-ops', name, ...rest] as const;

export function useLiveOpsApi() {
  return usePortalRuntime().api;
}

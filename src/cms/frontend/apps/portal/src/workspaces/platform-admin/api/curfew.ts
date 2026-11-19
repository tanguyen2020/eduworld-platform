// Platform curfew hooks (F057 T052, F057-AC-23, ADR-0078): getPlatformCurfewSettings (ETag; `timezone`,
// `warning_minutes`, `is_default` read-only) and putPlatformCurfewSettings with If-Match and a mandatory reason. Only
// `start`, `end`, `reason` are sent (`additionalProperties: false`). Helpers below are display helpers (HH:MM shape and
// "đến … hôm sau"), the server validates the values.
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { LIMITS } from '../limits';
import { usePlatformApi, type Schemas } from './common';

export type PlatformCurfewSettings = Schemas['PlatformCurfewSettings'];
export interface CurfewInput {
  start: string;
  end: string;
  reason: string;
}

const KEY = ['platform', 'settings', 'curfew'];
const HHMM = new RegExp(LIMITS.curfewTime.pattern);

export const isHHMM = (v: string): boolean => HHMM.test(v);

/** A window whose end is not after its start ends the next day (e.g. 21:00 → 06:30). */
export const crossesMidnight = (start: string, end: string): boolean => isHHMM(start) && isHHMM(end) && end <= start;

export function usePlatformCurfew() {
  const api = usePlatformApi();
  return useQuery({ queryKey: KEY, queryFn: () => api.call(api.client.GET('/admin/settings/curfew', {})) });
}

export function usePutPlatformCurfew() {
  const api = usePlatformApi();
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: CurfewInput) => api.call(api.client.PUT('/admin/settings/curfew', { body: body as PlatformCurfewSettings })),
    onSuccess: (data) => qc.setQueryData(KEY, data),
  });
}

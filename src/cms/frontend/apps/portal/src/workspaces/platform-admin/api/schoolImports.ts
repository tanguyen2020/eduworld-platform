// School import hooks (F057 T032, F057-AC-13, FR-ORG-016): importSchools (multipart CSV, `dry_run`, X-Tenant-Id =
// target tenant, Idempotency-Key) then getSchoolImport polled until a final status; errors per row come from the server.
import { useQuery } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { targetTenantHeaders } from '../../../shared/api/withTenantScope';
import { JOB_POLL_INTERVAL_MS } from '../limits';
import { isFinalJob, usePlatformApi, type Schemas } from './common';

export type SchoolImportJob = Schemas['SchoolImportJob'];

export interface ImportVars {
  tenantId: string;
  dryRun: boolean;
  file: File;
  /** Distinguishes intents (a File serialises to `{}`): same file + same mode = same Idempotency-Key on retry. */
  fileKey: string;
}

export const fileKeyOf = (f: File) => `${f.name}:${f.size}:${f.lastModified}`;

export function useImportSchools() {
  const api = usePlatformApi();
  return useIdempotentMutation(({ tenantId, dryRun, file }: ImportVars, headers) => {
    const form = new FormData();
    form.append('file', file);
    form.append('dry_run', String(dryRun));
    return api.call<SchoolImportJob>(
      api.client.POST('/admin/schools:import', {
        params: { header: targetTenantHeaders(tenantId) as { 'X-Tenant-Id': string } },
        body: form as never,
        bodySerializer: (b: unknown) => b as FormData,
        headers,
      }),
    );
  });
}

export function useSchoolImport(importId: string | undefined) {
  const api = usePlatformApi();
  return useQuery({
    queryKey: ['platform', 'school-import', importId],
    queryFn: () => api.call(api.client.GET('/admin/school-imports/{import_id}', { params: { path: { import_id: importId! } } })),
    enabled: !!importId,
    refetchInterval: (q) => (q.state.status === 'error' || isFinalJob(q.state.data?.status) ? false : JOB_POLL_INTERVAL_MS),
  });
}

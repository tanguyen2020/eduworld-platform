// Read hooks of "Tải bộ cài" (F134 T035, F134-AC-17, ADMIN_PORTAL §10): listSchoolClientInstallers (module game) for one
// school of the user's scope — the server loads the signed `production` index, checks its signature and filters it by the
// grades of the school (F134-AC-13/14); the portal only shows the answer. Query keys carry the tenant of the data; the
// System Admin's chosen tenant goes in X-Tenant-Id (scope.headers). Schools for the picker: every cursor page of
// listTenantSchools (shared with "Hoạt động game", never one page as the whole list). No business rule here (AGENTS R1).
import { useQuery } from '@tanstack/react-query';
import { qk, useApi, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type SchoolClientInstaller = Schemas['SchoolClientInstaller'];
export type SchoolClientInstallerList = Schemas['SchoolClientInstallerList'];

export { useScopeSchools } from '../game-report/api/tenantGameSummary';

/** listSchoolClientInstallers of one school; runs only with a school and while the view action is enabled. */
export function useSchoolInstallers(schoolId: string | undefined, enabled = true) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useQuery({
    queryKey: qk('installers', scope, schoolId ?? null),
    queryFn: () =>
      api.call<SchoolClientInstallerList>(
        api.client.GET('/tenant/client-installers', { params: { query: { school_id: schoolId! } }, headers: scope.headers }),
      ),
    enabled: enabled && scope.ready && !!schoolId,
    staleTime: 5 * 60_000,
  });
}

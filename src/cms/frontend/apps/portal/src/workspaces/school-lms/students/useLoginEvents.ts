// Game login history of one student (F052 T019, F052-AC-06): listTenantStudentLoginEvents → whitelisted view model.
import { PAGE_SIZE, qk, useApi, usePagedList } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';
import { toLoginEventView, type LoginEventView } from './loginEventView';

export function useLoginEvents(studentId: string | undefined, range: { from?: string; to?: string } = {}) {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return usePagedList<LoginEventView>(
    qk('login-events', scope, studentId, range.from ?? '', range.to ?? ''),
    async (cursor) => {
      const res = await api.call(
        api.client.GET('/tenant/students/{student_id}/login-events', {
          params: {
            path: { student_id: studentId! },
            query: { limit: PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(range.from ? { from: range.from } : {}), ...(range.to ? { to: range.to } : {}) },
          },
          headers: scope.headers,
        }),
      );
      return { items: res.data.map(toLoginEventView), page: res.page };
    },
    scope.ready && !!studentId,
  );
}

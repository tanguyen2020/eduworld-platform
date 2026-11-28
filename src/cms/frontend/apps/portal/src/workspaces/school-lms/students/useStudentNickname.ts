// Nickname reset of a student (F136 T033, F136-AC-22): resetStudentNickname with X-Tenant-Id and one Idempotency-Key per
// confirmation, reused on retry (useIdempotentMutation). The rule (who may reset, what happens to the profile) stays in
// module game (AGENTS R1, R7).
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useApi } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export function useResetStudentNickname() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useIdempotentMutation((v: { studentId: string; reason: string }, intent) =>
    api.call(
      api.client.POST('/tenant/students/{student_id}/game-profile/nickname:reset', {
        params: { path: { student_id: v.studentId } },
        body: { reason: v.reason },
        headers: { ...scope.headers, ...intent },
      }),
    ),
  );
}

// Password reset of a student (F052 T059, F052-AC-31): resetStudentPassword with one Idempotency-Key per confirmation,
// reused on retry (useIdempotentMutation). Moved from the removed class screen (ADR-0095); no class involved.
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { useApi, type Schemas } from '../common/api';
import { useSchoolLmsScope } from '../common/scope';

export type PasswordResetResult = Schemas['StudentPasswordResetResult'];

export function useResetStudentPassword() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  return useIdempotentMutation<PasswordResetResult, { studentId: string; reason: string }>((v, intent) =>
    api.call(
      api.client.POST('/tenant/students/{student_id}/password:reset', {
        params: { path: { student_id: v.studentId } },
        body: { reason: v.reason },
        headers: { ...scope.headers, ...intent },
      }),
    ),
  );
}

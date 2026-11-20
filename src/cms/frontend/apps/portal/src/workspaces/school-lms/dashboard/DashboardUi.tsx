// Shared pieces of the F056 screens: dashboard base path from the catalog (ADR-0090), errors of dashboard reads
// (403 PERMISSION_DENIED → "Ngoài phạm vi quyền" without any data — F056-AC-19; 404 → "Không tìm thấy" without telling
// whether the resource exists — F056-AC-21; DASHBOARD_* → the widget hints of ops-ui — F056-AC-04), the attention
// reason tags (neutral wording, F056-AC-05) and links to the F055 wizard with prefilled parameters (F056-AC-10).
import type { DashboardWidgetError } from '@eduworld/ops-ui/dashboard';
import { Result, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { menuByKey } from '../../../permissions/menuRegistry';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { NotFoundResult } from '../common/errors';
import type { AttentionReason } from './api/dashboardApi';

/** Absolute paths of the menus F056 links between (from the generated catalog, never typed by hand). */
export const DASHBOARD_BASE = menuByKey('menu.school_lms.school_dashboard')!.path;
export const RESULTS_BASE = menuByKey('menu.school_lms.results')!.path;
export const ASSIGNMENTS_BASE = menuByKey('menu.school_lms.assignments')!.path;
export const SAFETY_BASE = menuByKey('menu.school_lms.safety_notifications')!.path;

export const isOutOfScope = (e: unknown) => e instanceof ApiError && e.status === 403 && e.code !== 'FEATURE_DISABLED';
export const isNotFound = (e: unknown) => e instanceof ApiError && e.status === 404;

/** Widget error of a dashboard read (ops-ui shows the hint of DASHBOARD_* codes itself). */
export function useWidgetError(error: unknown): DashboardWidgetError | null {
  const { t } = useTranslation();
  if (!error) return null;
  if (!(error instanceof ApiError)) return { code: 'UNKNOWN', message: t('error.generic') };
  if (isOutOfScope(error)) return { code: error.code, message: t('dashboard.outOfScope') };
  if (isNotFound(error)) return { code: error.code, message: t('dashboard.notFound') };
  return {
    code: error.code,
    message: (
      <>
        {t(error.i18nKey)}
        {error.traceId ? (
          <Typography.Text type="secondary" copyable={{ text: error.traceId }}>
            {' '}
            {t('shell.problem.traceId', { traceId: error.traceId })}
          </Typography.Text>
        ) : null}
      </>
    ),
  };
}

/** Whole-page error of a drill-down (class, student): out of scope / not found never show data. */
export function DashboardPageError({ error }: { error: unknown }) {
  const { t } = useTranslation();
  if (isNotFound(error)) return <NotFoundResult />;
  if (isOutOfScope(error))
    return (
      <div data-testid="dashboard-out-of-scope">
        <Result status="403" title={t('dashboard.outOfScope')} />
      </div>
    );
  return <ProblemMessage error={error} />;
}

/** Neutral label of an attention reason (F056-AC-05): "cần hỗ trợ", never "kém"; rapid_guess → "làm quá nhanh". */
export function ReasonTag({ reason }: { reason: AttentionReason }) {
  const { t } = useTranslation();
  return <Tag data-testid={`reason-${reason}`}>{t(`dashboard.reason.${reason}`)}</Tag>;
}

/** Link of the F055 wizard prefilled for a practice on one LO (`?mode=practice&objective_id=&class_id=&student_ids=`). */
export function practiceWizardHref(p: { objectiveId?: string; classId?: string; studentIds?: string[] }): string {
  const q = new URLSearchParams({ mode: 'practice' });
  if (p.objectiveId) q.set('objective_id', p.objectiveId);
  if (p.classId) q.set('class_id', p.classId);
  if (p.studentIds?.length) q.set('student_ids', p.studentIds.join(','));
  return `${ASSIGNMENTS_BASE}/new?${q.toString()}`;
}

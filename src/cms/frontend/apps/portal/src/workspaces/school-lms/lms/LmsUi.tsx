// Presentational helpers of the F055 screens: translated Problem (own LMS message by `code` + trace id, never the raw
// `detail` — F055-AC-06), permission / catalog-action checks for conditional UI (display only: the backend authorizes
// every call, ADR-0027 mục 3), status tags and date formatting. No data logic (AGENTS R1).
import { Alert, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import type { ActionKey } from '../../../generated/menu-catalog';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { ApiError } from '../../../shared/api/middleware/problem';
import type { components } from '../../../shared/api/schema.gen';
import { lmsErrorKey } from './model/problemMessages';

export function LmsProblem({ error, testId = 'lms-problem' }: { error: unknown; testId?: string }) {
  const { t } = useTranslation();
  if (!error) return null;
  const traceId = error instanceof ApiError ? error.traceId : undefined;
  return (
    <Alert
      type="error"
      showIcon
      role="alert"
      data-testid={testId}
      message={t(lmsErrorKey(error))}
      description={
        traceId ? (
          <Typography.Text type="secondary" copyable={{ text: traceId }}>
            {t('shell.problem.traceId', { traceId })}
          </Typography.Text>
        ) : undefined
      }
    />
  );
}

/** True when the session holds the permission (conditional rendering only). */
export function useHasPermission(code: string): boolean {
  const { view } = usePermissions();
  return !!view?.permissions.includes(code);
}

/** True when a catalog action is granted and not turned off (ADR-0090). */
export function useActionEnabled(key: ActionKey): boolean {
  const { view } = usePermissions();
  return !!view && actionStatus(key, view) === 'enabled';
}

type Target = components['schemas']['AssignmentTarget'];

/** Targets of an assignment as text (ADR-0095): "Khối 3, Khối 4 · thêm 2 · bớt 1" or "Học sinh cụ thể: 5". */
export function TargetsText({ targets }: { targets: readonly Target[] | undefined }) {
  const { t } = useTranslation();
  const list = targets ?? [];
  const grades = list.filter((x) => x.target_type === 'grade' && x.grade_level).map((x) => t('lms.wizard.gradeN', { n: x.grade_level }));
  const include = new Set(list.flatMap((x) => x.include_student_ids ?? [])).size;
  const exclude = new Set(list.flatMap((x) => x.exclude_student_ids ?? [])).size;
  const students = list.filter((x) => x.target_type === 'students').reduce((n, x) => n + (x.student_count ?? x.student_ids?.length ?? 0), 0);
  const parts = [
    grades.join(', '),
    include ? t('lms.wizard.includeCount', { n: include }) : '',
    exclude ? t('lms.wizard.excludeCount', { n: exclude }) : '',
    list.some((x) => x.target_type === 'students') ? t('lms.wizard.targetCount', { type: t('lms.targetMode.students'), n: students }) : '',
  ];
  return <Typography.Text data-testid="assignment-targets">{parts.filter(Boolean).join(' · ')}</Typography.Text>;
}

export function LmsTag({ group, value }: { group: string; value: string | undefined }) {
  const { t } = useTranslation();
  if (!value) return null;
  return <Tag data-testid={`tag-${group}`}>{t(`lms.${group}.${value}`)}</Tag>;
}

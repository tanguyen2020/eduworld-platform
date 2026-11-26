// Presentational helpers of the challenge screens (F108 US3): translated Problem (challenge-specific message by `code`,
// then the LMS / shared message, plus the trace id — never the raw `detail`, F108-AC-14), status tags and the grade
// text. No data logic (AGENTS R1).
import { Alert, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { lmsErrorKey } from '../lms/model/problemMessages';

/** Codes with a challenge-specific message (lms.challenges.errors.<CODE>). */
const OWN_CODES = ['RESOURCE_NOT_ASSIGNED', 'INVALID_STATE_TRANSITION', 'CONFLICT'];

export function challengeErrorKey(error: unknown): string {
  return error instanceof ApiError && OWN_CODES.includes(error.code) ? `lms.challenges.errors.${error.code}` : lmsErrorKey(error);
}

export function ChallengeProblem({ error, testId = 'challenge-problem' }: { error: unknown; testId?: string }) {
  const { t } = useTranslation();
  if (!error) return null;
  const traceId = error instanceof ApiError ? error.traceId : undefined;
  return (
    <Alert
      type="error"
      showIcon
      role="alert"
      data-testid={testId}
      message={t(challengeErrorKey(error))}
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

/** Tag of an enum value translated under `lms.challenges.<group>.<value>`. */
export function EnumTag({ group, value, testId }: { group: string; value: string | null | undefined; testId?: string }) {
  const { t } = useTranslation();
  if (!value) return null;
  return <Tag data-testid={testId}>{t(`lms.challenges.${group}.${value}`)}</Tag>;
}

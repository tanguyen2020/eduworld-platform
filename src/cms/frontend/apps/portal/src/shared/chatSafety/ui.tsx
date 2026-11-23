// Shared display pieces of the chat-safety screens (F122, workspaces E, D, C): error display by Problem `code` with the
// trace id (403 → the 403 page with trace id, no MFA step-up — ADR-0094; internal gateway unreachable → "Cần kết nối mạng
// quản trị (VPN)" — F122-AC-25), the Tenant column (names from the moderation tenants, F122 T064), reason limits of the operations. No rule here.
import { Result, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../api/middleware/problem';
import { ProblemMessage } from '../../shell/ProblemMessage';

/** Limits of the free-text fields of the chat-safety operations (specs/openapi/eduworld-api.v1.yaml). */
export const LIMITS = {
  caseNote: { source: 'schemas/ModerationActionRequest/properties/note', minLength: 3, maxLength: 1000 },
  caseSearch: { source: 'paths/listModerationCaseMessages/q', maxLength: 200 },
  category: { source: 'paths/listModerationCases/category_code', maxLength: 64 },
  sanctionReason: { source: 'schemas/SanctionCreate/properties/reason', minLength: 3, maxLength: 1000 },
  revokeReason: { source: 'schemas/ReasonRequest/properties/reason', minLength: 1, maxLength: 500 },
  notificationSummary: { source: 'schemas/SchoolSafetyNotificationCreate/properties/summary', minLength: 10, maxLength: 1000 },
  ackNote: { source: 'schemas/SchoolSafetyNotificationAck/properties/note', maxLength: 500 },
  term: { source: 'schemas/SensitiveTermInput/properties/term', minLength: 1, maxLength: 200 },
  termNote: { source: 'schemas/SensitiveTermInput/properties/note', maxLength: 500 },
  categoryName: { source: 'schemas/SensitiveTermCategoryInput/properties/name', maxLength: 120 },
  categoryDescription: { source: 'schemas/SensitiveTermCategoryInput/properties/description', maxLength: 1000 },
  categoryCode: { source: 'schemas/SensitiveTermCategoryInput/properties/code', maxLength: 64, pattern: '^[a-z][a-z0-9_]{1,40}$' },
  allowPhrase: { source: 'schemas/SensitiveAllowlistInput/properties/phrase', minLength: 1, maxLength: 200 },
  allowReason: { source: 'schemas/SensitiveAllowlistInput/properties/reason', maxLength: 500 },
  testText: { source: 'schemas/SensitiveTermTestRequest/properties/text', minLength: 1, maxLength: 500 },
  changeSummary: { source: 'schemas/SensitiveCatalogPublishRequest/properties/change_summary', minLength: 5, maxLength: 1000 },
  toggleReason: { source: 'paths/toggleTenantSensitiveCategory/body/reason', minLength: 10, maxLength: 500 },
  /** maxLength from the schema; ≥ 10 is the server rule MODERATION_SWITCH_REASON_REQUIRED (description of the field). */
  switchReason: { source: 'schemas/ModerationSwitchUpdate/properties/reason', minLength: 10, maxLength: 1000 },
  rejectReason: { source: 'paths/rejectModerationSwitchChange/body/reason', minLength: 1, maxLength: 500 },
  clubRemoveReason: { source: 'paths/removeSchoolClubMember/body/reason', minLength: 1, maxLength: 500 },
  policyReason: { source: 'schemas/ChatPolicyPut/properties/reason', minLength: 10, maxLength: 500 },
  userId: { source: 'schemas/Uuid', maxLength: 36 },
} as const;

/** Max students of one school notification (SchoolSafetyNotificationCreate.student_ids.maxItems). */
export const MAX_NOTIFIED_STUDENTS = 20;

/** Error of a chat-safety read or write: 403 PERMISSION_DENIED → 403 page with trace id; anything else → the Problem. */
export function SafetyError({ error }: { error: unknown }) {
  const { t } = useTranslation();
  if (error instanceof ApiError && error.status === 403 && error.code === 'PERMISSION_DENIED') {
    return (
      <Result
        status="403"
        title={t('shell.forbidden.title')}
        subTitle={t('shell.forbidden.description')}
        extra={error.traceId ? <Typography.Text type="secondary" copyable={{ text: error.traceId }}>{t('shell.problem.traceId', { traceId: error.traceId })}</Typography.Text> : null}
      />
    );
  }
  return <ProblemMessage error={error} />;
}

export function TenantName({ id, names }: { id?: string | null; names: ReadonlyMap<string, { name: string; code: string }> }) {
  if (!id) return null;
  const x = names.get(id);
  return x ? <span>{x.name}</span> : <Typography.Text code>{id}</Typography.Text>;
}

export function SeverityTag({ value }: { value?: string | null }) {
  const { t } = useTranslation();
  if (!value) return null;
  return <Tag color={value === 'p1' ? 'error' : value === 'p2' ? 'warning' : 'default'}>{t(`chatSafety.severity.${value}`)}</Tag>;
}

/** Short display of an id (the portal never shows a student's real name outside the school notification — R16). */
export const shortId = (id?: string | null): string => (id ? id.slice(-8) : '');

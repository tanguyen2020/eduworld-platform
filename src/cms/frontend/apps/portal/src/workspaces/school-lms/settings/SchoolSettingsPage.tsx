// School settings (F052 T042, F052-AC-22, F052-AC-29, F052-AC-30): access hours, class/school leaderboard,
// `student_club_creation` — updateSchoolSettings (JSON Merge Patch of changed fields, If-Match) with errors at the
// field — and the curfew card. A school of the `common` tenant (virtual school) shows only the platform curfew.
import { App, Button, Card, Form, Space, Spin, Switch } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { ApiError } from '../../../shared/api/middleware/problem';
import { applyFormError, QueryError, writeErrorKey } from '../common/errors';
import { SchoolSelect } from '../common/SchoolSelect';
import { useSchoolList } from '../schools/useSchools';
import { AllowedHoursField } from './AllowedHoursField';
import { CurfewSettingsCard } from './CurfewSettingsCard';
import { useSchoolSettings, useUpdateSchoolSettings, type SchoolSettings } from './useSettings';

type PolicyForm = Pick<SchoolSettings, 'allowed_hours' | 'class_leaderboard_enabled' | 'school_leaderboard_enabled' | 'student_club_creation'>;
const POLICY_FIELDS = ['allowed_hours', 'class_leaderboard_enabled', 'school_leaderboard_enabled', 'student_club_creation'] as const;

export function policyPatch(before: SchoolSettings, after: PolicyForm): Partial<SchoolSettings> {
  const patch: Record<string, unknown> = {};
  for (const f of POLICY_FIELDS) if (JSON.stringify(after[f] ?? null) !== JSON.stringify(before[f] ?? null)) patch[f] = after[f] ?? null;
  return patch as Partial<SchoolSettings>;
}

function SchoolPolicy({ schoolId, settings }: { schoolId: string; settings: SchoolSettings }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const { view } = usePermissions();
  const canEdit = !!view && actionStatus('action.school_lms.settings.edit_school', view) === 'enabled';
  const [form] = Form.useForm<PolicyForm>();
  const update = useUpdateSchoolSettings(schoolId);
  const submit = async (v: PolicyForm) => {
    const patch = policyPatch(settings, v);
    if (!Object.keys(patch).length) return;
    try {
      await update.mutateAsync(patch);
      message.success(t('portal.school_lms.common.saved'));
    } catch (e) {
      if (!applyFormError(form, e, [...POLICY_FIELDS], t, i18n.exists.bind(i18n))) message.error(t(writeErrorKey(e)));
    }
  };
  return (
    <Card title={t('portal.school_lms.settings.schoolPolicy')} data-testid="school-policy">
      <Form<PolicyForm> form={form} layout="vertical" initialValues={settings} onFinish={submit} disabled={!canEdit}>
        <Form.Item label={t('portal.school_lms.settings.allowedHours')}>
          <AllowedHoursField disabled={!canEdit} />
        </Form.Item>
        <Form.Item name="class_leaderboard_enabled" label={t('portal.school_lms.settings.classLeaderboard')} valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="school_leaderboard_enabled" label={t('portal.school_lms.settings.schoolLeaderboard')} valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="student_club_creation" label={t('portal.school_lms.settings.clubCreation')} valuePropName="checked">
          <Switch />
        </Form.Item>
        <ActionGate actionKey="action.school_lms.settings.edit_school">
          <Button type="primary" htmlType="submit" loading={update.isPending}>
            {t('portal.school_lms.common.save')}
          </Button>
        </ActionGate>
      </Form>
    </Card>
  );
}

export function SchoolSettingsPage() {
  const { t } = useTranslation();
  const [schoolId, setSchoolId] = useState<string | undefined>();
  const schools = useSchoolList();
  const q = useSchoolSettings(schoolId);
  const school = schools.items.find((s) => s.school_id === schoolId);
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Form.Item label={t('portal.school_lms.common.school')}>
        <SchoolSelect value={schoolId} onChange={setSchoolId} />
      </Form.Item>
      {q.error ? <QueryError error={q.error} resource={!(q.error instanceof ApiError && q.error.code === 'FEATURE_DISABLED')} /> : null}
      {schoolId && !q.data && !q.error ? <Spin aria-label={t('app.loading')} /> : null}
      {schoolId && q.data ? (
        <>
          <CurfewSettingsCard key={`${schoolId}-${q.dataUpdatedAt}`} schoolId={schoolId} settings={q.data} isCommon={!!school?.is_virtual} onReload={() => void q.refetch()} />
          <SchoolPolicy key={`p-${schoolId}-${q.dataUpdatedAt}`} schoolId={schoolId} settings={q.data} />
        </>
      ) : null}
    </Space>
  );
}

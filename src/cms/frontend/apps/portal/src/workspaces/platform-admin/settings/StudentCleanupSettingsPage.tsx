// Inactive-student cleanup settings (F057 T037, F057-AC-14, ADR-0032, ADMIN_PORTAL §3.10): `default_days` (switch off =
// null, cleanup disabled), `notice_days`, `allow_tenant_override`, `approval_threshold_percent` within the schema
// bounds, mandatory reason (3–500); warning under CONFIG_DEFAULTS `inactive_student_cleanup_low_days_warning` (still
// saved with the reason). PUT with If-Match (ETag of the GET); `422` field errors land on their field.
import { Alert, App as AntApp, Button, Card, Form, InputNumber, Spin, Switch } from 'antd';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useCleanupSettings, usePutCleanupSettings } from '../api/cleanup';
import { CLEANUP_LOW_DAYS_WARNING, LIMITS } from '../limits';
import { applyFieldErrors, ReasonField } from '../ui';

interface Values {
  enabled: boolean;
  default_days?: number;
  notice_days: number;
  allow_tenant_override: boolean;
  approval_threshold_percent: number;
  reason: string;
}

export function StudentCleanupSettingsCard({ canEdit }: { canEdit: boolean }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const settings = useCleanupSettings();
  const put = usePutCleanupSettings();
  const [form] = Form.useForm<Values>();
  const enabled = Form.useWatch('enabled', form);
  const days = Form.useWatch('default_days', form);

  useEffect(() => {
    const s = settings.data;
    if (s) {
      form.setFieldsValue({
        enabled: s.default_days !== null,
        default_days: s.default_days ?? undefined,
        notice_days: s.notice_days,
        allow_tenant_override: s.allow_tenant_override,
        approval_threshold_percent: s.approval_threshold_percent,
      });
    }
  }, [settings.data, form]);

  const save = (v: Values) =>
    put.mutate(
      {
        default_days: v.enabled ? (v.default_days ?? null) : null,
        notice_days: v.notice_days,
        allow_tenant_override: v.allow_tenant_override,
        approval_threshold_percent: v.approval_threshold_percent,
        reason: v.reason.trim(),
      },
      {
        onSuccess: () => (message.success(t('platform.common.saved')), form.setFieldValue('reason', '')),
        onError: (e) => applyFieldErrors(form, e, ['default_days', 'notice_days', 'approval_threshold_percent', 'reason'], t, (k) => i18n.exists(k)),
      },
    );

  return (
    <Card title={t('platform.settings.cleanupTitle')}>
      {settings.isPending ? <Spin aria-label={t('app.loading')} /> : null}
      {settings.error ? <ProblemMessage error={settings.error} /> : null}
      <Form form={form} name="student_cleanup" layout="vertical" disabled={!canEdit || !settings.data} onFinish={save}>
        <Form.Item name="enabled" label={t('platform.settings.cleanupEnabled')} valuePropName="checked">
          <Switch />
        </Form.Item>
        {enabled ? (
          <Form.Item name="default_days" label={t('platform.settings.defaultDays')} rules={[{ required: true }]}>
            <InputNumber aria-label={t('platform.settings.defaultDays')} min={LIMITS.cleanupDays.minimum} max={LIMITS.cleanupDays.maximum} precision={0} />
          </Form.Item>
        ) : null}
        {enabled && typeof days === 'number' && days < CLEANUP_LOW_DAYS_WARNING ? (
          <Alert type="warning" showIcon role="alert" message={t('platform.settings.lowDaysWarning', { days: CLEANUP_LOW_DAYS_WARNING })} />
        ) : null}
        <Form.Item name="notice_days" label={t('platform.settings.noticeDays')} rules={[{ required: true }]}>
          <InputNumber aria-label={t('platform.settings.noticeDays')} min={LIMITS.cleanupNoticeDays.minimum} max={LIMITS.cleanupNoticeDays.maximum} precision={0} />
        </Form.Item>
        <Form.Item name="allow_tenant_override" label={t('platform.settings.allowTenantOverride')} valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="approval_threshold_percent" label={t('platform.settings.approvalThreshold')} rules={[{ required: true }]}>
          <InputNumber aria-label={t('platform.settings.approvalThreshold')} min={LIMITS.cleanupThreshold.minimum} max={LIMITS.cleanupThreshold.maximum} precision={0} />
        </Form.Item>
        <ReasonField limit={LIMITS.cleanupReason} />
        {put.error ? <ProblemMessage error={put.error} /> : null}
        <ActionGate actionKey="action.platform_admin.system_settings.edit">
          <Button type="primary" htmlType="submit" loading={put.isPending}>
            {t('platform.common.save')}
          </Button>
        </ActionGate>
      </Form>
    </Card>
  );
}

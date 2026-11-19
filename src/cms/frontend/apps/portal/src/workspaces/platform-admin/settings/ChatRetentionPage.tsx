// Gameplay chat retention (F057 T037, F057-AC-16, ADR-0033): days ≥ `minimum_days` returned by the server (schema
// minimum when absent) and ≤ schema maximum, ephemeral hours, tenant override, mandatory reason; PUT with If-Match.
// `422 RETENTION_BELOW_MINIMUM` (or a VALIDATION_FAILED on the field) is shown on the days field by its code.
import { App as AntApp, Button, Card, Form, InputNumber, Spin, Switch } from 'antd';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useChatRetention, usePutChatRetention } from '../api/chatRetention';
import { LIMITS } from '../limits';
import { applyFieldErrors, ReasonField } from '../ui';

interface Values {
  gameplay_chat_retention_days: number;
  ephemeral_retention_hours: number;
  allow_tenant_override: boolean;
  reason: string;
}

export function ChatRetentionCard({ canEdit }: { canEdit: boolean }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const settings = useChatRetention();
  const put = usePutChatRetention();
  const [form] = Form.useForm<Values>();
  const min = settings.data?.minimum_days ?? LIMITS.chatRetentionDays.minimum;

  useEffect(() => {
    const s = settings.data;
    if (s) form.setFieldsValue({ gameplay_chat_retention_days: s.gameplay_chat_retention_days, ephemeral_retention_hours: s.ephemeral_retention_hours, allow_tenant_override: s.allow_tenant_override });
  }, [settings.data, form]);

  const save = (v: Values) =>
    put.mutate(
      { ...v, reason: v.reason.trim() },
      {
        onSuccess: () => (message.success(t('platform.common.saved')), form.setFieldValue('reason', '')),
        onError: (e) => {
          if (e instanceof ApiError && e.code === 'RETENTION_BELOW_MINIMUM') form.setFields([{ name: 'gameplay_chat_retention_days', errors: [t(e.i18nKey)] }]);
          else applyFieldErrors(form, e, ['gameplay_chat_retention_days', 'ephemeral_retention_hours', 'reason'], t, (k) => i18n.exists(k));
        },
      },
    );

  return (
    <Card title={t('platform.settings.chatTitle')}>
      {settings.isPending ? <Spin aria-label={t('app.loading')} /> : null}
      {settings.error ? <ProblemMessage error={settings.error} /> : null}
      <Form form={form} name="chat_retention" layout="vertical" disabled={!canEdit || !settings.data} onFinish={save}>
        <Form.Item
          name="gameplay_chat_retention_days"
          label={t('platform.settings.chatDays')}
          extra={t('platform.settings.chatMinimum', { min })}
          rules={[{ required: true }, { type: 'number', min, max: LIMITS.chatRetentionDays.maximum, message: t('platform.settings.chatMinimum', { min }) }]}
        >
          <InputNumber aria-label={t('platform.settings.chatDays')} precision={0} />
        </Form.Item>
        <Form.Item name="ephemeral_retention_hours" label={t('platform.settings.ephemeralHours')} rules={[{ required: true }]}>
          <InputNumber aria-label={t('platform.settings.ephemeralHours')} min={LIMITS.chatEphemeralHours.minimum} max={LIMITS.chatEphemeralHours.maximum} precision={0} />
        </Form.Item>
        <Form.Item name="allow_tenant_override" label={t('platform.settings.allowTenantOverride')} valuePropName="checked">
          <Switch />
        </Form.Item>
        <ReasonField limit={LIMITS.chatReason} />
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

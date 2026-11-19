// Platform curfew (F057 T052, F057-AC-23, ADR-0078, CHILD_SAFETY §4.1): shows the window from
// getPlatformCurfewSettings (ICT time zone, "Mặc định" when `is_default`, `warning_minutes` read only, "đến … hôm sau"
// when the window crosses midnight); save checks HH:MM (schema pattern) and the mandatory reason, asks a confirmation
// stating the impact (every student of every tenant: game sign-in, game session, chat), then
// putPlatformCurfewSettings with If-Match. `422` field errors land on `start`/`end`/`reason`, `412` stays as a message.
import { App as AntApp, Button, Card, Descriptions, Form, Input, Spin, Tag } from 'antd';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { crossesMidnight, usePlatformCurfew, usePutPlatformCurfew, type CurfewInput } from '../api/curfew';
import { LIMITS } from '../limits';
import { applyFieldErrors, ReasonField } from '../ui';

export function PlatformCurfewSettingsCard({ canEdit }: { canEdit: boolean }) {
  const { t, i18n } = useTranslation();
  const { modal, message } = AntApp.useApp();
  const curfew = usePlatformCurfew();
  const put = usePutPlatformCurfew();
  const [form] = Form.useForm<CurfewInput>();
  const start = Form.useWatch('start', form) ?? '';
  const end = Form.useWatch('end', form) ?? '';
  const timeRules = [
    { required: true, message: t('platform.settings.curfewFormat') },
    { pattern: new RegExp(LIMITS.curfewTime.pattern), message: t('platform.settings.curfewFormat') },
  ];

  useEffect(() => {
    if (curfew.data) form.setFieldsValue({ start: curfew.data.start, end: curfew.data.end });
  }, [curfew.data, form]);

  const save = (v: CurfewInput) =>
    modal.confirm({
      title: t('platform.settings.curfewConfirmTitle'),
      content: t('platform.settings.curfewConfirmContent'),
      okText: t('platform.common.confirm'),
      cancelText: t('platform.common.cancel'),
      onOk: () =>
        put
          .mutateAsync({ start: v.start, end: v.end, reason: v.reason.trim() })
          .then(() => (message.success(t('platform.common.saved')), form.setFieldValue('reason', '')))
          .catch((e: unknown) => {
            applyFieldErrors(form, e, ['start', 'end', 'reason'], t, (k) => i18n.exists(k));
          }),
    });

  const d = curfew.data;
  return (
    <Card title={t('platform.settings.curfewTitle')} extra={d?.is_default ? <Tag>{t('platform.settings.curfewDefault')}</Tag> : null}>
      {curfew.isPending ? <Spin aria-label={t('app.loading')} /> : null}
      {curfew.error ? <ProblemMessage error={curfew.error} /> : null}
      {d ? (
        <Descriptions column={1} size="small">
          <Descriptions.Item label={t('platform.settings.curfewTitle')}>
            <span data-testid="curfew-range">
              {t(crossesMidnight(start || d.start, end || d.end) ? 'platform.settings.curfewRangeNextDay' : 'platform.settings.curfewRange', { start: start || d.start, end: end || d.end })}
            </span>
          </Descriptions.Item>
          {d.timezone ? <Descriptions.Item>{t('platform.settings.curfewTimezone', { tz: d.timezone })}</Descriptions.Item> : null}
          {typeof d.warning_minutes === 'number' ? <Descriptions.Item>{t('platform.settings.curfewWarning', { minutes: d.warning_minutes })}</Descriptions.Item> : null}
        </Descriptions>
      ) : null}
      <Form form={form} name="platform_curfew" layout="vertical" disabled={!canEdit || !d} onFinish={save}>
        <Form.Item name="start" label={t('platform.settings.curfewStart')} rules={timeRules}>
          <Input aria-label={t('platform.settings.curfewStart')} maxLength={LIMITS.curfewTime.maxLength} />
        </Form.Item>
        <Form.Item name="end" label={t('platform.settings.curfewEnd')} rules={timeRules}>
          <Input aria-label={t('platform.settings.curfewEnd')} maxLength={LIMITS.curfewTime.maxLength} />
        </Form.Item>
        <ReasonField limit={LIMITS.curfewReason} />
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

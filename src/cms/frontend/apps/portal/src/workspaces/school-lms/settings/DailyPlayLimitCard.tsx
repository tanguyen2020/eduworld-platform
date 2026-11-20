// "Giới hạn chơi mỗi ngày" of a school (F052 T062, F052-AC-32, OQ-17, CHILD_SAFETY §4.2): off by default ("Tắt" when
// `daily_play_limit_minutes` is null); turning it on, changing or turning it off sends updateSchoolSettings
// (`daily_play_limit_minutes`, null = off) with a mandatory reason and If-Match. The range comes from the contract
// (LIMITS.dailyPlayLimit mirrors SchoolSettings.daily_play_limit_minutes, checked by fieldSchemas.test.ts). 422 → the
// field; 412 → reload. One rule for every student of the school; the `common` tenant is never limited (no edit).
import { App, Button, Card, Descriptions, Form, Input, InputNumber, Space, Switch, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { applyFormError, writeErrorKey } from '../common/errors';
import { FIELDS, LIMITS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { useUpdateSchoolSettings, type SchoolSettings } from './useSettings';

interface LimitForm {
  enabled: boolean;
  daily_play_limit_minutes?: number | null;
  reason: string;
}

const { minimum: MIN, maximum: MAX } = LIMITS.dailyPlayLimit;

export function DailyPlayLimitCard({ schoolId, settings, isCommon, onReload }: { schoolId: string; settings: SchoolSettings; isCommon: boolean; onReload: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<LimitForm>();
  const update = useUpdateSchoolSettings(schoolId);
  const field = useFieldRules();
  const [editing, setEditing] = useState(false);
  const enabled = Form.useWatch('enabled', form) ?? false;
  const current = settings.daily_play_limit_minutes ?? null;

  const save = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    try {
      await update.mutateAsync({ daily_play_limit_minutes: v.enabled ? (v.daily_play_limit_minutes ?? null) : null, reason: v.reason.trim() });
      message.success(t('portal.school_lms.common.saved'));
      setEditing(false);
    } catch (e) {
      if (e instanceof ApiError && e.code === 'VERSION_MISMATCH') {
        message.warning(t('portal.school_lms.curfew.reloaded'));
        onReload();
        return;
      }
      if (!applyFormError(form, e, ['daily_play_limit_minutes', 'reason'], t, i18n.exists.bind(i18n))) message.error(t(writeErrorKey(e)));
    }
  };

  return (
    <Card title={t('portal.school_lms.dailyLimit.title')} data-testid="daily-limit-card">
      <Descriptions column={1} size="small">
        <Descriptions.Item label={t('portal.school_lms.dailyLimit.current')}>
          <span data-testid="daily-limit-value">
            {isCommon ? t('portal.school_lms.dailyLimit.common') : current == null ? t('portal.school_lms.dailyLimit.off') : t('portal.school_lms.dailyLimit.minutes', { count: current })}
          </span>
        </Descriptions.Item>
      </Descriptions>
      <Typography.Paragraph type="secondary">{t('portal.school_lms.dailyLimit.note')}</Typography.Paragraph>
      {isCommon ? null : (
        <ActionGate actionKey="action.school_lms.settings.edit_school">
          {editing ? (
            <Form<LimitForm>
              form={form}
              layout="vertical"
              initialValues={{ enabled: current != null, daily_play_limit_minutes: current ?? undefined, reason: '' }}
              data-testid="daily-limit-form"
            >
              <Form.Item name="enabled" label={t('portal.school_lms.dailyLimit.enabled')} valuePropName="checked">
                <Switch />
              </Form.Item>
              {enabled ? (
                <Form.Item
                  name="daily_play_limit_minutes"
                  label={t('portal.school_lms.dailyLimit.limit')}
                  extra={t('portal.school_lms.dailyLimit.range', { min: MIN, max: MAX })}
                  rules={[{ required: true, type: 'integer', min: MIN, max: MAX, message: t('portal.school_lms.dailyLimit.range', { min: MIN, max: MAX }) }]}
                >
                  <InputNumber min={MIN} max={MAX} precision={0} addonAfter={t('portal.school_lms.dailyLimit.unit')} />
                </Form.Item>
              ) : null}
              <Form.Item name="reason" label={t('portal.school_lms.common.reason')} rules={field.rules(FIELDS.schoolSettingsReason, true)}>
                <Input.TextArea autoSize={{ minRows: 2, maxRows: 4 }} {...field.input(FIELDS.schoolSettingsReason)} />
              </Form.Item>
              <Space wrap>
                <Button onClick={() => setEditing(false)}>{t('portal.school_lms.common.cancel')}</Button>
                <Button type="primary" loading={update.isPending} onClick={() => void save()}>
                  {t('portal.school_lms.common.save')}
                </Button>
              </Space>
            </Form>
          ) : (
            <Button onClick={() => setEditing(true)}>{t('portal.school_lms.dailyLimit.edit')}</Button>
          )}
        </ActionGate>
      )}
    </Card>
  );
}

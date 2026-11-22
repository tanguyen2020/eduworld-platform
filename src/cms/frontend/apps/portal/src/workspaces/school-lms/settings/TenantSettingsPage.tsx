// Tenant settings (F052 T041, F052-AC-19, F052-AC-20): access hours, default language, inactive-student cleanup days
// (when `tenant.student_cleanup.configure`), gameplay chat retention (when `tenant.chat_retention.configure`).
// updateTenantSettings = JSON Merge Patch of the changed fields + If-Match; shows `effective_cleanup_days`; warns
// below the cleanup threshold. 403 PERMISSION_DENIED on the cleanup override / 422 RETENTION_BELOW_MINIMUM → error at
// that field and the saved value is kept.
import { Alert, App, Button, Card, Descriptions, Form, InputNumber, Space, Spin } from 'antd';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { ApiError } from '../../../shared/api/middleware/problem';
import { applyFormError, QueryError, writeErrorKey } from '../common/errors';
import { LIMITS } from '../common/fieldSchemas';
import { AllowedHoursField } from './AllowedHoursField';
import { cleanupWarning, mergePatch, type TenantSettingsForm } from './tenantSettingsForm';
import { useTenantSettings, useUpdateTenantSettings } from './useSettings';
import { SearchSelect } from '@eduworld/ops-ui';

export function TenantSettingsPage() {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const { view } = usePermissions();
  const q = useTenantSettings();
  const update = useUpdateTenantSettings();
  const [form] = Form.useForm<TenantSettingsForm>();
  const days = Form.useWatch('inactive_student_cleanup_days', form);
  const canEdit = !!view && actionStatus('action.school_lms.settings.edit', view) === 'enabled';
  const perms = view?.permissions ?? [];
  const editable = new Set<string>(['allowed_hours', 'default_locale']);
  if (perms.includes('tenant.student_cleanup.configure')) editable.add('inactive_student_cleanup_days');
  if (perms.includes('tenant.chat_retention.configure')) editable.add('gameplay_chat_retention_days');
  if (q.error) return <QueryError error={q.error} />;
  if (!q.data) return <Spin aria-label={t('app.loading')} />;
  const saved = q.data;
  const submit = async (v: TenantSettingsForm) => {
    const patch = mergePatch(saved, v, editable);
    if (!Object.keys(patch).length) return;
    try {
      await update.mutateAsync(patch);
      message.success(t('portal.school_lms.common.saved'));
    } catch (e) {
      const sent = Object.keys(patch);
      const fieldFor: Record<string, string> = { RETENTION_BELOW_MINIMUM: 'gameplay_chat_retention_days' };
      if (e instanceof ApiError && e.code === 'PERMISSION_DENIED' && sent.includes('inactive_student_cleanup_days')) fieldFor.PERMISSION_DENIED = 'inactive_student_cleanup_days';
      const target = e instanceof ApiError ? fieldFor[e.code] : undefined;
      if (target) form.setFieldValue(target as keyof TenantSettingsForm, saved[target as keyof typeof saved] ?? null);
      if (!applyFormError(form, e, sent, t, i18n.exists.bind(i18n), fieldFor)) message.error(t(writeErrorKey(e)));
      if (e instanceof ApiError && e.code === 'VERSION_MISMATCH') void q.refetch();
    }
  };
  const ro = (f: string) => !canEdit || !editable.has(f);
  return (
    <Card data-testid="tenant-settings">
      <Form<TenantSettingsForm> form={form} layout="vertical" initialValues={saved} onFinish={submit}>
        <Form.Item name="default_locale" label={t('portal.school_lms.settings.defaultLocale')}>
          <SearchSelect disabled={ro('default_locale')} options={(['vi-VN', 'en-US'] as const).map((l) => ({ value: l, label: t(`shell.locale.${l}`) }))} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.settings.allowedHours')}>
          <AllowedHoursField disabled={ro('allowed_hours')} />
        </Form.Item>
        <Form.Item
          name="inactive_student_cleanup_days"
          label={t('portal.school_lms.settings.cleanupDays')}
          extra={t('portal.school_lms.settings.cleanupDaysHelp')}
        >
          <InputNumber min={LIMITS.cleanupDays.minimum} max={LIMITS.cleanupDays.maximum} disabled={ro('inactive_student_cleanup_days')} aria-label={t('portal.school_lms.settings.cleanupDays')} />
        </Form.Item>
        {cleanupWarning(days) ? <Alert type="warning" showIcon role="alert" message={t('portal.school_lms.settings.cleanupWarning')} /> : null}
        <Descriptions size="small" column={1}>
          <Descriptions.Item label={t('portal.school_lms.settings.effectiveCleanupDays')}>
            {saved.effective_cleanup_days == null ? t('portal.school_lms.settings.cleanupOff') : t('portal.school_lms.students.days', { count: saved.effective_cleanup_days })}
          </Descriptions.Item>
        </Descriptions>
        <Form.Item name="gameplay_chat_retention_days" label={t('portal.school_lms.settings.chatRetention')} extra={t('portal.school_lms.settings.chatRetentionHelp')}>
          <InputNumber min={1} max={LIMITS.chatRetentionDays.maximum} disabled={ro('gameplay_chat_retention_days')} aria-label={t('portal.school_lms.settings.chatRetention')} />
        </Form.Item>
        <ActionGate actionKey="action.school_lms.settings.edit">
          <Space>
            <Button type="primary" htmlType="submit" loading={update.isPending}>
              {t('portal.school_lms.common.save')}
            </Button>
          </Space>
        </ActionGate>
      </Form>
    </Card>
  );
}

// Feature flag editor (F059 T043, F059-AC-20/21, ADR-0090): platform default (scope `platform`) or one tenant's
// override (scope `tenant`, target tenant → X-Tenant-Id, state enabled/disabled/inherit = remove the override), the
// description of a `feature` flag, mandatory reason (3–500). putFeatureFlag sends If-Match = version of the row. On
// `412` the list reloads and the editor shows "Đã có người sửa" with the new values; a protected item
// (`can_disable = false`) cannot be turned off.
import { Alert, App as AntApp, Form, Modal, Radio, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { usePutFeatureFlag, type FeatureFlag, type FeatureFlagState, type FeatureFlagWrite } from '../api/featureFlags';
import { LIMITS } from '../limits';
import { applyFieldErrors, LimitedField, ReasonField, TenantSelect } from '../ui';

interface FormValues {
  scope: FeatureFlagWrite['scope'];
  tenant_id?: string;
  state: FeatureFlagState;
  description?: string;
  reason: string;
}

export function FeatureFlagEditor({ flag, tenantId, onClose }: { flag: FeatureFlag; tenantId?: string; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<FormValues>();
  const put = usePutFeatureFlag();
  const [stale, setStale] = useState(false);
  const scope: FeatureFlagWrite['scope'] = Form.useWatch('scope', form) ?? (tenantId ? 'tenant' : 'platform');
  const initialState: FeatureFlagState = tenantId ? (flag.tenant_override == null ? 'inherit' : flag.tenant_override ? 'enabled' : 'disabled') : flag.default_enabled ? 'enabled' : 'disabled';
  const states: FeatureFlagState[] = scope === 'tenant' ? ['enabled', 'disabled', 'inherit'] : ['enabled', 'disabled'];

  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const body: FeatureFlagWrite = {
      scope: v.scope,
      state: v.state,
      reason: v.reason.trim(),
      ...(flag.flag_kind === 'feature' && v.description?.trim() ? { description: v.description.trim() } : {}),
    };
    setStale(false);
    put.mutate(
      { flag, body, tenantId: v.scope === 'tenant' ? v.tenant_id : undefined },
      {
        onSuccess: () => {
          message.success(t('platform.featureFlags.saved'));
          onClose();
        },
        onError: (e) => {
          if (e instanceof ApiError && e.status === 412) setStale(true);
          applyFieldErrors(form, e, ['scope', 'state', 'description', 'reason'], t, (k) => i18n.exists(k));
        },
      },
    );
  };

  return (
    <Modal open title={t('platform.featureFlags.editTitle', { key: flag.flag_key })} okText={t('platform.common.save')} cancelText={t('platform.common.cancel')} confirmLoading={put.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Typography.Paragraph type="secondary">{t('platform.featureFlags.currentVersion', { version: flag.version })}</Typography.Paragraph>
      <Form
        form={form}
        name="feature_flag"
        layout="vertical"
        initialValues={{ scope: tenantId ? 'tenant' : 'platform', tenant_id: tenantId, state: initialState, description: flag.description }}
        onValuesChange={(changed: Partial<FormValues>) => {
          if (changed.scope === 'platform' && form.getFieldValue('state') === 'inherit') form.setFieldValue('state', flag.default_enabled ? 'enabled' : 'disabled');
        }}
      >
        <Form.Item name="scope" label={t('platform.featureFlags.scope')}>
          <Radio.Group options={(['platform', 'tenant'] as const).map((x) => ({ value: x, label: t(`platform.featureFlags.scopeKind.${x}`) }))} />
        </Form.Item>
        {scope === 'tenant' ? (
          <Form.Item name="tenant_id" label={t('platform.featureFlags.targetTenant')} rules={[{ required: true, message: t('shell.tenantTarget.required') }]}>
            <TenantSelect ariaLabel={t('platform.featureFlags.targetTenant')} />
          </Form.Item>
        ) : null}
        <Form.Item name="state" label={t('platform.featureFlags.state')} rules={[{ required: true }]}>
          <Radio.Group options={states.map((x) => ({ value: x, label: t(`platform.featureFlags.stateKind.${x}`), disabled: x === 'disabled' && !flag.can_disable }))} />
        </Form.Item>
        {flag.can_disable ? null : <Alert type="info" showIcon message={t('platform.featureFlags.protectedHelp')} />}
        {flag.flag_kind === 'feature' ? <LimitedField name="description" label={t('platform.featureFlags.description')} limit={LIMITS.flagDescription} textarea /> : null}
        <ReasonField limit={LIMITS.flagReason} />
      </Form>
      {put.error ? <ProblemMessage error={put.error} /> : null}
      {stale ? <Alert type="warning" showIcon message={t('platform.featureFlags.stale', { version: flag.version })} /> : null}
    </Modal>
  );
}

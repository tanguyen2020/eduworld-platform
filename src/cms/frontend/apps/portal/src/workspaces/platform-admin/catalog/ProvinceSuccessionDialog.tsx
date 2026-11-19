// Province succession (F057 T033, F057-AC-12, ADR-0060): old unit → new unit, effective date, kind, reason ≤ 1000;
// createProvinceSuccession with Idempotency-Key. Old and new must differ (one succession per old unit, server-checked).
import { App as AntApp, DatePicker, Form, Modal, Select } from 'antd';
import type { Dayjs } from 'dayjs';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { SUCCESSION_KINDS, useCreateSuccession, type Province, type ProvinceSuccessionInput } from '../api/catalog';
import { LIMITS } from '../limits';
import { ReasonField } from '../ui';

interface Values {
  old_province_id: string;
  new_province_id: string;
  effective_date: Dayjs;
  kind: ProvinceSuccessionInput['kind'];
  reason: string;
}

export function ProvinceSuccessionDialog({ provinces, onClose }: { provinces: readonly Province[]; onClose: () => void }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<Values>();
  const create = useCreateSuccession();
  const options = provinces.map((p) => ({ value: p.province_id, label: `${p.name.vi} (${p.code})` }));
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    create.mutate(
      { old_province_id: v.old_province_id, new_province_id: v.new_province_id, effective_date: v.effective_date.format('YYYY-MM-DD'), kind: v.kind, reason: v.reason.trim() },
      { onSuccess: () => (message.success(t('platform.common.saved')), onClose()) },
    );
  };
  return (
    <Modal open title={t('platform.catalog.succession')} okText={t('platform.common.save')} cancelText={t('platform.common.cancel')} confirmLoading={create.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Form form={form} name="province_succession" layout="vertical" initialValues={{ kind: 'merged' }}>
        <Form.Item name="old_province_id" label={t('platform.catalog.oldProvince')} rules={[{ required: true }]}>
          <Select showSearch optionFilterProp="label" options={options} />
        </Form.Item>
        <Form.Item
          name="new_province_id"
          label={t('platform.catalog.newProvince')}
          dependencies={['old_province_id']}
          rules={[{ required: true }, ({ getFieldValue }) => ({ validator: (_, v?: string) => (v && v === getFieldValue('old_province_id') ? Promise.reject(new Error(t('platform.catalog.sameProvince'))) : Promise.resolve()) })]}
        >
          <Select showSearch optionFilterProp="label" options={options} />
        </Form.Item>
        <Form.Item name="effective_date" label={t('platform.catalog.effectiveDate')} rules={[{ required: true }]}>
          <DatePicker format="DD/MM/YYYY" />
        </Form.Item>
        <Form.Item name="kind" label={t('platform.catalog.kind')} rules={[{ required: true }]}>
          <Select options={SUCCESSION_KINDS.map((k) => ({ value: k, label: t(`platform.catalog.kinds.${k}`) }))} />
        </Form.Item>
        <ReasonField limit={LIMITS.successionReason} />
      </Form>
      {create.error ? <ProblemMessage error={create.error} /> : null}
    </Modal>
  );
}

// Provinces (F057 T033, F057-AC-12, ADR-0060, ADMIN_PORTAL §3.16): listProvinces with `status` (default `all`),
// search by name/code over the loaded catalog (≤ 500 units, one response), effective dates and `successor_code`.
// Edit = names and `official_code` only (updateProvince); a boundary change = createProvince (new unit) +
// createProvinceSuccession (ProvinceSuccessionDialog) — an old unit is never overwritten.
import { App as AntApp, Button, DatePicker, Form, Input, Modal, Segmented, Space, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useCountries, useCreateProvince, useProvinces, useUpdateProvince, type Province, type ProvinceInput, type ProvinceStatusFilter } from '../api/catalog';
import { LIMITS } from '../limits';
import { LimitedField, ReasonField } from '../ui';
import { ProvinceSuccessionDialog } from './ProvinceSuccessionDialog';
import { SearchSelect } from '@eduworld/ops-ui';

const PROVINCE_TYPES: readonly Province['province_type'][] = ['province', 'municipality'];

interface FormValues {
  code?: string;
  name_vi: string;
  name_en: string;
  short_vi: string;
  short_en: string;
  official_code?: string;
  country_code?: string;
  province_type?: Province['province_type'];
  effective_from?: { format: (f: string) => string } | null;
  reason: string;
}

function ProvinceForm({ province, onClose }: { province: Province | null; onClose: () => void }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<FormValues>();
  const create = useCreateProvince();
  const update = useUpdateProvince();
  const countries = useCountries();
  const mutation = province ? update : create;
  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const names = { name: { vi: v.name_vi.trim(), en: v.name_en.trim() }, short_name: { vi: v.short_vi.trim(), en: v.short_en.trim() } };
    const official = v.official_code?.trim() ? v.official_code.trim() : null;
    const ok = () => (message.success(t('platform.common.saved')), onClose());
    if (province) {
      update.mutate({ id: province.province_id, version: province.version, input: { ...names, official_code: official, reason: v.reason.trim() } }, { onSuccess: ok });
    } else {
      const input: ProvinceInput = {
        ...names,
        code: v.code!.trim(),
        country_code: v.country_code,
        province_type: v.province_type,
        official_code: official,
        effective_from: v.effective_from ? v.effective_from.format('YYYY-MM-DD') : null,
        status: 'active',
        reason: v.reason.trim(),
      };
      create.mutate(input, { onSuccess: ok });
    }
  };
  return (
    <Modal
      open
      title={province ? t('platform.catalog.editProvince', { name: province.name.vi }) : t('platform.catalog.createProvince')}
      okText={t('platform.common.save')}
      cancelText={t('platform.common.cancel')}
      confirmLoading={mutation.isPending}
      onOk={submit}
      onCancel={onClose}
      destroyOnHidden
    >
      {province ? null : <p>{t('platform.catalog.createProvinceHelp')}</p>}
      <Form
        form={form}
        name="province"
        layout="vertical"
        initialValues={
          province
            ? { name_vi: province.name.vi, name_en: province.name.en, short_vi: province.short_name.vi, short_en: province.short_name.en, official_code: province.official_code ?? '' }
            : { province_type: 'province' }
        }
      >
        {province ? null : <LimitedField name="code" label={t('platform.catalog.provinceCode')} limit={LIMITS.provinceCode} required />}
        <LimitedField name="name_vi" label={t('platform.catalog.nameVi')} limit={LIMITS.localizedName} required />
        <LimitedField name="name_en" label={t('platform.catalog.nameEn')} limit={LIMITS.localizedName} required />
        <LimitedField name="short_vi" label={t('platform.catalog.shortNameVi')} limit={LIMITS.localizedName} required />
        <LimitedField name="short_en" label={t('platform.catalog.shortNameEn')} limit={LIMITS.localizedName} required />
        <LimitedField name="official_code" label={t('platform.catalog.officialCode')} help={t('platform.catalog.officialCodeHelp')} limit={LIMITS.provinceOfficialCode} />
        {province ? null : (
          <>
            <Form.Item name="country_code" label={t('platform.catalog.countryCode')} rules={[{ required: true }]}>
              <SearchSelect options={(countries.data?.items ?? []).map((c) => ({ value: c.country_code, label: `${c.country_code} · ${c.name.vi}` }))} />
            </Form.Item>
            <Form.Item name="province_type" label={t('platform.catalog.provinceType')} rules={[{ required: true }]}>
              <SearchSelect options={PROVINCE_TYPES.map((x) => ({ value: x, label: t(`platform.catalog.provinceTypes.${x}`) }))} />
            </Form.Item>
            <Form.Item name="effective_from" label={t('platform.catalog.effectiveFrom')}>
              <DatePicker format="DD/MM/YYYY" />
            </Form.Item>
          </>
        )}
        <ReasonField limit={LIMITS.provinceReason} />
      </Form>
      {mutation.error ? <ProblemMessage error={mutation.error} /> : null}
    </Modal>
  );
}

export function ProvincesPage() {
  const { t } = useTranslation();
  const [status, setStatus] = useState<ProvinceStatusFilter>('all');
  const [search, setSearch] = useState('');
  const provinces = useProvinces(status);
  const [editing, setEditing] = useState<Province | 'new' | null>(null);
  const [succession, setSuccession] = useState(false);
  const rows = useMemo(() => {
    const s = search.trim().toLowerCase();
    const items = provinces.data?.items ?? [];
    return s ? items.filter((p) => p.code.includes(s) || p.name.vi.toLowerCase().includes(s) || p.name.en.toLowerCase().includes(s) || (p.official_code ?? '').includes(s)) : items;
  }, [provinces.data, search]);

  const columns: ColumnsType<Province> = [
    { key: 'code', title: t('platform.catalog.provinceCode'), dataIndex: 'code' },
    { key: 'official', title: t('platform.catalog.officialCode'), render: (_, r) => r.official_code ?? '' },
    { key: 'name', title: t('platform.catalog.nameVi'), render: (_, r) => r.name.vi },
    { key: 'type', title: t('platform.catalog.provinceType'), render: (_, r) => t(`platform.catalog.provinceTypes.${r.province_type}`) },
    { key: 'effective', title: t('platform.catalog.effective'), render: (_, r) => [r.effective_from ?? '', r.effective_to ?? ''].join(' → ') },
    {
      key: 'status',
      title: t('platform.common.status'),
      render: (_, r) => <Tag color={r.status === 'active' ? 'success' : 'default'}>{t(r.status === 'active' ? 'platform.catalog.statusActive' : 'platform.catalog.statusInactive')}</Tag>,
    },
    { key: 'successor', title: t('platform.catalog.successor'), render: (_, r) => r.successor_code ?? '' },
    {
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, r) => (
        <ActionGate actionKey="action.platform_admin.catalog.edit">
          <Button size="small" onClick={() => setEditing(r)}>
            {t('platform.common.edit')}
          </Button>
        </ActionGate>
      ),
    },
  ];

  return (
    <section aria-label={t('platform.catalog.provinces')}>
      <Space wrap>
        <Segmented<ProvinceStatusFilter>
          value={status}
          onChange={setStatus}
          options={[
            { value: 'all', label: t('platform.catalog.statusAll') },
            { value: 'active', label: t('platform.catalog.statusActive') },
            { value: 'inactive', label: t('platform.catalog.statusInactive') },
          ]}
        />
        <Input.Search aria-label={t('platform.catalog.searchProvince')} placeholder={t('platform.catalog.searchProvince')} maxLength={LIMITS.provinceCode.maxLength} showCount allowClear onChange={(e) => setSearch(e.target.value)} />
        <ActionGate actionKey="action.platform_admin.catalog.edit">
          <Space>
            <Button onClick={() => setEditing('new')}>{t('platform.catalog.createProvince')}</Button>
            <Button onClick={() => setSuccession(true)}>{t('platform.catalog.succession')}</Button>
          </Space>
        </ActionGate>
      </Space>
      {provinces.error ? <ProblemMessage error={provinces.error} /> : null}
      <Table rowKey="province_id" columns={columns} dataSource={rows} loading={provinces.isFetching} pagination={{ pageSize: 50, hideOnSinglePage: true }} />
      {editing ? <ProvinceForm province={editing === 'new' ? null : editing} onClose={() => setEditing(null)} /> : null}
      {succession ? <ProvinceSuccessionDialog provinces={provinces.data?.items ?? []} onClose={() => setSuccession(false)} /> : null}
    </section>
  );
}

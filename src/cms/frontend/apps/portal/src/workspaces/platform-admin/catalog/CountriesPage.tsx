// Countries (F057 T033, F057-AC-11, ADMIN_PORTAL §3.16): listCountries; with `platform.catalog.write` only turn a
// country on/off and edit its bilingual name through updateCountry, always with a reason (≤ 1000). No delete action.
import { App as AntApp, Button, Form, Modal, Space, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { useCountries, useUpdateCountry, type Country } from '../api/catalog';
import { LIMITS } from '../limits';
import { LimitedField, ReasonField } from '../ui';

export function CountriesPage() {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const countries = useCountries();
  const update = useUpdateCountry();
  const [toggling, setToggling] = useState<Country | null>(null);
  const [renaming, setRenaming] = useState<Country | null>(null);
  const [form] = Form.useForm<{ vi: string; en: string; reason: string }>();

  const done = () => {
    message.success(t('platform.common.saved'));
    setToggling(null);
    setRenaming(null);
  };
  const close = () => {
    setToggling(null);
    setRenaming(null);
    update.reset();
  };

  const columns: ColumnsType<Country> = [
    { key: 'code', title: t('platform.catalog.countryCode'), dataIndex: 'country_code' },
    { key: 'vi', title: t('platform.catalog.nameVi'), render: (_, r) => r.name.vi },
    { key: 'en', title: t('platform.catalog.nameEn'), render: (_, r) => r.name.en },
    { key: 'locale', title: t('platform.catalog.defaultLocale'), dataIndex: 'default_locale' },
    {
      key: 'status',
      title: t('platform.common.status'),
      render: (_, r) => <Tag color={r.status === 'active' ? 'success' : 'default'}>{t(r.status === 'active' ? 'platform.catalog.statusActive' : 'platform.catalog.statusInactive')}</Tag>,
    },
    {
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, r) => (
        <ActionGate actionKey="action.platform_admin.catalog.edit">
          <Space>
            <Button size="small" onClick={() => (form.setFieldsValue({ vi: r.name.vi, en: r.name.en }), setRenaming(r))}>
              {t('platform.catalog.editName')}
            </Button>
            <Button size="small" onClick={() => setToggling(r)}>
              {t(r.status === 'active' ? 'platform.catalog.deactivate' : 'platform.catalog.activate')}
            </Button>
          </Space>
        </ActionGate>
      ),
    },
  ];

  return (
    <section aria-label={t('platform.catalog.countries')}>
      {countries.error ? <ProblemMessage error={countries.error} /> : null}
      <Table rowKey="country_code" columns={columns} dataSource={countries.data?.items ?? []} loading={countries.isFetching} pagination={false} />
      <ReasonDialog
        open={!!toggling}
        title={toggling ? t('platform.catalog.toggleTitle', { name: toggling.name.vi }) : ''}
        maxLength={LIMITS.countryReason.maxLength}
        confirmLoading={update.isPending}
        error={update.error ? <ProblemMessage error={update.error} /> : null}
        onCancel={close}
        onSubmit={(reason) =>
          toggling && update.mutate({ code: toggling.country_code, version: toggling.version, input: { status: toggling.status === 'active' ? 'inactive' : 'active', reason } }, { onSuccess: done })
        }
      />
      <Modal
        open={!!renaming}
        title={t('platform.catalog.editName')}
        okText={t('platform.common.save')}
        cancelText={t('platform.common.cancel')}
        confirmLoading={update.isPending}
        onCancel={close}
        forceRender
        onOk={async () => {
          const v = await form.validateFields().catch(() => null);
          if (v && renaming) update.mutate({ code: renaming.country_code, version: renaming.version, input: { name: { vi: v.vi.trim(), en: v.en.trim() }, reason: v.reason.trim() } }, { onSuccess: done });
        }}
      >
        <Form form={form} name="country_name" layout="vertical">
          <LimitedField name="vi" label={t('platform.catalog.nameVi')} limit={LIMITS.localizedName} required />
          <LimitedField name="en" label={t('platform.catalog.nameEn')} limit={LIMITS.localizedName} required />
          <ReasonField limit={LIMITS.countryReason} />
        </Form>
        {update.error ? <ProblemMessage error={update.error} /> : null}
      </Modal>
    </section>
  );
}

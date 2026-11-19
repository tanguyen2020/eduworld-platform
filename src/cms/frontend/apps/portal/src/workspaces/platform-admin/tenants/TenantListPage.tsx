// Tenant screen (F057 T039, F057-AC-17, ADMIN_PORTAL §10.1 "Quản lý tenant"): listTenants filtered by `q`/`tenant_type`
// with cursor paging, create/edit through TenantForm, activate/disable after a confirmation with reason. Write actions
// are wrapped in <ActionGate action.platform_admin.tenants.edit> and hidden on small screens (read only).
import { Alert, Button, Input, Select, Space, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { TENANT_TYPES, useTenantList, useUpdateTenant, type TenantSummary, type TenantType } from '../api/tenants';
import { LIMITS } from '../limits';
import { PageTitle, useLargeScreen } from '../ui';
import { TenantForm } from './TenantForm';

export function TenantListPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const large = useLargeScreen();
  const [q, setQ] = useState('');
  const [tenantType, setTenantType] = useState<TenantType | undefined>();
  const list = useTenantList(q, tenantType);
  const [form, setForm] = useState<TenantSummary | 'new' | null>(null);
  const [toggling, setToggling] = useState<TenantSummary | null>(null);
  const update = useUpdateTenant();

  const columns: ColumnsType<TenantSummary> = [
    { key: 'code', title: t('platform.tenants.code'), dataIndex: 'code' },
    { key: 'name', title: t('platform.tenants.name'), dataIndex: 'name' },
    { key: 'type', title: t('platform.tenants.type'), render: (_, r) => <Tag>{t(`shell.tenantType.${r.tenant_type}`)}</Tag> },
    {
      key: 'status',
      title: t('platform.common.status'),
      render: (_, r) => <Tag color={r.status === 'active' ? 'success' : 'default'}>{t(r.status === 'active' ? 'platform.tenants.statusActive' : 'platform.tenants.statusDisabled')}</Tag>,
    },
    { key: 'schools', title: t('platform.tenants.schoolCount'), dataIndex: 'school_count' },
  ];
  if (large) {
    columns.push({
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, r) => (
        <ActionGate actionKey="action.platform_admin.tenants.edit">
          <Space>
            <Button size="small" onClick={() => setForm(r)}>
              {t('platform.common.edit')}
            </Button>
            <Button size="small" danger={r.status === 'active'} onClick={() => setToggling(r)}>
              {t(r.status === 'active' ? 'platform.tenants.disable' : 'platform.tenants.enable')}
            </Button>
          </Space>
        </ActionGate>
      ),
    });
  }

  const disabling = toggling?.status === 'active';
  return (
    <section>
      <PageTitle
        title={title}
        extra={
          large ? (
            <ActionGate actionKey="action.platform_admin.tenants.edit">
              <Button type="primary" onClick={() => setForm('new')}>
                {t('platform.tenants.create')}
              </Button>
            </ActionGate>
          ) : null
        }
      />
      {large ? null : <Alert type="info" showIcon message={t('platform.common.largeScreenReadOnly')} />}
      <Space wrap>
        <Input.Search
          aria-label={t('platform.tenants.searchPlaceholder')}
          placeholder={t('platform.tenants.searchPlaceholder')}
          maxLength={LIMITS.tenantSearch.maxLength}
          showCount
          allowClear
          onSearch={(v) => setQ(v.trim())}
        />
        <Select
          aria-label={t('platform.tenants.type')}
          placeholder={t('platform.tenants.allTypes')}
          allowClear
          value={tenantType}
          onChange={setTenantType}
          options={TENANT_TYPES.map((x) => ({ value: x, label: t(`shell.tenantType.${x}`) }))}
        />
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="tenant_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
      {form ? <TenantForm tenant={form === 'new' ? null : form} onClose={() => setForm(null)} /> : null}
      <ReasonDialog
        open={!!toggling}
        title={toggling ? t(disabling ? 'platform.tenants.disableTitle' : 'platform.tenants.enableTitle', { name: toggling.name }) : ''}
        description={disabling ? t('platform.tenants.disableContent') : undefined}
        maxLength={LIMITS.tenantUpdateReason.maxLength}
        minLength={LIMITS.tenantUpdateReason.minLength}
        confirmLoading={update.isPending}
        error={update.error ? <ProblemMessage error={update.error} /> : null}
        onCancel={() => {
          setToggling(null);
          update.reset();
        }}
        onSubmit={(reason) =>
          toggling &&
          update.mutate(
            { tenantId: toggling.tenant_id, patch: { status: disabling ? 'disabled' : 'active', reason } },
            { onSuccess: () => setToggling(null) },
          )
        }
      />
    </section>
  );
}

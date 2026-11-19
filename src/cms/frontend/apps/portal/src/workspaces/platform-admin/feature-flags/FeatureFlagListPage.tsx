// Menu "Feature flag · Bật/tắt menu & action" (F059 US6, F059-AC-20/21, FR-ORG-007, ADR-0090): listFeatureFlags by
// kind (feature/menu/action) and, when a tenant is picked, that tenant's override; columns key (+ menu label), kind,
// description, platform default, override, effective value, protected (`can_disable = false`), version, updated
// at/by. "Sửa" opens the editor (platform default or tenant override, mandatory reason, If-Match). Flags come from
// code and the menu catalog: no create, no delete.
import { Alert, Button, Select, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { MenuKey } from '../../../generated/menu-catalog';
import { ActionGate } from '../../../permissions/ActionGate';
import { menuByKey } from '../../../permissions/menuRegistry';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { FLAG_KINDS, useFeatureFlags, type FeatureFlag, type FeatureFlagKind } from '../api/featureFlags';
import { formatTime, PageTitle, TenantSelect, useLargeScreen } from '../ui';
import { FeatureFlagEditor } from './FeatureFlagEditor';

const onOff = (v: boolean) => (v ? 'platform.featureFlags.on' : 'platform.featureFlags.off');

export function FeatureFlagListPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const large = useLargeScreen();
  const [kind, setKind] = useState<FeatureFlagKind | undefined>();
  const [tenantId, setTenantId] = useState<string | undefined>();
  const list = useFeatureFlags(kind, tenantId);
  const [editing, setEditing] = useState<string | null>(null);
  const current = list.rows.find((f) => f.flag_key === editing) ?? null;

  const columns: ColumnsType<FeatureFlag> = [
    {
      key: 'key',
      title: t('platform.featureFlags.key'),
      render: (_, f) => {
        const menu = f.flag_kind === 'menu' ? menuByKey(f.flag_key as MenuKey) : undefined;
        return (
          <span>
            <Typography.Text code>{f.flag_key}</Typography.Text> {menu ? <Typography.Text type="secondary">{t(menu.labelKey)}</Typography.Text> : null}
          </span>
        );
      },
    },
    { key: 'kind', title: t('platform.featureFlags.kind'), render: (_, f) => <Tag>{t(`platform.featureFlags.kindLabel.${f.flag_kind}`)}</Tag> },
    { key: 'description', title: t('platform.featureFlags.description'), render: (_, f) => f.description },
    { key: 'default', title: t('platform.featureFlags.default'), render: (_, f) => t(onOff(f.default_enabled)) },
    ...(tenantId
      ? [{ key: 'override', title: t('platform.featureFlags.override'), render: (_: unknown, f: FeatureFlag) => (f.tenant_override == null ? t('platform.featureFlags.noOverride') : t(onOff(f.tenant_override))) }]
      : []),
    { key: 'effective', title: t('platform.featureFlags.effective'), render: (_, f) => <Tag color={f.effective_enabled ? 'success' : 'default'}>{t(onOff(f.effective_enabled))}</Tag> },
    { key: 'protected', title: t('platform.featureFlags.protected'), render: (_, f) => (f.can_disable ? null : <Tag color="warning">{t('platform.featureFlags.cannotDisable')}</Tag>) },
    { key: 'version', title: t('platform.featureFlags.version'), render: (_, f) => f.version },
    { key: 'updated', title: t('platform.featureFlags.updated'), render: (_, f) => (f.updated_at ? `${formatTime(f.updated_at)}${f.updated_by ? ` · ${f.updated_by}` : ''}` : null) },
  ];
  if (large) {
    columns.push({
      key: 'actions',
      title: t('platform.common.actions'),
      render: (_, f) => (
        <ActionGate actionKey="action.platform_admin.feature_toggles.update">
          <Button size="small" onClick={() => setEditing(f.flag_key)}>
            {t('platform.common.edit')}
          </Button>
        </ActionGate>
      ),
    });
  }

  return (
    <section>
      <PageTitle title={title} />
      <Space wrap>
        <Select
          aria-label={t('platform.featureFlags.kind')}
          allowClear
          placeholder={t('platform.featureFlags.allKinds')}
          value={kind}
          onChange={setKind}
          options={FLAG_KINDS.map((x) => ({ value: x, label: t(`platform.featureFlags.kindLabel.${x}`) }))}
        />
        <TenantSelect allowClear ariaLabel={t('platform.featureFlags.tenantView')} value={tenantId} onChange={(v) => setTenantId((v as string | undefined) || undefined)} />
        {large ? null : <Alert type="info" showIcon message={t('platform.common.largeScreenReadOnly')} />}
      </Space>
      <Typography.Paragraph type="secondary">{t('platform.featureFlags.fromCode')}</Typography.Paragraph>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="flag_key" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
      {current ? <FeatureFlagEditor flag={current} tenantId={tenantId} onClose={() => setEditing(null)} /> : null}
    </section>
  );
}

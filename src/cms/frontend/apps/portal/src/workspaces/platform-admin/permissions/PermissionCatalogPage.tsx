// Permission catalog, read only (F059 T020, F059-AC-01, FR-CMS-012, ADMIN_PORTAL §2.2): listAllPermissions grouped by
// `group_code`, filtered by `scope_kind` and code text (≤ PermissionCode maxLength), with `catalog_version` and, per
// permission, the roles (first page of listPlatformRoles) that contain it. Permissions are defined in code: there is
// no add/edit/delete here.
import { Collapse, Input, Select, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { groupPermissions, rolesByPermission, type Permission } from '../../../shared/authz/groupPermissions';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { SCOPE_KINDS, usePermissionCatalog, useRoleOptions, type PermissionScopeKind } from '../api/roles';
import { LIMITS } from '../limits';
import { limitInputProps } from '../ui';

export function PermissionCatalogPage() {
  const { t, i18n } = useTranslation();
  const catalog = usePermissionCatalog();
  const roles = useRoleOptions(undefined);
  const [scopeKind, setScopeKind] = useState<PermissionScopeKind | undefined>();
  const [text, setText] = useState('');
  const groups = useMemo(() => groupPermissions(catalog.data?.items ?? [], { scopeKind, text }), [catalog.data, scopeKind, text]);
  const holders = useMemo(() => rolesByPermission(roles.data ?? []), [roles.data]);

  const columns: ColumnsType<Permission> = [
    { key: 'code', title: t('platform.permissions.code'), render: (_, p) => <Typography.Text code>{p.code}</Typography.Text> },
    { key: 'name', title: t('platform.permissions.name'), render: (_, p) => (i18n.language.startsWith('en') ? p.name.en : p.name.vi) },
    { key: 'scope', title: t('platform.permissions.scopeKind'), render: (_, p) => <Tag>{t(`platform.scopeKind.${p.scope_kind}`)}</Tag> },
    {
      key: 'roles',
      title: t('platform.permissions.roles'),
      render: (_, p) => (holders.get(p.code) ?? []).map((c) => <Tag key={c}>{c}</Tag>),
    },
  ];

  return (
    <section aria-label={t('platform.access.tabPermissions')}>
      <Space wrap>
        <Select
          aria-label={t('platform.permissions.scopeKind')}
          allowClear
          placeholder={t('platform.permissions.allScopes')}
          value={scopeKind}
          onChange={setScopeKind}
          options={SCOPE_KINDS.map((x) => ({ value: x, label: t(`platform.scopeKind.${x}`) }))}
        />
        <Input.Search
          aria-label={t('platform.permissions.search')}
          placeholder={t('platform.permissions.search')}
          allowClear
          onSearch={setText}
          {...limitInputProps(LIMITS.permissionSearch)}
        />
        {catalog.data ? <Tag>{t('platform.permissions.catalogVersion', { version: catalog.data.catalog_version })}</Tag> : null}
      </Space>
      <Typography.Paragraph type="secondary">{t('platform.permissions.readOnly')}</Typography.Paragraph>
      {catalog.error ? <ProblemMessage error={catalog.error} /> : null}
      {catalog.data && groups.length ? (
      <Collapse
        key={`${scopeKind ?? 'all'}|${text}`}
        defaultActiveKey={groups.map((g) => g.groupCode)}
        items={groups.map((g) => ({
          key: g.groupCode,
          label: t('authz.permissionGroup', { group: g.groupCode, total: g.items.length }),
          children: <Table rowKey="code" size="small" columns={columns} dataSource={g.items} pagination={false} />,
        }))}
      />
      ) : null}
      {!catalog.isFetching && !groups.length ? <Typography.Text type="secondary">{t('platform.common.empty')}</Typography.Text> : null}
    </section>
  );
}

// PermissionPicker (F059 T019, F059-AC-03): checkable tree of the permission catalog grouped by `group_code`, offering
// only the permissions whose `scope_kind` matches the role (a `tenant` role never sees `platform.*`, `game.*`,
// `competition.config.*`). Read-only view of the catalog (no permission is created here); the value is the list of
// permission codes. A display filter only — the server answers 422 PERMISSION_NOT_ALLOWED_FOR_SCOPE anyway.
// Large catalogs (> VIRTUAL_THRESHOLD items, up to PermissionList.maxItems) render virtualized.
import { Empty, Tree, theme } from 'antd';
import type { DataNode } from 'antd/es/tree';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { groupPermissions, type Permission, type PermissionScopeKind } from './groupPermissions';

const GROUP_PREFIX = 'group:';
const VIRTUAL_THRESHOLD = 200;

export const permissionLabel = (p: Permission, language: string): string => `${p.code} — ${language.startsWith('en') ? p.name.en : p.name.vi}`;

export interface PermissionPickerProps {
  catalog: readonly Permission[];
  scopeKind: PermissionScopeKind;
  value?: string[];
  onChange?: (codes: string[]) => void;
  ariaLabel: string;
  disabled?: boolean;
}

export function PermissionPicker({ catalog, scopeKind, value = [], onChange, ariaLabel, disabled }: PermissionPickerProps) {
  const { t, i18n } = useTranslation();
  const { token } = theme.useToken();
  const groups = useMemo(() => groupPermissions(catalog, { scopeKind }), [catalog, scopeKind]);
  const offered = useMemo(() => new Set(groups.flatMap((g) => g.items.map((p) => p.code))), [groups]);
  const treeData: DataNode[] = groups.map((g) => ({
    key: `${GROUP_PREFIX}${g.groupCode}`,
    title: t('authz.permissionGroup', { group: g.groupCode, total: g.items.length }),
    children: g.items.map((p) => ({ key: p.code, title: permissionLabel(p, i18n.language) })),
  }));
  if (!treeData.length) return <Empty description={t('authz.noPermission')} />;
  const count = groups.reduce((n, g) => n + g.items.length, 0);
  return (
    <div role="group" aria-label={ariaLabel}>
      <Tree
        key={scopeKind}
        checkable
        selectable={false}
        disabled={disabled}
        defaultExpandAll={count <= VIRTUAL_THRESHOLD}
        height={count > VIRTUAL_THRESHOLD ? token.controlHeight * 12 : undefined}
        treeData={treeData}
        checkedKeys={value.filter((c) => offered.has(c))}
        onCheck={(checked) => {
          const keys = (Array.isArray(checked) ? checked : checked.checked).map(String);
          onChange?.(keys.filter((k) => !k.startsWith(GROUP_PREFIX)));
        }}
      />
    </div>
  );
}

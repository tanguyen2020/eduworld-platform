// Permission picker of a role (F052 T034, F052-AC-14, ADMIN_PORTAL §3.17): only permissions returned by
// listTenantPermissions with `scope_kind = tenant`, grouped by `group_code` (a group without permission is not shown);
// permissions the server refused (`422 VALIDATION_FAILED`, `PERMISSION_NOT_ALLOWED_FOR_SCOPE`) are marked in error.
import { Checkbox, Collapse, Space, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { localizedName } from '../common/format';
import { groupPermissions, type Permission } from '../users/canGrant';

export interface PermissionPickerProps {
  permissions: readonly Permission[];
  value: readonly string[];
  onChange?: (codes: string[]) => void;
  invalid?: ReadonlySet<string>;
  disabled?: boolean;
}

export function PermissionPicker({ permissions, value, onChange, invalid = new Set(), disabled }: PermissionPickerProps) {
  const { t, i18n } = useTranslation();
  const groups = groupPermissions(permissions);
  const toggle = (code: string, on: boolean) => onChange?.(on ? [...new Set([...value, code])] : value.filter((c) => c !== code));
  return (
    <Collapse
      defaultActiveKey={groups.map((g) => g.group)}
      data-testid="permission-picker"
      items={groups.map((g) => ({
        key: g.group,
        label: t(`portal.school_lms.roles.groups.${g.group}`, { defaultValue: g.group }),
        children: (
          <Space direction="vertical">
            {g.items.map((p) => (
              <Space key={p.code} direction="vertical" size={0}>
                <Checkbox checked={value.includes(p.code)} disabled={disabled} onChange={(e) => toggle(p.code, e.target.checked)} aria-invalid={invalid.has(p.code) || undefined}>
                  {`${localizedName(p.name, i18n.language)} (${p.code})`}
                </Checkbox>
                {invalid.has(p.code) ? (
                  <Typography.Text type="danger" role="alert">
                    {t('portal.school_lms.fieldErrors.PERMISSION_NOT_ALLOWED_FOR_SCOPE')}
                  </Typography.Text>
                ) : null}
              </Space>
            ))}
          </Space>
        ),
      }))}
    />
  );
}

/** Codes refused by the server: `errors[].field` = `permission_codes.<i>` (or the code itself) with that sub-code. */
export function refusedPermissions(fieldErrors: Record<string, { code: string }>, sent: readonly string[]): Set<string> {
  const out = new Set<string>();
  for (const [field, fe] of Object.entries(fieldErrors)) {
    if (fe.code !== 'PERMISSION_NOT_ALLOWED_FOR_SCOPE') continue;
    const m = /^permission_codes(?:\.|\[)(\d+)\]?$/.exec(field);
    if (m && sent[Number(m[1])]) out.add(sent[Number(m[1])]!);
    else if (sent.includes(field)) out.add(field);
  }
  return out;
}

// ManifestRefPicker (F079-AC-07, ADMIN_PORTAL §3.3): picks a reference of a game config — area, NPC, scene,
// interactable (client manifest of module game) or a published config (quest, item, NPC dialogue…) — from the list the
// caller loaded (every page). Searchable, no free id: there is no "tags" mode, so a typed value that is not in the list
// selects nothing. A stored value missing from the list (removed from the manifest, unpublished) is flagged; the
// server rejects it on submit (422 VALIDATION_FAILED on the field).
import { Alert, Flex } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';
import { SearchSelect } from '../select/SearchSelect';

export interface RefOption {
  value: string;
  label?: string;
}

export interface ManifestRefPickerProps {
  ariaLabel: string;
  options: readonly RefOption[];
  value: string | string[] | undefined;
  onChange: (value: string | string[] | undefined, option?: unknown) => void;
  multiple?: boolean;
  id?: string;
  loading?: boolean;
  truncated?: boolean;
  disabled?: boolean;
  allowClear?: boolean;
}

export function ManifestRefPicker({ ariaLabel, options, value, onChange, multiple, id, loading, truncated, disabled, allowClear = true }: ManifestRefPickerProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const known = new Set(options.map((o) => o.value));
  const values = value === undefined ? [] : Array.isArray(value) ? value : [value];
  const missing = loading ? [] : values.filter((v) => !known.has(v));
  return (
    <Flex vertical gap="small">
      <SearchSelect
        id={id}
        aria-label={ariaLabel}
        mode={multiple ? 'multiple' : undefined}
        allowClear={allowClear}
        loading={loading}
        disabled={disabled}
        truncated={truncated}
        value={value as never}
        onChange={(v, option) => onChange((v ?? undefined) as string | string[] | undefined, option)}
        options={[...options.map((o) => ({ value: o.value, label: o.label ? `${o.label} (${o.value})` : o.value, title: o.label ?? o.value })), ...missing.map((v) => ({ value: v, label: v, disabled: true }))]}
      />
      {missing.length > 0 && <Alert type="warning" showIcon message={t('game.ref.missing', { values: missing.join(', ') })} />}
    </Flex>
  );
}

// Field overrides shared by the game builders (F079-AC-07, AC-12, AC-14): date-times are picked and shown in ICT and
// sent in UTC (shared/release/ict), tenant targets come from the tenant list (every page, searchable), references from
// the client manifest / published content (refs.tsx). Display only.
import type { SchemaFieldContext } from '@eduworld/ops-ui/game-console';
import { ManifestRefPicker } from '@eduworld/ops-ui/game-console';
import { DatePicker, Flex, Typography } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { useTenantOptions } from '../../../shared/api/hooks/useTenantOptions';
import { ictToUtc, utcToIct } from '../../../shared/release/ict';
import { refOverrideFor } from './refs';

/** ICT wall-clock of an ISO instant as a dayjs (fields read as local wall time by the picker). */
export function ictDayjs(iso: string | undefined): Dayjs | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  const w = utcToIct(d);
  return dayjs(new Date(w.year, w.month - 1, w.day, w.hour, w.minute));
}

/** Picked wall-clock (ICT) → UTC ISO. */
export const pickedUtcIso = (d: Dayjs) => ictToUtc({ year: d.year(), month: d.month() + 1, day: d.date(), hour: d.hour(), minute: d.minute() }).toISOString();

export function IctDateTimeField({ ctx }: { ctx: SchemaFieldContext }) {
  const { t } = useTranslation();
  return (
    <Flex vertical gap="small">
      <DatePicker
        id={ctx.id}
        aria-label={ctx.label}
        showTime={{ format: 'HH:mm' }}
        format="DD/MM/YYYY HH:mm"
        needConfirm={false}
        disabled={ctx.readOnly}
        status={ctx.error ? 'error' : undefined}
        value={ictDayjs(typeof ctx.value === 'string' ? ctx.value : undefined)}
        onChange={(d) => ctx.onChange(d ? pickedUtcIso(d) : undefined)}
      />
      <Typography.Text type="secondary">{t('gameOps.field.ictHint')}</Typography.Text>
    </Flex>
  );
}

function TenantIdsField({ ctx }: { ctx: SchemaFieldContext }) {
  const q = useTenantOptions('', true);
  return (
    <ManifestRefPicker
      id={ctx.id}
      ariaLabel={ctx.label}
      multiple
      disabled={ctx.readOnly}
      loading={q.isPending}
      truncated={q.truncated}
      options={q.items.map((x) => ({ value: x.tenant_id, label: x.name }))}
      value={ctx.value as string[] | undefined}
      onChange={(v) => ctx.onChange(Array.isArray(v) && v.length ? v : undefined)}
    />
  );
}

/** The renderOverride of every game builder: ICT date-times, tenant targets, references. */
export function gameOverrides(idField: string) {
  const refs = refOverrideFor(idField);
  return (ctx: SchemaFieldContext): ReactNode | undefined => {
    if (ctx.schema.format === 'date-time') return <IctDateTimeField ctx={ctx} />;
    if (ctx.key === 'tenant_ids') return <TenantIdsField ctx={ctx} />;
    return refs(ctx);
  };
}

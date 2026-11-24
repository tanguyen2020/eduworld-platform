// DiffTable (F054-AC-03, F079-AC-04): the field-by-field changes of a version diff (`ContentVersionDiff.changes`:
// path, op, before, after) as a table — shared by Content Studio (grouped by kind) and the Game Operations Console.
import { Table, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import type { components } from '../api/schema.gen';

export type DiffChange = components['schemas']['ContentVersionDiff']['changes'][number];

/** Value of a diff side as short display text (JSON for objects, capped). */
export function diffValueText(v: unknown, max = 400): string {
  if (v === undefined) return '';
  const s = typeof v === 'string' ? v : JSON.stringify(v);
  return s.length > max ? `${s.slice(0, max)}…` : s;
}

export interface DiffTableProps {
  changes: readonly DiffChange[];
  fromVersion: number;
  toVersion: number;
}

export function DiffTable({ changes, fromVersion, toVersion }: DiffTableProps) {
  const { t } = useTranslation();
  const columns = [
    { key: 'path', title: t('release.diff.field'), render: (_: unknown, c: DiffChange) => <Typography.Text code>{c.path}</Typography.Text> },
    { key: 'op', title: t('release.diff.op'), render: (_: unknown, c: DiffChange) => <Tag>{t(`release.diff.ops.${c.op}`)}</Tag> },
    { key: 'before', title: t('release.diff.before', { version: fromVersion }), render: (_: unknown, c: DiffChange) => <Typography.Text delete={c.op !== 'added'}>{diffValueText(c.before)}</Typography.Text> },
    { key: 'after', title: t('release.diff.after', { version: toVersion }), render: (_: unknown, c: DiffChange) => <Typography.Text mark={c.op !== 'removed'}>{diffValueText(c.after)}</Typography.Text> },
  ];
  return <Table size="small" rowKey={(c) => `${c.path}:${c.op}`} columns={columns} dataSource={[...changes]} pagination={false} scroll={{ x: true }} />;
}

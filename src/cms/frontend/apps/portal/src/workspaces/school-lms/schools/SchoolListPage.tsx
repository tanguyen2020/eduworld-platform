// "Trường" list (F052 T016, F052-AC-01, F052-AC-27): listTenantSchools filtered by status, names in the UI locale; only
// what the API returns is shown (a School Admin receives their school only). Row → detail (relative route).
import { Segmented, Space, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { QueryError } from '../common/errors';
import { localizedName } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { SourceTag, StatusTag } from '../common/ui';
import { useSchoolList, type School, type SchoolStatus } from './useSchools';

const STATUSES: SchoolStatus[] = ['active', 'pending_review', 'disabled'];

export function SchoolListPage() {
  const { t, i18n } = useTranslation();
  const [status, setStatus] = useState<SchoolStatus | 'all'>('all');
  const list = useSchoolList(status === 'all' ? undefined : status);
  const name = (s: School) => localizedName(s.name, i18n.language);
  const columns: ColumnType<School>[] = [
    { key: 'title', title: t('portal.school_lms.schools.name'), render: (_, s) => <Link to={s.school_id}>{name(s)}</Link> },
    { key: 'code', title: t('portal.school_lms.schools.code'), dataIndex: 'code' },
    { key: 'province', title: t('portal.school_lms.schools.province'), render: (_, s) => (s.province ? localizedName(s.province.name, i18n.language) : t('portal.school_lms.schools.noProvince')) },
    { key: 'level', title: t('portal.school_lms.schools.level'), render: (_, s) => t(`portal.school_lms.schools.levels.${s.school_level}`) },
    { key: 'status', title: t('portal.school_lms.schools.status'), render: (_, s) => <StatusTag group="school" value={s.status} /> },
    { key: 'source', title: t('portal.school_lms.common.source'), render: (_, s) => <SourceTag source={s.source} /> },
  ];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Segmented
        aria-label={t('portal.school_lms.schools.status')}
        value={status}
        onChange={(v) => setStatus(v as SchoolStatus | 'all')}
        options={[{ value: 'all', label: t('portal.school_lms.common.all') }, ...STATUSES.map((s) => ({ value: s, label: t(`portal.school_lms.status.school.${s}`) }))]}
      />
      {list.error ? <QueryError error={list.error} /> : null}
      <ResponsiveTable<School>
        rowKey="school_id"
        columns={columns}
        data={list.items}
        loading={list.isFetching}
        cardTitle={(s) => <Link to={s.school_id}>{name(s)}</Link>}
        hasMore={list.hasMore}
        onLoadMore={list.loadMore}
      />
      {!list.isFetching && list.items.length === 0 ? <Typography.Text type="secondary">{t('portal.school_lms.common.empty')}</Typography.Text> : null}
    </Space>
  );
}

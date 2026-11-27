// List of "Sự kiện trường" (F081-AC-06, T015): listSchoolEvents in cursor pages ("Xem thêm"), status filter (in SQL),
// name, template, ICT schedule, audience (schools, grades — no class, ADR-0095), status tag (token colors). System Admin:
// the topbar tenant filter (several tenants, empty = all) becomes `tenant_ids` and a Tenant column is shown; a row opens
// with its tenant. Below 768 px the table is a list of cards (view only).
import { SearchSelect } from '@eduworld/ops-ui';
import { Button, Flex, Form, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { menuByKey } from '../../../permissions/menuRegistry';
import { useTenantOptions } from '../../../shared/api/hooks/useTenantOptions';
import { formatIct } from '../../../shared/release/ict';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { useTenantColumn } from '../../../shell/TenantControls';
import { ProblemMessage } from '../common/errors';
import { localizedName } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { PageTitle } from '../common/ui';
import { LIST_STATUSES, TENANT_PARAM, useEventTemplates, useSchoolEvents, type SchoolEvent, type SchoolEventStatus } from './api';
import { EventStatusTag } from './ui';

/** Path of the "Sự kiện trường" screen, from the generated menu catalog (ADR-0090, F051-AC-34). */
export const SCHOOL_EVENTS_BASE = menuByKey('menu.school_lms.school_events')!.path;

/** Link of one event (System Admin keeps the tenant of the row in the URL). */
export const eventPath = (e: Pick<SchoolEvent, 'event_id' | 'tenant_id'>, crossTenant: boolean, suffix = '') =>
  `${SCHOOL_EVENTS_BASE}/${e.event_id}${suffix}${crossTenant && e.tenant_id ? `?${TENANT_PARAM}=${e.tenant_id}` : ''}`;

export function AudienceText({ audience }: { audience: SchoolEvent['audience'] }) {
  const { t } = useTranslation();
  const grades = audience?.grade_levels ?? [];
  const schools = audience?.school_ids?.length ?? 0;
  const g = grades.length ? t('portal.school_lms.schoolEvents.audience.grades', { list: [...grades].sort((a, b) => a - b).join(', ') }) : t('portal.school_lms.schoolEvents.audience.allGrades');
  const s = schools ? t('portal.school_lms.schoolEvents.audience.schools', { count: schools }) : t('portal.school_lms.schoolEvents.audience.allSchools');
  return (
    <span>
      {s} · {g}
    </span>
  );
}

export function SchoolEventListPage({ title }: { title: string }) {
  const { t, i18n } = useTranslation();
  const cross = useCrossTenantScope();
  const [status, setStatus] = useState<SchoolEventStatus | undefined>();
  const list = useSchoolEvents(status);
  const tenants = useTenantOptions('', cross.crossTenant);
  const tenantColumn = useTenantColumn<SchoolEvent>(new Map((tenants.data ?? []).map((x) => [x.tenant_id, x])));
  // template names of the session tenant (tenant users); System Admin sees the code-less id fallback
  const templates = useEventTemplates({}, 'session', !cross.crossTenant);
  const templateName = useMemo(() => new Map(templates.items.map((x) => [x.template_id, localizedName(x.name, i18n.language)])), [templates.items, i18n.language]);
  const name = (r: SchoolEvent) => (
    <Link to={eventPath(r, cross.crossTenant)}>{localizedName(r.name, i18n.language)}</Link>
  );
  const columns: ColumnType<SchoolEvent>[] = [
    { key: 'name', title: t('portal.school_lms.schoolEvents.list.name'), render: (_, r) => name(r) },
    ...(tenantColumn ? [tenantColumn] : []),
    { key: 'template', title: t('portal.school_lms.schoolEvents.list.template'), render: (_, r) => templateName.get(r.template_id) ?? '' },
    {
      key: 'schedule',
      title: t('portal.school_lms.schoolEvents.list.schedule'),
      render: (_, r) => `${formatIct(r.starts_at)} – ${formatIct(r.ends_at)}`,
    },
    { key: 'audience', title: t('portal.school_lms.schoolEvents.list.audience'), render: (_, r) => <AudienceText audience={r.audience} /> },
    { key: 'status', title: t('portal.school_lms.schoolEvents.list.status'), render: (_, r) => <EventStatusTag status={r.status} /> },
  ];
  return (
    <section>
      <PageTitle
        title={title}
        extra={
          <ActionGate actionKey="action.school_lms.school_events.edit">
            <Link to={`${SCHOOL_EVENTS_BASE}/new`}>
              <Button type="primary">{t('portal.school_lms.schoolEvents.create')}</Button>
            </Link>
          </ActionGate>
        }
      />
      <Flex vertical gap="middle">
        <Typography.Paragraph type="secondary">{t('portal.school_lms.schoolEvents.list.hint')}</Typography.Paragraph>
        <Form layout="inline" name="school_event_filters">
          <Form.Item label={t('portal.school_lms.schoolEvents.list.status')}>
            <SearchSelect<SchoolEventStatus>
              aria-label={t('portal.school_lms.schoolEvents.list.status')}
              allowClear
              placeholder={t('portal.school_lms.schoolEvents.list.allStatuses')}
              value={status}
              onChange={(v?: SchoolEventStatus) => setStatus(v)}
              options={LIST_STATUSES.map((s) => ({ value: s, label: t(`portal.school_lms.schoolEvents.status.${s}`) }))}
              style={{ minWidth: '12rem' }}
            />
          </Form.Item>
        </Form>
        {list.error ? <ProblemMessage error={list.error} /> : null}
        <ResponsiveTable<SchoolEvent>
          rowKey="event_id"
          columns={columns}
          data={list.rows}
          loading={list.isFetching}
          cardTitle={(r) => name(r)}
          hasMore={list.hasNextPage}
          onLoadMore={() => void list.fetchNextPage()}
          caption={title}
        />
        {!list.isFetching && !list.error && list.rows.length === 0 ? <Typography.Text type="secondary">{t('portal.school_lms.schoolEvents.list.empty')}</Typography.Text> : null}
      </Flex>
    </section>
  );
}

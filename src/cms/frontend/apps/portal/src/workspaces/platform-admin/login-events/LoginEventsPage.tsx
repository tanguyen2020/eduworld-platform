// Menu "Hoạt động đăng nhập game (mọi tenant)" (F059 US5, F059-AC-18/19, FR-GAME-013, ADR-0031): login events of
// students across tenants — tenant filter (`tenant_ids[]`), student id, `from`/`to` (default last 7 days); columns
// tenant, student, platform, device label, client version, in/out, `end_reason` (Vietnamese label). Cursor paging
// ("Tải thêm"); no IP anywhere (not in the contract either).
import { Button, DatePicker, Form, Input, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs, { type Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TenantFilter } from '../../../shell/TenantControls';
import { endReasonKey, useLoginEvents, type LoginEvent } from '../api/loginEvents';
import { useTenantDirectory } from '../api/tenants';
import { isUuid, LOGIN_EVENTS_DEFAULT_DAYS } from '../limits';
import { formatTime, PageTitle, TenantCell } from '../ui';

interface FormValues {
  range: [Dayjs, Dayjs];
  student_id?: string;
}

export function LoginEventsPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const [form] = Form.useForm<FormValues>();
  const [filter, setFilter] = useState(() => {
    const now = dayjs();
    return { from: now.subtract(LOGIN_EVENTS_DEFAULT_DAYS, 'day').toISOString(), to: now.toISOString(), studentId: undefined as string | undefined };
  });
  const list = useLoginEvents(filter);
  const directory = useTenantDirectory();

  const columns: ColumnsType<LoginEvent> = [
    { key: 'tenant', title: t('platform.common.tenant'), render: (_, r) => <TenantCell tenantId={r.tenant_id} directory={directory} /> },
    { key: 'student', title: t('platform.loginEvents.student'), render: (_, r) => <Typography.Text code>{r.student_id}</Typography.Text> },
    { key: 'platform', title: t('platform.loginEvents.platform'), render: (_, r) => t(`platform.loginEvents.platformKind.${r.platform}`) },
    { key: 'device', title: t('platform.loginEvents.device'), render: (_, r) => r.device_label },
    { key: 'version', title: t('platform.loginEvents.clientVersion'), render: (_, r) => r.client_version },
    { key: 'in', title: t('platform.loginEvents.loginAt'), render: (_, r) => formatTime(r.login_at) },
    { key: 'out', title: t('platform.loginEvents.logoutAt'), render: (_, r) => formatTime(r.logout_at) },
    { key: 'end', title: t('platform.loginEvents.endReasonLabel'), render: (_, r) => <Tag>{t(endReasonKey(r.end_reason))}</Tag> },
  ];

  return (
    <section>
      <PageTitle title={title} />
      <Form
        form={form}
        name="login_events"
        layout="inline"
        initialValues={{ range: [dayjs(filter.from), dayjs(filter.to)] }}
        onFinish={(v) => setFilter({ from: v.range[0].toISOString(), to: v.range[1].toISOString(), studentId: v.student_id?.trim() || undefined })}
      >
        <Form.Item name="range" label={t('platform.audit.range')} rules={[{ required: true, message: t('platform.audit.rangeRequired') }]}>
          <DatePicker.RangePicker showTime format="DD/MM/YYYY HH:mm" allowClear={false} />
        </Form.Item>
        <Form.Item
          name="student_id"
          label={t('platform.loginEvents.studentId')}
          rules={[{ validator: (_, v?: string) => (!v?.trim() || isUuid(v.trim()) ? Promise.resolve() : Promise.reject(new Error(t('platform.common.uuidInvalid')))) }]}
        >
          <Input allowClear maxLength={36} />
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit">
            {t('platform.audit.apply')}
          </Button>
        </Form.Item>
      </Form>
      <Space wrap>
        <TenantFilter />
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table rowKey="login_event_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('platform.common.loadMore')}
        </Button>
      ) : null}
    </section>
  );
}

// Screen of `menu.school_lms.chat_safety` (F122 US8, US6; F122-AC-17, AC-21, AC-22, AC-28): "Chính sách chat" of a school
// (ChatPolicyPanel) and "Nhóm từ" of the tenant, read only — name, description, severity, state; no term of the platform
// catalogue and no write button (the Safety Team changes them on request, ADR-0072). Each tab shows only when the user
// holds its permission (display only; the backend authorizes every call).
import { Table, Tabs, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '../../../permissions/usePermissions';
import { SafetyError } from '../../../shared/chatSafety/ui';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { useTenantCategories, type TenantCategory } from './api';
import { ChatPolicyPanel } from './ChatPolicyPanel';

function CategoriesReadOnly() {
  const { t } = useTranslation();
  const list = useTenantCategories(true);
  const columns: ColumnsType<TenantCategory> = [
    { key: 'name', title: t('chatSafety.categories.name'), render: (_, r) => r.name },
    { key: 'description', title: t('chatSafety.categories.description'), render: (_, r) => r.description ?? '' },
    { key: 'severity', title: t('chatSafety.categories.severity'), render: (_, r) => t(`chatSafety.termSeverity.${r.severity}`) },
    {
      key: 'state',
      title: t('chatSafety.categories.inTenant'),
      render: (_, r) => (r.enabled_platform && r.enabled_in_tenant ? <Tag color="success">{t('chatSafety.common.on')}</Tag> : <Tag>{t('chatSafety.common.off')}</Tag>),
    },
  ];
  return (
    <>
      {list.error ? <SafetyError error={list.error} /> : null}
      <Table rowKey="code" columns={columns} dataSource={list.data?.data ?? []} loading={list.isFetching} pagination={false} scroll={{ x: true }} />
    </>
  );
}

export function ChatSafetyScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const has = (p: string) => !!view?.permissions.includes(p);
  const items = [
    ...(has('school.settings.read') || has('school.settings.write') ? [{ key: 'policy', label: t('chatSafety.policy.tab'), children: <ChatPolicyPanel /> }] : []),
    ...(has('chat.sensitive_terms.read') ? [{ key: 'categories', label: t('chatSafety.categories.tab'), children: <CategoriesReadOnly /> }] : []),
  ];
  return (
    <section>
      <PageTitle title={title} />
      <SingleTenantGate>{() => <Tabs items={items} />}</SingleTenantGate>
    </section>
  );
}

// Menu "Gán cho tenant · Clone giữa tenant" (F057 US2/US3, ADMIN_PORTAL §3.7): tab "Gán cho tenant" — pick a platform
// record (type + id, or `?resource_type=&resource_id=` when opened from a content list of workspace A/B) and manage its
// tenants (AssignedTenantsTab); tab "Clone giữa tenant" — CloneWizard. Each tab sits behind its catalog action.
// Assign and clone need a large screen (ADMIN_PORTAL §1.5): smaller screens get a read-only notice.
import { Alert, Button, Form, Input, Select, Space, Tabs } from 'antd';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import type { ActionKey } from '../../../generated/menu-catalog';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { ASSIGNABLE_TYPES } from '../api/assignments';
import type { ResourceType } from '../api/common';
import { AssignedTenantsTab } from '../assignments/AssignedTenantsTab';
import { CloneWizard } from '../clones/CloneWizard';
import { isUuid } from '../limits';
import { PageTitle, useLargeScreen } from '../ui';

interface RecordRef {
  resourceType: ResourceType;
  resourceId: string;
}

function AssignPanel() {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const fromUrl = useMemo<RecordRef | null>(() => {
    const rt = params.get('resource_type') as ResourceType | null;
    const id = params.get('resource_id');
    return rt && ASSIGNABLE_TYPES.includes(rt) && id && isUuid(id) ? { resourceType: rt, resourceId: id } : null;
  }, [params]);
  const [form] = Form.useForm<{ resource_type: ResourceType; resource_id: string }>();
  return (
    <Space direction="vertical">
      <Form
        form={form}
        name="assign_record"
        layout="inline"
        initialValues={fromUrl ? { resource_type: fromUrl.resourceType, resource_id: fromUrl.resourceId } : {}}
        onFinish={(v) => setParams({ resource_type: v.resource_type, resource_id: v.resource_id.trim() })}
      >
        <Form.Item name="resource_type" label={t('platform.tenantData.resourceType')} rules={[{ required: true }]}>
          <Select aria-label={t('platform.tenantData.resourceType')} options={ASSIGNABLE_TYPES.map((x) => ({ value: x, label: t(`platform.resourceType.${x}`) }))} />
        </Form.Item>
        <Form.Item
          name="resource_id"
          label={t('platform.tenantData.resourceId')}
          rules={[{ required: true }, { validator: (_, v?: string) => (!v || isUuid(v.trim()) ? Promise.resolve() : Promise.reject(new Error(t('platform.common.uuidInvalid')))) }]}
        >
          <Input aria-label={t('platform.tenantData.resourceId')} maxLength={36} showCount />
        </Form.Item>
        <Button htmlType="submit">{t('platform.tenantData.show')}</Button>
      </Form>
      <Alert type="info" message={t('platform.tenantData.pickRecordHelp')} />
      {fromUrl ? <AssignedTenantsTab key={`${fromUrl.resourceType}:${fromUrl.resourceId}`} resourceType={fromUrl.resourceType} resourceId={fromUrl.resourceId} /> : null}
    </Space>
  );
}

export function TenantDataPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const large = useLargeScreen();
  const [tab, setTab] = useState('assign');
  const { view } = usePermissions();
  // tabs follow the catalog actions (F057-AC-22): a turned-off or forbidden action hides its tab
  const enabled = (k: ActionKey) => !!view && actionStatus(k, view) === 'enabled';
  const items = [
    enabled('action.platform_admin.tenant_data.view') ? { key: 'assign', label: t('platform.tenantData.assignTab'), children: <AssignPanel /> } : null,
    enabled('action.platform_admin.tenant_data.clone') ? { key: 'clone', label: t('platform.tenantData.cloneTab'), children: large ? <CloneWizard /> : null } : null,
  ].filter((x) => x !== null);
  return (
    <section>
      <PageTitle title={title} />
      {large ? null : <Alert type="info" showIcon message={t('platform.common.largeScreenReadOnly')} />}
      <Tabs activeKey={items.some((x) => x.key === tab) ? tab : items[0]?.key} onChange={setTab} items={items} />
    </section>
  );
}

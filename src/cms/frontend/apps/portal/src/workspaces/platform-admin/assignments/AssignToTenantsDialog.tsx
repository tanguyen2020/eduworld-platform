// "Gán cho tenant" dialog (F057 T021, F057-AC-05, ADMIN_PORTAL §3.7): pick several tenants (listTenants search,
// ≤ maxItems of TenantAssignmentRequest.tenant_ids) or the [Proposed] options "mọi tenant hiện tại" / "tenant tạo
// sau", then assignToTenants with one Idempotency-Key per intent (kept on retry). Exported for workspaces A/B, whose
// platform lists open it on a record. Platform data → no X-Tenant-Id.
import { Alert, App as AntApp, Checkbox, Form, Modal } from 'antd';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useAssign } from '../api/assignments';
import type { ResourceType } from '../api/common';
import { LIMITS } from '../limits';
import { TenantSelect } from '../ui';

export interface AssignToTenantsDialogProps {
  resourceType: ResourceType;
  resourceId: string;
  open: boolean;
  onClose: () => void;
}

interface Values {
  tenant_ids?: string[];
  apply_to_all_tenants?: boolean;
  apply_to_new_tenants?: boolean;
}

export function AssignToTenantsDialog({ resourceType, resourceId, open, onClose }: AssignToTenantsDialogProps) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<Values>();
  const assign = useAssign();
  const allTenants = Form.useWatch('apply_to_all_tenants', form);

  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const tenantIds = v.apply_to_all_tenants ? [] : (v.tenant_ids ?? []);
    assign.mutate(
      {
        resource_type: resourceType,
        resource_id: resourceId,
        ...(tenantIds.length ? { tenant_ids: tenantIds } : {}),
        apply_to_all_tenants: !!v.apply_to_all_tenants,
        apply_to_new_tenants: !!v.apply_to_new_tenants,
      },
      {
        onSuccess: (res) => {
          message.success(t('platform.assign.done', { count: res.data.length }));
          form.resetFields();
          onClose();
        },
      },
    );
  };

  return (
    <Modal
      open={open}
      title={t('platform.assign.title')}
      okText={t('platform.common.confirm')}
      cancelText={t('platform.common.cancel')}
      confirmLoading={assign.isPending}
      onOk={submit}
      onCancel={() => {
        assign.reset();
        onClose();
      }}
      destroyOnHidden
    >
      <Form form={form} name="assign_tenants" layout="vertical" preserve={false}>
        <Form.Item
          name="tenant_ids"
          label={t('platform.assign.tenants')}
          dependencies={['apply_to_all_tenants']}
          rules={[
            ({ getFieldValue }) => ({
              validator: (_, v?: string[]) =>
                getFieldValue('apply_to_all_tenants') || (v && v.length > 0) ? Promise.resolve() : Promise.reject(new Error(t('platform.assign.tenantsRequired'))),
            }),
          ]}
        >
          <TenantSelect multiple ariaLabel={t('platform.assign.tenants')} maxCount={LIMITS.assignTenantIds.maxItems} />
        </Form.Item>
        <Form.Item name="apply_to_all_tenants" valuePropName="checked">
          <Checkbox>{t('platform.assign.allTenants')}</Checkbox>
        </Form.Item>
        <Form.Item name="apply_to_new_tenants" valuePropName="checked">
          <Checkbox>{t('platform.assign.newTenants')}</Checkbox>
        </Form.Item>
        {allTenants ? <Alert type="info" showIcon message={t('platform.assign.proposedNote')} /> : null}
      </Form>
      {assign.error ? <ProblemMessage error={assign.error} /> : null}
    </Modal>
  );
}

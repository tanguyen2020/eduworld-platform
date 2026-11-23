// Screen of `menu.live_ops.sanctions` (F122 US1, F122-AC-03): sanctions across tenants (module tenant filter, student id,
// active only), cursor pages; apply a sanction (X-Tenant-Id = tenant of the student, reason 3–1000 with a counter,
// `account_lock` needs a second confirmation) and revoke one (reason). Idempotency-Key per user intent; the server
// decides durations and every rule (AGENTS R1, R10).
import { Alert, Button, Checkbox, Flex, Form, Input, InputNumber, Modal, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect } from '@eduworld/ops-ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { LIMITS, SafetyError, TenantName } from '../../../shared/chatSafety/ui';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { useCreateSanction, useRevokeSanction, useSanctions, type Sanction, type SanctionCreate } from './api';
import { formatTime, ModerationTargetTenant, ModerationTenantFilter, useModerationTenantNames } from './CasesScreen';

const TYPES: SanctionCreate['sanction_type'][] = ['warning', 'mute', 'restrict_text', 'restrict_dm', 'restrict_multiplayer', 'restrict_competition', 'account_lock'];
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

interface CreateForm {
  tenant_id: string;
  user_id: string;
  sanction_type: SanctionCreate['sanction_type'];
  duration_minutes?: number;
  reason: string;
}

function CreateSanctionDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const [form] = Form.useForm<CreateForm>();
  const create = useCreateSanction();
  const [pending, setPending] = useState<CreateForm | null>(null);
  const [understood, setUnderstood] = useState(false);
  const send = (v: CreateForm) =>
    create.mutate(
      { tenantId: v.tenant_id, body: { user_id: v.user_id.trim(), sanction_type: v.sanction_type, reason: v.reason.trim(), ...(v.duration_minutes ? { duration_minutes: v.duration_minutes } : {}) } },
      { onSuccess: onClose },
    );
  const uuidRule = { validator: (_: unknown, v?: string) => (!v || UUID.test(v.trim()) ? Promise.resolve() : Promise.reject(new Error(t('chatSafety.common.uuidInvalid')))) };
  return (
    <Modal open title={t('chatSafety.sanctions.create')} onCancel={onClose} footer={null} destroyOnHidden>
      <Form
        form={form}
        layout="vertical"
        name="create_sanction"
        onFinish={(v) => {
          if (v.sanction_type === 'account_lock') {
            setUnderstood(false);
            setPending(v);
          } else send(v);
        }}
      >
        <Form.Item name="tenant_id" label={t('chatSafety.common.targetTenant')} rules={[{ required: true, message: t('chatSafety.common.targetTenantRequired') }]}>
          <ModerationTenantFilterSingle />
        </Form.Item>
        <Form.Item name="user_id" label={t('chatSafety.sanctions.student')} rules={[{ required: true, message: t('chatSafety.sanctions.studentRequired') }, uuidRule]}>
          <Input maxLength={LIMITS.userId.maxLength} />
        </Form.Item>
        <Form.Item name="sanction_type" label={t('chatSafety.case.sanctionType')} rules={[{ required: true, message: t('chatSafety.case.sanctionTypeRequired') }]}>
          <SearchSelect aria-label={t('chatSafety.case.sanctionType')} options={TYPES.map((s) => ({ value: s, label: t(`chatSafety.sanctionType.${s}`) }))} />
        </Form.Item>
        <Form.Item name="duration_minutes" label={t('chatSafety.case.duration')}>
          <InputNumber min={1} max={525600} aria-label={t('chatSafety.case.duration')} />
        </Form.Item>
        <Form.Item name="reason" label={t('chatSafety.sanctions.reason')} rules={openapiRules(LIMITS.sanctionReason, { required: true, t: ops.t })}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...openapiInputProps(LIMITS.sanctionReason)} />
        </Form.Item>
        {create.error ? <ProblemMessage error={create.error} /> : null}
        <Button type="primary" htmlType="submit" loading={create.isPending}>
          {t('chatSafety.sanctions.apply')}
        </Button>
      </Form>
      <Modal
        open={!!pending}
        title={t('chatSafety.case.lockConfirmTitle')}
        okText={t('chatSafety.case.lockConfirm')}
        okButtonProps={{ danger: true, disabled: !understood }}
        cancelText={t('chatSafety.common.cancel')}
        onCancel={() => setPending(null)}
        onOk={() => {
          if (pending) send(pending);
          setPending(null);
        }}
      >
        <Typography.Paragraph>{t('chatSafety.case.lockConfirmText')}</Typography.Paragraph>
        <Checkbox checked={understood} onChange={(e) => setUnderstood(e.target.checked)}>
          {t('chatSafety.case.lockUnderstood')}
        </Checkbox>
      </Modal>
    </Modal>
  );
}

/** One target tenant (antd Form control): the module tenant filter limited to one value. */
export function ModerationTenantFilterSingle({ value, onChange }: { value?: string; onChange?: (id: string | undefined) => void }) {
  return <ModerationTargetTenant value={value} onChange={onChange} />;
}

export function SanctionsScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  const [tenantIds, setTenantIds] = useState<string[]>([]);
  const [userId, setUserId] = useState<string | undefined>();
  const [active, setActive] = useState(true);
  const list = useSanctions({ tenantIds, userId, active: active || undefined });
  const names = useModerationTenantNames();
  const revoke = useRevokeSanction();
  const [creating, setCreating] = useState(false);
  const [revoking, setRevoking] = useState<Sanction | null>(null);
  const columns: ColumnsType<Sanction> = [
    { key: 'tenant', title: t('chatSafety.common.tenant'), render: (_, r) => <TenantName id={r.tenant_id} names={names} /> },
    { key: 'student', title: t('chatSafety.sanctions.student'), render: (_, r) => <Typography.Text code>{r.user_id}</Typography.Text> },
    { key: 'type', title: t('chatSafety.case.sanctionType'), render: (_, r) => <Tag>{t(`chatSafety.sanctionType.${r.sanction_type}`)}</Tag> },
    { key: 'by', title: t('chatSafety.sanctions.issuedBy'), render: (_, r) => t(`chatSafety.sanctions.by.${r.issued_by_kind}`) },
    { key: 'from', title: t('chatSafety.sanctions.from'), render: (_, r) => formatTime(r.starts_at) },
    { key: 'to', title: t('chatSafety.sanctions.to'), render: (_, r) => formatTime(r.ends_at) },
    { key: 'state', title: t('chatSafety.sanctions.state'), render: (_, r) => (r.revoked_at ? <Tag>{t('chatSafety.sanctions.revoked')}</Tag> : <Tag color="processing">{t('chatSafety.sanctions.active')}</Tag>) },
    {
      key: 'actions',
      title: t('chatSafety.common.actions'),
      render: (_, r) =>
        r.revoked_at ? null : (
          <ActionGate actionKey="action.live_ops.sanctions.write">
            <Button size="small" onClick={() => setRevoking(r)}>
              {t('chatSafety.sanctions.revoke')}
            </Button>
          </ActionGate>
        ),
    },
  ];
  return (
    <section>
      <Flex justify="space-between" align="center" wrap gap="small">
        <Typography.Title level={1}>{title}</Typography.Title>
        <ActionGate actionKey="action.live_ops.sanctions.write">
          <Button type="primary" onClick={() => setCreating(true)}>
            {t('chatSafety.sanctions.create')}
          </Button>
        </ActionGate>
      </Flex>
      <Form layout="inline" name="sanction_filters">
        <Form.Item label={t('chatSafety.common.tenantFilter')}>
          <ModerationTenantFilter value={tenantIds} onChange={setTenantIds} />
        </Form.Item>
        <Form.Item label={t('chatSafety.sanctions.student')}>
          <Input.Search aria-label={t('chatSafety.sanctions.student')} allowClear maxLength={LIMITS.userId.maxLength} onSearch={(v) => setUserId(UUID.test(v.trim()) ? v.trim() : undefined)} />
        </Form.Item>
        <Form.Item>
          <Checkbox checked={active} onChange={(e) => setActive(e.target.checked)}>
            {t('chatSafety.sanctions.activeOnly')}
          </Checkbox>
        </Form.Item>
      </Form>
      {list.error ? <SafetyError error={list.error} /> : null}
      {revoke.error ? <Alert type="error" showIcon message={<ProblemMessage error={revoke.error} />} /> : null}
      <Table rowKey="sanction_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} scroll={{ x: true }} locale={{ emptyText: t('chatSafety.common.empty') }} />
      {list.hasNextPage ? (
        <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('chatSafety.common.loadMore')}
        </Button>
      ) : null}
      {creating ? <CreateSanctionDialog onClose={() => setCreating(false)} /> : null}
      <ReasonDialog
        open={!!revoking}
        title={t('chatSafety.sanctions.revoke')}
        maxLength={LIMITS.revokeReason.maxLength}
        minLength={LIMITS.revokeReason.minLength}
        confirmLoading={revoke.isPending}
        onCancel={() => setRevoking(null)}
        onSubmit={(reason) => revoking && revoke.mutate({ sanction: revoking, reason }, { onSuccess: () => setRevoking(null) })}
      />
    </section>
  );
}

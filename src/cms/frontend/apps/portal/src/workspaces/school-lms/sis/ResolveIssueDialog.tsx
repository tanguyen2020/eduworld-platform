// Resolve one sync conflict (F052 T026, F052-AC-10, F052-AC-11): "Đã sửa ở nguồn" or "Bỏ qua", reason required
// (SchoolReconciliationResolve.reason 3–500, counter), resolveSchoolReconciliationIssue with one Idempotency-Key per
// intent: confirming twice or retrying after a network error with the same choice reuses it.
import { App, Form, Input, Modal, Radio } from 'antd';
import { useTranslation } from 'react-i18next';
import { applyFormError, writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { useResolveIssue, type ReconciliationIssue, type ResolveBody } from './useReconciliation';

export function ResolveIssueDialog({ schoolId, issue, onClose }: { schoolId: string; issue: ReconciliationIssue; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<ResolveBody>();
  const resolve = useResolveIssue(schoolId);
  const field = useFieldRules();
  const ok = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    try {
      await resolve.mutateAsync({ issueId: issue.issue_id, body: { resolution: v.resolution, reason: v.reason.trim() } });
      message.success(t('portal.school_lms.recon.resolved'));
      onClose();
    } catch (e) {
      if (!applyFormError(form, e, ['resolution', 'reason'], t, i18n.exists.bind(i18n))) message.error(t(writeErrorKey(e)));
    }
  };
  return (
    <Modal
      open
      title={t('portal.school_lms.recon.resolveTitle', { type: t(`portal.school_lms.recon.types.${issue.issue_type}`) })}
      okText={t('portal.school_lms.recon.resolve')}
      cancelText={t('portal.school_lms.common.cancel')}
      confirmLoading={resolve.isPending}
      onOk={ok}
      onCancel={onClose}
      destroyOnHidden
    >
      <Form<ResolveBody> form={form} layout="vertical" initialValues={{ resolution: 'fixed_in_source' }} preserve={false}>
        <Form.Item name="resolution" label={t('portal.school_lms.recon.resolution')} rules={[{ required: true }]}>
          <Radio.Group
            options={(['fixed_in_source', 'ignore'] as const).map((r) => ({ value: r, label: t(`portal.school_lms.recon.resolutions.${r}`) }))}
          />
        </Form.Item>
        <Form.Item name="reason" label={t('portal.school_lms.common.reason')} rules={field.rules(FIELDS.resolveReason, true)}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...field.input(FIELDS.resolveReason)} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

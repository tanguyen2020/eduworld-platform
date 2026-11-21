// Resolve one sync conflict (F052 T026, T061, F052-AC-10, F052-AC-11): "Đã sửa ở nguồn", "Bỏ qua" or — on a duplicate issue not
// yet proposed — "Đề nghị gộp tài khoản" (merge_requested, OQ F045-7: the issue stays open, a System Admin merges), reason required
// (SchoolReconciliationResolve.reason 3–500, counter), resolveSchoolReconciliationIssue with one Idempotency-Key per
// intent: confirming twice or retrying after a network error with the same choice reuses it.
import { Alert, App, Form, Input, Modal, Radio } from 'antd';
import { useTranslation } from 'react-i18next';
import { applyFormError, writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { useResolveIssue, type ReconciliationIssue, type ResolveBody } from './useReconciliation';

/** Issue types a School Admin may propose to merge (F011 T041). */
const MERGEABLE = new Set(['duplicate_identifier', 'possible_duplicate']);

/** Resolutions offered for an issue: merge_requested only on a duplicate issue not proposed yet. */
export function resolutionsFor(issue: ReconciliationIssue): ResolveBody['resolution'][] {
  const out: ResolveBody['resolution'][] = ['fixed_in_source', 'ignore'];
  if (MERGEABLE.has(issue.issue_type) && issue.resolution !== 'merge_requested') out.push('merge_requested');
  return out;
}

export function ResolveIssueDialog({ schoolId, issue, onClose }: { schoolId: string; issue: ReconciliationIssue; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<ResolveBody>();
  const resolve = useResolveIssue(schoolId);
  const field = useFieldRules();
  const choice = Form.useWatch('resolution', form);
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
            options={resolutionsFor(issue).map((r) => ({ value: r, label: t(`portal.school_lms.recon.resolutions.${r}`) }))}
          />
        </Form.Item>
        {choice === 'merge_requested' ? <Alert type="info" showIcon message={t('portal.school_lms.recon.mergeHint')} /> : null}
        <Form.Item name="reason" label={t('portal.school_lms.common.reason')} rules={field.rules(FIELDS.resolveReason, true)}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...field.input(FIELDS.resolveReason)} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

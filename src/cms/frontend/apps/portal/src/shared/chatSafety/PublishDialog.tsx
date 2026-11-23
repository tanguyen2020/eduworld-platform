// Publish dialog of a sensitive-term catalogue (F122-AC-08, F122-AC-15): mandatory change summary (5–1000 characters,
// counter); for a tenant catalogue the summary must carry the tenant's request code (ADR-0072: the Safety Team writes on the
// tenant's request). Shared by workspace E (tenant terms) and D (platform catalogue).
import { Form, Input, Modal } from 'antd';
import { openapiInputProps, openapiRules, OPS_UI_NS } from '@eduworld/ops-ui';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../shell/ProblemMessage';
import { LIMITS } from './ui';

/** Request code of the tenant in the publish summary, e.g. "YC-2026-012". */
export const REQUEST_CODE = /\bYC-[A-Z0-9-]{3,}\b/;

/** Publish dialog: mandatory change summary (5–1000; tenant catalogue: must carry the tenant's request code). */
export function PublishDialog({ open, requireRequestCode, loading, error, onCancel, onSubmit }: {
  open: boolean;
  requireRequestCode?: boolean;
  loading?: boolean;
  error?: unknown;
  onCancel: () => void;
  onSubmit: (summary: string) => void;
}) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const [form] = Form.useForm<{ change_summary: string }>();
  const codeRule = {
    validator: (_: unknown, v?: string) => (!requireRequestCode || !v || REQUEST_CODE.test(v) ? Promise.resolve() : Promise.reject(new Error(t('chatSafety.publish.requestCodeRequired')))),
  };
  return (
    <Modal
      open={open}
      title={t('chatSafety.publish.title')}
      okText={t('chatSafety.publish.button')}
      cancelText={t('chatSafety.common.cancel')}
      confirmLoading={loading}
      destroyOnHidden
      onCancel={onCancel}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (v) onSubmit(v.change_summary.trim());
      }}
    >
      <Form form={form} layout="vertical" name="publish_catalog" preserve={false}>
        <Form.Item
          name="change_summary"
          label={t('chatSafety.publish.summary')}
          extra={requireRequestCode ? t('chatSafety.publish.requestCodeHelp') : undefined}
          rules={[...openapiRules(LIMITS.changeSummary, { required: true, t: ops.t }), codeRule]}
        >
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...openapiInputProps(LIMITS.changeSummary)} />
        </Form.Item>
      </Form>
      {error ? <ProblemMessage error={error} /> : null}
    </Modal>
  );
}

// ReasonDialog (F051 T050): mandatory reason of a sensitive action (feature toggles, sanctions, restores…) with the
// `maxLength` of the OpenAPI field and a character counter (ops-ui CharCounter rules, ADR-0041). For F052–F059.
import { Form, Input, Modal } from 'antd';
import { useTranslation } from 'react-i18next';

export interface ReasonDialogProps {
  open: boolean;
  title: string;
  /** `maxLength` of the reason field in OpenAPI (e.g. 1000 for feature toggles). */
  maxLength: number;
  confirmLoading?: boolean;
  onSubmit: (reason: string) => void;
  onCancel: () => void;
}

export function ReasonDialog({ open, title, maxLength, confirmLoading, onSubmit, onCancel }: ReasonDialogProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<{ reason: string }>();
  return (
    <Modal
      open={open}
      title={title}
      okText={t('shell.reason.ok')}
      cancelText={t('shell.reason.cancel')}
      confirmLoading={confirmLoading}
      destroyOnHidden
      onCancel={onCancel}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (v) onSubmit(v.reason.trim());
      }}
    >
      <Form form={form} layout="vertical" preserve={false} requiredMark={false}>
        <Form.Item name="reason" label={t('shell.reason.label')} rules={[{ required: true, whitespace: true, message: t('shell.reason.required') }, { max: maxLength }]}>
          <Input.TextArea maxLength={maxLength} showCount autoSize={{ minRows: 3, maxRows: 8 }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

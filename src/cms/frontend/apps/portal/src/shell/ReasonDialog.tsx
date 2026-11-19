// ReasonDialog (F051 T050): mandatory reason of a sensitive action (feature toggles, sanctions, restores…) with the
// `maxLength` (and `minLength`) of the OpenAPI field and a character counter (ops-ui CharCounter rules, ADR-0041).
// F057 adds the consequence text above the field and the error of the last attempt inside the dialog. For F052–F059.
import { Form, Input, Modal, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

export interface ReasonDialogProps {
  open: boolean;
  title: string;
  /** `maxLength` of the reason field in OpenAPI (e.g. 1000 for feature toggles). */
  maxLength: number;
  /** `minLength` of the reason field when the schema has one (e.g. 3 for restoreTrashItem). */
  minLength?: number;
  /** Consequence of the action shown above the field (disable a tenant, unassign…). */
  description?: ReactNode;
  /** Error of the last attempt (e.g. a ProblemMessage), shown inside the dialog. */
  error?: ReactNode;
  confirmLoading?: boolean;
  onSubmit: (reason: string) => void;
  onCancel: () => void;
}

export function ReasonDialog({ open, title, maxLength, minLength, description, error, confirmLoading, onSubmit, onCancel }: ReasonDialogProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<{ reason: string }>();
  const min = minLength ? [{ min: minLength, message: t('shell.reason.minLength', { min: minLength }) }] : [];
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
      {description ? <Typography.Paragraph>{description}</Typography.Paragraph> : null}
      <Form form={form} name="reason_dialog" layout="vertical" preserve={false} requiredMark={false}>
        <Form.Item name="reason" label={t('shell.reason.label')} rules={[{ required: true, whitespace: true, message: t('shell.reason.required') }, { max: maxLength }, ...min]}>
          <Input.TextArea maxLength={maxLength} showCount autoSize={{ minRows: 3, maxRows: 8 }} />
        </Form.Item>
      </Form>
      {error}
    </Modal>
  );
}

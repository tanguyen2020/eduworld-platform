// Mandatory reason of a sensitive write (revoke UserRole, save/delete role, exclude from cleanup…) with the OpenAPI
// limits of that reason (min/max length, counter — ADR-0041, F052-AC-26). Like shell/ReasonDialog plus `minLength`.
import { Form, Input, Modal, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { FieldSchema } from './fieldSchemas';
import { useFieldRules } from './ui';

export interface ReasonModalProps {
  open: boolean;
  title: string;
  schema: FieldSchema;
  okText?: string;
  danger?: boolean;
  confirmLoading?: boolean;
  children?: ReactNode;
  onSubmit: (reason: string) => void;
  onCancel: () => void;
}

export function ReasonModal({ open, title, schema, okText, danger, confirmLoading, children, onSubmit, onCancel }: ReasonModalProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<{ reason: string }>();
  const field = useFieldRules();
  return (
    <Modal
      open={open}
      title={title}
      okText={okText ?? t('shell.reason.ok')}
      cancelText={t('shell.reason.cancel')}
      okButtonProps={{ danger }}
      confirmLoading={confirmLoading}
      destroyOnHidden
      onCancel={onCancel}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (v) onSubmit(v.reason.trim());
      }}
    >
      {children ? <Typography.Paragraph>{children}</Typography.Paragraph> : null}
      <Form form={form} layout="vertical" preserve={false} requiredMark={false}>
        <Form.Item name="reason" label={t('shell.reason.label')} rules={field.rules(schema, true)}>
          <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} {...field.input(schema)} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

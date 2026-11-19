// ReleaseReasonDialog (F054-AC-07, AC-08, AC-09, AC-19, AC-29): confirm dialog of a release change that needs a reason
// (unpublish, archive, delete): consequence text, "where it is used" list (getContentUsages), reason capped by the
// OpenAPI maxLength with a counter, warning-colored confirm button, error of the last attempt inside the dialog.
import { Flex, Form, Input, Modal, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { UsageList } from '../usage/UsageTab';
import type { ItemRef } from './useRelease';

export interface ReleaseReasonDialogProps {
  open: boolean;
  title: string;
  okText: string;
  description?: ReactNode;
  item?: ItemRef;
  maxLength: number;
  /** false: the reason may be empty (delete of a never-published draft). */
  required?: boolean;
  loading?: boolean;
  error?: unknown;
  onSubmit: (reason: string) => void;
  onCancel: () => void;
}

export function ReleaseReasonDialog({ open, title, okText, description, item, maxLength, required = true, loading, error, onSubmit, onCancel }: ReleaseReasonDialogProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<{ reason?: string }>();
  return (
    <Modal
      open={open}
      title={title}
      okText={okText}
      cancelText={t('content_studio.cancel')}
      okButtonProps={{ danger: true }}
      confirmLoading={loading}
      destroyOnHidden
      onCancel={onCancel}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (v) onSubmit((v.reason ?? '').trim());
      }}
    >
      <Flex vertical gap="small">
        {description ? <Typography.Paragraph>{description}</Typography.Paragraph> : null}
        {item ? (
          <section aria-label={t('content_studio.usage.title')}>
            <Typography.Text strong>{t('content_studio.usage.title')}</Typography.Text>
            <UsageList item={item} compact />
          </section>
        ) : null}
        <Form form={form} layout="vertical" preserve={false} requiredMark={required}>
          <Form.Item name="reason" label={t('content_studio.release.reason')} rules={[...(required ? [{ required: true, whitespace: true, message: t('content_studio.release.reasonRequired') }] : []), { max: maxLength }]}>
            <Input.TextArea aria-label={t('content_studio.release.reason')} maxLength={maxLength} showCount autoSize={{ minRows: 2, maxRows: 6 }} />
          </Form.Item>
        </Form>
        {error ? <ProblemMessage error={error} /> : null}
      </Flex>
    </Modal>
  );
}

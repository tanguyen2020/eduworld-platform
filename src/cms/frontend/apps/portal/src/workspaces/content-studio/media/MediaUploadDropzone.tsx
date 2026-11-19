// MediaUploadDropzone (F054-AC-17, ADR-0044, TC-CNT-008): drag & drop several files; each is pre-checked (type, size
// by kind) BEFORE presignMediaUpload; license required, alt text required for images; then sha256 → presign
// (Idempotency-Key) → PUT to the quarantine URL → complete → status followed until `ready` / `rejected`. Server
// refusals (MEDIA_TYPE_NOT_ALLOWED, MEDIA_TOO_LARGE…) are shown per file. A System Admin chooses the owner first.
import { InboxOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Checkbox, Flex, Form, Input, List, Typography, Upload } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { problemOf } from '../../../shell/ProblemMessage';
import { useContentAccess } from '../shared/access';
import { LIMITS } from '../shared/config';
import { OWNER_FIELD, ownerOf, OwnerScopeField } from '../shared/OwnerScopeField';
import { MediaStatusTag } from './MediaPicker';
import { checkMediaFile, formatMb, MEDIA_ACCEPT, type MediaCheck } from './mediaRules';
import { sha256OfFile, useMediaUpload } from './useMediaUpload';

interface Row {
  uid: string;
  file: File;
  check: MediaCheck;
  assetId?: string;
  error?: string;
}

interface UploadForm {
  license: string;
  alt_text?: string;
  exam?: boolean;
  [OWNER_FIELD]?: string;
}

export function MediaUploadDropzone({ onUploaded }: { onUploaded?: () => void }) {
  const { t } = useTranslation();
  const access = useContentAccess();
  const upload = useMediaUpload();
  const [rows, setRows] = useState<Row[]>([]);
  const [busy, setBusy] = useState(false);
  const [form] = Form.useForm<UploadForm>();
  const hasImage = rows.some((r) => r.check.ok && r.check.kind === 'image' && !r.assetId);
  const pending = rows.filter((r) => r.check.ok && !r.assetId);

  const reason = (c: MediaCheck) => (c.ok ? '' : c.code === 'MEDIA_TOO_LARGE' ? t('content_studio.media.tooLarge', { max: formatMb(c.maxBytes ?? 0) }) : t('content_studio.media.typeNotAllowed'));

  const start = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const owner = access.crossTenant ? ownerOf(v[OWNER_FIELD]) : undefined;
    setBusy(true);
    for (const r of pending) {
      if (!r.check.ok) continue;
      try {
        const sha256 = await sha256OfFile(r.file);
        const assetId = await upload.mutateAsync({ file: r.file, kind: r.check.kind, license: v.license.trim(), altText: r.check.kind === 'image' ? v.alt_text?.trim() : undefined, isExamMaterial: !!v.exam, owner, sha256 });
        setRows((xs) => xs.map((x) => (x.uid === r.uid ? { ...x, assetId, error: undefined } : x)));
      } catch (e) {
        const msg = e instanceof ApiError && (e.code === 'MEDIA_TYPE_NOT_ALLOWED' || e.code === 'MEDIA_TOO_LARGE') ? t(`error.${e.code}`) : t(problemOf(e).key);
        setRows((xs) => xs.map((x) => (x.uid === r.uid ? { ...x, error: msg } : x)));
      }
    }
    setBusy(false);
    onUploaded?.();
  };

  return (
    <Card size="small" title={t('content_studio.media.uploadTitle')}>
      <Flex vertical gap="middle">
        <Upload.Dragger
          multiple
          accept={MEDIA_ACCEPT}
          showUploadList={false}
          beforeUpload={(file) => {
            setRows((xs) => [...xs, { uid: file.uid, file, check: checkMediaFile(file) }]);
            return false;
          }}
        >
          <p>
            <InboxOutlined />
          </p>
          <Typography.Paragraph>{t('content_studio.media.dropHint')}</Typography.Paragraph>
          <Typography.Text type="secondary">{t('content_studio.media.limitsHint')}</Typography.Text>
        </Upload.Dragger>
        {rows.length > 0 && (
          <List
            size="small"
            bordered
            data-testid="upload-rows"
            dataSource={rows}
            renderItem={(r) => (
              <List.Item key={r.uid} data-file={r.file.name}>
                <Flex vertical>
                  <Typography.Text>
                    {r.file.name} · {formatMb(r.file.size)}
                  </Typography.Text>
                  {!r.check.ok && <Typography.Text type="danger">{reason(r.check)}</Typography.Text>}
                  {r.error && <Typography.Text type="danger">{r.error}</Typography.Text>}
                </Flex>
                {r.assetId ? <MediaStatusTag assetId={r.assetId} /> : null}
              </List.Item>
            )}
          />
        )}
        <Form form={form} layout="vertical" requiredMark>
          {access.crossTenant && <OwnerScopeField />}
          <Form.Item name="license" label={t('content_studio.media.license')} rules={[{ required: true, whitespace: true, message: t('content_studio.media.licenseRequired') }, { max: LIMITS.license }]}>
            <Input aria-label={t('content_studio.media.license')} maxLength={LIMITS.license} showCount />
          </Form.Item>
          {hasImage && (
            <Form.Item name="alt_text" label={t('content_studio.media.altText')} rules={[{ required: true, whitespace: true, message: t('content_studio.media.altRequired') }, { max: LIMITS.altText }]}>
              <Input.TextArea aria-label={t('content_studio.media.altText')} maxLength={LIMITS.altText} showCount autoSize={{ minRows: 1, maxRows: 4 }} />
            </Form.Item>
          )}
          <Form.Item name="exam" valuePropName="checked">
            <Checkbox>{t('content_studio.media.exam')}</Checkbox>
          </Form.Item>
        </Form>
        {rows.some((r) => !r.check.ok) && <Alert type="warning" showIcon message={t('content_studio.media.someRefused')} />}
        <Button type="primary" disabled={pending.length === 0} loading={busy} onClick={() => void start()}>
          {t('content_studio.media.uploadN', { count: pending.length })}
        </Button>
      </Flex>
    </Card>
  );
}

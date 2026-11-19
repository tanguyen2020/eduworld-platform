// MediaPicker (F053-AC-05, TC-CNT-008, CONTENT_MODEL §7): the media slot of the editors — pick an existing asset
// (listMediaAssets) or upload a new one (license required; alt text required for images), then follow the pipeline
// status until `ready` / `rejected` (reported to the page, which blocks Gửi duyệt meanwhile). No new asset beyond
// `assets_per_version_max`. Upload needs `media.upload` (ActionGate).
import { Button, Checkbox, Flex, Form, Input, List, Modal, Space, Tag, Typography } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { LIMITS } from '../shared/config';
import { sha256OfFile, useMediaAsset, useMediaList, useMediaUpload, type MediaKind, type MediaStatus } from './useMediaUpload';

export interface MediaPickerProps {
  id: string;
  kind: Extract<MediaKind, 'image' | 'audio' | 'video'>;
  value?: string;
  onChange: (assetId: string | undefined) => void;
  /** Pipeline status of the chosen asset (blocks submit until ready). */
  onStatus?: (assetId: string, status: MediaStatus) => void;
  disabled?: boolean;
  /** No more new asset in this version (assets_per_version_max reached). */
  limitReached?: boolean;
  owner?: { body: Record<string, unknown>; headers: Record<string, string> };
}

const STATUS_COLOR: Record<MediaStatus, string> = { quarantined: 'processing', processing: 'processing', ready: 'success', rejected: 'error' };

export function MediaStatusTag({ assetId, onStatus }: { assetId: string; onStatus?: MediaPickerProps['onStatus'] }) {
  const { t } = useTranslation();
  const q = useMediaAsset(assetId);
  const status = q.data?.status;
  useEffect(() => {
    if (status) onStatus?.(assetId, status);
  }, [assetId, status, onStatus]);
  if (!q.data) return <Tag>{t('content_studio.media.loading')}</Tag>;
  return (
    <Space>
      <Typography.Text>{q.data.original_filename}</Typography.Text>
      <Tag color={STATUS_COLOR[q.data.status]} data-media-status={q.data.status}>
        {t(`content_studio.media.status.${q.data.status}`)}
      </Tag>
      {q.data.status === 'rejected' && q.data.reject_reason && <Typography.Text type="danger">{q.data.reject_reason}</Typography.Text>}
    </Space>
  );
}

export function MediaPicker({ id, kind, value, onChange, onStatus, disabled = false, limitReached = false, owner }: MediaPickerProps) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<'none' | 'library' | 'upload'>('none');
  const [q, setQ] = useState('');
  const list = useMediaList(kind, q, mode === 'library');
  const upload = useMediaUpload();
  const [form] = Form.useForm<{ license: string; alt_text?: string; exam?: boolean }>();
  const [file, setFile] = useState<File | null>(null);
  const blocked = disabled || (limitReached && !value);

  const submitUpload = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v || !file) return;
    const sha256 = await sha256OfFile(file);
    const assetId = await upload.mutateAsync({ file, kind, license: v.license.trim(), altText: v.alt_text?.trim(), isExamMaterial: v.exam, sha256, owner });
    onChange(assetId);
    setMode('none');
    setFile(null);
    form.resetFields();
  };

  return (
    <Flex vertical gap="small" data-field={id} id={id}>
      <Space wrap>
        {value ? <MediaStatusTag assetId={value} onStatus={onStatus} /> : <Typography.Text type="secondary">{t('content_studio.media.none')}</Typography.Text>}
        <Button size="small" disabled={blocked} onClick={() => setMode('library')}>
          {t('content_studio.media.choose')}
        </Button>
        <ActionGate actionKey="action.content_studio.media.upload">
          <Button size="small" disabled={blocked} onClick={() => setMode('upload')}>
            {t('content_studio.media.upload')}
          </Button>
        </ActionGate>
        {value && !disabled && (
          <Button size="small" danger onClick={() => onChange(undefined)}>
            {t('content_studio.media.remove')}
          </Button>
        )}
      </Space>
      {limitReached && !value && <Typography.Text type="warning">{t('content_studio.media.limit')}</Typography.Text>}

      <Modal open={mode === 'library'} title={t('content_studio.media.libraryTitle')} footer={null} onCancel={() => setMode('none')} destroyOnHidden>
        <Flex vertical gap="small">
          <Input.Search aria-label={t('content_studio.media.search')} placeholder={t('content_studio.media.search')} maxLength={100} allowClear onSearch={setQ} />
          {list.error ? <ProblemMessage error={list.error} /> : null}
          <List
            loading={list.isFetching}
            dataSource={list.data?.data ?? []}
            renderItem={(a) => (
              <List.Item
                actions={[
                  <Button
                    key="pick"
                    size="small"
                    type="primary"
                    disabled={a.status === 'rejected'}
                    onClick={() => {
                      onChange(a.asset_id);
                      setMode('none');
                    }}
                  >
                    {t('content_studio.media.pick')}
                  </Button>,
                ]}
              >
                <Space>
                  <Typography.Text>{a.original_filename}</Typography.Text>
                  <Tag color={STATUS_COLOR[a.status]}>{t(`content_studio.media.status.${a.status}`)}</Tag>
                  {a.alt_text && <Typography.Text type="secondary">{a.alt_text}</Typography.Text>}
                </Space>
              </List.Item>
            )}
          />
        </Flex>
      </Modal>

      <Modal
        open={mode === 'upload'}
        title={t('content_studio.media.uploadTitle')}
        okText={t('content_studio.media.uploadOk')}
        cancelText={t('content_studio.cancel')}
        okButtonProps={{ disabled: !file }}
        confirmLoading={upload.isPending}
        onOk={() => void submitUpload().catch(() => undefined)}
        onCancel={() => setMode('none')}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item label={t('content_studio.media.file')} required>
            <input
              type="file"
              aria-label={t('content_studio.media.file')}
              accept={kind === 'image' ? 'image/*' : kind === 'audio' ? 'audio/*' : 'video/*'}
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </Form.Item>
          <Form.Item name="license" label={t('content_studio.media.license')} rules={[{ required: true, whitespace: true, message: t('content_studio.media.licenseRequired') }]}>
            <Input maxLength={LIMITS.license} showCount />
          </Form.Item>
          {kind === 'image' && (
            <Form.Item name="alt_text" label={t('content_studio.media.altText')} rules={[{ required: true, whitespace: true, message: t('content_studio.media.altRequired') }]}>
              <Input.TextArea maxLength={LIMITS.altText} showCount autoSize={{ minRows: 2, maxRows: 4 }} />
            </Form.Item>
          )}
          <Form.Item name="exam" valuePropName="checked">
            <Checkbox>{t('content_studio.media.examMaterial')}</Checkbox>
          </Form.Item>
        </Form>
        {upload.error ? <ProblemMessage error={upload.error} /> : null}
      </Modal>
    </Flex>
  );
}

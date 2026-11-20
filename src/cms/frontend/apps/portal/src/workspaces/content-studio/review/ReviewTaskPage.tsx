// ReviewTaskPage (F054-AC-13, AC-30, SC-001, CONTENT_MODEL §6): one review task — by default the diff against the
// version students get now (or the previous version for a first publication) and the student preview of the version
// under review; "Duyệt" (with the audio-exemption acknowledgement when the version has one) and "Từ chối" (reason
// required + per-field comments sent as `field_comments[]` — JSON pointer path, ≤ 50, OQ F054-1). Comments of an
// earlier rejection of the same version are shown above. The author of the version gets no decision buttons (the
// server refuses anyway: reviewer ≠ author). Deciding works on every breakpoint; the diff needs ≥ 1024 px.
import { DomainTag } from '@eduworld/ops-ui';
import { Alert, AutoComplete, App as AntApp, Button, Card, Checkbox, Flex, Form, Input, Modal, Result, Skeleton, Space, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { useMe } from '../../../shared/api/hooks/useMe';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { PreviewPanel } from '../preview/PreviewPanel';
import { useVersionDetail, useVersions, type ItemRef } from '../release/useRelease';
import { VersionDiffView } from '../release/VersionDiffView';
import { useContentAccess } from '../shared/access';
import { CONTENT_TYPES } from '../shared/contentTypes';
import { FIELDS, LIMITS } from '../shared/fieldSchemas';
import { ReviewFieldComments } from './ReviewFieldComments';
import { toFieldComments } from './reviewRules';
import { useReviewDecision } from './useReviewTasks';

interface RejectForm {
  reason: string;
  comments?: { path?: string; comment?: string }[];
}

export function ReviewTaskPage({ item, version }: { item: ItemRef; version: number }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { message } = AntApp.useApp();
  const access = useContentAccess();
  const me = useMe();
  const detail = useVersionDetail(item, version);
  const history = useVersions(item);
  const decision = useReviewDecision();
  const [ack, setAck] = useState(false);
  const [rejecting, setRejecting] = useState(false);
  const [form] = Form.useForm<RejectForm>();

  if (detail.isPending || history.isPending) return <Skeleton active />;
  if (detail.error) return detail.error instanceof ApiError && detail.error.status === 404 ? <Result status="404" title={t('content_studio.review.notFound')} /> : <ProblemMessage error={detail.error} />;
  const v = detail.data.data;
  const current = history.data?.current_version;
  const against = current !== undefined && current !== version ? current : version > 1 ? version - 1 : undefined;
  const isAuthor = !!v.author_id && v.author_id === me.data?.user_id;
  const exemption = v.audio_exempt_reason;
  const canDecide = v.status === 'in_review' && access.question.review && !isAuthor;
  const back = () => navigate('../../../', { relative: 'path' });
  const lang = v.content_language === 'en' ? 'en' : 'vi';
  // suggestions for the field pointer: the top-level fields of the version under review (free text stays allowed)
  const pointers = Object.keys(v.content ?? {}).map((k) => ({ value: `/${k}` }));
  const maxComments = LIMITS.reviewFieldComments.maxItems;

  const approve = () =>
    decision.mutate(
      { ...item, version, action: 'approve', audioExemptionAcknowledged: !!exemption && ack },
      {
        onSuccess: () => {
          message.success(t('content_studio.review.approved', { version }));
          back();
        },
      },
    );

  return (
    <Flex vertical gap="middle" data-testid="review-task">
      <Flex align="center" gap="small" wrap>
        <Typography.Title level={1}>{t('content_studio.review.taskTitle', { type: t(`content_studio.contentType.${item.contentType}`), version })}</Typography.Title>
        <DomainTag kind="content_status" value={v.status} />
      </Flex>
      {isAuthor && <Alert type="info" showIcon message={t('content_studio.review.ownContent')} />}
      <ReviewFieldComments item={item} version={version} />
      {decision.error ? <ProblemMessage error={decision.error} /> : null}
      {canDecide && (
        <Card size="small">
          <Flex vertical gap="small">
            {exemption && (
              <Checkbox checked={ack} onChange={(e) => setAck(e.target.checked)}>
                {t('content_studio.review.ackAudioOne', { reason: exemption })}
              </Checkbox>
            )}
            <Space wrap>
              <ActionGate actionKey={CONTENT_TYPES[item.contentType].actions.workflow} mode="disable">
                <Button type="primary" disabled={!!exemption && !ack} loading={decision.isPending && decision.variables?.action === 'approve'} onClick={approve}>
                  {t('content_studio.review.approve')}
                </Button>
              </ActionGate>
              <ActionGate actionKey={CONTENT_TYPES[item.contentType].actions.workflow} mode="disable">
                <Button danger onClick={() => setRejecting(true)}>
                  {t('content_studio.review.reject')}
                </Button>
              </ActionGate>
            </Space>
          </Flex>
        </Card>
      )}
      {against !== undefined ? (
        <VersionDiffView item={item} version={version} against={against} canReview={access.question.review} />
      ) : (
        <Typography.Text type="secondary">{t('content_studio.review.firstVersion')}</Typography.Text>
      )}
      <PreviewPanel item={item} version={version} contentLanguage={lang} />
      <Modal
        open={rejecting}
        title={t('content_studio.review.rejectTitle', { version })}
        okText={t('content_studio.review.reject')}
        okButtonProps={{ danger: true }}
        cancelText={t('content_studio.cancel')}
        confirmLoading={decision.isPending}
        destroyOnHidden
        onCancel={() => setRejecting(false)}
        onOk={async () => {
          const f = await form.validateFields().catch(() => null);
          if (!f) return;
          const fieldComments = toFieldComments(f.comments ?? []);
          decision.mutate(
            { ...item, version, action: 'reject', note: f.reason.trim(), fieldComments },
            {
              onSuccess: () => {
                message.success(t('content_studio.review.rejected', { version }));
                setRejecting(false);
                back();
              },
            },
          );
        }}
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="reason" label={t('content_studio.review.reason')} rules={[{ required: true, whitespace: true, message: t('content_studio.release.reasonRequired') }, { max: FIELDS.transitionNote.maxLength }]}>
            <Input.TextArea aria-label={t('content_studio.review.reason')} maxLength={FIELDS.transitionNote.maxLength} showCount autoSize={{ minRows: 2, maxRows: 6 }} />
          </Form.Item>
          <Form.List name="comments">
            {(fields, { add, remove }) => (
              <Flex vertical gap="small">
                {fields.map((field, i) => (
                  <Space key={field.key} align="start" wrap>
                    <Form.Item name={[field.name, 'path']} extra={i === 0 ? t('content_studio.review.fieldHelp') : undefined}>
                      <AutoComplete
                        aria-label={t('content_studio.review.field', { n: i + 1 })}
                        placeholder={t('content_studio.review.fieldPlaceholder')}
                        maxLength={FIELDS.reviewFieldPath.maxLength}
                        options={pointers}
                        filterOption={(input, o) => String(o?.value ?? '').toLowerCase().includes(input.toLowerCase())}
                        style={{ minWidth: 200 }}
                      />
                    </Form.Item>
                    <Form.Item name={[field.name, 'comment']} rules={[{ required: true, whitespace: true, message: t('content_studio.review.commentRequired') }, { max: FIELDS.reviewFieldComment.maxLength }]}>
                      <Input.TextArea aria-label={t('content_studio.review.comment', { n: i + 1 })} placeholder={t('content_studio.review.commentPlaceholder')} maxLength={FIELDS.reviewFieldComment.maxLength} autoSize />
                    </Form.Item>
                    <Button onClick={() => remove(field.name)}>{t('content_studio.review.removeComment')}</Button>
                  </Space>
                ))}
                <Button disabled={fields.length >= maxComments} onClick={() => add({ path: '', comment: '' })}>
                  {t('content_studio.review.addComment')}
                </Button>
                {fields.length >= maxComments && <Typography.Text type="secondary">{t('content_studio.review.commentsMax', { max: maxComments })}</Typography.Text>}
              </Flex>
            )}
          </Form.List>
        </Form>
        {decision.error ? <ProblemMessage error={decision.error} /> : null}
      </Modal>
    </Flex>
  );
}

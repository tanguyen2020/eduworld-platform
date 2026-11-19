// StudentViewPreview (F053-AC-03, AC-19): "Như học sinh thấy" — the draft being edited through toStudentView() (no
// feedback, hint, explanation or answer), with a vi / en switch; and for a saved version, the server preview session
// (`createLearningPreviewSession`, render payload without answers — ADR-0016).
import { QuestionPreview, type QuestionDraft } from '@eduworld/ops-ui/content';
import { Button, Flex, Segmented, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import type { components } from '../../../shared/api/schema.gen';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { ProblemMessage } from '../../../shell/ProblemMessage';

type PreviewSession = components['schemas']['LearningPreviewSession'];

export interface StudentViewPreviewProps {
  draft: QuestionDraft;
  contentLanguage: 'vi' | 'en';
  saved?: { contentId: string; version: number };
  dirty: boolean;
}

export function StudentViewPreview({ draft, contentLanguage, saved, dirty }: StudentViewPreviewProps) {
  const { t } = useTranslation();
  const { api } = usePortalRuntime();
  const [lang, setLang] = useState<'vi' | 'en'>(contentLanguage);
  const [server, setServer] = useState<PreviewSession | null>(null);
  const session = useIdempotentMutation((v: { contentId: string; version: number; lang: 'vi' | 'en' }, intent) =>
    api.call<PreviewSession>(
      api.client.POST('/cms/learning/preview/sessions', {
        body: { content_type: 'question', content_id: v.contentId, version: v.version, display_language: v.lang === 'vi' ? 'vi-VN' : 'en-US' },
        headers: intent as never,
      }),
    ),
  );
  const item = server?.items?.[0];
  return (
    <Flex vertical gap="small">
      <Segmented
        aria-label={t('content_studio.preview.language')}
        value={lang}
        onChange={(v) => setLang(v as 'vi' | 'en')}
        options={[
          { value: 'vi', label: t('content_studio.preview.vi') },
          { value: 'en', label: t('content_studio.preview.en') },
        ]}
      />
      <QuestionPreview type={draft.type} stem={draft.stem} renderPayload={draft.render_payload} language={lang} contentLanguage={contentLanguage} />
      {saved && (
        <ActionGate actionKey="action.content_studio.reviews.preview">
          <Button loading={session.isPending} onClick={() => session.mutate({ ...saved, lang }, { onSuccess: setServer })}>
            {t('content_studio.preview.server')}
          </Button>
        </ActionGate>
      )}
      {saved && dirty && <Typography.Text type="secondary">{t('content_studio.preview.unsaved')}</Typography.Text>}
      {session.error ? <ProblemMessage error={session.error} /> : null}
      {item && (
        <div data-testid="server-preview">
          <QuestionPreview type={item.type} stem={(item.render_payload as { stem?: QuestionDraft['stem'] }).stem} renderPayload={item.render_payload} language={lang} contentLanguage={contentLanguage} />
        </div>
      )}
    </Flex>
  );
}

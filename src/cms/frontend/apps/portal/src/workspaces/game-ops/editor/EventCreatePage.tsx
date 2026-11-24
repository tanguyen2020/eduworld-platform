// EventCreatePage (F079-AC-12, AC-13, ADMIN_PORTAL §3.15): the Event Builder wizard of a new event — the 5 content steps
// (EventWizard), then 6 Preview (the draft is created / saved first: createGameConfig or updateGameConfigDraft with
// If-Match; `422` errors go back to their step, which is marked and blocked), 7 Gửi duyệt (transitionGameConfigVersion
// submit_review; TRANSLATION_INCOMPLETE and the other validation errors map to their field), 8 Lên lịch (publish with an
// ICT `publish_at` once another reviewer approved — reviewer ≠ author is the server's rule).
import { initialValue, withConsts } from '@eduworld/ops-ui/game-console';
import { Alert, App as AntApp, Button, Card, Flex, Form, Input, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import { saveDraft, useConfigWrites, type GameConfigVersion } from '../api/gameConfigs';
import { EventWizard, stepsWithErrors } from '../builders/event/EventWizard';
import { actionOf, type ConfigTypeDef } from '../registry/configTypes';
import { useGameAccess } from '../shared/access';
import { fieldErrorsOf, GameOpsError } from '../shared/errors';
import { TITLE_MAX } from './ConfigEditorPage';
import { PreviewPanel } from './PreviewPanel';
import { PublishDialog } from './PublishDialog';

interface Saved {
  configId: string;
  version: number;
  etag: string | null;
  status: GameConfigVersion['status'];
}

export function EventCreatePage({ def }: { def: ConfigTypeDef }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const { api } = usePortalRuntime();
  const access = useGameAccess(def);
  const writes = useConfigWrites();
  const [title, setTitle] = useState('');
  const [payload, setPayload] = useState<Record<string, unknown>>(() => withConsts(initialValue(def.schema!) ?? {}, def.schema!) as Record<string, unknown>);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState<unknown>(null);
  const [saved, setSaved] = useState<Saved | null>(null);
  const [step, setStep] = useState(0);
  const [publishing, setPublishing] = useState(false);
  const exists = (k: string) => i18n.exists(k);
  const body = () => ({ ...(title.trim() ? { title: title.trim() } : {}), payload: withConsts(payload, def.schema!) as Record<string, unknown> });

  const fail = (e: unknown) => {
    setError(e);
    const fe = fieldErrorsOf(e, t, exists);
    setErrors(fe);
    const bad = [...stepsWithErrors(fe)];
    if (bad.length) setStep(Math.min(...bad));
  };

  /** Creates the draft the first time, saves it afterwards (If-Match); false when the server refused it. */
  const ensureSaved = async (): Promise<boolean> => {
    try {
      if (!saved) {
        const r = await writes.create.mutateAsync({ configType: def.type, ...body() });
        setSaved({ configId: r.data.config_id, version: r.data.version, etag: r.etag, status: r.data.status });
      } else {
        const r = await saveDraft(api, { configType: def.type, configId: saved.configId }, saved.version, body(), saved.etag);
        setSaved({ ...saved, etag: r.etag, status: r.data.status });
      }
      setErrors({});
      setError(null);
      return true;
    } catch (e) {
      fail(e);
      return false;
    }
  };

  const submit = () =>
    saved &&
    writes.transition.mutate(
      { configType: def.type, configId: saved.configId, version: saved.version, action: 'submit_review' },
      {
        onSuccess: (v) => {
          setSaved({ ...saved, status: v.status });
          message.success(t('gameOps.editor.done.submit_review'));
          setStep(7);
        },
        onError: fail,
      },
    );

  const editorLink = saved ? `../${saved.configId}/v/${saved.version}` : '..';

  return (
    <Flex vertical gap="middle" data-testid="event-create">
      <Typography.Title level={1}>{t('gameOps.event.createTitle')}</Typography.Title>
      <Card>
        <Flex vertical gap="middle">
          <Form layout="vertical" component="div">
            <Form.Item label={t('gameOps.editor.title')} htmlFor="event-title">
              <Input id="event-title" maxLength={TITLE_MAX} showCount value={title} onChange={(e) => setTitle(e.target.value)} />
            </Form.Item>
          </Form>
          {error && !Object.keys(errors).length ? <GameOpsError error={error} /> : null}
          <EventWizard
            def={def}
            value={payload}
            onChange={setPayload}
            readOnly={saved !== null && saved.status !== 'draft' && saved.status !== 'rejected'}
            errors={errors}
            step={step}
            onStepChange={setStep}
            onBeforeExtra={ensureSaved}
            extraSteps={[
              {
                title: t('gameOps.event.step.preview'),
                content: saved ? <PreviewPanel configType={def.type} configId={saved.configId} version={saved.version} /> : <Alert type="info" showIcon message={t('gameOps.sim.saveFirst')} />,
              },
              {
                title: t('gameOps.event.step.submit'),
                content: (
                  <Flex vertical gap="small" data-testid="event-submit-step">
                    <Typography.Paragraph>{t('gameOps.event.submitHint')}</Typography.Paragraph>
                    <ActionGate actionKey={actionOf(def, 'workflow')} mode="disable">
                      <Button type="primary" disabled={!saved || saved.status !== 'draft'} loading={writes.transition.isPending} onClick={() => void submit()}>
                        {t('gameOps.event.submit')}
                      </Button>
                    </ActionGate>
                  </Flex>
                ),
              },
              {
                title: t('gameOps.event.step.schedule_publish'),
                content: (
                  <Flex vertical gap="small" data-testid="event-schedule-step">
                    {saved?.status === 'approved' && access.publish ? (
                      <ActionGate actionKey={actionOf(def, 'publish')} mode="disable">
                        <Button type="primary" onClick={() => setPublishing(true)}>
                          {t('gameOps.event.schedulePublish')}
                        </Button>
                      </ActionGate>
                    ) : (
                      <Alert type="info" showIcon message={t('gameOps.event.waitReview')} />
                    )}
                    {saved && <Link to={editorLink} relative="path">{t('gameOps.event.openEditor')}</Link>}
                  </Flex>
                ),
              },
            ]}
          />
        </Flex>
      </Card>
      {saved && <PublishDialog item={{ configType: def.type, configId: saved.configId }} version={publishing ? saved.version : null} onClose={() => setPublishing(false)} />}
    </Flex>
  );
}

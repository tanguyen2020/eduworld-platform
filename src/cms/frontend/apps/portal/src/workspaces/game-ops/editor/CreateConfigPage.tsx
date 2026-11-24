// CreateConfigPage (F079-AC-02, AC-05, AC-08, AC-20): a new config of a type — title + the payload form of its builder
// held in memory, then createGameConfig (version 1 draft, one Idempotency-Key per intent: a retry after a network error
// sends the same key). The server validates the payload (F078): `422` field errors are shown on their field; on success
// the editor of v1 opens (autosave from there). Events use the Event Builder wizard instead (EventCreatePage).
import { initialValue, withConsts } from '@eduworld/ops-ui/game-console';
import { Alert, Button, Card, Flex, Form, Input, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { EDITOR_MIN_WIDTH_QUERY } from '../../../shared/release/config';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { useConfigWrites } from '../api/gameConfigs';
import { ConfigBuilder } from '../builders/ConfigBuilder';
import { actionOf, CONFIG_TYPES, isConfigType, type ConfigTypeDef } from '../registry/configTypes';
import { fieldErrorsOf, GameOpsError } from '../shared/errors';
import { TITLE_MAX } from './ConfigEditorPage';
import { EventCreatePage } from './EventCreatePage';

export function CreateConfigPage() {
  const { configType } = useParams();
  const wide = useMediaQuery(EDITOR_MIN_WIDTH_QUERY);
  if (!isConfigType(configType)) return <GameOpsError error={null} />;
  if (!wide) return <LargeScreenRequired />;
  const def = CONFIG_TYPES[configType];
  if (def.builder === 'event') return <EventCreatePage def={def} />;
  return <CreateForm def={def} />;
}

/** The first payload of a new config: the `const` fields of its schema (schema_version, paid_track = false…). */
export function newPayload(def: ConfigTypeDef): Record<string, unknown> {
  return def.schema ? ((withConsts(initialValue(def.schema) ?? {}, def.schema) as Record<string, unknown>) ?? {}) : {};
}

function CreateForm({ def }: { def: ConfigTypeDef }) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { create } = useConfigWrites();
  const [title, setTitle] = useState('');
  const [payload, setPayload] = useState<Record<string, unknown>>(() => newPayload(def));
  const errors = fieldErrorsOf(create.error, t, (k) => i18n.exists(k));
  return (
    <Flex vertical gap="middle" data-testid="create-config">
      <Typography.Title level={1}>{t('gameOps.create.title', { type: t(`gameOps.type.${def.type}`) })}</Typography.Title>
      <Card>
        <Flex vertical gap="middle">
          <Alert type="info" showIcon message={t('gameOps.create.hint')} />
          <Form layout="vertical" component="div">
            <Form.Item label={t('gameOps.editor.title')} htmlFor="game-config-new-title">
              <Input id="game-config-new-title" maxLength={TITLE_MAX} showCount value={title} onChange={(e) => setTitle(e.target.value)} />
            </Form.Item>
          </Form>
          <ConfigBuilder def={def} value={payload} onChange={setPayload} readOnly={false} errors={errors} />
          {create.error ? <GameOpsError error={create.error} /> : null}
          <Flex gap="small">
            <Button onClick={() => navigate('..', { relative: 'path' })}>{t('gameOps.common.cancel')}</Button>
            <ActionGate actionKey={actionOf(def, 'edit')} mode="disable">
              <Button
                type="primary"
                loading={create.isPending}
                onClick={() =>
                  create.mutate(
                    { configType: def.type, ...(title.trim() ? { title: title.trim() } : {}), payload: withConsts(payload, def.schema ?? {}) as Record<string, unknown> },
                    { onSuccess: (r) => navigate(`../${r.data.config_id}/v/${r.data.version}`, { relative: 'path' }) },
                  )
                }
              >
                {t('gameOps.create.submit')}
              </Button>
            </ActionGate>
          </Flex>
        </Flex>
      </Card>
    </Flex>
  );
}

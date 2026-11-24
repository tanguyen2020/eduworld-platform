// PreviewPanel (F079-AC-17, ADMIN_PORTAL §5, ADR-0047): "Xem trước" of one saved version — createGamePreviewSession
// (Idempotency-Key) then getGamePreviewSnapshot with the token (kept in memory, never logged), shown in GamePreviewFrame
// (Godot Web build of the runtime config `gamePreviewUrl`). The game look exists only inside that frame. Without a
// configured preview build the button explains it instead.
import { GamePreviewFrame } from '@eduworld/ops-ui/game-console';
import { Alert, Button, Card, Flex } from 'antd';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import { usePreview } from '../api/console';
import type { ConfigType } from '../registry/configTypes';
import { GameOpsError } from '../shared/errors';

export function PreviewPanel({ configType, configId, version }: { configType: ConfigType; configId: string; version: number }) {
  const { t } = useTranslation();
  const { config } = usePortalRuntime();
  const preview = usePreview();
  return (
    <Card size="small" title={t('gameOps.preview.title')} data-testid="preview-panel">
      <Flex vertical gap="small">
        {!config.gamePreviewUrl ? (
          <Alert type="info" showIcon message={t('gameOps.preview.notConfigured')} />
        ) : (
          <ActionGate actionKey="action.game_ops.review.preview" mode="disable">
            <Button loading={preview.isPending} onClick={() => preview.mutate({ configType, configId, version })}>
              {t('gameOps.preview.open', { version })}
            </Button>
          </ActionGate>
        )}
        {preview.error ? <GameOpsError error={preview.error} /> : null}
        {preview.data && config.gamePreviewUrl && <GamePreviewFrame src={config.gamePreviewUrl} snapshot={preview.data} title={t('gameOps.preview.frameTitle', { version })} />}
      </Flex>
    </Card>
  );
}

// OutdatedDiffDrawer (F082 T022, F082-AC-10, ADMIN_PORTAL §1.5): what changed in the original of an outdated game text —
// diffGameConfigVersions between the version the text was translated in (`translated_in_version`) and the current one,
// limited to that pointer. The translator updates the target text by hand: no machine translation.
import { textKey } from '@eduworld/ops-ui/game-console';
import { Alert, Drawer, Flex, Skeleton, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { DiffTable } from '../../../shared/release/DiffTable';
import { useDiff, type ItemRef } from '../api/gameConfigs';
import { GameOpsError } from '../shared/errors';

export interface OutdatedField {
  pointer: string;
  translated_in_version: number;
}

export function OutdatedDiffDrawer({ item, version, field, onClose }: { item: ItemRef; version: number; field: OutdatedField | null; onClose: () => void }) {
  const { t } = useTranslation();
  const diff = useDiff(item, field ? version : undefined, field?.translated_in_version);
  const key = field ? textKey(field.pointer) : '';
  const changes = (diff.data?.changes ?? []).filter((c) => textKey(c.path) === key || textKey(c.path).startsWith(`${key}/`));
  return (
    <Drawer open={!!field} onClose={onClose} title={t('gameOps.translations.diffTitle', { field: key })} destroyOnHidden>
      {field && (
        <Flex vertical gap="middle" data-testid="outdated-diff">
          <Typography.Text type="secondary">{t('gameOps.translations.diffRange', { from: field.translated_in_version, to: version })}</Typography.Text>
          {field.translated_in_version === version ? (
            <Alert type="info" showIcon message={t('gameOps.translations.diffSameVersion')} />
          ) : diff.isPending ? (
            <Skeleton active />
          ) : diff.error ? (
            <GameOpsError error={diff.error} />
          ) : (
            <DiffTable changes={changes.length ? changes : (diff.data?.changes ?? [])} fromVersion={field.translated_in_version} toVersion={version} />
          )}
        </Flex>
      )}
    </Drawer>
  );
}

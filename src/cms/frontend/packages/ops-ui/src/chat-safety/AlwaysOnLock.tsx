// AlwaysOnLock (F122 T007, F122-AC-10, F122-AC-22, ADR-0020 §6, ADR-0078): a control that cannot be turned off —
// PII/grooming steps of the moderation pipeline, gameplay chat match/team/party — shown as a "Luôn bật" tag and an
// explanation instead of a switch. Display only: the server refuses the change anyway.
import { Flex, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export interface AlwaysOnLockProps {
  /** Why it is always on (translated by the caller), shown next to the lock. */
  reason?: string;
}

export function AlwaysOnLock({ reason }: AlwaysOnLockProps) {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Flex align="center" gap="small" wrap data-always-on="true">
      <Tag>{t('chatSafety.alwaysOn')}</Tag>
      {reason ? <Typography.Text type="secondary">{reason}</Typography.Text> : null}
    </Flex>
  );
}

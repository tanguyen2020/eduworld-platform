// `device.cfg` guide of "Tải bộ cài" (F134 T035, F134-AC-17, ADR-0101 mục 8): Android shared tablets get `device.cfg`
// (`shared_device`, `lan_cache_url`) from the school MDM through managed configuration. Static help, every string from
// the translation file; nothing is generated or sent from here.
import { Card, Flex, Typography } from 'antd';
import { useTranslation } from 'react-i18next';

const K = 'portal.school_lms.installers.deviceCfg.';
/** Keys of `device.cfg` read by the Android client (GODOT_ARCHITECTURE §5.1, PERFORMANCE_BUDGET §10.1): identifiers, not UI text. */
const DEVICE_CFG_KEYS = { sharedDevice: 'shared_device', lanCacheUrl: 'lan_cache_url' } as const;

export function DeviceConfigGuide() {
  const { t } = useTranslation();
  return (
    <Card title={t(`${K}title`)} data-testid="device-cfg-guide">
      <Flex vertical gap="small">
        <Typography.Paragraph>{t(`${K}intro`)}</Typography.Paragraph>
        <Typography.Paragraph>
          <Typography.Text code>{DEVICE_CFG_KEYS.sharedDevice}</Typography.Text> {t(`${K}sharedDevice`)}
        </Typography.Paragraph>
        <Typography.Paragraph>
          <Typography.Text code>{DEVICE_CFG_KEYS.lanCacheUrl}</Typography.Text> {t(`${K}lanCache`)}
        </Typography.Paragraph>
        <Typography.Text strong>{t(`${K}exampleTitle`)}</Typography.Text>
        <pre data-testid="device-cfg-example">
          <Typography.Text code>{t(`${K}example`)}</Typography.Text>
        </pre>
        <Typography.Text type="secondary">{t(`${K}note`)}</Typography.Text>
      </Flex>
    </Card>
  );
}

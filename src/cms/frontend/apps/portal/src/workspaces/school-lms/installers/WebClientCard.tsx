// "Mở bản Web" card of "Tải bộ cài" (F134 T035, ADR-0101 mục 8): PC phòng máy play the Web client, no desktop installer.
// The link is `web_client_url` of listSchoolClientInstallers (hidden when null — environment not configured); the card
// tells whether THIS browser offers WebGL 2 (detectWebGl2). The page wraps it in the view ActionGate. Display only.
import { Alert, Button, Card, Flex, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { detectWebGl2, type WebGlSupport } from './webgl';

const K = 'portal.school_lms.installers.web.';

export function WebClientCard({ url, detect = detectWebGl2 }: { url: string; detect?: () => WebGlSupport }) {
  const { t } = useTranslation();
  const [support] = useState<WebGlSupport>(() => detect());
  return (
    <Card title={t(`${K}title`)} data-testid="web-client-card">
      <Flex vertical gap="small">
        <Typography.Paragraph>{t(`${K}description`)}</Typography.Paragraph>
        <Alert
          type={support === 'supported' ? 'success' : 'warning'}
          showIcon
          role="status"
          data-testid="webgl2-check"
          message={t(support === 'supported' ? `${K}supported` : `${K}unsupported`)}
        />
        <Button type="primary" href={url} target="_blank" rel="noopener noreferrer">
          {t(`${K}open`)}
        </Button>
      </Flex>
    </Card>
  );
}

// Home: links to the 5 workspaces (no access decision here — ADMIN_PORTAL §1.2, backend decides).
import { Card, List, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { WORKSPACES } from './workspaces';

export function Home() {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="ew-home-title">
      <Typography.Title level={1} id="ew-home-title">
        {t('app.home.title')}
      </Typography.Title>
      <Typography.Paragraph type="secondary">{t('app.home.description')}</Typography.Paragraph>
      <List
        grid={{ gutter: 16, xs: 1, sm: 2, lg: 3 }}
        dataSource={[...WORKSPACES]}
        renderItem={(w) => (
          <List.Item>
            <Card>
              <Link to={`/${w.path}`}>{t(`workspace.${w.id}.title`)}</Link>
            </Card>
          </List.Item>
        )}
      />
    </section>
  );
}

// Portal entry (F050, F051). Loads the runtime config first (AGENTS R3); a missing config shows a translated error.
import { OpsThemeProvider } from '@eduworld/ops-ui';
import { Result } from 'antd';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './app/App';
import { readPreferences } from './app/preferences';
import { loadRuntimeConfig, RuntimeConfigError } from './app/runtime-config';
import { createI18n } from './i18n';

async function start() {
  const root = createRoot(document.getElementById('root')!);
  const preferences = readPreferences();
  const i18n = await createI18n(preferences.locale);
  document.documentElement.lang = preferences.locale;
  document.title = i18n.t('app.title');
  try {
    const config = await loadRuntimeConfig();
    root.render(
      <StrictMode>
        <App config={config} i18n={i18n} preferences={preferences} />
      </StrictMode>,
    );
  } catch (e) {
    const missing = e instanceof RuntimeConfigError ? e.missing.join(', ') : '';
    root.render(
      <OpsThemeProvider mode={preferences.theme} locale={preferences.locale}>
        <Result status="error" title={i18n.t('app.configError.title')} subTitle={i18n.t('app.configError.description', { missing })} />
      </OpsThemeProvider>,
    );
  }
}

void start();

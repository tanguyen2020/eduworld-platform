// Test helper: renders an ops-ui component inside the theme + i18n providers (vi default).
import { render } from '@testing-library/react';
import type { ReactElement } from 'react';
import { I18nextProvider } from 'react-i18next';
import { OpsThemeProvider } from '../theme/OpsThemeProvider';
import { createTestI18n } from './i18n';

export async function renderOps(ui: ReactElement, lng: 'vi-VN' | 'en-US' = 'vi-VN', mode: 'light' | 'dark' = 'light') {
  const i18n = await createTestI18n(lng);
  const utils = render(
    <I18nextProvider i18n={i18n}>
      <OpsThemeProvider mode={mode}>{ui}</OpsThemeProvider>
    </I18nextProvider>,
  );
  return { ...utils, i18n };
}

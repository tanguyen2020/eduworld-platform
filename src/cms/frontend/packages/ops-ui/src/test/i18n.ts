// Test helper: i18next instance with the ops-ui namespace (vi default).
import i18next from 'i18next';
import { initReactI18next } from 'react-i18next';
import { OPS_UI_NS, opsUiResources } from '../i18n';

export async function createTestI18n(lng: 'vi-VN' | 'en-US' = 'vi-VN') {
  const i = i18next.createInstance();
  await i.use(initReactI18next).init({
    lng,
    fallbackLng: 'vi-VN',
    ns: [OPS_UI_NS],
    defaultNS: OPS_UI_NS,
    resources: { 'vi-VN': { [OPS_UI_NS]: opsUiResources.vi }, 'en-US': { [OPS_UI_NS]: opsUiResources.en } },
    interpolation: { escapeValue: false },
  });
  return i;
}

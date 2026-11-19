// Translations of workspace D (F057 T003, AGENTS §7): `platform.*` keys of the `portal` namespace, shipped inside the
// lazy chunk of the workspace and added to the i18n instance when the workspace mounts (vi-VN default, en-US).
import type { i18n as I18n } from 'i18next';
import { PORTAL_NS } from '../../../i18n';
import en from './en.json';
import vi from './vi.json';

export const platformResources = { 'vi-VN': vi, 'en-US': en } as const;

const registered = new WeakSet<I18n>();

/** Adds the `platform.*` bundle once per i18n instance (idempotent, synchronous). */
export function registerPlatformI18n(i18n: I18n): void {
  if (registered.has(i18n)) return;
  for (const [lng, bundle] of Object.entries(platformResources)) i18n.addResourceBundle(lng, PORTAL_NS, { platform: bundle }, true, true);
  registered.add(i18n);
}

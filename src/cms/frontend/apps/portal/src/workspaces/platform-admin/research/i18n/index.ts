// Translations of the research screens (F035 US8, AGENTS §7): `platform.research.*` keys of the `portal` namespace,
// added to the i18n instance with the workspace D bundle (vi-VN default, en-US).
import type { i18n as I18n } from 'i18next';
import { PORTAL_NS } from '../../../../i18n';
import en from './en.json';
import vi from './vi.json';

export const researchResources = { 'vi-VN': vi, 'en-US': en } as const;

const registered = new WeakSet<I18n>();

/** Adds the `platform.research.*` bundle once per i18n instance (idempotent, synchronous). */
export function registerResearchI18n(i18n: I18n): void {
  if (registered.has(i18n)) return;
  for (const [lng, bundle] of Object.entries(researchResources)) i18n.addResourceBundle(lng, PORTAL_NS, { platform: { research: bundle } }, true, true);
  registered.add(i18n);
}

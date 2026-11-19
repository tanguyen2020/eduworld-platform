// Display helpers (pure): localized names (`vi` required, `en` optional — ADR-0056) and dates in the UI locale.
import dayjs from 'dayjs';

export interface LocalizedLike {
  vi: string;
  en?: string;
}

export function localizedName(name: LocalizedLike | undefined | null, locale: string): string {
  if (!name) return '';
  return locale.startsWith('en') && name.en ? name.en : name.vi;
}

export function formatDateTime(value: string | null | undefined): string {
  return value ? dayjs(value).format('DD/MM/YYYY HH:mm') : '';
}

export function formatDate(value: string | null | undefined): string {
  return value ? dayjs(value).format('DD/MM/YYYY') : '';
}

// language: Accept-Language follows the UI language (API_DESIGN §2, ADR-0056).
export function applyLanguage(headers: Headers, locale: string): void {
  if (locale) headers.set('Accept-Language', locale);
}

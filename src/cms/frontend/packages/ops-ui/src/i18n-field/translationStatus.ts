// Bilingual learning text (ADR-0056, CONTENT_MODEL §8, question.schema.json `i18nText`): a plain string is written in
// the item's `content_language`; an object `{vi, en}` carries a translation. Field status for the badges of
// F053-AC-22: `complete` (Đủ), `partial` (only the original — Thiếu bản dịch), `missing_original` (Thiếu bản gốc, blocks
// Gửi duyệt / Xuất bản — F053-AC-23), `outdated` (server `outdated_fields`, Lỗi thời). Pure functions.

export type ContentLang = 'vi' | 'en';
export type I18nTextValue = string | { vi?: string; en?: string };
export type LangMap = Partial<Record<ContentLang, string>>;
export type FieldTranslationStatus = 'complete' | 'partial' | 'missing_original' | 'outdated' | 'empty';

export const CONTENT_LANGS: readonly ContentLang[] = ['vi', 'en'];

export const otherLang = (l: ContentLang): ContentLang => (l === 'vi' ? 'en' : 'vi');

/** Value → per-language map (a plain string belongs to `contentLanguage`). */
export function toLangMap(value: I18nTextValue | null | undefined, contentLanguage: ContentLang): LangMap {
  if (value === null || value === undefined) return {};
  if (typeof value === 'string') return value ? { [contentLanguage]: value } : {};
  const out: LangMap = {};
  for (const l of CONTENT_LANGS) if (value[l]) out[l] = value[l];
  return out;
}

/** Per-language map → stored value: empty languages dropped (minLength 1); only the original → plain string. */
export function fromLangMap(map: LangMap, contentLanguage: ContentLang): I18nTextValue | undefined {
  const clean: LangMap = {};
  for (const l of CONTENT_LANGS) if (map[l] && map[l]!.length > 0) clean[l] = map[l];
  const langs = Object.keys(clean) as ContentLang[];
  if (langs.length === 0) return undefined;
  if (langs.length === 1 && langs[0] === contentLanguage) return clean[contentLanguage];
  return clean;
}

export function fieldStatus(value: I18nTextValue | null | undefined, contentLanguage: ContentLang, outdated = false): FieldTranslationStatus {
  const m = toLangMap(value, contentLanguage);
  if (!m.vi && !m.en) return 'empty';
  if (!m[contentLanguage]) return 'missing_original';
  if (outdated) return 'outdated';
  return m[otherLang(contentLanguage)] ? 'complete' : 'partial';
}

/** Text of a value in a language (fallback: the original). */
export function textIn(value: I18nTextValue | null | undefined, lang: ContentLang, contentLanguage: ContentLang): string {
  const m = toLangMap(value, contentLanguage);
  return m[lang] ?? m[contentLanguage] ?? '';
}

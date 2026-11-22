// @eduworld/ops-ui — Admin/Ops design system (ADR-0047). No API calls here (ADMIN_PORTAL §1.1).
export { OpsThemeProvider, useOpsTheme, resolveThemeConfig, DAYJS_LOCALES } from './theme/OpsThemeProvider';
export type { ThemeMode, Density, OpsLocale, OpsThemeState, OpsThemeProviderProps } from './theme/OpsThemeProvider';
export { lightTheme, darkTheme, compactTheme, compactOverrides, authTheme, domainColors, TOKENS_VERSION } from './theme/antd-theme';
export { useAccentColor, ACCENT_BORDER_WIDTH } from './theme/accent';
export type { Accent } from './theme/accent';
export { DomainTag, domainTokenName } from './components/DomainTag/DomainTag';
export type { DomainKind, DomainTagProps } from './components/DomainTag/DomainTag';
export { openapiRules } from './forms/openapiRules';
export type { OpenapiStringSchema, OpenapiRuleOptions, Translate } from './forms/openapiRules';
export { CharCounter, openapiInputProps } from './forms/CharCounter';
export { OPS_UI_NS, opsUiResources } from './i18n';
export { SearchSelect } from './select/SearchSelect';
export type { SearchSelectProps } from './select/SearchSelect';
export { foldVi, nodeText, optionText, viFilterOption } from './select/viSearch';

// Content editing components (F053: editor shell, formulas, bilingual fields, question editor) are exported from
// `@eduworld/ops-ui/content` so that MathJax / MathLive stay in the Content Studio chunk, out of the portal entry.

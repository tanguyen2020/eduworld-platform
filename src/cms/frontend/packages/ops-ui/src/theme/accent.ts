// Accent colour of KPI cards and dashboard widgets (DESIGN_SYSTEM §4.1 "Thẻ KPI/widget", teacher notebook theme):
// a semantic Ant Design colour of the active theme or a business colour of shared-tokens (`domainColors`, per mode).
// The width is the generated token `dashboard.accent.width`. Colour is never the only channel: cards keep a title.
import { theme } from 'antd';
import { domainColors } from './antd-theme';
import { useOpsTheme } from './OpsThemeProvider';

/** Semantic accent, or a business colour token name such as `color.subject.math`. */
export type Accent = 'primary' | 'success' | 'warning' | 'error' | 'info' | `color.${string}`;

/** Top border width of an accented card (CSS variable from tokens.css). */
export const ACCENT_BORDER_WIDTH = 'var(--ew-dashboard-accent-width)';

/** Resolves an accent to a colour of the active mode; undefined when no accent (or an unknown business colour). */
export function useAccentColor(accent: Accent | undefined): string | undefined {
  const { token } = theme.useToken();
  const { mode } = useOpsTheme();
  if (!accent) return undefined;
  switch (accent) {
    case 'primary':
      return token.colorPrimary;
    case 'success':
      return token.colorSuccess;
    case 'warning':
      return token.colorWarning;
    case 'error':
      return token.colorError;
    case 'info':
      return token.colorInfo;
    default:
      return domainColors[accent]?.[mode];
  }
}

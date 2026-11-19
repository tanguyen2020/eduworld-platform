// DomainTag (F050-AC-08, TC-UI-008, DESIGN_SYSTEM §4.4, §6): subject, rarity or content status shown with the
// business color from shared-tokens (`domainColors`, same names as the game) AND a text label — color is never
// the only channel. The dot is decorative (aria-hidden); the label carries the meaning.
import { Tag, theme } from 'antd';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../../i18n';
import { domainColors } from '../../theme/antd-theme';
import { useOpsTheme } from '../../theme/OpsThemeProvider';

export type DomainKind = 'subject' | 'rarity' | 'content_status' | 'brand';

export interface DomainTagProps {
  kind: DomainKind;
  /** Code as in the API, e.g. `math`, `epic`, `in_review`. */
  value: string;
}

/** Token name of a business color, e.g. `color.subject.math`. */
export const domainTokenName = (kind: DomainKind, value: string) => `color.${kind}.${value}`;

export function DomainTag({ kind, value }: DomainTagProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const { mode } = useOpsTheme();
  const { token } = theme.useToken();
  const name = domainTokenName(kind, value);
  const colors = domainColors[name];
  const label = t(`domain.${kind}.${value}`, { defaultValue: value });
  const dot = colors ? (
    <span
      aria-hidden="true"
      data-testid="domain-tag-dot"
      style={{ display: 'inline-block', width: token.sizeXS, height: token.sizeXS, borderRadius: '50%', marginInlineEnd: token.marginXS, backgroundColor: colors[mode] }}
    />
  ) : undefined;
  return (
    <Tag icon={dot} data-domain-token={name} data-domain-kind={kind}>
      {label}
    </Tag>
  );
}

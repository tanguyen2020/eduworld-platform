// MasteryHeatmap (F056 T026, F056-AC-09, NFR-A11Y-003, DESIGN_SYSTEM §4.4, SCHOOL_LMS §8.3): LO heatmap whose cells carry
// the mastery level symbol (○◔◑◕●) AND a token color — color is never the only channel. A real table (caption, column
// and row headers) so screen readers read "row · column · value"; every cell is focusable with the keyboard, has an
// aria-label and a tooltip with the exact value. Cell texts and descriptions come from the caller (server numbers).
import { Flex, Tooltip, Typography, theme } from 'antd';
import type { CSSProperties, ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export const MASTERY_LEVELS = ['not_started', 'beginning', 'developing', 'proficient', 'mastered'] as const;
export type MasteryLevel = (typeof MASTERY_LEVELS)[number];

/** Level symbols (SCHOOL_LMS §8.3): not a translation, a shape code shown next to the color. */
export const MASTERY_SYMBOL: Record<MasteryLevel, string> = {
  not_started: '○',
  beginning: '◔',
  developing: '◑',
  proficient: '◕',
  mastered: '●',
};

export interface HeatmapCell {
  level: MasteryLevel;
  /** Short visible text (e.g. a count or a percent, already formatted). */
  text: string;
  /** Exact value for the tooltip and the screen reader. */
  description: string;
  /** No student in this cell: drawn without fill. */
  muted?: boolean;
}

export interface HeatmapRow {
  key: string;
  header: ReactNode;
  cells: (HeatmapCell | null)[];
  /** Optional last column (e.g. "Giao ôn tập theo LO"). */
  action?: ReactNode;
}

export interface MasteryHeatmapProps {
  caption: string;
  rowHeader: string;
  columns: { key: string; label: ReactNode }[];
  rows: HeatmapRow[];
  actionHeader?: string;
}

type Tok = ReturnType<typeof theme.useToken>['token'];

/** Token colors of a level (Admin semantic tokens, ADR-0047 — no literal color). */
export function levelColors(level: MasteryLevel, token: Tok): { background: string; border: string } {
  switch (level) {
    case 'not_started':
      return { background: token.colorFillSecondary, border: token.colorBorder };
    case 'beginning':
      return { background: token.colorErrorBg, border: token.colorErrorBorder };
    case 'developing':
      return { background: token.colorWarningBg, border: token.colorWarningBorder };
    case 'proficient':
      return { background: token.colorInfoBg, border: token.colorInfoBorder };
    case 'mastered':
      return { background: token.colorSuccessBg, border: token.colorSuccessBorder };
  }
}

export function MasteryHeatmap({ caption, rowHeader, columns, rows, actionHeader }: MasteryHeatmapProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const { token } = theme.useToken();
  const cellStyle = (c: HeatmapCell): CSSProperties => {
    const colors = levelColors(c.level, token);
    return {
      background: c.muted ? 'transparent' : colors.background,
      border: `${token.lineWidth}px ${token.lineType} ${colors.border}`,
      padding: token.paddingXS,
      textAlign: 'center',
      minWidth: token.controlHeightLG * 2,
      whiteSpace: 'nowrap',
    };
  };
  const headStyle: CSSProperties = { padding: token.paddingXS, textAlign: 'start', fontWeight: token.fontWeightStrong };
  return (
    <Flex vertical gap="small">
      <div style={{ overflowX: 'auto' }}>
        <table data-testid="mastery-heatmap" style={{ borderCollapse: 'separate', borderSpacing: token.marginXXS }}>
          <caption style={{ textAlign: 'start', paddingBlockEnd: token.paddingXS }}>{caption}</caption>
          <thead>
            <tr>
              <th scope="col" style={headStyle}>
                {rowHeader}
              </th>
              {columns.map((c) => (
                <th key={c.key} scope="col" style={headStyle}>
                  {c.label}
                </th>
              ))}
              {actionHeader ? (
                <th scope="col" style={headStyle}>
                  {actionHeader}
                </th>
              ) : null}
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.key}>
                <th scope="row" style={headStyle}>
                  {r.header}
                </th>
                {r.cells.map((c, i) =>
                  c ? (
                    <Tooltip key={columns[i]?.key ?? i} title={c.description}>
                      <td tabIndex={0} aria-label={c.description} data-level={c.level} style={cellStyle(c)}>
                        <span aria-hidden="true">{MASTERY_SYMBOL[c.level]}</span> <span aria-hidden="true">{c.text}</span>
                      </td>
                    </Tooltip>
                  ) : (
                    <td key={columns[i]?.key ?? i} aria-label={t('dashboard.heatmap.noData')} />
                  ),
                )}
                {actionHeader ? <td style={{ padding: token.paddingXS }}>{r.action}</td> : null}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Flex wrap gap="middle" data-testid="mastery-legend" role="list" aria-label={t('dashboard.heatmap.legend')}>
        {MASTERY_LEVELS.map((l) => {
          const colors = levelColors(l, token);
          return (
            <Typography.Text key={l} role="listitem">
              <span aria-hidden="true" style={{ background: colors.background, border: `${token.lineWidth}px ${token.lineType} ${colors.border}`, paddingInline: token.paddingXXS, marginInlineEnd: token.marginXXS }}>
                {MASTERY_SYMBOL[l]}
              </span>
              {t(`dashboard.mastery.${l}`)}
            </Typography.Text>
          );
        })}
      </Flex>
    </Flex>
  );
}

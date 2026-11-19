// RichContentEditor (F053 T016, F053-AC-16, AC-21; question.schema.json richContent): structured blocks `paragraph`,
// `heading`, `list`, `math`, `image` (alt required), `audio`, `table`, `callout` — never free HTML. Text is bilingual
// (BilingualField) with a formula button; a math block uses MathInput. Blocks move with Lên / Xuống buttons (keyboard
// accessible). Media is chosen through the host's `mediaSlot` (the portal's MediaPicker: list + upload, F053-AC-05).
// Presentational: no API call.
import { Button, Card, Flex, Form, Input, InputNumber, Select, Space, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { BilingualField } from '../i18n-field/BilingualField';
import type { ContentLang, I18nTextValue } from '../i18n-field/translationStatus';
import { OPS_UI_NS } from '../i18n';
import { MathInput } from '../math/MathInput';
import { MathInsertButton } from '../math/MathInsertButton';
import type { ReactNode } from 'react';
import { BASIC_BLOCKS, emptyBlock, RICH_ALT_MAX, RICH_LATEX_MAX, RICH_TEXT_MAX, type RichBlock, type RichBlockType, type RichContent } from './types';

export interface MediaSlotArgs {
  kind: 'image' | 'audio' | 'video';
  value?: string;
  onChange: (assetId: string | undefined) => void;
  /** Field id (jump target). */
  id: string;
}

export interface RichContentEditorProps {
  /** Field id prefix, e.g. `stem` → `stem.blocks.0.text`. */
  id: string;
  label: string;
  value: RichContent | undefined;
  onChange: (value: RichContent) => void;
  contentLanguage: ContentLang;
  allowedBlocks?: readonly RichBlockType[];
  readOnly?: boolean;
  /** Errors by field id (422 mapping, formula errors). */
  errors?: Readonly<Record<string, string>>;
  outdatedFields?: ReadonlySet<string>;
  mediaSlot?: (args: MediaSlotArgs) => ReactNode;
  /** At least one block with text (stem, option). */
  required?: boolean;
}

export function RichContentEditor({ id, label, value, onChange, contentLanguage, allowedBlocks = BASIC_BLOCKS, readOnly = false, errors = {}, outdatedFields, mediaSlot, required = false }: RichContentEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const blocks = value?.blocks ?? [];
  const set = (next: RichBlock[]) => onChange({ ...(value ?? {}), blocks: next });
  const update = (i: number, patch: Partial<RichBlock>) => set(blocks.map((b, j) => (j === i ? { ...b, ...patch } : b)));
  const move = (i: number, d: -1 | 1) => {
    const j = i + d;
    if (j < 0 || j >= blocks.length) return;
    const next = [...blocks];
    [next[i], next[j]] = [next[j]!, next[i]!];
    set(next);
  };
  const remove = (i: number) => set(blocks.filter((_, j) => j !== i));
  const tools = (lang: ContentLang, insert: (s: string) => void) => <MathInsertButton onInsert={insert} language={lang} disabled={readOnly} />;

  const textField = (i: number, b: RichBlock, multiline: boolean) => {
    const fid = `${id}.blocks.${i}.text`;
    return (
      <BilingualField
        id={fid}
        label={t(`richContent.block.${b.type}`)}
        value={b.text}
        onChange={(v) => update(i, { text: v })}
        contentLanguage={contentLanguage}
        maxLength={RICH_TEXT_MAX}
        required={required && i === 0}
        multiline={multiline}
        readOnly={readOnly}
        error={errors[fid]}
        outdated={outdatedFields?.has(fid)}
        renderTools={tools}
      />
    );
  };

  const blockBody = (b: RichBlock, i: number) => {
    const fid = `${id}.blocks.${i}`;
    switch (b.type) {
      case 'paragraph':
      case 'callout':
        return textField(i, b, true);
      case 'heading':
        return (
          <Flex vertical>
            <Form.Item label={t('richContent.headingLevel')}>
              <InputNumber min={1} max={4} value={b.level ?? 2} disabled={readOnly} onChange={(v) => update(i, { level: v ?? 2 })} />
            </Form.Item>
            {textField(i, b, false)}
          </Flex>
        );
      case 'list':
        return (
          <Flex vertical gap="small">
            {(b.items ?? []).map((item, k) => (
              <Flex key={k} gap="small" align="start">
                <div style={{ flex: 1 }}>
                  <BilingualField
                    id={`${fid}.items.${k}`}
                    label={t('richContent.listItem', { n: k + 1 })}
                    value={item}
                    onChange={(v) => update(i, { items: (b.items ?? []).map((x, m) => (m === k ? (v as I18nTextValue) : x)) })}
                    contentLanguage={contentLanguage}
                    maxLength={RICH_TEXT_MAX}
                    readOnly={readOnly}
                    error={errors[`${fid}.items.${k}`]}
                    renderTools={tools}
                  />
                </div>
                {!readOnly && (
                  <Button size="small" onClick={() => update(i, { items: (b.items ?? []).filter((_, m) => m !== k) })}>
                    {t('richContent.remove')}
                  </Button>
                )}
              </Flex>
            ))}
            {!readOnly && (
              <Button size="small" onClick={() => update(i, { items: [...(b.items ?? []), ''] })}>
                {t('richContent.addItem')}
              </Button>
            )}
          </Flex>
        );
      case 'math':
        return (
          <Form.Item label={t('richContent.block.math')} validateStatus={errors[`${fid}.latex`] ? 'error' : undefined} data-field={`${fid}.latex`}>
            <MathInput
              label={t('richContent.block.math')}
              value={b.latex ?? ''}
              onChange={(v) => update(i, { latex: v.slice(0, RICH_LATEX_MAX) })}
              display
              error={errors[`${fid}.latex`]}
              language={contentLanguage}
              readOnly={readOnly}
            />
          </Form.Item>
        );
      case 'image':
      case 'audio':
      case 'video':
        return (
          <Flex vertical gap="small">
            {mediaSlot ? (
              mediaSlot({ kind: b.type, value: b.media_ref, onChange: (v) => update(i, { media_ref: v }), id: `${fid}.media_ref` })
            ) : (
              <Typography.Text type="secondary">{b.media_ref ?? t('richContent.noMedia')}</Typography.Text>
            )}
            {errors[`${fid}.media_ref`] && <Typography.Text type="danger">{errors[`${fid}.media_ref`]}</Typography.Text>}
            {b.type === 'image' && (
              <Form.Item
                label={t('richContent.alt')}
                required
                validateStatus={!b.alt || errors[`${fid}.alt`] ? 'error' : undefined}
                help={errors[`${fid}.alt`] ?? (!b.alt ? t('richContent.altRequired') : undefined)}
                data-field={`${fid}.alt`}
              >
                <Input id={`${fid}.alt`} value={b.alt ?? ''} maxLength={RICH_ALT_MAX} showCount readOnly={readOnly} onChange={(e) => update(i, { alt: e.target.value })} />
              </Form.Item>
            )}
          </Flex>
        );
      case 'table':
        return <TableBlock block={b} readOnly={readOnly} onChange={(rows) => update(i, { rows })} />;
      default:
        return null;
    }
  };

  return (
    <Flex vertical gap="small" data-field={id} aria-label={label} role="group">
      <Typography.Text strong>{label}</Typography.Text>
      {blocks.map((b, i) => (
        <Card
          key={i}
          size="small"
          title={t(`richContent.block.${b.type}`)}
          extra={
            !readOnly && (
              <Space size="small">
                <Button size="small" disabled={i === 0} aria-label={t('richContent.moveUp')} onClick={() => move(i, -1)}>
                  {t('richContent.up')}
                </Button>
                <Button size="small" disabled={i === blocks.length - 1} aria-label={t('richContent.moveDown')} onClick={() => move(i, 1)}>
                  {t('richContent.down')}
                </Button>
                <Button size="small" danger onClick={() => remove(i)}>
                  {t('richContent.remove')}
                </Button>
              </Space>
            )
          }
        >
          {blockBody(b, i)}
        </Card>
      ))}
      {!readOnly && (
        <Select<RichBlockType>
          aria-label={t('richContent.addBlock')}
          placeholder={t('richContent.addBlock')}
          value={null as unknown as RichBlockType}
          options={allowedBlocks.map((type) => ({ value: type, label: t(`richContent.block.${type}`) }))}
          onChange={(type) => set([...blocks, emptyBlock(type)])}
        />
      )}
    </Flex>
  );
}

function TableBlock({ block, readOnly, onChange }: { block: RichBlock; readOnly: boolean; onChange: (rows: I18nTextValue[][]) => void }) {
  const { t } = useTranslation(OPS_UI_NS);
  const rows = (block.rows ?? [['']]) as string[][];
  const cols = Math.max(1, ...rows.map((r) => r.length));
  const setCell = (r: number, c: number, v: string) => onChange(rows.map((row, i) => (i === r ? row.map((x, j) => (j === c ? v : x)) : row)));
  return (
    <Flex vertical gap="small">
      <table>
        <tbody>
          {rows.map((row, r) => (
            <tr key={r}>
              {Array.from({ length: cols }, (_, c) => (
                <td key={c}>
                  <Input aria-label={t('richContent.cell', { row: r + 1, col: c + 1 })} value={typeof row[c] === 'string' ? row[c] : ''} maxLength={RICH_TEXT_MAX} readOnly={readOnly} onChange={(e) => setCell(r, c, e.target.value)} />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {!readOnly && (
        <Space>
          <Button size="small" onClick={() => onChange([...rows, Array.from({ length: cols }, () => '')])}>
            {t('richContent.addRow')}
          </Button>
          <Button size="small" onClick={() => onChange(rows.map((r) => [...r, '']))}>
            {t('richContent.addColumn')}
          </Button>
        </Space>
      )}
    </Flex>
  );
}

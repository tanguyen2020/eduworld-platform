// BilingualField (F053-AC-21, AC-22, ADMIN_PORTAL §1.5, ADR-0056): a learning text edited in two columns vi / en
// (tabs VI | EN on narrow screens); the `content_language` column comes first and is labelled "Gốc". A badge shows
// the field status (Đủ / Thiếu / Lỗi thời / Thiếu bản gốc). Each language input is capped by the OpenAPI/JSON Schema
// `maxLength` with a counter (ADR-0041). `renderTools(lang, insert)` lets the caller add the formula button.
import { Badge, Col, Flex, Form, Grid, Input, Row, Tabs, Tag, theme, Typography } from 'antd';
import { useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { CharCounter } from '../forms/CharCounter';
import { OPS_UI_NS } from '../i18n';
import { fieldStatus, fromLangMap, otherLang, toLangMap, type ContentLang, type FieldTranslationStatus, type I18nTextValue } from './translationStatus';

export interface BilingualFieldProps {
  /** DOM id prefix (jump target of the blocker list). */
  id: string;
  label: string;
  value: I18nTextValue | undefined;
  onChange: (value: I18nTextValue | undefined) => void;
  contentLanguage: ContentLang;
  maxLength: number;
  required?: boolean;
  multiline?: boolean;
  /** Server `outdated_fields` contains this field. */
  outdated?: boolean;
  readOnly?: boolean;
  /** Error message shown under the field (422 mapping, formula error…). */
  error?: string;
  layout?: 'columns' | 'tabs';
  /** Extra controls per language; `insert(text)` inserts at the caret. */
  renderTools?: (lang: ContentLang, insert: (text: string) => void) => ReactNode;
}

const STATUS_COLOR: Record<FieldTranslationStatus, 'success' | 'warning' | 'error' | 'default'> = {
  complete: 'success',
  partial: 'warning',
  missing_original: 'error',
  outdated: 'warning',
  empty: 'default',
};

export function TranslationBadge({ status }: { status: FieldTranslationStatus }) {
  const { t } = useTranslation(OPS_UI_NS);
  if (status === 'empty') return null;
  return (
    <Tag data-translation-status={status} color={STATUS_COLOR[status]}>
      {t(`bilingual.status.${status}`)}
    </Tag>
  );
}

export function BilingualField(props: BilingualFieldProps) {
  const { id, label, value, onChange, contentLanguage, maxLength, required = false, multiline = false, outdated = false, readOnly = false, error, renderTools } = props;
  const { t } = useTranslation(OPS_UI_NS);
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const layout = props.layout ?? (screens.lg ? 'columns' : 'tabs');
  const map = toLangMap(value, contentLanguage);
  const status = fieldStatus(value, contentLanguage, outdated);
  const order: ContentLang[] = [contentLanguage, otherLang(contentLanguage)];
  const inputs = useRef<Partial<Record<ContentLang, HTMLTextAreaElement | HTMLInputElement | null>>>({});

  const setLang = (lang: ContentLang, text: string) => onChange(fromLangMap({ ...map, [lang]: text }, contentLanguage));
  const insertAt = (lang: ContentLang) => (text: string) => {
    const el = inputs.current[lang];
    const cur = map[lang] ?? '';
    const pos = el && typeof el.selectionStart === 'number' ? el.selectionStart : cur.length;
    setLang(lang, (cur.slice(0, pos) + text + cur.slice(pos)).slice(0, maxLength));
  };

  const editor = (lang: ContentLang) => {
    const isOriginal = lang === contentLanguage;
    const missing = required && isOriginal && !map[lang];
    const common = {
      id: `${id}.${lang}`,
      'aria-label': t('bilingual.inputLabel', { label, lang: t(`bilingual.lang.${lang}`) }),
      'aria-required': required && isOriginal,
      'aria-invalid': missing || !!error,
      value: map[lang] ?? '',
      readOnly,
      maxLength,
      status: missing || error ? ('error' as const) : undefined,
      onChange: (e: { target: { value: string } }) => setLang(lang, e.target.value),
    };
    return (
      <Flex vertical gap="small" key={lang}>
        <Flex justify="space-between" align="center">
          <Typography.Text>
            {t(`bilingual.lang.${lang}`)}
            {isOriginal && (
              <Tag bordered={false} color="processing" style={{ marginInlineStart: token.marginXS }}>
                {t('bilingual.original')}
              </Tag>
            )}
          </Typography.Text>
          <CharCounter count={(map[lang] ?? '').length} max={maxLength} />
        </Flex>
        {multiline ? (
          <Input.TextArea {...common} ref={(r) => void (inputs.current[lang] = r?.resizableTextArea?.textArea ?? null)} rows={3} />
        ) : (
          <Input {...common} ref={(r) => void (inputs.current[lang] = r?.input ?? null)} />
        )}
        {!readOnly && renderTools?.(lang, insertAt(lang))}
      </Flex>
    );
  };

  return (
    <Form.Item
      label={
        <Flex gap="small" align="center">
          <span>{label}</span>
          <TranslationBadge status={status} />
        </Flex>
      }
      required={required}
      validateStatus={error || (required && !map[contentLanguage]) ? 'error' : undefined}
      help={error ?? (required && !map[contentLanguage] && status !== 'empty' ? t('bilingual.originalRequired') : undefined)}
      data-field={id}
    >
      {layout === 'columns' ? (
        <Row gutter={token.margin}>
          {order.map((l) => (
            <Col span={12} key={l}>
              {editor(l)}
            </Col>
          ))}
        </Row>
      ) : (
        <Tabs
          size="small"
          items={order.map((l) => ({
            key: l,
            label: (
              <Badge dot={!map[l]} offset={[token.marginXXS, 0]}>
                {t(`bilingual.tab.${l}`)}
              </Badge>
            ),
            children: editor(l),
          }))}
        />
      )}
    </Form.Item>
  );
}

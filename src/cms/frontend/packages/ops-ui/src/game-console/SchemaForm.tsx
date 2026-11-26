// SchemaForm (F079-AC-02, AC-05, AC-08, AC-15, AC-20; ADMIN_PORTAL §1.4, ADR-0041): the editor form of a game config
// payload generated from the JSON Schema of its type (specs/schemas/*.schema.json): one control per property — text
// capped by `maxLength` with a counter, number with min/max, switch, searchable select for `enum`, bilingual vi/en text
// (`{vi, en}` objects, ADR-0056) with its translation status, nested objects and arrays (add / remove / move). `const`
// properties (schema_version, `paid_track: false`…) are filled and never shown; `managedKeys` (ids/version the server
// fills) are hidden. `renderOverride` lets a builder replace one field (reference pickers from the client manifest,
// F079-AC-07). Server field errors (`errors[].field`, any separator) are shown under their field. Presentational and
// controlled: no API call, no business rule — the server validates the payload again (AGENTS R1).
import { DatePicker, Flex, Form, Input, InputNumber, Switch, Button, Card, Typography, theme, Row, Col, Tabs, Tag } from 'antd';
import dayjs from 'dayjs';
import { useMemo, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { openapiInputProps } from '../forms/CharCounter';
import { OPS_UI_NS } from '../i18n';
import { TranslationBadge } from '../i18n-field/BilingualField';
import { SearchSelect } from '../select/SearchSelect';
import { canonicalPath, effective, initialValue, isLocalized, languageSchema, pathKey, setIn, typeOf, type JsonSchema, type PathSegment } from './schemaModel';

export interface SchemaFieldContext {
  path: PathSegment[];
  /** Property name (the last string segment of the path). */
  key: string;
  schema: JsonSchema;
  value: unknown;
  onChange: (value: unknown) => void;
  readOnly: boolean;
  label: string;
  id: string;
  required: boolean;
  error?: string;
}

export interface SchemaFormProps {
  schema: JsonSchema;
  value: unknown;
  onChange: (value: unknown) => void;
  /** Field label of a property (the caller translates; `path` is the full path). */
  label: (key: string, path: PathSegment[]) => string;
  /** Root properties filled by the server (item id, version id, version, status, owner scope). */
  managedKeys?: readonly string[];
  /** Root properties not rendered here (a builder edits them in its own panel). */
  omitKeys?: readonly string[];
  /** Server field errors by field path (any separator; see canonicalPath). */
  errors?: Record<string, string>;
  readOnly?: boolean;
  /** Replaces the control of one field; return undefined to keep the generated one. */
  renderOverride?: (ctx: SchemaFieldContext) => ReactNode | undefined;
  /** DOM id prefix of the fields (jump target of the blocker list). */
  idPrefix?: string;
  /** Path of `value` inside the payload (a builder rendering a sub-object, e.g. one quest step). */
  basePath?: PathSegment[];
  /**
   * Translation state of the game texts from the server (F082-AC-10): `outdated` = JSON pointers of `outdated_fields`
   * (any separator, `/vi`/`/en` suffix ignored); `onShowDiff` opens the diff of the original of that pointer.
   */
  translation?: { outdated?: readonly string[]; onShowDiff?: (pointer: string) => void };
  /** Layout of the bilingual texts: two columns or VI | EN tabs (default: columns on large screens). */
  layout?: 'columns' | 'tabs';
}

/** Canonical key of a game-text pointer: `/steps/0/title/en` → `steps/0/title`. */
export function textKey(pointer: string): string {
  return canonicalPath(pointer).replace(/\/(vi|en)$/, '');
}

interface Ctx {
  root: JsonSchema;
  label: SchemaFormProps['label'];
  errors: Record<string, string>;
  readOnly: boolean;
  override?: SchemaFormProps['renderOverride'];
  idPrefix: string;
  managed: Set<string>;
  omit: Set<string>;
  basePath: PathSegment[];
  outdated: Map<string, string>;
  onShowDiff?: (pointer: string) => void;
  layout?: 'columns' | 'tabs';
}

const LONG_TEXT = 255;
const LANGS = ['vi', 'en'] as const;
const ERROR_STATUS = 'error' as const;

export function SchemaForm({ schema, value, onChange, label, managedKeys = [], omitKeys = [], errors = {}, readOnly = false, renderOverride, idPrefix = 'field', basePath = [], translation, layout }: SchemaFormProps) {
  const errorMap = useMemo(() => Object.fromEntries(Object.entries(errors).map(([k, v]) => [canonicalPath(k), v])), [errors]);
  const outdated = useMemo(() => new Map((translation?.outdated ?? []).map((p) => [textKey(p), p])), [translation?.outdated]);
  const ctx: Ctx = {
    root: schema,
    label,
    errors: errorMap,
    readOnly,
    override: renderOverride,
    idPrefix,
    managed: new Set(managedKeys),
    omit: new Set(omitKeys),
    basePath,
    outdated,
    onShowDiff: translation?.onShowDiff,
    layout,
  };
  const s = effective(schema, schema);
  return (
    <Form layout="vertical" component="div" disabled={readOnly}>
      <ObjectFields ctx={ctx} schema={s} path={[]} value={value} onChange={onChange} />
    </Form>
  );
}

function ObjectFields({ ctx, schema, path, value, onChange }: { ctx: Ctx; schema: JsonSchema; path: PathSegment[]; value: unknown; onChange: (v: unknown) => void }) {
  const obj = value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
  const required = new Set(schema.required ?? []);
  return (
    <>
      {Object.entries(schema.properties ?? {}).map(([key, prop]) => {
        if (path.length === 0 && (ctx.managed.has(key) || ctx.omit.has(key))) return null;
        const eff = effective(prop, ctx.root);
        if (eff.const !== undefined) return null;
        return (
          <Field
            key={key}
            ctx={ctx}
            name={key}
            schema={eff}
            path={[...path, key]}
            required={required.has(key)}
            value={obj[key]}
            onChange={(v) => onChange(setIn(obj, [key], v))}
          />
        );
      })}
    </>
  );
}

function Field({ ctx, name, schema, path, required, value, onChange }: { ctx: Ctx; name: string; schema: JsonSchema; path: PathSegment[]; required: boolean; value: unknown; onChange: (v: unknown) => void }) {
  const { t } = useTranslation(OPS_UI_NS);
  const full = [...ctx.basePath, ...path];
  const key = pathKey(full);
  const id = `${ctx.idPrefix}-${full.join('-')}`;
  const label = ctx.label(name, full);
  const error = ctx.errors[key];
  const override = ctx.override?.({ path: full, key: name, schema, value, onChange, readOnly: ctx.readOnly, label, id, required, error });
  const item = (control: ReactNode, extra?: ReactNode) => (
    <Form.Item label={label} htmlFor={id} required={required} validateStatus={error ? 'error' : undefined} help={error} extra={extra ?? schema.description} data-testid={`field-${full.join('-')}`}>
      {control}
    </Form.Item>
  );
  if (override !== undefined) return <>{item(override)}</>;

  const type = typeOf(schema);
  if (isLocalized(schema)) return <LocalizedInput ctx={ctx} id={id} label={label} schema={schema} path={full} required={required} value={value} onChange={onChange} error={error} />;
  if (schema.enum) {
    return item(
      <SearchSelect
        id={id}
        aria-label={label}
        allowClear={!required}
        disabled={ctx.readOnly}
        value={value as never}
        onChange={(v) => onChange(v === null ? undefined : v)}
        options={schema.enum.map((v) => ({ value: v as string, label: String(v) }))}
      />,
    );
  }
  switch (type) {
    case 'string': {
      if (schema.format === 'date-time' || schema.format === 'date') {
        const dateOnly = schema.format === 'date';
        return item(
          <DatePicker
            id={id}
            aria-label={label}
            showTime={dateOnly ? false : { format: 'HH:mm' }}
            format={dateOnly ? 'DD/MM/YYYY' : 'DD/MM/YYYY HH:mm'}
            value={typeof value === 'string' && value ? dayjs(value) : null}
            onChange={(d) => onChange(d ? (dateOnly ? d.format('YYYY-MM-DD') : d.toISOString()) : undefined)}
          />,
        );
      }
      const long = (schema.maxLength ?? 0) > LONG_TEXT;
      const common = { id, 'aria-label': label, value: typeof value === 'string' ? value : '', onChange: (e: { target: { value: string } }) => onChange(e.target.value === '' ? undefined : e.target.value), ...openapiInputProps(schema as never) };
      return item(long ? <Input.TextArea {...common} autoSize={{ minRows: 2, maxRows: 8 }} /> : <Input {...common} />);
    }
    case 'integer':
    case 'number':
      return item(
        <InputNumber
          id={id}
          aria-label={label}
          min={schema.minimum}
          max={schema.maximum}
          precision={type === 'integer' ? 0 : undefined}
          value={typeof value === 'number' ? value : null}
          onChange={(v) => onChange(v === null ? undefined : v)}
        />,
      );
    case 'boolean':
      return item(<Switch id={id} aria-label={label} checked={value === true} onChange={(v) => onChange(v)} />);
    case 'array':
      return <ArrayField ctx={ctx} id={id} label={label} schema={schema} path={path} required={required} value={value} onChange={onChange} error={error} />;
    case 'object': {
      if (!schema.properties || Object.keys(schema.properties).length === 0) return item(<JsonInput id={id} label={label} value={value} onChange={onChange} readOnly={ctx.readOnly} />);
      const present = value !== undefined && value !== null;
      return (
        <fieldset aria-label={label} id={id} data-testid={`field-${full.join('-')}`}>
          <Card
            size="small"
            title={label}
            extra={
              !required && !ctx.readOnly ? (
                present ? (
                  <Button size="small" type="link" onClick={() => onChange(undefined)}>
                    {t('game.form.removeObject')}
                  </Button>
                ) : (
                  <Button size="small" type="link" onClick={() => onChange(initialValue(schema, ctx.root) ?? {})}>
                    {t('game.form.addObject')}
                  </Button>
                )
              ) : null
            }
          >
            {error && <Typography.Text type="danger">{error}</Typography.Text>}
            {present || required ? <ObjectFields ctx={ctx} schema={schema} path={path} value={value} onChange={onChange} /> : <Typography.Text type="secondary">{t('game.form.notSet')}</Typography.Text>}
          </Card>
        </fieldset>
      );
    }
    default:
      return item(<JsonInput id={id} label={label} value={value} onChange={onChange} readOnly={ctx.readOnly} />);
  }
}

function LocalizedInput({ ctx, id, label, schema, path, required, value, onChange, error }: { ctx: Ctx; id: string; label: string; schema: JsonSchema; path: PathSegment[]; required: boolean; value: unknown; onChange: (v: unknown) => void; error?: string }) {
  const { t } = useTranslation(OPS_UI_NS);
  const { token } = theme.useToken();
  const layout = ctx.layout ?? 'columns';
  const v = (value && typeof value === 'object' ? value : {}) as Record<string, string | undefined>;
  const filled = ['vi', 'en'].filter((l) => (v[l] ?? '').trim() !== '').length;
  const outdatedPointer = ctx.outdated.get(pathKey(path));
  const status = filled === 2 ? (outdatedPointer ? 'outdated' : 'complete') : filled === 1 ? 'partial' : 'empty';
  const set = (lang: 'vi' | 'en', text: string) => {
    const next = { ...v, [lang]: text === '' ? undefined : text };
    onChange(next.vi === undefined && next.en === undefined ? undefined : next);
  };
  const editor = (lang: 'vi' | 'en') => {
    const ls = effective(languageSchema(schema, lang), ctx.root);
    const langError = ctx.errors[pathKey([...path, lang])];
    const long = (ls.maxLength ?? 0) > LONG_TEXT;
    const common = {
      id: `${id}-${lang}`,
      'aria-label': `${label} (${lang.toUpperCase()})`,
      value: v[lang] ?? '',
      status: langError ? ERROR_STATUS : undefined,
      onChange: (e: { target: { value: string } }) => set(lang, e.target.value),
      ...openapiInputProps(ls as never),
    };
    return (
      <Flex vertical gap="small">
        {layout === 'columns' && (
          <Flex gap="small" align="center">
            <Typography.Text type="secondary">{lang.toUpperCase()}</Typography.Text>
            {lang === 'vi' && (
              <Tag bordered={false} color="processing">
                {t('bilingual.original')}
              </Tag>
            )}
          </Flex>
        )}
        {long ? <Input.TextArea {...common} autoSize={{ minRows: 2, maxRows: 6 }} /> : <Input {...common} />}
        {langError && <Typography.Text type="danger">{langError}</Typography.Text>}
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
      extra={
        outdatedPointer && ctx.onShowDiff ? (
          <Button size="small" type="link" onClick={() => ctx.onShowDiff?.(outdatedPointer)}>
            {t('game.form.showOriginalDiff')}
          </Button>
        ) : undefined
      }
      required={required}
      validateStatus={error ? 'error' : undefined}
      help={error}
      data-testid={`field-${path.join('-')}`}
    >
      {layout === 'columns' ? (
        <Row gutter={[token.marginXS, token.marginXS]}>
          {LANGS.map((lang) => (
            <Col xs={24} lg={12} key={lang}>
              {editor(lang)}
            </Col>
          ))}
        </Row>
      ) : (
        <Tabs
          size="small"
          items={LANGS.map((lang) => ({ key: lang, label: lang.toUpperCase(), children: editor(lang) }))}
        />
      )}
    </Form.Item>
  );
}

function ArrayField({ ctx, id, label, schema, path, required, value, onChange, error }: { ctx: Ctx; id: string; label: string; schema: JsonSchema; path: PathSegment[]; required: boolean; value: unknown; onChange: (v: unknown) => void; error?: string }) {
  const { t } = useTranslation(OPS_UI_NS);
  const list = Array.isArray(value) ? value : [];
  const items = effective(schema.items, ctx.root);
  const itemType = typeOf(items);
  const full = [...ctx.basePath, ...path];
  // primitive lists: one multi-select (enum) or a tag list
  if (items.enum || itemType === 'string' || itemType === 'integer' || itemType === 'number') {
    const override = ctx.override?.({ path: full, key: String(path[path.length - 1]), schema, value, onChange, readOnly: ctx.readOnly, label, id, required, error });
    return (
      <Form.Item label={label} htmlFor={id} required={required} validateStatus={error ? 'error' : undefined} help={error} data-testid={`field-${full.join('-')}`}>
        {override ?? (
          <SearchSelect
            id={id}
            aria-label={label}
            mode={items.enum ? 'multiple' : 'tags'}
            disabled={ctx.readOnly}
            value={list as never}
            maxCount={schema.maxItems}
            onChange={(v) => {
              const arr = (v as unknown[]).map((x) => (itemType === 'integer' || itemType === 'number' ? Number(x) : x));
              onChange(arr.length ? arr : undefined);
            }}
            options={(items.enum ?? []).map((v) => ({ value: v as string, label: String(v) }))}
          />
        )}
      </Form.Item>
    );
  }
  const move = (from: number, to: number) => {
    const next = [...list];
    const [x] = next.splice(from, 1);
    next.splice(to, 0, x);
    onChange(next);
  };
  const full2 = (i: number): PathSegment[] => [...path, i];
  return (
    <fieldset aria-label={label} id={id} data-testid={`field-${full.join('-')}`}>
      <Card
        size="small"
        title={
          <Flex gap="small" align="center">
            <span>{label}</span>
            <Typography.Text type="secondary">{t('game.form.count', { count: list.length, max: schema.maxItems ?? '∞' })}</Typography.Text>
          </Flex>
        }
      >
        {error && <Typography.Text type="danger">{error}</Typography.Text>}
        <Flex vertical gap="small">
          {list.map((item, i) => (
            <Card
              key={i}
              size="small"
              type="inner"
              title={t('game.form.item', { n: i + 1 })}
              extra={
                ctx.readOnly ? null : (
                  <Flex gap="small">
                    <Button size="small" aria-label={t('game.form.moveUp', { n: i + 1 })} disabled={i === 0} onClick={() => move(i, i - 1)}>{t('game.form.up')}</Button>
                    <Button size="small" aria-label={t('game.form.moveDown', { n: i + 1 })} disabled={i === list.length - 1} onClick={() => move(i, i + 1)}>{t('game.form.down')}</Button>
                    <Button size="small" danger aria-label={t('game.form.removeItem', { n: i + 1 })} onClick={() => onChange(list.filter((_, j) => j !== i))}>{t('game.form.remove')}</Button>
                  </Flex>
                )
              }
            >
              {itemType === 'object' ? (
                <ObjectFields ctx={ctx} schema={items} path={full2(i)} value={item} onChange={(v) => onChange(list.map((x, j) => (j === i ? v : x)))} />
              ) : (
                <JsonInput id={`${id}-${i}`} label={t('game.form.item', { n: i + 1 })} value={item} onChange={(v) => onChange(list.map((x, j) => (j === i ? v : x)))} readOnly={ctx.readOnly} />
              )}
            </Card>
          ))}
          {!ctx.readOnly && (
            <Button
              disabled={schema.maxItems !== undefined && list.length >= schema.maxItems}
              onClick={() => onChange([...list, initialValue(items, ctx.root) ?? (itemType === 'object' ? {} : undefined)])}
            >
              {t('game.form.add', { label })}
            </Button>
          )}
        </Flex>
      </Card>
    </fieldset>
  );
}

/** Free-form JSON of a node the schema does not describe field by field (validated by the server). */
function JsonInput({ id, label, value, onChange, readOnly }: { id: string; label: string; value: unknown; onChange: (v: unknown) => void; readOnly: boolean }) {
  const { t } = useTranslation(OPS_UI_NS);
  const [text, setText] = useState(() => (value === undefined ? '' : JSON.stringify(value, null, 2)));
  const [bad, setBad] = useState(false);
  return (
    <Flex vertical gap="small">
      <Input.TextArea
        id={id}
        aria-label={label}
        value={text}
        disabled={readOnly}
        status={bad ? 'error' : undefined}
        autoSize={{ minRows: 2, maxRows: 10 }}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => {
          if (text.trim() === '') {
            setBad(false);
            onChange(undefined);
            return;
          }
          try {
            onChange(JSON.parse(text));
            setBad(false);
          } catch {
            setBad(true);
          }
        }}
      />
      <Typography.Text type={bad ? 'danger' : 'secondary'}>{bad ? t('game.form.jsonInvalid') : t('game.form.jsonHint')}</Typography.Text>
    </Flex>
  );
}


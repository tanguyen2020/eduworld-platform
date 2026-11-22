// Editors of the 11 QuestionType values (F053-AC-01, AC-02, AC-03; QUESTION_BANK §3). Each edits the type's
// `render_payload` + `answer_key` (question.schema.json `$defs/<type>`); ids come from itemId (never A/B/C/D).
// Registry only — QuestionEditor picks the editor of the draft type.
import { Button, Card, Checkbox, Flex, Form, Input, InputNumber, Radio, Space, Switch, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { BilingualField } from '../i18n-field/BilingualField';
import type { ContentLang, I18nTextValue } from '../i18n-field/translationStatus';
import { OPS_UI_NS } from '../i18n';
import { RichContentEditor, type MediaSlotArgs } from '../rich-content/RichContentEditor';
import { emptyRichContent } from '../rich-content/types';
import { itemId, type ItemPrefix } from './itemId';
import { LIMITS, type OptionItem, type QuestionDraft, type QuestionType, type TextMatch } from './mappers';
import { OptionFeedbackFields } from './OptionFeedbackFields';
import { SearchSelect } from '../select/SearchSelect';

export interface TypeEditorProps {
  draft: QuestionDraft;
  /** Merges into render_payload / answer_key (and stem for blank insertion). */
  set: (patch: Partial<Pick<QuestionDraft, 'render_payload' | 'answer_key' | 'stem'>>) => void;
  lang: ContentLang;
  readOnly: boolean;
  errors: Readonly<Record<string, string>>;
  mediaSlot?: (args: MediaSlotArgs) => ReactNode;
}

const IMAGE = 'image';
const FIELD = { backdrop: 'render_payload.background_media_ref', image: 'render_payload.image_media_ref' } as const;

const arr = <T,>(v: unknown): T[] => (Array.isArray(v) ? (v as T[]) : []);
const obj = <T,>(v: unknown): Record<string, T> => (v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, T>) : {});

function FieldError({ errors, field }: { errors: Readonly<Record<string, string>>; field: string }) {
  return errors[field] ? (
    <Typography.Text type="danger" role="alert" data-error-for={field}>
      {errors[field]}
    </Typography.Text>
  ) : null;
}

/** List of rich-content options with add / remove / move; `extra(item, i)` renders per-option controls. */
function OptionList({
  field,
  prefix,
  items,
  min,
  max,
  onChange,
  extra,
  p,
  labelKey = 'question.option',
}: {
  field: string;
  prefix: ItemPrefix;
  items: OptionItem[];
  min: number;
  max: number;
  onChange: (items: OptionItem[]) => void;
  extra?: (item: OptionItem, i: number) => ReactNode;
  p: TypeEditorProps;
  labelKey?: string;
}) {
  const { t } = useTranslation(OPS_UI_NS);
  const move = (i: number, d: -1 | 1) => {
    const j = i + d;
    if (j < 0 || j >= items.length) return;
    const next = [...items];
    [next[i], next[j]] = [next[j]!, next[i]!];
    onChange(next);
  };
  return (
    <Flex vertical gap="small" data-field={field}>
      {items.map((o, i) => (
        <Card
          key={o.id}
          size="small"
          title={
            <Space>
              {t(labelKey, { n: i + 1 })}
              <Typography.Text type="secondary" code>
                {o.id}
              </Typography.Text>
            </Space>
          }
          extra={
            !p.readOnly && (
              <Space size="small">
                <Button size="small" disabled={i === 0} onClick={() => move(i, -1)} aria-label={t('richContent.moveUp')}>
                  {t('richContent.up')}
                </Button>
                <Button size="small" disabled={i === items.length - 1} onClick={() => move(i, 1)} aria-label={t('richContent.moveDown')}>
                  {t('richContent.down')}
                </Button>
                <Button size="small" danger disabled={items.length <= min} onClick={() => onChange(items.filter((_, j) => j !== i))}>
                  {t('richContent.remove')}
                </Button>
              </Space>
            )
          }
        >
          <Flex vertical gap="small">
            <RichContentEditor
              id={`${field}.${i}.content`}
              label={t(labelKey, { n: i + 1 })}
              value={o.content}
              onChange={(content) => onChange(items.map((x, j) => (j === i ? { ...x, content } : x)))}
              contentLanguage={p.lang}
              allowedBlocks={['paragraph', 'math', 'image']}
              readOnly={p.readOnly}
              errors={p.errors}
              mediaSlot={p.mediaSlot}
              required
            />
            {extra?.(o, i)}
          </Flex>
        </Card>
      ))}
      {!p.readOnly && (
        <Button disabled={items.length >= max} onClick={() => onChange([...items, { id: itemId(prefix, items.map((x) => x.id)), content: emptyRichContent() }])}>
          {t('question.addOption', { max })}
        </Button>
      )}
      <FieldError errors={p.errors} field={field} />
    </Flex>
  );
}

function ChoiceEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const single = p.draft.type === 'single_choice';
  const rp = p.draft.render_payload;
  const ak = p.draft.answer_key;
  const opts = arr<OptionItem>(rp.options);
  const correct = new Set<string>(single ? (typeof ak.correct_option_id === 'string' ? [ak.correct_option_id] : []) : arr<string>(ak.correct_option_ids));
  const [min, max] = LIMITS.options[single ? 'single_choice' : 'multiple_choice'];
  const setOptions = (next: OptionItem[]) => {
    const ids = new Set(next.map((o) => o.id));
    // removing the option that was the answer clears it: saving is blocked until a new answer is picked (F053-AC-02)
    const answer = single ? (correct.size && ids.has([...correct][0]!) ? { correct_option_id: [...correct][0] } : {}) : { correct_option_ids: [...correct].filter((c) => ids.has(c)) };
    p.set({ render_payload: { ...rp, options: next }, answer_key: answer });
  };
  const toggle = (id: string, on: boolean) => {
    if (single) p.set({ answer_key: on ? { correct_option_id: id } : {} });
    else p.set({ answer_key: { correct_option_ids: on ? [...correct, id] : [...correct].filter((c) => c !== id) } });
  };
  return (
    <Flex vertical gap="middle">
      <Space wrap>
        <Checkbox checked={rp.shuffle_options !== false} disabled={p.readOnly} onChange={(e) => p.set({ render_payload: { ...rp, shuffle_options: e.target.checked } })}>
          {t('question.shuffleOptions')}
        </Checkbox>
        {!single && (
          <Checkbox checked={rp.show_correct_count === true} disabled={p.readOnly} onChange={(e) => p.set({ render_payload: { ...rp, show_correct_count: e.target.checked } })}>
            {t('question.showCorrectCount')}
          </Checkbox>
        )}
      </Space>
      <OptionList
        field="render_payload.options"
        prefix="o"
        items={opts}
        min={min}
        max={max}
        onChange={setOptions}
        p={p}
        extra={(o, i) => (
          <Flex vertical gap="small">
            {single ? (
              <Radio checked={correct.has(o.id)} disabled={p.readOnly} onChange={(e) => toggle(o.id, e.target.checked)}>
                {t('question.correctAnswer')}
              </Radio>
            ) : (
              <Checkbox checked={correct.has(o.id)} disabled={p.readOnly} onChange={(e) => toggle(o.id, e.target.checked)}>
                {t('question.correctAnswer')}
              </Checkbox>
            )}
            {!correct.has(o.id) && (
              <OptionFeedbackFields
                id={`render_payload.options.${i}`}
                option={o}
                onChange={(next) => p.set({ render_payload: { ...rp, options: opts.map((x) => (x.id === o.id ? next : x)) } })}
                contentLanguage={p.lang}
                readOnly={p.readOnly}
                errors={p.errors}
              />
            )}
          </Flex>
        )}
      />
      <FieldError errors={p.errors} field={single ? 'answer_key.correct_option_id' : 'answer_key.correct_option_ids'} />
    </Flex>
  );
}

function TrueFalseEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const rp = p.draft.render_payload;
  const ak = p.draft.answer_key;
  const statements = arr<{ id: string; content: OptionItem['content'] }>(rp.statements);
  const multi = statements.length > 0;
  const values = obj<boolean>(ak.statement_values);
  const tf = (value: boolean | undefined, onChange: (v: boolean) => void, name: string) => (
    <Radio.Group value={value} disabled={p.readOnly} onChange={(e) => onChange(e.target.value as boolean)} aria-label={name}>
      <Radio value>{t('question.true')}</Radio>
      <Radio value={false}>{t('question.false')}</Radio>
    </Radio.Group>
  );
  return (
    <Flex vertical gap="middle">
      <Switch
        checkedChildren={t('question.tfMulti')}
        unCheckedChildren={t('question.tfSingle')}
        checked={multi}
        disabled={p.readOnly}
        onChange={(on) =>
          on
            ? p.set({ render_payload: { statements: [{ id: itemId('s'), content: emptyRichContent() }] }, answer_key: { statement_values: {} } })
            : p.set({ render_payload: {}, answer_key: {} })
        }
      />
      {multi ? (
        <OptionList
          field="render_payload.statements"
          prefix="s"
          labelKey="question.statement"
          items={statements}
          min={LIMITS.options.statements[0]}
          max={LIMITS.options.statements[1]}
          p={p}
          onChange={(next) => p.set({ render_payload: { ...rp, statements: next.map(({ id, content }) => ({ id, content })) }, answer_key: { statement_values: Object.fromEntries(Object.entries(values).filter(([k]) => next.some((s) => s.id === k))) } })}
          extra={(s) => tf(values[s.id], (v) => p.set({ answer_key: { statement_values: { ...values, [s.id]: v } } }), t('question.correctAnswer'))}
        />
      ) : (
        <Form.Item label={t('question.correctAnswer')}>{tf(typeof ak.value === 'boolean' ? ak.value : undefined, (v) => p.set({ answer_key: { value: v } }), t('question.correctAnswer'))}</Form.Item>
      )}
      <FieldError errors={p.errors} field="answer_key.value" />
    </Flex>
  );
}

function AcceptedAnswers({ value, onChange, p, field }: { value: Partial<TextMatch>; onChange: (v: TextMatch) => void; p: TypeEditorProps; field: string }) {
  const { t } = useTranslation(OPS_UI_NS);
  const accepted = value.accepted ?? [];
  return (
    <Flex vertical gap="small" data-field={field}>
      <SearchSelect
        mode="tags"
        aria-label={t('question.accepted')}
        placeholder={t('question.accepted')}
        value={accepted}
        disabled={p.readOnly}
        open={false}
        tokenSeparators={[';']}
        onChange={(v: string[]) => onChange({ ...value, accepted: v.map((x) => x.slice(0, LIMITS.acceptedAnswer)).filter((x) => x.trim()) })}
      />
      <Space wrap>
        <SearchSelect
          aria-label={t('question.match')}
          value={value.match ?? 'normalized'}
          disabled={p.readOnly}
          options={[
            { value: 'normalized', label: t('question.matchNormalized') },
            { value: 'exact', label: t('question.matchExact') },
          ]}
          onChange={(m) => onChange({ accepted, ...value, match: m })}
        />
        <Checkbox checked={value.case_sensitive === true} disabled={p.readOnly} onChange={(e) => onChange({ accepted, ...value, case_sensitive: e.target.checked })}>
          {t('question.caseSensitive')}
        </Checkbox>
        <Checkbox checked={value.allow_diacritics_missing === true} disabled={p.readOnly} onChange={(e) => onChange({ accepted, ...value, allow_diacritics_missing: e.target.checked })}>
          {t('question.allowDiacriticsMissing')}
        </Checkbox>
      </Space>
      <FieldError errors={p.errors} field={field} />
    </Flex>
  );
}

function FillBlankEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const rp = p.draft.render_payload;
  const blanks = arr<{ id: string; input: string; max_length?: number; choices?: string[] }>(rp.blanks);
  const keys = obj<Partial<TextMatch>>(p.draft.answer_key.blanks);
  const setBlanks = (next: typeof blanks, nextKeys = keys) =>
    p.set({ render_payload: { ...rp, blanks: next }, answer_key: { blanks: Object.fromEntries(next.map((b) => [b.id, nextKeys[b.id] ?? { accepted: [], match: 'normalized' }])) } });
  const insertInStem = (id: string) => {
    const blocks = [...p.draft.stem.blocks];
    const i = blocks.findIndex((b) => b.type === 'paragraph');
    const marker = `{{blank:${id}}}`;
    if (i < 0) blocks.push({ type: 'paragraph', text: marker });
    else {
      const b = blocks[i]!;
      const text = typeof b.text === 'string' ? `${b.text} ${marker}` : b.text ? { ...b.text, [p.lang]: `${b.text[p.lang] ?? ''} ${marker}`.trim() } : marker;
      blocks[i] = { ...b, text };
    }
    p.set({ stem: { ...p.draft.stem, blocks } });
  };
  return (
    <Flex vertical gap="middle">
      <Typography.Text type="secondary">{t('question.blankHelp', { marker: '{{blank:…}}' })}</Typography.Text>
      {blanks.map((b, i) => (
        <Card key={b.id} size="small" title={<Space>{t('question.blank', { n: i + 1 })}<Typography.Text code>{`{{blank:${b.id}}}`}</Typography.Text></Space>} extra={!p.readOnly && (
          <Space>
            <Button size="small" onClick={() => insertInStem(b.id)}>{t('question.insertBlank')}</Button>
            <Button size="small" danger disabled={blanks.length <= 1} onClick={() => setBlanks(blanks.filter((x) => x.id !== b.id))}>{t('richContent.remove')}</Button>
          </Space>
        )}>
          <Flex vertical gap="small">
            <Space wrap>
              <SearchSelect
                aria-label={t('question.blankInput')}
                value={b.input}
                disabled={p.readOnly}
                options={['text', 'numeric', 'word_bank', 'dropdown'].map((v) => ({ value: v, label: t(`question.blankInputs.${v}`) }))}
                onChange={(input) => setBlanks(blanks.map((x) => (x.id === b.id ? { ...x, input } : x)))}
              />
              <InputNumber aria-label={t('question.maxLength')} min={1} max={200} value={b.max_length} disabled={p.readOnly} onChange={(v) => setBlanks(blanks.map((x) => (x.id === b.id ? { ...x, max_length: v ?? undefined } : x)))} />
            </Space>
            {b.input === 'dropdown' && (
              <SearchSelect mode="tags" open={false} aria-label={t('question.choices')} placeholder={t('question.choices')} value={b.choices ?? []} disabled={p.readOnly} onChange={(v: string[]) => setBlanks(blanks.map((x) => (x.id === b.id ? { ...x, choices: v.map((c) => c.slice(0, LIMITS.label)) } : x)))} />
            )}
            <AcceptedAnswers field={`answer_key.blanks.${b.id}`} value={keys[b.id] ?? {}} p={p} onChange={(v) => setBlanks(blanks, { ...keys, [b.id]: v })} />
          </Flex>
        </Card>
      ))}
      {!p.readOnly && (
        <Button disabled={blanks.length >= LIMITS.options.blanks[1]} onClick={() => setBlanks([...blanks, { id: itemId('b', blanks.map((x) => x.id)), input: 'text', max_length: 30 }])}>
          {t('question.addBlank')}
        </Button>
      )}
      <Form.Item label={t('question.wordBank')}>
        <SearchSelect mode="tags" open={false} aria-label={t('question.wordBank')} value={arr<string>(rp.word_bank)} disabled={p.readOnly} onChange={(v: string[]) => p.set({ render_payload: { ...rp, word_bank: v.map((x) => x.slice(0, LIMITS.wordBank)) } })} />
      </Form.Item>
      <FieldError errors={p.errors} field="stem" />
    </Flex>
  );
}

function ShortAnswerEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const rp = p.draft.render_payload;
  return (
    <Flex vertical gap="middle">
      <Space wrap>
        <Form.Item label={t('question.maxLength')}>
          <InputNumber min={1} max={500} value={rp.max_length as number | undefined} disabled={p.readOnly} onChange={(v) => p.set({ render_payload: { ...rp, max_length: v ?? 100 } })} />
        </Form.Item>
        <Form.Item label={t('question.keyboard')}>
          <SearchSelect value={(rp.keyboard as string) ?? 'text'} disabled={p.readOnly} options={['text', 'numeric'].map((v) => ({ value: v, label: t(`question.keyboards.${v}`) }))} onChange={(keyboard) => p.set({ render_payload: { ...rp, keyboard } })} />
        </Form.Item>
      </Space>
      <AcceptedAnswers field="answer_key.accepted" value={p.draft.answer_key as Partial<TextMatch>} p={p} onChange={(v) => p.set({ answer_key: { ...v } })} />
    </Flex>
  );
}

function NumericEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const rp = p.draft.render_payload;
  const ak = p.draft.answer_key;
  const tol = obj<unknown>(ak.tolerance) as { type?: string; value?: number };
  return (
    <Flex vertical gap="middle">
      <Space wrap align="end">
        <Form.Item label={t('question.numericValue')} validateStatus={p.errors['answer_key.value'] ? 'error' : undefined} help={p.errors['answer_key.value']}>
          <InputNumber aria-label={t('question.numericValue')} value={typeof ak.value === 'number' ? ak.value : null} disabled={p.readOnly} onChange={(v) => p.set({ answer_key: { ...ak, value: v ?? undefined } })} />
        </Form.Item>
        <Form.Item label={t('question.tolerance')}>
          <SearchSelect value={tol.type ?? 'exact'} disabled={p.readOnly} options={['exact', 'absolute', 'relative'].map((v) => ({ value: v, label: t(`question.tolerances.${v}`) }))} onChange={(type) => p.set({ answer_key: { ...ak, tolerance: type === 'exact' ? { type } : { type, value: tol.value ?? 0 } } })} />
        </Form.Item>
        {tol.type && tol.type !== 'exact' && (
          <Form.Item label={t('question.toleranceValue')}>
            <InputNumber min={0} value={tol.value} disabled={p.readOnly} onChange={(v) => p.set({ answer_key: { ...ak, tolerance: { ...tol, value: v ?? 0 } } })} />
          </Form.Item>
        )}
        <Form.Item label={t('question.unitLabel')}>
          <Input value={(rp.unit_label as string) ?? ''} maxLength={LIMITS.unitLabel} disabled={p.readOnly} onChange={(e) => p.set({ render_payload: { ...rp, unit_label: e.target.value || undefined } })} />
        </Form.Item>
      </Space>
      <Space wrap>
        <Checkbox checked={rp.allow_fraction_input === true} disabled={p.readOnly} onChange={(e) => p.set({ render_payload: { ...rp, allow_fraction_input: e.target.checked } })}>
          {t('question.allowFraction')}
        </Checkbox>
        <Checkbox checked={ak.accept_equivalent_forms !== false} disabled={p.readOnly} onChange={(e) => p.set({ answer_key: { ...ak, accept_equivalent_forms: e.target.checked } })}>
          {t('question.acceptEquivalent')}
        </Checkbox>
        <SearchSelect aria-label={t('question.decimalSeparator')} value={(rp.decimal_separator as string) ?? ','} disabled={p.readOnly} options={[{ value: ',', label: ',' }, { value: '.', label: '.' }]} onChange={(decimal_separator) => p.set({ render_payload: { ...rp, decimal_separator } })} />
      </Space>
    </Flex>
  );
}

function MatchingEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const rp = p.draft.render_payload;
  const left = arr<OptionItem>(rp.left);
  const right = arr<OptionItem>(rp.right);
  const pairs = arr<[string, string]>(p.draft.answer_key.pairs);
  const pairOf = (l: string) => pairs.find((x) => x[0] === l)?.[1];
  const setPairs = (next: [string, string][]) => p.set({ answer_key: { pairs: next } });
  const prune = (l: OptionItem[], r: OptionItem[]) => pairs.filter(([a, b]) => l.some((x) => x.id === a) && r.some((x) => x.id === b));
  return (
    <Flex vertical gap="middle">
      <Typography.Text strong>{t('question.leftColumn')}</Typography.Text>
      <OptionList
        field="render_payload.left"
        prefix="l"
        items={left}
        min={LIMITS.options.matchingLeft[0]}
        max={LIMITS.options.matchingLeft[1]}
        p={p}
        onChange={(next) => p.set({ render_payload: { ...rp, left: next }, answer_key: { pairs: prune(next, right) } })}
        extra={(o) => (
          <SearchSelect
            aria-label={t('question.pairWith')}
            placeholder={t('question.pairWith')}
            allowClear
            disabled={p.readOnly}
            value={pairOf(o.id)}
            options={right.map((r, i) => ({ value: r.id, label: t('question.rightItem', { n: i + 1, id: r.id }) }))}
            onChange={(v?: string) => setPairs([...pairs.filter((x) => x[0] !== o.id), ...(v ? [[o.id, v] as [string, string]] : [])])}
          />
        )}
      />
      <Typography.Text strong>{t('question.rightColumn')}</Typography.Text>
      <OptionList field="render_payload.right" prefix="r" items={right} min={LIMITS.options.matchingRight[0]} max={LIMITS.options.matchingRight[1]} p={p} onChange={(next) => p.set({ render_payload: { ...rp, right: next }, answer_key: { pairs: prune(left, next) } })} />
      <FieldError errors={p.errors} field="answer_key.pairs" />
    </Flex>
  );
}

function OrderingEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const rp = p.draft.render_payload;
  const items = arr<OptionItem>(rp.items);
  return (
    <Flex vertical gap="middle">
      <Typography.Text type="secondary">{t('question.orderingHelp')}</Typography.Text>
      <OptionList
        field="render_payload.items"
        prefix="i"
        labelKey="question.orderItem"
        items={items}
        min={LIMITS.options.ordering[0]}
        max={LIMITS.options.ordering[1]}
        p={p}
        onChange={(next) => p.set({ render_payload: { ...rp, items: next, shuffle: true }, answer_key: { order: next.map((x) => x.id) } })}
      />
    </Flex>
  );
}

function DragDropEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const rp = p.draft.render_payload;
  const drags = arr<OptionItem>(rp.draggables);
  const zones = arr<{ id: string; label: I18nTextValue; capacity?: number | null }>(rp.zones);
  const placements = obj<string>(p.draft.answer_key.placements);
  const setZones = (next: typeof zones) =>
    p.set({ render_payload: { ...rp, zones: next }, answer_key: { placements: Object.fromEntries(Object.entries(placements).filter(([, z]) => next.some((x) => x.id === z))) } });
  return (
    <Flex vertical gap="middle">
      <Typography.Text strong>{t('question.zones')}</Typography.Text>
      {zones.map((z, i) => (
        <Flex key={z.id} gap="small" align="start">
          <div style={{ flex: 1 }}>
            <BilingualField id={`render_payload.zones.${i}.label`} label={t('question.zone', { n: i + 1, id: z.id })} value={z.label || undefined} onChange={(label) => setZones(zones.map((x) => (x.id === z.id ? { ...x, label: label ?? '' } : x)))} contentLanguage={p.lang} maxLength={1000} required readOnly={p.readOnly} error={p.errors[`render_payload.zones.${i}.label`]} />
          </div>
          <InputNumber aria-label={t('question.capacity')} placeholder={t('question.capacity')} min={1} value={z.capacity ?? null} disabled={p.readOnly} onChange={(v) => setZones(zones.map((x) => (x.id === z.id ? { ...x, capacity: v ?? null } : x)))} />
          {!p.readOnly && (
            <Button size="small" danger disabled={zones.length <= LIMITS.options.zones[0]} onClick={() => setZones(zones.filter((x) => x.id !== z.id))}>
              {t('richContent.remove')}
            </Button>
          )}
        </Flex>
      ))}
      {!p.readOnly && (
        <Button disabled={zones.length >= LIMITS.options.zones[1]} onClick={() => setZones([...zones, { id: itemId('z', zones.map((x) => x.id)), label: '', capacity: null }])}>
          {t('question.addZone')}
        </Button>
      )}
      <Typography.Text strong>{t('question.draggables')}</Typography.Text>
      <OptionList
        field="render_payload.draggables"
        prefix="d"
        labelKey="question.draggable"
        items={drags}
        min={LIMITS.options.draggables[0]}
        max={LIMITS.options.draggables[1]}
        p={p}
        onChange={(next) => p.set({ render_payload: { ...rp, draggables: next }, answer_key: { placements: Object.fromEntries(Object.entries(placements).filter(([d]) => next.some((x) => x.id === d))) } })}
        extra={(d) => (
          <SearchSelect
            aria-label={t('question.placeIn')}
            placeholder={t('question.distractor')}
            allowClear
            disabled={p.readOnly}
            value={placements[d.id]}
            options={zones.map((z, i) => ({ value: z.id, label: t('question.zoneShort', { n: i + 1, id: z.id }) }))}
            onChange={(v?: string) => {
              const next = { ...placements };
              if (v) next[d.id] = v;
              else delete next[d.id];
              p.set({ answer_key: { placements: next } });
            }}
          />
        )}
      />
      <Checkbox checked={rp.reusable_draggables === true} disabled={p.readOnly} onChange={(e) => p.set({ render_payload: { ...rp, reusable_draggables: e.target.checked } })}>
        {t('question.reusableDraggables')}
      </Checkbox>
      {p.mediaSlot?.({ kind: IMAGE, value: (rp.background_media_ref as string | null) ?? undefined, onChange: (v) => p.set({ render_payload: { ...rp, background_media_ref: v ?? null } }), id: FIELD.backdrop })}
      <FieldError errors={p.errors} field="answer_key.placements" />
    </Flex>
  );
}

interface Region {
  id: string;
  shape: 'circle' | 'rect' | 'polygon';
  cx?: number;
  cy?: number;
  r?: number;
  x?: number;
  y?: number;
  w?: number;
  h?: number;
  points?: [number, number][];
}

function ImageBasedEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const rp = p.draft.render_payload;
  const ak = p.draft.answer_key;
  const size = obj<number>(rp.image_size);
  const hotspot = rp.interaction !== 'label';
  const regions = arr<Region>(ak.regions);
  const correct = new Set(arr<string>(ak.correct_region_ids));
  const num = (label: string, value: number | undefined, onChange: (v: number) => void) => (
    <InputNumber aria-label={label} placeholder={label} value={value} disabled={p.readOnly} onChange={(v) => onChange(v ?? 0)} />
  );
  const setRegion = (id: string, patch: Partial<Region>) => p.set({ answer_key: { ...ak, regions: regions.map((r) => (r.id === id ? { ...r, ...patch } : r)) } });
  const labels = arr<OptionItem>(rp.labels);
  const points = arr<{ id: string; x: number; y: number }>(rp.drop_points);
  const placements = obj<string>(ak.placements);
  return (
    <Flex vertical gap="middle">
      {p.mediaSlot?.({ kind: IMAGE, value: rp.image_media_ref as string | undefined, onChange: (v) => p.set({ render_payload: { ...rp, image_media_ref: v } }), id: FIELD.image })}
      <FieldError errors={p.errors} field="render_payload.image_media_ref" />
      <Space wrap>
        {num(t('question.imageWidth'), size.w, (w) => p.set({ render_payload: { ...rp, image_size: { ...size, w } } }))}
        {num(t('question.imageHeight'), size.h, (h) => p.set({ render_payload: { ...rp, image_size: { ...size, h } } }))}
        <SearchSelect
          aria-label={t('question.interaction')}
          value={hotspot ? 'hotspot' : 'label'}
          disabled={p.readOnly}
          options={[
            { value: 'hotspot', label: t('question.interactions.hotspot') },
            { value: 'label', label: t('question.interactions.label') },
          ]}
          onChange={(v) =>
            v === 'hotspot'
              ? p.set({ render_payload: { ...rp, interaction: 'hotspot', labels: undefined, drop_points: undefined, max_selections: 1 }, answer_key: { regions: [], correct_region_ids: [] } })
              : p.set({ render_payload: { ...rp, interaction: 'label', labels: [], drop_points: [], max_selections: undefined }, answer_key: { placements: {} } })
          }
        />
      </Space>
      {hotspot ? (
        <Flex vertical gap="small" data-field="answer_key.regions">
          <Typography.Text type="secondary">{t('question.regionsHelp')}</Typography.Text>
          {regions.map((r) => (
            <Space key={r.id} wrap>
              <Typography.Text code>{r.id}</Typography.Text>
              <SearchSelect value={r.shape} disabled={p.readOnly} aria-label={t('question.shape')} options={['circle', 'rect'].map((v) => ({ value: v, label: t(`question.shapes.${v}`) }))} onChange={(shape) => setRegion(r.id, { shape })} />
              {r.shape === 'circle' ? (
                <>
                  {num(t('question.coords.cx'), r.cx, (cx) => setRegion(r.id, { cx }))}
                  {num(t('question.coords.cy'), r.cy, (cy) => setRegion(r.id, { cy }))}
                  {num(t('question.coords.r'), r.r, (rr) => setRegion(r.id, { r: rr }))}
                </>
              ) : (
                <>
                  {num(t('question.coords.x'), r.x, (x) => setRegion(r.id, { x }))}
                  {num(t('question.coords.y'), r.y, (y) => setRegion(r.id, { y }))}
                  {num(t('question.coords.w'), r.w, (w) => setRegion(r.id, { w }))}
                  {num(t('question.coords.h'), r.h, (h) => setRegion(r.id, { h }))}
                </>
              )}
              <Checkbox checked={correct.has(r.id)} disabled={p.readOnly} onChange={(e) => p.set({ answer_key: { ...ak, correct_region_ids: e.target.checked ? [...correct, r.id] : [...correct].filter((c) => c !== r.id) } })}>
                {t('question.correctAnswer')}
              </Checkbox>
              {!p.readOnly && (
                <Button size="small" danger onClick={() => p.set({ answer_key: { ...ak, regions: regions.filter((x) => x.id !== r.id), correct_region_ids: [...correct].filter((c) => c !== r.id) } })}>
                  {t('richContent.remove')}
                </Button>
              )}
            </Space>
          ))}
          {!p.readOnly && <Button onClick={() => p.set({ answer_key: { ...ak, regions: [...regions, { id: itemId('h', regions.map((x) => x.id)), shape: 'circle', cx: 0, cy: 0, r: 10 }] } })}>{t('question.addRegion')}</Button>}
          <FieldError errors={p.errors} field="answer_key.correct_region_ids" />
        </Flex>
      ) : (
        <Flex vertical gap="small">
          {points.map((pt) => (
            <Space key={pt.id} wrap>
              <Typography.Text code>{pt.id}</Typography.Text>
              {num(t('question.coords.x'), pt.x, (x) => p.set({ render_payload: { ...rp, drop_points: points.map((q) => (q.id === pt.id ? { ...q, x } : q)) } }))}
              {num(t('question.coords.y'), pt.y, (y) => p.set({ render_payload: { ...rp, drop_points: points.map((q) => (q.id === pt.id ? { ...q, y } : q)) } }))}
            </Space>
          ))}
          {!p.readOnly && <Button onClick={() => p.set({ render_payload: { ...rp, drop_points: [...points, { id: itemId('x', points.map((x) => x.id)), x: 0, y: 0 }] } })}>{t('question.addDropPoint')}</Button>}
          <OptionList
            field="render_payload.labels"
            prefix="p"
            labelKey="question.label"
            items={labels}
            min={0}
            max={LIMITS.options.draggables[1]}
            p={p}
            onChange={(next) => p.set({ render_payload: { ...rp, labels: next } })}
            extra={(l) => (
              <SearchSelect
                aria-label={t('question.placeIn')}
                allowClear
                value={placements[l.id]}
                disabled={p.readOnly}
                options={points.map((pt) => ({ value: pt.id, label: pt.id }))}
                onChange={(v?: string) => {
                  const next = { ...placements };
                  if (v) next[l.id] = v;
                  else delete next[l.id];
                  p.set({ answer_key: { placements: next } });
                }}
              />
            )}
          />
          <FieldError errors={p.errors} field="answer_key.placements" />
        </Flex>
      )}
    </Flex>
  );
}

function JsonField({ label, value, onChange, readOnly, field, errors }: { label: string; value: unknown; onChange: (v: Record<string, unknown>) => void; readOnly: boolean; field: string; errors: Readonly<Record<string, string>> }) {
  const { t } = useTranslation(OPS_UI_NS);
  let text = '';
  try {
    text = JSON.stringify(value ?? {}, null, 2);
  } catch {
    text = '{}';
  }
  return (
    <Form.Item label={label} validateStatus={errors[field] ? 'error' : undefined} help={errors[field] ?? t('question.jsonHelp')}>
      <Input.TextArea
        aria-label={label}
        defaultValue={text}
        readOnly={readOnly}
        rows={6}
        onBlur={(e) => {
          try {
            const v = JSON.parse(e.target.value || '{}') as unknown;
            if (v && typeof v === 'object' && !Array.isArray(v)) onChange(v as Record<string, unknown>);
          } catch {
            // invalid JSON stays in the box; the value is unchanged
          }
        }}
      />
    </Form.Item>
  );
}

function InteractiveEditor(p: TypeEditorProps) {
  const { t } = useTranslation(OPS_UI_NS);
  const rp = p.draft.render_payload;
  const ak = p.draft.answer_key;
  return (
    <Flex vertical gap="middle">
      <Space wrap>
        <Form.Item label={t('question.widget')}>
          <Input value={(rp.widget as string) ?? ''} maxLength={64} disabled={p.readOnly} onChange={(e) => p.set({ render_payload: { ...rp, widget: e.target.value } })} />
        </Form.Item>
        <Form.Item label={t('question.widgetVersion')}>
          <Input value={(rp.widget_version as string) ?? ''} maxLength={64} disabled={p.readOnly} onChange={(e) => p.set({ render_payload: { ...rp, widget_version: e.target.value } })} />
        </Form.Item>
        <Form.Item label={t('question.validator')} validateStatus={p.errors['answer_key.validator'] ? 'error' : undefined} help={p.errors['answer_key.validator']}>
          <Input value={(ak.validator as string) ?? ''} maxLength={64} disabled={p.readOnly} onChange={(e) => p.set({ answer_key: { ...ak, validator: e.target.value } })} />
        </Form.Item>
      </Space>
      <JsonField label={t('question.widgetConfig')} field="render_payload.config" value={rp.config} readOnly={p.readOnly} errors={p.errors} onChange={(config) => p.set({ render_payload: { ...rp, config } })} />
      <JsonField label={t('question.validatorParams')} field="answer_key.params" value={ak.params} readOnly={p.readOnly} errors={p.errors} onChange={(params) => p.set({ answer_key: { ...ak, params } })} />
    </Flex>
  );
}

/** Editor component per QuestionType (all 11 — F053-AC-01). */
export const TYPE_EDITORS: Record<QuestionType, (p: TypeEditorProps) => ReactNode> = {
  single_choice: ChoiceEditor,
  multiple_choice: ChoiceEditor,
  true_false: TrueFalseEditor,
  fill_in_blank: FillBlankEditor,
  short_answer: ShortAnswerEditor,
  numeric: NumericEditor,
  matching: MatchingEditor,
  ordering: OrderingEditor,
  drag_drop: DragDropEditor,
  image_based: ImageBasedEditor,
  interactive: InteractiveEditor,
};

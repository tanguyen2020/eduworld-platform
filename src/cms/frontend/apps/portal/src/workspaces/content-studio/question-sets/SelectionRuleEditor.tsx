// SelectionRuleEditor (F054-AC-20, question-set.schema.json, QUESTION_BANK §6): static items (published question
// versions picked from the question bank — immutable `question_version_id`) and/or selection rules (subject, grades,
// topics, LOs, difficulty range, types, count) with the current pool size of each rule from the server
// (getQuestionSetPoolStats of the saved version: available / required, insufficient, warning).
import { DeleteOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Flex, Input, InputNumber, List, Modal, Segmented, Space, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { PoolStats } from '../builders/useDocument';
import { useObjectives, useTopics } from '../curriculum/useCurriculum';
import type { QuestionAuthoring } from '../questions/useQuestion';
import { LIMITS } from '../shared/config';
import { SET_LIMITS, type QuestionSetState, type SelectionRule, type SetKind } from './questionSetModel';
import { SearchSelect } from '@eduworld/ops-ui';

const TYPES = ['single_choice', 'multiple_choice', 'true_false', 'fill_in_blank', 'short_answer', 'numeric', 'matching', 'ordering', 'drag_drop', 'image_based', 'interactive'];

function useQuestionSearch(q: string, enabled: boolean) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  // A list of rows: cursor pages, "Tải thêm" (owner rule 2026-11-22).
  return useCursorPages<QuestionAuthoring>(
    ['content-studio', 'list', scopeKey, 'question-picker', q],
    (cursor) => api.call(api.client.GET('/cms/learning/questions', { params: { query: { status: 'published', ...(q ? { q } : {}), ...(cursor ? { cursor } : {}) } } })),
    { enabled },
  );
}

export function QuestionPicker({ open, onPick, onClose }: { open: boolean; onPick: (q: QuestionAuthoring) => void; onClose: () => void }) {
  const { t } = useTranslation();
  const [q, setQ] = useState('');
  const [search, setSearch] = useState('');
  const list = useQuestionSearch(search, open);
  return (
    <Modal open={open} title={t('content_studio.questionSet.pickTitle')} footer={null} onCancel={onClose} destroyOnHidden>
      <Input.Search aria-label={t('content_studio.list.search')} placeholder={t('content_studio.list.search')} maxLength={LIMITS.search} value={q} onChange={(e) => setQ(e.target.value)} onSearch={(v) => setSearch(v.trim())} />
      <List
        loading={list.isFetching}
        dataSource={list.rows}
        loadMore={
          list.hasNextPage ? (
            <Button size="small" onClick={() => void list.fetchNextPage()} loading={list.isFetchingNextPage}>
              {t('content_studio.loadMore')}
            </Button>
          ) : null
        }
        renderItem={(x) => (
          <List.Item
            key={x.content_version_id ?? x.content_id}
            actions={[
              <Button key="add" size="small" disabled={!x.content_version_id} onClick={() => onPick(x)}>
                {t('content_studio.questionSet.add')}
              </Button>,
            ]}
          >
            {t('content_studio.questionSet.questionLabel', { id: x.content_id.slice(-8), version: x.version, type: x.type })}
          </List.Item>
        )}
      />
    </Modal>
  );
}

function RuleCard({ rule, index, state, onChange, onRemove, readOnly, pool }: { rule: SelectionRule; index: number; state: QuestionSetState; onChange: (r: SelectionRule) => void; onRemove: () => void; readOnly: boolean; pool?: PoolStats['rules'][number] }) {
  const { t } = useTranslation();
  const subject = rule.filters.subject_id ?? state.subject_id;
  const topics = useTopics({ subject_id: subject }, !!subject);
  const objectives = useObjectives({ subject_id: subject }, !!subject);
  const f = rule.filters;
  const set = (patch: Partial<SelectionRule['filters']>) => onChange({ ...rule, filters: { ...f, ...patch } });
  return (
    <Card
      size="small"
      data-testid={`rule-${index}`}
      title={t('content_studio.questionSet.rule', { n: index + 1 })}
      extra={
        <Space>
          {pool ? (
            <Tag color={pool.insufficient ? 'error' : pool.warning ? 'warning' : 'success'} data-testid={`pool-${index}`}>
              {t('content_studio.questionSet.pool', { available: pool.available, required: pool.required })}
            </Tag>
          ) : (
            <Tag>{t('content_studio.questionSet.poolUnknown')}</Tag>
          )}
          {!readOnly && <Button size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.questionSet.removeRule', { n: index + 1 })} onClick={onRemove} />}
        </Space>
      }
    >
      <Flex gap="small" wrap>
        <SearchSelect mode="multiple" aria-label={t('content_studio.question.topic')} placeholder={t('content_studio.question.topic')} disabled={readOnly} value={f.topic_ids ?? []} options={(topics.data?.data ?? []).map((x) => ({ value: x.topic_id, label: `${x.code ?? ''} ${x.name}`.trim() }))} onChange={(v: string[]) => set({ topic_ids: v })} style={{ minWidth: 200 }} />
        <SearchSelect mode="multiple" aria-label={t('content_studio.question.objectives')} placeholder={t('content_studio.question.objectives')} disabled={readOnly} value={f.objective_ids ?? []} options={(objectives.data?.data ?? []).map((o) => ({ value: o.objective_id, label: o.code }))} onChange={(v: string[]) => set({ objective_ids: v })} style={{ minWidth: 200 }} />
        <SearchSelect mode="multiple" aria-label={t('content_studio.questionSet.types')} placeholder={t('content_studio.questionSet.types')} disabled={readOnly} value={f.types ?? []} options={TYPES.map((x) => ({ value: x, label: t(`ops-ui:question.types.${x}`) }))} onChange={(v: string[]) => set({ types: v })} style={{ minWidth: 200 }} />
        <Space>
          <InputNumber aria-label={t('content_studio.questionSet.difficultyMin')} placeholder={t('content_studio.questionSet.difficultyMin')} disabled={readOnly} min={1} max={5} value={f.difficulty?.min} onChange={(v) => set({ difficulty: { ...f.difficulty, min: v ?? undefined } })} />
          <InputNumber aria-label={t('content_studio.questionSet.difficultyMax')} placeholder={t('content_studio.questionSet.difficultyMax')} disabled={readOnly} min={1} max={5} value={f.difficulty?.max} onChange={(v) => set({ difficulty: { ...f.difficulty, max: v ?? undefined } })} />
        </Space>
        <InputNumber aria-label={t('content_studio.questionSet.count', { n: index + 1 })} addonBefore={t('content_studio.questionSet.countLabel')} disabled={readOnly} min={1} max={SET_LIMITS.count} value={rule.count} onChange={(v) => onChange({ ...rule, count: v ?? 1 })} />
      </Flex>
    </Card>
  );
}

export interface SelectionRuleEditorProps {
  state: QuestionSetState;
  onChange: (s: QuestionSetState) => void;
  readOnly: boolean;
  pool?: PoolStats;
  /** No saved version yet: pool sizes appear after the first save. */
  saved: boolean;
}

export function SelectionRuleEditor({ state, onChange, readOnly, pool, saved }: SelectionRuleEditorProps) {
  const { t } = useTranslation();
  const [picking, setPicking] = useState(false);
  const showItems = state.kind !== 'rule_based';
  const showRules = state.kind !== 'static';
  return (
    <Flex vertical gap="middle" data-testid="selection-rule-editor">
      <Segmented
        aria-label={t('content_studio.questionSet.kind')}
        value={state.kind}
        disabled={readOnly}
        onChange={(v) => onChange({ ...state, kind: v as SetKind })}
        options={(['static', 'rule_based', 'mixed'] as const).map((k) => ({ value: k, label: t(`content_studio.questionSet.kinds.${k}`) }))}
      />
      {showItems && (
        <section aria-label={t('content_studio.questionSet.items')}>
          <Typography.Title level={4}>{t('content_studio.questionSet.items')}</Typography.Title>
          <List
            size="small"
            bordered
            dataSource={state.items}
            locale={{ emptyText: t('content_studio.questionSet.noItems') }}
            renderItem={(it, i) => (
              <List.Item
                key={`${it.question_version_id}:${i}`}
                actions={
                  readOnly
                    ? []
                    : [
                        <Button key="rm" size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.questionSet.removeItem', { n: i + 1 })} onClick={() => onChange({ ...state, items: state.items.filter((_, k) => k !== i) })} />,
                      ]
                }
              >
                {`${i + 1}. ${it.label ?? it.question_version_id}`}
              </List.Item>
            )}
          />
          {!readOnly && (
            <Button disabled={state.items.length >= SET_LIMITS.items} onClick={() => setPicking(true)}>
              {t('content_studio.questionSet.addQuestion')}
            </Button>
          )}
        </section>
      )}
      {showRules && (
        <section aria-label={t('content_studio.questionSet.rules')}>
          <Typography.Title level={4}>{t('content_studio.questionSet.rules')}</Typography.Title>
          {!saved && <Alert type="info" showIcon message={t('content_studio.questionSet.saveForPool')} />}
          <Flex vertical gap="small">
            {state.rules.map((r, i) => (
              <RuleCard
                key={i}
                rule={r}
                index={i}
                state={state}
                readOnly={readOnly}
                pool={pool?.rules.find((p) => p.rule_index === i)}
                onChange={(nr) => onChange({ ...state, rules: state.rules.map((x, k) => (k === i ? nr : x)) })}
                onRemove={() => onChange({ ...state, rules: state.rules.filter((_, k) => k !== i) })}
              />
            ))}
          </Flex>
          {!readOnly && (
            <Button disabled={state.rules.length >= SET_LIMITS.rules} onClick={() => onChange({ ...state, rules: [...state.rules, { filters: {}, count: 5 }] })}>
              {t('content_studio.questionSet.addRule')}
            </Button>
          )}
        </section>
      )}
      <QuestionPicker
        open={picking}
        onClose={() => setPicking(false)}
        onPick={(q) => {
          onChange({ ...state, items: [...state.items, { question_version_id: q.content_version_id!, label: t('content_studio.questionSet.questionLabel', { id: q.content_id.slice(-8), version: q.version, type: q.type }) }] });
          setPicking(false);
        }}
      />
    </Flex>
  );
}

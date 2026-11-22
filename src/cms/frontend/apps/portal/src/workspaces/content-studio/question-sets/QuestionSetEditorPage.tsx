// QuestionSetEditorPage (F054-AC-20, AC-23, ADMIN_PORTAL §3.2): Question Set editor on the shared builder frame —
// metadata, purpose, shuffle options, static items and/or selection rules with the pool size of each rule.
import { Flex, Form, Switch } from 'antd';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { DocMetaFields } from '../builders/DocMetaFields';
import { DocumentEditorPage, type BuilderArgs } from '../builders/DocumentEditorPage';
import { usePoolStats, type DocAuthoring, type QuestionSetAuthoring } from '../builders/useDocument';
import { emptyQuestionSet, fromQuestionSet, questionSetIssues, toQuestionSetBody, type QuestionSetState, type SetPurpose } from './questionSetModel';
import { SelectionRuleEditor } from './SelectionRuleEditor';
import { SearchSelect } from '@eduworld/ops-ui';

const PURPOSES: SetPurpose[] = ['exercise', 'minigame', 'quest', 'competition', 'general'];

function QuestionSetBody({ state, onChange, readOnly, errors, server, id }: BuilderArgs<QuestionSetState> & { id?: string }) {
  const { t } = useTranslation();
  const pool = usePoolStats('question_set', id, server?.version, state.kind !== 'static');
  return (
    <Flex vertical gap="middle">
      <DocMetaFields value={state} onChange={onChange} readOnly={readOnly} languageLocked={!!id} errors={errors} />
      <Form layout="vertical" component="div" disabled={readOnly}>
        <Form.Item label={t('content_studio.questionSet.purpose')}>
          <SearchSelect aria-label={t('content_studio.questionSet.purpose')} value={state.purpose} options={PURPOSES.map((p) => ({ value: p, label: t(`content_studio.questionSet.purposes.${p}`) }))} onChange={(purpose: SetPurpose) => onChange({ ...state, purpose })} />
        </Form.Item>
        <Flex gap="large" wrap>
          <Flex gap="small" align="center">
            <Switch aria-label={t('content_studio.questionSet.shuffleItems')} checked={state.shuffle_items} onChange={(v) => onChange({ ...state, shuffle_items: v })} />
            {t('content_studio.questionSet.shuffleItems')}
          </Flex>
          <Flex gap="small" align="center">
            <Switch aria-label={t('content_studio.questionSet.shuffleOptions')} checked={state.shuffle_options} onChange={(v) => onChange({ ...state, shuffle_options: v })} />
            {t('content_studio.questionSet.shuffleOptions')}
          </Flex>
        </Flex>
      </Form>
      <SelectionRuleEditor state={state} onChange={onChange} readOnly={readOnly} pool={pool.data} saved={!!id} />
    </Flex>
  );
}

export function QuestionSetEditorPage({ id }: { id?: string }) {
  const { t } = useTranslation();
  const blockers = useCallback((s: QuestionSetState) => questionSetIssues(s, t), [t]);
  return (
    <DocumentEditorPage<QuestionSetState>
      type="question_set"
      id={id}
      title={{ create: t('content_studio.questionSet.newTitle'), edit: t('content_studio.questionSet.editTitle') }}
      empty={emptyQuestionSet}
      fromServer={(d: DocAuthoring) => fromQuestionSet(d as QuestionSetAuthoring)}
      toBody={async (s) => toQuestionSetBody(s)}
      blockers={blockers}
      body={(a) => <QuestionSetBody {...a} id={id} />}
    />
  );
}

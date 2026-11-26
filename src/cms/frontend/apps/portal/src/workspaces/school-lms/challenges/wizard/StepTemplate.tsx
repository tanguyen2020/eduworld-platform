// Step 1 "Template" (F108 T028; F108-AC-14): templates assigned to the tenant only (listChallengeTemplates — the server
// returns only assigned ones; whole list, searchable) and a read-only summary of the chosen template: type, format,
// grades, fixed reward (the school never edits rules or rewards — SC-003). Presentational only (AGENTS R1).
import { Descriptions, Form, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import type { ChallengeTemplate } from '../model/challengeForm';
import { ChallengeProblem, EnumTag } from '../ChallengeUi';
import { SearchSelect } from '@eduworld/ops-ui';

export interface StepTemplateProps {
  templates: ChallengeTemplate[];
  loading: boolean;
  loadError: unknown;
  value: string | undefined;
  onChange: (templateId: string) => void;
  error?: string;
}

export function StepTemplate({ templates, loading, loadError, value, onChange, error }: StepTemplateProps) {
  const { t } = useTranslation();
  const tpl = templates.find((x) => x.template_id === value);
  return (
    <>
      <ChallengeProblem error={loadError} />
      <Form.Item label={t('lms.challenges.wizard.template')} required validateStatus={error ? 'error' : undefined} help={error}>
        <SearchSelect
          showSearch
          optionFilterProp="label"
          aria-label={t('lms.challenges.wizard.template')}
          placeholder={t('lms.challenges.wizard.pickTemplate')}
          loading={loading}
          value={value}
          onChange={(id: string) => onChange(id)}
          notFoundContent={t('lms.challenges.wizard.noTemplate')}
          options={templates.map((x) => ({ value: x.template_id, label: x.name }))}
        />
      </Form.Item>
      {tpl ? (
        <Descriptions bordered size="small" column={1} data-testid="template-summary">
          <Descriptions.Item label={t('lms.challenges.detail.kind')}>
            <EnumTag group="kind" value={tpl.kind} />
            <EnumTag group="format" value={tpl.format} />
            <EnumTag group="tournamentFormat" value={tpl.tournament_format} />
            <EnumTag group="participantType" value={tpl.participant_type} />
          </Descriptions.Item>
          <Descriptions.Item label={t('lms.challenges.wizard.grades')}>{(tpl.grade_levels ?? []).map((n) => t('lms.challenges.gradeN', { n })).join(', ')}</Descriptions.Item>
          {tpl.description ? <Descriptions.Item label={t('lms.challenges.wizard.description')}>{tpl.description}</Descriptions.Item> : null}
          {tpl.reward_summary ? (
            <Descriptions.Item label={t('lms.challenges.wizard.reward')}>
              <Typography.Text>{tpl.reward_summary}</Typography.Text>
            </Descriptions.Item>
          ) : null}
        </Descriptions>
      ) : null}
    </>
  );
}

// SimulationTemplatesScreen (F066 T053, F066-AC-26, F066-AC-29; menu `menu.content_studio.simulations` of the catalog):
// the packaged simulation templates of module learning (listSimulationTemplates) — code, version, kind, status, server
// checker, parameters and metrics. Read only: templates ship with the code (ADR-0052, no script upload); activities use
// them from the Activity Builder (SimulationEditor).
import { Flex, Table, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useSimulationTemplates, type SimulationTemplate } from '../activities/editors/simulation/SimulationEditor';

function TemplateTable() {
  const { t } = useTranslation();
  const templates = useSimulationTemplates();
  return (
    <>
      {templates.error ? <ProblemMessage error={templates.error} /> : null}
      <Table<SimulationTemplate>
        rowKey={(x) => `${x.template_code}@${x.template_version}`}
        loading={templates.isFetching}
        pagination={false}
        data-testid="simulation-templates"
        dataSource={templates.data?.data ?? []}
        columns={[
          { title: t('content_studio.simulationTemplates.code'), render: (_, x) => `${x.template_code} v${x.template_version}` },
          { title: t('content_studio.simulationTemplates.titleCol'), dataIndex: 'title' },
          { title: t('content_studio.simulationTemplates.kind'), render: (_, x) => x.simulation_kind ?? '—' },
          { title: t('content_studio.simulationTemplates.status'), render: (_, x) => <Tag>{t(`content_studio.simulationTemplates.statuses.${x.status ?? 'active'}`)}</Tag> },
          { title: t('content_studio.simulationTemplates.checker'), render: (_, x) => t(x.checker_available ? 'content_studio.simulationTemplates.yes' : 'content_studio.simulationTemplates.no') },
          { title: t('content_studio.simulationTemplates.parameters'), render: (_, x) => Object.keys((x.parameter_schema as { properties?: object }).properties ?? {}).join(', ') },
          { title: t('content_studio.simulationTemplates.metrics'), render: (_, x) => (x.metrics ?? []).map((m) => m.metric).join(', ') },
        ]}
      />
    </>
  );
}

export function SimulationTemplatesScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  return (
    <Flex vertical gap="middle">
      <Typography.Title level={1}>{title}</Typography.Title>
      <Typography.Paragraph type="secondary">{t('content_studio.simulationTemplates.screenHelp')}</Typography.Paragraph>
      <ActionGate actionKey="action.content_studio.simulations.view">
        <TemplateTable />
      </ActionGate>
    </Flex>
  );
}

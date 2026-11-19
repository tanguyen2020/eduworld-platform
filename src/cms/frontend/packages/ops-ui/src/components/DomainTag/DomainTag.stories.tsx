// DomainTag stories (F050-AC-08, AC-18): every business color with its label, light / dark / compact.
import type { Meta, StoryObj } from '@storybook/react';
import { Flex, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { domainColors } from '../../theme/antd-theme';
import { OPS_UI_NS } from '../../i18n';
import { DomainTag, type DomainKind } from './DomainTag';

const groups: DomainKind[] = ['subject', 'rarity', 'content_status', 'brand'];

function AllDomainTags() {
  const { t } = useTranslation(OPS_UI_NS);
  return (
    <Flex vertical gap="middle">
      {groups.map((kind) => (
        <section key={kind}>
          <Typography.Text type="secondary">{t(`group.${kind}`)}</Typography.Text>
          <Flex wrap gap="small">
            {Object.keys(domainColors)
              .filter((k) => k.startsWith(`color.${kind}.`))
              .map((k) => (
                <DomainTag key={k} kind={kind} value={k.split('.')[2]!} />
              ))}
          </Flex>
        </section>
      ))}
    </Flex>
  );
}

const meta: Meta<typeof DomainTag> = {
  title: 'ops-ui/DomainTag',
  component: DomainTag,
  render: () => <AllDomainTags />,
};
export default meta;

type Story = StoryObj<typeof DomainTag>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };

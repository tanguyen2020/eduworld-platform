// Screen of `menu.school_lms.game_activities` (F055-AC-22, FR-LMS-007): read-only catalog of published quests and
// mini-games that can be assigned (listGameActivities). Workspace C has no screen or button to configure them
// (ADR-0027); assigning goes through the wizard.
import { Flex, List, Select, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SingleTenantGate } from '../../common/scope';
import { PageTitle } from '../../common/ui';
import { usePickerChoices } from '../api/contentPicker';
import { LmsProblem } from '../LmsUi';

function Catalog() {
  const { t, i18n } = useTranslation();
  const [grade, setGrade] = useState<number | undefined>();
  const items = usePickerChoices('game', { gradeLevel: grade }, i18n.language);
  return (
    <Flex vertical gap="middle" data-testid="game-activities">
      <Typography.Paragraph type="secondary">{t('lms.games.help')}</Typography.Paragraph>
      <Select
        allowClear
        aria-label={t('lms.wizard.grade')}
        placeholder={t('lms.wizard.grade')}
        value={grade}
        onChange={setGrade}
        options={[1, 2, 3, 4, 5, 6, 7, 8, 9].map((g) => ({ value: g, label: t('lms.wizard.gradeN', { n: g }) }))}
      />
      <LmsProblem error={items.error} />
      <List
        loading={items.isFetching}
        dataSource={items.data ?? []}
        locale={{ emptyText: t('lms.picker.empty') }}
        renderItem={(c) => (
          <List.Item>
            <Typography.Text>{c.title}</Typography.Text> <Tag>{t(`lms.contentType.${c.contentType}`)}</Tag>
          </List.Item>
        )}
      />
    </Flex>
  );
}

export function GameActivitiesScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <Catalog />}</SingleTenantGate>
    </section>
  );
}

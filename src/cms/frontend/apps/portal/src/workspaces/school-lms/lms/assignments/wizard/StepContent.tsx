// Step 1 "Nội dung" (F055 T018, F055-AC-01, AC-22): tabs Activity / Bộ câu hỏi / Đề / Bài học / Hoạt động game, grade
// filter; the lists come from usePickerChoices (published, server-graded, quests and mini-games read-only).
import { Button, Empty, Flex, List, Select, Tabs, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePickerChoices, type PickerTab } from '../../api/contentPicker';
import type { ContentChoice } from '../../model/assignmentForm';
import { LmsProblem } from '../../LmsUi';

export const PICKER_TABS: PickerTab[] = ['activity', 'question_set', 'assessment_blueprint', 'lesson', 'game'];
const GRADES = [1, 2, 3, 4, 5, 6, 7, 8, 9];

export interface ContentPickerProps {
  value?: ContentChoice;
  onChange: (c: ContentChoice) => void;
  tabs?: PickerTab[];
  objectiveId?: string;
  /** Path builder: game content is limited to quests (PathNode content types). */
  questsOnly?: boolean;
}

export function ContentPicker({ value, onChange, tabs = PICKER_TABS, objectiveId, questsOnly }: ContentPickerProps) {
  const { t, i18n } = useTranslation();
  const [tab, setTab] = useState<PickerTab>(tabs[0]!);
  const [grade, setGrade] = useState<number | undefined>();
  const choices = usePickerChoices(tab, { gradeLevel: grade, objectiveId }, i18n.language);
  const items = (choices.data ?? []).filter((c) => !questsOnly || tab !== 'game' || c.contentType === 'quest');
  return (
    <Flex vertical gap="small" data-testid="content-picker">
      <Select
        allowClear
        aria-label={t('lms.wizard.grade')}
        placeholder={t('lms.wizard.grade')}
        value={grade}
        onChange={setGrade}
        options={GRADES.map((g) => ({ value: g, label: t('lms.wizard.gradeN', { n: g }) }))}
      />
      <Tabs activeKey={tab} onChange={(k) => setTab(k as PickerTab)} items={tabs.map((k) => ({ key: k, label: t(`lms.picker.${k}`) }))} />
      <LmsProblem error={choices.error} />
      <List
        loading={choices.isFetching}
        locale={{ emptyText: <Empty description={t('lms.picker.empty')} /> }}
        dataSource={items}
        renderItem={(c) => {
          const selected = value?.contentId === c.contentId && value.contentType === c.contentType;
          return (
            <List.Item
              actions={[
                <Button key="pick" type={selected ? 'primary' : 'default'} aria-pressed={selected} onClick={() => onChange(c)}>
                  {selected ? t('lms.picker.selected') : t('lms.picker.pick')}
                </Button>,
              ]}
            >
              <Typography.Text>{c.title}</Typography.Text> <Tag>{t(`lms.contentType.${c.contentType}`)}</Tag>
            </List.Item>
          );
        }}
      />
    </Flex>
  );
}

export function StepContent({ value, onChange, objectiveId, error }: { value?: ContentChoice; onChange: (c: ContentChoice) => void; objectiveId?: string; error?: string }) {
  const { t } = useTranslation();
  return (
    <Flex vertical gap="small">
      {value ? (
        <Typography.Text data-testid="selected-content">
          {t('lms.wizard.selectedContent', { title: value.title })} <Tag>{t(`lms.contentType.${value.contentType}`)}</Tag>
        </Typography.Text>
      ) : null}
      {error ? (
        <Typography.Text type="danger" role="alert">
          {error}
        </Typography.Text>
      ) : null}
      <ContentPicker value={value} onChange={onChange} objectiveId={objectiveId} />
    </Flex>
  );
}

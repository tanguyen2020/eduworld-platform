// Screens of the F054 menus of workspace B (ADR-0090: menu keys and paths from the generated catalog): list + editor
// routes of question sets, lessons and assessment blueprints; the release page of a learning path opened by id (the
// contract has no learning-path authoring API yet — F054 manages its versions and releases only).
import { Button, Card, Flex, Input, Space, Typography } from 'antd';
import { useState, type ComponentType } from 'react';
import { useTranslation } from 'react-i18next';
import { Route, Routes, useNavigate, useParams } from 'react-router-dom';
import { BlueprintEditorPage } from './blueprints/BlueprintEditorPage';
import { LessonBuilderPage } from './lessons/LessonBuilderPage';
import { ContentListPage } from './list/ContentListPage';
import { QuestionSetEditorPage } from './question-sets/QuestionSetEditorPage';
import { ContentReleaseTabs } from './release/ContentReleaseTabs';
import { useVersions } from './release/useRelease';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

type EditorPage = ComponentType<{ id?: string }>;

function Edit({ page: Page }: { page: EditorPage }) {
  const { id } = useParams();
  return <Page id={id} />;
}

function docScreen(type: 'question_set' | 'lesson' | 'assessment_blueprint', Page: EditorPage) {
  return function DocScreen({ title }: { title: string }) {
    return (
      <Routes>
        <Route index element={<ContentListPage type={type} title={title} />} />
        <Route path="new" element={<Page />} />
        <Route path=":id" element={<Edit page={Page} />} />
      </Routes>
    );
  };
}

export const QuestionSetsScreen = docScreen('question_set', QuestionSetEditorPage);
export const LessonsScreen = docScreen('lesson', LessonBuilderPage);
export const AssessmentsScreen = docScreen('assessment_blueprint', BlueprintEditorPage);

function LearningPathRelease() {
  const { id } = useParams();
  const versions = useVersions({ contentType: 'learning_path', contentId: id! });
  const latest = Math.max(1, ...(versions.data?.data ?? []).map((v) => v.version));
  return (
    <Card>
      <ContentReleaseTabs item={{ contentType: 'learning_path', contentId: id! }} version={latest} />
    </Card>
  );
}

function LearningPathHome({ title }: { title: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [id, setId] = useState('');
  const valid = UUID.test(id.trim());
  return (
    <Flex vertical gap="middle">
      <Typography.Title level={1}>{title}</Typography.Title>
      <Typography.Paragraph type="secondary">{t('content_studio.learningPath.help')}</Typography.Paragraph>
      <Space.Compact>
        <Input aria-label={t('content_studio.learningPath.openById')} placeholder={t('content_studio.learningPath.openById')} value={id} maxLength={36} onChange={(e) => setId(e.target.value)} onPressEnter={() => valid && navigate(id.trim())} />
        <Button disabled={!valid} onClick={() => navigate(id.trim())}>
          {t('content_studio.question.open')}
        </Button>
      </Space.Compact>
    </Flex>
  );
}

export function LearningPathsScreen({ title }: { title: string }) {
  return (
    <Routes>
      <Route index element={<LearningPathHome title={title} />} />
      <Route path=":id" element={<LearningPathRelease />} />
    </Routes>
  );
}

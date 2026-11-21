// Screens of the F054 menus of workspace B (ADR-0090: menu keys and paths from the generated catalog): list + editor
// routes of question sets, lessons and assessment blueprints (ADR-0095: no learning path).
import { type ComponentType } from 'react';
import { Route, Routes, useParams } from 'react-router-dom';
import { BlueprintEditorPage } from './blueprints/BlueprintEditorPage';
import { LessonBuilderPage } from './lessons/LessonBuilderPage';
import { ContentListPage } from './list/ContentListPage';
import { QuestionSetEditorPage } from './question-sets/QuestionSetEditorPage';

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

// QuestionBankScreen — menu `menu.content_studio.question_bank` (F053 US1, F054 US1/US5). Sub-routes under the catalog
// path: index = list + search (F054-AC-24), `new`, `import` (F054-AC-27, `learning_content.import`), `:questionId` =
// editor (F053) + release tabs (versions, releases, usages, preview — F054 US1). Paths come from the generated catalog
// (ADR-0090).
import { Collapse } from 'antd';
import { useTranslation } from 'react-i18next';
import { Route, Routes, useParams } from 'react-router-dom';
import { ImportWizardPage } from '../import/ImportWizardPage';
import { ContentListPage } from '../list/ContentListPage';
import { ContentReleaseTabs } from '../release/ContentReleaseTabs';
import { QuestionEditorPage } from './QuestionEditorPage';
import { useCmsQuestion } from './useQuestion';

/**
 * Release part of a saved question (reads the cached question of the editor), folded by default so the heavy question
 * editor renders first; opening it mounts the tabs.
 */
function QuestionRelease({ questionId }: { questionId: string }) {
  const { t } = useTranslation();
  const q = useCmsQuestion(questionId);
  const data = q.data?.data;
  if (!data) return null;
  return (
    <Collapse
      destroyOnHidden
      items={[
        {
          key: 'release',
          label: t('content_studio.release.section'),
          children: <ContentReleaseTabs item={{ contentType: 'question', contentId: questionId }} version={data.version} assigned={!!data.assigned} contentLanguage={data.content_language === 'en' ? 'en' : 'vi'} onDraftCreated={() => void q.refetch()} />,
        },
      ]}
    />
  );
}

function EditRoute() {
  const { questionId } = useParams();
  return (
    <>
      <QuestionEditorPage questionId={questionId} />
      {questionId && <QuestionRelease questionId={questionId} />}
    </>
  );
}

export function QuestionBankScreen({ title }: { title: string }) {
  return (
    <Routes>
      <Route index element={<ContentListPage type="question" title={title} />} />
      <Route path="new" element={<QuestionEditorPage />} />
      <Route path="import" element={<ImportWizardPage />} />
      <Route path=":questionId" element={<EditRoute />} />
    </Routes>
  );
}

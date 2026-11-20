// "Chấm lại đề" of an assignment with a graded exam session (F055 T048, F055-AC-23): only with
// learning_content.re_evaluate (and the catalog action), opens the shared RegradeDialog of F058 with scope
// exam_session; the questions come from the locked papers (listExamSessionPapers). No manual score field anywhere.
import { Button } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../../permissions/ActionGate';
import { RegradeDialog } from '../../../content-studio/regrade/RegradeDialog';
import { useExamPapers } from '../api/exams';
import { useHasPermission } from '../LmsUi';

export function ExamRegradeButton({ examSessionId }: { examSessionId: string }) {
  const { t } = useTranslation();
  const allowed = useHasPermission('learning_content.re_evaluate');
  const [open, setOpen] = useState(false);
  const papers = useExamPapers(open ? examSessionId : undefined);
  if (!allowed) return null;
  const seen = new Set<string>();
  const questionOptions = (papers.data?.data ?? []).flatMap((p) =>
    p.sections.flatMap((s) =>
      s.items
        .filter((i) => !seen.has(i.question_version_id) && seen.add(i.question_version_id))
        .map((i) => ({ value: i.question_version_id, label: t('lms.regrade.questionLabel', { variant: p.variant_no, position: i.position }) })),
    ),
  );
  return (
    <>
      <ActionGate actionKey="action.content_studio.regrade.request">
        <Button onClick={() => setOpen(true)} data-testid="exam-regrade">
          {t('lms.regrade.open')}
        </Button>
      </ActionGate>
      {open && <RegradeDialog open scope="exam_session" versionId={examSessionId} questionOptions={questionOptions} onClose={() => setOpen(false)} />}
    </>
  );
}

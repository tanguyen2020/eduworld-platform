// QuestionBankScreen — menu `menu.content_studio.question_bank` (F053 US1). Sub-routes under the catalog path:
// index (entry: "Tạo câu hỏi", open a question by its id), `new`, `:questionId`. Search / lists / import / review queue
// belong to F054 (spec "Ngoài phạm vi"); paths come from the generated catalog (ADR-0090).
import { Button, Card, Flex, Input, Space, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Route, Routes, useNavigate, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { QuestionEditorPage } from './QuestionEditorPage';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function QuestionBankHome({ title }: { title: string }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [id, setId] = useState('');
  const valid = UUID.test(id.trim());
  return (
    <section aria-labelledby="ew-cs-qb">
      <Flex vertical gap="middle">
        <Typography.Title level={1} id="ew-cs-qb">
          {title}
        </Typography.Title>
        <Card>
          <Flex vertical gap="middle">
            <ActionGate actionKey="action.content_studio.question_bank.edit">
              <Button type="primary" onClick={() => navigate('new')}>
                {t('content_studio.question.create')}
              </Button>
            </ActionGate>
            <Space.Compact>
              <Input aria-label={t('content_studio.question.openById')} placeholder={t('content_studio.question.openById')} value={id} maxLength={36} onChange={(e) => setId(e.target.value)} onPressEnter={() => valid && navigate(id.trim())} />
              <Button disabled={!valid} onClick={() => navigate(id.trim())}>
                {t('content_studio.question.open')}
              </Button>
            </Space.Compact>
            <Typography.Text type="secondary">{t('content_studio.question.searchLater')}</Typography.Text>
          </Flex>
        </Card>
      </Flex>
    </section>
  );
}

function EditRoute() {
  const { questionId } = useParams();
  return <QuestionEditorPage questionId={questionId} />;
}

export function QuestionBankScreen({ title }: { title: string }) {
  return (
    <Routes>
      <Route index element={<QuestionBankHome title={title} />} />
      <Route path="new" element={<QuestionEditorPage />} />
      <Route path=":questionId" element={<EditRoute />} />
    </Routes>
  );
}

// FixedForm (F054 T071, from F035; LEARNING_EFFICACY_PLAN §3.2, EXAM_ENGINE §3): a fixed-form blueprint section — the
// same published question versions for every student, in order, each with its points (rules = runs of equal points,
// `fixed_question_version_ids`, `variants.strategy = fixed` in blueprintModel). Questions come from the question bank
// picker (published versions only); the server checks the references (efficacy instruments only for an efficacy
// blueprint, 422 otherwise). Presentational: totals are computed from the rules sent.
import { ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Flex, InputNumber, List, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { QuestionPicker } from '../question-sets/SelectionRuleEditor';
import { fixedRules, type FixedItem } from './blueprintModel';
import { BP_LIMITS, totalPoints, totalQuestions } from './matrixModel';

export interface FixedFormProps {
  items: FixedItem[];
  onChange: (items: FixedItem[]) => void;
  targetPoints: number;
  readOnly: boolean;
}

/** Most questions of a fixed form: BP_LIMITS.rules rules × BP_LIMITS.count questions (schema of the section). */
const MAX_ITEMS = BP_LIMITS.rules * BP_LIMITS.count;

export function FixedForm({ items, onChange, targetPoints, readOnly }: FixedFormProps) {
  const { t } = useTranslation();
  const [picking, setPicking] = useState(false);
  const rules = fixedRules(items);
  const points = totalPoints(rules);
  const move = (i: number, by: number) => {
    const next = [...items];
    const [x] = next.splice(i, 1);
    next.splice(i + by, 0, x!);
    onChange(next);
  };
  return (
    <Card size="small" title={t('content_studio.blueprint.fixed.title')} data-field="fixed">
      <Flex vertical gap="small">
        <Typography.Paragraph type="secondary">{t('content_studio.blueprint.fixed.hint')}</Typography.Paragraph>
        <List
          size="small"
          data-testid="fixed-items"
          locale={{ emptyText: t('content_studio.blueprint.fixed.empty') }}
          dataSource={items}
          renderItem={(it, i) => (
            <List.Item
              key={it.question_version_id}
              actions={
                readOnly
                  ? []
                  : [
                      <Button key="up" size="small" icon={<ArrowUpOutlined />} disabled={i === 0} aria-label={t('content_studio.blueprint.fixed.up', { n: i + 1 })} onClick={() => move(i, -1)} />,
                      <Button key="down" size="small" icon={<ArrowDownOutlined />} disabled={i === items.length - 1} aria-label={t('content_studio.blueprint.fixed.down', { n: i + 1 })} onClick={() => move(i, 1)} />,
                      <Button key="rm" size="small" danger icon={<DeleteOutlined />} aria-label={t('content_studio.blueprint.fixed.remove', { n: i + 1 })} onClick={() => onChange(items.filter((_, j) => j !== i))} />,
                    ]
              }
            >
              <Flex gap="middle" align="center" wrap>
                <Typography.Text>{t('content_studio.blueprint.fixed.item', { n: i + 1, id: it.question_version_id.slice(-8) })}</Typography.Text>
                <InputNumber
                  size="small"
                  aria-label={t('content_studio.blueprint.fixed.points', { n: i + 1 })}
                  addonAfter={t('content_studio.blueprint.pointsUnit')}
                  min={0.25}
                  step={0.25}
                  disabled={readOnly}
                  value={it.points}
                  onChange={(v) => onChange(items.map((x, j) => (j === i ? { ...x, points: v ?? 1 } : x)))}
                />
              </Flex>
            </List.Item>
          )}
        />
        {!readOnly && (
          <Button onClick={() => setPicking(true)} disabled={items.length >= MAX_ITEMS}>
            {t('content_studio.blueprint.fixed.add')}
          </Button>
        )}
        <Typography.Text data-testid="fixed-totals">{t('content_studio.blueprint.totals', { questions: totalQuestions(rules), points, target: targetPoints })}</Typography.Text>
        {items.length > 0 && Math.abs(points - targetPoints) > 0.001 ? <Alert type="warning" showIcon message={t('content_studio.blueprint.fixedPointsMismatch', { points, target: targetPoints })} /> : null}
      </Flex>
      <QuestionPicker
        open={picking}
        onClose={() => setPicking(false)}
        onPick={(q) => {
          const id = q.content_version_id;
          if (id && !items.some((x) => x.question_version_id === id)) onChange([...items, { question_version_id: id, points: 1, type: q.type }]);
          setPicking(false);
        }}
      />
    </Card>
  );
}

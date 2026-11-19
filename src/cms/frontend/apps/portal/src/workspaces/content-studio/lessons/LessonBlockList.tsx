// LessonBlockList (F054-AC-21, ADMIN_PORTAL §3.2): the blocks of a lesson — rich_text (RichContentEditor), media
// (MediaPicker + bilingual caption), worked_example (WorkedExampleBlock, F053) and activity (activity id) — added up to
// `blocks.maxItems`, reordered by drag & drop (HTML5) or with the Lên / Xuống buttons (keyboard), each marked
// required or not. Formulas inside the blocks are rendered by the page into `math_assets` on save.
import { ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined, HolderOutlined } from '@ant-design/icons';
import { BilingualField, RichContentEditor, WorkedExampleBlock, type MediaSlotArgs } from '@eduworld/ops-ui/content';
import { Button, Card, Checkbox, Flex, Input, Space, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { MediaPicker } from '../media/MediaPicker';
import { LIMITS } from '../shared/config';
import { needsSpokenStem } from '../shared/gates';
import { addBlock, BLOCK_KINDS, LESSON_LIMITS, moveBlock, type BlockKind, type LessonBlock, type LessonState } from './lessonModel';

export interface LessonBlockListProps {
  state: LessonState;
  onChange: (s: LessonState) => void;
  readOnly: boolean;
}

export function LessonBlockList({ state, onChange, readOnly }: LessonBlockListProps) {
  const { t } = useTranslation();
  const [dragging, setDragging] = useState<number | null>(null);
  const blocks = state.blocks;
  const setBlocks = (b: LessonBlock[]) => onChange({ ...state, blocks: b });
  const setBlock = (i: number, patch: Partial<LessonBlock>) => setBlocks(blocks.map((b, k) => (k === i ? { ...b, ...patch } : b)));
  const lang = state.content_language;
  const workedIds = blocks.filter((b) => b.kind === 'worked_example').map((b) => b.id ?? '');
  const mediaSlot = (a: MediaSlotArgs) => <MediaPicker id={a.id} kind={a.kind} value={a.value} onChange={a.onChange} disabled={readOnly} />;
  const full = blocks.length >= LESSON_LIMITS.blocks;

  const body = (b: LessonBlock, i: number) => {
    switch (b.kind) {
      case 'rich_text':
        return <RichContentEditor id={`blocks.${i}.content`} label={t('content_studio.lesson.kinds.rich_text')} value={b.content} onChange={(content) => setBlock(i, { content })} contentLanguage={lang} readOnly={readOnly} mediaSlot={mediaSlot} />;
      case 'media':
        return (
          <Flex vertical gap="small">
            <MediaPicker id={`blocks.${i}.media_ref`} kind="image" value={b.media_ref} onChange={(media_ref) => setBlock(i, { media_ref })} disabled={readOnly} />
            <BilingualField id={`blocks.${i}.caption`} label={t('content_studio.lesson.caption')} value={b.caption} onChange={(v) => setBlock(i, { caption: (v ?? {}) as LessonBlock['caption'] })} contentLanguage={lang} maxLength={LIMITS.name} readOnly={readOnly} />
          </Flex>
        );
      case 'worked_example':
        return b.worked ? (
          <WorkedExampleBlock
            value={b.worked}
            onChange={(worked) => setBlock(i, { worked, id: worked.id })}
            contentLanguage={lang}
            requireAudio={needsSpokenStem(state.grade_levels)}
            otherIds={workedIds.filter((x) => x !== b.id)}
            readOnly={readOnly}
            mediaSlot={mediaSlot}
          />
        ) : null;
      case 'activity':
        return <Input aria-label={t('content_studio.lesson.activityId', { n: i + 1 })} placeholder={t('content_studio.lesson.activityIdPlaceholder')} maxLength={36} value={b.activity_id ?? ''} disabled={readOnly} onChange={(e) => setBlock(i, { activity_id: e.target.value.trim() })} />;
    }
  };

  return (
    <Flex vertical gap="middle" data-testid="lesson-blocks">
      <Typography.Text type="secondary">{t('content_studio.lesson.blockCount', { count: blocks.length, max: LESSON_LIMITS.blocks })}</Typography.Text>
      {blocks.map((b, i) => (
        <Card
          key={b.key}
          size="small"
          data-block={b.kind}
          data-field={`blocks.${i}`}
          draggable={!readOnly}
          onDragStart={() => setDragging(i)}
          onDragOver={(e) => e.preventDefault()}
          onDrop={() => {
            if (dragging !== null) setBlocks(moveBlock(blocks, dragging, i));
            setDragging(null);
          }}
          title={
            <Space>
              {!readOnly && <HolderOutlined aria-hidden />}
              <Typography.Text>{t('content_studio.lesson.blockTitle', { n: i + 1, kind: t(`content_studio.lesson.kinds.${b.kind}`) })}</Typography.Text>
              {b.required && <Tag color="blue">{t('content_studio.lesson.required')}</Tag>}
            </Space>
          }
          extra={
            !readOnly && (
              <Space>
                <Checkbox checked={!!b.required} onChange={(e) => setBlock(i, { required: e.target.checked })}>
                  {t('content_studio.lesson.required')}
                </Checkbox>
                <Button size="small" icon={<ArrowUpOutlined />} disabled={i === 0} aria-label={t('content_studio.lesson.moveUp', { n: i + 1 })} onClick={() => setBlocks(moveBlock(blocks, i, i - 1))} />
                <Button size="small" icon={<ArrowDownOutlined />} disabled={i === blocks.length - 1} aria-label={t('content_studio.lesson.moveDown', { n: i + 1 })} onClick={() => setBlocks(moveBlock(blocks, i, i + 1))} />
                <Button size="small" icon={<DeleteOutlined />} aria-label={t('content_studio.lesson.remove', { n: i + 1 })} onClick={() => setBlocks(blocks.filter((_, k) => k !== i))} />
              </Space>
            )
          }
        >
          {body(b, i)}
        </Card>
      ))}
      {!readOnly && (
        <Space wrap>
          {BLOCK_KINDS.map((k: BlockKind) => (
            <Button key={k} disabled={full} onClick={() => setBlocks(addBlock(blocks, k))}>
              {t('content_studio.lesson.add', { kind: t(`content_studio.lesson.kinds.${k}`) })}
            </Button>
          ))}
        </Space>
      )}
    </Flex>
  );
}

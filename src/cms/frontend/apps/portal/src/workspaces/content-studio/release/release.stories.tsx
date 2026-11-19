// Workspace B release stories (F054 T062, TC-UI-005): version diff by group (with / without answers), rollback impact
// before the second confirmation, and the blueprint LO × difficulty matrix with an insufficient cell — Light / Dark /
// Compact. MOCK story data only (no API, no real content).
import type { Meta, StoryObj } from '@storybook/react';
import { Flex } from 'antd';
import { withPortal } from '../../../shell/storyDecorator';
import { BlueprintMatrix } from '../blueprints/BlueprintMatrix';
import { fromRules } from '../blueprints/matrixModel';
import { RollbackImpact } from './RollbackDialog';
import { DiffGroups } from './VersionDiffView';

// MOCK story data
const DIFF = {
  from_version: 1,
  to_version: 2,
  changes: [
    { path: '/stem/blocks/0/text', op: 'changed' as const, before: { vi: 'Tính 7 × 8' }, after: { vi: 'Tính nhẩm 7 × 8' } },
    { path: '/answer_key/correct_option_id', op: 'changed' as const, before: 'o_aaaa', after: 'o_bbbb' },
    { path: '/media_refs/0', op: 'added' as const, after: '0193a4d2-0000-7000-8000-000000000077' },
    { path: '/difficulty', op: 'changed' as const, before: 2, after: 3 },
  ],
};
const LO = ['0193a4d2-0000-7000-8000-000000000031', '0193a4d2-0000-7000-8000-000000000032'];
const MATRIX = fromRules([
  { objective_ids: [LO[0]!], difficulty: { min: 1, max: 1 }, count: 4, points_each: 1 },
  { objective_ids: [LO[1]!], difficulty: { min: 2, max: 2 }, count: 3, points_each: 2 },
]);
const LABELS = new Map([
  [LO[0]!, 'LO1 — Nhân với 2'],
  [LO[1]!, 'LO2 — Nhân với 3'],
]);

function Gallery() {
  return (
    <Flex vertical gap="large">
      <DiffGroups diff={DIFF} canReview />
      <DiffGroups diff={DIFF} canReview={false} />
      <RollbackImpact from={2} to={1} running={7} reason="v2 sai đáp án câu 3" />
      <BlueprintMatrix
        matrix={MATRIX}
        onChange={() => undefined}
        objectiveLabels={LABELS}
        objectiveOptions={[...LABELS].map(([value, label]) => ({ value, label }))}
        targetPoints={10}
        pools={{ [`${LO[1]}:2`]: { available: 1, required: 3, insufficient: true, warning: true }, [`${LO[0]}:1`]: { available: 12, required: 4, insufficient: false, warning: false } }}
        highlightInsufficient
        readOnly={false}
      />
    </Flex>
  );
}

const meta: Meta<typeof Gallery> = { title: 'portal/ContentStudio/Release', component: Gallery, decorators: [withPortal] };
export default meta;

type Story = StoryObj<typeof Gallery>;

export const Light: Story = { parameters: { opsTheme: { mode: 'light', density: 'default' } } };
export const Dark: Story = { parameters: { opsTheme: { mode: 'dark', density: 'default' } } };
export const Compact: Story = { parameters: { opsTheme: { mode: 'light', density: 'compact' } } };

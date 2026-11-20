// F035 T065/T067 — F035-AC-36, AC-37: the "Đo hiệu quả" box shows only in the platform scope (no session tenant) for
// whoever holds the edit action of the editor, never in a tenant scope, and hides when the action is turned off; the
// authoring body sends the flag (with exam_only) only once the author touched it. Permissions and tenant source are
// MOCKS of the shell hooks (test only).
import { fireEvent, render, screen } from '@testing-library/react';
import { I18nextProvider } from 'react-i18next';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createI18n } from '../../../i18n';
import type { PermissionView } from '../../../permissions/menuRegistry';
import { EfficacyInstrumentField } from './EfficacyInstrumentField';
import { toAuthoringBody } from './useQuestion';
import type { QuestionDraft } from '@eduworld/ops-ui/content';

const mock = vi.hoisted(() => ({ tenantId: null as string | null, view: undefined as PermissionView | undefined }));

vi.mock('../../../shared/auth/TenantContext', () => ({ useTenantSource: () => ({ getTenantId: () => mock.tenantId }) }));
vi.mock('../../../permissions/usePermissions', () => ({ usePermissions: () => ({ view: mock.view, isLoading: false, crossTenant: false }) }));

const CONTENT_WRITE: PermissionView = { workspaces: ['content_studio'], permissions: ['learning_content.write', 'learning_content.read'] };

async function renderField(onChange = vi.fn(), checked = false, kind: 'question' | 'blueprint' = 'question') {
  const i18n = await createI18n('vi-VN');
  render(
    <I18nextProvider i18n={i18n}>
      <EfficacyInstrumentField kind={kind} checked={checked} onChange={onChange} actionKey={kind === 'question' ? 'action.content_studio.question_bank.edit' : 'action.content_studio.assessments.edit'} />
    </I18nextProvider>,
  );
  return onChange;
}

afterEach(() => {
  mock.tenantId = null;
  mock.view = undefined;
});

describe('EfficacyInstrumentField (F035-AC-36)', () => {
  it('platform scope + edit permission → the box and the label "Đo hiệu quả"', async () => {
    mock.view = CONTENT_WRITE;
    const onChange = await renderField();
    const box = screen.getByRole('checkbox', { name: 'Đo hiệu quả' });
    fireEvent.click(box);
    expect(onChange).toHaveBeenCalledWith(true);
    expect(screen.getByText(/Chỉ ở phạm vi platform/)).toBeInTheDocument();
  });

  it('checked shows the "Đo hiệu quả" tag; blueprint help mentions fixed variants', async () => {
    mock.view = CONTENT_WRITE;
    await renderField(vi.fn(), true, 'blueprint');
    expect(screen.getAllByText('Đo hiệu quả')).toHaveLength(2);
    expect(screen.getByText(/variants\.strategy = fixed/)).toBeInTheDocument();
  });

  it('never in a tenant scope, even with the edit permission', async () => {
    mock.view = CONTENT_WRITE;
    mock.tenantId = '0193a4d2-0000-7000-8000-00000000000a';
    await renderField();
    expect(screen.queryByRole('checkbox', { name: 'Đo hiệu quả' })).toBeNull();
  });

  it('hidden without the edit permission or when the edit action is turned off (F035-AC-37)', async () => {
    mock.view = { workspaces: ['content_studio'], permissions: ['learning_content.read'] };
    await renderField();
    expect(screen.queryByRole('checkbox', { name: 'Đo hiệu quả' })).toBeNull();
    mock.view = { ...CONTENT_WRITE, disabled_actions: ['action.content_studio.question_bank.edit'] };
    await renderField();
    expect(screen.queryByRole('checkbox', { name: 'Đo hiệu quả' })).toBeNull();
  });
});

describe('toAuthoringBody efficacy flag (F035-AC-01)', () => {
  const draft = { type: 'single_choice', stem: { blocks: [] }, render_payload: {}, answer_key: {}, scoring_rule: { mode: 'all_or_nothing' }, hints: [] } as unknown as QuestionDraft;
  it('not touched → not sent; checked → efficacy_instrument + exam_only; unchecked → false only', async () => {
    expect(toAuthoringBody(draft, { content_language: 'vi' }, [], [])).not.toHaveProperty('efficacy_instrument');
    expect(toAuthoringBody(draft, { content_language: 'vi', efficacy_instrument: true }, [], [])).toMatchObject({ efficacy_instrument: true, exam_only: true });
    const off = toAuthoringBody(draft, { content_language: 'vi', efficacy_instrument: false }, [], []);
    expect(off).toMatchObject({ efficacy_instrument: false });
    expect(off).not.toHaveProperty('exam_only');
  });
});

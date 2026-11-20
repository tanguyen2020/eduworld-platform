// F066 US4 (whole portal + MOCK backend, AGENTS R17; Playwright with the real backend pending — T052): editors of
// audio_lesson and simulation plugged into the F058 registry — AC-25 audio, transcript, segments, auto-graded questions,
// ACTIVITY_CONFIG_INVALID on the field · AC-26 template from listSimulationTemplates, parameters from parameter_schema,
// metrics from the template, no script field, read only without learning_content.activity.write · AC-29 turned-off
// menu.content_studio.simulations hides the template list and its screen.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MockActivities } from '../../../test/mock-activities';
import { P, problem, renderPublishing, stopPortals, TENANT_CONTENT_PERMS, TENANTS, tenantUser, type MockPublishing } from '../../../test/publishing-harness';

afterEach(() => {
  stopPortals();
  vi.unstubAllGlobals();
});

const TENANT = TENANTS.thDemoA.tenant_id;
const meta = { title: { vi: 'Nghe và trả lời' }, grade_levels: [6], subject_id: P(10), objective_ids: [P(31)], difficulty: 2, content_language: 'vi' };
const AUDIO = P(501);
const VTT = P(502);
const QV = P(101);

function withActivities(setup?: (a: MockActivities, m: MockPublishing) => void) {
  return (m: MockPublishing) => {
    const a = new MockActivities(m);
    setup?.(a, m);
    holder.acts = a;
  };
}
const holder: { acts?: MockActivities } = {};

function addMedia(m: MockPublishing) {
  m.media.push({ asset_id: AUDIO, kind: 'audio', status: 'ready', original_filename: 'bai-nghe.mp3', duration_ms: 60_000, tenant_id: TENANT, variants: [] });
  m.media.push({ asset_id: VTT, kind: 'subtitle', status: 'ready', original_filename: 'bai-nghe.vtt', tenant_id: TENANT, variants: [] });
}

const audioDef = (content: Record<string, unknown>) => ({
  activity_type: 'audio_lesson',
  ...meta,
  config: { content, interaction: { allow_speed_change: true, show_transcript: 'after_complete' }, evaluation: { strategies: ['completion', 'auto'], max_score: 10 }, feedback: { timing: 'immediate' }, completion_rule: { mode: 'min_watched_ratio', min_watched_ratio: 0.9 } },
});
const simDef = { activity_type: 'simulation', ...meta, config: { content: { simulation_kind: 'physics', template_code: 'physics.lever', min_template_version: 1, parameters: { arm_length_m: 1, slots_per_side: 4, mass_1_kg: 2, mass_2_kg: 1 }, goals: [{ goal_id: 'g1', description: { vi: 'Cân bằng' }, check: { metric: 'balanced', operator: 'eq', value: 1 } }], seed_policy: 'per_attempt' }, evaluation: { strategies: ['deterministic_check', 'auto'], max_score: 10 }, feedback: { timing: 'immediate' }, completion_rule: { mode: 'goals_met' } } };

describe('Audio lesson and simulation editors (F066 US4)', () => {
  it('AC-25: step 1 offers "Bài nghe" and "Mô phỏng" from the registry', async () => {
    await renderPublishing('/content-studio/activities/new', { account: tenantUser(), setup: withActivities() });
    const step = await screen.findByTestId('activity-type-step');
    expect(await within(step).findByRole('button', { name: 'Bài nghe' })).toBeInTheDocument();
    expect(within(step).getByRole('button', { name: 'Mô phỏng' })).toBeInTheDocument();
  });

  it('AC-25: transcript required, overlapping segments flagged, only published auto-graded questions accepted; an empty segment title is flagged', async () => {
    const ID = P(601);
    const setup = withActivities((a, m) => {
      addMedia(m);
      a.questions = [
        { content_id: P(201), content_version_id: QV, version: 1, type: 'single_choice', status: 'published', title: { vi: 'Câu 1' } },
        { content_id: P(202), version: 1, type: 'single_choice', status: 'draft', title: { vi: 'Câu 2 (bản nháp)' } },
      ];
      a.add(ID, [{ status: 'draft', content: audioDef({ audio_asset_id: AUDIO, segments: [{ segment_id: 's1', title: {}, start_ms: 0, end_ms: 20_000 }, { segment_id: 's2', title: {}, start_ms: 15_000, end_ms: 30_000 }], questions: [] }) }], { tenant_id: TENANT });
    });
    await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup });
    expect(await screen.findByTestId('audio-lesson-editor')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTestId('audio-duration')).toHaveTextContent('60'), { timeout: 30_000 });
    expect(await screen.findByTestId('transcript-issue')).toHaveTextContent('Cần transcript trước khi gửi duyệt.');
    expect((await screen.findAllByText('Đoạn 2 chồng lấn với đoạn 1.')).length).toBeGreaterThan(0);
    expect(screen.getByLabelText('Tiêu đề đoạn 1').closest('.ant-input-affix-wrapper')).toHaveClass('ant-input-status-error');
    fireEvent.click(within(screen.getByTestId('audio-lesson-editor')).getByRole('button', { name: '+ Chọn câu hỏi đã xuất bản' }));
    const dialog = await screen.findByRole('dialog');
    const adds = await within(dialog).findAllByRole('button', { name: /Thêm|Add/ });
    expect(adds[1]).toBeDisabled(); // a question without a published version cannot be picked
    fireEvent.click(adds[0]!);
    expect(await within(screen.getByTestId('audio-questions')).findByText(`Câu 1 · …${QV.slice(-8)}`)).toBeInTheDocument();
  });

  it('AC-25: a valid draft autosaves; 422 ACTIVITY_CONFIG_INVALID is shown on the segment field', async () => {
    const ID = P(602);
    const setup = withActivities((a, m) => {
      addMedia(m);
      a.add(ID, [{ status: 'draft', content: audioDef({ audio_asset_id: AUDIO, transcript_asset_id: VTT, segments: [{ segment_id: 's1', title: { vi: 'Đoạn 1' }, start_ms: 0, end_ms: 20_000 }], questions: [{ question_version_id: QV, at_ms: 10_000 }] }) }], { tenant_id: TENANT });
    });
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup });
    // the draft must be in the form before typing, or loading it would reset the edit
    await waitFor(() => expect(screen.getByLabelText('Tiêu đề (Tiếng Việt)')).toHaveValue('Nghe và trả lời'), { timeout: 30_000 });
    fireEvent.change(screen.getByLabelText('Tiêu đề (Tiếng Việt)'), { target: { value: 'Nghe và trả lời (mới)' } });
    await waitFor(() => expect(backend.calls('PATCH', `/cms/learning/activities/${ID}`)).toHaveLength(1), { timeout: 60_000 });
    const body = backend.calls('PATCH', `/cms/learning/activities/${ID}`)[0]!.body as { config: { content: { audio_asset_id: string; transcript_asset_id: string } } };
    expect(body.config.content).toMatchObject({ audio_asset_id: AUDIO, transcript_asset_id: VTT });
    holder.acts!.nextWrite = problem(422, 'ACTIVITY_CONFIG_INVALID', { errors: [{ field: '/config/content/segments/0/end_ms', code: 'INVALID', message: 'segment ends after the audio' }] });
    fireEvent.change(screen.getByLabelText('Tiêu đề (Tiếng Việt)'), { target: { value: 'Nghe và trả lời (lần 2)' } });
    expect(await within(screen.getByTestId('audio-segments')).findByText(/segment ends after the audio/, {}, { timeout: 60_000 })).toBeInTheDocument();
  });

  it('AC-26: template from listSimulationTemplates, parameters from parameter_schema, metrics from the template, no script field', async () => {
    const { backend } = await renderPublishing('/content-studio/activities/new?type=simulation', { account: tenantUser(), setup: withActivities() });
    const editor = await screen.findByTestId('simulation-editor');
    fireEvent.mouseDown(within(editor).getByRole('combobox', { name: 'Template mô phỏng' }));
    fireEvent.click(await screen.findByTitle('Cân bằng đòn bẩy (physics.lever v1)'));
    expect(await screen.findByLabelText('Chiều dài mỗi cánh tay đòn (m)')).toBeInTheDocument();
    expect(screen.getByLabelText('Khối lượng vật 2 (kg)')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '+ Thêm mục tiêu' }));
    fireEvent.mouseDown(await screen.findByRole('combobox', { name: 'Đại lượng của mục tiêu 1' }));
    expect(await screen.findByTitle('torque_diff (N·m)')).toBeInTheDocument();
    expect(screen.getByText('Mô phỏng chỉ dùng template đóng gói sẵn; không có ô nhập script hay mã.')).toBeInTheDocument();
    expect(editor.querySelector('textarea')).toBeNull();
    expect(within(editor).queryByLabelText(/script|code|mã nguồn/i)).toBeNull();
    expect(backend.calls('GET', '/cms/learning/simulation-templates').length).toBeGreaterThan(0);
  });

  it('AC-26: without learning_content.activity.write the simulation editor is read only', async () => {
    const ID = P(603);
    const setup = withActivities((a) => {
      a.add(ID, [{ status: 'draft', content: simDef }], { tenant_id: TENANT });
    });
    await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(TENANT_CONTENT_PERMS.filter((p) => p !== 'learning_content.activity.write')), setup });
    const editor = await screen.findByTestId('simulation-editor');
    expect(await screen.findByLabelText('Chiều dài mỗi cánh tay đòn (m)')).toBeDisabled();
    expect(within(editor).getByRole('combobox', { name: 'Template mô phỏng' })).toBeDisabled();
    expect(within(editor).queryByRole('button', { name: '+ Thêm mục tiêu' })).toBeNull();
  });

  it('AC-29: menu.content_studio.simulations turned off — no template list in the editor', async () => {
    const ID = P(604);
    const setup = withActivities((a) => {
      a.add(ID, [{ status: 'draft', content: simDef }], { tenant_id: TENANT });
    });
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup, disabledMenus: ['menu.content_studio.simulations'] });
    expect(await screen.findByTestId('simulation-templates-off')).toBeInTheDocument();
    expect(backend.calls('GET', '/cms/learning/simulation-templates')).toHaveLength(0);
  });

  it('AC-26, AC-29: the simulations screen lists the packaged templates (read only)', async () => {
    await renderPublishing('/content-studio/simulations', { account: tenantUser(), setup: withActivities() });
    const table = await screen.findByTestId('simulation-templates');
    expect(await within(table).findByText('physics.lever v1')).toBeInTheDocument();
    expect(within(table).getByText('torque_diff, balanced')).toBeInTheDocument();
  });

  it('AC-29: the simulations screen URL shows "Tính năng đang tắt" when the menu is turned off', async () => {
    await renderPublishing('/content-studio/simulations', { account: tenantUser(), setup: withActivities(), disabledMenus: ['menu.content_studio.simulations'] });
    expect(await screen.findByTestId('feature-disabled')).toBeInTheDocument();
    expect(screen.queryByTestId('simulation-templates')).toBeNull();
  });
});

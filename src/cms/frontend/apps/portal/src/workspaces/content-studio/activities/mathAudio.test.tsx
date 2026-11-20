// F058 T019, T020 (whole portal + MOCK backend, AGENTS R17): F058-AC-08 formulas of the instructions rendered in the
// browser and sent as math_assets, a broken formula and a server 422 MATH_RENDER_FAILED shown on the field (TC-UI-013);
// F058-AC-09 grades 1–3 need the voice-over of the instructions or an exemption before Gửi duyệt, 422 AUDIO_REQUIRED
// shown on the field (TC-CNT-010).
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MockActivities } from '../../../test/mock-activities';
import { P, problem, renderPublishing, stopPortals, TENANTS, tenantUser, type MockPublishing } from '../../../test/publishing-harness';

afterEach(() => {
  stopPortals();
  vi.unstubAllGlobals();
});

const TENANT = TENANTS.thDemoA.tenant_id;
const QV = P(101);
const config = {
  content: { question_source: { kind: 'questions', questions: [{ question_version_id: QV }] }, allowed_question_types: ['single_choice', 'multiple_choice', 'true_false', 'image_based'] },
  evaluation: { strategies: ['auto'], max_score: 10 },
  feedback: { timing: 'immediate' },
  completion_rule: { mode: 'submit' },
};
const def = (extra: Record<string, unknown>) => ({ activity_type: 'quiz', title: { vi: 'Phân số' }, grade_levels: [4], subject_id: P(10), objective_ids: [P(31)], difficulty: 2, content_language: 'vi', config, ...extra });
const holder: { acts?: MockActivities } = {};
const seed = (id: string, extra: Record<string, unknown>) => (m: MockPublishing) => {
  holder.acts = new MockActivities(m);
  holder.acts.add(id, [{ status: 'draft', content: def(extra) }], { tenant_id: TENANT });
};
const btn = (name: string) => screen.queryAllByRole('button', { name }).at(0) ?? null;

async function editTitle(value: string) {
  await waitFor(() => expect(screen.getByLabelText('Tiêu đề (Tiếng Việt)')).toHaveValue('Phân số'), { timeout: 30_000 });
  fireEvent.change(screen.getByLabelText('Tiêu đề (Tiếng Việt)'), { target: { value } });
}

describe('Activity Builder — formulas and voice-over (F058)', () => {
  it('AC-08: formulas of the instructions are sent as math_assets; 422 MATH_RENDER_FAILED turns the field red', async () => {
    const ID = P(311);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: seed(ID, { instructions: { vi: 'Tính \\( \\frac{1}{2} + \\frac{1}{3} \\)' } }) });
    await editTitle('Phân số (mới)');
    expect(screen.getAllByRole('button', { name: '∑ Công thức' }).length).toBeGreaterThanOrEqual(2);
    await waitFor(() => expect(backend.calls('PATCH', `/cms/learning/activities/${ID}`)).toHaveLength(1), { timeout: 60_000 });
    const body = backend.calls('PATCH', `/cms/learning/activities/${ID}`)[0]!.body as { math_assets: { math_id: string; latex: string; svg: string }[] };
    expect(body.math_assets).toHaveLength(1);
    expect(body.math_assets[0]).toMatchObject({ latex: '\\frac{1}{2} + \\frac{1}{3}' });
    expect(body.math_assets[0]!.math_id).toMatch(/^[0-9a-f]{64}$/);
    expect(body.math_assets[0]!.svg).toContain('<svg');
    holder.acts!.nextWrite = problem(422, 'MATH_RENDER_FAILED', { errors: [{ field: '/math_assets/0/svg', code: 'INVALID' }] });
    fireEvent.change(screen.getByLabelText('Tiêu đề (Tiếng Việt)'), { target: { value: 'Phân số (lần 2)' } });
    expect((await screen.findAllByText('Máy chủ từ chối công thức này; hãy sửa trước khi gửi duyệt.', {}, { timeout: 60_000 })).length).toBeGreaterThan(0);
  });

  it('AC-08: a broken formula is shown on its field and blocks saving and Gửi duyệt', async () => {
    const ID = P(312);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: seed(ID, { instructions: { vi: 'Tính \\( \\frac{1}{ \\)' } }) });
    expect((await screen.findAllByText(/Công thức lỗi: \\frac\{1\}\{/, {}, { timeout: 30_000 })).length).toBeGreaterThan(0);
    await waitFor(() => expect(btn('Gửi duyệt')).toBeDisabled());
    await editTitle('Phân số (mới)');
    await new Promise((r) => setTimeout(r, 1500));
    expect(backend.calls('PATCH', `/cms/learning/activities/${ID}`)).toHaveLength(0);
  });

  it('AC-09: grades 1–3 — Gửi duyệt locked without the voice-over; an exemption unlocks it and is saved', async () => {
    const ID = P(313);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: seed(ID, { grade_levels: [2] }) });
    expect(await screen.findByText('Âm thanh đọc hướng dẫn')).toBeInTheDocument();
    expect((await screen.findAllByText('Hoạt động lớp 1–3 thiếu âm thanh đọc hướng dẫn (hoặc lý do miễn).')).length).toBeGreaterThan(0);
    await waitFor(() => expect(btn('Gửi duyệt')).toBeDisabled());
    fireEvent.change(screen.getByLabelText('Lý do miễn âm thanh'), { target: { value: 'Bài nhận biết chữ: đọc to làm lộ đáp án' } });
    await waitFor(() => expect(btn('Gửi duyệt')).toBeEnabled());
    await waitFor(() => expect(backend.calls('PATCH', `/cms/learning/activities/${ID}`).length).toBeGreaterThan(0), { timeout: 60_000 });
    expect(backend.calls('PATCH', `/cms/learning/activities/${ID}`).at(-1)!.body).toMatchObject({ audio_exempt_reason: 'Bài nhận biết chữ: đọc to làm lộ đáp án' });
  });

  it('AC-09: 422 AUDIO_REQUIRED from the server is shown on the voice-over field', async () => {
    const ID = P(314);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: seed(ID, { grade_levels: [1], audio_exempt_reason: 'Nhận biết chữ cái' }) });
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path.endsWith(':transition') ? Promise.resolve(problem(422, 'AUDIO_REQUIRED', { errors: [{ field: '/instructions_audio_ref', code: 'REQUIRED', message: 'Thiếu âm thanh đọc hướng dẫn' }] })) : Promise.resolve(undefined)));
    fireEvent.click(await waitFor(() => {
      const b = btn('Gửi duyệt');
      expect(b).toBeEnabled();
      return b!;
    }));
    const field = (await screen.findByText('Âm thanh đọc hướng dẫn')).closest('.ant-form-item') as HTMLElement;
    expect(await within(field).findByText('Thiếu âm thanh đọc hướng dẫn', {}, { timeout: 30_000 })).toBeInTheDocument();
  });

  it('AC-09: grade 4 has no voice-over field', async () => {
    const ID = P(315);
    await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: seed(ID, {}) });
    await waitFor(() => expect(screen.getByLabelText('Tiêu đề (Tiếng Việt)')).toHaveValue('Phân số'), { timeout: 30_000 });
    expect(screen.queryByText('Âm thanh đọc hướng dẫn')).toBeNull();
  });
});

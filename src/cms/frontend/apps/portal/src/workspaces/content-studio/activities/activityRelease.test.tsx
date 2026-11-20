// F058 T026, T027, T028, T030 (whole portal + MOCK backend, AGENTS R17; Playwright with the real backend pending —
// T031): F058-AC-10 preview of an activity version (createLearningPreviewSession content_type = activity, vi / en, no
// attempt) · AC-11 the student frame has no answer, option feedback, hint or explanation (TC-LRN-012) · AC-12 Gửi
// duyệt / Duyệt / Từ chối with If-Match + Idempotency-Key, reason required, no Duyệt for the author (TC-CMS-001…004) ·
// AC-14 the publish dialog says running attempts keep their version; a double click makes one release (TC-ACT-005).
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MockActivities } from '../../../test/mock-activities';
import type { PVersion } from '../../../test/mock-publishing';
import { P, renderPublishing, stopPortals, TENANT_CONTENT_PERMS, TENANTS, tenantUser, type MockPublishing } from '../../../test/publishing-harness';

afterEach(() => {
  stopPortals();
  vi.unstubAllGlobals();
});

const TENANT = TENANTS.thDemoA.tenant_id;
const ME = '0193a4d2-1111-7000-8000-0000000000e0'; // sub of tenantUser()
const OTHER = P(900);
const QV = P(101);
const rich = (vi: string) => ({ blocks: [{ type: 'paragraph', text: { vi, en: vi } }] });
const content = {
  activity_type: 'quiz',
  title: { vi: 'Luyện bảng nhân 7' },
  grade_levels: [4],
  subject_id: P(10),
  objective_ids: [P(31)],
  difficulty: 2,
  content_language: 'vi',
  config: {
    content: { question_source: { kind: 'questions', questions: [{ question_version_id: QV }] }, allowed_question_types: ['single_choice'] },
    evaluation: { strategies: ['auto'], max_score: 10 },
    feedback: { timing: 'immediate' },
    completion_rule: { mode: 'submit' },
  },
};
type V = Pick<PVersion, 'status' | 'author_id'>;
const seed = (id: string, versions: V[], current?: number) => (m: MockPublishing) => {
  new MockActivities(m).add(id, versions.map((v) => ({ ...v, content })), { tenant_id: TENANT, current });
  m.previewItems = [
    {
      type: 'single_choice',
      render_payload: { options: [{ id: 'o_a', content: rich('56') }, { id: 'o_b', content: rich('54') }] },
      correct: { option_id: 'o_a' },
      feedback: { o_b: { vi: 'Phản hồi chỉ dành cho tác giả' } },
      hints: [rich('Gợi ý chỉ dành cho tác giả')],
      explanation: rich('Lời giải chỉ dành cho tác giả'),
    },
  ];
};
/** learning_content.activity.review gates Duyệt / Từ chối of an activity (ADMIN_PORTAL §3.13); the catalog-derived set lacks it. */
const reviewer = () => tenantUser([...TENANT_CONTENT_PERMS, 'learning_content.activity.review']);
const btn = (name: string) => screen.queryAllByRole('button', { name }).at(0) ?? null;
const findBtn = (name: string) => waitFor(() => {
  const b = btn(name);
  expect(b).not.toBeNull();
  return b!;
}, { timeout: 30_000 });
const transitions = (backend: { requests: { method: string; path: string }[] }) => backend.requests.filter((r) => r.method === 'POST' && r.path.endsWith(':transition'));

describe('Activity Builder — preview and workflow (F058 US2)', () => {
  it('AC-10, AC-11: preview session of the activity version in vi / en, no answer in the student frame, no attempt', async () => {
    const ID = P(321);
    const { backend, mock } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: seed(ID, [{ status: 'published', author_id: OTHER }], 1) });
    fireEvent.click(await screen.findByRole('tab', { name: 'Xem trước' }, { timeout: 30_000 }));
    fireEvent.click(await screen.findByRole('button', { name: 'Xem trước v1' }));
    expect(await screen.findByTestId('preview-vi')).toHaveTextContent('Câu 1 (vi)');
    const panel = screen.getByTestId('preview-panel');
    fireEvent.click(within(panel).getByText('Song song'));
    expect(await screen.findByTestId('preview-en')).toHaveTextContent('Question 1 (en)');
    const sessions = backend.calls('POST', '/cms/learning/preview/sessions').map((s) => s.body as { content_type: string; content_id: string; version: number; display_language: string });
    expect(sessions.every((s) => s.content_type === 'activity' && s.content_id === ID && s.version === 1)).toBe(true);
    expect(sessions.map((s) => s.display_language).sort()).toEqual(['en-US', 'vi-VN']);
    for (const secret of ['Phản hồi chỉ dành cho tác giả', 'Gợi ý chỉ dành cho tác giả', 'Lời giải chỉ dành cho tác giả']) expect(within(panel).queryByText(secret)).toBeNull();
    expect(panel.innerHTML).not.toContain('correct');
    expect(mock.attempts).toBe(0);
    expect(backend.requests.some((r) => /attempts|learning-events|xp/.test(r.path))).toBe(false);
  });

  it('AC-12: Gửi duyệt sends transition submit_review with If-Match and Idempotency-Key', async () => {
    const ID = P(322);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: seed(ID, [{ status: 'draft', author_id: ME }]) });
    fireEvent.click(await findBtn('Gửi duyệt'));
    await waitFor(() => expect(transitions(backend)).toHaveLength(1), { timeout: 30_000 });
    const tr = backend.requests.find((r) => r.method === 'POST' && r.path.endsWith(':transition'))!;
    expect(tr.path).toBe(`/cms/learning/content/activity/${ID}/versions/1:transition`);
    expect(tr.body).toMatchObject({ action: 'submit_review' });
    expect(tr.headers['if-match']).toBeTruthy();
    expect(tr.headers['idempotency-key']).toBeTruthy();
  });

  it('AC-12: a reviewer approves a version of another author', async () => {
    const ID = P(323);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: reviewer(), setup: seed(ID, [{ status: 'in_review', author_id: OTHER }]) });
    fireEvent.click(await findBtn('Duyệt'));
    await waitFor(() => expect(transitions(backend)).toHaveLength(1), { timeout: 30_000 });
    expect(transitions(backend)[0]).toMatchObject({ body: { action: 'approve' } });
  });

  it('AC-12: Từ chối needs a reason, sent as note', async () => {
    const ID = P(324);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: reviewer(), setup: seed(ID, [{ status: 'in_review', author_id: OTHER }]) });
    fireEvent.click(await findBtn('Từ chối'));
    const dialog = await screen.findByRole('dialog', { name: 'Lý do từ chối' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    expect(await within(dialog).findByText('Vui lòng nhập lý do.')).toBeInTheDocument();
    expect(transitions(backend)).toHaveLength(0);
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Câu 3 chưa đúng chương trình' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xác nhận' }));
    await waitFor(() => expect(transitions(backend)).toHaveLength(1), { timeout: 30_000 });
    expect(transitions(backend)[0]).toMatchObject({ body: { action: 'reject', note: 'Câu 3 chưa đúng chương trình' } });
  });

  it('AC-12: the author of the version has no Duyệt / Từ chối', async () => {
    const ID = P(325);
    await renderPublishing(`/content-studio/activities/${ID}`, { account: reviewer(), setup: seed(ID, [{ status: 'in_review', author_id: ME }]) });
    await waitFor(() => expect(screen.getByLabelText('Tiêu đề (Tiếng Việt)')).toHaveValue('Luyện bảng nhân 7'), { timeout: 30_000 });
    await new Promise((r) => setTimeout(r, 500));
    expect(btn('Duyệt')).toBeNull();
    expect(btn('Từ chối')).toBeNull();
  });

  it('AC-14: publishing over a published version says running attempts keep it; a double click makes one release', async () => {
    const ID = P(326);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: seed(ID, [{ status: 'published', author_id: OTHER }, { status: 'approved', author_id: OTHER }], 1) });
    fireEvent.click(await findBtn('Xuất bản ngay'));
    const dialog = await screen.findByRole('dialog', { name: 'Xuất bản phiên bản v2' });
    expect(within(dialog).getByTestId('publish-notice')).toHaveTextContent('Học sinh đang làm giữ phiên bản cũ tới khi xong lượt; lượt mới nhận phiên bản mới.');
    const ok = within(dialog).getByRole('button', { name: 'Xuất bản ngay' });
    fireEvent.click(ok);
    fireEvent.click(ok);
    const publishes = () => backend.requests.filter((r) => r.method === 'POST' && r.path.endsWith(':publish'));
    await waitFor(() => expect(publishes().length).toBeGreaterThan(0), { timeout: 30_000 });
    await new Promise((r) => setTimeout(r, 500));
    const keys = new Set(publishes().map((r) => r.headers['idempotency-key']));
    expect(keys.size).toBe(1);
    expect([...keys][0]).toBeTruthy();
  });
});

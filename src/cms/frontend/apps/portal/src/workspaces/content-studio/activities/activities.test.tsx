// F058 (whole portal + MOCK backend, AGENTS R17; Playwright with the real backend pending — T031, T040, T047):
//   AC-01 type step · AC-02, AC-03, AC-28 create a quiz with picked questions (Idempotency-Key) · AC-05, AC-06
//   autosave with If-Match, 422 ACTIVITY_CONFIG_INVALID mapped to the field · AC-07, AC-14 published = read only,
//   "Tạo bản nháp từ v1" · AC-13 type in preview locks publishing · AC-15…AC-18 Video Manager rules · AC-22…AC-25
//   system re-evaluation with impact preview and status · AC-26 no create without learning_content.activity.write.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MockActivities } from '../../../test/mock-activities';
import { P, problem, renderPublishing, stopPortals, TENANT_CONTENT_PERMS, TENANTS, tenantUser, type MockPublishing } from '../../../test/publishing-harness';

afterEach(() => {
  stopPortals();
  vi.unstubAllGlobals();
});

const TENANT = TENANTS.thDemoA.tenant_id;
const LO = P(31);
const meta = { title: { vi: 'Luyện bảng nhân 3' }, grade_levels: [3], subject_id: P(10), objective_ids: [LO], difficulty: 2, content_language: 'vi' };
const quizConfig = (questions: unknown[]) => ({
  content: { question_source: { kind: 'questions', questions }, allowed_question_types: ['single_choice', 'multiple_choice', 'true_false', 'image_based'] },
  evaluation: { strategies: ['auto'], max_score: 10 },
  feedback: { timing: 'immediate' },
  completion_rule: { mode: 'submit' },
});
const QV = P(101);

function withActivities(setup?: (a: MockActivities, m: MockPublishing) => void) {
  let acts!: MockActivities;
  return {
    setup: (m: MockPublishing) => {
      acts = new MockActivities(m);
      setup?.(acts, m);
    },
    get acts() {
      return acts;
    },
  };
}

describe('Activity Builder (F058)', () => {
  it('AC-01: step 1 offers the registered types with an editor only', async () => {
    const h = withActivities();
    await renderPublishing('/content-studio/activities/new', { account: tenantUser(), setup: h.setup });
    const step = await screen.findByTestId('activity-type-step');
    expect(await within(step).findByRole('button', { name: 'Bài trắc nghiệm' })).toBeInTheDocument();
    expect(within(step).getByRole('button', { name: 'Bài tập tương tác' })).toBeInTheDocument();
    expect(within(step).getByRole('button', { name: 'Bài học video' })).toBeInTheDocument();
    expect(within(step).queryByRole('button', { name: /Speaking|Minigame/i })).toBeNull();
    // picking a type opens its editor on the same route (found by the E2E T040: it used to open /activities/new/new)
    fireEvent.click(within(step).getByRole('button', { name: 'Bài học video' }));
    expect(await screen.findByTestId('video-lesson-editor')).toBeInTheDocument();
  });

  it('AC-26: no create without learning_content.activity.write', async () => {
    const readOnly = withActivities();
    await renderPublishing('/content-studio/activities', { account: tenantUser(TENANT_CONTENT_PERMS.filter((p) => p !== 'learning_content.activity.write')), setup: readOnly.setup });
    expect(await screen.findByText('Bạn chỉ có quyền xem hoạt động học tập.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Tạo hoạt động' })).toBeNull();
  });

  it('AC-02, AC-03, AC-28: create a quiz — a numeric question is refused, the picked one is sent with an Idempotency-Key', async () => {
    const h = withActivities((a) => {
      a.questions = [
        { content_id: P(201), content_version_id: QV, version: 1, type: 'single_choice', status: 'published', title: { vi: 'Câu 1' } },
        { content_id: P(202), content_version_id: P(102), version: 1, type: 'numeric', status: 'published', title: { vi: 'Câu 2' } },
      ];
    });
    const { backend } = await renderPublishing('/content-studio/activities/new?type=quiz', { account: tenantUser(), setup: h.setup });
    fireEvent.change(await screen.findByLabelText('Tiêu đề (Tiếng Việt)'), { target: { value: 'Luyện bảng nhân 3' } });
    fireEvent.click(screen.getByRole('button', { name: '+ Chọn câu hỏi đã xuất bản' }));
    const dialog = await screen.findByRole('dialog');
    const adds = await within(dialog).findAllByRole('button', { name: /Thêm|Add/ });
    fireEvent.click(adds[1]!);
    expect(await screen.findByText(/không dùng được trong hoạt động này/)).toBeInTheDocument();
    fireEvent.click(adds[0]!);
    expect(await screen.findByText('1/100 câu')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Lưu' }));
    await waitFor(() => expect(backend.calls('POST', '/cms/learning/activities')).toHaveLength(1));
    const post = backend.calls('POST', '/cms/learning/activities')[0]!;
    expect(post.headers['idempotency-key']).toBeTruthy();
    const body = post.body as { activity_type: string; title: unknown; config: { content: { question_source: { questions: { question_version_id: string }[] } } } };
    expect(body.activity_type).toBe('quiz');
    expect(body.title).toEqual({ vi: 'Luyện bảng nhân 3' });
    expect(body.config.content.question_source.questions.map((q) => q.question_version_id)).toEqual([QV]);
  });

  it('AC-05, AC-06: autosave PATCH with If-Match; 422 ACTIVITY_CONFIG_INVALID shows the field and blocks submit', async () => {
    const ID = P(301);
    const h = withActivities((a) => {
      a.add(ID, [{ status: 'draft', content: { activity_type: 'quiz', ...meta, config: quizConfig([{ question_version_id: QV }]) } }], { tenant_id: TENANT });
    });
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: h.setup });
    // the draft must be in the form before typing, or loading it would reset the edit
    await waitFor(() => expect(screen.getByLabelText('Tiêu đề (Tiếng Việt)')).toHaveValue('Luyện bảng nhân 3'), { timeout: 30_000 });
    fireEvent.change(screen.getByLabelText('Tiêu đề (Tiếng Việt)'), { target: { value: 'Luyện bảng nhân 3 (mới)' } });
    await waitFor(() => expect(backend.calls('PATCH', `/cms/learning/activities/${ID}`)).toHaveLength(1), { timeout: 60_000 });
    const patch = backend.calls('PATCH', `/cms/learning/activities/${ID}`)[0]!;
    expect(patch.headers['if-match']).toBe('W/"1"');
    expect((patch.body as { title: unknown; config: unknown }).title).toEqual({ vi: 'Luyện bảng nhân 3 (mới)' });
    h.acts.nextWrite = problem(422, 'ACTIVITY_CONFIG_INVALID', { errors: [{ field: '/config/content/question_source/questions/0', code: 'INVALID', message: 'is not a published question version' }] });
    fireEvent.change(screen.getByLabelText('Tiêu đề (Tiếng Việt)'), { target: { value: 'Luyện bảng nhân 3 (lần 2)' } });
    expect(await screen.findByText(/is not a published question version/, {}, { timeout: 60_000 })).toBeInTheDocument();
  });

  it('AC-07, AC-14: a published version is read only; "Tạo bản nháp từ v1" creates version 2; AC-13 preview type locks publish', async () => {
    const ID = P(302);
    const h = withActivities((a) => {
      a.add(ID, [{ status: 'published', content: { activity_type: 'quiz', ...meta, config: quizConfig([{ question_version_id: QV }]) } }], { tenant_id: TENANT, current: 1 });
      a.types = a.types.map((x) => (x.activity_type === 'quiz' ? { ...x, status: 'preview', min_client_version: '1.4.0' } : x));
    });
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: h.setup });
    expect(await screen.findByText('Phiên bản v1 đã xuất bản nên chỉ đọc. Tạo bản nháp mới để sửa.')).toBeInTheDocument();
    expect(await screen.findByTestId('type-preview')).toHaveTextContent('1.4.0');
    expect(screen.getByLabelText('Tiêu đề (Tiếng Việt)')).toHaveAttribute('readonly');
    fireEvent.click(screen.getAllByRole('button', { name: 'Tạo bản nháp từ v1' })[0]!);
    await waitFor(() => expect(backend.calls('POST', `/cms/learning/activities/${ID}/versions`)).toHaveLength(1));
    expect(backend.calls('POST', `/cms/learning/activities/${ID}/versions`)[0]!.headers['idempotency-key']).toBeTruthy();
  });

  it('AC-15…AC-18: Video Manager — duration from the asset, cue outside the video, speech needs a vi subtitle', async () => {
    const ID = P(303);
    const VIDEO = P(401);
    const h = withActivities((a, m) => {
      m.media.push({ asset_id: VIDEO, kind: 'video', status: 'ready', original_filename: 'vong-tuan-hoan.mp4', duration_ms: 240_000, tenant_id: TENANT, variants: [{ variant: 'mp4_720', url: 'https://cdn.example.invalid/v.mp4' }] });
      a.add(ID, [{ status: 'draft', content: { activity_type: 'video_lesson', ...meta, config: { content: { video_asset_id: VIDEO, chapters: [], subtitles: [], embedded_questions: [{ cue_id: 'q1', at_ms: 300_000, question_version_id: QV }] }, interaction: { allow_seek: 'watched_only' }, evaluation: { strategies: ['completion', 'auto'], max_score: 10 }, feedback: { timing: 'immediate' }, completion_rule: { mode: 'min_watched_ratio', min_watched_ratio: 0.9, require_required_cues: true } } } }], { tenant_id: TENANT });
    });
    await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: h.setup });
    await waitFor(() => expect(screen.getByTestId('video-duration')).toHaveTextContent('240'), { timeout: 30_000 });
    expect((await screen.findAllByText('Cue q1 nằm ngoài thời lượng video.')).length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('checkbox', { name: 'Video có lời (bắt buộc phụ đề tiếng Việt)' }));
    expect(await screen.findByTestId('subtitle-issue')).toHaveTextContent('Video có lời cần phụ đề tiếng Việt.');
    expect(screen.getAllByLabelText('Cho phép tua').length).toBeGreaterThan(0);
  });

  it('AC-22…AC-25: system re-evaluation — impact before confirming, one request, status followed', async () => {
    const ID = P(304);
    const h = withActivities((a) => {
      a.add(ID, [{ status: 'published', content: { activity_type: 'quiz', ...meta, config: quizConfig([{ question_version_id: QV }]) } }], { tenant_id: TENANT, current: 1 });
      a.affected[`${ID.slice(0, -4)}0001`] = 3;
    });
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: h.setup });
    const entry = await screen.findByTestId('regrade-entry');
    fireEvent.click(within(entry).getByRole('button', { name: 'Chấm lại hệ thống…' }));
    const dialog = await screen.findByTestId('regrade-dialog');
    expect(within(dialog).getByText(/Bài làm gốc và evaluation cũ được giữ/)).toBeInTheDocument();
    expect(within(dialog).queryByLabelText(/điểm/i)).toBeNull();
    fireEvent.change(within(dialog).getByLabelText('Lý do (10–1.000 ký tự)'), { target: { value: 'Đáp án câu 1 bị nhập sai' } });
    expect(await screen.findByTestId('regrade-impact')).toHaveTextContent('3 bài làm sẽ được chấm lại.');
    fireEvent.click(screen.getByRole('button', { name: 'Xác nhận chấm lại' }));
    expect(await screen.findByTestId('regrade-status', {}, { timeout: 30_000 })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTestId('regrade-status')).toHaveTextContent('Hoàn tất'), { timeout: 30_000 });
    expect(backend.calls('POST', '/cms/learning/regrade-requests')).toHaveLength(1);
    expect(backend.calls('POST', '/cms/learning/regrade-requests')[0]!.headers['idempotency-key']).toBeTruthy();
  });

  it('AC-22: the re-evaluation entry is hidden without learning_content.re_evaluate', async () => {
    const ID = P(305);
    const noPerm = withActivities((a) => {
      a.add(ID, [{ status: 'published', content: { activity_type: 'quiz', ...meta, config: quizConfig([{ question_version_id: QV }]) } }], { tenant_id: TENANT, current: 1 });
    });
    await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(TENANT_CONTENT_PERMS.filter((p) => p !== 'learning_content.re_evaluate')), setup: noPerm.setup });
    expect(await screen.findByText('Phiên bản v1 đã xuất bản nên chỉ đọc. Tạo bản nháp mới để sửa.')).toBeInTheDocument();
    expect(screen.queryByTestId('regrade-entry')).toBeNull();
  });

  it('AC-27: EduWorld content assigned to the tenant is read only with the "Từ EduWorld" badge', async () => {
    const ID = P(306);
    const h = withActivities((a) => {
      a.add(ID, [{ status: 'published', content: { activity_type: 'quiz', ...meta, config: quizConfig([{ question_version_id: QV }]) } }], { current: 1 });
    });
    await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: h.setup });
    expect((await screen.findAllByText('Từ EduWorld')).length).toBeGreaterThan(0);
    expect(screen.getByLabelText('Tiêu đề (Tiếng Việt)')).toHaveAttribute('readonly');
  });

  it('AC-29: text inputs take maxLength from the contract; AC-30: below 1024 px the builder is read only', async () => {
    const ID = P(307);
    const h = withActivities((a) => {
      a.add(ID, [{ status: 'draft', content: { activity_type: 'quiz', ...meta, config: quizConfig([{ question_version_id: QV }]) } }], { tenant_id: TENANT });
    });
    await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup: h.setup, width: 900 });
    expect(await screen.findByText('Tác vụ này cần màn hình lớn hơn')).toBeInTheDocument();
    const title = await screen.findByLabelText('Tiêu đề (Tiếng Việt)');
    expect(title).toHaveAttribute('readonly');
    expect(Number(title.getAttribute('maxlength'))).toBeGreaterThan(0);
  });
});

// F054 T020, T023 (whole portal + MOCK backend; Playwright pending — OQ F051-9): F054-AC-10 preview of a chosen version
// vi / en / side by side, "Thử làm" scored by the server (submitPreviewResponse) with option feedback, hints one level
// at a time and the explanation — no attempt, no learning event (ADR-0031).
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { P, renderPublishing, stopPortals } from '../../../test/publishing-harness';

afterEach(() => stopPortals());

const QS = P(901);
const rich = (vi: string) => ({ blocks: [{ type: 'paragraph', text: { vi, en: vi } }] });

describe('Xem trước và thử làm (F054-AC-10)', () => {
  it('vi / en / side by side; try-out wrong → feedback + hint 1, next hint, then correct; no attempt', async () => {
    const { backend, mock } = await renderPublishing(`/content-studio/question-sets/${QS}`, {
      setup: (m) => {
        m.add({ id: QS, type: 'question_set', tenant_id: null, title: { vi: 'Bộ thử' }, current: 1, versions: [{ status: 'published', content: { definition: { kind: 'static', purpose: 'exercise', items: [] } } }] });
        m.previewItems = [
          {
            type: 'single_choice',
            render_payload: { options: [{ id: 'o_a', content: rich('56') }, { id: 'o_b', content: rich('54') }] },
            correct: { option_id: 'o_a' },
            feedback: { o_b: { vi: 'Em đã nhân 7 × 8 thành 54?' } },
            hints: [rich('Nhớ bảng nhân 7'), rich('7 × 8 = 7 × 7 + 7')],
            explanation: rich('7 × 8 = 56'),
          },
        ];
      },
    });
    fireEvent.click(await screen.findByRole('tab', { name: 'Xem trước' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Xem trước v1' }));
    expect(await screen.findByTestId('preview-vi')).toHaveTextContent('Câu 1 (vi)');
    const panel = screen.getByTestId('preview-panel');
    fireEvent.click(within(panel).getByText('Song song'));
    expect(await screen.findByTestId('preview-en')).toHaveTextContent('Question 1 (en)');
    expect(screen.getByTestId('preview-vi')).toBeInTheDocument();
    const sessions = backend.calls('POST', '/cms/learning/preview/sessions');
    expect(sessions.map((s) => (s.body as { display_language: string }).display_language).sort()).toEqual(['en-US', 'vi-VN']);
    fireEvent.click(within(panel).getByText('Tiếng Việt'));
    fireEvent.click(within(panel).getByLabelText('Thử làm'));
    const run = await screen.findByTestId('try-out-1');
    fireEvent.click(within(run).getByText('54'));
    fireEvent.click(within(run).getByRole('button', { name: 'Kiểm tra' }));
    const result = await within(run).findByTestId('try-out-result');
    expect(within(result).getByText('Chưa đúng')).toBeInTheDocument();
    expect(within(result).getByText('Em đã nhân 7 × 8 thành 54?')).toBeInTheDocument();
    expect(within(result).getByText('Gợi ý cấp 1')).toBeInTheDocument();
    expect(within(result).queryByText('Gợi ý cấp 2')).toBeNull();
    fireEvent.click(within(result).getByRole('button', { name: 'Xem gợi ý cấp 2' }));
    expect(await within(result).findByText('Gợi ý cấp 2')).toBeInTheDocument();
    fireEvent.click(within(run).getByText('56'));
    fireEvent.click(within(run).getByRole('button', { name: 'Kiểm tra' }));
    await waitFor(() => expect(within(run).getByText('Đúng')).toBeInTheDocument());
    const answers = backend.requests.filter((r) => r.method === 'POST' && r.path.endsWith('/responses'));
    expect(answers.map((a) => a.body)).toEqual([{ position: 1, response: { option_id: 'o_b' } }, { position: 1, response: { option_id: 'o_a' } }]);
    expect(mock.attempts).toBe(0);
    expect(backend.requests.some((r) => /attempts|learning-events|xp/.test(r.path))).toBe(false);
  });
});

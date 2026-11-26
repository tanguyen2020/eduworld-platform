// F108 T025 (US3) — F108-AC-15: list and detail of a challenge / school tournament (status, registrations, check-in,
// bracket with the real names of the own school's students, results) and the organiser actions — publish, register
// on behalf of the audience (grade ± students), confirm the list, generate the bracket, confirm / override a node
// (mandatory reason + 2-step confirmation), withdraw an entry, cancel — each calling exactly its F107 operationId.
// Backend = MOCK (challenges-mock.ts).
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { accountWith, SCHOOL_PERMS } from '../../../../test/school-lms-mock';
import { AUDIENCE, CH_IDS, confirmedEntries, ENTRY, generatedNodes, NODE, renderChallenges, stopPortals, TENANTS } from './challenges-mock';

afterEach(() => {
  stopPortals();
  cleanup();
});

const DETAIL = `/school-lms/challenges/${CH_IDS.challengeTournament}`;
const TOUR = `/competition/tournaments/${CH_IDS.tournament}`;
const click = (name: string | RegExp) => fireEvent.click(screen.getByRole('button', { name }));

describe('Danh sách thử thách (F108-AC-15)', () => {
  it('cursor table: name, kind, grade, status, schedule; grade filter sent as grade_level', async () => {
    const { backend } = await renderChallenges('/school-lms/challenges');
    const table = await screen.findByTestId('challenge-table');
    expect(await within(table).findByText('Giải Toán khối 3')).toBeInTheDocument();
    expect(within(table).getByText('Đua nhanh khối 2')).toBeInTheDocument();
    expect(within(table).getAllByText('Giải trường').length).toBeGreaterThan(0);
    expect(within(table).getByText('Khối 3')).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Khối' }));
    fireEvent.click(screen.getAllByTitle('Khối 2').filter((e) => e.classList.contains('ant-select-item-option')).at(-1)!);
    await waitFor(() => expect(within(screen.getByTestId('challenge-table')).queryByText('Giải Toán khối 3')).toBeNull());
    expect(backend.calls('GET', '/lms/challenges').some((r) => new URLSearchParams(r.search).get('grade_level') === '2')).toBe(true);
    expect(backend.calls('GET', '/lms/challenges').every((r) => new URLSearchParams(r.search).get('limit') === '50' && !r.search.includes('class_id'))).toBe(true);
  });
});

describe('Chi tiết giải trường (F108-AC-15)', () => {
  it('publish → register on behalf of the audience → confirm → generate bracket: each its operation; real names shown', async () => {
    const { backend } = await renderChallenges(DETAIL);
    const detail = await screen.findByTestId('challenge-detail');
    expect(within(detail).getByTestId('challenge-audience')).toHaveTextContent('3');
    expect(await screen.findByTestId('tournament-status')).toHaveTextContent('Nháp');

    click('Công bố giải');
    await waitFor(() => expect(screen.getByTestId('tournament-status')).toHaveTextContent('Đang mở đăng ký'));
    const publish = backend.calls('POST', `${TOUR}:publish`);
    expect(publish).toHaveLength(1);
    expect(publish[0]!.headers['idempotency-key']).toBeTruthy();
    expect(publish[0]!.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);

    click('Đăng ký hộ theo đối tượng');
    expect(await screen.findByTestId('register-result')).toHaveTextContent('Đã đăng ký 3 học sinh; bỏ qua 1');
    const reg = backend.calls('POST', `${TOUR}/entries`);
    expect(reg).toHaveLength(1);
    expect(reg[0]!.body).toEqual({});
    expect(reg[0]!.headers['idempotency-key']).toBeTruthy();
    const entries = await screen.findByTestId('entry-table');
    for (const name of ['Nguyễn An', 'Lê Bình', 'Phạm Cường']) expect(await within(entries).findByText(name)).toBeInTheDocument();
    // nickname is shown next to the real name, never instead of it
    expect(within(entries).getByText('Mèo Xanh')).toBeInTheDocument();

    click('Xác nhận danh sách');
    await waitFor(() => expect(backend.calls('POST', `${TOUR}/entries:confirm`)).toHaveLength(1));
    expect(backend.calls('POST', `${TOUR}/entries:confirm`)[0]!.headers['idempotency-key']).toBeTruthy();
    await waitFor(() => expect(within(screen.getByTestId('entry-table')).getAllByText('Đã xác nhận')).toHaveLength(3));

    click('Tạo bracket');
    await waitFor(() => expect(backend.calls('POST', `${TOUR}:generate-bracket`)).toHaveLength(1));
    expect(backend.calls('POST', `${TOUR}:generate-bracket`)[0]!.headers['idempotency-key']).toBeTruthy();
    fireEvent.click(screen.getByRole('tab', { name: 'Bracket' }));
    const node = await screen.findByTestId(`node-${NODE.semi1}`);
    expect(within(node).getByText('Nguyễn An')).toBeInTheDocument();
    expect(within(node).getByText('Lê Bình')).toBeInTheDocument();
    expect(within(screen.getByTestId(`node-${NODE.semi2}`)).getAllByText('Miễn đấu').length).toBeGreaterThan(0);
  });

  it('confirm a node → confirmBracketNode; withdraw an entry → withdrawTournamentEntry', async () => {
    const { backend } = await renderChallenges(DETAIL, {
      setup: (s) => {
        s.tournament.status = 'scheduled';
        s.entries = confirmedEntries();
        s.nodes = generatedNodes();
      },
    });
    fireEvent.click(await screen.findByRole('tab', { name: 'Bracket' }));
    const node = await screen.findByTestId(`node-${NODE.semi1}`);
    fireEvent.click(within(node).getByRole('button', { name: 'Xác nhận kết quả' }));
    await waitFor(() => expect(backend.calls('POST', `${TOUR}/nodes/${NODE.semi1}:confirm`)).toHaveLength(1));
    await waitFor(() => expect(within(screen.getByTestId(`node-${NODE.semi1}`)).getByText('Đã xác nhận')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('tab', { name: 'Đăng ký & check-in' }));
    const row = (await within(screen.getByTestId('entry-table')).findByText('Phạm Cường')).closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: 'Rút' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Đồng ý' }));
    await waitFor(() => expect(backend.calls('DELETE', `${TOUR}/entries/${ENTRY[CH_IDS.studentCuong4]}`)).toHaveLength(1));
  });

  it('override a node: reason mandatory (button locked without it), then a second confirmation; sends winner + reason with an Idempotency-Key', async () => {
    const { backend, state } = await renderChallenges(DETAIL, {
      setup: (s) => {
        s.tournament.status = 'in_progress';
        s.entries = confirmedEntries();
        s.nodes = generatedNodes();
      },
    });
    fireEvent.click(await screen.findByRole('tab', { name: 'Bracket' }));
    const node = await screen.findByTestId(`node-${NODE.semi1}`);
    fireEvent.click(within(node).getByRole('button', { name: 'Điều chỉnh kết quả' }));
    const dialog = await screen.findByRole('dialog', { name: 'Điều chỉnh kết quả trận' });
    fireEvent.click(within(dialog).getByRole('radio', { name: /Lê Bình/ }));
    const go = within(dialog).getByRole('button', { name: 'Tiếp' });
    expect(go).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: '   ' } });
    expect(go).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Trọng tài xác nhận lỗi kết nối' } });
    expect(go).not.toBeDisabled();
    fireEvent.click(go);
    // step 2: summary, nothing sent yet
    expect(await within(dialog).findByText(/Đổi người thắng của trận thành Lê Bình/)).toBeInTheDocument();
    expect(backend.calls('POST', `${TOUR}/nodes/${NODE.semi1}:override`)).toHaveLength(0);
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xác nhận đổi kết quả' }));
    await waitFor(() => expect(backend.calls('POST', `${TOUR}/nodes/${NODE.semi1}:override`)).toHaveLength(1));
    const call = backend.calls('POST', `${TOUR}/nodes/${NODE.semi1}:override`)[0]!;
    expect(call.body).toEqual({ winner_entry_id: ENTRY[CH_IDS.studentBinh], reason: 'Trọng tài xác nhận lỗi kết nối' });
    expect(call.headers['idempotency-key']).toBeTruthy();
    await waitFor(() => expect(state.nodes[0]!.status).toBe('overridden'));
    await waitFor(() => expect(within(screen.getByTestId(`node-${NODE.semi1}`)).getByText('Đã điều chỉnh')).toBeInTheDocument());
  });

  it('results tab: entries by final rank with real names', async () => {
    await renderChallenges(DETAIL, {
      setup: (s) => {
        s.tournament.status = 'completed';
        s.entries = confirmedEntries().map((e, i) => ({ ...e, final_rank: [2, 3, 1][i], status: i === 2 ? 'champion' : 'eliminated' }));
        s.nodes = generatedNodes();
      },
    });
    fireEvent.click(await screen.findByRole('tab', { name: 'Kết quả' }));
    const results = await screen.findByTestId('result-table');
    await waitFor(() => expect(within(results).getAllByRole('row').length).toBeGreaterThan(3));
    const rows = within(results).getAllByRole('row').slice(1);
    expect(rows.map((r) => r.textContent)).toEqual([expect.stringContaining('Phạm Cường'), expect.stringContaining('Nguyễn An'), expect.stringContaining('Lê Bình')]);
    expect(AUDIENCE).toHaveLength(3);
  });

  it('cancel tournament → cancelTournament with reason; cancel a timed-quiz challenge → cancelChallenge with reason', async () => {
    const { backend } = await renderChallenges(DETAIL);
    fireEvent.click(await screen.findByRole('button', { name: 'Hủy giải' }));
    const dialog = await screen.findByRole('dialog', { name: 'Hủy giải đấu' });
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Trùng lịch kiểm tra' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Hủy giải' }));
    await waitFor(() => expect(backend.calls('POST', `${TOUR}:cancel`)).toHaveLength(1));
    expect(backend.calls('POST', `${TOUR}:cancel`)[0]!.body).toEqual({ reason: 'Trùng lịch kiểm tra' });
    stopPortals();
    cleanup();

    const r2 = await renderChallenges(`/school-lms/challenges/${CH_IDS.challengeQuiz}`);
    expect(await screen.findByTestId('challenge-detail')).toBeInTheDocument();
    expect(screen.queryByTestId('tournament-panel')).toBeNull();
    click('Hủy thử thách');
    const d2 = await screen.findByRole('dialog', { name: 'Hủy thử thách' });
    fireEvent.change(within(d2).getByLabelText('Lý do'), { target: { value: 'Đổi lịch' } });
    fireEvent.click(within(d2).getByRole('button', { name: 'Hủy thử thách' }));
    await waitFor(() => expect(r2.backend.calls('POST', `/lms/challenges/${CH_IDS.challengeQuiz}:cancel`)).toHaveLength(1));
    expect(r2.backend.calls('POST', `/lms/challenges/${CH_IDS.challengeQuiz}:cancel`)[0]!.body).toEqual({ reason: 'Đổi lịch' });
    expect(r2.backend.calls('POST', `/lms/challenges/${CH_IDS.challengeQuiz}:cancel`)[0]!.headers['idempotency-key']).toBeTruthy();
    await waitFor(() => expect(screen.getByTestId('challenge-status')).toHaveTextContent('Đã hủy'));
  });

  it('another tenant’s challenge → Không tìm thấy', async () => {
    await renderChallenges(DETAIL, { account: accountWith('ad.truong.b', TENANTS.thcsDemoB, SCHOOL_PERMS) });
    expect(await screen.findByTestId('school-lms-not-found')).toBeInTheDocument();
  });
});

// F108 T024 (US3) — F108-AC-14 (TC-CMS-AUTHZ-07): the wizard lists only the templates assigned to the tenant
// (listChallengeTemplates), builds the form only from the template's `allowed_params` (no rule, reward or chat input),
// picks the audience in one school of the scope — one grade within the template's `grade_levels`, students of the
// same school to add / leave out (≤ 500 each, searched only in that school, exclude wins), no class picker (ADR-0095)
// — shows the list sizes (the server computes the effective students), sends createChallenge with one
// Idempotency-Key per click (double click = one challenge), and shows 403 RESOURCE_NOT_ASSIGNED / 422
// VALIDATION_FAILED on their field. Backend = MOCK (challenges-mock.ts).
import { act, cleanup, fireEvent, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { tenantFilterStore } from '../../../../shell/tenantFilterStore';
import { CH_IDS, IDS, problem, renderChallenges, stopPortals, TENANTS } from './challenges-mock';
import { fillWizard, next, pickOption, pickStudent, typeDate, waitSchool } from './wizard-helpers';

afterEach(() => {
  stopPortals();
  cleanup();
  tenantFilterStore.set([]);
});

const URL = '/school-lms/challenges/new';

describe('Wizard thử thách khối / giải trường (F108-AC-14)', () => {
  it('templates of the tenant only; the form holds only allowed_params (no rules, rewards, chat)', async () => {
    const { backend } = await renderChallenges(URL);
    fireEvent.mouseDown(await screen.findByRole('combobox', { name: 'Template' }));
    expect(await screen.findByTitle('Giải Toán khối')).toBeInTheDocument();
    expect(screen.getByTitle('Thử thách Timed Quiz')).toBeInTheDocument();
    expect(screen.queryByTitle('Thử thách THCS B')).toBeNull();
    fireEvent.click(screen.getAllByTitle('Giải Toán khối').filter((e) => e.classList.contains('ant-select-item-option')).at(-1)!);
    // fixed reward of the template is read-only text
    expect(await screen.findByText('Huy hiệu Vô địch khối')).toBeInTheDocument();
    next();
    await screen.findByLabelText('Tên thử thách');
    for (const label of ['Số người tham gia tối đa', 'Đóng đăng ký lúc', 'Tự động xác nhận đăng ký']) expect(screen.getByLabelText(label), label).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Cách xếp hạt giống' })).toBeInTheDocument();
    // registration_open_at is not allowed by this template
    expect(screen.queryByLabelText('Mở đăng ký lúc')).toBeNull();
    expect(screen.queryByLabelText(/phần thưởng|luật|điểm|chat/i)).toBeNull();
    expect(screen.getByLabelText('Số người tham gia tối đa')).toBeInTheDocument();
    const calls = backend.calls('GET', '/lms/catalog/challenge-templates');
    expect(calls.length).toBeGreaterThan(0);
    expect(calls.every((r) => r.headers['x-tenant-id'] === TENANTS.thDemoA.tenant_id)).toBe(true);
  });

  it('another template: its own parameter only (unknown key → text box ≤ 255)', async () => {
    await renderChallenges(URL);
    await pickOption('Template', 'Thử thách Timed Quiz');
    next();
    const theme = (await screen.findByLabelText('theme_code')) as HTMLInputElement;
    expect(theme.maxLength).toBe(255);
    expect(screen.queryByLabelText('Số người tham gia tối đa')).toBeNull();
    expect((screen.getByLabelText('Tên thử thách') as HTMLInputElement).maxLength).toBe(80);
  });

  it('audience: one school, one grade within grade_levels ∩ school grades, students searched only in that school; no class picker (ADR-0095)', async () => {
    const { backend } = await renderChallenges(URL);
    await pickOption('Template', 'Giải Toán khối');
    next();
    fireEvent.change(await screen.findByLabelText('Tên thử thách'), { target: { value: 'Giải' } });
    typeDate('Bắt đầu lúc', '2026-12-01 08:00:00');
    next();
    await waitSchool();
    expect(screen.queryByRole('combobox', { name: /Lớp/ })).toBeNull();
    expect(screen.queryByText(/Lớp/)).toBeNull();
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Khối' }));
    // template grade_levels [3, 4]; school A teaches 1–5 → only Khối 3, Khối 4
    const open = () => [...document.querySelectorAll('.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option')].map((e) => e.getAttribute('title'));
    await waitFor(() => expect(open()).toEqual(['Khối 3', 'Khối 4']));
    fireEvent.click(screen.getAllByTitle('Khối 3').filter((e) => e.classList.contains('ant-select-item-option')).at(-1)!);
    await pickStudent('Thêm học sinh (cùng trường)', 4, 'Cường', 'Phạm Cường (hs.cuong.4b)');
    await pickStudent('Loại trừ học sinh', 4, 'Cường', 'Phạm Cường (hs.cuong.4b)');
    expect(screen.getByTestId('exclude-wins')).toHaveTextContent('1 học sinh vừa được thêm vừa bị loại trừ');
    expect(screen.getByTestId('audience-preview')).toHaveTextContent('thêm 1 học sinh');
    expect(screen.getByTestId('audience-preview')).toHaveTextContent('loại trừ 1 học sinh');
    const searches = backend.requests.filter((r) => /^\/tenant\/schools\/[^/]+\/grades\/\d+\/students$/.test(r.path));
    expect(searches.length).toBeGreaterThan(0);
    expect(searches.every((r) => r.path.startsWith(`/tenant/schools/${IDS.schoolA}/`) && r.headers['x-tenant-id'] === TENANTS.thDemoA.tenant_id)).toBe(true);
    expect(backend.requests.some((r) => /\/classes|study-groups/.test(r.path))).toBe(false);
  });

  it('create: body = template, name, school, grade target + students target, exclude list, allowed params; double click = one key, one challenge → detail', async () => {
    const { backend, state } = await renderChallenges(URL);
    await fillWizard();
    const review = screen.getByTestId('challenge-review');
    expect(review).toHaveTextContent('Giải Toán khối 3');
    expect(review).toHaveTextContent('Khối 3');
    const create = screen.getByRole('button', { name: 'Tạo thử thách' });
    act(() => {
      fireEvent.click(create);
      fireEvent.click(create);
    });
    expect(await screen.findByTestId('challenge-detail')).toBeInTheDocument();
    const creates = backend.calls('POST', '/lms/challenges');
    expect(creates.length).toBeGreaterThan(0);
    expect(new Set(creates.map((r) => r.headers['idempotency-key'])).size).toBe(1);
    expect(state.challenges.size).toBe(3);
    const body = creates[0]!.body as Record<string, unknown>;
    expect(body).toMatchObject({ template_id: CH_IDS.tplTournament, name: 'Giải Toán khối 3', school_id: IDS.schoolA, params: { max_entries: 32 } });
    expect(body.targets).toEqual([
      { target_type: 'grade', grade_level: 3 },
      { target_type: 'students', student_ids: [CH_IDS.studentCuong4] },
    ]);
    expect(body.exclude_student_ids).toEqual([CH_IDS.studentBinh]);
    expect(Object.keys(body.params as object)).toEqual(['max_entries']);
    expect(body).not.toHaveProperty('tenant_id');
    expect(body).not.toHaveProperty('class_id');
    // the server-computed audience is shown on the detail page
    expect(await screen.findByTestId('challenge-audience')).toHaveTextContent('3');
  });

  it('403 RESOURCE_NOT_ASSIGNED → back to the template step, message on the template field with trace id', async () => {
    const { backend } = await renderChallenges(URL);
    backend.overrides.unshift((r) => (r.method === 'POST' && r.path === '/lms/challenges' ? problem(403, 'RESOURCE_NOT_ASSIGNED', { detail: 'raw server detail' }) : undefined));
    await fillWizard({ include: false, exclude: false });
    fireEvent.click(screen.getByRole('button', { name: 'Tạo thử thách' }));
    const field = (await screen.findByRole('combobox', { name: 'Template' })).closest('.ant-form-item')!;
    await waitFor(() => expect(field).toHaveClass('ant-form-item-has-error'));
    await waitFor(() => expect(field).toHaveTextContent('Template này không được gán (hoặc vừa bị gỡ) cho đơn vị'));
    expect(screen.getByTestId('wizard-error')).toHaveTextContent('trace-resource_not_assigned');
    expect(screen.queryByText('raw server detail')).toBeNull();
  });

  it('422 VALIDATION_FAILED → each error on its field and step (param, exclude list)', async () => {
    const { backend } = await renderChallenges(URL);
    let first = true;
    backend.overrides.unshift((r) => {
      if (r.method !== 'POST' || r.path !== '/lms/challenges') return undefined;
      if (first) {
        first = false;
        return problem(422, 'VALIDATION_FAILED', { errors: [{ field: '/exclude_student_ids/0', code: 'NOT_IN_SCHOOL' }] });
      }
      return problem(422, 'VALIDATION_FAILED', { errors: [{ field: '/params/max_entries', code: 'OUT_OF_RANGE' }] });
    });
    await fillWizard({ include: false });
    fireEvent.click(screen.getByRole('button', { name: 'Tạo thử thách' }));
    const exclude = await screen.findByRole('combobox', { name: 'Loại trừ học sinh' });
    await waitFor(() => expect(exclude.closest('.ant-form-item')).toHaveClass('ant-form-item-has-error'));
    await waitFor(() => expect(exclude.closest('.ant-form-item')).toHaveTextContent('NOT_IN_SCHOOL'));
    next();
    await screen.findByTestId('challenge-review');
    fireEvent.click(screen.getByRole('button', { name: 'Tạo thử thách' }));
    const max = await screen.findByLabelText('Số người tham gia tối đa');
    await waitFor(() => expect(max.closest('.ant-form-item')).toHaveClass('ant-form-item-has-error'));
    await waitFor(() => expect(max.closest('.ant-form-item')).toHaveTextContent('OUT_OF_RANGE'));
  });

  it('required fields are checked before leaving a step (shape only)', async () => {
    const { backend } = await renderChallenges(URL);
    await screen.findByRole('combobox', { name: 'Template' });
    next();
    expect(await screen.findByText('Trường này bắt buộc.')).toBeInTheDocument();
    await pickOption('Template', 'Giải Toán khối');
    next();
    await screen.findByLabelText('Tên thử thách');
    next();
    expect((await screen.findAllByText('Trường này bắt buộc.')).length).toBe(2);
    expect(backend.calls('POST', '/lms/challenges')).toHaveLength(0);
  });
});

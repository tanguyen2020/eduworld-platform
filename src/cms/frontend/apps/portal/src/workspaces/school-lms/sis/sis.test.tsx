// F052 T021, T022, T023 — F052-AC-08, F052-AC-09, F052-AC-10, F052-AC-11, F052-AC-25, F052-AC-27 (TC-AUTH-074): CSV
// import through the screen (presigned PUT, progress counts), one import per click, conflicts listed with the
// suggested action, resolve with mandatory reason and the same Idempotency-Key on retry; read-only users get no write
// button; small screens get the "cần màn hình lớn hơn" notice. Backend = MOCK.
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountWith, IDS, problem, READ_ONLY_PERMS, renderSchoolLms, stopPortals, TENANTS } from '../../../test/school-lms-harness';

afterEach(() => {
  stopPortals();
  vi.unstubAllGlobals();
});

const pickFile = async (name = 'hoc_sinh.csv') => {
  const input = (await waitFor(() => {
    const el = document.querySelector('input[type="file"]');
    if (!el) throw new Error('no file input');
    return el;
  })) as HTMLInputElement;
  fireEvent.change(input, { target: { files: [new File(['student_external_id,username\nHS001,hs.an.3a\n'], name, { type: 'text/csv' })] } });
};

describe('Nhập CSV', () => {
  it('create (Idempotency-Key) → PUT presigned URL without Authorization → progress counts (F052-AC-08, F052-AC-09)', async () => {
    const storage = vi.fn(async () => new Response(null, { status: 200 }));
    vi.stubGlobal('fetch', storage);
    const { backend } = await renderSchoolLms('/school-lms/sis-sync');
    await pickFile();
    const start = await screen.findByRole('button', { name: 'Tải lên và nhập' });
    await waitFor(() => expect(start).toBeEnabled());
    fireEvent.click(start);
    fireEvent.click(start);
    const progress = await screen.findByTestId('sis-import-progress');
    expect(within(progress).getByText('Hoàn tất, có xung đột')).toBeInTheDocument();
    expect(within(progress).getAllByText('3').length).toBeGreaterThan(0);
    const creates = backend.calls('POST', `/schools/${IDS.schoolA}/sis-imports`);
    expect(creates).toHaveLength(1);
    expect(creates[0]!.headers['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
    expect(creates[0]!.body).toMatchObject({ file_name: 'hoc_sinh.csv', entity_kinds: ['students'], dry_run: true });
    expect((creates[0]!.body as { sha256: string }).sha256).toMatch(/^[0-9a-f]{64}$/);
    const [url, init] = storage.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe('https://storage.test/upload/sis.csv?sig=mock');
    expect(JSON.stringify(init.headers)).not.toMatch(/authorization|x-tenant-id/i);
  });

  it('a non-CSV file is refused before any call (F052-AC-08)', async () => {
    const { backend } = await renderSchoolLms('/school-lms/sis-sync');
    await pickFile('hoc_sinh.xlsx');
    expect(await screen.findByText('Chỉ nhận file .csv.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Tải lên và nhập' })).toBeDisabled();
    expect(backend.calls('POST', `/schools/${IDS.schoolA}/sis-imports`)).toHaveLength(0);
  });

  it('import form needs a large screen (F052-AC-27)', async () => {
    await renderSchoolLms('/school-lms/sis-sync', { width: 390 });
    expect(await screen.findByText(/màn hình lớn hơn/)).toBeInTheDocument();
  });
});

describe('Xung đột đồng bộ', () => {
  it('lists conflicts with suggested action; resolve needs a reason; retry reuses the Idempotency-Key (F052-AC-10, F052-AC-11, TC-AUTH-074)', async () => {
    const { backend, state } = await renderSchoolLms('/school-lms/sis-sync');
    const path = `/schools/${IDS.schoolA}/reconciliation-issues/${IDS.issueDup}:resolve`;
    let fail = true;
    backend.overrides.unshift((r) => (r.path === path && fail ? ((fail = false), problem(409, 'CONFLICT')) : undefined));
    fireEvent.click(await screen.findByRole('tab', { name: 'Xung đột đồng bộ' }));
    expect(await screen.findByText('Trùng tên đăng nhập/email', { selector: 'td' })).toBeInTheDocument();
    expect(screen.getByText('Sửa dữ liệu trong SIS/CSV rồi nhập lại.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Xử lý' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xử lý' }));
    expect(await within(dialog).findByText('Trường này bắt buộc.')).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Đã đổi username trong SIS' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xử lý' }));
    await waitFor(() => expect(backend.calls('POST', path)).toHaveLength(1));
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xử lý' }));
    await waitFor(() => expect(state.issueStatus).toBe('resolved'));
    const keys = backend.calls('POST', path).map((r) => r.headers['idempotency-key']);
    expect(keys).toHaveLength(2);
    expect(keys[0]).toBe(keys[1]);
    expect(backend.calls('POST', path)[1]!.body).toEqual({ resolution: 'fixed_in_source', reason: 'Đã đổi username trong SIS' });
  });

  it('F052 T061 (OQ F045-7): "Đề nghị gộp tài khoản" on a duplicate issue sends merge_requested + reason; the issue stays open, marked as proposed', async () => {
    const { backend, state } = await renderSchoolLms('/school-lms/sis-sync');
    const path = `/schools/${IDS.schoolA}/reconciliation-issues/${IDS.issueDup}:resolve`;
    fireEvent.click(await screen.findByRole('tab', { name: 'Xung đột đồng bộ' }));
    expect(await screen.findByText('Trùng tên đăng nhập/email', { selector: 'td' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Xử lý' }));
    const dialog = await screen.findByRole('dialog');
    fireEvent.click(within(dialog).getByLabelText('Đề nghị gộp tài khoản'));
    expect(within(dialog).getByText(/Quản trị hệ thống sẽ gộp/)).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText('Lý do'), { target: { value: 'Cùng một học sinh có hai tài khoản' } });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Xử lý' }));
    await waitFor(() => expect(backend.calls('POST', path)).toHaveLength(1));
    expect(backend.calls('POST', path)[0]!.body).toEqual({ resolution: 'merge_requested', reason: 'Cùng một học sinh có hai tài khoản' });
    expect(backend.calls('POST', path)[0]!.headers['idempotency-key']).toBeTruthy();
    expect(state.issueStatus).toBe('open');
    expect(await screen.findByText('Đã đề nghị gộp')).toBeInTheDocument();
    // Proposed already: the dialog no longer offers the merge request.
    fireEvent.click(screen.getByRole('button', { name: 'Xử lý' }));
    const again = await screen.findByRole('dialog');
    expect(within(again).queryByLabelText('Đề nghị gộp tài khoản')).toBeNull();
  });

  it('a read-only user has no Tải lên / Xử lý button (F052-AC-25)', async () => {
    await renderSchoolLms('/school-lms/sis-sync', { account: accountWith('qlcm.truong.a', TENANTS.thDemoA, READ_ONLY_PERMS) });
    fireEvent.click(await screen.findByRole('tab', { name: 'Xung đột đồng bộ' }));
    expect(await screen.findByText('Trùng tên đăng nhập/email', { selector: 'td' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Xử lý' })).toBeNull();
    fireEvent.click(screen.getByRole('tab', { name: 'Nhập CSV' }));
    expect(screen.queryByRole('button', { name: 'Tải lên và nhập' })).toBeNull();
  });
});

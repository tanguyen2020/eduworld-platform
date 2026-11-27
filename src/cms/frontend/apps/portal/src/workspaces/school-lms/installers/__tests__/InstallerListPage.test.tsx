// F134 T032 — "Tải bộ cài" of workspace C (F134-AC-17, TC-CMS-WS-01; ADR-0101 mục 7–8). Backend = MOCK
// (test/installers-mock.ts + the F052 MOCK, test only, AGENTS R17).
// F134-AC-17: table of platform, version, grade bands, size, SHA-256 (copy button), download link; `device.cfg` guide
// from the translation file; 503 shows the message of its `code` and no link; a Tenant Admin of a `unit` tenant picks a
// school (every page of listTenantSchools, searchable) before the API is called; "Mở bản Web" uses web_client_url
// (hidden when null) with a WebGL 2 check.
import { render, screen, waitFor, within } from '@testing-library/react';
import { I18nextProvider } from 'react-i18next';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createI18n } from '../../../../i18n';
import { INST, installersState, renderInstallers } from '../../../../test/installers-mock';
import { problem, stopPortals, TENANTS } from '../../../../test/portal-harness';
import { IDS } from '../../../../test/school-lms-mock';
import { formatSize } from '../format';
import { WebClientCard } from '../WebClientCard';
import { detectWebGl2 } from '../webgl';

afterEach(() => {
  stopPortals();
  vi.restoreAllMocks();
});

const URL = '/school-lms/installers';
const calls = (backend: { calls: (m: string, p: string) => { search: string; headers: Record<string, string> }[] }) => backend.calls('GET', '/tenant/client-installers');

describe('Tải bộ cài (F134-AC-17)', () => {
  it('only school of the scope used directly: rows with platform, version, grade bands, size, SHA-256, download link; device.cfg guide', async () => {
    const { backend } = await renderInstallers(URL);
    const page = await screen.findByTestId('installers-page');
    expect(within(page).getByRole('heading', { level: 1, name: 'Bộ cài game cho trường' })).toBeInTheDocument();
    const download = await within(page).findByRole('link', { name: 'Tải bộ cài 1.4.0 cho Khối 1–3' });
    expect(download).toHaveAttribute('href', INST.url);
    const req = calls(backend)[0]!;
    expect(new URLSearchParams(req.search).get('school_id')).toBe(IDS.schoolA);
    expect(req.headers['x-tenant-id']).toBe(TENANTS.thDemoA.tenant_id);
    // no school picker for a scope with one school
    expect(within(page).queryByRole('combobox', { name: 'Trường' })).toBeNull();
    const row = download.closest('tr')!;
    expect(within(row).getByText('Android (APK)')).toBeInTheDocument();
    expect(within(row).getByText('1.4.0')).toBeInTheDocument();
    expect(within(row).getByText('Khối 1–3')).toBeInTheDocument();
    expect(within(row).getByText('Android 8.0')).toBeInTheDocument();
    expect(within(row).getByText(formatSize(159_822_643, 'vi-VN'))).toBeInTheDocument();
    expect(within(row).getByTestId('installer-sha256')).toHaveTextContent(INST.sha);
    expect(within(page).getByText('Khối 4–5')).toBeInTheDocument();
    expect(within(page).getAllByRole('link').filter((a) => a.getAttribute('href') === INST.url2)).toHaveLength(1);
    // device.cfg guide for Android shared tablets (shared_device, lan_cache_url) from the translation file
    const guide = within(page).getByTestId('device-cfg-guide');
    expect(within(guide).getByText('shared_device')).toBeInTheDocument();
    expect(within(guide).getByText('lan_cache_url')).toBeInTheDocument();
    expect(within(guide).getByText('Chỉ áp dụng cho thiết bị Android. Bản Web không đọc device.cfg.')).toBeInTheDocument();
    // no Windows installer (ADR-0101)
    expect(page.textContent).not.toMatch(/Windows \(|\.exe|\.msi/);
  });

  it('copy button puts the SHA-256 on the clipboard', async () => {
    const { user } = await renderInstallers(URL);
    const row = (await screen.findByRole('link', { name: 'Tải bộ cài 1.4.0 cho Khối 1–3' })).closest('tr')!;
    await user.click(within(row).getByRole('button', { name: 'Sao chép SHA-256' }));
    expect(await screen.findByText('Đã sao chép SHA-256.')).toBeInTheDocument();
    await expect(navigator.clipboard.readText()).resolves.toBe(INST.sha);
  });

  it('503 UNAVAILABLE (index not loadable / bad signature): message of the code, trace id, no download or Web link', async () => {
    const state = installersState();
    state.answer = () => problem(503, 'UNAVAILABLE');
    await renderInstallers(URL, { state });
    const error = await screen.findByTestId('installers-error', {}, { timeout: 60_000 });
    expect(error).toHaveTextContent('Chưa tải được danh sách bộ cài');
    expect(error).toHaveTextContent('Mã truy vết: trace-unavailable');
    const page = screen.getByTestId('installers-page');
    expect(within(page).queryAllByRole('link').filter((a) => /example\.test/.test(a.getAttribute('href') ?? ''))).toHaveLength(0);
    expect(within(page).queryByTestId('web-client-card')).toBeNull();
    expect(within(page).queryByRole('button', { name: 'Sao chép SHA-256' })).toBeNull();
  });

  it('404 (school outside the scope): "not found" message, no link', async () => {
    const state = installersState();
    state.answer = () => problem(404, 'NOT_FOUND');
    await renderInstallers(URL, { state });
    expect(await screen.findByTestId('installers-error')).toHaveTextContent('Không tìm thấy trường này trong phạm vi của bạn.');
    expect(screen.queryByTestId('web-client-card')).toBeNull();
  });

  it('Tenant Admin of a unit tenant must pick a school first (every page, searchable) before the API is called', async () => {
    const { backend, user } = await renderInstallers(URL, { as: 'ad.donvi' });
    expect(await screen.findByTestId('installers-pick-school')).toBeInTheDocument();
    await waitFor(() => expect(backend.calls('GET', '/tenant/schools').some((r) => new URLSearchParams(r.search).get('cursor') === 'p2')).toBe(true));
    expect(calls(backend)).toHaveLength(0);
    const picker = screen.getByRole('combobox', { name: 'Trường' });
    await user.click(picker);
    await user.type(picker, 'don vi 2');
    await user.click((await screen.findAllByTitle('TH Đơn vị 2')).at(-1)!);
    await waitFor(() => expect(calls(backend)).toHaveLength(1));
    expect(new URLSearchParams(calls(backend)[0]!.search).get('school_id')).toBe(INST.schoolU2);
    expect(calls(backend)[0]!.headers['x-tenant-id']).toBe(TENANTS.pgdDemo.tenant_id);
    expect(await screen.findByRole('link', { name: 'Tải bộ cài 1.4.0 cho Khối 1–3' })).toBeInTheDocument();
    expect(screen.queryByTestId('installers-pick-school')).toBeNull();
  });

  it('"Mở bản Web" card: web_client_url in a new tab with the WebGL 2 check', async () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(((id: string) => (id === 'webgl2' ? ({} as RenderingContext) : null)) as never);
    await renderInstallers(URL);
    const card = await screen.findByTestId('web-client-card');
    const open = within(card).getByRole('link', { name: 'Mở bản Web' });
    expect(open).toHaveAttribute('href', INST.webUrl);
    expect(open).toHaveAttribute('target', '_blank');
    expect(open.getAttribute('rel')).toContain('noopener');
    expect(within(card).getByTestId('webgl2-check')).toHaveTextContent('Trình duyệt này hỗ trợ WebGL 2');
  });

  it('"Mở bản Web" card hidden when web_client_url is null (environment not configured)', async () => {
    const state = installersState();
    state.list = { ...state.list, web_client_url: null };
    await renderInstallers(URL, { state });
    expect(await screen.findByRole('link', { name: 'Tải bộ cài 1.4.0 cho Khối 1–3' })).toBeInTheDocument();
    expect(screen.queryByTestId('web-client-card')).toBeNull();
  });
});

describe('WebGL 2 check (ADR-0101 mục 8)', () => {
  it('detectWebGl2: context → supported; null or exception → unsupported', () => {
    const doc = (getContext: () => unknown) => ({ createElement: () => ({ getContext }) }) as unknown as Document;
    expect(detectWebGl2(doc(() => ({})))).toBe('supported');
    expect(detectWebGl2(doc(() => null))).toBe('unsupported');
    expect(
      detectWebGl2(
        doc(() => {
          throw new Error('blocked');
        }),
      ),
    ).toBe('unsupported');
  });

  it('WebClientCard shows the unsupported message when the browser has no WebGL 2', async () => {
    const i18n = await createI18n('vi-VN');
    render(
      <I18nextProvider i18n={i18n}>
        <WebClientCard url={INST.webUrl} detect={() => 'unsupported'} />
      </I18nextProvider>,
    );
    expect(screen.getByTestId('webgl2-check')).toHaveTextContent('Trình duyệt này không hỗ trợ WebGL 2');
  });
});

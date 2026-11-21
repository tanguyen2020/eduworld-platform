# Quickstart: F051 — Portal shell: đăng nhập, chọn tenant sau đăng nhập, menu theo quyền, bộ lọc tenant

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F004, F012, F050.
- Lệnh (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make up-app`; `make portal-catalog`; `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm dev` → `http://localhost:5173` (dev server proxy `/api/v1` cùng origin để cookie `__Secure-ew_portal_rt`/`__Host-ew_portal_csrf` hoạt động [Confirmed]).
- Tài khoản demo (mật khẩu seed local): `ad.truong.a` (school_admin, 1 tenant), `qlcm.truong.a` (academic_manager, 2 tenant), `sa.admin` (system_admin), `cm.noidung` (content_admin), `hs.an.3a` (học sinh).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Mở portal chưa đăng nhập | Trang `/login` của portal (không request tới Keycloak), chỉ ô tên đăng nhập/email và mật khẩu (không ô mã xác thực — ADR-0094), không có bước chọn trường | F051-AC-01 |
| 2 | Đăng nhập `ad.truong.a` bằng email chữ hoa | Vào thẳng `th-demo-a`; DevTools Application không có token trong storage | F051-AC-02, F051-AC-03 |
| 3 | Đăng nhập `qlcm.truong.a` | Màn "Chọn đơn vị làm việc" 2 tenant; chọn `th-demo-a` không gọi lại `portalLogin`/không token mới; request sau có `X-Tenant-Id` | F051-AC-03, F051-AC-05, F051-AC-06 |
| 4 | Đăng nhập `hs.an.3a` | `portalLogin` → `403 PORTAL_ACCESS_STAFF_ONLY`, thông điệp chặn học sinh; nếu tắt điều kiện role (test env) → API trả cùng mã, đăng xuất | F051-AC-04 |
| 5 | `qlcm.truong.a`: network log, mở `/game-ops` | Chỉ chunk `school-lms`, `content-studio`; `/game-ops` → trang 403 | F051-AC-09, F051-AC-10, F051-AC-11 |
| 6 | `ad.donvi` thu hồi một UserRole của `qlcm.truong.a` trên `pgd-demo` | Menu đổi trong ≤ `principal_cache_ttl` + 1 chu kỳ | F051-AC-12 |
| 7 | `sa.admin` với token `auth_time` quá 8 giờ (test env) gọi workspace D | Hộp đăng nhập lại (mật khẩu + mã), thử lại request, giữ đúng URL | F051-AC-13 |
| 8 | Tắt route gateway nội bộ, `sa.admin` mở workspace D | "Cần kết nối mạng quản trị (VPN)" | F051-AC-14 |
| 9 | Gọi API trả `code` lạ (MSW trong Storybook) | Thông điệp chung + `trace_id` | F051-AC-15 |
| 10 | Đổi ngôn ngữ en, theme tối, compact; thu nhỏ < 768 px | Chuỗi đổi ngay; theme đổi; sidebar thành drawer, bộ lọc tenant trong ☰ | F051-AC-16, F051-AC-17, F051-AC-19 |
| 11 | Chạy axe trên shell 3 breakpoint | 0 lỗi serious/critical | F051-AC-18 |
| 12 | Hết phiên (đặt idle ngắn ở realm test) | Hộp "Phiên làm việc đã hết", đăng nhập lại về đúng URL | F051-AC-07 |
| 13 | Đang ở `/school-lms/...` với tenant `th-demo-a`, bấm F5; đóng tab rồi mở lại sau 5 phút | Vào lại đúng URL và tenant, không hiện trang đăng nhập; network không có token trong `localStorage`/`sessionStorage` | F051-AC-28 |
| 14 | Không thao tác quá 30 phút (đặt ngưỡng ngắn ở test env) rồi tải lại; đặt `auth_time` cũ hơn 10 giờ (test realm) | Về trang đăng nhập Keycloak; sau đăng nhập về đúng URL | F051-AC-29 |
| 13b | Đăng xuất | `POST /auth/portal/logout` kèm `X-EW-CSRF`, cookie bị xóa; Back không mở lại dữ liệu | F051-AC-08 |
| 14b | `qlcm.truong.a` có bản nháp chưa lưu, đổi sang `pgd-demo` (không token mới); gửi lại request header cũ bằng DevTools | Cảnh báo bản nháp; menu mới; request header cũ → `403 TENANT_MISMATCH` → về màn chọn đơn vị | F051-AC-20, F051-AC-21 |
| 15 | `sa.admin` chọn bộ lọc `th-demo-a` + `pgd-demo`, mở danh sách câu hỏi | Query có `tenant_ids` lặp, không header; cột Tenant | F051-AC-22, F051-AC-23 |
| 16 | `sa.admin` mở màn chỉ nhận header (ví dụ Cài đặt tenant) | Yêu cầu chọn đúng một tenant | F051-AC-24 |
| 17 | `sa.admin` mở form tạo (bất kỳ) | Trường "Tenant" bắt buộc, không có "tất cả tenant" | F051-AC-25 |
| 18 | `ad.truong.a` mở danh sách | Không có bộ lọc/cột Tenant; không có `tenant_ids` | F051-AC-26 |
| 19 | Dán chuỗi dài hơn `maxLength` vào ô tìm tenant | Bị cắt/chặn tại giới hạn | F051-AC-27 |
| 20 | `/login`: `sa.admin` thiếu mã / mã sai / mật khẩu sai; rồi đúng; 11 lần sai/phút | Cùng một thông điệp chung, ô mật khẩu và mã bị xóa; vào được; lần 11 hiện thời gian chờ | F051-AC-30 |
| 21 | Tài khoản còn `UPDATE_PASSWORD` đăng nhập → đổi mật khẩu yếu rồi hợp lệ | Màn thiết lập, lỗi policy theo field, quay lại `/login` (không còn màn "Kiểm tra email" — ADR-0094) | F051-AC-31 |
| 22 | `/forgot-password` với email `ad.truong.a` và email lạ; mở link từ Mailpit; dùng lại link | Cùng thông điệp; token biến khỏi URL, đặt mật khẩu → `/login`; link cũ → "không còn hiệu lực" | F051-AC-32 |
| 23 | DevTools: `document.cookie`, Application storage; 3 request song song khi access token hết hạn; mở hai tab rồi F5 cả hai | Không thấy refresh token; một lời gọi `refreshPortalToken` có `X-EW-CSRF`, request được thử lại; cả hai tab còn phiên | F051-AC-02, F051-AC-33 |
| 24 | Sửa `specs/portal/menu-catalog.yaml` thử (thêm menu không component) rồi `make portal-catalog` + test; `git diff` file sinh | Test fail "thiếu component"; CI phát hiện file sinh lệch | F051-AC-34 |
| 25 | `sa.admin` (F015) tắt `menu.school_lms.assignments` cho `th-demo-a`; `ad.truong.a` chờ làm mới quyền, mở URL menu đó | Menu biến mất; URL → "Tính năng đang tắt", không gọi API | F051-AC-35 |
| 26 | Tắt `action.school_lms.assignments.publish`; mở chi tiết bài giao | Nút "Xuất bản" ẩn (hoặc khóa có tooltip ở `mode="disable"`) | F051-AC-36 |
| 27 | Tắt menu trong lúc `ad.truong.a` đang ở màn đó rồi bấm tải lại danh sách | Thông báo "Tính năng đang tắt" + `trace_id`, không đăng xuất, menu biến mất | F051-AC-37 |

## 3. Lệnh test tự động

```bash
make web-lint
make web-test                       # Vitest unit/component
cd src/cms/frontend && pnpm --filter portal test:e2e -- --grep @F051   # Playwright, backend local thật
make storybook
```

## 4. Bằng chứng cần nộp

- Kết quả `make web-test`, Playwright (số pass/fail), báo cáo axe; HAR network kịch bản 5 và 23 (không có token trong storage, có `X-EW-CSRF`); ảnh chụp 3 breakpoint sáng/tối; `trace_id` của kịch bản 14 và 27.

## 5. Bằng chứng (2026-11-19, local — Windows, Node 20.19, pnpm 10.34.9, Chromium headless shell của Playwright 1.64.1)

Backend F012 chưa chạy được (binary chưa nối — OQ F010-2) nên mọi kịch bản §2 được kiểm tự động bằng **MOCK backend hợp đồng** `apps/portal/src/test/mock-backend.ts` (trả lời `/auth/portal/*`, `/me/*`, `/auth/me`, `/admin/tenants` theo `specs/openapi/eduworld-api.v1.yaml`, mô phỏng cookie refresh HttpOnly và cookie CSRF, 6 tài khoản demo). **Không phải tích hợp thật**; E2E Playwright `@F051` với backend local và kiểm realm (T052) còn mở (OQ F051-9).

| Tiêu chí | Bằng chứng tự động (Vitest, `src/**`) | Kết quả |
|---|---|---|
| AC-01, AC-30 | `auth/LoginPage.test.tsx` (trang `/login` đủ ô, chỉ một request tới gateway portal, một `portalLogin` gồm `username`/`password`/`device_id`/`locale` (không `otp` — ADR-0094), lỗi chung xóa mật khẩu, `429` → "đợi 30 giây") | 8/8 pass |
| AC-02 | `auth/tokenStore.test.ts` (không token/mật khẩu/mã/ticket trong storage hay cookie, chỉ ghi `ew.portal.deviceId`; gọi `/auth/portal/*` `credentials: include`, không `Authorization`/`X-Tenant-Id`), `sessionLifecycle.test.tsx` (F5 không lưu token) | 7/7, pass |
| AC-03, AC-04 | `session/sessionMachine.test.ts` (định tuyến thuần), `LoginPage.test.tsx` (1 tenant, nhiều tenant + `returnUrl`, không quyền, học sinh), `sessionLifecycle.test.tsx` › staff only | 9/9, pass |
| AC-05, AC-06, AC-27 | `session/tenantSession.test.tsx` › tenant selection (cùng `Authorization`, không `portalLogin`/refresh mới, `X-Tenant-Id`, nhớ lựa chọn, ô tìm `maxlength=100`); `shared/api/withTenantScope.test.ts` (header phiên thắng mọi header, bỏ `tenant_ids`) | 3/3, 7/7 pass |
| AC-07, AC-08 | `auth/sessionLifecycle.test.tsx` › session expired (hộp "Phiên làm việc đã hết" → `/login` → về đúng URL), sign-out online + lỗi mạng (`X-EW-CSRF`, xóa token/cache/tenant tab) | pass |
| AC-09, AC-10, AC-11 | `app/router.test.tsx` (chỉ import chunk `school-lms`; `/game-ops` của `qlcm.truong.a` → 403, không import chunk platform, không gọi API màn), `permissions/menuRegistry.test.ts`; `pnpm build` + `check-chunks` | 5/5, 7/7 pass; 5 chunk `workspace-*` |
| AC-12 | `permissions/catalogMenus.test.tsx` (chu kỳ = `principalCacheTtlSeconds`, focus `always`, `403 PERMISSION_DENIED` → tải lại ngay) | pass |
| AC-13, AC-14, AC-15 | `shell/errors.test.tsx` (hộp đăng nhập lại, thử lại request với token mới, giữ URL; hủy → `/login?returnUrl=`; gateway nội bộ lỗi mạng → banner VPN; `ProblemMessage` theo `code` + `trace_id`, mã lạ → chung) | 4/4 pass |
| AC-16, AC-17, AC-18, AC-19 | `shell/layout.test.tsx` (desktop/tablet/mobile, drawer ☰, bộ chọn tenant trong ☰ < 768 px, vùng chạm 44 px, đổi theme/compact/ngôn ngữ không tải lại, skip link + landmark), `i18n/keys.test.ts` (nhãn danh mục vi/en) | 5/5, 9/9 pass |
| AC-20, AC-21 | `session/tenantSession.test.tsx` › switching tenant (cảnh báo bản nháp, không request nào mang header cũ, cache tenant cũ bị xóa, về trang tổng quan), tenant access changed (`TENANT_MISMATCH`, `TENANT_ACCESS_DISABLED`) | 3/3 pass |
| AC-22..AC-26 | `shell/TenantFilter.test.tsx` (`sa.admin` thấy bộ lọc, `listTenants` qua gateway nội bộ, lưu `sessionStorage`, ô tìm cắt ở 100; `ad.truong.a`, `cm.noidung` không thấy, không `tenant_ids`), `withTenantScope.test.ts` | 4/4, 7/7 pass |
| AC-28, AC-29 | `auth/sessionLifecycle.test.tsx` (F5 khôi phục URL + tenant tab qua `refreshPortalToken` + `X-EW-CSRF`; phiên mất → `/login?returnUrl=`; idle > 30 phút → không refresh, `portalLogout`, `/login`; `localStorage` bị chặn vẫn đăng nhập được) | pass |
| AC-31, AC-32 | `auth/accountRecovery.test.tsx` (lỗi policy theo field, màn kiểm tra email, ticket hết hạn; quên mật khẩu cùng thông điệp; token xóa khỏi URL, không vào storage/history; link hết hiệu lực; `<meta name="referrer" content="no-referrer">`) | 11/11 pass |
| AC-33 | `auth/refreshScheduler.test.ts` (single-flight, `X-EW-CSRF`, tab thua cuộc đua xoay vòng đọc lại cookie và thử lại 1 lần, Web Locks, làm mới chủ động khi còn tương tác; apiClient: 3 request `TOKEN_EXPIRED` → 1 refresh, mỗi request thử lại 1 lần) | 6/6 pass |
| AC-34..AC-37 | `scripts/gen-menu-catalog.test.mjs`, `permissions/catalogMenus.test.tsx` (file sinh khớp YAML, mỗi khóa menu có `screens`, không khai báo menu tay; menu tắt ẩn + URL → "Tính năng đang tắt" không mở màn; `ActionGate` ẩn/khóa; `403 FEATURE_DISABLED` → thông báo `feature_key` + `trace_id`, tải lại quyền, không đăng xuất); `pnpm gen:check` | 2/2, 8/8 pass |

Tổng: `pnpm lint` 0 lỗi, `pnpm typecheck` 0 lỗi, `pnpm gen:check` khớp (OpenAPI + danh mục), `pnpm test` (2 worker) **34 file / 228 test pass** (830 s), `pnpm build` OK (5 chunk workspace, không chunk nào trong closure tĩnh của entry; không có mã MOCK trong `dist`). Storybook: `pnpm storybook:build` OK, `STORYBOOK_A11Y_PORT=6017 node tooling/storybook-a11y.mjs` (axe WCAG 2.1 A/AA) **27/27 pass** (6 component khung × Light/Dark/Compact + 9 story ops-ui) — lượt đầu 3 fail vì placeholder "Tất cả tenant" 1,83:1 → token `colorTextPlaceholder` (OQ F051-12); cổng 6007 bị một http-server cũ chiếm nên chạy cổng 6017 (tooling nay báo lỗi khi cổng bận thay vì kiểm nhầm bản build cũ). Visual: `pnpm test:visual` **55/55 pass** (36 baseline `win32` mới cho 18 story khung portal × zoom 100%/200%, 18 baseline ops-ui không đổi; một lượt bị timeout do CPU, chạy lại pass). Liên repo: `node scripts/specs/check-features.mjs` 0 lỗi, `node scripts/specs/check-portal-catalog.mjs` 0 lỗi, `bash scripts/ci/test-web-rules.sh` 3/3 pass, `design`: `node checks/run-all.mjs` OK (tokens@0.3.0, 188/188 cặp tương phản AA).

Chưa làm/không tự động: kịch bản 2–8, 12–14, 20–23 với Keycloak + backend thật, HAR network, ảnh 3 breakpoint toàn trang, kịch bản 6 (thu hồi UserRole thật) — chờ F004/F012 chạy được (OQ F051-9).

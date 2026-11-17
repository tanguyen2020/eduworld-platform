# Quickstart: F051 — Portal shell: đăng nhập, chọn tenant sau đăng nhập, menu theo quyền, bộ lọc tenant

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F004, F012, F050.
- Lệnh (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make up-app`; `make portal-catalog`; `cd web && pnpm install && pnpm gen:api && pnpm dev` → `http://localhost:5173` (dev server proxy `/api/v1` cùng origin để cookie `__Secure-ew_portal_rt`/`__Host-ew_portal_csrf` hoạt động [Proposed]).
- Tài khoản demo (mật khẩu seed local): `ad.truong.a` (school_admin, 1 tenant), `qlcm.truong.a` (academic_manager, 2 tenant), `sa.admin` (system_admin), `cm.noidung` (content_admin), `hs.an.3a` (học sinh).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Mở portal chưa đăng nhập | Trang `/login` của portal (không request tới Keycloak), có ô mã xác thực 6 số, không có bước chọn trường | F051-AC-01 |
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
| 21 | Tài khoản còn `UPDATE_PASSWORD` đăng nhập → đổi mật khẩu yếu rồi hợp lệ; tài khoản platform chưa có OTP | Màn thiết lập, lỗi policy theo field, quay lại `/login`; màn "Kiểm tra email" | F051-AC-31 |
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
cd web && pnpm --filter portal test:e2e -- --grep @F051   # Playwright, backend local thật
make storybook
```

## 4. Bằng chứng cần nộp

- Kết quả `make web-test`, Playwright (số pass/fail), báo cáo axe; HAR network kịch bản 5 và 23 (không có token trong storage, có `X-EW-CSRF`); ảnh chụp 3 breakpoint sáng/tối; `trace_id` của kịch bản 14 và 27.

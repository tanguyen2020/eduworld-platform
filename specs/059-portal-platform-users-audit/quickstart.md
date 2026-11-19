# Quickstart: F059 — Workspace D — người dùng/role platform, catalog permission, audit toàn hệ thống, login events liên tenant, feature flags, màn tích hợp

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F012, F015, F057, F014, F016 (lock), F011 (SIS connection), F050, F051.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản: `sa.admin` (MFA tắt ở local; bật OTP seed để thử step-up), `cm.noidung`, `cm.duyet`, `sm.antoan`, `ad.truong.a`, `ad.donvi`, `qlcm.truong.a`, học sinh `hs.an.3a`, `hs.locked`.
- Dữ liệu: `hs.an.3a` đã vào game trên 2 thiết bị (seed hoặc chạy F061) để có `end_reason = replaced_by_other_device`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `sa.admin` mở Permission catalog, lọc `scope_kind = platform` | Nhóm theo `group_code`, có `catalog_version`, cột role chứa permission; không nút sửa | F059-AC-01 |
| 2 | Role: lọc 2 tenant, loại `tenant` | Role dùng chung + role tùy biến của 2 tenant, nhãn "Hệ thống", số người | F059-AC-02 |
| 3 | Tạo role platform `liveops_lead` (3 permission `game.*`) | `201`; picker role `tenant` không có `platform.*`/`game.*` | F059-AC-03 |
| 4 | Gọi `createPlatformRole` (curl) role `tenant` chứa `game.quest.write`; mở `system_admin` | `422 PERMISSION_NOT_ALLOWED_FOR_SCOPE` hiển thị theo `code`; role hệ thống chỉ có "Nhân bản" | F059-AC-04 |
| 5 | Xóa `liveops_lead` khi còn người dùng; thu hồi rồi xóa | `409` hiển thị; sau thu hồi xóa thành công có lý do | F059-AC-05 |
| 6 | UserRole: lọc theo `cm.duyet` | Cột phạm vi, tenant, hiệu lực, nguồn, người gán | F059-AC-06 |
| 7 | Gán `liveops_lead` cho `cm.duyet`; gán `tenant_admin` scope tenant cho nhân sự `pgd-demo` | Network: lần 1 không `X-Tenant-Id`; lần 2 `X-Tenant-Id` = `pgd-demo`, chỉ role `tenant` | F059-AC-07 |
| 8 | Gán role platform cho `ad.truong.a` (curl); gửi lại cùng `Idempotency-Key` | `422` theo `code`; retry không tạo bản ghi thứ hai | F059-AC-08 |
| 9 | Thu hồi UserRole vừa gán có lý do | Biến mất khỏi danh sách; người bị thu hồi mất menu ≤ 60 s | F059-AC-09 |
| 10 | Gán `safety_moderator` không nhập ngày đào tạo | Form chặn; sau delta request có `training_completed_at` | F059-AC-10 |
| 11 | Người dùng: tìm `qlcm` | Thấy 2 tenant membership, trạng thái; học sinh `hs.an.3a` nhãn "SIS", không nút gán role | F059-AC-11 |
| 12 | Tạo nhân sự platform email trùng `ad.truong.a` | `409 EMAIL_TAKEN` + gợi ý gán role cho tài khoản sẵn có | F059-AC-12 |
| 13 | Khóa `hs.an.3a` có lý do | `lockUser` `204`; trạng thái `locked` | F059-AC-13 |
| 14 | Mở chi tiết `qlcm.truong.a` | UserRole theo tenant; không mật khẩu/token/IP | F059-AC-14 |
| 15 | Nhật ký toàn hệ thống: 24 giờ, action `user_role.changed`, 2 tenant | Dòng của bước 7, 9; ngăn chi tiết có `changes`, `trace_id` | F059-AC-15 |
| 16 | Nhật ký phân quyền | Chỉ `role.*`, `user_role.*` có trước/sau, lý do | F059-AC-16 |
| 17 | Chọn khoảng `from`–`to` 2 năm | `422` hiển thị giới hạn; không có nút sửa/xóa | F059-AC-17 |
| 18 | Đăng nhập game: lọc `th-demo-a`, học sinh `hs.an.3a`, 7 ngày | Có `replaced_by_other_device`, nền tảng, thiết bị rút gọn | F059-AC-18 |
| 19 | Kiểm DOM/network màn đăng nhập game | Không có IP; phân trang cursor | F059-AC-19 |
| 20 | Feature flag: bật ghi đè cho `th-demo-a`, lưu có lý do | `200`, version tăng | F059-AC-20 |
| 21 | Hai tab cùng sửa một flag | Tab sau nhận `412` → tải lại, báo "Đã có người sửa" | F059-AC-21 |
| 22 | Tích hợp: xem Keycloak, APISIX, tab SIS | Issuer, realm, client; nhóm route; connection + lần sync gần nhất | F059-AC-22 |
| 23 | Quét response/DOM màn Tích hợp | Không có secret/credential; không ô nhập | F059-AC-23 |
| 24 | `ad.truong.a`, `cm.noidung`, `sm.antoan` mở `/portal/platform-admin/roles`, `/portal/platform-admin/audit` | Trang 403; không tải chunk D | F059-AC-24 |
| 25 | Token không MFA; tắt gateway nội bộ | Step-up MFA; banner "Cần kết nối mạng quản trị (VPN)" | F059-AC-25 |
| 26 | Chặn mạng khi gán UserRole rồi thử lại | Hai request cùng `Idempotency-Key`; một UserRole | F059-AC-26 |
| 27 | Dán chuỗi dài vào mã role, lý do, email | Cắt theo `maxLength`, có bộ đếm | F059-AC-27 |
| 28 | `sa.admin` tắt mặc định platform `menu.platform_admin.student_activity`, tải lại portal | Menu "Hoạt động đăng nhập game" biến mất; `listLoginEventsCrossTenant` → `403 FEATURE_DISABLED`; Người dùng & phân quyền, Nhật ký, Bật/tắt menu vẫn hiện (khóa, không tắt được) | F059-AC-28 |

## 3. Lệnh test tự động

```bash
cd src/cms/frontend
pnpm --filter portal test -- src/workspaces/platform-admin src/shared/authz
pnpm --filter portal exec playwright test e2e/platform
make web-lint
make web-test
```

## 4. Bằng chứng cần nộp

- Kết quả Vitest/Playwright (số pass/fail), network log kịch bản 7, 24 (header, chunk), `trace_id` kịch bản 3, 7, 13, kết quả quét secret kịch bản 23, ảnh màn hình sáng/tối.

### Bằng chứng 2026-11-20 (triển khai, mức integration)

Chạy trong `src/cms/frontend` (tối đa 2 worker), commit `b75d83a`. Backend là MOCK theo contract (`src/test/mock-backend.ts` + `src/test/platform-backend.ts` + `src/test/platform-access-backend.ts`); Playwright với backend thật, axe/ảnh sáng-tối, network log và `trace_id` của gateway thật chờ môi trường compose (OQ F059-9). Phần contract chưa có (`listPlatformUsers`, `createPlatformUser`, `getIntegrationOverview`, lọc tiền tố action) theo OQ F059-1…3.

| Lệnh | Kết quả |
|---|---|
| `pnpm lint` · `pnpm typecheck` · `pnpm gen:check` | xanh (trước và sau rebase lên `origin/main`) |
| `npx vitest run --maxWorkers=2` (toàn bộ web, sau rebase) | 77 file, 562 test: 561 pass, 1 lỗi do click khi dropdown còn `pointer-events: none` dưới tải CPU (`Users.test.tsx`) → sửa helper chọn option, chạy lại `Users.test.tsx` 4/4 pass |
| Test F059 (`__tests__/AccessControlRoles`, `UserRoles`, `Users`, `AuditLogs`, `LoginEventsFlags`, `IntegrationsGuard`, `shared/authz/groupPermissions.test.ts`, `limits.test.ts`) | 7 file mới, 41 test + 23 test mới trong `limits.test.ts` (22 giới hạn + CONFIG_DEFAULTS), pass |
| `pnpm build` (+ `check-chunks`) | 5 chunk workspace, chunk `workspace-platform-admin` không do entry tải |
| `node scripts/specs/check-features.mjs` | 93 feature, 0 lỗi, 0 cảnh báo |
| `node scripts/specs/check-portal-catalog.mjs` | 0 lỗi |

| Kịch bản | Test |
|---|---|
| 1 | `AccessControlRoles.test.tsx` › Permission catalog (nhóm `group_code`, `catalog_version`, role chứa, lọc phạm vi, không nút ghi) |
| 2–5 | `AccessControlRoles.test.tsx` (không `X-Tenant-Id`, "Hệ thống"/"Dùng chung", role tenant chỉ xem; tạo + `Idempotency-Key`, picker theo `scope_kind`; `422 PERMISSION_NOT_ALLOWED_FOR_SCOPE` giữ form, thử lại giữ khóa; sửa `If-Match` + lý do, `412`/`403` theo code; `404` đóng editor; xóa `409` khi còn người dùng, `If-Match` + `reason`; nhân bản role hệ thống) |
| 6–10, 26 | `UserRoles.test.tsx` (cột, lọc `user_id`/`tenant_ids[]`; nhân sự platform → role platform, không header, mất mạng → thử lại cùng `Idempotency-Key`, `409`; nhân sự tenant → tenant đích, role của tenant, scope `school` + id, `X-Tenant-Id`; `safety_moderator` bắt buộc ngày đào tạo, `422 ROLE_NOT_FOUND`; thu hồi có lý do + header tenant của UserRole) |
| 11–14 | `Users.test.tsx` (`listPlatformUsers`: chỉ nhân sự platform khi chưa chọn tenant, `tenant_ids[]` + cột "Thuộc tenant", `q`/loại/`status`, nhãn SIS, học sinh chỉ đọc; gán role từ dòng chỉ tenant thành viên — OQ F059-5; ngăn chi tiết UserRole mọi tenant, không mật khẩu/token/IP; khóa/mở khóa/vô hiệu hóa có lý do; tạo nhân sự trường `409 EMAIL_TAKEN` + gợi ý; tạo nhân sự platform `createPlatformUser` không `X-Tenant-Id`, `409 USERNAME_TAKEN` + gợi ý — OQ F059-1) |
| 15–17 | `AuditLogs.test.tsx` (24 giờ mặc định, lọc vào URL, `tenant_ids[]`, `changes` escape, `trace_id`; nhật ký phân quyền một truy vấn `action_prefix=role&action_prefix=user_role` — OQ F059-3; `422` giữ bộ lọc + giới hạn 93 ngày) |
| 18–19 | `LoginEventsFlags.test.tsx` (7 ngày, nhãn `end_reason`, cursor "Tải thêm", không IP) |
| 20–21 | `LoginEventsFlags.test.tsx` (mặc định platform + `If-Match` + lý do, mục bảo vệ không tắt được, ghi đè tenant `X-Tenant-Id`, `412` tải lại + "Đã có người sửa") |
| 22–23 | `IntegrationsGuard.test.tsx` (tab SIS: connection, trường, lần sync; trường ngoài schema không vào DOM, không ô nhập; Keycloak/APISIX từ `getIntegrationOverview`, nhóm route "Chưa có dữ liệu" — OQ F059-2, F059-12) |
| 24–25, 28 | `IntegrationsGuard.test.tsx` (403 + không tải chunk D cho `ad.truong.a`/`cm.noidung`/`sm.antoan` ở cả 5 URL; menu ẩn khi thiếu `platform.audit.read`; `PERMISSION_DENIED` + `trace_id`; MFA step-up; menu tắt → "Tính năng đang tắt", mục bảo vệ còn; action tắt ẩn nút; `403 FEATURE_DISABLED` tải lại quyền) |
| 27 | `limits.test.ts` (giới hạn = OpenAPI, `principal_cache_ttl`/`audit_query_max_range_days` = CONFIG_DEFAULTS) + `maxlength`/bộ đếm trong test màn |

### Bằng chứng 2026-11-20 (portal bật màn chờ backend — OQ F057-2/3/4, F059-1/2/3/5)

Chạy trong `src/cms/frontend` (tối đa 2 worker), sau rebase lên `origin/main` (`7e0a59a`). Backend là MOCK theo contract mới (`src/test/platform-backend.ts`, `src/test/platform-access-backend.ts`); backend thật chờ compose (OQ F057-10, F059-9).

| Lệnh | Kết quả |
|---|---|
| `pnpm lint` · `pnpm typecheck` · `pnpm gen:check` | xanh |
| `pnpm build` (+ `check-chunks`) | 5 chunk workspace, không chunk nào do entry tải |
| `npx vitest run --maxWorkers=2` (toàn bộ web) | 77 file, 575 test: 573 pass, 2 lỗi → sửa: `groupPermissions.test.ts` (kỳ vọng cũ `role.created`, đổi theo `action_prefix`) và `TrashPage.test.tsx` (click khi lớp loading còn `pointer-events: none` dưới tải CPU → click thường); chạy lại 2 file: 16/16 pass (thêm 1 test `memberTenantsOf`) |
| Test thay đổi (`SystemSettings`, `TenantListPage`, `Catalog`, `Users`, `AuditLogs`, `IntegrationsGuard`, `limits.test.ts`) | 7 file, 102 test pass (Users 8/8 sau khi tách test ngăn chi tiết) |
| `node scripts/specs/check-features.mjs` · `node scripts/specs/check-portal-catalog.mjs` | 93 feature, 0 lỗi, 0 cảnh báo · 0 lỗi |

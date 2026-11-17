# Quickstart: F059 — Workspace D — người dùng/role platform, catalog permission, audit toàn hệ thống, login events liên tenant, feature flags, màn tích hợp

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F012, F015, F057, F014, F016 (lock), F011 (SIS connection), F050, F051.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; `cd web && pnpm install && pnpm gen:api && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
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
cd web
pnpm --filter portal test -- src/workspaces/platform-admin src/shared/authz
pnpm --filter portal exec playwright test e2e/platform
make web-lint
make web-test
```

## 4. Bằng chứng cần nộp

- Kết quả Vitest/Playwright (số pass/fail), network log kịch bản 7, 24 (header, chunk), `trace_id` kịch bản 3, 7, 13, kết quả quét secret kịch bản 23, ảnh màn hình sáng/tối.

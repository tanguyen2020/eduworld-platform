# Quickstart: F012 — Phân quyền Permission/Role/UserRole và phiên portal (`/me/*`, `principals:resolve`)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F010 (và F002, F004 của M-C0).
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `ad.truong.a`, `ad.truong.b` (School Admin), `qlcm.truong.a` (Academic Manager 2 tenant), `ad.donvi` (Tenant Admin), `sa.admin` (System Admin), `cm.noidung` (Content Admin), `hs.an.3a` (học sinh).
- Token portal: `POST /api/v1/auth/portal/login` (form riêng của portal, ADR-0091; local `sa.admin` dùng OTP seed theo F004 T043); lời gọi `resolvePrincipal` giữa module chạy trong tiến trình qua facade `identity` (không token).
- Tài khoản thử thiết lập lần đầu: nhân sự còn `UPDATE_PASSWORD` (F004 T043); hộp thư local (Mailpit theo LOCAL_DEVELOPMENT) để xem email đặt lại mật khẩu/đăng ký OTP.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Khởi động module identity với catalog thêm `game.quest.write` vào `school_admin` (nhánh test) | Tiến trình dừng với lỗi `ValidateCatalog` | F012-AC-02 |
| 2 | `ad.truong.a` đăng nhập portal: `listMyTenants`, `getMyPermissions`, `getMe` (không gọi `createAuthSession`) | 1 tenant; menu B, C; không có `game.*`, `platform.*`; audit đăng nhập | F012-AC-04, F012-AC-08, F012-AC-10 |
| 3 | `qlcm.truong.a` gọi `listMyTenants`; cùng access token gọi `getMyPermissions` với `X-Tenant-Id` `pgd-demo` rồi `th-demo-a`; thu hồi UserRole ở `th-demo-a` rồi gọi lại | 2 tenant; đổi tenant không cần token mới; sau thu hồi → `403 TENANT_MISMATCH` | F012-AC-04, F012-AC-07 |
| 4 | Token `eduworld-portal` của `hs.an.3a` (tắt điều kiện role trong realm test) gọi `/api/v1/tenant/users` | `403 PORTAL_ACCESS_STAFF_ONLY` + audit | F012-AC-05 |
| 5 | `sa.admin` token thiếu `acr` MFA gọi `/api/v1/admin/roles` | `403 MFA_REQUIRED` | F012-AC-06 |
| 6 | `ad.donvi` gọi `listTenantUsers?tenant_ids=<th-demo-a>` | `tenant_ids` bị bỏ qua, chỉ `pgd-demo` | F012-AC-09 |
| 7 | Module `learning` gọi `resolvePrincipal` (facade test) cho `qlcm.truong.a`/`th-demo-a`; gọi lại với tenant không thuộc membership | Principal scope `subject`; `TENANT_MISMATCH` | F012-AC-11, F012-AC-12 |
| 8 | Thêm import `modules/identity/api` vào một module không có trong `x-allowed-callers` của `resolvePrincipal`; `curl :9080/internal/v1/principals:resolve` | `make arch-check` fail; 404 (không có route HTTP nội bộ) | F012-AC-13 |
| 9 | `ad.donvi` tạo role tùy biến chứa `platform.cross_tenant.read`; sửa role `school_admin` | `422 PERMISSION_NOT_ALLOWED_FOR_SCOPE`; `403` | F012-AC-16, F012-AC-17 |
| 10 | `ad.truong.a` gán `academic_manager` scope môn cho nhân sự trường A; gán ở trường B | Thành công + `user_role.changed`; `403 PERMISSION_DENIED` | F012-AC-22, F012-AC-23 |
| 11 | Thu hồi UserRole của `qlcm.truong.a` khi đang đăng nhập | Mất quyền ≤ 60 s ở module learning | F012-AC-14, F012-AC-23 |
| 12 | `ad.donvi` tạo nhân sự email trùng (khác hoa thường) / không email | `409 EMAIL_TAKEN` / `422` | F012-AC-26 |
| 13 | `portalLogin` `ad.truong.a` (không OTP), `sa.admin` (OTP đúng); xem `Set-Cookie` và giải mã token | `200`; cookie `__Secure-ew_portal_rt` HttpOnly SameSite=Strict Path `/api/v1/auth/portal`, `__Host-ew_portal_csrf`; `sa.admin` có `amr` chứa `otp` | F012-AC-27 |
| 14 | `portalLogin` sai mật khẩu; `sa.admin` thiếu OTP; OTP sai; username không tồn tại | Cùng `401 INVALID_CREDENTIALS`, cùng thông điệp | F012-AC-28 |
| 15 | `portalLogin` bằng `hs.an.3a` (mật khẩu đúng) | `403 PORTAL_ACCESS_STAFF_ONLY` + audit | F012-AC-29 |
| 16 | `refreshPortalToken` không header `X-EW-CSRF`; có header đúng; gửi lại refresh cookie cũ; `portalLogout` hai lần | `403 CSRF_TOKEN_INVALID`; `200` token + cookie mới; `401 REFRESH_TOKEN_INVALID`; `204` ×2 | F012-AC-30 |
| 17 | `grep` log `core-api` + Tempo theo mật khẩu/OTP mẫu kịch bản 13–14 | 0 kết quả | F012-AC-31 |
| 18 | 11 lần `portalLogin`/phút cùng username từ nhiều `device_id`; 50 username khác từ một IP | Lần 11 → `429 RATE_LIMITED`; IP chung không bị chặn | F012-AC-32 |
| 19 | `portalLogin` tài khoản còn `UPDATE_PASSWORD` → `setupPortalPassword` mật khẩu yếu, rồi hợp lệ; dùng lại ticket | `403 ACCOUNT_SETUP_REQUIRED` + ticket; `422 PASSWORD_POLICY_VIOLATION`; thành công (`remaining_steps`); `401 SETUP_TICKET_INVALID` | F012-AC-33, F012-AC-34 |
| 20 | Tài khoản platform chưa có OTP đăng nhập | `403 ACCOUNT_SETUP_REQUIRED` `required_steps=[configure_otp]`, `otp_setup_email_sent=true`; email có link trang action-token | F012-AC-35 |
| 21 | `forgotPortalPassword` email `ad.truong.a` / email học sinh / email không tồn tại; mở link; dùng link lần hai | `202` ×3, chỉ `ad.truong.a` nhận email; đặt mật khẩu được, phiên cũ bị thu hồi; `401 PASSWORD_RESET_TOKEN_INVALID` | F012-AC-36, F012-AC-37 |
| 22 | `getMyPermissions` của `ad.truong.a` khi F015 tắt `menu.school_lms.assignments` cho `th-demo-a` | `disabled_menus` có menu; `disabled_actions` có mọi action của menu | F012-AC-39 |
| 23 | `ad.truong.a` gọi `listAssignments`; `ad.truong.b` (tenant khác) gọi; `hs.an.3a` (token game) gọi API học sinh; tạm dừng PostgreSQL identity + xóa cache rồi gọi lại | `403 FEATURE_DISABLED` (`feature_key`); `200`; không bị ảnh hưởng; `503 UNAVAILABLE` | F012-AC-40, F012-AC-41 |

## 3. Lệnh test tự động

```bash
make test MOD=identity
make test-integration MOD=identity
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), bảng ma trận authorization sinh từ OpenAPI, trace_id kịch bản 7 và 11, log khởi động thất bại kịch bản 1.
- Header `Set-Cookie` kịch bản 13, kết quả quét log kịch bản 17, bảng operation bị chặn bởi `featuregate` kịch bản 23.

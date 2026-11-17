# Quickstart: F013 — Đăng nhập game qua backend: login, chọn tenant, refresh, logout, bảo vệ đăng nhập, hàng chờ

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F011, F012 (và F004: client `eduworld-game-backend`, secret local mount vào module identity).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo (mật khẩu seed local): `hs.an.3a`, `hs.tudo` (tenant chung), `hs.nosis`, `hs.locked`, `ad.truong.a`, `qlcm.truong.a`; học sinh hai tenant tạo bằng fixture test `hs.hai.tenant`.
- Gọi bằng `curl`/Postman CLI qua APISIX public `http://localhost:9080/api/v1`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `gameLogin` `hs.an.3a` (`device_id`, `client_version` hợp lệ, `platform=windows`) | `200`; `bootstrap.tenant_id` = `th-demo-a`, lớp 3A, `is_common_tenant=false`; nhật ký xác thực `single_membership` | F013-AC-01, F013-AC-02, F013-AC-07 |
| 2 | `gameLogin` `hs.tudo` | `200`, `is_common_tenant=true`, lớp `common-g4` | F013-AC-08 |
| 3 | `gameLogin` `hs.nosis` rồi dùng refresh token Keycloak (log test) | `403 ACCOUNT_NOT_PROVISIONED`; refresh `invalid_grant`; audit | F013-AC-03 |
| 4 | `gameLogin` `ad.truong.a`, `qlcm.truong.a` | `403 GAME_ACCESS_STUDENT_ONLY`, không có token, audit | F013-AC-04 |
| 5 | `gameLogin` `hs.locked`; body thêm `school_code` | `403 ACCOUNT_LOCKED`; `422 VALIDATION_FAILED` | F013-AC-05, F013-AC-06 |
| 6 | `gameLogin` `hs.hai.tenant` → `selectGameLoginTenant`; dùng lại ticket; ticket từ `device_id` khác | `403 TENANT_NOT_SELECTED` + ticket → `200`; `401 LOGIN_TICKET_INVALID` | F013-AC-09, F013-AC-10, F013-AC-11 |
| 7 | `refreshGameToken` (`renew`, đúng `device_id`) hai lần với cùng refresh token | Lần 1 token mới (khác token cũ); lần 2 `401 REFRESH_TOKEN_REUSED`, token mới của lần 1 cũng bị từ chối (`401 REFRESH_TOKEN_INVALID`); audit `auth.refresh_token_reused` | F013-AC-12, F013-AC-13 |
| 7b | `refreshGameToken` với `device_id` khác thiết bị đăng nhập; rồi với đúng `device_id` | `401 REFRESH_TOKEN_DEVICE_MISMATCH`, Keycloak không có event refresh; lần sau `200` | F013-AC-27 |
| 7c | `refreshGameToken` `mode=resume` + `client_version` hợp lệ; rồi phát event fixture `user.access_revoked` cho `hs.an.3a` (T045, không cần F016) và `resume` lại | `200` + `bootstrap` (tenant `th-demo-a`); sau event: `401 REFRESH_TOKEN_INVALID` (phiên bị thu hồi qua event). Khóa thật qua F016: quickstart F016 bước 13 | F013-AC-28, F013-AC-29 |
| 7d | Đổi thời gian Keycloak test realm vượt idle (`game_session_sso` rút ngắn trong test) rồi refresh | `401 REFRESH_TOKEN_INVALID` | F013-AC-13, F013-AC-15 |
| 8 | `gameLogout` rồi refresh | `204`; refresh `401 REFRESH_TOKEN_INVALID`; `game_auth_sessions.revoke_reason = logout` | F013-AC-14 |
| 9 | Gửi mật khẩu mẫu duy nhất (thành công, sai, 5xx giả lập), quét log APISIX/`core-api`/Tempo | Không có chuỗi mẫu | F013-AC-16 |
| 10 | Sai mật khẩu 6 lần `hs.binh.3a`; 11 lần/phút từ một `X-Device-Id` | `401 INVALID_CREDENTIALS` cùng thông điệp; `429 RATE_LIMITED` + `Retry-After` | F013-AC-17, F013-AC-18 |
| 11 | `client_version` thấp hơn `EW_GAME_MIN_CLIENT_VERSION` | `426`; Keycloak không có event đăng nhập | F013-AC-20 |
| 12 | Gọi route tenant-scoped thiếu header / header tenant khác | `400 TENANT_HEADER_REQUIRED` / `403 TENANT_MISMATCH` + audit | F013-AC-22, F013-AC-23 |
| 13 | `gameLogin` `hs.hai.tenant`, xem audit và `/metrics` | Hàm `identity.memberships_for_user` được gọi bằng `identity_app`; audit `cross_tenant.accessed` (`reason = auth_login`), `eduworld_tenant_filter_bypass_total{reason="auth_login"}` tăng; `choices` chỉ gồm hai tenant của user | F013-AC-30 |
| 14 | `gameLogin` sai mật khẩu; xem `identity.login_events` | Dòng `tenant_id` NULL, ghi bởi `identity_platform` | F013-AC-31 |
| 15 | Clock giả 21:35 ICT: `hs.an.3a` `gameLogin`; thiết bị có phiên đã lưu gọi `refreshGameToken` `mode=resume`; đặt lại 06:31 và resume | `403 CURFEW_ACTIVE` + `reopen_at` 06:30; phiên không bị thu hồi; 06:31 resume `200` | F013-AC-32 |

## 3. Lệnh test tự động

```bash
make test MOD=identity
make test-integration MOD=identity
k6 run tests/perf/login_queue.js   # TC-PERF-016, staging
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 1 và 6 (không chứa body), kết quả quét log kịch bản 9, báo cáo k6 hàng chờ.

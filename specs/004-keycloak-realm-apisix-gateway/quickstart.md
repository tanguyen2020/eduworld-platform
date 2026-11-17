# Quickstart: F004 — Keycloak realm as code và APISIX gateway (route, `tenant-header`, gateway nội bộ)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F001 (F002 cần cho bước kiểm 401 phía backend)
- Lệnh: `make up` (keycloak, apisix public `:9080`, apisix-internal `:9180` [Proposed]), `make keycloak-import`, `make apisix-render ENV=local`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo (mật khẩu seed local): `hs.an.3a`, `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`, `sa.admin`, `sm.antoan`; thêm tài khoản nhân sự còn `UPDATE_PASSWORD` (T043) cho kịch bản 27.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `node deploy/keycloak/scripts/check-realm.mjs`; Admin API liệt kê client | Đủ 10 client; không secret thật; file chung không có `users` | F004-AC-01, F004-AC-08 |
| 2 | Password grant `hs.an.3a` qua `eduworld-game-backend` (có secret) / `eduworld-portal` / không secret | Thành công / `unauthorized_client` / `invalid_client` | F004-AC-02 |
| 3 | Password grant `ad.truong.a` qua `eduworld-game-backend` | Bị từ chối (`invalid_grant`) | F004-AC-03 |
| 4 | (Thay bằng kịch bản 25 — portal không còn màn đăng nhập Keycloak, ADR-0091) | — | F004-AC-03 |
| 5 | Client credentials `svc-identity`; thử client credentials với `svc-game` | `svc-identity` nhận token có role `realm-management` tối thiểu; `svc-game` → `invalid_client` (không tồn tại); realm không có scope `internal-*` | F004-AC-04 |
| 6 | Giải mã access token của `hs.an.3a` | Có `aud: eduworld-api`, `roles`; không `email`, `given_name`, `organization` | F004-AC-05 |
| 7 | Đọc cấu hình realm (Admin API) | Brute-force theo tài khoản, chính sách mật khẩu, TTL token/phiên đúng tham số | F004-AC-06 |
| 8 | `make keycloak-import` lần 2 | Log "no changes" | F004-AC-07 |
| 9 | `curl :9080/api/v1/admin/tenants`, `/api/v1/sis/imports`, `/api/v1/game-admin/quests`, `/internal/v1/principals:resolve` | 404 từ APISIX, upstream không nhận request | F004-AC-09 |
| 10 | `curl :9180/api/v1/admin/tenants` từ IP ngoài CIDR | 403 | F004-AC-10 |
| 11 | `GET :9080/api/v1/lms/assignments` có token, thiếu `X-Tenant-Id`; header `abc` | 400 `TENANT_HEADER_REQUIRED`; 400 `BAD_REQUEST` (Problem JSON) | F004-AC-11 |
| 12 | Gửi header hợp lệ, xem access log + trace | Có `tenant.id`; request tới upstream | F004-AC-12 |
| 13 | Token sai `aud` tới `/api/v1/learning/...` | 401 tại APISIX | F004-AC-13 |
| 14 | `node deploy/apisix/scripts/check-routes.mjs` | Pass; sửa upstream sai → fail | F004-AC-14 |
| 15 | `node deploy/apisix/scripts/check-no-body-logging.mjs`; bật `include_req_body` thử | Pass; bật → fail | F004-AC-15 |
| 16 | Gửi mật khẩu `Pw-UNIQ-7f3c…` tới `/auth/game/login`; `grep` log + Tempo | 0 kết quả | F004-AC-16 |
| 17 | 11 lần/phút cùng `X-Device-Id`; 11 lần/phút cùng username nhiều thiết bị; thiết bị khác cùng IP | Lần 11: 429 `RATE_LIMITED`; thiết bị/username khác vẫn qua | F004-AC-17 |
| 18 | Xem header response 429 | `RateLimit-*`, `Retry-After`, body Problem | F004-AC-18 |
| 19 | Render cấu hình `ENV=staging`, kiểm `ssl` + HSTS | TLS ≥ 1.2, HSTS bật, HTTP trên `/auth/game/*` bị từ chối | F004-AC-19 |
| 20 | Kết nối `ws://localhost:9080/realtime/v1?ticket=x`; xem access log | Upgrade được; log không có `ticket=` | F004-AC-20 |
| 21 | Preflight CORS từ `http://localhost:5173` và origin lạ | Origin allowlist được phép; origin lạ bị chặn | F004-AC-21 |
| 22 | `grep -R "http://" deploy/apisix/routes deploy/keycloak/realm-eduworld.json` | Chỉ biến `${EW_*}`/`${env.*}` | F004-AC-22 |
| 23 | PR mẫu sửa `routes.yaml` | Workflow `deploy-config.yml` đăng diff lên PR | F004-AC-23 |
| 24 | Admin API đọc client `eduworld-portal`; password grant không secret / có secret | Confidential, Standard Flow tắt, không redirect URI; không secret → `invalid_client`; có secret → token | F004-AC-24, F004-AC-02 |
| 25 | Password grant `eduworld-portal` (có secret): `ad.truong.a` mật khẩu đúng; `sa.admin` thiếu OTP / OTP sai / OTP đúng; `hs.an.3a` mật khẩu đúng | Token; `invalid_grant` ×2 cùng lỗi như sai mật khẩu, rồi token; `hs.an.3a` bị chặn sau bước mật khẩu (thông điệp `portalStudentDenied`) | F004-AC-25, F004-AC-03 |
| 26 | Giải mã token của `ad.truong.a` và `sa.admin` (OTP) | `amr` = [`pwd`] / [`pwd`, `otp`]; `acr` mức mật khẩu / MFA; không claim `organization` | F004-AC-26, F004-AC-05 |
| 27 | Gọi `execute-actions-email` `CONFIGURE_TOTP` cho tài khoản thử; mở link | Trang action-token theme `eduworld-ops` (vi/en) quét QR; realm không có browser flow/login theme cho portal | F004-AC-27 |
| 28 | `POST :9080/api/v1/auth/portal/login` qua host portal với mật khẩu mẫu duy nhất; 11 lần/phút cùng username; cùng `device_id`; nhiều username khác từ một IP | Log/trace không chứa mật khẩu; lần 11 → `429 RATE_LIMITED`; IP chung không bị chặn; route không yêu cầu `X-Tenant-Id` | F004-AC-28 |
| 29 | Đọc research R5 | Có kết quả spike (`amr`/`acr`, required action, CONFIGURE_TOTP, thông điệp chặn học sinh) | F004-AC-29 |

## 3. Lệnh test tự động

```bash
node --test deploy/apisix/scripts/ deploy/keycloak/scripts/
make apisix-plugin-test          # test-nginx cho ew-tenant-header, ew-login-key
postman collection run specs/postman/gateway.postman_collection.json -e specs/postman/env.local.postman_environment.json
```

## 4. Bằng chứng cần nộp

- Output script kiểm, kết quả quét log TC-AUTH-060 (kịch bản 16), bảng 404 của prefix nội bộ, diff realm lần áp thứ hai.
- Kết quả spike R5 (token đã giải mã có/không OTP), quét log `/auth/portal/*` (kịch bản 28).

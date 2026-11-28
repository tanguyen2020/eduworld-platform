# Contracts: F135 — Học sinh tự đăng ký trong game vào tenant chung (bước phụ huynh, mã khôi phục, chống lạm dụng)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec module `specs/openapi/modules/identity.v1.yaml` (public). Endpoint `/auth/game/*` là public như `gameLogin`: không token, không `X-Tenant-Id` (ngoại lệ đã có ở FR-AUTH-018), rate limit ở APISIX + backend, không log body.

| operationId | Method + path | Dùng cho | Trạng thái |
|---|---|---|---|
| `getRegistrationTerms` | `GET /api/v1/auth/game/registration-terms` (`Accept-Language`) → `200 {enabled, terms_version, locale, text, max_lengths}` | Hiện/ẩn "Tạo tài khoản", văn bản điều khoản bước phụ huynh | **Mới** |
| `registerStudent` | `POST /api/v1/auth/game/register` (`Idempotency-Key`; body `StudentSelfRegistration`: `username`, `password`, `grade_level`, `date_of_birth`, `device_id`, `platform`, `client_version`, `parent_consent{accepted, terms_version}`, `parent_email?`) → `201 SelfRegistrationResult {user_id, username, recovery_code, recovery_code_redacted}`; `403 REGISTRATION_DISABLED`, `403 CURFEW_ACTIVE` (`reopen_at`), `409 USERNAME_TAKEN`, `409 IDEMPOTENCY_KEY_REUSED`, `422 VALIDATION_FAILED`, `429 REGISTRATION_LIMIT_REACHED` (`retry_after`), `429 REGISTRATION_BUSY` (`Retry-After`), `503 UNAVAILABLE`; `Cache-Control: no-store` | Tạo tài khoản | **Mới** |
| `recoverStudentPassword` | `POST /api/v1/auth/game/password:recover` (`Idempotency-Key`; `{username, recovery_code, new_password, device_id}`) → `200 {recovery_code}` (mã mới, một lần, `no-store`); `401 RECOVERY_INVALID`, `422`, `429 RATE_LIMITED` | Quên mật khẩu bằng mã | **Mới** |
| `requestParentPasswordReset` | `POST /api/v1/auth/game/password:parent-link` (`{username, device_id}`) → luôn `202` | Gửi link đặt lại cho email phụ huynh đã xác nhận | **Mới** |
| `confirmParentEmail` | `POST /api/v1/auth/parent/email:confirm` (`{token}`) → `204`; `410 LINK_EXPIRED` | Trang công khai `/phu-huynh/xac-nhan` | **Mới** |
| `resetPasswordByParentLink` | `POST /api/v1/auth/parent/password:reset` (`Idempotency-Key`; `{token, new_password}`) → `200 {username, recovery_code}`; `410 LINK_EXPIRED`, `422` | Trang công khai `/phu-huynh/dat-lai-mat-khau` | **Mới** |
| `getSelfRegistrationEvidence` | `GET /api/v1/admin/common-tenant/students/{student_id}/self-registration` (`X-Tenant-Id` = `common`, quyền `platform.user.manage`, gateway nội bộ) → `200 SelfRegistrationEvidence {terms_version, consented_at, platform, client_version, parent_email_masked, parent_email_confirmed_at}`; `404` | Chi tiết học sinh (System Admin) | **Mới** |
| `listTenantGradeStudents`, `listTenantUsers`, `/admin/users` (list) | Có sẵn | Thêm field `account_source` + query `account_source` | **Sửa** |
| `resetStudentPassword` | Có sẵn | Response thêm `recovery_code` (chỉ khi `account_source = self_registration`) | **Sửa** |
| `createCommonTenantStudents` | Có sẵn | Sửa mô tả "Không có tự đăng ký (F010-AC-26)" → trỏ `registerStudent` (ADR-0104) | **Sửa** (chỉ mô tả) |
| `putPlatformSettings` (cài đặt platform sẵn có, F015) | Có sẵn | Khóa `self_registration.enabled` | **Sửa** (schema cài đặt) |
| `gameLogin`, `createGameSession` | Có sẵn | Không đổi — client gọi sau đăng ký | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có: mọi dữ liệu thuộc module identity; module khác nhận event | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng (đăng ký trước khi có phiên) | — |

## 4. Event (NATS)

| Event | Producer → Consumer | Schema | Trạng thái |
|---|---|---|---|
| `user.provisioned` | identity phát (owner F010) → game (GameProfile), AUD | `specs/events/payloads/identity/user.provisioned.v1.schema.json` | Có sẵn — **Sửa** (owner F010): enum `source` thêm `self_registration` |
| `student.placement_changed` | identity phát (owner F010) → learning, game, competition, chat | `specs/events/payloads/identity/student.placement_changed.v1.schema.json` | Có sẵn — không đổi |
| `user.access_revoked` | identity phát (owner F011) → realtime, game | `specs/events/payloads/identity/user.access_revoked.v1.schema.json` | Có sẵn — không đổi (`reason = password_changed`) |
| `platform_settings.changed` | identity phát (owner F015) | `specs/events/payloads/identity/platform_settings.changed.v1.schema.json` | Có sẵn — không đổi |

## 5. JSON Schema nội dung

| Schema | Thay đổi |
|---|---|
| — | Không có. Văn bản điều khoản là seed Markdown có version của module identity (trả qua `getRegistrationTerms`, `text` `maxLength` 8000) |

## 6. Delta cần thực hiện

| # | File | Thay đổi | Task |
|---|---|---|---|
| D1 | `specs/openapi/modules/identity.v1.yaml` | 7 operation mới ở §1; schema `StudentSelfRegistration` (`additionalProperties: false`; `username` `pattern ^[a-z0-9._-]{3,32}$` sau chuẩn hóa — server chuẩn hóa trước khi kiểm, `maxLength` 32; `password` `maxLength` 128; `date_of_birth` `format: date`; `grade_level` 1–9; `device_id` uuid; `platform` enum; `client_version` `maxLength` 32; `parent_email` `format: email` `maxLength` 254; `terms_version` `maxLength` 32), `SelfRegistrationResult`, `PasswordRecovery`, `ParentLinkRequest`, `ParentTokenConfirm`, `ParentPasswordReset`, `RegistrationTerms` (`text` `maxLength` 8000), `SelfRegistrationEvidence` (`parent_email_masked` `maxLength` 254); `account_source` enum trong `StudentSummary`/`UserSummary` + query; ví dụ cho mọi response | T002 |
| D2 | Problem codes (API_DESIGN bảng mã lỗi, `check-problem-codes`) | `REGISTRATION_DISABLED` (403), `REGISTRATION_LIMIT_REACHED` (429), `REGISTRATION_BUSY` (429), `RECOVERY_INVALID` (401), `LINK_EXPIRED` (410) | T003 |
| D3 | `specs/events/payloads/identity/user.provisioned.v1.schema.json` (owner F010) | `source` enum thêm `self_registration` (thêm giá trị) | T004 |
| D4 | Schema cài đặt platform (F015) + CONFIG_DEFAULTS | `self_registration.enabled` (bool, mặc định `true`) | T004 |
| D5 | `deploy/apisix` | Route công khai `/api/v1/auth/game/register`, `/password:recover`, `/password:parent-link`, `/api/v1/auth/parent/*` (không auth, `limit-count` theo `device_id`/tên đăng nhập/token, tắt log body) | T005 |
| D6 | Bundle `specs/openapi/eduworld-api.v1.yaml` | `node scripts/openapi/split-public.mjs`, `make gen MOD=identity`, `make specs-lint`, `make specs-breaking` (chỉ thêm — không breaking) | T002 |
| D7 | `specs/portal/menu-catalog.yaml` | Route công khai phụ huynh không có trong catalog menu (không cần quyền); bộ lọc "Tự đăng ký" dùng action sẵn có `action.platform_admin.access_control.edit_users` | T005 |

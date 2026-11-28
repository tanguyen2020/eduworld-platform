# Data Model: F135 — Học sinh tự đăng ký trong game vào tenant chung (bước phụ huynh, mã khôi phục, chống lạm dụng)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Database `eduworld_identity`. Mọi bảng mới dùng mixin `entx` tenant + soft-delete (ADR-0039, ADR-0089); `tenant_id` luôn là tenant `common`; unique dùng partial index `WHERE deleted_at IS NULL`; mọi chuỗi có `MaxLen` (ADR-0041).

| Bảng | Trạng thái | Cột chính | Ghi chú |
|---|---|---|---|
| `identity.self_registrations` | Mới | `id` (UUIDv7), `tenant_id`, `user_id` (unique partial), `terms_version varchar(32)`, `terms_locale varchar(8)`, `consented_at timestamptz` (server), `device_id_hash varchar(64)` (HMAC-SHA256 hex), `device_hash_key_version smallint`, `platform varchar(16)`, `client_version varchar(32)`, `parent_email_enc bytea` (NULL nếu bỏ trống), `parent_email_hash varchar(64)` (HMAC, NULL), `parent_email_confirmed_at timestamptz`, `created_at`, `deleted_at`, `deleted_by` | Bằng chứng đồng ý; index `(device_id_hash, created_at)` và `(created_at)` để đếm giới hạn; không IP, không user-agent thô |
| `identity.recovery_codes` | Mới | `id`, `tenant_id`, `user_id`, `code_hash varchar(128)` (argon2id, PHC string), `created_at`, `used_at`, `revoked_at`, `revoke_reason varchar(32)` (`used`, `rotated`, `admin_reset`, `anonymized`), `deleted_at` | Một mã hiệu lực: partial unique `(user_id) WHERE used_at IS NULL AND revoked_at IS NULL AND deleted_at IS NULL` |
| `identity.recovery_attempts` | Mới | `id`, `tenant_id`, `username_norm_hash varchar(64)`, `device_id_hash varchar(64)`, `attempted_at`, `result varchar(16)` | Append-only (không `deleted_at`, ngoại lệ bảng append-only R21), giữ `recovery_attempt_retention` = 30 ngày rồi ẩn danh (chỉ đếm); đếm `recovery_attempt_limit` từ PostgreSQL |
| `identity.parent_link_tokens` | Mới | `id`, `tenant_id`, `user_id`, `kind varchar(24)` (`email_confirm`, `password_reset`), `token_hash varchar(64)` (SHA-256), `expires_at`, `used_at`, `created_at`, `deleted_at` | Token một lần |
| `identity.users` | Có sẵn — **Thêm cột** `account_source varchar(24)` (`sis`, `platform_common`, `self_registration`, `manual`; backfill từ ExternalIdentity/`user.provisioned` đã có) | `display_name = NULL` với tài khoản tự đăng ký | Expand migration, không xóa cột (R12) |
| `org.students` | Có sẵn | `full_name = NULL`, `date_of_birth_enc`, `grade_level`, `school_id` = School ảo | Không đổi cấu trúc |
| `identity.email_deliveries` | Có sẵn (F124) | Thêm giá trị `template` `parent_email_confirm`, `parent_password_reset` | Người nhận lưu mã hóa như hiện có |
| `platform.platform_settings` | Có sẵn | Khóa `self_registration.enabled` (bool) | Công tắc, audit qua `platform_settings.changed` |

RLS: bảng mới chỉ lọc tenant như mọi bảng identity (ADR-0089); role DB của module identity; không GRANT cho module khác.

## 2. Vòng đời / trạng thái

- **Đăng ký:** `validated` → `keycloak_created` → `committed` (users + students + evidence + recovery code + outbox) | `compensated` (Keycloak `enabled = false` khi transaction lỗi, ghi `reconciliation_issue` loại `orphan_keycloak_user` nếu bù lỗi).
- **Email phụ huynh:** `none` | `pending` (thư đã xếp) → `confirmed` (link) | `expired` (hết `parent_email_confirm_ttl`, gửi lại được qua System Admin).
- **Mã khôi phục:** `active` → `used` (khôi phục thành công) → mã mới `active`; `active` → `revoked` (`rotated` khi đặt lại qua link phụ huynh, `admin_reset`, `anonymized`).
- **Token link:** `issued` → `used` | `expired`.
- **Tài khoản:** như học sinh tenant chung (`active` → `locked`/`deactivated`, soft-delete khi dọn, khôi phục khi đăng nhập lại — ADR-0032); gộp → nguồn soft-delete (F045).

## 3. Quy tắc kiểm tra dữ liệu

- `grade_level` 1–9; `date_of_birth` hợp lệ, không ở tương lai, tuổi theo năm học trong `[grade + 4, grade + 7]` (`self_registration_age_margin` −1/+2) và 5–16.
- `username`: trim, NFC, chữ thường; `^[a-z0-9._-]{3,32}$`; không bắt đầu bằng tiền tố trong `self_registration_reserved_username_prefixes`; duy nhất trên SSO và `identity.users` (ADR-0069).
- `password`: theo password policy của realm; `maxLength` 128 ở contract; không lưu.
- `parent_consent.accepted = true` bắt buộc; `terms_version` phải là version hiệu lực.
- `parent_email`: tùy chọn, `format: email`, `maxLength` 254; chuẩn hóa chữ thường trước HMAC.
- `device_id`: UUID bản cài (như `gameLogin`), bắt buộc.
- Không nhận trường lạ (`additionalProperties: false`) — chặn `full_name`, `school`, `phone`.

## 4. Event / outbox

| Event | Vai trò | Ghi chú |
|---|---|---|
| `user.provisioned` | Phát (owner F010) | Cùng schema; thêm giá trị `source = self_registration` vào enum (delta schema F010, không breaking: consumer bỏ qua giá trị lạ) |
| `student.placement_changed` | Phát (owner F010) | Như học sinh tenant chung mới (School ảo, khối) |
| `user.access_revoked` | Phát (owner F011) | Khi khôi phục/đặt lại mật khẩu (`reason = password_changed`) |
| `platform_settings.changed` | Phát (owner F015) | Khi System Admin đổi `self_registration.enabled` |

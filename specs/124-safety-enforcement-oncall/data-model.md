# Data Model: F124 — Thực thi xử phạt và vận hành Safety Team: consumer `sanction.applied` (khóa tài khoản), email thông báo vụ việc cho trường, vòng trực, break-glass

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Chuỗi theo ADR-0041 (`MaxLen(n)` → `varchar(n)`). Bảng mới cập nhật DATABASE_DESIGN ở task Polish.

## 1. Entity

### AccountLock — `identity.account_locks` (Mới · DB `eduworld_identity`)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `lock_id` | UUIDv7 | PK | mixin `Base` |
| `user_id` | uuid | NOT NULL, FK `identity.users` | |
| `tenant_id` | uuid | NULL chỉ khi `source = platform_admin` và user là nhân sự platform | R8; khóa học sinh luôn có tenant |
| `source` | `varchar(32)` | CHECK `school_admin/platform_admin/sis/moderation_sanction` | |
| `source_ref_id` | uuid | NULL được | `sanction_id` khi `moderation_sanction` |
| `case_id` | uuid | NULL được | chỉ ID, không nội dung |
| `reason` | `varchar(1000)` | | nội bộ; không trả cho học sinh |
| `created_by` | uuid | NULL = hệ thống | |
| `created_at` | timestamptz | NOT NULL | |
| `released_at`, `released_by` | timestamptz, uuid | | |
| `release_reason` | `varchar(1000)` | | |
| `source_event_id` | uuid | | event gây khóa/giải phóng |
| `deleted_at`, `deleted_by` | | | R21 (không dùng xóa nghiệp vụ) |

**Index**: `(user_id) WHERE released_at IS NULL AND deleted_at IS NULL`; UNIQUE `(source, source_ref_id) WHERE source_ref_id IS NOT NULL AND deleted_at IS NULL` (một khóa/sanction, kể cả khi `lifted` đến trước — tạo dòng đã `released_at`).
**Bất biến**: `identity.users.status = locked` ⇔ có ít nhất một khóa `released_at IS NULL` (trừ `deactivated`/`anonymized`). Migration expand: backfill một dòng `school_admin`/`platform_admin` cho mỗi user đang `locked` từ `status_reason`, `status_changed_by` của F016.

### EmailDelivery — `identity.email_deliveries` (Mới · DB `eduworld_identity`)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `delivery_id` | UUIDv7 | PK | |
| `tenant_id` | uuid | NULL cho email nhân sự platform (on-call, break-glass) | |
| `source_event_id` | uuid | NOT NULL | |
| `template_code` | `varchar(64)` | CHECK `school_safety_notification/school_safety_reminder/moderation_oncall_page/break_glass_request` | |
| `recipient_user_id` | uuid | NOT NULL | **không** lưu địa chỉ email |
| `params` | jsonb | ≤ 2 KB; chỉ ID và thời điểm (`school_id`, `notification_id`, `ack_due_at`, `reminder_no`, `case_id`, `grant_id`, `severity`) | không tên học sinh/nội dung |
| `status` | `varchar(16)` | CHECK `pending/sent/failed/skipped` | |
| `attempts` | smallint | DEFAULT 0 | |
| `next_attempt_at`, `sent_at`, `created_at` | timestamptz | | |
| `last_error_code` | `varchar(64)` | | mã SMTP, không thân lỗi |

**Index**: UNIQUE `(source_event_id, recipient_user_id, template_code)`; `(status, next_attempt_at) WHERE status = 'pending'`. Bảng append + cập nhật trạng thái; không DELETE.

### ModerationCase — `chat.moderation_cases` (Có sẵn · Thêm cột · DB `eduworld_chat`)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `assigned_via` | `varchar(16)` | **Thêm cột**, CHECK `manual/rotation`, NULL khi chưa gán | khớp payload `moderation.case_assigned` (EVENT_DESIGN) |
| `oncall_page_level` | `varchar(16)` | **Thêm cột**, CHECK `primary/secondary/lead/system_admin` | mức đã gọi gần nhất |
| `last_paged_at` | timestamptz | **Thêm cột** | |

**Index**: `(severity, status, created_at) WHERE status = 'open' AND severity = 'p1'` (job gọi trực, đọc qua Bypass `moderation_case`).

### OncallShift — `chat.moderation_oncall_shifts` (Mới · bảng platform G, không `tenant_id`)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `shift_id` | UUIDv7 | PK | |
| `moderator_user_id` | uuid | NOT NULL | nhân sự platform có `moderation.case.act` |
| `oncall_role` | `varchar(16)` | CHECK `primary/secondary` | |
| `starts_at`, `ends_at` | timestamptz | CHECK `ends_at > starts_at`; độ dài ≤ `oncall_shift_max_hours` [Confirmed] | |
| `note` | `varchar(500)` | | |
| `created_by`, `created_at`, `version` | | | `If-Match` không bắt buộc (tạo/xóa) |
| `deleted_at`, `deleted_by` | | | soft-delete (R21) |

**Index**: `(oncall_role, starts_at, ends_at) WHERE deleted_at IS NULL`. Không chồng lấn cùng `oncall_role` — kiểm trong command dưới `pg_advisory_xact_lock(hash('oncall:'||role))` + test.

### BreakGlassGrant — `chat.break_glass_grants` (Mới · DB `eduworld_chat`)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `grant_id` | UUIDv7 | PK | |
| `tenant_id` | uuid | NOT NULL | tenant đích (ghi luôn có tenant, R20) |
| `scope_type` | `varchar(16)` | CHECK `channel/user` | |
| `scope_id` | uuid | NOT NULL | `channel_id` hoặc `user_id` |
| `window_from`, `window_to` | timestamptz | `window_to > window_from`, ≤ `break_glass_max_window_days` [Confirmed] | khoảng thời gian tin được đọc |
| `duration_minutes` | int | 1…`break_glass_max_duration_hours`×60 [Confirmed] | |
| `reason` | `varchar(1000)` | NOT NULL, ≥ 20 | |
| `status` | `varchar(16)` | CHECK `pending/approved/rejected/expired/revoked` | |
| `requested_by`, `requested_at` | uuid, timestamptz | NOT NULL | |
| `decided_by`, `decided_at` | uuid, timestamptz | CHECK `decided_by <> requested_by` | hai người |
| `decision_note` | `varchar(1000)` | | |
| `expires_at` | timestamptz | đặt khi `approved` | |
| `revoked_by`, `revoked_at` | uuid, timestamptz | | |
| `revoke_reason` | `varchar(1000)` | | |
| `messages_viewed_count` | int | DEFAULT 0 | hậu kiểm |
| `version` | int | `If-Match` | |
| `deleted_at`, `deleted_by` | | | R21 |

**Index**: `(status, expires_at) WHERE status = 'approved'`; `(tenant_id, requested_at DESC)`.

## 2. Vòng đời / trạng thái

| Entity | Từ | Sang | Điều kiện |
|---|---|---|---|
| AccountLock | (mới) | hiệu lực | `sanction.applied` `account_lock` / lock của F016 |
| AccountLock | hiệu lực | giải phóng | `sanction.lifted` (nguồn kiểm duyệt) · unlock cùng nguồn (F016) |
| EmailDelivery | `pending` | `sent` / `failed` / `skipped` | gửi được / hết số lần thử / người nhận không hợp lệ |
| ModerationCase (p1, `open`) | `oncall_page_level` NULL | `primary` → `secondary` → `lead` → `system_admin` | tạo case / 1× / 2× `p1_unclaimed_alert_minutes` / quá hạn `report_sla` |
| BreakGlassGrant | `pending` | `approved` / `rejected` | người duyệt khác người yêu cầu |
| BreakGlassGrant | `approved` | `expired` / `revoked` | quá `expires_at` / thu hồi |

## 3. Quy tắc kiểm tra dữ liệu

- Chỉ khóa học sinh (`user_type = student`) theo sanction; `tenant_id` envelope khớp membership ([MULTITENANCY](../../docs/06-architecture/MULTITENANCY.md)).
- Khóa nguồn `moderation_sanction` chỉ giải phóng bởi `sanction.lifted`; unlock của F016 chỉ giải phóng nguồn `school_admin`/`platform_admin`/`sis` tương ứng.
- Người nhận email thông báo: UserRole `school_admin` hiệu lực ở `school_id`, user `active`, không soft-delete ([ADMIN_PORTAL §2.3](../../docs/04-admin-portal/ADMIN_PORTAL.md)).
- Phạm vi break-glass `user`: chỉ tin `sender_id = scope_id` trong tenant đích; `channel`: mọi tin của channel; luôn giới hạn `[window_from, window_to)`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `sanction.applied` · `sanction.lifted` (tiêu thụ, identity) | F116/F121 áp/gỡ sanction | `specs/events/payloads/chat/sanction.applied.v1.schema.json`, `sanction.lifted.v1.schema.json` (Cần thêm — owner F116 cho `applied`, owner F121 cho `lifted`) |
| `moderation.school_notified` (tiêu thụ, identity) | F121 gửi thông báo trường | `…/chat/moderation.school_notified.v1.schema.json` (Cần thêm — owner F121; Cần sửa: thêm `notification_id`, `ack_due_at`) |
| `moderation.school_notification_reminded` (phát chat, tiêu thụ identity) | Job nhắc lại F121 | `…/chat/moderation.school_notification_reminded.v1.schema.json` (Cần thêm) |
| `moderation.oncall_paged` (phát chat, tiêu thụ identity) | Gọi một mức trực | `…/chat/moderation.oncall_paged.v1.schema.json` (Cần thêm): `case_id`, `severity`, `level`, `moderator_user_id?` |
| `break_glass.requested` (phát chat, tiêu thụ identity) | Tạo yêu cầu | `…/chat/break_glass.requested.v1.schema.json` (Cần thêm): `grant_id`, `tenant_id`, `requested_by` |
| `moderation.case_assigned` (phát chat, `assigned_via = rotation`) | Gán case p1 theo vòng trực | Schema owner F121 — F124 phát cùng schema |
| `user.status_changed` · `user.access_revoked` · `user.access_restored` (phát identity) | Khóa/mở theo sanction | Có sẵn — owner F011 (`user.status_changed`, `user.access_revoked`), owner F016 (`user.access_restored`); `reason` thêm giá trị `moderation_sanction` |
| `audit.recorded` | Mọi thao tác ở bảng §2 | Có sẵn — action `user.locked`, `user.unlocked`, `moderation.oncall_shift.created/deleted`, `moderation.oncall.paged`, `break_glass.requested/approved/rejected/revoked/expired/messages_viewed` |

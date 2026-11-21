# Data Model: F121 — Moderation: báo cáo, sanction, hàng đợi Safety Team liên tenant, thông báo vụ việc cho trường

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Mọi bảng ở DB `eduworld_chat`, schema `chat`, loại T: `tenant_id uuid NOT NULL` + RLS; đọc liên tenant chỉ qua `tenancy.Bypass(ctx, "moderation_case")` (ADR-0072). Chuỗi có `MaxLen` (ADR-0041).

## 1. Entity

### Report — `chat.reports` (Có sẵn · Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `report_id` | uuid | PK | |
| `tenant_id`, `school_id` | uuid | NOT NULL | trường của người báo cáo |
| `reporter_id` | uuid | NOT NULL | không trả cho người bị báo cáo |
| `target_type` | `varchar(16)` | CHECK `message/user` | |
| `target_id` | uuid | NOT NULL | |
| `reason` | `varchar(32)` | CHECK enum MODERATION §5 | |
| `note` | `varchar(500)` | **Thêm cột** | chỉ chuẩn hóa, không chặn |
| `context` | jsonb | **Thêm cột** | `{channel_id, channel_type, match_id?, party_id?}` |
| `case_id` | uuid | **Thêm cột** NOT NULL | |
| `weight` | `numeric(3,2)` | **Thêm cột** DEFAULT 1 | lạm dụng báo cáo [Confirmed] |
| `status` | `varchar(16)` | `open/linked/dismissed` | |
| `evidence` | jsonb | | snapshot ±10 tin (gốc + revision) |
| `created_at`, `deleted_at` | | | |

**Index**: `(tenant_id, target_type, target_id, created_at DESC)`; UNIQUE `(reporter_id, target_type, target_id, case_id) WHERE deleted_at IS NULL`.

### ModerationCase — `chat.moderation_cases` (Có sẵn · Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `case_id` | uuid | PK | |
| `school_ids` | uuid[] | **Thêm cột**, ≤ 20 | case liên trường |
| `status` | `varchar(16)` | CHECK `open/in_review/escalated/resolved/reopened` | MODERATION §6.2 |
| `severity` | `varchar(4)` | CHECK `p1/p2/p3` | |
| `source` | `varchar(32)` | CHECK `report/filter_alert/async_analysis/self_harm_signal` | |
| `category_code` | `varchar(64)` | | không có nội dung danh mục |
| `channel_id`, `channel_type` (`varchar(16)`) | | | |
| `target_type` (`varchar(16)`), `target_id` | | | khóa gộp 24 h |
| `subject_user_ids` | uuid[] | ≤ 20 | |
| `assignee_moderator_id` | uuid | | có sẵn |
| `report_count` | int | | |
| `first_response_due_at`, `sla_due_at` | timestamptz | **Thêm cột** `first_response_due_at` | theo `report_sla` |
| `hold_sanction_id` | uuid | **Thêm cột** | hạn chế tạm thời p1 |
| `school_notified_at`, `school_ack_at` | timestamptz | có sẵn | |
| `acknowledged_by` | uuid | **Thêm cột** | |
| `contact_outcome` | `varchar(32)` | **Thêm cột** CHECK `parent_contacted/unable_to_contact/not_needed` | |
| `contact_path` | `varchar(32)` | **Thêm cột** CHECK `school/platform_direct` | tenant `common` |
| `resolved_at` | timestamptz | **Thêm cột** | |
| `first_response_at`, `escalated_at` | timestamptz | **Thêm cột** | phản hồi đầu tiên (gán), leo thang SLA một lần |
| `assigned_via` | `varchar(16)` | **Thêm cột** `manual/rotation` | vòng trực F124 |
| `target_type`, `target_id`, `channel_id` | | | khóa gộp 24 h |
| `source_event_id` | uuid | **Thêm cột**, UNIQUE khi khác NULL | idempotency consumer `chat.message_held` |
| `version` | int | `If-Match` | |

**Index**: `(status, severity, sla_due_at)` (hàng đợi liên tenant, đọc qua Bypass); `(tenant_id, target_type, target_id) WHERE status IN ('open','in_review')`.

### ModerationAction — `chat.moderation_actions` (Có sẵn, append-only TA)

`action_id`, `tenant_id`, `case_id`, `action varchar(32)` (`assign/warn/mute/restrict/remove_message/escalate/dismiss/close/reopen/system_escalate/hold/notify_school`), `actor_id`, `actor_kind varchar(16)` (`moderator/system`), `note varchar(1000)`, `sanction_id`, `message_id`, `at`. Không UPDATE/DELETE.

### Sanction — `chat.sanctions` (Có sẵn · Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `sanction_id`, `user_id`, `school_id`, `case_id` | uuid | | |
| `sanction_type` | `varchar(32)` | CHECK MODERATION §7 | |
| `scope` | jsonb | | `{all: true}` hoặc `{channel_id}` |
| `reason` | `varchar(1000)` | NOT NULL | nội bộ |
| `reason_code` | `varchar(64)` | **Thêm cột** | mã thông điệp hiển thị cho học sinh |
| `issued_by` | uuid | | |
| `issued_by_kind` | `varchar(16)` | **Thêm cột** `system/moderator` | |
| `starts_at`, `ends_at`, `revoked_at` | timestamptz | | |
| `revoked_by` | uuid | **Thêm cột** | |
| `revoke_reason` | `varchar(1000)` | **Thêm cột** | |
| `lifted_at` | timestamptz | **Thêm cột** | đã phát `sanction.lifted` (gỡ/hết hạn; job `sanction-expirer` idempotent) |

**Index**: `(user_id) WHERE revoked_at IS NULL` (nạp cache); `(ends_at) WHERE revoked_at IS NULL`.

### ModerationEvidence — `chat.moderation_evidence` (Có sẵn — DATABASE_DESIGN §2.16; bảng tạo ở migration F121, F117 chỉ ghi thêm bản sao trước purge)

`evidence_id`, `case_id`, `school_id`, `snapshot jsonb`, `sha256`, `created_at`; bất biến, mọi lượt xem audit.

### SchoolSafetyNotification — `chat.school_safety_notifications` (Mới)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `notification_id` | uuid | PK | |
| `tenant_id`, `case_id`, `school_id` | uuid | NOT NULL | |
| `severity` | `varchar(4)` | | |
| `category_code` | `varchar(64)` | | |
| `students` | jsonb | ≤ 20 phần tử `{student_id, role}` | tên/khối đọc từ `ref.students`/identity lúc trả (không còn lớp — ADR-0095) |
| `measures_taken` | `varchar(500)` | | |
| `summary` | `varchar(1000)` | NOT NULL | tóm tắt theo mẫu, không chat |
| `school_notified_at`, `ack_due_at` | timestamptz | NOT NULL | |
| `reminder_count` | smallint | DEFAULT 0 | |
| `last_reminded_at` | timestamptz | | |
| `school_ack_at` | timestamptz | | |
| `acknowledged_by` | uuid | | |
| `contact_outcome` | `varchar(32)` | CHECK như case | |
| `ack_note` | `varchar(500)` | | |
| `created_by`, `deleted_at`, `deleted_by` | | | |

**Index**: `(tenant_id, school_id, school_ack_at)`; `(ack_due_at) WHERE school_ack_at IS NULL`. Cập nhật DATABASE_DESIGN §2.12 (task T050).

## 2. Vòng đời / trạng thái

| Từ | Sang | Điều kiện |
|---|---|---|
| `open` | `in_review` | `assignModerationCase` |
| `in_review` | `resolved` | `close`/`dismiss` (sau sanction hoặc không vi phạm) |
| `in_review` | `escalated` | `escalate` hoặc job SLA |
| `escalated` | `in_review` | trưởng ca nhận |
| `resolved` | `reopened` | `reopen` (kháng nghị, bằng chứng mới) |
| `reopened` | `in_review` | gán lại |

Sanction: hiệu lực từ `starts_at` → hết `ends_at` hoặc `revoked_at`.

## 3. Quy tắc kiểm tra dữ liệu

- Gộp case: cùng `(tenant_id, target_type, target_id)` có case `open`/`in_review` tạo trong 24 h.
- `restrict_multiplayer` > 7 ngày, `account_lock` → actor có `moderation.case.lead` hoặc `system_admin`.
- `student_ids` của thông báo ⊆ `subject_user_ids` ∩ học sinh của `school_id`; tenant `common` không có thông báo.
- `summary`/`note` không được chứa nội dung tin (quy trình mẫu tóm tắt, review ở F122); server không echo tin vào thông báo.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `report.created` | Tạo report | `specs/events/payloads/chat/report.created.v1.schema.json` (Có — F121) |
| `moderation.case_assigned` | Gán case | `…/moderation.case_assigned.v1.schema.json` (Có — F121) |
| `moderation.school_notified` | Gửi thông báo | `…/moderation.school_notified.v1.schema.json` (Có — F121) |
| `moderation.school_acknowledged` | Trường xác nhận | `…/moderation.school_acknowledged.v1.schema.json` (Có — F121) |
| `sanction.applied` | Áp sanction thủ công/theo case | `…/sanction.applied.v1.schema.json` — owner F116; F121 mở rộng theo EVENT_DESIGN §4.9 (`sanction_type`, `until`, `reason_code`, `issued_by_kind`, `source_version`; bỏ `type`/`ends_at`/`reason` nội bộ) |
| `sanction.lifted` | Gỡ/hết hạn | `…/sanction.lifted.v1.schema.json` (Có — F121) |
| `chat.message_removed` | Gỡ tin | `…/chat.message_removed.v1.schema.json` (Có — F121) |
| `report.resolved` | Case `resolved`: mỗi người báo cáo một event, F123 tạo thông báo `moderation_notice` (F121-AC-16) | `…/report.resolved.v1.schema.json` (Có — F121) |

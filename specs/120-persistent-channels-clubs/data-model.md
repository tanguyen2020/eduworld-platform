# Data Model: F120 — Channel bền vững (world, area, team, DM), club, chính sách theo cấp, tìm kiếm, retention

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Mọi bảng dưới đây: mixin `Base` (UUIDv7 `id`), `tenant_id uuid NOT NULL` + RLS, `created_at/updated_at`, `deleted_at/deleted_by` (trừ ghi chú append-only), interceptor `src/backend/pkg/entx` (ADR-0039). Chuỗi `MaxLen` theo ADR-0041.

## 1. Entity

### Channel — `chat.channels` (Có sẵn · Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `channel_id` | uuid | PK | |
| `channel_type` | `varchar(16)` | CHECK enum 6 loại `world/area/match/party/team/direct` (ADR-0095: `class`/`study_group` bỏ khỏi CHECK khi không còn dòng — ADR-0095 §8, owner lõi F115) | F120 dùng `world`, `area`, `team` (cố định, club), `direct` |
| `is_gameplay` | bool | CHECK DATABASE_DESIGN §2.12 | `false` cho mọi channel của F120 |
| `scope_ref` | jsonb | | `{club_id}` · `{school_id}` hoặc `{tenant_id}` (world) · `{room_id}` (area) · `{user_low_id,user_high_id}` (direct) |
| `school_id` | uuid | NULL với world toàn tenant/direct liên trường | |
| `policy_id` | uuid | | F120 không dùng (không còn policy dưới cấp trường — ADR-0095); effective policy tính theo trường/tenant của channel |
| `retention_days` | int | | từ cấu hình (Assumptions spec) |
| `status` | `varchar(16)` (`MaxLen(16)`) | **Thêm cột** CHECK `open/read_only/frozen` | CHAT_SYSTEM §6 có `status`; `archived` (study group bị xóa) bỏ theo ADR-0095 [Proposed] |
| `capabilities` | jsonb | **Thêm cột**, CHECK `(capabilities->>'voice')::bool IS NOT TRUE` | FR-CHAT-007 |
| `last_seq`, `opened_at`, `closed_at`, `purge_after` | | | có sẵn |

**Index**: UNIQUE `(tenant_id, channel_type, scope_ref) WHERE deleted_at IS NULL AND NOT is_gameplay` (một channel/phạm vi) — **Mới**.

### ChannelMember — `chat.channel_members` (Có sẵn · Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `channel_id`, `user_id` | uuid | PK | |
| `school_id`, `role` (`varchar(16)`), `muted_until`, `last_delivered_seq`, `last_read_seq`, `unread_mentions`, `show_read_receipts`, `notification_level` (`varchar(16)`) | | | có sẵn |
| `joined_at` | timestamptz | **Thêm cột** NOT NULL | quyền xem từ `joined_at` |
| `left_at` | timestamptz | **Thêm cột** | rời/bị gỡ khỏi club |
| `source_event_id` | uuid | **Thêm cột** | truy vết lệnh/event gây thay đổi thành viên (club, `user.blocked`) |

### Message — `chat.messages` (Có sẵn · Thêm cột) · `chat.message_revisions` (Có sẵn) · `chat.ephemeral_messages` (Có sẵn)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `anonymized_at` | timestamptz | **Thêm cột** trên `chat.messages` | retention; API lọc `anonymized_at IS NULL` |
| `search_tsv` | tsvector generated | GIN | có sẵn (DDL §3.8) |

### ChatPolicy — `chat.chat_policies` (Có sẵn · Thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `policy_id` | uuid | PK | |
| `tenant_id` | uuid | NULL = platform (bảng T*) | RLS cho phép đọc dòng platform |
| `scope_type` | `varchar(16)` | CHECK `platform/tenant/school` | bỏ `class` (ADR-0095, FR-MOD-004) |
| `scope_id` | uuid | NULL khi platform | |
| `school_id` | uuid | | scope school |
| `rules` | jsonb | validate `chat-policy.schema.json` v5 ở use case; CHECK `octet_length(rules::text) <= 65536` | |
| `version` | int | `If-Match` | |
| `reason` | `varchar(500)` | **Thêm cột** NOT NULL | |
| `updated_by` | uuid | **Thêm cột** | |

**Index**: UNIQUE `(coalesce(tenant_id,'00000000-0000-0000-0000-000000000000'), scope_type, coalesce(scope_id,'00000000-0000-0000-0000-000000000000')) WHERE deleted_at IS NULL`.

### PersistentRetentionRun — `chat.persistent_retention_runs` (Mới, G append-only)

`run_id uuid` PK, `started_at`, `finished_at`, `tenant_id`, `channels_scanned int`, `messages_anonymized int`, `evidence_copied int`, `status varchar(16)` (`running/succeeded/failed`), `error_code varchar(64)`. Không chứa nội dung. Cập nhật DATABASE_DESIGN §2.12 (task T050).

### Club — `social.clubs`, `club_members`, `club_join_requests`, `club_invitations`, `club_goals`, `club_goal_progress`, `club_events` (Có sẵn — DATABASE_DESIGN §2.11)

Cột và độ dài giữ nguyên DATABASE_DESIGN §2.11 (`name varchar(64)`, `description varchar(500)`, `icon_id varchar(64)`, `club_type varchar(32)`, `status varchar(32)`, `title varchar(128)`, `metric varchar(64)`, `description varchar(1000)` cho event…). Thay đổi:

- `clubs.club_type` CHECK bỏ `study_group` (Cần sửa theo ADR-0084/ADR-0095; bảng mới chưa có dữ liệu nên đổi trực tiếp trong migration đầu). Không có cột/lựa chọn thêm cả lớp (ADR-0095).
- `clubs.chat_channel_id` → channel `team` cố định.
- `club_goal_progress`: thêm `last_event_id uuid` (Thêm cột) để idempotent theo event cùng `infra.processed_events`.

### Projection — `ref.students`, `ref.schools` (Có sẵn §2.17)

`ref.schools` cần thêm cột `student_club_creation varchar(32)` từ `school.updated` (Thêm cột).

## 2. Vòng đời / trạng thái

| Entity | Chuyển trạng thái |
|---|---|
| Channel bền vững | `open` → `read_only` (club khóa/lưu trữ) · `open` ↔ `frozen` (direct, block/unblock) |
| ChannelMember | thêm (`joined_at`) → rời (`left_at`); vào lại tạo dòng hiệu lực mới (cập nhật `joined_at`, xóa `left_at`) |
| Club | `pending_approval` → `active` → `locked` ↔ `active` → `archived` |
| Message (bền vững) | bình thường → `anonymized_at` (retention) |

## 3. Quy tắc kiểm tra dữ liệu

- Rule policy không có khóa `grade_band`/`grade_level`; always-on theo [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md); cấp dưới chỉ siết chặt.
- Thành viên channel chỉ là học sinh (`ref.students`), cùng tenant; club liên trường chỉ khi tenant `unit` bật (ADR-0054).
- ≤ `clubs_per_student_max` club active/học sinh, ≤ `max_members` thành viên/club (kiểm trong transaction, `SELECT … FOR UPDATE` trên club).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `chat_policy.updated` | Lưu policy | `specs/events/payloads/chat/chat_policy.updated.v1.schema.json` (Cần thêm) |
| `club.created` / `club.approved` / `club.archived` | Vòng đời club | `specs/events/payloads/chat/club.*.v1.schema.json` (Cần thêm) |
| `club.member_joined` / `club.member_left` | Thành viên đổi | như trên |
| `club.goal_completed` | Đạt `target` | như trên |

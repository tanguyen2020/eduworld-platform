# Data Model: F117 — Chat gameplay luôn có (`match`/`team`/`party`) và purge chat gameplay

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§1.1, §2.12, §2.16, §2.17, §3.0, §3.8b)

Database `eduworld_chat`. Bảng `chat.channels`, `chat.channel_members`, `chat.gameplay_messages`, `chat.gameplay_partition_channels` do F115 tạo; F117 dùng thêm các cột vòng đời (`is_gameplay`, `status`, `closed_at`, `purge_after`).

## 1. Entity

### PartyProjection — `ref.parties` (Có sẵn, projection)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `party_id` | uuid | PK | |
| `tenant_id` | uuid | NOT NULL | |
| `member_ids` | `uuid[]` | CHECK `cardinality ≤ 8` | |
| `status` | `varchar(64)` | `open/in_activity/disbanded` | |
| `source_version` | bigint | | chỉ áp bản mới hơn |

### GameplayChatDailyStats — `chat.gameplay_chat_daily_stats` (Có sẵn, T)

`tenant_id`, `day date`, `channel_type varchar(64)`, `channels int`, `messages int`, `masked int`, `blocked int`, `reported int`; PK `(tenant_id, day, channel_type)`. Rollup trước purge, giữ lâu dài.

### GameplayPurgeRun — `chat.gameplay_purge_runs` (Có sẵn, G append-only)

`run_id`, `started_at`, `finished_at`, `tenant_id`, `kind varchar(64)` CHECK `gameplay/ephemeral`, `channels_purged int`, `messages_purged bigint`, `partitions_dropped int`, `deferred_open_case int`, `evidence_copied int`, `status varchar(64)` CHECK `succeeded/partial/failed/blocked` (giá trị `blocked` **Mới** — R3 research).

### ModerationEvidence — `chat.moderation_evidence` (Có sẵn, T, bất biến; migration F121 tạo bảng và ghi snapshot report, F117 ghi thêm bản sao trước purge + grant `chat_purger`)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `evidence_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id` | uuid | NOT NULL `tenant_id` | |
| `case_id` | uuid | | NULL khi chỉ có report/sanction lúc sao (F121 gắn sau) |
| `source_ref` | jsonb | CHECK `octet_length ≤ 2048` | **Mới**: `{report_id?, case_id?, sanction_id?}` để biết lý do giữ |
| `snapshot` | jsonb | CHECK `octet_length ≤ 65536` | tin + revision + ±`evidence_context` tin, `sender_id`, `created_at`, `channel_type`, `match_id`/`party_id`, `catalog_version`, kết quả filter |
| `sha256` | bytea | CHECK `octet_length = 32` | |
| `created_at` | timestamptz | | |

Không UPDATE/DELETE (trigger); không bị purge; lượt xem ghi audit (F121).

### EphemeralMessage — `chat.ephemeral_messages` (Có sẵn, T, P ngày `created_at`)

`message_id`, `channel_id`, `tenant_id`, `school_id`, `sender_id`, `kind varchar(64)`, `body varchar(1000)` (đã kiểm tra/che), `quick_message_id varchar(64)`, `moderation_state varchar(64)`, `created_at`. F117 tạo bảng + purge; ghi tin ở F120.

### Cột dùng trên `chat.channels`

`is_gameplay`, `scope_ref` (`{party_id}`, `{match_id}`, `{match_id, team_id}`), `status`, `opened_at`, `closed_at`, `purge_after`. Index `(purge_after) WHERE is_gameplay AND closed_at IS NOT NULL`.

## 2. Vòng đời / trạng thái

```text
[*] → Open (party.created / match.started)
Open → Open (member_joined/left, reconnect)
Open → Closed (match.completed / match.aborted / party.disbanded; closed_at, purge_after)
Closed → Closed (report/case mở → hoãn; module moderation chưa sẵn sàng → blocked)
Closed → Purged (job gameplay-chat-purge, role chat_purger)
```

## 3. Quy tắc kiểm tra dữ liệu

- `purge_after = closed_at + max(gameplay_chat_retention_days hiệu lực, 3 ngày)`.
- Role `chat_purger`: `SELECT, UPDATE, DELETE` chỉ trên `chat.gameplay_messages`, `chat.channels`, `chat.channel_members`, `chat.ephemeral_messages`, `chat.message_revisions`, `chat.message_reactions`, `chat.pinned_messages`, `chat.gameplay_partition_channels`; `INSERT` trên `chat.moderation_evidence`, `chat.gameplay_chat_daily_stats`, `chat.gameplay_purge_runs`, `infra.outbox_events`; RLS `purger_gameplay_only` (DATABASE_DESIGN §3.0).
- Xóa revision/reaction/pin chỉ của `message_id` thuộc channel gameplay đã hết hạn (điều kiện trong query).
- Validator always-on: rule cấp `tenant/school/class` cho `match/team/party` phải `enabled = true`, đủ 5 `allowed_kinds`, không `allowed_hours`, `slow_mode_seconds ≤ 10`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `chat.channel_opened` | Tạo/đổi membership channel gameplay | `specs/events/payloads/chat/chat.channel_opened.v1.schema.json` (Mới) |
| `chat.channel_closed` | Đóng channel (`closed_at`, `purge_after`) | `specs/events/payloads/chat/chat.channel_closed.v1.schema.json` (Mới) |
| `chat.gameplay_purged` | Kết thúc lượt purge | `specs/events/payloads/chat/chat.gameplay_purged.v1.schema.json` (Mới) |
| `chat.daily_stats` | Rollup ngày | `specs/events/payloads/chat/chat.daily_stats.v1.schema.json` (Mới) |
| JetStream `ew.chat.<tenant_id>.cursor.<channel_id>` | Con trỏ đọc gộp | `specs/events/payloads/chat/chat-cursor.v1.schema.json` (Mới) |

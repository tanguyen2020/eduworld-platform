# Data Model: F116 — Pipeline kiểm tra tin nhắn, danh mục từ nhạy cảm, công tắc kiểm duyệt

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§1.4, §2.12) · Thuộc tính: [MODERATION §7, §11, §12](../../docs/03-social-multiplayer/MODERATION.md)

Database `eduworld_chat`, schema `chat`. Độ dài chuỗi theo DATABASE_DESIGN §1.4 (mã/enum `varchar(64)`, tên `varchar(128|255)`, `term`/`phrase` `varchar(255)`, lý do/ghi chú/mô tả `varchar(1000)`).

## 1. Entity

### SensitiveTermCategory — `chat.sensitive_term_categories` (Có sẵn, H; CHECK `owner_scope = 'platform'`)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `category_id` | UUIDv7 | PK | |
| `code` | `varchar(64)` | UNIQUE `(code) WHERE tenant_id IS NULL AND deleted_at IS NULL` | `profanity`, `insult`, `violence`, `sexual`, `drugs`, `self_harm`, `grooming_scam`, `politics_religion`, `pii_pattern`… |
| `name` | `varchar(128)` | NOT NULL | |
| `description` | `varchar(1000)` | | |
| `severity` | `varchar(64)` | CHECK `watch/low/medium/high/critical` | |
| `default_action` | `varchar(64)` | CHECK `allow/mask/block/hold_for_review/block_and_alert` | |
| `enabled`, `allow_tenant_toggle` | bool | NOT NULL | |
| `created_by`, `updated_at`, `deleted_at`, `deleted_by` | | | R21 |

### SensitiveTerm — `chat.sensitive_terms` (Có sẵn, H)

`term_id` PK; `tenant_id` (NULL = platform); `owner_scope`; `category_id` FK; `term varchar(255)`; `match_type varchar(64)` CHECK `word/phrase/contains/pattern`; `locale varchar(64)` CHECK `vi/en`; `variants varchar(255)[]` CHECK `cardinality ≤ 50`; `severity_override`, `action_override varchar(64)`; `enabled`; `status varchar(64)` CHECK `draft/published`; `note varchar(1000)`; `created_by`; `published_in_version bigint`; `deleted_at`, `deleted_by`. UNIQUE `(coalesce(tenant_id, '0…0'), category_id, lower(term), match_type) WHERE deleted_at IS NULL`. CHECK `match_type <> 'pattern' OR char_length(term) <= 200`. Nội dung chỉ đọc qua query có `chat.sensitive_terms.read`.

### SensitiveAllowlistEntry — `chat.sensitive_allowlist` (Có sẵn, H)

`entry_id` PK; `tenant_id?`; `owner_scope`; `phrase varchar(255)`; `locale varchar(64)`; `reason varchar(1000)`; `deleted_at`, `deleted_by`. UNIQUE `(coalesce(tenant_id,…), lower(phrase)) WHERE deleted_at IS NULL`.

### SensitiveCatalogVersion — `chat.sensitive_catalog_versions` (Có sẵn, H append-only)

`catalog_version bigint`, `scope varchar(64)` CHECK `platform/tenant`, `tenant_id?`, `published_at`, `published_by`, `change_summary varchar(1000)`, `term_count int`, `checksum varchar(128)`, `regression_recall numeric(5,4)`, `regression_fp_rate numeric(5,4)`. PK `(coalesce(tenant_id,…), catalog_version)`. Không UPDATE/DELETE (trigger chặn).

### TenantCategorySetting — `chat.tenant_category_settings` (Có sẵn, T)

`tenant_id`, `category_id`, `enabled`, `reason varchar(1000)`, `updated_by`, `updated_at`. UNIQUE `(tenant_id, category_id) WHERE deleted_at IS NULL`. Ghi chỉ khi category `allow_tenant_toggle`.

### ModerationSwitch — `chat.moderation_switches` (Có sẵn, H)

`switch_id` PK; `scope varchar(64)` CHECK `platform/tenant`; `tenant_id` (NULL = platform); `enabled_all bool`; `steps jsonb` (`sensitive_terms, spam, bullying, async_analysis` — bool, CHECK `octet_length ≤ 1024`; khóa `pii` cũ bị bỏ qua và migrate về bật — ADR-0078); `reason varchar(1000)`; `expires_at` (bắt buộc khi có bước tắt, ≤ `moderation_switch_max_duration`); `updated_by`, `updated_at`. UNIQUE `(coalesce(tenant_id,…)) WHERE deleted_at IS NULL`; CHECK có bước tắt ⇒ `reason IS NOT NULL`.

### ModerationSwitchRequest — `chat.moderation_switch_requests` (Mới, ADR-0078)

`request_id` PK; `scope varchar(64)` CHECK `platform/tenant`; `tenant_id?`; `requested jsonb` (CHECK `octet_length ≤ 1024`); `reason varchar(1000)`; `expires_at` (thời hạn tắt); `status varchar(64)` CHECK `pending_approval/approved/rejected/expired`; `proposed_by`, `approved_by?` (CHECK `approved_by <> proposed_by`); `created_at`, `decided_at?`; yêu cầu `pending_approval` hết hạn sau 24 giờ. Chỉ yêu cầu **tắt** qua bảng này; bật lại ghi thẳng `moderation_switches`.

### ModerationSwitchChange — `chat.moderation_switch_changes` (Có sẵn, TA/G append-only)

`change_id`, `tenant_id?`, `before jsonb`, `after jsonb`, `reason varchar(1000)`, `expires_at`, `actor_id`, `changed_at`, `source varchar(64)` CHECK `manual/auto_expire`.

### ModerationDecision — `chat.moderation_decisions` (Có sẵn, TA, P tháng `created_at`)

`decision_id`, `tenant_id`, `channel_id`, `message_id?`, `sender_id`, `decision varchar(64)` CHECK `allow/unchecked/mask/block/hold_for_review/block_and_alert`, `category_codes varchar(64)[]` (CHECK ≤ 20), `steps_run varchar(64)[]`, `steps_skipped varchar(64)[]`, `catalog_version_platform bigint`, `catalog_version_tenant bigint`, `switch_snapshot_id uuid`, `latency_ms int`, `created_at`. Không cột nội dung.

### Sanction — `chat.sanctions` (Có sẵn trong DATABASE_DESIGN §2.12, cột theo MODERATION §7; độ dài **Mới**, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `sanction_id` | UUIDv7 | PK | |
| `tenant_id`, `school_id`, `user_id` | uuid | NOT NULL (trừ `school_id`) | |
| `type` | `varchar(64)` | CHECK `warning/mute/restrict_text/restrict_dm/restrict_multiplayer/restrict_competition/account_lock` | F116 chỉ tạo `mute` |
| `scope` | `varchar(64)` | CHECK `channel/all` | |
| `channel_id` | uuid | khi `scope = channel` | |
| `reason` | `varchar(1000)` | | `auto_spam`, `block_and_alert_hold` |
| `case_id` | uuid | | F121 |
| `issued_by` | uuid | NULL = hệ thống | |
| `starts_at`, `ends_at`, `revoked_at` | timestamptz | | |

**Index**: `(tenant_id, user_id, ends_at) WHERE revoked_at IS NULL AND deleted_at IS NULL`. Cache Valkey cho kiểm tra nhanh (key **Mới** `ew:chat:sanction:<user_id>`, TTL = `ends_at`).

### QuickMessage — `chat.quick_messages` (Có sẵn, G)

`quick_message_id varchar(64)` PK (khớp contract string 64), `category varchar(64)` CHECK theo enum `QuickMessage.category`, `text_vi varchar(255)`, `text_en varchar(255)` NOT NULL (ADR-0056), `emoji varchar(64)`, `sort_order int`, `status varchar(64)` CHECK `active/retired`, `deleted_at`. DATABASE_DESIGN chưa ghi kiểu khóa → cập nhật (task Polish).

## 2. Vòng đời / trạng thái

- Term/allowlist: `draft` → (publish) `published`; xóa mềm có hiệu lực ở version kế tiếp.
- Catalog version: chỉ thêm; version hiệu lực = lớn nhất theo scope.
- Switch: `on` ↔ `off` (lý do bắt buộc khi off) → `auto_expire` về `on` tại `expires_at`.
- Sanction: tạo → hết hạn (`ends_at`) hoặc thu hồi (`revoked_at`, F121).

## 3. Quy tắc kiểm tra dữ liệu

- Category platform-only; tenant chỉ thêm term vào category có sẵn ([MODERATION §11.3](../../docs/03-social-multiplayer/MODERATION.md)).
- Pattern RE2 ≤ 200 ký tự, cấm backreference/lookaround; ≤ 200 pattern/tenant [Proposed].
- Seed/fixture chỉ dùng term placeholder (`__test_term_01__`), không từ nhạy cảm thật (R15).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `sensitive_term.created` / `.updated` / `.deleted` | Ghi term (platform/tenant) | `specs/events/payloads/chat/sensitive_term.created.v1.schema.json`, `sensitive_term.updated.v1.schema.json`, `sensitive_term.deleted.v1.schema.json` (Mới) |
| `sensitive_catalog.published` | Publish version | `specs/events/payloads/chat/sensitive_catalog.published.v1.schema.json` (Mới) |
| `chat_policy.moderation_switch_changed` | Đổi công tắc (manual/auto_expire) | `specs/events/payloads/chat/chat_policy.moderation_switch_changed.v1.schema.json` (Mới) |
| `chat_policy.tenant_category_toggled` | Bật/tắt category tenant | `specs/events/payloads/chat/chat_policy.tenant_category_toggled.v1.schema.json` (Mới) |
| `chat.message_held` | Tin `held` được persist | `specs/events/payloads/chat/chat.message_held.v1.schema.json` (Mới) |
| `sanction.applied` | Auto-mute hệ thống | `specs/events/payloads/chat/sanction.applied.v1.schema.json` (Mới) — owner F116; F121 thêm `case_id` tương thích |

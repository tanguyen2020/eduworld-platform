# Data Model: F026 — Phát hành nội dung: publish/rollback/unpublish, snapshot, manifest, ghim version, nơi sử dụng

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

### ContentItem — bảng định danh item (Thêm cột, DATABASE_DESIGN §3.13)

Áp cho `content.questions`, `content.question_sets`, `content.assessment_blueprints`, `content.lessons`, `content.learning_materials`, `content.activities`, `lms.learning_paths`:

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `current_version_id` | uuid NULL | FK version của chính item; trigger `check_release_target` | NULL = chưa/không publish |
| `latest_version_no` | int | NOT NULL DEFAULT 0 | |

### ContentVersion — bảng `*_versions` (Thêm cột, §3.13)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `change_note` | `varchar(500)` | | |
| `publish_at` | timestamptz NULL | | bản sao lịch hẹn đang chờ (đọc nhanh) |
| `snapshot_key` | `varchar(512)` | | Valkey/CDN key |
| `snapshot_sha256` | `varchar(64)` | | |
| `based_on_version_id` | uuid NULL | | |
| `published_by` | uuid NULL | | Mới (`ContentVersionSummary.published_by`) |

Trigger `forbid_published_update` (§3.3) gắn cho mọi bảng `*_versions`; partial unique `(item_id) WHERE status IN ('draft','in_review') AND deleted_at IS NULL` (một bản mở/item, Mới [Confirmed]).

### ContentRelease — `content.content_releases` (Có sẵn §3.13, append-only)

Cột theo DDL §3.13: `release_id`, `tenant_id` NULL, `item_type varchar(64)`, `item_id`, `action varchar(16)` (`publish`/`rollback`/`unpublish`), `from_version_id`, `to_version_id`, `reason varchar(500)`, `scheduled_for`, `released_by`, `released_at`, `idempotency_key varchar(64)`; UNIQUE `(item_type, item_id, idempotency_key)`; index `(item_type, item_id, released_at DESC)`. Role app chỉ `INSERT/SELECT`.

### ScheduledPublication — `content.scheduled_publications` (Mới)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `schedule_id` | UUIDv7 | PK | trả là `release_id` trong `202 ContentRelease` |
| `tenant_id` | uuid NULL | RLS | NULL = platform |
| `item_type` | `varchar(64)` | | |
| `item_id`, `version_id` | uuid | NOT NULL | |
| `publish_at` | timestamptz | NOT NULL | |
| `status` | `varchar(64)` | CHECK IN (`scheduled`,`applied`,`cancelled`,`failed`) | |
| `failure_code` | `varchar(64)` | | mã guard lỗi |
| `applied_release_id` | uuid NULL | | |
| `change_note` | `varchar(500)` | | |
| `requested_by`, `cancelled_by` | uuid / uuid NULL | | |
| `idempotency_key` | `varchar(64)` | UNIQUE `(item_type, item_id, idempotency_key)` | |
| `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `row_version` | | partial unique `(item_type, item_id) WHERE status='scheduled' AND deleted_at IS NULL` | một lịch chờ/item |

**Index**: `(status, publish_at) WHERE status = 'scheduled'`.

### ContentManifest — `game.content_manifests` (Có sẵn §3.13, sở hữu module game)

F026 chỉ ghi `entries` mục `item_type ∈ LearningContentType`: `{item_type, item_id, version_id, version_no, snapshot_url?, sha256, activity_type?, min_client_version?}`; `cause_release_id` = `release_id` của learning.

### Valkey (cache, không phải nguồn sự thật)

| Key | Giá trị | TTL |
|---|---|---|
| `ew:learning:cv:<item_type>:<item_id>:<version_no>` | snapshot JSON nén + sha256 | trượt (`content_snapshot_cache_ttl`, đăng ký mới) |
| `ew:learning:current:<tenant_id \| _platform>:<item_type>:<item_id>` | `version_id` | theo VALKEY_DESIGN |

## 2. Vòng đời / trạng thái

| Từ | Thao tác | Tới | Con trỏ |
|---|---|---|---|
| `approved` | publish (ngay) | `published` | = version |
| `approved` | publish (hẹn giờ) | `approved` + lịch `scheduled` → `published` khi áp dụng | đổi khi áp dụng |
| `published` | rollback/roll forward | `published` | = version đích |
| (item) | unpublish | — | NULL |
| `published` (≠ con trỏ) | archive | `archived` | không đổi |

## 3. Quy tắc kiểm tra dữ liệu

- [CONTENT_MODEL §5, §6](../../docs/02-learning/CONTENT_MODEL.md): con trỏ chỉ trỏ version `published` của chính item; rollback/unpublish bắt buộc lý do; archive không phải con trỏ.
- `CHECK (action = 'publish' OR reason IS NOT NULL)`, `CHECK ((action = 'unpublish') = (to_version_id IS NULL))` (§3.13).
- Snapshot: danh sách trường bị loại = F026-AC-12; mỗi loại nội dung có `Strip` riêng và test hợp đồng.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `content.published` | publish ngay hoặc áp dụng lịch | `specs/events/payloads/learning/content.published.v1.schema.json` (Cần sửa: thêm `min_client_version`, `snapshot_url`, `snapshot_sha256`, `cdn_eligible`) |
| `content.rolled_back` | rollback | Cần thêm `content.rolled_back.v1.schema.json` |
| `content.unpublished` | unpublish | Cần thêm `content.unpublished.v1.schema.json` |
| `content.publish_scheduled` | tạo lịch | Cần thêm `content.publish_scheduled.v1.schema.json` |
| `content.version_archived` | archive | Cần thêm `content.version_archived.v1.schema.json` |
| `content.manifest_updated` (game, phát từ consumer manifest của F026) | Manifest tenant dựng lại | Cần thêm `specs/events/payloads/game/content.manifest_updated.v1.schema.json` — owner F026 (F078 phát cùng schema) |

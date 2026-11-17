# Data Model: F025 — Workflow duyệt nội dung, song ngữ, lồng tiếng `g1_3`

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

### ReviewTask — `content.review_tasks` (Có sẵn §2.5 — Thêm cột chi tiết)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `task_id` | UUIDv7 | PK | mixin `HybridContent` (tên cũ `PlatformBase`, ADR-0089) |
| `tenant_id` | uuid NULL | NULL ⇔ lane `platform_learning`; RLS | R8, ADR-0029 |
| `review_lane` | `varchar(64)` | CHECK IN (`tenant`,`platform_learning`) | Mới (ADMIN_PORTAL §4) |
| `content_type` | `varchar(64)` | enum `LearningContentType` | |
| `content_id`, `content_version_id` | uuid | NOT NULL | |
| `version` | int | NOT NULL | số version |
| `school_id` | uuid NULL | | lọc reviewer scope trường |
| `author_id`, `assignee_id` | uuid / uuid NULL | | |
| `status` | `varchar(64)` | CHECK IN (`open`,`approved`,`rejected`,`cancelled`) | |
| `comments` | jsonb | CHECK `octet_length ≤ 65536` | `[{path, text ≤ 2000, by, at}]` |
| `submitted_at`, `due_at`, `resolved_at` | timestamptz | | `due_at` theo `content_review_sla_days` (đăng ký mới) |
| `resolved_by` | uuid NULL | | |
| `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `row_version` (optimistic ent) | | partial unique `(content_version_id) WHERE status='open' AND deleted_at IS NULL` | một task mở/version |

**Index**: `(tenant_id, review_lane, status, submitted_at)`; `(assignee_id, status)`.

### ContentTransition — `content.content_transitions` (Có sẵn §2.5, append-only)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `transition_id` | UUIDv7 | PK | mixin `AppendOnly` |
| `tenant_id` | uuid NULL | RLS | NULL = platform |
| `content_type` | `varchar(64)` | | |
| `content_id`, `content_version_id` | uuid | NOT NULL | |
| `action` | `varchar(64)` | `submit_review`/`approve`/`reject`/`revise` (F026 thêm `publish`/`archive`/…) | Mới |
| `from_status`, `to_status` | `varchar(64)` | | |
| `actor_id` | uuid | NOT NULL | |
| `note` | `varchar(2000)` | | khớp `maxLength 2000` của body |
| `idempotency_key` | `varchar(64)` | UNIQUE `(content_version_id, idempotency_key)` | Mới |
| `at` | timestamptz | NOT NULL | |

### Cột thêm vào mọi bảng `*_versions` của `LearningContentType` (Thêm cột)

Áp cho `content.question_versions`, `question_set_versions`, `assessment_blueprint_versions`, `lesson_versions`, `learning_materials` (version), `content.activity_versions`, `lms.learning_path` versions:

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `content_language` | `varchar(64)` | NOT NULL CHECK IN (`vi`,`en`) | CONTENT_MODEL §3 (nếu F020 chưa thêm) |
| `translation_status` | `varchar(64)` | CHECK IN (`complete`,`partial`,`outdated`) | tính khi lưu |
| `translation_meta` | jsonb | CHECK `octet_length ≤ 65536` | `{pointer: {source_sha256, translated_at}}` |

Chỉ `question_versions`, `activity_versions`, `lesson_versions`: `audio_exempt_reason varchar(255) NULL`, `audio_exempt_approved_by uuid NULL` (Mới).

### TranslationJob — `content.translation_jobs` (Mới)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `job_id` | UUIDv7 | PK | |
| `tenant_id` | uuid NULL | RLS; CHECK với `owner_scope` | |
| `owner_scope` | `varchar(64)` | `platform`/`tenant` | |
| `kind`, `status`, `format` | `varchar(64)` | `export`/`import`; `queued`/`running`/`succeeded`/`failed`; `xliff`/`csv` | |
| `source_language`, `target_language` | `varchar(64)` | `vi`/`en` | |
| `request` | jsonb | ≤ 65536 B | `content_types[]`, `content_ids[]`, `only_missing`, `dry_run` |
| `input_asset_id` | uuid NULL | | file import (media `document`) |
| `result_storage_key` | `varchar(512)` | | `ew-exports/translations/<job_id>.<ext>` |
| `report` | jsonb | ≤ 65536 B | `total`, `updated`, `skipped`, `errors[≤500]` |
| `requested_by` | uuid | | |
| `created_at`, `finished_at`, `deleted_at`, `deleted_by` | | | |

## 2. Vòng đời / trạng thái

| Từ | Action | Tới | Điều kiện |
|---|---|---|---|
| `draft` | `submit_review` | `in_review` | `PublishReadinessGuard` đạt (schema, media `ready`, audio `g1_3`, bản `content_language`) |
| `in_review` | `approve` | `approved` | reviewer ≠ author, đúng lane/scope; miễn trừ audio → `audio_exemption_acknowledged` |
| `in_review` | `reject` | `rejected` | `note` bắt buộc |
| `rejected`, `approved` | `revise` | `draft` | tác giả hoặc người có `*.write` |

`approved → published`, `published → archived`: F026.

## 3. Quy tắc kiểm tra dữ liệu

- Bảng chuyển trạng thái và điều kiện: [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md); audio: §8.1; ngôn ngữ: §8.
- Version `published`/`archived` bất biến (trigger `content.forbid_published_update`, DATABASE_DESIGN §3.3).
- `review_lane = tenant` ⇔ `tenant_id IS NOT NULL`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `content.version_submitted` | `submit_review` thành công | Cần thêm `specs/events/payloads/learning/content.version_submitted.v1.schema.json` |
| `content.version_approved` | `approve` | Cần thêm `content.version_approved.v1.schema.json` |
| `content.version_rejected` | `reject` | Cần thêm `content.version_rejected.v1.schema.json` |
| `audit.recorded` | mọi transition, export/import | envelope chung ([EVENT_DESIGN §2](../../docs/07-backend/EVENT_DESIGN.md)) |

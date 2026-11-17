# Data Model: F027 — Media pipeline, công thức SVG, preview session

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

### MediaAsset — `content.media_assets` (Có sẵn §3.13 — Thêm cột)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `asset_id` | UUIDv7 | PK | mixin `HybridContent` (tên cũ `PlatformBase`, ADR-0089) |
| `tenant_id` | uuid NULL | CHECK `(owner_scope='platform') = (tenant_id IS NULL)`; RLS | R8 |
| `owner_scope` | `varchar(64)` | `platform`/`tenant` | |
| `kind` | `varchar(16)` | `image`,`audio`,`video`,`model3d`,`document` | |
| `original_filename` | `varchar(255)` | NOT NULL | |
| `mime_type` | `varchar(127)` | NOT NULL | |
| `size_bytes` | bigint | > 0 | |
| `width`, `height`, `duration_ms` | int NULL | | |
| `sha256` | `varchar(64)` | NOT NULL | partial unique `(coalesce(tenant_id,…), sha256) WHERE deleted_at IS NULL` |
| `bucket` | `varchar(64)` | `ew-media-public`/`ew-media-private` | |
| `variants` | jsonb | CHECK ≤ 16384 B | `{variant: {key, bytes, mime}}` |
| `alt_text` | `varchar(500)` | bắt buộc với `image` (use case) | |
| `license` | `varchar(255)` | NOT NULL | D17 đã chốt (DATABASE_DESIGN §3.13, CONTENT_MODEL §7) |
| `status` | `varchar(16)` | `quarantined`,`processing`,`ready`,`rejected` | |
| `reject_reason` | `varchar(255)` | | `virus`/`type_mismatch`/`too_large`/`transcode_failed` |
| `quarantine_key` | `varchar(512)` | NOT NULL | Mới — `quarantine/<asset_id>` |
| `is_exam_material` | bool | DEFAULT false | Mới — buộc `ew-media-private` |
| `upload_completed_at`, `processed_at` | timestamptz NULL | | Mới |
| `uploaded_by` | uuid | NOT NULL | |
| `created_at`, `updated_at`, `deleted_at`, `deleted_by` | | | R21 |

### MediaUsage — `content.media_usages` (Có sẵn §3.13)

`asset_id` FK, `item_type varchar(64)`, `version_id uuid`; PK `(asset_id, item_type, version_id)`. Thêm cột `tenant_id uuid NULL` (Mới, lọc `used_in` theo tenant + RLS).

### MathSvg — `content.math_svgs` (Có sẵn §2.6a, G)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `sha256` | `char(64)` | PK | hash SVG đã sanitize |
| `storage_key` | `varchar(128)` | NOT NULL | `math/<sha256>.svg` |
| `latex` | `varchar(2000)` | NOT NULL | `latex_max_length` |
| `speech_text` | `varchar(1000)` | | theo ngôn ngữ của khối |
| `size_bytes` | int | ≤ `math_svg_max` | |
| `sanitized_at` | timestamptz | | |

### PreviewSession — Valkey `ew:learning:preview:<preview_session_id>` (không bền vững)

`{tenant_id, principal_id, content_type, content_id, version, display_language, items: [{position, question_version_id}]}`; TTL = `preview_token_ttl` (đăng ký mới). Không phải dữ liệu quan trọng (R9); mất Valkey → tạo phiên mới.

## 2. Vòng đời / trạng thái

| Từ | Sự kiện | Tới |
|---|---|---|
| (mới) | `presignMediaUpload` | `quarantined` |
| `quarantined` | `completeMediaUpload` khớp | `processing` (+ `media.uploaded`) |
| `quarantined` | `completeMediaUpload` không khớp | `rejected` |
| `processing` | worker sạch + biến thể xong | `ready` (+ `media.processed`) |
| `processing` | virus / sai magic bytes / lỗi chuyển đổi | `rejected` (+ `media.rejected`) |

## 3. Quy tắc kiểm tra dữ liệu

- Loại/đầu ra/giới hạn: [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md); bucket và signed URL: ADR-0044 mục 2.
- Công thức: [CONTENT_MODEL §9.1](../../docs/02-learning/CONTENT_MODEL.md), ADR-0055 mục 4, 7.
- Asset được version tham chiếu không soft-delete được (ADR-0044 mục 4).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `media.uploaded` | `completeMediaUpload` khớp | Cần thêm `specs/events/payloads/learning/media.uploaded.v1.schema.json` |
| `media.processed` | worker `ready` | Cần thêm `media.processed.v1.schema.json` |
| `media.rejected` | `rejected` | Cần thêm `media.rejected.v1.schema.json` |

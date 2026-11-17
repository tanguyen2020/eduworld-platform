# Data Model: F020 — Nền module learning: projection, mô hình nội dung, gán platform→tenant, thùng rác tenant

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

### Mixin `ContentItemMixin` (ent, dùng cho `content.questions`, `content.question_sets`, `curriculum.curricula`…) — quy ước loại H (Có sẵn, DATABASE_DESIGN §2 "Quy ước cột chung")

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `<item>_id` | UUIDv7 | PK | |
| `tenant_id` | uuid NULL | FK `ref.tenants`; RLS loại H | NULL = platform |
| `owner_scope` | `varchar(64)` (`MaxLen(64)`) | CHECK IN (`platform`,`tenant`); CHECK `(owner_scope='platform') = (tenant_id IS NULL)` | FR-CNT-005 |
| `current_version_id` | uuid NULL | trigger: chỉ trỏ version `published` của chính item | ADR-0043 |
| `latest_version_no` | int | NOT NULL DEFAULT 0 | |
| `cloned_from_id`, `cloned_from_tenant_id`, `cloned_by` | uuid NULL | | ADR-0029 |
| `cloned_at` | timestamptz NULL | | |
| `created_at`, `updated_at`, `created_by`, `updated_by`, `version` | | `version` optimistic concurrency | `src/backend/pkg/entx.HybridContent` (tên cũ `PlatformBase`, ADR-0089) |
| `deleted_at`, `deleted_by`, `delete_reason` | timestamptz / uuid / `varchar(64)` | partial unique `WHERE deleted_at IS NULL` | R21 |

### Mixin `ContentVersionMixin` (Có sẵn — cột theo DATABASE_DESIGN §3.3, §3.13)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `version` | int | UNIQUE `(item_id, version)` | |
| `status` | `varchar(64)` | CHECK IN (`draft`,`in_review`,`approved`,`published`,`archived`,`rejected`) | |
| `tenant_id`, `owner_scope` | như item | CHECK như item | sao từ item |
| `author_id` | uuid | NOT NULL | |
| `reviewer_id`, `published_at` | uuid / timestamptz NULL | bắt buộc theo status (domain) | FR-CNT-002 |
| `content_language` | `varchar(64)` | CHECK IN (`vi`,`en`), DEFAULT `vi` | ADR-0056 · Thêm cột (chưa có ở DATABASE_DESIGN §3.3) |
| `curriculum_version_id` | uuid | NOT NULL (trừ media) | Thêm cột |
| `change_note` | `varchar(500)` | | §3.13 |
| `publish_at` | timestamptz NULL | | §3.13 |
| `snapshot_key` | `varchar(512)` NULL | | §3.13 |
| `snapshot_sha256` | `varchar(64)` NULL | | §3.13 |
| `based_on_version_id` | uuid NULL | | §3.13 |
| `deleted_at`, `deleted_by`, `delete_reason` | | chỉ cho `draft`/`rejected` (domain) | ADR-0028 |
| `cloned_from_id`, `cloned_from_tenant_id`, `cloned_at`, `cloned_by` | | | ADR-0029 |

### ContentRelease — `content.content_releases` (Có sẵn, DDL DATABASE_DESIGN §3.13)

Cột: `release_id`, `tenant_id` NULL, `item_type varchar(64)`, `item_id`, `action varchar(16)` (`publish`/`rollback`/`unpublish`), `from_version_id`, `to_version_id`, `reason varchar(500)`, `scheduled_for`, `released_by`, `released_at`, `idempotency_key varchar(64)`. Append-only, loại TA\*, mixin `TenantOptionalAppendOnly` (ADR-0089 §2); role app chỉ INSERT/SELECT; dòng `tenant_id` NULL chỉ ghi bằng `learning_platform` dưới `PlatformScope`. F020 tạo bảng; F021, F026 ghi.

### ContentTransition — `content.content_transitions` (Có sẵn, DATABASE_DESIGN §2.5 — DDL chi tiết Mới)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `transition_id` | UUIDv7 | PK | |
| `tenant_id` | uuid NULL | RLS loại H | |
| `content_type` | `varchar(64)` | `LearningContentType` hoặc `curriculum` | |
| `content_version_id` | uuid | idx | |
| `from_status`, `to_status` | `varchar(64)` | | |
| `actor_id` | uuid | NOT NULL | |
| `at` | timestamptz | DEFAULT now() | |
| `note` | `varchar(2000)` | | khớp `maxLength` của transition request |

Append-only, loại TA\*, mixin `TenantOptionalAppendOnly`; dòng `tenant_id` NULL chỉ ghi bằng `learning_platform` dưới `PlatformScope` (ADR-0089 §2).

### TenantResourceAssignment — `infra.tenant_resource_assignments` (Có sẵn, DDL DATABASE_DESIGN §3.0)

`assignment_id` PK, `resource_type varchar(64)`, `resource_id`, `tenant_id` FK `ref.tenants`, `assigned_by`, `assigned_at`, `deleted_at`, `deleted_by`. Index: `tra_active_uq (resource_type, resource_id, tenant_id) WHERE deleted_at IS NULL`, `tra_tenant_idx (tenant_id, resource_type, resource_id) WHERE deleted_at IS NULL`. Thêm cột `delete_reason varchar(64)` theo quy ước loại T (Thêm cột).

### Projection `ref.*` (Có sẵn, DATABASE_DESIGN §2.17)

| Bảng | Cột |
|---|---|
| `ref.tenants` | `tenant_id` PK, `code varchar(64)`, `tenant_type varchar(64)`, `status varchar(64)`, `settings_subset jsonb` (CHECK ≤ 16384 byte), `source_version bigint` |
| `ref.schools` | `school_id` PK, `tenant_id`, `name varchar(255)`, `name_i18n jsonb`, `province_code varchar(64)`, `province_name_i18n jsonb`, `school_level varchar(64)`, `is_virtual bool`, `status varchar(64)`, `source_version` |
| `ref.classes` | `class_id` PK, `tenant_id`, `school_id`, `grade_level smallint`, `name varchar(128)`, `academic_year_id`, `is_virtual`, `source_version` |
| `ref.provinces` | `code varchar(64)` PK, `country_code varchar(2)`, `name_i18n jsonb`, `status varchar(64)`, `source_version` |
| `ref.students` | `student_id` PK, `tenant_id`, `school_id`, `grade_level`, `status varchar(64)`, `deleted_at`, `source_version` |
| `ref.enrollments` | PK `(student_id, class_id, start_date)`, `tenant_id`, `status varchar(64)`, `end_date`, `source_version` |

**Quan hệ**: mọi bảng T/H của `eduworld_learning` FK `tenant_id` → `ref.tenants`. Không FK chéo database.
**Index**: `ref.students (tenant_id, school_id)`, `ref.enrollments (tenant_id, class_id) WHERE status='active'`.

## 2. Vòng đời / trạng thái

State machine version (domain `contentmodel.Lifecycle`, nguồn [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md#6-workflow-trạng-thái)):

| Từ | Action | Đến | Điều kiện ở F020 |
|---|---|---|---|
| — | create / new_version | `draft` | |
| `draft` | `submit_review` | `in_review` | ContentMeta hợp lệ |
| `in_review` | `approve` / `reject` | `approved` / `rejected` | reviewer ≠ author; reject cần note |
| `rejected`, `approved` | `revise` | `draft` | |
| `approved` | `publish` | `published` | (điều kiện đầy đủ ở F025/F026) |
| `published` | `archive` | `archived` | không phải con trỏ hiện hành |

Assignment: `active` (`deleted_at IS NULL`) → `revoked` (soft-delete).

## 3. Quy tắc kiểm tra dữ liệu

- `(owner_scope = 'platform') = (tenant_id IS NULL)` ở mọi bảng H.
- Version `published`/`archived` không UPDATE cột nội dung: hàm mẫu `content.forbid_published_update()` tạo ở F020, gắn trigger cho từng bảng version ở F021/F022/F024 ([DATABASE_DESIGN §3.3](../../docs/07-backend/DATABASE_DESIGN.md#33-question-version-bất-biến-sau-publish)).
- Soft-delete version chỉ cho `draft`/`rejected`; item chỉ soft-delete khi `current_version_id IS NULL`.
- Assignment chỉ cho `owner_scope = platform`, tenant `active` trong `ref.tenants`.
- RLS: T/TA/TA\*/H theo [MULTITENANCY §5.3](../../docs/06-architecture/MULTITENANCY.md#53-policy-rls): T/TA/TA\* chỉ điều kiện tenant (dòng tenant NULL chỉ ghi bằng `learning_platform`), H tenant + gán, không policy nào có `deleted_at` (ADR-0089 §1); view `<table>_live` cho `learning_readonly`; `REVOKE DELETE, TRUNCATE` cho `learning_app`, `learning_platform`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `<resource_type>.deleted` | Soft-delete item | `specs/events/payloads/learning/resource.deleted.v1.schema.json` (Cần thêm) |
| `<resource_type>.restored` | Khôi phục | `specs/events/payloads/learning/resource.restored.v1.schema.json` (Cần thêm) |
| `audit.recorded` | Gán/gỡ gán/clone/xóa/khôi phục/bypass | envelope `specs/events/envelope.schema.json` |

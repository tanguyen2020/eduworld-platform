# Data Model: F045 — Platform Administration backend: saga gán/clone, thùng rác mọi tenant, đọc liên tenant

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Database `eduworld_identity`, schema `platform` (bảng G, ghi bằng role `identity_platform`, không có RLS theo tenant vì là dữ liệu điều phối platform; mọi phiên ghi audit — DATABASE_DESIGN §1.1). Bảng `infra.tenant_resource_assignments` nằm ở database của module sở hữu tài nguyên và do feature của service đó tạo (F020 với learning).

## 1. Entity

### TenantCloneJob — `platform.tenant_clone_jobs` (Có sẵn, DATABASE_DESIGN §2.0)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `job_id` | UUIDv7 | PK | |
| `source_tenant_id`, `target_tenant_id` | uuid | NOT NULL, CHECK `source_tenant_id <> target_tenant_id` | |
| `resource_type` | `varchar(64)` (`MaxLen(64)`) | NOT NULL | Trong allowlist clone |
| `resource_ids` | `uuid[]` | CHECK `cardinality(resource_ids) BETWEEN 1 AND 500` | |
| `options` | `jsonb` | CHECK `octet_length ≤ 16384` | |
| `status` | `varchar(64)` | CHECK `IN ('queued','running','succeeded','partially_failed','failed')` | |
| `id_map` | `jsonb` | CHECK `octet_length ≤ 1048576` | nguồn → bản sao, gồm phụ thuộc |
| `stats` | `jsonb` | CHECK `octet_length ≤ 16384` | số bản ghi theo loại |
| `saga_id` | uuid | NOT NULL | **Thêm cột** — liên kết `saga_runs` |
| `requested_by` | uuid | NOT NULL | |
| `idempotency_key` | `varchar(64)` | UNIQUE `(requested_by, idempotency_key)` | |
| `created_at`, `finished_at` | `timestamptz` | | |

### SagaRun — `platform.saga_runs` (Có sẵn)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `saga_id` | UUIDv7 | PK | |
| `saga_type` | `varchar(64)` | CHECK gồm `tenant_clone`, `trash_restore`, **`tenant_assign`, `tenant_unassign`** (Thêm giá trị) | |
| `subject` | `jsonb` | CHECK `octet_length ≤ 16384` | resource, tenant liên quan |
| `status` | `varchar(64)` | CHECK `IN ('running','compensating','succeeded','failed')` | |
| `requested_by` | uuid | NOT NULL | |
| `idempotency_key` | `varchar(64)` | UNIQUE `(saga_type, idempotency_key)` | |
| `started_at`, `finished_at` | `timestamptz` | | |
| `next_run_at` | `timestamptz` | NULL | **Thêm cột** — lịch retry, index `(status, next_run_at)` |

### SagaStep — `platform.saga_steps` (Có sẵn)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `saga_id`, `step_no` | uuid, int | PK `(saga_id, step_no)` | |
| `service` | `varchar(64)` | NOT NULL | `learning`/`game`/`competition`/`chat`/`identity` |
| `action` | `varchar(64)` | NOT NULL | `clone`, `assign`, `unassign`, `restore` |
| `status` | `varchar(64)` | CHECK `IN ('pending','running','succeeded','failed','skipped')` | |
| `attempts` | int | NOT NULL DEFAULT 0 | |
| `request`, `response` | `jsonb` | CHECK `octet_length ≤ 1048576` | `response` chứa `id_map` của bước |
| `error` | `varchar(1000)` | NULL | không chứa PII |
| `updated_at` | `timestamptz` | | |

### TenantDefaultAssignment — `platform.tenant_default_assignments` (Có sẵn)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `default_assignment_id` | UUIDv7 | PK | **Thêm cột** (DATABASE_DESIGN chưa ghi PK) |
| `resource_type` | `varchar(64)` | NOT NULL | |
| `resource_id` | uuid | NOT NULL | |
| `created_by` | uuid | NOT NULL | |
| `created_at` | `timestamptz` | | |
| `deleted_at`, `deleted_by` | | partial unique `(resource_type, resource_id) WHERE deleted_at IS NULL` | Gỡ = soft-delete |

### TenantResourceAssignment — `infra.tenant_resource_assignments` (Có sẵn, ở DB module sở hữu)

Không tạo trong identity. F045 chỉ ghi qua internal API của module sở hữu; cấu trúc ở DATABASE_DESIGN §3.0.

### UserMerge — `identity.user_merges` (Có sẵn, G, DATABASE_DESIGN §2.16; thêm cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `merge_id` | UUIDv7 | PK | dùng làm `Idempotency-Key` khi gọi `mergeUserData` |
| `from_user_id` | uuid | NOT NULL, UNIQUE | một user chỉ bị gộp một lần |
| `into_user_id` | uuid | NOT NULL, CHECK `<> from_user_id` | |
| `school_id` | uuid | NULL | trường của issue |
| `reason` | `varchar(1000)` | NOT NULL, ≥ 3 ký tự | không chứa PII |
| `merged_by` | uuid | NOT NULL | System Admin duyệt |
| `merged_at` | `timestamptz` | NOT NULL | |
| `saga_id` | uuid | NOT NULL | **Mới** [Confirmed] — liên kết `platform.saga_runs` (`saga_type = user_merge`); task cập nhật DATABASE_DESIGN |
| `reconciliation_issue_id` | uuid | NULL | **Mới** [Confirmed] |

Append-only (không UPDATE/DELETE). User nguồn soft-delete với `delete_reason = merged`.

### Trash provider của identity (không có bảng mới)

Loại do identity sở hữu: `class_template`, `school_settings`, `tenant_settings` (theo `ResourceType`); đọc bản ghi `deleted_at IS NOT NULL` qua `tenancy.Bypass(ctx, "trash")`, khôi phục bằng `SET deleted_at = NULL, deleted_by = NULL` sau khi kiểm tra partial unique.

## 2. Vòng đời / trạng thái

| Thực thể | Chuyển trạng thái |
|---|---|
| TenantCloneJob | `queued` → `running` → `succeeded` · `running` → `partially_failed` (một số bước lỗi hết retry) · `running` → `failed` (bước đầu lỗi) |
| SagaRun | `running` → `succeeded` · `running` → `failed`; clone không bù trừ (bản sao đã tạo là `draft`, giữ để System Admin xử lý) |
| SagaRun `user_merge` | bước identity → `mergeUserData` learning, game, competition, chat (song song) → `succeeded` · bước `blocked`/hết retry → `failed` (không bù trừ; chạy lại từ bước dở, bước `done` không chạy lại) |
| SagaStep | `pending` → `running` → `succeeded`/`failed`; lỗi tạm thời → `pending` với `attempts++` và `next_run_at` theo backoff của engine saga (deadline mỗi bước theo `module_call_deadline`, loại bước saga) |

## 3. Quy tắc kiểm tra dữ liệu

- Một tenant đích cho mỗi job clone; nguồn ≠ đích; tenant phải `active` (`platform.tenants.status`).
- Allowlist clone (code, `clone_plan.go`): `curriculum`, `lesson`, `question`, `question_set`, `assessment_blueprint`, `media`, `learning_path`, `assignment` (bài mẫu, không submission), `challenge_template`, `class_template`, `school_settings`, `tenant_settings`; không có loại dữ liệu học sinh (R22).
- Gán chỉ cho tài nguyên platform (`tenant_id IS NULL`), kiểm tra ở module sở hữu.
- Mỗi bước saga idempotent theo `Idempotency-Key = <saga_id>:<step_no>`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `tenant_data.assigned` | Mỗi tenant được gán thành công | Cần thêm `specs/events/payloads/identity/tenant_data.assigned.v1.schema.json` |
| `tenant_data.unassigned` | Mỗi tenant được gỡ gán | Cần thêm `specs/events/payloads/identity/tenant_data.unassigned.v1.schema.json` |
| `tenant_data.cloned` | Job clone kết thúc | Cần thêm `specs/events/payloads/identity/tenant_data.cloned.v1.schema.json` |
| `audit.recorded` | Mọi thao tác ghi/đọc liên tenant; `user.merged` khi saga gộp xong | Theo [EVENT_DESIGN §2](../../docs/07-backend/EVENT_DESIGN.md#2-envelope-chuẩn) |

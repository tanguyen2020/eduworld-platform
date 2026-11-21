# Data Model: F011 — Đồng bộ SIS bằng CSV và provisioning tài khoản

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§2.1, §2.3, §3.1, §3.14)

Database `eduworld_identity`. Bảng T có cột chung `tenant_id`, `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `delete_reason varchar(64)`; UNIQUE là partial `WHERE deleted_at IS NULL`.

## 1. Entity

### SISConnection — `sis.sis_connections` (Có sẵn, T; sửa cột)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `sis_connection_id` | UUIDv7 | PK | |
| `provider_kind` | `varchar(64)` | `contract_v1`/`csv_import` | |
| `contract_version` | `varchar(64)` | | vd. `1.1.0` |
| `config` | jsonb | CHECK `octet_length <= 65536`; NULL với `csv_import` | `base_url`, `auth_type`, `token_url` — chỉ `contract_v1` (F017 dùng); tạo cột ngay để không phải migrate lại |
| `secret_ref` | `varchar(512)` | NULL với `csv_import` | tham chiếu secret manager — F017 |
| `status` | `varchar(64)` | `active`/`auth_error`/`contract_error`/`paused` | |
| `delta_watermark` | `varchar(512)` | NULL | F017 |
| `schedule` | `varchar(64)` | NULL | cron theo `sis_sync_schedule` — F017 |
| ~~`school_ids uuid[]`~~ | — | **Bỏ** | thay bằng `sis_connection_schools` (ADR-0046) — task cập nhật DATABASE_DESIGN |

### SISConnectionSchool — `sis.sis_connection_schools` (Có sẵn, T)

`sis_connection_id`, `school_id`, `tenant_id`, `active`, `assigned_by`, `assigned_at`; PK `(sis_connection_id, school_id)`; UNIQUE `(school_id) WHERE active AND deleted_at IS NULL` → `SIS_SCHOOL_ALREADY_CONNECTED`.

### SisImport — `sis.sis_imports` (Có sẵn, T)

`import_id`, `school_id`, `requested_by`, `storage_key varchar(512)`, `file_sha256 varchar(128)`, `size_bytes bigint`, `entity_kinds varchar(64)[]` (CHECK `cardinality <= 4`), `status varchar(64)` (`uploaded`/`validating`/`dry_run_ready`/`applying`/`succeeded`/`failed`), `dry_run boolean`, `sync_run_id`, `stats jsonb`, `idempotency_key varchar(64)`.

### SyncRun — `sis.sync_runs` (Có sẵn, G)

`sync_run_id`, `sis_connection_id`, `tenant_id` (Thêm cột, để lọc audit/UI), `mode varchar(64)` (`full`/`delta`/`webhook`/`csv_import`), `status varchar(64)` (`running`/`succeeded`/`partially_failed`/`failed`), `stats jsonb` (fetched/created/updated/unchanged/errors), `started_at`, `finished_at`.

### StagingRecord — `sis.staging_records` (Có sẵn, G, P tháng); SyncRecordError — `sis.sync_record_errors` (Có sẵn, G, P)

- `staging_records`: `sync_run_id`, `entity_kind varchar(64)`, `external_id varchar(64)`, `payload jsonb` (đã chuẩn hóa), `raw_hash varchar(128)`, `action varchar(64)` (`create`/`update`/`noop`/`deactivate`), `status varchar(64)` (`pending`/`applied`/`skipped`/`pending_identifier`); payload ẩn danh hóa sau `sis_staging_retention`.
- `sync_record_errors`: `sync_run_id`, `entity_kind`, `external_id`, `row_no int`, `column_name varchar(64)` (Thêm cột), `error_code varchar(64)` (`SIS_CONTRACT_VIOLATION`…), `message varchar(1000)`, `occurred_at`.

### SISLink — `sis.sis_links`; SyncedStudent — `sis.synced_students`; StudentLoginKey — `sis.student_login_keys` (Có sẵn, T)

- `sis_links`: `entity_type varchar(64)`, `entity_id`, `sis_connection_id`, `external_id varchar(64)`, `school_id`, `last_hash varchar(128)`, `last_synced_at`; UNIQUE `(sis_connection_id, entity_type, external_id)`.
- `synced_students`: PK `(sis_connection_id, student_external_id varchar(64))`, `school_id`, `student_code varchar(32)`, `record_status varchar(32)` (`active`/`inactive`/`transferred_out`/`graduated`/`suspended`), `national_student_id_hash varchar(64)`, `last_hash varchar(64)`, `last_synced_at`.
- `student_login_keys`: `sis_connection_id`, `login_type varchar(16)` (`username`/`idp_subject`), `idp_alias varchar(64)`, `login_key varchar(255)`, `student_external_id varchar(64)`; UNIQUE `(sis_connection_id, login_type, coalesce(idp_alias,''), login_key) WHERE deleted_at IS NULL`.

### ReconciliationIssue — `sis.reconciliation_issues` (Có sẵn, T; thêm cột)

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `issue_id` | UUIDv7 | PK |
| `school_id` | uuid | NULL với `unmapped_school` |
| `sis_connection_id` | uuid | **Thêm cột** |
| `issue_type` | `varchar(64)` | danh sách ở spec F011-AC-14 + `grade_mismatch`, `deactivation_review`, `ambiguous_login`, `school_duplicate` |
| `entity_refs` | jsonb | id nội bộ/external, không PII thô |
| `suggested_action` | `varchar(64)` | |
| `status` | `varchar(64)` | `open`/`resolved`/`ignored` |
| `resolution` | `varchar(64)` | **Thêm cột**: `link`/`same_person`/`ignore`/`fixed_in_source` |
| `resolution_note` | `varchar(1000)` | **Thêm cột** |
| `resolved_by`, `resolved_at` | | |

### ExternalIdentity — `identity.external_identities` (Có sẵn, G); SisIdentityLink — `identity.sis_identity_links` (Có sẵn, T)

- `external_identities`: `issuer varchar(512)`, `subject varchar(255)`, `user_id`, `idp_alias varchar(64)`, `status` (`active`/`superseded`), `last_login_at`; UNIQUE `(issuer, subject) WHERE deleted_at IS NULL`, UNIQUE `(user_id, issuer) WHERE status='active' AND deleted_at IS NULL`; có `deleted_at`/`deleted_by` (G, `GlobalBase`, theo [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)).
- `sis_identity_links`: `link_id`, `user_id`, `school_id`, `sis_connection_id`, `student_external_id varchar(64)`, `status varchar(16)` (`active`/`superseded`), `linked_at`, `superseded_at`, `superseded_by`; UNIQUE một active/user và một active/khóa nguồn.

## 2. Vòng đời / trạng thái

| Entity | Chuyển trạng thái |
|---|---|
| SyncRun | `running` → `succeeded` / `partially_failed` / `failed` |
| SISConnection | `active` → `auth_error` / `contract_error` / `paused` → `active` (System Admin kích hoạt lại) |
| SisImport | `uploaded` → `validating` → `dry_run_ready` → `applying` → `succeeded`/`failed` |
| ReconciliationIssue | `open` → `resolved`/`ignored` |
| User (do apply) | `pending` (tạo) · `active` → `locked` (suspended) / `deactivated` (inactive, graduated) |

## 3. Quy tắc kiểm tra dữ liệu

- Chuẩn hóa định danh: trim, Unicode NFC, chữ thường; unique `lower(username)`, `lower(email)` ([ACCOUNT_PROVISIONING §1](../../docs/09-identity/ACCOUNT_PROVISIONING.md)).
- Một trường chỉ một connection active; khóa nguồn học sinh `(sis_connection_id, student_external_id)`.
- Trường do EduWorld sở hữu (nickname, avatar, settings game) không bao giờ ghi đè từ SIS.
- Apply lô 500, idempotent theo `raw_hash`; lỗi lô rollback lô đó.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `school.synced`, `student.synced` | Apply SIS | `specs/events/payloads/identity/` — Có sẵn (áp 2026-11-17) |
| `student.placement_changed` | Apply học sinh SIS (trường + khối + trạng thái hiệu lực — ADR-0095) | Schema owner F010 — F011 phát cùng schema |
| `student.provisioning_requested` | Học sinh mới | Có sẵn (áp 2026-11-17) |
| `user.provisioned` | Tạo user học sinh từ SIS | Schema owner F010 (T004) — F011 phát cùng schema |
| `student.sis_link_created`, `student.sis_link_superseded` | Tạo/chuyển link | Có sẵn (áp 2026-11-17) |
| `user.status_changed`, `user.access_revoked` | Trạng thái từ SIS | Có sẵn (áp 2026-11-17) |
| `sis.sync_completed`, `sis.reconciliation_issue_raised` | Kết thúc run / tạo issue | Có sẵn (áp 2026-11-17) |

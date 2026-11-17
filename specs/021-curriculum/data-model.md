# Data Model: F021 — Curriculum: cấu trúc, LO, DAG tiên quyết, version, mapping, import/export, seed GDPT

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

Bảng `curriculum.*` liệt kê tên ở DATABASE_DESIGN §2.4, §2.16 nhưng chưa có DDL: cột dưới đây là **Mới** (task T040 cập nhật DATABASE_DESIGN). Bảng loại H mang cột quy ước H (`tenant_id` NULL, `owner_scope`, `deleted_*`, `cloned_*`, `created/updated_*`, `version` concurrency).

## 1. Entity

### Curriculum — `curriculum.curricula` (Có sẵn tên, cột Mới) · loại H · mixin `ContentItemMixin`

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `curriculum_id` | UUIDv7 | PK | |
| `code` | `varchar(64)` | CHECK `^[A-Z][A-Z0-9_]{1,63}$`; UNIQUE `(coalesce(tenant_id,'0…0'), code) WHERE deleted_at IS NULL` | |
| `name` | jsonb (`LearningName`) | CHECK `octet_length ≤ 2048` | |
| `framework` | `varchar(64)` NULL | | vd. `GDPT_2018` |
| `current_version_id`, `latest_version_no` | | từ mixin | con trỏ release |

### CurriculumVersion — `curriculum.curriculum_versions` · loại H

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `curriculum_version_id` | UUIDv7 | PK | |
| `curriculum_id` | uuid | FK | |
| `version` | int | UNIQUE `(curriculum_id, version)` | số thứ tự |
| `version_label` | `varchar(16)` | CHECK `^[0-9]{1,4}\.[0-9]{1,4}$`; UNIQUE `(curriculum_id, version_label) WHERE deleted_at IS NULL` | `MAJOR.MINOR` |
| `status` | `varchar(64)` | CHECK 6 trạng thái | |
| `effective_from`, `effective_to` | date NULL | CHECK `effective_to IS NULL OR effective_to >= effective_from` | |
| `supersedes_version_id`, `based_on_version_id` | uuid NULL | | |
| `author_id`, `reviewer_id`, `published_at` | | | |
| `change_note` | `varchar(1000)` | | biên bản rà soát (CURRICULUM_MODEL §7) |

Trigger `curriculum_versions_immutable` (hàm `content.forbid_published_update` của F020, cột so sánh: `version_label`, `effective_from`, `effective_to`, `supersedes_version_id`). Bảng con dưới đây có trigger `curriculum.forbid_child_change_when_published` kiểm status version cha.

### Subject — `curriculum.subjects` · loại G

`subject_id` PK; `code varchar(64)` UNIQUE `WHERE deleted_at IS NULL` (CHECK `^[a-z][a-z0-9_]{1,63}$`, vd. `math`); `name jsonb` (`LearningName`, ≤ 2048 byte); `grade_levels smallint[]` CHECK `cardinality BETWEEN 1 AND 9` và mỗi phần tử 1–9; `icon_key varchar(64)`; `deleted_*`.

### SubjectGrade — `curriculum.subject_grades` · loại H

`subject_grade_id` PK; `curriculum_version_id`; `subject_id`; `grade_level smallint` CHECK 1–9; `periods_per_year smallint` NULL; UNIQUE `(curriculum_version_id, subject_id, grade_level) WHERE deleted_at IS NULL`.

### Topic — `curriculum.topics` · loại H

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `topic_id` | UUIDv7 | PK |
| `curriculum_version_id`, `subject_id` | uuid | NOT NULL |
| `grade_level` | smallint | CHECK 1–9 |
| `parent_topic_id` | uuid NULL | FK self |
| `code` | `varchar(64)` | UNIQUE `(curriculum_version_id, code) WHERE deleted_at IS NULL` |
| `name` | `varchar(255)` | NOT NULL |
| `sort_order` | int | ≥ 1 |

Index `(curriculum_version_id, subject_id, grade_level, sort_order) WHERE deleted_at IS NULL`.

### LearningObjective — `curriculum.learning_objectives` · loại H

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `objective_id` | UUIDv7 | PK | |
| `curriculum_version_id`, `topic_id` | uuid | NOT NULL | |
| `code` | `varchar(64)` | CHECK regex mã LO; UNIQUE `(curriculum_version_id, code) WHERE deleted_at IS NULL` | DATABASE_DESIGN §2.4 |
| `stable_key` | `varchar(64)` | NOT NULL, mặc định = `code` | |
| `description` | `varchar(1000)` | NOT NULL | |
| `bloom_level` | `varchar(64)` | CHECK IN (`remember`,`understand`,`apply`,`analyze`,`evaluate`,`create`) | |
| `yccd_ref` | `varchar(64)` NULL | | DATABASE_DESIGN §2.4 |
| `competency_ids` | uuid[] | CHECK `cardinality ≤ 20` | DATABASE_DESIGN §2.4 |
| `prerequisite_ids` | uuid[] | CHECK `cardinality ≤ 20` | Thêm cột |
| `sort_order` | int | | |

Index GIN `prerequisite_ids`; index `(curriculum_version_id, topic_id)`.

### Competency — `curriculum.competencies` (Mới) · loại H

`competency_id` PK; `curriculum_version_id`; `code varchar(64)` (`UPPER_SNAKE`) UNIQUE `(curriculum_version_id, code) WHERE deleted_at IS NULL`; `name jsonb` (≤ 2048 byte); `kind varchar(64)` CHECK IN (`general_competency`,`subject_competency`,`quality`); `subject_id` NULL, CHECK `(kind = 'subject_competency') = (subject_id IS NOT NULL)`.

### LoVersionMapping — `curriculum.lo_version_mappings` (Có sẵn — đổi khóa, Thêm cột) · loại H

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `mapping_id` | UUIDv7 | PK | Thêm (DATABASE_DESIGN dùng PK `(from_lo_id, to_lo_id)`, không biểu diễn được `removed`) |
| `curriculum_version_id` | uuid | NOT NULL | version mới |
| `from_lo_id` | uuid | NOT NULL | LO version cũ |
| `to_lo_id` | uuid NULL | CHECK `(relation = 'removed') = (to_lo_id IS NULL)` | |
| `relation` | `varchar(64)` | CHECK IN (`same`,`split`,`merge`,`replaces`,`removed`) | |
| `tenant_id`, `owner_scope`, `deleted_*` | | quy ước H | Thêm cột |

UNIQUE `(curriculum_version_id, from_lo_id, coalesce(to_lo_id,'0…0')) WHERE deleted_at IS NULL`.

### Textbook — `curriculum.textbook_series`, `textbooks`, `textbook_sections`, `textbook_section_objectives` (Có sẵn, DATABASE_DESIGN §2.16) · loại G

- series: `series_id`, `code varchar(64)` UNIQUE, `name varchar(255)`, `publisher varchar(255)`.
- textbook: `textbook_id`, `series_id`, `subject_id`, `grade_level smallint`, `volume smallint`; UNIQUE `(series_id, subject_id, grade_level, volume)`.
- section: `section_id`, `textbook_id`, `chapter varchar(128)`, `lesson_no smallint`, `title varchar(255)`, `page_range varchar(16)` (CHECK `^[0-9]{1,4}(-[0-9]{1,4})?$`).
- section_objectives: PK `(section_id, learning_objective_id)`.

### ImportJob — `content.import_jobs` (Mới) · loại H

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `import_id` | UUIDv7 | PK |
| `tenant_id`, `owner_scope` | | quy ước H |
| `kind` | `varchar(64)` | CHECK IN (`curriculum_yaml`,`curriculum_csv`) — F024 mở rộng |
| `asset_id` | uuid NULL | file qua media (F027); NULL khi chạy từ `cmd/seed` |
| `source_ref` | `varchar(512)` NULL | đường dẫn seed cục bộ (chỉ `cmd/seed`) |
| `dry_run` | bool | |
| `status` | `varchar(64)` | CHECK IN (`queued`,`running`,`succeeded`,`failed`) |
| `created_count`, `rejected_count` | int | ≥ 0 |
| `errors` | jsonb | CHECK `jsonb_array_length ≤ 500` và `octet_length ≤ 262144` |
| `result_refs` | jsonb | ID đã tạo (≤ 65536 byte) |
| `idempotency_key` | `varchar(64)` | UNIQUE `(created_by, idempotency_key)` |
| `created_by`, `created_at`, `finished_at` | | |

### Version mặc định năm học — `lms.school_curriculum_settings` (Có sẵn — owner F040, chỉ đọc)

Cùng database `eduworld_learning`: query `listCurricula` đọc bảng này (read context) để chọn version áp dụng cho trường; chưa có dòng (hoặc F040 chưa triển khai) → version published có khoảng hiệu lực chứa ngày hiện tại (F021-AC-19). Không có projection hay event cài đặt trường từ module identity.

## 2. Vòng đời / trạng thái

CurriculumVersion theo `contentmodel.Lifecycle` (F020): `draft → in_review → approved → published → archived`, `in_review → rejected → draft`. Publish: một version `published` là con trỏ; version published cũ vẫn `published` (tra cứu lịch sử) đến khi archive.

## 3. Quy tắc kiểm tra dữ liệu

- Mã LO: `^[A-Z]{2,8}\.G[1-9]\.[A-Z]{1,6}\.[A-Z][A-Z0-9_]{0,40}(\.[0-9]{1,2})?$` ([CURRICULUM_MODEL §4](../../docs/02-learning/CURRICULUM_MODEL.md#4-mã-learning-objective-chuẩn-proposed)); `G<grade>` phải bằng `grade_level` của topic.
- Tiên quyết: cùng version, không tự tham chiếu, đồ thị không chu trình.
- Publish `GDPT2018`: mọi LO có `yccd_ref`. Publish version có `supersedes_version_id`: mọi LO version cũ có mapping.
- Chỉ một version `draft`/`in_review` mở mỗi curriculum.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `curriculum.version_published` | Publish version | `specs/events/payloads/learning/curriculum.version_published.v1.schema.json` (Cần thêm): `curriculum_id`, `curriculum_version_id`, `version_label`, `supersedes_version_id?`, `owner_scope`, `tenant_id?`, `mappings[] {from_lo_id, to_lo_id?, relation}` (≤ 5000), `published_by` |
| `curriculum.deleted` / `curriculum.restored` | Soft-delete/khôi phục | `resource.deleted.v1` / `resource.restored.v1` (F020) |
| `audit.recorded` | Mọi thao tác ghi | envelope |

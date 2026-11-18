# Data Model: F010 — Tenant, danh mục quốc gia/tỉnh/trường, tổ chức trường–lớp–enrollment

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) (§2.0, §2.1, §2.2, §2.2a, §3.0–§3.2)

Database `eduworld_identity`. Cột chung bảng T: `tenant_id` (FK `platform.tenants`), `created_at`, `updated_at`, `deleted_at`, `deleted_by`, `delete_reason varchar(64)`; mọi UNIQUE là partial `WHERE deleted_at IS NULL`.

## 1. Entity

### Tenant — `platform.tenants` (Có sẵn, G)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `tenant_id` | UUIDv7 | PK | |
| `code` | `varchar(64)` | UNIQUE | |
| `name` | `varchar(255)` | NOT NULL | |
| `tenant_type` | `varchar(64)` | CHECK `school`/`unit`/`common`; UNIQUE `(tenant_type) WHERE tenant_type='common'` | |
| `status` | `varchar(64)` | CHECK `active`/`disabled` | |
| `keycloak_org_id` | uuid | UNIQUE | ghi sau khi worker tạo Organization |
| `keycloak_org_alias` | `varchar(64)` | UNIQUE, CHECK `^[a-z0-9-]{3,63}$` | |
| `login_code` | `varchar(16)` | UNIQUE, CHECK 6 ký tự | dùng khi bật mở rộng sau (ADR-0063) |
| `settings` | jsonb | NOT NULL `{}` | nội dung do F015 |
| `deleted_at`, `deleted_by` | | | R21 |

### Country / Province / ProvinceSuccession — `catalog.countries`, `catalog.provinces`, `catalog.province_successions` (Có sẵn, G)

| Bảng | Cột chính | Ràng buộc |
|---|---|---|
| `countries` | `country_code varchar(2)` PK, `name_i18n jsonb`, `default_locale varchar(16)`, `status varchar(64)` | Release 1.0 chỉ `VN` active |
| `provinces` | `province_id uuid` PK, `country_code`, `code varchar(64)`, `official_code varchar(16)`, `name_i18n`, `short_name_i18n`, `province_type varchar(64)`, `effective_from`, `effective_to`, `status varchar(64)` | UNIQUE `code`; CHECK `status='inactive'` ⇔ `effective_to IS NOT NULL` |
| `province_successions` | `old_province_id` PK, `new_province_id`, `effective_date`, `kind varchar(64)` | FK hai cột → `provinces` |

### School — `org.schools` (Có sẵn, T)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `school_id` | UUIDv7 | PK | |
| `code` | `varchar(64)` | UNIQUE | |
| `name` | `varchar(255)` | | `name_i18n jsonb` `{vi,en}` |
| `country_code` | `varchar(2)` | FK `catalog.countries` | |
| `province_id` | uuid | FK `catalog.provinces`; NULL chỉ với School ảo | chỉ đơn vị `active` khi tạo/sửa |
| `moet_code` | `varchar(32)` | UNIQUE WHERE NOT NULL | |
| `school_level` | `varchar(64)` | `primary`/`lower_secondary`/`multi_level` | |
| `grade_min`, `grade_max` | smallint | CHECK 1–9 | |
| `address` | `varchar(500)` | | |
| `status` | `varchar(64)` | `active`/`pending_review`/`disabled` | theo contract `School.status` (2026-11-19) |
| `is_virtual` | boolean | | School ảo tenant `common` |
| `settings` | jsonb | | nội dung do F015 |

### AcademicYear — `org.academic_years`; Class — `org.classes` (Có sẵn, T)

- `academic_years`: `academic_year_id`, `school_id`, `name varchar(32)`, `start_date`, `end_date`, `is_current`; UNIQUE `(school_id) WHERE is_current`.
- `classes`: `class_id`, `school_id`, `academic_year_id`, `grade_level` (CHECK 1–9), `name varchar(128)`, `is_virtual`, `status varchar(64)`; UNIQUE `(school_id, academic_year_id, name)`, UNIQUE `(class_id, school_id)`; idx `(school_id, grade_level)`.

### User — `identity.users` (Có sẵn, G); Student — `org.students` (Có sẵn, T)

- `users`: cột theo DDL §3.1 (`user_type` CHECK `student`/`staff`, `status`, `username varchar(128)` chuẩn hóa, `email varchar(320)`, `display_name varchar(128)`, `locale varchar(64)`, `primary_tenant_id`, `primary_school_id`, `provisioned_at`, `last_login_at`, `deleted_at`, `delete_reason`); UNIQUE `lower(username)`, UNIQUE `lower(email)` WHERE NOT NULL AND `deleted_at IS NULL`; CHECK học sinh có `primary_tenant_id`.
- `students`: `student_id` (= `user_id`), `school_id`, `grade_level`, `student_code varchar(32)`, `full_name varchar(128)`, `date_of_birth_enc bytea`, `national_id_hash varchar(64)`, `status varchar(64)`; UNIQUE `(school_id, student_code)`.

### ExternalIdentity `platform_common` — `identity.external_identities` (Có sẵn, G; quyết định Technical Architect 2026-11-17)

`external_ref` của học sinh tenant chung (`createCommonTenantStudents`) lưu thành một dòng `issuer = platform_common`, `subject` = `external_ref` (≤ 64), `idp_alias = NULL`, `status = active`; không thêm cột ở `org.students`. Duy nhất theo `(issuer, subject)` trong các bản ghi chưa xóa (ADR-0028, ADR-0089); trùng → dòng `failed` `IDENTITY_CONFLICT`. `platform_common` không thuộc allowlist issuer đăng nhập ([ACCOUNT_PROVISIONING §3.4](../../docs/09-identity/ACCOUNT_PROVISIONING.md#34-tài-khoản-tenant-chung-do-platform-tạo-adr-0063)).

### Enrollment — `org.enrollments` (Có sẵn, T)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `enrollment_id` | UUIDv7 | PK |
| `school_id`, `student_id`, `class_id` | uuid | FK ghép `(class_id, school_id)` → `classes` |
| `status` | `varchar(64)` | `active`/`transferred`/`ended` |
| `start_date`, `end_date` | date | CHECK `end_date >= start_date` |
| `source` | `varchar(64)` | `sis`/`manual`/`platform_common` |

**Index**: `enrollments_one_active_uq (student_id) WHERE status='active' AND deleted_at IS NULL`; `(tenant_id, class_id, status)`.

### TenantMembership — view `identity.tenant_memberships_v` (Có sẵn, view)

Hợp: học sinh (enrollment active → `tenant_id`) ∪ nhân sự (`identity.user_roles` hiệu lực, có từ F012). Đúng một dòng mỗi `(tenant_id, user_id)`.

## 2. Vòng đời / trạng thái

| Entity | Chuyển trạng thái |
|---|---|
| Tenant | `active` ⇄ `disabled` (System Admin); soft-delete chỉ qua Thùng rác (F045) |
| School | `pending_review` → `active` (duyệt) · `active` ⇄ `disabled` |
| Enrollment | `active` → `transferred` (chuyển lớp/trường) · `active` → `ended` (nghỉ học, F011/F016) |

## 3. Quy tắc kiểm tra dữ liệu

- Tenant `school` có đúng một School; tenant `common` chỉ có School `is_virtual` ([DATABASE_DESIGN §2.2](../../docs/07-backend/DATABASE_DESIGN.md)).
- Chuyển lớp/trường: đóng + mở enrollment trong cùng transaction ([IDENTITY_MAPPING §6](../../docs/09-identity/IDENTITY_MAPPING.md)).
- RLS policy loại T theo [DATABASE_DESIGN §4](../../docs/07-backend/DATABASE_DESIGN.md); không policy DELETE.
- Seed migration: tenant `common`, School ảo, lớp ảo `common-g1…g9`, danh mục từ `specs/reference-data`.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `tenant.created` / `tenant.updated` | Tạo/sửa/bật-tắt tenant | `specs/events/payloads/identity/` — Có sẵn (áp 2026-11-17) |
| `province.updated` | Tạo/sửa đơn vị, ghi kế thừa | `specs/events/payloads/identity/province.updated.v1.schema.json` (có sẵn) |
| `school.updated` | Sửa/duyệt trường | Có sẵn (áp 2026-11-17) |
| `class.created` / `class.updated` | Tạo/sửa lớp (thủ công, tenant chung) | Có sẵn (áp 2026-11-17) (lớp từ SIS dùng `class.synced`, F011) |
| `enrollment.changed` | Mở/đóng enrollment | Có sẵn (áp 2026-11-17) |
| `student.transferred` | Chuyển trường | Có sẵn (áp 2026-11-17) |
| `tenant_membership.changed` | Học sinh đổi tenant | Có sẵn (áp 2026-11-17) |
| `user.provisioned` | Tạo học sinh tenant chung | Có sẵn (áp 2026-11-17) |

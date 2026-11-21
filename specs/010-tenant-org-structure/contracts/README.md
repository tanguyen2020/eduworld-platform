# Contracts: F010 — Tenant, danh mục quốc gia/tỉnh/trường, tổ chức trường–lớp–enrollment

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/identity.v1.yaml` (gộp vào `specs/openapi/eduworld-api.v1.yaml`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listTenants` | `GET /api/v1/admin/tenants` | identity.v1.yaml | `platform.tenant.manage` (hoặc `platform.cross_tenant.read`) | Có sẵn |
| `createTenant` | `POST /api/v1/admin/tenants` | identity.v1.yaml | `platform.tenant.manage` | Có sẵn (áp 2026-11-17) |
| `updateTenant` | `PATCH /api/v1/admin/tenants/{tenant_id}` | identity.v1.yaml | `platform.tenant.manage` | Có sẵn (áp 2026-11-17) |
| `listAdminTenantSchools` | `GET /api/v1/admin/tenants/{tenant_id}/schools` | identity.v1.yaml | `platform.tenant.manage` | Có sẵn (áp 2026-11-17) |
| `createAdminTenantSchool` | `POST /api/v1/admin/tenants/{tenant_id}/schools` | identity.v1.yaml | `platform.tenant.manage` | Có sẵn (áp 2026-11-17) |
| `updateAdminTenantSchool` | `PATCH /api/v1/admin/tenants/{tenant_id}/schools/{school_id}` | identity.v1.yaml | `platform.tenant.manage` | Có sẵn (áp 2026-11-17) |
| `listCountries` · `listProvinces` · `listCatalogSchools` | `GET /api/v1/catalog/*` | identity.v1.yaml | đã xác thực (`listCatalogSchools` tenant-scoped) | Có sẵn |
| `createCountry` · `updateCountry` · `createProvince` · `updateProvince` · `createProvinceSuccession` | `/api/v1/admin/catalog/*` | identity.v1.yaml | `platform.catalog.write` | Có sẵn |
| `importSchools` · `getSchoolImport` | `POST /api/v1/admin/schools:import` · `GET /api/v1/admin/school-imports/{import_id}` | identity.v1.yaml | `platform.catalog.write` | Có sẵn |
| `listTenantSchools` · `getTenantSchool` | `GET /api/v1/tenant/schools[/{school_id}]` | identity.v1.yaml | `tenant.school.read` | Có sẵn |
| `updateTenantSchool` · `approveTenantSchool` | `PATCH /api/v1/tenant/schools/{school_id}` · `POST …:approve` | identity.v1.yaml | `tenant.school.write` | Có sẵn |
| `listTenantSchoolClasses` · `listUnassignedStudents` | `GET /api/v1/tenant/schools/{school_id}/classes` · `GET /api/v1/tenant/students/unassigned` | identity.v1.yaml | `tenant.class.read` | Có sẵn |
| `listClasses` | `GET /api/v1/schools/{school_id}/classes` | identity.v1.yaml | `lms.class.read` (scope trường) | Có sẵn |
| `getMyStudentProfile` | `GET /api/v1/students/me` | identity.v1.yaml | học sinh (self, `game_player`) | Có sẵn |
| `getStudentProfile` | `GET /api/v1/students/{student_id}` | identity.v1.yaml | `school.people.read` (scope) + audit | Có sẵn |
| `createCommonTenantStudents` | `POST /api/v1/admin/common-tenant/students` (một hoặc lô ≤ 500, kèm `grade_level`) | identity.v1.yaml | `platform.user.manage` | Có sẵn (áp 2026-11-17, quyết định Technical Architect — ACCOUNT_PROVISIONING §3.4) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `getTenant` | learning/game/competition/chat/realtime → identity | identity.internal.v1.yaml | learning, game, competition, chat, realtime | Có sẵn |
| `checkMemberships` | learning/game/competition/chat → identity | identity.internal.v1.yaml | learning, game, competition, chat | Có sẵn |
| `listProjectionSnapshot` | learning/game/competition/chat (job rebuild projection `ref.*`) → identity | identity.internal.v1.yaml | learning, game, competition, chat | Có sẵn (áp 2026-11-17) — contract; query/facade ở task T043 |

Keycloak Admin REST (ra ngoài, không phải contract EduWorld): organizations, members, users, realm roles, `POST users/{id}/logout`.

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `tenant.created`, `tenant.updated` | identity phát; identity worker, learning/game/competition/chat (`ref.tenants`) tiêu thụ | `specs/events/payloads/identity/tenant.created.v1.schema.json`, `tenant.updated.v1.schema.json` | Có sẵn (áp 2026-11-17) |
| `province.updated` | identity phát; learning, game tiêu thụ | `province.updated.v1.schema.json` | Có sẵn |
| `school.updated` | identity phát; mọi module (`ref.schools`) | `school.updated.v1.schema.json` | Có sẵn (áp 2026-11-17) |
| `class.created`, `class.updated` | identity phát; chat, learning | `class.created.v1.schema.json`, `class.updated.v1.schema.json` | Có sẵn (áp 2026-11-17) — Deprecated (ADR-0095) |
| `enrollment.changed` | identity phát; learning, game, chat | `enrollment.changed.v1.schema.json` | Có sẵn (áp 2026-11-17) — Deprecated (ADR-0095), thay bằng `student.placement_changed` |
| `student.transferred` | identity phát; chat, competition, learning | `student.transferred.v1.schema.json` | Có sẵn (áp 2026-11-17) |
| `student.placement_changed` | identity phát (mọi nguồn đổi trường/khối/hiệu lực — ADR-0095); learning, game, competition, chat, cache Principal tiêu thụ | `student.placement_changed.v1.schema.json` | Có sẵn (áp 2026-11-21, WP-S); producer ở WP-I1 |
| `tenant_membership.changed` | identity phát và tiêu thụ (worker); mọi module invalidate Principal | `tenant_membership.changed.v1.schema.json` | Có sẵn (áp 2026-11-17) |
| `user.provisioned` | identity phát; game (GameProfile, F070) | `user.provisioned.v1.schema.json` | Có sẵn (áp 2026-11-17) |
| `user_role.changed` | F012 phát; worker F010 tiêu thụ | `user_role.changed.v1.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/reference-data/*` | Seed quốc gia/tỉnh/kế thừa | Có sẵn |

## 6. Delta cần thực hiện

- **Đã áp 2026-11-17** ở spec gộp rồi `split-public.mjs`: `createTenant` (`TenantCreate` → `TenantDetail`), `updateTenant` (`TenantUpdate`, merge-patch), `listAdminTenantSchools` (`SchoolPage`), `createAdminTenantSchool` (`AdminSchoolCreate`), `updateAdminTenantSchool` (`AdminSchoolUpdate`), tham số `TenantIdPath`; `x-portal-action` `action.platform_admin.tenants.view`/`.edit` (menu catalog cập nhật `operation_ids`). **`createCommonTenantStudents` đã áp 2026-11-17** theo quyết định Technical Architect (ACCOUNT_PROVISIONING §3.4): `POST /admin/common-tenant/students`, `X-Tenant-Id` = tenant `common`, `Idempotency-Key`; body `CommonTenantStudentsCreate` (≤ `common_student_batch_max` dòng `CommonTenantStudentInput`: `username`, `display_name`, `grade_level`, `external_ref`); `200 CommonTenantStudentsResult` (`results[]` theo dòng, `initial_password` một lần, `password_redacted` khi replay, `Cache-Control: no-store`); `x-portal-action: action.platform_admin.access_control.edit_users`. Nguyên văn delta: OpenAPI `identity.v1.yaml`: thêm `createTenant`, `updateTenant` (`If-Match`, `Idempotency-Key`), `listAdminTenantSchools`, `createAdminTenantSchool`, `updateAdminTenantSchool`, `createCommonTenantStudents` theo [API_DESIGN §9.16](../../../docs/07-backend/API_DESIGN.md); mọi chuỗi có `maxLength`; tag `x-internal: true` (gateway nội bộ).
- **Đã áp 2026-11-17 (T004).** Event payload JSON Schema: `tenant.created`, `tenant.updated`, `school.updated`, `class.created`, `class.updated`, `enrollment.changed`, `student.transferred`, `tenant_membership.changed`, `user.provisioned` trong `specs/events/payloads/identity/` (mỗi file có `examples`). `user.provisioned.source` gồm `platform_common` (F010-AC-24; EVENT_DESIGN §4.1 đã đồng bộ).
- **Đã áp 2026-11-21 (ADR-0095, WP-S — chỉ contract).** Event payload `student.placement_changed.v1` (thay `enrollment.changed`; owner F010) + dòng EVENT_DESIGN §4.1/§3.2 + `specs/events/README.md`; `enrollment.changed`, `class.created`, `class.updated` đánh dấu Deprecated (schema còn tới khi WP-I1 gỡ producer). Producer, xóa schema cũ và viết lại `listProjectionSnapshot.entity_type` (bỏ class/enrollment) ở WP-I1.
- Kiểm tra: `make specs-lint`, `make gen MOD=identity`.
- **Đã áp 2026-11-17 (phần contract của T043).** Thêm operation facade `listProjectionSnapshot` `GET /internal/v1/projection-snapshots/{entity_type}` (`ProjectionSnapshotPage`; F010 là owner duy nhất; F020 là bên gọi đầu tiên) — query và facade còn ở task T043.

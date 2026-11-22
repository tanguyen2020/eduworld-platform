# Contracts: F016 — Vòng đời tài khoản: khóa, thu hồi truy cập, dọn học sinh không đăng nhập, ẩn danh hóa

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/identity.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `lockUser` | `POST /api/v1/admin/users/{user_id}:lock` | identity.v1.yaml | `platform.user.manage` (role platform, gateway nội bộ) | Có sẵn |
| `unlockUser` · `deactivateUser` | `POST /api/v1/admin/users/{user_id}:unlock` · `:deactivate` | identity.v1.yaml | `platform.user.manage` | Cần thêm |
| `lockTenantUser` · `unlockTenantUser` | `POST /api/v1/tenant/users/{user_id}:lock` · `:unlock` (`reason`, `Idempotency-Key`) | identity.v1.yaml | `tenant.user.lock` [Confirmed] (School Admin: học sinh trường mình) | Cần thêm |
| `resetStudentPassword` | `POST /api/v1/tenant/students/{student_id}/password:reset` (`reason`, `Idempotency-Key`; System Admin: `X-Tenant-Id` = tenant đích, gồm `common`) | identity.v1.yaml | `lms.student.password_reset` (School Admin trường mình, Tenant Admin, System Admin) | Có sẵn (áp 2026-11-17, quyết định Technical Architect) — `200 StudentPasswordResetResult` (`initial_password` một lần, `no-store`, replay `password_redacted`) |
| `previewTenantStudentCleanup` | `GET /api/v1/tenant/student-cleanup/preview` | identity.v1.yaml | `tenant.student_cleanup.read` | Có sẵn |
| `createStudentCleanupExclusions` · `deleteStudentCleanupExclusions` | `POST/DELETE /api/v1/tenant/student-cleanup/exclusions` | identity.v1.yaml | `tenant.student_cleanup.exclude` | Có sẵn |
| `listStudentCleanupRuns` | `GET /api/v1/admin/student-cleanup/runs?tenant_ids=` | identity.v1.yaml | `platform.student_cleanup.approve` (đọc) | Có sẵn |
| `approveStudentCleanupRun` | `POST /api/v1/admin/student-cleanup/runs/{run_id}:approve` | identity.v1.yaml | `platform.student_cleanup.approve` | Có sẵn |
| `restoreTenantTrashItem` | `POST /api/v1/tenant/trash/{resource_type}/{resource_id}:restore` (`resource_type = student`) | identity.v1.yaml | `tenant.trash.restore` | Có sẵn |
| `listDataRequests` · `createDataRequest` · `getDataRequest` | `/api/v1/admin/data-requests*` | identity.v1.yaml | `platform.data_request.write` | **Đã gỡ** (owner 2026-11-22, ADR-0097) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `cleanupStudentData` | identity → game, learning (competition, chat khi có) | `game.internal.v1.yaml`, `learning.internal.v1.yaml` | identity | Có sẵn |
| `restoreStudentData` | identity → game, learning | như trên | identity | Có sẵn |
| `anonymizeUserData` | identity → game, learning | như trên | identity | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| Close code 4403 access revoked | realtime → client (F090, theo `user.access_revoked`) | Có sẵn (`specs/asyncapi/eduworld-realtime.v1.yaml`) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `user.status_changed`, `user.access_revoked` | identity → mọi module, realtime | xem F011 (owner F011) | Có sẵn (áp 2026-11-17), owner F011 |
| `user.access_restored` | identity → AUD, Keycloak worker | `specs/events/payloads/identity/user.access_restored.v1.schema.json` | Cần thêm |
| `student.cleanup_scheduled` | identity → NTF, AUD | `student.cleanup_scheduled.v1.schema.json` | Cần thêm |
| `student.cleaned_up`, `student.restored` | identity → game, learning, competition, chat, realtime, Keycloak worker (gỡ/thêm membership Organization, không disable user) | `student.cleaned_up.v1.schema.json` (F016 tạo ở T004; F013 chỉ dùng fixture), `student.restored.v1.schema.json` | Có sẵn (áp 2026-11-17) (cần cho fixture F013/F014) |
| `user.anonymized` | identity → mọi module | `user.anonymized.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- `identity.v1.yaml`: thêm `unlockUser`, `deactivateUser`, `lockTenantUser`, `unlockTenantUser` (body `{reason}` maxLength 1000, `Idempotency-Key`); `DataRequestCreate.kind` chỉ `export`/`anonymize` (không `physical_erase`, ADR-0070) nếu chưa đúng.
- `cleanupStudentData` đã trả `StudentLifecycleResult` (`done`/`blocked`/`nothing_to_do`, `blocked_reason`) — không cần sửa; ghi chú: ACCOUNT_PROVISIONING §7b ghi `done | skipped | failed`, EVENT_DESIGN/ACCOUNT_PROVISIONING ghi close code 4003 trong khi AsyncAPI dùng 4403 — sửa tài liệu theo spec.
- **Áp một phần 2026-11-17:** `student.cleaned_up`, `student.restored` đã có (M-C1 F013/F014 tiêu thụ qua fixture); `user.access_restored`, `student.cleanup_scheduled`, `user.anonymized` còn Cần thêm (M-C3). Event payload: `user.access_restored`, `student.cleanup_scheduled`, `student.restored`, `user.anonymized` và `student.cleaned_up` (F016 sở hữu; F013 T045 chỉ dùng fixture).
- **Đã áp 2026-11-17:** `resetStudentPassword` + schema `StudentPasswordResetRequest`/`StudentPasswordResetResult`, permission `lms.student.password_reset` (ADMIN_PORTAL §2.2, §2.3), action `action.school_lms.classes.reset_student_password`, `user.access_revoked.reason` thêm `password_changed` (schema owner F011), quy tắc ACCOUNT_PROVISIONING §6d (task T048).
- Catalog permission: thêm `tenant.user.lock` (ADMIN_PORTAL §2.2, `src/backend/pkg/authz/catalog.go`).
- Kiểm: `make specs-lint`, `make gen MOD=identity`, `make gen MOD=game`, `make gen MOD=learning`, `make test-contract` (facade test), `make arch-check`.

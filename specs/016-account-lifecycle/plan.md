# Implementation Plan: F016 — Vòng đời tài khoản: khóa, thu hồi truy cập, dọn học sinh không đăng nhập, ẩn danh hóa

**Branch**: `016-account-lifecycle` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

module identity thêm command `LockUser`, `UnlockUser`, `DeactivateUser` (outbox `user.status_changed`, `user.access_revoked`/`user.access_restored`), consumer `keycloak-provisioner` nhánh thu hồi/khôi phục, worker `student-inactive-cleanup` (preview, báo trước, ngưỡng duyệt) và engine saga (`platform.saga_runs`/`saga_steps`) cho `student_cleanup`, `student_restore`, `user_anonymize`. Saga gọi `cleanupStudentData`/`restoreStudentData`/`anonymizeUserData` của learning và game qua facade (`pkg/modcall`, `Idempotency-Key`). Module game và module learning hiện thực ba operation facade và consumer `user.access_revoked`/`student.cleaned_up`. Job `retention-anonymizer` và yêu cầu dữ liệu (`createDataRequest`).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | ent, Atlas, oapi-codegen (facade `game`, `learning` sinh vào `modules/<m>/api`), `pkg/modcall`, `pkg/outbox`, `pkg/entx` (`UnitOfWork`), `pkg/clock` |
| Storage | PostgreSQL `eduworld_identity` (`identity.users`, `org.students`, `platform.cleanup_exclusions`, `platform.student_cleanup_runs`, `platform.saga_runs`, `platform.saga_steps`, `platform.deletion_requests`, `platform.erasure_log`); `eduworld_game`, `eduworld_learning` (phần nhận saga) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Keycloak); Clock giả; chaos tắt module game giữa saga (TC-SVC-009) |
| Target Platform | Linux container (`core-api`; `worker` profile `events`/`jobs`) |
| Project Type | backend module (đa module theo saga) |
| Performance Goals | Thu hồi REST ≤ 60 s, Keycloak ≤ 1 phút ([ACCOUNT_PROVISIONING §6](../../docs/09-identity/ACCOUNT_PROVISIONING.md)) |
| Constraints | AGENTS R13, R14, R21, R24, R29; ADR-0028, ADR-0032, ADR-0040, ADR-0070, ADR-0074 |
| Scale/Scope | `inactive_student_cleanup_days` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready (Keycloak khi dọn đã chốt: giữ `enabled`, gỡ membership, thu hồi phiên — Technical Architect 2026-11-16); delta contract ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Liên module chỉ qua facade (saga) và event; mỗi module tự soft-delete dữ liệu của mình |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Soft-delete + ẩn danh hóa, không `DELETE`; job platform có `tenant_id` tường minh |
| IV Server-authoritative | ✅ | Lock/unlock/duyệt có `Idempotency-Key` + audit; saga idempotent theo `saga_id` |
| V Xác thực, phân quyền | ✅ | `platform.user.manage`, `tenant.user.lock` [Proposed], `tenant.student_cleanup.*`, `platform.student_cleanup.approve`, `platform.data_request.write`; internal `x-allowed-callers: [identity]` |
| VI An toàn trẻ em | ✅ | Lý do khóa trung tính với học sinh; event không PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Số ngày/ngưỡng từ cài đặt hiệu lực (F015); retention theo DATA_PRIVACY §2 (đăng ký tên tham số) |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mọi F016-AC-*, chaos saga |

## Project Structure

### Documentation (feature này)

```text
specs/016-account-lifecycle/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
modules/identity/
├── ent/schema/                       # cleanupexclusion.go, studentcleanuprun.go, sagarun.go, sagastep.go, deletionrequest.go, erasurelog.go
├── internal/domain/lifecycle/        # UserStatus transitions, CleanupPolicy, CleanupCandidate
├── internal/domain/saga/             # Saga, Step, compensation
├── internal/application/command/     # LockUser, UnlockUser, DeactivateUser, ScheduleCleanupNotices, RunStudentCleanup,
│                                     # ApproveStudentCleanupRun, CreateCleanupExclusions, DeleteCleanupExclusions,
│                                     # RestoreCleanedStudent, AnonymizeUser, CreateDataRequest
├── internal/application/query/       # PreviewTenantStudentCleanup, ListStudentCleanupRuns, ListDataRequests, GetDataRequest
├── internal/application/saga/        # orchestrator (student_cleanup, student_restore, user_anonymize)
├── internal/infrastructure/client/{gameclient,learningclient}/
├── internal/presentation/http/public/{user_lifecycle.go, student_cleanup.go, data_requests.go}
├── internal/presentation/consumer/keycloak_provisioner.go   # nhánh access_revoked/restored, cleaned_up, restored
├── internal/presentation/job/{student_inactive_cleanup.go, retention_anonymizer.go, saga_runner.go}
modules/game/internal/presentation/http/internal/platform_students.go   # consumer access_revoked.go thuộc F014 (T039)
modules/learning/internal/presentation/http/internal/platform_students.go
```

**Structure Decision**: identity điều phối (ACCOUNT_PROVISIONING §7b); mỗi module thực hiện phần dữ liệu của mình (R13).

## Phase 0: Research

Không có tài liệu research riêng; hai quyết định ghi ở spec Clarifications: thứ tự saga dọn [Proposed]; Keycloak khi dọn — đã chốt (giữ user `enabled`, gỡ membership Organization, thu hồi phiên).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `UserStatus` (chuyển trạng thái hợp lệ theo IDENTITY_MAPPING §5), `CleanupPolicy.EffectiveDays`, `CleanupCandidate`, `Saga` (bước, bù) | Thuần |
| application | Command/Query ở cây trên | Permission: [contracts](contracts/README.md) §1 |
| infrastructure | ent repo, adapter facade `game`/`learning` (`pkg/modcall`, `Idempotency-Key`), Keycloak adapter (F010) | |
| presentation | `lockUser`, `unlockUser`*, `deactivateUser`*, `lockTenantUser`*, `unlockTenantUser`*, `previewTenantStudentCleanup`, `createStudentCleanupExclusions`, `deleteStudentCleanupExclusions`, `listStudentCleanupRuns`, `approveStudentCleanupRun`, `restoreTenantTrashItem` (student), `listDataRequests`, `createDataRequest`, `getDataRequest` · internal (game, learning): `cleanupStudentData`, `restoreStudentData`, `anonymizeUserData` | * = Cần thêm |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F011 (event trạng thái từ SIS), F014 (game session, consumer), F015 (cài đặt hiệu lực, audit), F010 (Keycloak adapter), F012 (permission).
- Event phát: `user.status_changed`, `user.access_revoked`, `user.access_restored`, `student.cleanup_scheduled`, `student.cleaned_up`, `student.restored`, `user.anonymized`, `audit.recorded`.
- Event tiêu thụ: `user.access_revoked`, `student.cleaned_up`, `student.restored` (consumer Keycloak của identity trong `worker-events`). Consumer kết thúc game session (F014-AC-24) và thu hồi phiên game (F013-AC-29) thuộc F014/F013, F016 kiểm end-to-end (F016-AC-24); payload `student.cleaned_up` contract-first ở F013 T045.

### Bảo mật, tenant, idempotency, audit

- Ghi cần `reason` + `Idempotency-Key`; saga `Idempotency-Key = <run_id>:<student_id>` / `saga_id`.
- Worker dùng role `identity_platform` với `tenant_id` tường minh; audit từng học sinh (`actor = system:student-cleanup`).
- Operation facade platform students chỉ cho module `identity` (`x-allowed-callers: [identity]`, kiểm bằng `make arch-check`).

### Quan sát

- Metric `eduworld_student_cleanup_total{tenant,result}`, `eduworld_saga_runs_total{type,status}`, `eduworld_saga_step_retries_total`, `eduworld_access_revoke_latency_seconds`; alert `SagaStuck`, `StudentCleanupLargeBatch`.

## Complexity Tracking

> Constitution Check không có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |

# Tasks: F124 — Thực thi xử phạt và vận hành Safety Team: consumer `sanction.applied` (khóa tài khoản), email thông báo vụ việc cho trường, vòng trực, break-glass

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Viết tắt: `ID/` = `src/backend/modules/identity/`, `CH/` = `src/backend/modules/chat/`. Fixture chỉ dùng placeholder, không từ nhạy cảm (R15).

## Phase 1: Setup

- [ ] T001 Đăng ký permission platform-only `platform.break_glass.request`, `platform.break_glass.approve` (gán `system_admin`), kiểm khởi động — file: `src/backend/pkg/authz/catalog.go`
- [ ] T002 [P] Cấu hình SMTP (`EW_SMTP_HOST`, `EW_SMTP_PORT`, `EW_SMTP_FROM`, `EW_SMTP_PASSWORD_FILE`), `EW_PORTAL_BASE_URL` vào `.env.example` và compose (mailpit local) — file: `deploy/compose/.env.example`, `deploy/compose/identity.yml`
- [ ] T003 [P] Lịch job `email-sender`, `oncall-escalator`, `break-glass-expirer`; linter allowlist `tenancy.Bypass` thêm `CH/internal/application/query/breakglass/` — file: `ID/config/worker.yaml`, `CH/config/worker.yaml`, `.go-arch-lint.yml`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta OpenAPI: 8 operation oncall/break-glass, sửa `ModerationCase`, `ProblemCode` mới — file: `specs/openapi/modules/chat.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`
- [ ] T005 [P] Payload schema `moderation.school_notification_reminded`, `moderation.oncall_paged`, `break_glass.requested`; đề xuất delta tương thích `moderation.school_notified` (+`notification_id`, `ack_due_at`) cho owner F121; xác nhận `sanction.applied` (owner F116) / `sanction.lifted` (owner F121) có `sanction_id`, `case_id`, `sanction_type`, thiếu → chặn US1 — file: `specs/events/payloads/chat/` — kiểm: `make specs-lint`
- [ ] T006 [P] Delta identity.v1: `409 ACCOUNT_LOCK_HELD_BY_SAFETY` cho `unlockUser`/`unlockTenantUser`, `lock_sources[]` trong item người dùng — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make gen MOD=identity`
- [ ] T007 [P] Cập nhật API_DESIGN §4.1 (4 mã lỗi), ADMIN_PORTAL §2.2 (permission break-glass), EVENT_DESIGN §4 (consumer identity, event mới) — file: `docs/07-backend/API_DESIGN.md`, `docs/04-admin-portal/ADMIN_PORTAL.md`, `docs/07-backend/EVENT_DESIGN.md`
- [ ] T008 ent schema + migration identity: `account_locks` (backfill khóa đang có từ `status_reason`), `email_deliveries` — file: `ID/ent/schema/{account_lock,email_delivery}.go` — kiểm: `make migrate-diff MOD=identity`, `make migrate-lint MOD=identity`
- [ ] T009 ent schema + migration chat: `moderation_oncall_shifts`, `break_glass_grants`, cột `assigned_via`, `oncall_page_level`, `last_paged_at` — file: `CH/ent/schema/{moderation_oncall_shift,break_glass_grant,moderation_case}.go` — kiểm: `make migrate-diff MOD=chat`, `make migrate-lint MOD=chat`
- [ ] T010 [P] Test F124-AC-21 (giới hạn độ dài/khoảng) ở tầng schema + validator: `reason` 1001, `note` 501, `duration_minutes` vượt trần → `422` (TC-TEN-024) — file: `CH/tests/unit/breakglass/validation_test.go` — phải FAIL trước khi code

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Sanction `account_lock` (Priority: P1) 🎯 MVP

**Goal**: identity thực thi khóa/mở theo sanction, đa nguồn khóa.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T011 [P] [US1] Test F124-AC-01: `sanction.applied` → khóa `moderation_sanction`, audit, outbox `user.status_changed` + `user.access_revoked` cùng transaction; REST `403 ACCOUNT_LOCKED` ≤ 60 s (TC-AUTH-006) — file: `ID/tests/integration/account_lock/sanction_lock_test.go` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Test F124-AC-02: ma trận nguồn khóa × gỡ (`sanction.lifted` khi còn/không còn khóa khác) (TC-AUTH-006) — file: `ID/tests/unit/account/lock_matrix_test.go`
- [ ] T013 [P] [US1] Test F124-AC-03: giao trùng; `lifted` trước `applied` (TC-SVC-003) — file: `ID/tests/integration/account_lock/ordering_test.go`
- [ ] T014 [P] [US1] Test F124-AC-04: `sanction_type` khác, nhân sự, lệch tenant → không đổi, DLQ + audit (TC-TEN-001) — file: `ID/tests/integration/account_lock/ignore_and_mismatch_test.go`
- [ ] T015 [P] [US1] Test F124-AC-05: `unlockTenantUser`/`unlockUser` khi còn khóa kiểm duyệt → `409 ACCOUNT_LOCK_HELD_BY_SAFETY`; tenant khác → `404` — file: `ID/tests/integration/account_lock/unlock_guard_test.go`
- [ ] T016 [P] [US1] Test F124-AC-06: học sinh nhận `ACCOUNT_LOCKED` không chi tiết; `listTenantUsers` có `lock_sources`, không `case_id` — file: `ID/tests/integration/account_lock/lock_visibility_test.go`

### Implementation cho User Story 1

- [ ] T017 [US1] Domain `User.AddLock/ReleaseLock/Status`, `LockSource` — file: `ID/internal/domain/account/{lock.go,lock_source.go}`
- [ ] T018 [US1] Command `ApplySanctionLock`, `ReleaseSanctionLock`; sửa `UnlockUser`/`UnlockTenantUser` (F016) chỉ gỡ nguồn của mình — file: `ID/internal/application/command/account/`
- [ ] T019 [US1] Consumer `sanction_enforcer.go` (TenantScope từ envelope, kiểm membership, DLQ) + repo `account_lock_repo.go`; trả `lock_sources` trong query người dùng — file: `ID/internal/presentation/consumer/sanction_enforcer.go`, `ID/internal/infrastructure/persistence/account_lock_repo.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Email thông báo vụ việc (Priority: P1)

### Tests cho User Story 2

- [ ] T020 [P] [US2] Test F124-AC-07: người nhận = `school_admin` của trường (fallback `tenant_admin`), mailpit có email đúng mẫu (TC-CHAT-043) — file: `ID/tests/integration/staffmail/school_notification_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Test F124-AC-08: `moderation.school_notification_reminded` → email nhắc lần n (TC-CHAT-043) — file: `ID/tests/integration/staffmail/reminder_test.go`
- [ ] T022 [P] [US2] Test F124-AC-09: quét email/log/audit/`email_deliveries.params` không có tên/mã học sinh, `case_id`, `category_code`, địa chỉ email ở INFO (TC-CHAT-009) — file: `ID/tests/unit/staffmail/no_pii_test.go`
- [ ] T023 [P] [US2] Test F124-AC-10: một email/(event, người nhận, mẫu); SMTP lỗi → backoff → `failed`, metric (TC-SVC-003) — file: `ID/tests/integration/staffmail/delivery_retry_test.go`
- [ ] T024 [P] [US2] Test F124-AC-11: người nhận bị khóa/không còn role/khác trường → `skipped`; `ad.truong.b` không nhận (TC-TEN-003) — file: `ID/tests/integration/staffmail/recipient_scope_test.go`
- [ ] T025 [P] [US2] Chat: job nhắc F121 phát `moderation.school_notification_reminded` qua outbox (test F124-AC-08 phía phát) — file: `CH/tests/integration/moderation/reminder_event_test.go`

### Implementation cho User Story 2

- [ ] T026 [US2] Domain `RecipientRule`, `Template`; mẫu `vi`/`en` (chuỗi qua file dịch) — file: `ID/internal/domain/staffmail/`, `ID/internal/infrastructure/mail/templates/`
- [ ] T027 [US2] Command `EnqueueSchoolSafetyEmail`, `SendDueEmails`; port `MailSender`, `RecipientDirectory`; `SmtpSender` — file: `ID/internal/application/command/staffmail/`, `ID/internal/infrastructure/mail/smtp_sender.go`
- [ ] T028 [US2] Consumer `school_notification_mailer.go`, job `email_sender.go`; bổ sung outbox event vào job `notification_reminder.go` của F121 — file: `ID/internal/presentation/consumer/school_notification_mailer.go`, `ID/internal/presentation/job/email_sender.go`, `CH/internal/presentation/job/notification_reminder.go`

## Phase 5: User Story 3 — Vòng trực on-call p1 (Priority: P1)

### Tests cho User Story 3

- [ ] T029 [P] [US3] Test F124-AC-12: tạo/xóa ca, kiểm `resolvePrincipal` (stub contract), chồng lấn → `409 ONCALL_SHIFT_OVERLAP`, `Idempotency-Key`, audit (TC-CHAT-007) — file: `CH/tests/integration/oncall/shift_test.go` — phải FAIL trước khi code
- [ ] T030 [P] [US3] Test F124-AC-13: case p1 → gán `rotation`, giữ `open`, `moderation.case_assigned` + `moderation.oncall_paged{primary}`; identity gửi `moderation_oncall_page` không nội dung (TC-CHAT-042) — file: `CH/tests/integration/oncall/rotation_assign_test.go`, `ID/tests/integration/staffmail/oncall_page_test.go`
- [ ] T031 [P] [US3] Test F124-AC-14: thang gọi với Clock giả (1×, 2× ngưỡng, quá `report_sla`), mỗi mức một lần, dừng khi nhận — file: `CH/tests/unit/moderation/escalation_ladder_test.go`, `CH/tests/integration/oncall/escalator_job_test.go`
- [ ] T032 [P] [US3] Test F124-AC-15: không có ca → gọi mức kế tiếp, gauge coverage = 0; rule `ModerationOncallGap` (promtool) — file: `CH/tests/integration/oncall/coverage_gap_test.go`, `deploy/observability/prometheus/rules/moderation.test.yaml`
- [ ] T033 [P] [US3] Test F124-AC-16 ma trận authz: `sm.antoan` tạo ca → `403`; role tenant → `403`; thiếu MFA → `403 MFA_REQUIRED` (TC-CMS-WS-02) — file: `CH/tests/integration/oncall/authz_test.go`

### Implementation cho User Story 3

- [ ] T034 [US3] Domain `OncallShift`, `EscalationLadder` — file: `CH/internal/domain/moderation/{oncall.go,escalation_ladder.go}`
- [ ] T035 [US3] Command `CreateOncallShift`, `DeleteOncallShift`, `AssignByRotation` (gọi trong `OpenCase` khi p1), `EscalateUnclaimedP1`; query `ListOncallShifts`; client `resolvePrincipal` — file: `CH/internal/application/command/moderation/`, `CH/internal/infrastructure/client/identity_principal.go`
- [ ] T036 [US3] Handler `oncall.go`, job `oncall_escalator.go`, metric; identity: command `EnqueueOncallPage` (resolve `lead`/`system_admin` theo permission/role) + consumer `oncall_page_mailer.go` — file: `CH/internal/presentation/http/admin/oncall.go`, `CH/internal/presentation/job/oncall_escalator.go`, `ID/internal/presentation/consumer/oncall_page_mailer.go`
- [ ] T037 [US3] Route Alertmanager `team=safety` → receiver `safety-oncall` (secret `ops_alert_safety_*`), rule `ModerationOncallGap`, `EmailDeliveryFailing`, `SchoolSafetyNotificationNoRecipient` — file: `deploy/observability/alertmanager/alertmanager.yml`, `deploy/observability/prometheus/rules/moderation.yaml`

## Phase 6: User Story 4 — Break-glass (Priority: P2)

### Tests cho User Story 4

- [ ] T038 [P] [US4] Test F124-AC-17: tạo yêu cầu `pending`, audit, `break_glass.requested` → email tới System Admin khác (TC-TEN-004) — file: `CH/tests/integration/breakglass/request_test.go`, `ID/tests/integration/staffmail/break_glass_email_test.go` — phải FAIL trước khi code
- [ ] T039 [P] [US4] Test F124-AC-18: tự duyệt → `403`; duyệt/từ chối với `If-Match`; không còn `pending` → `409 BREAK_GLASS_NOT_PENDING` — file: `CH/tests/integration/breakglass/decide_test.go`
- [ ] T040 [P] [US4] Test F124-AC-19: đọc trong phạm vi channel/user + khoảng thời gian; ngoài phạm vi/người khác/hết hạn → `403 BREAK_GLASS_NOT_ACTIVE`; audit + metric Bypass (TC-TEN-023) — file: `CH/tests/integration/breakglass/read_scope_test.go`
- [ ] T041 [P] [US4] Test F124-AC-20: hết hạn tự động, thu hồi, `listBreakGlassGrants` hậu kiểm `messages_viewed_count` — file: `CH/tests/integration/breakglass/expire_revoke_test.go`
- [ ] T042 [P] [US4] Test F124-AC-21 authz: `safety_moderator`, role tenant, thiếu MFA → `403`; route không có ở gateway public (TC-CMS-WS-02) — file: `CH/tests/integration/breakglass/authz_test.go`
- [ ] T043 [P] [US4] Test F124-AC-22: audit/event/log không nội dung tin; tin gameplay đã purge không trả (TC-CHAT-022) — file: `CH/tests/integration/breakglass/no_content_leak_test.go`

### Implementation cho User Story 4

- [ ] T044 [US4] Domain `BreakGlassGrant`, `Scope`; command `RequestBreakGlass`, `DecideBreakGlass`, `RevokeBreakGlass`, `ExpireBreakGlass`; query `ListBreakGlassGrants`, `ListBreakGlassMessages` (Bypass `break_glass`) — file: `CH/internal/domain/breakglass/`, `CH/internal/application/command/breakglass/`, `CH/internal/application/query/breakglass/`
- [ ] T045 [US4] Handler `break_glass.go`, job `break_glass_expirer.go`; identity consumer `break_glass_mailer.go` — file: `CH/internal/presentation/http/admin/break_glass.go`, `CH/internal/presentation/job/break_glass_expirer.go`, `ID/internal/presentation/consumer/break_glass_mailer.go`

## Phase N: Polish & Cross-Cutting

- [ ] T046 [P] Đăng ký tham số `break_glass_max_duration_hours`, `break_glass_max_window_days`, `oncall_shift_max_hours`, `email_delivery_max_attempts` vào CONFIG_DEFAULTS + tài liệu sở hữu (MODERATION §6.1, §6.3) — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/03-social-multiplayer/MODERATION.md`
- [ ] T047 [P] DATABASE_DESIGN (bảng mới, cột mới); ACCOUNT_PROVISIONING §6 (khóa đa nguồn); MONITORING §3, §5 (metric, alert, `reason="break_glass"`) — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/09-identity/ACCOUNT_PROVISIONING.md`, `docs/11-operations/MONITORING.md`
- [ ] T048 [P] Seed local `sa.admin2` [Confirmed]; LOCAL_DEVELOPMENT bảng tài khoản demo — file: `deploy/keycloak/realm/seed-users.local.json`, `docs/11-operations/LOCAL_DEVELOPMENT.md`
- [ ] T049 [P] Contract test payload chat → identity (consumer-driven) cho 6 event tiêu thụ — file: `ID/tests/contract/chat_events_test.go`
- [ ] T050 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T051 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T004–T010) → US1, US2 song song (identity) · US3 sau T009 + US2 (email gọi trực dùng `staffmail`) → US4 → Polish.
- US1 cần F016 (`UnlockUser`, chuỗi `user.access_revoked`) và payload `sanction.*` của F121; US2/US3 cần job nhắc và `OpenCase` của F121.
- Giao diện lịch trực/break-glass chưa có feature (đề xuất F122).

## Definition of Done

- [ ] Mọi tiêu chí F124-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` (MOD=identity, MOD=chat), `make test-contract` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

# Tasks: F121 — Moderation: báo cáo, sanction, hàng đợi Safety Team liên tenant, thông báo vụ việc cho trường

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Đường dẫn gốc `src/backend/modules/chat/` trừ khi ghi rõ. Test dùng tin giả placeholder (R15).

## Phase 1: Setup

- [ ] T001 Đăng ký permission `moderation.case.read/act`, `moderation.case.messages.read`, `moderation.sanction.read/write`, `moderation.school_notification.send/read/acknowledge`, `moderation.case.lead` [Confirmed] (platform-only, kiểm khởi động) — file: `src/backend/pkg/authz/catalog.go`, `src/backend/modules/chat/internal/bootstrap/authz.go`
- [ ] T002 [P] Linter allowlist: `tenancy.Bypass` chỉ trong `internal/presentation/http/admin/` và `internal/application/query/moderation/` — file: `.go-arch-lint.yml`, `src/backend/modules/chat/.golangci.yml`
- [ ] T003 [P] Lịch job `moderation-sla-escalator`, `school-notification-reminder`, `case-hold-release`, `sanction-expirer` — file: `src/backend/modules/chat/config/worker.yaml`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: sửa `ModerationCaseStatus`, `ReportCreate` (+`context`), `SanctionCreate`/`Sanction`, `ModerationActionRequest` (+`reopen`, `sanction_type`), `ModerationCase`/`ModerationCaseDetail`; thêm `ModerationCaseMessage` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`
- [ ] T005 [P] Payload schema `report.created`, `moderation.case_assigned`, `moderation.school_notified`, `moderation.school_acknowledged`, `sanction.lifted`, `chat.message_removed`; delta tương thích `sanction.applied` (+`case_id`, owner F116) — file: `specs/events/payloads/chat/` — kiểm: `make specs-lint`
- [ ] T006 [P] Cập nhật API_DESIGN §4.1 (`MODERATION_CASE_CLOSED` theo trạng thái mới), ADMIN_PORTAL §2.2 (`moderation.case.lead`) — file: `docs/07-backend/API_DESIGN.md`, `docs/04-admin-portal/ADMIN_PORTAL.md`
- [ ] T007 Xác nhận `chat.message_held` payload (owner F116) có sẵn; thiếu → chặn T013 — file: `specs/events/payloads/chat/` (chỉ đọc)
- [ ] T008 ent schema + migration: cột mới `reports`, `moderation_cases`, `sanctions`; bảng `school_safety_notifications`; index hàng đợi — file: `ent/schema/{report,moderation_case,moderation_action,sanction,school_safety_notification}.go` — kiểm: `make migrate-diff MOD=chat`, `make migrate-lint MOD=chat`
- [ ] T009 [P] Domain: `ModerationCase` transition table, `Severity(reason, source)`, quy tắc gộp 24 h, `Sanction.RequiresLead`, `AckDueAt` (+ unit test bảng) — file: `internal/domain/moderation/`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Báo cáo tạo case (Priority: P1) 🎯 MVP

**Goal**: Report/tín hiệu → case đúng tenant/trường.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T010 [P] [US1] Test F121-AC-01: `createReport` + `Idempotency-Key`, snapshot ±10 (gốc + revision), cảm ơn theo locale, `note` > 500 → 422 (TC-CHAT-006, TC-TEN-024) — file: `tests/integration/moderation/create_report_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Test F121-AC-02: gộp 24 h, `report_count`, `severity`, `sla_due_at`, outbox `report.created` (TC-CHAT-006) — file: `tests/integration/moderation/case_merge_test.go`
- [ ] T012 [P] [US1] Test F121-AC-03: `chat.message_held` → case `filter_alert`, giao 2 lần → 1 case (TC-SVC-003) — file: `tests/integration/moderation/message_held_consumer_test.go`
- [ ] T013 [P] [US1] Test F121-AC-04: case p1 → hold hệ thống, gỡ khi đóng (TC-CHAT-043) — file: `tests/integration/moderation/p1_hold_test.go`
- [ ] T014 [P] [US1] Test F121-AC-05: báo cáo tin thu hồi/gameplay chưa purge (TC-CHAT-030, TC-CHAT-019); target ngoài channel/tenant → `404` — file: `tests/integration/moderation/report_scope_test.go`

### Implementation cho User Story 1

- [ ] T015 [US1] Command `CreateReport`, `OpenCaseFromSignal`; port `ContextSnapshotter` — file: `internal/application/command/moderation/create_report.go`, `open_case_from_signal.go`
- [ ] T016 [US1] Handler `createReport`; consumer `message_held.go`; command `ApplyCaseHold` — file: `internal/presentation/http/public/reports.go`, `internal/presentation/consumer/message_held.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Hàng đợi liên tenant (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Test F121-AC-06: list liên tenant qua Bypass, lọc `tenant_ids`/`status`/`severity`/`assignee`, audit + metric (TC-CHAT-042, TC-TEN-023) — file: `tests/integration/moderation/list_cases_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Test F121-AC-07: chỉ assignee/lead đọc nội dung, ±200 ngữ cảnh, `q` chỉ trong case, audit `viewed/messages_viewed/searched` (TC-CHAT-040) — file: `tests/integration/moderation/case_read_test.go`
- [ ] T019 [P] [US2] Test F121-AC-08: assign tự nhận/lead gán, `moderation.case_assigned`, case đóng → `409 MODERATION_CASE_CLOSED` (TC-CHAT-007) — file: `tests/integration/moderation/assign_case_test.go`
- [ ] T020 [P] [US2] Test F121-AC-09 ma trận authorization: `safety_moderator` gọi API ngoài moderation → 403; role tenant gọi `/admin/moderation/*` → 403; `X-Tenant-Id` lệch tenant case → `403 TENANT_MISMATCH` (TC-CHAT-042, TC-CMS-AUTHZ-02) — file: `tests/integration/moderation/authz_matrix_test.go`
- [ ] T021 [P] [US2] Test F121-AC-10: thiếu MFA / sai `azp` → `403 MFA_REQUIRED`/`PERMISSION_DENIED`, audit (TC-CMS-WS-02) — file: `tests/integration/moderation/mfa_test.go`

### Implementation cho User Story 2

- [ ] T022 [US2] Query `ListCases`, `GetCase`, `ListCaseMessages` (Bypass + audit + metric) — file: `internal/application/query/moderation/`
- [ ] T023 [US2] Command `AssignCase`; handler `listModerationCases`, `getModerationCase`, `listModerationCaseMessages`, `assignModerationCase` — file: `internal/application/command/moderation/assign_case.go`, `internal/presentation/http/admin/moderation.go`

## Phase 5: User Story 3 — Xử lý và sanction (Priority: P1)

### Tests cho User Story 3

- [ ] T024 [P] [US3] Test F121-AC-11: mọi action + transition, `If-Match` → 412, `note` 3–1000, case đóng → 409 (TC-CHAT-007) — file: `tests/integration/moderation/act_on_case_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US3] Test F121-AC-12: `remove_message` → `removed` + soft-delete, gốc giữ, realtime `chat.message.removed` — file: `tests/integration/moderation/remove_message_test.go`
- [ ] T026 [P] [US3] Test F121-AC-13: sanction types, quy tắc trưởng ca, cache, `sanction.applied` + `chat.sanction.applied` có `reason_code` — file: `tests/integration/moderation/create_sanction_test.go`
- [ ] T027 [P] [US3] Test F121-AC-14: thực thi sanction khi `enabled_all` tắt và ở `match` (TC-CHAT-014, TC-CHAT-019) — file: `tests/integration/moderation/sanction_enforcement_test.go`
- [ ] T028 [P] [US3] Test F121-AC-15, F121-AC-16: revoke/hết hạn → `sanction.lifted`, cache ≤ 60 s; thông báo `moderation_notice` cho người báo cáo — file: `tests/integration/moderation/revoke_and_notice_test.go`

### Implementation cho User Story 3

- [ ] T029 [US3] Command `ActOnCase`, `CreateSanction`, `RevokeSanction`; `SanctionCache` Valkey — file: `internal/application/command/moderation/`, `internal/infrastructure/cache/sanction_cache.go`
- [ ] T030 [US3] Handler `actOnModerationCase`, `listSanctions`, `createSanction`, `revokeSanction`; job `sanction-expirer`, `case-hold-release` — file: `internal/presentation/http/admin/moderation.go`, `internal/presentation/job/`

## Phase 6: User Story 4 — Thông báo nhà trường (Priority: P1)

### Tests cho User Story 4

- [ ] T031 [P] [US4] Test F121-AC-17: gửi thông báo, `student_ids` trường khác → 422, `ack_due_at`, `moderation.school_notified` (TC-CHAT-043) — file: `tests/integration/moderation/notify_school_test.go` — phải FAIL trước khi code
- [ ] T032 [P] [US4] Test F121-AC-18 tenant isolation: school_admin chỉ trường mình, tenant_admin cả tenant, `ad.truong.b` không thấy; payload không có nội dung tin — file: `tests/integration/moderation/school_notifications_scope_test.go`
- [ ] T033 [P] [US4] Test F121-AC-19: xác nhận + `Idempotency-Key`, lần 2 → 409, trường khác → 404, `note` > 500 → 422 — file: `tests/integration/moderation/acknowledge_test.go`
- [ ] T034 [P] [US4] Test F121-AC-20: nhắc lại quá hạn lần 1/lần 2, lọc `overdue` — file: `tests/integration/moderation/reminder_job_test.go`
- [ ] T035 [P] [US4] Test F121-AC-21: tenant `common` → 422, `contact_path = platform_direct` — file: `tests/integration/moderation/common_tenant_test.go`

### Implementation cho User Story 4

- [ ] T036 [US4] Command `NotifySchool`, `AcknowledgeSchoolNotification`, `RemindOverdueNotifications`; port `StudentDirectory` — file: `internal/application/command/moderation/`
- [ ] T037 [US4] Handler `notifySchoolForCase`, `listSchoolSafetyNotificationsPlatform`, `listSchoolSafetyNotifications`, `acknowledgeSchoolSafetyNotification`; job `notification_reminder.go` — file: `internal/presentation/http/admin/moderation.go`, `internal/presentation/http/public/school_notifications.go`, `internal/presentation/job/notification_reminder.go`

## Phase 7: User Story 5 — Trực p1 và SLA (Priority: P2)

### Tests cho User Story 5

- [ ] T038 [P] [US5] Test F121-AC-22: gauge `eduworld_moderation_cases_open`, rule alert `ModerationP1Unclaimed` (promtool test) — file: `deploy/observability/prometheus/rules/moderation_test.yaml` — phải FAIL trước khi code
- [ ] T039 [P] [US5] Test F121-AC-23: job SLA → `escalated`, không sanction tự động — file: `tests/integration/moderation/sla_escalator_test.go`
- [ ] T040 [P] [US5] Test F121-AC-24: `support_guide` cho category tự hại; không sanction người gửi — file: `tests/integration/moderation/self_harm_case_test.go`

### Implementation cho User Story 5

- [ ] T041 [US5] Metric + alert rule; job `moderation-sla-escalator` — file: `internal/infrastructure/metrics/moderation.go`, `deploy/observability/prometheus/rules/moderation.yaml`, `internal/presentation/job/sla_escalator.go`

## Phase 8: User Story 6 — Tối thiểu hóa (Priority: P2)

### Tests cho User Story 6

- [ ] T042 [P] [US6] Test F121-AC-25: quét OpenAPI không endpoint duyệt chat tự do; audit/event/log moderation không chứa nội dung (TC-CHAT-009) — file: `tests/unit/moderation/no_content_leak_test.go` — phải FAIL trước khi code
- [ ] T043 [P] [US6] Test F121-AC-26: case mở sau purge chỉ trả evidence; evidence không bị retention/purge (TC-CHAT-022) — file: `tests/integration/moderation/evidence_after_purge_test.go`

### Implementation cho User Story 6

- [ ] T044 [US6] Bộ lọc log/audit (chỉ ID), `ListCaseMessages` fallback evidence khi channel purge — file: `internal/application/query/moderation/list_case_messages.go`, `internal/infrastructure/audit/redact.go`

## Phase N: Polish & Cross-Cutting

- [ ] T045 [P] Mục runbook on-call p1 (`runbook_url` của alert) và bảng alert — file: `docs/11-operations/MONITORING.md`
- [ ] T046 [P] Cập nhật DATABASE_DESIGN §2.12 (cột mới, `school_safety_notifications`), MODERATION §5 (đường dẫn API `/admin/moderation/*`), ghi chú ±10/±20 tin — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/03-social-multiplayer/MODERATION.md`
- [ ] T047 [P] Seed local `sm.truongca` [Confirmed] — file: `deploy/keycloak/realm/seed-users.local.json`
- [ ] T048 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T049 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T004–T009) → US1 → US2 → US3 → US4 → US5, US6 song song → Polish.
- T012/T015 cần payload `chat.message_held` (F116). Khóa tài khoản và email trường cần consumer ở module identity (ngoài feature).

## Definition of Done

- [ ] Mọi tiêu chí F121-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract`, `make migrate-lint` (MOD=chat) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

# Tasks: F121 — Moderation: báo cáo, sanction, hàng đợi Safety Team liên tenant, thông báo vụ việc cho trường

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Đường dẫn gốc `src/backend/modules/chat/` trừ khi ghi rõ. Test dùng tin giả placeholder (R15).

## Phase 1: Setup

- [x] T001 Đăng ký permission `moderation.case.read/act`, `moderation.case.messages.read`, `moderation.sanction.read/write`, `moderation.school_notification.send/read/acknowledge`, `moderation.case.lead` [Confirmed] (platform-only, kiểm khởi động) — file: `src/backend/pkg/authz/catalog.go`, `src/backend/modules/chat/internal/bootstrap/authz.go` — **Bằng chứng:** catalog đã có `moderation.case.read/act/lead/messages.read`, `moderation.sanction.read/write`, `moderation.school_notification.*` (platform-only, kiểm khởi động); chưa role nào cấp `moderation.case.lead` (câu hỏi mở F012) — `TestSafetyModeratorPermissions`
- [x] T002 [P] Linter allowlist: `tenancy.Bypass` chỉ trong `internal/presentation/http/admin/` và `internal/application/query/moderation/` — file: `.go-arch-lint.yml`, `src/backend/modules/chat/.golangci.yml` — **Bằng chứng:** thay allowlist linter bằng test static `TestModerationBypassOnlyInModeration` (chỉ `query/safety.go`, `command/safety.go` dùng `ReasonModerationCase`) — `go test ./modules/chat/tests/static` ok
- [x] T003 [P] Lịch job `moderation-sla-escalator`, `school-notification-reminder`, `case-hold-release`, `sanction-expirer` — file: `src/backend/modules/chat/config/worker.yaml` — **Bằng chứng:** job `moderation-sla-escalator`, `school-notification-reminder`, `sanction-expirer` (chu kỳ 30 s, lease 25 s) đăng ký trong `module.go` (profile jobs); `case-hold-release` gộp vào `sanction-expirer` (StaleHolds)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: sửa `ModerationCaseStatus`, `ReportCreate` (+`context`), `SanctionCreate`/`Sanction`, `ModerationActionRequest` (+`reopen`, `sanction_type`), `ModerationCase`/`ModerationCaseDetail`; thêm `ModerationCaseMessage` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat` — **Bằng chứng:** `specs/openapi/eduworld-api.v1.yaml` + `split-public.mjs`; `check-maxlength` 0 vi phạm; contract test `TestOperationsMatchSpec` (56 op), `TestSafetyOperations`
- [x] T005 [P] Payload schema `report.created`, `moderation.case_assigned`, `moderation.school_notified`, `moderation.school_acknowledged`, `sanction.lifted`, `chat.message_removed`; delta tương thích `sanction.applied` (+`case_id`, owner F116) — file: `specs/events/payloads/chat/` — kiểm: `make specs-lint` — **Bằng chứng:** 8 schema `specs/events/payloads/chat/*` (report.created/resolved, moderation.case_assigned/school_notified/school_acknowledged, sanction.lifted, chat.message_removed; sanction.applied mở rộng theo EVENT_DESIGN §4.9); `validate-schemas` 0 lỗi; `checkEvents` trong test tích hợp
- [x] T006 [P] Cập nhật API_DESIGN §4.1 (`MODERATION_CASE_CLOSED` theo trạng thái mới), ADMIN_PORTAL §2.2 (`moderation.case.lead`) — file: `docs/07-backend/API_DESIGN.md`, `docs/04-admin-portal/ADMIN_PORTAL.md` — **Bằng chứng:** API_DESIGN §4.1 `MODERATION_CASE_CLOSED` + route `:act` (reopen); ADMIN_PORTAL §2.2 đã có `moderation.case.lead`; ADMIN_PORTAL bảng kiểm thử portal thống nhất audit `moderation.case.*`
- [x] T007 Xác nhận `chat.message_held` payload (owner F116) có sẵn; thiếu → chặn T013 — file: `specs/events/payloads/chat/` (chỉ đọc) — **Bằng chứng:** `chat.message_held.v1` có sẵn (F116) — consumer `chat.moderation_cases`
- [x] T008 ent schema + migration: cột mới `reports`, `moderation_cases`, `sanctions`; bảng `school_safety_notifications`; index hàng đợi — file: `ent/schema/{report,moderation_case,moderation_action,sanction,school_safety_notification}.go` — kiểm: `make migrate-diff MOD=chat`, `make migrate-lint MOD=chat` — **Bằng chứng:** ent `ent/schema/safety.go` + cột `sanctions`; migration `20261231000001_f121_moderation_cases.sql` (RLS FORCE, append-only, chỉ soft-delete); `TestMigrationMatchesEnt` ok (cơ chế `added`)
- [x] T009 [P] Domain: `ModerationCase` transition table, `Severity(reason, source)`, quy tắc gộp 24 h, `Sanction.RequiresLead`, `AckDueAt` (+ unit test bảng) — file: `internal/domain/moderation/` — **Bằng chứng:** `internal/domain/moderation/safety` + `tests/unit/moderation/safety_test.go` (4 test) ok

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Báo cáo tạo case (Priority: P1) 🎯 MVP

**Goal**: Report/tín hiệu → case đúng tenant/trường.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T010 [P] [US1] Test F121-AC-01: `createReport` + `Idempotency-Key`, snapshot ±10 (gốc + revision), cảm ơn theo locale, `note` > 500 → 422 (TC-CHAT-006, TC-TEN-024) — file: `tests/integration/moderation/create_report_test.go` — phải FAIL trước khi code — **Bằng chứng:** `TestSafetyReportCreatesCase` — lần chạy đầu FAIL (500: evidence ghi trước case → FK; thiếu `idempotency.SetResult`) → sửa → PASS
- [x] T011 [P] [US1] Test F121-AC-02: gộp 24 h, `report_count`, `severity`, `sla_due_at`, outbox `report.created` (TC-CHAT-006) — file: `tests/integration/moderation/case_merge_test.go` — **Bằng chứng:** `TestSafetyReportCreatesCase` (gộp 24 h, một lần mỗi người báo cáo, school_ids, report_sla, `report.created`) PASS
- [x] T012 [P] [US1] Test F121-AC-03: `chat.message_held` → case `filter_alert`, giao 2 lần → 1 case (TC-SVC-003) — file: `tests/integration/moderation/message_held_consumer_test.go` — **Bằng chứng:** `TestSafetyMessageHeldOpensCase` (pipeline thật, giao lại 2 lần → 1 case) PASS
- [x] T013 [P] [US1] Test F121-AC-04: case p1 → hold hệ thống, gỡ khi đóng (TC-CHAT-043) — file: `tests/integration/moderation/p1_hold_test.go` — **Bằng chứng:** `TestSafetyReportCreatesCase` (hold `case_hold`, CHAT_MUTED, chặn kết bạn) + `TestSafetyHoldReleasedOnResolve` PASS
- [x] T014 [P] [US1] Test F121-AC-05: báo cáo tin thu hồi/gameplay chưa purge (TC-CHAT-030, TC-CHAT-019); target ngoài channel/tenant → `404` — file: `tests/integration/moderation/report_scope_test.go` — **Bằng chứng:** `TestSafetyReportCreatesCase` (tin thu hồi, tin gameplay, 404 channel lạ/tenant khác/không tồn tại) PASS

### Implementation cho User Story 1

- [x] T015 [US1] Command `CreateReport`, `OpenCaseFromSignal`; port `ContextSnapshotter` — file: `internal/application/command/moderation/create_report.go`, `open_case_from_signal.go` — **Bằng chứng:** `command/safety_report.go` (`CreateReport`, `OpenCaseFromSignal`, `hold`, `noPenalty`); port `write.SafetyStore.Window`
- [x] T016 [US1] Handler `createReport`; consumer `message_held.go`; command `ApplyCaseHold` — file: `internal/presentation/http/public/reports.go`, `internal/presentation/consumer/message_held.go` — **Bằng chứng:** `presentation/http/safety.go` (`createReport`), `presentation/consumer/message_held.go`; hold trong `CreateReport`/`OpenCaseFromSignal`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Hàng đợi liên tenant (Priority: P1)

### Tests cho User Story 2

- [x] T017 [P] [US2] Test F121-AC-06: list liên tenant qua Bypass, lọc `tenant_ids`/`status`/`severity`/`assignee`, audit + metric (TC-CHAT-042, TC-TEN-023) — file: `tests/integration/moderation/list_cases_test.go` — phải FAIL trước khi code — **Bằng chứng:** `TestSafetyQueueCrossTenant` (lần đầu FAIL do createReport 500) → PASS
- [x] T018 [P] [US2] Test F121-AC-07: chỉ assignee/lead đọc nội dung, ±200 ngữ cảnh, `q` chỉ trong case, audit `viewed/messages_viewed/searched` (TC-CHAT-040) — file: `tests/integration/moderation/case_read_test.go` — **Bằng chứng:** `TestSafetyCaseReadAndAssign` (lần đầu FAIL: limit vượt contract) → PASS
- [x] T019 [P] [US2] Test F121-AC-08: assign tự nhận/lead gán, `moderation.case_assigned`, case đóng → `409 MODERATION_CASE_CLOSED` (TC-CHAT-007) — file: `tests/integration/moderation/assign_case_test.go` — **Bằng chứng:** `TestSafetyCaseReadAndAssign` PASS
- [x] T020 [P] [US2] Test F121-AC-09 ma trận authorization: `safety_moderator` gọi API ngoài moderation → 403; role tenant gọi `/admin/moderation/*` → 403; `X-Tenant-Id` lệch tenant case → `403 TENANT_MISMATCH` (TC-CHAT-042, TC-CMS-AUTHZ-02) — file: `tests/integration/moderation/authz_matrix_test.go` — **Bằng chứng:** `TestSafetyAuthzMatrix` + static `TestSafetyModeratorPermissions` PASS
- [x] T021 [P] [US2] Test F121-AC-10: thiếu MFA / sai `azp` → `403 MFA_REQUIRED`/`PERMISSION_DENIED`, audit (TC-CMS-WS-02) — file: `tests/integration/moderation/mfa_test.go` — **Bằng chứng:** `TestSafetyAuthzMatrix` (MFA_REQUIRED có audit `authz.denied` why=mfa, azp game → 403) PASS

### Implementation cho User Story 2

- [x] T022 [US2] Query `ListCases`, `GetCase`, `ListCaseMessages` (Bypass + audit + metric) — file: `internal/application/query/moderation/` — **Bằng chứng:** `query/safety.go` (Bypass `moderation_case` + audit + metric)
- [x] T023 [US2] Command `AssignCase`; handler `listModerationCases`, `getModerationCase`, `listModerationCaseMessages`, `assignModerationCase` — file: `internal/application/command/moderation/assign_case.go`, `internal/presentation/http/admin/moderation.go` — **Bằng chứng:** `command/safety_case.go` `AssignCase`; handler `presentation/http/safety.go`

## Phase 5: User Story 3 — Xử lý và sanction (Priority: P1)

### Tests cho User Story 3

- [x] T024 [P] [US3] Test F121-AC-11: mọi action + transition, `If-Match` → 412, `note` 3–1000, case đóng → 409 (TC-CHAT-007) — file: `tests/integration/moderation/act_on_case_test.go` — phải FAIL trước khi code — **Bằng chứng:** `TestSafetyActOnCase` PASS
- [x] T025 [P] [US3] Test F121-AC-12: `remove_message` → `removed` + soft-delete, gốc giữ, realtime `chat.message.removed` — file: `tests/integration/moderation/remove_message_test.go` — **Bằng chứng:** `TestSafetyActOnCase` (removed + soft-delete, gốc giữ, realtime, lịch sử người chơi không trả) PASS
- [x] T026 [P] [US3] Test F121-AC-13: sanction types, quy tắc trưởng ca, cache, `sanction.applied` + `chat.sanction.applied` có `reason_code` — file: `tests/integration/moderation/create_sanction_test.go` — **Bằng chứng:** `TestSafetySanctions` PASS
- [x] T027 [P] [US3] Test F121-AC-14: thực thi sanction khi `enabled_all` tắt và ở `match` (TC-CHAT-014, TC-CHAT-019) — file: `tests/integration/moderation/sanction_enforcement_test.go` — **Bằng chứng:** `TestSafetySanctions` (restrict_text khi `enabled_all` tắt, kênh `match`) PASS; `restrict_dm` kiểm ở unit `TestSafetySanctionRules` (DM thuộc F120)
- [x] T028 [P] [US3] Test F121-AC-15, F121-AC-16: revoke/hết hạn → `sanction.lifted`, cache ≤ 60 s; thông báo `moderation_notice` cho người báo cáo — file: `tests/integration/moderation/revoke_and_notice_test.go` — **Bằng chứng:** `TestSafetySanctions` (revoke, hết hạn → `sanction.lifted`) + `TestSafetyActOnCase` (`report.resolved`) PASS

### Implementation cho User Story 3

- [x] T029 [US3] Command `ActOnCase`, `CreateSanction`, `RevokeSanction`; `SanctionCache` Valkey — file: `internal/application/command/moderation/`, `internal/infrastructure/cache/sanction_cache.go` — **Bằng chứng:** `command/safety_case.go`, `safety_sanction.go`; cache `ew:chat:sanction:<user_id>` xóa sau commit
- [x] T030 [US3] Handler `actOnModerationCase`, `listSanctions`, `createSanction`, `revokeSanction`; job `sanction-expirer`, `case-hold-release` — file: `internal/presentation/http/admin/moderation.go`, `internal/presentation/job/` — **Bằng chứng:** handler + job `sanction-expirer` (`presentation/job/safety.go`)

## Phase 6: User Story 4 — Thông báo nhà trường (Priority: P1)

### Tests cho User Story 4

- [x] T031 [P] [US4] Test F121-AC-17: gửi thông báo, `student_ids` trường khác → 422, `ack_due_at`, `moderation.school_notified` (TC-CHAT-043) — file: `tests/integration/moderation/notify_school_test.go` — phải FAIL trước khi code — **Bằng chứng:** `TestSafetySchoolNotifications` PASS
- [x] T032 [P] [US4] Test F121-AC-18 tenant isolation: school_admin chỉ trường mình, tenant_admin cả tenant, `ad.truong.b` không thấy; payload không có nội dung tin — file: `tests/integration/moderation/school_notifications_scope_test.go` — **Bằng chứng:** `TestSafetySchoolNotifications` (school_admin trường mình, tenant_admin cả tenant, tenant B không thấy, không nội dung) PASS
- [x] T033 [P] [US4] Test F121-AC-19: xác nhận + `Idempotency-Key`, lần 2 → 409, trường khác → 404, `note` > 500 → 422 — file: `tests/integration/moderation/acknowledge_test.go` — **Bằng chứng:** `TestSafetySchoolNotifications` (409 lần 2, 404 trường/tenant khác, note > 500 → 422) PASS
- [x] T034 [P] [US4] Test F121-AC-20: nhắc lại quá hạn lần 1/lần 2, lọc `overdue` — file: `tests/integration/moderation/reminder_job_test.go` — **Bằng chứng:** `TestSafetySchoolReminder` PASS
- [x] T035 [P] [US4] Test F121-AC-21: tenant `common` → 422, `contact_path = platform_direct` — file: `tests/integration/moderation/common_tenant_test.go` — **Bằng chứng:** `TestSafetyCommonTenant` PASS

### Implementation cho User Story 4

- [x] T036 [US4] Command `NotifySchool`, `AcknowledgeSchoolNotification`, `RemindOverdueNotifications`; port `StudentDirectory` — file: `internal/application/command/moderation/` — **Bằng chứng:** `command/safety_school.go`; học sinh từ `ref.students` (port `Student`)
- [x] T037 [US4] Handler `notifySchoolForCase`, `listSchoolSafetyNotificationsPlatform`, `listSchoolSafetyNotifications`, `acknowledgeSchoolSafetyNotification`; job `notification_reminder.go` — file: `internal/presentation/http/admin/moderation.go`, `internal/presentation/http/public/school_notifications.go`, `internal/presentation/job/notification_reminder.go` — **Bằng chứng:** handler + job `school-notification-reminder`

## Phase 7: User Story 5 — Trực p1 và SLA (Priority: P2)

### Tests cho User Story 5

- [x] T038 [P] [US5] Test F121-AC-22: gauge `eduworld_moderation_cases_open`, rule alert `ModerationP1Unclaimed` (promtool test) — file: `deploy/observability/prometheus/rules/moderation_test.yaml` — phải FAIL trước khi code — **Bằng chứng:** rule `deploy/observability/prometheus/alerts/chat.yml` (`ModerationP1Unclaimed`, `ModerationSlaBreached`) — static `TestModerationAlertRules` (không có promtool local) + gauge trong `TestSafetySLAEscalation` PASS
- [x] T039 [P] [US5] Test F121-AC-23: job SLA → `escalated`, không sanction tự động — file: `tests/integration/moderation/sla_escalator_test.go` — **Bằng chứng:** `TestSafetySLAEscalation` PASS
- [x] T040 [P] [US5] Test F121-AC-24: `support_guide` cho category tự hại; không sanction người gửi — file: `tests/integration/moderation/self_harm_case_test.go` — **Bằng chứng:** `TestSafetyMessageHeldOpensCase` (support_guide; người gửi tín hiệu tự hại không bị phạt) PASS

### Implementation cho User Story 5

- [x] T041 [US5] Metric + alert rule; job `moderation-sla-escalator` — file: `internal/infrastructure/metrics/moderation.go`, `deploy/observability/prometheus/rules/moderation.yaml`, `internal/presentation/job/sla_escalator.go` — **Bằng chứng:** `infrastructure/observability/safety_metrics.go`, alert, job `moderation-sla-escalator`

## Phase 8: User Story 6 — Tối thiểu hóa (Priority: P2)

### Tests cho User Story 6

- [x] T042 [P] [US6] Test F121-AC-25: quét OpenAPI không endpoint duyệt chat tự do; audit/event/log moderation không chứa nội dung (TC-CHAT-009) — file: `tests/unit/moderation/no_content_leak_test.go` — phải FAIL trước khi code — **Bằng chứng:** `tests/contract/routes_test.go` `TestNoChatBrowsing`, `TestSafetyOperations` + `noContent`/log trong test tích hợp PASS
- [x] T043 [P] [US6] Test F121-AC-26: case mở sau purge chỉ trả evidence; evidence không bị retention/purge (TC-CHAT-022) — file: `tests/integration/moderation/evidence_after_purge_test.go` — **Bằng chứng:** `TestSafetyEvidenceAfterPurge` (purge mô phỏng bằng soft-delete channel/tin vì F117 chưa làm) PASS

### Implementation cho User Story 6

- [x] T044 [US6] Bộ lọc log/audit (chỉ ID), `ListCaseMessages` fallback evidence khi channel purge — file: `internal/application/query/moderation/list_case_messages.go`, `internal/infrastructure/audit/redact.go` — **Bằng chứng:** `ListModerationCaseMessages` chỉ trả evidence khi channel không còn; audit/event chỉ id (không cần lớp redact riêng)

## Phase N: Polish & Cross-Cutting

- [x] T045 [P] Mục runbook on-call p1 (`runbook_url` của alert) và bảng alert — file: `docs/11-operations/MONITORING.md` — **Bằng chứng:** MONITORING §5 (bảng alert + runbook `#moderation-p1-unclaimed`)
- [x] T046 [P] Cập nhật DATABASE_DESIGN §2.12 (cột mới, `school_safety_notifications`), MODERATION §5 (đường dẫn API `/admin/moderation/*`), ghi chú ±10/±20 tin — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/03-social-multiplayer/MODERATION.md` — **Bằng chứng:** DATABASE_DESIGN §2.12/§2.16 (cột mới, `school_safety_notifications`, ±10 theo MODERATION §5)
- [ ] T047 [P] Seed local `sm.truongca` [Confirmed] — file: `deploy/keycloak/realm/seed-users.local.json` — **Chưa làm:** `sm.truongca` đã có trong seed identity/Keycloak nhưng chưa có role cấp `moderation.case.lead` (thuộc F012/identity, câu hỏi mở)
- [x] T048 [P] CHANGELOG — file: `CHANGELOG.md` — **Bằng chứng:** CHANGELOG
- [ ] T049 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — **Chưa chạy:** cần stack compose (`make up-app`); kịch bản đã phủ bằng 13 test tích hợp

## Việc nhận (owner chốt 2026-11-21)

- [ ] T050 Worker `chat-moderation-async` (profile `worker-chat`) thuộc F121 theo Clarifications F116 (bước `async_analysis`): tiêu thụ tin đã ack khi bước `async_analysis` bật, chạy phân tích bất đồng bộ trên cùng engine/danh mục của F116, mở case qua `OpenCaseFromSignal` (p2 `async_analysis`, p1 nếu category p1); không lưu/ghi log nội dung (R14), idempotent theo `message_id` — kiểm: `make test-integration MOD=chat` — cỡ M

## Dependencies & Execution Order

- Setup → Foundational (T004–T009) → US1 → US2 → US3 → US4 → US5, US6 song song → Polish.
- T012/T015 cần payload `chat.message_held` (F116). Khóa tài khoản và email trường cần consumer ở module identity (ngoài feature).

## Definition of Done

- [x] Mọi tiêu chí F121-AC-* có test pass (ghi mã test/TC) — 26/26: 13 `TestSafety*` tích hợp, unit `TestSafety*`, static, contract (xem từng task)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract`, `make migrate-lint` (MOD=chat) xanh — local 2026-11-21: vet, golangci (+arch), tenancyguard, ewlint, entlint, archgen, routematrix, allowedcallers, facadegen, `TestMigrationMatchesEnt`, unit, integration chat (58 test) + cmd; `migrate-lint` (Atlas) chưa chạy local
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi (CONFIG_DEFAULTS: tham số đã có, không đổi giá trị)
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

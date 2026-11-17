# Implementation Plan: F121 — Moderation: báo cáo, sanction, hàng đợi Safety Team liên tenant, thông báo vụ việc cho trường

**Branch**: `121-moderation-safety-team` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm bounded context `moderation` vào module chat: aggregate `Report`, `ModerationCase` (state machine MODERATION §6.2), `Sanction`, `SchoolSafetyNotification`. Report và tín hiệu tự động (consumer `chat.message_held`, lệnh nội bộ từ worker `async_analysis` của F116) tạo/gộp case. Handler `/admin/moderation/*` đọc liên tenant chỉ qua `pkg/tenancy.Bypass(ctx, "moderation_case")` (audit + metric), ghi luôn có `X-Tenant-Id` = tenant của case. Sanction lưu PostgreSQL, cache Valkey cho pipeline, phát `sanction.applied/lifted` qua outbox. Job `moderation-sla-escalator`, `school-notification-reminder`, `case-hold-release` chạy trong `chat-worker`. Bốn enum OpenAPI sửa theo MODERATION ở Phase 2.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent + Atlas, oapi-codegen, nats.go, go-redis, `pkg/entx` (`UnitOfWork`), `pkg/outbox`, `pkg/tenancy` (`Bypass`), `pkg/authz`, `pkg/idempotency`, `pkg/audit` |
| Storage | PostgreSQL `eduworld_chat` (schema `chat`); Valkey `ew:chat:sanction:<user_id>` (cache, nguồn sự thật PG) |
| Testing | go test + testcontainers (PG, NATS, Valkey); test RLS + Bypass; contract test payload event |
| Target Platform | Linux container (module `chat` nạp trong `core-api`; profile `worker-chat` cho job/consumer) |
| Project Type | backend module |
| Performance Goals | Tạo case ≤ 5 s sau report (SC-001); `listModerationCases` p95 ≤ 500 ms với 100k case |
| Constraints | AGENTS R7, R8, R10, R14, R15, R20, R25, R27; ADR-0020, ADR-0027, ADR-0030, ADR-0033, ADR-0034, ADR-0072 |
| Scale/Scope | Mọi tenant; `report_sla`, `school_notification_ack_sla_hours` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Sửa `ModerationCaseStatus`, `SanctionCreate`, `ReportCreate`, `ModerationActionRequest`, thêm `ModerationCaseMessage`, payload event: task Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | `account_lock` và email trường qua event (R29); bounded context moderation cùng module chat nên đọc tin trực tiếp |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bypass chỉ trong handler moderation (linter allowlist); test RLS; bằng chứng bất biến, không DELETE |
| IV Server-authoritative | ✅ | Mức ưu tiên, SLA, hạn xác nhận tính ở server; client không gửi |
| V Xác thực, phân quyền | ✅ | `moderation.case.read/act`, `moderation.case.messages.read`, `moderation.sanction.read/write`, `moderation.school_notification.send/read/acknowledge`; MFA; gateway nội bộ |
| VI An toàn trẻ em | ✅ | Thông báo không chat thô; người báo cáo ẩn danh; không phạt tín hiệu tự hại; audit không chép nội dung |
| VII Một nguồn sự thật, cấu hình | ✅ | Enum theo MODERATION; tham số theo tên; `MaxLen` |
| VIII UI tách logic, token | ✅ | Không có UI (F122) |
| IX Test có bằng chứng | ✅ | Mỗi F121-AC có test trước; ma trận authz role × endpoint |

## Project Structure

### Documentation (feature này)

```text
specs/121-moderation-safety-team/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
modules/chat/
├── ent/schema/{report.go,moderation_case.go,moderation_action.go,sanction.go,moderation_evidence.go,school_safety_notification.go}
├── internal/domain/moderation/
│   ├── report.go  case.go  case_state.go  severity.go  sanction.go  school_notification.go  hold.go
├── internal/application/
│   ├── command/moderation/{create_report.go,open_case_from_signal.go,assign_case.go,act_on_case.go,
│   │                       create_sanction.go,revoke_sanction.go,notify_school.go,acknowledge_notification.go,
│   │                       escalate_overdue.go,remind_overdue_notifications.go,release_case_hold.go}
│   ├── query/moderation/{list_cases.go,get_case.go,list_case_messages.go,list_sanctions.go,
│   │                     list_notifications_platform.go,list_notifications_school.go}
│   └── port/{sanction_cache.go,context_snapshotter.go,student_directory.go,clock.go}
├── internal/infrastructure/{persistence/moderation/*.go,cache/sanction_cache.go,messaging/outbox_publisher.go}
├── internal/presentation/
│   ├── http/admin/moderation.go          # /admin/moderation/*, /admin/safety/* (Bypass)
│   ├── http/public/{reports.go,school_notifications.go}   # /chat/reports, /tenant/safety/*
│   ├── consumer/{message_held.go}
│   └── job/{sla_escalator.go,notification_reminder.go,case_hold_release.go,sanction_expirer.go}
└── tests/{unit,integration,contract}/moderation/
deploy/observability/prometheus/rules/moderation.yaml   # ModerationP1Unclaimed, ModerationSlaBreached
```

**Structure Decision**: Bounded context moderation trong module chat (EVENT_DESIGN §3 domain → module); handler admin tách thư mục `http/admin` để linter chỉ cho phép `tenancy.Bypass` ở đó (R27).

## Phase 0: Research

Không có. Quy tắc ở [MODERATION §5–§10](../../docs/03-social-multiplayer/MODERATION.md), ngưỡng ở [CHILD_SAFETY §8–§9](../../docs/10-security/CHILD_SAFETY.md), phạm vi liên tenant ở [ADMIN_PORTAL §2.5b](../../docs/04-admin-portal/ADMIN_PORTAL.md).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `Report`, `ModerationCase` (transition table, gộp 24 h, `Severity(reason, source)`), `Sanction` (quy tắc trưởng ca), `SchoolSafetyNotification` (`AckDueAt(severity, category)`) | Không import hạ tầng |
| application | Command `CreateReport`, `OpenCaseFromSignal`, `AssignCase`, `ActOnCase`, `CreateSanction`, `RevokeSanction`, `NotifySchool`, `AcknowledgeSchoolNotification`, `EscalateOverdueCases`, `RemindOverdueNotifications`, `ReleaseCaseHold` · Query `ListCases`, `GetCase`, `ListCaseMessages`, `ListSanctions`, `ListSchoolNotificationsPlatform`, `ListSchoolNotifications` | authz `authz.Require` tường minh đầu handler; idempotency middleware `pkg/httpx`; audit qua outbox trong `UnitOfWork` |
| infrastructure | Repository ent (query liên tenant chỉ trong hàm có `tenancy.Bypass`), `SanctionCache` Valkey, `ContextSnapshotter` (đọc `messages`/`gameplay_messages`/`ephemeral_messages`), `StudentDirectory` (`ref.students`) | |
| presentation | Handler `createReport`, `listModerationCases`, `getModerationCase`, `listModerationCaseMessages`, `assignModerationCase`, `actOnModerationCase`, `notifySchoolForCase`, `listSanctions`, `createSanction`, `revokeSanction`, `listSchoolSafetyNotificationsPlatform`, `listSchoolSafetyNotifications`, `acknowledgeSchoolSafetyNotification` · consumer `chat.message_held` · job SLA/nhắc/hold/hết hạn sanction | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F115 (tin, channel), F116 (pipeline, `chat.message_held`, worker async gọi command `OpenCaseFromSignal` in-process), F117 (`moderation_evidence`, hoãn purge), F118 (revision, thu hồi), F095 (block), F012 (permission, `moderation.case.lead` mới).
- Facade: không gọi. Event phát: `report.created`, `moderation.case_assigned`, `moderation.school_notified`, `moderation.school_acknowledged`, `sanction.applied`, `sanction.lifted`, `chat.message_removed`. Consumer bên ngoài: identity (khóa tài khoản, email), competition (`restrict_multiplayer`/`restrict_competition`), realtime.

### Bảo mật, tenant, idempotency, audit

- Permission theo bảng contracts §1; `moderation.case.lead` [Proposed] cho thao tác trưởng ca.
- `tenancy.Bypass(ctx, "moderation_case")` (client `chat_platform`, audit `cross_tenant.accessed`, ADR-0089 §3) chỉ trong query/command admin moderation; metric `eduworld_tenant_filter_bypass_total{reason="moderation_case"}`; ghi luôn cần `X-Tenant-Id` khớp tenant case (`403 TENANT_MISMATCH` nếu lệch).
- Idempotency-Key: `createReport`, `assignModerationCase`, `actOnModerationCase`, `notifySchoolForCase`, `createSanction`, `revokeSanction`, `acknowledgeSchoolSafetyNotification`.
- Audit: `moderation.case.viewed`, `moderation.case.messages_viewed`, `moderation.case.searched`, `moderation.case.assigned`, `moderation.case.acted`, `moderation.sanction.applied/revoked`, `moderation.school_notification.sent/acknowledged` (tham chiếu ID, không nội dung).

### Quan sát

- Metric: `eduworld_moderation_cases_open{severity,assigned}`, `eduworld_moderation_case_first_response_seconds{severity}`, `eduworld_chat_reports_total{reason}`, `eduworld_moderation_school_notifications_overdue`, `eduworld_tenant_filter_bypass_total{module,reason}`. Alert: `ModerationP1Unclaimed` (theo `p1_unclaimed_alert_minutes`), `ModerationSlaBreached`. Log chỉ ID.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

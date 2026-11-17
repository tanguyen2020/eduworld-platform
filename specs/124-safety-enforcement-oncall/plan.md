# Implementation Plan: F124 — Thực thi xử phạt và vận hành Safety Team: consumer `sanction.applied` (khóa tài khoản), email thông báo vụ việc cho trường, vòng trực, break-glass

**Branch**: `124-safety-enforcement-oncall` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Hai module, giao tiếp chỉ bằng event (R29). **module identity**: (1) consumer `sanction-enforcer` cho `sanction.applied/lifted` — aggregate `User` của F016 được mở rộng với tập `AccountLock` theo nguồn; trạng thái `locked` ⇔ còn khóa hiệu lực; tái dùng outbox `user.access_revoked/restored` và nhánh Keycloak của F016. (2) module `staffmail`: consumer `moderation.school_notified`, `moderation.school_notification_reminded`, `moderation.oncall_paged`, `break_glass.requested` → xác định người nhận theo UserRole/permission → ghi `email_deliveries` → worker `email-sender` gửi SMTP có retry. **module chat** (module moderation của F121): `OncallShift` + command phân công `rotation` khi tạo case p1 + job `oncall-escalator`; `BreakGlassGrant` (hai người duyệt) + query đọc tin qua `tenancy.Bypass(ctx, "break_glass")`. Không UI (đề xuất F122).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent + Atlas, oapi-codegen, nats.go, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox`, `src/backend/pkg/messaging`, `src/backend/pkg/tenancy`, `src/backend/pkg/authz`, `src/backend/pkg/idempotency`, `src/backend/pkg/audit`, `src/backend/pkg/modcall`, `src/backend/pkg/clock`; `net/smtp` (STARTTLS) + `text/template`/`html/template` |
| Storage | PostgreSQL `eduworld_identity` (`identity.account_locks`, `identity.email_deliveries`), `eduworld_chat` (`chat.moderation_oncall_shifts`, `chat.break_glass_grants`, cột mới `chat.moderation_cases`) |
| Testing | go test + testcontainers (PG, NATS, Keycloak, mailpit); Clock giả; contract test payload event chat ↔ identity |
| Target Platform | Linux container: module `identity`, `chat` nạp trong `core-api`; profile `worker-events` (identity) và `worker-chat` (chat) |
| Project Type | backend module ×2 |
| Performance Goals | Khóa ≤ 60 s REST / ≤ 1 phút Keycloak (AC-AUTH-006); email ≤ 5 phút (SC-003); job gọi trực chạy mỗi 1 phút |
| Constraints | AGENTS R7, R8, R10, R14, R20, R21, R27, R29; ADR-0027, ADR-0039, ADR-0042, ADR-0072 |
| Scale/Scope | Mọi tenant; `p1_unclaimed_alert_minutes`, `report_sla`, `school_notification_ack_sla_hours` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | 8 operation, 3 payload event, ProblemCode, permission mới ở Phase 2 (T004–T007) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | chat ↔ identity chỉ qua event + `resolvePrincipal` (facade `identity`); consumer gọi command handler qua interface có kiểu |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Consumer dùng `TenantScope` từ envelope + kiểm membership; Bypass `break_glass` chỉ trong handler System Admin (R27 — API liên tenant của System Admin); `moderation_oncall_shifts` là bảng platform không chứa dữ liệu học sinh; chỉ soft-delete |
| IV Server-authoritative | ✅ | Mốc gọi trực, hạn grant tính ở server; ghi có `Idempotency-Key` + audit |
| V Xác thực, phân quyền | ✅ | `moderation.case.read/lead`, `platform.break_glass.request/approve` platform-only; hai người khác nhau; MFA; gateway nội bộ |
| VI An toàn trẻ em | ✅ | Email/audit/log không tên học sinh, không nội dung; học sinh chỉ thấy `ACCOUNT_LOCKED` trung tính (R14, R16) |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên; 4 tham số mới [Proposed] đăng ký CONFIG_DEFAULTS (T046); SMTP/URL portal qua env (R3); mẫu email trong file dịch |
| VIII UI tách logic, token | ✅ | Không có UI; email HTML dùng màu từ `design/admin-tokens/` sinh sẵn |
| IX Test có bằng chứng | ✅ | Mỗi F124-AC có test viết trước; ma trận nguồn khóa; ma trận authz |

## Project Structure

### Documentation (feature này)

```text
specs/124-safety-enforcement-oncall/
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
src/backend/modules/identity/
├── ent/schema/{account_lock.go,email_delivery.go}
├── internal/domain/account/{lock.go,lock_source.go}              # User.AddLock/ReleaseLock, IsLocked()
├── internal/domain/staffmail/{template.go,recipient_rule.go}
├── internal/application/command/account/{apply_sanction_lock.go,release_sanction_lock.go}
├── internal/application/command/staffmail/{enqueue_school_notification.go,enqueue_oncall_page.go,
│                                           enqueue_break_glass_request.go,send_due_emails.go}
├── internal/application/port/{mail_sender.go,recipient_directory.go}
├── internal/infrastructure/mail/{smtp_sender.go,templates/{vi,en}/*.tmpl}
├── internal/infrastructure/persistence/{account_lock_repo.go,email_delivery_repo.go,recipient_directory.go}
├── internal/presentation/consumer/{sanction_enforcer.go,school_notification_mailer.go,oncall_page_mailer.go,break_glass_mailer.go}
├── internal/presentation/job/email_sender.go
└── tests/{unit,integration,contract}/{account_lock,staffmail}/
src/backend/modules/chat/
├── ent/schema/{moderation_oncall_shift.go,break_glass_grant.go}  (+ moderation_case.go: cột mới)
├── internal/domain/moderation/{oncall.go,escalation_ladder.go}
├── internal/domain/breakglass/{grant.go,scope.go}
├── internal/application/command/moderation/{create_oncall_shift.go,delete_oncall_shift.go,assign_by_rotation.go,escalate_unclaimed_p1.go}
├── internal/application/command/breakglass/{request.go,decide.go,revoke.go,expire.go}
├── internal/application/query/{moderation/list_oncall_shifts.go,breakglass/{list_grants.go,list_messages.go}}
├── internal/application/port/principal_directory.go
├── internal/infrastructure/client/identity_principal.go          # resolvePrincipal qua src/backend/pkg/modcall
├── internal/presentation/http/admin/{oncall.go,break_glass.go}   # Bypass chỉ ở đây + query breakglass
├── internal/presentation/job/{oncall_escalator.go,break_glass_expirer.go}
└── tests/{unit,integration,contract}/{oncall,breakglass}/
deploy/observability/prometheus/rules/moderation.yaml             # ModerationOncallGap, EmailDeliveryFailing
deploy/observability/alertmanager/alertmanager.yml                # route team=safety → safety-oncall
```

**Structure Decision**: Khóa tài khoản đặt trong aggregate User của identity (F016) để giữ một nguồn sự thật trạng thái; email nhân sự ở identity vì chỉ identity giữ email (ADR-0037). Vòng trực và break-glass thuộc module moderation/chat của module chat (EVENT_DESIGN §3: moderation → module chat), handler tách thư mục `http/admin` để linter giới hạn Bypass.

## Phase 0: Research

Không có. Quy tắc ở [MODERATION §6–§7, §9](../../docs/03-social-multiplayer/MODERATION.md), [CHILD_SAFETY §7, §11](../../docs/10-security/CHILD_SAFETY.md), [ACCOUNT_PROVISIONING §6](../../docs/09-identity/ACCOUNT_PROVISIONING.md). Gửi SMTP chuẩn (local mailpit) không cần đánh giá nhà cung cấp; nhà cung cấp email production theo OQ-10 [TBD], chỉ đổi cấu hình.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (identity) | `User.AddLock(source, ref)`, `User.ReleaseLock(source, ref)`, `User.Status()`; `RecipientRule` (school_admin → fallback tenant_admin); `Template` (khóa tham số cho phép) | Không import hạ tầng |
| application (identity) | Command `ApplySanctionLock`, `ReleaseSanctionLock`, `EnqueueSchoolSafetyEmail`, `EnqueueOncallPage`, `EnqueueBreakGlassRequestEmail`, `SendDueEmails`; sửa `UnlockUser`/`UnlockTenantUser` (F016) chỉ giải phóng nguồn của mình | Idempotency theo `event_id` ở consumer; audit qua outbox trong `UnitOfWork` |
| domain (chat) | `OncallShift` (không chồng lấn), `EscalationLadder(now, created_at, levels)`, `BreakGlassGrant` (two-person, `IsActive(now)`, `InScope(msg)`) | |
| application (chat) | Command `CreateOncallShift`, `DeleteOncallShift`, `AssignByRotation` (gọi trong `OpenCase` của F121 khi p1), `EscalateUnclaimedP1`, `RequestBreakGlass`, `DecideBreakGlass`, `RevokeBreakGlass`, `ExpireBreakGlass` · Query `ListOncallShifts`, `ListBreakGlassGrants`, `ListBreakGlassMessages` | Permission theo contracts §1 |
| infrastructure | Repo ent; `SmtpSender` (env `EW_SMTP_HOST`, `EW_SMTP_PORT`, `EW_SMTP_FROM`, secret `EW_SMTP_PASSWORD_FILE`); `RecipientDirectory` (UserRole + users); `IdentityPrincipalClient` (`resolvePrincipal`, `src/backend/pkg/modcall`); outbox publisher | |
| presentation | Consumer identity: `sanction.applied`, `sanction.lifted`, `moderation.school_notified`, `moderation.school_notification_reminded`, `moderation.oncall_paged`, `break_glass.requested` · Job `email-sender` (30 s), `oncall-escalator` (1 phút), `break-glass-expirer` (1 phút) · Handler 8 operationId | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F016 (lock/unlock, Keycloak, `user.access_*`), F015 (audit, platform settings), F121 (case, sanction, thông báo trường, job nhắc, `moderation.case.lead`), F012 (permission catalog, UserRole), F131 (Alertmanager).
- Facade: chat → identity `resolvePrincipal`. Event phát: `moderation.oncall_paged`, `moderation.school_notification_reminded` (thêm vào job F121), `break_glass.requested`, `moderation.case_assigned` (`rotation`), `user.*` (identity). Event tiêu thụ: xem contracts §4.
- Link trong email: `EW_PORTAL_BASE_URL` + đường dẫn route của F122 (cấu hình `config/email_links.yaml`), không hardcode URL môi trường (R3).

### Bảo mật, tenant, idempotency, audit

- Consumer identity: `TenantScope(envelope.tenant_id)`; học sinh không thuộc tenant đó → DLQ + audit `security.tenant_mismatch`. Người nhận email chỉ lấy trong tenant/trường của event.
- `tenancy.Bypass(ctx, "break_glass")` (reason trong danh sách đóng ADR-0089 §3, client `chat_platform`, audit `cross_tenant.accessed`) chỉ trong `query/breakglass/list_messages.go` (allowlist linter); metric `eduworld_tenant_filter_bypass_total{reason="break_glass"}`. Job `oncall-escalator` đọc case p1 qua Bypass `moderation_case` sẵn có của F121.
- Idempotency-Key: `createModerationOncallShift`, `deleteModerationOncallShift`, `createBreakGlassRequest`, `approve/reject/revokeBreakGlass*`; consumer theo `processed_events`; email theo UNIQUE `(source_event_id, recipient_user_id, template_code)`.
- Audit: `user.locked`/`user.unlocked` (nguồn), `moderation.oncall_shift.created/deleted`, `moderation.oncall.paged`, `break_glass.requested/approved/rejected/revoked/expired/messages_viewed`, `notification.email_sent` (chỉ `recipient_user_id`, template).

### Quan sát

- Metric: `eduworld_account_locks_active{source}`, `eduworld_email_deliveries_total{template,status}`, `eduworld_moderation_oncall_pages_total{level}`, `eduworld_moderation_oncall_coverage{oncall_role}`, `eduworld_break_glass_grants{status}`, `eduworld_tenant_filter_bypass_total{reason="break_glass"}`.
- Alert: `EmailDeliveryFailing` (tỷ lệ `failed` > 0 trong 15 phút), `ModerationOncallGap` (coverage = 0), `SchoolSafetyNotificationNoRecipient`; route `team=safety` → `safety-oncall`. Log chỉ ID, không địa chỉ email (R14).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

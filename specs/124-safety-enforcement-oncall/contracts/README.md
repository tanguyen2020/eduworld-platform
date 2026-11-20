# Contracts: F124 — Thực thi xử phạt và vận hành Safety Team: consumer `sanction.applied` (khóa tài khoản), email thông báo vụ việc cho trường, vòng trực, break-glass

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` + bản theo module `specs/openapi/modules/{chat,identity}.v1.yaml`. Route `/admin/*` chỉ gateway nội bộ, `azp = eduworld-portal` + MFA (ADR-0027).

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listModerationOncallShifts` | `GET /api/v1/admin/moderation/oncall-shifts?from=&to=&oncall_role=` | chat.v1 | `moderation.case.read` | Cần thêm |
| `createModerationOncallShift` | `POST /api/v1/admin/moderation/oncall-shifts` (`Idempotency-Key`) | chat.v1 | `moderation.case.lead` [Confirmed, F121] | Cần thêm |
| `deleteModerationOncallShift` | `DELETE /api/v1/admin/moderation/oncall-shifts/{shift_id}` (soft-delete) | chat.v1 | `moderation.case.lead` | Cần thêm |
| `createBreakGlassRequest` | `POST /api/v1/admin/break-glass/grants` (`X-Tenant-Id` đích, `Idempotency-Key`) | chat.v1 | `platform.break_glass.request` [Confirmed] | Cần thêm |
| `listBreakGlassGrants` | `GET /api/v1/admin/break-glass/grants?status=&tenant_ids=` | chat.v1 | `platform.break_glass.approve` hoặc `.request` (chỉ của mình) | Cần thêm |
| `approveBreakGlassRequest` · `rejectBreakGlassRequest` | `POST /api/v1/admin/break-glass/grants/{grant_id}:approve` · `:reject` (`If-Match`, `Idempotency-Key`) | chat.v1 | `platform.break_glass.approve` (khác người yêu cầu) | Cần thêm |
| `revokeBreakGlassGrant` | `POST /api/v1/admin/break-glass/grants/{grant_id}:revoke` | chat.v1 | `platform.break_glass.request` hoặc `.approve` | Cần thêm |
| `listBreakGlassMessages` | `GET /api/v1/admin/break-glass/grants/{grant_id}/messages?cursor=&limit=` | chat.v1 | người yêu cầu của grant `approved` còn hạn | Cần thêm |
| `assignModerationCase` · `getModerationCase` · `listModerationCases` | `/api/v1/admin/moderation/cases*` | eduworld-api | như F121 | Có sẵn — Cần sửa: `ModerationCase` thêm `assigned_via`, `oncall_page_level` |
| `unlockUser` · `unlockTenantUser` | `POST /api/v1/admin/users/{user_id}:unlock` · `/api/v1/tenant/users/{user_id}:unlock` | identity.v1 | như F016 | Cần thêm (F016) — Cần sửa: response `409 ACCOUNT_LOCK_HELD_BY_SAFETY` |
| `listTenantUsers` · `lockUser` | `GET /api/v1/tenant/users` · `POST /api/v1/admin/users/{user_id}:lock` | identity.v1 | như F012/F016 | Có sẵn — Cần sửa: item người dùng thêm `lock_sources[]` (enum `school_admin/platform_admin/sis/moderation_sanction`), không có `case_id` |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `resolvePrincipal` | module chat → module identity (kiểm người trực có `moderation.case.act`) | `specs/openapi/modules/identity.internal.v1.yaml` | `chat` có sẵn | Có sẵn |

Khóa tài khoản và email đi qua event (R29), không có operation facade mới.

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Đóng WS khi khóa dùng `user.access_revoked` của F016/F090 | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `sanction.applied` · `sanction.lifted` | chat (F116/F121) → identity | `specs/events/payloads/chat/sanction.applied.v1.schema.json`, `sanction.lifted.v1.schema.json` | Cần thêm (owner F116 cho `applied`, owner F121 cho `lifted`); cần `sanction_id`, `case_id`, `sanction_type`, `user_id`, `ends_at` |
| `moderation.school_notified` | chat (F121) → identity | `…/chat/moderation.school_notified.v1.schema.json` | Cần thêm (owner F121) — Cần sửa: thêm `notification_id`, `ack_due_at` |
| `moderation.school_notification_reminded` | chat → identity | `…/chat/moderation.school_notification_reminded.v1.schema.json` (`notification_id`, `tenant_id`, `school_id`, `reminder_no`, `ack_due_at`) | Cần thêm |
| `moderation.oncall_paged` | chat → identity | `…/chat/moderation.oncall_paged.v1.schema.json` (`case_id`, `severity`, `level` enum `primary/secondary/lead/system_admin`, `moderator_user_id?`) | Cần thêm |
| `moderation.case_assigned` | chat → AUD, NTF | `…/chat/moderation.case_assigned.v1.schema.json` (`assigned_via`) | Cần thêm (owner F121) |
| `break_glass.requested` | chat → identity | `…/chat/break_glass.requested.v1.schema.json` (`grant_id`, `tenant_id`, `requested_by`) | Cần thêm |
| `user.status_changed` · `user.access_revoked` · `user.access_restored` | identity → mọi module | `specs/events/payloads/identity/…` | Cần thêm (owner F011 cho `user.status_changed`/`user.access_revoked`, owner F016 cho `user.access_restored`) — `reason` thêm `moderation_sanction` |
| `audit.recorded` | chat, identity → identity | envelope chuẩn | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- **Cần thêm** (chat.v1 + eduworld-api): 8 operation ở §1 với schema `OncallShift`, `OncallShiftCreate` (`note` maxLength 500, `oncall_role` enum), `BreakGlassGrant`, `BreakGlassRequestCreate` (`reason` minLength 20 maxLength 1000, `scope_type` enum, `duration_minutes` integer có `maximum`), `BreakGlassDecision` (`decision_note` maxLength 1000), `BreakGlassRevoke` (`reason` maxLength 1000), `BreakGlassMessage` (giống `ModerationCaseMessage` của F121, không revision). Tag `x-internal: true`.
- **Cần sửa** `ModerationCase`: `assigned_via` enum `manual/rotation`, `oncall_page_level` enum.
- **Cần sửa** identity.v1: `unlockUser`/`unlockTenantUser` thêm `409`; item người dùng thêm `lock_sources[]` (enum, không chuỗi tự do).
- **Cần thêm** `ProblemCode`: `ACCOUNT_LOCK_HELD_BY_SAFETY`, `ONCALL_SHIFT_OVERLAP`, `BREAK_GLASS_NOT_PENDING`, `BREAK_GLASS_NOT_ACTIVE` (+ API_DESIGN §4.1).
- **Cần thêm** payload schema: `moderation.school_notification_reminded`, `moderation.oncall_paged`, `break_glass.requested`; **Cần sửa** `moderation.school_notified` (F121). Nếu F121 chưa thêm `sanction.applied/lifted` thì F124 chặn ở T005.
- Catalog permission: `platform.break_glass.request`, `platform.break_glass.approve` (platform-only).
- EVENT_DESIGN §4: thêm identity làm consumer của `sanction.applied/lifted`, `moderation.school_notified` (NTF email), event mới ở §4.

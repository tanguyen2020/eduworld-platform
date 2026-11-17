# Contracts: F121 — Moderation: báo cáo, sanction, hàng đợi Safety Team liên tenant, thông báo vụ việc cho trường

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (`x-service: chat`). Route `/admin/*` chỉ gateway nội bộ, `azp = eduworld-portal` + MFA.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createReport` | `POST /api/v1/chat/reports` | eduworld-api | học sinh (`game_player`) | Có sẵn (Cần sửa `ReportCreate`) |
| `listModerationCases` | `GET /api/v1/admin/moderation/cases` | eduworld-api | `moderation.case.read` | Có sẵn |
| `getModerationCase` | `GET /api/v1/admin/moderation/cases/{case_id}` | eduworld-api | `moderation.case.read` (+ assignee/lead) | Có sẵn (Cần sửa `ModerationCase`) |
| `listModerationCaseMessages` | `GET /api/v1/admin/moderation/cases/{case_id}/messages` | eduworld-api | `moderation.case.messages.read` | Có sẵn (Cần sửa item → `ModerationCaseMessage`) |
| `assignModerationCase` | `POST /api/v1/admin/moderation/cases/{case_id}:assign` | eduworld-api | `moderation.case.act` | Có sẵn |
| `actOnModerationCase` | `POST /api/v1/admin/moderation/cases/{case_id}:act` | eduworld-api | `moderation.case.act` | Có sẵn (Cần sửa `ModerationActionRequest`) |
| `notifySchoolForCase` | `POST /api/v1/admin/moderation/cases/{case_id}:notify-school` | eduworld-api | `moderation.school_notification.send` | Có sẵn |
| `listSanctions` · `createSanction` | `GET/POST /api/v1/admin/moderation/sanctions` | eduworld-api | `moderation.sanction.read` / `.write` | Có sẵn (Cần sửa `SanctionCreate`) |
| `revokeSanction` | `POST /api/v1/admin/moderation/sanctions/{sanction_id}:revoke` | eduworld-api | `moderation.sanction.write` | Có sẵn |
| `listSchoolSafetyNotificationsPlatform` | `GET /api/v1/admin/safety/school-notifications` | eduworld-api | `moderation.school_notification.send` | Có sẵn |
| `listSchoolSafetyNotifications` | `GET /api/v1/tenant/safety/school-notifications` | eduworld-api | `moderation.school_notification.read` | Có sẵn |
| `acknowledgeSchoolSafetyNotification` | `POST /api/v1/tenant/safety/school-notifications/{notification_id}:acknowledge` | eduworld-api | `moderation.school_notification.acknowledge` | Có sẵn |
| `listNotifications` | `GET /api/v1/notifications` | eduworld-api | học sinh (thông báo `moderation_notice`) | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không (khóa tài khoản, email trường qua event; R29) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `chat.sanction.applied` | server → người bị áp | Có sẵn |
| `chat.message.removed` | server → thành viên channel | Có sẵn |
| `notification.item.created` | server → học sinh (kết quả báo cáo) | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `chat.message_held` | Tiêu thụ (F116) | `specs/events/payloads/chat/chat.message_held.v1.schema.json` | Cần thêm (owner F116) |
| `report.created` | Phát | `specs/events/payloads/chat/report.created.v1.schema.json` | Cần thêm |
| `moderation.case_assigned` | Phát | `…/moderation.case_assigned.v1.schema.json` | Cần thêm |
| `moderation.school_notified` · `moderation.school_acknowledged` | Phát (consumer identity: email) | `…/moderation.school_*.v1.schema.json` | Cần thêm |
| `sanction.applied` | Phát (consumer identity: `account_lock`; competition: `restrict_*`) | `…/sanction.applied.v1.schema.json` | Có ở F116 (owner F116) — F121 thêm `case_id` tương thích |
| `sanction.lifted` | Phát (consumer identity, competition) | `…/sanction.lifted.v1.schema.json` | Cần thêm |
| `chat.message_removed` | Phát | `…/chat.message_removed.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không | — |

## 6. Delta cần thực hiện

Task Phase 2: T004–T007.

- **Cần sửa** `ModerationCaseStatus` → `[open, in_review, escalated, resolved, reopened]` (MODERATION §6.2); cập nhật mô tả `MODERATION_CASE_CLOSED` ở API_DESIGN (§4.1 ghi `resolved`/`dismissed`).
- **Cần sửa** `ReportCreate.reason` → `[rude_language, bullying, asked_personal_info, spam, inappropriate_content, cheating, other]`; thêm `context` {`channel_id` uuid, `match_id` uuid, `party_id` uuid} (`additionalProperties: false`); `note` giữ maxLength 500.
- **Cần sửa** `SanctionCreate.sanction_type` → `[warning, mute, restrict_text, restrict_dm, restrict_multiplayer, restrict_competition, account_lock]`; thêm `scope` {`channel_id`?} ; `Sanction` thêm `reason_code` maxLength 64, `issued_by_kind` enum `system|moderator`, `revoked_by`, `revoke_reason` maxLength 1000.
- **Cần sửa** `ModerationActionRequest.action` thêm `reopen`; thêm `sanction_type` (bắt buộc khi `action ∈ {mute, restrict}`).
- **Cần sửa** `ModerationCase` thêm `school_ids` (maxItems 20), `first_response_due_at`, `contact_path` enum `school|platform_direct`, `contact_outcome`, `version`; `ModerationCaseDetail` thêm `support_guide` (maxLength 64), `evidence[]` {`evidence_id`, `created_at`}.
- **Cần thêm** schema `ModerationCaseMessage` {`message_id`, `channel_id`, `channel_type`, `seq`, `sender_id`, `sender_nickname` maxLength 32, `sender_school_id`, `kind`, `body` maxLength 1000 (nội dung gốc), `body_display` maxLength 1000, `revisions[]` {`revision`, `body` maxLength 1000, `decision` maxLength 64, `edited_at`} maxItems 50, `moderation_state`, `recalled_at`, `created_at`, `in_snapshot` bool}; `listModerationCaseMessages` trả `ModerationCaseMessage`.
- **Cần thêm** payload schema event ở mục 4 (phạm vi chat), theo [EVENT_DESIGN §4.9](../../../docs/07-backend/EVENT_DESIGN.md); không chứa nội dung tin.
- **Tài liệu**: thêm `moderation.case.lead` vào ADMIN_PORTAL §2.2 [Proposed]; thống nhất tên audit `moderation.case.viewed` (MODERATION) với `moderation_case.viewed` (ADMIN_PORTAL §2.5b).

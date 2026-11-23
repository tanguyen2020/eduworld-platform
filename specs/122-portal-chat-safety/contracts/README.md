# Contracts: F122 — Portal an toàn chat: kiểm duyệt (E), danh mục và công tắc (D), chính sách chat trường (C)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (`x-service: chat`). `/admin/*` chỉ gateway nội bộ + role platform.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listModerationCases` · `getModerationCase` · `listModerationCaseMessages` | `GET /api/v1/admin/moderation/cases[/{case_id}[/messages]]` | eduworld-api | `moderation.case.read`, `moderation.case.messages.read` | Có sẵn (sửa schema ở F121) |
| `assignModerationCase` · `actOnModerationCase` · `notifySchoolForCase` | `POST /api/v1/admin/moderation/cases/{case_id}:assign|:act|:notify-school` | eduworld-api | `moderation.case.act`, `moderation.school_notification.send` | Có sẵn |
| `listSanctions` · `createSanction` · `revokeSanction` | `/api/v1/admin/moderation/sanctions[/{sanction_id}:revoke]` | eduworld-api | `moderation.sanction.read/write` | Có sẵn |
| `listSchoolSafetyNotificationsPlatform` | `GET /api/v1/admin/safety/school-notifications` | eduworld-api | `moderation.school_notification.send` | Có sẵn |
| `listSchoolSafetyNotifications` · `acknowledgeSchoolSafetyNotification` | `GET /api/v1/tenant/safety/school-notifications` · `POST …/{notification_id}:acknowledge` | eduworld-api | `moderation.school_notification.read/acknowledge` | Có sẵn |
| `listSensitiveCategories` · `createSensitiveCategory` · `updateSensitiveCategory` · `deleteSensitiveCategory` | `/api/v1/admin/chat/sensitive-categories[/{category_id}]` | eduworld-api | `chat.sensitive_terms.read/write` | Có sẵn |
| `listSensitiveTerms` · `createSensitiveTerm` · `updateSensitiveTerm` · `deleteSensitiveTerm` · `testSensitiveTerms` | `/api/v1/admin/chat/sensitive-terms[/{term_id}|:test]` | eduworld-api | `chat.sensitive_terms.read/write` | Có sẵn |
| `listSensitiveAllowlist` · `createSensitiveAllowlistEntry` · `deleteSensitiveAllowlistEntry` | `/api/v1/admin/chat/sensitive-allowlist[/{entry_id}]` | eduworld-api | `chat.sensitive_terms.write` | Có sẵn |
| `publishSensitiveCatalog` · `listSensitiveCatalogVersions` | `POST /api/v1/admin/chat/sensitive-catalog:publish` · `GET …/versions` | eduworld-api | `chat.sensitive_terms.write` / `.read` | Có sẵn |
| `listModerationSwitches` · `setModerationSwitches` · `listModerationSwitchHistory` | `/api/v1/admin/chat/moderation-switches[/history]` | eduworld-api | `chat.policy.toggle` (đọc: + Safety Moderator) | Có sẵn |
| `listTenantSensitiveTerms` · `createTenantSensitiveTerm` · `updateTenantSensitiveTerm` · `deleteTenantSensitiveTerm` · `publishTenantSensitiveCatalog` | `/api/v1/admin/chat/tenant-terms[/{term_id}|:publish]` | eduworld-api | `chat.tenant_terms.write` | Có sẵn |
| `toggleTenantSensitiveCategory` | `POST /api/v1/admin/chat/tenant-categories/{code}:toggle` | eduworld-api | `chat.tenant_category.toggle` | Có sẵn |
| `listTenantSensitiveCategories` | `GET /api/v1/tenant/chat/categories` | eduworld-api | `chat.sensitive_terms.read` | Có sẵn |
| `getTenantModerationStatus` | `GET /api/v1/tenant/chat/moderation-status` | eduworld-api | nhân sự tenant đã đăng nhập (`tenant.settings.read` hoặc `school.settings.read`) | Có sẵn (F122 T004/T005, 2026-11-23; `x-portal-shell`) |
| `getChatPolicy` · `putChatPolicy` · `getEffectiveChatPolicy` | `/api/v1/tenant/chat-policies/…` | eduworld-api | `school.settings.read/write` | Có sẵn (F120) |
| `listClubs` · `approveClub` · `archiveClub` · `removeClubMember` | `/api/v1/social/clubs…` | eduworld-api | `social.club.approve` | Có sẵn (F120) — `removeClubMember` là route `game_player`, nhân sự không gọi được (câu hỏi mở, spec Clarifications 2026-11-23) |
| `getMyPermissions` | `GET /api/v1/me/permissions` | eduworld-api | đã đăng nhập | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (frontend) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Portal không dùng WebSocket (poll 30 s cho banner/hàng đợi) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Portal không phát/tiêu thụ | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/chat-policy.schema.json` | Rule form chính sách chat (giới hạn, enum) | Có sẵn |

## 6. Delta cần thực hiện

Task Phase 2: T004.

- **Đã thêm (2026-11-23)** `getTenantModerationStatus` — `x-portal-shell: true` thay cho action catalog (banner thuộc vỏ portal); `GET /tenant/chat/moderation-status` (`x-service: chat`, header `X-Tenant-Id`) → `TenantModerationStatus` {`tenant_id`, `enabled_all` bool, `disabled_steps[]` ($ref `ModerationStep`, maxItems 5), `expires_at` Timestamp nullable, `source_scope` enum `platform|tenant`} — không có `reason`, không có `actor_id`; `403` khi không có quyền tenant. Đề xuất owner module chat (F116) — feature này tạo task contract, hiện thực backend ở F116 hoặc F122 tùy ai làm trước.
- Phụ thuộc delta của F120 (`getChatPolicy`, `putChatPolicy`, `getEffectiveChatPolicy`, `listClubs` lọc `status`/`school_id`) và F121 (`ModerationCaseMessage`, enum sanction/trạng thái case).
- ADR-0078: UI dùng `approveModerationSwitchChange` và API ghi `chat_policy_preset` (owner F116, F120).

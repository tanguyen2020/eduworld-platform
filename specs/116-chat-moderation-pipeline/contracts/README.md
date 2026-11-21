# Contracts: F116 — Pipeline kiểm tra tin nhắn, danh mục từ nhạy cảm, công tắc kiểm duyệt

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/chat.v1.yaml`. Mọi `/admin/chat/*` chỉ gateway nội bộ, `azp = eduworld-portal` + role platform (R20).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listSensitiveCategories` | `GET /api/v1/admin/chat/sensitive-categories` | chat.v1.yaml | `chat.sensitive_terms.read` | Có sẵn |
| `createSensitiveCategory` | `POST /api/v1/admin/chat/sensitive-categories` (`Idempotency-Key`) | chat.v1.yaml | `chat.sensitive_terms.write` | Có sẵn |
| `updateSensitiveCategory` / `deleteSensitiveCategory` | `PATCH` (`If-Match`) / `DELETE` (xóa mềm) `/api/v1/admin/chat/sensitive-categories/{category_id}` | chat.v1.yaml | `chat.sensitive_terms.write` | Có sẵn |
| `listSensitiveTerms` | `GET /api/v1/admin/chat/sensitive-terms?category_id=&q=&tenant_ids=` | chat.v1.yaml | `chat.sensitive_terms.read` | Có sẵn |
| `createSensitiveTerm` / `updateSensitiveTerm` / `deleteSensitiveTerm` | `POST` · `PATCH`/`DELETE …/sensitive-terms/{term_id}` | chat.v1.yaml | `chat.sensitive_terms.write` | Có sẵn |
| `testSensitiveTerms` | `POST /api/v1/admin/chat/sensitive-terms:test` | chat.v1.yaml | `chat.sensitive_terms.read` | Có sẵn |
| `listSensitiveAllowlist` / `createSensitiveAllowlistEntry` / `deleteSensitiveAllowlistEntry` | `/api/v1/admin/chat/sensitive-allowlist[/{entry_id}]` | chat.v1.yaml | `chat.sensitive_terms.read` / `.write` | Có sẵn |
| `publishSensitiveCatalog` | `POST /api/v1/admin/chat/sensitive-catalog:publish` (`Idempotency-Key`; `422 SENSITIVE_CATALOG_REGRESSION_FAILED`) | chat.v1.yaml | `chat.sensitive_terms.write` | Có sẵn |
| `listSensitiveCatalogVersions` | `GET /api/v1/admin/chat/sensitive-catalog/versions?tenant_ids=` | chat.v1.yaml | `chat.sensitive_terms.read` | Có sẵn |
| `listModerationSwitches` | `GET /api/v1/admin/chat/moderation-switches?tenant_ids=` | chat.v1.yaml | `chat.policy.toggle` | Có sẵn |
| `setModerationSwitches` | `PUT /api/v1/admin/chat/moderation-switches` (`Idempotency-Key`; `422 MODERATION_SWITCH_REASON_REQUIRED`) | chat.v1.yaml | `chat.policy.toggle` | Có sẵn |
| `listModerationSwitchHistory` | `GET /api/v1/admin/chat/moderation-switches/history?tenant_ids=` | chat.v1.yaml | `chat.policy.toggle` | Có sẵn |
| `listTenantSensitiveTerms` / `createTenantSensitiveTerm` / `updateTenantSensitiveTerm` / `deleteTenantSensitiveTerm` | `/api/v1/admin/chat/tenant-terms[/{term_id}]` (`X-Tenant-Id` = tenant đích) | chat.v1.yaml | `chat.sensitive_terms.read` (list) · `chat.tenant_terms.write` | Có sẵn |
| `publishTenantSensitiveCatalog` | `POST /api/v1/admin/chat/tenant-terms:publish` | chat.v1.yaml | `chat.tenant_terms.write` | Có sẵn |
| `toggleTenantSensitiveCategory` | `POST /api/v1/admin/chat/tenant-categories/{code}:toggle` (`403 TENANT_TOGGLE_NOT_ALLOWED`) | chat.v1.yaml | `chat.tenant_category.toggle` | Có sẵn |
| `listTenantSensitiveCategories` | `GET /api/v1/tenant/chat/categories` (gateway public) | chat.v1.yaml | `chat.sensitive_terms.read` (Tenant Admin, School Admin: chỉ category, không term) | Có sẵn |
| `listQuickMessages` | `GET /api/v1/chat/quick-messages?channel_type=` | chat.v1.yaml | `game_player` | Có sẵn |
| `sendChannelMessage` (`451`) | `POST /api/v1/chat/channels/{channel_id}/messages` | chat.v1.yaml | `game_player` | Cần sửa (`category_code` trong Problem; `params` cho quick) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `system.command.failed` (`CHAT_CONTENT_BLOCKED` + `details.category_code`, `CHAT_MUTED`, `CHAT_RATE_LIMITED`) | server → người gửi | Cần sửa (khai báo `details.category_code` maxLength 64) |
| `chat.message.held` | server → người gửi | Có sẵn |
| `chat.message.send` (`kind = quick`, `quick_message_id`, `params`) | client → server | Cần sửa (thêm `params`) |
| `chat.sanction.applied` | server → người bị mute | Có sẵn (enum `sanction_type` lệch MODERATION §7 — ghi ở §6) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `sensitive_term.created/updated/deleted` | chat → AUD | `specs/events/payloads/chat/sensitive_term.created.v1.schema.json` (+ `updated`, `deleted`) | Cần thêm |
| `sensitive_catalog.published` | chat → profile `worker-chat` (pub/sub version), AUD | `…/sensitive_catalog.published.v1.schema.json` | Cần thêm |
| `chat_policy.moderation_switch_changed` | chat → chat (invalidate), AUD, MON, NTF Tenant Admin | `…/chat_policy.moderation_switch_changed.v1.schema.json` | Cần thêm |
| `chat_policy.tenant_category_toggled` | chat → chat (invalidate), AUD | `…/chat_policy.tenant_category_toggled.v1.schema.json` | Cần thêm |
| `chat.message_held` | chat-persister → MOD (F121) | `…/chat.message_held.v1.schema.json` | Cần thêm |
| `sanction.applied` | chat → chat (cache), RT, AUD | `…/sanction.applied.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/sensitive-term-catalog.schema.json` | Xuất/nhập snapshot danh mục, checksum version | Có sẵn |
| `specs/schemas/chat-policy.schema.json` | `validation.strictness`, `sensitive_term_policy` của effective policy | Có sẵn |

## 6. Delta cần thực hiện

- `Problem`: thêm `category_code` (string, maxLength 64) cho `CHAT_CONTENT_BLOCKED`/`CHAT_CONTENT_HELD` (hiện chỉ có `policy_reason`); AsyncAPI `system.command.failed.details.category_code`.
- `MessageCreate` và AsyncAPI `chat.message.send`: thêm `params` (object, `maxProperties` 5, value string maxLength 64) cho quick message có tham số (CHAT_SYSTEM §8.5); thêm response `403` (`CHAT_MUTED`) cho `sendChannelMessage`.
- AsyncAPI `chat.sanction.applied.sanction_type` (`restrict_quick_only`, `chat_disabled`) lệch `sanction_type` MODERATION §7 (`restrict_text`, …) → thống nhất theo MODERATION (F121 chốt; F116 chỉ phát `mute`).
- Event payload `specs/events/payloads/chat/`: 6 schema ở §4.
- Cần thêm (không chặn F116): API quản lý bảng teencode/leetspeak (MODERATION §3.1, §9) — đề xuất `GET/PUT /admin/chat/normalization-maps` (`chat.sensitive_terms.write`), UI ở F122.
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`.
- ADR-0078 (RB-801): `ModerationSwitches.steps` bỏ `pii`; `expires_at` bắt buộc khi tắt; yêu cầu tắt trả `202` `pending_approval`; thêm `approveModerationSwitchChange`; mã `MODERATION_SWITCH_STEP_NOT_ALLOWED`, `MODERATION_SWITCH_EXPIRY_REQUIRED`, `MODERATION_SWITCH_SELF_APPROVAL` (đã thêm vào `ProblemCode`, API_DESIGN §4.1) — task T041.

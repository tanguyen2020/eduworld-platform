# Contracts: F136 — Nickname tự gõ qua kiểm duyệt (giữ "Gợi ý"), báo cáo và đặt lại nickname

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Dùng cho | Trạng thái |
|---|---|---|---|
| `updateGameProfile` | `PATCH /api/v1/game/profile` (`application/merge-patch+json`, `GameProfileUpdate`) | Đặt/đổi nickname: `nickname` (tự gõ, `maxLength: 32`) **hoặc** `nickname_option_id` (`oneOf`); `422 NICKNAME_NOT_ALLOWED` (một mã chung), `409 NICKNAME_UNAVAILABLE`, `409 NICKNAME_CHANGE_LIMIT` (+ `retry_after`), `429 RATE_LIMITED` (+ `Retry-After`), `503 UNAVAILABLE`, `422 VALIDATION_FAILED` (cả hai trường) | Có sẵn (F070) — **Sửa** (`specs/openapi/modules/game.v1.yaml`) |
| `suggestNicknames` | `GET /api/v1/game/profile/nickname-suggestions` | Nút "Gợi ý"/"Gợi ý khác" | Có sẵn (F070) — không đổi |
| `getGameProfile` | `GET /api/v1/game/profile` | `setup_required` (cả sau đặt lại), `nickname` `null`; thêm `nickname_changes_left` (int, số lần đổi còn trong tháng) và `nickname_change_available_at` (date-time, khi hết lượt) | Có sẵn (F070) — **Sửa** (thêm hai trường chỉ đọc) |
| `resetStudentNickname` | `POST /api/v1/tenant/students/{student_id}/game-profile/nickname:reset` (`Idempotency-Key`, `X-Tenant-Id`; body `{reason}` `maxLength: 500`) → `200 {student_id, nickname: null, reset_at}`; `403`, `404`, `422` | School Admin/Tenant Admin/System Admin đặt lại nickname; permission `lms.student.nickname_reset`; `x-portal-action` cho trang học sinh | **Mới** (`game.v1.yaml`, module game) |
| `createReport` | `POST /api/v1/chat/reports` | Báo cáo nickname: `target_type = user`, `reason = inappropriate_content`, `context.reported_field` enum `[nickname]` | Có sẵn (F121) — **Sửa** (`chat.v1.yaml`: thêm `reported_field`) |
| Hành động case (`actOnModerationCase`, `POST /api/v1/admin/moderation/cases/{case_id}:act`) | gateway nội bộ, `safety_moderator` | `action = nickname_reset` (tạo sanction tức thời) | Có sẵn (F121) — **Sửa** (thêm giá trị enum `action`) |
| `getModerationCase` | gateway nội bộ | Bằng chứng hiện `reported_nickname` (bản chụp ≤ 32) khi case có báo cáo nickname | Có sẵn (F121) — **Sửa** (trường chỉ đọc) |

Mọi route người chơi vẫn qua middleware `game_player`; `resetStudentNickname` là route tenant (portal, `azp = eduworld-portal`).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `moderateDisplayText` | game → chat | `specs/openapi/modules/chat.internal.v1.yaml` | `[competition, game]` | Có sẵn (F116) — **Sửa**: `DisplayTextCheckRequest.context` thêm `nickname`; thêm `game` vào caller |
| `screenNicknamePersonalData` | game → identity | `specs/openapi/modules/identity.internal.v1.yaml` (`POST /internal/v1/students/{student_id}/nickname:screen`, body `{text}` `maxLength: 32`) → `{decision: allowed\|blocked}` | `[game]` | **Mới** |
| `anonymizeUserData` | identity (saga) → game | `specs/openapi/modules/game.internal.v1.yaml` | (giữ) | Có sẵn (F070) — hành vi mở rộng: ẩn danh `game.nickname_changes` (không đổi contract) |

Mọi operation dùng `x-context-required: [tenant_scope]`; không trả họ tên, tên trường, từ đã khớp hay danh mục (R15, R16).

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có message mới; tên mới đến client khác qua `ref.player_profiles` (projection của `game_profile.updated`) như F070/F091 | — |

## 4. Event (NATS)

| Event | Producer → Consumer | Schema | Trạng thái |
|---|---|---|---|
| `game_profile.updated` | game phát (owner F070) → competition, chat (`ref.player_profiles`) | `specs/events/payloads/game/game_profile.updated.v1.schema.json` | Có sẵn — không đổi (`nickname` `null` hợp lệ) |
| `sanction.applied` | Tiêu thụ — chat phát (owner F116) → game (consumer `game.sanction_nickname_reset`), identity, competition | `specs/events/payloads/chat/sanction.applied.v1.schema.json` | Có sẵn — **Sửa** (owner F116): enum `sanction_type` thêm `nickname_reset` |

## 5. JSON Schema nội dung

| Schema | Thay đổi |
|---|---|
| — | Không có schema nội dung mới (danh mục từ nhạy cảm là dữ liệu CMS sẵn có, F116) |

## 6. Delta cần thực hiện

| # | File | Thay đổi | Task |
|---|---|---|---|
| D1 | `specs/openapi/modules/game.v1.yaml` | `GameProfileUpdate` = `oneOf` (`nickname` string `minLength: 1`, `maxLength: 32` \| `nickname_option_id` uuid) + `settings`/`preferred_language`; bỏ câu "trường nickname tự do bị từ chối 422 (F070-AC-10)"; response `422 NICKNAME_NOT_ALLOWED`, `429`, `503`; `GameProfile` thêm `nickname_changes_left`, `nickname_change_available_at`; operation mới `resetStudentNickname` (`IdempotencyKey`, `TenantIdHeader`, `reason` `maxLength: 500`, `x-portal-action`) | T001 |
| D2 | `specs/openapi/modules/chat.v1.yaml` | `ReportCreate.context.reported_field` enum `[nickname]`; `action` của case thêm `nickname_reset`; bằng chứng case thêm `reported_nickname` (`maxLength: 32`) | T002 |
| D3 | `specs/openapi/modules/chat.internal.v1.yaml` | `DisplayTextCheckRequest.context` thêm `nickname`, `text` `maxLength` giữ 255 (nickname ≤ 32 do bên gọi); `x-allowed-callers: [competition, game]`; ví dụ `context: nickname` | T003 |
| D4 | `specs/openapi/modules/identity.internal.v1.yaml` | Operation mới `screenNicknamePersonalData` + schema `NicknameScreenRequest` (`text` `maxLength: 32`), `NicknameScreenResult` (`decision` enum) + ví dụ | T003 |
| D5 | `specs/events/payloads/chat/sanction.applied.v1.schema.json` (owner F116) | `sanction_type` enum thêm `nickname_reset` (thêm giá trị, không breaking cho consumer bỏ qua loại lạ) | T004 |
| D6 | `src/backend/pkg/authz/catalog.go` + `specs/portal/menu-catalog.yaml` | Permission tenant `lms.student.nickname_reset` (School Admin, Tenant Admin; System Admin có sẵn qua tenant đích); action portal "Đặt lại nickname" | T005 |
| D7 | Problem codes (`docs/07-backend/API_DESIGN.md` bảng mã lỗi, `check-problem-codes`) | `NICKNAME_NOT_ALLOWED` (422) — tài liệu API_DESIGN do task Polish cập nhật | T005 |
| D8 | Bundle `specs/openapi/eduworld-api.v1.yaml` | `node scripts/openapi/split-public.mjs` / `make gen MOD=game`, `MOD=chat`, `MOD=identity`; `make specs-breaking` (thêm `oneOf` cho request là mở rộng; ghi `breaking-allowlist.yaml` nếu oasdiff báo) | T006 |

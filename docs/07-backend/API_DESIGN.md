# API Design

> Trạng thái: Draft · Phiên bản: 0.2 (microservices) · Owner: Backend Architect · Cập nhật: 2026-11-15
> Liên quan: [ADR-0036](../adr/ADR-0036-microservices-theo-he-thong-lon-thay-adr-0001.md) · [ADR-0040](../adr/ADR-0040-goi-service-to-service-bang-typed-rest-client-sinh-tu-openap.md) · [ADR-0041](../adr/ADR-0041-moi-cot-truong-kieu-chuoi-phai-co-gioi-han-do-dai.md) · [Specs theo service](../../specs/openapi/services) · [ADR-0008](../adr/ADR-0008-access-token-cua-keycloak-la-credential-api-realtime-dung-ti.md) · [ADR-0009](../adr/ADR-0009-apache-apisix-lam-api-gateway-backend-van-tu-authorize.md) · [ADR-0010](../adr/ADR-0010-rest-openapi-3-1-contract-first-problem-details-cursor-pagin.md) · [OpenAPI spec](../../specs/openapi/eduworld-api.v1.yaml) · [AsyncAPI realtime](../../specs/asyncapi) · [BACKEND_DESIGN](BACKEND_DESIGN.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md)

**Mục đích & phạm vi.** Quy ước REST, catalog mã lỗi, catalog endpoint public/internal và định tuyến APISIX. Quy tắc nghiệp vụ của endpoint nằm ở tài liệu domain; WebSocket ở AsyncAPI + REALTIME_ARCHITECTURE.

## 1. Nguyên tắc

1. **Contract-first** [Proposed — ADR-0010, ADR-0036]: sửa spec trước, review, rồi sinh code (`oapi-codegen`) và client (Godot API layer, TypeScript cho `web/`, typed client giữa service).
   - **API public:** nguồn sự thật là spec gộp `specs/openapi/eduworld-api.v1.yaml`; mỗi operation có `x-service`. Spec public theo service `specs/openapi/services/<service>.v1.yaml` được **sinh tự động** bằng `scripts/openapi/split-public.mjs` (không sửa tay; CI chạy `--check`).
   - **API internal:** mỗi service sở hữu `specs/openapi/services/<service>.internal.v1.yaml` (sửa tay). Interface platform ops chuẩn định nghĩa một lần trong `specs/openapi/services/common/internal-common.v1.yaml` và được đồng bộ vào từng file bằng `scripts/openapi/sync-platform-ops.mjs` (§12).
2. REST cho nghiệp vụ; WebSocket (`/realtime/v1`, [AsyncAPI](../../specs/asyncapi)) cho chat, notification, presence, party/lobby, matchmaking events, battle room [Confirmed — Master Prompt §12.3, §14].
3. Không expose database model; DTO API độc lập với bảng [Confirmed — §12.3].
4. Mọi endpoint yêu cầu authN; mọi endpoint kiểm tra authZ + tenant trong Go [Confirmed — §11]. Ngoại lệ có chủ đích: `/livez`, `/readyz` (chỉ mạng private) và `GET /game/client-config` (public, chỉ trả cấu hình không nhạy cảm, có rate limit) [Proposed].
5. Client không bao giờ gửi điểm, kết quả, XP, rating; server tính (ADR-0016) [Confirmed — §4.3, §5.6].
6. **Phạm vi tài liệu:** quy ước API + catalog endpoint (method, path, quyền, idempotency, phase) + internal API. Quy tắc nghiệp vụ của từng endpoint nằm ở tài liệu domain được link trong mỗi mục §9; không lặp lại ở đây.

## 2. Quy ước

| Hạng mục | Quy ước |
|---|---|
| Base path | `/api/v1/<domain>/...` — domain theo route map Master Prompt §11 |
| Versioning | Path (`/api/v1`). Thêm field tùy chọn/endpoint mới = không breaking. Breaking → `/api/v2` chạy song song ≥ 1 học kỳ; header `Deprecation` + `Sunset` trên v1 |
| Resource naming | Danh từ số nhiều, `kebab-case` trong path (`/realtime-tickets`), ID trong path là UUIDv7 |
| Action không CRUD | Hậu tố `:verb` theo Google AIP (`POST /learning/activity-attempts/{id}:submit`, `POST /multiplayer/parties/{id}:start`) |
| JSON | `snake_case`, UTF-8; `null` khác với vắng mặt (PATCH dùng JSON Merge Patch RFC 7396) |
| Thời gian | ISO-8601 UTC (`2026-11-15T08:30:00Z`); thời lượng dạng số nguyên `_ms` hoặc `_seconds` |
| Enum | chuỗi `snake_case`; client phải chấp nhận giá trị lạ (forward-compatible) |
| Ngôn ngữ (ADR-0056) | Endpoint học sinh (`/auth/game/*`, `/auth/session`, `/auth/me`, `/game/*`, `/learning/*`, `/exams/*`, `/multiplayer/*`, `/competition/*`, `/leaderboards*`, `/chat/*`, `/social/*`, `/notifications*`) nhận header `Accept-Language: vi-VN` (mặc định) \| `en-US`; vắng mặt → `preferred_language` của học sinh, rồi `default_locale` của tenant. Response chọn ngôn ngữ trả header `Content-Language`. Ảnh hưởng `title`/`detail` của Problem và văn bản hiển thị đã chọn, **không** ảnh hưởng `code`. Văn bản game trong JSON là `LocalizedName`/`LocalizedText` (`{vi, en}` đều bắt buộc); văn bản nội dung học tập là `LearningName`/`LearningText` (ngôn ngữ gốc `content_language` bắt buộc, bản dịch tùy chọn); khóa nội dung `vi`/`en`, locale đầy đủ `vi-VN`/`en-US` chỉ dùng ở HTTP/client. Dữ liệu cache dài (manifest, snapshot) luôn trả đủ hai ngôn ngữ. Publish nội dung game/sự kiện platform thiếu bản `en` → `422 TRANSLATION_INCOMPLETE` |
| Request ID | APISIX sinh `X-Request-Id`; response trả lại cùng `traceparent` |
| Thiết bị (game) | Godot gửi `X-Device-Id` (UUID sinh lần đầu cài, lưu `user://`) trên mọi request game; bắt buộc ở `POST /game/sessions`. Server so với `device_id` của game session active để áp quy tắc một thiết bị (ADR-0031). Không phải định danh phần cứng, không dùng làm credential |
| Kích thước | Body tối đa 256 KB (mặc định), 64 KB cho chat/answer, upload media qua presigned URL (không đi qua API) |
| Header tenant (ADR-0065) | **`X-Tenant-Id` (UUID) bắt buộc** trên mọi request REST tới dữ liệu tenant-scoped của game và portal. Client lấy giá trị từ `POST /auth/session` (game) hoặc từ tenant đã chọn sau đăng nhập (portal, ADR-0067). APISIX: thiếu → `400 TENANT_HEADER_REQUIRED`, sai định dạng → `400 BAD_REQUEST`. Backend: header phải khớp tenant của phiên (claim `organization` + membership DB) → nếu không `403 TENANT_MISMATCH` (audit + security event); `Principal.tenant_id` lấy từ kết quả kiểm tra, không lấy thẳng header. **Ngoại lệ không cần header:** `POST /auth/session`, `GET /me/tenants`, `GET /game/client-config`, `/livez`, `/readyz`. System Admin: ghi gửi header = tenant đích; đọc liên tenant dùng `tenant_ids` và **không** gửi header. WebSocket: tenant cố định khi cấp ticket. Đổi tenant = token mới + header mới |
| Tenant (đọc) | Người dùng tenant: mọi endpoint tự khóa vào `tenant_id` của phiên (đã kiểm tra khớp `X-Tenant-Id`, ADR-0005, ADR-0026, ADR-0065); tham số `tenant_ids` gửi lên bị **bỏ qua**. **Chỉ System Admin** (`platform.cross_tenant.read`): list endpoint dữ liệu tenant nhận `tenant_ids` (query, lặp lại: `?tenant_ids=a&tenant_ids=b`; vắng mặt = tất cả tenant), response mỗi item có `tenant_id`, audit `cross_tenant.viewed` (ADR-0027 §6) |
| Tenant (ghi) | Người dùng tenant: ghi vào tenant phiên; gửi `tenant_id` khác → `403 TENANT_MISMATCH`. System Admin: header `X-Tenant-Id` = tenant đích (thiếu → `400 TENANT_HEADER_REQUIRED`), body `tenant_id` nếu có phải trùng header (khác → `422`) hoặc `owner_scope = platform` cho dữ liệu dùng chung; không có ghi "mọi tenant" ngầm định |
| Dữ liệu platform được gán | Tenant đọc được dữ liệu platform (`tenant_id = null`) chỉ khi có assignment active (ADR-0029); tham chiếu mục chưa gán → `403 RESOURCE_NOT_ASSIGNED`; dữ liệu được gán là chỉ đọc với tenant |
| Service sở hữu | Mỗi operation thuộc đúng một service (`x-service` trong spec gộp), xem §2.1. Ngoại lệ có chủ đích: `/game-admin/{config_type}` định tuyến theo `config_type` (`x-service-routing`) |
| Độ dài chuỗi | Mọi field `string` có `maxLength` (ADR-0041), trừ `enum`/`const`/`format` uuid·date·date-time·time·duration·binary·byte. Vượt giới hạn → `422 VALIDATION_FAILED` với `errors[].code = MAX_LENGTH` (§13) |
| Xóa | `DELETE` = **soft-delete** (ADR-0028): trả `204`, đặt `deleted_at`/`deleted_by`, body tùy chọn `{ "reason": "..." }`. Mọi `GET` mặc định loại bản ghi đã xóa; `GET` theo ID của bản ghi đã xóa → `404`. Không có tham số `include_deleted` ở API thường; bản ghi đã xóa chỉ đọc/khôi phục qua API Thùng rác (§9.16). Field `deleted_at` chỉ xuất hiện trong schema của Thùng rác. Bảng append-only (attempt answer, learning event, ledger, audit) không có `DELETE` |

### 2.1. Service sở hữu endpoint (ADR-0036)

APISIX định tuyến theo prefix; service phục vụ phải tự kiểm tra authN/authZ/tenant (ADR-0009). Bảng này đồng bộ với `x-service` trong spec gộp (Spectral rule `operation-must-declare-service`).

| Prefix public `/api/v1/...` | Service | Spec sinh ra |
|---|---|---|
| `auth/*`, `me/*`, `catalog/*`, `students/me`, `students/{id}`, `schools/*`, `tenant/*` (trừ `tenant/chat/*`, `tenant/students/*/login-events`, `tenant/analytics/*`, `tenant/reports*`), `admin/*` (trừ `admin/chat/*`, `admin/login-events`, `admin/economy/*`, `admin/analytics/*`, `admin/reports*`), `sis/*` | `identity-service` | `identity.v1.yaml` |
| `learning/*`, `exams/*`, `students/{id}/progress`, `tenant/analytics/learning/*`, `tenant/reports*`, `admin/analytics/learning/*`, `admin/reports*`, `cms/learning/*` (gồm `cms/learning/translations*`), `cms/media*`, `lms/*` (trừ `lms/event-templates`, `lms/school-events*`) | `learning-service` | `learning.v1.yaml` |
| `game/*`, `game-media*`, `game-admin/preview/*`, `game-admin/translations*`, `game-admin/events/*`, `tenant/students/{id}/login-events`, `admin/login-events`, `admin/economy/*`, `admin/analytics/game/*`, `tenant/analytics/game/*`, **route ngoại lệ trong `/lms/*`:** `lms/event-templates`, `lms/school-events*` (sự kiện trường — [EVENT_SEASON_SYSTEM §10](../01-game-design/EVENT_SEASON_SYSTEM.md#10-sự-kiện-của-trường-tenant)), `game-admin/{config_type}` với `quests`, `minigame-configs`, `npc-dialogues`, `world-areas`, `story-chapters`, `events`, `event-templates`, `season-tracks`, `items`, `reward-rules`, `levels`, `progression-curves`, `challenges`, `bosses`, `achievements`, `collections`, `shop-items`, `currencies` | `game-service` | `game.v1.yaml` |
| `multiplayer/*`, `competition/*`, `leaderboards/*`, `game-admin/{config_type}` với `matchmaking-configs`, `ranking-rules`, `seasons`, `tournament-templates`, `challenge-templates`, `leaderboard-definitions` | `competition-service` | `competition.v1.yaml` |
| `chat/*`, `social/*`, `notifications/*`, `admin/chat/*`, `tenant/chat/*`, `game-admin/{config_type}` với `sticker-packs`, `gifs` | `chat-service` | `chat.v1.yaml` |
| `/realtime/v1` (WebSocket) | `realtime-gateway` | AsyncAPI `specs/asyncapi/` |

Cài đặt platform/tenant (`/admin/settings/*`, `/tenant/settings`) luôn thuộc `identity-service` (kho cài đặt tập trung); service khác nhận giá trị qua event `platform_settings.changed`/`tenant_settings.changed` hoặc `GET /internal/v1/tenants/{id}`. Thùng rác, gán và clone dữ liệu (`/admin/trash`, `/tenant/trash`, `/admin/tenant-assignments`, `/admin/tenant-clones`) do `identity-service` điều phối, gọi interface platform ops của từng service (§12.2).

## 3. Response envelope

- Đối tượng đơn: trả trực tiếp object.
- Danh sách:

```json
{
  "data": [ { "...": "..." } ],
  "page": { "next_cursor": "eyJpZCI6IjAxOT...", "has_more": true, "limit": 20 }
}
```

## 4. Error model — Problem Details

`Content-Type: application/problem+json` (RFC 9457):

```json
{
  "type": "https://docs.eduworld.vn/errors/exam-already-submitted",
  "title": "Bài thi đã được nộp",
  "status": 409,
  "code": "EXAM_ALREADY_SUBMITTED",
  "detail": "Lượt làm bài này đã được nộp lúc 08:41.",
  "instance": "/api/v1/exams/attempts/0193...:submit",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "errors": [ { "field": "response.value", "code": "REQUIRED", "message": "Bắt buộc" } ]
}
```

`code` là hợp đồng ổn định (client xử lý theo `code`, không theo `title`/`detail`).

### 4.1. Catalog mã lỗi

Bảng này và enum `ProblemCode` trong [OpenAPI](../../specs/openapi/eduworld-api.v1.yaml) là **một danh sách duy nhất** (số mã không ghi cứng ở đây; CI so sánh bảng và enum). Tiền tố theo domain: `ACCOUNT_*`/`TENANT_*` identity, `ATTEMPT_*`/`ANSWER_*`/`ASSIGNMENT_*` exercise, `EXAM_*`, `PARTY_*`, `MM_*` matchmaking, `CHAT_*`. Tài liệu domain phải dùng đúng mã ở đây.

**Chung**

| HTTP | `code` | Khi nào |
|---|---|---|
| 400 | `BAD_REQUEST` | JSON không parse được; header `X-Tenant-Id` sai định dạng UUID |
| 400 | `TENANT_HEADER_REQUIRED` | Thiếu header `X-Tenant-Id` ở route tenant-scoped (APISIX trả trước khi tới service; middleware tenant của service kiểm tra lại) — ADR-0065 |
| 401 | `UNAUTHENTICATED` | Thiếu/sai token |
| 401 | `TOKEN_EXPIRED` | Access token hết hạn — client refresh rồi gọi lại |
| 403 | `PERMISSION_DENIED` | Có tài nguyên trong scope nhưng thiếu permission (ADR-0027), hoặc gọi route platform-only (`/game-admin/*`, `/admin/*`) bằng token có `azp` không hợp lệ hoặc không có role platform |
| 403 | `MFA_REQUIRED` | Gọi route platform-only bằng token `eduworld-portal` chưa xác thực MFA (`acr` thấp hơn mức yêu cầu); client yêu cầu đăng nhập lại với MFA (step-up) — ADR-0027 mục 3 |
| 403 | `CALLER_NOT_ALLOWED` | **Internal API** (§12): service gọi không nằm trong `x-allowed-callers` (`azp`), sai `aud`/scope của service token. Không xuất hiện ở API public |
| 403 | `FORBIDDEN_SCOPE` | Thao tác vượt scope được cấp (vd. giáo viên tạo attempt cho lớp không phân công) |
| 403 | `POLICY_DENIED` | Policy chung chặn — kèm `policy_reason` (policy chuyên biệt dùng mã domain bên dưới) |
| 404 | `NOT_FOUND` | Không tồn tại **hoặc ngoài tenant scope** |
| 409 | `CONFLICT` | Xung đột chung |
| 409 | `IDEMPOTENCY_KEY_REUSED` | Cùng key, khác payload |
| 409 | `IDEMPOTENCY_IN_PROGRESS` | Request cùng key đang xử lý |
| 412 | `VERSION_MISMATCH` | `If-Match` không khớp ETag |
| 422 | `VALIDATION_FAILED` | Lỗi validate field (`errors[]`) |
| 422 | `INVALID_STATE_TRANSITION` | Chuyển trạng thái workflow không hợp lệ |
| 428 | `IDEMPOTENCY_KEY_REQUIRED` | Endpoint bắt buộc key nhưng thiếu |
| 428 | `PRECONDITION_REQUIRED` | Thiếu `If-Match` khi cập nhật CMS |
| 429 | `RATE_LIMITED` | Vượt rate limit (APISIX hoặc nghiệp vụ) |
| 500 | `INTERNAL` | Lỗi không mong đợi (không lộ chi tiết) |
| 503 | `UNAVAILABLE` / `MAINTENANCE` | Phụ thuộc hỏng (kể cả service nội bộ timeout hoặc circuit breaker mở — **không fail-open**, §12.3) / bảo trì |

**Identity & tài khoản** ([ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md), [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md))

| HTTP | `code` | Khi nào |
|---|---|---|
| 401 | `INVALID_CREDENTIALS` | `POST /auth/game/login`: Keycloak từ chối username/mật khẩu — tài khoản không tồn tại, sai mật khẩu, **hoặc** đang bị khóa tạm do brute-force (Keycloak khóa sau 5 lần sai, 1–15 phút). Bước xác thực SSO chạy **trước** mọi kiểm tra tenant (ADR-0069 mục 3). Một thông điệp chung "Tên đăng nhập hoặc mật khẩu chưa đúng", không tiết lộ tài khoản có tồn tại hay đang bị khóa; game thêm gợi ý "Nếu nhập sai nhiều lần, hãy đợi vài phút" [Proposed] — ADR-0068 |
| 401 | `LOGIN_TICKET_INVALID` | `POST /auth/game/tenant:select`: `login_ticket` hết hạn (2 phút), đã dùng, không tồn tại hoặc khác `device_id` — game quay lại form đăng nhập (ADR-0068) |
| 426 | `CLIENT_VERSION_UNSUPPORTED` | `client_version` gửi ở `/auth/game/login` thấp hơn `min_client_version` của nền tảng; body có `min_client_version`, `store_url` — game hiển thị màn cập nhật (ADR-0068) |
| 429 | `LOGIN_QUEUE_BUSY` | Hàng chờ đăng nhập toàn cục đầy (ADR-0045) tại `/auth/game/login`; có `Retry-After`, game hiển thị "Đang vào lớp…" và tự thử lại. Khác `RATE_LIMITED` (vượt giới hạn theo thiết bị/username) |
| 403 | `ACCOUNT_NOT_PROVISIONED` | Xác thực đúng nhưng tài khoản không có membership tenant SIS nào và không thuộc tenant chung (game), hoặc không khớp bản ghi SIS; **không JIT** (ADR-0063). Portal: tài khoản không có UserRole portal dùng `PORTAL_ACCESS_STAFF_ONLY` |
| 403 | `ACCOUNT_LOCKED` / `ACCOUNT_DEACTIVATED` | Trạng thái `User` không active |
| 403 | `NO_ACTIVE_ENROLLMENT` | Học sinh không có enrollment active |
| 403 | `TENANT_ACCESS_DISABLED` | Tenant (trường độc lập / đơn vị) bị tắt truy cập EduWorld — thay `SCHOOL_ACCESS_DISABLED` (ADR-0026) |
| 403 | `RESOURCE_NOT_ASSIGNED` | Tham chiếu dữ liệu platform chưa được gán cho tenant (ADR-0029) |
| 403 | `ACCESS_OUTSIDE_ALLOWED_HOURS` | Ngoài khung giờ trường cho phép |
| 403 | `GAME_ACCESS_STUDENT_ONLY` | Token game (client `eduworld-game-backend`, ADR-0068) (hoặc API người chơi `/game/*`, `/multiplayer/*`, `/competition/*` phía người chơi, realtime ticket) thuộc User không phải `student` — giáo viên, vận hành tenant, System Admin không vào game (ADR-0031); với `POST /auth/game/login`: tài khoản không có realm role `student` (ADR-0068) |
| 403 | `PORTAL_ACCESS_STAFF_ONLY` | Route portal (`/cms/*`, `/lms/*`, `/tenant/*`, `/schools/*`, `/game-admin/*`, `/admin/*`) gọi bằng token không có `azp = eduworld-portal`, hoặc User là `student`, hoặc người dùng không còn UserRole portal hiệu lực ở bất kỳ tenant nào và không có role platform — ADR-0066, ADR-0067; ghi audit |
| 409 | `USERNAME_TAKEN` | Tạo/sửa tài khoản (thủ công hoặc import) với username **đã thuộc tài khoản khác** — trong tenant hoặc trên SSO (Keycloak, username duy nhất toàn realm; so sánh sau chuẩn hóa trim/NFC/chữ thường), kể cả tài khoản đã soft-delete chưa ẩn danh hóa. Nếu là **cùng một người** đã có tài khoản thì không tạo tài khoản mới mà thêm membership/UserRole cho tài khoản sẵn có (ACCOUNT_PROVISIONING); SIS/import dùng `reconciliation_issue` `duplicate_identifier` — ADR-0069 |
| 409 | `EMAIL_TAKEN` | Như `USERNAME_TAKEN` với email (không phân biệt hoa thường; Keycloak tắt "Duplicate emails") — ADR-0069 |
| 422 | `RETENTION_BELOW_MINIMUM` | Cấu hình `gameplay_chat_retention_days` < 3 ngày (ADR-0033) |
| 409 | `ACTIVE_SESSION_ON_OTHER_DEVICE` | `POST /game/sessions` khi học sinh đang có game session active trên thiết bị khác (`X-Device-Id` khác). Body có `other_device {platform, device_label, last_seen_at}`; client hỏi xác nhận rồi gọi lại với `takeover: true` (ADR-0031) |
| 409 | `IDENTITY_CONFLICT` | `(iss, sub)` khớp nhiều ứng viên — cần reconciliation |
| 409 | `SIS_SCHOOL_ALREADY_CONNECTED` | Gán trường cho SIS connection khi trường đã có connection khác đang hoạt động (ADR-0046) |
| 403 | `TENANT_NOT_SELECTED` | Tài khoản (đã xác thực SSO) có **nhiều** tenant còn hiệu lực. **Game** (`POST /auth/game/login`): body có `choices[]` (các tenant của học sinh) + `login_ticket` (dùng một lần, 2 phút, gắn `device_id`); game hiển thị danh sách rồi gọi `POST /auth/game/tenant:select` (không nhập lại mật khẩu). **Portal:** nhân sự chọn tenant sau đăng nhập qua `GET /me/tenants` và bộ chọn trên topbar. Không chọn tenant trước đăng nhập (ADR-0063, ADR-0067, ADR-0069). Người chỉ thuộc tenant chung vào thẳng tenant chung `common` (ADR-0026) |
| 403 | `TENANT_MISMATCH` | Tenant trong token (claim `organization`) không nằm trong danh sách tenant mà user là thành viên theo DB (vd. học sinh tenant A chọn tenant B), hoặc người dùng tenant ghi dữ liệu với `tenant_id` khác tenant phiên, hoặc header `X-Tenant-Id` khác tenant của phiên (token + membership) — ADR-0026, ADR-0065; ghi audit + security event |

**Content & exercise** ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), [QUESTION_BANK](../02-learning/QUESTION_BANK.md))

| HTTP | `code` | Khi nào |
|---|---|---|
| 409 | `CONTENT_NOT_EDITABLE` | Sửa version đã `published` (ADR-0015) |
| 409 | `CONTENT_VERSION_CONFLICT` | Publish/sửa Draft khi version đã bị người khác thay đổi (`If-Match` lệch, ADR-0043) |
| 409 | `CONTENT_NOT_PUBLISHED` | Thao tác cần nội dung đang publish (giao bài, bắt đầu lượt, gán tenant) nhưng item chưa publish/đã gỡ |
| 422 | `ROLLBACK_TARGET_INVALID` | Rollback tới version không phải `published` của chính item, version `archived`, hoặc đúng version đang là con trỏ |
| 415 | `MEDIA_TYPE_NOT_ALLOWED` | Loại file không được phép hoặc magic bytes không khớp `mime_type` (ADR-0044) |
| 413 | `MEDIA_TOO_LARGE` | Vượt giới hạn dung lượng theo loại media |
| 422 | `MEDIA_NOT_READY` | Submit review/publish khi media tham chiếu chưa `ready` |
| 422 | `QUESTION_POOL_INSUFFICIENT` | Selection rule không đủ câu hỏi |
| 404 | `ATTEMPT_NOT_FOUND` | Attempt không tồn tại/ngoài scope (dùng khi client cần phân biệt với `NOT_FOUND` chung) |
| 409 | `ATTEMPT_NOT_IN_PROGRESS` | Gửi trả lời cho attempt không còn `in_progress` |
| 409 | `ATTEMPT_ALREADY_SUBMITTED` | Nộp trùng attempt |
| 422 | `ATTEMPT_DEADLINE_PASSED` | Nộp sau `deadline_at` + grace |
| 410 | `ATTEMPT_EXPIRED` | Attempt hết hạn resume |
| 409 | `ANSWER_LIMIT_REACHED` | Vượt số lần trả lời cho một item |
| 422 | `INVALID_RESPONSE` | `response` không đúng cấu trúc của question type |
| 422 | `ITEM_REF_INVALID` | Tham chiếu item trong câu trả lời (`position`, `question_version_id`, `cue_id`, `branch_point_id`, `option_id`) không thuộc attempt (ADR-0059) |
| 422 | `ACTIVITY_TYPE_NOT_SUPPORTED` | Không có plugin cho `activity_type` hoặc client quá cũ (ADR-0048, [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)) |
| 422 | `ACTIVITY_CONFIG_INVALID` | Cấu hình activity không qua schema của loại khi lưu/publish |
| 409 | `ATTEMPT_PENDING_REVIEW` | Thao tác không hợp lệ khi attempt đang chờ giáo viên chấm |
| 409 | `EVALUATION_NOT_EDITABLE` | Sửa evaluation đã `superseded` |
| 422 | `MATH_RENDER_FAILED` | Lưu nội dung có công thức thiếu SVG, SVG không hợp lệ hoặc LaTeX ngoài allowlist (ADR-0055) |
| 422 | `VIDEO_NOT_READY` | Video/audio của activity chưa xử lý xong |
| 422 | `VIDEO_SEEK_NOT_ALLOWED` | Báo tiến độ vượt phần cho phép tua |
| 422 | `CUE_ANSWER_REQUIRED` | Submit khi còn câu hỏi chèn bắt buộc |
| 422 | `RESPONSE_TOO_LONG` / `RESPONSE_TOO_SHORT` | Bài viết vượt/không đủ giới hạn từ hoặc ký tự |
| 404 | `REVIEW_NOT_ASSIGNED` | Giáo viên mở/chấm bài không được phân công (trả 404 như mọi tài nguyên ngoài scope) |
| 422 | `RUBRIC_INCOMPLETE` | Chấm chưa chọn mức cho mọi tiêu chí bắt buộc |
| 422 | `SUBMISSION_FILE_REJECTED` | File sản phẩm/ghi âm không qua quét virus hoặc sai định dạng/độ dài |
| 422 | `SIMULATION_LOG_INVALID` | Action log mô phỏng không hợp lệ (ngoài phạm vi, quá dài) |
| 403 | `SPEAKING_NOT_ENABLED` | Tenant chưa bật hoạt động ghi âm (dữ liệu giọng nói trẻ em) |
| 403 | `ASSIGNMENT_NOT_OPEN` | Ngoài `open_at`–`due_at` |
| 409 | `ASSIGNMENT_MAX_ATTEMPTS` | Hết lượt làm bài được giao |

**Exam** ([EXAM_ENGINE](../02-learning/EXAM_ENGINE.md))

| HTTP | `code` | Khi nào |
|---|---|---|
| 403 | `EXAM_NOT_ASSIGNED` | Học sinh không thuộc đối tượng của exam session |
| 403 | `EXAM_NOT_OPEN` / `EXAM_WINDOW_CLOSED` | Chưa mở / đã đóng cửa sổ thi |
| 409 | `EXAM_MAX_ATTEMPTS` | Hết lượt |
| 409 | `EXAM_ACTIVE_ON_OTHER_DEVICE` | Attempt đang active trên thiết bị khác |
| 422 | `EXAM_TIME_OVER` | Hết giờ (server time) |
| 409 | `EXAM_ALREADY_SUBMITTED` | Nộp trùng |
| 403 | `EXAM_RESULTS_NOT_RELEASED` | Kết quả chưa công bố |

**Game** ([QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md), [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md), [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md), [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md), [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md))

| HTTP | `code` | Khi nào |
|---|---|---|
| 409 | `STEP_PRECONDITION_FAILED` | Báo bước quest khi điều kiện chưa thỏa |
| 422 | `ITEM_NOT_OWNED` | Avatar dùng item không có trong inventory |
| 422 | `EQUIP_SLOT_INVALID` | Vật phẩm không hợp với slot trang bị |
| 409 | `QUEST_ALREADY_COMPLETED` | Nhận lại quest `once` đã hoàn thành |
| 409 | `QUEST_COOLDOWN` | Nhận quest `repeatable` khi đang cooldown (kèm `retry_after`) |
| 403 | `AREA_LOCKED` | Vào area chưa mở khóa (kèm điều kiện còn thiếu) |
| 409 | `DISCOVERY_ALREADY_FOUND` | Báo lại điểm khám phá đã tìm |
| 403 | `CHALLENGE_NOT_ELIGIBLE` | Sai khối, chưa mở khóa, ngoài thời gian |
| 409 | `CHALLENGE_ATTEMPT_LIMIT` | Hết lượt thử trong ngày/kỳ hoặc đang cooldown |
| 409 | `BOSS_ENCOUNTER_FULL` | Encounter đủ người hoặc đã bắt đầu |
| 409 | `BOSS_PHASE_CLOSED` | Nộp cho phase đã kết thúc |
| 409 | `INSUFFICIENT_BALANCE` | Không đủ Xu |
| 409 | `PURCHASE_LIMIT_REACHED` | Vượt giới hạn mua |
| 409 | `SHOP_ITEM_UNAVAILABLE` | Hết thời gian bán, không gán cho tenant, sai khối |
| 409 | `ITEM_ALREADY_OWNED` | Mua vật phẩm không xếp chồng đã có |
| 403 | `TRADE_DISABLED` | Yêu cầu trao đổi vật phẩm (tắt trong Release 1.0) |
| 404 | `EVENT_NOT_AVAILABLE` | Sự kiện không thuộc phạm vi học sinh hoặc không ở trạng thái `live`/`ended` ([EVENT_SEASON_SYSTEM §17](../01-game-design/EVENT_SEASON_SYSTEM.md#17-lỗi)) |
| 409 | `EVENT_ENDED` | Bắt đầu hoạt động sự kiện sau `end_at` hoặc nhận thưởng sau hạn nhận |
| 409 | `EVENT_MILESTONE_NOT_REACHED` | Nhận thưởng mốc điểm sự kiện chưa đạt |
| 409 | `SEASON_TIER_NOT_REACHED` | Nhận thưởng cấp season track chưa đạt |
| 422 | `EVENT_TEMPLATE_PARAM_NOT_ALLOWED` | Tenant tạo/sửa sự kiện trường với tham số ngoài `allowed_params` của template |

**Song ngữ** ([ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md))

| HTTP | `code` | Khi nào |
|---|---|---|
| 422 | `TRANSLATION_INCOMPLETE` | Publish nội dung game/sự kiện platform thiếu bản `en` (hoặc `vi`) ở trường bắt buộc song ngữ; `errors[]` liệt kê JSON pointer còn thiếu |


**Party, room, competition** ([PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md), [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md))

| HTTP | `code` | Khi nào |
|---|---|---|
| 409 | `PARTY_ALREADY_IN_PARTY` | Học sinh đã có active party (một active party/học sinh — Addendum §6) |
| 409 | `PARTY_FULL` | Đạt `max_members` |
| 404 | `PARTY_NOT_FOUND` | Party không tồn tại/đã giải tán/ngoài scope |
| 403 | `PARTY_NOT_LEADER` | Thao tác chỉ dành cho leader |
| 403 | `PARTY_INVITE_NOT_ALLOWED` | `invitation_policy` không cho phép người mời |
| 429 | `PARTY_INVITE_RATE_LIMITED` | Vượt giới hạn lời mời |
| 410 | `PARTY_INVITATION_EXPIRED` | Lời mời hết hạn |
| 403 | `PARTY_POLICY_VIOLATION` | Vi phạm rule school/class/grade/block |
| 409 | `PARTY_MEMBER_NOT_ONLINE` | Chuyển leader cho member offline |
| 422 | `PARTY_ACTIVITY_NOT_ELIGIBLE` | Có member không đủ điều kiện activity |
| 403 | `ROOM_NOT_MEMBER` | Truy cập room không phải thành viên |
| 409 | `MM_TICKET_ALREADY_ACTIVE` | Đã có matchmaking ticket |
| 409 | `MATCH_RESULT_ALREADY_RECORDED` | Ghi kết quả trận lần 2 |
| 410 | `DUEL_EXPIRED` | Lời thách đấu bạn (friend duel) đã hết hạn 60 s hoặc bị hủy |

**Social** ([SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md))

| HTTP | `code` | Khi nào |
|---|---|---|
| 403 | `SOCIAL_SCOPE_VIOLATION` | Kết bạn, mời club, thách đấu hoặc tham gia club ngoài phạm vi cho phép (khác trường khi tenant chưa bật, khác tenant — ADR-0054) hoặc đã block nhau. Dùng chung cho friend/club/duel (không có `CLUB_SCOPE_VIOLATION` riêng) |
| 409 | `FRIEND_LIMIT_REACHED` | Vượt ≤ 100 bạn hoặc ≤ 10 lời mời đang chờ (SOCIAL_SYSTEM §4.1) |
| 404 | `FRIEND_CODE_INVALID` | Mã bạn bè sai hoặc đã hết hạn 10 phút |
| 409 | `CLUB_FULL` | Club đã đủ `max_members` |
| 409 | `CLUB_LIMIT_REACHED` | Học sinh đã ở tối đa 5 club |
| 409 | `CLUB_NOT_APPROVED` | Thao tác trên club `pending_approval` (mời, tạo mục tiêu/sự kiện) trước khi giáo viên duyệt |

**Chat** ([CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [MODERATION](../03-social-multiplayer/MODERATION.md))

| HTTP | `code` | Khi nào |
|---|---|---|
| 403 | `CHAT_NOT_ALLOWED` | Policy không cho phép gửi ở channel này |
| 403 | `CHAT_KIND_NOT_ALLOWED` | Loại tin (text/quick/emoji) không được phép trong channel theo policy trường/lớp (không theo grade band) |
| 451 | `CHAT_CONTENT_BLOCKED` | Bị bộ lọc chặn (kèm `category_code` của danh mục từ nhạy cảm hoặc `pii`/`spam`/`bullying`) — **không** echo nội dung, không trả từ đã khớp (ADR-0030) |
| 202 | `CHAT_CONTENT_HELD` | Tin bị giữ chờ kiểm duyệt (`hold_for_review`/`block_and_alert`) — dùng làm `code` trong body phản hồi, không phải lỗi |
| 422 | `MODERATION_SWITCH_REASON_REQUIRED` | Bật/tắt công tắc kiểm duyệt không kèm `reason` hợp lệ (ADR-0030) |
| 403 | `TENANT_TOGGLE_NOT_ALLOWED` | Tenant bật/tắt category có `allow_tenant_toggle = false` |
| 422 | `SENSITIVE_TERM_INVALID` | Term không hợp lệ (pattern RE2 lỗi/quá dài, trùng term hiệu lực, tenant sửa term platform) |
| 422 | `SENSITIVE_CATALOG_REGRESSION_FAILED` | Publish danh mục bị chặn vì test hồi quy không đạt ngưỡng |
| 403 | `CHAT_MUTED` | Người gửi đang bị mute (sanction) |
| 429 | `CHAT_RATE_LIMITED` | Vượt rate của policy |
| 422 | `CHAT_POLICY_ALWAYS_ON` | Cố tắt/giới hạn channel gameplay (`match`, `team` theo trận, `party`) ở cấp tenant/school/class — các channel này luôn bật (ADR-0020 §6) |
| 409 | `CHAT_CHANNEL_CLOSED` | Gửi tin vào channel gameplay đã đóng (trận kết thúc / party giải tán) |
| 422 | `CHAT_POLICY_RELAX_NOT_ALLOWED` | Cấp dưới cố nới policy cấp trên |
| 409 | `CHAT_EDIT_WINDOW_EXPIRED` | Sửa tin quá 15 phút (hoặc không phải người gửi / tin không phải text) (ADR-0034) |
| 409 | `CHAT_RECALL_WINDOW_EXPIRED` | Thu hồi tin quá 24 giờ (ADR-0034) |
| 403 | `CHAT_FORWARD_NOT_ALLOWED` | Forward khác tenant, sang channel không có quyền gửi, ra khỏi world/area, hoặc tin đã thu hồi/bị che/bị gỡ/đang held |
| 409 | `CHAT_PIN_LIMIT_REACHED` | Channel đã có 3 tin ghim |
| 422 | `CHAT_MENTION_LIMIT` | Hơn 5 mention trong một tin, hoặc mention người không phải thành viên/đã block |
| 503 | `CHAT_TEMPORARILY_UNAVAILABLE` | NATS JetStream hoặc cấp `seq` không sẵn sàng; client thử lại (ADR-0035) — có `Retry-After` |
| 403 | `CHAT_FEATURE_DISABLED` | Tính năng (pin/typing/receipts/forward/search…) không bật cho loại channel này (§7.1 CHAT_SYSTEM) |
| 422 | `CHAT_ASSET_NOT_AVAILABLE` | `sticker_id`/`gif_id` không tồn tại, chưa publish, chưa gán cho tenant, hoặc sticker pack chưa mở khóa |

**Dashboard & báo cáo** ([DASHBOARD_DATA](DASHBOARD_DATA.md), ADR-0062)

| HTTP | `code` | Khi nào |
|---|---|---|
| 422 | `DASHBOARD_RANGE_TOO_LARGE` | Thiếu `from`/`to`, `to < from`, khoảng > 730 ngày (`dashboard_max_range_days`), hoặc yêu cầu `grain` mịn hơn quy tắc (DASHBOARD_DATA §8) |
| 429 | `DASHBOARD_BUSY` | Người dùng đã có 4 truy vấn dashboard đang chạy (giới hạn đồng thời theo người dùng) hoặc pool replica bão hòa; có `Retry-After` |
| 504 | `DASHBOARD_QUERY_TIMEOUT` | Truy vấn bị hủy ở database sau `statement_timeout` 5 giây; client gợi ý thu hẹp khoảng thời gian |
| 429 | `EXPORT_LIMIT_REACHED` | Vượt số export đồng thời/ngày của người dùng hoặc tenant (DASHBOARD_DATA §10); có `Retry-After` |

**Sub-code trong `errors[]`** (không phải `code` cấp Problem, không có trong `ProblemCode`): `PERMISSION_NOT_ALLOWED_FOR_SCOPE` — trong `422 VALIDATION_FAILED` khi lưu role `tenant` chứa permission `platform.*`/`game.*`/`competition.config.*` (ADR-0066); `REQUIRED`, `TOO_LONG`, `PATTERN_MISMATCH`… — lỗi validate field.

WebSocket dùng enum `ErrorCode` riêng trong [AsyncAPI](../../specs/asyncapi/eduworld-realtime.v1.yaml) (trả qua `system.command.failed`). Mã nào có ở cả hai kênh thì **trùng tên** với `ProblemCode` (vd. `CHAT_CONTENT_BLOCKED`, `PARTY_NOT_LEADER`, `ROOM_NOT_MEMBER`). Các mã chỉ có trên WS: `UNKNOWN_TYPE`, `PARTY_NOT_READY`, `ROOM_FULL`, `ANSWER_LATE`, `ANSWER_ALREADY_SUBMITTED`, `MATCH_NOT_ACTIVE`, `PROPOSAL_EXPIRED`.

Thêm mã mới → cập nhật bảng này + `specs` (enum `ProblemCode`) trong cùng MR. CI chạy `node scripts/specs/check-problem-codes.mjs`: so bảng §4.1 với enum `ProblemCode` (OpenAPI) và enum `ErrorCode` (AsyncAPI) cùng danh sách mã chỉ-WS ở trên.

## 5. Pagination, filtering, sorting

- Cursor-based: `?limit=20&cursor=<opaque>`; `limit` mặc định 20, tối đa 100. Cursor là base64url của `(sort_key, id)` có ký HMAC để chống sửa [Proposed].
- Không hỗ trợ offset cho bảng lớn (chat, event, attempt). Bảng nhỏ trong CMS có thể dùng `?page=` [Proposed].
- Filter: tham số tường minh theo field (`?grade_level=3&subject_id=...&status=published`); nhiều giá trị: lặp tham số (`?status=draft&status=in_review`). Không có ngôn ngữ query tự do.
- Sort: `?sort=-created_at,name` chỉ trên whitelist field có index.
- Tìm kiếm text (CMS): `?q=` dùng PostgreSQL full-text (`unaccent` + `simple`) [Proposed].

## 6. Idempotency

| Hạng mục | Quy tắc |
|---|---|
| Header | `Idempotency-Key: <UUID>` (client sinh, UUIDv4/v7) |
| Bắt buộc với | Endpoint có cột "Idem" = **Bắt buộc** trong catalog §9 (submit, nộp thi, tạo party, gửi lời mời, chấp nhận lời mời, matchmaking ticket, gửi tin nhắn qua REST, trao thưởng thủ công, xác nhận kết quả trận/giải) |
| Phạm vi key | `(user_id, method, route_template, key)` |
| Lưu | `platform.idempotency_records`: hash request body, status, response (code + body), `expires_at` = +24h (`EW_IDEMPOTENCY_TTL`) |
| Lặp lại cùng payload | Trả **nguyên response đã lưu** + header `Idempotent-Replayed: true` |
| Cùng key khác payload | `409 IDEMPOTENCY_KEY_REUSED` |
| Đang xử lý | `409 IDEMPOTENCY_IN_PROGRESS` + `Retry-After: 1` (khóa Valkey `ew:idem:lock:<hash>`, TTL 30 s) |
| Lỗi 5xx | Không lưu (client được phép retry); 4xx được lưu |

Idempotency của nghiệp vụ còn được bảo vệ bằng unique constraint DB (một submit/attempt, một `match_results` per match, `xp_ledger` unique) — header chỉ là lớp đầu.

## 7. Optimistic concurrency (CMS/LMS)

- `GET` tài nguyên có thể sửa trả `ETag: W/"<version>"`.
- `PATCH`/`PUT`/workflow transition bắt buộc `If-Match`; thiếu → `428 PRECONDITION_REQUIRED`; lệch → `412 VERSION_MISMATCH` (client tải lại và merge).
- Áp dụng: content draft (question, question set, lesson, quest, minigame config, NPC dialogue, reward rule), assignment, chat policy, school settings.

## 8. Rate limit

APISIX (`limit-count` với Valkey policy store) + giới hạn nghiệp vụ trong Go. Header trả về (IETF draft RateLimit):

```
RateLimit-Limit: 60
RateLimit-Remaining: 12
RateLimit-Reset: 18
Retry-After: 18        # chỉ khi 429
```

| Nhóm route | Giới hạn mặc định [Proposed] | Khóa |
|---|---|---|
| `/api/v1/auth/game/login`, `/auth/game/tenant:select` (ADR-0068) | 10 lần/phút theo `X-Device-Id` **và** 10 lần/phút theo username đã chuẩn hóa (plugin Lua đọc field `username`, không log body); hàng chờ toàn cục theo năng lực (ADR-0045) → `429 LOGIN_QUEUE_BUSY` + `Retry-After` | thiết bị + username + toàn cục |
| `/api/v1/auth/*` (còn lại) | 20 req/phút | IP + `sub` |
| Đọc chung | 300 req/phút | `sub` |
| Ghi chung | 60 req/phút | `sub` |
| `/learning/activity-attempts/*/responses` | 120 req/phút | `sub` |
| `/multiplayer/parties/*/invitations` | 10 lời mời/phút, 50/ngày | `sub` (nghiệp vụ, Valkey) |
| `/social/friend-requests` | 10/giờ, 30/ngày | `sub` |
| `/chat/channels/*/messages` (REST fallback) | theo `ChatPolicy` (mặc định 5 tin/10 s) | `sub` + channel |
| `/cms/learning/media:presign` | 30/phút | `sub` |
| `/schools/*/sis-imports` | 10/giờ | `sub` + `school_id` |
| `/game/telemetry` | 30/phút | `sub` |

## 9. Endpoint catalog

Ký hiệu **Role**: `STU` student, `TCH` teacher, `TA` tenant_admin, `SA` school_admin (một trường trong tenant), `AM` academic_manager (tổ chuyên môn), `CA` content_admin (platform), `CR` content_reviewer, `SYS` system_admin (người duy nhất xem/ghi liên tenant, thay `platform_admin` cũ), `GD` game_designer/liveops/competition_admin, `SVC` service account nội bộ. Scope ghi trong ngoặc: `(self)` chính mình, `(class)` lớp được phân công, `(school)` trường trong scope, `(owner)` phạm vi nội dung. **Idem**: B = bắt buộc `Idempotency-Key`, T = tự nhiên idempotent (GET/PUT/DELETE), — = không. **Phase** theo [ROADMAP](../12-project/ROADMAP.md).

### 9.1. Auth & profile — `/api/v1/auth`, `/api/v1/students`

**Service:** `identity-service` (riêng `/students/{id}/progress` → `learning-service`).

| Method | Path | Mô tả | Role (scope) | Idem | Phase |
|---|---|---|---|---|---|
| POST | `/auth/game/login` | **Đăng nhập game bằng form trong game (ADR-0068). Public, không cần token và `X-Tenant-Id`.** Body `{username, password, device_id, client_version, platform, locale, takeover?}` (`GameLoginRequest`). Một người một tài khoản SSO, username/email duy nhất trên Keycloak (ADR-0069). identity-service: (1) **xác thực SSO** qua token endpoint Keycloak bằng client confidential `eduworld-game-backend` (Direct Access Grants; secret chỉ ở identity-service) — sai → `401 INVALID_CREDENTIALS`, không phải học sinh → `403 GAME_ACCESS_STUDENT_ONLY`; (2) **kiểm tra tài khoản theo `keycloak_sub`**: `user_type`, tenant còn hiệu lực, enrollment, login sync SIS (ADR-0061); (3) **quyết định tenant** (ADR-0063): 1 tenant → vào thẳng; nhiều tenant → `403 TENANT_NOT_SELECTED` + `choices[]` + `login_ticket` (2 phút); chỉ tenant chung → tenant chung; không có tenant hợp lệ → `403 ACCOUNT_NOT_PROVISIONED`; (4) một thiết bị (ADR-0031) và tạo game session. Trả `GameLoginResponse {tokens, bootstrap: PlayerBootstrap, game_session_id}` (`bootstrap.tenant_id` → header `X-Tenant-Id`). Lỗi khác: `409 ACTIVE_SESSION_ON_OTHER_DEVICE` (gọi lại với `takeover: true`), `426 CLIENT_VERSION_UNSUPPORTED`, `429 RATE_LIMITED` / `429 LOGIN_QUEUE_BUSY`. Body **không bao giờ log** (APISIX, service, tracing); không dùng `Idempotency-Key` (không lưu token vào kho idempotency) — `gameLogin` | Không (public) | — | P1 |
| POST | `/auth/game/tenant:select` | `{login_ticket, tenant_id}` khi học sinh thuộc nhiều tenant; không nhập lại mật khẩu. `login_ticket` lưu Valkey, **dùng một lần, TTL 2 phút**, gắn `device_id`; hết hạn/đã dùng/sai thiết bị → `401 LOGIN_TICKET_INVALID` (game quay về form). `tenant_id` không thuộc `choices` → `403 TENANT_MISMATCH`. Trả giống `/auth/game/login`. — `selectGameLoginTenant` | Không (public, ticket) | — | P1 |
| POST | `/auth/game/token:refresh` | `{refresh_token}` → access/refresh token mới (đi qua backend vì refresh token thuộc client confidential). Refresh token hết hạn (idle 30 phút / tối đa 10 giờ, ADR-0064) hoặc bị thu hồi → `401 UNAUTHENTICATED`, game về form đăng nhập. Cần `X-Tenant-Id`; backend kiểm tra membership như ADR-0065. — `refreshGameToken` | STU | — | P1 |
| POST | `/auth/game/logout` | Thu hồi refresh token + phiên Keycloak, đóng game session, thu hồi realtime ticket; gọi khi thoát game hoặc người dùng bấm Đăng xuất. Idempotent (`204` cả khi token đã hết hạn). — `gameLogout` | STU | T | P1 |
| POST | `/auth/session` | Bootstrap sau login **portal** (và client khác dùng luồng trình duyệt): map `(iss,sub)` → User, **không JIT** (ADR-0063; không có membership/UserRole → `403 ACCOUNT_NOT_PROVISIONED` / `403 PORTAL_ACCESS_STAFF_ONLY`), trả bootstrap của phiên. Game gọi `/auth/game/login` thay vì endpoint này (ADR-0068). Không cần `X-Tenant-Id` — `createAuthSession` | Mọi role | T | P1 |
| GET | `/auth/me` | Thông tin `Principal` hiện tại (roles, scopes) | Mọi role | T | P1 |
| — | ~~`/auth/tenants:resolve`~~ | **Không có trong Release 1.0** — đăng nhập không chọn tenant trước (ADR-0063, ADR-0067). Tra mã trường/tenant trước đăng nhập là **mở rộng sau (ADR-0063)**, cần ADR mới | — | — | — |
| GET | `/me/tenants` | **Không cần `X-Tenant-Id`.** Danh sách tenant mà người dùng có membership và (với portal) có UserRole portal còn hiệu lực: `tenant_id`, `tenant_alias`, `tenant_type`, tên, logo, `roles[]` tóm tắt, cờ `has_platform_role`. Portal dùng để: 1 tenant → vào thẳng; nhiều tenant → màn chọn tenant sau đăng nhập và bộ chọn trên topbar; rỗng và không có role platform → `403 PORTAL_ACCESS_STAFF_ONLY` (ADR-0067). Học sinh gọi qua client game nhận đúng một tenant phiên. — `listMyTenants` | Mọi role | T | P1 |
| GET | `/me/permissions` | Role + permission hiệu lực trong tenant của header `X-Tenant-Id` (người dùng tenant: bắt buộc; System Admin bỏ header = phạm vi platform) + danh sách `workspaces` (`school_lms`, `content_studio`, `game_ops`, `live_ops`, `platform_admin`) để dựng workspace switcher và menu của Admin Portal; không phải nguồn quyết định quyền (ADR-0027) — `getMyPermissions` | Mọi role CMS | T | P1 |
| POST | `/auth/logout` | Đóng game session, thu hồi realtime ticket; client tự gọi Keycloak end-session | Mọi role | T | P1 |
| POST | `/auth/sessions:revoke-all` | Đăng xuất trên mọi thiết bị: thu hồi mọi Keycloak session của user + kết thúc game session (ADR-0031) — `revokeAllSessions` | Mọi role (self) | B | P2 |
| GET | `/students/me` | Hồ sơ học sinh tối giản (lớp, khối, trường) | STU (self) | T | P1 |
| GET | `/students/{student_id}` | Hồ sơ học sinh (dạng đầy đủ cho GV) — `getStudentProfile` | TCH (class), SA (school) | T | P3 |
| GET | `/students/{student_id}/progress` | Tiến độ & mastery (**→ `learning-service`**) — `getStudentProgress` | STU (self), TCH (class), SA | T | P3 |

Tính năng đăng nhập nhanh (mã lớp + PIN, thẻ QR, mật khẩu hình ảnh — ADR-0045 mục 4) và tra tenant trước đăng nhập **không có trong Release 1.0**: Release 1.0 chỉ đăng nhập username + mật khẩu (ADR-0063). Các endpoint `/lms/classes/{id}/login-codes*`, `/tenant/students/{id}/login-card|login-pin|picture-password`, internal `/internal/v1/login-credentials:verify` và `/auth/tenants:resolve` là **mở rộng sau (ADR-0063)** — đã gỡ khỏi OpenAPI, cần ADR mới để đưa lại.

#### 9.1.1. Danh mục — `/api/v1/catalog` (ADR-0060)

**Service:** `identity-service`. Dữ liệu quốc gia/tỉnh là dữ liệu platform (không tenant-scoped): `X-Tenant-Id` tùy chọn; `/catalog/schools` tenant-scoped.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/catalog/countries` | Quốc gia `active` (song ngữ theo `Accept-Language`, `ETag`) — `listCountries` | Mọi người dùng đã đăng nhập | T | P1 |
| GET | `/catalog/provinces?country=VN&status=active` | Tỉnh/thành theo quốc gia (34 đơn vị hiện hành của Việt Nam; `status=inactive|all` chỉ System Admin) — `listProvinces` | Mọi người dùng đã đăng nhập | T | P1 |
| GET | `/catalog/schools?province=&school_level=&q=&tenant_ids=` | Danh mục trường (cursor): người dùng tenant thấy trường trong tenant; System Admin lọc `tenant_ids` — `listCatalogSchools` | Mọi role portal | T | P1 |

Trường, lớp, khối, tỉnh của học sinh trả trong `PlayerBootstrap.school` / `.class` (song ngữ) — không cần endpoint `/me/profile/school` riêng.

### 9.2. Game — `/api/v1/game`

**Service:** `game-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/game/client-config` | `min_client_version`, cờ bảo trì, URL manifest, URL API đăng nhập game (`/auth/game/login`, ADR-0068; game không dùng OIDC trình duyệt) — **không cần đăng nhập**, không cần `X-Tenant-Id` (`?platform&version`) | public | T | P1 |
| GET | `/game/profile` | `GameProfile` của mình | STU (self) | T | P1 |
| PATCH | `/game/profile` | Đổi nickname (từ danh sách duyệt), settings, `preferred_language` (`vi-VN`/`en-US`, ADR-0056) | STU (self) | T | P1 |
| GET | `/game/profile/nickname-suggestions?count=` | Gợi ý nickname tự sinh chưa ai dùng trong tenant, theo ngôn ngữ đang chọn (không gõ tự do) — `suggestNicknames` | STU (self) | T | P1 |
| PUT | `/game/profile/avatar` | Cập nhật avatar config (item phải có trong inventory) | STU (self) | T | P2 |
| POST | `/game/sessions` | Tạo `GameSession` (platform, client_version, device_model; header bắt buộc `X-Device-Id`). **Chỉ học sinh** (`GAME_ACCESS_STUDENT_ONLY`). Ghi `game.login_events`, cập nhật `last_login_at`. Đã có session active ở thiết bị khác → `409 ACTIVE_SESSION_ON_OTHER_DEVICE`; gọi lại với `{takeover: true}` để đăng xuất thiết bị kia (ADR-0031) | STU | B | P1 |
| POST | `/game/sessions/{game_session_id}:end` | Kết thúc phiên (`end_reason = logout`; client gọi khi bấm "Đăng xuất" hoặc đóng game) | STU (self) | T | P1 |
| POST | `/game/sessions/{game_session_id}:heartbeat` | Dự phòng khi mất WS: giữ session active (timeout 45 s không heartbeat → `end_reason = timeout`) | STU (self) | T | P1 |
| POST | `/game/sessions/{game_session_id}/realtime-tickets` | Cấp ticket WS 60 s, dùng 1 lần (ADR-0008) | STU (self) | — | P1 |
| GET | `/game/world/manifest` | Manifest thế giới: area, scene, asset bundle PCK (version, checksum, URL CDN) | STU | T | P1 |
| GET | `/game/world/areas` | Danh sách `WorldArea` + quyền truy cập | STU | T | P2 |
| POST | `/game/world/areas/{area_id}:enter` | Xin vào area: kiểm tra quyền, gán instance theo ADR-0012 | STU | T | P2 |
| POST | `/game/world/friends/{user_id}:join` | "Tham gia bạn": vào khu/instance bạn bè đang đứng nếu cùng phạm vi và còn chỗ (`SOCIAL_SCOPE_VIOLATION`, `AREA_LOCKED`) — `joinFriendInWorld` | STU | B | P5 |
| GET | `/game/content-manifest` | Content manifest của tenant (item → version hiện hành, snapshot URL/sha256, bundle, area); `ETag` + `If-None-Match` → `304`. Gọi khi khởi động và khi vào màn mới (ADR-0043) | STU | T | P2 |
| POST | `/game/levels/{level_id}:start` | Vào màn: tạo `LevelSession`, **ghim** version nội dung theo con trỏ hiện hành, trả danh sách snapshot/bundle/media cần preload (signed URL nếu private) | STU | B | P2 |
| POST | `/game/level-sessions/{level_session_id}:end` | Rời/hoàn thành màn — bỏ ghim; màn sau nhận version mới nhất | STU | B | P2 |
| GET | `/game/world/npcs/{npc_id}/dialogue` | NPC dialogue version đang publish | STU | T | P2 |
| GET | `/game/quests` | Quest khả dụng (lọc theo type, trạng thái, `area_id`) | STU | T | P2 |
| GET | `/game/quests/{quest_id}` | Chi tiết quest + `QuestProgress` | STU | T | P2 |
| POST | `/game/quests/{quest_id}:accept` | Nhận quest | STU | B | P2 |
| POST | `/game/quests/{quest_id}:abandon` | Bỏ quest (attempt/learning record đã ghi giữ nguyên) | STU | B | P2 |
| POST | `/game/quests/{quest_id}/steps/{step_id}:report` | Báo bước tương tác (`talk_npc`/`go_to`/`collect`), server kiểm tra điều kiện (`STEP_PRECONDITION_FAILED`) | STU | B | P2 |
| POST | `/game/quests/{quest_id}/steps/{step_id}/activities:start` | Bắt đầu activity học tập của bước → tạo `Attempt` (mode `quest`) | STU | B | P2 |
| POST | `/game/minigames/{minigame_config_id}:start` | Bắt đầu mini-game → `Attempt` (mode `minigame`) + render payload | STU | B | P2 |
| GET | `/game/inventory` | Inventory | STU (self) | T | P2 |
| GET | `/game/achievements` | Achievement/badge + tiến độ | STU (self) | T | P2 |
| GET | `/game/xp` | Level, XP, cap còn lại hôm nay (giữ tương thích; thay bằng `/game/progression`) | STU (self) | T | P2 |
| GET | `/game/progression` | Level, danh hiệu, XP trong level, XP cần, cap còn lại hôm nay, curve version | STU (self) | T | P2 |
| GET | `/game/progression/xp-history?cursor=` | Lịch sử giao dịch XP thân thiện | STU (self) | T | P2 |
| GET | `/game/progression/levels` | Bảng level + phần thưởng sắp tới | STU | T | P2 |
| GET | `/game/world/exploration` | Tiến độ khám phá từng area, khu đã mở, điều kiện mở khu tiếp | STU (self) | T | P2 |
| POST | `/game/world/areas/{area_id}/discoveries/{interactable_id}:discover` | Báo tìm thấy điểm khám phá/vật sưu tầm (`DISCOVERY_ALREADY_FOUND`) | STU | B | P2 |
| GET | `/game/avatar-presets` · PUT `/game/avatar-presets/{slot}` | Bộ phối đồ lưu nhanh (1–3) | STU (self) | T | P2 |
| GET | `/game/challenges?type=&area_id=&difficulty=` | Challenge khả dụng (lọc grade/unlock) | STU | T | P3 |
| GET | `/game/challenges/{challenge_id}` | Chi tiết + lượt thử còn lại | STU | T | P3 |
| POST | `/game/challenges/{challenge_id}:start` | Bắt đầu (`difficulty`, `party_id?`) → `ChallengeAttempt` (`CHALLENGE_NOT_ELIGIBLE`, `CHALLENGE_ATTEMPT_LIMIT`) | STU | B | P3 |
| POST | `/game/challenge-attempts/{challenge_attempt_id}/activities:next` | Hoạt động tiếp theo (tạo attempt học tập qua learning-service) | STU (self) | B | P3 |
| GET | `/game/challenge-attempts/{challenge_attempt_id}` | Trạng thái, điểm, sức bền | STU (self) | T | P3 |
| POST | `/game/challenge-attempts/{challenge_attempt_id}:abandon` | Bỏ challenge | STU (self) | B | P3 |
| GET | `/game/bosses?area_id=` | Boss khả dụng | STU | T | P3 |
| POST | `/game/bosses/{boss_id}/encounters` | Tạo encounter solo hoặc cho party (leader) | STU | B | P3 |
| GET | `/game/boss-encounters/{encounter_id}` | Trạng thái encounter (dự phòng khi mất WS) | STU (member) | T | P3 |
| POST | `/game/boss-encounters/{encounter_id}:leave` | Rời encounter | STU (member) | B | P3 |
| GET | `/game/inventory/history?cursor=` | Lịch sử nhận/dùng vật phẩm | STU (self) | T | P2 |
| GET | `/game/wallet` | Số dư Xu, cap Xu còn lại hôm nay | STU (self) | T | P2 |
| GET | `/game/wallet/ledger?cursor=` | Lịch sử Xu | STU (self) | T | P2 |
| GET | `/game/shop/items?kind=` | Hàng cosmetic đang bán | STU | T | P2 |
| POST | `/game/shop/items/{shop_item_id}:purchase` | Mua bằng Xu (`INSUFFICIENT_BALANCE`, `PURCHASE_LIMIT_REACHED`, `SHOP_ITEM_UNAVAILABLE`, `ITEM_ALREADY_OWNED`) | STU | B | P2 |
| GET | `/game/collections` · `/game/collections/{collection_id}` | Bộ sưu tập + tiến độ + cách nhận | STU (self) | T | P2 |
| POST | `/game/telemetry` | Telemetry không PII, theo lô: `events[]` với `type` ∈ `screen_view`, `tutorial_step`, `perf_sample`, `load_time`, `client_error`, `crash`, `network_sample` (RTT/reconnect — thay `samples[kind=network]` cũ); mỗi lô kèm `platform`, `client_version`, `device_tier`, `preset`, `renderer`, `locale` (thuộc tính: [GAME_ANALYTICS §3.2](../11-operations/GAME_ANALYTICS.md#32-nguồn-client-telemetry-post-apiv1gametelemetry)); server tách lỗi → error tracking, phần còn lại → analytics (`TelemetryBatch`, tối đa 100 event/lô) | STU | — | P2 |
| GET | `/game/events?status=live,upcoming` | Sự kiện đang chạy/sắp diễn ra trong phạm vi học sinh ([EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md)) | STU | T | P3 |
| GET | `/game/events/{event_id}` | Chi tiết run hiện tại: thành phần, lịch, phần thưởng, tiến độ của mình (`EVENT_NOT_AVAILABLE`) | STU | T | P3 |
| POST | `/game/events/{event_id}:join` | Tham gia sự kiện (idempotent; tự động khi hoàn thành hoạt động đầu tiên) | STU | B | P3 |
| POST | `/game/events/{event_id}/milestones/{milestone_id}:claim` | Nhận thưởng mốc điểm sự kiện (`EVENT_MILESTONE_NOT_REACHED`, `EVENT_ENDED`) | STU | B | P3 |
| GET | `/game/community-goals/{goal_id}` | Tiến độ community goal theo phạm vi, mốc, đóng góp theo lớp khi được phép | STU | T | P3 |
| GET | `/game/season-track` | Season track mùa hiện tại: cấp, điểm, trần còn lại hôm nay, phần thưởng, đã nhận | STU (self) | T | P4 |
| POST | `/game/season-track/tiers/{tier}:claim` | Nhận thưởng cấp (`SEASON_TIER_NOT_REACHED`) | STU (self) | B | P4 |
| GET | `/game/season-track/history` | Các mùa trước | STU (self) | T | P4 |

### 9.3. Learning — `/api/v1/learning`

**Service:** `learning-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/learning/curricula` | Curriculum + version đang áp dụng cho trường — `listCurricula` | Mọi role | T | P2 |
| GET | `/learning/subjects` | Môn học theo khối — `listSubjects` | Mọi role | T | P2 |
| GET | `/learning/topics` | Cây chủ đề (`?subject_id&grade_level`) | Mọi role | T | P2 |
| GET | `/learning/objectives` | Learning objectives | Mọi role | T | P2 |
| GET | `/learning/lessons/{lesson_id}` | Lesson published + activities (version ghim theo level session) — `getLesson` | STU, TCH | T | P2 |
| POST | `/learning/lessons/{lesson_id}:complete` | Đánh dấu hoàn thành bài học (server kiểm tra điều kiện xem/activity) — `completeLesson` | STU | B | P2 |
| GET | `/learning/progress/me` | Tiến độ & mastery của mình (dạng thân thiện) | STU (self) | T | P2 |
| GET | `/learning/recommendations/me` | Gợi ý ôn tập theo LO yếu, câu sai đến hạn, bài sắp hết hạn — `getMyRecommendations` | STU (self) | T | P3 |
| GET | `/learning/my-assignments?status=` | Bài thầy cô giao cho học sinh đang đăng nhập (Quest journal) kèm trạng thái và `attempt_id` đang dở — `listMyAssignments`. Đặt dưới `/learning` vì API học sinh thuộc `/learning`, `/lms/*` dành cho nhân sự | STU (self) | T | P3 |
| — | ~~`/learning/events`~~ | **Đã bỏ** (SPEC-3): tiến độ xem/hoạt động đi qua `reportActivityProgress` và attempt (server tự phát `activity.started`), số liệu hành vi màn hình qua `POST /game/telemetry`; không có ingest event học tập từ client | — | — | — |

#### 9.3.1. Learning activities & attempt — một họ API (ADR-0048, ADR-0059)

**Một họ attempt duy nhất** `/learning/activity-attempts` cho mọi `activity_type` (quiz, interactive_exercise, minigame, video, audio, speaking, written_response, simulation, project) và mọi `mode` trừ `exam` (practice, assignment, quest, minigame, challenge, competition, retry_wrong; đề kiểm tra dùng `/exams/*` §9.5 vì có cửa sổ thời gian và chấm trì hoãn, cùng entity Attempt) — [ADR-0059](../adr/ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md). Nhóm route `/api/v1/exercises` **đã bỏ**. Quest, challenge/boss và trận đấu được start bởi game-service/competition-service qua internal `POST /internal/v1/attempts`; sau đó client dùng chung các endpoint dưới đây. Ánh xạ `mode` → `context`: [ACTIVITY_ENGINE §9](../02-learning/ACTIVITY_ENGINE.md#9-api). operationId là tên đề xuất, OpenAPI chốt.

| Method | Path | operationId (đề xuất) | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|---|
| GET | `/learning/activities/{activity_id}` | `getLearningActivity` | Activity published (render payload, **không đáp án**), có `math_assets` SVG | STU, TCH | T | P2 |
| POST | `/learning/activities/{activity_id}/attempts` | `startActivityAttempt` | Bắt đầu attempt từ activity, ghim version (ADR-0043), trả render payload + vị trí resume | STU | B | P2 |
| POST | `/learning/activity-attempts` | `startAdHocAttempt` | Bắt đầu không gắn activity cố định: `source.kind` = `question_set` \| `selection` (chủ đề/LO/độ khó) \| `retry_wrong` | STU | B | P2 |
| GET | `/learning/activity-attempts` | `listMyActivityAttempts` | Lịch sử attempt của mình (`?status&mode&cursor`) | STU (self) | T | P2 |
| GET | `/learning/activity-attempts/{attempt_id}` | `getActivityAttempt` | Trạng thái, items, tiến độ, evaluation có hiệu lực — dùng để resume | STU (self), TCH (class) | T | P2 |
| POST | `/learning/activity-attempts/{attempt_id}/responses` | `submitActivityResponse` | Trả lời một item (câu hỏi, câu chèn video, chọn nhánh, câu hỏi kèm mô phỏng); practice trả kết quả ngay | STU (self) | B | P2 |
| POST | `/learning/activity-attempts/{attempt_id}/progress` | `reportActivityProgress` | Heartbeat tiến độ video/audio/mô phỏng (idempotent `client_seq`) | STU (self) | B | P3 |
| POST | `/learning/activity-attempts/{attempt_id}:pause` | `pauseActivityAttempt` | Tạm dừng theo resume policy | STU (self) | B | P2 |
| POST | `/learning/activity-attempts/{attempt_id}:submit` | `submitActivityAttempt` | Nộp; chấm tự động hoặc chuyển `pending_review` | STU (self) | B | P2 |
| GET | `/learning/activity-attempts/{attempt_id}/review` | `getActivityAttemptReview` | Xem lại: đúng/sai, giải thích, điểm rubric (theo `result_visibility`) | STU (self), TCH (class) | T | P2 |
| GET | `/learning/wrong-questions` | `listMyWrongQuestions` | Danh sách câu sai để ôn (nguồn cho `retry_wrong`) | STU (self) | T | P2 |
| PUT | `/learning/activity-attempts/{attempt_id}/written-response` | — | Lưu nháp bài viết (văn bản, đáp số) | STU (self) | B | P3 |
| POST | `/learning/activity-attempts/{attempt_id}/recordings:presign` | — | Xin URL upload ghi âm (bucket private) | STU (self) | B | P4 |
| POST | `/learning/activity-attempts/{attempt_id}/recordings/{recording_id}:complete` | — | Xác nhận upload | STU (self) | B | P4 |
| GET | `/learning/activity-attempts/{attempt_id}/recordings/{recording_id}` | — | Signed URL nghe lại (15 phút) | STU (self), TCH (class) | T | P4 |
| POST | `/learning/activity-attempts/{attempt_id}/submissions:presign` | — | Upload sản phẩm project / ảnh bài làm ([Proposed]) | STU (self) | B | P4 |
| POST | `/learning/activity-attempts/{attempt_id}/simulation-runs` | — | Gửi action log để server kiểm chứng | STU (self) | B | P4 |

Không có endpoint phản hồi phát âm tự động: Release 1.0 không có AI giọng nói, bài nói do giáo viên chấm tay qua `/lms/reviews` (ADR-0059).

### 9.4. Exercises — đã gộp

Nhóm `/api/v1/exercises` đã bỏ; mọi attempt dùng §9.3.1 (ADR-0059).

### 9.5. Exams — `/api/v1/exams`

**Service:** `learning-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/exams/sessions` | Exam session khả dụng (mock/chapter/semester...) | STU | T | P3 |
| GET | `/exams/sessions/{exam_session_id}` | Chi tiết (thời gian, cấu trúc, `is_official`) | STU, TCH | T | P3 |
| POST | `/exams/sessions/{exam_session_id}/attempts` | Bắt đầu làm đề (sinh đề từ blueprint, `deadline_at` server) | STU | B | P3 |
| PUT | `/exams/attempts/{attempt_id}/answers/{item_id}` | Lưu câu trả lời (autosave, không trả đúng/sai) | STU (self) | T | P3 |
| POST | `/exams/attempts/{attempt_id}:submit` | Nộp bài (chống nộp trùng: `EXAM_ALREADY_SUBMITTED`) | STU (self) | B | P3 |
| GET | `/exams/attempts/{attempt_id}/result` | Kết quả + phân tích câu sai + gợi ý ôn (khi chính sách cho phép) | STU (self), TCH (class) | T | P3 |
| POST | `/exams/sessions` | Tạo exam session cho lớp | TCH (class), AM | B | P3 |

### 9.6. Multiplayer — `/api/v1/multiplayer`

**Service:** `competition-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| POST | `/multiplayer/parties` | Tạo party (`party_type`, `visibility`, `max_members` mặc định 4, cho phép 2–8) | STU | B | P4 |
| GET | `/multiplayer/parties/me` | Active party của mình | STU | T | P4 |
| GET | `/multiplayer/parties/{party_id}` | Chi tiết party + member status | STU (member) | T | P4 |
| PATCH | `/multiplayer/parties/{party_id}` | Party settings (tên, visibility, invitation policy) | STU (leader) | T | P4 |
| POST | `/multiplayer/parties/{party_id}:join` | Tham gia party `visibility=class/friends` (party `private` bắt buộc qua invitation; Class Group dùng `join_code`) | STU | B | P4 |
| POST | `/multiplayer/parties/{party_id}:leave` | Rời party (leader rời → chuyển leader tự động) | STU (member) | T | P4 |
| POST | `/multiplayer/parties/{party_id}:disband` | Giải tán | STU (leader) | T | P4 |
| POST | `/multiplayer/parties/{party_id}/members/{student_id}:kick` | Kick | STU (leader) | T | P4 |
| POST | `/multiplayer/parties/{party_id}:transfer-leader` | Chuyển leader | STU (leader) | B | P4 |
| PUT | `/multiplayer/parties/{party_id}/members/me/ready` | Ready / Not ready | STU (member) | T | P4 |
| PUT | `/multiplayer/parties/{party_id}/activity` | Chọn activity/game mode | STU (leader) | T | P4 |
| POST | `/multiplayer/parties/{party_id}:start` | Start session: kiểm tra member, policy lớp, tạo room/GameSession chung | STU (leader) | B | P4 |
| POST | `/multiplayer/parties/{party_id}/invitations` | Mời bạn (rate limit, chặn nếu block/policy) | STU (member theo policy) | B | P4 |
| GET | `/multiplayer/invitations` | Lời mời nhận được (pending) | STU | T | P4 |
| POST | `/multiplayer/invitations/{invitation_id}:accept` | Chấp nhận | STU (invitee) | B | P4 |
| POST | `/multiplayer/invitations/{invitation_id}:reject` | Từ chối | STU (invitee) | T | P4 |
| POST | `/multiplayer/invitations/{invitation_id}:cancel` | Hủy lời mời đã gửi | STU (inviter/leader) | T | P4 |
| GET | `/multiplayer/rooms/{room_id}` | Thông tin room (type, members, endpoint WS) | STU (member) | T | P4 |

#### 9.6.1. Phân chia REST và WebSocket (party, matchmaking)

Khớp với [AsyncAPI](../../specs/asyncapi/eduworld-realtime.v1.yaml) và [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md):

| Nhóm thao tác | Kênh chính | Lý do |
|---|---|---|
| Tạo party, `:join`, mời, chấp nhận/từ chối/hủy lời mời, kick, chuyển leader, giải tán, rời party, sửa settings | **Chỉ REST** (`Idempotency-Key` với thao tác tạo/mời/chấp nhận/join) | Thay đổi membership có thẩm quyền, cần idempotency, audit, rate limit nghiệp vụ |
| Tạo matchmaking ticket | **Chỉ REST** `POST /competition/matchmaking/tickets` (`Idempotency-Key`) | Kiểm tra điều kiện grade/policy, chống ticket trùng (`MM_TICKET_ALREADY_ACTIVE`) |
| Ready/Not ready, chọn activity, start session (party lobby) | **WS chính**: `party.member.ready_set`, `party.activity.select`, `party.session.start`. REST tương ứng là **fallback** khi WS mất | Tương tác lobby cần độ trễ thấp; cả hai đường gọi cùng một use case nên kết quả giống nhau |
| Chấp nhận/từ chối match proposal, hủy ticket | **WS chính**: `matchmaking.proposal.accept`/`decline`, `matchmaking.ticket.cancel`. REST (`proposal:accept`/`:decline`, `DELETE tickets/{id}`) là fallback | Ready check 15 s |
| Sự kiện trạng thái (member joined/left, invitation received, ticket updated, match assigned) | **Chỉ WS** (server → client); client resync bằng REST `GET` khi `system.session.resume_failed` | Push realtime |

Quy tắc: lệnh WS không bao giờ tạo hoặc xóa membership. REST là nguồn có thẩm quyền cho mọi thay đổi trạng thái bền vững.

### 9.7. Competition — `/api/v1/competition`

**Service:** `competition-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/competition/modes` | Chế độ thi đấu khả dụng theo khối/môn | STU | T | P4 |
| POST | `/competition/matchmaking/tickets` | Vào hàng đợi (solo hoặc party leader) | STU | B | P4 |
| GET | `/competition/matchmaking/tickets/{ticket_id}` | Trạng thái ticket (fallback khi mất WS) | STU (owner) | T | P4 |
| DELETE | `/competition/matchmaking/tickets/{ticket_id}` | Rời hàng đợi | STU (owner) | T | P4 |
| POST | `/competition/duels` | Thách đấu bạn (friend duel, unranked, hết hạn 60 s — MATCHMAKING §8) | STU | B | P4 |
| POST | `/competition/duels/{duel_id}:accept` | Nhận thách đấu → tạo match unranked (`DUEL_EXPIRED`) | STU (opponent) | B | P4 |
| POST | `/competition/duels/{duel_id}:decline` | Từ chối thách đấu | STU (opponent) | T | P4 |
| POST | `/competition/matches/{match_id}/proposal:accept` | Ready check chấp nhận | STU (participant) | T | P4 |
| POST | `/competition/matches/{match_id}/proposal:decline` | Từ chối | STU (participant) | T | P4 |
| GET | `/competition/matches/{match_id}` | Trạng thái trận + kết quả (sau khi kết thúc) | STU (participant), TCH (class) | T | P4 |
| GET | `/competition/matches` | Match history của mình | STU (self) | T | P4 |
| GET | `/competition/ratings/me` | Rating/tier theo mode/subject/season | STU (self) | T | P4 |
| GET | `/competition/seasons` | Mùa hiện tại/đã qua | Mọi role | T | P4 |
| GET | `/competition/tournaments` | Giải đấu trong scope (class/school/organization) | STU, TCH, SA | T | P4 |
| — | *(không có `POST /competition/tournaments`)* | Tenant tạo giải trường/lớp qua `POST /lms/challenges` từ **tournament template** của platform; luật/phần thưởng lấy từ template (ADR-0027) | — | — | P4 |
| GET | `/competition/tournaments/{tournament_id}` | Chi tiết + bracket | STU, TCH, SA | T | P4 |
| POST | `/competition/tournaments/{tournament_id}/entries` | Đăng ký (cá nhân/đội) | STU | B | P4 |
| POST | `/competition/tournaments/{tournament_id}:generate-bracket` | Chia bảng/tạo bracket | TCH, SA (owner) | B | P4 |
| POST | `/competition/tournaments/{tournament_id}/matches/{match_id}:confirm-result` | Xác nhận kết quả (khi cần trọng tài) | TCH, SA (owner) | B | P4 |

Gửi đáp án trong trận **chỉ qua WebSocket** (server-authoritative state machine, [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md)).

### 9.8. Leaderboards — `/api/v1/leaderboards`

**Service:** `competition-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/leaderboards` | Danh sách `LeaderboardDefinition` khả dụng (scope × period × metric) | STU, TCH | T | P4 |
| GET | `/leaderboards/{leaderboard_id}` | Trang xếp hạng (`?period_key&cursor`) — chỉ nickname/avatar | STU, TCH | T | P4 |
| GET | `/leaderboards/{leaderboard_id}/me` | Vị trí của mình + lân cận ±5 | STU | T | P4 |

### 9.9. Chat — `/api/v1/chat`

**Service:** `chat-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/chat/channels` | Channel của mình + unread count | STU, TCH | T | P5 |
| GET | `/chat/channels/{channel_id}` | Chi tiết + policy hiệu lực (allowed kinds) | member | T | P5 |
| GET | `/chat/channels/{channel_id}/messages` | Lịch sử (`?before_seq&after_seq&limit`) theo retention/policy — hợp nhất PostgreSQL + bộ đệm gần nhất (ADR-0035); tin thu hồi/gỡ trả placeholder | member | T | P4 |
| GET | `/chat/channels/{channel_id}/pins` | Tin đang ghim (≤ 3) | member | T | P4 |
| GET | `/chat/channels/{channel_id}/messages/{message_id}/receipts` | Trạng thái đã nhận/đã xem: nhóm ≤ 8 → danh sách; class/study_group → số đếm (danh sách chỉ cho giáo viên) | member | T | P4 |
| GET | `/chat/search` | Tìm full-text (`?q&channel_id&sender_id&from&to&cursor`) trong channel mình là thành viên; tenant-scoped; không trả tin thu hồi/bị chặn | STU, TCH | T | P5 |
| GET | `/chat/stickers` | Sticker pack được gán cho tenant + trạng thái mở khóa của học sinh | STU | T | P4 |
| GET | `/chat/gifs` | Thư viện GIF nội bộ (`?tag&cursor`) | STU | T | P4 |
| POST | `/chat/channels/{channel_id}/messages` | Gửi tin (fallback REST; đường chính là WS) | member theo policy | B | P5 |
| POST | `/chat/channels/{channel_id}/read` | Cập nhật `last_read_seq` | member | T | P5 |
| PUT | `/chat/channels/{channel_id}/mute` | Tắt thông báo channel | member | T | P5 |
| POST | `/chat/direct-channels` | Mở DM (chỉ giữa bạn bè không block nhau, ADR-0020) | STU, TCH | B | P5 |
| GET | `/chat/quick-messages?channel_type=` | Bộ Quick Message (dùng chung mọi lớp), văn bản song ngữ `{vi, en}`; tin gửi chỉ mang `quick_message_id`, người nhận hiển thị theo ngôn ngữ của mình (ADR-0056) — `listQuickMessages` | STU | T | P5 |
| POST | `/chat/messages/{message_id}/reactions` | Reaction emoji (REST fallback của `chat.reaction.toggle`) | member | T | P4 |
| PATCH | `/chat/messages/{message_id}` | Sửa tin (REST fallback của `chat.message.edit`, `If-Match: revision`) | người gửi | T | P4 |
| POST | `/chat/messages/{message_id}:recall` | Thu hồi (REST fallback) | người gửi | T | P4 |
| POST | `/chat/messages/{message_id}:forward` | Forward tới ≤ 5 channel (REST fallback) | member | B | P4 |
| PUT/DELETE | `/chat/channels/{channel_id}/pins/{message_id}` | Ghim / bỏ ghim (REST fallback) | theo §7.8 CHAT_SYSTEM | T | P4 |
| GET | `/chat/moderation/cases/{case_id}/search` | Tìm trong snapshot + ngữ cảnh của case (không có tìm kiếm toàn bộ chat) | người được gán case | T | P5 |
| POST | `/chat/reports` | Báo cáo tin nhắn/người dùng | Mọi role | B | P5 |
| GET | `/chat/moderation/cases` | Hàng đợi kiểm duyệt (scope trường/lớp) | TCH (class), SA (school) | T | P5 |
| POST | `/chat/moderation/cases/{case_id}/actions` | Xử lý: warn/mute/restrict/remove_message/escalate/dismiss | TCH, SA | B | P5 |
| GET | `/chat/policies/{scope_type}/{scope_id}` | Xem chat policy | SA, TCH (class), SYS | T | P5 |
| PUT | `/chat/policies/{scope_type}/{scope_id}` | Cập nhật (chỉ siết chặt hơn cấp trên) — `If-Match` | SA (school), TCH (class), SYS | T | P5 |

**An toàn chat — danh mục từ nhạy cảm & công tắc kiểm duyệt (ADR-0030).** Danh mục không bao giờ trả cho client game; mọi endpoint ghi có audit.

| Method | Path | Mô tả | Role · permission | Idem | Phase |
|---|---|---|---|---|---|
| GET/POST | `/admin/chat/sensitive-categories` | Category platform (code, severity, default_action, enabled, allow_tenant_toggle) | SYS · `chat.sensitive_terms.read/write` | T/B | P5 |
| PATCH/DELETE | `/admin/chat/sensitive-categories/{category_id}` | Sửa / xóa mềm category (`If-Match`) | SYS · `chat.sensitive_terms.write` | T | P5 |
| GET/POST | `/admin/chat/sensitive-terms?category_id=&q=&tenant_ids=` | Term platform (và term tenant khi lọc `tenant_ids`) | SYS · `chat.sensitive_terms.read/write` | T/B | P5 |
| PATCH/DELETE | `/admin/chat/sensitive-terms/{term_id}` | Sửa / xóa mềm term | SYS · `chat.sensitive_terms.write` | T | P5 |
| POST | `/admin/chat/sensitive-terms:test` | Thử chuỗi `{text, tenant_id?}` → danh sách `{category_code, severity, action, span}` + decision | SYS · `chat.sensitive_terms.read` | T | P5 |
| GET/POST/DELETE | `/admin/chat/sensitive-allowlist` | Ngoại lệ (cụm hợp lệ) | SYS · `chat.sensitive_terms.write` | T/B/T | P5 |
| POST | `/admin/chat/sensitive-catalog:publish` | Publish thay đổi → `catalog_version` mới (chặn nếu test hồi quy fail: `SENSITIVE_CATALOG_REGRESSION_FAILED`) | SYS · `chat.sensitive_terms.write` | B | P5 |
| GET | `/admin/chat/sensitive-catalog/versions` | Lịch sử version | SYS | T | P5 |
| GET | `/admin/chat/moderation-switches?tenant_ids=` | Trạng thái công tắc (platform + từng tenant, giá trị hiệu lực) | SYS · `chat.policy.toggle` | T | P5 |
| PUT | `/admin/chat/moderation-switches` | Body `{scope (platform/tenant), tenant_id?, enabled_all, steps{sensitive_terms,pii,spam,bullying,async_analysis}, reason, expires_at?}`; thiếu reason → `MODERATION_SWITCH_REASON_REQUIRED` | SYS · `chat.policy.toggle` | B | P5 |
| GET | `/admin/chat/moderation-switches/history?tenant_ids=` | Lịch sử bật/tắt | SYS | T | P5 |
| GET/POST | `/tenant/chat/sensitive-terms` | Term bổ sung của tenant (chỉ thêm, category có sẵn) | TA, tenant moderator · `chat.tenant_terms.write` | T/B | P5 |
| PATCH/DELETE | `/tenant/chat/sensitive-terms/{term_id}` | Sửa / xóa mềm term của tenant (không sửa term platform → `SENSITIVE_TERM_INVALID`) | TA · `chat.tenant_terms.write` | T | P5 |
| POST | `/tenant/chat/sensitive-terms:test` | Thử chuỗi với danh mục hiệu lực của tenant | TA · `chat.sensitive_terms.read` | T | P5 |
| POST | `/tenant/chat/sensitive-catalog:publish` | Publish term tenant → version tenant | TA · `chat.tenant_terms.write` | B | P5 |
| GET | `/tenant/chat/categories` | Category platform + trạng thái bật/tắt trong tenant + trạng thái công tắc hiệu lực (để hiển thị banner) | TA, moderator | T | P5 |
| POST | `/tenant/chat/categories/{code}:toggle` | `{enabled, reason}`; chỉ khi `allow_tenant_toggle` (ngược lại `TENANT_TOGGLE_NOT_ALLOWED`) | TA · `chat.tenant_category.toggle` | B | P5 |

### 9.10. Social — `/api/v1/social`

**Service:** `chat-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/social/friends` | Danh sách bạn + presence tóm tắt | STU | T | P5 |
| DELETE | `/social/friends/{user_id}` | Hủy kết bạn (không thông báo người kia) | STU | T | P5 |
| POST | `/social/friend-codes` | Tạo mã bạn bè 10 phút | STU | B | P5 |
| POST | `/social/friend-codes:redeem` | Nhập mã bạn bè → gửi lời mời (`FRIEND_CODE_INVALID`, `SOCIAL_SCOPE_VIOLATION`) | STU | B | P5 |
| POST | `/social/friend-requests` | Gửi lời mời kết bạn (mặc định chỉ cùng trường) | STU | B | P5 |
| GET | `/social/friend-requests` | Lời mời đến/đi (`?direction=incoming`) | STU | T | P5 |
| POST | `/social/friend-requests/{request_id}:accept` | Chấp nhận | STU (addressee) | B | P5 |
| POST | `/social/friend-requests/{request_id}:reject` | Từ chối | STU (addressee) | T | P5 |
| GET | `/social/blocks` | Danh sách chặn | STU | T | P5 |
| PUT | `/social/blocks/{user_id}` | Chặn | STU | T | P5 |
| DELETE | `/social/blocks/{user_id}` | Bỏ chặn | STU | T | P5 |
| GET | `/social/mutes` | Danh sách người đã tắt tiếng | STU | T | P5 |
| PUT/DELETE | `/social/mutes/{user_id}` | Tắt tiếng / bỏ tắt tiếng | STU | T | P5 |
| GET/POST | `/social/clubs` | Club của tôi / có thể tham gia; tạo club (HS tạo → `pending_approval`) — Study Group, Team cố định là **loại club** (ADR-0050) | STU, TCH | T/B | P5 |
| GET/PATCH | `/social/clubs/{club_id}` | Chi tiết; sửa thông tin (owner/officer, qua kiểm duyệt, `If-Match`) | Thành viên; owner/officer | T | P5 |
| POST | `/social/clubs/{club_id}:approve` | Giáo viên duyệt club do HS tạo | TCH (supervisor) | B | P5 |
| POST | `/social/clubs/{club_id}:leave`, `:archive` | Rời club; giải tán/lưu trữ | Thành viên; owner, TCH | T/B | P5 |
| GET | `/social/clubs/{club_id}/members` | Thành viên (nickname, vai trò) | Thành viên | T | P5 |
| PATCH/DELETE | `/social/clubs/{club_id}/members/{user_id}` | Đổi vai trò; gỡ thành viên | Owner, TCH | T | P5 |
| GET/POST | `/social/clubs/{club_id}/join-requests` | Yêu cầu chờ duyệt; xin tham gia | Owner/officer; STU | T/B | P5 |
| POST | `/social/club-join-requests/{request_id}:accept`, `:reject` | Duyệt / từ chối | Owner/officer | B/T | P5 |
| POST | `/social/clubs/{club_id}/invitations` | Mời (≤ 50/lần, kiểm tra phạm vi, block) | Owner/officer | B | P5 |
| POST | `/social/club-invitations/{invitation_id}:accept`, `:decline` | Nhận / từ chối lời mời club | STU (invitee) | B/T | P5 |
| GET/POST | `/social/clubs/{club_id}/goals` | Mục tiêu tuần (đếm từ event, không chia sẻ điểm cá nhân) | Thành viên; owner, TCH | T/B | P5 |
| GET/POST | `/social/clubs/{club_id}/events` | Lịch sự kiện club (giải nội bộ từ template) | Thành viên; owner, TCH | T/B | P5 |

### 9.11. Notifications — `/api/v1/notifications`

**Service:** `chat-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/notifications` | Thông báo (`?unread=true`) | Mọi role | T | P2 |
| POST | `/notifications/{notification_id}:read` | Đánh dấu đã đọc | owner | T | P2 |
| POST | `/notifications:read-all` | Đọc tất cả | owner | T | P2 |

### 9.12. Schools (quản trị trường) — `/api/v1/schools`

**Service:** `identity-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/schools` | Trường trong scope | SA, SYS | T | P1 |
| GET | `/schools/{school_id}` | Chi tiết + settings | SA (school), SYS | T | P1 |
| PATCH | `/schools/{school_id}/settings` | Cài đặt trường (giờ truy cập, tính năng; không có JIT — ADR-0063) — `If-Match` | SA (school) | T | P3 |
| GET | `/schools/{school_id}/classes` | Lớp theo năm học | TCH, SA | T | P1 |
| GET | `/schools/{school_id}/classes/{class_id}/students` | Danh sách học sinh lớp | TCH (class), SA | T | P3 |
| GET | `/schools/{school_id}/teachers` | Giáo viên | SA | T | P3 |
| GET / POST | `/schools/{school_id}/user-roles` | Xem / gán UserRole thủ công (`source = manual`) trong phạm vi trường (`scope_type` = `school`/`class`/`subject` thuộc trường) — chỉ role tenant mà người gán được phép cấp (ADR-0066); thay `PUT /schools/{id}/role-assignments/{id}` cũ — `listSchoolUserRoles`, `grantSchoolUserRole` | SA · `tenant.user_role.write` (scope trường) | T / B | P3 |
| POST | `/schools/{school_id}/sis-imports` | School Admin tải CSV SIS (`csv_import`, cột theo EduWorld SIS Contract): trả presigned URL, file ≤ 10 MB [Proposed], worker xử lý bất đồng bộ → SyncRun | SA (school) | B | P1 |
| GET | `/schools/{school_id}/sis-imports/{import_id}` | Trạng thái import (dòng hợp lệ/bị từ chối, số reconciliation issue) | SA (school) | T | P1 |

`sis-imports` là **đường public duy nhất** chạm tới SIS: chỉ role `school_admin` trong scope trường, không lộ cấu hình kết nối/credential. Mọi thao tác SIS khác ở `/api/v1/sis/*` (gateway nội bộ).

### 9.13. CMS nội dung học tập — `/api/v1/cms/learning`

**Service:** `learning-service`.

Vùng **Learning Management** (ADR-0027): dùng được bởi vận hành tenant và đội platform qua Admin Portal (client `eduworld-portal`, workspace B và C). **Không** chứa bất kỳ cấu hình game nào (đó là §9.13b). Mọi endpoint ghi dùng `If-Match`. Nội dung có `owner_scope` (platform/organization/school); tenant chỉ ghi nội dung `owner_scope` của trường/organization mình. Ký hiệu thêm: `SCE`/`SCR` school content editor/reviewer.

| Method | Path | Mô tả | Permission (role điển hình) | Idem | Phase |
|---|---|---|---|---|---|
| GET/POST | `/cms/learning/curricula`, `/cms/learning/curricula/{id}/versions` | Curriculum + version | `curriculum.read` (mọi role CMS) / `curriculum.write` (AM scope tenant, CA) | T/B | P3 |
| GET/POST/PATCH | `/cms/learning/subjects`, `/cms/learning/topics`, `/cms/learning/objectives` | Cây chương trình | `curriculum.write` (AM, CA) | T/B/T | P3 |
| GET | `/cms/learning/questions` | Tìm kiếm question bank (`?q&grade_level&subject_id&topic_id&objective_id&difficulty&status&type`) | `learning_content.read` (CA, AM, SA, TCH, SCE, SCR, CR) | T | P2 |
| POST | `/cms/learning/questions` | Tạo question (version 1 draft) | `learning_content.write` (CA, SA, TCH, SCE) | B | P2 |
| GET | `/cms/learning/questions/{question_id}/versions/{version}` | Xem version (kể cả answer key — chỉ role CMS có `learning_content.read` trong scope) | `learning_content.read` | T | P2 |
| PATCH | `/cms/learning/questions/{question_id}/versions/{version}` | Sửa draft (`CONTENT_NOT_EDITABLE` nếu đã publish) | `learning_content.write` (owner) | T | P2 |
| POST | `/cms/learning/questions/{question_id}/versions` | Tạo version mới từ version cũ | `learning_content.write` | B | P2 |
| POST | `/cms/learning/content/{content_type}/{content_id}/versions/{version}:transition` | Workflow `submit_review/approve/reject/archive` (publish/rollback/unpublish dùng endpoint riêng bên dưới); `content_type ∈ {question, question_set, assessment_blueprint, lesson}` | `learning_content.write/review/publish` theo transition | B | P2 |
| GET | `/cms/learning/content/{content_type}/{content_id}/versions` | Lịch sử version + version đang là con trỏ release (ADR-0043) | `learning_content.read` | T | P2 |
| GET | `/cms/learning/content/{content_type}/{content_id}/versions/{version}:diff?with={v}` | So sánh hai version (diff theo field/khối; `answer_key` chỉ với `learning_content.review`) | `learning_content.read` | T | P2 |
| POST | `/cms/learning/content/{content_type}/{content_id}/versions/{version}:publish` | Xuất bản ngay hoặc hẹn giờ (`publish_at`): dựng snapshot → đổi con trỏ → `content.published`; `If-Match` | `learning_content.publish` | B | P2 |
| POST | `/cms/learning/content/{content_type}/{content_id}:rollback` | Dùng lại version đã publish (`to_version`, `reason` bắt buộc); chỉ đổi con trỏ | `content.release.rollback` | B | P2 |
| POST | `/cms/learning/content/{content_type}/{content_id}:unpublish` | Gỡ xuất bản (`reason`); lượt đang chơi giữ version đã ghim | `learning_content.publish` | B | P2 |
| GET | `/cms/learning/content/{content_type}/{content_id}/releases` | Lịch sử publish/rollback/unpublish | `learning_content.read` | T | P2 |
| POST | `/cms/learning/preview/sessions` | Preview nội dung học tập (theo version bất kỳ) | `learning_content.read` | B | P3 |
| GET/POST/PATCH | `/cms/learning/question-sets` (+ `/versions`) | Question set / selection rule | `learning_content.write` | T/B/T | P2 |
| GET/POST/PATCH | `/cms/learning/assessment-blueprints` (+ `/versions`) | Ma trận đề | `learning_content.write` | T/B/T | P3 |
| GET/POST/PATCH | `/cms/learning/lessons` (+ `/versions`) | Bài học | `learning_content.write` | T/B/T | P3 |
| GET/POST/PATCH | `/cms/learning/activities` (+ `/versions`) | Learning activity mọi `activity_type`; `config` validate theo [schema loại](../../specs/schemas/activities/activity-base.schema.json) (`ACTIVITY_CONFIG_INVALID`); có công thức thì gửi kèm SVG MathJax 4 (`MATH_RENDER_FAILED`) — publish/rollback dùng endpoint chung `content/{content_type}/...` với `content_type = activity` [Proposed] | `learning_content.write` | T/B/T | P2 |
| GET | `/cms/learning/activity-types` | Danh mục `activity_type` + JSON Schema cấu hình (editor registry của portal) | `learning_content.read` | T | P2 |
| GET/POST/PATCH | `/cms/learning/rubrics` (+ `/versions`) | Rubric ([WRITTEN_RESPONSE_RUBRIC](../02-learning/WRITTEN_RESPONSE_RUBRIC.md)); publish/rollback qua endpoint chung | `learning_content.write` | T/B/T | P3 |
| GET | `/cms/learning/simulation-templates` | Template mô phỏng đã đăng ký + `parameter_schema` (chỉ đọc; platform đăng ký khi phát hành game) | `learning_content.read` | T | P4 |
| POST | `/cms/media:presign-upload` | Xin presigned PUT lên `ew-media-quarantine` (giới hạn loại + dung lượng, `sha256` để khử trùng); asset `quarantined` → media worker (ADR-0044) | `media.upload` | B | P2 |
| GET | `/cms/media`, `/cms/media/{asset_id}` | Thư viện media: trạng thái pipeline, biến thể, nơi đang dùng | `learning_content.read` | T | P2 |
| POST | `/cms/learning/imports` | Bulk import câu hỏi/bài học | `learning_content.import` | B | P3 |
| GET | `/cms/learning/review-tasks` | Hàng đợi review theo lane (`school`/`organization`/`platform_learning`) | `learning_content.review` | T | P2 |
| GET | `/cms/learning/audit` | Audit trail nội dung trong scope | `learning_content.read` + scope | T | P3 |
| POST | `/cms/learning/translations:export` | Xuất trường cần dịch (XLIFF/CSV, mặc định chỉ trường còn thiếu) → job `202` — `exportLearningTranslations` (ADR-0056) | `learning_content.translate` | B | P3 |
| POST | `/cms/learning/translations:import` | Nhập bản dịch (file đã upload qua `/cms/media:presign-upload`); tạo version Draft mới, không sửa version đã publish (ADR-0043); `dry_run` để kiểm tra khóa — `importLearningTranslations` | `learning_content.translate` | B | P3 |
| GET | `/cms/learning/translations/jobs/{job_id}` | Trạng thái job dịch + báo cáo (cập nhật/bỏ qua/lỗi) — `getLearningTranslationJob` | `learning_content.translate` | T | P3 |

### 9.13b. Game Studio & cấu hình thi đấu — `/api/v1/game-admin` (INTERNAL, platform-only)

**Service:** `game-service` / `competition-service` / `chat-service` theo `config_type` (cột Service bên dưới).

**ADR-0027.** Chỉ publish trên **APISIX nội bộ** (VPN); backend chỉ chấp nhận token `azp = eduworld-portal` có MFA (`acr`, thiếu → `403 MFA_REQUIRED`) và Principal có role platform với permission tương ứng. Role tenant không bao giờ có permission `game.*`/`competition.config.*` nên không thể gọi các endpoint này kể cả khi vào được mạng nội bộ. OpenAPI dùng path tham số `/game-admin/{config_type}` với `config_type` là enum `GameConfigType`; payload của từng loại validate theo JSON Schema trong `specs/schemas/` (nếu có).

| `config_type` | Service | Nội dung | Permission | Schema | Phase |
|---|---|---|---|---|---|
| `quests` | `game-service` | Quest definition + version | `game.quest.write` | `quest.schema.json` | P2 |
| `minigame-configs` | `game-service` | Cấu hình mini-game | `game.minigame.write` | `minigame-config.schema.json` | P2 |
| `npc-dialogues` | `game-service` | Hội thoại NPC | `game.npc.write` | `npc-dialogue.schema.json` | P2 |
| `world-areas` | `game-service` | Vùng/area: tên hiển thị, scene, capacity, `access_policy`, `unlock_rule` ([WORLD_DESIGN §3, §7](../01-game-design/WORLD_DESIGN.md)) | `game.world.write` | — | P2 |
| `events` | `game-service` | Sự kiện live-ops mọi loại (thay `world-events`; [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md)) | `game.event.write` | `event.schema.json` | P3 |
| `event-templates` | `game-service` | Template sự kiện cho tenant (`allowed_params`) | `game.event.write` | — | P3 |
| `season-tracks` | `game-service` | Season track miễn phí theo mùa | `game.season_track.write` | `season-track.schema.json` | P4 |
| `story-chapters` | `game-service` | Chương truyện "Thắp lại Tia Sáng" (vùng, chuỗi main quest, thứ tự) | `game.quest.write` | — | P2 |
| `items` | `game-service` | Vật phẩm/cosmetic | `game.item.write` | — | P2 |
| `reward-rules` | `game-service` | Reward rule + cap + diminishing | `game.reward_rule.write` | `reward-rule.schema.json` | P2 |
| `levels` | `game-service` | Bảng XP → level (giữ tương thích; dùng `progression-curves`) | `game.level.write` | — | P2 |
| `progression-curves` | `game-service` | Đường cong XP, danh hiệu, phần thưởng level | `game.progression.write` | `progression-curve.schema.json` | P2 |
| `challenges` | `game-service` | Challenge PvE (11 loại, 5 mức độ khó) | `game.challenge.write` | `challenge.schema.json` | P3 |
| `bosses` | `game-service` | Boss nhiều phase | `game.boss.write` | `boss.schema.json` | P3 |
| `achievements` | `game-service` | Thành tựu, tier, phần thưởng | `game.achievement.write` | `achievement.schema.json` | P2 |
| `collections` | `game-service` | Bộ sưu tập (pet, knowledge card, cosmetic, badge, exploration, event) | `game.collection.write` | — | P2 |
| `shop-items` | `game-service` | Mặt hàng Shop (cosmetic, giá Xu) | `game.shop.write` | `shop-item.schema.json` | P2 |
| `currencies` | `game-service` | Loại tiền trong game, cap ngày | `game.economy.write` | — | P2 |
| `matchmaking-configs` | `competition-service` | Queue, grade band, cửa sổ rating | `competition.config.matchmaking.write` | — | P4 |
| `ranking-rules` | `competition-service` | Tham số Glicko-2, soft reset | `competition.config.ranking.write` | — | P4 |
| `seasons` | `competition-service` | Mùa giải | `competition.config.season.write` | — | P4 |
| `tournament-templates` | `competition-service` | Template giải đấu (tenant tạo giải từ đây) | `competition.config.tournament_template.write` | — | P4 |
| `challenge-templates` | `competition-service` | Template thử thách lớp | `competition.config.tournament_template.write` | — | P4 |
| `leaderboard-definitions` | `competition-service` | Định nghĩa bảng xếp hạng | `competition.config.leaderboard.write` | — | P4 |
| `sticker-packs` | `chat-service` | Sticker pack + sticker (upload qua presigned URL, `unlock_rule`, publish, gán tenant) | `game.chat_assets.write` | — | P4 |
| `gifs` | `chat-service` | Thư viện GIF nội bộ (upload presigned, tag, duyệt, publish) | `game.chat_assets.write` | — | P4 |

| Method | Path | Mô tả | Idem | Phase |
|---|---|---|---|---|
| GET/POST | `/game-admin/{config_type}` | Danh sách / tạo (version 1 draft) | T/B | P2 |
| GET/PATCH | `/game-admin/{config_type}/{config_id}/versions/{version}` | Xem / sửa draft (`If-Match`) | T | P2 |
| POST | `/game-admin/{config_type}/{config_id}/versions/{version}:transition` | `submit_review/approve/reject/publish/archive/rollback` (+ `effective_from` cho cấu hình thi đấu); review cần `game.content.review`, publish cần `game.content.publish` | B | P2 |
| GET | `/game-admin/{config_type}/{config_id}/versions` (+ `/versions/{version}:diff?with=`) | Lịch sử version, diff (ADR-0043) | T | P2 |
| POST | `/game-admin/{config_type}/{config_id}/versions/{version}:publish` | Xuất bản ngay/hẹn giờ; cập nhật content manifest các tenant liên quan; cần `game.content.publish` | B | P2 |
| POST | `/game-admin/{config_type}/{config_id}:rollback` / `:unpublish` | Đổi con trỏ về version cũ / gỡ (lý do bắt buộc; `content.release.rollback`) | B | P2 |
| GET | `/game-admin/{config_type}/{config_id}/releases` | Lịch sử release | T | P2 |
| POST / GET | `/game-media:presign-upload`, `/game-media/{asset_id}` | Upload/xem media nội dung game (`game-service`, gateway nội bộ; tách prefix khỏi `/game-admin/{config_type}` để path không mơ hồ) | B / T | P2 |
| POST | `/game-admin/preview/sessions` | Preview token Godot Web cho draft | B | P3 |
| POST | `/game-admin/translations:export` / `:import` | Xuất/nhập file dịch nội dung game (XLIFF/CSV, job `202`); nhập tạo version Draft mới; permission `game.content.translate` (ADR-0056) — `exportGameTranslations`, `importGameTranslations` | B | P3 |
| GET | `/game-admin/translations/jobs/{job_id}` | Trạng thái job dịch — `getGameTranslationJob` | T | P3 |
| POST | `/game-admin/events/{event_id}/runs/{run_id}:suspend` / `:resume` / `:cancel` | Dừng khẩn cấp / tiếp tục / hủy sự kiện đang chạy; lý do bắt buộc; audit; permission `game.event.operate` ([EVENT_SEASON_SYSTEM §5](../01-game-design/EVENT_SEASON_SYSTEM.md#5-vòng-đời-sự-kiện)) | B | P3 |
| GET | `/game-admin/events/{event_id}/runs/{run_id}/stats` | Số tham gia, tiến độ community goal, tỷ lệ nhận thưởng; `game.liveops.read` | T | P3 |

Mã lỗi sự kiện/mùa (`EVENT_NOT_AVAILABLE`, `EVENT_ENDED`, `EVENT_MILESTONE_NOT_REACHED`, `SEASON_TIER_NOT_REACHED`, `EVENT_TEMPLATE_PARAM_NOT_ALLOWED`) và `TRANSLATION_INCOMPLETE` nằm trong §4.1 và enum `ProblemCode`. `world-events` đã bỏ khỏi enum `GameConfigType` (thay bằng `events`).

### 9.14. LMS — `/api/v1/lms`

**Service:** `learning-service`.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET/POST | `/lms/courses` | Khóa học theo lớp × môn | TCH (class), AM | T/B | P3 |
| GET/POST/PATCH | `/lms/learning-paths` | Lộ trình học | TCH, AM | T/B/T | P3 |
| POST | `/lms/assignments` | Giao bài (content_ref, target, open/due, max_attempts, result_visibility) | TCH (class) | B | P3 |
| GET | `/lms/assignments` | Bài đã giao (GV); học sinh dùng `/learning/my-assignments` | TCH | T | P3 |
| GET | `/lms/assignments/{assignment_id}` | Chi tiết | TCH (owner), STU (target) | T | P3 |
| PATCH | `/lms/assignments/{assignment_id}` | Sửa (trước `open_at`; sau đó chỉ gia hạn) — `If-Match` | TCH (owner) | T | P3 |
| GET | `/lms/assignments/{assignment_id}/submissions` | Kết quả từng học sinh | TCH (owner) | T | P3 |
| GET | `/lms/classes/{class_id}/dashboard` | Teacher dashboard: tiến độ, LO yếu, hoạt động; `from`/`to` bắt buộc, phản hồi có `grain`/`data_as_of`/`is_partial` (ADR-0062) — `getClassDashboard` | TCH (class) | T | P3 |
| GET | `/lms/dashboards/teacher?class_id=&subject_id=&from=&to=&page_size=` | Gói widget trang chủ giáo viên trong một lần gọi (rollup, read replica) — `getTeacherDashboard` | TCH (class) | T | P3 |
| GET | `/lms/reviews` | Hàng đợi chấm bài (tự luận, nói, dự án): `?class_id&subject_id&status&due_before` | TCH (được phân công) | T | P3 |
| GET | `/lms/reviews/{review_task_id}` | Bài làm + rubric (version đã ghim) + gợi ý AI nếu tenant bật (ADR-0053) | TCH (được phân công) | T | P3 |
| POST | `/lms/reviews/{review_task_id}:grade` | Chấm theo rubric + nhận xét (nhận xét có công thức gửi kèm SVG, ADR-0055) | TCH (được phân công) | B | P3 |
| POST | `/lms/reviews/{review_task_id}:return` | Trả bài cho học sinh sửa | TCH (được phân công) | B | P3 |
| POST | `/lms/reviews/{review_task_id}:regrade` | Chấm lại (`reason` bắt buộc) → evaluation mới, cũ `superseded` | TCH (được phân công), AM | B | P3 |
| POST | `/lms/project-groups` | Lập nhóm dự án trong lớp | TCH (class) | B | P4 |
| GET | `/lms/classes/{class_id}/analytics` | Analytics theo LO/topic/thời gian | TCH (class), SA | T | P3 |
| GET | `/lms/schools/{school_id}/analytics` | Analytics trường | SA, AM | T | P3 |
| GET | `/lms/schools/{school_id}/analytics/series?metric=&grade_level=&subject_id=&from=&to=&grain=` | Chuỗi thời gian cấp trường (≤ 120 điểm) — `getSchoolAnalyticsSeries` | SA, AM · `analytics.school.read` | T | P3 |
| GET/POST | `/lms/challenges` | Thử thách lớp / giải trường **tạo từ template platform**; chỉ điền `allowed_params` (tham số khác → `422`) — ADR-0027 | TCH (class), SA, AM · `lms.challenge.write` | T/B | P4 |
| GET | `/lms/event-templates` | Template sự kiện đã gán cho tenant. **Route ngoại lệ → `game-service`** | TA, SA, TCH · `lms.school_event.write` | T | P3 |
| GET/POST | `/lms/school-events` | Sự kiện trường tạo từ template; chỉ `allowed_params` (`EVENT_TEMPLATE_PARAM_NOT_ALLOWED`); tên qua kiểm tra từ nhạy cảm ([EVENT_SEASON_SYSTEM §10](../01-game-design/EVENT_SEASON_SYSTEM.md#10-sự-kiện-của-trường-tenant)). **Route ngoại lệ → `game-service`** | TA, SA, TCH (khi trường cho phép) · `lms.school_event.write` | T/B | P3 |
| GET/PATCH | `/lms/school-events/{event_id}` | Xem/sửa trước khi chạy (`If-Match`) | như trên | T | P3 |
| POST | `/lms/school-events/{event_id}:cancel` | Hủy sự kiện trường (lý do) | như trên | B | P3 |
| GET | `/lms/catalog/challenge-templates` | Template thử thách/giải đấu đã publish (chỉ đọc) | TCH, SA, AM | T | P4 |
| GET | `/lms/catalog/game-activities` | Quest/mini-game đã publish để giao bài (chỉ đọc; không cấu hình) | TCH, SA, AM | T | P2 |

**Quy ước dashboard (ADR-0062, [DASHBOARD_DATA §8](DASHBOARD_DATA.md#8-quy-tắc-truy-vấn))** áp cho mọi endpoint dashboard/analytics (`/lms/dashboards/*`, `/lms/classes/{id}/dashboard`, `/lms/*/analytics*`, `/tenant/analytics/*`, `/admin/analytics/*`): `from`, `to` **bắt buộc** (khoảng ≤ 730 ngày → `422 DASHBOARD_RANGE_TOO_LARGE`); `grain` (`day`/`week`/`month`) chỉ là gợi ý, server chọn theo khoảng (≤ 31 ngày → ngày, ≤ 1 năm → tuần, > 1 năm → tháng) và chỉ chấp nhận thô hơn; danh sách chi tiết `page_size` ≤ 100 (cursor); phản hồi luôn có `grain`, `data_as_of`, `is_partial`; `429 DASHBOARD_BUSY`, `504 DASHBOARD_QUERY_TIMEOUT`. Chỉ đọc rollup trên read replica.

Assignment có thể tham chiếu `content_type ∈ {quest, minigame_config}` nhưng chỉ version `published` lấy từ catalog; LMS không có endpoint sửa nội dung game.

### 9.15. SIS (internal) — `/api/v1/sis`

**Service:** `identity-service`.

**Không publish ra public gateway** (ADR-0009). Chỉ qua APISIX internal/VPN quản trị.

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET/POST | `/sis/connections` | Cấu hình kết nối SIS (credential ref, không plaintext) | SYS | T/B | P1 |
| POST | `/sis/connections/{connection_id}/sync-runs` | Kích hoạt full/delta sync | SYS, SA (school), SVC | B | P1 |
| GET | `/sis/connections/{connection_id}/sync-runs` | Lịch sử sync | SYS, SA | T | P1 |
| POST | `/sis/imports` | Upload CSV (`csv_import`, cột theo EduWorld SIS Contract) qua presigned URL | SA, SYS | B | P1 |
| GET | `/sis/reconciliation-issues` | Xung đột cần xử lý | SA, SYS | T | P1 |
| POST | `/sis/reconciliation-issues/{issue_id}:resolve` | Xử lý (link/merge/ignore) | SA, SYS | B | P1 |

Webhook từ SIS **không** nằm trong `/api/v1/sis/*`. Đây là luồng machine-to-machine nên dùng `/internal/sis/webhooks/{connection_id}` (§9.17).

**Contract SIS hướng ra ngoài.** Ngoài API của EduWorld ở trên, EduWorld còn sở hữu [EduWorld SIS Contract v1](../../specs/sis/eduworld-sis-contract.v1.yaml): các endpoint **trường triển khai** để worker của `identity-service` gọi (pull), cùng payload `ChangeBatch` mà SIS đẩy tới webhook nội bộ. Contract này là API riêng, không thuộc `/api/v1`, và có version độc lập (`/eduworld/v1` trên hạ tầng của trường). DTO trong contract do EduWorld định nghĩa; EduWorld không nhận DTO riêng của SIS (ADR-0021, [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md)).

### 9.16. Admin (internal) — `/api/v1/admin`

**Service:** `identity-service` (ngoại lệ ghi ở cột Service).

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET/POST/PATCH | `/admin/tenants` | Quản lý tenant (`tenant_type` school/unit/common, + tạo Keycloak Organization) — list nhận `q` tìm theo mã/tên cho dropdown tenant | SYS | T/B/T | P1 |
| GET/POST/PATCH | `/admin/tenants/{tenant_id}/schools` | Trường thuộc tenant | SYS | T/B/T | P1 |
| GET / PUT | `/admin/sis/connections/{connection_id}/schools` | Danh sách / gán tập trường cho một SIS connection; mỗi trường chỉ một connection hoạt động (`409 SIS_SCHOOL_ALREADY_CONNECTED`, ADR-0046) | SYS | T / B | P1 |
| POST | `/admin/users/{user_id}:lock` / `:unlock` / `:deactivate` | Khóa/mở/vô hiệu hóa tài khoản (phát `user.access_revoked`) | SYS (TA/SA trong tenant qua `/tenant/users`) | B | P1 |
| GET / POST | `/admin/roles?role_scope_kind=&tenant_ids=` | Role platform và role dùng chung (`tenant_id = null`), xem role tùy biến của mọi tenant / tạo role platform hoặc role tenant dùng chung. Role `tenant` chứa permission platform/`game.*`/`competition.config.*` → `422 VALIDATION_FAILED`. | SYS · `platform.role.write` (đọc: `platform.role.read`) | T / B | P1 |
| GET / PATCH / DELETE | `/admin/roles/{role_id}` | Chi tiết / sửa (`If-Match`, bắt buộc `reason`) / soft-delete; `is_system` không sửa/xóa được. Phát `role.updated`. | SYS · `platform.role.write` | T | P1 |
| GET | `/admin/permissions` | Catalog permission đầy đủ (tenant + platform, `scope_kind`). | SYS | T | P1 |
| GET / POST | `/admin/user-roles?user_id=&tenant_ids=` | Xem UserRole liên tenant (không gửi `X-Tenant-Id`) / gán role platform (`scope_type = platform`, không cần header) hoặc role tenant (header `X-Tenant-Id` = tenant đích). Phát `user_role.changed`, audit. | SYS · `platform.user_role.write` | T / B | P1 |
| DELETE | `/admin/user-roles/{user_role_id}` | Thu hồi UserRole (soft-delete, bắt buộc `reason`). | SYS · `platform.user_role.write` | T | P1 |
| POST | `/admin/catalog/countries` · PATCH `/admin/catalog/countries/{code}` | Thêm / bật-tắt, sửa tên quốc gia (`If-Match`) — `createCountry`, `updateCountry` | SYS · `platform.catalog.write` | B / T | P1 |
| POST | `/admin/catalog/provinces` · PATCH `/admin/catalog/provinces/{province_id}` | Tạo đơn vị mới / sửa tên, `official_code`, đặt `inactive` (kèm `effective_to`); phát `province.updated` — `createProvince`, `updateProvince` | SYS · `platform.catalog.write` | B / T | P1 |
| POST | `/admin/catalog/province-successions` | Ghi kế thừa đơn vị cũ → mới (mỗi đơn vị cũ một kế thừa) — `createProvinceSuccession` | SYS · `platform.catalog.write` | B | P1 |
| POST | `/admin/schools:import` | Import CSV trường vào tenant đích (header `X-Tenant-Id` bắt buộc), `dry_run`, bất đồng bộ `202` → `GET /admin/school-imports/{import_id}` — `importSchools`, `getSchoolImport` | SYS · `platform.catalog.write` | B | P1 |
| GET | `/admin/audit-logs?tenant_ids=` | Truy vấn audit (gồm nhật ký thay đổi phân quyền `role.*`, `user_role.*`) | SYS | T | P1 |
| GET | `/admin/login-events?tenant_ids=&student_id=&from=&to=` | **→ `game-service`.** Lịch sử đăng nhập game liên tenant (ADR-0031), dropdown tenant | SYS · `platform.cross_tenant.read` | T | P1 |
| GET/PUT | `/admin/settings/student-cleanup` | Cấu hình platform `inactive_student_cleanup_days` (mặc định 180 [Proposed]), `notice_days`, `allow_tenant_override`, ngưỡng cảnh báo % (ADR-0032) — `If-Match`, bắt buộc `reason` | SYS · `platform.settings.write` | T | P2 |
| GET | `/admin/student-cleanup/runs?tenant_ids=` | Lịch sử các lần chạy worker dọn (số dọn, bị loại trừ, bị chặn do vượt ngưỡng) | SYS | T | P2 |
| GET/PUT | `/admin/settings/chat-retention` | Thời hạn lưu chat gameplay `gameplay_chat_retention_days` (mặc định 7, tối thiểu 3 → `422 RETENTION_BELOW_MINIMUM`), `ephemeral_retention_hours` (24), `allow_tenant_override` — bắt buộc `reason`, audit (ADR-0033) | SYS · `platform.settings.write` | T | P4 |
| POST | `/admin/student-cleanup/runs/{run_id}:approve` | Xác nhận thủ công lần dọn vượt ngưỡng > 10% học sinh tenant [Proposed] | SYS | B | P2 |
| GET/PUT | `/admin/feature-flags/{flag_key}` | Feature flag | SYS | T | P1 |
| POST | `/admin/data-requests` | Yêu cầu xuất/ẩn danh dữ liệu học sinh (privacy, ADR-0028 §8) | SYS | B | P6 |
| GET | `/admin/trash?resource_type=&tenant_ids=` | **Thùng rác**: bản ghi đã soft-delete (mọi tenant + platform), có `deleted_at`, `deleted_by`, `delete_reason` (ADR-0028) | SYS · `platform.trash.read` | T | P2 |
| POST | `/admin/trash/{resource_type}/{id}:restore` | Khôi phục (kiểm tra unique → `409`) | SYS · `platform.trash.restore` | B | P2 |
| POST | `/admin/tenant-assignments` | Gán dữ liệu platform cho tenant: `{resource_type, resource_id, tenant_ids[], apply_to_new_tenants}` (ADR-0029) | SYS · `platform.tenant_data.assign` | B | P2 |
| DELETE | `/admin/tenant-assignments` | Gỡ gán (soft-delete assignment): `{resource_type, resource_id, tenant_ids[]}` | SYS · `platform.tenant_data.assign` | B | P2 |
| GET | `/admin/tenant-assignments?resource_type=&resource_id=` | Tenant đang được gán | SYS | T | P2 |
| POST | `/admin/tenant-clones` | Clone `{source_tenant_id, target_tenant_id, resource_type, resource_ids[]}` → job bất đồng bộ (`202`); không clone dữ liệu học sinh | SYS · `platform.tenant_data.clone` | B | P3 |
| GET | `/admin/tenant-clones/{job_id}` | Trạng thái job clone (số bản ghi, ánh xạ ID, lỗi) | SYS | T | P3 |

**Tenant self-service — `/api/v1/tenant`** (public gateway, Admin Portal workspace C, tenant của phiên):

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET/PATCH | `/tenant/settings` | Cài đặt tenant (khung giờ, chat policy tenant, thông báo, `inactive_student_cleanup_days` khi platform cho phép ghi đè — ADR-0032; `gameplay_chat_retention_days` ≥ 3 khi được phép — ADR-0033) — `If-Match` | TA (`tenant.settings.write`; cleanup cần `tenant.student_cleanup.configure`) | T | P3 |
| GET | `/tenant/students/{student_id}/login-events?from=&to=` | **→ `game-service`.** Lịch sử đăng nhập game của học sinh (thời điểm, nền tảng, thiết bị rút gọn, lý do kết thúc; không có IP) (ADR-0031) | TA, SA (trường), TCH (lớp) · `lms.student_activity.read` | T | P1 |
| GET | `/tenant/student-cleanup/preview` | Danh sách học sinh **sắp bị dọn** (last login, ngày dự kiến dọn) (ADR-0032) | TA, SA · `tenant.student_cleanup.read` | T | P2 |
| POST / DELETE | `/tenant/student-cleanup/exclusions` | Loại trừ học sinh khỏi đợt dọn hoặc gia hạn `{student_ids[], until?, reason}` / gỡ loại trừ | TA, SA · `tenant.student_cleanup.exclude` | B | P2 |
| GET | `/tenant/schools?status=` | Trường trong tenant (tenant `unit` có nhiều trường) — `listTenantSchools` | TA, SA · `tenant.school.read` | T | P1 |
| GET / PATCH | `/tenant/schools/{school_id}` | Xem / sửa thông tin hiển thị (tên song ngữ, tỉnh `active`, `moet_code`, cấp học, khối, địa chỉ; `If-Match`). Trường SIS chỉ sửa phần hiển thị, dữ liệu nguồn ghi đè ở lần đồng bộ sau (ADR-0061) — `getTenantSchool`, `updateTenantSchool` | TA, SA · `tenant.school.write` | T | P1 |
| POST | `/tenant/schools/{school_id}:approve` | Duyệt trường `pending_review` → `active` — `approveTenantSchool` | TA · `tenant.school.write` | T | P1 |
| GET | `/tenant/schools/{school_id}/classes?academic_year=&grade=` | Lớp và sĩ số — `listTenantSchoolClasses` | TA, SA · `tenant.class.read` | T | P1 |
| GET | `/tenant/students/unassigned?school_id=` | Học sinh SIS chưa có lớp hợp lệ (`missing_class`; học sinh phải thuộc đúng một trường và một lớp — ADR-0060) — `listUnassignedStudents` | TA, SA · `tenant.class.read` | T | P1 |
| GET | `/tenant/users?q=&user_type=&school_id=&role_code=` | Người dùng có membership trong tenant (nhân sự + học sinh), kèm UserRole hiệu lực. — `listTenantUsers` | TA, SA (trường) · `tenant.user.read` | T | P3 |
| POST | `/tenant/users` | Tạo tài khoản nhân sự thủ công (ngoài SIS) trong tenant của phiên: username, email **bắt buộc** với nhân sự, tên, ít nhất một UserRole ban đầu. Một người một tài khoản SSO (ADR-0069): username/email đã thuộc tài khoản khác (trong tenant hoặc trên SSO) → `409 USERNAME_TAKEN` / `409 EMAIL_TAKEN`; nếu là **cùng một người** đã có tài khoản ở tenant khác thì thêm membership/UserRole qua `POST /tenant/user-roles` thay vì tạo tài khoản mới — `createTenantUser` | TA · `tenant.user.write` | B | P3 |
| PATCH | `/tenant/users/{user_id}` | Sửa thông tin nhân sự do tenant quản lý (`If-Match`); đổi username/email kiểm tra trùng trên SSO (ADR-0069) — `updateTenantUser` | TA · `tenant.user.write` | T | P3 |
| GET | `/tenant/permissions` | Catalog permission **tenant** (định nghĩa trong code, nhóm theo workspace, tên song ngữ) để dựng màn tạo role; không trả permission platform. — `listTenantPermissions` | TA · `tenant.role.read` | T | P3 |
| GET / POST | `/tenant/roles` | Danh sách role dùng được trong tenant (role hệ thống `is_system` chỉ đọc + role tùy biến của tenant) / tạo role tùy biến [Proposed] từ permission tenant; chứa permission platform/`game.*`/`competition.config.*` → `422 VALIDATION_FAILED` (`errors[].code = PERMISSION_NOT_ALLOWED_FOR_SCOPE`). — `listTenantRoles`, `createTenantRole` | TA · `tenant.role.read` / `tenant.role.write` | T / B | P3 |
| GET / PATCH / DELETE | `/tenant/roles/{role_id}` | Chi tiết / sửa (`If-Match`) / soft-delete role tùy biến; role `is_system` → `403 PERMISSION_DENIED`; xóa role còn UserRole hiệu lực → `409 CONFLICT`. Phát `role.updated`. — `getTenantRole`, `updateTenantRole`, `deleteTenantRole` | TA · `tenant.role.write` | T | P3 |
| GET / POST | `/tenant/user-roles?user_id=&role_id=&scope_type=&scope_id=` | Danh sách / gán UserRole trong tenant: `{user_id, role_id, scope_type (tenant/school/class/subject), scope_id, valid_from, valid_to}`; `source = manual`, `granted_by` = người gọi. Không gán role platform, không gán role portal cho học sinh (`422`). Phát `user_role.changed`. — `listTenantUserRoles`, `grantTenantUserRole` | TA · `tenant.user_role.write`; SA trong trường | T / B | P3 |
| DELETE | `/tenant/user-roles/{user_role_id}` | Thu hồi (soft-delete) UserRole; UserRole `source = sis` chỉ thu hồi được khi bật ghi đè thủ công, nếu không → `409 CONFLICT` [Proposed]. Phát `user_role.changed` — `revokeTenantUserRole` | TA · `tenant.user_role.write` | T | P3 |
| GET | `/tenant/trash?resource_type=` | Thùng rác của tenant [Proposed] | TA · `tenant.trash.read` | T | P3 |
| POST | `/tenant/trash/{resource_type}/{id}:restore` | Khôi phục trong tenant [Proposed] | TA · `tenant.trash.restore` | B | P3 |

Game Analytics (game-service, ADR-0058, [GAME_ANALYTICS §11](../11-operations/GAME_ANALYTICS.md#11-api)); operationId lần lượt `getGameMetrics`, `getGameRetention`, `getGameFunnel`, `getGameContentStats`, `getGamePerformance`, `createGameAnalyticsExport`, `getTenantGameSummary`:

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/admin/analytics/game/metrics?metric_codes=&from=&to=&grain=&tenant_ids=&school_ids=&grade_bands=&platforms=&locales=&device_tiers=&client_versions=` | **→ `game-service`.** Chuỗi thời gian chỉ số Game Analytics | SYS, live-ops, game designer · `game.analytics.read` | T | P4 |
| GET | `/admin/analytics/game/retention?cohort_from=&cohort_to=&…` | **→ `game-service`.** Ma trận cohort retention | `game.analytics.read` | T | P4 |
| GET | `/admin/analytics/game/funnels/{funnel_code}?…` | **→ `game-service`.** Funnel onboarding/quest | `game.analytics.read` | T | P4 |
| GET | `/admin/analytics/game/content-stats?kind=quest\|challenge\|boss&sort=&…` | **→ `game-service`.** Thống kê theo nội dung | `game.analytics.read` | T | P4 |
| GET | `/admin/analytics/game/performance?…` | **→ `game-service`.** Hiệu năng thực tế theo nền tảng/hạng thiết bị/preset/phiên bản | `game.analytics.read` | T | P4 |
| POST | `/admin/analytics/game/exports` | **→ `game-service`.** Xuất CSV bất đồng bộ (`202`) | `game.analytics.read` | B | P4 |
| GET | `/tenant/analytics/game/summary?from=&to=&school_ids=&grade_bands=` | **→ `game-service`.** Báo cáo tổng hợp tenant (ẩn ô < 5) — gateway public | TA · `analytics.tenant.read`; SA · `analytics.school.read` | T | P4 |

Dashboard học tập & báo cáo (learning-service, ADR-0062, [DASHBOARD_DATA §14](DASHBOARD_DATA.md#14-api-đề-xuất)):

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| GET | `/tenant/analytics/learning/summary?school_ids=&grade_level=&from=&to=&grain=` | **→ `learning-service`.** Tổng hợp học tập cấp tenant (ẩn ô < 5) — `getTenantLearningSummary` | TA · `analytics.tenant.read`; SA · `analytics.school.read` | T | P3 |
| POST | `/tenant/reports` | **→ `learning-service`.** Export báo cáo tenant bất đồng bộ (`202`, URL ký 15 phút, audit `report.exported`; vượt giới hạn → `429 EXPORT_LIMIT_REACHED`) — `createTenantReport` | TA, SA · `report.export` | B | P3 |
| GET | `/tenant/reports/{report_id}` | **→ `learning-service`.** Trạng thái + link ký (chỉ người tạo) — `getTenantReport` | Người tạo | T | P3 |
| GET | `/admin/analytics/learning/summary?tenant_ids=&from=&to=&grain=` | **→ `learning-service`.** Tổng hợp liên tenant (không gửi `X-Tenant-Id`) — `getPlatformLearningSummary` | SYS · `platform.cross_tenant.read` | T | P4 |
| POST | `/admin/reports` · GET `/admin/reports/{report_id}` | **→ `learning-service`.** Export liên tenant — `createPlatformReport`, `getPlatformReport` | SYS · `report.export` | B / T | P4 |

Kinh tế game (game-service, ADR-0049):

| Method | Path | Mô tả | Role | Idem | Phase |
|---|---|---|---|---|---|
| POST | `/admin/economy/adjustments` | **→ `game-service`.** Bút toán bù Xu/vật phẩm/XP, lý do bắt buộc, trạng thái `pending_approval` | SYS · `game.economy.adjust` | B | P2 |
| POST | `/admin/economy/adjustments/{adjustment_id}:approve` | **→ `game-service`.** Người thứ hai (khác người tạo) duyệt → ghi sổ cái | SYS · `game.economy.adjust` | B | P2 |

### 9.17. Route nội bộ — tiền tố và luồng machine-to-machine

**Quy ước tiền tố** [Proposed]:

| Tiền tố | Dành cho | Gateway | Xác thực |
|---|---|---|---|
| `/api/v1/game-admin/*` | **Người** vận hành platform (Game Studio, cấu hình thi đấu) — ADR-0027 | **Chỉ APISIX internal** (VPN/admin CIDR) | Bearer token Keycloak `azp = eduworld-portal` + MFA (`acr`) + role platform |
| `/api/v1/admin/*`, `/api/v1/sis/*` | **Người** quản trị (System Admin, School Admin có quyền SIS) dùng Admin Portal | **Chỉ APISIX internal** (VPN/admin CIDR) | Bearer token Keycloak (`azp = eduworld-portal`, MFA với role platform) + authorization trong Go |
| `/internal/v1/*` | **Service ↔ service** (ADR-0040), xem §12 | **Không qua APISIX**; chỉ mạng private (Service DNS) | Service token Keycloak (client credentials `svc-*`), `aud = svc-<callee>-api`, allowlist `azp` |
| `/internal/sis/*`, `/internal/auth/*` | **Hệ thống ngoài → EduWorld**: SIS webhook, Keycloak backchannel logout | APISIX internal/partner (IP allowlist) | HMAC/mTLS (SIS), chữ ký logout token (Keycloak) |
| `/api/v1/schools/{school_id}/sis-imports` | Ngoại lệ public có kiểm soát (§9.12) | Public | Bearer + role `school_admin` scope trường |

Luồng từ hệ thống ngoài (đều do `identity-service` phục vụ):

| Method | Path | Mô tả | Gọi bởi | Idem | Phase |
|---|---|---|---|---|---|
| POST | `/internal/sis/webhooks/{connection_id}` | Webhook delta từ SIS (ký HMAC hoặc mTLS, chống replay bằng timestamp + `delivery_id`) → job `IDENTITY_SIS_JOBS` | SIS | B (`delivery_id`) | P1+ |
| POST | `/internal/auth/backchannel-logout` | Keycloak backchannel logout → phát `user.access_revoked`; game-service đóng game session, gateway cắt WS | Keycloak | T | P1 |

Health/readiness của từng service (`/livez`, `/readyz`, `/metrics`) nằm trên cổng quản trị riêng, không thuộc `/internal/v1` (xem [DEPLOYMENT](../11-operations/DEPLOYMENT.md)).

### 9.18. Realtime (WebSocket)

**Service:** `realtime-gateway` (chỉ giữ kết nối; định tuyến lệnh tới service bằng NATS request/reply `ew.cmd.<service>.<domain>.<action>`, input tần suất cao qua NATS core `ew.in.room.<room_id>`, sự kiện xuống qua Valkey Pub/Sub — ADR-0036).

| Path | Mô tả | Xác thực |
|---|---|---|
| `GET /realtime/v1?ticket=...` (Upgrade) | Kết nối WS duy nhất cho mọi kênh realtime | Ticket 1 lần do `game-service` cấp (ADR-0008) |

Message types: [AsyncAPI](../../specs/asyncapi) và [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md).

## 10. APISIX route mapping

Upstream là service Docker Swarm (staging/production) hoặc Compose service (local) của từng microservice (ADR-0024, ADR-0036), phân giải bằng DNS nội bộ (`tasks.<service>` để APISIX health check từng replica). Plugin chung cho mọi route public có token: `openid-connect` (bearer_only, JWKS, `aud = eduworld-api`), `request-id`, `opentelemetry`, `prometheus`, `cors` theo origin. **Kiểm tra header tenant (ADR-0065):** plugin `tenant-header` [Proposed: plugin Lua nhỏ hoặc `serverless-pre-function`] gắn ở mọi route tenant-scoped — thiếu `X-Tenant-Id` → `400 TENANT_HEADER_REQUIRED`, sai định dạng UUID → `400 BAD_REQUEST`; giá trị dùng làm khóa `limit-count` theo tenant và gắn vào log/trace (`tenant.id`). Route **không** gắn plugin: `/auth/game/login`, `/auth/game/tenant:select`, `/auth/session`, `/me/tenants`, `/game/client-config`, `/realtime/v1` (tenant cố định trong ticket); route `/catalog/countries`, `/catalog/provinces`, `/cms/*`, `/me/permissions` và `/admin/*` gắn ở chế độ "tùy chọn" (System Admin đọc liên tenant không gửi header; ghi dữ liệu tenant thì service tự đòi header). APISIX chỉ kiểm tra có mặt và định dạng; việc khớp tenant do service kiểm tra (§11).

| Route (uri) | Gateway | Plugins bổ sung | Upstream | Ghi chú |
|---|---|---|---|---|
| `/api/v1/auth/game/login`, `/api/v1/auth/game/tenant:select` | public | **không** `openid-connect`, **không** `tenant-header`; `limit-count` 10/phút theo `X-Device-Id` + 10/phút theo username (plugin đọc field, không log), hàng chờ toàn cục → `429 LOGIN_QUEUE_BUSY`; body 4 KB; **tắt ghi body** ở logger và `opentelemetry` (không thu `request_body`), chỉ HTTPS | `identity-service` | ADR-0068: mật khẩu đi qua route này; không log, không trace body |
| `/api/v1/auth/game/token:refresh`, `/api/v1/auth/game/logout` | public | không `openid-connect` (refresh token trong body, service kiểm tra); `limit-count` 20/phút theo thiết bị; tắt ghi body | `identity-service` | Refresh token không vào log |
| `/api/v1/catalog/*` | public + internal (Admin Portal) | `limit-count` 60/phút; `tenant-header` tùy chọn (`countries`, `provinces`), bắt buộc (`schools`); cache `ETag` | `identity-service` | Danh mục ADR-0060 |
| `/api/v1/auth/*` (còn lại), `/api/v1/me/*`, `/api/v1/students/me` | public | `limit-count` 20/phút (IP+sub); `tenant-header` trừ `/auth/session`, `/me/tenants` | `identity-service` | |
| `/api/v1/{schools,tenant}/*` (trừ ngoại lệ dưới) | public + internal (Admin Portal) | `limit-count`, body 1 MB, chấp nhận azp `eduworld-portal`, `tenant-header` | `identity-service` | |
| `/api/v1/schools/*/sis-imports*` | public | `limit-count` 10/giờ, body 16 KB | `identity-service` | File đi qua presigned URL |
| `/api/v1/tenant/students/*/login-events` | public | như `/tenant/*` | `game-service` | Ưu tiên route cụ thể hơn `/tenant/*` |
| `/api/v1/tenant/chat/*` | public | như `/tenant/*` | `chat-service` | |
| `/api/v1/{learning,exams}/*` | public | `limit-count` (§8), body 256 KB | `learning-service` | |
| `/api/v1/{cms/learning,cms/media,lms}/*` (trừ ngoại lệ dưới) | public + internal (Admin Portal) | body 1 MB, azp `eduworld-portal` | `learning-service` | |
| `/api/v1/lms/event-templates`, `/api/v1/lms/school-events*` | public + internal (Admin Portal) | như `/lms/*` | `game-service` | Route ngoại lệ (sự kiện trường); ưu tiên route cụ thể hơn `/lms/*` |
| `/api/v1/tenant/analytics/game/*` | public | như `/tenant/*` | `game-service` | Báo cáo game tổng hợp của tenant |
| `/api/v1/tenant/analytics/learning/*`, `/api/v1/tenant/reports*` | public | như `/tenant/*`; `limit-count` 30/phút/người dùng | `learning-service` | Dashboard học tập, export (ADR-0062) |
| `/api/v1/game/*` | public | `limit-count` (§8), body 256 KB; `/game/client-config` **không** `openid-connect`; `/game/telemetry` 30/phút, body 32 KB | `game-service` | |
| `/api/v1/{multiplayer,competition,leaderboards}/*` | public | `limit-req` burst, body 256 KB | `competition-service` | |
| `/api/v1/{chat,social,notifications}/*` | public | body 64 KB (chat) | `chat-service` | Rate limit nghiệp vụ trong Go |
| `/api/v1/game-admin/(quests\|minigame-configs\|npc-dialogues\|world-areas\|story-chapters\|events\|event-templates\|season-tracks\|items\|reward-rules\|levels\|progression-curves\|challenges\|bosses\|achievements\|collections\|shop-items\|currencies)(/.*)?`, `/api/v1/game-admin/preview/*`, `/api/v1/game-admin/events/*`, `/api/v1/game-admin/translations*`, `/api/v1/game-media(:presign-upload|/.*)` | **internal** | `openid-connect` azp `eduworld-portal` + `acr` MFA, `ip-restriction` (VPN/admin CIDR) | `game-service` | Route theo regex của `config_type` (`x-service-routing`) |
| `/api/v1/game-admin/(matchmaking-configs\|ranking-rules\|seasons\|tournament-templates\|challenge-templates\|leaderboard-definitions)(/.*)?` | **internal** | như trên | `competition-service` | |
| `/api/v1/game-admin/(sticker-packs\|gifs)(/.*)?` | **internal** | như trên | `chat-service` | |
| `/api/v1/admin/login-events` | **internal** | azp `eduworld-portal` + MFA, `ip-restriction` | `game-service` | |
| `/api/v1/admin/chat/*` | **internal** | như trên | `chat-service` | |
| `/api/v1/admin/economy/*` | **internal** | như trên | `game-service` | Bút toán bù kinh tế game (ADR-0049) |
| `/api/v1/admin/analytics/game/*` | **internal** | như trên | `game-service` | Game Analytics (ADR-0058) |
| `/api/v1/admin/analytics/learning/*`, `/api/v1/admin/reports*` | **internal** | như trên | `learning-service` | Dashboard học tập liên tenant, export (ADR-0062) |
| `/api/v1/{admin,sis}/*` (còn lại) | **internal** | như trên | `identity-service` | Không tồn tại trên public gateway |
| `/realtime/v1` | public | `enable_websocket: true`, `limit-conn` (per IP); **không** `openid-connect` (ticket kiểm trong gateway) | `realtime-gateway` | Idle timeout 75 s, ping 25 s |
| `/internal/sis/webhooks/*` | internal/partner | `hmac-auth` hoặc mTLS, `ip-restriction` (IP SIS), `limit-count` | `identity-service` | Machine-to-machine |
| `/internal/auth/backchannel-logout` | internal | `ip-restriction` (mạng Keycloak) | `identity-service` | |
| `/internal/v1/*` | **không route** | — | — | Chỉ gọi trực tiếp trong mạng private (§12) |
| `/livez`, `/readyz`, `/metrics` | **không route** | — | — | Cổng quản trị riêng của từng service |

Cấu hình declarative (`deploy/apisix/apisix.yaml` local, ADC cho staging/prod) — xem [DEPLOYMENT](../11-operations/DEPLOYMENT.md). APISIX **không** quyết định quyền nghiệp vụ. Bảng route phải khớp §2.1 và `x-service` (kiểm tra trong CI bằng script so route ↔ spec [Proposed]).

## 11. Authorization per route — quy tắc chung

0. Middleware `tenant` (ADR-0065): đọc `X-Tenant-Id` (thiếu ở route tenant-scoped → `400 TENANT_HEADER_REQUIRED`), so với tenant của phiên (claim `organization` nếu có + membership DB qua `memberships:check`, cache 60 s); không khớp → `403 TENANT_MISMATCH` + audit; `Principal.tenant_id` lấy từ kết quả kiểm tra. Token game không có claim `organization` (ADR-0068): header phải là membership hợp lệ của học sinh và trùng tenant của game session. Route portal (`/cms/*`, `/lms/*`, `/tenant/*`, `/schools/*`, `/game-admin/*`, `/admin/*`): `azp = eduworld-portal` và `user_type ≠ student`, nếu không → `403 PORTAL_ACCESS_STAFF_ONLY` + audit (ADR-0066).
1. Middleware `auth` → `Principal` = hợp permission của mọi UserRole còn hiệu lực trong tenant đã kiểm tra và phạm vi tài nguyên (ADR-0066) (resolve qua `identity-service` `POST /internal/v1/principals:resolve`, cache 60 s; user không active → `ACCOUNT_LOCKED`/`ACCOUNT_DEACTIVATED`). identity-service lỗi → `503 UNAVAILABLE`, **không fail-open**.
1b. Middleware `game_player` cho mọi route người chơi (`/game/*` trừ `client-config`, `/multiplayer/*`, `/competition/*` phía người chơi, `/learning/activities/*`, `/learning/activity-attempts/*` và `/exams/*` từ game, realtime ticket): `azp = eduworld-game-backend` (token cấp qua `/auth/game/login`, ADR-0068) **và** `Principal.user_type = student`, nếu không → `403 GAME_ACCESS_STUDENT_ONLY` + audit (ADR-0031). Token game gọi API vận hành (`/tenant/*`, `/cms/*`, `/lms/*`, `/admin/*`, `/game-admin/*`) → `PERMISSION_DENIED`.
2. Handler gọi use case (CQRS command/query, ADR-0038); pipeline mediator gọi `p.ScopeFor(permission)` → `TenantScope`.
3. Tài nguyên theo ID được load **qua repository có scope**; ngoài scope = `404 NOT_FOUND`.
4. Kiểm tra quan hệ tinh (owner, member, leader, class assignment) trong domain/application.
5. Hành động quản trị, kiểm duyệt, thay đổi quyền, xem dữ liệu học sinh hàng loạt phát event `audit.recorded` qua outbox; `identity-service` tiêu thụ và ghi `audit.audit_logs` tập trung (ADR-0042).
6. Permission platform-only (`game.*`, `competition.config.*`, `platform.*`) chỉ hợp lệ khi token có `azp = eduworld-portal` + MFA (`acr`; thiếu → `403 MFA_REQUIRED`) và Principal có role `role_scope_kind = platform`. Role tenant không bao giờ chứa các permission này (catalog kiểm tra lúc khởi động) — vi phạm trả `403 PERMISSION_DENIED`, ghi audit `authz.denied` (ADR-0027, [ADMIN_PORTAL §2](../04-admin-portal/ADMIN_PORTAL.md#2-permission-model)).
7. Gọi internal (§12): bên nhận kiểm tra service token (`aud`, scope `internal-<callee>`, `azp` ∈ `x-allowed-callers`) **và** vẫn áp tenant scope theo `X-Tenant-Id` + quyền nghiệp vụ của `X-On-Behalf-Of` khi thao tác thay người dùng.

Ma trận role × permission đầy đủ: [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md). Test sinh từ catalog §9 ([BACKEND_DESIGN §15](BACKEND_DESIGN.md#15-kiểm-thử-backend)).

## 12. Internal API giữa service (ADR-0036, ADR-0040)

### 12.1. Quy ước

| Hạng mục | Quy ước |
|---|---|
| Spec | `specs/openapi/services/<service>.internal.v1.yaml` (contract-first, sửa tay). Interface platform ops trong `common/internal-common.v1.yaml`, đồng bộ bằng `scripts/openapi/sync-platform-ops.mjs` |
| Path | `/internal/v1/...`, cùng quy ước đặt tên, Problem Details, cursor pagination và Idempotency-Key như API public |
| Mạng | Chỉ mạng private (Service DNS `http://<service>:8080`), không qua APISIX; mTLS tùy chọn [Proposed] |
| Xác thực | Token Keycloak client credentials của client `svc-identity`, `svc-learning`, `svc-game`, `svc-competition`, `svc-chat`, `svc-realtime`; `aud = svc-<callee>-api`, scope `internal-<callee>`; bên gọi cache token đến gần hết hạn |
| Phân quyền | Mỗi operation có `x-allowed-callers: [svc-...]`; `azp` ngoài danh sách → `403 CALLER_NOT_ALLOWED` |
| Header | `traceparent` (bắt buộc truyền tiếp), `X-Request-Id`, `X-Tenant-Id` (tenant đang xử lý; bên nhận vẫn tự kiểm tra), `X-On-Behalf-Of` (user gốc, để audit/quyền), `Idempotency-Key` (bắt buộc cho POST thay đổi trạng thái) |
| Client | **Typed client sinh bằng `oapi-codegen`** từ spec internal — tương đương Refit: khai báo bằng spec, gọi như interface Go. Bọc trong adapter tầng infrastructure, expose qua port tầng application (ADR-0038) |
| Lỗi | Dùng chung catalog §4.1; lỗi mạng/timeout/circuit mở của phụ thuộc → bên gọi trả `503 UNAVAILABLE` cho client, **không fail-open** |

### 12.2. Bản đồ gọi đồng bộ

| Bên gọi → bên nhận | Endpoint | Mục đích | Idem |
|---|---|---|---|
| mọi service → identity | `POST /internal/v1/principals:resolve` (**chuẩn**; trả thêm `sis_connection_id`, `student_external_id` khi có — ADR-0046; `GET /internal/v1/principals/{user_id}?tenant_id=` là alias deprecated) | Role, permission, scope của user trong tenant; cache 60 s, vô hiệu khi nhận `user_role.changed`/`role.updated`/`tenant_membership.changed`/`user.access_revoked` | T |
| mọi service → identity | `GET /internal/v1/tenants/{tenant_id}` | Thông tin tenant, trường, cài đặt hiệu lực | T |
| mọi service → identity | `POST /internal/v1/memberships:check` | Kiểm tra hàng loạt (≤ 100 user) thuộc tenant/trường/lớp | T |
| ~~Keycloak authenticator tùy biến (`svc-keycloak-authn`) → identity~~ | ~~`POST /internal/v1/login-credentials:verify`~~ | **Mở rộng sau (ADR-0063)** — đăng nhập nhanh (mã lớp + PIN, thẻ QR, mật khẩu hình ảnh, ADR-0045) không có trong Release 1.0; đã gỡ khỏi `identity.internal.v1.yaml` | — |
| game → learning | `POST /internal/v1/attempts` | Mở attempt cho bước quest/mini-game (`mode` = `quest` hoặc `minigame`) | B |
| game → learning | `GET /internal/v1/attempts/{attempt_id}` | Đối soát kết quả attempt (không có đáp án) | T |
| competition → learning | `POST /internal/v1/question-plans` | Bộ câu hỏi cho trận (render payload, không đáp án) | B |
| competition → learning | `POST /internal/v1/question-plans/{plan_id}/answers:score` | Chấm câu trả lời trong trận (server-authoritative), ghi evidence `competition` | B |
| identity → learning, game, competition, chat | `POST /internal/v1/platform/tenant-clones/{job_id}/steps` | Bước clone của saga clone tenant (ADR-0029) | B |
| identity → mỗi service | `GET /internal/v1/platform/trash`, `POST /internal/v1/platform/trash/{resource_type}/{resource_id}:restore` | Gộp Thùng rác và khôi phục (ADR-0028) | T / B |
| identity → mỗi service | `POST /internal/v1/platform/students/{student_id}:cleanup` / `:restore` | Saga dọn/khôi phục học sinh (ADR-0032); service trả `blocked` khi có hoạt động đang chạy | B |
| identity → mỗi service | `POST /internal/v1/platform/users/{user_id}:merge-into` / `:anonymize` | Saga gộp tài khoản trùng / ẩn danh hóa theo yêu cầu xóa (ADR-0028 mục 8) | B |

Phản ứng phụ không cần kết quả ngay (cộng XP, cập nhật quest, mở/đóng channel chat, projection `ref_*`, audit) **không** gọi đồng bộ mà dùng domain event trên NATS JetStream ([EVENT_DESIGN](EVENT_DESIGN.md), [specs/events](../../specs/events/README.md)).

### 12.3. Resilience (mặc định [Proposed])

| Thuộc tính | Giá trị | Ghi chú |
|---|---|---|
| Timeout | 800 ms cho đọc (`principals:resolve`, `tenants`, `memberships:check`, `GET attempts`); 2 s cho ghi; 5 s cho bước saga | Theo endpoint, cấu hình trong adapter |
| Retry | Tối đa 2 lần, backoff 100 ms × 2ⁿ + jitter; **chỉ** cho GET hoặc POST có `Idempotency-Key` | Không retry 4xx (trừ 429 theo `Retry-After`) |
| Circuit breaker | Mở khi ≥ 50% lỗi trong 20 request gần nhất; nửa mở sau 10 s | Mỗi cặp (bên gọi, endpoint) |
| Bulkhead | Giới hạn concurrency 50 request/phụ thuộc/instance | Tránh lan lỗi |
| Fallback | Chỉ dùng cache còn hạn cho dữ liệu đọc không nhạy cảm (tenant info); **không** fallback cho quyền | Principal hết hạn cache + identity lỗi → 503 |

Thư viện: `failsafe-go` [Proposed]. Metric: `eduworld_internal_client_requests_total{caller,callee,operation,outcome}`, `eduworld_internal_client_duration_seconds`, `eduworld_internal_client_circuit_state`.

## 13. Giới hạn độ dài chuỗi và validation (ADR-0041)

1. Mọi `type: string` trong OpenAPI (public, theo service, internal), AsyncAPI, SIS contract, JSON Schema (`specs/schemas`, `specs/events`) có `maxLength`. Ngoại lệ: `enum`, `const`, `format` ∈ {uuid, date, date-time, time, duration, binary, byte}. Bảng độ dài chuẩn ở ADR-0041 (mã 64, tên ngắn 128, tên/tiêu đề 255, email 320, URL 2048, khóa lưu trữ/cursor/token 512, mô tả/tin nhắn 1.000, nội dung dài 20.000).
2. Handler sinh từ OpenAPI validate request **trước** khi vào application. Vượt giới hạn → `422 VALIDATION_FAILED`:

```json
{
  "type": "https://docs.eduworld.vn/errors/validation-failed",
  "title": "Dữ liệu không hợp lệ",
  "status": 422,
  "code": "VALIDATION_FAILED",
  "errors": [ { "field": "body.nickname", "code": "MAX_LENGTH", "message": "Tối đa 128 ký tự" } ]
}
```

3. Độ dài tính theo **ký tự Unicode** (code point) sau chuẩn hóa NFC, khớp `varchar(n)` của PostgreSQL. Cột DB dùng cùng giá trị với spec (ent `MaxLen(n)`, ADR-0039).
4. Godot và CMS đặt giới hạn ô nhập theo cùng giá trị (lấy từ spec khi sinh client).
5. CI chặn merge khi thiếu `maxLength`: Spectral rule `string-must-have-maxlength` (`specs/.spectral.yaml`) cho OpenAPI và `scripts/specs/check-maxlength.mjs` cho mọi file spec.

## 14. Phủ luồng chơi end-to-end

Bảng kiểm tra spec phủ đủ luồng chơi xương sống của [SYSTEM_ARCHITECTURE §5](../06-architecture/SYSTEM_ARCHITECTURE.md) (game-first). REST = `operationId` trong [OpenAPI](../../specs/openapi/eduworld-api.v1.yaml); WS = message `type` trong [AsyncAPI](../../specs/asyncapi/eduworld-realtime.v1.yaml). Rà lần cuối 2026-11-15 sau khi hợp nhất attempt (ADR-0059), thêm song ngữ (ADR-0056) và sự kiện/mùa: **mọi bước đều có contract**; bước/contract bổ sung trong lượt này được đánh dấu 🆕. Mọi `operationId` dưới đây tồn tại trong spec gộp (kiểm bằng script khi rà).

| # | Bước người chơi | Service | REST (`operationId`) | WS (message) | Tài liệu nghiệp vụ |
|---|---|---|---|---|---|
| 1 | Kiểm tra phiên bản client, mở form đăng nhập trong game (không chọn tenant trước) | game | `getClientConfig` | — | [STUDENT_GAME_UX_CORE](../05-ux/STUDENT_GAME_UX_CORE.md) |
| 2 | Đăng nhập bằng form trong game qua backend (ADR-0068); nhiều tenant → chọn tenant bằng `login_ticket`; làm mới token; thoát | identity | `gameLogin` (xác thực SSO trước, rồi quyết định tenant — ADR-0063, ADR-0069), `selectGameLoginTenant`, `refreshGameToken`, `gameLogout`, `getMe` | — | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |
| 3 | Bootstrap game, chọn ngôn ngữ, tải manifest | game | `getGameProfile`, 🆕 `updateGameProfile` (`preferred_language`), `getContentManifest`, `getWorldManifest` | — | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| 4 | Tạo game session (một thiết bị), lấy ticket, mở WS | game, realtime-gateway | `createGameSession`, `heartbeatGameSession`, `createRealtimeTicket` | `system.session.welcome`, `session.session.replaced` | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| 5 | Chọn nickname, tạo avatar lần đầu / đổi trang phục | game | 🆕 `suggestNicknames`, `updateGameProfile`, `listAvatarPresets`, `updateAvatar` | 🆕 `hub.player.appearance_changed` | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| 6 | Vào hub/khu, thấy người chơi khác | game | `listWorldAreas`, `enterWorldArea` | `hub.instance.join`, `hub.instance.assigned`, `hub.player.move`, `hub.snapshot.updated` | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| 7 | Nói chuyện NPC, nhận quest | game | `getNpcDialogue`, `listQuests`, `acceptQuest` | — | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| 8 | Vào màn (ghim version nội dung) | game | `startLevel` | — | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) |
| 9 | Bắt đầu activity trong quest / luyện tập tự do | game → learning | `startQuestActivity` (game gọi `POST /internal/v1/attempts`), `startActivityAttempt`, 🆕 `startAdHocAttempt` (question set, chọn chủ đề, `retry_wrong`), `getActivityAttempt` (resume), 🆕 `pauseActivityAttempt` | — | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| 10 | Trả lời, nhận phản hồi đúng/sai (server chấm), xem lại, ôn câu sai | learning | `submitActivityResponse`, `reportActivityProgress`, `submitActivityAttempt`, `getActivityAttemptReview`, `listMyActivityAttempts`, 🆕 `listMyWrongQuestions` | — | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| 11 | Quest tiến triển / hoàn thành (bất đồng bộ qua event) | game | `reportQuestStep`, `getQuest` | 🆕 `quest.progress.updated` | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| 12 | Nhận XP, lên cấp, phần thưởng | game | `getProgression`, `listXpHistory` | `progression.xp.updated`, `progression.level.reached` | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| 13 | Mở khóa khu, khám phá, chương truyện | game | `getWorldExploration` (gồm `chapters`), `reportDiscovery` | 🆕 `world.area.unlocked` | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| 14 | Xem túi đồ, ví Xu, mua đồ trang trí | game | `listInventory`, `getWallet`, `listShopItems`, `purchaseShopItem` | — | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| 15 | Challenge cá nhân | game → learning | `listGameChallenges`, `startGameChallenge` | `challenge.attempt.updated` | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| 16 | Kết bạn, club | chat | `sendFriendRequest`, `acceptFriendRequest`, `redeemFriendCode`, `createClub`, `requestJoinClub` | — | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) |
| 17 | Lập party, mời bạn, sẵn sàng, bắt đầu | competition | `createParty`, `inviteToParty`, `joinParty`, `setPartyReady`, `selectPartyActivity`, `startPartySession` | `party.*` (vd. `party.member.joined`, `party.session.started`) | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) |
| 18 | Boss nhóm / co-op quest | game / competition | `createBossEncounter`, `getBossEncounter` | `boss.encounter.ready`, `boss.answer.submit`, `boss.phase.*`, `coop.*` | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| 19 | Ghép trận 1v1/đội, thách đấu bạn | competition | `createMatchmakingTicket`, `createDuel`, `acceptDuel`, `acceptMatchProposal` | `matchmaking.*`, `battle.question.shown`, `battle.answer.submit`, `battle.match.ended` | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md), [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| 20 | Chat trong trận / party / club (quick message hiển thị theo ngôn ngữ người nhận) | chat | `listMyChannels`, `listChannelMessages`, 🆕 `listQuickMessages` | `chat.message.send`, `chat.message.created` | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) |
| 21 | Xem tiến độ học tập của mình | learning | `getMyProgress` | — | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) |
| 22 | Làm bài được giao trong game | learning | 🆕 `listMyAssignments` → bắt đầu activity như bước 9 | — | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| 23 | Giáo viên xem tiến độ lớp, chấm bài cần chấm tay (gồm bài nói — không có AI giọng nói, ADR-0059) | learning | `getClassDashboard`, `listReviewTasks`, `gradeReviewTask` | — | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| 24 | 🆕 Xem sự kiện đang chạy, tham gia, nhận thưởng mốc | game | `listGameEvents`, `getGameEvent`, `joinGameEvent`, `claimEventMilestone` | `event.status.changed`, `event.progress.updated` | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| 25 | 🆕 Đóng góp community goal của trường | game | `getCommunityGoal` | `event.community_goal.updated` | [EVENT_SEASON_SYSTEM §6](../01-game-design/EVENT_SEASON_SYSTEM.md#6-community-goal) |
| 26 | 🆕 Season track: xem cấp, nhận thưởng cấp, xem mùa trước | game | `getSeasonTrack`, `claimSeasonTier`, `listSeasonTrackHistory` | `season.track.updated` | [EVENT_SEASON_SYSTEM §7](../01-game-design/EVENT_SEASON_SYSTEM.md#7-season-và-season-track) |
| 27 | 🆕 Tham gia bạn bè đang ở khu khác | game | `joinFriendInWorld` | `hub.instance.assigned` | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) |
| 28 | 🆕 Đăng xuất / đăng xuất mọi thiết bị | identity, game | `logout`, `revokeAllSessions` | `session.session.replaced` | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) |

**Kết luận rà soát:** không còn bước thiếu contract. Lượt 2026-11-15 (SPEC-2) thêm các bước 24–28 và contract 🆕 ở bước 3, 5, 9, 10, 20, 22; message WS mới: `event.progress.updated`, `event.community_goal.updated`, `event.status.changed`, `season.track.updated`. Ba message 🆕 được thêm vào AsyncAPI trong lượt rà này (`quest.progress.updated`, `world.area.unlocked`, `hub.player.appearance_changed`) vì trước đó client chỉ biết tiến độ quest/mở khu/đổi trang phục của người khác bằng cách gọi lại REST.

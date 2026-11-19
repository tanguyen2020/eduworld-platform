# Feature Specification: F014 — Game session lát mỏng: một thiết bị, login events, client-config, manifest tối thiểu

**Feature Branch**: `014-game-session-thin` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `game` · **Mốc**: M-C1 · **Phụ thuộc**: F013
**Input**: Sau đăng nhập (F013), game tạo game session server-side; tối đa một session active mỗi học sinh, takeover có xác nhận; ghi lịch sử đăng nhập game (không IP thô) và lần vào game gần nhất; kết thúc phiên khi đăng xuất/đóng game/mất heartbeat; client-config để buộc cập nhật; content manifest tối thiểu theo tenant. FR nguồn: FR-AUTH-023, FR-GAME-007, FR-GAME-008, FR-GAME-013, FR-GAME-014.

## Clarifications

### Session 2026-11-16

- Q: Game session tạo trong `gameLogin` hay `POST /game/sessions`? → A: `createGameSession` (`POST /game/sessions`) của module game, gọi ngay sau `gameLogin`/`selectGameLoginTenant` — đã chốt bởi ADR-0083 (đóng CODE_PLAN D15; ADR-0031, ADR-0045, R30). `409 ACTIVE_SESSION_ON_OTHER_DEVICE` và `takeover` chỉ ở endpoint này; đăng nhập không còn `game_session_id`/`takeover`.
- Q: Takeover có cần nhập lại mật khẩu, và dùng `Idempotency-Key` nào? → A: Không nhập lại mật khẩu (token game từ `gameLogin` đủ, ADR-0083 mục 2). Lời gọi lại có body `takeover: true` khác lời gọi đầu nên client PHẢI sinh `Idempotency-Key` mới; dùng lại key cũ → `409 IDEMPOTENCY_KEY_REUSED` (API_DESIGN §6).
- Q: `gameLogout` có đóng game session? → A: Không (ADR-0083 mục 3). Client gọi `endGameSession` trước `gameLogout`; nếu không gọi được, job `session-timeout` đóng phiên theo heartbeat timeout.
- Q: `users.last_login_at` cập nhật đồng bộ hay bất đồng bộ? → A: Bất đồng bộ: module identity tiêu thụ `student.game_logged_in` (DATABASE_DESIGN §3.12, BACKEND_DESIGN §6.1). IDENTITY_MAPPING §3.3 ghi "cùng transaction" là không khả thi giữa hai database (ADR-0037) — cần sửa tài liệu.
- Q: Mở lại game bằng refresh token đã lưu (ADR-0086) ảnh hưởng gì tới game session? → A: Game client gọi `createGameSession` sau `gameLogin`/`selectGameLoginTenant` **hoặc** sau `refreshGameToken` `mode = resume` (F013); quy tắc một thiết bị, `409`/`takeover` không đổi. Takeover phải thu hồi cả phiên game lưu trên thiết bị cũ: `game_session.replaced` mang thêm `old_device_id`, consumer `revoke_on_session_replaced` (module identity, feature này) gọi `GameAuthSessionRevoker` (F013) → refresh/resume của thiết bị cũ nhận `401 REFRESH_TOKEN_INVALID`. Thoát game trên thiết bị cá nhân chỉ gọi `endGameSession`, không `gameLogout`.
- Q: "Manifest tối thiểu" là manifest nào? → A: `getContentManifest` theo tenant (ETag/304), có thể rỗng khi chưa có nội dung phát hành; manifest thế giới (`getWorldManifest`) thuộc F071.
- Q: `game.game_profiles.last_game_login_at`? → A: Bảng GameProfile do F070 tạo; F070 bổ sung cập nhật trường này vào transaction tạo session.
- Q: Consumer kết thúc session khi khóa/dọn tài khoản thuộc F014 hay F016? → A: Đã chốt theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý): F014 hiện thực consumer `user.access_revoked`/`user.status_changed`/`student.cleaned_up` (F014-AC-24) và kiểm bằng event fixture theo payload schema contract-first (F013 T045); kiểm end-to-end là F016-AC-24. Không thêm F016 vào cột Phụ thuộc.

### Session 2026-11-17

- Q: Chặn tạo game session và kết thúc phiên đang chạy khi tới giờ giới nghiêm (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

### Session 2026-11-19 (triển khai)

Quyết định khi triển khai (theo mặc định `[Proposed]` của spec/tài liệu sở hữu) và câu hỏi mở còn lại:

- Projection `ref.tenants`, `ref.schools`, `ref.students` của module game viết tay trong migration (không ent schema), cùng quyết định của F020 cho module learning (DATABASE_DESIGN §2.17 cột tối thiểu, không mixin); T006 chỉ còn ent schema `GameSession` (`entx.Base`) và `LoginEvent` (`entx.TenantAppendOnly`, bảng partition viết tay). `game.content_manifests` viết tay (khóa ghép `(tenant_id, manifest_version)`, do F026/F078 ghi).
- T002 (adapter facade `identity`): không cần adapter riêng trong module game — Principal được `pkg/httpx` phân giải qua `cmd/internal/wiring.PrincipalResolver` (facade `resolvePrincipal`, `pkg/modcall`) với cache `pkg/authz` (R30); use case chỉ đọc Principal trong context.
- `ref.schools.settings_subset.curfew_window` (từ `school.synced`/`school.updated`) mở rộng `platform_curfew`; `platform_curfew` đọc từ `EW_PLATFORM_CURFEW` (cùng biến module identity dùng ở F013).
- `tenant_admin` không có `lms.student_activity.read` trong catalog F012 → `listTenantStudentLoginEvents` trả `403 PERMISSION_DENIED`; grant giới hạn trường mà học sinh ở trường khác → `404` (F014-AC-12).
- `game.login_events` hiện chỉ có partition DEFAULT (giống `identity.login_events` F013); partition tháng do job partition bảng nghiệp vụ của feature sau.
- Khóa HMAC `ip_hash`: `EW_GAME_IP_HASH_KEY(_FILE)` (≥ 32 ký tự, xoay vòng bằng triển khai khóa mới).
- OQ F014-1: `login_events.sis_connection_id` — Principal (facade `resolvePrincipal`) chưa có `sis_connection_id` nên cột để `NULL`; cần bổ sung vào Principal hoặc projection (owner F011/F012).
- OQ F014-2: `platform_curfew` của module game lấy từ cấu hình `EW_PLATFORM_CURFEW` cho tới khi F015 phát `platform_settings.changed` (`key = platform_curfew`) và DATABASE_DESIGN định nghĩa bảng projection cài đặt platform ở các module nhận.
- OQ F014-3: dùng lại `Idempotency-Key` của một lời gọi bị `409 ACTIVE_SESSION_ON_OTHER_DEVICE` cho lời gọi `takeover: true` — API_DESIGN §6 (owner) không lưu response lỗi nên lời gọi lại được xử lý như mới (không phải `409 IDEMPOTENCY_KEY_REUSED` như Clarification 2026-11-16). F014-AC-07 được kiểm với key của lời gọi thành công (`409 IDEMPOTENCY_KEY_REUSED`). Cần thống nhất giữa spec và API_DESIGN.
- OQ F014-4: `safety_moderator` (role platform, không tenant) gọi route người chơi kèm `X-Tenant-Id` nhận `403 TENANT_MISMATCH` (middleware tenant của `pkg/httpx` chạy trước guard `game_player`) thay vì `403 GAME_ACCESS_STUDENT_ONLY`; vẫn là 403, không ghi gì.
- OQ F014-5: học sinh thuộc nhiều tenant có session active ở tenant khác: RLS không cho thấy session đó, unique index trả `409 ACTIVE_SESSION_ON_OTHER_DEVICE` không có `other_device` và `takeover` không kết thúc được session ở tenant khác (chờ timeout 45 s hoặc thiết kế tra cứu liên tenant).
- OQ F014-6: metric projection — MONITORING đặt `eduworld_projection_lag_seconds{module, projection}` nhưng module learning (F020) đã đăng ký `eduworld_projection_lag_seconds{table}`; module game dùng `eduworld_game_projection_lag_seconds{table}` và `eduworld_game_consumer_rejected_total` để không trùng registry; cần một metric chung trong `pkg/`.
- OQ F014-7 (cùng OQ F010-2): binary `core-api`/`worker` chưa nối route, consumer, job của module nào (identity F010–F013 cũng vậy); module game cung cấp `config.SessionFrom`, `http.Register`, `consumer.*Options`, `job.*` để composition root nối sau. E2E hai client trên stack chạy thật (T036) vì vậy được kiểm ở mức integration qua chuỗi `pkg/httpx` thật (`TestTakeover`).
  - 2026-11-20 [Proposed]: binary đã nối theo OQ F010-2 (specs/010-tenant-org-structure/spec.md, phiên "nối binary"): `module.go` của module khai báo route/relay/consumer/job, `cmd/internal/wiring.Compose` ghép module.
- OQ F014-8: `game.content_manifests` theo DDL DATABASE_DESIGN §3.13 (`etag varchar(64)`, `entries` ≤ 4 MiB) — data-model ghi `varchar(128)`/64 KiB; đã theo tài liệu sở hữu.
- OQ F014-9: message trên Valkey `ew:rt:user:<id>` là `{type, payload}` (payload theo AsyncAPI `session.session.replaced`/`session.session.ended`); gateway F090 bọc `ServerEnvelope` (`v`, `seq`, `ts`) [Proposed].

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tạo game session, một thiết bị mỗi tài khoản (Priority: P1)

Học sinh vào game sau đăng nhập; nếu đang chơi ở máy khác, máy mới hỏi xác nhận; đồng ý thì máy cũ bị đăng xuất kèm thông báo.

**Why this priority**: Quy tắc một thiết bị là bắt buộc (ADR-0031, R23); mọi API người chơi sau đó gắn với session.

**Independent Test**: Hai client headless: A tạo session; B tạo → `409 ACTIVE_SESSION_ON_OTHER_DEVICE`; B gọi lại `takeover: true` → `201`, A nhận `session.session.replaced`, chỉ một session active trong DB.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F014-AC-01 | KHI học sinh (token `azp = eduworld-game-backend`, `user_type = student`, đang hoạt động) gọi `createGameSession` với `X-Device-Id`, `X-Tenant-Id`, `Idempotency-Key`, module game PHẢI tạo session ghi nền tảng, `client_version`, thiết bị rút gọn, `keycloak_sid` và trả `201` | FR-GAME-007 · ADR-0031 | TC-AUTH-019 |
| F014-AC-02 | NẾU người gọi không phải học sinh hoặc học sinh không còn hoạt động trong projection, THÌ module game PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY`, không tạo session/login event và ghi audit | FR-AUTH-023 · AC-AUTH-015 · AGENTS R23 | TC-AUTH-013 |
| F014-AC-03 | KHI học sinh có session active trên thiết bị khác còn heartbeat trong ngưỡng timeout, module game PHẢI trả `409 ACTIVE_SESSION_ON_OTHER_DEVICE` kèm `other_device` (nền tảng, nhãn thiết bị rút gọn, thời điểm bắt đầu; không `device_id` thô) | FR-AUTH-023 · AC-AUTH-016 | TC-AUTH-015 |
| F014-AC-04 | KHI gọi lại với `takeover: true` (cùng token game, `Idempotency-Key` mới, không cần mật khẩu), module game PHẢI kết thúc session cũ (`replaced_by_other_device`, `replaced_by_session_id`) và tạo session mới trong một transaction, phát `game_session.replaced`; thiết bị cũ PHẢI nhận `session.session.replaced` rồi đóng kết nối 4409, và module identity PHẢI thu hồi phiên Keycloak theo `sid` cũ cùng phiên game đã lưu của thiết bị cũ (`game_auth_sessions`, `revoke_reason = takeover`; refresh/resume của thiết bị cũ → `401 REFRESH_TOKEN_INVALID`) | FR-AUTH-023 · AC-AUTH-016 · AC-AUTH-028 · ADR-0083 · ADR-0086 | TC-AUTH-015, TC-AUTH-087 |
| F014-AC-05 | KHI hai thiết bị gọi `createGameSession` đồng thời, module game PHẢI cho đúng một thành công và thiết bị kia nhận `409` | FR-AUTH-023 | TC-AUTH-016 |
| F014-AC-06 | KHI session cũ không heartbeat quá ngưỡng timeout, module game PHẢI kết thúc session cũ với `timeout` và tạo session mới không hỏi xác nhận; cùng `device_id` PHẢI kết thúc session cũ (`logout`) rồi tạo mới | FR-AUTH-023 · FR-GAME-014 | TC-AUTH-017 |
| F014-AC-07 | NẾU thiếu `Idempotency-Key`, THÌ module game PHẢI trả `428 PRECONDITION_REQUIRED`; gửi lại cùng key và cùng body PHẢI trả cùng kết quả, không tạo session thứ hai; cùng key nhưng body khác (ví dụ thêm `takeover: true`) PHẢI trả `409 IDEMPOTENCY_KEY_REUSED` | FR-GAME-007 · NFR-SEC-004 · [API_DESIGN §6](../../docs/07-backend/API_DESIGN.md) | — |
| F014-AC-08 | NẾU `X-Tenant-Id` khác tenant của học sinh, THÌ module game PHẢI trả `403 TENANT_MISMATCH`; session luôn mang `tenant_id` của học sinh | FR-GAME-007 · ADR-0065 | TC-TEN-011 |
| F014-AC-25 | TRONG KHI thời điểm server nằm trong giới nghiêm hiệu lực của học sinh, module game PHẢI từ chối `createGameSession` (kể cả `takeover`) bằng `403 CURFEW_ACTIVE` kèm `reopen_at`, không tạo session hay login event thành công | NFR-SAFE-005 · AC-AUTH-032 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 | TC-AUTH-092 |

---

### User Story 2 — Lịch sử đăng nhập game và lần vào game gần nhất (Priority: P1)

Mỗi lần vào game được ghi thời điểm vào/ra (giờ server), nền tảng, thiết bị rút gọn, lý do kết thúc; nhà trường xem lịch sử và "số ngày chưa vào game"; không ai thấy IP.

**Why this priority**: Nhà trường cần theo dõi hoạt động; job dọn học sinh (F016) cần `last_login_at`.

**Independent Test**: Vào game → đúng một dòng login event, `users.last_login_at` cập nhật; `ad.truong.a` xem lịch sử của `hs.an.3a` không có IP/`device_id`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F014-AC-09 | KHI tạo session thành công, module game PHẢI ghi đúng một login event append-only (thời điểm server, nền tảng, thiết bị rút gọn, `client_version`, `ip_hash` HMAC, `sis_connection_id`) và phát `student.game_logged_in` trong cùng transaction | FR-GAME-013 · AC-AUTH-017 | TC-AUTH-019 |
| F014-AC-10 | KHI nhận `student.game_logged_in`, module identity PHẢI cập nhật `last_login_at` của học sinh (idempotent, không lùi thời gian) | FR-GAME-013 · ADR-0031 | TC-AUTH-019 |
| F014-AC-11 | HỆ THỐNG PHẢI không lưu IP thô ở bất kỳ đâu và không trả IP, `ip_hash`, `device_id` trong mọi response lịch sử đăng nhập | FR-GAME-013 · AGENTS R23 · NFR-PRIV-002 | TC-AUTH-019 |
| F014-AC-12 | KHI Tenant Admin/School Admin/Academic Manager có `lms.student_activity.read` gọi `listTenantStudentLoginEvents`, module game PHẢI trả lịch sử của học sinh trong scope; học sinh trường khác cùng tenant hoặc tenant khác PHẢI trả `404` | FR-GAME-013 · NFR-SEC-006 | TC-TEN-001 |
| F014-AC-13 | KHI System Admin gọi `listLoginEventsCrossTenant` với `tenant_ids`, module game PHẢI trả dữ liệu có cột `tenant_id` và ghi audit `cross_tenant.accessed`; role khác PHẢI nhận `403 PERMISSION_DENIED` | FR-GAME-013 · ADR-0027 | TC-TEN-018 |

---

### User Story 3 — Kết thúc phiên: đăng xuất, đóng game, mất heartbeat (Priority: P1)

Bấm "Đăng xuất" hoặc đóng game: client gọi `endGameSession` rồi `gameLogout` (F013, chỉ thu hồi token — ADR-0083); thiết bị tắt đột ngột thì phiên tự kết thúc khi mất heartbeat.

**Why this priority**: Session treo chặn đăng nhập máy khác và làm sai lịch sử.

**Independent Test**: `endGameSession` → `ended_at`, `logout`; không heartbeat quá ngưỡng → job đặt `timeout`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F014-AC-14 | KHI học sinh gọi `endGameSession` cho session của mình, module game PHẢI đặt `ended_at`, `end_reason = logout`, ghi `logout_at`/`end_reason` vào login event (một lần) và phát `student.game_logged_out`, `game_session.ended` | FR-GAME-014 | TC-AUTH-019 |
| F014-AC-15 | KHI học sinh gọi `heartbeatGameSession`, module game PHẢI cập nhật `last_heartbeat_at`; NẾU session đã bị thay hoặc đã kết thúc, THÌ PHẢI trả `409` | FR-GAME-014 | — |
| F014-AC-16 | TRONG KHI session active không nhận heartbeat (REST hoặc WebSocket) quá ngưỡng timeout của FR-GAME-014, job `session-timeout` PHẢI kết thúc session với `timeout` và cập nhật login event, kể cả khi client đã gọi `gameLogout` (identity) mà không gọi `endGameSession` | FR-GAME-014 · ADR-0083 | TC-AUTH-017 |
| F014-AC-17 | NẾU học sinh gọi `endGameSession`/`heartbeatGameSession` với session của người khác, THÌ module game PHẢI trả `404` | FR-GAME-014 · NFR-SEC-006 | TC-TEN-001 |
| F014-AC-24 | KHI nhận `user.access_revoked` (hoặc `user.status_changed` sang khóa) hay `student.cleaned_up` cho học sinh đang có session active, module game PHẢI kết thúc session với `end_reason` theo ánh xạ của [EVENT_DESIGN](../../docs/07-backend/EVENT_DESIGN.md) (`account_locked`; `cleanup`; `access_revoked` cho lý do `access_revoked`/`admin`/`password_changed`), ghi `logout_at`/`end_reason` vào login event, phát `game_session.ended`, xử lý idempotent theo `event_id`; consumer PHẢI được kiểm bằng event fixture hợp lệ theo payload schema contract-first (F013 T045), kiểm end-to-end khóa/dọn thật thuộc F016-AC-24 | FR-GAME-014 · FR-AUTH-010 · ADR-0031 · [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) | TC-AUTH-006 |
| F014-AC-18 | HỆ THỐNG PHẢI cho mọi route người chơi của feature (`createGameSession`, `endGameSession`, `heartbeatGameSession`, `getContentManifest`) trả `403 GAME_ACCESS_STUDENT_ONLY` với token nhân sự (`school_admin`, `academic_manager`, `tenant_admin`, `safety_moderator`, `system_admin`) | FR-AUTH-023 · AGENTS R23 | TC-AUTH-014 |
| F014-AC-26 | KHI tới đầu giới nghiêm hiệu lực, job `curfew-session-closer` PHẢI kết thúc mọi game session active của học sinh bị áp với `end_reason = curfew` (sai lệch ≤ 1 phút), ghi login event, phát `game_session.ended`, publish `session.session.ended` (`reason = curfew`, `reopen_at`) qua Valkey cho client (gateway F090 giao rồi đóng `4410`); tiến độ đã ghi server giữ nguyên; cảnh báo trước `curfew_warning_minutes` do client hiển thị từ khung giới nghiêm trong bootstrap (F015-AC-22) | NFR-SAFE-005 · AC-AUTH-032 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) | TC-AUTH-092 |

---

### User Story 4 — Client-config và buộc cập nhật (Priority: P1)

Game hỏi cấu hình trước khi hiện form đăng nhập: phiên bản tối thiểu, bảo trì, URL đăng nhập; bản cũ bị buộc cập nhật.

**Why this priority**: Không chặn client cũ thì API thay đổi làm hỏng trải nghiệm.

**Independent Test**: `GET /game/client-config?platform=android&version=0.9.0` (không token, không `X-Tenant-Id`) → `update_required = true` khi thấp hơn tối thiểu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F014-AC-19 | KHI client gọi `getClientConfig` (không token, không `X-Tenant-Id`), module game PHẢI trả `min_client_version`, `latest_client_version`, `update_required` theo `version` gửi lên, `maintenance`, `login.login_url`, `login.remember_username_default`, `supported_locales`, và không chứa client id/secret hay cấu hình OIDC | FR-GAME-008 · ADR-0068 | TC-AUTH-010 |
| F014-AC-20 | NẾU `client_version` của `createGameSession` thấp hơn `min_client_version`, THÌ module game PHẢI trả `426 CLIENT_VERSION_UNSUPPORTED` | FR-GAME-008 | TC-AUTH-066 |
| F014-AC-21 | TRONG KHI bảo trì đang bật, module game PHẢI trả `maintenance.active = true` kèm thông điệp và `createGameSession` PHẢI trả `503 MAINTENANCE` | FR-GAME-008 | — |

---

### User Story 5 — Content manifest tối thiểu theo tenant (Priority: P2)

Game tải manifest nội dung của tenant khi khởi động và cache theo `ETag`.

**Why this priority**: Khung để M-C2 đưa nội dung đã phát hành vào game; chưa có nội dung vẫn phải chạy.

**Independent Test**: `getContentManifest` lần đầu → `200` + `ETag`; gửi `If-None-Match` → `304`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F014-AC-22 | KHI học sinh gọi `getContentManifest`, module game PHẢI trả manifest mới nhất của tenant phiên (`tenant_id`, `manifest_version`, `etag`, `items`, `min_client_version`), trả manifest rỗng `manifest_version = 0` khi tenant chưa có nội dung | FR-GAME-007 · ADR-0043 | — |
| F014-AC-23 | KHI request có `If-None-Match` khớp `etag` hiện hành, module game PHẢI trả `304`; manifest của tenant khác PHẢI không bao giờ được trả | FR-GAME-007 · NFR-SEC-006 | TC-TEN-001 |

### Edge Cases

- NẾU nhận `user.access_revoked`/`student.cleaned_up` khi đang có session, THÌ kết thúc session với `account_locked`/`cleanup`/`access_revoked` theo lý do (consumer ở feature này, F014-AC-24; kiểm end-to-end ở F016-AC-24).
- NẾU học sinh takeover khi đang trong trận, THÌ xử lý grace ở module competition (F105, TC-AUTH-018).
- NẾU Valkey lỗi khi đẩy `session.session.replaced`, THÌ session cũ vẫn kết thúc trong DB; thiết bị cũ nhận `409` ở heartbeat kế tiếp.
- NẾU projection `ref.students` chưa có học sinh (event đến chậm), THÌ trả `503 UNAVAILABLE` có `Retry-After`, không tạo session.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-023 | Một session active mỗi học sinh, takeover, thu hồi phiên Keycloak cũ | [ADR-0031](../../docs/adr/ADR-0031-game-chi-danh-cho-hoc-sinh-luu-lich-su-dang-nhap-mot-thiet-b.md) |
| FR-GAME-007 | Game session server-side ghi nền tảng, phiên bản, lý do kết thúc; manifest tối thiểu | [DATABASE_DESIGN §3.12](../../docs/07-backend/DATABASE_DESIGN.md) |
| FR-GAME-008 | Client-config, phiên bản tối thiểu, buộc cập nhật | [API_DESIGN §9.2](../../docs/07-backend/API_DESIGN.md) |
| FR-GAME-013 | Login events, `last_login_at`, xem lịch sử không IP | [ADMIN_PORTAL §3.10](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-GAME-014 | Đăng xuất, đóng game kết thúc phiên, timeout heartbeat | [KEYCLOAK_SSO §7](../../docs/09-identity/KEYCLOAK_SSO.md) |

### Key Entities

- **GameSession**: phiên chơi server-side của một học sinh trên một thiết bị; trạng thái active/ended, lý do kết thúc, session thay thế.
- **GameLoginEvent**: bản ghi append-only mỗi lần vào game (vào/ra, nền tảng, thiết bị rút gọn, không IP thô).
- **ClientConfig**: cấu hình công khai cho client (phiên bản, bảo trì, URL đăng nhập).
- **ContentManifest**: danh mục nội dung đã phát hành của tenant có version và ETag.
- **Projection học sinh/tenant**: bản sao tối thiểu từ identity để kiểm học sinh còn hoạt động.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `game_remember_username` | Giá trị `login.remember_username_default` |
| `supported_locales` | `supported_locales` trong client-config |
| `tenant_header_required` | `getClientConfig` miễn `X-Tenant-Id` |
| `idempotency_record_ttl` | Lưu kết quả `createGameSession` theo `Idempotency-Key` |
| `api_rate_default` | Rate limit heartbeat/end |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| `createGameSession` một transaction, không gọi đồng bộ identity ngoài `resolvePrincipal` có cache; p95 theo `login_slo` | AGENTS R30 · ADR-0045 |
| Không IP thô; `ip_hash` HMAC khóa xoay vòng | AGENTS R23 · NFR-PRIV-002 |
| Middleware `game_player` mọi route người chơi | AGENTS R23 |
| Chỉ soft-delete; login events append-only | AGENTS R21 · NFR-SCAL-004 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 trường hợp hai session active của cùng học sinh trong test đồng thời 100 lần.
- **SC-002**: Thiết bị cũ bị đăng xuất trong ≤ 5 giây sau takeover (khi có realtime) và refresh token cũ bị từ chối ở lần gọi kế tiếp.
- **SC-003**: Mỗi lần vào game tạo đúng một login event; 0 IP thô trong DB/log.
- **SC-004**: `createGameSession` p95 đạt ngưỡng `login_slo` ở tải `login_capacity_target` trên staging.

## Ngoài phạm vi

- Realtime ticket (`createRealtimeTicket`), WebSocket, giao `session.session.replaced` tới client → F090 (F014 chỉ publish lên Valkey).
- GameProfile, nickname, `last_game_login_at` → F070; manifest thế giới, area → F071; nội dung manifest do phát hành → F026/F078.
- UI Godot (hộp thoại takeover, đăng xuất, buộc cập nhật) → F061; màn "Hoạt động đăng nhập" → F052.
- Phát event khóa/dọn tài khoản và kiểm end-to-end khóa/dọn thật → kết thúc session → F016 (F016-AC-24); telemetry client → F080.

## Assumptions

- `min_client_version` và ngưỡng timeout heartbeat (FR-GAME-014) chưa có tên trong CONFIG_DEFAULTS; đề xuất `min_client_version`, `game_session_heartbeat_timeout` (task Polish) [Proposed].
- F013 đã đưa `sid` vào token game; module game đọc `sid` từ JWT đã xác thực.
- Projection `ref.tenants`, `ref.students` của module game được tạo ở feature này (module game chưa có feature nào trước đó).

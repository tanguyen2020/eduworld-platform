# Feature Specification: F013 — Đăng nhập game qua backend: login, chọn tenant, refresh, logout, bảo vệ đăng nhập, hàng chờ

**Feature Branch**: `013-game-login-backend` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `identity` · **Mốc**: M-C1 · **Phụ thuộc**: F011, F012
**Input**: Học sinh nhập username + mật khẩu trong form của game; module identity xác thực với Keycloak bằng client confidential `eduworld-game-backend`, xác định tenant theo membership, trả token + PlayerBootstrap; chọn tenant sau đăng nhập; refresh qua backend có xoay vòng, phát hiện dùng lại, gắn thiết bị và mở lại game bằng refresh token đã lưu (`mode = resume`, ADR-0086); thu hồi phiên game khi đăng xuất/takeover/đổi mật khẩu/khóa/thu hồi quyền/dọn học sinh; rate limit, brute-force, hàng chờ; header `X-Tenant-Id` bắt buộc. FR nguồn: FR-AUTH-002, FR-AUTH-003, FR-AUTH-007, FR-AUTH-008, FR-AUTH-009, FR-AUTH-017, FR-AUTH-018, FR-AUTH-022, FR-AUTH-027, FR-AUTH-028.

## Clarifications

### Session 2026-11-16

- Q: Đăng nhập có tạo luôn game session (`GameLoginResponse.game_session_id`, CODE_PLAN Bước 0 D15)? → A: **Không** — đã chốt bởi ADR-0083 (đóng CODE_PLAN D15). `gameLogin`/`selectGameLoginTenant` chỉ xác thực, xác định tenant, trả `tokens` + `bootstrap`; module identity không gọi module game trên đường đăng nhập (AGENTS R30). Game session, `409 ACTIVE_SESSION_ON_OTHER_DEVICE` và `takeover` thuộc `createGameSession` (F014). Contract đã bỏ `game_session_id` khỏi `GameLoginResponse`, `takeover` khỏi `GameLoginRequest`/`GameTenantSelectRequest`, `409`/`other_device` khỏi đăng nhập; còn lại: `PlayerBootstrap.game_profile` thành tùy chọn (hồ sơ game lấy từ module game, F070).
- Q: Logout đóng game session thế nào khi module identity không chạm DB game? → A: `gameLogout` chỉ thu hồi refresh token + phiên Keycloak (ADR-0083 mục 3). Client gọi `endGameSession` (F014) rồi `gameLogout`; nếu client không gọi được, game session tự kết thúc theo heartbeat timeout (F014).
### Session 2026-11-16 (ADR-0086)

- Q: Game có phải nhập mật khẩu mỗi lần mở (ADR-0064)? → A: **Không** trên thiết bị cá nhân — ADR-0086 thay ADR-0064 mục 1, 2, 4, 5. Client lưu refresh token trong kho bảo mật OS (F060/F061); server: phiên client `eduworld-game-backend` idle 7 ngày / tối đa 30 ngày (`game_session_sso`), không offline token. `refreshGameToken` nhận `device_id` (bắt buộc) và `mode` (`renew` | `resume`); `resume` kiểm tra lại tài khoản/tenant/enrollment và trả `bootstrap`.
- Q: Phát hiện dùng lại và gắn thiết bị dựa vào đâu? → A: module identity ghi phiên đăng nhập game (`game_auth_sessions`: `sid`, `device_id`, SHA-256 của refresh token hiện hành + ≤ 20 hash đã xoay; không lưu token thô). Hash đã xoay → `401 REFRESH_TOKEN_REUSED` + thu hồi cả phiên; `device_id` khác → `401 REFRESH_TOKEN_DEVICE_MISMATCH` (không thu hồi); hết hạn/thu hồi/không nhận ra → `401 REFRESH_TOKEN_INVALID`. Ba mã mới đã thêm vào `ProblemCode` + API_DESIGN §4.1.
- Q: Thu hồi phiên game khi nào, ai làm? → A: module identity: `gameLogout` (US3); consumer `user.access_revoked`/`user.status_changed` (khóa, xử phạt, thu hồi quyền), `student.cleaned_up` (ADR-0085); takeover (consumer `game_session.replaced` thuộc F014) và đổi/đặt lại mật khẩu (F016) gọi port `GameAuthSessionRevoker` của feature này. Mỗi lần thu hồi: `revoked_at` + xóa phiên Keycloak theo `sid` + audit.
- Q: Nguồn `min_client_version` cho kiểm tra `426` ở module identity? → A: Cấu hình triển khai dùng chung với module game (`EW_GAME_MIN_CLIENT_VERSION`) [Confirmed]; cần đăng ký tên tham số trong CONFIG_DEFAULTS (task Polish).

### Session 2026-11-17 (ADR-0089)

- Q: Đường đăng nhập đọc membership thế nào khi chưa có tenant (interceptor trả `ErrMissingTenantScope`, RLS trả 0 dòng khi chưa đặt `app.tenant_id`)? → A: module identity gọi hàm `identity.memberships_for_user(user_id)` (`SECURITY DEFINER`, owner là role `identity_membership_reader` `NOLOGIN` + `BYPASSRLS` chỉ có `SELECT` trên nguồn membership, chỉ trả cột membership của đúng user; `EXECUTE` cấp cho `identity_app`) bên trong `tenancy.Bypass(ctx, "auth_login")`, chỉ trên route `/auth/session`, `/auth/game/*` và `login-credentials:verify`; role ứng dụng không có `BYPASSRLS`; mỗi lần gọi có audit `cross_tenant.accessed` + metric `eduworld_tenant_filter_bypass_total{reason="auth_login"}` ([MULTITENANCY §4.4](../../docs/06-architecture/MULTITENANCY.md#44-quy-tắc-interceptor-theo-mixin-ba-loại-scope-và-cổng-bypass-adr-0089)).
- Q: Mixin của `game_auth_sessions` và `login_events`? → A: `game_auth_sessions` là bảng T, mixin `entx.Base`; `identity.login_events` là bảng TA\*, mixin `TenantOptionalAppendOnly`. Dòng đã biết tenant ghi bằng `identity_app` dưới `TenantScope`; dòng `tenant_id` NULL (thất bại trước khi xác định tenant) ghi bằng `identity_platform` dưới `PlatformScope` (ADR-0089 §2).
- Q: F013 (M-C1) tiêu thụ `user.access_revoked`, `user.status_changed`, `student.cleaned_up` do F016 (M-C3) phát — xử lý phụ thuộc ngược mốc thế nào? → A: Đã chốt theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý): F013 không tạo schema của producer (owner F011 cho `user.*`, F016 cho `student.cleaned_up` — EVENT_DESIGN §4); task T045 chỉ tạo event fixture theo payload của EVENT_DESIGN §4.1, F013 hiện thực consumer và kiểm bằng fixture (F013-AC-29), contract test với schema của owner bật khi file có → `Done` ở M-C1. Kiểm end-to-end (khóa/dọn thật → thu hồi phiên game) là F016-AC-24. Không thêm F016 vào cột Phụ thuộc.
- Q: Chặn đăng nhập và vào lại bằng refresh token trong giờ giới nghiêm (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

### Session 2026-11-19 (triển khai)

- Q: Học sinh còn required action Keycloak (mật khẩu tạm `UPDATE_PASSWORD`) đăng nhập game trả gì? Contract game không có `ACCOUNT_SETUP_REQUIRED`. → A: [Confirmed] `401 INVALID_CREDENTIALS` như mọi lỗi thông tin đăng nhập (không lộ trạng thái tài khoản); nhà trường đặt lại mật khẩu học sinh (F016) — OQ F013-1.
- Q: Học sinh thuộc hai tenant (US2) có xảy ra với mô hình dữ liệu hiện tại? → A: Không: `org.students.student_id` là khóa chính (= `user_id`) và `enrollments_one_active_uq` chỉ cho một enrollment active trên toàn hệ thống, nên `memberships_for_user` trả tối đa một tenant học sinh. Luồng `TENANT_NOT_SELECTED` + `login_ticket` + `selectGameLoginTenant` vẫn hiện thực đầy đủ và được kiểm bằng test double thêm membership thứ hai (`extraMembership`, có nhãn) — OQ F013-2 (cùng gốc OQ F011-1 học sinh trùng người liên tenant).
- Q: Claim `organization` (F013-AC-25)? → A: Realm hiện không gán scope `organization` cho client nào nên token không có claim này. `pkg/httpx` đọc claim (dạng danh sách alias hoặc object theo alias) và từ chối token có > 1 organization bằng `403 TENANT_NOT_SELECTED`; đối chiếu alias với tenant đang hoạt động (`TENANT_ACCESS_DISABLED`) cần alias trong Principal (facade `resolvePrincipal`) — OQ F013-3 [Confirmed: làm khi bật scope `organization`].
- Q: Khung giới nghiêm của trường (F015)? → A: `CurfewSource` là port; F013 dùng `platform_curfew` từ `EW_PLATFORM_CURFEW` (mặc định 21:30–06:30 ICT, [Confirmed] ADR-0078) cho mọi học sinh, mọi khối, tenant chung; F015 thay bằng nguồn hợp khung trường — OQ F013-4.
- Q: `ip_hash`? → A: HMAC-SHA-256 khóa `EW_LOGIN_IP_HASH_KEY` của địa chỉ kết nối tới `core-api`; sau APISIX cần cấu hình proxy tin cậy để lấy IP client thật [Confirmed] — OQ F013-5.
- Q: Keycloak thật trong testcontainers (plan Testing)? → A: Như F012, test integration dùng Keycloak giả có nhãn (`fakeKeycloak`, endpoint token/logout/Admin `DELETE /sessions/{sid}` theo Keycloak 26) cùng PostgreSQL và Valkey thật; kiểm với realm thật thuộc smoke F004/quickstart. Handler và consumer chưa nối vào binary `core-api`/`worker` (cùng OQ F010-2).

### Session 2026-11-20 (owner chốt)

- OQ F013-1 → **Chốt:** giữ `401 INVALID_CREDENTIALS` cho tài khoản còn required action; nhà trường đặt lại mật khẩu (F016).
- OQ F013-2 → **Chốt:** không xảy ra theo mô hình dữ liệu (OQ F011-1: một học sinh một tenant); giữ luồng `TENANT_NOT_SELECTED` + test double có nhãn.
- OQ F013-3 → **Chốt:** không bật client scope `organization` trong Release 1.0 (portal theo ADR-0091 mục 4, game không dùng); mã đọc claim giữ nguyên.
- OQ F013-4 → đã giải quyết bởi F015 (`gameauth.SettingsPolicy`).
- OQ F013-5 → **Chốt:** `httpx.ClientIP` với `EW_TRUSTED_PROXY_CIDRS` (task ở F002).
- k6 hàng chờ đăng nhập → chạy khi có môi trường staging (không phải câu hỏi mở).
- OQ-17 giới hạn chơi/ngày (owner chốt 2026-11-20, CHILD_SAFETY §4, CONFIG_DEFAULTS `daily_play_limit_minutes`) → **Chốt:** module identity từ chối `gameLogin`, `selectGameLoginTenant`, `refreshGameToken` `mode = resume` khi học sinh đã đạt giới hạn/ngày của trường, bằng mã lỗi mới `403 DAILY_PLAY_LIMIT_REACHED` kèm `reopen_at` = 00:00 ICT hôm sau (cùng shape `CURFEW_ACTIVE`). Không gọi module game đồng bộ (R30, ADR-0045): identity dựng projection thời gian chơi trong ngày từ `game_session.ended` (`duration_ms`, đã có trong EVENT_DESIGN) và đọc giới hạn trong cài đặt trường của chính module (F015-AC-30); module game là nơi cưỡng chế chính (F014-AC-27). Thêm F013-AC-33. Mã lỗi mới cần đăng ký ở API_DESIGN §4.1 (agent docs) và enum `ProblemCode`.

### Session 2026-11-21 (ADR-0095 — [Confirmed], WP-I1)

- Q: Đăng nhập game khi bỏ enrollment? → A: [Confirmed] Membership tenant của học sinh = học sinh hiệu lực (`org.students.status = active`, chưa xóa mềm, trường còn sống). `NO_ACTIVE_ENROLLMENT` giữ mã, nghĩa mới "không có trường/khối hiệu lực" (chuỗi game: "Em chưa có thông tin trường/khối trong năm học này."); login sync nhánh B dùng kết quả `missing_placement`. Bootstrap không còn `class`; khối lấy từ `student.grade_level`; `policies.party.cross_class_allowed` bị gỡ khỏi contract. Test: `TestGameLogin`, `TestLoginSync`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Học sinh đăng nhập bằng form trong game (Priority: P1)

Học sinh (`hs.an.3a`) nhập username + mật khẩu, không chọn trường; vào thẳng tenant duy nhất của mình với bootstrap đúng trường/lớp/khối (US-AUTH-001).

**Why this priority**: Không có đăng nhập thì không có trải nghiệm game nào.

**Independent Test**: Gọi `gameLogin` với `hs.an.3a` → `200`, `bootstrap.tenant_id` = `th-demo-a`, lớp 3A; `hs.nosis` → `403 ACCOUNT_NOT_PROVISIONED`, refresh token vừa cấp trả `invalid_grant`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F013-AC-01 | KHI học sinh gửi đúng username + mật khẩu, module identity PHẢI xác thực qua token endpoint Keycloak bằng client confidential `eduworld-game-backend` (secret chỉ ở module identity) trước khi đọc membership, và trả `200` chỉ gồm `tokens` + `bootstrap` (không có `game_session_id`, không tạo game session, không gọi module game, không có nhánh `409`) | FR-AUTH-002 · ADR-0068 · ADR-0069 · ADR-0083 | TC-AUTH-079 |
| F013-AC-02 | KHI học sinh thuộc đúng một tenant (SIS hoặc tenant chung), `bootstrap` PHẢI có `tenant_id`, `tenant_type`, `is_common_tenant`, `student`, `school` (tên song ngữ, tỉnh), `class`, `grade_band`, `policies`, `feature_flags`, lấy từ dữ liệu của module identity (login sync F011) | FR-AUTH-007 · AC-AUTH-001 | TC-AUTH-001 |
| F013-AC-03 | NẾU user không có ExternalIdentity hoặc không thuộc tenant chung lẫn tenant SIS nào, THÌ module identity PHẢI trả `403 ACCOUNT_NOT_PROVISIONED`, thu hồi phiên Keycloak theo `sid`, ghi audit `account_not_provisioned`, không tạo user/membership nào | FR-AUTH-017 · AC-AUTH-005 · ADR-0063 | TC-AUTH-032 |
| F013-AC-04 | NẾU `user_type ≠ student` (kể cả Keycloak trả `invalid_grant` do điều kiện role), THÌ module identity PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY`, thu hồi phiên vừa tạo, ghi audit `game_access_denied_non_student`, không phát token | FR-AUTH-022 · AC-AUTH-015 | TC-AUTH-064 |
| F013-AC-05 | NẾU user `locked`/`deactivated`, hoặc học sinh tenant thường không có enrollment active sau login sync, THÌ module identity PHẢI trả lần lượt `403 ACCOUNT_LOCKED`/`403 ACCOUNT_DEACTIVATED`/`403 NO_ACTIVE_ENROLLMENT` theo thứ tự kiểm tra IDENTITY_MAPPING §4.1 | FR-AUTH-007 · AC-AUTH-006 | TC-SIS-103 |
| F013-AC-06 | NẾU body đăng nhập có trường ngoài `username`, `password`, `device_id`, `client_version`, `platform`, `locale` (ví dụ mã trường, `tenant_id`, `takeover`) hoặc username chứa `@`, THÌ module identity PHẢI trả `422 VALIDATION_FAILED` mà không gọi Keycloak | FR-AUTH-017 · AC-AUTH-019 · ADR-0063 | — |
| F013-AC-07 | KHI đăng nhập thành công, module identity PHẢI cập nhật `external_identities.last_login_at`, chuyển user `pending → active` (phát `user.activated` lần đầu) và ghi một dòng nhật ký xác thực có `tenant_resolution`, `sis_connection_id`, `ip_hash` (không IP thô) | FR-AUTH-007 · ADR-0046 | TC-AUTH-019 |
| F013-AC-08 | KHI tài khoản học sinh tenant chung đăng nhập, `bootstrap.is_common_tenant` PHẢI là `true` và lớp là lớp ảo theo khối | FR-AUTH-017 · AC-AUTH-011 | TC-ORG-108 |
| F013-AC-30 | KHI `gameLogin`, `selectGameLoginTenant` hoặc `refreshGameToken` cần membership trước khi có `TenantScope`, module identity PHẢI gọi `identity.memberships_for_user(user_id)` của user vừa xác thực bên trong `tenancy.Bypass(ctx, "auth_login")` bằng role `identity_app` (không `BYPASSRLS`), ghi audit `cross_tenant.accessed` và tăng `eduworld_tenant_filter_bypass_total{reason="auth_login"}`; kết quả PHẢI chỉ gồm tenant của chính user đó; NẾU code ngoài route `/auth/session`, `/auth/game/*`, `login-credentials:verify` dùng lý do `auth_login` hoặc gọi hàm này, THÌ test kiến trúc PHẢI fail | ADR-0089 §3 · [MULTITENANCY §4.4](../../docs/06-architecture/MULTITENANCY.md) | — |
| F013-AC-31 | KHI đăng nhập thất bại trước khi xác định tenant, module identity PHẢI ghi dòng nhật ký xác thực `tenant_id` NULL bằng role `identity_platform` dưới `PlatformScope`; NẾU `identity_app` thử ghi dòng `tenant_id` NULL vào `identity.login_events`, THÌ RLS PHẢI từ chối; NẾU `identity_app` truy vấn `identity.tenant_memberships_v` trực tiếp khi chưa đặt `app.tenant_id`, THÌ PHẢI nhận 0 dòng | ADR-0089 §2–3 · AC-ORG-028 | TC-AUTH-019 |
| F013-AC-32 | TRONG KHI thời điểm server (ICT) nằm trong giới nghiêm hiệu lực của học sinh (`platform_curfew` hợp khung trường, F015-AC-22), module identity PHẢI từ chối `gameLogin`, `selectGameLoginTenant` và `refreshGameToken` `mode = resume` bằng `403 CURFEW_ACTIVE` kèm `reopen_at`, kiểm sau bước xác thực SSO và trạng thái tài khoản; với `resume` phiên game PHẢI không bị thu hồi (học sinh vào lại sau giờ giới nghiêm không cần mật khẩu); `renew` của phiên đang chạy không bị chặn (phiên game đã kết thúc theo F014-AC-26); áp như nhau cho mọi khối, tenant `common` bắt buộc | NFR-SAFE-005 · AC-AUTH-032 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 · ADR-0086 | TC-AUTH-092 |
| F013-AC-33 | TRONG KHI trường của học sinh bật `daily_play_limit_minutes` và tổng thời gian game session của học sinh trong ngày (ICT, projection từ `game_session.ended`) đã đạt giới hạn, module identity PHẢI từ chối `gameLogin`, `selectGameLoginTenant` và `refreshGameToken` `mode = resume` bằng `403 DAILY_PLAY_LIMIT_REACHED` kèm `reopen_at` = 00:00 ICT hôm sau, kiểm sau xác thực SSO, không thu hồi phiên game lưu trên thiết bị; trường không bật hoặc tenant `common` thì không áp | NFR-SAFE-005 · [CHILD_SAFETY §4](../../docs/10-security/CHILD_SAFETY.md) · ADR-0078 | — |

---

### User Story 2 — Học sinh thuộc nhiều tenant chọn tenant sau đăng nhập (Priority: P1)

Học sinh hiếm khi thuộc hai tenant; sau xác thực nhận danh sách và chọn, không nhập lại mật khẩu.

**Why this priority**: Bắt buộc theo ADR-0063; thiếu thì người dùng nhiều tenant không vào được.

**Independent Test**: Gắn học sinh test vào hai tenant → `gameLogin` trả `403 TENANT_NOT_SELECTED` + `choices[]` + `login_ticket`; `selectGameLoginTenant` → `200` tenant đã chọn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F013-AC-09 | KHI học sinh thuộc nhiều tenant, module identity PHẢI trả `403 TENANT_NOT_SELECTED` kèm `choices[]` (alias, tên, loại) và `login_ticket` dùng một lần, hết hạn theo `game_login_ticket_ttl`, gắn `device_id` | FR-AUTH-017 · AC-AUTH-011 | TC-TEN-007 |
| F013-AC-10 | KHI gọi `selectGameLoginTenant` với ticket hợp lệ và `tenant_id` thuộc `choices`, module identity PHẢI trả token + bootstrap của tenant đó mà không yêu cầu mật khẩu | FR-AUTH-017 · FR-AUTH-019 | TC-AUTH-065 |
| F013-AC-11 | NẾU ticket đã dùng, hết hạn hoặc từ `device_id` khác, THÌ module identity PHẢI trả `401 LOGIN_TICKET_INVALID`; `tenant_id` ngoài `choices` PHẢI trả `403 TENANT_MISMATCH`; cả hai không phát token | FR-AUTH-017 | TC-AUTH-065 |

---

### User Story 3 — Refresh, giữ phiên và đăng xuất (Priority: P1)

Trong lúc game mở, token được làm mới tự động không hỏi mật khẩu; trên thiết bị cá nhân, mở lại game khi phiên còn hạn dùng refresh token đã lưu (`mode = resume`) thay cho mật khẩu; bấm "Đăng xuất" thu hồi phiên (US-AUTH-002, ADR-0086).

**Why this priority**: Access token 5 phút; thiếu refresh thì học sinh bị đẩy ra liên tục; thiếu `resume` thì học sinh nhỏ phải nhập mật khẩu mỗi lần mở game.

**Independent Test**: `gameLogin` → `refreshGameToken` (`resume`) → token mới + `bootstrap`; refresh cũ dùng lại → `401 REFRESH_TOKEN_REUSED` và token mới nhất cũng bị từ chối; `device_id` khác → `401 REFRESH_TOKEN_DEVICE_MISMATCH`; `gameLogout` → `204`, refresh sau đó `401 REFRESH_TOKEN_INVALID`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F013-AC-12 | KHI game gọi `refreshGameToken` với refresh token hợp lệ, `device_id` trùng thiết bị gắn phiên và `X-Tenant-Id` thuộc membership, module identity PHẢI đổi token với Keycloak bằng client `eduworld-game-backend` (rotation), cập nhật hash token hiện hành của phiên và trả cặp token mới (refresh token mới, token cũ vô hiệu) | FR-AUTH-008 · AC-AUTH-002 · ADR-0086 | TC-AUTH-002, TC-AUTH-068 |
| F013-AC-13 | NẾU refresh token hết hạn (idle/max theo `game_session_sso`), đã bị thu hồi hoặc không thuộc phiên nào, THÌ module identity PHẢI trả `401 REFRESH_TOKEN_INVALID`; NẾU token là token **đã xoay vòng** của một phiên, THÌ PHẢI trả `401 REFRESH_TOKEN_REUSED`, thu hồi cả phiên (`sid`, kể cả token mới nhất, `revoke_reason = refresh_reuse`), ghi audit + security event | FR-AUTH-008 · AC-AUTH-003 · AC-AUTH-027 · ADR-0086 | TC-AUTH-083, TC-AUTH-085 |
| F013-AC-14 | KHI game gọi `gameLogout`, module identity PHẢI thu hồi refresh token và phiên Keycloak theo `sid` (`game_auth_sessions.revoke_reason = logout`), trả `204` kể cả khi token đã hết hạn (idempotent), ghi audit, và PHẢI không đóng game session hay gọi module game (client gọi `endGameSession` trước — F014) | FR-AUTH-009 · AC-AUTH-004 · ADR-0083 · ADR-0086 | TC-AUTH-004, TC-AUTH-088 |
| F013-AC-15 | HỆ THỐNG PHẢI không lưu access/refresh token thô của học sinh ở kho bền nào của server (chỉ SHA-256 của refresh token trong `game_auth_sessions`, ngoài phiên Keycloak) và không cấp `offline_access`; phiên client `eduworld-game-backend` PHẢI hết hạn theo `game_session_sso` | FR-AUTH-008 · ADR-0086 | TC-AUTH-003, TC-AUTH-083 |
| F013-AC-27 | NẾU `device_id` trong body khác thiết bị gắn với phiên lúc đăng nhập, THÌ module identity PHẢI trả `401 REFRESH_TOKEN_DEVICE_MISMATCH` mà không gọi Keycloak, không phát token, ghi audit, và PHẢI không thu hồi phiên của thiết bị gốc | FR-AUTH-008 · AC-AUTH-027 · ADR-0086 | TC-AUTH-086 |
| F013-AC-28 | KHI `refreshGameToken` có `mode = resume`, module identity PHẢI kiểm tra lại `user_type`, trạng thái tài khoản, tenant và enrollment như `gameLogin` (thứ tự IDENTITY_MAPPING §4.1) rồi trả token mới + `bootstrap` của tenant trong `X-Tenant-Id`; không còn hợp lệ → `403` tương ứng (không phát token, thu hồi phiên game); `client_version` thấp hơn tối thiểu → `426` (giữ phiên) | FR-AUTH-008 · AC-AUTH-020 · ADR-0086 | TC-AUTH-082 |
| F013-AC-29 | KHI nhận `user.access_revoked`/`user.status_changed` sang khóa (khóa, xử phạt, thu hồi quyền), `student.cleaned_up`, hoặc được gọi qua port `GameAuthSessionRevoker` (takeover — consumer `game_session.replaced` của F014; đổi/đặt lại mật khẩu — F016), module identity PHẢI thu hồi các phiên game liên quan (`revoked_at`, `revoke_reason`, xóa phiên Keycloak theo `sid`), xử lý idempotent theo `event_id`, ghi audit; lần refresh kế tiếp từ thiết bị đó PHẢI nhận `401 REFRESH_TOKEN_INVALID`. Consumer PHẢI được kiểm bằng event fixture hợp lệ theo payload schema contract-first (không chờ F016); kiểm end-to-end khóa/dọn thật thuộc F016-AC-24 | FR-AUTH-008 · AC-AUTH-028 · ADR-0086 · ADR-0031 · ADR-0085 | TC-AUTH-087 |

---

### User Story 4 — Bảo vệ đăng nhập và hàng chờ (Priority: P1)

Mật khẩu không bao giờ bị ghi lại; dò mật khẩu bị chặn theo tài khoản, không theo IP trường; giờ cao điểm có hàng chờ thay vì sập.

**Why this priority**: Mật khẩu học sinh đi qua backend (ADR-0068) — rủi ro bảo mật cao nhất của feature.

**Independent Test**: Gửi mật khẩu mẫu duy nhất rồi quét log APISIX/`core-api`/trace; sai 5 lần → khóa tạm với cùng thông điệp; 11 lần/phút từ một thiết bị → `429`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F013-AC-16 | HỆ THỐNG PHẢI không ghi body đăng nhập/refresh, mật khẩu, `refresh_token`, `login_ticket` vào log, trace, telemetry hay error report (kể cả lỗi 5xx/panic) và PHẢI xóa mật khẩu khỏi bộ nhớ sau khi gọi Keycloak (best effort) | FR-AUTH-003 · AC-AUTH-022 · ADR-0068 | TC-AUTH-060 |
| F013-AC-17 | KHI sai mật khẩu, username không tồn tại hoặc tài khoản đang khóa tạm (brute-force theo `brute_force_lockout`), module identity PHẢI trả cùng `401 INVALID_CREDENTIALS`, cùng thông điệp, chênh lệch thời gian phản hồi p50 < 50 ms | FR-AUTH-028 · AC-AUTH-023 | TC-AUTH-062 |
| F013-AC-18 | KHI một thiết bị (`X-Device-Id`) hoặc một username vượt ngưỡng `login_rate`, module identity PHẢI trả `429 RATE_LIMITED` + `Retry-After`; học sinh khác cùng IP NAT PHẢI không bị ảnh hưởng | FR-AUTH-028 · AGENTS R30 | TC-AUTH-063 |
| F013-AC-19 | KHI vượt ngưỡng đăng nhập toàn cục theo `login_rate`, module identity PHẢI trả `429 LOGIN_QUEUE_BUSY` + `Retry-After` thay vì lỗi 5xx | FR-AUTH-027 · ADR-0045 | TC-AUTH-044 |
| F013-AC-20 | NẾU `client_version` thấp hơn `min_client_version`, THÌ module identity PHẢI trả `426 CLIENT_VERSION_UNSUPPORTED` mà không gọi Keycloak | FR-AUTH-028 · FR-GAME-008 | TC-AUTH-066 |
| F013-AC-21 | NẾU token endpoint Keycloak quá `keycloak_token_call_timeout` hoặc circuit breaker mở, THÌ module identity PHẢI trả `503 UNAVAILABLE` có `Retry-After`, không lộ chi tiết | FR-AUTH-027 · NFR-AVAIL-005 | TC-PERF-016 |

---

### User Story 5 — Header `X-Tenant-Id` và kiểm tra chéo tenant ở mọi request (Priority: P1)

Sau đăng nhập, mọi request tenant-scoped mang `X-Tenant-Id`; backend đối chiếu với membership trong DB, không bao giờ đổi tenant theo header.

**Why this priority**: Lớp chặn rò dữ liệu giữa tenant (ADR-0065).

**Independent Test**: Gọi route tenant-scoped không header → `400`; header tenant khác → `403 TENANT_MISMATCH` + audit; route miễn trừ không cần header.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F013-AC-22 | NẾU request tenant-scoped thiếu `X-Tenant-Id`, THÌ `core-api` PHẢI trả `400 TENANT_HEADER_REQUIRED` (cả khi gọi thẳng bỏ qua gateway); header sai định dạng UUID PHẢI trả `400 BAD_REQUEST` | FR-AUTH-018 · AC-AUTH-024 | TC-TEN-011 |
| F013-AC-23 | NẾU `X-Tenant-Id` không khớp tenant của phiên (membership DB, claim nếu có, game session), THÌ `core-api` PHẢI trả `403 TENANT_MISMATCH`, ghi audit + security event, tăng `eduworld_auth_tenant_mismatch_total` | FR-AUTH-018 · AC-AUTH-010 | TC-TEN-006 |
| F013-AC-24 | HỆ THỐNG PHẢI miễn header cho đúng các route theo `tenant_header_required` (`/auth/game/login`, `/auth/game/tenant:select`, `/auth/session`, `/me/tenants`, `/game/client-config`, health) | FR-AUTH-018 · ADR-0065 | TC-TEN-011 |
| F013-AC-25 | NẾU token có claim `organization` nhiều tenant hoặc alias/id không khớp tenant đang hoạt động, THÌ `core-api` PHẢI trả `403 TENANT_NOT_SELECTED` / `403 TENANT_ACCESS_DISABLED` | FR-AUTH-018 · ADR-0026 | TC-TEN-007 |
| F013-AC-26 | NẾU token có chữ ký sai, `aud` khác `eduworld-api`, `iss` ngoài allowlist hoặc `azp` không được phép, THÌ backend PHẢI trả `401 UNAUTHENTICATED` độc lập với APISIX | FR-AUTH-018 · AC-AUTH-009 | TC-AUTH-009 |

### Edge Cases

- NẾU học sinh đang chuyển trường giữa hai tenant (đã rời trường cũ, trường mới chưa đồng bộ), THÌ `403 ACCOUNT_NOT_PROVISIONED` với thông điệp chuyển trường; token còn mang tenant cũ → `TENANT_MISMATCH`.
- NẾU học sinh bị dọn (`inactive_cleanup`) đăng nhập lại, THÌ không trả `ACCOUNT_NOT_PROVISIONED`; khôi phục do F016.
- NẾU Valkey lỗi, THÌ login ticket không tạo được → `503 UNAVAILABLE` (không cho chọn tenant thiếu ticket); rate limit chuyển sang giới hạn trong bộ nhớ của replica.
- NẾU ngoài khung giờ trường cấu hình, THÌ `403 ACCESS_OUTSIDE_ALLOWED_HOURS` (bật khi F015 có cài đặt trường); với `mode = resume` phiên game **không** bị thu hồi vì lý do này (học sinh vào lại được trong khung giờ).
- NẾU client gửi hai refresh song song với cùng token (lỗi single-flight phía client), THÌ lời gọi thứ hai bị coi là dùng lại (`REFRESH_TOKEN_REUSED`) và phiên bị thu hồi — client F061 bắt buộc refresh đơn luồng.
- NẾU Keycloak đã xoay token nhưng client không nhận được phản hồi (mất mạng), THÌ token client đang giữ đã là token cũ; lần thử lại → `REFRESH_TOKEN_REUSED` → đăng nhập lại (chấp nhận, ưu tiên an toàn) [Confirmed].

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-002 | `POST /auth/game/login` qua backend, client confidential, game không có secret | [KEYCLOAK_SSO §6](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-003 | Không lưu/log mật khẩu, xóa khỏi bộ nhớ | [SECURITY_ARCHITECTURE §3.1b](../../docs/10-security/SECURITY_ARCHITECTURE.md) |
| FR-AUTH-007 | PlayerBootstrap (tenant, trường, lớp, khối, policy, flag) | [IDENTITY_MAPPING §3](../../docs/09-identity/IDENTITY_MAPPING.md) |
| FR-AUTH-008 | Refresh qua backend: rotation, phát hiện dùng lại, gắn `device_id`, `mode = resume` (mở lại game bằng token đã lưu), thu hồi phiên game theo sự kiện, không offline token (phần server; lưu token trên client → F060/F061) | [KEYCLOAK_SSO §5.1](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-009 | `POST /auth/game/logout` thu hồi refresh + phiên Keycloak | [KEYCLOAK_SSO §7](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-017 | Không chọn trường; xác định tenant sau xác thực; `login_ticket`; `ACCOUNT_NOT_PROVISIONED` | [IDENTITY_MAPPING §3](../../docs/09-identity/IDENTITY_MAPPING.md) |
| FR-AUTH-018 | `X-Tenant-Id` bắt buộc, kiểm tra chéo membership | [MULTITENANCY §8.4](../../docs/06-architecture/MULTITENANCY.md) |
| FR-AUTH-022 | Chỉ học sinh đăng nhập game | [KEYCLOAK_SSO §3.1b](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-027 | Hàng chờ `429 LOGIN_QUEUE_BUSY` | [KEYCLOAK_SSO §12](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-028 | Rate limit, brute-force, lỗi chung, `426` | [SECURITY_ARCHITECTURE §14](../../docs/10-security/SECURITY_ARCHITECTURE.md) |

### Key Entities

- **GameLoginAttempt**: một lần gửi form đăng nhập (không lưu mật khẩu); kết quả và cách xác định tenant.
- **LoginTicket**: vé ngắn hạn, dùng một lần, gắn thiết bị, cho phép chọn tenant sau xác thực.
- **PlayerBootstrap**: ngữ cảnh phiên game: tenant, trường, lớp, khối, policy rút gọn, feature flag.
- **AuthLoginEvent**: nhật ký xác thực (thành công/thất bại), không IP thô.
- **GameAuthSession**: phiên đăng nhập game trên một thiết bị (gắn phiên Keycloak và `device_id`), giữ dấu vết token hiện hành/đã xoay (chỉ hash) và lý do thu hồi.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `access_token_ttl`, `game_session_sso` | TTL token, idle/max phiên game (7 ngày / 30 ngày), lịch sử hash token đã xoay |
| `game_login_ticket_ttl` | Hạn `login_ticket` |
| `login_rate`, `brute_force_lockout` | Rate limit thiết bị/username/toàn cục, khóa tạm |
| `keycloak_token_call_timeout` | Timeout gọi Keycloak |
| `game_login_body_logging` | Luôn tắt ghi body |
| `api_rate_default` | Rate limit `/auth/game/token:refresh` |
| `tenant_header_required` | Danh sách route miễn header |
| `login_capacity_target`, `login_slo` | Mục tiêu năng lực và SLO |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không gọi đồng bộ module khác trên đường đăng nhập (trừ login sync nhánh B của F011) | AGENTS R30 · ADR-0045 · ADR-0061 |
| Mật khẩu chỉ qua TLS, không log | NFR-SEC-002 · ADR-0068 |
| Rate limit theo thiết bị/username, không theo IP đơn lẻ | NFR-SEC-003 · AGENTS R30 |
| Mọi chuỗi request có `maxLength` | AGENTS R28 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Học sinh hợp lệ đăng nhập thành công với p95 ≤ ngưỡng `login_slo` của `auth/game/login` trên staging.
- **SC-002**: 0 lần xuất hiện mật khẩu mẫu trong mọi nguồn log/trace/crash sau bộ test bảo mật.
- **SC-003**: Ở tải gấp đôi `login_capacity_target`, 0 lỗi 5xx dây chuyền; mọi client vào được sau khi hết đỉnh.
- **SC-004**: 100% route tenant-scoped trả `400`/`403` đúng khi thiếu/sai `X-Tenant-Id` trong test sinh từ OpenAPI.

## Ngoài phạm vi

- Form S01, lưu refresh token trong kho bảo mật OS (adapter `SecureStorage` F060), luồng khởi động tự vào game, nút Đăng xuất, nhớ username, máy dùng chung, hàng chờ "Đang vào lớp…" phía Godot → F060/F061.
- Game session (`createGameSession`), một thiết bị, `409 ACTIVE_SESSION_ON_OTHER_DEVICE` + `takeover`, `endGameSession`, heartbeat timeout, login events game, client-config → F014 (ADR-0083).
- Cấu hình realm/client/flow Keycloak, brute-force realm, rate limit APISIX → F004.
- Phiên portal, `/me/*` → F012; login sync → F011; khôi phục học sinh bị dọn khi đăng nhập lại, use case đổi/đặt lại mật khẩu (gọi thu hồi phiên game của feature này) → F016.
- Đăng nhập nhanh (QR, PIN, mật khẩu hình ảnh), IdP trường, chọn tenant trước — mở rộng sau (ADR-0063).

## Assumptions

- F004 đã cấu hình client `eduworld-game-backend` (chỉ Direct Access Grants, rotation reuse 0, client session idle 7 ngày / max 30 ngày, realm SSO Session ≥ giá trị đó, không `offline_access`) và brute-force theo tài khoản.
- F014 phát `game_session.replaced` có `old_device_id`/`old_keycloak_sid`; consumer `revoke_on_session_replaced` của F014 (trong module identity) gọi `GameAuthSessionRevoker` để thu hồi phiên game của thiết bị cũ.
- Phân biệt lỗi role và lỗi mật khẩu từ Keycloak cần kiểm chứng (KEYCLOAK_SSO §3.1b); lớp backend luôn kiểm `user_type`.
- Certificate pinning là việc của client [Confirmed].
- ADR-0089 (làm rõ 2026-11-17) đã chốt hai câu hỏi trước đây: `identity.login_events` dùng mixin `TenantOptionalAppendOnly`, dòng tenant NULL ghi bằng `identity_platform`; đường đăng nhập đọc membership qua hàm `SECURITY DEFINER` `identity.memberships_for_user`, không cần `BYPASSRLS`.

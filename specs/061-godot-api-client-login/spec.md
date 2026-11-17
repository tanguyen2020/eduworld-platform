# Feature Specification: F061 — `ApiClient`, đăng nhập game trong form, chọn tenant, takeover, buộc cập nhật, đăng xuất

**Feature Branch**: `061-godot-api-client-login` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `godot/` · **Mốc**: M-C1 · **Phụ thuộc**: F013, F014, F060
**Input**: Lớp mạng REST (`ApiClient`) và luồng vào game của học sinh trong Godot: kiểm tra phiên bản, form đăng nhập S01 gửi `POST /auth/game/login`, chọn tenant sau đăng nhập, xác nhận takeover một thiết bị, giữ phiên bằng refresh token trong kho bảo mật của hệ điều hành và tự vào lại khi phiên còn hạn (ADR-0086; Web và máy dùng chung chỉ RAM), refresh có rotation, giữ game session bằng heartbeat, đăng xuất/thoát. FR nguồn: FR-AUTH-002, FR-AUTH-003, FR-AUTH-008, FR-AUTH-009, FR-AUTH-017, FR-AUTH-023, FR-AUTH-027, FR-GAME-008, FR-GAME-014.

## Clarifications

### Session 2026-11-16

- Q: Đăng nhập tạo game session luôn hay gọi `POST /game/sessions` riêng? → A: Gọi riêng — ADR-0083 (đóng CODE_PLAN §3 D15). `gameLogin`/`selectGameLoginTenant` chỉ trả `tokens.*` + `bootstrap` (không `game_session_id`); ngay sau đó `GameSessionService` gọi `createGameSession` (module game) với `X-Device-Id`, `X-Tenant-Id`, `Idempotency-Key` và lưu `game_session_id` từ `GameSession`.
- Q: Takeover hỏi lại mật khẩu không? → A: Không (AC-UX-0103, ADR-0083 mục 2). `409 ACTIVE_SESSION_ON_OTHER_DEVICE` chỉ đến từ `createGameSession`, lúc client đã có token; "Tiếp tục ở đây" gọi lại `createGameSession` với `takeover: true` và `Idempotency-Key` mới (body khác → key cũ bị `409 IDEMPOTENCY_KEY_REUSED`). Mật khẩu bị xóa khỏi RAM ngay khi `gameLogin` có phản hồi cuối, client không giữ mật khẩu cho takeover ([research.md](research.md) R1).
- Q: Đăng xuất có đóng game session ở identity không? → A: Không (ADR-0083 mục 3). Client gọi `endGameSession` trước rồi `gameLogout`; nếu không gọi được, module game đóng phiên theo heartbeat timeout.
- Q: Có nhớ tenant đã chọn trên thiết bị? → A: Không như một lựa chọn trên S01 (không chọn sẵn ở `TenantChoice`). Từ ADR-0086, `tenant_id` của phiên được lưu **cùng refresh token** trong `SecureStorage` chỉ để gửi `X-Tenant-Id` khi tự vào lại; đăng xuất xóa cả hai (xem Session ADR-0086).
- Q: Giữ phiên khi chưa có WebSocket (M-C1)? → A: Gọi `heartbeatGameSession` theo chu kỳ `ws_heartbeat`; server kết thúc phiên quá `ws_timeout` không heartbeat. Khi có `RealtimeClient` (F092), heartbeat WS thay thế.
- Q: Cấu hình OIDC trong `Config` (`oidc_issuer`, `oidc_client_id`)? → A: Không có (ADR-0068, CODE_PLAN §3 D16).

### Session 2026-11-16 (ADR-0086)

- Q: Mở lại game có phải nhập mật khẩu không? → A: Trên thiết bị cá nhân **không**, nếu phiên còn hạn: ADR-0086 thay ADR-0064 mục 1, 2, 4, 5. Sau đăng nhập, refresh token + `tenant_id` được lưu qua `SecureStorage` (F060: Windows DPAPI, Android Keystore; iOS Keychain ở F133). Khởi động: có bản ghi → `refreshGameToken` `mode=resume` + `device_id` → `200` (token mới + `bootstrap`) → `createGameSession` → vào game, không hiện form; thất bại xác định → xóa bản ghi, hiện form. Web và `shared_device = true` dùng `MemoryOnly`, giữ hành vi cũ (luôn hiện form).
- Q: Mã lỗi refresh? → A: `401 REFRESH_TOKEN_INVALID` (hết hạn/thu hồi), `401 REFRESH_TOKEN_REUSED` (dùng lại token đã xoay vòng — server thu hồi cả phiên), `401 REFRESH_TOKEN_DEVICE_MISMATCH` (sai thiết bị); thay `401 UNAUTHENTICATED` cũ ở `refreshGameToken` (contract do F013 sở hữu). Lỗi mạng/5xx/`429` khi resume giữ bản ghi và thử lại; `426` → ForceUpdate, giữ bản ghi.
- Q: Thoát game có đăng xuất không? → A: Thiết bị cá nhân: thoát = `endGameSession` (best effort), **giữ** refresh token; chỉ nút **Đăng xuất** (Cài đặt → Tài khoản) gọi `endGameSession` → `gameLogout` → xóa `SecureStorage`. Máy dùng chung: thoát = Đăng xuất. Tham số `game_mobile_background_relogin` bị bỏ (mobile mở lại dùng token đã lưu).
- Q: Refresh song song? → A: Single-flight bắt buộc; mọi token mới ghi đè `SecureStorage` ngay — gửi lại token cũ sẽ bị coi là dùng lại và cả phiên bị thu hồi.

### Session 2026-11-17

- Q: Màn "Tiếp tục là <nickname>?" khi tự vào lại và xử lý giờ giới nghiêm phía client (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Đăng nhập bằng form trong game (Priority: P1) 🎯 MVP

`hs.an.3a` mở game trên Windows/Android, thấy form trong game, nhập username + mật khẩu, vào thẳng tenant `th-demo-a` (US-AUTH-001, US-AUTH-002).

**Why this priority**: Cổng vào duy nhất của học sinh; exit M-C1.

**Independent Test**: Chạy game với backend compose, đăng nhập `hs.an.3a`, kiểm request, access token chỉ trong RAM, refresh token chỉ trong `SecureStorage` (không file rõ); thử sai mật khẩu, tài khoản nhân sự, `hs.nosis`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F061-AC-01 | KHI game khởi động, game client PHẢI gọi `getClientConfig` (`platform`, `version`, không token, không `X-Tenant-Id`) trước khi hiện form; NẾU `update_required = true` hoặc phiên bản client nhỏ hơn `min_client_version` THÌ PHẢI hiện màn ForceUpdate và không cho đăng nhập; NẾU `maintenance.active` THÌ PHẢI hiện màn bảo trì và kiểm tra lại định kỳ | FR-GAME-008 · [GODOT_ARCHITECTURE §5](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-AUTH-066 |
| F061-AC-02 | HỆ THỐNG PHẢI hiển thị form S01 vẽ bằng Godot chỉ gồm logo, chọn ngôn ngữ, ô tên đăng nhập, ô mật khẩu (`secret`, nút hiện/ẩn), nút "Đăng nhập" (tắt khi một ô trống), "Nhớ tên đăng nhập", "Quên mật khẩu? Liên hệ nhà trường để đặt lại"; không mở trình duyệt, Custom Tabs, WebView, không có bước chọn trường hay ô mã trường | FR-AUTH-002 · FR-AUTH-017 · ADR-0068 · [STUDENT_GAME_UX_CORE S01](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) | TC-AUTH-048 |
| F061-AC-03 | KHI học sinh bấm "Đăng nhập", `AuthService` PHẢI gửi `gameLogin` với body `GameLoginRequest` (`username` đã trim, `password`, `device_id`, `client_version`, `platform`, `locale`) qua HTTPS, không gửi `Authorization`, `X-Tenant-Id`, `Idempotency-Key`, và giới hạn độ dài ô nhập theo `maxLength` của schema | FR-AUTH-002 · ADR-0063 · ADR-0041 | TC-AUTH-001 |
| F061-AC-04 | KHI `gameLogin`/`selectGameLoginTenant` trả `200`, game client PHẢI giữ `tokens.access_token` chỉ trong RAM, ghi `tokens.refresh_token` + `bootstrap.tenant_id` vào `SecureStorage` (thiết bị cá nhân; `MemoryOnly` trên Web/máy dùng chung), lưu bootstrap trong `AppState` (RAM), xóa ô mật khẩu và bộ đệm mật khẩu, rồi gọi `createGameSession` (`X-Device-Id`, `X-Tenant-Id`, `Idempotency-Key` mới, body `GameSessionCreate`) và chỉ khi nhận `201` mới lưu `game_session_id` vào `AppState` và chuyển sang S02/S03; PHẢI không ghi mật khẩu hay access token xuống bất kỳ đâu và không ghi refresh token ngoài `SecureStorage` (`user://` dạng rõ, file cấu hình, localStorage) | FR-AUTH-008 · ADR-0086 · ADR-0065 · ADR-0083 | TC-AUTH-082 |
| F061-AC-05 | NẾU nhận `401 INVALID_CREDENTIALS`, THÌ game client PHẢI hiển thị một thông điệp chung (thêm gợi ý đợi từ lần sai thứ 3 trên thiết bị), xóa ô mật khẩu, đưa focus về ô mật khẩu và không tự thử lại | FR-AUTH-017 · [GODOT_ARCHITECTURE §9](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-AUTH-062 |
| F061-AC-06 | NẾU nhận `429 LOGIN_QUEUE_BUSY`, THÌ game client PHẢI hiện "Đang vào lớp…", tự gửi lại sau `Retry-After` cộng jitter tới khi thành công hoặc học sinh bấm Hủy; NẾU nhận `429 RATE_LIMITED`, THÌ PHẢI hiện đếm ngược theo `Retry-After` và không tự gửi | FR-AUTH-027 · ADR-0045 · [GAME_NETWORKING §3](../../docs/08-game-client/GAME_NETWORKING.md) | TC-AUTH-044 |
| F061-AC-07 | NẾU nhận `403 GAME_ACCESS_STUDENT_ONLY`, THÌ game client PHẢI hiện màn hướng dẫn dùng Admin Portal, không giữ token nào và quay về form khi bấm "Đăng nhập tài khoản khác" | FR-AUTH-017 · ADR-0031 | TC-AUTH-064 |
| F061-AC-08 | NẾU nhận `403 ACCOUNT_NOT_PROVISIONED`, THÌ game client PHẢI hiện "Tài khoản của em chưa được kích hoạt…", xóa mọi token trong RAM và không vào game | FR-AUTH-017 · ADR-0063 | TC-TEN-014 |
| F061-AC-09 | NẾU nhận `403` với `code` `ACCOUNT_LOCKED`, `ACCOUNT_DEACTIVATED`, `NO_ACTIVE_ENROLLMENT`, `TENANT_ACCESS_DISABLED`, `ACCESS_OUTSIDE_ALLOWED_HOURS`, `422 VALIDATION_FAILED` hoặc lỗi mạng, THÌ game client PHẢI hiện thông điệp từ khóa dịch `ui.error.<CODE>` theo ngôn ngữ đã chọn, không hiện `detail` từ server | [GODOT_ARCHITECTURE §9](../../docs/08-game-client/GODOT_ARCHITECTURE.md) · NFR-I18N-001 | TC-AUTH-006 |
| F061-AC-10 | HỆ THỐNG PHẢI không ghi mật khẩu, token, `login_ticket` hay body của `/auth/game/*` vào log, ring buffer, telemetry, crash report của client (`game_login_body_logging` tắt) | FR-AUTH-003 · ADR-0068 · AGENTS R14 | TC-AUTH-060 |
| F061-AC-11 | HỆ THỐNG PHẢI build ra artifact không chứa client secret, URL token endpoint Keycloak, `oidc_issuer`/`oidc_client_id`, intent filter OAuth callback | FR-AUTH-002 · ADR-0068 · AGENTS R3, R4 | TC-AUTH-010 |
| F061-AC-12 | Ở NƠI "Nhớ tên đăng nhập" được bật (`game_remember_username`) và thiết bị không phải `shared_device`, game client PHẢI chỉ lưu `last_username` vào `user://settings.cfg` và điền sẵn khi mở lại, ô mật khẩu luôn trống; TRONG KHI `device.cfg` có `shared_device: true`, PHẢI ẩn tùy chọn và không lưu username | FR-AUTH-008 · ADR-0086 | TC-AUTH-045 |

---

### User Story 2 — Chọn tenant sau đăng nhập (Priority: P1)

Học sinh thuộc hai tenant thấy "Em muốn vào trường nào hôm nay?" sau khi xác thực, chọn một trường, vào game không nhập lại mật khẩu (FR-AUTH-017).

**Why this priority**: Một phần bắt buộc của luồng đăng nhập ADR-0063.

**Independent Test**: Tài khoản thử thuộc 2 tenant (fixture backend F013) → `TenantChoice` → chọn; để quá hạn ticket rồi chọn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F061-AC-13 | KHI nhận `403 TENANT_NOT_SELECTED`, game client PHẢI hiện `TenantChoice` từ `choices[]` (tên, `school_name`, logo) và khi học sinh chọn PHẢI gọi `selectGameLoginTenant` với `login_ticket`, `tenant_id`, `device_id`, không gửi mật khẩu; ticket chỉ giữ trong RAM | FR-AUTH-017 · ADR-0063 · [KEYCLOAK_SSO §6.2](../../docs/09-identity/KEYCLOAK_SSO.md) | TC-TEN-014 |
| F061-AC-14 | NẾU `selectGameLoginTenant` trả `401 LOGIN_TICKET_INVALID` hoặc `403 TENANT_MISMATCH`, THÌ game client PHẢI hiện "Phiên chọn trường đã hết, em đăng nhập lại nhé" (hoặc thông điệp chung), xóa ticket và về form với ô mật khẩu trống | FR-AUTH-017 · `game_login_ticket_ttl` | TC-AUTH-065 |
| F061-AC-15 | HỆ THỐNG PHẢI không lưu lựa chọn tenant ngoài bản ghi phiên trong `SecureStorage` (dùng cho `X-Tenant-Id` khi tự vào lại), PHẢI không chọn sẵn tenant ở `TenantChoice` và không đổi tenant trong phiên (muốn đổi phải đăng xuất) | ADR-0086 · ADR-0065 | — |

---

### User Story 3 — `ApiClient` chuẩn cho mọi service phía client (Priority: P1)

Mọi service Godot (`QuestService`, `LearningService`…) gọi REST qua một `ApiClient` gắn header chuẩn, parse Problem Details, retry an toàn, tự refresh token.

**Why this priority**: Nền cho F062–F064 và mọi tính năng sau.

**Independent Test**: Unit test với `FakeHttp`; integration với backend compose gọi `getContentManifest` sau đăng nhập.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F061-AC-16 | `ApiClient` PHẢI gắn `Authorization: Bearer`, `X-Tenant-Id` = `bootstrap.tenant_id` (trừ `getClientConfig`, `gameLogin`, `selectGameLoginTenant`), `X-Device-Id`, `X-Client-Version`, `X-Client-Platform`, `Accept-Language`, `traceparent` mới mỗi request, và `Idempotency-Key` do caller cấp cho thao tác ghi quan trọng (giữ nguyên khi retry) | ADR-0065 · AGENTS R10 · [GAME_NETWORKING §2.1](../../docs/08-game-client/GAME_NETWORKING.md) | TC-TEN-011 |
| F061-AC-17 | `ApiClient` PHẢI chỉ tự retry lỗi mạng/timeout/502/503/504 với `GET` hoặc request có `Idempotency-Key`, backoff theo `client_reconnect_backoff` tối đa 3 lần; `429` (trừ hàng chờ đăng nhập) chờ `Retry-After` tối đa 2 lần; request ghi không có `Idempotency-Key` không bao giờ tự retry | [GAME_NETWORKING §2.3](../../docs/08-game-client/GAME_NETWORKING.md) | — |
| F061-AC-18 | KHI request trả `401` hoặc access token còn dưới 60 giây, `AuthService` PHẢI refresh single-flight qua `refreshGameToken` (`refresh_token`, `device_id`, `mode: "renew"`, header `X-Tenant-Id`), thay refresh token mới trong RAM và ghi đè `SecureStorage` rồi lặp lại request một lần; NẾU refresh trả `401 REFRESH_TOKEN_INVALID`, `REFRESH_TOKEN_REUSED` hoặc `REFRESH_TOKEN_DEVICE_MISMATCH` THÌ PHẢI xóa token (RAM + `SecureStorage`), phát `EventBus.session_expired` và về form | FR-AUTH-008 · ADR-0086 · `access_token_ttl` | TC-AUTH-002 |
| F061-AC-19 | NẾU nhận `400 TENANT_HEADER_REQUIRED` hoặc `403 TENANT_MISMATCH` ở route sau đăng nhập, THÌ game client PHẢI gửi telemetry `client_bug` (với `400`), xóa token và về form với "Em hãy đăng nhập lại" | ADR-0065 · [GAME_NETWORKING §2.3](../../docs/08-game-client/GAME_NETWORKING.md) | TC-TEN-011 |
| F061-AC-20 | `ApiClient` PHẢI parse lỗi theo Problem Details (`type`, `status`, `code`, `trace_id`, `errors[]`, trường mở rộng như `choices`, `other_device`, `min_client_version`) và trả kết quả có kiểu cho caller; UI ánh xạ theo `code`, không theo `title` | ADR-0010 · [GAME_NETWORKING §2.2](../../docs/08-game-client/GAME_NETWORKING.md) | — |

---

### User Story 4 — Một thiết bị và takeover (Priority: P1)

`hs.an.3a` đang chơi trên máy A, đăng nhập máy B → B hỏi xác nhận; đồng ý → A bị đăng xuất kèm thông báo (FR-AUTH-023).

**Why this priority**: [Confirmed] ADR-0031; exit M-C1 "takeover giữa hai thiết bị".

**Independent Test**: Hai instance Godot headless `--device-id` khác nhau cùng tài khoản.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F061-AC-21 | KHI `createGameSession` trả `409 ACTIVE_SESSION_ON_OTHER_DEVICE`, game client PHẢI hiện hộp xác nhận không tự đóng với `other_device` (nền tảng, nhãn thiết bị, lúc hoạt động), nút an toàn "Ở lại máy kia" bên trái; "Tiếp tục ở đây" PHẢI gọi lại `createGameSession` với `takeover: true`, `Idempotency-Key` mới và token hiện có, không hỏi và không gửi mật khẩu; "Ở lại máy kia" PHẢI gọi `gameLogout` (thu hồi token vừa cấp), xóa RAM và về form | FR-AUTH-023 · ADR-0031 · ADR-0083 · AC-UX-0103 | TC-AUTH-015 |
| F061-AC-22 | KHI `heartbeatGameSession` trả `409` (phiên đã bị thay) hoặc `404`, game client PHẢI dừng gameplay, hiện hộp "Em đã được đăng xuất vì tài khoản vừa đăng nhập trên thiết bị khác…" không tự đóng, xóa token (RAM + `SecureStorage` — phiên của thiết bị cũ đã bị server thu hồi), không tự kết nối lại và về form | FR-AUTH-023 · ADR-0086 · [GODOT_ARCHITECTURE §9](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-AUTH-015 |

---

### User Story 5 — Đăng xuất, thoát game và vòng đời phiên (Priority: P1)

Học sinh bấm "Đăng xuất" hoặc đóng cửa sổ; máy phòng tin học tự đăng xuất khi bỏ quên và luôn hỏi mật khẩu khi mở lại (FR-AUTH-009, FR-GAME-014, FR-AUTH-008).

**Why this priority**: An toàn tài khoản trên máy dùng chung.

**Independent Test**: Bấm Đăng xuất rồi mở lại; đóng cửa sổ trên máy cá nhân và máy `shared_device`; mô phỏng mobile ở nền; `shared_device` idle.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F061-AC-23 | KHI học sinh bấm "Đăng xuất" (Cài đặt → Tài khoản, sau xác nhận) — hoặc cửa sổ nhận `NOTIFICATION_WM_CLOSE_REQUEST` trên máy `shared_device` — game client PHẢI gọi `endGameSession` (best effort) **trước**, rồi `gameLogout` với `refresh_token` (identity không đóng game session), xóa mọi token/tenant/`game_session_id` trong RAM **và** bản ghi `SecureStorage`, rồi về form hoặc thoát | FR-AUTH-009 · FR-GAME-014 · ADR-0086 · ADR-0083 | TC-AUTH-088 |
| F061-AC-24 | TRONG KHI có game session và chưa có kết nối realtime, game client PHẢI gọi `heartbeatGameSession` (kèm `X-Device-Id`) mỗi `ws_heartbeat` để server không kết thúc phiên theo `ws_timeout` | FR-GAME-014 · [CONFIG_DEFAULTS](../../docs/00-overview/CONFIG_DEFAULTS.md) | TC-AUTH-017 |
| F061-AC-25 | KHI thiết bị `shared_device` không hoạt động quá `shared_device_idle_logout`, game client PHẢI đăng xuất như F061-AC-23 và về form; KHI ứng dụng mobile quay lại từ nền hoặc được mở lại sau khi bị hệ điều hành đóng, game client PHẢI tiếp tục bằng refresh token (RAM hoặc `SecureStorage`) mà không hỏi mật khẩu khi phiên còn hạn | FR-AUTH-008 · FR-GAME-014 · ADR-0086 | TC-AUTH-047 |
| F061-AC-26 | KHI cửa sổ nhận `NOTIFICATION_WM_CLOSE_REQUEST` trên thiết bị cá nhân, game client PHẢI chỉ gọi `endGameSession` (best effort) và PHẢI giữ bản ghi `SecureStorage`; TRONG KHI chạy Web hoặc `shared_device = true`, game mở lại PHẢI luôn bắt đầu ở form với ô mật khẩu trống, không dùng token cũ | FR-AUTH-008 · AC-UX-0101 · ADR-0086 | TC-AUTH-084 |
| F061-AC-27 | NẾU `gameLogin` trả `426 CLIENT_VERSION_UNSUPPORTED`, THÌ game client PHẢI hiện ForceUpdate với `min_client_version` và nút mở `store_url` (nếu có), không thử lại | FR-GAME-008 · [GODOT_ARCHITECTURE §9](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-AUTH-066 |
| F061-AC-28 | NẾU `createGameSession` sau đăng nhập trả lỗi khác `409 ACTIVE_SESSION_ON_OTHER_DEVICE` (`403 GAME_ACCESS_STUDENT_ONLY`, `426`, `503`, hoặc lỗi mạng sau khi `ApiClient` retry cùng `Idempotency-Key`), THÌ game client PHẢI hiện thông điệp `ui.error.<CODE>`, gọi `gameLogout`, xóa mọi token trong RAM và về form (hoặc ForceUpdate với `426`), không vào game khi chưa có `game_session_id` | FR-GAME-014 · FR-AUTH-023 · ADR-0083 | — |

---

### User Story 6 — Tự vào lại game khi phiên còn hạn (Priority: P1)

`hs.an.3a` chơi trên laptop cá nhân hôm thứ Hai, tắt game; thứ Tư mở lại → thấy "Đang vào lớp…" rồi vào thẳng Main Hub, không nhập mật khẩu. Sau 8 ngày không chơi (quá idle) hoặc sau khi nhà trường đặt lại mật khẩu → hiện form với thông báo phiên đã hết (ADR-0086).

**Why this priority**: ADR-0086 [Confirmed — Technical Architect]; giảm tải băm mật khẩu đầu tiết, giúp học sinh nhỏ.

**Independent Test**: Integration headless với backend compose: đăng nhập, thoát, mở lại → resume; tua thời gian phiên Keycloak (realm test idle ngắn) → form; dùng lại refresh token cũ từ tiến trình thứ hai → cả phiên bị thu hồi; đổi `device_id` → bị từ chối.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F061-AC-29 | KHI game khởi động trên thiết bị cá nhân, `getClientConfig` hợp lệ và `SecureStorage` có bản ghi phiên, game client PHẢI hiện màn chờ "Đang vào lớp…" (không hiện form) và gọi `refreshGameToken` với `refresh_token`, `device_id`, `client_version`, `mode: "resume"` và `X-Tenant-Id` = `tenant_id` đã lưu; KHI nhận `200`, PHẢI ghi đè refresh token mới vào `SecureStorage` trước mọi request khác, áp `bootstrap`, hiện màn xác nhận (F061-AC-35) rồi mới gọi `createGameSession` (409 → hộp takeover như F061-AC-21) rồi vào S02/S03 | FR-AUTH-008 · ADR-0086 · ADR-0083 · AC-UX-0101 | TC-AUTH-082 |
| F061-AC-30 | NẾU `refreshGameToken` (resume hoặc renew) trả `401 REFRESH_TOKEN_INVALID`, `401 REFRESH_TOKEN_REUSED`, `401 REFRESH_TOKEN_DEVICE_MISMATCH` hoặc `403` (`ACCOUNT_LOCKED`, `ACCOUNT_DEACTIVATED`, `NO_ACTIVE_ENROLLMENT`, `TENANT_ACCESS_DISABLED`, `TENANT_MISMATCH`, `GAME_ACCESS_STUDENT_ONLY`), THÌ game client PHẢI xóa bản ghi `SecureStorage` và token trong RAM, rồi hiện form S01 với thông điệp `ui.login.session_expired` (với `401`) hoặc `ui.error.<CODE>` (với `403`), tên đăng nhập điền sẵn nếu đã bật nhớ | FR-AUTH-008 · ADR-0086 | TC-AUTH-083 |
| F061-AC-31 | NẾU resume gặp lỗi mạng, `5xx` hoặc `429`, THÌ game client PHẢI giữ bản ghi `SecureStorage`, hiện "Đang kết nối…" và thử lại theo `client_reconnect_backoff`, kèm nút "Đăng nhập bằng tài khoản khác" (= Đăng xuất best effort rồi về form); NẾU resume trả `426 CLIENT_VERSION_UNSUPPORTED`, THÌ PHẢI hiện ForceUpdate và giữ bản ghi | ADR-0086 · FR-GAME-008 · AC-UX-0112 | — |
| F061-AC-32 | TRONG KHI `device.cfg` có `shared_device = true` hoặc chạy bản Web, game client PHẢI dùng `SecureStorage` `MemoryOnly`, PHẢI không để lại refresh token trong kho bảo mật, `user://` hay localStorage sau khi thoát, và PHẢI coi đóng cửa sổ/tải lại tab là Đăng xuất | ADR-0086 · FR-AUTH-008 | TC-AUTH-084 |
| F061-AC-33 | `AuthService` PHẢI refresh single-flight (mọi caller chờ chung một lời gọi) và PHẢI không bao giờ gửi lại refresh token đã được thay bằng token mới; KHI server trả `401 REFRESH_TOKEN_REUSED`, game client PHẢI gửi telemetry `auth_refresh_reused` (không kèm token) và xử lý như F061-AC-30 | ADR-0086 · FR-AUTH-008 | TC-AUTH-085 |
| F061-AC-34 | KHI phiên bị server thu hồi (đổi/đặt lại mật khẩu, khóa/xử phạt, thu hồi quyền, dọn học sinh, takeover, `device_id` khác), lần refresh/resume kế tiếp của game client PHẢI kết thúc ở form theo F061-AC-30 mà không vòng lặp thử lại | ADR-0086 · FR-AUTH-009 | TC-AUTH-087 |
| F061-AC-35 | KHI `refreshGameToken` `mode = resume` trả `200` (mọi nền tảng, `resume_confirm_screen`), game client PHẢI hiện màn S01b "Tiếp tục là <nickname>?" (nickname từ `bootstrap`, khóa dịch `ui.login.resume_confirm.*`) trước khi gọi `createGameSession`; "Tiếp tục" → `createGameSession`; "Không phải tôi" → `gameLogout`, xóa bản ghi `SecureStorage` và token trong RAM, hiện form S01 không điền sẵn tên | AC-AUTH-031 · ADR-0078 · ADR-0086 · [STUDENT_GAME_UX_CORE S01](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) | TC-AUTH-091 |
| F061-AC-36 | NẾU `gameLogin`, `refreshGameToken` (resume) hoặc `createGameSession` trả `403 CURFEW_ACTIVE`, THÌ game client PHẢI hiện `CurfewNotice` với giờ mở lại từ `reopen_at`, không hiện form và PHẢI giữ bản ghi `SecureStorage`; TRONG KHI đang chơi, client PHẢI cảnh báo trước `curfew_warning_minutes` theo khung giới nghiêm trong `bootstrap` và giờ server, rồi KHI nhận `session.session.ended` (`curfew`, close `4410`, không tự kết nối lại) hoặc tới giờ, PHẢI về `CurfewNotice` và giữ bản ghi `SecureStorage` | AC-AUTH-032 · AC-UX-0115 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 | TC-AUTH-092 |

### Edge Cases

- NẾU mất mạng khi đang đăng nhập, THÌ PHẢI hiện "Chưa có kết nối Internet" + "Thử lại", không lưu mật khẩu chờ gửi (F061-AC-09).
- NẾU học sinh bấm Hủy khi đang ở hàng chờ, THÌ PHẢI dừng retry và xóa mật khẩu khỏi bộ đệm (F061-AC-06).
- NẾU nhiều request cùng gặp `401`, THÌ chỉ một lần refresh (F061-AC-18).
- NẾU `choices[]` có hơn số thẻ hiển thị được, THÌ danh sách cuộn được và đọc được bằng trình đọc màn hình.
- NẾU Web export bị reload tab, THÌ coi như mở game mới và hiện form (F061-AC-32).
- NẾU `device_id` chưa có (lần đầu cài), THÌ sinh UUID và lưu `user://device_id` (Web: localStorage) trước khi gọi `gameLogin` (ADR-0031).
- NẾU `user://device_id` bị xóa nhưng bản ghi `SecureStorage` còn, THÌ resume trả `401 REFRESH_TOKEN_DEVICE_MISMATCH` → xóa bản ghi, hiện form (F061-AC-30, TC-AUTH-086).
- NẾU game bị kill ngay sau khi nhận token mới nhưng trước khi ghi `SecureStorage`, THÌ lần mở sau gửi token cũ → server coi là dùng lại, thu hồi phiên → form (F061-AC-30); chấp nhận được.
- NẾU ghi `SecureStorage` thất bại, THÌ tiếp tục phiên với RAM, lần mở sau hiện form (F060-AC-26).
- NẾU học sinh thuộc nhiều tenant và tự vào lại, THÌ dùng tenant của phiên đã lưu, không hiện `TenantChoice` (F061-AC-15).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-002 | Form trong game gửi `gameLogin`, không trình duyệt | [KEYCLOAK_SSO §6](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-003 | Không log/telemetry mật khẩu; xóa khỏi bộ nhớ | ADR-0068 |
| FR-AUTH-008 | Giữ phiên bằng refresh token trong `SecureStorage` (thiết bị cá nhân), tự vào lại khi còn hạn; access token chỉ RAM; Web/máy dùng chung chỉ RAM; refresh single-flight có rotation; nhớ username | ADR-0086 · [GODOT_ARCHITECTURE §5.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md#52-lưu-phiên-bằng-securestorage-adr-0086) |
| FR-AUTH-009 | Nút Đăng xuất: `endGameSession` → `gameLogout` → xóa token RAM + `SecureStorage` | [KEYCLOAK_SSO §7](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-017 | Form không chọn trường; chọn tenant sau đăng nhập; `ACCOUNT_NOT_PROVISIONED` | ADR-0063 |
| FR-AUTH-023 | `createGameSession` sau đăng nhập, hộp takeover (gọi lại `takeover: true`), thông báo bị thay | ADR-0031 · ADR-0083 |
| FR-AUTH-027 | Hàng chờ "Đang vào lớp…" tự thử lại | ADR-0045 |
| FR-GAME-008 | Kiểm tra phiên bản, buộc cập nhật | [GODOT_ARCHITECTURE §5](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |
| FR-GAME-014 | Nút Đăng xuất, đóng game kết thúc phiên, heartbeat | [GODOT_ARCHITECTURE §5](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |

### Key Entities

- **ClientConfig**: phiên bản tối thiểu, bảo trì, gợi ý nhớ username.
- **LoginRequest / LoginResult**: thông tin đăng nhập gửi đi và kết quả (token, bootstrap hoặc lỗi có mã); không có game session (ADR-0083).
- **TokenSet**: access/refresh token và hạn; access token chỉ RAM.
- **StoredSession**: bản ghi phiên trên thiết bị cá nhân (refresh token, tenant của phiên, thời điểm lưu) trong kho bảo mật của hệ điều hành; không có trên Web/máy dùng chung.
- **PlayerBootstrap**: tenant của phiên, hồ sơ, trường, lớp, policy, cờ tính năng.
- **TenantChoice**: tenant học sinh có thể chọn sau xác thực.
- **OtherDevice**: thiết bị đang giữ phiên khi takeover.
- **GameSession**: phiên chơi trên một thiết bị, tạo bằng `createGameSession` sau đăng nhập.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `access_token_ttl`, `game_session_sso` | Lịch refresh, hết phiên (idle/max phiên game, ADR-0086) |
| `game_remember_username` | Mặc định ô "Nhớ tên đăng nhập" |
| `shared_device_idle_logout` | Tự đăng xuất máy dùng chung |
| `game_login_ticket_ttl` | Thông điệp hết phiên chọn trường |
| `login_rate` | Hành vi khi `429` |
| `client_reconnect_backoff` | Backoff REST |
| `ws_heartbeat`, `ws_timeout` | Chu kỳ heartbeat game session khi chưa có WS |
| `tenant_header_required` | Danh sách route không gửi `X-Tenant-Id` |
| `game_login_body_logging` | Tắt log body đăng nhập |
| `login_slo` | Mục tiêu thời gian đăng nhập |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| GDScript static typing; UI không gọi HTTP; service `RefCounted`/autoload test được | ADR-0014 · AGENTS R1 · [GODOT_ARCHITECTURE §1](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |
| Chuỗi UI qua `locale/{vi,en}.po`, khóa `ui.login.*`, `ui.error.<CODE>` | NFR-I18N-001 · ADR-0056 |
| Theme từ `design/game-tokens`, không hardcode style trong `.tscn` | AGENTS R32 · ADR-0047 |
| Chỉ `https`; certificate pinning [Proposed] | [GAME_NETWORKING §9](../../docs/08-game-client/GAME_NETWORKING.md) |
| Từ bấm Đăng nhập tới phản hồi theo `login_slo` | NFR-PERF-019 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Học sinh 1 tenant vào được game sau một lần bấm "Đăng nhập" trên Windows và Android trong kịch bản quickstart.
- **SC-002**: Quét `user://`, thư mục ứng dụng, localStorage sau 10 phút chơi: 0 mật khẩu, 0 access token, 0 refresh token dạng rõ; trên thiết bị cá nhân refresh token chỉ có trong `SecureStorage`; máy `shared_device`/Web: 0 refresh token ở mọi nơi sau khi thoát.
- **SC-006**: Mở lại game trên thiết bị cá nhân khi phiên còn hạn: 100% lượt vào thẳng hub không hiện form trong kịch bản quickstart; phiên hết hạn/bị thu hồi: 100% về form.
- **SC-003**: Quét log client, crash report, telemetry với mật khẩu mẫu: 0 lần xuất hiện.
- **SC-004**: Takeover giữa hai thiết bị: thiết bị cũ hiện thông báo bị thay trong ≤ 1 chu kỳ heartbeat sau khi thiết bị mới xác nhận.
- **SC-005**: 100% route sau đăng nhập có `X-Tenant-Id`; 0 request đăng nhập có header này (kiểm bằng integration).

## Ngoài phạm vi

- Backend đăng nhập, ticket, refresh, logout, hàng chờ: F013; game session (`createGameSession`, `409`/takeover, end, heartbeat timeout), login events, client-config: F014.
- Khung project, autoload stub, theme, i18n, Prism mock, lớp `SecureStorage` + adapter Windows/Android/`MemoryOnly`: F060; adapter iOS Keychain và loại file phiên khỏi backup trong build release: F133.
- Phát hiện dùng lại refresh token, gắn `device_id`, thu hồi phía server: F013.
- `RealtimeClient`, nhận `session.session.replaced`/close 4409 qua WebSocket: F092 (F061 dùng heartbeat REST).
- Tạo avatar lần đầu (S02): F070; Main Hub: F062.
- Đăng nhập nhanh (QR, PIN, IdP trường): ngoài Release 1.0 (ADR-0063).
- Certificate pinning: việc còn mở GODOT_ARCHITECTURE §14.

## Assumptions

- Bộ đệm mật khẩu chỉ sống trong lúc request đăng nhập (kể cả hàng chờ `LOGIN_QUEUE_BUSY`) đang chạy; takeover không cần mật khẩu (ADR-0083, [research.md](research.md) R1).
- [Proposed] Dùng `ws_heartbeat`/`ws_timeout` cho heartbeat REST tới khi có tham số riêng.
- Contract `refreshGameToken` có `device_id`, `mode` (`renew`/`resume`), `GameTokenRefreshResponse.bootstrap` và các mã `REFRESH_TOKEN_*` theo ADR-0086 (F013 cập nhật OpenAPI).
- Prism mock (F060, nhãn MOCK) chỉ dùng khi backend F013/F014 chưa Done; feature chỉ Done khi chạy với backend thật.

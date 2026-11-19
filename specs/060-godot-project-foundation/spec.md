# Feature Specification: F060 — Khung project Godot: autoload, theme sinh từ token, i18n vi/en, CI, Prism mock

**Feature Branch**: `060-godot-project-foundation` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/game/` · **Mốc**: M-C0 · **Phụ thuộc**: F005
**Input**: Khung project Godot 4.x (GDScript static typing) theo GODOT_ARCHITECTURE: autoload nền, phân lớp Service ← Gameplay ← UI, Theme và mật độ theo grade band sinh từ `design/game-tokens/`, song ngữ vi-VN/en-US qua gettext, định dạng theo locale, pipeline CI headless, mock API có nhãn để làm client trước khi backend sẵn sàng, và kho bảo mật `SecureStorage` theo nền tảng để giữ phiên game (ADR-0086) — NFR-I18N-001, NFR-I18N-003, NFR-A11Y-001, NFR-MAINT-007.

## Clarifications

### Session 2026-11-16

- Q: File cấu hình môi trường: `res://config/<env>.json` (GODOT_ARCHITECTURE §4), `res://config/env/<env>.json` (KEYCLOAK_SSO §11) hay `src/game/config/env/local.cfg` (LOCAL_DEVELOPMENT §4)? → A: `res://config/env/<env>.json` (JSON, sinh lúc build theo `--env`) — xem [research.md](research.md) R1 [Proposed]; hai tài liệu còn lại sửa ở task tài liệu.
- Q: `Config` có khóa `oidc_issuer`, `oidc_client_id` (GODOT_ARCHITECTURE §4)? → A: Không. Game đăng nhập qua `POST /auth/game/login` (ADR-0068); gỡ dấu vết OIDC theo D16.
- Q: Mật độ (density) trước khi có bootstrap (chưa biết khối)? → A: Dùng `g1_3` (chữ/nút lớn nhất) cho màn trước đăng nhập để đạt NFR-A11Y-001; sau bootstrap theo `grade_band` [Proposed].
- Q: Autoload nào thuộc F060? → A: `Config`, `Log`, `EventBus`, `AppState`, `SceneRouter`, `AudioDirector` (khung âm lượng), `Settings`; `AuthService`, `ApiClient` → F061; `RealtimeClient` → F092; `ContentCache`, `QualityManager` → F062. Thứ tự giữ đúng bảng GODOT_ARCHITECTURE §4.

### Session 2026-11-16 (ADR-0086)

- Q: ADR-0086 cho game lưu refresh token trong kho bảo mật của hệ điều hành trên thiết bị cá nhân — lớp lưu trữ thuộc feature nào? → A: Interface `SecureStorage` + `SecureStorageFactory` + adapter Windows DPAPI, Android Keystore, `MemoryOnly` thuộc F060 (khung nền tảng, export Windows/Android có ở F060); adapter iOS Keychain thuộc F133 cùng pipeline iOS (macOS/Linux không phát hành trong Release 1.0 → `MemoryOnly`); dùng `SecureStorage` để tự vào lại, đăng xuất, máy dùng chung thuộc F061. Thay ràng buộc "không token trên đĩa" (ADR-0064) bằng: không mật khẩu, không access token trên đĩa; refresh token chỉ qua `SecureStorage` ([GODOT_ARCHITECTURE §5.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md#52-lưu-phiên-bằng-securestorage-adr-0086)).
- Q: `device.cfg` `shared_device = true` ảnh hưởng thế nào tới F060? → A: `SecureStorageFactory` trả `MemoryOnly` (chỉ RAM) cho `shared_device = true` và Web; F060 chỉ đọc khóa, hành vi đăng xuất ở F061.

### Session 2026-11-17

- Q: Windows mặc định máy dùng chung khi thiếu `device.cfg` (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

### Session 2026-11-19 (triển khai — Câu hỏi mở, dùng mặc định [Proposed])

- Q: `login_path` trong data-model là `/api/v1/auth/game/login` trong khi `api_base_url` đã gồm `/api/v1` (LOCAL_DEVELOPMENT §4)? → A: [Proposed] `login_path = /auth/game/login`, **tương đối với `api_base_url`**; F061 ghép hai giá trị.
- Q: Ba mức cỡ chữ (F060-AC-09) không có token riêng? → A: [Proposed] hệ số 1,0 / 1,175 / 1,35 lấy từ `UiTokens.NUMBER_RATIO_100` và `NUMBER_RATIO_135` (mức giữa là trung bình), áp lên bản sao của Theme sinh lúc chạy; xác nhận với UX ở S01 (F061).
- Q: `device.cfg` ở `user://` (spec) hay thư mục cài đặt (GODOT_ARCHITECTURE §5.1)? → A: [Proposed] đường dẫn là project setting `eduworld/device/config_path` (mặc định `user://device.cfg` theo spec); bộ cài F133 có thể trỏ sang thư mục cài đặt chỉ admin ghi. Mặc định `shared_device_default` theo nền tảng nằm ở project setting `eduworld/device/shared_device_default/<platform>`.
- Q: Máy dev không có toolchain C++/Android — adapter native kiểm ở đâu? → A: [Proposed] GDExtension DPAPI (godot-cpp tag `godot-4.5-stable`) và AAR Keystore build trong `godot.yml` (job `platform-windows` chạy bộ test `SecureStorage` với DPAPI thật trên runner Windows; job `platform-android`). Checkout không có DLL/AAR → `is_available() = false`, factory trả `MemoryOnly` (không có đường lưu dạng rõ). Phần lưu file (ghi tạm + đổi tên, bản ghi hỏng, log không kèm giá trị) test bằng cipher giả `tests/fixtures/fake_cipher.gd`. Test trên emulator Android (phần Android của F060-AC-23/25/26) chờ lane thiết bị của F133.
- Q: Baseline ảnh `UiGallery` khác nhau giữa GPU/OS? → A: [Proposed] baseline theo nền tảng `tests/scene/baselines/<os>/`; nền tảng chưa có baseline thì `tools/screenshot_gallery.gd` ghi ảnh ứng viên, báo "baseline missing" và CI tải artifact để duyệt (như F050).
- Q: Domain dev/staging/production chưa chốt (OQ-10)? → A: [Proposed] `config/env/{dev,staging,production}.json` dùng placeholder `*.eduworld.example` như `servers` của OpenAPI; thay khi OQ-10 có câu trả lời. Prism phục vụ path không có tiền tố `/api/v1` nên `mock.json` dùng `http://localhost:4011`.
- Q: Ký bản Android trong CI PR? → A: [Proposed] `godot.yml` export Android **debug-signed** (không cần secret); ký release bằng keystore từ GitHub Environment thuộc `release.yml`/F133.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Project Godot đúng kiến trúc lớp, cấu hình không chứa secret (Priority: P1)

Godot dev mở project đúng phiên bản ghim, có autoload nền theo thứ tự chuẩn, scene khởi động, cấu hình môi trường an toàn và lint chặn UI gọi mạng.

**Why this priority**: G-1…G-6, R1, R2, ADR-0014; F061–F064 dựng trên khung này.

**Independent Test**: Chạy headless `Boot.tscn` với `--env=local`; chạy lint typed + lint lớp; quét artifact export.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F060-AC-01 | `src/game/project.godot` PHẢI dùng phiên bản trong `src/game/.godot-version` và bật `debug/gdscript/warnings/untyped_declaration = error`; NẾU script có khai báo không typed, THÌ CI PHẢI fail | ADR-0014 · [GODOT_ARCHITECTURE §1](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | — |
| F060-AC-02 | Autoload PHẢI được khai báo theo thứ tự GODOT_ARCHITECTURE §4 cho các thành phần của feature này (`Config`, `Log`, `EventBus`, `AppState`, `SceneRouter`, `AudioDirector`, `Settings`), và `res://scenes/bootstrap/Boot.tscn` PHẢI khởi động được ở chế độ headless | GODOT_ARCHITECTURE §4 · NFR-MAINT-007 | — |
| F060-AC-03 | `Config` PHẢI đọc `res://config/env/<env>.json` theo tham số `--env`, PHẢI KHÔNG chứa secret hay khóa `oidc_issuer`/`oidc_client_id`; NẾU artifact export chứa client secret, URL token endpoint Keycloak hoặc khóa OIDC, THÌ CI PHẢI fail | ADR-0068 · AGENTS R3, R4 | TC-AUTH-010 |
| F060-AC-04 | `Log` PHẢI ghi log có cấu trúc vào ring buffer và PHẢI lọc token, mật khẩu, header `Authorization`, họ tên trước khi ghi | AGENTS R14 · GODOT_ARCHITECTURE §4 | — |
| F060-AC-05 | NẾU script trong `scenes/ui/**` hoặc script gameplay tạo `HTTPRequest`, `HTTPClient`, `WebSocketPeer` hoặc gọi trực tiếp API, THÌ lint lớp PHẢI fail | AGENTS R1 · GODOT_ARCHITECTURE §1 (G-4), §6 | — |
| F060-AC-06 | `EventBus` PHẢI chỉ khai báo signal trong danh sách cố định của `event_bus.gd`, tên thì quá khứ, có kiểu tham số; NẾU tên signal không đúng quy ước, THÌ lint PHẢI fail | GODOT_ARCHITECTURE §7 · AGENTS §7 | — |

---

### User Story 2 — Giao diện theo Theme sinh từ token, mật độ theo khối (Priority: P1)

Học sinh lớp 1–3 thấy chữ và nút đủ lớn; mọi màn lấy style từ Theme sinh; không ai override màu/font trong scene.

**Why this priority**: NFR-A11Y-001, NFR-MAINT-007, R32.

**Independent Test**: Mở `UiGallery.tscn` ở 4 density, chụp ảnh so baseline; thêm override màu vào `.tscn` → lint đỏ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F060-AC-07 | Mọi `Control` PHẢI lấy style từ `res://resources/ui/theme/eduworld_theme*.tres` sinh bởi F005; NẾU `.tscn` override màu, font hoặc cỡ chữ ngoài allowlist, THÌ `src/game/tools/lint_theme_overrides.gd` PHẢI fail | NFR-MAINT-007 · AGENTS R32 · [DESIGN_SYSTEM §5](../../docs/05-ux/DESIGN_SYSTEM.md) | TC-UI-003 |
| F060-AC-08 | `ThemeService` PHẢI áp biến thể density theo `AppState.player.grade_band`; TRONG KHI chưa có bootstrap, HỆ THỐNG PHẢI dùng density `g1_3`; KHI density đổi, mọi màn đang mở PHẢI cập nhật mà không khởi động lại | NFR-A11Y-001 · DESIGN_SYSTEM §5.2 | — |
| F060-AC-09 | `Settings` PHẢI lưu cỡ chữ (3 mức), tương phản cao, giảm chuyển động, ngôn ngữ, âm lượng vào `user://settings.cfg` và PHẢI KHÔNG lưu dữ liệu nghiệp vụ hay token | NFR-A11Y-001 · GODOT_ARCHITECTURE §10 | — |
| F060-AC-10 | Scene `res://scenes/ui/debug/UiGallery.tscn` (chỉ build debug) PHẢI hiển thị component nền ở 4 density, tương phản cao và locale `vi`, `en`, `xx-PS`; CI PHẢI chụp ảnh và so baseline | DESIGN_SYSTEM §9 · NFR-A11Y-001 | TC-UI-001 |
| F060-AC-11 | NẾU `src/game/` chứa thư viện, Theme hoặc token của Admin UI web, THÌ CI PHẢI fail | AGENTS R32 · ADR-0047 | TC-UI-010 |

---

### User Story 3 — Song ngữ vi/en và chịu chuỗi dài (Priority: P1)

Học sinh chọn tiếng Việt hoặc tiếng Anh, đổi ngay khi đang chơi; số, ngày, giờ hiển thị đúng locale; giao diện không vỡ khi chuỗi dài.

**Why this priority**: NFR-I18N-001 [Confirmed] (ADR-0056), NFR-I18N-003.

**Independent Test**: Đổi locale trong `UiGallery`; chạy `extract_pot.gd`; unit test `LocaleFormat`; ảnh pseudo-locale.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F060-AC-12 | Chuỗi UI tĩnh PHẢI qua gettext `res://locale/vi.po`, `en.po` với khóa `ui.<screen>.<element>[.<state>]`; mã lỗi API PHẢI ánh xạ khóa `ui.error.<CODE>`, thiếu khóa dùng `ui.error.generic` và gửi telemetry `i18n.missing_key` (không hiện khóa thô) | NFR-I18N-001 · ADR-0056 · [GODOT_ARCHITECTURE §10.1](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | — |
| F060-AC-13 | NẾU khóa trong `messages.pot` (sinh bởi `src/game/tools/extract_pot.gd`) thiếu ở `vi.po` hoặc `en.po`, THÌ CI PHẢI fail | NFR-I18N-001 | TC-I18N-005 |
| F060-AC-14 | KHI người dùng đổi ngôn ngữ, HỆ THỐNG PHẢI gọi `TranslationServer.set_locale()`, phát `EventBus.locale_changed` và cập nhật màn đang mở mà không khởi động lại | NFR-I18N-001 · ADR-0056 | TC-I18N-004 |
| F060-AC-15 | Locale PHẢI chỉ thuộc `supported_locales`; mặc định trước bootstrap là `vi-VN`, sau bootstrap theo hồ sơ học sinh rồi `tenant_default_locale` | NFR-I18N-001 · ADR-0056 | — |
| F060-AC-16 | `LocaleFormat` PHẢI định dạng số, ngày, giờ, thời lượng theo bảng GODOT_ARCHITECTURE §10.1 cho `vi-VN` và `en-US`, không phụ thuộc locale hệ điều hành | NFR-I18N-003 | TC-I18N-007 |
| F060-AC-17 | Build debug PHẢI có pseudo-locale `xx-PS` kéo dài chuỗi; KHI chụp các màn của khung ở `vi`, `en`, `xx-PS`, PHẢI KHÔNG có chuỗi hardcode và PHẢI KHÔNG tràn/cắt chữ ngoài ellipsis có tooltip theo `text_expansion_allowance` | NFR-I18N-003 · DESIGN_SYSTEM §5.5 | TC-I18N-006 |
| F060-AC-18 | Font trong Theme PHẢI hiển thị đủ tiếng Việt có dấu và Latin mở rộng; test render chuỗi mẫu PHẢI không có glyph thiếu | NFR-I18N-003 · NFR-A11Y-001 | — |

---

### User Story 4 — CI headless và mock API có nhãn (Priority: P2)

Godot dev có pipeline tự động (lint, test, export Windows/Android, smoke) và làm được màn hình trên mock sinh từ spec khi backend chưa sẵn sàng, mà không lọt mock vào bản phát hành.

**Why this priority**: CODE_PLAN §2 cho phép game client chạy trên Prism mock có nhãn (R17); F061 cần ngay.

**Independent Test**: Chạy `make up` profile `mock`, boot game với `--env=mock`; build release và quét `mock`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F060-AC-19 | KHI PR thay đổi `src/game/**`, `specs/**`, `design/game-tokens/**` hoặc `design/shared-tokens/**`, workflow `.github/workflows/godot.yml` (GitHub Actions, runner Linux) PHẢI chạy theo thứ tự: `design-tokens-check` → gdlint + typed check + lint override theme + lint lớp → GdUnit4 headless → export Windows và Android → smoke boot | [GODOT_ARCHITECTURE §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) · DEPLOYMENT §10.2 · ADR-0088 | — |
| F060-AC-20 | Compose profile `mock` PHẢI chạy Prism từ `specs/openapi/eduworld-api.v1.yaml`; cấu hình `res://config/env/mock.json` PHẢI có `is_mock: true` và game PHẢI hiển thị nhãn "MOCK" cố định trên mọi màn khi dùng cấu hình này | AGENTS R17 · CODE_PLAN §2 | — |
| F060-AC-21 | NẾU preset export phát hành chứa `mock.json`, cờ `is_mock` hoặc URL mock, THÌ CI PHẢI fail | AGENTS R17 · CODE_PLAN §2 | — |
| F060-AC-22 | Test contract PHẢI parse mọi response mẫu từ `specs/openapi` và `specs/examples` mà client sẽ dùng thành model GDScript không lỗi | GODOT_ARCHITECTURE §12 · NFR-MAINT-002 | — |

---

### User Story 5 — Kho bảo mật theo nền tảng để giữ phiên game (Priority: P1)

Học sinh dùng máy cá nhân không phải nhập mật khẩu mỗi lần mở game: refresh token được giữ trong kho bảo mật của hệ điều hành; máy phòng tin học và bản Web không giữ gì xuống đĩa.

**Why this priority**: ADR-0086; F061 (tự vào lại, đăng xuất) cần lớp lưu trữ này.

**Independent Test**: Unit test từng adapter trên runner Windows và thiết bị/emulator Android: `save` → `load` trả đúng, file trên đĩa chỉ là ciphertext; `shared_device = true` → `MemoryOnly`; bản ghi hỏng → `load` rỗng, không crash.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F060-AC-23 | `SecureStorageFactory.create()` PHẢI trả adapter theo nền tảng: Windows → DPAPI (`CryptProtectData`, scope CurrentUser), Android → Android Keystore (khóa AES-GCM không xuất được, ciphertext trong bộ nhớ riêng của app), Web hoặc nền tảng chưa có adapter → `MemoryOnly`; mọi adapter PHẢI hiện thực cùng interface `is_available`/`save`/`load`/`erase` | ADR-0086 · [GODOT_ARCHITECTURE §5.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md#52-lưu-phiên-bằng-securestorage-adr-0086) | TC-AUTH-082 |
| F060-AC-24 | KHI `user://device.cfg` có `shared_device = true`, `SecureStorageFactory` PHẢI trả `MemoryOnly`, và sau khi tiến trình thoát PHẢI KHÔNG còn bản ghi nào trong kho bảo mật, `user://` hay localStorage | ADR-0086 | TC-AUTH-084 |
| F060-AC-25 | Adapter native PHẢI chỉ ghi ciphertext xuống đĩa; giá trị rõ PHẢI KHÔNG xuất hiện trong `user://`, `settings.cfg`, `device.cfg` hay ring buffer `Log` | ADR-0086 · AGENTS R14 | TC-AUTH-084 |
| F060-AC-26 | NẾU bản ghi không giải mã được (file hỏng, khóa Keystore bị xóa, user Windows khác), THÌ `load` PHẢI trả rỗng kèm mã lỗi mà không crash, và `erase` PHẢI xóa bản ghi; NẾU `save` thất bại, THÌ adapter PHẢI trả lỗi và ghi `secure_storage_write_failed` vào `Log` (không kèm giá trị) | ADR-0086 | — |
| F060-AC-27 | Ở NƠI game chạy trên Windows và `user://device.cfg` không có khóa `shared_device` (thiếu file hoặc thiếu khóa), `Config` PHẢI coi `shared_device = true` theo `shared_device_default` (nên `SecureStorageFactory` trả `MemoryOnly`); Android, iOS mặc định `false`; Web luôn `MemoryOnly` | AC-AUTH-030 · ADR-0078 · ADR-0086 | TC-AUTH-090 |

### Edge Cases

- NẾU `--env` không hợp lệ hoặc file cấu hình thiếu khóa bắt buộc, THÌ `Boot` hiển thị màn lỗi cấu hình (dịch) và không tiếp tục (F060-AC-03).
- NẾU Theme sinh không tồn tại (chưa chạy `make design-tokens`), THÌ CI fail ở bước `design-tokens-check` (F060-AC-19).
- NẾU đổi ngôn ngữ khi một hội thoại/câu hỏi đang hiển thị, THÌ nội dung đó giữ ngôn ngữ cũ tới mục tiếp theo (áp dụng khi F063 có; khung chỉ phát `locale_changed`) (F060-AC-14).
- Authorization, tenant isolation, idempotency, Problem Details từ server: thuộc `ApiClient`/`AuthService` (F061); khung chỉ có ánh xạ khóa `ui.error.<CODE>` (F060-AC-12).
- NẾU người dùng sao chép thư mục `user://` sang máy hoặc tài khoản Windows khác, THÌ bản ghi `SecureStorage` không giải mã được → `load` rỗng (F060-AC-26), F061 hiện form đăng nhập.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| NFR-I18N-001 | Gettext vi/en, khóa dịch, đổi ngôn ngữ lúc chạy, kiểm khóa trong CI | GODOT_ARCHITECTURE §10.1, ADR-0056 |
| NFR-I18N-003 | `LocaleFormat`, pseudo-locale, font phủ tiếng Việt, bố cục chịu chuỗi dài | GODOT_ARCHITECTURE §10.1, DESIGN_SYSTEM §5.5 |
| NFR-A11Y-001 | Density theo grade band (mặc định g1_3 trước bootstrap), cỡ chữ/tương phản cao/giảm chuyển động | DESIGN_SYSTEM §5.2, GODOT_ARCHITECTURE §10 |
| NFR-MAINT-007 | Theme sinh từ token, lint override theme | DESIGN_SYSTEM §5, §9 |

### Key Entities

- **Cấu hình môi trường**: URL API/realtime/CDN, phiên bản client, cờ mock; không secret.
- **Trạng thái ứng dụng (AppState)**: player bootstrap, policies, feature flags, connectivity.
- **Cài đặt người dùng**: ngôn ngữ, cỡ chữ, tương phản, chuyển động, âm lượng (theo thiết bị).
- **Khóa dịch**: định danh chuỗi UI bất biến, bản dịch vi/en.
- **Kho bảo mật (SecureStorage)**: nơi giữ bí mật của phiên game trên thiết bị (refresh token, tenant của phiên) bằng cơ chế của hệ điều hành; chỉ RAM trên máy dùng chung và Web.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `supported_locales` | Danh sách locale hợp lệ |
| `tenant_default_locale` | Locale mặc định sau bootstrap |
| `text_expansion_allowance` | Kiểm bố cục chịu chuỗi dài |
| `shared_device_idle_logout` | Khóa cấu hình thiết bị `device.cfg` (đọc ở F060, dùng ở F061); `shared_device = true` → `SecureStorage` `MemoryOnly` |
| `lan_cache_url` | Khóa `device.cfg` (đọc ở F060, dùng ở F062) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| UI/gameplay không gọi HTTP; không business logic trong scene | AGENTS R1 · G-4 |
| Không hardcode nội dung học tập/chuỗi UI trong scene | AGENTS R2, §7 |
| Không mật khẩu, không access token trên đĩa; refresh token chỉ qua `SecureStorage` (thiết bị cá nhân), chỉ RAM trên máy dùng chung/Web | ADR-0086 |
| Mock phải gắn nhãn, không vào bản phát hành | AGENTS R17 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% chuỗi hiển thị của khung đi qua file dịch; 0 khóa thiếu ở `vi`/`en`.
- **SC-002**: Đổi ngôn ngữ cập nhật màn đang mở trong ≤ 1 giây, không khởi động lại.
- **SC-003**: 0 override màu/font trong `.tscn` ngoài allowlist; 0 script UI gọi mạng.
- **SC-004**: 0 artifact phát hành chứa secret, khóa OIDC hay cấu hình mock.
- **SC-005**: Cỡ chữ cơ sở màn trước đăng nhập đạt mức NFR-A11Y-001.
- **SC-006**: 0 giá trị bí mật dạng rõ trên đĩa sau bộ test `SecureStorage` (Windows, Android); 0 bản ghi trên máy `shared_device`.

## Ngoài phạm vi

- `ApiClient`, `AuthService`, form đăng nhập S01, chọn tenant, takeover → F061; `RealtimeClient` → F092.
- `ContentCache`, `QualityManager`, Main Hub, benchmark cảnh (TC-PERF-018) → F062; activity renderer → F063.
- Export iOS/Web hoàn chỉnh, ký code, phát hành store → F133; adapter `SecureStorage` iOS Keychain, loại file phiên khỏi Auto Backup trong manifest release → F133.
- Dùng `SecureStorage` cho tự vào lại, refresh, đăng xuất, máy dùng chung → F061.

## Assumptions

- Godot 4.x bản stable tại Phase 1 (ghim minor), GdUnit4, gdtoolkit (`gdlint`) [Proposed — ADR-0014].
- Đầu ra Theme/`UiTokens` của F005 có sẵn; F060 không sửa file sinh.
- Prism (`stoplight/prism:4`, xem LOCAL_DEVELOPMENT §3.1) chạy trong compose profile `mock` cổng `4011` [Proposed]; dữ liệu mock chỉ là ví dụ trong spec.

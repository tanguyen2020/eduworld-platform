# UX màn hình học sinh — Core (S01–S07)

> Trạng thái: Draft · Phiên bản: 1.4 · Owner: UX/UI Designer + Game Director · Cập nhật: 2026-11-24 (S01: bố cục ba cột cụ thể của form, kích thước theo token, ngôn ngữ dạng chip VI/EN, hiện/ẩn mật khẩu bằng icon, thông báo có icon, Mây chào — owner báo giao diện vỡ) · 2026-11-24 (ADR-0101: bỏ Windows, client chỉ Web/Android/iOS) · 2026-11-17 (S01: màn "Tiếp tục là <nickname>?" khi tự vào lại, máy dùng chung, màn giờ giới nghiêm ADR-0078; bỏ đăng nhập QR/PIN — ADR-0076 Rejected) · 2026-11-16 (ADR-0083, ADR-0086)
> Liên quan: [STUDENT_GAME_UX (quy ước chung)](STUDENT_GAME_UX.md) · [ADR-0083](../adr/ADR-0083-dang-nhap-game-va-tao-game-session-tach-biet.md) · [ADR-0086](../adr/ADR-0086-giu-phien-game-bang-refresh-token.md) · [DESIGN_SYSTEM §5](DESIGN_SYSTEM.md#5-game-design-system-godot-3d) · [ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md) · [ADR-0057](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md) · [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md)

## Mục đích và phạm vi

Đặc tả UX chi tiết 7 màn cốt lõi của học sinh: S01 Đăng nhập, S02 Tạo avatar lần đầu, S03 Main Hub HUD, S04 World map, S05 Quest journal, S06 Challenge/Boss, S07 Level-up. Mọi màn tuân theo quy ước chung (nền tảng, HUD, điều khiển, song ngữ, hiệu năng, trạng thái, trợ năng, mẫu đặc tả) trong [STUDENT_GAME_UX](STUDENT_GAME_UX.md); tài liệu này không lặp lại các quy ước đó. Nghiệp vụ (quy tắc, tham số) thuộc tài liệu nghiệp vụ được link ở từng màn.

Ký hiệu grade band: `g1_3`, `g4_5`, `g6_9` (`g6_7`, `g8_9` dùng chung bố cục, khác mật độ theo [DESIGN_SYSTEM §5.2](DESIGN_SYSTEM.md#52-mật-độ-theo-grade-band)).

---

## S01 — Đăng nhập

**1. Mục đích.** Đưa học sinh vào game bằng **username + password** do EduWorld cấp (ADR-0063), chọn ngôn ngữ (ADR-0056), xác nhận khi tài khoản đang dùng trên thiết bị khác (ADR-0031). **Trên thiết bị cá nhân, phiên đăng nhập được giữ** (refresh token trong kho bảo mật của hệ điều hành, [ADR-0086](../adr/ADR-0086-giu-phien-game-bang-refresh-token.md)): mở lại game khi phiên còn hạn (idle 7 ngày, tối đa 30 ngày) thì hiện màn **"Tiếp tục là <nickname>?"** rồi vào game, **không hiện form** (mọi nền tảng, [Confirmed — Technical Architect 2026-11-17]); phiên hết hạn/bị thu hồi, học sinh đã bấm Đăng xuất hoặc "Không phải tôi", bản Web hoặc máy dùng chung (`shared_device`) → hiện form và nhập mật khẩu. PC phòng máy chơi bản Web nên luôn hiện form khi mở lại (không có bản Windows, [ADR-0101](../adr/ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md)). Không có checkbox "Ghi nhớ đăng nhập" — việc giữ phiên do loại thiết bị quyết định; vẫn có "Nhớ tên đăng nhập". **Học sinh không chọn trường**: sau khi đăng nhập thành công, backend tự xác định tenant theo membership (ADR-0063 mục 3) — 1 tenant → vào thẳng; nhiều tenant → chọn **sau** đăng nhập; không thuộc tenant chung và không thuộc tenant SIS nào → không cho đăng nhập (`ACCOUNT_NOT_PROVISIONED`).

QR lớp + PIN, thẻ QR, mật khẩu hình ảnh **không làm** (ADR-0076 Rejected 2026-11-17); học sinh lớp 1–3 dùng username/mật khẩu ngắn do trường in phát và màn "Tiếp tục là…" trên thiết bị cá nhân để giảm số lần gõ. Mã thiết bị, SSO Google/Microsoft qua email trường và ô "Nhập mã trường" ngoài Release 1.0 (ADR-0063 mục 1) — không hiển thị trên UI; thiết kế giữ ở mục "Mở rộng sau" của [KEYCLOAK_SSO §8](../09-identity/KEYCLOAK_SSO.md).

**2. Người dùng & khối.** Mọi học sinh; nhân sự nhà trường và vận hành bị chặn bằng thông điệp `GAME_ACCESS_STUDENT_ONLY`.

**3. Điểm vào / ra.** Vào: mở ứng dụng khi không có phiên đã lưu (lần đầu, Web, máy dùng chung), sau khi bấm **Đăng xuất** (Cài đặt → Tài khoản, [STUDENT_GAME_UX_SOCIAL §10](STUDENT_GAME_UX_SOCIAL.md#10-cài-đặt)), phiên hết hạn/bị thu hồi (kể cả khi tự vào lại thất bại), bị đăng xuất do thiết bị khác. Mở ứng dụng khi còn phiên đã lưu → màn chờ "Đang vào lớp…" → màn **S01b "Tiếp tục là <nickname>?"** → S02/S03, không qua form. Ra: S02 (chưa có avatar) hoặc S03.

Màn là **một form vẽ bằng Godot** (ADR-0068): logo, chọn ngôn ngữ, ô tên đăng nhập, ô mật khẩu (ẩn ký tự, nút hiện/ẩn), nút "Đăng nhập", ô "Nhớ tên đăng nhập trên máy này", dòng "Quên mật khẩu? Liên hệ nhà trường để đặt lại". Game gửi username/password tới `POST /api/v1/auth/game/login` của module identity (TLS); module identity xác thực với Keycloak. **Không** mở trình duyệt hệ thống, Custom Tabs, `ASWebAuthenticationSession`, WebView hay chuyển ứng dụng; không có trang Keycloak cho học sinh.

Trạng thái con (khớp [GODOT_ARCHITECTURE §5](../08-game-client/GODOT_ARCHITECTURE.md#5-vòng-đời-ứng-dụng)): `CheckVersion` → `ResumingSession` (**chỉ khi** có phiên đã lưu: splash "Đang vào lớp…", gọi `refreshGameToken` `mode=resume`; thành công → `ResumeConfirm` (S01b "Tiếp tục là <nickname>?": "Tiếp tục" → `CreatingSession`; "Không phải tôi" → đăng xuất, xóa phiên đã lưu → `LoginForm`), thất bại xác định → `LoginForm` kèm thông báo, giờ giới nghiêm → `Curfew`) → `LoginForm` → `SigningIn` (đang gọi `/auth/game/login`, có thể xếp hàng) → `TenantChoice` (**chỉ khi** `TENANT_NOT_SELECTED`) → `CreatingSession` (gọi `createGameSession` ngay sau khi có token — đăng nhập không tự tạo phiên chơi, [ADR-0083](../adr/ADR-0083-dang-nhap-game-va-tao-game-session-tach-biet.md)) → `ConfirmTakeover` (chỉ khi `createGameSession` trả 409) → vào game.

**4. Bố cục Web/PC**

```text
Tự vào lại khi còn phiên (thiết bị cá nhân, không có form)
┌───────────────────────────────────────────────────────────────────────────┐
│ (Ảnh minh họa Quảng trường Tia Sáng)                                      │
│                         [Logo EduWorld]                                   │
│                      Đang vào lớp… ◌                                      │
│                                                                           │
│          (sau 5 s chưa xong) [Đăng nhập bằng tài khoản khác]              │
└───────────────────────────────────────────────────────────────────────────┘

S01b — Tiếp tục là <nickname>? (sau khi tự vào lại thành công, mọi nền tảng)
┌───────────────────────────────────────────────────────────────────────────┐
│ (Ảnh minh họa Quảng trường Tia Sáng)                                      │
│                    [Avatar 3D nhỏ của học sinh]                           │
│                  Tiếp tục là MèoXanh42?                                   │
│        [      Tiếp tục      ] (chính)     [ Không phải tôi ]              │
└───────────────────────────────────────────────────────────────────────────┘

Giờ giới nghiêm (21:30–06:30 hoặc khung chặt hơn của trường)
┌───────────────────────────────────────────────────────────────────────────┐
│                [Mây ngủ dưới trăng]                                       │
│      Đã đến giờ nghỉ rồi! EduWorld mở lại lúc 06:30.                     │
│                         [ Đóng ]                                          │
└───────────────────────────────────────────────────────────────────────────┘

Form đăng nhập trong game (một màn duy nhất)
┌───────────────────────────────────────────────────────────────────────────┐
│ (Ảnh minh họa Quảng trường Tia Sáng, tĩnh, có lớp parallax nhẹ)    [VI|EN] │
│                  ┌───────────────────────────────────────┐                │
│                  │          [Logo EduWorld]              │                │
│                  │ Tên đăng nhập                         │                │
│                  │ [ hs.thdemo.000123        ] (điền sẵn) │               │
│                  │ Mật khẩu                              │                │
│                  │ [ ••••••••                     👁 ]   │                │
│                  │ [          Đăng nhập          ] (chính)│               │
│                  │ [✓] Nhớ tên đăng nhập trên máy này    │                │
│                  │ Quên mật khẩu? Liên hệ nhà trường     │                │
│                  │ để đặt lại                            │                │
│                  └───────────────────────────────────────┘                │
│ Phiên bản 1.0.3 · Trợ giúp                                   Mây (NPC) 👋 │
└───────────────────────────────────────────────────────────────────────────┘

Phiên đã hết (sau khi tự vào lại thất bại) — thông báo trên form
│ ⓘ Phiên đăng nhập đã hết, em đăng nhập lại nhé.                        │

Lỗi (ngay dưới ô mật khẩu, ô mật khẩu được xóa)
│ ⚠ Tên đăng nhập hoặc mật khẩu chưa đúng. Nếu nhập sai nhiều lần,      │
│   hãy đợi vài phút rồi thử lại.                                       │

Sau đăng nhập — chỉ khi thuộc nhiều tenant      Hộp xác nhận thiết bị khác
┌──────────────────────────────────┐           ┌──────────────────────────────────┐
│ Em muốn vào trường nào hôm nay?  │           │ Tài khoản đang dùng trên          │
│ ( ) THCS Demo B — Hà Nội  [logo] │           │ "Tablet Android" (lúc 07:42).    │
│ ( ) TH Demo A — Huế       [logo] │           │ Tiếp tục ở đây sẽ đăng xuất máy kia.│
│ (chọn trong 2 phút)              │           │                                  │
│ [Đăng xuất]          [Vào game]  │           │ [Ở lại máy kia]  [Tiếp tục ở đây] │
└──────────────────────────────────┘           └──────────────────────────────────┘
```

**5. Bố cục Mobile** (điện thoại ngang; tablet: cùng bố cục, thẻ rộng 50% màn)

```text
┌──────────────────────────────────────────────────────────────┐
│(minh họa tĩnh)  [Logo]                                [VI|EN]│
│  Mây: "Chào em!"   Tên đăng nhập [ hs.thdemo.000123     ]    │
│                    Mật khẩu      [ ••••••          👁 ]      │
│                    [            Đăng nhập            ]       │
│                    [✓] Nhớ tên đăng nhập · Quên mật khẩu?    │
└──────────────────────────────────────────────────────────────┘
Khi bàn phím ảo mở: form cuộn lên, ẩn minh họa và Mây, nút Đăng nhập luôn thấy.
```

Ô nhập cao ≥ 48 dp (lớp 1–3 ≥ 64 dp); ô mật khẩu dùng `LineEdit` với `secret = true`, bàn phím ảo kiểu mật khẩu (không gợi ý, không tự sửa), tắt sao chép từ ô mật khẩu. Hộp chọn trường sau đăng nhập dùng danh sách thẻ lớn.

**5a. Quy tắc bố cục form (Godot, [Confirmed — owner báo lỗi 2026-11-24, F061 Clarifications Session 2026-11-24]).**

- **Mật độ trước đăng nhập:** chưa biết khối nên form dùng Theme `g1_3` (`eduworld/ui/pre_bootstrap_density`): chữ 24 px, mọi ô nhập, nút, chip ngôn ngữ, ô "Nhớ tên đăng nhập" và nút "Quên mật khẩu?" cao ≥ 64 px ([DESIGN_SYSTEM §5.2](DESIGN_SYSTEM.md#52-mật-độ-theo-grade-band)).
- **Ba cột trong lề màn hình:** trái — logo (Tia Sáng + wordmark "EduWorld"), Mây với bong bóng lời chào (`ui.login.title` + `ui.login.mascot_greeting`, đồng thời là hướng dẫn cho lớp 1–3), phiên bản ở góc dưới; giữa — `LoginCard` căn giữa; phải — chọn ngôn ngữ ở góc trên. Độ rộng thẻ và hai cột lấy từ token `component.form_card.width`/`side_width`; stretch `canvas_items` + `expand` giữ chiều rộng logic ≥ 1280 px nên ba cột luôn vừa (1280×720, 1366×768, 1920×1080, 1024×600, cửa sổ dọc).
- **Thẻ không có tiêu đề riêng** (logo và lời chào của Mây thay); thứ tự: `InfoBanner` (ẩn khi không có), tên đăng nhập, mật khẩu + nút hiện/ẩn, `InlineError` (ẩn khi không có), Đăng nhập, Nhớ tên đăng nhập, Quên mật khẩu? (mở hộp hướng dẫn ngay dưới).
- **Vừa màn hình:** ở 1280×720 logic, thẻ đầy đủ kể cả lỗi dài nhất (sai mật khẩu + gợi ý đợi) không cần cuộn; chỉ khi chữ dài hơn nhiều (pseudo-locale, phóng chữ) thẻ mới cuộn dọc và Mây được ẩn để cột trái không bị cắt. Không có nút/ô nào bị kéo giãn: nút chỉ có icon là hình vuông, ô nhập cao đúng một dòng.
- **Chọn ngôn ngữ:** icon quả địa cầu + hai chip `VI` | `EN` (`ui.login.language_*_short`); chip đang chọn tô nền và có dấu ✓ (không chỉ dựa vào màu), tên đầy đủ "Tiếng Việt"/"English" là tooltip và nhãn trình đọc màn hình.
- **Hiện/ẩn mật khẩu:** nút icon con mắt (vuông, cao bằng ô nhập) bật/tắt; icon đổi mắt ↔ mắt gạch, nhãn trình đọc màn hình và tooltip `ui.login.show_password`/`hide_password`.
- **Thông báo:** `InlineError` nền đỏ nhạt, viền đỏ, icon tam giác "!" và chữ màu chữ chính; `InfoBanner` nền xanh nhạt, icon "i"; cả hai được đọc ngay khi xuất hiện (`accessibility_live`).
- **Đang đăng nhập:** nút Đăng nhập tắt, chữ "Đang vào…" + vòng xoay nhỏ ở đầu nút (đứng yên khi bật giảm chuyển động), hai ô nhập chỉ đọc.
- **Hình nền và nhân vật:** trời ấm, mặt trời, mây, đảo nổi, Tia Sáng và đồi vẽ bằng hình vector từ token `component.backdrop.*`/`component.mascot.*` (không ảnh tải về, tĩnh, nét ở mọi cỡ cửa sổ, [WORLD_DESIGN §11](../01-game-design/WORLD_DESIGN.md#11-art-direction)).

**6. Thành phần.** `ResumeSplash` ("Đang vào lớp…", nút phụ "Đăng nhập bằng tài khoản khác"), `ResumeConfirmCard` (avatar, "Tiếp tục là <nickname>?", nút "Tiếp tục" và "Không phải tôi"), `CurfewNotice` (giờ mở lại), `InfoBanner` (phiên hết), `LoginCard`, `TextField` (username), `PasswordField` (nút hiện/ẩn), `PrimaryButton`, `Checkbox` (nhớ tên), `LanguageToggle`, `InlineError`, `QueueIndicator` ("Đang vào lớp…"), `TenantChoiceList` (sau đăng nhập), `SchoolBadge` (chỉ trong `TenantChoiceList`), `ModalDialog` (takeover, lỗi), `NpcHint` (Mây). Tất cả là UI Godot ([GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md)).

**7. Dữ liệu & API.** `getClientConfig` (phiên bản, bảo trì); khi có phiên đã lưu: `refreshGameToken` (`POST /auth/game/token:refresh` `{refresh_token, device_id, client_version, mode: "resume"}`, `X-Tenant-Id` đã lưu) → `200` token mới + `bootstrap` / `401 REFRESH_TOKEN_INVALID` · `REFRESH_TOKEN_REUSED` · `REFRESH_TOKEN_DEVICE_MISMATCH` hoặc `403` tài khoản/tenant → xóa phiên đã lưu, hiện form (ADR-0086); `200` → S01b, "Không phải tôi" gọi `POST /auth/game/logout` và xóa phiên đã lưu; lỗi giờ giới nghiêm → `CurfewNotice`, giữ phiên đã lưu ([CHILD_SAFETY §4.1](../10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078)); `POST /auth/game/login` `{username, password, device_id, client_version, locale}` → `200` `tokens.*` + `bootstrap` (`PlayerBootstrap`; `tenant_id` → header `X-Tenant-Id` cho mọi request sau, ADR-0065; không có `game_session_id`) / `403 TENANT_NOT_SELECTED` kèm `choices[]` + `login_ticket` → `TenantChoice` → `POST /auth/game/tenant:select` (không nhập lại mật khẩu, ticket 2 phút) / `401 INVALID_CREDENTIALS` / `403 ACCOUNT_NOT_PROVISIONED` / `403 GAME_ACCESS_STUDENT_ONLY` / `426 CLIENT_VERSION_UNSUPPORTED` / `429 RATE_LIMITED` hoặc `LOGIN_QUEUE_BUSY` (`Retry-After`); `createGameSession` gọi ngay sau đăng nhập/chọn trường (header `X-Device-Id`, `X-Tenant-Id`, `Idempotency-Key`; 409 `ACTIVE_SESSION_ON_OTHER_DEVICE` kèm `other_device` → hộp takeover → gọi lại với `takeover=true` và `Idempotency-Key` mới, **không** nhập lại mật khẩu — ADR-0083); `createRealtimeTicket`; thoát game trên thiết bị cá nhân: `endGameSession` (`:end`), giữ phiên đã lưu; **Đăng xuất** (và thoát game trên máy dùng chung): `endGameSession` rồi `POST /auth/game/logout` rồi xóa phiên đã lưu. Message WS `session.session.replaced` dẫn về màn này với thông báo. Chi tiết: [API_DESIGN §9.1](../07-backend/API_DESIGN.md#91-auth--profile--apiv1auth-apiv1students).

**8. Trạng thái.**

| Trạng thái | Hiển thị |
|---|---|
| Đang kiểm tra phiên bản | Logo + thanh tiến độ mảnh; có phiên đã lưu → `ResumeSplash`, không có → form |
| Đang tự vào lại (`ResumingSession`, ADR-0086) | `ResumeSplash` "Đang vào lớp…"; không hiện form; sau 5 s hiện nút phụ "Đăng nhập bằng tài khoản khác" (= Đăng xuất rồi về form) |
| Tự vào lại thành công (S01b) | "Tiếp tục là <nickname>?" với avatar; focus mặc định ở "Tiếp tục"; không tự chuyển sau thời gian chờ; "Không phải tôi" → đăng xuất, xóa phiên đã lưu, về form trống (không điền sẵn tên của người trước) |
| Giờ giới nghiêm (đăng nhập hoặc tự vào lại) | `CurfewNotice` "Đã đến giờ nghỉ rồi! EduWorld mở lại lúc {giờ}."; không hiện form; phiên đã lưu được giữ. Đang chơi: cảnh báo trước 10 phút "Còn 10 phút nữa là giờ nghỉ", tới giờ lưu tiến độ và về màn này |
| Tự vào lại thất bại do phiên hết/bị thu hồi (`401 REFRESH_TOKEN_*`) | Form với `InfoBanner` "Phiên đăng nhập đã hết, em đăng nhập lại nhé."; tên đăng nhập điền sẵn nếu đã bật nhớ |
| Tự vào lại gặp lỗi tài khoản (`403 ACCOUNT_LOCKED`, `NO_ACTIVE_ENROLLMENT`, `TENANT_ACCESS_DISABLED`…) | Thông điệp theo mã như khi đăng nhập, rồi về form |
| Tự vào lại mất mạng/máy chủ bận | Giữ `ResumeSplash` với "Đang kết nối…", tự thử lại; không xóa phiên đã lưu |
| Form trống / điền sẵn tên | Ô mật khẩu luôn trống; nút Đăng nhập tắt khi một trong hai ô trống |
| Đang đăng nhập | Nút chuyển "Đang vào…" + spinner, khóa form; hủy được sau 10 s |
| Hàng chờ (`429 LOGIN_QUEUE_BUSY`) | `QueueIndicator` "Đang vào lớp… nhiều bạn đang vào cùng lúc", tự thử lại theo `Retry-After` + jitter, nút "Hủy" |
| Thử quá nhiều (`429 RATE_LIMITED`) | "Em thử nhiều lần quá. Hãy đợi 1 phút rồi thử lại nhé." + đếm ngược |
| Sai username/password hoặc khóa tạm (`401 INVALID_CREDENTIALS`) | `InlineError` "Tên đăng nhập hoặc mật khẩu chưa đúng" (+ "Nếu nhập sai nhiều lần, hãy đợi vài phút" từ lần sai thứ 3 trên thiết bị); không tiết lộ tài khoản có tồn tại hay đang bị khóa; xóa ô mật khẩu, focus về ô mật khẩu |
| Đang xác định trường | "Đang vào thế giới…", không có bước chọn trường |
| Đang tạo phiên chơi (`createGameSession`) | Vẫn "Đang vào thế giới…"; lỗi mạng → "Thử lại" (gọi lại `createGameSession`, không đăng nhập lại) |
| Thuộc nhiều trường | `TenantChoiceList`; **không** chọn sẵn lựa chọn của lần trước, không lưu lựa chọn xuống `user://`; khi tự vào lại bằng phiên đã lưu, game dùng trường của phiên đó (đăng xuất để đổi trường) (ADR-0086); quá 2 phút (`401 LOGIN_TICKET_INVALID`) → "Phiên chọn trường đã hết, em đăng nhập lại nhé" và về form |
| Tài khoản chưa được cấp (`ACCOUNT_NOT_PROVISIONED`) | "Tài khoản của em chưa được kích hoạt trong EduWorld. Em hãy báo nhà trường kiểm tra nhé." + nút "Đăng nhập tài khoản khác"; không vào game |
| Tài khoản nhân sự (`GAME_ACCESS_STUDENT_ONLY`) | "Tài khoản này không dùng để vào game. Cán bộ nhà trường vui lòng dùng trang quản trị." |
| Cần cập nhật (`426 CLIENT_VERSION_UNSUPPORTED` hoặc client-config) | Hộp "Có phiên bản mới" + nút cập nhật; không cho vào game |
| Bảo trì | Minh họa + giờ dự kiến; tự kiểm tra lại mỗi 60 s |
| Lỗi tài khoản khác | Thông điệp theo `ProblemCode` ([GODOT_ARCHITECTURE §9](../08-game-client/GODOT_ARCHITECTURE.md#9-error-handling--offline)): `ACCOUNT_LOCKED`, `NO_ACTIVE_ENROLLMENT`, `TENANT_ACCESS_DISABLED` |
| Mất mạng | "Chưa có kết nối Internet" + "Thử lại" |
| Đăng nhập thành công | Chuyển cảnh mờ sang S02/S03 (≤ 1 s) |

**9. Tương tác & phản hồi.** Enter ở ô tên đăng nhập → chuyển sang ô mật khẩu; Enter ở ô mật khẩu = Đăng nhập. Tab/D-pad đi theo thứ tự: tên → mật khẩu → nút hiện/ẩn → Đăng nhập → Nhớ tên → Quên mật khẩu. Hộp takeover không tự đóng, nút an toàn ("Ở lại máy kia") đặt bên trái. Ô "Nhớ tên đăng nhập trên máy này" mặc định bật; khi bật, tên đăng nhập lần trước được điền sẵn (chỉ lưu tên đăng nhập trong `user://`), mật khẩu luôn phải nhập khi form hiện. Không có checkbox "Ghi nhớ đăng nhập". Bấm "Quên mật khẩu?" mở hộp hướng dẫn trong game (không mở web). Hộp takeover (cả khi tự vào lại): "Tiếp tục ở đây" gọi lại `createGameSession` với `takeover=true`, không hỏi lại mật khẩu; "Ở lại máy kia" → đăng xuất (`/auth/game/logout`, xóa phiên đã lưu), về form. **Thoát game** trên thiết bị cá nhân → `:end` game session, giữ phiên để lần sau tự vào. **Đăng xuất** ở Cài đặt → Tài khoản ([STUDENT_GAME_UX_SOCIAL §10](STUDENT_GAME_UX_SOCIAL.md#10-cài-đặt)), hỏi xác nhận "Em muốn đăng xuất? Lần sau em sẽ cần nhập mật khẩu." → `:end`, `POST /auth/game/logout`, xóa phiên đã lưu, về form (ADR-0083, ADR-0086). Máy dùng chung (`device.cfg` `shared_device: true` nếu có; bản Web luôn xử lý như máy dùng chung — ADR-0101): không lưu phiên, ẩn ô nhớ tên, không nhớ tên đăng nhập, thoát game = đăng xuất, tự đăng xuất sau 30 phút không hoạt động; mở lại luôn hiện form.

**10. Khác biệt theo grade band.** `g1_3`: chữ và nút lớn, có nút loa đọc hướng dẫn "Em nhập tên đăng nhập và mật khẩu nhà trường đưa", cho phép hiện mật khẩu mặc định; `g4_5`, `g6_9`: như mặc định. Cùng một phương thức đăng nhập cho mọi khối (ADR-0063). Trước khi đăng nhập game chưa biết khối, nên form luôn dùng cỡ `g1_3` cho mọi học sinh; hướng dẫn hiện bằng chữ trong bong bóng của Mây, mật khẩu mặc định ẩn (máy dùng chung) và hiện bằng nút con mắt; nút loa chờ có tệp lồng tiếng ([F061 Clarifications Session 2026-11-24](../../specs/061-godot-api-client-login/spec.md#session-2026-11-24-giao-diện-s01)) [Confirmed].

**11. Song ngữ.** Khóa trong game: `ui.login.title` (lời chào của Mây — vi "Chào em!" / en "Hi there!"), `ui.login.mascot_greeting`, `ui.login.wordmark`, `ui.login.language_vi_short`, `ui.login.language_en_short`, `ui.login.language_label`, `ui.login.hide_password`, `ui.login.username_label`, `ui.login.password_label`, `ui.login.show_password`, `ui.login.button_sign_in`, `ui.login.signing_in`, `ui.login.forgot_password_hint`, `ui.login.forgot_password_dialog`, `ui.login.remember_username`, `ui.login.invalid_credentials`, `ui.login.wait_hint`, `ui.login.rate_limited`, `ui.login.queue_busy`, `ui.login.staff_account`, `ui.login.ticket_expired`, `ui.login.resolving_tenant`, `ui.login.not_provisioned`, `ui.login.tenant_choice.title`, `ui.login.tenant_choice.enter`, `ui.login.takeover.title`, `ui.login.takeover.body`, `ui.login.takeover.stay`, `ui.login.takeover.continue`, `ui.login.update_required`, `ui.login.maintenance`, `ui.login.resuming`, `ui.login.use_other_account`, `ui.login.session_expired`, `ui.login.resume_confirm.title` (vi "Tiếp tục là {nickname}?" / en "Continue as {nickname}?"), `ui.login.resume_confirm.continue`, `ui.login.resume_confirm.not_me` (vi "Không phải tôi" / en "Not me"), `ui.login.curfew` (vi "Đã đến giờ nghỉ rồi! EduWorld mở lại lúc {time}."), `ui.login.curfew_warning`, `ui.settings.logout_confirm`. Thông điệp lỗi chọn theo `code` của Problem, không hiển thị `detail` từ server.
Ví dụ: `ui.login.forgot_password_hint` — vi "Quên mật khẩu? Liên hệ nhà trường để đặt lại" / en "Forgot your password? Contact your school to reset it"; `ui.login.invalid_credentials` — vi "Tên đăng nhập hoặc mật khẩu chưa đúng" / en "Username or password is not correct"; `ui.login.tenant_choice.title` — vi "Em muốn vào trường nào hôm nay?" / en "Which school are you playing in today?". Chuỗi dài nhất để kiểm: `ui.login.takeover.body` (vi ~120 ký tự).

**12. Trợ năng.** Focus mặc định ở ô tên đăng nhập (hoặc ô mật khẩu nếu tên đã điền sẵn); mọi ô có nhãn hiển thị (không chỉ placeholder) và nhãn cho trình đọc màn hình; lỗi được đọc ngay khi xuất hiện; nút hiện/ẩn có nhãn; hỗ trợ phóng chữ theo cài đặt game; danh sách chọn trường đọc được bằng trình đọc màn hình.

**13. Hiệu năng máy tầm trung.** Form là ảnh 2D tĩnh + Control; tải trước gói Hub trong nền khi người dùng đang nhập. Mục tiêu: từ mở app tới form ≤ 3 s; từ bấm Đăng nhập tới phản hồi p95 ≤ 1 s khi không xếp hàng; từ đăng nhập xong tới S03 theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md).

**14. An toàn/riêng tư.** Mật khẩu chỉ nằm trong ô nhập và bộ nhớ request; sau khi gửi (thành công hay lỗi) ô mật khẩu bị xóa, không ghi mật khẩu vào log/telemetry/crash report, không lưu `user://` (ADR-0068). Chỉ gửi qua HTTPS tới tên miền API (certificate pinning [Confirmed]); không bao giờ gửi tới Keycloak trực tiếp. Access token chỉ nằm trong RAM; refresh token (kèm trường của phiên) chỉ lưu trong kho bảo mật của hệ điều hành trên thiết bị cá nhân (Android Keystore, iOS Keychain), chỉ RAM trên Web và máy dùng chung (ADR-0086, ADR-0101); màn S01b chỉ hiện nickname + avatar (không tên thật) để bạn khác dùng chung máy nhận ra không phải tài khoản của mình; mật khẩu không bao giờ được lưu; không hiển thị tên học sinh trước khi đăng nhập xong; thông điệp lỗi không tiết lộ tài khoản tồn tại, bị khóa hay thuộc trường nào; danh sách chọn trường chỉ hiển thị sau xác thực và chỉ gồm trường của chính người dùng; quên mật khẩu không reset qua email — School Admin đặt lại trên Admin Portal.

**15. AC.**

| Mã | Tiêu chí |
|---|---|
| AC-UX-0101 | Thiết bị cá nhân: thoát game rồi mở lại khi phiên còn hạn → "Đang vào lớp…" → S01b "Tiếp tục là <nickname>?" → "Tiếp tục" vào S02/S03, không hiện form; phiên hết hạn/bị thu hồi → form với thông báo "Phiên đăng nhập đã hết", tên đăng nhập điền sẵn nếu đã bật nhớ; Web và máy dùng chung luôn hiện form (ADR-0086) |
| AC-UX-0102 | Form đăng nhập là UI trong game, chỉ có username/password + Đăng nhập (+ chọn ngôn ngữ, nhớ tên, hướng dẫn quên mật khẩu); **không** mở trình duyệt/WebView, **không** có bước chọn trường, ô mã trường hay nút đăng nhập nhanh (ADR-0068) |
| AC-UX-0103 | `createGameSession` trả 409 `ACTIVE_SESSION_ON_OTHER_DEVICE` → hộp xác nhận; chọn "Tiếp tục ở đây" vào game mà không nhập lại mật khẩu; chọn "Ở lại máy kia" về form |
| AC-UX-0104 | Đổi ngôn ngữ trên form áp dụng ngay cho form, thông điệp lỗi và toàn bộ màn sau khi vào game |
| AC-UX-0105 | Tài khoản nhân sự (School Admin, Academic Manager, vận hành) đăng nhập game → thông điệp hướng dẫn dùng Admin Portal, không vào game |
| AC-UX-0106 | Học sinh thuộc đúng 1 tenant (SIS hoặc tenant chung) vào thẳng game; thuộc nhiều tenant thấy `TenantChoiceList` chỉ sau xác thực, không có lựa chọn nào được chọn sẵn từ lần mở game trước, chọn xong vào game không nhập lại mật khẩu |
| AC-UX-0107 | Tài khoản không thuộc tenant chung và không thuộc tenant SIS nào → thông điệp "chưa được kích hoạt", không vào game, không tự đưa vào tenant chung |
| AC-UX-0108 | Sai mật khẩu, tài khoản không tồn tại, tài khoản khóa tạm: cùng một thông điệp; ô mật khẩu bị xóa sau mỗi lần gửi |
| AC-UX-0109 | Quá tải đăng nhập → "Đang vào lớp…" và tự vào khi đến lượt, không cần bấm lại |
| AC-UX-0110 | Để màn chọn trường quá 2 phút rồi chọn → thông báo hết phiên chọn trường, quay về form |
| AC-UX-0111 | Bấm Đăng xuất (Cài đặt → Tài khoản), xác nhận → về form; mở lại game → form, không tự vào (ADR-0086) |
| AC-UX-0112 | Tự vào lại khi mất mạng → giữ màn "Đang kết nối…" và tự thử lại, không xóa phiên; nút "Đăng nhập bằng tài khoản khác" đưa về form |
| AC-UX-0113 | S01b: bấm "Không phải tôi" → phiên Keycloak bị thu hồi, phiên đã lưu bị xóa, form trống không điền sẵn tên; mở lại game → form (TC-AUTH-091) |
| AC-UX-0114 | ~~Game Windows cài mặc định: đăng nhập, thoát, mở lại → luôn hiện form; cài với "Máy cá nhân" → S01b (TC-AUTH-090)~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows; Web luôn hiện form khi mở lại theo FR-AUTH-008); mã giữ để truy vết, không tái sử dụng |
| AC-UX-0115 | Mở game trong giờ giới nghiêm → `CurfewNotice` với giờ mở lại, không hiện form; đang chơi lúc 21:20 → cảnh báo, 21:30 lưu tiến độ và về `CurfewNotice` (TC-AUTH-092) |

---

## S02 — Tạo avatar lần đầu

**1. Mục đích.** Học sinh tạo nhân vật của mình trong ≤ 2 phút, cảm thấy nhân vật "là của mình", không lộ thông tin cá nhân ([CHARACTER_SYSTEM §5.0](../01-game-design/CHARACTER_SYSTEM.md#50-tạo-avatar-lần-đầu), §5.3).

**2. Người dùng & khối.** Học sinh chưa có `avatar_config`. `g1_3` dùng luồng rút gọn.

**3. Điểm vào / ra.** Vào: sau S01 khi bootstrap báo chưa có avatar. Ra: S03 (Chương 0 "Lời mời" — quest hướng dẫn đầu tiên với Mây).

Các bước: (1) Chọn nhân vật mẫu → (2) Tùy chỉnh (dáng, màu da, tóc, màu tóc, trang phục mặc định) → (3) Chọn nickname → (4) Xem trước và xác nhận. `g1_3`: (1) → màu tóc → (3) → (4).

**4. Bố cục Web/PC**

```text
┌───────────────────────────────────────────────────────────────────────────┐
│ Tạo nhân vật của em             ● Chọn mẫu ─ ○ Tùy chỉnh ─ ○ Tên ─ ○ Xong   │
├──────────────────────────────┬────────────────────────────────────────────┤
│                              │ Nhân vật mẫu                               │
│                              │ [M1] [M2] [M3]                             │
│     (Xem trước 3D,           │ [M4] [M5] [M6]                             │
│      xoay bằng kéo chuột,    │ ─────────────────────────────              │
│      nhân vật vẫy tay)       │ Dáng người   (●)(○)(○)(○)                  │
│                              │ Màu da       ■ ■ ■ ■ ■ ■                   │
│      [⟲ Xoay]  [🔍 Zoom]     │ Kiểu tóc     [◀ Tóc 03 ▶]                  │
│                              │ Màu tóc      ■ ■ ■ ■ ■ ■ ■ ■               │
│                              │ Trang phục   [◀ Mặc định ▶]                │
├──────────────────────────────┴────────────────────────────────────────────┤
│ [Quay lại]                                     [Bỏ qua tùy chỉnh] [Tiếp ▶] │
└───────────────────────────────────────────────────────────────────────────┘

Bước Tên:  Chọn tên của em  ( Cú Mèo Chăm Chỉ 27 )( Rồng Xanh Vui Vẻ 08 )( Mèo Mây 15 )
           [↻ Gợi ý khác]   — không nhập tên tự do
```

**5. Bố cục Mobile** (điện thoại ngang; tablet: panel phải rộng hơn, mẫu hiển thị 3×2)

```text
┌──────────────────────────────────────────────────────────────┐
│ Tạo nhân vật   ●─○─○─○                                         │
├────────────────────────┬─────────────────────────────────────┤
│                        │ [Mẫu] [Dáng] [Da] [Tóc] [Đồ]  (tab)  │
│  (Xem trước 3D,        │ ┌───┐┌───┐┌───┐                      │
│   vuốt để xoay)        │ │M1 ││M2 ││M3 │  ◀ cuộn ngang ▶       │
│                        │ └───┘└───┘└───┘                      │
├────────────────────────┴─────────────────────────────────────┤
│ [Quay lại]                              [Bỏ qua] [ Tiếp ▶ ]   │
└──────────────────────────────────────────────────────────────┘
```

**6. Thành phần.** `StepIndicator`, `AvatarPreview3D` (SubViewport), `PresetGrid`, `SwatchPicker`, `OptionCarousel`, `TabBar` (mobile), `NicknameChoiceChips`, `PrimaryButton`.

**7. Dữ liệu & API.** Danh mục preset/màu/tóc/trang phục mặc định lấy từ content manifest (`getContentManifest`, item cosmetic mặc định) — không cần mạng cho từng lần đổi; danh sách nickname gợi ý: **đề xuất** `GET /api/v1/game/profile/nickname-suggestions` (`suggestNicknames`, trả 3–5 gợi ý theo ngôn ngữ hiện tại, server đảm bảo duy nhất trong tenant); xác nhận: `updateAvatar` (`PUT /game/profile/avatar`) + `updateGameProfile` (nickname). Event `avatar.created`.

**8. Trạng thái.** Đang tải mẫu: khung xám + nhân vật bóng mờ. Lỗi lưu: giữ lựa chọn, nút "Thử lại". Nickname vừa bị người khác chọn (409): tự làm mới gợi ý, thông báo nhẹ. Mất mạng ở bước xác nhận: giữ lựa chọn cục bộ, gửi lại khi có mạng (idempotency key).

**9. Tương tác & phản hồi.** Mỗi lựa chọn đổi xem trước ngay (≤ 100 ms, không gọi API). Hoàn tất: nhân vật vẫy tay, Mây chào bằng nickname mới, chuyển cảnh vào Quảng trường Tia Sáng. Âm thanh nhẹ khi đổi lựa chọn (tắt được).

**10. Khác biệt theo grade band.** `g1_3`: 3 bước, nút ≥ 64 dp, đọc to hướng dẫn, nickname gợi ý kèm hình con vật; `g4_5`: đủ bước, có nút "Bỏ qua tùy chỉnh"; `g6_9`: đủ bước, thêm thanh trượt màu tóc trong bảng màu cố định.

**11. Song ngữ.** Khóa: `ui.avatar.title`, `ui.avatar.step_preset`, `ui.avatar.step_customize`, `ui.avatar.step_name`, `ui.avatar.step_done`, `ui.avatar.body_type`, `ui.avatar.skin_tone`, `ui.avatar.hair_style`, `ui.avatar.hair_color`, `ui.avatar.outfit`, `ui.avatar.skip_customize`, `ui.avatar.more_names`, `ui.avatar.confirm`. Nickname gợi ý sinh theo ngôn ngữ đang chọn (vi: "Cú Mèo Chăm Chỉ 27", en: "Busy Owl 27") và **giữ nguyên** khi đổi ngôn ngữ sau này (nickname là tên riêng). Chuỗi dài nhất: nickname vi (tối đa 24 ký tự [Confirmed]).

**12. Trợ năng.** Mỗi lựa chọn màu có tên đọc được ("Màu tóc nâu sẫm"); điều hướng bàn phím trên lưới; giảm chuyển động tắt xoay tự động.

**13. Hiệu năng máy tầm trung.** Xem trước 3D dùng `SubViewport` 512×512 (preset Trung bình), 384×384 (Thấp), 30 FPS, một đèn, không bóng động; chỉ nạp mesh/texture của lựa chọn đang xem, nạp trước lựa chọn kế tiếp; thumbnail lựa chọn là ảnh 2D render sẵn (ADR-0047), không render 3D cho từng ô.

**14. An toàn/riêng tư.** Không cho nhập tên tự do, không tải ảnh; không có tùy chọn ngoại hình người lớn, biểu tượng chính trị/tôn giáo.

**15. AC.**

| Mã | Tiêu chí |
|---|---|
| AC-UX-0201 | Học sinh `g1_3` hoàn tất tạo avatar trong ≤ 3 bước, không cần gõ phím |
| AC-UX-0202 | Đổi bất kỳ lựa chọn nào cập nhật xem trước ≤ 100 ms trên máy tầm trung, không gọi API |
| AC-UX-0203 | Không có ô nhập tên tự do; "Gợi ý khác" trả nickname chưa dùng trong tenant |
| AC-UX-0204 | Xác nhận xong vào Quảng trường Tia Sáng và bắt đầu quest hướng dẫn Chương 0 |

---

## S03 — Main Hub HUD

**1. Mục đích.** Lớp giao diện thường trực khi học sinh ở trong thế giới: cho biết đang ở đâu, nên làm gì tiếp, tiến độ của mình, và mở nhanh các chức năng — nhưng nhường phần lớn màn hình cho thế giới 3D ([GAME_DESIGN](../01-game-design/GAME_DESIGN.md), [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md)).

**2. Người dùng & khối.** Mọi học sinh, mọi vùng đất (trừ màn toàn phần như S16–S18, S06 khi đang trong phase).

**3. Điểm vào / ra.** Vào: sau S02/S01, sau khi đóng bất kỳ panel nào, sau chuyển vùng. Ra: mọi màn khác theo sơ đồ điều hướng ([STUDENT_GAME_UX §11](STUDENT_GAME_UX.md#11-sơ-đồ-điều-hướng)).

**4. Bố cục Web/PC.** Theo khung chuẩn [STUDENT_GAME_UX §3](STUDENT_GAME_UX.md#3-khung-hud-chuẩn), chi tiết:

```text
┌───────────────────────────────────────────────────────────────────────────┐
│[Av] Cú Mèo 27 · Nhà thám hiểm   Lv12 [██████░░░ 340/500]   [Xu 340][🔔3][⚙]│
│ Vương quốc Số › Cầu Đá  (tên khu)                    [☀ Lễ hội Trung thu ▸]│
│┌ Nhiệm vụ đang theo dõi ─┐                                 ┌Minimap─┐     │
││★ Sửa cây cầu (Chương 1)  │                                 │ ·  ▲   │     │
││  ○ Gặp Ông Số            │                                 │   ◎    │     │
││  ○ Giải 5 câu đo độ dài  │                                 └────────┘     │
││☐ Nhà trường giao (2)     │                                                │
│└──────────────[J] Tất cả┘                                                │
│                                (thế giới 3D)                              │
│                                    [E] Nói chuyện ▸ Ông Số                │
│┌Chat · Party ▾──────────┐                                                 │
││Mèo Vàng: đi boss nhé   │  [👥 Party 3/4]                 [😊 T] [🗺 M] [🏠]│
│└────────────────────────┘                                                 │
└───────────────────────────────────────────────────────────────────────────┘
```

**5. Bố cục Mobile.** (điện thoại ngang; tablet: theo dõi quest mở sẵn, chat dạng khung nhỏ như PC)

```text
┌──────────────────────────────────────────────────────────────┐
│[Av]Lv12[███░]  Vương quốc Số          [☀▸] [Xu 340][🔔3][☰]   │
│[★ Sửa cây cầu · Gặp Ông Số ▾]                      ┌──────┐   │
│                                                    │minimap│  │
│                                                    └──────┘   │
│                    (thế giới 3D)            [🗺] [📜] [🏠]     │
│[💬3] [👥3/4]                                                  │
│  ╭───╮                                        (Nói ▸Ông Số)   │
│  │ ◉ │                                       (⤒)  (😊)        │
│  ╰───╯                                                        │
└──────────────────────────────────────────────────────────────┘
```

**6. Thành phần.** `PlayerBadge` (avatar, nickname, danh hiệu, level), `XpBar`, `CurrencyChip`, `NotificationBell`, `QuestTracker` (≤ 3 quest), `AreaTitle`, `EventChip`, `Minimap` (ảnh 2D + marker), `ContextActionButton`, `Joystick` (mobile), `ChatDock`, `PartyChip`, `HomeButton`, `ToastStack` (≤ 3 toast).

**7. Dữ liệu & API.** Khi vào: `getGameProfile`, `getProgression`, `listQuests?status=active`, `listWorldAreas`, `getWorldExploration`, ví Xu (`getWallet`). Cập nhật realtime (không polling): `progression.xp.updated`, `progression.level.reached` (mở S07), `quest.progress.updated`, `world.area.unlocked`, `notification.item.created`, `hub.instance.assigned`, chat theo [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md). Vào vùng: `enterWorldArea`. Bài nhà trường giao: `listMyAssignments` (`GET /api/v1/learning/my-assignments`, đã có trong OpenAPI; [SCHOOL_LMS §11](../04-admin-portal/SCHOOL_LMS.md#11-api)) hoặc quest nguồn `assignment` trong `listQuests` [Confirmed].

**8. Trạng thái.**

| Trạng thái | Hiển thị |
|---|---|
| Đang vào vùng | Màn chuyển vùng (ảnh vùng + mẹo học tập), HUD ẩn |
| Không có quest | Theo dõi quest hiện "Nói chuyện với Mây để nhận nhiệm vụ" |
| Mất kết nối realtime | Banner "Đang kết nối lại…"; nhân vật dừng; nút tương tác mờ |
| Đạt trần XP ngày | Thanh XP có biểu tượng mặt trăng + tooltip "Hôm nay em đã nhận đủ XP, phần học vẫn được ghi nhận" |
| Nhắc nghỉ | Hộp nhẹ của Mây theo [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| Đang trong party/boss | `PartyChip` hiện số người; vào boss thì HUD chuyển sang bố cục S06 |

**9. Tương tác & phản hồi.** XP tăng: số bay từ nguồn lên thanh XP (≤ 0,8 s). Quest bước hoàn thành: dấu ✓ + âm nhỏ. `ContextActionButton` đổi nhãn khi lại gần đối tượng. Nút 🏠 "Về Quảng trường" ≤ 1 thao tác (xác nhận nếu đang trong hoạt động).

**10. Khác biệt theo grade band.** `g1_3`: ẩn minimap, theo dõi 1 quest với hình NPC lớn, bấm-để-đi bật, chat chỉ hiện nút; `g4_5`: theo dõi 2 quest, minimap; `g6_9`: theo dõi 3 quest, minimap, khung chat mở trên PC.

**11. Song ngữ.** Khóa: `ui.hub.level_title`, `ui.hub.xp_label`, `ui.hub.quest_tracker.title`, `ui.hub.quest_tracker.empty`, `ui.hub.quest_tracker.view_all`, `ui.hub.assignments_count`, `ui.hub.action.<verb>` (`talk`, `pick_up`, `enter`, `read`), `ui.hub.home`, `ui.hub.daily_cap_reached`, `ui.hub.party_chip`. Tên vùng/khu/NPC lấy từ dữ liệu `i18nText`; chế độ hiển thị song song: "Vương quốc Số · Math Kingdom". Chuỗi dài nhất: tên khu + chương ở `AreaTitle` (giới hạn 1 dòng, xuống dòng không được — dùng dấu "…" và tooltip).

**12. Trợ năng.** Phím tắt hiện trên nút (PC); chế độ tương phản cao viền chữ HUD; giảm chuyển động tắt số bay và rung camera; mọi thông báo có thể mở lại trong 🔔.

**13. Hiệu năng máy tầm trung.** HUD ≤ 60 Control; cập nhật theo event; minimap là ảnh 2D vẽ sẵn của khu + marker (không camera thứ hai); tối đa 3 toast, gộp toast cùng loại; tên người chơi khác dùng `Label3D` chỉ trong bán kính hiển thị (≤ 20 nhân vật, [ADR-0057](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md)); HUD không gây thêm > 10 draw call.

**14. An toàn/riêng tư.** Nhân vật khác chỉ hiện nickname + danh hiệu; chạm vào nhân vật khác mở thẻ nhỏ có "Kết bạn", "Mời party", "Chặn", "Báo cáo" (≤ 2 chạm).

**15. AC.**

| Mã | Tiêu chí |
|---|---|
| AC-UX-0301 | Mọi chức năng chính (bản đồ, nhiệm vụ, túi đồ, bạn bè, party, chat, cài đặt, về Quảng trường) mở được trong ≤ 2 thao tác từ HUD trên cả PC và mobile |
| AC-UX-0302 | XP/Xu/quest cập nhật trên HUD trong ≤ 1 s sau message realtime, không polling |
| AC-UX-0303 | Trên máy tầm trung, HUD không làm FPS cảnh Hub giảm quá 5% so với khi ẩn HUD |
| AC-UX-0304 | Vùng giữa màn không có panel cố định; HUD không che safe area trên điện thoại có notch |

---

## S04 — World map

**1. Mục đích.** Cho học sinh thấy toàn bộ thế giới 8 vùng đất, vùng nào đã mở và cần gì để mở, tiến độ khám phá, và dịch chuyển nhanh bằng cổng gió ([WORLD_DESIGN §3, §6–§8](../01-game-design/WORLD_DESIGN.md)).

**2. Người dùng & khối.** Mọi học sinh. Vùng môn học bị trường tắt không hiển thị.

**3. Điểm vào / ra.** Vào: phím M / nút 🗺, NPC dẫn đường, thông báo "Vùng mới đã mở". Ra: đóng (về S03) hoặc dịch chuyển (màn chuyển vùng → S03 ở vùng đích).

**4. Bố cục Web/PC**

```text
┌───────────────────────────────────────────────────────────────────────────┐
│ Bản đồ thế giới                               Đã khám phá 37%   [✕ Esc]     │
├───────────────────────────────────────────────────┬───────────────────────┤
│        (Bản đồ minh họa 2D vẽ tay, zoom/pan)      │ Vương quốc Số          │
│   [Rừng Ngôn Ngữ 42%]      [Núi Thử Thách 🔒]     │ Math Kingdom           │
│            ╲                 ╱                    │ Chương 1 · đang làm    │
│      [Thị trấn Trung Tâm ★ bạn đang ở đây]        │ Khám phá: 60% ████░░   │
│            ╱       │        ╲                     │ • Điểm khám phá 6/10   │
│ [Vương quốc Số 60%] [Đảo Khoa Học 15%] [Đấu trường]│ • Vật sưu tầm 3/8      │
│                    │                              │ • Quest khu vực 4/7    │
│     [Vương quốc Lịch Sử 0%]  [Đại dương Khám Phá 🔒]│ [Dịch chuyển (cổng gió)]│
│  Chú thích: ◎ cổng gió đã kích hoạt  🔒 chưa mở   │ Bạn bè ở đây: 2        │
└───────────────────────────────────────────────────┴───────────────────────┘
Vùng khóa → panel phải: "Mở khi: hoàn thành ≥ 1 chương môn học (0/1)" + nút "Xem nhiệm vụ gợi ý"
```

**5. Bố cục Mobile** (điện thoại ngang: panel chi tiết thành bảng trượt từ phải 40% màn; tablet: như PC)

```text
┌──────────────────────────────────────────────────────────────┐
│ Bản đồ thế giới     Khám phá 37%                       [✕]   │
│  (bản đồ 2D, kéo/chụm để zoom)        ┌──────────────────────┐│
│   [Rừng 42%]   [Núi 🔒]               │ Vương quốc Số  60%    ││
│      [Thị trấn ★]                     │ Chương 1 · đang làm   ││
│ [V.q.Số 60%] [Đảo 15%] [Đấu trường]   │ [ Dịch chuyển ]       ││
│                                       └──────────────────────┘│
└──────────────────────────────────────────────────────────────┘
```

**6. Thành phần.** `WorldMapCanvas` (ảnh 2D + vùng bấm được), `RegionPin` (trạng thái: hiện tại/mở/khóa/có sự kiện), `ProgressRing`, `RegionDetailPanel`, `UnlockRequirementList`, `TeleportButton`, `Legend`.

**7. Dữ liệu & API.** `listWorldAreas` (trạng thái mở/khóa + điều kiện còn thiếu), `getWorldExploration` (%, điểm khám phá, vật sưu tầm, chương), `enterWorldArea` (dịch chuyển qua cổng gió; 403 `AREA_LOCKED` hiển thị điều kiện), WS `world.area.unlocked` (hiệu ứng mở khóa khi đang mở bản đồ). Vị trí bạn bè trên bản đồ: chỉ số lượng bạn ở vùng, theo presence ([SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)).

**8. Trạng thái.** Đang tải: ảnh bản đồ hiện ngay (đóng gói sẵn), số liệu hiện skeleton. Vùng khóa: tô xám nhẹ + 🔒 + điều kiện bằng lời. Vùng có sự kiện: biểu tượng lấp lánh. Lỗi tải số liệu: bản đồ vẫn dùng được, chỉ ẩn %. Mất mạng: nút Dịch chuyển mờ.

**9. Tương tác & phản hồi.** Chạm/nhấp vùng → panel chi tiết; nhấp đúp → dịch chuyển (PC). Mở khóa vùng mới: đám mây tan dần trên vùng + âm thanh, Mây giới thiệu một câu. Không có hiệu ứng mất tiến độ.

**10. Khác biệt theo grade band.** `g1_3`: bản đồ dạng "bảng chọn hình" — 8 thẻ hình lớn, chạm là dịch chuyển (không cần đi bộ, theo WORLD_DESIGN §6), không hiện %, chỉ hiện ngôi sao; `g4_5`: bản đồ + % khám phá; `g6_9`: thêm chi tiết điểm khám phá/vật sưu tầm.

**11. Song ngữ.** Khóa: `ui.worldmap.title`, `ui.worldmap.explored_percent`, `ui.worldmap.you_are_here`, `ui.worldmap.locked`, `ui.worldmap.unlock_when`, `ui.worldmap.teleport`, `ui.worldmap.friends_here`, `ui.worldmap.legend.*`. Tên vùng là dữ liệu `i18nText` theo [ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md) (vi "Đại dương Khám Phá" / en "Discovery Ocean"); bật hiển thị song song thì `RegionPin` hiện tên phụ cỡ nhỏ. Ảnh bản đồ **không chứa chữ** (chữ vẽ bằng UI để dịch được). Chuỗi dài nhất: điều kiện mở khóa.

**12. Trợ năng.** Điều hướng bàn phím giữa các vùng (mũi tên), đọc to tên vùng + trạng thái; màu khóa/mở kèm biểu tượng.

**13. Hiệu năng máy tầm trung.** Bản đồ là **ảnh 2D** (2048×1152, nén theo nền tảng) + ≤ 30 marker, không render 3D; zoom/pan bằng transform của Control; số liệu tải một lần khi mở, cache 60 s.

**14. An toàn/riêng tư.** Chỉ hiện số bạn bè ở mỗi vùng, không hiện vị trí chính xác hay người lạ.

**15. AC.**

| Mã | Tiêu chí |
|---|---|
| AC-UX-0401 | Vùng khóa luôn hiện điều kiện mở bằng lời và tiến độ (ví dụ 0/1 chương); không có lựa chọn mở bằng Xu |
| AC-UX-0402 | Dịch chuyển tới vùng đã mở hoàn tất (gồm màn chuyển vùng) trong ngân sách chuyển vùng của ADR-0057 |
| AC-UX-0403 | `g1_3` dịch chuyển tới vùng bất kỳ đã mở bằng 1 chạm |
| AC-UX-0404 | Mở bản đồ trên máy tầm trung ≤ 200 ms (ảnh đã nạp sẵn) |

---

## S05 — Quest journal

**1. Mục đích.** Nơi học sinh xem và quản lý mọi nhiệm vụ: chính (theo chương), phụ, ngày/tuần, học tập, khám phá, party, thử thách, sự kiện — và **bài nhà trường giao** — với bước tiếp theo rõ ràng ([QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md), [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) FR-LMS-006).

**2. Người dùng & khối.** Mọi học sinh.

**3. Điểm vào / ra.** Vào: phím J / nút 📜, "Tất cả" trong theo dõi quest, NPC Bưu tá Nhím (bài giao). Ra: đóng; "Theo dõi"; "Đi tới" (đặt điểm đến trên minimap hoặc dịch chuyển); "Bắt đầu" bước học tập (S16).

**4. Bố cục Web/PC**

```text
┌───────────────────────────────────────────────────────────────────────────┐
│ Nhật ký nhiệm vụ                                                 [✕ Esc]   │
├───────────────┬───────────────────────────────┬───────────────────────────┤
│ Đang làm (5)  │ ★ Sửa cây cầu        Chương 1 │ Sửa cây cầu                │
│ Nhà trường(2) │ ☐ Bài: Phép nhân bảng 3  Hạn T6│ Vương quốc Số · Ông Số     │
│ Ngày (3)  ⟳4h │ ○ Daily: 3 bài Toán    1/3    │ "Cây cầu đá bị gãy…"       │
│ Tuần (3)      │ ○ Khám phá bờ biển     2/5    │ Bước:                      │
│ Sự kiện       │ ○ Party: Rồng Phân Số  (nhóm) │ ✓ 1 Gặp Ông Số             │
│ Đã xong       │                               │ ▶ 2 Giải 5 câu đo độ dài   │
│               │                               │ ○ 3 Mang đá về cầu         │
│ [Lọc môn ▾]   │                               │ Phần thưởng: 120 XP, 30 Xu │
│               │                               │ [Theo dõi] [Đi tới] [Bắt đầu]│
└───────────────┴───────────────────────────────┴───────────────────────────┘
```

**5. Bố cục Mobile** (điện thoại ngang: hai cột — danh sách và chi tiết; nhóm thành tab trên đầu; tablet: ba cột như PC)

```text
┌──────────────────────────────────────────────────────────────┐
│ Nhiệm vụ [Đang làm][Nhà trường giao 2][Ngày][Tuần][Sự kiện][Xong] [✕]│
├──────────────────────────┬───────────────────────────────────┤
│ ★ Sửa cây cầu      C.1   │ Sửa cây cầu · Ông Số               │
│ ☐ Phép nhân bảng 3 T6    │ ✓ Gặp Ông Số                       │
│ ○ Daily 3 bài Toán 1/3   │ ▶ Giải 5 câu đo độ dài             │
│ ○ Khám phá bờ biển 2/5   │ 120 XP · 30 Xu                     │
│                          │ [Theo dõi] [Đi tới] [ Bắt đầu ]     │
└──────────────────────────┴───────────────────────────────────┘
```

**6. Thành phần.** `FilterTabs`, `QuestList` (ảo hóa), `QuestRow` (loại, tên, tiến độ, hạn), `QuestDetail` (NPC, mô tả, danh sách bước, phần thưởng), `StepList`, `RewardChips`, `DeadlineBadge`, `ResetCountdown` (ngày/tuần), `ActionBar`.

**7. Dữ liệu & API.** `listQuests` (lọc trạng thái/loại), `getQuest`, `acceptQuest`, `abandonQuest`, `startQuestActivity` (bước học tập → S16), `reportQuestStep` (bước không học tập, do server xác nhận), WS `quest.progress.updated`. Bài nhà trường giao: `listMyAssignments` (xem S03). Phiên bản nội dung đã ghim theo quest đang làm ([CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), ADR-0043): journal hiển thị theo version đã ghim.

**8. Trạng thái.** Rỗng (tab Đang làm): "Hãy tìm NPC có dấu ! để nhận nhiệm vụ" + nút "Hỏi Mây". Bài giao quá hạn: nhãn "Đã quá hạn — vẫn làm được nếu bài cho phép nộp muộn" (`late_policy`) (màu trung tính). Quest thất bại (theo điều kiện thất bại): "Thử lại" không phạt, không mất phần đã xong. Hết hạn daily/weekly: mờ và chuyển vào "Đã xong/hết hạn". Đang tải: skeleton 5 hàng.

**9. Tương tác & phản hồi.** "Theo dõi" ghim lên HUD (tối đa theo grade band). "Đi tới": đặt điểm đến + đường dẫn sáng trên mặt đất trong vùng hiện tại; khác vùng thì đề xuất dịch chuyển. Hủy quest có hộp xác nhận. Hoàn thành quest: hộp phần thưởng ngắn rồi về danh sách.

**10. Khác biệt theo grade band.** `g1_3`: chỉ 3 tab (Đang làm, Nhà trường giao, Đã xong); thẻ quest và hướng dẫn bước có lồng tiếng + phụ đề kèm icon, mỗi quest một thẻ lớn có hình NPC, bước hiển thị bằng biểu tượng; `g4_5`: đủ tab, ẩn bộ lọc môn; `g6_9`: đủ tab + lọc môn + sắp xếp theo hạn.

**11. Song ngữ.** Khóa: `ui.quest.title`, `ui.quest.tab.active|assigned|daily|weekly|event|done`, `ui.quest.track`, `ui.quest.untrack`, `ui.quest.go_to`, `ui.quest.start`, `ui.quest.abandon`, `ui.quest.abandon_confirm`, `ui.quest.reward`, `ui.quest.due`, `ui.quest.overdue`, `ui.quest.resets_in`, `ui.quest.empty`. Tên/mô tả/bước quest là `i18nText`; bài giao do nhà trường đặt tên chỉ có ngôn ngữ người tạo nhập (hiển thị nguyên văn). Chuỗi dài nhất: mô tả quest (cuộn trong panel).

**12. Trợ năng.** Danh sách đọc theo thứ tự hàng; trạng thái bước bằng biểu tượng + chữ; bàn phím ↑/↓ chọn quest, Enter mở hành động chính.

**13. Hiệu năng máy tầm trung.** Danh sách ảo hóa (≤ 12 hàng tạo cùng lúc); chi tiết tải khi chọn; ảnh NPC là thumbnail 2D (ADR-0047); không xem trước 3D trong journal.

**14. An toàn/riêng tư.** Quest party chỉ hiện nickname thành viên; bài giao không hiện điểm của bạn khác.

**15. AC.**

| Mã | Tiêu chí |
|---|---|
| AC-UX-0501 | Bài nhà trường vừa publish xuất hiện ở tab "Nhà trường giao" và có thông báo 🔔 mà không cần khởi động lại game |
| AC-UX-0502 | "Bắt đầu" ở bước học tập mở S16 đúng hoạt động và version đã ghim |
| AC-UX-0503 | Quest thất bại/hết giờ không hiển thị thông điệp tiêu cực và giữ phần tiến độ hợp lệ theo QUEST_SYSTEM |
| AC-UX-0504 | Danh sách 100 quest cuộn mượt ≥ 30 FPS trên điện thoại tầm trung |

---

## S06 — Challenge / Boss

**1. Mục đích.** Chọn và chơi thử thách (11 loại, 5 mức độ khó) và boss nhiều phase, solo hoặc party; thấy rõ tiến độ boss, phase, thời gian và đóng góp — không tạo áp lực tiêu cực ([CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)).

**2. Người dùng & khối.** Mọi học sinh; `g1_3` chỉ mở easy–hard theo mặc định. Party vào boss khi mọi thành viên đủ điều kiện khối.

**3. Điểm vào / ra.** Vào: cổng thử thách/NPC ở vùng, Núi Thử Thách, quest bước `challenge`/`boss`, Party lobby (S10). Ra: kết quả thử thách/boss → S18 (phần học tập) → S03; "Rời" giữa chừng.

Gồm 3 trạng thái màn: **(a) Chọn** (thông tin + độ khó) → **(b) Đang chơi** (HUD boss/challenge phủ lên thế giới, câu hỏi qua S16/S17) → **(c) Kết thúc** (tóm tắt + phần thưởng).

**4. Bố cục Web/PC**

```text
(a) Chọn
┌───────────────────────────────────────────────────────────────────────────┐
│ Rồng Phân Số · Boss Chương 1                                   [✕ Esc]     │
├──────────────────────────────┬────────────────────────────────────────────┤
│ (thumbnail boss render sẵn)   │ Chủ đề: Phân số (Toán 4) · 3 phase         │
│                              │ Độ khó: (Dễ)(●Thường)(Khó)(Chuyên gia🔒)    │
│                              │ Thời gian mỗi câu: 30 s · Lượt hôm nay 3/5  │
│                              │ Phần thưởng: Huy hiệu Rồng, 200 XP, 50 Xu  │
│                              │ Party: 3/4 sẵn sàng  [Mời thêm]            │
│                              │ [Chơi một mình]          [ Vào trận ]      │
└──────────────────────────────┴────────────────────────────────────────────┘

(b) Đang chơi — HUD boss
┌───────────────────────────────────────────────────────────────────────────┐
│ Rồng Phân Số  [██████████░░░░░░░░░ năng lượng 52%]   Phase 2/3  ⏱ 01:45    │
│ Party: Cú ✓ · Mèo ✓ · Rồng … · cần mỗi bạn đạt phần đóng góp [Rời]          │
│                         (thế giới 3D: boss + nhân vật)                     │
│         ┌──────────── Câu hỏi (S16/S17, panel giữa-dưới) ─────────┐        │
│         │ 3/4 + 1/8 = ?    (A) 7/8   (B) 4/12   (C) 1   (D) 5/8   │        │
│         └─────────────────────────────────────────────────────────┘        │
└───────────────────────────────────────────────────────────────────────────┘

(c) Kết thúc: "Hạ Rồng Phân Số!" hoặc "Em đã tới phase 2/3" + phần thưởng + [Xem kết quả học tập] [Về vùng]
```

**5. Bố cục Mobile** (điện thoại ngang; tablet: như PC)

```text
(b) Đang chơi
┌──────────────────────────────────────────────────────────────┐
│ Rồng Phân Số [███████░░░░ 52%] P2/3 ⏱01:45            [Rời]  │
│ Cú ✓  Mèo ✓  Rồng ✓ (đã đạt phần đóng góp)                    │
│               (boss 3D, camera cố định)                       │
│ ┌──────────────────────────────────────────────────────────┐ │
│ │ 3/4 + 1/8 = ?                                             │ │
│ │ [ 7/8 ] [ 4/12 ] [ 1 ] [ 5/8 ]     (nút ≥ 48/64 dp)        │ │
│ └──────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
(a) Chọn: thumbnail trên-trái, thông tin + độ khó dạng danh sách cuộn bên phải, nút "Vào trận" cố định dưới-phải.
```

Trong lúc chơi trên mobile **ẩn joystick** (camera cố định kiểu sân khấu) để vùng câu hỏi đủ rộng.

**6. Thành phần.** `ChallengeInfoCard`, `DifficultySelector` (khóa kèm điều kiện), `AttemptCounter`, `RewardPreview`, `PartyReadyList`, `BossEnergyBar`, `PhaseIndicator`, `PhaseTimer`, `ContributionList` (tiến độ cá nhân so với `min_contribution`, không xếp hạng), `QuestionPanelHost` (nhúng renderer S16/S17), `ResultSummary`.

**7. Dữ liệu & API.** Thử thách: `listGameChallenges`, `getGameChallenge`, `startGameChallenge`, `nextChallengeActivity`, `getGameChallengeAttempt`, `abandonGameChallengeAttempt`, WS `challenge.attempt.updated`. Boss: `listBosses`, `createBossEncounter`, `getBossEncounter`, `leaveBossEncounter`; WS lệnh `boss.encounter.ready`, `boss.answer.submit`; WS sự kiện `boss.encounter.updated`, `boss.phase.started`, `boss.phase.completed`, `boss.encounter.ended`. Chấm điểm do server ([ADR-0016](../adr/ADR-0016-server-authoritative-scoring-answer-key-khong-bao-gio-roi-se.md)); timer hiển thị theo `deadline_at` của server.

**8. Trạng thái.**

| Trạng thái | Hiển thị |
|---|---|
| Hết lượt hôm nay | "Em đã dùng hết lượt hôm nay, quay lại vào ngày mai nhé" + gợi ý luyện tập không giới hạn |
| Cooldown sau thất bại (mức khó) | Đồng hồ đếm ngược nhẹ + gợi ý ôn LO |
| Chờ party | Danh sách sẵn sàng; người chưa sẵn sàng có biểu tượng đồng hồ |
| Trả lời sai | Boss "phản đòn" vui, giải thích ngắn; thanh năng lượng **không** tăng lại |
| Hết giờ phase | "Thử lại phase với câu hỏi mới", giữ phase trước |
| Mất kết nối | "Đang kết nối lại…" trong thời gian grace của boss ([MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md)); về lại đúng phase |
| Kết thúc chưa hạ boss | "Em đã tới phase 2/3" + LO nên ôn, không chữ "Thất bại" |

**9. Tương tác & phản hồi.** Đúng: hiệu ứng tia sáng bắn vào boss, thanh năng lượng giảm, rung nhẹ (mobile). Chuyển phase: boss đổi hình dạng ≤ 2 s, câu thoại song ngữ. Hạ boss: pháo hoa ngắn + phần thưởng. Không hiển thị bảng "ai kém nhất" trong party.

**10. Khác biệt theo grade band.** `g1_3`: chỉ easy–hard, thời gian/câu ×1,5 (giá trị thuộc CHALLENGE_BOSS_SYSTEM), timer dạng thanh màu không có số giây, đọc to câu hỏi; `g4_5`: timer có số; `g6_9`: hiện thêm hệ số thưởng theo độ khó.

**11. Song ngữ.** Khóa: `ui.challenge.difficulty.easy|normal|hard|expert|legendary`, `ui.challenge.attempts_left`, `ui.challenge.time_per_question`, `ui.challenge.solo`, `ui.challenge.enter`, `ui.challenge.no_attempts`, `ui.boss.energy`, `ui.boss.phase`, `ui.boss.need_contribution`, `ui.boss.leave`, `ui.boss.leave_confirm`, `ui.boss.defeated`, `ui.boss.reached_phase`, `ui.boss.retry_phase`. Tên boss, câu thoại boss (`presentation`) là `i18nText`. Câu hỏi hiển thị theo ngôn ngữ nội dung ([ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md) §3). Chuỗi dài nhất: `ui.boss.need_contribution`.

**12. Trợ năng.** Timer có tùy chọn "thêm thời gian" nếu trường bật cho học sinh cần hỗ trợ [Confirmed]; thanh năng lượng có số %; âm thanh không phải là tín hiệu duy nhất.

**13. Hiệu năng máy tầm trung.** Cảnh boss là arena riêng nhẹ (ngân sách cảnh trong [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md)); tối đa 8 nhân vật; hiệu ứng tấn công dùng particle ≤ 200 hạt (preset Trung bình), tắt ở preset Thấp; camera cố định trên mobile; panel câu hỏi là Control 2D (không SubViewport 3D).

**14. An toàn/riêng tư.** Danh sách đóng góp chỉ hiện dấu ✓ khi từng bạn đã đạt phần đóng góp tối thiểu (`contribution_met` từ server — [CHALLENGE_BOSS_SYSTEM §3.5](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)), **không** hiện số câu đúng của bạn khác; kết quả học tập riêng từng người.

**15. AC.**

| Mã | Tiêu chí |
|---|---|
| AC-UX-0601 | Mức độ khó bị khóa hiển thị điều kiện mở; `g1_3` không thấy expert/legendary khi chưa được trường bật |
| AC-UX-0602 | Timer phase khớp `deadline_at` của server (lệch hiển thị ≤ 1 s) |
| AC-UX-0603 | Trả lời sai không làm tăng lại năng lượng boss và không có từ ngữ tiêu cực |
| AC-UX-0604 | Mất kết nối trong grace rồi vào lại: về đúng phase, tiến độ không mất |
| AC-UX-0605 | Boss party 4 người ≥ 30 FPS trên điện thoại tầm trung ở preset Trung bình |

---

## S07 — Level-up

**1. Mục đích.** Ăn mừng khi học sinh lên cấp, cho thấy danh hiệu/phần thưởng mới và mục tiêu tiếp theo, rồi trả lại quyền điều khiển nhanh ([PROGRESSION_SYSTEM §4.3–§4.6](../01-game-design/PROGRESSION_SYSTEM.md#4-level-và-đường-cong-xp)).

**2. Người dùng & khối.** Mọi học sinh.

**3. Điểm vào / ra.** Vào: WS `progression.level.reached` (thường sau S18 hoặc hoàn thành quest). Nếu đang trong hoạt động có giờ (S06 phase, S16 bài kiểm tra), **hoãn** đến khi kết thúc. Ra: "Tuyệt!" về màn trước; "Trang bị ngay" mở S08 đúng vật phẩm; "Hành trình của tôi" mở lịch sử.

**4. Bố cục Web/PC**

```text
┌───────────────────────────────────────────────────────────────────────────┐
│                        (thế giới mờ phía sau — lớp tối 40%)                │
│                ┌─────────────────────────────────────────┐                │
│                │        ✦  LÊN CẤP 20!  ✦                 │                │
│                │   Danh hiệu mới: Người phiêu lưu         │                │
│                │   (nhân vật ăn mừng — xem trước 3D)       │                │
│                │   Phần thưởng: [🎩 Mũ thám hiểm][😊 Emote][Xu 50] │        │
│                │   Còn 640 XP nữa tới cấp 21               │                │
│                │ [Hành trình của tôi] [Trang bị ngay] [ Tuyệt! ]│          │
│                └─────────────────────────────────────────┘                │
└───────────────────────────────────────────────────────────────────────────┘
```

**5. Bố cục Mobile** (điện thoại ngang: thẻ chiếm 70% chiều cao, phần thưởng cuộn ngang; tablet: như PC)

```text
┌──────────────────────────────────────────────────────────────┐
│        ┌────────────────────────────────────────────┐        │
│        │ ✦ LÊN CẤP 20! ✦   Người phiêu lưu           │        │
│        │ (ăn mừng)   [🎩][😊][Xu 50]  ◀ ▶            │        │
│        │ [Trang bị ngay]              [ Tuyệt! ]      │        │
│        └────────────────────────────────────────────┘        │
└──────────────────────────────────────────────────────────────┘
```

**6. Thành phần.** `CelebrationModal`, `LevelBadgeLarge`, `TitleReveal`, `RewardCarousel` (độ hiếm theo màu `rarity.*` + biểu tượng), `AvatarPreview3D` (dùng lại của S02), `NextLevelHint`, `ActionBar`.

**7. Dữ liệu & API.** Payload `progression.level.reached` (level mới, danh hiệu, phần thưởng đã cấp); chi tiết: `getProgression`, `listProgressionLevels`; lịch sử: `listXpHistory` ("Hành trình của tôi"). Phần thưởng đã được server cấp idempotent trước khi hiển thị (không có nút "Nhận" phải bấm mới được).

**8. Trạng thái.** Nhiều level cùng lúc: hiển thị level cao nhất, gộp phần thưởng, ghi "Lên 2 cấp!". Phần thưởng chưa tải được ảnh: biểu tượng chung + tên. Mất mạng: modal vẫn hiển thị từ payload đã nhận. Đang trong hoạt động có giờ: xếp hàng, hiện sau.

**9. Tương tác & phản hồi.** Hiệu ứng ≤ 2,5 s (tia sáng + chữ lớn), âm thanh fanfare ngắn, rung nhẹ; bấm bất kỳ đâu ngoài nút để bỏ qua hiệu ứng; tự đóng sau 10 s nếu không thao tác (phần thưởng vẫn nằm trong túi đồ).

**10. Khác biệt theo grade band.** `g1_3`: chỉ một nút lớn "Tuyệt!", đọc to "Em đã lên cấp 20!", phần thưởng dạng hình lớn; `g4_5`: thêm "Trang bị ngay"; `g6_9`: thêm XP còn lại và "Hành trình của tôi".

**11. Song ngữ.** Khóa: `ui.levelup.title` (có tham số `{level}`), `ui.levelup.multi` (`{count}`), `ui.levelup.new_title`, `ui.levelup.rewards`, `ui.levelup.xp_to_next` (`{xp}`, `{next}`), `ui.levelup.equip_now`, `ui.levelup.journey`, `ui.levelup.ok`. Danh hiệu là `i18nText` (vi "Người phiêu lưu" / en "Adventurer"). Dùng định dạng số theo ngôn ngữ (vi "1.250", en "1,250"). Chuỗi dài nhất: `ui.levelup.xp_to_next`.

**12. Trợ năng.** Giảm chuyển động: thay hiệu ứng bằng thẻ tĩnh; thông báo cho trình đọc màn hình đọc level + danh hiệu; focus mặc định ở "Tuyệt!".

**13. Hiệu năng máy tầm trung.** Hiệu ứng là particle 2D ≤ 64 hạt + tween; xem trước 3D dùng lại `SubViewport` của S02 (nạp khi cần, giải phóng khi đóng); không dừng thế giới 3D nhưng giảm render scale nền 30% trong lúc modal mở.

**14. An toàn/riêng tư.** Không thông báo level-up của mình cho người lạ; bạn bè chỉ thấy level mới trên thẻ nickname.

**15. AC.**

| Mã | Tiêu chí |
|---|---|
| AC-UX-0701 | Level-up trong lúc làm bài có giới hạn thời gian không ngắt bài; modal hiện ngay sau khi kết thúc |
| AC-UX-0702 | Phần thưởng đã có trong túi đồ trước khi modal hiện; không có thao tác "nhận" bắt buộc |
| AC-UX-0703 | Hiệu ứng có thể bỏ qua bằng 1 thao tác; giảm chuyển động thay bằng thẻ tĩnh |
| AC-UX-0704 | Lên nhiều cấp cùng lúc chỉ hiện một modal gộp |

---

## Tổng hợp đề xuất phát sinh

| Loại | Nội dung | Nhãn |
|---|---|---|
| Endpoint | `GET /api/v1/game/profile/nickname-suggestions` (`suggestNicknames`) — gợi ý nickname duy nhất trong tenant theo ngôn ngữ | [Confirmed] |
| Endpoint | `GET /api/v1/learning/my-assignments` (`listMyAssignments`, đã có trong OpenAPI) — bài nhà trường giao cho học sinh hiện hành; gộp vào `listQuests` nguồn `assignment` | [Confirmed] |
| Quy ước | Ảnh minh họa bản đồ/nền không chứa chữ (để dịch) | [Confirmed] |
| Quy ước | Level-up hoãn khi đang trong hoạt động có giới hạn thời gian | [Confirmed] |
| Giới hạn | Nickname tối đa 24 ký tự | [Confirmed] |

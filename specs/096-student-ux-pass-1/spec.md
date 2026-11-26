# Feature Specification: F096 — Student UX pass 1: nhận diện EduWorld và làm mượt hành trình học sinh

**Feature Branch**: `096-student-ux-pass-1` · **Created**: 2026-11-26 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`, `design/icons/game`, `src/game/web`) · **Mốc**: M3 · **Phụ thuộc**: F060, F061, F062, F063, F064, F070, F071, F133
**Input**: Owner 2026-11-26 (qua orchestrator): (1) "thiết kế game thật dễ dùng và thu hút học sinh" (khối 1–9); (2) "màn hình Godot Engine có thể đổi thành giao diện chuyên về Eduworld hay không?" — thay mọi màn mang nhận diện Godot bằng nhận diện EduWorld. Owner đánh giá bằng bản Web (renderer Compatibility). Feature chỉ đổi phía client (Godot + Web shell + token/icon), không đổi luật gameplay hay hợp đồng backend (AGENTS R34). FR nguồn: FR-GAME-001, FR-GAME-003, FR-GAME-010, FR-AUTH-002, FR-RWD-002; NFR-A11Y-001, NFR-I18N-001, NFR-I18N-003, NFR-COMP-004, NFR-MAINT-007, NFR-SAFE-003.

## Clarifications

### Session 2026-11-26

Owner có lệnh thường trực "chốt hết": điểm thiếu được chốt bằng mặc định an toàn, thân thiện với trẻ nhất và ghi [Confirmed].

- Q: Phạm vi lượt này? → A: [Confirmed — giao việc orchestrator 2026-11-26] Phần A (nhận diện) làm trọn; phần B sửa các vấn đề tác động lớn nhất của bảng kiểm (mục "Bảng kiểm UX") trong mức giữ được chất lượng; phần còn lại ghi follow-up ở "Ngoài phạm vi". Không đổi luật gameplay, không đổi contract backend.
- Q: Ảnh boot splash lấy từ đâu? → A: [Confirmed] Dựng bằng pipeline của dự án: công cụ `tools/brand_assets.gd` vẽ lại đúng các thành phần S01 đã có (`SparkMark` Tia Sáng + wordmark "EduWorld" theme `LogoLabel` + `CloudyMascot` Mây) ra PNG; không vẽ tay ngoài pipeline. Asset tự làm, MIT (ADR-0102), có dòng trong `src/game/assets/LICENSES.md`. Nền boot splash = màu token `component.backdrop.sky_top` (cùng nền trời của S01), lọc ảnh bật, giãn "keep" (giữ tỉ lệ).
- Q: Màn chờ Web có dùng được màu token không? → A: [Confirmed — đã có từ F133] `src/game/web/shell_tokens.css` sinh bởi `make design-tokens`; lượt này mở rộng tiền tố sinh ra (logo, nền trời, Mây, thanh tiến độ) để shell không có literal màu (R32). Mây và Tia Sáng trong shell là SVG nội tuyến tô bằng biến CSS.
- Q: Icon ứng dụng? → A: [Confirmed] Một icon EduWorld (Tia Sáng trên nền trời bo góc, có Mây) sinh cùng công cụ: `application/config/icon` (PC/Web favicon + apple-touch-icon do `html/export_icon`), Android `launcher_icons/main_192x192` + adaptive foreground/background 432, iOS `icons/icon_1024x1024`. Tiêu đề cửa sổ và tab "EduWorld" (`config/name`, `$GODOT_PROJECT_NAME`).
- Q: Không tài liệu nào dự kiến màn "Thông tin / Giấy phép" — đặt thông báo giấy phép Godot (MIT) ở đâu? → A: [Confirmed — cách nhỏ nhất khớp spec] Thêm nhóm **Thông tin** vào Cài đặt (S15) với nút "Giấy phép" mở một trang cuộn trong panel Cài đặt: phiên bản game, văn bản MIT của Godot lấy từ `Engine.get_license_text()`, danh sách thành phần bên thứ ba của engine từ `Engine.get_copyright_info()`, font OFL (Baloo 2, Be Vietnam Pro) và asset tự làm MIT. Văn bản pháp lý hiển thị nguyên văn tiếng Anh (không dịch); tiêu đề, nút qua `.po`. Ghi vào STUDENT_GAME_UX_SOCIAL §10.
- Q: Hướng dẫn lần đầu hiện khi nào trên máy dùng chung (Web phòng máy)? → A: [Confirmed] Hiện khi (a) học sinh vừa tạo nhân vật S02 trong phiên này, hoặc (b) thiết bị chưa từng hiện hướng dẫn (cờ cục bộ `onboarding_hub_seen` trong `user://settings.cfg`, không phải dữ liệu nghiệp vụ, không PII). Luôn có "Bỏ qua"; tối đa 3 bước, mỗi bước một câu ngắn kèm icon; mở lại được từ Cài đặt ("Xem lại hướng dẫn"). Không chặn di chuyển sau khi đóng.
- Q: "Đi đâu tiếp?" hiển thị thế nào mà không đặt logic nghiệp vụ trong UI? → A: [Confirmed] Thẻ **Việc tiếp theo** luôn hiện bên trái HUD (quest tracker F072 ở bên phải, không chồng nhau): có bài nhà trường giao → "Làm bài nhà trường giao (n)" mở Nhật ký tab Nhà trường giao; không có → "Mở Bản đồ để khám phá" mở World Map. Chọn gợi ý bằng model thuần `NextStepHint` (đọc số bài từ AppState, không gọi mạng, không quyết định nghiệp vụ — R1).
- Q: Phản hồi phần thưởng thế nào cho vui mà không tạo áp lực (R19)? → A: [Confirmed — STUDENT_GAME_UX_LEARNING §10.3 "hiệu ứng chúc mừng ≤ 1,5 s", NFR-SAFE-003] Màn kết quả có dải chúc mừng: Mây vui, sao nhỏ bay lên một lần ≤ 1,5 s (tắt khi "Giảm chuyển động"), số sao = số câu đúng với `g1_3`; không đếm ngược, không "quay lại ngày mai", không chuỗi ngày, không so sánh với bạn, không thông điệp mất mát. Phần thưởng chỉ là thứ server đã trao.
- Q: Ký tự emoji (💡 🔔 🎉 ⭐ 🏅 🔊 🚩 🔥) trong chuỗi UI? → A: [Confirmed — NFR-COMP-004, bản Web không có font hệ thống] Bản Web chỉ có font đóng gói (Baloo 2, Be Vietnam Pro) nên emoji thành ô vuông. Mọi ký tự trong `vi.po`/`en.po` PHẢI có trong font đóng gói; biểu tượng dùng icon SVG `design/icons/game` (token màu) thay emoji.
- Q: Màn dọc (portrait)? → A: [Confirmed — STUDENT_GAME_UX §2 "ngang trên mọi nền tảng"] Không thiết kế bố cục dọc; ảnh 720×1280 chỉ để kiểm cửa sổ Web dọc không vỡ (canvas_items co giãn).
- Q: Lỗi 503 trên S01? → A: [Confirmed — giao việc orchestrator] Mã `UNAVAILABLE` và HTTP 502/503/504 không có Problem → "Hệ thống đang bận, em thử lại sau ít phút nhé." / "The system is busy. Please try again in a few minutes." dùng chung `ui.error.UNAVAILABLE`.
- Q: Nút "Hủy" khi đăng nhập lâu? → A: [Confirmed — STUDENT_GAME_UX_CORE S01 §8 "hủy được sau 10 s"] Sau 10 s ở trạng thái "Đang vào…" hiện nút phụ "Hủy" (vùng chạm theo khối); bấm → `AuthService.cancel_login()` (bỏ phản hồi đang chờ, xóa mật khẩu khỏi RAM, form dùng lại được).

**Câu hỏi mở**

- F096-1: S02 Tạo nhân vật — khung xem trước 3D nhỏ và nhiều khoảng trống, mẫu nhân vật chỉ có chữ (không ảnh) cho `g1_3` [Proposed: lượt UX sau, cần ảnh thumbnail mẫu từ pipeline `thumbnail_renderer`]. → **Đóng ở [F097](../097-student-ux-pass-2/spec.md)** (2026-11-26: sân khấu xem trước lớn + bàn xoay, mẫu là ô hình chân dung vẽ theo màu mẫu).
- F096-2: Lồng tiếng thật cho bước hướng dẫn của Mây (`g1_3`) cần asset audio có version [Proposed: khi có asset, bước hướng dẫn phát giọng; nay phụ đề + icon theo STUDENT_GAME_UX_LEARNING §7].
- F096-3: Phụ đề `VoiceCaption` lặp lại nguyên đề bài ngay trên đề (S16) — đúng spec F063 nhưng thừa chữ [Proposed: chỉ hiện phụ đề khi tắt tiếng/không có tai nghe, cần quyết định UX ở LEARNING]. → **Đóng ở [F097](../097-student-ux-pass-2/spec.md)** (2026-11-26: thẻ đề là phụ đề, đề hiện một lần).
- F096-4: Ở điểm spawn Hub, marker Tia Sáng (cao > 1,5 m, F094-AC-13) của `quest.hub.welcome_sign` (0, 0, 12) nằm giữa camera và nhân vật, che phần thân dưới [Proposed: làm mờ/ẩn marker khi cách camera < khoảng cách tầm với — quyết định ở WORLD_RENDERING, không dời seed]. → **Đóng ở [F097](../097-student-ux-pass-2/spec.md)** (2026-11-26: ẩn marker gần camera hơn 5 m và gần hơn nhân vật, WORLD_RENDERING §5.1).

## Bảng kiểm UX (trước khi sửa)

Chụp bằng `tools/capture_screens.gd` (renderer Compatibility = bản Web, fixture + fake, không backend) ở 1280×720, 1024×600 và 720×1280, `vi` và `en`, khối `g1_3` (và `g6_7` cho màn có khác biệt). Điểm 1 (kém) – 5 (tốt) theo: **R**õ ràng · **D**ễ dùng · **T**hu hút · **P**hong cách chibi anime cartoon (ADR-0103) · **A** trợ năng (tương phản, không chỉ màu, chữ đọc được). Ảnh: `screenshots/<màn>-before-<cỡ>-<ngôn ngữ>-<khối>.png`.

| Màn | R | D | T | P | A | Vấn đề chính (trước) | Ưu tiên | Sửa ở |
|---|---|---|---|---|---|---|---|---|
| Boot splash (engine) | 1 | 3 | 1 | 1 | 3 | Logo Godot mặc định, nền xám | P1 | F096-AC-01 |
| Màn chờ Web (shell F133) | 3 | 4 | 1 | 1 | 4 | Chỉ panel chữ, thanh `<progress>` mặc định, nền xanh đậm, không logo/Mây; favicon là icon Godot | P1 | F096-AC-02, AC-03 |
| Splash trong game (`splash`) | 3 | 3 | 1 | 2 | 4 | Panel kem viền xanh, chỉ chữ "EduWorld / Đang tải…" | P1 | F096-AC-06 |
| S01 Đăng nhập (`login*`) | 4 | 3 | 4 | 4 | 4 | Đang vào không hủy được; 503 chỉ "Máy chủ đang bận" | P1 | F096-AC-16, AC-17 |
| S01 Tự vào lại, giờ nghỉ (`resume`, `curfew`) | 3 | 3 | 1 | 2 | 4 | Panel trống, không nhận diện, không Mây | P2 | F096-AC-06 |
| S02 Tạo nhân vật (`character`) | 3 | 3 | 2 | 3 | 3 | Nhân vật xem trước nhỏ, nhiều khoảng trống, mẫu chỉ có chữ | P2 | Câu hỏi mở F096-1 |
| S03 Vào Hub lần đầu (`hub`) | 1 | 2 | 1 | 2 | 2 | **Camera nằm sát đất, chui vào marker biển chào: không thấy gì**; thanh toast tối rỗng; chưa có hướng dẫn, chưa có "đi đâu" | P1 | F096-AC-07, AC-08, AC-10, AC-11 |
| S03 HUD (`hub`, `hub_offline`) | 3 | 2 | 2 | 3 | 3 | 6 nút chữ không icon (lớp 1–3 cần icon + chữ), "(J)" là phím PC, "Emote" chưa dịch, chuông là emoji | P1 | F096-AC-09, AC-15 |
| S05 Nhật ký / Nhà trường giao (`journal`) | 4 | 4 | 3 | 3 | 4 | Đọc được; trạng thái bằng ký hiệu chữ | P3 | — |
| S16 Làm bài (`activity*`) | 4 | 4 | 3 | 3 | 3 | Emoji 💡 🔇 (ô vuông trên Web) | P1 | F096-AC-15 |
| S18 Kết quả (`result`) | 3 | 3 | 2 | 2 | 3 | Hai khung lớn trống, emoji 🎉 ⭐ 🏅, header còn "1/11", không có khoảnh khắc chúc mừng | P1 | F096-AC-14, AC-15 |
| S04 Bản đồ (`world_map`) | 1 | 1 | 1 | 1 | 1 | `g1_3`: ô vuông xanh trống (không hình, không tên); `g4+`: hàng vỡ (điều kiện rộng 1 ký tự, nút "Đi" mất chữ) | P1 | F096-AC-12 |
| S15 Cài đặt (`settings`) | 3 | 3 | 2 | 3 | 2 | Preset đang chọn chữ tối trên nền xanh đậm (không đạt AA); ngôn ngữ đang chọn không hiện; không có giấy phép | P1 | F096-AC-04, AC-13 |

### Sau khi sửa (2026-11-26)

Ảnh: `screenshots/<màn>-after-<cỡ>-<ngôn ngữ>-<khối>.png` (Compatibility) và `screenshots/web_*-after-*.png` (shell Web trong Chromium). Điểm theo cùng thang R · D · T · P · A.

| Màn | Trước | Sau | Đã sửa |
|---|---|---|---|
| Boot splash (engine) | 1·3·1·1·3 | 5·5·4·5·5 | Ảnh EduWorld (Tia Sáng + chữ + Mây) trên nền trời token, không logo Godot |
| Màn chờ Web | 3·4·1·1·4 | 5·5·4·5·5 | Logo, Mây nói "Đang chuẩn bị thế giới cho em…", thanh bo tròn; favicon/apple-touch-icon EduWorld |
| Splash trong game | 3·3·1·2·4 | 5·5·4·5·5 | Nền trời + logo + Mây nói trạng thái (liền mạch với boot splash) |
| S01 Đăng nhập | 4·3·4·4·4 | 4·5·4·4·4 | Nút "Hủy" sau 10 s; 503 → "Hệ thống đang bận, em thử lại sau ít phút nhé." |
| S01 Tự vào lại, giờ nghỉ, hàng chờ | 3·3·1·2·4 | 5·4·4·5·5 | Cùng bố cục nhận diện, Mây nói nội dung |
| S02 Tạo nhân vật | 3·3·2·3·3 | 3·3·2·3·3 | Không sửa lượt này (F096-1); nút ‹ › thay ◀ ▶ (không có trong font) |
| S03 Vào Hub lần đầu | 1·2·1·2·2 | 4·4·4·4·4 | Camera sau lưng, nhìn xuống; Mây hướng dẫn 3 bước; thẻ "Việc tiếp theo"; toast có chữ |
| S03 HUD | 3·2·2·3·3 | 5·4·4·4·4 | Icon + chữ mọi nút, "Biểu cảm", không "(J)", chuông là icon + số |
| S05 Nhật ký | 4·4·3·3·4 | 4·4·3·4·4 | Icon trạng thái thay ký hiệu ✓ ngoài font |
| S16 Làm bài | 4·4·3·3·3 | 4·4·3·4·4 | ✓/✗ phản hồi, loa, gợi ý là icon (trước đây ô vuông trên Web); timer không dùng ⏳ |
| S18 Kết quả | 3·3·2·2·3 | 5·4·4·4·4 | Dải chúc mừng (Mây, sao icon cho `g1_3`, ≤ 1,5 s), phần thưởng có icon, bỏ "1/11" |
| S04 Bản đồ | 1·1·1·1·1 | 4·5·4·4·4 | `g1_3`: thẻ icon + tên; khối khác: hàng icon/tên/tiến độ/nút "Đến đây" có chữ |
| S15 Cài đặt | 3·3·2·3·2 | 4·4·3·4·4 | Chip có ✓ cho ngôn ngữ/preset (đạt AA), nhóm Thông tin: Giấy phép, Xem lại hướng dẫn |

Còn lại (ghi follow-up): marker Tia Sáng của biển chào (`quest.hub.welcome_sign`, seed F072) đứng ngay trước nhân vật ở điểm spawn Hub, che một phần thân (F096-4: làm mờ marker gần camera — cần quyết định WORLD_RENDERING); S02 (F096-1).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Thấy EduWorld từ giây đầu, không thấy Godot (Priority: P1)

Học sinh mở bản Web/ứng dụng: tab, favicon, icon ứng dụng, màn chờ tải và màn khởi động đều là EduWorld (Tia Sáng, chữ "EduWorld", Mây, nền trời ấm), không còn logo hay icon Godot; thông báo giấy phép MIT của Godot vẫn xem được trong Cài đặt.

**Why this priority**: Owner yêu cầu trực tiếp; ấn tượng đầu quyết định trẻ có muốn chơi.

**Independent Test**: `test_branding.gd` (ProjectSettings, preset export, file sinh), `tools/brand_assets.gd --check`, `node --test src/game/web/tests/`, export Web (`index.icon.png`, `index.apple-touch-icon.png` là icon EduWorld), ảnh shell trong Chromium.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F096-AC-01 | HỆ THỐNG PHẢI đặt `application/boot_splash/*` của project thành ảnh EduWorld sinh bởi pipeline (Tia Sáng + wordmark "EduWorld" + Mây), nền màu token `component.backdrop.sky_top`, lọc ảnh bật, giữ tỉ lệ; boot splash KHÔNG ĐƯỢC là ảnh mặc định của Godot | Owner 2026-11-26 · ADR-0102 · `GODOT_ARCHITECTURE` §5 · AGENTS R32 | TC-UI-001 |
| F096-AC-02 | KHI bản Web đang tải, shell PHẢI hiện logo EduWorld, Mây, thanh tiến độ bo tròn và câu thân thiện ("Đang chuẩn bị thế giới cho em…" / "Getting your world ready…") theo ngôn ngữ trình duyệt, KHÔNG có logo Godot, mọi màu từ biến CSS sinh từ token; và PHẢI giữ kiểm tra WebGL 2, thông báo trình duyệt chưa hỗ trợ, khởi động âm thanh sau cử chỉ đầu tiên và chính sách heap của F133 | Owner 2026-11-26 · F133-AC-16 · `PERFORMANCE_BUDGET` §14 · NFR-COMP-004 · AGENTS R32 | TC-UI-003 |
| F096-AC-03 | HỆ THỐNG PHẢI đặt icon EduWorld sinh bởi pipeline cho project (`application/config/icon`, dùng cho favicon và apple-touch-icon của Web qua `html/export_icon`), Android (launcher 192, adaptive foreground/background 432) và iOS (1024), và tiêu đề cửa sổ/tab là "EduWorld" | Owner 2026-11-26 · ADR-0101 · ADR-0102 | — |
| F096-AC-04 | KHI học sinh mở Cài đặt → Thông tin → Giấy phép, HỆ THỐNG PHẢI hiện phiên bản game, văn bản giấy phép MIT của Godot (từ engine), danh sách thành phần bên thứ ba của engine, font OFL và asset MIT của dự án | Godot MIT · ADR-0102 · `STUDENT_GAME_UX_SOCIAL` §10 | — |
| F096-AC-05 | HỆ THỐNG PHẢI tái tạo được mọi ảnh nhận diện (boot splash, icon) bằng `tools/brand_assets.gd` từ các thành phần S01 và token; NẾU file ảnh, kích thước, cài đặt project/preset hay dòng `LICENSES.md` lệch, THÌ `brand_assets.gd --check` và test PHẢI thất bại | ADR-0102 · NFR-MAINT-007 | TC-UI-001 |
| F096-AC-06 | Các màn chờ/thông báo trước khi vào thế giới (splash trong game, "Đang vào lớp…", giờ nghỉ, hàng chờ) PHẢI dùng nền trời S01, logo EduWorld và Mây thay cho panel trống | `STUDENT_GAME_UX_CORE` S01 §8 · `DESIGN_SYSTEM` §5.1 | — |

---

### User Story 2 — Vào Hub là biết mình ở đâu và đi đâu (Priority: P1)

Lần đầu vào Quảng trường Tia Sáng, học sinh thấy nhân vật của mình từ phía sau, Mây chào và chỉ 3 điều quan trọng (bài nhà trường giao, bản đồ/cổng gió, cách đi), có thể bỏ qua; sau đó thẻ "Việc tiếp theo" luôn nói em nên làm gì; các nút HUD có hình và chữ.

**Why this priority**: Màn vào Hub là nơi học sinh bỏ cuộc nếu không thấy gì hoặc không biết bấm đâu (bảng kiểm: Hub 1–2 điểm).

**Independent Test**: `test_camera_rig.gd`, `test_main_hud.gd`, `test_onboarding_guide.gd`, `test_notification_bell.gd`; ảnh `hub-*`, `hub_onboarding-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F096-AC-07 | KHI người chơi xuất hiện ở điểm spawn, camera góc ba PHẢI nằm phía trên và phía sau nhân vật với góc nghiêng ban đầu trong khoảng `pitch_min_deg`…`pitch_max_deg` của `CameraRig` | FR-GAME-003 · `STUDENT_GAME_UX` §4 · F062-AC-02 | — |
| F096-AC-08 | KHI HUD nhận một thông báo ngắn (toast), HỆ THỐNG PHẢI hiện đúng nội dung chữ của nó (không có thanh rỗng) | `STUDENT_GAME_UX_CORE` S03 §6 · F064-AC-20 | — |
| F096-AC-09 | Mỗi nút của HUD PHẢI có icon SVG từ `design/icons/game` kèm nhãn qua `.po` (không gợi ý phím trong nhãn; phím tắt ở tooltip), mọi nhãn đã dịch `vi`/`en`, và vùng chạm ≥ chiều cao nút tối thiểu của khối (≥ 64 dp với `g1_3`) | FR-GAME-010 · `STUDENT_GAME_UX` §4.3 · NFR-I18N-001 · NFR-A11Y-001 | TC-UI-003 |
| F096-AC-10 | KHI học sinh vào Hub lần đầu trên thiết bị hoặc ngay sau khi tạo nhân vật, HỆ THỐNG PHẢI hiện hướng dẫn của Mây tối đa 3 bước (mỗi bước một câu ngắn, icon, làm nổi nút HUD liên quan) có "Tiếp" và "Bỏ qua"; KHI hướng dẫn đóng, HỆ THỐNG PHẢI không hiện lại trên thiết bị đó trừ khi học sinh chọn "Xem lại hướng dẫn" trong Cài đặt | `WORLD_DESIGN` §2 (Mây hướng dẫn) · `STUDENT_GAME_UX_CORE` S03 · FR-GAME-010 | — |
| F096-AC-11 | TRONG KHI ở thế giới, HUD PHẢI luôn hiện thẻ "Việc tiếp theo": có bài nhà trường giao → mở Nhật ký tab Nhà trường giao; không có → mở World Map; gợi ý chọn bởi model thuần không gọi mạng | `STUDENT_GAME_UX_CORE` S03, S05 · AGENTS R1 | — |

---

### User Story 3 — Mọi màn đọc được, bấm được, đẹp trên Web (Priority: P1)

**Why this priority**: Bản đồ `g1_3` không dùng được, danh sách bản đồ vỡ, Cài đặt thiếu tương phản, emoji thành ô vuông trên Web.

**Independent Test**: `test_world_map.gd`, `test_settings_panel.gd`, `test_font_coverage.gd`, `test_grade_band_touch_targets.gd`; ảnh `world_map-*`, `settings-*`, `about-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F096-AC-12 | World Map PHẢI hiện với `g1_3` mỗi vùng là một ô hình lớn có icon theo `icon_key` và tên ngắn (chạm một lần để đi), và với khối khác mỗi hàng có tên, tiến độ hoặc điều kiện mở khóa (xuống dòng trong cột của nó) và nút "Đi" có chữ; không phần tử nào bị cắt ở 1280×720 và 1024×600 | `STUDENT_GAME_UX_CORE` S04 · F071-AC-26, AC-27 · NFR-I18N-003 | — |
| F096-AC-13 | Ở Cài đặt, lựa chọn đang chọn (ngôn ngữ, preset đồ họa) PHẢI hiện bằng nền nhấn + dấu ✓ icon (không chỉ màu) với chữ đạt tương phản WCAG AA | `STUDENT_GAME_UX` §8 · NFR-A11Y-001 · `DESIGN_SYSTEM` §5.1 | TC-UI-001 |
| F096-AC-15 | Mọi ký tự trong `vi.po` và `en.po` PHẢI có trong font đóng gói của Theme (Baloo 2, Be Vietnam Pro); NẾU một chuỗi chứa ký tự font không phủ (vd. emoji), THÌ test PHẢI thất bại; biểu tượng dùng icon SVG token | NFR-COMP-004 · NFR-I18N-003 · AGENTS R32 | TC-UI-003 |
| F096-AC-18 | HỆ THỐNG PHẢI có test scene đo vùng chạm của các màn đã sửa (HUD, hướng dẫn, thẻ Việc tiếp theo, World Map, Cài đặt, Giấy phép, kết quả, nút Hủy S01) ở cả 4 khối: mỗi nút ≥ chiều cao nút tối thiểu của khối (≥ 64 dp với `g1_3`) | `STUDENT_GAME_UX` §4.3 · AC-UX-0005 · NFR-A11Y-001 | — |

---

### User Story 4 — Phần thưởng vui, không áp lực (Priority: P1)

**Independent Test**: `test_activity_result.gd`; ảnh `result-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F096-AC-14 | KHI màn kết quả mở, HỆ THỐNG PHẢI hiện dải chúc mừng (Mây, sao icon; `g1_3` số sao = số câu đúng) với hiệu ứng một lần ≤ 1,5 s, không hiệu ứng TRONG KHI "Giảm chuyển động" bật; KHÔNG ĐƯỢC có đếm ngược, chuỗi ngày, lời mời quay lại, so sánh với người khác hay thông điệp mất mát; phần thưởng chỉ là phần server đã trao | NFR-SAFE-003 · FR-RWD-002 · AGENTS R5, R19 · `STUDENT_GAME_UX_LEARNING` §10.3 | — |

---

### User Story 5 — Đăng nhập không bị kẹt (Priority: P1)

**Independent Test**: `test_login_scene.gd`, `test_l10n.gd`; ảnh `login_busy_cancel-*`, `login_error_busy-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F096-AC-16 | TRONG KHI S01 ở "Đang vào…" quá 10 s, HỆ THỐNG PHẢI hiện nút "Hủy"; KHI học sinh bấm "Hủy", HỆ THỐNG PHẢI bỏ yêu cầu đang chờ, xóa mật khẩu khỏi bộ nhớ và trả form về trạng thái dùng được | FR-AUTH-002 · `STUDENT_GAME_UX_CORE` S01 §8 · F061-AC-06 | — |
| F096-AC-17 | NẾU backend trả `UNAVAILABLE` hoặc HTTP 502/503/504, THÌ HỆ THỐNG PHẢI hiện "Hệ thống đang bận, em thử lại sau ít phút nhé." (`en`: "The system is busy. Please try again in a few minutes.") ở S01 và mọi màn dùng `L10n.error_text` | FR-AUTH-002 · `GODOT_ARCHITECTURE` §9 · NFR-I18N-001 | — |

---

### User Story 6 — Bằng chứng duyệt (Priority: P1)

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F096-AC-19 | HỆ THỐNG PHẢI có ảnh trước/sau cho mỗi màn đã sửa ở 1280×720 và 1024×600 (thêm 720×1280 cho Hub và đăng nhập), `vi` và `en`, trên renderer Compatibility (`tools/capture_screens.gd`), cùng ảnh shell Web trong Chromium | `DESIGN_SYSTEM` §8, §9 · NFR-COMP-004 | TC-UI-007 |
| F096-AC-20 | Mọi chuỗi mới PHẢI qua khóa `.po` `vi`/`en`; `extract_pot --check`, `pseudo_locale --check` PHẢI pass và các màn đã sửa KHÔNG ĐƯỢC cắt chữ ở nút chính với pseudo-locale | NFR-I18N-001 · NFR-I18N-003 · `STUDENT_GAME_UX` §5 | — |

### Edge Cases

- NẾU "Giảm chuyển động" bật, THÌ dải chúc mừng và chỉ báo hướng dẫn không chạy tween; nội dung vẫn hiện đủ (F096-AC-14, F096-AC-10).
- NẾU đổi ngôn ngữ khi hướng dẫn hoặc trang Giấy phép đang mở, THÌ chữ đổi ngay, văn bản pháp lý giữ nguyên tiếng Anh (F096-AC-04, F096-AC-10).
- NẾU cửa sổ Web dọc (720×1280), THÌ UI co giãn theo `canvas_items` không chồng lấn (không có bố cục dọc riêng — Clarifications).
- NẾU học sinh bấm "Hủy" đúng lúc phản hồi đăng nhập về, THÌ phản hồi đó bị bỏ (thế hệ đăng nhập đã đổi), không vào game nửa chừng (F096-AC-16).
- NẾU trình duyệt không có WebGL 2, THÌ shell vẫn hiện trang thông báo của F133 (có logo, Mây) và không tải engine (F096-AC-02).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-001 | Nhận diện thân thiện, phong cách chibi cartoon ở màn chờ, splash, icon, chúc mừng | `DESIGN_SYSTEM` §5 · ADR-0103 |
| FR-GAME-003 | Camera góc ba đúng hướng khi vào thế giới | `STUDENT_GAME_UX` §4 · `GODOT_ARCHITECTURE` §6 |
| FR-GAME-010 | UI theo khối: icon + chữ, vùng chạm, hướng dẫn ngắn, bản đồ hình cho lớp 1–3 | `STUDENT_GAME_UX` §4.3 · `STUDENT_GAME_UX_CORE` S03, S04 |
| FR-AUTH-002 | Form đăng nhập: hủy khi chờ lâu, thông điệp máy chủ bận | `STUDENT_GAME_UX_CORE` S01 · `GODOT_ARCHITECTURE` §9 |
| FR-RWD-002 | Phản hồi phần thưởng không dark pattern, không loot box | `PROGRESSION_SYSTEM` · `CHILD_SAFETY` |

### Key Entities

- **Ảnh nhận diện** (`resources/branding/*.png`, sinh bởi `tools/brand_assets.gd`): boot splash, icon project/Android/iOS.
- **`BrandLockup`** (component UI): Tia Sáng + wordmark + Mây, dùng ở splash trong game, màn chờ và công cụ sinh ảnh.
- **`OnboardingGuide`** (model thuần) + cờ thiết bị `onboarding_hub_seen` (`user://settings.cfg`).
- **`NextStepHint`** (model thuần): gợi ý "Việc tiếp theo" từ số bài nhà trường giao.
- **Icon game mới** (`design/icons/game/*.svg` → `resources/ui/icons/`): nút HUD, vùng của World Map, sao, Mây vui, loa, gợi ý.
- **Token** `tokens@0.13.0`: biến shell CSS cho logo, nền trời, Mây, thanh tiến độ.

### Tham số

| Tham số | Dùng để |
|---|---|
| Thời gian hiện nút "Hủy" khi đăng nhập: 10 s (`STUDENT_GAME_UX_CORE` S01 §8) | F096-AC-16 |
| Hiệu ứng chúc mừng ≤ 1,5 s (`STUDENT_GAME_UX_LEARNING` §10.3) | F096-AC-14 |
| Số bước hướng dẫn tối đa: 3 (`STUDENT_GAME_UX_CORE` S03, mục hướng dẫn lần đầu) | F096-AC-10 |
| `quality_presets`, `renderer_default` (CONFIG_DEFAULTS) | Chụp ảnh Compatibility (F096-AC-19) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không logic nghiệp vụ trong scene; không gọi mạng từ UI | AGENTS R1 |
| Không hardcode màu/cỡ; file sinh không sửa tay | AGENTS R32 · NFR-MAINT-007 |
| Mọi chuỗi UI qua `.po` `vi`/`en` | NFR-I18N-001 |
| Không pay-to-win, không FOMO/dark pattern | AGENTS R19 · NFR-SAFE-003 |
| License asset MIT/OFL; giữ thông báo MIT của Godot | ADR-0102 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Không còn logo/icon Godot ở boot splash, màn chờ Web, favicon, apple-touch-icon và icon ứng dụng.
- **SC-002**: Mọi màn ở bảng kiểm đạt ≥ 3 điểm mỗi tiêu chí sau khi sửa (trừ S02 — Câu hỏi mở F096-1); Hub, World Map, Cài đặt, Kết quả tăng ít nhất 2 điểm ở tiêu chí kém nhất.
- **SC-003**: GdUnit4 toàn bộ xanh; mọi lint xanh; export Web thành công; test shell xanh.

## Ngoài phạm vi

- Đổi luật gameplay, phần thưởng, quest hay contract backend (R34) — mọi thay đổi cần backend ghi follow-up.
- S02 Tạo nhân vật bố cục lại + ảnh mẫu (F096-1), lồng tiếng hướng dẫn (F096-2), phụ đề trùng đề (F096-3).
- Màn chưa có trong client: túi đồ/shop/bộ sưu tập (F069), party, chat, đấu trường, sự kiện, Level-up S07 (F069/F074 UI).
- Bố cục dọc riêng; đổi phím trong Cài đặt; các nhóm Cài đặt chưa có (âm thanh, trợ năng đầy đủ).

## Assumptions

- Font đóng gói không có emoji; bản Web không dùng font hệ thống (Godot Web không hỗ trợ SystemFont).
- Ảnh nhận diện render bằng GPU nên `--check` so kích thước, cài đặt và nguồn, không so từng pixel giữa các máy.

# Feature Specification: F113 — Student UX pass 5: game luôn chơi ngang (khóa ngang Android/iOS, màn "Xoay ngang" trên Web)

**Feature Branch**: `113-student-ux-pass-5` · **Created**: 2026-11-26 · **Status**: Done · **Module**: `src/game/` (+ `src/game/web`, `design/icons/game`) · **Mốc**: M3 · **Phụ thuộc**: F096, F097, F098, F099, F133
**Input**: Lệnh thường trực của owner "thiết kế game thật dễ dùng và thu hút học sinh" (lớp 1–9) và câu hỏi của owner "điện thoại mặc định xoay ngang luôn được không?". Quy tắc đã chốt [STUDENT_GAME_UX §2](../../docs/05-ux/STUDENT_GAME_UX.md#2-nền-tảng-và-hướng-màn-hình): hướng màn hình **ngang trên mọi nền tảng**, không thiết kế bố cục dọc. Hôm nay `project.godot` không đặt hướng màn hình (Android/iOS dùng mặc định một chiều ngang, không xoay được sang chiều ngang kia), còn bản Web khi cửa sổ dọc (điện thoại cầm dọc, cửa sổ trình duyệt hẹp) vẫn vẽ game trên canvas logic 1280×2276 — chữ chỉ ≈ 56% cỡ token. Chỉ đổi hiển thị và cấu hình client; không đổi hợp đồng API hay luật nghiệp vụ (AGENTS R1, R32, R33, R34). FR nguồn: FR-GAME-009, FR-GAME-010; NFR-COMP-001, NFR-COMP-003, NFR-COMP-004, NFR-A11Y-001.

## Clarifications

### Session 2026-11-26

Owner có lệnh thường trực "chốt hết": điểm thiếu được chốt bằng mặc định an toàn, thân thiện với trẻ nhất, theo tài liệu đã [Confirmed], và ghi [Confirmed].

- Q: Có làm canvas dọc 720×1280 (F099-1) và kiểm "dải trống" cho dạng nhiều bước ở cửa sổ dọc (F099-2) không? → A: [Confirmed] **Không.** Cả hai bị thay thế (superseded) bởi quy tắc ngang trên mọi nền tảng của [STUDENT_GAME_UX §2](../../docs/05-ux/STUDENT_GAME_UX.md#2-nền-tảng-và-hướng-màn-hình) ("Không thiết kế bố cục dọc"). Hành vi khi cửa sổ dọc là màn "Xoay ngang" (dưới đây), không phải bố cục dọc. Bố cục dọc F097–F099 đã có (một cột thẻ đáp án, Mây liền dưới đáp án…) giữ lại làm dự phòng vô hại, không làm thêm. Ghi chú này cũng thêm vào Câu hỏi mở của [F099](../099-student-ux-pass-4/spec.md).
- Q: Android/iOS khóa hướng thế nào? → A: [Confirmed] `display/window/handheld/orientation` = **sensor landscape** (giá trị enum `4`, `DisplayServer.SCREEN_SENSOR_LANDSCAPE` của Godot 4.7): chỉ chơi ngang, được xoay giữa hai chiều ngang theo cảm biến. Preset Android không có tùy chọn ghi đè hướng; preset iOS không thêm `UISupportedInterfaceOrientations` qua `application/additional_plist_content` (Godot sinh "Landscape Left" + "Landscape Right" từ cài đặt dự án). Kiểm bằng test cấu hình đọc `project.godot` và `export_presets.cfg`.
- Q: Bản Web khi cửa sổ dọc? → A: [Confirmed] Trình duyệt không ép xoay được ở mọi nơi, nên: khi khung nhìn dọc (**cao > rộng**), game hiện màn toàn màn hình **"Xoay ngang điện thoại để chơi nhé!"** thay cho UI game — nền trời S01, Mây trong bong bóng lời, icon điện thoại xoay (tự vẽ, `design/icons/game/rotate_phone.svg`), lời phụ "Trò chơi đang chờ em. Xoay ngang là chơi tiếp ngay." và dòng cho máy tính "Trên máy tính, em kéo cửa sổ rộng ra nhé."; chuỗi qua `.po` vi/en. Khi khung nhìn ngang lại, màn ẩn và game chạy tiếp. Khung nhìn vuông (cao = rộng) coi là ngang.
- Q: Trong lúc màn "Xoay ngang" hiện, game làm gì? → A: [Confirmed] Game **tạm dừng** (`SceneTree.paused`): thế giới, activity, video/bài nghe, nhập liệu đều dừng và không nhận input; chỉ màn "Xoay ngang" chạy. Mạng và phiên **không dừng**: `ApiClient`, `RealtimeClient` chạy ở `PROCESS_MODE_ALWAYS` (request đang bay hoàn tất, WebSocket vẫn poll/heartbeat), heartbeat game session dùng timer `process_always` sẵn có — để không mất phiên, không mất trạng thái. Khi xoay lại: bỏ dừng, mọi scene giữ nguyên (không tải lại, không mất chữ đang gõ, không mất câu đang làm). Guard chỉ bỏ dừng nếu chính nó đã dừng (không phá dừng của nơi khác).
- Q: Có thử khóa ngang bằng API trình duyệt không? → A: [Confirmed] Có, im lặng khi không được: khi học sinh bấm "Chơi" trên thiết bị cảm ứng (`pointer: coarse`), shell thử `requestFullscreen()` rồi `screen.orientation.lock('landscape')` (Chrome Android hỗ trợ khi toàn màn hình, sau cử chỉ người dùng); trong game, khi chuyển sang dọc trên Web, client thử `screen.orientation.lock('landscape')` qua `JavaScriptBridge`. Mọi lỗi/không hỗ trợ (iOS Safari, máy tính) bỏ qua im lặng — không lỗi trang, không thông báo.
- Q: Trước khi engine chạy (màn chờ Web)? → A: [Confirmed] Shell `src/game/web/shell.html` hiện cùng lời nhắc bằng HTML (Mây SVG, icon xoay, vi/en theo ngôn ngữ trình duyệt, chỉ biến của `shell_tokens.css`) khi cửa sổ dọc ở trạng thái `checking`/`loading`/`ready`/`starting`; giữ nguyên hành vi F133/F096 (kiểm WebGL 2, banner trình duyệt không chính thức, chính sách heap, engine chỉ khởi động sau "Chơi"). Trang thông báo không hỗ trợ/lỗi **không** bị lời nhắc che (học sinh phải đọc được lý do). Điện thoại cầm ngang (cửa sổ từ 2:1) là đường chính nên màn chờ dùng Mây nhỏ (1/3 cỡ token) để logo, câu trạng thái và nút "Chơi" vừa 390 px chiều cao (trước: logo và nút bị cắt — phát hiện khi xem ảnh 844×390).
- Q: Chữ của màn "Xoay ngang" ở canvas dọc (logic 1280×2276 trở lên) có bị nhỏ không? → A: [Confirmed] Không: trong lúc màn nhắc hiện, `content_scale_factor` của cửa sổ được đặt để canvas có cạnh ngắn bằng cạnh ngắn canvas cơ sở (`display/window/size/viewport_height` = 720; cửa sổ dọc → hệ số 16/9, canvas 720×1558 cho 390×844), nên chữ hiện như trên canvas ngang của cùng điện thoại và sắc nét (font oversampling theo content scale — phóng `CanvasLayer` làm chữ nhỏ bị nhòe, đã thử và bỏ); khi màn ẩn, hệ số cũ được trả lại. Màn nhắc luôn dùng Theme **mật độ lớn nhất** (`g1_3`, giữ cỡ chữ/tương phản người dùng chọn): một thông điệp ngắn cho mọi lớp, chữ to nhất, không bao giờ nhỏ hơn cỡ của khối học sinh. Icon điện thoại import SVG ở 6× để nét. Không thêm tham số mới.
- Q: Logic đặt ở đâu? → A: [Confirmed] Service client (AGENTS R1): `OrientationPolicy` (thuần, tĩnh: khung nhìn dọc?, hệ số content scale của màn nhắc) và autoload `OrientationGuard` (theo dõi cỡ cửa sổ, dừng/chạy tiếp, thử khóa hướng trên Web, tạo màn nhắc). Scene `RotatePrompt.tscn` chỉ hiển thị.
- Q: Ma trận khối và bộ chụp ảnh? → A: [Confirmed] Bỏ 720×1280 khỏi ma trận 4 khối và bộ chụp; thêm điện thoại ngang **844×390** (logic 1558×720). Giữ **một** kiểm cửa sổ dọc: màn "Xoay ngang" ở 4 khối (logic 1280×2276 và 1280×2770 — cửa sổ 720×1280 và 390×844). Bộ chụp có cỡ `844x390` và `390x844` (màn `rotate_prompt`).
- Q: Ảnh bằng chứng? → A: [Confirmed] Màn nhắc trong Chromium (bản Web export, preset "Web") ở 390×844 `vi` và `en` (shell trước khi chạy và game sau khi chạy rồi xoay dọc); game ở 844×390 và 1280×720 cho 4 khối (Compatibility, `tools/capture_screens.gd`); màn nhắc native 390×844 4 khối; kết quả test cấu hình hướng native.

## Bảng kiểm UX

Ảnh `screenshots/<màn>-<tag>-<cỡ>-<ngôn ngữ>-<khối>.png` (renderer Compatibility, fixture + fake) và `screenshots/web-*.png` (Chromium headless, bản Web export). Kết quả xem ảnh ghi ở [quickstart §5](quickstart.md).

| Tình huống | Trước | Sau |
|---|---|---|
| Android/iOS cầm ngang rồi lật 180° | Không xoay theo (một chiều ngang cố định) | Xoay theo cảm biến giữa hai chiều ngang; không bao giờ dọc |
| Web, điện thoại cầm dọc, màn chờ | Màn chờ hẹp, nút "Chơi" | Lời nhắc "Xoay ngang điện thoại để chơi nhé!" với Mây + icon xoay; xoay ngang thì thấy nút "Chơi" |
| Web, đang chơi rồi xoay dọc | Game co về canvas 1280×2276, chữ ≈ 56% | Game dừng sau màn "Xoay ngang" (chữ đúng cỡ khối); xoay lại chơi tiếp đúng chỗ |
| Web trên máy tính, cửa sổ kéo hẹp hơn chiều cao | Như trên | Màn "Xoay ngang" có dòng "Trên máy tính, em kéo cửa sổ rộng ra nhé." |

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Điện thoại/tablet app luôn ngang (Priority: P1) 🎯

Học sinh lớp 3 mở app trên tablet đặt trên bàn, lật ngược tablet: game xoay theo, vẫn ngang. Cầm dọc: game vẫn ngang (hệ điều hành không xoay sang dọc).

**Why this priority**: Quy tắc đã chốt (STUDENT_GAME_UX §2); bố cục mọi màn được thiết kế cho ngang.

**Independent Test**: `tests/unit/core/test_orientation_config.gd` (đọc `project.godot`, `export_presets.cfg`).

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F113-AC-01 | `project.godot` PHẢI đặt `display/window/handheld/orientation` = sensor landscape (`DisplayServer.SCREEN_SENSOR_LANDSCAPE`) để Android và iOS chỉ hiển thị ngang và xoay theo cảm biến giữa hai chiều ngang | `STUDENT_GAME_UX` §2 · NFR-COMP-001 · NFR-COMP-003 | — |
| F113-AC-02 | Preset export Android và iOS KHÔNG ĐƯỢC ghi đè hướng màn hình: preset Android không có tùy chọn hướng; `application/additional_plist_content` của iOS KHÔNG ĐƯỢC chứa `UISupportedInterfaceOrientations` hay hướng dọc; test cấu hình PHẢI fail nếu một trong hai điều này sai | `STUDENT_GAME_UX` §2 · `GODOT_ARCHITECTURE` §11 | — |

---

### User Story 2 — Web cầm dọc: nhắc xoay ngang thân thiện (Priority: P1)

Học sinh lớp 2 mở bản Web trên điện thoại cầm dọc: màn chờ hiện Mây nói "Xoay ngang điện thoại để chơi nhé!" cạnh hình điện thoại xoay. Em xoay ngang, bấm "Chơi". Đang làm bài, em lỡ xoay dọc: game dừng, Mây nhắc xoay lại; xoay ngang, câu hỏi và đáp án em đã chọn vẫn nguyên.

**Why this priority**: Không có bố cục dọc (STUDENT_GAME_UX §2) nên cửa sổ dọc phải có một màn đọc được, không vỡ, không mất bài.

**Independent Test**: `tests/unit/core/test_orientation_policy.gd`, `tests/scene/test_orientation_guard.gd`, `tests/scene/test_grade_band_matrix.gd` (màn nhắc ở cửa sổ dọc), `src/game/web/tests/shell.test.mjs`, `src/game/web/tests/shell.browser.test.mjs`; ảnh `rotate_prompt-*`, `web-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F113-AC-03 | KHI khung nhìn của cửa sổ game chuyển sang dọc (cao > rộng) do xoay hoặc đổi cỡ, HỆ THỐNG PHẢI hiện màn toàn màn hình "Xoay ngang" (nền trời, Mây có bong bóng lời, icon điện thoại xoay, tiêu đề, lời phụ, dòng cho máy tính) phủ toàn bộ UI game; KHI khung nhìn ngang lại (kể cả vuông), HỆ THỐNG PHẢI ẩn màn đó | `STUDENT_GAME_UX` §2 · Owner 2026-11-26 | — |
| F113-AC-04 | TRONG KHI màn "Xoay ngang" hiện, game PHẢI tạm dừng (SceneTree dừng, scene game không xử lý và không nhận input) còn `ApiClient` và `RealtimeClient` PHẢI tiếp tục chạy; KHI màn ẩn, HỆ THỐNG PHẢI bỏ dừng mà không tải lại scene (trạng thái scene, chữ đang nhập, câu đang làm giữ nguyên); guard KHÔNG ĐƯỢC bỏ dừng một SceneTree do nơi khác dừng | Owner 2026-11-26 · `STUDENT_GAME_UX` §2 · AGENTS R5 (server giữ trạng thái) · NFR-COMP-004 | — |
| F113-AC-05 | TRONG KHI màn "Xoay ngang" hiện, `content_scale_factor` của cửa sổ PHẢI đặt canvas có cạnh ngắn bằng cạnh ngắn canvas cơ sở (`display/window/size/viewport_height`) và PHẢI trả lại giá trị cũ khi màn ẩn; màn PHẢI phủ toàn khung nhìn và dùng Theme mật độ lớn nhất (`g1_3`) nên chữ không nhỏ hơn `density.<band>.font.small` của khối học sinh; test ma trận PHẢI kiểm màn này ở 4 khối × cửa sổ 720×1280 và 390×844 (chữ ≥ cỡ nhỏ nhất của khối, không tràn, phủ khung) | `DESIGN_SYSTEM` §5.2, §9 · NFR-A11Y-001 | TC-UI-003 |
| F113-AC-06 | TRONG KHI màn chờ Web ở trạng thái `checking`, `loading`, `ready` hoặc `starting` và cửa sổ dọc, shell PHẢI hiện lời nhắc xoay ngang bằng HTML (Mây SVG nội tuyến, icon xoay, tiếng Việt hoặc tiếng Anh theo ngôn ngữ trình duyệt, chỉ biến `shell_tokens.css`); trang thông báo `unsupported`/`error` KHÔNG ĐƯỢC bị lời nhắc che; mọi hành vi F133-AC-16 và F096-AC-02 PHẢI giữ (test shell hiện có xanh) | Owner 2026-11-26 · F133-AC-16 · F096-AC-02 · AGENTS R32 | TC-UI-001 |
| F113-AC-07 | KHI học sinh bấm "Chơi" trên thiết bị cảm ứng và trình duyệt có API, shell PHẢI thử vào toàn màn hình rồi `screen.orientation.lock('landscape')`; KHI khung nhìn game chuyển sang dọc trên Web, client PHẢI thử `screen.orientation.lock('landscape')`; mọi lỗi hoặc thiếu API PHẢI bị bỏ qua im lặng (không lỗi trang, engine vẫn khởi động) | Owner 2026-11-26 · NFR-COMP-004 | — |
| F113-AC-08 | Logic phát hiện hướng, dừng/chạy tiếp và khóa hướng PHẢI nằm trong `OrientationPolicy`/`OrientationGuard` (service client); scene màn nhắc chỉ hiển thị; mọi chuỗi mới PHẢI qua `.po` vi/en (`extract_pot --check`, `pseudo_locale --check`, `test_font_coverage.gd`) và mọi màu/cỡ từ token (`lint_theme_overrides`, `make design-tokens-check`) | AGENTS R1, R32 · NFR-I18N-001 | TC-UI-001 |

---

### User Story 3 — Dọn phần dọc, giữ ngang xanh (Priority: P2)

**Independent Test**: `tests/scene/test_grade_band_matrix.gd`, GdUnit4 toàn bộ, ảnh `*-after-{1280x720,844x390}-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F113-AC-09 | Ma trận 4 khối PHẢI kiểm mọi màn học sinh ở cửa sổ ngang 1280×720, 1024×600 và 844×390 (logic 1280×720, 1280×750, 1558×720), gồm kiểm "dải trống" F099-AC-07; cửa sổ 720×1280 KHÔNG ĐƯỢC còn trong ma trận màn game và bộ chụp ảnh (chỉ còn kiểm màn "Xoay ngang" của F113-AC-05) | `STUDENT_GAME_UX` §2 · `DESIGN_SYSTEM` §9 · F098-AC-09 | TC-UI-003 |
| F113-AC-10 | Test GdUnit4 hiện có (F096–F099, ma trận khối ngang, F133 cấu hình preset) PHẢI xanh; bố cục dọc F097–F099 đã có KHÔNG ĐƯỢC bị xóa | AGENTS R34 · F099-AC-07 | — |
| F113-AC-11 | HỆ THỐNG PHẢI có ảnh: màn nhắc trong Chromium 390×844 `vi` và `en` (shell và trong game), game 844×390 và 1280×720 ở 4 khối, màn nhắc native 390×844 ở 4 khối (renderer Compatibility, `tools/capture_screens.gd`); lỗi phát hiện khi xem ảnh PHẢI được sửa hoặc ghi ở Câu hỏi mở | `DESIGN_SYSTEM` §8, §9 · NFR-COMP-004 | TC-UI-007 |

### Edge Cases

- NẾU khung nhìn vuông (cao = rộng), THÌ coi là ngang, không hiện màn nhắc (F113-AC-03).
- NẾU học sinh xoay dọc giữa một request (đăng nhập, nộp bài), THÌ request vẫn hoàn tất phía sau màn nhắc; kết quả hiện khi xoay lại (F113-AC-04).
- NẾU một nơi khác đã dừng SceneTree trước khi màn nhắc hiện, THÌ khi xoay lại guard để nguyên trạng thái dừng đó (F113-AC-04).
- NẾU cửa sổ dọc khi trang Web báo "không hỗ trợ WebGL 2" hay lỗi khởi động, THÌ hiện thông báo đó, không hiện lời nhắc (F113-AC-06).
- NẾU trình duyệt từ chối toàn màn hình/khóa hướng (iOS Safari, máy tính), THÌ không có gì xảy ra ngoài màn nhắc (F113-AC-07).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-009 | Chạy đúng trên Web/Android/iOS: khóa ngang native, màn nhắc xoay trên Web | `STUDENT_GAME_UX` §2 · `GODOT_ARCHITECTURE` §10.2, §11 |
| FR-GAME-010 | Màn nhắc theo khối (chữ, vùng chạm theo mật độ), Mây hướng dẫn | `STUDENT_GAME_UX` §2 · `DESIGN_SYSTEM` §5.2 |

### Key Entities

- **`OrientationPolicy`** (`scripts/core/orientation_policy.gd`, tĩnh): `is_portrait(window)`, `prompt_scale(view, base_short)`, `prompt_content_scale(window, base)`, `base_size()`.
- **`OrientationGuard`** (autoload #14, `scripts/core/orientation_guard.gd`): `apply(window)`, `prompt_visible`, tín hiệu `prompt_shown()`/`prompt_hidden()`; dừng/chạy tiếp; thử khóa hướng trên Web.
- **`RotatePrompt`** (`scenes/ui/rotate_prompt/RotatePrompt.tscn`): view.
- **Icon** `rotate_phone` (`design/icons/game/`); khóa dịch `ui.rotate.{title,hint,desktop}`.

### Tham số

| Tham số | Dùng để |
|---|---|
| `display/window/handheld/orientation` = 4 (sensor landscape) (`GODOT_ARCHITECTURE` §10.2) | F113-AC-01 |
| Canvas cơ sở 1280×720, `stretch canvas_items`/`expand` (`STUDENT_GAME_UX` §2, `GODOT_ARCHITECTURE` §10.2) | F113-AC-05, F113-AC-09 |
| `density.<band>.font.small`, `density.<band>.button.min_height` (`DESIGN_SYSTEM` §5.2) | F113-AC-05 |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Logic ở service, scene chỉ hiển thị | AGENTS R1 |
| Không hardcode màu/cỡ; file sinh không sửa tay | AGENTS R32 |
| Một nguồn cho quy tắc hướng màn hình (`STUDENT_GAME_UX` §2), tài liệu khác chỉ link | AGENTS R33 |
| Không đổi contract; không làm bố cục dọc mới | AGENTS R34 |
| Shell không dùng storage trình duyệt | ADR-0086 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Android/iOS: 0 hướng dọc trong cấu hình (test cấu hình xanh).
- **SC-002**: Web cửa sổ dọc: 100% thời gian hiện màn nhắc (shell hoặc game), 0 lần mất trạng thái khi xoay lại (test guard).
- **SC-003**: Ma trận ngang 4 khối × 3 cửa sổ + màn nhắc 4 khối × 2 cửa sổ dọc xanh; GdUnit4 toàn bộ xanh; mọi lint xanh; export Web thành công.

## Ngoài phạm vi

- Canvas dọc, bố cục dọc mới, kiểm "dải trống" ở cửa sổ dọc (F099-1, F099-2 — superseded).
- Chặn trình duyệt điện thoại hay trang "Hãy tải ứng dụng EduWorld" — owner chốt 2026-11-26: không chặn (xem F113-1).
- Đổi contract API, luật nghiệp vụ, preset chất lượng.

## Assumptions

- Godot 4.7 sinh `UISupportedInterfaceOrientations` (iOS) và `screenOrientation` (Android) từ `display/window/handheld/orientation`; không kiểm trên thiết bị thật trong lượt này (CI export Android/iOS của F133 dùng cùng cấu hình).
- Ảnh render bằng GPU (Intel Iris Xe, Compatibility); ảnh Web chụp bằng Chromium headless của Playwright (SwiftShader).

## Câu hỏi mở

- F113-1 [Confirmed — owner 2026-11-26: "cho chơi trên trình duyệt điện thoại (bắt buộc cầm ngang)"; `STUDENT_GAME_UX` §2 đã sửa, shell không chặn — giữ nguyên hành vi hiện có]: `STUDENT_GAME_UX` §2 ghi "Trình duyệt trên điện thoại — Không hỗ trợ trong Release 1.0; trang báo 'Hãy tải ứng dụng EduWorld'", nhưng shell F133 chỉ có banner cho trình duyệt không chính thức, không chặn điện thoại. Lượt này không đổi chính sách đó (màn "Xoay ngang" làm cửa sổ dọc an toàn dù có chặn hay không); quyết định chặn/không chặn thuộc F133/F134.

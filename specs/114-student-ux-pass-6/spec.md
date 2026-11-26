# Feature Specification: F114 — Student UX pass 6: phong cách viền hoạt hình phân tầng và màn Cài đặt mới

**Feature Branch**: `114-student-ux-pass-6` · **Created**: 2026-11-26 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`) · **Mốc**: M3 · **Phụ thuộc**: F096, F097, F098, F099, F113
**Input**: Owner gửi prompt thiết kế màn Cài đặt kiểu hoạt hình (viền đen 3px, bóng cứng phía dưới, tiêu đề in hoa, tab dọc, thanh trượt có %, công tắc, hộp xác nhận). Orchestrator review và đề xuất: giữ xanh thương hiệu làm màu chính (tím trùng màu môn Tiếng Anh), màu mực có sẵn `palette.ink.900` cho viền/bóng/chữ, đỏ chỉ cho hành động rời đi/phá hủy, lớp phủ 50% + 35% cho hộp lồng, áp phong cách **phân tầng cho toàn game qua token**, nét mảnh hơn cho lớp 6–9. Owner xem preview (canvas "EduWorld Settings Preview", bản A xanh) và duyệt 2026-11-26: "tôi đồng ý với hướng đề xuất của bạn và tuyệt đối phải hợp với game giáo dục". Chỉ đổi hiển thị phía client và cài đặt cục bộ đã có; không đổi API, không thêm nhóm cài đặt cần backend (AGENTS R1, R32, R34). FR nguồn: FR-GAME-010; NFR-A11Y-001.

## Clarifications

### Session 2026-11-26

Owner có lệnh thường trực "chốt hết": điểm thiếu được chốt bằng mặc định an toàn, thân thiện với trẻ nhất và ghi [Confirmed].

- Q: Màu chính? → A: [Confirmed — owner duyệt bản A] Xanh thương hiệu `color.accent.primary` (`palette.blue.600`). Không thêm màu tím chủ đạo.
- Q: Màu viền và bóng? → A: [Confirmed] Một màu mực duy nhất `palette.ink.900` (#1E2A44, cũng là màu chữ chính), qua token mới `color.outline`. Bóng là **cạnh cứng phía dưới** cùng màu mực, không bóng mờ.
- Q: Áp ở đâu? → A: [Confirmed — owner duyệt] Ba tầng:
  - **Tầng 1 — bấm được:** mọi biến thể nút (chính, phụ, nguy hiểm, `IconButton`, `HintButton`), `ChoiceChip`, công tắc, núm và rãnh thanh trượt, tab; cùng **khung cửa sổ** (panel cửa sổ, hộp thoại, thanh tiêu đề). Viền mực + cạnh dưới; khi nhấn, cạnh dưới thu lại và nội dung lún xuống đúng phần chênh (cảm giác nhấn thật). Khung cửa sổ có viền + cạnh dưới nhưng không có trạng thái nhấn.
  - **Tầng 2 — thông tin:** chip HUD (`CurrencyChip`), toast, `SpeechBubble`, `InfoBannerPanel`, `InlineErrorPanel`, thanh tiến độ: chỉ viền mực mảnh, không cạnh dưới.
  - **Tầng 3 — giữ nguyên:** chữ đề bài và đoạn đọc (`StemLabel`, `QuestionCard`), thẻ đáp án `AnswerTile*` (đã có viền màu nhấn + cạnh kẹo của F097/F099 — bộ màu và trạng thái không đổi), `TextButton` (dạng liên kết), thế giới/nhân vật 3D (viền toon theo ADR-0103, việc riêng), Admin Portal (R32).
- Q: Độ dày theo khối? → A: [Confirmed] Token density mới: viền tầng 1 `g1_3` 3, `g4_5` 3, `g6_7` 2, `g8_9` 2 px; viền tầng 2 = 2 px mọi khối; cạnh dưới `g1_3` 5, `g4_5` 4, `g6_7` 3, `g8_9` 3 px; cạnh khi nhấn 1 px mọi khối. Lý do: lớp 6–9 thấy nét quá dày là "trẻ con", nét mảnh hơn vẫn cùng phong cách.
- Q: Tương phản cao? → A: [Confirmed] Biến thể tương phản cao dùng viền tầng 1 = 3 px và viền tầng 2 = 3 px ở mọi khối; cạnh dưới giữ theo khối.
- Q: Lựa chọn đang chọn (chip) màu gì? → A: [Confirmed] `ChoiceChip` đang chọn = nền vàng nổi `component.choice_chip.selected_bg` (#FFC93C, primitive mới `palette.yellow.400`) + chữ mực + icon ✓; chưa chọn = nền trắng chữ mực. Quy ước: **xanh = hành động/vị trí** (nút chính, tab đang mở), **vàng = giá trị đang chọn**. Áp cho mọi hàng chip (Cài đặt, Cửa hàng, Bộ sưu tập, Thử thách, bộ lọc). Thay quy tắc "đang chọn = nền nhấn + ✓" của F096 (`DESIGN_SYSTEM` §5.1, `STUDENT_GAME_UX_SOCIAL` §10).
- Q: Nhãn mục trong panel? → A: [Confirmed] Biến thể `SectionTag`: viên thuốc nền vàng nhạt `component.section_tag.bg` (#FFF1B8, primitive mới `palette.yellow.100b` hoặc tên tương đương do plan chốt), chữ nâu đậm `component.section_tag.text` (#6B4E00, primitive mới `palette.yellow.900`), viền mực 2 px, chữ in hoa cỡ `density.<band>.font.small`, đậm.
- Q: Đỏ dùng khi nào? → A: [Confirmed] Biến thể nút nguy hiểm `DangerButton` (`component.button.danger.*`, nền `palette.red.700`, chữ trắng) chỉ cho **rời đi/kết thúc**: nút đóng X của cửa sổ, tab/nút "Đăng xuất", nút xác nhận "Đăng xuất". Không dùng đỏ cho phản hồi học tập (giữ cam ấm, F097) hay cho "Hủy".
- Q: Lớp phủ sau cửa sổ? → A: [Confirmed] `color.overlay.scrim` = mực 50% sau mọi cửa sổ modal; `color.overlay.scrim_nested` = mực 35% thêm sau hộp thoại lồng (tổng ≈ 70%). Không làm mờ (blur). Cửa sổ hiện bằng phóng nhẹ 0,96 → 1 trong ≤ 0,2 s (`UiTokens` thời lượng), tắt hẳn khi bật Giảm chuyển động.
- Q: Màn Cài đặt có những tab nào? → A: [Confirmed] Theo nhóm của `STUDENT_GAME_UX_SOCIAL` §10, **chỉ các thiết lập client đã có trong `Settings`/`QualitySettings`**: **Ngôn ngữ** (Tiếng Việt / English, tiêu đề song ngữ "Ngôn ngữ / Language"), **Đồ họa** (preset Thấp / Trung bình / Cao; bóng đổ, khử răng cưa, gợi ý như hiện có), **Âm thanh** (nhạc, hiệu ứng, giọng nói), **Trợ năng** (cỡ chữ 3 mức, tương phản cao, giảm chuyển động), **Thông tin** (xem lại hướng dẫn của Mây, Giấy phép, phiên bản). Nút **Đăng xuất** đỏ ở đáy cột tab. Mở lần đầu ở tab Ngôn ngữ (tiêu đề song ngữ giúp em lỡ đổi ngôn ngữ vẫn tìm lại được); trong một phiên mở lại ở tab dùng gần nhất.
- Q: `g1_3` thấy gì? → A: [Confirmed] Như `STUDENT_GAME_UX_SOCIAL` §10: đủ 5 tab + Đăng xuất (các tab đều thuộc nhóm `g1_3` được thấy); tab Đồ họa chỉ có 3 nút preset có icon (ẩn bóng đổ, khử răng cưa); tab Trợ năng có cả 3 thiết lập.
- Q: Thanh trượt âm lượng? → A: [Confirmed] Mỗi hàng: icon + tên (Nhạc nền / Hiệu ứng / Giọng nói), nút "−", thanh trượt, nút "+", nhãn "70%". Bước 10%; kéo thanh vẫn được. Nút −/+ giúp em nhỏ không phải kéo chính xác. Áp ngay qua `Settings.set_volume`, lưu cục bộ như hiện có. Nhãn nút −/+ cho trình đọc: "Giảm/Tăng âm lượng {tên}".
- Q: Công tắc bật/tắt? → A: [Confirmed] Component `ToggleSwitch` (trên `CheckButton`): rãnh viền mực, núm trắng viền mực; bật = rãnh màu nhấn, núm bên phải; tắt = rãnh xám `palette.ink.200`, núm bên trái; luôn có chữ "Bật"/"Tắt" cạnh công tắc (không chỉ màu/vị trí).
- Q: Hộp xác nhận Đăng xuất? → A: [Confirmed — owner duyệt preview] Thanh tiêu đề vàng nổi chữ mực "ĐĂNG XUẤT?", câu "Bạn muốn đăng xuất khỏi EduWorld?" + "Tiến độ của bạn đã được lưu rồi nhé.", nút "Hủy" (nút phụ trung tính, icon ✕) và "Đăng xuất" (`DangerButton`, icon cửa ra). Focus mặc định ở "Hủy". Luồng đăng xuất hiện có (`LogoutConfirm`, kết thúc game session) không đổi.
- Q: Bố cục cửa sổ? → A: [Confirmed] Cửa sổ giữa màn, thanh tiêu đề màu nhấn chữ trắng in hoa có cạnh cứng mực, nút X đỏ ở phải; cột tab bên trái nền nhạt của màu nhấn (`component.settings.tab_column_bg`), viền phải mực; nội dung bên phải cuộn dọc khi dài. Tab: icon + chữ; tab đang mở = nền nhấn chữ trắng + viền + cạnh dưới; tab khác = trong suốt chữ mực. Cửa sổ chừa ít nhất khoảng cách khối mỗi phía, không tràn ở 1280×720 và 844×390 (màn ngang duy nhất, F113).
- Q: Phím/tay cầm? → A: [Confirmed] Lên/Xuống giữa các tab, Phải vào nội dung, Trái về cột tab; `Esc` / nút Back đóng hộp xác nhận trước, rồi trang Giấy phép, rồi cửa sổ.
- Q: Ảnh trước/sau? → A: [Confirmed] Renderer Compatibility (= Web), `tools/capture_screens.gd`, 4 khối × 1280×720 và 844×390 `vi`, `en` cho `g4_5` 1280×720: mỗi tab Cài đặt, hộp xác nhận Đăng xuất, trang Giấy phép; và màn mẫu cho tầng 1/2: đăng nhập S01, HUD S03, Cửa hàng, Nhật ký quest, màn làm bài (câu chọn một), màn kết quả, một hộp thoại.

## Bảng kiểm UX

Ảnh `screenshots/<màn>-{before,after}-<cỡ>-<ngôn ngữ>-<khối>.png`, fixture + fake, không backend. Kết quả xem ảnh theo khối ghi ở quickstart.

| Màn | Trước (F113) | Sau |
|---|---|---|
| S15 Cài đặt | Một cột cuộn dài, chỉ Ngôn ngữ, Đồ họa, Đăng xuất, Thông tin; không có âm lượng, cỡ chữ, tương phản cao, giảm chuyển động dù `Settings` đã hỗ trợ | Cửa sổ có thanh tiêu đề, 5 tab icon + chữ, Đăng xuất đỏ ở đáy; âm lượng có −/+ và %, công tắc Bật/Tắt, chip vàng ✓ |
| Hộp xác nhận Đăng xuất | Hộp đơn giản | Tiêu đề vàng, lớp phủ lồng, Hủy trung tính (focus), Đăng xuất đỏ, icon + chữ |
| Nút toàn game | Nút phẳng, viền màu nhấn | Viền mực + cạnh cứng dưới, lún xuống khi nhấn; lớp 6–9 nét mảnh hơn |
| Chip lọc / lựa chọn | Đang chọn = nền xanh + ✓ | Đang chọn = nền vàng + chữ mực + ✓ |
| HUD, toast, bong bóng | Viền màu nhấn | Viền mực mảnh, không cạnh dưới |
| Đề bài, thẻ đáp án | — | Không đổi |

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Nhìn là biết cái gì bấm được (Priority: P1) 🎯

Học sinh lớp 1 thấy mọi nút có viền đậm và "đế" phía dưới như món đồ chơi; bấm vào nút lún xuống. Chip "Trung bình" đang chọn có nền vàng và dấu ✓. Học sinh lớp 8 thấy cùng phong cách nhưng nét mảnh hơn.

**Why this priority**: Phân biệt rõ phần bấm được / phần đọc là nền của dễ dùng cho lớp nhỏ; phải xong trước khi dựng màn Cài đặt trên các biến thể mới.

**Independent Test**: `design/` checks (`make design-tokens-check`); `tests/unit/core/test_outline_style.gd` (đọc StyleBox của Theme theo khối); ảnh màn mẫu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F114-AC-01 | Token `color.outline`, `color.overlay.scrim`, `color.overlay.scrim_nested`, `density.<band>.outline.{interactive,info}`, `density.<band>.depth.{rest,pressed}`, `component.choice_chip.selected_{bg,text}`, `component.section_tag.*`, `component.button.danger.*`, `component.toggle.*`, `component.settings.*` PHẢI được khai báo trong `design/game-tokens/` với giá trị của Clarifications; Theme, `UiTokens` và CSS shell PHẢI được sinh bằng `make design-tokens` (không sửa tay), và mọi cặp chữ/nền mới PHẢI có `contrastWith` đạt WCAG AA trong `make design-tokens-check` | Owner 2026-11-26 · AGENTS R32 · NFR-A11Y-001 | TC-UI-001 |
| F114-AC-02 | Mỗi biến thể tầng 1 PHẢI có viền `color.outline` dày `density.<band>.outline.interactive` và cạnh dưới cùng màu dày `density.<band>.depth.rest` ở trạng thái thường/rê/focus; KHI nhấn, cạnh dưới PHẢI bằng `density.<band>.depth.pressed` và nội dung PHẢI dịch xuống đúng phần chênh để tổng chiều cao control không đổi | Owner 2026-11-26 · `DESIGN_SYSTEM` §5.1 | TC-UI-001 |
| F114-AC-03 | Khung cửa sổ và hộp thoại PHẢI có viền và cạnh dưới như tầng 1 nhưng không có trạng thái nhấn; mỗi biến thể tầng 2 PHẢI có viền `color.outline` dày `density.<band>.outline.info` và KHÔNG ĐƯỢC có cạnh dưới | Owner 2026-11-26 · `DESIGN_SYSTEM` §5.1 | TC-UI-001 |
| F114-AC-04 | StyleBox của các biến thể tầng 3 (`StemLabel`, `QuestionCard`, `AnswerTile1…6`, `AnswerTileCorrect`, `AnswerTileTry`, `AnswerTileRest`, `TextButton`) PHẢI giữ nguyên giá trị viền, màu và cạnh như `tokens@0.16.0` (test so giá trị) | Owner 2026-11-26 · F099-AC-01…03 | TC-UI-001 |
| F114-AC-05 | `ChoiceChip` đang chọn PHẢI có nền `component.choice_chip.selected_bg`, chữ `component.choice_chip.selected_text` và icon ✓; chưa chọn PHẢI nền trắng chữ mực không icon ✓ — trạng thái không chỉ bằng màu | Owner 2026-11-26 · NFR-A11Y-001 | TC-UI-001 |
| F114-AC-06 | TRONG KHI bật tương phản cao, viền tầng 1 và tầng 2 PHẢI dày 3 px ở mọi khối | Owner 2026-11-26 · NFR-A11Y-001 | TC-UI-001 |
| F114-AC-07 | KHI một cửa sổ modal mở, HỆ THỐNG PHẢI phủ `color.overlay.scrim` phía sau và thêm `color.overlay.scrim_nested` sau hộp thoại lồng; hiệu ứng hiện cửa sổ PHẢI ≤ 0,2 s và PHẢI không chạy TRONG KHI bật Giảm chuyển động | Owner 2026-11-26 · `STUDENT_GAME_UX_LEARNING` §10.3 | TC-UI-003 |

---

### User Story 2 — Màn Cài đặt dễ tìm, dễ chỉnh (Priority: P1)

Học sinh lớp 2 bấm bánh răng: cửa sổ "CÀI ĐẶT" mở ở tab "Ngôn ngữ / Language". Em chạm tab "Âm thanh", bấm "−" hai lần ở Nhạc nền thấy "80%" rồi "60%". Em chạm "Đăng xuất" (đỏ, dưới cùng), hộp vàng "ĐĂNG XUẤT?" hiện, nút "Hủy" đang sáng; em chọn Hủy và quay lại.

**Independent Test**: `tests/scene/ui/test_settings_panel.gd`; `tests/scene/test_grade_band_matrix.gd` (thêm các tab Cài đặt); ảnh `settings_*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F114-AC-08 | KHI mở Cài đặt, HỆ THỐNG PHẢI hiện cửa sổ có thanh tiêu đề ("CÀI ĐẶT"/"SETTINGS" in hoa) với nút đóng `DangerButton` icon ✕, cột tab trái gồm Ngôn ngữ, Đồ họa, Âm thanh, Trợ năng, Thông tin (mỗi tab icon + chữ) và nút Đăng xuất ở đáy cột, nội dung phải cuộn dọc khi dài; lần mở đầu tiên trong phiên PHẢI ở tab Ngôn ngữ, lần sau ở tab dùng gần nhất | Owner 2026-11-26 · `STUDENT_GAME_UX_SOCIAL` §10 | TC-UI-003 |
| F114-AC-09 | Tab Âm thanh PHẢI có ba hàng Nhạc nền / Hiệu ứng / Giọng nói, mỗi hàng nút −, thanh trượt, nút + (bước 10%) và nhãn phần trăm; KHI đổi giá trị, HỆ THỐNG PHẢI áp ngay qua `Settings.set_volume` và lưu cục bộ | `GODOT_ARCHITECTURE` §10 · `STUDENT_GAME_UX_SOCIAL` §10 | — |
| F114-AC-10 | Tab Đồ họa PHẢI dùng logic `QualitySettings` hiện có; Ở khối `g1_3` chỉ hiện 3 chip preset có icon, khối khác hiện thêm bóng đổ, khử răng cưa và gợi ý như hiện có | `STUDENT_GAME_UX_SOCIAL` §10 · `PERFORMANCE_BUDGET` | — |
| F114-AC-11 | Tab Trợ năng PHẢI có cỡ chữ 3 mức (chip), Tương phản cao và Giảm chuyển động (`ToggleSwitch` kèm chữ Bật/Tắt); KHI đổi, HỆ THỐNG PHẢI áp ngay qua `Settings.set_font_scale` / `set_high_contrast` / `set_reduce_motion` và lưu cục bộ | `GODOT_ARCHITECTURE` §10 · NFR-A11Y-001 | — |
| F114-AC-12 | Tab Ngôn ngữ PHẢI có tiêu đề song ngữ và 2 chip Tiếng Việt / English; tab Thông tin PHẢI có "Xem lại hướng dẫn của Mây", "Giấy phép" (trang trong vùng nội dung có nút Quay lại, văn bản như F096) và phiên bản game | `STUDENT_GAME_UX_SOCIAL` §10 · F096 | — |
| F114-AC-13 | KHI chọn Đăng xuất, HỆ THỐNG PHẢI hiện hộp xác nhận (tiêu đề vàng, lớp phủ lồng, "Hủy" trung tính nhận focus mặc định, "Đăng xuất" `DangerButton`, mỗi nút icon + chữ) và chỉ chạy luồng đăng xuất hiện có khi chọn "Đăng xuất"; `Esc`/Back PHẢI đóng hộp xác nhận trước, rồi trang Giấy phép, rồi cửa sổ | Owner 2026-11-26 · `GAME_NETWORKING` · `STUDENT_GAME_UX_SOCIAL` §10 | TC-UI-003 |
| F114-AC-14 | Ở mỗi khối × 1280×720 và 844×390 × `vi`/`en`, mọi control của Cài đặt PHẢI cao ≥ `density.<band>.button.min_height`, chữ ≥ `density.<band>.font.small`, không bị cắt hay tràn khỏi cửa sổ; toàn bộ Cài đặt PHẢI điều khiển được bằng bàn phím/tay cầm; không chuỗi/màu hardcode (lint theme + i18n) | `DESIGN_SYSTEM` §5.2, §9 · F098-AC-09 · AGENTS R32 | TC-UI-003 |

---

### User Story 3 — Cả game cùng một phong cách (Priority: P2)

Mọi màn dùng component chung tự đổi theo token; các màn dày đặc vẫn không tràn sau khi nút cao thêm cạnh dưới.

**Independent Test**: `tests/scene/test_grade_band_matrix.gd` toàn bộ; ảnh màn mẫu trước/sau.

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F114-AC-15 | Sau khi áp token, test ma trận khối (mọi màn hiện có × 4 khối × cửa sổ tham chiếu của F113) PHẢI vẫn pass: không control cắt chữ, không tràn khung, nút ≥ chiều cao tối thiểu của khối | F098-AC-09 · F113 | TC-UI-003 |
| F114-AC-16 | Ảnh trước/sau của các màn trong Clarifications PHẢI được chụp, xem theo khối và ghi kết quả trong quickstart; GdUnit4 toàn bộ, `lint_theme_overrides`, kiểm i18n và export Web PHẢI xanh | `DESIGN_SYSTEM` §9 · AGENTS R18 | — |

### Edge Cases

- Cỡ chữ +2 ở `g1_3` trên 844×390: nội dung tab cuộn dọc; cột tab không bị cắt (tab co chữ xuống dòng, không ellipsis mất nghĩa).
- Chuỗi tiếng Anh dài hơn 35% ("Sign out", "Reduce motion", "Anti-aliasing"): không tràn cột tab/hàng.
- Âm lượng 0% / 100%: nút − / + bị vô hiệu (mờ, vẫn có viền) ở đầu mút.
- Đổi ngôn ngữ khi đang ở tab Ngôn ngữ: tiêu đề, tab, nút cập nhật ngay (AC-UX-0002), vẫn ở tab Ngôn ngữ.
- Bật tương phản cao/cỡ chữ khi Cài đặt đang mở: cửa sổ dựng lại theo Theme mới, giữ tab đang mở.
- Mất `user://settings.cfg` hoặc lỗi lưu: dùng mặc định như hiện có, không chặn mở Cài đặt.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-010 | Phong cách viền phân tầng theo khối; màn Cài đặt theo khối | `DESIGN_SYSTEM` §5.1, §5.2 · `STUDENT_GAME_UX_SOCIAL` §10 |

### Key Entities

- **Token mới** (`tokens@0.17.0`): xem F114-AC-01; primitive mới `palette.yellow.{400,900}` và nền nhãn mục; biến thể Theme `DangerButton`, `SectionTag`, `TabButton`, `ToggleSwitch`, `WindowTitleBar`, `ScrimPanel` (tên chốt ở plan).
- **Component** `ToggleSwitch`, `VolumeRow` (nút −/+, `HSlider`, nhãn %), cửa sổ `SettingsPanel` dựng lại (tab + trang), `LogoutConfirm` đổi giao diện.
- **Test** `test_outline_style.gd`, `test_settings_panel.gd`; mở rộng `test_grade_band_matrix.gd`.

### Tham số

| Tham số | Dùng để |
|---|---|
| `density.<band>.outline.interactive` 3/3/2/2, `.outline.info` 2, `.depth.rest` 5/4/3/3, `.depth.pressed` 1 px (sở hữu: `DESIGN_SYSTEM` §5.2 — cập nhật trong feature này) | F114-AC-02, F114-AC-03, F114-AC-06 |
| `color.overlay.scrim` 50%, `scrim_nested` 35% mực; hiện cửa sổ ≤ 0,2 s (sở hữu: `DESIGN_SYSTEM` §5.1) | F114-AC-07 |
| Bước âm lượng 10% (sở hữu: `STUDENT_GAME_UX_SOCIAL` §10, hằng client) | F114-AC-09 |
| `density.<band>.button.min_height`, `density.<band>.font.small` (`DESIGN_SYSTEM` §5.2) | F114-AC-14 |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không logic nghiệp vụ trong scene; cài đặt đi qua `Settings`/`QualitySettings` hiện có | AGENTS R1 |
| Không hardcode màu/cỡ/chuỗi; file sinh không sửa tay | AGENTS R32 · ADR-0047 |
| Không đổi API, không thêm nhóm cài đặt cần backend | AGENTS R34 |
| Không áp lực, không dark pattern trong hộp xác nhận (Hủy là mặc định an toàn) | AGENTS R19 · NFR-SAFE-003 |
| Hiệu năng: StyleBox phẳng, không bóng mờ/shader mới | `STUDENT_GAME_UX_LEARNING` §2.1 · ADR-0057 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% biến thể tầng 1 có viền mực + cạnh dưới và trạng thái nhấn lún; 0 thay đổi ở biến thể tầng 3 (test).
- **SC-002**: Cài đặt có đủ 5 tab + Đăng xuất; âm lượng, cỡ chữ, tương phản cao, giảm chuyển động chỉnh được (trước: không có trong UI).
- **SC-003**: Ma trận khối, GdUnit4, lint, kiểm token và export Web xanh.

## Ngoài phạm vi

- Nhóm cài đặt cần backend hoặc chưa có trong `Settings`: Hồ sơ của em, Điều khiển (gán phím, joystick, rung), Riêng tư, "Đăng xuất trên mọi thiết bị", thời gian chơi hôm nay, hiển thị tên song ngữ, giới hạn FPS/độ phân giải động/số người chơi hiển thị/hiện FPS/tự phát hiện lại, phụ đề và giọng đọc (TTS) — xem Câu hỏi mở.
- Đổi bộ màu/trạng thái thẻ đáp án; viền toon cho 3D; Admin Portal.
- Âm thanh/hạt hiệu ứng mới.

## Assumptions

- Base canvas 1280×720, chỉ màn ngang (F113); 844×390 là điện thoại ngang tham chiếu.
- Font đóng gói phủ mọi chuỗi mới; ảnh render bằng renderer Compatibility; so sánh bằng mắt.

## Câu hỏi mở

- F114-1 [Proposed]: Thêm cài đặt Phụ đề (mặc định bật) và Tự đọc đề bài (mặc định bật cho `g1_3`, `STUDENT_GAME_UX_LEARNING` §7) vào tab Âm thanh/Trợ năng — cần trường mới trong `Settings` và renderer media/câu hỏi đọc giá trị; đề xuất feature riêng cùng TTS.
- F114-2 [Proposed]: Các nhóm Hồ sơ, Điều khiển, Riêng tư, Tài khoản đầy đủ của S15 cần API (`updateGameProfile`, revoke-all) — đề xuất feature riêng sau chuỗi multiplayer.
- F114-3 [Proposed]: Áp nút đóng đỏ `DangerButton` icon ✕ cho mọi cửa sổ khác (cửa sổ `UiBuild.window` của F069, Nhật ký quest, Phối đồ, Bản đồ, Phòng thi…) — F114 chỉ đổi nút Đóng của Cài đặt theo AC-08; các cửa sổ khác giữ nút "Đóng" phụ (đã có viền mực + cạnh dưới mới). Đề xuất làm trong pass UX kế tiếp.

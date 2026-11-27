# Feature Specification: F125 — Student UX pass 7: phóng to giao diện trên điện thoại và nút đóng đỏ cho mọi cửa sổ

**Feature Branch**: `125-student-ux-pass-7` · **Created**: 2026-11-27 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`) · **Mốc**: M3 · **Phụ thuộc**: F113, F114, F119, F123, F102
**Input**: Orchestrator xem ảnh 844×390 của F114/F119 và thấy: canvas cơ sở 1280×720 (`canvas_items`, `aspect = expand`) bị thu cả khối xuống cửa sổ điện thoại ngang, nên chữ và nút về vật lý chỉ còn ≈ 54% cỡ token (chữ 18 px hiện như ≈ 10 px; nút 44 px còn ≈ 24 px) — ảnh `specs/114-student-ux-pass-6/screenshots/settings_access-after-844x390-vi-g8_9.png`. F114 để lại câu hỏi mở F114-3: chỉ cửa sổ Cài đặt có nút đóng đỏ "✕ Đóng". Owner duyệt 2026-11-27: "có đổi và có spec phóng to trên điện thoại". Chỉ đổi hiển thị phía client; không đổi API, không đổi luật nghiệp vụ (AGENTS R1, R32, R34). FR nguồn: FR-GAME-010; NFR-A11Y-001, NFR-COMP-001.

## Clarifications

### Session 2026-11-27

Owner có lệnh thường trực "chốt hết": điểm thiếu được chốt bằng mặc định an toàn, thân thiện với trẻ nhất và ghi [Confirmed].

- Q: Thế nào là "điện thoại" (chế độ `compact`)? → A: [Confirmed] Khi **cạnh ngắn của cửa sổ** tính theo đơn vị độc lập mật độ (CSS px trên Web, dp trên Android, pt trên iOS) **< 500**. Không dựa vào user agent hay tên thiết bị. Tablet (≥ 600) và máy tính giữ như hiện tại (`regular`/`wide`). Đánh giá lại mỗi khi cửa sổ đổi cỡ (xoay, kéo cửa sổ trình duyệt). Định nghĩa này thay dòng "`compact` (điện thoại, chiều cao logic < 800)" của `STUDENT_GAME_UX` §2 (không đo được vì chiều cao logic luôn 720).
- Q: Phóng to bao nhiêu? → A: [Confirmed] Trong `compact`, đặt `content_scale_factor` để **chiều cao canvas logic = 540** (token/cài đặt `component.compact.logical_height`, không hardcode), không bao giờ < 1. Ví dụ 844×390 → canvas logic ≈ 1169×540, mọi thứ to ≈ 1,33× so với hiện tại. Cửa sổ thoát `compact` → trả hệ số 1. Cùng cơ chế `content_scale_factor` mà `OrientationGuard` đã dùng (F113); khi màn "Xoay ngang" hiện, hệ số của nó được ưu tiên và trả về đúng hệ số `compact` khi ẩn.
- Q: Ngưỡng vật lý tối thiểu? → A: [Confirmed] Trong `compact` ở cửa sổ tham chiếu 844×390 và 740×360 (điện thoại ngang nhỏ): chữ nhỏ nhất của khối ≥ **12 CSS px** vật lý; vùng chạm của mọi control bấm được ≥ **40 CSS px** vật lý. Để đạt vùng chạm, mật độ `compact` ghi đè `density.<band>.button.min_height` thành tối thiểu **56 logic** cho mọi khối (`g1_3` 64 giữ nguyên) — token `density.compact.button.min_height`. Cỡ chữ theo khối giữ nguyên (đã đủ ≥ 12 px sau phóng to).
- Q: Màn nào phải xếp lại? → A: [Confirmed] **Mọi màn** trong ma trận khối (`tests/scene/test_grade_band_matrix.gd`) phải pass ở canvas `compact` (không cắt chữ, không tràn, nút ≥ ngưỡng, quy tắc ≥ 3 tin chat F119, không dải trống > 25% ở màn làm bài F099). Màn nào không vừa chiều cao 540 thì xếp lại theo quy tắc đã có (cuộn dọc nội dung, cột hai bên, thu gọn phụ) — không thu nhỏ chữ dưới token của khối. HUD trong thế giới: giữ bốn góc, nút ít dùng gom vào "Thêm" nếu chồng lấn.
- Q: Kiểm tự động? → A: [Confirmed] Ma trận khối thêm cửa sổ `compact` 844×390 và 740×360 (giả lập cạnh ngắn < 500 bằng hệ số mật độ của test), và probe **vật lý**: đổi kích thước logic sang CSS px theo `content_scale_factor` rồi kiểm 12 px / 40 px. Probe phải báo lỗi trên bố cục hiện tại (kiểm ngược: hệ số 1 ở 844×390).
- Q: Nút đóng cửa sổ (F114-3)? → A: [Confirmed — owner duyệt 2026-11-27] Mọi **cửa sổ** dựng bằng khung cửa sổ chung (`UiBuild` panel window, thanh tiêu đề tầng 1) — Nhật ký quest, Tủ đồ/Phối đồ, Bản đồ, Cửa hàng, Bộ sưu tập/Thành tựu, Thử thách, Nhóm (nút "Rời nhóm" giữ nguyên, thêm đóng nếu đang dùng nút phụ), Hộp thông báo, Cài đặt, Sự kiện, … — dùng một nút đóng duy nhất `DangerButton` icon ✕ + chữ "Đóng" ở phải thanh tiêu đề, cùng vị trí, cùng phím `Esc`/Back. **Không** áp cho: hộp xác nhận (giữ "Hủy" trung tính), màn làm bài/thi (giữ luồng "Thoát" có xác nhận hiện có), màn toàn khung không phải cửa sổ (đăng nhập, màn chờ, "Xoay ngang"). Quy ước đỏ = rời đi/kết thúc của F114 giữ nguyên.
- Q: Ảnh trước/sau? → A: [Confirmed] Renderer Compatibility, `tools/capture_screens.gd`: mọi màn chính (đăng nhập, HUD, Cài đặt các tab, Nhật ký, Cửa hàng + hộp mua, Tủ đồ, Bản đồ, Thử thách, màn làm bài câu chọn một, kết quả, Hộp thông báo, Nhóm + chat, Đấu trường nếu đã có) × 4 khối ở 844×390 và 740×360 `vi`, `en` g4_5 844×390 — "trước" bằng bản hiện tại; thêm 1280×720 `vi` cho các cửa sổ đổi nút đóng.

### Session 2026-11-27 (plan — chốt bằng mặc định an toàn nhất, lệnh "chốt hết")

- Q: `density.compact.button.min_height` = 56 có đạt vùng chạm 40 CSS px ở 740×360 không? → A: [Confirmed] **Không**: canvas logic 540 trên cửa sổ cao 360 cho 0,667 CSS px mỗi px logic → 56 chỉ còn 37,3 px. Chốt **60** (60 × 0,667 = 40,0 px; 844×390: 43,3 px), vẫn thỏa "tối thiểu 56" của AC-03; `g1_3` giữ 64.
- Q: Cỡ chữ theo khối có đủ ≥ 12 CSS px sau phóng to? → A: [Confirmed] **Không** với `g6_7`/`g8_9`: `density.<band>.font.small` = 16 → 10,7 px ở 740×360 (11,6 px ở 844×390). Thêm token `density.compact.font.small` = **18** (18 × 0,667 = 12,0 px): trong `compact` mọi cỡ chữ < 18 được nâng lên 18; cỡ chữ của khối không bao giờ bị hạ (AC-05). Hai sàn `compact` sinh thành Theme `compact` riêng của từng khối (`eduworld_theme_<band>_compact.tres`), không sửa tay.
- Q: Cửa sổ `compact` hẹp hơn 16:9 (vd. cửa sổ trình duyệt 600×480)? → A: [Confirmed] Hệ số bị chặn để canvas logic không hẹp hơn 540 × 16/9 = 960 (giữ tỉ lệ canvas gốc); khi đó chiều cao logic > 540 (ít phóng hơn), vẫn ≥ 1. Điện thoại ngang (≥ 16:9) luôn đúng 540.
- Q: Bàn phím ảo? → A: [Confirmed] Chiều cao bàn phím ảo (`DisplayServer.virtual_keyboard_get_height`) được cộng lại vào cửa sổ trước khi quyết định chế độ và hệ số.
- Q: Ngưỡng 500 đặt ở đâu? → A: [Confirmed] Cài đặt dự án `eduworld/ui/compact_short_side` = 500 (sở hữu `STUDENT_GAME_UX` §2, đăng ký ở CONFIG_DEFAULTS); `component.compact.logical_height` và hai sàn là token.
- Q: Nhóm đang có nút phụ "Thu nhỏ"? → A: [Confirmed] Bỏ nút phụ "Thu nhỏ"; nút đỏ "✕ Đóng" luôn ở cuối thanh tiêu đề (trong nhóm vẫn chỉ đóng cửa sổ, nhóm còn nguyên — tooltip "Nhóm vẫn còn…"); "Rời nhóm" giữ nguyên bên cạnh.
- Q: Danh sách "cửa sổ"? → A: [Confirmed] Mọi cửa sổ có thanh tiêu đề của khung chung: Nhật ký quest, Bản đồ, Phối đồ (studio), Tủ đồ/Cửa hàng, Thành tựu/Bộ sưu tập, Thử thách (bảng, kết quả), Hành trình, menu "Thêm", Hộp thông báo, Cài đặt, Nhóm (+ chọn bạn mời, chọn hoạt động), Đấu trường, sảnh đề thi (ExamLobby — cửa sổ trước khi vào thi; màn làm bài thi giữ "Thoát"). Client chưa có màn "Sự kiện". **Không** áp: hộp xác nhận, sheet chat (menu tin nhắn, báo cáo), lời thoại NPC (bong bóng hội thoại), hướng dẫn giải trong màn làm bài, vòng biểu cảm, thông báo giờ nghỉ toàn khung, màn chờ, đăng nhập, "Xoay ngang".
- Q: Quy tắc "≥ 3 tin" của F119 trên canvas 540 (bảng mặt cười `g1_3` mở sẵn chiếm 2 hàng)? → A: [Confirmed] Trong `compact`: bảng mặt cười đóng mặc định ở mọi khối (hàng tin nhanh + ô nhập vẫn hiện, nút 😊 mở bảng bằng một chạm), nút tin nhanh `g1_3` cao bằng vùng chạm `compact` (không 1,2×); panel chat trượt của màn Nhóm thành tấm phủ cao hết màn hình bên phải (nút "Thu gọn" trả lại thanh Sẵn sàng / Bắt đầu). Chọn đọc được ≥ 3 tin hơn là giữ bảng mở (an toàn hơn cho trẻ: thấy lời bạn nói). Ghi ở `STUDENT_GAME_UX_SOCIAL` §7.
- Q: Màn không vừa canvas 540 xếp lại thế nào? → A: [Confirmed] Kết quả bài học: phần nội dung cuộn, nút "Về nhiệm vụ" ghim dưới; Đăng nhập: thẻ nhập co theo chỗ giữa hai cột (≤ `component.form_card.width`); cột trái màn trận cuộn cả cột khi bản dịch dài; tên/nhãn không bao giờ hẹp hơn từ dài nhất (`UiBuild.keep_words`); màn làm bài: ≤ 4 đáp án chọn một/chọn nhiều xếp một hàng, bong bóng Mây bỏ câu cổ vũ chung (giữ "Gợi ý" và mọi lời chấm); menu "Thêm" 3 cột; icon thanh tiêu đề trên ô trắng `TitleIconBadge` (icon nét xanh không biến mất trên thanh xanh) — chữ không bao giờ thu nhỏ.

### Session 2026-11-28 (polish sau ảnh F109 844×390 — [Confirmed], "chốt hết")

- Q: Màn có hàng nút ghim dưới vùng cuộn trên canvas 540 — thế nào là "không bị che"? → A: [Confirmed] Vùng cuộn dừng phía trên hàng nút và phần học sinh phải đọc ngay (kết quả trận: Mây + lời động viên + dòng điểm) thấy trọn ở trạng thái nghỉ, không bị mép cuộn cắt ngay trên nút; không đủ chỗ thì **xếp lại** (ẩn phần thừa như `MatchHeader` ở màn kết quả), không thu nhỏ chữ. Probe `BandLayoutProbe.footer_cuts` + test `test_compact_polish.gd`.
- Q: Băng thông báo chiếm một hàng trên câu hỏi (canvas 540)? → A: [Confirmed] Băng "{nickname} đang kết nối lại…" thu vào biểu tượng trạng thái trên thẻ người chơi khi câu hỏi hiện trên canvas `compact` (nhãn trợ năng + tooltip giữ câu đầy đủ) — mọi đáp án thấy không cần cuộn; chi tiết ở F109 Clarifications 2026-11-28 và [STUDENT_GAME_UX_SOCIAL §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md#8-competition-arena).
- Q: Danh sách chat ảo hóa có được phép để thẻ chồng nhau tạm thời? → A: [Confirmed] Không: thẻ tin không bao giờ chồng nhau ở mọi khối × cửa sổ (đo lại khi bề rộng/nội dung đổi) — `BandLayoutProbe.overlapping_messages` chạy trong `BandLayoutProbe.check` của ma trận khối. Ảnh sau: `specs/125-student-ux-pass-7/screenshots/*-polish-*.png`.
- Q: Rà soát ảnh 844×390 / 740×360 (4 khối `vi` + `en` `g4_5`) các màn F109 + Nhóm + chat còn lỗi gì? → A: [Confirmed] Sửa trong phạm vi các màn này ([STUDENT_GAME_UX_SOCIAL §5, §7, §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md)): (1) tab Thi đấu — hàng "Thách đấu bạn"/"Tìm trận" cắt thẻ chế độ → cột bên phải thẻ trên canvas `compact`; (2) Lịch sử — "Xem thêm" ở cuối danh sách trong vùng cuộn; (3) thanh hàng chờ đè thẻ "Việc tiếp theo" và nút "Về Quảng trường" → nút xuống dưới chữ, thanh trong dải 30%–75%; (4) popup lời mời nhóm nằm dưới thanh bên/nút "Biểu cảm" (che "Vào nhóm") → đặt giữa cột trái và cột phải HUD; (5) dải "Đúng rồi! +150 điểm" bị mép cuộn cắt (`g1_3`) → cuộn vào tầm nhìn; (6) gợi ý ô nhập chat bị cắt ("Type a") trong cột chat hẹp → nút Gửi chỉ còn biểu tượng (chữ thành tooltip + tên trợ năng). Còn lại là nội dung cuộn có thanh cuộn (danh sách xếp hạng, câu cần ôn, thẻ hoạt động của Nhóm) — không phải lỗi che/chồng.

## Bảng kiểm UX

| Màn | Trước | Sau |
|---|---|---|
| Mọi màn trên điện thoại ngang | Canvas 1280×720 thu nhỏ: chữ ≈ 10–13 px, nút ≈ 24–35 px | Canvas logic cao 540: chữ ≥ 12 px, nút ≥ 40 px vật lý ở mọi khối |
| Cửa sổ (Nhật ký, Tủ đồ, Bản đồ, Cửa hàng, …) | Nút "Đóng" phụ, vị trí khác nhau | Nút đỏ "✕ Đóng" cùng vị trí phải thanh tiêu đề |
| Máy tính, tablet | — | Không đổi |

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Chơi trên điện thoại vẫn đọc rõ, bấm trúng (Priority: P1) 🎯

Học sinh lớp 7 chơi bản Web trên điện thoại cầm ngang: chữ đủ to để đọc không phải nheo mắt, nút đủ lớn để bấm trúng bằng ngón cái; khi chuyển sang laptop, giao diện như cũ.

**Independent Test**: `tests/unit/core/test_compact_policy.gd`, `tests/scene/test_compact_scale.gd`; ma trận khối mở rộng; ảnh 844×390 / 740×360.

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F125-AC-01 | KHI cạnh ngắn của cửa sổ (đơn vị độc lập mật độ) < 500, HỆ THỐNG PHẢI vào chế độ `compact` và đặt `content_scale_factor` sao cho chiều cao canvas logic = `component.compact.logical_height` (không < 1); KHI cạnh ngắn ≥ 500, hệ số PHẢI = 1; quyết định PHẢI chạy lại mỗi khi cửa sổ đổi cỡ, không tải lại scene | Owner 2026-11-27 · `STUDENT_GAME_UX` §2 | TC-UI-003 |
| F125-AC-02 | TRONG KHI màn "Xoay ngang" (F113) hiện, hệ số của nó PHẢI được ưu tiên; KHI ẩn, hệ số PHẢI trở về giá trị của chế độ hiện hành (`compact` hoặc 1) | F113 · `GODOT_ARCHITECTURE` §10.2 | — |
| F125-AC-03 | TRONG `compact`, chiều cao tối thiểu của mọi control bấm được PHẢI ≥ `density.compact.button.min_height` (≥ 56 logic) ở mọi khối, và ở cửa sổ 844×390 và 740×360 chữ nhỏ nhất PHẢI ≥ 12 CSS px, vùng chạm PHẢI ≥ 40 CSS px vật lý | Owner 2026-11-27 · NFR-A11Y-001 | TC-UI-003 |
| F125-AC-04 | Ma trận khối PHẢI kiểm mọi màn hiện có × 4 khối × cửa sổ `compact` 844×390 và 740×360 (cùng 1280×720, 1024×600 như cũ): không cắt chữ, không tràn, nút ≥ ngưỡng, ≥ 3 tin chat (F119), không dải trống > 25% ở màn làm bài (F099); probe vật lý PHẢI báo lỗi trên bố cục hệ số 1 ở 844×390 (kiểm ngược) | F098-AC-09 · F099-AC-07 · F119 | TC-UI-003 |
| F125-AC-05 | Cỡ chữ của khối KHÔNG ĐƯỢC giảm dưới token `density.<band>.font.*` để vừa màn `compact`; màn không vừa PHẢI xếp lại (cuộn, cột, gom vào "Thêm") | Owner 2026-11-27 · `DESIGN_SYSTEM` §5.2 | — |

---

### User Story 2 — Mọi cửa sổ đóng cùng một cách (Priority: P1)

Học sinh lớp 2 mở Tủ đồ, Bản đồ hay Cửa hàng đều thấy cùng một nút đỏ "✕ Đóng" ở góc phải trên.

**Independent Test**: `tests/scene/ui/test_window_close_button.gd`; ảnh các cửa sổ.

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F125-AC-06 | Mọi cửa sổ dựng bằng khung cửa sổ chung PHẢI có đúng một nút đóng `DangerButton` (icon ✕ + chữ "Đóng") ở phải thanh tiêu đề, cùng vị trí, cùng chiều cao vùng chạm của khối/`compact`; `Esc`/Back PHẢI đóng cửa sổ như nút đó | Owner 2026-11-27 (F114-3) · F114-AC-08 | TC-UI-003 |
| F125-AC-07 | Hộp xác nhận, màn làm bài/thi và màn toàn khung không phải cửa sổ PHẢI giữ "Hủy" trung tính / luồng "Thoát" hiện có và KHÔNG ĐƯỢC dùng nút đóng đỏ | Owner 2026-11-27 · F114 | — |

---

### User Story 3 — Bằng chứng (Priority: P2)

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F125-AC-08 | Ảnh trước/sau theo Clarifications PHẢI được chụp, xem theo khối và ghi kết quả trong quickstart; GdUnit4 toàn bộ, lint Godot, kiểm token và export Web PHẢI xanh | `DESIGN_SYSTEM` §9 · AGENTS R18 | — |

### Edge Cases

- Xoay điện thoại giữa hai chiều ngang: hệ số không đổi, không nháy.
- Kéo cửa sổ trình duyệt máy tính xuống thấp hơn 500 px chiều cao: vào `compact` (chấp nhận được — giao diện to hơn cho cửa sổ nhỏ).
- Cỡ chữ +2 trong `compact`: nội dung cuộn dọc, không cắt.
- Ngôn ngữ `en` và pseudo +35% trong `compact`: không tràn.
- Bàn phím ảo mở khi gõ chat/tên: canvas không đổi hệ số chỉ vì bàn phím (chỉ cạnh ngắn của cửa sổ thực).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-010 | Giao diện theo khối đọc rõ, bấm trúng trên điện thoại; nút đóng thống nhất | `STUDENT_GAME_UX` §2 · `DESIGN_SYSTEM` §5.1, §5.2 |

### Key Entities

- `CompactPolicy` (logic thuần, `scripts/core/`) + nơi áp dụng (áp dụng trong `OrientationGuard`, chốt ở plan); token `component.compact.logical_height` = 540, `density.compact.button.min_height` = 60, `density.compact.font.small` = 18 (Clarifications plan); nút đóng chung trong `UiBuild` (`title_bar`, `close_button`).
- Test `test_compact_policy.gd`, `test_compact_scale.gd`, `test_window_close_button.gd`; probe vật lý trong `band_layout_probe.gd`.

### Tham số

| Tham số | Dùng để |
|---|---|
| Ngưỡng `compact`: cạnh ngắn < 500 đơn vị độc lập mật độ (sở hữu: `STUDENT_GAME_UX` §2 — cập nhật trong feature này) | F125-AC-01 |
| `component.compact.logical_height` = 540 (sở hữu: `DESIGN_SYSTEM` §5.2) | F125-AC-01 |
| `density.compact.button.min_height` = 60, `density.compact.font.small` = 18 (sở hữu: `DESIGN_SYSTEM` §5.2; Clarifications plan) | F125-AC-03, F125-AC-05 |
| Sàn vật lý chữ 12 CSS px, vùng chạm 40 CSS px (sở hữu: `DESIGN_SYSTEM` §9, chỉ trong test) | F125-AC-03, F125-AC-04 |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không logic nghiệp vụ trong scene; chính sách hiển thị ở service thuần | AGENTS R1 |
| Không hardcode màu/cỡ; file sinh không sửa tay | AGENTS R32 · ADR-0047 |
| Không đổi API/luật; không mở rộng phạm vi | AGENTS R34 |
| Hiệu năng: không thêm render pass; đổi hệ số chỉ khi cửa sổ đổi cỡ | ADR-0057 · NFR-COMP-001 |

## Success Criteria *(bắt buộc)*

- **SC-001**: Ở 844×390, chữ nhỏ nhất ≥ 12 px và vùng chạm ≥ 40 px vật lý ở cả 4 khối (trước: ≈ 10 px / ≈ 24 px ở `g8_9`).
- **SC-002**: 100% cửa sổ dùng khung chung có nút đóng đỏ cùng vị trí.
- **SC-003**: Ma trận khối, GdUnit4, lint, kiểm token, export Web xanh.

## Ngoài phạm vi

- Đổi canvas cơ sở 1280×720 cho máy tính/tablet; bố cục dọc (F113: chỉ ngang).
- Đổi luật thi đấu, chat, học tập; nội dung mới.

## Assumptions

- Điện thoại ngang phổ biến có cạnh ngắn 360–430 CSS px; tablet ≥ 600.
- Renderer Compatibility (= Web) cho ảnh; so sánh bằng mắt.

## Câu hỏi mở

- (chưa có)

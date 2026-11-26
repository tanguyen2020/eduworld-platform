# Feature Specification: F099 — Student UX pass 4: màu trạng thái rõ ràng, bố cục cân, trình phát media thân thiện

**Feature Branch**: `099-student-ux-pass-4` · **Created**: 2026-11-26 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`) · **Mốc**: M3 · **Phụ thuộc**: F065, F066, F096, F097, F098
**Input**: Lệnh thường trực của owner "thiết kế game thật dễ dùng và thu hút học sinh" (lớp 1–9); orchestrator xem ảnh F098 và thấy: (1) sau một lần chạm sai, màn có **hai thẻ xanh lá** — thẻ "Đáp án đúng" và thẻ số 3 chưa chọn (bộ màu kẹo số 3 là xanh lá giống trạng thái đúng; bộ số 2 cam giống trạng thái chưa đúng) — ảnh `specs/098-student-ux-pass-3/screenshots/activity_tap_incorrect-after-1280x720-vi-g1_3.png`; (2) màn làm bài `g8_9` 1280×720 và mọi khối ở 720×1280 có một khoảng trống lớn ở giữa; (3) trình phát video, bài nghe, mô phỏng (F065/F066) chưa cùng phong cách thân thiện của F097. Chỉ đổi hiển thị phía client; không đổi hợp đồng hoạt động, luật tua/lượt nghe/chấm hay luật thời điểm phản hồi (AGENTS R1, R5, R6, R34). FR nguồn: FR-GAME-010, FR-CNT-014, FR-CNT-015, FR-CNT-016; NFR-A11Y-001, NFR-SAFE-003, NFR-COMP-004.

## Clarifications

### Session 2026-11-26

Owner có lệnh thường trực "chốt hết": điểm thiếu được chốt bằng mặc định an toàn, thân thiện với trẻ nhất và ghi [Confirmed].

- Q: Bộ màu kẹo của thẻ đáp án đổi thế nào? → A: [Confirmed] Bỏ hẳn xanh lá và cam khỏi bộ màu (không chỉ đổi độ đậm). Sáu bộ theo sắc độ (hue) nằm ngoài vùng xanh lá/cam: 1 xanh dương, 2 mận (tím hồng), 3 hồng, 4 xanh biển (cyan đậm), 5 tím, 6 chàm — thứ tự để bốn thẻ đầu (lưới 2×2) và cặp đúng/sai khác nhau rõ nhất. Ràng buộc kiểm được: sắc độ màu nhấn của mỗi bộ cách sắc độ `component.answer_tile.correct_border` (xanh lá) và `component.answer_tile.try_border` (cam) **≥ 30°**; chữ trắng trên màu nhấn và chữ chính trên nền nhạt đạt WCAG AA. Câu đúng/sai giữ bộ 1 cho "Đúng", bộ 2 cho "Sai" (nay xanh dương / mận).
- Q: Sau khi chấm, các thẻ khác hiển thị gì? → A: [Confirmed] Khi kết quả có `is_correct` (có trạng thái đúng/chưa đúng), mọi thẻ **không** phải thẻ học sinh chọn và **không** phải thẻ "Đáp án đúng" chuyển sang kiểu **nghỉ trung tính** `AnswerTileRest`: nền trắng, viền xám xanh nhạt, chữ màu chữ chính (đọc rõ, không mờ, không gạch, không xám "thất bại"), **giữ huy hiệu số** với đĩa xám mực chữ trắng. Chỉ thẻ đã chọn (đúng/chưa đúng) và thẻ "Đáp án đúng" giữ màu trạng thái; mỗi trạng thái vẫn có icon + chữ (F098-AC-04). `is_correct` rỗng (ẩn phản hồi) → không đổi thẻ nào (F098-AC-04 giữ).
- Q: Áp dụng cho dạng câu nào? → A: [Confirmed] Mọi câu có thẻ đánh dấu được kết quả (chọn một, đúng/sai một mệnh đề) ở cả luồng chạm-là-trả-lời và luồng "Kiểm tra", gồm câu chèn trong video/bài nghe/mô phỏng. Dạng nhiều bước (chọn nhiều, ghép cặp…) không có thẻ trạng thái; bộ màu mới (không xanh lá/cam) loại bỏ nhầm lẫn với dải phản hồi xanh/cam trong bong bóng.
- Q: Khoảng trống giữa màn làm bài cân bằng thế nào? → A: [Confirmed] (1) Thẻ đáp án của câu chọn một, chọn nhiều và đúng/sai một mệnh đề **cao lên** chia đều khoảng trống, tối đa `component.answer_tile.grow_max` × cao tối thiểu của khối ở cửa sổ ngang (1,5×) và `component.answer_tile.grow_max_tall` × ở cửa sổ dọc (4×) — cỡ chữ giữ đúng cỡ đề của khối; khối thẻ của câu ít hàng hơn câu bốn lựa chọn (đúng/sai) được cao lên bằng khối thẻ của câu bốn lựa chọn (tham chiếu 2 hàng ở lưới 2×2 ngang, 4 hàng ở một cột dọc) để không để lại lỗ; (2) phần trống còn lại chia **đều trên và dưới** khối câu hỏi (căn giữa dọc); (3) cửa sổ dọc (khung làm bài cao hơn rộng): câu chọn một/chọn nhiều ≤ 4 lựa chọn và hai thẻ đúng/sai xếp **một cột**, và hàng Mây + bong bóng + nút chính đi **liền dưới** khối đáp án (cả cụm căn giữa dưới thanh tiêu đề) để phản hồi nằm cạnh đáp án. Thẻ không co dưới cao tối thiểu của khối; nội dung dài vẫn cuộn như cũ.
- Q: "Không có dải trống lớn" đo thế nào? → A: [Confirmed] Trong bảng làm bài (khung `ActivityBoard`), chiếu mọi control có nội dung hiển thị (chữ, nút, thanh, ảnh, Mây) lên trục dọc; không khoảng trống dọc nào giữa hai nội dung liền nhau, hoặc từ nội dung cuối tới mép dưới bảng, được cao hơn **25%** chiều cao khung nhìn. Kiểm ở màn chọn một, đúng/sai, đang chấm, đúng, chưa đúng × 4 khối × 3 cửa sổ. Dạng nhiều bước (ghép cặp, nhập số…) ngoài phạm vi kiểm này (chỉ căn giữa) — xem Câu hỏi mở.
- Q: Trình phát video/bài nghe có những nút nào? → A: [Confirmed] Nút chính lớn (kiểu nút chính, icon + chữ): video "Phát" ▶ / "Tạm dừng" ❚❚ / "Xem lại" ↻ (khi hết video); bài nghe "Nghe" ▶ / "Tạm dừng" ❚❚ / "Nghe lại" ↻ (lượt mới — vẫn xin server như F066, hết lượt thì mờ + chữ "Em đã nghe đủ số lượt"). Khi câu hỏi/điểm rẽ nhánh chiếm khung, thanh và hàng nút tạm ẩn để câu hỏi có đủ chỗ; "Lùi 10 giây" của bài nghe ẩn khi không có lượt đang nghe. Nút phụ "Lùi 10 giây" (icon + chữ) tua lùi trong phạm vi luật hiện có (video: lùi luôn được phép; bài nghe: tua trong lượt đang nghe). Khi video đang dừng, giữa khung video có nút lớn "Phát" (icon + chữ); chạm khung video = phát/tạm dừng. Mọi nút cao ≥ `density.<band>.button.min_height`.
- Q: Thanh tiến độ? → A: [Confirmed] Thanh `MediaTrack` bo tròn cao bằng vùng chạm của khối: phần đã phát tô màu nhấn, phần đã xem (được tua tới) tô nhạt, **vạch chương** tại mốc đầu chương, **dấu câu hỏi** là đĩa tròn có "?" (chưa trả lời) hoặc ✓ (đã trả lời) — phân biệt bằng hình, không chỉ màu; tên chương đang phát hiện bằng chữ trên nút "Chương" cùng hàng điều khiển; thời gian "00:06 / 01:40" cạnh thanh. Kéo/chạm thanh để tua vẫn qua luật hiện có (`watched_only` → không qua mép đã xem, toast `ui.video.seek_locked`, Mây nhắc). Bài nghe: vạch đoạn (`segments`) và dấu câu hỏi có `at_ms`.
- Q: Phụ đề? → A: [Confirmed] Thẻ phụ đề đọc rõ `CaptionCard` ở mép dưới khung video: nền trắng, viền xanh nhạt, chữ màu chữ chính, cỡ chữ đề của khối (`density.<band>.font.stem`), tối đa 2 dòng; tên người nói trong `<v …>` của WebVTT hiện bằng **chữ** ("Cô: Hôm nay…"), không bằng màu. Nút phụ đề ghi trạng thái bằng chữ ("Phụ đề · VI" / "Phụ đề · —"). Thay quy tắc "nền mờ" của F065-AC-16 (chữ vẫn ≥ cỡ chữ cơ sở của khối).
- Q: Mây gợi ý gì trong media? → A: [Confirmed] Bong bóng Mây (khung F097) đọc lời gợi ý của renderer khi không có phản hồi/gợi ý/lỗi: video — chưa phát/đang dừng "Bấm Phát để xem video nhé!", đang phát "Xem kỹ nhé! Dấu ? trên thanh là chỗ có câu hỏi." (không có câu hỏi: "Xem kỹ nhé!"), tua bị khóa "Em xem tới đây rồi mới tua tiếp được nhé!", hết video "Em xem hết rồi, giỏi quá!", điểm rẽ nhánh "Em muốn câu chuyện đi tiếp thế nào?"; bài nghe — sẵn sàng "Bấm Nghe để bắt đầu nhé!", đã nghe một lượt "Muốn nghe thêm thì bấm Nghe lại nhé!", đang nghe "Nghe kỹ nhé!", hết lượt "Em đã nghe đủ lượt rồi. Mình trả lời câu hỏi nhé!"; mô phỏng — "Làm theo các bước rồi bấm Thả nhé!", đang kiểm "Mây đang xem kết quả…", đạt "Em làm được rồi!", chưa đạt "Chưa được, mình thử lại nhé!", hết lượt "Hết lượt thử rồi. Mình trả lời câu hỏi nhé!". Lời tử tế, không đếm ngược, không so sánh (R19).
- Q: Câu hỏi trong media? → A: [Confirmed] Overlay câu hỏi dùng nền `ActivityBoard` và đúng renderer câu hỏi F097 (thẻ câu hỏi + thẻ đáp án theo khối); sau "Kiểm tra" thẻ hiện kết quả như câu thường (F098-AC-04 + F099 thẻ nghỉ). Câu chèn vẫn dùng "Kiểm tra" (không chạm-là-trả-lời — luồng mốc/cue riêng, F098 Ngoài phạm vi).
- Q: Mô phỏng? → A: [Confirmed] Cột bên là thẻ trắng (kiểu `QuestionCard`): "Mục tiêu" — mỗi mục tiêu một hàng có icon ✓ (server đã xác nhận đạt) hoặc ○ (chưa) + chữ; "Các bước" đánh số 1, 2, 3…; "Thả" là nút chính icon + chữ, "Làm lại" nút phụ icon ↻ + chữ, "Câu hỏi" nút phụ; "Lượt thử: 1/3" bằng chữ; mọi cỡ theo khối. Mục tiêu chỉ đánh dấu từ `SimulationRunResult` đã kiểm chứng (F066-AC-23 giữ).
- Q: Cỡ theo khối cho media? → A: [Confirmed] Nút theo vùng chạm của khối, chữ ≥ `density.<band>.font.small`, thanh tiến độ cao ≥ vùng chạm, phụ đề cỡ đề của khối. Màn video (đang phát + phụ đề), video câu hỏi chèn, bài nghe, mô phỏng thêm vào ma trận test 4 khối × 3 cửa sổ (F098-AC-09).
- Q: Icon mới? → A: [Confirmed] Tự vẽ trong `design/icons/game/` (MIT, ghi `assets/LICENSES.md`): `play_light`, `pause_light` (trắng trên nút chính), `rewind` (lùi 10 giây), `captions` (CC). Nút luôn kèm chữ.
- Q: Ảnh trước/sau? → A: [Confirmed] Mọi màn đổi × 4 khối (1280×720 và 1024×600 `vi`; `en` cho `g4_5` 1280×720; 720×1280 cho màn làm bài và chạm-chưa-đúng), renderer Compatibility (= bản Web), `tools/capture_screens.gd`; màn media mới (`activity_video`, `activity_video_cue`, `activity_audio`, `activity_simulation`) chụp "trước" bằng renderer cũ.
- Q: Chip lọc Cửa hàng bị cắt ở 1024×600 tiếng Anh (F098 để lại)? → A: [Confirmed] Làm trong lượt này (P3): hàng chip cuộn ngang (`UiBuild.chip_scroller`, dùng chung cho Cửa hàng, Bộ sưu tập, Thử thách) có nút mũi tên hai bên **chỉ khi** còn chip bị ẩn; mũi tên ở đầu/cuối hàng bị vô hiệu; vuốt vẫn cuộn được.

## Bảng kiểm UX

Ảnh `screenshots/<màn>-{before,after}-<cỡ>-<ngôn ngữ>-<khối>.png`, renderer Compatibility, fixture + fake, không backend. Kết quả xem ảnh theo khối ghi ở [quickstart §5](quickstart.md).

| Màn | Trước (F098) | Sau |
|---|---|---|
| Cửa hàng 1024×600 `en` | Chip "Emotes" bị cắt, không có dấu hiệu còn chip | Nút mũi tên hai bên khi hàng chip tràn |
| S16 chạm chưa đúng | Thẻ "Đáp án đúng" xanh + thẻ số 3 (xanh lá kẹo) trông như hai thẻ đúng | Chỉ thẻ đã chọn (cam ✗) và thẻ đáp án đúng (xanh ✓) có màu; thẻ khác nghỉ trung tính, giữ số |
| S16 bộ màu | Xanh dương, cam, xanh lá, tím, hồng, xanh ngọc | Xanh dương, mận, hồng, xanh biển, tím, chàm (không xanh lá/cam) |
| S16 `g8_9` 1280×720 / mọi khối 720×1280 | Khoảng trống lớn giữa đáp án và Mây | Thẻ cao lên, khối câu hỏi căn giữa; dọc: một cột, Mây liền dưới đáp án |
| Video | Nút chữ nhỏ, thanh trượt mảnh, phụ đề nền mờ | Nút lớn icon + chữ, thanh có vạch chương + dấu "?", phụ đề thẻ trắng, Mây gợi ý |
| Bài nghe | Thanh trượt + nút chữ, vùng trống | Thẻ bài nghe có loa + tên đoạn, nút lớn, thanh có dấu câu hỏi, Mây gợi ý |
| Mô phỏng | Ô đánh dấu nhỏ, gạch đầu dòng | Thẻ mục tiêu ✓/○ + chữ, bước đánh số, "Thả" nút chính icon + chữ, Mây gợi ý |

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Màu trạng thái không bao giờ nhầm (Priority: P1) 🎯

Học sinh lớp 2 chạm nhầm "9 quả": thẻ đó thành cam ấm có ✗ "Chưa đúng, không sao!", thẻ "18 quả" xanh có ✓ "Đáp án đúng"; hai thẻ còn lại thành thẻ trắng viền xám nhạt, vẫn có số 3, 4 — không còn thẻ xanh nào khác.

**Why this priority**: Nhầm thẻ đúng là lỗi học tập (học sinh nhớ sai đáp án).

**Independent Test**: `tests/scene/learning/test_answer_state_colours.gd`; `tests/unit/learning/test_answer_palette.gd`; ảnh `activity_tap_incorrect`, `activity_tap_correct`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F099-AC-01 | Bộ màu thẻ đáp án `component.answer_tile.accent_1…6` KHÔNG ĐƯỢC chứa sắc độ xanh lá hoặc cam: sắc độ của mỗi màu nhấn PHẢI cách sắc độ `component.answer_tile.correct_border` và `component.answer_tile.try_border` ít nhất 30°, và chữ trên nền/màu nhấn PHẢI đạt WCAG AA | Orchestrator 2026-11-26 · NFR-A11Y-001 · `DESIGN_SYSTEM` §5.1 | TC-UI-001 |
| F099-AC-02 | KHI kết quả chấm của câu chọn một hoặc đúng/sai một mệnh đề có `is_correct`, mọi thẻ không phải thẻ học sinh chọn và không phải thẻ "Đáp án đúng" PHẢI chuyển sang kiểu nghỉ trung tính `AnswerTileRest` (chữ màu chữ chính, giữ huy hiệu số) và KHÔNG ĐƯỢC dùng màu nền/viền/huy hiệu của trạng thái đúng hoặc chưa đúng | Orchestrator 2026-11-26 · `STUDENT_GAME_UX_LEARNING` §4.1b | TC-UI-001 |
| F099-AC-03 | KHI có kết quả, chỉ thẻ đã chọn và thẻ "Đáp án đúng" (chỉ khi server gửi `correct_response`) PHẢI mang màu trạng thái, mỗi thẻ kèm huy hiệu hình (✓/✗) và nhãn chữ; KHI `is_correct` rỗng, HỆ THỐNG KHÔNG ĐƯỢC đổi kiểu thẻ nào | F098-AC-04 · NFR-A11Y-001 · AGENTS R6 | — |
| F099-AC-04 | Mọi cặp màu chữ mới (thẻ nghỉ, huy hiệu nghỉ, thẻ phụ đề, thẻ bài nghe) PHẢI khai báo `contrastWith` và đạt AA trong `make design-tokens-check` | NFR-A11Y-001 · AGENTS R32 | TC-UI-001 |

---

### User Story 2 — Bố cục cân, không khoảng trống lớn (Priority: P1)

Học sinh lớp 8 làm bài trên laptop: thẻ đáp án to hơn, câu hỏi nằm giữa bảng. Học sinh lớp 4 mở bản Web trên điện thoại dọc: bốn thẻ xếp một cột cao, Mây và nút "Tiếp" ngay dưới đáp án.

**Independent Test**: `tests/scene/test_grade_band_matrix.gd` (kiểm "dải trống"), `tests/scene/learning/test_balanced_layout.gd`; ảnh `activity*` 720×1280 và `g8_9`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F099-AC-05 | KHI vùng làm bài còn trống, thẻ đáp án của câu chọn một, chọn nhiều và đúng/sai một mệnh đề PHẢI cao lên chia đều khoảng trống, không vượt `component.answer_tile.grow_max` × `density.<band>.answer.min_height` ở cửa sổ ngang và `component.answer_tile.grow_max_tall` × ở cửa sổ dọc, không thấp hơn `density.<band>.answer.min_height`, cỡ chữ giữ `density.<band>.font.stem`; phần trống còn lại PHẢI chia đều trên và dưới khối câu hỏi | Orchestrator 2026-11-26 · `STUDENT_GAME_UX_LEARNING` §3.3a · `LEARNING_EXPERIENCE` §8 | TC-UI-003 |
| F099-AC-06 | TRONG KHI khung làm bài cao hơn rộng, câu chọn một/chọn nhiều có ≤ 4 lựa chọn PHẢI xếp một cột, và hàng Mây + bong bóng + nút chính PHẢI nằm liền dưới khối đáp án (cả cụm căn giữa dưới thanh tiêu đề) | Orchestrator 2026-11-26 · `STUDENT_GAME_UX_LEARNING` §3.2 | TC-UI-003 |
| F099-AC-07 | Với mỗi khối × cửa sổ 1280×720, 1024×600, 720×1280, test ma trận PHẢI kiểm ở màn chọn một, đúng/sai, đang chấm, đúng, chưa đúng rằng trong bảng làm bài không có khoảng trống dọc nào giữa hai nội dung liền nhau hoặc từ nội dung cuối tới mép dưới bảng cao hơn 25% chiều cao khung nhìn, và probe PHẢI báo lỗi trên một bố cục có dải trống (kiểm ngược) | `DESIGN_SYSTEM` §9 · F098-AC-09 | TC-UI-003 |

---

### User Story 3 — Trình phát video, bài nghe, mô phỏng thân thiện (Priority: P2)

Học sinh lớp 3 mở video "Phép nhân": giữa khung có nút lớn "▶ Phát", Mây nói "Bấm Phát để xem video nhé!". Khi phát, phụ đề hiện trong thẻ trắng chữ to "Cô: Hôm nay mình học phép nhân."; thanh tiến độ có vạch chương và dấu "?" chỗ có câu hỏi. Tới dấu "?", video dừng, câu hỏi hiện bằng thẻ câu hỏi và thẻ đáp án như bài thường.

**Independent Test**: `tests/scene/video/test_media_player_ux.gd`, `tests/scene/audio/test_audio_lesson_player.gd`, `tests/scene/simulation/test_simulation_host.gd`, `tests/scene/test_grade_band_matrix.gd`; ảnh `activity_video*`, `activity_audio`, `activity_simulation`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F099-AC-08 | Trình phát video và bài nghe PHẢI có nút chính icon + chữ đổi theo trạng thái (video "Phát"/"Tạm dừng"/"Xem lại"; bài nghe "Nghe"/"Tạm dừng"/"Nghe lại") và nút "Lùi 10 giây" icon + chữ; KHI video đang dừng, giữa khung video PHẢI có nút lớn "Phát" icon + chữ; mọi nút PHẢI cao ≥ `density.<band>.button.min_height`; "Lùi 10 giây" PHẢI đi qua luật tua hiện có | Orchestrator 2026-11-26 · `STUDENT_GAME_UX_LEARNING` §5, §6 · F065-AC-17 · F066-AC-19 | TC-VID-001 |
| F099-AC-09 | Thanh tiến độ media PHẢI cao ≥ `density.<band>.button.min_height`, tô phần đã phát và phần đã xem, có vạch tại mốc đầu mỗi chương (đoạn của bài nghe) và dấu câu hỏi hình tròn có "?" khi chưa trả lời, ✓ khi đã trả lời (hình, không chỉ màu); tên chương đang phát PHẢI hiện bằng chữ (nút "Chương"); tua bằng thanh PHẢI giữ luật `allow_seek` hiện có | Orchestrator 2026-11-26 · F065-AC-17 · NFR-A11Y-001 | TC-VID-001 |
| F099-AC-10 | Phụ đề PHẢI hiện trong thẻ `CaptionCard` (nền sáng, viền, chữ màu chữ chính, cỡ `density.<band>.font.stem`); tên người nói của `<v …>` PHẢI hiện bằng chữ trước lời; nút phụ đề PHẢI ghi trạng thái bằng chữ; trạng thái phụ đề và người nói KHÔNG ĐƯỢC chỉ dùng màu | Orchestrator 2026-11-26 · F065-AC-16 · NFR-A11Y-001 | TC-VID-001 |
| F099-AC-11 | TRONG KHI bong bóng Mây không có phản hồi, gợi ý hay lỗi, nó PHẢI hiện lời gợi ý của trình phát theo trạng thái (video: chưa phát/đang phát/tua bị khóa/hết video/rẽ nhánh; bài nghe: sẵn sàng/đang nghe/hết lượt; mô phỏng: hướng dẫn/đang kiểm/đạt/chưa đạt/hết lượt) bằng lời tử tế, không đếm ngược, không so sánh | Orchestrator 2026-11-26 · `STUDENT_GAME_UX_LEARNING` §3.3a · AGENTS R19 · NFR-SAFE-003 | — |
| F099-AC-12 | KHI câu hỏi chèn (video, bài nghe) hoặc câu hỏi kết luận (mô phỏng) mở, overlay PHẢI dùng nền `ActivityBoard` và renderer câu hỏi F097 (thẻ câu hỏi, thẻ đáp án theo khối); KHI có kết quả, thẻ PHẢI theo F099-AC-02/AC-03 | Orchestrator 2026-11-26 · F097-AC-04 | — |
| F099-AC-13 | Mô phỏng PHẢI hiện mục tiêu dạng hàng có icon ✓ (chỉ khi `SimulationRunResult` đã kiểm chứng đạt) hoặc ○ kèm chữ, các bước đánh số, "Thả" là nút chính icon + chữ, "Làm lại" nút phụ icon + chữ, số lượt thử bằng chữ "Lượt thử: n/m"; cỡ theo khối | Orchestrator 2026-11-26 · F066-AC-23 · `STUDENT_GAME_UX_LEARNING` §8 | TC-SIM-001 |
| F099-AC-14 | Trình phát media KHÔNG ĐƯỢC đổi request/response, heartbeat, luật tua, lượt nghe, lượt thử hay chấm: test F065/F066 hiện có PHẢI xanh | AGENTS R5, R6, R34 · FR-CNT-014, FR-CNT-015, FR-CNT-016 | — |
| F099-AC-15 | Test ma trận PHẢI phủ màn video (đang phát + phụ đề), video câu hỏi chèn, bài nghe và mô phỏng ở 4 khối × 3 cửa sổ: nút ≥ vùng chạm, chữ ≥ `density.<band>.font.small`, thanh tiến độ cao ≥ vùng chạm, không tràn khung | F098-AC-09 · NFR-A11Y-001 | TC-UI-003 |

---

### User Story 4 — Bằng chứng trước/sau 4 khối (Priority: P1)

**Independent Test**: ảnh `screenshots/*-{before,after}-*-{g1_3,g4_5,g6_7,g8_9}.png`; bảng khối × màn ở quickstart §5.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F099-AC-16 | HỆ THỐNG PHẢI có ảnh "trước" và "sau" cho mọi màn đổi ở cả 4 khối (1280×720 và 1024×600 `vi`; `en` 1280×720 cho `g4_5`; 720×1280 cho màn làm bài và chạm-chưa-đúng) trên renderer Compatibility bằng `tools/capture_screens.gd`, và lỗi phát hiện khi xem ảnh PHẢI được sửa hoặc ghi ở Câu hỏi mở | `DESIGN_SYSTEM` §8, §9 · NFR-COMP-004 | TC-UI-007 |
| F099-AC-18 | KHI hàng chip lọc (Cửa hàng, Bộ sưu tập, Thử thách) rộng hơn chỗ của nó, HỆ THỐNG PHẢI hiện nút mũi tên hai bên (cao ≥ vùng chạm của khối, mũi tên ở đầu/cuối hàng bị vô hiệu) cuộn hàng khi bấm; KHI hàng vừa chỗ, KHÔNG ĐƯỢC hiện mũi tên | F098 Câu hỏi mở (chip Cửa hàng 1024×600 `en`) · `STUDENT_GAME_UX_SOCIAL` §3 | TC-UI-003 |
| F099-AC-17 | Mọi chuỗi mới PHẢI qua `.po` `vi`/`en` (`extract_pot --check`, `pseudo_locale --check`, `test_font_coverage.gd` xanh); mọi màu/cỡ mới PHẢI từ token (`lint_theme_overrides`, `make design-tokens-check` xanh) | NFR-I18N-001 · AGENTS R32 · F098-AC-11 | TC-UI-001 |

### Edge Cases

- NẾU câu có 5–6 lựa chọn, THÌ ở cửa sổ dọc vẫn một cột như F097 (đã một cột) và thẻ cao lên theo F099-AC-05 (F099-AC-06).
- NẾU nội dung cao hơn vùng làm bài (đề dài, nhiều lựa chọn), THÌ thẻ giữ cao tối thiểu, không căn giữa, nội dung cuộn như cũ (F099-AC-05).
- NẾU học sinh xoay/đổi cỡ cửa sổ, THÌ bố cục tính lại; thẻ không đổi cao khi bong bóng phản hồi hiện ra trừ khi không đủ chỗ (F099-AC-05).
- NẾU video không có phụ đề, THÌ nút phụ đề ẩn và không có thẻ phụ đề (F099-AC-10).
- NẾU bài nghe hết lượt, THÌ nút chính mờ, chữ "Em đã nghe đủ số lượt", Mây nhắc trả lời câu hỏi (F099-AC-08, F099-AC-11).
- NẾU `max_runs` không giới hạn, THÌ không hiện "Lượt thử" (F099-AC-13).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-010 | UI theo khối: thẻ trạng thái rõ, bố cục cân, media theo vùng chạm/cỡ chữ của khối | `STUDENT_GAME_UX_LEARNING` §3.3a, §4.1a, §4.1b · `DESIGN_SYSTEM` §5.1, §5.2 |
| FR-CNT-014 | Trình phát video: nút, thanh tiến độ, phụ đề, câu hỏi chèn (chỉ hiển thị) | `VIDEO_LEARNING` · `STUDENT_GAME_UX_LEARNING` §5 |
| FR-CNT-015 | Trình phát bài nghe (chỉ hiển thị) | `AUDIO_LANGUAGE_LEARNING` · `STUDENT_GAME_UX_LEARNING` §6 |
| FR-CNT-016 | Khung mô phỏng: mục tiêu, bước, nút (chỉ hiển thị) | `SIMULATION_ENGINE` · `STUDENT_GAME_UX_LEARNING` §8 |

### Key Entities

- **Token mới** (`tokens@0.16.0`): bộ màu `component.answer_tile.{bg,accent}_1…6` mới; `component.answer_tile.{rest_bg,rest_border,rest_accent}`; `component.answer_tile.{grow_max,grow_max_tall}`; `component.caption.{bg,border,text}`; `component.media_track.{track,watched,played,marker,marker_text,chapter}`; Theme `AnswerTileRest`, `CaptionCard`, `CaptionLabel`, `MediaStage`.
- **`AnswerBadge.Verdict.REST`** (huy hiệu số trung tính), **`QuestionRendererBase.fit_height()`** (thẻ cao lên), **`ActivityRendererBase.fit_layout()` / `coach_hint()` / tín hiệu `coach_hint_changed`**, bố cục cân trong `ActivityHost`.
- **`MediaTrack`** (`scenes/ui/components/media_track.gd`, `Range`): thanh tiến độ vẽ bằng token, vạch chương, dấu câu hỏi.
- **Test** `test_answer_state_colours.gd`, `test_answer_palette.gd`, `test_balanced_layout.gd`, `test_media_player_ux.gd`; `BandLayoutProbe.empty_bands()`.

### Tham số

| Tham số | Dùng để |
|---|---|
| `component.answer_tile.grow_max` = 1,5; `component.answer_tile.grow_max_tall` = 4 (`DESIGN_SYSTEM` §5.1) | F099-AC-05 |
| Ngưỡng dải trống 25% chiều cao khung nhìn (`DESIGN_SYSTEM` §9, chỉ trong test) | F099-AC-07 |
| Khoảng lùi 10 giây (`STUDENT_GAME_UX_LEARNING` §5.1, hằng client) | F099-AC-08 |
| `density.<band>.button.min_height`, `density.<band>.font.small`, `density.<band>.font.stem`, `density.<band>.answer.min_height` (`DESIGN_SYSTEM` §5.2, `LEARNING_EXPERIENCE` §8) | F099-AC-05, F099-AC-08…F099-AC-10, F099-AC-15 |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Client không chấm, không biết đáp án; thẻ "Đáp án đúng" chỉ từ `correct_response` server gửi | AGENTS R5, R6 · ADR-0016 |
| Quyết định bố cục/lời gợi ý ở renderer/helper, không luật nghiệp vụ trong scene; lượt nghe/tua/lượt thử vẫn là của server | AGENTS R1 |
| Không đổi contract hoạt động, luật FeedbackTiming, luật tua, lượt nghe | AGENTS R33, R34 |
| Lời tử tế, cam ấm không đỏ, không đếm ngược/so sánh | AGENTS R19 · NFR-SAFE-003 |
| Không hardcode màu/cỡ; file sinh không sửa tay | AGENTS R32 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Sau khi chấm, số thẻ mang màu xanh lá = số thẻ đúng/"Đáp án đúng" (trước: có thể thêm 1 thẻ kẹo xanh lá).
- **SC-002**: Không dải trống > 25% khung nhìn ở màn chọn một/đúng-sai/phản hồi × 4 khối × 3 cửa sổ (trước: ~32% ở `g8_9` 1280×720, ~78% ở 720×1280).
- **SC-003**: Màn video/bài nghe/mô phỏng pass ma trận 4 khối × 3 cửa sổ; GdUnit4 toàn bộ xanh; mọi lint xanh; export Web thành công.

## Ngoài phạm vi

- Đổi contract hoạt động, luật FeedbackTiming/ResultRelease, luật tua, lượt nghe, lượt thử, luật chấm (R34).
- Chạm-là-trả-lời trong câu chèn media (F098 Ngoài phạm vi giữ).
- Đổi tỉ lệ cơ sở của cửa sổ dọc (logic 1280×2276) cho toàn game — chữ trên điện thoại dọc vẫn nhỏ về vật lý (Câu hỏi mở F099-1).
- Âm thanh/hạt hiệu ứng mới; video thật trong ảnh chụp (fixture không giải mã video).

## Assumptions

- Font đóng gói phủ mọi chuỗi mới; ảnh render bằng GPU (Intel Iris Xe, Compatibility); so sánh bằng mắt, không so pixel.
- Ảnh media dùng playback giả (TEST-ONLY) nên khung video tối, không có hình.

## Câu hỏi mở

- F099-1 [Proposed]: Điện thoại cầm dọc trên Web dùng canvas logic 1280×2276 (stretch `canvas_items`/`expand` trên cơ sở 1280×720) nên chữ về vật lý chỉ ≈ 56% cỡ token. Đề xuất feature riêng: đổi cơ sở canvas sang 720×1280 khi cửa sổ dọc và rà lại mọi màn ở bề ngang logic 720.
- F099-2 [Proposed]: Dạng nhiều bước (ghép cặp, nhập số, sắp xếp…) ở cửa sổ dọc chỉ được căn giữa, vẫn còn khoảng trống trên/dưới; chưa kiểm "dải trống" cho các màn này.

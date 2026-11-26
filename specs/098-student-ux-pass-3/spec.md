# Feature Specification: F098 — Student UX pass 3: chạm là trả lời, ma trận kiểm tra 4 khối

**Feature Branch**: `098-student-ux-pass-3` · **Created**: 2026-11-26 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`) · **Mốc**: M3 · **Phụ thuộc**: F063, F069, F096, F097
**Input**: Owner 2026-11-26: "đáp án thể hiện luôn khi chọn đáp án làm được không? hiện tại chưa thấy làm" — hôm nay học sinh chọn một đáp án rồi phải bấm thêm "Kiểm tra" mới được chấm. Owner duyệt thiết kế "chạm là trả lời" (phần 1) và kế hoạch "ma trận test khối 1–9" (phần 2): Godot test và ảnh chụp phủ `g1_3` nhiều, `g6_7` một phần, `g4_5` 0 ảnh, `g8_9` 4 file test và 0 ảnh. Chỉ đổi cách tương tác/hiển thị phía client; không đổi hợp đồng hoạt động, luật FeedbackTiming hay luật chấm (AGENTS R1, R5, R6, R34). FR nguồn: FR-EXR-003, FR-QB-013, FR-GAME-010; NFR-A11Y-001, NFR-I18N-001, NFR-COMP-004, NFR-SAFE-003.

## Clarifications

### Session 2026-11-26

Owner có lệnh thường trực "chốt hết": điểm thiếu được chốt bằng mặc định an toàn, thân thiện với trẻ nhất và ghi [Confirmed].

- Q: Chạm là trả lời áp dụng ở đâu? → A: [Confirmed — owner duyệt 2026-11-26] Câu **chọn một** (`single_choice`) và **đúng/sai một mệnh đề** (`true_false` không có `statements[]`), chỉ khi FeedbackTiming **hiệu lực** của attempt là `immediate` (ACTIVITY_ENGINE §8.1): `attempt.feedback_timing = immediate` **và** `mode ∉ {exam, diagnostic, competition}` **và** `result_release` rỗng hoặc `immediate`. Thực tế: mode `practice`, `quest`, `challenge`, `minigame`, `retry_wrong` và bài giao cấu hình `immediate`. Client chỉ đọc giá trị server trả trong `Attempt`; không tự suy luật (ACTIVITY_ENGINE sở hữu luật thời điểm).
- Q: Renderer nào? → A: [Confirmed] Renderer từng câu một của `quiz` và `interactive_exercise` (`QuizActivityRenderer`). Câu chèn video/bài nghe/mô phỏng giữ "Kiểm tra" (luồng mốc/cue riêng của F065/F066); trình mini-game F073 có vòng nhập riêng nên không đổi; đề thi (ExamRunner F064) không đổi.
- Q: Chạm làm gì? → A: [Confirmed — owner duyệt] (1) Gửi ngay qua API trả lời từng câu hiện có (`POST …/responses`); server chấm, client không biết đáp án (R5, R6). (2) Trong lúc chấm, mọi thẻ khóa (không nhận chạm/phím) và thẻ vừa chạm có nhãn "Đang chấm…"; không gửi hai lần; lỗi mạng tạm thời gửi lại cùng `Idempotency-Key` (hàng đợi của `LearningService`, khóa theo vị trí + câu trả lời). (3) Có kết quả: thẻ vừa chạm thành trạng thái **đúng** (nền xanh lá nhạt, viền xanh, huy hiệu ✓, nhãn "Đúng rồi!") hoặc **chưa đúng** (nền cam ấm nhạt, viền cam, huy hiệu ✗ mềm, nhãn "Chưa đúng, không sao!"). (4) Nếu kết quả có `correct_response` (chỉ khi chính sách cho phép — contract) thì thẻ đó có trạng thái "Đáp án đúng" (✓); giải thích/phản hồi theo phương án hiện trong bong bóng của Mây như F097; không hiện gì server không gửi. (5) Nét mặt Mây đổi theo kết quả (vui / động viên — F097).
- Q: Dạng nhiều bước? → A: [Confirmed — owner duyệt] Chọn nhiều, sắp xếp, kéo thả, điền khuyết, nhập số, ghép cặp, đúng/sai nhiều mệnh đề, câu hình ảnh, trả lời ngắn, tương tác: giữ nút "Kiểm tra".
- Q: Mode không đổi? → A: [Confirmed — owner duyệt] `exam`, `diagnostic`, `competition` và mọi attempt có `feedback_timing ≠ immediate` hoặc `result_release ∉ {rỗng, immediate}`: chạm chỉ chọn, vẫn "Kiểm tra"/"Lưu", không phản hồi giữa chừng (F063-AC-14 giữ nguyên).
- Q: Chạm nhầm? → A: [Confirmed — owner duyệt] Mặc định **một chạm**. Có cài đặt tùy chọn theo khối "chạm hai lần để chắc chắn", mặc định **tắt** cho cả 4 khối: cấu hình client `ProjectSettings eduworld/learning/tap_confirm_bands` (danh sách khối, mặc định rỗng; đăng ký `tap_confirm_bands` ở CONFIG_DEFAULTS §13, quy tắc ở STUDENT_GAME_UX_LEARNING §4.1b). Không có chính sách backend mới. Khi khối bật: chạm lần 1 chọn thẻ (✓) và Mây nói "Chạm lần nữa để chắc chắn nhé!"; chạm lại **đúng thẻ đó** (hoặc phím số đó, hoặc nút chính) mới gửi; chạm thẻ khác chỉ đổi lựa chọn.
- Q: Gửi lỗi thì sao? → A: [Confirmed] Lỗi bị từ chối (4xx): thẻ mở khóa, lựa chọn giữ nguyên, bong bóng báo lỗi như F063-AC-10, nút chính "Gửi lại" (↻) hiện; chạm lại thẻ đang chọn cũng gửi lại. Mất mạng: câu trả lời vào hàng đợi (F063-AC-06), thẻ giữ "Đang chấm…", banner mất kết nối; có mạng thì kết quả hiện như thường.
- Q: Hướng dẫn trên câu? → A: [Confirmed] Khi chạm là trả lời, dòng hướng dẫn của câu đổi thành "Chạm vào đáp án em chọn" (chế độ hai chạm: "Chạm hai lần vào đáp án em chọn"); nút "Kiểm tra" ẩn khi đang trả lời (không có bước thứ hai). Phím `1–6` = chạm thẻ số đó; `Enter` = nút chính; focus bàn phím luôn thấy (vòng focus của Theme).
- Q: Trợ năng? → A: [Confirmed — NFR-A11Y-001] Trạng thái thẻ không bao giờ chỉ bằng màu: "Đang chấm…" là nhãn chữ + huy hiệu ba chấm; đúng = ✓ + chữ; chưa đúng = ✗ + chữ; đáp án đúng = ✓ + chữ.
- Q: Thời gian trả lời / rapid guess? → A: [Confirmed] Không đổi: `client_elapsed_ms` chỉ tham khảo, server dùng đồng hồ của nó (ADR-0016); cờ `rapid_guess` do server tính (ACTIVITY_ENGINE §11.1).
- Q: Thử lại câu sai (`attempts_per_question`)? → A: Ngoài phạm vi (client hiện chưa có luồng thử lại; giữ như F063/F097 — câu sau khi chấm khóa, "Tiếp").
- Q: Ma trận khối kiểm gì? → A: [Confirmed — owner duyệt "ma trận test khối 1–9"] Test scene chạy cho **mỗi** khối `g1_3`, `g4_5`, `g6_7`, `g8_9` ở ba cửa sổ tham chiếu 1280×720, 1024×600, 720×1280 (logic 1280×720, 1280×750, 1280×2276 với `stretch canvas_items/expand`): mọi nút hiển thị cao ≥ `density.<band>.button.min_height`; mọi chữ ≥ `density.<band>.font.small`; thẻ đáp án cao ≥ `density.<band>.answer.min_height` và chữ thẻ = `density.<band>.font.stem`; không control hiển thị nào tràn khỏi khung nhìn (trừ nội dung trong `ScrollContainer`, kiểm khung cuộn); nhãn một dòng không bị cắt chữ. Màn: S01 đăng nhập, HUD Hub, làm bài (chọn một, đúng/sai, ghép cặp, nhập số), phản hồi (đang chấm, đúng, chưa đúng), kết quả, tạo nhân vật S02, cửa hàng/tủ đồ (F069), lên cấp, bản đồ thế giới.
- Q: Ảnh theo khối? → A: [Confirmed] Cùng các màn ở 4 khối (`tools/capture_screens.gd`, renderer Compatibility = bản Web): 1280×720 và 1024×600 `vi` cho cả 4 khối, thêm `en` 1280×720 cho `g4_5`, thêm 720×1280 cho màn chạm-đúng và HUD; xem ảnh và sửa lỗi phát hiện.

## Bảng kiểm UX

Ảnh `screenshots/<màn>-after-<cỡ>-<ngôn ngữ>-<khối>.png`, renderer Compatibility, fixture + fake, không backend. Kết quả xem ảnh theo khối ghi ở [quickstart §5](quickstart.md#5-bằng-chứng-2026-11-26-local-windows-11-intel-iris-xe-godot-472-console).

| Màn | Trước (F097) | Sau |
|---|---|---|
| S16 chọn một / đúng-sai, mode practice | Chạm chọn → bấm "Kiểm tra" → kết quả (2 bước) | Chạm → "Đang chấm…" trên thẻ → thẻ đúng/chưa đúng có icon + chữ, đáp án đúng nếu server gửi, Mây đổi nét mặt (1 bước) |
| S16 dạng nhiều bước | Kiểm tra | Giữ nguyên |
| Mọi màn ở `g4_5`, `g8_9` | Không có ảnh; test khối chỉ ở vài màn | Ảnh 4 khối; test khối ở mọi màn của ma trận |

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Chạm là trả lời (Priority: P1) 🎯

Học sinh lớp 2 luyện tập ở Thư viện Gió: chạm thẻ "18 quả" — thẻ hiện "Đang chấm…", rồi thành thẻ xanh có ✓ "Đúng rồi!", Mây cười. Chạm nhầm "9 quả" ở câu sau: thẻ thành cam ấm có ✗ "Chưa đúng, không sao!", thẻ "18 quả" hiện "Đáp án đúng" (vì bài luyện tập cho phép), Mây nói lời giải thích.

**Why this priority**: Owner yêu cầu trực tiếp; bỏ một bước thao tác ở dạng câu dùng nhiều nhất cho lớp nhỏ.

**Independent Test**: `tests/scene/learning/test_tap_to_answer.gd`, `tests/unit/learning/test_instant_answer_policy.gd`; ảnh `activity_tap_*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F098-AC-01 | KHI học sinh chạm (hoặc bấm phím `1–6`) một thẻ đáp án của câu chọn một hay đúng/sai một mệnh đề TRONG KHI attempt có `feedback_timing = immediate`, `mode ∉ {exam, diagnostic, competition}` và `result_release` rỗng hoặc `immediate`, HỆ THỐNG PHẢI gửi ngay câu trả lời qua `POST …/responses` (đúng hình `QuestionResponse`, không điểm/`is_correct`) mà không cần bấm "Kiểm tra" | Owner 2026-11-26 · `ACTIVITY_ENGINE` §8.1 · FR-EXR-003 · AGENTS R5, R6 | TC-LRN-002 |
| F098-AC-02 | NẾU attempt ở mode `exam`, `diagnostic` hoặc `competition`, hoặc `feedback_timing ≠ immediate`, hoặc `result_release ∉ {rỗng, immediate}`, hoặc câu là dạng nhiều bước (chọn nhiều, sắp xếp, kéo thả, điền khuyết, nhập số, ghép cặp, đúng/sai nhiều mệnh đề…), THÌ chạm thẻ PHẢI chỉ chọn đáp án, KHÔNG ĐƯỢC gửi, và nút "Kiểm tra" PHẢI giữ như F063/F097 | Owner 2026-11-26 · `ACTIVITY_ENGINE` §8.1 · F063-AC-14 | TC-LRN-007 |
| F098-AC-03 | TRONG KHI câu trả lời đang được chấm, HỆ THỐNG PHẢI khóa mọi thẻ của câu (chạm/phím không đổi lựa chọn) và hiện nhãn "Đang chấm…" kèm huy hiệu ba chấm trên thẻ vừa chạm; chạm/phím lặp lại KHÔNG ĐƯỢC gửi thêm yêu cầu; một lần gửi lại do mất mạng PHẢI dùng lại cùng `Idempotency-Key` | Owner 2026-11-26 · AGENTS R10 · F063-AC-06 | TC-LRN-008 |
| F098-AC-04 | KHI server trả kết quả có `is_correct`, thẻ vừa chạm PHẢI chuyển sang trạng thái đúng (nền xanh lá nhạt, huy hiệu ✓, nhãn "Đúng rồi!") hoặc chưa đúng (nền cam ấm nhạt, huy hiệu ✗, nhãn "Chưa đúng, không sao!"); trạng thái KHÔNG ĐƯỢC chỉ bằng màu; KHI kết quả có `correct_response`, thẻ tương ứng PHẢI có trạng thái "Đáp án đúng" (✓ + chữ); KHI kết quả không có `correct_response`, HỆ THỐNG KHÔNG ĐƯỢC đánh dấu thẻ nào khác; KHI `is_correct` rỗng, thẻ KHÔNG ĐƯỢC có trạng thái đúng/chưa đúng | Owner 2026-11-26 · NFR-A11Y-001 · NFR-SAFE-003 · AGENTS R6, R19 | — |
| F098-AC-05 | KHI có kết quả chạm-trả-lời, nét mặt Mây PHẢI là vui nếu đúng, động viên nếu chưa đúng; bong bóng PHẢI hiện bảng kết quả và giải thích/phản hồi theo phương án server gửi (F097-AC-06…08); nút chính PHẢI là "Tiếp"/"Hoàn thành" | F097-AC-08 · `STUDENT_GAME_UX_LEARNING` §3.3a | — |
| F098-AC-06 | KHI server từ chối câu trả lời (4xx), thẻ PHẢI mở khóa, lựa chọn giữ nguyên, lỗi hiện như F063-AC-10, và học sinh PHẢI gửi lại được bằng nút chính "Gửi lại" hoặc chạm lại thẻ đang chọn | F063-AC-10 | — |
| F098-AC-07 | TRONG KHI chạm là trả lời, dòng hướng dẫn của câu PHẢI là "Chạm vào đáp án em chọn" (hai chạm: "Chạm hai lần vào đáp án em chọn"), nút "Kiểm tra" PHẢI ẩn; phím `1–6` PHẢI hành xử như chạm thẻ số đó; mọi thẻ PHẢI có vòng focus bàn phím của Theme | `STUDENT_GAME_UX_LEARNING` §3.6 · NFR-A11Y-001 | — |

---

### User Story 2 — Chạm hai lần để chắc chắn (Priority: P3)

Trường muốn lớp 1–3 tránh chạm nhầm: bật "chạm hai lần" cho `g1_3`. Học sinh chạm "18 quả" — thẻ có ✓, Mây nói "Chạm lần nữa để chắc chắn nhé!"; chạm lại thẻ đó mới chấm.

**Independent Test**: `test_tap_to_answer.gd`, `test_instant_answer_policy.gd`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F098-AC-08 | HỆ THỐNG PHẢI đọc cài đặt client `eduworld/learning/tap_confirm_bands` (mặc định rỗng = một chạm ở mọi khối); NẾU khối của học sinh có trong danh sách, THÌ chạm lần đầu PHẢI chỉ chọn thẻ và Mây PHẢI nhắc "Chạm lần nữa để chắc chắn nhé!", và chỉ chạm lại đúng thẻ đó (hoặc phím số đó, hoặc nút chính) mới gửi; chạm thẻ khác PHẢI chỉ đổi lựa chọn | Owner 2026-11-26 · `STUDENT_GAME_UX_LEARNING` §4.1b · `CONFIG_DEFAULTS` §13 | — |

---

### User Story 3 — Ma trận kiểm tra 4 khối (Priority: P1)

Đội phát triển chạy một bộ test scene cho mọi khối và mọi cửa sổ tham chiếu; owner xem ảnh của cả 4 khối.

**Independent Test**: `tests/scene/test_grade_band_matrix.gd`; ảnh `screenshots/*-{g1_3,g4_5,g6_7,g8_9}.png`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F098-AC-09 | Với mỗi khối `g1_3`, `g4_5`, `g6_7`, `g8_9` và mỗi cửa sổ 1280×720, 1024×600, 720×1280, test scene PHẢI kiểm trên S01, HUD Hub, làm bài (chọn một, đúng/sai, ghép cặp, nhập số), phản hồi (đang chấm, đúng, chưa đúng), kết quả, S02, cửa hàng/tủ đồ, lên cấp, bản đồ: mọi nút hiển thị cao ≥ `density.<band>.button.min_height`, mọi chữ ≥ `density.<band>.font.small`, thẻ đáp án cao ≥ `density.<band>.answer.min_height` với chữ = `density.<band>.font.stem`, không control hiển thị nào tràn khỏi khung nhìn (ngoài nội dung cuộn), nhãn một dòng không bị cắt | `STUDENT_GAME_UX` §4.3 · `DESIGN_SYSTEM` §5.2 · `LEARNING_EXPERIENCE` §8 · NFR-A11Y-001 | TC-UI-003 |
| F098-AC-10 | HỆ THỐNG PHẢI có ảnh "sau" cho cùng các màn ở cả 4 khối (1280×720 và 1024×600 `vi`; `en` 1280×720 cho `g4_5`; 720×1280 cho màn chạm-đúng và HUD) trên renderer Compatibility bằng `tools/capture_screens.gd`, và lỗi phát hiện khi xem ảnh PHẢI được sửa hoặc ghi ở Câu hỏi mở | `DESIGN_SYSTEM` §8, §9 · NFR-COMP-004 | TC-UI-007 |
| F098-AC-11 | Mọi chuỗi mới PHẢI qua `.po` `vi`/`en` (`extract_pot --check`, `pseudo_locale --check`, `test_font_coverage.gd` xanh); mọi màu/cỡ mới PHẢI từ token (`lint_theme_overrides`, `make design-tokens-check` xanh) | NFR-I18N-001 · AGENTS R32 · F097-AC-13 | TC-UI-001 |

### Edge Cases

- NẾU học sinh chạm hai thẻ thật nhanh, THÌ chỉ thẻ đầu được gửi (thẻ khóa ngay khi gửi) (F098-AC-03).
- NẾU server trả `is_correct = null` dù mode cho phép (cấu hình bài giao lạ), THÌ chỉ "Đã lưu câu trả lời." như F063-AC-14, thẻ không có trạng thái đúng/sai (F098-AC-04).
- NẾU attempt được mở lại (resume) với câu đã có câu trả lời lưu, THÌ không tự gửi khi dựng lại câu (chỉ thao tác chạm/phím của học sinh mới gửi) (F098-AC-01).
- NẾU hết giờ trong lúc đang chấm, THÌ luồng hết giờ của F063 thắng (khóa, nộp) (F098-AC-03).
- NẾU khối không có trong `UiTokens.DENSITY_BANDS`, THÌ dùng khối mặc định của ThemeService (F098-AC-09).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-EXR-003 | Phản hồi đúng/sai ngay sau từng câu trong practice — không cần bước "Kiểm tra" với câu một bước | `ACTIVITY_ENGINE` §8.1 (thời điểm) · `STUDENT_GAME_UX_LEARNING` §4.1b (tương tác) |
| FR-QB-013 | Phản hồi theo phương án, đáp án đúng chỉ từ kết quả chấm | `ACTIVITY_ENGINE` §8.2 |
| FR-GAME-010 | UI theo khối: kiểm tra cỡ chữ, vùng chạm, bố cục ở cả 4 khối | `STUDENT_GAME_UX` §4.3 · `DESIGN_SYSTEM` §5.2 |

### Key Entities

- **`InstantAnswerPolicy`** (`scripts/learning/instant_answer_policy.gd`): đọc `AttemptModel` + `QuestionRenderModel` + cài đặt client, trả có chạm-là-trả-lời / có hai chạm.
- **Tín hiệu `choice_tapped(repeated)`** (QuestionRendererBase → ActivityRendererBase → ActivityHost); `set_grading`, `show_verdict` của câu chọn một/đúng-sai.
- **`ResponseResultModel.correct_response`** (chỉ đọc từ contract, chỉ hiển thị, không lưu/log).
- **Biến thể Theme mới** (`tokens@0.15.0`): `AnswerTileCorrect`, `AnswerTileTry`, `TileTagCorrect`, `TileTagTry`, `TileTagPending`; `AnswerBadge.verdict`.
- **Test** `tests/scene/test_grade_band_matrix.gd` + helper `tests/fakes/band_layout_probe.gd`.

### Tham số

| Tham số | Dùng để |
|---|---|
| `tap_confirm_bands` = rỗng (`ProjectSettings eduworld/learning/tap_confirm_bands`, `CONFIG_DEFAULTS` §13, quy tắc `STUDENT_GAME_UX_LEARNING` §4.1b) | F098-AC-08 |
| `density.<band>.button.min_height` 64/56/48/44, `density.<band>.font.small`, `density.<band>.font.stem` 32/28/24/22, `density.<band>.answer.min_height` 88/80/72/64 (`DESIGN_SYSTEM` §5.2, `LEARNING_EXPERIENCE` §8) | F098-AC-09 |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Client không chấm, không biết đáp án; đáp án đúng chỉ từ `correct_response` server gửi; không lưu/log câu trả lời | AGENTS R5, R6, R14 · ADR-0016 |
| Quyết định "có chạm-là-trả-lời" nằm trong helper `scripts/learning/`, không trong scene | AGENTS R1 |
| Không đổi luật FeedbackTiming/ResultRelease, không chính sách backend mới | AGENTS R33, R34 · `ACTIVITY_ENGINE` §8.1 |
| Lời tử tế, không đỏ gắt, không từ phạt | AGENTS R19 · NFR-SAFE-003 |
| Không hardcode màu/cỡ; file sinh không sửa tay | AGENTS R32 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Câu chọn một/đúng-sai ở practice cần **1** thao tác để có phản hồi (trước: 2).
- **SC-002**: Ma trận 4 khối × 3 cửa sổ × mọi màn của F098-AC-09 pass; ảnh có cho cả 4 khối.
- **SC-003**: GdUnit4 toàn bộ xanh; mọi lint xanh; export Web thành công.

## Ngoài phạm vi

- Đổi luật FeedbackTiming/ResultRelease, luật chấm, số lượt thử lại (`attempts_per_question`) hay contract hoạt động (R34).
- Chạm-là-trả-lời cho câu chèn video/bài nghe/mô phỏng, mini-game F073, đề thi F064, trận đấu (F109).
- Cài đặt "chạm hai lần" trong màn Cài đặt của học sinh hay trong portal (chỉ cấu hình client lượt này).
- Hiệu ứng hạt/âm thanh mới cho kết quả (P-5 giữ như F097).

## Assumptions

- Server trả `feedback_timing`/`result_release` hiệu lực trong `Attempt` (contract hiện có).
- Font đóng gói phủ mọi chuỗi mới; ảnh render bằng GPU (Intel Iris Xe, Compatibility); so sánh bằng mắt, không so pixel.

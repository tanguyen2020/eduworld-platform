# Feature Specification: F097 — Student UX pass 2: màn làm bài như một trò chơi, tạo nhân vật bằng hình, marker không che nhân vật

**Feature Branch**: `097-student-ux-pass-2` · **Created**: 2026-11-26 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`, `design/icons/game`) · **Mốc**: M3 · **Phụ thuộc**: F063, F065, F066, F070, F071, F094, F096
**Input**: Owner (lệnh thường trực) "thiết kế game thật dễ dùng và thu hút học sinh" (khối 1–9); orchestrator 2026-11-26 duyệt ảnh `activity_feedback-after-1280x720-vi-g1_3.png` của F096: màn làm bài vẫn như một biểu mẫu (ô trắng chữ nhỏ, đề hiện hai lần — F096-3, không hình/linh vật/cảm giác tiến độ, dày đặc với lớp 1–3). Lượt 2 làm lại màn học tập (dùng nhiều nhất), S02 Tạo nhân vật (F096-1) và marker che nhân vật ở spawn Hub (F096-4). Chỉ đổi cách hiển thị phía client; không đổi hợp đồng hoạt động, không chấm điểm ở client (AGENTS R5, R6, R34). FR nguồn: FR-GAME-001, FR-GAME-010, FR-RWD-002; NFR-A11Y-001, NFR-I18N-001, NFR-I18N-003, NFR-COMP-004, NFR-MAINT-007, NFR-SAFE-003.

## Clarifications

### Session 2026-11-26

Owner có lệnh thường trực "chốt hết": điểm thiếu được chốt bằng mặc định an toàn, thân thiện với trẻ nhất và ghi [Confirmed].

- Q: Phạm vi lượt này? → A: [Confirmed — giao việc orchestrator 2026-11-26] (1) mọi renderer học tập (quiz, bài tập tương tác, kéo thả, điền khuyết, nhập số, ghép cặp, sắp xếp; khung chung dùng cho video/bài nghe/mô phỏng/mini-game ở phần câu hỏi), (2) S02 (F096-1), (3) F096-4, (4) nếu kịp: Nhật ký nhiệm vụ và Tủ đồ cùng phong cách. Túi đồ/shop/bộ sưu tập chưa có trong client (F069) → follow-up.
- Q: Phụ đề `VoiceCaption` lặp đề (F096-3)? → A: [Confirmed] Đề bài hiện **một lần** trong thẻ câu hỏi của renderer; khi renderer vẽ đề, thẻ đề **chính là phụ đề** (STUDENT_GAME_UX_LEARNING §7: chữ to theo cỡ đề bài của khối). `VoiceOverPlayer` không còn hiện khung phụ đề lặp lại (kể cả khi tắt tiếng hay không có file lồng tiếng); nó chỉ phát lồng tiếng (tự phát `g1_3`, phím `R`, nút "Đọc to" trên thẻ đề). Khung phụ đề riêng chỉ còn cho nội dung không vẽ đề (vd. lời hướng dẫn S02).
- Q: Nút "Đọc to" hiện khi nào? → A: [Confirmed] Trên thẻ câu hỏi, khi câu có bản ghi lồng tiếng (`stem.audio_ref` hoặc media `audio`). Không TTS lượt này (OQ-23 giữ: không TTS phía server; TTS phía client là follow-up).
- Q: Thẻ đáp án trông thế nào mà không chỉ dựa vào màu? → A: [Confirmed] Sáu bộ màu (xanh dương, cam, xanh lá, tím, hồng, xanh ngọc) nền nhạt + viền màu nhấn + viền đáy dày (kiểu kẹo, ADR-0103); huy hiệu số bên trái = phím `1–6`; đang chọn = nền màu nhấn chữ trắng **và** huy hiệu thành dấu ✓ (hình, không chỉ màu). Sau khi chấm, thẻ không bị làm xám: thẻ đã chọn giữ dáng "đã chọn", chỉ không nhận chạm/nhấp/focus nữa.
- Q: Cỡ thẻ và chữ theo khối? → A: [Confirmed — LEARNING_EXPERIENCE §8, DESIGN_SYSTEM §5.2] Chữ đề và thẻ = cỡ chữ đề bài tối thiểu của khối (`g1_3` 32, `g4_5` 28, `g6_7` 24, `g8_9` 22 px; token `density.<band>.font.stem`); chiều cao thẻ tối thiểu 88/80/72/64 px (`density.<band>.answer.min_height`, luôn ≥ vùng chạm của khối, `g1_3` ≥ 64 dp).
- Q: Tiến độ thay cho "1/11"? → A: [Confirmed — STUDENT_GAME_UX_LEARNING §3.7] Đường tiến độ ở header: câu đã qua = ngôi sao đặc, câu hiện tại = vòng lớn, câu còn lại = chấm rỗng (ba hình khác nhau); > 15 câu thành thanh bo tròn có ngôi sao ở vị trí hiện tại; vẫn giữ số "1/11" bên cạnh; 1 câu thì không vẽ đường. Không đếm ngược, không so sánh.
- Q: Mây xuất hiện thế nào? → A: [Confirmed — WORLD_DESIGN §3, STUDENT_GAME_UX_LEARNING §3.3] Mây (bản nhỏ, 120 px) ở góc dưới trái, bong bóng lời bên phải chứa: câu động viên khi chưa có gì ("Em làm được mà!" / có gợi ý: "… Cần giúp thì bấm Gợi ý nhé."), nút Gợi ý (Mây nhắc), kết quả chấm, lỗi. Nét mặt: bình thường; vui (mắt cười) khi đúng; động viên (mày nhướng, cười hiền) khi chưa đúng — không bao giờ buồn/giận.
- Q: Phản hồi sai thế nào để không có cảm giác bị phạt? → A: [Confirmed — NFR-SAFE-003, AGENTS R19] Bảng kết quả nền nhạt có viền: đúng = xanh lá + icon ✓ trong đĩa xanh + "Đúng rồi! Em giỏi quá!"; chưa đúng = cam ấm (không đỏ gắt) + icon ✗ mềm trong đĩa cam + "Chưa đúng rồi. Không sao, mình cùng xem lại nhé!" (`en`: "That's right! Great job!" / "Not quite. That's okay, let's look at it together!"). Không từ "sai rồi", "trừ", "phạt". Dải kết quả dùng chung cho mini-game (cùng biến thể Theme).
- Q: Nút thoát / gợi ý / chính có nhất quán? → A: [Confirmed] Thoát luôn trên trái (icon cửa + "Thoát"), Gợi ý luôn trong bong bóng của Mây (icon bóng đèn), nút chính luôn dưới phải với icon theo hành động (✓ Kiểm tra, → Tiếp/Hoàn thành); hộp tạm dừng có Mây, "Làm tiếp" (→) và "Tạm dừng" (⏸). Sau khi chấm, nút Gợi ý ẩn đến câu sau (bong bóng dành cho kết quả). Câu "Chọn một đáp án" chỉ còn là hướng dẫn của câu (không lặp ở thanh dưới).
- Q: S02 bố cục và ảnh mẫu (F096-1)? → A: [Confirmed] Khung xem trước lớn ở giữa trái (≈ 55% bề ngang, trên nền trời S01, camera gần để nhân vật chibi chiếm gần hết khung), quay chậm như bàn xoay ở **mọi** bước (0,3 rad/s; bước xem lại 0,6 rad/s), dừng 3 s sau khi học sinh tự xoay; tắt khi "Giảm chuyển động". Mẫu nhân vật là **ô hình** (`PictureTile`): chân dung chibi vẽ bằng vector theo màu da/tóc/trang phục của mẫu (token `avatar.*`, tint item) + tên bên dưới; ô đang chọn có dấu ✓. Màu da/tóc là chấm tròn có viền mực, có ✓ khi chọn (cũng dùng ở Tủ đồ). Không ảnh tải về (ảnh `thumbnail_asset_id` của server là follow-up).
- Q: Marker che nhân vật ở spawn Hub (F096-4)? → A: [Confirmed — owner "chốt hết"] Ẩn marker thế giới (đá Tia Sáng của `WorldAnchor`, `QuestMarker3D`) khi **gần camera hơn `marker_near_fade_m` = 5 m và gần camera hơn nhân vật** (tức đứng giữa camera và nhân vật); hiện lại khi xa hơn ngưỡng + 0,5 m (trễ chống nhấp nháy). Quy tắc ở WORLD_RENDERING §5.1, tham số `eduworld/world/marker_near_fade_m` (CONFIG_DEFAULTS). Không dời seed, `Area3D` tương tác vẫn hoạt động. Ẩn (không mờ dần) vì shader toon dùng chung không có alpha theo instance.
- Q: Ảnh minh họa cho câu hỏi? → A: [Confirmed] Ảnh của câu đến từ nội dung (R2); client không tự thêm hình cho từng câu. Cảm giác "game" đến từ nền trời, bảng bo tròn, thẻ màu, Mây, đường tiến độ.
- Q: Bảng kiểm "trước" của màn học tập? → A: chụp bằng `tools/capture_screens.gd` (Compatibility) ngay trên main b307207f, thêm màn mới vào công cụ (chọn đáp án, sai, gợi ý, từng dạng câu).

## Bảng kiểm UX

Ảnh `screenshots/<màn>-{before,after}-<cỡ>-<ngôn ngữ>-<khối>.png`, renderer Compatibility (= bản Web), fixture + fake, không backend. Điểm 1–5 theo **R**õ ràng · **D**ễ dùng · **T**hu hút · **P**hong cách chibi anime cartoon (ADR-0103) · **A** trợ năng.

| Màn | Trước | Vấn đề chính (trước) | Sau | Đã sửa |
|---|---|---|---|---|
| S16 Làm bài — chọn một/nhiều (`activity`, `activity_selected`, `activity_multiple`) | 3·3·1·1·3 | Ô trắng chữ nhỏ "1. 9 quả"; đề hiện hai lần (phụ đề + đề); "Chọn một đáp án" hai lần; chỉ "1/11"; khối lớn và nhỏ trông như nhau | 5·5·4·4·5 | Bảng trên nền trời; thẻ đề trắng chữ Baloo 2 cỡ đề của khối + "Đọc to"; 4 thẻ màu bo tròn có huy hiệu số, chọn = nền nhấn + ✓; đường tiến độ sao/vòng/chấm + "1/11"; Mây + bong bóng động viên; nút Kiểm tra có icon |
| S16 Kết quả câu (`activity_feedback`, `activity_incorrect`) | 3·3·1·1·3 | Dải đỏ đậm "Chưa đúng…" như bị phạt; giải thích trong hộp viền xanh lớn; thẻ bị làm xám mất lựa chọn | 5·4·4·4·5 | Bong bóng của Mây: bảng nhạt xanh/cam có icon đĩa ✓/✗ + lời tử tế; Mây vui/động viên; thẻ đã chọn vẫn hiện; "Xem ví dụ" cạnh giải thích; nút → Tiếp |
| S16 Gợi ý (`activity_hint`) | 3·3·2·2·3 | Nút tím rời rạc, chữ gợi ý không gắn với Mây | 5·5·4·4·4 | Gợi ý nằm trong bong bóng của Mây ("Mây nhắc em một chút nhé.") |
| S16 Đúng/sai, ghép cặp, sắp xếp, kéo thả, điền khuyết, nhập số | 3·3·1·1·3 | Nút xanh đặc giống nhau, thẻ kéo nhỏ, nút Lên/Xuống chỉ chữ | 5·4·4·4·4 | Thẻ màu có huy hiệu (thứ tự/phím), vùng thả trắng bo tròn, Lên/Xuống có icon mũi tên, bàn phím số phím lớn viền |
| S02 Tạo nhân vật (`character`) | 3·3·2·3·3 | Nhân vật nhỏ giữa khoảng trống; mẫu chỉ có chữ; ô màu vuông | 5·5·5·5·4 | Sân khấu lớn trên nền trời, nhân vật đầy khung, quay như bàn xoay; mẫu là ô hình chân dung + tên + ✓; ô màu tròn có ✓; nút xoay/‹ › có icon |
| S03 Hub spawn (`hub`) | 4·4·4·4·4 | Đá Tia Sáng của biển chào che chân nhân vật | 5·4·4·4·4 | Marker giữa camera và nhân vật được ẩn; nhân vật thấy trọn |
| S05 Nhật ký (`journal`) | 4·4·3·4·4 | Tab xanh đặc giống nút | 4·4·4·4·5 | Tab là chip (đang chọn = nền nhấn + ✓), Đóng/Bắt đầu có icon |

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Làm bài như chơi một trò chơi (Priority: P1)

Học sinh lớp 1 mở một bài: thấy một bảng vui trên nền trời, đề bài to trong thẻ trắng có nút "Đọc to", bốn thẻ đáp án to nhiều màu có số; chọn một thẻ thì thẻ đổi màu và có dấu ✓; Mây ở góc nói "Em làm được mà!"; đường sao ở trên cho biết đang ở câu nào.

**Why this priority**: Màn học tập là màn học sinh dùng nhiều nhất; orchestrator đánh giá vẫn "như biểu mẫu".

**Independent Test**: `tests/scene/learning/test_activity_player_ux.gd`, `test_question_renderers.gd`, `test_voice_over.gd`; ảnh `activity*-after-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F097-AC-01 | Mỗi lựa chọn của câu chọn một/nhiều, đúng/sai, ghép cặp (cột trái), sắp xếp, thẻ kéo thả và nhãn ảnh PHẢI là một thẻ đáp án Theme `AnswerTile1…6` (6 bộ màu luân phiên) có huy hiệu số bên trái bằng phím `1–6` (không tiền tố "1." trong chữ); thẻ đang chọn PHẢI có nền màu nhấn và huy hiệu đổi thành dấu ✓ | `STUDENT_GAME_UX_LEARNING` §4.1, §4.3 · `DESIGN_SYSTEM` §5.1 · NFR-A11Y-001 · ADR-0103 | TC-UI-001 |
| F097-AC-02 | Với mỗi khối, thẻ đáp án PHẢI cao ít nhất `density.<band>.answer.min_height` (88/80/72/64 px, ≥ vùng chạm của khối, `g1_3` ≥ 64 dp) và chữ thẻ cùng chữ đề PHẢI bằng cỡ đề bài của khối `density.<band>.font.stem` (32/28/24/22 px); kiểm bằng test scene ở cả 4 khối | `LEARNING_EXPERIENCE` §8 · `DESIGN_SYSTEM` §5.2 · NFR-A11Y-001 | — |
| F097-AC-03 | Mỗi câu hỏi PHẢI có thẻ câu hỏi (`QuestionCard`) chứa đề bài (`StemLabel`) và, khi câu có bản ghi lồng tiếng, nút "Đọc to" có icon loa; bấm nút PHẢI phát lại lồng tiếng như phím `R` | `STUDENT_GAME_UX_LEARNING` §4.1, §7 · F063-AC-04 | — |
| F097-AC-04 | TRONG KHI renderer hiện đề bài, đề bài PHẢI xuất hiện đúng một lần trên màn (không khung phụ đề lặp lại đề) ở mọi khối, cả khi tắt tiếng hay không có bản ghi; lồng tiếng vẫn tự phát với `g1_3` | F096-3 · `STUDENT_GAME_UX_LEARNING` §7 | — |
| F097-AC-05 | Header PHẢI có đường tiến độ: câu đã qua = ngôi sao đặc, câu hiện tại = vòng lớn, câu còn lại = chấm rỗng (hình khác nhau, không chỉ màu), > 15 câu thành thanh; vẫn hiện "n/N"; không đếm ngược hay so sánh | `STUDENT_GAME_UX_LEARNING` §3.3, §3.7 · NFR-SAFE-003 | — |

---

### User Story 2 — Phản hồi tử tế, có Mây đồng hành (Priority: P1)

Sau "Kiểm tra", Mây nói trong bong bóng: đúng thì "Đúng rồi! Em giỏi quá!" với dấu ✓ xanh và Mây cười; chưa đúng thì "Chưa đúng rồi. Không sao, mình cùng xem lại nhé!" với dấu ✗ mềm trong đĩa cam, lời giải thích theo phương án em chọn, Mây động viên — không màu đỏ gắt, không từ phạt.

**Independent Test**: `test_activity_player_ux.gd`, `test_feedback_strip.gd`; ảnh `activity_feedback-*`, `activity_incorrect-*`, `activity_hint-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F097-AC-06 | KHI server trả kết quả chấm có `is_correct`, HỆ THỐNG PHẢI hiện bảng kết quả gồm icon (✓ trong đĩa xanh / ✗ trong đĩa cam) VÀ chữ; KHÔNG ĐƯỢC báo đúng/sai chỉ bằng màu; KHI FeedbackTiming ẩn kết quả, HỆ THỐNG PHẢI chỉ báo "đã lưu" như F063-AC-14 | NFR-A11Y-001 · F063-AC-14 · `STUDENT_GAME_UX_LEARNING` §4.8 | — |
| F097-AC-07 | KHI câu trả lời chưa đúng, bảng kết quả PHẢI dùng nền cam ấm nhạt (không phải màu `color.feedback.incorrect` đỏ) và lời tử tế "Chưa đúng rồi. Không sao, mình cùng xem lại nhé!" / "Not quite. That's okay, let's look at it together!"; chuỗi KHÔNG ĐƯỢC chứa từ phạt ("sai rồi", "trừ", "phạt", "wrong", "penalty", "lose") | NFR-SAFE-003 · AGENTS R19 · `LEARNING_EXPERIENCE` §4 | — |
| F097-AC-08 | TRONG KHI làm bài, HỆ THỐNG PHẢI hiện Mây ở góc dưới trái với bong bóng lời chứa câu động viên (khi không có gì khác), nút Gợi ý, kết quả chấm và lỗi; nét mặt Mây PHẢI là vui khi đúng, động viên khi chưa đúng, bình thường ở câu mới | `STUDENT_GAME_UX_LEARNING` §3.3, §4.7 · `WORLD_DESIGN` §3 | — |
| F097-AC-10 | Nút Thoát (trên trái), Gợi ý (trong bong bóng) và nút chính (dưới phải: Kiểm tra / Tiếp / Hoàn thành) PHẢI có icon kèm chữ ở mọi câu; mọi nút hiển thị trên màn làm bài PHẢI cao ≥ chiều cao nút tối thiểu của khối ở cả 4 khối | `STUDENT_GAME_UX_LEARNING` §3.6, §3.7 · NFR-A11Y-001 | — |

---

### User Story 3 — Tạo nhân vật bằng hình (Priority: P1)

Học sinh lớp 1 chọn nhân vật bằng cách nhìn chân dung (không cần đọc tên), thấy nhân vật của mình to ở giữa và quay chậm, xoay được bằng tay hoặc nút.

**Independent Test**: `tests/scene/character/test_character_creation_ux.gd` + `test_character_creation_scene.gd` (F070 giữ nguyên); ảnh `character-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F097-AC-09 | S02 PHẢI hiện khung xem trước 3D lớn hơn cột lựa chọn và cao hơn nửa màn, trên nền trời, tự quay chậm ở mọi bước (dừng một lúc sau khi học sinh tự xoay, đứng yên khi "Giảm chuyển động"); mẫu nhân vật PHẢI là ô hình (chân dung theo màu da/tóc/trang phục của mẫu + tên, ✓ khi chọn), màu da/tóc là chấm tròn có ✓ khi chọn; mọi lựa chọn ≥ vùng chạm của khối; hành vi F070 (bước, gợi ý tên, xác nhận) giữ nguyên | F096-1 · `STUDENT_GAME_UX_CORE` S02 · F070-AC-05, F070-AC-06 | — |

---

### User Story 4 — Marker không che nhân vật (Priority: P2)

Vừa vào Quảng trường Tia Sáng, học sinh thấy trọn nhân vật của mình; đá Tia Sáng của biển chào không chắn giữa camera và nhân vật.

**Independent Test**: `tests/unit/world/test_marker_fader.gd`; ảnh `hub-*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F097-AC-11 | NẾU một marker thế giới (nhóm `camera_fade`: đá Tia Sáng `WorldAnchor`, `QuestMarker3D`) gần camera hơn `marker_near_fade_m` (mặc định 5 m, `eduworld/world/marker_near_fade_m`) VÀ gần camera hơn nhân vật, THÌ HỆ THỐNG PHẢI ẩn marker đó; KHI marker ra xa hơn ngưỡng + 0,5 m hoặc xa hơn nhân vật, HỆ THỐNG PHẢI hiện lại; vị trí seed và vùng tương tác KHÔNG ĐƯỢC đổi | F096-4 · `WORLD_RENDERING` §5.1 · owner "chốt hết" | — |

---

### User Story 5 — Bằng chứng duyệt (Priority: P1)

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F097-AC-12 | HỆ THỐNG PHẢI có ảnh trước/sau cho mỗi màn đã sửa ở 1280×720 và 1024×600 (thêm 720×1280 cho màn làm bài và Hub), `vi` và `en`, khối `g1_3` và `g6_7`, trên renderer Compatibility (`tools/capture_screens.gd`, chạy từng màn một tiến trình) | `DESIGN_SYSTEM` §8, §9 · NFR-COMP-004 | TC-UI-007 |
| F097-AC-13 | Mọi chuỗi mới PHẢI qua `.po` `vi`/`en`; mọi ký tự PHẢI có trong font đóng gói (`test_font_coverage.gd` xanh); `extract_pot --check`, `pseudo_locale --check` PHẢI pass; mọi màu/cỡ PHẢI từ token (`lint_theme_overrides`, `make design-tokens-check` xanh) | NFR-I18N-001 · NFR-COMP-004 · AGENTS R32 · F096-AC-15 | TC-UI-003 |

### Edge Cases

- NẾU câu không có bản ghi lồng tiếng, THÌ không có nút "Đọc to"; đề vẫn hiện một lần (F097-AC-03, AC-04).
- NẾU "Giảm chuyển động" bật, THÌ xem trước S02 đứng yên (xoay bằng nút/kéo vẫn được) (F097-AC-09).
- NẾU bài có > 15 câu, THÌ đường tiến độ thành thanh (F097-AC-05); bài 1 câu không vẽ đường.
- NẾU bảng kết quả + giải thích dài, THÌ vùng thẻ cuộn được; nút chính luôn thấy (F097-AC-10).
- NẾU camera phóng gần (3 m), THÌ marker cạnh hoặc sau nhân vật vẫn hiện (chỉ ẩn marker đứng giữa camera và nhân vật) (F097-AC-11).
- NẾU câu là mini-game (F073), THÌ dải kết quả mini-game dùng cùng bảng nền nhạt + chữ chính (F097-AC-06, AC-07).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-001 | Phong cách chibi cartoon cho màn học tập và S02 (thẻ kẹo, Mây, nền trời) | `DESIGN_SYSTEM` §5 · ADR-0103 |
| FR-GAME-010 | UI theo khối: cỡ đề/thẻ, vùng chạm, icon + chữ, chọn bằng hình cho lớp 1–3 | `STUDENT_GAME_UX_LEARNING` §3.7, §4 · `STUDENT_GAME_UX_CORE` S02 |
| FR-RWD-002 | Phản hồi không dark pattern, không áp lực | `LEARNING_EXPERIENCE` §4 · `CHILD_SAFETY` |

### Key Entities

- **Biến thể Theme mới** (`tokens@0.14.0`): `AnswerTile1…6`, `QuestionCard`, `StemLabel`, `InstructionLabel`, `DropZone`, `ActivityBoard`, `PictureTile`, `AnswerGrid`/`AnswerBox`/`AnswerFlow`; `FeedbackCorrectPanel`/`FeedbackIncorrectPanel`/`FeedbackLabel` đổi sang nền nhạt.
- **Component mới**: `AnswerBadge`, `ProgressTrail`, `AvatarThumb`, `ColorDot` (`scenes/ui/components/`); `CloudyMascot` thêm `compact`, `side_tail`, `mood`.
- **`MarkerFader`** (`scripts/world/marker_fader.gd`) + nhóm `camera_fade`.

### Tham số

| Tham số | Dùng để |
|---|---|
| `density.<band>.font.stem` 32/28/24/22 px (`LEARNING_EXPERIENCE` §8) | F097-AC-02 |
| `density.<band>.answer.min_height` 88/80/72/64 px (`DESIGN_SYSTEM` §5.2) | F097-AC-02 |
| `marker_near_fade_m` = 5 m, trễ 0,5 m (`WORLD_RENDERING` §5.1, `CONFIG_DEFAULTS`) | F097-AC-11 |
| Bàn xoay S02 0,3 / 0,6 rad/s, dừng 3 s sau thao tác tay (`STUDENT_GAME_UX_CORE` S02) | F097-AC-09 |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chỉ hiển thị; không chấm điểm, không đáp án ở client; không đổi contract hoạt động | AGENTS R1, R5, R6, R34 |
| Không hardcode màu/cỡ; file sinh không sửa tay; icon qua pipeline | AGENTS R32 · NFR-MAINT-007 |
| Mọi chuỗi qua `.po`; ký tự có trong font đóng gói | NFR-I18N-001 · NFR-COMP-004 |
| Không đếm ngược, chuỗi ngày, so sánh, thông điệp mất mát | AGENTS R19 · NFR-SAFE-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Mọi màn ở bảng kiểm đạt ≥ 4 điểm ở R, D, A và ≥ 4 ở T sau khi sửa; màn làm bài tăng ≥ 2 điểm ở T và P.
- **SC-002**: Đề bài xuất hiện đúng 1 lần trên mọi ảnh "sau" của màn làm bài.
- **SC-003**: GdUnit4 toàn bộ xanh; mọi lint xanh; export Web thành công.

## Ngoài phạm vi

- Đổi luật chấm, số lượt thử, phản hồi theo phương án hay contract hoạt động (R34).
- TTS phía client khi thiếu file lồng tiếng; ảnh thumbnail mẫu từ server (`thumbnail_asset_id`); ảnh minh họa câu hỏi do client tự thêm (R2).
- Làm lại video/bài nghe/mô phỏng phần trình phát (chỉ phần câu hỏi dùng thẻ mới); túi đồ/shop/bộ sưu tập (F069), party, chat, đấu trường.
- Mờ dần marker bằng alpha (shader toon dùng chung không có alpha theo instance).

## Assumptions

- Font đóng gói (Baloo 2, Be Vietnam Pro) phủ đủ mọi chuỗi mới; số trên huy hiệu vẽ bằng font Theme.
- Ảnh render bằng GPU (Intel Iris Xe, Compatibility); so sánh trực quan bằng mắt, không so pixel.

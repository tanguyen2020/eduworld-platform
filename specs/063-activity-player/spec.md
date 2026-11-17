# Feature Specification: F063 — Activity player: renderer registry, renderer 11 loại câu hỏi, gợi ý, kết quả, công thức SVG

**Feature Branch**: `063-activity-player` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `godot/` · **Mốc**: M-C2 · **Phụ thuộc**: F030, F032, F062 (phần F032 — gợi ý, gợi ý ôn tập — dùng Prism mock có nhãn tới khi Done, [CODE_PLAN §5 (\*)](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md))
**Input**: Khung activity player chung cho mọi `activity_type`, renderer `quiz` và `interactive_exercise` với 11 dạng câu hỏi, gợi ý nhiều cấp, phản hồi sau chấm, màn kết quả, công thức SVG. FR nguồn: FR-EXR-003, FR-EXR-004, FR-EXR-012, FR-EXR-015, FR-CNT-017, FR-QB-007, FR-GAME-018.

## Clarifications

### Session 2026-11-16

- Q: Client biết câu có gợi ý bằng cách nào? → A: `QuestionRender` cần trường `hint_count` (GODOT_ARCHITECTURE §6.2) — chưa có trong spec, thêm ở delta contract; trong lúc chờ, nút Gợi ý hiện khi mode cho phép và ẩn sau `HINTS_EXHAUSTED`/`HINTS_NOT_ALLOWED`.
- Q: `min_client_version` theo `activity_type` lấy ở đâu? → A: `ActivityRender.min_client_version` (có sẵn) khi start; manifest theo loại cần thêm (`ContentManifest.activity_min_client_versions`) để chặn trước khi start.
- Q: Phần thưởng game trên màn kết quả khi chưa có module game progression (M2)? → A: Hiện `AttemptResult.rewards` nếu có; vùng "Phần thưởng" ẩn khi rỗng. Không tự tính XP.
- Q: SVG lỗi thì hiển thị gì? → A: `MathAssetRef` không có `latex`; dùng `speech_text` làm văn bản thay thế.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Làm hoạt động câu hỏi trong khung activity player (Priority: P1)

Học sinh mở một hoạt động (bài giao, luyện tập) → khung activity player mở, thế giới 3D tạm dừng, câu hỏi hiện theo 11 dạng, học sinh trả lời, nộp, quay về cảnh game; thoát giữa chừng thì làm tiếp được (US-EXR-002).

**Why this priority**: Là lõi vòng Content Loop M-C2: học sinh làm trong Godot → server chấm.

**Independent Test**: `hs.an.3a` mở activity `quiz` có đủ 11 dạng câu (seed), trả lời, nộp; thoát ở câu 3, mở lại → tiếp tục câu 3 với câu trả lời đã lưu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F063-AC-01 | KHI nhận intent `activity_host`, `LearningService` PHẢI gọi `startActivityAttempt` với `Idempotency-Key`, body `{mode, context_type, context_id, recommendation_id?}` và `ActivityHost` PHẢI chọn renderer theo `activity_type` qua `ActivityRendererRegistry` | FR-EXR-004 · ADR-0059 · [GODOT_ARCHITECTURE §6.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-LRN-002 |
| F063-AC-02 | game client PHẢI render đủ 11 `QuestionType` (`single_choice`, `multiple_choice`, `true_false`, `fill_in_blank`, `short_answer`, `numeric`, `matching`, `ordering`, `drag_drop`, `image_based`, `interactive`) trên PC và mobile; ghép cặp và kéo thả PHẢI có cách chạm-chạm trên mobile | FR-QB-007 · [STUDENT_GAME_UX_LEARNING §4.3](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-LRN-016 |
| F063-AC-03 | Mỗi renderer câu hỏi PHẢI chỉ phát `response_ready(position, response)` đúng shape `QuestionResponse` của loại, không chấm, không giữ đáp án; client PHẢI không gửi `score`/`is_correct` | FR-EXR-003 · ADR-0016 · AGENTS R5, R6 | TC-LRN-007 |
| F063-AC-04 | game client PHẢI hiển thị tiếng Việt có dấu, ảnh và phát audio item (`media.kind = audio`); `g1_3` PHẢI tự phát lồng tiếng đề khi có asset và luôn hiện phụ đề + icon khi tắt tiếng | FR-QB-007 · [STUDENT_GAME_UX_LEARNING §7](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | — |
| F063-AC-05 | KHI học sinh bấm "Kiểm tra", `LearningService` PHẢI gọi `submitActivityResponse` `{kind: question, position, response, client_elapsed_ms}` với `Idempotency-Key` mới cho mỗi lần trả lời và giữ nguyên key khi retry | FR-EXR-003 · AGENTS R10 · [GAME_NETWORKING §2.3](../../docs/08-game-client/GAME_NETWORKING.md) | TC-LRN-002 |
| F063-AC-06 | NẾU mất mạng khi gửi câu trả lời, THÌ game client PHẢI giữ câu trả lời trong hàng đợi RAM `PendingWrites`, hiện banner `ui.activity_player.offline` và gửi lại cùng `Idempotency-Key` khi có mạng; PHẢI không ghi render payload/câu trả lời xuống đĩa | FR-EXR-004 · [GODOT_ARCHITECTURE §8.2, §9](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-LRN-009 |
| F063-AC-07 | KHI học sinh thoát giữa chừng, game client PHẢI gọi `pauseActivityAttempt`; KHI mở lại trong `attempt_resume_window`, PHẢI dựng lại từ `getActivityAttempt` (items, `saved_responses`, `hints_revealed`) đúng câu đang làm và đúng version đã ghim | FR-EXR-004 · AC-EXR-006 · [ACTIVITY_ENGINE §6.3](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-LRN-009 |
| F063-AC-08 | KHI học sinh hoàn thành câu cuối, game client PHẢI gọi `submitActivityAttempt` với `Idempotency-Key`; NẾU nhận `409 ATTEMPT_ALREADY_SUBMITTED`, THÌ PHẢI coi như thành công và tải kết quả | AC-EXR-003 · [GODOT_ARCHITECTURE §9](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-LRN-008 |
| F063-AC-09 | KHI mở activity player toàn màn, game client PHẢI gọi `QualityManager.suspend_world("activity")`; KHI đóng, PHẢI `resume_world` và quay về đúng cảnh game trước đó với phản hồi NPC/trở lại quest | FR-GAME-018 · [STUDENT_GAME_UX_LEARNING §2.1](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-PERF-020 |
| F063-AC-10 | NẾU server trả `410 ATTEMPT_EXPIRED`, `403 ASSIGNMENT_NOT_OPEN`, `409 ASSIGNMENT_MAX_ATTEMPTS`, `409 ATTEMPT_NOT_IN_PROGRESS` hoặc `422 INVALID_RESPONSE`, THÌ game client PHẢI hiện thông điệp `ui.error.<CODE>` (mã chi tiết chỉ ở màn "Chi tiết") và không mất câu trả lời đã nhập | [ACTIVITY_ENGINE §9](../../docs/02-learning/ACTIVITY_ENGINE.md) · [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md) | TC-LRN-008 |
| F063-AC-11 | NẾU token không phải học sinh (`403 GAME_ACCESS_STUDENT_ONLY`) hoặc attempt thuộc học sinh/tenant khác (`404`), THÌ game client PHẢI không hiển thị dữ liệu nào của attempt và về trạng thái lỗi/Login theo F061 | AGENTS R7, R8, R23 · ADR-0031 · ADR-0065 | TC-AUTH-014, TC-TEN-001 |
| F063-AC-12 | Ô nhập `short_answer`/`fill_in_blank`/`numeric` PHẢI giới hạn độ dài theo giới hạn trong render payload và PHẢI không gửi chuỗi vượt giới hạn | ADR-0041 · [STUDENT_GAME_UX_LEARNING §4.3](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-TEN-024 |
| F063-AC-13 | KHI `translation_missing = true`, game client PHẢI hiển thị bản gốc kèm nhãn `ui.learning.no_translation`; câu đang làm PHẢI giữ ngôn ngữ khi đổi locale | NFR-I18N-002 · ADR-0056 | TC-I18N-003, TC-I18N-004 |

---

### User Story 2 — Phản hồi sau chấm và gợi ý nhiều cấp (Priority: P1)

Sau "Kiểm tra", học sinh thấy đúng/sai, giải thích và phản hồi theo phương án đã chọn khi FeedbackTiming cho phép; bấm "Gợi ý" để mở từng cấp (US-EXR-003).

**Why this priority**: Phản hồi thân thiện và gợi ý là giá trị sư phạm chính của practice/quest.

**Independent Test**: Practice với câu có 2 cấp gợi ý: bấm Gợi ý 2 lần → hiện cấp 1, 2; lần 3 nút mờ; chọn phương án sai có feedback → dải phản hồi hiện đúng câu feedback.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F063-AC-14 | KHI `ActivityResponseResult` có `is_correct`/`explanation`/`feedback`, game client PHẢI hiện icon ✓/✗ + chữ, giải thích và phản hồi theo phương án ở dải phản hồi; KHI các trường này `null`, PHẢI không hiện đúng/sai | FR-EXR-003 · AC-EXR-007 · [LEARNING_EXPERIENCE §4.1](../../docs/02-learning/LEARNING_EXPERIENCE.md) | TC-LRN-012 |
| F063-AC-15 | KHI học sinh bấm Gợi ý, `LearningService.request_next_hint(attempt_id, position)` PHẢI gọi `revealNextHint` với `Idempotency-Key` và `HintPanel` PHẢI hiện `hint.text` + công thức kèm "Gợi ý {n}/{total}" | FR-EXR-012 · [ACTIVITY_ENGINE §8.3](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-LRN-011 |
| F063-AC-16 | NẾU server trả `409 HINTS_EXHAUSTED`, THÌ nút Gợi ý PHẢI mờ với `ui.activity_player.hints_exhausted`; NẾU `403 HINTS_NOT_ALLOWED` hoặc mode ∈ {`exam`, `diagnostic`, `competition`}, THÌ nút PHẢI ẩn; client PHẢI không tự đếm `hints_used` | FR-EXR-012 · [STUDENT_GAME_UX_LEARNING §4.7](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-LRN-011 |
| F063-AC-17 | KHI `feedback.worked_example_ref` có giá trị, game client PHẢI hiện nút `ui.feedback.worked_example` mở overlay chỉ đọc từ `getLesson` | FR-EXR-003 · [STUDENT_GAME_UX_LEARNING §4.8](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | — |

---

### User Story 3 — Hiển thị công thức Toán/Lý/Hóa (Priority: P1)

Đề, lựa chọn, gợi ý, giải thích có công thức hiển thị bằng SVG render sẵn, đúng màu Theme, đọc to được (FR-CNT-017).

**Why this priority**: Exit criterion M-C2 yêu cầu câu hỏi có công thức hiển thị đúng trong Godot.

**Independent Test**: Câu có phân số inline và công thức block trong đề + lựa chọn: hiển thị sắc nét ở 2 DPI, đúng màu Theme, đọc to dùng `speech_text`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F063-AC-18 | game client PHẢI tải SVG theo `math_assets[].svg_url`, kiểm `sha256`, cache bất biến tại `user://cache/math/<sha256>.svg` và rasterize một lần theo DPI + density (cache texture theo `(sha256, scale, màu)`) | FR-CNT-017 · ADR-0055 · [GODOT_ARCHITECTURE §6.3](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-MATH-003 |
| F063-AC-19 | `MathInline` PHẢI chèn công thức inline vào `RichTextLabel` hoặc khối riêng (`display = block`), tô màu theo Theme và PHẢI không có ô nhập công thức trong game | FR-CNT-017 · ADR-0055 | TC-MATH-001 |
| F063-AC-20 | NẾU SVG tải lỗi, sai `sha256` hoặc vượt `math_svg_max`, THÌ game client PHẢI hiển thị `speech_text` thay thế và gửi telemetry, không crash | FR-CNT-017 · ADR-0055 | — |

---

### User Story 4 — Màn kết quả học tập (Priority: P2)

Sau khi nộp, học sinh thấy kết quả học tập (đúng/tổng, LO cần ôn, mức mastery theo khối, gợi ý của Mây) tách biệt với phần thưởng game; bài chưa công bố kết quả hiện thông điệp chờ.

**Why this priority**: Hoàn tất vòng phản hồi; phụ thuộc review/recommendation của F031/F032.

**Independent Test**: Nộp practice 8 câu → màn kết quả có 2 vùng; bài giao `result_release = manual` → chỉ thấy "Kết quả sẽ có khi nhà trường công bố".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F063-AC-21 | KHI attempt `graded` và kết quả được phép xem, game client PHẢI hiện màn kết quả gồm vùng "Kết quả học tập" (từ `getActivityAttemptReview`: điểm, `weak_objectives`, câu sai) và vùng "Phần thưởng trong game" (chỉ từ `AttemptResult.rewards`, ẩn khi rỗng) | FR-EXR-003 · [STUDENT_GAME_UX_LEARNING §10](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-LRN-004 |
| F063-AC-22 | game client PHẢI hiển thị điểm/mastery theo grade band (`g1_3` số câu đúng/tổng + sao, không thang 10) và PHẢI không so sánh với học sinh khác, không có nhận xét giáo viên | [LEARNING_EXPERIENCE §7](../../docs/02-learning/LEARNING_EXPERIENCE.md) · ADR-0072 · AGENTS R16 | — |
| F063-AC-23 | NẾU `result_release` ≠ `immediate` và kết quả chưa công bố, THÌ game client PHẢI hiện `ui.activity_player.results_later` và không gọi lại review cho tới khi học sinh mở lại | FR-EXR-003 · [ACTIVITY_ENGINE §8.1](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-LMS-002 |
| F063-AC-24 | KHI review có `recommendations[]`, game client PHẢI hiện gợi ý của Mây và nút bắt đầu luyện tập (gọi `startAdHocAttempt` với `recommendation_id`) | FR-EXR-003 · [LEARNING_EXPERIENCE §6.2](../../docs/02-learning/LEARNING_EXPERIENCE.md) | — |

---

### User Story 5 — Phiên bản client và loại hoạt động chưa hỗ trợ (Priority: P2)

Client cũ gặp `activity_type` hoặc dạng câu chưa hỗ trợ thì hiện "Cần cập nhật game" thay vì lỗi (FR-EXR-015).

**Why this priority**: Bảo vệ trải nghiệm khi nội dung đi trước bản build.

**Independent Test**: Build có registry thiếu `simulation`: mở activity `simulation` → màn cập nhật, có telemetry, không crash.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F063-AC-25 | NẾU `activity_type` không có trong `ActivityRendererRegistry`, `ActivityRender.min_client_version` lớn hơn `client_version`, hoặc server trả `422 ACTIVITY_TYPE_NOT_SUPPORTED`, THÌ game client PHẢI hiện `ui.activity_player.update_required` + nút cập nhật, gửi telemetry, không crash | FR-EXR-015 · [ACTIVITY_ENGINE §12](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-ACT-004 |
| F063-AC-26 | NẾU một item có `type` không có trong `QuestionRendererRegistry`, THÌ game client PHẢI hiện thông báo cập nhật trong vùng nội dung của item đó và cho thoát giữ tiến độ | FR-EXR-015 | TC-ACT-004 |

### Edge Cases

- NẾU mở activity khi đã có attempt `in_progress` cùng nguồn, THÌ server trả attempt cũ; client resume (F063-AC-07).
- NẾU đổi thiết bị giữa chừng, THÌ resume trên thiết bị mới qua `getActivityAttempt` (một attempt chỉ active một thiết bị — ADR-0031).
- NẾU tải nội dung > 8 s, THÌ hiện nút Thử lại; lỗi tải → `ui.activity_player.error_load` + Thử lại/Quay lại (F063-AC-10).
- NẾU hết giờ (mode có `deadline_at`), THÌ khóa nhập, hiện `ui.activity_player.time_up`, tải kết quả (server tự nộp).
- NẾU publish version mới khi đang làm, THÌ tiếp tục version đã ghim (TC-ACT-005).
- NẾU học sinh chưa chọn đáp án, THÌ nút chính mờ + `ui.question.select_answer`.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-EXR-003 | Hiển thị phản hồi đúng/sai, giải thích, phản hồi theo phương án theo FeedbackTiming; màn kết quả | [ACTIVITY_ENGINE §8](../../docs/02-learning/ACTIVITY_ENGINE.md), [LEARNING_EXPERIENCE §4](../../docs/02-learning/LEARNING_EXPERIENCE.md) |
| FR-EXR-004 | Resume từ `getActivityAttempt`, pause khi thoát, hàng đợi ghi khi mất mạng | [ACTIVITY_ENGINE §6.3](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-012 | `HintPanel` gọi `revealNextHint` | [ACTIVITY_ENGINE §8.3](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-015 | Kiểm `min_client_version`/registry, màn cần cập nhật | [ACTIVITY_ENGINE §12](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-CNT-017 | `MathInline` + `math_svg_cache` | [GODOT_ARCHITECTURE §6.3](../../docs/08-game-client/GODOT_ARCHITECTURE.md), ADR-0055 |
| FR-QB-007 | Tiếng Việt có dấu, công thức, ảnh, audio trong renderer | [QUESTION_BANK](../../docs/02-learning/QUESTION_BANK.md) |
| FR-GAME-018 | Mở/đóng player trong ngữ cảnh game, suspend/resume thế giới | [LEARNING_EXPERIENCE §2.1](../../docs/02-learning/LEARNING_EXPERIENCE.md) |

### Key Entities

- **Activity attempt** (đọc/ghi qua API): lượt làm một hoạt động — mode, trạng thái, hạn, item hiển thị (không đáp án), câu trả lời đã lưu, FeedbackTiming/ResultRelease.
- **Question item render**: vị trí, loại câu, payload hiển thị, media, công thức, số cấp gợi ý.
- **Response result**: kết quả chấm của server cho một lần trả lời.
- **Attempt review**: kết quả sau nộp, LO yếu, gợi ý ôn tập.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `attempt_resume_window`, `attempt_idle_timeout` | Hành vi resume/pause (server quyết định; client hiển thị) |
| `attempts_per_question` | Số lượt thử (server quyết định; client hiển thị kết quả) |
| `load_time`, `ui_cpu_budget` | Mở activity ≤ mục tiêu; ngân sách UI |
| `math_svg_max` | Giới hạn kích thước SVG nhận |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Render payload không chứa đáp án; client không chấm | ADR-0016 · AGENTS R5, R6 |
| Activity player ≥ 30 FPS mobile/web, ≥ 60 FPS PC; phản hồi chạm ≤ 100 ms | NFR-COMP-001 · [PERFORMANCE_BUDGET §3, §7](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |
| Không yêu cầu phản xạ nhanh; không chỉ dùng màu báo đúng/sai | NFR-A11Y-002 · NFR-A11Y-001 |
| Chuỗi UI qua `.po`, theme từ token | NFR-I18N-001 · ADR-0047 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 11/11 dạng câu trả lời được trên PC và mobile ngang, chỉ bằng chuột/bàn phím hoặc cảm ứng.
- **SC-002**: 0 trường đáp án/feedback xuất hiện trong dữ liệu client trước khi chấm (quét fixture và log).
- **SC-003**: Mất mạng giữa bài không làm mất câu trả lời đã nhập trong 100% lần thử nghiệm.
- **SC-004**: Mở activity player ≤ mục tiêu `load_time` khi media đã preload trên DEV-AND-MID1.

## Ngoài phạm vi

- Renderer `video_lesson`: F065; `audio_lesson`, `simulation`: F066; `minigame`: F073. Bài kiểm tra có giờ (`/exams/*`): F064.
- Chấm, gợi ý, gợi ý ôn tập phía server: F030, F032. Start attempt từ quest/challenge: F072/F076.
- Preview trong CMS (Godot Web): ngoài phạm vi feature này.

## Assumptions

- F030 cung cấp họ API attempt; F032 cung cấp `revealNextHint`, `getMyRecommendations`/`recommendations` trong review — M-C2 dùng Prism mock gắn nhãn; F063 chỉ `Done` khi chạy với F030, F032 thật.
- Response schema theo loại câu (Bước 0 S1) và render payload đóng chặt (S4) chốt trước khi viết answer model.
- `QuestionRender.hint_count`, `ContentManifest.activity_min_client_versions` là delta [Proposed] (contracts §6).

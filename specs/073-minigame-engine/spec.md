# Feature Specification: F073 — Mini-game Engine: `MiniGameConfig`, plugin `minigame`, scene Godot 11 loại

**Feature Branch**: `073-minigame-engine` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `game` + module `learning` + `src/game/` · **Mốc**: M2 · **Phụ thuộc**: F022, F072
**Input**: Mini-game là lớp trình bày trên câu hỏi của Question Bank: cấu hình data-driven có version (`MiniGameConfig`), không có ngân hàng câu hỏi riêng; mỗi lượt chơi là một attempt chấm ở server qua plugin `minigame` của Activity Registry; điểm học tập và điểm vui tách riêng; Godot có scene tái sử dụng cho 11 loại. FR nguồn: FR-MG-001…004, FR-CNT-013.

## Clarifications

### Session 2026-11-16

- Q: Ai sở hữu `MiniGameConfig` và module learning lấy cấu hình bằng cách nào? → A: module game sở hữu (`game.minigame_configs`, item `minigame_config` trên lõi phát hành F071). Khi publish, module game phát `content.published` (`item_type = minigame_config`) mang snapshot cấu hình (không có đáp án); module learning giữ projection bất biến theo version và plugin `minigame` đọc projection — không gọi ngược đồng bộ khi bắt đầu lượt chơi (R29).
- Q: Kiểm "question_source trả loại câu không tương thích" lúc publish thế nào khi câu hỏi ở learning? → A: module game gọi operation facade mới `validateMinigameQuestionSource` của module learning (facade `learning`, chỉ trên đường publish, không trên đường chơi).
- Q: Bắt đầu mini-game qua `startMiniGame` (game, MINIGAME_ENGINE §5) hay `startActivityAttempt` (learning, ACTIVITY_ENGINE §9)? → A: Cả hai hội tụ vào plugin `minigame`: tự chơi và bước quest đi qua module game (`startMiniGame`, `startQuestActivity` → `createAttempt`); bài nhà trường giao dùng learning activity `activity_type = minigame` và `startActivityAttempt` (F041/F063). `startMiniGame` không nhận `context_type = assignment` (`422`).
- Q: Game score tính ở đâu? → A: Plugin `minigame` trong module learning tính theo giờ server khi chấm từng câu (cần cấu hình chấm, chuỗi đúng, thời điểm phát item); module game nhận kết quả qua `activity.graded` (mở rộng phần `minigame`) để ghi lượt chơi và phát `minigame.completed`.
- Q: 11 loại trong một mốc? → A: FR-MG-001 ghi "P2 (6 loại) – sau": 6 loại lõi là P1–P2 của feature; 5 loại còn lại (`calculation`, `puzzle`, `word_game`, `science_simulation`, `timed_challenge`) là story P3 cùng feature.

### Session 2026-11-23 (triển khai)

Quyết định khi triển khai (owner "chốt hết": mặc định an toàn nhất, mọi mục [Confirmed]):

- Q: Bảng tương thích `minigame_type` ↔ loại câu ở đâu? → A: chỉ một bản ở backend — plugin `minigame` của module learning (`compat.go`, test đối chiếu từng dòng MINIGAME_ENGINE §2); module game không giữ bản sao, quyết định publish từ `compatible`/`available_count`/`reasons` của facade `validateMinigameQuestionSource` (T009/T014 dời test bảng sang learning). [Confirmed]
- Q: Mã HTTP của câu trả lời muộn (AC-11)? → A: `ATTEMPT_DEADLINE_PASSED` giữ status của catalog ProblemCode (`422`), không đổi sang `409`; sau khi worker đã chấm lượt hết giờ, câu trả lời tiếp là `409 ATTEMPT_NOT_IN_PROGRESS`. [Confirmed]
- Q: Bảng/cột thêm ngoài data-model? → A: `assessment.attempts.game_streak` (chuỗi đúng cho `streak_bonus`) và `minigame_config_version_id` (version cấu hình ghim, cần cho activity `minigame` bọc cấu hình); `completion_met` + `game_score` lưu trong `evaluations.details.minigame` (đọc lại khi GET, giữ khi chấm lại); `source_kind` thêm `minigame_config`. `ref.minigame_config_versions` là SQL viết tay không mixin entx như mọi `ref.*` (bất biến, `ON CONFLICT DO NOTHING`; app chỉ SELECT). `game.minigame_config_versions.grade_levels`/`question_types` là `jsonb` (quy ước repo) thay `smallint[]`/`varchar[]`. [Confirmed]
- Q: "Thời điểm phát item" để tính `time_bonus`? → A: giờ server của câu trả lời trước (hoặc lúc bắt đầu attempt) tới lúc server nhận câu trả lời; không dùng `last_activity_at` (tạm dừng/gợi ý làm lệch) và không bao giờ dùng `client_elapsed_ms`. Công thức: đủ `max_bonus` khi ≤ `full_bonus_within_ms`, giảm tuyến tính về 0 tại `per_item_time_seconds` (hoặc `time_limit_seconds`/`question_count`, hoặc 2 × `full_bonus_within_ms`), làm tròn. Khối chưa biết = khối 1–5 (không `time_bonus`, `wrong_penalty = 0`). `points_only` bỏ cả thưởng tốc độ và chuỗi. `game_points` là điểm áp thật (có thể âm khi trừ), `game_score` không âm; `game_points`/`streak` không trả khi feedback chưa cho hiện (tránh lộ đúng/sai). [Confirmed]
- Q: `learning_score` và `completion_rule`? → A: `sum_question_scores` = Σ điểm câu; `correct_ratio` = số câu đúng / số câu (score/max); `min_learning_score` so điểm học tập tuyệt đối; `passed` chỉ từ `pass_threshold` của activity bọc. [Confirmed]
- Q: Rút câu? → A: question set: xáo theo seed của attempt rồi cắt `question_count`; selection rule: lọc pool theo loại tương thích trước (nới selection không bao giờ lấy loại khác); `exclude_recently_seen_days`, `retry_policy` chưa áp trong Release (mỗi lượt seed mới). `validateMinigameQuestionSource` đọc metadata `content.question_versions` dưới PlatformScope (view delivery cần tenant phiên). [Confirmed]
- Q: "Version hiện hành" của activity `minigame` bọc cấu hình khi `minigame_config_version = null`? → A: version có `published_at` mới nhất trong projection (chỉ tiêu thụ `content.published`); rollback con trỏ phản ánh khi F078 phát `content.rolled_back`. Module game luôn ghim version tường minh khi gọi `createAttempt`; version chưa tới projection → `409` (retry) → `startMiniGame` `503` + `Retry-After`. [Confirmed]
- Q: `startMiniGame` với `context_type = quest_step`? → A: chỉ là nhãn ngữ cảnh của lượt chơi (bắt buộc `context_id`); hoàn thành bước quest chỉ qua `startQuestActivity` (bộ xử lý bước `minigame`, mode `quest`, hoàn thành khi `completion_met = true`). `MiniGameSession.attempt` là Attempt rút gọn (không có `items`); client đọc câu qua `GET /learning/activity-attempts/{id}` (`render.render_payload = {minigame_type, minigame_config_version_id, config}`); thêm `play_id`; `play_id` suy ra (UUIDv5) từ `Idempotency-Key` để gọi lại dùng lại lượt và attempt. [Confirmed]
- Q: Phần thưởng (F074-1)? → A: `minigame.completed` là trigger của Reward Engine (`game.reward_engine_game`): `source_type = minigame`, `source_id = play_id`, `source_code = activity.completed` chỉ khi `completion_met ∧ rewards_eligible`, chỉ áp rule có `reward_rule_id` của cấu hình (khi có); `activity.graded` mode `minigame` không cộng XP (không thưởng hai lần). Bước quest `minigame` thưởng theo quest. [Confirmed]
- Q: Lượt hết giờ không có câu trả lời? → A: learning để `expired`, không chấm, không `activity.graded` → lượt ở module game giữ `in_progress`, không thưởng, không tính vào `plays_today` (chỉ đếm lượt `completed`). Quy tắc "thoát `timed_challenge` = abandoned" phía server không có lệnh riêng: cấu hình `resumable = false` + `deadline_at` đóng lượt. [Confirmed]
- Q: Seed (T001)? → A: `seed/minigames/` (2 cấu hình + quest "Phiên chợ của bác Gấu Nâu" + gán 3 tenant demo). `cmd/seed` của module game không có module learning nên **không** publish chúng (không bao giờ publish cấu hình chưa kiểm nguồn câu — SC-003), chỉ ghi log; publish qua đường có facade learning (Game Operations Console F078). `TestMiniGameSeedFiles` chứng minh file seed publish được với facade. [Confirmed]
- Q: Hợp đồng scene Godot? → A: signal thì quá khứ theo lint (`setup_completed`, `response_prepared(item_id, response)`, `item_id` = position); `end_of_round` hành xử như `end_of_game` (cấu hình chưa có khái niệm vòng); `per_item_time_seconds` chưa hiện đồng hồ riêng; âm thanh đúng/sai là clip PCM sinh trong code tới khi có asset âm thanh thật (gắn nhãn placeholder). MINIGAME_ENGINE §4–§5 cập nhật theo. [Confirmed]

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Cấu hình mini-game hợp lệ và phát hành (Priority: P1)

Game designer cấu hình "Ghép phép nhân" (matching, bảng nhân 6–9, 6 câu, 180 giây); hệ thống chỉ cho phát hành khi nguồn câu hỏi tương thích loại mini-game.

**Why this priority**: Mọi lượt chơi cần cấu hình đã publish; cấu hình sai làm hỏng học tập.

**Independent Test**: Publish config `matching` với question set chỉ có câu `matching` → thành công, learning projection có version; config `matching` với selection rule gồm `single_choice` → `422`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F073-AC-01 | HỆ THỐNG PHẢI chấp nhận `minigame_type` thuộc 11 loại và cấu hình đủ độ khó, thời gian, số câu, điểm, cách tính kết quả, phản hồi, điều kiện hoàn thành, `reward_rule_id`, trình bày, giới hạn theo `minigame-config.schema.json` | FR-MG-001 · FR-MG-003 | — |
| F073-AC-02 | NẾU `question_source` (question set hoặc selection rule) có thể trả loại câu ngoài bảng tương thích của `minigame_type`, không đủ câu cho `question_count`, hoặc question set chưa publish, THÌ module game PHẢI từ chối publish `422 ACTIVITY_CONFIG_INVALID` kèm lý do | FR-MG-002 · AC-MG-004 | TC-ACT-003 |
| F073-AC-03 | HỆ THỐNG PHẢI không lưu câu hỏi trong cấu hình mini-game: mọi câu lấy từ Question Bank qua `question_source` | FR-MG-002 · [MINIGAME_ENGINE §1](../../docs/08-game-client/MINIGAME_ENGINE.md#1-nguyên-tắc) | — |
| F073-AC-04 | KHI cấu hình được publish, module game PHẢI phát `content.published` (`item_type = minigame_config`) mang snapshot; module learning PHẢI lưu version đó bất biến (event lặp không tạo bản thứ hai) | FR-MG-004 · ADR-0043 | TC-SVC-003 |
| F073-AC-05 | HỆ THỐNG PHẢI từ chối publish khi `name`, `instructions` thiếu `vi` hoặc `en` (`422 TRANSLATION_INCOMPLETE`) hoặc chuỗi vượt `maxLength` của schema (`422 VALIDATION_FAILED`) | NFR-I18N-002 · ADR-0041 | — |

---

### User Story 2 — Chơi mini-game, server chấm (Priority: P1)

Học sinh lớp 3 chơi "Hái quả" ở vườn Ông Số: từng câu được phát không kèm đáp án, trả lời được chấm ở server, hiệu ứng đúng/sai chạy sau kết quả server (US-MG-001).

**Why this priority**: Lõi giá trị của mini-game và yêu cầu chống gian lận (ADR-0016).

**Independent Test**: `startMiniGame` (free_play) → `MiniGameSession` không có `answer_key`; gửi câu trả lời kèm `is_correct`/`game_score` giả → bị bỏ qua; nộp → `learning_score` và `game_score` riêng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F073-AC-06 | KHI học sinh gọi `startMiniGame` (`Idempotency-Key`, `context_type` `free_play` hoặc `quest_step`) cho cấu hình đã publish, gán cho tenant và có khối của học sinh, module game PHẢI ghim version cấu hình (và version question set) theo con trỏ, gọi `createAttempt` (mode `minigame` hoặc `quest`) và trả `201 MiniGameSession` (attempt, `minigame_type`, cấu hình hiển thị, `deadline_at`, `server_time`) | FR-MG-004 · FR-MG-002 · ADR-0043 | TC-LRN-002 |
| F073-AC-07 | HỆ THỐNG PHẢI không chứa `answer_key`, `correct`, `scoring_rule`, phản hồi theo phương án trong render payload hay cấu hình hiển thị gửi client | FR-MG-004 · AC-MG-001 · AGENTS R6 | TC-ACT-001, TC-LRN-007 |
| F073-AC-08 | KHI học sinh gửi câu trả lời, module learning PHẢI chấm theo `scoring_rule` của question version và tính `game_points` từ thời điểm server (phát item → nhận trả lời), chuỗi đúng; trường `score`, `is_correct`, `game_score`, `client_elapsed_ms` do client gửi PHẢI không được dùng để tính điểm | FR-MG-003 · AC-MG-002 · AGENTS R5 | TC-LRN-007 |
| F073-AC-09 | Ở NƠI học sinh thuộc khối 1–5, plugin `minigame` PHẢI không cộng thưởng tốc độ (`time_bonus`) và `wrong_penalty = 0` bất kể cấu hình | FR-MG-003 · NFR-A11Y-002 · [LEARNING_MECHANICS §2](../../docs/01-game-design/LEARNING_MECHANICS.md#2-nguyên-tắc-tích-hợp-nội-tại) | — |
| F073-AC-10 | KHI học sinh nộp, module learning PHẢI trả và lưu riêng `learning_score` (tổng điểm câu), `game_score`, `completion_met` theo `completion_rule`; mastery/evidence PHẢI chỉ dùng câu trả lời, không dùng `game_score` | FR-MG-003 · AC-MG-006 | TC-GAM-001…003 |
| F073-AC-11 | NẾU câu trả lời đến sau `deadline_at`, THÌ module learning PHẢI trả `409 ATTEMPT_DEADLINE_PASSED`; hết giờ PHẢI tự đóng attempt `expired` và chấm phần đã trả lời | FR-MG-003 · AC-MG-003 | TC-EXM-001 |
| F073-AC-12 | NẾU `startMiniGame` nhận `context_type = assignment`, cấu hình không gán tenant/sai khối, THÌ module game PHẢI trả `422 VALIDATION_FAILED` / `404 NOT_FOUND` tương ứng | FR-MG-004 · ADR-0029 | TC-TEN-019 |
| F073-AC-13 | HỆ THỐNG PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY` cho `startMiniGame` với token nhân sự, và chỉ cho học sinh thao tác attempt/lượt chơi của chính mình trong tenant phiên | AGENTS R8, R23 · NFR-SEC-006 | TC-AUTH-014, TC-TEN-001 |

---

### User Story 3 — Plugin `minigame` trong Activity Registry (Priority: P1)

Nhà trường giao một hoạt động mini-game như mọi activity; quest tham chiếu mini-game qua bước `minigame`; thêm loại mới không sửa quest/assignment.

**Why this priority**: FR-CNT-013 — mọi loại hoạt động đi qua interface plugin chung.

**Independent Test**: Activity `minigame` (bọc config) publish được, `startActivityAttempt` chạy qua plugin; test hợp đồng plugin chung xanh; quest có bước `minigame` publish được sau khi đăng ký handler.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F073-AC-14 | module learning PHẢI đăng ký plugin `minigame` (`ConfigSchema`, `ValidateConfig`, `BuildRenderPayload`, `Respond`, `Evaluate`, `CompletionRule`, `EmitEvidence`) và PHẢI qua bộ test hợp đồng chung của plugin | FR-CNT-013 · ADR-0048 | TC-ACT-001 |
| F073-AC-15 | NẾU activity `minigame` tham chiếu `minigame_config_id`/version chưa có trong projection hoặc `question_set_override` không tương thích, THÌ module learning PHẢI trả `422 ACTIVITY_CONFIG_INVALID` khi lưu/publish | FR-CNT-013 · [ACTIVITY_ENGINE §5](../../docs/02-learning/ACTIVITY_ENGINE.md#5-interface-plugin-go) | TC-ACT-003 |
| F073-AC-16 | KHI học sinh bắt đầu bước quest `minigame`, module game PHẢI dùng bộ xử lý bước `minigame` đăng ký vào Quest Engine, tạo attempt mode `quest` qua plugin `minigame`, và hoàn thành bước khi `completion_met = true` | FR-CNT-013 · FR-MG-004 · [QUEST_SYSTEM §3](../../docs/01-game-design/QUEST_SYSTEM.md#3-step-types) | TC-ACT-002 |
| F073-AC-17 | KHI publish version cấu hình mới trong lúc học sinh đang chơi, attempt đang chạy PHẢI giữ câu hỏi và luật chơi của version đã ghim | FR-MG-004 · ADR-0043 | TC-ACT-005 |

---

### User Story 4 — Kết thúc lượt chơi và tín hiệu phần thưởng (Priority: P2)

Sau khi nộp, game ghi lượt chơi, phát `minigame.completed` cho Reward Engine; chơi lại nhiều lần vẫn luyện tập được nhưng không được thưởng thêm.

**Why this priority**: Nối với XP (F074) và chống farm; cần sau khi chơi được.

**Independent Test**: Nộp lượt chơi → `minigame.completed` một lần; chơi lần `max_plays_per_day + 1` → vẫn chơi được, event có `rewards_eligible = false`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F073-AC-18 | KHI nhận `activity.graded` của attempt `minigame`, module game PHẢI cập nhật lượt chơi (`learning_score`, `game_score`, `completion_met`) và phát `minigame.completed` đúng một lần (event lặp không phát lại) | FR-MG-003 · [EVENT_DESIGN §4.5](../../docs/07-backend/EVENT_DESIGN.md#45-game) | TC-SVC-003 |
| F073-AC-19 | NẾU số lượt chơi của học sinh với cấu hình trong ngày vượt `limits.max_plays_per_day`, THÌ module game PHẢI vẫn cho chơi (luyện tập) và đặt `rewards_eligible = false` trong `minigame.completed` | FR-MG-003 · AC-MG-005 · NFR-SAFE-003 | — |
| F073-AC-20 | HỆ THỐNG PHẢI không cấp XP/Xu/item trong module mini-game; phần thưởng do Reward Engine (F074) áp `reward_rule_id` | FR-MG-003 · AGENTS R10 | — |

---

### User Story 5 — Scene Godot cho 6 loại lõi (Priority: P2)

Godot có `MiniGameHost` và scene cho `multiple_choice`, `true_false`, `fill_in_blank`, `drag_and_drop`, `matching`, `ordering`; scene không gọi mạng, không tự chấm.

**Why this priority**: FR-MG-001 Phase P2 yêu cầu 6 loại; trình bày cho US2.

**Independent Test**: GdUnit4 với fixture render payload: mỗi scene phát `response_ready` đúng format loại câu; hiệu ứng chỉ chạy sau `show_feedback`; teardown không rò node.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F073-AC-21 | game client PHẢI chọn scene theo `minigame_type` qua `MiniGameRegistry`, biến thể hình ảnh theo `presentation.scene_variant`; loại chưa có scene PHẢI hiện "Cần cập nhật game", gửi telemetry, không crash | FR-MG-004 · FR-MG-001 | TC-ACT-004 |
| F073-AC-22 | Scene mini-game PHẢI chỉ phát `response_ready(item_id, response)` đúng định dạng response của loại câu, không gọi mạng, không tự tính đúng/sai hay điểm; hiệu ứng đúng/sai chỉ chạy sau `show_feedback` hoặc ở cuối (`end_of_round`/`end_of_game`) | FR-MG-004 · ADR-0016 | TC-LRN-016 |
| F073-AC-23 | game client PHẢI hiển thị đồng hồ theo `deadline_at` và `server_time` (không dừng giờ khi mất mạng), gửi lại câu trả lời trong hàng đợi cùng `Idempotency-Key` khi có mạng | FR-MG-004 · [MINIGAME_ENGINE §5](../../docs/08-game-client/MINIGAME_ENGINE.md#5-vòng-đời) | — |
| F073-AC-24 | Ở NƠI học sinh thuộc khối 1–5, game client PHẢI dùng vùng chạm ≥ 64 px, có âm thanh + biểu tượng cho đúng/sai (màu không là tín hiệu duy nhất), hỗ trợ chuột, cảm ứng, bàn phím | FR-MG-004 · NFR-A11Y-001 | — |

---

### User Story 6 — 5 loại mở rộng (Priority: P3)

Thêm `calculation`, `puzzle`, `word_game`, `science_simulation`, `timed_challenge` theo cùng hợp đồng.

**Why this priority**: FR-MG-001 cho phép làm sau 6 loại lõi.

**Independent Test**: Như US5 cho 5 loại; `timed_challenge` nhận lô item trước và vẫn chấm ở server; `science_simulation` nộp trạng thái cuối như response câu `interactive`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F073-AC-25 | game client PHẢI có scene cho `calculation`, `puzzle`, `word_game`, `science_simulation`, `timed_challenge` theo hợp đồng `MiniGameBase`, đạt cùng tiêu chí F073-AC-21…F073-AC-24 | FR-MG-001 · FR-MG-004 | — |
| F073-AC-26 | KHI chơi `timed_challenge`, plugin PHẢI phát sẵn lô render payload để client không chờ mạng giữa câu, nhưng kết quả mỗi câu PHẢI vẫn do server chấm; `science_simulation` PHẢI nộp trạng thái cuối như response của câu `interactive` | FR-MG-001 · [MINIGAME_ENGINE §2, §5](../../docs/08-game-client/MINIGAME_ENGINE.md) | — |

### Edge Cases

- NẾU projection cấu hình ở learning chưa nhận version vừa publish khi học sinh bắt đầu, THÌ `createAttempt` trả `409`; module game trả `503 UNAVAILABLE` có `Retry-After` (F072-AC-18 tương tự).
- NẾU học sinh thoát giữa chừng, THÌ attempt `in_progress` resume trong `attempt_resume_window`; `timed_challenge` không resume (thoát = `abandoned`).
- NẾU question pool không đủ câu khi start (`QUESTION_POOL_INSUFFICIENT`), THÌ trả lỗi thân thiện, không tạo lượt chơi.
- NẾU cấu hình bị gỡ gán tenant giữa lượt, THÌ lượt đang chơi hoàn thành trên version ghim; lượt mới `404`.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-MG-001 | 11 `minigame_type`, bảng tương thích loại câu; scene 6 loại lõi (P2) + 5 loại mở rộng (P3) | [MINIGAME_ENGINE §2](../../docs/08-game-client/MINIGAME_ENGINE.md#2-loại-mini-game-và-tương-thích-câu-hỏi) |
| FR-MG-002 | `question_source` từ Question Bank; kiểm tương thích khi publish; không có câu hỏi riêng | [MINIGAME_ENGINE §1, §3](../../docs/08-game-client/MINIGAME_ENGINE.md) |
| FR-MG-003 | Cấu hình độ khó, thời gian, số câu, điểm, cách tính kết quả, phản hồi, hoàn thành, reward rule; learning/game score tách; giới hạn lượt chơi | [MINIGAME_ENGINE §3, §6](../../docs/08-game-client/MINIGAME_ENGINE.md) |
| FR-MG-004 | `MiniGameHost` + scene tái sử dụng nhận `MiniGameConfig` + render payload; `startMiniGame` ghim version | [MINIGAME_ENGINE §4–§5](../../docs/08-game-client/MINIGAME_ENGINE.md) |
| FR-CNT-013 | Plugin `minigame` trong Activity Registry; quest/assignment tham chiếu qua interface chung | [ACTIVITY_ENGINE §3, §5](../../docs/02-learning/ACTIVITY_ENGINE.md), ADR-0048 |

### Key Entities

- **MiniGameConfig / MiniGameConfigVersion**: cấu hình mini-game platform có version, gán tenant.
- **MiniGameConfigSnapshot (projection ở learning)**: bản bất biến của version cấu hình để plugin chấm.
- **MiniGamePlay**: lượt chơi của học sinh: version ghim, ngữ cảnh, attempt, điểm học tập, điểm vui, hoàn thành, đủ điều kiện thưởng.
- **Minigame activity**: learning activity `activity_type = minigame` bọc một cấu hình (bài nhà trường giao).
- **MiniGameRegistry / MiniGameBase (client)**: ánh xạ loại → scene và hợp đồng scene.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `attempt_resume_window` | Resume lượt chơi không tính giờ |
| `learning_block_max_items` | Kiểm `question_count` theo grade band khi publish (cảnh báo khi vượt trong ngữ cảnh quest) |
| `idempotency_record_ttl` | `startMiniGame`, gửi/nộp câu trả lời |
| `api_rate_default` | Rate limit `startMiniGame` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chấm và tính game score ở server, không gửi đáp án | ADR-0016 · AGENTS R5, R6 |
| learning đọc cấu hình qua projection event, game gọi learning chỉ qua facade `learning` | AGENTS R13, R29 · ADR-0074 · ADR-0042 |
| Không thưởng tốc độ lớp 1–5; không pay-to-win | NFR-SAFE-003 · NFR-A11Y-002 · AGENTS R19 |
| Scene trong ngân sách hiệu năng máy tầm trung | NFR-COMP-001 · ADR-0057 |
| UI theo Theme game, chuỗi qua file dịch | AGENTS R32 · NFR-I18N-001 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 trường đáp án/chấm điểm trong payload gửi client (contract test mọi `minigame_type`).
- **SC-002**: 100% điểm học tập và điểm vui khớp tính lại từ câu trả lời và giờ server trong test giả mạo client.
- **SC-003**: 0 cấu hình có nguồn câu không tương thích được publish.
- **SC-004**: Phản hồi đúng/sai hiện ≤ 100 ms sau khi nhận kết quả server trên máy tầm trung.

## Ngoài phạm vi

- Cấp XP/Xu/item theo `reward_rule_id`, cap → F074/F075; game score trong thi đấu → F104–F106.
- Biên tập cấu hình, preview Godot Web trên Game Operations Console (`/game-admin/minigame-configs`) → F078/F079; activity builder trên portal → F058.
- Giao bài mini-game cho lớp (assignment, `startActivityAttempt`, tab "Nhà trường giao") → F041/F055/F063; `difficulty = adaptive` → sau Release (Phase 3, [Confirmed]).
- Cờ thời gian trả lời bất thường dùng `rapid_guess` sẵn có → F032; cơ chế học tập nội tại pilot (LEARNING_MECHANICS §7) là nội dung, không phải engine.

## Assumptions

- F022/F023 cung cấp validator, scorer server-side và render payload cho 11 `QuestionType`; F030 cung cấp Activity Registry, `createAttempt`, họ API `/learning/activity-attempts`; F063 cung cấp `ActivityHost` và hợp đồng renderer.
- Seed 2 cấu hình (matching Toán lớp 3, true_false) + quest mẫu "Phiên chợ của bác Gấu Nâu" có bước `minigame`, gán 3 tenant demo.
- Tên loại theo `minigame-config.schema.json` (`drag_and_drop`); OpenAPI `MiniGameSession` dùng `MiniGameType` (`drag_and_drop`) — đã sửa ở Phase 2.

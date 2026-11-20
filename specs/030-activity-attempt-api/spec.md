# Feature Specification: F030 — Activity Registry và Attempt API (`quiz`, `interactive_exercise`)

**Feature Branch**: `030-activity-attempt-api` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `learning` · **Mốc**: M-C2 · **Phụ thuộc**: F026
**Input**: Activity Registry với interface plugin chung (ADR-0048) và hai plugin đầu `quiz`, `interactive_exercise`; một họ API attempt `/api/v1/learning/activity-attempts` (ADR-0059): start (ghim version), trả lời, tạm dừng, resume, nộp idempotent, xem lại; chấm server-side, answer key không rời server (ADR-0016). FR nguồn: FR-CNT-013, FR-EXR-002, FR-EXR-004…007, FR-EXR-016, FR-QB-003.

## Clarifications

### Session 2026-11-16

- Q: Mode nào thuộc F030? → A: `practice` (từ activity và ad-hoc `question_set`/`selection`), `quest` và `minigame` qua facade `learning` cho module game. `assignment` → F041; `retry_wrong`, `diagnostic`, gợi ý `hints:next`, cờ `rapid_guess` → F032; `exam` → F042; `competition` → F105.
- Q: Tên domain event khi nộp? → A: `activity.submitted` + `activity.graded` (đã có payload schema, EVENT_DESIGN §4.3); không phát song song `exercise.submitted` [Confirmed].
- Q: Mã lỗi vị trí item không tồn tại? → A: `422 VALIDATION_FAILED` (mã `ITEM_POSITION_INVALID` của ACTIVITY_ENGINE §9 chưa có trong `ProblemCode`).
- Q: Phản hồi đến sau hạn? → A: Theo FR-EXR-016: phản hồi sau `deadline_at` + `deadline_grace` → `410 ATTEMPT_EXPIRED`; nộp sau hạn → `422 ATTEMPT_DEADLINE_PASSED` (API_DESIGN §4.1).

### Session 2026-11-17

- Q: Tên event khi attempt bị bỏ dở và nhóm `exercise.*` trong learning event? → A: Domain event `activity.abandoned` (EVENT_DESIGN, RB-206); `learning-event.schema.json` còn `exercise.started`/`exercise.submitted` → delta contract đổi sang `activity.*` (task T052). Mode `challenge` dùng `context_type` `challenge_attempt` (F076) hoặc `boss_encounter` (+ `phase`, boss F103) [Confirmed].

### Session 2026-11-20 (triển khai — Foundation + US4)

- Q: Lưu activity thế nào? → A: Activity là **loại nội dung ghép** như F024: `content.activities` (item, H, con trỏ release) + `content.activity_versions` (cột chung bảng version F024, `ContentVersionMixin`, `WorkflowFields`); `activity_type`, `instructions`, `difficulty`, `time_limit_seconds`, `reward_rule_id`, `config`, `math_assets` nằm trong `body`, `description` là cột `summary`; cột sinh `activity_type` (CHECK `ActivityType`) phục vụ lọc. Bảng H (không G như DATABASE_DESIGN cũ): activity tenant + activity platform gán qua `resource_type = activity` [Confirmed]. Workflow (F025) và release (F026) dùng chung endpoint `/cms/learning/content/activity/...`.
- Q: Gán activity platform cho tenant? → A: CHECK `tra_resource_type_ck` thêm `activity`; `ResourceType` của facade/API platform-ops chưa có `activity` → **Câu hỏi mở** (F029/F045 bổ sung); test dùng dòng gán trực tiếp.
- Q: `speaking`/`written_response` khi tạo? → A: Bị chặn ở tầng contract (enum `ActivityType`) với `422 VALIDATION_FAILED`; `422 ACTIVITY_TYPE_NOT_SUPPORTED` cho loại trong enum chưa có plugin (`video_lesson`, `audio_lesson`, `simulation`, `minigame`) [Confirmed].
- Q: `ActivityDefinitionInput` không có `curriculum_version_id`? → A: Lấy curriculum version của LO đầu tiên [Confirmed].
- Q: `updateCmsActivityDraft`? → A: Body `application/json` là `ActivityDefinitionInput` đầy đủ (contract), sửa version mới nhất khi còn `draft` (`If-Match` = ETag của version đó); version khác `draft` → `409 CONTENT_NOT_EDITABLE`; đổi `activity_type` → `422`.
- Q: `feedback.timing = after_close` cho activity? → A: Từ chối (`ACTIVITY_CONFIG_INVALID`): cần cửa sổ bài giao/đề (F041, F042) [Confirmed].
- Q: Readiness thiếu plugin? → A: `infrastructure/health.RegistryCheck` + `SQLPublishedTypes` (pool `learning_platform`); đăng ký vào `/health/ready` thuộc `module.go` (agent wiring); `min_client_version` lấy từ registry thay bảng tạm của F026.
- Q: Partition `learning_events`, `attempt_answers`? → A: Migration tạo sẵn partition tháng 2026-11 → 2027-12; job duy trì là F031-AC-04.

### Session 2026-11-20 (triển khai — US1, US2, US3, US5, US6)

- Q: `school_id` của attempt? → A: Lấy từ projection `ref.students` (F020); học sinh chưa có trong projection của tenant → `403 PERMISSION_DENIED` [Confirmed].
- Q: Thứ tự item và `presentation`? → A: Item xáo theo `seed` khi `interaction.shuffle_items` (mặc định true); `presentation` (thứ tự phương án, tham số) lưu ở `attempt_items`; resume render lại bằng cùng `seed` trên version ghim. `max_score` của attempt = Σ `max_score` item (§7.2); `config.evaluation.max_score` chưa dùng để quy đổi [Confirmed].
- Q: Lượt thử? → A: `FeedbackTiming = immediate`: lần đầu tính điểm, thử lại `is_scored = false` khi `allow_unscored_retry` (mặc định true); phản hồi trì hoãn: mọi lần đều tính điểm, lần cuối có hiệu lực. Tổng số lần mỗi item ≤ `attempts_per_question` theo khối (g1_3: 3, khác: 2; khối từ `ref.students`). `answer_rate` chưa áp dụng (Câu hỏi mở, cần cơ chế rate limit dùng chung).
- Q: Learning event `question.answered`? → A: Ghi cho mọi lần trả lời (kể cả `is_scored = false`), khóa `qa:<attempt>:<position>:<attempt_no>`; `class_id` snapshot từ `ref.enrollments` đang active. `hints_used = 0`, `rapid_guess = false` tới F032.
- Q: `getActivityAttempt` có kết thúc pause? → A: Không (query không ghi); pause kết thúc ở lệnh ghi kế tiếp (trả lời/nộp), thời gian pause cộng vào `paused_ms`. Idle: job đặt `paused_at = last_activity_at + attempt_idle_timeout`.
- Q: Hết hạn không có câu trả lời? → A: `expired`, không chấm, không phát event; có ≥ 1 câu → nộp `timeout`, `submitted_at = deadline_at`.
- Q: Mode/ngữ cảnh public? → A: `startActivityAttempt`: `practice` (`free_play`/`lesson`/`learning_path`/`recommendation`) và `minigame` (`minigame_session`); `startAdHocAttempt`: `practice` với `question_set` (bộ tĩnh; bộ theo quy tắc → Câu hỏi mở) hoặc `selection`; mode khác → `422 VALIDATION_FAILED` (F032, F041).
- Q: Attempt ad-hoc: `activity_type`, `source_version_id`? → A: `quiz` khi mọi câu là loại quiz, ngược lại `interactive_exercise`; `source_kind` (`activity`/`question_set`/`selection`), `source_version_id` = version bộ câu hỏi, hoặc chính `attempt_id` với `selection` (câu ghim ở item) [Confirmed].
- Q: Facade `createAttempt`? → A: `ActivityRef.content_type` thêm `activity`; principal của lời gọi là học sinh (on-behalf, `student_id` phải trùng); `mode` ∈ {quest, minigame} với `context_type` tương ứng; `getAttemptResult` theo `TenantScope` (tenant khác → `404 ATTEMPT_NOT_FOUND`). Adapter phía module game và facade test bên gọi thuộc feature game tiêu thụ (F072) [Confirmed].
- Q: Version mới của activity? → A: Contract chưa có `createCmsActivityVersion`; `command.Docs.NewVersion` dùng được (test), endpoint → **Câu hỏi mở** (F053).
- Q: Review khi `feedback_timing` `never`/`after_close`? → A: `403 PERMISSION_DENIED` [Confirmed]; review trả `explanation` (richContent theo ngôn ngữ attempt), `feedback` của phương án đã chọn ở lần trả lời cuối, `weak_objectives` = LO của item sai/bỏ trống.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Bắt đầu luyện tập từ activity đã xuất bản (Priority: P1)

Học sinh mở một activity trong game; server ghim version hiện hành và trả nội dung hiển thị không có đáp án.

**Why this priority**: Điểm vào của mọi lượt học trong Content Loop M-C2.

**Independent Test**: Activity `quiz` đã publish (F026) → `startActivityAttempt` bằng token học sinh → attempt `in_progress`, items có `question_version_id` ghim, payload không có trường đáp án.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F030-AC-01 | KHI học sinh gọi `startActivityAttempt` cho activity đang publish có plugin, module learning PHẢI tạo attempt `in_progress` (`mode = practice`), ghim `source_version_id` và `question_version_id` từng item theo con trỏ lúc start, ghi `started_at`, `seed`, `display_language` (server), trả `201 Attempt` kèm `server_time`, `feedback_timing`. | FR-EXR-002 · FR-CNT-011 · AC-EXR-005 | TC-LRN-002 |
| F030-AC-02 | HỆ THỐNG PHẢI không trả trong `ActivityRender`, `QuestionRender`, `Attempt` các trường `answer_key`, `scoring_rule`, `accepted_answers`, `correct_option_id(s)`, `explanation`, `feedback`, `options[].feedback`, `hints`, `misconception_tag`, `min_response_ms` trước khi chấm. | FR-QB-003 · ADR-0016 · AC-EXR-001 | TC-LRN-007 |
| F030-AC-03 | NẾU activity chưa publish hoặc đã gỡ, THÌ module learning PHẢI trả `409 CONTENT_NOT_PUBLISHED`; NẾU activity thuộc tenant khác hoặc là nội dung platform chưa gán cho tenant học sinh, THÌ PHẢI trả `404 NOT_FOUND`. | ADR-0029 · R8 | TC-TEN-001 |
| F030-AC-04 | TRONG KHI học sinh đã có attempt `practice` `in_progress` cho cùng `source_version_id` còn trong `attempt_resume_window`, module learning PHẢI trả lại attempt đó thay vì tạo mới. | FR-EXR-004 · [ACTIVITY_ENGINE §6.3](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-LRN-009 |
| F030-AC-05 | HỆ THỐNG PHẢI yêu cầu `Idempotency-Key` cho start (thiếu → `428 IDEMPOTENCY_KEY_REQUIRED`); cùng key → cùng attempt; vượt `attempt_start_rate` → `429 RATE_LIMITED`. | R10 · [API_DESIGN §6](../../docs/07-backend/API_DESIGN.md) | TC-LRN-008 |
| F030-AC-06 | NẾU token không phải `eduworld-game-backend` hoặc `user_type ≠ student`, THÌ module learning PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY`; NẾU học sinh đọc/ghi attempt của người khác, THÌ PHẢI trả `404 ATTEMPT_NOT_FOUND`. | R23 · ADR-0031 · AC-ACT-015 | TC-TEN-001 |
| F030-AC-07 | KHI gọi `listMyActivityAttempts`, module learning PHẢI chỉ trả attempt của học sinh đang đăng nhập, lọc `mode`/`status`/`activity_type`, phân trang cursor. | FR-EXR-007 | TC-LRN-006 |

---

### User Story 2 — Trả lời, chấm và nộp bài (Priority: P1)

Học sinh trả lời từng câu; server chấm, phản hồi theo `FeedbackTiming`; học sinh nộp một lần, điểm do server tính.

**Why this priority**: Chấm server-side là yêu cầu cứng (ADR-0016) và là bằng chứng cho mastery (F031).

**Independent Test**: Practice 10 câu → 10 lần `submitActivityResponse` → `:submit` → 1 attempt `graded`, 10 item, ≥ 10 answer, evaluation `final`, outbox `activity.submitted` + `activity.graded`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F030-AC-08 | KHI học sinh gọi `submitActivityResponse` cho item hợp lệ, module learning PHẢI chấm bằng answer key của version ghim, ghi `attempt_answers` (append-only, `answered_at` và `response_ms` theo server) và learning event `question.answered` trong cùng transaction, trả `ActivityResponseResult` theo `feedback_timing` (`immediate`: `is_correct`, `score`, `explanation`, `feedback`; khác: chỉ `accepted`). | FR-EXR-002 · [ACTIVITY_ENGINE §7, §8](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-LRN-003 |
| F030-AC-09 | HỆ THỐNG PHẢI bỏ qua mọi trường `score`, `is_correct`, `max_score`, `duration_ms` client gửi kèm; điểm chỉ do server tính. | ADR-0016 · AC-EXR-002 | TC-LRN-007 |
| F030-AC-10 | NẾU `response` sai cấu trúc của `QuestionType` THÌ module learning PHẢI trả `422 INVALID_RESPONSE`; NẾU `position` không có trong attempt THÌ PHẢI trả `422 VALIDATION_FAILED`; NẾU vượt `attempts_per_question` THÌ PHẢI trả `409 ANSWER_LIMIT_REACHED` (lần thử lại sau lần đầu ghi `is_scored = false`). | [QUESTION_BANK §8](../../docs/02-learning/QUESTION_BANK.md) · [LEARNING_EXPERIENCE §4.2](../../docs/02-learning/LEARNING_EXPERIENCE.md) | TC-LRN-016 |
| F030-AC-11 | NẾU attempt không còn `in_progress`, THÌ module learning PHẢI trả `409 ATTEMPT_NOT_IN_PROGRESS` cho trả lời và tạm dừng. | [ACTIVITY_ENGINE §6.2](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-LRN-008 |
| F030-AC-12 | KHI gọi `submitActivityAttempt`, module learning PHẢI trong một transaction có điều kiện `status = 'in_progress'` chuyển `submitted → graded`, tính `score = Σ final_score`, `max_score`, `duration_ms = submitted_at − started_at − paused_ms` (server; `client_elapsed_ms` chỉ lưu tham khảo), tạo evaluation `source = system`, `effective = true`, ghi learning event + outbox `activity.submitted`, `activity.graded`. | FR-EXR-002 · FR-EXR-006 · FR-EXR-008 · AC-ACT-011 | TC-LRN-004 |
| F030-AC-13 | KHI `:submit` lặp cùng `Idempotency-Key`, module learning PHẢI trả response đã lưu; NẾU khác key sau khi đã nộp, THÌ PHẢI trả `409 ATTEMPT_ALREADY_SUBMITTED`; hai lệnh đồng thời chỉ một có hiệu lực, điểm không đổi. | FR-EXR-005 · AC-EXR-003 | TC-LRN-008 |
| F030-AC-14 | KHI học sinh làm lại một activity, module learning PHẢI tạo attempt mới; attempt cũ, câu trả lời và evaluation cũ không đổi, kể cả khi đã publish version mới. | AC-EXR-004 · AC-EXR-005 · FR-CMS-007 | TC-ACT-005 |
| F030-AC-15 | KHI gọi `getActivityAttemptReview` cho attempt `graded` có `feedback_timing` ≠ `never`, module learning PHẢI trả từng item với `is_correct`, `score`, `explanation`, `feedback` của phương án đã chọn; NẾU attempt chưa `graded`, THÌ PHẢI trả `409 CONFLICT`. | [ACTIVITY_ENGINE §8.1](../../docs/02-learning/ACTIVITY_ENGINE.md) · FR-QB-003 | TC-LRN-012 |

---

### User Story 3 — Tạm dừng, resume và hết hạn (Priority: P1)

Mất mạng giữa bài, học sinh mở lại trong cửa sổ resume và tiếp tục đúng câu; bài tính giờ hết hạn thì server tự nộp. US-EXR-002.

**Why this priority**: Mạng trường học không ổn định; mất bài làm gây bỏ cuộc.

**Independent Test**: Trả lời 3/10 câu, đóng game, mở lại → `getActivityAttempt` trả đúng 3 câu đã lưu, câu 4 tiếp theo; quiz tính giờ để quá hạn → attempt `graded` với `submit_reason = timeout`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F030-AC-16 | KHI gọi `getActivityAttempt` cho attempt `in_progress`, module learning PHẢI trả items và `saved_responses` trên đúng version đã ghim, phản hồi đã nhận theo `feedback_timing`, `deadline_at`, `server_time`. | FR-EXR-004 · AC-EXR-006 | TC-LRN-009 |
| F030-AC-17 | KHI gọi `pauseActivityAttempt` hoặc attempt không hoạt động quá `attempt_idle_timeout`, module learning PHẢI đặt `paused_at`; thời gian tạm dừng PHẢI bị loại khỏi `duration_ms`. | FR-EXR-004 · FR-EXR-006 | TC-LRN-009 |
| F030-AC-18 | KHI attempt `practice` tạm dừng/không hoạt động quá `attempt_resume_window`, worker PHẢI chuyển `abandoned`, phát `activity.abandoned`; mọi lệnh sau đó trên attempt PHẢI nhận `410 ATTEMPT_EXPIRED`. | FR-EXR-004 · [ACTIVITY_ENGINE §6.2](../../docs/02-learning/ACTIVITY_ENGINE.md) · [EVENT_DESIGN](../../docs/07-backend/EVENT_DESIGN.md) | TC-LRN-009 |
| F030-AC-19 | KHI quá `deadline_at` (activity có `time_limit_seconds`), worker PHẢI chuyển `expired` và tự nộp nếu đã có câu trả lời (`submit_reason = timeout`, `graded`); phản hồi đến sau `deadline_at` + `deadline_grace` PHẢI nhận `410 ATTEMPT_EXPIRED`, nộp sau hạn nhận `422 ATTEMPT_DEADLINE_PASSED`. | FR-EXR-016 · FR-EXR-006 | — |

---

### User Story 4 — Activity Registry và soạn activity (Priority: P2)

Content team tạo activity `quiz`/`interactive_exercise` theo JSON Schema của loại; registry từ chối loại ngoài Release; mọi plugin qua bộ test hợp đồng chung.

**Why this priority**: Cho phép quest/assignment/exam chỉ tham chiếu `activity_id` + version; mở đường thêm loại mới không sửa lõi.

**Independent Test**: `createCmsActivity` loại `speaking` → `422 ACTIVITY_TYPE_NOT_SUPPORTED`; `quiz` có `config` sai schema → `422 ACTIVITY_CONFIG_INVALID`; chạy test hợp đồng cho 2 plugin.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F030-AC-20 | NẾU `activity_type` ngoài `ActivityType` của Release (ví dụ `written_response`, `speaking`, `project`) hoặc chưa có plugin đăng ký, THÌ module learning PHẢI từ chối tạo/publish activity bằng `422 ACTIVITY_TYPE_NOT_SUPPORTED`. | FR-CNT-013 · AC-EXR-008 · ADR-0072 | — |
| F030-AC-21 | NẾU `config` không qua JSON Schema của loại khi lưu draft hoặc publish, THÌ module learning PHẢI trả `422 ACTIVITY_CONFIG_INVALID` kèm JSON pointer. | FR-CNT-013 · ADR-0048 | TC-ACT-003 |
| F030-AC-22 | KHI gọi `listActivityTypes`, module learning PHẢI trả mỗi loại đã đăng ký với `schema_url`, `schema_version`, `evaluation_strategies`, `min_client_version`, `status`. | FR-CNT-013 · FR-EXR-015 | TC-ACT-004 |
| F030-AC-23 | HỆ THỐNG PHẢI chạy bộ test hợp đồng chung cho mọi plugin (render không lộ đáp án, `Evaluate` idempotent, evidence đúng schema) và PHẢI báo không sẵn sàng (readiness fail) khi có nội dung published của loại không có plugin. | ADR-0048 · AC-ACT-002 | TC-ACT-001 |
| F030-AC-24 | NẾU người gọi `createCmsActivity`/`updateCmsActivityDraft` thiếu `learning_content.activity.write`, thuộc tenant khác (→ `404`) hoặc thiếu `If-Match` (→ `428`), THÌ module learning PHẢI từ chối; chuỗi vượt `maxLength` → `422 VALIDATION_FAILED`. | R7 · R8 · R28 · ADR-0041 | TC-TEN-024 |

---

### User Story 5 — Attempt cho bước quest/mini-game từ module game (Priority: P2)

module game mở attempt khi học sinh bắt đầu bước quest/mini-game; client dùng chung API public để làm bài; kết quả quay lại module game qua event.

**Why this priority**: Gameplay M2 tiêu thụ; contract cần ổn định từ M-C2 cho Godot (F063).

**Independent Test**: Module game (fake bên gọi) gọi facade `createAttempt` → học sinh dùng `getActivityAttempt`/`:submit` → `getAttemptResult` trả điểm, event `activity.submitted` có `context_type = quest_step`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F030-AC-25 | KHI module `game` gọi `createAttempt` với `mode ∈ {quest, minigame}` và activity đang publish, module learning PHẢI tạo attempt cho `student_id` với `context`, ghim version, trả `AttemptCreated`; mode khác → `422 VALIDATION_FAILED`; cùng `Idempotency-Key` → attempt cũ. | FR-CNT-013 · ADR-0074 · ADR-0040 | TC-ACT-002 |
| F030-AC-26 | NẾU một module khác `game` import facade `createAttempt`/`getAttemptResult` (ngoài `x-allowed-callers`), THÌ `make arch-check` PHẢI fail và các operation này PHẢI KHÔNG có route HTTP; `getAttemptResult` PHẢI không chứa đáp án và trả `404` khi `TenantScope` trong `context` không khớp attempt. | R29 · [API_DESIGN §11](../../docs/07-backend/API_DESIGN.md) | TC-SVC-006 |

---

### User Story 6 — Luyện tập ad-hoc từ bộ câu hỏi hoặc điều kiện chọn (Priority: P2)

Học sinh luyện theo bộ câu hỏi đã publish hoặc theo môn/chủ đề/LO/độ khó, không cần activity cố định.

**Why this priority**: Practice Zone dùng nhiều nhất; không chặn MVP từ activity.

**Independent Test**: `startAdHocAttempt` `source.kind = selection` (topic, 10 câu) → 10 item cùng topic; pool chỉ 5 câu → `QUESTION_POOL_INSUFFICIENT`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F030-AC-27 | KHI gọi `startAdHocAttempt` với `source.kind ∈ {question_set, selection}`, module learning PHẢI chọn câu đã publish trong tenant (gồm nội dung platform được gán) theo [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md) với `seed` lưu lại, ghim version từng câu, trả `201 Attempt`; NẾU không đủ câu, THÌ PHẢI trả `422 QUESTION_POOL_INSUFFICIENT`. | FR-EXR-007 · FR-QB-004 | TC-LRN-001 |

### Edge Cases

- Unpublish activity khi học sinh đang làm: attempt tiếp tục trên version ghim (F030-AC-14, F026).
- Hai thiết bị: một game session active (ADR-0031) nên attempt chỉ được thao tác từ thiết bị hiện hành; thiết bị mới resume bình thường (F030-AC-16).
- Câu không trả lời khi nộp: `final_score = 0`.
- Nộp attempt chưa trả lời câu nào: hợp lệ, điểm 0; auto-submit khi hết hạn chỉ chạy nếu có ≥ 1 câu trả lời, ngược lại `expired` không chấm.
- Học sinh đổi ngôn ngữ giữa chừng: `display_language` của attempt không đổi (ADR-0056).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CNT-013 | Registry + interface plugin, plugin `quiz`, `interactive_exercise`; từ chối loại ngoài Release | [ACTIVITY_ENGINE §3, §5, §12](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-002 | Lưu attempt, answer, đúng/sai, điểm, thời lượng, thời điểm nộp, version, LO | [ACTIVITY_ENGINE §6.4](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-004 | Resume, pause, abandoned theo cửa sổ resume | [ACTIVITY_ENGINE §6.3](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-005 | Submit idempotent, một lần | [ACTIVITY_ENGINE §11](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-006 | Thời gian theo server | [ACTIVITY_ENGINE §6.4](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-007 | Một họ API attempt (public + facade) | [ACTIVITY_ENGINE §9](../../docs/02-learning/ACTIVITY_ENGINE.md) · ADR-0059 |
| FR-EXR-016 | `expired`, auto-submit, `ATTEMPT_EXPIRED` | [ACTIVITY_ENGINE §6.2, §11](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-QB-003 | Render payload/DTO không có answer key, scoring rule | [QUESTION_BANK §8](../../docs/02-learning/QUESTION_BANK.md) · ADR-0016 |

### Key Entities

- **LearningActivity / ActivityVersion**: đơn vị học làm được, có `activity_type`, cấu hình theo schema loại, bất biến khi publish.
- **ActivityPlugin**: hiện thực interface chung cho một `activity_type`.
- **Attempt**: một lượt học sinh làm một version đã ghim (mode, trạng thái, thời gian server, điểm).
- **AttemptItem / AttemptAnswer**: câu đã giao (version ghim, cách hiển thị) và các lần trả lời (append-only).
- **Evaluation**: kết quả chấm; đúng một evaluation hiệu lực mỗi attempt.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `attempt_idle_timeout` | Tự đặt `paused_at` khi không hoạt động |
| `attempt_resume_window` | Hạn resume practice/quest trước khi `abandoned` |
| `deadline_grace` | Ân hạn mạng sau `deadline_at` |
| `attempts_per_question` | Số lượt thử mỗi câu theo khối |
| `attempt_start_rate` | Giới hạn số attempt bắt đầu |
| `answer_rate` | Giới hạn tốc độ trả lời/nộp |
| `suspicious_speed_flag` | Gắn cờ gửi ANTI_CHEAT (không phạt) |
| `idempotency_record_ttl` | Giữ kết quả Idempotency-Key |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không log câu trả lời học sinh ở INFO | AGENTS R14 |
| Learning event ghi cùng transaction với thay đổi nghiệp vụ | [EVENT_DESIGN §5.4](../../docs/07-backend/EVENT_DESIGN.md) |
| Gọi từ module game chỉ qua facade `learning` + `x-allowed-callers` | R29 · ADR-0074 · ADR-0040 |
| Điểm học tập không bị XP/vật phẩm ảnh hưởng | ADR-0018 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 response API học sinh chứa trường đáp án (contract test quét mọi operation `/learning/*`).
- **SC-002**: 100% attempt `graded` có đúng một evaluation hiệu lực và điểm tính lại từ `attempt_answers` khớp.
- **SC-003**: Học sinh mất mạng mở lại trong cửa sổ resume tiếp tục đúng câu trong 100% kịch bản E2E.
- **SC-004**: Nộp trùng (cùng/khác key, đồng thời) không bao giờ tạo hai kết quả.

## Ngoài phạm vi

- `assignment` (ResultRelease, `ASSIGNMENT_*`) → F041; `retry_wrong`, `diagnostic`, `hints:next`, `rapid_guess` → F032; `exam` → F042; `competition` → F105.
- `video_lesson`, `audio_lesson`, `simulation`, `minigame` plugin → F065, F066, F073.
- Mastery, `learning_objective.*`, re-evaluate → F031.
- Renderer Godot → F063; màn soạn activity → F053.

## Assumptions

- Scorer và validator 8 loại câu cơ bản do F022 cung cấp; `short_answer`, `image_based`, `interactive` dùng được trong `interactive_exercise` khi F023 Done.
- Bảng `learning.learning_events` (partition tháng đầu tiên) tạo ở F030 vì F030 ghi event trong transaction; job tạo partition, kiểm schema và mastery thuộc F031.
- `context_type` của attempt dùng hợp các giá trị trong `ActivityAttemptStart`, `AdHocAttemptStart`, `AttemptCreateRequest` (khác danh sách trong DATABASE_DESIGN §3.4) cộng `challenge_attempt`, `boss_encounter` cho mode `challenge` (ACTIVITY_ENGINE §6.1, §9: `challenge_attempt_id` hoặc `boss_encounter_id` + `phase`) [Confirmed] — task cập nhật DATABASE_DESIGN.

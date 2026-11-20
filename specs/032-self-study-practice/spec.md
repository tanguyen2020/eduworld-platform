# Feature Specification: F032 — Chế độ luyện tập, hỗ trợ tự học (gợi ý, chẩn đoán, `rapid_guess`), gợi ý ôn tập tự động

**Feature Branch**: `032-self-study-practice` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C3 · **Phụ thuộc**: F031
**Input**: Học sinh tự học không có người lớn theo dõi (ADR-0072): tự luyện có phản hồi ngay, gợi ý nhiều cấp, lọc đoán bừa, ôn câu sai, luyện trộn, bài chẩn đoán và gợi ý ôn tập tự động có giải thích. FR nguồn: FR-EXR-001, FR-EXR-003, FR-EXR-012…014, FR-PRG-004…008, NFR-A11Y-002.

## Clarifications

### Session 2026-11-16

- Q: Rule gợi ý cần dữ liệu LMS/exam (R1 bài giao sắp hạn, R5 sau exam, R6 lịch LMS) thuộc feature nào? → A: F032 làm rule engine + R2, R3, R4, R7, R8, R9; R1 do F041, R5 do F042, R6 do F040 đăng ký vào engine ([LEARNING_ANALYTICS §6](../../docs/02-learning/LEARNING_ANALYTICS.md#6-recommendation-rules)).
- Q: Trọng số `w_hint`, `w_guess` và loại `rapid_guess` khỏi mastery do F031 hay F032? → A: F031 dựng `MasteryModel` v1; F032 bổ sung các trọng số này thành `model_version` mới theo [LEARNING_ANALYTICS §3.1](../../docs/02-learning/LEARNING_ANALYTICS.md#31-định-nghĩa) (FR-PRG-006, FR-EXR-012).
- Q: Mode `assignment`, `exam`, `quest` có trong F032? → A: Không. F032 chỉ phủ mode `practice`, `retry_wrong`, `diagnostic` của FR-EXR-001.

### Session 2026-11-20 (owner chốt — việc nhận từ OQ F030)

- Question set theo quy tắc trong `startAdHocAttempt` → **Chốt:** `source.kind = question_set` nhận cả bộ tĩnh và bộ theo luật: bộ theo luật được chọn câu lúc bắt đầu bằng `selection.Select` theo luật của version đã ghim, ghim `question_version_id` từng câu (R31); thiếu câu → `422` theo pool stats của F024.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tự luyện có phản hồi ngay (Priority: P1)

Học sinh vào Practice Zone chọn môn/chủ đề/LO, hệ thống chọn câu theo mức mastery hiện tại; mỗi câu được chấm ngay kèm giải thích và phản hồi theo phương án đã chọn, được thử lại theo khối lớp, không giới hạn thời gian.

**Why this priority**: Là vòng lặp tự học cơ bản; các story khác (gợi ý, ôn câu sai, gợi ý tự động) đều dẫn về attempt luyện tập.

**Independent Test**: `hs.an.3a` gọi `startAdHocAttempt` (`source.kind = selection`), trả lời đúng/sai từng câu, nhận `ActivityResponseResult` có `explanation`/`feedback`; render payload không chứa đáp án.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F032-AC-01 | KHI học sinh gọi `startAdHocAttempt` với `mode = practice` và `source.kind ∈ {selection, question_set}`, module learning PHẢI tạo attempt ghim `question_version_id` hiện hành của từng câu và trả render payload không chứa `answer_key`, `scoring_rule`, `explanation`, `options[].feedback`, `hints` | FR-EXR-001 · AC-EXR-001 · [ACTIVITY_ENGINE §9](../../docs/02-learning/ACTIVITY_ENGINE.md#9-api) | TC-LRN-002, TC-LRN-007 |
| F032-AC-02 | KHI một lần trả lời có tính điểm được chấm trong attempt có `FeedbackTiming = immediate`, HỆ THỐNG PHẢI trả `is_correct`, `explanation` và `feedback` của đúng phương án đã chọn trong `ActivityResponseResult` | FR-EXR-003 · [ACTIVITY_ENGINE §8.2](../../docs/02-learning/ACTIVITY_ENGINE.md#82-phản-hồi-theo-phương-án-nhiễu-confirmed) | TC-LRN-012 |
| F032-AC-03 | Ở NƠI số lượt thử theo `attempts_per_question` còn, HỆ THỐNG PHẢI nhận lần thử lại với `is_scored = false` và giữ điểm, evidence của lần trả lời đầu; NẾU vượt số lượt, THÌ HỆ THỐNG PHẢI trả `409 ANSWER_LIMIT_REACHED` | [LEARNING_EXPERIENCE §4.2](../../docs/02-learning/LEARNING_EXPERIENCE.md#42-số-lượt-thử-confirmed) | — |
| F032-AC-04 | HỆ THỐNG PHẢI không đặt `deadline_at` và không tính điểm theo tốc độ cho attempt `practice`, `retry_wrong`, `diagnostic` | NFR-A11Y-002 | — |
| F032-AC-05 | KHI `source.kind = selection` không chỉ định `difficulty`, HỆ THỐNG PHẢI chọn độ khó theo level mastery hiện tại (`beginning` 1–2, `developing` 2–3, `proficient` 3–4, `mastered` 4–5) bằng PRNG seed của attempt để tái lập được | [LEARNING_EXPERIENCE §5](../../docs/02-learning/LEARNING_EXPERIENCE.md#5-scaffolding) · [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện) | — |
| F032-AC-06 | NẾU pool vẫn thiếu câu sau khi nới `avoid_recent` và độ khó ±1, THÌ HỆ THỐNG PHẢI trả `422 QUESTION_POOL_INSUFFICIENT` và không tạo attempt | [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện) | — |
| F032-AC-07 | NẾU body chứa chuỗi vượt `maxLength`, enum sai hoặc `question_count` ngoài 1–50, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` chỉ rõ trường; trường `score`/`is_correct` do client gửi PHẢI bị bỏ qua | AC-EXR-002 · ADR-0041 | TC-TEN-024, TC-LRN-007 |
| F032-AC-08 | KHI học sinh gọi start lặp lại cùng `Idempotency-Key`, HỆ THỐNG PHẢI trả đúng attempt đã tạo; KHI đã có attempt `in_progress` cùng nguồn trong `attempt_resume_window`, HỆ THỐNG PHẢI trả attempt đó để resume | [ACTIVITY_ENGINE §6.3](../../docs/02-learning/ACTIVITY_ENGINE.md#63-resume) | TC-LRN-009 |
| F032-AC-09 | HỆ THỐNG PHẢI chỉ cho principal `user_type = student` gọi các route học sinh của feature này và chỉ trên dữ liệu của chính mình trong tenant của phiên; nhân sự gọi PHẢI nhận `403 PERMISSION_DENIED`; header `X-Tenant-Id` khác tenant phiên PHẢI nhận `403 TENANT_MISMATCH` | AC-AUTH-024 · AC-ORG-001 | TC-TEN-001, TC-TEN-011 |

---

### User Story 2 — Gợi ý nhiều cấp trong attempt (Priority: P1)

Học sinh lớp 3 bí ở một câu, bấm "Gợi ý" để mở lần lượt gợi ý cấp 1, cấp 2; dùng gợi ý không mất điểm, chỉ giảm trọng số evidence mastery.

**Why this priority**: Gợi ý là công cụ chính để học sinh tự vượt khó khi không có người hướng dẫn (US-EXR-003).

**Independent Test**: Trong attempt practice, gọi `revealNextHint` hai lần rồi trả lời đúng → `question.answered.hints_used = 2`; lần gọi thứ `total_levels + 1` → `HINTS_EXHAUSTED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F032-AC-10 | KHI học sinh gọi `revealNextHint` cho item của attempt mình đang `in_progress`, module learning PHẢI trả `{level, total_levels, hint}` của cấp kế tiếp theo thứ tự `hints[]` trong version đã ghim và tăng `attempt_items.hints_revealed` đúng 1 | FR-EXR-012 · US-EXR-003 · [ACTIVITY_ENGINE §8.3](../../docs/02-learning/ACTIVITY_ENGINE.md#83-gợi-ý-trong-attempt-confirmed) | TC-LRN-011 |
| F032-AC-11 | NẾU attempt có mode ∈ {`exam`, `diagnostic`, `competition`} hoặc `FeedbackTiming ≠ immediate`, THÌ HỆ THỐNG PHẢI trả `403 HINTS_NOT_ALLOWED` | FR-EXR-012 | TC-LRN-011 |
| F032-AC-12 | NẾU đã mở hết cấp gợi ý của item (kể cả item không có `hints[]`), THÌ HỆ THỐNG PHẢI trả `409 HINTS_EXHAUSTED` | FR-EXR-012 | TC-LRN-011 |
| F032-AC-13 | NẾU attempt không còn `in_progress`, THÌ HỆ THỐNG PHẢI trả `409 ATTEMPT_NOT_IN_PROGRESS`; NẾU attempt thuộc học sinh khác hoặc tenant khác, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND` không lộ sự tồn tại | AC-ORG-001 | TC-LRN-011, TC-TEN-001 |
| F032-AC-14 | KHI ghi một lần trả lời có tính điểm, HỆ THỐNG PHẢI lưu `hints_used` = số cấp đã mở **trước** lần trả lời đó vào `attempt_answers` và learning event `question.answered`; gợi ý mở sau lần trả lời có tính điểm PHẢI không đổi evidence của lần đó | AC-EXR-007 · [ACTIVITY_ENGINE §8.3](../../docs/02-learning/ACTIVITY_ENGINE.md#83-gợi-ý-trong-attempt-confirmed) | TC-LRN-011 |
| F032-AC-15 | KHI `revealNextHint` được gửi lại cùng `Idempotency-Key`, HỆ THỐNG PHẢI trả lại cùng cấp gợi ý và không mở thêm cấp | [API_DESIGN §6](../../docs/07-backend/API_DESIGN.md#6-idempotency) | — |

---

### User Story 3 — Lọc đoán bừa và trọng số evidence (Priority: P1)

Học sinh bấm bừa thật nhanh: điểm bài vẫn ghi, nhưng câu đó không làm sai lệch mastery; câu đúng nhờ gợi ý hoặc câu dễ đoán trúng mang ít thông tin hơn.

**Why this priority**: Mastery sai lệch làm hỏng gợi ý tự động và dashboard nhà trường (AC-PRG-002).

**Independent Test**: Trả lời `single_choice` sau 800 ms → `rapid_guess = true`, điểm attempt không đổi, `objective_masteries` không đổi; golden test bổ sung ở LEARNING_ANALYTICS §3.3 khớp.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F032-AC-16 | KHI ghi lần trả lời có tính điểm, HỆ THỐNG PHẢI tính `response_ms = min(client_elapsed_ms, server_interval_ms)` (trừ thời gian pause) và gắn `rapid_guess = true` khi `response_ms` nhỏ hơn `min_response_ms` của câu hoặc ngưỡng mặc định `rapid_guess_min_response_ms` theo loại câu (nhân hệ số cho `g1_3`) | FR-PRG-006 · AC-PRG-002 · [ACTIVITY_ENGINE §11.1](../../docs/02-learning/ACTIVITY_ENGINE.md#111-cờ-rapid_guess--trả-lời-quá-nhanh-để-kịp-đọc-đề-confirmed) | TC-LRN-013 |
| F032-AC-17 | HỆ THỐNG PHẢI giữ nguyên điểm attempt khi lần trả lời có `rapid_guess = true` hoặc `hints_used > 0` | [ACTIVITY_ENGINE §7.2](../../docs/02-learning/ACTIVITY_ENGINE.md#72-tính-điểm) | TC-LRN-013 |
| F032-AC-18 | KHI cập nhật mastery từ `question.answered`, learning_record PHẢI bỏ qua evidence `rapid_guess` (không đổi `M`, `n`, ngày cho `d_min`) và với lần trả lời đúng PHẢI nhân `w_hint(hints_used)` và `w_guess = 1 − c(type)` theo `guess_coefficient` | FR-PRG-006 · FR-EXR-012 · AC-PRG-001 · [LEARNING_ANALYTICS §3.1](../../docs/02-learning/LEARNING_ANALYTICS.md#31-định-nghĩa) | TC-LRN-010, TC-LRN-013 |
| F032-AC-19 | Ở NƠI attempt có mode `minigame` hoặc `competition`, HỆ THỐNG PHẢI không áp ngưỡng mặc định theo loại câu mà dùng ngưỡng trong cấu hình mini-game/trận | [ACTIVITY_ENGINE §11.1](../../docs/02-learning/ACTIVITY_ENGINE.md#111-cờ-rapid_guess--trả-lời-quá-nhanh-để-kịp-đọc-đề-confirmed) | — |
| F032-AC-20 | HỆ THỐNG PHẢI không trả cờ `rapid_guess` hay cảnh báo "làm quá nhanh" trong response cho học sinh | [ACTIVITY_ENGINE §11.1](../../docs/02-learning/ACTIVITY_ENGINE.md#111-cờ-rapid_guess--trả-lời-quá-nhanh-để-kịp-đọc-đề-confirmed) · AGENTS R19 | — |

---

### User Story 4 — Ôn câu sai và luyện trộn chủ đề (Priority: P2)

Câu sai lượt đầu vào danh sách ôn theo lịch 1–3–7 ngày; khi ôn, học sinh nhận câu "anh em" thay vì lặp đúng câu cũ. Sau khi đã học vài chủ đề, học sinh luyện xen kẽ nhiều chủ đề.

**Why this priority**: Củng cố trí nhớ dài hạn (US-EXR-001), cần US1 chạy trước.

**Independent Test**: Làm sai một câu có ≥ 1 câu anh em đã publish → `listMyWrongQuestions` có câu đó; `startAdHocAttempt` `retry_wrong` nhận câu anh em khác `question_id`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F032-AC-21 | KHI lần trả lời đầu có tính điểm của một câu sai (không `rapid_guess`), HỆ THỐNG PHẢI đưa câu vào danh sách ôn với hạn ôn theo `retry_wrong_schedule`; KHI học sinh đúng 2 lần liên tiếp ở 2 ngày khác nhau, HỆ THỐNG PHẢI đưa câu ra khỏi danh sách | US-EXR-001 · [LEARNING_EXPERIENCE §6.1](../../docs/02-learning/LEARNING_EXPERIENCE.md#61-ôn-câu-sai-retry_wrong) | TC-LRN-005 |
| F032-AC-22 | KHI học sinh gọi `startAdHocAttempt` với `source.kind = retry_wrong`, HỆ THỐNG PHẢI tạo attempt `mode = retry_wrong` chọn câu anh em (cùng ≥ 1 LO, cùng loại, cùng độ khó, khác `question_id`, chưa gặp trong cửa sổ `avoid_recent`), nới độ khó ±1 khi thiếu và chỉ dùng lại câu gốc khi hết câu anh em | FR-EXR-013 · [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện) | TC-LRN-005, TC-LRN-013 |
| F032-AC-23 | `listMyWrongQuestions` PHẢI chỉ trả câu trong danh sách ôn của chính học sinh, kèm hạn ôn, không chứa đáp án hay giải thích | FR-EXR-013 · ADR-0016 | TC-TEN-001 |
| F032-AC-24 | KHI `source.kind = selection` có `interleave = true` và ≥ 2 topic đã học của cùng môn, HỆ THỐNG PHẢI chia câu theo `interleave_mix` và không xếp quá số câu liên tiếp cùng topic quy định; NẾU mode là `exam` hoặc `diagnostic`, THÌ HỆ THỐNG PHẢI từ chối `interleave` bằng `422 VALIDATION_FAILED` | FR-EXR-014 · [LEARNING_EXPERIENCE §5.2](../../docs/02-learning/LEARNING_EXPERIENCE.md#52-luyện-trộn-chủ-đề-interleaving-confirmed) | — |

---

### User Story 5 — Bài chẩn đoán đặt mastery ban đầu (Priority: P2)

Đầu năm học hoặc đầu chủ đề, học sinh làm bài khởi động ngắn; kết quả đặt mức khởi đầu theo từng LO để không phải làm lại thứ đã biết và không bỏ sót LO tiên quyết yếu.

**Why this priority**: Giúp gợi ý tự động chọn đúng điểm bắt đầu (R3, R7), cần mastery từ F031.

**Independent Test**: `hs.binh.3a` chưa có evidence làm bài chẩn đoán Toán 3 → các LO có ≥ 2 câu nhận `M0` theo công thức, level vẫn `not_started`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F032-AC-25 | KHI học sinh gọi `startAdHocAttempt` với `source.kind = diagnostic` (`subject_id` + `grade_level` hoặc `topic_id`), HỆ THỐNG PHẢI tạo attempt `mode = diagnostic` gồm LO trọng tâm và LO tiên quyết, số câu tối đa và số câu mỗi LO theo `diagnostic_prior`, `FeedbackTiming = after_submit`, không gợi ý | FR-PRG-007 · [LEARNING_ANALYTICS §3.5](../../docs/02-learning/LEARNING_ANALYTICS.md#35-bài-chẩn-đoán-đặt-m0-ban-đầu-confirmed) | TC-LRN-014 |
| F032-AC-26 | KHI attempt `diagnostic` chuyển `graded`, learning_record PHẢI đặt `M0` cho mỗi LO có ≥ 2 câu được chấm (đã loại `rapid_guess`) và chưa có evidence theo `diagnostic_prior`, lan `M0` tới LO tiên quyết chưa có evidence, ghi `prior_source = diagnostic` và không đổi `n`, `d_min`, level | FR-PRG-007 | TC-LRN-014 |
| F032-AC-27 | HỆ THỐNG PHẢI phát event của attempt `diagnostic` với `context.mode = diagnostic` để consumer loại khỏi bảng xếp hạng, báo cáo kết quả bài giao và XP theo điểm | FR-PRG-007 · [EVENT_DESIGN §5.1](../../docs/07-backend/EVENT_DESIGN.md#51-trường-bổ-sung-bắt-buộc) | — |
| F032-AC-28 | KHI học sinh làm lại bài chẩn đoán, HỆ THỐNG PHẢI giữ nguyên lịch sử attempt và chỉ áp `M0` mới cho LO vẫn chưa có evidence | FR-PRG-007 | TC-LRN-014 |

---

### User Story 6 — Gợi ý ôn tập tự động có giải thích (Priority: P1)

Mây (NPC dẫn đường) luôn cho học sinh tối đa 3 gợi ý "nên học gì tiếp" kèm lý do dễ hiểu; đây là con đường học chính thay cho chỉ định của giáo viên.

**Why this priority**: ADR-0072 xác định gợi ý tự động là kênh chính (FR-PRG-008, US-PRG-001).

**Independent Test**: Học sinh có LO `developing` với ≥ 3 evidence và LO tiên quyết `proficient` → `getMyRecommendations` trả gợi ý rule R2 có `recommendation_id` và lý do; bắt đầu attempt với `recommendation_id` → gợi ý chuyển `started`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F032-AC-29 | KHI học sinh gọi `getMyRecommendations`, HỆ THỐNG PHẢI trả tối đa 3 gợi ý `active` sắp theo ưu tiên rule, mỗi gợi ý có `recommendation_id`, `rule_code`, `kind`, `target_id` và lý do song ngữ vi/en | FR-PRG-004 · FR-PRG-008 · US-PRG-001 | — |
| F032-AC-30 | KHI có `question.answered`, `activity.submitted`, `activity.graded`, `attempt.re_evaluated` của học sinh hoặc job hằng đêm chạy, worker PHẢI tính lại gợi ý theo rule R2, R3, R4, R7, R8, R9 và phát `learning.recommendation_generated` khi danh sách thay đổi | FR-PRG-004 · [LEARNING_ANALYTICS §6](../../docs/02-learning/LEARNING_ANALYTICS.md#6-recommendation-rules) | — |
| F032-AC-31 | HỆ THỐNG PHẢI chỉ gợi ý nội dung đã publish và khả dụng cho tenant (riêng tenant hoặc platform được gán); NẾU học sinh bỏ qua cùng target 3 lần, THÌ HỆ THỐNG PHẢI không gợi ý lại target đó | FR-PRG-008 · ADR-0029 | — |
| F032-AC-32 | KHI start attempt có `context.recommendation_id` thuộc đúng học sinh, HỆ THỐNG PHẢI lưu `attempts.recommendation_id`, chuyển gợi ý sang `started` và đưa id vào domain event `activity.started` (`recommendation_id`, payload owner F030); NẾU id không hợp lệ hoặc của học sinh khác, THÌ HỆ THỐNG PHẢI bỏ qua id mà không trả lỗi | [ACTIVITY_ENGINE §9](../../docs/02-learning/ACTIVITY_ENGINE.md#9-api) | — |
| F032-AC-33 | Rule engine PHẢI cho phép feature khác đăng ký rule mới (R1, R5, R6) qua interface chung mà không sửa engine và các rule đã có | [LEARNING_EXPERIENCE §6.2](../../docs/02-learning/LEARNING_EXPERIENCE.md#62-recommendation-release-10-rule-based) | — |

---

### User Story 7 — Học sinh xem tiến độ dạng thân thiện (Priority: P3)

Học sinh xem mastery của mình theo LO bằng biểu tượng phù hợp độ tuổi; lớp 6–9 thấy thêm phần trăm và xu hướng.

**Why this priority**: Tạo động lực, nhưng game client (F063) vẫn chạy được với US1–US6 trước.

**Independent Test**: `hs.an.3a` gọi `getMyMastery` → level + biểu tượng, không có giá trị số; `hs.dung.7a` → có giá trị số.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F032-AC-34 | KHI học sinh gọi `getMyProgress` hoặc `getMyMastery`, HỆ THỐNG PHẢI trả level và mã biểu tượng theo grade band, chỉ trả giá trị mastery dạng số cho khối 6–9, chỉ dữ liệu của chính học sinh và không có nhãn mức TT27/TT22 hay so sánh với bạn khác | FR-PRG-005 · [LEARNING_EXPERIENCE §7](../../docs/02-learning/LEARNING_EXPERIENCE.md#7-hiển-thị-tiến-bộ-cho-học-sinh) | TC-TEN-001 |

### Edge Cases

- NẾU client gửi `client_elapsed_ms` lớn hơn khoảng server đo, THÌ HỆ THỐNG PHẢI dùng khoảng server (F032-AC-16); thời gian pause không tính vào `response_ms`.
- NẾU nội dung được publish/rollback khi học sinh đang làm, THÌ attempt PHẢI tiếp tục trên version đã ghim, kể cả `hints[]` (F032-AC-10, ADR-0043).
- NẾU cùng event `question.answered` giao hai lần, THÌ mastery và danh sách ôn PHẢI chỉ cập nhật một lần (`infra.processed_events`).
- NẾU LO trong bài chẩn đoán có < 2 câu được chấm, THÌ LO đó giữ `M0` mặc định (F032-AC-26).
- NẾU học sinh tenant chung (`common`) luyện tập, THÌ pool chỉ gồm nội dung platform được gán cho `common` (AC-ORG-007).
- NẾU học sinh chuyển lớp, THÌ danh sách ôn và gợi ý giữ theo học sinh, không theo lớp.
- NẾU re-evaluate làm một câu từ sai thành đúng, THÌ câu PHẢI rời danh sách ôn và gợi ý được tính lại (F032-AC-30).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-EXR-001 | Mode `practice`, `retry_wrong`, `diagnostic` (mode khác thuộc F030/F041/F042/F072) | [ACTIVITY_ENGINE §6.1](../../docs/02-learning/ACTIVITY_ENGINE.md#61-mode--hoạt-động-được-gọi-từ-đâu-trong-game) |
| FR-EXR-003 | Phản hồi đúng/sai + giải thích + phản hồi theo phương án ngay sau mỗi câu ở practice | [ACTIVITY_ENGINE §8.1](../../docs/02-learning/ACTIVITY_ENGINE.md#81-feedbacktiming-và-resultrelease) |
| FR-EXR-012 | `revealNextHint`, `hints_used`, trọng số `w_hint` | [ACTIVITY_ENGINE §8.3](../../docs/02-learning/ACTIVITY_ENGINE.md#83-gợi-ý-trong-attempt-confirmed) |
| FR-EXR-013 | Danh sách ôn câu sai, chọn câu anh em | [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện) |
| FR-EXR-014 | Luyện trộn chủ đề | [LEARNING_EXPERIENCE §5.2](../../docs/02-learning/LEARNING_EXPERIENCE.md#52-luyện-trộn-chủ-đề-interleaving-confirmed) |
| FR-PRG-004 | API và worker gợi ý ôn tập theo LO yếu | [LEARNING_ANALYTICS §6](../../docs/02-learning/LEARNING_ANALYTICS.md#6-recommendation-rules) |
| FR-PRG-005 | Tiến độ thân thiện theo độ tuổi | [LEARNING_EXPERIENCE §7](../../docs/02-learning/LEARNING_EXPERIENCE.md#7-hiển-thị-tiến-bộ-cho-học-sinh) |
| FR-PRG-006 | Cờ `rapid_guess`, loại khỏi mastery, hệ số đoán trúng | [LEARNING_ANALYTICS §3.1](../../docs/02-learning/LEARNING_ANALYTICS.md#31-định-nghĩa) |
| FR-PRG-007 | Bài chẩn đoán đặt `M0` | [LEARNING_ANALYTICS §3.5](../../docs/02-learning/LEARNING_ANALYTICS.md#35-bài-chẩn-đoán-đặt-m0-ban-đầu-confirmed) |
| FR-PRG-008 | Gợi ý tự động có lý do là con đường học chính | [LEARNING_ANALYTICS §6](../../docs/02-learning/LEARNING_ANALYTICS.md#6-recommendation-rules) |
| NFR-A11Y-002 | Practice/retry/diagnostic không giới hạn thời gian | [LEARNING_EXPERIENCE §10](../../docs/02-learning/LEARNING_EXPERIENCE.md#10-accessibility-và-hòa-nhập) |

### Key Entities

- **Attempt (mode practice/retry_wrong/diagnostic)**: lượt làm ad-hoc, ghim version từng câu, có `recommendation_id` tùy chọn; dùng entity Attempt của F030.
- **HintReveal**: số cấp gợi ý đã mở của một item trong attempt; `hints_used` ghi trên từng lần trả lời.
- **Cờ rapid_guess**: thuộc tính của lần trả lời có tính điểm; không phải thực thể riêng.
- **WrongQuestionEntry**: câu sai lượt đầu của học sinh, gắn LO, hạn ôn kế tiếp, chuỗi đúng liên tiếp.
- **Recommendation**: gợi ý cho học sinh (rule, loại, target, LO, lý do, ưu tiên, trạng thái, số lần bỏ qua).
- **Diagnostic prior**: `M0` theo LO gắn trên ObjectiveMastery cùng nguồn (`prior_source`, attempt chẩn đoán).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `attempts_per_question` | Số lượt thử mỗi câu theo grade band |
| `attempt_resume_window`, `attempt_idle_timeout` | Resume và tự pause attempt ad-hoc |
| `w_hint`, `guess_coefficient`, `mastery_model` | Trọng số evidence khi dùng gợi ý, hệ số đoán trúng, version mô hình |
| `rapid_guess_min_response_ms` | Ngưỡng mặc định theo loại câu |
| `retry_wrong_schedule` | Lịch ôn câu sai |
| `interleave_mix` | Tỉ lệ và số câu liên tiếp khi luyện trộn |
| `diagnostic_prior` | Số câu tối đa, số câu/LO, công thức `M0` |
| `answer_rate`, `attempt_start_rate` | Rate limit trả lời/start |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không gửi đáp án, giải thích, phản hồi phương án, gợi ý chưa mở xuống client | AGENTS R5, R6 · ADR-0016 |
| Lượt làm ghim version, đọc gợi ý theo version đã ghim | AGENTS R11, R31 · ADR-0043 |
| Không log câu trả lời, nội dung gợi ý ở mức INFO | AGENTS R14 |
| Không giới hạn thời gian cho practice | NFR-A11Y-002 |
| Mọi truy vấn qua `TenantScope` + `deleted_at IS NULL` | AGENTS R8, R27 · ADR-0005, ADR-0039 |
| Không cảnh báo gây xấu hổ, không áp lực | AGENTS R19 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% render payload và response trước khi chấm trong bộ contract test không chứa đáp án, giải thích, phản hồi phương án hay gợi ý chưa mở.
- **SC-002**: Golden test mastery (bảng và ví dụ bổ sung LEARNING_ANALYTICS §3.3) khớp tuyệt đối; 0 evidence `rapid_guess` làm đổi mastery trong bộ dữ liệu kiểm thử.
- **SC-003**: Với bộ dữ liệu có ≥ 1 câu anh em published, 100% lượt `retry_wrong` nhận câu khác `question_id` gốc.
- **SC-004**: 100% học sinh có ít nhất một LO thỏa điều kiện R2/R3/R4/R7/R8/R9 nhận ≥ 1 gợi ý có lý do sau lần tính lại kế tiếp.
- **SC-005**: Kịch bản quickstart chạy hết với tài khoản demo, mọi `F032-AC-*` có test pass.

## Ngoài phạm vi

- API attempt lõi (start từ activity, responses, submit, review, resume) — F030; mastery v1, recompute, re-evaluate — F031.
- Mode `assignment` và rule R1 — F041; exam và rule R5 — F042; lịch LMS và rule R6 — F040.
- Hiển thị "làm quá nhanh" và hiệu quả gợi ý trên School Learning Dashboard — F043; item analysis dùng `rapid_guess`/`hints_used` — F044.
- Giao diện gợi ý, nút "Gợi ý", NPC Mây trong Godot — F062, F063; miễn giảm XP khi ôn LO tiên quyết — F074.
- Recommendation dựa trên ML; kỹ năng viết/nói — deferred (ADR-0082).

## Assumptions

- Ngưỡng `rapid_guess`, `w_hint`, `guess_coefficient`, `diagnostic_prior`, `interleave_mix` lấy từ cấu hình platform (Confirmed trong CONFIG_DEFAULTS); lịch ôn, độ khó thích ứng và rule gợi ý là [Confirmed].
- F031 đã có `MasteryModel` v1, consumer mastery và bảng cấu hình mô hình; F032 thêm version cấu hình mới, không đổi model cũ.
- Lý do gợi ý là mã + bản dịch trong file dịch của module (song ngữ vi/en, ADR-0056); không sinh văn bản tự do.
- Seed GDPT Toán lớp 1–5 (OQ-08 [Confirmed]) đủ câu anh em và LO tiên quyết cho kịch bản demo.

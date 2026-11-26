# Feature Specification: F076 — Challenge Engine solo, 5 mức khó, room host dùng chung

**Feature Branch**: `076-challenge-engine-solo` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `game` (+ `src/backend/pkg/roomhost`) · **Mốc**: M3 · **Phụ thuộc**: F072, F074
**Input**: Challenge PvE một người chơi trên hoạt động học tập thật: các loại `time`, `perfect`, `survival`, `puzzle`, `knowledge`, `weekly`, `seasonal`; năm mức khó ánh xạ dải độ khó câu trong khối của học sinh; mỗi hoạt động là attempt học tập cá nhân do module learning chấm; thời gian theo giờ server; giới hạn lượt/ngày, cooldown; thưởng qua Reward Engine. Dựng thư viện room host dùng chung `src/backend/pkg/roomhost` (lease owner, timer, checkpoint, grace) cho boss (F103) và battle (F105). FR nguồn: FR-CHL-001, FR-CHL-003.

## Clarifications

### Session 2026-11-16

- Q: Loại nào thuộc F076? → A: Bảy loại chơi một mình (`time`, `perfect`, `survival`, `puzzle`, `knowledge`, `weekly`, `seasonal`). `boss`, `coop`, `party` → F103; `tournament` (vòng PvE của giải) → F107.
- Q: Câu hỏi theo mức khó lấy thế nào? → A: module game gọi `createAttempt` (learning internal) với mode `challenge`, dải độ khó của mức và khối học sinh; module learning chọn câu và chấm (ADR-0016). Contract internal hiện chỉ cho `quest`/`minigame` → delta "Cần sửa" ([contracts](contracts/README.md) §6).
- Q: "Room host dùng chung" nghĩa là gì ở F076? → A: F076 tạo `src/backend/pkg/roomhost` (không logic nghiệp vụ) và dùng nó cho timer/lease/checkpoint của challenge solo; F103 (boss) và F105 (battle) dùng lại. Lựa chọn ghi ở [research.md](research.md).
- Q: Mỗi "hoạt động" có bao nhiêu câu? → A: Theo `activity_source` của challenge version; loại `survival` dùng một câu mỗi hoạt động để trừ sức bền theo từng câu.

### Session 2026-11-24 (triển khai)

Quyết định khi triển khai, lấy phương án an toàn nhất; tất cả [Confirmed] theo lệnh thường trực "chốt hết" của owner.

- Q: `startGameChallenge` trả "hoạt động đầu tiên" thế nào khi `GameChallengeAttempt` không có field hoạt động? → A: `startGameChallenge` chỉ tạo và trả attempt (không gọi module learning, không bao giờ `503`); hoạt động đầu tiên lấy bằng `nextChallengeActivity`. Không đổi contract. [Confirmed]
- Q: Module learning chọn câu theo mức khó thế nào? → A: Mode `challenge` bắt buộc `difficulty_min/max`, `grade_level` (khớp khối học sinh ở `ref.students` của module learning, khác → `422`), `objective_ids` tùy chọn. Module learning chọn câu đã publish của pool tenant theo môn của activity, đúng khối, trong dải độ khó, thuộc LO của challenge và loại câu activity hiển thị được — thay cho item riêng của activity (activity chỉ là định dạng). Chọn nghiêm ngặt: chỉ nới `avoid_recent`, không nới dải độ khó ±1 hay loại câu như luyện tập (QUESTION_BANK §6). Pool không đủ → `422 QUESTION_POOL_INSUFFICIENT`; activity media/mô phỏng → `422 ACTIVITY_CONFIG_INVALID`. Không bao giờ phát câu khối khác hoặc ngoài dải (SC-002). [Confirmed]
- Q: Số câu và thời gian mỗi hoạt động? → A: Số câu tự nhiên của activity; `survival` một câu với `time_limit_seconds` = `seconds_per_item` của mức (bảng `difficulty_levels` khi profile không ghi) × 1,5 cho khối 1–3, tối thiểu 10 s. Các loại khác dùng `deadline_at` của challenge. [Confirmed]
- Q: Nguồn `question_selection` và kiểm "hoạt động đã publish"? → A: Chỉ nhận `activity_ref_list` (`createAttempt` chỉ mở learning activity — tiền lệ F072); `question_selection` → `422 VALIDATION_FAILED` (`activity_source/kind`). Module game không có đường đọc đồng bộ nội dung learning (R29) nên khi publish chỉ kiểm định dạng id; activity chưa publish bị chặn ở `nextChallengeActivity` (`422 ITEM_REF_INVALID`) và ở `submit_review` của F078 (F078-AC-07). Quest/vùng trong `unlock_rule` và `area_id` kiểm khi publish (`422 ITEM_REF_INVALID`). [Confirmed]
- Q: Luật theo loại? → A: `time`, `survival`, `puzzle`, `weekly`, `seasonal` lặp danh sách hoạt động; `perfect`, `knowledge` đi một lượt. `score` = số câu đúng do module learning chấm. `goal.metric`: `correct_answers` (điểm), `completed_activities`/`rounds_survived` (số hoạt động đã chấm), `puzzle_solved` (hoạt động `passed`), `accuracy_ratio` (chỉ `perfect`/`knowledge`, ≤ 1; mục tiêu 1 → câu sai đầu tiên là `failed`, còn lại đánh giá khi hết bộ). `collective_correct_answers` chỉ cho `coop`/`party`. Hết bộ chưa đạt → `failed` (`activities_exhausted`). Survival trừ 1 sức bền mỗi câu sai, hồi 1 sau `challenge_survival_regen_streak` (3) hoạt động đúng hết liên tiếp, không vượt sức bền của mức. [Confirmed]
- Q: Hạn chót và tiếp tục? → A: `deadline_at` = `time_limit_seconds` × 1,5 cho khối 1–3; challenge không có `time_limit_seconds` không có hạn chót và tiếp tục trong `attempt_resume_window` tính từ `started_at`, sau đó `abandoned` (`resume_expired`). Kết quả chấm chỉ được tính khi `occurred_at` ≤ `deadline_at` + `deadline_grace`; timer room host và job `challenge-deadline` (15 s) kết thúc tại hạn + `deadline_grace`. `nextChallengeActivity` sau hạn → `422 INVALID_STATE_TRANSITION`. [Confirmed]
- Q: Đếm lượt và nghỉ chờ? → A: Lượt = mọi attempt của (học sinh, challenge, `period_key`) kể cả `abandoned` (bỏ cuộc không phải lượt miễn phí; F076 không có "abandoned do lỗi hệ thống"). Kỳ: ngày theo `quest_reset_time` (04:00 ICT), tuần ISO cho `weekly`, cả cửa sổ `availability` cho `seasonal` (`S<ngày bắt đầu ICT>`); `retry_after` = lần reset kế tiếp hoặc hết mùa. Nghỉ chờ chỉ sau `failed`: `retry.cooldown_seconds_after_fail` của version, không có thì `challenge_fail_cooldown` từ mức Hard. [Confirmed]
- Q: Đang có challenge `in_progress` (kể cả cùng challenge, key khác)? → A: `409 CONFLICT` kèm `challenge_attempt_id` để client tiếp tục bằng `getGameChallengeAttempt`. [Confirmed]
- Q: "Chưa gán" (AC-02) nghĩa là gì? → A: Challenge là tài nguyên platform gán cho tenant qua `infra.tenant_resource_assignments` (`resource_type = challenge`, ADR-0029); chưa gán hoặc không chơi một mình được (`boss`/`coop`/`party`/`tournament`, `party_required`) → `404`. Sai khối/chưa mở khóa/ngoài thời gian → `404` khi đọc, `403 CHALLENGE_NOT_ELIGIBLE` khi bắt đầu (AC-05). [Confirmed]
- Q: Phần thưởng? → A: Reward Engine (F074) thêm trigger `challenge.completed`: mặc định chỉ thưởng `succeeded`; `failed` chỉ khi rule ghi `conditions.challenge_statuses`; `abandoned` không bao giờ; không bao giờ trừ XP/Xu. `grants[].scale_by = reward_multiplier` nhân XP/Xu theo mức trước giảm dần và cap (cap không bị bỏ qua). `first_completion_only` của trigger này cần `first_clear` của payload (lần `succeeded` đầu tiên của học sinh trên mọi version). Khóa ledger theo `challenge_attempt_id` → gửi lặp thưởng một lần (SC-005). `rewards.first_clear` của cấu hình chỉ để hiển thị (`first_clear_rewards`); cấp thưởng luôn qua reward rule. [Confirmed]
- Q: Room host dùng chung? → A: `src/backend/pkg/roomhost` thêm `Host` (lease độc quyền, quét orphan, timer theo `pkg/clock`, grace theo loại room, checkpoint qua port của module) và `ValkeyRegistry` (`ew:rt:room:<id>`, `:lease` SET NX PX 10 s gia hạn 3 s, index orphan riêng mỗi module `ew:rt:rooms:game`). Room `challenge_solo` chạy trong worker profile `events` (`game.challenge_room_host`). Room host hub/battle của module competition giữ registry riêng (gia hạn cả member) — F103/F105 chuyển sang `roomhost.Host` khi cần, F076 không sửa module competition. [Confirmed]
- Q: Học sinh bị khóa/dọn? → A: Consumer `game.session_end_on_revoke` cũng chuyển attempt `in_progress` thành `abandoned` (`student_unavailable`) với `user.access_revoked` (trừ `password_changed`), `user.status_changed` khóa/vô hiệu/ẩn danh và `student.cleaned_up`. [Confirmed]
- Q: F077 chờ F076 kiểm challenge trong sự kiện? → A: Publish sự kiện có `components.challenge_ids` không phải challenge đã publish (hoặc trong cùng lô) → `422 ITEM_REF_INVALID`. [Confirmed]
- Q: Seed challenge (T004)? → A: Dời: activity của learning dev seed sinh id lúc chạy nên chưa tham chiếu cố định được; test tích hợp dùng nội dung test (Câu hỏi mở F076-2). [Confirmed]

### Session 2026-11-26 (delta đọc cho F069, ma trận khối 1–9)

Quyết định khi triển khai delta F069 T004 và ma trận khối ([TEST_STRATEGY §6.10](../../docs/12-project/TEST_STRATEGY.md#610-ma-trận-khối-19-backend)); tất cả [Confirmed] theo lệnh thường trực "chốt hết" của owner.

- Q: `active_attempt_id` của `GameChallengeSummary` lấy ở đâu? → A: Attempt `in_progress` của chính học sinh cho challenge đó (tối đa một nhờ `challenge_attempts_one_active_uq`), đọc cùng truy vấn lượt đã dùng; `null` khi không có (khóa luôn có mặt). Có trên `listGameChallenges` và `getGameChallenge`. [Confirmed]
- Q: `difficulties[]` của `GameChallengeDetail` gồm những mức nào? → A: Mọi mức có profile trong version, dễ trước: mức chọn được (`available = true`, `locked_reason_code = null`, trùng `available_difficulties`) và mức khóa vì khối (`available = false`, `locked_reason_code = grade_not_allowed`). Khối 1–3 **không** nhận mức trên `g1_3_challenge_rules` (expert, legendary), kể cả dạng khóa — AC-UX-0601 "g1_3 không thấy expert/legendary" thắng mô tả "mức khóa hiện lý do" của S06 §6a (phương án an toàn hơn cho trẻ). `time_limit_seconds`, `time_per_activity_seconds` đã nhân hệ số khối 1–3 (`time_per_activity_seconds` tối thiểu 10 s như lượt survival), `lives` chỉ loại survival, `reward_multiplier` của profile. [Confirmed]
- Q: `not_unlocked`, `school_disabled`? → A: Có trong enum của contract nhưng F076 không có mở khóa theo mức hay công tắc trường theo mức, nên không bao giờ trả; chỉ `grade_not_allowed`. [Confirmed]
- Q: `description`, `rules_summary` đổi sang `LocalizedText`? → A: Không làm trong delta này: đổi kiểu `string` → object là thay đổi phá vỡ (oasdiff); `description` vẫn trả chuỗi theo `Accept-Language` — Câu hỏi mở F076-3. [Confirmed]

### Câu hỏi mở

- F076-1: Bước quest `challenge` (QUEST_SYSTEM) và QST tiêu thụ `challenge.completed` — ngoài AC của F076, chờ feature quest kế tiếp.
- F076-2: Seed challenge cho quickstart (T004, T039) — cần learning dev seed có id activity cố định.
- F076-3 — **Đã trả lời 2026-11-27 [Confirmed]:** thêm trường additive `description_i18n`, `rules_summary_i18n` — xem Session 2026-11-27.

### Session 2026-11-27 (delta F076-3 — [Confirmed — orchestrator 2026-11-27, lệnh "chốt hết": chỉ thêm trường])

- Q: Mô tả song ngữ cho client? → A: `GameChallengeDetail` thêm `description_i18n` và `rules_summary_i18n` (`LocalizedText`); `description`/`rules_summary` (chuỗi theo `Accept-Language`) giữ nguyên cho client cũ. `description_i18n` = `description` vi + en của version (văn bản game luôn đủ hai bản khi publish — CONTENT_MODEL §8; thiếu một bản thì không trả). `rules_summary_i18n` chưa có nguồn (`challenge.schema.json` không có tóm tắt luật) nên chưa trả, giống `rules_summary`. Quy tắc ở [CHALLENGE_BOSS_SYSTEM](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md). [Confirmed]

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Xem challenge khả dụng (Priority: P1)

Học sinh mở Núi Thử Thách, thấy các challenge hợp khối, đã mở khóa, đang mở, với mức khó chọn được và lượt thử còn lại.

**Why this priority**: Điểm vào của mọi challenge; lọc khối là ràng buộc học tập.

**Independent Test**: `hs.an.3a` (khối 3) gọi `listGameChallenges` → chỉ challenge có khối 3, mức `easy`–`hard`; `hs.dung.7a` thấy thêm `expert`, `legendary`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F076-AC-01 | KHI học sinh gọi `listGameChallenges`, module game PHẢI chỉ trả challenge đang publish có `grade_levels` chứa khối học sinh, thỏa `unlock_rule`, trong `availability`, mode `solo`/`party_optional`, với `available_difficulties` đã lọc theo khối (khối 1–3 theo `g1_3_challenge_rules`) và `attempts_left` | FR-CHL-001 · [CHALLENGE_BOSS_SYSTEM §2.2](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | TC-CHL-002 |
| F076-AC-02 | KHI học sinh gọi `getGameChallenge` cho challenge không khả dụng với mình (khác khối, chưa mở, ngoài thời gian, chưa gán), module game PHẢI trả `404 NOT_FOUND`; khả dụng thì PHẢI trả lượt thử còn lại hôm nay/kỳ và thời điểm hết cooldown | FR-CHL-001 | — |
| F076-AC-03 | HỆ THỐNG PHẢI cho mọi route `/game/challenges*`, `/game/challenge-attempts/*` chỉ qua `game_player`: token nhân sự PHẢI nhận `403 GAME_ACCESS_STUDENT_ONLY`; `X-Tenant-Id` khác tenant phiên PHẢI nhận `403 TENANT_MISMATCH`; attempt của học sinh khác PHẢI trả `404` | AGENTS R23 · NFR-SEC-006 | TC-TEN-001 |

---

### User Story 2 — Bắt đầu challenge theo mức khó (Priority: P1)

Học sinh chọn mức khó và bắt đầu; hệ thống ghim phiên bản, tính hạn chót theo giờ server; hết lượt hoặc đang nghỉ chờ thì được báo thân thiện.

**Why this priority**: Tạo phiên chơi server-authoritative là điều kiện cho mọi thứ sau.

**Independent Test**: `startGameChallenge` (`hard`) → `201` có `deadline_at`; lần thứ (`challenge_attempts_per_day` + 1) trong ngày → `409 CHALLENGE_ATTEMPT_LIMIT`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F076-AC-04 | KHI học sinh gọi `startGameChallenge` hợp lệ với `difficulty` và `Idempotency-Key`, module game PHẢI tạo ChallengeAttempt `in_progress` ghim `challenge_version_id`, đặt `deadline_at` theo giờ server (thời gian theo mức, nhân hệ số khối 1–3), `lives_left` theo mức, phát `challenge.started` và trả attempt kèm hoạt động đầu tiên | FR-CHL-001 · [CHALLENGE_BOSS_SYSTEM §2.3](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | TC-CHL-001 |
| F076-AC-05 | NẾU học sinh sai khối, chưa mở khóa, ngoài thời gian, chọn mức không cho phép với khối, hoặc gửi `party_id`, THÌ module game PHẢI trả `403 CHALLENGE_NOT_ELIGIBLE` và không tạo attempt | FR-CHL-001 | TC-CHL-002 |
| F076-AC-06 | NẾU học sinh đã dùng hết `challenge_attempts_per_day` (hoặc lượt theo kỳ với `weekly`/`seasonal`) hoặc đang trong `challenge_fail_cooldown`, THÌ module game PHẢI trả `409 CHALLENGE_ATTEMPT_LIMIT` kèm thời điểm thử lại | FR-CHL-001 · AC-CHL-008 | TC-CHL-008 |
| F076-AC-07 | KHI gửi lại cùng `Idempotency-Key`, module game PHẢI trả cùng attempt; NẾU thiếu key THÌ PHẢI trả `428 PRECONDITION_REQUIRED`; NẾU học sinh đang có challenge `in_progress` khác THÌ PHẢI trả `409 CONFLICT` | NFR-SEC-004 · AGENTS R10 | TC-CHL-006 |

---

### User Story 3 — Hoạt động học tập cá nhân, chấm ở server (Priority: P1)

Mỗi lượt trong challenge là bài học tập thật của chính học sinh; điểm, sức bền, mục tiêu cập nhật từ kết quả chấm của server; xong mục tiêu thì thắng, hết sức bền hoặc trượt ngưỡng thì thua và được thử lại.

**Why this priority**: FR-CHL-003 — learning record riêng, chấm server-side.

**Independent Test**: `nextChallengeActivity` → attempt học tập mode `challenge` của đúng học sinh; phát `activity.graded` sai 3 câu ở `survival` 3 sức bền → `failed`, `challenge.completed` một lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F076-AC-08 | KHI học sinh gọi `nextChallengeActivity`, module game PHẢI gọi `createAttempt` của module learning (mode `challenge`, ngữ cảnh challenge attempt, `student_id` của phiên, dải độ khó của mức, khối học sinh, `objective_ids`) qua facade `learning`, lưu `attempt_id` vào challenge attempt và trả `ChallengeNextActivity`; payload hiển thị PHẢI không chứa đáp án | FR-CHL-003 · ADR-0016 | TC-CHL-003 |
| F076-AC-09 | KHI học sinh khối 3 chọn `hard`, hệ thống PHẢI chỉ phát câu khối 3 với độ khó trong dải của mức `hard` | FR-CHL-001 · AC-CHL-002 | TC-CHL-002 |
| F076-AC-10 | KHI module learning phát `activity.graded` của attempt thuộc challenge, module game PHẢI cập nhật `score`, `lives_left`, tiến độ `goal` theo `challenge_type` chỉ từ kết quả chấm và đẩy `challenge.attempt.updated`; event lặp PHẢI không cập nhật hai lần | FR-CHL-001 · AGENTS R5 | TC-CHL-001 |
| F076-AC-11 | KHI đạt `goal`, module game PHẢI chuyển `succeeded`; KHI hết sức bền, trượt ngưỡng `perfect` hoặc quá `deadline_at`, PHẢI chuyển `failed`; mỗi kết thúc PHẢI phát đúng một `challenge.completed` (`status`, `score`, `difficulty`, `reward_multiplier`, `first_clear`, `objective_ids`) để Reward Engine thưởng theo rule, phần thưởng lần đầu chỉ một lần | FR-CHL-001 · AC-CHL-006 | TC-CHL-006 |
| F076-AC-12 | NẾU facade `learning` trả lỗi hoặc hết deadline, THÌ `nextChallengeActivity` PHẢI trả `503 UNAVAILABLE` không đổi trạng thái challenge; gọi lại cùng `Idempotency-Key` PHẢI trả cùng attempt học tập | NFR-AVAIL-005 · ADR-0074 | TC-SVC-010 |

---

### User Story 4 — Thời gian server, bỏ cuộc, tiếp tục sau mất kết nối (Priority: P1)

Đồng hồ trên máy học sinh không ảnh hưởng kết quả; bỏ cuộc giữ nguyên bài đã làm; rớt mạng hoặc đổi thiết bị vẫn tiếp tục trước hạn chót.

**Why this priority**: Chống gian lận thời gian và tránh mất công sức của trẻ.

**Independent Test**: Đổi giờ client, chờ quá `deadline_at` → server tự `failed`; `getGameChallengeAttempt` từ thiết bị khác trả đúng điểm/sức bền.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F076-AC-13 | HỆ THỐNG PHẢI kết thúc challenge khi tới `deadline_at` theo giờ server (timer của room host, dự phòng job `challenge-deadline`) bất kể đồng hồ client; `nextChallengeActivity` sau hạn PHẢI trả `422 INVALID_STATE_TRANSITION` | FR-CHL-001 · AC-CHL-001 | TC-CHL-001 |
| F076-AC-14 | KHI học sinh gọi `abandonGameChallengeAttempt`, module game PHẢI chuyển `abandoned`, giữ nguyên attempt học tập đã ghi, phát `challenge.completed` (`abandoned`); attempt đã kết thúc PHẢI trả `422 INVALID_STATE_TRANSITION` | FR-CHL-001 | — |
| F076-AC-15 | KHI học sinh mất kết nối hoặc chuyển thiết bị (takeover), `getGameChallengeAttempt` PHẢI trả trạng thái từ checkpoint (`score`, `lives_left`, `activities_completed`, `deadline_at`) và học sinh PHẢI tiếp tục được tới `deadline_at` (challenge không giới hạn giờ: trong `attempt_resume_window`), sau đó attempt chuyển `abandoned` | FR-CHL-001 · ADR-0031 | — |
| F076-AC-16 | HỆ THỐNG PHẢI không có route nào nhận `score`, `lives_left`, kết quả hay "hoàn thành" từ client cho challenge | ADR-0016 · AGENTS R5 | TC-CHL-006 |

---

### User Story 5 — Thư viện room host dùng chung (Priority: P2)

Đội backend có một thư viện chung cho phòng chơi server-authoritative: một node sở hữu mỗi phòng, timer theo giờ server, checkpoint, thời gian ân hạn khi mất kết nối.

**Why this priority**: Tránh ba cài đặt khác nhau cho challenge, boss, battle (CHALLENGE_BOSS_SYSTEM §4).

**Independent Test**: Hai instance worker cùng nhận một phòng → chỉ một giữ lease; tắt owner → instance kia chiếm lease và tiếp tục timer từ checkpoint.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F076-AC-17 | `src/backend/pkg/roomhost` PHẢI cấp lease độc quyền mỗi phòng cho một node; KHI owner ngừng gia hạn, node khác PHẢI chiếm lease sau khi lease hết hạn và khôi phục trạng thái từ checkpoint, không chạy timer hai lần | [CHALLENGE_BOSS_SYSTEM §4](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) · [REALTIME_ARCHITECTURE](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) | — |
| F076-AC-18 | `src/backend/pkg/roomhost` PHẢI tính timer bằng đồng hồ server, lưu checkpoint qua port do module cung cấp, hỗ trợ grace theo loại phòng, và không import code nghiệp vụ của module nào | AGENTS R13 · NFR-MAINT-001 | — |
| F076-AC-19 | NẾU Valkey không khả dụng, THÌ job `challenge-deadline` PHẢI kết thúc challenge quá `deadline_at` dựa trên PostgreSQL; không mất kết quả đã chấm | NFR-AVAIL-003 · AGENTS R9 | — |

---

### User Story 6 — Kiểm tra cấu hình challenge (Priority: P2)

Game Designer lưu cấu hình challenge sai thì bị chặn với lỗi rõ ràng trước khi publish.

**Why this priority**: Cấu hình sai (độ khó ngoài khối, hoạt động chưa publish) làm hỏng trải nghiệm học tập.

**Independent Test**: Version challenge có profile `expert` cho khối 2 → `422`; tham chiếu question set chưa publish → `422`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F076-AC-20 | NẾU version `challenges` không qua `challenge.schema.json`, có `question_difficulty_min > max`, mức ngoài `g1_3_challenge_rules` cho khối 1–3, hoạt động tham chiếu chưa publish, loại `boss`/`coop`/`party`/`tournament` dùng mode `solo`, hoặc thiếu `time_limit_seconds` với loại `time`, THÌ module game PHẢI từ chối `422 VALIDATION_FAILED` chỉ rõ field; thiếu bản `en` của văn bản PHẢI trả `422 TRANSLATION_INCOMPLETE` | FR-CHL-001 · [ADMIN_PORTAL §3.12](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F076-AC-21 | NẾU tham số truy vấn (`area_id`, `cursor`) vượt `maxLength` hoặc enum `type`/`difficulty` sai, THÌ module game PHẢI trả `422 VALIDATION_FAILED` | ADR-0041 | TC-TEN-024 |

### Edge Cases

- NẾU challenge version mới được publish khi học sinh đang chơi, THÌ attempt tiếp tục theo version đã ghim (ADR-0043).
- NẾU `activity.graded` đến sau khi challenge đã `failed` vì hết giờ, THÌ kết quả học tập vẫn ghi ở module learning, challenge không đổi trạng thái.
- NẾU học sinh bị dọn/khóa giữa challenge, THÌ attempt chuyển `abandoned` khi nhận `student.cleaned_up`/`user.access_revoked`.
- NẾU học sinh trả lời quá nhanh (`rapid_guess`), THÌ câu đó vẫn trừ/cộng tiến độ theo kết quả chấm nhưng không cho XP (F074-AC-06).
- NẾU challenge `weekly` qua thứ Hai 04:00 ICT (`quest_reset_time`) giữa lượt, THÌ lượt đang chạy tính vào kỳ đã bắt đầu.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHL-001 | Challenge Engine: 7 loại solo, 5 mức khó theo khối/môn/LO/hoạt động, vòng đời, lượt thử, cooldown (boss/coop/party: F103; tournament: F107) | [CHALLENGE_BOSS_SYSTEM §2](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| FR-CHL-003 | Mỗi hoạt động trong challenge là attempt học tập cá nhân, learning record riêng | CHALLENGE_BOSS_SYSTEM §2.3 · [ADR-0016](../../docs/adr/ADR-0016-server-authoritative-scoring-answer-key-khong-bao-gio-roi-se.md) |

### Key Entities

- **Challenge / ChallengeVersion**: cấu hình challenge platform có version: loại, khối, môn, LO, các mức khó, nguồn hoạt động, mục tiêu, thời gian, lượt thử, mở khóa.
- **DifficultyProfile**: tham số một mức khó (dải độ khó câu, thời gian/câu, sức bền, hệ số thưởng, khối được chọn).
- **ChallengeAttempt**: một lượt chơi của học sinh, ghim version, trạng thái, điểm, sức bền, hạn chót, danh sách attempt học tập.
- **Room (room host)**: phòng chơi server-authoritative có owner node, timer, checkpoint, grace.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `difficulty_levels` | Năm mức và tham số mặc định theo mức |
| `g1_3_challenge_rules` | Hệ số thời gian và mức tối đa cho khối 1–3 |
| `challenge_attempts_per_day` | Lượt thử/ngày mặc định |
| `challenge_fail_cooldown` | Thời gian chờ sau `failed` từ mức Hard |
| `attempt_resume_window` | Thời gian tiếp tục challenge không giới hạn giờ |
| `deadline_grace` | Dung sai hạn chót khi nhận kết quả chấm |
| `quest_reset_time` | Ranh giới kỳ của `weekly` |
| `idempotency_record_ttl` | Lưu kết quả `startGameChallenge`/`nextChallengeActivity` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chấm điểm chỉ ở module learning; module game không nhận kết quả từ client | ADR-0016 · AGENTS R5, R6 |
| Gọi module learning qua facade `learning` (`src/backend/pkg/modcall`), không fail-open | AGENTS R29 · ADR-0074 |
| Lượt chơi ghim version | AGENTS R11, R31 · ADR-0043 |
| `src/backend/pkg/roomhost` không chứa nghiệp vụ, không import service | AGENTS R13 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% challenge `time` kết thúc trong ±2 giây quanh `deadline_at` theo giờ server ở test đổi giờ client.
- **SC-002**: 0 câu hỏi ngoài khối hoặc ngoài dải độ khó của mức trong 1.000 lượt thử tự động.
- **SC-003**: 100% hoạt động trong challenge có attempt học tập riêng đúng `student_id`.
- **SC-004**: Owner node bị tắt → phòng tiếp tục trên node khác trong ≤ 15 giây, không mất điểm đã chấm.
- **SC-005**: Gửi lặp 100 lần lệnh bắt đầu/kết thúc → phần thưởng cấp đúng một lần.

## Ngoài phạm vi

- Boss Battle, challenge `coop`/`party`, đóng góp nhóm → F103; vòng `tournament` → F107; challenge template thi đấu lớp/trường (competition) → F107/F108.
- XP/Xu từ `challenge.completed` (Reward Engine) → F074/F075 (F076 chỉ phát event và delta trigger).
- HTTP workflow `/game-admin/challenges` → F078; Challenge Builder UI → F079; giao challenge cho lớp qua LMS → F041.
- Màn chơi challenge trong Godot → nhóm F (chưa có mã trong danh mục).
- Trọng số evidence `challenge`/`timed_challenge` (`w_ctx`) → F031.

## Assumptions

- module learning (F030) mở `createAttempt` cho mode `challenge` với dải độ khó và khối trước khi US3 Done (delta §6).
- Reward rule có trigger `challenge.completed` (delta `reward-rule.schema.json`) và dùng `conditions.first_completion_only` cho phần thưởng lần đầu; hệ số `reward_multiplier` đọc từ payload event [Confirmed].
- Nếu F091 đã có lease/out cho hub, F076 trích phần đó vào `src/backend/pkg/roomhost` thay vì viết mới [Confirmed].

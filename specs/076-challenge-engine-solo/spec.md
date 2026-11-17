# Feature Specification: F076 — Challenge Engine solo, 5 mức khó, room host dùng chung

**Feature Branch**: `076-challenge-engine-solo` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `game` (+ `src/backend/pkg/roomhost`) · **Mốc**: M3 · **Phụ thuộc**: F072, F074
**Input**: Challenge PvE một người chơi trên hoạt động học tập thật: các loại `time`, `perfect`, `survival`, `puzzle`, `knowledge`, `weekly`, `seasonal`; năm mức khó ánh xạ dải độ khó câu trong khối của học sinh; mỗi hoạt động là attempt học tập cá nhân do module learning chấm; thời gian theo giờ server; giới hạn lượt/ngày, cooldown; thưởng qua Reward Engine. Dựng thư viện room host dùng chung `src/backend/pkg/roomhost` (lease owner, timer, checkpoint, grace) cho boss (F103) và battle (F105). FR nguồn: FR-CHL-001, FR-CHL-003.

## Clarifications

### Session 2026-11-16

- Q: Loại nào thuộc F076? → A: Bảy loại chơi một mình (`time`, `perfect`, `survival`, `puzzle`, `knowledge`, `weekly`, `seasonal`). `boss`, `coop`, `party` → F103; `tournament` (vòng PvE của giải) → F107.
- Q: Câu hỏi theo mức khó lấy thế nào? → A: module game gọi `createAttempt` (learning internal) với mode `challenge`, dải độ khó của mức và khối học sinh; module learning chọn câu và chấm (ADR-0016). Contract internal hiện chỉ cho `quest`/`minigame` → delta "Cần sửa" ([contracts](contracts/README.md) §6).
- Q: "Room host dùng chung" nghĩa là gì ở F076? → A: F076 tạo `src/backend/pkg/roomhost` (không logic nghiệp vụ) và dùng nó cho timer/lease/checkpoint của challenge solo; F103 (boss) và F105 (battle) dùng lại. Lựa chọn ghi ở [research.md](research.md).
- Q: Mỗi "hoạt động" có bao nhiêu câu? → A: Theo `activity_source` của challenge version; loại `survival` dùng một câu mỗi hoạt động để trừ sức bền theo từng câu.

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
- Reward rule có trigger `challenge.completed` (delta `reward-rule.schema.json`) và dùng `conditions.first_completion_only` cho phần thưởng lần đầu; hệ số `reward_multiplier` đọc từ payload event [Proposed].
- Nếu F091 đã có lease/out cho hub, F076 trích phần đó vào `src/backend/pkg/roomhost` thay vì viết mới [Proposed].

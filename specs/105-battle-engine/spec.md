# Feature Specification: F105 — Battle engine server-authoritative và 8 chế độ thi đấu

**Feature Branch**: `105-battle-engine` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `competition` (+ module `learning` internal `question-plans`) · **Mốc**: M4b · **Phụ thuộc**: F030, F104
**Input**: Trận đấu chạy như state machine phía server: server chọn bộ câu, gửi câu không kèm đáp án, nhận câu trả lời qua WebSocket, chấm và tính điểm theo giờ server, xử lý mất kết nối, ghi kết quả đúng một lần và ghi bằng chứng học tập. FR nguồn: FR-CMP-001…006, FR-MP-001, FR-LMS-004 (phần thực thi thử thách khối — ADR-0095).

## Clarifications

### Session 2026-11-16

- Q: Endpoint `createQuestionPlan`/`scorePlanAnswer` do feature nào làm? → A: F105 làm cả bên nhận ở module learning (dùng port `Delivery` của F022) trước, rồi bên gọi ở module competition (AGENTS §3 bước 3).
- Q: "8 chế độ" gồm gì? → A: Room state machine cho `battle_1v1`, `team_battle`, `coop_challenge`, `timed_quiz`, trận giải (`tournament`), trận bot luyện tập, trận thách đấu bạn (unranked) và thử thách khối `grade_challenge` (timed quiz từ template; trước ADR-0095 là Class Challenge). Seasonal Competition là mùa giải (F106); tạo giải và thử thách khối thuộc F107.
- Q: Hết grace 1v1 thì kết thúc trận ngay hay chơi tiếp tới hết? → A: Kết thúc ngay: người rời `abandoned`, đối thủ thắng, tính rating ([MULTIPLAYER_GAMEPLAY §9.2](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#92-grace-theo-chế-độ-tài-liệu-này-sở-hữu-giá-trị) sở hữu giá trị; AC-MP-005).
- Q: Đáp án đúng để reveal lấy ở đâu? → A: Từ module learning sau khi câu đóng, qua internal `revealPlanItem` (Cần thêm); module competition không lưu answer key bền vững.

### Session 2026-11-17

- Q: Thời điểm tạo question plan (RB-414) và event kết thúc trận (RB-507)? → A: Question plan tạo **khi tạo trận** (mở room từ proposal/ticket), không đợi `countdown` ([REALTIME_ARCHITECTURE §7.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md)); plan lỗi → trận `aborted` trước khi đếm ngược. Domain event kết thúc là `match.completed` (WS `battle.match.ended` chỉ là message realtime).

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Thi đấu lớp/thử thách lớp còn không? → A: Không. FR-CMP-001 (PRD viết lại): Grade/School/Seasonal Competition — **thi đấu khối thay thi đấu lớp** ([ADR-0095 mục 5 (D5)](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md#5-game-xã-hội-chat-d2-d5-d7-confirmed--owner-2026-11-21)). Chế độ thứ 8 của F105 là thử thách khối `grade_challenge` (Timed Quiz từ template, đối tượng khối ± học sinh cùng trường do F107 quản lý); luật trận không đổi.
- Q: Lớp 1–3 ở Timed Quiz thấy gì thay "mục tiêu lớp"? → A: Kết quả của chính em và tiến độ tập thể `grade_goal` của thử thách (tổng câu đúng của người tham gia so với mục tiêu trong template): không thứ hạng cá nhân, không nickname người khác, ô đóng góp < 5 học sinh bị ẩn ([COMPETITION_SYSTEM §3.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#34-timed-quiz), §9.2). Field `class_goal` trong delta `battle.match.ended` đổi thành `grade_goal` [Proposed — tên field theo ADR-0095; cách tính tiến độ theo template].
- Q: Nhân sự xem trận bằng quyền nào? → A: `lms.student.read` (đổi tên từ `lms.class.read`, ADR-0095 mục 6).

### Session 2026-11-21 (triển khai — [Confirmed], owner chốt 2026-11-20)

Quyết định khi triển khai, chọn phương án bảo thủ nhất (công bằng, không áp lực xếp hạng — R19; owner duyệt trước các đề xuất mặc định):

- Mở room: consumer `competition.battle_open` (worker events) nhận `match.created` và gửi NATS request `ew.int.competition.battle.open` (queue group `competition`) tới một replica `core-api`; replica nhận lease (`SET NX`) rồi gọi `createQuestionPlan` **ngoài transaction** (R29), ghim `question_plan_id`/`question_plan_version`/`question_plan` (chỉ `index`, `question_instance_id`, `question_version_id`, `time_limit_ms`, `points_multiplier`) và tạo `multiplayer.rooms` (`room_id = match_id`, `area_id` = mode). Trận nhiều tenant (phạm vi platform, chỉ F107 tạo) chưa chơi được cho tới khi F107 có pool câu hỏi platform: envelope `match.created` không có tenant → consumer từ chối vĩnh viễn (DLQ); nếu vẫn được mở với một tenant → `aborted` `plan_failure`.
- Vào room: client gửi `battle.state.request` khi nhận `matchmaking.match.assigned`; reply chứa snapshot và `subscribe` `ew:rt:room:<match_id>:out`. Lobby chuyển sang `countdown` (3 s) khi mọi người đã vào hoặc sau 15 s; người chưa vào được tính như mất kết nối (grace của mode bắt đầu). Message cho một người (snapshot, `battle.answer.evaluated`, `battle.match.ended` của Timed Quiz) đi `ew:rt:user:<id>`; message chung đi room output.
- `question_instance_id` = `qi_` + 16 hex đầu của SHA-256(`<match_id>:<index>`): ổn định qua failover, không lộ `question_version_id`.
- Từ chối câu trả lời: câu chưa phát hoặc không thuộc plan → `VALIDATION_FAILED` + signal `answer_before_shown`; câu đã đóng → `ANSWER_LATE` + signal `answer_closed_question`; response sai shape loại câu, trường lạ (vd. `score`), `question_instance_id` > 64 → `VALIDATION_FAILED`, room không đổi. Chấm: learning trả 4xx (response không hợp lệ) → câu sai; learning không trả lời → câu được làm lại (tối đa 2 lần, lần 3 → `aborted` `server_failure`).
- Learning: plan tạo một attempt `mode = competition` (`context_type = match`, `source_kind = selection`, `feedback = after_close`) cho mỗi người chơi ngay khi tạo plan, nên câu của trận được tính "đã gặp" cho `avoid_recent` 7 ngày; "loại trừ câu đã gặp" theo bước nới của QUESTION_BANK §6 (thiếu câu thì nới, vẫn thiếu → `QUESTION_POOL_INSUFFICIENT` → trận `aborted` `plan_failure`). `late` = nhận sau `expires_at` của plan (thời gian chơi + 15 phút).
- `revealPlanItem` trả thêm `question` (render payload) để `getMatch` dựng phần xem lại sau trận; `correct_response` chỉ cho loại có đáp án cố định không tham số.
- Luật mode đọc từ mục `battle`/`bot_profiles` của cấu hình matchmaking đã publish mà trận ghim (`ruleset_version_id`); khóa thiếu → giá trị CONFIG_DEFAULTS. Đề xuất mặc định mới: Timed Quiz 10 câu × 20 s (lớp 1–3: 30 s), tối đa 40, top 5 từ lớp 4, `grade_goal` = 60% (người × câu); Co-op Challenge 12 câu × 25 s, mục tiêu 10 câu đúng tập thể, grace 120 s (như quest co-op); lobby chờ 15 s, countdown 3 s, reveal 3 s, chờ chấm 3 s, chờ reveal 2 s; bot lớp 1–3: 60%, 6–18 s; lớp 4–5: 70%, 4–14 s; lớp 6–9: 75%, 3–12 s.
- 1v1: khi người hết grace mà đối thủ cũng đang mất kết nối → `cancelled` (`all_left`), mọi người `no_contest`, không ai bị tính bỏ trận. Học sinh bỏ trận bot → `cancelled` (không ghi gì bất lợi). `abandon_penalty` chỉ đếm trận **ranked** `completed` có outcome `abandoned` (trận bot/bạn bè/giải/hủy/aborted không tính).
- Đội: cả đội rời → đội kia thắng (`abandon`); mọi người rời → `cancelled`. Timed Quiz và Co-op: outcome `no_contest`, không `winner_team`, `rated = false`. `grade_goal` ẩn `contributors_count` khi < 5 học sinh (ADR-0095).
- Nhân sự xem trận: `getMatch` không có guard `game_player` ở route (một route cho STU và SA, API_DESIGN §9.7); use case từ chối nhân sự dùng token game (`GAME_ACCESS_STUDENT_ONLY`) và chỉ cho nhân sự có quyền xem học sinh trong trường của một người tham gia (khác trường → `404`). Catalog vẫn là `lms.class.read` cho tới khi WP đổi tên `lms.student.read` (ADR-0095 mục 6) — F105 dùng hằng số của catalog.
- Vào Timed Quiz/thử thách khối (T034 `JoinTimedQuizRoom`): không có command riêng — F107 tạo trận với danh sách học sinh (≤ 40, mỗi người một `match_participants`) và học sinh vào room bằng `battle.state.request` như mọi trận; vào muộn chỉ trả lời được câu đang mở và các câu sau. Co-op Challenge dùng `coop.*` (task = câu hỏi); `coop.action.perform` của thành viên → `VALIDATION_FAILED` (không có puzzle; puzzle co-op là quest của module game).
- Biến môi trường mới: `EW_BATTLE_TICK_HZ` (10), `EW_BATTLE_ROOM_CHECKPOINT` (5 s, làm mới checkpoint ngoài mỗi lần chuyển trạng thái), `EW_BATTLE_PLAN_DEADLINE` (3 s), `EW_BATTLE_SCORE_DEADLINE` (2 s).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Trận 1v1 Knowledge Battle do server điều khiển (Priority: P1) 🎯 MVP

`hs.an.3a` và `hs.binh.3a` vào phòng sau ready check; đếm ngược, 7 câu Toán lớp 3 hiện cùng lúc cho cả hai, mỗi người trả lời một lần; sau mỗi câu cả hai thấy đáp án; điểm chỉ tính câu đúng (lớp 3 không có bonus tốc độ).

**Why this priority**: Lõi của mọi chế độ thi đấu; FR-CMP-002 là điều kiện công bằng.

**Independent Test**: Hai bot WS chơi 1 trận `battle_1v1` lớp 3: mọi `battle.question.shown` không có `answer_key`/`correct_response`; điểm mỗi câu đúng = 100; trận lớp 7 có `time_bonus` ≤ 50.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F105-AC-01 | KHI tạo trận (mở room từ proposal/ticket, trước `countdown`), module competition PHẢI gọi module learning `createQuestionPlan` (khối thấp nhất, môn, chủ đề, số câu và thời gian theo luật mode) và ghim `question_plan_version`; mọi người chơi PHẢI nhận cùng câu, cùng `shown_at`/`deadline_at` theo giờ server | FR-CMP-002 · [COMPETITION_SYSTEM §3.1](../../docs/01-game-design/COMPETITION_SYSTEM.md#31-1v1-knowledge-battle-confirmed) · ADR-0043 | TC-CMP-001, TC-CMP-003 |
| F105-AC-02 | module learning PHẢI chọn câu đã publish khả dụng cho tenant, đúng khối/môn/chủ đề, loại trừ câu một trong các người chơi đã gặp trong 7 ngày, và trả render payload không có `answer_key`, `scoring_rule`, `explanation`, `options[].feedback` | FR-CMP-002 · ADR-0016 · AGENTS R6 | TC-LRN-007 |
| F105-AC-03 | KHI nhận `battle.answer.submit`, room host PHẢI gắn `received_at` server, chỉ chấp nhận từ thành viên room cho câu đang mở thuộc plan; trả lời sau `deadline_at` + grace PHẢI bị từ chối `ANSWER_LATE`; trả lời lần hai PHẢI bị bỏ (`ANSWER_ALREADY_SUBMITTED`, giữ lần đầu); trả lời câu chưa phát/đã đóng/không thuộc trận PHẢI bị từ chối và ghi integrity signal | AC-CMP-006 · [ANTI_CHEAT §3–§4](../../docs/10-security/ANTI_CHEAT.md#3-kiểm-tra-thời-gian) | TC-CMP-007 |
| F105-AC-04 | HỆ THỐNG PHẢI chấm mỗi câu qua `scorePlanAnswer` và tính điểm: đúng 100, sai/không trả lời 0, câu cuối ×2; Ở NƠI `grade_level` trận ≥ `speed_bonus_min_grade`, HỆ THỐNG PHẢI cộng bonus tốc độ ≤ 50 tuyến tính theo `received_at − shown_at`; trận dưới ngưỡng PHẢI không có bonus | FR-CMP-006 · AC-CMP-103 · `battle_1v1_format` | TC-CMP-007 |
| F105-AC-05 | KHI câu đóng (mọi người đã trả lời hoặc hết giờ), HỆ THỐNG PHẢI gửi đồng thời `battle.answer.evaluated` (kết quả riêng, đáp án đúng, giải thích nếu mode cho phép) và `battle.question.revealed` (bảng điểm) tới mọi người chơi | ADR-0016 · [COMPETITION_SYSTEM §4](../../docs/01-game-design/COMPETITION_SYSTEM.md#4-chấm-điểm-phía-server) | TC-CMP-003 |
| F105-AC-06 | HỆ THỐNG PHẢI xác định thắng theo tổng điểm, bằng điểm → hòa; mọi trường điểm/kết quả/thời gian do client gửi PHẢI bị bỏ qua (schema lệnh không có trường điểm) | FR-CMP-002 · AGENTS R5 · ADR-0016 | TC-CMP-007 |

---

### User Story 2 — Kết quả ghi một lần và bằng chứng học tập (Priority: P1)

Trận kết thúc; kết quả được lưu bền vững đúng một lần dù worker retry hay hai node cùng ghi; từng câu trả lời trở thành bằng chứng học tập trọng số thấp.

**Why this priority**: Rating, Competition Points, phần thưởng (F106, module game) đều dựa trên kết quả này.

**Independent Test**: Gọi `RecordMatchResult` hai lần cho cùng `match_id` → một dòng `match_results`, một event `match.completed`; mỗi câu đã chấm có `attempt_answer` `mode = competition` ở module learning.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F105-AC-07 | KHI state machine kết thúc trận, HỆ THỐNG PHẢI ghi `MatchResult` (PK `match_id`), kết quả từng người tham gia và outbox `match.completed` trong một transaction rồi mới gửi `battle.match.ended`; ghi lần hai PHẢI trả `MATCH_RESULT_ALREADY_RECORDED` nội bộ và không phát event thứ hai | FR-CMP-003 · AC-CMP-005 · [ANTI_CHEAT §8](../../docs/10-security/ANTI_CHEAT.md#8-bảo-vệ-api-thi-đấu) | TC-CMP-009 |
| F105-AC-08 | HỆ THỐNG PHẢI ghi audit `match_result.recorded` (match, mode, ranked, người tham gia, lý do kết thúc) và không có REST endpoint nào cho client tạo/sửa kết quả trận | FR-CMP-003 · AGENTS R10 | TC-CMP-009 |
| F105-AC-09 | KHI module learning chấm một câu trong trận, module learning PHẢI ghi `attempt_answer` `mode = competition` (trọng số mastery thấp theo cấu hình mastery) idempotent theo `(plan_id, index, student_id)`; câu `late` PHẢI không tính điểm | FR-CMP-004 · ADR-0017 | — |
| F105-AC-10 | HỆ THỐNG PHẢI lưu kết quả trận, người tham gia, điểm trong PostgreSQL; kết quả đã ghi PHẢI còn nguyên sau khi xóa hoặc khởi động lại Valkey | AGENTS R9 · ADR-0004 | TC-CMP-006 |

---

### User Story 3 — Mất kết nối, kết nối lại, đổi thiết bị, sự cố server (Priority: P1)

Mạng nhà `hs.binh.3a` chập chờn; em quay lại trong 30 giây và chơi tiếp đúng câu hiện tại; nếu không quay lại, `hs.an.3a` thắng. Node chạy phòng bị tắt giữa trận, trận khôi phục từ checkpoint.

**Why this priority**: Trẻ em chơi trên mạng trường học yếu; mất công là trải nghiệm tệ nhất.

**Independent Test**: Ngắt WS 10 s → đối thủ thấy `battle.participant.status_changed` (`grace_until`), kết nối lại nhận `battle.state.snapshot`; ngắt 35 s ở 1v1 → trận `completed`, người rời `abandoned`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F105-AC-11 | KHI người chơi mất kết nối, HỆ THỐNG PHẢI giữ chỗ trong `disconnect_grace` của mode, gửi `battle.participant.status_changed` (`connected = false`, `grace_until`) và để đồng hồ câu chạy tiếp (không trả lời = 0); KHI kết nối lại trong grace, HỆ THỐNG PHẢI gửi `battle.state.snapshot` đúng trạng thái hiện tại | FR-CMP-005 · AC-CMP-003 · [MULTIPLAYER_GAMEPLAY §9](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#9-mất-kết-nối-kết-nối-lại-đổi-thiết-bị) | TC-CMP-004, TC-CMP-005 |
| F105-AC-12 | NẾU người chơi `battle_1v1` quá grace, THÌ HỆ THỐNG PHẢI kết thúc trận: người rời `abandoned`, đối thủ `win`, có tính rating; NẾU cả hai rời, THÌ trận `cancelled`, không tính rating | FR-CMP-005 · AC-MP-005 | TC-CMP-004 |
| F105-AC-13 | Ở NƠI mode là `team_battle`, HỆ THỐNG PHẢI cho đội tiếp tục thiếu người và ghi thành viên quá grace là `abandoned`; Ở NƠI mode là `timed_quiz`, HỆ THỐNG PHẢI giữ chỗ tới hết giờ quiz và tự chốt các câu đã trả lời | [MULTIPLAYER_GAMEPLAY §9.2](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#92-grace-theo-chế-độ-tài-liệu-này-sở-hữu-giá-trị) | — |
| F105-AC-14 | KHI học sinh takeover sang thiết bị mới trong grace (ADR-0031), HỆ THỐNG PHẢI cho vào lại cùng room bằng principal (không bằng room token chia sẻ), giữ membership và ghi `leave_reason = replaced_by_other_device` cho phiên cũ | FR-AUTH-023 · [ANTI_CHEAT §8](../../docs/10-security/ANTI_CHEAT.md#8-bảo-vệ-api-thi-đấu) | — |
| F105-AC-15 | KHI owner node chết, room host mới PHẢI khôi phục từ checkpoint sau mỗi chuyển trạng thái và chơi lại câu đang dở (`replay_current_question = true`); NẾU không có checkpoint hợp lệ, THÌ trận PHẢI chuyển `aborted`, không có kết quả, không đổi rating, không tính abandon, người chơi được thông báo | AC-MP-006 · [REALTIME_ARCHITECTURE §5.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md#51-room-registry--failover-module competition) | — |
| F105-AC-16 | KHI học sinh có số trận `abandoned` trong 24 giờ đạt ngưỡng `abandon_penalty`, HỆ THỐNG PHẢI ghi hạn chế ranked có thời hạn kèm audit; trận `aborted`/`cancelled` do sự cố server PHẢI không được tính | [MULTIPLAYER_GAMEPLAY §9.1](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#91-luật-chung) | — |

---

### User Story 4 — Team Battle, Co-op Challenge, Timed Quiz, bot và thử thách khối (Priority: P2)

Học sinh lớp 6 đấu đội 3v3 xoay vòng lượt chính; party lớp 2 cùng "dập lửa" trước khi hết giờ; học sinh khối 4 của trường làm Timed Quiz thử thách khối do nhà trường tạo; học sinh một mình đấu với Robot Học Giỏi.

**Why this priority**: Hoàn thiện các chế độ FR-CMP-001 sau khi lõi 1v1 ổn định.

**Independent Test**: Trận `team_battle` 10 câu: mỗi câu đúng một người lượt chính, lượt hỗ trợ nhận 30%; Timed Quiz lớp 2 trả kết quả của chính em + `grade_goal` (tiến độ tập thể), không trả thứ hạng cá nhân, không nickname người khác; trận bot gắn nhãn Robot, `rated = false`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F105-AC-17 | Ở NƠI mode là `team_battle`, HỆ THỐNG PHẢI xoay vòng thành viên trả lời lượt chính mỗi câu, cho thành viên khác lượt hỗ trợ theo `team_battle_format`, tính tổng điểm đội và chỉ mở cho khối ≥ 4 | FR-CMP-001 · [COMPETITION_SYSTEM §3.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#32-team-battle-confirmed) | TC-MM-004 |
| F105-AC-18 | Ở NƠI mode là `coop_challenge`, HỆ THỐNG PHẢI chạy room `coop` với mục tiêu tập thể và đồng hồ server, ghi mỗi câu trả lời vào attempt của chính học sinh đó, không xếp hạng cá nhân và không có rating | FR-CMP-001 · [COMPETITION_SYSTEM §3.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#33-co-op-challenge) | TC-PTY-010 |
| F105-AC-19 | Ở NƠI mode là `timed_quiz` (gồm thử thách khối `grade_challenge` tạo từ template), HỆ THỐNG PHẢI cho 1–40 học sinh cùng bộ câu, áp luật điểm 1v1 (lớp 1–5 không bonus tốc độ), trả top 5 + vị trí của chính mình; lớp 1–3 PHẢI chỉ nhận kết quả của chính em và tiến độ tập thể `grade_goal` của thử thách (không thứ hạng cá nhân, không nickname người khác, ô < 5 học sinh ẩn — ADR-0095) | FR-CMP-001 · FR-LMS-004 · [COMPETITION_SYSTEM §3.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#34-timed-quiz) | — |
| F105-AC-20 | Ở NƠI trận `is_bot = true`, HỆ THỐNG PHẢI chạy bot có độ chính xác và thời gian trả lời theo cấu hình, hiển thị nhãn "Robot", `rated = false`, không cộng Competition Points/leaderboard, vẫn ghi bằng chứng học tập của học sinh | [MATCHMAKING §9](../../docs/03-social-multiplayer/MATCHMAKING.md#9-bot-luyện-tập-fallback) | — |
| F105-AC-21 | Ở NƠI trận thuộc giải (`tournament_node_id`), HỆ THỐNG PHẢI dùng luật 1v1/đội của giải, grace trận giải (tối đa 2 lần/trận), xử thua `forfeit` khi vượt, và luôn `rated = false` | [COMPETITION_SYSTEM §10.7](../../docs/01-game-design/COMPETITION_SYSTEM.md#107-no-show-disconnect-forfeit-kết-quả-confirmed) | — |

---

### User Story 5 — Room, lịch sử trận, phân quyền và chống gian lận (Priority: P1)

Học sinh xem lại trận của mình với câu hỏi và giải thích; School Admin xem trận của học sinh trường mình; người ngoài không xem được; trả lời bất thường nhanh bị ghi nhận để xem xét.

**Why this priority**: Điều V và VI của constitution; ROOM/Lobby/Match/Session là FR-MP-001.

**Independent Test**: `hs.chi.3b` gọi `getMatch` của trận `hs.an.3a`/`hs.binh.3a` → 404; `ad.truong.a` gọi → 200 (nickname); bot trả lời 3 câu < `cheat_fast_answer_flag` → `competition.integrity_signals` có dòng, không bonus.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F105-AC-22 | HỆ THỐNG PHẢI tạo room `lobby` → `battle`/`coop` với bản ghi tóm tắt và `session_memberships` (`joined_at`, `left_at`, `leave_reason`, `match_id`, `game_session_id`); lệnh WS từ người không phải thành viên PHẢI nhận `ROOM_NOT_MEMBER`; ticket WS của tenant khác PHẢI nhận `PERMISSION_DENIED` | FR-MP-001 · AC-MP-008 · [MULTIPLAYER_GAMEPLAY §4, §10](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#10-session-membership) | TC-TEN-012 |
| F105-AC-23 | KHI người tham gia gọi `getMatch`/`listMyMatches`, HỆ THỐNG PHẢI trả trạng thái, kết quả, và sau khi trận kết thúc là câu hỏi + đáp án + giải thích; người không tham gia PHẢI nhận `404 NOT_FOUND`; nhân sự có `lms.student.read` (ADR-0095 mục 6) trong trường của học sinh PHẢI xem được qua portal; đối thủ chỉ hiển thị nickname/avatar, không trả số rating | AGENTS R16 · [API_DESIGN §9.7](../../docs/07-backend/API_DESIGN.md#97-competition--apiv1competition) | TC-TEN-001 |
| F105-AC-24 | KHI học sinh trả lời nhanh hơn `cheat_fast_answer_flag` liên tiếp ≥ 3 câu, HỆ THỐNG PHẢI ghi `integrity_signals` và bỏ bonus tốc độ của các câu đó, không tự hủy kết quả hay phạt | [ANTI_CHEAT §6](../../docs/10-security/ANTI_CHEAT.md#6-anomaly-detection-confirmed) · [COMPETITION_SYSTEM §4](../../docs/01-game-design/COMPETITION_SYSTEM.md#4-chấm-điểm-phía-server) | — |
| F105-AC-25 | HỆ THỐNG PHẢI chỉ cho principal qua `game_player` gọi `getMatch`/`listMyMatches` phía game và lệnh `battle.*`/`coop.*`; nhân sự gọi route người chơi PHẢI nhận `403 GAME_ACCESS_STUDENT_ONLY`; `X-Tenant-Id` khác tenant phiên PHẢI nhận `403 TENANT_MISMATCH` | AGENTS R23 · ADR-0031 | TC-AUTH-014, TC-TEN-011 |
| F105-AC-26 | HỆ THỐNG PHẢI phát `match.started`, `match.completed`, `match.aborted` kèm danh sách người tham gia/đội để module chat mở/đóng channel `match`/`team`, và PHẢI không có cờ hay tham số nào tắt chat trận | ADR-0020 · AGENTS R15 · [EVENT_DESIGN §4.8](../../docs/07-backend/EVENT_DESIGN.md#48-competition) | — |
| F105-AC-27 | NẾU `battle.answer.submit` có `question_instance_id` vượt `maxLength`, response sai schema loại câu, hoặc trường lạ, THÌ HỆ THỐNG PHẢI từ chối `VALIDATION_FAILED` mà không đổi trạng thái room | ADR-0041 | TC-TEN-024 |

### Edge Cases

- NẾU module learning lỗi khi tạo plan, THÌ trận `aborted` trước câu 1 (không rating, không abandon) và người chơi được về queue với ưu tiên (F105-AC-15).
- NẾU `scorePlanAnswer` lỗi/timeout, THÌ room host retry idempotent; vẫn lỗi → câu được làm lại, quá 2 lần → trận `aborted` (không rating, không abandon) theo [REALTIME_ARCHITECTURE §7.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md#71-state-machine-knowledge-battle).
- NẾU câu hỏi được publish/rollback giữa trận, THÌ trận vẫn dùng version đã ghim (F105-AC-01).
- NẾU người chơi trả lời đúng lúc hết giờ, THÌ chỉ `received_at` server quyết định (F105-AC-03).
- NẾU tài khoản bị khóa/thu hồi giữa trận, THÌ kết nối bị cắt, xử lý như mất kết nối quá grace.
- NẾU Timed Quiz có học sinh vào muộn sau câu 1, THÌ chỉ được trả lời các câu còn lại.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CMP-001 | State machine 1v1, Team Battle, Co-op Challenge, Timed Quiz, trận giải, bot | [COMPETITION_SYSTEM §2–§3](../../docs/01-game-design/COMPETITION_SYSTEM.md#2-các-mode-thi-đấu-confirmed--master-prompt-53) |
| FR-CMP-002 | Question plan server, chấm và điểm thời gian server-side | [COMPETITION_SYSTEM §4](../../docs/01-game-design/COMPETITION_SYSTEM.md#4-chấm-điểm-phía-server) |
| FR-CMP-003 | Kết quả ghi một lần, idempotent, audit | [COMPETITION_SYSTEM §4](../../docs/01-game-design/COMPETITION_SYSTEM.md#4-chấm-điểm-phía-server), [ANTI_CHEAT §4](../../docs/10-security/ANTI_CHEAT.md#4-duplicate-submission-idempotency--replay) |
| FR-CMP-004 | Bằng chứng học tập `competition` trọng số thấp | [COMPETITION_SYSTEM §4](../../docs/01-game-design/COMPETITION_SYSTEM.md#4-chấm-điểm-phía-server) |
| FR-CMP-005 | Grace và forfeit theo mode | [MULTIPLAYER_GAMEPLAY §9.2](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#92-grace-theo-chế-độ-tài-liệu-này-sở-hữu-giá-trị) |
| FR-CMP-006 | Không bonus tốc độ khối < `speed_bonus_min_grade` | [COMPETITION_SYSTEM §3.1](../../docs/01-game-design/COMPETITION_SYSTEM.md#31-1v1-knowledge-battle-confirmed) |
| FR-MP-001 | Room `lobby`/`battle`/`coop`, team, match, session membership | [MULTIPLAYER_GAMEPLAY §4, §10](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#4-không-gian-chơi-chung-room) |
| FR-LMS-004 | Thực thi thử thách khối (Timed Quiz) do School Admin tạo | [SCHOOL_LMS §7](../../docs/04-admin-portal/SCHOOL_LMS.md#7-thử-thách-lớp-và-giải-trường) |

### Key Entities

- **Match**: trận với mode, khối, môn, plan câu hỏi đã ghim, trạng thái `lobby → running → completed | cancelled | aborted`.
- **Room**: không gian chơi ngắn hạn (`lobby`, `battle`, `coop`) do một instance sở hữu, có checkpoint.
- **Question Plan**: danh sách câu (render payload, không đáp án) cho một trận, có hạn dùng; thuộc module learning.
- **Match Result**: kết quả duy nhất của trận (đội thắng, điểm từng người, lý do kết thúc, có tính rating hay không).
- **Session Membership**: ai ở room nào, khi nào, lý do rời.
- **Integrity Signal**: tín hiệu bất thường để xem xét, không tự phạt.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `battle_1v1_format` | Số câu, thời gian/câu, điểm, bonus, câu cuối |
| `team_battle_format` | Số câu, tỉ lệ lượt hỗ trợ |
| `speed_bonus_min_grade` | Khối tối thiểu có bonus tốc độ |
| `disconnect_grace` | Grace theo mode |
| `abandon_penalty` | Hạn chế ranked khi bỏ trận lặp lại |
| `cheat_fast_answer_flag` | Ngưỡng trả lời quá nhanh |
| `tournament_timers` | Grace trận giải |
| `xp_competition_matches_per_day` | (module game dùng; F105 chỉ phát event) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Điểm, thời gian, kết quả tính ở server; đáp án không xuống client trước khi câu đóng | AGENTS R5, R6 · ADR-0016 |
| Kết quả idempotent + audit | AGENTS R10 · ADR-0010 |
| Plan ghim version | AGENTS R11, R31 · ADR-0043 |
| Gọi module learning qua facade `learning` (`src/backend/pkg/modcall`) | AGENTS R29 · ADR-0074 |
| Chat trận always-on | AGENTS R15 · ADR-0020 |
| Không log câu trả lời học sinh ở INFO | AGENTS R14 |
| Tải liên module battle → learning p95 | TC-PERF-013 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 trường đáp án/giải thích xuất hiện trong mọi message gửi client trước khi câu đóng (contract test trên 1.000 trận mô phỏng).
- **SC-002**: Với retry, replay và hai node cùng ghi, mỗi trận có đúng một kết quả và đúng một event `match.completed`.
- **SC-003**: 100% trận lớp 1–5 có điểm mỗi câu đúng bằng 100, không có bonus tốc độ.
- **SC-004**: Mất kết nối 10 s rồi quay lại: người chơi tiếp tục đúng câu hiện tại trong ≤ 2 s sau khi kết nối lại.
- **SC-005**: Lời gọi competition → learning p95 đạt ngưỡng TC-PERF-013 ở 1× CCU.

## Ngoài phạm vi

- Ghép trận, ready check, duel, proposal giải — F104.
- Rating, Competition Points, leaderboard, mùa — F106 (tiêu thụ `match.completed`).
- Tạo giải/thử thách khối, bracket — F107; giao diện trận trong Godot — F109.
- XP/phần thưởng thi đấu — module game (F074); mở/đóng channel chat trận, purge — F117.
- Vô hiệu trận (`voided`) và hoàn tác rating khi có bằng chứng gian lận mạnh (ANTI_CHEAT §9 mức 2) — feature xử lý vi phạm sau này.

## Assumptions

- [Confirmed] `src/backend/pkg/roomhost` (lease, checkpoint, input `ew.in.room.<room_id>`) có sẵn từ F091/F101; nếu chưa, F105 tạo trong `src/backend/pkg/roomhost` với review Backend Architect.
- [Confirmed] Bot luyện tập: độ chính xác và phân bố thời gian trả lời là cấu hình trong payload matchmaking config (`bot_profiles`), mặc định theo [MATCHMAKING §9](../../docs/03-social-multiplayer/MATCHMAKING.md#9-bot-luyện-tập-fallback).
- [Confirmed] Thử thách khối Timed Quiz: học sinh thuộc đối tượng (khối ± học sinh, F107) vào phòng trong khung từ `starts_at` trừ 5 phút tới câu cuối (`joinChallenge` của F107).
- Trọng số mastery cho `competition` theo ADR-0017 (cấu hình mastery của module learning).

# Feature Specification: F109 — Giao diện Arena, trận đấu, bảng xếp hạng, giải đấu trong Godot

**Feature Branch**: `109-godot-arena-competition-ui` · **Created**: 2026-11-16 · **Status**: In progress · **Module**: `src/game/` · **Mốc**: M4b · **Phụ thuộc**: F102, F105, F106, F107 (+ F063 renderer câu hỏi, F092 `RealtimeClient`, F119 chat trận)
**Input**: Màn Competition Arena của học sinh: chọn chế độ, hàng chờ, ready check, chơi trận 1v1/đội theo message server, màn kết quả chỉ hiển thị tier, lịch sử trận, bảng xếp hạng theo khối và chính sách trường, danh sách/chi tiết/đăng ký/check-in giải đấu, bracket, thách đấu bạn. FR nguồn: FR-CMP-001, FR-CMP-006, FR-RNK-003, FR-LDB-003, FR-LDB-004.

## Clarifications

### Session 2026-11-16

- Q: Client biết chế độ nào mở cho học sinh (theo khối) bằng cách nào mà không hardcode bảng khối của COMPETITION_SYSTEM §2? → A: Server trả danh sách chế độ khả dụng (`listArenaModes`, delta owner F105); client chỉ hiển thị. Grade band chỉ dùng cho density và cách trình bày UX.
- Q: Ẩn nội dung "trả lời nhanh được thêm điểm" với lớp 1–5 dựa vào đâu? → A: Cờ `speed_bonus_enabled` trong `battle.countdown.started`/`battle.state.snapshot` (delta owner F105); client không so `grade_level` với `speed_bonus_min_grade`.
- Q: `Rating` và `Match.participants[].rating_delta` hiện chứa số rating thô — client xử lý thế nào? → A: Theo COMPETITION_SYSTEM §8.2 (mọi khối chỉ thấy tier): client bỏ các trường này khi parse; delta sửa schema (owner F106).
- Q: Cách hiển thị bảng theo khối (lớp 1–3 mục tiêu tập thể, 4–5 top 10, 6–9 top 50 ± quanh mình) do ai quyết? → A: Server (F106) quyết theo `leaderboard_visibility` và trả `display_mode`; client chỉ render theo `display_mode`.
- Q: Chi tiết giải, bracket, đăng ký, check-in có contract chưa? → A: Chỉ có `listTournaments`; các operation còn lại là delta theo COMPETITION_SYSTEM §14.2 (owner F107).

### Session 2026-11-17

- Q: Loại thông báo giải sau đối soát (RB-513)? → A: Thông báo dùng một enum `kind` chung REST/WS (F123); giải dùng `kind = tournament_update`, không còn `category`.

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Bảng lớp trong game đổi thế nào? → A: Theo [ADR-0095 mục 5 (D5)](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md#5-game-xã-hội-chat-d2-d5-d7-confirmed--owner-2026-11-21) không còn "Mục tiêu lớp", "Lớp em tuần này", "Nhiệm vụ của lớp"; thay bằng "Mục tiêu của khối" — `display_mode = grade_goal` render bằng **`GradeGoalBar`** (thay `ClassGoalBar`): tiến độ tập thể của khối trong trường, phần em đã góp, số bạn đã góp sức (ẩn khi < 5); không thứ hạng cá nhân, không nickname người khác ([COMPETITION_SYSTEM §9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#92-danh-mục-bảng-mặc-định-confirmed), FR-LDB-005). "Bạn nổi bật tuần này" (`highlights`) bị bỏ theo F106 [Proposed].
- Q: Dòng bảng còn tên lớp? → A: Không — chỉ nickname + avatar (R16, FR-LDB-003).
- Q: Giải trong game? → A: Giải khối/thử thách khối của trường (`grade_challenge`, F107) thay giải lớp; học sinh chỉ thấy giải mà mình thuộc đối tượng (khối ± học sinh, trừ danh sách loại trừ) — server lọc, client không tự lọc theo khối.

### Session 2026-11-27 (triển khai client — đối chiếu contract trên main)

- Q: Delta contracts/README §6 đã được F105–F107 áp dưới tên nào? → A: Client dùng đúng contract hiện có, không tự thêm trường: `listCompetitionModes` (`CompetitionMode`: `mode`, `matchmaking_types`, `ranked`, `min_grade`, `subjects` — server chỉ trả chế độ khả dụng nên **không có thẻ "khóa kèm lý do"**), `getMyRatings` (`tier_code`/`tier_name`/`provisional`, không số rating), `LeaderboardPage.visibility` (`grade_goal` / `top_n_percentile` / `ranked` — tên contract của `display_mode`), `registerTournamentEntry` (= `createTournamentEntry`), lỗi giải là 404 / 403 `POLICY_DENIED` / 409 `CONFLICT` (đủ chỗ) / 422 `INVALID_STATE_TRANSITION` (ngoài cửa sổ) vì chưa có `ProblemCode` `TOURNAMENT_*`. [Confirmed — contract main]
- Q: `speed_bonus_enabled`, `primary_user_ids`, `encouragement` chưa có trong AsyncAPI — client làm gì? → A: Đọc khi server gửi (tên đúng delta §6), vắng = cách đọc an toàn: không chỉ dẫn/hiệu ứng tốc độ, không nhãn lượt chính, không lời động viên riêng; "Thưởng tốc độ" chỉ hiện khi `time_bonus > 0` của `battle.answer.evaluated`. Owner F105 bổ sung trường vào AsyncAPI. [Proposed]
- Q: Kênh chat của trận lấy từ đâu khi contract chưa có `chat_channel_ids`? → A: Channel `match` / `team` đang mở (`status = open`) trong `listMyChannels` — học sinh chỉ ở một trận một lúc; không tự dựng id kênh. Owner F105/F117 thêm `chat_channel_ids` vào `matchmaking.match.assigned` / `battle.state.snapshot`. [Proposed]
- Q: Câu hỏi trong trận dùng khóa lựa chọn nào? → A: AsyncAPI `QuestionRender.options[].option_id`, render của module learning dùng `id` — client nhận cả hai (chuẩn hóa về `id` cho renderer F063), gửi `{option_id}` như `AnswerResponse`. Owner F105 thống nhất một tên. [Proposed]
- Q: Chat trận nằm đâu khi câu hỏi hiện (AC-13)? → A: Giữa hai câu: cột phải cao hết màn (quy tắc "≥ 3 tin"); khi câu hỏi hiện: dải 2 dòng cạnh `MatchHeader` (góc trên phải), câu hỏi chiếm cả chiều ngang — không che đáp án; cửa sổ hẹp/chữ dài: dải dưới câu hỏi, câu hỏi cuộn. [Confirmed — F109, theo ma trận khối]
- Q: `409 MM_TICKET_ALREADY_ACTIVE` không kèm `ticket_id` — mở thanh hàng chờ hiện có thế nào? → A: Đọc lại ticket đang giữ trong RAM (`getMatchmakingTicket`); không có (vd. vừa mở lại app) → "Em đang trong hàng chờ rồi." và chờ `matchmaking.ticket.updated`. Owner F104 thêm `ticket_id` vào Problem. [Proposed]
- Q: Ô ghi chú báo cáo (AC-16 `note` ≤ maxLength, bộ đếm)? → A: Màn kết quả/lịch sử dùng bảng báo cáo F119 (`ChatPlayerSafety` + `ChatDialogs`, lý do bằng hình + chữ, ngữ cảnh `match_id`) — F119 không gửi chữ tự do của trẻ (MODERATION §5, CHILD_SAFETY); không thêm ô ghi chú. [Proposed — chờ owner xác nhận bỏ `note` khỏi AC-16]
- Q: Người nhận biết có lời thách đấu bằng cách nào? → A: Contract chưa có message/notification cho lời thách đấu đến (`duel.sent` chỉ là event NATS) — client có hộp nhận lời (`ui_requested("arena", {duel, nickname})`) nhưng chưa có nguồn giao; đề xuất `notification.item.created` kind `duel_invitation` hoặc WS `duel.received`. [Proposed]

### Session 2026-11-27 (delta server đã có — owner F104/F105/F107/F117, [Confirmed] "chốt hết" phương án an toàn nhất)

Các câu `[Proposed]` ở phiên trên về contract server nay đã có trên main (chỉ thêm, không phá — `oasdiff` 0 breaking). Client đọc theo đây; khi trường vắng (server cũ, checkpoint cũ) vẫn giữ cách đọc an toàn của phiên trên.

- Q: `speed_bonus_enabled`? → A: **Server cung cấp.** `battle.countdown.started` và `battle.state.snapshot` luôn có `speed_bonus_enabled` (boolean) theo luật trận đã ghim (khối ≥ `speed_bonus_min_grade`); không kèm công thức/số điểm bonus (R6). [Confirmed]
- Q: `primary_user_ids`? → A: **Server cung cấp.** Chỉ `team_battle`: `battle.question.shown` và `battle.state.snapshot` (khi đang có câu) có `primary_user_ids` (mỗi đội một người lượt chính của câu, theo thứ tự đội, ≤ 8); 1v1/Timed Quiz không có trường. [Confirmed]
- Q: `encouragement`? → A: **Server cung cấp.** `battle.match.ended` gửi **riêng** cho người vừa thua `loss_streak_encouragement` (3, CONFIG_DEFAULTS) trận liên tiếp (thua/xử thua/bỏ trận; hòa/thắng ngắt chuỗi; `no_contest` bỏ qua) có `encouragement` {`code` = `loss_streak_review`, `message_code` (khóa dịch, vd. `competition.encouragement.loss_streak_review`), `subject_id`, `topic_id`?} — lời hiển thị là chuỗi dịch của client, không có nội dung học tập trong message; người còn lại nhận bản không có trường. Delta cũ `{code, topic_id}` được mở rộng thành shape này (thêm `message_code`, `subject_id`; `topic_id` chỉ khi trận có chủ đề). [Confirmed]
- Q: `chat_channel_ids`? → A: **Server cung cấp.** `matchmaking.match.assigned` (gửi riêng theo đội) và `battle.state.snapshot` (theo người xem) có `chat_channel_ids` {`match`, `team`?} = id tất định `pkg/chatid` của kênh F117; `team` chỉ có ở Team Battle và là kênh đội của chính người nhận. Kênh được module chat mở khi `match.started` (câu đầu) — trước đó gửi tin bị từ chối; client không cần tra `listMyChannels` nữa (vẫn dùng được làm dự phòng). [Confirmed]
- Q: Khóa lựa chọn? → A: **Thống nhất: `id` là khóa chuẩn** (như render của learning và renderer F063). `QuestionRender.options[]` của mọi message `battle.*`/`coop.*` có cả `id` và `option_id` (cùng giá trị, `option_id` deprecated — alias giữ hết v1). Câu trả lời giữ nguyên `AnswerResponse` `{option_id}` mang giá trị `id`. [Confirmed]
- Q: `409 MM_TICKET_ALREADY_ACTIVE` kèm `ticket_id`? → A: **Server cung cấp.** Problem có extension `ticket_id` khi ticket đang giữ có chính người gọi (solo hoặc ticket đội có em); ticket chỉ của bạn khác trong party → không kèm id (không lộ ticket của người khác). [Confirmed]
- Q: Lời thách đấu đến? → A: **Server cung cấp WS** `competition.duel.received` (chỉ người được thách, sau commit `createDuel`, chỉ khi đã qua luật thách đấu — bạn bè cùng tenant, cùng nhóm khối, không chặn nhau, cả hai được chơi): {`duel_id`, `challenger` (`PublicPlayer`: user_id, nickname, avatar, level — R16), `subject_id`, `topic_id`?, `expires_at`}. Không tạo thông báo F123 lưu trữ (duel sống 60 s; offline thì hết hạn) — phương án an toàn nhất, không thêm `NotificationKind`. [Confirmed]
- Q: Mã lỗi giải? → A: **Server cung cấp** `TOURNAMENT_NOT_ELIGIBLE` (403, bị hạn chế thi đấu), `TOURNAMENT_FULL` (409, kể cả đóng sớm vì đủ chỗ), `TOURNAMENT_REGISTRATION_CLOSED` (422, ngoài cửa sổ đăng ký), `TOURNAMENT_CHECK_IN_CLOSED` (422, ngoài cửa sổ check-in) — cùng HTTP status như trước; đã đăng ký → vẫn `409 CONFLICT`; ngoài đối tượng vẫn `404`. Đường đăng ký hộ của người tổ chức giữ `INVALID_STATE_TRANSITION`. [Confirmed]
- Ghi chú cho client: fixture Godot sinh từ ví dụ AsyncAPI (`gen_fixtures`) cần chạy lại ở lượt client tiếp theo (ví dụ mới: `matchmaking.match.assigned` team_battle, `battle.question.shown` team_battle, `battle.match.ended` loss_streak_encouragement, `competition.duel.received`).

### Session 2026-11-27 (client đọc delta server — bỏ cách đọc dự phòng, [Confirmed])

Client Godot đã đọc các trường của phiên trên; các cách đọc dự phòng `[Proposed]` của phiên "triển khai client" được **thay thế** như sau (vẫn đọc an toàn khi trường vắng — server/checkpoint cũ):

- `speed_bonus_enabled`: đọc ở `battle.countdown.started` (cập nhật luật trận) và snapshot; gợi ý "Trả lời đúng và nhanh để được thêm điểm nhé!" chỉ hiện ở giai đoạn chờ/đếm ngược khi cờ `true`, ẩn khi câu hỏi hiện; "Thưởng tốc độ" vẫn chỉ khi `time_bonus > 0`.
- `primary_user_ids`: mỗi đội người trả lời chính câu này có ngôi sao + chữ đậm trong `MatchHeader`; học sinh đọc "Lượt của em" / "Em hỗ trợ".
- `encouragement`: client chỉ nhận `code` đã biết; `message_code` `competition.encouragement.<code>` là khóa `ui.competition.encouragement.<code>` của client (cùng quy ước `ui.notification.<code>` của F123, để `extract_pot` kiểm được), `{subject}`/`{topic}` là tên môn/chủ đề đọc từ `listSubjects`/`listTopics` (không có chủ đề → câu `….subject` chỉ nêu môn); hiện thành dòng thứ hai trong bong bóng Mây ở màn kết quả.
- `chat_channel_ids`: chat trận mở theo id của snapshot, trước snapshot theo `matchmaking.match.assigned`; `listMyChannels` chỉ còn là dự phòng khi server không gửi id (giữ id đã biết khi snapshot sau thiếu trường).
- Khóa lựa chọn: `id` chuẩn; option chỉ có `option_id` vẫn được chuẩn hóa về `id`; câu trả lời vẫn `{option_id: <id>}`.
- `409 MM_TICKET_ALREADY_ACTIVE` có `ticket_id` → `getMatchmakingTicket` của id đó mở thanh hàng chờ; không có id → ticket trong RAM, không có nữa → "Em đang trong hàng chờ rồi."
- `competition.duel.received`: hộp `DuelIncomingPrompt` trên HUD (thẻ công khai của bạn thách, môn/chủ đề, "Luyện tập, không tính hạng", thanh thời gian theo `expires_at`, lớp 1–3 không có số giây), "Nhận lời" → `acceptDuel`, "Để lần sau" → `declineDuel`; tự đóng khi hết hạn; không hiện khi đang thi/thử thách/làm bài/trong trận hoặc tắt popup (cùng quy tắc lời mời nhóm F102).
- Lỗi giải: bốn mã `TOURNAMENT_*` có câu riêng `ui.error.<CODE>`; `409 CONFLICT` nay là "Em đã đăng ký giải này rồi." (không còn hiểu là "đủ chỗ"); `POLICY_DENIED`/`404` → "Giải này chưa dành cho em."; `INVALID_STATE_TRANSITION` → "Chưa tới giờ hoặc đã hết giờ cho việc này."; sau 409/422 chi tiết giải được đọc lại.
- Câu hỏi mở còn lại (không thuộc delta này): bỏ `note` khỏi F109-AC-16 (bảng báo cáo F119 không có chữ tự do) vẫn chờ owner xác nhận.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Chọn chế độ, tìm trận, ready check (Priority: P1) 🎯 MVP

Học sinh lớp 5 vào Đấu trường (NPC hoặc phím `K`), chọn 1v1 môn Toán, bấm Tìm trận, vẫn đi lại trong thế giới khi chờ, bấm "Vào trận" khi có ready check (US-MM-001).

**Why this priority**: Cửa vào duy nhất của mọi trận; không có thì không chơi được.

**Independent Test**: `hs.an.3a` và `hs.binh.3a` cùng tìm trận 1v1 Toán → cả hai thấy ready check → vào BattleArena; một người hủy hàng chờ → thanh trạng thái biến mất.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F109-AC-01 | KHI học sinh mở Arena, game client PHẢI hiển thị `ModeCard` đúng các chế độ server trả (khả dụng/khóa kèm lý do), `TierBadge` từ `getMyRatings` và `SubjectTopicPicker`; client PHẢI không tự quyết chế độ theo khối | FR-CMP-001 · [STUDENT_GAME_UX_SOCIAL §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) · [COMPETITION_SYSTEM §2](../../docs/01-game-design/COMPETITION_SYSTEM.md) | TC-CMP-001 |
| F109-AC-02 | KHI học sinh bấm Tìm trận, `MatchmakingService` PHẢI gọi `createMatchmakingTicket` (`mode`, `subject_id`, `topic_id?`) với `Idempotency-Key`, hiện `QueueStatusBar` nổi trên HUD theo `matchmaking.ticket.updated` (`wait_ms`, `search_widened`) và cho học sinh di chuyển trong thế giới; Hủy PHẢI gửi `matchmaking.ticket.cancel` (fallback `cancelMatchmakingTicket`) | FR-MM-004 · [GAME_NETWORKING §7](../../docs/08-game-client/GAME_NETWORKING.md) | TC-MM-001 |
| F109-AC-03 | KHI nhận `matchmaking.proposal.created`, game client PHẢI hiện `ReadyCheckDialog` đếm ngược tới `deadline_at` theo `Clock` server; "Vào trận" PHẢI gửi `matchmaking.proposal.accept` (fallback `acceptMatchProposal`); KHI nhận `matchmaking.proposal.cancelled` với `requeued = true` PHẢI quay về thanh hàng chờ; KHI nhận `matchmaking.match.assigned` PHẢI chuyển sang `BattleArena` | FR-MM-004 · AC-CMP-001 · [GAME_NETWORKING §6, §7](../../docs/08-game-client/GAME_NETWORKING.md) | TC-CMP-002 |
| F109-AC-04 | NẾU `createMatchmakingTicket` trả `409 MM_TICKET_ALREADY_ACTIVE`, THÌ game client PHẢI mở thanh hàng chờ hiện có qua `getMatchmakingTicket`; NẾU ticket `expired` hoặc `cancel_reason = timeout`, THÌ PHẢI gợi ý "Thử chế độ khác"/"Luyện với Robot" (nhãn unranked rõ ràng); NẾU `403`, THÌ PHẢI hiện thông điệp `ui.error.<CODE>` | FR-MM-005 · [STUDENT_GAME_UX_SOCIAL §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | TC-MM-001 |

---

### User Story 2 — Chơi trận 1v1 và đấu đội theo server (Priority: P1)

Hai học sinh nhận cùng câu hỏi, chọn đáp án một lần, thấy "Đã ghi nhận", đúng/sai và điểm chỉ khi server công bố; mất mạng thì tự vào lại trận.

**Why this priority**: Lõi trải nghiệm thi đấu; phải đúng server-authoritative (constitution IV).

**Independent Test**: Chạy trận 1v1 fixture AsyncAPI trên scene test và trận thật với hai client headless; ngắt mạng một client 10 giây giữa câu 3 → UI dựng lại đúng từ snapshot.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F109-AC-05 | KHI nhận `battle.question.shown`, `BattleArena` PHẢI render `question` bằng `QuestionRendererRegistry` (F063), `MatchHeader` PHẢI hiển thị điểm hai bên/đội và tiến độ câu chỉ từ message server, thanh thời gian theo `deadline_at − Clock.server_now()` | FR-CMP-001 · ADR-0016 · [GAME_NETWORKING §6](../../docs/08-game-client/GAME_NETWORKING.md) | TC-CMP-003 |
| F109-AC-06 | KHI học sinh chọn đáp án (chạm hoặc phím `1`–`4`), `BattleClient` PHẢI gửi đúng một `battle.answer.submit` (`match_id`, `question_instance_id`, `response`, `client_elapsed_ms` chỉ tham khảo), khóa input và hiện "Đã ghi nhận"; HỆ THỐNG PHẢI không hiển thị đúng/sai/điểm trước `battle.answer.evaluated`/`battle.question.revealed` và không gửi điểm từ client | AGENTS R5 · ADR-0016 · [GAME_NETWORKING §7](../../docs/08-game-client/GAME_NETWORKING.md) | TC-CMP-007 |
| F109-AC-07 | NẾU nhận lỗi `ANSWER_LATE`, `ANSWER_ALREADY_SUBMITTED` hoặc `MATCH_NOT_ACTIVE`, THÌ game client PHẢI giữ trạng thái đã khóa, hiện thông điệp thân thiện theo `code` và không gửi lại | AC-CMP-006 · [GAME_NETWORKING §7](../../docs/08-game-client/GAME_NETWORKING.md) | TC-CMP-007 |
| F109-AC-08 | KHI nhận `battle.answer.evaluated`/`battle.question.revealed`, game client PHẢI hiển thị ✓/✗ (không chỉ dùng màu), đáp án đúng, điểm câu; dòng "thưởng tốc độ" PHẢI chỉ hiện khi server gửi `time_bonus > 0` | FR-CMP-006 · [COMPETITION_SYSTEM §3.1](../../docs/01-game-design/COMPETITION_SYSTEM.md) | TC-CMP-007 |
| F109-AC-09 | TRONG KHI `speed_bonus_enabled = false`, game client PHẢI không hiển thị chỉ dẫn/hiệu ứng khuyến khích trả lời nhanh; TRONG KHI grade band `g1_3`, PHẢI không hiển thị đếm ngược số giây (chỉ thanh màu) và vùng chạm đáp án ≥ 64 dp | FR-CMP-006 · NFR-A11Y-002 · [STUDENT_GAME_UX_SOCIAL §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | — |
| F109-AC-10 | Ở NƠI `mode = team_battle`, game client PHẢI hiển thị điểm theo đội và đánh dấu lượt chính/lượt hỗ trợ của em theo dữ liệu server (`primary_user_ids`, delta) | FR-CMP-001 · [COMPETITION_SYSTEM §3.2](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |
| F109-AC-11 | KHI nhận `battle.participant.status_changed` `connected = false` của người khác, game client PHẢI hiện "Đối thủ đang kết nối lại…" tới `grace_until`; KHI chính em reconnect, PHẢI gửi `battle.state.request` và dựng lại toàn bộ UI từ `battle.state.snapshot` (không dùng state cục bộ); `battle.match.paused`/`resumed` PHẢI hiện/ẩn lớp phủ chờ | FR-CMP-005 · FR-MP-005 · AC-CMP-003 | TC-CMP-004, TC-CMP-005 |
| F109-AC-12 | TRONG KHI ở `BattleArena`, game client PHẢI tạm dừng thế giới 3D (`QualityManager.suspend_world("battle")`), dùng nền đấu trường trong ngân sách [STUDENT_GAME_UX_SOCIAL §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) và cập nhật theo sự kiện, không polling | [PERFORMANCE_BUDGET §7](../../docs/08-game-client/PERFORMANCE_BUDGET.md) · [STUDENT_GAME_UX_SOCIAL §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | TC-PERF-018 |
| F109-AC-13 | HỆ THỐNG PHẢI có vùng `MatchChatStrip` (component của F119, channel `match`/`team`) luôn hiện trong trận, tự thu nhỏ khi câu hỏi hiển thị và trên mobile không che vùng đáp án; PHẢI không có tùy chọn tắt chat trận | ADR-0020 · AGENTS R15 · [COMPETITION_SYSTEM §3.1](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |

---

### User Story 3 — Kết quả trận, tier, lịch sử (Priority: P1)

Sau trận học sinh thấy Thắng/Hòa/"Chưa thắng lần này", câu cần ôn, Competition Points, tier (không số rating), có thể báo cáo/chặn đối thủ, xem lại lịch sử trận.

**Why this priority**: Kết thúc vòng chơi; nơi dễ phát sinh áp lực và lộ rating nhất.

**Independent Test**: Nhận `battle.match.ended` fixture (thắng, thua, `cancelled`) → kiểm màn kết quả; quét cây node không có chuỗi rating số.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F109-AC-14 | KHI nhận `battle.match.ended` `status = completed`, `ResultScreen` PHẢI hiển thị kết cục bằng ngôn từ động viên, điểm từng câu, câu cần ôn (link ôn tập), Competition Points và tier sau khi tải lại `getMyRatings`, nút [Chơi lại] [Về Đấu trường]; NẾU `status` là `cancelled`/`aborted`, THÌ PHẢI hiện "Trận đã hủy, không tính kết quả" | FR-CMP-001 · [STUDENT_GAME_UX_SOCIAL §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | TC-CMP-006 |
| F109-AC-15 | game client PHẢI không hiển thị số rating, RD hay rating delta ở bất kỳ màn nào với mọi khối: model PHẢI bỏ trường `rating`, `rd`, `rating_delta` khi parse; tên tier lấy từ dữ liệu server (`i18nText`), không hardcode | FR-RNK-003 · FR-RNK-002 · AC-CMP-101 · [COMPETITION_SYSTEM §8.2](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |
| F109-AC-16 | KHI học sinh bấm Báo cáo/Chặn đối thủ trên màn kết quả, game client PHẢI gọi `createReport` (`target_type = user`, `reason` trong enum, `note` ≤ `maxLength`, có bộ đếm) với `Idempotency-Key` hoặc `blockUser`; KHI server gửi thông điệp động viên (`encouragement`, delta), PHẢI hiện thông điệp + gợi ý ôn chủ đề | AGENTS R10 · ADR-0041 · [COMPETITION_SYSTEM §12](../../docs/01-game-design/COMPETITION_SYSTEM.md) | TC-TEN-024 |
| F109-AC-17 | KHI mở Lịch sử trận, game client PHẢI hiển thị `listMyMatches` (lọc `mode`, cursor) và chi tiết `getMatch`; người chơi khác chỉ hiện nickname, avatar, tier | FR-CMP-001 · AGENTS R16 · [COMPETITION_SYSTEM §14.2](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |

---

### User Story 4 — Bảng xếp hạng theo khối và chính sách trường (Priority: P1)

Học sinh lớp 3 mở Bảng xếp hạng thấy "Mục tiêu của khối" — thanh tiến độ tập thể của khối mình trong trường và phần em đã góp (không thứ hạng cá nhân, không tên bạn nào); học sinh lớp 7 thấy top + vị trí quanh mình; trường tắt một bảng thì bảng biến mất (US-LDB-001).

**Why this priority**: Rủi ro an toàn trẻ em cao nhất trong feature (so sánh công khai, lộ danh tính).

**Independent Test**: `hs.an.3a`, `hs.em.4a`, `hs.dung.7a` mở cùng màn; `ad.truong.a` tắt "Mục tiêu của khối" trong policy trường → `hs.an.3a` không còn thấy bảng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F109-AC-18 | KHI mở Bảng xếp hạng, `LeaderboardService` PHẢI lấy danh sách bảng từ server (`listLeaderboards`, delta) và chỉ hiển thị bảng server trả (đã lọc khối và `disabled_leaderboard_definitions`); danh sách rỗng PHẢI hiện trạng thái rỗng thân thiện | FR-LDB-004 · [COMPETITION_SYSTEM §9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |
| F109-AC-19 | `LeaderboardView` PHẢI render theo `display_mode` server trả: `grade_goal` → `GradeGoalBar` (thanh tiến độ tập thể của khối + phần em đã góp + số bạn đã góp sức khi server trả), không thứ hạng cá nhân, không nickname người khác (ADR-0095); `top_n` → top N + dòng của em; `percentile` → "Em đang ở nhóm X%"; `ranked_around_me` → top + quanh em; client PHẢI không tự tính hạng hay cắt danh sách | FR-LDB-004 · FR-LDB-005 · [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |
| F109-AC-20 | Mỗi dòng bảng PHẢI chỉ hiển thị nickname, avatar; PHẢI không hiển thị tên thật, mã học sinh, tên lớp, và không tên trường/khối chi tiết ở scope liên trường | FR-LDB-003 · AGENTS R16 · [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |
| F109-AC-21 | NẾU `getLeaderboard` trả `404`, THÌ game client PHẢI hiện "Bảng này hiện không mở" và bỏ tab đó; NẾU `stale = true`, THÌ PHẢI gắn nhãn "Đang cập nhật"; client PHẢI chỉ gửi `leaderboard_id`/`period_key` do server cung cấp | FR-LDB-004 · AGENTS R8 | TC-TEN-001 |
| F109-AC-22 | HỆ THỐNG PHẢI không có thông báo kiểu "bạn bị vượt qua" và không tự làm mới bảng định kỳ (chỉ khi mở hoặc kéo làm mới) | NFR-SAFE-003 · AGENTS R19 · [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |

---

### User Story 5 — Giải đấu: xem, đăng ký, check-in, bracket (Priority: P2)

Học sinh xem giải khối/thử thách khối do nhà trường tạo, đăng ký, check-in khi mở cửa sổ, xem bracket, nhận thông báo trận giải và vào qua ready check.

**Why this priority**: Giải chạy theo lịch, không chặn trận thường.

**Independent Test**: Seed một giải khối 3 của TH Demo A (`grade_challenge`, loại trừ `hs.chi.3b`) ở `registration_open`; `hs.an.3a` đăng ký, check-in, xem "con đường" bracket; `hs.chi.3b` (bị loại trừ) và `hs.em.4a` (khối 4) không thấy giải.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F109-AC-23 | KHI mở thẻ Giải đấu, game client PHẢI hiển thị `listTournaments` (cursor) trong phạm vi của em với trạng thái, lịch và chi tiết từ `getTournament` (delta) | FR-TRN-002 · [COMPETITION_SYSTEM §10.3](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |
| F109-AC-24 | KHI học sinh đăng ký, rút hoặc check-in, `TournamentService` PHẢI gọi `createTournamentEntry`/`withdrawTournamentEntry`/`checkInTournamentEntry` (delta) với `Idempotency-Key`, chỉ bật nút check-in trong cửa sổ server trả; NẾU lỗi `TOURNAMENT_NOT_ELIGIBLE`/`TOURNAMENT_REGISTRATION_CLOSED`/`TOURNAMENT_FULL`/`TOURNAMENT_CHECK_IN_CLOSED` (delta), THÌ PHẢI hiện thông điệp theo `code` | FR-TRN-002 · AGENTS R10 · [COMPETITION_SYSTEM §10.4](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |
| F109-AC-25 | KHI xem bracket, game client PHẢI vẽ cây từ `getTournamentBracket` (delta) cho `g6_7`/`g8_9` và dạng "con đường" đơn giản cho `g1_3`/`g4_5`, chỉ hiện nickname/tên đội | FR-LDB-003 · [COMPETITION_SYSTEM §10.8](../../docs/01-game-design/COMPETITION_SYSTEM.md) | — |
| F109-AC-26 | KHI nhận `notification.item.created` `kind = tournament_update`, game client PHẢI hiện toast mở giải; trận giải PHẢI vào qua ready check như F109-AC-03 | FR-NTF-001 · [COMPETITION_SYSTEM §10.6](../../docs/01-game-design/COMPETITION_SYSTEM.md) | TC-CMP-002 |

---

### User Story 6 — Thách đấu bạn (Priority: P3)

Học sinh thách đấu một bạn (unranked); bạn nhận lời mời, chấp nhận hoặc từ chối trong thời hạn.

**Why this priority**: Kênh vào trận phụ; cần danh sách bạn bè (F095).

**Independent Test**: `hs.an.3a` thách `hs.binh.3a` → chấp nhận → vào trận; chờ quá hạn → "Lời thách đấu đã hết hạn".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F109-AC-27 | KHI học sinh thách đấu bạn, game client PHẢI gọi `createDuel` (`opponent_id`, `subject_id`, `topic_id?`) với `Idempotency-Key`; người nhận PHẢI có hộp `acceptDuel`/`declineDuel`; NẾU `410 DUEL_EXPIRED`, THÌ hiện "Lời thách đấu đã hết hạn"; NẾU `SOCIAL_SCOPE_VIOLATION`, THÌ hiện thông điệp chung không nêu lý do | FR-CMP-001 · [COMPETITION_SYSTEM §2](../../docs/01-game-design/COMPETITION_SYSTEM.md) | TC-CMP-010 |

---

### User Story 7 — Bảo mật, tenant, song ngữ (Priority: P1)

**Why this priority**: Constitution III, V, VI, VII.

**Independent Test**: Unit `ApiClient`/service với fake; dùng `match_id` của `hs.dung.7a` khi đăng nhập `hs.an.3a`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F109-AC-28 | HỆ THỐNG PHẢI gửi `X-Tenant-Id` phiên ở mọi request REST, `Idempotency-Key` cho mọi thao tác ghi (giữ nguyên khi thử lại), message realtime chỉ qua `RealtimeClient`; client PHẢI không kết nối NATS/Valkey | AGENTS R8, R10, R25 · ADR-0065 | TC-TEN-001 |
| F109-AC-29 | NẾU nhận `404` cho `match_id`/`tournament_id`/`leaderboard_id` ngoài phạm vi, THÌ game client PHẢI hiện "Không tìm thấy" và không hiển thị dữ liệu nào; NẾU `403 GAME_ACCESS_STUDENT_ONLY`/`401`, THÌ xử lý chung theo F061 | AGENTS R7, R23 · ADR-0031 | TC-AUTH-014 |
| F109-AC-30 | Mọi chuỗi UI PHẢI qua khóa `ui.arena.*`, `ui.leaderboard.*`, `ui.tournament.*` (vi/en) và chịu được chuỗi dài hơn 35%; telemetry PHẢI không chứa PII hay nội dung câu trả lời | NFR-OBS-003 · ADR-0056 · AGENTS R14 | TC-I18N-005, TC-I18N-006 |

### Edge Cases

- NẾU mất WebSocket khi đang chờ hàng, THÌ client PHẢI hỏi `getMatchmakingTicket` sau reconnect; ticket mất (FR-MM-006) → hiện "Hàng chờ đã kết thúc, em tìm lại nhé" (F109-AC-04).
- NẾU nhận message có `state_version` cũ hơn snapshot hiện tại, THÌ bỏ qua; thiếu version → `battle.state.request` (F109-AC-11).
- NẾU `battle.match.resumed` có `replay_current_question = true`, THÌ mở khóa và hiển thị lại câu hiện tại.
- NẾU tên tier/nickname dài, THÌ rút gọn có tooltip/nhấn giữ (F109-AC-30).
- NẾU học sinh đóng app giữa trận rồi mở lại trong grace, THÌ sau đăng nhập và `battle.state.request` vào lại trận (F109-AC-11).
- NẾU `grade_goal.contributors_count` bị server ẩn (< 5 học sinh góp sức), THÌ chỉ hiện thanh tiến độ và phần của em, không hiện ô trống hay số 0.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CMP-001 | UI các chế độ 1v1, đội, co-op, giải; lịch sử trận; thách đấu bạn | [COMPETITION_SYSTEM §2–§3](../../docs/01-game-design/COMPETITION_SYSTEM.md) |
| FR-CMP-006 | Không hiển thị/khuyến khích tốc độ khi server tắt bonus; `g1_3` không đếm số giây | [COMPETITION_SYSTEM §3.1](../../docs/01-game-design/COMPETITION_SYSTEM.md) |
| FR-RNK-003 | Chỉ hiển thị tier, không số rating (mọi khối) | [COMPETITION_SYSTEM §8.2](../../docs/01-game-design/COMPETITION_SYSTEM.md) |
| FR-LDB-003 | Bảng/bracket chỉ nickname, không trường/khối chi tiết liên trường | [COMPETITION_SYSTEM §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md) |
| FR-LDB-004 | Hiển thị đúng bảng server trả theo khối và policy trường | [COMPETITION_SYSTEM §9.2, §9.4](../../docs/01-game-design/COMPETITION_SYSTEM.md) |
| FR-LDB-005 | Hiển thị mục tiêu tập thể của khối (`grade_goal`)/nỗ lực theo `display_mode` | [COMPETITION_SYSTEM §9.1](../../docs/01-game-design/COMPETITION_SYSTEM.md) |
| FR-RNK-002 | Không trộn rating với số liệu học tập trên UI | [COMPETITION_SYSTEM §6](../../docs/01-game-design/COMPETITION_SYSTEM.md) |
| FR-MM-004, FR-MM-005 | Hàng chờ, ready check, gợi ý khi chờ lâu (phía client) | [MATCHMAKING](../../docs/03-social-multiplayer/MATCHMAKING.md) |
| FR-CMP-005, FR-MP-005 | Hiển thị mất kết nối, resume trận | [MULTIPLAYER_GAMEPLAY](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| FR-TRN-002 | Đăng ký, check-in, xem bracket (phía học sinh) | [COMPETITION_SYSTEM §10](../../docs/01-game-design/COMPETITION_SYSTEM.md) |
| FR-NTF-001 | Toast thông báo trận giải | [COMPETITION_SYSTEM §10.6](../../docs/01-game-design/COMPETITION_SYSTEM.md) |

### Key Entities

- **ArenaMode**: chế độ thi đấu khả dụng với em, lý do khóa, cờ thưởng tốc độ.
- **MatchmakingTicket / Proposal**: lượt chờ ghép và đề xuất trận cần xác nhận.
- **BattleView**: trạng thái hiển thị trận dựng từ snapshot/message server (câu hiện tại, hạn, điểm, kết nối).
- **MatchSummary**: kết quả trận (kết cục, điểm, Competition Points), không rating.
- **TierView**: mã + tên tier, provisional.
- **LeaderboardView**: bảng, kỳ, chế độ hiển thị, các dòng (nickname, avatar, giá trị), mục tiêu tập thể của khối (không tên).
- **TournamentView / BracketView / MyEntry**: giải, cây đấu, lượt đăng ký của em.
- **Duel**: lời thách đấu bạn có hạn.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `mm_ready_check` | Thời hạn ready check (client hiển thị theo `deadline_at`) |
| `battle_1v1_format`, `team_battle_format` | Bố cục số câu/tiến độ (giá trị từ server) |
| `speed_bonus_min_grade` | Quy tắc server đặt `speed_bonus_enabled` |
| `tiers` | Tên/ngưỡng tier (server), ẩn số rating thô |
| `leaderboard_visibility` | Server chọn `display_mode` theo khối |
| `disabled_leaderboard_definitions` | Bảng bị trường tắt không xuất hiện |
| `tournament_timers` | Cửa sổ check-in, no-show (server) |
| `disconnect_grace` | Thời gian hiển thị "đang kết nối lại" (`grace_until` server) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không logic nghiệp vụ trong scene; service `RefCounted` + model typed | AGENTS R1 · [GODOT_ARCHITECTURE §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |
| Điểm, kết quả, thứ hạng chỉ từ server; không gửi điểm | AGENTS R5, R6 · ADR-0016 |
| Theme/token game, không hardcode style | AGENTS R32 · ADR-0047 |
| Font ≥ ngưỡng `g1_3`, ✓/✗ không chỉ màu | NFR-A11Y-001 |
| Không dark pattern, không so sánh gây áp lực | AGENTS R19 · NFR-SAFE-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Học sinh từ màn Arena tới lúc vào trận trong ≤ 3 thao tác (chọn chế độ → Tìm trận → Vào trận).
- **SC-002**: 0 trường hợp hiển thị số rating, RD hoặc tên thật/tên trường trong bộ scene test và E2E (quét cây node + ảnh chụp).
- **SC-003**: Sau mất mạng ≤ grace, 100% trận thử nghiệm dựng lại đúng câu hiện tại và điểm từ snapshot.
- **SC-004**: Học sinh lớp 1–3 không thấy thứ hạng cá nhân hay đếm ngược số giây ở 100% màn thi đấu.
- **SC-005**: BattleArena giữ FPS mục tiêu của preset trên thiết bị tầm trung tham chiếu.

## Ngoài phạm vi

- Matchmaking, battle engine, chấm điểm, rating, leaderboard, tournament phía server: F104, F105, F106, F107.
- Panel/lời mời/lobby party, vào trận từ party: F102. Hàng chờ của party leader dùng lại `MatchmakingService` của F109.
- Component chat trận (`MatchChatStrip`, quick message, báo cáo tin): F119; F109 chỉ đặt vùng hiển thị.
- Tạo/tổ chức giải, override kết quả (portal): F108 và workspace C/A; Event center, season track: F077.
- Co-op Challenge gameplay (thử thách thời gian của party): F103/F076; F109 chỉ hiện thẻ chế độ và vào qua hàng chờ.
- Hub nhiều người, `RealtimeClient`: F092.

## Assumptions

- [Confirmed] Delta contract ở [contracts/README.md §6](contracts/README.md) được owner F105/F106/F107 chấp nhận; trước đó dùng Prism mock + fixture AsyncAPI gắn nhãn MOCK (CODE_PLAN §2), F109 chỉ `Done` với backend thật.
- [Confirmed] Seed thêm một học sinh 7A (cùng khối với `hs.dung.7a`) để kiểm 1v1 khối 6–9 và `ranked_around_me`; nếu chưa có, dùng bot unranked.
- `RealtimeClient`, `Clock` (F061/F092) và `QuestionRendererRegistry` (F063) có sẵn.

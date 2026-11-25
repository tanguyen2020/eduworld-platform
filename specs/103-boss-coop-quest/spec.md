# Feature Specification: F103 — Boss battle nhiều phase và quest co-op (shared progress, bước `interdependent`)

**Feature Branch**: `103-boss-coop-quest` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `game` · **Mốc**: M4a · **Phụ thuộc**: F076, F101 (và F072 quest engine, F074 Reward Engine)
**Input**: Boss nhiều phase server-authoritative, chơi solo hoặc theo party; đóng góp cá nhân/nhóm, thử lại phase không phạt, thưởng chỉ cho người đủ đóng góp; quest co-op theo `shared_progress_policy`; bước `interdependent` mỗi thành viên giữ một phần dữ kiện; kết quả học tập luôn riêng từng học sinh; operation facade cho Party Lobby (F101). FR nguồn: FR-CHL-002, FR-CHL-003, FR-QST-002, FR-QST-007, FR-QST-009, FR-PTY-013.

## Clarifications

### Session 2026-11-16

- Q: Encounter boss cho party tạo qua `createBossEncounter` (`mode = party`) hay qua Party Lobby? → A: Chỉ qua Party Lobby (`startPartySession` của F101 → facade `startPartyActivity`); `createBossEncounter` chỉ còn `mode = solo` (delta contract). Tránh hai cơ chế Ready và party không chuyển `in_activity` — [research.md](research.md) R2.
- Q: Câu trả lời boss chấm thế nào? → A: module game tạo bộ câu hỏi riêng mỗi thành viên mỗi lượt phase qua module learning (question plan, `mode = challenge`) và chấm từng câu bằng `scorePlanAnswer`; module learning ghi learning record đúng học sinh (ADR-0016) — research R1.
- Q: Dữ kiện của role (`clue_content_ref`) lấy ở đâu để không lộ cho người khác? → A: module game lấy bản render dữ kiện đã publish qua facade `learning` và chỉ trả cho người giữ role (hoặc sau khi chia sẻ) — research R3.
- Q: Tiến độ chung của quest co-op lưu ở đâu? → A: Bản ghi phiên quest chung của party trong `eduworld_game` (mới), cạnh `QuestProgress` cá nhân — research R4.
- Q: `allow_continue_on_disconnect` của quest (MULTIPLAYER_GAMEPLAY §9.2) không có trong `quest.schema.json`? → A: Dùng quy tắc QUEST_SYSTEM §6: tiếp tục nếu `party_mode ≠ party_required` hoặc số người còn lại ≥ `party_size.min`.

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Bỏ khái niệm lớp ảnh hưởng F103? → A: Gần như không: điều kiện hoạt động party xét theo khối/grade band của từng thành viên (không theo lớp); party lớp 1–3 cùng khối với leader theo [PARTY_SYSTEM](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) ([ADR-0095 mục 5 (D2)](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md#5-game-xã-hội-chat-d2-d5-d7-confirmed--owner-2026-11-21)). F103-AC-02 đổi chữ "khối/lớp thực" thành "khối/trường thực" (guard R16), không đổi hành vi.

### Session 2026-11-24 (triển khai) — [Confirmed] (owner "chốt hết")

- Q: Bảng nội dung boss (`boss_definitions`/`boss_versions`) chưa có ở F076/F078? → A: F103 tạo chúng trên lõi phát hành F071 (`item_type = boss`, kiểm `boss.schema.json` + quy tắc domain: tổng `energy_share` = 1, `completion_rule` khớp requirement, `party_size` 1–8), config type `bosses` của Game Operations Console (dùng module loại chung), gán tenant `resource_type = boss` (ADR-0029). Chỉ phục vụ requirement `kind = activity`; `minigame`/`interaction`/`group_puzzle` bị từ chối khi publish (chưa có feature).
- Q: "Requirement xong" và `completion_rule` tính thế nào? → A: requirement xong với một thành viên khi số câu đúng được tính ≥ ⌈`min_correct_ratio` × min(`min_items`, số câu plan)⌉, mặc định tỷ lệ của `quest_pass_rule` (0,7); `all` = mọi thành viên còn trong encounter xong mọi requirement; `any_n` = n requirement khác nhau đã xong bởi ít nhất một người; `collective_count` = tổng câu đúng được tính ≥ `collective_target` và mọi người còn trong encounter ≥ `min_contribution`. Phase không khai báo `min_contribution` dùng 1 câu đúng cho `eligible`. Thử lại phase đặt lại đóng góp của phase đó; `eligible` = đạt `min_contribution` ở mọi phase được vượt khi người đó còn trong encounter (cột `cleared`). Mỗi requirement mỗi lượt tối đa 10 câu (`boss_requirement_questions`).
- Q: Chạy encounter ở đâu, live state? → A: Trạng thái ở PostgreSQL (dòng encounter + bảng con là checkpoint, R9); lệnh WS do core-api xử lý trong transaction khóa dòng encounter; room host (`boss`, `party_quest`) chỉ chạy timer (hạn Ready, phase, tổng, grace) và kiểm tra mỗi 5 s; `src/backend/pkg/roomhost` đọc lại thời điểm hạn từ checkpoint sau mỗi timer không kết thúc room. Không dùng cột `checkpoint jsonb`/`owner_node` và kênh `ew:rt:room:<id>:out`: mọi message gửi từng thành viên qua `ew:rt:user:<id>` (≤ 8 người) — câu hỏi và kết quả từng câu chỉ tới chính người đó.
- Q: "Mất kết nối" của thành viên đo bằng gì? → A: Thành viên online khi có game session active với heartbeat trong `game_session_heartbeat_timeout` (heartbeat do realtime gửi khi WS sống); grace tính từ lần kiểm tra đầu thấy offline; quay lại từ thiết bị khác (session mới) là online lại.
- Q: Gọi `createQuestionPlan` cho boss với khóa nào? → A: `match_id` mang khóa plan UUIDv5 của (encounter, phase, lượt, học sinh, requirement); `mode = boss`, `context_type = boss_encounter`, `context_id = encounter_id`, `attempt_mode = challenge`, `activity_ref` theo `per_grade`; một participant mỗi plan. `question_instance_id` = `<plan_id>:<index>`. Module learning trả `rapid_guess` trong `scorePlanAnswer` (ngưỡng `rapid_guess_min_response_ms`); câu `rapid_guess` vẫn là learning record nhưng không tính đóng góp, ghi audit `boss.integrity_flagged`.
- Q: Lượt boss trong ngày đếm thế nào? → A: `challenge_attempts_per_day` theo (học sinh, boss, ngày ICT theo `quest_reset_time`), mọi encounter đã tạo kể cả bỏ dở — trừ encounter `abandoned` do lỗi module learning (`plan_failure`).
- Q: Phần thưởng boss qua Reward Engine ra sao? → A: consumer `game.reward_engine_game` nhận `boss.encounter_completed` (`result = defeated`) và với mỗi người `eligible` áp rule đã publish của event (source `boss.defeated`) cùng "rule nội dung" sinh từ `defeat_rewards` (+ `first_defeat_rewards` khi `first_defeat`) của version ghim — cùng cap `xp_daily_cap`, cap Xu của applier, giảm dần theo version; idempotent theo `(encounter_id, student_id, rule)` của `xp_ledger`. `first_defeat` = học sinh chưa từng hạ boss này khi `eligible`.
- Q: Mã lỗi khi thiếu `Idempotency-Key`? → A: `428 IDEMPOTENCY_KEY_REQUIRED` (quy ước chung của `pkg/httpx` và facade, API_DESIGN §4.1) — đáp ứng F103-AC-28 về status 428.
- Q: Lệnh WS với encounter không thuộc mình? → A: encounter không có trong tenant của phiên → `PERMISSION_DENIED` (không lộ tồn tại); cùng tenant nhưng không là thành viên → `ROOM_NOT_MEMBER`; câu hỏi của thành viên khác → `PERMISSION_DENIED`; REST trả `404`.
- Q: `reason_code` `party_size_out_of_range`/`sanctioned`? → A: cỡ party là thuộc tính của hoạt động (`min_members`/`max_members`; `startPartyActivity` ngoài khoảng → `422 PARTY_ACTIVITY_NOT_ELIGIBLE` kèm min/max), không thêm mã theo thành viên; `sanctioned` chưa có nguồn trong module game (enum giữ nguyên). Danh sách ứng viên giới hạn 50 trong truy vấn (contract `maxItems: 50`, không có cursor).
- Q: Quest co-op — `activity_ref`, tiến độ có sẵn, version? → A: `activity_ref` là `code` của quest; thành viên đang có `QuestProgress` active của quest thì dùng tiếp (gắn `party_id`), không thì bắt đầu mới ghim version hiện hành; bước chung áp theo version ghim của từng người (bỏ qua nếu version không có bước đó).
- Q: `assignee` mặc định của bước khung trong policy chia sẻ? → A: `any_member` (chia sẻ); `each_member` khai báo tường minh thì không chia sẻ. Bước học tập không bao giờ chia sẻ.
- Q: `party.disbanded` khi quest không khai báo `failure_conditions party_disbanded`? → A: tiến độ giữ nguyên (QUEST_SYSTEM §2.5: thất bại mặc định không mất tiến độ); phiên đóng `abandoned`. Có khai báo: `reset_step` (mặc định) hoặc `fail_quest` (phiên `failed`).
- Q: Ai hoàn thành bước `collective_target`/`interdependent` của party? → A: consumer `game.party_quest_on_activity_graded` (consumer F072 bỏ qua hai loại bước này khi progress có `party_id`); không còn phiên chung thì áp `pass_rule` của từng người. Phân vai khi bước chuyển `active` lần đầu; leader được gán lại vai cả ở chế độ `auto`.
- Q: Thẻ dữ kiện render gì? → A: `getLearningMaterialRenders` trả block `rich_text`/`media` của version đã publish (version cho trước hoặc con trỏ hiện hành); `worked_example`/`activity` bị bỏ (có lời giải). Tiêu đề role của thẻ chưa chia sẻ vẫn hiện (không phải dữ kiện), nội dung thì không.
- Q: `QuestProgress.party_progress` trong REST? → A: điền trong push `quest.progress.updated` (AC-15); field REST giữ trong contract, chưa điền ở release này.
- Q: `event_boss.phase_cleared`/`boss_phases_cleared` của F077? → A: không thuộc spec F103 — để lại cho live-ops (event `boss.phase_completed` đã có để nối sau).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Boss nhiều phase, solo và party (Priority: P1)

Học sinh (một mình hoặc cùng party) đánh boss của khu: mỗi phase trả lời câu hỏi theo khối của mình, thanh năng lượng boss giảm; hết giờ phase thì thử lại với câu mới; hạ boss thì người đủ đóng góp nhận thưởng.

**Why this priority**: Boss là đỉnh mỗi chương truyện và là hoạt động party chính của M4a.

**Independent Test**: Party 2 người (`hs.an.3a`, `hs.binh.3a`) start boss mẫu 2 phase từ lobby → cả hai nhận câu hỏi riêng; trả lời đủ `collective_target` → phase 2; hạ boss → `boss.encounter_completed` có `eligible` từng người, mỗi người một learning record.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F103-AC-01 | KHI học sinh gọi `createBossEncounter` (`mode = solo`, `Idempotency-Key`) với boss khả dụng (khối thuộc `grade_levels`, khu đã mở, boss không `party_required`), module game PHẢI tạo encounter `lobby` ghim `boss_version_id`, đặt hạn tổng theo giờ server và trả `201`; NẾU không đủ điều kiện, THÌ PHẢI trả `403 CHALLENGE_NOT_ELIGIBLE`; `mode = party` qua route này PHẢI trả `422 VALIDATION_FAILED` | FR-CHL-002 · [CHALLENGE_BOSS_SYSTEM §3.3, §3.6](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | — |
| F103-AC-02 | KHI module competition gọi `listPartyActivityCandidates`/`checkPartyActivityEligibility`, module game PHẢI trả theo từng thành viên `eligible` + `reason_code` (khối, grade band, tiên quyết, khu chưa mở, hết lượt), `min_members`/`max_members` theo `party_size`, không trả khối/trường thực của học sinh (ADR-0095: không còn lớp) | FR-CHL-002 · FR-QST-007 · AC-PTY-005 | TC-PTY-006 |
| F103-AC-03 | KHI module competition gọi `startPartyActivity` loại `boss` (`Idempotency-Key = party_activity_id`), module game PHẢI kiểm lại điều kiện mọi thành viên, tạo encounter `party` với danh sách thành viên và trả `external_ref = encounter_id`, `room_type = boss`, `scene_ref`; gọi lại cùng key PHẢI trả cùng encounter | FR-CHL-002 · [PARTY_SYSTEM §6.1](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-006 |
| F103-AC-04 | KHI mọi thành viên đã gửi `boss.encounter.ready` (người chưa sẵn sàng quá `boss_reconnect_grace` coi là vắng, còn lại ≥ `party_size.min`), module game PHẢI vào phase tiếp theo, tạo bộ câu hỏi riêng cho từng thành viên theo khối của chính người đó (`per_grade`), gửi câu hỏi không có đáp án tới từng người và đẩy `boss.phase.started` có `deadline_at` giờ server | FR-CHL-002 · FR-CHL-003 · ADR-0016 | TC-CHL-002 |
| F103-AC-05 | KHI thành viên gửi `boss.answer.submit`, module game PHẢI lấy người trả lời từ phiên (không từ payload), đóng dấu thời điểm server, chuyển module learning chấm và ghi vào learning record của chính người đó, cập nhật đóng góp và năng lượng boss, đẩy `boss.encounter.updated`; gửi lặp cùng câu PHẢI chỉ tính một lần | FR-CHL-003 · FR-PTY-013 · AC-CHL-003 | TC-CHL-003 |
| F103-AC-06 | NẾU câu trả lời thuộc phase đã đóng, THÌ module game PHẢI trả `BOSS_PHASE_CLOSED`; NẾU người gửi không là thành viên encounter, THÌ PHẢI trả `ROOM_NOT_MEMBER` và encounter không bị ảnh hưởng | FR-CHL-002 · [CHALLENGE_BOSS_SYSTEM §9](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | TC-PTY-011 |
| F103-AC-07 | KHI phase đạt `completion_rule` (`all`, `any_n`, hoặc `collective_count` với mỗi người ≥ `min_contribution`), module game PHẢI giảm năng lượng theo `energy_share`, phát `boss.phase_completed`, chuyển phase kế hoặc `defeated` khi hết phase | FR-CHL-002 · [CHALLENGE_BOSS_SYSTEM §3.2, §3.4](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | TC-CHL-004 |
| F103-AC-08 | KHI hết giờ phase với `on_fail = retry_phase`, module game PHẢI bắt đầu lại phase với câu hỏi mới và giữ tiến độ các phase trước; NẾU vượt `max_phase_retries`, hết thời gian tổng hoặc `on_fail = end_encounter`, THÌ PHẢI kết thúc encounter kèm tóm tắt số phase đã vượt | FR-CHL-002 · AC-CHL-005 | TC-CHL-005 |
| F103-AC-09 | HỆ THỐNG PHẢI không gửi tới thành viên khác đúng/sai, số câu đúng hay xếp hạng đóng góp của từng người (chỉ `contribution_met` và tiến độ chung); kết quả từng câu chỉ gửi cho người trả lời; trả lời sai không làm mất tiến độ đã đạt | FR-CHL-002 · FR-PTY-013 · AGENTS R19 · [CHALLENGE_BOSS_SYSTEM §3.5](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | TC-CHL-005 |
| F103-AC-10 | KHI encounter kết thúc, module game PHẢI trong một transaction chốt kết quả, xác định `eligible` (đạt `min_contribution`) từng người, trao `defeat_rewards`/`first_defeat_rewards` qua Reward Engine idempotent theo `(encounter_id, student_id)`, phát `boss.encounter_completed` (kèm `party_activity_id`) và đẩy `boss.encounter.ended`; xử lý lặp PHẢI không cấp thưởng lần hai | FR-CHL-002 · AC-CHL-004 · AC-CHL-006 | TC-CHL-004, TC-CHL-006 |
| F103-AC-11 | TRONG KHI encounter chạy, module game PHẢI giữ chỗ thành viên mất kết nối trong `reconnect_grace_seconds` của boss version, tiếp tục khi số người online ≥ `party_size.min`, trả lại trạng thái đầy đủ (`getBossEncounter`, snapshot) khi quay lại kể cả từ thiết bị khác; hết grace PHẢI đánh dấu rời và giữ đóng góp; dưới `party_size.min` quá grace PHẢI kết thúc `abandoned` | FR-CHL-002 · AC-CHL-007 · ADR-0031 | TC-CHL-007 |
| F103-AC-12 | KHI học sinh gọi `leaveBossEncounter` hoặc nhận `party.member_left`/`party.member_kicked`/`party.disbanded`, module game PHẢI cho người đó rời encounter, giữ đóng góp và learning record đã ghi; encounter không có ai vào sau `boss_reconnect_grace` kể từ khi tạo PHẢI đóng `abandoned`, không thưởng | FR-CHL-002 · FR-PTY-013 | — |

---

### User Story 2 — Quest co-op với tiến độ chung (Priority: P1)

Party nhận quest co-op; bước khung (đi tới, nói chuyện, thu thập) do một người làm tính cho cả nhóm; bước học tập mỗi người tự làm; bước đếm chung cộng đóng góp của mọi người (US-PTY-003).

**Why this priority**: AC-PTY-006 và vòng lặp co-op của M4a.

**Independent Test**: Quest `shared_steps`: A hoàn thành `go_to` → B cũng được tính; bước `activity` của B chỉ xong khi B tự đạt `pass_rule`; quest `collective_count` 20 câu đúng, `min_contribution` 5 → hoàn thành khi tổng ≥ 20 và mỗi người ≥ 5.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F103-AC-13 | KHI module competition gọi `startPartyActivity` loại `quest`, module game PHẢI kiểm `party_mode ≠ solo`, `party_size`, `grade_levels`, tiên quyết của từng thành viên, tạo hoặc gắn `QuestProgress` cá nhân của mỗi thành viên cùng `quest_version_id` ghim và `party_id`, tạo phiên quest chung và trả `external_ref`, `room_type = private_instance`, `area_id` | FR-QST-002 · FR-QST-007 · [QUEST_SYSTEM §6](../../docs/01-game-design/QUEST_SYSTEM.md) | TC-PTY-006 |
| F103-AC-14 | TRONG KHI `shared_progress_policy = shared_steps` hoặc `collective_count`, KHI một thành viên hoàn thành bước khung (`talk_npc`/`go_to`/`collect`/`discover`, `assignee = any_member`) trong instance của party, module game PHẢI đánh dấu bước hoàn thành cho mọi thành viên của phiên; bước `activity`/`minigame` PHẢI chỉ hoàn thành với từng người khi chính người đó đạt `pass_rule`; với `individual` mỗi người tự làm mọi bước | FR-QST-007 · AC-PTY-006 | TC-PTY-007 |
| F103-AC-15 | TRONG KHI bước có `collective_target`, module game PHẢI cộng `metric` từ attempt riêng của các thành viên vào tiến độ chung, hoàn thành bước khi đạt `count`, chỉ tính bước hoàn thành/thưởng cho thành viên đóng góp ≥ `min_contribution`, và đẩy `quest.progress.updated` có tiến độ chung | FR-QST-007 · [QUEST_SYSTEM §6](../../docs/01-game-design/QUEST_SYSTEM.md) | TC-PTY-007 |
| F103-AC-16 | HỆ THỐNG PHẢI không sao chép attempt, điểm, mastery giữa các thành viên; mọi bước học tập tạo attempt riêng gắn `student_id` của người làm; tiến độ chung chỉ dùng cho quest và phần thưởng | FR-PTY-013 · FR-QST-007 · AC-PTY-009 | TC-PTY-010 |
| F103-AC-17 | KHI thành viên thỏa `completion_rule`, module game PHẢI phát `quest.completed` cho người đó (thưởng qua Reward Engine idempotent); KHI phiên quest chung kết thúc (mọi thành viên còn lại hoàn thành, thất bại, hoặc party giải tán), PHẢI phát `quest.party_completed` đúng một lần kèm `party_activity_id` và `outcome` | FR-QST-007 · FR-QST-002 | TC-PTY-007 |
| F103-AC-18 | KHI nhận `boss.encounter_completed` với kết quả `defeated`, module game PHẢI hoàn thành bước `boss` của quest liên quan chỉ cho thành viên `eligible` | FR-QST-002 · [QUEST_SYSTEM §3](../../docs/01-game-design/QUEST_SYSTEM.md) | TC-CHL-004 |
| F103-AC-19 | KHI thành viên rời party hoặc mất kết nối quá `coop_quest_reconnect_grace`, module game PHẢI giữ `QuestProgress` cá nhân và các bước chung đã hoàn thành; phiên tiếp tục nếu `party_mode ≠ party_required` hoặc còn ≥ `party_size.min`; KHI nhận `party.disbanded`, PHẢI áp `failure_conditions` `party_disbanded` (`reset_step` mặc định hoặc `fail_quest`) và đóng phiên | FR-QST-007 · [QUEST_SYSTEM §2.5, §6](../../docs/01-game-design/QUEST_SYSTEM.md) | TC-PTY-008 |

---

### User Story 3 — Bước `interdependent`: mỗi người giữ một phần dữ kiện (Priority: P2)

Mỗi thành viên giữ một vai (ví dụ "Người đo", "Người đọc bản đồ") với thẻ dữ kiện riêng; chỉ ghép thẻ của cả nhóm mới giải được câu cuối; mỗi người tự trả lời câu cuối.

**Why this priority**: Biến co-op thành hợp tác thật; cần nền US2 trước.

**Independent Test**: Bước 2 role, party 2 người: A chỉ thấy thẻ role của A; A chia sẻ → B thấy; mỗi người làm câu cuối tạo 2 attempt riêng; B ngắt kết nối quá grace → A giữ cả hai role.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F103-AC-20 | KHI bước `interdependent` chuyển `active`, module game PHẢI gán role theo `role_assignment` (`auto`: chia đều theo thứ tự vào phòng; `leader_assigns`: leader gọi `assignQuestStepRoles`), mỗi thành viên online có ít nhất một role khi `allow_multi_role`, và đẩy `quest.roles.assigned`; người không phải leader gán role PHẢI nhận `403 PARTY_NOT_LEADER` | FR-QST-009 · [QUEST_SYSTEM §3.2](../../docs/01-game-design/QUEST_SYSTEM.md) | — |
| F103-AC-21 | KHI thành viên gọi `getQuestStepClues`, module game PHẢI chỉ trả dữ kiện đã render của role người đó giữ và thẻ đã được chia sẻ; dữ kiện PHẢI không chứa đáp án, và response/message gửi thành viên khác PHẢI không chứa thẻ chưa chia sẻ | FR-QST-009 · ADR-0016 · [QUEST_SYSTEM §11](../../docs/01-game-design/QUEST_SYSTEM.md) (AC-QST-014) | — |
| F103-AC-22 | KHI người giữ role gọi `shareQuestStepClue` (`Idempotency-Key`), module game PHẢI đưa thẻ lên bảng chung và đẩy `quest.clue.shared` tới mọi thành viên; người không giữ role đó PHẢI nhận `403 PERMISSION_DENIED`; gọi lặp PHẢI không tạo thẻ trùng | FR-QST-009 · [QUEST_SYSTEM §3.2](../../docs/01-game-design/QUEST_SYSTEM.md) | — |
| F103-AC-23 | KHI thành viên bắt đầu câu hỏi cuối của bước `interdependent`, module game PHẢI tạo attempt riêng cho người đó; bước hoàn thành theo `completion` (`all_members` hoặc `min_members_count`), và thành viên không đạt `pass_rule` PHẢI không được tính hoàn thành/thưởng bước | FR-QST-009 · FR-PTY-013 | TC-PTY-010 |
| F103-AC-24 | KHI người giữ role mất kết nối quá `coop_quest_reconnect_grace` và `allow_multi_role = true`, module game PHẢI chuyển role cho thành viên còn lại, giữ các thẻ đã chia sẻ và cho bước tiếp tục hoàn thành được; quest `party_optional` chơi solo PHẢI giao mọi role cho người chơi | FR-QST-009 · [QUEST_SYSTEM §11](../../docs/01-game-design/QUEST_SYSTEM.md) (AC-QST-015) | TC-PTY-008 |

---

### User Story 4 — Phân quyền, cô lập tenant, giới hạn đầu vào (Priority: P1)

**Why this priority**: Điều III, IV, V của constitution.

**Independent Test**: Sinh test từ OpenAPI: route người chơi × token nhân sự → `403`; module `chat` import facade party-activity → `make arch-check` fail; `hs.dung.7a` đọc encounter của `hs.an.3a` → `404`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F103-AC-25 | HỆ THỐNG PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY` cho `listBosses`, `createBossEncounter`, `getBossEncounter`, `leaveBossEncounter`, `getQuestStepClues`, `shareQuestStepClue`, `assignQuestStepRoles` khi token không phải token game của học sinh | AGENTS R23 · ADR-0031 | TC-AUTH-014 |
| F103-AC-26 | NẾU module khác `competition` (ngoài `x-allowed-callers`) import operation facade party-activity của `src/backend/modules/game/api`, THÌ `make arch-check` PHẢI fail; mọi thành viên trong request PHẢI thuộc tenant của `TenantScope` trong context, nếu không module game PHẢI trả `422 VALIDATION_FAILED` | AGENTS R29 · ADR-0074 | TC-SVC-005 |
| F103-AC-27 | NẾU học sinh đọc/ghi encounter, phiên quest chung hoặc thẻ dữ kiện không thuộc mình hoặc thuộc tenant khác (REST hay lệnh WS `boss.*`), THÌ module game PHẢI trả `404` / `PERMISSION_DENIED` không tiết lộ tồn tại; mọi truy vấn PHẢI lọc tenant + `deleted_at IS NULL` | AGENTS R8 · ADR-0065 | TC-TEN-001, TC-TEN-012 |
| F103-AC-28 | NẾU thiếu `Idempotency-Key` ở `createBossEncounter`, `leaveBossEncounter`, `shareQuestStepClue`, `assignQuestStepRoles`, `startPartyActivity`, THÌ module game PHẢI trả `428 PRECONDITION_REQUIRED`; chuỗi vượt `maxLength` (role, `phase_id`, `requirement_id`) PHẢI trả `422 VALIDATION_FAILED` | AGENTS R10 · ADR-0041 | TC-TEN-024 |

### Edge Cases

- NẾU module learning lỗi khi tạo bộ câu hỏi đầu phase, THÌ thử lại (idempotent); quá 2 lần → encounter `abandoned`, không thưởng, không tính lượt thử của học sinh (như REALTIME §7.1 bước 3).
- NẾU owner node của encounter chết, THÌ khôi phục từ checkpoint; không có checkpoint hợp lệ → `abandoned`, thông báo, không tính lượt ([MULTIPLAYER_GAMEPLAY §4](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md)).
- NẾU thành viên có khối không có `per_grade` riêng, THÌ dùng `activity_ref` chung của requirement (đã qua kiểm `grade_levels`).
- NẾU publish boss/quest version mới khi encounter/phiên đang chạy, THÌ giữ version đã ghim (ADR-0043).
- NẾU câu trả lời quá nhanh (ngưỡng ANTI_CHEAT), THÌ không tính đóng góp, gắn `integrity_signals`.
- NẾU học sinh trả lời qua API attempt public cho câu hỏi boss, THÌ không áp dụng (câu hỏi boss chỉ trả lời qua `boss.answer.submit`).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHL-002 | Boss nhiều phase, đóng góp cá nhân/nhóm, retry không phạt, server-authoritative trong module game | [CHALLENGE_BOSS_SYSTEM §3, §4](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| FR-CHL-003 | Mỗi câu boss là learning record cá nhân | [CHALLENGE_BOSS_SYSTEM §2.3, §3.4](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| FR-QST-002 | Loại quest `party` và `boss` (bước `boss`, quest co-op); 10 loại còn lại → F072 | [QUEST_SYSTEM §2.3](../../docs/01-game-design/QUEST_SYSTEM.md) |
| FR-QST-007 | `shared_progress_policy`: `individual`, `shared_steps`, `collective_count` | [QUEST_SYSTEM §6](../../docs/01-game-design/QUEST_SYSTEM.md) |
| FR-QST-009 | Bước `interdependent` có role, thẻ dữ kiện, chia sẻ, câu cuối cá nhân | [QUEST_SYSTEM §3.2](../../docs/01-game-design/QUEST_SYSTEM.md) |
| FR-PTY-013 | Không chia sẻ kết quả học tập trong boss/quest co-op | [PARTY_SYSTEM §7.1](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |

### Key Entities

- **BossEncounter**: một trận boss (solo/party) ghim version, phase hiện tại, năng lượng, số lần thử lại phase, hạn, kết quả.
- **BossEncounterMember / Contribution**: thành viên và đóng góp mỗi phase (số câu đúng, hoạt động hoàn thành, đạt `min_contribution`).
- **PartyQuestSession**: tiến độ chung của party cho một quest version (bước chung, tổng `collective_target`, phân vai, thẻ đã chia sẻ).
- **QuestProgress**: tiến độ cá nhân (F072) có `party_id` khi chơi co-op.
- **RoleAssignment / ClueCard**: vai của thành viên trong bước `interdependent` và thẻ dữ kiện đã chia sẻ.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `boss_reconnect_grace` | Mặc định khi boss version không đặt `reconnect_grace_seconds`; hạn chờ Ready |
| `coop_quest_reconnect_grace` | Grace thành viên quest co-op, chuyển role |
| `interdependent_roles` | Số role hợp lệ mỗi bước |
| `quest_pass_rule` | `pass_rule` mặc định của bước học tập |
| `challenge_attempts_per_day` | Giới hạn lượt boss (lý do `attempt_limit`) |
| `max_active_quests` | Giới hạn quest active khi gắn quest co-op |
| `idempotency_record_ttl` | Kết quả theo `Idempotency-Key` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chấm điểm chỉ ở module learning; câu hỏi/dữ kiện không chứa đáp án | AGENTS R5, R6 · ADR-0016 |
| Encounter/phiên quest ghim version | AGENTS R31 · ADR-0043 |
| Không xếp hạng/làm xấu hổ trong nhóm; thưởng có cap | AGENTS R19 · NFR-SAFE-003 |
| Trạng thái live ở Valkey có checkpoint, kết quả cuối ở PostgreSQL | AGENTS R9 · ADR-0004 |
| Gọi module learning qua facade `learning`; kết thúc/thưởng qua event | AGENTS R29 · ADR-0042 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% câu trả lời boss/quest co-op tạo learning record gắn đúng học sinh trả lời; 0 bản ghi học tập của học sinh không tự trả lời.
- **SC-002**: Phần thưởng boss/quest cấp đúng một lần khi event bị giao lặp 3 lần (0 bản ghi thưởng trùng).
- **SC-003**: Năng lượng boss cập nhật tới mọi thành viên trong ≤ 1 giây sau câu trả lời đúng (p95, party 4 người).
- **SC-004**: 0 message gửi tới thành viên khác chứa đúng/sai từng người hoặc thẻ dữ kiện chưa chia sẻ trong bộ test.

## Ngoài phạm vi

- Party, lobby, Start Session, giữ chỗ phòng `private_instance` → F100, F101. Room host dùng chung (`src/backend/pkg/roomhost`), challenge solo, 5 mức độ khó → F076.
- Quest engine nền (10 loại quest còn lại, bước `talk_npc`/`go_to`/`activity`, `pass_rule`) → F072; Reward Engine/cap XP → F074; vật phẩm thưởng → F075.
- Room `coop` puzzle (`party_activity`, `coop.*`) của module competition và `coop_challenge` → F105.
- Giao diện boss S06, HUD tiến độ quest chung, thẻ dữ kiện trong Godot: chưa có feature trong danh mục (đề xuất bổ sung). Party UI → F102.
- Soạn boss/quest (`/game-admin/*`) → F078, F079. Chat `party` khi `share_mode = clue_card_and_chat` → F117.

## Assumptions

- F076 cung cấp `src/backend/pkg/roomhost` (owner node, lease, checkpoint, timer) và aggregate challenge; F072 cung cấp `QuestProgress`, consumer `activity.graded`; F074 cung cấp Reward Engine idempotent trong module game.
- module learning thêm `game` vào `x-allowed-callers` của `createQuestionPlan`/`scorePlanAnswer` và thêm API render dữ kiện (delta contract, nhóm D sở hữu) [Confirmed].
- Mã lý do không đủ điều kiện (`reason_code`) là enum mới trong spec internal (facade) [Confirmed].

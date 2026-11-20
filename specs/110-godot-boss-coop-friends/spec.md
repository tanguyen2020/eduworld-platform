# Feature Specification: F110 — Godot — màn boss co-op, thẻ dữ kiện bước `interdependent`, danh sách bạn bè

**Feature Branch**: `110-godot-boss-coop-friends` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/game/` · **Mốc**: M4a · **Phụ thuộc**: F095, F102, F103 (và F063 renderer câu hỏi, F072 quest tracker, F092 `RealtimeClient`, F069 `LevelUpQueue`)
**Input**: Học sinh đánh boss nhiều phase một mình hoặc cùng party (HUD năng lượng, phase, đồng hồ server, câu hỏi riêng theo khối, đóng góp không xếp hạng); thấy tiến độ quest co-op chung; nhận vai và thẻ dữ kiện ở bước `interdependent`, chia sẻ thẻ, tự trả lời câu cuối; quản lý bạn bè (danh sách, trạng thái online, lời mời, mã bạn bè, mời party, chặn, tắt tiếng). Màn S06 (phần boss), S11 (phần bạn bè) của [STUDENT_GAME_UX](../../docs/05-ux/STUDENT_GAME_UX.md). FR nguồn: FR-CHL-002, FR-CHL-003, FR-QST-002, FR-QST-007, FR-QST-009, FR-PTY-013, FR-SOC-001…003, FR-MP-003.

## Clarifications

### Session 2026-11-16

- Q: Boss solo thuộc F069 hay F110? → A: F110 làm màn boss cho cả solo (`createBossEncounter` `mode = solo`) và party (vào từ Party Lobby F102 qua `party.session.started` `room_type = boss`); cùng một scene `BossArena`. F069 chỉ làm challenge solo của F076.
- Q: Wireframe S06 hiện số câu đúng từng bạn ("Cú(✓4) Mèo(✓3)") nhưng F103-AC-09 và AsyncAPI chỉ có `contribution_met`? → A: Theo [CHALLENGE_BOSS_SYSTEM §3.5](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) và contract: với bạn chỉ hiện ✓ "đã đạt đóng góp" hoặc biểu tượng đồng hồ; số câu đúng chỉ hiện của chính mình. Sửa wireframe S06 ở task Polish.
- Q: Câu hỏi boss render bằng gì? → A: `QuestionRendererRegistry` của F063 trong `QuestionPanelHost` (Control 2D), trả lời bằng lệnh WS `boss.answer.submit` (không qua API attempt public).
- Q: `SocialService` do F110 hay F119 tạo? → A: F110 (M4a, trước F119) tạo `scripts/social/social_service.gd` đủ API bạn bè/chặn/tắt tiếng; F119 dùng lại cho menu tin nhắn. Nút "Báo cáo" mở `ReportDialog` của F119, ẩn khi F119 chưa có.
- Q: Tab "Tìm bạn" có "Bạn cùng lớp", "Vừa chơi cùng", club, "Tham gia bạn", "Thách đấu", "Nhắn tin"? → A: Không ở F110 (chưa có API hoặc thuộc feature khác — xem Ngoài phạm vi). Tab "Tìm bạn" gồm nhóm "Ở gần đây" (người chơi cùng hub instance do F092 cung cấp) và Mã bạn bè; không có ô tìm kiếm tự do.

### Session 2026-11-17

- Q: Enum presence và ẩn trạng thái sau đối soát (RB-404)? → A: Enum chung theo [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md); "Ẩn trạng thái" là cờ `hidden` của `presence.status.set` (delta F091), không phải giá trị `invisible`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Đánh boss nhiều phase, solo và party (Priority: P1)

Học sinh vào arena boss, cùng bạn trả lời câu hỏi theo khối của mình, thấy thanh năng lượng boss giảm, phase chuyển, hết giờ thì thử lại phase; hạ boss thì người đủ đóng góp nhận thưởng.

**Why this priority**: Boss là hoạt động party chính của M4a và đỉnh mỗi chương truyện.

**Independent Test**: Hai instance Godot (`hs.an.3a`, `hs.binh.3a`) bắt đầu boss mẫu 2 phase từ lobby: mỗi người thấy câu hỏi riêng; trả lời đúng → năng lượng giảm trên cả hai màn ≤ 1 s; màn B không thấy A đúng hay sai; ngắt mạng A 30 s → A về đúng phase.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F110-AC-01 | KHI học sinh chạm cổng/NPC boss, game client PHẢI hiện `BossInfoCard` từ `listBosses` (tên, số phase, chế độ); boss cho phép `solo` PHẢI có nút "Chơi một mình" gọi `createBossEncounter` (`mode = solo`, `Idempotency-Key` giữ khi thử lại) rồi tải `BossArena`; boss không có `solo` PHẢI chỉ có nút "Rủ bạn" mở Party (F102); `403 CHALLENGE_NOT_ELIGIBLE`/`409 CHALLENGE_ATTEMPT_LIMIT` PHẢI hiện thông điệp thân thiện | FR-CHL-002 · [STUDENT_GAME_UX_CORE S06](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) | TC-CHL-002 |
| F110-AC-02 | KHI nhận `party.session.started` có `room_type = boss`, game client PHẢI tải `BossArena` qua `SceneRouter` với `encounter_id`, gửi `boss.encounter.ready` (`ready = true`) sau khi cảnh tải xong và hiện `PartyReadyList` (✓ hoặc đồng hồ cho người chưa sẵn sàng) | FR-CHL-002 · FR-QST-002 · [PARTY_SYSTEM §6](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-006 |
| F110-AC-03 | KHI nhận `boss.encounter.updated`, `BossStore` PHẢI áp theo `state_version` (bỏ bản cũ hơn; lệch → `getBossEncounter`) và hiển thị `BossEnergyBar` có số %, `PhaseIndicator`, `PhaseTimer` tính từ `phase_deadline_at`/`deadline_at` bằng `Clock` (lệch ≤ 1 s; `g1_3` dạng thanh màu không số giây) | FR-CHL-002 · AC-UX-0602 | TC-CHL-005 |
| F110-AC-04 | KHI nhận `boss.question.assigned`, game client PHẢI hiển thị câu hỏi trong `QuestionPanelHost` bằng renderer F063 (payload không đáp án) và gửi `boss.answer.submit` (`encounter_id`, `phase_id`, `requirement_id`, `question_instance_id`, `response`, `client_elapsed_ms`); TRONG KHI chờ `system.command.acked`/`failed`, PHẢI khóa nộp lại câu đó; gửi lại sau mất kết nối PHẢI giữ nguyên `question_instance_id` | FR-CHL-003 · ADR-0016 · AC-CHL-003 | TC-CHL-003 |
| F110-AC-05 | KHI nhận `boss.answer.evaluated` của chính mình, game client PHẢI hiện hiệu ứng tia sáng nếu đúng hoặc boss "phản đòn" vui kèm giải thích nếu sai, không chữ "Sai" đỏ; năng lượng boss PHẢI chỉ đổi theo `boss.encounter.updated`/`boss.phase.completed` (không bao giờ tăng ở client); NẾU nhận `BOSS_PHASE_CLOSED`, THÌ PHẢI đóng câu đó và chờ phase kế | FR-CHL-002 · AC-CHL-005 · AC-UX-0603 | TC-CHL-005 |
| F110-AC-06 | `ContributionList` PHẢI hiện với mỗi bạn chỉ trạng thái `contribution_met` (✓ hoặc đồng hồ) và yêu cầu `min_contribution` bằng lời; số câu đúng PHẢI chỉ hiện của chính mình; PHẢI không có xếp hạng, không cho biết ai trả lời sai | FR-PTY-013 · AGENTS R19 · [CHALLENGE_BOSS_SYSTEM §3.5](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | TC-CHL-005 |
| F110-AC-07 | KHI nhận `boss.phase.started` có `retry = true`, game client PHẢI hiện băng "Thử lại phase với câu hỏi mới" và giữ tiến độ phase trước; KHI nhận `boss.phase.completed`, PHẢI phát hoạt ảnh chuyển phase ≤ 2 s và câu thoại theo `presentation_ref` từ gói nội dung đã ghim (song ngữ) | FR-CHL-002 · AC-CHL-005 | TC-CHL-005 |
| F110-AC-08 | KHI nhận `boss.encounter.ended`, game client PHẢI mở `BossResult`: `defeated` → "Hạ {boss}!" và phần thưởng của chính mình từ `rewards`; `time_up`/`abandoned` → "Em đã tới phase {n}/{total}" kèm gợi ý ôn, không chữ "Thất bại"; party PHẢI quay về lobby khi nhận `party.session.ended` (F102) | FR-CHL-002 · AC-CHL-004 | TC-CHL-004 |
| F110-AC-09 | TRONG KHI mất kết nối, game client PHẢI hiện "Đang kết nối lại…" và khóa nộp câu; KHI kết nối lại (kể cả takeover sang thiết bị khác), PHẢI gọi `getBossEncounter`, gửi lại `boss.encounter.ready` và về đúng phase với câu hỏi đang mở của mình; thành viên `connected = false` PHẢI hiện mờ kèm nhãn chữ | FR-CHL-002 · AC-CHL-007 · AC-UX-0604 · ADR-0031 | TC-CHL-007 |
| F110-AC-10 | KHI học sinh bấm "Rời" và xác nhận, game client PHẢI gọi `leaveBossEncounter` (`Idempotency-Key`) rồi về vùng (solo) hoặc lobby (party); NẾU nhận `ROOM_NOT_MEMBER` hoặc `404`, THÌ PHẢI rời cảnh về Hub với thông điệp chung | FR-CHL-002 · [CHALLENGE_BOSS_SYSTEM §9](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | — |
| F110-AC-11 | `BossArena` party 4 người PHẢI đạt ≥ 30 FPS trên điện thoại tầm trung preset Trung bình: ≤ 8 nhân vật, particle ≤ 200 hạt (tắt ở preset Thấp), camera cố định và ẩn joystick trên mobile, panel câu hỏi là Control 2D | NFR-COMP-001 · AC-UX-0605 · AC-PERF-002 | TC-PERF-018 |

---

### User Story 2 — Tiến độ quest co-op chung (Priority: P2)

Party làm quest co-op: bước khung một người làm tính cho cả nhóm, bước học tập mỗi người tự làm; HUD cho thấy tiến độ chung và mình đã đóng góp đủ chưa (US-PTY-003).

**Why this priority**: AC-PTY-006 của M4a; cần quest tracker F072 và phiên chung F103.

**Independent Test**: Quest `collective_count` 20 câu, `min_contribution` 5: A làm 12 câu, B làm 3 → thanh chung 75%, màn B hiện "Em cần thêm 2 câu", màn A không thấy số câu của B.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F110-AC-12 | KHI nhận `quest.progress.updated` có `party_progress`, `QuestTracker` (F072) PHẢI hiện `PartyProgressStrip` (thanh `collective_progress` + ✓ `my_contribution_met`) và đánh dấu bước khung hoàn thành cho cả nhóm theo server; PHẢI không hiển thị số liệu học tập của bạn khác | FR-QST-007 · FR-PTY-013 · AC-PTY-006 | TC-PTY-007 |
| F110-AC-13 | KHI học sinh làm bước học tập trong quest co-op, game client PHẢI gọi `startQuestActivity` cho chính mình và mở `ActivityHost` (F063); kết quả chỉ hiển thị cho người làm | FR-QST-007 · FR-PTY-013 · AC-PTY-009 | TC-PTY-010 |

---

### User Story 3 — Bước `interdependent`: vai, thẻ dữ kiện, chia sẻ (Priority: P2)

Mỗi bạn giữ một vai ("Người đo", "Người đọc bản đồ") và một thẻ dữ kiện chỉ mình thấy; bấm "Chia sẻ dữ kiện" để đưa thẻ lên bảng chung; mỗi người tự trả lời câu cuối.

**Why this priority**: Biến co-op thành hợp tác thật (FR-QST-009); cần US2.

**Independent Test**: Bước 2 vai, party 2 người: A chỉ thấy thẻ của A; A chia sẻ → thẻ hiện ở B ≤ 1 s; B ngắt mạng quá grace → A nhận thêm vai B và toast; mỗi người mở câu cuối riêng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F110-AC-14 | KHI nhận `quest.roles.assigned`, `InterdependentStepPanel` PHẢI hiện vai của mình (tên `LocalizedName`, `icon_key`) và vai của bạn (nickname + tên vai, không có thẻ); với `role_assignment = leader_assigns`, chỉ leader PHẢI thấy `RoleAssignDialog` gọi `assignQuestStepRoles` (`Idempotency-Key`); `403 PARTY_NOT_LEADER` PHẢI hiện thông điệp thân thiện | FR-QST-009 · [QUEST_SYSTEM §3.2](../../docs/01-game-design/QUEST_SYSTEM.md) | — |
| F110-AC-15 | KHI mở bước, game client PHẢI gọi `getQuestStepClues` và chỉ hiển thị các `ClueCard` server trả (của vai mình + đã chia sẻ); client PHẢI không tự ghép hay suy dữ kiện, không lưu thẻ xuống đĩa | FR-QST-009 · ADR-0016 · AC-QST-014 | — |
| F110-AC-16 | KHI người giữ vai bấm "Chia sẻ dữ kiện", game client PHẢI gọi `shareQuestStepClue` (`Idempotency-Key` giữ khi thử lại); KHI nhận `quest.clue.shared`, thẻ PHẢI xuất hiện trên `SharedClueBoard` của mọi thành viên ≤ 1 giây; với `share_mode = clue_card_and_chat` PHẢI hiện nút mở `ChatDock` kênh `party` (F119), với `clue_card` không cần gõ chữ | FR-QST-009 · ADR-0020 | — |
| F110-AC-17 | KHI học sinh bấm "Trả lời câu cuối", game client PHẢI gọi `startQuestActivity` cho chính mình và mở `ActivityHost`; PHẢI không có đường nộp thay bạn hay xem đáp án của bạn | FR-QST-009 · FR-PTY-013 · AC-QST-014 | TC-PTY-010 |
| F110-AC-18 | KHI nhận `quest.roles.assigned` mới do bạn mất kết nối quá grace, game client PHẢI cập nhật panel, hiện toast "Em giữ thêm vai {role}" và giữ các thẻ đã chia sẻ; quest `party_optional` chơi một mình PHẢI hiện mọi vai và thẻ của mình | FR-QST-009 · AC-QST-015 | TC-PTY-008 |

---

### User Story 4 — Bạn bè: danh sách, lời mời, mã bạn bè, chặn, tắt tiếng (Priority: P1)

Học sinh mở Bạn bè (phím `F`), thấy ai đang online, gửi/nhận lời mời trong trường, đọc mã cho bạn ngồi cạnh, mời bạn vào party, chặn hoặc tắt tiếng người làm phiền (US-SOC-001).

**Why this priority**: Đường vào mời party (F102) và an toàn trẻ em (FR-SOC-003).

**Independent Test**: `hs.an.3a` tạo mã → `hs.binh.3a` nhập → lời mời → chấp nhận → cả hai thấy nhau online ≤ 2 s; `hs.an.3a` chặn `hs.binh.3a` → biến khỏi danh sách; lời mời bị từ chối vẫn "Đang chờ" ở người gửi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F110-AC-19 | KHI học sinh mở `FriendsScreen` (phím `F`, NPC Chú Ong Kết Nối, HUD), game client PHẢI hiển thị `listFriends` (cursor, ảo hóa) nhóm Online/Offline với nickname, avatar 2D, level, trạng thái bằng biểu tượng + chữ; PHẢI không hiển thị tên thật, trường, lớp, điểm học tập | FR-SOC-001 · AGENTS R16 · AC-SOC-010 | — |
| F110-AC-20 | KHI nhận `presence.friend.updated`, game client PHẢI cập nhật trạng thái bạn, gộp tối đa 1 lần/giây cho danh sách; bạn không có dữ liệu PHẢI hiện offline; TRONG KHI mất mạng, trạng thái PHẢI hiện "—" | FR-MP-003 · [STUDENT_GAME_UX_SOCIAL §6](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | — |
| F110-AC-21 | KHI mở tab Lời mời, game client PHẢI hiển thị `listFriendRequests` (`incoming`, `outgoing`); chấp nhận gọi `acceptFriendRequest` (`Idempotency-Key`), từ chối gọi `rejectFriendRequest`; lời mời đi PHẢI luôn hiện "Đang chờ" (không bao giờ "bị từ chối") tới khi biến mất | FR-SOC-001 · AC-SOC-003 · AC-UX-105 | — |
| F110-AC-22 | KHI học sinh bấm "Kết bạn" trên thẻ người chơi trong Hub hoặc nhóm "Ở gần đây", game client PHẢI gọi `sendFriendRequest` (`addressee_id`, `Idempotency-Key`); `403 SOCIAL_SCOPE_VIOLATION` PHẢI hiện "Em chỉ kết bạn được với bạn cùng trường", `409 FRIEND_LIMIT_REACHED`/`429 RATE_LIMITED` (theo `Retry-After`) PHẢI hiện thông điệp thân thiện; PHẢI không có ô tìm người theo tên/ID | FR-SOC-001 · FR-SOC-002 · AC-SOC-002 | TC-SOC-002 |
| F110-AC-23 | KHI học sinh tạo mã, game client PHẢI gọi `createFriendCode` và hiện mã cỡ lớn, đếm ngược theo `expires_at`, nút đọc to từng ký tự; ô "Nhập mã" PHẢI tự viết hoa, chỉ nhận `A-Z0-9`, tối đa 12 ký tự, bật nút gửi khi khớp `^[A-Z0-9]{6,12}$`, rồi gọi `redeemFriendCode`; `404 FRIEND_CODE_INVALID` PHẢI hiện "Mã không đúng hoặc đã hết hạn" | FR-SOC-001 · ADR-0041 | TC-SOC-001 |
| F110-AC-24 | Trên hồ sơ bạn, game client PHẢI có "Mời party" (qua `PartyService` F102: `inviteToParty`, chưa có party thì `createParty` trước), "Hủy kết bạn" (xác nhận → `removeFriend`), "Tắt tiếng"/"Chặn" (xác nhận một bước có lý do bằng biểu tượng → `muteUser`/`blockUser`), "Báo cáo" (mở `ReportDialog` F119); mọi thao tác an toàn ≤ 2 chạm; sau khi chặn, người đó PHẢI biến khỏi danh sách | FR-SOC-003 · AC-SOC-004 · [STUDENT_GAME_UX §9](../../docs/05-ux/STUDENT_GAME_UX.md) | TC-CHAT-005, TC-SOC-003 |
| F110-AC-25 | KHI mở tab Đã chặn, game client PHẢI hiển thị `listBlocks` và `listMutes` với nút bỏ chặn (`unblockUser`)/bỏ tắt tiếng (`unmuteUser`); PHẢI không có màn hay thông điệp nào cho biết ai đã chặn mình | FR-SOC-003 · AGENTS R16 | — |
| F110-AC-26 | KHI học sinh bật "Ẩn trạng thái", game client PHẢI gửi `presence.status.set` (`hidden = true`) và hiển thị mình là "Đang ẩn"; tắt thì gửi `hidden = false` | FR-MP-003 · [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |

---

### User Story 5 — Song ngữ, trợ năng, token, an toàn, phân quyền (Priority: P2)

**Why this priority**: Bắt buộc theo constitution (V, VI, VIII); kiểm sau luồng chính.

**Independent Test**: Pseudo-locale `xx` cho `BossArena`, `InterdependentStepPanel`, `FriendsScreen`; `lint_theme_overrides.gd`; đăng nhập `ad.truong.a`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F110-AC-27 | Mọi chuỗi UI PHẢI qua khóa `ui.boss.*`, `ui.quest.coop.*`, `ui.quest.clue.*`, `ui.friends.*`, `ui.friendcode.*`, `ui.error.<CODE>`; dữ liệu `LocalizedName`/`LocalizedText` chọn theo ngôn ngữ; bố cục PHẢI chịu chuỗi dài hơn 35% (`ui.boss.need_contribution`, `ui.friendcode.instructions`) | NFR-I18N-001 · NFR-I18N-003 · ADR-0056 | TC-I18N-004 |
| F110-AC-28 | game client PHẢI lấy style từ Theme sinh từ `design/game-tokens` theo density (vùng chạm ≥ 48 dp, `g1_3` ≥ 64 dp); trạng thái online, sẵn sàng, đóng góp, đúng/sai PHẢI có biểu tượng + chữ, không chỉ màu | NFR-A11Y-001 · ADR-0047 · AGENTS R32 | TC-UI-001, TC-UI-003 |
| F110-AC-29 | game client PHẢI chỉ hiển thị nickname/avatar/level/danh hiệu của người khác, giữ bạn bè/thẻ dữ kiện/trạng thái boss chỉ trong RAM, xóa store khi đăng xuất hoặc `session.session.replaced`; script trong `scenes/**` PHẢI chỉ gọi method public của service | AGENTS R1 · AGENTS R16 · ADR-0064 | — |
| F110-AC-30 | Mọi request PHẢI qua `ApiClient` (gắn `X-Tenant-Id`); NẾU nhận `403 GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH`, THÌ PHẢI theo luồng F061; NẾU nhận `404` cho encounter, thẻ dữ kiện hay bạn, THÌ PHẢI đóng màn liên quan và làm mới, không hiển thị chi tiết; `422 VALIDATION_FAILED` PHẢI ghi telemetry `client_bug` không PII | AGENTS R23 · ADR-0065 · ADR-0041 | TC-AUTH-014 |

### Edge Cases

- NẾU `progression.level.reached` đến giữa phase, THÌ `LevelUpQueue` (F069) giữ tới khi `boss.encounter.ended` (`AppState.blocking_activity = &"boss"`).
- NẾU nhận lời mời party/kết bạn khi đang trong phase boss, THÌ chỉ tăng số trên chuông, không bật popup che câu hỏi.
- NẾU boss version mới publish giữa trận, THÌ client dùng `presentation_ref` của version đã ghim trong gói nội dung đã tải.
- NẾU bạn chuyển trường/khác tenant, THÌ biến khỏi danh sách ở lần tải sau (server lọc); client không giữ cache qua phiên.
- NẾU chặn một bạn đang cùng party, THÌ party xử lý theo F100 (client chỉ hiện kết quả server trả).
- NẾU nhận `quest.clue.shared` cho thẻ đã có, THÌ không tạo thẻ trùng (khóa theo `role`).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHL-002 | Màn boss nhiều phase: năng lượng, phase, thử lại, đóng góp không xếp hạng, kết quả | [CHALLENGE_BOSS_SYSTEM §3](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) · [STUDENT_GAME_UX_CORE S06](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) |
| FR-CHL-003 | Câu hỏi boss riêng từng người, kết quả chỉ cho người trả lời | CHALLENGE_BOSS_SYSTEM §2.3, §3.4 |
| FR-QST-002 | Vào quest/bước loại `boss` và `party` từ client | [QUEST_SYSTEM §2.3](../../docs/01-game-design/QUEST_SYSTEM.md) |
| FR-QST-007 | Hiển thị tiến độ quest chung theo `shared_progress_policy` | QUEST_SYSTEM §6 |
| FR-QST-009 | Vai, thẻ dữ kiện, chia sẻ, câu cuối cá nhân | QUEST_SYSTEM §3.2 |
| FR-PTY-013 | Không hiển thị kết quả học tập của bạn trong boss/quest co-op | [PARTY_SYSTEM §7.1](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-SOC-001 | Danh sách bạn, lời mời, mã bạn bè, hủy kết bạn | [SOCIAL_SYSTEM §4](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |
| FR-SOC-002 | Không tìm kiếm tự do; thông điệp ngoài phạm vi trường | SOCIAL_SYSTEM §7 · [ADR-0054](../../docs/adr/ADR-0054-tuong-tac-xa-hoi-lien-truong-cross-school-social-interaction.md) |
| FR-SOC-003 | Chặn, tắt tiếng, danh sách đã chặn | SOCIAL_SYSTEM §5 |
| FR-MP-003 (phần hiển thị) | Trạng thái online của bạn, ẩn trạng thái | [MULTIPLAYER_GAMEPLAY §5](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |

NFR: NFR-I18N-001, NFR-I18N-003, NFR-A11Y-001, NFR-COMP-001.

### Key Entities

- **BossEncounterModel**: trận boss (trạng thái, phase, năng lượng, hạn phase, thành viên + sẵn sàng/kết nối/đạt đóng góp, `state_version`).
- **BossQuestionModel**: câu hỏi được giao cho mình (phase, requirement, `question_instance_id`, render payload, hạn).
- **PartyQuestProgressModel**: tiến độ chung một bước (tỉ lệ, mình đã đạt đóng góp).
- **RoleAssignmentModel / ClueCardModel**: vai trong bước `interdependent`; thẻ dữ kiện đã render (của mình hoặc đã chia sẻ).
- **FriendModel / FriendRequestModel / FriendCodeModel / RelationModel**: bạn (thẻ công khai + trạng thái), lời mời, mã bạn bè, quan hệ chặn/tắt tiếng.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `boss_reconnect_grace`, `coop_quest_reconnect_grace` | Chỉ hiển thị băng chờ (thời điểm do server) |
| `interdependent_roles` | Bố cục panel 2–4 vai |
| `party_max_members` | Bố cục `ContributionList`/`PartyReadyList` |
| `friend_limits`, `friend_request_ttl`, `friend_code_ttl` | Chỉ hiển thị (đếm ngược dùng `expires_at` server) |
| `presence_ttl` | Trạng thái mặc định offline |
| `client_reconnect_backoff`, `supported_locales` | Kết nối lại WS; ngôn ngữ UI |
| `target_fps`, `quality_presets` | Ngân sách cảnh boss theo preset |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Năng lượng boss cập nhật ≤ 1 s (p95, party 4) | F103 SC-003 · AC-UX-0605 |
| Không business logic trong UI; không gửi kết quả/đóng góp từ client | AGENTS R1, R5 · ADR-0016 |
| Không xếp hạng/làm xấu hổ trong nhóm | AGENTS R19 · NFR-SAFE-003 |
| Chỉ nickname + avatar; không lộ ai chặn mình | AGENTS R16 · ADR-0054 |
| Theme/token; song ngữ | AGENTS R32 · ADR-0047 · ADR-0056 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Năng lượng boss và thẻ dữ kiện chia sẻ hiển thị trên màn mọi thành viên trong ≤ 1 giây (p95, party 4 người, mạng bình thường).
- **SC-002**: 0 màn trong bộ scene test hiển thị đúng/sai, số câu đúng của bạn khác hoặc thẻ dữ kiện chưa chia sẻ.
- **SC-003**: Học sinh lớp 1–3 kết bạn bằng mã bạn bè trong ≤ 4 thao tác mỗi bên.
- **SC-004**: Boss party 4 người ≥ 30 FPS trên thiết bị Android tầm trung tham chiếu, preset Trung bình.
- **SC-005**: Người gửi lời mời không phân biệt được lời mời bị từ chối với lời mời chưa trả lời trong mọi màn.

## Ngoài phạm vi

- Backend boss/quest co-op/party-activity → F103; bạn bè/chặn/tắt tiếng/presence → F095, F091; party, lobby, `InviteFriendPicker` → F100, F101, F102.
- Renderer câu hỏi, `ActivityHost` → F063; quest tracker/journal nền → F072; challenge solo, HUD tiến trình, `LevelUpQueue` → F069.
- Chat party, `ReportDialog`, quick message → F119; báo cáo/sanction backend → F121; thông báo lời mời khi offline → F123.
- Club/study group trong game → F120 (chưa có feature Godot); "Bạn cùng lớp", "Vừa chơi cùng", "Tham gia bạn" (endpoint đề xuất ở STUDENT_GAME_UX_SOCIAL §11 chưa có trong spec) — chưa có feature; "Thách đấu" → F109; tin nhắn trực tiếp → F118/F119.
- Room `coop` puzzle và `coop_challenge` của module competition → F105/F109.

## Assumptions

- Trước khi F095/F103 `Done`, client chạy trên Prism mock sinh từ `specs/` (profile `mock`, nhãn MOCK — R17); feature chỉ `Done` với backend thật.
- Message/operation "Cần thêm/Cần sửa" của F103 (`boss.question.assigned`, `boss.answer.evaluated`, `quest.roles.assigned`, `quest.clue.shared`, `party_progress`, `getQuestStepClues`, `shareQuestStepClue`, `assignQuestStepRoles`, `getBossEncounter` có `my_questions`) và F101 (`room_type: boss`) có trước US1/US3 ([contracts §6](contracts/README.md)).
- Kết nối lại: gửi lại `boss.encounter.ready` để gateway đăng ký lại room `ew:rt:room:<encounter_id>:out` [Confirmed — xác nhận với F103].
- `PlayerCard.presence` và `presence.friend.updated.status` dùng một enum chung theo MULTIPLAYER_GAMEPLAY §5 (`offline`, `online`, `in_activity`, `in_match`, `away`; RB-404, delta F091).

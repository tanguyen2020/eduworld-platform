# Feature Specification: F101 — Party Lobby, Start Session, leader/thành viên mất kết nối

**Feature Branch**: `101-party-lobby-session` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `competition` · **Mốc**: M4a · **Phụ thuộc**: F014, F100 (phần `quest`/`boss` gọi module game do F103 cung cấp; trước khi F103 Done dùng stub contract có nhãn MOCK)
**Input**: Party Lobby realtime (thành viên, online, Ready, leader chọn hoạt động, danh sách hoạt động đủ điều kiện), Start Session theo luồng 9 bước, kết thúc phiên, xử lý thành viên/leader mất kết nối (grace, tự chuyển leader, dormant), kết quả học tập luôn riêng từng người. FR nguồn: FR-PTY-006…008, FR-PTY-012, FR-PTY-013.

## Clarifications

### Session 2026-11-16

- Q: Ai kiểm tra điều kiện quest/boss và tạo encounter/phiên quest co-op? → A: module game (sở hữu quest/boss, ADR-0074) qua facade `game` do module competition gọi ở bước (3) và (4) của luồng 9 bước; `explore` do module competition cấp `private_instance` — [research.md](research.md) R1.
- Q: `PARTY_ACTIVITY_NOT_ELIGIBLE` là 409 (PARTY_SYSTEM §6.1) hay 422 (API_DESIGN §5)? → A: `409` theo PARTY_SYSTEM (sở hữu luật party) và response đã khai báo của `startPartySession`; API_DESIGN cần sửa.
- Q: Phát hiện mất kết nối thế nào khi gateway không có DB? → A: worker `party-presence-sweeper` đọc presence `ew:presence:user:<id>` do realtime ghi (VALKEY_DESIGN §3.1) — research R3.
- Q: Ba enum loại hoạt động không khớp (OpenAPI `PartyActivitySelection`, AsyncAPI `ActivityRef.kind`, DB `party_activities.activity_type`) và đều thiếu `boss`? → A: Thống nhất một enum `PartyActivityType` = `explore`, `quest`, `boss`, `learning_challenge`, `coop_challenge`, `team_battle` (delta Phase 2); M4a chỉ bật `explore`, `quest`, `boss` — research R4.
- Q: "Leader kết thúc phiên" (`leader_ended`) có API không? → A: Chưa có operation; Release 1.0 kết thúc theo hoàn thành/hết giờ/rời/giải tán. Ghi Ngoài phạm vi.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Party Lobby: sẵn sàng, chọn hoạt động (Priority: P1)

Các thành viên thấy nhau trong lobby (avatar, online, Ready), bấm Sẵn sàng; leader chọn hoạt động trong danh sách mà cả nhóm đủ điều kiện (US-PTY-002).

**Why this priority**: Lobby là điều kiện của Start Session và là trải nghiệm chính khi chờ chơi chung.

**Independent Test**: 3 client headless trong một party; B `party.member.ready_set` → A, C nhận `party.member.ready_changed` ≤ 1 s; leader chọn quest → mọi người nhận `party.activity.selected`, ready reset; B chọn → `PARTY_NOT_LEADER`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F101-AC-01 | KHI thành viên đặt Ready/Not ready (`setPartyReady` hoặc WS `party.member.ready_set`) trong khi party `open`, module competition PHẢI lưu trạng thái lobby và đẩy `party.member.ready_changed` (kèm `all_ready`) tới mọi thành viên trong ≤ 1 giây p95, không ghi domain event | FR-PTY-006 · AC-PTY-003 | TC-PTY-004 |
| F101-AC-02 | NẾU party không ở `open`, người gọi không là thành viên, hoặc lệnh lobby của một thành viên vượt giới hạn tần suất, THÌ module competition PHẢI trả `422 INVALID_STATE_TRANSITION`, `404 PARTY_NOT_FOUND` hoặc `429 RATE_LIMITED` tương ứng | FR-PTY-006 · [PARTY_SYSTEM §10](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | — |
| F101-AC-03 | KHI leader chọn hoạt động (`selectPartyActivity` hoặc WS `party.activity.select`) với `expected_version` khớp `state_version`, module competition PHẢI lưu lựa chọn, reset Ready của mọi thành viên, tăng `state_version`, phát `party.activity_selected` và đẩy `party.activity.selected` kèm `ineligible_members` (mã lý do) | FR-PTY-006 · AC-PTY-004 | TC-PTY-005 |
| F101-AC-04 | NẾU người chọn hoạt động không phải leader, THÌ module competition PHẢI trả `403 PARTY_NOT_LEADER`; NẾU `expected_version` lệch, THÌ PHẢI trả `409 VERSION_MISMATCH` kèm `state_version` hiện tại | FR-PTY-006 · AC-PTY-004 | TC-PTY-005 |
| F101-AC-05 | KHI thành viên gọi `listPartyEligibleActivities`, module competition PHẢI trả hoạt động mà mọi thành viên đủ điều kiện và hoạt động chỉ một số đủ điều kiện kèm cảnh báo theo thành viên (mã lý do + yêu cầu của hoạt động, không lộ khối/lớp thực của bạn khác) | FR-PTY-006 · AGENTS R16 · [PARTY_SYSTEM §5](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-005 |
| F101-AC-06 | KHI thành viên mở lobby hoặc kết nối lại, module competition PHẢI cung cấp snapshot đầy đủ (thành viên, presence, Ready, hoạt động đã chọn, `state_version`) qua `getParty` và `party.state.snapshot`; NẾU state tạm bị mất, THÌ PHẢI dựng lại từ dữ liệu bền vững với Ready = false | FR-PTY-006 · ADR-0004 | TC-PTY-004 |

---

### User Story 2 — Bắt đầu và kết thúc phiên chơi chung (Priority: P1)

Khi mọi người sẵn sàng, leader bấm Bắt đầu; cả nhóm vào cùng một phòng; hết phiên thì quay lại lobby, mỗi người xem kết quả học tập của riêng mình (US-PTY-003).

**Why this priority**: Giá trị cốt lõi của party là chơi chung trong cùng một phiên.

**Independent Test**: Party 3 người đều Ready, leader `startPartySession` → `201`, 3 client nhận `party.session.started` cùng `room_id`; thành viên không đủ khối → `409 PARTY_ACTIVITY_NOT_ELIGIBLE`, không ai vào phiên.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F101-AC-07 | KHI leader gọi `startPartySession` (`Idempotency-Key`, `expected_version`) và mọi thành viên online đều Ready, số thành viên online nằm trong khoảng tối thiểu–tối đa của hoạt động và mọi thành viên đủ điều kiện, module competition PHẢI trong một transaction tạo party activity `running`, ghi session membership cho từng thành viên tham gia, đặt party `in_activity`, phát `party.activity_started`, trả `201` (`room_id`, `game_session_ref`, `members`) và đẩy `party.session.started` cùng `room_id` tới mọi thành viên tham gia | FR-PTY-007 · AC-PTY-005 | TC-PTY-006 |
| F101-AC-08 | NẾU có thành viên không đủ điều kiện (khối/lớp theo hoạt động, grade band, tiên quyết quest, mở khóa, sanction), THÌ module competition PHẢI trả `409 PARTY_ACTIVITY_NOT_ELIGIBLE` kèm danh sách `student_id` + mã lý do, không tạo activity/room và không đưa thành viên nào vào phiên | FR-PTY-007 · AC-PTY-005 | TC-PTY-006 |
| F101-AC-09 | NẾU người gọi không phải leader, THÌ module competition PHẢI trả `403 PARTY_NOT_LEADER`; NẾU còn thành viên online chưa Ready hoặc thiếu số tối thiểu, THÌ PHẢI trả `409 PARTY_NOT_READY`; thiếu `Idempotency-Key` → `428 PRECONDITION_REQUIRED`; gửi lại cùng key PHẢI trả cùng kết quả và chỉ một activity | FR-PTY-007 · AC-PTY-004 · AGENTS R10 | TC-PTY-005 |
| F101-AC-10 | KHI hoạt động là `quest` hoặc `boss`, module competition PHẢI lấy điều kiện và tham chiếu phiên (encounter boss hoặc phiên quest chung) từ module game (idempotent theo `party_activity_id`); NẾU module game lỗi hoặc quá thời gian, THÌ PHẢI không đổi trạng thái party và trả `503 UNAVAILABLE` có `Retry-After`; hoạt động `explore` và `quest` PHẢI được cấp `private_instance` của khu tương ứng, `boss` dùng phòng của encounter | FR-PTY-007 · AGENTS R29 · ADR-0012 | TC-PTY-006 |
| F101-AC-11 | NẾU học sinh không thuộc session membership của phiên gửi lệnh vào phòng `private_instance` của party, THÌ module competition PHẢI trả `ROOM_NOT_MEMBER` và phòng không bị ảnh hưởng | FR-PTY-007 · AC-PTY-010 | TC-PTY-011 |
| F101-AC-12 | KHI nhận `boss.encounter_completed` hoặc `quest.party_completed` của activity đang chạy, khi mọi thành viên rời phiên, khi số thành viên online dưới tối thiểu quá ngưỡng, hoặc khi party bị giải tán, module competition PHẢI kết thúc activity (`ended_at`, `end_reason` ∈ `completed`/`timeout`/`all_left`/`disbanded`/`server_failure`), đóng session membership, đưa party về `open` (trừ `disbanded`) với Ready reset, phát `party.activity_ended` (thêm `party.activity_completed` khi `completed`) và đẩy `party.session.ended` | FR-PTY-007 · [PARTY_SYSTEM §6.3](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-006 |
| F101-AC-13 | HỆ THỐNG PHẢI không lưu và không trả điểm, kết quả học tập hay mastery của bất kỳ thành viên nào trong party activity, tóm tắt phiên hay message party; kết quả học tập chỉ đọc qua API attempt của chính học sinh | FR-PTY-013 · AC-PTY-009 | TC-PTY-010 |

---

### User Story 3 — Mất kết nối: thành viên và leader (Priority: P1)

Một bạn rớt mạng thì cả nhóm vẫn chơi tiếp (nếu hoạt động cho phép) và bạn đó quay lại đúng phòng; leader rớt mạng quá lâu thì nhóm có leader mới (US-PTY-004).

**Why this priority**: Mạng trường học không ổn định; không xử lý thì party bị kẹt.

**Independent Test**: Ngắt WS của C trong phiên quest: A, B tiếp tục, thấy C `disconnected`; C kết nối lại trong grace → cùng `room_id`. Ngắt leader A ở lobby quá `party_leader_grace` (Clock giả) → leader = thành viên online gia nhập sớm nhất, `party.leader.transferred` reason `leader_disconnected`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F101-AC-14 | KHI presence của một thành viên hết hạn hoặc chuyển trạng thái, module competition PHẢI cập nhật trạng thái thành viên (`online`/`away`/`in_activity`/`disconnected`) trong một chu kỳ quét, đẩy `party.member.status_changed`, và reset Ready của thành viên đó nếu party đang ở lobby; thành viên vẫn giữ tư cách thành viên | FR-PTY-012 · [PARTY_SYSTEM §9](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-008 |
| F101-AC-15 | KHI leader mất kết nối, module competition PHẢI bắt đầu grace `party_leader_grace` (snapshot báo leader đang kết nối lại); NẾU leader quay lại trong grace, THÌ PHẢI giữ nguyên leader; NẾU hết grace, THÌ PHẢI chuyển leader cho thành viên online gia nhập sớm nhất, phát `party.leader_transferred` (reason `leader_disconnected`), đẩy `party.leader.transferred` và ghi audit | FR-PTY-008 · AC-PTY-008 | TC-PTY-009 |
| F101-AC-16 | NẾU hết grace leader mà không còn thành viên online, THÌ module competition PHẢI giữ party ở trạng thái ngủ (dormant) và giao leader cho thành viên đầu tiên online trở lại; party `friend`/`custom`/`study_group` không có ai online quá `party_dormant_timeout` PHẢI tự giải tán với reason `inactivity` | FR-PTY-008 · [PARTY_SYSTEM §9](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) | TC-PTY-009 |
| F101-AC-17 | TRONG KHI party `in_activity` (`explore`/`quest`), khi một thành viên mất kết nối, module competition PHẢI giữ chỗ trong `disconnect_grace` của loại phòng, cho thành viên khác tiếp tục khi hoạt động cho phép, đưa thành viên quay lại trong grace (kể cả từ thiết bị khác sau takeover) về đúng `room_id`; hết grace PHẢI đóng membership với lý do `disconnected` | FR-PTY-012 · AC-PTY-007 · ADR-0031 | TC-PTY-008 |
| F101-AC-18 | NẾU hoạt động không cho tiếp tục khi thiếu người hoặc số thành viên online dưới tối thiểu quá ngưỡng, THÌ module competition PHẢI kết thúc activity với `timeout` như F101-AC-12 | FR-PTY-012 · AC-PTY-007 | TC-PTY-008 |

---

### User Story 4 — Phân quyền và cô lập tenant (Priority: P1)

**Why this priority**: Điều III, V của constitution.

**Independent Test**: Sinh test từ OpenAPI cho 4 operation mới/được dùng × token nhân sự → `403`; lệnh WS với `party_id` tenant khác → từ chối.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F101-AC-19 | HỆ THỐNG PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY` cho `setPartyReady`, `selectPartyActivity`, `startPartySession`, `listPartyEligibleActivities` khi token không phải token game của học sinh | AGENTS R23 · ADR-0031 | TC-AUTH-014 |
| F101-AC-20 | NẾU REST hoặc lệnh WS (`party.member.ready_set`, `party.activity.select`, `party.session.start`) tham chiếu party của tenant/trường khác hoặc party người gọi không là thành viên, THÌ module competition PHẢI từ chối (`404 PARTY_NOT_FOUND` / `system.command.failed` `PERMISSION_DENIED`) không tiết lộ party tồn tại; gọi facade `game` PHẢI mang `TenantScope` của party trong context (`src/backend/pkg/modcall`) | AGENTS R8 · AGENTS R29 · ADR-0065 | TC-TEN-012 |

### Edge Cases

- NẾU leader gửi start khi một thành viên vừa rời (state_version đổi), THÌ `409 VERSION_MISMATCH`; client làm mới snapshot rồi thử lại.
- NẾU commit thất bại sau khi module game đã tạo encounter/phiên quest, THÌ module game tự đóng phòng không có ai vào sau `disconnect_grace` (research R2); thử lại cùng `Idempotency-Key` nhận lại cùng phòng.
- NẾU thành viên offline (không online) lúc start, THÌ không bắt buộc Ready và không được đưa vào phiên; vẫn là thành viên party.
- NẾU party bị giải tán khi đang `in_activity`, THÌ activity kết thúc `disbanded`; module game tự xử lý encounter/quest theo `party.disbanded` (F103).
- NẾU Valkey mất, THÌ lobby dựng lại từ PG (Ready reset, grace leader bắt đầu lại từ đầu); không mất membership.
- NẾU leader đổi hoạt động, THÌ mọi Ready reset và client hiển thị thông báo (F102).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-PTY-006 | Lobby: thành viên, presence, Ready, leader chọn hoạt động/mode, danh sách đủ điều kiện, snapshot | [PARTY_SYSTEM §5](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-007 | Start Session 9 bước, kết thúc phiên, session membership | [PARTY_SYSTEM §6](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) · [MULTIPLAYER_GAMEPLAY §10](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| FR-PTY-008 | Grace leader, tự chuyển leader, dormant | [PARTY_SYSTEM §9](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| FR-PTY-012 | Thành viên mất kết nối, tiếp tục khi hoạt động cho phép, quay lại đúng phòng | [MULTIPLAYER_GAMEPLAY §9](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| FR-PTY-013 | Không chia sẻ kết quả học tập cá nhân trong party | [PARTY_SYSTEM §1, §7.1](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |

### Key Entities

- **Lobby state**: trạng thái tạm của party (hoạt động đã chọn, Ready, presence từng thành viên, phiên bản).
- **PartyActivity**: một phiên chơi chung của party (loại, tham chiếu nội dung, phòng, trạng thái, lý do kết thúc, tóm tắt không có điểm).
- **SessionMembership**: ai ở phòng nào, vào/ra khi nào, lý do ra.
- **Room**: bản ghi tóm tắt phòng `private_instance` do module competition cấp; phòng `boss` thuộc module game.
- **Leader grace**: khoảng chờ leader kết nối lại trước khi tự chuyển leader.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `party_leader_grace` | Grace leader mất kết nối |
| `party_dormant_timeout` | Tự giải tán party không ai online |
| `disconnect_grace` | Giữ chỗ thành viên trong phòng theo loại (quest co-op, hub) |
| `party_seat_hold` | Giữ chỗ cho party khi leader chuyển khu |
| `private_instance_capacity` | Sức chứa `private_instance` |
| `ws_timeout` | Ngưỡng presence hết hạn |
| `idempotency_record_ttl` | Lưu kết quả `startPartySession` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Ready/presence chỉ ở Valkey (tạm), activity/membership ở PostgreSQL | AGENTS R9 · ADR-0004 |
| Grace timer do worker module competition quản lý, không phụ thuộc node gateway | [PARTY_SYSTEM §9](../../docs/03-social-multiplayer/PARTY_SYSTEM.md) |
| Gọi module game qua facade `game`; `startPartyActivity` mang `Idempotency-Key` | AGENTS R29 · ADR-0074 |
| Lobby cập nhật ≤ 1 s p95 | AC-PTY-003 |
| Không lộ điểm/khối/lớp của bạn khác | AGENTS R16 · FR-PTY-013 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Thay đổi Ready hiển thị cho mọi thành viên trong ≤ 1 giây ở p95 (party 4 người, mạng bình thường).
- **SC-002**: 100% lần start có thành viên không đủ điều kiện không đưa ai vào phiên; 100% lần start hợp lệ đưa mọi thành viên tham gia vào cùng một phòng.
- **SC-003**: Leader mất kết nối: party có leader mới trong ≤ grace + 1 chu kỳ quét; leader quay lại trong grace giữ vai trò ở 100% trường hợp test.
- **SC-004**: 0 điểm/kết quả học tập của học sinh khác xuất hiện trong dữ liệu party hoặc message gửi tới thành viên.

## Ngoài phạm vi

- Tạo/mời/kick/giải tán party → F100. Giao diện lobby trong Godot → F102.
- Logic encounter boss, tiến độ quest chung, bước `interdependent`, điều kiện quest/boss → F103 (F101 chỉ gọi facade `game`).
- Room host `private_instance` (đồng bộ vị trí, `hub.instance.join`) → F091; F101 cấp bản ghi phòng, membership và kiểm tra quyền vào phòng.
- Hoạt động `coop_challenge`, `learning_challenge`, `team_battle` và ticket ghép trận cho party `competition_team` → F104, F105.
- "Đi theo trưởng nhóm" khi chuyển khu (message WS chưa có trong AsyncAPI) → F091.
- Leader chủ động kết thúc phiên (`leader_ended`): chưa có operation; đề xuất bổ sung khi có nhu cầu.
- Chat `party` trong lobby/phiên → F117, F119.

## Assumptions

- Ngưỡng "số online dưới tối thiểu quá 120 s" (PARTY_SYSTEM §9) và tần suất lệnh lobby 5/giây (§10) chưa có tên CONFIG_DEFAULTS; đề xuất `party_activity_min_members_timeout`, `party_lobby_cmd_rate` (task Polish) [Confirmed].
- Chu kỳ quét presence 5 s theo PARTY_SYSTEM §9 là hằng số code của worker [Confirmed].
- F103 phát `boss.encounter_completed` có `party_activity_id` và event mới `quest.party_completed`; F090 ghi presence `ew:presence:user:<id>`.
- Giờ truy cập/giới hạn chơi (OQ-17, đã trả lời 2026-11-20): không kiểm riêng ở party/lobby; game session bị từ chối hoặc kết thúc theo giờ giới nghiêm và giới hạn/ngày ở F014 nên học sinh không vào được phòng.

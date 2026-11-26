# Party, Group & Cooperative Multiplayer

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: Multiplayer Engineer + Game Director · Cập nhật: 2026-11-27 (F102: §12.4 thành phần Godot đã hiện thực, tên scene theo GODOT_ARCHITECTURE §3) · 2026-11-15
> Liên quan: [ADR-0050 Party vs Club](../adr/ADR-0050-party-tam-thoi-va-guild-club-ben-vung-la-hai-khai-niem-tach.md) · [ADR-0054](../adr/ADR-0054-tuong-tac-xa-hoi-lien-truong-cross-school-social-interaction.md) · [MULTIPLAYER_GAMEPLAY](MULTIPLAYER_GAMEPLAY.md) · [SOCIAL_SYSTEM](SOCIAL_SYSTEM.md) · [MATCHMAKING](MATCHMAKING.md) · [CHAT_SYSTEM](CHAT_SYSTEM.md) · [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) · [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) · [DOMAIN_MODEL §6](../06-architecture/DOMAIN_MODEL.md#6-multiplayer-party--competition) · [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) · [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) · [OpenAPI](../../specs/openapi) · [AsyncAPI](../../specs/asyncapi/eduworld-realtime.v1.yaml)

> [!IMPORTANT]
> **Bỏ khái niệm lớp — [ADR-0095](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) (owner chốt 2026-11-21).** Đã áp vào R7, R12, visibility, loại party trong tài liệu này: không có `visibility = class`, cờ `cross_class`, cấp policy `class`; party lớp 1–3 cùng đúng khối, khác trường được trong cùng tenant (ADR-0054), không liên tenant; party `study_group` đổi tên `study_party`. Code, contract (`competition.v1.yaml` `PartyType`/`PartyVisibility`, AsyncAPI, `party.created.v2`), migration `20261231000002_adr0095_noclass.sql` và spec F100/F101 đã sửa ở WP-C1 (2026-11-21); cấp trường của `party_policies` có thêm khóa `same_grade_only` (chỉ bật) ([CODE_PLAN §4.2](../12-project/CODE_PLAN_CMS_LMS_FIRST.md#42-kế-hoạch-bỏ-khái-niệm-lớp-adr-0095)).

Tài liệu này hiện thực hóa **toàn bộ Party Addendum** (nguồn [Confirmed]) và **sở hữu** mọi luật về Party (nhóm chơi **tạm thời**): thao tác, loại party, Party Lobby, co-op session, leader/member disconnect, rate limit lời mời. Nhóm **bền vững** (Club/Guild, Study Group, Team cố định) thuộc [SOCIAL_SYSTEM §6](SOCIAL_SYSTEM.md#6-club--guild-nhóm-bền-vững--adr-0050-confirmed) (ADR-0050). Domain chạy trong **module competition**, database `eduworld_competition`. Mọi tham số số học chưa có trong addendum mang nhãn [Confirmed].

## 1. Mục tiêu

- Nhiều học sinh lập một Party để cùng di chuyển, khám phá, làm quest, giải puzzle, tham gia learning challenge và team competition **[Confirmed]**.
- Mỗi học sinh vẫn có **learning attempt và kết quả riêng**; không chia sẻ kết quả học tập cá nhân giữa thành viên **[Confirmed]**.
- Không cho phép dùng Party để vượt qua các điều kiện bảo vệ học sinh **[Confirmed]**.

## 2. Party Management — thao tác

| Thao tác | REST | Lệnh WS (client→server) | Ai được làm | Ghi chú |
|---|---|---|---|---|
| Create Party | `POST /api/v1/multiplayer/parties` | — | Học sinh không có active party | `Idempotency-Key` bắt buộc |
| Get Party | `GET /api/v1/multiplayer/parties/{party_id}` | — | Member | |
| Get my Party | `GET /api/v1/multiplayer/parties/me` | — | Học sinh | |
| Update Settings | `PATCH /api/v1/multiplayer/parties/{party_id}` | — | Leader | name, visibility, invitation_policy, max_members (≥ số member hiện tại) |
| Invite Friend | `POST /api/v1/multiplayer/parties/{party_id}/invitations` | — (invitee nhận `party.invitation.received`) | Leader; member nếu `invitation_policy=members` | Rate limit §10 |
| List my invitations | `GET /api/v1/multiplayer/invitations?status=pending` | — | Invitee | |
| Accept Invitation | `POST /api/v1/multiplayer/invitations/{invitation_id}:accept` | — | Invitee | Kiểm tra lại toàn bộ rule tại thời điểm accept |
| Reject Invitation | `POST /api/v1/multiplayer/invitations/{invitation_id}:reject` | — | Invitee | |
| Cancel Invitation | `POST /api/v1/multiplayer/invitations/{invitation_id}:cancel` | — | Inviter, leader | |
| Join Party | `POST /api/v1/multiplayer/parties/{party_id}:join` | — | Chỉ khi `visibility=friends` và thỏa rule; party `private` bắt buộc có invitation (không còn `visibility=class` — ADR-0095) | |
| Leave Party | `POST /api/v1/multiplayer/parties/{party_id}:leave` | — | Member | Leader rời → chuyển leader tự động §9 |
| Kick Member | `POST /api/v1/multiplayer/parties/{party_id}/members/{student_id}:kick` | — | Leader | Người bị kick không được mời lại vào party đó trong `party_kick_reinvite_block` (10 phút) [Confirmed] |
| Transfer Leader | `POST /api/v1/multiplayer/parties/{party_id}:transfer-leader` | — | Leader | Đích phải là member `online` |
| Disband Party | `POST /api/v1/multiplayer/parties/{party_id}:disband` | — | Leader; `safety_moderator` (khi xử lý vụ việc, có audit) | Kết thúc activity đang chạy theo §6.3 |
| Set Ready | `PUT /api/v1/multiplayer/parties/{party_id}/members/me/ready` | `party.member.ready_set` | Member | Chỉ ở Party Lobby |
| Select Activity / Mode | `PUT /api/v1/multiplayer/parties/{party_id}/activity` | `party.activity.select` | Leader | |
| Start Session | `POST /api/v1/multiplayer/parties/{party_id}:start` | `party.session.start` | Leader | §6 |
| Party Chat (**always-on**, ADR-0020 mục 6; văn bản tự do có từ P4 theo FR-CHAT-009) | Channel `party` — [CHAT_SYSTEM](CHAT_SYSTEM.md) §4.1 | `chat.message.send` | Member | module competition ghi outbox `party.created`/`party.member_joined` cùng transaction tạo party/thêm member; module chat tiêu thụ event để tạo channel và thêm member (idempotent, thường < 1 s — hai database khác nhau nên không thể cùng transaction, ADR-0036/0042); không tắt được bằng cài đặt trường/lớp; đóng (read-only) khi disband, purge sau `gameplay_chat_retention_days` (ADR-0033) |

Thao tác ghi đi qua REST (có `Idempotency-Key`); WS chỉ dùng cho ready, chọn activity và start session. Mọi thay đổi trạng thái sinh domain event theo [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) (`party.created`, `party.invitation_sent|_accepted|_rejected|_expired`, `party.member_joined`, `party.member_left`, `party.member_kicked`, `party.leader_transferred`, `party.activity_started`, `party.activity_ended`, `party.disbanded`) qua outbox → NATS JetStream (ADR-0042). module competition phát message WS qua realtime theo [AsyncAPI](../../specs/asyncapi/eduworld-realtime.v1.yaml): `party.invitation.received`, `party.invitation.updated`, `party.member.joined`, `party.member.left`, `party.member.status_changed`, `party.member.ready_changed`, `party.leader.transferred`, `party.activity.selected`, `party.session.started`, `party.session.ended`, `party.party.disbanded`, `party.state.snapshot`. Lỗi theo Problem Details với `code` ổn định: `PARTY_ALREADY_IN_PARTY`, `PARTY_FULL`, `PARTY_NOT_LEADER`, `PARTY_INVITE_NOT_ALLOWED` (`invitation_policy` hoặc chặn nhau — không tiết lộ lý do), `PARTY_POLICY_VIOLATION` (phạm vi trường/tenant/lớp/khối, sanction, chặn mời lại sau kick — có audit; [API_DESIGN §5](../07-backend/API_DESIGN.md)), `PARTY_INVITE_RATE_LIMITED`, `PARTY_NOT_FOUND`, `PARTY_INVITATION_EXPIRED`, `PARTY_MEMBER_NOT_ONLINE`, `409 PARTY_ACTIVITY_NOT_ELIGIBLE`.

## 3. Thuộc tính Party

| Thuộc tính | Field | Mặc định | Nhãn |
|---|---|---|---|
| Party ID | `party_id` (UUIDv7) | | [Confirmed] |
| Party Name | `name` | Sinh tự động "Nhóm của <nickname>"; tên tự đặt đi qua bộ lọc [MODERATION](MODERATION.md) | [Confirmed] / lọc [Confirmed] |
| Leader ID | `leader_id` | Người tạo | [Confirmed] |
| Members | `party_members` | | [Confirmed] |
| Maximum Members | `max_members` | 4 (cấu hình 2–8; giới hạn cứng platform 8) | [Confirmed] cấu hình / giá trị [Confirmed] |
| Party Type | `party_type` | `friend` | [Confirmed] |
| Visibility | `visibility` ∈ `private`, `friends` (bỏ `class`; party `class` còn mở chuyển `private` — ADR-0095) | `private` | [Confirmed] / giá trị [Confirmed] |
| Invitation Policy | `invitation_policy` ∈ `leader_only`, `members` | `leader_only` | [Confirmed] / giá trị [Confirmed] |
| Current Activity | `current_activity_id` → `multiplayer.party_activities` | null | [Confirmed] |
| Current Game Session | `current_game_session_ref` (room/match id) | null | [Confirmed] |
| Tenant / Trường | `tenant_id`, `school_id` | Của leader lúc tạo | [Confirmed] |
| Club nguồn | `club_id` (nullable) | Có khi party được tạo từ một club ([SOCIAL_SYSTEM §6.4](SOCIAL_SYSTEM.md#64-quan-hệ-party--club)); chỉ mời thành viên club | [Confirmed] |
| Grade band | `grade_band` | Grade band của leader | [Confirmed] |
| Status | `status` ∈ `open`, `in_activity`, `disbanded` | `open` | [Confirmed] |
| Created/Updated At | `created_at`, `updated_at` | | [Confirmed] |

## 4. Party Types

| `party_type` | Ai tạo | Thời hạn | Đặc thù | Nhãn |
|---|---|---|---|---|
| `friend` | Học sinh | Tạm thời — tự giải tán sau 30 phút không có member online | Chỉ mời bạn bè | [Confirmed] |
| `study_party` (đổi tên từ `study_group` — ADR-0095) | Học sinh (lớp 6–9) | Tạm thời — **phiên học nhóm** ("Nhóm cùng học") do học sinh lập; không gắn `club_id`; hệ thống không còn nhóm học tập LMS | Mặc định chọn activity học tập | [Confirmed] |
| `quest` | Học sinh khi nhận cooperative quest | Đến khi quest kết thúc | `current_activity` cố định là quest | [Confirmed] |
| `competition_team` | Học sinh | Theo mùa/giải | Dùng cho team matchmaking/tournament; size = size đội của mode | [Confirmed] |
| `custom` | Học sinh lớp 6–9 | Tạm thời | Cấu hình tự do trong khuôn khổ policy | [Confirmed] |

Party chỉ do học sinh tạo và chơi; nhân sự nhà trường không tạo hay quản trị party (không vào game, ADR-0031, ADR-0072). Không có party theo lớp và không có nhóm học tập LMS ([ADR-0095](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md)); học sinh học cùng nhau bằng party `friend`/`quest`/`study_party` theo các quy tắc §8.

## 5. Party Lobby

Party Lobby là **màn hình + trạng thái realtime** của party khi chưa (hoặc giữa các) hoạt động.

```text
┌─────────────── Party: "Nhóm Sao Băng" (Quest Party) ────────────┐
│ 👑 MèoXanh42 (bạn)   ● Online   ✔ Sẵn sàng                     │
│    CáHeoVui07        ● Online   ✔ Sẵn sàng                     │
│    GấuNâu15          ● Online   … Chưa sẵn sàng                │
│    [+ Mời bạn]                                                  │
│─────────────────────────────────────────────────────────────────│
│ Hoạt động: [Co-op Quest ▼] "Sửa cầu Rừng Phiêu Lưu"             │
│ [Chat nhóm] [Bắt đầu] (chỉ leader, khi tất cả sẵn sàng)         │
└─────────────────────────────────────────────────────────────────┘
```

Trải nghiệm: tạo party từ Social Zone/menu/danh sách bạn bè/club; mời bằng cách chạm avatar bạn (lớp nhỏ không cần gõ tên); lời mời là popup nhỏ có thể tắt; kết thúc hoạt động có tổng kết chung + kết quả học tập **riêng** từng người (chỉ người đó thấy điểm của mình).

| Chức năng | Mô tả | Nguồn dữ liệu |
|---|---|---|
| Danh sách thành viên | nickname, avatar, level, vai trò leader | PG + cache |
| Avatar & trạng thái online | Enum presence chung ([MULTIPLAYER_GAMEPLAY §5](MULTIPLAYER_GAMEPLAY.md#5-presence)); thành viên đang trong grace mất kết nối hiển thị nhãn "đang kết nối lại" | Valkey presence |
| Ready / Not Ready | Từng member toggle; reset khi đổi activity | Valkey `ew:party:<party_id>:members` |
| Leader Controls | Kick, transfer, đổi settings, chọn activity/mode, start | WS/REST |
| Activity Selection | Danh sách activity **mà mọi member đều đủ điều kiện** (grade, quyền, quest prerequisite); activity mà chỉ một số member đủ điều kiện hiển thị kèm cảnh báo, không thể start | REST `GET /api/v1/multiplayer/parties/{id}/eligible-activities` |
| Activity Type | Một enum `PartyActivityType` dùng chung cho REST, realtime và DB: `explore`, `quest`, `boss`, `learning_challenge`, `coop_challenge`, `team_battle`. Boss nhóm chỉ bắt đầu từ Party Lobby (leader chọn `boss`, start theo §6; encounter do module game tạo qua event/API của luồng này — [CHALLENGE_BOSS_SYSTEM §7](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)) | |
| Invitation | Mời bạn bè online/offline (offline nhận notification) | |
| Group Chat | Channel `party` nhúng, **luôn có** (thu gọn được, không tắt được); quick message + text; menu Tắt tiếng / Chặn / Báo cáo trên từng tin | CHAT (ADR-0020 mục 6) |
| Start Session | Nút Start chỉ active khi: leader, ≥ `min_members` của activity, tất cả member online đều `ready` | |

Lobby state: Ready, trạng thái thành viên và các timer ở Valkey (key do [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) sở hữu: `ew:party:<party_id>:lobby`); hoạt động leader chọn lưu bền vững ở `multiplayer.parties` ([DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md)). Mỗi thay đổi tăng `state_version`; client gửi `expected_version` cho lệnh leader để tránh race (lệch → `412 VERSION_MISMATCH`, WS `CONFLICT`). Hiện thực và quyết định triển khai: [F101](../../specs/101-party-lobby-session/spec.md).

## 6. Party Session

### 6.1. Luồng 9 bước [Confirmed]

```mermaid
sequenceDiagram
  autonumber
  participant L as Leader (Godot)
  participant M as Members (Godot)
  participant RT as realtime
  participant API as module competition (party)
  participant AZ as module identity (principal / membership)
  participant GS as game_session / room
  participant VK as Valkey
  participant PG as PostgreSQL

  L->>API: POST /multiplayer/parties/{id}:start (Idempotency-Key, activity_id, expected_version)
  API->>VK: Đọc lobby state (lock ew:party:<id>:lock 5s)
  API->>API: (1) Xác nhận caller là leader hiện tại
  API->>API: (2) Kiểm tra thành viên: online, ready, ≥ min_members, ≤ max của activity
  API->>AZ: (3) Kiểm tra khối/grade/quyền tham gia từng member với activity
  alt Có member không đủ điều kiện
    API-->>L: 409 PARTY_ACTIVITY_NOT_ELIGIBLE + danh sách member/lý do
  end
  API->>PG: (4) Tạo party_activity + room (private_instance | coop | matchmaking ticket)
  API->>PG: (5) Gán thành viên: session_memberships, quest_progress riêng từng HS (+ shared progress record nếu quest cho phép)
  API->>PG: Outbox party.activity_started
  API-->>L: 201 {party_activity_id, room_id, join_info}
  PG-->>RT: Event party.activity_started (outbox → NATS → PUBLISH ew:rt:user:<member_id>)
  RT-->>L: (6) party.session.started {room_id, area, spawn}
  RT-->>M: (6) party.session.started
  L->>RT: hub.instance.join (private_instance)
  M->>RT: hub.instance.join (private_instance)
  RT-->>L: (7) Đồng bộ trạng thái: party.state.snapshot, coop.state.snapshot, hub.snapshot.updated
  RT-->>M: (7)
  loop Trong session
    RT->>RT: (8) Theo dõi heartbeat, disconnect/reconnect (MULTIPLAYER_GAMEPLAY §9)
  end
  RT->>API: Activity hoàn thành / hết giờ / leader end
  API->>PG: (9) Kết thúc session: party_activity kết thúc, kết quả từng HS đã ghi qua attempt riêng, party_histories
  API->>PG: Outbox party.activity_ended
  RT-->>L: party.session.ended {summary}
  RT-->>M: party.session.ended
```

Phía module game của bước (3)–(5) do F103 hiện thực (facade `listPartyActivityCandidates`, `checkPartyActivityEligibility`, `startPartyActivity`, chỉ module competition gọi được): `activity_ref` là `code` của quest/boss; mọi thành viên phải là học sinh của tenant trong `TenantScope` (khác → `422`); lý do không đủ điều kiện chỉ là mã + yêu cầu của hoạt động (`grade_levels:3`), không bao giờ khối/trường của học sinh; `startPartyActivity` idempotent theo `party_activity_id` — boss trả `room_type = boss` (encounter), quest trả `private_instance` (phiên quest chung, [QUEST_SYSTEM §6](../01-game-design/QUEST_SYSTEM.md#6-party--shared-progress)); thiếu `Idempotency-Key` → `428`.

### 6.2. Kiểm tra ở bước (3) — eligibility

| Điều kiện | Mặc định | Nhãn |
|---|---|---|
| Cùng `school_id` | Bắt buộc; khác trường cùng tenant `unit` khi tenant bật `cross_school_social_enabled`; không bao giờ khác tenant (ADR-0054) | [Confirmed] |
| Grade band | Mọi member trong cùng grade band, hoặc chênh tối đa 1 band liền kề nếu activity khai báo `allow_mixed_grade_band=true` (không bao giờ `g1_3` với `g8_9`) | [Confirmed] |
| Activity grade | `activity.grade_levels` chứa `grade_level` của từng member | [Confirmed] (kiểm tra khối) |
| Trạng thái tài khoản | `active`, không bị sanction `restrict_multiplayer` | [Confirmed] |
| Giờ truy cập | Trong khung giờ trường cho phép, ngoài giờ giới nghiêm platform và chưa chạm giới hạn ngày của trường nếu trường bật ([CHILD_SAFETY §4](../10-security/CHILD_SAFETY.md#4-thời-gian-chơi-và-sức-khỏe-oq-17), OQ-17) | [Confirmed — owner 2026-11-20] |
| Block | Không có cặp member nào block nhau (kiểm tra cả lúc invite/join) | [Confirmed] |
| Prerequisite quest | Mỗi member đáp ứng prerequisite của quest | [Confirmed] |

### 6.3. Kết thúc session

- Nguyên nhân: `completed`, `timeout`, `leader_ended`, `all_left`, `disbanded`, `server_failure`.
- Attempt học tập đang dở của từng học sinh: chuyển `abandoned` (practice/quest) hoặc `submitted` tự động (nếu activity là timed challenge) theo cấu hình activity.
- Party quay lại `open` (Party Lobby) sau khi session kết thúc, trừ `disbanded`. Channel `party` vẫn mở suốt vòng đời party (kể cả trong session); trận đội trong session có thêm channel `match`/`team` riêng.
- Party `disbanded` → channel `party` **đóng** (`closed_at`), chỉ đọc; purge vật lý sau `gameplay_chat_retention_days` (mặc định 7, tối thiểu 3) trừ khi còn report/case (ADR-0033).

## 7. Cooperative Gameplay

| Khả năng [Confirmed] | Hiện thực |
|---|---|
| Cùng di chuyển trong game world | Party được gán cùng `hub_instance` ([MULTIPLAYER_GAMEPLAY §6](MULTIPLAYER_GAMEPLAY.md#6-gán-instance)); "Follow leader" khi chuyển area: member nhận đề nghị follow (message WS riêng — [Confirmed], chưa có trong AsyncAPI v1) (không bắt buộc) |
| Cùng khám phá bản đồ | `private_instance` của quest exploration |
| Cùng thực hiện Quest | Cooperative Quest: `quest_version.shared_progress_policy` (xem dưới) |
| Cùng giải Educational Puzzle | Room `coop`: state machine server-authoritative; mỗi đóng góp là một câu trả lời của **một** học sinh → ghi vào attempt của học sinh đó |
| Learning Challenge | Mỗi member làm attempt riêng, kết quả nhóm = hàm tổng hợp (tổng/trung bình/ngưỡng) chỉ dùng cho tiến độ game & reward |
| Team Competition | Party `competition_team` vào team matchmaking ([MATCHMAKING §8](MATCHMAKING.md#8-party-queue--friend-duel); triển khai và quy tắc cỡ đội/cùng khối: [F104 Clarifications 2026-11-21](../../specs/104-matchmaking/spec.md#clarifications)) |
| Nhiệm vụ nhiều bước | Step gán cho `any_member`, `each_member`, `role:<x>` (vai trò trong nhóm); bước `interdependent` mỗi người giữ một phần dữ kiện ([QUEST_SYSTEM §3.2](../01-game-design/QUEST_SYSTEM.md#32-gán-bước-trong-party-và-bước-interdependent-confirmed)) |

### 7.1. Shared Quest Progress

`shared_progress_policy` cấu hình theo quest version **[Confirmed]**. Enum và ngữ nghĩa do [QUEST_SYSTEM §6](../01-game-design/QUEST_SYSTEM.md#6-party--shared-progress) sở hữu (khớp `quest.schema.json`): `individual` (mặc định) · `shared_steps` (bước khung chia sẻ, **bước học tập mỗi member tự làm** — mặc định cho cooperative quest) · `collective_count` (mục tiêu tổng của nhóm, mỗi câu trả lời vẫn ghi vào attempt riêng). PARTY_SYSTEM chỉ thực thi trong session co-op.

Bất biến **[Confirmed]**: không bao giờ copy `attempt`, `score`, `objective_mastery` từ member này sang member khác. Reward game của bước chia sẻ áp riêng cho từng member theo `reward_rule` (có cap, chống farm — [DOMAIN_MODEL §8](../06-architecture/DOMAIN_MODEL.md#8-gamification)).

## 8. Group Rules

| # | Quy tắc | Hiện thực | Nhãn |
|---|---|---|---|
| R1 | Một học sinh chỉ có một active Party (Addendum §6 ghi cho MVP; giữ cho Release 1.0) | Unique index `party_members(student_id) WHERE left_at IS NULL` + kiểm tra trong transaction | [Confirmed] |
| R2 | `max_members` cấu hình được | Party setting, giới hạn bởi platform (8) và activity | [Confirmed] |
| R3 | Leader quản lý Party | Kiểm tra `leader_id` ở mọi lệnh leader (đọc từ PG trong transaction, không tin client) | [Confirmed] |
| R4 | Chuyển Leader | Thủ công + tự động §9 | [Confirmed] |
| R5 | Xử lý Leader disconnect | §9 | [Confirmed] |
| R6 | Party riêng tư cần lời mời/quyền | `visibility=private` chỉ vào qua invitation `accepted` còn hạn | [Confirmed] |
| R7 | Giới hạn theo school, grade, activity | `party_policies` cấp platform/school (cấp school do `school_admin` đặt, chỉ siết): `allowed_party_types`, `same_grade_only` (chỉ bật; platform luôn bật cho lớp 1–3 — R12), `cross_grade_band`, `max_members_cap`; không còn cấp `class` và cờ `cross_class` (ADR-0095) | [Confirmed] (viết lại theo ADR-0095) |
| R8 | Không vượt qua điều kiện bảo vệ học sinh | Mọi kiểm tra chat policy, block, sanction, grade (ghép hoạt động) áp dụng y nguyên trong party; party chat dùng **cùng ChatPolicy cho mọi thành viên** (không phân biệt lớp, ADR-0020) và mọi tin phải qua kiểm tra hợp lệ | [Confirmed] / cơ chế [Confirmed] |
| R9 | Chống spam lời mời | §10 | [Confirmed] |
| R10 | Chống duplicate membership | Unique index R1 + unique `(party_id, student_id) WHERE left_at IS NULL`; accept invitation idempotent | [Confirmed] |
| R11 | Rate limiting & audit log | §10, §11 | [Confirmed] |
| R12 | Party lớp 1–3 | Chỉ tạo `friend`/`quest`; mọi thành viên phải **cùng đúng khối** (`grade_level`) với leader; **không bắt buộc cùng trường** nhưng **luôn cùng tenant** — khác trường trong tenant chỉ khi tenant bật tương tác liên trường (ADR-0054), party liên tenant không được phép (R8, ADR-0005); điều kiện riêng của loại party giữ nguyên (party `friend` chỉ mời bạn bè); người chơi khác chỉ thấy nickname + avatar (R16). Vi phạm khối → từ chối với lý do quy tắc khối. Giới hạn ghép nhóm, **không** giới hạn chat — party chat như mọi khối theo ADR-0020. Quy tắc này nới quy tắc cũ "bạn bè cùng lớp" theo quyết định owner ([ADR-0095](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) mục 5) | [Confirmed — owner 2026-11-21] |

## 9. Leader disconnect & member disconnect

```mermaid
stateDiagram-v2
  [*] --> LeaderOnline
  LeaderOnline --> LeaderGrace: leader mất kết nối
  LeaderGrace --> LeaderOnline: reconnect ≤ 60s
  LeaderGrace --> AutoTransfer: hết 60s
  AutoTransfer --> LeaderOnline: leader mới = member online gia nhập lâu nhất
  AutoTransfer --> Dormant: không còn member online
  Dormant --> LeaderOnline: member đầu tiên online trở lại thành leader
  Dormant --> Disbanded: 30 phút không ai online (friend/custom/study_party)
```

| Tình huống | Hành vi | Nhãn |
|---|---|---|
| Leader mất kết nối | Grace `party_leader_grace` (**60 s**); trong grace member vẫn chơi activity, lệnh leader tạm khóa; Party Lobby hiển thị "Trưởng nhóm đang kết nối lại" | [Confirmed] |
| Hết grace | Auto-transfer cho member **online gia nhập lâu nhất** (`joined_at` nhỏ nhất); domain event `party.leader_transferred` (WS `party.leader.transferred`) reason `leader_disconnected` | [Confirmed] |
| Leader cũ quay lại | Là member thường; leader mới có thể transfer lại | [Confirmed] |
| Member mất kết nối trong activity | Grace theo mode ([MULTIPLAYER_GAMEPLAY §9.2](MULTIPLAYER_GAMEPLAY.md#92-grace-theo-chế-độ-tài-liệu-này-sở-hữu-giá-trị)); người còn lại tiếp tục theo quy tắc disconnect của activity (quest: [QUEST_SYSTEM §6](../01-game-design/QUEST_SYSTEM.md#6-party--shared-progress); boss: [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)) [Confirmed]; nếu số member online < `min_members` quá `party_activity_min_members_timeout` (120 s) → activity kết thúc `timeout` | [Confirmed] |
| Member mất kết nối ở Party Lobby | Hiện "đang kết nối lại"; ready reset; vẫn là member đến khi party dormant/disband | [Confirmed] |

Grace timer do module competition quản lý (không phụ thuộc node gateway giữ kết nối leader): thời điểm mất kết nối lưu trong lobby state (`ew:party:<party_id>:lobby`, [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md)) và job `party-presence-sweeper` trong worker quét mỗi 5 s, đọc presence của module competition (`ew:party:presence:<user_id>`) — không phụ thuộc node ([F101](../../specs/101-party-lobby-session/spec.md)).

## 10. Rate limit & chống spam

| Hành động | Giới hạn | Nhãn |
|---|---|---|
| Gửi lời mời (`party_invite_rate`) | 10/phút, **50/ngày** mỗi học sinh | [Confirmed — Technical Architect 2026-11-16] |
| Lời mời tới cùng một người (`party_invite_repeat_rules`) | 1 pending; sau khi bị reject: chờ 5 phút; 3 lần reject/ngày → chặn mời người đó đến hết ngày | [Confirmed — Technical Architect 2026-11-16] |
| Lời mời tồn tại (`party_invite_ttl`) | Hết hạn sau 5 phút (`expires_at`) | [Confirmed] |
| Pending invitations tối đa nhận (`party_pending_invites_max`) | 20; vượt → lời mời cũ nhất hết hạn | [Confirmed — Technical Architect 2026-11-16] |
| Create/disband party (`party_create_rate`) | 10 lần/giờ | [Confirmed — Technical Architect 2026-11-16] |
| Kick (`party_kick_rate`) | 10/giờ mỗi leader | [Confirmed — Technical Architect 2026-11-16] |
| Lệnh lobby: ready, chọn activity (`party_lobby_cmd_rate`) | 5/giây | [Confirmed — Technical Architect 2026-11-16] |
| Hoạt động thiếu người (`party_activity_min_members_timeout`) | Số thành viên online < `min_members` quá 120 s → activity kết thúc `timeout` (§9) | [Confirmed — Technical Architect 2026-11-16] |

Giới hạn lời mời, tạo/giải tán và kick đếm theo cửa sổ trượt (1 phút, 1 giờ, 24 giờ) trên bản ghi PostgreSQL (`party_invitations`, `party_histories`) trong transaction của lệnh, dưới khóa theo người gửi (F100 — chính xác khi nhiều replica, không phụ thuộc Valkey); `party_lobby_cmd_rate` (F101) dùng Valkey sliding window (`ew:rl:...` — VALKEY_DESIGN). Vượt ngưỡng nhiều lần → ghi `audit` và tăng điểm nghi spam cho [MODERATION](MODERATION.md).

## 11. Audit log

Ghi `audit.audit_logs` (qua event) cho: `party.created`, `party.disbanded` (ai, lý do), `party.member_kicked`, `party.leader_transferred` (thủ công/tự động), invitation bị từ chối do policy (`PARTY_INVITE_NOT_ALLOWED`), vượt rate limit, `safety_moderator` can thiệp party (giải tán, gỡ thành viên khi xử lý case). Không ghi nội dung chat vào audit (chat có lưu trữ riêng).

## 12. Thành phần kỹ thuật

### 12.1. Golang (module competition, bounded context `party`)

| Module [Confirmed] | Trách nhiệm | Tiến trình |
|---|---|---|
| `PartyService` | Create, update settings, disband, transfer leader, get | api |
| `PartyMemberService` | Join, leave, kick, membership invariant R1/R10, eligibility | api |
| `PartyInvitationService` | Send/accept/reject/cancel/expire, rate limit, block check | api + worker (expire sweeper) |
| `PartyLobbyService` | Lobby state (ready, activity, mode), fan-out WS, version | realtime |
| `CooperativeSessionService` | 9 bước session, tạo `party_activity`/room/ticket, kết thúc, leader grace | api + realtime + worker |

Phụ thuộc (ADR-0036, ADR-0040): projection `ref_*` học sinh (trường, khối) và friend/block (nhận qua event từ module identity và module chat), `principals:resolve` của module identity, eligibility quest/boss của module game (facade, gọi trong tiến trình), `room.Allocator` và `matchmaking.Tickets` nội bộ, event `party.created`/`party.disbanded` để module chat tạo/đóng channel `party`, `audit.recorded`.

### 12.2. Valkey [Confirmed] (tên key cuối cùng theo VALKEY_DESIGN)

| Mục đích | Key đề xuất | Kiểu | TTL |
|---|---|---|---|
| Realtime Party State | `ew:party:<party_id>`, `ew:party:<party_id>:members` | hash (leader, status, `state_version`; thành viên → vai trò) | 24 h, tái tạo từ PG |
| Lobby: Ready, trạng thái thành viên, timer grace/dormant | `ew:party:<party_id>:lobby` (F101) | hash | 24 h |
| Party của học sinh | `ew:party:user:<user_id>` | string | 24 h |
| Invitation Cache | `ew:party:invite:<invitation_id>`, `ew:party:invites:user:<user_id>` | hash / sorted set theo `expires_at` | `party_invite_ttl` |
| Party đang theo dõi (sweeper), giới hạn lệnh lobby, khóa job | `ew:party:active`, `ew:rl:party_lobby:<user_id>`, `ew:lock:job:party-presence-sweeper` (F101) | zset / string | — |
| Realtime Event Distribution | Không có topic party: fan-out tới `ew:rt:user:<member_id>` của từng thành viên ([REALTIME_ARCHITECTURE §5](../06-architecture/REALTIME_ARCHITECTURE.md#5-routing-sự-kiện-và-room)) | pub/sub | — |

### 12.3. PostgreSQL — database `eduworld_competition`, schema `multiplayer` [Confirmed]

Mọi bảng có `tenant_id`, soft-delete và RLS theo tenant (ADR-0005, ADR-0028); cột chuẩn và DDL ở [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md).

| Bảng | Cột chính | Ràng buộc |
|---|---|---|
| `parties` | `party_id`, `tenant_id`, `school_id`, `club_id?` (chỉ khi tạo từ club), `name`, `leader_id`, `party_type`, `visibility`, `invitation_policy`, `max_members`, `grade_band`, `status`, `current_activity_id`, `current_game_session_ref`, `created_at`, `updated_at`, `disbanded_at` | RLS theo `school_id` |
| `party_members` | `party_id`, `student_id`, `role`, `joined_at`, `left_at`, `left_reason` | unique `(student_id) WHERE left_at IS NULL` |
| `party_invitations` | `invitation_id`, `party_id`, `inviter_id`, `invitee_id`, `status`, `created_at`, `expires_at`, `responded_at` | unique `(party_id, invitee_id) WHERE status='pending'` |
| `party_histories` | `party_id`, `event_type`, `actor_id`, `data`, `occurred_at` | append-only (Party History) |
| `party_activities` | `party_activity_id`, `party_id`, `activity_type` (`PartyActivityType`: `explore`/`quest`/`boss`/`learning_challenge`/`coop_challenge`/`team_battle`), `activity_ref`, `room_id`, `match_id?`, `status`, `started_at`, `ended_at`, `end_reason` | (Party Activity) |
| `session_memberships` | xem [MULTIPLAYER_GAMEPLAY §10](MULTIPLAYER_GAMEPLAY.md#10-session-membership) | (Session Membership) |
| `party_policies` | `scope_type`, `scope_id`, `rules` (JSONB) | |

### 12.4. Godot [Confirmed]

| Thành phần | Scene/script đề xuất | Ghi chú |
|---|---|---|
| Party UI (HUD mini) | `scenes/ui/hud/PartyHud.tscn` | Avatar nhỏ + trạng thái các member |
| Party Lobby | `scenes/social/PartyLobby.tscn`, `PartyMemberCard.tscn`, `ActivitySelector.tscn` | Danh sách member, ready, activity, start |
| Friend Invitation UI | `scenes/social/InviteFriendPicker.tscn`, `InvitationToast.tscn` | Chỉ hiện bạn bè đủ điều kiện; popup nhận lời mời có countdown |
| Member Status | `scripts/multiplayer/party_service.gd`, `party_store.gd` | State store `RefCounted`, nhận event WS, phát signal |
| Leader Controls | Trong party_lobby | Ẩn với member thường (backend vẫn kiểm tra) |
| Group Chat | Nhúng `chat_panel` channel `party` — luôn hiển thị trong lobby và khi chơi cùng (thu gọn được) | ADR-0020 mục 6 |
| Cooperative Gameplay | `scripts/multiplayer/coop_room_client.gd` | Hiển thị đóng góp của member, không hiển thị điểm học tập của người khác |

UI lớp 1–5: nút lớn, mời bằng avatar bạn bè, không cần gõ tên; lớp 6–9: thêm settings nâng cao.

Hiện thực client F102 (tên scene/script theo [GODOT_ARCHITECTURE §3](../08-game-client/GODOT_ARCHITECTURE.md)): `scenes/social/{PartyLobby,PartyMemberCard,InviteFriendPicker,ActivitySelector,InvitationToast}.tscn`, `scenes/ui/hud/PartyHud.tscn` (slot `PartyChip` của HUD), `scripts/multiplayer/{party_service,party_store}.gd` + `models/`. Bố cục và câu chữ: [STUDENT_GAME_UX_SOCIAL §5](../05-ux/STUDENT_GAME_UX_SOCIAL.md#5-party-lobby). Client không tự kiểm quy tắc §8 (loại party theo khối, cùng khối/trường/tenant, điều kiện hoạt động): chỉ hiển thị danh sách và lý do server trả. Chat nhóm nhúng chờ F119 (slot `EmbeddedChat` ẩn); `coop_room_client.gd` thuộc feature co-op Godot chưa có trong danh mục.

## 13. State machines

### 13.1. Party

```mermaid
stateDiagram-v2
  [*] --> open: create
  open --> in_activity: session.start (9 bước OK)
  in_activity --> open: session.ended
  open --> disbanded: disband / dormant timeout / còn 0 member
  in_activity --> disbanded: disband (activity kết thúc reason=disbanded)
  disbanded --> [*]
```

### 13.2. Invitation

```mermaid
stateDiagram-v2
  [*] --> pending: send (pass policy + rate limit)
  pending --> accepted: accept (re-check policy, còn chỗ, invitee chưa có party)
  pending --> rejected: reject
  pending --> expired: quá expires_at / party disbanded / invitee vào party khác
  pending --> cancelled: inviter/leader cancel / inviter bị kick
  accepted --> [*]
  rejected --> [*]
  expired --> [*]
  cancelled --> [*]
```

Accept khi invitee đang ở party khác: trả `PARTY_ALREADY_IN_PARTY` kèm lựa chọn "Rời nhóm hiện tại và tham gia" (client gọi leave rồi accept; không làm ngầm).

## 14. Acceptance criteria (Addendum §8) và test case

Mã `AC-PTY-NNN` do [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md) cấp (ADR-0082); bảng dưới chi tiết hóa từng AC của PRD thành test case, tài liệu này không cấp mã AC riêng.

| AC (PRD) | Tiêu chí [Confirmed] | Test case (tóm tắt) | Loại test |
|---|---|---|---|
| AC-PTY-001 | Học sinh A có thể tạo Party | A (không có party) `POST /parties` → 201, A là leader, `party.created` phát; gọi lại cùng `Idempotency-Key` → cùng party; A tạo party thứ hai → 409 `PARTY_ALREADY_IN_PARTY` | API, integration |
| AC-PTY-002 | A có thể mời B, C, D | A mời B, C, D (bạn bè cùng trường, cùng band) → 3 invitation `pending`, B/C/D nhận `party.invitation.received` qua WS hoặc notification nếu offline; mời E đã block A → 403 `PARTY_INVITE_NOT_ALLOWED` | API, WS |
| AC-PTY-002 | B, C, D chấp nhận lời mời | B, C, D accept → member count 4; accept trùng (retry) không tạo member trùng; với `max_members=3`, D accept → 409 `PARTY_FULL`; invitation quá 5 phút → `PARTY_INVITATION_EXPIRED` | API, concurrency test (B/C/D accept đồng thời) |
| AC-PTY-003 | Các thành viên nhìn thấy nhau trong Party Lobby | Sau accept, cả 4 client nhận `party.member.joined` và snapshot lobby với 4 avatar, trạng thái online trong ≤ 1 s (p95) | WS e2e, Godot integration |
| AC-PTY-004 | Leader có thể chọn hoạt động | A `party.activity.select` (quest co-op lớp 3) → mọi member nhận `party.activity.selected`, ready reset; B (không phải leader) select → `PARTY_NOT_LEADER` | WS |
| AC-PTY-005 | Mọi thành viên đủ điều kiện có thể tham gia cùng Game Session | Tất cả ready → A start → 9 bước, cả 4 nhận `party.session.started` cùng `room_id`, vào cùng `private_instance`; nếu D không đủ grade → 409 với lý do, không ai bị đưa vào session | API + WS e2e |
| AC-PTY-006 | Các thành viên có thể cùng thực hiện nhiệm vụ | Quest `shared_steps`: B kích hoạt cổng → bước tính cho cả nhóm; bước activity mỗi HS làm attempt riêng; quest hoàn thành khi điều kiện nhóm đạt | Integration (quest + exercise) |
| AC-PTY-007 | Một thành viên mất kết nối, những người còn lại tiếp tục nếu chế độ cho phép | Kill kết nối C: A, B, D tiếp tục, thấy C "đang kết nối lại"; C reconnect trong 120 s → về đúng instance, tiến độ giữ nguyên; activity `party_required` còn dưới `party_size.min` người → kết thúc `timeout` sau `party_activity_min_members_timeout` | Chaos/WS e2e |
| AC-PTY-008 | Leader mất kết nối có chính sách chuyển leader/phục hồi | Kill kết nối A: trong 60 s lệnh leader bị khóa; A quay lại trong 60 s → vẫn leader; nếu không → leader = member online gia nhập sớm nhất, `party.leader.transferred` reason `leader_disconnected`, audit ghi | WS e2e với clock giả lập |
| AC-PTY-009 | Kết quả học tập từng thành viên ghi nhận riêng | Sau quest: 4 `attempt` riêng với `student_id` riêng; điểm/mastery của B không đổi khi A trả lời; query DB xác nhận không có attempt nào có `student_id` khác người trả lời | Integration, data assertion |
| AC-PTY-010 | Không có thành viên trái phép tham gia Party riêng | E (không có invitation) `:join` party private → 403; E dùng invitation của B (sai invitee) → 403; học sinh trường khác dùng `party_id` → 404 (tenant isolation); E sửa client gửi `room.join` vào room party → `ROOM_NOT_MEMBER` | Security test, API, WS |

Kiểm thử bổ sung: rate limit lời mời (AC-PTY-011: lời mời thứ 11 trong 1 phút → 429 `PARTY_INVITE_RATE_LIMITED`), duplicate membership khi accept 2 invitation đồng thời từ 2 party khác nhau (AC-PTY-012: chỉ 1 thành công). Chat party always-on (AC-PTY-013: A tạo party → channel `party` được module chat tạo từ event `party.created` trong ≤ 1 s; B, C, D accept → tự là member và chat được; policy trường đặt `party.enabled=false` bị từ chối `CHAT_POLICY_ALWAYS_ON`). Party giải tán (thuộc AC-CHAT-010: channel `party` read-only ngay, bị purge sau `gameplay_chat_retention_days`, tin có report được giữ làm bằng chứng — ADR-0033).

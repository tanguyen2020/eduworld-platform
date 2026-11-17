# Feature Specification: F111 — Godot — club trong game, Event center, nhóm "Bạn cùng lớp"/"Vừa chơi cùng", nút "Tham gia bạn"

**Feature Branch**: `111-godot-clubs-events-social` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `godot/` · **Mốc**: M5 · **Phụ thuộc**: F077, F110, F120 (và F071 `SceneRouter.request_area`, F072 `QuestService`, F091/F092 `hub.instance.join`/`HubSync`, F095 bạn bè, F119 `ChatDock`, F123 deep link thông báo)
**Input**: Học sinh "Tham gia bạn" đang ở khu công khai; tìm bạn qua nhóm "Bạn cùng lớp" và "Vừa chơi cùng" (không tìm kiếm tự do); mở Event center xem sự kiện live-ops, mục tiêu cộng đồng, nhiệm vụ ngày/tuần và season track miễn phí, nhận thưởng mốc; xem, xin vào, nhận lời mời và tạo club theo chính sách `student_club_creation` của trường. Màn S11 (phần Tìm bạn, Club, Tham gia bạn) và S14 của [STUDENT_GAME_UX](../../docs/05-ux/STUDENT_GAME_UX.md). FR nguồn (phía client): FR-SOC-001…004, FR-MP-002, FR-MP-003, FR-EVT-001, FR-EVT-003…008, FR-GAM-001, FR-GAME-010.

## Clarifications

### Session 2026-11-16

- Q: "Tham gia bạn" gọi REST `joinFriendInWorld` hay lệnh WS? → A: Theo F091 (owner gán instance, module competition): `hub.instance.join` có `follow_user_id`; khác khu thì vào khu qua `SceneRouter.request_area` (F071 `enterWorldArea`) trước. Client **không** gọi `joinFriendInWorld`; endpoint này chờ F071/F091 bỏ hoặc đổi nghĩa (contracts §6).
- Q: Nguồn dữ liệu "Bạn cùng lớp", "Vừa chơi cùng"? → A: Chưa có contract. Đề xuất `listFriendSuggestions` (module chat module social, owner F095) — server lọc phạm vi, chặn, bạn hiện có. "Ở gần đây" và Mã bạn bè giữ nguyên của F110.
- Q: Client biết trường cho học sinh tạo club bằng gì? → A: Đề xuất thêm `PlayerBootstrap.policies.social` (owner F013/F015); chỉ để ẩn/hiện nút, `createClub` vẫn do server kiểm (F120-AC-17).
- Q: Lớp 1–3 có tab Club? → A: Theo [STUDENT_GAME_UX_SOCIAL §6](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md): `g1_3` chỉ xem club của mình (do nhà trường tạo) và nhận lời mời; không có club đề xuất, không tạo club. Khác biệt chỉ ở UX (ADR-0020).
- Q: "Mục tiêu tuần: học 4/7 ngày" lấy từ đâu? → A: Là tiến độ weekly quest do server tính ([QUEST_SYSTEM §2.4](../../docs/01-game-design/QUEST_SYSTEM.md#24-daily-và-weekly-quest)); client chỉ hiển thị `QuestProgress`, không tự đếm ngày.
- Q: Lớp 4–5 có tab Mùa? → A: [Proposed] Có (UX chỉ quy định `g1_3` rút gọn, `g6_9` đủ); `g4_5` dùng bố cục đủ 4 tab.

### Session 2026-11-17

- Q: Mã lỗi khi trường tắt tạo club (RB-626) và enum presence (RB-404)? → A: `createClub` khi tắt trả `403 PERMISSION_DENIED`; `SOCIAL_SCOPE_VIOLATION` chỉ cho vi phạm phạm vi (khác trường/tenant, chặn). Presence theo enum MULTIPLAYER_GAMEPLAY §5; ẩn trạng thái hiện là `offline`. Thông báo club dùng enum `kind` chung (RB-513).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — "Tham gia bạn" trong thế giới (Priority: P1)

An (3A) thấy Bình đang ở Vương quốc Số, bấm "Tham gia bạn" và xuất hiện cạnh Bình; nếu khu đầy hoặc Bình đang trong trận thì nhận lời giải thích thân thiện.

**Why this priority**: Cảm giác "có bạn trong thế giới" (SOCIAL_SYSTEM §2); FR-MP-002 ưu tiên bạn bè cùng instance.

**Independent Test**: Hai instance Godot `hs.an.3a`, `hs.binh.3a` (bạn bè). B vào `learning_zone.math`; A ở `main_hub` bấm "Tham gia bạn" → cùng `room_id`; B vào trận → nút của A xám kèm lý do.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F111-AC-01 | KHI học sinh bấm "Tham gia bạn", game client PHẢI dùng `area_id` mới nhất từ `presence.friend.updated`: cùng khu hiện tại → gửi `hub.instance.join` (`area_id`, `follow_user_id`); khác khu → `SceneRouter.request_area(area_id, {follow_user_id})` (gọi `enterWorldArea`, tải zone) rồi `hub.instance.join` có `follow_user_id`; PHẢI không gọi `joinFriendInWorld` | FR-SOC-001 · FR-MP-002 · [SOCIAL_SYSTEM §4.2](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) · AC-SOC-005 | — |
| F111-AC-02 | game client PHẢI chỉ bật "Tham gia bạn" khi bạn có trạng thái `online`/`away` (không `in_match`), có `area_id`, `activity` khác `exam`, và bản thân không ở hoạt động chặn (`AppState.blocking_activity`, bài kiểm tra) hay mất mạng; trường hợp khác PHẢI hiện nút xám kèm lý do bằng chữ; điều kiện ở client chỉ là gợi ý, server quyết định | FR-MP-003 · [STUDENT_GAME_UX_SOCIAL §6](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) · AC-MP-010 | — |
| F111-AC-03 | KHI nhận `hub.instance.assigned` cho lệnh có `follow_user_id`, game client PHẢI đặt nhân vật tại `spawn`, dựng người chơi cùng instance qua `HubSync` (F092) và hiện toast "Em đã đến chỗ {nickname}" | FR-MP-002 · [MULTIPLAYER_GAMEPLAY §6](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | — |
| F111-AC-04 | NẾU nhận `system.command.failed` `ROOM_FULL`, THÌ game client PHẢI gửi lại `hub.instance.join` không có `follow_user_id` cho khu đó và hiện "Khu vực của bạn đã đầy"; NẾU `PERMISSION_DENIED`/`NOT_FOUND`, THÌ PHẢI vào instance thường và hiện "Chưa thể tham gia bạn lúc này"; PHẢI không hiển thị vị trí, instance hay lý do chi tiết | AC-SOC-005 · AC-MP-010 · AGENTS R16 | — |
| F111-AC-05 | NẾU `enterWorldArea` trả `403 AREA_LOCKED`, THÌ game client PHẢI ở lại khu hiện tại, hiện điều kiện mở khóa theo F071 và không gửi `hub.instance.join`; TRONG KHI lệnh tham gia đang chờ, PHẢI khóa nút để không gửi trùng | FR-MP-002 · [WORLD_DESIGN §6](../../docs/01-game-design/WORLD_DESIGN.md) | — |

---

### User Story 2 — Nhóm "Bạn cùng lớp" và "Vừa chơi cùng" (Priority: P2)

Học sinh mở tab Tìm bạn, thấy bạn cùng lớp và bạn vừa cùng party/trận/club, bấm "Kết bạn" — không có ô tìm tên.

**Why this priority**: FR-SOC-002 chỉ cho tìm bạn trong phạm vi; cần US1 không bắt buộc.

**Independent Test**: `hs.an.3a` mở Tìm bạn: nhóm "Bạn cùng lớp" có `hs.binh.3a` (chưa kết bạn), không có `hs.chi.3b`; sau một party với `hs.chi.3b`, nhóm "Vừa chơi cùng" có `hs.chi.3b` kèm biểu tượng party.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F111-AC-06 | KHI mở tab "Tìm bạn", game client PHẢI hiển thị nhóm "Ở gần đây" (F110), "Bạn cùng lớp" (`listFriendSuggestions` `source = classmates`), "Vừa chơi cùng" (`source = recently_played`, ngữ cảnh party/trận/club bằng biểu tượng + chữ) và Mã bạn bè (F110); PHẢI không có ô tìm kiếm tự do theo tên/ID | FR-SOC-001 · FR-SOC-002 · AC-SOC-002 · AC-UX-105 | TC-SOC-001 |
| F111-AC-07 | Mỗi gợi ý PHẢI chỉ hiện nickname, avatar 2D, level; PHẢI không hiện tên thật, trường, lớp; game client PHẢI ẩn thêm người đã là bạn, có lời mời đang chờ hoặc trong danh sách chặn của mình; "Kết bạn" PHẢI gọi `SocialService.send_request` (F110, `Idempotency-Key`) và map `SOCIAL_SCOPE_VIOLATION`, `FRIEND_LIMIT_REACHED`, `RATE_LIMITED` như F110 | FR-SOC-002 · FR-SOC-003 · AGENTS R16 · ADR-0054 | TC-SOC-002 |
| F111-AC-08 | NẾU một nhóm rỗng, THÌ game client PHẢI hiện trạng thái rỗng kèm gợi ý dùng Mã bạn bè; NẾU `listFriendSuggestions` lỗi hoặc chưa có, THÌ PHẢI ẩn nhóm đó và giữ "Ở gần đây", Mã bạn bè; `g1_3` PHẢI chọn bằng chạm avatar lớn | FR-SOC-001 · FR-GAME-010 | — |

---

### User Story 3 — Event center: sự kiện và mục tiêu cộng đồng (Priority: P1)

Học sinh mở Event center, thấy "Lễ hội Trăng Rằm · còn 5 ngày", thanh "Mục tiêu cả trường", nhận quest sự kiện và nhận thưởng mốc — không bị thúc ép.

**Why this priority**: Đường hiển thị duy nhất cho live-ops F077 (FR-EVT-001, FR-EVT-004).

**Independent Test**: Seed một sự kiện `world_festival` có community goal cho TH Demo A: `hs.an.3a` thấy sự kiện, tham gia, nhận mốc; `hs.dung.7a` (tenant khác) không thấy; dừng khẩn cấp → thẻ đổi "Tạm dừng" ≤ 5 s.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F111-AC-09 | KHI mở Event center (biểu tượng lịch trên HUD, bảng tin Quảng trường Tia Sáng, deep link thông báo), game client PHẢI gọi `listGameEvents` (`status = live,upcoming`) và hiện `EventCard`: tên theo ngôn ngữ (thiếu `en` dùng `vi`), banner, thời gian còn lại dạng số ngày, nhãn "Sắp diễn ra" cho `scheduled`; PHẢI không đếm giờ/phút/giây, không nhấp nháy | FR-EVT-001 · FR-EVT-006 · FR-EVT-007 · [EVENT_SEASON_SYSTEM §9](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) | TC-EVT-001 |
| F111-AC-10 | KHI mở chi tiết, game client PHẢI gọi `getGameEvent` và hiện mô tả, quest sự kiện (nút "Đi" giao `QuestService` F072 dẫn đường), mốc thưởng, `daily_points_remaining` bằng lời trung tính; "Tham gia" PHẢI gọi `joinGameEvent` (`Idempotency-Key` giữ khi thử lại); `404 EVENT_NOT_AVAILABLE`/`409 EVENT_ENDED` PHẢI đóng chi tiết hoặc làm mới với thông điệp thân thiện | FR-EVT-001 · AC-EVT-001 | TC-EVT-001 |
| F111-AC-11 | KHI học sinh bấm "Nhận" ở mốc `reached` chưa `claimed`, game client PHẢI gọi `claimEventMilestone` với `Idempotency-Key` giữ nguyên khi thử lại và chỉ đánh dấu đã nhận khi server trả `RewardClaimResult`, hiển thị `rewards` từ server; `409 EVENT_MILESTONE_NOT_REACHED` PHẢI làm mới chi tiết | FR-EVT-003 · ADR-0049 · AC-EVT-012 | TC-EVT-012 |
| F111-AC-12 | KHI nhận `event.status.changed`, game client PHẢI làm mới danh sách: `suspended` → ẩn thành phần sự kiện, hiện "Sự kiện tạm dừng"; `ended` → chỉ cho nhận mốc đã đạt tới `claim_deadline_at` và ghi "Phần thưởng chưa nhận sẽ được gửi tự động"; `cancelled` → gỡ thẻ; PHẢI không cắt lượt chơi đang dở | FR-EVT-003 · AC-EVT-003 | TC-EVT-003 |
| F111-AC-13 | KHI chi tiết có `community_goal` hoặc nhận `event.community_goal.updated`, game client PHẢI hiện `CommunityGoalBar` (tổng/mục tiêu, mốc 25/50/75/100, "Em đã góp sức" theo `my_contributed`); PHẢI không hiện xếp hạng cá nhân; `by_class` chỉ hiện khi server trả và khối không phải `g1_3`, sắp theo tên lớp, không số thứ hạng | FR-EVT-004 · AC-EVT-006 · AGENTS R19 | TC-EVT-006 |
| F111-AC-14 | KHI nhận `event.progress.updated`, game client PHẢI thay điểm sự kiện, `daily_points_remaining` bằng giá trị server và đánh dấu mốc vừa đạt bằng huy hiệu tĩnh (không popup chặn); PHẢI không tự cộng điểm | FR-EVT-001 · ADR-0016 | — |

---

### User Story 4 — Hôm nay, Tuần này, Mùa (Priority: P2)

Học sinh xem nhiệm vụ ngày/tuần, bấm "Đi"; xem season track miễn phí và nhận thưởng cấp; xong việc ngày thì được gợi ý nghỉ.

**Why this priority**: FR-EVT-005, FR-EVT-007; cần US3 khung màn.

**Independent Test**: `hs.dung.7a` mở tab Mùa: thấy cấp/điểm, nhận thưởng cấp 1 (gửi lại cùng key → một lần), không có track trả phí; `hs.an.3a` (`g1_3`) chỉ thấy tab Hôm nay và Sự kiện.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F111-AC-15 | KHI mở tab "Hôm nay"/"Tuần này", game client PHẢI lấy quest `daily`/`weekly` qua `QuestService` (F072, `listQuests` theo `quest_type`), hiện trạng thái, tiến độ theo `QuestProgress` server, giờ đặt lại từ `available_until`; "Đi" PHẢI giao F072 nhận quest/dẫn đường; KHI mọi nhiệm vụ ngày xong, PHẢI hiện "Tuyệt! Em đã xong nhiệm vụ hôm nay. Nghỉ ngơi chút nhé", không gợi ý chơi thêm | FR-EVT-007 · AC-UX-111 · [QUEST_SYSTEM §2.4](../../docs/01-game-design/QUEST_SYSTEM.md#24-daily-và-weekly-quest) | TC-EVT-009 |
| F111-AC-16 | KHI mở tab "Mùa", game client PHẢI gọi `getSeasonTrack` và hiện dải cấp ảo hóa (chỉ tạo mốc trong khung nhìn), điểm, cấp, `daily_points_remaining`, cấp mốc nổi bật, mục "Mùa trước" từ `listSeasonTrackHistory`; PHẢI không có track trả phí, nút mua cấp hay giá tiền; `404` PHẢI hiện "Mùa mới sắp bắt đầu" | FR-EVT-005 · FR-GAM-001 · ADR-0049 | TC-EVT-007 |
| F111-AC-17 | KHI bấm "Nhận" ở cấp đã đạt, game client PHẢI gọi `claimSeasonTier` (`Idempotency-Key` giữ khi thử lại), hiện `rewards` server trả; `409 SEASON_TIER_NOT_REACHED` PHẢI làm mới; KHI nhận `season.track.updated`, PHẢI cập nhật điểm/cấp; màn PHẢI ghi "Phần thưởng chưa nhận sẽ tự gửi khi hết mùa" | FR-EVT-005 · AC-EVT-007 | TC-EVT-007 |
| F111-AC-18 | Ở NƠI khối của học sinh là `g1_3`, game client PHẢI chỉ hiện tab "Hôm nay" và "Sự kiện" với hình minh họa và nút "Đi" lớn; khối khác PHẢI có đủ 4 tab | FR-GAME-010 · [STUDENT_GAME_UX_SOCIAL §9](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | — |

---

### User Story 5 — Club: xem, xin vào, nhận lời mời, tạo (Priority: P2)

Học sinh lớp 7 xem club của mình và club đề xuất trong trường, xin vào CLB Toán, nhận lời mời, tạo club "Nhà thám hiểm" khi trường cho phép.

**Why this priority**: FR-SOC-004 phía client; cần F120 backend.

**Independent Test**: Trường đặt `student_club_creation = requires_approval`: `hs.dung.7a` tạo club → "Đang chờ nhà trường duyệt"; `ad.truong.b` duyệt ở portal → club `active`, có nút Chat club; trường `disabled` → không có nút "Tạo club".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F111-AC-19 | KHI mở tab Club, game client PHẢI gọi `listClubs` `scope = mine` và (trừ `g1_3`) `scope = discoverable`, cùng lời mời đang chờ (`listMyClubInvitations`); thẻ club PHẢI có tên, biểu tượng, loại, `member_count`/`max_members`, trạng thái (`pending_approval` → "Đang chờ nhà trường duyệt") | FR-SOC-004 · [SOCIAL_SYSTEM §6](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | TC-SOC-004 |
| F111-AC-20 | KHI mở trang club, game client PHẢI gọi `getClub`, `listClubMembers` (nickname, avatar, vai trò), `listClubGoals` (thanh tiến độ tập thể + "Em đã góp {my_contribution}"), `listClubEvents`; PHẢI không hiện điểm, mastery, đóng góp từng người khác, tên thật hay trường của thành viên | FR-SOC-004 · AC-SOC-008 · AGENTS R16 | TC-SOC-006 |
| F111-AC-21 | KHI học sinh bấm "Xin vào", game client PHẢI gọi `requestJoinClub` (`Idempotency-Key`) và hiện "Đang chờ duyệt"; lời mời PHẢI chấp nhận bằng `acceptClubInvitation` (`Idempotency-Key`) hoặc từ chối bằng `declineClubInvitation`; `409 CLUB_FULL`, `409 CLUB_LIMIT_REACHED`, `409 CLUB_NOT_APPROVED`, `403 SOCIAL_SCOPE_VIOLATION` PHẢI hiện thông điệp `ui.error.<CODE>` thân thiện | FR-SOC-004 · FR-SOC-002 · AC-SOC-007 · ADR-0054 | TC-SOC-005 |
| F111-AC-22 | game client PHẢI chỉ hiện "Tạo club" khi `policies.social.student_club_creation` khác `disabled` và `club_creation_eligible = true`; form PHẢI chỉ cho loại `interest_group`/`competition_team`, tên 2–64 ký tự, mô tả ≤ 500 ký tự (bộ đếm, chặn vượt), biểu tượng từ thư viện; gửi `createClub` (`Idempotency-Key`); `pending_approval` → "Club đang chờ nhà trường duyệt", `active` → mở trang club | FR-SOC-004 · AC-SOC-006 · ADR-0041 | TC-SOC-004 |
| F111-AC-23 | NẾU `createClub` trả `451 CHAT_CONTENT_BLOCKED`, THÌ game client PHẢI hiện thông điệp kiểm duyệt chung (không nêu từ khớp, không echo), giữ form để sửa; NẾU `403 PERMISSION_DENIED` (trường tắt tạo club), THÌ PHẢI hiện "Trường em chưa mở tạo club" và ẩn nút; `409 CLUB_LIMIT_REACHED` PHẢI hiện giới hạn bằng lời | AGENTS R15 · [MODERATION §2](../../docs/03-social-multiplayer/MODERATION.md) · AC-SOC-006 | TC-SOC-004 |
| F111-AC-24 | KHI học sinh bấm "Rời club" và xác nhận một bước, game client PHẢI gọi `leaveClub`; "Chat club" PHẢI mở `ChatDock` (F119) với `chat_channel_id` khi club `active`; club `locked`/`archived` PHẢI chỉ xem, không có nút xin vào/mời/chat gửi | FR-SOC-004 · ADR-0050 | TC-SOC-008 |

---

### User Story 6 — Owner/officer duyệt yêu cầu và mời bạn (Priority: P3)

Học sinh là owner/officer club duyệt yêu cầu xin vào và mời bạn bè.

**Why this priority**: Club do học sinh tạo cần người duyệt (SOCIAL_SYSTEM §6.2); ít người dùng hơn US5.

**Independent Test**: `hs.dung.7a` (owner club `active`) thấy tab Yêu cầu, chấp nhận một yêu cầu; mời một bạn đã chặn mình → kết quả chung không lộ việc bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F111-AC-25 | TRONG KHI `my_role` là `owner`/`officer`, game client PHẢI hiện tab "Yêu cầu" (`listClubJoinRequests`) với "Chấp nhận" (`acceptClubJoinRequest`, `Idempotency-Key`) và "Từ chối" (`rejectClubJoinRequest`); vai trò ở client chỉ quyết định hiển thị, `403 PERMISSION_DENIED` PHẢI ẩn nút và làm mới | FR-SOC-004 · [SOCIAL_SYSTEM §6.2](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | TC-SOC-005 |
| F111-AC-26 | KHI owner/officer mời, game client PHẢI chỉ cho chọn từ danh sách bạn bè (không nhập tên), gửi `inviteToClub` (`invitee_ids` ≤ 50, `Idempotency-Key`); người không mời được (`SOCIAL_SCOPE_VIOLATION`, đã chặn) PHẢI hiện chung "Chưa mời được", không nêu lý do chặn | FR-SOC-003 · AC-SOC-004 · AGENTS R16 | TC-SOC-006 |

---

### User Story 7 — Song ngữ, trợ năng, token, an toàn, phân quyền (Priority: P2)

**Why this priority**: Bắt buộc theo constitution (V, VI, VIII); kiểm sau luồng chính.

**Independent Test**: Pseudo-locale `xx` cho `EventCenterScreen`, `ClubPage`, `FindFriendsTab`; `lint_theme_overrides.gd`; đăng nhập game bằng `ad.truong.a`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F111-AC-27 | Mọi chuỗi UI PHẢI qua khóa `ui.events.*`, `ui.season.*`, `ui.club.*`, `ui.friends.*`, `ui.error.<CODE>`; dữ liệu `LocalizedName`/`TenantLocalizedName` chọn theo ngôn ngữ; ngày giờ theo ICT, định dạng theo locale; bố cục PHẢI chịu chuỗi dài hơn 35% (`ui.events.all_done`, `ui.friends.join_unavailable.*`) | NFR-I18N-001 · NFR-I18N-003 · FR-EVT-008 · ADR-0056 | TC-I18N-004 |
| F111-AC-28 | game client PHẢI lấy style từ Theme sinh từ `design/game-tokens` theo density (vùng chạm ≥ 48 dp, `g1_3` ≥ 64 dp); trạng thái (presence, chờ duyệt, đã nhận) và thời gian còn lại PHẢI có chữ, không chỉ màu | NFR-A11Y-001 · ADR-0047 · AGENTS R32 | TC-UI-001, TC-UI-003 |
| F111-AC-29 | Event center, club và tìm bạn PHẢI không có: đếm ngược giây, nhấp nháy, streak/chuỗi ngày, câu "chỉ còn…", popup thúc chơi, thông báo ngoài game; chấm báo nội dung mới trên HUD PHẢI tĩnh và biến mất khi đã mở | AGENTS R19 · NFR-SAFE-003 · FR-EVT-007 | TC-EVT-009 |
| F111-AC-30 | game client PHẢI chỉ hiển thị nickname/avatar/level/vai trò club của người khác, giữ store club/sự kiện/gợi ý chỉ trong RAM, xóa khi đăng xuất hoặc `session.session.replaced`; script trong `scenes/**` PHẢI chỉ gọi method public của service | AGENTS R1 · AGENTS R16 · ADR-0064 | — |
| F111-AC-31 | Mọi request PHẢI qua `ApiClient` (gắn `X-Tenant-Id` của phiên); NẾU nhận `403 GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH`, THÌ PHẢI theo luồng F061; NẾU nhận `404` cho sự kiện/club/lời mời, THÌ PHẢI đóng màn liên quan và làm mới, không hiển thị chi tiết; `422 VALIDATION_FAILED`/`428` PHẢI ghi telemetry `client_bug` không PII | AGENTS R23 · ADR-0065 · ADR-0041 | TC-AUTH-014 |
| F111-AC-32 | Event center, trang club và tab Tìm bạn PHẢI giữ ≥ 30 FPS trên điện thoại tầm trung preset Trung bình: danh sách ảo hóa, banner/ảnh thưởng là thumbnail cache, thế giới nền giảm tần số cập nhật khi mở màn phụ | NFR-COMP-001 · [STUDENT_GAME_UX_SOCIAL §2](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) | — |

### Edge Cases

- NẾU bạn đổi khu trong lúc tải zone để tham gia, THÌ server quyết định theo `follow_user_id`; thất bại xử lý theo F111-AC-04.
- NẾU bạn bật "Ẩn trạng thái" hoặc offline, THÌ không có `area_id`, nút "Tham gia bạn" xám (F111-AC-02).
- NẾU học sinh đang trong party, THÌ instance theo party (MULTIPLAYER_GAMEPLAY §6); client chỉ hiển thị kết quả server.
- NẾU sự kiện chuyển `ended` khi đang mở chi tiết, THÌ áp F111-AC-12 ngay, không mất mốc đã đạt.
- NẾU nhận cùng `event.progress.updated` hai lần, THÌ thay giá trị (không cộng dồn).
- NẾU `ClubType` server còn `study_group`, THÌ client không hiển thị club loại đó trong Club (study group thuộc module learning, ADR-0084).
- NẾU club bị khóa khi đang mở trang, THÌ lần làm mới kế hiện chế độ chỉ xem (F111-AC-24).
- NẾU notification deep link trỏ tới club/sự kiện đã không còn, THÌ áp F111-AC-31 (`404`).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-SOC-001 | "Tham gia bạn", nhóm gợi ý kết bạn | [SOCIAL_SYSTEM §4](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |
| FR-SOC-002 | Tìm bạn chỉ trong phạm vi (không tìm tự do); club cùng trường/liên trường theo tenant | SOCIAL_SYSTEM §4.1, §7 · [ADR-0054](../../docs/adr/ADR-0054-tuong-tac-xa-hoi-lien-truong-cross-school-social-interaction.md) |
| FR-SOC-003 | Ẩn người đã chặn khỏi gợi ý, lời mời club | SOCIAL_SYSTEM §5 |
| FR-SOC-004 | Club: xem, xin vào, lời mời, tạo theo `student_club_creation`, duyệt/mời của owner/officer | SOCIAL_SYSTEM §6 |
| FR-MP-002 | Ưu tiên cùng instance với bạn qua "Tham gia bạn" | [MULTIPLAYER_GAMEPLAY §6](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) |
| FR-MP-003 (phần hiển thị) | Dùng presence của bạn để bật/tắt "Tham gia bạn" | MULTIPLAYER_GAMEPLAY §5 |
| FR-EVT-001 | Danh sách/chi tiết sự kiện, quest sự kiện, điểm sự kiện | [EVENT_SEASON_SYSTEM §3–§4](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) |
| FR-EVT-003 | Hiển thị vòng đời (tạm dừng, kết thúc, thời hạn nhận thưởng) | EVENT_SEASON_SYSTEM §5 |
| FR-EVT-004 | Thanh mục tiêu cộng đồng, không xếp hạng cá nhân | EVENT_SEASON_SYSTEM §6 |
| FR-EVT-005 | Season track miễn phí, nhận thưởng cấp, lịch sử mùa | EVENT_SEASON_SYSTEM §7 |
| FR-EVT-006 (phần hiển thị) | Sự kiện trường hiện như sự kiện khác, tên `vi` khi thiếu `en` | EVENT_SEASON_SYSTEM §10–§11 |
| FR-EVT-007 | Không streak, không ép online, gợi ý nghỉ | EVENT_SEASON_SYSTEM §9 |
| FR-EVT-008 (phần hiển thị) | Song ngữ nội dung sự kiện | EVENT_SEASON_SYSTEM §11 |
| FR-GAM-001 (phần Season Reward) | Nhận thưởng mùa trong game | [PROGRESSION_SYSTEM](../../docs/01-game-design/PROGRESSION_SYSTEM.md) |
| FR-GAME-010 | Bố cục theo khối (`g1_3` rút gọn) | [STUDENT_GAME_UX_SOCIAL §6, §9](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) |

NFR: NFR-I18N-001, NFR-I18N-003, NFR-A11Y-001, NFR-SAFE-003, NFR-COMP-001.

### Key Entities

- **FriendSuggestion**: thẻ công khai người chơi + nguồn (`classmates`/`recently_played`) + ngữ cảnh chơi cùng.
- **JoinFriendIntent**: ý định tham gia (bạn, khu đích, trạng thái chờ/thành công/thất bại).
- **LiveEvent / EventMilestone / CommunityGoal**: sự kiện đang/sắp diễn ra trong phạm vi, mốc thưởng, tiến độ chung.
- **SeasonTrack / SeasonTier**: track mùa miễn phí, cấp, thưởng, đã nhận.
- **Club / ClubMember / ClubGoal / ClubEvent / ClubInvitation / ClubJoinRequest**: theo [SOCIAL_SYSTEM §6.5](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md).
- **SocialPolicy**: chính sách xã hội hiệu lực rút gọn cho client (tạo club, liên trường) — chỉ để hiển thị.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `student_club_creation` | Ẩn/hiện "Tạo club" (giá trị từ bootstrap) |
| `club_max_members` / `clubs_per_student_max` | Chỉ hiển thị (giá trị trong `Club.max_members` và lỗi server) |
| `hub_instance_capacity` | Chỉ để hiểu `ROOM_FULL` (server quyết định) |
| `presence_ttl` | Trạng thái mặc định offline khi không có presence |
| `event_upcoming_visible_days`, `event_claim_grace_hours` | Chỉ hiển thị (server lọc, `claim_deadline_at`) |
| `event_points_daily_cap`, `season_points_daily_cap` | Chỉ hiển thị phần còn lại do server trả |
| `community_goal_progress_flush_seconds` | Nhịp cập nhật thanh mục tiêu (do server đẩy) |
| `season_track_tiers` / `season_track_points_per_tier`, `season_unclaimed_rewards` | Bố cục dải cấp; thông điệp tự cấp |
| `quest_reset_time` | Thông điệp giờ đặt lại (giá trị từ `available_until`) |
| `supported_locales`, `client_reconnect_backoff` | Ngôn ngữ UI; kết nối lại WS |
| `target_fps`, `quality_presets` | Ngân sách màn phụ |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không business logic trong UI; không gửi điểm/tiến độ/đóng góp từ client | AGENTS R1, R5 · ADR-0016 |
| Không FOMO/dark pattern; season track miễn phí | AGENTS R19 · NFR-SAFE-003 · ADR-0049 |
| Chỉ nickname + avatar; không lộ ai chặn mình, không lộ vị trí khi từ chối | AGENTS R16 · ADR-0054 |
| Văn bản club qua pipeline kiểm duyệt server, không hiển thị từ khớp | AGENTS R15 · ADR-0020 |
| Theme/token; song ngữ | AGENTS R32 · ADR-0047 · ADR-0056 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: ≥ 95% lượt "Tham gia bạn" hợp lệ (bạn ở khu công khai, còn chỗ, cùng partition) đặt học sinh cùng instance với bạn trong ≤ 5 giây (cùng khu ≤ 2 giây).
- **SC-002**: 0 màn trong bộ scene test hiển thị tên thật, trường, lớp, điểm học tập của người chơi khác hoặc xếp hạng cá nhân trong sự kiện/club.
- **SC-003**: 0 phần tử đếm ngược theo giây, streak hay track trả phí trong Event center (kiểm bằng quét scene và pseudo-locale).
- **SC-004**: Học sinh lớp 4–9 tạo được club (khi trường cho phép) trong ≤ 5 thao tác; nhận thưởng mốc/cấp trong ≤ 2 thao tác từ HUD.
- **SC-005**: Event center, trang club ≥ 30 FPS trên thiết bị Android tầm trung tham chiếu, preset Trung bình.

## Ngoài phạm vi

- Backend: sự kiện, community goal, season track → F077; club, channel club → F120; bạn bè, gợi ý kết bạn, presence → F095, F091; gán instance theo `follow_user_id` → F091.
- Danh sách bạn, lời mời, mã bạn bè, chặn/tắt tiếng, "Ở gần đây" → F110; mời party → F102; "Thách đấu" → F109; DM và tab channel bền vững (khu vực, lớp, DM) → F119/F120.
- Owner sửa thông tin club, bổ nhiệm officer, gỡ thành viên, tạo mục tiêu/sự kiện club, lưu trữ club trong game → chưa có feature (nhân sự trường làm ở portal, F122); duyệt club → F122.
- Boss, shop, bộ sưu tập của sự kiện → F110, F069 (Event center chỉ dẫn tới quest); trang trí thế giới theo sự kiện → F071/F077; sự kiện trường ở portal → F081.
- Hộp thông báo, toast lời mời club → F123 (F111 chỉ đăng ký deep link); study group LMS → F040.

## Assumptions

- Trước khi F077, F095, F120 `Done`, client chạy trên Prism mock sinh từ `specs/` (profile `mock`, nhãn MOCK — R17); feature chỉ `Done` với backend thật.
- Contract "Cần thêm/Cần sửa" ([contracts §6](contracts/README.md)) có trước US tương ứng: `listFriendSuggestions` (US2), `listMyClubInvitations` và `PlayerBootstrap.policies.social` (US5), `hub.instance.join.follow_user_id` (US1).
- [Proposed] Cửa sổ "Vừa chơi cùng" là tham số server mới (đề xuất `friend_suggestion_recent_window`, owner SOCIAL_SYSTEM) — client không biết giá trị.
- [Proposed] Tên thật của bạn cùng lớp (SOCIAL_SYSTEM §4.1) không hiển thị vì `PlayerCard` chưa có trường; F111 chỉ dùng nickname.
- `ChatDock` (F119) mở được channel `team` cố định của club theo `channel_id`; khi chưa hỗ trợ, nút "Chat club" ẩn.

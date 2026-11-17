# Tasks: F102 — Giao diện party trong Godot: panel, lời mời, lobby, ready

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Thêm input action `open_party` (phím `P`) — file: `src/game/project.godot`
- [ ] T002 [P] Khóa dịch `ui.party.*` (title, alone, invite, ready, start, not_eligible, leader_reconnecting, new_leader, leave_confirm, disband_confirm, kick_confirm, transfer_confirm, switch_party_confirm, activity_changed, disbanded, kicked) và `ui.party.error.<CODE>` — file: `src/game/locale/vi.po`, `src/game/locale/en.po`
- [ ] T003 [P] Fixture từ example OpenAPI/AsyncAPI (party 1–4 thành viên, lời mời, snapshot, message WS mọi loại, eligible activities đủ/thiếu điều kiện, Problem) — file: `src/game/tests/fixtures/multiplayer/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Model typed `PartyModel`, `PartyMemberModel`, `InvitationModel`, `EligibleActivityModel` (`from_dict` bỏ trường lạ) + test parse mọi fixture — file: `src/game/scripts/multiplayer/models/`, `src/game/tests/unit/test_party_models.gd` — kiểm: `make godot-test`
- [ ] T005 `PartyStore` (`apply_snapshot`, `apply_message`, `state_version`, signal typed) — file: `src/game/scripts/multiplayer/party_store.gd`
- [ ] T006 Khung `PartyService` (RefCounted, `ApiClient`/`RealtimeClient` qua constructor, map Problem → khóa dịch, `Idempotency-Key` theo intent), đăng ký trong `ServiceLocator`, `AppState.party`, signal `EventBus.party_invitation_received` — file: `src/game/scripts/multiplayer/party_service.gd`, `src/game/scripts/core/app_state.gd`, `src/game/scripts/core/event_bus.gd`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tạo nhóm, mời bạn, nhận lời mời (Priority: P1) 🎯 MVP

**Goal**: Tạo party và mời/nhận lời mời trên client.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T007 [P] [US1] Unit test F102-AC-01, F102-AC-04, F102-AC-05 với `FakeApiClient`: tạo party giữ key khi retry; `PARTY_ALREADY_IN_PARTY` chỉ leave+accept khi xác nhận; Problem → khóa dịch, không mã thô — file: `src/game/tests/unit/test_party_service.gd` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Scene test F102-AC-02: picker chỉ hiện danh sách server trả, `g1_3` không có ô nhập — file: `src/game/tests/scene/test_invite_friend_picker.gd`
- [ ] T009 [P] [US1] Scene test F102-AC-03: toast đếm ngược theo `expires_at` (Clock giả), tự ẩn 10 s, mở lại từ chuông, hết hạn biến mất; không bật khi đang ở activity player — file: `src/game/tests/scene/test_invitation_toast.gd`

### Implementation cho User Story 1

- [ ] T010 [P] [US1] `InviteFriendPicker` (nguồn `listFriends`) — file: `src/game/scenes/social/InviteFriendPicker.tscn`, `src/game/scenes/social/invite_friend_picker.gd`
- [ ] T011 [P] [US1] `InvitationToast` + danh sách lời mời ở chuông — file: `src/game/scenes/social/InvitationToast.tscn`, `src/game/scenes/social/invitation_toast.gd`
- [ ] T012 [US1] `PartyService`: create, invite, list/accept/reject/cancel invitation, leave — file: `src/game/scripts/multiplayer/party_service.gd`

## Phase 4: User Story 2 — Party Lobby: thấy nhau, sẵn sàng, chọn hoạt động, bắt đầu (Priority: P1)

### Tests cho User Story 2

- [ ] T013 [P] [US2] Unit test F102-AC-07, F102-AC-08: áp message theo `state_version`, lệch → gọi `getParty`; Ready gửi WS, fallback REST khi WS đóng, hiển thị theo xác nhận — file: `src/game/tests/unit/test_party_store.gd` — phải FAIL trước khi code
- [ ] T014 [P] [US2] Scene test F102-AC-06, F102-AC-09: thẻ thành viên 2D, chỉ vẽ lại thẻ đổi; điều khiển leader chỉ ở leader; nút Bắt đầu xám kèm lý do — file: `src/game/tests/scene/test_party_lobby.gd`
- [ ] T015 [P] [US2] Scene test F102-AC-10: hoạt động thiếu điều kiện có cảnh báo, không chọn được; thẻ lớn `g1_3`, dropdown `g6_9` — file: `src/game/tests/scene/test_activity_selector.gd`
- [ ] T016 [P] [US2] Scene test F102-AC-11, F102-AC-12: `party.session.started` → đếm ngược + `SceneRouter.go_to(scene_ref)`; `party.session.ended` → lobby, không điểm của bạn; xác nhận rời/giải tán/kick/chuyển; thu nhỏ → `PartyHud` — file: `src/game/tests/scene/test_party_session_transition.gd`

### Implementation cho User Story 2

- [ ] T017 [P] [US2] `PartyMemberCard` (thumbnail cache, presence icon + chữ, Ready, vương miện, menu leader) — file: `src/game/scenes/social/PartyMemberCard.tscn`, `src/game/scenes/social/party_member_card.gd`
- [ ] T018 [P] [US2] `ActivitySelector` — file: `src/game/scenes/social/ActivitySelector.tscn`, `src/game/scenes/social/activity_selector.gd`
- [ ] T019 [US2] `PartyLobby` (bố cục PC hai cột / mobile, slot `EmbeddedChat` ẩn khi chưa có F119) + `PartyHud` — file: `src/game/scenes/social/PartyLobby.tscn`, `src/game/scenes/social/party_lobby.gd`, `src/game/scenes/ui/hud/PartyHud.tscn`, `src/game/scenes/ui/hud/party_hud.gd`
- [ ] T020 [US2] `PartyService`: ready, select, start, kick, transfer, disband, update settings, listPartyEligibleActivities; nối `RealtimeClient.message_received` → `PartyStore` — file: `src/game/scripts/multiplayer/party_service.gd`

## Phase 5: User Story 3 — Mất kết nối của bạn, của leader và của chính mình (Priority: P1)

### Tests cho User Story 3

- [ ] T021 [P] [US3] Scene test F102-AC-13, F102-AC-14: thành viên `disconnected` → thẻ/khung mờ; băng leader đang kết nối lại; `party.leader.transferred` `leader_disconnected` → toast + bật điều khiển leader cho người mới — file: `src/game/tests/scene/test_party_disconnect_ui.gd` — phải FAIL trước khi code
- [ ] T022 [P] [US3] Unit test F102-AC-15: mất mạng khóa thao tác ghi, giữ dữ liệu; kết nối lại → `getMyParty`, vào lại `room_id` khi `in_activity` — file: `src/game/tests/unit/test_party_reconnect.gd`

### Implementation cho User Story 3

- [ ] T023 [US3] Xử lý presence/leader grace trong `PartyStore` và UI; resync khi `EventBus.network_state_changed` — file: `src/game/scripts/multiplayer/party_store.gd`, `src/game/scenes/social/party_lobby.gd`, `src/game/scenes/ui/hud/party_hud.gd`

## Phase 6: User Story 4 — Song ngữ, trợ năng, giao diện theo token, an toàn (Priority: P2)

### Tests cho User Story 4

- [ ] T024 [P] [US4] Scene test F102-AC-16, F102-AC-18: đổi locale cập nhật ngay, chuỗi dài +35% không cắt; bố cục mobile 2400×1080 và PC 1280×720; Ready có chữ; thông báo vào/ra cho giọng đọc — file: `src/game/tests/scene/test_party_layout_i18n.gd` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Chạy `lint_theme_overrides.gd` trên scene party + kiểm biến thể density (F102-AC-17) — file: `src/game/tests/scene/test_party_theme.gd`
- [ ] T026 [P] [US4] Unit test F102-AC-19, F102-AC-20: không hiển thị trường ngoài nickname/avatar/level; không ghi `user://`; xóa store khi logout/`session.session.replaced`; quét script `scenes/**` không gọi `ApiClient`/`RealtimeClient` trực tiếp — file: `src/game/tests/unit/test_party_privacy_arch.gd`

### Implementation cho User Story 4

- [ ] T027 [US4] Hoàn thiện khóa dịch, density, trợ năng (announce), xóa store khi đăng xuất/takeover — file: `src/game/scripts/multiplayer/party_store.gd`, `src/game/scenes/social/party_lobby.gd`

## Phase 7: Polish & Cross-Cutting

- [ ] T028 [P] Integration headless với backend: tạo → mời → chấp nhận → Ready → Bắt đầu → kết thúc (TC-PTY-001…006) — file: `src/game/tests/integration/test_party_flow_headless.gd`
- [ ] T029 [P] Đo FPS lobby 4 thành viên trên thiết bị tầm trung tham chiếu; ghi vào báo cáo hiệu năng — file: `src/game/tests/perf/PartyLobbyBenchmark.tscn`
- [ ] T030 [P] Cập nhật PARTY_SYSTEM §12.4 theo tên scene GODOT_ARCHITECTURE; CHANGELOG — file: `docs/03-social-multiplayer/PARTY_SYSTEM.md`, `CHANGELOG.md`
- [ ] T031 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → US4 → Polish.
- Trước khi F100/F101 Done chạy trên Prism mock (profile `mock`, nhãn MOCK, không vào build phát hành — R17); feature chỉ Done khi chạy với backend thật.

## Definition of Done

- [ ] Mọi tiêu chí F102-AC-* có test pass (ghi mã test/TC)
- [ ] `make godot-test`, `make godot-export-check`, `make design-tokens-check` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

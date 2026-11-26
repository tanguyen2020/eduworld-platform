# Tasks: F102 — Giao diện party trong Godot: panel, lời mời, lobby, ready

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Thêm input action `open_party` (phím `P`) — file: `src/game/project.godot` — ✓ `open_party` (P) trong `project.godot`; WorldRoot mở màn `party`
- [x] T002 [P] Khóa dịch `ui.party.*` (title, alone, invite, ready, start, not_eligible, leader_reconnecting, new_leader, leave_confirm, disband_confirm, kick_confirm, transfer_confirm, switch_party_confirm, activity_changed, disbanded, kicked) và `ui.party.error.<CODE>` — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — ✓ khóa `ui.party.*` vi/en, `messages.pot`, `xx_PS.po` (`extract_pot --check`, `pseudo_locale --check` xanh)
- [x] T003 [P] Fixture từ example OpenAPI/AsyncAPI (party 1–4 thành viên, lời mời, snapshot, message WS mọi loại, eligible activities đủ/thiếu điều kiện, Problem) — file: `src/game/tests/fixtures/multiplayer/` — ✓ ví dụ OpenAPI (getMyParty, listMyInvitations, listPartyEligibleActivities, listFriends, startPartySession) + AsyncAPI `party.*` (16 ví dụ, kiểm bằng ajv) → `gen_fixtures.mjs` sinh `tests/fixtures/contract/{openapi,asyncapi}/` (`--check` xanh)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Model typed `PartyModel`, `PartyMemberModel`, `InvitationModel`, `EligibleActivityModel` (`from_dict` bỏ trường lạ) + test parse mọi fixture — file: `src/game/scripts/multiplayer/models/`, `src/game/tests/unit/test_party_models.gd` — kiểm: `make godot-test` — ✓ `tests/unit/multiplayer/test_party_models.gd` 8/8
- [x] T005 `PartyStore` (`apply_snapshot`, `apply_message`, `state_version`, signal typed) — file: `src/game/scripts/multiplayer/party_store.gd` — ✓ `party_store.gd`
- [x] T006 Khung `PartyService` (RefCounted, `ApiClient`/`RealtimeClient` qua constructor, map Problem → khóa dịch, `Idempotency-Key` theo intent), đăng ký trong `ServiceLocator`, `AppState.party`, signal `EventBus.party_invitation_received` — file: `src/game/scripts/multiplayer/party_service.gd`, `src/game/scripts/core/app_state.gd`, `src/game/scripts/core/event_bus.gd` — ✓ `party_service.gd`, `ServiceLocator.party`, `AppState.party`, `EventBus.party_invitation_received`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tạo nhóm, mời bạn, nhận lời mời (Priority: P1) 🎯 MVP

**Goal**: Tạo party và mời/nhận lời mời trên client.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Unit test F102-AC-01, F102-AC-04, F102-AC-05 với `FakeApiClient`: tạo party giữ key khi retry; `PARTY_ALREADY_IN_PARTY` chỉ leave+accept khi xác nhận; Problem → khóa dịch, không mã thô — file: `src/game/tests/unit/test_party_service.gd` — phải FAIL trước khi code — ✓ `tests/unit/multiplayer/test_party_service.gd` 12/12
- [x] T008 [P] [US1] Scene test F102-AC-02: picker chỉ hiện danh sách server trả, không nhóm/lọc theo lớp, không tự lọc khối/trường (ADR-0095), `g1_3` không có ô nhập — file: `src/game/tests/scene/test_invite_friend_picker.gd` — ✓ `tests/scene/social/test_party_invites.gd` (bảng mời: 3 test)
- [x] T009 [P] [US1] Scene test F102-AC-03: toast đếm ngược theo `expires_at` (Clock giả), tự ẩn 10 s, mở lại từ chuông, hết hạn biến mất; không bật khi đang ở activity player — file: `src/game/tests/scene/test_invitation_toast.gd` — ✓ `tests/scene/social/test_party_invites.gd` (popup, HUD, chuông: 6 test)

### Implementation cho User Story 1

- [x] T010 [P] [US1] `InviteFriendPicker` (nguồn `listFriends`) — file: `src/game/scenes/social/InviteFriendPicker.tscn`, `src/game/scenes/social/invite_friend_picker.gd` — ✓
- [x] T011 [P] [US1] `InvitationToast` + danh sách lời mời ở chuông — file: `src/game/scenes/social/InvitationToast.tscn`, `src/game/scenes/social/invitation_toast.gd` — ✓ (+ mục "Lời mời cho em" trên màn Nhóm, `party_forms.gd`)
- [x] T012 [US1] `PartyService`: create, invite, list/accept/reject/cancel invitation, leave — file: `src/game/scripts/multiplayer/party_service.gd` — ✓

## Phase 4: User Story 2 — Party Lobby: thấy nhau, sẵn sàng, chọn hoạt động, bắt đầu (Priority: P1)

### Tests cho User Story 2

- [x] T013 [P] [US2] Unit test F102-AC-07, F102-AC-08: áp message theo `state_version`, lệch → gọi `getParty`; Ready gửi WS, fallback REST khi WS đóng, hiển thị theo xác nhận — file: `src/game/tests/unit/test_party_store.gd` — phải FAIL trước khi code — ✓ `tests/unit/multiplayer/test_party_store.gd` 10/10
- [x] T014 [P] [US2] Scene test F102-AC-06, F102-AC-09: thẻ thành viên 2D, chỉ vẽ lại thẻ đổi; điều khiển leader chỉ ở leader; nút Bắt đầu xám kèm lý do — file: `src/game/tests/scene/test_party_lobby.gd` — ✓ `tests/scene/social/test_party_lobby.gd`
- [x] T015 [P] [US2] Scene test F102-AC-10: hoạt động thiếu điều kiện có cảnh báo, không chọn được; thẻ lớn `g1_3`, dropdown `g6_9` — file: `src/game/tests/scene/test_activity_selector.gd` — ✓ `tests/scene/social/test_activity_selector.gd` 4/4
- [x] T016 [P] [US2] Scene test F102-AC-11, F102-AC-12: `party.session.started` → đếm ngược + `SceneRouter.go_to(scene_ref)`; `party.session.ended` → lobby, không điểm của bạn; xác nhận rời/giải tán/kick/chuyển; thu nhỏ → `PartyHud` — file: `src/game/tests/scene/test_party_session_transition.gd` — ✓ `test_party_lobby.gd` (đếm ngược, tóm tắt, xác nhận, thu nhỏ) + `tests/unit/multiplayer/test_party_session_router.gd` 3/3 (thay `test_party_session_transition.gd`)

### Implementation cho User Story 2

- [x] T017 [P] [US2] `PartyMemberCard` (thumbnail cache, presence icon + chữ, Ready, vương miện, menu leader) — file: `src/game/scenes/social/PartyMemberCard.tscn`, `src/game/scenes/social/party_member_card.gd` — ✓
- [x] T018 [P] [US2] `ActivitySelector` — file: `src/game/scenes/social/ActivitySelector.tscn`, `src/game/scenes/social/activity_selector.gd` — ✓
- [x] T019 [US2] `PartyLobby` (bố cục PC hai cột / mobile, slot `EmbeddedChat` ẩn khi chưa có F119) + `PartyHud` — file: `src/game/scenes/social/PartyLobby.tscn`, `src/game/scenes/social/party_lobby.gd`, `src/game/scenes/ui/hud/PartyHud.tscn`, `src/game/scenes/ui/hud/party_hud.gd` — ✓ (+ `party_dialog.gd`, `party_forms.gd`; HUD `tests/scene/social/test_party_hud.gd` 2/2)
- [x] T020 [US2] `PartyService`: ready, select, start, kick, transfer, disband, update settings, listPartyEligibleActivities; nối `RealtimeClient.message_received` → `PartyStore` — file: `src/game/scripts/multiplayer/party_service.gd` — ✓

## Phase 5: User Story 3 — Mất kết nối của bạn, của leader và của chính mình (Priority: P1)

### Tests cho User Story 3

- [x] T021 [P] [US3] Scene test F102-AC-13, F102-AC-14: thành viên `disconnected` → thẻ/khung mờ; băng leader đang kết nối lại; `party.leader.transferred` `leader_disconnected` → toast + bật điều khiển leader cho người mới — file: `src/game/tests/scene/test_party_disconnect_ui.gd` — phải FAIL trước khi code — ✓ trong `test_party_lobby.gd` (thẻ mờ, băng trưởng nhóm, trưởng nhóm mới) + `test_party_hud.gd` (gộp file)
- [x] T022 [P] [US3] Unit test F102-AC-15: mất mạng khóa thao tác ghi, giữ dữ liệu; kết nối lại → `getMyParty`, vào lại `room_id` khi `in_activity` — file: `src/game/tests/unit/test_party_reconnect.gd` — ✓ `tests/unit/multiplayer/test_party_reconnect.gd` 4/4

### Implementation cho User Story 3

- [x] T023 [US3] Xử lý presence/leader grace trong `PartyStore` và UI; resync khi `EventBus.network_state_changed` — file: `src/game/scripts/multiplayer/party_store.gd`, `src/game/scenes/social/party_lobby.gd`, `src/game/scenes/ui/hud/party_hud.gd` — ✓

## Phase 6: User Story 4 — Song ngữ, trợ năng, giao diện theo token, an toàn (Priority: P2)

### Tests cho User Story 4

- [x] T024 [P] [US4] Scene test F102-AC-16, F102-AC-18: đổi locale cập nhật ngay, chuỗi dài +35% không cắt; bố cục mobile 2400×1080 và PC 1280×720; Ready có chữ; thông báo vào/ra cho giọng đọc — file: `src/game/tests/scene/test_party_layout_i18n.gd` — phải FAIL trước khi code — ✓ `test_party_lobby.gd` (đổi ngôn ngữ, bố cục điện thoại 1558×720) + `test_grade_band_matrix.gd` `test_party_lobby_fits_every_band_with_long_text` (pseudo-locale +35%)
- [x] T025 [P] [US4] Chạy `lint_theme_overrides.gd` trên scene party + kiểm biến thể density (F102-AC-17) — file: `src/game/tests/scene/test_party_theme.gd` — ✓ `lint_theme_overrides` xanh + `test_grade_band_matrix.gd` (`test_party_lobby_fits_every_band`, `test_hub_with_party_fits_every_band`: 4 khối × 3 cửa sổ)
- [x] T026 [P] [US4] Unit test F102-AC-19, F102-AC-20: không hiển thị trường ngoài nickname/avatar/level; không ghi `user://`; xóa store khi logout/`session.session.replaced`; quét script `scenes/**` không gọi `ApiClient`/`RealtimeClient` trực tiếp — file: `src/game/tests/unit/test_party_privacy_arch.gd` — ✓ `tests/unit/multiplayer/test_party_privacy_arch.gd` 5/5

### Implementation cho User Story 4

- [x] T027 [US4] Hoàn thiện khóa dịch, density, trợ năng (announce), xóa store khi đăng xuất/takeover — file: `src/game/scripts/multiplayer/party_store.gd`, `src/game/scenes/social/party_lobby.gd` — ✓

## Phase 7: Polish & Cross-Cutting

- [ ] T028 [P] Integration headless với backend: tạo → mời → chấp nhận → Ready → Bắt đầu → kết thúc (TC-PTY-001…006) — file: `src/game/tests/integration/test_party_flow_headless.gd` — ◐ viết xong, **bỏ qua khi chưa có backend** (`EDUWORLD_IT_API_BASE_URL`): chờ chạy hai client thật
- [ ] T029 [P] Đo FPS lobby 4 thành viên trên thiết bị tầm trung tham chiếu; ghi vào báo cáo hiệu năng — file: `src/game/tests/perf/PartyLobbyBenchmark.tscn` — ◐ CI guard `tests/perf/test_party_lobby_budget.gd` (không 3D, 84 Control hiện ≤ 150, cập nhật Ready ≈ 1 ms ≤ 3 ms); **đo FPS trên thiết bị tầm trung tham chiếu chờ lab** (thay `PartyLobbyBenchmark.tscn`)
- [x] T030 [P] Cập nhật PARTY_SYSTEM §12.4 theo tên scene GODOT_ARCHITECTURE; CHANGELOG — file: `docs/03-social-multiplayer/PARTY_SYSTEM.md`, `CHANGELOG.md` — ✓ PARTY_SYSTEM §12.4, STUDENT_GAME_UX_SOCIAL §5, STUDENT_GAME_UX_CORE S03, GODOT_ARCHITECTURE §3/§4, GAME_NETWORKING §8, DESIGN_SYSTEM §5.1, CONFIG_DEFAULTS, CHANGELOG
- [ ] T031 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — ◐ phần tự động + ảnh ghi ở quickstart §5; kịch bản hai thiết bị chờ chạy thật

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → US4 → Polish.
- Trước khi F100/F101 Done chạy trên Prism mock (profile `mock`, nhãn MOCK, không vào build phát hành — R17); feature chỉ Done khi chạy với backend thật.

## Definition of Done

- [ ] Mọi tiêu chí F102-AC-* có test pass (ghi mã test/TC) — phía client 20/20 AC có test pass với fake + fixture (quickstart §5); phần cần hai client với backend của AC-07 (≤ 1 s), AC-11 (vào đúng instance), AC-13/14/15 (mất kết nối thật) chờ chạy thật
- [x] `make godot-test`, `make godot-export-check`, `make design-tokens-check` xanh (quickstart §5)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — fake/fixture chỉ trong `tests/`, `tools/`
- [x] Tài liệu sở hữu, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật (In progress — chờ chạy thật)

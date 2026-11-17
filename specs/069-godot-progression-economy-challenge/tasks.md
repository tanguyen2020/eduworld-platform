# Tasks: F069 — Godot — tiến trình, kho đồ, shop, bộ sưu tập, thành tựu, challenge solo

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Thêm input action `open_inventory` (`I`) nếu chưa có, `open_shop` (`B`) — file: `godot/project.godot`
- [ ] T002 [P] Khóa dịch `ui.progression.*`, `ui.levelup.*`, `ui.journey.*` (gồm `ui.journey.source.<source_code>` theo catalog PROGRESSION_SYSTEM §3.1), `ui.inventory.*` (gồm `ui.inventory.reason.<reason_code>`), `ui.shop.*`, `ui.achievement.*`, `ui.collection.*`, `ui.challenge.*`, `ui.error.<CODE>` cho mã của feature — file: `godot/locale/vi.po`, `godot/locale/en.po`
- [ ] T003 [P] Fixture từ example OpenAPI/AsyncAPI (tiến trình thường/cap/max level, level-up 1 và 2 cấp, kho 500 món, shop đủ trạng thái, Problem, thành tựu ẩn, bộ sưu tập, challenge khối 3/7, attempt mọi trạng thái) — nhãn MOCK — file: `godot/tests/fixtures/{progression,inventory,achievement,challenge}/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Theo dõi delta F076 (`active_attempt_id`, `difficulties[]`, mô tả `LocalizedText`) — owner F076 cập nhật `specs/openapi/eduworld-api.v1.yaml` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game`
- [ ] T005 Theo dõi delta F075 (`how_to_obtain`, `hidden`, nhãn ledger song ngữ, `listInventory?kind`, `422`/`428`) — owner F075 — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T006 Sinh lại fixture sau khi T004/T005 merge + test contract parse mọi example — file: `godot/tests/contract/test_f069_fixtures.gd` — kiểm: `make godot-test`
- [ ] T007 Model typed theo data-model (`from_dict` bỏ trường lạ) + test parse — file: `godot/scripts/{progression,inventory,achievement,challenge}/models/`, `godot/tests/unit/models/test_f069_models.gd`
- [ ] T008 `AppState` thêm `progression`, `wallet`, `blocking_activity`; `EventBus` thêm `level_up_shown`, `wallet_changed`; đăng ký `ProgressionService`, `AchievementService`, `ChallengeClient` trong `ServiceLocator`; dispatcher `progression.*`/`challenge.*` — file: `godot/scripts/core/app_state.gd`, `godot/scripts/core/event_bus.gd`, `godot/scripts/core/service_locator.gd`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tiến trình trên HUD, lên cấp, "Hành trình của tôi" (Priority: P1) 🎯 MVP

**Goal**: HUD tiến trình, S07, Hành trình. **Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test F069-AC-01, F069-AC-02: `ProgressionService.load()` gọi `getProgression`; message thay giá trị (không cộng delta), `capped`, `xp_to_next = null` — file: `godot/tests/unit/progression/test_progression_service.gd` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Unit test F069-AC-03: `LevelUpQueue` gộp nhiều message, giữ khi `blocking_activity` khác rỗng, phát khi về rỗng — file: `godot/tests/unit/progression/test_level_up_queue.gd`
- [ ] T011 [P] [US1] Scene test F069-AC-04: bỏ qua hiệu ứng 1 chạm, tự đóng 10 s (`FakeClock`), nút theo `g1_3`/`g4_5`/`g6_9`, không nút "Nhận" — file: `godot/tests/scene/test_level_up_modal.gd`
- [ ] T012 [P] [US1] Scene test F069-AC-05: Hành trình phân trang cursor, nhãn từ `source_code`, không hiển thị ID — file: `godot/tests/scene/test_journey_screen.gd`
- [ ] T013 [P] [US1] Unit test F069-AC-06: quét mọi request của `FakeApiClient`/`FakeRealtimeClient` trong bộ test feature không có khóa `xp`, `level`, `score`, `amount`, `lives_left` — file: `godot/tests/unit/test_f069_no_client_scores.gd`

### Implementation cho User Story 1

- [ ] T014 [US1] `ProgressionService`, `ProgressionStore`, `LevelUpQueue` — file: `godot/scripts/progression/progression_service.gd`, `godot/scripts/progression/progression_store.gd`, `godot/scripts/progression/level_up_queue.gd`
- [ ] T015 [P] [US1] `XpBar`, `CurrencyChip`, danh hiệu trên `PlayerBadge`; bật slot trong `MainHud` — file: `godot/scenes/ui/hud/XpBar.tscn`, `godot/scenes/ui/hud/CurrencyChip.tscn`, `godot/scenes/ui/hud/main_hud.gd`
- [ ] T016 [P] [US1] `LevelUpModal` (dùng lại `AvatarPreview3D` F070, `RewardCarousel`) và `JourneyScreen` — file: `godot/scenes/progression/LevelUpModal.tscn`, `godot/scenes/progression/JourneyScreen.tscn`

## Phase 4: User Story 2 — Tủ đồ, Shop, Lịch sử (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Unit test F069-AC-10, F069-AC-11: một key mỗi ý định, giữ khi retry 5xx/mất mạng, số dư từ `balance_after`, map Problem → `ui.error.<CODE>`, `428`/`IDEMPOTENCY_KEY_REUSED` → telemetry `client_bug` — file: `godot/tests/unit/inventory/test_inventory_service_purchase.gd` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Scene test F069-AC-07, F069-AC-09: tab theo điểm vào, lọc `kind`, `VirtualItemGrid` chỉ tạo ô hiển thị + 1 hàng, một `AvatarPreview3D`; nút Mua xám "Cần thêm {n} Xu" — file: `godot/tests/scene/test_inventory_shop_screen.gd`
- [ ] T019 [P] [US2] Scene test F069-AC-08: trang bị qua `ProfileService` giả, `ITEM_NOT_OWNED`/`EQUIP_SLOT_INVALID` → xem trước về cũ — file: `godot/tests/scene/test_wardrobe_equip.gd`
- [ ] T020 [P] [US2] Scene test F069-AC-10, F069-AC-12: luôn có `PurchaseConfirmDialog` (giá, số dư sau mua); tab Lịch sử nhãn `reason_code`, ẩn ở `g1_3` — file: `godot/tests/scene/test_purchase_dialog.gd`
- [ ] T021 [P] [US2] Test quét F069-AC-13: không có SDK IAP/thanh toán trong `addons/`, export preset; không scene/khóa dịch "loot", "gift", "trade", đếm ngược khuyến mãi — file: `godot/tests/unit/test_no_real_money_no_lootbox.gd`

### Implementation cho User Story 2

- [ ] T022 [US2] Mở rộng `InventoryService` (kho theo `kind`, ví, shop, mua, lịch sử) + `WalletStore` — file: `godot/scripts/inventory/inventory_service.gd`, `godot/scripts/inventory/wallet_store.gd`
- [ ] T023 [P] [US2] `VirtualItemGrid`, `FilterChips`, `ItemDetailBar` (thumbnail atlas, tải lười) — file: `godot/scenes/inventory/VirtualItemGrid.tscn`, `godot/scenes/inventory/FilterChips.tscn`, `godot/scenes/inventory/ItemDetailBar.tscn`
- [ ] T024 [US2] `InventoryShopScreen` nhúng `CustomizationStudio` (F070) làm tab Tủ đồ + `ShopTab`, `ShopItemCard`, `PurchaseConfirmDialog`, `LedgerTab`; NPC Thợ may mở tab Shop qua `SceneRouter` intent — file: `godot/scenes/inventory/InventoryShopScreen.tscn`, `godot/scenes/inventory/ShopTab.tscn`, `godot/scenes/inventory/PurchaseConfirmDialog.tscn`, `godot/scenes/inventory/LedgerTab.tscn`

## Phase 5: User Story 3 — Challenge một người theo mức khó (Priority: P1)

### Tests cho User Story 3

- [ ] T025 [P] [US3] Unit test F069-AC-16, F069-AC-17: `start` giữ key khi retry, map `CHALLENGE_NOT_ELIGIBLE`/`CHALLENGE_ATTEMPT_LIMIT`/`CONFLICT`; `next` key theo lượt, `503` thử lại cùng key, mở `ActivityHost` với `attempt` — file: `godot/tests/unit/challenge/test_challenge_client.gd` — phải FAIL trước khi code
- [ ] T026 [P] [US3] Unit test F069-AC-18, F069-AC-20: thời gian còn lại từ `deadline_at` + `FakeClock` khi đổi giờ máy; resume qua `active_attempt_id`; `INVALID_STATE_TRANSITION` → tải lại — file: `godot/tests/unit/challenge/test_challenge_store_timer.gd`
- [ ] T027 [P] [US3] Scene test F069-AC-14, F069-AC-15: board chỉ hiện `available_difficulties`, mức khóa chỉ khi có `locked_reason_code`, `party_required` không có nút solo; hết lượt/cooldown — file: `godot/tests/scene/test_challenge_board.gd`
- [ ] T028 [P] [US3] Scene test F069-AC-19: kết quả `failed` không có "Thất bại", phần thưởng từ server; bỏ cuộc qua xác nhận — file: `godot/tests/scene/test_challenge_result.gd`

### Implementation cho User Story 3

- [ ] T029 [US3] `ChallengeClient`, `ChallengeStore` (đặt/xóa `AppState.blocking_activity`) — file: `godot/scripts/challenge/challenge_client.gd`, `godot/scripts/challenge/challenge_store.gd`
- [ ] T030 [P] [US3] `ChallengeBoard`, `ChallengeInfoCard`, `DifficultySelector` — file: `godot/scenes/challenge/ChallengeBoard.tscn`, `godot/scenes/challenge/ChallengeInfoCard.tscn`, `godot/scenes/challenge/DifficultySelector.tscn`
- [ ] T031 [US3] `ChallengeHud` (timer `g1_3` dạng thanh), `ChallengeResult`, nối `ActivityHost` (F063) — file: `godot/scenes/challenge/ChallengeHud.tscn`, `godot/scenes/challenge/ChallengeResult.tscn`

## Phase 6: User Story 4 — Thành tựu, bộ sưu tập, huy hiệu trưng bày (Priority: P2)

### Tests cho User Story 4

- [ ] T032 [P] [US4] Scene test F069-AC-21, F069-AC-22: thành tựu ẩn chỉ "?" kể cả khi fixture có `title`; không nút "Nhận" — file: `godot/tests/scene/test_achievement_screen.gd` — phải FAIL trước khi code
- [ ] T033 [P] [US4] Scene test F069-AC-23, F069-AC-24: món chưa có xám + "Chưa có" + "Cách nhận", sổ dán `g1_3`; huy hiệu trưng bày chỉ món sở hữu, số ô theo server, lưu qua `ProfileService` — file: `godot/tests/scene/test_collection_badges.gd`

### Implementation cho User Story 4

- [ ] T034 [US4] `AchievementService` — file: `godot/scripts/achievement/achievement_service.gd`
- [ ] T035 [US4] `AchievementCollectionScreen`, `AchievementRow`, `CollectionGrid` (shader xám dùng chung texture), `BadgeShowcaseEditor` — file: `godot/scenes/achievement/`

## Phase 7: User Story 5 — Song ngữ, trợ năng, token, hiệu năng, an toàn (Priority: P2)

### Tests cho User Story 5

- [ ] T036 [P] [US5] Scene test F069-AC-25: đổi `vi`↔`en` cập nhật ngay, pseudo-locale `xx` +40% không cắt nút chính, số theo locale — file: `godot/tests/scene/test_f069_i18n_layout.gd` — phải FAIL trước khi code
- [ ] T037 [P] [US5] `lint_theme_overrides.gd` + density + độ hiếm màu kèm biểu tượng (F069-AC-26) — file: `godot/tests/scene/test_f069_theme.gd`
- [ ] T038 [P] [US5] Benchmark F069-AC-27: lưới 500 ô ≥ 30 FPS, mở panel ≤ 200 ms, HUD không cập nhật trong `_process` — file: `godot/tests/perf/InventoryGridBenchmark.tscn`, `godot/tests/perf/HudBenchmark.tscn`
- [ ] T039 [P] [US5] Unit test F069-AC-28: không ghi `user://`, xóa store khi logout/`session.session.replaced`, khóa ghi khi mất mạng; quét `scenes/**` không gọi `ApiClient`/`RealtimeClient` — file: `godot/tests/unit/test_f069_privacy_arch.gd`
- [ ] T040 [P] [US5] Unit test F069-AC-29, F069-AC-30: `X-Tenant-Id` có ở mọi request; `GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` → `EventBus.session_expired`; `404` làm mới danh sách; `422` → telemetry `client_bug` không PII — file: `godot/tests/unit/test_f069_auth_errors.gd`

### Implementation cho User Story 5

- [ ] T041 [US5] Hoàn thiện khóa dịch, density, announce trợ năng, xóa store khi đăng xuất/takeover, khóa ghi khi mất mạng — file: `godot/scripts/progression/progression_store.gd`, `godot/scripts/inventory/wallet_store.gd`, `godot/scripts/challenge/challenge_store.gd`

## Phase 8: Polish & Cross-Cutting

- [ ] T042 [P] Integration headless với backend: Hub → mua → challenge `time` → level-up sau kết quả (TC-ECO-001, TC-CHL-001…003) — file: `godot/tests/integration/test_progression_shop_challenge_headless.gd`
- [ ] T043 [P] Cập nhật GODOT_ARCHITECTURE §3 (thư mục mới), STUDENT_GAME_UX §4.1 (phím `B`), STUDENT_GAME_UX_SOCIAL §4 (bỏ phím `J`), CHANGELOG — file: `docs/08-game-client/GODOT_ARCHITECTURE.md`, `docs/05-ux/STUDENT_GAME_UX.md`, `docs/05-ux/STUDENT_GAME_UX_SOCIAL.md`, `CHANGELOG.md`
- [ ] T044 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → US4 → US5 → Polish. US2 và US3 độc lập nhau sau US1 (dùng `CurrencyChip`, `LevelUpQueue`).
- US3 cần `ActivityHost` (F063) và `active_attempt_id` (T004); trước khi F074–F076 Done chạy trên Prism mock (profile `mock`, nhãn MOCK, không vào build phát hành — R17).

## Definition of Done

- [ ] Mọi tiêu chí F069-AC-* có test pass (ghi mã test/TC)
- [ ] `make godot-test`, `make godot-export-check`, `make design-tokens-check` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

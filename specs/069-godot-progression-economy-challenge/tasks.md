# Tasks: F069 — Godot — tiến trình, kho đồ, shop, bộ sưu tập, thành tựu, challenge solo

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Quyết định khi triển khai ghi ở [spec.md — Clarifications, Session 2026-11-26](spec.md); điểm còn mở F069-1…3 ở cuối mục đó.

## Phase 1: Setup

- [x] T001 Input action `open_shop` (`B`) — `open_wardrobe` (`I`) đã có từ F070 — file: `src/game/project.godot`
- [x] T002 [P] Khóa dịch `ui.progression.*`, `ui.levelup.*`, `ui.journey.*` (gồm `ui.journey.source.<source_code>` theo catalog PROGRESSION_SYSTEM §3.1), `ui.inventory.*` (gồm `ui.inventory.reason.<reason_code>`, `ui.inventory.rarity.*`), `ui.shop.*`, `ui.achievement.*`, `ui.collection.*`, `ui.challenge.*`, `ui.reward.*`, `ui.menu.*`, `ui.error.<CODE>` (8 mã mới) — file: `src/game/locale/vi.po`, `src/game/locale/en.po`, `src/game/scripts/core/l10n.gd`
- [x] T003 [P] Fixture: 16 schema F069 thêm vào `tools/gen_fixtures.mjs` (mẫu chuỗi có `pattern` nay khớp pattern, vd `CatalogCode` → `sample1`); builder dữ liệu mẫu nhãn MOCK `tests/fakes/f069_fixtures.gd` (tiến trình thường/cap/max, level-up 1 và 2 cấp, kho 9 món + 500 món, shop đủ trạng thái, Problem, thành tựu ẩn, bộ sưu tập 3/10, challenge khối 3/7, attempt mọi trạng thái) — file: `src/game/tools/gen_fixtures.mjs`, `src/game/tests/fixtures/contract/`, `src/game/tests/fakes/f069_fixtures.gd`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Theo dõi delta F076 (`active_attempt_id`, `difficulties[]`, mô tả `LocalizedText`) — owner F076 — **chưa có trong OpenAPI** (Câu hỏi mở F069-1); client đã đọc tolerant khi có (`ChallengeSummaryModel`, `ChallengeDetailModel`)
- [ ] T005 Theo dõi delta F075 (`how_to_obtain` `LocalizedText`, `hidden`, nhãn ledger song ngữ) — owner F075 — `how_to_obtain` đã có (chuỗi), `hidden` chưa có: client coi `category = hidden` chưa mở là ẩn; nhãn chuỗi/`LocalizedName` đều đọc được (`DataRead.text_of`)
- [x] T006 Sinh fixture + test contract parse mọi example F069 — file: `src/game/tests/contract/test_f069_fixtures.gd`
- [x] T007 Model typed theo data-model (`from_dict` bỏ trường lạ, `DataRead` đọc an toàn) — file: `src/game/scripts/{progression,inventory,achievement,challenge}/models/`, `src/game/scripts/core/models/data_read.gd`
- [x] T008 `AppState.blocking_activity` + `begin_blocking`/`end_blocking` (+ `ExamRunner` đặt `exam`); `EventBus.level_up_shown`, `wallet_changed`; `ServiceLocator` tạo `ProgressionService`, `AchievementService`, `ChallengeClient`, nối `InventoryService` (realtime, profile, telemetry), xóa khi `end_session`, đọc lại khi `resync`; dispatcher `progression.*`/`challenge.*`/`notification.*` qua `RealtimeClient.register_handler`; lớp gốc `GameApiService` — file: `src/game/scripts/core/app_state.gd`, `src/game/scripts/core/event_bus.gd`, `src/game/scripts/core/service_locator.gd`, `src/game/scripts/services/game_api_service.gd`, `src/game/scenes/exam/exam_runner.gd`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tiến trình trên HUD, lên cấp, "Hành trình của tôi" (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T009 [P] [US1] Unit F069-AC-01, AC-02 — `tests/unit/progression/test_progression_service.gd` (8 test)
- [x] T010 [P] [US1] Unit F069-AC-03 — `tests/unit/progression/test_level_up_queue.gd` (3 test: gộp, hoãn theo `blocking_activity`, gộp vào modal đang mở)
- [x] T011 [P] [US1] Scene F069-AC-03, AC-04 — `tests/scene/progression/test_level_up_modal.gd` (5 test, gồm WorldRoot hoãn modal trong challenge)
- [x] T012 [P] [US1] Scene F069-AC-01, AC-02, AC-05, AC-27 — `tests/scene/progression/test_journey_and_hud.gd` (4 test)
- [x] T013 [P] [US1] F069-AC-06 — `test_no_request_carries_progress_values` (progression) và `test_no_request_carries_score_lives_or_result` (challenge)

### Implementation cho User Story 1

- [x] T014 [US1] `ProgressionService`, `ProgressionStore`, `LevelUpQueue` — file: `src/game/scripts/progression/`
- [x] T015 [P] [US1] `XpBar` (bảng `XpPanel` dưới `PlayerBadge`), `CurrencyChip` + nút "Thêm" (cột phải), `GameMenu` — file: `src/game/scenes/ui/hud/`
- [x] T016 [P] [US1] `LevelUpModal` (Mây ăn mừng, `RewardRow` — không xem trước 3D, Clarifications 2026-11-26) và `JourneyScreen` — file: `src/game/scenes/progression/`

## Phase 4: User Story 2 — Tủ đồ, Shop, Lịch sử (Priority: P1)

### Tests cho User Story 2

- [x] T017 [P] [US2] Unit F069-AC-10, AC-11, AC-12 — `tests/unit/inventory/test_inventory_service_purchase.gd` (8 test)
- [x] T018 [P] [US2] Scene F069-AC-07, AC-09 — `tests/scene/inventory/test_inventory_shop_screen.gd`
- [x] T019 [P] [US2] Scene F069-AC-08 — `test_equip_takes_the_server_answer_and_reverts_on_refusal`
- [x] T020 [P] [US2] Scene F069-AC-10, AC-12 — `test_shop_need_more_and_always_confirms_with_balance_after`, `test_network_failure_keeps_the_dialog_and_the_key`, `test_history_hidden_for_g1_3_and_friendly_for_older_grades`
- [x] T021 [P] [US2] Quét F069-AC-13 — `tests/unit/test_f069_safety_privacy.gd::test_no_real_money_no_loot_box_no_trade_no_countdown`

### Implementation cho User Story 2

- [x] T022 [US2] Mở rộng `InventoryService` (kho theo `kind`, ví, shop, mua, lịch sử, trang bị qua `ProfileService`) + `WalletStore` — file: `src/game/scripts/inventory/`
- [x] T023 [P] [US2] `VirtualItemGrid`, `FilterChips`, `ItemThumb`, `RarityMark`, `RewardRow`, `UiBuild`, `AvatarPreview3D` — file: `src/game/scenes/ui/components/`, `src/game/scenes/character/AvatarPreview3D.tscn`
- [x] T024 [US2] `InventoryShopScreen` (tab Tủ đồ/Cửa hàng/Lịch sử, thanh chi tiết, hộp xác nhận mua; "Phối đồ" mở `CustomizationStudio` F070) — screen id `inventory`; HUD Tủ đồ/`I`, chip Xu/`B` — file: `src/game/scenes/inventory/`, `src/game/scripts/core/ui_screen_registry.gd`, `src/game/scenes/world/world_root.gd`

## Phase 5: User Story 3 — Challenge một người theo mức khó (Priority: P1)

### Tests cho User Story 3

- [x] T025 [P] [US3] Unit F069-AC-16, AC-17 — `tests/unit/challenge/test_challenge_client.gd`
- [x] T026 [P] [US3] Unit F069-AC-18, AC-20 — `test_clock_follows_deadline_and_ignores_the_device_clock`, `test_resume_takes_back_a_run_and_invalid_state_reloads`, `test_conflict_offers_to_continue_the_run_in_progress` (14 test trong file)
- [x] T027 [P] [US3] Scene F069-AC-14, AC-15 — `tests/scene/challenge/test_challenge_screens.gd`
- [x] T028 [P] [US3] Scene F069-AC-19 (+ AC-18 HUD) — cùng file (8 test)

### Implementation cho User Story 3

- [x] T029 [US3] `ChallengeClient`, `ChallengeStore` (đồng hồ server, `blocking_activity`) — file: `src/game/scripts/challenge/`
- [x] T030 [P] [US3] `ChallengeBoard` (ChallengeInfoCard + DifficultySelector trong màn) — file: `src/game/scenes/challenge/challenge_board.gd`
- [x] T031 [US3] `ChallengeHud` (dải trên cùng, `g1_3` thanh không số giây), `ChallengeResult`, nối `ActivityHost` (`set_top_inset`, intent `activity_host` với `attempt_id`) — file: `src/game/scenes/challenge/`, `src/game/scenes/learning/activity_host.gd`

## Phase 6: User Story 4 — Thành tựu, bộ sưu tập, huy hiệu trưng bày (Priority: P2)

### Tests cho User Story 4

- [x] T032 [P] [US4] Scene F069-AC-21, AC-22 — `tests/scene/achievement/test_achievement_screen.gd`
- [x] T033 [P] [US4] Scene F069-AC-23, AC-24 — cùng file (5 test)

### Implementation cho User Story 4

- [x] T034 [US4] `AchievementService` — file: `src/game/scripts/achievement/achievement_service.gd`
- [x] T035 [US4] `AchievementCollectionScreen` (dòng thành tựu, trang bộ sưu tập với `ItemThumb` + shader xám `ui_greyscale.gdshader`, huy hiệu trưng bày) — file: `src/game/scenes/achievement/`, `src/game/resources/art/shaders/ui_greyscale.gdshader`

## Phase 7: User Story 5 — Song ngữ, trợ năng, token, hiệu năng, an toàn (Priority: P2)

### Tests cho User Story 5

- [x] T036 [P] [US5] F069-AC-25 — `tests/scene/test_f069_i18n_theme_perf.gd` (khóa vi/en, đổi ngôn ngữ, định dạng số, pseudo-locale)
- [x] T037 [P] [US5] F069-AC-26 — cùng file (`ThemeRules` cho scene/script F069) + `test_rarity_is_colour_and_shape_with_the_word_for_older_grades`, `test_touch_targets_follow_the_band`
- [x] T038 [P] [US5] F069-AC-27 — `test_grid_of_500_items_keeps_a_small_pool_while_scrolling`, `test_panels_open_within_200_ms`, `test_hud_widgets_never_poll` (headless: đo pool/thời gian mở, không đo FPS thiết bị — FPS Android tầm trung là bằng chứng tay theo `tests/perf` lab)
- [x] T039 [P] [US5] F069-AC-28 — `tests/unit/test_f069_safety_privacy.gd` (`user://`, `end_session`, LayerRules) + khóa ghi khi mất mạng ở scene test S08
- [x] T040 [P] [US5] F069-AC-29, AC-30 — cùng file (`X-Tenant-Id`, `GAME_ACCESS_STUDENT_ONLY` → F061, `TENANT_MISMATCH` → ApiClient, `422` → `client_bug`, chỉ gửi uuid/enum của server)

### Implementation cho User Story 5

- [x] T041 [US5] Khóa dịch, density theo khối (`UiBuild.target`), xóa store khi đăng xuất/takeover (`ServiceLocator.end_session`), khóa Mua/Trang bị/Vào khi `offline` (`GameApiService`)

## Phase 8: Polish & Cross-Cutting

- [x] T042 [P] Integration headless với backend — `tests/integration/test_progression_shop_challenge_headless.gd` (bỏ qua khi không có `EDUWORLD_IT_API_BASE_URL`, như các test integration khác)
- [x] T043 [P] Tài liệu: GODOT_ARCHITECTURE §3–§4, STUDENT_GAME_UX §3, STUDENT_GAME_UX_CORE S03 §6b / S06 §6a / S07 §6a, STUDENT_GAME_UX_SOCIAL §3–§4, DESIGN_SYSTEM §5.1, PROGRESSION_SYSTEM §4.6, INVENTORY_ECONOMY §5.1, CHALLENGE_BOSS_SYSTEM §2.5, CODE_PLAN, TRACEABILITY_MATRIX, CHANGELOG, `design/CHANGELOG.md`, `assets/LICENSES.md` (phím `B` đã có ở STUDENT_GAME_UX §4.1; SOCIAL §4 đã bỏ phím `J`)
- [x] T044 Quickstart: kịch bản 1–20 chạy tự động trên fake (bảng dưới) + ảnh Compatibility ở `screenshots/`; kịch bản với stack compose (tài khoản seed thật) chưa chạy trên máy dùng chung

## Bảng AC → test

| AC | Test (GdUnit4) |
|---|---|
| F069-AC-01 | `test_progression_service::test_entering_reads_get_progression_into_the_store`, `test_journey_and_hud::test_hud_shows_progression_and_wallet_from_the_server` |
| F069-AC-02 | `test_progression_service::test_xp_message_replaces_values_without_polling`, `::test_capped_message_shows_the_moon_and_null_is_max_level`, `::test_span_change_reloads_progression`, `test_journey_and_hud::test_message_updates_the_hud_in_the_same_frame_without_polling` |
| F069-AC-03 | `test_level_up_queue` (3 test), `test_progression_service::test_level_reached_is_queued_and_reloads`, `test_level_up_modal::test_world_holds_the_modal_during_a_challenge_and_shows_it_after` |
| F069-AC-04 | `test_level_up_modal::test_heading_rewards_and_no_claim_button`, `::test_buttons_follow_the_grade_band`, `::test_focus_skip_and_auto_close`, `::test_reduced_motion_is_a_static_card` |
| F069-AC-05 | `test_progression_service::test_history_pages_by_cursor_with_friendly_labels`, `test_journey_and_hud::test_journey_lists_friendly_lines_and_pages_by_cursor` |
| F069-AC-06 | `test_progression_service::test_no_request_carries_progress_values`, `test_challenge_client::test_no_request_carries_score_lives_or_result` |
| F069-AC-07 | `test_inventory_shop_screen::test_wardrobe_tab_loads_inventory_by_kind_with_one_preview`, `test_f069_i18n_theme_perf::test_grid_of_500_items_keeps_a_small_pool_while_scrolling` |
| F069-AC-08 | `test_inventory_shop_screen::test_equip_takes_the_server_answer_and_reverts_on_refusal` |
| F069-AC-09 | `test_inventory_shop_screen::test_shop_need_more_and_always_confirms_with_balance_after`, `test_inventory_service_purchase::test_affordability_is_a_display_hint_only` |
| F069-AC-10 | `test_inventory_service_purchase::test_purchase_carries_one_key_and_takes_balance_after`, `::test_network_failure_keeps_the_key_for_the_retry`, `::test_server_error_keeps_the_key`, `test_inventory_shop_screen::test_shop_need_more_and_always_confirms_with_balance_after`, `::test_network_failure_keeps_the_dialog_and_the_key` |
| F069-AC-11 | `test_inventory_service_purchase::test_refusals_are_codes_and_refresh_wallet_and_shop`, `::test_precondition_and_key_reuse_are_client_bugs`, `test_inventory_shop_screen::test_refusal_shows_a_friendly_message_never_the_code` |
| F069-AC-12 | `test_inventory_service_purchase::test_inventory_by_kind_and_ledgers_by_cursor`, `test_inventory_shop_screen::test_history_hidden_for_g1_3_and_friendly_for_older_grades` |
| F069-AC-13 | `test_f069_safety_privacy::test_no_real_money_no_loot_box_no_trade_no_countdown` |
| F069-AC-14 | `test_challenge_screens::test_board_shows_only_the_server_difficulties_per_grade`, `::test_locked_level_only_with_the_server_reason`, `::test_party_challenge_has_no_solo_button` |
| F069-AC-15 | `test_challenge_screens::test_no_tries_left_and_cooldown_lock_the_enter_button`, `test_challenge_client::test_not_found_detail_refreshes_the_list` |
| F069-AC-16 | `test_challenge_client::test_start_then_next_opens_the_activity_player_on_the_server_attempt`, `::test_start_refusals_are_friendly_notices`, `::test_conflict_offers_to_continue_the_run_in_progress`, `::test_start_key_is_kept_on_a_network_retry`, `test_challenge_screens::test_enter_starts_the_run` |
| F069-AC-17 | `test_challenge_client::test_next_uses_one_key_per_turn_and_retries_503_with_the_same_key`, `::test_learning_submission_fetches_the_next_activity`, `::test_done_finishes_the_run_and_opens_the_result` |
| F069-AC-18 | `test_challenge_client::test_clock_follows_deadline_and_ignores_the_device_clock`, `::test_messages_of_another_attempt_are_ignored`, `test_challenge_screens::test_hud_clock_bar_for_g1_3_and_digits_for_g6_7_and_abandon_confirms` |
| F069-AC-19 | `test_challenge_screens::test_failed_result_is_kind_and_shows_server_rewards`, `::test_succeeded_and_abandoned_results`, `::test_hud_clock_bar_for_g1_3_and_digits_for_g6_7_and_abandon_confirms`, `test_challenge_client::test_abandon_carries_a_key_and_ends_the_run` |
| F069-AC-20 | `test_challenge_client::test_resume_takes_back_a_run_and_invalid_state_reloads`, `::test_conflict_offers_to_continue_the_run_in_progress` (phần `active_attempt_id` chờ delta F076 — F069-1) |
| F069-AC-21 | `test_achievement_screen::test_hidden_achievement_shows_only_a_question_mark` |
| F069-AC-22 | `test_achievement_screen::test_unlocked_says_received_and_no_claim_button`, `test_level_up_modal::test_heading_rewards_and_no_claim_button` |
| F069-AC-23 | `test_achievement_screen::test_collection_greys_missing_items_with_how_to_obtain`, `::test_g1_3_collection_is_a_sticker_book_of_bigger_pictures` |
| F069-AC-24 | `test_achievement_screen::test_showcase_keeps_owned_badges_within_the_server_slots_and_saves` |
| F069-AC-25 | `test_f069_i18n_theme_perf::test_every_f069_key_exists_in_vi_and_en`, `::test_locale_switch_and_number_format`, `::test_pseudo_locale_keeps_the_main_buttons_whole`, `test_pseudo_locale`, `test_extract_pot` |
| F069-AC-26 | `test_f069_i18n_theme_perf::test_f069_scenes_have_no_theme_override_and_no_hardcoded_text`, `test_inventory_shop_screen::test_rarity_is_colour_and_shape_with_the_word_for_older_grades`, `::test_touch_targets_follow_the_band` |
| F069-AC-27 | `test_f069_i18n_theme_perf::test_grid_of_500_items_keeps_a_small_pool_while_scrolling`, `::test_panels_open_within_200_ms`, `test_journey_and_hud::test_hud_widgets_never_poll` |
| F069-AC-28 | `test_f069_safety_privacy::test_data_stays_in_ram_and_scenes_never_call_the_network`, `::test_session_end_drops_every_f069_store`, `test_inventory_shop_screen::test_offline_locks_buy_and_equip_but_keeps_the_items` |
| F069-AC-29 | `test_f069_safety_privacy::test_tenant_header_and_session_ending_refusals`, `test_challenge_client::test_not_found_detail_refreshes_the_list` |
| F069-AC-30 | `test_f069_safety_privacy::test_validation_failed_is_a_client_bug_without_pii`, `::test_client_sends_only_server_ids_and_enums` |

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 → US4 → US5 → Polish. US2 và US3 độc lập nhau sau US1 (dùng `CurrencyChip`, `LevelUpQueue`).
- US3 cần `ActivityHost` (F063); `active_attempt_id` (T004) là delta F076 chưa có — client đọc khi có.

## Definition of Done

- [x] Mọi tiêu chí F069-AC-* có test pass (bảng AC → test)
- [x] GdUnit4 toàn bộ, lint client, `make design-tokens-check`, export Web xanh (số liệu ở CODE_PLAN và báo cáo)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17: fixture/fake chỉ ở `tests/`)
- [x] Tài liệu sở hữu, CHANGELOG cập nhật
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

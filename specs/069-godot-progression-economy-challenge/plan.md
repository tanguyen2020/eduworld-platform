# Implementation Plan: F069 — Godot — tiến trình, kho đồ, shop, bộ sưu tập, thành tựu, challenge solo

**Branch**: `069-godot-progression-economy-challenge` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm bốn domain service phía client theo [GODOT_ARCHITECTURE §4, §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md): `ProgressionService` (REST tiến trình + message `progression.*`), mở rộng `InventoryService` của F070 (kho, ví, shop, lịch sử), `AchievementService` (thành tựu, bộ sưu tập) và `ChallengeClient` (challenge solo + message `challenge.*`, tên theo dispatcher [GAME_NETWORKING §4.2](../../docs/08-game-client/GAME_NETWORKING.md)). Mỗi service là `RefCounted` tạo bởi `ServiceLocator`, nhận `ApiClient`/`RealtimeClient` qua constructor, giữ store thuần trong RAM và phát signal typed; UI chỉ đọc store. HUD bật các slot `XpBar`, `CurrencyChip` mà F062 để ẩn; `LevelUpQueue` hoãn S07 khi `AppState.blocking_activity` khác rỗng. Challenge dùng `ActivityHost` của F063 cho từng hoạt động; đồng hồ chỉ từ `deadline_at` + `Clock`. Phát triển trên Prism mock (nhãn MOCK) tới khi F074–F076 `Done`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim trong `src/game/.godot-version`), static typing bắt buộc (ADR-0014) |
| Primary Dependencies | GdUnit4; autoload `Config`, `Log`, `EventBus`, `AppState`, `ApiClient`, `RealtimeClient`, `SceneRouter`, `Settings`, `QualityManager`; F063 `ActivityHost`/`LearningService`; F070 `ProfileService`, `InventoryService`, `AvatarPreview3D`, `ItemVisual` |
| Storage | Không có dữ liệu bền; store trong RAM, không ghi `user://` (ADR-0064) |
| Testing | GdUnit4 unit (`FakeApiClient`, `FakeRealtimeClient`, `FakeClock`) + scene runner headless; contract fixture từ `specs/`; benchmark `tests/perf`; integration headless với docker compose |
| Target Platform | Windows 10+, Android 9+, iOS, Web (ADR-0013) |
| Project Type | game client |
| Performance Goals | Panel mở ≤ 200 ms, lưới 500 ô ≥ 30 FPS, HUD ≤ 60 Control ([STUDENT_GAME_UX §6](../../docs/05-ux/STUDENT_GAME_UX.md), [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md)) |
| Constraints | AGENTS R1, R2, R5, R16, R17, R19, R32 · ADR-0016, ADR-0047, ADR-0049, ADR-0056, ADR-0057, ADR-0064, ADR-0065 |
| Scale/Scope | 5 màn (S03 phần tiến trình, S06 challenge solo, S07, S08, S09) + "Hành trình của tôi"; ≤ 500 vật phẩm/học sinh |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Chỉ tiêu thụ contract F074–F076; delta ghi ở [contracts §6](contracts/README.md) và task Phase 2 (T004–T006) do owner backend thực hiện |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client gọi API public/WS qua `ApiClient`/`RealtimeClient`; tách Service → Store → UI; không gọi internal API, không NATS/Valkey (R25) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` do `ApiClient` gắn; store chỉ RAM, xóa khi đăng xuất/takeover |
| IV Server-authoritative | ✅ | Không gửi XP/điểm/kết quả; giá, số dư, lượt, hạn chót, phần thưởng từ server; mua/bắt đầu/next/bỏ có `Idempotency-Key` |
| V Xác thực, phân quyền | ✅ | Token RAM (F061); `GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` về đăng nhập; quyền thật kiểm ở backend |
| VI An toàn trẻ em | ✅ | Không IAP, loot box, đếm ngược khuyến mãi, chữ "Thất bại"; không hiển thị dữ liệu người khác; không log PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Không chứa curve/danh hiệu/luật khối; tham số theo tên CONFIG_DEFAULTS; chuỗi qua `.po` |
| VIII UI tách logic, token | ✅ | UI chỉ gọi method public service; Theme sinh từ `design/game-tokens`, `lint_theme_overrides.gd` |
| IX Test có bằng chứng | ✅ | Mỗi F069-AC có task test viết trước; mock gắn nhãn, không vào build phát hành |

## Project Structure

### Documentation (feature này)

```text
specs/069-godot-progression-economy-challenge/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/game/
├── project.godot                               # input action open_inventory (I), open_shop (B) — thêm nếu F070 chưa khai báo
├── scenes/
│   ├── ui/hud/{XpBar.tscn, CurrencyChip.tscn}  # + xp_bar.gd, currency_chip.gd; bật slot trong MainHud (F062), PlayerBadge hiện danh hiệu
│   ├── progression/{LevelUpModal.tscn, JourneyScreen.tscn}            # + level_up_modal.gd, journey_screen.gd
│   ├── inventory/{InventoryShopScreen.tscn, ShopTab.tscn, ShopItemCard.tscn, PurchaseConfirmDialog.tscn,
│   │              LedgerTab.tscn, VirtualItemGrid.tscn, ItemDetailBar.tscn, FilterChips.tscn}   # nhúng CustomizationStudio (F070) làm tab Tủ đồ
│   ├── achievement/{AchievementCollectionScreen.tscn, AchievementRow.tscn, CollectionGrid.tscn, BadgeShowcaseEditor.tscn}
│   └── challenge/{ChallengeBoard.tscn, ChallengeInfoCard.tscn, DifficultySelector.tscn, ChallengeHud.tscn, ChallengeResult.tscn}
├── scripts/
│   ├── progression/{progression_service.gd, progression_store.gd, level_up_queue.gd, models/{progression_model.gd, xp_entry_model.gd, level_model.gd, reward_model.gd}}
│   ├── inventory/{inventory_service.gd (mở rộng F070), wallet_store.gd, models/{wallet_model.gd, ledger_entry_model.gd, shop_item_model.gd, inventory_item_model.gd}}
│   ├── achievement/{achievement_service.gd, models/{achievement_model.gd, collection_model.gd}}
│   ├── challenge/{challenge_client.gd, challenge_store.gd, models/{challenge_summary.gd, challenge_detail.gd, challenge_attempt_model.gd}}
│   └── core/{app_state.gd (thêm progression, wallet, blocking_activity), event_bus.gd (thêm signal level_up_shown, wallet_changed)}
├── locale/{vi.po,en.po}                        # ui.progression.*, ui.levelup.*, ui.journey.*, ui.inventory.*, ui.shop.*, ui.achievement.*, ui.collection.*, ui.challenge.*
└── tests/
    ├── unit/{progression,inventory,achievement,challenge}/test_*.gd
    ├── scene/{test_xp_bar.gd, test_level_up_modal.gd, test_inventory_shop_screen.gd, test_purchase_dialog.gd, test_achievement_screen.gd, test_challenge_flow.gd}
    ├── contract/test_f069_fixtures.gd
    ├── perf/{InventoryGridBenchmark.tscn, HudBenchmark.tscn}
    ├── integration/test_progression_shop_challenge_headless.gd
    └── fixtures/{progression,inventory,achievement,challenge}/*.json   # sinh từ example OpenAPI/AsyncAPI
```

**Structure Decision**: Theo [GODOT_ARCHITECTURE §3](../../docs/08-game-client/GODOT_ARCHITECTURE.md) (scene `PascalCase.tscn`, script `snake_case`); thêm thư mục `scenes/progression`, `scenes/inventory`, `scenes/achievement`, `scenes/challenge`, `scripts/progression`, `scripts/achievement`, `scripts/challenge` (cập nhật §3 ở task Polish). Store tách khỏi service để test áp message/response độc lập.

## Phase 0: Research

Không có — luồng và bố cục đã có ở STUDENT_GAME_UX_CORE S03/S06/S07, STUDENT_GAME_UX_SOCIAL §3–§4, contract F074–F076; quyết định ranh giới F069/F070/F110 và phím tắt ghi ở Clarifications.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | Model typed `from_dict` (bỏ trường lạ) — xem data-model | Không logic nghiệp vụ |
| application (client) | `ProgressionService` (`load()`, `load_history(cursor)`, `load_levels()`; xử lý `progression.xp.updated`, `progression.level.reached`) · `LevelUpQueue` (`push`, `flush_when_free`) · `InventoryService` (`load_inventory(kind)`, `load_wallet()`, `load_shop()`, `purchase(shop_item_id)`, `load_ledgers(cursor)`) · `AchievementService` (`load_achievements(category)`, `load_collections()`, `load_collection(id)`) · `ChallengeClient` (`list(filter)`, `get(id)`, `start(id, difficulty)`, `next()`, `abandon()`, `resume(active_attempt_id)`; xử lý `challenge.attempt.updated`) | Signal: `progression_changed(p: ProgressionModel)`, `level_up_ready(batch: Array[LevelUpModel])`, `wallet_changed(w: WalletModel)`, `purchase_completed(result: Dictionary)`, `purchase_failed(code: StringName)`, `challenge_attempt_changed(a: ChallengeAttemptModel)`, `challenge_finished(a: ChallengeAttemptModel)`, `request_failed(code: StringName)` |
| presentation (UI) | `XpBar`, `CurrencyChip`, `LevelUpModal`, `JourneyScreen`, `InventoryShopScreen` (+ tab), `PurchaseConfirmDialog`, `AchievementCollectionScreen`, `BadgeShowcaseEditor`, `ChallengeBoard`, `ChallengeInfoCard`, `DifficultySelector`, `ChallengeHud`, `ChallengeResult` | Chỉ đọc store, gọi method public |
| tích hợp | `RealtimeClient` dispatcher: `progression.*` → `ProgressionService`, `challenge.*` → `ChallengeClient`; `notification.item.created` (F123) → `InventoryService.load_wallet()`; `ActivityHost` (F063) mở bằng intent `activity_host` với `attempt` có sẵn; `ProfileService.update_avatar` (F070) | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F062 (HUD slot, `SceneRouter` intent), F063 (`ActivityHost`, `LearningService.resume`), F070 (`ProfileService`, `CustomizationStudio`, `InventoryService` bản đầu), F074/F075/F076 (API, message), F092 (`RealtimeClient`), F061 (`ApiClient`, `Clock`, luồng lỗi đăng nhập), F060 (Theme, i18n, Prism mock).
- Không gọi service nào ngoài API public; không kết nối NATS/Valkey (R25).

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key` UUIDv7 sinh mới mỗi ý định cho `purchaseShopItem`, `startGameChallenge`, `nextChallengeActivity` (theo số thứ tự hoạt động), `abandonGameChallengeAttempt`; giữ nguyên khi retry; không tự retry lệnh ghi không có key. Audit do backend (F075/F076).
- Log client không chứa nickname/ID học sinh ở mức INFO (R14); telemetry `client_bug` chỉ có `operationId`, `code`, `trace_id`.

### Quan sát

- Telemetry client (F080 khi có): `shop_open`, `purchase_confirm`, `challenge_start`, `challenge_finish` (status), thời gian mở panel; không PII. `traceparent` do `ApiClient` gắn.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

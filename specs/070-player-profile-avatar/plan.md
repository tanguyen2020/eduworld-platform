# Implementation Plan: F070 — Hồ sơ người chơi, nickname, tạo avatar lần đầu, trang phục, emote

**Branch**: `070-player-profile-avatar` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

module game thêm aggregate `GameProfile` (nickname, `AvatarConfig`, `avatar_created_at`) và `CharacterCatalog` (nội dung platform có version). `EnsureGameProfile` chạy trong transaction `StartGameSession` (F014). `UpdateAvatar` kiểm sở hữu (item mặc định của danh mục hoặc `game.inventory_items`) và slot theo `ItemDefinition.equip_slot`, tính `avatar_config_hash`, ghi outbox `avatar.created` (lần đầu) / `avatar.updated` + `game_profile.updated`. Nickname: `SuggestNicknames` sinh từ bộ từ danh mục, loại tên đã dùng trong tenant, giữ option trong Valkey có TTL; `ChangeNickname` dựa unique partial index `(tenant_id, lower(nickname))`. Godot: màn S02 `CharacterCreation`, `CustomizationStudio` (Tủ đồ), `EmoteWheel`, `avatar_builder.gd` dùng chung.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 · GDScript (Godot 4.x, static typing) |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox`, `src/backend/pkg/httpx` (`game_player`, `tenant`), `src/backend/pkg/tenancy`, Valkey client · GdUnit4 |
| Storage | PostgreSQL `eduworld_game` (`game.game_profiles`, `game.avatar_presets`, `game.nickname_changes`, `game.character_catalogs`/`_versions`, `game.item_definitions`/`_versions`, `game.inventory_items`); Valkey `ew:game:nick:opt:<student_id>` (tạm) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); GdUnit4 headless |
| Target Platform | Linux container (`core-api` + `worker`, module `game`) · Android, iOS, Web (ADR-0101) |
| Project Type | backend module + game client |
| Performance Goals | S02 đổi lựa chọn ≤ 100 ms; avatar trong ngân sách nhân vật [PERFORMANCE_BUDGET §5, §8](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |
| Constraints | AGENTS R1, R2, R8, R13, R16, R19, R21, R23, R26, R28, R32; ADR-0031, ADR-0043, ADR-0047, ADR-0054, ADR-0056 |
| Scale/Scope | 1 profile/học sinh/tenant; `avatar_creation_presets`, `saved_outfits_max`, `emote_wheel_slots` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Sửa `AvatarConfig`, `GameProfile`, thêm `getCharacterCatalog`, ProblemCode, payload event ở Phase 2 trước code |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module game; không gọi module khác; command (write context, `UnitOfWork` + outbox) và query (read context, không ghi) tách (ADR-0087) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng T có `tenant_id` + `deleted_at` + RLS; `nickname_changes` TA; test isolation mọi endpoint |
| IV Server-authoritative | ✅ | Sở hữu item, slot, `avatar_created_at`, nickname do server quyết; client chỉ hiển thị |
| V Xác thực, phân quyền | ✅ | `game_player` mọi route; không route nhân sự mới |
| VI An toàn trẻ em | ✅ | Không nhập tên tự do; payload event/response không họ tên/trường/lớp; không log PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Danh mục là dữ liệu version; tham số theo CONFIG_DEFAULTS (2 tên mới đăng ký ở Polish); chuỗi có `MaxLen` |
| VIII UI tách logic, token | ✅ | Scene chỉ gọi `ProfileService`; Theme từ `design/game-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F070-AC có task test; test authz/isolation |

## Project Structure

### Documentation (feature này)

```text
specs/070-player-profile-avatar/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/game/
├── ent/schema/                       # gameprofile.go, avatarpreset.go, nicknamechange.go, charactercatalog*.go, itemdefinition*.go, inventoryitem.go
├── internal/domain/profile/          # GameProfile, AvatarConfig (validate slot/ownership), Nickname, NicknameGenerator
├── internal/domain/catalog/          # CharacterCatalog (presets, palette, default items, nickname parts)
├── internal/application/command/     # ensure_game_profile.go, update_avatar.go, change_nickname.go, save_avatar_preset.go
├── internal/application/query/       # get_game_profile.go, get_character_catalog.go, suggest_nicknames.go, list_avatar_presets.go
├── internal/application/port/        # ProfileRepository, CatalogReader, OwnershipReader, NicknameOptionStore, Clock
├── internal/infrastructure/persistence/  # ent repos
├── internal/infrastructure/cache/nickname_option_store.go   # Valkey
├── internal/presentation/http/public/profile.go             # handlers operationId
└── tests/{unit,integration,contract}/
src/backend/modules/game/seed/character_catalog.v1.json         # danh mục v1 (vi/en)
src/game/
├── scenes/character/{CharacterCreation.tscn, CustomizationStudio.tscn, EmoteWheel.tscn, AvatarRig.tscn}
├── scripts/player/{profile_service.gd, avatar_builder.gd, anim_driver.gd, emote_wheel.gd, character_creation.gd}
├── scripts/inventory/inventory_service.gd                   # đọc danh mục + sở hữu (cache)
└── tests/unit/player/, tests/fixtures/profile/
```

**Structure Decision**: module game theo BACKEND_DESIGN §3 và skeleton F014; Godot theo [GODOT_ARCHITECTURE §3, §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md). `ProfileService` là `RefCounted` tạo bởi `ServiceLocator`.

## Phase 0: Research

Không có — mô hình dữ liệu ở CHARACTER_SYSTEM §5, INVENTORY_ECONOMY §2–§3; quyết định Clarifications ghi trong spec.md.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `GameProfile` (SetAvatar, ChangeNickname, NeedsSetup), `AvatarConfig.Validate(catalog, owned, slots)`, `NicknameGenerator` (tính từ + danh từ + 2 số, ≤ 24 ký tự) | Thuần, Clock/Rand inject |
| application | Command `EnsureGameProfile`, `UpdateAvatar`, `ChangeNickname`, `SaveAvatarPreset` · Query `GetGameProfile`, `GetCharacterCatalog`, `SuggestNicknames`, `ListAvatarPresets` | Command ở `command/`, query ở `query/`, presentation gọi qua interface có kiểu; authz `game_player` |
| infrastructure | ent repo (TenantScope), `NicknameOptionStore` Valkey (TTL `nickname_suggestion_ttl`), `OwnershipReader` (`inventory_items` + default items) | |
| presentation | `getGameProfile`, `updateGameProfile`, `suggestNicknames`, `updateAvatar`, `listAvatarPresets`, `saveAvatarPreset`, `getCharacterCatalog` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F014 (`StartGameSession`, `ref.students`), F062 (Player, `anim_driver.gd`, HUD), F005 (Theme), F061 (bootstrap → S02).
- Event phát: `avatar.created`, `avatar.updated`, `game_profile.updated` (→ `ref.player_profiles` ở competition/chat, analytics).
- Không facade mới.

### Bảo mật, tenant, idempotency, audit

- `game_player` (`azp = eduworld-game-backend`, `user_type = student`); `TenantScope` từ Principal; header `X-Tenant-Id` kiểm chéo.
- `updateAvatar`/`saveAvatarPreset` là PUT idempotent; `updateGameProfile` đổi nickname ghi `game.nickname_changes` (vết kiểm tra) — không phải thao tác R10 nên không bắt buộc `Idempotency-Key`.

### Quan sát

- Metric `eduworld_game_avatar_created_total`, `eduworld_game_avatar_update_rejected_total{reason}`, `eduworld_game_nickname_conflict_total`; span `profile.update_avatar`. Log không chứa nickname kèm `student_id` ở INFO.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

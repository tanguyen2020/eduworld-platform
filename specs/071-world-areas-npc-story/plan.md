# Implementation Plan: F071 — Thế giới: 8 vùng đất, zone, world map, cổng, mở khóa, mạch truyện, NPC và hội thoại

**Branch**: `071-world-areas-npc-story` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

module game thêm (1) **lõi nội dung game có version** dùng chung cho F071–F078: mixin ent `VersionedDefinition`/`VersionedContent`, `game.content_releases`, command nội bộ `PublishGameContent` (dựng snapshot Valkey `ew:game:cv:<type>:<id>:<v>` + object storage, cập nhật con trỏ, rebuild `game.content_manifests` cho tenant liên quan, outbox `content.published`), lệnh seed gọi command này; (2) module **world**: `WorldArea`, `StoryChapter`, `NpcDialogue`, `LevelPackage` (G, version), `LevelSession`, `AreaUnlock`, `ExplorationProgress`, `StoryProgress` (T); `UnlockEvaluator` thuần chạy trong consumer event; `InstanceAllocator` v0 + `PresenceStore` Valkey; projection `ref.mastery_summary`. Godot: `WorldService`, `SceneRouter` chuyển zone theo WORLD_RENDERING §3, `AreaPortal`, `NPC` + `DialogueRunner`, World Map S04, zone scenes 12 `area_id`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 · GDScript (Godot 4.x) |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox`, `src/backend/pkg/idempotency`, `src/backend/pkg/httpx`, `src/backend/pkg/tenancy`, nats.go, Valkey client, S3 client (floci) · GdUnit4 |
| Storage | PostgreSQL `eduworld_game` (bảng ở [data-model.md](data-model.md)); Valkey snapshot + presence (tạm); floci bucket snapshot/bundle |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey, floci); GdUnit4 headless; benchmark zone headless (TC-PERF-018) |
| Target Platform | Linux container (`core-api` + `worker`) · Windows, Android, iOS, Web |
| Project Type | backend module + game client |
| Performance Goals | `area_transition_max`; zone theo [PERFORMANCE_BUDGET §11](../../docs/08-game-client/PERFORMANCE_BUDGET.md); `listWorldAreas`/`getWorldExploration` p95 theo `api_rate_default` tải chuẩn |
| Constraints | AGENTS R2, R5, R8, R13, R21, R23, R26, R27, R29, R31, R32; ADR-0012, ADR-0016, ADR-0043, ADR-0047, ADR-0054, ADR-0056, ADR-0057 |
| Scale/Scope | 8 vùng / 12 area / 7 chương / 13 NPC seed; `hub_instance_capacity` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Schema `world-area`, `story-chapter`, `level-package`, payload event, sửa `enterWorldArea`/`getNpcDialogue` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Mastery qua event → `ref.mastery_summary`; quest/level/achievement qua event; không gọi đồng bộ module khác |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng T có RLS + soft-delete; nội dung G platform; test isolation |
| IV Server-authoritative | ✅ | Unlock, khám phá, chương do server; `reportDiscovery`/`startLevel` có `Idempotency-Key`; version ghim |
| V Xác thực, phân quyền | ✅ | `game_player` mọi route; `PublishGameContent` chỉ nội bộ (seed), API platform ở F078 |
| VI An toàn trẻ em | ✅ | Instance không trộn trường; World Map chỉ số bạn (F095); không PII trong event |
| VII Một nguồn sự thật, cấu hình | ✅ | Tên vùng/NPC/chương là dữ liệu; tham số theo CONFIG_DEFAULTS; `MaxLen` |
| VIII UI tách logic, token | ✅ | Scene gọi `WorldService`; Theme/material từ token |
| IX Test có bằng chứng | ✅ | Mỗi F071-AC có task test; allocator v0 gắn nhãn, không phải mock trong đường thật |

## Project Structure

### Documentation (feature này)

```text
specs/071-world-areas-npc-story/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/game/
├── ent/schema/                         # mixin versioned.go; worldarea*.go, storychapter*.go, npcdialogue*.go, level*.go,
│                                       # levelsession.go, areaunlock.go, explorationprogress.go, storyprogress.go,
│                                       # contentrelease.go, refmasterysummary.go, refschool.go
├── internal/domain/content/            # VersionedItem, ReleasePointer, ManifestEntry
├── internal/domain/world/              # WorldArea, UnlockRule + UnlockEvaluator, Exploration, StoryChapterRules, InstanceKey
├── internal/application/command/       # publish_game_content.go, enter_world_area.go, start_level.go, end_level_session.go,
│                                       # report_discovery.go, evaluate_area_unlocks.go, update_story_progress.go
├── internal/application/query/         # get_world_manifest.go, list_world_areas.go, get_world_exploration.go, get_npc_dialogue.go
├── internal/application/port/          # ContentStore, SnapshotStore, ManifestBuilder, InstanceAllocator, PresenceStore, MasteryReader
├── internal/infrastructure/content/    # snapshot_valkey_s3.go, manifest_builder.go
├── internal/infrastructure/world/      # instance_allocator_v0.go (NHÃN: allocator tạm, F091 thay), presence_valkey.go
├── internal/presentation/http/public/world.go
├── internal/presentation/consumer/     # unlock_on_events.go, story_on_quest_completed.go, mastery_projection.go, school_projection.go
├── cmd/seed/                                       # seed qua PublishGameContent
└── seed/world/                         # areas.v1.json, story_chapters.v1.json, npc_dialogues/*.json, levels.v1.json
src/game/
├── scenes/world/{WorldRoot.tscn, AreaPortal.tscn, WorldMap.tscn, zones/<area>/Zone.tscn}
├── scenes/npc/{NPC.tscn, DialogueBubble.tscn}
├── scripts/world/{world_service.gd, zone_loader.gd, area_portal.gd, spawn_point.gd, interactable.gd, dialogue_runner.gd, world_map.gd}
├── scripts/core/scene_router.gd        # thêm request_area()
└── tests/unit/world/, tools/zone_lint.gd
```

**Structure Decision**: Lõi phát hành đặt ở `internal/domain/content` vì quest (F072), mini-game (F073), item/reward (F074–F075) dùng lại; module `world` theo WORLD_DESIGN. Godot theo [GODOT_ARCHITECTURE §3](../../docs/08-game-client/GODOT_ARCHITECTURE.md#3-cấu-trúc-thư-mục-srcgame) và [WORLD_RENDERING §2–§5](../../docs/08-game-client/WORLD_RENDERING.md).

## Phase 0: Research

Không có — thuật toán phát hành ở ADR-0043/DATABASE_DESIGN §3.13, unlock ở WORLD_DESIGN §7, chuyển zone ở WORLD_RENDERING §3. Hai quyết định tạm (allocator v0, presence Valkey) ghi ở Clarifications của spec.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `UnlockRule.Evaluate(facts) (met, missing[])`; `WorldArea.IsLearningArea()`; `ExplorationProgress.Record(id, kind, totals)`; `StoryChapterRules.Status(completedChapters)`; `InstanceKey(tenant, school, area, grade_band)` | Thuần, không I/O |
| application | Command `PublishGameContent` (nội bộ), `EnterWorldArea`, `StartLevel`, `EndLevelSession`, `ReportDiscovery`, `EvaluateAreaUnlocks`, `UpdateStoryProgress` · Query `GetWorldManifest`, `ListWorldAreas`, `GetWorldExploration`, `GetNpcDialogue` | Command/query tách (ADR-0087); `Idempotency-Key` (middleware `src/backend/pkg/httpx`) cho `StartLevel`, `EndLevelSession`, `ReportDiscovery` |
| infrastructure | ent repo; `SnapshotStore` (Valkey + floci, khóa bất biến); `ManifestBuilder`; `InstanceAllocatorV0`; `PresenceStore` Valkey `ew:game:presence:<student_id>`; publisher realtime Valkey `ew:rt:user:<id>` | |
| presentation | `getWorldManifest`, `listWorldAreas`, `enterWorldArea`, `startLevel`, `endLevelSession`, `getNpcDialogue`, `getWorldExploration`, `reportDiscovery` · consumer `game.unlock_on_events`, `game.story_on_quest_completed`, `game.mastery_projection`, `game.school_projection` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F026 (mẫu phát hành), F070 (`game_profiles.last_area_id`), F014 (game session), F062 (Player, `Interactable`, `SceneRouter`, `QualityManager`).
- Event phát: `content.published` (game), `world.area_entered`, `world.area_unlocked`, `world.discovery_found`, `world.area_completed`, `story.chapter_completed`.
- Event tiêu thụ: `quest.completed` (F072), `level.reached` (F074), `achievement.unlocked` (F075), `event.participation_started` (F077), `learning_objective.progressed` (learning), `school.updated` (identity).
- Realtime: `world.area.unlocked`, `content.manifest.updated` qua Valkey `ew:rt:user:<id>` / `ew:rt:tenant:<id>`.

### Bảo mật, tenant, idempotency, audit

- `game_player` cho mọi route; `TenantScope`; nội dung G đọc theo tenant (world/NPC/chương áp cho mọi tenant).
- `Idempotency-Key`: `startLevel`, `endLevelSession`, `reportDiscovery`; publish ghi `content_releases` (append-only) + audit `game_content.published`.

### Quan sát

- Metric `eduworld_game_area_enter_total{result}`, `eduworld_game_area_unlocked_total`, `eduworld_game_discovery_total{result}`, `eduworld_game_manifest_build_seconds`, `eduworld_projection_lag_seconds{table="mastery_summary"}`; span `world.enter_area`, `content.publish`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

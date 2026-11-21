# Implementation Plan: F078 — Game Operations Console backend: workflow nội dung game, thumbnail 3D, catalog cho nhà trường

**Branch**: `078-game-ops-console-backend` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Mở HTTP `/api/v1/game-admin/*` và `/api/v1/game-media*` trên module game (gateway nội bộ) quanh aggregate `GameConfigVersion` (BACKEND_DESIGN §3.1). Middleware `platform_staff` (azp `eduworld-portal`, role platform) + bảng quyền theo `config_type`. Command `CreateGameConfig`, `CreateGameConfigVersion`, `UpdateGameConfigDraft`, `TransitionGameConfigVersion`, `PublishGameConfig` (ngay/hẹn giờ), `RollbackGameConfig`, `UnpublishGameConfig`, `CreatePreviewSession`, `PresignGameMedia` dùng `ConfigTypeRegistry` (module của F071–F077) để validate và `OnPublished`. Job `scheduled-publish`, consumer `game.render_thumbnails` + container `game-renderer`. Internal `applyTenantAssignment` (gọi từ module identity) và `listAssignedGameActivities` (gọi từ module learning) cho catalog nhà trường.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24; GDScript (Godot 4.x headless) cho script render |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server public + internal), `src/backend/pkg/httpx` (`platform_staff`), `src/backend/pkg/authz`, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/idempotency`, `src/backend/pkg/outbox`, `src/backend/pkg/natsx`, AWS SDK v2 S3 (floci), jsonschema validator, diff JSON |
| Storage | PostgreSQL `eduworld_game`: bảng item/version của từng loại (F071–F077), `game.content_releases`, `game.content_manifests`, `game.client_manifests`, `game.media_assets`, `game.media_usages`, **Mới** `game.preview_sessions`, `game.render_jobs`; `infra.tenant_resource_assignments`; Valkey `ew:game:cv:*`, `ew:game:current:*`, `ew:game:manifest:*`; object storage `ew-media-public` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey, floci); facade test với identity, learning; test sinh từ OpenAPI cho 403; GdUnit4 headless cho script render |
| Target Platform | Linux container (`core-api` + `worker`, module `game`; container `game-renderer`) |
| Project Type | backend module |
| Performance Goals | `publish_to_visible_max`; thumbnail ≤ 10 phút (SC-004) |
| Constraints | AGENTS R7, R11, R13, R20, R22, R26, R27, R29, R31; ADR-0027, ADR-0029, ADR-0043, ADR-0044, ADR-0047, ADR-0056 |
| Scale/Scope | 18 loại cấu hình của module game; `assets_per_version_max` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Operation mới (`createGameConfigVersion_game`, `cancelScheduledGameConfigPublish_game`, internal `applyTenantAssignment`, `listAssignedGameActivities`), `MediaKind` render, payload `content.*` game ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Catalog qua event + internal API (research R1); renderer không ghi DB; module loại nằm ở feature domain |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Nội dung `tenant_id NULL`; gán qua `tenant_resource_assignments` soft-delete; manifest theo tenant |
| IV Server-authoritative | ✅ | Version bất biến, ghim lượt chơi, `Idempotency-Key` + audit mọi thao tác ghi |
| V Xác thực, phân quyền | ✅ | `platform_staff` + role platform + quyền theo loại; reviewer ≠ author; internal `x-allowed-callers` |
| VI An toàn trẻ em | ✅ | Preview không tạo game session/XP; không dữ liệu học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số CONFIG_DEFAULTS; validate theo JSON Schema `specs/schemas` |
| VIII UI tách logic, token | ✅ | Không có UI web; thumbnail là dữ liệu nhận diện (ADR-0047) |
| IX Test có bằng chứng | ✅ | Test mỗi F078-AC-*, test 403 sinh từ OpenAPI |

## Project Structure

### Documentation (feature này)

```text
specs/078-game-ops-console-backend/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/game/
├── ent/schema/                     # previewsession.go, renderjob.go (+ mixin release cho bảng item/version nếu thiếu)
├── internal/domain/gameconfig/     # Workflow (state machine, reviewer ≠ author), PublishPolicy (hẹn giờ), RollbackPolicy, RefCheck
├── internal/application/command/   # create_game_config.go, create_game_config_version.go, update_game_config_draft.go,
│                                   # transition_game_config.go, publish_game_config.go, rollback_game_config.go,
│                                   # unpublish_game_config.go, cancel_scheduled_publish.go, create_preview_session.go,
│                                   # presign_game_media.go, apply_tenant_assignment.go, record_render_result.go
├── internal/application/query/     # list_game_configs.go, get_game_config_version.go, list_versions.go, diff_versions.go,
│                                   # list_releases.go, get_game_media.go, list_assigned_game_activities.go
├── internal/infrastructure/storage/ # presign S3 (floci)
├── internal/infrastructure/render/  # publisher job NATS work-queue, result consumer
├── internal/presentation/http/public/   # game_admin.go, game_media.go, gameadmin_permissions.go
├── internal/presentation/http/internal/ # tenant_assignments.go, game_activities.go
├── internal/presentation/consumer/      # render_thumbnails.go, render_results.go
├── internal/presentation/job/           # scheduled_publish.go
└── tests/{unit,integration,contract}/
deploy/compose/game-renderer.yml        # container Godot headless
src/game/tools/thumbnail_renderer/render_thumbnails.gd
deploy/apisix/                           # route nội bộ /game-admin/*, /game-media* (không có trên gateway public)
```

**Structure Decision**: Workflow chung một bộ handler cho mọi `config_type` (đường dẫn generic trong OpenAPI); khác biệt theo loại nằm trong module đăng ký. Renderer tách container (research R2).

## Phase 0: Research

→ [research.md](research.md): R1 catalog qua event + internal API; R2 render qua work-queue + container Godot; R3 bảng quyền theo loại.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `gameconfig.Workflow` (`draft → in_review → approved/rejected → published → archived`), `PublishPolicy`, `RollbackPolicy`, `RefCheck` (client manifest, tham chiếu đã publish, song ngữ) | Thuần |
| application | Command/Query như cây trên; permission theo bảng dưới | authz `authz.Require` tường minh đầu handler; idempotency middleware `src/backend/pkg/httpx`; validation trong DTO/handler |
| infrastructure | Repo generic theo `ConfigTypeRegistry`; snapshot Valkey; manifest rebuilder; S3 presign; NATS render job | |
| presentation | 15 operation `/game-admin/*` + `/game-media*` của game.v1; internal `applyTenantAssignment`, `listAssignedGameActivities`; consumer render; job hẹn giờ | |

Bảng quyền ghi theo `config_type` (đọc: quyền ghi của loại hoặc `game.content.review`/`game.content.publish`; duyệt: `game.content.review`; publish/rollback/unpublish/hủy hẹn giờ: `game.content.publish`):

| `config_type` | Permission ghi |
|---|---|
| `quests` | `game.quest.write` |
| `minigame-configs` | `game.minigame.write` |
| `npc-dialogues` | `game.npc.write` |
| `world-areas`, `story-chapters` | `game.world.write` |
| `items` | `game.item.write` |
| `shop-items` | `game.shop.write` |
| `collections` | `game.collection.write` |
| `reward-rules` | `game.reward_rule.write` |
| `progression-curves`, `progression-levels` | `game.progression.write` |
| `challenges` | `game.challenge.write` |
| `bosses` | `game.boss.write` |
| `achievements` | `game.achievement.write` |
| `currencies` | `game.economy.write` |
| `events`, `event-templates` | `game.event.write` |
| `season-tracks` | `game.season_track.write` |
| media (`presignGameMediaUpload`) | bất kỳ quyền ghi `game.*` ở trên |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F071 (kernel, client manifest, content manifest), F072–F077 (module loại), F014 (`game.content_manifests`), F045 (identity gọi `applyTenantAssignment`), F041 (learning dựng projection catalog), F004 (route APISIX nội bộ).
- Event phát: `content.version_submitted`, `content.version_approved`, `content.version_rejected`, `content.published`, `content.rolled_back`, `content.unpublished`, `content.publish_scheduled`, `content.version_archived`, `content.manifest_updated` (schema F026), `media.uploaded`, `audit.recorded`.
- Event tiêu thụ: `media.processed`, `media.rejected`, kết quả render.

### Bảo mật, tenant, idempotency, audit

- `platform_staff` (azp `eduworld-portal`, role platform) trên mọi route; facade nội bộ: caller theo `x-allowed-callers` (`make arch-check`).
- Ghi dữ liệu platform không theo tenant; manifest/assignment theo tenant đích.
- `Idempotency-Key` cho mọi POST ghi; `If-Match` cho update/transition/publish.
- Audit theo ADMIN_PORTAL §7; `authz.denied` cho mọi từ chối.

### Quan sát

- Metric `eduworld_game_config_transition_total{config_type,action}`, `eduworld_game_publish_to_visible_seconds`, `eduworld_game_render_job_total{result}`, `eduworld_game_render_duration_seconds`, `eduworld_game_authz_denied_total{route}`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

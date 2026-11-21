# Implementation Plan: F079 — Workspace A — Game Operations Console UI (world, NPC, quest builder, reward, item, event, season)

**Branch**: `079-game-ops-console-ui` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Xây chunk workspace `game-ops` (F051) gọi API `/api/v1/game-admin/*` qua gateway nội bộ + role platform. Lõi là `ContentEditorShell` (ops-ui) dùng chung cho mọi `GameConfigType` thuộc module game: danh sách, form sinh từ JSON Schema, autosave `If-Match`, workflow, version/diff/release, rollback. Trên lõi có builder đặc thù: `QuestStepEditor`, `DialogueGraphEditor`, `ProgressionCurveEditor` (biểu đồ + mô phỏng server), `EventWizard` 8 bước + `EventCalendar`, `SeasonTrackEditor`, `EventTemplateEditor`; `ThumbnailImage`/`TurntableViewer` và `GamePreviewFrame`. Bốn delta contract (owner F078/F074/F077) đưa vào Phase 2.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript strict (React 18, Vite) |
| Primary Dependencies | Ant Design 5 + Pro (ProTable, ProForm, StepsForm, Calendar), TanStack Query, openapi-fetch (F050), `@rjsf/antd` hoặc form sinh từ JSON Schema [Confirmed], React Flow (graph quest/NPC) [Confirmed], chart theo DESIGN_SYSTEM §4.4, i18next |
| Storage | Không (RAM; autosave lên server) |
| Testing | Vitest + Testing Library + MSW (test-only, nhãn `MOCK`), Playwright E2E với backend compose (gateway nội bộ local), axe, visual regression |
| Target Platform | Desktop ≥ 1280 px cho builder; tablet/điện thoại chỉ xem |
| Project Type | web app (workspace A) + component `src/cms/frontend/packages/ops-ui` |
| Performance Goals | Danh sách ≤ 2 s trang đầu; autosave mỗi 10 s (ADMIN_PORTAL §1.4) |
| Constraints | AGENTS R1, R2, R3, R10, R11, R19, R20, R32; ADR-0027, ADR-0043, ADR-0047, ADR-0056, ADR-0057 |
| Scale/Scope | 21 loại cấu hình module game; payload ≤ giới hạn schema |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `GameConfigVersion.media`, `simulateGameConfig`, `getGameClientManifest`, `getGameConfigSchema`, tham số `listGameConfigs` là task Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ gọi API qua APISIX nội bộ; không logic nghiệp vụ ở client |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Dữ liệu platform (`tenant_id = NULL`); không có xóa vĩnh viễn; version publish chỉ đọc |
| IV Server-authoritative | ✅ | Validate nghiệp vụ, mô phỏng, cảnh báo lịch do server |
| V Xác thực, phân quyền | ✅ | Menu theo permission; 403; 403 kèm `trace_id` (không step-up — ADR-0094); E2E role tenant |
| VI An toàn trẻ em | ✅ | Không trường tiền thật/loot box; không dark pattern (cảnh báo > 3 phiên/tuần) |
| VII Một nguồn sự thật, cấu hình | ✅ | Form sinh từ JSON Schema; giới hạn từ OpenAPI; tham số theo tên |
| VIII UI tách logic, token | ✅ | Theme `admin-tokens`; màu trạng thái/loại sự kiện từ `shared-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F079-AC có test viết trước |

## Project Structure

### Documentation (feature này)

```text
specs/079-game-ops-console-ui/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/cms/frontend/packages/ops-ui/src/
├── content-editor/{ContentEditorShell.tsx,SchemaForm.tsx,VersionTab.tsx,DiffView.tsx,WorkflowBar.tsx,useAutosave.ts}
├── game/{GamePreviewFrame.tsx,ThumbnailImage.tsx,TurntableViewer.tsx,ManifestRefPicker.tsx}
└── *.stories.tsx
src/cms/frontend/apps/portal/src/workspaces/game-ops/
├── routes.tsx                     # /game-ops/{:configType,:configType/:configId/v/:version,events/calendar,progression}
├── registry/configTypes.ts        # config_type → permission, builder, schema id (chỉ metadata hiển thị)
├── api/{gameConfigs.ts,preview.ts,manifest.ts,simulate.ts,schemas.ts}
├── list/ConfigListPage.tsx
├── builders/
│   ├── quest/QuestStepEditor.tsx
│   ├── npc/DialogueGraphEditor.tsx
│   ├── progression/{ProgressionCurveEditor.tsx,SimulationPanel.tsx}
│   ├── event/{EventWizard.tsx,steps/*.tsx,EventCalendarPage.tsx,EventTemplateEditor.tsx}
│   ├── season/SeasonTrackEditor.tsx
│   └── generic/GenericSchemaBuilder.tsx       # reward-rules, items, shop-items, currencies, world-areas…
├── i18n/{vi,en}.json
└── __tests__/
src/cms/frontend/apps/portal/tests/e2e/game-ops/*.spec.ts
```

**Structure Decision**: Khung editor và thành phần game-media ở `ops-ui` để F108 (thi đấu), sticker/GIF và Learning Content Studio dùng lại (ADMIN_PORTAL §1.4); builder đặc thù trong chunk `game-ops` (ADR-0027 mục 7).

## Phase 0: Research

Không có quyết định cần nghiên cứu riêng; thư viện form schema và graph là lựa chọn hiện thực [Confirmed], không đổi contract.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — | Không có |
| application | Hook `api/*.ts`, `useAutosave` | Mutation sinh `Idempotency-Key` một lần/thao tác; autosave dừng khi 409/412 |
| infrastructure | `apiClient` (F050), base URL `VITE_API_INTERNAL_BASE_URL` | Không hardcode URL (R3) |
| presentation | Page gọi `listGameConfigs`, `createGameConfig`, `getGameConfigVersion`, `updateGameConfigDraft`, `transitionGameConfigVersion`, `listGameConfigVersions`, `diffGameConfigVersions`, `publishGameConfigVersion`, `rollbackGameConfig`, `unpublishGameConfig`, `listGameConfigReleases`, `createGamePreviewSession`, `presignGameMediaUpload`, `getGameMediaAsset`; sau delta: `simulateGameConfig`, `getGameClientManifest`, `getGameConfigSchema` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F051 (shell, route, step-up, VPN), F050 (`apiClient`, theme), F078 (workflow, thumbnail, manifest), F077 (event/season-track), F074 (progression-curves), F057 (hộp gán tenant).
- Dùng bởi: F080 (link từ panel analytics mở item), F108 (dùng `ContentEditorShell`).

### Bảo mật, tenant, idempotency, audit

- Permission theo `registry/configTypes.ts` (chỉ để ẩn/hiện): `game.quest.write`, `game.minigame.write`, `game.npc.write`, `game.world.write`, `game.event.write`, `game.season_track.write`, `game.item.write`, `game.shop.write`, `game.collection.write`, `game.reward_rule.write`, `game.progression.write`, `game.challenge.write`, `game.boss.write`, `game.achievement.write`, `game.economy.write`, `game.content.review`, `game.content.publish`, `content.release.rollback`.
- Idempotency-Key: `createGameConfig`, `transitionGameConfigVersion`, `publishGameConfigVersion`, `rollbackGameConfig`, `unpublishGameConfig`, `createGamePreviewSession`, `presignGameMediaUpload`; `If-Match` cho `updateGameConfigDraft`. Audit ở backend.

### Quan sát

- Log client: `code`, `trace_id`, `config_type`, route; không payload nội dung.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.game_ops.world`, `menu.game_ops.npcs`, `menu.game_ops.quests`, `menu.game_ops.minigames`, `menu.game_ops.progression`, `menu.game_ops.items`, `menu.game_ops.shop`, `menu.game_ops.rewards`, `menu.game_ops.events`, `menu.game_ops.review`, `menu.game_ops.media`, `menu.game_ops.dashboard` |
| Action dùng | `action.game_ops.world.view`, `action.game_ops.world.edit`, `action.game_ops.world.workflow`, `action.game_ops.world.publish`, `action.game_ops.world.rollback`, `action.game_ops.npcs.view`, `action.game_ops.npcs.edit`, `action.game_ops.npcs.workflow`, `action.game_ops.npcs.publish`, `action.game_ops.npcs.rollback`, `action.game_ops.quests.view`, `action.game_ops.quests.edit`, `action.game_ops.quests.workflow`, `action.game_ops.quests.publish`, `action.game_ops.quests.rollback`, `action.game_ops.minigames.view`, `action.game_ops.minigames.edit`, `action.game_ops.minigames.workflow`, `action.game_ops.minigames.publish`, `action.game_ops.minigames.rollback`, `action.game_ops.progression.view`, `action.game_ops.progression.edit`, `action.game_ops.progression.workflow`, `action.game_ops.progression.publish`, `action.game_ops.progression.rollback`, `action.game_ops.items.view`, `action.game_ops.items.edit`, `action.game_ops.items.workflow`, `action.game_ops.items.publish`, `action.game_ops.items.rollback`, `action.game_ops.shop.view`, `action.game_ops.shop.edit`, `action.game_ops.shop.workflow`, `action.game_ops.shop.publish`, `action.game_ops.shop.rollback`, `action.game_ops.rewards.view`, `action.game_ops.rewards.edit`, `action.game_ops.rewards.workflow`, `action.game_ops.rewards.publish`, `action.game_ops.rewards.rollback`, `action.game_ops.events.view`, `action.game_ops.events.edit`, `action.game_ops.events.workflow`, `action.game_ops.events.publish`, `action.game_ops.events.rollback`, `action.game_ops.review.view`, `action.game_ops.review.preview`, `action.game_ops.media.view`, `action.game_ops.media.upload`, `action.game_ops.dashboard.view` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `getGameClientManifest`, `getGameConfigSchema` (Cần thêm) → cần `x-portal-action` (đề xuất `action.game_ops.review.view` hoặc `x-portal-shell`)

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

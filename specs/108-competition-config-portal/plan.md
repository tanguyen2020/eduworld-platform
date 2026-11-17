# Implementation Plan: F108 — Cấu hình thi đấu `/game-admin/*` và UI workspace A

**Branch**: `108-competition-config-portal` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Hai phần, backend trước. (1) **module competition**: phía ghi của bảng cấu hình `game_configs`/`game_config_versions`/`content_releases` (F104 đã tạo, chỉ đọc): command tạo/sửa nháp, transition workflow, publish có `effective_from`, rollback/unpublish; validator theo JSON Schema (7 schema mới) + invariant domain; outbox `competition_config.published`; consumer áp runtime (mùa, leaderboard definition — dùng command của F106). Router `/game-admin/*` gắn middleware `platform_admin` (azp `eduworld-portal`, MFA `acr`, role platform) chỉ ở listener gateway nội bộ. (2) **src/cms/frontend/apps/portal**: module `competition` trong chunk `game-ops` (workspace A) và module `challenges` trong chunk `school-lms` (workspace C), gọi API qua `apiClient` (F050) với hook TanStack Query; không business logic.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 / TypeScript (React, Vite) |
| Primary Dependencies | chi, ent, oapi-codegen, `santhosh-tekuri/jsonschema` (validate payload), `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/authz`, `src/backend/pkg/outbox` / Ant Design 5 + Pro, `ops-ui` (`ContentEditorShell`), TanStack Query, React Router, React Hook Form + Zod/JSON Schema form, openapi-typescript, i18next |
| Storage | PostgreSQL `eduworld_competition` (`competition.game_configs`, `game_config_versions`, `content_releases`) |
| Testing | go test + testcontainers; Vitest + Testing Library + MSW (handler sinh từ OpenAPI, nhãn `MOCK` chỉ trong test); Playwright E2E với backend compose; axe |
| Target Platform | Linux container; trình duyệt desktop (editor), tablet/điện thoại xem |
| Project Type | backend module + web app |
| Performance Goals | API `/game-admin/*` theo baseline [SYSTEM_ARCHITECTURE §7](../../docs/06-architecture/SYSTEM_ARCHITECTURE.md#7-non-functional-baseline); trang danh sách < 2 s |
| Constraints | AGENTS R1, R7, R10, R11, R19, R20, R26–R28, R31, R32 · ADR-0027, ADR-0043, ADR-0047, ADR-0072 |
| Scale/Scope | 7 loại cấu hình, vài chục version/loại; workspace C theo số trường |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta (`GameConfigInput`, 7 JSON Schema, payload `competition_config.published`) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Backend chỉ module competition; portal chỉ gọi API |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Cấu hình là G (platform); workspace C luôn một tenant (System Admin chọn tenant đích); không hard-delete |
| IV Server-authoritative | ✅ | Validation và invariant ở server; publish idempotent + audit |
| V Xác thực, phân quyền | ✅ | `competition.config.*` theo loại, `game.content.review/publish`, MFA + gateway nội bộ; `lms.challenge.write` cho workspace C |
| VI An toàn trẻ em | ✅ | Invariant chặn nới ràng buộc khối, bonus tốc độ lớp nhỏ, XP ở bảng lớp, phần thưởng trả tiền |
| VII Một nguồn sự thật, cấu hình | ✅ | Giá trị khởi tạo theo tên CONFIG_DEFAULTS; schema `maxLength` |
| VIII UI tách logic, token | ✅ | Hook `api/` là lớp duy nhất gọi mạng; style từ token admin |
| IX Test có bằng chứng | ✅ | Mỗi `F108-AC-*` có task test viết trước (Go + Vitest/Playwright) |

## Project Structure

### Documentation (feature này)

```text
specs/108-competition-config-portal/
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
src/backend/modules/competition/
├── internal/domain/config/
│   ├── workflow.go           # draft → in_review → approved → published → archived; rejected
│   ├── invariants.go         # ràng buộc khối, bonus tốc độ, tier tăng dần, xp ở scope lớp, allowed_params, phần thưởng
│   └── effective.go          # effective_from ≥ now, mùa không chồng
├── internal/application/command/config/  # create_config.go, update_draft.go, transition.go, publish.go, rollback.go, unpublish.go, apply_published.go
├── internal/application/query/config/    # list_configs.go, get_version.go, list_versions.go, diff_versions.go, list_releases.go
├── internal/infrastructure/schema/validator.go   # nạp specs/schemas/*.schema.json (embed)
├── internal/presentation/http/admin/game_admin_handler.go   # 11 operationId *_competition, listener nội bộ
├── internal/presentation/consumer/config_published_consumer.go
└── tests/{unit,integration,contract}/config/

src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/
├── routes.tsx                 # /game-ops/competition/{matchmaking,ranking,seasons,templates,leaderboards}
├── api/configs.ts             # useConfigs, useConfigVersion, useSaveDraft, useTransition, usePublish, useRollback, useVersions, useDiff, useReleases
├── model/problemMessages.ts   # ProblemCode → khóa i18n
├── pages/{ConfigListPage,ConfigEditorPage,ConfigHistoryPage}.tsx
├── forms/{MatchmakingForm,RankingRulesForm,SeasonForm,TemplateForm,LeaderboardDefinitionForm}.tsx
├── dialogs/{PublishDialog,RollbackDialog,ReviewDialog}.tsx
├── i18n/{vi,en}.json
└── __tests__/*.test.tsx
src/cms/frontend/apps/portal/src/workspaces/school-lms/challenges/
├── routes.tsx                 # /school-lms/challenges, /school-lms/challenges/:id
├── api/challenges.ts          # useChallengeTemplates, useCreateChallenge, useChallenges, useChallenge, useCancelChallenge, useTournament*, useBracket, useOverrideNode
├── pages/{ChallengeListPage,ChallengeDetailPage}.tsx
├── wizard/{ChallengeWizard,StepTemplate,StepParams,StepTargets,StepReview}.tsx
├── bracket/BracketView.tsx
├── i18n/{vi,en}.json
└── __tests__/*.test.tsx
src/cms/frontend/apps/portal/e2e/{game-ops-competition,school-lms-challenges}.spec.ts
```

**Structure Decision**: Workspace A/C là chunk lazy-load riêng (ADR-0027 mục 7, route theo F051); form cấu hình sinh từ JSON Schema để không lặp quy tắc ở client.

## Phase 0: Research

Không có. Workflow/publish/rollback theo [API_DESIGN §9.13](../../docs/07-backend/API_DESIGN.md) và ADR-0043; màn hình theo [ADMIN_PORTAL §3.4, §10.1](../../docs/04-admin-portal/ADMIN_PORTAL.md#34-competition-management-84--workspace-a-platform-only-adr-0027).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `ConfigWorkflow`, `Invariants` (theo `config_type`), `EffectivePolicy` | Thuần |
| application | Command `CreateConfig`, `UpdateDraft`, `Transition`, `Publish`, `Rollback`, `Unpublish`, `ApplyPublished` · Query `ListConfigs`, `GetVersion`, `ListVersions`, `DiffVersions`, `ListReleases` | Permission theo loại (spec F108-AC-02) |
| infrastructure | ent repo (F104 schema), JSON Schema validator, outbox | |
| presentation | Handler admin; consumer `competition_config.published`; portal page/hook | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F104 (bảng cấu hình), F106 (`MaterializeDefinitions`, season command), F107 (API workspace C), F079 (khung workspace A, menu), F051 (shell, `withTenantScope`), F050 (`ops-ui`, `apiClient`), F004 (route gateway nội bộ).
- Event phát: `competition_config.published`, `audit.recorded`.

### Bảo mật, tenant, idempotency, audit

- `/game-admin/*`: middleware `platform_admin` + permission; listener nội bộ (cổng admin) — không có route ở gateway public.
- Workspace C: `lms.challenge.write`, `X-Tenant-Id` (System Admin chọn tenant đích).
- `Idempotency-Key`: transition, publish, rollback, unpublish; portal sinh một key mỗi thao tác (`crypto.randomUUID`).
- Audit: `competition_config.created/updated/transitioned/published/rolled_back/unpublished`.

### Quan sát

- Metric: `eduworld_competition_config_publish_total{config_type}`, `eduworld_competition_config_validation_failed_total{config_type}`.
- Span: `Publish`, `ApplyPublished`; portal gửi `traceparent` qua `apiClient`.

Ước lượng: 1,5 sprint backend + 2 sprint web [Proposed].


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.game_ops.competition`, `menu.school_lms.challenges` |
| Action dùng | `action.game_ops.competition.view`, `action.game_ops.competition.edit`, `action.game_ops.competition.workflow`, `action.game_ops.competition.publish`, `action.game_ops.competition.rollback`, `action.school_lms.challenges.view`, `action.school_lms.challenges.create`, `action.school_lms.challenges.manage` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `listTournaments` → `action.school_lms.challenges.view` (đã gắn). `getChallenge`, `getTournament`, `getTournamentBracket` (Cần thêm) → `action.school_lms.challenges.view`; `publishTournament`, `cancelTournament`, `cancelChallenge` (Cần thêm) → `action.school_lms.challenges.manage` (đã có trong catalog, UI-only tới khi có operation)

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |

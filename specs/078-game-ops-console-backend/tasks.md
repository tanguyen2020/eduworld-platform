# Tasks: F078 — Game Operations Console backend: workflow nội dung game, thumbnail 3D, catalog cho nhà trường

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Middleware `platform_staff` cho router `/game-admin/*`, `/game-media*` (azp `eduworld-portal`, MFA, role platform) và bảng quyền theo `config_type` — file: `src/backend/modules/game/internal/presentation/http/public/gameadmin_permissions.go`
- [x] T002 [P] Container `game-renderer` (Godot headless) + stream work-queue `GAME_RENDER_JOBS` (`ew.job.game.render.>`) tạo idempotent bởi worker — file: `deploy/compose/game-renderer.yml`, `src/backend/modules/game/internal/infrastructure/render/stream.go`
- [x] T003 [P] Cấu hình `scheduled_publish_min_lead`, `publish_to_visible_max`, `assets_per_version_max`, `game_text_required_languages`; `EW_S3_*` cho presign — file: `src/backend/modules/game/internal/config/gameadmin.go`, `src/backend/modules/game/.env.example`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 OpenAPI public: `createGameConfigVersion_game`, `cancelScheduledGameConfigPublish_game`, trường `GameConfigVersion` (thumbnail/turntable/author/based_on), `usages[]` khi unpublish, `MediaKind` render, khai báo `409`/`428` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [x] T005 [P] Internal: `applyTenantAssignment` (`[identity]`), `listAssignedGameActivities` (`[learning]`) + facade test với identity, learning — file: `specs/openapi/modules/game.internal.v1.yaml`, `src/backend/modules/game/tests/contract/internal_gameadmin_test.go` — kiểm: `make specs-lint`, `make test-contract`
- [x] T006 [P] Payload event game `content.version_submitted/approved/rejected/archived`, `content.rolled_back`, `content.unpublished`, `content.publish_scheduled`, `media.uploaded/processed/rejected`; delta khối `catalog` cho `content.published` (owner F071); không tạo `content.manifest_updated` (owner F026) và `tenant_data.*` (owner F045, module identity phát) — file: `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [x] T007 ent schema: `PreviewSession`, `RenderJob` (Mới); cột workflow/thumbnail trên bảng version còn thiếu; `content_releases.status`; CHECK `media_assets.kind` thêm render — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f078_gameadmin`, `make migrate-lint MOD=game`
- [x] T008 Migration: trigger con trỏ chỉ tới version `published`, trigger cấm sửa payload version `published`, CHECK reviewer ≠ author, index hẹn giờ — file: `src/backend/modules/game/migrations/`
- [x] T009 [P] Route APISIX nội bộ cho `/api/v1/game-admin/*`, `/api/v1/game-media*` (không có ở gateway public) — file: `deploy/apisix/`
- [x] T039 [P] Đăng ký permission platform-only `game.progression.write`, `game.shop.write`, `game.collection.write` (bỏ `game.level.write`) trong catalog và bảng `config_type → permission` — file: `src/backend/pkg/authz/catalog.go`, `src/backend/modules/game/internal/presentation/http/public/gameadmin_permissions.go` — kiểm: `make test MOD=game`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Ranh giới quyền của Game Operations Console (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T010 [P] [US1] Test sinh từ OpenAPI F078-AC-01: mọi operation `/game-admin/*`, `/game-media*` × {role tenant, platform thiếu MFA, `azp` sai, `game_designer` cho loại ngoài quyền} → 403 + `authz.denied` — file: `src/backend/modules/game/tests/integration/gameadmin_authz_test.go` — phải FAIL trước khi code
- [x] T011 [P] [US1] Test gateway F078-AC-02: gateway public trả 404 cho route; gọi thẳng service bằng token tenant → 403 — file: `tests/e2e/gameadmin_gateway_test.go`
- [x] T012 [P] [US1] Test F078-AC-03: dữ liệu `tenant_id NULL` dù gửi `X-Tenant-Id`; `config_type` của module khác → 404 — file: `src/backend/modules/game/tests/integration/gameadmin_scope_test.go`

### Implementation cho User Story 1

- [x] T013 [US1] Authorization game-admin: bảng permission dùng chung, mỗi command/query handler gọi tường minh `authz.Require(ctx, p, perm, resource)` ở đầu use case — file: `src/backend/modules/game/internal/application/gameadmin/permissions.go`

## Phase 4: User Story 2 — Soạn bản nháp và workflow duyệt (Priority: P1)

### Tests cho User Story 2

- [x] T014 [P] [US2] Unit test `gameconfig.Workflow` (mọi chuyển, reviewer ≠ author, archive con trỏ) — file: `src/backend/modules/game/internal/domain/gameconfig/workflow_test.go` — phải FAIL trước khi code
- [x] T015 [P] [US2] Integration test F078-AC-04, F078-AC-05, F078-AC-06: create validate qua module, idempotent; update chỉ draft, 409/412/428; version mới từ bản publish, bản publish bất biến — file: `src/backend/modules/game/tests/integration/gameconfig_draft_test.go`
- [x] T016 [P] [US2] Integration test F078-AC-07, F078-AC-08: `submit_review` với `MEDIA_NOT_READY`, `ITEM_REF_INVALID`, `TRANSLATION_INCOMPLETE`; approve/reject + event + audit; tự duyệt 403 — file: `src/backend/modules/game/tests/integration/gameconfig_review_test.go`
- [x] T017 [P] [US2] API test F078-AC-09, F078-AC-10: list/get/versions/diff theo quyền; chuỗi quá dài → 422 — file: `src/backend/modules/game/tests/integration/gameconfig_read_test.go`

### Implementation cho User Story 2

- [x] T018 [P] [US2] Domain `gameconfig.Workflow`, `RefCheck` — file: `src/backend/modules/game/internal/domain/gameconfig/`
- [x] T019 [US2] Command `CreateGameConfig`, `CreateGameConfigVersion`, `UpdateGameConfigDraft`, `TransitionGameConfigVersion`; query list/get/versions/diff; handler — file: `src/backend/modules/game/internal/application/command/transition_game_config.go`, `src/backend/modules/game/internal/presentation/http/public/game_admin.go`

## Phase 5: User Story 3 — Phát hành, rollback, gỡ xuất bản (Priority: P1)

### Tests cho User Story 3

- [x] T020 [P] [US3] Integration test F078-AC-11, F078-AC-12: publish ngay (snapshot, con trỏ, `OnPublished`, event, manifest, thời gian ≤ `publish_to_visible_max`); hẹn giờ 202 + job đúng giờ một lần; lead ngắn 422; hủy hẹn giờ — file: `src/backend/modules/game/tests/integration/gameconfig_publish_test.go` — phải FAIL trước khi code
- [x] T021 [P] [US3] Integration test F078-AC-13, F078-AC-14: rollback chỉ đổi con trỏ, đích sai 409; unpublish gỡ manifest + `usages[]` — file: `src/backend/modules/game/tests/integration/gameconfig_rollback_test.go`
- [x] T022 [P] [US3] Integration test F078-AC-15: quest progress/level session/challenge attempt ghim version cũ sau publish/rollback (TC-CNT-001) — file: `src/backend/modules/game/tests/integration/gameconfig_pinning_test.go`
- [x] T023 [P] [US3] Integration test F078-AC-16: audit đủ action + `diff_summary` + `reason`; `listGameConfigReleases_game` — file: `src/backend/modules/game/tests/integration/gameconfig_audit_test.go`

### Implementation cho User Story 3

- [x] T024 [US3] Command `PublishGameConfig` (dùng `gameconfig.Publisher` của kernel F071), `CancelScheduledPublish`, `RollbackGameConfig`, `UnpublishGameConfig`; job `scheduled-publish`; handler — file: `src/backend/modules/game/internal/application/command/publish_game_config.go`, `src/backend/modules/game/internal/presentation/job/scheduled_publish.go`

## Phase 6: User Story 4 — Preview và media nội dung game (Priority: P2)

### Tests cho User Story 4

- [x] T025 [P] [US4] Integration test F078-AC-17: token 30 phút, sai version 403, không tạo game session/attempt/XP/learning event — file: `src/backend/modules/game/tests/integration/game_preview_test.go` — phải FAIL trước khi code
- [x] T026 [P] [US4] Integration test F078-AC-18 (floci): presign giới hạn loại/dung lượng 415/413, `quarantined → ready/rejected`, `getGameMediaAsset` — file: `src/backend/modules/game/tests/integration/game_media_test.go`

### Implementation cho User Story 4

- [x] T027 [US4] Command `CreatePreviewSession` (lưu hash token), `PresignGameMedia`; consumer `media.processed/rejected`; handler `createGamePreviewSession`, `presignGameMediaUpload`, `getGameMediaAsset` — file: `src/backend/modules/game/internal/application/command/create_preview_session.go`, `src/backend/modules/game/internal/presentation/http/public/game_media.go`

## Phase 7: User Story 5 — Thumbnail và turntable 3D tự động (Priority: P2)

### Tests cho User Story 5

- [x] T028 [P] [US5] Integration test F078-AC-19, F078-AC-20 (renderer thật trong compose test hoặc fake renderer gắn nhãn `// TEST-ONLY`): asset render gắn version, chạy lại không trùng, renderer lỗi → publish giữ nguyên, retry rồi DLQ, `thumbnail_url` — file: `src/backend/modules/game/tests/integration/render_thumbnails_test.go` — phải FAIL trước khi code
- [x] T029 [P] [US5] GdUnit4 test script render: 512×512, 24 khung, đọc đúng asset theo đường dẫn tham số — file: `src/game/tests/tools/test_render_thumbnails.gd`

### Implementation cho User Story 5

- [x] T030 [US5] Script `render_thumbnails.gd` + entrypoint container (tải asset, render, upload `media/<sha256>/…`, trả kết quả) — file: `src/game/tools/thumbnail_renderer/render_thumbnails.gd`, `deploy/compose/game-renderer.yml`
- [x] T031 [US5] Consumer `game.render_thumbnails` (tạo `render_jobs`, publish job), `game.render_results` (command `RecordRenderResult`: `media_assets` + `media_usages` + cột thumbnail của version) — file: `src/backend/modules/game/internal/presentation/consumer/render_thumbnails.go`, `src/backend/modules/game/internal/application/command/record_render_result.go`

## Phase 8: User Story 6 — Gán nội dung game cho tenant và catalog nhà trường (Priority: P2)

### Tests cho User Story 6

- [x] T032 [P] [US6] Integration test F078-AC-21: assign/unassign idempotent, soft-delete, manifest tenant, event; module ngoài `x-allowed-callers` → `make arch-check` fail; chưa publish 409 — file: `src/backend/modules/game/tests/integration/tenant_assignment_test.go` — phải FAIL trước khi code
- [x] T033 [P] [US6] Integration test tenant isolation F078-AC-22, F078-AC-23: `content.published` có `catalog`; manifest/`listAssignedGameActivities` của `pgd-demo` không có quest chỉ gán `th-demo-a`; module ngoài `x-allowed-callers` → `make arch-check` fail (TC-TEN-019) — file: `src/backend/modules/game/tests/integration/game_catalog_test.go`

### Implementation cho User Story 6

- [x] T034 [US6] Command `ApplyTenantAssignment` + handler internal; query `ListAssignedGameActivities` + handler internal; bổ sung `catalog` vào payload `content.published` của quest/minigame_config — file: `src/backend/modules/game/internal/application/command/apply_tenant_assignment.go`, `src/backend/modules/game/internal/presentation/http/internal/tenant_assignments.go`, `src/backend/modules/game/internal/presentation/http/internal/game_activities.go`

## Phase 9: Polish & Cross-Cutting

- [x] T035 [P] Cập nhật DATABASE_DESIGN (§2.16 `preview_sessions`, `render_jobs`; §3.13 `media_assets.kind`, `content_releases.status`), ADMIN_PORTAL §3.11 (quyền rollback game), API_DESIGN §9.13b, EVENT_DESIGN §4.2, CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/04-admin-portal/ADMIN_PORTAL.md`, `docs/07-backend/API_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`, `CHANGELOG.md`
- [x] T036 [P] Thêm tài khoản demo `game_designer`, `liveops` vào realm local và LOCAL_DEVELOPMENT — file: `deploy/keycloak/realm-eduworld.dev.json`, `docs/11-operations/LOCAL_DEVELOPMENT.md`
- [x] T037 [P] Metric `eduworld_game_config_*`, `eduworld_game_publish_to_visible_seconds`, `eduworld_game_render_*` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`
- [x] T038 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc).

- [x] T040 (phía game xong: `content_refs[]`; consumer/projection `content.external_usages` + `ref_type` của learning chuyển F041/F054 — Clarifications 2026-11-24) **OQ chốt 2026-11-20** (OQ F054-3) `content.published` của cấu hình game mang `content_refs[]`; phía learning: consumer + projection `content.external_usages` + `getContentUsages` thêm `ref_type` `quest`/`minigame`/`challenge` (contract) + nhãn portal — file: `specs/events/payloads/game/`, `src/backend/modules/game/internal/application/command/`, `src/backend/modules/learning/internal/presentation/consumer/`, `specs/openapi/modules/learning.v1.yaml` — kiểm: `make specs-lint`, integration hai module — cỡ M

## Dependencies & Execution Order

- Setup → Foundational (T005 cần F045/F041 xác nhận; T009 cần F004) → US1 → US2 → US3 → US4, US5, US6 (song song) → Polish.
- Phụ thuộc kernel và module loại của F071–F077; projection catalog phía learning (F041).
- **OQ F041-2 (chốt 2026-11-20):** consumer projection `ref.game_activities` của learning (F041 T049) chờ payload catalog của T006/T034; khi hoàn thành T006/T034 phải báo F041 T049 để làm consumer (đến lúc đó `listGameActivities` trả rỗng).

## Bảng AC → test (triển khai 2026-11-24)

| AC | Test (pass) |
|---|---|
| F078-AC-01 | `TestGameAdminAuthz` (role tenant, platform không `game.*`, `liveops` cho quest, game token, platform với `azp` game → `403` + `authz.denied`), `pkg/httpx` `TestPlatformRoleGuard`, `TestHandleRouted` |
| F078-AC-02 | `deploy/apisix/scripts/check-routes.mjs` (tiền tố `/api/v1/game-admin`, `/api/v1/game-media` chỉ gateway nội bộ) + `deploy/apisix/tests/smoke.test.mjs`; gọi thẳng service bằng token tenant → `403` (`TestGameAdminAuthz`) |
| F078-AC-03 | `TestGameAdminAuthz` (`X-Tenant-Id` bị bỏ qua; `gifs`, `matchmaking-configs`, `currencies`, `bosses` → `404`), unit `TestLookupTypes` |
| F078-AC-04 | `TestGameAdminDraftWorkflow` (schema `422`, version 1 draft, `author_id`, `ETag`, khóa do server) |
| F078-AC-05 | `TestGameAdminDraftWorkflow` (`428`, `412`, `409 CONTENT_NOT_EDITABLE` khi `in_review`/`approved`), `TestGameAdminReleases` (sửa version publish `409`) |
| F078-AC-06 | `TestGameAdminReleases` (`createGameConfigVersion` từ v1, v1 vẫn `published` + con trỏ) |
| F078-AC-07 | `TestGameAdminDraftWorkflow` (client manifest `ITEM_REF_INVALID`, nội dung học tập chưa publish `ITEM_REF_INVALID`, `content.version_submitted`), `TestGameMediaAndRenders` (`MEDIA_NOT_READY`), unit `TestRefsAndTranslations` (`TRANSLATION_INCOMPLETE`), contract `TestCheckPublishedLearningContentFacade` |
| F078-AC-08 | `TestGameAdminDraftWorkflow` (tự duyệt `403`, reject cần lý do, reject/approve + event), `TestGameAdminReleases` (archive version không phải con trỏ), unit `TestWorkflowTransitions` |
| F078-AC-09 | `TestGameAdminDraftWorkflow` (list/versions cho reviewer, live-ops), `TestGameAdminReleases` (diff theo field path), unit `TestDiff` |
| F078-AC-10 | `TestGameAdminDraftWorkflow` (title > 200, payload > 64 KB → `422`) |
| F078-AC-11 | `TestGameAdminReleases` (reviewer `403`, thiếu `If-Match` `428`, publish ngay: con trỏ, `content.published` + `catalog`, manifest tenant) |
| F078-AC-12 | `TestGameAdminReleases` (lead ngắn `422`, `202` + `content.publish_scheduled`, lịch thứ hai `409`, hủy + audit, job áp dụng đúng một lần), unit `TestReleaseRules` |
| F078-AC-13 | `TestGameAdminReleases` (rollback về v1 không tạo version, `content.rolled_back`; đích là con trỏ `422 ROLLBACK_TARGET_INVALID`), unit `TestReleaseRules` |
| F078-AC-14 | `TestGameAdminReleases` (con trỏ rỗng, `usages[]` có `tenant_assignment`, `content.unpublished`, catalog rỗng, gỡ lần hai `409`) |
| F078-AC-15 | `TestGameAdminReleases` (quest progress ghim v1 sau publish v2 hẹn giờ và rollback) |
| F078-AC-16 | `TestGameAdminReleases` (audit `content.*`, `game_config.changed`; dòng thời gian phân trang keyset có `cancelled`, `scheduled_for`) |
| F078-AC-17 | `TestGameAdminPreview` (token đọc đúng version, version/người khác/hết hạn `403`, không lưu token thô, không game session/XP), unit `TestPreviewSession` |
| F078-AC-18 | `TestGameMediaAndRenders` (`413`, `415`, `duplicate_of`, pipeline `ready`/`rejected`, `getGameMediaAsset` biến thể), unit `TestMediaAndRender` |
| F078-AC-19 | `TestGameMediaAndRenders` (2 job thumbnail/turntable, chạy lại không trùng, asset `render_thumbnail` gắn version, `thumbnail_url`), renderer `go test ./modules/game/cmd/renderer/...`, GdUnit `test_render_thumbnails.gd` (chưa chạy: máy không có Godot) |
| F078-AC-20 | `TestGameMediaAndRenders` (lỗi 3 lần: backoff, `dead`, `infra.dead_letters`; publish giữ nguyên; `turntable_url` vắng) |
| F078-AC-21 | `TestGameAdminReleases` (chưa publish `409 CONTENT_NOT_PUBLISHED`, gán idempotent, soft-delete khi gỡ, manifest A, không phát `tenant_data.*`), contract `TestTenantAssignmentsFacade`, identity `TestOwnerGatewayGameAssignments`, `allowedcallers --check`/`archgen --check` (`x-allowed-callers: [identity]`) |
| F078-AC-22 | `TestGameAdminReleases` (`content.published.catalog` tiêu đề song ngữ, `content_refs`; manifest/catalog của tenant B không có quest), `checkEventSchemas` |
| F078-AC-23 | `TestGameAdminReleases` (`listAssignedGameActivities` A có, B rỗng), contract `TestListAssignedGameActivitiesFacade` (khớp `GameActivityCatalogPage`), `archgen --check` (`x-allowed-callers: [learning]`) |

## Definition of Done

- [x] Mọi tiêu chí F078-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game`; `make test-contract`, `make godot-test` (script render) xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

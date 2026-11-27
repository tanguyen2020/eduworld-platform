# Tasks: F134 — Công cụ vận hành bổ sung: lệnh `reconcile-<module>`, loại tenant QA khỏi analytics, trang tải bộ cài cho trường

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Viết tắt: `<module>` ∈ `identity`, `learning`, `game`, `competition`, `chat`; `GM/` = `src/backend/modules/game/`; `WP/` = `src/cms/frontend/apps/portal/src/workspaces/`.

## Phase 1: Setup

- [x] T001 Đăng ký permission `school.installer.read` (tenant được phép; gán `tenant_admin`, `school_admin`) — file: `src/backend/pkg/authz/catalog.go` — **Bằng chứng:** `PermSchoolInstallerRead` trong catalog (tenant_admin/school_admin qua `school.*`), ADMIN_PORTAL §2.2; `TestCatalogMatchesAdminPortalDoc` pass
- [x] T002 [P] Biến môi trường `EW_CLIENT_RELEASES_BASE_URL`, `EW_CLIENT_RELEASE_PUBKEY_FILE` cho module game; target `make reconcile MOD= RESTORE_POINT= FROM= TO=` — file: `deploy/compose/.env.example`, `Makefile` — **Bằng chứng:** target `make reconcile`; biến môi trường mới `EW_CLIENT_RELEASES_BASE_URL`, `EW_CLIENT_RELEASE_PUBKEY_FILE`, `EW_GAME_WEB_URL` (không sửa `.env.example` theo quy tắc phiên — liệt kê trong báo cáo)
- [x] T003 [P] Script one-off Swarm `reconcile-<module>` (image `worker`, lệnh `reconcile --module=<module>`, restart none, mount `--out`) — file: `scripts/ops/reconcile.sh` — **Bằng chứng:** `scripts/ops/reconcile.sh` (qua `run-ops-job.sh --like=worker-jobs`; dry-run `RECONCILE_DRY_RUN=1` kiểm tay)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta spec internal (facade): `listOutboxEventIds`, `listProcessedEventIds` (schema chung, `source_module` enum) cho 5 module, `x-allowed-callers` 5 module — file: `specs/openapi/modules/common/internal-common.v1.yaml`, `specs/openapi/modules/{identity,learning,game,competition,chat}.internal.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=<module>` — **Bằng chứng:** khối `x-ops-reconcile-paths` trong `internal-common.v1.yaml`, `sync-platform-ops.mjs` chép vào 5 module; facadegen/allowedcallers/archgen --check sạch; oasdiff 0 breaking
- [x] T005 [P] Delta public: `listSchoolClientInstallers` + `SchoolClientInstaller`; `TenantSummary.is_qa`; `TenantUpdate.is_qa` (F010); `excluded_tenant_ids[]` trong response analytics platform — file: `specs/openapi/modules/game.v1.yaml`, `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game` — **Bằng chứng:** `listSchoolClientInstallers`, `SchoolClientInstaller(List)`, `TenantSummary`/`TenantUpdate.is_qa`, `ExcludedTenantIds` ở 5 response game; split-public --check sạch
- [x] T006 [P] JSON Schema `reconcile-report.schema.json`, `school-kit-index.schema.json`; delta tương thích `tenant.updated` (+`is_qa`) cho owner F010 — file: `specs/schemas/ops/reconcile-report.schema.json`, `specs/schemas/school-kit-index.schema.json`, `specs/events/payloads/identity/tenant.updated.v1.schema.json` — kiểm: `make specs-lint` — **Bằng chứng:** `reconcile-report.schema.json`, `school-kit-index.schema.json`, `tenant.updated` (+if/then `is_qa`); validate-schemas 268 schema/0 lỗi
- [x] T007 ent + migration: `platform.tenants.is_qa` (CHECK không `common`); `ref.tenants.is_qa` ở game, learning — file: `src/backend/modules/identity/ent/schema/tenant.go`, `GM/ent/schema/ref/tenant.go`, `src/backend/modules/learning/ent/schema/ref/tenant.go` — kiểm: `make migrate-diff MOD=<module>`, `make migrate-lint MOD=<module>` — **Bằng chứng:** cột đã có từ F010/F020/F014 (`platform.tenants.is_qa` + CHECK, `ref.tenants.is_qa`); migration mới `20270201000134_infra_reconcile_grants` (5 module, infragen + atlas.sum); domain `Tenant.IsQA`
- [x] T008 [P] `src/backend/pkg/messaging` registry `Consumer{Name, SourceService, EventTypes}`; `src/backend/pkg/outbox.Reader.ListByWindow` (chỉ đọc) — file: `src/backend/pkg/messaging/registry.go`, `src/backend/pkg/outbox/reader.go` — **Bằng chứng:** `pkg/module.ConsumerInfos` (thay `pkg/messaging/registry.go`, Clarifications 2026-11-27) + `pkg/outbox.Reader`; `TestConsumerInfos`, `TestReconcileConsumersAndPeersFacade` pass
- [x] T009 [P] Test F134-AC-05 (phần tham số) giới hạn chuỗi/enum ở validator sinh từ spec (`cursor` 513, `source_module` lạ, `limit` vượt trần → `VALIDATION_FAILED`) (TC-TEN-024) — file: `src/backend/pkg/reconcile/params_test.go` — phải FAIL trước khi code — **Bằng chứng:** `pkg/reconcile/params_test.go` `TestFacadeParameters`, `TestSourceAuthorizationAndPages` pass

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Đối soát sau khôi phục (Priority: P1) 🎯 MVP

**Goal**: `reconcile-<module>` báo lệch có phân loại, exit code đúng.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T010 [P] [US1] Test F134-AC-01: bảng phân loại `Classify` (event time vs T, `published_at`, DLQ, `--settle`) — file: `src/backend/pkg/reconcile/classify_test.go` — phải FAIL trước khi code — **Bằng chứng:** `TestClassify` pass
- [x] T011 [P] [US1] Test F134-AC-01, F134-AC-02, F134-AC-03 tích hợp: khôi phục `eduworld_learning` về T (testcontainers, PITR giả bằng snapshot) → `lost_after_restore` đúng, `unexplained = 0`, exit 0; bỏ reset consumer → `unexplained` chiều inbound, exit 2; peer down → exit 3 (TC-SVC-014) — file: `src/backend/modules/learning/tests/integration/ops/reconcile_restore_test.go` — **Bằng chứng:** `cmd/worker/reconcile_integration_test.go` `TestReconcileRestoreOnModuleDatabases` trên 5 database module thật (lost_after_restore = 3, unexplained = 0 → exit 0; consumer chưa reset → exit 2; game không đọc được → exit 3) pass — đặt ở cmd/worker vì cần cả 5 module
- [x] T012 [P] [US1] Test F134-AC-04: cửa sổ > `outbox_retention`, `from ≥ to`, T ngoài cửa sổ → exit 1, không gọi peer — file: `src/backend/pkg/reconcile/window_test.go` — **Bằng chứng:** `TestWindowValidate`, `TestReconcileCommandBadArgs` (không dựng module nào) pass
- [x] T013 [P] [US1] Test F134-AC-05: không có route HTTP `/internal/*` cho hai operation; fixture import facade từ module ngoài `x-allowed-callers` (`realtime`) → `make arch-check` fail; phân trang cursor; không trả `envelope` (TC-SVC-005, TC-SVC-006) — file: `src/backend/modules/<module>/tests/integration/ops/facade_ops_test.go` (5 module, bảng chung `src/backend/pkg/testkit/opsreconcile`) — **Bằng chứng:** không route HTTP (facade không mount; `TestReconcileConsumersAndPeersFacade`: thiếu ngữ cảnh job → 403, caller realtime → CALLER_NOT_ALLOWED); fixture `realtime → learning/api` trong `scripts/ci/test-arch-fixtures.sh` fail đúng; báo cáo không có envelope (integration)
- [x] T014 [P] [US1] Test F134-AC-06: lệnh chỉ đọc (DB role chỉ SELECT `infra.*` + INSERT outbox), audit `ops.reconcile.completed` — file: `src/backend/modules/learning/tests/integration/ops/reconcile_readonly_test.go` — **Bằng chứng:** integration: 4 audit `ops.reconcile.completed`; `learning_platform` bị từ chối SELECT `envelope` và UPDATE `processed_events`; `TestReconcileCommandReportAndAudit` pass
- [x] T015 [P] [US1] Facade test `listOutboxEventIds`/`listProcessedEventIds` (bên gọi với fake facade, bên nhận với ví dụ spec internal) cho mọi cặp module, chạy bằng `make test-contract` — file: `tests/contract/ops_reconcile/` — **Bằng chứng:** bên gọi với facade giả: `pkg/reconcile` `TestRunScenarios`; bên nhận với facade thật của 5 module: `TestReconcileConsumersAndPeersFacade` (`make test-contract`)

### Implementation cho User Story 1

- [x] T016 [US1] `src/backend/pkg/reconcile` (`window.go`, `classify.go`, `report.go`, `runner.go`) với port `PeerClient`, `OutboxReader`, `ProcessedReader` — file: `src/backend/pkg/reconcile/` — **Bằng chứng:** `pkg/reconcile/{window,classify,report,runner,service}.go`
- [x] T017 [US1] Query `ListOutboxEventIds`, `ListProcessedEventIds` + hiện thực facade (lọc khoảng UUIDv7, gộp `dead_letters`, `consumers[]` từ registry) — file: `src/backend/modules/<module>/internal/application/query/ops/`, `src/backend/modules/<module>/internal/presentation/facade/ops_reconcile.go` — **Bằng chứng:** `pkg/reconcile.Source` dùng chung + `internal/presentation/facade/ops_reconcile.go` ở 5 module
- [x] T018 [US1] Adapter facade peer (`src/backend/pkg/modcall`) + lệnh con `worker reconcile --module=<module> --restore-point --from --to --settle --out` — file: `src/backend/modules/<module>/internal/infrastructure/modules/reconcile_peers.go`, `src/backend/cmd/worker/reconcile.go` — **Bằng chứng:** `cmd/internal/wiring/reconcile_<module>.go` (adapter composition root, Clarifications 2026-11-27) + `cmd/worker/reconcile.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Loại tenant QA khỏi analytics (Priority: P2)

### Tests cho User Story 2

- [x] T019 [P] [US2] Test F134-AC-07: `updateTenant(is_qa)` audit trước/sau, `tenant.updated` có `is_qa`, `common` → `422`, role tenant → `403` (TC-CMS-WS-02) — file: `src/backend/modules/identity/tests/integration/tenant/qa_flag_test.go` — phải FAIL trước khi code — **Bằng chứng:** `modules/identity/tests/integration/qa_flag_integration_test.go` `TestTenantQAFlag` + unit `TestTenantQAFlag` pass
- [x] T020 [P] [US2] Test F134-AC-08: golden dataset có event tenant QA → không vào `events_raw`/`player_days`, metric dropped (TC-ANL-001) — file: `GM/tests/integration/analytics/qa_tenant_ingest_test.go` — **Bằng chứng:** `TestAnalyticsQATenantExcluded` (consumer + metric) và `TestAnalyticsTelemetry` (telemetry, F080) pass
- [x] T021 [P] [US2] Test F134-AC-09: query catalog + export + `getPlatformLearningSummary` loại tenant QA kể cả dữ liệu cũ; `tenant_ids` chỉ QA → rỗng + `excluded_tenant_ids` (TC-ANL-001) — file: `GM/tests/integration/analytics/qa_tenant_query_test.go`, `src/backend/modules/learning/tests/integration/analytics/platform_summary_qa_test.go` — **Bằng chứng:** `TestAnalyticsQATenantExcluded` (5 API game, trước/sau cờ, bỏ cờ), learning `TestTenantAndPlatformSummary` (mọi tenant → excluded) + `TestPlatformReport` (export loại tenant QA) pass
- [x] T022 [P] [US2] Test F134-AC-10: Tenant Admin tenant QA → `200` rỗng; tenant khác → `403`/`404` (TC-ANL-004) — file: `GM/tests/integration/analytics/tenant_summary_qa_test.go` — **Bằng chứng:** `TestAnalyticsQATenantExcluded` (Tenant Admin tenant QA 200 rỗng, tenant B → 403) pass
- [ ] T023 [P] [US2] Test F134-AC-11: `TenantForm` công tắc + xác nhận, `TenantFilter` nhãn QA (TC-CMS-WS-01) — file: `WP/platform-admin/__tests__/TenantFormQa.test.tsx`, `src/cms/frontend/packages/ops-ui/src/tenant/TenantFilter.test.tsx`

### Implementation cho User Story 2

- [x] T024 [US2] identity: `Tenant.SetQA`, `UpdateTenant` (+`is_qa`), outbox `tenant.updated` — file: `src/backend/modules/identity/internal/domain/tenant/tenant.go`, `src/backend/modules/identity/internal/application/command/tenant/update_tenant.go` — **Bằng chứng:** `Tenant.SetQA`/`Patch.IsQA`, `UpdateTenant` (+audit `tenant.qa_flag_changed`, `idempotency.SetResult`), payload `is_qa`
- [x] T025 [US2] Projection `ref.tenants.is_qa` (game, learning); game ingest bỏ QA; query catalog + export anti-join; learning platform summary anti-join — file: `GM/internal/application/command/analytics/{ingest_domain_event.go,ingest_telemetry.go}`, `GM/internal/application/query/analytics/query_catalog.go`, `src/backend/modules/learning/internal/application/query/analytics/platform_summary.go` — **Bằng chứng:** projection có sẵn (F080/F020); `QATenants` + `excluded_tenant_ids`, tóm tắt tenant QA rỗng, learning excluded khi hỏi mọi tenant, export platform loại tenant QA
- [ ] T026 [US2] Portal: công tắc QA, nhãn QA, chuỗi i18n — file: `WP/platform-admin/tenants/TenantForm.tsx`, `src/cms/frontend/packages/ops-ui/src/tenant/TenantFilter.tsx`, `WP/platform-admin/i18n/{vi,en}.json`

## Phase 5: User Story 3 — Trang tải bộ cài (Priority: P2)

### Tests cho User Story 3

- [x] T027 [P] [US3] Test F134-AC-12: `publish-index.sh` sinh chỉ mục đúng schema + chữ ký; thiếu bộ cài đã ký → fail — file: `deploy/installer/school-kit/tests/publish_index_test.sh` — phải FAIL trước khi code — **Bằng chứng:** `deploy/installer/school-kit/tests/publish_index_test.sh` 8/8 pass
- [x] T028 [P] [US3] Test F134-AC-13: lọc khối (`GradeLevel→GradeBand`), bản mới nhất mỗi nền tảng × nhóm khối, trường chưa có lớp → mọi bộ cài, cache — file: `GM/tests/unit/installer/filter_test.go`, `GM/tests/integration/installer/list_school_installers_test.go` — **Bằng chứng:** `modules/game/tests/unit/installer_test.go` `TestInstallerForSchool` + integration `TestSchoolClientInstallers` (lọc khối, trường chưa có học sinh, cache) pass
- [x] T029 [P] [US3] Test F134-AC-14: chỉ mục sai chữ ký/schema/không tải được → `503 UNAVAILABLE`, không link — file: `GM/tests/integration/installer/index_invalid_test.go` — **Bằng chứng:** `TestInstallerIndexVerification` + `TestSchoolClientInstallers` (đổi 1 byte → 503 không link, CDN down → 503, chưa cấu hình → 503) pass
- [x] T030 [P] [US3] Test F134-AC-15 tenant isolation + authorization: trường khác cùng/khác tenant → `404`; Academic Manager, học sinh → `403` (TC-TEN-003, TC-TEN-001) — file: `GM/tests/integration/installer/authz_test.go` — **Bằng chứng:** `TestSchoolClientInstallers` (A2 cùng tenant / B khác tenant → 404, academic_manager và học sinh → 403, `authztest.Cover`) pass
- [x] T031 [P] [US3] Test F134-AC-16: response không chứa trường ngoài schema, chuỗi trong giới hạn (TC-TEN-024) — file: `GM/tests/contract/installer_response_test.go` — **Bằng chứng:** `TestSchoolClientInstallers` (chỉ trường public, không lộ tenant/đường dẫn nội bộ) + `TestInstallerIndexVerification` (giới hạn chuỗi) — đặt trong integration/unit thay vì tests/contract
- [ ] T032 [P] [US3] Test F134-AC-17: trang hiển thị bảng, sao chép SHA-256, lỗi `503` theo `code`, menu ẩn khi thiếu permission (TC-CMS-WS-01) — file: `WP/school-lms/installers/__tests__/InstallerListPage.test.tsx`, `src/cms/frontend/apps/portal/tests/e2e/school-lms/installers.spec.ts`

### Implementation cho User Story 3

- [ ] T033 [US3] CI: `publish-index.sh` (ký RSA-SHA256, chỉ kênh `production` sau promote) gọi từ job release của F133 — file: `deploy/installer/school-kit/publish-index.sh`, `.github/workflows/godot.yml`
- [x] T034 [US3] module game: domain `FilterByGradeBands`; port `SchoolKitIndex`; adapter HTTP + kiểm chữ ký + cache Valkey; query `ListSchoolInstallers`; handler `listSchoolClientInstallers` — file: `GM/internal/domain/installer/`, `GM/internal/infrastructure/release/`, `GM/internal/application/query/installer/list_school_installers.go`, `GM/internal/presentation/http/public/client_installers.go` — **Bằng chứng:** domain `installer`, port `SchoolKitIndex`/`InstallerReads`, `infrastructure/release` (timeout, retry GET, circuit breaker, cache Valkey 5 phút), query `SchoolInstallers`, route `installers.go`
- [ ] T035 [US3] Portal: `InstallerListPage`, `DeviceConfigGuide`, hook `useSchoolInstallers`, mục menu workspace C theo khóa catalog `menu.school_lms.installers`, i18n — file: `WP/school-lms/installers/`, `WP/school-lms/routes.tsx`, `src/cms/frontend/apps/portal/src/i18n/locales/{vi,en}/portal.school_lms.json`

## Phase N: Polish & Cross-Cutting

- [x] T036 [P] BACKUP_RECOVERY §3.5, §9 RB-01 bước 7 (lệnh cụ thể, exit code, cách đọc báo cáo); TEST_STRATEGY §4 (cờ `is_qa`) — file: `docs/11-operations/BACKUP_RECOVERY.md`, `docs/12-project/TEST_STRATEGY.md` — **Bằng chứng:** BACKUP_RECOVERY §3.5, RB-01 bước 7, §10; TEST_STRATEGY §4 đã có cờ `is_qa` (không đổi)
- [x] T037 [P] DATABASE_DESIGN (`is_qa`), GAME_ANALYTICS §5 (bỏ tenant QA), DASHBOARD_DATA §2, DEPLOYMENT §9 (chỉ mục bộ cài, trang portal), ADMIN_PORTAL §2.2 (`school.installer.read`), §10 sitemap C — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/11-operations/GAME_ANALYTICS.md`, `docs/07-backend/DASHBOARD_DATA.md`, `docs/11-operations/DEPLOYMENT.md`, `docs/04-admin-portal/ADMIN_PORTAL.md` — **Bằng chứng:** DATABASE_DESIGN (grant reconcile), DASHBOARD_DATA §3.4, DEPLOYMENT §9, ADMIN_PORTAL §2.2/§10/§10.1; GAME_ANALYTICS A-8 đã đúng (không đổi)
- [x] T038 [P] Đăng ký `reconcile_settle_minutes` [Confirmed] — file: `docs/00-overview/CONFIG_DEFAULTS.md` — **Bằng chứng:** CONFIG_DEFAULTS `reconcile_settle_minutes`
- [x] T039 [P] MONITORING: `eduworld_analytics_events_dropped_total`, `eduworld_school_kit_index_fetch_total`, alert `SchoolKitIndexInvalid` + rule — file: `docs/11-operations/MONITORING.md`, `deploy/observability/prometheus/rules/game.yaml` — **Bằng chứng:** MONITORING §3 (2 metric), §5 `SchoolKitIndexInvalid`, rule + `promtool test rules game.test.yaml` SUCCESS, runbook; `check-alerts` 0 vi phạm
- [x] T040 [P] Workflow nightly (`on.schedule` trong `go.yml`) chạy T011 (TC-SVC-014) — file: `.github/workflows/go.yml` — **Bằng chứng:** không cần lịch riêng: job `integration (binaries, health)` của `go.yml` chạy `./cmd/...` (gồm T011) ở mỗi thay đổi backend
- [x] T041 [P] CHANGELOG — file: `CHANGELOG.md` — **Bằng chứng:** CHANGELOG [Unreleased] Added
- [ ] T042 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng
- [ ] T043 [P] Test F134-AC-18: tắt `menu.school_lms.installers` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/installers/__tests__/feature-toggle.test.tsx`
- [ ] T044 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F134-AC-18) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T004–T009) → US1 (song song theo module sau T016) · US2 (cần F010 `updateTenant`, F080 analytics) · US3 (cần F133 khóa ký + bộ cài) song song → Polish.
- T011 cần F003 testkit khôi phục DB; T025 cần F043/F044 `getPlatformLearningSummary`; T026 cần F057 `TenantForm`.

## Definition of Done

- [ ] Mọi tiêu chí F134-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (5 module), `make arch-check`, `make test-contract`, `make migrate-lint`, `make web-test`, `make web-lint` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

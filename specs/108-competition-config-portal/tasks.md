# Tasks: F108 — Cấu hình thi đấu `/game-admin/*` và UI workspace A

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Đăng ký router `/game-admin/*` trên listener nội bộ với middleware `platform_admin`; consumer `competition_config.published` — file: `src/backend/modules/competition/module.go` (nối vào `src/backend/cmd/core-api`, `src/backend/cmd/worker`)
- [ ] T002 [P] Tạo module `competition` trong chunk `game-ops` và `challenges` trong chunk `school-lms`, đăng ký route + mục menu theo permission — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/routes.tsx`, `src/cms/frontend/apps/portal/src/workspaces/school-lms/challenges/routes.tsx`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Delta OpenAPI `GameConfigInput` (`config_id`, `based_on_version_id`), mô tả `409`/`412` — file: `specs/openapi/modules/competition.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition`, `pnpm gen:api`
- [ ] T004 [P] 7 JSON Schema cấu hình thi đấu + `GameConfigType` thêm `competition-rewards` — file: `specs/schemas/matchmaking-config.schema.json`, `specs/schemas/ranking-rules.schema.json`, `specs/schemas/season.schema.json`, `specs/schemas/tournament-template.schema.json`, `specs/schemas/challenge-template.schema.json`, `specs/schemas/leaderboard-definition.schema.json`, `specs/schemas/competition-reward.schema.json`, `specs/openapi/modules/competition.v1.yaml` — kiểm: `make specs-lint`
- [ ] T005 [P] Payload `competition_config.published.v1`; catalog EVENT_DESIGN §4.8 — file: `specs/events/payloads/competition/competition_config.published.v1.schema.json`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint`
- [ ] T006 [P] Route APISIX gateway nội bộ cho 7 `config_type` competition — file: `deploy/apisix/routes/internal-game-admin.yaml`
- [ ] T007 [P] Kiểm seed hiện có (F104/F106/F107) hợp lệ với 7 schema — file: `src/backend/modules/competition/seed/`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Cấu hình thi đấu phiên bản hóa qua `/game-admin/*` (Priority: P1) 🎯 MVP

**Goal**: Phía ghi cấu hình, workflow, publish/rollback, áp runtime.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T008 [P] [US1] Unit test F108-AC-04: invariant theo loại (ràng buộc khối, bonus tốc độ dưới `speed_bonus_min_grade`, tier tăng dần, `xp` ở scope trường/khối, từ chối scope `class`/metric `class_goal`/`kind` `class_challenge`/seeding `separate_class` — ADR-0095, `allowed_params`, phần thưởng) + validate JSON Schema — file: `src/backend/modules/competition/tests/unit/config/invariants_test.go` — phải FAIL trước khi code
- [ ] T009 [P] [US1] Unit test F108-AC-05: workflow, bất biến version publish, `effective_from` ≥ now, mùa không chồng, rollback target — file: `src/backend/modules/competition/tests/unit/config/workflow_test.go`
- [ ] T010 [P] [US1] Integration test F108-AC-01, F108-AC-06, F108-AC-07 (TC-CMS-001…004): 11 operationId × 7 loại; `If-Match` thiếu/lệch; idempotent publish; audit; `content_releases` — file: `src/backend/modules/competition/tests/integration/config/game_admin_api_test.go`
- [ ] T011 [P] [US1] Test authorization F108-AC-02, F108-AC-03 (TC-CMS-AUTHZ-01, TC-CMS-AUTHZ-02, TC-CMS-WS-02, TC-CMS-WS-03): permission theo loại, review/publish; role tenant → `403`; thiếu MFA → `403 MFA_REQUIRED`; route không có ở gateway public — file: `src/backend/modules/competition/tests/integration/config/authz_test.go`
- [ ] T012 [P] [US1] Integration test (đồng hồ giả) F108-AC-08: `competition_config.published` áp mùa, leaderboard definition đúng `effective_from`; trận đang chạy giữ version — file: `src/backend/modules/competition/tests/integration/config/apply_published_test.go`

### Implementation cho User Story 1

- [ ] T013 [P] [US1] Domain `ConfigWorkflow`, `Invariants`, `EffectivePolicy` — file: `src/backend/modules/competition/internal/domain/config/workflow.go`, `src/backend/modules/competition/internal/domain/config/invariants.go`, `src/backend/modules/competition/internal/domain/config/effective.go`
- [ ] T014 [P] [US1] Validator JSON Schema (embed `specs/schemas`) — file: `src/backend/modules/competition/internal/infrastructure/schema/validator.go`
- [ ] T015 [US1] Command `CreateConfig`, `UpdateDraft`, `Transition`, `Publish`, `Rollback`, `Unpublish` (tx + outbox + audit); query `ListConfigs`, `GetVersion`, `ListVersions`, `DiffVersions`, `ListReleases` — file: `src/backend/modules/competition/internal/application/command/config/`, `src/backend/modules/competition/internal/application/query/config/`
- [ ] T016 [US1] Handler 11 operationId `*_competition` + middleware `platform_admin` — file: `src/backend/modules/competition/internal/presentation/http/admin/game_admin_handler.go`
- [ ] T017 [US1] Command `ApplyPublished` + consumer (gọi `MaterializeDefinitions`, season của F106) — file: `src/backend/modules/competition/internal/application/command/config/apply_published.go`, `src/backend/modules/competition/internal/presentation/consumer/config_published_consumer.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Màn "Thi đấu" trong Game Operations Console (workspace A) (Priority: P1)

**Goal**: Danh sách, editor, duyệt, publish, so sánh, rollback cho 7 loại.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T018 [P] [US2] E2E F108-AC-09 (TC-CMS-WS-01, TC-CMS-AUTHZ-05): người dùng tenant mở `/game-ops/competition` → 403, network log không có chunk `game-ops`; `sa.admin` thấy menu — file: `src/cms/frontend/apps/portal/e2e/game-ops-competition.spec.ts` — phải FAIL trước khi code
- [ ] T019 [P] [US2] Component test F108-AC-10, F108-AC-11: list/editor/diff/history/publish/rollback gọi đúng hook; lỗi theo `ProblemCode` (MSW nhãn MOCK), thông báo VPN — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/__tests__/config_pages.test.tsx`
- [ ] T020 [P] [US2] Kiểm F108-AC-12 (TC-UI-003, TC-UI-006): stylelint/ESLint chặn hardcode, axe màn danh sách/editor, i18n vi/en đủ khóa — file: `src/cms/frontend/apps/portal/e2e/game-ops-competition-a11y.spec.ts`

### Implementation cho User Story 2

- [ ] T021 [P] [US2] Hook `api/configs.ts` (query key có loại cấu hình, `Idempotency-Key` một lần/thao tác) + `problemMessages.ts` — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/api/configs.ts`, `src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/model/problemMessages.ts`
- [ ] T022 [US2] Page `ConfigListPage`, `ConfigEditorPage` (`ContentEditorShell` + form từ JSON Schema), `ConfigHistoryPage`; dialog publish/rollback/review; form 5 nhóm; i18n — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/pages/`, `src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/forms/`, `src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/dialogs/`, `src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/i18n/vi.json`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — School Admin tổ chức thử thách khối/giải trường (workspace C) (Priority: P1)

**Goal**: Wizard tạo từ template, quản lý giải, bracket.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T023 [P] [US3] E2E F108-AC-13 (TC-CMS-AUTHZ-06): `qlcm.truong.a`, `ad.donvi` không có menu, URL → 403; `ad.truong.a` thấy — file: `src/cms/frontend/apps/portal/e2e/school-lms-challenges.spec.ts` — phải FAIL trước khi code
- [ ] T024 [P] [US3] Component + E2E F108-AC-14 (TC-CMS-AUTHZ-07): wizard chỉ `allowed_params`, đối tượng một trường: khối ± học sinh thêm/loại trừ cùng trường (≤ 500, không bộ chọn lớp — ADR-0095), một `Idempotency-Key`/lần bấm, hiển thị `RESOURCE_NOT_ASSIGNED`/`VALIDATION_FAILED` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/challenges/__tests__/wizard.test.tsx`
- [ ] T025 [P] [US3] Component test F108-AC-15: chi tiết giải, bracket tên thật, thao tác gọi đúng operationId F107, override cần lý do + xác nhận 2 bước — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/challenges/__tests__/detail.test.tsx`
- [ ] T026 [P] [US3] Test tenant F108-AC-16 (TC-CMS-TEN-03): System Admin chưa chọn tenant → nút khóa; chọn → `X-Tenant-Id` đúng; đổi tenant khi wizard mở → reset — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/challenges/__tests__/tenant_scope.test.tsx`

### Implementation cho User Story 3

- [ ] T027 [P] [US3] Hook `api/challenges.ts` (F107 operationId, `withTenantScope` của F051) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/challenges/api/challenges.ts`
- [ ] T028 [US3] Wizard (`StepTemplate`, `StepParams` dựng từ `allowed_params`, `StepTargets` (trường → khối, thêm/loại trừ học sinh cùng trường, xem trước số học sinh — ADR-0095), `StepReview`), `ChallengeListPage`, `ChallengeDetailPage`, `BracketView`, i18n — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/challenges/wizard/`, `src/cms/frontend/apps/portal/src/workspaces/school-lms/challenges/pages/`, `src/cms/frontend/apps/portal/src/workspaces/school-lms/challenges/bracket/BracketView.tsx`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: Polish & Cross-Cutting

- [ ] T029 [P] Cập nhật ADMIN_PORTAL §3.4/§10 (màn "Thi đấu", "Thử thách khối / giải trường" — nhãn theo ADR-0095), ghi `competition.config.reward.write` cho `competition-rewards`; đề xuất tài khoản demo `competition_admin` trong LOCAL_DEVELOPMENT — file: `docs/04-admin-portal/ADMIN_PORTAL.md`, `docs/11-operations/LOCAL_DEVELOPMENT.md`
- [ ] T030 [P] CHANGELOG; trạng thái F108 và cột FR chính (thêm FR-LMS-004) trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T031 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/108-competition-config-portal/quickstart.md`
- [ ] T032 [P] Test F108-AC-17: tắt `menu.school_lms.challenges` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/competition/feature-toggle.spec.ts`
- [ ] T033 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F108-AC-17) — file: `src/cms/frontend/apps/portal/src/workspaces/game-ops/competition/routes.tsx`

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T034 **OQ chốt 2026-11-20** (OQ F054-3, F045-3) `content_refs[]` trong event publish cấu hình thi đấu + `ref_type = competition` ở learning; facade platform-ops của module competition — file: `specs/events/payloads/competition/`, `src/backend/modules/competition/`, `src/backend/modules/learning/internal/presentation/consumer/` — kiểm: `make specs-lint`, `make test-contract` — cỡ M

## Dependencies & Execution Order

- Setup → Foundational (T003–T007) → US1 (backend) → US2, US3 (song song, web) → Polish.
- US3 cần API F107 (T013, T018, T023, T029 của F107); T017 cần F106 `MaterializeDefinitions`; US2 cần khung workspace A của F079.

## Definition of Done

- [ ] Mọi tiêu chí F108-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

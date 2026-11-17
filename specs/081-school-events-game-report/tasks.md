# Tasks: F081 — Workspace C — sự kiện trường từ template

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Route lazy cho mục menu "Sự kiện trường" = `menu.school_lms.school_events` (registry sinh từ catalog, F051; không khai báo tay) trong chunk `school-lms` (mục "Hoạt động game" thuộc F080 T055) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/routes.tsx`
- [ ] T002 [P] Khóa i18n vi/en cho module `portal.school_lms.school_events.*` và map `ProblemCode` → thông điệp — file: `src/cms/frontend/apps/portal/src/i18n/locales/vi/portal.school_lms.json`, `src/cms/frontend/apps/portal/src/i18n/locales/en/portal.school_lms.json`
- [ ] T003 [P] Seed: template "Tuần lễ Toán học" có community goal gán `th-demo-a` — file: `deploy/seed/game-config/event-templates/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Contract: thêm `getSchoolEventProgress` + schema `SchoolEventProgress`; `EventTemplate.quest_option_items`/`challenge_option_items`; `SchoolEvent.status` + enum tham số `status` của `listSchoolEvents` (kiểm delta F077 T005 đã có `409`/`503`/`428`) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/game.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [ ] T006 Sinh client web từ OpenAPI mới — file: `src/cms/frontend/apps/portal/src/shared/api/schema.gen.d.ts`, `src/cms/frontend/apps/portal/src/shared/api/operation-meta.gen.ts` — kiểm: `pnpm gen:api`, `make gen-check MOD=game`

**Checkpoint**: contract và client sinh sẵn sàng; không có migration (không bảng mới).

## Phase 3: User Story 1 — Tạo sự kiện trường từ template (Priority: P1) 🎯 MVP

**Goal**: School/Tenant Admin tạo sự kiện chỉ với tham số template cho phép.

**Independent Test**: như spec US1.

### Tests cho User Story 1

- [ ] T007 [P] [US1] Unit test F081-AC-01, F081-AC-02: danh sách template, trạng thái rỗng, form chỉ dựng `allowed_params`, picker từ `*_option_items`, đối tượng theo lớp trong scope — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/CreateSchoolEventWizard.test.tsx` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Unit test F081-AC-03, F081-AC-04: `Idempotency-Key` giữ khi retry; map `EVENT_TEMPLATE_PARAM_NOT_ALLOWED`/`VALIDATION_FAILED` về field, `503` giữ form, `409` thông điệp — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/createSubmit.test.tsx`
- [ ] T009 [P] [US1] Unit test F081-AC-05: `vi` bắt buộc, `en` tùy chọn, bộ đếm `maxLength`, ngày ICT — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/fields.test.tsx`

### Implementation cho User Story 1

- [ ] T010 [P] [US1] Hook `useEventTemplates`, `useCreateSchoolEvent` (Idempotency-Key một lần/ý định) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/api/eventTemplates.ts`, `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/api/schoolEvents.ts`
- [ ] T011 [US1] `CreateSchoolEventWizard` + `AllowedParamField`, `AudiencePicker` (`listTenantSchoolClasses`), `TemplateOptionPicker`, `mapProblem.ts` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/CreateSchoolEventWizard.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Danh sách, sửa, hủy sự kiện trường (Priority: P1)

### Tests cho User Story 2

- [ ] T012 [P] [US2] Unit test F081-AC-06, F081-AC-07: cột/lọc, chế độ System Admin (`tenant_ids`, cột Tenant, `TenantTargetField`), `If-Match`, `412`/`409` → tải lại, ẩn Sửa theo trạng thái — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/SchoolEventListPage.test.tsx`
- [ ] T013 [P] [US2] Unit test F081-AC-08: lý do 3–1000 ký tự, xác nhận 2 bước, `Idempotency-Key` — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/CancelSchoolEventDialog.test.tsx`
- [ ] T014 [P] [US2] E2E authorization + tenant isolation F081-AC-09: `qlcm.truong.a` → 403 không menu; `ad.truong.b` URL sự kiện TH Demo A → "Không tìm thấy" — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/school-events.spec.ts`

### Implementation cho User Story 2

- [ ] T015 [US2] `SchoolEventListPage`, `SchoolEventDetailPage`, `EditSchoolEventForm`, `CancelSchoolEventDialog` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/SchoolEventListPage.tsx`

## Phase 5: User Story 3 — Tiến độ sự kiện trường (Priority: P2)

### Tests cho User Story 3

- [ ] T016 [P] [US3] Integration test F081-AC-10: tổng từ `community_goal_progress` (xóa Valkey không đổi kết quả), `by_class` chỉ dải khối EVENT_SEASON_SYSTEM §6, không `student_id`/tên/hạng trong JSON — file: `src/backend/modules/game/tests/integration/liveops/school_event_progress_test.go` — phải FAIL trước khi code
- [ ] T017 [P] [US3] Integration test authorization + tenant isolation F081-AC-12: thiếu permission 403; trường khác/tenant khác 404; `X-Tenant-Id` lệch 403 `TENANT_MISMATCH` — file: `src/backend/modules/game/tests/integration/liveops/school_event_progress_authz_test.go`
- [ ] T018 [P] [US3] Unit test UI F081-AC-11: thanh mốc, bảng lớp sắp theo tên, không cột hạng, "Cập nhật lúc" — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/SchoolEventProgressPanel.test.tsx`
- [ ] T019 [P] [US3] Contract test provider `getSchoolEventProgress` theo OpenAPI — file: `src/backend/modules/game/tests/contract/school_event_progress_contract_test.go`

### Implementation cho User Story 3

- [ ] T020 [P] [US3] Domain `ClassBreakdownPolicy` (dải khối, tham chiếu EVENT_SEASON_SYSTEM §6) — file: `src/backend/modules/game/internal/domain/liveops/class_breakdown_policy.go`
- [ ] T021 [US3] Query `GetSchoolEventProgress` + repo SQL (`tenant_id`, `deleted_at IS NULL`, `GROUP BY class_id` ⨝ `ref.classes`) + handler — file: `src/backend/modules/game/internal/application/query/liveops/get_school_event_progress.go`, `src/backend/modules/game/internal/infrastructure/persistence/liveops/school_event_progress_repo.go`, `src/backend/modules/game/internal/presentation/http/public/school_events.go`
- [ ] T022 [US3] `SchoolEventProgressPanel`, `ClassContributionTable`, hook `useSchoolEventProgress` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/progress/SchoolEventProgressPanel.tsx`

## Phase 6: User Story 4 — Ràng buộc chung của màn (Priority: P2)

- [ ] T029 [P] [US4] E2E F081-AC-19: lỗi hiển thị `code` + `trace_id`; viewport điện thoại chỉ xem; axe 0 lỗi serious (TC-UI-006) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/school-events-common.spec.ts`
- [ ] T030 [P] [US4] Kiểm lint không import component/Theme game, không hardcode style (TC-UI-003, TC-UI-010) — file: `src/cms/frontend/.eslintrc.cjs`, `src/cms/frontend/eslint-rules/no-game-imports.js` — kiểm: `make web-lint`

## Phase 7: Polish & Cross-Cutting

- [ ] T031 [P] Cập nhật tài liệu: API_DESIGN (`getSchoolEventProgress`), đề xuất đăng ký dải khối xem đóng góp theo lớp vào CONFIG_DEFAULTS, CHANGELOG — file: `docs/07-backend/API_DESIGN.md`, `CHANGELOG.md`
- [ ] T032 [P] Visual regression danh sách/tiến độ sáng/tối (TC-UI-004) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/visual.spec.ts`
- [ ] T033 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/081-school-events-game-report/quickstart.md`
- [ ] T034 [P] Test F081-AC-20: tắt `menu.school_lms.school_events` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/school-events-feature-toggle.spec.ts`
- [ ] T035 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F081-AC-20) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T004, T006) → US1 → US2 → US3 (cần backend T020–T021 trước T022) → US4 → Polish.
- Phụ thuộc ngoài: F052 (chunk `school-lms`), F077 US5 Done (API sự kiện trường). Không phụ thuộc F080: task T005, T023–T028 cũ (báo cáo hoạt động game) đã chuyển sang F080 T055–T061; mã T005, T023–T028 bỏ trống.

## Definition of Done

- [ ] Mọi tiêu chí F081-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (module game) và `make web-lint`, `make web-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

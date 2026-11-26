# Tasks: F081 — Workspace C — sự kiện trường từ template

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Route lazy cho mục menu "Sự kiện trường" = `menu.school_lms.school_events` (registry sinh từ catalog, F051; không khai báo tay) trong chunk `school-lms` (mục "Hoạt động game" thuộc F080 T055) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/routes.tsx`
- [x] T002 [P] Khóa i18n vi/en cho module `portal.school_lms.school_events.*` và map `ProblemCode` → thông điệp — file: `src/cms/frontend/apps/portal/src/i18n/locales/vi/portal.school_lms.json`, `src/cms/frontend/apps/portal/src/i18n/locales/en/portal.school_lms.json`
- [x] T003 [P] Seed: template "Tuần lễ Toán học" có community goal gán `th-demo-a` — file: `deploy/seed/game-config/event-templates/`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Contract: thêm `getSchoolEventProgress` + schema `SchoolEventProgress`; `EventTemplate.quest_option_items`/`challenge_option_items`; `SchoolEvent.status` + enum tham số `status` của `listSchoolEvents` (kiểm delta F077 T005 đã có `409`/`503`/`428`) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/game.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [x] T006 Sinh client web từ OpenAPI mới — file: `src/cms/frontend/apps/portal/src/shared/api/schema.gen.d.ts`, `src/cms/frontend/apps/portal/src/shared/api/operation-meta.gen.ts` — kiểm: `pnpm gen:api`, `make gen-check MOD=game`

**Checkpoint**: contract và client sinh sẵn sàng; không có migration (không bảng mới).

## Phase 3: User Story 1 — Tạo sự kiện trường từ template (Priority: P1) 🎯 MVP

**Goal**: School/Tenant Admin tạo sự kiện chỉ với tham số template cho phép.

**Independent Test**: như spec US1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Unit test F081-AC-01, F081-AC-02: danh sách template, trạng thái rỗng, form chỉ dựng `allowed_params`, picker từ `*_option_items`, đối tượng theo trường/khối trong scope (ADR-0095) — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/CreateSchoolEventWizard.test.tsx` — phải FAIL trước khi code
- [x] T008 [P] [US1] Unit test F081-AC-03, F081-AC-04: `Idempotency-Key` giữ khi retry; map `EVENT_TEMPLATE_PARAM_NOT_ALLOWED`/`VALIDATION_FAILED` về field, `503` giữ form, `409` thông điệp — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/createSubmit.test.tsx`
- [x] T009 [P] [US1] Unit test F081-AC-05: `vi` bắt buộc, `en` tùy chọn, bộ đếm `maxLength`, ngày ICT — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/fields.test.tsx`

### Implementation cho User Story 1

- [x] T010 [P] [US1] Hook `useEventTemplates`, `useCreateSchoolEvent` (Idempotency-Key một lần/ý định) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/api/eventTemplates.ts`, `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/api/schoolEvents.ts`
- [x] T011 [US1] `CreateSchoolEventWizard` + `AllowedParamField`, `AudiencePicker` (trường + khối từ `grade_levels` của template — ADR-0095), `TemplateOptionPicker`, `mapProblem.ts` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/CreateSchoolEventWizard.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Danh sách, sửa, hủy sự kiện trường (Priority: P1)

### Tests cho User Story 2

- [x] T012 [P] [US2] Unit test F081-AC-06, F081-AC-07: cột/lọc, chế độ System Admin (`tenant_ids`, cột Tenant, `TenantTargetField`), `If-Match`, `412`/`409` → tải lại, ẩn Sửa theo trạng thái — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/SchoolEventListPage.test.tsx`
- [x] T013 [P] [US2] Unit test F081-AC-08: lý do 3–1000 ký tự, xác nhận 2 bước, `Idempotency-Key` — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/CancelSchoolEventDialog.test.tsx`
- [ ] T014 [P] [US2] E2E authorization + tenant isolation F081-AC-09: `qlcm.truong.a` → 403 không menu; `ad.truong.b` URL sự kiện TH Demo A → "Không tìm thấy" — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/school-events.spec.ts`

### Implementation cho User Story 2

- [x] T015 [US2] `SchoolEventListPage`, `SchoolEventDetailPage`, `EditSchoolEventForm`, `CancelSchoolEventDialog` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/SchoolEventListPage.tsx`

## Phase 5: User Story 3 — Tiến độ sự kiện trường (Priority: P2)

### Tests cho User Story 3

- [x] T016 [P] [US3] Integration test F081-AC-10: tổng từ `community_goal_progress` (xóa Valkey không đổi kết quả), `by_grade` chỉ dải khối EVENT_SEASON_SYSTEM §6, khối < 5 học sinh đóng góp bị ẩn, không `student_id`/tên/hạng trong JSON — file: `src/backend/modules/game/tests/integration/liveops/school_event_progress_test.go` — phải FAIL trước khi code
- [x] T017 [P] [US3] Integration test authorization + tenant isolation F081-AC-12: thiếu permission 403; trường khác/tenant khác 404; `X-Tenant-Id` lệch 403 `TENANT_MISMATCH` — file: `src/backend/modules/game/tests/integration/liveops/school_event_progress_authz_test.go`
- [x] T018 [P] [US3] Unit test UI F081-AC-11: thanh mốc, bảng khối sắp theo khối, không cột hạng, "Cập nhật lúc" — file: `src/cms/frontend/apps/portal/tests/unit/school-lms/school-events/SchoolEventProgressPanel.test.tsx`
- [x] T019 [P] [US3] Contract test provider `getSchoolEventProgress` theo OpenAPI — file: `src/backend/modules/game/tests/contract/school_event_progress_contract_test.go`

### Implementation cho User Story 3

- [x] T020 [P] [US3] Domain `GradeBreakdownPolicy` (dải khối + ẩn ô < 5, tham chiếu EVENT_SEASON_SYSTEM §6, ADR-0095) — file: `src/backend/modules/game/internal/domain/liveops/grade_breakdown_policy.go`
- [x] T021 [US3] Query `GetSchoolEventProgress` + repo SQL (`tenant_id`, `deleted_at IS NULL`, `GROUP BY grade_level`) + handler — file: `src/backend/modules/game/internal/application/query/liveops/get_school_event_progress.go`, `src/backend/modules/game/internal/infrastructure/persistence/liveops/school_event_progress_repo.go`, `src/backend/modules/game/internal/presentation/http/public/school_events.go`
- [x] T022 [US3] `SchoolEventProgressPanel`, `GradeContributionTable`, hook `useSchoolEventProgress` — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/progress/SchoolEventProgressPanel.tsx`

## Phase 6: User Story 4 — Ràng buộc chung của màn (Priority: P2)

- [ ] T029 [P] [US4] E2E F081-AC-19: lỗi hiển thị `code` + `trace_id`; viewport điện thoại chỉ xem; axe 0 lỗi serious (TC-UI-006) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/school-events-common.spec.ts`
- [x] T030 [P] [US4] Kiểm lint không import component/Theme game, không hardcode style (TC-UI-003, TC-UI-010) — file: `src/cms/frontend/.eslintrc.cjs`, `src/cms/frontend/eslint-rules/no-game-imports.js` — kiểm: `make web-lint`

## Phase 7: Polish & Cross-Cutting

- [x] T031 [P] Cập nhật tài liệu: API_DESIGN (`getSchoolEventProgress`), đề xuất đăng ký dải khối xem đóng góp theo khối vào CONFIG_DEFAULTS, CHANGELOG — file: `docs/07-backend/API_DESIGN.md`, `CHANGELOG.md`
- [ ] T032 [P] Visual regression danh sách/tiến độ sáng/tối (TC-UI-004) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/visual.spec.ts`
- [ ] T033 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/081-school-events-game-report/quickstart.md`
- [ ] T034 [P] Test F081-AC-20: tắt `menu.school_lms.school_events` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/school-lms/school-events-feature-toggle.spec.ts`
- [x] T035 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F081-AC-20) — file: `src/cms/frontend/apps/portal/src/workspaces/school-lms/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T004, T006) → US1 → US2 → US3 (cần backend T020–T021 trước T022) → US4 → Polish.
- Phụ thuộc ngoài: F052 (chunk `school-lms`), F077 US5 Done (API sự kiện trường). Không phụ thuộc F080: task T005, T023–T028 cũ (báo cáo hoạt động game) đã chuyển sang F080 T055–T061; mã T005, T023–T028 bỏ trống.


## Ghi chú triển khai (2026-11-25/26)

- **Vị trí file:** test Vitest theo quy ước portal ở `src/cms/frontend/apps/portal/src/workspaces/school-lms/school-events/__tests__/` (`SchoolEvents.test.tsx` dựng cả portal với MOCK `src/test/school-events-mock.ts`; `mapProblem.test.ts` thuần), không ở `tests/unit/...`. Màn ở `school-events/` (`SchoolEventsScreen`, `SchoolEventListPage`, `CreateSchoolEventWizard`, `SchoolEventForm` — gộp AllowedParamField/AudiencePicker/TemplateOptionPicker, `SchoolEventDetailPage`, `EditSchoolEventPage`, `CancelSchoolEventDialog`, `SchoolEventProgressPanel` + `GradeContributionTable`, `api.ts`, `mapProblem.ts`, `ui.tsx`); i18n `portal.school_lms.schoolEvents.*`. Route gắn vào `screens` của workspace C (lazy chunk) theo catalog — không khai báo menu tay.
- **Backend:** query `GetSchoolEventProgress` ở `internal/application/query/liveops_school_progress.go`, SQL ở `internal/infrastructure/persistence/liveopssql/school_progress.go`, handler + route trong `internal/presentation/http/liveops.go`; integration test ở `tests/integration/liveops_school_progress_test.go` (thư mục phẳng hiện có, không `tests/integration/liveops/`); contract provider là `tests/contract/routes_test.go` (`TestOperationsMatchSpec`: method, path, chế độ tenant `TenantIdHeaderOptional`, idempotency, status — 88 operation).
- **T020:** dùng lại `liveops.VisibleGrades` của F077 (dải khối 4–9 + ẩn khối < `analytics_min_cell_size`, đã có test domain) thay vì tạo `GradeBreakdownPolicy` trùng (R33).
- **T003:** seed template "Tuần lễ Toán học" có community goal, gán `th-demo-a`, đã có từ F077 (`src/backend/modules/game/seed/liveops/templates.v1.json` + `assignments.v1.json`).
- **T030:** lint `no-game-imports`, stylelint chặn hardcode đã có (F050/F052); file mới qua `eslint --max-warnings=0`.
- **Chưa làm (cần backend compose + Playwright):** T014, T029, T034 (E2E), T032 (visual regression), T033 (quickstart thủ công). Các tiêu chí của chúng đã có test Vitest toàn portal + integration test module game (bảng dưới).

## AC → test

| Tiêu chí | Test |
|---|---|
| F081-AC-01 | Vitest `SchoolEvents.test.tsx` › "AC-01, AC-02, AC-05 …" (template của tenant, tên/loại/mô tả/thời lượng tối đa), › "AC-01: a tenant without an assigned template …" (trạng thái rỗng, không form); Go `TestSchoolEvents` (template của tenant) |
| F081-AC-02 | Vitest › "AC-01, AC-02, AC-05 …" (chỉ `allowed_params`, quest từ `quest_option_items`, khối 1–9, không ô luật/điểm, body không có tham số ngoài template); `mapProblem.test.ts` › "only the allowed parameters leave the form"; Go `TestSchoolEventsCrossTenant` (`quest_option_items` tiêu đề + khối của version đang phát hành) |
| F081-AC-03 | Vitest › "AC-03, AC-04 …" (cùng `Idempotency-Key` khi thử lại sau `503`, tạo đúng một sự kiện, mở chi tiết) |
| F081-AC-04 | Vitest › "AC-03, AC-04 …" (từ nhạy cảm → lỗi chung tại ô tên, không hiện từ; `503` giữ form + code/trace_id; `409` → đạt tối đa); `mapProblem.test.ts` › "422 VALIDATION_FAILED …", "422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED …"; Go `TestSchoolEvents` (`errors[].field` của `EVENT_TEMPLATE_PARAM_NOT_ALLOWED`) |
| F081-AC-05 | Vitest › "AC-01, AC-02, AC-05 …" (`maxlength` 255/4000, vi bắt buộc, giờ ICT → UTC, thời lượng + tối đa của template); `fieldSchemas.test.ts` (giới hạn khớp OpenAPI); `mapProblem.test.ts` › "ICT wall time ↔ UTC" |
| F081-AC-06 | Vitest › "AC-06, AC-07, AC-08 …" (danh sách tenant phiên, lọc trạng thái trong query, `suspended`), › "AC-06: System Admin lists several tenants …" (`tenant_ids` lặp, không `X-Tenant-Id`, cột Tenant, chọn tenant đích); Go `TestSchoolEventsCrossTenant` (liên tenant, keyset, audit, tenant user bỏ qua `tenant_ids`, role platform khác 403), `pkg/httpx` `TestTenantIDsOnlyForSystemAdmin` (`tenant_ids` lặp lại) |
| F081-AC-07 | Vitest › "AC-06, AC-07, AC-08 …" (`If-Match` "1", `412` → thông báo + Tải lại → `If-Match` "2"; sự kiện live/suspended không có nút Sửa); `mapProblem.test.ts` › "409 … 412 …", "edit only while scheduled" |
| F081-AC-08 | Vitest › "AC-06, AC-07, AC-08 …" (lý do 3–1000 có bộ đếm, xác nhận 2 bước "phần thưởng … giữ nguyên", `Idempotency-Key`) |
| F081-AC-09 | Vitest › "AC-09: without lms.school_event.write …" (trang 403, không menu; sự kiện tenant khác → "Không tìm thấy") |
| F081-AC-10 | Go `TestSchoolEventProgress` (tổng từ PostgreSQL khi bộ đếm Valkey sai, số tham gia, mốc, `by_grade` chỉ khối 4–9 và ≥ 5 học sinh, theo `show_grade_contributions`, `data_as_of`, không `student_id`/tên/nickname/hạng, `school_id` cho sự kiện nhiều trường) |
| F081-AC-11 | Vitest › "AC-11: participants, bar with the server milestones …" (mốc của server, bảng khối sắp tăng dần, không cột hạng, "Cập nhật lúc", làm mới thủ công) |
| F081-AC-12 | Go `TestSchoolEventProgress` (`academic_manager` 403 `PERMISSION_DENIED` + `authz.denied`, trường khác/tenant khác/trường ngoài đối tượng 404, `X-Tenant-Id` lệch 403 `TENANT_MISMATCH`, token học sinh 403; `authztest.Cover` CrossTenant + RoleDenied) |
| F081-AC-19 | Vitest › "AC-19, AC-20 …" (điện thoại: "Tác vụ này cần màn hình lớn hơn", danh sách dạng thẻ; lỗi hiện `code` + `trace_id`) |
| F081-AC-20 | Vitest › "AC-19, AC-20 …" (action tắt → ẩn "Tạo sự kiện", menu tắt → ẩn, `403 FEATURE_DISABLED` → thông báo + tải lại `getMyPermissions`) |

## Definition of Done

- [x] Mọi tiêu chí F081-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (module game) và `make web-lint`, `make web-test` xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

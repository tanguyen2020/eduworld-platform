# Tasks: F054 — Workspace B — media, bộ câu hỏi/lesson/blueprint, version và rollback, hàng đợi duyệt, preview, import, thùng rác

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Gốc mã `web/apps/portal/src/workspaces/content-studio/`. Dùng lại F053 (`ContentEditorShell`, `RichContentEditor`, `renderMath`, `MediaPicker`, `useWorkflow`) và F051 (`withTenantScope`, `useIdempotentMutation`, `ReasonDialog`, `mapValidationErrors`, `LargeScreenRequired`). Mock MSW chỉ trong test (`// MOCK (test-only)`).

## Phase 1: Setup

- [ ] T001 Thêm mục menu workspace B (Duyệt nội dung, Thư viện media, Bộ câu hỏi, Bài học, Ma trận đề, Danh sách nội dung, Import, Thùng rác) theo khóa menu từ `specs/portal/menu-catalog.yaml` (registry sinh, F051; không khai báo menu tay); mục không có trong catalog xem "Khoảng trống catalog" trong plan.md — file: `web/apps/portal/src/workspaces/content-studio/routes.tsx`
- [ ] T002 [P] Khóa i18n cho màn phát hành/duyệt/import/thùng rác vi/en — file: `web/apps/portal/src/i18n/locales/vi/portal.content_studio.json`, `web/apps/portal/src/i18n/locales/en/portal.content_studio.json`
- [ ] T003 [P] Playwright project `@F054`, fixture CSV import (2 dòng lỗi, 1 công thức cấm), ảnh hợp lệ, file `.exe` đổi đuôi — file: `web/apps/portal/tests/e2e/fixtures/content/import_questions.csv`
- [ ] T004 [P] Ghim phiên bản `xlsx`, `papaparse` — file: `web/apps/portal/package.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T005 Contract: thêm `listContentUsages` + schema item — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T006 Contract: thêm `deleteContent` (soft-delete theo `content_type`) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T007 Contract: thêm `submitLearningPreviewResponse` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T008 Contract: thêm `cancelScheduledPublish` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T009 Contract: sửa enum `lane` (`tenant`, `platform_learning`) của `ContentReviewTask`/`listContentReviewTasks`; thêm `field_comments[]` vào body `transitionContentVersion` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T010 Contract: thêm bộ lọc `topic_id`, `objective_id`, `difficulty`, `translation_status` cho `searchQuestions`; `topic_id`, `objective_id` cho list question set/lesson/blueprint — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T011 Contract: thêm `pool_sizes[]` vào response `getCmsQuestionSet` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T012 Contract: thêm `math_assets[]` vào `ContentImportRequest` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`
- [ ] T013 Sinh lại kiểu client — file: `web/apps/portal/src/api/schema.d.ts` — kiểm: `pnpm gen:api`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Lịch sử version, xuất bản hẹn giờ, rollback, preview (Priority: P1) 🎯 MVP

**Goal**: Quản lý version/release và preview theo ADR-0043.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T014 [P] [US1] Unit test `releaseOptions`: version hợp lệ để rollback (published, ≠ con trỏ, cùng item) và archive (≠ con trỏ) (F054-AC-05, F054-AC-08) — file: `web/apps/portal/tests/unit/content-studio/release/releaseOptions.test.ts` — phải FAIL trước khi code
- [ ] T015 [P] [US1] Unit test `PublishDialog`: `publish_at` ≥ hiện tại + `scheduled_publish_min_lead`, hiển thị ICT gửi UTC, tóm tắt diff, cảnh báo media chưa `ready`, hủy hẹn giờ (F054-AC-04) — file: `web/apps/portal/tests/unit/content-studio/release/PublishDialog.test.tsx`
- [ ] T016 [P] [US1] Unit test `RollbackDialog`: lý do bắt buộc, xác nhận lần 2, mô tả ảnh hưởng, map `ROLLBACK_TARGET_INVALID`/403, ẩn khi thiếu `content.release.rollback` (F054-AC-05, F054-AC-06) — file: `web/apps/portal/tests/unit/content-studio/release/RollbackDialog.test.tsx`
- [ ] T017 [P] [US1] Unit test `VersionHistoryTab`/`ReleaseTimeline`: cột, Tag trạng thái, version published/archived chỉ đọc, không có thao tác sửa/xóa version published (F054-AC-01, F054-AC-02, F054-AC-09) — file: `web/apps/portal/tests/unit/content-studio/release/VersionHistoryTab.test.tsx`
- [ ] T018 [P] [US1] Unit test `VersionDiffView` ẩn diff `answer_key` khi thiếu `learning_content.review` (F054-AC-03) — file: `web/apps/portal/tests/unit/content-studio/release/VersionDiffView.test.tsx`
- [ ] T019 [P] [US1] Unit test `UnpublishDialog` (lý do, danh sách nơi dùng) và `ArchiveDialog` (F054-AC-07, F054-AC-08) — file: `web/apps/portal/tests/unit/content-studio/release/UnpublishArchive.test.tsx`
- [ ] T020 [P] [US1] Unit test `TryOutRunner`: công tắc vi/en/song song, gợi ý từng cấp, phản hồi phương án từ `submitLearningPreviewResponse` (F054-AC-10) — file: `web/apps/portal/tests/unit/content-studio/preview/TryOutRunner.test.tsx`
- [ ] T021 [P] [US1] Unit test giữ Idempotency-Key khi thử lại cho publish/rollback/unpublish/archive (F054-AC-11) — file: `web/apps/portal/tests/unit/content-studio/release/idempotency.test.ts`
- [ ] T022 [US1] E2E phát hành: lịch sử, hẹn giờ + hủy, rollback khi `hs.an.3a` đang làm (giữ version ghim), gỡ xuất bản, archive (F054-AC-01, F054-AC-02, F054-AC-04, F054-AC-05, F054-AC-07, F054-AC-08; TC-CMS-001, TC-CNT-002, TC-CNT-004) — file: `web/apps/portal/tests/e2e/content-studio/release.spec.ts`
- [ ] T023 [US1] E2E preview/thử làm không tạo attempt; bấm Xuất bản hai lần → một release (F054-AC-10, F054-AC-11; TC-CNT-003) — file: `web/apps/portal/tests/e2e/content-studio/preview.spec.ts`
- [ ] T024 [US1] E2E authorization + tenant isolation cho `listContentVersions`, `diffContentVersions`, `rollbackContent`, `cancelScheduledPublish` (mới), `submitLearningPreviewResponse` (mới): `ad.truong.b` với item `th-demo-a` → 404; người thiếu permission không thấy nút, gọi thẳng → 403 (F054-AC-03, F054-AC-06, F054-AC-09) — file: `web/apps/portal/tests/e2e/content-studio/release-authz.spec.ts`

### Implementation cho User Story 1

- [ ] T025 [P] [US1] `useVersions`, `useRelease`, `releaseOptions.ts` — file: `web/apps/portal/src/workspaces/content-studio/release/useRelease.ts`
- [ ] T026 [US1] `VersionHistoryTab`, `ReleaseTimeline`, `VersionDiffView` (tab Phiên bản/Phát hành trong `ContentEditorShell`) — file: `web/apps/portal/src/workspaces/content-studio/release/VersionHistoryTab.tsx`
- [ ] T027 [US1] `PublishDialog`, `RollbackDialog`, `UnpublishDialog`, `ArchiveDialog` — file: `web/apps/portal/src/workspaces/content-studio/release/RollbackDialog.tsx`
- [ ] T028 [US1] `PreviewPanel`, `TryOutRunner`, `usePreview` — file: `web/apps/portal/src/workspaces/content-studio/preview/PreviewPanel.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Hàng đợi duyệt (Priority: P1)

**Goal**: Reviewer xử lý việc chờ duyệt theo lane.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T029 [P] [US2] Unit test `ReviewQueuePage`: lọc lane/`content_type`/"giao cho tôi", cảnh báo quá hạn (F054-AC-12) — file: `web/apps/portal/tests/unit/content-studio/review/ReviewQueuePage.test.tsx` — phải FAIL trước khi code
- [ ] T030 [P] [US2] Unit test `ReviewTaskPage`: mặc định diff với version đang xuất bản + preview, từ chối cần lý do, `field_comments` (F054-AC-13) — file: `web/apps/portal/tests/unit/content-studio/review/ReviewTaskPage.test.tsx`
- [ ] T031 [P] [US2] Unit test `BulkApproveDialog`: giới hạn số item, Idempotency-Key riêng mỗi item, kết quả từng item (F054-AC-14) — file: `web/apps/portal/tests/unit/content-studio/review/BulkApproveDialog.test.tsx`
- [ ] T032 [US2] E2E duyệt: `cm.noidung` gửi 3 câu, `cm.duyet` duyệt hàng loạt + từ chối; tenant isolation: `ad.truong.a` không thấy lane platform/tenant khác (F054-AC-12, F054-AC-13, F054-AC-14, F054-AC-15; TC-CMS-001, TC-CMS-TEN-01) — file: `web/apps/portal/tests/e2e/content-studio/review.spec.ts`

### Implementation cho User Story 2

- [ ] T033 [P] [US2] `useReviewTasks`, `ReviewQueuePage` — file: `web/apps/portal/src/workspaces/content-studio/review/ReviewQueuePage.tsx`
- [ ] T034 [US2] `ReviewTaskPage` (dùng `VersionDiffView`, `PreviewPanel`, `useWorkflow`) — file: `web/apps/portal/src/workspaces/content-studio/review/ReviewTaskPage.tsx`
- [ ] T035 [US2] `BulkApproveDialog` — file: `web/apps/portal/src/workspaces/content-studio/review/BulkApproveDialog.tsx`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Thư viện media và "đang dùng ở đâu" (Priority: P2)

**Goal**: Quản lý media tập trung, báo cáo nơi sử dụng.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T036 [P] [US3] Unit test `MediaLibraryPage`: lọc `kind`/`status`/`q`, hiển thị `reject_reason`, biến thể (F054-AC-16) — file: `web/apps/portal/tests/unit/content-studio/media/MediaLibraryPage.test.tsx` — phải FAIL trước khi code
- [ ] T037 [P] [US3] Unit test `MediaUploadDropzone`: kiểm loại/dung lượng trước presign, bắt buộc `alt_text`/`license`, map `MEDIA_TYPE_NOT_ALLOWED`/`MEDIA_TOO_LARGE` (F054-AC-17) — file: `web/apps/portal/tests/unit/content-studio/media/MediaUploadDropzone.test.tsx`
- [ ] T038 [P] [US3] Unit test `MediaDetailDrawer`: `used_in`, không có thao tác ghi đè (F054-AC-18) — file: `web/apps/portal/tests/unit/content-studio/media/MediaDetailDrawer.test.tsx`
- [ ] T039 [P] [US3] Unit test `UsageTab` và hộp xác nhận archive/gỡ/xóa hiển thị nơi dùng (F054-AC-19) — file: `web/apps/portal/tests/unit/content-studio/usage/UsageTab.test.tsx`
- [ ] T040 [US3] E2E media + nơi sử dụng; tenant isolation: `ad.truong.b` gọi `getMediaAsset`/`listContentUsages` (mới) của `th-demo-a` → 404 (F054-AC-16, F054-AC-17, F054-AC-18, F054-AC-19; TC-CNT-008) — file: `web/apps/portal/tests/e2e/content-studio/media.spec.ts`

### Implementation cho User Story 3

- [ ] T041 [US3] `MediaLibraryPage`, `MediaUploadDropzone`, `MediaDetailDrawer` — file: `web/apps/portal/src/workspaces/content-studio/media/MediaLibraryPage.tsx`
- [ ] T042 [P] [US3] `UsageTab`, `useContentUsages` — file: `web/apps/portal/src/workspaces/content-studio/usage/UsageTab.tsx`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Question Set, Lesson, Assessment Blueprint builder (Priority: P2)

**Goal**: Ba builder dùng `ContentEditorShell`.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T043 [P] [US4] Unit test `SelectionRuleEditor`: chế độ tĩnh/luật theo `question-set.schema.json`, hiển thị `pool_sizes` (F054-AC-20) — file: `web/apps/portal/tests/unit/content-studio/question-sets/SelectionRuleEditor.test.tsx` — phải FAIL trước khi code
- [ ] T044 [P] [US4] Unit test `LessonBlockList`: kéo-thả 4 loại khối, ≤ `maxItems`, khối bắt buộc, `completion_rule`, `math_assets` (F054-AC-21) — file: `web/apps/portal/tests/unit/content-studio/lessons/LessonBlockList.test.tsx`
- [ ] T045 [P] [US4] Unit test `BlueprintMatrix`: ma trận LO × độ khó, tổng điểm, `QUESTION_POOL_INSUFFICIENT` chỉ ô thiếu (F054-AC-22) — file: `web/apps/portal/tests/unit/content-studio/blueprints/BlueprintMatrix.test.tsx`
- [ ] T046 [P] [US4] Unit test lưu draft kèm `If-Match`, `CONTENT_NOT_EDITABLE` → chỉ đọc (F054-AC-23) — file: `web/apps/portal/tests/unit/content-studio/builders/draftSave.test.ts`
- [ ] T047 [US4] E2E ba builder + sinh đề mẫu; authorization: `cm.duyet` không tạo/sửa được (F054-AC-20, F054-AC-21, F054-AC-22, F054-AC-23; TC-CNT-003) — file: `web/apps/portal/tests/e2e/content-studio/builders.spec.ts`

### Implementation cho User Story 4

- [ ] T048 [US4] `QuestionSetEditorPage`, `SelectionRuleEditor`, `useQuestionSet` — file: `web/apps/portal/src/workspaces/content-studio/question-sets/QuestionSetEditorPage.tsx`
- [ ] T049 [US4] `LessonBuilderPage`, `LessonBlockList`, `useLesson` — file: `web/apps/portal/src/workspaces/content-studio/lessons/LessonBuilderPage.tsx`
- [ ] T050 [US4] `BlueprintEditorPage`, `BlueprintMatrix`, `useBlueprint` (sinh đề mẫu qua `createLearningPreviewSession`) — file: `web/apps/portal/src/workspaces/content-studio/blueprints/BlueprintEditorPage.tsx`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Danh sách và tìm kiếm, import hàng loạt, xóa mềm và Thùng rác (Priority: P3)

**Goal**: Tìm kiếm, import có chạy thử, xóa mềm, khôi phục.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T051 [P] [US5] Unit test `ContentFilters` → tham số query (`q`, môn, khối, loại, trạng thái, ngôn ngữ, nguồn, `topic_id`, `objective_id`), cột Tenant cho System Admin (F054-AC-24) — file: `web/apps/portal/tests/unit/content-studio/list/ContentFilters.test.tsx` — phải FAIL trước khi code
- [ ] T052 [P] [US5] Unit test `DeleteContentDialog`: `If-Match`, lý do bắt buộc với published, gọi `deleteQuestion`/`deleteContent` (F054-AC-25) — file: `web/apps/portal/tests/unit/content-studio/list/DeleteContentDialog.test.tsx`
- [ ] T053 [P] [US5] Unit test `TenantTrashPage`: lọc `resource_type`, khôi phục, `restorable = false`/409 (F054-AC-26) — file: `web/apps/portal/tests/unit/content-studio/trash/TenantTrashPage.test.tsx`
- [ ] T054 [P] [US5] Unit test `parseImportFile` + `renderImportMath`: lỗi công thức theo dòng trước khi gửi, gom `math_assets`, chia theo `request_body_max` (F054-AC-27) — file: `web/apps/portal/tests/unit/content-studio/import/parseImport.test.ts`
- [ ] T055 [US5] E2E danh sách/xóa/thùng rác/import; tenant isolation: `ad.truong.b` gọi `deleteContent` (mới), `restoreTenantTrashItem`, `getContentImport` của `th-demo-a` → 404 (F054-AC-24, F054-AC-25, F054-AC-26, F054-AC-27; TC-CMS-DEL-01, TC-CMS-DEL-02, TC-TEN-015, TC-CMS-TEN-02) — file: `web/apps/portal/tests/e2e/content-studio/list-import-trash.spec.ts`
- [ ] T056 [US5] E2E ẩn hành động theo permission (`cm.duyet`, `qlcm.truong.a`, người không có `tenant.trash.read`) và xử lý 403 (F054-AC-28) — file: `web/apps/portal/tests/e2e/content-studio/permissions.spec.ts`

### Implementation cho User Story 5

- [ ] T057 [US5] `ContentListPage`, `ContentFilters`, `useContentList` — file: `web/apps/portal/src/workspaces/content-studio/list/ContentListPage.tsx`
- [ ] T058 [P] [US5] `DeleteContentDialog` — file: `web/apps/portal/src/workspaces/content-studio/list/DeleteContentDialog.tsx`
- [ ] T059 [P] [US5] `TenantTrashPage`, `useTenantTrash` — file: `web/apps/portal/src/workspaces/content-studio/trash/TenantTrashPage.tsx`
- [ ] T060 [US5] `ImportWizardPage`, `parseImportFile`, `renderImportMath`, `useContentImport` (upload → `createContentImport` dry-run → `getContentImport` → chạy thật) — file: `web/apps/portal/src/workspaces/content-studio/import/ImportWizardPage.tsx`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase 8: Polish & Cross-Cutting

- [ ] T061 [P] E2E giới hạn `maxLength` + map `VALIDATION_FAILED` (F054-AC-29; TC-TEN-024); < 768 px dạng thẻ, duyệt được, builder/diff/import → "cần màn hình lớn hơn" + axe (F054-AC-30; TC-UI-006) — file: `web/apps/portal/tests/e2e/content-studio/responsive-a11y.spec.ts`
- [ ] T062 [P] Story Storybook sáng/tối cho `VersionDiffView`, `RollbackDialog`, `ReviewTaskPage`, `BlueprintMatrix` (TC-UI-005) — file: `web/apps/portal/src/workspaces/content-studio/release/release.stories.tsx`
- [ ] T063 [P] Cập nhật ADMIN_PORTAL §3.5/§4 (lane theo contract), CHANGELOG — file: `docs/04-admin-portal/ADMIN_PORTAL.md`, `CHANGELOG.md`
- [ ] T064 [P] Đăng ký vào CONFIG_DEFAULTS: giới hạn duyệt hàng loạt, SLA hàng đợi duyệt (tài liệu sở hữu ADMIN_PORTAL §4), giới hạn dung lượng media theo loại (CONTENT_MODEL §7), TTL preview (ADMIN_PORTAL §5); portal đọc từ cấu hình — file: `docs/00-overview/CONFIG_DEFAULTS.md`
- [ ] T065 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/054-content-studio-publishing/quickstart.md`
- [ ] T066 Cập nhật trạng thái F054 — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T067 [P] Test F054-AC-31: tắt `action.content_studio.lessons.rollback` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `web/apps/portal/tests/e2e/content-studio/publishing-feature-toggle.spec.ts`
- [ ] T068 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F054-AC-31) — file: `web/apps/portal/src/workspaces/content-studio/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T005–T013; backend owner F024–F027 hiện thực delta) → US1, US2 (song song) → US3, US4 (song song) → US5 → Polish.
- T026 cần T025; T034 cần T026, T028; T049 cần F053 `RichContentEditor`, `WorkedExampleBlock`; T060 cần F053 `renderMath`.

## Definition of Done

- [ ] Mọi tiêu chí F054-AC-* có test pass (ghi mã test/TC)
- [ ] `make web-lint`, `make web-test`, Playwright `@F054`, `make storybook` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

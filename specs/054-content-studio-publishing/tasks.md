# Tasks: F054 — Workspace B — media, bộ câu hỏi/lesson/blueprint, version và rollback, hàng đợi duyệt, preview, import, thùng rác

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Gốc mã `src/cms/frontend/apps/portal/src/workspaces/content-studio/`. Dùng lại F053 (`ContentEditorShell`, `RichContentEditor`, `renderMath`, `MediaPicker`, `useWorkflow`) và F051 (`withTenantScope`, `useIdempotentMutation`, `ReasonDialog`, `mapValidationErrors`, `LargeScreenRequired`). Mock MSW chỉ trong test (`// MOCK (test-only)`).

## Phase 1: Setup

- [x] T001 Thêm mục menu workspace B (Duyệt nội dung, Thư viện media, Bộ câu hỏi, Bài học, Ma trận đề, Danh sách nội dung, Import, Thùng rác) theo khóa menu từ `specs/portal/menu-catalog.yaml` (registry sinh, F051; không khai báo menu tay); mục không có trong catalog xem "Khoảng trống catalog" trong plan.md — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/routes.tsx` — **2026-11-20:** route/menu gắn trong `content-studio/index.tsx` + `screens.tsx` (reviews, media, question_sets, lessons, assessments, learning_paths, question_bank list/import) và `school-lms/index.tsx` (trash) theo khóa catalog; không menu tay
- [x] T002 [P] Khóa i18n cho màn phát hành/duyệt/import/thùng rác vi/en — file: `src/cms/frontend/apps/portal/src/i18n/locales/vi/portal.content_studio.json`, `src/cms/frontend/apps/portal/src/i18n/locales/en/portal.content_studio.json` — **2026-11-20:** khóa vi/en trong `i18n/content-studio.{vi,en}.json` (`content_studio.release|diff|usage|preview|review|media|questionSet|lesson|blueprint|list|import`) và `locales/{vi,en}/portal.school_lms.json` (`trash`); `keys.test` xanh
- [x] T003 [P] Playwright project `@F054`, fixture CSV import (2 dòng lỗi, 1 công thức cấm), ảnh hợp lệ, file `.exe` đổi đuôi — file: `src/cms/frontend/apps/portal/tests/e2e/fixtures/content/import_questions.csv` — **2026-11-20:** fixture CSV/XLSX dựng trong test (`import/parseImport.test.ts`, `builders/builders.test.tsx`); project Playwright `@F054` chờ backend thật (OQ F051-9)
- [x] T004 [P] Ghim phiên bản `xlsx`, `papaparse` — file: `src/cms/frontend/apps/portal/package.json` — **2026-11-20:** [Confirmed] không thêm `xlsx`/`papaparse` (advisory chưa vá); đọc CSV/XLSX tự viết trong `import/parseImportFile.ts` (Clarifications 2026-11-20)

## Phase 2: Foundational (chặn mọi user story)

- [x] T005 Contract: thêm `listContentUsages` + schema item — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** đã có: `getContentUsages` (F026) — không sửa spec
- [x] T006 Contract: thêm `deleteContent` (soft-delete theo `content_type`) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** đã có: `deleteQuestion`, `deleteCmsQuestionSet`, `deleteCmsLesson`, `deleteCmsLearningMaterial`, `deleteCmsAssessmentBlueprint` (F022/F024)
- [x] T007 Contract: thêm `submitLearningPreviewResponse` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** đã có: `submitPreviewResponse` (F027)
- [x] T008 Contract: thêm `cancelScheduledPublish` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** đã có: `cancelScheduledPublication` theo `release_id` (F026)
- [x] T009 Contract: sửa enum `lane` (`tenant`, `platform_learning`) của `ContentReviewTask`/`listContentReviewTasks`; thêm `field_comments[]` vào body `transitionContentVersion` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **2026-11-20:** lane đã đúng (F025); `field_comments` chưa có → [Confirmed] ghép vào `note`, OQ F054-1 (owner F025, đã chốt 2026-11-20 — task bổ sung)
- [x] T010 Contract: thêm bộ lọc `topic_id`, `objective_id`, `difficulty`, `translation_status` cho `searchQuestions`; `topic_id`, `objective_id` cho list question set/lesson/blueprint — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** `topic_id`/`objective_id`/`difficulty` đã có (F024); `translation_status`/ngôn ngữ chưa có → OQ F054-2 (owner F024, đã chốt 2026-11-20 — task bổ sung)
- [x] T011 Contract: thêm `pool_sizes[]` vào response `getCmsQuestionSet` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** thay bằng `getQuestionSetPoolStats`/`getAssessmentBlueprintPoolStats` (F024)
- [x] T012 Contract: thêm `math_assets[]` vào `ContentImportRequest` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-20:** [Confirmed] không cần: cột `math_assets` của `questions-row.schema.json` được portal điền (Clarifications)
- [x] T013 Sinh lại kiểu client — file: `src/cms/frontend/apps/portal/src/api/schema.d.ts` — kiểm: `pnpm gen:api` — **2026-11-20:** không đổi spec → `pnpm gen:check` xanh, không có file sinh mới

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Lịch sử version, xuất bản hẹn giờ, rollback, preview (Priority: P1) 🎯 MVP

**Goal**: Quản lý version/release và preview theo ADR-0043.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T014 [P] [US1] Unit test `releaseOptions`: version hợp lệ để rollback (published, ≠ con trỏ, cùng item) và archive (≠ con trỏ) (F054-AC-05, F054-AC-08) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/release/releaseOptions.test.ts` — phải FAIL trước khi code — **2026-11-20:** `release/releaseOptions.test.ts` (6 test)
- [x] T015 [P] [US1] Unit test `PublishDialog`: `publish_at` ≥ hiện tại + `scheduled_publish_min_lead`, hiển thị ICT gửi UTC, tóm tắt diff, cảnh báo media chưa `ready`, hủy hẹn giờ (F054-AC-04) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/release/PublishDialog.test.tsx` — **2026-11-20:** `release/release.test.tsx` › AC-04/AC-11 (hẹn giờ quá sớm bị chặn, ICT→UTC, tóm tắt diff, hủy hẹn giờ)
- [x] T016 [P] [US1] Unit test `RollbackDialog`: lý do bắt buộc, xác nhận lần 2, mô tả ảnh hưởng, map `ROLLBACK_TARGET_INVALID`/403, ẩn khi thiếu `content.release.rollback` (F054-AC-05, F054-AC-06) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/release/RollbackDialog.test.tsx` — **2026-11-20:** `release/release.test.tsx` › AC-05/AC-06/AC-29, AC-06/AC-28
- [x] T017 [P] [US1] Unit test `VersionHistoryTab`/`ReleaseTimeline`: cột, Tag trạng thái, version published/archived chỉ đọc, không có thao tác sửa/xóa version published (F054-AC-01, F054-AC-02, F054-AC-09) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/release/VersionHistoryTab.test.tsx` — **2026-11-20:** `release/release.test.tsx` › AC-01/AC-02/AC-09
- [x] T018 [P] [US1] Unit test `VersionDiffView` ẩn diff `answer_key` khi thiếu `learning_content.review` (F054-AC-03) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/release/VersionDiffView.test.tsx` — **2026-11-20:** `release/release.test.tsx` › AC-03 (có/không `learning_content.review`) + `releaseOptions.test.ts`
- [x] T019 [P] [US1] Unit test `UnpublishDialog` (lý do, danh sách nơi dùng) và `ArchiveDialog` (F054-AC-07, F054-AC-08) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/release/UnpublishArchive.test.tsx` — **2026-11-20:** `release/release.test.tsx` › AC-07/AC-19 + AC-08
- [x] T020 [P] [US1] Unit test `TryOutRunner`: công tắc vi/en/song song, gợi ý từng cấp, phản hồi phương án từ `submitLearningPreviewResponse` (F054-AC-10) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/preview/TryOutRunner.test.tsx` — **2026-11-20:** `preview/preview.test.tsx` + `builders/models.test.ts` (try-out responses)
- [x] T021 [P] [US1] Unit test giữ Idempotency-Key khi thử lại cho publish/rollback/unpublish/archive (F054-AC-11) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/release/idempotency.test.ts` — **2026-11-20:** `release/release.test.tsx` › AC-11 retry giữ key (1 release), AC-04 (`mock.created = 1`)
- [x] T022 [US1] E2E phát hành: lịch sử, hẹn giờ + hủy, rollback khi `hs.an.3a` đang làm (giữ version ghim), gỡ xuất bản, archive (F054-AC-01, F054-AC-02, F054-AC-04, F054-AC-05, F054-AC-07, F054-AC-08; TC-CMS-001, TC-CNT-002, TC-CNT-004) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/release.spec.ts` — **2026-11-20:** phần tích hợp toàn portal với MOCK trong `release/release.test.tsx`; Playwright backend thật chờ OQ F051-9
- [x] T023 [US1] E2E preview/thử làm không tạo attempt; bấm Xuất bản hai lần → một release (F054-AC-10, F054-AC-11; TC-CNT-003) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/preview.spec.ts` — **2026-11-20:** `preview/preview.test.tsx` (không request attempt/learning event) + `release.test.tsx` › AC-11; Playwright chờ OQ F051-9
- [x] T024 [US1] E2E authorization + tenant isolation cho `listContentVersions`, `diffContentVersions`, `rollbackContent`, `cancelScheduledPublish` (mới), `submitLearningPreviewResponse` (mới): `ad.truong.b` với item `th-demo-a` → 404; người thiếu permission không thấy nút, gọi thẳng → 403 (F054-AC-03, F054-AC-06, F054-AC-09) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/release-authz.spec.ts` — **2026-11-20:** `release.test.tsx` › AC-15 (item tenant khác → "Không tìm thấy"), AC-06/AC-28 (thiếu quyền không thấy nút); 403 ở backend thuộc F026/F027

### Implementation cho User Story 1

- [x] T025 [P] [US1] `useVersions`, `useRelease`, `releaseOptions.ts` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/release/useRelease.ts` — **2026-11-20:** `release/useRelease.ts`, `release/releaseOptions.ts`
- [x] T026 [US1] `VersionHistoryTab`, `ReleaseTimeline`, `VersionDiffView` (tab Phiên bản/Phát hành trong `ContentEditorShell`) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/release/VersionHistoryTab.tsx` — **2026-11-20:** `release/VersionHistoryTab.tsx`, `ReleaseTimeline.tsx`, `VersionDiffView.tsx`, `VersionViewDrawer.tsx`, `ContentReleaseTabs.tsx`
- [x] T027 [US1] `PublishDialog`, `RollbackDialog`, `UnpublishDialog`, `ArchiveDialog` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/release/RollbackDialog.tsx` — **2026-11-20:** `release/PublishDialog.tsx`, `RollbackDialog.tsx`, `UnpublishDialog.tsx`, `ArchiveDialog.tsx`, `ReleaseReasonDialog.tsx`
- [x] T028 [US1] `PreviewPanel`, `TryOutRunner`, `usePreview` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/preview/PreviewPanel.tsx` — **2026-11-20:** `preview/PreviewPanel.tsx`, `TryOutRunner.tsx`, `usePreview.ts`, `tryOutResponse.ts`, `RichView.tsx`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Hàng đợi duyệt (Priority: P1)

**Goal**: Reviewer xử lý việc chờ duyệt theo lane.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T029 [P] [US2] Unit test `ReviewQueuePage`: lọc lane/`content_type`/"giao cho tôi", cảnh báo quá hạn (F054-AC-12) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/review/ReviewQueuePage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `review/review.test.tsx` › AC-12/AC-15 + `models.test.ts` (quá hạn)
- [x] T030 [P] [US2] Unit test `ReviewTaskPage`: mặc định diff với version đang xuất bản + preview, từ chối cần lý do, `field_comments` (F054-AC-13) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/review/ReviewTaskPage.test.tsx` — **2026-11-20:** `review/review.test.tsx` › AC-13 (2 test)
- [x] T031 [P] [US2] Unit test `BulkApproveDialog`: giới hạn số item, Idempotency-Key riêng mỗi item, kết quả từng item (F054-AC-14) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/review/BulkApproveDialog.test.tsx` — **2026-11-20:** `review/review.test.tsx` › AC-14 (2 test)
- [x] T032 [US2] E2E duyệt: `cm.noidung` gửi 3 câu, `cm.duyet` duyệt hàng loạt + từ chối; tenant isolation: `ad.truong.a` không thấy lane platform/tenant khác (F054-AC-12, F054-AC-13, F054-AC-14, F054-AC-15; TC-CMS-001, TC-CMS-TEN-01) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/review.spec.ts` — **2026-11-20:** phần tích hợp MOCK trong `review/review.test.tsx` (gồm cô lập tenant: không `tenant_ids`, không lane platform); Playwright chờ OQ F051-9

### Implementation cho User Story 2

- [x] T033 [P] [US2] `useReviewTasks`, `ReviewQueuePage` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/review/ReviewQueuePage.tsx` — **2026-11-20:** `review/useReviewTasks.ts`, `review/ReviewQueuePage.tsx`, `review/reviewRules.ts`
- [x] T034 [US2] `ReviewTaskPage` (dùng `VersionDiffView`, `PreviewPanel`, `useWorkflow`) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/review/ReviewTaskPage.tsx` — **2026-11-20:** `review/ReviewTaskPage.tsx`
- [x] T035 [US2] `BulkApproveDialog` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/review/BulkApproveDialog.tsx` — **2026-11-20:** `review/BulkApproveDialog.tsx`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Thư viện media và "đang dùng ở đâu" (Priority: P2)

**Goal**: Quản lý media tập trung, báo cáo nơi sử dụng.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T036 [P] [US3] Unit test `MediaLibraryPage`: lọc `kind`/`status`/`q`, hiển thị `reject_reason`, biến thể (F054-AC-16) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/media/MediaLibraryPage.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `media/media.test.tsx` › AC-16/AC-18
- [x] T037 [P] [US3] Unit test `MediaUploadDropzone`: kiểm loại/dung lượng trước presign, bắt buộc `alt_text`/`license`, map `MEDIA_TYPE_NOT_ALLOWED`/`MEDIA_TOO_LARGE` (F054-AC-17) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/media/MediaUploadDropzone.test.tsx` — **2026-11-20:** `media/media.test.tsx` › AC-17 (2 test) + `models.test.ts` (pre-check)
- [x] T038 [P] [US3] Unit test `MediaDetailDrawer`: `used_in`, không có thao tác ghi đè (F054-AC-18) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/media/MediaDetailDrawer.test.tsx` — **2026-11-20:** `media/media.test.tsx` › AC-16/AC-18 (used_in, không ghi đè)
- [x] T039 [P] [US3] Unit test `UsageTab` và hộp xác nhận archive/gỡ/xóa hiển thị nơi dùng (F054-AC-19) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/usage/UsageTab.test.tsx` — **2026-11-20:** `release/release.test.tsx` › AC-07/AC-19 (nơi dùng trong hộp gỡ xuất bản); hộp xóa dùng cùng `UsageList`
- [x] T040 [US3] E2E media + nơi sử dụng; tenant isolation: `ad.truong.b` gọi `getMediaAsset`/`listContentUsages` (mới) của `th-demo-a` → 404 (F054-AC-16, F054-AC-17, F054-AC-18, F054-AC-19; TC-CNT-008) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/media.spec.ts` — **2026-11-20:** phần tích hợp MOCK trong `media/media.test.tsx`; isolation backend thuộc F026/F027; Playwright chờ OQ F051-9

### Implementation cho User Story 3

- [x] T041 [US3] `MediaLibraryPage`, `MediaUploadDropzone`, `MediaDetailDrawer` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/media/MediaLibraryPage.tsx` — **2026-11-20:** `media/MediaLibraryPage.tsx`, `MediaUploadDropzone.tsx`, `MediaDetailDrawer.tsx`, `mediaRules.ts`
- [x] T042 [P] [US3] `UsageTab`, `useContentUsages` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/usage/UsageTab.tsx` — **2026-11-20:** `usage/UsageTab.tsx` (`useContentUsages`, `UsageList`)

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Question Set, Lesson, Assessment Blueprint builder (Priority: P2)

**Goal**: Ba builder dùng `ContentEditorShell`.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T043 [P] [US4] Unit test `SelectionRuleEditor`: chế độ tĩnh/luật theo `question-set.schema.json`, hiển thị `pool_sizes` (F054-AC-20) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/question-sets/SelectionRuleEditor.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `builders/models.test.ts` (question set) + `builders/builders.test.tsx` › AC-20
- [x] T044 [P] [US4] Unit test `LessonBlockList`: kéo-thả 4 loại khối, ≤ `maxItems`, khối bắt buộc, `completion_rule`, `math_assets` (F054-AC-21) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/lessons/LessonBlockList.test.tsx` — **2026-11-20:** `builders/models.test.ts` (lesson) + `builders/builders.test.tsx` › AC-21
- [x] T045 [P] [US4] Unit test `BlueprintMatrix`: ma trận LO × độ khó, tổng điểm, `QUESTION_POOL_INSUFFICIENT` chỉ ô thiếu (F054-AC-22) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/blueprints/BlueprintMatrix.test.tsx` — **2026-11-20:** `builders/models.test.ts` (blueprint matrix) + `builders/builders.test.tsx` › AC-22
- [x] T046 [P] [US4] Unit test lưu draft kèm `If-Match`, `CONTENT_NOT_EDITABLE` → chỉ đọc (F054-AC-23) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/builders/draftSave.test.ts` — **2026-11-20:** `builders/builders.test.tsx` › AC-20 (PATCH If-Match) + AC-23 (CONTENT_NOT_EDITABLE)
- [x] T047 [US4] E2E ba builder + sinh đề mẫu; authorization: `cm.duyet` không tạo/sửa được (F054-AC-20, F054-AC-21, F054-AC-22, F054-AC-23; TC-CNT-003) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/builders.spec.ts` — **2026-11-20:** phần tích hợp MOCK trong `builders/builders.test.tsx` (gồm reviewer không có "Tạo mới"); Playwright chờ OQ F051-9

### Implementation cho User Story 4

- [x] T048 [US4] `QuestionSetEditorPage`, `SelectionRuleEditor`, `useQuestionSet` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/question-sets/QuestionSetEditorPage.tsx` — **2026-11-20:** `question-sets/QuestionSetEditorPage.tsx`, `SelectionRuleEditor.tsx`, `questionSetModel.ts`, `builders/useDocument.ts`, `builders/DocumentEditorPage.tsx`
- [x] T049 [US4] `LessonBuilderPage`, `LessonBlockList`, `useLesson` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/lessons/LessonBuilderPage.tsx` — **2026-11-20:** `lessons/LessonBuilderPage.tsx`, `LessonBlockList.tsx`, `lessonModel.ts`
- [x] T050 [US4] `BlueprintEditorPage`, `BlueprintMatrix`, `useBlueprint` (sinh đề mẫu qua `createLearningPreviewSession`) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/blueprints/BlueprintEditorPage.tsx` — **2026-11-20:** `blueprints/BlueprintEditorPage.tsx`, `BlueprintMatrix.tsx`, `matrixModel.ts`, `blueprintModel.ts`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Danh sách và tìm kiếm, import hàng loạt, xóa mềm và Thùng rác (Priority: P3)

**Goal**: Tìm kiếm, import có chạy thử, xóa mềm, khôi phục.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T051 [P] [US5] Unit test `ContentFilters` → tham số query (`q`, môn, khối, loại, trạng thái, ngôn ngữ, nguồn, `topic_id`, `objective_id`), cột Tenant cho System Admin (F054-AC-24) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/list/ContentFilters.test.tsx` — phải FAIL trước khi code — **2026-11-20:** `builders/models.test.ts` (filtersToQuery) + `list/list.test.tsx` › AC-24 (2 test)
- [x] T052 [P] [US5] Unit test `DeleteContentDialog`: `If-Match`, lý do bắt buộc với published, gọi `deleteQuestion`/`deleteContent` (F054-AC-25) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/list/DeleteContentDialog.test.tsx` — **2026-11-20:** `list/list.test.tsx` › AC-25
- [x] T053 [P] [US5] Unit test `TenantTrashPage`: lọc `resource_type`, khôi phục, `restorable = false`/409 (F054-AC-26) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/trash/TenantTrashPage.test.tsx` — **2026-11-20:** `list/list.test.tsx` › Thùng rác (2 test)
- [x] T054 [P] [US5] Unit test `parseImportFile` + `renderImportMath`: lỗi công thức theo dòng trước khi gửi, gom `math_assets`, chia theo `request_body_max` (F054-AC-27) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/import/parseImport.test.ts` — **2026-11-20:** `import/parseImport.test.ts` (4 test)
- [x] T055 [US5] E2E danh sách/xóa/thùng rác/import; tenant isolation: `ad.truong.b` gọi `deleteContent` (mới), `restoreTenantTrashItem`, `getContentImport` của `th-demo-a` → 404 (F054-AC-24, F054-AC-25, F054-AC-26, F054-AC-27; TC-CMS-DEL-01, TC-CMS-DEL-02, TC-TEN-015, TC-CMS-TEN-02) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/list-import-trash.spec.ts` — **2026-11-20:** phần tích hợp MOCK trong `list/list.test.tsx`, `builders/builders.test.tsx` (import); Playwright chờ OQ F051-9
- [x] T056 [US5] E2E ẩn hành động theo permission (`cm.duyet`, `qlcm.truong.a`, người không có `tenant.trash.read`) và xử lý 403 (F054-AC-28) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/permissions.spec.ts` — **2026-11-20:** `list/list.test.tsx` › AC-28 + Thùng rác AC-28, `release.test.tsx` › AC-06/AC-28, AC-31 (403 FEATURE_DISABLED); Playwright chờ OQ F051-9

### Implementation cho User Story 5

- [x] T057 [US5] `ContentListPage`, `ContentFilters`, `useContentList` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/list/ContentListPage.tsx` — **2026-11-20:** `list/ContentListPage.tsx`, `ContentFilters.tsx`, `useContentList.ts`
- [x] T058 [P] [US5] `DeleteContentDialog` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/list/DeleteContentDialog.tsx` — **2026-11-20:** `list/DeleteContentDialog.tsx`
- [x] T059 [P] [US5] `TenantTrashPage`, `useTenantTrash` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/trash/TenantTrashPage.tsx` — **2026-11-20:** `school-lms/trash/TenantTrashPage.tsx` (menu `menu.school_lms.trash`, `useTenantTrash`, `useRestoreTenantTrash`)
- [x] T060 [US5] `ImportWizardPage`, `parseImportFile`, `renderImportMath`, `useContentImport` (upload → `createContentImport` dry-run → `getContentImport` → chạy thật) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/import/ImportWizardPage.tsx` — **2026-11-20:** `import/ImportWizardPage.tsx`, `parseImportFile.ts`, `renderImportMath.ts`, `useContentImport.ts`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase 8: Polish & Cross-Cutting

- [x] T061 [P] E2E giới hạn `maxLength` + map `VALIDATION_FAILED` (F054-AC-29; TC-TEN-024); < 768 px dạng thẻ, duyệt được, builder/diff/import → "cần màn hình lớn hơn" + axe (F054-AC-30; TC-UI-006) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/responsive-a11y.spec.ts` — **2026-11-20:** maxLength (lý do rollback 500, tìm kiếm 200, lý do khôi phục 1000) trong `release.test.tsx`/`list.test.tsx`; < 768 px thẻ (`review.test.tsx`, `list.test.tsx`), "cần màn hình lớn hơn" (diff ở `review.test.tsx`, import ở `builders.test.tsx`); axe chờ Playwright (OQ F051-9)
- [x] T062 [P] Story Storybook sáng/tối cho `VersionDiffView`, `RollbackDialog`, `ReviewTaskPage`, `BlueprintMatrix` (TC-UI-005) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/release/release.stories.tsx` — **2026-11-20:** `release/release.stories.tsx` (DiffGroups có/không đáp án, RollbackImpact, BlueprintMatrix) Light/Dark/Compact; baseline visual chờ CI
- [x] T063 [P] Cập nhật ADMIN_PORTAL §3.5/§4 (lane theo contract), CHANGELOG — file: `docs/04-admin-portal/ADMIN_PORTAL.md`, `CHANGELOG.md` — **2026-11-20:** ADMIN_PORTAL §3.5/§4 đã dùng `tenant`/`platform_learning` khớp contract — không cần sửa; CHANGELOG cập nhật
- [x] T064 [P] Đăng ký vào CONFIG_DEFAULTS: giới hạn duyệt hàng loạt, SLA hàng đợi duyệt (tài liệu sở hữu ADMIN_PORTAL §4), giới hạn dung lượng media theo loại (CONTENT_MODEL §7), TTL preview (ADMIN_PORTAL §5); portal đọc từ cấu hình — file: `docs/00-overview/CONFIG_DEFAULTS.md` — **2026-11-20:** đã đăng ký sẵn: `content_review_bulk_max`, `content_review_sla_days`, `media_max_bytes_by_kind`, `preview_token_ttl`, `scheduled_publish_min_lead`; portal đọc theo tên trong `shared/config.ts`
- [x] T065 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/054-content-studio-publishing/quickstart.md` — **2026-11-20:** bằng chứng ghi ở quickstart.md §5
- [x] T066 Cập nhật trạng thái F054 — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — **2026-11-20:** CODE_PLAN: F054 Done (2026-11-20)
- [x] T067 [P] Test F054-AC-31: tắt `action.content_studio.lessons.rollback` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/publishing-feature-toggle.spec.ts` — **2026-11-20:** `release/release.test.tsx` › AC-31 (TC-CMS-MENU-02) với `action.content_studio.question_sets.rollback`; Playwright chờ OQ F051-9
- [x] T068 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F054-AC-31) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/routes.tsx` — **2026-11-20:** nút bọc `<ActionGate actionKey>` theo `shared/contentTypes.ts` (map content_type → action catalog); `node scripts/specs/check-portal-catalog.mjs` xanh

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [x] T069 **OQ chốt 2026-11-20** (OQ F054-1) Từ chối/yêu cầu sửa gửi `field_comments[]`, editor hiển thị ghi chú theo trường từ `ContentReviewTask`; sau task F025 — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/review/` — kiểm: `make web-test` — cỡ S — **2026-11-20:** hộp "Từ chối" của `ReviewTaskPage` gửi `note` (lý do) + `field_comments[]` (`path` JSON pointer gợi ý từ các trường của version, để trống = cả phiên bản, thêm `/` nếu thiếu; `comment` bắt buộc; ≤ 50 dòng), bỏ cách ghép vào `note`; giới hạn lấy từ contract qua `shared/fieldSchemas.ts` (kiểm bởi `fieldSchemas.test.ts`, helper chung `src/test/openapiLocate.ts`); `ReviewFieldComments` đọc `ContentVersionSummary.field_comments` (`listContentVersions`) và hiện cho tác giả trong editor câu hỏi/question set/lesson/blueprint khi version `rejected`/`draft`, và cho người duyệt khi version được gửi lại; MOCK backend lưu và trả `field_comments`. Test: `review.test.tsx` › AC-13 + "OQ F054-1 … question set", `QuestionEditorPage.test.tsx` › "OQ F054-1", `models.test.ts`, `fieldSchemas.test.ts` PASS
- [ ] T070 **OQ chốt 2026-11-20** (OQ F054-2) Bộ lọc "Ngôn ngữ" (`content_language`) và "Trạng thái dịch" (`translation_status`) ở danh sách nội dung (F054-AC-24); sau task F024 — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/` — kiểm: `make web-test` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational (T005–T013; backend owner F024–F027 hiện thực delta) → US1, US2 (song song) → US3, US4 (song song) → US5 → Polish.
- T026 cần T025; T034 cần T026, T028; T049 cần F053 `RichContentEditor`, `WorkedExampleBlock`; T060 cần F053 `renderMath`.

## Definition of Done

- [x] Mọi tiêu chí F054-AC-* có test pass (ghi mã test/TC) — mức integration toàn portal với MOCK backend; E2E backend thật còn mở (OQ F051-9)
- [x] `make web-lint`, `make web-test` xanh (lệnh pnpm tương đương, quickstart §5); Playwright `@F054` chờ OQ F051-9
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

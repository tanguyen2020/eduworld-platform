# Tasks: F058 — Workspace B — Learning Activity Builder (quiz, interactive_exercise, video_lesson), UI re-evaluate

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Route lazy `/portal/content-studio/activities{,/new,/:activityId}` trong chunk `content-studio`, mục menu "Hoạt động học tập" = `menu.content_studio.activities` của catalog (registry sinh, F051; hiện theo `required_permissions` và `disabled_menus`) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/activities/routes.tsx`
- [ ] T002 [P] Thêm `hls.js`, `ajv` (2020) + `ajv-formats` vào `ops-ui`; sinh schema activity bằng `pnpm gen:schemas` — file: `src/cms/frontend/packages/ops-ui/package.json`, `src/cms/frontend/apps/portal/src/api/schemas/activities/`
- [ ] T003 [P] File dịch vi/en (`portal.content_studio.activities.*`, `error.ACTIVITY_CONFIG_INVALID`, `error.ACTIVITY_TYPE_NOT_SUPPORTED`, `error.VIDEO_NOT_READY`, `error.EVALUATION_NOT_EDITABLE`, nhãn chấm lại) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/activities/i18n/vi.json`, `en.json`
- [ ] T004 [P] Playwright project tag `@F058`, fixture video ngắn + `.vtt` — file: `src/cms/frontend/apps/portal/tests/e2e/fixtures/content/activities.json`

## Phase 2: Foundational (chặn mọi user story)

- [x] T005 Delta contract (owner F030): `createCmsActivityVersion` (đã có: F030 T054, 2026-11-20) — file: `specs/openapi/modules/learning.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T006 Delta contract (owner F031): `previewRegradeRequest` + `RegradeImpact`, `listRegradeRequests`, thêm trường version vào `RegradeRequest` (cùng `getRegradeRequest` của F031) — file: `specs/openapi/modules/learning.v1.yaml` — kiểm: `make specs-lint`
- [x] T007 Delta contract (owner F027): `MediaKind` thêm `subtitle` (`text/vtt`); cập nhật CONTENT_MODEL §7 — file: `specs/openapi/modules/learning.v1.yaml`, `docs/02-learning/CONTENT_MODEL.md` — kiểm: `make specs-lint` — ✅ làm ở F027 T040 (2026-11-20): `MediaKind.subtitle` (`text/vtt`), CONTENT_MODEL §7, migration `20261221000001`
- [ ] T008 Sửa ADMIN_PORTAL §3.13 permission builder = `learning_content.activity.write`; ghi CHANGELOG — file: `docs/04-admin-portal/ADMIN_PORTAL.md`, `CHANGELOG.md` — kiểm: `make docs-check`
- [ ] T009 Sinh lại client TS — file: `src/cms/frontend/apps/portal/src/api/generated/eduworld-api.d.ts` — kiểm: `pnpm gen:api` + CI diff
- [ ] T010 [P] `SchemaForm` (ajv draft 2020-12, map lỗi JSON Pointer ↔ field, bộ đếm `maxLength`) + test — file: `src/cms/frontend/packages/ops-ui/src/activity-editor/SchemaForm.tsx`, `SchemaForm.test.tsx`
- [ ] T011 [P] MSW handler test-only (nhãn `MOCK`) cho API activity, preview, regrade, media — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/activities/msw/handlers.ts`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Soạn quiz và interactive_exercise (Priority: P1) 🎯 MVP

**Goal**: Tạo/sửa activity câu hỏi theo schema của loại, autosave, version.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T012 [P] [US1] Test F058-AC-01: bước 1 chỉ loại có editor và `status ≠ disabled`, không có `written_response`/`speaking`/`project` — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/activities/ActivityTypeStep.test.tsx` — phải FAIL trước khi code
- [ ] T013 [P] [US1] Test F058-AC-02: metadata bắt buộc, validate schema tức thời, `createCmsActivity` có `Idempotency-Key` — file: `…/activities/ActivityEditorPage.create.test.tsx`
- [ ] T014 [P] [US1] Test F058-AC-03: ba `questionSource.kind`, chặn loại câu ngoài `allowed_question_types`, `maxItems` (TC-ACT-003) — file: `src/cms/frontend/packages/ops-ui/src/activity-editor/widgets/QuestionSourceField.test.tsx`
- [ ] T015 [P] [US1] Test F058-AC-04: chiến lược chấm lọc theo `evaluation_strategies`, `FeedbackTiming`, `completion_rule`, `timed.total_seconds` — file: `…/activity-editor/widgets/EvaluationField.test.tsx`
- [ ] T016 [P] [US1] Test F058-AC-05: autosave `If-Match`, `412`/`409 CONTENT_VERSION_CONFLICT` dừng autosave (TC-CNT-003) — file: `…/activities/ActivityEditorPage.autosave.test.tsx`
- [ ] T017 [P] [US1] Test F058-AC-06: `422 ACTIVITY_CONFIG_INVALID` ánh xạ `errors[].field` (TC-ACT-003) — file: `…/activities/ActivityEditorPage.errors.test.tsx`
- [ ] T018 [P] [US1] Test F058-AC-07: version `published` chỉ đọc, "Tạo bản nháp từ v{n}" gọi `createCmsActivityVersion`, `CONTENT_NOT_EDITABLE` (TC-ACT-005) — file: `…/activities/ActivityEditorPage.published.test.tsx`
- [ ] T019 [P] [US1] Test F058-AC-08: `math_assets` gửi kèm, `MATH_RENDER_FAILED` tô đỏ (TC-UI-013) — file: `…/activities/ActivityEditorPage.math.test.tsx`
- [ ] T020 [P] [US1] Test F058-AC-09: `BilingualField` theo `content_language`, khối 1–3 thiếu audio hướng dẫn khóa Gửi duyệt, `AUDIO_REQUIRED` (TC-CNT-010) — file: `…/activities/ActivityEditorPage.audio.test.tsx`

### Implementation cho User Story 1

- [ ] T021 [P] [US1] `editorRegistry` + hook `useActivityTypes` — file: `…/activities/editorRegistry.ts`, `…/activities/api/activityTypes.ts`
- [ ] T022 [P] [US1] `QuestionPicker`, widget `QuestionSourceField`, `EvaluationField`, `FeedbackTimingField`, `CompletionRuleField`, `TimedQuizField` + story — file: `src/cms/frontend/packages/ops-ui/src/activity-editor/QuestionPicker.tsx`, `…/activity-editor/widgets/*.tsx`
- [ ] T023 [US1] Hook `useActivity` (get/create/update/createVersion, ETag, Idempotency-Key) — file: `…/activities/api/activities.ts`
- [ ] T024 [US1] `ActivityTypeStep`, `ActivityEditorPage` trong `ContentEditorShell` (F053), `ActivityListPage` — file: `…/activities/ActivityTypeStep.tsx`, `ActivityEditorPage.tsx`, `ActivityListPage.tsx`
- [ ] T025 [US1] Kiểm tra sớm `questionSourceRules` — file: `…/activities/validators/questionSourceRules.ts`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Preview và workflow (Priority: P1)

### Tests cho User Story 2

- [ ] T026 [P] [US2] Test F058-AC-10: `createLearningPreviewSession` (`content_type = activity`, `display_language`), công tắc vi/en, tạo lại khi hết hạn — file: `…/activities/ActivityPreviewPanel.test.tsx` — phải FAIL trước khi code
- [ ] T027 [P] [US2] Test F058-AC-11: khung học sinh không render `answer_key`/`options[].feedback`/gợi ý (TC-LRN-012) — file: `…/activities/ActivityPreviewPanel.noAnswers.test.tsx`
- [ ] T028 [P] [US2] Test F058-AC-12: Gửi duyệt/Duyệt/Từ chối/Xuất bản theo permission, ẩn Duyệt với tác giả, lý do khi từ chối, `If-Match` + `Idempotency-Key` (TC-CMS-001…004) — file: `…/activities/useActivityWorkflow.test.tsx`
- [ ] T029 [P] [US2] Test F058-AC-13: loại `status = preview` khóa Xuất bản, hiện `min_client_version`, `ACTIVITY_TYPE_NOT_SUPPORTED` (TC-ACT-004) — file: `…/activities/PublishGate.test.tsx`
- [ ] T030 [P] [US2] Test F058-AC-14: hộp thoại xuất bản nêu ghim version; bấm lặp một release (TC-ACT-005) — file: `…/activities/PublishDialog.test.tsx`
- [ ] T031 [US2] E2E soạn quiz → preview → duyệt → xuất bản → `hs.an.3a` làm trong game nhận v1 — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/activity-quiz.spec.ts`

### Implementation cho User Story 2

- [ ] T032 [P] [US2] Hook `useActivityPreview`, `useActivityWorkflow` — file: `…/activities/api/preview.ts`, `…/activities/api/workflow.ts`
- [ ] T033 [US2] `ActivityPreviewPanel` (renderer web dùng chung F053), cổng xuất bản theo registry — file: `…/activities/ActivityPreviewPanel.tsx`, `…/activities/PublishGate.tsx`

## Phase 5: User Story 3 — Video Manager (Priority: P2)

### Tests cho User Story 3

- [ ] T034 [P] [US3] Test F058-AC-15: chọn/upload video, poll tới `ready`, `duration_ms` chỉ đọc, khóa Gửi duyệt khi chưa `ready` (TC-CNT-008) — file: `…/activities/VideoLessonEditor.media.test.tsx` — phải FAIL trước khi code
- [ ] T035 [P] [US3] Test F058-AC-16, F058-AC-18: chương không chồng lấn trong thời lượng; cue duy nhất trong `[0, duration_ms]`, câu `published` (TC-VID-001) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/activities/videoRules.test.ts`
- [ ] T036 [P] [US3] Test F058-AC-17: phụ đề đúng pattern, một `is_default`, "có lời" bắt buộc `vi` (TC-VID-004) — file: `src/cms/frontend/packages/ops-ui/src/activity-editor/video/SubtitleList.test.tsx`
- [ ] T037 [P] [US3] Test F058-AC-19: nhánh 2–4 lựa chọn, `target_segment_id` tồn tại, có `is_ending` — file: `…/activity-editor/video/BranchEditor.test.tsx`
- [ ] T038 [P] [US3] Test F058-AC-20: điền sẵn `allow_seek`, `completion_rule` theo `video_completion`, cảnh báo `video_recommended_length` (TC-VID-002) — file: `…/activities/VideoLessonEditor.defaults.test.tsx`
- [ ] T039 [P] [US3] Test F058-AC-21: preview HLS dừng tại cue, phụ đề bật/tắt, danh sách chương (TC-VID-004) — file: `…/activity-editor/video/HlsPreviewPlayer.test.tsx`
- [ ] T040 [US3] E2E tạo video 2 chương, phụ đề `vi`, cue 01:30 → preview → xuất bản — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/activity-video.spec.ts`

### Implementation cho User Story 3

- [ ] T041 [P] [US3] `VideoTimeline`, `ChapterEditor`, `CueEditor`, `BranchEditor`, `SubtitleList`, `HlsPreviewPlayer` + story — file: `src/cms/frontend/packages/ops-ui/src/activity-editor/video/*.tsx`
- [ ] T042 [US3] `videoRules` + `VideoLessonEditor` nối `MediaPicker` (F053) — file: `…/activities/validators/videoRules.ts`, `…/activities/VideoLessonEditor.tsx`

## Phase 6: User Story 4 — Chấm lại hệ thống (Priority: P2)

### Tests cho User Story 4

- [ ] T043 [P] [US4] Test F058-AC-22: nút chỉ với `learning_content.re_evaluate`; `action`, `corrected_version_id` khi `use_new_key`, lý do 10–1.000, số attempt từ `previewRegradeRequest` (TC-LRN-015) — file: `src/cms/frontend/packages/ops-ui/src/activity-editor/regrade/RegradeDialog.test.tsx` — phải FAIL trước khi code
- [ ] T044 [P] [US4] Test F058-AC-23: `createRegradeRequest` `202`, `Idempotency-Key` giữ khi retry, polling `getRegradeRequest`, lịch sử `listRegradeRequests` (TC-LRN-015) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/regrade/useRegrade.test.tsx`
- [ ] T045 [P] [US4] Test F058-AC-24: `school_admin` không thấy nút, `403 PERMISSION_DENIED` theo `code`, DOM không có ô sửa điểm (TC-LRN-015) — file: `…/regrade/RegradeEntryButton.test.tsx`
- [ ] T046 [P] [US4] Test F058-AC-25: thông điệp giữ bài làm gốc và evaluation cũ — file: `…/activity-editor/regrade/RegradeDialog.copy.test.tsx`
- [ ] T047 [US4] E2E `qlcm.truong.a` chấm lại question version lỗi → `completed`; `ad.truong.a` không thấy nút — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/regrade.spec.ts`

### Implementation cho User Story 4

- [ ] T048 [P] [US4] `RegradeDialog` (nhận `scope`, `version_id`) + story — file: `src/cms/frontend/packages/ops-ui/src/activity-editor/regrade/RegradeDialog.tsx`
- [ ] T049 [US4] Hook `useRegrade`; `RegradeEntryButton` gắn vào trang version câu hỏi (F053) và activity; `RegradeStatusPage`, `RegradeHistoryTab` — file: `…/activities/api/regrade.ts`, `src/cms/frontend/apps/portal/src/workspaces/content-studio/regrade/*.tsx`

## Phase 7: User Story 5 — Phân quyền, tenant, idempotency, giới hạn (Priority: P1)

### Tests cho User Story 5

- [ ] T050 [P] [US5] Test F058-AC-26: thiếu `learning_content.activity.write` → chỉ xem; `403` → thông điệp — file: `…/activities/permissions.test.tsx` — phải FAIL trước khi code
- [ ] T051 [P] [US5] Test tenant isolation + authorization F058-AC-27: `source = assigned` chỉ đọc "Từ EduWorld"; System Admin bắt buộc tenant đích/Platform; `ad.truong.b` không thấy activity của `th-demo-a` (TC-CMS-TEN-03, TC-CMS-ASG-02) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/activity-tenancy.spec.ts`
- [ ] T052 [P] [US5] Test F058-AC-28: mọi mutation F058 có `Idempotency-Key` giữ khi retry, lỗi hiển thị `code` + `trace_id` (TC-CNT-003) — file: `…/activities/idempotency.test.ts`
- [ ] T053 [P] [US5] Test F058-AC-29: ô nhập theo `maxLength` + bộ đếm (TC-TEN-024) — file: `…/activities/inputLimits.test.tsx`
- [ ] T054 [P] [US5] Test F058-AC-30: < 1024 px → `LargeScreenRequired`, chỉ xem — file: `…/activities/responsive.test.tsx`

### Implementation cho User Story 5

- [ ] T055 [US5] Guard permission trên route/nút, `TargetTenantField` (F057) trong form tạo, `AssignedBadge` (F053), `LargeScreenRequired` — file: `…/activities/ActivityEditorPage.tsx`, `…/activities/routes.tsx`

## Phase N: Polish & Cross-Cutting

- [ ] T056 [P] Lint hardcode style và không import component/Theme game (TC-UI-003, TC-UI-010) — kiểm: `make web-lint`
- [ ] T057 [P] axe + visual regression builder quiz, Video Manager, hộp thoại chấm lại (TC-UI-004, TC-UI-006) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/activity-a11y-visual.spec.ts`
- [ ] T058 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md`
- [ ] T059 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng
- [ ] T060 [P] Test F058-AC-31: tắt `menu.content_studio.activities` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/activity-feature-toggle.spec.ts`
- [ ] T061 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F058-AC-31) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T005–T009 chặn AC-07, AC-17, AC-22, AC-23) → US1 → US2 → US5 → US3, US4 (song song) → Polish.
- US3 cần F065 (plugin `video_lesson`) và F027 (biến thể HLS); US4 cần F031 job re-evaluate.

## Definition of Done

- [ ] Mọi tiêu chí F058-AC-* có test pass (ghi mã test/TC)
- [ ] `make web-lint`, `make web-test`, Playwright E2E `@F058` xanh với backend thật
- [ ] Constitution Check trong plan.md vẫn đạt; MSW chỉ trong test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

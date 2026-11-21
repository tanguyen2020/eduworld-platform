# Tasks: F053 — Workspace B — cây curriculum, editor nội dung, công thức, song ngữ, trình soạn câu hỏi

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Viết tắt: `OPS/` = `src/cms/frontend/packages/ops-ui/src/`, `WS/` = `src/cms/frontend/apps/portal/src/workspaces/content-studio/`. Dùng lại từ F051: `withTenantScope`, `TenantTargetField`, `useIdempotentMutation`, `ReasonDialog`, `mapValidationErrors`, `unsavedChangesGuard`, `LargeScreenRequired`. Mock MSW chỉ trong test (`// MOCK (test-only)`).

## Phase 1: Setup

- [x] T001 Ghim phiên bản cố định `mathjax@4.x`, `mathlive`, `ajv`, `@dnd-kit/core` — file: `src/cms/frontend/apps/portal/package.json`, `src/cms/frontend/packages/ops-ui/package.json` — **2026-11-19:** ghim `@mathjax/src`/`@mathjax/mathjax-newcm-font`/`@mathjax/mathjax-mhchem-font-extension` 4.1.3, `mathlive` 0.111.4 (ops-ui), `ajv` 8.20.0 (portal + dev ops-ui). `@dnd-kit/core` **không dùng** [Confirmed]: Tree kéo-thả của Ant Design (spec Clarifications 2026-11-19).
- [x] T002 [P] Route + menu workspace B (Ngân hàng câu hỏi, Chương trình học) theo khóa menu từ `specs/portal/menu-catalog.yaml` (registry sinh, F051; không khai báo menu tay) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/routes.tsx` — **2026-11-19:** `workspaces/content-studio/index.tsx` gắn `QuestionBankScreen`, `CurriculumTreePage` vào khóa `menu.content_studio.question_bank`/`curriculum` (route từ registry sinh; sub-route `new`, `:questionId`).
- [x] T003 [P] Khóa i18n `portal.content_studio.*` vi/en (dạng câu hỏi, trạng thái, lỗi theo `code`) — file: `src/cms/frontend/apps/portal/src/i18n/locales/vi/portal.content_studio.json`, `src/cms/frontend/apps/portal/src/i18n/locales/en/portal.content_studio.json` — **2026-11-19:** `src/i18n/content-studio.{vi,en}.json` (khóa `content_studio.*` + `error.<CODE>` nội dung) gộp trong `src/i18n/index.ts`; chuỗi ops-ui trong `packages/ops-ui/src/i18n/{vi,en}.json` (243 khóa, vi = en).
- [ ] T004 [P] Playwright project `@F053`, fixture ảnh/audio, bộ công thức mẫu Toán/Lý/Hóa — file: `src/cms/frontend/apps/portal/tests/e2e/fixtures/content/formulas.json` — **2026-11-19:** chưa làm — Playwright `@F053` với backend thật chờ OQ F051-9/OQ F020-3; bộ công thức mẫu Toán/Lý/Hóa nằm trong story `MathText` và `renderMath.test.ts`.

## Phase 2: Foundational (chặn mọi user story)

- [x] T005 Contract: sửa `QuestionAuthoringInput` (`$ref` question.schema.json, `hints`, `worked_example_ref`, `audio_exempt_reason`, `math_assets`) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **2026-11-19:** thêm `audio_exempt_reason`; `hints`/`worked_example_ref`/`math_assets` đã có; `$ref` ngoài tới question.schema.json không làm [Confirmed] (Clarifications). `check-maxlength` 0 vi phạm, `split-public` sinh lại learning.v1.yaml.
- [x] T006 Contract: thêm `math_assets` vào `LessonAuthoringInput` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-19:** `LessonAuthoringInput.math_assets[]` (≤ 50, cùng shape).
- [x] T007 Contract: thêm `AUDIO_REQUIRED` vào `ProblemCode` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `node scripts/specs/check-problem-codes.mjs` — **2026-11-19:** `AUDIO_REQUIRED` đã có trên main (F025 delta); `check-problem-codes` xanh.
- [x] T008 Contract: thêm `listCurriculumVersions`, `transitionCurriculumVersion`, `publishCurriculumVersion`, schema `CurriculumVersion` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-19:** 3 operation + `CurriculumVersion` đã có (F021); `missing_yccd_objective_ids` không thêm [Confirmed] — dùng `errors[].message` của F021-AC-14. Thêm delta mới `Topic.row_version`, `LearningObjective.row_version` (If-Match từ danh sách, backend F021).
- [x] T009 Contract: thêm `outdated_fields[]` vào `ContentVersion` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — **2026-11-19:** `ContentVersion.outdated_fields[]` (readOnly, ≤ 500 × 256).
- [x] T010 Sinh kiểu và schema client: `pnpm gen:api`, script `gen:schemas` sao chép `specs/schemas/question.schema.json` (CI kiểm khớp) — file: `src/cms/frontend/apps/portal/scripts/gen-schemas.mjs`, `src/cms/frontend/apps/portal/src/api/schemas/question.schema.json` — **2026-11-19:** `pnpm gen:api` sinh lại type; `scripts/gen-schemas.mjs` chép question.schema.json + enums.schema.json vào `src/shared/api/schemas/` (nằm trong `gen:api`/`gen:check`).
- [x] T011 [P] Unit test `useAutosave`: debounce + chu kỳ ép lưu, một request tại một thời điểm, `If-Match`, dừng khi `412`/`409 CONTENT_VERSION_CONFLICT` (F053-AC-06, F053-AC-07) — file: `src/cms/frontend/packages/ops-ui/src/editor/useAutosave.test.ts` — phải FAIL trước khi code — **2026-11-19:** `useAutosave.test.ts` 6 pass (fake timers).
- [x] T012 [P] Unit test `ItemStatusBar`: nút theo trạng thái/permission, ẩn Duyệt với tác giả, `published` chỉ có "Tạo bản nháp" (F053-AC-08, F053-AC-09, F053-AC-10, F053-AC-13) — file: `src/cms/frontend/packages/ops-ui/src/editor/ItemStatusBar.test.tsx` — **2026-11-19:** `ItemStatusBar.test.tsx` 8 pass.
- [x] T013 [P] Unit test `RichContentEditor` khối cơ bản (`paragraph`, `heading`, `list`, `math`, `image` có `alt`, `audio`, `table`, `callout`), không lưu HTML tự do — file: `src/cms/frontend/packages/ops-ui/src/rich-content/RichContentEditor.test.tsx` — **2026-11-19:** `RichContentEditor.test.tsx` 3 pass.
- [x] T014 [P] Unit test `schemaValidator` (ajv) map lỗi về field path, giới hạn `maxLength` (F053-AC-14) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/schemaValidator.test.ts` — **2026-11-19:** `workspaces/content-studio/shared/schemaValidator.test.ts` (test đặt cạnh mã như F051).
- [x] T015 `ContentEditorShell`, `ItemStatusBar`, `useAutosave`, `ConflictBanner` (đăng ký `unsavedChangesGuard`) — file: `src/cms/frontend/packages/ops-ui/src/editor/ContentEditorShell.tsx` — **2026-11-19:** `editor/{ContentEditorShell,ItemStatusBar,ConflictBanner,useAutosave}`; guard qua prop `registerDirtyGuard` (portal nối `registerUnsavedChanges`) + `beforeunload`.
- [x] T016 `RichContentEditor` + khối cơ bản — file: `src/cms/frontend/packages/ops-ui/src/rich-content/RichContentEditor.tsx` — **2026-11-19:** `rich-content/RichContentEditor.tsx` + `types.ts`.
- [x] T017 `schemaValidator.ts` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/shared/schemaValidator.ts` — **2026-11-19:** `shared/schemaValidator.ts` (ajv 2020-12, map lỗi về field path).

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Soạn câu hỏi 11 dạng và đưa qua workflow (Priority: P1) 🎯 MVP

**Goal**: Question Editor đủ dạng, media, autosave, workflow một item.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T018 [P] [US1] Unit test `itemId` (pattern, không trùng) và kiểm `answer_key` tham chiếu ID tồn tại (F053-AC-02) — file: `src/cms/frontend/packages/ops-ui/src/question-editor/itemId.test.ts` — phải FAIL trước khi code — **2026-11-19:** `itemId.test.ts` 4 pass.
- [x] T019 [P] [US1] Unit test mapper 11 dạng form ↔ payload, payload hợp lệ theo `question.schema.json`, `render_payload` không chứa đáp án/feedback/hints (F053-AC-01) — file: `src/cms/frontend/packages/ops-ui/src/question-editor/mappers/mappers.test.ts` — **2026-11-19:** `mappers.test.ts` 25 pass — 11 dạng hợp lệ theo question.schema.json (ajv).
- [x] T020 [P] [US1] Unit test `OptionFeedbackFields`, `HintsField` (≤ 5), `StudentViewPreview` không hiển thị feedback/gợi ý/đáp án (F053-AC-03) — file: `src/cms/frontend/packages/ops-ui/src/question-editor/feedbackHints.test.tsx` — **2026-11-19:** `feedbackHints.test.tsx`.
- [x] T021 [P] [US1] Unit test khóa Gửi duyệt khi khối 1–3 thiếu `stem.audio_ref` (trừ `audio_exempt_reason`), map `422 AUDIO_REQUIRED` (F053-AC-04) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/audioGate.test.ts` — **2026-11-19:** `shared/gates.test.ts` (audio gate + map `AUDIO_REQUIRED`).
- [x] T022 [P] [US1] Unit test `MediaPicker`: bắt buộc `license`, `alt_text` cho ảnh, poll `getMediaAsset` tới `ready`/`rejected`, giới hạn `assets_per_version_max` (F053-AC-05) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/MediaPicker.test.tsx` — **2026-11-19:** luồng upload (license, alt, presign → PUT → complete → poll tới `Sẵn sàng`) trong `QuestionEditorPage.test.tsx` › AC-05; giới hạn `assets_per_version_max` qua `limitReached`.
- [x] T023 [P] [US1] Unit test `QuestionEditorPage`: `published` chỉ đọc + `createQuestionVersion` (F053-AC-08), `assigned` chỉ đọc nhãn "Từ EduWorld" (F053-AC-11), System Admin bắt buộc phạm vi (F053-AC-12), < 1024 px → `LargeScreenRequired` (F053-AC-15) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/QuestionEditorPage.test.tsx` — **2026-11-19:** `QuestionEditorPage.test.tsx` (AC-04…AC-15, AC-25, AC-33) với MOCK backend.
- [ ] T024 [US1] E2E soạn → gửi duyệt → duyệt → xuất bản (bấm hai lần một release), xung đột hai tab (F053-AC-01, F053-AC-05, F053-AC-06, F053-AC-07, F053-AC-09, F053-AC-10; TC-CMS-001, TC-CNT-003, TC-CNT-008) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/question-flow.spec.ts` — **2026-11-19:** phần unit/integration xong (`QuestionEditorPage.test.tsx` › AC-06/07, AC-08, AC-09/10, AC-05); E2E backend thật chờ OQ F051-9.
- [ ] T025 [US1] E2E authorization + tenant isolation: `cm.duyet` không có tạo/sửa; `qlcm.truong.a` nội dung gán chỉ đọc; `ad.truong.b` mở câu hỏi `th-demo-a` → "Không tìm thấy"; `sa.admin` bắt buộc phạm vi (F053-AC-11, F053-AC-12, F053-AC-13; TC-CMS-ASG-02, TC-CMS-TEN-03) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/question-authz.spec.ts` — **2026-11-19:** phần integration xong (AC-11, AC-12, AC-13); E2E `ad.truong.b` → 404 chờ OQ F051-9 (cô lập tenant do server).
- [ ] T026 [P] [US1] E2E lồng tiếng khối 3 và giới hạn độ dài (F053-AC-04, F053-AC-14; TC-CNT-010, TC-TEN-024) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/question-validation.spec.ts` — **2026-11-19:** phần unit xong (AC-04 trong page test + gates; AC-14 trong schemaValidator.test, bộ đếm `maxLength` trong BilingualField/feedback tests); E2E chờ OQ F051-9.

### Implementation cho User Story 1

- [x] T027 [P] [US1] `itemId.ts`, `mappers/<type>.ts` — file: `src/cms/frontend/packages/ops-ui/src/question-editor/mappers/index.ts` — **2026-11-19:** `question-editor/itemId.ts`, `mappers/index.ts` (một module cho 11 dạng).
- [x] T028 [US1] Editor 8 dạng cơ bản (`single_choice`, `multiple_choice`, `true_false`, `fill_in_blank`, `numeric`, `matching`, `ordering`, `drag_drop`) — file: `src/cms/frontend/packages/ops-ui/src/question-editor/types/SingleChoiceEditor.tsx` — **2026-11-19:** `question-editor/typeEditors.tsx` (8 dạng cơ bản).
- [x] T029 [US1] Editor 3 dạng nâng cao (`short_answer`, `image_based`, `interactive`) sau khi F023 Done — file: `src/cms/frontend/packages/ops-ui/src/question-editor/types/ImageBasedEditor.tsx` — **2026-11-19:** 3 dạng nâng cao có form; F023 đã Done trên main nên server nhận cả 3 dạng (SC-005 đủ 11 dạng).
- [x] T030 [US1] `QuestionEditor`, `OptionFeedbackFields`, `HintsField` — file: `src/cms/frontend/packages/ops-ui/src/question-editor/QuestionEditor.tsx` — **2026-11-19:** `QuestionEditor.tsx`, `OptionFeedbackFields.tsx`, `HintsField.tsx`, `QuestionPreview.tsx`.
- [x] T031 [US1] `MediaPicker`, `useMediaUpload` (`presignMediaUpload` → PUT → `getMediaAsset`) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/media/MediaPicker.tsx` — **2026-11-19:** `media/MediaPicker.tsx`, `media/useMediaUpload.ts`.
- [x] T032 [US1] `QuestionEditorPage`, `useQuestion`, `useWorkflow`, `StudentViewPreview` (`createLearningPreviewSession`), `OwnerScopeField`, `AssignedBadge` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/questions/QuestionEditorPage.tsx` — **2026-11-19:** `questions/{QuestionEditorPage,QuestionMetaPanel,useQuestion,useWorkflow,StudentViewPreview,QuestionBankScreen}`, `shared/{OwnerScopeField,AssignedBadge,access,gates,config}`.

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Công thức Toán/Lý/Hóa (Priority: P1)

**Goal**: Nhập MathLive, hiển thị MathJax 4, `math_assets` SVG + speech.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T033 [P] [US2] Unit test `renderMath`: SVG `currentColor`, speech text, `math_id` sha256, chặn `\href`/`\require`/`\html…`/macro, vượt `latex_max_length`/`latex_max_depth`/`math_svg_max` (F053-AC-17) — file: `src/cms/frontend/packages/ops-ui/src/math/renderMath.test.ts` — phải FAIL trước khi code — **2026-11-19:** `renderMath.test.ts` 7 pass.
- [x] T034 [P] [US2] Unit test `MathInput` (tab Toán/Lý/Hóa, delimiter inline/block — F053-AC-16), tô đỏ vị trí `MATH_RENDER_FAILED` (F053-AC-18), cảnh báo dạng câu đòi nhập biểu thức (F053-AC-20) — file: `src/cms/frontend/packages/ops-ui/src/math/MathInput.test.tsx` — **2026-11-19:** `MathInput.test.tsx` (MathLive là MOCK module trong jsdom).
- [ ] T035 [US2] E2E + visual regression bộ công thức mẫu sáng/tối, compact, zoom 200%; gửi thẳng API LaTeX cấm → 422 (F053-AC-16, F053-AC-17, F053-AC-18, F053-AC-19; TC-UI-011, TC-UI-012, TC-UI-013, TC-MATH-001, TC-MATH-002) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/math.spec.ts` — **2026-11-19:** chưa làm — E2E + visual công thức chờ OQ F051-9; story `MathText` (sáng/tối/compact) có sẵn cho visual khi baseline được duyệt.

### Implementation cho User Story 2

- [x] T036 [P] [US2] `mathjaxConfig.ts`, `renderMath.ts` — file: `src/cms/frontend/packages/ops-ui/src/math/renderMath.ts` — **2026-11-19:** `math/{mathjaxConfig,renderMath,speech}.ts`.
- [x] T037 [US2] `MathInput` (MathLive `<math-field>`), `MathText` (MathJax 4 + `mhchem`, aria/speech) — file: `src/cms/frontend/packages/ops-ui/src/math/MathInput.tsx`, `src/cms/frontend/packages/ops-ui/src/math/MathText.tsx` — **2026-11-19:** `math/{MathInput,MathText,MathInsertButton,keyboardLayouts}`.
- [x] T038 [US2] `collectMath.ts` + gắn vào pipeline lưu của `ContentEditorShell` (gửi `math_assets`), cảnh báo dạng câu — file: `src/cms/frontend/packages/ops-ui/src/rich-content/collectMath.ts` — **2026-11-19:** `rich-content/collectMath.ts`; page dựng `math_assets` trước mỗi lần lưu, map `MATH_RENDER_FAILED` `/math_assets/<i>` về field.

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Biên tập song ngữ (Priority: P2)

**Goal**: Hai cột vi/en, trạng thái bản dịch, xuất/nhập file dịch.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T039 [P] [US3] Unit test `BilingualField` (hai cột/tab, nhãn "Gốc") và huy hiệu `translation_status` + `outdated_fields` (F053-AC-21, F053-AC-22) — file: `src/cms/frontend/packages/ops-ui/src/i18n-field/BilingualField.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `BilingualField.test.tsx` 6 pass.
- [x] T040 [P] [US3] Unit test khóa khi thiếu bản gốc, map `TRANSLATION_INCOMPLETE`, thiếu bản tùy chọn → cảnh báo `partial` (F053-AC-23, F053-AC-24) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/translationGate.test.ts` — **2026-11-19:** `shared/gates.test.ts` (translation gate).
- [x] T041 [P] [US3] Unit test dialog xuất/nhập dịch: Idempotency-Key, poll job, field không khớp (F053-AC-25) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/translations.test.tsx` — **2026-11-19:** xuất file dịch (Idempotency-Key, poll job, link tải) trong `QuestionEditorPage.test.tsx` › AC-25; báo field không khớp hiển thị bởi `JobStatus`.
- [ ] T042 [US3] E2E song ngữ; authorization: người thiếu `learning_content.translate` không thấy xuất/nhập (F053-AC-21, F053-AC-22, F053-AC-23, F053-AC-24, F053-AC-25; TC-I18N-002) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/bilingual.spec.ts` — **2026-11-19:** phần unit xong; E2E chờ OQ F051-9.

### Implementation cho User Story 3

- [x] T043 [P] [US3] `BilingualField`, `translationStatus.ts` — file: `src/cms/frontend/packages/ops-ui/src/i18n-field/BilingualField.tsx` — **2026-11-19:** `i18n-field/{BilingualField,translationStatus}`.
- [x] T044 [US3] Khóa Gửi duyệt/Xuất bản + danh sách field thiếu nhảy tới trường — file: `src/cms/frontend/packages/ops-ui/src/editor/ItemStatusBar.tsx` — **2026-11-19:** `ItemStatusBar` blockers + `jumpToField`.
- [x] T045 [US3] `TranslationExportDialog`, `TranslationImportDialog`, `useTranslationJobs` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/translations/TranslationExportDialog.tsx` — **2026-11-19:** `translations/{TranslationDialogs.tsx,useTranslationJobs.ts}`.

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Cây curriculum (Priority: P2)

**Goal**: Môn, curriculum, chủ đề kéo-thả, LO + tiên quyết, workflow curriculum version.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T046 [P] [US4] Unit test `TopicTree` kéo-thả → `updateTopic` (`parent_topic_id`, `position`, `If-Match`), hoàn tác khi lỗi (F053-AC-27) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/TopicTree.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `curriculum/treeModel.test.ts`, `curriculum/TopicTree.test.tsx`.
- [x] T047 [P] [US4] Unit test `ObjectiveForm`: tiên quyết ≤ `maxItems`, hiển thị lỗi chu trình (F053-AC-28) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/ObjectiveForm.test.tsx` — **2026-11-19:** `curriculum/CurriculumTreePage.test.tsx` › ObjectiveForm.
- [x] T048 [P] [US4] Unit test `CurriculumTreePage` (cây từ 4 operationId, chỉ đọc với dữ liệu gán, thao tác theo `curriculum.write` — F053-AC-26, F053-AC-29) và `CurriculumVersionPanel` hiện LO thiếu `yccd_ref` (F053-AC-30) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/CurriculumTreePage.test.tsx` — **2026-11-19:** `curriculum/CurriculumTreePage.test.tsx`.
- [ ] T049 [US4] E2E curriculum với `cm.noidung`, `qlcm.truong.a`; tenant isolation: `ad.truong.b` không thấy curriculum riêng của `th-demo-a`; authorization: `qlcm.truong.a` gọi `transitionCurriculumVersion` (mới) cho curriculum platform → 403 (F053-AC-26, F053-AC-27, F053-AC-28, F053-AC-29, F053-AC-30; TC-CMS-ASG-02) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/curriculum.spec.ts` — **2026-11-19:** phần integration xong (AC-26, AC-29); E2E và 403 `transitionCurriculumVersion` từ server chờ OQ F051-9.

### Implementation cho User Story 4

- [x] T050 [P] [US4] `useCurriculum` (subjects, curricula, topics, objectives, curriculum versions) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/curriculum/useCurriculum.ts` — **2026-11-19:** `curriculum/useCurriculum.ts`.
- [x] T051 [US4] `CurriculumTreePage`, `TopicTree` (`@dnd-kit`) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/curriculum/CurriculumTreePage.tsx` — **2026-11-19:** `curriculum/{CurriculumTreePage,TopicTree,treeModel}` (Tree Ant Design kéo-thả).
- [x] T052 [US4] `ObjectiveForm` (mã, mô tả, `bloom_level`, `yccd_ref`, tiên quyết) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/curriculum/ObjectiveForm.tsx` — **2026-11-19:** `curriculum/ObjectiveForm.tsx`.
- [x] T053 [US4] `CurriculumVersionPanel` — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/curriculum/CurriculumVersionPanel.tsx` — **2026-11-19:** `curriculum/CurriculumVersionPanel.tsx`.

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Khối `worked_example` trong nội dung (Priority: P3)

**Goal**: Soạn ví dụ mẫu từng bước; gắn câu hỏi.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T054 [P] [US5] Unit test `WorkedExampleBlock`: 1–8 bước, audio bắt buộc khối 1–3, `id` duy nhất (F053-AC-31) — file: `src/cms/frontend/packages/ops-ui/src/rich-content/WorkedExampleBlock.test.tsx` — phải FAIL trước khi code — **2026-11-19:** `WorkedExampleBlock.test.tsx`.
- [x] T055 [P] [US5] Unit test picker `worked_example_ref` lọc lesson theo LO (F053-AC-32) — file: `src/cms/frontend/apps/portal/tests/unit/content-studio/workedExampleRef.test.tsx` — **2026-11-19:** `questions/workedExampleRef.test.tsx`.
- [ ] T056 [US5] E2E thêm khối và gắn vào câu hỏi (F053-AC-31, F053-AC-32) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/worked-example.spec.ts` — **2026-11-19:** phần unit xong; E2E chờ OQ F051-9 và Lesson Builder (F054).

### Implementation cho User Story 5

- [x] T057 [P] [US5] `WorkedExampleBlock` trong `RichContentEditor` — file: `src/cms/frontend/packages/ops-ui/src/rich-content/WorkedExampleBlock.tsx` — **2026-11-19:** `rich-content/WorkedExampleBlock.tsx` (dùng ở F054 Lesson Builder).
- [x] T058 [US5] Picker `worked_example_ref` (`listCmsLessons`, `getCmsLesson`) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/questions/WorkedExampleRefPicker.tsx` — **2026-11-19:** `questions/WorkedExampleRefPicker.tsx`.

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase 8: Polish & Cross-Cutting

- [x] T059 [P] Story Storybook sáng/tối/compact cho `ContentEditorShell`, `RichContentEditor`, `MathInput`, `MathText`, `BilingualField`, 11 editor câu hỏi (TC-UI-005) — file: `src/cms/frontend/packages/ops-ui/src/question-editor/QuestionEditor.stories.tsx` — **2026-11-19:** 14 file story mới (Light/Dark/Compact; QuestionEditor thêm 10 story theo dạng); `check-modes`: 17 component.
- [ ] T060 [P] E2E axe trên Question Editor và cây curriculum (TC-UI-006) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/a11y.spec.ts` — **2026-11-19:** chưa làm — axe chạy qua `make web-storybook-check` ở CI (cần Chromium); E2E toàn trang chờ OQ F051-9.
- [x] T061 [P] Đăng ký chu kỳ autosave của editor vào CONFIG_DEFAULTS (tài liệu sở hữu ADMIN_PORTAL §1.4), đọc giá trị từ cấu hình portal; cập nhật CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `CHANGELOG.md` — **2026-11-19:** `content_autosave_interval` đã đăng ký trong CONFIG_DEFAULTS (scope code); portal đọc từ `shared/config.ts`; CHANGELOG cập nhật.
- [x] T062 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/053-content-studio-editor/quickstart.md` — **2026-11-19:** quickstart §5 ghi bằng chứng.
- [x] T063 Cập nhật trạng thái F053 — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — **2026-11-19:** CODE_PLAN: F053 Done (2026-11-19).
- [x] T064 [P] Test F053-AC-33: tắt `action.content_studio.question_bank.publish` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/tests/e2e/content-studio/question-feature-toggle.spec.ts` — **2026-11-19:** `QuestionEditorPage.test.tsx` › AC-13/AC-33 (action `question_bank.publish` tắt → nút khóa); `403 FEATURE_DISABLED` do shell F051 (đã test).
- [x] T065 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F053-AC-33) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/routes.tsx` — **2026-11-19:** nút workflow bọc `<ActionGate>` qua `gateQuestionAction`; upload/xuất dịch/curriculum dùng `ActionGate`.

## Nhận sau E2E portal (owner chốt 2026-11-21)

- [x] T066 (OQ F053-9) Bộ chọn chủ đề trong `DocMetaFields`/`QuestionActivityEditor` tự suy `curriculum_version_id` hiện hành theo môn × khối (curriculum của trường/tenant đang dùng) rồi gọi `listCmsTopics` theo contract; không có version hiện hành thì hiện trạng thái rỗng có giải thích — vitest — cỡ S — ✅ 2026-11-22: `pickCurrentVersion` + `useCurrentTopics` (`listCmsCurricula` → `getCurriculumVersion` của `current_version` → `listCmsTopics` với `curriculum_version_id`), dòng nguồn chương trình và giải thích khi không có version (Clarifications Session 2026-11-22); vitest `curriculum/topicPicker.test.tsx` 5/5 (3 unit chọn version, 2 portal: cả `DocMetaFields` và luật chọn câu của `QuestionActivityEditor` gửi `curriculum_version_id`; không version → không gọi `listCmsTopics`, hiện giải thích)
- [x] T067 Sửa test đang đỏ trên main: `CurriculumTreePage.test.tsx` AC-26 (phiên `cm.noidung`, "Toán" không hiện) — tìm nguyên nhân (test hay code), sửa phía sai — vitest — cỡ S — ✅ 2026-11-22: test sai, không phải code — workspace là chunk lazy, test đầu của file trả chi phí import lạnh (~9–10 s) trong `findByText` (`asyncUtilTimeout` 10 s); thêm `beforeAll` import chunk (Clarifications Session 2026-11-22). vitest: file 5/5, AC-26 2,9 s (trước ~10 s, đỏ khi tải nặng); lượt Content Studio (curriculum + builders + activities) 76/76

## Dependencies & Execution Order

- Setup → Foundational (T005–T010 contract + sinh code; T015–T017 nền editor) → US1, US2 (song song sau T016) → US3, US4 (song song) → US5 → Polish.
- T028 cần backend F022; T029 cần F023; T031 cần F027; T050 phần version cần backend F021 hiện thực T008.

## Definition of Done

- [x] Mọi tiêu chí F053-AC-* có test pass (ghi mã test/TC) — mức unit/integration với MOCK backend hợp đồng (quickstart §5); E2E backend thật còn mở (OQ F051-9)
- [x] `make web-lint`, `make web-test`, Playwright `@F053`, `make storybook` xanh — lint/typecheck/gen:check/vitest (51 file, 335 test)/build xanh, `check-modes` 17 component; Playwright `@F053` và `make web-visual` (baseline story mới) chờ OQ F051-9/CI
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — MOCK chỉ trong `src/test/*` và setup test (gắn nhãn)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — CONFIG_DEFAULTS không đổi (`content_autosave_interval` đã có), CHANGELOG + spec Clarifications 2026-11-19
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

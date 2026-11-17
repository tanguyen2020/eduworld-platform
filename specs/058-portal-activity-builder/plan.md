# Implementation Plan: F058 — Workspace B — Learning Activity Builder (quiz, interactive_exercise, video_lesson), UI re-evaluate

**Branch**: `058-portal-activity-builder` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm vào workspace B (chunk `content-studio`) trang Learning Activity Builder: bước 1 chọn `activity_type` từ `listActivityTypes`, bước 2 form sinh từ JSON Schema của loại (ajv 2020 + bộ widget riêng cho `questionSource`, `evaluation`, `feedback`, `completionRule`), metadata, preview qua `createLearningPreviewSession`, và thao tác workflow trên một item (`transitionContentVersion`, `publishContentVersion` với `content_type = activity`). Editor `video_lesson` (Video Manager) có timeline chương/cue/nhánh và trình phát HLS (hls.js). Hộp thoại "Chấm lại hệ thống" dùng chung (`RegradeDialog`) gắn vào trang version câu hỏi và activity, gọi `createRegradeRequest`. Tái dùng `ContentEditorShell`, `MathInput`, `BilingualField`, `MediaPicker` của F053; `QuestionPicker` mới ở `ops-ui`. Năm delta contract (owner F030, F031, F027) vào Phase 2.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript strict (React 18, Vite) |
| Primary Dependencies | Ant Design 5 + Pro (ProForm, StepsForm, ProTable, Slider), TanStack Query, openapi-typescript, ajv (draft 2020-12) + ajv-formats, React Hook Form, hls.js, MathLive/MathJax 4 (qua F053), i18next |
| Storage | Không (state client, TanStack Query cache) |
| Testing | Vitest + Testing Library + MSW (test-only, nhãn `MOCK`), Playwright E2E với backend compose |
| Target Platform | Trình duyệt desktop ≥ 1024 px; màn hẹp chỉ xem |
| Project Type | web app (workspace B) + component `web/packages/ops-ui` |
| Performance Goals | Form sinh từ schema ≤ 300 ms với schema ≤ 600 dòng; autosave không chặn nhập |
| Constraints | AGENTS R1, R3, R5, R6, R10, R28, R32; ADR-0015, ADR-0016, ADR-0043, ADR-0044, ADR-0047, ADR-0048, ADR-0055, ADR-0056, ADR-0072 |
| Scale/Scope | ≤ 100 câu/quiz, ≤ 50 chương, ≤ 30 cue, ≤ 20 nhánh/segment (theo `maxItems` schema) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `createCmsActivityVersion`, `previewRegradeRequest`, `getRegradeRequest`, `listRegradeRequests`, `MediaKind.subtitle` là task Phase 2; không chạm nội dung deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ gọi API public module learning qua APISIX; không logic nghiệp vụ trong component |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` theo phiên; System Admin chọn tenant đích; dữ liệu được gán chỉ đọc; không có nút xóa vĩnh viễn |
| IV Server-authoritative | ✅ | Validate client chỉ để báo sớm; server validate `ValidateConfig`; preview không đáp án; chấm lại do server; Idempotency-Key mọi thao tác ghi |
| V Xác thực, phân quyền | ✅ | Ẩn/hiện theo `getMyPermissions` (`learning_content.activity.write/.review`, `learning_content.publish`, `learning_content.re_evaluate`, `media.upload`); backend vẫn chặn |
| VI An toàn trẻ em | ✅ | Màn chấm lại chỉ hiển thị số lượng attempt, không danh sách tên học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn từ schema sinh tự động; tham số theo tên CONFIG_DEFAULTS; FR từ PRD |
| VIII UI tách logic, token | ✅ | Hook `api/*` gọi API; theme `admin-tokens`; không component game |
| IX Test có bằng chứng | ✅ | Mỗi F058-AC có test viết trước, MSW chỉ trong test |

## Project Structure

### Documentation (feature này)

```text
specs/058-portal-activity-builder/
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
web/packages/ops-ui/src/activity-editor/
├── SchemaForm.tsx                # form từ JSON Schema (ajv), map lỗi JSON Pointer → field
├── QuestionPicker.tsx            # searchQuestions status=published, lọc theo allowed_question_types
├── widgets/{QuestionSourceField,EvaluationField,FeedbackTimingField,CompletionRuleField,TimedQuizField}.tsx
├── video/{VideoTimeline,ChapterEditor,CueEditor,BranchEditor,SubtitleList,HlsPreviewPlayer}.tsx
├── regrade/RegradeDialog.tsx     # dùng chung, nhận scope + version
└── *.stories.tsx
web/apps/portal/src/workspaces/content-studio/activities/
├── routes.tsx                    # /portal/content-studio/activities{,/new,/:activityId}
├── ActivityListPage.tsx          # listCmsActivities lọc loại/môn/khối/trạng thái
├── ActivityTypeStep.tsx          # bước 1 chọn loại (editorRegistry ∩ listActivityTypes)
├── ActivityEditorPage.tsx        # ContentEditorShell + metadata + SchemaForm
├── VideoLessonEditor.tsx         # Video Manager
├── ActivityPreviewPanel.tsx      # createLearningPreviewSession
├── editorRegistry.ts             # activity_type → editor (quiz, interactive_exercise, video_lesson)
├── validators/{videoRules,questionSourceRules}.ts   # kiểm tra sớm phía client (mốc, chồng lấn, loại câu)
├── api/{activities,activityTypes,preview,regrade}.ts
└── i18n/{vi,en}.json
web/apps/portal/src/workspaces/content-studio/regrade/{RegradeEntryButton,RegradeStatusPage,RegradeHistoryTab}.tsx
web/apps/portal/src/api/schemas/activities/*.schema.json   # sinh từ specs/schemas (pnpm gen:schemas), không sửa tay
web/apps/portal/tests/{unit/content-studio/activities,unit/content-studio/regrade,e2e/content-studio}/
```

**Structure Decision**: Component tái sử dụng (form từ schema, timeline video, `RegradeDialog`) ở `ops-ui` để F066 (audio/simulation) và F055 (chấm lại đề) dùng lại; page nối API trong workspace `content-studio` cùng chunk với F053/F054 (một workspace B).

## Phase 0: Research

Không có. Cấu trúc activity ở [ACTIVITY_ENGINE §4, §7, §8.1, §12](../../docs/02-learning/ACTIVITY_ENGINE.md), video ở [VIDEO_LEARNING §3–§7](../../docs/02-learning/VIDEO_LEARNING.md), màn ở [ADMIN_PORTAL §1.4, §3.13, §5](../../docs/04-admin-portal/ADMIN_PORTAL.md). Chọn hls.js cho preview web vì CMS phát HLS theo VIDEO_LEARNING §2.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — | Không có (frontend) |
| application | Hook `useActivity`, `useActivityTypes`, `useActivityWorkflow`, `useActivityPreview`, `useRegrade` | Mutation sinh `Idempotency-Key` một lần/thao tác; polling `getRegradeRequest`, `getMediaAsset` dừng ở trạng thái kết thúc |
| infrastructure | `apiClient` (F050), base URL từ env `VITE_API_PUBLIC_BASE_URL` (dữ liệu tenant) và `VITE_API_INTERNAL_BASE_URL` (dữ liệu platform); schema JSON sinh từ `specs/schemas/activities` | Không hardcode URL (R3) |
| presentation | Page/dialog gọi `listActivityTypes`, `listCmsActivities`, `createCmsActivity`, `getCmsActivity`, `updateCmsActivityDraft`, `searchQuestions`, `listCmsQuestionSets`, `listMediaAssets`, `presignMediaUpload`, `getMediaAsset`, `createLearningPreviewSession`, `transitionContentVersion`, `publishContentVersion`, `listContentVersions`, `createRegradeRequest`; sau delta: `createCmsActivityVersion`, `previewRegradeRequest`, `getRegradeRequest`, `listRegradeRequests` | |

### Ánh xạ form → `ActivityDefinitionInput`

| Phần form | Trường |
|---|---|
| Metadata | `activity_type`, `title`, `description`, `instructions`, `grade_levels`, `subject_id`, `topic_id`, `objective_ids`, `difficulty`, `time_limit_seconds`, `content_language`, `tenant_id` (System Admin) |
| Cấu hình loại | `config.content` (`question_source`, `allowed_question_types`, `timed`, `worked_example_ref` / video `video_asset_id`, `chapters`, `subtitles`, `embedded_questions`, `segments`, `branches`), `config.interaction`, `config.evaluation`, `config.feedback`, `config.completion_rule` |
| Công thức | `math_assets[]` (F053 `collectMath`) |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F030 (CMS activity API, registry), F031 (`createRegradeRequest`), F053 (`ContentEditorShell`, `MathInput`, `BilingualField`, `MediaPicker`), F065 (plugin `video_lesson`, schema), F027 (media), F054 (lịch sử/rollback dùng chung, không chặn).
- Dùng bởi: F066 (form từ schema, timeline media), F055 (gắn `RegradeDialog` scope `exam_session`, F055-AC-23, RB-222).

### Bảo mật, tenant, idempotency, audit

- Permission: `learning_content.read`, `learning_content.activity.write`, `learning_content.activity.review`, `learning_content.publish`, `learning_content.re_evaluate`, `media.upload`.
- Tenant: `X-Tenant-Id` theo phiên (F051); System Admin dùng `TargetTenantField` (F057) khi tạo; danh sách `source = own/assigned/platform`.
- Idempotency-Key: `createCmsActivity`, `createCmsActivityVersion`, `transitionContentVersion`, `publishContentVersion`, `createRegradeRequest`, `createLearningPreviewSession`.
- Audit do backend ghi: `content.version_created`, `content.submitted`, `content.approved`, `content.rejected`, `content.published`, `content.re_evaluated`.

### Quan sát

- Log client: `code`, `trace_id`, route, `activity_type`; không ghi nội dung đề, đáp án, lý do nhập tay (R14).


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.content_studio.activities` |
| Action dùng | `action.content_studio.activities.view`, `action.content_studio.activities.edit`, `action.content_studio.activities.workflow`, `action.content_studio.activities.publish`, `action.content_studio.activities.rollback`, `action.content_studio.regrade.request`, `action.content_studio.media.view`, `action.content_studio.media.upload`, `action.content_studio.reviews.preview`, `action.content_studio.question_bank.view`, `action.content_studio.question_sets.view` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `createCmsActivityVersion`, `listRegradeRequests`, `getRegradeRequest`, `previewRegradeRequest` (Cần thêm) → đề xuất `action.content_studio.activities.edit`, `action.content_studio.regrade.request`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

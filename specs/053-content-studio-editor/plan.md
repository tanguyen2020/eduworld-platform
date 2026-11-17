# Implementation Plan: F053 — Workspace B — cây curriculum, editor nội dung, công thức, song ngữ, trình soạn câu hỏi

**Branch**: `053-content-studio-editor` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Xây component soạn thảo dùng chung trong `src/cms/frontend/packages/ops-ui` (presentational, không gọi API): `ContentEditorShell` + `ItemStatusBar` + `useAutosave`, `RichContentEditor` (khối `paragraph`, `heading`, `list`, `math`, `image`, `audio`, `table`, `callout`, `worked_example`), `MathInput` (MathLive) / `MathText` (MathJax 4) / `renderMath` (LaTeX → SVG `currentColor` + speech text trên trình duyệt), `BilingualField`, `QuestionEditor` với 11 editor theo `QuestionType`. Trong `src/cms/frontend/apps/portal/src/workspaces/content-studio/` nối API: Question Editor page, Curriculum tree, media picker, xuất/nhập file dịch. Validation client dùng chính `specs/schemas/question.schema.json` (ajv) + `maxLength` OpenAPI; server vẫn là nơi quyết định (validator F022/F023, sanitize SVG F027, reviewer ≠ author).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript (strict) · React 18 · Vite |
| Primary Dependencies | `mathjax@4.x` (tex → svg, `mhchem`, speech), `mathlive` (ghim phiên bản cố định), Ant Design 5 + Pro (ProForm, Tree, Tabs), `@dnd-kit` (kéo-thả khối/chủ đề) [Proposed], `ajv` 2020-12, TanStack Query, React Hook Form + Zod |
| Storage | Không có (client); dữ liệu ở `eduworld_learning` (F020–F027) |
| Testing | Vitest + Testing Library + MSW (`// MOCK (test-only)`); Playwright E2E + screenshot (bộ công thức mẫu); Storybook 8 |
| Target Platform | Trình duyệt desktop/tablet ≥ 1024 px cho editor; mọi breakpoint cho xem |
| Project Type | Web app (workspace chunk) + thư viện component |
| Performance Goals | Render 50 công thức < 1 s trên máy văn phòng [Proposed]; autosave không chặn gõ |
| Constraints | AGENTS R1, R2, R6, R11, R28, R31, R32 · ADR-0015, ADR-0043, ADR-0044, ADR-0047, ADR-0055, ADR-0056, ADR-0072 |
| Scale/Scope | 11 dạng câu hỏi, ~8 component ops-ui, 4 màn portal |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta contract (authoring inputs, `AUDIO_REQUIRED`, curriculum version, `outdated_fields`) ở Phase 2 (T005–T009) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ gọi API public module learning; ops-ui không gọi API |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Header tenant phiên; nội dung gán chỉ đọc; E2E cô lập (T025, T049) |
| IV Server-authoritative | ✅ | Đáp án chỉ hiển thị trong CMS cho người có quyền; preview học sinh qua `createLearningPreviewSession` (không đáp án); xuất bản Idempotency-Key |
| V Xác thực, phân quyền | ✅ | Ẩn/hiện theo `learning_content.*`, `curriculum.*`; xử lý 403 |
| VI An toàn trẻ em | ✅ | Không dữ liệu học sinh; không log nội dung đề/đáp án ra console |
| VII Một nguồn sự thật, cấu hình | ✅ | Validation từ `question.schema.json` + OpenAPI; tham số theo tên CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | ops-ui presentational; công thức `currentColor`; theme token |
| IX Test có bằng chứng | ✅ | Mỗi F053-AC-* có test trước code |

## Project Structure

### Documentation (feature này)

```text
specs/053-content-studio-editor/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/cms/frontend/packages/ops-ui/src/
├── editor/{ContentEditorShell.tsx,ItemStatusBar.tsx,useAutosave.ts,ConflictBanner.tsx}
├── rich-content/{RichContentEditor.tsx,blocks/*.tsx,WorkedExampleBlock.tsx,collectMath.ts}
├── math/{MathInput.tsx,MathText.tsx,renderMath.ts,mathjaxConfig.ts}
├── i18n-field/{BilingualField.tsx,translationStatus.ts}
└── question-editor/{QuestionEditor.tsx,itemId.ts,types/{SingleChoice,MultipleChoice,TrueFalse,FillInBlank,ShortAnswer,Numeric,Matching,Ordering,DragDrop,ImageBased,Interactive}Editor.tsx,mappers/*.ts,OptionFeedbackFields.tsx,HintsField.tsx}
src/cms/frontend/apps/portal/src/workspaces/content-studio/
├── index.tsx, routes.tsx   # route theo khóa `menu.content_studio.*` của catalog (ADR-0090)
├── questions/{QuestionEditorPage.tsx,useQuestion.ts,useWorkflow.ts,StudentViewPreview.tsx}
├── media/{MediaPicker.tsx,useMediaUpload.ts}
├── curriculum/{CurriculumTreePage.tsx,TopicTree.tsx,ObjectiveForm.tsx,useCurriculum.ts,CurriculumVersionPanel.tsx}
├── translations/{TranslationExportDialog.tsx,TranslationImportDialog.tsx,useTranslationJobs.ts}
└── shared/{OwnerScopeField.tsx,AssignedBadge.tsx,schemaValidator.ts}
src/cms/frontend/apps/portal/src/api/schemas/question.schema.json   # sinh từ specs/schemas (pnpm gen:schemas), không sửa tay
src/cms/frontend/apps/portal/tests/{unit/content-studio,e2e/content-studio}/
```

**Structure Decision**: Component soạn thảo tái sử dụng (F054, F079 dùng `ContentEditorShell`) đặt ở `ops-ui` theo [ADMIN_PORTAL §1.4](../../docs/04-admin-portal/ADMIN_PORTAL.md); page nối API ở workspace `content-studio`.

## Phase 0: Research

Xem [research.md](research.md): R1 render công thức trên trình duyệt; R2 validation client bằng `question.schema.json`; R3 autosave + `If-Match`; R4 sinh `itemId`.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | `itemId.ts`, `mappers/<type>.ts` (form ↔ `stem`/`render_payload`/`answer_key`/`scoring_rule`/`hints`), `collectMath.ts`, `translationStatus.ts` | Hàm thuần, test unit; không quyết định điểm |
| application (client) | `useQuestion` (`getCmsQuestion`, `createQuestion`, `updateQuestionDraft`, `createQuestionVersion`), `useWorkflow` (`transitionContentVersion`, `publishContentVersion`), `useCurriculum`, `useMediaUpload`, `useTranslationJobs` | Mutation ghi qua `useIdempotentMutation` (F051) |
| infrastructure | `apiClient` (F050), `renderMath` (MathJax 4 trong trình duyệt), ajv | |
| presentation | `QuestionEditorPage`, `CurriculumTreePage`, dialog dịch, `MediaPicker` | `LargeScreenRequired` < 1024 px |

**Permission (ẩn/hiện; server vẫn kiểm):** `learning_content.read` (xem), `learning_content.write` (tạo/sửa/tạo bản nháp), `learning_content.review` (Duyệt/Từ chối), `learning_content.publish` (Xuất bản ngay), `learning_content.translate` (xuất/nhập dịch), `media.upload` (upload), `curriculum.read`/`curriculum.write`/`curriculum.publish`.

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F051 (shell, `withTenantScope`, `TenantTargetField`, `useIdempotentMutation`, `unsavedChangesGuard`), F050 (`apiClient`, theme), F021 (curriculum API), F022/F023 (question validator), F027 (media, preview session, sanitize SVG).
- Không tiêu thụ event. F054 dùng lại `ContentEditorShell`, `RichContentEditor`, `MathInput`.

### Bảo mật, tenant, idempotency, audit

- Idempotency-Key: `createQuestion`, `createQuestionVersion`, `transitionContentVersion`, `publishContentVersion`, `presignMediaUpload`, `createSubject`/`createCurriculum`/`createTopic`/`createObjective`, `exportLearningTranslations`, `importLearningTranslations`. `If-Match` cho mọi PATCH draft và transition/publish.
- System Admin: `OwnerScopeField` (Platform hoặc tenant đích qua `TenantTargetField`).
- Audit `content.version_created`, `content.submitted`, `content.approved`, `content.rejected`, `content.published`, `media.uploaded` ghi phía server.
- Chuỗi LaTeX coi là dữ liệu không tin cậy: MathJax cấu hình không nạp extension `require`/`html`/`href` ([SECURITY_ARCHITECTURE §8b](../../docs/10-security/SECURITY_ARCHITECTURE.md)).

### Quan sát

- Đo thời gian render công thức (client metric không PII) [Proposed]; không log đề/đáp án ra console; lỗi hiển thị `trace_id`.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.content_studio.question_bank`, `menu.content_studio.curriculum` |
| Action dùng | `action.content_studio.question_bank.view`, `action.content_studio.question_bank.edit`, `action.content_studio.question_bank.workflow`, `action.content_studio.question_bank.publish`, `action.content_studio.curriculum.view`, `action.content_studio.curriculum.edit`, `action.content_studio.reviews.preview`, `action.content_studio.media.view`, `action.content_studio.media.upload`, `action.content_studio.lessons.view`, `action.content_studio.translations.translate`, `action.content_studio.curriculum.publish` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `listCurriculumVersions`, `transitionCurriculumVersion`, `publishCurriculumVersion` (Cần thêm trong OpenAPI) → gắn `x-portal-action` khi thêm: `action.content_studio.curriculum.view`/`edit`; publish → `action.content_studio.curriculum.publish` (đã có trong catalog, UI-only tới khi có operation)

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

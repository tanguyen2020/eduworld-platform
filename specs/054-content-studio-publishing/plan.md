# Implementation Plan: F054 — Workspace B — media, bộ câu hỏi/lesson/blueprint, version và rollback, hàng đợi duyệt, preview, import, thùng rác

**Branch**: `054-content-studio-publishing` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Bổ sung vào workspace `content-studio` các màn quản lý/phát hành: tab Phiên bản + Phát hành + Sử dụng cho mọi nội dung có version (`VersionHistoryTab`, `VersionDiffView`, `PublishDialog` có hẹn giờ, `RollbackDialog`, `UnpublishDialog`, `ArchiveDialog`), `PreviewPanel` (render + "Thử làm"), `ReviewQueuePage`, `MediaLibraryPage`, `QuestionSetEditorPage`, `LessonBuilderPage`, `BlueprintEditorPage`, `ContentListPage` (tìm kiếm + bộ lọc + xóa mềm), `ImportWizardPage`, `TenantTrashPage`. Tất cả dùng `ContentEditorShell`, `RichContentEditor`, `renderMath` của F053 và `withTenantScope`/`useIdempotentMutation`/`ReasonDialog` của F051. Quy tắc phát hành (con trỏ, ghim version, reviewer ≠ author) do module learning; portal chỉ hiển thị và gọi API.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript (strict) · React 18 · Vite |
| Primary Dependencies | Ant Design 5 + Pro (ProTable, StepsForm, Timeline, Upload.Dragger), TanStack Query, `@dnd-kit/core`, SheetJS `xlsx` (đọc file trên trình duyệt) [Confirmed], `papaparse` (CSV) [Confirmed], `ops-ui` (F053), `diff` hiển thị theo field (tự viết trên JSON diff của `ContentVersionDiff`) |
| Storage | Không có (client); dữ liệu ở `eduworld_learning` (F024–F027) |
| Testing | Vitest + Testing Library + MSW (`// MOCK (test-only)`); Playwright E2E với backend local; axe |
| Target Platform | Trình duyệt; builder/diff/import ≥ 1024 px, duyệt và xem mọi breakpoint |
| Project Type | Web app (workspace chunk) |
| Performance Goals | Danh sách cursor; parse file 2.000 dòng < 3 s trên trình duyệt [Confirmed] |
| Constraints | AGENTS R10, R11, R21, R23, R31, R32 · ADR-0028, ADR-0031, ADR-0043, ADR-0044, ADR-0055, ADR-0056 |
| Scale/Scope | ~12 màn; tham số `scheduled_publish_min_lead`, `publish_to_visible_max`, `assets_per_version_max` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta contract (`listContentUsages`, `deleteContent`, `submitLearningPreviewResponse`, `cancelScheduledPublish`, lane, bộ lọc tìm kiếm, `pool_sizes`, `field_comments`, `math_assets` import) ở Phase 2 (T005–T012) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ API public module learning; nơi sử dụng từ projection phía server (F026) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Header tenant phiên; reviewer tenant không thấy lane platform; soft-delete; E2E cô lập (T024, T040, T055) |
| IV Server-authoritative | ✅ | Preview/thử làm chấm ở server, không lưu attempt; rollback chỉ đổi con trỏ; Idempotency-Key mọi thao tác phát hành |
| V Xác thực, phân quyền | ✅ | `learning_content.review/publish/import`, `content.release.rollback`, `tenant.trash.*`, `media.upload`; xử lý 403 |
| VI An toàn trẻ em | ✅ | Không dữ liệu học sinh; diff `answer_key` chỉ với `*.review` |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; giới hạn chưa đăng ký → task T064 |
| VIII UI tách logic, token | ✅ | Theme token; Tag trạng thái từ `shared-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F054-AC-* có test trước code |

## Project Structure

### Documentation (feature này)

```text
specs/054-content-studio-publishing/
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
src/cms/frontend/apps/portal/src/workspaces/content-studio/
├── release/{VersionHistoryTab.tsx,ReleaseTimeline.tsx,VersionDiffView.tsx,PublishDialog.tsx,RollbackDialog.tsx,
│            UnpublishDialog.tsx,ArchiveDialog.tsx,useVersions.ts,useRelease.ts}
├── preview/{PreviewPanel.tsx,TryOutRunner.tsx,usePreview.ts}
├── review/{ReviewQueuePage.tsx,ReviewTaskPage.tsx,BulkApproveDialog.tsx,useReviewTasks.ts}
├── usage/{UsageTab.tsx,useContentUsages.ts}
├── media/{MediaLibraryPage.tsx,MediaDetailDrawer.tsx,MediaUploadDropzone.tsx}   # MediaPicker/useMediaUpload từ F053
├── question-sets/{QuestionSetEditorPage.tsx,SelectionRuleEditor.tsx,useQuestionSet.ts}
├── lessons/{LessonBuilderPage.tsx,LessonBlockList.tsx,useLesson.ts}
├── blueprints/{BlueprintEditorPage.tsx,BlueprintMatrix.tsx,useBlueprint.ts}
├── list/{ContentListPage.tsx,ContentFilters.tsx,DeleteContentDialog.tsx,useContentList.ts}
├── import/{ImportWizardPage.tsx,parseImportFile.ts,renderImportMath.ts,useContentImport.ts}
└── trash/{TenantTrashPage.tsx,useTenantTrash.ts}
src/cms/frontend/apps/portal/tests/{unit/content-studio,e2e/content-studio}/
```

**Structure Decision**: Cùng chunk `content-studio` với F053 (một workspace B); thư mục theo nhóm màn của [ADMIN_PORTAL §3.11, §4, §5](../../docs/04-admin-portal/ADMIN_PORTAL.md).

## Phase 0: Research

Không có — ADR-0043/0044/0055 và tài liệu sở hữu đủ; lựa chọn thư viện đọc file ghi [Confirmed] ở Technical Context.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | `parseImportFile.ts` (đọc dòng, cột theo mẫu import), `renderImportMath.ts` (dùng `renderMath` F053, gom lỗi theo dòng), `releaseOptions.ts` (version hợp lệ để rollback/archive — chỉ để hiển thị) | Hàm thuần |
| application (client) | `useVersions` (`listContentVersions`, `diffContentVersions`, `listContentReleases`), `useRelease` (`publishContentVersion`, `cancelScheduledPublish`, `rollbackContent`, `unpublishContent`, `transitionContentVersion`), `usePreview`, `useReviewTasks`, `useContentUsages`, `useQuestionSet`, `useLesson`, `useBlueprint`, `useContentList`, `useContentImport`, `useTenantTrash` | Ghi qua `useIdempotentMutation` |
| infrastructure | `apiClient` (F050), PUT presigned upload | |
| presentation | Page/dialog trong cây trên | `LargeScreenRequired` cho builder/diff/import/rollback |

**Màn → permission (ẩn/hiện; server vẫn kiểm):**

| Màn / hành động | Permission |
|---|---|
| Xem phiên bản, phát hành, sử dụng, preview, danh sách | `learning_content.read` |
| Hàng đợi duyệt, duyệt/từ chối, diff `answer_key` | `learning_content.review` |
| Xuất bản ngay/hẹn giờ, hủy hẹn giờ, gỡ xuất bản | `learning_content.publish` |
| Rollback | `content.release.rollback` |
| Archive, tạo/sửa/xóa question set/lesson/blueprint | `learning_content.write` |
| Upload media | `media.upload` |
| Import | `learning_content.import` |
| Thùng rác tenant | `tenant.trash.read` / `tenant.trash.restore` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F053 (`ContentEditorShell`, `RichContentEditor`, `renderMath`, `MediaPicker`, `useWorkflow`), F051 (shell, tenant, idempotency, `ReasonDialog`), F024 (question set/lesson/blueprint, import, tìm kiếm), F025 (workflow duyệt, song ngữ), F026 (phát hành, nơi sử dụng), F027 (media, preview).
- Không tiêu thụ event.

### Bảo mật, tenant, idempotency, audit

- Idempotency-Key: `publishContentVersion`, `cancelScheduledPublish`, `rollbackContent`, `unpublishContent`, `transitionContentVersion`, `createCms*`, `create*Version`, `createContentImport`, `presignMediaUpload`, `restoreTenantTrashItem`, `createLearningPreviewSession`, `submitLearningPreviewResponse`. `If-Match` cho draft, transition, publish, delete.
- System Admin: danh sách đọc liên tenant `tenant_ids` + cột Tenant; mọi thao tác ghi chọn tenant đích (F051).
- Audit server: `content.published`, `content.publish_scheduled`, `content.rolled_back`, `content.unpublished`, `content.archived`, `content.deleted`, `content.restored`, `media.uploaded`; reason bắt buộc theo [ADMIN_PORTAL §7](../../docs/04-admin-portal/ADMIN_PORTAL.md).

### Quan sát

- Không log nội dung câu hỏi/đáp án ra console; lỗi hiển thị `trace_id`; đếm lỗi parse import theo mã (client, không PII) [Confirmed].


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.content_studio.reviews`, `menu.content_studio.media`, `menu.content_studio.question_sets`, `menu.content_studio.lessons`, `menu.content_studio.assessments`, `menu.content_studio.learning_paths` |
| Action dùng | `action.content_studio.reviews.view`, `action.content_studio.reviews.preview`, `action.content_studio.media.view`, `action.content_studio.media.upload`, `action.content_studio.question_sets.edit`, `action.content_studio.question_sets.view`, `action.content_studio.question_sets.workflow`, `action.content_studio.question_sets.publish`, `action.content_studio.question_sets.rollback`, `action.content_studio.lessons.edit`, `action.content_studio.lessons.view`, `action.content_studio.lessons.workflow`, `action.content_studio.lessons.publish`, `action.content_studio.lessons.rollback`, `action.content_studio.assessments.edit`, `action.content_studio.assessments.view`, `action.content_studio.assessments.workflow`, `action.content_studio.assessments.publish`, `action.content_studio.assessments.rollback`, `action.content_studio.question_bank.view`, `action.content_studio.question_bank.workflow`, `action.content_studio.question_bank.publish`, `action.content_studio.question_bank.rollback`, `action.content_studio.question_bank.import`, `action.content_studio.activities.view`, `action.content_studio.activities.workflow`, `action.content_studio.activities.publish`, `action.content_studio.activities.rollback`, `action.content_studio.learning_paths.view`, `action.content_studio.learning_paths.workflow`, `action.content_studio.learning_paths.publish`, `action.content_studio.learning_paths.rollback`, `action.school_lms.trash.view`, `action.school_lms.trash.restore` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- Catalog không có menu "Danh sách nội dung", "Import", "Thùng rác" trong workspace B: danh sách dùng `view` của từng menu nội dung, import là `action.content_studio.question_bank.import`, Thùng rác dùng `menu.school_lms.trash` (tenant) / `menu.platform_admin.trash` (platform) — cần xác nhận hoặc bổ sung catalog
- `cancelScheduledPublish`, `listContentUsages`, `deleteContent` (Cần thêm) → cần `x-portal-action` (đề xuất `…publish`, `…media.view`, `…edit` theo `content_type`)

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

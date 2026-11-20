# Feature Specification: F054 — Workspace B — media, bộ câu hỏi/lesson/blueprint, version và rollback, hàng đợi duyệt, preview, import, thùng rác

**Feature Branch**: `054-content-studio-publishing` · **Created**: 2026-11-16 · **Status**: Done · **Hoàn tất**: 2026-11-20 · **Module**: `src/cms/frontend/apps/portal` (workspace `content_studio`) · **Mốc**: M-C3 · **Phụ thuộc**: F024, F025, F026, F053
**Input**: Phần quản lý và phát hành của Learning Content Studio: lịch sử version, so sánh, xuất bản hẹn giờ, rollback, gỡ xuất bản, archive, preview và "thử làm"; hàng đợi duyệt; thư viện media và báo cáo "đang dùng ở đâu"; Question Set / Lesson / Assessment Blueprint builder; danh sách và tìm kiếm nội dung; import hàng loạt; xóa mềm và Thùng rác của tenant. FR nguồn: FR-CMS-006, FR-CMS-007, FR-CMS-014, FR-CNT-008, FR-CNT-009, FR-CNT-010, FR-CNT-021, FR-QB-011.

## Clarifications

### Session 2026-11-16

- Q: "Thử làm" trong preview chấm bằng API nào? → A: Contract chỉ có `createLearningPreviewSession` (trả render, không chấm) → "Cần thêm" `submitLearningPreviewResponse` (không lưu attempt, không phát event — ADR-0031), owner F027.
- Q: "Đang dùng ở đâu" của nội dung (FR-CNT-008)? → A: `MediaAsset.used_in` có sẵn cho media; nội dung chưa có → "Cần thêm" `listContentUsages`, owner F026.
- Q: Xóa bộ câu hỏi/lesson/blueprint? → A: Contract chỉ có `deleteQuestion` → "Cần thêm" `deleteContent` chung theo `content_type`.
- Q: Lane hàng đợi duyệt? → A: [ADMIN_PORTAL §3.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) dùng `tenant`/`platform_learning`; contract `ContentReviewTask.lane` còn `school`/`organization` → "Cần sửa" (D8). Portal hiển thị theo giá trị contract sau khi sửa.
- Q: Thùng rác của System Admin? → A: Workspace D, F057. F054 chỉ Thùng rác tenant ([Confirmed], hiện khi có `tenant.trash.read`).
- Q: Import `lessons_docx` (Word/MathType)? → A: Ngoài phạm vi F054 (không render công thức MathType trên trình duyệt được); F054 làm `questions_xlsx`, `questions_csv`.

### Session 2026-11-20 (triển khai — quyết định [Confirmed] và câu hỏi mở)

- Q: Delta contract T005–T012 khi triển khai? → A: Đã có trên main dưới tên khác, F054 **không** sửa OpenAPI: `listContentUsages` = `getContentUsages` (F026, `ContentUsage.ref_type`, `pinned_sessions`); `deleteContent` = các operation xóa theo loại `deleteQuestion`, `deleteCmsQuestionSet`, `deleteCmsLesson`, `deleteCmsLearningMaterial`, `deleteCmsAssessmentBlueprint` (F022/F024, `If-Match`, `reason`); `submitLearningPreviewResponse` = `submitPreviewResponse` (F027, `PreviewResponseResult` có `feedback[]`, `hint_levels[]`, `explanation`); `cancelScheduledPublish` = `cancelScheduledPublication` theo `release_id` (F026); enum `lane` = `tenant | platform_learning` (F025); bộ lọc `topic_id`, `objective_id`, `difficulty` của `searchQuestions` và `topic_id`, `objective_id` của list question set/lesson/blueprint (F024); `pool_sizes` = `getQuestionSetPoolStats` / `getAssessmentBlueprintPoolStats` (F024, `insufficient`/`warning` theo luật).
- Q: `field_comments[]` của `transitionContentVersion` chưa có (body `additionalProperties: false`)? → A: [Confirmed] Portal ghép ghi chú theo trường vào `note` dạng dòng `[đường dẫn] ghi chú` (tổng ≤ 2000) để không bị `422`. **OQ F054-1** (owner F025, đã chốt 2026-11-20): thêm `field_comments[] { path ≤ 256, comment ≤ 1000 }` vào contract + lưu ở review task để tác giả thấy theo trường.
- Q: `ContentImportRequest.math_assets[]` (T012)? → A: [Confirmed] Không cần: mẫu cột `questions-row.schema.json` đã có cột `math_assets` mỗi dòng. Portal đọc file trên trình duyệt, render mọi công thức (MathJax 4, giới hạn `latex_max_length`/`latex_max_depth`/`math_svg_max`), báo lỗi theo dòng **trước khi gửi**, ghi SVG + lời đọc vào cột `math_assets` còn trống rồi tải lên **CSV chuẩn hóa** (`questions_csv`, giữ nguyên số dòng; tệp XLSX cũng được chuẩn hóa thành CSV). Tệp đi qua object storage nên không cần chia theo `request_body_max`.
- Q: SheetJS `xlsx` / `papaparse` (T004)? → A: [Confirmed] Không thêm phụ thuộc: bản `xlsx` trên npm (0.18.5) còn advisory chưa vá. Portal tự đọc CSV (RFC 4180) và sheet đầu của XLSX (zip đọc bằng `DecompressionStream('deflate-raw')` của trình duyệt + `DOMParser`).
- Q: Bộ lọc "ngôn ngữ" và `translation_status` của danh sách (AC-24)? → A: Contract chưa có tham số này cho `searchQuestions`/list → **OQ F054-2** (owner F024, đã chốt 2026-11-20); portal chưa hiển thị hai bộ lọc này. Bộ lọc "nguồn" (`source`) chỉ có ở câu hỏi.
- Q: "Đang dùng ở đâu" gồm quest/challenge (module game/competition)? → A: `ContentUsage.ref_type` hiện chỉ có tham chiếu của module learning (question set, lesson, learning material, blueprint, activity, assignment, exam session) → **OQ F054-3** (owner F026 → producer F078/F108, đã chốt 2026-11-20): thêm tham chiếu từ projection game/competition.
- Q: Danh sách nội dung, Import, Thùng rác không có menu riêng trong catalog? → A: Theo plan: danh sách = màn chính của từng menu nội dung (`question_bank`, `question_sets`, `lessons`, `assessments`); Import = route `question-bank/import` gắn `action.content_studio.question_bank.import`; Thùng rác tenant = `menu.school_lms.trash` (workspace C, mã đặt ở `workspaces/school-lms/trash/` để giữ chunk theo workspace). Lộ trình học chưa có API soạn thảo: menu `learning_paths` mở theo mã để quản lý phiên bản/phát hành/sử dụng/xem trước.
- Q: Tham số không đọc được từ portal (`scheduled_publish_min_lead` là platform setting)? → A: Dùng giá trị CONFIG_DEFAULTS trong `shared/config.ts` (5 phút; `content_review_sla_days` = 3; `content_review_bulk_max` = 50; `media_max_bytes_by_kind`; `publish_to_visible_max` = 30 s); server kiểm lại. Giới hạn tệp import (tài liệu) 20 MB [Confirmed].
- Q: Kéo-thả khối bài học? → A: [Confirmed] HTML5 drag & drop + nút Lên/Xuống (bàn phím), không thêm `@dnd-kit` (giống F053). Ma trận đề sửa section đầu; section khác giữ nguyên [Confirmed].
- Q: "Thử làm" mọi dạng câu? → A: [Confirmed] Portal nhận câu trả lời cho một/nhiều lựa chọn, đúng/sai, điền chỗ trống, trả lời ngắn, số, sắp xếp; nối cặp, kéo thả, câu dựa trên hình, tương tác hiển thị "thử làm trong LMS/game".
- Q: `ResponsiveTable` dùng chung workspace B và C? → A: Chuyển sang `src/shell/ResponsiveTable.tsx` (nhãn "Xem thêm" truyền vào), `school-lms/common/ResponsiveTable` giữ wrapper — không import chéo workspace.
- Q: E2E Playwright với backend thật (T003, T022–T024, T032, T040, T047, T055, T056, T061, T067)? → A: Như F051–F053 (OQ F051-9): mọi F054-AC-* được kiểm ở mức integration toàn portal với MOCK backend hợp đồng (`src/test/mock-publishing.ts`, nhãn MOCK); E2E backend thật, axe và baseline visual cho story mới chờ CI khi backend nối route.

### Session 2026-11-20 (owner chốt)

- OQ F054-1 → **Chốt:** owner F025 thêm `field_comments[]` (`path` ≤ 256, `comment` ≤ 1000, maxItems 50) vào `transitionContentVersion`; portal gửi theo trường và hiển thị cho tác giả theo trường, bỏ cách ghép vào `note`. Task ở F025 và tasks.md này.
- OQ F054-2 → **Chốt:** owner F024 thêm `content_language`, `translation_status` cho `searchQuestions` và list bốn loại; portal thêm hai bộ lọc (F054-AC-24). Task ở F024 và tasks.md này.
- OQ F054-3 → **Chốt:** "Đang dùng ở đâu" thêm quest/mini-game/challenge/thi đấu qua projection tham chiếu ngoài của module learning (Session owner chốt của F026); producer F078/F108. Portal hiển thị `ref_type` mới khi contract có (không cần task riêng nếu bảng dùng nhãn theo `ref_type`; nhãn dịch thêm cùng task F078).
- E2E backend thật, axe, baseline visual (OQ F051-9) → chạy khi có môi trường compose — không phải câu hỏi mở.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Lịch sử version, xuất bản hẹn giờ, rollback, preview (Priority: P1)

Reviewer/Academic Manager xem mọi version, so sánh, xem chỉ đọc, xuất bản ngay hoặc hẹn giờ, dùng lại version cũ cho học sinh (rollback) có lý do, gỡ xuất bản, archive; xem trước và thử làm như học sinh. (US-CMS-002)

**Why this priority**: Kiểm soát phát hành là điều kiện để trường dùng nội dung ở M-C3 mà không làm hỏng lượt học đang chạy.

**Independent Test**: Câu hỏi v1 → v2 xuất bản; rollback về v1 có lý do; lượt đang làm giữ v2, lượt mới nhận v1 (AC-CNT-002).

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F054-AC-01 | Tab "Phiên bản" PHẢI liệt kê `listContentVersions` (số, trạng thái bằng Tag màu nghiệp vụ, tác giả, người duyệt, người xuất bản, `change_note`, thời gian, lọc trạng thái) và dòng thời gian `listContentReleases` (publish/rollback/unpublish, người làm, lý do). | FR-CNT-009 · [ADMIN_PORTAL §3.11](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-001 |
| F054-AC-02 | KHI mở một version `published` hoặc `archived`, Portal PHẢI hiển thị chỉ đọc, không có nút Sửa, có "Tạo bản nháp từ phiên bản này". | FR-CNT-010, AC-CNT-003, FR-CMS-007 | TC-CMS-001 |
| F054-AC-03 | KHI chọn hai version, Portal PHẢI gọi `diffContentVersions` và hiển thị diff theo field/khối (văn bản, đáp án, media, metadata); diff `answer_key` chỉ hiển thị khi người dùng có `learning_content.review`. | FR-CNT-010, AC-CNT-003 | — |
| F054-AC-04 | KHI chọn "Hẹn giờ" xuất bản, Portal PHẢI chỉ cho chọn `publish_at` ≥ hiện tại + `scheduled_publish_min_lead` (hiển thị giờ ICT, gửi UTC), hiện hộp tóm tắt diff với version đang xuất bản và cảnh báo media chưa `ready`, và cho hủy hẹn giờ trước giờ chạy (Cần thêm). | FR-CNT-009 · ADR-0043 | TC-CNT-004 |
| F054-AC-05 | KHI rollback, Portal PHẢI chỉ cho chọn version `published` khác con trỏ hiện tại của cùng item, bắt buộc lý do (≤ `maxLength` của `RollbackRequest.reason`), xác nhận lần 2, hiển thị ảnh hưởng (lượt đang chạy giữ version cũ, lượt mới nhận version chọn) và gọi `rollbackContent` (Idempotency-Key). | FR-CNT-009, AC-CNT-002 | TC-CNT-002 |
| F054-AC-06 | NẾU server trả `ROLLBACK_TARGET_INVALID` hoặc `403 PERMISSION_DENIED`, THÌ Portal PHẢI hiển thị lỗi tương ứng; nút Rollback ẩn khi thiếu `content.release.rollback`. | FR-CMS-006 · [ADMIN_PORTAL §3.11](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CNT-002 |
| F054-AC-07 | KHI gỡ xuất bản, Portal PHẢI bắt buộc lý do, cảnh báo bài giao/quest/đề đang tham chiếu (từ báo cáo nơi sử dụng) và gọi `unpublishContent` (Idempotency-Key). | FR-CNT-009, FR-CNT-008 | — |
| F054-AC-08 | KHI archive, Portal PHẢI chỉ cho version không phải con trỏ hiện hành, bắt buộc lý do, gọi `transitionContentVersion` action `archive`. | FR-CMS-006 · [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) | — |
| F054-AC-09 | Portal PHẢI không cung cấp bất kỳ thao tác nào sửa version đã xuất bản, xóa version đã xuất bản hay thay đổi kết quả bài đã làm; hành động rollback/gỡ xuất bản hiển thị màu cảnh báo và luôn có lý do. | FR-CMS-007 · ADR-0043 | — |
| F054-AC-10 | KHI bấm "Xem trước", Portal PHẢI gọi `createLearningPreviewSession` cho version đã chọn với công tắc vi / en / song song, render bằng renderer web dùng chung; ở chế độ "Thử làm", Portal PHẢI gửi câu trả lời qua `submitLearningPreviewResponse` (Cần thêm), hiển thị gợi ý từng cấp và phản hồi theo phương án như học sinh thấy, không tạo attempt. | FR-CMS-006, US-CMS-002 · [ADMIN_PORTAL §5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F054-AC-11 | Thao tác ghi quan trọng (xuất bản, hẹn giờ, rollback, gỡ xuất bản, archive, duyệt/từ chối, khôi phục, import) PHẢI gửi một Idempotency-Key ổn định cho mỗi ý định người dùng (thử lại dùng lại key) và chỉ hiển thị một kết quả. | NFR-SEC-004 · AGENTS R10 | TC-CNT-003 |

---

### User Story 2 — Hàng đợi duyệt (Priority: P1)

Reviewer của tenant (lane tenant) hoặc Content Reviewer platform (lane `platform_learning`) xem việc chờ duyệt, mở diff + preview, duyệt/từ chối có lý do, duyệt hàng loạt.

**Why this priority**: Reviewer ≠ author là bắt buộc; không có hàng đợi thì reviewer không biết việc cần làm.

**Independent Test**: `cm.noidung` gửi 3 câu; `cm.duyet` duyệt hàng loạt 2 câu, từ chối 1 câu có lý do.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F054-AC-12 | KHI người có `learning_content.review` mở "Duyệt nội dung", Portal PHẢI liệt kê `listContentReviewTasks` theo lane, `content_type`, "giao cho tôi", hiển thị thời điểm gửi và cảnh báo quá hạn theo [ADMIN_PORTAL §4](../../docs/04-admin-portal/ADMIN_PORTAL.md). | FR-CMS-006 | — |
| F054-AC-13 | KHI mở một task, Portal PHẢI mặc định hiển thị diff với version đang xuất bản và preview, cho Duyệt hoặc Từ chối (lý do bắt buộc, ghi chú theo field) qua `transitionContentVersion`. | FR-CMS-006 · [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001 |
| F054-AC-14 | KHI duyệt hàng loạt, Portal PHẢI giới hạn số item theo [ADMIN_PORTAL §4](../../docs/04-admin-portal/ADMIN_PORTAL.md), gọi từng item với Idempotency-Key riêng và báo kết quả từng item (kể cả lỗi người duyệt là tác giả). | FR-CMS-006 | — |
| F054-AC-15 | TRONG KHI người dùng là reviewer của tenant, Portal PHẢI không hiển thị task lane platform hay task của tenant khác (chỉ dữ liệu API trả về, không gửi `tenant_ids`). | AC-CMS-002 · ADR-0027 | TC-CMS-TEN-01 |

---

### User Story 3 — Thư viện media và "đang dùng ở đâu" (Priority: P2)

Biên tập viên tìm, upload, xem trạng thái pipeline, biến thể, license, alt text; xem nội dung đang tham chiếu media và nơi đang dùng một nội dung trước khi archive/gỡ/xóa.

**Why this priority**: Upload cơ bản đã có trong editor (F053); thư viện là quản lý tập trung.

**Independent Test**: Upload file `.exe` đổi đuôi `.png` → `rejected` có lý do; mở một ảnh đang dùng → danh sách version tham chiếu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F054-AC-16 | Thư viện media PHẢI liệt kê `listMediaAssets` lọc `kind`/`status`/`q`, hiển thị trạng thái `quarantined`/`processing`/`ready`/`rejected` kèm `reject_reason`, biến thể, `license`, `alt_text`. | FR-CNT-008 · [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-008 |
| F054-AC-17 | KHI upload (kéo-thả nhiều file), Portal PHẢI kiểm tra loại và dung lượng theo [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md) trước khi gọi `presignMediaUpload`, bắt buộc `alt_text` (ảnh) và `license`, hiển thị tiến trình tới `ready`/`rejected`; NẾU server trả `MEDIA_TYPE_NOT_ALLOWED`/`MEDIA_TOO_LARGE`, THÌ hiển thị lý do. | AC-CNT-004 · ADR-0044 | TC-CNT-008 |
| F054-AC-18 | Chi tiết media PHẢI hiển thị "đang dùng ở version nào" từ `used_in`; thay ảnh PHẢI là upload asset mới rồi chọn trong bản nháp (không có thao tác ghi đè file). | FR-CNT-008 · ADR-0044 | — |
| F054-AC-19 | KHI mở tab "Sử dụng" của một nội dung, Portal PHẢI hiển thị nơi đang tham chiếu (question set, lesson, đề, bài giao, quest, challenge) từ `listContentUsages` (Cần thêm) và hiển thị lại danh sách này trong hộp xác nhận archive/gỡ xuất bản/xóa. | FR-CNT-008 | — |

---

### User Story 4 — Question Set, Lesson, Assessment Blueprint builder (Priority: P2)

Biên tập viên dựng bộ câu hỏi (tĩnh hoặc theo luật chọn), bài học nhiều khối, ma trận đề; xem trước đề mẫu từ ma trận.

**Why this priority**: Cần cho giao bài/đề ở M-C3 (F041, F042); dùng lại `ContentEditorShell` của F053.

**Independent Test**: Tạo blueprint "Phép nhân" 3 LO × 3 mức độ khó, sinh đề mẫu; pool thiếu → thông điệp rõ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F054-AC-20 | Question Set editor PHẢI hỗ trợ chế độ tĩnh và chế độ luật chọn theo `question-set.schema.json`, hiển thị kích thước pool hiện tại cho từng luật (Cần sửa). | FR-QB-004 · [ADMIN_PORTAL §3.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F054-AC-21 | Lesson Builder PHẢI cho kéo-thả khối `rich_text`/`media`/`worked_example`/`activity` (≤ `maxItems`), đánh dấu khối bắt buộc, chọn `completion_rule`, gắn LO, và gửi công thức trong `math_assets`. | FR-CNT-020 · [ADMIN_PORTAL §3.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F054-AC-22 | Assessment Blueprint editor PHẢI hiển thị ma trận LO × độ khó, kiểm tổng điểm, và nút "Sinh đề mẫu" gọi `createLearningPreviewSession` (không lưu đề); NẾU server trả `QUESTION_POOL_INSUFFICIENT`, THÌ chỉ ra ô thiếu câu. | FR-QB-005 · [ADMIN_PORTAL §5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F054-AC-23 | KHI lưu bản nháp question set/lesson/blueprint, Portal PHẢI dùng `updateCmsQuestionSetDraft`/`updateCmsLessonDraft`/`updateCmsAssessmentBlueprintDraft` kèm `If-Match`; NẾU nhận `CONTENT_NOT_EDITABLE`, THÌ chuyển chỉ đọc và gợi ý tạo bản nháp. | ADR-0043 · [CONTENT_MODEL §5](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-003 |

---

### User Story 5 — Danh sách và tìm kiếm, import hàng loạt, xóa mềm và Thùng rác (Priority: P3)

Biên tập viên tìm nội dung theo metadata và từ khóa không dấu, import câu hỏi từ Excel/CSV có báo lỗi theo dòng, xóa mềm, Tenant Admin khôi phục từ Thùng rác.

**Why this priority**: Tăng năng suất; dữ liệu nhỏ ban đầu vẫn dùng được danh sách cơ bản.

**Independent Test**: Import CSV 20 câu (2 dòng lỗi, 1 công thức lỗi) chạy thử → báo đúng dòng; xóa 1 câu → vào Thùng rác → khôi phục.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F054-AC-24 | Danh sách nội dung PHẢI lọc theo môn, khối, loại, trạng thái, ngôn ngữ, nguồn (tenant/được gán) và từ khóa `q` (server tìm không dấu); System Admin thấy cột Tenant (theo F051). | FR-CNT-021, AC-CMS-003 | TC-CMS-TEN-02 |
| F054-AC-25 | KHI xóa nội dung, Portal PHẢI gọi `deleteQuestion`/`deleteContent` (Cần thêm) kèm `If-Match`, bắt buộc lý do với nội dung đã xuất bản, sau đó bản ghi biến mất khỏi danh sách; danh sách thường không có bộ lọc "hiện cả đã xóa". | FR-CMS-014, AC-CMS-004 · ADR-0028 | TC-CMS-DEL-01, TC-TEN-015 |
| F054-AC-26 | TRONG KHI người dùng có `tenant.trash.read`, Portal PHẢI hiển thị Thùng rác của tenant (`listTenantTrash`, lọc `resource_type`) và khôi phục qua `restoreTenantTrashItem` khi có `tenant.trash.restore`; `restorable = false` hoặc `409` PHẢI hiển thị lý do trùng mã. | FR-CMS-014 | TC-CMS-DEL-02 |
| F054-AC-27 | KHI import câu hỏi, Portal PHẢI đọc file `xlsx`/`csv` trên trình duyệt, render công thức (MathJax 4) và báo lỗi công thức theo dòng trước khi gửi, upload file qua `presignMediaUpload`, gọi `createContentImport` `dry_run = true`, hiển thị lỗi theo dòng (`row`, `field`, `code`, `message`) từ `getContentImport`, rồi mới cho chạy thật tạo Draft. | FR-QB-011 · ADR-0055 | — |
| F054-AC-28 | Portal PHẢI ẩn Import khi thiếu `learning_content.import`, ẩn Xuất bản/Gỡ xuất bản khi thiếu `learning_content.publish`, ẩn Thùng rác khi thiếu `tenant.trash.read`, và hiển thị thông điệp không có quyền khi server trả `403 PERMISSION_DENIED`. | AC-CMS-001 · [ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F054-AC-29 | Portal PHẢI giới hạn mọi ô nhập (lý do, `change_note`, ghi chú duyệt, tìm kiếm, tiêu đề) theo `maxLength` của OpenAPI và ánh xạ `422 VALIDATION_FAILED` về đúng trường. | ADR-0041 | TC-TEN-024 |
| F054-AC-30 | Ở NƠI độ rộng < 768 px, Portal PHẢI hiển thị danh sách dạng thẻ và cho đọc, preview, duyệt/từ chối có lý do; diff, import, builder và rollback hiển thị "Tác vụ này cần màn hình lớn hơn" (< 1024 px). | [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F054-AC-31 | KHI menu/action của feature (`menu.content_studio.reviews`, `menu.content_studio.media`, `menu.content_studio.question_sets`, `menu.content_studio.lessons`, `menu.content_studio.assessments`, `menu.content_studio.learning_paths` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU version được hẹn giờ bị sửa trạng thái ở nơi khác trước giờ chạy, THÌ Portal PHẢI làm mới thanh trạng thái khi nhận `409`/`412` (AC-04, AC-11).
- NẾU rollback tới version đang là con trỏ, THÌ lựa chọn đó bị vô hiệu (AC-05).
- NẾU import có > `maxItems` lỗi của `ContentImportJob.errors`, THÌ Portal PHẢI hiển thị số lỗi tổng và nói rõ danh sách bị cắt.
- NẾU media `rejected`, THÌ không cho chọn vào bản nháp (AC-16).
- NẾU khôi phục bản ghi có cha đã xóa, THÌ hiển thị thông điệp server theo quy tắc aggregate (AC-26).
- NẾU nội dung là dữ liệu platform được gán, THÌ không có Xóa/Rollback/Gỡ xuất bản cho người dùng tenant (chỉ đọc, F053-AC-11).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CMS-006 | Hàng đợi duyệt, archive, preview/thử làm, giao diện rollback; audit hiển thị ở dòng thời gian | [ADMIN_PORTAL §3.5, §3.11, §4, §5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-007 | Không thao tác nào sửa lịch sử: version published chỉ đọc, không xóa version published | [ADMIN_PORTAL §9](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-014 | Xóa mềm, Thùng rác tenant | ADR-0028 · [ADMIN_PORTAL §3.8](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CNT-008 | Báo cáo nơi sử dụng (media + nội dung), cảnh báo trước archive/gỡ/xóa | [CONTENT_MODEL](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-009 | Xuất bản ngay/hẹn giờ, rollback, gỡ xuất bản, lịch sử release | [CONTENT_MODEL §5](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-010 | Xem version chỉ đọc, so sánh hai version, preview | [CONTENT_MODEL §5.1](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-021 | Danh sách + tìm kiếm theo metadata và từ khóa | [CONTENT_MODEL](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-QB-011 | Import câu hỏi Excel/CSV có dry-run, lỗi theo dòng | [QUESTION_BANK §10](../../docs/02-learning/QUESTION_BANK.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Version nội dung, release**: bản nội dung theo số version; release là lần đổi con trỏ (publish/rollback/unpublish) có người làm và lý do.
- **Việc duyệt (review task)**: version chờ duyệt theo lane và tenant.
- **Media asset**: file qua pipeline, biến thể, nơi đang dùng.
- **Bộ câu hỏi, bài học, ma trận đề**: nội dung có version, dùng lại câu hỏi và khối nội dung.
- **Lần import**: file, chạy thử, lỗi theo dòng, số bản nháp tạo.
- **Mục Thùng rác**: bản ghi đã xóa mềm, có thể khôi phục.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `scheduled_publish_min_lead` | Thời điểm sớm nhất được hẹn giờ xuất bản |
| `publish_to_visible_max` | Thông điệp "Đang chuẩn bị cache → Đã xuất bản" |
| `assets_per_version_max` | Giới hạn asset khi dựng lesson/question set |
| `latex_max_length` / `latex_max_depth` / `math_svg_max` | Kiểm công thức khi import |
| `request_body_max` | Chia nhỏ `math_assets` khi import lớn |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không sửa version published; rollback chỉ đổi con trỏ | AGENTS R11, R31 · ADR-0043 |
| Chỉ soft-delete; không "xóa vĩnh viễn" | AGENTS R21 · ADR-0028 |
| Preview không tạo attempt/learning event/XP | ADR-0031 · AGENTS R23 |
| Theme token, WCAG 2.1 AA | NFR-MAINT-007, NFR-A11Y-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Reviewer xử lý (duyệt hoặc từ chối có lý do) một việc trong hàng đợi trong ≤ 2 phút gồm xem diff và preview.
- **SC-002**: 100% thao tác rollback/gỡ xuất bản có lý do và xuất hiện trong dòng thời gian release.
- **SC-003**: Import 200 câu hỏi có báo đúng 100% dòng lỗi ở bước chạy thử, không tạo bản nháp nào trước khi chạy thật.
- **SC-004**: Bản ghi bị xóa không xuất hiện ở bất kỳ danh sách thường nào và khôi phục được từ Thùng rác.
- **SC-005**: 0 lần double-submit tạo hai release hoặc hai import.

## Ngoài phạm vi

- Editor câu hỏi, công thức, song ngữ, cây curriculum, `ContentEditorShell` → F053.
- Thùng rác mọi tenant, gán/clone dữ liệu platform → F057.
- `GamePreviewFrame` (Godot Web) cho nội dung game → F079; preview học tập dùng renderer web.
- Import `lessons_docx` (Word/MathType) → công cụ theo CONTENT_PRODUCTION_PLAN, chưa có feature portal.
- Chấm lại hệ thống (re-evaluate) → chưa có feature portal sở hữu (backend F031).
- Item analysis, thống kê chất lượng câu hỏi trong Question Bank browser → F044 (backend), UI cùng F056.

## Assumptions

- Giới hạn duyệt hàng loạt, SLA hàng đợi, giới hạn dung lượng media theo [ADMIN_PORTAL §4](../../docs/04-admin-portal/ADMIN_PORTAL.md) và [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md) [Confirmed], chưa đăng ký trong CONFIG_DEFAULTS (task T064).
- Thùng rác tenant là [Confirmed] (FR-CMS-014); màn chỉ hiện khi có permission.
- `listContentUsages` gồm tham chiếu từ module game/module competition qua projection của F026.

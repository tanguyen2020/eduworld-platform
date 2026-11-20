# Feature Specification: F024 — Question set, lesson, assessment blueprint, worked example, import hàng loạt, tìm kiếm

**Feature Branch**: `024-question-set-lesson-blueprint` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C2 · **Phụ thuộc**: F022
**Input**: Các loại nội dung ghép từ câu hỏi và khối nội dung: question set (tĩnh/theo luật; `purpose = exercise` là Exercise), lesson và learning material (gồm khối `worked_example`), assessment blueprint (ma trận đề); import hàng loạt; tìm kiếm full-text tiếng Việt không dấu. FR nguồn: FR-CNT-001, FR-CNT-007, FR-CNT-020, FR-CNT-021, FR-QB-004, FR-QB-005, FR-QB-011, FR-CMS-003.

## Clarifications

### Session 2026-11-16

- Q: `worked_example_ref` của câu hỏi là UUID (schema) hay `{content_item_id, block_id}` (CONTENT_MODEL §9.2)? → A: Theo tài liệu sở hữu [CONTENT_MODEL §9.2](../../docs/02-learning/CONTENT_MODEL.md#92-khối-worked_example-confirmed): đối tượng `{content_item_id, block_id}` (đổi ở Session 2026-11-17); `question.schema.json` và `interactive-exercise.schema.json` sửa ở Phase 2.
- Q: Khối `activity` trong lesson tham chiếu activity của F030 — F024 kiểm thế nào? → A: Khối `activity` cần validator do F030 đăng ký; trước đó lesson có khối `activity` bị từ chối rõ ràng (`errors[].code = BLOCK_KIND_NOT_ENABLED`).
- Q: "Tạo đề từ Question Bank theo blueprint" (FR-QB-005) gồm sinh `ExamPaper`? → A: Không. F024 soạn và kiểm blueprint (tổng điểm, pool đủ, ràng buộc phản hồi); sinh đề/biến thể là F042 ([EXAM_ENGINE §4](../../docs/02-learning/EXAM_ENGINE.md#4-sinh-đề)).
- Q: Import Word/MathType (`lessons_docx`)? → A: Theo [CONTENT_PRODUCTION_PLAN](../../docs/02-learning/CONTENT_PRODUCTION_PLAN.md), ngoài phạm vi F024; `kind = lessons_docx` trả lỗi rõ ràng.

### Session 2026-11-17

- Q: Shape `worked_example_ref`? → A: `{content_item_id, block_id}` theo [QUESTION_BANK §2](../../docs/02-learning/QUESTION_BANK.md) và DATABASE_DESIGN `question_versions` (RB-203): `content_item_id` là id lesson hoặc learning material, không có trường `content_type` riêng.

### Session 2026-11-19 (triển khai)

- OQ F024-1: Bốn bảng version (`question_set_versions`, `lesson_versions`, `learning_material_versions`, `assessment_blueprint_versions`) dùng **cùng bộ cột**: metadata chung (`title`, `summary`, `subject_id`, `curriculum_version_id`, `grade_levels`, `topic_ids`, `objective_ids`, `media_refs`, `worked_example_ids` — jsonb như OQ F021-2/F022-5) và `body jsonb` (≤ 512 KB) chứa phần riêng của loại (set: `definition` + `default_feedback_timing`; lesson: `blocks`, `completion_rule`, `estimated_minutes`; material: `blocks`, `material_kind`; blueprint: `definition`, `exam_type`, `feedback_timing`, `result_release`, `efficacy_instrument`) thay cho các cột riêng của data-model (`kind`, `purpose`, `selection_rules`, `sections`, `scoring`, `shuffle_policy`, `result_visibility`…). Repository của bốn loại sinh từ một mẫu (`scripts/dev/gen-learning-docs.mjs`). → [Confirmed].
- OQ F024-2: Không có cột `search_text` do ứng dụng ghép: `search_tsv` là cột sinh `to_tsvector('simple', content.f_search_text(title, summary, body))` (`content.f_search_text(VARIADIC jsonb[])` IMMUTABLE lấy mọi giá trị chuỗi, `unaccent`, cắt 20.000 ký tự) + GIN; `question_versions.search_tsv` sinh từ `stem`, `render_payload` (cột viết tay trong migration F024, ngoài ent). `content.forbid_published_update()` coi `search_tsv` là cột ghi sổ. → [Confirmed].
- OQ F024-3: Input bổ sung `curriculum_version_id`, `topic_ids`/`topic_id`, `objective_ids` cho set/lesson/material/blueprint (contract); lesson/material dùng `grade_level`/`topic_id` đơn (như `LessonAuthoringInput`) lưu vào mảng. LO của khối `worked_example` được gộp vào `objective_ids` của version. PATCH nhận `AuthoringPatch` (mọi trường tùy chọn, server kiểm lại cả version) thay `*AuthoringInput` có `required`. → [Confirmed].
- OQ F024-4: Không có endpoint GET version cho bốn loại: `ETag` của `getCms<Type>` là row version của **version mới nhất**; `deleteCms<Type>` so `If-Match` với giá trị đó (khác OQ F022-6 của câu hỏi). → [Confirmed].
- OQ F024-5: Tham chiếu câu hỏi của set/blueprint (F024-AC-02): câu riêng tenant khác, câu không tồn tại, hoặc set platform tham chiếu câu tenant → `422 VALIDATION_FAILED` kèm pointer (không phân biệt 404 để không lộ dữ liệu); câu platform chưa gán → `403 RESOURCE_NOT_ASSIGNED` (kiểm bằng `SystemPlatformScope("learning-reference-check")`, như F020). → [Confirmed].
- OQ F024-6: Pool stats đếm câu có con trỏ release trong phạm vi (view `question_delivery_v`, bỏ `efficacy_instrument`), lọc theo `subject_id` của version rồi khớp luật không nới lỏng (`selection.Available`); luật blueprint với `fixed_question_version_ids` đếm câu cố định đang là release. `variant_balance_max_mean_p_diff` dùng hằng 0,05 (`blueprint.DefaultMaxMeanPDiff`) vì module learning chưa đọc platform settings (như OQ F022-9) và được **ghi vào** `definition.variants.balance` khi thiếu. → [Confirmed].
- OQ F024-7: Import (`createContentImport`): `questions_csv`, `questions_xlsx` (excelize, sheet đầu), `questions_qti_lite` (zip `assessmentItem` + `metadata.csv`; choice/order/textEntry), `content_json` (`{"questions": [QuestionAuthoringInput…]}` dùng id) cần `learning_content.import`; `lessons_docx` → `422` `errors[].code = IMPORT_KIND_NOT_SUPPORTED`. Mỗi dòng chạy validator của `createQuestion` trong savepoint riêng: dòng hợp lệ thành draft (job `succeeded` khi có ít nhất một dòng tạo được, khác import curriculum all-or-nothing của F021), dòng lỗi vào `errors[]` theo số dòng (dòng 1 là tiêu đề); thiếu `math_assets` cho công thức → `MATH_RENDER_FAILED`. Cảnh báo trùng `content_hash` chưa làm. Import lesson/material/set bằng `content_json` chưa làm (chỉ câu hỏi). → [Confirmed].
- OQ F024-8: Clone question set (F024-AC-22): câu riêng tenant nguồn được clone (draft, `cloned_from_*`) và item trỏ sang version mới, câu platform giữ nguyên; `id_map` của bước clone (`cloned[]`) chỉ chứa cặp set — cặp câu hỏi con được ghi vào `IDMap` của bước (dùng lại trong cùng job), chưa trả qua facade. → [Confirmed].
- OQ F024-9: `learning_material` thêm vào `ResourceType` (contentmodel, `internal-common` enum, `resource.deleted/restored.v1`) để Thùng rác/gán/clone dùng chung (F024-AC-23). → [Confirmed].
- OQ F024-10: Khối `activity` luôn `BLOCK_KIND_NOT_ENABLED` tới F030 (`command.Docs.ActivityEnabled`). Media của khối `media`/`audio_ref` được kiểm và ghi `media_usages` khi môi trường có thư viện media (F027). → [Confirmed].
- OQ F024-11: Test tích hợp đặt phẳng `tests/integration/content_*_test.go`; benchmark 100.000 version (T038) chưa chạy — GIN được tạo, chưa đo p95. → [Confirmed].
- OQ F024-12: Route/handler/job chưa nối vào binary (cùng OQ F010-2/F020-3). → [Confirmed].

### Session 2026-11-20 (follow-up OQ F053)

- Q: Công thức trong khối rich_text/worked_example của lesson? → A: `LessonAuthoringInput.math_assets[]` (cùng shape với câu hỏi) lưu trong body của version lesson; khi có pipeline công thức (F027) server kiểm LaTeX, sanitize SVG và lưu `math/<sha256>.svg` như câu hỏi, trả `svg_asset` (không trả lại SVG thô); test `TestF053Followups`.

### Session 2026-11-20 (owner chốt)

- Mọi OQ F024-1…12 giữ [Confirmed]. OQ F024-11 (benchmark T038) → **Chốt:** chạy cùng đợt đo tải F132. OQ F024-12 → đã nối binary 2026-11-20.
- OQ F054-2 (bộ lọc ngôn ngữ, `translation_status`) → **Chốt:** `searchQuestions` và danh sách question set/lesson/learning material/blueprint thêm tham số tùy chọn `content_language` (`ContentLanguage`) và `translation_status` (`TranslationStatus`), lọc trên version mới nhất; portal F054 thêm hai bộ lọc. Task bổ sung.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Question set tĩnh và theo luật (Priority: P1)

Biên tập viên ghép câu cố định hoặc khai báo luật chọn câu; editor hiện kích thước pool hiện tại để biết luật có đủ câu không.

**Why this priority**: Exercise, mini-game, quest, bài giao đều dùng question set (FR-CNT-001, FR-QB-004).

**Independent Test**: Tạo set tĩnh và set theo luật cho Toán 3; tham chiếu câu ngoài phạm vi; xem pool stats.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F024-AC-01 | KHI người có `learning_content.write` gọi `createCmsQuestionSet` với `Idempotency-Key`, HỆ THỐNG PHẢI kiểm `definition` theo `question-set.schema.json` (`static` cần `items`, `rule_based` cần `selection_rules`, `mixed` cần cả hai) và tạo version 1 `draft` | FR-CNT-001 · FR-QB-004 · FR-CMS-003 | — |
| F024-AC-02 | KHI lưu `items`, HỆ THỐNG PHẢI yêu cầu mọi `question_version_id` tồn tại trong phạm vi tenant (riêng hoặc platform được gán) và không trùng; set platform tham chiếu câu riêng tenant PHẢI bị `422 VALIDATION_FAILED`; câu platform chưa gán PHẢI trả `403 RESOURCE_NOT_ASSIGNED` | [CONTENT_MODEL §4](../../docs/02-learning/CONTENT_MODEL.md#4-phạm-vi-sở-hữu-owner_scope--adr-0005-adr-0029) · ADR-0029 | TC-CMS-ASG-01 |
| F024-AC-03 | NẾU một luật có tổng `distribution[].count` khác `count`, `difficulty.min > max`, hoặc quá 20 luật, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` chỉ rõ luật | FR-QB-004 | — |
| F024-AC-04 | KHI gọi `getQuestionSetPoolStats`, HỆ THỐNG PHẢI trả cho từng luật số câu khả dụng (version con trỏ hiện hành trong phạm vi tenant), cờ `insufficient` khi pool < `count` và cờ `warning` khi pool < 3 × `count` | FR-QB-004 · [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện) | — |
| F024-AC-05 | NẾU sửa version không ở `draft`, THÌ HỆ THỐNG PHẢI trả `409 CONTENT_NOT_EDITABLE`; `PATCH` thiếu `If-Match` → `428`, lệch → `412`; tạo version mới khi đã có bản nháp mở → `409 CONFLICT` | ADR-0015 · R11 | TC-CMS-002 |

---

### User Story 2 — Lesson, learning material và worked example (Priority: P1)

Biên tập viên soạn bài học gồm khối văn bản, media và ví dụ mẫu từng bước; câu hỏi tham chiếu ví dụ mẫu để học sinh xem lại sau khi sai.

**Why this priority**: Lesson là nội dung Learning Zone/LMS; worked example là hỗ trợ sư phạm P2 (FR-CNT-020).

**Independent Test**: Tạo lesson có `worked_example` hợp lệ/không hợp lệ; tạo learning material; gắn `worked_example_ref` từ câu hỏi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F024-AC-06 | KHI tạo/sửa lesson hoặc learning material, HỆ THỐNG PHẢI kiểm 1–100 khối thuộc loại cho phép (`rich_text`, `media`, `worked_example`; `activity` chỉ khi validator F030 đã đăng ký, nếu không `errors[].code = BLOCK_KIND_NOT_ENABLED`), `objective_ids` thuộc curriculum version của nội dung, số media ≤ `assets_per_version_max` | FR-CNT-001 · FR-CMS-003 · [CONTENT_MODEL §9](../../docs/02-learning/CONTENT_MODEL.md#9-định-dạng-rich-content) | — |
| F024-AC-07 | KHI lưu khối `worked_example`, HỆ THỐNG PHẢI yêu cầu `id` duy nhất trong version, `problem`, 1–8 `steps`, `objective_ids` không rỗng, `self_explain_prompt` tùy chọn, và từ chối mọi khối câu hỏi/activity lồng bên trong | FR-CNT-020 · [CONTENT_MODEL §9.2](../../docs/02-learning/CONTENT_MODEL.md#92-khối-worked_example-confirmed) | — |
| F024-AC-08 | KHI câu hỏi lưu `worked_example_ref`, HỆ THỐNG PHẢI kiểm khối tồn tại trong lesson/learning material cùng phạm vi và có ít nhất một LO chung với câu hỏi; không thỏa PHẢI trả `422 VALIDATION_FAILED` | FR-CNT-020 · FR-QB-013 | — |
| F024-AC-09 | HỆ THỐNG PHẢI cung cấp CRUD learning material (danh sách, tạo, xem, version mới, sửa nháp, xóa) cùng quy tắc version/phạm vi với lesson | FR-CNT-001 · FR-CMS-003 | — |

---

### User Story 3 — Assessment blueprint (ma trận đề) (Priority: P2)

Content Admin hoặc Academic Manager soạn ma trận LO × độ khó × loại câu × điểm; hệ thống kiểm tổng điểm và độ đủ của pool trước khi gửi duyệt.

**Why this priority**: Đầu vào bắt buộc của Exam Engine (F042, FR-QB-005).

**Independent Test**: Tạo blueprint "Kiểm tra giữa kỳ I – Toán 3" theo EXAM_ENGINE §3; sai tổng điểm; pool thiếu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F024-AC-10 | KHI lưu blueprint, HỆ THỐNG PHẢI kiểm `definition` theo `assessment-blueprint.schema.json`, tổng `count × points_each` bằng `total_points`, `feedback_timing` khác `immediate`, `fixed_question_version_ids` (nếu có) dài đúng `count` và không trùng câu giữa các luật, mọi `objective_ids`/`topic_ids` thuộc curriculum version; vi phạm PHẢI trả `422 VALIDATION_FAILED` | FR-QB-005 · [EXAM_ENGINE §3](../../docs/02-learning/EXAM_ENGINE.md#3-assessment-blueprint) | TC-EXM-001 |
| F024-AC-11 | KHI gọi `getAssessmentBlueprintPoolStats`, HỆ THỐNG PHẢI trả cho từng luật pool khả dụng và cờ thiếu khi pool < `count × variants.count` (`independent_draw`) hoặc < `count` (chiến lược khác) | FR-QB-005 · [EXAM_ENGINE §3](../../docs/02-learning/EXAM_ENGINE.md#3-assessment-blueprint) | — |
| F024-AC-12 | KHI blueprint không khai `variants.balance.max_mean_p_diff`, HỆ THỐNG PHẢI dùng giá trị cấu hình `variant_balance_max_mean_p_diff` | FR-QB-005 · [EXAM_ENGINE §4.1](../../docs/02-learning/EXAM_ENGINE.md#41-cân-bằng-độ-khó-giữa-các-biến-thể-confirmed) | — |

---

### User Story 4 — Import hàng loạt câu hỏi và nội dung (Priority: P2)

Đội nội dung nạp hàng trăm câu từ CSV/Excel/QTI-lite hoặc JSON theo `specs/schemas` vào trạng thái nháp, nhận báo cáo lỗi theo dòng.

**Why this priority**: Seed pilot trước khi có CMS UI (FR-CNT-007) và năng suất biên tập (FR-QB-011).

**Independent Test**: Import file 100 dòng có 5 dòng lỗi ở dry-run rồi thật.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F024-AC-13 | KHI người có `learning_content.import` gọi `createContentImport` với `kind` `questions_csv`, `questions_xlsx`, `questions_qti_lite` hoặc `content_json` và `Idempotency-Key`, HỆ THỐNG PHẢI tạo job `202`, kiểm từng dòng bằng đúng validator của API soạn thảo, tạo version `draft` cho dòng hợp lệ (không gì khi `dry_run`), và trả `errors[]` theo dòng qua `getContentImport` | FR-QB-011 · FR-CNT-007 | — |
| F024-AC-14 | NẾU dòng có khối công thức mà thiếu `math_assets` tương ứng, THÌ HỆ THỐNG PHẢI từ chối dòng đó với `code = MATH_RENDER_FAILED` | FR-QB-011 · ADR-0055 | TC-MATH-002 |
| F024-AC-15 | NẾU file vượt `csv_import_max`, sai định dạng, hoặc `kind = lessons_docx`, THÌ job PHẢI kết thúc `failed` (hoặc `422 VALIDATION_FAILED` với `IMPORT_KIND_NOT_SUPPORTED`) và không ghi dữ liệu | FR-QB-011 · NFR-MAINT-005 | — |
| F024-AC-16 | KHI job import hoàn tất, HỆ THỐNG PHẢI ghi audit (người gọi, số tạo, số từ chối) và nội dung tạo ra PHẢI thuộc tenant của phiên (hoặc platform với Content Admin) | R10 · FR-CNT-005 | — |

---

### User Story 5 — Tìm kiếm nội dung (Priority: P3)

Biên tập viên tìm "nhan vat" ra nội dung có "nhân vật", lọc thêm theo khối, môn, chủ đề, LO, độ khó, trạng thái.

**Why this priority**: FR-CNT-021 P3; cần khi ngân hàng lớn.

**Independent Test**: Nạp nội dung có dấu; tìm không dấu ở bốn loại; kiểm phạm vi tenant.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F024-AC-17 | KHI gọi `searchQuestions`, `listCmsQuestionSets`, `listCmsLessons`, `listCmsLearningMaterials`, `listCmsAssessmentBlueprints` với `q`, HỆ THỐNG PHẢI tìm full-text không phân biệt dấu và hoa thường trên tiêu đề/đề bài/văn bản khối, kết hợp với bộ lọc khối, môn, chủ đề, LO, độ khó, trạng thái | FR-CNT-021 · [CONTENT_MODEL §10](../../docs/02-learning/CONTENT_MODEL.md#10-yêu-cầu-chức-năng) | — |
| F024-AC-18 | HỆ THỐNG PHẢI chỉ trả kết quả trong phạm vi tenant (riêng + platform được gán), không gồm bản ghi đã xóa; System Admin dùng `tenant_ids` | AC-ORG-001 · AC-CMS-002 | TC-CMS-TEN-01 |
| F024-AC-19 | NẾU `q` dài hơn 200 ký tự, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` | NFR-MAINT-005 | TC-TEN-024 |

---

### User Story 6 — Phân quyền, Thùng rác, gán và clone cho các loại mới (Priority: P2)

Các loại nội dung mới tham gia cơ chế xóa mềm, gán và clone của F020; nhân sự tenant chỉ thấy dữ liệu được phép.

**Why this priority**: AC-CMS-005 (gán/clone question set) và AC-ORG-001.

**Independent Test**: Gán set platform cho tenant A; clone set của tenant A sang tenant C; xóa và khôi phục lesson.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F024-AC-20 | HỆ THỐNG PHẢI yêu cầu `learning_content.read` để đọc, `learning_content.write` để ghi, `learning_content.import` để import; thiếu PHẢI trả `403 PERMISSION_DENIED`; nội dung riêng tenant khác PHẢI trả `404 NOT_FOUND` ở mọi endpoint của feature | AC-ORG-001 · NFR-SEC-006 | TC-TEN-001 |
| F024-AC-21 | KHI System Admin gán question set platform cho tenant A, HỆ THỐNG PHẢI cho nhân sự tenant A thấy ở chế độ chỉ đọc và tenant B không thấy | AC-CMS-005 · FR-CMS-015 | TC-TEN-019, TC-CMS-ASG-02 |
| F024-AC-22 | KHI clone question set từ tenant A sang tenant C, HỆ THỐNG PHẢI sao sâu set và các câu hỏi riêng tenant A mà set tham chiếu (ID mới, `draft`, `cloned_from_*`), giữ nguyên tham chiếu câu platform, trả `id_map`, và không sao dữ liệu học sinh | AC-CMS-005 · R22 | TC-TEN-020, TC-CMS-CLN-01 |
| F024-AC-23 | KHI xóa question set, lesson, learning material hoặc blueprint chưa là con trỏ release, HỆ THỐNG PHẢI soft-delete, đưa vào Thùng rác và ghi audit; khôi phục PHẢI qua Thùng rác | FR-CMS-014 · ADR-0028 | TC-TEN-015 |

### Edge Cases

- NẾU set tĩnh tham chiếu version câu hỏi đã bị archive, THÌ vẫn lưu được ở nháp nhưng F026 chặn publish (tham chiếu con phải published) (F024-AC-02).
- NẾU set platform được gán tham chiếu câu platform chưa gán cho tenant, THÌ học sinh tenant vẫn làm được vì đi theo set đã gán [Confirmed] — quyền theo item gốc được gán (F024-AC-21).
- NẾU khối `worked_example` bị xóa khỏi version mới của lesson, THÌ câu hỏi tham chiếu version cũ vẫn trỏ đúng version cũ; lưu câu mới tham chiếu khối không còn → `422` (F024-AC-08).
- NẾU import có hai dòng trùng nội dung, THÌ cả hai được tạo (không khử trùng tự động) và báo cảnh báo theo `content_hash` (F024-AC-13).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CNT-001 | Lesson, Learning Material, Question Set (gồm Exercise), Assessment | [CONTENT_MODEL §2](../../docs/02-learning/CONTENT_MODEL.md#2-các-loại-nội-dung) |
| FR-CNT-007 | Import JSON/CSV theo `specs/schemas` cho seed | [CONTENT_MODEL §10](../../docs/02-learning/CONTENT_MODEL.md#10-yêu-cầu-chức-năng) |
| FR-CNT-020 | Khối `worked_example`, tham chiếu từ câu hỏi | [CONTENT_MODEL §9.2](../../docs/02-learning/CONTENT_MODEL.md#92-khối-worked_example-confirmed) |
| FR-CNT-021 | Tìm full-text không dấu + bộ lọc metadata | [CONTENT_MODEL §10](../../docs/02-learning/CONTENT_MODEL.md#10-yêu-cầu-chức-năng) |
| FR-QB-004 | Selection rules trong question set, pool stats | [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện) |
| FR-QB-005 | Soạn và kiểm blueprint (sinh đề ở F042) | [EXAM_ENGINE §3](../../docs/02-learning/EXAM_ENGINE.md#3-assessment-blueprint) |
| FR-QB-011 | Import Excel/CSV/QTI-lite vào Draft (Word/MathType ngoài phạm vi) | [QUESTION_BANK §10](../../docs/02-learning/QUESTION_BANK.md#10-yêu-cầu-chức-năng) |
| FR-CMS-003 | Backend Content Management cho lesson, material, question set, assessment, exercise | [ADMIN_PORTAL §3.2](../../docs/04-admin-portal/ADMIN_PORTAL.md#32-content-management-82) |

### Key Entities

- **QuestionSet / QuestionSetVersion**: tập câu tĩnh (`items` tham chiếu version câu bất biến) và/hoặc luật chọn; `purpose` (`exercise`, `minigame`, `quest`, `competition`, `general`).
- **Lesson / LessonVersion**, **LearningMaterial / LearningMaterialVersion**: chuỗi khối nội dung, LO, điều kiện hoàn thành.
- **WorkedExample**: khối ví dụ mẫu từng bước, định danh bằng `block_id` trong version.
- **AssessmentBlueprint / AssessmentBlueprintVersion**: ma trận đề (section, luật, điểm, biến thể, công bố kết quả).
- **PoolStats**: kích thước pool theo luật tại thời điểm hỏi.
- **ImportJob** (khung của F021): job nhập có báo cáo theo dòng.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `assets_per_version_max` | Số media mỗi version lesson/material |
| `csv_import_max` | Kích thước file import |
| `request_body_max` | Body API CMS |
| `variant_balance_max_mean_p_diff` | Mặc định `variants.balance.max_mean_p_diff` của blueprint |
| `idempotency_record_ttl` | Khóa idempotency cho tạo/import |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Version published bất biến; tham chiếu `question_version_id` bất biến | ADR-0015 · AGENTS R11 |
| Không đưa câu hỏi đề lên CDN (blueprint/đề chỉ qua API có quyền) | ADR-0043 · AGENTS R31 |
| Mọi chuỗi/mảng/khối JSON có giới hạn | NFR-MAINT-005 · ADR-0041 |
| Tenant isolation 100% endpoint | NFR-SEC-006 |
| Clone không mang dữ liệu học sinh | AGENTS R22 · ADR-0029 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Biên tập viên tạo set theo luật và biết ngay luật nào thiếu câu trước khi gửi duyệt (100% luật có pool stats).
- **SC-002**: Import 500 câu hợp lệ hoàn tất trong một job, 100% dòng lỗi có số dòng, trường và mã lỗi.
- **SC-003**: Tìm "nhan vat" trả 100% nội dung chứa "nhân vật" trong phạm vi tenant và 0 nội dung ngoài phạm vi.
- **SC-004**: Blueprint mẫu EXAM_ENGINE §3 lưu được; mọi biến thể sai tổng điểm bị chặn.

## Ngoài phạm vi

- Sinh `ExamPaper`, biến thể, exam session — F042. Item analysis/chất lượng đề — F044.
- Khối `activity` trong lesson, activity registry — F030. Learning path — F040.
- Workflow duyệt, audio `g1_3`, bản dịch — F025; publish/rollback/snapshot/kiểm tham chiếu con published — F026; media pipeline, kiểm SVG công thức, preview — F027.
- Import Word/MathType (`lessons_docx`) — công cụ theo [CONTENT_PRODUCTION_PLAN](../../docs/02-learning/CONTENT_PRODUCTION_PLAN.md), chưa gán feature.
- UI editor/browser/import — F053, F054.

## Assumptions

- Question set `purpose = exercise` là "Exercise" của FR-CNT-001, không có bảng riêng ([CONTENT_MODEL §2](../../docs/02-learning/CONTENT_MODEL.md#2-các-loại-nội-dung)).
- QTI-lite = tập con QTI 2.1 cho `choiceInteraction`, `textEntryInteraction`, `orderInteraction`, `matchInteraction` ánh xạ sang 8 loại cơ bản; định nghĩa đầy đủ ghi ở research.md [Confirmed].
- Tìm kiếm dùng PostgreSQL full-text + `unaccent` (API_DESIGN §5) [Confirmed].

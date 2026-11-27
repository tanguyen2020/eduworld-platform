# Feature Specification: F028 — Import Word/MathType (`lessons_docx`) qua Pandoc

**Feature Branch**: `028-docx-math-import` · **Created**: 2026-11-27 · **Status**: In progress · **Module**: `learning` + `src/cms/frontend/apps/portal` · **Mốc**: M-C3 · **Phụ thuộc**: F024, F027, F054
**Input**: Người soạn có sẵn câu hỏi trong Word với công thức Equation/MathType. Import `.docx` theo mẫu quy ước: tách câu, công thức OMML → LaTeX (Pandoc, image `eduworld/media-tools`) qua allowlist, chỉ tạo **Draft**, kèm báo cáo từng câu. FR nguồn: FR-QB-011 (phần Word/MathType), FR-CNT-017, FR-CNT-006. Quy tắc sở hữu: [CONTENT_PRODUCTION_PLAN §8](../../docs/02-learning/CONTENT_PRODUCTION_PLAN.md#8-import-wordmathtype-confirmed).

## Clarifications

### Session 2026-11-27 (owner chốt — lệnh "chốt hết", chọn phương án an toàn nhất)

- Q: Draft tạo ngay trong job hay qua portal? → A: Hai giai đoạn (research R5). Job `lessons_docx` chỉ phân tích và lưu báo cáo từng câu; portal render SVG công thức bằng MathJax 4 rồi tạo Draft từng câu qua `createDraftFromImportItem`, vì SVG chỉ render trên trình duyệt (ADR-0055 §4). Nội dung câu do server giữ, client chỉ gửi SVG của đúng các công thức server đã phân tích. → [Confirmed]
- Q: `dry_run` với `lessons_docx`? → A: Job luôn chỉ phân tích, không tạo câu hỏi; giá trị `dry_run` được lưu nhưng không đổi hành vi. → [Confirmed]
- Q: Đối tượng MathType OLE không có công thức Office Math? → A: Câu **lỗi** `FORMULA_NEEDS_REWRITE` ("cần soạn lại công thức"), không tạo Draft thiếu công thức; OOXML không mang MathML của OLE (research R3). → [Confirmed]
- Q: File có macro? → A: Từ chối cả file `MACRO_NOT_ALLOWED`. Liên kết ngoài: giữ chữ, bỏ link, cảnh báo. → [Confirmed]
- Q: Ảnh không có alt text hoặc định dạng ngoài PNG/JPEG/WebP? → A: Câu lỗi (`IMAGE_ALT_MISSING`, `IMAGE_TYPE_NOT_ALLOWED`); ảnh hợp lệ đi vào media pipeline F027 khi tạo Draft, giấy phép lấy từ file Word. → [Confirmed]
- Q: Loại câu hỗ trợ? → A: `single_choice` (một đáp án đúng) và `multiple_choice` (≥ 2 đáp án đúng, ≥ 3 phương án); loại khác → `UNSUPPORTED_QUESTION_FORMAT`, soạn trong editor F053. → [Confirmed]
- Q: Giới hạn? → A: File ≤ `csv_import_max` (giới hạn media `document` sẵn có); `docx_import_max_questions`, `docx_convert_timeout`, `docx_convert_memory`, `docx_uncompressed_max`, `docx_zip_entries_max` đăng ký ở CONFIG_DEFAULTS (scope code). → [Confirmed]
- Q: Pandoc chạy ở đâu? → A: Image `eduworld/media-tools` = stage `media-tools` của `deploy/docker/worker.Dockerfile` (ffmpeg + Pandoc); image `worker` dựng trên stage đó, job `import-jobs` profile `jobs` gọi `pandoc` như tiến trình ngoài với `--sandbox`. → [Confirmed]
- Q: Tiêu chí nghiệm thu "tỉ lệ chuyển đổi đúng trên bộ mẫu thật Toán/Lý/Hóa"? → A: ≥ 95% công thức đúng và render được, 0 câu sai đáp án; đo trên file thật trước khi mở cho người soạn (F028-AC-16, cần dữ liệu thật). → [Confirmed]

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Phân tích file Word thành báo cáo từng câu (Priority: P1)

Biên tập viên nội dung (`content_editor`, `tenant_content_editor`, quản lý chuyên môn có `learning_content.import`) tải lên file `.docx` soạn theo mẫu; hệ thống chuyển đổi, tách câu và cho biết từng câu nhập được, nhập được kèm cảnh báo, hay lỗi vì sao — trước khi tạo bất cứ thứ gì.

**Why this priority**: Không có báo cáo đáng tin thì không ai dám nhập hàng loạt; đây là giá trị cốt lõi và là điều kiện của US2.

**Independent Test**: Tải file mẫu 50 câu có công thức OMML + vài câu lỗi cố ý → job `succeeded`, 50 câu `ready`, câu lỗi có mã; không có câu hỏi nào được tạo.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F028-AC-01 | KHI người có `learning_content.import` gọi `createContentImport` với `kind = lessons_docx`, file `.docx` đã upload và `Idempotency-Key`, HỆ THỐNG PHẢI trả job `202`, và job worker PHẢI chuyển file bằng Pandoc như tiến trình ngoài (`--sandbox`, không mạng, timeout `docx_convert_timeout`, heap `docx_convert_memory`), tách câu theo mẫu và lưu cho mỗi câu một mục báo cáo (số thứ tự, nhãn "Câu n", trạng thái `ready`/`warning`/`error`, loại câu, trích đề, danh sách vấn đề có mã) mà không tạo câu hỏi hay version nào, kể cả khi `dry_run = false` | FR-QB-011 · CONTENT_PRODUCTION_PLAN §8 | — |
| F028-AC-02 | KHI phân tích một câu, HỆ THỐNG PHẢI đọc khối đầu file (`Môn`, `Khối`, `Chương trình`, `Phiên bản chương trình`, `Chủ đề`, `Ngôn ngữ`) và trong câu các nhãn `A.`…`H.`, đáp án in đậm hoặc dòng `Đáp án:`, `Gợi ý k:`, `Giải thích:`, `LO:`, `Độ khó:`, `Chủ đề:` (ghi đè), tạo nội dung `single_choice` khi có một đáp án đúng và `multiple_choice` khi có từ hai, và phân giải mã môn/curriculum/topic/LO trong phạm vi chủ file (`UNKNOWN_CODE` nếu không có) | FR-QB-011 · CONTENT_PRODUCTION_PLAN §8 | — |
| F028-AC-03 | KHI file có công thức Office Math (OMML), HỆ THỐNG PHẢI giữ LaTeX do Pandoc sinh (chuẩn hóa khoảng trắng), đặt công thức inline trong văn bản dạng `\( … \)` và công thức một dòng thành khối `math`, kiểm mỗi công thức bằng allowlist chung của ADR-0055 và đánh lỗi `MATH_NOT_ALLOWED` cho câu có công thức bị từ chối | FR-CNT-017 · ADR-0055 · CONTENT_MODEL §9.1 | TC-MATH-002 |
| F028-AC-04 | NẾU câu chứa đối tượng MathType/Equation OLE (không phải Office Math), THÌ HỆ THỐNG PHẢI đánh câu `error` với mã `FORMULA_NEEDS_REWRITE` và không cho tạo Draft của câu đó | CONTENT_PRODUCTION_PLAN §8 | — |
| F028-AC-05 | NẾU câu thiếu phương án/đáp án, đáp án mâu thuẫn hoặc trỏ phương án không có, thiếu metadata, có bảng, loại câu không hỗ trợ, ảnh thiếu alt hoặc sai định dạng, đoạn văn vượt độ dài khối, quá 5 gợi ý hoặc quá 50 công thức, THÌ HỆ THỐNG PHẢI đánh câu `error` với mã tương ứng; NẾU chỉ có liên kết ngoài, chú thích, danh sách tự đánh số hoặc chỉ số định dạng chữ, THÌ câu PHẢI là `warning` và vẫn nhập được (link bị bỏ, giữ chữ) | FR-QB-011 · CONTENT_PRODUCTION_PLAN §8 bước 5 | — |
| F028-AC-06 | NẾU file vượt `csv_import_max`, không phải `.docx` hợp lệ, có macro, vượt `docx_zip_entries_max`/`docx_uncompressed_max`, không có câu nào, vượt `docx_import_max_questions` hoặc chuyển đổi quá `docx_convert_timeout`, THÌ job PHẢI kết thúc `failed` với mã lỗi cấp file trong `errors[]` và không lưu mục câu nào | FR-QB-011 · CONTENT_PRODUCTION_PLAN §8 bước 5 | — |
| F028-AC-07 | KHI job `lessons_docx` kết thúc, HỆ THỐNG PHẢI đặt `succeeded` khi có ít nhất một câu nhập được (ngược lại `failed`), `created_count` = số câu nhập được, `rejected_count` = số câu lỗi, `errors[]` theo số thứ tự câu, `result.items_total`/`items_importable`, và ghi audit `content.import_requested`, `content.imported` (người gọi, số câu) — không ghi nội dung câu/công thức vào log | FR-QB-011 · R10 · R14 | — |
| F028-AC-08 | KHI người có `learning_content.import` gọi `listContentImportItems`, HỆ THỐNG PHẢI trả các mục theo số thứ tự, phân trang keyset (`limit` ≤ 100, `cursor`), lọc được theo `status`, mỗi mục kèm công thức (LaTeX, inline/khối), ảnh (tham chiếu, alt) và nội dung đã phân tích để xem trước | FR-QB-011 · API_DESIGN §5 | — |

---

### User Story 2 — Tạo Draft từ từng câu đã phân tích (Priority: P1)

Sau khi xem báo cáo, người soạn bấm tạo; mỗi câu nhập được thành câu hỏi version 1 ở trạng thái Draft, công thức có SVG, ảnh qua media pipeline; báo cáo cập nhật câu nào đã tạo, câu nào bị validator từ chối và vì sao. Không bao giờ tự publish.

**Why this priority**: Kết quả cuối của import; không có thì báo cáo vô dụng.

**Independent Test**: Với job của US1, tạo Draft cho mọi câu `ready` → đúng số version `draft` mới thuộc tenant, câu `created` có `question_id`; gọi lại cùng `Idempotency-Key` trả cùng kết quả; câu `error` → `409`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F028-AC-09 | KHI người có `learning_content.import` gọi `createDraftFromImportItem` với `Idempotency-Key` và `math_assets` cho một câu `ready`/`warning`/`draft_failed` của job `lessons_docx` `succeeded`, HỆ THỐNG PHẢI tạo câu hỏi version 1 trạng thái `draft` (không bao giờ published) từ nội dung server đã phân tích + SVG gửi kèm, kiểm bằng đúng validator của `createQuestion`, đặt câu `created` kèm `question_id`/`question_version_id`, ghi audit `content.version_created` (nguồn import, `import_id`, số câu); gọi lại cùng khóa PHẢI trả lại response đã lưu | FR-QB-011 · R10 · R11 · ADR-0015 | — |
| F028-AC-10 | NẾU `math_assets` thiếu SVG cho một công thức của câu, có công thức không thuộc câu, hoặc SVG/LaTeX không qua kiểm tra, THÌ HỆ THỐNG PHẢI trả `422 MATH_RENDER_FAILED` và không tạo gì | FR-CNT-017 · ADR-0055 §4 | TC-MATH-002 |
| F028-AC-11 | NẾU câu ở trạng thái `error`, đã `created`, hoặc job không phải `lessons_docx` đã `succeeded`, THÌ HỆ THỐNG PHẢI trả `409 CONFLICT` (`errors[].code` = `IMPORT_ITEM_NOT_IMPORTABLE` hoặc `IMPORT_ITEM_ALREADY_CREATED`) và không ghi dữ liệu | R10 · FR-QB-011 | — |
| F028-AC-12 | NẾU validator của `createQuestion` từ chối nội dung câu, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` với `errors[]`, đặt câu `draft_failed` kèm vấn đề, và cho phép tạo lại | FR-QB-011 · F024-AC-13 | — |
| F028-AC-13 | KHI câu có ảnh nhúng hợp lệ, HỆ THỐNG PHẢI đưa ảnh vào media pipeline (object quarantine, asset `processing`, event `media.uploaded`, dùng lại asset trùng sha256 cùng phạm vi, `license` của file Word) và Draft PHẢI tham chiếu asset đó qua `media_ref` | FR-CNT-006 · ADR-0044 · CONTENT_PRODUCTION_PLAN §8 bước 2 | — |
| F028-AC-14 | HỆ THỐNG PHẢI yêu cầu `learning_content.import` cho `createContentImport` (`lessons_docx`), `listContentImportItems`, `createDraftFromImportItem`; thiếu PHẢI trả `403 PERMISSION_DENIED`; job/câu của tenant khác PHẢI trả `404 NOT_FOUND`; Draft PHẢI thuộc chủ của job (tenant phiên hoặc platform) | AC-ORG-001 · NFR-SEC-006 · R7 · R8 | TC-TEN-001 |

---

### User Story 3 — Màn Import Word trên Admin Portal (Priority: P2)

Trong Workspace B, màn Import có thêm nguồn "Word (.docx)": chọn file, nhập giấy phép, chờ phân tích, xem bảng báo cáo từng câu (phân trang, lọc trạng thái), bấm tạo Draft cho các câu nhập được và theo dõi tiến độ, kết quả từng câu.

**Why this priority**: Người soạn dùng qua portal; API đã kiểm độc lập ở US1–US2.

**Independent Test**: Vitest với backend MOCK hợp đồng: tải file → job → bảng 2 trang → tạo Draft → câu `created`, câu lỗi render công thức báo lỗi không gọi API.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F028-AC-15 | KHI người có action `action.content_studio.question_bank.import` chọn nguồn Word, HỆ THỐNG PHẢI tải `.docx` (≤ `csv_import_max`, `license`) qua media upload, chờ `ready`, tạo job `lessons_docx`, hiển thị tổng kết và bảng câu phân trang phía server lọc theo trạng thái, render công thức bằng MathJax 4 và tạo Draft lần lượt từng câu nhập được (mỗi câu một `Idempotency-Key`, thanh tiến độ, kết quả từng câu); câu render công thức lỗi PHẢI được báo mà không gọi API | FR-QB-011 · FR-CMS-003 · ADMIN_PORTAL §4 · R32 | — |
| F028-AC-16 | KHI đo trên bộ file Word thật Toán/Lý/Hóa do EdTech cung cấp trước khi mở cho người soạn, HỆ THỐNG PHẢI chuyển đúng ngữ nghĩa và render được ≥ 95% công thức OMML và không nhập sai đáp án câu nào | CONTENT_PRODUCTION_PLAN §8 · AC-CPP-002 | — |

### Edge Cases

- NẾU hai câu cùng nhãn "Câu 3", THÌ cả hai được giữ theo số thứ tự xuất hiện, nhãn giữ nguyên (F028-AC-01).
- NẾU đoạn trước câu đầu tiên không phải nhãn khối đầu, THÌ bỏ qua (tiêu đề tài liệu).
- NẾU một công thức xuất hiện nhiều lần trong câu, THÌ chỉ cần một SVG (F028-AC-10).
- NẾU người soạn gọi tạo Draft cho câu `draft_failed` sau khi sửa không gì (cùng nội dung), THÌ validator trả lại cùng lỗi (F028-AC-12).
- NẾU công cụ chuyển đổi không có trên worker, THÌ job `failed` với `CONVERTER_UNAVAILABLE` (lỗi vận hành nhìn thấy được).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-QB-011 | Import Word/MathType vào Draft kèm báo cáo từng câu | [CONTENT_PRODUCTION_PLAN §8](../../docs/02-learning/CONTENT_PRODUCTION_PLAN.md#8-import-wordmathtype-confirmed), [QUESTION_BANK §10](../../docs/02-learning/QUESTION_BANK.md#10-yêu-cầu-chức-năng) |
| FR-CNT-017 | Công thức từ Word qua allowlist, SVG render trên portal | [CONTENT_MODEL §9.1](../../docs/02-learning/CONTENT_MODEL.md#91-công-thức-toán-lý-hóa-adr-0055) |
| FR-CNT-006 | Ảnh nhúng đi qua media pipeline | [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md#7-media-asset-pipeline-adr-0044) |

### Key Entities

- **ImportJob** (F021/F024): job `lessons_docx` với báo cáo tổng.
- **ImportItem**: một câu của file Word — số thứ tự, nhãn, trạng thái, loại câu, trích đề, vấn đề, công thức, ảnh, nội dung đã phân tích, câu hỏi đã tạo.
- **Question/QuestionVersion** (F022): Draft được tạo.
- **MediaAsset** (F027): ảnh nhúng.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `csv_import_max` | Kích thước tối đa file `.docx` |
| `docx_import_max_questions` | Số câu tối đa một file |
| `docx_convert_timeout` | Thời gian tối đa một lần chạy Pandoc |
| `docx_convert_memory` | Heap tối đa của Pandoc |
| `docx_uncompressed_max`, `docx_zip_entries_max` | Chống zip bomb |
| `latex_max_length`, `latex_max_depth`, `math_svg_max` | Allowlist công thức (F027) |
| `idempotency_record_ttl` | Khóa idempotency |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chỉ Draft, không sửa nội dung Published | R11 · ADR-0015 |
| Idempotency + audit cho tạo job, tạo Draft | R10 |
| Lọc tenant, chủ job | R8 · ADR-0005 |
| `authz.Require` mỗi use case | R7 |
| Không log nội dung câu/công thức | R14 |
| File chỉ ở object storage, không lưu blob trong DB | R31 · ADR-0044 |
| Pandoc là tiến trình ngoài có timeout, không link vào Go | CONTENT_PRODUCTION_PLAN §8 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: File mẫu 50 câu có công thức OMML → 50 Draft có SVG, câu lỗi có báo cáo, 0 version published (AC-CPP-002).
- **SC-002**: 100% câu lỗi có mã lỗi và nhãn câu; người soạn biết sửa gì mà không cần đọc log.
- **SC-003**: Phân tích file 200 câu xong trong một lượt job (≤ `docx_convert_timeout` cho bước Pandoc).

## Ngoài phạm vi

- Loại câu khác `single_choice`/`multiple_choice`, import lesson/learning material từ Word — soạn trong editor F053.
- Xuất ngược ra Word; sửa nội dung câu trên màn import (sửa trong editor F053 sau khi tạo Draft).
- Phát hiện trùng nội dung tự động (giống F024).

## Assumptions

- Bộ mẫu thật Toán/Lý/Hóa do EdTech cung cấp ngoài repo (bản quyền); đo theo quickstart §2.6 (F028-AC-16).
- Media `document` nhận thêm MIME `.docx`; file Word quét virus như mọi media (F027).

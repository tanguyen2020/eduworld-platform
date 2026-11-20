# Feature Specification: F022 — Question Bank 8 loại cơ bản: validator, scorer server-side, render payload không đáp án

**Feature Branch**: `022-question-bank-core` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C2 · **Phụ thuộc**: F021
**Input**: Ngân hàng câu hỏi tập trung cho 8 loại cơ bản (`single_choice`, `multiple_choice`, `true_false`, `fill_in_blank`, `numeric`, `matching`, `ordering`, `drag_drop`): soạn và kiểm hợp lệ, chấm phía server có partial credit, dựng render payload không lộ đáp án, chọn câu ngẫu nhiên theo điều kiện. FR nguồn: FR-QB-001…004, FR-QB-006, FR-QB-007, FR-QB-013, FR-QB-015.

## Clarifications

### Session 2026-11-16

- Q: `options[].feedback` lưu ở đâu khi schema đặt nó trong `render_payload.options[]`? → A: Nhận ở dạng soạn theo `question.schema.json`, khi lưu server tách `feedback` và `misconception_tag` sang phần chỉ server; cột render chỉ còn dữ liệu hiển thị ([QUESTION_BANK §2](../../docs/02-learning/QUESTION_BANK.md#2-cấu-trúc-một-question-version), DATABASE_DESIGN §3.3 "render_payload KHÔNG có đáp án").
- Q: Khối lớp của câu hỏi là một giá trị hay mảng? → A: Mảng `grade_levels` theo tài liệu sở hữu metadata [CONTENT_MODEL §3](../../docs/02-learning/CONTENT_MODEL.md#3-metadata-bắt-buộc-confirmed--master-prompt-42) và `question.schema.json`; DATABASE_DESIGN §3.3 và `QuestionAuthoringInput.grade_level` sửa theo (delta).
- Q: "Tránh câu đã làm gần đây" cần lịch sử attempt chưa có ở F022? → A: `Select` nhận danh sách `question_id` cần loại do bên gọi tính (F030 đọc `attempt_items`); F022 không đọc bảng attempt.
- Q: Ba loại `short_answer`, `image_based`, `interactive`? → A: F023. Trước khi F023 xong, tạo câu loại này bị từ chối rõ ràng.

### Session 2026-11-19 (triển khai)

- OQ F022-1: `numeric.Parse` dùng `math/big.Rat` (thư viện chuẩn, số thập phân chính xác) thay `cockroachdb/apd` của plan T022 — `apd` chưa có trong `go.mod` và không cần ngoại lệ depguard mới. → [Confirmed] giữ `big.Rat`; đổi sang `apd` chỉ khi cần phép toán thập phân có context (làm tròn) ở F023.
- OQ F022-2: Bố cục package khác plan: các `TypeHandler` 8 loại, render và shuffle nằm chung package `internal/domain/question` (`types.go`, `registry.go`) thay cho `types/`, `render/`; fixture 8 ví dụ ở `internal/domain/question/testdata/qb_examples/`; test tích hợp là các file `tests/integration/question_*_test.go` trong package `integration` dùng chung môi trường (không tách thư mục `question/`). → [Confirmed] giữ; không đổi hành vi.
- OQ F022-3: `question_version_objectives` có khóa thay thế `question_version_objective_id` + cột `question_id`, `tenant_id`, `owner_scope` (bảng H: RLS cần `question_id` để kiểm assignment) thay cho PK ghép của DATABASE_DESIGN §2.5 (đã cập nhật). → [Confirmed].
- OQ F022-4: Render `ordering` (không bao giờ hiện đúng thứ tự) và `multiple_choice` có `show_correct_count` cần đáp án: `Delivery.Render` đọc `answer_key` phía server qua `DeliveryReader.Grading`, không bao giờ đưa vào DTO (contract test `TestRenderDTOHasNoAnswerData`). → [Confirmed].
- OQ F022-5: `grade_levels`, `tags`, `media_refs` lưu `jsonb` (ent JSON) thay `smallint[]`/`uuid[]`; CHECK `jsonb_typeof` + giới hạn phần tử, GIN `jsonb_path_ops`. → [Confirmed] (DATABASE_DESIGN §2.5/§3.3 đã ghi).
- OQ F022-6: `ETag` của `getCmsQuestion` là row version của **câu hỏi** (dùng cho `If-Match` của `deleteQuestion`); `ETag` của version lấy từ `getCmsQuestionVersion`/`createQuestion`/`updateQuestionDraft`/`createQuestionVersion`. → [Confirmed].
- OQ F022-7: View `content.question_delivery_v` chạy quyền owner + FORCE RLS của bảng gốc nên chỉ phục vụ phạm vi tenant (học sinh); `Delivery` từ chối khi không có đúng một session tenant. → [Confirmed].
- OQ F022-8: Consumer `learning.qb_pool_invalidation` và port `Delivery` chưa nối vào binary `core-api`/`worker` (cùng OQ F020-3); chưa có producer `content.published` cho câu hỏi tới F026 — test gọi `Handle` trực tiếp. → [Confirmed] nối khi F026/F030 dùng.
- OQ F022-9: `assets_per_version_max` (CONFIG_DEFAULTS, phạm vi platform, [Confirmed] 50) dùng hằng `command.AssetsPerVersionMax = 50` vì module learning chưa đọc platform settings (sở hữu identity). → [Confirmed] đọc từ settings khi có facade.
- OQ F022-10: `worked_example_ref` là object `{content_item_id, block_id}` (DATABASE_DESIGN §2.5); chưa kiểm tồn tại block vì worked example thuộc F024. → [Confirmed].
- OQ F022-11: Tham số `q` (tìm toàn văn) của `searchQuestions` được nhận nhưng bỏ qua — tìm kiếm toàn văn thuộc F024. → [Confirmed].
- OQ F022-12: Cache answer key `ew:learning:key:*` (T024) chưa làm: `Delivery.Grade` đọc version published (bất biến) từ PostgreSQL mỗi lần; benchmark `BenchmarkGrade` chỉ đo phần domain. → [Confirmed] thêm cache khi F030 đo p95 chấm.
- OQ F022-13: Luồng publish (`status = published` + `current_version_id`) thuộc F025/F026; test tích hợp đặt trạng thái bằng SQL dưới role `learning_platform` (helper `publishQuestion`, gắn nhãn TEST). → [Confirmed].
- OQ F022-14: Pool cache Valkey `ew:learning:qb:pool:<tenant_id>:<hash(subject_id)>` (TTL 5 phút) giữ toàn bộ câu hiện hành của một môn; bộ lọc còn lại áp trong bộ nhớ bởi `selection.Select`. → [Confirmed].

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Soạn câu hỏi 8 loại có kiểm hợp lệ (Priority: P1)

Biên tập viên (`cm.noidung` platform, `ad.truong.a` tenant) tạo câu hỏi, sửa nháp, tạo version mới từ câu đã publish; hệ thống chặn câu sai cấu trúc trước khi lưu. US-CMS-001.

**Why this priority**: Không có câu hỏi hợp lệ thì không có học tập, mini-game hay thi đấu.

**Independent Test**: Gọi API CMS tạo một câu mỗi loại từ ví dụ QUESTION_BANK §3; gửi các biến thể sai; kiểm quyền và cô lập tenant.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F022-AC-01 | KHI người có `learning_content.write` gọi `createQuestion` với một trong 8 loại cơ bản và `Idempotency-Key`, HỆ THỐNG PHẢI kiểm theo `question.schema.json` và validator ngữ nghĩa của loại, rồi tạo câu hỏi với version 1 ở `draft` và trả `ETag` | FR-QB-001 · FR-QB-002 · US-CMS-001 | — |
| F022-AC-02 | NẾU `type` là `short_answer`, `image_based` hoặc `interactive` khi validator loại đó chưa được đăng ký, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` với `errors[].code = TYPE_NOT_ENABLED` | FR-QB-001 | — |
| F022-AC-03 | NẾU ID phương án/mục/vùng/chỗ trống trùng nhau, sai mẫu `itemId`, hoặc `answer_key` tham chiếu ID không tồn tại (gồm: `single_choice` không đúng một đáp án, `ordering` ít hơn 3 mục, chỗ trống trong đề thiếu đáp án, `numeric` có giá trị không phải số thập phân), THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` kèm JSON pointer từng lỗi | FR-QB-002 · [QUESTION_BANK §2–§3](../../docs/02-learning/QUESTION_BANK.md#3-mười-một-dạng-câu-hỏi) | TC-LRN-016 |
| F022-AC-04 | KHI lưu `scoring_rule`, HỆ THỐNG PHẢI chỉ chấp nhận `partial_method` hợp lệ cho loại câu (vd. `adjacent_pairs`/`position` chỉ cho `ordering`) và `max_score` lớn hơn 0 | FR-QB-015 | — |
| F022-AC-05 | KHI lưu nội dung đề, phương án, giải thích, gợi ý, HỆ THỐNG PHẢI chuẩn hóa Unicode NFC, chỉ nhận khối nội dung được phép cho câu hỏi (không có `worked_example`), nhận `math_assets` cho khối công thức và giới hạn số media theo `assets_per_version_max` | FR-QB-007 · [CONTENT_MODEL §9](../../docs/02-learning/CONTENT_MODEL.md#9-định-dạng-rich-content) | — |
| F022-AC-06 | KHI lưu `options[].feedback`, `options[].misconception_tag`, `hints[]` (tối đa 5), `explanation`, `min_response_ms`, `worked_example_ref`, HỆ THỐNG PHẢI lưu chúng ở phần chỉ server, tách khỏi dữ liệu render | FR-QB-013 · FR-QB-002 · ADR-0016 | TC-LRN-012 |
| F022-AC-07 | NẾU sửa version không ở `draft`, THÌ HỆ THỐNG PHẢI trả `409 CONTENT_NOT_EDITABLE`; KHI gọi `createQuestionVersion` từ một version, HỆ THỐNG PHẢI tạo version `draft` mới (`based_on_version_id`) và từ chối nếu đã có version `draft`/`in_review` mở (`409 CONFLICT`) | ADR-0015 · R11 | TC-CMS-002 |
| F022-AC-08 | NẾU `updateQuestionDraft` thiếu `If-Match`, THÌ HỆ THỐNG PHẢI trả `428 PRECONDITION_REQUIRED`; NẾU lệch, THÌ PHẢI trả `412 VERSION_MISMATCH` | [API_DESIGN §7](../../docs/07-backend/API_DESIGN.md#7-optimistic-concurrency-cmslms) | — |
| F022-AC-09 | NẾU người dùng thiếu `learning_content.write`, hoặc người dùng tenant ghi câu hỏi platform (kể cả được gán), THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED`; câu hỏi riêng tenant khác PHẢI trả `404 NOT_FOUND` | AC-ORG-001 · AC-CMS-005 | TC-TEN-001 |
| F022-AC-10 | KHI `getCmsQuestion`/`getCmsQuestionVersion` được gọi bởi người có `learning_content.read` trong phạm vi, HỆ THỐNG PHẢI trả đầy đủ `answer_key`, phản hồi phương án, gợi ý; mọi endpoint khác ngoài CMS PHẢI không trả các trường này | FR-QB-003 · [API_DESIGN §9.13](../../docs/07-backend/API_DESIGN.md#913-cms-nội-dung-học-tập--apiv1cmslearning) | — |
| F022-AC-11 | KHI `deleteQuestion` được gọi cho câu chưa là con trỏ release, HỆ THỐNG PHẢI soft-delete, đưa vào Thùng rác, ghi audit; attempt đã dùng version của câu đó PHẢI vẫn xem lại được | AC-CMS-004 · FR-CMS-014 | TC-TEN-015, TC-CMS-DEL-01 |
| F022-AC-12 | NẾU chuỗi hoặc khối JSON vượt giới hạn (vd. đáp án chấp nhận > 255 ký tự, body > `request_body_max`), THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` chỉ rõ field | NFR-MAINT-005 · ADR-0041 | TC-TEN-024 |
| F022-AC-13 | KHI gọi `searchQuestions` với bộ lọc môn, khối, chủ đề, LO, độ khó, trạng thái, loại, nguồn (`own`/`assigned`/`platform`), HỆ THỐNG PHẢI trả đúng kết quả trong phạm vi tenant (System Admin dùng `tenant_ids`) | FR-QB-002 · FR-CNT-005 | TC-CMS-TEN-01 |

---

### User Story 2 — Chấm câu trả lời phía server (Priority: P1)

Mọi nơi dùng câu hỏi (attempt, mini-game, battle) gọi một bộ chấm duy nhất; điểm chỉ do server tính.

**Why this priority**: Nguyên tắc server-authoritative (ADR-0016) và là lõi của F030.

**Independent Test**: Bảng test (response → điểm) cho 8 loại, gồm biên và response sai kiểu; property test shuffle.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F022-AC-14 | HỆ THỐNG PHẢI chấm 8 loại chỉ từ `answer_key`, `scoring_rule`, `presentation` và response, trả điểm trong `[min_score, max_score]`, `is_correct` khi đạt điểm tối đa và kết quả theo từng phần | FR-QB-001 · ADR-0016 | TC-LRN-003 |
| F022-AC-15 | KHI chấm `multiple_choice`, HỆ THỐNG PHẢI áp đúng công thức `all_or_nothing`, `right_minus_wrong`, `proportional` ở QUESTION_BANK §3.2 | FR-QB-015 · [QUESTION_BANK §3.2](../../docs/02-learning/QUESTION_BANK.md#32-multiple_choice--nhiều-lựa-chọn-đúng) | TC-LRN-003 |
| F022-AC-16 | KHI chấm `ordering`, `matching`, `drag_drop`, `true_false` nhiều mệnh đề, `fill_in_blank`, HỆ THỐNG PHẢI áp `position`/`adjacent_pairs`/`proportional` theo định nghĩa; draggable mồi đặt vào vùng bất kỳ PHẢI tính sai | FR-QB-015 · [QUESTION_BANK §3.8–§3.9](../../docs/02-learning/QUESTION_BANK.md#38-ordering--sắp-xếp-thứ-tự) | TC-LRN-003 |
| F022-AC-17 | KHI so khớp `fill_in_blank` với `match = normalized`, HỆ THỐNG PHẢI áp đủ 8 bước chuẩn hóa tiếng Việt (NFC, khoảng trắng, chữ thường, dấu câu đầu/cuối, vị trí dấu thanh cũ/mới, `allow_diacritics_missing`) và quy tắc riêng cho `content_language = en`; `match = exact` chỉ bước 1–2; bộ ≥ 200 cặp test tiếng Việt PHẢI đạt | FR-QB-006 · [QUESTION_BANK §4](../../docs/02-learning/QUESTION_BANK.md#4-chuẩn-hóa-văn-bản-tiếng-việt-match--normalized) | TC-LRN-003 |
| F022-AC-18 | KHI chấm `numeric`, HỆ THỐNG PHẢI parse số thập phân không dùng float, nhận `,` và `.` theo `content_language`, nhận dạng tương đương (`25/2`, `12,50`) khi `accept_equivalent_forms`, và áp dung sai `exact`/`absolute`/`relative` | FR-QB-006 · [QUESTION_BANK §3.6](../../docs/02-learning/QUESTION_BANK.md#36-numeric--đáp-số) | TC-LRN-003 |
| F022-AC-19 | NẾU response không khớp `$defs.<type>Response` (rỗng, ID lạ, trùng, sai kiểu, thừa trường), THÌ HỆ THỐNG PHẢI trả lỗi `INVALID_RESPONSE` (HTTP `422` ở API dùng bộ chấm) và không bao giờ `500` | AC-EXR-002 · [QUESTION_BANK §11](../../docs/02-learning/QUESTION_BANK.md#11-kiểm-thử) | TC-LRN-016 |
| F022-AC-20 | HỆ THỐNG PHẢI cho cùng điểm với mọi thứ tự hiển thị (shuffle) của cùng response | FR-QB-003 | TC-LRN-003 |
| F022-AC-21 | HỆ THỐNG PHẢI không nhận `score`, `is_correct` hay đáp án do client gửi làm đầu vào chấm | AC-EXR-002 · R5 | TC-LRN-007 |

---

### User Story 3 — Render payload không đáp án (Priority: P1)

Game và preview nhận câu hỏi đã trộn thứ tự, không có bất kỳ thông tin suy ra đáp án.

**Why this priority**: FR-QB-003/AC-EXR-001 là điều kiện bắt buộc trước khi học sinh chạm vào câu hỏi.

**Independent Test**: Contract test quét DTO render của mọi loại; snapshot test shuffle theo seed.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F022-AC-22 | HỆ THỐNG PHẢI dựng DTO render bằng danh sách trắng trường theo loại; DTO PHẢI không chứa `answer_key`, `scoring_rule`, `explanation`, `options[].feedback`, `options[].misconception_tag`, `hints`, `min_response_ms`, `accepted*`, `correct_option_id(s)` ở bất kỳ loại nào | FR-QB-003 · AC-EXR-001 · AC-EXR-007 | TC-LRN-007, TC-LRN-012 |
| F022-AC-23 | KHI dựng render với một seed, HỆ THỐNG PHẢI trộn phương án/mục phía server một cách tái lập, giữ nguyên ID, và với `ordering` thứ tự hiển thị PHẢI khác thứ tự đúng | FR-QB-003 · [QUESTION_BANK §8](../../docs/02-learning/QUESTION_BANK.md#8-chống-lộ-đáp-án) | TC-LRN-007 |
| F022-AC-24 | HỆ THỐNG PHẢI đọc dữ liệu giao cho học sinh qua view không có cột đáp án (`content.question_delivery_v`) và chỉ đọc `answer_key` trong bộ chấm | ADR-0016 · R6 | TC-LRN-007 |
| F022-AC-25 | KHI version có công thức, DTO render PHẢI dùng `math_ref` + `speech_text` thay cho LaTeX để client game không cần bộ dựng công thức | FR-QB-007 · ADR-0055 | TC-MATH-003 |

---

### User Story 4 — Chọn câu ngẫu nhiên theo điều kiện (Priority: P2)

Practice, mini-game, question set theo luật lấy câu từ ngân hàng theo khối/chủ đề/LO/độ khó, tái lập theo seed.

**Why this priority**: FR-QB-004 P2, cần cho F024 (question set theo luật) và F030.

**Independent Test**: Nạp pool published có con trỏ, gọi `Select` với nhiều luật và seed; pool thiếu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F022-AC-26 | KHI `Select` được gọi với phạm vi tenant, luật chọn, seed và danh sách loại trừ, HỆ THỐNG PHẢI chỉ chọn version là con trỏ hiện hành, nằm trong phạm vi (riêng tenant hoặc platform được gán), khớp bộ lọc, không `exam_only` trừ khi luật cho phép, theo `distribution`; cùng đầu vào PHẢI cho cùng kết quả | FR-QB-004 · [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện) | — |
| F022-AC-27 | NẾU pool không đủ, THÌ HỆ THỐNG PHẢI nới theo `relaxation` (`avoid_recent` → độ khó ±1 → `types`) và vẫn thiếu thì trả `QUESTION_POOL_INSUFFICIENT` | FR-QB-004 | — |
| F022-AC-28 | KHI một câu hỏi được publish, rollback, unpublish, hoặc tài nguyên được gán/gỡ gán, HỆ THỐNG PHẢI xóa cache pool liên quan của tenant | [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện) · ADR-0043 | — |

### Edge Cases

- NẾU `multiple_choice` có `show_correct_count = true`, THÌ render được kèm số đáp án đúng (thông tin hiển thị cho phép, F022-AC-22).
- NẾU câu `drag_drop` có zone `capacity` và response vượt sức chứa, THÌ phần vượt tính sai (F022-AC-16).
- NẾU response `numeric` mơ hồ (`1.234`), THÌ hiểu theo `content_language` của câu (F022-AC-18).
- NẾU câu platform được gán bị gỡ gán, THÌ `Select` không còn chọn câu đó cho tenant (F022-AC-26, F022-AC-28).
- NẾU câu có bản dịch nhưng đáp án chữ chưa khai cho ngôn ngữ dịch, THÌ chỉ phát ở ngôn ngữ gốc ([QUESTION_BANK §4](../../docs/02-learning/QUESTION_BANK.md#4-chuẩn-hóa-văn-bản-tiếng-việt-match--normalized)).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-QB-001 | 8 loại cơ bản (3 loại còn lại F023) | [QUESTION_BANK §3](../../docs/02-learning/QUESTION_BANK.md#3-mười-một-dạng-câu-hỏi) |
| FR-QB-002 | Cấu trúc question version, soạn/sửa/version mới | [QUESTION_BANK §2](../../docs/02-learning/QUESTION_BANK.md#2-cấu-trúc-một-question-version) |
| FR-QB-003 | Render DTO danh sách trắng, view delivery | [QUESTION_BANK §8](../../docs/02-learning/QUESTION_BANK.md#8-chống-lộ-đáp-án) |
| FR-QB-004 | `Select` theo `SelectionRule`, seed, nới lỏng, cache pool | [QUESTION_BANK §6](../../docs/02-learning/QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện) |
| FR-QB-006 | Chuẩn hóa tiếng Việt cho `fill_in_blank`, dung sai `numeric` (`short_answer` ở F023) | [QUESTION_BANK §4](../../docs/02-learning/QUESTION_BANK.md#4-chuẩn-hóa-văn-bản-tiếng-việt-match--normalized) |
| FR-QB-007 | Rich content tiếng Việt có dấu, khối công thức/ảnh/audio | [CONTENT_MODEL §9](../../docs/02-learning/CONTENT_MODEL.md#9-định-dạng-rich-content) |
| FR-QB-013 | Lưu phản hồi phương án, gợi ý, tham chiếu worked example phía server | [QUESTION_BANK §2](../../docs/02-learning/QUESTION_BANK.md#2-cấu-trúc-một-question-version) |
| FR-QB-015 | Partial credit theo `scoring_rule` | [QUESTION_BANK §3](../../docs/02-learning/QUESTION_BANK.md#3-mười-một-dạng-câu-hỏi) |

### Key Entities

- **Question**: định danh câu hỏi (phạm vi sở hữu, con trỏ version hiện hành).
- **QuestionVersion**: loại, đề, dữ liệu hiển thị, đáp án, quy tắc chấm, giải thích, gợi ý, phản hồi phương án, metadata; bất biến sau publish.
- **Presentation**: cách server đã hiển thị câu cho một lượt (thứ tự sau trộn; tham số ở F023).
- **GradeResult**: điểm, đúng/sai, kết quả từng phần, ID phương án đã chọn (để F030 trả phản hồi).
- **SelectionRule**: bộ lọc, số lượng, phân bổ độ khó, nới lỏng.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `assets_per_version_max` | Số media tối đa trong một version |
| `latex_max_length` / `latex_max_depth` / `math_svg_max` | Giới hạn công thức nhận trong `math_assets` (kiểm sâu ở F027) |
| `rapid_guess_min_response_ms` | Giá trị mặc định khi câu không khai `min_response_ms` (dùng ở F031) |
| `request_body_max` | Kích thước body CMS |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Answer key/scoring rule không bao giờ ra client | ADR-0016 · AGENTS R6 |
| Version published bất biến | ADR-0015 · AGENTS R11 |
| Domain chấm thuần Go, coverage domain ≥ 70% | NFR-MAINT-001 · NFR-MAINT-003 |
| Mọi chuỗi/khối JSON có giới hạn | NFR-MAINT-005 · ADR-0041 |
| Tenant isolation 100% endpoint | NFR-SEC-006 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 trường đáp án/gợi ý/phản hồi xuất hiện trong DTO render qua contract test của cả 8 loại.
- **SC-002**: 100% case trong bảng chấm (gồm ≥ 200 cặp chuẩn hóa tiếng Việt) cho điểm đúng; 0 lỗi `500` với response sai kiểu.
- **SC-003**: Cùng seed và cùng pool cho cùng tập câu và cùng thứ tự trong 100% lần chạy.
- **SC-004**: Biên tập viên tạo được câu hỏi hợp lệ của mỗi loại trong một lần gọi API; câu sai cấu trúc bị chặn trước khi lưu với vị trí lỗi cụ thể.

## Ngoài phạm vi

- `short_answer`, `image_based`, `interactive`, câu tham số hóa — F023.
- Workflow duyệt, kiểm audio `g1_3`, bản dịch — F025; publish/rollback/snapshot — F026; kiểm LaTeX/SVG và media `ready` — F027.
- Attempt, trả phản hồi theo `FeedbackTiming`, `hints:next`, `avoid_recent` từ lịch sử — F030/F032.
- Tìm kiếm full-text `q`, question set, import hàng loạt — F024. Item analysis — F044.
- Editor câu hỏi trên portal — F053.

## Assumptions

- Ví dụ JSON trong QUESTION_BANK §3 là fixture chuẩn cho test chấm [Confirmed].
- `worked_example_ref` lưu dạng JSON `{content_item_id, block_id}` (QUESTION_BANK §2, RB-203); validator tồn tại/LO chung do F024 cắm vào.
- Bộ ≥ 200 cặp chuẩn hóa tiếng Việt do EdTech Specialist cung cấp; trước đó dùng bộ do đội backend soạn và đánh dấu cần rà soát [Confirmed].

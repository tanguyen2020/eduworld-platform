# Feature Specification: F023 — Question Bank nâng cao: short_answer, image_based, interactive, câu tham số hóa

**Feature Branch**: `023-question-bank-advanced` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C2 · **Phụ thuộc**: F022
**Input**: Bổ sung 3 loại câu còn lại của `QuestionType` (`short_answer`, `image_based`, `interactive`) và câu hỏi tham số hóa vào registry của F022, giữ nguyên nguyên tắc chấm phía server và render không đáp án. FR nguồn: FR-QB-001, FR-QB-006, FR-QB-008, FR-QB-009.

## Clarifications

### Session 2026-11-16

- Q: Fuzzy match "không áp dụng cho môn Tiếng Việt/Tiếng Anh chính tả" — chặn hay chỉ cảnh báo? → A: Chặn khi lưu câu thuộc môn `vietnamese` hoặc `english` có `fuzzy.enabled = true` [Proposed] — đơn giản, kiểm được, tránh chấm sai chính tả ([QUESTION_BANK §4](../../docs/02-learning/QUESTION_BANK.md#4-chuẩn-hóa-văn-bản-tiếng-việt-match--normalized) bước 7).
- Q: Loại nào được tham số hóa? → A: `single_choice` và `numeric` — hai loại `question.schema.json` cho phép `params`.
- Q: Widget `interactive` đầu tiên? → A: `balance_scale` phiên bản `1.0`, validator `balance_scale.equal` theo ví dụ [QUESTION_BANK §3.11](../../docs/02-learning/QUESTION_BANK.md#311-interactive--câu-hỏi-tương-tác--mô-phỏng) [Proposed]; widget mới là release kỹ thuật (validator Go + scene Godot ở F063).

### Session 2026-11-19 (triển khai)

- OQ F023-1: Bố cục package khác plan: handler 3 loại mới ở `internal/domain/question/advanced.go` (không có thư mục `types/`, như OQ F022-2), hình học (tròn, chữ nhật, đa giác even-odd, kiểm tự cắt) nằm trong handler `image_based`; `expr`, `params`, `widget` là package riêng; Levenshtein và chuẩn hóa tiếng Anh dùng lại `normalize` của F022 (đã có). → [Proposed].
- OQ F023-2: Registry đủ 11 loại là `question.Full()`; `question.Base()` (8 loại) vẫn trả `TYPE_NOT_ENABLED` cho 3 loại mới (F022-AC-02 kiểm ở domain). Binary chưa nối (OQ F020-3) nên môi trường test dùng `Full()`. → [Proposed].
- OQ F023-3: Chặn fuzzy theo môn áp cho mọi đáp án chữ có `fuzzy.enabled = true` trong `answer_key` (`short_answer` và các chỗ trống `fill_in_blank`), theo mã môn `vietnamese`, `english` của danh mục. → [Proposed].
- OQ F023-4: `choice` của `params` chỉ nhận giá trị số (biến chữ không dùng được trong biểu thức) [Proposed]; số thập phân hiển thị tối đa 6 chữ số sau dấu phẩy; `presentation.params` lưu chuỗi thập phân dấu chấm. `Registry.GradeIn(content, presentation, response)` chấm theo giá trị của presentation (sinh lại từ `seed` nếu `params` trống); `Delivery.Grade` và preview (F027) dùng nó. → [Proposed].
- OQ F023-5: `interactive` — action log theo `interactiveResponse` (`{t_ms, action, args}`; `balance_scale`: `place`/`remove` với `args.w`), `state` là `{right_weights}`; mỗi quả cân có sẵn dùng tối đa một lần; `balance_scale.equal` so tổng đĩa phải với `params.target` (mặc định tổng đĩa trái). Lệch phát lại → điểm 0 + `GradeResult.integrity_mismatch` + metric `eduworld_qb_interactive_integrity_mismatch_total{widget}`; lưu tín hiệu gian lận thuộc F030/F031. → [Proposed].
- OQ F023-6: `image_based` thêm `render_payload.max_selections.maximum = 10` (khớp tối đa 10 điểm của response) và `answer_key.placements.maxProperties = 16`; chạm nhiều lần cùng một vùng tính một lần; với `partial/proportional` hotspot cho điểm theo phần vùng trúng khi không vượt `max_selections`. Ảnh phải nằm trong `media_refs` của version (kiểm có thật/`ready` thuộc F027/F025). → [Proposed].
- OQ F023-7: Test tích hợp đặt phẳng `tests/integration/question_advanced_test.go`; benchmark T031 chưa viết (đo khi F030 cần). → [Proposed].

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Câu trả lời ngắn chấm tự động (Priority: P1)

Biên tập viên soạn câu `short_answer` có danh sách đáp án chấp nhận; học sinh gõ "Ha Noi" hay "hà nội" được chấm đúng theo chính sách của câu.

**Why this priority**: FR-QB-001 yêu cầu 6 loại đầu ở P2, `short_answer` là loại thứ 5.

**Independent Test**: Bảng test (response → điểm) cho `short_answer` với `normalized`/`exact`, dấu, fuzzy, `content_language = en`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F023-AC-01 | KHI lưu câu `short_answer`, HỆ THỐNG PHẢI yêu cầu ít nhất một đáp án chấp nhận (mỗi đáp án 1–255 ký tự), `fuzzy.max_edit_distance` 1–2 và `fuzzy.min_length` ≥ 3, và trả `422 VALIDATION_FAILED` khi vi phạm | FR-QB-001 · [QUESTION_BANK §3.5](../../docs/02-learning/QUESTION_BANK.md#35-short_answer--trả-lời-ngắn) | TC-LRN-016 |
| F023-AC-02 | KHI chấm `short_answer`, HỆ THỐNG PHẢI dùng cùng bộ chuẩn hóa với `fill_in_blank` (F022), áp `allow_diacritics_missing`, và chỉ áp fuzzy (khoảng cách Levenshtein ≤ `max_edit_distance`) khi chuỗi đã chuẩn hóa dài ≥ `min_length` | FR-QB-006 · [QUESTION_BANK §4](../../docs/02-learning/QUESTION_BANK.md#4-chuẩn-hóa-văn-bản-tiếng-việt-match--normalized) | TC-LRN-003 |
| F023-AC-03 | NẾU câu thuộc môn `vietnamese` hoặc `english` bật fuzzy, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` với `errors[].code = FUZZY_NOT_ALLOWED_FOR_SUBJECT` | FR-QB-006 | — |
| F023-AC-04 | KHI câu có `content_language = en`, HỆ THỐNG PHẢI chuẩn hóa theo quy tắc tiếng Anh (không bỏ dấu, chuẩn hóa nháy/khoảng trắng, không phân biệt hoa thường trừ khi `case_sensitive`) | FR-QB-006 | TC-LRN-003 |

---

### User Story 2 — Câu hỏi dựa trên hình (hotspot, gắn nhãn) (Priority: P2)

Học sinh chạm vào vùng trên ảnh hoặc kéo nhãn vào điểm thả; tọa độ vùng đúng không bao giờ gửi xuống client.

**Why this priority**: FR-QB-001 đủ 11 loại ở P3; image_based cần cho môn Tự nhiên & Xã hội, Khoa học.

**Independent Test**: Tạo câu hotspot (hình tròn, chữ nhật, đa giác) và câu gắn nhãn; chấm điểm trong/ngoài vùng; quét DTO render.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F023-AC-05 | KHI lưu câu `image_based`, HỆ THỐNG PHẢI kiểm `image_media_ref` có trong `media_refs`, kích thước ảnh dương, vùng hợp lệ theo hình (`circle` r > 0, `rect` w,h > 0, `polygon` ≥ 3 đỉnh), mọi tọa độ trong `image_size`, `correct_region_ids` thuộc `regions`, `max_selections` ≥ số vùng đúng; với `label`, mọi `placements` tham chiếu nhãn và điểm thả có thật | FR-QB-001 · [QUESTION_BANK §3.10](../../docs/02-learning/QUESTION_BANK.md#310-image_based--câu-hỏi-dựa-trên-hình-hotspot--gắn-nhãn) | TC-LRN-016 |
| F023-AC-06 | HỆ THỐNG PHẢI loại `regions`, `correct_region_ids`, `placements` khỏi DTO render; với `label` chỉ gửi nhãn và điểm thả | ADR-0016 · AC-EXR-001 | TC-LRN-007 |
| F023-AC-07 | KHI chấm hotspot, HỆ THỐNG PHẢI xác định điểm thuộc vùng (tròn, chữ nhật, đa giác theo even-odd), đúng khi tập vùng trúng bằng tập vùng đúng và số điểm ≤ `max_selections`; KHI chấm gắn nhãn, PHẢI so ánh xạ nhãn → điểm thả theo `scoring_rule` | FR-QB-001 | TC-LRN-003 |
| F023-AC-08 | NẾU response có điểm ngoài `image_size`, quá 10 điểm, nhãn/điểm thả lạ, THÌ HỆ THỐNG PHẢI trả `INVALID_RESPONSE` | [QUESTION_BANK §11](../../docs/02-learning/QUESTION_BANK.md#11-kiểm-thử) | TC-LRN-016 |

---

### User Story 3 — Câu tương tác có validator đăng ký (Priority: P2)

Học sinh thao tác widget (cân thăng bằng); client gửi trạng thái cuối và nhật ký thao tác; server phát lại và kiểm chứng bằng validator Go thuần.

**Why this priority**: FR-QB-009; chặn gửi thẳng đáp án.

**Independent Test**: Gửi action log hợp lệ, action log giả (trạng thái cuối không tái tạo được), widget/validator chưa đăng ký.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F023-AC-09 | KHI lưu câu `interactive`, HỆ THỐNG PHẢI yêu cầu `widget` + `widget_version` có trong registry, `answer_key.validator` thuộc widget đó, `config` và `answer_key.params` hợp lệ theo schema của widget; không thỏa PHẢI trả `422 VALIDATION_FAILED` | FR-QB-009 | — |
| F023-AC-10 | HỆ THỐNG PHẢI chỉ chấm `interactive` bằng validator Go thuần đăng ký lúc build và không bao giờ thực thi script hay biểu thức do người soạn cung cấp | FR-QB-009 · ADR-0016 | — |
| F023-AC-11 | TRONG KHI `scoring_rule.require_action_log = true`, HỆ THỐNG PHẢI phát lại `action_log` từ `config` ban đầu, từ chối thao tác không hợp lệ (vd. vượt `max_moves`), và chỉ chấm đúng khi trạng thái phát lại trùng `state` gửi lên và validator đạt; lệch PHẢI cho điểm 0 và trả cờ `integrity_mismatch` trong kết quả chấm | FR-QB-009 · AC-EXR-002 | TC-LRN-007 |
| F023-AC-12 | NẾU `action_log` quá 500 thao tác hoặc `state` vượt giới hạn của schema response, THÌ HỆ THỐNG PHẢI trả `INVALID_RESPONSE` | FR-QB-009 | TC-LRN-016 |
| F023-AC-13 | KHI portal gọi `listQuestionWidgets`, HỆ THỐNG PHẢI trả danh sách widget, phiên bản, schema `config` và validator đã đăng ký (chỉ người có `learning_content.read`) | FR-QB-009 · [ADMIN_PORTAL §3.2](../../docs/04-admin-portal/ADMIN_PORTAL.md#32-content-management-82) | — |

---

### User Story 4 — Câu hỏi tham số hóa (Priority: P2)

Biên tập viên viết "{{a}} × {{b}} = ?" với ràng buộc; mỗi lượt làm bài server sinh giá trị riêng theo seed, render đề đã thay số và chấm theo đúng giá trị đã sinh.

**Why this priority**: FR-QB-008; tạo câu anh em vô hạn cho luyện tập và chống chép bài.

**Independent Test**: Lưu câu tham số hóa hợp lệ/không hợp lệ; sinh với nhiều seed; chấm; quét DTO.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F023-AC-14 | KHI lưu câu có `params` (chỉ `single_choice`, `numeric`), HỆ THỐNG PHẢI kiểm tên biến, kiểu (`int`, `decimal`, `choice`), `min ≤ max`, `step > 0`, mọi `{{biến}}` trong đề/phương án/đáp án đã khai báo, mọi ràng buộc và biểu thức đáp án parse được bằng evaluator an toàn, và tồn tại ít nhất một bộ giá trị thỏa ràng buộc; vi phạm PHẢI trả `422 VALIDATION_FAILED` | FR-QB-008 · [QUESTION_BANK §5](../../docs/02-learning/QUESTION_BANK.md#5-câu-hỏi-tham-số-hóa-proposed) | — |
| F023-AC-15 | HỆ THỐNG PHẢI chỉ cho evaluator dùng số, biến đã khai báo, `+ − × ÷ ^`, so sánh, phép logic và/hoặc/phủ định và ngoặc; mọi định danh hoặc lời gọi hàm khác PHẢI bị từ chối khi lưu | FR-QB-008 | — |
| F023-AC-16 | KHI tạo `Presentation` với một seed, HỆ THỐNG PHẢI sinh bộ giá trị thỏa ràng buộc, tái lập với cùng seed, lưu vào `presentation.params`, và thay giá trị vào đề, phương án trước khi trả cho client (định dạng số theo `content_language`) | FR-QB-008 | TC-LRN-003 |
| F023-AC-17 | KHI chấm câu tham số hóa, HỆ THỐNG PHẢI tính đáp án bằng evaluator trên `presentation.params` rồi áp quy tắc chấm của loại (dung sai `numeric`) | FR-QB-008 · FR-QB-006 | TC-LRN-003 |
| F023-AC-18 | HỆ THỐNG PHẢI không gửi định nghĩa `params`, ràng buộc hay biểu thức đáp án trong DTO render, chỉ gửi nội dung đã thay giá trị | FR-QB-008 · ADR-0016 | TC-LRN-007 |

---

### User Story 5 — Phân quyền và giới hạn cho các loại mới (Priority: P1)

Các loại mới đi qua cùng API CMS của F022 nên phải giữ nguyên phân quyền, cô lập tenant và giới hạn độ dài.

**Why this priority**: Không mở thêm đường rò dữ liệu khi thêm loại.

**Independent Test**: Chạy lại bộ test authz/isolation của F022 với fixture 3 loại mới và `listQuestionWidgets`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F023-AC-19 | HỆ THỐNG PHẢI áp cùng phân quyền (`learning_content.read/write`) và cô lập tenant của F022 cho câu `short_answer`, `image_based`, `interactive` và câu tham số hóa; câu của tenant khác PHẢI trả `404 NOT_FOUND` | AC-ORG-001 · NFR-SEC-006 | TC-TEN-001 |
| F023-AC-20 | NẾU chuỗi hoặc mảng vượt giới hạn (đáp án > 255 ký tự, đa giác > 64 đỉnh, quá 10 biến, ràng buộc > 255 ký tự), THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` chỉ rõ field | NFR-MAINT-005 · ADR-0041 | TC-TEN-024 |

### Edge Cases

- NẾU `short_answer` có hai đáp án chấp nhận trùng sau chuẩn hóa, THÌ lưu được, chấm như một (F023-AC-02).
- NẾU học sinh chạm đúng hai lần vào cùng một vùng, THÌ tính một lần trúng (F023-AC-07).
- NẾU đa giác tự cắt, THÌ vẫn dùng even-odd, validator cảnh báo bằng lỗi `422` khi cạnh tự cắt [Proposed] (F023-AC-05).
- NẾU ràng buộc tham số chỉ thỏa với rất ít bộ giá trị, THÌ kiểm khả thi khi lưu theo số lần thử tối đa; không tìm được → `422` (F023-AC-14).
- NẾU phép chia trong biểu thức gặp mẫu 0 khi sinh, THÌ coi bộ giá trị đó không thỏa và sinh lại (F023-AC-16).
- NẾU widget có phiên bản mới, THÌ câu cũ giữ `widget_version` cũ và vẫn chấm bằng validator phiên bản cũ (F023-AC-09).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-QB-001 | `short_answer`, `image_based`, `interactive` (đủ 11 loại cùng F022) | [QUESTION_BANK §3.5, §3.10, §3.11](../../docs/02-learning/QUESTION_BANK.md#3-mười-một-dạng-câu-hỏi) |
| FR-QB-006 | Chuẩn hóa + fuzzy cho `short_answer`; chuẩn hóa tiếng Anh | [QUESTION_BANK §4](../../docs/02-learning/QUESTION_BANK.md#4-chuẩn-hóa-văn-bản-tiếng-việt-match--normalized) |
| FR-QB-008 | Tham số hóa: khai báo, evaluator an toàn, sinh theo seed, render, chấm | [QUESTION_BANK §5](../../docs/02-learning/QUESTION_BANK.md#5-câu-hỏi-tham-số-hóa-proposed) |
| FR-QB-009 | Registry widget/validator Go, replay `action_log` | [QUESTION_BANK §3.11](../../docs/02-learning/QUESTION_BANK.md#311-interactive--câu-hỏi-tương-tác--mô-phỏng) |

### Key Entities

- **QuestionWidget**: widget tương tác đã đăng ký (mã, phiên bản, schema cấu hình, mô phỏng phát lại, các validator).
- **ParamSpec**: khai báo biến, kiểu, miền giá trị, ràng buộc của câu tham số hóa.
- **Presentation.params**: bộ giá trị đã sinh cho một lượt, dùng để render và chấm.
- **Region**: vùng hotspot trên ảnh (chỉ phía server).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `assets_per_version_max` | Số media (gồm ảnh của `image_based`) trong version |
| `request_body_max` | Kích thước body CMS |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không thực thi mã do người soạn viết; không eval tổng quát | [QUESTION_BANK §3.11, §5](../../docs/02-learning/QUESTION_BANK.md#5-câu-hỏi-tham-số-hóa-proposed) · AGENTS R5 |
| Đáp án/vùng/biểu thức không ra client | ADR-0016 · AGENTS R6 |
| Mọi chuỗi/mảng có giới hạn | NFR-MAINT-005 · ADR-0041 |
| Coverage domain ≥ 70% | NFR-MAINT-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Đủ 11 loại `QuestionType` tạo, chấm và render được; 0 trường đáp án/vùng/biểu thức trong DTO render qua contract test.
- **SC-002**: 100% action log giả (trạng thái cuối không tái tạo được) bị chấm 0 trong bộ test.
- **SC-003**: Với 1.000 seed khác nhau, mọi bộ giá trị sinh ra thỏa ràng buộc và cùng seed cho cùng giá trị.
- **SC-004**: 0 đường thực thi biểu thức ngoài evaluator an toàn (kiểm bằng test fuzz đầu vào evaluator).

## Ngoài phạm vi

- Scene Godot của widget và renderer 3 loại mới — F063.
- Lưu `presentation` vào `attempt_items`, gắn tín hiệu gian lận khi `integrity_mismatch` — F030/F031 (ANTI_CHEAT).
- Mô phỏng khoa học dạng activity (`simulation`) — F066.
- Câu tự luận chấm tay — deferred (ADR-0082).
- Upload/kiểm ảnh `ready` — F027; kiểm audio `g1_3` — F025.

## Assumptions

- Danh sách môn áp quy tắc chặn fuzzy theo mã môn `vietnamese`, `english` của danh mục môn (F021) [Proposed].
- Đa giác tối đa 64 đỉnh, tối đa 20 vùng, tối đa 10 biến và 10 ràng buộc mỗi câu (giới hạn độ dài schema theo ADR-0041, bổ sung vào `question.schema.json`) [Proposed].
- Widget `balance_scale@1.0` là widget tham chiếu duy nhất ở Release này cho tới khi đội game thêm widget khác [Proposed].

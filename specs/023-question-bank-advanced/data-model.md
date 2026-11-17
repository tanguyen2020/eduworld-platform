# Data Model: F023 — Question Bank nâng cao: short_answer, image_based, interactive, câu tham số hóa

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

### QuestionVersion — `content.question_versions` (Có sẵn — không đổi cột, xem [F022 data-model](../022-question-bank-core/data-model.md))

F023 chỉ thêm cấu trúc JSON mới bên trong các cột đã có:

| Cột | Loại câu | Nội dung | Gửi client |
|---|---|---|---|
| `render_payload` | `short_answer` | `max_length` (≤ 500), `keyboard` | ✔ |
| `answer_key` | `short_answer` | `accepted[]` (1–50 phần tử, mỗi phần tử ≤ 255), `match`, `allow_diacritics_missing`, `case_sensitive`, `fuzzy {enabled, max_edit_distance 1–2, min_length ≥ 3}` | ✘ |
| `render_payload` | `image_based` | `interaction` (`hotspot`/`label`), `image_media_ref`, `image_size {w,h}`, `max_selections`, `labels[]` (≤ 16), `drop_points[]` (≤ 16) | ✔ |
| `answer_key` | `image_based` | `regions[]` (≤ 20; `polygon.points` ≤ 64), `correct_region_ids[]`, `placements` | ✘ |
| `render_payload` | `interactive` | `widget`, `widget_version`, `config` | ✔ (`config` không chứa đích) |
| `answer_key` | `interactive` | `validator`, `params` | ✘ |
| `render_payload.params` | `single_choice`, `numeric` | định nghĩa biến (≤ 10), `constraints[]` (≤ 10, mỗi ≤ 255) | ✘ (tách khỏi DTO) |
| `answer_key.value` | `numeric` tham số hóa | biểu thức ≤ 255 ký tự | ✘ |

### Value object (không lưu bảng riêng ở F023)

| Value object | Trường | Lưu ở |
|---|---|---|
| `Presentation.params` | `{<tên biến>: <giá trị decimal dạng chuỗi>}` | F030: `assessment.attempt_items.presentation` |
| `GradeResult.integrity_mismatch` | bool | F030 ghi `assessment.integrity_signals` khi true |
| `QuestionWidget` | `code varchar(64)`, `version varchar(64)` (`^[0-9]+\.[0-9]+$`), `config_schema`, `validators[]` | Registry trong code (release kỹ thuật), không lưu DB |

## 2. Vòng đời / trạng thái

Như F022 (`contentmodel.Lifecycle`). Widget: thêm phiên bản mới không xóa phiên bản cũ khỏi registry khi còn câu published tham chiếu.

## 3. Quy tắc kiểm tra dữ liệu

- `short_answer`: chặn `fuzzy.enabled` với môn `vietnamese`, `english`.
- `image_based`: tọa độ trong `[0, w] × [0, h]`; `circle.r > 0`; `rect.w, h > 0`; `polygon` 3–64 đỉnh, không cạnh tự cắt; `image_media_ref ∈ media_refs`.
- `interactive`: `(widget, widget_version)` có trong registry; `validator` thuộc widget; `config`, `params` hợp lệ theo schema widget.
- Tham số: tên `^[a-z][a-z0-9_]{0,15}$`; `int`/`decimal` có `min ≤ max`, `step > 0`; `choice` có 1–20 giá trị; placeholder `{{tên}}` khớp khai báo; biểu thức chỉ dùng token cho phép, độ sâu AST ≤ 32, số mũ ≤ 10; khả thi khi lưu.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | F023 không phát event mới (tạo/sửa câu dùng audit của F022) | — |

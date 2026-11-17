# Research: F024 — Question set, lesson, assessment blueprint, worked example, import hàng loạt, tìm kiếm

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Full-text tiếng Việt không dấu

- **Decision**: Bật extension `unaccent`; hàm `content.f_unaccent(text) RETURNS text IMMUTABLE PARALLEL SAFE` bọc `public.unaccent('public.unaccent', $1)`. Mỗi bảng version có `search_text varchar(20000)` (ứng dụng ghép tiêu đề + văn bản thuần của khối/đề, cắt ở 20.000 ký tự) và cột sinh `search_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', content.f_unaccent(lower(search_text)))) STORED` + GIN. Truy vấn `search_tsv @@ websearch_to_tsquery('simple', content.f_unaccent(lower(:q)))`, luôn kèm điều kiện scope + `deleted_at IS NULL`.
- **Rationale**: Khớp API_DESIGN §5 (`unaccent` + `simple`); `unaccent` gốc là STABLE nên cần wrapper IMMUTABLE để dùng trong cột sinh/index; `simple` không stem, phù hợp tiếng Việt đơn âm tiết.
- **Alternatives**: `pg_trgm` (tốt cho gõ dở, index lớn hơn — có thể bổ sung sau); OpenSearch (thêm hạ tầng, trái nguyên tắc tối giản phụ thuộc).

## R2. Định nghĩa QTI-lite

- **Decision**: Nhận gói zip QTI 2.1 chỉ gồm `assessmentItem` với `choiceInteraction` (→ `single_choice`/`multiple_choice` theo `maxChoices`), `textEntryInteraction` trong `itemBody` (→ `fill_in_blank`), `extendedTextInteraction` bị từ chối, `orderInteraction` (→ `ordering`), `matchInteraction` (→ `matching`), `gapMatchInteraction` (→ `drag_drop`); `responseDeclaration/correctResponse` → `answer_key`; `mapping` → `partial`; metadata LO/độ khó đọc từ `imsmd` hoặc cột phụ trong manifest CSV đi kèm. Không chạy `responseProcessing` tùy biến (chỉ template chuẩn).
- **Rationale**: Đủ cho nguồn câu hỏi phổ biến, không thực thi logic chấm bên ngoài (ADR-0016), ánh xạ 1–1 sang validator F022.
- **Alternatives**: QTI 3.0 đầy đủ (quá rộng); chỉ CSV/Excel (không nhận được ngân hàng có sẵn của đối tác).

## R3. Đọc Excel

- **Decision**: `github.com/xuri/excelize/v2` ở tầng infrastructure (stream reader, giới hạn số dòng theo `csv_import_max`); cột theo mẫu `specs/examples/import/questions.template.xlsx`.
- **Rationale**: Thuần Go, đọc stream, phổ biến.
- **Alternatives**: Chuyển Excel → CSV ở client (mất kiểu dữ liệu, khó báo lỗi theo ô).

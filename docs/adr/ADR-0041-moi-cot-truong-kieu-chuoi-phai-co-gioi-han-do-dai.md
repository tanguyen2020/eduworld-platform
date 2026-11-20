# ADR-0041 — Mọi cột/trường kiểu chuỗi phải có giới hạn độ dài

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md), [API_DESIGN](../07-backend/API_DESIGN.md), [specs](../../specs/README.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder yêu cầu: các cột dữ liệu kiểu string phải có giới hạn maxlength.

## Quyết định

- **Database:** mọi cột chuỗi dùng `varchar(n)` với `n` tường minh, khai báo trong ent bằng `field.String(...).MaxLen(n)`. **Không** dùng `text` không giới hạn. Nội dung dài dùng `varchar(n)` với `n` lớn có lý do (vd 20.000) hoặc `jsonb` kèm CHECK `octet_length(col::text) <= N`.
- **Bảng độ dài chuẩn [Confirmed]:**

  | Loại | Max | Ví dụ |
  |---|---|---|
  | Mã/code/slug/enum | 64 | `tenant.code`, `subject.code`, `status` |
  | Tên ngắn | 128 | `nickname`, tên lớp, tên party |
  | Tên/tiêu đề | 255 | tên trường, tiêu đề bài học |
  | Email / URL / khóa lưu trữ | 320 / 2048 / 512 | `work_email`, `media.url`, `storage_key` |
  | Mô tả ngắn | 1.000 | mô tả quest |
  | Tin nhắn chat | 1.000 (theo policy, mặc định 300 hiển thị) | `message.body` |
  | Nội dung dài | 20.000 | nội dung bài học, giải thích |
  | LaTeX công thức | 2.000 | `latex` (ADR-0055) |
  | External ID | 64 | `student_external_id` |

- **API và spec:** mọi `type: string` trong OpenAPI, AsyncAPI, JSON Schema phải có `maxLength` (trừ `format: uuid|date|date-time`, enum, const, hoặc `pattern` có giới hạn). Spectral rule `string-must-have-maxlength` và `scripts/specs/check-maxlength.mjs` chặn merge nếu thiếu.
- **Validation:** handler kiểm tra độ dài (sinh từ OpenAPI) trước khi vào application; vượt giới hạn trả `422 VALIDATION_FAILED`.
- Godot và Admin Portal giới hạn ô nhập theo cùng giá trị (lấy từ spec).

## Hệ quả

(+) Chống dữ liệu phình và tấn công payload lớn; contract rõ ràng. (−) Phải chọn giới hạn hợp lý cho từng trường; đổi giới hạn cần migration.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted | Stakeholder |
| 2026-11-15 | Thêm giới hạn LaTeX 2.000 | ADR-0055 |

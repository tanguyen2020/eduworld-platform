# Quickstart: F022 — Question Bank 8 loại cơ bản: validator, scorer server-side, render payload không đáp án

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020, F021 (seed `GDPT2018_SAMPLE` đã publish và gán cho `th-demo-a`).
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events`.
- Tài khoản demo: `cm.noidung` (content_admin, câu platform), `ad.truong.a` (school_admin `th-demo-a`, có `learning_content.write`), `ad.truong.b` (`pgd-demo`), `qlcm.truong.a` (academic_manager), `sa.admin`.
- Fixture: `src/backend/modules/learning/tests/integration/question/fixtures/qb_examples/*.json` (8 ví dụ QUESTION_BANK §3).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` `POST /api/v1/cms/learning/questions` với từng ví dụ 8 loại (header `X-Tenant-Id`, `Idempotency-Key`) | `201`, version 1 `draft`, có `ETag`; gửi lại cùng key → cùng response | F022-AC-01 |
| 2 | Tạo câu `short_answer` | `422`, `errors[].code = TYPE_NOT_ENABLED` (trước F023) | F022-AC-02 |
| 3 | Gửi `single_choice` có 2 đáp án đúng; `ordering` 2 mục; `matching` cặp tham chiếu `r_9`; option id `A` | `422` kèm JSON pointer từng lỗi | F022-AC-03 |
| 4 | `ordering` với `partial_method = right_minus_wrong` | `422` | F022-AC-04 |
| 5 | `GET /cms/learning/questions/{id}` | Có `answer_key`, `options[].feedback`, `hints`; bản lưu `render_payload` không có `feedback` | F022-AC-06, F022-AC-10 |
| 6 | PATCH nháp không `If-Match` / `If-Match` cũ | `428` / `412` | F022-AC-08 |
| 7 | `ad.truong.b` GET câu của `th-demo-a`; `ad.truong.a` PATCH câu platform được gán | `404` / `403` | F022-AC-09 |
| 8 | `searchQuestions?subject_id=…&grade_level=3&type=numeric&source=own` | Chỉ câu khớp của `th-demo-a` | F022-AC-13 |
| 9 | `ad.truong.a` xóa câu nháp; `sa.admin` xem Thùng rác và khôi phục | Biến mất khỏi danh sách, có trong Thùng rác, khôi phục được | F022-AC-11 |
| 10 | Gửi đáp án chấp nhận 300 ký tự | `422` chỉ rõ field | F022-AC-12 |
| 11 | `make test MOD=learning` (gói `question/...`) | Bảng chấm 8 loại, ≥ 200 cặp chuẩn hóa, property shuffle pass | F022-AC-14 … F022-AC-21 |
| 12 | `make test-contract` (render DTO, view delivery) | Không trường đáp án trong DTO; `ordering` khác thứ tự đúng; `math_ref` thay LaTeX | F022-AC-22 … F022-AC-25 |
| 13 | `make test-integration MOD=learning` (selection) | Cùng seed cùng kết quả; pool thiếu → `QUESTION_POOL_INSUFFICIENT`; cache bị xóa sau `content.published` | F022-AC-26 … F022-AC-28 |
| 14 | Tạo version mới từ version đang `in_review` khi đã có nháp mở; PATCH version không draft | `409 CONFLICT` / `409 CONTENT_NOT_EDITABLE` | F022-AC-07 |
| 15 | Lưu câu có ký tự tổ hợp NFD, khối `worked_example`, 51 media | NFC khi đọc lại; `worked_example` và vượt media → `422` | F022-AC-05 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make lint MOD=learning
make test MOD=learning
make test-integration MOD=learning
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test theo mã F022-AC-* (số case bảng chấm, số cặp chuẩn hóa), báo cáo contract test DTO, trace_id bước 1 và 9.

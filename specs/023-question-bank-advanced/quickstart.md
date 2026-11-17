# Quickstart: F023 — Question Bank nâng cao: short_answer, image_based, interactive, câu tham số hóa

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F022 (F027 để ảnh `image_based` qua pipeline media; trước đó dùng `asset_id` fixture trong integration test).
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api`.
- Tài khoản demo: `cm.noidung` (content_admin), `ad.truong.a` (school_admin `th-demo-a`), `ad.truong.b` (`pgd-demo`).
- Fixture: `modules/learning/tests/integration/question/advanced/fixtures/` (ví dụ QUESTION_BANK §3.5, §3.10, §3.11, §5).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` tạo câu `short_answer` ("con mèo"/"mèo"), rồi bản thiếu `accepted` | `201`; bản lỗi `422` | F023-AC-01 |
| 2 | Tạo câu môn Tiếng Việt bật fuzzy | `422 FUZZY_NOT_ALLOWED_FOR_SUBJECT` | F023-AC-03 |
| 3 | `make test MOD=learning` gói `types` (`short_answer`) | "Mèo", " con  mèo. " đúng; "con meo" sai khi `allow_diacritics_missing=false`; câu `en` không bỏ dấu | F023-AC-02, F023-AC-04 |
| 4 | Tạo câu hotspot có vùng ngoài ảnh; polygon 2 đỉnh | `422` | F023-AC-05 |
| 5 | `make test-contract` (render `image_based`, `interactive`, tham số hóa) | Không `regions`, `placements`, `params`, biểu thức trong DTO | F023-AC-06, F023-AC-18 |
| 6 | Bảng chấm hotspot/label và response 11 điểm | Đúng/sai theo vùng; 11 điểm → `INVALID_RESPONSE` | F023-AC-07, F023-AC-08 |
| 7 | Tạo câu `interactive` widget `abc@1.0` | `422` | F023-AC-09 |
| 8 | `GET /api/v1/cms/learning/question-widgets` | Có `balance_scale` `1.0`, validator `balance_scale.equal` | F023-AC-13 |
| 9 | Test replay: log hợp lệ; log giả (state không tái tạo được); log 501 thao tác | Đúng / điểm 0 + `integrity_mismatch` / `INVALID_RESPONSE`; không có đường thực thi script | F023-AC-10, F023-AC-11, F023-AC-12 |
| 10 | Tạo câu `numeric` `{{a}} × {{b}}` với ràng buộc `a != b`; biểu thức `os.exit(1)`; ràng buộc không thể thỏa `a > 9` với `a ≤ 9` | `201` / `422` / `422` | F023-AC-14, F023-AC-15 |
| 11 | Test sinh 1.000 seed + chấm theo `presentation.params` | Thỏa ràng buộc, tái lập, đề đã thay số, chấm đúng | F023-AC-16, F023-AC-17 |
| 12 | `ad.truong.b` GET câu `interactive` của `th-demo-a`; polygon 65 đỉnh | `404` / `422` | F023-AC-19, F023-AC-20 |

## 3. Lệnh test tự động

```bash
make test MOD=learning
go test -fuzz=FuzzEval -fuzztime=60s ./modules/learning/internal/domain/question/expr/
make test-integration MOD=learning
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test theo mã F023-AC-*, log fuzz (số input, 0 crash), báo cáo contract test DTO.

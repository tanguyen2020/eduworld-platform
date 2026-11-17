# Quickstart: F024 — Question set, lesson, assessment blueprint, worked example, import hàng loạt, tìm kiếm

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020, F021 (`GDPT2018_SAMPLE` published, gán `th-demo-a`), F022 (câu hỏi Toán 3). F045 cho bước 12–13 qua portal (hoặc facade test). F027 để import qua portal (file upload); trước đó chạy import bằng integration test.
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events`.
- Tài khoản demo: `cm.noidung` (content_admin), `qlcm.truong.a` (academic_manager `th-demo-a`), `ad.truong.a` (school_admin `th-demo-a`), `ad.truong.b` (`pgd-demo`), `sa.admin`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` tạo set tĩnh 5 câu của `th-demo-a` | `201`, version 1 `draft` | F024-AC-01 |
| 2 | Set tham chiếu câu của `pgd-demo`; câu platform chưa gán; `cm.noidung` tạo set platform tham chiếu câu tenant | `404`/`422` / `403 RESOURCE_NOT_ASSIGNED` / `422` | F024-AC-02 |
| 3 | Set theo luật có `distribution` tổng 9 cho `count = 10` | `422` | F024-AC-03 |
| 4 | `GET …/question-sets/{id}/versions/1:pool-stats` với luật cần 10 câu khi pool 12 | `available = 12`, `warning = true`, `insufficient = false` | F024-AC-04 |
| 5 | PATCH set không `If-Match`; PATCH version đã khóa | `428`; `409 CONTENT_NOT_EDITABLE` | F024-AC-05 |
| 6 | `qlcm.truong.a` tạo lesson có `worked_example` 9 bước; có khối `activity` | `422` (steps); `422 BLOCK_KIND_NOT_ENABLED` | F024-AC-06, F024-AC-07 |
| 7 | Tạo lesson hợp lệ có `we_mul7`; câu hỏi đặt `worked_example_ref` tới `we_mul7` (cùng LO) và tới `we_khong_co` | Lưu được / `422` | F024-AC-08 |
| 8 | CRUD learning material (`/cms/learning/learning-materials`) | Tạo, xem, version mới, sửa nháp, xóa | F024-AC-09 |
| 9 | `cm.noidung` tạo blueprint theo EXAM_ENGINE §3; bản tổng 9,5 điểm; bản `feedback_timing = immediate` | `201` / `422` / `422` | F024-AC-10 |
| 10 | `:pool-stats` của blueprint `independent_draw` 4 biến thể; blueprint không khai `max_mean_p_diff` | Cờ thiếu đúng luật; giá trị mặc định lấy từ cấu hình | F024-AC-11, F024-AC-12 |
| 11 | Import 100 dòng CSV (5 dòng lỗi, 1 dòng công thức thiếu `math_assets`) dry-run rồi thật; import `lessons_docx`; file quá cỡ | Dry-run không ghi; thật tạo 95 draft, `errors` có số dòng; `MATH_RENDER_FAILED`; `docx`/quá cỡ bị từ chối; audit import | F024-AC-13, F024-AC-14, F024-AC-15, F024-AC-16 |
| 12 | `searchQuestions?q=nhan vat`, `listCmsLessons?q=nhan vat&grade_level=3`; `q` 201 ký tự | Ra nội dung "nhân vật" của tenant; không ra của `pgd-demo`; `422` | F024-AC-17, F024-AC-18, F024-AC-19 |
| 13 | `sa.admin` gán set platform cho `th-demo-a` | `ad.truong.a` thấy (chỉ đọc), `ad.truong.b` không thấy | F024-AC-21 |
| 14 | Clone set của `th-demo-a` sang `pgd-demo` (saga F045) | Bản sao draft, câu tenant được sao, câu platform giữ tham chiếu, `id_map`, không có attempt nào bị sao | F024-AC-22 |
| 15 | Xóa lesson draft; `ad.truong.b` GET lesson của `th-demo-a`; `ad.truong.a` không có `learning_content.import` gọi import | Vào Thùng rác / `404` / `403` (nếu role thiếu quyền) | F024-AC-23, F024-AC-20 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make test MOD=learning
make test-integration MOD=learning
make test-contract
make migrate-lint MOD=learning
make specs-lint
```

## 4. Bằng chứng cần nộp

- Kết quả test theo mã F024-AC-*, báo cáo job import (số tạo/từ chối), EXPLAIN truy vấn tìm kiếm, trace_id bước 11 và 14.

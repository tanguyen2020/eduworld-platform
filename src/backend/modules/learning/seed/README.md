# Seed — module learning

Dữ liệu giả cho local ([LOCAL_DEVELOPMENT §5](../../../../../docs/11-operations/LOCAL_DEVELOPMENT.md)). Không seed vào schema `ref` (projection nạp qua event).

| File | Nội dung | Nạp bằng |
|---|---|---|
| `curriculum/subjects.yaml` | Danh mục môn học mẫu (`math`, `vietnamese`, `english`) | `make seed` (`go -C src/backend run ./modules/learning/cmd/seed`) |
| `curriculum/gdpt2018_math_g1_5.yaml` | Curriculum `GDPT2018_SAMPLE` — GDPT 2018 Toán lớp 1–5 rút gọn (16 LO, `yccd_ref` trống, CURRICULUM_MODEL §7), định dạng `specs/schemas/curriculum-import.schema.json` | cùng lệnh: đi qua use case import của CMS rồi publish; chạy lại không tạo bản ghi trùng (F021-AC-23) |
| `content/math_g3.dev.json` | **Chỉ dev**: nội dung platform Toán khối 3 trên `GDPT2018_SAMPLE` (4 câu hỏi có tag `devseed-mg3-*`, bộ câu hỏi, activity quiz, bài học; tiêu đề `[DEV] …`; miễn trừ audio g1_3 vì chưa thu giọng đọc) và danh sách tenant demo được gán (ADR-0029) | `EW_SEED_PASSWORD=… make seed-content` sau `make up-app` + `make seed-check`: đi qua API CMS công khai (tạo → `submit_review` → `approve` → `publish`, tác giả `cm.noidung` ≠ người duyệt `cm.duyet`; activity do `sa.admin` duyệt vì cần `learning_content.activity.review`) và `assignToTenants` của System Admin; chạy lại không tạo gì mới |

Mỗi file `*.yaml` khác `subjects.yaml` trong thư mục `curriculum/` được nạp theo thứ tự tên file.

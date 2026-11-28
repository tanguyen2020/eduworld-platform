# Seed — module learning

Dữ liệu giả cho local ([LOCAL_DEVELOPMENT §5](../../../../../docs/11-operations/LOCAL_DEVELOPMENT.md)). Không seed vào schema `ref` (projection nạp qua event).

| File | Nội dung | Nạp bằng |
|---|---|---|
| `curriculum/subjects.yaml` | Danh mục môn học mẫu (`math`, `vietnamese`, `english`) | `make seed` (`go -C src/backend run ./modules/learning/cmd/seed`) |
| `curriculum/gdpt2018_math_g1_5.yaml` | Curriculum `GDPT2018_SAMPLE` — GDPT 2018 Toán lớp 1–5 rút gọn (16 LO, `yccd_ref` trống, CURRICULUM_MODEL §7), định dạng `specs/schemas/curriculum-import.schema.json` | cùng lệnh: đi qua use case import của CMS rồi publish; chạy lại không tạo bản ghi trùng (F021-AC-23) |
| `content/math_g3.dev.json` | **Chỉ dev**: nội dung platform Toán khối 3 trên `GDPT2018_SAMPLE` (4 câu hỏi có tag `devseed-mg3-*`, bộ câu hỏi, activity quiz, bài học; kho câu `pool_questions` — 8 `true_false` độ khó 1–3, 4 `matching` độ khó 2–3, tag `devseed-mg3-tf*`/`devseed-mg3-m*` — được publish và gán nhưng không thuộc bộ câu hỏi/activity nào: nguồn câu của 2 mini-game DEV của module game; tiêu đề `[DEV] …`; miễn trừ audio g1_3 vì chưa thu giọng đọc) và danh sách tenant được gán: `th-demo-a`, `pgd-demo`, `common` (ADR-0029, MULTITENANCY §8.3) | `EW_SEED_PASSWORD=… make seed-content` sau `make up-app` + `make seed-check`: đi qua API CMS công khai (tạo → `submit_review` → `approve` → `publish`, tác giả `cm.noidung` ≠ người duyệt `cm.duyet`; activity do `sa.admin` duyệt vì cần `learning_content.activity.review`) và `assignToTenants` của System Admin; chạy lại không tạo gì mới (tìm theo tag/tiêu đề; version đã publish không bao giờ bị sửa — đổi nội dung một câu thì dùng `key` mới, R11) |
| `assignments/demoa1_g3.dev.json` | **Chỉ dev**: bài giao của nội dung trên cho `th-demo-a`, trường `DEMOA1`, khối 3 — 1 bài activity, 1 bài question set (mode `practice`, mở từ 1 giờ trước lần chạy đầu, hạn 365 ngày); không có bài cho `pgd-demo` (F062 T018 kiểm cô lập tenant), không có đề (chưa seed assessment blueprint) | cùng `make seed-content`, sau phần nội dung: `ad.truong.a` (school_admin DEMOA1) gọi `createAssignment` → `publishAssignment`; tìm lại theo trường + khối + tiêu đề nên chạy lại không đổi gì; `-assignments=""` để bỏ qua |

Mỗi file `*.yaml` khác `subjects.yaml` trong thư mục `curriculum/` được nạp theo thứ tự tên file.

# Quickstart: F054 — Workspace B — media, bộ câu hỏi/lesson/blueprint, version và rollback, hàng đợi duyệt, preview, import, thùng rác

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F024, F025, F026, F053 (và F027); delta contract T005–T012 có backend.
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make up-app`; `cd web && pnpm gen:api && pnpm dev`.
- Tài khoản demo: `cm.noidung` (content_admin), `cm.duyet` (content_reviewer), `qlcm.truong.a` (academic_manager), `ad.donvi` (tenant_admin `pgd-demo`, giả định được cấp `tenant.trash.*`), `ad.truong.b`, `sa.admin`; học sinh `hs.an.3a` (kiểm ghim version trong game).
- File: CSV 20 câu (2 dòng lỗi, 1 công thức `\href`), ảnh PNG hợp lệ, file `.exe` đổi đuôi `.png`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Mở câu hỏi có v1, v2 → tab Phiên bản | Bảng version + dòng thời gian release | F054-AC-01 |
| 2 | Mở v1 (published) | Chỉ đọc, "Tạo bản nháp từ phiên bản này" | F054-AC-02 |
| 3 | So sánh v1–v2 bằng `cm.duyet` và bằng người không có `learning_content.review` | Diff theo field; chỉ người có review thấy diff đáp án | F054-AC-03 |
| 4 | Hẹn giờ xuất bản v3 sớm hơn mức tối thiểu; rồi hợp lệ; hủy hẹn giờ | Ngày quá sớm bị vô hiệu; hộp tóm tắt diff; hủy thành công | F054-AC-04 |
| 5 | `hs.an.3a` đang làm bài v2; `qlcm.truong.a` rollback về v1 có lý do | Lượt đang làm giữ v2; lượt mới nhận v1; timeline có lý do | F054-AC-05 |
| 6 | Rollback tới version của item khác qua API; người thiếu `content.release.rollback` | `ROLLBACK_TARGET_INVALID`; không có nút Rollback | F054-AC-06 |
| 7 | Gỡ xuất bản câu đang dùng trong question set | Cảnh báo danh sách nơi dùng, lý do bắt buộc | F054-AC-07, F054-AC-19 |
| 8 | Archive version cũ (không phải con trỏ) | Thành công; con trỏ → không có lựa chọn | F054-AC-08 |
| 9 | Duyệt mọi màn version | Không có thao tác sửa/xóa version published | F054-AC-09 |
| 10 | Xem trước vi/en/song song; Thử làm sai → gợi ý cấp 1, phản hồi phương án | Không tạo attempt (kiểm DB/learning event không đổi) | F054-AC-10 |
| 11 | Bấm Xuất bản hai lần / mất mạng rồi thử lại | Một release | F054-AC-11 |
| 12 | `cm.noidung` gửi 3 câu; `cm.duyet` mở Hàng đợi | 3 task, thời điểm gửi, cảnh báo quá hạn (dữ liệu cũ) | F054-AC-12 |
| 13 | Mở task: diff + preview; từ chối có ghi chú theo field | Tác giả thấy lý do + ghi chú | F054-AC-13 |
| 14 | Duyệt hàng loạt 2 task, có 1 task do chính `cm.duyet` soạn | Kết quả từng item, item tự soạn báo lỗi | F054-AC-14 |
| 15 | Reviewer tenant `ad.truong.a` mở Hàng đợi | Không thấy lane platform/tenant khác | F054-AC-15 |
| 16 | Thư viện media lọc `rejected` | Có lý do từ chối | F054-AC-16 |
| 17 | Upload `.exe` đổi đuôi `.png` và ảnh hợp lệ thiếu alt text | `MEDIA_TYPE_NOT_ALLOWED`; ảnh hợp lệ bị chặn đến khi có alt text | F054-AC-17 |
| 18 | Mở ảnh đang dùng | "Đang dùng ở version nào"; không có ghi đè file | F054-AC-18 |
| 19 | Tạo question set luật chọn "Toán 3, LO nhân, độ khó 2" | Hiển thị kích thước pool | F054-AC-20 |
| 20 | Lesson Builder: kéo khối, thêm `worked_example`, công thức | Lưu có `math_assets` | F054-AC-21 |
| 21 | Blueprint 3 LO × 3 độ khó → Sinh đề mẫu; giảm pool | Đề mẫu; `QUESTION_POOL_INSUFFICIENT` chỉ ô thiếu | F054-AC-22 |
| 22 | Sửa lesson đã published qua tab khác → lưu | `CONTENT_NOT_EDITABLE` → chỉ đọc + gợi ý | F054-AC-23 |
| 23 | Danh sách: tìm "phan so" (không dấu), lọc môn/khối; `sa.admin` lọc 2 tenant | Kết quả có "phân số"; cột Tenant | F054-AC-24 |
| 24 | Xóa câu đã published có lý do | Biến mất khỏi danh sách | F054-AC-25 |
| 25 | `ad.donvi` mở Thùng rác, khôi phục; khôi phục câu đã có mã trùng | Khôi phục được; trùng → lý do | F054-AC-26 |
| 26 | Import CSV mẫu: chạy thử | Lỗi công thức báo trước khi gửi; lỗi dòng từ server; chưa tạo Draft | F054-AC-27 |
| 27 | `cm.duyet` (không có import) và người không có `tenant.trash.read` | Không có nút Import/Thùng rác | F054-AC-28 |
| 28 | Dán chuỗi quá dài vào lý do rollback | Bị chặn + bộ đếm | F054-AC-29 |
| 29 | Thu nhỏ < 768 px | Danh sách thẻ, duyệt được; diff/import/builder → "cần màn hình lớn hơn" | F054-AC-30 |
| 30 | `ad.truong.b` mở URL version/usages/review task của `th-demo-a` | "Không tìm thấy" | F054-AC-15 |
| 31 | `sa.admin` tắt `action.content_studio.lessons.rollback` cho `th-demo-a`; `qlcm.truong.a` mở tab Phiên bản của bài học | Nút "Rollback"/"Gỡ xuất bản" ẩn/khóa; `rollbackContent` (`content_type = lesson`) → `403 FEATURE_DISABLED`; rollback câu hỏi vẫn chạy | F054-AC-31 |

## 3. Lệnh test tự động

```bash
make web-lint
make web-test
cd web && pnpm --filter portal test:e2e -- --grep @F054
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), báo cáo axe; ảnh timeline release sau rollback; `trace_id` kịch bản 5 và 26; truy vấn chứng minh preview không tạo attempt (kịch bản 10).

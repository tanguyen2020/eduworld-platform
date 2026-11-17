# Quickstart: F053 — Workspace B — cây curriculum, editor nội dung, công thức, song ngữ, trình soạn câu hỏi

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F021, F022, F027, F051 (F023 cho 3 dạng nâng cao); delta contract T005–T009 có backend.
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make up-app`; `cd web && pnpm gen:api && pnpm gen:schemas && pnpm dev`; `pnpm storybook`.
- Tài khoản demo: `cm.noidung` (content_admin), `cm.duyet` (content_reviewer), `qlcm.truong.a` (academic_manager `th-demo-a`), `ad.truong.b` (tenant khác), `sa.admin`.
- Ảnh mẫu PNG có alt text; file âm thanh đề bài cho khối 3.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `cm.noidung` mở "Tạo câu hỏi", duyệt qua 11 dạng | Mỗi dạng có form riêng | F053-AC-01 |
| 2 | Tạo `single_choice` 4 phương án, chọn đáp án, xóa phương án đang là đáp án | ID dạng `o_xxxx`; lưu bị chặn đến khi chọn lại đáp án | F053-AC-02 |
| 3 | Thêm feedback + `misconception_tag` cho phương án sai, 3 gợi ý; mở "Như học sinh thấy" | Preview không có feedback/gợi ý/đáp án | F053-AC-03 |
| 4 | Đặt khối 3, không có audio đề → Gửi duyệt | Nút khóa, liệt kê field thiếu; thêm audio → mở | F053-AC-04 |
| 5 | Chèn ảnh mới (upload) | Bắt buộc license + alt text; trạng thái tới `ready` | F053-AC-05 |
| 6 | Gõ liên tục 30 giây | "Đang lưu…" → "Đã lưu lúc HH:mm"; request có `If-Match` | F053-AC-06 |
| 7 | Mở cùng câu ở tab thứ hai, sửa và lưu; quay lại tab đầu sửa | Banner xung đột, không ghi đè | F053-AC-07 |
| 8 | `cm.noidung` Gửi duyệt; `cm.duyet` Duyệt, Xuất bản ngay (bấm hai lần) | Một release; thanh trạng thái "Đã xuất bản v1"; tác giả không thấy nút Duyệt | F053-AC-09, F053-AC-10 |
| 9 | Mở lại câu v1 | Chỉ đọc + "Tạo bản nháp từ v1" → v2 draft | F053-AC-08 |
| 10 | `qlcm.truong.a` mở câu platform đã gán cho `th-demo-a` | Nhãn "Từ EduWorld", chỉ đọc | F053-AC-11 |
| 11 | `sa.admin` tạo câu không chọn phạm vi | Bắt buộc chọn Platform hoặc tenant đích | F053-AC-12 |
| 12 | `cm.duyet` mở editor | Không có nút tạo/sửa; có Duyệt | F053-AC-13 |
| 13 | Dán chuỗi dài hơn giới hạn vào phương án, `change_note` | Bị chặn + bộ đếm | F053-AC-14 |
| 14 | Thu nhỏ < 1024 px | "Tác vụ này cần màn hình lớn hơn", xem chỉ đọc | F053-AC-15 |
| 15 | Nhập `\frac{3}{4}` bằng bàn phím ảo, `\ce{2H2 + O2 -> 2H2O}`; lưu | Request có `math_assets` (SVG `currentColor` + speech) | F053-AC-16, F053-AC-17 |
| 16 | Nhập `\href{x}{y}` / công thức sai cú pháp | Chặn ở client; nếu gửi thẳng API → `422 MATH_RENDER_FAILED`, công thức tô đỏ | F053-AC-17, F053-AC-18 |
| 17 | Xem bộ công thức mẫu theme sáng/tối | Khớp ảnh chuẩn | F053-AC-19 |
| 18 | Câu có công thức chọn dạng `short_answer` | Cảnh báo ADR-0055 | F053-AC-20 |
| 19 | Soạn đề vi/en, xóa bản `vi` (gốc) | Hai cột, nhãn "Gốc"; huy hiệu Thiếu; Gửi duyệt khóa | F053-AC-21, F053-AC-22, F053-AC-23 |
| 20 | Câu `vi` không có `en` → xuất bản | Thành công, cảnh báo `partial` | F053-AC-24 |
| 21 | Xuất XLIFF, sửa `en`, nhập lại | Job xong, Draft mới, báo field không khớp | F053-AC-25 |
| 22 | Mở "Chương trình học" Toán 3, kéo chủ đề | Cây đúng; `updateTopic` kèm `If-Match`; lỗi → hoàn tác | F053-AC-26, F053-AC-27 |
| 23 | Thêm LO tiên quyết tạo chu trình | Lỗi hiển thị | F053-AC-28 |
| 24 | `qlcm.truong.a` xem curriculum platform; tạo curriculum riêng tenant | Chỉ đọc với platform; tạo riêng được | F053-AC-29 |
| 25 | `cm.noidung` gửi duyệt curriculum version thiếu `yccd_ref` | Danh sách LO thiếu | F053-AC-30 |
| 26 | Thêm khối `worked_example` 3 bước; gắn vào câu hỏi | Lưu đúng; câu có `worked_example_ref` | F053-AC-31, F053-AC-32 |
| 27 | `ad.truong.b` mở URL câu hỏi của `th-demo-a` | "Không tìm thấy" | F053-AC-13 |
| 28 | `sa.admin` tắt `action.content_studio.question_bank.publish` cho `th-demo-a`; `cm.duyet` mở câu hỏi đã duyệt | Nút "Xuất bản" ẩn/khóa với tooltip "Tính năng đang tắt"; gọi thẳng `publishContentVersion` → `403 FEATURE_DISABLED` (`feature_key`) | F053-AC-33 |

## 3. Lệnh test tự động

```bash
make web-lint
make web-test
cd web && pnpm --filter portal test:e2e -- --grep @F053
make storybook
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail); ảnh visual regression công thức (sáng/tối); HAR kịch bản 15; `trace_id` kịch bản 8.

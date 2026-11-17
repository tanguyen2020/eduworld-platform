# Quickstart: F058 — Workspace B — Learning Activity Builder (quiz, interactive_exercise, video_lesson), UI re-evaluate

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F030, F031, F053, F065, F027 (và F054 nếu kiểm lịch sử version).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm gen:schemas && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản: `cm.noidung` (content_admin), `cm.duyet` (content_reviewer), `qlcm.truong.a` (academic_manager, tenant `th-demo-a` scope Toán), `ad.truong.a` (school_admin), `sa.admin` (system_admin), học sinh demo `hs.an.3a`.
- Dữ liệu: câu hỏi Toán lớp 3 seed đã xuất bản (gồm `single_choice`, `numeric`); một video ngắn (~4 phút) đã `ready`; một file `.vtt` tiếng Việt.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `cm.noidung` → Activity → Tạo mới | Chỉ thấy `quiz`, `interactive_exercise`, `video_lesson` (không `written_response`/`speaking`/`project`) | F058-AC-01 |
| 2 | Chọn `quiz`, để trống LO, bấm Lưu | Ô LO báo bắt buộc; thêm LO → `createCmsActivity` 201, request có `Idempotency-Key` | F058-AC-02, F058-AC-28 |
| 3 | Nguồn câu hỏi = danh sách, chọn 5 câu `single_choice`; thử thêm câu `numeric` | Câu `numeric` bị chặn; đổi sang bộ câu hỏi và quy tắc chọn hoạt động | F058-AC-03 |
| 4 | Chọn chiến lược chấm, `feedback.timing`, quy tắc hoàn thành; bật quiz tính giờ 5 giây | Chỉ thấy chiến lược của loại; 5 giây bị chặn theo `minimum` | F058-AC-04 |
| 5 | Mở cùng activity ở tab thứ hai, sửa và lưu; quay lại tab đầu | Tab đầu dừng autosave, báo xung đột, không ghi đè | F058-AC-05 |
| 6 | Dùng DevTools sửa request để `config` sai schema | `422 ACTIVITY_CONFIG_INVALID` ánh xạ đúng ô | F058-AC-06 |
| 7 | Nhập `\frac{3}{4}` vào hướng dẫn; nhập `\href{x}{y}` | Request có `math_assets`; công thức cấm bị tô đỏ | F058-AC-08 |
| 8 | Khối lớp 3 không có audio hướng dẫn → Gửi duyệt | Nút khóa, liệt kê trường thiếu | F058-AC-09 |
| 9 | Xem trước (vi rồi en) | Khung học sinh hiển thị câu, network preview không có `answer_key`/`feedback`; không có attempt mới | F058-AC-10, F058-AC-11 |
| 10 | `cm.noidung` Gửi duyệt; kiểm nút Duyệt; `cm.duyet` duyệt → Xuất bản ngay, bấm hai lần | Tác giả không thấy Duyệt; một release, trạng thái "Đã xuất bản v1" | F058-AC-12, F058-AC-14 |
| 11 | Mở activity v1 đã xuất bản → "Tạo bản nháp từ v1" (sau delta) | Không có nút Sửa; tạo v2 draft | F058-AC-07 |
| 12 | Giả lập registry trả `video_lesson` `status = preview` | Lưu nháp được, Xuất bản khóa, hiện `min_client_version` | F058-AC-13 |
| 13 | Tạo `video_lesson`: chọn video đang `processing` rồi `ready` | Gửi duyệt khóa tới khi `ready`; `duration_ms` chỉ đọc | F058-AC-15 |
| 14 | Thêm 2 chương chồng lấn; sửa không chồng lấn | Chặn khi chồng lấn | F058-AC-16 |
| 15 | Đánh dấu "có lời", không phụ đề `vi` → Gửi duyệt; thêm `.vtt` | Bị chặn rồi qua | F058-AC-17 |
| 16 | Cue bắt buộc tại 90 000 ms; cue tại 300 000 ms | Cue thứ hai bị chặn, báo đúng cue | F058-AC-18 |
| 17 | Thêm nhánh trỏ segment không tồn tại; không có segment kết thúc | Báo lỗi từng lựa chọn | F058-AC-19 |
| 18 | Xem giá trị điền sẵn tua/hoàn thành; dùng video dài hơn khuyến nghị cho khối 3 | `watched_only`, `min_watched_ratio` theo `video_completion`; cảnh báo không chặn | F058-AC-20 |
| 19 | Xem trước video | Dừng tại 01:30 hiện câu hỏi (không đáp án); bật/tắt phụ đề; danh sách chương | F058-AC-21 |
| 20 | `qlcm.truong.a`: câu hỏi v2 sai đáp án đã có 3 attempt, tạo v3 sửa và xuất bản; mở v2 → Chấm lại hệ thống | Hiện số attempt bị ảnh hưởng; `use_new_key` bắt buộc chọn v3; lý do < 10 ký tự bị chặn | F058-AC-22, F058-AC-25 |
| 21 | Xác nhận, bấm lặp; theo dõi trạng thái và tab lịch sử | Một yêu cầu; `completed`, `re_evaluated_attempts = 3` | F058-AC-23 |
| 22 | `ad.truong.a` mở v2 | Không có nút; gọi API trực tiếp → thông điệp `PERMISSION_DENIED` | F058-AC-24 |
| 23 | `ad.truong.a` mở builder | Chỉ xem, không có Tạo/Sửa activity | F058-AC-26 |
| 24 | `qlcm.truong.a` mở activity platform được gán; `sa.admin` tạo activity không chọn tenant | Nhãn "Từ EduWorld" chỉ đọc; `sa.admin` bị chặn tới khi chọn tenant/Platform | F058-AC-27 |
| 25 | Dán chuỗi dài vào tiêu đề, tiêu đề chương, lý do | Giới hạn theo `maxLength`, có bộ đếm | F058-AC-29 |
| 26 | Thu cửa sổ < 1024 px | "Tác vụ này cần màn hình lớn hơn", chỉ xem | F058-AC-30 |
| 27 | `sa.admin` tắt `menu.content_studio.activities` cho `th-demo-a`; `cm.noidung` tải lại portal | Menu "Hoạt động học tập" biến mất; URL `/content-studio/activities` → trang "Tính năng đang tạm tắt"; `listCmsActivities` → `403 FEATURE_DISABLED` | F058-AC-31 |

## 3. Lệnh test tự động

```bash
cd src/cms/frontend
pnpm --filter ops-ui test -- src/activity-editor
pnpm --filter portal test -- tests/unit/content-studio/activities tests/unit/content-studio/regrade
pnpm --filter portal exec playwright test tests/e2e/content-studio --grep @F058
make web-lint
make web-test
```

## 4. Bằng chứng cần nộp

- Kết quả Vitest/Playwright (số pass/fail), network log kịch bản 9, 10, 21; `trace_id` kịch bản 2, 10, 21; ảnh màn hình builder quiz, Video Manager, hộp thoại chấm lại (sáng/tối).

# Quickstart: F055 — Workspace C — LMS: course, learning path, wizard giao bài, theo dõi nộp bài, lịch, nhóm học tập

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F040, F041, F042, F050, F051, F052.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo (mật khẩu seed local): `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`, `ad.donvi`, `sm.antoan`, `sa.admin`; học sinh `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` → Giao bài → wizard, tab Activity | Chỉ loại tự chấm đã publish; không có `written_response` | F055-AC-01 |
| 2 | Bước Đối tượng | Chỉ lớp/nhóm/học sinh TH Demo A | F055-AC-02 |
| 3 | Chọn `graded` | `follow_latest` bị tắt; payload không có `feedback_timing`/`result_release` | F055-AC-03 |
| 4 | Nhập tiêu đề vượt `maxLength`, `due_at < open_at` | Báo lỗi tại trường, nút Giao bài tắt | F055-AC-04 |
| 5 | Bấm Giao bài hai lần nhanh; chặn mạng rồi thử lại | Một bài; hai request cùng `Idempotency-Key` | F055-AC-05 |
| 6 | Chọn quest chưa gán (giả lập gỡ gán) | Thông điệp `RESOURCE_NOT_ASSIGNED` | F055-AC-06 |
| 7 | `sa.admin` mở wizard | Bắt buộc chọn tenant đích; request có `X-Tenant-Id` tenant đích | F055-AC-07 |
| 8 | Mở chi tiết bài; `hs.an.3a` làm bài trong game | Bảng submission cập nhật, nhãn "làm quá nhanh" khi có | F055-AC-08 |
| 9 | Gia hạn `hs.an.3a`, miễn `hs.binh.3a` (có lý do) | Trạng thái dòng cập nhật | F055-AC-09 |
| 10 | Nhắc hai lần trong ngày | Lần 2: "Bài này đã được nhắc hôm nay", nút khóa | F055-AC-10 |
| 11 | Bài `manual` đã đóng → Công bố; hủy bài đã có submission | Công bố OK; hủy báo xung đột | F055-AC-11 |
| 12 | Sửa bài `open` ở hai tab, lưu tab cũ | Chỉ sửa được hạn/hướng dẫn; tab cũ nhận báo tải lại | F055-AC-12 |
| 13 | Khóa học → ẩn course Tiếng Việt 3A | `status = hidden` | F055-AC-13 |
| 14 | Dựng path 3 node `mastery_gate`, mở khóa node cho `hs.binh.3a` | Path lưu; unlock cần lý do | F055-AC-14, F055-AC-15 |
| 15 | Tạo nhóm "Ôn Nhân 3A"; xóa nhóm | Nhóm dùng được ở wizard; xóa sau xác nhận | F055-AC-16 |
| 16 | Lịch tuần: thêm/sửa/xóa mục | Lịch cập nhật | F055-AC-17 |
| 17 | Bài `graded` có đề → tab Hỗ trợ thời gian, +25% cho `hs.binh.3a` | Lưu thành công (sau delta T004) | F055-AC-18 |
| 18 | `ad.donvi` mở LMS; `qlcm.truong.a` tạo bài course Tiếng Việt | Không có nút ghi / `403` hiển thị, dữ liệu giữ nguyên | F055-AC-19 |
| 19 | `sm.antoan` mở URL `/portal/school-lms/lms/assignments` | Trang 403, không tải chunk C | F055-AC-20 |
| 20 | `ad.truong.b` mở URL bài của TH Demo A; gửi header cũ sau đổi đơn vị (`qlcm.truong.a`) | "Không tìm thấy"; `TENANT_MISMATCH` → màn chọn đơn vị | F055-AC-21 |
| 21 | Duyệt toàn bộ menu workspace C | Không có màn sửa quest/mini-game | F055-AC-22 |
| 22 | `sa.admin` tắt `menu.school_lms.study_groups` cho `th-demo-a`; `ad.truong.a` mở wizard giao bài | Menu "Nhóm học tập" biến mất; bước chọn đối tượng không còn "Nhóm học tập"; `listStudyGroups` → `403 FEATURE_DISABLED` | F055-AC-24 |

## 3. Lệnh test tự động

```bash
cd src/cms/frontend
pnpm --filter portal test -- src/workspaces/school-lms/lms
pnpm --filter portal exec playwright test e2e/lms
make web-lint
make web-test
```

## 4. Bằng chứng cần nộp

- Kết quả Vitest/Playwright (số pass/fail), báo cáo axe, `trace_id` của kịch bản 5 và 9, ảnh màn hình wizard và chi tiết bài (sáng/tối).

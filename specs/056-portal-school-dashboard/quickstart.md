# Quickstart: F056 — Workspace C — School Learning Dashboard và báo cáo

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F043, F044, F052, F055, F057 (bộ lọc tenant), F050, F051.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; nạp golden dataset dashboard của F043; `cd web && pnpm install && pnpm gen:api && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Chuẩn bị dữ liệu: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` làm bài giao do `qlcm.truong.a` tạo (F055); một học sinh làm rất nhanh để có cờ `rapid_guess`.
- Tài khoản: `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`, `ad.donvi`, `sa.admin`, `sm.antoan`, `cm.noidung`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` → Tiến độ & báo cáo → Tổng quan, "Tuần này" | 5 widget đúng thứ tự | F056-AC-01 |
| 2 | Quan sát widget trong ngày | "Cập nhật lúc HH:mm", "Tạm tính", nhãn "Theo ngày" | F056-AC-02 |
| 3 | Mở date picker, chọn khoảng > giới hạn | Ngày ngoài giới hạn bị vô hiệu; không có "toàn bộ thời gian" | F056-AC-03 |
| 4 | Giả lập `429 DASHBOARD_BUSY` (6 tab song song) và `504` | Widget tự thử lại ≤ 2 lần; 504 gợi ý thu hẹp; widget khác vẫn hiện | F056-AC-04 |
| 5 | Xem "Cần chú ý" | Học sinh làm nhanh có nhãn "làm quá nhanh"; không bảng xếp hạng | F056-AC-05 |
| 6 | `ad.donvi` xem tổng hợp đơn vị có trường < 5 học sinh hoạt động | Ô hiển thị "< 5" | F056-AC-06, F056-AC-17 |
| 7 | Widget An toàn → Xem | Chỉ số lượng + điều hướng; không có nội dung chat | F056-AC-07 |
| 8 | Bấm lớp 3A | Trang lớp giữ khoảng/môn; breadcrumb trường → lớp | F056-AC-08 |
| 9 | Heatmap: dùng bàn phím + trình đọc màn hình | Ký hiệu mức, nhãn đọc được | F056-AC-09 |
| 10 | LO yếu → "Giao ôn tập theo LO" → "Giao bài" | Wizard F055 điền sẵn; bài tạo sau 2 thao tác | F056-AC-10 |
| 11 | Mở hồ sơ `hs.an.3a` (sau delta T004) | Mastery, xu hướng, lịch sử, gợi ý, tab đăng nhập; không IP/bạn bè/chat | F056-AC-11 |
| 12 | Course Toán 3A → Kết quả | Cột bài graded, điểm thang theo `score_scale`, không cột XP | F056-AC-12 |
| 13 | Bài graded → Phân tích (sau delta T005) | Phổ điểm, p-value, sai phổ biến, `rapid_guess` loại khỏi p-value | F056-AC-13 |
| 14 | Xuất kết quả bài giao XLSX | "Đang chuẩn bị" → link có hạn | F056-AC-14 |
| 15 | Mở 3 export liền; mở link sau khi hết hạn | Thông điệp `EXPORT_LIMIT_REACHED`; link hết hạn báo tạo lại | F056-AC-15 |
| 16 | Xuất báo cáo tham khảo TT27/TT22 | Màn và file có nhãn "Tham khảo — không thay đánh giá chính thức" | F056-AC-16 |
| 17 | `sa.admin` → Tổng hợp platform, lọc 2 tenant | Cột Tenant; số liệu 2 tenant | F056-AC-18 |
| 18 | `qlcm.truong.a` (scope Toán) mở bộ lọc môn; gọi Tiếng Việt qua URL | Chỉ Toán; URL môn khác → "Ngoài phạm vi quyền" | F056-AC-19 |
| 19 | `sm.antoan`, `cm.noidung` mở URL dashboard | Trang 403, không có request dashboard | F056-AC-20 |
| 20 | `ad.truong.b` mở `/portal/school-lms/dashboard/classes/<id lớp 3A>` | "Không tìm thấy" | F056-AC-21 |
| 21 | Dán chuỗi dài vào ô tìm học sinh | Bị cắt theo `maxLength` | F056-AC-22 |
| 22 | `sa.admin` tắt `action.school_lms.reports.export` cho `th-demo-a`; `ad.truong.a` mở Tiến độ & báo cáo | Nút "Xuất báo cáo" ẩn/khóa; `createLmsReport` → `403 FEATURE_DISABLED`; dashboard vẫn xem được | F056-AC-23 |

## 3. Lệnh test tự động

```bash
cd web
pnpm --filter ops-ui test -- src/dashboard
pnpm --filter portal test -- src/workspaces/school-lms/dashboard
pnpm --filter portal exec playwright test e2e/dashboard
make web-lint
make web-test
make storybook
```

## 4. Bằng chứng cần nộp

- Kết quả Vitest/Playwright, báo cáo axe, ảnh visual regression (sáng/tối/compact, mobile), `trace_id` của kịch bản 1 và 14, ảnh màn hình tổng quan và heatmap.

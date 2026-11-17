# Quickstart: F081 — Workspace C — sự kiện trường từ template

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F052, F077 (và F050, F051); delta T004, T006 đã có backend.
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make migrate`, `make seed`, `make up-app`; `cd web && pnpm install && pnpm gen:api && pnpm dev`.
- Tài khoản demo: `ad.truong.a` (school_admin TH Demo A), `ad.truong.b` (school_admin THCS Demo B), `qlcm.truong.a` (academic_manager), `sa.admin` (system_admin); học sinh `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b`.
- Dữ liệu: seed F077 (template "Tuần lễ Toán học" gán `th-demo-a`, có community goal); fixture từ nhạy cảm placeholder (không dùng từ thật — R15).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` mở Sự kiện trường → Tạo | Chỉ template đã gán; `ad.truong.b` (tenant khác) thấy danh sách rỗng + hướng dẫn | F081-AC-01 |
| 2 | Chọn template, điền form | Chỉ trường `allowed_params`; quest/challenge/thưởng từ danh sách template; lớp chỉ của TH Demo A | F081-AC-02, F081-AC-05 |
| 3 | Bấm Tạo, mô phỏng mất mạng rồi bấm lại | Một sự kiện `scheduled`, cùng `Idempotency-Key` trong network log | F081-AC-03 |
| 4 | Tên chứa từ fixture; tắt module chat rồi tạo; tạo sự kiện thứ (`school_events_concurrent_per_tenant` + 1) | Lỗi tại trường tên không có từ khớp; `503` giữ form; `409` báo đạt tối đa | F081-AC-04 |
| 5 | `sa.admin` mở danh sách, lọc 2 tenant; tạo mới | Cột Tenant; form bắt chọn tenant đích | F081-AC-06 |
| 6 | Sửa ngày ở hai tab; tab thứ hai lưu | `412` → báo dữ liệu đổi, tải lại; sự kiện `live` không có nút Sửa | F081-AC-07 |
| 7 | Hủy không lý do; hủy có lý do | Bị chặn; sau xác nhận 2 bước → `cancelled` | F081-AC-08 |
| 8 | `qlcm.truong.a` mở URL Sự kiện trường; `ad.truong.b` dán URL sự kiện TH Demo A | 403 không menu; "Không tìm thấy" | F081-AC-09 |
| 9 | `hs.an.3a`, `hs.chi.3b` làm hoạt động sự kiện; `ad.truong.a` mở Tiến độ | Số tham gia, thanh mốc, bảng lớp theo tên lớp; response không có `student_id` | F081-AC-10, F081-AC-11 |
| 10 | `ad.truong.b` gọi `getSchoolEventProgress` của TH Demo A | `404` | F081-AC-12 |
| 11 | Mở các màn trên điện thoại (Playwright viewport 390 px) | Chỉ xem; form hiển thị "Tác vụ này cần màn hình lớn hơn" | F081-AC-19 |
| 12 | `sa.admin` tắt `menu.school_lms.school_events` cho `th-demo-a`; `ad.truong.a` tải lại portal | Menu "Sự kiện trường" biến mất; `createSchoolEvent` → `403 FEATURE_DISABLED`; `ad.truong.b` (tenant khác) vẫn thấy menu | F081-AC-20 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-contract
make web-lint && make web-test
pnpm --filter portal exec playwright test tests/e2e/school-lms/school-events.spec.ts tests/e2e/school-lms/school-events-common.spec.ts
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), trace_id của bước 3 và 9, ảnh màn hình danh sách/tiến độ (sáng/tối), báo cáo axe.

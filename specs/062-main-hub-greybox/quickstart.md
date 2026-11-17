# Quickstart: F062 — Main Hub greybox, character controller, `QualityManager`, tab "Nhà trường giao"

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F060, F061 (bắt buộc); F041 (cho nghiệm thu Done — trước đó dùng profile `mock` của F060, nhãn MOCK).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); Godot chạy `-- --env=local` (backend thật) hoặc `-- --env=mock` (Prism).
- Tài khoản demo (mật khẩu `Demo@12345`): `hs.an.3a`, `hs.binh.3a` (th-demo-a/3A), `hs.dung.7a` (pgd-demo), `qlcm.truong.a`, `ad.truong.a`.
- Dữ liệu: `ad.truong.a` đã giao 1 bài activity và 1 đề cho lớp 3A (F055/F041 hoặc seed module learning).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Xóa `user://settings.cfg`, đăng nhập `hs.an.3a` | Benchmark ≤ 5 s, preset được chọn, vào Hub tại `main_hub.default` | F062-AC-14, F062-AC-01 |
| 2 | Đi bằng WASD/joystick, nhảy, xoay camera | Animation idle/walk/run/jump đúng; không giật | F062-AC-02, F062-AC-03 |
| 3 | Đến bảng `board.school_assignments`, nhấn E/chạm | Nút hành động hiện; mở tab "Nhà trường giao" | F062-AC-04, F062-AC-07 |
| 4 | Xem danh sách | 2 bài của 3A, hạn theo locale, trạng thái icon + chữ; không thấy bài của 7A | F062-AC-08, F062-AC-11 |
| 5 | Bấm "Bắt đầu" bài activity rồi bài đề (trước F063/F064) | Thông báo cần cập nhật, không crash; telemetry ghi | F062-AC-09, F062-AC-10 |
| 6 | `ad.truong.a` giao thêm bài; học sinh rời panel và quay lại Hub | Bài mới xuất hiện không cần khởi động lại | F062-AC-07 |
| 7 | Đăng nhập `hs.dung.7a` | Chỉ thấy bài của 7A pgd-demo | F062-AC-11, F062-AC-12 |
| 8 | Đổi ngôn ngữ vi ↔ en trong Cài đặt | HUD/tab đổi ngay | F062-AC-06 |
| 9 | Đăng nhập tài khoản có `grade_band = g1_3` | Theme density lớn, bấm-để-đi bật, ẩn minimap | F062-AC-05, F062-AC-02 |
| 10 | Bootstrap mock `break_reminder_minutes = 1` | Hộp nhắc nghỉ sau 1 phút, đóng được | F062-AC-21 |
| 11 | Chạy profile `mock` | Nhãn "MOCK" hiện trên HUD | F062-AC-13 |
| 12 | Trên DEV-AND-MID1 đo khởi động lạnh → Hub và FPS | ≤ `load_time`; FPS p10 ≥ 27 | F062-AC-20, F062-AC-16 |

## 3. Lệnh test tự động

```bash
make godot-test
make design-tokens-check
godot --headless --path godot -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests/perf
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), báo cáo benchmark `main_hub` (draw call, tam giác, frame time CPU), ảnh HUD ở `g1_3`/`g6_7` và `vi`/`en`/`xx-PS`, bảng FPS trên thiết bị lab, trace_id của lời gọi `listMyAssignments`.

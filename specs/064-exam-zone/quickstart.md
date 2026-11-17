# Quickstart: F064 — Exam Zone: thi có giờ theo server, tự nộp, resume, thông báo bài mới

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F042, F063 (và F041, F062).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); Godot `-- --env=local`; toxiproxy giữa client và APISIX cho bước mất mạng.
- Tài khoản demo (mật khẩu `Demo@12345`): `qlcm.truong.a` (tạo đề), `ad.truong.a`, `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b`, `hs.dung.7a`.
- Dữ liệu: `qlcm.truong.a` tạo đề "Phép nhân" 15 phút từ blueprint, giao cho 3A (`result_release = after_close`); một đề practice `immediate` cho 3A.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` ở Hub; `qlcm.truong.a` giao đề; học sinh đóng một panel → quay lại Hub | Toast bài mới + badge 🔔; chạm mở tab "Nhà trường giao" | F064-AC-20, F064-AC-22 |
| 2 | Mở đề "Phép nhân" | Màn quy chế đủ thông tin | F064-AC-01 |
| 3 | Bắt đầu, trả lời vài câu, đổi đáp án | Đồng hồ khớp server ± 1 s; không đúng/sai; mỗi lần đổi gọi `saveExamAnswer` | F064-AC-02, F064-AC-03, F064-AC-05 |
| 4 | Đánh dấu câu 4, bấm Nộp ở `hs.binh.3a` | Hộp xác nhận liệt kê câu chưa làm/đánh dấu; nộp thành công | F064-AC-07 |
| 5 | `hs.chi.3b` mở đề của 3A | `EXAM_NOT_ASSIGNED`, runner không mở | F064-AC-09 |
| 6 | `hs.an.3a`: cắt mạng 2 phút, trả lời 3 câu, nối mạng | Banner; đồng hồ vẫn chạy; 3 câu đồng bộ | F064-AC-14 |
| 7 | Tắt game, mở lại sau 3 phút, vào lại đề | Làm tiếp đúng câu, thời gian còn lại đúng | F064-AC-15 |
| 8 | Để hết giờ không nộp | Khóa nhập, "bài đã được nộp tự động"; server `submit_reason = auto_timeout` | F064-AC-12 |
| 9 | Xem kết quả trước khi đề đóng | `ui.exam.results_pending` | F064-AC-17 |
| 10 | Đề practice `immediate`: nộp → xem kết quả → "Luyện LO này" | Kết quả theo LO, câu sai; mở activity player | F064-AC-18, F064-AC-19 |
| 11 | `hs.dung.7a` gọi `getExamResult` với attempt của 3A | 404, không hiển thị | F064-AC-10 |

## 3. Lệnh test tự động

```bash
make godot-test
godot --headless --path src/game -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests/integration/exam
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), video kịch bản 6–8, log đo lệch đồng hồ, trace_id của start/save/submit, ảnh màn kết quả ở `g1_3` và `g6_7`.

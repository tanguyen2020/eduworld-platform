# Quickstart: F063 — Activity player: renderer registry, renderer 11 loại câu hỏi, gợi ý, kết quả, công thức SVG

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F060, F061, F062; F030, F032 cho nghiệm thu Done (trước đó profile `mock`, nhãn MOCK).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); Godot `-- --env=local`.
- Tài khoản demo (mật khẩu `Demo@12345`): `hs.an.3a`, `hs.chi.3b` (th-demo-a), `hs.dung.7a` (pgd-demo), `ad.truong.a`, `qlcm.truong.a`.
- Dữ liệu: seed module learning có activity `quiz` đủ 11 dạng câu (có công thức, ảnh, audio, câu có 2 cấp gợi ý, phương án sai có `feedback`); `ad.truong.a` giao activity cho 3A với `result_release = manual`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` mở activity practice từ tab "Nhà trường giao"/thư viện | Khung player mở, thế giới 3D tạm dừng, renderer `quiz` | F063-AC-01, F063-AC-09 |
| 2 | Trả lời lần lượt 11 dạng câu trên PC rồi trên Android (chạm-chạm ghép cặp/kéo thả) | Mọi dạng gửi được; server chấm | F063-AC-02, F063-AC-03, F063-AC-05 |
| 3 | Câu có công thức + ảnh + audio | SVG sắc, đúng màu Theme; audio phát | F063-AC-04, F063-AC-18, F063-AC-19 |
| 4 | Bấm Gợi ý 3 lần ở câu có 2 cấp | Cấp 1, 2 hiện; lần 3 nút mờ | F063-AC-15, F063-AC-16 |
| 5 | Chọn phương án sai có feedback | Dải phản hồi hiện feedback của phương án + nút ví dụ mẫu | F063-AC-14, F063-AC-17 |
| 6 | Tắt mạng, trả lời 1 câu, bật mạng | Banner offline; câu gửi lại tự động | F063-AC-06 |
| 7 | Thoát ở câu 3, mở lại | Resume đúng câu 3, câu trả lời cũ còn | F063-AC-07 |
| 8 | Nộp; gọi lại submit (mô phỏng retry) | Kết quả hiển thị; 409 coi như thành công | F063-AC-08, F063-AC-21 |
| 9 | Làm bài giao `result_release = manual` | "Kết quả sẽ có khi nhà trường công bố" | F063-AC-23 |
| 10 | Đổi sang `en` khi activity chỉ có bản `vi` | Bản `vi` + nhãn "Chưa có bản tiếng Anh" | F063-AC-13 |
| 11 | Dùng `attempt_id` của `hs.dung.7a` khi đăng nhập `hs.an.3a` | 404, không hiển thị dữ liệu | F063-AC-11 |
| 12 | Build thiếu renderer `simulation`, mở activity `simulation` | Màn cần cập nhật, có telemetry | F063-AC-25 |

## 3. Lệnh test tự động

```bash
make godot-test
godot --headless --path src/game -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests/contract/learning
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), video/ảnh 11 dạng câu trên PC + mobile, ảnh công thức ở 2 DPI, trace_id của start/respond/submit, báo cáo FPS activity player trên DEV-AND-MID1.

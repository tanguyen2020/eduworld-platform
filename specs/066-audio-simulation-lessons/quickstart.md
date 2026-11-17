# Quickstart: F066 — Bài nghe và mô phỏng: plugin `audio_lesson`, `simulation` + checker Go, template Godot

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F030, F031, F063, F065, F027, F058.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); Godot chạy `res://scenes/bootstrap/bootstrap.tscn`; web `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm dev`.
- Tài khoản demo: `hs.an.3a` (học sinh `th-demo-a`), `hs.dung.7a` (tenant `pgd-demo`, thử cô lập), `cm.noidung` (content_admin), `cm.duyet` (content_reviewer), `ad.truong.a` (thử route người chơi).
- Dữ liệu: seed bài nghe Tiếng Anh 3 "My name is Mai" (4 câu: 2 `single_choice`, 1 `ordering`, 1 `fill_in_blank`, `max_plays = 2`, transcript) và mô phỏng `physics.lever` v1 (khối lượng 2 kg/1 kg, mục tiêu "cân bằng đòn bẩy", `max_runs = 5`), đã publish và gán cho `th-demo-a`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `cm.noidung` lưu bài nghe có đoạn chồng nhau; publish bài nghe thiếu transcript | `422 ACTIVITY_CONFIG_INVALID` chỉ đúng trường | F066-AC-01 |
| 2 | `hs.an.3a` mở bài nghe trong Godot; xem payload (log debug) | Có URL Ogg, câu không đáp án, `plays_used = 0`; transcript ẩn | F066-AC-02, F066-AC-03 |
| 3 | Nghe hết 2 lượt, bấm phát lần 3 | Nút mờ "Em đã nghe đủ số lượt"; API `422 AUDIO_PLAY_LIMIT_REACHED`; vẫn trả lời được | F066-AC-04, F066-AC-18, F066-AC-19 |
| 4 | Câu nghe chép gõ "  da  nang " | Chấm đúng theo chuẩn hóa QUESTION_BANK §4 | F066-AC-05 |
| 5 | Thoát giữa bài, mở lại trên thiết bị khác | Tiếp tục đúng vị trí, `plays_used` giữ, câu đã trả lời giữ | F066-AC-07 |
| 6 | Nộp; gửi lại cùng key; gửi key khác | Hoàn thành; cùng kết quả; `409 ATTEMPT_ALREADY_SUBMITTED` | F066-AC-06 |
| 7 | Quét OpenAPI/AsyncAPI + network Godot; tạo activity `speaking` qua API | Không endpoint ghi âm, không xin micro; `422 ACTIVITY_TYPE_NOT_SUPPORTED` | F066-AC-08, F066-AC-20 |
| 8 | `cm.noidung` lưu mô phỏng có tham số ngoài khoảng / thêm trường `script` | `422 ACTIVITY_CONFIG_INVALID` | F066-AC-09 |
| 9 | `hs.an.3a` mở mô phỏng; xem payload | Có `template_code`, `seed`, `goals` chỉ `goal_id` + mô tả, không `check` | F066-AC-10, F066-AC-21 |
| 10 | Đặt vật cân bằng, bấm Thả | "Đạt mục tiêu!" sau khi server trả `verified`, `goals_met` đạt; learning event `simulation.goal_met`; outbox `simulation.run_verified` | F066-AC-11, F066-AC-22, F066-AC-23 |
| 11 | Postman gửi log không cân bằng + `client_outcome` "đạt" | `goal_met = false`; integrity signal `simulation_outcome_mismatch` | F066-AC-12 |
| 12 | Postman gửi log 501 hành động / `t_ms` giảm / seed khác | `422 SIMULATION_LOG_INVALID`; run `rejected`; Godot hiện "Thí nghiệm chưa ghi đúng" | F066-AC-13, F066-AC-23 |
| 13 | Chạy 5 run, gửi run thứ 6; gửi lại run 3 cùng key | `422 SIMULATION_RUN_LIMIT_REACHED`; run 3 trả kết quả cũ | F066-AC-14 |
| 14 | Template quan sát (`checker_available = false`, fixture test) | Run `not_verifiable`, không evidence từ mô phỏng | F066-AC-15 |
| 15 | Trả lời câu kết luận, nộp | Điểm = câu hỏi + mục tiêu theo run verified cuối; evidence LO | F066-AC-16 |
| 16 | `make test MOD=learning` (vector) và `make godot-test` (vector) | Cùng kết quả mọi vector; checker p95 trong mục tiêu | F066-AC-17, F066-AC-24 |
| 17 | Godot Web export, preset Thấp, mở mô phỏng | Bản giản lược, ≥ 30 FPS, nạp ≤ 3 s | F066-AC-24 |
| 18 | Godot bản cũ không có template | Màn "Cần cập nhật game", không crash | F066-AC-21 |
| 19 | `cm.noidung` soạn bài nghe qua Activity Builder | Chọn audio/transcript, đoạn, câu theo mốc, lượt nghe; lỗi hiển thị theo trường | F066-AC-25 |
| 20 | `cm.noidung` cấu hình mô phỏng; đăng nhập role không có `learning_content.activity.write` | Form từ `parameter_schema`, không ô script; role kia không có nút tạo, API `403` | F066-AC-26 |
| 21 | `hs.dung.7a` gọi attempt/run của `hs.an.3a`; `ad.truong.a` gọi route người chơi | `404`; `403 GAME_ACCESS_STUDENT_ONLY` | F066-AC-27 |
| 22 | Gửi `action` 65 ký tự, `args` 21 thuộc tính | `422 VALIDATION_FAILED` | F066-AC-28 |
| 23 | `sa.admin` tắt `menu.content_studio.simulations` cho `th-demo-a`; `cm.noidung` tạo activity mô phỏng | Menu "Mô phỏng" biến mất; ô chọn template mô phỏng báo "Tính năng đang tạm tắt" (`listSimulationTemplates` → `403 FEATURE_DISABLED`); editor audio vẫn dùng được | F066-AC-29 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make test MOD=learning
make test-integration MOD=learning
make migrate-lint MOD=learning
make godot-test
cd src/cms/frontend && pnpm --filter portal test -- src/workspaces/content-studio/activities/editors
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), báo cáo vector chung Go + GdUnit4, `trace_id` kịch bản 3, 10, 12, số đo FPS/thời gian nạp kịch bản 17, ảnh màn hình bài nghe, mô phỏng và editor.

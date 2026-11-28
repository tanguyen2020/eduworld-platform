# Quickstart: F135 — Học sinh tự đăng ký trong game vào tenant chung (bước phụ huynh, mã khôi phục, chống lạm dụng)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc: F010, F012, F013, F016, F045, F057, F061, F124, F125 (F136 nếu muốn gõ nickname ở S02).
- Lệnh: `make up`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); Mailpit/SMTP local của stack để xem thư; mở `src/game/` bằng Godot 4.x, chạy `res://scenes/bootstrap/bootstrap.tscn` với `--device-id` mới cho mỗi kịch bản.
- Tài khoản: `sa.platform` (System Admin, portal), `ad.truong.a` (School Admin tenant khác, để kiểm phân quyền), một học sinh tenant `common` do System Admin tạo (đối chứng).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Mở game, S01 | Có nút "Tạo tài khoản"; tắt `self_registration_enabled` → nút ẩn | F135-AC-01, F135-AC-25 |
| 2 | Đăng ký khối 4, ngày sinh hợp lý, tên `lan.choi`, không email, đánh dấu đồng ý | `201`, màn mã khôi phục, vào S02; DB có bằng chứng, không họ tên | F135-AC-02…AC-06, AC-10, AC-11, AC-16 |
| 3 | Đăng ký khối 1 với năm sinh của học sinh 12 tuổi; tên `hs.abc`; bỏ ô đồng ý; gửi thêm `full_name` bằng curl | `422` theo field; không có user Keycloak mới | F135-AC-07 |
| 4 | Đăng ký lại tên `Lan.Choi` | "Tên đăng nhập đã có người dùng" | F135-AC-08 |
| 5 | Đặt đồng hồ server 22:00 ICT, đăng ký | `CurfewNotice` | F135-AC-09 |
| 6 | Đăng ký có email phụ huynh; mở thư trong Mailpit, bấm link; bấm lại | "Cảm ơn phụ huynh…"; lần 2 "Link đã hết hạn" | F135-AC-12, AC-13, AC-14 |
| 7 | Giả lỗi DB sau khi tạo user Keycloak (toxiproxy) | `503`; user Keycloak `enabled = false`; gửi lại cùng key → hoàn tất | F135-AC-05 |
| 8 | "Quên mật khẩu?" → mã đúng | Mật khẩu mới, mã mới; phiên cũ trên thiết bị khác bị đăng xuất | F135-AC-18, AC-21 |
| 9 | Mã sai 6 lần | 5 lần `401` chung, lần 6 `429`; đăng nhập bằng mật khẩu đúng vẫn được | F135-AC-19 |
| 10 | "Gửi link cho phụ huynh" với tài khoản có/không email đã xác nhận | Cả hai `202`; chỉ tài khoản có email nhận thư; trang đặt lại đổi được mật khẩu | F135-AC-20 |
| 11 | Một `device_id` đăng ký 4 lần; 40 `device_id` cùng IP | Lần 4 `429 REGISTRATION_LIMIT_REACHED`; 40 lần `201` | F135-AC-22, SC-005 |
| 12 | Đặt `self_registration_global_hourly_limit` = 2 (test) | Lần 3 `429 REGISTRATION_BUSY` | F135-AC-23 |
| 13 | Gửi lại cùng `Idempotency-Key`; khác body | Replay không có mã; khác body `409` | F135-AC-17, AC-24 |
| 14 | So quyền học sinh tự đăng ký với học sinh tenant chung đối chứng | Cùng kết quả (chat, bạn bè, club, ghép trận, quest, luyện tập); "Nhà trường giao" rỗng thân thiện | F135-AC-27, AC-28 |
| 15 | `sa.platform` lọc "Tự đăng ký", xem bằng chứng, đặt lại mật khẩu, khóa | Nhãn đúng; email che; mã khôi phục mới; audit; `ad.truong.a` → `404`/`403` | F135-AC-31…AC-33 |
| 16 | Gộp tài khoản tự đăng ký vào tài khoản trường (saga F045) | XP, vật phẩm, quest, attempts ở tài khoản đích | F135-AC-29 |
| 17 | Chạy dọn học sinh với ngày giả; ẩn danh hóa | Dọn như học sinh tenant chung; ẩn danh xóa ngày sinh, email, mã | F135-AC-30 |
| 18 | Điện thoại 844×390, 740×360; `en`, pseudo | Bố cục theo F125, không tràn chữ | F135-AC-34…AC-36 |
| 19 | Quét log APISIX/identity/client, `user://` | Không mật khẩu, mã, ngày sinh, email | F135-AC-26, AC-37 |
| 20 | Đăng nhập game bằng tài khoản nhân sự | `GAME_ACCESS_STUDENT_ONLY` như cũ | F135-AC-38 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=identity && make lint MOD=identity
make test MOD=identity && make test-integration MOD=identity   # testcontainers PostgreSQL + Keycloak
make migrate-lint MOD=identity
make specs-lint && make specs-breaking
make godot-test && make godot-export-check
make web-lint && make web-test
```

## 4. Bằng chứng cần nộp

- Số pass/fail Go unit/integration, GdUnit4, Vitest/Playwright; ma trận khối + ảnh trong `screenshots/` (F135-AC-36) với bảng màn × khối đã xem; kết quả kịch bản §2 với stack thật (ghi rõ kịch bản chỉ chạy với fake).

## 5. Bằng chứng

Chưa có — ghi khi triển khai (T045–T048).

# Quickstart: F136 — Nickname tự gõ qua kiểm duyệt (giữ "Gợi ý"), báo cáo và đặt lại nickname

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc Done: F070 (GameProfile, gợi ý nickname), F116 (pipeline + `moderateDisplayText`), F121 (case, sanction), F052 (trang học sinh portal), F122 (hàng đợi case); F125 (`compact`).
- Lệnh: `make up`, `make seed`, `make up-app`; mở `src/game/` bằng Godot 4.x, chạy `res://scenes/bootstrap/bootstrap.tscn` hai instance với `--device-id` khác nhau ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Danh mục từ nhạy cảm test: chỉ placeholder (ví dụ `tu_cam_mau_01`) nạp qua seed danh mục của F116 — không dùng từ thật (R15).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (TH Demo A), `hs.dung.7a` (tenant khác), `ad.truong.a` (School Admin TH Demo A), một `safety_moderator` platform; một học sinh chưa có nickname (tạo mới hoặc đặt lại).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Học sinh mới đến bước Tên S02 | Ô gõ có bộ đếm, dòng "Đừng dùng tên thật của em", chip gợi ý + "Gợi ý khác"; lớp 1–3 chạm chip là xong | F136-AC-01 |
| 2 | Gõ "Mn" rồi "Siêu Nhân 123456" | Nút "Tiếp" mờ kèm gợi ý độ dài/chữ số; không gọi API | F136-AC-02 |
| 3 | Gõ "Siêu Nhân Sao Băng", xác nhận | `updateGameProfile {nickname}` `200`; Mây chào bằng tên mới; `game_profile.updated` | F136-AC-03, F136-AC-04 |
| 4 | Gửi body có cả `nickname` và `nickname_option_id` (curl) | `422 VALIDATION_FAILED` | F136-AC-03 |
| 5 | Chạm chip rồi sửa một chữ | Gửi `nickname` tự gõ, không gửi option id | F136-AC-06 |
| 6 | Gõ lần lượt: "Minh 0912 345 678", "mail a còng gmail chấm com", "zalo minhxinh", "@minh_123", placeholder danh mục, họ tên thật của em, tên trường của em | Mỗi lần: `422 NICKNAME_NOT_ALLOWED` cùng một câu, chuỗi giữ trong ô, không nêu lý do | F136-AC-05, F136-AC-10…F136-AC-12, F136-AC-16 |
| 7 | `hs.binh.3a` gõ "siêu nhân sao băng" | `409 NICKNAME_UNAVAILABLE` "Tên này đã có bạn dùng rồi" | F136-AC-13 |
| 8 | Dừng container của chat (hoặc fake trả lỗi) rồi gõ tên | `503`, không lưu; chọn chip gợi ý vẫn được | F136-AC-14 |
| 9 | Gửi 11 tên trong 10 phút | Lần 11 `429` + `Retry-After`; không có sanction | F136-AC-15 |
| 10 | Cài đặt → Hồ sơ của em → Nickname: đổi lần 1 rồi lần 2 trong tháng | Lần 1 `200`; lần 2 khóa ô với ngày đổi lại được (không đếm ngược) | F136-AC-07, F136-AC-08 |
| 11 | Client thứ hai trong Hub quan sát | Nhãn trên đầu, thẻ người chơi đổi sang tên mới | F136-AC-09 |
| 12 | `hs.binh.3a` chạm nhân vật `hs.an.3a` → Báo cáo → "Tên không phù hợp" | `201`, lời cảm ơn; case có bản chụp nickname phía server | F136-AC-17, F136-AC-18 |
| 13 | `safety_moderator` chọn "Đặt lại nickname" trên case | Sanction `nickname_reset`; game đặt `nickname = NULL`; người khác thấy "Bạn mới"; `hs.an.3a` vào lại chỉ làm bước Tên | F136-AC-19, F136-AC-20, F136-AC-23 |
| 14 | `ad.truong.a` mở trang học sinh `hs.binh.3a` → "Đặt lại nickname" với lý do; thử với học sinh trường khác; `hs.binh.3a` tự gọi API | `200` + audit; trường khác `404`; học sinh `403`; gửi lại cùng key → cùng kết quả | F136-AC-21, F136-AC-22 |
| 15 | Pseudo-locale `xx-PS`, `en`; lint theme | Không tràn chữ; không màu/cỡ chữ hardcode | F136-AC-24, F136-AC-25 |
| 16 | Điện thoại 844×390 và 740×360, mở bàn phím ảo ở bước Tên | Ô gõ nằm trên bàn phím, nút ≥ ngưỡng | F136-AC-25, F136-AC-26 |
| 17 | Đặt nickname "[b]An[/b] Vui" (nếu qua kiểm tra định dạng thì bị chặn vì ký tự) và quét mã hiển thị | Ký tự `[`/`]` bị chặn ở định dạng; mọi nơi hiển thị nickname là văn bản thuần | F136-AC-27 |
| 18 | `hs.dung.7a` (tenant khác) dùng "Siêu Nhân Sao Băng"; gửi `X-Tenant-Id` sai; token nhân sự gọi `updateGameProfile` | Tenant khác được phép; `403 TENANT_MISMATCH`; `403 GAME_ACCESS_STUDENT_ONLY` | F136-AC-28, F136-AC-29 |
| 19 | Xem log/trace sau bước 6 | Không có chuỗi bị từ chối; audit chỉ mã kết quả | F136-AC-30 |
| 20 | Ẩn danh hóa học sinh có lịch sử đổi tên | `game_profiles` và `nickname_changes` không còn nickname cũ | F136-AC-31 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=game && make gen-check MOD=chat && make gen-check MOD=identity
make test MOD=game && make test-integration MOD=game
make test MOD=chat && make test-integration MOD=chat
make test MOD=identity && make test-integration MOD=identity
make test-contract && make arch-check
make specs-lint && make specs-breaking
make godot-test && make godot-export-check
make web-lint && make web-test
```

## 4. Bằng chứng cần nộp

- Số pass/fail của từng lệnh §3 (lưu ý giới hạn CPU: tối đa 2 shard integration), bảng AC → test ở plan.md đã tick, ảnh ma trận khối trong `screenshots/` theo F136-AC-26 với bảng màn × khối đã xem từng ảnh, kịch bản §2 chạy với backend thật (ghi rõ bước nào chỉ chạy với fake).

## 5. Bằng chứng

Chưa có — ghi khi triển khai (T041–T044).

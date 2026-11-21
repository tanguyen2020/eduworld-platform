# Quickstart: F123 — Thông báo trong game: realtime, lưu bền vững, không push

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F090, F092, F115, F095, F041 (F100, F074 cho nguồn party/phần thưởng).
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=chat`; Godot chạy client với backend compose.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (`th-demo-a`), `hs.dung.7a` (`pgd-demo`), `qlcm.truong.a` (giao bài môn Toán), `ad.truong.a`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.binh.3a` trong game; `hs.an.3a` gửi lời mời kết bạn (F095) | `hs.binh.3a` nhận `notification.item.created` (`kind=friend_request`) ≤ 2 s, badge +1, toast hiện | F123-AC-01, F123-AC-04, F123-AC-15, F123-AC-16 |
| 2 | Phát lại event `friend_request.sent` cùng `event_id` qua NATS | Không có thông báo thứ hai | F123-AC-03 |
| 3 | `qlcm.truong.a` giao bài Toán cho khối 3 của TH Demo A, exclude `hs.binh.3a` (F041) | `hs.an.3a` có thông báo `assignment_new`; `hs.binh.3a` không có | F123-AC-02 |
| 4 | Dừng module learning khi giao bài cho danh sách học sinh gồm `hs.chi.3b` | Consumer thử lại, không tạo thông báo thiếu; bật lại → `hs.chi.3b` nhận | F123-AC-07 |
| 5 | Kiểm `params` các thông báo trên trong DB và WS | Chỉ nickname, tên bài, ID; không tên thật/trường/khối | F123-AC-05 |
| 6 | Phát `assignment.published` với `tenant_id` của `pgd-demo` nhưng đích khối 3 của TH Demo A | Không thông báo nào cho học sinh `th-demo-a` | F123-AC-06 |
| 7 | `hs.an.3a` `listNotifications?unread=true`; `hs.dung.7a` gọi `markNotificationRead` với id của `hs.an.3a` | Danh sách đúng + `unread_count`; `404` | F123-AC-08, F123-AC-09 |
| 8 | `hs.an.3a` mở inbox (Godot) và bấm thông báo bài giao | `notification.item.mark_read` gửi; mở tab "Nhà trường giao"; `unread_count` giảm | F123-AC-09, F123-AC-18 |
| 9 | `markAllNotificationsRead` | `unread_count = 0` | F123-AC-10 |
| 10 | Gọi `listNotifications` thiếu `X-Tenant-Id`; header `pgd-demo` với token `th-demo-a` | `400 TENANT_HEADER_REQUIRED`; `403 TENANT_MISMATCH` | F123-AC-11 |
| 11 | Chạy `scripts/ci/check-no-push-sdk.sh`; tìm endpoint đăng ký token push trong OpenAPI | Không có SDK/endpoint push | F123-AC-13, F123-AC-14 |
| 12 | Tắt popup trong Cài đặt, tạo lời mời; vào bài kiểm tra rồi tạo lời mời | Chỉ badge, không toast | F123-AC-16 |
| 13 | Đổi ngôn ngữ `en-US`, mở inbox | Nội dung theo `ui.notification.<code>` tiếng Anh | F123-AC-17, F123-AC-19 |

## 3. Lệnh test tự động

```bash
make test MOD=chat
make test-integration MOD=chat
make test-contract                 # chat ↔ learning listAssignmentRecipients, payload event
make godot-test                    # notifications unit + scene
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), đo độ trễ kịch bản 1 (p95), truy vấn chứng minh không trùng (kịch bản 2), ảnh HUD badge/toast/inbox, output `check-no-push-sdk.sh`.

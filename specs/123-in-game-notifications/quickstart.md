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

Phần Godot (F123 T024–T028, chạy local 2026-11-27, Godot 4.7.2 headless):

| Lệnh | Kết quả |
|---|---|
| GdUnit4 `res://tests/unit/notifications` | 24/24 pass (`test_notification_service.gd`: F123-AC-15, AC-16 phía service, AC-17, AC-18; F064-AC-21 giữ) |
| GdUnit4 `res://tests/scene/notifications` | 12/12 pass (`test_notification_ui.gd`: chuông + `CountBadge`, toast, chỉ badge khi tắt popup/lên cấp, ≤ 3 toast, intent, hộp thông báo, rỗng/lỗi, đổi ngôn ngữ, tab Cài đặt, Theme/khóa — F123-AC-15…19) |
| GdUnit4 `res://tests/scene/test_grade_band_matrix.gd` | 27/27 pass (thêm `hud_notifications`, `notification_inbox`, `notification_inbox_empty`, `settings_notifications` × 4 khối × 1280×720 / 1024×600 / 844×390, cả cỡ chữ +2 cho tab Cài đặt) |
| GdUnit4 `res://tests/integration/test_notifications_e2e.gd` | bỏ qua (cần backend compose, `EDUWORLD_IT_API_BASE_URL`) — như các test live khác |
| GdUnit4 toàn bộ `res://tests` | 1135 test case — 1123 pass, 0 lỗi, 0 fail, 12 bỏ qua (test live cần backend); `scan_artifact_test` 22/22; export Web + `check_web_export` + `scan_artifact` OK |

## 5. Ảnh trước/sau và kết quả xem theo khối (phần Godot)

Renderer Compatibility (= Web), `tools/capture_screens.gd`, fixture + fake (`NotificationFixtures`, không backend). Ảnh ở [screenshots/](screenshots/) theo mẫu `<màn>-{before,after}-<cỡ>-<ngôn ngữ>-<khối>.png`: sau — `hub`, `hub_notifications`, `notifications`, `notifications_empty`, `settings_notifications` × 4 khối × 1280×720 và 844×390 `vi` + `en` `g4_5` 1280×720; trước (main trước F123) — `hub`, `settings`, `settings_access` × `g1_3`, `g8_9` 1280×720. Đã xem từng ảnh.

| Màn  khối | g1_3 | g4_5 | g6_7 | g8_9 |
|---|---|---|---|---|
| HUD (`hub`): chuông có viên thuốc số chưa đọc | Đạt — "2" trong viên thuốc xanh bên phải icon, nút rộng ra, không đè icon | Đạt | Đạt | Đạt — 844×390 số nhỏ nhưng đọc được |
| HUD có thông báo (`hub_notifications`): 3 toast + số | Đạt — icon chuông trắng + chữ + mũi tên, toast dài xuống 2 dòng, không che nút | Đạt (en: "School tasks waiting for you: 1" sau khi sửa số nhiều) | Đạt | Đạt |
| Hộp thông báo (`notifications`) | Đạt — 1280×720 dòng thứ 5 cần cuộn (thanh cuộn hiện), nhãn "MỚI" rõ | Đạt — 5 dòng vừa khung | Đạt | Đạt |
| Hộp rỗng (`notifications_empty`) | Đạt — Mây + bong bóng ôm vừa chữ (đã sửa bong bóng cao bằng Mây) | Đạt | Đạt | Đạt |
| Cài đặt → Thông báo (`settings_notifications`) | Đạt — 6 tab + Đăng xuất vừa cột sau khi đổi cột tab sang `FieldBox` (trước đó "Thông tin" bị cắt, phải cuộn) | Đạt (en: "Notifications", "Show pop-up notices while playing") | Đạt | Đạt |

Sửa sau khi xem ảnh: icon ✓ trắng của dòng "kết quả" không thấy trên thẻ sáng → đổi sang `tick` (màu nhấn); bong bóng Mây ở hộp rỗng co theo chữ; cột tab Cài đặt `g1_3` không còn cắt tab cuối; câu tiếng Anh "You have 1 school tasks" → "School tasks waiting for you: 1".

**T031 (quickstart end-to-end với full stack):** chưa chạy — stack local của owner đang chạy và kịch bản cần tạo lời mời kết bạn/giao bài thật trên dữ liệu seed; để không làm xáo trộn stack, T031 để mở. Test `test_notifications_e2e.gd` sẵn sàng chạy khi có `EDUWORLD_IT_API_BASE_URL`.

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), đo độ trễ kịch bản 1 (p95), truy vấn chứng minh không trùng (kịch bản 2), ảnh HUD badge/toast/inbox, output `check-no-push-sdk.sh`.

# Quickstart: F111 — Godot — club trong game, Event center, nhóm "Bạn cùng lớp"/"Vừa chơi cùng", nút "Tham gia bạn"

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F077, F095, F120, F091/F092, F071, F072, F110, F119 (cho nghiệm thu Done; trước đó dùng profile `mock` của F060, nhãn MOCK).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); hai instance Godot `-- --env=local --device-id=<khác nhau>`.
- Tài khoản demo (mật khẩu seed): `hs.an.3a` (A), `hs.binh.3a` (B) cùng lớp 3A, đã là bạn (fixture `friend.added`); `hs.chi.3b` (C, khác lớp cùng trường TH Demo A); `hs.dung.7a` (D, THCS Demo B, tenant `pgd-demo`); `hs.em.4a` (E, TH Demo C, cùng tenant `unit` với D); `ad.truong.a`, `ad.truong.b` (school_admin).
- Seed (dữ liệu mẫu): sự kiện `world_festival` "Lễ hội Trăng Rằm" `live` cho TH Demo A có community goal và 2 mốc; season track mùa hiện tại; THCS Demo B đặt `student_club_creation = requires_approval`; TH Demo A giữ `disabled` và có club `school_club` "CLB Toán" do `ad.truong.a` tạo, thêm cả lớp 3A.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | B vào `learning_zone.math`; A ở `main_hub` mở Bạn bè, chọn B, bấm "Tham gia bạn" | A tải khu Toán, cùng `room_id` với B, toast "Em đã đến chỗ Bình" | F111-AC-01, F111-AC-03 |
| 2 | B vào trận Arena; A mở hồ sơ B | Nút "Tham gia bạn" xám + lý do bằng chữ | F111-AC-02 |
| 3 | Lấp đầy instance của B tới `hub_instance_capacity` bằng bot; A bấm "Tham gia bạn" | A vào instance khác cùng khu, thông điệp "Khu vực của bạn đã đầy", không lộ instance | F111-AC-04 |
| 4 | B ở `challenge_mountain` (A chưa mở khu); A bấm "Tham gia bạn" | A ở lại khu cũ, thấy điều kiện mở khóa; không gửi `hub.instance.join`; nút khóa khi đang chờ | F111-AC-05 |
| 5 | A mở tab Tìm bạn | Nhóm "Ở gần đây", "Bạn cùng lớp" (không có B vì đã là bạn), "Vừa chơi cùng", Mã bạn bè; không có ô tìm kiếm; chỉ nickname/avatar/level | F111-AC-06, F111-AC-07 |
| 6 | A và C chơi một party, A mở lại Tìm bạn; giả lập lỗi `listFriendSuggestions` | C có trong "Vừa chơi cùng" + biểu tượng party; khi lỗi nhóm ẩn, Mã bạn bè còn | F111-AC-07, F111-AC-08 |
| 7 | A bấm biểu tượng lịch trên HUD | Thẻ "Lễ hội Trăng Rằm · còn n ngày", không đếm giây; D (tenant khác) không thấy sự kiện | F111-AC-09 |
| 8 | A mở chi tiết, bấm "Tham gia", làm quest sự kiện, nhận mốc 1 (bấm hai lần khi mạng chậm) | Tham gia một lần; điểm tăng theo `event.progress.updated`; mốc nhận đúng một lần, hiện phần thưởng server | F111-AC-10, F111-AC-11, F111-AC-14 |
| 9 | Vận hành dừng khẩn cấp run (F079), rồi tiếp tục, rồi kết thúc | Thẻ "Sự kiện tạm dừng" → bình thường → "Phần thưởng chưa nhận sẽ được gửi tự động" | F111-AC-12 |
| 10 | Đóng góp vào community goal từ hai học sinh | Thanh tổng/mục tiêu cập nhật, "Em đã góp sức"; không xếp hạng; A (`g1_3`) không thấy `by_class` | F111-AC-13 |
| 11 | A mở tab Hôm nay, làm xong 3 nhiệm vụ ngày | Nút "Đi" dẫn đường; thông điệp nghỉ ngơi, không gợi ý chơi thêm; A chỉ có tab Hôm nay, Sự kiện | F111-AC-15, F111-AC-18 |
| 12 | D mở tab Mùa, nhận thưởng cấp 1 hai lần cùng key, xem Mùa trước | Một lần cấp; không có track trả phí/giá; thông điệp tự gửi khi hết mùa | F111-AC-16, F111-AC-17 |
| 13 | A mở tab Club | Thấy "CLB Toán" (club của em), không có club đề xuất, không có "Tạo club"; trang club chỉ nickname/vai trò, mục tiêu tập thể + "Em đã góp …" | F111-AC-19, F111-AC-20 |
| 14 | D mở tab Club, tạo club `interest_group` tên 1 ký tự rồi tên hợp lệ; tên chứa từ nhạy cảm (fixture placeholder) | Tên 1 ký tự bị chặn ở form; tên hợp lệ → "Club đang chờ nhà trường duyệt"; từ nhạy cảm → thông điệp chung, không echo | F111-AC-22, F111-AC-23 |
| 15 | `ad.truong.b` duyệt club ở portal; D mở trang club | Club `active`, nút Chat club mở `ChatDock` channel club | F111-AC-24 |
| 16 | Bật liên trường cho `pgd-demo`; E thấy club của D trong "Club đề xuất", bấm "Xin vào"; D chấp nhận ở tab Yêu cầu; E rời club (xác nhận) | "Đang chờ duyệt" → thành viên → rời | F111-AC-21, F111-AC-25, F111-AC-24 |
| 17 | D và E kết bạn bằng mã; D mời E qua `ClubInvitePicker`; E chấp nhận ở tab Club; profile `mock`: `inviteToClub` trả `SOCIAL_SCOPE_VIOLATION` cho một invitee | Lời mời hiện ở E và chấp nhận được; với invitee lỗi D thấy "Chưa mời được", không nêu lý do | F111-AC-19, F111-AC-21, F111-AC-26 |
| 18 | Đổi `vi`↔`en`, pseudo-locale `xx`, `lint_theme_overrides.gd` | Không cắt chữ; 0 override; trạng thái có chữ | F111-AC-27, F111-AC-28 |
| 19 | Quét scene Event center/Club/Tìm bạn | Không có đếm giây, streak, nhấp nháy, track trả phí; chấm HUD tĩnh | F111-AC-29 |
| 20 | Đăng xuất A; takeover; `ad.truong.a` đăng nhập game; mở club của tenant khác qua deep link giả | Store xóa, không file `user://`; `GAME_ACCESS_STUDENT_ONLY`; `404` → đóng màn | F111-AC-30, F111-AC-31 |
| 21 | `EventCenterBenchmark` trên Android tầm trung | ≥ 30 FPS preset Trung bình | F111-AC-32 |

## 3. Lệnh test tự động

```bash
make godot-test          # GdUnit4 unit + scene + contract + integration headless (2 client)
make godot-export-check
make design-tokens-check
```

## 4. Bằng chứng cần nộp

- Kết quả `make godot-test` (pass/fail), video kịch bản 1–4 trên hai thiết bị, ảnh Event center/Club/Tìm bạn ở `vi`/`en`, PC/mobile, `g1_3`/`g6_7`, số đo FPS kịch bản 21.

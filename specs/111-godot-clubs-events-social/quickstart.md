# Quickstart: F111 — Godot — club trong game, Event center, nhóm "Vừa chơi cùng", nút "Tham gia bạn"

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F077, F095, F120, F091/F092, F071, F072, F110, F119 (cho nghiệm thu Done; trước đó dùng profile `mock` của F060, nhãn MOCK).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); hai instance Godot `-- --env=local --device-id=<khác nhau>`.
- Tài khoản demo (mật khẩu seed): `hs.an.3a` (A), `hs.binh.3a` (B) khối 3 TH Demo A, đã là bạn (fixture `friend.added`); `hs.chi.3b` (C, khối 3 cùng trường TH Demo A, chưa là bạn); `hs.dung.7a` (D, THCS Demo B, tenant `pgd-demo`); `hs.em.4a` (E, TH Demo C, cùng tenant `unit` với D); `ad.truong.a`, `ad.truong.b` (school_admin).
- Seed (dữ liệu mẫu): sự kiện `world_festival` "Lễ hội Trăng Rằm" `live` cho TH Demo A có community goal và 2 mốc; season track mùa hiện tại; THCS Demo B đặt `student_club_creation = requires_approval`; TH Demo A giữ `disabled` và có club `school_club` "CLB Toán" do `ad.truong.a` tạo, thành viên là A và B (danh sách học sinh, không "thêm cả lớp" — ADR-0095).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | B vào `learning_zone.math`; A ở `main_hub` mở Bạn bè, chọn B, bấm "Tham gia bạn" | A tải khu Toán, cùng `room_id` với B, toast "Em đã đến chỗ Bình" | F111-AC-01, F111-AC-03 |
| 2 | B vào trận Arena; A mở hồ sơ B | Nút "Tham gia bạn" xám + lý do bằng chữ | F111-AC-02 |
| 3 | Lấp đầy instance của B tới `hub_instance_capacity` bằng bot; A bấm "Tham gia bạn" | A vào instance khác cùng khu, thông điệp "Khu vực của bạn đã đầy", không lộ instance | F111-AC-04 |
| 4 | B ở `challenge_mountain` (A chưa mở khu); A bấm "Tham gia bạn" | A ở lại khu cũ, thấy điều kiện mở khóa; không gửi `hub.instance.join`; nút khóa khi đang chờ | F111-AC-05 |
| 5 | A mở tab Tìm bạn | Nhóm "Ở gần đây", "Vừa chơi cùng" (rỗng, có gợi ý Mã bạn bè), Mã bạn bè; không có nhóm "Bạn cùng lớp", không có ô tìm kiếm; chỉ nickname/avatar/level | F111-AC-06, F111-AC-07, F111-AC-08 |
| 6 | A và C chơi một party, A mở lại Tìm bạn; giả lập lỗi `listFriendSuggestions` | C có trong "Vừa chơi cùng" + biểu tượng party; khi lỗi nhóm ẩn, Mã bạn bè còn | F111-AC-07, F111-AC-08 |
| 7 | A bấm biểu tượng lịch trên HUD | Thẻ "Lễ hội Trăng Rằm · còn n ngày", không đếm giây; D (tenant khác) không thấy sự kiện | F111-AC-09 |
| 8 | A mở chi tiết, bấm "Tham gia", làm quest sự kiện, nhận mốc 1 (bấm hai lần khi mạng chậm) | Tham gia một lần; điểm tăng theo `event.progress.updated`; mốc nhận đúng một lần, hiện phần thưởng server | F111-AC-10, F111-AC-11, F111-AC-14 |
| 9 | Vận hành dừng khẩn cấp run (F079), rồi tiếp tục, rồi kết thúc | Thẻ "Sự kiện tạm dừng" → bình thường → "Phần thưởng chưa nhận sẽ được gửi tự động" | F111-AC-12 |
| 10 | Đóng góp vào community goal từ hai học sinh (sự kiện bật `show_grade_contributions`) | Thanh tổng/mục tiêu cập nhật, "Em đã góp sức"; không xếp hạng; A (`g1_3`) không thấy `by_grade`; học sinh khối 4+ thấy `by_grade` không tên, khối < 5 học sinh không có dòng | F111-AC-13 |
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

## 5. Bằng chứng 2026-11-28 (phần Godot client)

Chạy trên máy local, Godot 4.7.2 (headless cho GdUnit4, Compatibility/OpenGL cho ảnh), FakeHttp + FakeWebSocketPeer + fixture contract (TEST-ONLY, AGENTS R17) — **chưa** chạy với backend thật. GdUnit4 toàn bộ (chia theo thư mục): unit 970, contract 37, integration 18 (16 skip — cần môi trường thật, có sẵn từ trước), perf 17, tools 11, visual 4, scene 633 → **1690 test, 0 lỗi, 0 fail, 16 skip**. F111: 28 file test, 109 test; viết test trước, ghi đỏ → xanh từng nhóm trong [tasks.md](tasks.md). Lint: `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `bake_art --check`, `brand_assets --check`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check` (273 file), `check_no_admin_ui` — OK; `check-features`, `check-cross-features`, docs `check-links` — 0 lỗi.

Bảng F111-AC → test: [tasks.md](tasks.md#bằng-chứng-f111-ac--test-2026-11-28-gdunit4-headless-fake--fixture-contract).

Ảnh review (`screenshots/`): 19 màn F111 (`f111_<view>`) ở 1280×720, 844×390, 740×360 `vi` `g1_3`; `g6_7` ở 1280×720 và 844×390 cho `clubs`, `club_create`, `events_week`, `events_season`, `events_no_season`, `event_detail` (tab Mùa/Tuần này không có ở lớp 1–3); mẫu `en` ở 1280×720 và 844×390 (`friends_join`, `friends_suggest`, `club_page`, `events_today`, `event_detail`, `events_list`); `emote_icon_only-hub-*` (nút biểu cảm chỉ biểu tượng, yêu cầu chủ sản phẩm). Lệnh: `godot --path src/game --rendering-method gl_compatibility --rendering-driver opengl3 -s res://tools/capture_screens.gd -- --screens=f111_… --sizes=1280x720,844x390,740x360 --locales=vi-VN --band=g1_3`.

Còn chờ: kịch bản §2 với backend thật và hai thiết bị (T044, T046 — kịch bản 1–4, 9, 15–17, 20), đo FPS kịch bản 21 (SC-005).

Polish điện thoại 2026-11-28 (T047): ảnh `screenshots/*-polish-*` — 17 màn F111 ở 1280×720, 844×390, 740×360 `vi` `g1_3`; `en` `g1_3` cho `club_page`, `friends_suggest`, `event_detail`, `events_today`; `g6_7` cho `events_season`, `events_week`, `clubs`, `friends_suggest` — đã xem từng ảnh: đầu trang club một dòng + thành viên hai thẻ một hàng (≥ 3 thành viên thấy ngay), "Vừa chơi cùng" thấy trọn hàng ô đầu, dải cấp Mùa thấy ngay, điểm + nhiệm vụ đầu của chi tiết sự kiện thấy ngay; sửa thêm khi xem ảnh: tab "Tuần này" mất biểu tượng trên tab đang chọn (đổi biểu tượng sách màu), ô "Vừa chơi cùng" xuống dòng không đều (nickname + cấp một hàng cố định), dòng tiêu đề nhóm bị bẻ từng chữ (tiêu đề không xuống dòng khi chia dòng).

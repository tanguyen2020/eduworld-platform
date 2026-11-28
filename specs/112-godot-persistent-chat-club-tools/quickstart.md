# Quickstart: F112 — Godot — channel chat bền vững (khu vực, DM) và công cụ chủ club (sửa club, phó nhóm, mục tiêu/sự kiện club)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc: F110, F111 (client), F119 (client chat), F120, F121 (backend Done), F091/F092 (hub instance, realtime), F125 (`compact`).
- Lệnh: `make up`, `make seed`, `make up-app`; mở `src/game/` bằng Godot 4.x, chạy `res://scenes/bootstrap/bootstrap.tscn` hai instance với `--device-id` khác nhau ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (TH Demo A, bạn bè), `hs.dung.7a` (owner club "Nhà thám hiểm" đã duyệt), `hs.em.7a` (bạn của Dũng, thành viên club); một học sinh tenant `common`; `ad.truong.a` (portal) để khóa club và đặt preset/giờ chat.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | An và Bình cùng `main_hub`, mở chat HUD | Tab Khu vực, Cả trường, CLB của em, Bạn bè; tin Khu vực của nhau hiện; không tab lớp/khối | F112-AC-01, F112-AC-02 |
| 2 | An vào `learning_zone.math` khi tab Khu vực đang mở | Tab Khu vực đổi sang channel khu mới, không còn tin cũ | F112-AC-03 |
| 3 | Đóng khung chat; Bình gửi tin club | Badge số trên nút chat HUD; World/Area không có số, chỉ `@` khi được nhắc | F112-AC-04, F112-AC-05 |
| 4 | An vào bài kiểm tra (Exam Zone) | Nút chat HUD ẩn; ra khỏi bài thì hiện lại | F112-AC-06 |
| 5 | Dũng mở hồ sơ Em → "Nhắn tin", gửi tin; Em trả lời | `openDirectChannel` `201`, tab DM có nickname + avatar của Em; lần sau `200` cùng channel | F112-AC-07, F112-AC-10 |
| 6 | Em chặn Dũng | Tab DM của Dũng khóa với "Cuộc trò chuyện này đang tạm đóng", vẫn đọc được tin cũ, không nói ai chặn | F112-AC-09 |
| 7 | Học sinh tenant `common` mở hồ sơ một người | Không có nút "Nhắn tin"; không có tab Cả trường; Khu vực chỉ tin nhanh + biểu cảm | F112-AC-07, F112-AC-08, F112-AC-12 |
| 8 | `ad.truong.a` thu hẹp giờ chat của trường; An gửi tin Cả trường ngoài giờ | Ô nhập khóa "Kênh này đang nghỉ…", tin bị gỡ, không tự gửi lại | F112-AC-11 |
| 9 | `ad.truong.a` chọn preset "chỉ tin nhanh + biểu cảm" | Mọi khối chỉ thấy hàng tin nhanh + bảng biểu cảm | F112-AC-12 |
| 10 | `ad.truong.a` khóa club; Dũng bị gỡ khỏi một club khác | Club khóa: chỉ đọc; club bị gỡ: tab biến mất, không lý do | F112-AC-13 |
| 11 | Mở menu tin ở Cả trường | Không có Chuyển tiếp, Ghim, đã xem; không có danh sách thành viên | F112-AC-14 |
| 12 | Dũng tìm "di boss" | Kết quả có dấu "đi boss", đoạn trích văn bản thuần; chạm → nhảy tới tin; tìm 1 ký tự → nút mờ | F112-AC-15, F112-AC-16, F112-AC-17 |
| 13 | Dũng (owner) mở Quản lý club, đổi mô tả; Em (officer) mở trang club | Dũng lưu được; `412` giả lập → form tải lại; Em không thấy nút Quản lý | F112-AC-18, F112-AC-19 |
| 14 | Dũng đặt tên club có từ không phù hợp (danh mục test) | Thông điệp kiểm duyệt chung, form giữ nguyên | F112-AC-20 |
| 15 | Dũng chọn Em làm phó nhóm rồi thôi | `updateClubMemberRole` officer → member; dòng owner không thao tác | F112-AC-21 |
| 16 | Dũng tạo mục tiêu 30 hoạt động; ngắt mạng khi gửi rồi nối lại | Một mục tiêu duy nhất (cùng `Idempotency-Key`); không có đóng góp từng người | F112-AC-22 |
| 17 | Dũng tạo sự kiện "Luyện tập chung" thứ Bảy 9:00 | Không có loại giải nội bộ; giờ đã qua không chọn được; hiển thị ngày + giờ, không đếm ngược | F112-AC-23, F112-AC-24 |
| 18 | Pseudo-locale `xx-PS`, `en`; lint theme | Không tràn chữ; không màu/cỡ chữ hardcode; nút ✕ Đóng đỏ | F112-AC-25, F112-AC-26 |
| 19 | Chạy trên điện thoại 844×390 và 740×360 | Tấm phủ cao hết màn, nút ≥ ngưỡng, ≥ 3 tin mỗi tab | F112-AC-27, F112-AC-28 |
| 20 | 10 tin/giây vào Cả trường trong 60 s trên Android tầm trung | ≥ 30 FPS preset Trung bình | F112-AC-29 |
| 21 | Quét mã `src/game/`; đăng nhập game bằng tài khoản nhân sự | Không NATS/Valkey, không `grade_band` trong logic chat, không BBCode; nhân sự bị chặn theo F061 | F112-AC-30, F112-AC-31 |

## 3. Lệnh test tự động

```bash
make godot-test            # GdUnit4 headless: tests/unit/chat, tests/unit/social, tests/scene/chat, tests/scene/social, ma trận khối
make godot-export-check
make design-tokens-check   # nếu có token mới
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (số pass/fail), ma trận khối (4 khối × 1280×720 / 1024×600 / 844×390 / 740×360 × `vi`/`en`/pseudo) kèm probe vật lý và kiểm ngược, ảnh trước/sau trong `screenshots/` theo F112-AC-28 với bảng màn × khối đã xem từng ảnh, số liệu FPS thiết bị lab, kịch bản §2 chạy với backend thật (ghi rõ kịch bản nào chỉ chạy với fake).

## 5. Bằng chứng

2026-11-28, phần Godot client — máy local, Godot 4.7.2 (headless cho GdUnit4, Compatibility/OpenGL cho ảnh), FakeHttp + FakeWebSocketPeer + fixture contract (TEST-ONLY, AGENTS R17) — **chưa** chạy với backend thật.

**Test (viết trước, đỏ → xanh — chi tiết ở [tasks.md](tasks.md)):** 65 test F112 xanh; GdUnit4 toàn bộ 1760 test — unit 1005, scene 667, contract 37, integration 18 (16 skip), perf 18, tools 11, visual 4 — 0 lỗi; lint Godot, export PCK Android + scan_artifact xanh

**Kịch bản §2 tự động hóa bằng fake:** 1–4 (`test_chat_hub_state`, `test_chat_hub`), 5–7 (`test_direct_channels`, `test_persistent_channel_states`), 8–11 (`test_persistent_channel_states`), 12 (`test_chat_search_service`, `test_highlight_text`, `test_chat_search_panel`), 13–17 (`test_club_manage_service`, `test_club_manage`), 18–19 (`test_f112_band_matrix`), 21 (`test_f112_boundaries`). **Còn chờ môi trường thật:** kịch bản §2 với backend compose và hai client (T038), kịch bản 20 — FPS Android tầm trung (T039, SC-005).

**Ảnh (`screenshots/`, `f112_<màn>-after-<cỡ>-<ngôn ngữ>-<khối>.png`):** 13 màn (`hub_area`, `hub_world`, `hub_club`, `hub_friends`, `hub_dm`, `hub_dm_frozen`, `search_hits`, `search_empty`, `manage_edit`, `manage_officers`, `manage_goal`, `manage_event`, `hud`) × {1280×720, 844×390, 740×360} `vi` `g1_3`; × {844×390, 740×360} `vi` `g4_5`/`g6_7`/`g8_9`; 844×390 `en` `g4_5`; 1280×720 `vi` `g8_9`; màn F111 đổi (`f111_friends_join` — nút "Nhắn tin", `f111_club_page` — nút "Quản lý club") ở 3 cỡ `vi`/`en` `g1_3`.

| Màn | Khối đã xem | Lỗi thấy khi xem ảnh → đã sửa |
|---|---|---|
| `hub_*` (điện thoại) | g1_3, g4_5, g6_7, g8_9 | Hàng tab dưới thanh tiêu đề làm "≥ 3 tin" hụt (`message list 151 px < 194`) → tab lên dòng thanh tiêu đề, tấm phủ cả màn; tab đang chọn xanh trên thanh xanh khó thấy → tab sáng, tab chọn vàng + ✓; kính lúp đứng trước tab khi mở lại → thứ tự icon · tab · kính lúp · Đóng; tab DM mới mở bị che một nửa → cuộn tab vào tầm nhìn sau một khung |
| `hub_*` (PC) | g1_3, g8_9 | Tab chọn mất biểu tượng (nét xanh trên nền xanh) → dấu ✓; khung PC thấp làm "≥ 3 tin" hụt ở `g1_3` → khung cao hết màn, rộng 40% |
| `hub_dm_frozen` | mọi khối | Mũi tên của hàng tin nhanh còn lại khi khóa → ẩn cả thanh tin nhanh, chỉ còn băng "Cuộc trò chuyện này đang tạm đóng" |
| `hub_friends` | mọi khối | "Nhắn tin" nút xanh mất biểu tượng → nút phụ (biểu tượng chat thấy được) |
| `search_*` | mọi khối | Từ khóa in đậm không thấy khi font không có đậm → thêm gạch chân |
| `hud` | g1_3, g8_9 | Nút chat ở hàng "Sự kiện/Thêm" bị toast "Em có 1 bài…" che → chuyển xuống hàng dưới-phải trước "Về Quảng trường"; badge đè chữ "Chat" → đặt cao hơn |
| `manage_*` | mọi khối | Không lỗi chặn; nút "Lưu" mờ khi chưa đổi gì (biểu tượng theo kiểu nút mờ của Theme) |

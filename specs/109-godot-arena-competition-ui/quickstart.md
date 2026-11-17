# Quickstart: F109 — Giao diện Arena, trận đấu, bảng xếp hạng, giải đấu trong Godot

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F061, F063, F092, F102, F105, F106, F107 (trước đó profile `mock` Prism + fixture AsyncAPI, nhãn MOCK; chỉ nghiệm thu `Done` với backend thật).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); chạy hai instance Godot `-- --env=local --device-id=<khác nhau>`.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (lớp 3, `g1_3`), `hs.chi.3b`, `hs.em.4a` (`g4_5`), `hs.dung.7a` (`g6_7`), `ad.truong.a` (policy trường), `qlcm.truong.a`.
- Dữ liệu: season đang mở; Class Tournament 3A ở `registration_open` (tạo bằng `ad.truong.a` qua portal từ template đã gán); bảng "Mục tiêu lớp", "Lớp em tuần này", "Xếp hạng rating" có dữ liệu.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` nói chuyện NPC Trọng tài Sấm / nhấn `K` | Arena mở; thẻ chế độ theo server; TierBadge chỉ tên tier | F109-AC-01, F109-AC-15 |
| 2 | Chọn 1v1 Toán → Tìm trận; đi lại trong Hub; bấm Hủy rồi tìm lại | Thanh hàng chờ nổi, hủy biến mất | F109-AC-02 |
| 3 | `hs.binh.3a` tìm cùng chế độ; cả hai bấm Vào trận; thử lần hai một bên từ chối | Ready check đếm theo server; người đã nhận quay lại hàng chờ | F109-AC-03 |
| 4 | Gọi tìm trận khi đã có ticket; chờ tới timeout | Mở thanh hiện có; gợi ý chế độ khác/Robot | F109-AC-04 |
| 5 | Trong trận: trả lời bằng chạm và phím `1`–`4` | Khóa ngay "Đã ghi nhận"; đúng/sai chỉ sau reveal, có ✓/✗ | F109-AC-05, F109-AC-06, F109-AC-08 |
| 6 | Gửi lại câu đã khóa (công cụ debug) | `ANSWER_ALREADY_SUBMITTED` → thông điệp, không gửi lại | F109-AC-07 |
| 7 | Quan sát trận lớp 3 | Không số giây, không "thưởng tốc độ", không gợi ý trả lời nhanh; vùng chạm lớn | F109-AC-09 |
| 8 | Trận đội (fixture/2v2 với `hs.dung.7a` + bot) | Điểm theo đội, đánh dấu lượt chính | F109-AC-10 |
| 9 | Ngắt mạng `hs.binh.3a` 10 giây giữa câu 3 | Bên kia thấy "Đối thủ đang kết nối lại…"; bên rớt dựng lại đúng câu từ snapshot | F109-AC-11 |
| 10 | Đo FPS BattleArena trên DEV-AND-MID1 | Không render thế giới 3D; FPS đạt preset | F109-AC-12 |
| 11 | Mở/đóng câu hỏi khi có tin chat trận; trên Android | Strip thu nhỏ, không che đáp án, không có nút tắt chat | F109-AC-13 |
| 12 | Kết thúc trận; mô phỏng trận `cancelled` | Màn kết quả động viên, points, tier; "Trận đã hủy" | F109-AC-14 |
| 13 | Báo cáo đối thủ với ghi chú dài; chặn đối thủ | Ô ghi chú giới hạn + bộ đếm; gửi một lần | F109-AC-16 |
| 14 | Lịch sử trận → chi tiết | Danh sách cursor, đối thủ chỉ nickname/tier | F109-AC-17 |
| 15 | `hs.an.3a`, `hs.em.4a`, `hs.dung.7a` mở Bảng xếp hạng | Lớp 3: mục tiêu lớp; lớp 4: top + "nhóm X%"; lớp 7: top + quanh mình; chỉ nickname | F109-AC-18, F109-AC-19, F109-AC-20 |
| 16 | `ad.truong.a` tắt bảng lớp của trường; `hs.an.3a` tải lại; mở `leaderboard_id` của trường B | Bảng biến mất; 404 → "Bảng này hiện không mở" | F109-AC-21, F109-AC-18 |
| 17 | Xem log thông báo sau khi bị bạn vượt điểm | Không có thông báo "bị vượt qua"; bảng không tự làm mới | F109-AC-22 |
| 18 | `hs.an.3a` mở Giải đấu → đăng ký → check-in khi mở cửa sổ → xem bracket | Trạng thái cập nhật; bracket "con đường"; `hs.chi.3b` không thấy giải | F109-AC-23, F109-AC-24, F109-AC-25 |
| 19 | Tới giờ trận giải | Toast thông báo → ready check | F109-AC-26 |
| 20 | `hs.an.3a` thách `hs.binh.3a`; lần hai để hết hạn | Vào trận unranked; "Lời thách đấu đã hết hạn" | F109-AC-27 |
| 21 | Xem network log; dùng `match_id` của `hs.dung.7a` | `X-Tenant-Id` mọi request, `Idempotency-Key` ghi; 404 "Không tìm thấy" | F109-AC-28, F109-AC-29 |
| 22 | Đổi `en` và pseudo-locale | Mọi màn không tràn chữ, không chuỗi hardcode | F109-AC-30 |

## 3. Lệnh test tự động

```bash
make godot-test
godot --headless --path src/game -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests/unit/competition -a res://tests/scene/competition -a res://tests/contract/competition
make design-tokens-check
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), video trận 1v1 hai client + reconnect, ảnh màn kết quả/bảng xếp hạng ba khối (vi/en), trace_id ticket/trận/đăng ký giải, báo cáo FPS BattleArena trên DEV-AND-MID1.

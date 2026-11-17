# Quickstart: F092 — `RealtimeClient` Godot và Main Hub nhiều người chơi

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F090, F091, F061, F062, F070.
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make up-app`; Godot mở `godot/project.godot`, `Config` `realtime_url="ws://localhost:9080/realtime/v1"`; profile `chaos` bật toxiproxy.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (`th-demo-a`), `hs.dung.7a` (`pgd-demo`), `ad.truong.a`; bot hub `tests/load/hubbot` (F091).
- Thiết bị lab: DEV-AND-MID1, DEV-PC-MID, DEV-WEB-MID ([PERFORMANCE_BUDGET §2](../../docs/08-game-client/PERFORMANCE_BUDGET.md)).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Đăng nhập `hs.an.3a`, vào Hub | Log state `FetchingTicket → Connecting → Welcomed → Online`; không có ticket trong log | F092-AC-01, F092-AC-07 |
| 2 | Đăng nhập portal-only `ad.truong.a` vào game (mock trả 403 ở ticket nếu qua được) | Màn hướng dẫn nhân sự, không kết nối WS | F092-AC-02 |
| 3 | Chặn mạng 10 s bằng toxiproxy | Banner "Đang kết nối lại…", avatar khác đứng yên; sau đó resume, banner tắt | F092-AC-03, F092-AC-05, F092-AC-08 |
| 4 | `hs.binh.3a` đăng nhập cùng lúc trên client thứ hai | Hai client thấy nhau trong cùng instance; avatar và nickname đúng | F092-AC-10, F092-AC-15 |
| 5 | A chạy vòng quanh quảng trường | B thấy A di chuyển mượt; băng thông lên của A ≤ `hub_sync_bandwidth` | F092-AC-11, F092-AC-12 |
| 6 | A dùng emote "Vẫy tay"; spam emote | B thấy animation; spam bị chặn với thông báo thân thiện | F092-AC-16 |
| 7 | A đổi trang phục (F070) | B thấy trang phục mới | F092-AC-15 |
| 8 | Bật cheat debug dịch chuyển 50 m | Nhân vật A trượt về vị trí server trong 200 ms | F092-AC-14 |
| 9 | Kill instance competition giữ lease | Không tải lại scene; tiếp tục thấy người chơi | F092-AC-17 |
| 10 | Đăng nhập `hs.an.3a` trên máy thứ ba và chọn takeover | Máy cũ hiện `ReplacedNotice`, không tự kết nối lại | F092-AC-04 |
| 11 | `hs.dung.7a` vào Hub cùng lúc | Không xuất hiện trong instance của `hs.an.3a` | F092-AC-19 |
| 12 | Đổi ngôn ngữ sang `en-US` khi banner đang hiện | Banner đổi ngôn ngữ ngay | F092-AC-18 |
| 13 | 39 bot + client thật trên DEV-AND-MID1, preset Trung bình, 10 phút | FPS p10 ≥ ngưỡng AC-PERF-001; băng thông ≤ `hub_sync_bandwidth` | F092-AC-20, F092-AC-21, F092-AC-22 |
| 14 | Soak 60 phút với bot vào/ra liên tục | RAM không tăng quá ngưỡng AC-PERF-006 | F092-AC-23 |

## 3. Lệnh test tự động

```bash
make godot-test                                  # unit + scene (FakeWebSocketPeer, fixture AsyncAPI)
make godot-test SUITE=integration                # 2 client headless với backend compose (+ toxiproxy)
make godot-export-check
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), video/ảnh kịch bản 4–5, báo cáo FPS p10 và băng thông từ lab (kịch bản 13), log state machine kịch bản 3 và 10.

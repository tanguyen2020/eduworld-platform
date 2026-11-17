# Quickstart: F091 — Hub instance, presence, đồng bộ vị trí

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F090, F070; seed area `main_hub`, `social_zone`, `learning_zone.math` phát qua `content.published` (`item_type = world_area`) (F071 hoặc seed module game).
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make seed-check`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events`.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (`th-demo-a`, khối 3), `hs.em.4a`, `hs.dung.7a` (`pgd-demo`); bot WS `tests/load/hubbot` (40 bot khối 3 trường TH Demo A sinh từ seed load).
- Bạn bè: phát fixture `friend.added (hs.an.3a, hs.binh.3a)` qua NATS (hoặc dùng F095 nếu đã Done).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` mở WS (F090), gửi `hub.instance.join {area_id: main_hub}` | `hub.instance.assigned` đủ trường; dòng `multiplayer.rooms` `hub_instance`, `grade_band = g1_3` | F091-AC-01 |
| 2 | `hs.binh.3a` join `main_hub` | Cùng `room_id` (bạn bè) | F091-AC-03 |
| 3 | `hs.em.4a` (khối 4, tenant khác) join `main_hub` | `room_id` khác; không thấy `hs.an.3a` | F091-AC-02 |
| 4 | Chạy 39 bot khối 3 vào `main_hub`, rồi `hs.chi.3b` join | Instance đầu đủ 40; `hs.chi.3b` sang instance mới | F091-AC-04 |
| 5 | `hs.chi.3b` gửi `hub.instance.join {area_id: main_hub, follow_user_id: <hs.an.3a>}` khi chưa là bạn | `PERMISSION_DENIED`, không lộ `room_id` | F091-AC-05 |
| 6 | Join `area_id: "khong_co"`; `area_id` dài 100 ký tự | `NOT_FOUND`; `VALIDATION_FAILED` | F091-AC-06 |
| 7 | `hs.em.4a` gửi `hub.player.move` với `room_id` của `hs.an.3a` | `ROOM_NOT_MEMBER` | F091-AC-07 |
| 8 | Bot gửi vị trí nhảy 50 m trong 100 ms | `hub.player.corrected {reason: speed}`; lặp 25 lần/phút → `competition.integrity_signals` + audit | F091-AC-12, F091-AC-13 |
| 9 | Đo 40 bot trong 10 phút | Snapshot 10 Hz, chỉ thực thể trong 9 ô; băng thông ≤ `hub_sync_bandwidth`; tick p95 trong ngân sách | F091-AC-14, F091-AC-16 |
| 10 | Kiểm payload `hub.player.joined` | Chỉ `eid`, `PublicPlayer` (nickname, avatar_config, level) | F091-AC-15 |
| 11 | `hs.an.3a` ngắt mạng 10 s rồi resume | Về cùng instance, thấy lại người khác ≤ 2 s; quá 30 s → `hub.player.left` | F091-AC-09 |
| 12 | Kill instance competition giữ lease | Instance khác nhận, client nhận `hub.instance.migrated` | F091-AC-20 |
| 13 | Đổi trang phục (F070 `updateAvatar`) | Người cùng instance nhận `hub.player.appearance_changed` | F091-AC-19 |
| 14 | Kiểm `ew:gateway:presence:user:<hs.an.3a>` sau kết nối, sau join, sau đóng 4408 và sau 50 s | `online` → `area_id=main_hub` → `away` → `offline`; mỗi bước có một message `ew.rt.presence.<tenant>` | F091-AC-21, F091-AC-23, F091-AC-24 |
| 15 | Mọi người rời instance, chờ `hub_instance_idle_close` | `rooms.status = closed`, `closed_at` có giá trị | F091-AC-10 |

## 3. Lệnh test tự động

```bash
make test MOD=competition
make test-integration MOD=competition
make test-integration MOD=realtime      # presence
go test ./modules/competition/tests/load/... -run Hub40 -tags load   # TC-PERF-003, TC-PERF-023
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), báo cáo tick p95 và băng thông của kịch bản 9, truy vấn chứng minh không instance nào trộn partition, trace_id kịch bản 1.

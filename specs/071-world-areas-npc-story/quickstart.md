# Quickstart: F071 — Thế giới: 8 vùng đất, zone, world map, cổng, mở khóa, mạch truyện, NPC và hội thoại

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F026, F070 (và F014, F062).
- Lệnh: `make up`, `make seed` (nạp `modules/game/seed/world/*` qua `PublishGameContent`), `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.chi.3b` (TH Demo A), `hs.em.4a` (TH Demo C, tenant `pgd-demo`), `hs.dung.7a` (THCS Demo B), `ad.truong.a`.
- Phát event thử (trước khi F072 xong) bằng `nats pub ew.evt.game.quest.completed "$(cat <fixture>)"` trong `nats-box` (`make tools`), fixture ở `modules/game/tests/fixtures/events/` (envelope chuẩn EVENT_DESIGN §2).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` (level 1, Xu 0) `listWorldAreas` | 12 `area_id` của 8 vùng, tên vi/en; `learning_zone.math` mở; `quest_zone.discovery_ocean` khóa | F071-AC-01, F071-AC-02 |
| 2 | Tắt môn Lịch sử ở cài đặt TH Demo A rồi `listWorldAreas` | Không có `learning_zone.history` | F071-AC-03 |
| 3 | `getWorldManifest`; đổi tên vùng ở seed v2 rồi publish | Bundle có `sha256`; `manifest_version` tăng, ETag đổi, tên mới hiện ở lần vào màn sau | F071-AC-04, F071-AC-05 |
| 4 | `enterWorldArea("learning_zone.math")` + `startLevel` | `room_id`, `spawn_point_id`, `level_id`; level session ghim version | F071-AC-07, F071-AC-11 |
| 5 | `enterWorldArea("challenge_mountain")` | `403 AREA_LOCKED` + `missing_requirements` (0/2 chương) | F071-AC-08 |
| 6 | `hs.an.3a` và `hs.em.4a` cùng vào `main_hub` (khác trường) | `room_id` khác nhau | F071-AC-09 |
| 7 | Phát `quest.completed` main quest chương 0 (2 lần) | 1 dòng `world_area_unlocks`, 1 `world.area_unlocked`, WS `world.area.unlocked`; chương 1–4 `available` | F071-AC-14, F071-AC-15, F071-AC-22 |
| 8 | `getNpcDialogue("npc.mr_numbers")` cho lớp 3 và lớp 7 | Graph đúng khối, vi/en; NPC sai → 404 | F071-AC-17 |
| 9 | Phát `quest.completed` cho mọi main quest chương 1 | Chương 1 `completed`, `story.chapter_completed` một lần, chương 5 mở | F071-AC-21, F071-AC-23 |
| 10 | Trong `learning_zone.math`: `reportDiscovery` điểm hợp lệ, gọi lại, gọi từ `main_hub` | 200 + % tăng; `409 DISCOVERY_ALREADY_FOUND`; `422 STEP_PRECONDITION_FAILED` | F071-AC-24, F071-AC-25 |
| 11 | Token `ad.truong.a` gọi `listWorldAreas` | `403 GAME_ACCESS_STUDENT_ONLY` | F071-AC-06 |
| 12 | Godot DEV-PC-MID: cổng gió Main Hub → Vương quốc Số; mở World Map; tài khoản lớp 3 dùng bảng chọn hình | Chuyển vùng trong ngân sách; bản đồ % đúng, vùng khóa có điều kiện; 1 chạm dịch chuyển | F071-AC-12, F071-AC-26, F071-AC-27 |
| 13 | Godot: nói chuyện Ông Số → action `open_minigame` → xong tổng kết | Mở activity, quay về cạnh NPC, không tự nối hoạt động | F071-AC-19, F071-AC-20 |
| 14 | CI: `make godot-test` (zone lint + benchmark headless) | 0 ID trùng, trong ngân sách cảnh | F071-AC-13 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-contract
make godot-test
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 4, 7, 10; video chuyển vùng trên DEV-AND-MID1; báo cáo benchmark zone.

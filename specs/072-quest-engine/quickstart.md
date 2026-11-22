# Quickstart: F072 — Quest Engine: 12 loại quest, bước, tiến độ qua event, lọc theo khối

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F030, F071 (và F014, F031).
- Lệnh: `make up`, `make seed` (quest `src/backend/modules/game/seed/quests/` + gán 3 tenant; tenant chưa được chiếu thì chạy lại `make seed-game` sau `make seed-check`), `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a` (lớp 3), `hs.em.4a` (lớp 4, tenant `pgd-demo`), `hs.tudo` (tenant chung, khối 4), `ad.truong.a`.
- Token game + `createGameSession` + `enterWorldArea("learning_zone.math")` (F071) trước khi báo bước.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` `listQuests?status=available` | Quest Toán lớp 3 (`math.g3.market_helper`) + main quest chương 0; lọc `quest_type=side`, `area_id=learning_zone.math` đúng (seed không có quest `learning` — Clarifications 2026-11-22) | F072-AC-01 |
| 2 | `hs.em.4a` `getQuest` quest chỉ lớp 3; `hs.tudo` gọi quest chưa gán tenant chung | `404`; `404` | F072-AC-02 |
| 3 | `hs.an.3a` `getQuest` (vi rồi en) | Tên/mô tả/bước song ngữ, phần thưởng dự kiến | F072-AC-03 |
| 4 | `acceptQuest` (K1), gửi lại K1 | `QuestProgress` ghim version, bước 1 `active`; cùng response | F072-AC-04 |
| 5 | Nhận quest chương 1 khi chưa xong chương 0; nhận quá `max_active_quests` | `403 QUEST_PREREQUISITE_NOT_MET`; `409 QUEST_ACTIVE_LIMIT` | F072-AC-05, F072-AC-06 |
| 6 | `reportQuestStep` `go_to` ở `learning_zone.math`; rồi từ `main_hub`; báo lặp | Bước xong + WS `quest.progress.updated`; `409 STEP_PRECONDITION_FAILED` (API_DESIGN §4.1); tính 1 lần | F072-AC-09, F072-AC-10, F072-AC-11 |
| 7 | `startQuestActivity` bước học tập → làm bài trong activity player, nộp đạt | Attempt `mode = quest`; sau `activity.graded` bước `completed` | F072-AC-14, F072-AC-15 |
| 8 | `reportQuestStep` cho bước học tập; bắt đầu bước quá `max_attempts_per_day` | `409 STEP_PRECONDITION_FAILED`; `409 QUEST_STEP_ATTEMPT_LIMIT` | F072-AC-16, F072-AC-17 |
| 9 | Dừng module learning, gọi `startQuestActivity` | `503 UNAVAILABLE` + `Retry-After`, tiến độ không đổi | F072-AC-18 |
| 10 | Hoàn thành bước cuối | `quest.completed` một lần; learning event `quest.completed` trong `eduworld_learning`; chương 0 hoàn thành (F071) | F072-AC-19, F072-AC-21 |
| 11 | Vào game lần đầu trong ngày; vào lại; chỉnh Clock giả qua mốc reset | Nhận daily đúng số; không thêm; quest cũ `expired`, không trừ XP/Xu | F072-AC-22, F072-AC-23 |
| 12 | Publish v2 quest khi `hs.an.3a` đang làm v1; `hs.binh.3a` nhận sau | A giữ v1; B nhận v2 | F072-AC-24 |
| 13 | Nhận lại quest `once` đã xong; quest có `time_limit_minutes` hết giờ | `409 QUEST_ALREADY_COMPLETED`; chỉ bước liên quan đặt lại | F072-AC-25, F072-AC-26 |
| 14 | Seed quest thiếu `en`; quest có bước `boss` | Publish `422 TRANSLATION_INCOMPLETE`; `422 VALIDATION_FAILED` | F072-AC-27 |
| 15 | Token `ad.truong.a` gọi `listQuests` | `403 GAME_ACCESS_STUDENT_ONLY` | F072-AC-08 |
| 16 | Godot: mở Quest Journal (J), tab Đang làm, "Theo dõi", "Bắt đầu" | Journal/Tracker cập nhật từ WS, không gọi lại REST | F072-AC-09 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-integration MOD=learning   # consumer learning event quest.completed
make test-contract                           # game ↔ learning createAttempt
make godot-test
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 7 (game → learning → event → game), truy vấn `game.quest_progress`, ảnh Quest Journal `g1_3` và `g6_9`.

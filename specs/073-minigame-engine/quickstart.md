# Quickstart: F073 — Mini-game Engine: `MiniGameConfig`, plugin `minigame`, scene Godot 11 loại

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F022, F072 (và F023, F030, F063, F071).
- Lệnh: `make up`, `make seed` (câu hỏi Toán lớp 1–5 ở learning; `src/backend/modules/game/seed/minigames/` qua `PublishGameContent`), `make up-app`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a` (lớp 3), `hs.dung.7a` (lớp 7, tenant khác), `ad.truong.a`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Seed publish config `matching` (question set bảng nhân 6–9) | Thành công; `ref.minigame_config_versions` ở learning có version; event lặp không thêm dòng | F073-AC-01, F073-AC-03, F073-AC-04 |
| 2 | Seed config `matching` với selection rule `single_choice`; config thiếu `en` | `422 ACTIVITY_CONFIG_INVALID`; `422 TRANSLATION_INCOMPLETE` | F073-AC-02, F073-AC-05 |
| 3 | `hs.an.3a` `startMiniGame` (free_play) | `201 MiniGameSession`; payload không có `answer_key`/`correct`/feedback | F073-AC-06, F073-AC-07 |
| 4 | Gửi câu trả lời kèm `is_correct: true`, `game_score: 9999` | Bị bỏ qua; điểm theo server; lớp 3 không có `time_bonus` | F073-AC-08, F073-AC-09 |
| 5 | Nộp | `learning_score`, `game_score`, `completion_met` riêng; evidence chỉ từ câu trả lời | F073-AC-10 |
| 6 | Config `timed_challenge`, gửi câu sau `deadline_at` | `409 ATTEMPT_DEADLINE_PASSED`; attempt `expired` | F073-AC-11, F073-AC-26 |
| 7 | `startMiniGame` với `context_type = assignment`; `hs.dung.7a` chơi config lớp 3; token `ad.truong.a` | `422`; `404`; `403 GAME_ACCESS_STUDENT_ONLY` | F073-AC-12, F073-AC-13 |
| 8 | Lưu activity `minigame` trỏ config chưa có trong projection; chạy test hợp đồng plugin | `422 ACTIVITY_CONFIG_INVALID`; TC-ACT-001 xanh | F073-AC-14, F073-AC-15 |
| 9 | Quest "Phiên chợ của bác Gấu Nâu": bước `minigame` → chơi đạt | Bước `completed`; publish config v2 giữa lượt không đổi câu của lượt | F073-AC-16, F073-AC-17 |
| 10 | Sau nộp: kiểm event; chơi lần `max_plays_per_day + 1` | `minigame.completed` một lần; lần vượt `rewards_eligible = false`, vẫn chơi được | F073-AC-18, F073-AC-19, F073-AC-20 |
| 11 | Godot DEV-AND-MID1: chơi 6 loại lõi; tắt mạng giữa chừng | Scene đúng loại; hiệu ứng sau kết quả server; đồng hồ theo server, gửi lại khi có mạng; nút ≥ 64 px | F073-AC-21, F073-AC-22, F073-AC-23, F073-AC-24 |
| 12 | Godot: 5 loại mở rộng với fixture | Đạt cùng tiêu chí | F073-AC-25 |

## 3. Lệnh test tự động

```bash
make test MOD=game && make test MOD=learning
make test-integration MOD=game
make test-integration MOD=learning
make test-contract
make godot-test
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 3–5 (game → learning → event → game), dump payload kịch bản 3 (không đáp án), video 6 loại lõi trên DEV-AND-MID1.

# Quickstart: F076 — Challenge Engine solo, 5 mức khó, room host dùng chung

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F072, F074 (và F030 với delta `createAttempt` mode `challenge`).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events` (chạy 2 instance worker cho kịch bản failover), `make seed-check` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Seed config: `go -C src/backend run ./modules/game/cmd/worker seed-config --dir deploy/seed/game-config` (challenge `time` khối 3, `survival` khối 3–7, `weekly` khối 7; question set Toán khối 3 và 7 đã publish ở module learning).
- Tài khoản demo: `hs.an.3a` (khối 3), `hs.dung.7a` (khối 7), `ad.truong.a`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` gọi `listGameChallenges` | Chỉ challenge khối 3; `available_difficulties` tối đa `hard` | F076-AC-01 |
| 2 | `hs.an.3a` `getGameChallenge` của challenge khối 7 | `404` | F076-AC-02 |
| 3 | `hs.an.3a` `startGameChallenge` (`hard`, key K1); gửi lại K1 | `201`, `deadline_at` theo giờ server; lần 2 cùng attempt | F076-AC-04, F076-AC-07 |
| 4 | `hs.an.3a` chọn `expert` | `403 CHALLENGE_NOT_ELIGIBLE` | F076-AC-05 |
| 5 | `nextChallengeActivity` | Attempt học tập mode `challenge` của `hs.an.3a`, câu khối 3 độ khó trong dải `hard`, không có đáp án | F076-AC-08, F076-AC-09 |
| 6 | Trả lời trong Godot (hoặc phát `activity.graded`) sai hết ở `survival` | `lives_left` giảm từng câu; về 0 → `failed`, một `challenge.completed`, client nhận `challenge.attempt.updated` | F076-AC-10, F076-AC-11 |
| 7 | Dừng module learning rồi gọi `nextChallengeActivity` | `503 UNAVAILABLE`, challenge không đổi | F076-AC-12 |
| 8 | Bắt đầu challenge `time`, đổi đồng hồ máy client, chờ quá `deadline_at` | Server tự `failed`; gọi tiếp `422 INVALID_STATE_TRANSITION` | F076-AC-13 |
| 9 | Đổi thiết bị (takeover), gọi `getGameChallengeAttempt` | Đúng điểm/sức bền/hạn chót | F076-AC-15 |
| 10 | Thử lại tới khi quá `challenge_attempts_per_day` | `409 CHALLENGE_ATTEMPT_LIMIT` kèm thời điểm thử lại | F076-AC-06 |
| 11 | Tắt worker đang giữ lease của một room | Worker còn lại chiếm lease, timer tiếp tục | F076-AC-17 |
| 12 | Token `ad.truong.a` gọi `listGameChallenges`; `hs.an.3a` đọc attempt của `hs.dung.7a` | `403 GAME_ACCESS_STUDENT_ONLY`; `404` | F076-AC-03 |

## 3. Lệnh test tự động

```bash
go -C src/backend test ./pkg/roomhost/...
make test MOD=game
make test-integration MOD=game
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test theo mã AC, trace_id kịch bản 5 (game → learning), log failover kịch bản 11, kết quả SC-002 (1.000 lượt không câu ngoài khối).

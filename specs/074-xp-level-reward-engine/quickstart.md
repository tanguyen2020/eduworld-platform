# Quickstart: F074 — XP ledger, level, đường cong XP, danh hiệu, Reward Engine có cap

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F070, F071, F072 (và F030/F031 phát `activity.graded`, `learning_objective.mastered`, `lesson.completed`).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events`, `make seed-check` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Seed config platform (curve mặc định theo `xp_curve`, reward rule cho mỗi `source_code`) nạp bằng `go run ./cmd/worker seed-config --module=game --dir deploy/seed/game-config` (đã publish).
- Tài khoản demo: `hs.an.3a` (khối 3), `hs.dung.7a` (tenant `pgd-demo`), `ad.truong.a` (nhân sự, để thử bị chặn).
- Công cụ phát event thử: `nats pub ew.evt.learning.activity.graded --header Nats-Msg-Id:<uuid> '<envelope>'` (fixture ở `modules/game/tests/fixtures/events/`).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` hoàn thành quest side trong Godot (hoặc phát `quest.completed`) | Một dòng `gamification.xp_ledger`; `game_profiles.xp_total` tăng; client nhận `progression.xp.updated` ≤ 2 s | F074-AC-01, F074-AC-03 |
| 2 | Phát lại đúng envelope bước 1 | Không có dòng mới | F074-AC-02 |
| 3 | Phát `activity.graded` cùng `content_version_id` 4 lần (event_id khác) | XP 100% / 50% / 25% / 0% | F074-AC-04 |
| 4 | Phát event cho đến khi tổng XP hôm nay chạm cap | Bút toán cuối `capped = true`; `getProgression.daily_xp_remaining = 0` | F074-AC-05 |
| 5 | `activity.graded` có `first_try_correct_count = 0` | 0 XP | F074-AC-06 |
| 6 | `hs.dung.7a` làm nội dung khối 3 không có `recommendation_id` rồi có `recommendation_id` | Lần 1 × `xp_below_grade_multiplier`; lần 2 đầy đủ | F074-AC-07 |
| 7 | Phát `learning_objective.mastered` cùng LO 2 lần | XP nguồn `mastery.objective_mastered` một lần | F074-AC-08 |
| 7b | Phát `lesson.completed` (fixture hợp lệ `lesson.completed.v1`) hai lần cùng `lesson_version_id` | Một bút toán nguồn `lesson.completed` (`source_type = lesson`) | F074-AC-27 |
| 8 | Phát event có `tenant_id` khác tenant học sinh | Không bút toán; event vào DLQ `ew.dlq.game.reward_engine` | F074-AC-11 |
| 9 | Đẩy XP vượt 2 ngưỡng level | 2 `level.reached`, 2 dòng `level_history`, RewardGrant mỗi level có thưởng | F074-AC-13, F074-AC-14 |
| 10 | `getProgression`, `listXpHistory`, `listProgressionLevels` bằng token `hs.an.3a` | Dữ liệu của mình; lịch sử không có `source_id`/`rule_id` | F074-AC-17, F074-AC-18 |
| 11 | Gọi bước 10 bằng token `ad.truong.a`; token `hs.an.3a` với `X-Tenant-Id` của `pgd-demo` | `403 GAME_ACCESS_STUDENT_ONLY`; `403 TENANT_MISMATCH` | F074-AC-19 |
| 12 | Seed curve v2 tăng ngưỡng rồi publish (học sinh đang level 12) | Level giữ 12; `xp_to_next` theo v2 | F074-AC-21 |
| 13 | Phát `student.synced` đổi `grade_level` 3→4 cho `hs.an.3a` hai lần | Level/XP giữ; quest `learning` khối 3 `expired`; lần 2 không đổi | F074-AC-25 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail theo mã AC), trace_id kịch bản 1 và 9, truy vấn `SELECT sum(amount) … = xp_total`, log job `xp-reconcile` 0 lệch.

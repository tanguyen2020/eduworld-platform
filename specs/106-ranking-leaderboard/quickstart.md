# Quickstart: F106 — Ranking Glicko-2, tier, season và 13 bảng xếp hạng

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F105 (`match.completed`), F104; seed Ranking Rules, mùa "pre-season", danh mục leaderboard mặc định ([COMPETITION_SYSTEM §9.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#92-danh-mục-bảng-mặc-định-proposed)).
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (lớp 3, TH Demo A), `hs.chi.3b`, `hs.dung.7a` (lớp 7, `pgd-demo`), `hs.em.4a` (lớp 4), `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`. Dữ liệu 50.000 học sinh cho SC-005: script `tests/perf/seed_leaderboard.go`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `go test ./internal/domain/ranking -run Glickman` | Khớp r'≈1464.06, RD'≈151.52 (sai số < 0.01) | F106-AC-01 |
| 2 | Phát `match.completed` ranked (fixture lớp 7) hai lần | `ratings` đổi một lần; `rating_changes` 1 dòng/học sinh; `rating.updated` 1 lần | F106-AC-01, F106-AC-02 |
| 3 | Phát `match.completed` bot, duel, giải, `aborted` | Rating và points không đổi | F106-AC-03 |
| 4 | Fixture trận đội 3v3 có 1 người `abandoned` | Mỗi thành viên cập nhật theo đối thủ ảo; người bỏ nhận 0 | F106-AC-04 |
| 5 | Học sinh mới chơi 3 trận gọi `getMyRatings` | `provisional = true`, "Đang xếp hạng (3/10)", không tier; không ở bảng `rating` | F106-AC-05 |
| 6 | Đồng hồ giả +7 ngày, chạy job RD | RD tăng không vượt max; rating giữ nguyên | F106-AC-06 |
| 7 | `hs.dung.7a` gọi `getMyRatings` sau 12 trận | Có `tier_code`, `tier_name`; không `rating`/`rd`; quét mọi route học sinh không có trường số rating | F106-AC-07, F106-AC-08 |
| 8 | Rating dao động quanh ngưỡng tier trong hysteresis | Tier không đổi qua lại | F106-AC-08 |
| 9 | `qlcm.truong.a` gọi `getSchoolDashboard` (learning) | Không có Game Rating | F106-AC-09 |
| 10 | Fixture 21 trận ranked trong một ngày | Tổng ≤ `competition_points_daily_cap`; phần vượt `amount = 0`, `reason = capped`; thua không trừ | F106-AC-10, F106-AC-11 |
| 11 | `hs.dung.7a` `listLeaderboards`, mở "Đấu trường trường" | Danh mục đúng khối; top 50 + `around_me` ±3, chỉ nickname/avatar | F106-AC-12, F106-AC-17, F106-AC-18, F106-AC-21 |
| 12 | Replay `match.completed` và `activity.graded` | Điểm bảng không tăng lần hai | F106-AC-13 |
| 13 | `valkey-cli DEL` key bảng rồi gọi `getLeaderboard` | `stale = true` → sau rebuild khớp sổ cái | F106-AC-14 |
| 14 | Đồng hồ giả tới 00:00 thứ Hai ICT | Snapshot top 100 + rank mọi học sinh lớp; `leaderboard.period_closed` 1 lần; `getLeaderboardHistory` trả kỳ cũ | F106-AC-15, F106-AC-21 |
| 15 | `hs.an.3a` hoàn thành 12 hoạt động (2 gắn cờ `rapid_guess`), 1 bài ôn gợi ý | `effort_points` theo bảng, bỏ attempt `rapid_guess`, ≤ trần ngày; definition scope `class` metric `xp` bị từ chối | F106-AC-16 |
| 16 | `hs.an.3a` (lớp 3) mở "Mục tiêu lớp"; `hs.em.4a` (lớp 4) ngoài top 10 | Lớp 3: `visibility = class_goal`, không rank cá nhân; lớp 4: "nhóm X%", không hiển thị người cuối | F106-AC-17 |
| 17 | Bật bảng Toàn hệ thống khi trường có `allow_cross_school_listing = false` | Học sinh trường đó không xuất hiện; response không tên thật/`student_code` | F106-AC-18 |
| 18 | `ad.truong.a` tắt "Lớp em tuần này" (cài đặt trường) | Học sinh trường A không thấy bảng; gọi trực tiếp → `404` | F106-AC-19 |
| 19 | `hs.chi.3b` đọc bảng lớp 3A; `hs.dung.7a` đọc bảng trường TH Demo A; `ad.truong.b` đọc bảng lớp 3A; header tenant sai | `404`; `404`; `404`; `403 TENANT_MISMATCH` | F106-AC-20 |
| 20 | `getLeaderboard` với `period_key` 100 ký tự | `422 VALIDATION_FAILED` | F106-AC-21 |
| 21 | Đồng hồ giả qua `ends_at` mùa, rồi `starts_at` mùa mới | `season.ended`, `season_results` (tier cao nhất); rating mới theo `season_soft_reset`, points từ 0; `listSeasons` trả mùa cũ | F106-AC-22, F106-AC-23, F106-AC-24 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=competition
make lint MOD=competition
make test MOD=competition
make test-integration MOD=competition
make migrate-lint MOD=competition
```

## 4. Bằng chứng cần nộp

- Kết quả test vector Glickman; số test pass/fail `ranking`, `leaderboard`, `season`; thời gian rebuild 50.000 học sinh (SC-005); contract test quét trường rating.

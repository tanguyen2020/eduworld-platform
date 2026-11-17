# Quickstart: F077 — Event & Season live-ops: vòng đời sự kiện, community goal, season track, sự kiện trường

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F072, F074, F075 (F078 nếu publish qua HTTP; nếu chưa, dùng `seed-config`). Season track cần `season.started` (F106 hoặc fixture).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events` (2 instance cho kịch bản scheduler), `make seed-check` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Seed config: sự kiện `world_festival` khối 3–5 (`th-demo-a`, `pgd-demo`) có điểm sự kiện, 2 mốc, community goal scope `school`; season track 30 cấp; template sự kiện trường gán `th-demo-a`.
- Ở local đặt `start_at`/`end_at`/`claim_until` của sự kiện seed cách hiện tại vài phút để quan sát chuyển trạng thái; integration test dùng fake clock (`src/backend/pkg/clock`).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (`th-demo-a`), `hs.dung.7a` (khối 7), `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`, `sa.admin` (vận hành, MFA tắt ở local).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Nạp sự kiện, tua tới `start_at` với 2 worker | Một chuyển `scheduled → live`, `event.started`, manifest tenant có thành phần sự kiện | F077-AC-01, F077-AC-02 |
| 2 | `hs.an.3a` và `hs.dung.7a` gọi `listGameEvents`, `getGameEvent` | `hs.an.3a` thấy; `hs.dung.7a` `404 EVENT_NOT_AVAILABLE` | F077-AC-07 |
| 3 | `hs.an.3a` hoàn thành quest sự kiện; phát lại event | Tham gia tạo một lần; điểm sự kiện cộng một lần; `event.progress.updated` | F077-AC-08, F077-AC-09 |
| 4 | `claimEventMilestone` 2 lần cùng key; mốc chưa đạt | Một grant + audit; `409 EVENT_MILESTONE_NOT_REACHED` | F077-AC-10 |
| 5 | `sa.admin` `suspendEventRun` (lý do) khi `hs.binh.3a` đang làm quest sự kiện | Thành phần ẩn; quest đang dở hoàn thành; không thưởng mới; audit có lý do | F077-AC-03, F077-AC-04 |
| 6 | Token `ad.truong.a` gọi `suspendEventRun` | `403`, `authz.denied` | F077-AC-06 |
| 7 | Phát lại đóng góp cùng `source_event_id`; vượt trần ngày; xóa key `ew:game:cgoal:*` | Không cộng thêm; tổng dựng lại khớp PostgreSQL | F077-AC-13, F077-AC-14 |
| 8 | Đẩy tổng tới mốc 50% | `community_goal.milestone_reached` một lần; `hs.an.3a` (có đóng góp) nhận thưởng, `hs.chi.3b` (không đóng góp) không | F077-AC-15, F077-AC-16 |
| 9 | Tua quá `claim_until` | Mốc đạt chưa nhận được tự cấp; run `archived` | F077-AC-05 |
| 10 | Phát fixture `season.started`; tích điểm mùa vượt trần; `claimSeasonTier` 2 lần; phát `season.ended` | Trần ngày đúng; một grant; thưởng chưa nhận tự cấp | F077-AC-17, F077-AC-18, F077-AC-19, F077-AC-20 |
| 11 | `ad.truong.a` `createSchoolEvent` với tham số ngoài template; tên chứa từ thuộc danh mục thử (placeholder); sự kiện thứ 4 | `422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED`; `422` không nêu từ; `409` | F077-AC-22, F077-AC-23, F077-AC-24 |
| 12 | `ad.truong.b` đọc sự kiện của `ad.truong.a`; `qlcm.truong.a` tạo sự kiện | `404`; `403 PERMISSION_DENIED` | F077-AC-21, F077-AC-25 |
| 13 | Seed sự kiện platform thiếu `en`; lễ hội 5 ngày; bundle trang trí vượt ngân sách | `422 TRANSLATION_INCOMPLETE`; `422`; `422` | F077-AC-26, F077-AC-27, F077-AC-28 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test theo mã AC, log scheduler (thời điểm chuyển trạng thái so với `start_at`), truy vấn tổng đóng góp trước/sau xóa Valkey, audit của kịch bản 5 và 11.

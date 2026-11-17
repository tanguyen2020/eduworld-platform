# Quickstart: F080 — Game Analytics và Live Ops dashboard (workspace E)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F014, F015, F051, F052, F056, F077, F078, F079 (và các feature phát event nguồn).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); Prometheus trong compose observability.
- Tài khoản: `sa.admin`, tài khoản `liveops` seed [Proposed: `lo.vanhanh`], `ad.donvi`, `ad.truong.a`, `ad.truong.b`, `sm.antoan`; học sinh `hs.an.3a`, `hs.dung.7a`.
- Dữ liệu: golden dataset `src/backend/modules/game/tests/fixtures/analytics/golden/` (event giả danh), script phát event `make seed-analytics` [Proposed].

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` (Godot hoặc curl token game) gửi lô telemetry hợp lệ; lô có khóa lạ | `202`; `422` | F080-AC-01 |
| 2 | Gửi 31 lô/phút; dùng token portal | `429` + `Retry-After`; `403 GAME_ACCESS_STUDENT_ONLY` | F080-AC-02 |
| 3 | Tắt telemetry sản phẩm cho TH Demo A; gửi `screen_view` + `perf_sample` | Chỉ `perf_sample` vào raw; client-config trả cờ tắt | F080-AC-03 |
| 4 | Gửi `crash`; xem log | Log có `crash_signature`, không PII | F080-AC-04 |
| 4b | Tắt telemetry sản phẩm cho TH Demo A; `hs.an.3a` gửi `break_reminder_shown` (`reminder_no` 1 rồi 2) và gửi lại cùng `client_event_id`; gửi `reminder_no = 0` | 2 dòng raw `source = client` có `session_key`; lần gửi lại không thêm dòng; `422` | F080-AC-34 |
| 5 | Phát `game_session.started`, `quest.completed` 2 lần cùng `event_id` | 1 dòng raw/event, `player_key` không phải `student_id` | F080-AC-05 |
| 6 | Chạy quét schema analytics | Không cột định danh | F080-AC-06 |
| 7 | Yêu cầu ẩn danh `hs.an.3a` | Raw/`player_days` `player_key = NULL`; rollup không đổi | F080-AC-07 |
| 8 | Nạp golden dataset (gồm `break_reminder_shown`) → chạy rollup giờ + chốt ngày | Số khớp tính tay (kể cả `sessions_over_reminder`, `sessions_over_second_reminder`), `is_final = true` | F080-AC-08, F080-AC-10, F080-AC-35 |
| 9 | Phát event trễ 30 h và 50 h | 30 h tính lại; 50 h chỉ tăng metric | F080-AC-09 |
| 10 | `lo.vanhanh` gọi `getGameMetrics` 7/90/730 ngày | `grain` day/week/month, có `data_as_of` | F080-AC-11 |
| 11 | Gọi 731 ngày; 5 truy vấn đồng thời | `422`; `429 DASHBOARD_BUSY` | F080-AC-12 |
| 12 | Xuất CSV; 3 export cùng lúc | Link ký hết hạn đúng; export thứ 3 `429` | F080-AC-13 |
| 13 | `ad.donvi` gọi `/admin/analytics/game/metrics` qua gateway public | `404` gateway / `403` backend | F080-AC-14 |
| 14 | `ad.donvi` `getTenantGameSummary`; `ad.truong.a` lọc trường khác | Chỉ `pgd-demo`; `403`/`404` | F080-AC-15 |
| 15 | Trường có 3 học sinh hoạt động | Ô `null`, `suppressed_cells` > 0 | F080-AC-16 |
| 16 | Mở `/live-ops/dashboard`; chạy bot CCU | KPI đổi ≤ 30 s, "Cập nhật lúc"; dừng Prometheus → thông báo lỗi, không số 0 | F080-AC-17, F080-AC-20 |
| 17 | Quét `/metrics` các binary (`core-api`, `realtime`, `worker`) | Đủ metric FR-OPS-003, nhãn không user id | F080-AC-18 |
| 18 | Mở Giám sát sự kiện, Tạm dừng không lý do rồi có lý do | Bị chặn; xác nhận 2 bước, `Idempotency-Key` | F080-AC-19 |
| 19 | Mở Game Analytics, đổi bộ lọc, so sánh kỳ | 11 panel, widget tải độc lập | F080-AC-21 |
| 20 | Chọn khoảng 3 năm; giả lập `DASHBOARD_BUSY` | Date picker chặn; tự thử lại ≤ 2 lần | F080-AC-22 |
| 21 | Bấm quest bị bỏ nhiều; bấm Xuất | Mở item ở workspace A; job xuất báo khi xong | F080-AC-23 |
| 22 | `sm.antoan`, `ad.donvi` mở `/live-ops/analytics` | Trang 403; `ad.donvi` không tải chunk | F080-AC-24 |
| 23 | Token không MFA; tắt gateway nội bộ | Step-up; "Cần kết nối mạng quản trị (VPN)" | F080-AC-25 |
| 24 | `client_versions` 21 phần tử; lý do dừng sự kiện quá dài | `422 VALIDATION_FAILED` | F080-AC-26 |
| 25 | `ad.donvi` mở Hoạt động game (workspace C), 30 ngày, lọc trường, khối | 5 nhóm widget tải độc lập; không có Xu/danh sách học sinh | F080-AC-27, F080-AC-28 |
| 26 | Nhóm 3 học sinh (seed rollup `pgd-demo`) | Ô "< 5" + chú thích số ô ẩn | F080-AC-29 |
| 27 | Chọn khoảng 731 ngày; giả lập `429` | Gợi ý thu hẹp; tự thử lại tối đa 2 lần | F080-AC-30 |
| 28 | Quét query catalog + EXPLAIN field mới của `TenantGameSummary` | Chỉ bảng `analytics.rollup_*`/`funnel_daily`/`perf_daily`, replica | F080-AC-31 |
| 29 | `ad.truong.a` lọc trường THCS Demo B; `qlcm.truong.a` scope môn gọi API | `403 TENANT_MISMATCH`/`PERMISSION_DENIED` | F080-AC-32 |
| 30 | `sa.admin` tắt mặc định platform `action.live_ops.game_analytics.export`; người dùng live-ops mở Game Analytics | Nút "Xuất dữ liệu" ẩn/khóa; `createGameAnalyticsExport` → `403 FEATURE_DISABLED`; các panel vẫn xem được | F080-AC-33 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=game
make lint MOD=game
make test MOD=game
make test-integration MOD=game
make migrate-lint MOD=game
cd src/cms/frontend && pnpm --filter portal test -- src/workspaces/live-ops && pnpm --filter portal exec playwright test tests/e2e/live-ops tests/e2e/school-lms/game-report.spec.ts
```

## 4. Bằng chứng cần nộp

- Kết quả test; bảng so golden dataset; EXPLAIN query catalog; kết quả tải k6 (p95); ảnh Live dashboard, Game Analytics, Hoạt động game (workspace C) sáng/tối; `trace_id` kịch bản 10, 12, 18.

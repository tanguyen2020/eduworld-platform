# Quickstart: F043 — Rollup analytics và School Learning Dashboard API

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F031, F041 (và F032 cho khối tự học & gợi ý).
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); local dùng primary với role `learning_dashboard`.
- Dữ liệu: chạy kịch bản học sinh làm bài của F041 (3A: `hs.an.3a`, `hs.binh.3a`; 3B: `hs.chi.3b`) hoặc nạp golden dataset `src/backend/modules/learning/tests/fixtures/analytics/golden/`.
- Tài khoản demo: `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a` (scope môn Toán), `ad.donvi` (tenant_admin `pgd-demo`), `sa.admin`, `sm.antoan`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Học sinh 3A nộp bài; chờ ≤ `school_dashboard_refresh` | `progress_daily`, `rollup_class_daily` có dòng hôm nay `is_final=false` | F043-AC-01, F043-AC-05, F043-AC-06 |
| 2 | Phát lại một event đã xử lý | Không đếm hai lần | F043-AC-02 |
| 3 | Chạy job `dashboard-day-close` thủ công; gửi event muộn 30 giờ và 50 giờ | Ngày cũ `is_final=true`; muộn 30 giờ được chốt lại; 50 giờ tăng metric late | F043-AC-03, F043-AC-04 |
| 4 | `ad.truong.a` gọi `getSchoolDashboard?school_id=<TH Demo A>&from=-7d&to=today` | Đủ khối bài giao, LO yếu, cần chú ý, tự học; `is_partial=true`; không có XP/rating/chat | F043-AC-07, F043-AC-08, F043-AC-09, F043-AC-11 |
| 5 | `getClassDashboard` cho 3A; `getSchoolAnalyticsSeries` 90 ngày | Heatmap lớp × LO; chuỗi `grain=week` ≤ 120 điểm | F043-AC-10, F043-AC-12, F043-AC-17 |
| 6 | `qlcm.truong.a` gọi dashboard môn Tiếng Việt; `ad.truong.b` gọi với `school_id` TH Demo A; `sm.antoan` gọi | `403 PERMISSION_DENIED`; `404`; `403 PERMISSION_DENIED` | F043-AC-13, F043-AC-14, F043-AC-15 |
| 7 | Gọi với khoảng 731 ngày; thiếu `from`; `page_size=500` | `422 DASHBOARD_RANGE_TOO_LARGE`; trang ≤ 100 | F043-AC-16 |
| 8 | Mở 5 truy vấn song song một user; truy vấn cố ý > 5 s (fixture) | `429 DASHBOARD_BUSY`; `504 DASHBOARD_QUERY_TIMEOUT` | F043-AC-19 |
| 9 | Tắt Valkey rồi gọi lại; dừng replica (staging) | Vẫn trả đúng số liệu; replica dừng → 503 | F043-AC-20, F043-AC-21 |
| 10 | `ad.truong.a` gọi `getStudentLearningProfile` cho `hs.an.3a`; `ad.truong.b` gọi cùng id | Có dữ liệu + audit `learning.profile_viewed`; `404` | F043-AC-23, F043-AC-24 |
| 11 | `ad.donvi` gọi `getTenantLearningSummary` (gửi kèm `tenant_ids` của `th-demo-a`) | Chỉ trường `pgd-demo`; ô < 5 học sinh hiện ẩn; `tenant_ids` bị bỏ qua | F043-AC-25, F043-AC-27 |
| 12 | `sa.admin` gọi `getPlatformLearningSummary?tenant_ids=…` qua gateway nội bộ | Có `tenant_id` mỗi dòng, audit `cross_tenant.accessed` | F043-AC-26 |
| 13 | Chạy kiểm thử tải trên dữ liệu 2 năm (staging) | p95 ≤ 2 s, không truy vấn > 5 s; EXPLAIN không Seq Scan | F043-AC-18, F043-AC-22 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make lint MOD=learning
make test MOD=learning
make test-integration MOD=learning
make migrate-lint MOD=learning
```

## 4. Bằng chứng cần nộp

- Kết quả golden dataset (bảng so sánh), báo cáo EXPLAIN của query catalog, kết quả tải TC-PERF-025 (p95, max), trace_id kịch bản 4 và 12, bản ghi audit `cross_tenant.accessed`, `learning.profile_viewed`.

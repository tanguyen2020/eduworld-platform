# Báo cáo tải (perf reports)

> Trạng thái: Draft · Owner: Tech Lead · Cập nhật: 2026-11-27 (F132)
> Liên quan: [TEST_STRATEGY §8](../TEST_STRATEGY.md#8-performance--load-test-plan) · [DEPLOYMENT §4.2](../../11-operations/DEPLOYMENT.md) · [F132](../../../specs/132-load-chaos-pentest-anticheat/spec.md)

**Mục đích.** Thư mục chứa báo cáo của từng lần chạy tải trên **staging** (F132-AC-06), tên `<YYYY-MM-DD>-<run>.md`. Báo cáo do [`scripts/perf/render-report.mjs`](../../../scripts/perf/render-report.mjs) sinh; không viết tay số liệu.

**Chưa có lần chạy nào trên staging.** Staging tương đương production (F130) chưa tồn tại, nên chưa có báo cáo và **chưa có năng lực nào được công bố** (ADR-0045 §7). Lần chạy `local`/`dev` chỉ là smoke, báo cáo nằm ở `tmp/perf/<run_id>/report.md` (bị `.gitignore`) và không bao giờ được chép vào đây.

## 1. Chạy

| Bước | Lệnh |
|---|---|
| Sinh dữ liệu giả (tenant `perf-*`) rồi nạp qua CSV import của module identity | `make datagen ENV=staging TENANTS=20 SCHOOLS_PER_TENANT=5 STUDENTS=50000` |
| Một kịch bản | `make perf RUN=<run> ENV=staging` (danh sách `run` ở [`tests/load/scenarios.yaml`](../../../tests/load/scenarios.yaml)) |
| Scale độc lập (F132-AC-05) | `make perf RUN=scale_independent ENV=staging SCALE_SERVICE=core-api=<peak>@600` |
| Theo lịch / trước release | workflow `perf.yml` (job `staging`, runner `[self-hosted, deploy, loadgen]`, cổng `vars.PERF_ENABLED`) |

Runner tải đặt ngoài cụm: [tests/load/runner/README.md](../../../tests/load/runner/README.md).

## 2. Nội dung bắt buộc của một báo cáo (data-model §1)

| Mục | Nguồn |
|---|---|
| `run_id`, kịch bản (mã TC), thời gian | `scripts/perf/run.sh` |
| Môi trường + sizing theo vai trò node | `EW_SIZING_FILE` (Tech Lead điền theo cụm thật) |
| Tag backend `backend/vX.Y.Z`, commit, digest từng image | `docker service ls` của stack trên manager staging |
| Warm-up: `ops-scheduler` đã đưa mọi service lên `peak` (TC-PERF-017) | `scripts/perf/warmup-check.mjs` (bắt buộc cho TC-PERF-002, TC-PERF-014) |
| Ngưỡng k6 (p95, tỷ lệ lỗi, bộ đếm) | `k6 --summary-export`, ngưỡng từ `scenarios.yaml` |
| Độ trễ phía bot (p50/p95/p99) | `loadbot -summary` |
| Kiểm tra Prometheus (WS/replica, circuit breaker, lag, pool, CPU Keycloak…) | `scripts/perf/prom-checks.mjs` trên Prometheus staging (F131) |
| CPU máy tải | `scripts/perf/check-generator-saturation.sh` (node_exporter `job="loadgen"`) |
| Biểu đồ | link Grafana theo `testid` = `run_id` |
| Kết luận + điểm nghẽn | `render-report.mjs` + phân tích của Tech Lead |

## 3. Đọc kết luận

| Kết luận | Khi nào |
|---|---|
| **đạt** | Mọi ngưỡng k6 và kiểm tra Prometheus đạt, máy tải không bão hòa, warm-up đạt (nếu áp dụng) |
| **không đạt** | Một ngưỡng hay kiểm tra không đạt, hoặc một kiểm tra **không có số liệu** (thiếu metric không bao giờ là đạt) |
| **không hợp lệ** | Máy tải vượt `perf_generator_cpu_max` (giới hạn đo được là của máy tải), hoặc thiếu số đo CPU máy tải |

Chỉ một báo cáo TC-PERF-014 **đạt** trên staging được ghi câu "đạt `target_ccu`"; khi đó hiệu chỉnh bảng sizing [DEPLOYMENT §4.2](../../11-operations/DEPLOYMENT.md) theo số đo trong cùng MR. Không tài liệu nào khác được ghi đã đạt `target_ccu` khi chưa có báo cáo như vậy (F132-AC-06).

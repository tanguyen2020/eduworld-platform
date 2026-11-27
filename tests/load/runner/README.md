# Runner tải (F132 T003)

Máy phát tải chạy **ngoài cụm Swarm** để không tranh tài nguyên với hệ thống đang đo (plan F132, ADR-0024). Trạng thái: **chờ môi trường** — staging tương đương production (F130) và các VM runner chưa có; mọi bước dưới đây đã được viết thành script nhưng chưa chạy trên staging.

## 1. Cấu hình

| Mục | Giá trị |
|---|---|
| Số VM | ≥ 3 VM 16 vCPU (plan F132; số cuối cùng chốt theo lần chạy đầu: mỗi VM giữ CPU dưới `perf_generator_cpu_max`) |
| Vị trí | Cùng vùng mạng với staging, đi vào qua APISIX public như client thật (không gọi thẳng `core-api`) |
| Runner GitHub Actions | self-hosted, nhãn `[self-hosted, deploy, loadgen]`, gắn GitHub Environment `staging` (ADR-0088) |
| Phần mềm | Docker (image `grafana/k6:0.57.0`), Go (build `loadbot`), Node 20 (script báo cáo), node_exporter |
| Giám sát | Prometheus staging scrape node_exporter của VM với `job="loadgen"` → `check-generator-saturation.sh prom` |
| Metric tải | k6 remote write (`K6_PROMETHEUS_RW_SERVER_URL`) và `loadbot` `/metrics` (`eduworld_loadbot_*`), nhãn `testid`/`scenario` |
| Sizing của lần chạy | `tests/load/runner/staging-sizing.json` (Tech Lead điền theo cụm thật trước lần chạy; không có số mặc định) |

## 2. Bí mật và biến (GitHub Environment `staging`)

| Tên | Loại | Dùng cho |
|---|---|---|
| `LOADBOT_PASSWORD` | secret | Mật khẩu chung của tài khoản bot `perf-*` (đặt khi nạp dữ liệu) |
| `SWARM_TLS_CA`, `SWARM_TLS_CERT`, `SWARM_TLS_KEY` | secret | Đọc digest image, scale một service (F132-AC-05) |
| `STAGING_API_BASE_URL`, `PROMETHEUS_URL`, `PROMETHEUS_RW_URL`, `GRAFANA_URL`, `SWARM_MANAGER_HOST` | variable | Đích tải và nguồn số liệu |
| `PERF_ENABLED` | variable | Bật job `staging` của `perf.yml` khi staging sẵn sàng |
| `PERF_TENANTS`, `PERF_SCHOOLS_PER_TENANT`, `PERF_STUDENTS`, `PERF_SEED` | variable | Cùng tham số với lần `make datagen` đã nạp |

## 3. Trình tự một lần chạy

1. `make datagen ENV=staging …` một lần, nạp `perf-*/schools.csv` + `students.csv` qua CSV import (F011), nhân sự từ `staff.csv` qua API identity; đặt mật khẩu bot.
2. Chờ khung warm-up (`warm_up_windows`) hoặc kích `ops-scheduler` lên `peak` (TC-PERF-017).
3. `workflow_dispatch` `perf.yml` với `run` (hoặc `make perf RUN=<run> ENV=staging` trên runner).
4. Lấy artifact (thư mục `tmp/perf/<run_id>/` + báo cáo), commit báo cáo vào `docs/12-project/perf-reports/`.

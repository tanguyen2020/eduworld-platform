# Quickstart: F131 — Monitoring, alerting, dashboard vận hành, runbook

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F002 (`src/backend/pkg/otel`, log JSON), F130 (staging Swarm, network `admin`, metric backup/scale), F004 (plugin APISIX, client Keycloak cho công cụ).
- Kiểm cấu hình nhanh ở local: `make up-obs` (F001) + `make obs-check` (promtool, amtool, script `scripts/obs/*`). Nghiệm thu trên **staging**.
- Tài khoản: `sa.admin` (system_admin — Grafana Admin), một nhân sự vận hành platform (role `platform_ops`), `ad.truong.a` (school_admin — phải bị từ chối), học sinh QA `hs.qa.probe` (tenant QA, chỉ staging/production) cho probe; `hs.an.3a` tạo lưu lượng mẫu.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Mở Prometheus `Status → Targets` trên staging | Mọi target (`core-api`, `realtime`, `worker-*`, exporter, APISIX, Keycloak, NATS, PG, Valkey, node, cAdvisor, swarm-exporter) `up == 1` | F131-AC-01 |
| 2 | `make obs-metric-catalog ENV=staging` | Exit 0; xóa tạm một metric khỏi build thử → job liệt kê metric thiếu | F131-AC-02 |
| 3 | `hs.an.3a` làm một quest (`core-api`: module game → facade learning qua `src/backend/pkg/modcall` → event NATS → `worker-events`); lấy `trace_id` từ Problem Details lỗi cố ý | Tempo có trace đủ span gateway/server/client/producer/consumer/DB; Loki có log cùng `trace_id` | F131-AC-03 |
| 4 | Gửi 100 request thường + 5 request lỗi | Tempo giữ đủ 5 trace lỗi, ~10% trace thường | F131-AC-04 |
| 5 | Ghi log thử chứa JWT/email/SĐT giả từ endpoint test của `core-api` | Loki chỉ thấy giá trị đã che; retention Loki = `app_log_retention` | F131-AC-05 |
| 6 | Thêm label `school_id` vào rule thử | `obs-label-policy` đỏ | F131-AC-06 |
| 7 | Đăng nhập Grafana bằng `ad.truong.a`; bằng nhân sự vận hành không MFA | Cả hai bị từ chối; `sa.admin` (MFA) vào được; truy vấn Loki có nhật ký | F131-AC-07 |
| 8 | `make obs-rules` | `promtool test rules` pass mọi alert; xóa một rule → job báo thiếu so với MONITORING §5 | F131-AC-08, F131-AC-12 |
| 9 | Dừng mọi replica `worker-chat` trên staging | Một thông báo `ServiceDown{service="worker",profile="chat"}` tới kênh trực; alert con bị inhibit | F131-AC-09 |
| 10 | Dừng Alertmanager 15 phút | Kênh ngoài báo mất heartbeat `Watchdog` | F131-AC-10 |
| 11 | `make alert-drill ENV=staging` | Mọi alert critical tới kênh trực ≤ 2 phút; báo cáo `alert-drills/<YYYY-MM>.md` | F131-AC-11 |
| 12 | `make obs-runbooks`; xóa mục "Leo thang" của một runbook | Job đỏ | F131-AC-13 |
| 13 | Mở 13 dashboard sau 10 phút lưu lượng mẫu | Panel có dữ liệu; `obs-dashboards` pass | F131-AC-14 |
| 14 | Truy vấn recording rule SLO; tạo 5xx giả lập vượt burn rate | Giá trị SLI hợp lệ; `SLOBurnRateFast` bắn | F131-AC-15 |
| 15 | Mở dashboard Chat Safety | Chỉ số đếm/tỷ lệ, không nội dung, không tên học sinh | F131-AC-16 |
| 16 | Gây panic có recover ở endpoint test staging | Issue GlitchTip có `trace_id`, `service`, `module`, không body; một GitHub Issue | F131-AC-17, F131-AC-18 |
| 17 | Chặn route `/api/v1/game/client-config` ở APISIX staging 15 phút | `eduworld_synthetic_probe_success{probe="client_config"} = 0`, `SyntheticProbeFailing` bắn sau 2 chu kỳ | F131-AC-19 |
| 18 | Hạ ngưỡng toàn cục login staging để probe nhận `429 LOGIN_QUEUE_BUSY` | Probe ghi `result="rate_limited"`, chờ `Retry-After`; không attempt nào được tạo | F131-AC-20 |

## 3. Lệnh test tự động

```bash
make obs-check                      # promtool check/test rules, amtool check-config, scripts/obs/*
node --test scripts/obs/*.test.mjs
go -C src/backend test ./tools/swarm-exporter/...
make obs-metric-catalog ENV=staging
```

## 4. Bằng chứng cần nộp

- Kết quả `promtool test rules`, báo cáo diễn tập alert, ảnh trace xuyên binary/module + log cùng `trace_id`, ảnh 13 dashboard, issue GlitchTip mẫu (không PII).

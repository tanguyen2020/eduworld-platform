# DashboardSlow

> Runbook của alert `DashboardSlow` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/dashboard.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _p95 dashboard queries of module <module> above 2 s for 15 minutes_
- Điều kiện (MONITORING §5): p95 `eduworld_dashboard_query_duration_seconds` > 2 s trong 15 phút (giờ học)

## Tác động

Dashboard trường chậm > 2 s p95.

## Chẩn đoán

1. `query_id` chậm, EXPLAIN, cache hit

## Xử lý

1. Tối ưu query/index; xem ngưỡng nâng cấp DASHBOARD_DATA §12

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [DASHBOARD_DATA](../../07-backend/DASHBOARD_DATA.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

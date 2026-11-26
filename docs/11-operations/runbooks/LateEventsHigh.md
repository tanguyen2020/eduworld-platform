# LateEventsHigh

> Runbook của alert `LateEventsHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/dashboard.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _More than 0.5% of the day's events arrive later than 48 hours_
- Điều kiện (MONITORING §5): `eduworld_rollup_late_events_total{bucket=">48h"}` > 0,5% event trong ngày

## Tác động

Nhiều event đến muộn > 48 h: rollup không sửa lại được.

## Chẩn đoán

1. Client offline lâu, consumer chậm

## Xử lý

1. Cân nhắc backfill có chủ đích

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [DASHBOARD_DATA](../../07-backend/DASHBOARD_DATA.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

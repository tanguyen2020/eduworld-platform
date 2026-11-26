# RollupLagHigh

> Runbook của alert `RollupLagHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/dashboard.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Rollup <rollup> of module <module> is behind_
- Điều kiện (MONITORING §5): `eduworld_rollup_lag_seconds` > 900 s (learning) hoặc > 3.600 s (game) trong 15 phút

## Tác động

Số liệu dashboard trễ.

## Chẩn đoán

1. Consumer rollup, NATS pending

## Xử lý

1. Scale/sửa consumer rollup

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [DASHBOARD_DATA](../../07-backend/DASHBOARD_DATA.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

# AnalyticsRawRowsHigh

> Runbook của alert `AnalyticsRawRowsHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/game.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _analytics.events_raw passed 400 million rows (80% of analytics_store_migration_threshold)_
- Điều kiện (MONITORING §5): `analytics.events_raw` > 400 triệu dòng (80% ngưỡng ADR-0058) trong 1 giờ

## Tác động

`analytics.events_raw` tới 80% ngưỡng chuyển kho.

## Chẩn đoán

1. Tốc độ tăng dòng

## Xử lý

1. Chuẩn bị ADR chuyển kho/lưu trữ lạnh (ADR-0058, ADR-0070)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [GAME_ANALYTICS](../GAME_ANALYTICS.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

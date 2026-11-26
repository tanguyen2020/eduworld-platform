# AnalyticsRollupSlow

> Runbook của alert `AnalyticsRollupSlow` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/game.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _A tenant day of the Game Analytics rollup takes more than 2 hours (p95)_
- Điều kiện (MONITORING §5): p95 rollup một ngày-tenant của Game Analytics > 2 giờ trong 1 giờ (§10, [GAME_ANALYTICS §10](GAME_ANALYTICS.md))

## Tác động

Rollup Game Analytics một ngày-tenant > 2 giờ: dashboard trễ.

## Chẩn đoán

1. Job `analytics-rollup-hourly`/`dashboard-day-close-game`, kích thước `events_raw`

## Xử lý

1. Tối ưu truy vấn/partition; xem ngưỡng chuyển kho GAME_ANALYTICS §10

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [GAME_ANALYTICS](../GAME_ANALYTICS.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

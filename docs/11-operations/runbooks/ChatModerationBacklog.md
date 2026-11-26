# ChatModerationBacklog

> Runbook của alert `ChatModerationBacklog` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _A high moderation case waits for more than 30 minutes_
- Điều kiện (MONITORING §5): Case `high` chờ > 30 phút

## Tác động

Case mức high chờ > 30 phút.

## Chẩn đoán

1. Hàng đợi Safety Team theo severity

## Xử lý

1. Báo ca trực Platform Safety Team (ADR-0072)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Trưởng ca Safety Team.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

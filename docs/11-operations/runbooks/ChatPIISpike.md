# ChatPIISpike

> Runbook của alert `ChatPIISpike` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _PII blocked in chat above 5× the daily baseline (incident or spam campaign?)_
- Điều kiện (MONITORING §5): Tỷ lệ PII blocked (`eduworld_chat_sensitive_matches_total{category="pii"}`) > 5× baseline 15 phút (sàn 1/phút [Proposed])

## Tác động

Lượng PII bị chặn tăng đột biến: sự cố hoặc chiến dịch spam.

## Chẩn đoán

1. Dashboard Grafana **Chat Safety** (chỉ số đếm, không nội dung)

## Xử lý

1. Safety Team xem case liên quan; siết rate limit nếu spam

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Safety Team lead.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [CHILD_SAFETY](../../10-security/CHILD_SAFETY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

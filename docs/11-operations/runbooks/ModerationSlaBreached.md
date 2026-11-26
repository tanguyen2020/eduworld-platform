# ModerationSlaBreached

> Runbook của alert `ModerationSlaBreached` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _A p1 moderation case missed its first-response SLA (report_sla: 1 hour)_
- Điều kiện (MONITORING §5): Case p1 chưa gán quá 1 giờ (`report_sla`, job `moderation-sla-escalator` đã leo thang)

## Tác động

Case p1 quá SLA phản hồi đầu (1 giờ).

## Chẩn đoán

1. Case đã `escalated` trong hàng đợi

## Xử lý

1. Trưởng ca nhận case; ghi biên bản

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Trưởng ca Safety Team, sau đó System Admin.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [CHILD_SAFETY](../../10-security/CHILD_SAFETY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

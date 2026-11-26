# ModerationSchoolNotificationsOverdue

> Runbook của alert `ModerationSchoolNotificationsOverdue` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _School safety notifications past ack_due_at without acknowledgement_
- Điều kiện (MONITORING §5): `eduworld_moderation_school_notifications_overdue > 0` trong 1 giờ

## Tác động

Trường chưa xác nhận thông báo vụ việc nghiêm trọng quá hạn.

## Chẩn đoán

1. `listSchoolSafetyNotificationsPlatform?overdue=true`

## Xử lý

1. Theo dõi trường theo MODERATION §6.4

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

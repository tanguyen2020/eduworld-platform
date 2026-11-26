# SchoolSafetyNotificationNoRecipient

> Runbook của alert `SchoolSafetyNotificationNoRecipient` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _A school safety notification has no School Admin or Tenant Admin to email_
- Điều kiện (MONITORING §5): `increase(eduworld_school_safety_notification_no_recipient_total[1h]) > 0` (trường không có School Admin/Tenant Admin nhận email, F124)

## Tác động

Trường không có School Admin/Tenant Admin nhận email thông báo vụ việc.

## Chẩn đoán

1. Danh sách quản trị của trường (portal)

## Xử lý

1. Gán School Admin (hoặc Tenant Admin)
2. Safety Team liên hệ trường trực tiếp (MODERATION §6.4)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Safety Team lead.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

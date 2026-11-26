# EmailDeliveryFailing

> Runbook của alert `EmailDeliveryFailing` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Staff emails of template <template> failed after email_delivery_max_attempts_
- Điều kiện (MONITORING §5): `increase(eduworld_email_deliveries_total{status="failed"}[15m]) > 0` (email nhân sự hết `email_delivery_max_attempts`, F124)

## Tác động

Email nhân sự (thông báo vụ việc an toàn) không gửi được sau `email_delivery_max_attempts`.

## Chẩn đoán

1. Relay SMTP (`EW_SMTP_HOST`/`EW_SMTP_PORT`; mailpit ở local)
2. `identity.email_deliveries` (`status = failed`, `last_error_code` chỉ là mã SMTP — không có địa chỉ)

## Xử lý

1. Thông báo vụ việc vẫn hiển thị trong portal của trường; với case p1, Safety Team liên hệ trực tiếp đầu mối an toàn của trường (MODERATION §6.4)
2. Sửa relay rồi theo dõi lượt gửi mới (`eduworld_email_deliveries_total{status="sent"}`); lượt `failed` không tự gửi lại

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Safety Team lead và người trực vận hành.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [CHILD_SAFETY](../../10-security/CHILD_SAFETY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

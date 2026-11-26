# AlertmanagerNotificationsFailing

> Runbook của alert `AlertmanagerNotificationsFailing` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/meta.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Alertmanager cannot deliver notifications through <integration>_
- Điều kiện (MONITORING §5): `alertmanager_notifications_failed_total` tăng liên tục 10 phút

## Tác động

Người trực không nhận được thông báo.

## Chẩn đoán

1. Log Alertmanager theo `integration`
2. Secret `ops_alert_*` còn hiệu lực? nhà cung cấp chat/SMS/email lỗi?

## Xử lý

1. Sửa secret (SOPS → `make secrets-sync`) hoặc kênh; thông báo người trực qua kênh dự phòng

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

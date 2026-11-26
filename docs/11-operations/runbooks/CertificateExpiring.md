# CertificateExpiring

> Runbook của alert `CertificateExpiring` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/edge.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _TLS certificate of <instance> expires in less than 14 days_
- Điều kiện (MONITORING §5): TLS cert hết hạn < 14 ngày

## Tác động

Chứng chỉ TLS sắp hết hạn: client sẽ không kết nối được.

## Chẩn đoán

1. `probe_ssl_earliest_cert_expiry` theo endpoint

## Xử lý

1. Gia hạn chứng chỉ (ACME/nhà cung cấp) và kiểm tra lại probe

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ trừ khi < 3 ngày.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

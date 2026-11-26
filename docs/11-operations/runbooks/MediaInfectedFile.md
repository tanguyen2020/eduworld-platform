# MediaInfectedFile

> Runbook của alert `MediaInfectedFile` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/content.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _The antivirus rejected an uploaded media file_
- Điều kiện (MONITORING §5): `eduworld_media_infected_total` tăng

## Tác động

Antivirus phát hiện file nhiễm trong upload.

## Chẩn đoán

1. Audit tài khoản upload (portal), không mở file

## Xử lý

1. Khóa tài khoản nếu cố ý; báo trường

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Báo Security lead.

## Liên quan

- [SECURITY_ARCHITECTURE](../../10-security/SECURITY_ARCHITECTURE.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

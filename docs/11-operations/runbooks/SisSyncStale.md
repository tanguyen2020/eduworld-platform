# SisSyncStale

> Runbook của alert `SisSyncStale` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _SIS connection <connection> has no successful sync for 26 hours_
- Điều kiện (MONITORING §5): `time() - eduworld_sis_last_success_timestamp` > 26 giờ theo connection ([SIS_INTEGRATION §8](../09-identity/SIS_INTEGRATION.md))

## Tác động

Danh sách học sinh/xếp lớp của trường không được cập nhật > 26 giờ.

## Chẩn đoán

1. `listSisSyncRuns` của connection (portal workspace platform)
2. `eduworld_sis_http_requests_total` theo status

## Xử lý

1. Chạy lại sync khi SIS của trường sẵn sàng
2. Báo trường nếu SIS của họ lỗi

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [SIS_INTEGRATION](../../09-identity/SIS_INTEGRATION.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

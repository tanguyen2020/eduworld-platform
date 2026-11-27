# CircuitBreakerOpen

> Runbook của alert `CircuitBreakerOpen` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/api.yaml` · Mức: **warning / critical**.

## Triệu chứng

- Thông báo: _Circuit breaker <module> → <integration> open for 2 minutes_
- Điều kiện (MONITORING §5): `eduworld_external_client_circuit_state == 2` > 2 phút cho một cặp `module→integration`

## Tác động

Tính năng phụ thuộc tích hợp ngoài (Keycloak, SIS, object storage) đang suy giảm; critical khi là Keycloak.

## Chẩn đoán

1. `eduworld_external_client_circuit_state{module,integration}` (2 = open, 1 = half-open): cặp nào mở, từ khi nào; Keycloak: `eduworld_identity_org_sync_total{result="circuit_open"}`, `eduworld_keycloak_token_call_duration_seconds`; log adapter theo `trace_id`
2. Hệ thống ngoài có sống không (Keycloak, SIS của trường, `storage-s3`)

## Xử lý

1. Khôi phục hệ thống ngoài; breaker tự half-open rồi đóng

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [SIS_INTEGRATION](../../09-identity/SIS_INTEGRATION.md)
- [runbooks/README](README.md)

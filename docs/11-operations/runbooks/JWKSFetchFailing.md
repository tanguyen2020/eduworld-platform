# JWKSFetchFailing

> Runbook của alert `JWKSFetchFailing` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _<service> cannot refresh the Keycloak JWKS for 5 minutes_
- Điều kiện (MONITORING §5): Lỗi tải JWKS > 5 phút

## Tác động

Binary không xác minh được token mới khi khóa Keycloak xoay vòng: đăng nhập/gọi API sẽ lỗi 401.

## Chẩn đoán

1. `eduworld_jwks_refresh_total` theo `service`/`result`
2. Kết nối từ binary tới Keycloak (network `edge`/`data`), Keycloak sống?

## Xử lý

1. Khôi phục Keycloak/network; binary dùng cache JWKS cho tới khi tải lại được

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [SECURITY_ARCHITECTURE](../../10-security/SECURITY_ARCHITECTURE.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

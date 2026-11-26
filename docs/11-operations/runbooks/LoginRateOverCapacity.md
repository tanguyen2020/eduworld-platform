# LoginRateOverCapacity

> Runbook của alert `LoginRateOverCapacity` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Login load beyond Keycloak capacity (over 110/s, or successes halved while errors grow)_
- Điều kiện (MONITORING §5): > 110/s trong 2 phút hoặc `rate(eduworld_auth_logins_total{result="ok",method="password"}[1m])` giảm > 50% trong khi login_error tăng

## Tác động

Vượt năng lực Keycloak: đăng nhập thất bại hoặc rất chậm cho nhiều trường.

## Chẩn đoán

1. CPU/DB Keycloak, lỗi `login_error`
2. Ngưỡng rate limit toàn cục 120/s đang áp

## Xử lý

1. Scale Keycloak + `core-api`
2. Nếu không đủ năng lực: hạ ngưỡng toàn cục để hàng chờ 429 bảo vệ hệ thống
3. Báo các trường đang đăng nhập đồng loạt (lịch học)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [SECURITY_ARCHITECTURE](../../10-security/SECURITY_ARCHITECTURE.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

# LoginRateNearCapacity

> Runbook của alert `LoginRateNearCapacity` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _More than 80 logins/s on Keycloak (80% of the 100/s design peak)_
- Điều kiện (MONITORING §5): `rate(keycloak_user_events_total{event="login"}[1m])` > 80/s trong 2 phút (80% đỉnh thiết kế 100/s, ADR-0064 mục 6)

## Tác động

Tải đăng nhập > 80% đỉnh thiết kế 100/s.

## Chẩn đoán

1. `keycloak_user_events_total{event="login"}`
2. Hàng chờ 429 (`eduworld_auth_ratelimit_rejected_total{scope="global"}`) có hoạt động

## Xử lý

1. Scale Keycloak lên 6 replica
2. Xác nhận hàng chờ đăng nhập trả `429 LOGIN_QUEUE_BUSY` thay vì quá tải

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

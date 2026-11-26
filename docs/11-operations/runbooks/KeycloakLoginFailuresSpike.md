# KeycloakLoginFailuresSpike

> Runbook của alert `KeycloakLoginFailuresSpike` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Failed game logins above 5× the daily baseline (brute force or IdP failure?)_
- Điều kiện (MONITORING §5): Login game thất bại (`eduworld_auth_game_login_total{result=~"invalid_credentials|account_locked"}`) > 5× baseline trong 10 phút (sàn 1/phút [Proposed])

## Tác động

Nhiều lượt đăng nhập game thất bại (sai mật khẩu, khóa tài khoản).

## Chẩn đoán

1. `eduworld_auth_game_login_total` theo `result`
2. Có trùng giờ đầu tiết của một trường (học sinh quên mật khẩu)?

## Xử lý

1. Brute force: siết rate limit theo username/tenant
2. Lỗi IdP: kiểm tra Keycloak

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [SECURITY_ARCHITECTURE](../../10-security/SECURITY_ARCHITECTURE.md)
- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

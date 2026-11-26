# AccountNotProvisionedSpike

> Runbook của alert `AccountNotProvisionedSpike` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _More than 50 logins/hour refused with ACCOUNT_NOT_PROVISIONED_
- Điều kiện (MONITORING §5): `eduworld_auth_logins_total{result="not_provisioned"}` > 50/giờ ở một trường

## Tác động

Học sinh có tài khoản IdP nhưng chưa được cấp trong EduWorld bị từ chối đăng nhập.

## Chẩn đoán

1. `eduworld_auth_logins_total{result="not_provisioned"}` theo tenant
2. SIS sync của tenant: `SisSyncStale`, `SISSyncFailed`

## Xử lý

1. Sửa SIS sync hoặc cấp tài khoản theo ACCOUNT_PROVISIONING
2. Báo School Admin của trường (ADR-0063)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ; ticket cho đội identity.

## Liên quan

- [ACCOUNT_PROVISIONING](../../09-identity/ACCOUNT_PROVISIONING.md)
- [SIS_INTEGRATION](../../09-identity/SIS_INTEGRATION.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

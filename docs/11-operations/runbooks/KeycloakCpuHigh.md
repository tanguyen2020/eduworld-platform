# KeycloakCpuHigh

> Runbook của alert `KeycloakCpuHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Keycloak container <name> uses more than 75% of its CPU limit_
- Điều kiện (MONITORING §5): CPU Keycloak > 75% trong 5 phút

## Tác động

Keycloak gần bão hòa CPU; đăng nhập sẽ chậm.

## Chẩn đoán

1. CPU theo container (cAdvisor), số lượt đăng nhập/s
2. JVM GC (`jvm_*`)

## Xử lý

1. Scale thêm replica Keycloak trên node `idp`
2. Kiểm tra warm-up theo khung giờ cao điểm (`warm_up_windows`)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Trong giờ học kéo dài > 30 phút: gọi người trực secondary.

## Liên quan

- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

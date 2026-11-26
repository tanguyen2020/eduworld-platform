# LoginErrorSpike

> Runbook của alert `LoginErrorSpike` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Keycloak login errors above 5× the daily baseline (credential stuffing?)_
- Điều kiện (MONITORING §5): `keycloak_user_events_total{event="login_error"}` tăng > 5× baseline 15 phút (baseline = tốc độ 1 ngày trước; sàn 1 lỗi/phút [Proposed])

## Tác động

Có thể là credential stuffing hoặc IdP lỗi.

## Chẩn đoán

1. `keycloak_user_events_total{event="login_error"}` theo realm/client
2. Brute force detection của Keycloak; phân bố theo trường (không theo IP đơn lẻ — NAT trường học)

## Xử lý

1. Nếu tấn công: siết rate limit theo tenant/username (không chặn IP đơn lẻ, ADR-0045)
2. Nếu IdP lỗi: báo nhà cung cấp/đội identity

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Nghi tấn công: báo Security lead.

## Liên quan

- [SECURITY_ARCHITECTURE](../../10-security/SECURITY_ARCHITECTURE.md)
- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

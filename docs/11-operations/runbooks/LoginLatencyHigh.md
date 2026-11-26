# LoginLatencyHigh

> Runbook của alert `LoginLatencyHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _p95 password login → hub above 3 s for 5 minutes (login_slo)_
- Điều kiện (MONITORING §5): p95 `eduworld_auth_login_duration_seconds{method="password"}` > 3 s trong 5 phút (giờ học)

## Tác động

Học sinh chờ đăng nhập > 3 s (vi phạm `login_slo`), dễ gây đăng nhập lại hàng loạt.

## Chẩn đoán

1. Dashboard Grafana **Identity**: login theo result, Keycloak events
2. CPU/RAM Keycloak (`KeycloakCpuHigh`), DB Keycloak, `eduworld_keycloak_token_call_duration_seconds`
3. Hàng chờ `eduworld_auth_waiting_room_queued`

## Xử lý

1. Scale Keycloak (tối đa 6 replica, ADR-0064) và `core-api`
2. Kiểm tra warm-up theo lịch (`scale-schedule.yaml`) đã chạy chưa (`ScheduledScaleFailed`)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

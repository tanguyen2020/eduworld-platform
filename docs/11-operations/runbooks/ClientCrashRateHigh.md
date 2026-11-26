# ClientCrashRateHigh

> Runbook của alert `ClientCrashRateHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/client.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Crash rate of client <client_version> above 2% of sessions_
- Điều kiện (MONITORING §5): Crash rate theo `client_version` > 2% phiên trong 1 giờ (`eduworld_client_crashes_total` / `eduworld_client_sessions_total`)

## Tác động

Phiên bản client mới crash > 2% phiên.

## Chẩn đoán

1. Dashboard Grafana **Client**; crash report (GlitchTip, không PII)

## Xử lý

1. Cân nhắc dừng rollout store; báo đội Godot

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [PERFORMANCE_BUDGET](../../08-game-client/PERFORMANCE_BUDGET.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

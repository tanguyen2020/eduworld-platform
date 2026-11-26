# GameTakeoverSpike

> Runbook của alert `GameTakeoverSpike` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/game.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Game session takeovers above 5× the daily baseline (shared accounts or leaked passwords?)_
- Điều kiện (MONITORING §5): `eduworld_game_session_takeovers_total` tăng > 5× baseline 1 h (sàn 10 lượt/giờ [Proposed])

## Tác động

Nhiều phiên game bị đăng nhập thiết bị khác: dùng chung tài khoản hoặc lộ mật khẩu hàng loạt.

## Chẩn đoán

1. `eduworld_game_session_takeovers_total` theo platform; tập trung một trường?

## Xử lý

1. Báo trường; yêu cầu đổi mật khẩu nếu nghi lộ

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Nghi lộ hàng loạt: báo Security lead.

## Liên quan

- [SECURITY_ARCHITECTURE](../../10-security/SECURITY_ARCHITECTURE.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

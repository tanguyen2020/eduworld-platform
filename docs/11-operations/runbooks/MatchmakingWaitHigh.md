# MatchmakingWaitHigh

> Runbook của alert `MatchmakingWaitHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/competition.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Matchmaking wait p95 > 60 s (<mode>)_
- Điều kiện (MONITORING §5): p95 `mm_wait_seconds{result="matched"}` > 60 s trong 15 phút (thống nhất [MATCHMAKING §10](../03-social-multiplayer/MATCHMAKING.md), F104)

## Tác động

Học sinh chờ ghép trận lâu (> 60 s p95).

## Chẩn đoán

1. Kích thước hàng đợi, lượt matchmaker

## Xử lý

1. Xem mở rộng rating window; kiểm tra matchmaker

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [MATCHMAKING](../../03-social-multiplayer/MATCHMAKING.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

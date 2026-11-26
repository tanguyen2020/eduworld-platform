# DuplicateMatchResults

> Runbook của alert `DuplicateMatchResults` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/competition.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _More than 20 duplicate match results rejected in 5 minutes (<mode>)_
- Điều kiện (MONITORING §5): `match_results_total{result="duplicate_rejected"}` tăng đột biến > 20/5 phút

## Tác động

Nhiều kết quả trận trùng bị từ chối: lỗi client hoặc gian lận.

## Chẩn đoán

1. Phiên bản client, cờ anti-cheat

## Xử lý

1. Báo đội Godot/anti-cheat

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [MATCHMAKING](../../03-social-multiplayer/MATCHMAKING.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

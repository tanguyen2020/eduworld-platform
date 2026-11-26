# MatchmakerLoopSlow

> Runbook của alert `MatchmakerLoopSlow` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/competition.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Matchmaker pass p95 > 1 s_
- Điều kiện (MONITORING §5): p95 một lượt matchmaker > 1 s trong 10 phút ([MATCHMAKING §10](../../03-social-multiplayer/MATCHMAKING.md))

## Tác động

Một lượt matchmaker > 1 s: ghép trận chậm.

## Chẩn đoán

1. Độ trễ Valkey, số hàng đợi mỗi worker

## Xử lý

1. Scale/khởi động lại `worker-jobs`

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [MATCHMAKING](../../03-social-multiplayer/MATCHMAKING.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

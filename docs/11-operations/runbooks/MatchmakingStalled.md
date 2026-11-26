# MatchmakingStalled

> Runbook của alert `MatchmakingStalled` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/competition.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Queue <queue> has players but no new proposal for 3 minutes_
- Điều kiện (MONITORING §5): `mm_queue_size` > 0 và không có proposal mới trong 3 phút (giờ học)

## Tác động

Có người chờ nhưng không có proposal mới: matchmaker kẹt.

## Chẩn đoán

1. Lock matchmaker (Valkey `ew:lock:*`), log `worker-jobs`

## Xử lý

1. Khởi động lại `worker-jobs`; giải phóng lock kẹt theo VALKEY_DESIGN

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [MATCHMAKING](../../03-social-multiplayer/MATCHMAKING.md)
- [VALKEY_DESIGN](../../07-backend/VALKEY_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

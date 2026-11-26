# CompetitionRoomTickSlow

> Runbook của alert `CompetitionRoomTickSlow` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/competition.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Room tick p99 > 20 ms (<room_type>)_
- Điều kiện (MONITORING §5): p99 tick phòng > 20 ms trong 10 phút ([REALTIME_ARCHITECTURE §8](../06-architecture/REALTIME_ARCHITECTURE.md))

## Tác động

Tick phòng chậm: trận/hub giật.

## Chẩn đoán

1. Số phòng mỗi replica `core-api`, CPU

## Xử lý

1. Scale `core-api`; xem phân bổ phòng

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [REALTIME_ARCHITECTURE](../../06-architecture/REALTIME_ARCHITECTURE.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

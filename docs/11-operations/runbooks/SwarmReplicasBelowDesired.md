# SwarmReplicasBelowDesired

> Runbook của alert `SwarmReplicasBelowDesired` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/swarm.yaml` · Mức: **warning / critical**.

## Triệu chứng

- Thông báo: _<swarm_service> runs fewer replicas than desired for 5 minutes_
- Điều kiện (MONITORING §5): `swarm_service_replicas_running < swarm_service_replicas_desired` > 5 phút

## Tác động

Năng lực giảm; nếu < 50% thì có nguy cơ quá tải.

## Chẩn đoán

1. `docker service ps` — task lỗi (`rejected`: thiếu tài nguyên/placement)

## Xử lý

1. Bổ sung node/tài nguyên; sửa placement constraint

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

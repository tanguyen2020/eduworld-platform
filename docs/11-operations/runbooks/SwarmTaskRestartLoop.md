# SwarmTaskRestartLoop

> Runbook của alert `SwarmTaskRestartLoop` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/swarm.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Tasks of <swarm_service> restarted more than 5 times in 15 minutes_
- Điều kiện (MONITORING §5): `increase(swarm_task_restarts_total[15m]) > 5` cho một Swarm service

## Tác động

Container crash lặp lại.

## Chẩn đoán

1. `docker service ps eduworld_<swarm-service> --no-trunc` và `docker service logs eduworld_<swarm-service> --since 15m` (từ node manager, qua VPN)
2. Exit code (137 = OOM?), healthcheck lỗi

## Xử lý

1. Sau deploy: `make deploy-rollback ENV=<env> SERVICE=<swarm-service>` nếu bắt đầu sau một bản deploy ([DEPLOYMENT §11](../DEPLOYMENT.md))
2. OOM: tăng `limits.memory` theo DEPLOYMENT

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

# NodeCapacityHigh

> Runbook của alert `NodeCapacityHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/swarm.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _CPU of node <node> (role <role>) above 75% for 10 minutes in school hours_
- Điều kiện (MONITORING §5): CPU node vai trò `app`/`realtime`/`idp` > 75% trong 10 phút giờ học

## Tác động

Node `app`/`realtime`/`idp` > 75% CPU giờ học: sắp quá tải (không có autoscale, ADR-0024).

## Chẩn đoán

1. Dashboard Grafana **Binary** và node_exporter theo node

## Xử lý

1. Scale thủ công `docker service scale`; xem lại `scale-schedule.yaml`

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

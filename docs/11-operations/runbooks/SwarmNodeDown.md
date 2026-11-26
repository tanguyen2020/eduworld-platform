# SwarmNodeDown

> Runbook của alert `SwarmNodeDown` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/swarm.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Swarm node <node> (role <role>) is not ready for 2 minutes_
- Điều kiện (MONITORING §5): `swarm_node_ready == 0` > 2 phút

## Tác động

Swarm dời task stateless sang node khác; thành phần có trạng thái ghim trên node đó (PostgreSQL, NATS, Valkey, storage) bị ảnh hưởng. Mất quorum manager: không deploy/scale được.

## Chẩn đoán

1. `docker node ls`, kết nối tới node (hạ tầng)
2. Thành phần ghim trên node: `deploy/swarm/nodes/<env>.yaml`

## Xử lý

1. Khôi phục node; dữ liệu theo runbook RB-xx của BACKUP_RECOVERY nếu cần
2. Mất quorum: khôi phục manager theo quy trình Swarm (`docker swarm init --force-new-cluster` chỉ khi được Technical Lead duyệt)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Gọi Technical Lead hạ tầng ngay.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

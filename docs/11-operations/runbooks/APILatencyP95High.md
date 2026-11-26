# APILatencyP95High

> Runbook của alert `APILatencyP95High` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/api.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _p95 <kind> latency of module <module> above its threshold (<service>)_
- Điều kiện (MONITORING §5): p95 đọc > 400 ms hoặc ghi > 800 ms trong 10 phút

## Tác động

Học sinh thấy chậm; nếu kéo dài sẽ vi phạm SLO latency.

## Chẩn đoán

1. Dashboard Grafana **Module**: p95 theo route, `eduworld_db_pool_acquire_duration_seconds`, query chậm
2. Dashboard Grafana **Data stores**: CPU/kết nối PostgreSQL, Valkey latency
3. Trace chậm trong Tempo (lọc `duration > 400ms`)

## Xử lý

1. Pool DB cạn: tăng replica `core-api` theo `scale-schedule.yaml` hoặc thủ công `docker service scale`
2. Query chậm mới sau deploy: rollback hoặc thêm index theo quy trình migration
3. Canary đang chạy: alert pending làm canary tự hủy (scripts/deploy/canary.sh)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [DATABASE_DESIGN](../../07-backend/DATABASE_DESIGN.md)
- [runbooks/README](README.md)

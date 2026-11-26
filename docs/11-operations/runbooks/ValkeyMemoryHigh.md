# ValkeyMemoryHigh

> Runbook của alert `ValkeyMemoryHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/datastore.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Valkey <instance> uses more than 85% of maxmemory_
- Điều kiện (MONITORING §5): > 85% maxmemory

## Tác động

Valkey gần maxmemory: eviction, mất cache/rate limit.

## Chẩn đoán

1. `redis_memory_used_bytes`, key lớn (theo VALKEY_DESIGN)

## Xử lý

1. Tăng maxmemory/RAM node; xem key không có TTL

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [VALKEY_DESIGN](../../07-backend/VALKEY_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

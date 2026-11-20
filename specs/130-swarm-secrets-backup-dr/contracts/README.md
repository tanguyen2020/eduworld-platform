# Contracts: F130 — Staging/production Docker Swarm, secret, backup PITR và DR drill

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| — | F130 không tạo endpoint. Cổng quản trị `/livez` `/readyz` `/metrics` `/version` không thuộc OpenAPI ([DEPLOYMENT §14](../../../docs/11-operations/DEPLOYMENT.md)) | — | — | Không áp dụng |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Runbook khôi phục gọi lệnh con của binary `worker` (`projection rebuild`, `retention-reapply`, `reconcile --module=<module>` [Confirmed]); các lệnh này gọi use case/facade trong tiến trình do module sở hữu định nghĩa (ADR-0074), F130 không thêm operation facade | — | — | Không áp dụng |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `system.server.draining` | realtime → client khi rolling update ([DEPLOYMENT §11.2](../../../docs/11-operations/DEPLOYMENT.md)) | **Cần thêm** — chưa có trong `specs/asyncapi/eduworld-realtime.v1.yaml`; F090 phát, F130 cấu hình `stop_grace_period` |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không phát. Runbook reset consumer durable trên stream `EVT_*`, `CHAT_MESSAGES` bằng `deliver_policy=by_start_time` ([BACKUP_RECOVERY §3.5](../../../docs/11-operations/BACKUP_RECOVERY.md)) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `deploy/swarm/schema/scale-schedule.schema.json` | Kiểm `scale-schedule.yaml` (cấu hình hạ tầng, không thuộc `specs/`) | Mới (trong feature) |

## 6. Delta cần thực hiện

- **Cần thêm** message AsyncAPI `system.server.draining` (server → client, payload `{ reconnect_after_ms: integer 0–120000, reason: enum [deploy, maintenance] }`, sau đó đóng `1012` — REALTIME_ARCHITECTURE §3.1, RB-401) — task T004; phối hợp F090 (bên phát).
- Không có delta OpenAPI/event. Hợp đồng ngoài: APISIX Admin API `traffic-split` (canary), Docker Engine API (qua socket proxy allowlist).

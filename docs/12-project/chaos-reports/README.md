# Chaos reports

> Trạng thái: Draft · Owner: DevOps · Cập nhật: 2026-11-27 (F132)
> Liên quan: [F132 spec](../../../specs/132-load-chaos-pentest-anticheat/spec.md) · [TEST_STRATEGY §2, §3](../TEST_STRATEGY.md) · [BACKUP_RECOVERY §8](../../11-operations/BACKUP_RECOVERY.md) · [MONITORING §5](../../11-operations/MONITORING.md) · [runbooks](../../11-operations/runbooks/)

Mỗi lần chạy bộ chaos (workflow `chaos.yml`: hằng tuần trên staging và `workflow_dispatch` trước mỗi release — F132-AC-14) sinh một file `<YYYY-MM-DD>.md` trong thư mục này bằng [`tests/chaos/run-chaos.sh`](../../../tests/chaos/run-chaos.sh). File do runner viết, không sửa tay; người chạy commit file cùng bằng chứng release.

**Chưa có lần chạy staging nào** (staging chưa tồn tại — F130 "In progress"). Chưa kịch bản nào được công bố là đạt; bằng chứng hiện có chỉ là kiểm thử cục bộ của runner và của `core-api` (mục "Bằng chứng cục bộ" bên dưới).

## Kịch bản

Định nghĩa ở [`tests/chaos/scenarios/`](../../../tests/chaos/scenarios) (schema: [data-model §1](../../../specs/132-load-chaos-pentest-anticheat/data-model.md)); kiểm schema, tên alert F131 và runbook bằng `node tests/chaos/lib/scenario.mjs validate`.

| Kịch bản | TC | Tiêu chí | Lỗi tiêm (staging) | Alert F131 kỳ vọng |
|---|---|---|---|---|
| `learning-db-down` | TC-SVC-010 | F132-AC-08 | `ALTER DATABASE eduworld_learning ALLOW_CONNECTIONS false` + ngắt phiên, qua ops job trên Patroni leader | `ModuleDatabaseUnavailable`, `ModuleHighErrorRate` |
| `identity-db-down` | TC-SVC-011 | F132-AC-09 | như trên với `eduworld_identity`, 5 phút | `ModuleDatabaseUnavailable` |
| `nats-down` | TC-SVC-012 | F132-AC-10 | `docker service scale` `nats-1..3` = 0, 5 phút | `ChatJetStreamUnavailable`, `OutboxLagHigh` |
| `competition-db-down` | TC-SVC-013 | F132-AC-11 | như `learning-db-down` với `eduworld_competition` | `ModuleDatabaseUnavailable` |
| `pg-switchover` | TC-SVC-013 | F132-AC-11 | drill F130 `scripts/ops/patroni-switchover.sh` (gián đoạn ghi < 60 s, 0 dòng mất) | `PostgresReplicationLag` |
| `valkey-down` | TC-PERF-008 | F132-AC-12 | `valkey-cache` + replica = 0, 5 phút | `ValkeyDown` |
| `node-drain` | — | F132-AC-13 | `docker node update --availability drain <TARGET>` | `SwarmReplicasBelowDesired` |
| `worker-chat-down` | — | F132-AC-25 | `worker-chat` = 0, 5 phút; đối soát tin đã ack | `ServiceDown`, `ChatPersistLagHigh` |
| `core-api-replica-kill` | — | F132-AC-25 | Pumba `kill -s SIGKILL` một task trên node của nó | `SwarmReplicasBelowDesired` |
| `external-fault` | TC-SVC-004 | F132-AC-26 | Toxiproxy tạm (`chaos-toxiproxy`, network `data`) trước Keycloak Admin / SIS / object storage, toxic `timeout` | `CircuitBreakerOpen` |
| `latency` | — | F132-AC-08, AC-10 | Pumba `netem delay` 200 ± 50 ms trên node chạy NATS hoặc `pg-router` | `APILatencyP95High` |

Quy tắc của runner:

- Từ chối `EW_ENV` ngoài `local`/`dev`/`staging` (F132-AC-07); chế độ `compose` (local) không bao giờ chạm container `eduworld-*`.
- Phục hồi **luôn** chạy (kể cả khi bị ngắt hay tiêm lỗi hỏng giữa chừng); không hội tụ trong `recovery_timeout` thì `docker service update --force` các service backend và kịch bản bị đánh dấu **failed**.
- Không có câu trả lời Prometheus không bao giờ là đạt; đối soát thiếu (không có ledger bot hoặc DSN chỉ đọc) là **failed**.
- Alert kỳ vọng được ghi "đã thấy/không thấy" (`ALERTS` pending hoặc firing); alert có `for:` dài hơn cửa sổ lỗi chỉ mang tính thông tin.
- Tải nền 0,3× `target_ccu` do bộ tải F132 chạy song song; bot ghi ledger id attempt/tin chat nhận 2xx/ack vào `EW_CHAOS_LEDGER_DIR`; đối soát bằng [`reconcile/attempts.sql`](../../../tests/chaos/reconcile/attempts.sql) và [`reconcile/chat_messages.sql`](../../../tests/chaos/reconcile/chat_messages.sql) (chỉ đọc, tenant `perf-*`, `deleted_at IS NULL`).

## Mẫu báo cáo (mỗi kịch bản một mục)

```markdown
## <scenario> (<TC>) — passed | failed

| Field | Value |
|---|---|
| Acceptance | F132-AC-NN |
| Load | 0.3x target_ccu (ccu_10k) |
| Fault | `<type>` on `<target>` for <s>s |
| Injected / recovered (UTC) | <t0> / <t1> |
| Recovery | clean | FORCED … | FAILED … |
| Reconciliation | db_submitted=… bot_acked=… missing=0 |

| Phase | Expectation | Value | Condition | Result |
|---|---|---|---|---|

| Expected alert | Seen | Runbook |
|---|---|---|
```

Đầu file ghi môi trường, commit, tag backend (`EW_BACKEND_VERSION`) và người chạy.

## Bằng chứng cục bộ (không thay thế lần chạy staging)

- `node --test tests/chaos/` — runner với docker/curl/psql/sleep giả (TEST DOUBLE): từ chối production, phục hồi luôn chạy, kịch bản không hồi phục bị failed, đối soát mất dữ liệu bị failed, trường báo cáo.
- `TestCoreAPIChaosModuleIsolation` (`src/backend/cmd/core-api/chaos_integration_test.go`, testcontainers): `core-api` thật với PostgreSQL/NATS/Valkey dùng một lần — mất database `eduworld_learning` thì route module khác vẫn 2xx, readiness và `eduworld_db_pool_connections` báo lỗi, tự phục hồi; NATS đứng và Valkey mất thì REST vẫn phục vụ; hai truy vấn đối soát chạy trên schema đã migrate.

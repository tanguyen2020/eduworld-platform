# Implementation Plan: F130 — Staging/production Docker Swarm, secret, backup PITR và DR drill

**Branch**: `130-swarm-secrets-backup-dr` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Viết stack file Docker Swarm cho staging/production theo DEPLOYMENT §4 (node theo vai trò, overlay network mã hóa, thành phần có trạng thái ghim node, Patroni một cụm cho 5 database module, NATS 3 node, Valkey + Sentinel, Keycloak, object storage theo ADR-0075 — cụm Garage/SeaweedFS 3 node `storage` khi tự host, hoặc chỉ cấu hình endpoint khi dùng S3 managed), workflow GitHub Actions deploy theo [ADR-0074](../../docs/adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md)/[ADR-0088](../../docs/adr/ADR-0088-ci-cd-bang-github-actions.md) (job migrate `replicated-job` cho từng database module → `docker stack deploy` `app.stack.yml` với 5 Swarm service `core-api`, `realtime`, `worker-events`, `worker-chat`, `worker-jobs`; canary `core-api` bằng APISIX `traffic-split`; rollback theo Swarm service), đồng bộ secret SOPS → Docker secret có version, `ops-scheduler` scale theo lịch học qua socket proxy allowlist, bộ script backup (pgBackRest, object storage, etcd, realm export) đẩy metric textfile, và script restore test/runbook RB-01…05 có báo cáo. Không có code nghiệp vụ.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | YAML (Compose spec 3.9 cho Swarm), Bash (POSIX + `set -euo pipefail`), Go ≥ 1.24 (`src/backend/tools/ops-scheduler`) |
| Primary Dependencies | Docker Engine Swarm ≥ 25, pgBackRest 2.5x (TLS server), Patroni + etcd riêng, SOPS + age, `aws` CLI (S3 sync tới đích backup), HAProxy (socket proxy), APISIX Admin API (`traffic-split`), `keycloak-config-cli` |
| Storage | PostgreSQL 17 (Patroni, một cụm, 5 database module + `keycloak`), object storage theo `object_storage_backend` (ADR-0075: S3-compatible managed tại Việt Nam hoặc Garage/SeaweedFS ≥ 3 node RF3; floci chỉ local/CI), repo S3 ngoài hạ tầng chính |
| Testing | Bats (`tests/ops/*.bats`) cho script; `go test` cho `ops-scheduler`; diễn tập trên staging (bằng chứng ở `restore-tests/`) |
| Target Platform | VM Linux amd64, Docker Swarm 3 manager |
| Project Type | Hạ tầng + công cụ vận hành |
| Performance Goals | Rollback ≤ 15 phút; RPO/RTO theo `rpo_rto`; restore test trong ngưỡng BACKUP_RECOVERY §8 |
| Constraints | ADR-0024, ADR-0074, ADR-0088, ADR-0075, ADR-0039, AGENTS R3, R4, R12, R14 |
| Scale/Scope | Topology pilot (staging) và Release 1.0 (production) — DEPLOYMENT §4.1, §4.2 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Không có endpoint mới; delta AsyncAPI `system.server.draining` ở task Phase 2 (T004); không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Mỗi Swarm service chỉ nhận credential của module nó nạp, mỗi module một database/role; `ops-scheduler` không import `src/backend/modules/` |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Credential DB theo module (không `CONNECT` chéo database), network `data` tách; khôi phục không xóa vật lý, chạy `retention-reapply` |
| IV Server-authoritative | ✅ | Không áp dụng |
| V Xác thực, phân quyền | ✅ | Secret chỉ qua Docker secrets; Docker API chỉ TLS client cert của self-hosted runner deploy + socket proxy allowlist; deploy production cần approval (GitHub Environment `production`) |
| VI An toàn trẻ em | ✅ | Staging chỉ dữ liệu giả; truy cập backup có ghi nhận; log không secret/PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Lịch scale đọc `warm_up_windows`; số liệu sizing theo DEPLOYMENT, không định nghĩa lại |
| VIII UI tách logic, token | ✅ | Không áp dụng |
| IX Test có bằng chứng | ✅ | Bats/go test cho script và scheduler; diễn tập ghi báo cáo |

## Project Structure

### Documentation (feature này)

```text
specs/130-swarm-secrets-backup-dr/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
deploy/swarm/
├── networks.sh                         # tạo overlay edge/app/data/admin (encrypted)
├── nodes/<env>.yaml                    # inventory nhãn node: role, zone, storage, hostname
├── schema/scale-schedule.schema.json   # schema nội bộ (maxLength cho chuỗi)
└── {staging,production}/
    ├── infra.stack.yml  data.stack.yml  ops.stack.yml
    ├── app.stack.yml                   # core-api, realtime, worker-events, worker-chat, worker-jobs
    ├── migrate.job.tpl.yml             # one-off replicated-job `migrate --module=<module>`
    ├── <swarm-service>.env  (không secret)   scale-schedule.yaml   ops-jobs.crontab
deploy/secrets/{staging,production}/<owner>.sops.yaml   # SOPS (age)
deploy/backup/{pgbackrest.conf,pgbackrest-server.conf,Dockerfile}  # image eduworld/backup-tools
deploy/apisix/canary/traffic-split.tpl.yaml
deploy/ops-scheduler/socket-proxy.cfg   # HAProxy allowlist
src/backend/tools/ops-scheduler/{cmd/ops-scheduler/main.go,internal/{schedule,capacity,dockerapi,metrics}/}   # thuộc Go module src/backend (go.mod duy nhất, ADR-0074, ADR-0092)
scripts/deploy/{validate-stacks.sh,check-secret-refs.sh,deploy-service.sh,run-migrate-job.sh,canary.sh,rollback.sh,secrets-sync.sh}
scripts/backup/{pg_backup.sh,pg_logical_dump.sh,object_storage_backup.sh,etcd_snapshot.sh,keycloak_export.sh,restore_test.sh,lib-metrics.sh}
scripts/ops/{rb01-pitr-restore.sh,rb03-cluster-restore.sh,rb04-keycloak-restore.sh,rb05-object-storage-restore.sh,reset-consumers.sh,patroni-switchover.sh}
.github/workflows/deploy-config.yml     # validate stack/secret refs/scale-schedule (PR theo path deploy/**)
.github/workflows/release.yml           # migrate → deploy dev/staging/production (environment), canary, rollback (workflow_dispatch)
.github/workflows/secrets-sync.yml      # workflow_dispatch, environment staging/production, self-hosted runner `deploy`
tests/ops/*.bats
docs/11-operations/restore-tests/README.md
```

**Structure Decision**: Theo DEPLOYMENT §4.5 (`deploy/swarm/<env>/`) và BACKUP_RECOVERY §10 (`scripts/backup/`). `ops-scheduler` là công cụ trong `src/backend/tools/` của Go module `src/backend` (`src/backend/go.mod` duy nhất, ADR-0074, ADR-0092), không phải module nghiệp vụ, không DB. Stack `ops.stack.yml` chứa `ops-scheduler`, socket proxy, `backup-tools` (supercronic với `ops-jobs.crontab`, không quyền Docker API), bastion; observability do F131 thêm vào `infra.stack.yml`.

## Phase 0: Research

→ [research.md](research.md): R1 nguồn secret (SOPS), R2 pgBackRest repo host + TLS server, R3 cơ chế lịch của `ops-scheduler` (scale + job backup/restore test), R4 canary bằng APISIX `traffic-split`.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md) (không có bảng nghiệp vụ)
- **Contracts** → [contracts/README.md](contracts/README.md) (không có delta contract)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `src/backend/tools/ops-scheduler/internal/schedule` | Kiểu `Schedule{Service, Baseline, Peak, Windows}`; quy tắc `Peak ≥ Baseline`, khung giờ không chồng |
| application | `capacity.Check(reservations, nodeFree)` · `Planner.Next(now)` | Từ chối scale khi vượt năng lực (F130-AC-23) |
| infrastructure | `dockerapi` (HTTP tới socket proxy), `metrics` (Prometheus `/metrics` trên `:9090`) | Không truy cập Docker socket trực tiếp |
| presentation | `src/backend/tools/ops-scheduler/cmd/ops-scheduler` (vòng lặp mỗi 30 s; lệnh con `validate <file>`) | Job backup/restore chạy trong service `backup-tools` (research R3), không qua `ops-scheduler` |

### Tích hợp và phụ thuộc

- Phụ thuộc: F001 (image, Dockerfile, `/livez` `/readyz`, init role DB), F004 (realm, route APISIX), F003 (outbox relay, consumer durable, lệnh `projection rebuild`), F016 (`retention-reapply`).
- Tiêu thụ: F131 (metric backup/scale, observability trong `infra.stack.yml`), F132 (staging đầy đủ), F133 (bucket `ew-game-bundles`, CDN).
- Deploy job (self-hosted runner nhãn `deploy`, environment `staging`/`production`): `DOCKER_HOST=tcp://swarm-manager:2376` + client cert CI; job migrate từng module rồi `docker stack deploy --with-registry-auth -c deploy/swarm/<env>/app.stack.yml eduworld`.

### Bảo mật, tenant, idempotency, audit

- Secret: SOPS → `docker secret create <name>_v<N>`; `check-secret-refs.sh` kiểm tiền tố theo module mà Swarm service nạp (DEPLOYMENT §3.1, §13); `kc_game_backend` chỉ `core-api`, `chat_purger_dsn` chỉ `worker-chat`, `realtime` không có DB credential.
- Network: `data` không gắn APISIX/realtime; cụm object storage tự host (nếu chọn) chỉ trên `data`, không publish cổng ra Internet (ADR-0075, DEPLOYMENT §9b).
- Deploy idempotent: chạy lại `deploy-service.sh` cùng tag không đổi spec (Swarm so sánh spec).
- Ghi nhận: mỗi deploy/rollback/restore ghi một dòng JSON (`actor`, `env`, `service` = Swarm service, `tag`, `action`, `reason`) vào log workflow và artifact; restore ghi báo cáo `restore-tests/`.

### Quan sát

- Metric: `eduworld_ops_scale_actions_total{service,action,result}` (`ops-scheduler` `/metrics`), `eduworld_backup_*{target,type}` (job `backup-tools` ghi textfile vào volume của node_exporter trên node `ops`). Deploy/rollback quan sát qua metric Swarm của F131 (`swarm_service_replicas_*`, `swarm_task_restarts_total`). Alert và dashboard: F131.

### Rủi ro ADR

- ADR-0074 Accepted (2026-11-17): đã áp — `app.stack.yml` 5 Swarm service, 3 image + `migrate`, một cụm PostgreSQL, secret theo module/binary.
- ADR-0075 Accepted (thay ADR-0071): đã áp — F130-AC-14, F130-AC-20, `object_storage_backup.sh`, RB-05 viết cho cả hai backend; phần phụ thuộc lựa chọn (cụm `storage` trong `data.stack.yml`, cách bật replication) chốt sau spike trước pilot (`object_storage_backend`). ADR-0079 không ảnh hưởng F130.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

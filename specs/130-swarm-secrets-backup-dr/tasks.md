# Tasks: F130 — Staging/production Docker Swarm, secret, backup PITR và DR drill

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Inventory nhãn node (`role`, `zone`, `storage`, `hostname`) cho staging/production và local-swarm; script gắn nhãn `docker node update --label-add` — file: `deploy/swarm/nodes/production.yaml` — 2026-11-27: `deploy/swarm/nodes/{staging,production}.yaml` + `scripts/deploy/label-nodes.mjs` (local-swarm bỏ: một node không mang được nhiều `role`; kiểm nhanh bằng `make swarm-validate`, không `swarm init`)
- [x] T002 [P] Script tạo overlay `edge`, `app`, `data`, `admin` (`--opt encrypted` cho app/data/admin), idempotent — file: `deploy/swarm/networks.sh` — `networks.sh` (test `deploy_scripts.test.mjs`)
- [x] T003 [P] Target Makefile `swarm-validate`, `swarm-up`, `deploy-service`, `deploy-rollback`, `secrets-sync`, `restore-test` — file: `Makefile` — thêm `swarm-up`, `ops-job`, `test-ops`, `test-ops-integration`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: thêm message `system.server.draining` (server → client, `reconnect_after_ms` 0–120000, `reason` enum `deploy|maintenance`; đóng `1012` sau đó) vào AsyncAPI, phối hợp F090 (F090 T005) — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint` — đã có từ F090 (`system.server.draining` trong `eduworld-realtime.v1.yaml`), không đổi
- [x] T005 Schema `scale-schedule` (chuỗi có `maxLength`, `peak ≥ baseline`) — file: `deploy/swarm/schema/scale-schedule.schema.json`
- [x] T006 `validate-stacks.sh`: `docker stack config` từng file, cấm `latest`, bắt buộc `placement`/`resources`/`update_config`/`rollback_config`/`healthcheck`, network theo DEPLOYMENT §4.3, datastore không `ports` — file: `scripts/deploy/validate-stacks.sh` — hiện thực bằng `scripts/deploy/validate-stacks.sh` + `check-stacks.mjs`/`check-secret-refs.mjs`/`check-scale-schedule.mjs` theo `deploy/swarm/policy.yaml`
- [x] T007 Image `eduworld/backup-tools` (pgBackRest, aws CLI, etcdctl, supercronic, jq; non-root) — file: `deploy/backup/Dockerfile` — kèm `deploy/postgres/patroni.Dockerfile` (PostgreSQL 17 + Patroni + pgBackRest + PgBouncer); supercronic và nats CLI build từ module Go ghim version

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Deploy, rollback và scale từng Swarm service độc lập trên Swarm (Priority: P1) 🎯 MVP

**Goal**: Stack file + workflow deploy/migrate/canary/rollback theo Swarm service (ADR-0074, ADR-0088).

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T008 [P] [US1] Bats test F130-AC-01: fixture stack vi phạm (`latest`, thiếu `placement`, datastore có `ports`) làm `validate-stacks.sh` fail — file: `tests/ops/validate_stacks.bats` — phải FAIL trước khi code — test bằng `node:test` thay Bats (repo chuẩn hóa `node --test`; Bats không có trên máy/CI): `tests/ops/swarm_policy.test.mjs`
- [x] T009 [P] [US1] Bats test F130-AC-02: `run-migrate-job.sh` với job giả exit 1 → không gọi `stack deploy` (stub `docker`) — file: `tests/ops/migrate_job.bats` — `tests/ops/deploy_scripts.test.mjs` (docker giả lập `tests/ops/docker-stub.sh`)
- [ ] T010 [P] [US1] Kịch bản staging F130-AC-03 (TC-SVC-015): deploy image `core-api` có `/livez` lỗi, đo restart `realtime`/`worker-*` = 0, rollback ≤ 15 phút — file: `tests/ops/staging/rollback_isolation.sh` — viết xong; **chờ môi trường staging**
- [x] T011 [P] [US1] Bats test F130-AC-04, F130-AC-05: `canary.sh` abort `core-api-canary` khi truy vấn Prometheus stub vượt ngưỡng; `deploy-service.sh production` từ chối tag `backend/vX.Y.Z` có digest chưa có ở staging — file: `tests/ops/canary.bats` — `deploy_scripts.test.mjs` (canary hủy/promote/từ chối khi thiếu rule alert; production từ chối digest lệch, ngoài `maintenance_window`)
- [ ] T012 [P] [US1] Kịch bản staging F130-AC-06, F130-AC-07 (TC-SVC-007): kiểm healthcheck/`stop_grace_period`, thử kết nối datastore từ task chỉ gắn `edge` — file: `tests/ops/staging/network_isolation.sh` — viết xong; **chờ môi trường staging**

### Implementation cho User Story 1

- [x] T013 [US1] `infra.stack.yml` staging/production: APISIX + etcd, Keycloak (Infinispan JDBC_PING), NATS 3 node + exporter, Valkey `cache`/`realtime`/`ratelimit` + Sentinel — file: `deploy/swarm/production/infra.stack.yml` — production + staging (pilot); tách `valkey-realtime` sau [Proposed], xem DEPLOYMENT §8
- [x] T014 [US1] `data.stack.yml`: Patroni một cụm (5 database module + `keycloak`, ADR-0074 mục 4) + etcd Patroni + PgBouncer, `pgbackrest server` cạnh Patroni, object storage theo ADR-0093: SeaweedFS (master ×3, volume, filer, S3 gateway) trên 3 node `storage`, replication 3 bản, chỉ network `data`, image ghim digest; secret `s3_<binary>` + `EW_S3_ENDPOINT` (khi chuyển S3 managed theo RB-05b chỉ đổi secret và gỡ service `storage`) — file: `deploy/swarm/production/data.stack.yml` — SeaweedFS 3 node (staging 1), `pg-router` theo REST Patroni, `pgbackrest-N` TLS server
- [x] T015 [P] [US1] `app.stack.yml` với 5 Swarm service `core-api`, `realtime`, `worker-events`, `worker-chat`, `worker-jobs` (`src/backend/cmd/worker --profile=events|chat|jobs`), image `ghcr.io/<org>/eduworld/<image>:${BACKEND_VERSION}`, `EW_ADMIN_ADDR=:9090`, `stop_grace_period` 40 s/150 s, healthcheck `/livez`; template job `migrate --module=<module>`; `<swarm-service>.env` không secret — file: `deploy/swarm/production/app.stack.yml` — kèm `ops.stack.yml`, `<swarm-service>.env`, `stack.env`; job migrate dựng bằng `run-migrate-job.sh` (không cần file template)
- [x] T016 [P] [US1] Bản staging (topology pilot) dùng cùng cấu trúc, khác replica/sizing — file: `deploy/swarm/staging/app.stack.yml`
- [x] T017 [US1] `run-migrate-job.sh` (`docker service create --mode replicated-job --restart-condition none --network data --secret <module>_migrator_dsn ghcr.io/<org>/eduworld/migrate:<tag> --module=<module>`, lần lượt 5 module, chờ mọi task Complete, timeout theo DEPLOYMENT §12) — file: `scripts/deploy/run-migrate-job.sh`
- [x] T018 [US1] `deploy-service.sh` (kiểm quorum manager, so digest staging, `docker stack deploy --with-registry-auth`) và `rollback.sh` (`docker service rollback`) — file: `scripts/deploy/deploy-service.sh` — kèm `--service` (một service), `--migrate-only`/`--skip-migrate`/`--redeploy`, kiểm `maintenance_window`
- [x] T019 [US1] `canary.sh` (`core-api-canary`) + template `traffic-split` APISIX, theo dõi 30 phút qua Prometheus theo từng module — file: `scripts/deploy/canary.sh` — ngưỡng không định nghĩa lại: hủy khi Prometheus có alert `ModuleHighErrorRate`/`APILatencyP95High` pending/firing cho `core-api-canary`; thiếu rule → từ chối (rule do F131)
- [x] T020 [US1] Workflow `deploy-config.yml` (job `validate`: `validate-stacks.sh`, `check-secret-refs.sh`, schema `scale-schedule`) và job deploy trong `release.yml` (`deploy-dev`, `deploy-staging` environment `staging`, `deploy-production` environment `production` có required reviewers + kiểm khung giờ, `rollback` qua `workflow_dispatch`), chạy trên self-hosted runner nhãn `deploy`, action ghim SHA — file: `.github/workflows/release.yml` — `deploy-config.yml` job `swarm` + `backup-restore`; `release.yml` `build-ops-image`, `deploy-staging`, `deploy-production`, `rollback`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Secret chỉ qua Docker secrets, mỗi Swarm service chỉ thấy secret của module nó nạp (Priority: P1)

**Goal**: SOPS → Docker secret có version, kiểm owner, xoay vòng.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T021 [P] [US2] Bats test F130-AC-08, F130-AC-09: stack/env chứa giá trị giống secret hoặc mount secret sai owner (`kc_game_backend` vào `worker-events`, `chat_purger_dsn` vào `core-api`, DB credential vào `realtime`) → fail — file: `tests/ops/secret_refs.bats` — phải FAIL trước khi code — `tests/ops/swarm_policy.test.mjs`
- [x] T022 [P] [US2] Bats test F130-AC-10: `secrets-sync.sh` tạo `_v<N+1>` khi hash đổi, không tạo khi không đổi, không xóa `_v<N>` khi còn tham chiếu — file: `tests/ops/secrets_sync.bats` — `tests/ops/deploy_scripts.test.mjs`
- [x] T023 [P] [US2] Go test F130-AC-11 (TC-AUTH-060): loader `*_FILE` của `src/backend/pkg/config` thoát lỗi khi file thiếu/rỗng, log không chứa giá trị — file: `src/backend/pkg/config/secretfile_test.go` — kèm sửa lỗi: file secret rỗng trước đây được chấp nhận — `config.ReadSecretFile` dùng chung cho `pkg/config`, `pkg/module`, `cmd/migrate`; thêm `pkg/module/resources_test.go`

### Implementation cho User Story 2

- [x] T024 [US2] File SOPS mẫu (khóa, không giá trị thật) + `.sops.yaml` creation rules theo môi trường — file: `deploy/secrets/.sops.yaml` — `.sops.yaml` (người nhận age là placeholder tới khi có khóa thật) + `deploy/secrets/secrets.template.yaml` + `scripts/deploy/list-secrets.mjs`; `.gitignore` chỉ cho commit `*.sops.yaml`
- [x] T025 [US2] `secrets-sync.sh` (giải mã, hash, `docker secret create <owner>_<name>_v<N>`, cập nhật biến version trong stack) và `check-secret-refs.sh` — file: `scripts/deploy/secrets-sync.sh` — `check-secret-refs.mjs` thay `check-secret-refs.sh`
- [x] T026 [US2] Workflow `secrets-sync.yml` (`workflow_dispatch`, environment `staging`/`production` có required reviewers, self-hosted runner nhãn `deploy`) — file: `.github/workflows/secrets-sync.yml`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Backup PITR cụm PostgreSQL (từng database module) và object storage (Priority: P1)

**Goal**: Backup liên tục có metric.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [x] T027 [P] [US3] Bats test F130-AC-13: `pg_backup.sh` với pgbackrest stub thành công/thất bại ghi đúng file metric textfile — file: `tests/ops/backup_metrics.bats` — phải FAIL trước khi code — `tests/ops/backup_scripts.test.mjs` + chạy thật trong `tests/ops/integration/run.sh`
- [ ] T028 [P] [US3] Kịch bản staging F130-AC-12, F130-AC-15: `pgbackrest info` có full/diff/WAL, repo mã hóa; credential ghi không đọc được repo — file: `tests/ops/staging/backup_repo.sh` — viết xong (`tests/ops/staging/backup_repo.sh`); tách quyền ghi/đọc S3 (AC-15) **chờ backend thật** — floci không có IAM
- [x] T029 [P] [US3] Bats test F130-AC-14: `object_storage_backup.sh` bỏ `ew-media-quarantine`, phát hiện checksum lệch; kiểm versioning bật và metric độ trễ replication bucket dữ liệu học sinh ≤ RPO `rpo_rto` — file: `tests/ops/object_storage_backup.bats` — `backup_scripts.test.mjs` + integration (versioning, quarantine, checksum, độ trễ replication với replicator stand-in)

### Implementation cho User Story 3

- [x] T030 [US3] Cấu hình pgBackRest repo host + TLS server (một stanza cho cụm, cipher, retention theo BACKUP_RECOVERY §3.2, `backup-standby`) — file: `deploy/backup/pgbackrest.conf` — kèm `pgbackrest-db.conf`, `pgbackrest-server.conf`, `pgbackrest-staging.conf`, mẫu chính sách S3 `deploy/backup/s3-policies/`
- [x] T031 [US3] `lib-metrics.sh`, `pg_backup.sh`, `pg_logical_dump.sh` (từng database module) — file: `scripts/backup/pg_backup.sh` — kèm `pg_check.sh`; thư viện là `lib-backup.sh`
- [x] T032 [P] [US3] `object_storage_backup.sh` (rclone/`aws s3 sync` tới bản sao ngoài cụm, chỉ API S3 chuẩn), cấu hình versioning + replication bucket dữ liệu học sinh theo backend, `etcd_snapshot.sh`, `keycloak_export.sh` (không secret) — file: `scripts/backup/object_storage_backup.sh` — kèm `object_storage_replication_check.sh`, `object_storage_setup.sh`
- [x] T033 [US3] `ops.stack.yml` với `backup-tools` + `ops-jobs.crontab`, volume textfile node_exporter — file: `deploy/swarm/production/ops.stack.yml`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Diễn tập khôi phục, failover và DR drill (Priority: P1)

**Goal**: Restore test tự động và runbook RB-01…05 có báo cáo.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [x] T034 [P] [US4] Bats test F130-AC-16: `restore_test.sh --dry-run` sinh báo cáo đủ trường bắt buộc (data-model §1) — file: `tests/ops/restore_report.bats` — phải FAIL trước khi code — `backup_scripts.test.mjs`
- [ ] T035 [P] [US4] Kịch bản DR F130-AC-17 (TC-SVC-014): khôi phục `eduworld_learning` về T trên staging, reset consumer, rebuild projection, `worker reconcile --module=learning`; module khác vẫn phục vụ — file: `tests/ops/staging/restore_one_module.sh` — viết xong; phần database (khôi phục vào database mới + swap, dừng đúng T) chạy thật trong integration; **chờ staging** và lệnh `reconcile` (F134)
- [ ] T036 [P] [US4] Kịch bản F130-AC-18 (TC-SVC-013): switchover Patroni (cụm duy nhất), đo gián đoạn ghi, kiểm `core-api`/`worker-*` kết nối lại không restart task — file: `tests/ops/staging/patroni_switchover.sh` — viết xong; **chờ staging** (cần cụm Patroni thật)
- [ ] T037 [P] [US4] Kịch bản F130-AC-19, F130-AC-20, F130-AC-21: RB-04 (token cũ bị từ chối), RB-05 (checksum 100%), DR drill (đạt `rpo_rto`) — file: `tests/ops/staging/rb04_rb05_dr.sh` — viết xong; RB-05 khôi phục + kiểm 100 % chạy thật trong integration (stand-in); **chờ staging/production**

### Implementation cho User Story 4

- [x] T038 [US4] `restore_test.sh` (`--mode=full|module`, cụm tạm cô lập, đối soát attempt, ghi báo cáo) — file: `scripts/backup/restore_test.sh`
- [x] T039 [US4] `rb01-pitr-restore.sh`, `rb03-cluster-restore.sh` (thứ tự khởi động: data → `worker-events` → `core-api` → `worker-chat`, `worker-jobs` → `realtime`), `reset-consumers.sh` (`deliver_policy=by_start_time`) — file: `scripts/ops/rb01-pitr-restore.sh` — kèm `run-ops-job.sh`; thứ tự khởi động RB-03 theo BACKUP_RECOVERY (`core-api` → `worker-*` → `realtime`, tài liệu sở hữu); binary giữ vị trí consumer đã reset (`pkg/outbox`, integration test NATS)
- [x] T040 [P] [US4] `rb04-keycloak-restore.sh` (keycloak-config-cli, rotate keys, `not-before`), `rb05-object-storage-restore.sh` (mất 1 node `storage` khi tự host; dựng bucket/cụm mới và khôi phục khi mất toàn bộ), `patroni-switchover.sh` — file: `scripts/ops/rb04-keycloak-restore.sh`
- [x] T041 [P] [US4] Mẫu báo cáo khôi phục — file: `docs/11-operations/restore-tests/README.md`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Scale theo lịch học (Priority: P2)

**Goal**: `ops-scheduler` scale theo `warm_up_windows`.

**Independent Test**: Như spec.md User Story 5.

### Tests cho User Story 5

- [x] T042 [P] [US5] Go test F130-AC-22: `Planner.Next` scale lên `peak` trước khung `lead_minutes`, về `baseline` sau khung; metric tăng — file: `src/backend/tools/ops-scheduler/internal/schedule/planner_test.go` — phải FAIL trước khi code
- [x] T043 [P] [US5] Go test F130-AC-23: vượt năng lực → `rejected`; lỗi Docker API → `error`; proxy chặn `POST /services/create` — file: `src/backend/tools/ops-scheduler/internal/capacity/check_test.go` — kèm test scaler (Engine API giả) và proxy HAProxy thật trong integration (`POST /services/create` → 403)
- [x] T044 [P] [US5] Go test F130-AC-24: `validate` từ chối `peak < baseline`, service lạ, chuỗi quá dài — file: `src/backend/tools/ops-scheduler/cmd/ops-scheduler/validate_test.go`
- [ ] T045 [P] [US5] Kịch bản staging TC-PERF-017 cho F130-AC-22 — file: `tests/ops/staging/warmup.sh` — viết xong; **chờ staging**

### Implementation cho User Story 5

- [x] T046 [US5] Domain `schedule` + `capacity` + client `dockerapi` + metric `eduworld_ops_scale_actions_total` — file: `src/backend/tools/ops-scheduler/internal/schedule/planner.go` — kèm `dockerapi`, `scaler`, `metrics`
- [x] T047 [US5] `src/backend/tools/ops-scheduler/cmd/ops-scheduler` (vòng lặp 30 s, lệnh `validate`, `/metrics` `:9090`) + Dockerfile non-root — file: `src/backend/tools/ops-scheduler/cmd/ops-scheduler/main.go` — Dockerfile `deploy/docker/ops-scheduler.Dockerfile` (distroless non-root)
- [x] T048 [US5] HAProxy socket proxy allowlist + `scale-schedule.yaml` staging/production — file: `deploy/ops-scheduler/socket-proxy.cfg`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [x] T049 [P] Cập nhật DEPLOYMENT §4.5/§13/§14 theo thực tế (SOPS, `backup-tools`, `ops-jobs.crontab`, cổng `:9090` = `EW_ADMIN_ADDR`), BACKUP_RECOVERY §10 (TLS server, một stanza cho cụm) và §3.5, §7, §8 (`reconcile-<service>` → `reconcile-<module>`, khôi phục theo database module) — file: `docs/11-operations/DEPLOYMENT.md` — DEPLOYMENT §4.4, §4.5, §8, §10.2, §11.3, §12, §13, §14, §15; BACKUP_RECOVERY §3, §6, §8, §9, §10; LOCAL_DEVELOPMENT §7
- [x] T050 [P] CHANGELOG mục F130 — file: `CHANGELOG.md`
- [ ] T051 Chạy [quickstart.md](quickstart.md) trên staging và ghi bằng chứng — file: `specs/130-swarm-secrets-backup-dr/quickstart.md` — **chờ môi trường staging** (bằng chứng local: `make test-ops`, `make test-ops-integration`)

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). T052, T053 làm 2026-11-27 (phần cần môi trường staging thật ghi ở từng task).

- [x] T052 **OQ chốt 2026-11-20** (DEPLOYMENT §5) Đầu ra ADC sinh từ `deploy/apisix/routes/routes.yaml` + `routes/env/<env>.env` và CI job `apisix:sync` (dry-run diff ở PR, áp ở deploy, admin key qua secret chỉ job này dùng) — file: `deploy/apisix/scripts/`, `.github/workflows/` — cỡ M — **Bằng chứng (2026-11-27):** `render.mjs --format=adc` (render khi chạy, không commit giá trị môi trường), `deploy/apisix/adc.env` (ADC 0.31.0 ghim digest), `scripts/deploy/apisix-sync.sh` (lint → diff → sync từng gateway, từ chối khi canary `traffic-split` đang chạy), job `apisix-sync-diff` (PR, viewer key, Environment `staging-readonly`) trong `deploy-config.yml`, bước `apisix:sync` trong `release.yml` (staging sau rollout, production trước canary), workflow tay `apisix-sync.yml`, rule `check-workflows.mjs` `apisix-admin-key`, Makefile `apisix-adc`/`apisix-adc-validate`/`apisix-sync`. Sửa kèm: etcd prefix riêng `apisix-internal` (`/apisix-internal`, trước đó hai gateway đọc chung route), viewer key `apisix_viewer_key`, plugin `ew-*` mount vào gateway Swarm, healthcheck APISIX không dùng `curl` (image không có), canary chọn route theo nhãn `ew-upstream` và dừng khi không có route, `ew-api-rate` dùng user Valkey `apisix`. Kiểm: `node --test deploy/apisix/scripts/` 45/45 (gồm `render-adc.test.mjs` 10 test tương đương standalone ↔ ADC), `node --test tests/ops/apisix_sync.test.mjs` 4/4, `node --test scripts/ci/check-workflows.test.mjs` 13/13, `check-workflows` 12 workflow 0 lỗi, `validate-adc.sh` 19/19 (etcd auth + 2 APISIX 3.13 traditional + `adc` 0.31.0 thật: lint, diff viewer key, viewer không sync được, sync, tập route tách theo gateway, data plane, sync lặp lại, canary split chặn sync), `render.mjs --check`, `check-routes` 0, shellcheck 0. **Còn chờ staging:** sync vào etcd APISIX staging thật (biến `APISIX_ROUTES_ENV`, `APISIX_ADMIN_URL`, `APISIX_INTERNAL_ADMIN_URL`, secret `APISIX_ADMIN_KEY`/`APISIX_VIEWER_KEY`, Environment `staging-readonly`, `adc` trên runner `deploy`)
- [x] T053 **OQ chốt 2026-11-20** (OQ F090-2) ACL Valkey (user + key pattern `ew:rt:*`, `ew:game:rt:*` theo binary) và NATS (user + subject theo binary/profile, realtime chỉ publish `ew.cmd.*`/`ew.in.room.*`) trong `deploy/`, secret qua Swarm secret — file: `deploy/` — **Bằng chứng (2026-11-27):** mẫu `deploy/valkey/users.acl.template` (user `core-api`/`worker`/`realtime`/`apisix`/`replicator`/`sentinel`/`exporter`, `default` tắt, mật khẩu `{{sha256:<khóa SOPS>}}`), `deploy/nats/permissions.yaml` + `deploy/nats/accounts.template.conf` (operator/JWT, resolver MEMORY; realtime publish `ew.cmd.>`, `ew.in.room.>` và — theo mã — `ew.rt.presence.*`, `ew.evt.realtime.>`); `scripts/deploy/render-acl.mjs`; `secrets-sync.sh` render `valkey_acl` (dẫn xuất, không ở SOPS) và từ chối creds NATS lệch quyền; `scripts/deploy/nats-accounts.sh` (offline, nsc); `check-acl-templates.mjs` (86 literal Valkey, 61 literal NATS, 15 stream, 0 vấn đề; grant thừa/thiếu, wildcard, lệnh nguy hiểm, kích thước JWT); `nats.conf` `max_control_line: 16KB` (JWT worker ~6 KB vượt 4 KB — phát hiện bởi probe thật); `secrets.template.yaml` (+`apisix_viewer_key`, +`valkey_exporter_password`, −`valkey_acl`). Kiểm: `node --test tests/ops/` 63/63 trên Linux có jq (container `node:20-bookworm-slim`; trên Windows 28 pass/35 skip vì thiếu jq), gồm `acl_templates.test.mjs` 12/12 và secrets-sync dẫn xuất/drift; `validate-acl.sh` 100/100 (`valkey:8` AUTH + `ACL DRYRUN` cho phép/từ chối mỗi user; `nats-accounts.sh --init` + chạy lại; `nats-server -t` với `nats.conf` Swarm; probe publish/subscribe/JetStream theo user), `swarm-validate` (stack config + policy + secret refs) ok, shellcheck 0. **Còn chờ staging:** tạo kho nsc offline + creds thật, `secrets-sync` lên swarm staging, binary kết nối bằng ACL/creds thật (`EW_VALKEY_USERNAME`); redis_exporter của F131 dùng user `exporter` + secret `valkey_exporter_password` (service `valkey-exporter*`)

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 (T025 cần stack của T015) → US3 → US4 (cần US3) → US5 → Polish.
- T013, T014 trước T015; T030 trước T031; T046 trước T047.
- F131 dựng observability vào `infra.stack.yml` sau T013; F132 cần US1–US5 trên staging.

## Definition of Done

- [ ] Mọi tiêu chí F130-AC-* có test pass (ghi mã test/TC) — phần logic/cấu hình có test local; phần cần staging/production thật còn chờ (CODE_PLAN)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

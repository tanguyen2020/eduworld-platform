# Feature Specification: F130 — Staging/production Docker Swarm, secret, backup PITR và DR drill

**Feature Branch**: `130-swarm-secrets-backup-dr` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: hạ tầng (`deploy/swarm`, `deploy/secrets`, `deploy/backup`, `scripts/deploy`, `scripts/backup`, `tools/ops-scheduler`, workflow deploy GitHub Actions) · **Mốc**: M6 · **Phụ thuộc**: F001
**Input**: Dựng staging/production trên Docker Swarm (ADR-0024), phân phối secret qua Docker secrets, deploy/rollback/scale độc lập từng Swarm service backend theo [ADR-0074](../../docs/adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) (`core-api`, `realtime`, `worker-events`, `worker-chat`, `worker-jobs`), backup PITR cụm PostgreSQL (khôi phục được từng database module) + object storage, diễn tập khôi phục và DR — FR-OPS-002, FR-OPS-004, NFR-DEPL-001.

## Clarifications

### Session 2026-11-16

- Q: Nguồn gốc secret khi OQ-10 chưa chốt (Vault/OpenBao, KMS cloud hay SOPS)? → A: SOPS (age) mã hóa trong Git `deploy/secrets/<env>/`, workflow deploy giải mã và tạo Docker secret có hậu tố version; khóa age ở secret manager/secret của GitHub Environment; đổi sang Vault chỉ thay script đồng bộ ([DEPLOYMENT §13](../../docs/11-operations/DEPLOYMENT.md), [SECURITY_ARCHITECTURE §6](../../docs/10-security/SECURITY_ARCHITECTURE.md)) [Proposed, OQ-10].
- Q: Topology staging? → A: Topology pilot [DEPLOYMENT §4.1](../../docs/11-operations/DEPLOYMENT.md) với cùng stack file production (khác số replica/sizing trong `<swarm-service>.env`/override); production theo §4.2 [Proposed].
- Q: Cổng quản trị trong Swarm? → A: `EW_ADMIN_ADDR=:9090` trong container (mỗi task một network namespace), không publish, không route APISIX — khớp F001 (một cổng quản trị) và [DEPLOYMENT §14](../../docs/11-operations/DEPLOYMENT.md).
- Q: Công cụ backup PostgreSQL? → A: pgBackRest, repo host trên node `ops` kết nối node `data-pg` qua pgBackRest TLS server ([BACKUP_RECOVERY §3.1](../../docs/11-operations/BACKUP_RECOVERY.md)); chi tiết ở [research.md](research.md) [Proposed].
- Q: Lệnh `reconcile-<module>`, `projection rebuild`, `retention-reapply`, `saga resume` do ai làm? → A: Module sở hữu cung cấp dưới dạng lệnh con của binary `worker` (vd. `worker reconcile --module=<module>` [Proposed], ADR-0074); F130 điều phối trong runbook và kiểm chứng. Lệnh chưa có feature sở hữu → ghi ở Assumptions.

### Session 2026-11-17

- Q: Đơn vị triển khai backend sau ADR-0074? → A: 3 image (`core-api`, `realtime`, `worker`) + image `migrate`, đẩy lên `ghcr.io` ([ADR-0088](../../docs/adr/ADR-0088-ci-cd-bang-github-actions.md)); `deploy/swarm/<env>/app.stack.yml` chứa 5 Swarm service `core-api`, `realtime`, `worker-events`, `worker-chat`, `worker-jobs` (`cmd/worker --profile=events|chat|jobs`); một version backend `backend/vX.Y.Z` cho cả 3 binary; job migrate `migrate --module=<module>` cho từng database module trước rollout ([DEPLOYMENT §3.1, §11, §12](../../docs/11-operations/DEPLOYMENT.md)).
- Q: Nền CI/CD? → A: GitHub Actions (ADR-0088): `deploy-config.yml` (validate stack, secret refs, schema), `release.yml` (build, quét, ký, deploy theo tag), `security.yml` (gitleaks); job deploy chạy trên self-hosted runner nhãn `deploy` trong mạng quản trị; secret theo GitHub Environment `staging`/`production` (production có required reviewers).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Deploy, rollback và scale từng Swarm service độc lập trên Swarm (Priority: P1)

DevOps phát hành tag `backend/v1.4.0`: workflow `release.yml` chạy job migrate của từng database module rồi rolling update các Swarm service backend (production: canary `core-api`); lỗi thì rollback riêng Swarm service đó trong vài phút, các Swarm service khác không bị chạm.

**Why this priority**: NFR-DEPL-001 là bất biến; không có pipeline deploy thì không có staging cho load/chaos test (F132).

**Independent Test**: Trên staging, deploy bản mới cho `core-api`, cố ý làm task mới không healthy, xác nhận tự rollback và số lần restart task của 4 Swarm service backend còn lại (`realtime`, `worker-events`, `worker-chat`, `worker-jobs`) = 0.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F130-AC-01 | Mỗi môi trường staging/production PHẢI có `deploy/swarm/<env>/infra.stack.yml`, `data.stack.yml` và `app.stack.yml` chứa 5 Swarm service backend (`core-api`, `realtime`, `worker-events`, `worker-chat`, `worker-jobs`); image tham chiếu `ghcr.io` với tag bất biến (không `latest`), có `placement` theo `node.labels.role`, `resources.limits/reservations`, `update_config` (`start-first`, `failure_action: rollback`, `monitor`), `rollback_config`, `healthcheck`; NẾU stack file vi phạm, THÌ job `validate` của workflow `deploy-config.yml` PHẢI fail | NFR-DEPL-001 · [DEPLOYMENT §4.5](../../docs/11-operations/DEPLOYMENT.md) · ADR-0024 · ADR-0074 | — |
| F130-AC-02 | KHI deploy backend, CI PHẢI chạy image `migrate` (`--module=<module>`) cho từng database module dưới dạng `replicated-job` chỉ với secret `<module>_migrator_dsn`, chờ mọi task `Complete` rồi mới `docker stack deploy`; NẾU migrate của bất kỳ module nào thất bại hoặc quá thời gian, THÌ workflow PHẢI dừng và không rollout | NFR-DEPL-001 · [DEPLOYMENT §12](../../docs/11-operations/DEPLOYMENT.md) · ADR-0039 | — |
| F130-AC-03 | KHI deploy hoặc rollback một Swarm service backend, HỆ THỐNG PHẢI KHÔNG khởi động lại task của Swarm service khác; NẾU task mới không healthy trong thời gian `monitor`, THÌ Swarm PHẢI tự rollback; `make deploy-rollback SERVICE=<swarm-service> ENV=<env>` PHẢI đưa Swarm service đó về spec trước trong ≤ 15 phút | NFR-DEPL-001 · [DEPLOYMENT §11.3](../../docs/11-operations/DEPLOYMENT.md) | TC-SVC-015 |
| F130-AC-04 | Ở NƠI môi trường là production, CI PHẢI deploy `core-api-canary` (1–2 replica, image mới) và cấu hình APISIX `traffic-split` 10% trong 30 phút; NẾU error rate 5xx hoặc p95 của canary (theo từng module) vượt ngưỡng alert `ModuleHighErrorRate`/`APILatencyP95High`, THÌ CI PHẢI gỡ canary và dừng; nếu đạt thì cập nhật `core-api` chính rồi gỡ canary | [DEPLOYMENT §11.2](../../docs/11-operations/DEPLOYMENT.md) | — |
| F130-AC-05 | Job deploy production của `release.yml` (GitHub Environment `production`) PHẢI chỉ chạy với tag `backend/vX.Y.Z` có cùng image digest đã deploy staging thành công, có approval của Release Manager (required reviewer của environment) và ngoài khung giờ học (trừ nhãn hotfix) | [DEPLOYMENT §2, §10.3](../../docs/11-operations/DEPLOYMENT.md) · ADR-0088 | — |
| F130-AC-06 | Swarm `healthcheck` PHẢI dùng `/livez` qua lệnh con `healthcheck` của binary; APISIX PHẢI health check chủ động `/readyz` trên `tasks.core-api` và `tasks.realtime`; `stop_grace_period` PHẢI ≥ thời gian drain (`core-api` 40 s, `realtime` 150 s) | FR-OPS-002 · [DEPLOYMENT §14](../../docs/11-operations/DEPLOYMENT.md) · NFR-AVAIL-004 | — |
| F130-AC-07 | HỆ THỐNG PHẢI tạo overlay network `edge`, `app`, `data`, `admin` (`app`, `data`, `admin` có `--opt encrypted`); mỗi Swarm service chỉ gắn network trong [DEPLOYMENT §4.3](../../docs/11-operations/DEPLOYMENT.md); NẾU task chỉ gắn `edge` thử kết nối PostgreSQL/Valkey/NATS/object storage, THÌ kết nối PHẢI thất bại; datastore PHẢI KHÔNG publish cổng ra host | NFR-DEPL-002 · ADR-0024 · ADR-0075 | TC-SVC-007 |

---

### User Story 2 — Secret chỉ qua Docker secrets, mỗi Swarm service chỉ thấy secret của module nó nạp (Priority: P1)

Security Engineer cần bảo đảm không secret nào nằm trong image, Git (trừ dạng SOPS mã hóa), env file hay log; xoay secret không gián đoạn.

**Why this priority**: FR-OPS-004, AGENTS R3/R4; vi phạm là sự cố bảo mật.

**Independent Test**: `docker service inspect` mọi Swarm service không có biến chứa giá trị secret; thử mount `chat_purger_dsn` vào `core-api` → CI đỏ; xoay `learning_db_dsn` → rolling update không lỗi 5xx.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F130-AC-08 | Mọi credential trong bảng [DEPLOYMENT §13](../../docs/11-operations/DEPLOYMENT.md) PHẢI vào container chỉ qua Docker secret mount và biến `*_FILE`; `<swarm-service>.env` và stack file PHẢI KHÔNG chứa giá trị secret; NẾU phát hiện, THÌ job `validate` của `deploy-config.yml` và gitleaks của `security.yml` PHẢI fail | FR-OPS-004 · AGENTS R3, R4 | — |
| F130-AC-09 | Mỗi Swarm service PHẢI chỉ tham chiếu secret có tiền tố của module mà binary/profile đó nạp và secret riêng của binary (NATS creds) theo [DEPLOYMENT §13](../../docs/11-operations/DEPLOYMENT.md); `kc_game_backend` chỉ được mount vào `core-api` (module `identity`), `kc_svc_identity` chỉ vào Swarm service chạy module `identity`, `chat_purger_dsn` chỉ vào `worker-chat` (job `gameplay-chat-purge`), `<module>_migrator_dsn` chỉ vào job migrate, `realtime` PHẢI KHÔNG nhận DB credential; NẾU vi phạm, THÌ CI PHẢI fail | FR-OPS-004 · [DEPLOYMENT §13](../../docs/11-operations/DEPLOYMENT.md) · ADR-0068 · ADR-0074 | — |
| F130-AC-10 | KHI xoay secret theo `secret_rotation` hoặc khi nghi lộ, job `secrets-sync` PHẢI tạo secret mới hậu tố `_v<N+1>`, cập nhật stack bằng rolling update và giữ bản `_v<N>` tới khi rollout xong; client secret Keycloak (`svc-identity`, `eduworld-game-backend`) PHẢI có hai secret song song trong thời gian xoay | FR-OPS-004 · [SECURITY_ARCHITECTURE §6](../../docs/10-security/SECURITY_ARCHITECTURE.md) · [DEPLOYMENT §4.5](../../docs/11-operations/DEPLOYMENT.md) | — |
| F130-AC-11 | NẾU file secret bắt buộc không tồn tại hoặc rỗng khi khởi động, THÌ binary PHẢI thoát mã ≠ 0 với log chỉ nêu tên biến (không giá trị); log và `docker service inspect` PHẢI KHÔNG chứa giá trị secret | AGENTS R4, R14 · [MONITORING §8](../../docs/11-operations/MONITORING.md) | TC-AUTH-060 |

---

### User Story 3 — Backup PITR cụm PostgreSQL (từng database module) và object storage (Priority: P1)

DevOps có backup liên tục cho cụm PostgreSQL chứa 5 database module + `keycloak` (ADR-0074 mục 4), object storage, etcd APISIX; mỗi job báo trạng thái bằng metric để F131 cảnh báo.

**Why this priority**: FR-OPS-002 (backup script); RPO theo `rpo_rto` là điều kiện go-live (DEPLOYMENT §15).

**Independent Test**: Trên staging, chạy full + diff, kiểm `pgbackrest info`; dừng WAL archive giả lập → metric trạng thái = 0; object storage sync sang đích ngoài máy và so checksum.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F130-AC-12 | HỆ THỐNG PHẢI chạy WAL archiving liên tục, full backup hằng tuần, differential hằng ngày cho cụm PostgreSQL và logical dump `pg_dump -Fc` hằng tuần cho từng database module theo lịch và số bản giữ ở [BACKUP_RECOVERY §3.2](../../docs/11-operations/BACKUP_RECOVERY.md); repo backup PHẢI nằm trên S3 khác hạ tầng chính, mã hóa phía client và bật immutability | FR-OPS-002 · [BACKUP_RECOVERY §3](../../docs/11-operations/BACKUP_RECOVERY.md) | — |
| F130-AC-13 | KHI một job backup kết thúc, job PHẢI ghi `eduworld_backup_last_run_timestamp`, `eduworld_backup_last_status`, `eduworld_backup_duration_seconds` và (khi thành công) `eduworld_backup_last_success_timestamp` theo `target` (`postgres`, `object_storage`, `etcd`, `keycloak_export`); `pgbackrest check` PHẢI chạy hằng ngày | [BACKUP_RECOVERY §3.4, §10](../../docs/11-operations/BACKUP_RECOVERY.md) | — |
| F130-AC-14 | TRONG KHI staging/production chạy backend object storage theo `object_storage_backend` (S3-compatible managed tại Việt Nam hoặc Garage/SeaweedFS ≥ 3 node replication factor 3, chọn sau spike; floci chỉ ở local/CI), HỆ THỐNG PHẢI (a) bật versioning cho `ew-media-public`, `ew-media-private`, `ew-game-bundles`; (b) bật replication liên tục cho bucket chứa dữ liệu học sinh (`ew-media-private`; `ew-student-submissions` nếu có) sang vị trí thứ hai với độ trễ sao chép ≤ RPO dữ liệu học tập của `rpo_rto` và xuất metric độ trễ replication; (c) đồng bộ hằng ngày mọi bucket trừ `ew-media-quarantine` sang bản sao ngoài cụm/ngoài nhà cung cấp theo `object_storage_backup`, mã hóa khi lưu và kiểm checksum khóa `media/<sha256>/…` sau mỗi lần; snapshot etcd APISIX và export realm Keycloak (không secret) PHẢI chạy hằng ngày | [BACKUP_RECOVERY §1, §2, §6](../../docs/11-operations/BACKUP_RECOVERY.md) · ADR-0075 | — |
| F130-AC-15 | Credential ghi backup PHẢI chỉ có quyền ghi repo; credential đọc/khôi phục PHẢI tách riêng và chỉ cấp khi chạy runbook; mỗi lần khôi phục PHẢI ghi người thực hiện, thời điểm, mục đích vào báo cáo khôi phục | [BACKUP_RECOVERY §3.3](../../docs/11-operations/BACKUP_RECOVERY.md) | — |

---

### User Story 4 — Diễn tập khôi phục, failover và DR drill (Priority: P1)

DevOps chứng minh khôi phục được: PITR toàn cụm, khôi phục riêng một database module rồi đưa về nhất quán liên module, failover Patroni, khôi phục object storage, DR toàn site.

**Why this priority**: Exit criteria M6 (ROADMAP §3.7) và DEPLOYMENT §15 yêu cầu restore test trong 30 ngày.

**Independent Test**: Chạy `scripts/backup/restore_test.sh --mode=module --db=eduworld_learning` trên staging; báo cáo `restore-tests/<YYYY-MM>.md` có thời gian, đối soát.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F130-AC-16 | KHI chạy restore test tháng, `restore_test.sh` PHẢI khôi phục full + PITR về thời điểm ngẫu nhiên trong 7 ngày vào môi trường cô lập, hoàn thành trong ngưỡng [BACKUP_RECOVERY §8](../../docs/11-operations/BACKUP_RECOVERY.md), đối soát số attempt đã submit và ghi kết quả vào `docs/11-operations/restore-tests/<YYYY-MM>.md` | FR-OPS-002 · [BACKUP_RECOVERY §8](../../docs/11-operations/BACKUP_RECOVERY.md) | — |
| F130-AC-17 | KHI khôi phục riêng một database module về T, runbook PHẢI: thay database, reset consumer durable của module đó về T trên `EVT_*` (hoặc relay outbox khi quá `event_stream_retention`), rebuild projection `ref_*`, chạy `retention-reapply` và `reconcile-<module>`; các module khác PHẢI tiếp tục phục vụ | [BACKUP_RECOVERY §3.5, §7](../../docs/11-operations/BACKUP_RECOVERY.md) | TC-SVC-014 |
| F130-AC-18 | KHI switchover Patroni (diễn tập quý), gián đoạn ghi PHẢI < 60 s và không mất giao dịch đã commit; `core-api` và `worker-*` PHẢI tự kết nối lại qua PgBouncer mà không restart task | [BACKUP_RECOVERY §8, RB-02](../../docs/11-operations/BACKUP_RECOVERY.md) | TC-SVC-013 |
| F130-AC-19 | KHI khôi phục database `keycloak`, runbook RB-04 PHẢI rotate realm keys và đặt `not-before` trước khi mở lại đăng nhập | [BACKUP_RECOVERY §4, RB-04](../../docs/11-operations/BACKUP_RECOVERY.md) | — |
| F130-AC-20 | KHI diễn tập RB-05 trên staging, HỆ THỐNG PHẢI khôi phục object storage theo backend đã chọn: với Garage/SeaweedFS tự host, mất 1 node `storage` PHẢI không mất dữ liệu và không gián đoạn đọc/ghi, thay node xong cụm tự cân bằng; với cả hai backend, dựng bucket/cụm mới (cùng tên bucket, credential mới qua Docker secret) và khôi phục từ vị trí replication (bucket dữ liệu học sinh, mất ≤ RPO dữ liệu học tập) hoặc bản sao ngoài cụm gần nhất (bucket còn lại), checksum PHẢI khớp 100% object được `media_assets` tham chiếu, game và portal tải được media qua CDN/signed URL, trong ngưỡng [BACKUP_RECOVERY §8](../../docs/11-operations/BACKUP_RECOVERY.md) | ADR-0075 · RB-05 · [BACKUP_RECOVERY §1](../../docs/11-operations/BACKUP_RECOVERY.md) | — |
| F130-AC-21 | KHI diễn tập DR toàn site (hằng năm), HỆ THỐNG PHẢI khôi phục mọi database về cùng T, reset mọi consumer về T và đạt `rpo_rto` | [BACKUP_RECOVERY §1, §3.5, RB-03](../../docs/11-operations/BACKUP_RECOVERY.md) | — |

---

### User Story 5 — Scale theo lịch học (Priority: P2)

Trước giờ vào học, replica Keycloak, `core-api`, `realtime`… tự tăng đến mức đỉnh; sau khung giờ thì thu nhỏ — không autoscale theo tải.

**Why this priority**: ADR-0024 mục 4, ADR-0045; cần trước TC-PERF-002 (F132).

**Independent Test**: Đặt khung giờ thử trong `scale-schedule.yaml` staging; xác nhận replica đạt `peak` và healthy trước giờ bắt đầu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F130-AC-22 | KHI còn 15 phút trước một khung trong `warm_up_windows`, `ops-scheduler` PHẢI scale mọi Swarm service trong `deploy/swarm/<env>/scale-schedule.yaml` lên `peak`, thu về `baseline` sau khung, và tăng `eduworld_ops_scale_actions_total{service,action,result}` | ADR-0024 · ADR-0045 · [DEPLOYMENT §4.4](../../docs/11-operations/DEPLOYMENT.md) | TC-PERF-017 |
| F130-AC-23 | NẾU tổng `resources.reservations` sau scale vượt năng lực còn lại của node vai trò, hoặc lệnh Docker API lỗi, THÌ `ops-scheduler` PHẢI không scale, ghi `result="rejected"`/`"error"`; socket proxy PHẢI chỉ cho phép `GET /services`, `GET /nodes`, `GET /tasks`, `POST /services/{id}/update` | [DEPLOYMENT §4.4](../../docs/11-operations/DEPLOYMENT.md) · NFR-SCAL-003 | — |
| F130-AC-24 | NẾU `scale-schedule.yaml` sai schema (thiếu `baseline`/`peak`, `peak` < `baseline`, tên Swarm service không có trong stack, chuỗi vượt giới hạn độ dài schema), THÌ job `validate` của `deploy-config.yml` PHẢI fail với vị trí lỗi | ADR-0041 · AGENTS R33 | — |

### Edge Cases

- NẾU mất quorum manager Swarm, THÌ task đang chạy vẫn phục vụ; workflow deploy PHẢI fail sớm (kiểm `docker node ls`), không deploy một phần.
- NẾU migrate thành công nhưng rollout lỗi, THÌ rollback chỉ lùi code; schema mới tương thích ngược theo expand/contract (F130-AC-02, AGENTS R12).
- NẾU rollback `core-api`, THÌ cả 5 module domain cùng lùi (chung image); muốn tắt riêng một tính năng lỗi thì dùng feature flag ([DEPLOYMENT §11.3](../../docs/11-operations/DEPLOYMENT.md)).
- NẾU backup chạy trùng lúc switchover Patroni, THÌ pgBackRest PHẢI backup từ primary mới hoặc fail rõ ràng (metric status = 0), không tạo bản hỏng im lặng.
- NẾU restore một database cũ hơn lần dọn học sinh, THÌ job `student-inactive-cleanup` chạy lại theo quy tắc (BACKUP_RECOVERY §7); purge chat gameplay chạy lại (ADR-0033).
- Authorization ở mức endpoint, tenant isolation, idempotency, Problem Details: không áp dụng — F130 không tạo endpoint nghiệp vụ; Docker API chỉ qua TLS client cert của self-hosted runner deploy và socket proxy allowlist (F130-AC-23); cô lập dữ liệu ở mức network/credential (F130-AC-07, F130-AC-09). Giới hạn độ dài input: schema `scale-schedule` (F130-AC-24).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-OPS-002 | Stack file staging/production, script backup/restore, runbook tự động hóa, cập nhật DEPLOYMENT theo thực tế | DEPLOYMENT §4–§12, BACKUP_RECOVERY |
| FR-OPS-004 | Docker secrets có version, SOPS, phân quyền mount theo module/binary, xoay vòng | DEPLOYMENT §13, SECURITY_ARCHITECTURE §6 |
| NFR-DEPL-001 | Stack/canary/rollback/scale theo từng Swarm service (`core-api`, `realtime`, `worker-*`), migrate theo database module | DEPLOYMENT §4.5, §11, §12 |

### Key Entities

- **Stack môi trường**: tập stack file của một môi trường (`infra`, `data`, `app` với 5 Swarm service backend), network overlay, nhãn node theo vai trò.
- **Docker secret có version**: credential của một module/binary/job, tên `<owner>_<name>_v<N>`, bất biến.
- **Lịch scale**: `baseline`, `peak`, khung giờ cho từng Swarm service của một môi trường.
- **Repo backup**: stanza pgBackRest của cụm, bản backup object storage/etcd/realm, metric trạng thái.
- **Báo cáo khôi phục**: kết quả mỗi lần restore test/diễn tập (thời gian, người làm, mục đích, đối soát).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `warm_up_windows` | Khung giờ cao điểm cho `ops-scheduler` |
| `rpo_rto` | Mục tiêu diễn tập DR và cấu hình WAL archive |
| `object_storage_backup` | Lịch và số bản backup object storage, chu kỳ diễn tập RB-05 |
| `secret_rotation` | Chu kỳ xoay secret |
| `event_stream_retention` / `outbox_retention` | Chọn reset consumer hay relay outbox khi khôi phục |
| `alert_thresholds` | Ngưỡng `BackupMissing` mà metric backup phải phục vụ (alert do F131) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không Kubernetes, không autoscale theo tải | ADR-0024 |
| Ba binary, năm Swarm service backend, một cụm PostgreSQL chứa 5 database module | ADR-0074 |
| Object storage staging/production không dùng floci; bucket dữ liệu học sinh có RPO như dữ liệu học tập; bản sao ngoài cụm bắt buộc | ADR-0075 |
| Không auto-migrate; expand/contract | AGENTS R12 · ADR-0039 |
| Không secret trong Git/image/log | AGENTS R3, R4, R14 |
| Không xóa vật lý dữ liệu nghiệp vụ khi khôi phục (chỉ `retention-reapply` ẩn danh hóa) | ADR-0028 · ADR-0070 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Deploy/rollback một Swarm service backend gây 0 lần restart task của Swarm service khác; rollback hoàn tất ≤ 15 phút trong 100% lần diễn tập.
- **SC-002**: 0 giá trị secret xuất hiện trong stack file, env file, image, log (quét tự động mỗi workflow).
- **SC-003**: Mỗi database module có ít nhất một restore test PITR thành công trong 30 ngày gần nhất; RPO đo được ≤ `rpo_rto`.
- **SC-004**: Diễn tập khôi phục riêng một database module kết thúc với 0 lệch không giải thích được.
- **SC-005**: 100% khung cao điểm trong tháng có replica đạt `peak` và healthy trước giờ bắt đầu.

## Ngoài phạm vi

- Compose local, workflow CI theo path, quét image/SBOM → F001; route APISIX, realm Keycloak → F004.
- Dashboard, alert rule, runbook theo alert, error tracking → F131; load/chaos test, pentest → F132; build/phát hành client, CDN `play.<domain>` → F133.
- Lệnh nghiệp vụ `reconcile-<module>`, `projection rebuild`, `retention-reapply`, `saga resume` → module sở hữu (F003 platform kit, F016, F045 và feature nền của từng module); F130 chỉ điều phối.
- Dịch vụ managed cloud thay PostgreSQL/Valkey/object storage: không làm (OQ-10); Kubernetes: cần ADR mới.

## Assumptions

- OQ-10 chưa chốt: thiết kế không phụ thuộc cloud hay on-prem; secret qua SOPS [Proposed]; Vault/OpenBao thay được bằng script `secrets-sync`.
- CI/CD là GitHub Actions (ADR-0088): job deploy chạy trên self-hosted runner nhãn `deploy` trong mạng quản trị, có quyền Docker API manager qua TLS, không nhận job từ PR của fork; secret theo GitHub Environment `staging`/`production`; action ghim theo SHA commit (DEPLOYMENT §10).
- `reconcile-<module>` (lệnh con `worker reconcile --module=<module>` [Proposed]) chưa có feature sở hữu trong CODE_PLAN; F130 định nghĩa giao diện CLI (mã thoát, JSON kết quả) và chờ module bổ sung — rủi ro cho TC-SVC-014.
- ADR-0074 (modular monolith) Accepted 2026-11-17: stack, secret, migrate, canary, rollback trong spec này theo 3 binary/5 Swarm service và một cụm PostgreSQL.
- ADR-0075 Accepted (thay ADR-0071): staging/production dùng S3-compatible managed tại Việt Nam (ưu tiên) hoặc Garage/SeaweedFS ≥ 3 node; floci chỉ ở local/CI. Lựa chọn backend do spike trước pilot quyết định (`object_storage_backend` [TBD] trong CONFIG_DEFAULTS); F130-AC-14, F130-AC-20 viết để đạt với cả hai backend, stack `data.stack.yml` chỉ chứa cụm `storage` khi chọn tự host.

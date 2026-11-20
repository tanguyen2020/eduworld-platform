# Research: F130 — Staging/production Docker Swarm, secret, backup PITR và DR drill

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Nguồn secret (OQ-10, đã trả lời 2026-11-20: giữ SOPS)

- **Decision**: SOPS + age; file `deploy/secrets/<env>/<owner>.sops.yaml` (khóa = tên secret, giá trị mã hóa). Workflow `secrets-sync.yml` (self-hosted runner nhãn `deploy`, GitHub Environment `staging`/`production` có required reviewers) giải mã bằng khóa age lưu trong secret của environment, tạo `docker secret create <owner>_<name>_v<N>` khi giá trị đổi (so hash SHA-256 lưu ở label `ew.sha256`), rồi cập nhật stack. Khóa age production chỉ Release Manager + DevOps lead giữ.
- **Rationale**: DEPLOYMENT §13 cho phép "SOPS trong Git"; không thêm hạ tầng stateful khi OQ-10 còn mở; diff được review, có lịch sử.
- **Alternatives**: Vault/OpenBao (thêm cụm HA cần vận hành — để sau OQ-10, chỉ thay `secrets-sync.sh`); secret cloud KMS (phụ thuộc OQ-10).

## R2. pgBackRest: repo host và kết nối

- **Decision**: Image `eduworld/backup-tools` (pgBackRest, `aws` CLI, `etcdctl`, `jq`) chạy trên node `ops` làm repo host; mỗi node `data-pg` chạy `pgbackrest server` (TLS, cert riêng, chỉ trên network `data`) cạnh Patroni. Một stanza cho cụm PostgreSQL duy nhất (`eduworld`: 5 database module + `keycloak`, ADR-0074 mục 4, DEPLOYMENT §7) ở cả staging và production; khi một database tải lớn được chuyển sang cụm riêng (DEPLOYMENT §7, cần số đo) thì thêm stanza cho cụm đó. `archive-push` chạy từ PostgreSQL (`archive_command`) vào repo S3.
- **Rationale**: Không cần SSH; khôi phục `--db-include` một database (BACKUP_RECOVERY §3.1); backup chạy từ standby (`backup-standby=y`) giảm tải primary.
- **Alternatives**: WAL-G (không có `--db-include`); systemd timer trên node DB (ngoài Swarm, khó quan sát).

## R3. Lịch trong `ops-scheduler`

- **Decision**: Tách hai trách nhiệm. (a) `ops-scheduler` chỉ scale theo `scale-schedule.yaml`, gọi Docker API qua socket proxy allowlist (`GET /services`, `GET /nodes`, `GET /tasks`, `POST /services/{id}/update`) — khớp F130-AC-23. (b) Service `backup-tools` (1 replica, node `ops`, không quyền Docker API) chạy `supercronic` với `deploy/swarm/<env>/ops-jobs.crontab` (backup full/diff, logical dump, object storage, etcd, realm export, `pgbackrest check`, restore test tháng); mỗi job gọi `scripts/backup/*.sh` và ghi metric textfile.
- **Rationale**: Quyền Docker API tối thiểu cho tiến trình duy nhất cần nó (DEPLOYMENT §4.4); lịch backup nằm trong Git (BACKUP_RECOVERY §10).
- **Alternatives**: `ops-scheduler` tạo `replicated-job` cho backup (phải mở `POST /services/create` — quyền quá rộng); `swarm-cronjob` (bên thứ ba có quyền Docker API rộng); cron trên host (ngoài Git, khó audit).

## R4. Canary trên Swarm

- **Decision**: `canary.sh` tạo Swarm service `core-api-canary` (cùng network, label `ew.canary=true`), áp route APISIX `traffic-split` (weight 10 cho upstream `tasks.core-api-canary`) qua Admin API nội bộ; theo dõi 30 phút bằng truy vấn Prometheus (`eduworld_http_server_requests_total{service="core-api-canary",module=…,status_class="5xx"}` và p95, xét từng module) với ngưỡng đọc từ rule alert (không định nghĩa lại); đạt → `docker stack deploy` cập nhật `core-api` chính → gỡ `traffic-split` → xóa canary. `realtime` và `worker-*` không canary (rolling + drain, DEPLOYMENT §11.2).
- **Rationale**: DEPLOYMENT §11.2; Swarm không có canary gốc.
- **Alternatives**: Blue/green toàn stack (gấp đôi tài nguyên node `app`).

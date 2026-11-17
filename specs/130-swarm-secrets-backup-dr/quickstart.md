# Quickstart: F130 — Staging/production Docker Swarm, secret, backup PITR và DR drill

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F001 (image, health, init role DB), F004 (realm, route APISIX); có image `core-api`, `realtime`, `worker`, `migrate` trên `ghcr.io` để deploy.
- Kiểm thử nhanh trên máy dev: `docker swarm init` (1 node, gắn đủ nhãn bằng `deploy/swarm/nodes/local-swarm.yaml`), `make swarm-validate`, `make swarm-up ENV=staging-lite` (override 1 replica). Bằng chứng nghiệm thu chạy trên **staging** (topology [DEPLOYMENT §4.1](../../docs/11-operations/DEPLOYMENT.md)).
- Dữ liệu: seed giả theo [LOCAL_DEVELOPMENT §5](../../docs/11-operations/LOCAL_DEVELOPMENT.md) (không dữ liệu học sinh thật). Tài khoản demo: `hs.an.3a` (học sinh), `ad.truong.a` (school_admin) để kiểm ứng dụng còn hoạt động sau deploy/khôi phục.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `make swarm-validate ENV=production`; sửa một stack thành `image: …:latest` | Lần đầu exit 0; lần sau job `validate` của `deploy-config.yml` đỏ, chỉ ra file/dòng | F130-AC-01 |
| 2 | Tag `backend/v0.0.1-rc1`; xem workflow `release.yml` (deploy staging) | Job `migrate --module=<module>` (`replicated-job`) của cả 5 module Complete trước `stack deploy`; cố ý làm migration `learning` lỗi → workflow dừng, mọi Swarm service giữ bản cũ | F130-AC-02 |
| 3 | Deploy image `core-api` có `/livez` lỗi; đếm `swarm_task_restarts_total` của `realtime`, `worker-*` | Tự rollback sau `monitor`; Swarm service khác 0 restart; `make deploy-rollback SERVICE=core-api ENV=staging` < 15 phút | F130-AC-03 |
| 4 | Chạy `canary.sh` trên production-like (staging bật cờ canary) với image `core-api` trả 5xx 10% | `core-api-canary` bị gỡ, `core-api` chính không đổi | F130-AC-04 |
| 5 | Chạy job deploy production của `release.yml` với tag chưa qua staging | Job từ chối (digest không khớp/thiếu approval của environment `production`) | F130-AC-05 |
| 6 | `docker service inspect eduworld_core-api`; `docker exec` thử kết nối `postgres:5432` từ task APISIX | Healthcheck `/app/core-api healthcheck --endpoint=/livez`; kết nối từ APISIX thất bại | F130-AC-06, F130-AC-07 |
| 7 | Quét stack/env bằng `check-secret-refs.sh`; thêm `chat_purger_dsn` vào `core-api` hoặc `learning_db_dsn` vào `realtime` | Script đỏ, nêu secret sai owner | F130-AC-08, F130-AC-09 |
| 8 | Đổi giá trị `learning_db_dsn` trong SOPS, chạy `secrets:sync` | Có `learning_db_dsn_v2`, rolling update, 0 lỗi 5xx; `_v1` còn tới khi xong | F130-AC-10 |
| 9 | Xóa mount secret `game_db_dsn` của `core-api`, khởi động | Task thoát ≠ 0, log chỉ có tên biến | F130-AC-11 |
| 10 | `pgbackrest --stanza=eduworld info`; xem metric `eduworld_backup_last_success_timestamp` | Có full/diff, WAL liên tục; metric theo `target` | F130-AC-12, F130-AC-13 |
| 11 | Chạy `object_storage_backup.sh`, `etcd_snapshot.sh`, `keycloak_export.sh`; xem metric độ trễ replication `ew-media-private` | Bản sao ngoài cụm; `manifest.sha256` khớp; versioning bật; độ trễ replication ≤ RPO dữ liệu học tập; realm export không có secret | F130-AC-14 |
| 12 | Dùng credential ghi backup để đọc repo | Bị từ chối | F130-AC-15 |
| 13 | `restore_test.sh --mode=full` | Báo cáo `restore-tests/<YYYY-MM>.md`, thời gian trong ngưỡng, số attempt khớp | F130-AC-16 |
| 14 | `restore_test.sh --mode=module --db=eduworld_learning --target=<T>` | Consumer reset, projection rebuild, `worker reconcile --module=learning` 0 lệch không giải thích; các module khác vẫn phục vụ; `hs.an.3a` vẫn làm bài được | F130-AC-17 |
| 15 | `patroni-switchover.sh` ngoài giờ học | Gián đoạn ghi < 60 s; `core-api`, `worker-*` kết nối lại qua PgBouncer, 0 restart task | F130-AC-18 |
| 16 | `rb04-keycloak-restore.sh` trên staging | Realm keys mới, token cũ bị từ chối; `ad.truong.a` đăng nhập lại được | F130-AC-19 |
| 17 | `rb05-object-storage-restore.sh` (tự host: dừng node `storage-2` rồi thay node; cả hai backend: khôi phục vào bucket mới từ replication/bản sao ngoài cụm) | Mất 1 node không gián đoạn; checksum 100% object được tham chiếu; game, portal tải media qua CDN/signed URL | F130-AC-20 |
| 18 | DR drill (site dự phòng) theo RB-03 | Đạt `rpo_rto` | F130-AC-21 |
| 19 | Đặt khung thử trong `scale-schedule.yaml` staging | Replica đạt `peak` trước khung; metric `eduworld_ops_scale_actions_total{result="ok"}` | F130-AC-22 |
| 20 | Đặt `peak` vượt năng lực node; gọi `POST /services/create` qua socket proxy | `result="rejected"`; proxy trả 403 | F130-AC-23 |
| 21 | `ops-scheduler validate` file có `peak < baseline` | Exit ≠ 0 với vị trí lỗi | F130-AC-24 |

## 3. Lệnh test tự động

```bash
make swarm-validate ENV=staging && make swarm-validate ENV=production
bats tests/ops
go -C src/backend test ./tools/ops-scheduler/...
```

## 4. Bằng chứng cần nộp

- Log workflow deploy/rollback/canary, báo cáo `restore-tests/<YYYY-MM>.md` (full, service, switchover, RB-04, RB-05, DR), ảnh metric backup/scale, kết quả `bats`/`go test`.

# Data Model: F130 — Staging/production Docker Swarm, secret, backup PITR và DR drill

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F130 không tạo hay sửa bảng trong database của module nào. Dữ liệu bền vững của feature là **file cấu hình trong Git** và **bản backup ngoài database**; định dạng dưới đây là hợp đồng nội bộ giữa script và CI.

### Lịch scale — `deploy/swarm/<env>/scale-schedule.yaml` (Mới, schema `deploy/swarm/schema/scale-schedule.schema.json`)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `windows[]` | mảng `{name, start, end}` | `name` maxLength 32; `start`/`end` `HH:MM` (pattern) | Giá trị mặc định lấy từ `warm_up_windows`; giờ ICT |
| `lead_minutes` | integer | 0–60 | Mặc định theo DEPLOYMENT §4.4 (15) |
| `services[]` | mảng `{name, baseline, peak}` | `name` maxLength 64, phải khớp tên Swarm service trong stack (`core-api`, `realtime`, `worker-events`, `worker-chat`, `worker-jobs`, `keycloak`, `apisix`…); `peak ≥ baseline ≥ 1` | Số replica theo DEPLOYMENT §4.2 |

### Docker secret — tên `<owner>_<name>_v<N>`

| Thuộc tính | Ràng buộc |
|---|---|
| `owner` | một trong `identity`, `learning`, `game`, `competition`, `chat` (credential theo module), `core`, `worker`, `realtime` (NATS creds theo binary), `chat_purger`, `kc_game_backend`, `kc_svc_identity`, `backup`, `ops`, `apisix`, `keycloak`, `s3_<binary>` (credential object storage theo binary, [DEPLOYMENT §9b](../../docs/11-operations/DEPLOYMENT.md) [Confirmed]) (theo [DEPLOYMENT §13](../../docs/11-operations/DEPLOYMENT.md)) |
| label `ew.sha256` | hash giá trị (phát hiện đổi khi đồng bộ) |
| Bất biến | Đổi giá trị = tạo `_v<N+1>`; xóa bản cũ chỉ sau khi không Swarm service nào tham chiếu |

### Repo backup — S3 `eduworld-backups` (ngoài hạ tầng chính)

| Đường dẫn | Nội dung | Giữ lại |
|---|---|---|
| `/pgbackrest/<stanza>/` | WAL + full/diff pgBackRest của cụm (mã hóa), khôi phục được từng database (`--db-include`) | BACKUP_RECOVERY §3.2 |
| `/logical/<database>/<YYYY-MM-DD>.dump` | `pg_dump -Fc` theo database module | BACKUP_RECOVERY §3.2 |
| `/object-storage/<YYYY-MM-DD>/` | Bản sao ngoài cụm/ngoài nhà cung cấp của mọi bucket (trừ quarantine) + `manifest.sha256` | `object_storage_backup` |
| `/etcd/<YYYY-MM-DD>.db`, `/keycloak/<YYYY-MM-DD>.json` | Snapshot etcd APISIX, realm export không secret | 30 ngày |

### Báo cáo khôi phục — `docs/11-operations/restore-tests/<YYYY-MM>.md` (Mới)

Trường bắt buộc: loại kiểm thử (BACKUP_RECOVERY §8), môi trường, người thực hiện, mục đích, thời điểm đích T, thời gian bắt đầu/kết thúc, kết quả đối soát (số attempt submit, `reconcile-<module>`), vấn đề, link pipeline.

## 2. Vòng đời / trạng thái

Deploy backend (tag `backend/vX.Y.Z`): `validate` → `migrate job từng database module (Complete)` → `deploy staging` → `approve` (environment `production`) → `canary core-api 10% / 30'` → `promote` (`core-api`, rồi rolling `realtime`, `worker-*`) hoặc `abort` → (khi cần) `rollback` theo Swarm service.

## 3. Quy tắc kiểm tra dữ liệu

- Stack file chỉ tham chiếu secret của module/binary mà Swarm service nạp (F130-AC-09).
- `scale-schedule.yaml` hợp lệ theo schema (F130-AC-24).
- Không có lệnh `DELETE`/`TRUNCATE` trong runbook khôi phục; xóa theo hạn chỉ do `retention-reapply` (ẩn danh hóa) và job purge ADR-0033.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | F130 không phát event nghiệp vụ; runbook chỉ reset consumer/relay outbox có sẵn | — |

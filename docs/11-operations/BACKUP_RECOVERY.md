# Backup & Recovery

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: DevOps Engineer · Cập nhật: 2026-11-15
> Liên quan: [DEPLOYMENT](DEPLOYMENT.md) · [MONITORING](MONITORING.md) · [ADR-0004, ADR-0037, ADR-0042](../adr/README.md) · [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [SYSTEM_ARCHITECTURE §7](../06-architecture/SYSTEM_ARCHITECTURE.md#7-non-functional-baseline)

**Mục đích & phạm vi.** Nguồn duy nhất về backup, khôi phục, RPO/RTO, runbook RB-xx và kiểm thử khôi phục. Alert backup định nghĩa ở [MONITORING](MONITORING.md); retention dữ liệu nghiệp vụ → [DATA_PRIVACY](../10-security/DATA_PRIVACY.md).

## 1. Mục tiêu

| Mã | Mục tiêu | Giá trị | Nhãn |
|---|---|---|---|
| NFR-AVAIL-002 | RPO dữ liệu học tập (PostgreSQL, **áp dụng cho từng database service**) | ≤ 5 phút | [Proposed] |
| NFR-AVAIL-002 | RTO dữ liệu học tập | ≤ 4 giờ | [Proposed] |
| — | RPO Keycloak | ≤ 24 giờ cho cấu hình (cấu hình cũng có trong Git); ≤ 5 phút nếu Keycloak dùng chung cluster PG có PITR | [Proposed] |
| — | RPO media/object storage | ≤ 24 giờ | [Proposed] |
| — | Valkey | Không có RPO nghiệp vụ (ADR-0004) | [Confirmed] |
| — | NATS JetStream | Không backup; PostgreSQL là nguồn sự thật (ADR-0035, ADR-0042). Mất toàn bộ cluster NATS chỉ mất tin chat đã ack nhưng chưa persist (thường < 1 giây tin); domain event vẫn còn trong outbox 30 ngày để relay lại | [Proposed] |

## 2. Phân loại dữ liệu cần bảo vệ

| Dữ liệu | Nơi lưu | Mức độ quan trọng | Chiến lược |
|---|---|---|---|
| Learning records, attempts, exam results, mastery, nội dung học tập | Database `eduworld_learning` (learning-service) | Cao nhất | PITR + full backup + bản sao off-site |
| Tenant/School/Student/Enrollment, External identity, permission, audit, settings | Database `eduworld_identity` (identity-service) | Cao (tái đồng bộ được một phần từ SIS) | PITR |
| Game profile, quest progress, inventory, XP, game session, login events, cấu hình Game Studio | Database `eduworld_game` (game-service) + object storage | Cao | PITR + versioning object storage |
| Party, match results, ratings, tournaments, leaderboard snapshot | Database `eduworld_competition` (competition-service) | Trung bình–cao | PITR |
| Chat, friend, moderation, danh mục từ nhạy cảm | Database `eduworld_chat` (chat-service) | Cao về pháp lý; chịu retention và purge (ADR-0033) | PITR; ẩn danh hóa theo retention được áp lại sau restore (xem §7) |
| Projection `ref_*` trong mỗi database | Database của service nhận | Thấp, tái tạo được từ event/API của service nguồn | Không cần khôi phục riêng; rebuild bằng lệnh `<svc> projection rebuild` |
| Keycloak (realm, users, sessions) | DB `keycloak` + realm config trong Git | Cao | Backup DB + export cấu hình (không secret) |
| Media học tập, asset game | Object storage | Trung bình | Versioning + replication sang bucket/khu vực khác |
| Presence, queue, lobby, ticket, cache | Valkey | Thấp — tái tạo được | Không backup nghiệp vụ |
| Cấu hình APISIX | Git (declarative) + etcd | Thấp | Git là nguồn sự thật; snapshot etcd hằng ngày cho khôi phục nhanh |
| Secrets | Secret manager | Cao | Backup theo cơ chế của secret manager, tách khóa |

## 3. PostgreSQL

### 3.1. Công cụ
- **pgBackRest** (khuyến nghị) hoặc WAL-G [Proposed]; với managed PostgreSQL dùng PITR của nhà cung cấp + export logic bổ sung.
- Pilot: một stanza pgBackRest cho cluster chứa 5 database service + `keycloak`; khôi phục **một database** bằng `--db-include=<database>` vào cluster tạm. Khi tách cluster (mở rộng), mỗi cluster một stanza.
- Repository backup trên object storage S3-compatible **khác** hạ tầng chính (khác tài khoản/khác site), bật object lock/immutability 14 ngày để chống ransomware.

### 3.2. Lịch

| Loại | Tần suất | Giữ lại | Ghi chú |
|---|---|---|---|
| WAL archiving liên tục | `archive_timeout = 60s` | 14 ngày | Bảo đảm RPO ≤ 5 phút |
| Full backup | Chủ nhật 01:00 ICT | 4 bản (4 tuần) | Ngoài giờ học |
| Differential | Hằng ngày 01:00 ICT (trừ Chủ nhật) | 14 ngày | |
| Snapshot cuối học kỳ | Cuối mỗi học kỳ | 2 năm [TBD] OQ-12 | Lưu trữ lạnh, mã hóa |
| Logical dump (`pg_dump -Fc` theo **database service**) | Hằng tuần | 4 bản | Khôi phục chọn lọc một database/bảng |

### 3.3. Mã hóa và quyền
- Mã hóa phía client (pgBackRest `repo-cipher-type=aes-256-cbc`) với passphrase trong secret manager; TLS khi truyền.
- Credential ghi backup chỉ có quyền ghi vào repo backup; credential đọc/khôi phục tách riêng, chỉ cấp khi chạy runbook.
- Truy cập bản backup chứa dữ liệu học sinh được audit (ai, khi nào, mục đích).

### 3.4. Giám sát
- Metric `eduworld_backup_last_success_timestamp{target}` + alert `BackupMissing` (> 26h) và `BackupFailed` ([MONITORING](MONITORING.md) §5).
- `pgbackrest check` hằng ngày; cảnh báo khi WAL archive bị kẹt (`pg_stat_archiver.failed_count` tăng).

### 3.5. Nhất quán liên service sau khôi phục (ADR-0036, ADR-0037)

Mỗi service có database riêng nên khôi phục một database về thời điểm T làm nó **lệch** so với các service khác (vẫn ở hiện tại). Nguyên tắc xử lý:

| Tình huống | Cách đưa về nhất quán |
|---|---|
| Service khôi phục **mất event đã phát** sau T (ví dụ learning-service mất attempt sau T, nhưng game-service đã cộng XP) | Dữ liệu ở service khác vẫn giữ (không rollback dây chuyền). Đối soát bằng job `reconcile-<service>`: so `processed_events` ở service nhận với bản ghi nguồn, tạo danh sách lệch để xử lý thủ công hoặc bù trừ (ví dụ thu hồi XP qua bản ghi bù, không xóa) |
| Service khôi phục **chưa nhận event** sau T từ service khác | Reset consumer durable của service đó về thời điểm T (`deliver_policy=by_start_time`) trên stream `EVT_*` (giữ 7 ngày); nếu quá 7 ngày, relay lại từ `outbox_events` của service nguồn (giữ 30 ngày). Consumer idempotent nhờ `processed_events` |
| Projection `ref_*` cũ | Rebuild projection từ API internal của service nguồn (`<svc> projection rebuild --source=identity`) |
| Saga đang chạy dở tại T | Saga lưu trạng thái trong `eduworld_identity`; sau khôi phục chạy `saga resume`: bước đã xong ở service đích được nhận diện qua idempotency key, bước chưa xong chạy lại |
| Chat (JetStream còn tin sau T) | Như §5b: reset consumer `persister` về T để ghi bù |

Khôi phục toàn bộ hệ thống về cùng thời điểm T chỉ dùng khi thảm họa (RB-03): khôi phục mọi database về cùng T rồi reset mọi consumer về T.

## 4. Keycloak

- DB `keycloak`: nằm trong cùng chính sách PITR với PostgreSQL (DB riêng, cùng cluster ở pilot) hoặc backup riêng cùng lịch.
- Cấu hình realm: nguồn sự thật là `deploy/keycloak/realm-eduworld.json` trong Git (không có user/secret). Sau sự cố mất toàn bộ, có thể dựng lại realm từ Git, user học sinh được **re-provision** từ SIS sync (ADR-0021); liên kết IdP brokering được tạo lại khi đăng nhập.
- Theme/provider: nằm trong image build, không cần backup riêng.
- Lưu ý: khôi phục DB Keycloak về thời điểm cũ có thể làm sống lại phiên/refresh token đã thu hồi → sau khôi phục **bắt buộc** đổi realm keys (rotate) và đặt `not-before` cho realm để vô hiệu toàn bộ token cũ.

## 5. Valkey

- Không backup nghiệp vụ (ADR-0004). AOF/RDB chỉ để khởi động lại nhanh.
- Sau khi mất Valkey: presence tái lập khi client reconnect; matchmaking ticket mất → client tạo lại; party realtime state được dựng lại từ `multiplayer.parties`/`party_members` trong PostgreSQL; leaderboard cache dựng lại bởi worker job `leaderboard-rebuild`; rate-limit counter reset (chấp nhận).
- Runbook kiểm thử degraded mode định kỳ ([TEST_STRATEGY](../12-project/TEST_STRATEGY.md)).

## 5b. NATS JetStream

- Không backup (ADR-0035, ADR-0042): stream `CHAT_MESSAGES` là bộ đệm 72 h trước khi `chat-persister` ghi PostgreSQL; stream `EVT_*` là bản phát của outbox (outbox trong database mỗi service giữ 30 ngày, relay lại được).
- Mất 1 node: cluster R3 tiếp tục. Mất toàn bộ: dựng lại cluster từ cấu hình trong Git, worker tự tạo lại stream/consumer; tin đã ack nhưng chưa persist bị mất (chấp nhận, đo bằng `eduworld_chat_persist_lag_seconds`).
- Khôi phục PostgreSQL bằng PITR về thời điểm T: tin chat sau T còn trong JetStream (≤ 72 h) có thể được persister **ghi bù** bằng cách reset consumer `persister` về thời điểm T (`nats consumer` với `deliver_policy=by_start_time`); ghi idempotent theo `message_id` [Proposed].

## 6. Object storage

- Bucket theo ADR-0044: `ew-media-public`, `ew-media-private`, `ew-game-bundles` bật **versioning** (và object lock ở chế độ governance cho `ew-media-public` [Proposed]); `ew-media-quarantine` không backup (file tạm, lifecycle 7 ngày); `ew-exports` theo retention của DATA_PRIVACY. Bucket backup riêng theo §3.2.
- Object media và snapshot nội dung có khóa content-addressed (`media/<sha256>/...`, snapshot theo version + hash) nên **không bao giờ bị ghi đè**; noncurrent version chỉ phát sinh khi lỗi vận hành, giữ 30 ngày.
- Asset còn được bất kỳ content version nào tham chiếu không được xóa (ADR-0044 mục 4); job đối soát hằng tuần so `media_assets` (DB của service sở hữu) với object storage, báo object thiếu/mồ côi.
- Snapshot nội dung (ADR-0043) có thể **dựng lại** từ PostgreSQL (version bất biến) nếu mất: chạy lại `content-snapshot-builder` cho các version đang được tham chiếu, rồi dựng lại manifest các tenant.
- **Backup floci (ADR-0071):** object storage chạy trên floci một node, nên không có replication sẵn. Đồng bộ volume dữ liệu floci (hoặc `aws s3 sync` từng bucket trừ `ew-media-quarantine`) sang vị trí lưu trữ **khác máy** (ưu tiên khác site, trong lãnh thổ Việt Nam nếu OQ-12 yêu cầu) ít nhất **hằng ngày** [Proposed]; giữ 30 bản ngày + 12 bản tháng [Proposed]; mã hóa khi lưu; kiểm tra checksum khóa `media/<sha256>/…` sau mỗi lần đồng bộ.
- Versioning, object lock, SSE nêu trên chỉ áp dụng nếu floci hỗ trợ; kiểm chứng trên staging, tính năng không có thì dựa vào backup ngoài máy [TBD].
- Asset game đặt theo hash nội dung, bất biến → dễ khôi phục và kiểm tra toàn vẹn.

## 7. Retention và xóa dữ liệu trong backup

- Ứng dụng chỉ soft-delete và ẩn danh hóa (ADR-0028). Khi dữ liệu bị ẩn danh theo chính sách (vd. chat quá 180 ngày, tài khoản có yêu cầu xóa hợp lệ), bản gốc trong backup **tự hết hạn** theo vòng đời backup (tối đa 14 ngày PITR + snapshot học kỳ). Dữ liệu soft-delete vẫn có trong backup và được khôi phục cùng `deleted_at` (vẫn ẩn sau restore).
- Snapshot học kỳ dài hạn: [TBD] OQ-12 — nếu pháp luật yêu cầu xóa triệt để, áp dụng loại trừ schema `chat` khỏi snapshot dài hạn hoặc ghi danh sách "tombstone" để áp lại việc xóa sau khi khôi phục.
- Sau mọi lần khôi phục, chạy job `retention-reapply` để **ẩn danh hóa lại** dữ liệu đã quá hạn hoặc đã có yêu cầu xóa (identity-service đọc `deletion_requests`, `erasure_log` trong `eduworld_identity` rồi gọi `/internal/v1/platform/...` của từng service theo saga). Job không chạy `DELETE`; Release 1.0 không có `physical_erase` (ADR-0070).
- **Ngoại lệ ADR-0033:** chat gameplay (`chat.gameplay_messages`, channel `match`/`party`/`team` theo trận, `chat.ephemeral_messages`) bị purge vật lý bởi job `gameplay-chat-purge` sau thời hạn (mặc định 7 ngày / 24 h). Backup không cần giữ riêng dữ liệu này; sau khi khôi phục, job purge chạy lại sẽ xóa các partition quá hạn vừa được khôi phục (không cần `retention-reapply`). Bằng chứng moderation đã nằm trong `chat.moderation_evidence` nên vẫn còn.
- Học sinh bị dọn (ADR-0032) chỉ là soft-delete nên khôi phục backup không làm sống lại sai: trạng thái `deleted_at` được khôi phục cùng dữ liệu; nếu backup cũ hơn lần dọn, job `student-inactive-cleanup` chạy lại sẽ dọn tiếp theo cùng quy tắc.
- Release 1.0 không gỡ partition hết hạn và không xóa vật lý (ADR-0028, ADR-0070); ngoại lệ duy nhất là ADR-0033 ở trên.

## 8. Kiểm thử khôi phục định kỳ

| Kiểm thử | Tần suất | Tiêu chí đạt |
|---|---|---|
| Restore full + PITR về thời điểm ngẫu nhiên trong 7 ngày vào môi trường cô lập | Hằng tháng | Hoàn thành ≤ 2 giờ; đối soát số lượng attempt đã submit trong `eduworld_learning` khớp với metric thời điểm đó; app khởi động và đọc được |
| Restore **một database service** (PITR) vào cluster tạm rồi thay thế | Hằng quý (luân phiên service) | Các service khác chạy bình thường; consumer được reset, projection rebuild, job `reconcile-<service>` báo 0 lệch không giải thích được |
| Failover PostgreSQL (Patroni switchover) | Hằng quý, ngoài giờ học | Gián đoạn ghi < 60s; không mất giao dịch đã commit |
| Dựng lại Keycloak từ Git + DB backup | 6 tháng | Đăng nhập được bằng tài khoản demo staging |
| Mất Valkey (dừng hẳn) trên staging | Hằng quý | REST hoạt động; realtime tự phục hồi < 5 phút sau khi Valkey trở lại |
| Mất NATS (dừng cả cluster) trên staging | Hằng quý | Chat báo tạm gián đoạn, các tính năng khác chạy; sau khi NATS trở lại, gửi/nhận và persist bình thường, không trùng tin |
| Khôi phục object storage floci từ backup ngoài máy sang node mới (ADR-0071, RB-05) | Hằng quý | Hoàn thành ≤ 4 giờ; checksum khớp 100% object được `media_assets` tham chiếu; game và portal tải được media qua CDN |
| Diễn tập thảm họa toàn site (DR drill) | Hằng năm | Đạt RTO ≤ 4h trên site dự phòng |

Kết quả mỗi lần kiểm thử ghi vào `docs/11-operations/restore-tests/<YYYY-MM>.md` (thời gian, người thực hiện, kết quả, vấn đề).

## 9. Runbook khôi phục

### RB-01 — Lỗi logic làm hỏng dữ liệu (vd. migration/bug ghi sai) — khôi phục PITR

1. **Khoanh vùng**: xác định thời điểm sự cố `T_incident` từ log/audit, **service và database** bị ảnh hưởng, phạm vi bảng/trường.
2. **Đóng băng ghi** nếu lỗi còn tiếp diễn: tắt route ghi của service bị ảnh hưởng ở APISIX (hoặc bật maintenance toàn hệ thống nếu cần), dừng worker/consumer của service đó.
3. Thông báo kênh sự cố; chỉ định Incident Commander.
4. Khôi phục PITR về `T_incident − 1 phút` vào **cluster tạm** (không đè production):
   ```bash
   pgbackrest --stanza=eduworld --db-include=eduworld_learning --type=time --target="2026-11-20 08:14:00+07" \
     --pg1-path=/var/lib/postgresql/restore restore
   ```
5. Xác minh dữ liệu trên cluster tạm.
6. Chọn chiến lược:
   - **Phạm vi nhỏ**: trích xuất bản ghi đúng từ cluster tạm (`COPY`/`pg_dump -t`) → áp vào production bằng script đã review, trong transaction, có audit.
   - **Phạm vi toàn cục**: promote cluster tạm thành primary mới (dữ liệu sau `T_incident` bị mất → phải chấp nhận và thông báo), cập nhật DSN/PgBouncer.
7. Chạy `retention-reapply`; relay lại outbox của service đã khôi phục; reset consumer của service đó về T trên `EVT_*`; rebuild projection `ref_*`; chạy `reconcile-<service>` để đối soát với các service khác (§3.5).
8. Gỡ maintenance; theo dõi dashboard 1 giờ.
9. Post-mortem trong 5 ngày làm việc.

### RB-02 — Mất primary PostgreSQL (phần cứng)

1. Patroni tự failover sang standby; xác nhận primary mới và `pg_replication_lag`.
2. Nếu failover tự động không chạy: `patronictl failover --candidate db-2`.
3. Kiểm tra app `/readyz`, error rate.
4. Dựng lại node cũ thành standby (`patronictl reinit`).
5. Kiểm tra WAL archiving tiếp tục từ primary mới.

### RB-03 — Mất toàn bộ cluster PostgreSQL

1. Cấp hạ tầng mới (Infrastructure as Code).
2. `pgbackrest restore` bản full gần nhất + WAL đến thời điểm cuối cùng có thể (`--type=default`).
3. Khởi động, kiểm tra `pg_is_in_recovery()=false`, chạy kiểm tra toàn vẹn (đếm bản ghi, checksum).
4. Cập nhật DSN, khởi động lại 6 service và worker theo thứ tự: identity → learning → game → competition → chat → realtime-gateway; relay outbox mỗi service tiếp tục; reset mọi consumer về thời điểm khôi phục (§3.5).
5. Rotate credential DB (giả định khả năng bị xâm phạm nếu nguyên nhân chưa rõ).

### RB-04 — Khôi phục Keycloak

1. Khôi phục DB `keycloak` (PITR hoặc bản gần nhất).
2. Áp lại cấu hình từ Git bằng `keycloak-config-cli`.
3. **Rotate realm keys** + đặt `not-before` → buộc mọi client đăng nhập lại.
4. Kiểm tra brokering IdP, đăng nhập thử theo từng loại tài khoản.
5. Nếu DB Keycloak mất hoàn toàn: dựng realm từ Git → chạy SIS sync full để provisioning lại user → ExternalIdentity trong EduWorld vẫn dùng `(issuer, subject)`; **lưu ý** `subject` mới khác cũ → chạy job `identity-relink` ghép theo khóa SIS (`student_code` + `school_id`) đã được thiết kế trong [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md).

### RB-05 — Mất object storage chính (floci, ADR-0071)

1. Dựng floci mới trên node `storage` dự phòng (cùng image digest, cùng Docker secret, cùng tên service nội bộ nên các service không đổi cấu hình).
2. Khôi phục volume/bucket từ bản backup ngoài máy gần nhất (§6).
3. Kiểm tra checksum theo khóa `media/<sha256>/…`; job đối soát `media_assets` liệt kê object thiếu; dựng lại snapshot/manifest nội dung thiếu từ PostgreSQL (§6).
4. Media tải lên sau thời điểm backup (trong RPO 24 giờ) phải tải lên lại: gửi danh sách cho Content Admin/trường.

## 10. Script backup đề xuất

Đặt tại `scripts/backup/` (Phase 1), chạy bởi service lịch trên node `ops` của Docker Swarm (container `ops-scheduler`, ADR-0024) hoặc systemd timer trên node database.

```bash
#!/usr/bin/env bash
# scripts/backup/pg_backup.sh — bao bọc pgBackRest, đẩy metric cho Prometheus (textfile collector)
set -euo pipefail

TYPE="${1:-diff}"                        # full | diff | incr
STANZA="${PGBACKREST_STANZA:-eduworld}"
METRIC_FILE="${METRIC_DIR:-/var/lib/node_exporter/textfile}/eduworld_backup.prom"

start=$(date +%s)
if pgbackrest --stanza="$STANZA" --type="$TYPE" backup; then
  status=1
else
  status=0
fi
end=$(date +%s)

cat > "${METRIC_FILE}.tmp" <<EOF
eduworld_backup_last_run_timestamp{target="postgres",type="$TYPE"} $end
eduworld_backup_last_status{target="postgres",type="$TYPE"} $status
eduworld_backup_duration_seconds{target="postgres",type="$TYPE"} $((end-start))
EOF
if [ "$status" -eq 1 ]; then
  echo "eduworld_backup_last_success_timestamp{target=\"postgres\"} $end" >> "${METRIC_FILE}.tmp"
fi
mv "${METRIC_FILE}.tmp" "$METRIC_FILE"

pgbackrest --stanza="$STANZA" check
[ "$status" -eq 1 ]
```

Cấu hình pgBackRest mẫu (không chứa secret — passphrase/khóa S3 lấy từ biến môi trường do secret manager inject):

```ini
[global]
repo1-type=s3
repo1-s3-bucket=eduworld-backups
repo1-s3-endpoint=<s3-endpoint>
repo1-s3-region=<region>
repo1-path=/pgbackrest
repo1-cipher-type=aes-256-cbc
repo1-retention-full=4
repo1-retention-diff=14
process-max=4
compress-type=zst
start-fast=y

[eduworld]
pg1-path=/var/lib/postgresql/17/main
```

Các script khác: `scripts/backup/keycloak_export.sh` (export realm cấu hình không secret để so sánh với Git), `scripts/backup/etcd_snapshot.sh`, `scripts/backup/restore_test.sh` (tự động hóa kiểm thử hằng tháng §8).

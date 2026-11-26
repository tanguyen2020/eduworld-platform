# Backup & Recovery

> Trạng thái: Draft · Phiên bản: 0.5 · Owner: DevOps Engineer · Cập nhật: 2026-11-27 (F130: script backup/khôi phục/runbook trong `scripts/backup/`, `scripts/ops/`, repo host pgBackRest + TLS server, metric có nhãn `type`, credential ghi/khôi phục tách, biên bản `restore-tests/`) · 2026-11-17 (một cụm PostgreSQL, khôi phục theo database module, thứ tự khởi động theo binary — ADR-0074; object storage staging/production theo ADR-0075, thay ADR-0071: bỏ floci một node, RPO bucket riêng tư bằng dữ liệu học tập)
> Liên quan: [DEPLOYMENT](DEPLOYMENT.md) · [MONITORING](MONITORING.md) · [ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · [ADR-0004, ADR-0037, ADR-0042](../adr/README.md) · [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [SYSTEM_ARCHITECTURE §7](../06-architecture/SYSTEM_ARCHITECTURE.md#7-non-functional-baseline)

**Mục đích & phạm vi.** Nguồn duy nhất về backup, khôi phục, RPO/RTO, runbook RB-xx và kiểm thử khôi phục. Alert backup định nghĩa ở [MONITORING](MONITORING.md); retention dữ liệu nghiệp vụ → [DATA_PRIVACY](../10-security/DATA_PRIVACY.md).

## 1. Mục tiêu

| Mã | Mục tiêu | Giá trị | Nhãn |
|---|---|---|---|
| NFR-AVAIL-002 | RPO dữ liệu học tập (PostgreSQL, **áp dụng cho từng database module**) | ≤ 5 phút | [Confirmed] |
| NFR-AVAIL-002 | RTO dữ liệu học tập | ≤ 4 giờ | [Confirmed] |
| — | RPO Keycloak | ≤ 24 giờ cho cấu hình (cấu hình cũng có trong Git); ≤ 5 phút khi Keycloak nằm trong cụm PostgreSQL chung có PITR (mặc định, ADR-0074 mục 4) | [Confirmed] |
| — | RPO media/object storage | Bucket riêng tư (`ew-media-private`; `ew-student-submissions` nếu sau này có): **bằng dữ liệu học tập** (≤ 5 phút, [ADR-0075](../adr/ADR-0075-object-storage-production-khong-dung-floci.md)); media công khai, asset game: ≤ 24 giờ (dựng lại được từ PostgreSQL + nguồn CI) | [Confirmed — Technical Architect 2026-11-17] nguyên tắc · [Confirmed] số |
| — | Valkey | Không có RPO nghiệp vụ (ADR-0004) | [Confirmed] |
| — | NATS JetStream | Không backup; PostgreSQL là nguồn sự thật (ADR-0035, ADR-0042). Mất toàn bộ cluster NATS chỉ mất tin chat đã ack nhưng chưa persist (thường < 1 giây tin); domain event vẫn còn trong outbox 30 ngày để relay lại | [Confirmed] |

## 2. Phân loại dữ liệu cần bảo vệ

| Dữ liệu | Nơi lưu | Mức độ quan trọng | Chiến lược |
|---|---|---|---|
| Learning records, attempts, exam results, mastery, nội dung học tập | Database `eduworld_learning` (module learning) | Cao nhất | PITR + full backup + bản sao off-site |
| Tenant/School/Student/Enrollment, External identity, permission, audit, settings | Database `eduworld_identity` (module identity) | Cao (tái đồng bộ được một phần từ SIS) | PITR |
| Game profile, quest progress, inventory, XP, game session, login events, cấu hình Game Studio | Database `eduworld_game` (module game) + object storage | Cao | PITR + versioning object storage |
| Party, match results, ratings, tournaments, leaderboard snapshot | Database `eduworld_competition` (module competition) | Trung bình–cao | PITR |
| Chat, friend, moderation, danh mục từ nhạy cảm | Database `eduworld_chat` (module chat) | Cao về pháp lý; chịu retention và purge (ADR-0033) | PITR; ẩn danh hóa theo retention được áp lại sau restore (xem §7) |
| Projection `ref_*` trong mỗi database | Database của module nhận | Thấp, tái tạo được từ event hoặc facade của module nguồn | Không cần khôi phục riêng; rebuild bằng lệnh `worker projection rebuild --module=<module>` |
| Keycloak (realm, users, sessions) | DB `keycloak` + realm config trong Git | Cao | Backup DB + export cấu hình (không secret) |
| Media học tập, asset game | Object storage | Trung bình | Versioning + replication sang bucket/khu vực khác |
| Presence, queue, lobby, ticket, cache | Valkey | Thấp — tái tạo được | Không backup nghiệp vụ |
| Cấu hình APISIX | Git (declarative) + etcd | Thấp | Git là nguồn sự thật; snapshot etcd hằng ngày cho khôi phục nhanh |
| Secrets | Secret manager | Cao | Backup theo cơ chế của secret manager, tách khóa |

## 3. PostgreSQL

### 3.1. Công cụ
- **pgBackRest** (khuyến nghị) hoặc WAL-G [Confirmed]; với managed PostgreSQL dùng PITR của nhà cung cấp + export logic bổ sung.
- Mọi môi trường: **một cụm** PostgreSQL chứa 5 database module + `keycloak` (ADR-0074 mục 4), một stanza pgBackRest cho cả cụm; khôi phục **một database** bằng `--db-include=<database>` vào cụm tạm. Chỉ khi một database được chuyển sang cụm riêng (ADR-0074 mục 7) mới thêm stanza cho cụm đó.
- Repository backup trên object storage S3-compatible **khác** hạ tầng chính (khác tài khoản/khác site), bật object lock/immutability 14 ngày để chống ransomware.
- Triển khai (F130, research R2): repo host là service `backup-tools` (image `eduworld/backup-tools`, node `ops`, cấu hình `deploy/backup/pgbackrest.conf` — staging `pgbackrest-staging.conf`); cạnh mỗi node PostgreSQL có service `pgbackrest-N` chạy `pgbackrest server` (TLS, chỉ chứng chỉ client CN `backup-tools` được thao tác stanza `eduworld`, `deploy/backup/pgbackrest-server.conf`); PostgreSQL đẩy WAL bằng `archive_command` (`deploy/backup/pgbackrest-db.conf`). Image `eduworld/postgres-patroni` và `eduworld/backup-tools` dùng chung bản PostgreSQL 17 + pgBackRest.

### 3.2. Lịch

| Loại | Tần suất | Giữ lại | Ghi chú |
|---|---|---|---|
| WAL archiving liên tục | `archive_timeout = 60s` | 14 ngày | Bảo đảm RPO ≤ 5 phút |
| Full backup | Chủ nhật 01:00 ICT | 4 bản (4 tuần) | Ngoài giờ học |
| Differential | Hằng ngày 01:00 ICT (trừ Chủ nhật) | 14 ngày | |
| Snapshot cuối học kỳ | Cuối mỗi học kỳ | 2 năm [Confirmed] | Lưu trữ lạnh, mã hóa |
| Logical dump (`pg_dump -Fc` theo **database module**) | Hằng tuần | 4 bản | Khôi phục chọn lọc một database/bảng |

Ánh xạ vào pgBackRest (F130, `deploy/backup/pgbackrest.conf`): `repo1-retention-full=4`, `repo1-retention-diff=12` (≈ 14 ngày diff hằng ngày), `repo1-retention-archive-type=full` + `repo1-retention-archive=3` (WAL của 3 full gần nhất ≥ 14 ngày PITR). Lịch chạy: `deploy/swarm/<env>/ops-jobs.crontab` (supercronic trong `backup-tools`, giờ ICT). Logical dump, snapshot etcd và export realm lưu mã hóa trong bucket repo (`logical/`, `etcd/`, `keycloak/`); số bản giữ là lifecycle của prefix trên bucket repo — script không xóa. Snapshot cuối học kỳ: `pg_backup.sh full --annotation=semester` rồi sao sang lưu trữ lạnh — quy trình sao lạnh [TBD] khi chọn nhà cung cấp.

### 3.3. Mã hóa và quyền
- Mã hóa phía client (pgBackRest `repo-cipher-type=aes-256-cbc`) với passphrase trong secret manager; TLS khi truyền.
- Credential ghi backup chỉ có quyền ghi vào repo backup; credential đọc/khôi phục tách riêng, chỉ cấp khi chạy runbook. F130: mẫu chính sách S3 `deploy/backup/s3-policies/backup-writer.json` (PutObject + đọc metadata pgBackRest, không DeleteObject) và `backup-restore.json` (đọc toàn bộ); credential ghi (`backup_repo_write_*`) mount vào `pg-*` và `backup-tools`, credential khôi phục (`backup_restore_*`) không bao giờ nằm trong stack — `scripts/ops/run-ops-job.sh --restore` gắn vào job một lần; mỗi lần khôi phục ghi người thực hiện, thời điểm, mục đích (dòng audit JSON + biên bản).
- Truy cập bản backup chứa dữ liệu học sinh được audit (ai, khi nào, mục đích).

### 3.4. Giám sát
- Metric `eduworld_backup_last_run_timestamp`, `eduworld_backup_last_status` (1/0), `eduworld_backup_duration_seconds`, `eduworld_backup_last_success_timestamp` với nhãn `target` (`postgres`, `object_storage`, `etcd`, `keycloak_export`) và `type` (`full`, `diff`, `check`, `logical`, `offsite`, `replication`, `snapshot`, `realm`, `restore_full`, `restore_module`) — mỗi job một file textfile `eduworld_backup_<target>_<type>.prom` (thư mục của node_exporter trên node `ops`), ghi nguyên tử, lần lỗi giữ `last_success_timestamp` cũ; alert `BackupMissing` (> 26h, gộp `max by (target)`) và `BackupFailed` ([MONITORING](MONITORING.md) §5, F131). Độ trễ replication bucket riêng tư: `eduworld_object_storage_replication_lag_seconds{bucket}`.
- `pgbackrest check` hằng ngày (sau mỗi backup và riêng `pg_check.sh` lúc 12:00 ICT, `type="check"`); cảnh báo khi WAL archive bị kẹt (`pg_stat_archiver.failed_count` tăng).

### 3.5. Nhất quán liên module sau khôi phục (ADR-0037, ADR-0074)

Mỗi module có database riêng (dù chung một cụm) nên khôi phục một database về thời điểm T làm nó **lệch** so với các module khác (vẫn ở hiện tại). Nguyên tắc xử lý:

| Tình huống | Cách đưa về nhất quán |
|---|---|
| Module khôi phục **mất event đã phát** sau T (ví dụ module learning mất attempt sau T, nhưng module game đã cộng XP) | Dữ liệu ở module khác vẫn giữ (không rollback dây chuyền). Đối soát bằng job `reconcile-<module>`: so `processed_events` ở module nhận với bản ghi nguồn, tạo danh sách lệch để xử lý thủ công hoặc bù trừ (ví dụ thu hồi XP qua bản ghi bù, không xóa) |
| Module khôi phục **chưa nhận event** sau T từ module khác | Reset consumer durable của module đó về thời điểm T (`deliver_policy=by_start_time`) trên stream `EVT_*` (giữ 7 ngày); nếu quá 7 ngày, relay lại từ `outbox_events` của module nguồn (giữ 30 ngày). Consumer idempotent nhờ `processed_events` |
| Projection `ref_*` cũ | Rebuild projection qua facade của module nguồn, gọi trong tiến trình (`worker projection rebuild --module=<module> --source=identity`) |
| Saga đang chạy dở tại T | Saga lưu trạng thái trong `eduworld_identity`; sau khôi phục chạy `saga resume`: bước đã xong ở module đích được nhận diện qua idempotency key, bước chưa xong chạy lại |
| Chat (JetStream còn tin sau T) | Như §5b: reset consumer `persister` về T để ghi bù |

Khôi phục toàn bộ hệ thống về cùng thời điểm T chỉ dùng khi thảm họa (RB-03): khôi phục mọi database về cùng T rồi reset mọi consumer về T.

## 4. Keycloak

- DB `keycloak`: nằm trong cùng chính sách PITR với PostgreSQL (database riêng trong cụm chung) hoặc backup riêng cùng lịch.
- Cấu hình realm: nguồn sự thật là `deploy/keycloak/realm-eduworld.json` trong Git (không có user/secret). Sau sự cố mất toàn bộ, có thể dựng lại realm từ Git, user học sinh được **re-provision** từ SIS sync (ADR-0021); liên kết IdP brokering được tạo lại khi đăng nhập.
- Theme/provider: nằm trong image build, không cần backup riêng.
- Lưu ý: khôi phục DB Keycloak về thời điểm cũ có thể làm sống lại phiên/refresh token đã thu hồi → sau khôi phục **bắt buộc** đổi realm keys (rotate) và đặt `not-before` cho realm để vô hiệu toàn bộ token cũ.

## 5. Valkey

- Không backup nghiệp vụ (ADR-0004). AOF/RDB chỉ để khởi động lại nhanh.
- Sau khi mất Valkey: presence tái lập khi client reconnect; matchmaking ticket mất → client tạo lại; party realtime state được dựng lại từ `multiplayer.parties`/`party_members` trong PostgreSQL; leaderboard cache dựng lại bởi worker job `leaderboard-rebuild`; rate-limit counter reset (chấp nhận).
- Runbook kiểm thử degraded mode định kỳ ([TEST_STRATEGY](../12-project/TEST_STRATEGY.md)).

## 5b. NATS JetStream

- Không backup (ADR-0035, ADR-0042): stream `CHAT_MESSAGES` là bộ đệm 72 h trước khi `chat-persister` ghi PostgreSQL; stream `EVT_*` là bản phát của outbox (outbox trong database mỗi module giữ 30 ngày, relay lại được).
- Mất 1 node: cluster R3 tiếp tục. Mất toàn bộ: dựng lại cluster từ cấu hình trong Git, worker tự tạo lại stream/consumer; tin đã ack nhưng chưa persist bị mất (chấp nhận, đo bằng `eduworld_chat_persist_lag_seconds`).
- Khôi phục PostgreSQL bằng PITR về thời điểm T: tin chat sau T còn trong JetStream (≤ 72 h) có thể được persister **ghi bù** bằng cách reset consumer `persister` về thời điểm T (`nats consumer` với `deliver_policy=by_start_time`); ghi idempotent theo `message_id` [Confirmed].

## 6. Object storage

- Bucket theo ADR-0044: `ew-media-public`, `ew-media-private`, `ew-game-bundles` bật **versioning** (và object lock ở chế độ governance cho `ew-media-public` [Confirmed]); `ew-media-quarantine` không backup (file tạm, lifecycle 7 ngày); `ew-exports` theo retention của DATA_PRIVACY. Bucket backup riêng theo §3.2.
- Object media và snapshot nội dung có khóa content-addressed (`media/<sha256>/...`, snapshot theo version + hash) nên **không bao giờ bị ghi đè**; noncurrent version chỉ phát sinh khi lỗi vận hành, giữ 30 ngày.
- Asset còn được bất kỳ content version nào tham chiếu không được xóa (ADR-0044 mục 4); job đối soát hằng tuần so `media_assets` (DB của module sở hữu) với object storage, báo object thiếu/mồ côi.
- Snapshot nội dung (ADR-0043) có thể **dựng lại** từ PostgreSQL (version bất biến) nếu mất: chạy lại `content-snapshot-builder` cho các version đang được tham chiếu, rồi dựng lại manifest các tenant.
- **Staging/production không dùng floci** ([ADR-0075](../adr/ADR-0075-object-storage-production-khong-dung-floci.md), [Confirmed — Technical Architect 2026-11-17]; thay toàn phần ADR-0071). floci chỉ ở local/CI, không cần backup.
- **Backend: SeaweedFS tự host** [Confirmed — owner 2026-11-20, OQ-21] ([ADR-0093](../adr/ADR-0093-object-storage-tu-host-seaweedfs-truoc-chuyen-s3-managed-bang-cau-hinh.md), [DEPLOYMENT §9b](DEPLOYMENT.md#9b-object-storage-và-media-pipeline-adr-0044-adr-0043)); chuyển S3 managed tại Việt Nam khi có ngân sách theo RB-05b:
  - *SeaweedFS tự host (hiện hành):* replication factor 3 trên 3 node `storage` (mất 1 node không mất dữ liệu, không gián đoạn); bản sao **ngoài cụm, khác site** (trong lãnh thổ Việt Nam nếu OQ-12 yêu cầu) hằng ngày, giữ 30 bản ngày + 12 bản tháng; mã hóa khi lưu. Nếu spike cho thấy không có object lock: bản sao ngoài site dùng credential chỉ ghi (không có quyền xóa) và retention theo chính sách trên.
  - *S3 managed tại Việt Nam (sau khi chuyển):* replication của nhà cung cấp (đa vùng lỗi) + versioning; bản sao **ngoài tài khoản/nhà cung cấp** hằng ngày cho mọi bucket trừ `ew-media-quarantine` (`rclone`/`aws s3 sync`), giữ 30 bản ngày + 12 bản tháng [Confirmed].
- **Bucket riêng tư** (`ew-media-private`; `ew-student-submissions` nếu sau này có): RPO bằng dữ liệu học tập — bật replication liên tục sang vị trí thứ hai (cross-region replication của dịch vụ managed hoặc replication bất đồng bộ sang cụm/site thứ hai) để độ trễ sao chép ≤ 5 phút; giám sát độ trễ replication ([MONITORING](MONITORING.md)).
- Versioning, object lock, SSE, lifecycle phải được backend hỗ trợ — là tiêu chí chọn trong spike (ADR-0075 mục 4); kiểm tra checksum khóa `media/<sha256>/…` sau mỗi lần đồng bộ.
- Triển khai (F130): `object_storage_setup.sh` bật/kiểm versioning 3 bucket (API S3 chuẩn, SigV4); `object_storage_backup.sh` hằng ngày `rclone sync` mọi bucket trừ `ew-media-quarantine` sang bản sao ngoài cụm mã hóa phía client (rclone crypt): `object-storage/current/<bucket>`, object bị thay/xóa ở nguồn chuyển vào `object-storage/history/<YYYY-MM-DD>/<bucket>` (không xóa), `rclone cryptcheck` toàn bucket, kiểm `sha256(media/<sha256>/original.*) = <sha256>` cho object mới chép, manifest `object-storage/manifests/<YYYY-MM-DD>/<bucket>.sha256`; giữ 30 bản ngày + 12 bản tháng = lifecycle của `history/` trên bucket ngoài cụm. Replication liên tục bucket riêng tư: SeaweedFS `filer.sync` sang site thứ hai (`storage-filer-sync` trong `data.stack.yml`); `object_storage_replication_check.sh` (mỗi 5 phút) đo độ trễ đầu-cuối bằng object canary `_replication-canary/` và báo lỗi khi vượt RPO (`EW_RPO_SECONDS`, mặc định 300 s = `rpo_rto`).
- Asset game đặt theo hash nội dung, bất biến → dễ khôi phục và kiểm tra toàn vẹn.

## 7. Retention và xóa dữ liệu trong backup

- Ứng dụng chỉ soft-delete và ẩn danh hóa (ADR-0028). Khi dữ liệu bị ẩn danh theo chính sách (vd. chat quá 180 ngày, tài khoản có yêu cầu xóa hợp lệ), bản gốc trong backup **tự hết hạn** theo vòng đời backup (tối đa 14 ngày PITR + snapshot học kỳ). Dữ liệu soft-delete vẫn có trong backup và được khôi phục cùng `deleted_at` (vẫn ẩn sau restore).
- Snapshot học kỳ dài hạn [Confirmed]: yêu cầu xóa theo luật ngoài version này (ADR-0082); khi mở lại, phương án là loại trừ schema `chat` khỏi snapshot dài hạn hoặc ghi danh sách "tombstone" để áp lại việc xóa sau khi khôi phục.
- Sau mọi lần khôi phục, chạy job `retention-reapply` để **ẩn danh hóa lại** dữ liệu đã quá hạn hoặc đã có yêu cầu xóa (module identity đọc `deletion_requests`, `erasure_log` trong `eduworld_identity` rồi gọi facade platform của từng module trong tiến trình theo saga). Job không chạy `DELETE`; Release 1.0 không có `physical_erase` (ADR-0070).
- **Ngoại lệ ADR-0033:** chat gameplay (`chat.gameplay_messages`, channel `match`/`party`/`team` theo trận, `chat.ephemeral_messages`) bị purge vật lý bởi job `gameplay-chat-purge` sau thời hạn (mặc định 7 ngày / 24 h). Backup không cần giữ riêng dữ liệu này; sau khi khôi phục, job purge chạy lại sẽ xóa các partition quá hạn vừa được khôi phục (không cần `retention-reapply`). Bằng chứng moderation đã nằm trong `chat.moderation_evidence` nên vẫn còn.
- Học sinh bị dọn (ADR-0032) chỉ là soft-delete nên khôi phục backup không làm sống lại sai: trạng thái `deleted_at` được khôi phục cùng dữ liệu; nếu backup cũ hơn lần dọn, job `student-inactive-cleanup` chạy lại sẽ dọn tiếp theo cùng quy tắc.
- Release 1.0 không gỡ partition hết hạn của dữ liệu nghiệp vụ và không xóa vật lý (ADR-0028, ADR-0070); ngoại lệ là ADR-0033 ở trên và bốn bảng hạ tầng `infra.outbox_events`, `infra.processed_events`, `infra.idempotency_records`, `infra.dead_letters` được job `infra-partition-maintenance` gỡ partition hết hạn (ADR-0089 §5); sau khôi phục, job chạy lại sẽ gỡ các partition quá hạn vừa được khôi phục.

## 8. Kiểm thử khôi phục định kỳ

| Kiểm thử | Tần suất | Tiêu chí đạt |
|---|---|---|
| Restore full + PITR về thời điểm ngẫu nhiên trong 7 ngày vào môi trường cô lập | Hằng tháng | Hoàn thành ≤ 2 giờ; đối soát số lượng attempt đã submit trong `eduworld_learning` khớp với metric thời điểm đó; app khởi động và đọc được |
| Restore **một database module** (PITR) vào cụm tạm rồi thay thế | Hằng quý (luân phiên module) | Các module khác chạy bình thường (route của module đang khôi phục trả `503`); consumer được reset, projection rebuild, job `reconcile-<module>` báo 0 lệch không giải thích được |
| Failover PostgreSQL (Patroni switchover) | Hằng quý, ngoài giờ học | Gián đoạn ghi < 60s; không mất giao dịch đã commit |
| Dựng lại Keycloak từ Git + DB backup | 6 tháng | Đăng nhập được bằng tài khoản demo staging |
| Mất Valkey (dừng hẳn) trên staging | Hằng quý | REST hoạt động; realtime tự phục hồi < 5 phút sau khi Valkey trở lại |
| Mất NATS (dừng cả cluster) trên staging | Hằng quý | Chat báo tạm gián đoạn, các tính năng khác chạy; sau khi NATS trở lại, gửi/nhận và persist bình thường, không trùng tin |
| Khôi phục object storage từ bản sao ngoài cụm/ngoài nhà cung cấp (ADR-0075, RB-05); với cụm tự host: thêm diễn tập mất 1 node `storage` | Hằng quý | Hoàn thành ≤ 4 giờ; checksum khớp 100% object được `media_assets` tham chiếu; game và portal tải được media qua CDN |
| Diễn tập thảm họa toàn site (DR drill) | Hằng năm | Đạt RTO ≤ 4h trên site dự phòng |

Kết quả mỗi lần kiểm thử ghi vào `docs/11-operations/restore-tests/<YYYY-MM>.md` (thời gian, người thực hiện, kết quả, vấn đề) — mẫu ở [restore-tests/README](restore-tests/README.md). Restore test tháng/quý: `make restore-test ENV=<env> MODE=full|module [DB=…]` (`scripts/backup/restore_test.sh`: T ngẫu nhiên trong 7 ngày, cụm tạm chỉ socket cục bộ, đối soát attempt đã submit với replica live, ngưỡng thời gian, tự ghi biên bản, xóa cụm tạm); switchover: `scripts/ops/patroni-switchover.sh`; RB-04: `rb04-keycloak-restore.sh`; RB-05: `rb05-object-storage-restore.sh`; DR: `rb03-cluster-restore.sh`.

## 9. Runbook khôi phục

### RB-01 — Lỗi logic làm hỏng dữ liệu (vd. migration/bug ghi sai) — khôi phục PITR

1. **Khoanh vùng**: xác định thời điểm sự cố `T_incident` từ log/audit, **module và database** bị ảnh hưởng, phạm vi bảng/trường.
2. **Đóng băng ghi** nếu lỗi còn tiếp diễn: tắt route ghi của module bị ảnh hưởng ở APISIX theo prefix (hoặc bật maintenance toàn hệ thống nếu cần), tạm dừng consumer/job của module đó trong các profile worker (cờ tắt theo module, không cần dừng cả binary).
3. Thông báo kênh sự cố; chỉ định Incident Commander.
4. Khôi phục PITR về `T_incident − 1 phút` vào **cụm tạm** (không đè production) — F130: `make ops-job ENV=<env> LIKE=backup-tools RESTORE=1 SECRET=ops_pg_admin_pgpass CMD="rb01-pitr-restore.sh --phase=restore --db=<database> --target=… --actor=… --purpose=…"` khôi phục vào cụm tạm rồi chép sang database mới `<database>_restored_<stamp>` trên cụm chính (cùng quyền database), không chạm database đang chạy; lệnh pgBackRest tương đương:
   ```bash
   pgbackrest --stanza=eduworld --db-include=eduworld_learning --type=time --target="2026-11-20 08:14:00+07" \
     --pg1-path=/var/lib/postgresql/restore restore
   ```
5. Xác minh dữ liệu trên cụm tạm.
6. Chọn chiến lược:
   - **Phạm vi nhỏ**: trích xuất bản ghi đúng từ cụm tạm (`COPY`/`pg_dump -t`) → áp vào production bằng script đã review, trong transaction, có audit.
   - **Phạm vi toàn cục**: thay database bị lỗi bằng bản đã khôi phục (`rb01-pitr-restore.sh --phase=swap`: chặn kết nối, đổi tên `<database>` → `<database>_pre_<stamp>` (giữ lại, không DROP) và `<database>_restored_<stamp>` → `<database>`; hoặc promote cụm tạm nếu mọi database cùng bị ảnh hưởng); dữ liệu sau `T_incident` bị mất → phải chấp nhận và thông báo; cập nhật DSN/PgBouncer nếu đổi cụm.
7. Chạy `retention-reapply`; relay lại outbox của module đã khôi phục; reset consumer của module đó về T trên `EVT_*` (`scripts/ops/reset-consumers.sh --module=<module> --since=<T UTC> --yes` trong job một lần có `ops_nats_creds`, trong lúc `worker-events`/`worker-chat` tạm về 0 replica; binary giữ vị trí bắt đầu đã reset khi khởi động lại — `pkg/outbox`; T cũ hơn `event_stream_retention` → mã thoát 3: relay từ outbox); rebuild projection `ref_*` (`worker projection rebuild`); chạy `reconcile-<module>` (`worker reconcile --module=<module>`, F134) để đối soát với các module khác (§3.5).
8. Gỡ maintenance; theo dõi dashboard 1 giờ.
9. Post-mortem trong 5 ngày làm việc.

### RB-02 — Mất primary PostgreSQL (phần cứng)

1. Patroni tự failover sang standby; xác nhận primary mới và `pg_replication_lag`. `pg-router` (HAProxy theo REST Patroni) chuyển `:5432` sang leader mới trong vài giây; PgBouncer và binary kết nối lại, không restart task. Diễn tập quý: `scripts/ops/patroni-switchover.sh --yes` (đo gián đoạn ghi < 60 s, kiểm không mất dòng đã commit).
2. Nếu failover tự động không chạy: `patronictl failover --candidate db-2`.
3. Kiểm tra app `/readyz`, error rate.
4. Dựng lại node cũ thành standby (`patronictl reinit`).
5. Kiểm tra WAL archiving tiếp tục từ primary mới.

### RB-03 — Mất toàn bộ cụm PostgreSQL

1. Cấp hạ tầng mới (Infrastructure as Code).
2. `pgbackrest restore` bản full gần nhất + WAL đến thời điểm cuối cùng có thể (`--type=default`).
3. Khởi động, kiểm tra `pg_is_in_recovery()=false`, chạy kiểm tra toàn vẹn (đếm bản ghi, checksum).
4. Cập nhật DSN, khởi động lại theo thứ tự (`scripts/ops/rb03-cluster-restore.sh` tự động hóa bước 1–5): `core-api` (cả 5 module) → `worker-events`, `worker-chat`, `worker-jobs` → `realtime`; relay outbox mỗi module tiếp tục; reset mọi consumer về thời điểm khôi phục (§3.5). Vì cả 5 database cùng một cụm, mất cụm làm dừng mọi module cùng lúc.
5. Rotate credential DB (giả định khả năng bị xâm phạm nếu nguyên nhân chưa rõ).

### RB-04 — Khôi phục Keycloak

1. Khôi phục DB `keycloak` (PITR hoặc bản gần nhất).
2. Áp lại cấu hình từ Git bằng `keycloak-config-cli`.
3. **Rotate realm keys** + đặt `not-before` → buộc mọi client đăng nhập lại (`scripts/ops/rb04-keycloak-restore.sh --phase=rotate --yes`: tạo key provider rsa/hmac/aes mới ưu tiên cao hơn, vô hiệu provider cũ, `notBefore` = bây giờ + push revocation, tùy chọn kiểm token cũ bị từ chối).
4. Kiểm tra brokering IdP, đăng nhập thử theo từng loại tài khoản.
5. Nếu DB Keycloak mất hoàn toàn: dựng realm từ Git → chạy SIS sync full để provisioning lại user → ExternalIdentity trong EduWorld vẫn dùng `(issuer, subject)`; **lưu ý** `subject` mới khác cũ → chạy job `identity-relink` ghép theo khóa SIS (`student_code` + `school_id`) đã được thiết kế trong [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md).

### RB-05 — Mất object storage chính (ADR-0075, ADR-0093)

1. **Mất 1 node `storage` (SeaweedFS):** cụm replication 3 bản vẫn phục vụ; thay node, chạy `volume.fix.replication` để SeaweedFS sao chép lại đủ bản; theo dõi tiến độ rebalance. **Dịch vụ managed gián đoạn:** theo SLA nhà cung cấp; nếu kéo dài, chuyển `EW_S3_ENDPOINT` sang vị trí replication thứ hai (đổi cấu hình, redeploy `core-api` và `worker`).
2. **Mất toàn bộ cụm/bucket:** dựng cụm hoặc bucket mới (cùng tên bucket, credential mới qua Docker secret), khôi phục từ vị trí replication hoặc bản sao ngoài cụm gần nhất (§6) — `scripts/ops/rb05-object-storage-restore.sh --referenced=<danh sách khóa media_assets> --yes` (bucket riêng tư từ replica, còn lại từ bản sao ngoài cụm; kiểm 100 % khóa được tham chiếu, liệt kê khóa thiếu).
3. Kiểm tra checksum theo khóa `media/<sha256>/…`; job đối soát `media_assets` liệt kê object thiếu; dựng lại snapshot/manifest nội dung thiếu từ PostgreSQL (§6).
4. Bucket riêng tư khôi phục từ replication liên tục (RPO ≤ 5 phút); media công khai tải lên sau bản sao gần nhất (RPO 24 giờ) phải tải lên lại: gửi danh sách cho Content Admin.

### RB-05b — Chuyển object storage từ SeaweedFS sang S3 managed (ADR-0093)

Chạy khi owner chốt chuyển (có ngân sách). Không sửa mã; chỉ đổi cấu hình và di chuyển dữ liệu.

1. Ở đích: tạo 6 bucket cùng tên, bucket policy, versioning, lifecycle (quarantine 7 ngày), replication bucket riêng tư; tạo credential mỗi binary; kiểm nhà cung cấp hỗ trợ URL path-style (nếu không, bật cấu hình virtual-hosted style trong adapter trước — ADR-0093 mục 3).
2. `rclone sync` tăng dần SeaweedFS → đích cho mọi bucket trừ `ew-media-quarantine`; kiểm checksum khóa `media/<sha256>/…`; lặp tới khi chênh lệch nhỏ.
3. Trong `maintenance_window`: chặn ghi (upload portal, export, import SIS) bằng thông báo bảo trì; sync lần cuối; đối soát `media_assets` với đích (0 object thiếu).
4. Đổi Docker secret `EW_S3_ENDPOINT`, `EW_S3_REGION`, `EW_S3_ACCESS_KEY`, `EW_S3_SECRET_KEY` và origin của CDN; redeploy `core-api`, `worker`; kiểm presigned PUT/GET, tải media qua CDN từ game và portal.
5. Giữ cụm SeaweedFS chỉ đọc 30 ngày làm đường lùi (rollback = đổi lại cấu hình); sau đó gỡ node `storage` và cập nhật DEPLOYMENT §4, CONFIG_DEFAULTS `object_storage_backend`, lịch sử ADR-0093.

## 10. Script backup và khôi phục

Triển khai F130 (image `eduworld/backup-tools`, service `backup-tools` của `ops.stack.yml`, lịch `deploy/swarm/<env>/ops-jobs.crontab` chạy bằng supercronic — không qua `ops-scheduler`, không quyền Docker API). Mọi script: `set -euo pipefail`, secret chỉ đọc từ Docker secret vào môi trường tiến trình (không argv/log), log JSON, metric textfile §3.4; không lệnh xóa dữ liệu nghiệp vụ (repo backup hết hạn theo retention pgBackRest/lifecycle bucket).

| Script | Việc | Lịch |
|---|---|---|
| `scripts/backup/pg_backup.sh full\|diff` | pgBackRest backup + `check` | CN 01:00 full, T2–T7 01:00 diff |
| `scripts/backup/pg_check.sh` | `pgbackrest check` độc lập | hằng ngày 12:00 |
| `scripts/backup/pg_logical_dump.sh` | `pg_dump -Fc` từng database module (role `backup_reader`, replica), mã hóa vào `logical/` | CN 03:00 |
| `scripts/backup/object_storage_backup.sh` | bản sao ngoài cụm mọi bucket trừ quarantine, cryptcheck, checksum khóa, manifest | hằng ngày 02:00 |
| `scripts/backup/object_storage_replication_check.sh` | độ trễ replication bucket riêng tư ≤ RPO | mỗi 5 phút |
| `scripts/backup/object_storage_setup.sh [--check-only]` | versioning `ew-media-public`, `ew-media-private`, `ew-game-bundles` | hằng ngày (kiểm) |
| `scripts/backup/etcd_snapshot.sh` | snapshot etcd APISIX | hằng ngày 02:30 |
| `scripts/backup/keycloak_export.sh` | partial-export realm, từ chối nếu có secret chưa che | hằng ngày 02:45 |
| `scripts/backup/restore_test.sh` | restore test §8 + biên bản | theo yêu cầu (`make restore-test`, credential khôi phục) |
| `scripts/ops/rb01-pitr-restore.sh`, `rb03-cluster-restore.sh`, `rb04-keycloak-restore.sh`, `rb05-object-storage-restore.sh`, `reset-consumers.sh`, `patroni-switchover.sh` | runbook RB-01…RB-05, diễn tập | theo runbook (`make ops-job`) |

Cấu hình pgBackRest (không chứa secret): `deploy/backup/pgbackrest.conf` (repo host), `pgbackrest-db.conf` (node PostgreSQL), `pgbackrest-server.conf` (TLS server). Test: `make test-ops` (logic, CLI giả lập) và `make test-ops-integration` (chạy thật trên container dùng một lần: backup full/diff, WAL, logical dump, PITR full + module, RB-01, bản sao object storage, replication, RB-05, Docker API proxy; S3/site thứ hai/cụm mới là stand-in).

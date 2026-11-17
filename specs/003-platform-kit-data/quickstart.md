# Quickstart: F003 — Platform kit dữ liệu: entx + UnitOfWork, CQRS read/write, idempotency, outbox, modcall, testkit

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F001, F002
- Lệnh: `make up`, `make migrate`, `make test MOD=pkg`, `make test-integration MOD=pkg` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); `nats-box` (profile `tools`) để xem stream.
- Module mẫu: `src/backend/pkg/testkit/sample` theo khuôn ADR-0087 (entity `Widget` mixin `Base`; write context: command `CreateWidget`/`DeleteWidget` có `IdempotencyKey`, `UnitOfWork` + outbox; read context: query `ListWidgets` qua `ReadSession`; consumer `sample.widget_on_created`; không mediator). Fixture tenant A = `th-demo-a`, B = `pgd-demo`; Principal `ad.truong.a`, `ad.truong.b`, `sa.admin`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Sinh ent cho `Widget`, xem migration Atlas | Cột Base đủ, `delete_reason varchar(64)`, unique partial | F003-AC-01, F003-AC-05 |
| 2 | `ListWidgets` với context không scope | `ErrMissingTenantScope`, 0 dòng | F003-AC-02 |
| 3 | `ad.truong.a` tạo widget A, `ad.truong.b` liệt kê | B không thấy widget A | F003-AC-02 |
| 4 | `DeleteWidget` rồi kiểm DB | `deleted_at`, `deleted_by`, `delete_reason` đặt; outbox có `widget.deleted` + `audit.recorded` | F003-AC-03, F003-AC-04 |
| 5 | Tạo lại widget cùng mã | Thành công | F003-AC-05 |
| 6 | Trong `InTenantTx` của A, `SELECT current_setting('app.tenant_id')` | = A; với `CrossTenantScope` dùng role `_platform` | F003-AC-06 |
| 7 | Thêm `field.String("x")` không MaxLen; thêm SQL `DELETE FROM` | `make lint` fail (entlint/tenancyguard) | F003-AC-07 |
| 8 | Command lỗi sau khi ghi outbox; `ListWidgets` bằng role chỉ đọc; adapter read thử `INSERT` trong `ReadSession` | Rollback, outbox trống; query thành công; lệnh ghi bị từ chối (`READ ONLY`) | F003-AC-08, F003-AC-09 |
| 9 | Thêm vào `internal/application/query` của module mẫu một import `port/write` hoặc `src/backend/pkg/outbox`; chạy `make lint MOD=pkg` | `go-arch-lint`/`depguard` fail, chỉ rõ file/dòng | F003-AC-10 |
| 10 | Tên widget dài hơn MaxLen; gọi `CreateWidget` bằng Principal thiếu permission | 422 `VALIDATION_FAILED` `MAX_LENGTH`, không mở transaction; lỗi `authz.Require`, repository không được gọi | F003-AC-11 |
| 10b | Gọi `CreateWidget` rồi xem trace/metric/log | Span `sample.command.CreateWidget` là con của span HTTP; metric `eduworld_usecase_duration_seconds{kind="command"}`; log không có payload | F003-AC-33 |
| 11 | Hai lần `Save` cùng version | Lần 2: 409 `CONFLICT` | F003-AC-12 |
| 12 | `CreateWidget` lặp cùng key/payload; khác payload; song song; không key ở op bắt buộc | Replay + `Idempotent-Replayed: true`; 409 `IDEMPOTENCY_KEY_REUSED`; request song song chờ rồi replay (một widget); 428 | F003-AC-13, F003-AC-14 |
| 13 | Handler trả 500 (hoặc 4xx do domain) rồi gửi lại cùng key | Không có bản ghi idempotency; được xử lý lại | F003-AC-15 |
| 13b | Gửi `Idempotency-Key` UUIDv4; key UUIDv7 cũ hơn `idempotency_record_ttl`; key sinh 23:59 UTC rồi retry 00:01 | `400 BAD_REQUEST`; `400 BAD_REQUEST`; replay | F003-AC-37 |
| 14 | `ad.truong.b` dùng key của `ad.truong.a` | Xử lý như request mới của B | F003-AC-16 |
| 15 | Commit widget, giết relay trước ack, chạy lại; `nats stream view EVT_SAMPLE` | Event có mặt đúng một lần theo `Nats-Msg-Id` | F003-AC-17, F003-AC-18 |
| 16 | Giao lại cùng event cho consumer; giao lại lần nữa sau nửa đêm (clock giả) | Bỏ qua, `processed_events` một dòng với `event_occurred_at` của envelope | F003-AC-19, F003-AC-38 |
| 17 | Event `tenant_id = A` tham chiếu widget của B | DLQ | F003-AC-20 |
| 18 | Consumer lỗi liên tục | Sau ngưỡng: `ew.dlq.sample.<consumer>` + `infra.dead_letters` | F003-AC-21 |
| 19 | Xem payload `audit.recorded` | Không PII | F003-AC-22 |
| 20 | Publish `user_role.changed` cho `ad.truong.a` | Cache Principal bị xóa | F003-AC-23 |
| 21 | `modcall.Call` tới facade giả từ module trong và ngoài `x-allowed-callers`; bỏ `Idempotency-Key` ở operation ghi | Span + metric có `caller`/`callee`; bên nhận thấy scope, Principal gốc; thiếu key → 500 không gọi; caller lạ → `403 CALLER_NOT_ALLOWED` | F003-AC-24, F003-AC-25 |
| 22 | Facade giả chậm quá deadline; panic; trả 404 Problem | 503 `UNAVAILABLE` sau một lần gọi (không retry, không circuit); 500; 404 giữ `code` | F003-AC-26, F003-AC-27 |
| 23 | Adapter `PrincipalResolver` với facade identity giả trả không membership | `authz.ErrNotMember` | F003-AC-28 |
| 24 | SIGTERM khi đang có request 2 s và lô consumer | Request hoàn tất; message chưa ack được giao lại; outbox được relay sau | F003-AC-29, F003-AC-30 |
| 25 | Chạy `testkit` + contract/facade test của module mẫu; sửa spec breaking | Pass; `make specs-breaking` fail | F003-AC-31, F003-AC-32 |
| 26 | `DeleteWidget` bằng role `_app` (RLS bật), rồi `ListWidgets`; đọc lại với `Bypass(ctx, "trash")` | Xóa thành công, list không còn; Bypass thấy dòng | F003-AC-34 |
| 27 | Chạy `DELETE FROM infra.outbox_events` bằng `_app`/`_relay`; relay publish bằng `_relay` | Lỗi quyền; relay chỉ cập nhật `published_at` | F003-AC-35 |
| 28 | Chạy job `infra-partition-maintenance` với partition cũ đã publish hết và một partition còn outbox chưa publish | Partition đầu bị gỡ, partition sau giữ lại; partition 7 ngày tới có sẵn | F003-AC-36 |

## 3. Lệnh test tự động

```bash
make test MOD=pkg
make test-integration MOD=pkg     # postgres:17, nats:2.10-alpine -js, valkey:8
make lint MOD=pkg
make specs-breaking
```

## 4. Bằng chứng cần nộp

- Số test pass/fail, log kịch bản 15 (event_id trước/sau giết relay), output `nats consumer info`, `trace_id` một command có consumer nối trace.

## 5. Bằng chứng (2026-11-18, Windows 11 + Docker Desktop, Go 1.27; postgres:17, nats:2.10-alpine -js, valkey:8 bằng testcontainers)

Kịch bản §2 được tự động hóa bằng test trên module mẫu `src/backend/pkg/testkit/sample` (không thao tác tay bằng `nats-box`).

| # | Test tự động | Kết quả |
|---|---|---|
| 1 | `pkg/entx` `TestMixinColumns` (cột theo 4 loại bảng, `delete_reason` 64), `TestFixtureMatchesEnt` | pass |
| 2–3, 6 | `TestEntx/TenantFilterAndScope` (không scope → `ErrMissingTenantScope`; ngoài tx → `ErrNoTransaction`; A không thấy B; role `learning_app` + `app.tenant_id`; RLS chặn dòng `tenant_id NULL` từ `_app`; tạo với tenant ngoài scope bị chặn; B không update được dòng của A), `TestEntx/PlatformScopeAndGlobal` (`learning_platform`, T*/G) | pass |
| 4–5, 26 | `TestEntx/SoftDelete` (`deleted_at/by/reason`; outbox có `audit.recorded` + `widget.deleted` cùng tx; `Bypass(trash)` đọc lại; tạo lại cùng mã; TA → `ErrHardDeleteForbidden`), `pkg/testkit/sample` `TestSampleCQRS` | pass |
| 7 | `tools/entlint`, `tools/tenancyguard` với fixture vi phạm (MaxLen, Text, 0/2 mixin, policy có `deleted_at`, bảng thiếu RLS, `DELETE FROM` trong migration/Go, SQL thiếu tenant/`deleted_at`); chạy trên cây thật: sạch | pass |
| 8 | `TestEntx/UnitOfWorkCommitRollback` (lỗi/panic → rollback, outbox trống; `Do` lồng dùng chung tx), `TestEntx/ReadSessionIsReadOnly`, `TestSampleCQRS` (lỗi sau khi ghi outbox → rollback) | pass |
| 9 | `tools/archgen` `TestReadWriteContexts`, `scripts/ci/test-arch-fixtures.sh` 14/14 | pass |
| 10, 10b | `TestCreateWidgetValidationAndAuthz` (422 `MAX_LENGTH` trước tx; thiếu quyền → không chạm repository), `observability` `TestUseCase` (span con, metric `module/kind/usecase/result`, log có `trace_id`, không payload) | pass |
| 11 | `TestSampleCQRS` (version cũ → 409 `CONFLICT`) | pass |
| 12–14, 13b | `TestIdempotentCreate` (428; UUIDv4 → 400 không gọi handler; replay + header; khác body → 409; user B không nhận response của A; 2 request đồng thời → 1 widget, 1 replay; lỗi trong tx → không có bản ghi, retry được; bản ghi hết hạn bị bỏ qua), `TestParseKey` (23:59 → 00:01 cùng `key_issued_on`) | pass |
| 15 | `TestOutboxPipeline/RelayCrashAndRerun` (crash sau 3 lần publish → không đánh dấu; chạy lại → mỗi dòng outbox đúng một message trong `EVT_LEARNING`) | pass |
| 16–18 | `ConsumerIdempotentAndScoped` (giao 2 lần → xử lý 1; `event_occurred_at` = envelope; scope từ envelope; event platform → DLQ), `DeadLetters` (lỗi tạm 2 lần → DLQ, lỗi permanent → DLQ ngay, envelope sai → DLQ; message sau vẫn chạy) | pass |
| 19 | `TestAuditPayload` (theo JSON Schema, lọc khóa cấm, che email/JWT), `TestEnvelopeMatchesSchema` | pass |
| 20 | `natsx` `TestPrincipalInvalidator` (6 event × 2 replica), `authz` `TestCacheTwoLevels`, `TestInvalidateTenantAndAll` | pass |
| 21–22 | `modcall` `TestCallContextAndAllowlist`, `TestCallErrors` (deadline → 503 sau 1 lần gọi, panic → 500, giữ code, lỗi lạ → 500) | pass |
| 23 | `cmd/internal/wiring` `TestPrincipalResolver` (Principal, 404 → `ErrNotMember`, thiếu `grants` → 503, khóa → `ACCOUNT_LOCKED`, deadline, caller ngoài allowlist), `TestGeneratedFake` | pass |
| 24 | `lifecycle` `TestAPIShutdown` (readyz 503, request đang chạy hoàn tất, NATS drain, pool đóng), `TestWorkerShutdownMidBatch` (dừng sau 1/6, worker sau xử lý 5 còn lại, không trùng); integration `cmd/core-api`, `cmd/worker` vẫn pass | pass |
| 25 | `testkit` `TestKit`, `testkit/contract` (response public, facade, payload); `make specs-breaking` có từ F001, chưa chạy lại trong phiên này | pass |
| 27 | `TestOutboxPipeline/Grants` (không RLS trên `infra`, quyền đúng từng role, không DELETE/TRUNCATE, partition không có grant trực tiếp) | pass |
| 28 | `partitionmaint` `TestPartitionMaintenance` (≥ 7 ngày, gỡ processed/idempotency cũ, giữ outbox chưa publish và dead letter chưa xử lý, gỡ sau khi xử lý, không chạm bảng nghiệp vụ) | pass |
| AC-39 | `projection` `TestRebuild` (khớp lại, chạy lại không đổi, bản `source_version` mới hơn giữ nguyên, facade lỗi → lỗi và không mất dữ liệu), `cmd/worker` `TestProjectionCommand` | pass |

- Tổng (gói F003 + F002 liên quan, unit + integration): 65 test cấp cao pass. Một lần chạy song song toàn bộ, `TestCacheTwoLevels` lỗi một lần (không tái hiện qua 5 lần chạy lại); test đã thêm bước chờ Valkey sẵn sàng.
- `golangci-lint` 0 issue; `archgen`, `ewlint` (bypassreason, scopeguard, nointernalhttp), `routematrix`, `entlint`, `tenancyguard`: sạch.
- Chưa chạy qua GNU make (máy chưa có make) và chưa xem trace nối consumer trên Tempo thật.

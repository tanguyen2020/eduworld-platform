# Quickstart: F003 — Platform kit dữ liệu: entx + UnitOfWork, CQRS read/write, idempotency, outbox, modcall, testkit

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F001, F002
- Lệnh: `make up`, `make migrate`, `make test MOD=pkg`, `make test-integration MOD=pkg` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); `nats-box` (profile `tools`) để xem stream.
- Module mẫu: `pkg/testkit/sample` theo khuôn ADR-0087 (entity `Widget` mixin `Base`; write context: command `CreateWidget`/`DeleteWidget` có `IdempotencyKey`, `UnitOfWork` + outbox; read context: query `ListWidgets` qua `ReadSession`; consumer `sample.widget_on_created`; không mediator). Fixture tenant A = `th-demo-a`, B = `pgd-demo`; Principal `ad.truong.a`, `ad.truong.b`, `sa.admin`.

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
| 9 | Thêm vào `internal/application/query` của module mẫu một import `port/write` hoặc `pkg/outbox`; chạy `make lint MOD=pkg` | `go-arch-lint`/`depguard` fail, chỉ rõ file/dòng | F003-AC-10 |
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

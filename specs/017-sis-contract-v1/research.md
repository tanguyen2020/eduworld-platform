# Research: F017 — SIS `contract_v1`: provider API theo EduWorld SIS Contract, webhook, conformance kit, `sis-mock` (tách từ F011)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. `sis-mock`: Prism hay server mock tự viết

- **Decision**: Giữ Prism (`stoplight/prism:4`, xem LOCAL_DEVELOPMENT §3.1) mock `specs/sis/eduworld-sis-contract.v1.yaml` ở chế độ static (trả `examples`), thêm `examples` dữ liệu demo TH Demo A vào contract; giả lập lỗi bằng header `Prefer: code=401|404|429|503`. Test tích hợp của worker (phân trang nhiều trang, 410, timeout, webhook) dùng fake SIS `httptest` trong `tests/fakesis/` (nhãn `MOCK`, chỉ test).
- **Rationale**: LOCAL_DEVELOPMENT §2 đã chốt image Prism, cổng 4010; SIS_INTEGRATION §11 tách "mock sinh từ spec" (môi trường dev + conformance) và "fake SIS httptest" (kịch bản lỗi). Prism luôn khớp contract nên conformance kit chạy vào nó phát hiện test sai.
- **Alternatives**: Server Go có trạng thái (phân trang thật, delta) — loại vì trùng fake httptest và phải bảo trì song song contract; Prism dynamic (`-d`) — loại vì dữ liệu ngẫu nhiên không khớp seed, không kiểm được kịch bản demo.

## R2. Lock run và nonce webhook ở Valkey

- **Decision**: Lock `ew:identity:sis:lock:<connection_id>` (TTL 10 phút, heartbeat) và nonce `ew:identity:sis:wh:nonce:<delivery_id>` (TTL `sis_webhook_nonce_ttl`) ở Valkey; job lưu bền trong JetStream (`Nats-Msg-Id = delivery_id`); SyncRun lưu PostgreSQL.
- **Rationale**: Theo SIS_INTEGRATION §5. Mất Valkey chỉ làm mất khóa tạm: replay cũ hơn `sis_webhook_skew` đã bị chặn bởi timestamp; apply idempotent theo `raw_hash` (F011) nên xử lý trùng không sai dữ liệu (R9 đạt).
- **Alternatives**: Bảng `sis.webhook_deliveries` unique `delivery_id` — loại cho Release 1.0 vì thêm bảng ghi nóng mà không tăng an toàn so với timestamp + JetStream dedupe; `pg_advisory_lock` cho run — loại vì worker nhiều replica, lock theo kết nối DB dễ treo khi pod chết.

## R3. Sinh client từ OpenAPI 3.1 có `webhooks`

- **Decision**: oapi-codegen client mode cho `paths`; kiểu `ChangeBatch`/`Change` sinh từ `components.schemas` (dùng chung cho delta và webhook). Handler webhook viết tay (chi router), không sinh server từ `webhooks`. Validate response bằng validator JSON Schema của F011 (trích từ cùng file) trước khi map sang kiểu sinh.
- **Rationale**: oapi-codegen chưa sinh server cho `webhooks`; kiểu chung tránh lệch giữa delta và webhook.
- **Alternatives**: ogen — loại để thống nhất một generator trong repo (ADR-0040 dùng oapi-codegen).

## R4. Đọc secret theo `secret_ref`

- **Decision**: Port `SecretStore.Get(ctx, ref)`; hiện thực `filestore` đọc Docker/Swarm secret mount `/run/secrets/<ref>` (local: thư mục `deploy/compose/secrets/` không commit, chỉ `.example`). `secret_ref` pattern `^[a-z0-9][a-z0-9._/-]{0,511}$` (khớp `varchar(512)`); cache trong RAM ≤ 5 phút; không log.
- **Rationale**: DEPLOYMENT dùng Docker Swarm secret (ADR hiện hành); AGENTS R3, R4.
- **Alternatives**: Vault — chưa có ADR; thêm sau qua cùng port.

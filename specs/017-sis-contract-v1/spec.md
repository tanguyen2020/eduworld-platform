# Feature Specification: F017 — SIS `contract_v1`: provider API theo EduWorld SIS Contract, webhook, conformance kit, `sis-mock` (tách từ F011)

**Feature Branch**: `017-sis-contract-v1` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `identity` (+ `deploy/compose`, `specs/postman`) · **Mốc**: M2 · **Phụ thuộc**: F011
**Input**: Nhận dữ liệu SIS qua HTTP theo [EduWorld SIS Contract v1](../sis/eduworld-sis-contract.v1.yaml): connection `contract_v1` (cấu hình, `secret_ref`, health check), full sync theo lịch/thủ công, delta sync, webhook push, retry/backoff/`auth_error`/`contract_error`, adapter `lookupStudent` cho login sync, bộ conformance test và `sis-mock` để trường tự kiểm facade. Dùng lại validator → staging → reconciliation → apply của F011. FR nguồn: FR-SIS-001, FR-SIS-002, FR-SIS-003 (phần `contract_v1`), FR-SIS-004, FR-SIS-006 (phần pull), FR-SIS-007, FR-SIS-008, FR-SIS-009, FR-ORG-018 (adapter lookup).

## Clarifications

### Session 2026-11-16

- Q: F017 có tự reconcile/apply không? → A: Không. F017 chỉ hiện thực `SISProvider.Kind() = "contract_v1"`, lịch, webhook, retry; bản ghi đi vào pipeline staging → `Reconciler` → apply của F011 (F011 tasks "Dependencies": F017 dùng lại port và pipeline).
- Q: `sis-mock` là gì? → A: Container Prism mock `specs/sis/eduworld-sis-contract.v1.yaml` (LOCAL_DEVELOPMENT §2, cổng 4010) với `examples` dữ liệu demo; test tích hợp của worker dùng fake SIS `httptest` (chỉ test, gắn nhãn mock — SIS_INTEGRATION §11). Không viết server mock riêng.
- Q: Webhook secret cấp thế nào khi API không được trả credential? → A: System Admin tạo secret trong secret manager và chỉ nhập tham chiếu `webhook_secret_ref`; secret giao cho trường ngoài hệ thống (SIS_INTEGRATION §9: API không bao giờ trả credential) [Proposed].
- Q: Capability của connection lưu ở đâu? → A: Lấy từ `getHealth` lúc health check và đầu mỗi run, lưu trên connection (cột mới, data-model); login sync (F011) đọc capability này.
- Q: Ngưỡng 10% vi phạm, retry 6 lần, 10 req/s, 5 run lỗi liên tiếp chưa có tên trong CONFIG_DEFAULTS? → A: Dùng giá trị [Proposed] của SIS_INTEGRATION §2.5, §4.2, §7 làm hằng cấu hình code; task Polish đăng ký tên tham số.

### Session 2026-11-17

- Q: Timeout lookup và retention staging? → A: Chỉ dùng `sis_login_sync_timeout_ms` (bỏ `sis_lookup_timeout`); staging SIS giữ 30 ngày theo `sis_staging_retention` (SIS_INTEGRATION, RB-111).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Đăng ký connection `contract_v1` và kiểm tra kết nối (Priority: P1) 🎯 MVP

`sa.admin` tạo connection `contract_v1` cho `th-demo-a` (base URL facade, `auth_type`, `token_url`, `secret_ref`, `contract_version`, lịch), gán TH Demo A (F011), bấm kiểm tra kết nối, rồi chạy full sync dry-run để School Admin xem diff ([SIS_INTEGRATION §2.6](../../docs/09-identity/SIS_INTEGRATION.md)).

**Why this priority**: Không có connection thì không có pull, webhook hay lookup.

**Independent Test**: Tạo connection trỏ `sis-mock` → health check trả `contract_version` 1.x + capabilities; đổi base URL sang facade trả `2.0.0` → `contract_error`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F017-AC-01 | KHI người có `platform.sis_connection.manage` tạo connection `provider_kind = contract_v1` (`X-Tenant-Id` = tenant đích, `Idempotency-Key`), module identity PHẢI yêu cầu `base_url` (https), `auth_type` ∈ {`oauth2_client_credentials`, `api_key`, `mtls`}, `token_url` khi OAuth2, `secret_ref`, `contract_version` major 1, lưu `config` không chứa secret hay field-mapping và ghi audit | FR-SIS-001 · FR-SIS-007 · [SIS_INTEGRATION §4.2](../../docs/09-identity/SIS_INTEGRATION.md) | — |
| F017-AC-02 | KHI kiểm tra kết nối, module identity PHẢI gọi `getHealth`, lưu `capabilities` và thời điểm kiểm tra; NẾU `contract_version` major khác major của connection, THÌ PHẢI đặt `status = contract_error` và trả `422 SIS_CONTRACT_VERSION_UNSUPPORTED` | FR-SIS-001 · [SIS_INTEGRATION §2.4](../../docs/09-identity/SIS_INTEGRATION.md) | TC-AUTH-011 |
| F017-AC-03 | KHI kích hoạt full sync với `dry_run = true`, module identity PHẢI chạy pull + validate + staging + reconcile và ghi `stats` theo action nhưng không apply, không phát event nghiệp vụ | FR-SIS-003 · [SIS_INTEGRATION §2.6](../../docs/09-identity/SIS_INTEGRATION.md) | — |
| F017-AC-04 | HỆ THỐNG PHẢI không trả `secret_ref`, `webhook_secret_ref` hay credential ở bất kỳ response nào (chỉ cờ `has_secret`, `has_webhook_secret`) và không ghi chúng vào log/audit | FR-SIS-007 · AGENTS R4, R14 | — |

---

### User Story 2 — Full sync theo lịch và thủ công (Priority: P1)

Worker pull toàn bộ trường của connection (trường → năm học → lớp → học sinh → enrollment), validate theo contract, đưa vào pipeline F011; một connection chỉ một run tại một thời điểm.

**Why this priority**: Đường nhận dữ liệu chính của trường có API (US-SIS-001).

**Independent Test**: Trỏ connection vào fake SIS có 3 trang học sinh, 1 bản ghi sai enum, 1 trường lạ → run `partially_failed`, 1 lỗi `SIS_CONTRACT_VIOLATION`, trường lạ không lưu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F017-AC-05 | KHI tới lịch `sis_sync_schedule` hoặc người có quyền gọi `triggerSisSync` `kind = full`, module identity PHẢI tạo SyncRun `mode = full`, gọi `listSchools`, `listAcademicYears`, `listClasses`, `listStudents`, `listEnrollments` theo cursor cho mọi `school_external_id` đã ánh xạ của connection và chuyển bản ghi vào staging của F011 | FR-SIS-002 · FR-SIS-004 · [SIS_INTEGRATION §5](../../docs/09-identity/SIS_INTEGRATION.md) | TC-AUTH-030 |
| F017-AC-06 | NẾU bản ghi thiếu trường bắt buộc, sai kiểu/enum, THÌ module identity PHẢI ghi `SIS_CONTRACT_VIOLATION` theo bản ghi và tiếp tục; trường ngoài contract PHẢI bị bỏ, không lưu, tăng `eduworld_sis_contract_unknown_fields_total`; NẾU vượt ngưỡng vi phạm của [SIS_INTEGRATION §2.5](../../docs/09-identity/SIS_INTEGRATION.md), THÌ PHẢI dừng run và đặt connection `contract_error` | FR-SIS-003 · ADR-0021 | TC-AUTH-011 |
| F017-AC-07 | TRONG KHI một SyncRun của connection đang chạy, module identity PHẢI từ chối run mới của cùng connection bằng `409 SIS_SYNC_ALREADY_RUNNING` (lịch: bỏ qua, ghi metric) | [SIS_INTEGRATION §5](../../docs/09-identity/SIS_INTEGRATION.md) | — |
| F017-AC-08 | KHI full sync kết thúc, module identity PHẢI chuyển danh sách khóa nguồn đã thấy cho reconciler F011 để đánh dấu `missing_in_source`, cập nhật `last_success_at` và phát `sis.sync_completed` (`mode`, `status`, `stats`, `connection_status`) qua outbox | FR-SIS-006 · FR-SIS-008 | — |
| F017-AC-09 | NẾU `school_external_id` trả về chưa ánh xạ vào trường của connection, THÌ module identity PHẢI không tạo trường/tenant và để reconciler tạo issue `unmapped_school` | FR-SIS-001 · [SIS_INTEGRATION §2.3](../../docs/09-identity/SIS_INTEGRATION.md) | — |

---

### User Story 3 — Lỗi, retry và trạng thái connection (Priority: P1)

Facade chậm, trả 429/5xx hoặc thu hồi credential; worker retry đúng cách, không làm nghẽn hệ thống, báo đúng trạng thái.

**Why this priority**: Không xử lý lỗi thì một facade hỏng làm treo worker của mọi tenant (FR-SIS-006).

**Independent Test**: Fake SIS trả 503 hai lần rồi 200; 429 kèm `Retry-After`; 401 → kiểm số lần gọi, khoảng chờ, trạng thái connection.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F017-AC-10 | KHI request tới facade timeout, lỗi mạng hoặc 5xx, client `contract_v1` PHẢI retry exponential backoff + jitter theo [SIS_INTEGRATION §7](../../docs/09-identity/SIS_INTEGRATION.md), tôn trọng `Retry-After`; hết lượt PHẢI kết thúc run `failed` | FR-SIS-006 | — |
| F017-AC-11 | KHI facade trả 429, client PHẢI chờ theo `Retry-After` và giảm tốc độ gọi của connection; HỆ THỐNG PHẢI giới hạn tốc độ mỗi connection theo [SIS_INTEGRATION §4.2](../../docs/09-identity/SIS_INTEGRATION.md) | FR-SIS-006 | — |
| F017-AC-12 | KHI facade trả 401/403 (kể cả lỗi lấy token OAuth2), module identity PHẢI dừng run `failed`, đặt connection `auth_error`, không retry và phát `sis.sync_completed` có `connection_status = auth_error` | FR-SIS-006 · [SIS_INTEGRATION §7](../../docs/09-identity/SIS_INTEGRATION.md) | — |
| F017-AC-13 | KHI số run thất bại liên tiếp của một connection đạt ngưỡng circuit breaker của [SIS_INTEGRATION §7](../../docs/09-identity/SIS_INTEGRATION.md), module identity PHẢI đặt `paused` và ngừng lịch tới khi System Admin kích hoạt lại qua `updateSisConnection` (audit) | FR-SIS-006 | — |
| F017-AC-14 | TRONG KHI connection ở `paused`/`auth_error`/`contract_error`, module identity PHẢI từ chối `triggerSisSync` bằng `409 SIS_CONNECTION_NOT_ACTIVE` và không nhận webhook của connection | FR-SIS-006 | — |

---

### User Story 4 — Delta sync (Priority: P2)

Trường khai báo capability `delta`; worker lấy thay đổi theo watermark trong giờ hành chính.

**Why this priority**: Giảm độ trễ trạng thái (FR-SIS-008) nhưng full sync vẫn đủ cho MVP.

**Independent Test**: `listChanges` trả 2 trang; apply xong watermark = `next_cursor` trang cuối; lần sau gửi `since` đó; 410 → full sync.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F017-AC-15 | Ở NƠI connection có capability `delta`, module identity PHẢI chạy delta theo `sis_sync_schedule`: gọi `listChanges(since = delta_watermark)`, validate từng `Change` (`record` theo `entity_kind`, bắt buộc khi `upsert`) và đưa vào pipeline F011 theo thứ tự `sequence` | FR-SIS-004 · [SIS_INTEGRATION §5](../../docs/09-identity/SIS_INTEGRATION.md) | — |
| F017-AC-16 | module identity PHẢI chỉ lưu `delta_watermark` mới sau khi mọi lô của run apply thành công; NẾU facade trả 410, THÌ PHẢI xếp một full sync và không đổi watermark | FR-SIS-004 | — |
| F017-AC-17 | KHI `Change.operation = delete` hoặc `Student.status` ∈ {`inactive`, `graduated`, `suspended`, `transferred_out`}, module identity PHẢI chuyển cho reconciler F011 trong cùng run để trạng thái có hiệu lực trong một chu kỳ đồng bộ | FR-SIS-008 | TC-AUTH-036 |

---

### User Story 5 — Webhook push từ SIS (Priority: P2)

SIS có capability `webhook` đẩy `ChangeBatch` tới `POST /internal/sis/webhooks/{connection_id}`; EduWorld xác thực, chống replay, xử lý bất đồng bộ như delta.

**Why this priority**: Real-time là tùy chọn của trường (FR-SIS-004 P1–P3).

**Independent Test**: Gửi batch ký đúng → 202 + job; gửi lại cùng `delivery_id` → 409; chữ ký sai/lệch giờ → 401.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F017-AC-18 | KHI nhận webhook, module identity PHẢI kiểm `X-EduWorld-Signature` = HMAC-SHA256(secret từ `webhook_secret_ref`, `timestamp + "." + raw_body`) bằng so sánh thời gian hằng, `X-EduWorld-Timestamp` trong `sis_webhook_skew`; sai PHẢI trả `401` không tiết lộ lý do | FR-SIS-004 · FR-SIS-007 · `receiveSisChanges` | — |
| F017-AC-19 | NẾU `delivery_id` đã nhận trong `sis_webhook_nonce_ttl`, THÌ module identity PHẢI trả `409` và không xếp job; KHI hợp lệ, PHẢI validate `ChangeBatch` (≤ `maxItems`, `contract_version` major 1), publish job `webhook_batch` vào `IDENTITY_SIS_JOBS` với `Nats-Msg-Id = delivery_id` và chỉ trả `202` sau khi JetStream ack | FR-SIS-004 · AGENTS R10 · [SIS_INTEGRATION §5](../../docs/09-identity/SIS_INTEGRATION.md) | — |
| F017-AC-20 | NẾU connection không tồn tại, không active, không khai báo capability `webhook` hoặc thiếu `webhook_secret_ref`, THÌ module identity PHẢI trả `404`; route PHẢI chỉ có trên APISIX internal/partner có IP allowlist | FR-SIS-007 · [API_DESIGN §9.17](../../docs/07-backend/API_DESIGN.md) | — |
| F017-AC-21 | NẾU xử lý job `webhook_batch` lỗi, THÌ worker PHẢI không ack, để redeliver theo [SIS_INTEGRATION §7](../../docs/09-identity/SIS_INTEGRATION.md) và chuyển `DLQ_IDENTITY` khi hết lượt; mỗi batch PHẢI tạo SyncRun `mode = webhook` | FR-SIS-006 | — |

---

### User Story 6 — Adapter `lookupStudent` cho login sync (Priority: P1)

Login sync nhánh B/C (F011) cần tra một học sinh ở đúng SIS nguồn; F017 cung cấp hiện thực `SisLookupPort` cho connection `contract_v1`.

**Why this priority**: Học sinh đăng nhập lần đầu ở trường có API phải thấy đúng trường/lớp (ADR-0061).

**Independent Test**: Fake SIS trả học sinh có lớp / 404 / chậm hơn timeout / có trường lạ; kiểm kết quả port và số lời gọi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F017-AC-22 | KHI login sync gọi lookup cho connection `contract_v1` có capability `student_lookup`, adapter PHẢI gọi `lookupStudent(school_external_id, login_username)` với timeout `sis_login_sync_timeout_ms`, không retry, validate `Student` theo contract (bỏ trường lạ, tăng metric) rồi trả cho pipeline F011 | FR-ORG-018 · ADR-0061 | TC-SIS-102 · TC-SIS-107 |
| F017-AC-23 | NẾU connection không có `student_lookup` hoặc không active, THÌ adapter PHẢI trả `ErrUnsupported` không gọi mạng; facade 404 PHẢI trả "không tìm thấy"; timeout/5xx PHẢI trả lỗi tạm thời để F011 dùng dữ liệu gần nhất | FR-ORG-018 · ADR-0046 | TC-SIS-104 |
| F017-AC-24 | adapter PHẢI giới hạn lookup đồng thời mỗi connection theo `sis_login_sync_concurrency` và có circuit breaker riêng từng connection; connection treo PHẢI không ảnh hưởng connection khác | FR-ORG-018 · NFR-AVAIL-005 | TC-SIS-106 |

---

### User Story 7 — Conformance kit và `sis-mock` (Priority: P1)

Trường/nhà cung cấp SIS chạy bộ conformance test của EduWorld vào facade trên môi trường test tới khi pass; CI chạy cùng bộ test vào `sis-mock` (US-SIS-001).

**Why this priority**: Onboarding trường cần công cụ tự kiểm trước khi tạo connection (FR-SIS-009).

**Independent Test**: `postman collection run` bộ conformance với env trỏ `sis-mock` → mọi test endpoint bắt buộc pass; trỏ vào facade thiếu `/health` → fail rõ ràng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F017-AC-25 | HỆ THỐNG PHẢI cung cấp bộ conformance test sinh từ contract, chạy bằng Postman CLI với biến môi trường `sis_base_url`, `auth_type`, credential, `school_external_id`, `academic_year`; mọi endpoint bắt buộc PHẢI được kiểm status, JSON Schema của response, phân trang cursor (`has_more`/`next_cursor`, `limit` ≤ tối đa), lỗi `application/problem+json`, `401` khi thiếu credential | FR-SIS-009 · [SIS_INTEGRATION §2.2](../../docs/09-identity/SIS_INTEGRATION.md) | TC-AUTH-012 |
| F017-AC-26 | Ở NƠI facade khai báo capability `delta`/`student_lookup`/`webhook`, bộ test PHẢI chạy thêm kiểm `listChanges` (thứ tự `sequence`, 410 với watermark hết hạn), `lookupStudent` (200/404) và mẫu webhook ký HMAC; capability không khai báo PHẢI được báo "bỏ qua", không fail | FR-SIS-009 · FR-SIS-004 | TC-AUTH-012 |
| F017-AC-27 | `sis-mock` PHẢI phục vụ đúng contract (Prism) với dữ liệu demo khớp seed TH Demo A, cho phép giả lập 401/404/429/503 theo header `Prefer`; CI PHẢI chạy bộ conformance vào `sis-mock` và fail khi contract thay đổi làm test hỏng | FR-SIS-009 · [SIS_INTEGRATION §11](../../docs/09-identity/SIS_INTEGRATION.md) | TC-AUTH-012 |

---

### User Story 8 — Bảo vệ API connection và đồng bộ (Priority: P1)

**Why this priority**: Constitution điều III, V; API SIS chỉ cho người quản trị qua gateway nội bộ.

**Independent Test**: Ma trận role × operation (`sa.admin`, `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`) và tenant chéo.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F017-AC-28 | HỆ THỐNG PHẢI chỉ cho `platform.sis_connection.manage` tạo/sửa/kiểm tra connection; `triggerSisSync` PHẢI cho thêm người có `tenant.sis.import` khi `school_id` thuộc scope trường của họ và thuộc connection; thiếu quyền PHẢI nhận `403`, tenant/trường khác PHẢI nhận `404` | FR-SIS-007 · NFR-SEC-006 · AGENTS R7, R8 | TC-TEN-001 |
| F017-AC-29 | HỆ THỐNG PHẢI yêu cầu `Idempotency-Key` cho tạo connection và `triggerSisSync` (thiếu → `428 IDEMPOTENCY_KEY_REQUIRED`), ghi audit mọi thay đổi connection và mỗi SyncRun với `actor = system:sis-sync:<connection_id>` | AGENTS R10 · [SIS_INTEGRATION §8](../../docs/09-identity/SIS_INTEGRATION.md) | — |
| F017-AC-30 | NẾU input vượt `maxLength`/pattern (`base_url` URL ≤ 2048, `secret_ref` ≤ 512, mã ≤ 64) hoặc `base_url` không phải https, THÌ module identity PHẢI trả `422 VALIDATION_FAILED`; route `/api/v1/sis/*` PHẢI không tồn tại trên gateway public | ADR-0041 · FR-SIS-007 | — |

### Edge Cases

- NẾU `getHealth` trả `degraded`, THÌ vẫn chạy run nhưng ghi cảnh báo trong `stats`.
- NẾU OAuth2 token hết hạn giữa run, THÌ client lấy token mới một lần; lỗi tiếp → F017-AC-12.
- NẾU facade trả trang có `has_more = true` nhưng `next_cursor` rỗng hoặc lặp lại, THÌ dừng run `failed` với lỗi `contract_error` của run (không lặp vô hạn).
- NẾU lịch full sync trùng một run delta đang chạy, THÌ run full chờ lượt sau (F017-AC-07).
- NẾU webhook đến khi connection đang có full sync, THÌ job xếp hàng và chạy sau khi lock được nhả.
- NẾU `secret_ref` không đọc được từ secret manager, THÌ run `failed`, connection `auth_error`, không log tham chiếu đầy đủ.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-SIS-001 | Client `contract_v1` sinh từ contract; connection `contract_v1`; version check | [SIS_INTEGRATION §2](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-002 | Pull trường, năm học, lớp, học sinh, enrollment, trạng thái; không giáo viên | [SIS_INTEGRATION §2.3](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-003 | Phần `contract_v1`: validate response theo contract, từ chối theo bản ghi (CSV → F011) | [SIS_INTEGRATION §2.5, §4.2](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-004 | Full theo lịch, delta, webhook | [SIS_INTEGRATION §5](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-006 | Retry/backoff, `auth_error`, `contract_error`, circuit breaker, SyncRun + audit của pull/webhook | [SIS_INTEGRATION §7–§8](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-007 | `secret_ref`, không trả credential, webhook chỉ gateway nội bộ/allowlist | [SIS_INTEGRATION §9](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-008 | Delta/webhook đưa trạng thái vào apply trong một chu kỳ (apply → F011) | [SIS_INTEGRATION §5](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-009 | Conformance kit + `sis-mock` | [SIS_INTEGRATION §2.6, §11](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-ORG-018 | Adapter `lookupStudent` cho `SisLookupPort` (logic nhánh A/B/C → F011) | [SIS_INTEGRATION §4.5](../../docs/09-identity/SIS_INTEGRATION.md) |

### Key Entities

- **SISConnection (`contract_v1`)**: tenant, tập trường, base URL, cách xác thực, tham chiếu secret (pull và webhook), phiên bản contract, capabilities, lịch, watermark delta, trạng thái, số run lỗi liên tiếp.
- **SyncRun**: một lần full/delta/webhook (có thể dry-run), người/lịch kích hoạt, thống kê, kết quả.
- **ChangeBatch / Change**: lô thay đổi từ delta hoặc webhook, có `delivery_id`, `sequence`.
- **Conformance kit**: bộ test + môi trường mẫu giao cho trường.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `sis_sync_schedule` | Lịch full/delta theo tenant |
| `sis_login_sync_timeout_ms` | Timeout adapter lookup (login sync) |
| `sis_login_sync_concurrency` | Giới hạn lookup đồng thời/connection |
| `sis_webhook_skew` / `sis_webhook_nonce_ttl` | Kiểm timestamp và chống replay webhook |
| `sis_staging_retention` | Staging của run pull (áp dụng bởi job F011) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chỉ module identity (trong `src/backend/cmd/worker`) gọi SIS; client game/portal không bao giờ gọi | ADR-0021 · FR-SIS-007 |
| Không gọi SIS trên đường đăng nhập ngoài nhánh lookup; không retry đồng bộ | ADR-0045 · ADR-0061 · AGENTS R30 |
| Không log PII, credential, `secret_ref` | AGENTS R4, R14 |
| Egress TLS 1.2+, kiểm cert; mTLS khi `auth_type = mtls` | [SIS_INTEGRATION §9](../../docs/09-identity/SIS_INTEGRATION.md) |
| Mọi chuỗi có giới hạn; chỉ soft-delete | AGENTS R21, R28 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Một trường mới đi từ "có facade" tới "full sync dry-run có diff" trong ≤ 1 ngày làm việc nhờ conformance kit (đo ở trường pilot).
- **SC-002**: Full sync 50.000 học sinh hoàn tất < 15 phút [Proposed] ([SIS_INTEGRATION §11](../../docs/09-identity/SIS_INTEGRATION.md)).
- **SC-003**: Học sinh bị SIS báo nghỉ học không tạo được game session mới sau ≤ 1 chu kỳ delta/webhook.
- **SC-004**: 0 credential/`secret_ref` xuất hiện trong response, log hoặc audit (quét tự động).
- **SC-005**: 100% endpoint bắt buộc của contract có test trong conformance kit, chạy xanh với `sis-mock` ở mọi PR sửa `specs/sis/`.

## Ngoài phạm vi

- Validator, staging, reconciliation, apply, provisioning, logic login sync nhánh A/B/C, import CSV, connection `csv_import`, list run/lỗi/issue → F011.
- Lan truyền khóa tới Keycloak/realtime/game session → F016.
- Màn hình SIS trên portal (connection, run, issue): School Admin → F052; System Admin → F059.
- On-demand lookup qua IdP trường/JIT (song song nhiều connection) — mở rộng sau (ADR-0063).
- Cấu hình route APISIX nền tảng → F004 (F017 chỉ thêm route webhook).

## Assumptions

- Adapter chỉ dùng `sis_login_sync_timeout_ms`; `sis_lookup_timeout` đã bỏ (RB-111).
- [Proposed] Ngưỡng vi phạm, lịch backoff, giới hạn tốc độ, ngưỡng circuit breaker theo SIS_INTEGRATION §2.5, §4.2, §7; đăng ký CONFIG_DEFAULTS ở task Polish.
- [Proposed] Xoay vòng webhook secret = cập nhật `webhook_secret_ref`; batch ký bằng secret cũ trong lúc đổi bị `401` và SIS gửi lại.
- [Proposed] Môi trường local cho phép `base_url` http tới `sis-mock` qua biến môi trường chỉ có trong compose dev; staging/production luôn bắt buộc https (F017-AC-30).
- OQ-01: chưa trường nào triển khai facade; contract vẫn Draft (v1.2.0).

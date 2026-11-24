# Data Model: F134 — Công cụ vận hành bổ sung: lệnh `reconcile-<module>`, loại tenant QA khỏi analytics, trang tải bộ cài cho trường

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

### Tenant — `platform.tenants` (Có sẵn · Thêm cột · DB `eduworld_identity`)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `is_qa` | `boolean` | **Thêm cột** NOT NULL DEFAULT `false`; CHECK `NOT (is_qa AND tenant_type = 'common')` | Chỉ System Admin đổi; migration expand (default false) |

### Projection — `ref.tenants` (Có sẵn · Thêm cột · DB `eduworld_game`, `eduworld_learning`)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `is_qa` | `boolean` | **Thêm cột** NOT NULL DEFAULT `false` | consumer `tenant.created/updated` cập nhật theo `source_version` (DATABASE_DESIGN quy tắc projection) |

**Index**: `ref.tenants (tenant_id) WHERE is_qa` (tập nhỏ; query dashboard dùng `NOT EXISTS`/anti-join).

### Bảng hạ tầng dùng để đối soát (Có sẵn · chỉ đọc · mọi database module — DATABASE_DESIGN §3.9)

| Bảng | Cột đọc | Ghi chú |
|---|---|---|
| `infra.outbox_events` | `event_id` (UUIDv7), `event_type`, `tenant_id`, `created_at`, `published_at` | Lọc cửa sổ theo `created_at`; giữ `outbox_retention` |
| `infra.processed_events` | `consumer`, `event_id`, `processed_at` | Lọc cửa sổ theo khoảng `event_id` UUIDv7 (`uuidv7_floor(from)`…`uuidv7_ceil(to)`) trên PK `(consumer, event_id)` |
| `infra.dead_letters` | `consumer`, `event_id` | Đánh dấu `dead_lettered` |

Không tạo bảng mới cho đối soát: báo cáo là file JSON (schema ở §1 cuối), audit qua `audit.recorded`.

### Báo cáo đối soát — `reconcile-report.schema.json` (file, Mới)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `run_id` | uuid | |
| `module` | enum `identity/learning/game/competition/chat` | |
| `restore_point`, `window_from`, `window_to`, `generated_at` | date-time | |
| `status` | enum `complete/incomplete` | `incomplete` khi một module không đọc được |
| `peers[]` | `{module, direction (enum outbound/inbound), status, error_code (≤64)}` | |
| `counts` | object `{lost_after_restore, pending_publish, pending_delivery, dead_lettered, unexplained}` integer | |
| `mismatches[]` | `{event_id (uuid), event_type (≤64), consumer (≤128), tenant_id (uuid?), event_time (date-time), classification (enum như `counts`)}` | ≤ 100 000 mục; vượt → cắt + `truncated: true` |

### Chỉ mục bộ cài trường — `play.<domain>/releases/school-kit/<channel>.json` + `.sig` (file CDN, Mới)

| Trường | Kiểu | Ràng buộc |
|---|---|---|
| `channel` | enum `staging/production` | |
| `generated_at` | date-time | |
| `kits[]` | `{kit_id (≤64, pattern [a-z0-9-]), platform (enum android — ADR-0101), client_version (SemVer ≤32), grade_bands[] (enum g1_3/g4_5/g6_7/g8_9), url (uri https ≤2048), sha256 (hex 64), size_bytes (int ≥0), min_os (≤32), released_at (date-time)}` | ≤ 50 mục |

Chữ ký RSA-SHA256 bằng cùng khóa phát hành của F133 (manifest phát hành); module game kiểm bằng khóa công khai cấu hình.

## 2. Vòng đời / trạng thái

| Entity | Từ | Sang | Điều kiện |
|---|---|---|---|
| Tenant.`is_qa` | `false` | `true` | `updateTenant` (System Admin), tenant không phải `common` |
| Tenant.`is_qa` | `true` | `false` | `updateTenant`; dữ liệu trong thời gian QA không được backfill |
| Mismatch | — | phân loại | Quy tắc ở [spec Clarifications](spec.md) (event time so với T, `published_at`, DLQ, `--settle`) |

## 3. Quy tắc kiểm tra dữ liệu

- Cửa sổ đối soát `window_to − window_from ≤ outbox_retention`, `window_from ≤ restore_point ≤ window_to`.
- Event lọc theo `event_types` mà consumer bên nhận đăng ký (registry `src/backend/pkg/messaging`), tránh đếm event không ai tiêu thụ.
- Cờ QA không áp cho tenant `common`; dashboard luôn anti-join `ref.tenants.is_qa`.
- Bộ cài chỉ trả khi chỉ mục đúng schema và chữ ký; lọc `grade_bands ∩ grade_bands(trường)` với `grade_bands(trường)` suy từ `ref.classes.grade_level` của lớp đang hoạt động.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `tenant.updated` | Đổi `is_qa` | `specs/events/payloads/identity/tenant.updated.v1.schema.json` (owner F010; F134 thêm `is_qa` tương thích, `changed_fields[]`) |
| `audit.recorded` | `tenant.qa_flag_changed`, `ops.reconcile.completed` (module được đối soát) | Có sẵn (envelope chuẩn) |

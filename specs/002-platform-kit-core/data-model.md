# Data Model: F002 — Platform kit lõi: config, observability, problem, httpx, authz, tenancy

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F002 không tạo bảng PostgreSQL. Dữ liệu tạm duy nhất là cache Principal trong Valkey (ADR-0004, không phải nguồn sự thật; nguồn ở `eduworld_identity`, F012). Helper RLS chỉ **sinh SQL** cho bảng do feature khác tạo.

Cấu trúc dữ liệu trong bộ nhớ/Valkey để tham chiếu khi code:

| Cấu trúc | Khóa / vị trí | Giá trị | TTL | Ghi chú |
|---|---|---|---|---|
| Principal cache L1 | LRU trong tiến trình, khóa `(user_id, tenant_id, client_id)` | `authz.Principal` | theo `principal_cache` (L1) | Xóa khi `Invalidate` |
| Principal cache Valkey | `ew:<module>:principal:<user_id>:<tenant_id>` (tenant rỗng = `platform`) | JSON `Principal` (schema `Principal` của identity internal) + `version` | `principal_cache_ttl` | Prefix theo ACL module ([VALKEY_DESIGN](../../docs/07-backend/VALKEY_DESIGN.md)) |
| JWKS | bộ nhớ, theo `kid` | khóa công khai | làm mới nền | Không lưu Valkey |

Policy RLS do `tenancy.RLSPolicies` sinh (khớp DATABASE_DESIGN §4):

| Loại bảng | Policy | Điều kiện |
|---|---|---|
| T, T\*, TA\* | `tenant_read` / `tenant_insert` / `tenant_update` (TA\*: không `tenant_update`) | Chỉ `tenant_id = infra.current_tenant_id()` (không có `deleted_at`, ADR-0089 §1); không policy DELETE; `ENABLE` + `FORCE ROW LEVEL SECURITY` |
| TA | `tenant_read` / `tenant_insert` | `tenant_id = infra.current_tenant_id()` |
| H | `content_read` | dòng tenant hoặc `tenant_id IS NULL` có assignment active trong `infra.tenant_resource_assignments`; không có `deleted_at` |
| T, T\*, H (view) | `<schema>.<table>_live` | `WHERE deleted_at IS NULL`, `security_invoker`; `GRANT SELECT` cho `<module>_readonly`, không quyền trên bảng gốc |

Bảng G/GA và bảng hạ tầng `infra.*` (outbox, processed, idempotency, dead letters) không bật RLS (ADR-0089 §4). Policy cho `<module>_app` giữ điều kiện tenant thuần; dòng `tenant_id IS NULL` của T\*/TA\*/H chỉ ghi bằng `<module>_platform` dưới `PlatformScope` (ADR-0089 §2).

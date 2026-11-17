# Data Model: F059 — Workspace D — người dùng/role platform, catalog permission, audit toàn hệ thống, login events liên tenant, feature flags, màn tích hợp

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F059 là frontend, không tạo hay sửa bảng. Role, UserRole, User (`identity.*`), audit, feature flag thuộc module identity (F012, F015); login events (`game.login_events`) thuộc module game (F014); SIS connection thuộc module identity (F011, F017).

Trạng thái phía client (không bền vững):

| Trạng thái | Nơi giữ | Ghi chú |
|---|---|---|
| Bộ lọc tenant | Của F057 (`sessionStorage`, per-viewer) | Dùng chung mọi màn F059 |
| Bộ lọc màn (khoảng thời gian, action, user) | URL query string | Chia sẻ link nội bộ; không chứa PII ngoài `user_id` |
| ETag role / feature flag | RAM (TanStack Query) | Gửi `If-Match` khi sửa |
| `Idempotency-Key` mutation | RAM, một khóa mỗi lần bấm | Giữ khi thử lại |

Schema dùng (chỉ đọc định nghĩa): `Permission`, `PermissionList`, `PermissionScopeKind`, `PermissionCode`, `Role`, `RoleInput`, `RolePage`, `RoleCode`, `UserRole`, `UserRoleGrantRequest`, `UserRoleScopeType`, `UserRolePage`, `User`, `UserPage`, `TenantUserCreateRequest`, `AuditLogEntry`, `AuditLogPage`, `LoginEvent`, `LoginEventPage`, `LoginEndReason`, `SisConnectionSchool`, `SyncRun`, `TenantSummary` trong `specs/openapi/eduworld-api.v1.yaml`; sau delta: `PlatformUserCreateRequest`, `FeatureFlag`, `FeatureFlagList`, `FeatureFlagUpdate`, `IntegrationOverview`, `SisConnectionPage`, `SyncRunPage`.

# Data Model: F122 — Portal an toàn chat: kiểm duyệt (E), danh mục và công tắc (D), chính sách chat trường (C)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F122 là frontend, không tạo hay sửa bảng. Dữ liệu case, sanction, thông báo trường (F121), danh mục từ, công tắc (F116), chính sách chat, club (F120) thuộc module chat.

Trạng thái phía client (không bền vững):

| Trạng thái | Nơi giữ | Ghi chú |
|---|---|---|
| Bộ lọc tenant của hàng đợi case | `sessionStorage` khóa `ew.portal.moderation.tenantFilter.<userId>` (mảng `tenant_id`) | Per-viewer; bọc try/catch; xóa khi đăng xuất |
| Trạng thái kiểm duyệt cho banner | RAM (TanStack Query, `refetchInterval` 30 s) | Không lưu browser storage |
| Nội dung case, term, thử chuỗi | RAM, xóa khi rời màn | Không ghi `localStorage`/`sessionStorage` (R14) |
| Bản nháp form chính sách chat | RAM | Cảnh báo khi rời trang chưa lưu (F051 `unsavedChangesGuard`) |

Schema dùng (chỉ đọc định nghĩa): `ModerationCase`, `ModerationCaseDetail`, `ModerationActionRequest`, `Sanction`, `SanctionCreate`, `SchoolSafetyNotification`, `SchoolSafetyNotificationCreate`, `SchoolSafetyNotificationAck`, `SensitiveTermCategory(Input)`, `SensitiveTerm(Input)`, `SensitiveAllowlistEntry`, `SensitiveTermTestRequest/Result`, `SensitiveCatalogPublishRequest`, `SensitiveCatalogVersion`, `ModerationSwitchUpdate`, `ModerationSwitchState`, `ModerationSwitchChange`, `TenantSensitiveCategory`, `Club`, `ClubStatus` trong `specs/openapi/eduworld-api.v1.yaml`; sau delta: `ModerationCaseMessage` (F121), `ChatPolicyDocument`/`EffectiveChatPolicy` (F120), `TenantModerationStatus` (F122).

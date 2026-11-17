# Implementation Plan: F122 — Portal an toàn chat: kiểm duyệt (E), danh mục và công tắc (D), chính sách chat trường (C)

**Branch**: `122-portal-chat-safety` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Xây các màn an toàn chat trong ba chunk workspace của portal (F051): `live-ops/moderation` (hàng đợi case, chi tiết case + bằng chứng, sanction, thông báo nhà trường, từ bổ sung/nhóm từ tenant), `platform-admin/chat-safety` (danh mục từ nhạy cảm, thử chuỗi, publish/version, công tắc kiểm duyệt + lịch sử), `school-lms/safety` (thông báo vụ việc + xác nhận, chính sách chat trường/lớp, nhóm từ chỉ xem, club). Banner "Đang tắt kiểm duyệt" gắn vào `BannerSlot`. Mỗi màn = page Ant Design Pro + hook TanStack Query gọi operationId qua `apiClient` (F050) với `withTenantScope`/`TenantFilter` (F051). Không business logic ở client.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript strict (React 18, Vite) |
| Primary Dependencies | Ant Design 5 + Pro (ProTable, ProForm, StepsForm, Drawer, Descriptions, Timeline), TanStack Query, openapi-typescript/openapi-fetch (F050), React Hook Form + Zod (rule sinh từ OpenAPI), i18next |
| Storage | Không (cache RAM TanStack Query; không lưu nội dung term/case vào browser storage) |
| Testing | Vitest + Testing Library + MSW (test-only, nhãn `MOCK`), Playwright E2E với backend compose (gateway public + nội bộ), axe |
| Target Platform | Trình duyệt desktop/tablet; điện thoại cho thông báo trường và xử lý case nhanh |
| Project Type | web app |
| Performance Goals | Hàng đợi case tải trang đầu ≤ 2 s; banner poll 30 s (≤ 60 s hiệu lực) |
| Constraints | AGENTS R1, R3, R10, R14, R15, R20, R32; ADR-0020, ADR-0027, ADR-0030, ADR-0047, ADR-0072 |
| Scale/Scope | Phân trang cursor mọi danh sách; nhiều tenant |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `getTenantModerationStatus` (Phase 2); phụ thuộc delta F120/F121 ghi rõ |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ gọi API public/nội bộ qua APISIX; không logic nghiệp vụ |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Ghi luôn `X-Tenant-Id` tenant đích/tenant case; người dùng tenant không gửi `tenant_ids`; không có xóa vĩnh viễn |
| IV Server-authoritative | ✅ | SLA, hạn xác nhận, quyết định thử chuỗi do server trả |
| V Xác thực, phân quyền | ✅ | Menu theo `getMyPermissions`; 403 page; MFA step-up; ma trận E2E |
| VI An toàn trẻ em | ✅ | Không màn nào hiển thị chat ngoài case; thông báo trường không có nội dung chat; không log term/tin; không control theo khối |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn độ dài từ schema; URL gateway từ env |
| VIII UI tách logic, token | ✅ | Theme `admin-tokens`; màu trạng thái từ `shared-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F122-AC có test viết trước |

## Project Structure

### Documentation (feature này)

```text
specs/122-portal-chat-safety/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/cms/frontend/apps/portal/src/
├── shell/banners/ModerationDisabledBanner.tsx           # đăng ký vào BannerSlot (F051)
├── shared/chatSafety/{useModerationStatus.ts,problemMessages.ts}
├── workspaces/live-ops/moderation/
│   ├── routes.tsx                                      # /live-ops/moderation/{cases,cases/:id,notifications,tenant-terms,tenant-categories}
│   ├── api/{cases.ts,sanctions.ts,schoolNotifications.ts,tenantTerms.ts}
│   ├── cases/{CaseQueuePage.tsx,CaseDetailPage.tsx,EvidencePanel.tsx,CaseActionBar.tsx,SanctionForm.tsx,NotifySchoolDrawer.tsx}
│   ├── notifications/PlatformNotificationsPage.tsx
│   ├── tenant/{TenantTermsPage.tsx,TenantCategoriesPage.tsx}
│   └── i18n/{vi,en}.json
├── workspaces/platform-admin/chat-safety/
│   ├── routes.tsx                                      # /platform-admin/chat-safety/{catalog,switches,switches/history}
│   ├── api/{catalog.ts,switches.ts}
│   ├── catalog/{CategoriesPage.tsx,TermsPage.tsx,AllowlistPage.tsx,PublishDrawer.tsx,VersionsPage.tsx}
│   └── switches/{SwitchesPage.tsx,SwitchConfirmModal.tsx,SwitchHistoryPage.tsx}
├── workspaces/school-lms/safety/
│   ├── routes.tsx                                      # /school-lms/safety/{notifications,chat-policy,categories,clubs}
│   ├── api/{notifications.ts,chatPolicy.ts,categories.ts,clubs.ts}
│   └── {NotificationsPage.tsx,NotificationDetail.tsx,ChatPolicyPage.tsx,ChannelPolicyCard.tsx,CategoriesPage.tsx,ClubsPage.tsx}
src/cms/frontend/packages/ops-ui/src/chat-safety/{SensitiveTermTester.tsx,SlaBadge.tsx,AlwaysOnLock.tsx}   # + stories
src/cms/frontend/apps/portal/tests/e2e/chat-safety/*.spec.ts
```

**Structure Decision**: Theo F051 (`src/workspaces/<slug>`, lazy-load); `SensitiveTermTester`, `SlaBadge`, `AlwaysOnLock` đặt ở `ops-ui` vì DESIGN_SYSTEM §4.3 liệt kê "trình xem danh mục + thử chuỗi" là trình soạn đặc thù dùng lại.

## Phase 0: Research

Không có. Màn hình ở [ADMIN_PORTAL §1.5.2, §3.9, §10.1](../../docs/04-admin-portal/ADMIN_PORTAL.md), nghiệp vụ ở [MODERATION](../../docs/03-social-multiplayer/MODERATION.md), mẫu UI ở [DESIGN_SYSTEM §4.3](../../docs/05-ux/DESIGN_SYSTEM.md).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | — | Không có |
| application | Hook `api/*.ts`, `useModerationStatus` | Mutation sinh `Idempotency-Key` một lần/thao tác; `If-Match` từ ETag; không tự retry ghi |
| infrastructure | `apiClient` (F050), base URL `VITE_API_PUBLIC_BASE_URL` / `VITE_API_INTERNAL_BASE_URL` | Lỗi mạng nội bộ → `VpnRequiredNotice` (F051) |
| presentation | Page gọi: `listModerationCases`, `getModerationCase`, `listModerationCaseMessages`, `assignModerationCase`, `actOnModerationCase`, `notifySchoolForCase`, `listSanctions`, `createSanction`, `revokeSanction`, `listSchoolSafetyNotificationsPlatform`, `listSchoolSafetyNotifications`, `acknowledgeSchoolSafetyNotification`, `listSensitiveCategories`, `createSensitiveCategory`, `updateSensitiveCategory`, `deleteSensitiveCategory`, `listSensitiveTerms`, `createSensitiveTerm`, `updateSensitiveTerm`, `deleteSensitiveTerm`, `testSensitiveTerms`, `listSensitiveAllowlist`, `createSensitiveAllowlistEntry`, `deleteSensitiveAllowlistEntry`, `publishSensitiveCatalog`, `listSensitiveCatalogVersions`, `listModerationSwitches`, `setModerationSwitches`, `listModerationSwitchHistory`, `listTenantSensitiveTerms`, `createTenantSensitiveTerm`, `updateTenantSensitiveTerm`, `deleteTenantSensitiveTerm`, `publishTenantSensitiveCatalog`, `listTenantSensitiveCategories`, `toggleTenantSensitiveCategory`, `listClubs`, `approveClub`, `archiveClub`, `removeClubMember`; sau delta: `getTenantModerationStatus`, `getChatPolicy`, `putChatPolicy`, `getEffectiveChatPolicy` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F051 (shell, `BannerSlot`, `TenantFilter`, `TenantTargetField`, step-up, VPN), F050, F116 (API danh mục/công tắc), F120 (chính sách chat, club), F121 (case, sanction, thông báo).
- Dùng bởi: F056 (widget thông báo an toàn chờ xác nhận có thể link tới màn C).

### Bảo mật, tenant, idempotency, audit

- Permission: `moderation.case.read/act`, `moderation.case.messages.read`, `moderation.sanction.read/write`, `moderation.school_notification.send/read/acknowledge`, `chat.sensitive_terms.read/write`, `chat.policy.toggle`, `chat.tenant_terms.write`, `chat.tenant_category.toggle`, `school.settings.read/write`, `social.club.approve`.
- Workspace E/D: token MFA, gateway nội bộ; `TenantFilter` trong hàng đợi case chỉ cho Safety Moderator/System Admin (không phải `platform.cross_tenant.read` toàn portal).
- Idempotency-Key cho mọi POST ghi; audit ghi ở backend.

### Quan sát

- Log client chỉ `code`, `trace_id`, route; không nội dung term, tin, tóm tắt, tên học sinh.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.live_ops.moderation_cases`, `menu.live_ops.sanctions`, `menu.live_ops.tenant_terms`, `menu.platform_admin.chat_safety`, `menu.platform_admin.moderation_switches`, `menu.school_lms.safety_notifications`, `menu.school_lms.chat_safety`, `menu.school_lms.clubs` |
| Action dùng | `action.live_ops.moderation_cases.view`, `action.live_ops.moderation_cases.act`, `action.live_ops.moderation_cases.notify_school`, `action.live_ops.sanctions.view`, `action.live_ops.sanctions.write`, `action.live_ops.tenant_terms.view`, `action.live_ops.tenant_terms.edit`, `action.live_ops.tenant_terms.toggle_category`, `action.platform_admin.chat_safety.view`, `action.platform_admin.chat_safety.edit`, `action.platform_admin.chat_safety.test`, `action.platform_admin.chat_safety.publish`, `action.platform_admin.moderation_switches.view`, `action.platform_admin.moderation_switches.toggle`, `action.school_lms.safety_notifications.view`, `action.school_lms.safety_notifications.acknowledge`, `action.school_lms.chat_safety.view`, `action.school_lms.chat_safety.edit`, `action.school_lms.clubs.view`, `action.school_lms.clubs.approve` |
| Không tắt được | `menu.live_ops.moderation_cases`, `menu.live_ops.sanctions`, `menu.school_lms.safety_notifications` không tắt được (ADR-0090 mục 2) |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `getChatPolicy`, `getEffectiveChatPolicy`, `putChatPolicy` (Cần thêm trong OpenAPI) → `action.school_lms.chat_safety.view` / `action.school_lms.chat_safety.edit` (action ghi đã có trong catalog, UI-only tới khi có operation)
- `approveModerationSwitchChange`, `getTenantModerationStatus` (Cần thêm) → đề xuất `action.platform_admin.moderation_switches.toggle` / `view`
- ~~Duyệt club chưa có menu~~ — đã đóng: `menu.school_lms.clubs` (`social.club.approve`): `listClubs`, `getClub` → `action.school_lms.clubs.view`; `approveClub`, `archiveClub` → `action.school_lms.clubs.approve`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|

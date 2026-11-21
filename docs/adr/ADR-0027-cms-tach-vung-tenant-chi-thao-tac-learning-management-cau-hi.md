# ADR-0027 — Admin Portal: một portal 5 workspace; tenant chỉ thao tác Learning Management, cấu hình game chỉ thuộc platform

> **Trạng thái:** Accepted · một phần superseded by ADR-0072 · một phần superseded by [ADR-0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md) · một phần superseded by ADR-0095 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md), [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md), [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md).** Không còn hiệu lực: giáo viên và moderator tenant trong workspace School & Learning Management (mục 1), giáo viên chọn quest/mini-game để giao bài (mục 5) — nay do `school_admin`/`academic_manager`; kiểm duyệt chuyển sang Platform Safety Team. **Còn hiệu lực:** một portal 5 workspace, tenant chỉ thao tác Learning Management, cấu hình game chỉ thuộc platform.

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)** (owner bỏ MFA/TOTP cho đăng nhập Admin Portal, 2026-11-21). Không còn hiệu lực: mục 3(c): token phải có MFA (`acr` mức MFA) và Keycloak bắt buộc OTP cho role platform; phần "bảo vệ bằng MFA" ở Hệ quả. **Còn hiệu lực:** mục 3(a) permission backend, (b) gateway nội bộ/VPN, role platform bắt buộc trên route platform, (d) lazy chunk không phải lớp bảo mật; toàn bộ phần còn lại.

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md):** workspace School & Learning Management không còn quản lý lớp (thay bằng học sinh theo khối); "thử thách lớp" (mục 5) đổi thành thử thách khối từ template. **Còn hiệu lực:** toàn bộ phần còn lại.

## Bối cảnh

Stakeholder yêu cầu: vận hành của mỗi tenant vào Admin Portal thao tác được nhưng chỉ trên chức năng Learning Management, không được chạm menu và cấu hình game; System Admin xem được mọi tenant; dùng **một portal duy nhất** (audit theo Master Prompt mới, C6).

## Quyết định

1. **Một Admin Portal duy nhất** (`web/apps/portal`, client Keycloak `eduworld-portal`, PKCE) với workspace switcher, sidebar theo permission, breadcrumb, trạng thái lưu/Draft/Published, lịch sử version. **5 workspace:**

   | Workspace | Người dùng | Phạm vi | API chính | Gateway |
   |---|---|---|---|---|
   | **School & Learning Management** | Tenant (Tenant Admin, School Admin, giáo viên, tổ chuyên môn, moderator tenant), System Admin | Lớp, học sinh, giao bài, tiến độ, báo cáo, SIS/CSV, cài đặt trường, An toàn & Kiểm duyệt của tenant | `/api/v1/lms/*`, `/api/v1/tenant/*`, `/api/v1/schools/*`, `/api/v1/chat/moderation/*` | Public |
   | **Learning Content Studio** | Biên tập/duyệt nội dung của tenant và platform | Curriculum, bài học, activity, question bank, đề, media, version/publish | `/api/v1/cms/learning/*` | Public |
   | **Game Operations Console** | Chỉ platform (game designer, live-ops, competition admin, System Admin) | World/zone/NPC, quest, challenge/boss, level & XP, achievement, item/shop/currency, event/season, competition, leaderboard, sticker/GIF | `/api/v1/game-admin/*` | **Nội bộ/VPN** |
   | **Game Live Operations** | Chỉ platform | CCU, sự kiện đang chạy, matchmaking, phiên chơi, sự cố, moderation trực tiếp (Trust & Safety) | `/api/v1/admin/*` (liveops) | **Nội bộ/VPN** |
   | **Platform Administration** | Chỉ System Admin | Tenant, user/role/permission, chính sách, Keycloak/SIS/APISIX tham chiếu, feature flag, audit, data retention, Thùng rác, gán/clone | `/api/v1/admin/*`, `/api/v1/sis/*` | **Nội bộ/VPN** |

2. **Permission model.** Authorization dùng permission (vd. `lms.assignment.write`, `learning_content.question.write`, `game.quest.write`, `competition.config.write`); role là tập permission. Catalog định nghĩa trong code, kiểm tra lúc khởi động: **role tenant không bao giờ chứa `game.*`, `competition.config.*`, `platform.*`**. Gán role vi phạm bị từ chối và ghi audit. Kiểm tra ở handler + use case.
3. **Phòng thủ nhiều lớp cho cấu hình game và platform:** (a) permission backend; (b) route `/api/v1/game-admin/*`, `/api/v1/admin/*`, `/api/v1/sis/*` chỉ publish trên gateway nội bộ/VPN; (c) các route đó yêu cầu token có **MFA** (claim `acr` mức MFA; Keycloak bắt buộc OTP cho role platform) và role platform; (d) mã workspace platform được tách thành **lazy-loaded chunk**, chỉ tải khi có permission — tối ưu hiển thị, **không phải lớp bảo mật**.
4. **Menu theo permission:** portal gọi `GET /api/v1/me/permissions` (trả `workspaces[]` và permission) để dựng menu; ẩn menu chỉ là tiện ích, backend vẫn quyết định.
5. **Tenant dùng nội dung game, không sửa cấu hình:** giáo viên **chọn** quest/mini-game/thử thách đã publish và được gán để giao bài; tạo "thử thách lớp/giải đấu trường" từ **template của platform** (chỉ đặt tên, lớp, lịch, bộ câu hỏi); không sửa luật chơi, phần thưởng, matchmaking/ranking [Confirmed].
6. **Chế độ liên tenant chỉ dành cho System Admin:**
   - Màn hình có dữ liệu theo tenant có **dropdown tìm kiếm tenant** (mặc định "Tất cả tenant", chọn nhiều) và cột "Tenant".
   - API danh sách nhận `tenant_ids[]` chỉ khi principal là System Admin; người dùng tenant luôn bị khóa vào tenant của phiên.
   - Thao tác ghi luôn chỉ định **một tenant đích** (hoặc phạm vi platform); không có ghi hàng loạt "tất cả tenant" ngầm định.
   - Mọi truy cập liên tenant ghi audit.

## Hệ quả

(+) Cấu hình game nhất quán toàn hệ thống; tenant không làm lệch luật chơi, phần thưởng, công bằng thi đấu; một portal cho mọi người dùng quản trị. (−) Mọi thay đổi game phải qua đội platform; workspace platform cần VPN; mất lớp bảo vệ "bundle tách biệt", thay bằng MFA + gateway nội bộ.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted: tách vùng Learning Management / Game Studio & Platform, 2 bundle `cms`/`admin` | Stakeholder |
| 2026-11-15 | Thêm chế độ liên tenant cho System Admin (dropdown tenant) | Stakeholder |
| 2026-11-15 | Một portal duy nhất 5 workspace; client `eduworld-portal`; lớp (d) đổi thành lazy chunk; bảo vệ bằng MFA + gateway nội bộ | Stakeholder (audit C6) |
| 2026-11-15 | Một phần superseded by ADR-0072 (bỏ vai trò giáo viên) | Technical Architect |
| 2026-11-21 | Một phần superseded by ADR-0094 (bỏ MFA/TOTP cho đăng nhập Admin Portal) | Owner dự án |
| 2026-11-21 | Một phần superseded by ADR-0095 (bỏ khái niệm lớp — học sinh theo khối) | Owner |

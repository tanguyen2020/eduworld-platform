# Admin Portal — CMS/LMS cho vận hành game

> Trạng thái: Draft · Phiên bản: 0.8 (một Admin Portal, 5 workspace; nhà trường quản trị, kiểm duyệt tập trung — ADR-0072) · Owner: Product Manager + Backend Architect + UX Architect · Cập nhật: 2026-11-17 (màn đăng nhập riêng qua backend — ADR-0091; danh mục menu/action và bật/tắt trong DB — ADR-0090)
> Liên quan: [ADR-0027](../adr/ADR-0027-cms-tach-vung-tenant-chi-thao-tac-learning-management-cau-hi.md) · [ADR-0005](../adr/ADR-0005-mo-hinh-multi-tenancy-shared-database-tenant-id-rls.md) · [ADR-0026](../adr/ADR-0026-dang-nhap-theo-tenant-xac-dinh-tenant-kiem-tra-cheo-mac-dinh.md) · [ADR-0028](../adr/ADR-0028-chi-soft-delete-khong-hard-delete-mac-dinh-khong-doc-du-lieu.md) · [ADR-0029](../adr/ADR-0029-system-admin-gan-du-lieu-cho-tenant-va-clone-du-lieu-giua-te.md) · [ADR-0022](../adr/ADR-0022-cms-lms-web-la-react-typescript-spa.md) · [ADR-0015](../adr/ADR-0015-content-versioning-bat-bien-sau-publish.md) · [ADR-0047](../adr/ADR-0047-hai-he-thong-giao-dien-game-godot-3d-va-admin-ops-cms-lms.md) · [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) · [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [SCHOOL_LMS](SCHOOL_LMS.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) · [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) · [ADR-0090](../adr/ADR-0090-danh-muc-menu-action-portal-bat-tat-trong-db.md) · [ADR-0091](../adr/ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) · [OQ-07](../12-project/OPEN_QUESTIONS.md)

## 1. Định vị

- **[Confirmed]** CMS quản lý **nội dung** (curriculum, nội dung học tập, nội dung game, cấu hình thi đấu). LMS quản lý **hoạt động học tập** (giao bài, theo dõi). Backend: nội dung học tập và LMS thuộc **module learning**; Game Studio (quest, mini-game, NPC, item, reward) thuộc **module game**; cấu hình thi đấu thuộc **module competition**; sticker/GIF và danh mục từ nhạy cảm thuộc **module chat**; tenant, người dùng, phân quyền, Thùng rác/gán/clone điều phối ở **module identity** (ADR-0036). SPA gọi qua APISIX, gateway định tuyến theo prefix tới module sở hữu.
- **[Confirmed — ADR-0005]** Tenant là một **trường độc lập** (`tenant_type = school`), một **đơn vị nhiều trường** (`unit`) hoặc **tenant chung** (`common`). Khóa cô lập dữ liệu là `tenant_id`; `school_id` là chiều phân loại bên trong tenant.
- **[Confirmed — ADR-0027 mục 7]** Giao diện quản trị là **một Admin Portal duy nhất** (`web/apps/portal`, một ứng dụng React, một Keycloak client confidential `eduworld-portal` chỉ dùng qua module identity — ADR-0091) gồm **5 workspace** (§1.2). Người dùng chỉ thấy workspace và menu mà permission cho phép và đang bật (§2.7, ADR-0090). Mô hình 2 bundle `cms`/`admin` trước đây đã bị thay thế.
- Hai nhóm người dùng và phạm vi dữ liệu không đổi:

| Nhóm | Người dùng | Workspace thấy được | Phạm vi dữ liệu | Gateway gọi API |
|---|---|---|---|---|
| **Vận hành tenant** | Tenant Admin, School Admin, Academic Manager (tổ chuyên môn), biên tập/duyệt nội dung của tenant | School & Learning Management, Learning Content Studio | **Đúng một tenant** (tenant của phiên) | Public (`/api/v1/cms/learning/*`, `/api/v1/lms/*`, `/api/v1/schools/*`, `/api/v1/tenant/*`) |
| **Đội EduWorld (platform)** | **System Admin** (người duy nhất xem dữ liệu liên tenant đầy đủ), Content Admin/Reviewer platform, Game Designer, Live-ops, Competition Admin, **Safety Moderator** (Platform Safety Team) | Theo role: cả 5 workspace với System Admin; Game Operations Console / Game Live Operations với đội game; Learning Content Studio (dữ liệu platform) với Content Admin; module Kiểm duyệt (workspace E) với Safety Moderator | System Admin: mọi tenant (dropdown chọn tenant, §2.5). Safety Moderator: liên tenant **chỉ trong module moderation** (§2.5b). Các role platform khác: chỉ dữ liệu platform (`tenant_id = NULL`) | Workspace chỉ dành cho platform gọi **gateway nội bộ/VPN** (`/api/v1/game-admin/*`, `/api/v1/admin/*`); token phải có **MFA** (`acr`) |

- **[Confirmed — ADR-0027]** Vận hành tenant **không thấy và không thao tác** menu, API hay cấu hình game: quest, mini-game config, NPC dialogue, map/zone/world event, item/cosmetic, reward rule, XP/level, matchmaking, ranking rule, season, tournament template, leaderboard definition, feature flag game.
- Tenant vẫn **dùng** được nội dung game đã publish và được gán cho tenant (ADR-0029): nhà trường (School Admin, Academic Manager) chọn quest/mini-game để giao bài; School Admin tạo thử thách lớp/giải trường **từ template do platform định nghĩa** (§3.6). Không sửa luật chơi, phần thưởng hay cấu hình thi đấu.
- **[Confirmed — ADR-0028]** Mọi thao tác "xóa" trong CMS là **soft-delete**; danh sách mặc định không hiển thị dữ liệu đã xóa; xem/khôi phục chỉ qua **Thùng rác** (§3.8).
- **[Confirmed — ADR-0029]** System Admin tạo dữ liệu dùng chung rồi **gán** cho tenant (chỉ đọc với tenant), và **clone** dữ liệu cấu hình/nội dung từ tenant này sang tenant khác (§3.7).
- Module Go `cms` điều phối workflow/review/preview. Dữ liệu nằm trong schema của module sở hữu (`curriculum`, `content`, `game`, `competition`, `gamification`).
- **[Confirmed]** CMS không được làm thay đổi lịch sử bài tập đã hoàn thành (§9).
- **[Confirmed — ADR-0072]** Không có vai trò giáo viên. Học sinh tự học trong game; nhà trường quản trị ở cấp trường qua workspace C ([SCHOOL_LMS](SCHOOL_LMS.md)); mọi hoạt động do server tự chấm, không có hàng đợi chấm tay; kiểm duyệt chat do **Platform Safety Team** (`safety_moderator`) xử lý tập trung, nhà trường chỉ nhận thông báo vụ việc nghiêm trọng (§3.9).

### 1.1. Giao diện: Admin/Ops UI, không theo phong cách game (ADR-0047)

- **[Confirmed — ADR-0047]** Toàn bộ Admin Portal (mọi workspace) dùng **Admin/Ops design system** dành cho vận hành game: gọn, trung tính, dày dữ liệu. **Không** dùng phong cách, component hay Theme của game. Chi tiết: [DESIGN_SYSTEM §4](../05-ux/DESIGN_SYSTEM.md#4-adminops-design-system-cmslmsadmin).
- **[Proposed]** Thư viện: **Ant Design 5** + Ant Design Pro components (ProLayout, ProTable, ProForm, QueryFilter, StatisticCard), theme EduWorld qua `ConfigProvider` sinh từ `design/admin-tokens/` (màu thương hiệu, compact, sáng/tối, locale `vi_VN`). Component đặc thù trong `web/packages/ops-ui`: trình soạn 11 dạng câu hỏi, graph editor quest/NPC dialogue, bracket giải đấu, thư viện media có thumbnail 3D, `GamePreviewFrame`.
- **Ánh xạ mẫu màn → component:** danh sách = QueryFilter (có dropdown tenant cho System Admin) + ProTable + thao tác hàng loạt; chi tiết = Tabs (Thông tin/Phiên bản/Phát hành/Sử dụng/Audit) + Timeline; form = ProForm/StepsForm với validation sinh từ OpenAPI (`maxLength`, `required`); dashboard = StatisticCard + chart (ECharts hoặc Ant Design Charts).
- Layout: ProLayout, sidebar menu **sinh từ danh mục** `specs/portal/menu-catalog.yaml` (§2.7) và lọc theo `GET /api/v1/me/permissions` (permission + `disabled_menus`/`disabled_actions`); header có bộ lọc tenant (System Admin, chọn nhiều) hoặc bộ chọn đơn vị làm việc (nhân sự thuộc nhiều tenant, §1.3b), trạng thái hệ thống, thông báo; banner cảnh báo hệ thống (ví dụ "Đang tắt kiểm duyệt", ADR-0030).
- Màu môn học, độ hiếm, trạng thái nội dung dùng **mã màu nghiệp vụ dùng chung** với game (`design/shared-tokens/`), hiển thị bằng `Tag`.
- Ảnh nhân vật/item/NPC/khu vực là **thumbnail render từ tài sản 3D của game** (DESIGN_SYSTEM §7), hiển thị như dữ liệu nhận diện. Chỉ bên trong khung preview Godot Web (§5) mới mang giao diện game.

### 1.2. Admin Portal: 5 workspace (ADR-0027 mục 7, Master Prompt tái cấu trúc §16)

| Workspace | Ai thấy | Gateway | Module (✅ đã có đặc tả chi tiết · 🆕 đặc tả mới lượt này · [TBD] để lượt sau) |
|---|---|---|---|
| **A. Game Operations Console** | Chỉ platform: Game Designer, Live-ops, Competition Admin, System Admin | Nội bộ + MFA | Game Dashboard [TBD] · Game World / Zone / Area ✅ (§3.3) · NPC Management ✅ · Quest Builder ✅ ([QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md)) · **Challenge Builder** 🆕 · **Boss Management** 🆕 · **Level & XP Configuration** 🆕 · **Achievement Builder** 🆕 · **Item / Inventory Catalog / Shop / Currency** 🆕 (§3.12) · Reward Configuration ✅ · **Event Builder & Season Track** 🆕 (§3.15, [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md)) · Player Management [TBD] · Party & Group Management [TBD] · Competition Management ✅ (§3.4) · Leaderboard Configuration ✅ · Sticker/GIF ✅ · Game Analytics [TBD] |
| **B. Learning Content Studio** | Tenant (dữ liệu tenant) + Content Admin/Reviewer platform (dữ liệu platform) + System Admin | Public (tenant) / nội bộ (dữ liệu platform) | Curriculum · Grade · Subject · Topic · Learning Objective · Learning Unit ✅ (§3.1) · Lesson Builder ✅ (§3.2) · **Learning Activity Builder theo activity type** 🆕 (§3.13) · Question Bank / Quiz / Exercise Builder ✅ · **Video Manager** 🆕 · **Audio Manager** 🆕 · **Simulation Configuration** 🆕 · Assessment Builder ✅ · Learning Path Builder 🟡 ([SCHOOL_LMS](SCHOOL_LMS.md)) · Preview ✅ (§5) · Versioning / Approval / Publish ✅ (§3.11, §4, §6) |
| **C. School & Learning Management** | Tenant + System Admin (dropdown tenant) | Public | School · Class · Student Directory ✅ · SIS Synchronization ✅ · Course & Learning Path ✅ · Learning Assignment ✅ · **School Learning Dashboard** (trường → lớp → học sinh) ✅ ([SCHOOL_LMS §8](SCHOOL_LMS.md#8-school-learning-dashboard)) · Learning Analytics ✅ · Assessment Results & báo cáo tham khảo TT27/TT22 ✅ · Reports ✅ · Thông báo an toàn của trường ✅ (§3.9) · Hoạt động học sinh ✅ (§3.10) · Cài đặt tenant/trường (gồm khung giờ truy cập) ✅ |
| **D. Platform Administration** | Chỉ System Admin | Nội bộ + MFA | User & Role ✅ · Permission (catalog, chỉ xem) ✅ · Tenant Management ✅ · Gán / Clone (ADR-0029) ✅ · School Policies ✅ · Keycloak / SIS / APISIX (tham chiếu cấu hình) ✅ · System Configuration (dọn học sinh, retention) ✅ · Audit Logs ✅ · Data Retention ✅ · Thùng rác ✅ · Feature Flags ✅ · Integration Monitoring [TBD] · Danh mục từ nhạy cảm + Công tắc kiểm duyệt ✅ (§3.9) |
| **E. Game Live Operations** | Chỉ platform (Live-ops, System Admin; Safety Moderator chỉ thấy module Kiểm duyệt) | Nội bộ + MFA | Online Players / CCU · Concurrent Sessions · Active Events · World Health · Matchmaking Status · Party Status · Game Session Status · **Kiểm duyệt — Platform Safety Team** ✅ (§3.9: hàng đợi case liên tenant, bằng chứng, sanction, từ bổ sung theo yêu cầu tenant, thông báo nhà trường) · Incident Management · Server Health · Operational Metrics: dashboard theo [DESIGN_SYSTEM §4](../05-ux/DESIGN_SYSTEM.md#4-adminops-design-system-cmslmsadmin) (KPI live-ops) và [MONITORING](../11-operations/MONITORING.md); màn hình chi tiết [TBD] |

- Tenant **không bao giờ** thấy workspace A, D, E; trong B và C chỉ thấy dữ liệu tenant mình + dữ liệu platform được gán (chỉ đọc).
- Safety Moderator chỉ thấy module Kiểm duyệt của workspace E; không thấy workspace A, B, C, D.
- Mã nguồn mỗi workspace là một **chunk lazy-load** trong `web/apps/portal/src/workspaces/<thư mục>`, tên thư mục cố định: A `game-ops`, B `content-studio`, C `school-lms`, D `platform-admin`, E `live-ops`; chỉ tải khi `GET /api/v1/me/permissions` trả về workspace đó. Đây là tối ưu tải trang, **không phải lớp bảo mật**: bảo mật dựa vào permission backend, gateway nội bộ và MFA (ADR-0027 mục 3).
- Workspace A, D, E gọi API chỉ có ở gateway nội bộ. Người dùng platform ngoài VPN mở portal vẫn thấy menu nhưng mọi thao tác trả lỗi kết nối; portal hiển thị thông báo "Cần kết nối mạng quản trị (VPN)" thay vì lỗi chung [Proposed].

### 1.3. Information Architecture (bố cục chung)

```text
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│ [Logo] [Workspace ▾] [🔍 Tìm kiếm toàn portal]   [Tenant: Tất cả ▾]*  [🔔] [Trạng thái HT] [👤 ▾] │  ← Topbar
├───────────────┬──────────────────────────────────────────────────────────────┬───────────┤
│ Sidebar theo  │ Breadcrumb: Workspace / Module / Mục                          │ Panel     │
│ permission    │ [Thanh trạng thái item: Bản nháp · Đã xuất bản v3 · Đã lưu ✓]│ thuộc tính│
│ (module của   │                                                              │ (tùy chọn)│
│ workspace)    │ Nội dung chính: danh sách / editor / dashboard               │           │
└───────────────┴──────────────────────────────────────────────────────────────┴───────────┘
* chỉ System Admin; người dùng tenant thấy tên tenant cố định (tenant unit: chọn Trường trong đơn vị)
```

| Thành phần | Hành vi |
|---|---|
| **Workspace switcher** | Dropdown các workspace người dùng có quyền; nhớ workspace gần nhất theo người dùng (per-viewer). Phím tắt `Ctrl+K` mở tìm kiếm, `G W` đổi workspace [Proposed] |
| **Tìm kiếm toàn portal** | Tìm theo mã/tên trong phạm vi quyền: học sinh, lớp, câu hỏi, activity, quest, item, tenant (System Admin). Kết quả nhóm theo workspace; không trả mục ngoài tenant/permission |
| **Bộ chọn tenant** | System Admin: bộ **lọc** tenant, dropdown tìm kiếm, mặc định "Tất cả tenant", chọn nhiều (§2.5), áp cho mọi màn có dữ liệu theo tenant. Nhân sự thuộc nhiều tenant: bộ **chọn đơn vị làm việc** (chọn một; đổi = đổi `X-Tenant-Id`, không xin token mới, §1.3b). Nhân sự một tenant: chỉ hiển thị tên đơn vị |
| **Thông báo** | Review chờ duyệt, publish hẹn giờ, job clone/import xong, thông báo vụ việc an toàn (nhà trường) / case mới (Safety Team), sự cố live-ops |
| **Trạng thái hệ thống** | Chấm màu tổng quan (lấy từ health API), banner cảnh báo cố định khi có sự cố hoặc "Đang tắt kiểm duyệt" (ADR-0030) |
| **User menu** | Hồ sơ, đổi ngôn ngữ (vi/en), chế độ sáng/tối, mật độ, đăng xuất |
| **Sidebar** | Menu của workspace hiện tại sinh từ danh mục menu (§2.7), lọc theo permission và trạng thái bật/tắt; nhóm mục có thể thu gọn |
| **Breadcrumb** | Workspace / Module / Mục (/ Phiên bản) |
| **Thanh trạng thái item** | Trạng thái workflow, version đang xuất bản, autosave ("Đang lưu…", "Đã lưu lúc 10:42", "Lỗi lưu, thử lại") |
| **Panel thuộc tính bên phải** | Metadata (LO, khối, môn, độ khó, tag), quyền, lịch sử version, tenant được gán; thu gọn được |
| **Lịch sử version** | Mở từ thanh trạng thái hoặc tab "Phiên bản" (§3.11) |

Responsive: chi tiết ở §1.5.

### 1.3b. Đăng nhập portal và chọn tenant sau đăng nhập (ADR-0091, ADR-0066, ADR-0067)

Admin Portal có **màn đăng nhập riêng do portal tự vẽ** (trang `/login`, Ant Design, theme sinh từ `design/admin-tokens/` — ADR-0047), **không chuyển hướng sang trang Keycloak** (ADR-0091 thay luồng Authorization Code + PKCE của ADR-0066 mục 1). Portal gửi form tới `POST /api/v1/auth/portal/login` (`portalLogin`); module identity gọi token endpoint Keycloak bằng client confidential `eduworld-portal` (Direct Access Grant, cấu hình ở [KEYCLOAK_SSO §3.3](../09-identity/KEYCLOAK_SSO.md)). **Học sinh không đăng nhập được** (chặn hai lớp: flow Keycloak chỉ cho realm role `tenant_staff`/`platform_staff`, và backend `403 PORTAL_ACCESS_STAFF_ONLY`). **Không chọn tenant/trường trước khi đăng nhập**; không có URL theo tenant.

```text
Desktop (≥ 1200 px)                                         Mobile (< 768 px)
┌──────────────────────────────────────────────────────────┐ ┌────────────────────────────┐
│  [Logo EduWorld] Cổng quản trị                  [VI|EN]  │ │ [Logo] Cổng quản trị [VI]  │
│                                                          │ │                            │
│        ┌──────────────────────────────────────┐          │ │ Tên đăng nhập hoặc email   │
│        │ Đăng nhập                            │          │ │ [ ad.truong.a           ]  │
│        │ Tên đăng nhập hoặc email             │          │ │ Mật khẩu                   │
│        │ [ qlcm.truong.a@thdemo.edu.vn    ]   │          │ │ [ ••••••••          👁 ]   │
│        │ Mật khẩu                             │          │ │ Mã xác thực 6 số           │
│        │ [ ••••••••••                  👁 ]   │          │ │ [ _ _ _ _ _ _ ]            │
│        │ Mã xác thực 6 số                     │          │ │ [       Đăng nhập       ]  │
│        │ [ _ _ _ _ _ _ ]                      │          │ │ Quên mật khẩu?             │
│        │ Bắt buộc với tài khoản vận hành      │          │ │                            │
│        │ platform; để trống nếu chưa cài.     │          │ │ Dành cho cán bộ nhà trường │
│        │ [            Đăng nhập           ]   │          │ │ và vận hành. Học sinh vui  │
│        │ Quên mật khẩu?                       │          │ │ lòng đăng nhập trong game. │
│        └──────────────────────────────────────┘          │ └────────────────────────────┘
│  Dành cho cán bộ nhà trường và đội vận hành. Học sinh    │
│  vui lòng đăng nhập trong game EduWorld.                 │
└──────────────────────────────────────────────────────────┘

Đổi mật khẩu tạm (ACCOUNT_SETUP_REQUIRED)      Chọn tenant (chỉ khi thuộc nhiều tenant)
┌──────────────────────────────────────┐     ┌──────────────────────────────────────────┐
│ Đặt mật khẩu mới                     │     │ Chọn đơn vị làm việc                     │
│ Lần đầu đăng nhập, hãy đổi mật khẩu  │     │ 🔍 [ Tìm theo tên/mã           ]         │
│ tạm do quản trị cấp.                 │     │ ( ) Phòng GD Cầu Giấy — Tenant Admin     │
│ Mật khẩu mới [ ••••••••      👁 ]    │     │ ( ) THCS Demo B — Quản trị trường        │
│ Nhập lại     [ ••••••••      👁 ]    │     │ [✓] Nhớ lựa chọn cho lần sau             │
│ (gợi ý chính sách mật khẩu)          │     │ [Đăng xuất]                  [Vào]       │
│ [ Lưu và đăng nhập lại ]             │     └──────────────────────────────────────────┘
└──────────────────────────────────────┘
```

**Form đăng nhập (ADR-0091 mục 1):**

- Một form gửi **một lần** cả tên đăng nhập/email, mật khẩu và mã xác thực 6 số (`otp`, tùy chọn), kèm `device_id` (sinh ngẫu nhiên, lưu per-browser) và `locale`. Không có bước hỏi trước "tài khoản này có cần mã xác thực không" để không lộ tài khoản nào là nhân sự platform.
- Mã xác thực chỉ là **TOTP** từ ứng dụng xác thực; portal không còn khóa bảo mật/WebAuthn.
- Trình duyệt nhớ (per-viewer, không phải bí mật) rằng lần trước đăng nhập có dùng mã xác thực để làm nổi bật ô mã; không nhớ mật khẩu, không nhớ mã.
- Mọi lỗi xác thực (sai tên, sai mật khẩu, sai/thiếu mã với tài khoản bắt buộc MFA, tài khoản bị khóa tạm) hiện **một** thông điệp chung (bảng lỗi bên dưới).
- Giới hạn số lần thử theo username, theo `device_id` và toàn cục (không theo IP — R30): tham số `portal_login_rate` ([SECURITY_ARCHITECTURE §14](../10-security/SECURITY_ARCHITECTURE.md)).

**Mật khẩu lần đầu và đăng ký mã xác thực (ADR-0091 mục 3):** khi Keycloak báo tài khoản còn bước thiết lập (sau khi mật khẩu đã đúng), API trả `403 ACCOUNT_SETUP_REQUIRED` kèm `setup_ticket` (dùng một lần, hạn `portal_setup_ticket_ttl` — [KEYCLOAK_SSO §3.3](../09-identity/KEYCLOAK_SSO.md)) và `required_steps[]`:

| Bước | Màn hình portal | API |
|---|---|---|
| `update_password` | "Đặt mật khẩu mới" (wireframe trên): mật khẩu mới + nhập lại, gợi ý chính sách mật khẩu của realm; vi phạm → lỗi theo field | `POST /api/v1/auth/portal/setup:password` (`setupPortalPassword`) → `remaining_steps[]`, `otp_setup_email_sent`; sau đó về `/login` đăng nhập lại bằng mật khẩu mới |
| `configure_otp` | "Kiểm tra email để cài ứng dụng xác thực": hướng dẫn mở link trong email, quét mã QR bằng ứng dụng xác thực rồi quay lại `/login`; nút "Về trang đăng nhập" | Module identity gửi email `execute-actions-email` (`CONFIGURE_TOTP`) của Keycloak; link mở **trang action-token của Keycloak** (theme `eduworld-ops`) — ngoại lệ duy nhất dùng trang Keycloak, không phải luồng đăng nhập OIDC |

`setup_ticket` hết hạn hoặc đã dùng → `401 SETUP_TICKET_INVALID`, portal về `/login`.

**Quên và đặt lại mật khẩu (ADR-0091 mục 3, chỉ nhân sự):**

| Màn | Hành vi |
|---|---|
| `/forgot-password` | Ô "Tên đăng nhập hoặc email" → `POST /api/v1/auth/portal/password:forgot` (`forgotPortalPassword`). Luôn hiện "Nếu tài khoản tồn tại, email hướng dẫn đã được gửi" (API luôn `202`, không lộ tài khoản có tồn tại). Học sinh không nhận email; mật khẩu học sinh do quản trị trường đặt lại |
| `/reset-password?token=…` | Link trong email (token dùng một lần, hạn `portal_password_reset_ttl` — [KEYCLOAK_SSO §3.3](../09-identity/KEYCLOAK_SSO.md)). Mật khẩu mới + nhập lại → `POST /api/v1/auth/portal/password:reset` (`resetPortalPassword`); thành công → mọi phiên portal của người dùng bị thu hồi, về `/login`. Token sai/hết hạn/đã dùng → `401 PASSWORD_RESET_TOKEN_INVALID`, nút "Gửi lại email" |

**Sau đăng nhập** API trả access token trong body (portal chỉ giữ **trong bộ nhớ**) và đặt refresh token trong cookie `HttpOnly; Secure; SameSite=Strict` cùng cookie CSRF (chi tiết [API_DESIGN §9.1](../07-backend/API_DESIGN.md), [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md)). Portal gọi `GET /api/v1/me/tenants` (không cần `X-Tenant-Id`). Portal **không** gọi `POST /api/v1/auth/session` (`createAuthSession`): endpoint đó thuộc luồng game và trả `PlayerBootstrap` cho học sinh.

| Trường hợp | Kết quả |
|---|---|
| Nhân sự platform (`staff` không có `primary_tenant_id`, chỉ có role platform: System Admin, Content Admin/Reviewer, Game Designer, Live-ops, Competition Admin, Safety Moderator — ADR-0081) | Vào phạm vi platform (bắt buộc mã xác thực ngay ở form đăng nhập). System Admin xem liên tenant bằng bộ lọc tenant (§2.5); khi ghi dữ liệu tenant chọn tenant đích → portal gửi `X-Tenant-Id` = tenant đích |
| Nhân sự thuộc đúng 1 tenant | Vào thẳng tenant đó; mọi request gửi `X-Tenant-Id` |
| Nhân sự thuộc nhiều tenant | Màn "Chọn đơn vị làm việc" (chọn sẵn tenant lần trước nếu đã nhớ, per-viewer); portal gửi `X-Tenant-Id` của tenant đã chọn — **không** xin token mới, không nhập lại mật khẩu; backend kiểm tra membership (ADR-0065, ADR-0091 mục 4) |
| Không còn UserRole portal hiệu lực ở tenant nào và không có role platform | Màn "Tài khoản chưa được cấp quyền quản trị. Vui lòng liên hệ quản trị trường." (`403 PORTAL_ACCESS_STAFF_ONLY`) + nút Đăng xuất |
| Tài khoản học sinh (mật khẩu đúng) | Flow Keycloak từ chối, API trả `403 PORTAL_ACCESS_STAFF_ONLY`; portal hiện "Tài khoản học sinh không dùng được cổng quản trị. Em hãy đăng nhập trong game nhé." Nếu lọt qua Keycloak: mọi route portal trả cùng mã, portal hiện cùng thông điệp và đăng xuất |

**Đổi tenant trên topbar:** nhân sự thuộc nhiều tenant thấy bộ chọn `[Đơn vị: THCS Demo B ▾]` trên topbar (mobile: trong menu ☰). Đổi tenant = đổi `X-Tenant-Id` + tải lại menu (`GET /me/permissions`, gồm `disabled_menus`/`disabled_actions` của tenant mới) và đưa về trang tổng quan của workspace; bản nháp chưa lưu được cảnh báo trước khi đổi. Không xin token mới (bỏ scope `organization:{alias}` cho portal — ADR-0091 mục 4). Không bao giờ trộn dữ liệu hai tenant trong một màn.

**Phiên portal (ADR-0091 mục 4, ADR-0086 mục 7):** giữ chính sách idle 30 phút / tối đa 10 giờ và MFA theo role (`portal_session_sso`, [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md)). Access token chỉ nằm trong bộ nhớ SPA (không `localStorage`/`sessionStorage`); refresh token nằm trong cookie `HttpOnly` mà JavaScript không đọc được. Tải lại trang hoặc mở tab mới: portal gọi `POST /api/v1/auth/portal/token:refresh` (kèm header `X-EW-CSRF` lấy từ cookie CSRF) để khôi phục phiên, **không** phải đăng nhập lại; quá idle/max → về `/login` của portal (giữ URL để quay lại sau đăng nhập). Route platform chỉ nhận `auth_time` trong 8 giờ: quá hạn → `403 MFA_REQUIRED`, portal mở hộp đăng nhập lại (mật khẩu + mã xác thực) ngay trên trang. Đăng xuất gọi `POST /api/v1/auth/portal/logout` (thu hồi refresh token và phiên Keycloak, xóa cookie). Học sinh vào lại game bằng refresh token lưu trên thiết bị là chính sách riêng của game, không áp dụng cho portal.

**Trạng thái lỗi màn đăng nhập** (mã lỗi theo [API_DESIGN §4.1](../07-backend/API_DESIGN.md#41-catalog-mã-lỗi)):

| Tình huống | `code` | Thông điệp |
|---|---|---|
| Sai tên đăng nhập/email, mật khẩu hoặc mã xác thực; thiếu mã với tài khoản bắt buộc MFA; tài khoản khóa tạm | `401 INVALID_CREDENTIALS` | "Tên đăng nhập, mật khẩu hoặc mã xác thực chưa đúng." (không tiết lộ tài khoản tồn tại, bị khóa hay cần mã) |
| Tài khoản còn bước thiết lập | `403 ACCOUNT_SETUP_REQUIRED` | Chuyển sang màn "Đặt mật khẩu mới" hoặc "Kiểm tra email để cài ứng dụng xác thực" |
| Tài khoản bị vô hiệu hóa trong EduWorld (mật khẩu đúng) | `403 ACCOUNT_DEACTIVATED` | "Tài khoản đã bị vô hiệu hóa. Vui lòng liên hệ quản trị." |
| Học sinh / không còn quyền portal | `403 PORTAL_ACCESS_STAFF_ONLY` | Như bảng trên |
| Thử quá nhiều lần | `429 RATE_LIMITED` | "Bạn đã thử quá nhiều lần, vui lòng đợi ít phút." (theo `Retry-After`) |
| Phiên hết hạn (idle 30 phút / tối đa 10 giờ) — `token:refresh` trả `401 REFRESH_TOKEN_INVALID` | — | Hộp "Phiên làm việc đã hết, vui lòng đăng nhập lại"; giữ URL để quay lại sau đăng nhập |
| Cookie CSRF thiếu/lệch khi làm mới phiên | `403 CSRF_TOKEN_INVALID` | Về `/login` (không hiện chi tiết kỹ thuật) |
| `403 TENANT_MISMATCH` khi đang dùng | — | "Đơn vị làm việc đã thay đổi quyền truy cập." → về màn chọn tenant |
| `403 MFA_REQUIRED` ở workspace platform | — | Hộp đăng nhập lại có mã xác thực (step-up) |
| `403 FEATURE_DISABLED` khi đang dùng | — | "Tính năng này đang tạm tắt." → tải lại menu (§2.7) |

**Định danh (ADR-0067, [ADR-0069](../adr/ADR-0069-username-email-duy-nhat-theo-tenant-va-dang-nhap-khong-chon-tenant.md)):** đăng nhập portal bằng username **hoặc** email; cả hai là duy nhất trên SSO (một người một tài khoản, không phân biệt hoa thường), nên không trùng trong cùng tenant; Keycloak xác thực trước, module identity kiểm tra tenant sau; nhân sự bắt buộc có email; một người làm nhiều trường dùng **một** tài khoản với nhiều membership và UserRole.

### 1.5. Bố cục responsive web/mobile của portal [Proposed]

Portal là web app (React + Ant Design 5, ADR-0047), chạy trên trình duyệt desktop, tablet và điện thoại. Cùng một ứng dụng, bố cục đổi theo breakpoint; quyền và dữ liệu không đổi theo thiết bị.

| Breakpoint | Độ rộng | Bố cục |
|---|---|---|
| **Desktop** | ≥ 1280 px | Topbar đầy đủ; sidebar mở (thu gọn được thành icon 64 px); panel thuộc tính bên phải; bảng nhiều cột |
| **Tablet** | 768–1279 px | Sidebar thành drawer (nút ☰); panel thuộc tính thành tab "Thuộc tính" trong trang; bảng ẩn cột phụ (cấu hình theo cột `priority`); form 2 cột → 1 cột dưới 1024 px |
| **Mobile** | < 768 px | Topbar rút gọn (☰, tên workspace, 🔔, 👤; tìm kiếm thành icon mở toàn màn; bộ chọn tenant của System Admin vào menu ☰); **bảng chuyển thành danh sách thẻ** (mỗi thẻ: tiêu đề, 2–3 trường chính, trạng thái, menu ⋯); form 1 cột, nút hành động chính cố định ở đáy; bộ lọc mở bằng ngăn kéo dưới (bottom sheet) |

| Quy tắc | Mô tả |
|---|---|
| Sidebar | Desktop: mở mặc định, nhớ trạng thái thu gọn theo người dùng; tablet/mobile: drawer đè nội dung, đóng khi chọn mục |
| Bảng → thẻ | Component `ResponsiveTable` (`web/packages/ops-ui`): khai báo cột có `priority` (1 = luôn hiện) và `cardTitle`; < 768 px tự render thẻ; thao tác hàng loạt chuyển thành chế độ chọn nhiều bằng nhấn giữ |
| Form | 1 cột trên mobile; nhóm trường thành bước (step) khi > 8 trường; trường công thức dùng `MathInput` (MathLive) có bàn phím ảo riêng |
| Vùng chạm | ≥ 44 px trên mobile/tablet |
| Hành động chính | Một nút chính cố định ở đáy màn mobile; hành động phụ trong menu ⋯ |
| Banner hệ thống | Luôn hiển thị trên mọi breakpoint (ví dụ "Đang tắt kiểm duyệt", sự cố live-ops) |

**Tác vụ hỗ trợ trên mobile:** xem dashboard (School Learning Dashboard, live-ops, analytics tổng hợp); duyệt nội dung (đọc, preview, phê duyệt/từ chối có lý do); xem thông báo vụ việc an toàn và xác nhận đã liên hệ phụ huynh (nhà trường); xử lý moderation (Safety Team: xem case, ẩn tin, mute, chuyển cấp); xem danh sách học sinh, lớp, bài giao; nhắc/gia hạn bài giao; xác nhận các yêu cầu (club, clone, cleanup) khi có quyền.

**Tác vụ chỉ desktop/tablet ≥ 1024 px:** builder phức tạp (Question/Lesson/Activity Builder nhiều dạng, Quest Builder, Challenge/Boss, Event & Season Builder, cấu hình progression/economy), import hàng loạt, so sánh version (diff), quản lý tenant/gán/clone, preview game 3D tương tác. Mở trên mobile hiển thị trang "Tác vụ này cần màn hình lớn hơn" kèm link gửi về email/máy tính (không mất ngữ cảnh) và vẫn cho xem chỉ đọc.

#### 1.5.1. School Learning Dashboard (dashboard trường)

Nghiệp vụ, widget và ngưỡng: [SCHOOL_LMS §8](SCHOOL_LMS.md#8-school-learning-dashboard). Người dùng: School Admin (trường mình), Academic Manager (scope môn/trường).

```text
Desktop (≥ 1280)
┌ Topbar ───────────────────────────────────────────────────────────────────────────────┐
├ Sidebar ─┬ School & LMS / TH Demo A / Tổng quan ───────────────────────────────────────┤
│ Tổng quan│ [Khối 3 ▾] [Lớp: Tất cả ▾] [Môn: Toán ▾] [Tuần này ▾]                       │
│ Bài giao │ ┌ Hoàn thành bài giao ┐ ┌ Cần chú ý ─────┐ ┌ An toàn ────────┐ ┌ LO yếu ──┐ │
│ Học sinh │ │ 3A 24/30 ▓▓▓▓▓▓▓░   │ │ 11 HS   [Xem]  │ │ 1 chờ xác nhận  │ │ 3 LO     │ │
│ Báo cáo  │ │ 3B 12/30 ▓▓▓░░░░░   │ │ 4 làm quá nhanh│ │ [Xem]           │ │ [Giao ôn]│ │
│ An toàn  │ └─────────────────────┘ └────────────────┘ └─────────────────┘ └──────────┘ │
│          │ ┌ Heatmap lớp × LO ───────────────────────┐ ┌ Tự học sau gợi ý ───────────┐ │
│          │ │ 3A ◑ ● ◔   3B ◔ ◑ ○   3C ● ◕ ◑          │ │ 68% HS làm · 37 LO tiến bộ  │ │
│          │ └─────────────────────────────────────────┘ └─────────────────────────────┘ │
└──────────┴────────────────────────────────────────────────────────────────────────────┘

Mobile (< 768)
┌ ☰  TH Demo A · Toán  🔔 👤 ┐
│ [Khối 3 ▾] [Tuần này ▾]     │
│ ┌ Hoàn thành  36/60 ▓▓▓░  ┐ │  thẻ KPI xếp dọc,
│ ┌ Cần chú ý 11 HS   [Xem] ┐ │  chạm để đi tới danh sách
│ ┌ An toàn: 1 chờ xác nhận ┐ │
│ ┌ LO yếu: Nhân 7, Chia 8  ┐ │
│ Heatmap lớp × LO (cuộn)     │
│ Bài giao sắp hạn (thẻ)      │
└─────────────────────────────┘
```

#### 1.5.2. Thông báo an toàn của trường

Nhà trường không xem nội dung chat và không xử lý case; Platform Safety Team xử lý và gửi **thông báo vụ việc nghiêm trọng** liên quan học sinh của trường để trường liên hệ phụ huynh (ADR-0072 mục 4, nghiệp vụ ở [MODERATION](../03-social-multiplayer/MODERATION.md)).

```text
Desktop
├ Sidebar ─┬ School & LMS / An toàn / Thông báo vụ việc ───────────────────────────────┤
│          │ [Trạng thái: Chờ xác nhận ▾] [Lớp ▾] [30 ngày ▾]                          │
│          │ ┌ Danh sách (40%) ──────────────┐ ┌ Chi tiết thông báo (60%) ───────────┐ │
│          │ │ ● 3A · Mức nghiêm trọng · hôm │ │ Học sinh: mã HS 3A-017              │ │
│          │ │ ○ 5B · Đã xác nhận · 12/11    │ │ Loại: an toàn cá nhân · mức p1      │ │
│          │ │ …                             │ │ Tóm tắt do Safety Team ghi          │ │
│          │ └───────────────────────────────┘ │ (không có nội dung chat)            │ │
│          │                                   │ Yêu cầu lúc 14:05 · hạn theo SLA    │ │
│          │                                   │ Ghi chú liên hệ [..............]    │ │
│          │                                   │ [Xác nhận đã liên hệ phụ huynh ▶]   │ │
│          │                                   └─────────────────────────────────────┘ │

Mobile
┌ ☰ An toàn (1)          🔔 ┐      chạm một thẻ →   ┌ ← 3A · Nghiêm trọng       ┐
│ [Bộ lọc ▾]               │                       │ Tóm tắt (không chat thô)  │
│ ┌ 3A · Chờ xác nhận     ┐│                       │ Yêu cầu 14:05 · hạn (SLA) │
│ ┌ 5B · Đã xác nhận      ┐│                       │ Ghi chú liên hệ           │
└──────────────────────────┘                       │ [Xác nhận đã liên hệ ▶]   │
                                                   └───────────────────────────┘
```

Xác nhận lưu thời điểm và người xác nhận vào case (audit); hạn xác nhận theo `school_notification_ack_sla_hours` do [MODERATION §6.4](../03-social-multiplayer/MODERATION.md#64-thông-báo-vụ-việc-cho-nhà-trường-proposed) sở hữu; thông báo quá hạn chưa xác nhận được nhắc lại cho School Admin và hiển thị cho Safety Team [Proposed]. API: `GET /api/v1/tenant/safety/school-notifications`, `POST /api/v1/tenant/safety/school-notifications/{notification_id}:acknowledge`.

#### 1.5.3. Danh sách nội dung (Learning Content Studio)

```text
Desktop
├ Sidebar ─┬ Learning Content Studio / Hoạt động ──────────────────────────────────────┤
│          │ [🔍] [Môn ▾] [Khối ▾] [Loại ▾] [Trạng thái ▾] [Ngôn ngữ: vi|en ▾] [+ Tạo]  │
│          │ ┌─────┬──────────────────────┬──────┬─────┬───────────┬─────┬──────────┐ │
│          │ │ ☐   │ Tên                  │ Loại │ Khối│ Trạng thái │ Bản │ Cập nhật │ │
│          │ │ ☐   │ Phân số là gì?       │ Video│ 4   │ Đã xuất bản│ v3  │ 10:42    │ │
│          │ │ ☐   │ Nghe: Chú mèo Mướp   │ Nghe │ 4   │ Chờ duyệt  │ v1  │ hôm qua  │ │
│          │ └─────┴──────────────────────┴──────┴─────┴───────────┴─────┴──────────┘ │

Mobile
┌ ☰ Hoạt động      🔍 🔔 ┐
│ [Bộ lọc ▾]   [+ Tạo]*   │  * mở trang "cần màn hình lớn hơn" với builder phức tạp
│ ┌ Phân số là gì?       ┐ │
│ │ Video · Khối 4 · v3  │ │  thẻ: tên, loại, khối, bản, trạng thái (chip màu nghiệp vụ)
│ │ ● Đã xuất bản    ⋯   │ │  ⋯ : Xem, Preview, Duyệt (nếu có quyền)
│ ┌ Nghe: Chú mèo Mướp   ┐ │
│ │ Bài nghe · Khối 4 ·v1│ │
│ │ ● Chờ duyệt  [Duyệt] │ │
└─────────────────────────┘
```

### 1.4. Editor chung cho nội dung có version

Mọi builder (activity, quest, challenge, boss, achievement, item, event…) dùng chung khung editor `ContentEditorShell` trong `web/packages/ops-ui`:

| Khả năng | Mô tả |
|---|---|
| Soạn | Form theo JSON Schema của loại nội dung (ProForm/StepsForm sinh từ schema); khối nội dung kéo-thả (drag & drop) cho bài học, bước quest, phase boss, objective |
| Validate | Validate tức thời theo schema (`required`, `maxLength`, enum) + validate nghiệp vụ phía server khi lưu; lỗi hiển thị theo field |
| Preview | Preview web (học tập) hoặc **Godot Web** trong `GamePreviewFrame` (game, activity cần renderer game) với snapshot draft qua `preview_token` |
| Workflow | Draft → Review → Approved → Publish (ngay/hẹn giờ) → Archived; rollback, unpublish (§3.11, §4, §6) |
| So sánh version | Diff theo field/khối giữa 2 version |
| Autosave & khóa | Autosave mỗi `content_autosave_interval` = 10 giây; optimistic concurrency (`ETag`/`If-Match`) chống ghi đè, `412`/`409 CONTENT_VERSION_CONFLICT` → dừng autosave, giữ thay đổi cục bộ và cho tải bản mới; cảnh báo khi người khác đang sửa |
| Audit | Mọi thao tác ghi audit (§7) |
| **Công thức Toán/Lý/Hóa** (ADR-0055) | Mọi trường có thể chứa công thức dùng **MathLive** `<math-field>` (component `MathInput`): nhập trực quan, bàn phím ảo tab Toán/Lý/Hóa (phân số, căn, lũy thừa, chỉ số, ma trận, `\ce{}` hóa học, `\pu{}` đơn vị), chèn inline `\( \)` hoặc block `\[ \]` vào rich text. Lưu **LaTeX** (kèm SVG + speech text render trên trình duyệt). Mọi nơi hiển thị (preview, danh sách, School Learning Dashboard, xem lại bài làm) dùng **MathJax 4** (component `MathText`, extension `mhchem`). Khi lưu/publish, portal dùng MathJax 4 **trên trình duyệt** render mỗi công thức thành SVG + speech text và gửi kèm nội dung (dùng cho game, vốn chỉ hiển thị công thức); editor chặn ngay công thức lỗi cú pháp. Server kiểm tra LaTeX theo allowlist và sanitize SVG; thiếu SVG, SVG không hợp lệ hoặc LaTeX chứa lệnh cấm → `422 MATH_RENDER_FAILED` kèm vị trí, editor tô đỏ đúng công thức lỗi và không cho gửi duyệt. **Bulk import** đi qua màn Import của portal (render trên trình duyệt trước khi gửi). Rule chặn lệnh nguy hiểm: [SECURITY_ARCHITECTURE §8b](../10-security/SECURITY_ARCHITECTURE.md#8b-công-thức-latex-là-dữ-liệu-không-tin-cậy-adr-0055) |

Áp dụng công thức cho các editor: Question editor (đề, phương án, đáp án, giải thích, gợi ý, phản hồi theo phương án), Lesson Builder (gồm khối `worked_example`), Learning Activity Builder, phụ đề/transcript có công thức.

### 1.5. Biên tập song ngữ (ADR-0056)

Game song ngữ `vi` + `en` ([ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md)); quy tắc bắt buộc/tùy chọn theo loại nội dung do [CONTENT_MODEL §8](../02-learning/CONTENT_MODEL.md#8-ngôn-ngữ-nội-dung-và-bản-dịch-adr-0056) sở hữu. Portal áp dụng như sau:

| Khả năng | Mô tả |
|---|---|
| **Hai cột vi / en** | Mọi trường `localized_text`/`i18nText` trong `ContentEditorShell` hiển thị hai cột cạnh nhau (component `BilingualField`); trên màn hẹp chuyển thành tab `VI | EN`. Ngôn ngữ gốc (`content_language`) đứng trước và có nhãn "Gốc" |
| **Trạng thái bản dịch** | Mỗi trường và mỗi version có huy hiệu `Đủ` / `Thiếu` / `Lỗi thời` (`translation_status`); bộ lọc danh sách "Cần dịch", "Bản dịch lỗi thời"; số lượng hiện trên dashboard workspace |
| **Chặn publish** | Nút Publish/Gửi duyệt bị khóa khi thiếu bản bắt buộc (nội dung game: cả `vi` và `en`; nội dung học tập: bản theo `content_language`), danh sách field thiếu bấm để nhảy tới; server vẫn kiểm tra và trả `422 TRANSLATION_INCOMPLETE` |
| **Preview theo ngôn ngữ** | `GamePreviewFrame` và preview học tập có công tắc `vi` / `en` / "song song" để người duyệt xem đúng bản học sinh thấy |
| **Xuất / nhập file dịch** | Xuất các trường cần dịch của một hoặc nhiều nội dung ra **XLIFF 2.1** hoặc **CSV** (khóa ổn định = `<content_type>/<id>/<version>/<json-pointer>`) cho người dịch ngoài hệ thống; nhập lại tạo Draft mới, so khớp khóa, báo trường không khớp; chỉ người có permission `learning_content.translate` hoặc `game.content.translate` (§2.2) |
| **Bản dịch lỗi thời** | Sửa bản gốc sau khi đã dịch → field dịch tự đánh dấu `Lỗi thời`, hiện diff bản gốc cũ/mới cho người dịch |
| **Giao diện portal** | Ngôn ngữ giao diện `vi` (mặc định) / `en` chọn ở user menu [Proposed]; chuỗi UI portal qua i18n của React (khóa `portal.<workspace>.<screen>.<element>`), không phụ thuộc ngôn ngữ nội dung đang soạn |
| **Không dịch máy tự động** | Không tự dịch khi publish. Gợi ý dịch máy (nếu bật sau này) chỉ điền Draft, người duyệt phải xác nhận [Proposed] |

## 2. Permission model

### 2.1. Nguyên tắc

1. Authorization dựa trên **permission** — quyền nguyên tử **định nghĩa trong code** ở platform kit (`pkg/authz/catalog.go`, có `scope_kind` `tenant`/`platform`; mọi binary kiểm tra catalog lúc khởi động — AGENTS R20). **Role** là tập permission **lưu trong DB** (`identity.roles`, `identity.role_permissions`): role hệ thống seed sẵn (`is_system`, không sửa/xóa) và role tùy biến của tenant [Proposed]. **UserRole** (`identity.user_roles`) gán role cho người dùng theo phạm vi và thời hạn (ADR-0066, [SECURITY_ARCHITECTURE §5](../10-security/SECURITY_ARCHITECTURE.md)).
2. Mỗi role có `role_scope_kind`:
   - `tenant`: UserRole ở scope `tenant`, `school` hoặc `subject`, luôn trong **một** tenant. Không có scope `class` hay `class × subject` theo phân công giảng dạy (ADR-0072).
   - `platform`: UserRole chỉ ở scope `platform`.
3. **Bất biến:** role `tenant` (kể cả role tùy biến) **không bao giờ** chứa permission thuộc nhóm `game.*`, `competition.config.*`, `platform.*`. Kiểm tra khi khởi động (role seed), khi lưu role (use case `CreateRole`/`UpdateRole` → `422`) và bằng trigger DB; trigger chặn UserRole role `platform` ở scope tenant và role portal cho học sinh ([DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md)).
3b. **Học sinh không có quyền portal:** route portal yêu cầu `azp = eduworld-portal` và `user_type = staff` (`403 PORTAL_ACCESS_STAFF_ONLY`), học sinh không được gán role portal (ADR-0066).
3d. **Hai loại người dùng (ADR-0081):** `user_type` chỉ có `student` và `staff`. Nhân sự trường là `staff` có `primary_tenant_id`, chỉ nhận role tenant; nhân sự platform là `staff` có `primary_tenant_id = NULL`, chỉ nhận role platform. Một người làm cả hai việc dùng hai tài khoản. Portal phân biệt phạm vi platform/tenant theo role và tenant của phiên, không theo `user_type`.
3c. **Đánh giá quyền:** Principal = hợp permission của mọi UserRole còn hiệu lực trong tenant hiện tại (header `X-Tenant-Id`, ADR-0065) và phạm vi tài nguyên; cache 60 s; đổi role/UserRole phát `role.updated`/`user_role.changed` để làm mới cache và menu.
4. **Chỉ `system_admin` có permission `platform.cross_tenant.read`** (xem dữ liệu của tenant). Các role platform khác thao tác dữ liệu platform (`tenant_id = NULL`) và không đọc được dữ liệu của tenant; ngoại lệ duy nhất là `safety_moderator` đọc liên tenant **chỉ trong module moderation** (§2.5b).
5. Mỗi use case gọi tường minh `authz.Require(ctx, p, perm, resource)` (`pkg/authz`) ở đầu handler command/query ([ADR-0087](../adr/ADR-0087-cqrs-tach-read-write-khong-mediator.md)). Endpoint `/api/v1/game-admin/*`, `/api/v1/admin/*` còn kiểm tra `azp = eduworld-portal`, **MFA** (`acr` mức MFA, `amr` chứa `otp`) và principal có role `platform`.
6. Menu và workspace **sinh từ danh mục** `specs/portal/menu-catalog.yaml` và lọc theo `GET /api/v1/me/permissions` với header `X-Tenant-Id` của phiên (trả role, permission, danh sách workspace, `disabled_menus[]`, `disabled_actions[]`; nhân sự platform không gửi header) — §2.7, ADR-0090. Ẩn/hiện menu chỉ là tiện ích hiển thị; menu/action bị tắt còn bị chặn ở backend (`featuregate`, `403 FEATURE_DISABLED`).

### 2.2. Permission catalog

| Nhóm | Permission | Role `tenant` | Role `platform` |
|---|---|---|---|
| `lms.*` | `lms.class.read`, `lms.assignment.read`, `lms.assignment.write`, `lms.challenge.write`, `lms.school_event.write` (sự kiện trường từ template — EVENT_SEASON_SYSTEM §10), `lms.schedule.write`, `lms.report.read`, `lms.course.write`, `lms.learning_path.write`, `lms.study_group.write` (nhóm học tập — ADR-0072), `lms.result.release` (công bố kết quả bài `result_release = manual`), `lms.student.password_reset` (đặt lại mật khẩu học sinh trong scope, mật khẩu mới hiển thị một lần — [ACCOUNT_PROVISIONING §6d](../09-identity/ACCOUNT_PROVISIONING.md#6d-đặt-lại-mật-khẩu-học-sinh)) | ✔ | ✔ (System Admin, theo tenant đích) |
| `learning_content.*` | `learning_content.read`, `learning_content.write`, `learning_content.review`, `learning_content.publish`, `content.release.rollback`, `learning_content.import`, `media.upload`, `learning_content.activity.write`, `learning_content.activity.review` (activity theo `activity_type` trong Release, ADR-0048, ADR-0072), `learning_content.re_evaluate` (kích hoạt hệ thống chấm lại khi nội dung có lỗi, `attempt.re_evaluated` — ADR-0072), `learning_content.translate` (dịch, xuất/nhập file dịch — ADR-0056) | ✔ (dữ liệu của tenant) | ✔ (dữ liệu platform; System Admin cả dữ liệu tenant) |
| `curriculum.*` | `curriculum.read`, `curriculum.write`, `curriculum.publish` | `read` mọi role; `write/publish` Academic Manager scope tenant (curriculum riêng của tenant) | ✔ |
| `tenant.*` | `tenant.settings.read`, `tenant.settings.write` (chat policy tenant, khung giờ, thông báo), `tenant.user.read`, `tenant.user.write` (tạo/sửa nhân sự thủ công; một người một tài khoản SSO — ADR-0069), `tenant.school.read`, `tenant.school.write` (sửa thông tin hiển thị, duyệt trường `pending_review` — ADR-0060), `tenant.class.read` (lớp, sĩ số, học sinh chưa có lớp — ADR-0060), `tenant.role.read`, `tenant.role.write` (role tùy biến của tenant [Proposed] — ADR-0066), `tenant.user_role.write` (gán/thu hồi UserRole trong tenant; thay `tenant.roles.assign`), `tenant.sis.import`, `tenant.sis.read`, `tenant.trash.read`, `tenant.trash.restore` [Proposed] | ✔ | ✔ (System Admin) |
| `school.*` | `school.settings.read`, `school.settings.write` (chính sách trường: khung giờ truy cập, chat policy cấp trường, bật/tắt bảng xếp hạng lớp/trường — [COMPETITION_SYSTEM §9.2](../01-game-design/COMPETITION_SYSTEM.md), `student_club_creation` mặc định tắt), `school.people.read` | ✔ | ✔ (System Admin) |
| `lms.student_activity.read` · `tenant.student_cleanup.*` | `lms.student_activity.read` (lịch sử đăng nhập game, lần đăng nhập cuối — ADR-0031), `tenant.student_cleanup.read` (danh sách sắp bị dọn), `tenant.student_cleanup.exclude` (loại trừ/gia hạn), `tenant.student_cleanup.configure` (đổi `inactive_student_cleanup_days` của tenant khi platform cho phép), `tenant.chat_retention.configure` (đổi `gameplay_chat_retention_days` ≥ 3 khi được phép — ADR-0033) | ✔ (TA/SA; Academic Manager chỉ `lms.student_activity.read` trong scope) | ✔ (System Admin) |
| `analytics.*` · `report.export` | `analytics.class.read`, `analytics.school.read`, `analytics.tenant.read` (Learning Analytics và báo cáo **tổng hợp** Game Analytics của tenant/trường — [GAME_ANALYTICS §7.2](../11-operations/GAME_ANALYTICS.md#72-báo-cáo-tenant-workspace-school--learning-management); dashboard đọc rollup — ADR-0062), `report.export` (xuất báo cáo bất đồng bộ `/tenant/reports`, `/admin/reports` — [DASHBOARD_DATA §10](../07-backend/DASHBOARD_DATA.md#10-xuất-báo-cáo-bất-đồng-bộ)) | ✔ | ✔ (System Admin) |
| `game.analytics.read` | Game Analytics toàn platform, mọi panel Live Ops ([GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md)) | **✘ (cấm)** | ✔ (System Admin, live-ops, game designer) |
| `moderation.*` (platform-only) | `moderation.case.*` (`read`, `act` — gồm `:assign`, `:act`, `:notify-school`; `messages.read` — đọc tin nhắn trong phạm vi case; `lead` — trưởng ca: quản lý lịch trực, áp sanction nặng theo [MODERATION §7](../03-social-multiplayer/MODERATION.md)), `moderation.sanction.read`, `moderation.sanction.write` (tắt tiếng, hạn chế, khóa chat có thời hạn), `moderation.school_notification.send` (gửi thông báo vụ việc nghiêm trọng cho trường) | **✘ (cấm)** | ✔ (Safety Moderator, System Admin — ADR-0072) |
| `moderation.school_notification.*` (tenant) | `moderation.school_notification.read` (xem thông báo vụ việc nghiêm trọng liên quan học sinh của trường, chỉ tóm tắt, không chat thô), `moderation.school_notification.acknowledge` (xác nhận đã liên hệ phụ huynh) | ✔ (Tenant Admin, School Admin) | ✔ (System Admin) |
| `chat.*` (tenant) | `chat.sensitive_terms.read` (xem danh mục hiệu lực của tenant, nội dung term platform ẩn) | ✔ (Tenant Admin, chỉ đọc) | ✔ |
| `chat.*` (platform) | `chat.sensitive_terms.write` (danh mục platform: category, term, allowlist, publish), `chat.policy.toggle` (công tắc kiểm duyệt platform/tenant), `chat.tenant_terms.write` (từ bổ sung của tenant, theo yêu cầu của tenant), `chat.tenant_category.toggle` (bật/tắt category của tenant khi `allow_tenant_toggle`) | **✘ (cấm)** | ✔ (`sensitive_terms.write`, `policy.toggle`: chỉ System Admin; `tenant_terms.write`, `tenant_category.toggle`: Safety Moderator, System Admin) |
| `game.*` | `game.content.translate` (dịch nội dung game, xuất/nhập file dịch — ADR-0056), `game.chat_assets.write` (sticker pack, thư viện GIF — ADR-0034), `game.quest.write`, `game.minigame.write`, `game.npc.write`, `game.world.write`, `game.event.write` (sự kiện, template sự kiện — thay `game.world_event.write`), `game.event.operate` (dừng/tiếp tục/hủy sự kiện đang chạy), `game.season_track.write`, `game.item.write`, `game.reward_rule.write`, `game.progression.write` (đường cong XP, ngưỡng level `progression_levels`, nguồn XP và cap — [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)), `game.challenge.write`, `game.boss.write`, `game.achievement.write`, `game.economy.write` (tiền tệ trong game — ADR-0049), `game.shop.write` (Shop: giá, giới hạn mua, lịch bán), `game.collection.write` (bộ sưu tập), `game.economy.adjust` (bút toán bù, chỉ System Admin + người duyệt thứ hai), `game.liveops.read` (Game Live Operations), `game.content.review`, `game.content.publish` | **✘ (cấm)** | ✔ |
| `competition.config.*` | `competition.config.matchmaking.write`, `competition.config.ranking.write`, `competition.config.season.write`, `competition.config.tournament_template.write`, `competition.config.leaderboard.write`, `competition.config.reward.write` | **✘ (cấm)** | ✔ |
| `platform.*` (bổ sung) | `platform.settings.write` (cấu hình platform: dọn học sinh, thời hạn chat gameplay — ADR-0032/0033), `platform.student_cleanup.approve` (xác nhận lần dọn vượt ngưỡng) | **✘ (cấm)** | ✔ (System Admin) |
| `platform.*` | `platform.catalog.write` (danh mục quốc gia/tỉnh/kế thừa tỉnh, import trường — ADR-0060), `platform.role.read`, `platform.role.write` (role platform và role dùng chung), `platform.user_role.write` (gán/thu hồi UserRole platform và UserRole tenant ở tenant đích), `platform.cross_tenant.read`, `platform.tenant.manage`, `platform.tenant_data.assign`, `platform.tenant_data.clone`, `platform.trash.read`, `platform.trash.restore`, `platform.user.manage`, `platform.feature_flag.write`, `platform.sis_connection.manage`, `platform.break_glass.request`, `platform.break_glass.approve` (truy cập nội dung ngoài phạm vi case, phê duyệt hai người — [MODERATION §6.1](../03-social-multiplayer/MODERATION.md)) | **✘ (cấm)** | ✔ (chủ yếu System Admin) |
| `platform.*` (nghiên cứu hiệu quả — [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md), ADR-0080) | `platform.efficacy_study.manage` (thỏa thuận nghiên cứu, study, lớp/cặp, đợt kiểm tra, rút khỏi nghiên cứu, báo cáo hiệu quả — `/admin/learning/research-agreements*`, `/admin/learning/efficacy-studies*`), `platform.efficacy_study.export` (xuất bộ dữ liệu định danh giả `report_type = efficacy_dataset`, cùng `report.export`), `platform.learning_experiment.manage` (thử nghiệm A/B cấu hình mastery và luật gợi ý — `/admin/learning/experiments*`) [Proposed] | **✘ (cấm)** | ✔ (System Admin; chưa có role EdTech riêng) |
| `audit.*` · `data_request` · `social.*` | `tenant.audit.read` (nhật ký audit của tenant), `platform.audit.read` (audit mọi tenant), `platform.data_request.write` (tiếp nhận/xử lý yêu cầu về dữ liệu cá nhân), `social.club.approve` (duyệt club do học sinh tạo, chính sách `student_club_creation`) | ✔ `tenant.audit.read` (Tenant Admin, School Admin trong trường), `social.club.approve` (School Admin); ✘ `platform.*` | ✔ (System Admin) |

Chat policy cấp tenant/trường (`tenant.settings.write`, `school.settings.write`) là **cài đặt an toàn** theo ADR-0020, không phải cấu hình game. Cấp dưới chỉ siết chặt hơn mặc định platform.

Catalog role kiểm tra thêm bất biến (ADR-0030, ADR-0072): `chat.sensitive_terms.write`, `chat.policy.toggle`, `chat.tenant_terms.write`, `chat.tenant_category.toggle` và `moderation.case.*`, `moderation.sanction.*` là permission **platform-only**, không bao giờ nằm trong role tenant (cùng cơ chế với `game.*`).

### 2.3. Role × permission

**Role tenant** (`role_scope_kind = tenant`, workspace B và C, luôn trong một tenant):

| Role | Scope gán | Permission chính |
|---|---|---|
| `tenant_admin` | tenant | `tenant.*` (gồm `tenant.user.*`, `tenant.role.*`, `tenant.user_role.write`, `tenant.trash.read/restore` — Thùng rác của tenant chỉ dành cho role này, §3.8), `school.*`, `lms.class.read`, `lms.assignment.read`, `lms.report.read`, `lms.school_event.write`, `lms.student.password_reset`, `learning_content.*`, `curriculum.read`, `analytics.tenant.read`, `moderation.school_notification.read/acknowledge`, `chat.sensitive_terms.read`. Với tenant `unit`: quản trị mọi trường trong đơn vị. Không tạo bài giao/thử thách (việc của trường) |
| `school_admin` | school (một trường trong tenant) | `school.*` (chính sách trường: khung giờ truy cập, chat policy cấp trường, tắt bảng xếp hạng lớp/trường, `student_club_creation`), `tenant.user.read`, `tenant.user_role.write` (chỉ scope trường mình, chỉ role không vượt quyền của chính mình), `lms.*` (gồm `lms.challenge.write` — chỉ role này trong nhóm tenant), `learning_content.read/write/review/publish` (dữ liệu của tenant, phạm vi trường; duyệt lane `tenant` theo §3.5, người duyệt ≠ tác giả), `analytics.school.read`, `analytics.class.read`, `moderation.school_notification.read/acknowledge` (trường mình), `tenant.sis.import` (trường mình) |
| `academic_manager` (tổ chuyên môn) | tenant / school / subject | `curriculum.read`, `curriculum.write/publish` (scope tenant), `learning_content.*` (gồm `learning_content.re_evaluate`), `lms.class.read`, `lms.course.write`, `lms.learning_path.write`, `lms.assignment.*`, `lms.schedule.write`, `lms.study_group.write`, `lms.result.release`, `lms.report.read`, `lms.student_activity.read`, `analytics.*` trong scope (không có `lms.challenge.write`) |
| `tenant_content_editor` | tenant / school | `learning_content.read/write/import`, `learning_content.activity.write`, `media.upload` |
| `tenant_content_reviewer` | tenant / school | `learning_content.read/review/publish`, `learning_content.activity.review` |

Không có role `teacher` hay `tenant_moderator` (ADR-0072). Gán UserRole bằng mã role không tồn tại → `422 ROLE_NOT_FOUND`.

**Role platform** (`role_scope_kind = platform`, workspace A–E theo role; đăng nhập `eduworld-portal` bắt buộc MFA):

| Role | Permission chính | Xem dữ liệu tenant |
|---|---|---|
| `system_admin` | Toàn bộ `platform.*` (gồm `platform.cross_tenant.read`, gán/clone, thùng rác, quản lý tenant), mọi permission Learning Management trên tenant đích, có thể kiêm `game.*`/`competition.config.*` | **Có — duy nhất** (dropdown tenant, audit) |
| `content_admin` | `learning_content.*` (gồm activity, `learning_content.re_evaluate`), `curriculum.*` trên dữ liệu platform | Không |
| `content_reviewer` | `learning_content.review/publish`, `game.content.review` trên dữ liệu platform | Không |
| `game_designer` | `game.*` (trừ `game.content.publish`, `game.economy.adjust`) | Không |
| `liveops` | `game.event.write`, `game.event.operate`, `game.season_track.write`, `game.content.publish`, `content.release.rollback` (rollback nội dung game, §3.11), `game.liveops.read`, `game.analytics.read`, `competition.config.season.write` | Không |
| `competition_admin` | `competition.config.*` | Không |
| `safety_moderator` (Platform Safety Team) | `moderation.case.read/act/messages.read` (trưởng ca được gán thêm `moderation.case.lead`), `moderation.sanction.read/write`, `moderation.school_notification.send`, `chat.sensitive_terms.read`, `chat.tenant_terms.write`, `chat.tenant_category.toggle` | **Chỉ trong module moderation** (case, bằng chứng, tin nhắn liên quan case) — §2.5b |

Không có role `platform_admin`/`trust_safety` riêng: quản trị tenant do `system_admin` làm; kiểm duyệt liên tenant do `safety_moderator` làm (`system_admin` có thể kiêm).

### 2.4. Ma trận chức năng (tóm tắt)

| Chức năng \ Role | Tenant Admin | School Admin | Academic Manager | Tenant Content Editor/Reviewer | System Admin | Content Admin | Safety Moderator | Game Designer / Live-ops | Competition Admin |
|---|---|---|---|---|---|---|---|---|---|
| Curriculum: xem | ✔ | ✔ | ✔ | ✔ | ✔ | ✔ | ✘ | ✔ | ✘ |
| Curriculum riêng của tenant: tạo/sửa/publish | ✘ | ✘ | ✔ | ✘ | ✔ (tenant đích) | ✘ | ✘ | ✘ | ✘ |
| Curriculum/nội dung platform: tạo/sửa/publish | ✘ | ✘ | ✘ | ✘ | ✔ | ✔ | ✘ | ✘ | ✘ |
| Nội dung học tập của tenant: soạn | ✔ | ✔ | ✔ | ✔ / ✘ | ✔ (tenant đích) | ✘ | ✘ | ✘ | ✘ |
| Nội dung học tập của tenant: duyệt/publish | ✔ | ✔ | ✔ | ✘ / ✔ (≠ author) | ✔ | ✘ | ✘ | ✘ | ✘ |
| Nội dung platform được gán: xem/dùng | ✔ (chỉ đọc) | ✔ | ✔ | ✔ | ✔ | ✔ | ✘ | — | — |
| Hệ thống chấm lại (re-evaluate) khi nội dung lỗi | ✘ | ✘ | ✔ (scope) | ✘ | ✔ | ✔ (platform) | ✘ | ✘ | ✘ |
| Course, lộ trình, giao bài, lịch, đề theo blueprint, nhóm học tập | chỉ xem | ✔ (trường mình) | ✔ (scope) | ✘ | ✔ (tenant đích) | ✘ | ✘ | ✘ | ✘ |
| Thử thách lớp / giải trường từ template | ✘ | ✔ (trường mình) | ✘ | ✘ | ✔ (tenant đích) | ✘ | ✘ | ✘ | ✘ |
| School Learning Dashboard, báo cáo/analytics | tenant (tổng hợp) | trường | scope | ✘ | mọi tenant (lọc) | ✘ | ✘ | ✘ | ✘ |
| Cài đặt tenant / trường (gồm khung giờ truy cập, chat policy trường), SIS import | ✔ / ✔ | – / ✔ | ✘ | ✘ | ✔ | ✘ | ✘ | ✘ | ✘ |
| Gán/thu hồi UserRole trong tenant | ✔ | trong trường | ✘ | ✘ | ✔ | ✘ | ✘ | ✘ | ✘ |
| Tạo/sửa role tùy biến của tenant [Proposed] | ✔ | ✘ | ✘ | ✘ | ✔ | ✘ | ✘ | ✘ | ✘ |
| Tạo/sửa role platform, gán UserRole platform | ✘ | ✘ | ✘ | ✘ | ✔ | ✘ | ✘ | ✘ | ✘ |
| Moderation case, sanction, đọc tin nhắn trong case | ✘ | ✘ | ✘ | ✘ | ✔ mọi tenant (audit) | ✘ | ✔ liên tenant, chỉ module moderation (audit) | ✘ | ✘ |
| Thông báo vụ việc an toàn: xem tóm tắt, xác nhận đã liên hệ phụ huynh | ✔ (tenant) | ✔ (trường mình) | ✘ | ✘ | ✔ | ✘ | gửi, theo dõi | ✘ | ✘ |
| Danh mục từ nhạy cảm platform (category, term, allowlist) | ✘ (chỉ xem danh mục hiệu lực) | ✘ | ✘ | ✘ | ✔ | ✘ | chỉ xem | ✘ | ✘ |
| Từ bổ sung của tenant / bật-tắt category (khi được phép) | ✘ (gửi yêu cầu cho Safety Team) | ✘ | ✘ | ✘ | ✔ (tenant đích) | ✘ | ✔ (theo yêu cầu tenant) | ✘ | ✘ |
| **Công tắc kiểm duyệt** (tổng / từng bước) | **✘** | **✘** | **✘** | **✘** | ✔ | ✘ | **✘** | ✘ | ✘ |
| Thùng rác | tenant | ✘ | ✘ | ✘ | ✔ mọi tenant | ✘ | ✘ | ✘ | ✘ |
| Gán dữ liệu platform cho tenant / clone giữa tenant | ✘ | ✘ | ✘ | ✘ | ✔ | ✘ | ✘ | ✘ | ✘ |
| **Quest, mini-game, NPC, world event, item, reward, level** | **✘** | **✘** | **✘** | **✘** | ✔ | ✘ | ✘ | ✔ | ✘ |
| **Matchmaking, ranking, season, tournament template, leaderboard** | **✘** | **✘** | **✘** | **✘** | ✔ | ✘ | ✘ | live-ops: season | ✔ |
| Quản lý tenant, feature flag | ✘ | ✘ | ✘ | ✘ | ✔ | ✘ | ✘ | ✘ | ✘ |

Nguyên tắc bổ sung:
- **Reviewer ≠ author** (bắt buộc kỹ thuật) trừ khi `self_approve_allowed` (scope trường, có audit).
- Mọi truy cập dữ liệu tenant của System Admin ghi audit (ai, tenant nào, màn hình nào — ADR-0027 §6); mọi lần Safety Moderator mở case/tin nhắn của một tenant ghi audit (§2.5b).

### 2.5. Chế độ liên tenant của System Admin (ADR-0027 §6)

| Hạng mục | Người dùng tenant | System Admin |
|---|---|---|
| Phạm vi | Luôn đúng **một** tenant của phiên; không có dropdown tenant; tham số `tenant_ids` gửi lên bị bỏ qua | Mọi tenant |
| Bộ lọc tenant | — | **Dropdown tìm kiếm tenant** trên mọi màn hình có dữ liệu theo tenant: tìm theo mã/tên, **mặc định "Tất cả tenant"**, chọn **nhiều** tenant; lựa chọn lưu theo phiên làm việc (per-viewer) |
| Danh sách | Không có cột tenant | Luôn có cột **Tenant** (tên + loại `school`/`unit`/`common`); với tenant `unit` có thêm cột Trường |
| Thao tác ghi | Ghi vào tenant của phiên | Luôn chọn **một tenant đích** (form bắt buộc trường "Tenant") hoặc phạm vi **Platform** cho dữ liệu dùng chung. Không có ghi hàng loạt "tất cả tenant" ngầm định |
| API | Mọi request gửi `X-Tenant-Id` = tenant phiên (ADR-0065); `tenant_ids` bị bỏ qua; header hoặc `tenant_id` khác tenant phiên → `403 TENANT_MISMATCH` | List: **không** gửi `X-Tenant-Id`, nhận `tenant_ids[]`; ghi: `X-Tenant-Id` = tenant đích (thiếu → `400 TENANT_HEADER_REQUIRED`) |
| Audit | Theo thao tác | Mọi lần xem/ghi dữ liệu tenant: `actor`, `tenant_ids`, màn hình/endpoint |

### 2.5b. Phạm vi liên tenant của Safety Moderator (ADR-0072 mục 4)

| Hạng mục | Quy tắc |
|---|---|
| Phạm vi đọc | Chỉ dữ liệu của module moderation: case, bằng chứng, tin nhắn **liên quan case** (cửa sổ ngữ cảnh quanh tin bị báo cáo theo [MODERATION](../03-social-multiplayer/MODERATION.md)), sanction, thông báo nhà trường. Không đọc học sinh, kết quả học tập, nội dung, LMS hay chat ngoài case |
| Cơ chế | Handler moderation gọi `tenancy.Bypass(ctx, "moderation_case")` (R27), có audit và metric; mọi API khác vẫn áp `TenantScope` và trả `403 PERMISSION_DENIED` cho role này |
| Bộ lọc tenant | Hàng đợi case có dropdown tenant (mặc định "Tất cả", chọn nhiều) như §2.5 nhưng chỉ trong module Kiểm duyệt |
| Thao tác ghi | Mỗi thao tác (ẩn tin, sanction, đóng case, gửi thông báo trường, sửa từ bổ sung) gửi `X-Tenant-Id` = tenant của case/tenant đích |
| Audit | `moderation.case.viewed`, `moderation.case.messages_viewed` (tenant, case, số tin), mọi thao tác ghi; metric số lần bypass theo người dùng |
| Đăng nhập | Bắt buộc MFA như mọi role platform; chỉ gateway nội bộ |

### 2.6. Tenant chung (`common`)

Tenant chung chứa học sinh không thuộc tenant nào (ADR-0026). Không có vận hành tenant riêng mặc định: System Admin quản trị (giao nội dung đã gán), Platform Safety Team kiểm duyệt chat như mọi tenant. Có thể chỉ định `tenant_admin` cho tenant chung nếu đội vận hành EduWorld cần [Proposed].

### 2.7. Danh mục menu/action và bật/tắt (ADR-0090)

Áp dụng **chỉ cho Admin Portal**; menu trong game client (Godot) không dùng cơ chế này.

**Danh mục là contract.** Nguồn duy nhất là `specs/portal/menu-catalog.yaml` (schema `specs/schemas/portal-menu-catalog.schema.json`, kiểm tra bằng `node scripts/specs/check-portal-catalog.mjs`), cấu trúc **workspace → menu → action**:

| Cấp | Khóa | Thuộc tính chính |
|---|---|---|
| Workspace | `school_lms`, `content_studio`, `game_ops`, `live_ops`, `platform_admin` | `label_key` |
| Menu | `menu.<workspace>.<menu>` (vd. `menu.school_lms.assignments`) | `path`, `label_key`, `required_permissions[]`, `order`, `can_disable` |
| Action | `action.<workspace>.<menu>.<action>` (vd. `action.school_lms.assignments.edit`) | `label_key`, `required_permissions[]`, `operation_ids[]` (operation OpenAPI mà action gọi), `can_disable` |

- `required_permissions` theo nghĩa **ít nhất một** (any-of): menu/action hiện khi người dùng có một trong các permission đã liệt kê (§2.2). Mục chỉ có giao diện (vd. Tổng quan) có `operation_ids` rỗng.
- Mỗi operation OpenAPI mà portal gọi khai báo `x-portal-action` = khóa action sở hữu nó (operation dùng chung theo tham số như `/game-admin/{config_type}` khai báo bảng ánh xạ theo tham số); operation của khung portal (đăng nhập, `me/*`, danh mục quốc gia/tỉnh/trường, thông báo) khai báo `x-portal-shell` và không bao giờ bị chặn. Quy ước chi tiết: [API_DESIGN](../07-backend/API_DESIGN.md) (§11).
- Từ danh mục sinh `menuRegistry`/`workspaceRegistry` cho `web/apps/portal` (không khai báo menu tay trong TSX) và bảng đọc-chỉ `platform.portal_menu_catalog` ([DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md), ghi bởi `cmd/migrate` lúc deploy). Thêm menu/action mới vẫn cần code (màn hình, API); database chỉ giữ danh mục và trạng thái bật/tắt.
- Một operation chỉ thuộc một action. Tắt một menu tắt mọi operation của menu đó, kể cả khi màn khác gọi để tra cứu (vd. danh sách môn học dùng trong bộ chọn của editor câu hỏi); màn "Bật/tắt menu & action" hiển thị cảnh báo trước khi tắt menu có operation được dùng chung.

**Trạng thái bật/tắt** lưu trong `platform.feature_flags` (`flag_kind` = `menu`/`action`, `parent_key` của action là menu cha):

- Không có dòng flag = **bật**. Mỗi flag = ghi đè của tenant (nếu có), nếu không thì mặc định platform (`default_enabled`).
- Trạng thái hiệu lực của action = flag của action **và** flag của menu cha. Menu tắt thì mọi action của nó tắt.
- **Chỉ System Admin** (`platform.feature_flag.write`, gateway nội bộ, MFA) đặt mặc định platform hoặc ghi đè cho một tenant đích; bắt buộc lý do và `If-Match`; ghi audit và phát `platform_settings.changed` để làm mới cache (thời gian áp dụng theo `feature_toggle_cache_ttl` — [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md)).

**Mục không tắt được** (`can_disable = false`; ghi cố tắt → `422 FEATURE_NOT_DISABLEABLE`), gồm cả mọi action thuộc các menu này:

| Workspace | Menu |
|---|---|
| C. School & Learning Management | Người dùng & phân quyền (`menu.school_lms.access_control`) · Thông báo an toàn của trường (`menu.school_lms.safety_notifications`) · Thùng rác (`menu.school_lms.trash`) |
| D. Platform Administration | Tenant (`menu.platform_admin.tenants`) · Người dùng & phân quyền (`menu.platform_admin.access_control`) · Feature flag · Bật/tắt menu & action (`menu.platform_admin.feature_toggles`) · Nhật ký toàn hệ thống (`menu.platform_admin.audit_logs`) · Thùng rác (`menu.platform_admin.trash`) |
| E. Game Live Operations | Hàng đợi case kiểm duyệt (`menu.live_ops.moderation_cases`) · Sanction (`menu.live_ops.sanctions`) |

**Backend chặn, không chỉ ẩn.** Middleware `featuregate` của `core-api` chạy sau xác thực (và kiểm tra tenant, `portal_staff`), trước `authz.Require`, chỉ với token `azp = eduworld-portal`: action hoặc menu cha đang tắt theo tenant của phiên → `403 FEATURE_DISABLED` (Problem Details có `feature_key`). Phiên System Admin liên tenant không gửi `X-Tenant-Id` dùng mặc định platform. `featuregate` không thay authorization. Chi tiết thứ tự middleware, cache và hành vi khi kho trạng thái không sẵn sàng: [API_DESIGN](../07-backend/API_DESIGN.md) (§11), [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md).

**Hiển thị trên portal.** `GET /api/v1/me/permissions` trả thêm `disabled_menus[]` và `disabled_actions[]` (hiệu lực cho phạm vi phiên; `disabled_actions` gồm cả action có menu cha tắt). Portal hiện menu khi workspace được phép, người dùng có ít nhất một permission yêu cầu và menu không nằm trong `disabled_menus`; nút của action không có quyền hoặc đang tắt bị ẩn (hoặc khóa kèm tooltip "Tính năng đang tắt"). Gặp `403 FEATURE_DISABLED` → thông báo "Tính năng này đang tạm tắt" và tải lại `GET /me/permissions`. Mở URL trực tiếp của menu đang tắt → trang "Tính năng đang tắt" của portal.

## 3. Module theo workspace

### 3.1. Curriculum Management (Master Prompt §8.1)

Quản lý chương trình, khối lớp, môn, chủ đề (cây kéo-thả), LO (mã chuẩn, prerequisite graph), mapping bộ sách, import/export YAML/CSV, so sánh hai version + bảng mapping LO. Chi tiết: [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md). Curriculum chuẩn là dữ liệu platform được gán cho tenant (ADR-0029); tenant có thể có curriculum riêng (Academic Manager).

### 3.2. Content Management (§8.2)

| Màn hình | Chức năng |
|---|---|
| Lesson editor | Khối nội dung (CONTENT_MODEL §9, gồm khối `worked_example` — ví dụ mẫu từng bước [Proposed]), chèn activity, gắn LO |
| Learning Material / Media library | Upload, trạng thái pipeline, license, tìm kiếm, "đang dùng ở đâu" |
| Question editor | 11 dạng (form theo dạng), preview như học sinh thấy, kiểm tra hợp lệ (mỗi option có id, key khớp), tham số hóa, `misconception_tag` và **phản hồi theo phương án sai** (`options[].feedback`, song ngữ; chỉ trả cho học sinh trong kết quả chấm) cho option sai; **gợi ý nhiều cấp** `hints[]` (PED-04, [QUESTION_BANK](../02-learning/QUESTION_BANK.md)); với khối lớp 1–3: tải **lồng tiếng** đề bài (thiếu → cảnh báo chặn publish, PED-07) [Proposed]; công thức nhập bằng MathLive, hiển thị bằng MathJax 4 (phương án lựa chọn có thể là công thức). Học sinh không gõ biểu thức trong game: câu hỏi có công thức dùng dạng chọn đáp án, đúng/sai, nhập số (numeric + dung sai), điền số/từ, ghép cặp, sắp xếp, kéo thả (ADR-0055 mục 6); editor cảnh báo nếu chọn dạng đòi học sinh nhập công thức |
| Question Bank browser | Lọc theo curriculum/grade/subject/topic/LO/difficulty/type/status/nguồn (của tenant / được gán từ platform); thống kê item analysis |
| Question Set editor | Static hoặc selection rules; hiển thị kích thước pool hiện tại |
| Assessment Blueprint editor | Ma trận LO × độ khó, kiểm tra tổng điểm, pool đủ, xem trước đề sinh mẫu |
| Exercise / Learning Activity | Gói question set + feedback policy mặc định |
| Bulk import | Excel/CSV/JSON mẫu → validate → **render công thức trên trình duyệt** (MathJax 4) → báo lỗi theo dòng (gồm công thức lỗi) → tạo draft |

Người dùng tenant thấy: dữ liệu `tenant_id` = tenant của mình **cộng** dữ liệu platform được gán cho tenant (nhãn "Từ EduWorld", **chỉ đọc**). Muốn chỉnh nội dung được gán thì yêu cầu System Admin clone sang tenant (ADR-0029). Nội dung tenant **không** gắn được vào quest/mini-game; việc gắn question set vào game content chỉ đội platform làm trong Game Studio.

### 3.3. Game Content Management (§8.3) — **workspace A (Game Operations Console), platform-only (ADR-0027)**

| Đối tượng | Editor | Permission | Ghi chú |
|---|---|---|---|
| Quest | Form + step list (talk_npc, go_to, activity, collect, minigame, party_activity), điều kiện, phần thưởng, lịch | `game.quest.write` | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| Mini-game Configuration | Chọn `minigame_type`, question source (QuestionSet/rule), độ khó, thời gian, số câu, scoring, feedback, completion, reward | `game.minigame.write` | [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) |
| Game Level | Bảng XP → level (`progression_levels`) | `game.progression.write` | |
| NPC Dialogue | Graph editor (node thoại, lựa chọn, điều kiện, action) | `game.npc.write` | |
| Reward Configuration | RewardRule: trigger, điều kiện, phần thưởng, cap, diminishing | `game.reward_rule.write` | |
| World / World Event | Zone, area; sự kiện theo thời gian (lễ hội, "Tuần lễ Toán học") dùng Event Builder §3.15 | `game.world.write`, `game.event.write` | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) |
| Item / Cosmetic | Định nghĩa vật phẩm, nguồn mở khóa | `game.item.write` | |
| Sticker pack | Tạo pack, upload sticker (presigned URL, webp/png ≤ 512 KB, alt text tiếng Việt), `unlock_rule` (mặc định / phần thưởng quest / item), duyệt, publish, gán tenant | `game.chat_assets.write` | ADR-0034; không có upload của học sinh |
| Thư viện GIF | Upload GIF (≤ 2 MB, ≤ 480 px, tự sinh preview tĩnh), gắn tag vi/en, duyệt (bắt buộc người duyệt khác người upload), publish/archive | `game.chat_assets.write` | ADR-0034; chỉ GIF nội bộ, không dịch vụ bên thứ ba |
| Game Activity | Liên kết activity học tập ↔ vị trí trong thế giới (bảng, cổng, NPC) | `game.world.write` | |

Game content là dữ liệu platform (`tenant_id = NULL`). Quest/mini-game muốn cho tenant giao bài thì System Admin **gán** cho tenant (ADR-0029); có tùy chọn "gán cho mọi tenant hiện tại và tenant mới" [Proposed]. Game content chỉ tham chiếu `area_id`/`npc_id`/`scene_ref` có trong **manifest** của bản build Godot hiện hành (`game.client_manifests`). API: `/api/v1/game-admin/*` ([API_DESIGN §9.13b](../07-backend/API_DESIGN.md)).

### 3.4. Competition Management (§8.4) — **workspace A, platform-only (ADR-0027)**

Matchmaking configuration (queue, grade band, mở rộng cửa sổ rating), ranking rules (Glicko-2 tham số, season reset), season, **tournament/challenge template**, leaderboard definition, competition reward. Chi tiết: [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md), [RANKING](../01-game-design/COMPETITION_SYSTEM.md), [TOURNAMENT](../01-game-design/COMPETITION_SYSTEM.md), [LEADERBOARD](../01-game-design/COMPETITION_SYSTEM.md). Cấu hình có version và hiệu lực từ thời điểm (`effective_from`), không áp hồi tố lên trận đã kết thúc. Permission `competition.config.*`. Template được gán cho tenant để tenant tạo thử thách/giải.

### 3.5. Content Workflow (§8.5)

Draft → Review → Approved → Published → Archived (state machine trong [CONTENT_MODEL §6](../02-learning/CONTENT_MODEL.md#6-workflow-trạng-thái)), áp dụng thống nhất cho mọi loại nội dung có version. Archive khác soft-delete: archive là trạng thái workflow (vẫn hiển thị khi lọc "Đã lưu trữ"), soft-delete đưa vào Thùng rác (§3.8).

| Luồng (`review_lane`) | Nội dung | Ai soạn | Ai duyệt/publish | Workspace |
|---|---|---|---|---|
| `tenant` | Câu hỏi, bộ câu hỏi, bài học, blueprint, curriculum riêng của tenant | Tenant Content Editor, Academic Manager, School Admin | Tenant Content Reviewer / Tenant Admin / School Admin (≠ author) | B |
| `platform_learning` | Nội dung học tập & curriculum platform (`tenant_id = NULL`) | Content Admin, System Admin | Content Reviewer platform | B (dữ liệu platform) |
| `platform_game` | Quest, mini-game, NPC, reward, world event, item, season, template... | Game Designer, Competition Admin | Content Reviewer (`game.content.review`) → Live-ops publish | A |

System Admin có thể duyệt thay trong lane `tenant` của một tenant đích khi tenant yêu cầu hỗ trợ (audit).

### 3.6. Tenant dùng nội dung game (không cấu hình)

Tenant chỉ **chọn** quest/mini-game đã publish và được gán (ADR-0029) để giao bài, và tạo thử thách lớp/giải trường từ **template** platform với `allowed_params`. Quy trình, API và nghiệm thu: [SCHOOL_LMS §4.1, §7](SCHOOL_LMS.md#7-thử-thách-lớp-và-giải-trường). Không có màn hình nào trong workspace B/C cho phép sửa cấu hình game.

### 3.7. Gán dữ liệu cho tenant và clone giữa tenant (ADR-0029) — chỉ System Admin, workspace D

**Gán (assignment).**

| Bước | Màn hình | API |
|---|---|---|
| Chọn dữ liệu platform (curriculum, lesson, question/question set, blueprint, template thử thách/giải, quest/mini-game, item sự kiện) | Danh sách dữ liệu platform → nút "Gán cho tenant" | — |
| Chọn tenant (dropdown tìm kiếm, chọn nhiều) hoặc "Mọi tenant hiện tại và tenant mới" [Proposed] | Hộp thoại gán | `POST /api/v1/admin/tenant-assignments` |
| Xem tenant đang được gán | Tab "Tenant được gán" của bản ghi | `GET /api/v1/admin/tenant-assignments?resource_type=&resource_id=` |
| Gỡ gán | Chọn tenant → gỡ (soft-delete assignment); lịch sử học tập giữ nguyên | `DELETE /api/v1/admin/tenant-assignments` |

Permission `platform.tenant_data.assign`. Tenant chỉ đọc dữ liệu được gán.

**Clone.**

| Bước | Mô tả |
|---|---|
| 1. Chọn tenant nguồn (dropdown), loại tài nguyên, bản ghi (chọn nhiều) | Chỉ dữ liệu cấu hình/nội dung: nội dung học tập, ngân hàng câu hỏi, lớp mẫu **không có học sinh**, giao bài mẫu, cài đặt trường/tenant |
| 2. Chọn **một** tenant đích | Không cho chọn "tất cả tenant" |
| 3. Xem trước đồ thị phụ thuộc | Ví dụ question set → question → media; phụ thuộc là dữ liệu platform thì giữ tham chiếu, không clone |
| 4. Chạy | `POST /api/v1/admin/tenant-clones` (`Idempotency-Key`), worker xử lý bất đồng bộ; theo dõi `GET /api/v1/admin/tenant-clones/{job_id}` |
| 5. Kết quả | Bản sao ở tenant đích, ID mới, trạng thái `draft` (nội dung có workflow), cột `cloned_from_id`, `cloned_from_tenant_id`, `cloned_at`, `cloned_by`; audit ở cả tenant nguồn và đích |

Permission `platform.tenant_data.clone`. **Không bao giờ** clone dữ liệu cá nhân/học tập của học sinh (học sinh, enrollment, attempt, kết quả, chat, mastery).

### 3.8. Thùng rác (ADR-0028)

- Mọi nút "Xóa" trong CMS gọi `DELETE` = soft-delete (đặt `deleted_at`, `deleted_by`, `delete_reason` tùy chọn). Hộp thoại xác nhận yêu cầu nhập lý do với nội dung đã publish.
- Danh sách thường **không bao giờ** hiển thị bản ghi đã xóa; không có bộ lọc "hiện cả đã xóa" ở màn hình thường.
- **Thùng rác** là màn hình riêng:

| Người dùng | Phạm vi | API |
|---|---|---|
| System Admin | Mọi tenant (dropdown tenant) + dữ liệu platform | `GET /api/v1/admin/trash?resource_type=&tenant_ids=` · `POST /api/v1/admin/trash/{resource_type}/{id}:restore` |
| Tenant Admin (`tenant.trash.read`, `tenant.trash.restore`; School Admin và role tenant khác không có Thùng rác) | Tenant của mình | `GET /api/v1/tenant/trash?resource_type=` · `POST /api/v1/tenant/trash/{resource_type}/{id}:restore` |

- Khôi phục kiểm tra unique (vd. mã câu hỏi đã có bản ghi mới cùng mã → `409`), khôi phục theo quan hệ theo quy tắc aggregate ([DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md)). Mọi thao tác xem/khôi phục ghi audit.
- Không có nút "Xóa vĩnh viễn" trong ứng dụng.

### 3.9. An toàn chat: danh mục từ nhạy cảm, công tắc kiểm duyệt và thông báo vụ việc

Chi tiết nghiệp vụ: [MODERATION §11–§12](../03-social-multiplayer/MODERATION.md). Kiểm duyệt **tập trung ở Platform Safety Team** (role `safety_moderator`, ADR-0072 mục 4): nhà trường không đọc chat, không xử lý case, không sửa danh mục từ; trường chỉ đặt chat policy cấp trường trong giới hạn platform và nhận thông báo vụ việc nghiêm trọng. Quy tắc chat như nhau cho mọi lớp (R15).

| Màn hình | Workspace | Người dùng | Chức năng |
|---|---|---|---|
| **Danh mục từ nhạy cảm** | D → An toàn chat | System Admin (`chat.sensitive_terms.write`) | CRUD category (code, severity, action mặc định, `enabled`, `allow_tenant_toggle`), term (match_type, locale, biến thể, override), allowlist; xóa = soft-delete; **thử chuỗi** (nhập câu → category/action sẽ áp); xem kết quả test hồi quy; gửi duyệt (four-eyes [Proposed]) và publish → `catalog_version` mới; lịch sử version |
| **Công tắc kiểm duyệt** | D → An toàn chat | System Admin (`chat.policy.toggle`) | Bảng theo tenant với **dropdown tenant** (mặc định "Tất cả", chọn nhiều) + dòng "Platform"; mỗi dòng có công tắc tổng `enabled_all` và 5 công tắc bước (`sensitive_terms`, `pii`, `spam`, `bullying`, `async_analysis`); khi tắt bắt buộc nhập **lý do**, tùy chọn **thời hạn tự bật lại**; hộp thoại xác nhận nêu rõ rủi ro với học sinh 6–15 tuổi; lịch sử thay đổi (audit) |
| **Hàng đợi case** | E → Kiểm duyệt | Safety Moderator, System Admin (`moderation.case.*`) | Danh sách case mọi tenant (dropdown tenant, mức p1–p3, loại, SLA, trạng thái); mở case → bằng chứng và tin nhắn liên quan case (`moderation.case.messages.read`, qua `tenancy.Bypass(ctx, "moderation_case")`, mỗi lần mở ghi audit — §2.5b); ẩn tin, áp sanction (`moderation.sanction.*`: tắt tiếng, hạn chế, khóa chat có thời hạn), chuyển cấp, đóng case có lý do. Không có ô tìm kiếm chat tự do ngoài case |
| **Thông báo nhà trường** | E → Kiểm duyệt (gửi) · C → An toàn (nhận, §1.5.2) | Safety Moderator gửi (`moderation.school_notification.send`); School Admin, Tenant Admin nhận (`moderation.school_notification.read`, `.acknowledge`) | Với vụ việc nghiêm trọng liên quan học sinh của trường: Safety Team ghi **tóm tắt** (không có nội dung chat thô, không có thông tin học sinh khác ngoài trường) → trường nhận thông báo, ghi chú liên hệ và **xác nhận đã liên hệ phụ huynh**; case lưu thời điểm yêu cầu và thời điểm trường xác nhận |
| **Từ bổ sung của tenant** | E → Kiểm duyệt | Safety Moderator, System Admin (`chat.tenant_terms.write`) | Thêm/sửa/xóa mềm term trong category có sẵn (chỉ siết chặt) **theo yêu cầu của tenant** (ghi mã yêu cầu), thử chuỗi trong tenant, publish → version tenant |
| **Nhóm từ của tenant** | E → Kiểm duyệt (bật/tắt) · C → Cài đặt tenant (chỉ xem) | Safety Moderator, System Admin (`chat.tenant_category.toggle`); Tenant Admin xem (`chat.sensitive_terms.read`) | Danh sách category platform (chỉ tên, mô tả, mức độ; nội dung term platform **ẩn** với người dùng tenant); bật/tắt category của tenant **chỉ khi** `allow_tenant_toggle = true`, ngược lại công tắc bị khóa kèm chú thích |
| **Chính sách trường liên quan an toàn/xã hội** | C → Cài đặt trường | School Admin (`school.settings.write`) | Tắt bảng xếp hạng lớp/trường cho trường mình ([COMPETITION_SYSTEM §9.2](../01-game-design/COMPETITION_SYSTEM.md)); `student_club_creation` (học sinh được đề xuất club, mặc định **tắt**; nhóm học tập luôn do School Admin/Academic Manager tạo). Đây là chính sách trường, không phải cấu hình game (R20) |
| **Giờ giới nghiêm** | C → Cài đặt trường · D → Cấu hình hệ thống | School Admin (`school.settings.write`, action `action.school_lms.settings.edit_school`) · System Admin (`platform.settings.write`) | Trường: xem giờ giới nghiêm platform (chỉ đọc) và giờ hiệu lực, đặt khung giới nghiêm của trường chỉ **mở rộng** (phải chứa trọn giờ platform, server từ chối `422 VALIDATION_FAILED`), bắt buộc lý do (`getSchoolSettings`/`updateSchoolSettings`). Platform: đặt `platform_curfew` có lý do và xác nhận (`getPlatformCurfewSettings`/`putPlatformCurfewSettings`). Không có lựa chọn theo khối (R15); quy tắc: [CHILD_SAFETY §4.1](../10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) |
| **Chat policy trường** | C → Cài đặt trường | School Admin (`school.settings.write`) | Siết chặt trong giới hạn platform (ADR-0020, ADR-0030): giờ chat, bật/tắt DM, world chat, hiển thị tên thật cho bạn cùng lớp; không tắt được chat `match`/`team`/`party` (always-on); không có điều kiện theo khối (R15). Giới hạn: [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) |
| **Lịch trực Safety Team** | E → Kiểm duyệt | Trưởng ca quản lý (`moderation.case.lead`); Safety Moderator, System Admin xem (`moderation.case.read`) | Lịch ca trực on-call 24/7 cho case p1: calendar tuần (ICT), mỗi ca có người trực, vai trò `primary`/`secondary`, ghi chú; tạo/xóa mềm ca (chồng lấn cùng vai trò → `409 ONCALL_SHIFT_OVERLAP`); chỉ báo khoảng trống không có người trực; chuỗi gọi và ngưỡng theo [MODERATION §6.3](../03-social-multiplayer/MODERATION.md) |
| **Break-glass** | D → An toàn chat | System Admin yêu cầu (`platform.break_glass.request`), một System Admin **khác** duyệt (`platform.break_glass.approve`) | Yêu cầu truy cập nội dung ngoài phạm vi case: lý do, thời hạn, phạm vi; hàng chờ duyệt; trạng thái còn hiệu lực/hết hạn; mọi bước ghi audit, không chép nội dung tin vào audit; quy trình ở [MODERATION §6.1](../03-social-multiplayer/MODERATION.md) |
| **Banner trạng thái** | Mọi workspace (topbar) | System Admin; Safety Moderator; Tenant Admin/School Admin của tenant | Khi bất kỳ bước nào của tenant đang tắt: banner đỏ cố định **"Đang tắt kiểm duyệt: <bước> — đến <thời hạn>"** trên mọi màn hình của tenant đó |

### 3.10. Hoạt động đăng nhập game và dọn học sinh (ADR-0031, ADR-0032, ADR-0033)

| Màn hình | Workspace | Người dùng | Chức năng |
|---|---|---|---|
| **Hoạt động đăng nhập** (trong hồ sơ học sinh và danh sách lớp) | C (một tenant; System Admin có dropdown tenant) | TA, SA, Academic Manager (scope); System Admin | Lần đăng nhập game gần nhất, số ngày chưa vào game, lịch sử phiên (thời điểm vào/ra, nền tảng, thiết bị rút gọn, lý do kết thúc như "đăng nhập thiết bị khác"). Không hiển thị IP. Lọc "chưa vào game ≥ N ngày" |
| **Học sinh sắp bị dọn** | C → Cài đặt tenant | TA, SA (`tenant.student_cleanup.read/exclude`) | Danh sách từ `GET /tenant/student-cleanup/preview` (ngày dự kiến dọn, lần đăng nhập cuối); chọn nhiều → **Loại trừ** / **Gia hạn đến ngày** (bắt buộc lý do); xem lịch sử các lần dọn của tenant |
| **Cài đặt dọn học sinh** | C → Cài đặt tenant (khi platform cho phép ghi đè) · D → Cấu hình hệ thống | TA (`tenant.student_cleanup.configure`) · System Admin (`platform.settings.write`) | Số ngày không đăng nhập (`0` = tắt), số ngày báo trước, ngưỡng % cần xác nhận (platform). Lưu bắt buộc lý do, audit. Cảnh báo khi đặt < 60 ngày (dễ dọn nhầm qua kỳ nghỉ hè) [Proposed] |
| **Lần dọn chờ xác nhận** | D → Cấu hình hệ thống | System Admin (`platform.student_cleanup.approve`) | Lần chạy vượt 10% học sinh của tenant ở trạng thái `awaiting_approval`; xem mẫu danh sách, **Xác nhận** hoặc **Hủy** |
| **Thời hạn lưu chat gameplay** | D → An toàn chat · C → Cài đặt tenant (khi được phép) | System Admin · TA (`tenant.chat_retention.configure`) | `gameplay_chat_retention_days` (mặc định 7, tối thiểu 3 — nhập < 3 bị chặn `RETENTION_BELOW_MINIMUM`), `ephemeral_retention_hours` (24, chỉ platform); hiển thị thống kê purge gần nhất (số channel/tin, không nội dung) |

**Không có** màn hình nào để vận hành vào game hoặc đóng vai học sinh: preview game trong CMS chạy Godot Web export với dữ liệu mẫu, không tạo game session/login event (ADR-0031).

### 3.11. Phát hành nội dung: Draft/Publish, lịch sử, rollback (ADR-0043)

Áp dụng cho mọi loại nội dung có version trong mọi workspace (nội dung học tập ở workspace B; nội dung game, sticker/GIF ở workspace A). Mô hình chi tiết: [CONTENT_MODEL §5](../02-learning/CONTENT_MODEL.md#5-version-release-pointer-và-draftpublish-adr-0015-adr-0043).

| Màn hình / thành phần | Chức năng | Permission |
|---|---|---|
| **Thanh trạng thái item** (đầu mọi trang soạn) | Badge: `Bản nháp` / `Đang duyệt` / `Đã duyệt` / `Đã xuất bản v{n}` / `Có bản nháp mới (v{n+1}) chưa xuất bản` / `Đã gỡ xuất bản`; hiển thị version học sinh đang nhận (con trỏ) và số lượt chơi/học còn đang ghim version cũ | `*.read` |
| **Soạn bản nháp** | "Tạo bản nháp từ v{n}" khi item đã publish; autosave, `If-Match`; ghi `change_note` (≤ 500 ký tự) | `*.write` |
| **Xuất bản** | Nút "Xuất bản ngay" hoặc "Hẹn giờ" (`publish_at`, giờ ICT, ≥ 5 phút tới) [Proposed]; hộp thoại tóm tắt thay đổi (diff với version đang xuất bản), cảnh báo nếu media chưa `ready`; tiến trình "Đang chuẩn bị cache → Đã xuất bản"; huỷ hẹn giờ trước giờ chạy | `*.publish` |
| **Lịch sử phiên bản** | Bảng mọi version: số, trạng thái, tác giả, người duyệt, người xuất bản, thời gian, `change_note`; dòng thời gian release (publish/rollback/unpublish, người làm, lý do); lọc theo trạng thái | `*.read` |
| **Xem version** | Mở nội dung một version ở chế độ **chỉ đọc** (kể cả version cũ đã publish, version archived) | `*.read` |
| **So sánh** | Chọn 2 version → diff theo field/khối (text, đáp án, media thay đổi, metadata). Reviewer thấy diff mặc định với version đang xuất bản | `*.read`; xem diff `answer_key` cần `*.review` |
| **Preview trong game** | Nút "Xem trong game" chạy Godot Web export với snapshot của version đã chọn (không tạo game session, không XP) — [§5](#5-preview) | `*.read` |
| **Rollback** | Trên một version đã publish: "Dùng lại phiên bản này cho học sinh". Bắt buộc lý do (≤ 500 ký tự), xác nhận lần 2, hiển thị ảnh hưởng: học sinh đang chơi/học giữ version hiện tại cho đến khi xong lượt; lượt mới nhận version được chọn | `content.release.rollback` (tenant: Tenant Admin, Academic Manager, reviewer tenant; platform: Content Admin, Live-ops) |
| **Gỡ xuất bản** | Con trỏ về rỗng; bắt buộc lý do; cảnh báo các bài giao/quest đang tham chiếu | `*.publish` |
| **Thư viện media** | Upload (kéo-thả, nhiều file), tiến trình `quarantined → processing → ready/rejected` kèm lý do từ chối (sai loại file, virus, quá dung lượng); xem biến thể; `alt_text` bắt buộc cho ảnh; license; "đang dùng ở version nào"; thay ảnh = upload asset mới rồi chọn trong bản nháp — [CONTENT_MODEL §7](../02-learning/CONTENT_MODEL.md#7-media-asset-pipeline-adr-0044) | `media.upload`, `*.read` |

Quy tắc giao diện: không có nút "Sửa" trên version đã publish (chỉ "Tạo bản nháp từ phiên bản này"); hành động rollback/gỡ xuất bản hiển thị màu cảnh báo và luôn ghi lý do vào audit.

### 3.12. Builder mới của Game Operations Console (workspace A, platform-only)

Nghiệp vụ chi tiết nằm trong tài liệu product; CMS chỉ đặc tả màn hình. Mọi builder dùng `ContentEditorShell` (§1.4), dữ liệu platform (`tenant_id = NULL`), API dưới `/api/v1/game-admin/` (gateway nội bộ + MFA).

| Builder | Nguồn nghiệp vụ | Thành phần chính | Validation đặc thù | Permission |
|---|---|---|---|---|
| **Level & XP Configuration** | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | Bảng ngưỡng level (sinh từ công thức đường cong + chỉnh tay), biểu đồ đường cong XP, danh hiệu theo mốc level, phần thưởng lên cấp, bảng nguồn XP + cap ngày/tuần, mô phỏng "học sinh chơi N phút/ngày đạt level nào sau 4 tuần" | Ngưỡng tăng đơn điệu; cap > 0; thay đổi chỉ áp dụng từ `effective_from`, không thu hồi XP đã cấp | `game.progression.write` |
| **Challenge Builder** | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) | Chọn loại thử thách, độ khó (Easy → Legendary), điều kiện theo khối/môn/LO, activity tham chiếu (Activity Registry), thời gian, luật thắng/thua, phần thưởng, lịch | Activity tham chiếu đã publish; độ khó phù hợp khối; không có cơ chế phạt gây xấu hổ | `game.challenge.write` [Proposed] |
| **Boss Management** | CHALLENGE_BOSS_SYSTEM | Hồ sơ boss (model 3D qua thumbnail/turntable), các phase kéo-thả (mỗi phase: loại yêu cầu, activity, ngưỡng tiến độ, giới hạn thời gian), đóng góp cá nhân/đội, retry, phần thưởng; preview Godot | Mỗi phase có điều kiện kết thúc; tổng thời gian ≤ giới hạn cấu hình; phase dùng activity hợp khối | `game.boss.write` [Proposed] |
| **Achievement Builder** | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | Định nghĩa, trigger (event), điều kiện, tiến độ, phần thưởng, hiển thị (ẩn/hiện), version | Trigger là event có trong [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md); điều kiện không dựa vào điểm học tập chính thức | `game.achievement.write` [Proposed] |
| **Item / Inventory Catalog / Shop / Currency** | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md), ADR-0049 | Danh mục item (loại, độ hiếm, slot, thumbnail), nguồn nhận, giá trong Shop (chỉ tiền trong game), giới hạn mua, lịch bán; cấu hình tiền tệ; tra cứu sổ cái và **bút toán bù** có duyệt | Không có giá bằng tiền thật; không vật phẩm ngẫu nhiên trả phí; bút toán bù cần người duyệt khác người tạo | `game.item.write` (item), `game.shop.write` (Shop), `game.collection.write` (bộ sưu tập), `game.economy.write` (tiền tệ), `game.economy.adjust` (bút toán bù) [Proposed] |

### 3.13. Builder mới của Learning Content Studio (workspace B)

Chỉ có builder cho hoạt động **server tự chấm** (ADR-0072). Tự luận, rubric và hàng đợi chấm ngoài version này (ADR-0082).

| Builder / màn | Nguồn nghiệp vụ | Thành phần chính | Validation đặc thù | Permission |
|---|---|---|---|---|
| **Learning Activity Builder** | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), ADR-0048 | Bước 1 chọn `activity_type` từ Activity Registry (chỉ loại trong Release: `quiz`, `interactive_exercise`, `minigame`, `video_lesson`, `audio_lesson`, `simulation`); bước 2 form sinh từ `ConfigSchema` của loại đó; metadata (khối, môn, chủ đề, LO, độ khó, thời gian, chấm điểm, FeedbackTiming, reward); preview bằng renderer tương ứng | Theo schema của loại; loại chưa có renderer trên client hiện hành → cảnh báo không cho publish; loại ngoài Release không xuất hiện trong danh sách (API trả `422`); khối lớp 1–3 thiếu lồng tiếng đề bài/hướng dẫn → chặn publish (PED-07) [Proposed] | `learning_content.activity.write`; duyệt `learning_content.activity.review` |
| **Video Manager** | [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) | Upload (media pipeline ADR-0044), chương (chapter), phụ đề (VTT), câu hỏi chèn theo mốc thời gian, điều kiện hoàn thành | Phụ đề tiếng Việt bắt buộc với video có lời [Proposed]; mốc câu hỏi nằm trong thời lượng | `learning_content.activity.write`, `media.upload` |
| **Audio Manager** | [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md) | Bài nghe (audio, transcript, câu hỏi nghe hiểu tự chấm); lồng tiếng đề bài cho khối lớp 1–3. Bài nói ngoài version này (ADR-0082) | Bản nháp được thiếu transcript; publish bài nghe bắt buộc có transcript ([AUDIO_LANGUAGE_LEARNING §2](../02-learning/AUDIO_LANGUAGE_LEARNING.md)) | `learning_content.activity.write`, `media.upload` |
| **Simulation Configuration** | [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md), ADR-0052 | Chọn mô phỏng có sẵn trong client (theo manifest), nhập tham số, mục tiêu, câu hỏi gắn kèm; preview Godot | Tham số trong khoảng mô phỏng cho phép; không nạp script | `learning_content.activity.write` |
| **Chấm lại hệ thống** (từ trang version nội dung) | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), [EXAM_ENGINE §9](../02-learning/EXAM_ENGINE.md#9-chấm-lại-regrade) | Khi phát hiện đáp án/câu lỗi: chọn version lỗi, xem số attempt bị ảnh hưởng, nhập lý do → hệ thống re-evaluate, phát `attempt.re_evaluated` | Lý do bắt buộc; không sửa evaluation cũ (superseded); audit | `learning_content.re_evaluate` (Content Admin, Academic Manager). Chấm lại một **đề thi** (`scope = exam_session`) mở từ trang kết quả đề ở workspace C ([SCHOOL_LMS §4.5](SCHOOL_LMS.md#45-công-bố-kết-quả-và-chấm-lại)) |

### 3.14. Live Ops dashboards & Game Analytics

Định nghĩa chỉ số, nguồn dữ liệu, retention: [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) (ADR-0058). Portal chỉ hiển thị.

| Màn | Workspace | Nội dung | Bộ lọc | Quyền |
|---|---|---|---|---|
| **Live dashboard** | E. Game Live Operations | Số liệu thời gian thực từ MONITORING: CCU, phiên đang chạy, trận, hàng chờ ghép trận, tin bị chặn, lỗi; làm mới 15–30 s | Tenant (dropdown nhiều tenant), khu vực | `game.liveops.read` |
| **Game Analytics** | E. Game Live Operations | 11 panel theo [GAME_ANALYTICS §7.1](../11-operations/GAME_ANALYTICS.md#71-workspace-e--game-live-operations-platform): tổng quan, retention, onboarding, quest & cốt truyện, challenge & boss, tiến trình, kinh tế Xu, xã hội & thi đấu, sự kiện, chơi lành mạnh, hiệu năng thiết bị | Khoảng ngày, tenant (nhiều), trường, khối/grade band, nền tảng, ngôn ngữ, hạng thiết bị, phiên bản client, so sánh kỳ | `game.analytics.read` (platform-only) |
| **Báo cáo hoạt động game của tenant** | C. School & Learning Management | Bộ chỉ số tổng hợp [GAME_ANALYTICS §7.2](../11-operations/GAME_ANALYTICS.md#72-báo-cáo-tenant-workspace-school--learning-management); ô < 5 học sinh bị ẩn | Trường (tenant `unit`), khối, khoảng ngày | `analytics.tenant.read` (Tenant Admin), `analytics.school.read` (School Admin trường mình; Academic Manager có UserRole scope tenant/trường). Dữ liệu game không có chiều môn: UserRole chỉ scope `subject` → `403 PERMISSION_DENIED` |

- Component: StatisticCard, chart chuỗi thời gian, heatmap cohort, biểu đồ funnel, ProTable top nội dung; xuất CSV qua job bất đồng bộ.
- Trạng thái: đang tải; không đủ dữ liệu (ô bị ẩn hiển thị "< 5" bằng component dùng chung `SuppressedCell` của `web/packages/ops-ui`, [DESIGN_SYSTEM §4](../05-ux/DESIGN_SYSTEM.md#4-adminops-design-system-cmslmsadmin)); dữ liệu hôm nay tạm tính (nhãn "cập nhật hằng giờ"); lỗi API theo `code`.
- Từ panel quest/challenge mở thẳng item trong Game Operations Console (workspace A) để chỉnh độ khó.
- Responsive: desktop đầy đủ; điện thoại chỉ xem thẻ tổng quan và chơi lành mạnh.

#### 3.14.1. Quy tắc UI chung cho mọi dashboard (ADR-0062)

Áp dụng cho School Learning Dashboard, dashboard tenant, platform, Game Analytics. Cơ chế dữ liệu phía server: [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md).

| Quy tắc | Hành vi |
|---|---|
| Bộ chọn khoảng thời gian | Preset: Hôm nay, 7 ngày, 30 ngày, Học kỳ này, Năm học này, 12 tháng; tùy chọn tối đa **2 năm** — ngày ngoài giới hạn bị vô hiệu trong date picker; không có lựa chọn "toàn bộ thời gian" |
| Độ hạt | Hiển thị nhãn độ hạt server trả về (Theo ngày / Theo tuần / Theo tháng); người dùng chỉ chọn được độ hạt thô hơn |
| Thời điểm dữ liệu | Mỗi widget có "Cập nhật lúc HH:mm"; số liệu hôm nay hoặc chưa chốt có nhãn **"Tạm tính"** |
| Trạng thái "đang tính" | Ngày vừa qua chưa chốt (trước 02:30 ICT) hoặc backfill đang chạy: hiển thị "Đang tính số liệu…" thay vì số 0 |
| Chỉ số mới | Hiển thị "Chưa có số liệu trước ngày …" theo `available_from` |
| Truy vấn bị từ chối | `DASHBOARD_RANGE_TOO_LARGE`: gợi ý thu hẹp khoảng; `DASHBOARD_BUSY`: tự thử lại sau 2–5 giây, tối đa 2 lần; `DASHBOARD_QUERY_TIMEOUT`: gợi ý thu hẹp phạm vi (trường → lớp) |
| Tải widget | Mỗi widget tải độc lập (skeleton riêng), không chặn cả trang; tối đa 4 yêu cầu song song từ một trang |
| Xuất báo cáo | Nút **Xuất** tạo job bất đồng bộ: thông báo "Đang chuẩn bị báo cáo", báo trong portal khi xong, link tải hết hạn 15 phút; không tải file trực tiếp từ request |
| Ô dữ liệu nhỏ | Báo cáo cấp tenant/platform hiển thị "< 5" thay cho số khi ô có dưới 5 học sinh |

### 3.15. Event Builder & Season (workspace A), giám sát sự kiện (E), sự kiện trường (C)

Nghiệp vụ: [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md). Portal chỉ đặc tả màn hình; lịch mùa thuộc Competition Management (§3.4).

| Màn | Workspace | Thành phần chính | Validation / quy tắc | Permission |
|---|---|---|---|---|
| **Danh sách sự kiện** | A | ProTable: tên (vi/en), loại, trạng thái chạy (Scheduled/Live/Suspended/Ended/Cancelled/Archived), lịch, đối tượng, phiên bản; lọc theo loại/trạng thái/khoảng ngày/tenant | — | `game.event.write` (xem/sửa), `game.liveops.read` (xem) [Proposed] |
| **Lịch sự kiện** | A | Calendar tháng/tuần (ICT), mỗi sự kiện một thanh màu theo loại; cảnh báo chồng chéo (2 sự kiện trang trí cùng vùng, quá nhiều sự kiện cùng khối) | Một vùng tối đa một bộ trang trí cùng lúc | `game.liveops.read` |
| **Event Builder (wizard)** | A | Dùng `ContentEditorShell` (§1.4), 8 bước: ① Thông tin song ngữ (tên, mô tả, tagline vi/en, theme, banner) → ② Đối tượng (phạm vi, tenant/trường/lớp, khối, môn; số học sinh ước tính) → ③ Nội dung tham chiếu (chọn quest/challenge/boss/shop item/collection đã publish; community goal: metric, phạm vi, mục tiêu, mốc, trần/ngày) → ④ Phần thưởng (tham gia, mốc điểm sự kiện, mốc community goal; chọn từ catalog) → ⑤ Lịch (bắt đầu/kết thúc ICT, cửa sổ ngày trong tuần, thời gian nhận thưởng) → ⑥ Preview (Event Center và trang trí trong Godot Web; chọn preset Thấp/Trung bình) → ⑦ Gửi duyệt → ⑧ Lên lịch (publish) | Thiếu bản `en` của sự kiện platform → chặn publish (`TRANSLATION_INCOMPLETE`); nội dung tham chiếu phải đã publish; phần thưởng không có tiền thật; lễ hội ≥ 7 ngày; cảnh báo nếu mốc thưởng cần > 3 phiên/tuần; gói trang trí vượt ngân sách mobile → chặn | `game.event.write`; duyệt `game.content.review`; publish `game.content.publish` |
| **Giám sát sự kiện đang chạy** | E | Trạng thái run, số học sinh tham gia, tiến độ community goal theo phạm vi, tỷ lệ nhận thưởng, lỗi; nút **Tạm dừng khẩn cấp / Tiếp tục / Hủy** (bắt buộc nhập lý do, xác nhận 2 bước) | Dừng khẩn cấp không cắt lượt chơi đang dở; mọi thao tác ghi audit | `game.event.operate` [Proposed] |
| **Season Track** | A | Chọn mùa (từ lịch mùa §3.4), bảng 30 cấp × phần thưởng, nguồn điểm mùa + trần/ngày, mô phỏng "chơi N ngày/tuần đạt cấp nào", preview màn Season Track | `paid_track` luôn tắt; thưởng chưa nhận tự cấp khi hết mùa; không phần thưởng ảnh hưởng học tập | `game.season_track.write` [Proposed] |
| **Template sự kiện cho tenant** | A | Template + `allowed_params` (tên, ngày ≤ 28 ngày, đối tượng trong tenant, mục tiêu community goal, danh sách quest/challenge/phần thưởng được chọn); gán tenant (§3.7) | Chỉ tham chiếu nội dung platform đã publish | `game.event.write` |
| **Sự kiện của trường** | C | Danh sách sự kiện trường; tạo từ template đã gán (form chỉ hiện tham số cho phép); xem tiến độ community goal theo lớp (`getSchoolEventProgress`, module game, chỉ số tổng hợp theo lớp, không có danh sách học sinh); hủy | Tham số ngoài template → `422`; tên qua kiểm tra từ nhạy cảm; tối đa 3 sự kiện chạy đồng thời/tenant | `lms.school_event.write` (Tenant Admin, School Admin) [Proposed] |

- Song ngữ: form có hai cột vi/en cạnh nhau, chỉ báo độ đầy đủ bản dịch.
- Responsive: Event Builder và Season Track chỉ trên desktop/tablet ngang; điện thoại xem danh sách, lịch, giám sát và dừng khẩn cấp.

### 3.16. Danh mục: quốc gia, tỉnh/thành, trường, lớp (ADR-0060)

**Workspace D — Platform Administration (System Admin):**

| Màn | Chức năng | Ghi chú |
|---|---|---|
| Quốc gia | Danh sách, bật/tắt quốc gia (Release 1.0 chỉ `VN`), tên song ngữ | Không xóa; tắt = `inactive` |
| Tỉnh/thành | Danh sách theo quốc gia, lọc `active`/`inactive`, tìm theo tên/mã; xem **lịch sử hiệu lực** và **kế thừa** (đơn vị cũ → đơn vị mới); sửa tên song ngữ, nhập `official_code` khi đã đối chiếu văn bản | Seed từ [specs/reference-data](../../specs/reference-data/) (34 đơn vị VN hiện hành + 63 đơn vị cũ). Thay đổi địa giới = tạo đơn vị mới + kế thừa, không sửa đè |
| Danh mục trường (liên tenant) | Bảng trường với bộ lọc **tenant** (dropdown, ADR-0027 mục 6), **tỉnh/thành**, **cấp học**, trạng thái (`active`/`pending_review`/`inactive`), nguồn (SIS/thủ công); xem số lớp/học sinh (từ bảng tổng hợp, ADR-0062) | Thao tác ghi chọn một tenant đích |
| Import trường (CSV) | Tải CSV theo cột của DTO `School` trong [EduWorld SIS Contract](../../specs/sis/eduworld-sis-contract.v1.yaml) (`school_external_id`, `school_code`, `name`, `province_code`, `moet_code`, `school_level`…), dry-run hiển thị diff, áp dụng bất đồng bộ | Mã tỉnh cũ tự ánh xạ qua kế thừa; mã không hợp lệ → dòng lỗi |

**Workspace C — School & Learning Management (Tenant Admin, School Admin):**

| Màn | Chức năng | Ghi chú |
|---|---|---|
| Trường của tenant | Xem/sửa thông tin hiển thị (tên song ngữ, tỉnh/thành, cấp học, địa chỉ); **duyệt trường `pending_review`** (chọn tỉnh/thành khi SIS không gửi) | Trường do SIS sở hữu: trường dữ liệu SIS chỉ đọc (sửa ở SIS); tỉnh/thành sửa được khi SIS không cung cấp |
| Lớp | Danh sách lớp theo năm học/khối, sĩ số, số course đang dùng | Lớp SIS chỉ đọc; trường dùng CSV tạo lớp qua import |
| Học sinh chưa có lớp | Danh sách `reconciliation_issue` loại `missing_class` (học sinh không vào được game, ADR-0060) với hướng dẫn bổ sung lớp trong SIS/CSV | Không có thao tác "gán lớp tạm" trên portal |

Permission: `platform.catalog.write` (platform-only), `tenant.school.write`, `tenant.class.read`. Bảng endpoint đề xuất: [SCHOOL_LMS §11](SCHOOL_LMS.md) và mục API dưới đây.

### 3.17. Người dùng, Role và UserRole (ADR-0066, ADR-0067)

Ba màn trong workspace C (Tenant Admin; School Admin giới hạn trường mình) và workspace D (System Admin, mọi tenant + platform). API: [API_DESIGN §9.16](../07-backend/API_DESIGN.md#916-admin-internal--apiv1admin) (**spec: chưa**).

**a) Role** — danh sách role dùng được trong tenant: role hệ thống (`is_system`, nhãn "Hệ thống", chỉ xem) và role tùy biến của tenant [Proposed].

```text
┌ Role ───────────────────────────────────────────────────────── [+ Tạo role] ┐
│ 🔍 [Tìm role]   Loại: [Tất cả ▾]                                            │
│ Mã               Tên                    Loại        Số người  Cập nhật      │
│ school_admin     Quản trị trường        Hệ thống    12        —             │
│ tenant_admin     Quản trị đơn vị        Hệ thống    3         —             │
│ to_truong_toan   Tổ trưởng Toán         Tùy biến    5         12/11         │
└─────────────────────────────────────────────────────────────────────────────┘
┌ Sửa role: Tổ trưởng Toán ───────────────────────────────────────────────────┐
│ Mã [to_truong_toan]  Tên (vi) [Tổ trưởng Toán]  Tên (en) [Math lead]        │
│ Mô tả [..........................................]                          │
│ Permission (nhóm theo workspace, chỉ permission tenant):                    │
│  ▸ Lớp & giao bài   [✓] lms.assignment.write  [✓] lms.report.read  [ ] ...  │
│  ▸ Nội dung học tập [✓] learning_content.review [ ] learning_content.publish│
│  ▸ Báo cáo          [✓] analytics.school.read                               │
│ Ảnh hưởng: 5 người đang có role này — thay đổi áp dụng trong ≤ 60 s         │
│ [Hủy]                                              [Lưu (cần lý do)]        │
└─────────────────────────────────────────────────────────────────────────────┘
```

- Picker chỉ liệt kê permission `scope_kind = tenant` (`GET /tenant/permissions`); permission `game.*`, `competition.config.*`, `platform.*` không bao giờ xuất hiện, server vẫn kiểm tra (`422`, `PERMISSION_NOT_ALLOWED_FOR_SCOPE`).
- Role hệ thống: nút "Nhân bản thành role tùy biến" thay cho "Sửa". Xóa role còn người dùng → `409`, gợi ý thu hồi UserRole trước. Lưu có `If-Match` và bắt buộc lý do; phát `role.updated`.
- System Admin (workspace D) thấy thêm role platform và role tùy biến của mọi tenant (lọc tenant), tạo/sửa role platform (`platform.role.write`).

**b) Người dùng** — danh sách người có membership trong tenant; tạo nhân sự thủ công.

```text
┌ Người dùng ─────────────────────────────────────────────── [+ Thêm nhân sự] ┐
│ 🔍 [Tên, username, email]  Loại: [Nhân sự ▾]  Trường: [Tất cả ▾]  Role: [▾]  │
│ Họ tên          Username             Email                  Role (phạm vi) │
│ Cán bộ A        ad.truong.a          ad.a@thdemo.edu.vn     School Admin   │
│ Cán bộ B        qlcm.truong.a        qlcm.a@thdemo.edu.vn   Academic Mgr   │
└─────────────────────────────────────────────────────────────────────────────┘
Form thêm nhân sự: Họ tên* · Username* (gợi ý <vai trò>.<mã trường>, ví dụ ad.truong.a) · Email* · Role ban đầu* + phạm vi*
Lỗi: "Tên đăng nhập đã được dùng" (409 USERNAME_TAKEN) · "Email đã được dùng" (409 EMAIL_TAKEN)
     → gợi ý: "Nếu đây là cùng một người đang làm ở đơn vị khác, hãy gán role cho tài khoản sẵn có"
```

- Username và email kiểm tra trùng trên **SSO và trong tenant**, không phân biệt hoa thường (ADR-0067, ADR-0069); nếu trùng với tài khoản ở tenant khác mà là cùng một người thì gán role cho tài khoản sẵn có thay vì tạo mới; email bắt buộc với nhân sự. Học sinh hiển thị chỉ đọc (dữ liệu từ SIS), không gán được role portal.
- Học sinh đến từ SIS hiển thị nhãn "SIS"; thông tin định danh do SIS sở hữu không sửa tay được. Nhân sự portal luôn tạo thủ công (SIS chỉ đồng bộ trường, lớp và học sinh — ADR-0072).

**c) UserRole** — gán/thu hồi role theo người dùng, theo đơn vị/trường/môn, có thời hạn.

```text
┌ Phân quyền: Cán bộ B (qlcm.truong.a) ─────────────────────── [+ Gán role] ┐
│ Role              Phạm vi                Hiệu lực            Người gán     │
│ Academic Manager  Môn Toán · TH Demo A   01/09/26 → —        ad.truong.a   │
│ Tổ trưởng Toán    Môn Toán · TH Demo A   15/11/26 → 31/05/27 ad.truong.a [Thu hồi]│
└──────────────────────────────────────────────────────────────────────────┘
Hộp gán role: Role [▾ chỉ role người gán được phép cấp] · Phạm vi: (•) Đơn vị ( ) Trường [▾] ( ) Môn [▾]
              · Từ ngày [..] Đến ngày [..] (tùy chọn) · Lý do* · [Gán]
```

- Không gán role platform trong tenant, không gán role platform cho nhân sự có tenant gốc hay role tenant cho nhân sự platform (ADR-0081); không gán role portal cho học sinh (`422`). School Admin chỉ gán trong trường mình và chỉ role không vượt quyền của chính mình [Proposed].
- Mọi UserRole portal là thủ công; không có UserRole sinh từ SIS (ADR-0072).
- Mọi gán/thu hồi phát `user_role.changed`, ghi audit; người được gán thấy menu mới trong ≤ 60 s. **Nhật ký phân quyền** liệt kê `role.*`/`user_role.*` (ai, khi nào, trước/sau, lý do).

### 3.18. Feature flag · Bật/tắt menu & action (workspace D, ADR-0090)

Màn của **Platform Administration**, chỉ System Admin (`platform.feature_flag.write`, gateway nội bộ, MFA). Menu này không tắt được (§2.7). Quy tắc trạng thái: §2.7; API: `GET /api/v1/admin/portal-menu-catalog` (`listPortalMenuCatalog`), `GET /api/v1/admin/feature-flags` (`listFeatureFlags`), `PUT /api/v1/admin/feature-flags/{flag_key}` (`putFeatureFlag`).

```text
┌─ Bật/tắt menu & action ─────────────────────────────── Danh mục v2026.11.17-3 ┐
│ Phạm vi: (•) Mặc định platform  ( ) Tenant [▾ THCS Demo B]   🔍 [Tìm menu/action] │
│ [Tab: Menu & action] [Tab: Feature flag game]                                      │
├────────────────────────────────────────────────────────────────────────────────────┤
│ Mục                                         Mặc định   Ghi đè tenant  Hiệu lực   │
│ ▾ C. School & Learning Management                                                  │
│   ▾ Giao bài & lịch — Bài tập/đề             Bật        Tắt            ● Tắt  [Sửa]│
│       Tạo/sửa bài giao                       Bật        —              ○ Tắt (menu)│
│       Công bố kết quả                        Bật        —              ○ Tắt (menu)│
│   ▸ Thử thách lớp / giải trường              Tắt        Kế thừa        ● Tắt  [Sửa]│
│   ▸ Người dùng & phân quyền 🔒               Bật        —              ● Bật       │
│ ▸ B. Learning Content Studio                                                       │
│ [Nhật ký thay đổi]                                                                 │
└────────────────────────────────────────────────────────────────────────────────────┘
Hộp "Sửa": Trạng thái (•) Bật ( ) Tắt ( ) Kế thừa mặc định (chỉ khi phạm vi tenant)
           · Lý do* (≤ 1000 ký tự) · cảnh báo nếu menu có operation dùng chung · [Lưu]
```

| Thành phần | Hành vi |
|---|---|
| Cây danh mục | Workspace → menu → action theo `order`; nhãn theo `label_key`; mục `can_disable = false` có biểu tượng khóa 🔒, không có nút Sửa |
| Phạm vi | Mặc định platform (`scope = platform`) hoặc một tenant đích (`scope = tenant`, gửi `X-Tenant-Id` = tenant đích); không có ghi hàng loạt "mọi tenant" |
| Cột | Mặc định platform · Ghi đè của tenant đang chọn (`—` nếu không có) · Hiệu lực (action tắt vì menu cha tắt hiện "Tắt (menu)") |
| Sửa | Hộp chọn Bật/Tắt/Kế thừa (Kế thừa = gỡ ghi đè, chỉ ở phạm vi tenant) + lý do bắt buộc; gửi kèm `If-Match` = version đang xem |
| Xung đột | `412 VERSION_MISMATCH` → "Mục đã được người khác thay đổi", tải lại dòng, giữ lý do đã nhập |
| Mục bảo vệ | Nếu gọi API trực tiếp: `422 FEATURE_NOT_DISABLEABLE` |
| Nhật ký | Mở Nhật ký toàn hệ thống lọc `action = feature_flag.updated` (ai, khi nào, phạm vi, trước/sau, lý do) |
| Tab Feature flag game | Flag `flag_kind = feature` (bootstrap game/portal, F015) dùng cùng hộp Sửa |

Thay đổi có hiệu lực ở mọi phiên trong thời gian `feature_toggle_cache_ttl` ([VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md)); người dùng đang mở menu vừa bị tắt nhận `403 FEATURE_DISABLED` ở thao tác kế tiếp và menu được ẩn sau khi portal tải lại quyền.

## 4. Workflow duyệt

```mermaid
sequenceDiagram
  actor A as Author
  participant CMS as Admin Portal
  participant API as module learning (cms) / module game (game-admin)
  actor R as Reviewer
  A->>CMS: Soạn draft
  CMS->>API: PATCH /cms/learning/questions/{id}/versions/{v} (autosave, If-Match)
  A->>CMS: Gửi review
  CMS->>API: POST .../versions/{v}:transition (submit_review)
  API->>API: kiểm tra permission + tenant, validate schema, media ready
  API-->>R: ReviewTask (notification, chỉ reviewer cùng lane và cùng tenant)
  R->>CMS: Mở review: diff với version trước, preview
  alt Đạt
    R->>API: approve (comment)
    A->>API: publish (ngay / publish_at)
  else Không đạt
    R->>API: reject (reason, comment theo field)
    API-->>A: notification
  end
```

- **ReviewTask**: `task_id`, `tenant_id` (NULL với lane platform), `content_type`, `content_version_id`, `review_lane` (`tenant` | `platform_learning` | `platform_game`), `assignee_scope`, `assignee_id?`, `status`, `due_at`, `comments[]` (gắn field path).
- Hàng đợi review lọc theo lane, tenant và scope reviewer. Reviewer tenant không bao giờ thấy task lane platform hay task của tenant khác. Task chờ quá `content_review_sla_days` = 3 ngày hiển thị cảnh báo; `due_at` = thời điểm gửi duyệt + `content_review_sla_days`.
- Duyệt hàng loạt tối đa `content_review_bulk_max` = 50 item mỗi lần; mỗi item gọi riêng với `Idempotency-Key` riêng và vẫn kiểm tra reviewer ≠ author.
- Khóa sửa đồng thời và autosave: §1.4.

## 5. Preview

| Loại | Cách preview | Workspace |
|---|---|---|
| Question / Question Set | Renderer web dùng chung component với LMS + chế độ "thử làm" (chấm bằng API preview, không lưu attempt; hiện gợi ý từng cấp và phản hồi theo phương án như học sinh thấy) | B |
| Lesson | Renderer web | B |
| Mini-game / Quest / NPC dialogue / Challenge / Boss (draft) | Godot Web export nhúng nạp cấu hình draft qua token preview ngắn hạn (Phase 3) [Proposed] | A |
| Quest/mini-game đã publish và được gán (để nhà trường xem trước khi giao) | Mô tả, LO, ảnh/video minh họa; chạy thử bản published | C (chỉ đọc) |
| Assessment Blueprint | Sinh 1 đề mẫu (không lưu ExamPaper) | B |

Preview game chạy trong component `GamePreviewFrame` (iframe riêng chứa Godot Web export, renderer Compatibility/WebGL 2); phần còn lại của màn hình theo Admin/Ops UI (ADR-0047). Danh sách và form hiển thị thumbnail/turntable render sẵn từ asset 3D (DESIGN_SYSTEM §7) để không phải tải bản preview chỉ để nhận diện nội dung.

Preview API: `POST /api/v1/cms/learning/preview/sessions` (nội dung học tập) và `POST /api/v1/game-admin/preview/sessions` (nội dung game) → `preview_token` (hạn `preview_token_ttl` = 30 phút, chỉ đọc draft được chỉ định). Preview **không** tạo learning event, XP hay attempt.

## 6. Publish & rollback (ADR-0043)

Mô hình version, release pointer, pinning, snapshot/cache và thời gian hiệu lực: **duy nhất** ở [CONTENT_MODEL §5](../02-learning/CONTENT_MODEL.md#5-version-release-pointer-và-draftpublish-adr-0015-adr-0043). Màn hình phát hành: §3.11. Quy tắc riêng của portal:

- Rollback bắt buộc lý do và permission `content.release.rollback`; ghi audit.
- Rollback curriculum version chỉ khi chưa tenant nào chọn version mới làm mặc định, hoặc có phê duyệt System Admin.

## 7. Audit trail

Truy vấn nhật ký audit trên portal giới hạn mỗi lần tối đa `audit_query_max_range_days` = 93 ngày [Confirmed — Technical Architect 2026-11-16]; khoảng dài hơn dùng xuất báo cáo bất đồng bộ.

Mọi thao tác ghi `audit.audit_logs`:

| Field | Ví dụ |
|---|---|
| `actor_id`, `actor_roles`, `azp`, `workspace`, `acr` | `eduworld-portal`, `game_ops`, `mfa` |
| `tenant_id` / `tenant_ids` | Tenant của thao tác; với xem liên tenant của System Admin: danh sách tenant trong bộ lọc |
| `action` | `sensitive_term.created/updated/deleted`, `sensitive_catalog.published`, `chat_policy.moderation_switch_changed`, `chat_policy.tenant_category_toggled`, `content.version_created`, `content.submitted`, `content.approved`, `content.rejected`, `content.published`, `content.archived`, `content.rolled_back`, `content.unpublished`, `content.publish_scheduled`, `content.deleted`, `content.restored`, `content.re_evaluated`, `media.uploaded`, `game_config.changed`, `tenant_data.assigned`, `tenant_data.unassigned`, `tenant_data.cloned`, `cross_tenant.accessed`, `moderation.case.viewed`, `moderation.case.messages_viewed`, `school_notification.sent`, `school_notification.acknowledged`, `authz.denied` |
| `resource_type`, `resource_id`, `version` | `question`, uuid, 4 |
| `diff_summary` | Danh sách field thay đổi |
| `reason` | Bắt buộc với reject/archive/rollback/delete nội dung đã publish/restore |
| `ip_hash`, `user_agent`, `trace_id`, `at` | |

Mọi lần bị từ chối vì thiếu permission ở `/game-admin/*`, `/admin/*` hoặc gọi thiếu MFA / không phải client `eduworld-portal` ghi `authz.denied` và cảnh báo bảo mật nếu lặp lại (≥ 5 lần/10 phút [Proposed]).

## 8. API (tóm tắt)

| Vùng | Prefix | Gateway / điều kiện token | Endpoint mẫu |
|---|---|---|---|
| Nội dung học tập | `/api/v1/cms/learning/` | Public (tenant) · nội bộ (dữ liệu platform) | `curricula`, `questions`, `question-sets`, `lessons`, `assessment-blueprints`, `media`, `imports`, `review-tasks`, `preview/sessions`, `audit` |
| Workflow | `/api/v1/cms/learning/content/{type}/{id}/versions/{v}:transition` | như trên | `submit_review/approve/reject/archive` |
| Phát hành (ADR-0043) | `/api/v1/cms/learning/content/{type}/{id}/` (tương tự `/api/v1/game-admin/{config_type}/{id}/`) | như trên | `versions`, `versions/{v}`, `versions/{v}:diff?with=`, `versions/{v}:publish`, `:rollback`, `:unpublish`, `releases` |
| Media (ADR-0044) | `/api/v1/cms/media` (game: `/api/v1/game-media`, chỉ gateway nội bộ) | như trên | `media:presign-upload` (`presignMediaUpload`), `media/{asset_id}:complete-upload` (`completeMediaUpload`, báo upload xong để vào pipeline), `media/{asset_id}`, `media?kind=&status=&q=` |
| LMS | `/api/v1/lms/` | như trên | `assignments` (+ `:publish`, `:cancel`, `:release-results`, `:archive`, `:remind`), `challenges`, `catalog/*`, `courses`, `learning-paths`, `schedules`, `dashboards/school`, `reports` ([SCHOOL_LMS §11](SCHOOL_LMS.md#11-api)) |
| Thông báo an toàn (trường) | `/api/v1/tenant/safety/` | Public | `school-notifications` (`moderation.school_notification.read`), `school-notifications/{notification_id}:acknowledge` (`moderation.school_notification.acknowledge`) [Proposed] |
| Tenant & trường | `/api/v1/tenant/`, `/api/v1/schools/{school_id}/` | như trên | `tenant/settings` (gồm `inactive_student_cleanup_days`, `gameplay_chat_retention_days`), `tenant/trash` [Proposed], `tenant/students/{id}/login-events`, `tenant/student-cleanup/preview`, `tenant/student-cleanup/exclusions`, `schools/{id}/settings`, `sis-imports` |
| Đăng nhập portal (ADR-0091) | `/api/v1/auth/portal/` | Public, không token, không `X-Tenant-Id`; không log body | `login` (`portalLogin`), `token:refresh` (`refreshPortalToken`, cookie + `X-EW-CSRF`), `logout` (`portalLogout`, cookie + `X-EW-CSRF`), `setup:password` (`setupPortalPassword`), `password:forgot` (`forgotPortalPassword`), `password:reset` (`resetPortalPassword`) |
| Menu & tenant | `/api/v1/me/permissions`, `/api/v1/me/tenants` | `eduworld-portal` | `me/tenants` (không cần `X-Tenant-Id`) → tenant được vào sau đăng nhập; `me/permissions` (header `X-Tenant-Id`) → role + permission + workspace + `disabled_menus[]`/`disabled_actions[]` (§2.7) |
| Phân quyền (ADR-0066) | `/api/v1/tenant/`, `/api/v1/admin/` | Public (tenant) · nội bộ (platform) | `tenant/permissions`, `tenant/roles`, `tenant/roles/{id}`, `tenant/user-roles`, `tenant/users`; `admin/permissions`, `admin/roles`, `admin/user-roles` — **spec: chưa** |
| **Game Studio** | `/api/v1/game-admin/` | **chỉ gateway nội bộ · `eduworld-portal` + MFA + role platform** | `{config_type}` (quests, minigame-configs, npc-dialogues, events, event-templates, items, reward-rules, progression-levels, sticker-packs, gifs…), `preview/sessions` |
| **Cấu hình thi đấu** | `/api/v1/game-admin/` | như trên | `matchmaking-configs`, `ranking-rules`, `seasons`, `tournament-templates`, `challenge-templates`, `leaderboard-definitions` |
| Bật/tắt menu & action (ADR-0090) | `/api/v1/admin/` | chỉ gateway nội bộ · `eduworld-portal` + MFA · `platform.feature_flag.write` | `portal-menu-catalog` (`listPortalMenuCatalog`), `feature-flags` (`listFeatureFlags`), `feature-flags/{flag_key}` (`putFeatureFlag`, `If-Match`, lý do, `X-Tenant-Id` = tenant đích khi ghi đè) |
| Platform | `/api/v1/admin/` | như trên | `tenants`, `tenant-assignments`, `tenant-clones`, `trash`, `users`, `feature-flags`, `login-events`, `settings/student-cleanup`, `student-cleanup/runs` (+ `:approve`), `settings/chat-retention` |
| **An toàn chat (platform)** | `/api/v1/admin/chat/` | **chỉ gateway nội bộ · `eduworld-portal` + MFA + role platform** | `sensitive-categories`, `sensitive-terms` (+ `:test`), `sensitive-allowlist`, `sensitive-catalog:publish`, `moderation-switches` |
| An toàn chat (tenant, chỉ đọc) | `/api/v1/tenant/chat/` | Public | `categories` (danh mục hiệu lực, `chat.sensitive_terms.read`) |
| **Kiểm duyệt (Platform Safety Team)** | `/api/v1/admin/moderation/`, `/api/v1/admin/chat/tenant-terms`, `/api/v1/admin/chat/tenant-categories/{code}:toggle` | **chỉ gateway nội bộ · `eduworld-portal` + MFA + role platform** | `cases`, `cases/{id}`, `cases/{id}/messages`, `cases/{id}:act`, `sanctions`; thông báo trường `GET /api/v1/admin/safety/school-notifications`; ghi gửi `X-Tenant-Id` = tenant của case [Proposed] |

Quy ước liên tenant: list endpoint nhận `tenant_ids[]` (chỉ System Admin, không gửi `X-Tenant-Id`); ghi của System Admin gửi `X-Tenant-Id` = tenant đích; response cho System Admin có `tenant_id`. Danh mục đầy đủ: [API_DESIGN](../07-backend/API_DESIGN.md) và `specs/openapi/eduworld-api.v1.yaml`.

## 9. Bảo vệ lịch sử học tập

| Thao tác CMS | Ảnh hưởng lên attempt/learning event đã có |
|---|---|
| Sửa câu hỏi đã publish | Không — tạo version mới; attempt cũ trỏ version cũ |
| Archive câu hỏi / question set | Không — vẫn render khi xem lại bài cũ; chỉ loại khỏi chọn mới |
| Soft-delete câu hỏi / question set | Không — bản ghi vẫn tồn tại (ADR-0028); bài cũ vẫn xem lại được; chỉ ẩn khỏi danh sách và chọn mới |
| Gỡ gán dữ liệu platform khỏi tenant | Không — attempt/assignment cũ giữ `content_version_id`; chỉ không giao mới được |
| Đổi đáp án đúng | Không tự động; chỉ qua **regrade** có phê duyệt, lưu revision ([EXAM_ENGINE §9](../02-learning/EXAM_ENGINE.md#9-chấm-lại-regrade)) |
| Đổi curriculum version | Không sửa LO trong event cũ; mastery chiếu qua mapping |
| Đổi reward rule (platform) | Không thu hồi phần thưởng đã cấp; áp cho event sau `effective_from` |

Kiểm thử bắt buộc: test tích hợp cho từng dòng bảng trên.

## 10. Màn hình chính (sitemap theo workspace)

Một portal, menu từng workspace sinh từ danh mục menu (§2.7), lọc theo permission và trạng thái bật/tắt. System Admin thấy bộ lọc tenant trên topbar (mặc định "Tất cả tenant", chọn nhiều) và cột Tenant ở mọi danh sách có dữ liệu theo tenant; nhân sự thuộc nhiều tenant thấy bộ chọn đơn vị làm việc (§1.3b).

```text
[/login của portal — username/email + mật khẩu + mã xác thực 6 số (một request, ADR-0091)] → ([Đặt mật khẩu mới] · [Kiểm tra email cài ứng dụng xác thực] khi ACCOUNT_SETUP_REQUIRED) → [Chọn đơn vị — chỉ khi nhiều tenant]
[/forgot-password] → email → [/reset-password?token=…] → [/login]
EduWorld Admin Portal   [Workspace ▾]   [Tenant ▾ — System Admin: lọc · nhân sự nhiều tenant: chọn đơn vị]
│
├── C. School & Learning Management   (tenant · System Admin)
│   ├── Tổng quan (việc cần làm: review chờ, bài sắp hạn, import lỗi, thông báo an toàn chờ xác nhận)
│   ├── Trường (tenant unit) · Lớp & học sinh (từ SIS, chỉ đọc) · Đồng bộ SIS/CSV
│   ├── Khóa học & lộ trình học (course tự tạo từ lớp × môn theo khối)
│   ├── Giao bài & lịch — Bài tập / đề · Bài chẩn đoán · Hoạt động game được gán (chỉ đọc) · Nhóm học tập · Thử thách lớp / giải trường (template, School Admin)
│   ├── Tiến độ & báo cáo — School Learning Dashboard (trường → lớp → học sinh) · Bản đồ kỹ năng · Kết quả bài/đề (hộp thoại chấm lại đề `scope = exam_session`, Academic Manager) · Báo cáo tham khảo TT27/TT22 · Analytics
│   ├── An toàn — Thông báo vụ việc (xác nhận đã liên hệ phụ huynh) · Chat policy trường · Nhóm từ của tenant (chỉ xem)
│   ├── Hoạt động học sinh (lần đăng nhập game cuối, lịch sử phiên)
│   ├── Người dùng & phân quyền — Người dùng (nhân sự, học sinh chỉ đọc) · UserRole · Role (hệ thống chỉ đọc, tùy biến [Proposed]) · Nhật ký phân quyền
│   ├── Cài đặt tenant / trường (khung giờ truy cập, thông báo, dọn học sinh, thời hạn chat gameplay khi được phép)
│   ├── Thùng rác (Tenant Admin, nếu được bật) [Proposed]
│   └── Nhật ký (audit trong tenant)
│
├── B. Learning Content Studio   (tenant: dữ liệu tenant + dữ liệu được gán chỉ đọc · platform: dữ liệu platform)
│   ├── Chương trình học — Curriculum · Khối · Môn · Chủ đề · Learning Objective · Learning Unit
│   ├── Bài học (Lesson Builder) · Learning Activity Builder (theo activity type)
│   ├── Ngân hàng câu hỏi · Bộ câu hỏi · Quiz/Exercise · Ma trận đề / Assessment · Chấm lại hệ thống
│   ├── Video · Audio · Mô phỏng · Media
│   ├── Lộ trình học (Learning Path Builder)
│   └── Duyệt nội dung (lane tenant / platform_learning) · Preview
│
├── A. Game Operations Console   (chỉ platform · gateway nội bộ · MFA)
│   ├── Game Dashboard [TBD]
│   ├── Thế giới — World · Zone · Area · NPC & hội thoại · World event
│   ├── Quest Builder · Challenge Builder · Boss Management
│   ├── Level & XP · Achievement · Item / Shop / Tiền tệ · Phần thưởng
│   ├── Sự kiện & mùa · Thi đấu (matchmaking, ranking, mùa giải, template, leaderboard)
│   ├── Tài nguyên chat — Sticker pack · Thư viện GIF
│   ├── Người chơi · Party & nhóm [TBD]
│   └── Duyệt & publish (lane platform_game) · Preview Godot Web
│
├── E. Game Live Operations   (chỉ platform · gateway nội bộ · MFA)
│   ├── Live dashboard (CCU, phiên, sự kiện đang chạy, hàng chờ ghép trận, party, sức khỏe thế giới/server)
│   ├── Game Analytics (tổng quan, retention, onboarding, quest, challenge/boss, tiến trình, kinh tế, xã hội, sự kiện, chơi lành mạnh, hiệu năng thiết bị — §3.14)
│   ├── Kiểm duyệt — Platform Safety Team (Safety Moderator, System Admin): hàng đợi case mọi tenant · sanction · thông báo nhà trường · từ bổ sung / nhóm từ của tenant · lịch trực on-call · báo cáo khẩn
│   └── Sự cố (incident) [TBD]
│
└── D. Platform Administration   (chỉ System Admin · gateway nội bộ · MFA)
    ├── Tenant (trường độc lập, đơn vị, tenant chung; kích hoạt; SIS connection)
    ├── Người dùng & phân quyền — Người dùng (mọi tenant) · Role platform & role dùng chung · UserRole (platform + tenant đích) · Permission catalog (chỉ xem)
    ├── Gán cho tenant · Clone giữa tenant (job)
    ├── Chính sách trường · An toàn chat — Danh mục từ nhạy cảm · Công tắc kiểm duyệt · Break-glass
    ├── Cấu hình hệ thống — Dọn học sinh · Thời hạn chat gameplay · Retention · Feature flag · Bật/tắt menu & action (§3.18)
    ├── Tích hợp — Keycloak · SIS · APISIX (tham chiếu, chỉ xem) · Integration Monitoring [TBD]
    ├── Thùng rác (mọi tenant + platform)
    └── Nhật ký toàn hệ thống
```

### 10.1. Đặc tả màn hình chính

Mỗi màn dưới đây áp dụng bố cục §1.3 và editor §1.4. Lỗi API hiển thị theo `code` của Problem Details (thông điệp tiếng Việt, kèm `trace_id` để báo hỗ trợ).

| Màn | Mục đích | Vai trò | Component chính | Hành động | Dữ liệu | Trạng thái | Validation | Lỗi thường gặp | Quyền | Responsive |
|---|---|---|---|---|---|---|---|---|---|---|
| Danh sách nội dung (câu hỏi, activity, quest…) | Tìm và quản lý nội dung | Biên tập, reviewer, Academic Manager, đội game | QueryFilter + ProTable + bulk action + cột trạng thái/version/tenant | Lọc, tạo, nhân bản nháp, gửi duyệt hàng loạt, xóa mềm, gán tenant (System Admin) | Mã, tiêu đề, loại, khối/môn/LO, trạng thái, version xuất bản, người sửa, thời gian | Rỗng (gợi ý tạo mới), đang tải (skeleton), lỗi (thử lại), không có quyền | Bộ lọc giới hạn độ dài theo spec | `PERMISSION_DENIED`, `TENANT_MISMATCH` | `*.read` | Tablet: ẩn cột phụ; điện thoại: dạng thẻ, chỉ xem |
| Editor nội dung | Soạn/duyệt một item | Biên tập, reviewer | `ContentEditorShell`, panel thuộc tính, preview, `MathInput`/`MathText` | Lưu nháp, gửi duyệt, duyệt/từ chối, publish/hẹn giờ, rollback, so sánh version | Nội dung version, metadata, lịch sử, comment review | Nháp / đang duyệt / đã duyệt / đã xuất bản / có nháp mới / gỡ xuất bản | JSON Schema loại nội dung + `maxLength` + kiểm tra nghiệp vụ server + kiểm tra LaTeX/sanitize SVG công thức | `CONTENT_VERSION_CONFLICT`, `MEDIA_NOT_READY`, `VALIDATION_FAILED`, `MATH_RENDER_FAILED` | `*.write`/`*.review`/`*.publish` | Desktop/tablet; điện thoại chỉ duyệt |
| School Learning Dashboard | Trả lời 5 câu hỏi của nhà trường ([SCHOOL_LMS §8](SCHOOL_LMS.md#8-school-learning-dashboard)) | School Admin, Academic Manager | StatisticCard, heatmap lớp × LO / học sinh × LO, ProTable học sinh, chart | Lọc khối/lớp/môn/thời gian, drill-down trường → lớp → học sinh, giao hoạt động/ôn tập cho lớp | Tiến độ bài giao, mastery, cờ "làm quá nhanh", tự học sau gợi ý, lần vào game cuối, thông báo an toàn chờ xác nhận | Chưa có dữ liệu, dữ liệu trễ (eventual), "Tạm tính" | — | `PERMISSION_DENIED` (ngoài scope), `DASHBOARD_RANGE_TOO_LARGE` | `analytics.school.read` | Tablet đầy đủ; điện thoại tóm tắt |
| Thông báo an toàn của trường | Nhận thông báo vụ việc, xác nhận đã liên hệ phụ huynh (§1.5.2) | School Admin, Tenant Admin | ProTable + panel chi tiết | Lọc trạng thái/lớp, ghi chú, xác nhận | Tóm tắt do Safety Team ghi, mức độ, thời điểm yêu cầu, hạn; **không** có nội dung chat | Chờ xác nhận / đã xác nhận / quá hạn | Ghi chú ≤ 500 ký tự | `PERMISSION_DENIED` (trường khác), `CONFLICT` (đã xác nhận) | `moderation.school_notification.read`, `.acknowledge` | Mọi breakpoint |
| Hàng đợi case kiểm duyệt | Xử lý case liên tenant | Safety Moderator, System Admin | QueryFilter (dropdown tenant) + ProTable + panel bằng chứng | Ẩn tin, sanction, chuyển cấp, đóng case, gửi thông báo trường | Case, bằng chứng, tin nhắn liên quan case, lịch sử sanction | Mới / đang xử lý / chờ trường xác nhận / đóng | Lý do bắt buộc khi sanction/đóng | `MFA_REQUIRED`, `PERMISSION_DENIED` | `moderation.case.*`, `moderation.sanction.*` | Desktop/tablet; điện thoại xem và xử lý nhanh |
| Builder game (quest, challenge, boss, level/XP, achievement, item) | Cấu hình nội dung game | Game Designer, Live-ops | `ContentEditorShell`, graph/kéo-thả, thumbnail 3D, `GamePreviewFrame` | Soạn, preview Godot, gửi duyệt, publish, rollback, gán tenant | Cấu hình version, tham chiếu activity/scene | Như editor | Schema + tham chiếu tồn tại trong client manifest | `MFA_REQUIRED`, `CONTENT_VERSION_CONFLICT` | `game.*` | Chỉ desktop |
| Live dashboard | Giám sát vận hành | Live-ops, System Admin | StatisticCard, chart thời gian thực (làm mới 15–30 s), bảng sự kiện | Lọc tenant/khu vực, mở chi tiết, chuyển sang kiểm duyệt | CCU, phiên, trận, hàng chờ, tin bị chặn, lỗi | Bình thường / cảnh báo / sự cố | — | Mất kết nối gateway nội bộ ("Cần VPN") | `game.liveops.read` [Proposed] | Desktop; tablet xem |
| Quản lý tenant | Tạo/sửa tenant | System Admin | ProTable + ProForm | Tạo, kích hoạt/tắt, cấu hình SIS, gán nội dung | Mã, loại, trạng thái, số trường/học sinh | Đang hoạt động / tạm tắt | Mã tenant duy nhất, ≤ 64 ký tự | `CONFLICT` | `platform.tenant.manage` | Desktop |
| Đăng nhập portal (`/login`, §1.3b) | Nhân sự đăng nhập không rời portal | Mọi nhân sự (học sinh bị chặn) | ProForm (username/email, mật khẩu, mã 6 số), chọn ngôn ngữ, link Quên mật khẩu | Đăng nhập (một request), đổi mật khẩu tạm, quên/đặt lại mật khẩu | Không lưu mật khẩu/mã; nhớ per-browser lần trước có dùng mã | Đang gửi, lỗi chung, cần thiết lập tài khoản, bị giới hạn số lần thử | Username/email ≤ 254, mã đúng 6 chữ số | `INVALID_CREDENTIALS`, `ACCOUNT_SETUP_REQUIRED`, `PORTAL_ACCESS_STAFF_ONLY`, `RATE_LIMITED` | Không cần token | Mọi breakpoint |
| Bật/tắt menu & action (§3.18) | Ngừng/mở lại menu, action của portal không cần deploy | System Admin | Tree + ProTable, chọn phạm vi platform/tenant, ModalForm lý do | Bật/tắt/kế thừa theo phạm vi, xem nhật ký | Danh mục (workspace → menu → action), mặc định, ghi đè tenant, hiệu lực, version | Bật / tắt / tắt do menu cha / khóa (không tắt được) | Lý do bắt buộc ≤ 1000 ký tự, `If-Match` | `FEATURE_NOT_DISABLEABLE`, `VERSION_MISMATCH`, `MFA_REQUIRED` | `platform.feature_flag.write` | Desktop |


## 11. Yêu cầu chức năng

Mã chuẩn `FR-CMS-*`/`FR-LMS-*` nằm trong [PRODUCT_REQUIREMENTS §2.26–2.27](../00-overview/PRODUCT_REQUIREMENTS.md). Bảng dưới là yêu cầu chi tiết ở mức thiết kế, đánh mã `CMS-D-NNN` để không trùng số với PRD.

| ID | Yêu cầu | Nhãn |
|---|---|---|
| CMS-D-001 | Role tenant: Tenant Admin, School Admin, Academic Manager, Tenant Content Editor/Reviewer; role platform: System Admin, Content Admin, Content Reviewer, Game Designer, Live-ops, Competition Admin, Safety Moderator. Không có Teacher, Tenant Moderator | [Confirmed] ADR-0072 |
| CMS-D-002 | Module 8.1–8.4; 8.3 và 8.4 chỉ dành cho platform | [Confirmed] |
| CMS-D-003 | Workflow + versioning + preview + publishing + rollback + audit | [Confirmed] |
| CMS-D-004 | Không thay đổi lịch sử bài tập đã hoàn thành | [Confirmed] |
| CMS-D-005 | Reviewer ≠ author | [Proposed] |
| CMS-D-006 | Publish theo lịch | [Proposed] |
| CMS-D-007 | Godot Web preview cho game content | [Proposed] |
| CMS-D-008 | Bulk import có báo lỗi theo dòng | [Proposed] |
| CMS-D-009 | Game content chỉ tham chiếu scene có trong client manifest | [Proposed] |
| CMS-D-010 | Vận hành tenant đăng nhập CMS và thao tác chức năng Learning Management trong đúng một tenant | [Confirmed] ADR-0027 |
| CMS-D-011 | Vận hành tenant không thấy và không gọi được menu/API cấu hình game (`/api/v1/game-admin/*`) | [Confirmed] ADR-0027 |
| CMS-D-012 | Permission catalog trong code; role tenant không chứa `game.*`/`competition.config.*`/`platform.*`; kiểm tra lúc khởi động + trigger DB | [Confirmed] ADR-0027 |
| CMS-D-013 | **Một Admin Portal** (`web/apps/portal`, client `eduworld-portal`) với 5 workspace; workspace hiển thị theo permission; mã workspace lazy-load (không phải lớp bảo mật); workspace platform gọi gateway nội bộ, token bắt buộc MFA (thay mô hình 2 bundle) | [Confirmed] ADR-0027 mục 7 |
| CMS-D-014 | Menu sinh từ danh mục `specs/portal/menu-catalog.yaml`, lọc theo `GET /api/v1/me/permissions` (permission + `disabled_menus`/`disabled_actions`) | [Confirmed] ADR-0027, ADR-0090 |
| CMS-D-015 | Tenant tạo thử thách lớp/giải trường từ template được gán, chỉ điền tham số cho phép | [Proposed] ADR-0027 |
| CMS-D-016 | System Admin là người duy nhất xem dữ liệu liên tenant: dropdown tìm kiếm tenant (mặc định "Tất cả", chọn nhiều), cột Tenant, ghi luôn chọn một tenant đích | [Confirmed] ADR-0027 §6 |
| CMS-D-017 | Xóa = soft-delete; danh sách không hiển thị dữ liệu đã xóa; Thùng rác để xem/khôi phục (System Admin; Tenant Admin nếu được cấp) | [Confirmed] ADR-0028 / [Proposed] Tenant Admin |
| CMS-D-018 | System Admin gán dữ liệu platform cho tenant (chỉ đọc với tenant) và gỡ gán | [Confirmed] ADR-0029 |
| CMS-D-019 | System Admin clone dữ liệu cấu hình/nội dung từ tenant nguồn sang một tenant đích, bất đồng bộ, không clone dữ liệu học sinh | [Confirmed] ADR-0029 |
| CMS-D-020 | System Admin quản lý danh mục từ nhạy cảm platform (category, term, allowlist, thử chuỗi, publish có version) | [Confirmed] ADR-0030 |
| CMS-D-021 | System Admin bật/tắt công tắc kiểm duyệt (tổng/từng bước) theo platform/tenant, lý do bắt buộc, thời hạn tùy chọn, audit, banner cảnh báo | [Confirmed] ADR-0030 |
| CMS-D-022 | Tenant thêm từ bổ sung (chỉ siết chặt) và bật/tắt category khi `allow_tenant_toggle` | [Proposed] ADR-0030 |
| CMS-D-023 | Draft/Publish cho mọi nội dung có version; publish ngay hoặc hẹn giờ | [Confirmed] ADR-0043 |
| CMS-D-024 | Xem lịch sử version, xem nội dung từng version đã publish (chỉ đọc), diff, preview trong game | [Confirmed] ADR-0043 |
| CMS-D-025 | Rollback = dùng lại version đã publish cho học sinh (đổi con trỏ, lý do bắt buộc, audit); unpublish | [Confirmed] ADR-0043 |
| CMS-D-026 | Thư viện media: upload presigned lên object storage, theo dõi pipeline, alt text, "đang dùng ở đâu" | [Confirmed] ADR-0044 |
| CMS-D-027 | Learning Content Studio chỉ có builder cho hoạt động server tự chấm; không có rubric, hàng đợi chấm, trả bài | [Confirmed] ADR-0072 |
| CMS-D-028 | School Learning Dashboard cho School Admin/Academic Manager (trường → lớp → học sinh), đọc rollup | [Confirmed] ADR-0072 |
| CMS-D-029 | Safety Moderator đọc liên tenant chỉ trong module moderation qua `tenancy.Bypass(ctx, "moderation_case")`, có audit và metric | [Confirmed] ADR-0072 |
| CMS-D-030 | Nhà trường nhận thông báo vụ việc nghiêm trọng (tóm tắt, không chat thô) và xác nhận đã liên hệ phụ huynh; case lưu thời điểm yêu cầu/xác nhận | [Confirmed] ADR-0072 |
| CMS-D-031 | Từ bổ sung và bật/tắt nhóm từ cấp tenant chỉ Safety Team ghi theo yêu cầu tenant; role tenant không có quyền ghi | [Confirmed] ADR-0072 |
| CMS-D-032 | Question editor có gợi ý nhiều cấp và phản hồi theo phương án sai (chỉ trả sau khi chấm); khối lớp 1–3 bắt buộc lồng tiếng đề bài trước khi publish | [Proposed] PED-04, PED-07 |
| CMS-D-033 | Danh mục menu/action của portal là contract (`specs/portal/menu-catalog.yaml`) có bản sao đọc-chỉ trong DB; menu portal sinh từ danh mục; mỗi operation portal gọi khai báo `x-portal-action` (§2.7) | [Confirmed] ADR-0090 |
| CMS-D-034 | Chỉ System Admin bật/tắt menu/action theo mặc định platform hoặc ghi đè cho một tenant đích (lý do, `If-Match`, audit); mục bảo vệ không tắt được; backend chặn `403 FEATURE_DISABLED`, portal ẩn menu/action đang tắt (§2.7, §3.18) | [Confirmed] ADR-0090 |
| CMS-D-035 | Màn đăng nhập portal tự vẽ, gửi mật khẩu và mã xác thực trong một request qua module identity, lỗi chung; đổi mật khẩu tạm, đăng ký mã xác thực qua email, quên/đặt lại mật khẩu; access token trong bộ nhớ, refresh token trong cookie `HttpOnly` có CSRF; đổi tenant bằng `X-Tenant-Id` (§1.3b) | [Confirmed] ADR-0091 |

## 12. Kiểm thử phân quyền bắt buộc

Các TC ghi "(đổi — ADR-0072)" dùng tác nhân theo role hiện hành (§2.3); nhóm `TC-CMS-SAF-*`, `TC-CMS-LMS-*` mới theo ADR-0072. Không TC nào bị xóa mã.

| ID | Kịch bản | Kỳ vọng |
|---|---|---|
| TC-CMS-AUTHZ-01 | School Admin (token `eduworld-portal`) gọi `POST /api/v1/game-admin/quests` qua gateway public | Không có route (404 tại APISIX); không chạm backend |
| TC-CMS-AUTHZ-02 | Token `eduworld-portal` của Tenant Admin gọi thẳng backend `/api/v1/game-admin/quests` (bỏ qua gateway, môi trường test) | 403 `PERMISSION_DENIED`, audit `authz.denied` |
| TC-CMS-AUTHZ-03 | System Admin gán role `game_designer` ở scope `tenant` | Bị từ chối (trigger DB + validate use case), audit |
| TC-CMS-AUTHZ-04 | Catalog code có role tenant chứa `game.quest.write` | Tiến trình không start (test unit trên catalog) |
| TC-CMS-RBAC-01 | Tenant Admin tạo role tùy biến chứa `game.quest.write` hoặc `platform.cross_tenant.read` (gọi API trực tiếp, bỏ qua picker) | `422 VALIDATION_FAILED` (`PERMISSION_NOT_ALLOWED_FOR_SCOPE`); chèn thẳng `role_permissions` bị trigger chặn; audit |
| TC-CMS-RBAC-02 | Gán UserRole `school_admin` cho tài khoản học sinh (đổi — ADR-0072) | `422`; trigger DB chặn nếu bỏ qua use case |
| TC-CMS-RBAC-03 | Thu hồi UserRole của Academic Manager đang đăng nhập (đổi — ADR-0072) | Trong ≤ 60 s mất quyền tương ứng (Principal cache bị xóa bởi `user_role.changed`), menu cập nhật |
| TC-CMS-RBAC-04 | Sửa role tùy biến (bỏ một permission) | Mọi người có role mất permission trong ≤ 60 s; `role.updated` có `permissions_removed[]` |
| TC-CMS-RBAC-05 | Sửa/xóa role `is_system` | `403 PERMISSION_DENIED` |
| TC-CMS-RBAC-06 | School Admin trường A (`ad.truong.a`) gán UserRole ở trường B (của `ad.truong.b`) cùng tenant | `403 PERMISSION_DENIED` |
| TC-CMS-RBAC-07 | Gán UserRole mã `teacher` hoặc `tenant_moderator`; gán UserRole scope `class` | `422 ROLE_NOT_FOUND` / `422 VALIDATION_FAILED` (scope không hợp lệ) |
| TC-CMS-LOGIN-01 | Học sinh đăng nhập form `/login` của portal bằng username hoặc email (mật khẩu đúng) | Flow direct grant Keycloak từ chối, API `403 PORTAL_ACCESS_STAFF_ONLY` và thông điệp cho học sinh; nếu cấu hình Keycloak sai, mọi API portal trả `403 PORTAL_ACCESS_STAFF_ONLY`, audit (ADR-0091) |
| TC-CMS-LOGIN-02 | Nhân sự 1 tenant / nhiều tenant / chỉ role platform / không còn UserRole | Vào thẳng / chọn đơn vị sau đăng nhập và đổi trên topbar bằng `X-Tenant-Id` (không xin token mới) / phạm vi platform, mã xác thực bắt buộc ở form / `403 PORTAL_ACCESS_STAFF_ONLY` |
| TC-CMS-LOGIN-03 | Đổi đơn vị trên topbar (cùng access token, chỉ đổi `X-Tenant-Id`); gửi `X-Tenant-Id` của tenant không có membership/UserRole portal hoặc vừa bị thu hồi | Đổi đơn vị thành công không xin token mới, menu tải lại theo tenant mới; tenant không hợp lệ → `403 TENANT_MISMATCH` + audit (ADR-0065, ADR-0091 mục 4) |
| TC-CMS-LOGIN-04 | Thêm nhân sự với email đã dùng ở tenant khác (khác hoa thường) | `409 EMAIL_TAKEN`; không tạo tài khoản thứ hai; UI gợi ý gán role cho tài khoản sẵn có nếu cùng người (ADR-0069) |
| TC-CMS-AUTHZ-05 | School Admin đăng nhập portal (đổi — ADR-0072) | Chỉ tải chunk workspace B, C (kiểm tra network log); không có menu workspace A/D/E; mở URL trực tiếp `/portal/game-ops/...` → trang 403 của portal |
| TC-CMS-AUTHZ-09 | Game Designer có token **không** MFA gọi `/api/v1/game-admin/quests` (gateway nội bộ) | 401/403 `MFA_REQUIRED`, audit `authz.denied` |
| TC-CMS-AUTHZ-10 | Token của client khác (`eduworld-game-backend`, token học sinh) gọi `/api/v1/game-admin/*` hoặc `/api/v1/lms/*` | 403 `PORTAL_ACCESS_STAFF_ONLY` (azp không hợp lệ / học sinh), audit |
| TC-CMS-AUTHZ-06 | `GET /me/permissions` cho Academic Manager và School Admin (đổi — ADR-0072) | Không có `game.*`, `competition.config.*`, `platform.*`, `moderation.case.*`, `chat.tenant_terms.write` |
| TC-CMS-AUTHZ-07 | School Admin tạo challenge từ template với tham số ngoài `allowed_params`; Academic Manager tạo challenge (đổi — ADR-0072) | 422 / 403 `PERMISSION_DENIED` |
| TC-CMS-AUTHZ-08 | School Admin tenant A giao bài cho lớp tenant B (đổi — ADR-0072) | 404 (ngoài tenant) |
| TC-CMS-TEN-01 | Tenant Admin gửi `GET /cms/learning/questions?tenant_ids=<B>` | Tham số bị bỏ qua, chỉ trả dữ liệu tenant A |
| TC-CMS-TEN-02 | System Admin list câu hỏi không chọn tenant | Trả dữ liệu mọi tenant, mỗi dòng có `tenant_id`; audit `cross_tenant.accessed` |
| TC-CMS-TEN-03 | System Admin tạo câu hỏi không gửi `tenant_id` và không chọn phạm vi platform | 422 |
| TC-CMS-TEN-04 | Content Admin (platform, không phải System Admin) list dữ liệu tenant | 403 `PERMISSION_DENIED` |
| TC-CMS-DEL-01 | Xóa câu hỏi rồi list | Không xuất hiện; có trong Thùng rác; bài cũ vẫn xem lại được |
| TC-CMS-DEL-02 | Khôi phục câu hỏi khi đã có câu hỏi mới cùng mã | 409 |
| TC-CMS-ASG-01 | Tenant giao quest chưa được gán cho tenant | 403 `RESOURCE_NOT_ASSIGNED` |
| TC-CMS-ASG-02 | Tenant sửa câu hỏi platform được gán | 403 (chỉ đọc) |
| TC-CMS-CLN-01 | Clone lớp mẫu từ tenant A sang B | Bản sao không có học sinh/enrollment; `cloned_from_*` đúng; audit ở A và B |
| TC-CMS-MOD-01 | Tenant Admin gọi `PUT /api/v1/admin/chat/moderation-switches` | Không có route ở gateway public; gọi thẳng backend → 403 `PERMISSION_DENIED` |
| TC-CMS-MOD-02 | Catalog role tenant chứa `chat.policy.toggle` hoặc `chat.sensitive_terms.write` | Tiến trình không start |
| TC-CMS-MOD-03 | Safety Moderator bật/tắt category có `allow_tenant_toggle=false` cho tenant X (đổi — ADR-0072) | 403 `TENANT_TOGGLE_NOT_ALLOWED` |
| TC-CMS-MOD-04 | System Admin tắt bước `pii` của tenant X không nhập lý do | 422 `MODERATION_SWITCH_REASON_REQUIRED` |
| TC-CMS-REL-01 | Tenant Content Editor (không có `content.release.rollback`) gọi `:rollback` (đổi — ADR-0072) | 403 `PERMISSION_DENIED` |
| TC-CMS-REL-02 | Rollback không nhập lý do / tới version `archived` / version của item khác | 422 / `ROLLBACK_TARGET_INVALID` |
| TC-CMS-REL-03 | Publish bản nháp đã bị người khác sửa sau khi mở | 409 `CONTENT_VERSION_CONFLICT` |
| TC-CMS-REL-04 | Mở version cũ đã publish | Hiển thị chỉ đọc, không có nút Sửa; có "Tạo bản nháp từ phiên bản này" |
| TC-CMS-MED-01 | Upload file .exe đổi đuôi .png / ảnh 15 MB | `MEDIA_TYPE_NOT_ALLOWED` / `MEDIA_TOO_LARGE`; asset `rejected` |
| TC-CMS-MED-02 | Submit review khi media còn `processing` | 422 `MEDIA_NOT_READY` |
| TC-CMS-MOD-05 | Tenant X đang tắt `pii` | Banner hiện ở topbar portal của người dùng tenant X và của System Admin khi lọc tenant X; không hiện ở tenant Y |
| TC-CMS-MOD-06 | Tenant Admin gọi API ghi từ bổ sung hoặc bật/tắt nhóm từ của tenant mình (đổi — ADR-0072) | Không có menu ghi; gateway public không có route; gọi thẳng backend → 403 `PERMISSION_DENIED` |
| TC-CMS-ACT-01 | Academic Manager scope trường A (`qlcm.truong.a`) xem lịch sử đăng nhập học sinh trường B cùng tenant (đổi — ADR-0072) | 404 |
| TC-CMS-ACT-02 | Response login events | Không có trường IP / `ip_hash` / `device_id` thô |
| TC-CMS-CLN-02 | TA đổi `inactive_student_cleanup_days` khi platform `allow_tenant_override = false` | 403 `PERMISSION_DENIED` |
| TC-CMS-CLN-03 | TA loại trừ học sinh khỏi đợt dọn | Học sinh không bị dọn ở lần chạy kế tiếp; audit |
| TC-CMS-RET-01 | Đặt `gameplay_chat_retention_days = 2` | 422 `RETENTION_BELOW_MINIMUM` |
| TC-CMS-GAME-01 | School Admin/TA dùng tài khoản portal đăng nhập game (đổi — ADR-0072) | `403 GAME_ACCESS_STUDENT_ONLY`; không có login event |
| TC-CMS-SAF-01 | Safety Moderator (`sm.antoan`) liệt kê case không chọn tenant, mở tin nhắn của một case | Trả case mọi tenant; tin nhắn chỉ trong phạm vi case; audit `moderation_case.viewed`/`moderation_case.messages_viewed`; metric bypass tăng |
| TC-CMS-SAF-02 | Safety Moderator gọi `GET /cms/learning/questions?tenant_ids=…`, `GET /lms/dashboards/school`, `GET /tenant/students/{id}/login-events` | 403 `PERMISSION_DENIED` (ngoài module moderation) |
| TC-CMS-SAF-03 | School Admin gọi `GET /admin/moderation/cases/{id}/messages` hoặc mở thông báo an toàn | Không có route ở gateway public / 403; payload thông báo không có trường nội dung tin nhắn |
| TC-CMS-SAF-04 | School Admin trường A xác nhận thông báo của trường B; xác nhận lần hai | 404 / 409 `CONFLICT`; lần đầu lưu thời điểm + người xác nhận, audit `school_notification.acknowledged` |
| TC-CMS-SAF-05 | Safety Moderator đăng nhập form portal không nhập mã xác thực; token không có MFA (giả lập) gọi route moderation | `401 INVALID_CREDENTIALS` (thông điệp chung, ADR-0091) / `403 MFA_REQUIRED` |
| TC-CMS-LMS-01 | Tạo activity `written_response`/`speaking`/`project` hoặc assignment tham chiếu loại đó | `422` (loại ngoài version này, ADR-0082) |
| TC-CMS-LMS-02 | Academic Manager kích hoạt re-evaluate một version có lỗi; Tenant Content Editor gọi cùng API | Evaluation mới, cũ `superseded`, `attempt.re_evaluated`, audit `content.re_evaluated` / 403 |
| TC-CMS-MENU-01 | Deploy với danh mục mới: `cmd/migrate` đồng bộ `platform.portal_menu_catalog`; `identity_app` thử INSERT/UPDATE bảng danh mục | Bảng khớp file danh mục (`catalog_version`); `identity_app` bị từ chối |
| TC-CMS-MENU-02 | System Admin tắt `menu.school_lms.challenges` cho tenant A | Trong `feature_toggle_cache_ttl`: người dùng tenant A không thấy menu (`disabled_menus`), gọi API của action thuộc menu → `403 FEATURE_DISABLED` (`feature_key`); tenant B không bị ảnh hưởng |
| TC-CMS-MENU-03 | Tắt `menu.platform_admin.feature_toggles` hoặc action của nó; Tenant Admin/Content Admin gọi `putFeatureFlag`; System Admin thiếu lý do / thiếu `If-Match` | `422 FEATURE_NOT_DISABLEABLE` / `403 PERMISSION_DENIED` / `422 VALIDATION_FAILED` / `428 PRECONDITION_REQUIRED`; ghi thành công có audit `feature_flag.updated` + `platform_settings.changed` |
| TC-CMS-MENU-04 | Gọi operation thuộc action đang tắt bằng token game (`eduworld-game-backend`), không token, token portal thiếu permission; System Admin liên tenant không gửi `X-Tenant-Id`; gọi `getMyPermissions` | Token game không bị chặn; `401` trước; `403 FEATURE_DISABLED` (featuregate trước `authz.Require`); System Admin theo mặc định platform; operation `x-portal-shell` không bao giờ bị chặn |
| TC-CMS-MENU-05 | CI: thêm operation có `x-portal-action` không có trong danh mục; danh mục tham chiếu operation không tồn tại hoặc có `x-portal-action` khác | `check-portal-catalog` thất bại, chỉ ra hai chiều lệch |
| TC-CMS-MENU-06 | Tắt menu rồi bật riêng một action của menu đó; bật lại menu | Action vẫn tắt khi menu tắt (`disabled_actions` có action); bật menu → action theo flag riêng |

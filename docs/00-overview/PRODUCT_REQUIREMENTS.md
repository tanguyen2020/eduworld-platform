# Product Requirements Document (PRD) — EduWorld

> Trạng thái: Draft · Phiên bản: 0.3 · Owner: Product Manager · Cập nhật: 2026-11-15
> Liên quan: [PRODUCT_VISION](PRODUCT_VISION.md) · [USER_PERSONAS](USER_PERSONAS.md) · [GLOSSARY](GLOSSARY.md) · [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) · [DOMAIN_MODEL](../06-architecture/DOMAIN_MODEL.md) · [README](../adr/README.md) · [OPEN_QUESTIONS](../12-project/OPEN_QUESTIONS.md) · [ROADMAP](../12-project/ROADMAP.md) · [TEST_STRATEGY](../12-project/TEST_STRATEGY.md)

## 0. Cách đọc tài liệu

- **Nhãn:** `[Confirmed]` = có trong Master Prompt/Party Addendum ([VIBE_CODING_PROMPT](../../VIBE_CODING_PROMPT.md)); `[Proposed]` = đội dự án đề xuất; `[TBD]` = phụ thuộc Open Question.
- **Phase:** `P1`…`P6` theo [ROADMAP](../12-project/ROADMAP.md). P1 Foundation · P2 Core Educational Gameplay · P3 Exam & Learning Management · P4 Multiplayer & Competition · P5 Social & Chat · P6 Production Readiness & Release 1.0. **Release 1.0 = hoàn thành P6 với toàn bộ FR thuộc Master Prompt + Party Addendum** — mọi FR có nhãn phase P1–P6 đều nằm trong Release 1.0; không có FR nào bị cắt thành "sau MVP" [Confirmed — stakeholder 2026-11-15].
- **Mã:** theo [docs/README §3.4](../README.md#34-mã-định-danh). Tài liệu thiết kế chi tiết của từng domain là nơi mở rộng yêu cầu; PRD là danh sách gốc. Thêm yêu cầu mới → thêm mã mới, không tái sử dụng mã đã xóa.
- Một yêu cầu chỉ được đánh dấu "Done" khi có test chứng minh (Master Prompt §20, §24) — không chấp nhận giao diện giả lập.

## 1. Phạm vi

**Trong phạm vi:** game client Godot 4.x 3D, Go backend (microservices theo hệ thống lớn, ADR-0036), Keycloak SSO, APISIX, Valkey, PostgreSQL, tích hợp SIS, Admin Portal (CMS/LMS), teacher dashboard, multiplayer/party/competition, chat an toàn, gamification, observability, triển khai Docker.

**Phạm vi Release 1.0:** game hoàn chỉnh theo Master Prompt + Party Addendum trên Windows, Android, iOS, Web (ADR-0013, OQ-04). **Chỉ làm khi có quyết định** (không thuộc Master Prompt hoặc chỉ yêu cầu thiết kế mở rộng): parent portal (OQ-13), thanh toán (OQ-14), thi chính thức có proctoring (OQ-18), full LRS endpoint. **Không làm trong Release 1.0:** voice chat, xóa vật lý dữ liệu theo luật ([ADR-0070](../adr/ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md)). Bản sắc sản phẩm và KPI: [PRODUCT_VISION](PRODUCT_VISION.md).

### 1.1. Phạm vi Release 1.0 theo domain

| Hạng mục | Phạm vi Release 1.0 | Tài liệu sở hữu |
|---|---|---|
| Nền tảng client | Windows, Android, iOS, Web (Web dùng preset đồ họa thấp) [Proposed, OQ-04] | [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) |
| Đăng nhập & SIS | Keycloak; học sinh đăng nhập **username + password bằng form trong game qua backend** mỗi lần mở game (ADR-0063, ADR-0064, ADR-0068); Admin Portal có màn đăng nhập riêng, chặn học sinh, phân quyền Role/UserRole (ADR-0066), không chọn tenant trước đăng nhập, username/email duy nhất (ADR-0067); header `X-Tenant-Id` bắt buộc (ADR-0065) ( đăng nhập nhanh và IdP trường là mở rộng sau), provisioning từ SIS và tài khoản tenant chung do platform tạo, SIS qua EduWorld SIS Contract hoặc CSV, reconciliation, chuyển lớp/trường, khóa/vô hiệu hóa, dọn học sinh không đăng nhập | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md), [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md), [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) |
| Thế giới & cốt truyện | 8 vùng đất bao trọn 7 khu chức năng bắt buộc; mạch truyện 7 chương; world map, cổng, khám phá, mở khóa vùng; world event | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| Nhân vật | Tạo avatar, skin, outfit, hair, accessories, 6 emote 3D, animation, preset, equipment slot | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| Progression | XP, level, XP curve cấu hình, danh hiệu, phần thưởng lên cấp; tách 5 loại số liệu | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| Quest | Đủ 12 loại (main, side, daily, weekly, learning, exploration, social, party, challenge, seasonal, special event, boss) | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| Challenge & boss | 11 loại challenge, 5 mức độ khó, boss nhiều phase cá nhân/party | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| Kinh tế | Xu + sổ cái, Shop đồ trang trí, collection, achievement, badge; trading tắt | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| Thi đấu | 8 mode, Glicko-2, tier, season, 13 bảng xếp hạng, tournament (single/double elimination, round robin, team, school, class) | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| Học tập | Curriculum đa chương trình/phiên bản; 9 loại activity (quiz, interactive, written/rubric, video, audio, speaking, simulation, mini-game, project); 11 loại câu hỏi; exam đủ hình thức; mastery; công thức MathJax/MathLive; nội dung 4 môn lớp 1–9 theo OQ-08 | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md), [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) |
| Multiplayer | Instance, party (Addendum đầy đủ), lobby, co-op quest/boss, reconnect, đổi thiết bị; matchmaking 8 chế độ | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md), [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md), [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| Social & chat | Bạn bè, club/guild, 8 loại channel, chat luôn có trong trận/party, đủ tính năng nhắn tin, kiểm duyệt + danh mục từ nhạy cảm + công tắc, moderation | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md), [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [MODERATION](../03-social-multiplayer/MODERATION.md) |
| Admin Portal | Một portal 5 workspace: School & LMS, Learning Content Studio, Game Operations Console, Game Live Operations, Platform Administration; Draft/Publish/rollback, gán/clone dữ liệu, thùng rác | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md), [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| Kinh doanh | Không in-app purchase [Proposed, OQ-14] | — |


## 2. Functional Requirements

### 2.1. Identity & Authentication — `AUTH` (Master Prompt §10.1–10.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-AUTH-001 | Học sinh dùng tài khoản do nhà trường cấp; không có màn hình tự đăng ký trong game | [Confirmed] | P1 |
| FR-AUTH-002 | Godot đăng nhập bằng **form username/password trong game**, gửi qua HTTPS tới `POST /api/v1/auth/game/login`; identity-service xác thực với Keycloak bằng client confidential `eduworld-game-backend` (Direct Access Grants); game không có client secret, không mở trình duyệt/WebView (ADR-0068, thay ADR-0007 cho game). Portal giữ Authorization Code + PKCE trên trình duyệt | [Confirmed — người duyệt 2026-11-15] | P1 |
| FR-AUTH-003 | EduWorld không lưu mật khẩu học sinh; xác thực ở Keycloak/IdP. Mật khẩu đi qua game và identity-service (ADR-0068) phải: chỉ qua TLS, không log/trace/telemetry body đăng nhập ở APISIX, backend, client; xóa khỏi bộ nhớ sau khi gọi Keycloak (best effort) | [Confirmed] | P1 |
| FR-AUTH-004 | Backend xác minh token (chữ ký JWKS, `iss`, `aud=eduworld-api`, `exp`) độc lập với APISIX (ADR-0008, ADR-0009) | [Confirmed] | P1 |
| FR-AUTH-005 | Định danh ngoài bằng `(issuer, subject)`; không dùng email/tên hiển thị làm khóa | [Confirmed] | P1 |
| FR-AUTH-006 | Just-in-Time provisioning khi đăng nhập lần đầu qua IdP trường, chỉ khi khớp bản ghi SIS — **ngoài Release 1.0** (ADR-0063: Release 1.0 tài khoản chỉ đến từ SIS provisioning hoặc do platform tạo trong tenant chung) | Mở rộng sau | — |
| FR-AUTH-007 | `POST /api/v1/auth/game/login` (game) / `POST /api/v1/auth/session` (portal) trả bootstrap: `tenant_id` của phiên, profile, school, grade, feature flags, policies áp dụng | [Proposed] | P1 |
| FR-AUTH-008 | **Không tự đăng nhập khi mở lại game** (ADR-0064): mỗi lần mở game nhập username + password vào form trong game (ô mật khẩu luôn trống), không "Ghi nhớ đăng nhập", chỉ được nhớ username (tùy chọn, tắt sẵn trên máy phòng tin học); access/refresh token **chỉ trong RAM**; trong lúc game mở refresh tự động (access token TTL 5 phút) và reconnect không hỏi mật khẩu; phiên game idle 30 phút / max 10 giờ, không offline token; mobile ở nền > 30 phút → đăng nhập lại; CMS idle 30m / max 10h; admin idle 15m + MFA | [Confirmed] chính sách / [Proposed] tham số | P1 |
| FR-AUTH-009 | Logout (và thoát game bình thường, ADR-0064): `POST /api/v1/auth/game/logout` — identity-service thu hồi refresh token + phiên Keycloak; game xóa token trong RAM, đóng WS | [Confirmed] | P1 |
| FR-AUTH-010 | Tài khoản bị khóa/vô hiệu hóa: từ chối request REST, cắt kết nối realtime qua event `user.access_revoked` | [Confirmed] | P1 |
| FR-AUTH-011 | Realtime ticket dùng 1 lần, TTL 60 giây để mở WebSocket `/realtime/v1` (ADR-0008) | [Proposed] | P1 |
| FR-AUTH-012 | Cấu hình realm/client/issuer qua environment, không hardcode | [Confirmed] | P1 |
| FR-AUTH-013 | Keycloak clients: `eduworld-game-backend` (confidential, chỉ Direct Access Grants, secret chỉ ở identity-service, chỉ realm role `student` — ADR-0068; thay client public `eduworld-game`), `eduworld-portal` (Admin Portal, theme `eduworld-ops`, MFA theo role platform — ADR-0027 mục 7, ADR-0066), `eduworld-api`; thêm service account cho worker provisioning | [Confirmed] / service account [Proposed] | P1 |
| FR-AUTH-014 | Hỗ trợ nhiều issuer (multi-realm sau này) nhờ khóa `(issuer, subject)` | [Proposed] | P1 |
| FR-AUTH-015 | Device Authorization Grant cho máy dùng chung trong lớp — **ngoài Release 1.0** (ADR-0063) | Mở rộng sau | — |
| FR-AUTH-016 | Authorization theo **Permission** (trong code) → **Role** (DB, `identity.roles`) → **UserRole** (DB, `identity.user_roles`: scope platform/tenant/school/class/subject, `valid_from/to`, `source` sis/manual), kiểm tra ở mọi use case; role tenant không chứa permission `platform.*`/`game.*`/`competition.config.*`; đổi role/UserRole phát `role.updated`/`user_role.changed` (ADR-0066) | [Confirmed] | P1 |
| FR-AUTH-017 | **Đăng nhập username + password, không chọn trường** (ADR-0063, ADR-0068): form đăng nhập vẽ trong game gồm logo, chọn ngôn ngữ, username, password (nút hiện/ẩn), nút "Đăng nhập", "Nhớ tên đăng nhập", "Quên mật khẩu? Nhờ thầy cô đặt lại"; không có bước chọn trường, ô mã trường hay cách đăng nhập nhanh. Sau xác thực backend xác định tenant theo membership: 1 tenant (SIS hoặc tenant chung) → vào thẳng; nhiều → `403 TENANT_NOT_SELECTED` + `login_ticket` (2 phút, một lần), chọn **sau** đăng nhập bằng `POST /auth/game/tenant:select` (không nhập lại mật khẩu); không thuộc tenant chung và không thuộc tenant SIS nào → `403 ACCOUNT_NOT_PROVISIONED`, không cho đăng nhập, audit, thu hồi phiên Keycloak | [Confirmed — người duyệt 2026-11-15] | P1 |
| FR-AUTH-018 | Backend xác định và kiểm tra chéo tenant khi tạo phiên và ở mọi request theo membership trong DB. **Mọi request tenant-scoped mang header `X-Tenant-Id` bắt buộc** (ADR-0065): thiếu → `400 TENANT_HEADER_REQUIRED` (APISIX + service); header phải khớp tenant của phiên (membership DB, claim nếu có, game session) → không thì `403 TENANT_MISMATCH` + audit; ngoại lệ không cần header: `POST /auth/game/login`, `/auth/game/tenant:select`, `/auth/session`, `GET /me/tenants`, `GET /game/client-config`, health. Có claim → tenant phải thuộc membership (không → `TENANT_MISMATCH` + audit/security event; tenant tắt → `TENANT_ACCESS_DISABLED`); không claim → 1 membership: dùng; nhiều: `TENANT_NOT_SELECTED` kèm danh sách; 0: `ACCOUNT_NOT_PROVISIONED` (ADR-0063) | [Confirmed] | P1 |
| FR-AUTH-019 | Mỗi phiên có đúng một tenant; người thuộc nhiều tenant đổi tenant bằng token mới + header `X-Tenant-Id` mới (không nhập lại mật khẩu); WebSocket giữ tenant cố định từ lúc cấp ticket; username/email không dùng để suy ra tenant. System Admin: ghi dữ liệu tenant gửi `X-Tenant-Id` = tenant đích, đọc liên tenant dùng `tenant_ids` (ADR-0065) | [Confirmed] | P1 |
| FR-AUTH-020 | Trang đăng nhập Keycloak hiển thị tên/logo và IdP của tenant được gợi ý trước đăng nhập — **ngoài Release 1.0** (ADR-0063: không có gợi ý/chọn tenant trước đăng nhập; trang dùng theme `eduworld` chung) | Mở rộng sau | — |
| FR-AUTH-021 | **Tenant chung**: tài khoản học sinh trong tenant chung `common` chỉ do platform (System Admin) tạo/import có chủ đích, kèm School ảo, khối và lớp ảo theo khối (ADR-0060); bootstrap báo `is_common_tenant`. **Không** tự đưa người dùng không khớp tenant nào vào tenant chung (bỏ JIT, ADR-0063) | [Confirmed] (người duyệt) / chi tiết [Proposed] | P1 |
| FR-AUTH-022 | **Chỉ học sinh được đăng nhập game**: đăng nhập qua `/auth/game/login` (client `eduworld-game-backend`) của giáo viên, vận hành tenant, System Admin bị từ chối `403 GAME_ACCESS_STUDENT_ONLY` (backend bắt buộc; Keycloak flow điều kiện role `student` là lớp bổ sung); vận hành xem trải nghiệm qua preview trong CMS (ADR-0031) | [Confirmed] | P1 |
| FR-AUTH-023 | **Một thiết bị cho mỗi tài khoản học sinh**: tối đa 1 game session active; đăng nhập thiết bị khác → hộp thoại xác nhận trên thiết bị mới; đồng ý thì thiết bị cũ tự đăng xuất và hiển thị thông báo; Keycloak session của thiết bị cũ bị thu hồi (ADR-0031) | [Confirmed] | P1 |
| FR-AUTH-024 | **Dọn học sinh không đăng nhập game**: worker soft-delete học sinh có `last_login_at` (hoặc `provisioned_at` nếu chưa từng) + `inactive_student_cleanup_days` < hiện tại; số ngày cấu hình ở platform (mặc định 180 [Proposed]) và tenant; báo trước, loại trừ, khôi phục khi đăng nhập lại, xác nhận khi > 10% (ADR-0032) | [Confirmed] cơ chế / [Proposed] tham số | P2 |
| FR-AUTH-025 | **Ngoài Release 1.0** (ADR-0063) — đăng nhập nhanh không cần mật khẩu: mã QR lớp + PIN 4 số, thẻ QR cá nhân, mật khẩu hình ảnh cho lớp 1–3, SSO IdP trường qua tên miền email, ô "nhập mã trường"; giữ thiết kế ở [KEYCLOAK_SSO §8.1](../09-identity/KEYCLOAK_SSO.md#81-mở-rộng-sau--ngoài-release-10-adr-0063), bật bằng ADR mới | Mở rộng sau | — |
| FR-AUTH-026 | ~~Phiên game dài trên thiết bị (SSO idle 7 ngày / tối đa 30 ngày, refresh token lưu trên thiết bị)~~ — **bỏ khỏi Release 1.0**, thay bằng FR-AUTH-008 (ADR-0064: token chỉ trong RAM, mỗi lần mở game nhập mật khẩu) | Thay bởi FR-AUTH-008 | — |
| FR-AUTH-027 | Hàng chờ đăng nhập: khi vượt ngưỡng toàn cục, `POST /auth/game/login` trả `429 LOGIN_QUEUE_BUSY` + `Retry-After`, game hiển thị "Đang vào lớp…" và tự thử lại có jitter (vẫn thuộc Release 1.0 — ADR-0064 mục 6, ADR-0068 mục 5) | [Proposed] | P4 |
| FR-AUTH-028 | **Bảo vệ đăng nhập game qua backend** (ADR-0068): rate limit 10 lần/phút theo thiết bị và 10 lần/phút theo username (`429 RATE_LIMITED`); Keycloak brute-force khóa tạm sau 5 lần sai (1–15 phút); một thông điệp lỗi chung `401 INVALID_CREDENTIALS` cho sai mật khẩu/không có tài khoản/khóa tạm; chặn client cũ hơn `min_client_version` (`426 CLIENT_VERSION_UNSUPPORTED`); certificate pinning [Proposed] | [Confirmed] / tham số [Proposed] | P1 |
| FR-AUTH-029 | **Admin Portal có màn đăng nhập riêng** (theme vận hành `eduworld-ops`, username hoặc email + mật khẩu, MFA với role platform); **học sinh không đăng nhập được portal**: chặn ở Keycloak (chỉ realm role `tenant_staff`/`platform_staff`) và ở backend (`azp = eduworld-portal` + `user_type ≠ student`, nếu không `403 PORTAL_ACCESS_STAFF_ONLY` + audit) (ADR-0066) | [Confirmed — người duyệt 2026-11-15] | P1 |
| FR-AUTH-030 | **Portal không chọn tenant trước đăng nhập** (ADR-0067): role platform → phạm vi platform; 1 tenant → vào thẳng; nhiều tenant → chọn sau đăng nhập (`GET /me/tenants`), đổi ở bộ chọn trên topbar (token mới + `X-Tenant-Id` mới); không còn UserRole portal hiệu lực và không có role platform → `403 PORTAL_ACCESS_STAFF_ONLY`. Không có URL theo tenant | [Confirmed — người duyệt 2026-11-15] | P1 |
| FR-AUTH-031 | **Username và email không trùng trong cùng tenant; trên SSO một người một tài khoản** (username/email duy nhất trên Keycloak, không phân biệt hoa thường, sau chuẩn hóa trim/NFC/chữ thường) cho học sinh và nhân sự (ADR-0067, [ADR-0069](../adr/ADR-0069-username-email-duy-nhat-theo-tenant-va-dang-nhap-khong-chon-tenant.md)); đăng nhập SSO xác thực trước, identity-service kiểm tra tenant sau; trùng với tài khoản tenant khác mà là cùng một người → thêm membership; nhân sự bắt buộc có email; một người một tài khoản, nhiều membership/UserRole; tạo/sửa trùng → `409 USERNAME_TAKEN`/`409 EMAIL_TAKEN`; SIS/import trùng → không tạo, `reconciliation_issue` `duplicate_identifier`; soft-delete không giải phóng định danh đến khi ẩn danh hóa [Proposed] | [Confirmed — người duyệt 2026-11-15] | P1 |
| FR-AUTH-032 | **Quản lý Role/UserRole trên portal** (ADR-0066): Tenant Admin quản lý role tùy biến [Proposed] và UserRole trong tenant (`tenant.role.write`, `tenant.user_role.write`); School Admin gán UserRole trong trường; System Admin quản lý role platform và mọi tenant (`platform.role.write`, `platform.user_role.write`); role hệ thống chỉ đọc; nhật ký phân quyền | [Confirmed] / role tùy biến [Proposed] | P3 |

### 2.2. Organization & Multi-tenancy — `ORG` (§2.2, §9, §10.3)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-ORG-001 | Phân cấp Platform → Tenant → School → Class. Tenant là **một trường độc lập** (`school`), **một đơn vị nhiều trường** (`unit`) hoặc **tenant chung** (`common`); tenant là đơn vị cô lập dữ liệu (ADR-0005) | [Confirmed] | P1 |
| FR-ORG-002 | Quản lý AcademicYear, Class (grade_level 1–9), Enrollment có lịch sử | [Confirmed] | P1 |
| FR-ORG-003 | Phân biệt Keycloak User, EduWorld User, Student, Teacher, School, Class, Enrollment, External Identity | [Confirmed] | P1 |
| FR-ORG-004 | TeachingAssignment giới hạn giáo viên vào lớp × môn được phân công | [Confirmed] | P1 |
| FR-ORG-005 | Chuyển lớp, chuyển trường: đóng Enrollment cũ, mở mới, giữ lịch sử học tập gắn trường cũ theo policy | [Confirmed] | P1 |
| FR-ORG-006 | Mọi bảng nghiệp vụ có `tenant_id`; **mọi truy vấn phải lọc theo `tenant_id`**, enforcement ở app layer (TenantScope) + RLS; `school_id` là chiều bên trong tenant | [Confirmed] | P1 |
| FR-ORG-007 | School settings: chính sách chat, giờ truy cập, cho phép liên trường, JIT provisioning (`/api/v1/schools`) | [Proposed] | P1–P5 |
| FR-ORG-008 | Tại một thời điểm người dùng chỉ làm việc trên **một** tenant. Chỉ System Admin thấy mọi tenant; mỗi màn hình có dữ liệu theo tenant có **dropdown tìm kiếm tenant** để lọc; mọi truy cập liên tenant được audit (ADR-0027 §6) | [Confirmed] | P1 (API) / P3 (CMS) |
| FR-ORG-009 | Mỗi **Tenant** là một Keycloak Organization (`platform.tenants.keycloak_org_id`, `keycloak_org_alias`, `login_code`); tenant `unit` có một Organization cho mọi trường trong đơn vị (ADR-0026) | [Confirmed] | P1 |
| FR-ORG-010 | Membership Keycloak Organization đồng bộ tự động: học sinh thuộc đúng một tenant; giáo viên thuộc các tenant có phân công; chuyển sang tenant khác gỡ membership cũ và thu hồi phiên cũ; chuyển trường trong cùng tenant `unit` không đổi membership; reconciliation phát hiện member thêm tay | [Confirmed] | P1 |
| FR-ORG-011 | **Chỉ soft-delete**: ứng dụng không hard-delete; "xóa" đặt `deleted_at`/`deleted_by`; mọi truy vấn mặc định không trả dữ liệu đã soft-delete; role DB ứng dụng không có quyền `DELETE`; khôi phục qua Thùng rác (ADR-0028) | [Confirmed] | P1 |
| FR-ORG-012 | System Admin tạo dữ liệu dùng chung (platform) và **gán cho các tenant** muốn sử dụng; tenant được gán chỉ đọc (ADR-0029) | [Confirmed] | P1 (model) / P3 (CMS) |
| FR-ORG-013 | System Admin **clone dữ liệu** (nội dung, cấu hình) từ tenant này sang tenant khác; bản sao có ID mới và nguồn gốc `cloned_from_*`; không bao giờ clone dữ liệu học sinh (ADR-0029) | [Confirmed] | P3 |
| FR-ORG-014 | Nghĩa vụ xóa dữ liệu theo luật được đáp ứng bằng ẩn danh hóa; Release 1.0 không xóa vật lý (ngoại lệ duy nhất: purge chat gameplay ADR-0033) | [Confirmed] ADR-0070 | P5 |
| FR-ORG-015 | **Danh mục quốc gia** (ISO 3166-1) và **danh mục tỉnh/thành** theo quốc gia, có hiệu lực và kế thừa đơn vị cũ → mới; Việt Nam seed 34 đơn vị hiện hành + 63 đơn vị cũ; mã chính thức đối chiếu văn bản trước khi seed production (ADR-0060) | [Confirmed] danh mục, 34 đơn vị; [TBD] mã chính thức | P1 |
| FR-ORG-016 | **Danh mục trường**: mỗi trường thuộc một tenant và một tỉnh/thành, có mã ngành (nếu có), cấp học, khối áp dụng, tên song ngữ; quản trị trong Admin Portal (System Admin liên tenant, Tenant Admin trong tenant), import CSV (ADR-0060) | [Confirmed] | P1 |
| FR-ORG-017 | **Học sinh thuộc đúng một trường và một lớp** tại một thời điểm (một enrollment active); học sinh SIS thiếu lớp không vào game (`NO_ACTIVE_ENROLLMENT`) và tạo issue cho trường; tenant chung dùng lớp ảo theo khối (ADR-0060) | [Confirmed]; lớp ảo [Proposed] | P1 |
| FR-ORG-018 | **Đồng bộ SIS khi đăng nhập**: dữ liệu đủ & mới → không gọi SIS; thiếu trường/lớp → tra cứu SIS chặn ≤ 3 s; dữ liệu cũ hơn 24 giờ → làm mới nền; lan dữ liệu trường/lớp/học sinh sang các service game để hiển thị (ADR-0061) | [Confirmed] yêu cầu; tham số [Proposed] | P1 |

### 2.3. SIS Integration — `SIS` (§10.5)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-SIS-001 | EduWorld sở hữu DTO/API contract SIS ([EduWorld SIS Contract v1](../../specs/sis/eduworld-sis-contract.v1.yaml)); mỗi trường cung cấp API theo contract; không adapter theo DTO của SIS (ADR-0021) | [Confirmed] | P1 |
| FR-SIS-002 | Đồng bộ School, Class, Student, Teacher, Enrollment, Account Status | [Confirmed] | P1 |
| FR-SIS-003 | Hai cách nhận dữ liệu: `contract_v1` (pull HTTP theo contract) và `csv_import` (cột CSV = tên trường DTO) cho trường chưa có API; dữ liệu vi phạm contract bị từ chối theo bản ghi | [Confirmed] hướng / [Proposed] chi tiết | P1 |
| FR-SIS-009 | Bộ conformance test + mock server sinh từ contract để trường tự kiểm tra facade trước khi kết nối | [Proposed] | P1 |
| FR-SIS-004 | Full sync định kỳ + delta sync + webhook (khi trường khai báo capability `delta`/`webhook` trong contract) | [Proposed] | P1–P3 |
| FR-SIS-005 | Reconciliation: phát hiện bản ghi trùng/thiếu/xung đột, tạo ReconciliationIssue cho người duyệt | [Confirmed] | P1 |
| FR-SIS-006 | Sync error handling, retry có backoff, audit mọi lần chạy (SyncRun) | [Confirmed] | P1 |
| FR-SIS-007 | Godot không gọi SIS; SIS credential không lộ qua public gateway (`/api/v1/sis` chỉ gateway nội bộ) | [Confirmed] | P1 |
| FR-SIS-008 | Account deactivation/lock từ SIS lan truyền tới EduWorld trong ≤ 1 chu kỳ sync | [Proposed] | P1 |

### 2.4. Curriculum — `CUR` (§4.1)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CUR-001 | Cấu trúc Curriculum → Grade → Subject → Topic → Learning Objective → Learning Unit → Lesson → Learning Activity → Question/Exercise/Assessment | [Confirmed] | P2 |
| FR-CUR-002 | Hỗ trợ nhiều chương trình và phiên bản chương trình; không hardcode một chương trình | [Confirmed] | P2 |
| FR-CUR-003 | Learning Objective có mã ổn định (vd. `MATH.G3.NUM.MUL_TABLE`) | [Proposed] | P2 |
| FR-CUR-004 | Seed khung GDPT 2018 môn Toán lớp 1–5 cho pilot | [Proposed] OQ-08 | P2 |
| FR-CUR-005 | CMS quản lý cấu trúc chương trình (CRUD + workflow) | [Confirmed] | P3 |
| FR-CUR-006 | API duyệt curriculum cho client: `/api/v1/learning/curriculum` | [Proposed] | P2 |

### 2.5. Learning Content — `CNT` (§4.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CNT-001 | Loại nội dung: Lesson, Learning Material, Video, Image, Audio, Interactive Content, Question, Exercise, Question Set, Assessment, Game Activity | [Confirmed] | P2–P3 |
| FR-CNT-002 | Metadata bắt buộc: grade, subject, topic, LO, difficulty, version, status, author, reviewer, published_at | [Confirmed] | P2 |
| FR-CNT-003 | Trạng thái Draft → Review → Approved → Published → Archived (+ Rejected → Draft) | [Confirmed] / Rejected [Proposed] | P2 |
| FR-CNT-004 | Published version bất biến; attempt tham chiếu `content_version_id` (ADR-0015) | [Confirmed] | P2 |
| FR-CNT-005 | Nội dung thuộc platform (`tenant_id` NULL, tenant thấy khi được gán) hoặc thuộc một tenant; chia sẻ giữa tenant bằng gán/clone của System Admin (ADR-0029) | [Confirmed] | P2 |
| FR-CNT-006 | Media lưu object storage S3-compatible + CDN (không lưu file trong DB); pipeline quét virus, kiểm tra magic bytes, xóa EXIF, chuyển đổi nhiều biến thể; khóa content-addressed bất biến (ADR-0044) | [Confirmed] | P2 |
| FR-CNT-007 | Import nội dung hàng loạt (JSON/CSV theo `specs/schemas`) cho seed pilot trước khi có CMS UI | [Proposed] | P2 |
| FR-CNT-009 | Draft/Publish cho mọi nội dung có version; release pointer; publish ngay hoặc hẹn giờ; rollback = dùng lại version đã publish (đổi con trỏ, lý do bắt buộc); unpublish; lịch sử release (ADR-0043) | [Confirmed] | P2 |
| FR-CNT-010 | Xem nội dung từng version đã publish (chỉ đọc), so sánh hai version, preview trong game | [Confirmed] | P2–P3 |
| FR-CNT-011 | Version pinning: học sinh đang ở màn/quest/bài/attempt/trận giữ version cũ; lượt/màn mới nhận version mới nhất; exam session và bài giao ghim version | [Confirmed] | P2 |
| FR-CNT-012 | Publish đẩy snapshot bất biến lên Valkey/CDN; content manifest theo tenant (ETag/304); câu hỏi đề và answer key không lên CDN; học sinh nhận bản mới ≤ 30 giây sau publish [Proposed] | [Confirmed] | P2 |
| FR-CNT-013 | Activity Registry: mọi loại hoạt động học tập (quiz, interactive_exercise, written_response, video_lesson, audio_lesson, speaking, simulation, minigame, project) đi qua interface plugin chung; quest/assignment/exam/challenge chỉ tham chiếu `activity_id` + version ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), ADR-0048) | [Proposed] | P2 |
| FR-CNT-014 | Video bài học: chapter, phụ đề WebVTT vi/en, resume, câu hỏi chèn tại mốc thời gian, nhánh tương tác, hoàn thành theo khoảng đã xem thật ([VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md)) | [Confirmed] tính năng / [Proposed] tham số | P4 |
| FR-CNT-015 | Bài nghe và nói: listening có câu hỏi; bài nói: học sinh ghi âm trong game, **giáo viên nghe và chấm tay** theo rubric; **không có AI giọng nói** (nhận dạng/chấm phát âm) trong Release 1.0 (ADR-0059); tenant phải bật ([AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md)) | [Confirmed] | P4 |
| FR-CNT-016 | Mô phỏng: template Godot đóng gói, CMS chỉ tham số hóa, server kiểm chứng xác định bằng checker Go ([SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md), ADR-0052) | [Proposed] | P4 |
| FR-CNT-017 | Công thức Toán/Lý/Hóa: soạn bằng MathLive, hiển thị MathJax 4 trên portal, SVG render sẵn trong game; game chỉ hiển thị, không nhập công thức (ADR-0055) | [Confirmed] | P2 |

### 2.6. Question Bank — `QB` (§4.3)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-QB-001 | 11 loại: Single Choice, Multiple Choice, True/False, Fill in the Blank, Short Answer, Numeric Answer, Matching, Ordering, Drag and Drop, Image-based, Interactive | [Confirmed] | P2 (6 loại đầu) – P3 (đủ) |
| FR-QB-002 | Thuộc tính: ID, stem, render data, options, answer key, explanation, difficulty, grade, subject, topic, LO, scoring rule, version | [Confirmed] | P2 |
| FR-QB-003 | Answer key và scoring rule không bao giờ gửi về client (ADR-0016) | [Confirmed] | P2 |
| FR-QB-004 | Random câu hỏi theo điều kiện (grade/topic/LO/difficulty/loại trừ câu đã làm gần đây) | [Confirmed] | P2 |
| FR-QB-005 | Tạo đề từ Question Bank theo Assessment Blueprint | [Confirmed] | P3 |
| FR-QB-006 | Chấm Short Answer bằng so khớp chuẩn hóa (bỏ dấu cách thừa, không phân biệt hoa thường, danh sách đáp án chấp nhận); Numeric có dung sai | [Proposed] | P2 |
| FR-QB-007 | Nội dung câu hỏi hỗ trợ tiếng Việt có dấu, công thức toán đơn giản, hình ảnh, audio (môn Tiếng Anh) | [Proposed] | P2 |

### 2.7. Activity & Attempt Engine — `EXR` (§4.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-EXR-001 | Chế độ: bài cá nhân, theo quest, theo chủ đề, theo lớp, giáo viên giao, tự luyện, làm lại câu sai | [Confirmed] | P2 (giáo viên giao: P3) |
| FR-EXR-002 | Lưu Attempt, Answer, Correctness, Score, Duration, Submission Time, Content Version, LO | [Confirmed] | P2 |
| FR-EXR-003 | Practice: phản hồi đúng/sai + giải thích ngay sau từng câu | [Proposed] | P2 |
| FR-EXR-004 | Resume attempt chưa hoàn thành theo chính sách (TTL, số lần) | [Confirmed] / tham số [Proposed] | P2 |
| FR-EXR-005 | Submit idempotent (`Idempotency-Key`), một attempt chỉ submit một lần | [Confirmed] | P2 |
| FR-EXR-006 | Thời gian tính theo server timestamp; client elapsed chỉ tham khảo | [Confirmed] | P2 |
| FR-EXR-007 | Một họ API attempt `/api/v1/learning/activity-attempts` cho mọi loại hoạt động (start, responses, progress, submit, resume, review) — ADR-0059 | [Confirmed] | P2 |
| FR-EXR-008 | Evaluation tách khỏi Attempt: nhiều evaluation/attempt (system, teacher, AI gợi ý), đúng một evaluation có hiệu lực; chấm lại tạo evaluation mới, không sửa cũ ([ACTIVITY_ENGINE §7](../02-learning/ACTIVITY_ENGINE.md#7-evaluation)) | [Proposed] | P3 |
| FR-EXR-009 | Tự luận/trả lời dài/lời giải toán (văn bản + đáp số) chấm bằng rubric và hàng đợi chấm của giáo viên; trả bài, chấm lại, nhận xét ([WRITTEN_RESPONSE_RUBRIC](../02-learning/WRITTEN_RESPONSE_RUBRIC.md)) | [Confirmed] | P3 |
| FR-EXR-010 | Dự án nhiều bước, cá nhân/nhóm; mỗi học sinh có attempt và learning record riêng | [Confirmed] | P4 |
| FR-EXR-011 | AI hỗ trợ chấm chỉ là gợi ý, tắt mặc định, giáo viên phải duyệt (ADR-0053) | [Proposed] | Sau 1.0 |

### 2.8. Exam & Practice — `EXM` (§4.5)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-EXM-001 | Hình thức: Practice Test, Mock Exam, Chapter Test, Semester Test, Subject Test, Custom Assessment, Timed Exam | [Confirmed] | P3 |
| FR-EXM-002 | Tạo đề, random đề, random câu, xáo trộn đáp án, cấu hình thời gian & thang điểm | [Confirmed] | P3 |
| FR-EXM-003 | Nộp bài, chấm điểm, hiển thị kết quả theo `result_visibility`, phân tích câu sai, đề xuất nội dung ôn tập | [Confirmed] | P3 |
| FR-EXM-004 | Phân biệt luyện tập và kiểm tra chính thức (`is_official`) | [Confirmed] | P3 |
| FR-EXM-005 | Chống nộp trùng, chống sửa điểm từ client, tự nộp khi hết `deadline_at` | [Confirmed] | P3 |
| FR-EXM-006 | Kết quả thi lưu bền vững trong PostgreSQL, không phụ thuộc Valkey | [Confirmed] | P3 |
| FR-EXM-007 | Thi chính thức có giá trị pháp lý/proctoring | [TBD] OQ-18 | Chỉ khi có quyết định |

### 2.9. Learning Progress & Mastery — `PRG` (§4.6)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-PRG-001 | Theo dõi tiến độ, số bài hoàn thành, tỷ lệ chính xác, thời gian học, số lần làm, LO mastery, kỹ năng yếu, tiến bộ theo thời gian | [Confirmed] | P2–P3 |
| FR-PRG-002 | Mastery theo `(student, LO)` tính từ nhiều bằng chứng, không đồng nhất với điểm một lần làm bài | [Confirmed] | P2 |
| FR-PRG-003 | Mô hình mastery cấu hình được, mặc định Weighted Evidence (ADR-0017), lưu `model_version` | [Confirmed] cấu hình / [Proposed] thuật toán | P2 |
| FR-PRG-004 | Gợi ý ôn tập dựa trên LO yếu (`/api/v1/learning/recommendations`) | [Confirmed] | P3 |
| FR-PRG-005 | Học sinh xem tiến độ dạng thân thiện theo độ tuổi (sao/huy hiệu cho lớp 1–5, biểu đồ cho 6–9) | [Proposed] | P3 |

### 2.10. Learning Events & Analytics — `ANL` (§16)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-ANL-001 | Ghi learning events (activity/question/exercise/exam/lesson/quest/learning_objective.*) với event_id, student_id, school_id, session_id, activity_id, content_id, content_version, type, timestamp, payload, idempotency key | [Confirmed] | P2 |
| FR-ANL-002 | Lưu bền vững trong `learning.learning_events` (LRS nội bộ), tương thích tư duy xAPI | [Confirmed] / thiết kế [Proposed] | P2 |
| FR-ANL-003 | Analytics theo học sinh, giáo viên, lớp, trường, môn, chủ đề, LO | [Confirmed] | P3 |
| FR-ANL-004 | Không dùng dữ liệu hành vi ngoài mục đích đã công bố | [Confirmed] | P1+ |
| FR-ANL-005 | Aggregation chạy trong worker, dashboard đọc từ read model | [Proposed] | P3 |
| FR-ANL-006 | **Game Analytics** tách khỏi Learning Analytics và Monitoring: chỉ số engagement, retention, funnel, tiến trình, kinh tế, xã hội, chơi lành mạnh, hiệu năng theo hạng thiết bị — định nghĩa ở [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) | [Confirmed] tách / chỉ số [Proposed] | P4 |
| FR-ANL-007 | Nguồn: domain event NATS + telemetry client tối thiểu không PII; lưu schema `analytics` của game-service (ADR-0058) | [Proposed] | P4 |
| FR-ANL-008 | Định danh giả danh `player_key`; không lưu tên, nickname, `student_id`, nội dung chat trong analytics | [Proposed] | P4 |
| FR-ANL-009 | Dashboard Game Analytics trong workspace Game Live Operations (platform); báo cáo tổng hợp cho Tenant/School Admin, ẩn ô < 5 học sinh | [Proposed] | P4 |
| FR-ANL-010 | Không dùng thời gian chơi làm thước đo thành công; có chỉ số chơi lành mạnh (vượt mốc nhắc nghỉ, chơi khuya) | [Confirmed] | P4 |
| FR-ANL-011 | Tenant tắt được telemetry sản phẩm; telemetry ổn định (hiệu năng, crash) vẫn thu | [Proposed, OQ-12] | P4 |
| FR-ANL-012 | Có trang dashboard cho giáo viên/lớp, trường, tenant, platform và Game Analytics; số liệu theo ngày/tuần/tháng | [Confirmed — người duyệt] | P3–P4 |
| FR-ANL-013 | Dashboard chỉ đọc bảng tổng hợp dựng gia tăng; khoảng thời gian tối đa 2 năm, độ hạt tự chọn theo khoảng; hiển thị thời điểm cập nhật và nhãn tạm tính ([DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md)) | [Confirmed — ADR-0062] | P3 |
| FR-ANL-014 | Xuất báo cáo dashboard chạy bất đồng bộ, link tải có hạn, có audit | [Confirmed — ADR-0062] | P3 |

### 2.11. Game World & Character — `GAME` (§2.3, §3.1–3.3, §13)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-GAME-001 | Thế giới 3D stylized/cartoon/low-poly, thân thiện, không bạo lực | [Confirmed] | P2 |
| FR-GAME-002 | Thế giới gồm 8 vùng đất có cốt truyện, bao trọn 7 khu chức năng bắt buộc (Main Hub, Learning Zone theo môn, Practice Zone, Exam Zone, Competition Arena, Social Zone, Quest Zone) — bảng ánh xạ ở [WORLD_DESIGN §3](../01-game-design/WORLD_DESIGN.md#3-bản-đồ-vùng-đất) | [Confirmed] khu chức năng · [Proposed] tên vùng | P2 (Hub, vùng môn học, Practice, Quest) – P3 (Exam) – P4 (Arena) – P5 (Social) |
| FR-GAME-003 | Character controller, camera controller, animation state machine, NPC interaction | [Confirmed] | P2 |
| FR-GAME-004 | Avatar customization: skin, outfit, hair, accessories, emotes; character level; inventory | [Confirmed] | P2 |
| FR-GAME-015 | Luồng tạo avatar lần đầu (preset, dáng người, màu da/tóc, trang phục mặc định, nickname), equipment slot, avatar preset, 6 emote 3D mặc định (vẫy tay, vỗ tay, nhảy, ăn mừng, cảm ơn, xin lỗi) — [CHARACTER_SYSTEM §5.0, §7](../01-game-design/CHARACTER_SYSTEM.md) | [Confirmed] tính năng (Master Prompt mới §5.2, §13.5) / [Proposed] chi tiết | P2 |
| FR-GAME-016 | Mạch truyện "Thắp lại Tia Sáng" gồm 7 chương gắn vùng; chương môn học mở song song, không khóa học tập theo truyện ([WORLD_DESIGN §2.2](../01-game-design/WORLD_DESIGN.md#22-mạch-truyện-chính-confirmed--người-duyệt-2026-11-15-adr-0056)) | [Proposed] | P2–P4 |
| FR-GAME-017 | World map, cổng di chuyển, khám phá (discovery point, collectible, % hoàn thành bản đồ), mở khóa vùng theo level/quest/LO, không theo thanh toán ([WORLD_DESIGN §6–§8](../01-game-design/WORLD_DESIGN.md)) | [Proposed] | P2–P3 |
| FR-GAME-018 | Học tập lồng trong ngữ cảnh game: mỗi khối học tập ngắn rồi xen tương tác thế giới; không biến game thành chuỗi quiz ([GAME_DESIGN](../01-game-design/GAME_DESIGN.md), [LEARNING_EXPERIENCE §2.1](../02-learning/LEARNING_EXPERIENCE.md)) | [Confirmed] nguyên tắc · [Proposed] ngưỡng | P2 |
| FR-GAME-005 | Nickname sinh tự động/chọn từ danh sách duyệt; tên thật không hiển thị cho người chơi khác | [Proposed] | P2 |
| FR-GAME-006 | Game client data-driven: quest, mini-game, NPC dialogue, reward, cấu hình khu vực tải từ backend; cập nhật nội dung không cần build lại game | [Confirmed] | P2 |
| FR-GAME-007 | Game session server-side (`/api/v1/game/sessions`), ghi platform, client version, end reason | [Proposed] | P1 |
| FR-GAME-008 | Kiểm tra phiên bản client tối thiểu; buộc cập nhật khi không tương thích API | [Proposed] | P1 |
| FR-GAME-009 | Chạy được trên máy tầm trung: renderer Mobile/Compatibility, preset Thấp/Trung bình/Cao tự phát hiện, độ phân giải động, LOD, occlusion, instancing, lightmap bake, nén texture, streaming vùng ([PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md), ADR-0057) | [Confirmed] | P2–P6 |
| FR-GAME-010 | UI theo nhóm tuổi: lớp 1–5 trực quan/ít thao tác/chữ to + hỗ trợ đọc âm thanh; lớp 6–9 nhiều thông tin hơn | [Confirmed] / audio [Proposed] | P2 |
| FR-GAME-011 | Telemetry client (crash, FPS, load time) không chứa PII, rate limit (`/api/v1/game/telemetry`) | [Proposed] | P2 |
| FR-GAME-012 | Nhắc nghỉ sau thời gian chơi liên tục (mặc định 45 phút), giờ truy cập do trường cấu hình | [Proposed] OQ-17 | P2 |
| FR-GAME-013 | **Lưu thời gian đăng nhập game**: mỗi lần vào game ghi `game.login_events` (thời điểm vào/ra theo giờ server, nền tảng, thiết bị rút gọn, lý do kết thúc; `ip_hash`, không IP thô) và cập nhật `users.last_login_at`; vận hành tenant/giáo viên xem lịch sử và "số ngày chưa vào game" (ADR-0031) | [Confirmed] | P1 |
| FR-GAME-014 | Game có nút "Đăng xuất"; đóng game kết thúc phiên; thiết bị phòng máy dùng chung luôn xóa token khi đóng; phiên không heartbeat > 45 s tự kết thúc | [Proposed] | P1 |

### 2.12. Quest — `QST` (§3.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-QST-001 | Quest có: ID, tên, mô tả, loại, lớp, môn, chủ đề, LO, điều kiện bắt đầu, các bước, điều kiện hoàn thành, phần thưởng, thời gian, phiên bản | [Confirmed] | P2 |
| FR-QST-002 | 12 loại quest: Main, Side, Daily, Weekly, Learning, Exploration, Social, Party, Challenge, Seasonal, Special Event, Boss (quest schema v2; thay 7 loại trước đây) | [Confirmed] (Master Prompt mới §7) | P2 (Party/Challenge/Boss: P3–P4) |
| FR-QST-008 | Quest hỗ trợ narrative, NPC, level requirement, learning requirement, repeatability, cooldown, failure conditions; thất bại mặc định không mất tiến độ | [Confirmed] / chi tiết [Proposed] | P2 |
| FR-QST-003 | Quest liên kết nội dung học tập; bước `activity` tạo Attempt trong Activity & Attempt Engine | [Confirmed] | P2 |
| FR-QST-004 | Bước: talk_npc, go_to, activity, collect, minigame, party_activity | [Proposed] | P2 |
| FR-QST-005 | Tiến độ quest lưu server, hoàn thành bước qua domain event (ADR-0019) | [Proposed] | P2 |
| FR-QST-006 | Quest lọc theo grade_level của học sinh; không giao nội dung sai khối | [Confirmed] | P2 |
| FR-QST-007 | Shared Quest Progress theo cấu hình `shared_progress_policy`; kết quả học tập vẫn riêng từng học sinh | [Confirmed] | P4 |

### 2.13. Mini-game Engine — `MG` (§3.5)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-MG-001 | Loại: Multiple Choice, True/False, Fill in the Blank, Drag and Drop, Matching, Ordering, Calculation, Puzzle, Word Game, Science Simulation, Timed Challenge | [Confirmed] | P2 (6 loại) – sau |
| FR-MG-002 | Mini-game không có ngân hàng câu hỏi riêng; lấy từ Learning Engine qua QuestionSet/selection rule | [Confirmed] | P2 |
| FR-MG-003 | Cấu hình: độ khó, thời gian, số câu, điểm, cách tính kết quả, phản hồi đúng/sai, cơ chế hoàn thành, reward config | [Confirmed] | P2 |
| FR-MG-004 | Mỗi mini-game là scene Godot tái sử dụng, nhận `MiniGameConfig` + render payload | [Proposed] | P2 |

### 2.14. Multiplayer & Presence — `MP` (§5.1, §14)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-MP-001 | Hỗ trợ Multiplayer World, Room, Lobby, Team, Match, Session | [Confirmed] | P4 |
| FR-MP-002 | Hub instance tối đa 40 người, cấp theo school/grade band; bạn bè/party ưu tiên cùng instance (ADR-0012) | [Proposed] OQ-05 | P4 |
| FR-MP-003 | Presence/online status (Valkey), chỉ hiển thị cho bạn bè/lớp theo policy | [Confirmed] / hiển thị [Proposed] | P4 |
| FR-MP-004 | Đồng bộ vị trí qua WebSocket ≤10 Hz, interpolation, server kiểm tra tốc độ/biên (ADR-0011) | [Proposed] OQ-06 | P4 |
| FR-MP-005 | Heartbeat, timeout, reconnect, session recovery, disconnect handling | [Confirmed] | P4 |
| FR-MP-006 | Không gửi chuyển động 3D qua REST; Valkey không làm game simulation server | [Confirmed] | P4 |

### 2.15. Party — `PTY` (Party Addendum)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-PTY-001 | Create, Join, Leave, Invite Friend, Accept, Reject, Kick, Transfer Leader, Disband, Settings, Member Status, Party Chat | [Confirmed] | P4 (Party Chat văn bản tự do: P5) |
| FR-PTY-002 | Thuộc tính: Party ID, name, leader, members, max members, type, visibility, invitation policy, current activity, current game session, timestamps | [Confirmed] | P4 |
| FR-PTY-003 | Party types: Friend Party, Study Group, Quest Party, Competition Team, Class Group, Custom Group | [Confirmed] | P4 |
| FR-PTY-004 | Max members cấu hình được; mặc định 4, cho phép 2–8 | [Confirmed] cấu hình / [Proposed] giá trị | P4 |
| FR-PTY-005 | Một học sinh chỉ có một active Party tại một thời điểm (Addendum §6 ghi cho MVP; giữ nguyên cho Release 1.0); chống duplicate membership (unique constraint) | [Confirmed] | P4 |
| FR-PTY-006 | Party Lobby: danh sách thành viên, avatar, online, Ready/Not Ready, leader controls, chọn activity & game mode, mời, group chat, Start Session | [Confirmed] | P4 |
| FR-PTY-007 | Start Session: xác nhận leader → kiểm tra thành viên → kiểm tra điều kiện lớp/quyền → tạo Game Session/room → gán thành viên → gửi session info tới clients → đồng bộ trạng thái → theo dõi reconnect/disconnect → kết thúc và lưu kết quả | [Confirmed] | P4 |
| FR-PTY-008 | Leader disconnect: grace period 60 giây, sau đó tự chuyển leader cho thành viên tham gia sớm nhất còn online | [Confirmed] cơ chế / [Proposed] tham số | P4 |
| FR-PTY-009 | Party private cần lời mời/quyền; lời mời hết hạn sau 5 phút; không mời người đã block/bị block | [Confirmed] / tham số [Proposed] | P4 |
| FR-PTY-010 | Giới hạn theo school/class/grade/activity; mặc định chỉ cùng trường | [Confirmed] cấu hình / [Proposed] mặc định | P4 |
| FR-PTY-011 | Chống spam lời mời (rate limit), audit log cho create/kick/transfer/disband | [Confirmed] | P4 |
| FR-PTY-012 | Thành viên mất kết nối: các thành viên khác tiếp tục nếu activity cho phép (`continue_on_disconnect`) | [Confirmed] | P4 |
| FR-PTY-013 | Không chia sẻ kết quả học tập cá nhân giữa thành viên; mỗi người có attempt riêng | [Confirmed] | P4 |
| FR-PTY-014 | API `/api/v1/multiplayer/parties`, `/api/v1/multiplayer/invitations`; realtime event qua `/realtime/v1` | [Proposed] | P4 |

### 2.16. Matchmaking — `MM` (§5.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-MM-001 | Chế độ: Random, Skill-based, Grade-based, Subject-based, Topic-based, Friend, Team, Tournament | [Confirmed] | P4 |
| FR-MM-002 | Ràng buộc cứng (khối lớp, độ phù hợp nội dung, policy trường) kiểm tra **trước** rating; không ghép lớp 1 với lớp 9 | [Confirmed] | P4 |
| FR-MM-003 | Mặc định: lớp 1–5 chỉ ghép cùng khối (`grade_level`); lớp 6–9 theo `grade_band` (±1 khối khi chờ lâu); **chỉ cùng trường**. Ghép liên trường trong cùng tenant `unit` chỉ khi tenant bật; liên tenant chỉ ở sự kiện platform | [Proposed] | P4 |
| FR-MM-004 | Vòng đời: Queue → Ticket → Match Proposal → Lobby → Ready Check → Team Assignment → Match Start → Match End → Result; hỗ trợ Reconnect | [Confirmed] | P4 |
| FR-MM-005 | Cửa sổ rating mở rộng theo thời gian chờ; timeout ghép trận 90 giây đề xuất chế độ khác/bot luyện tập | [Proposed] | P4 |
| FR-MM-006 | Ticket chỉ trong Valkey; mất → client tạo lại (ADR-0004) | [Proposed] | P4 |

### 2.17. Competition / Battle — `CMP` (§5.3, §5.6)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CMP-001 | Chế độ: 1v1 Knowledge Battle, Team Battle, Co-op Challenge, Timed Quiz, Subject Tournament, Class/School/Seasonal Competition | [Confirmed] | P4 (Seasonal: P6+) |
| FR-CMP-002 | Battle là state machine server-authoritative: question plan do server chọn, chấm và tính điểm thời gian server-side | [Confirmed] | P4 |
| FR-CMP-003 | Kết quả trận ghi một lần (unique `match_id`), idempotent, audit | [Confirmed] | P4 |
| FR-CMP-004 | Câu trả lời trong trận ghi learning evidence với context `competition` (trọng số thấp cho mastery) | [Proposed] | P4 |
| FR-CMP-005 | Xử lý disconnect: grace 30 giây; quá hạn → xử thua phần còn lại/forfeit theo mode | [Proposed] | P4 |

### 2.18. Ranking — `RNK` (§5.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-RNK-001 | Hệ thống ranking độc lập; Glicko-2 theo `(student, mode, subject, season)` (ADR-0018) | [Confirmed] độc lập / [Proposed] Glicko-2 | P4 |
| FR-RNK-002 | Không dùng rating để đánh giá năng lực học tập; tách Learning Score, Mastery, Game Rating, Game XP, Competition Points | [Confirmed] | P4 |
| FR-RNK-003 | Hiển thị rating cho học sinh dưới dạng tier thân thiện, không số chính xác cho lớp 1–5 | [Proposed] | P4 |
| FR-RNK-004 | Season soft-reset rating | [Proposed] | P4 |

### 2.19. Leaderboard — `LDB` (§5.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-LDB-001 | Bảng: Global, School, Class, Grade, Subject, Topic, Game Mode, Friends, Team, Daily, Weekly, Monthly, Season | [Confirmed] | P4 (Global trong Release 1.0; quy tắc hiển thị nickname theo FR-LDB-003) |
| FR-LDB-002 | Leaderboard cache bằng Valkey sorted set, nguồn từ PostgreSQL | [Confirmed] | P4 |
| FR-LDB-003 | Leaderboard ngoài phạm vi trường chỉ hiển thị nickname, không tên trường/lớp chi tiết | [Proposed] | P4 |
| FR-LDB-004 | Trường có thể tắt leaderboard công khai cho lớp 1–3 | [Proposed] | P4 |

### 2.20. Tournament — `TRN` (§5.5)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-TRN-001 | Format: Single Elimination, Double Elimination, Round Robin, Team, School, Class Tournament | [Confirmed] | P4 (Single elim, Round robin) – P6 (còn lại) |
| FR-TRN-002 | Tạo giải, đăng ký, chia bảng, tạo bracket, lên lịch, xác nhận người tham gia, quản lý trận, xác nhận kết quả, trao thưởng, lưu lịch sử | [Confirmed] | P4 |
| FR-TRN-003 | Bracket lưu PostgreSQL, không chỉ Valkey | [Confirmed] | P4 |

### 2.21. Social — `SOC` (§6.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-SOC-001 | Friend list, friend request, invitation | [Confirmed] | P5 (cơ bản cho mời party: P4) |
| FR-SOC-002 | Mặc định chỉ kết bạn cùng trường (như nhau cho mọi lớp); tenant `unit` có thể mở liên trường trong tenant | [Proposed] | P5 |
| FR-SOC-003 | Block / Mute người dùng; block chặn chat, mời, ghép party | [Confirmed] | P5 (Block: P4 cùng party) |
| FR-SOC-004 | Team, Study Group do giáo viên hoặc học sinh tạo theo policy | [Confirmed] | P5 |

### 2.22. Chat — `CHAT` (§6.1–6.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CHAT-001 | Channel: World, Area, **Match**, Party, Team, Class, Study Group, Direct Message; cấu hình bằng policy | [Confirmed] | P4 (Match/Party/Team theo trận), P5 (còn lại) |
| FR-CHAT-002 | Không có rule = deny; quyền vào channel theo quan hệ (cùng trường/instance/nhóm/lớp/bạn bè) và policy, như nhau cho mọi lớp (ADR-0020) | [Confirmed] | P5 |
| FR-CHAT-003 | Text, emoji, reactions, quick messages, group chat, history, unread count, notifications | [Confirmed] | P4 (channel gameplay), P5 (còn lại) |
| FR-CHAT-004 | **Một bộ quy tắc chat cho mọi lớp 1–9** (không điều kiện theo `grade_level`/`grade_band`): đủ 7 channel; World = cùng trường (tenant `unit` có thể mở rộng toàn tenant); DM chỉ giữa bạn bè không block nhau; mọi học sinh dùng text, emoji, quick message, reaction (ADR-0020) | [Confirmed] (giá trị số [Proposed]) | P5 |
| FR-CHAT-005 | **Mọi tin nhắn phải hợp lệ**: pipeline đồng bộ phía server cho mọi `kind` — normalize → định dạng/độ dài → profanity (tiếng Việt có/không dấu, teencode, leetspeak, tiếng Anh) → PII → spam/flood → bắt nạt → allow/mask/block/hold_for_review; `block` không lưu nội dung | [Confirmed] chức năng / [Proposed] chi tiết pipeline | P5 |
| FR-CHAT-006 | Lịch sử chat theo quyền và retention (mặc định 180 ngày) | [Confirmed] / [TBD] OQ-12 | P5 |
| FR-CHAT-007 | Không có voice chat trong Release 1.0; điểm mở rộng chỉ ở mức thiết kế, không hiện thực (ADR-0051, ADR-0070) | [Confirmed] | — |
| FR-CHAT-008 | Giao tin nhắn realtime qua `/realtime/v1`, gửi qua REST hoặc WS có idempotency client message id | [Proposed] | P4 |
| FR-CHAT-009 | **Chat gameplay luôn có** (ADR-0020 mục 6): vào trận 1v1 → channel `match`; trận đội → `team` mỗi đội + `match` chung; tạo party → `party`. Tự tạo, tự join, đủ text/emoji/quick/reaction; không tắt được bằng cài đặt tenant/trường/lớp, không áp khung giờ; chỉ mute/block cá nhân hoặc sanction ngăn một người gửi | [Confirmed] | P4 |
| FR-CHAT-010 | UI chat trong lobby/trận/party: panel thu gọn được nhưng không tắt; quick message một chạm; tab Đội/Cả trận; menu Tắt tiếng/Chặn/Báo cáo trên từng tin; báo cáo từ màn hình kết quả trận | [Confirmed] tính năng / [Proposed] UX | P4 |
| FR-CHAT-011 | **Purge chat gameplay** (ADR-0033): channel `match`, `team` theo trận, `party` (sau giải tán) bị xóa vật lý sau `gameplay_chat_retention_days` (mặc định 7, tối thiểu 3) từ `closed_at`; World/Area sau 24 h; giữ bằng chứng của report/case/sanction trước khi purge; hoãn khi case mở; không áp dụng Class/Study Group/Team cố định/DM; người chơi được thông báo thời hạn lưu | [Confirmed] / [Proposed] số ngày | P4 |
| FR-CHAT-012 | Sửa tin trong 15 phút (chạy lại kiểm duyệt, lưu revision) và thu hồi trong 24 giờ (placeholder, giữ bằng chứng, vẫn báo cáo được) — ADR-0034 | [Confirmed] tính năng / [Proposed] thời hạn | P4 |
| FR-CHAT-013 | Reply, Quote (trích từ bản đã che), Forward (cùng tenant, ≤ 5 đích, chạy lại kiểm duyệt) — ADR-0034 | [Confirmed] / [Proposed] giới hạn | P4 |
| FR-CHAT-014 | Emoji, Sticker (pack platform, có thể mở khóa bằng phần thưởng), GIF (thư viện nội bộ đã duyệt, không dịch vụ bên thứ ba), Reactions (≤ 3/người/tin) — ADR-0034 | [Confirmed] / [Proposed] nguồn GIF nội bộ | P4 |
| FR-CHAT-015 | Mention `@nickname` (≤ 5/tin, thông báo + đếm riêng), Pin (≤ 3/channel), Copy (bản đã che), Typing indicator — ADR-0034 | [Confirmed] / [Proposed] giới hạn | P4 |
| FR-CHAT-016 | Trạng thái Sent/Delivered/Read theo con trỏ, Unread count + mention count, Lịch sử phân trang và tìm kiếm full-text tiếng Việt có/không dấu — ADR-0034 | [Confirmed] | P4 (tìm kiếm P5) |
| FR-CHAT-017 | Truyền tải chat qua NATS JetStream (bền vững, trước khi xác nhận Sent) + Valkey Pub/Sub (fan-out realtime), lưu PostgreSQL bởi `chat-persister` — ADR-0035 | [Confirmed] | P4 |

### 2.23. Moderation & Child Safety — `MOD` (§6.3, §17.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-MOD-001 | Content/profanity filtering, anti-spam, anti-bullying, PII detection | [Confirmed] | P5 |
| FR-MOD-002 | Report message, report user, block, mute | [Confirmed] | P5 |
| FR-MOD-003 | Moderation queue, xử lý theo phạm vi trường (teacher/school admin), audit mọi hành động | [Confirmed] | P5 |
| FR-MOD-004 | Chat policy theo platform → tenant → school → class (không theo grade — thay mục "Age/Grade-based Policy" của Master Prompt §6.3 theo ADR-0020); cấp dưới chỉ siết chặt hơn cấp trên | [Confirmed] | P5 |
| FR-MOD-005 | Không ai có quyền đọc toàn bộ chat của mọi trường; truy cập chat đều audit | [Confirmed] | P5 |
| FR-MOD-006 | Sanction: warn, mute có thời hạn, restrict (chỉ quick message), escalate | [Proposed] | P5 |
| FR-MOD-007 | Teacher controls: mute lớp, khóa channel lớp, xem report lớp mình | [Confirmed] | P5 |
| FR-MOD-008 | Không giám sát chat vượt quá mục đích an toàn cần thiết | [Confirmed] | P5 |
| FR-MOD-009 | Danh mục từ nhạy cảm quản lý trong CMS: category (severity, action mặc định), term (match type, biến thể tự sinh không dấu/teencode/leetspeak), allowlist, version; không hardcode (ADR-0030) | [Confirmed] | P5 |
| FR-MOD-010 | Action theo mức: `mask` / `block` / `hold_for_review` / `block_and_alert`; phản hồi không lộ từ đã khớp | [Confirmed] cơ chế / [Proposed] ánh xạ severity → action | P5 |
| FR-MOD-011 | Công tắc kiểm duyệt bật/tắt toàn bộ hoặc từng bước (`sensitive_terms`, `pii`, `spam`, `bullying`, `async_analysis`) theo platform/tenant; mặc định bật; chỉ System Admin; lý do bắt buộc, audit, banner CMS, alert | [Confirmed] ADR-0030 | P5 |
| FR-MOD-012 | Khi tắt vẫn giữ độ dài tối đa, rate limit, quyền channel, sanction, block/mute/report, moderation thủ công | [Confirmed] ADR-0030 | P5 |
| FR-MOD-013 | Tenant thêm từ bổ sung (chỉ siết chặt) và bật/tắt category khi `allow_tenant_toggle` | [Proposed] | P5 |
| FR-MOD-014 | Công tắc có thời hạn `expires_at` tự bật lại | [Proposed] | P5 |
| FR-MOD-015 | Publish danh mục chạy test hồi quy (recall ≥ 95%, false positive ≤ 3%) và reload không downtime ≤ 60 s | [Proposed] | P5 |

### 2.24. Notification — `NTF`

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-NTF-001 | Thông báo trong game: lời mời party/kết bạn, bài được giao, kết quả, phần thưởng, thông báo trường | [Confirmed] | P2 (in-game) – P5 (realtime đầy đủ) |
| FR-NTF-002 | Realtime qua WebSocket; lưu bền vững để xem lại (`/api/v1/notifications`) | [Proposed] | P5 |
| FR-NTF-003 | Không gửi push notification ngoài game tới học sinh trong Release 1.0; thông báo chỉ hiển thị trong game (inbox, HUD). Mở push sau này cần ADR mới và chính sách giờ được phép (OQ-17) | [Proposed] | P5 |

### 2.25. Gamification & Reward — `GAM`, `RWD` (§3.3, §7)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-GAM-001 | XP, Level, Achievement, Badge, Daily/Weekly Challenge, Season Reward, Collection | [Confirmed] | P2 (XP, Level, Badge) – P4 (Season) |
| FR-GAM-002 | XP từ hoàn thành bài học, bài tập, quest, hoạt động, hợp tác, thi đấu | [Confirmed] | P2–P4 |
| FR-GAM-003 | Reward Engine có rule + cap ngày/tuần + diminishing returns khi lặp nội dung; chống farm câu dễ | [Confirmed] | P2 |
| FR-GAM-004 | XP ledger append-only, idempotent theo `(source_type, source_id, student_id, rule_id)` | [Proposed] | P2 |
| FR-GAM-005 | Phần thưởng game không ảnh hưởng điểm/mastery/đánh giá chính thức | [Confirmed] | P2 |
| FR-RWD-001 | Inventory, cosmetic (outfit, hair, accessory, emote) mở khóa bằng bài tập, quest, sự kiện, thành tựu, thi đấu | [Confirmed] | P2 |
| FR-RWD-002 | Không cơ chế trả tiền để có kết quả học tập tốt hơn; không loot box | [Confirmed] / loot box [Proposed] | P2 |
| FR-RWD-003 | Reward có giá trị nghiệp vụ chỉ do server trao | [Confirmed] | P2 |
| FR-GAM-006 | Ba miền tiến trình (game, learning, competition) tách biệt, không gộp điểm; game level không phải bằng chứng học tập — [PROGRESSION_SYSTEM §2](../01-game-design/PROGRESSION_SYSTEM.md) | [Confirmed] | P2 |
| FR-GAM-007 | Catalog nguồn XP có mã; đường cong XP cấu hình được, có version, không hardcode trong Godot; danh hiệu theo khoảng level | [Confirmed] / tham số [Proposed] | P2 |
| FR-GAM-008 | Level-up event, phần thưởng lên cấp, progress bar, lịch sử level và giao dịch XP | [Confirmed] | P2 |
| FR-GAM-009 | World progression: mở khóa khu theo level/quest/LO/achievement, điểm khám phá, vật sưu tầm, hoàn thành bản đồ; khu học tập theo khối luôn mở; không trả tiền để mở khu | [Confirmed] / chi tiết [Proposed] | P2–P3 |
| FR-CHL-001 | Challenge Engine: 11 loại (time, perfect, survival, puzzle, knowledge, boss, coop, party, weekly, seasonal, tournament), 5 mức độ khó cấu hình theo grade/subject/LO/activity/hiệu suất/mode — [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) | [Confirmed] (Master Prompt mới §8) | P3 |
| FR-CHL-002 | Boss Battle nhiều phase, đóng góp cá nhân/nhóm, retry, không phạt khi sai; server-authoritative trong game-service | [Confirmed] / nơi chạy [Proposed] | P3–P4 |
| FR-CHL-003 | Mỗi câu/hoạt động trong challenge/boss là attempt học tập cá nhân; learning record riêng từng thành viên | [Confirmed] | P3 |
| FR-ECO-001 | Item catalog (cosmetic, outfit, hair, accessory, emote, pet, title, badge, frame, decoration, knowledge card), equipment slot, inventory, item history — [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | [Confirmed] | P2 |
| FR-ECO-002 | Tiền trong game "Xu" + sổ cái append-only, ví không âm, đối soát; không mua bằng tiền thật (ADR-0049) | [Proposed] (OQ-14) | P2 |
| FR-ECO-003 | Shop chỉ cosmetic, giá Xu, giới hạn mua, vật phẩm xác định; không loot box/hộp ngẫu nhiên trả phí | [Confirmed] không loot box / Shop [Proposed] | P2 |
| FR-ECO-004 | Collection (pet, knowledge card, cosmetic, badge, exploration, event), xử lý trùng lặp, trading tắt mặc định | [Confirmed] (Master Prompt mới §10.2) | P2–P3 |
| FR-ECO-005 | Achievement có định nghĩa, trigger, điều kiện, tiến độ, ẩn, tier, version; không dựa trên thời gian chơi | [Confirmed] | P2 |

### 2.25a. Event & Season (live-ops) — `EVT` (Master Prompt tái cấu trúc §12, §16.1)

Nguồn: [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md).

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-EVT-001 | Sự kiện live-ops: lễ hội thế giới, cuối tuần, sự kiện trường, community goal, chiến dịch học tập, boss đặc biệt, quest giới hạn thời gian; cấu hình dữ liệu, có version (ADR-0043) | [Confirmed] loại / [Proposed] danh mục | P3 |
| FR-EVT-002 | Event Builder trong Game Operations Console: lịch, đối tượng (khối/trường/tenant), nội dung tham chiếu, phần thưởng, preview, duyệt, publish, lên lịch, tiến độ, analytics | [Confirmed] | P3 |
| FR-EVT-003 | Vòng đời Scheduled → Live → Ended → Archived, dừng khẩn cấp/tiếp tục/hủy có lý do và audit; lượt chơi đang dở không bị cắt; tiến độ đã đạt không bao giờ bị giảm | [Proposed] | P3 |
| FR-EVT-004 | Community goal cộng dồn theo trường/tenant từ event server đã kiểm chứng, trần đóng góp mỗi học sinh/ngày, không xếp hạng cá nhân; thưởng mốc cho mọi người có đóng góp | [Proposed] | P3 |
| FR-EVT-005 | Season track miễn phí (không track trả phí), điểm mùa từ hoạt động đa dạng có trần/ngày, thưởng cosmetic; hết mùa tự cấp thưởng chưa nhận | [Confirmed] miễn phí / [Proposed] tham số | P4 |
| FR-EVT-006 | Tenant tổ chức sự kiện trường từ template platform, chỉ với `allowed_params` (ADR-0027, ADR-0029) | [Proposed] | P3 |
| FR-EVT-007 | Quy tắc lành mạnh: không phạt mất chuỗi, không ép online, không khóa học tập, tôn trọng giờ truy cập và nhắc nghỉ | [Confirmed — Master Prompt §12] | P3 |
| FR-EVT-008 | Nội dung sự kiện platform song ngữ bắt buộc (ADR-0056); trang trí trong ngân sách máy tầm trung, có biến thể preset Thấp (ADR-0057) | [Confirmed] | P3 |

Acceptance criteria: AC-EVT-001…012 ([EVENT_SEASON_SYSTEM §18](../01-game-design/EVENT_SEASON_SYSTEM.md#18-acceptance-criteria)).

### 2.26. CMS — `CMS` (§8)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CMS-001 | Vai trò: Platform Admin (→ `system_admin`), Content Admin, Academic Manager, School Admin, Teacher, Content Reviewer; bổ sung role tenant (`tenant_admin`, `tenant_content_editor/reviewer`, `tenant_moderator`) và role platform (`game_designer`, `liveops`, `competition_admin`) — [ADMIN_PORTAL §2](../04-admin-portal/ADMIN_PORTAL.md#2-permission-model) | [Confirmed] gốc / [Proposed] bổ sung | P3 |
| FR-CMS-002 | Curriculum Management (chương trình, khối, môn, chủ đề, LO) | [Confirmed] | P3 |
| FR-CMS-003 | Content Management: lesson, material, media, question bank, question set, assessment, exercise, learning activity | [Confirmed] | P3 |
| FR-CMS-004 | Game Content: quest, mini-game config, game level, NPC dialogue, reward config, world event, game activity — **chỉ platform** (workspace Game Operations Console, `/api/v1/game-admin/*`) | [Confirmed] (platform-only theo ADR-0027) | P3 |
| FR-CMS-005 | Competition Management: matchmaking config, ranking rules, tournament/challenge template, season, competition reward — **chỉ platform**; tenant tạo giải/thử thách từ template được gán | [Confirmed] (platform-only theo ADR-0027) | P4 |
| FR-CMS-006 | Workflow, versioning, preview (Godot Web export nhúng cho nội dung game), publishing, rollback theo chính sách, audit trail | [Confirmed] / preview Godot [Proposed] | P3 |
| FR-CMS-007 | CMS không làm thay đổi lịch sử bài đã hoàn thành | [Confirmed] | P3 |
| FR-CMS-008 | Giáo viên tạo nội dung phạm vi trường; reviewer cấp trường duyệt | [Proposed] OQ-07 | P3 |
| FR-CMS-009 | **Một Admin Portal** React (TypeScript) `web/apps/portal` với 5 workspace (School & Learning Management, Learning Content Studio, Game Operations Console, Game Live Operations, Platform Administration), OIDC qua `eduworld-portal` (ADR-0022, ADR-0027 mục 7); giao diện **Admin/Ops UI** cho vận hành game (Ant Design 5 + Ant Design Pro, theme EduWorld), **không** theo phong cách game (ADR-0047) | [Confirmed] React; [Proposed] TypeScript/Vite | P3 |
| FR-CMS-016 | Dashboard live-ops trong CMS/Admin: CCU, trận đang chạy, hàng chờ ghép trận, đăng nhập/hàng chờ đăng nhập, tin nhắn bị chặn, báo cáo chờ xử lý, lỗi theo service, publish/rollback gần đây, trạng thái SIS sync (DESIGN_SYSTEM §4.3) | [Proposed] | P3 |
| FR-CMS-017 | Thumbnail/turntable render tự động từ asset 3D của game cho mọi version publish, hiển thị trong CMS như dữ liệu nhận diện (ADR-0047) | [Proposed] | P3 |
| FR-CMS-010 | Vận hành của mỗi tenant đăng nhập Admin Portal (workspace School & Learning Management, Learning Content Studio) và thao tác chức năng Learning Management trong đúng một tenant | [Confirmed] ADR-0027 | P3 |
| FR-CMS-011 | Vận hành tenant không thấy và không thao tác được menu/cấu hình game; 4 lớp chặn: gateway, `azp`+MFA, permission catalog, bundle | [Confirmed] ADR-0027 | P3 |
| FR-CMS-012 | Menu dựng từ `GET /api/v1/me/permissions`; permission catalog trong code, role tenant không có `game.*`/`competition.config.*`/`platform.*` | [Confirmed] ADR-0027 | P1 |
| FR-CMS-013 | System Admin là người duy nhất xem dữ liệu liên tenant: dropdown tìm kiếm tenant (mặc định "Tất cả", chọn nhiều), cột Tenant, ghi luôn chọn một tenant đích | [Confirmed] ADR-0027 §6 | P3 |
| FR-CMS-014 | Xóa = soft-delete; dữ liệu đã xóa không hiển thị; Thùng rác cho System Admin (Tenant Admin nếu được cấp) | [Confirmed] ADR-0028 / [Proposed] Tenant Admin | P2 |
| FR-CMS-015 | System Admin gán dữ liệu platform cho tenant (chỉ đọc với tenant) và clone dữ liệu cấu hình/nội dung giữa tenant (không clone dữ liệu học sinh) | [Confirmed] ADR-0029 | P2 (gán) – P3 (clone) |

### 2.27. LMS — `LMS` (§9)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-LMS-001 | Module: Course, Learning Path, Enrollment, Assignment, Schedule, Lesson Delivery, Assessment, Learning Progress, Learning Analytics, Teacher Dashboard | [Confirmed] | P3 |
| FR-LMS-002 | Giáo viên giao bài tập, giao đề ôn tập (target: lớp/nhóm/danh sách HS), đặt open/due, số lần làm, hiển thị kết quả | [Confirmed] | P3 |
| FR-LMS-003 | Giáo viên theo dõi kết quả, tiến độ, kỹ năng yếu theo LO | [Confirmed] | P3 |
| FR-LMS-004 | Giáo viên tạo nhóm học tập và tổ chức thử thách học tập | [Confirmed] | P3 (nhóm) – P4 (thử thách) |
| FR-LMS-005 | Phân quyền theo trường, lớp, môn (TeachingAssignment) | [Confirmed] | P3 |
| FR-LMS-006 | Bài được giao xuất hiện trong game như quest/nhiệm vụ đặc biệt | [Proposed] | P3 |
| FR-LMS-007 | Giáo viên chỉ **chọn** quest/mini-game đã publish và được gán cho tenant để giao bài; thử thách/giải trường tạo từ template platform, chỉ điền `allowed_params` | [Confirmed] chọn, không cấu hình (ADR-0027) / [Proposed] `allowed_params` | P2–P4 |

### 2.28. Audit & Operations — `AUD`, `OPS` (§15, §17, §18)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-AUD-001 | Audit log cho: đăng nhập admin, thay đổi quyền, truy cập dữ liệu học sinh chéo phạm vi, publish nội dung, kết quả thi/trận, moderation, SIS sync | [Confirmed] | P1+ |
| FR-AUD-002 | Audit log append-only, partition theo tháng, retention theo OQ-12 | [Proposed] | P1 |
| FR-OPS-001 | Health, readiness, liveness endpoint cho mọi binary | [Confirmed] | P1 |
| FR-OPS-002 | Docker + Docker Compose local; environment example; migration; backup script; deployment docs | [Confirmed] | P1 |
| FR-OPS-003 | Metrics: online players, active sessions, matchmaking queue, match duration, API latency, error rate, chat rate, learning completion, exam submission, DB performance | [Confirmed] | P1–P6 |
| FR-OPS-004 | Không commit secrets; secret qua secret manager/Docker secrets | [Confirmed] | P1 |
| FR-OPS-005 | CI: lint, test, spec lint/breaking check, build Go/Godot/Web theo path | [Proposed] | P1 |

## 3. Non-functional Requirements

Giá trị số nhất quán với [SYSTEM_ARCHITECTURE §7](../06-architecture/SYSTEM_ARCHITECTURE.md#7-non-functional-baseline). NFR-PERF-001..004, NFR-PERF-011..016, NFR-SCAL-001..002, NFR-SCAL-005, NFR-AVAIL-001..002, NFR-SEC-001, NFR-PRIV-001, NFR-OBS-001 được định nghĩa ở đó và **không lặp lại** dưới đây; bảng sau bổ sung.

| ID | Yêu cầu | Giá trị | Nhãn |
|---|---|---|---|
| NFR-PERF-005 | Thời gian từ mở game đến vào Main Hub (đã đăng nhập, cache ấm) | Theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11 (hạng Tầm trung) | [Proposed] ADR-0057 |
| NFR-PERF-006 | Kích thước bản cài và gói nội dung theo vùng | Theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11; nội dung vùng tải theo nhu cầu | [Proposed] ADR-0057 |
| NFR-PERF-007 | Matchmaking wait p90 | ≤ 45 giây khi queue đủ người | [Proposed] |
| NFR-PERF-008 | Hub sync tick và băng thông | 10 Hz; băng thông theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §10 | [Proposed] |
| NFR-PERF-017 | Thời gian truy vấn dashboard (giáo viên, trường, tenant, platform, Game Analytics) tại API | p95 ≤ 2 giây với dữ liệu 2 năm; không truy vấn nào chạy quá 5 giây ([DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md), ADR-0062) | [Proposed] |
| NFR-PERF-018 | Dashboard không ảnh hưởng database nghiệp vụ | Chỉ đọc bảng rollup trên read replica; job chốt ngày làm tăng p95 ghi primary < 20% | [Confirmed — ADR-0062] |
| NFR-SCAL-003 | Scale ngang từng microservice (api, worker) và `realtime-gateway` độc lập (ADR-0036) | — | [Proposed] |
| NFR-SCAL-004 | Bảng lớn (learning_events, chat messages, audit logs, attempt answers) partition theo tháng | — | [Confirmed] cần kế hoạch / [Proposed] cách làm |
| NFR-AVAIL-003 | Mất Valkey không làm mất dữ liệu học tập; realtime suy giảm có kiểm soát | — | [Confirmed] |
| NFR-AVAIL-004 | Graceful shutdown: drain WS, hoàn tất request, không mất outbox | — | [Confirmed] |
| NFR-SEC-002 | TLS 1.2+ cho mọi kết nối ngoài; mật khẩu/secret không trong log | — | [Confirmed] |
| NFR-SEC-003 | Rate limit theo user & IP tại APISIX; rate limit nghiệp vụ trong Go cho submit, invite, chat | — | [Confirmed] |
| NFR-SEC-004 | Idempotency cho thao tác quan trọng (ADR-0010) | — | [Confirmed] |
| NFR-SEC-005 | Quét lỗ hổng dependency và image trong CI; pentest trước production | — | [Confirmed] |
| NFR-SEC-006 | Tenant isolation test tự động: HS/GV trường A không truy cập dữ liệu trường B | 100% endpoint tenant-scoped | [Confirmed] |
| NFR-PRIV-002 | Data minimization: không lưu dữ liệu học sinh không cần cho mục đích học tập | — | [Confirmed] |
| NFR-PRIV-003 | Retention & deletion policy có job tự động | Theo OQ-12 | [TBD] |
| NFR-PRIV-004 | Đánh giá pháp lý (PDP Việt Nam, bảo vệ trẻ em) trước production | — | [Confirmed] |
| NFR-SAFE-001 | 100% tin nhắn (mọi channel, mọi `kind`) qua pipeline kiểm tra hợp lệ phía server trước khi phát; không có đường gửi nào bỏ qua bộ lọc | — | [Confirmed] |
| NFR-SAFE-002 | Pre-send filter p95 ≤ 50 ms | — | [Proposed] |
| NFR-SAFE-003 | Không thiết kế cơ chế gây nghiện: không streak phạt, không thưởng đăng nhập liên tục mang tính ép buộc | — | [Confirmed] nguyên tắc / [Proposed] biện pháp |
| NFR-A11Y-001 | Font tối thiểu 24 px (1080p) cho lớp 1–3; tương phản WCAG AA cho UI chữ; hỗ trợ đọc to câu hỏi | — | [Proposed] |
| NFR-A11Y-003 | CMS/LMS/Admin đạt WCAG 2.1 AA (tương phản, bàn phím, trình đọc màn hình); kiểm tra tự động khi build token và bằng axe (ADR-0047) | — | [Proposed] |
| NFR-MAINT-007 | Không hardcode màu/font/kích thước trong web và scene Godot; mọi giá trị giao diện từ design token, CI chặn vi phạm (ADR-0047) | — | [Proposed] |
| NFR-A11Y-002 | Không yêu cầu phản xạ nhanh cho nội dung học tập bắt buộc; chế độ không giới hạn thời gian cho practice | — | [Proposed] |
| NFR-I18N-001 | **Song ngữ** `vi-VN` (mặc định) + `en-US`: mọi chuỗi UI game qua file dịch, không hardcode; đổi ngôn ngữ lúc chạy không cần khởi động lại ([ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md)) | — | [Confirmed] |
| NFR-I18N-002 | Văn bản game (tên vùng/NPC/item/quest/hội thoại/sự kiện/chương truyện) bắt buộc đủ `vi` + `en` trước khi publish; nội dung học tập có `content_language`, bản dịch tùy chọn, fallback có nhãn | — | [Confirmed] game · [Proposed] học tập |
| NFR-I18N-003 | Bố cục chịu chuỗi dài hơn 35%; font phủ đủ tiếng Việt có dấu và Latin; định dạng số/ngày/giờ theo locale | — | [Proposed] |
| NFR-I18N-004 | Không dịch máy chat giữa người chơi; quick message và tin hệ thống hiển thị theo ngôn ngữ người xem | — | [Confirmed] |
| NFR-OBS-002 | Structured log JSON, trace OTel (lan truyền qua HTTP service-to-service và NATS), metrics Prometheus cho mọi service và APISIX, nhãn `service` bắt buộc (ADR-0023) | — | [Confirmed] |
| NFR-OBS-003 | Client telemetry không chứa PII | — | [Proposed] |
| NFR-MAINT-001 | Clean Architecture trong mỗi service (domain → application → infrastructure/presentation), kiểm tra bằng linter; không import chéo giữa service, chỉ dùng chung `pkg/` (ADR-0036, ADR-0038) | — | [Confirmed] stakeholder 2026-11-15 |
| NFR-MAINT-002 | Contract-first: spec public **và internal** trước code, CI chặn breaking change không có version mới; contract test consumer-driven giữa service | — | [Confirmed] |
| NFR-MAINT-003 | Code coverage unit test domain/application layer ≥ 70% mỗi service | — | [Proposed] |
| NFR-MAINT-004 | CQRS: command thay đổi trạng thái qua aggregate + outbox; query không ghi dữ liệu (ADR-0038) | — | [Confirmed] stakeholder 2026-11-15 |
| NFR-MAINT-005 | **Mọi trường/cột kiểu chuỗi có giới hạn độ dài** ở DB (`varchar(n)`, ent `MaxLen`), API (OpenAPI/AsyncAPI `maxLength`) và JSON Schema; CI chặn thiếu giới hạn; vượt giới hạn trả `422` (ADR-0041) | 100% trường chuỗi | [Confirmed] stakeholder 2026-11-15 |
| NFR-MAINT-006 | Gọi service-to-service chỉ qua typed client sinh từ spec internal, có timeout/retry/circuit breaker và token `svc-*` (ADR-0040) | — | [Confirmed] stakeholder 2026-11-15 |
| NFR-DEPL-001 | **Triển khai độc lập**: mỗi service build, migrate, deploy, scale và rollback riêng mà không cần deploy service khác; thay đổi tương thích ngược giữa phiên bản N và N-1 của service lân cận (ADR-0036) | — | [Confirmed] stakeholder 2026-11-15 |
| NFR-DEPL-002 | Mỗi service một database riêng; không service nào truy cập database của service khác (ADR-0037) | — | [Confirmed] stakeholder 2026-11-15 |
| NFR-AVAIL-005 | (Đổi mã 2026-11-15, trước trùng NFR-AVAIL-003) Một service lỗi không làm sập service khác: các chức năng không phụ thuộc vẫn hoạt động; chức năng phụ thuộc suy giảm có thông báo (circuit breaker) | Chaos test TC-SVC-010…013 pass | [Proposed] |
| NFR-COMP-001 | **Chạy được trên máy tầm trung** (yêu cầu bắt buộc): mọi tính năng Release 1.0 đạt FPS mục tiêu ở hạng Tầm trung (PC iGPU UHD 630/Vega 8 + 8 GB RAM; Android 4 GB RAM Mali-G52/Adreno 610; iPhone 11; Web WebGL 2 trên PC tầm trung) và chơi được ở hạng Tối thiểu với preset Thấp; hạng, FPS, renderer, preset, ngân sách theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) | AC-PERF-001…011 | [Confirmed] yêu cầu ADR-0057 · thông số [Proposed] OQ-04 |
| NFR-COMP-003 | iOS 15+ (iPhone 11 / iPad thế hệ 8 trở lên là hạng Tầm trung) | Theo NFR-COMP-001 | [Proposed] OQ-04 |
| NFR-COMP-004 | Web: Chrome/Edge bản mới bắt buộc (Firefox/Safari [TBD]), WebGL 2, renderer Compatibility, preset tối đa Trung bình; tải đầu và bộ nhớ theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11, §14 | Theo NFR-COMP-001 | [Proposed] OQ-04 |
| NFR-COMP-002 | Client cũ vẫn hoạt động ≥ 1 học kỳ sau khi ra API v mới | — | [Proposed] |

## 4. User Stories chính

| ID | Là | Tôi muốn | Để | Phase |
|---|---|---|---|---|
| US-AUTH-001 | Học sinh | đăng nhập bằng username/mật khẩu trường cấp ngay trong game, không phải chọn trường (ADR-0063, ADR-0068, ADR-0069) | vào game mà không cần tạo tài khoản mới | P1 |
| US-AUTH-002 | Học sinh | được game nhớ tên đăng nhập, mỗi lần mở game chỉ nhập lại mật khẩu (ADR-0064) | vào nhanh mà tài khoản vẫn an toàn khi dùng chung máy | P1 |
| US-AUTH-003 | School Admin | khóa tài khoản học sinh đã nghỉ học | học sinh đó không vào được hệ thống | P1 |
| US-SIS-001 | School Admin | nhập danh sách lớp/học sinh từ SIS (CSV hoặc API) | không phải tạo tay từng tài khoản | P1 |
| US-SIS-002 | School Admin | xem và xử lý các bản ghi xung đột khi đồng bộ | dữ liệu học sinh chính xác | P1 |
| US-GAME-001 | Học sinh lớp 2 | chọn ngoại hình nhân vật bằng hình ảnh lớn, dễ bấm | có nhân vật của riêng mình | P2 |
| US-GAME-002 | Học sinh | đi lại trong Main Hub và đến Learning Zone môn Toán | bắt đầu học theo cách của game | P2 |
| US-QST-001 | Học sinh | nhận nhiệm vụ giúp NPC giải bài toán | học kiến thức thông qua câu chuyện | P2 |
| US-QST-002 | Học sinh | thấy nhiệm vụ hằng ngày phù hợp lớp mình | có mục tiêu ngắn mỗi ngày | P2 |
| US-MG-001 | Học sinh | chơi mini-game nối từ/kéo thả với câu hỏi đúng chương trình | luyện tập vui hơn | P2 |
| US-EXR-001 | Học sinh | làm lại các câu đã sai | củng cố kỹ năng còn yếu | P2 |
| US-EXR-002 | Học sinh | tiếp tục bài đang làm dở khi bị mất mạng | không mất công làm lại | P2 |
| US-GAM-001 | Học sinh | nhận XP và trang phục mới khi hoàn thành quest | có động lực tiếp tục | P2 |
| US-EXM-001 | Học sinh lớp 9 | làm đề thi thử có giới hạn thời gian và xem phân tích lỗi sai | chuẩn bị cho kỳ thi | P3 |
| US-LMS-001 | Giáo viên | giao bộ bài tập chương 3 cho lớp 4A, hạn thứ Sáu | học sinh luyện tập đúng trọng tâm | P3 |
| US-LMS-002 | Giáo viên | xem LO nào lớp còn yếu | điều chỉnh bài giảng | P3 |
| US-CMS-001 | Content Admin | soạn câu hỏi, gửi duyệt, xuất bản phiên bản mới | nội dung được kiểm soát chất lượng | P3 |
| US-CMS-002 | Content Reviewer | xem preview câu hỏi như trong game trước khi duyệt | phát hiện lỗi hiển thị | P3 |
| US-PTY-001 | Học sinh A | tạo party và mời bạn B, C, D | cùng làm quest với nhau | P4 |
| US-PTY-002 | Thành viên party | bấm Ready và thấy trạng thái các bạn | biết khi nào cả nhóm sẵn sàng | P4 |
| US-PTY-003 | Leader | chọn hoạt động và bắt đầu phiên chơi chung | cả nhóm vào cùng instance | P4 |
| US-PTY-004 | Thành viên party | tiếp tục chơi khi một bạn bị rớt mạng | không bị gián đoạn cả nhóm | P4 |
| US-MM-001 | Học sinh lớp 5 | tìm đối thủ cùng khối để đấu Toán 1v1 | thi đấu công bằng | P4 |
| US-LDB-001 | Học sinh | xem bảng xếp hạng tuần của lớp | biết mình đang ở đâu | P4 |
| US-TRN-001 | Giáo viên | tổ chức giải đấu kiến thức cấp lớp | tạo thi đua lành mạnh | P4 |
| US-SOC-001 | Học sinh lớp 7 | gửi lời mời kết bạn tới bạn cùng trường | chơi và học cùng nhau | P5 |
| US-CHAT-001 | Học sinh lớp 2 | gửi tin nhắn nhanh "Cố lên!" cho nhóm | giao tiếp mà không cần gõ chữ | P5 |
| US-CHAT-002 | Học sinh | báo cáo tin nhắn khiến mình khó chịu | được bảo vệ | P5 |
| US-MOD-001 | Giáo viên chủ nhiệm | xem và xử lý báo cáo trong lớp mình | giữ môi trường an toàn | P5 |
| US-MOD-002 | School Admin | cấu hình chính sách chat cho từng khối | phù hợp độ tuổi và quy định trường | P5 |

## 5. Acceptance Criteria chính

Mỗi AC phải được tự động hóa thành test (xem [TEST_STRATEGY](../12-project/TEST_STRATEGY.md)) trừ khi ghi chú "manual".

### 5.1. Authentication (Master Prompt §22)

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-AUTH-001 | Học sinh có bản ghi SIS hợp lệ đăng nhập bằng form trong game (`/auth/game/login`, ADR-0068) và nhận player bootstrap với đúng school, grade, class | P1 |
| AC-AUTH-002 | Access token hết hạn → client tự refresh, request được thử lại một lần, người dùng không bị đẩy ra | P1 |
| AC-AUTH-003 | Refresh token hết hạn/bị revoke → client quay về màn hình đăng nhập, không crash | P1 |
| AC-AUTH-004 | Logout hoặc thoát game → refresh token bị revoke, token trong RAM bị xóa, kết nối WS đóng (ADR-0064) | P1 |
| AC-AUTH-005 | User đăng nhập Keycloak nhưng không tồn tại trong SIS/dữ liệu đồng bộ → không tạo tài khoản, trả lỗi `ACCOUNT_NOT_PROVISIONED`, ghi audit | P1 |
| AC-AUTH-006 | Học sinh bị khóa → REST trả 403 `ACCOUNT_LOCKED`, kết nối realtime bị cắt trong ≤ 60 giây | P1 |
| AC-AUTH-007 | Học sinh chuyển lớp → sau sync, bootstrap trả lớp mới, lịch sử học tập lớp cũ vẫn truy xuất được | P1 |
| AC-AUTH-008 | Học sinh chuyển trường → không còn truy cập dữ liệu/channel trường cũ; dữ liệu trường cũ không lộ cho trường mới ngoài phạm vi policy | P1 |
| AC-AUTH-009 | Token có `aud` sai hoặc chữ ký sai bị từ chối cả ở APISIX và backend (test gọi thẳng backend) | P1 |
| AC-AUTH-010 | Token của học sinh tenant A mang claim `organization` của tenant B (giả lập) → backend trả `TENANT_MISMATCH`, có audit; đăng nhập lại bình thường → vào tenant A | P1 |
| AC-AUTH-011 | Đăng nhập không chọn tenant: user thuộc 1 tenant (SIS hoặc tenant chung do platform tạo) → vào tenant đó; thuộc nhiều tenant → `403 TENANT_NOT_SELECTED` kèm danh sách, chọn sau đăng nhập, không nhập lại mật khẩu; không thuộc tenant chung và không thuộc tenant SIS nào → `403 ACCOUNT_NOT_PROVISIONED`, không vào game, phiên Keycloak bị thu hồi, có audit, không tạo bản ghi trong tenant chung | P1 |
| AC-AUTH-012 | Giáo viên dạy ở 2 tenant: phiên tenant A không đọc được dữ liệu tenant B; đổi sang tenant B không phải nhập lại mật khẩu | P1 |
| AC-AUTH-013 | Học sinh chuyển sang trường thuộc tenant khác: sau SIS sync, phiên cũ bị thu hồi; đăng nhập lại → backend tự xác định tenant mới với lịch sử của chính học sinh. Chuyển trường trong cùng tenant `unit` → phiên và membership không đổi | P1 |
| AC-AUTH-014 | Máy phòng tin học (`device.cfg` `shared_device: true`): không nhớ username; thoát game hoặc 30 phút không hoạt động → đăng xuất; học sinh dùng máy sau phải nhập tài khoản và mật khẩu của mình | P1 |
| AC-AUTH-015 | Giáo viên/Tenant Admin/System Admin đăng nhập game → `403 GAME_ACCESS_STUDENT_ONLY`, không tạo game session/login event, có audit | P1 |
| AC-AUTH-016 | Học sinh đang chơi trên máy A đăng nhập máy B → B hiện hộp thoại xác nhận; chọn tiếp tục → A nhận thông báo "đã đăng xuất vì đăng nhập thiết bị khác" và về màn hình đăng nhập; refresh token của A không còn dùng được; chọn hủy → A không bị ảnh hưởng | P1 |
| AC-AUTH-017 | Mỗi lần vào game tạo đúng 1 `game.login_events` và cập nhật `last_login_at`; vận hành tenant xem được lịch sử, không thấy IP | P1 |
| AC-AUTH-019 | Màn đăng nhập game chỉ có username/password (+ ngôn ngữ, hướng dẫn quên mật khẩu); không có bước chọn trường, ô mã trường, nút đăng nhập nhanh, "Ghi nhớ đăng nhập" (ADR-0063, ADR-0064) | P1 |
| AC-AUTH-020 | Thoát game rồi mở lại (kể cả ngay sau đó) → bắt buộc nhập mật khẩu; kiểm tra đĩa, `user://`, Keychain/Keystore không có access/refresh token | P1 |
| AC-AUTH-021 | Game đang mở, mất mạng 2 phút rồi có lại → reconnect bằng refresh token trong RAM, không hỏi mật khẩu | P1 |
| AC-AUTH-022 | Log APISIX, log identity-service, trace và crash report không chứa mật khẩu mẫu sau khi gọi `POST /auth/game/login` (ADR-0068) | P1 |
| AC-AUTH-023 | Sai mật khẩu 5 lần → tài khoản khóa tạm; lần 6 với mật khẩu đúng vẫn nhận `401 INVALID_CREDENTIALS` cùng thông điệp; sau thời gian chờ đăng nhập được | P1 |
| AC-AUTH-024 | Thiếu `X-Tenant-Id` ở route tenant-scoped → `400 TENANT_HEADER_REQUIRED`; header khác tenant của phiên → `403 TENANT_MISMATCH` + audit (ADR-0065) | P1 |
| AC-AUTH-025 | Học sinh đăng nhập portal → bị chặn ở Keycloak; gọi API portal bằng token học sinh → `403 PORTAL_ACCESS_STAFF_ONLY` (ADR-0066) | P1 |
| AC-AUTH-026 | Tạo tài khoản với username/email đã tồn tại (khác hoa thường, kể cả tenant khác) → `409 USERNAME_TAKEN`/`EMAIL_TAKEN`; SIS gửi trùng (người khác) → issue `duplicate_identifier`; cùng một người → thêm membership (ADR-0067, ADR-0069) | P1 |
| AC-AUTH-018 | Học sinh không đăng nhập quá số ngày cấu hình (không bị loại trừ) bị soft-delete sau khi đã xuất hiện trong danh sách "sắp bị dọn"; đăng nhập lại khi vẫn hợp lệ trong SIS → khôi phục đủ dữ liệu | P2 |

### 5.2. Learning

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-EXR-001 | Tải bài: response không chứa `answer_key`/`scoring_rule` cho bất kỳ loại câu hỏi nào (contract test) | P2 |
| AC-EXR-002 | Làm bài & chấm: điểm do server tính; gửi field `score`/`is_correct` từ client bị bỏ qua hoặc từ chối | P2 |
| AC-EXR-003 | Submit hai lần cùng `Idempotency-Key` → cùng kết quả, một bản ghi; khác key sau khi đã submit → 409 `ATTEMPT_ALREADY_SUBMITTED` | P2 |
| AC-EXR-004 | Làm lại bài tạo attempt mới, lịch sử attempt cũ giữ nguyên | P2 |
| AC-EXR-005 | Attempt lưu đúng `question_version_id`; publish version mới không thay đổi kết quả attempt cũ | P2 |
| AC-EXR-006 | Resume: mất kết nối giữa bài, mở lại trong TTL → tiếp tục đúng câu đang làm | P2 |
| AC-EXM-001 | Hết `deadline_at` → server tự nộp; câu trả lời gửi sau deadline bị từ chối | P3 |
| AC-EXM-002 | Kết quả exam tồn tại sau khi xóa/khởi động lại Valkey | P3 |
| AC-PRG-001 | Một attempt điểm tuyệt đối không đưa LO lên `mastered` nếu chưa đủ điều kiện evidence (ADR-0017) | P2 |

### 5.3. Party (Party Addendum §8)

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-PTY-001 | Học sinh A tạo được Party; A là leader | P4 |
| AC-PTY-002 | A mời B, C, D; B, C, D nhận lời mời realtime và chấp nhận được | P4 |
| AC-PTY-003 | Các thành viên thấy nhau trong Party Lobby với avatar, online, ready state cập nhật ≤ 1 giây | P4 |
| AC-PTY-004 | Leader chọn hoạt động; chỉ leader thực hiện được Start Session | P4 |
| AC-PTY-005 | Mọi thành viên đủ điều kiện vào cùng Game Session/room; thành viên không đủ điều kiện (sai khối theo activity) bị chặn với lý do rõ ràng | P4 |
| AC-PTY-006 | Các thành viên cùng thực hiện được một quest cooperative với shared progress theo cấu hình | P4 |
| AC-PTY-007 | Một thành viên mất kết nối → thành viên còn lại tiếp tục nếu activity cho phép; thành viên đó reconnect trong grace period trở lại đúng room | P4 |
| AC-PTY-008 | Leader mất kết nối quá grace period → leader tự chuyển, mọi thành viên nhận event `party.leader_changed` | P4 |
| AC-PTY-009 | Mỗi thành viên có attempt và learning result riêng; không thành viên nào thấy điểm chi tiết của người khác trừ khi activity cấu hình hiển thị tổng điểm đội | P4 |
| AC-PTY-010 | Người không được mời không join được party private; join khi đã ở party khác bị từ chối `PARTY_ALREADY_IN_PARTY` | P4 |
| AC-PTY-011 | Vượt rate limit mời → 429; mọi kick/transfer/disband có audit log | P4 |

### 5.4. Competition

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-CMP-001 | Hai học sinh cùng khối vào queue → được ghép, ready check, vào trận | P4 |
| AC-CMP-002 | Học sinh lớp 1 và lớp 9 cùng rating không bao giờ được ghép | P4 |
| AC-CMP-003 | Disconnect/reconnect trong grace period → tiếp tục trận với state đúng | P4 |
| AC-CMP-004 | Hoàn thành trận → kết quả tính server-side, rating cập nhật đúng Glicko-2 (test vector chuẩn) | P4 |
| AC-CMP-005 | Gửi kết quả trận hai lần (retry/replay) → không tính trùng | P4 |
| AC-CMP-006 | Client gửi câu trả lời cho câu hỏi đã đóng hoặc không thuộc trận → bị từ chối và ghi anomaly | P4 |

### 5.5. Chat

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-CHAT-001 | Tạo channel class/party theo policy; thành viên hợp lệ tham gia được | P5 |
| AC-CHAT-002 | Gửi/nhận tin nhắn realtime; unread count đúng sau reconnect | P5 |
| AC-CHAT-003 | Cùng một tin nhắn ở cùng loại channel cho cùng quyết định (`allow`/`mask`/`block`) với học sinh lớp 1–3 và lớp 6–9; tin chứa PII bị chặn như nhau | P5 |
| AC-CHAT-004 | Tin nhắn chứa số điện thoại/email/link → bị block hoặc mask theo policy | P5 |
| AC-CHAT-005 | A block B → B không gửi DM, không mời party, không thấy A trong danh sách gợi ý | P5 |
| AC-CHAT-006 | Report tạo ModerationCase trong queue của đúng trường | P5 |
| AC-MOD-020 | Thêm term mới vào danh mục và publish → trong ≤ 60 s tin chứa term (kể cả bản không dấu/teencode) bị xử lý theo action của category, không cần deploy | P5 |
| AC-MOD-021 | System Admin tắt toàn bộ kiểm duyệt cho tenant X (có lý do) → tin ở tenant X gửi được với decision `unchecked`, vẫn bị giới hạn độ dài/rate limit; tenant Y không đổi; có audit, banner, alert | P5 |
| AC-MOD-022 | Tenant Admin không bật/tắt được công tắc kiểm duyệt; chỉ bật/tắt category có `allow_tenant_toggle` | P5 |
| AC-CHAT-007 | Không truy cập channel trái phép; học sinh/giáo viên trường A không đọc được tin nhắn trường B (kể cả qua ID đoán) | P5 |
| AC-CHAT-008 | Trận 1v1 bắt đầu → hai người chat được ngay trong channel `match`; trận đội → mỗi đội có channel `team` riêng; tạo party → channel `party` có ngay, người vào sau tự là member | P4 |
| AC-CHAT-009 | Policy trường/lớp không tắt được chat `match`/`team`/`party` (`CHAT_POLICY_ALWAYS_ON`) | P4 |
| AC-CHAT-010 | Tin trong trận đóng ngày D bị purge sau D + `gameplay_chat_retention_days`; tin đã bị báo cáo vẫn xem được trong moderation case sau purge; channel có case mở không bị purge; channel Class/DM không bao giờ bị purge | P4 |
| AC-CHAT-011 | Người gửi chỉ thấy trạng thái "đã gửi" sau khi NATS JetStream ack; khi NATS lỗi tin hiện "chưa gửi được" và tự gửi lại, không tạo trùng |
| AC-CHAT-012 | Sửa tin sau 15 phút hoặc thu hồi sau 24 giờ bị từ chối; tin đã thu hồi vẫn báo cáo được và moderator thấy nội dung gốc |
| AC-CHAT-013 | Forward sang channel của tenant khác bị từ chối; GIF chỉ chọn được từ thư viện nội bộ |
| AC-CHAT-014 | Trong party 4 người, người gửi thấy ai đã nhận/đã xem; tìm "nhan vat" ra tin "nhân vật" trong nhóm mình tham gia |

### 5.6. Multi-tenancy

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-ORG-001 | Người dùng tenant A không truy cập được bất kỳ tài nguyên nào của tenant B (test tự động trên mọi endpoint tenant-scoped) | P1+ |
| AC-ORG-002 | Giáo viên chỉ xem/quản lý lớp được phân công | P3 |
| AC-ORG-003 | School Admin chỉ quản lý phạm vi trường được phân quyền | P1 |
| AC-ORG-004 | System Admin quản trị toàn hệ thống và là người duy nhất xem dữ liệu liên tenant (dropdown tenant); mọi truy cập dữ liệu học sinh được audit | P1 |
| AC-CMS-001 | School Admin/Tenant Admin đăng nhập Admin Portal: chỉ thấy workspace B và C, không có menu Game Operations/Live Ops/Platform; gọi `/api/v1/game-admin/*` qua gateway public → không có route; gọi thẳng backend bằng token của mình → `403 PERMISSION_DENIED` + audit | P3 |
| AC-CMS-002 | Người dùng tenant gửi `tenant_ids` của tenant khác → tham số bị bỏ qua, chỉ thấy dữ liệu tenant mình | P3 |
| AC-CMS-003 | System Admin mở danh sách câu hỏi không chọn tenant → thấy mọi tenant, có cột Tenant; tạo bản ghi mà không chọn tenant đích (hoặc phạm vi platform) → `422` | P3 |
| AC-CMS-004 | Xóa một câu hỏi → biến mất khỏi danh sách, xuất hiện trong Thùng rác; bài đã làm vẫn xem lại được; khôi phục thành công | P2 |
| AC-CMS-005 | System Admin gán question set platform cho tenant A → giáo viên A thấy (chỉ đọc), giáo viên B không thấy; clone question set từ A sang B → B có bản draft riêng, không mang dữ liệu học sinh | P3 |
| AC-CNT-001 | Học sinh A đang ở màn dùng quest v3; CMS publish v4 → A vẫn thấy v3 đến khi rời màn; học sinh B vào màn sau publish thấy v4 |
| AC-CNT-002 | Rollback câu hỏi từ v5 về v4 có lý do → lượt làm bài mới dùng v4; `content_releases` ghi lại; attempt đang làm với v5 không bị đổi |
| AC-CNT-003 | Xem version cũ đã publish ở chế độ chỉ đọc và diff với version hiện hành |
| AC-CNT-004 | Upload ảnh có EXIF GPS → biến thể đã xử lý không còn EXIF; file sai magic bytes bị từ chối `MEDIA_TYPE_NOT_ALLOWED` |
| AC-CNT-005 | Giáo viên chấm bài tự luận theo rubric 4 tiêu chí → học sinh thấy điểm từng tiêu chí + nhận xét; mastery cập nhật cho LO gắn tiêu chí; chấm lại không sửa evaluation cũ |
| AC-CNT-006 | Video có câu hỏi chèn tại 01:30 → video dừng đúng mốc; tua tới cuối không làm activity hoàn thành |
| AC-CNT-007 | Mô phỏng đòn bẩy: client báo đạt nhưng action log không đạt → server trả không đạt, ghi integrity signal |
| AC-CNT-008 | Câu hỏi có công thức hiển thị SVG trong game; học sinh trả lời bằng chọn đáp án/nhập số; LaTeX ngoài allowlist bị chặn `MATH_RENDER_FAILED` khi lưu |
| AC-ORG-005 | Bỏ điều kiện tenant trong query (lỗi lập trình giả lập) vẫn bị RLS chặn | P1 |
| AC-ORG-006 | Dữ liệu đã soft-delete không xuất hiện ở bất kỳ API đọc nào; role `<svc>_app` chạy `DELETE` → lỗi quyền; CI phát hiện query thiếu `tenant_id`/`deleted_at IS NULL` | P1 |
| AC-ORG-007 | Học sinh tenant chung chỉ thấy nội dung platform được gán cho `common`; không thấy dữ liệu của tenant nào khác | P1 |

### 5.7. Gamification

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-GAM-001 | Làm lại cùng bài đã hoàn thành nhiều lần → XP giảm dần rồi về 0 theo rule | P2 |
| AC-GAM-002 | Vượt cap XP ngày → không cộng thêm, học sinh được thông báo thân thiện | P2 |
| AC-GAM-003 | Replay event `exercise.submitted` → XP không cộng trùng | P2 |
| AC-GAM-004…010 | Cộng XP, level-up + phần thưởng một lần, publish curve không giảm level, mở khóa khu, khu học tập luôn mở, khám phá ([PROGRESSION_SYSTEM §10](../01-game-design/PROGRESSION_SYSTEM.md)) | P2 |
| AC-GAM-011 | Học sinh mới đăng nhập lần đầu → màn tạo avatar → chọn preset + nickname → vào Main Hub; `avatar.created` phát một lần | P2 |
| AC-CHL-001…008 | Challenge theo giờ server, độ khó đúng khối, attempt học tập riêng, boss party chỉ thưởng người đủ đóng góp, retry phase không phạt, idempotent, reconnect ([CHALLENGE_BOSS_SYSTEM §10](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)) | P3 |
| AC-ECO-001…008 | Mua idempotent, không đủ Xu, đối soát ví, không tiền thật/loot box, quy đổi trùng lặp, achievement ẩn, knowledge card, trang bị item không sở hữu ([INVENTORY_ECONOMY §12](../01-game-design/INVENTORY_ECONOMY.md)) | P2 |

### 5.8. Game design và thế giới

| ID | Tiêu chí | Tài liệu sở hữu |
|---|---|---|
| AC-GDD-001…005 | Quest học tập có ngữ cảnh game, mỗi vùng có co-op/boss/khám phá, không tiền thật/loot box, học tập không bị khóa bởi level, phần thưởng game không đổi kết quả học tập | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md) |
| AC-WLD-001…004 | Đủ 7 khu chức năng trong 8 vùng, chuyển vùng < 5 s, instance theo trường, world map đúng % khám phá | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| AC-CMP-101…102 | Không ghép ranked lệch quá 1 khối (lớp 1–5 cùng khối); không lộ rating thô | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |

## 6. Phụ thuộc và giả định

| Giả định | Nhãn | Nếu sai thì |
|---|---|---|
| Nhà trường cung cấp được danh sách học sinh/lớp (CSV hoặc API) | [Proposed] | Không thể provisioning → chặn P1 pilot |
| Trường có IdP hoặc chấp nhận tài khoản local Keycloak | [TBD] OQ-02 | Thay đổi login flow cho từng trường |
| Danh sách thiết bị tham chiếu theo hạng (PERFORMANCE_BUDGET §2) khớp thiết bị thực tế ở trường | [TBD] OQ-04 | Hạ preset mặc định/ngân sách, không loại hạng Tầm trung (ADR-0057) |
| Nội dung pilot (Toán 1–5) có nguồn và bản quyền hợp lệ | [TBD] OQ-08 | Chậm P2 |
| Chính sách chat mặc định được trường/pháp lý chấp nhận | [TBD] OQ-09, OQ-12 | Điều chỉnh policy trước P5 |

## 7. Truy vết yêu cầu → tài liệu thiết kế

PRD là danh sách yêu cầu gốc; chi tiết, tham số và AC mở rộng nằm ở tài liệu sở hữu dưới đây. Giá trị mặc định cấu hình được: [CONFIG_DEFAULTS](CONFIG_DEFAULTS.md).

| Domain (mã) | Tài liệu sở hữu |
|---|---|
| GAME (thế giới, nhân vật), QST, GAM/RWD (XP, level, phần thưởng), CHL, ECO, CMP/RNK/LDB/TRN | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md) (GDD tổng), [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md), [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md), [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md), [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md), [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md), [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md), [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| CUR, CNT, QB, EXR, EXM, PRG, ANL, MG (activity) | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md), [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md), [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), [QUESTION_BANK](../02-learning/QUESTION_BANK.md), [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md), [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md), [WRITTEN_RESPONSE_RUBRIC](../02-learning/WRITTEN_RESPONSE_RUBRIC.md), [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md), [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md), [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) |
| MP, PTY, MM, SOC, CHAT, MOD, NTF | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md), [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md), [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md), [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md), [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [MODERATION](../03-social-multiplayer/MODERATION.md) |
| CMS, LMS | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md), [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md), [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) |
| AUTH, ORG, SIS | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md), [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md), [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md), [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md), [MULTITENANCY](../06-architecture/MULTITENANCY.md) |
| Client game | [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md), [WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md), [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md), [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) |
| Kiến trúc, API, dữ liệu | [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md), [DOMAIN_MODEL](../06-architecture/DOMAIN_MODEL.md), [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md), [API_DESIGN](../07-backend/API_DESIGN.md), [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md), [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) |
| AUD, OPS, NFR-SEC/PRIV/SAFE | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md), [CHILD_SAFETY](../10-security/CHILD_SAFETY.md), [DATA_PRIVACY](../10-security/DATA_PRIVACY.md), [ANTI_CHEAT](../10-security/ANTI_CHEAT.md), [DEPLOYMENT](../11-operations/DEPLOYMENT.md), [MONITORING](../11-operations/MONITORING.md), [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) |
| Kiểm thử, kế hoạch | [TEST_STRATEGY](../12-project/TEST_STRATEGY.md) (§6.0 ánh xạ 20 kịch bản nghiệm thu), [ROADMAP](../12-project/ROADMAP.md) |

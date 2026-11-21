# Product Requirements Document (PRD) — EduWorld

> Trạng thái: Draft · Phiên bản: 0.5 · Owner: Product Manager · Cập nhật: 2026-11-21 (ADR-0095: bỏ khái niệm lớp, enrollment, course, learning path, lịch, nhóm học tập — học sinh theo trường/khối; mã giữ nguyên, đánh dấu Removed hoặc viết lại) · 2026-11-21 (ADR-0094: bỏ MFA/mã xác thực khỏi đăng nhập portal — FR-AUTH-008, 013, 029, FR-MOD-003, FR-CMS-011, AC-AUTH-029, 033, 034) · 2026-11-17 (đăng nhập portal bằng form riêng qua backend ADR-0091, danh mục menu/action portal bật/tắt trong DB ADR-0090; đối soát mã AC/TC và phase theo quyết định Technical Architect 2026-11-16; quyết định 2026-11-17: phát hành theo pilot ADR-0073/0080, an toàn ADR-0078, không QR/PIN ADR-0076 Rejected, Windows máy dùng chung)
> Liên quan: [PRODUCT_VISION](PRODUCT_VISION.md) · [USER_PERSONAS](USER_PERSONAS.md) · [GLOSSARY](GLOSSARY.md) · [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) · [DOMAIN_MODEL](../06-architecture/DOMAIN_MODEL.md) · [README](../adr/README.md) · [OPEN_QUESTIONS](../12-project/OPEN_QUESTIONS.md) · [ROADMAP](../12-project/ROADMAP.md) · [TEST_STRATEGY](../12-project/TEST_STRATEGY.md)

## 0. Cách đọc tài liệu

- **Nhãn:** `[Confirmed]` = có trong Master Prompt/Party Addendum ([VIBE_CODING_PROMPT](../../VIBE_CODING_PROMPT.md)) hoặc đã được owner chốt (mọi đề xuất và mọi `[TBD]` được chốt ngày 2026-11-20, [docs/README §3.2](../README.md)); `[Proposed]` = đề xuất mới phát sinh sau ngày đó, chờ owner; `[TBD]` = phụ thuộc Open Question còn mở (hiện không còn).
- **Phase:** `P1`…`P6` theo [ROADMAP](../12-project/ROADMAP.md). P1 Foundation · P2 Core Educational Gameplay · P3 Exam & Learning Management · P4 Multiplayer & Competition · P5 Social & Chat · P6 Production Readiness & Release 1.0. **Release 1.0 = toàn bộ FR thuộc Master Prompt + Party Addendum**, hoàn thành ở cuối giai đoạn Hoàn thiện; mọi FR có nhãn phase P1–P6 đều thuộc phạm vi sản phẩm. Phát hành theo ba giai đoạn **Pilot → Mở rộng → Hoàn thiện** ([ADR-0073](../adr/ADR-0073-phat-hanh-theo-pilot-uu-tien-giao-duc.md), [Confirmed — Technical Architect 2026-11-17]); pilot chỉ đạt khi qua cổng hiệu quả học tập ([ADR-0080](../adr/ADR-0080-do-hieu-qua-hoc-tap-la-exit-criterion-pilot.md)). Phase là thứ tự phát triển; giai đoạn phát hành của từng nhóm chức năng ở [ROADMAP §2](../12-project/ROADMAP.md#2-tổng-quan-giai-đoạn-phase-và-milestone). Không yêu cầu nào bị bỏ, chỉ đổi thứ tự và điểm phát hành.
- **Mã:** theo [docs/README §3.4](../README.md#34-mã-định-danh). **PRD là nguồn duy nhất cấp mã FR và US** ([ADR-0082](../adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md) mục 2): tài liệu domain chỉ dùng đúng mã PRD (bảng "Yêu cầu (mã PRD)") cho phần chi tiết, không có bảng FR/US cục bộ; tiêu chí chi tiết của domain dùng mã `AC-<DOMAIN>-NNN` riêng. Thêm yêu cầu mới → thêm mã mới vào PRD trước, không tái sử dụng mã đã xóa.
- Một yêu cầu chỉ được đánh dấu "Done" khi có test chứng minh (Master Prompt §20, §24) — không chấp nhận giao diện giả lập.
- **Removed (ADR-0072):** yêu cầu gốc của Master Prompt gắn với vai trò giáo viên hoặc hoạt động chấm tay (tự luận, bài nói, project, rubric, hàng đợi chấm) không còn trong Release 1.0 theo [ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md). Mã giữ nguyên để truy vết, không tái sử dụng; chức năng chuyển cho nhà trường (`school_admin`, `academic_manager`) được viết lại ngay trong dòng của mã đó.
- **Removed / viết lại (ADR-0095):** không còn thực thể lớp (Class), enrollment, course, learning path, lịch, nhóm học tập; học sinh thuộc một trường với một khối. Yêu cầu chỉ gắn với các khái niệm đó ghi **Removed (ADR-0095)**; yêu cầu còn ý nghĩa được viết lại theo trường/khối ngay trong dòng của mã đó ("lớp 1–3", "lớp 6–9"… là cách gọi khối, vẫn giữ).
- **Deferred (ADR-0082):** yêu cầu thuộc nội dung không làm trong version này theo [ADR-0082](../adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md) mục 3 — **pháp lý** (rà soát theo luật bảo vệ dữ liệu cá nhân, cơ sở pháp lý xử lý, mô hình consent, DPIA, thời hạn thông báo vi phạm theo luật, quy định game cho người dưới 18 tuổi) và **kỹ năng viết/nói** (`written_response`, `speaking`, `project`, rubric, hàng đợi chấm, AI chấm bài). Mã giữ nguyên, không tái sử dụng, không triển khai; thiết kế (nếu có) ở [docs/99-deferred](../99-deferred/README.md). Nguyên tắc kỹ thuật bảo vệ dữ liệu (tối thiểu hóa, không log PII, định danh giả trong analytics, mã hóa, retention job, soft-delete + ẩn danh hóa, audit, cô lập tenant) vẫn áp dụng.

## 1. Phạm vi

**Trong phạm vi:** game client Godot 4.x 3D, Go backend (microservices theo hệ thống lớn, ADR-0036), Keycloak SSO, APISIX, Valkey, PostgreSQL, tích hợp SIS, Admin Portal (CMS/LMS), School Learning Dashboard cho nhà trường, multiplayer/party/competition, chat an toàn, gamification, observability, triển khai Docker.

**Phạm vi Release 1.0:** game hoàn chỉnh theo Master Prompt + Party Addendum trên Windows, Android, iOS, Web (ADR-0013, [Confirmed — Technical Architect 2026-11-17]; ADR-0079 Rejected), phát hành theo ba giai đoạn ([ADR-0073](../adr/ADR-0073-phat-hanh-theo-pilot-uu-tien-giao-duc.md)). **Chỉ làm khi có quyết định** (không thuộc Master Prompt hoặc chỉ yêu cầu thiết kế mở rộng): parent portal (OQ-13), thanh toán (OQ-14), thi chính thức có proctoring (OQ-18), full LRS endpoint. **Không làm trong Release 1.0:** voice chat, xóa vật lý dữ liệu theo luật ([ADR-0070](../adr/ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md)); vai trò giáo viên ([ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md)). **Ngoài version này** ([ADR-0082](../adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md)): kỹ năng viết/nói (`written_response`, `speaking`, `project`) và pháp lý (rà soát luật bảo vệ dữ liệu, consent, DPIA, quy định game dưới 18 tuổi). Học sinh tự học trong game; nhà trường quản trị ở cấp trường; kiểm duyệt chat tập trung ở Platform Safety Team. Bản sắc sản phẩm và KPI: [PRODUCT_VISION](PRODUCT_VISION.md).

### 1.1. Phạm vi Release 1.0 theo domain

| Hạng mục | Phạm vi Release 1.0 | Tài liệu sở hữu |
|---|---|---|
| Nền tảng client | Windows, Android, iOS, Web — làm đầy đủ cả 4 nền tảng từ pilot (ADR-0013 [Confirmed — Technical Architect 2026-11-17]; ADR-0079 Rejected); preset đồ họa theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) | [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) |
| Đăng nhập & SIS | Keycloak; học sinh đăng nhập **username + password bằng form trong game qua backend**; trên thiết bị cá nhân phiên còn hạn thì mở lại game tự vào sau màn "Tiếp tục là <nickname>?", máy dùng chung (Windows mặc định là máy dùng chung) và Web luôn nhập mật khẩu (ADR-0063, ADR-0068, ADR-0086); không có đăng nhập QR/PIN/mật khẩu hình (ADR-0076 Rejected); Admin Portal có màn đăng nhập riêng, chặn học sinh, phân quyền Role/UserRole (ADR-0066), không chọn tenant trước đăng nhập, username/email duy nhất (ADR-0067); header `X-Tenant-Id` bắt buộc (ADR-0065) ( đăng nhập nhanh và IdP trường là mở rộng sau), provisioning từ SIS và tài khoản tenant chung do platform tạo, SIS qua EduWorld SIS Contract hoặc CSV, reconciliation, chuyển lớp/trường, khóa/vô hiệu hóa, dọn học sinh không đăng nhập | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md), [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md), [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) |
| Thế giới & cốt truyện | 8 vùng đất bao trọn 7 khu chức năng bắt buộc; mạch truyện 7 chương; world map, cổng, khám phá, mở khóa vùng; world event | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| Nhân vật | Tạo avatar, skin, outfit, hair, accessories, 6 emote 3D, animation, preset, equipment slot | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) |
| Progression | XP, level, XP curve cấu hình, danh hiệu, phần thưởng lên cấp; tách 5 loại số liệu | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) |
| Quest | Đủ 12 loại (main, side, daily, weekly, learning, exploration, social, party, challenge, seasonal, special event, boss) | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) |
| Challenge & boss | 11 loại challenge, 5 mức độ khó, boss nhiều phase cá nhân/party | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| Kinh tế | Xu + sổ cái, Shop đồ trang trí, collection, achievement, badge; trading tắt | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) |
| Thi đấu | 8 mode, Glicko-2, tier, season, 13 bảng xếp hạng, tournament (single/double elimination, round robin, team, school, khối — ADR-0095) | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| Học tập | Curriculum đa chương trình/phiên bản; 6 loại activity server tự chấm (`quiz`, `interactive_exercise`, `minigame`, `video_lesson`, `audio_lesson`, `simulation`; ngoài Release: `written_response`, `speaking`, `project` — ADR-0072); 11 loại câu hỏi; gợi ý nhiều cấp và gợi ý ôn tập tự động; exam đủ hình thức; mastery; công thức MathJax/MathLive; nội dung 4 môn lớp 1–9 theo OQ-08 | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md), [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) |
| Multiplayer | Instance, party (Addendum đầy đủ), lobby, co-op quest/boss, reconnect, đổi thiết bị; matchmaking 8 chế độ | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md), [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md), [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) |
| Social & chat | Bạn bè, club/guild, 8 loại channel, chat luôn có trong trận/party, đủ tính năng nhắn tin, kiểm duyệt + danh mục từ nhạy cảm + công tắc, moderation | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md), [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [MODERATION](../03-social-multiplayer/MODERATION.md) |
| Admin Portal | Một portal 5 workspace: School & LMS, Learning Content Studio, Game Operations Console, Game Live Operations, Platform Administration; Draft/Publish/rollback, gán/clone dữ liệu, thùng rác; School Learning Dashboard; moderation tập trung cho Safety Team | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md), [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| Kinh doanh | Không in-app purchase [Confirmed, OQ-14] | — |


## 2. Functional Requirements

### 2.1. Identity & Authentication — `AUTH` (Master Prompt §10.1–10.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-AUTH-001 | Học sinh dùng tài khoản do nhà trường cấp; không có màn hình tự đăng ký trong game | [Confirmed] | P1 |
| FR-AUTH-002 | Godot đăng nhập bằng **form username/password trong game**, gửi qua HTTPS tới `POST /api/v1/auth/game/login`; module identity xác thực với Keycloak bằng client confidential `eduworld-game-backend` (Direct Access Grants); game không có client secret, không mở trình duyệt/WebView (ADR-0068, thay ADR-0007 cho game). Portal giữ Authorization Code + PKCE trên trình duyệt | [Confirmed — người duyệt 2026-11-15] | P1 |
| FR-AUTH-003 | EduWorld không lưu mật khẩu học sinh; xác thực ở Keycloak/IdP. Mật khẩu đi qua game và module identity (ADR-0068) phải: chỉ qua TLS, không log/trace/telemetry body đăng nhập ở APISIX, backend, client; xóa khỏi bộ nhớ sau khi gọi Keycloak (best effort) | [Confirmed] | P1 |
| FR-AUTH-004 | Backend xác minh token (chữ ký JWKS, `iss`, `aud=eduworld-api`, `exp`) độc lập với APISIX (ADR-0008, ADR-0009) | [Confirmed] | P1 |
| FR-AUTH-005 | Định danh ngoài bằng `(issuer, subject)`; không dùng email/tên hiển thị làm khóa | [Confirmed] | P1 |
| FR-AUTH-006 | Just-in-Time provisioning khi đăng nhập lần đầu qua IdP trường, chỉ khi khớp bản ghi SIS — **ngoài Release 1.0** (ADR-0063: Release 1.0 tài khoản chỉ đến từ SIS provisioning hoặc do platform tạo trong tenant chung) | Mở rộng sau | — |
| FR-AUTH-007 | `POST /api/v1/auth/game/login` (game) trả bootstrap: `tenant_id` của phiên, profile, school, grade, feature flags, policies áp dụng. Admin Portal không dùng bootstrap người chơi: sau `POST /api/v1/auth/portal/login` portal gọi `GET /me/tenants`, `GET /me/permissions` (ADR-0067, ADR-0091) | [Confirmed] | P1 |
| FR-AUTH-008 | **Giữ phiên game bằng refresh token** (ADR-0086, thay một phần ADR-0064): sau đăng nhập, game lưu refresh token trong kho bảo mật của hệ điều hành trên **thiết bị cá nhân** (Windows DPAPI, Android Keystore, iOS Keychain); access token và mật khẩu không bao giờ lưu xuống đĩa. Mở lại game khi phiên còn hạn → `POST /auth/game/token:refresh` kèm `device_id` rồi vào thẳng game, **không** hiện form; phiên hết hạn/bị thu hồi/sai thiết bị → xóa token, hiện form đăng nhập (ô tên đăng nhập điền sẵn nếu bật "Nhớ tên đăng nhập"). **Web** và máy `shared_device = true` chỉ giữ token trong RAM: mở lại phải nhập mật khẩu. **Windows mặc định là máy dùng chung** (`shared_device_default`: không lưu refresh token) trừ khi bộ cài được chọn "máy cá nhân" [Confirmed — Technical Architect 2026-11-17]. **Mọi nền tảng**, khi tự vào lại bằng refresh token, hiện màn "Tiếp tục là <nickname>?" có nút "Không phải tôi" (đăng xuất, xóa token) trước khi tạo game session [Confirmed — Technical Architect 2026-11-17]. Access token TTL 5 phút; phiên game (client `eduworld-game-backend`) idle 7 ngày / tối đa 30 ngày (`game_session_sso`), không offline token; refresh token xoay vòng mỗi lần dùng, dùng lại token cũ → thu hồi cả phiên; gắn `device_id`; thu hồi khi đăng xuất, takeover, đổi/đặt lại mật khẩu, khóa/xử phạt, thu hồi quyền, dọn học sinh. Trong lúc chơi refresh tự động và reconnect không hỏi mật khẩu. Portal giữ chính sách riêng: idle 30 phút / tối đa 10 giờ, phiên `platform_staff` tối đa 8 giờ từ lúc nhập mật khẩu (không MFA — ADR-0094), tải lại trang không đăng nhập lại khi phiên còn hạn | [Confirmed] chính sách (ADR-0086) / [Confirmed] Web chỉ RAM | P1 |
| FR-AUTH-009 | Đăng xuất chủ động (nút "Đăng xuất" trong game, ADR-0086 mục 6): kết thúc game session (`:end`) → `POST /api/v1/auth/game/logout` — module identity thu hồi refresh token + phiên Keycloak → game xóa token khỏi kho bảo mật và RAM, đóng WS. Thoát game trên thiết bị cá nhân chỉ kết thúc game session, giữ phiên đăng nhập; trên máy dùng chung thoát game = đăng xuất | [Confirmed] | P1 |
| FR-AUTH-010 | Tài khoản bị khóa/vô hiệu hóa: từ chối request REST, cắt kết nối realtime qua event `user.access_revoked` | [Confirmed] | P1 |
| FR-AUTH-011 | Realtime ticket dùng 1 lần, TTL 60 giây để mở WebSocket `/realtime/v1` (ADR-0008) | [Confirmed] | P1 |
| FR-AUTH-012 | Cấu hình realm/client/issuer qua environment, không hardcode | [Confirmed] | P1 |
| FR-AUTH-013 | Keycloak clients: `eduworld-game-backend` (confidential, chỉ Direct Access Grants, secret chỉ ở module identity, chỉ realm role `student` — ADR-0068; thay client public `eduworld-game`), `eduworld-portal` (Admin Portal: confidential, tắt Standard Flow, chỉ Direct Access Grants qua module identity, secret chỉ ở module identity, direct grant flow chỉ cho `tenant_staff`/`platform_staff`, không OTP — ADR-0027 mục 7, ADR-0066, ADR-0091, ADR-0094), `eduworld-api`; thêm service account cho worker provisioning | [Confirmed] / service account [Confirmed] | P1 |
| FR-AUTH-014 | Hỗ trợ nhiều issuer (multi-realm sau này) nhờ khóa `(issuer, subject)` | [Confirmed] | P1 |
| FR-AUTH-015 | Device Authorization Grant cho máy dùng chung trong lớp — **ngoài Release 1.0** (ADR-0063) | Mở rộng sau | — |
| FR-AUTH-016 | Authorization theo **Permission** (trong code) → **Role** (DB, `identity.roles`) → **UserRole** (DB, `identity.user_roles`: scope platform/tenant/school/subject, `valid_from/to`, `source` sis/manual), kiểm tra ở mọi use case; không có scope `class`/`class × subject` theo phân công (ADR-0072); role tenant không chứa permission `platform.*`/`game.*`/`competition.config.*`; đổi role/UserRole phát `role.updated`/`user_role.changed` (ADR-0066) | [Confirmed] | P1 |
| FR-AUTH-017 | **Đăng nhập username + password, không chọn trường** (ADR-0063, ADR-0068): form đăng nhập vẽ trong game gồm logo, chọn ngôn ngữ, username, password (nút hiện/ẩn), nút "Đăng nhập", "Nhớ tên đăng nhập", "Quên mật khẩu? Liên hệ nhà trường để đặt lại"; không có bước chọn trường, ô mã trường hay cách đăng nhập nhanh. Sau xác thực backend xác định tenant theo membership: 1 tenant (SIS hoặc tenant chung) → vào thẳng; nhiều → `403 TENANT_NOT_SELECTED` + `login_ticket` (2 phút, một lần), chọn **sau** đăng nhập bằng `POST /auth/game/tenant:select` (không nhập lại mật khẩu); không thuộc tenant chung và không thuộc tenant SIS nào → `403 ACCOUNT_NOT_PROVISIONED`, không cho đăng nhập, audit, thu hồi phiên Keycloak | [Confirmed — người duyệt 2026-11-15] | P1 |
| FR-AUTH-018 | Backend xác định và kiểm tra chéo tenant khi tạo phiên và ở mọi request theo membership trong DB. **Mọi request tenant-scoped mang header `X-Tenant-Id` bắt buộc** (ADR-0065): thiếu → `400 TENANT_HEADER_REQUIRED` (APISIX + `core-api`); header phải khớp tenant của phiên (membership DB, claim nếu có, game session) → không thì `403 TENANT_MISMATCH` + audit; ngoại lệ không cần header: `POST /auth/game/login`, `/auth/game/tenant:select`, `/auth/session`, `GET /me/tenants`, `GET /game/client-config`, health. Có claim → tenant phải thuộc membership (không → `TENANT_MISMATCH` + audit/security event; tenant tắt → `TENANT_ACCESS_DISABLED`); không claim → 1 membership: dùng; nhiều: `TENANT_NOT_SELECTED` kèm danh sách; 0: `ACCOUNT_NOT_PROVISIONED` (ADR-0063) | [Confirmed] | P1 |
| FR-AUTH-019 | Mỗi phiên có đúng một tenant; người thuộc nhiều tenant đổi tenant bằng header `X-Tenant-Id` mới được kiểm tra với membership (portal không xin token mới — ADR-0091; không nhập lại mật khẩu); WebSocket giữ tenant cố định từ lúc cấp ticket; username/email không dùng để suy ra tenant. System Admin: ghi dữ liệu tenant gửi `X-Tenant-Id` = tenant đích, đọc liên tenant dùng `tenant_ids` (ADR-0065) | [Confirmed] | P1 |
| FR-AUTH-020 | Trang đăng nhập Keycloak hiển thị tên/logo và IdP của tenant được gợi ý trước đăng nhập — **ngoài Release 1.0** (ADR-0063: không có gợi ý/chọn tenant trước đăng nhập; trang dùng theme `eduworld` chung) | Mở rộng sau | — |
| FR-AUTH-021 | **Tenant chung**: tài khoản học sinh trong tenant chung `common` chỉ do platform (System Admin) tạo/import có chủ đích, kèm School ảo và khối (không có lớp ảo — ADR-0095); bootstrap báo `is_common_tenant`. **Không** tự đưa người dùng không khớp tenant nào vào tenant chung (bỏ JIT, ADR-0063) | [Confirmed] (người duyệt) / chi tiết [Confirmed] | P1 |
| FR-AUTH-022 | **Chỉ học sinh được đăng nhập game**: đăng nhập qua `/auth/game/login` (client `eduworld-game-backend`) của nhân sự (`user_type = staff`, gồm nhân sự trường và nhân sự platform — ADR-0081) bị từ chối `403 GAME_ACCESS_STUDENT_ONLY` (backend bắt buộc; Keycloak flow điều kiện role `student` là lớp bổ sung); vận hành xem trải nghiệm qua preview trong CMS (ADR-0031) | [Confirmed] | P1 |
| FR-AUTH-023 | **Một thiết bị cho mỗi tài khoản học sinh**: tối đa 1 game session active; đăng nhập thiết bị khác → hộp thoại xác nhận trên thiết bị mới; đồng ý thì thiết bị cũ tự đăng xuất và hiển thị thông báo; Keycloak session của thiết bị cũ bị thu hồi (ADR-0031) | [Confirmed] | P1 |
| FR-AUTH-024 | **Dọn học sinh không đăng nhập game**: worker soft-delete học sinh có `last_login_at` (hoặc `provisioned_at` nếu chưa từng) + `inactive_student_cleanup_days` < hiện tại; số ngày cấu hình ở platform (mặc định 180 [Confirmed]) và tenant; báo trước, loại trừ, khôi phục khi đăng nhập lại, xác nhận khi > 10% (ADR-0032) | [Confirmed] cơ chế / [Confirmed] tham số | P2 |
| FR-AUTH-025 | **Không làm** mã QR lớp + PIN 4 số, thẻ QR cá nhân, mật khẩu hình ảnh cho lớp 1–3 (ADR-0076 Rejected 2026-11-17; đăng nhập chỉ username/password + giữ phiên bằng refresh token, FR-AUTH-008). **Ngoài Release 1.0** (ADR-0063): SSO IdP trường qua tên miền email, ô "nhập mã trường"; giữ thiết kế ở [KEYCLOAK_SSO §8.1](../09-identity/KEYCLOAK_SSO.md#81-mở-rộng-sau--ngoài-release-10-adr-0063), bật bằng ADR mới | Mở rộng sau | — |
| FR-AUTH-026 | ~~Phiên game dài trên thiết bị~~ — gộp vào FR-AUTH-008 theo ADR-0086 (refresh token trong kho bảo mật của thiết bị cá nhân, idle 7 ngày / tối đa 30 ngày, không offline token) | Gộp vào FR-AUTH-008 | — |
| FR-AUTH-027 | Hàng chờ đăng nhập: khi vượt ngưỡng toàn cục, `POST /auth/game/login` trả `429 LOGIN_QUEUE_BUSY` + `Retry-After`, game hiển thị "Đang vào lớp…" và tự thử lại có jitter (vẫn thuộc Release 1.0 — ADR-0064 mục 6, ADR-0068 mục 5) | [Confirmed] | P4 |
| FR-AUTH-028 | **Bảo vệ đăng nhập game qua backend** (ADR-0068): rate limit 10 lần/phút theo thiết bị và 10 lần/phút theo username (`429 RATE_LIMITED`); Keycloak brute-force khóa tạm sau 5 lần sai (1–15 phút); một thông điệp lỗi chung `401 INVALID_CREDENTIALS` cho sai mật khẩu/không có tài khoản/khóa tạm; chặn client cũ hơn `min_client_version` (`426 CLIENT_VERSION_UNSUPPORTED`); certificate pinning [Confirmed] | [Confirmed] / tham số [Confirmed] | P1 |
| FR-AUTH-029 | **Admin Portal có màn đăng nhập riêng do portal tự vẽ, qua backend** (ADR-0091, thay luồng chuyển hướng OIDC của ADR-0066 mục 1): form `/login` gồm username hoặc email, mật khẩu, chọn ngôn ngữ, "Quên mật khẩu?" — **không có mã xác thực/MFA với bất kỳ tài khoản nào, kể cả role platform** (ADR-0094, rủi ro owner chấp nhận); gửi **một request** `POST /api/v1/auth/portal/login`, module identity xác thực với Keycloak (client confidential `eduworld-portal`, Direct Access Grant); mọi lỗi xác thực trả một thông điệp chung `401 INVALID_CREDENTIALS`; tài khoản còn bước thiết lập → `403 ACCOUNT_SETUP_REQUIRED` + `setup_ticket` (đổi mật khẩu tạm trên portal); quên/đặt lại mật khẩu do portal xử lý (luôn `202`, token dùng một lần có hạn, chỉ nhân sự); access token chỉ trong bộ nhớ, refresh token trong cookie `HttpOnly; Secure; SameSite=Strict` có CSRF double-submit; rate limit theo username, thiết bị và toàn cục (không theo IP); không log body; **học sinh không đăng nhập được portal**: chặn ở Keycloak (chỉ realm role `tenant_staff`/`platform_staff`) và ở backend (`azp = eduworld-portal` + `user_type = staff` — ADR-0081, nếu không `403 PORTAL_ACCESS_STAFF_ONLY` + audit) (ADR-0066) | [Confirmed — người duyệt 2026-11-15] (sửa theo ADR-0091, ADR-0094) | P1 |
| FR-AUTH-030 | **Portal không chọn tenant trước đăng nhập** (ADR-0067): role platform → phạm vi platform; 1 tenant → vào thẳng; nhiều tenant → chọn sau đăng nhập (`GET /me/tenants`), đổi ở bộ chọn trên topbar bằng header `X-Tenant-Id` mới được kiểm tra với membership (ADR-0065), **không** xin token mới và không dùng scope `organization:{alias}` (ADR-0091 mục 4); không còn UserRole portal hiệu lực và không có role platform → `403 PORTAL_ACCESS_STAFF_ONLY`. Không có URL theo tenant | [Confirmed — người duyệt 2026-11-15] (sửa theo ADR-0091) | P1 |
| FR-AUTH-031 | **Username và email không trùng trong cùng tenant; trên SSO một người một tài khoản** (username/email duy nhất trên Keycloak, không phân biệt hoa thường, sau chuẩn hóa trim/NFC/chữ thường) cho học sinh và nhân sự (ADR-0067, [ADR-0069](../adr/ADR-0069-username-email-duy-nhat-theo-tenant-va-dang-nhap-khong-chon-tenant.md)); đăng nhập SSO xác thực trước, module identity kiểm tra tenant sau; trùng với tài khoản tenant khác mà là cùng một người → thêm membership; nhân sự bắt buộc có email; một người một tài khoản, nhiều membership/UserRole; tạo/sửa trùng → `409 USERNAME_TAKEN`/`409 EMAIL_TAKEN`; SIS/import trùng → không tạo, `reconciliation_issue` `duplicate_identifier`; soft-delete không giải phóng định danh đến khi ẩn danh hóa [Confirmed] | [Confirmed — người duyệt 2026-11-15] | P1 |
| FR-AUTH-032 | **Quản lý Role/UserRole trên portal** (ADR-0066): Tenant Admin quản lý role tùy biến [Confirmed] và UserRole trong tenant (`tenant.role.write`, `tenant.user_role.write`); School Admin gán UserRole trong trường; System Admin quản lý role platform và mọi tenant (`platform.role.write`, `platform.user_role.write`); role hệ thống chỉ đọc; nhật ký phân quyền | [Confirmed] / role tùy biến [Confirmed] | P3 |

### 2.2. Organization & Multi-tenancy — `ORG` (§2.2, §9, §10.3)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-ORG-001 | Phân cấp Platform → Tenant → School; học sinh thuộc trường với một khối (`grade_level` 1–9), không có thực thể lớp (ADR-0095). Tenant là **một trường độc lập** (`school`), **một đơn vị nhiều trường** (`unit`) hoặc **tenant chung** (`common`); tenant là đơn vị cô lập dữ liệu (ADR-0005) | [Confirmed] | P1 |
| FR-ORG-002 | Quản lý AcademicYear của trường và khối (`grade_level` 1–9) của học sinh; không có Class, không có Enrollment; khối tại thời điểm sự kiện được ghi trên dữ liệu học tập để giữ lịch sử, thay đổi trường/khối có audit (ADR-0095) | [Confirmed] (viết lại theo ADR-0095) | P1 |
| FR-ORG-003 | Phân biệt Keycloak User, EduWorld User, Student, Staff (nhân sự nhà trường/platform), School, External Identity; không có thực thể giáo viên (ADR-0072), không có Class/Enrollment (ADR-0095) | [Confirmed] (viết lại theo ADR-0072, ADR-0095) | P1 |
| FR-ORG-004 | **Removed (ADR-0072)** — phân công giảng dạy lớp × môn không còn; phạm vi nhân sự là trường hoặc môn qua UserRole (FR-AUTH-016) | Removed | — |
| FR-ORG-005 | Chuyển trường và lên khối: cập nhật trường/khối của học sinh trong một transaction, audit, phát event; giữ lịch sử học tập gắn trường/khối cũ theo policy; không còn chuyển lớp (ADR-0095) | [Confirmed] (viết lại theo ADR-0095) | P1 |
| FR-ORG-006 | Mọi bảng nghiệp vụ có `tenant_id`; **mọi truy vấn phải lọc theo `tenant_id`**, enforcement ở app layer (TenantScope) + RLS; `school_id` là chiều bên trong tenant | [Confirmed] | P1 |
| FR-ORG-007 | School settings: chính sách chat (gồm preset `quick_message_emote_only` cho cả trường — ADR-0078), giờ truy cập và khung giới nghiêm của trường (chỉ siết thêm so với `platform_curfew`), cho phép liên trường, JIT provisioning (`/api/v1/schools`) | [Confirmed] · preset, giới nghiêm [Confirmed — Technical Architect 2026-11-17] | P1–P5 |
| FR-ORG-008 | Tại một thời điểm người dùng chỉ làm việc trên **một** tenant. Chỉ System Admin thấy mọi tenant; mỗi màn hình có dữ liệu theo tenant có **dropdown tìm kiếm tenant** để lọc; mọi truy cập liên tenant được audit (ADR-0027 §6) | [Confirmed] | P1 (API) / P3 (CMS) |
| FR-ORG-009 | Mỗi **Tenant** là một Keycloak Organization (`platform.tenants.keycloak_org_id`, `keycloak_org_alias`, `login_code`); tenant `unit` có một Organization cho mọi trường trong đơn vị (ADR-0026) | [Confirmed] | P1 |
| FR-ORG-010 | Membership Keycloak Organization đồng bộ tự động: học sinh thuộc đúng một tenant; nhân sự thuộc các tenant có UserRole hiệu lực; chuyển sang tenant khác gỡ membership cũ và thu hồi phiên cũ; chuyển trường trong cùng tenant `unit` không đổi membership; reconciliation phát hiện member thêm tay | [Confirmed] | P1 |
| FR-ORG-011 | **Chỉ soft-delete**: ứng dụng không hard-delete; "xóa" đặt `deleted_at`/`deleted_by`; mọi truy vấn mặc định không trả dữ liệu đã soft-delete; role DB ứng dụng không có quyền `DELETE`; khôi phục qua Thùng rác (ADR-0028) | [Confirmed] | P1 |
| FR-ORG-012 | System Admin tạo dữ liệu dùng chung (platform) và **gán cho các tenant** muốn sử dụng; tenant được gán chỉ đọc (ADR-0029) | [Confirmed] | P1 (model) / P3 (CMS) |
| FR-ORG-013 | System Admin **clone dữ liệu** (nội dung, cấu hình) từ tenant này sang tenant khác; bản sao có ID mới và nguồn gốc `cloned_from_*`; không bao giờ clone dữ liệu học sinh (ADR-0029) | [Confirmed] | P3 |
| FR-ORG-014 | Nghĩa vụ xóa dữ liệu theo luật được đáp ứng bằng ẩn danh hóa; Release 1.0 không xóa vật lý (ngoại lệ duy nhất: purge chat gameplay ADR-0033) | [Confirmed] ADR-0070 | P5 |
| FR-ORG-015 | **Danh mục quốc gia** (ISO 3166-1) và **danh mục tỉnh/thành** theo quốc gia, có hiệu lực và kế thừa đơn vị cũ → mới; Việt Nam seed 34 đơn vị hiện hành + 63 đơn vị cũ; mã chính thức đối chiếu văn bản trước khi seed production (ADR-0060) | [Confirmed] danh mục, 34 đơn vị; [Confirmed — owner 2026-11-20] mã chính thức nhập từ danh mục mã đơn vị hành chính do cơ quan nhà nước ban hành trước khi seed production, không tự đặt | P1 |
| FR-ORG-016 | **Danh mục trường**: mỗi trường thuộc một tenant và một tỉnh/thành, có mã ngành (nếu có), cấp học, khối áp dụng, tên song ngữ; quản trị trong Admin Portal (System Admin liên tenant, Tenant Admin trong tenant), import CSV (ADR-0060) | [Confirmed] | P1 |
| FR-ORG-017 | **Học sinh thuộc đúng một trường với một khối** tại một thời điểm (hồ sơ học sinh hiệu lực, không enrollment); học sinh SIS thiếu trường/khối hợp lệ không vào game (`NO_ACTIVE_ENROLLMENT`, giữ mã) và tạo issue cho trường; tenant chung dùng School ảo + khối (ADR-0060, ADR-0095) | [Confirmed] (viết lại theo ADR-0095) | P1 |
| FR-ORG-018 | **Đồng bộ SIS khi đăng nhập**: dữ liệu đủ & mới → không gọi SIS; thiếu trường/khối → tra cứu SIS chặn ≤ 3 s; dữ liệu cũ hơn 24 giờ → làm mới nền; lan dữ liệu trường/khối/học sinh sang các module game để hiển thị (ADR-0061) | [Confirmed] yêu cầu; tham số [Confirmed] | P1 |

### 2.3. SIS Integration — `SIS` (§10.5)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-SIS-001 | EduWorld sở hữu DTO/API contract SIS ([EduWorld SIS Contract v1](../../specs/sis/eduworld-sis-contract.v1.yaml)); mỗi trường cung cấp API theo contract; không adapter theo DTO của SIS (ADR-0021) | [Confirmed] | P1 |
| FR-SIS-002 | Đồng bộ School, AcademicYear, Student (trường + khối), Account Status; không đồng bộ giáo viên, phân công giảng dạy (ADR-0072), lớp và enrollment (SIS contract 1.3.0, ADR-0095) | [Confirmed] (viết lại theo ADR-0072, ADR-0095) | P1 |
| FR-SIS-003 | Hai cách nhận dữ liệu: `contract_v1` (pull HTTP theo contract) và `csv_import` (cột CSV = tên trường DTO) cho trường chưa có API; dữ liệu vi phạm contract bị từ chối theo bản ghi | [Confirmed] hướng / [Confirmed] chi tiết | P1 |
| FR-SIS-009 | Bộ conformance test + mock server sinh từ contract để trường tự kiểm tra facade trước khi kết nối | [Confirmed] | P1 |
| FR-SIS-004 | Full sync định kỳ + delta sync + webhook (khi trường khai báo capability `delta`/`webhook` trong contract) | [Confirmed] | P1–P3 |
| FR-SIS-005 | Reconciliation: phát hiện bản ghi trùng/thiếu/xung đột, tạo ReconciliationIssue cho người duyệt | [Confirmed] | P1 |
| FR-SIS-006 | Sync error handling, retry có backoff, audit mọi lần chạy (SyncRun) | [Confirmed] | P1 |
| FR-SIS-007 | Godot không gọi SIS; SIS credential không lộ qua public gateway (`/api/v1/sis` chỉ gateway nội bộ) | [Confirmed] | P1 |
| FR-SIS-008 | Account deactivation/lock từ SIS lan truyền tới EduWorld trong ≤ 1 chu kỳ sync | [Confirmed] | P1 |

### 2.4. Curriculum — `CUR` (§4.1)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CUR-001 | Cấu trúc Curriculum → Grade → Subject → Topic → Learning Objective → Learning Unit → Lesson → Learning Activity → Question/Exercise/Assessment | [Confirmed] | P2 |
| FR-CUR-002 | Hỗ trợ nhiều chương trình và phiên bản chương trình; không hardcode một chương trình | [Confirmed] | P2 |
| FR-CUR-003 | Learning Objective có mã ổn định (vd. `MATH.G3.NUM.MUL_TABLE`) | [Confirmed] | P2 |
| FR-CUR-004 | Seed khung GDPT 2018 môn Toán lớp 1–5 cho pilot | [Confirmed] OQ-08 | P2 |
| FR-CUR-005 | CMS quản lý cấu trúc chương trình (CRUD + workflow) | [Confirmed] | P3 |
| FR-CUR-006 | API duyệt curriculum cho client: `/api/v1/learning/curriculum` | [Confirmed] | P2 |
| FR-CUR-007 | Đồ thị LO tiên quyết (`prerequisite_ids[]`, DAG) dùng cho gợi ý ôn tập và bài chẩn đoán; phát hiện chu trình khi lưu ([CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)) | [Confirmed] | P2 |
| FR-CUR-008 | Trường chọn curriculum version mặc định cho mỗi năm học (`school.settings.curriculum_version_id`); nhiều version tồn tại song song theo khối ([CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)) | [Confirmed] | P3 |
| FR-CUR-009 | LO bám GDPT 2018: thêm `yccd_ref` (mã yêu cầu cần đạt, ≤ 64 ký tự) và `competency_ids[]` (năng lực/phẩm chất); seed được EdTech review theo văn bản chương trình ([CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)) | [Confirmed] | P2 |
| FR-CUR-010 | Import/export curriculum dạng YAML/CSV qua use case CMS (có validate, `Idempotency-Key`, audit; seed cũng đi qua import, không SQL thủ công) ([CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)) | [Confirmed] | P2 |
| FR-CUR-011 | Khi publish CurriculumVersion mới: bảng mapping LO giữa hai version (`same`, `split`, `merge`, `replaces`, `removed`) và chiếu mastery sang version mới; learning event gốc không bị sửa ([CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)) | [Confirmed] | P3 |
| FR-CUR-012 | Mapping bộ sách giáo khoa → LO (bộ sách, tập, bài, n–n với LO) để lọc nội dung theo bài sách và chọn nội dung giao bài (không còn lịch LMS — ADR-0095); không sao chép nguyên văn sách khi chưa có giấy phép ([CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)); chỉ lưu metadata cho bộ sách trường pilot đang dùng | [Confirmed — owner 2026-11-20, OQ-08] | P3 |

### 2.5. Learning Content — `CNT` (§4.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CNT-001 | Loại nội dung: Lesson, Learning Material, Video, Image, Audio, Interactive Content, Question, Exercise, Question Set, Assessment, Game Activity | [Confirmed] | P2–P3 |
| FR-CNT-002 | Metadata bắt buộc: grade, subject, topic, LO, difficulty, version, status, author, reviewer, published_at | [Confirmed] | P2 |
| FR-CNT-003 | Trạng thái Draft → Review → Approved → Published → Archived (+ Rejected → Draft) | [Confirmed] / Rejected [Confirmed] | P2 |
| FR-CNT-004 | Published version bất biến; attempt tham chiếu `content_version_id` (ADR-0015) | [Confirmed] | P2 |
| FR-CNT-005 | Nội dung thuộc platform (`tenant_id` NULL, tenant thấy khi được gán) hoặc thuộc một tenant; chia sẻ giữa tenant bằng gán/clone của System Admin (ADR-0029) | [Confirmed] | P2 |
| FR-CNT-006 | Media lưu object storage S3-compatible + CDN (không lưu file trong DB); pipeline quét virus, kiểm tra magic bytes, xóa EXIF, chuyển đổi nhiều biến thể; khóa content-addressed bất biến (ADR-0044) | [Confirmed] | P2 |
| FR-CNT-007 | Import nội dung hàng loạt (JSON/CSV theo `specs/schemas`) cho seed pilot trước khi có CMS UI | [Confirmed] | P2 |
| FR-CNT-008 | Báo cáo "nội dung đang được dùng ở đâu" (quest, bài giao, đề, challenge tham chiếu) trước khi archive ([CONTENT_MODEL](../02-learning/CONTENT_MODEL.md)) | [Confirmed] | P3 |
| FR-CNT-009 | Draft/Publish cho mọi nội dung có version; release pointer; publish ngay hoặc hẹn giờ; rollback = dùng lại version đã publish (đổi con trỏ, lý do bắt buộc); unpublish; lịch sử release (ADR-0043) | [Confirmed] | P2 |
| FR-CNT-010 | Xem nội dung từng version đã publish (chỉ đọc), so sánh hai version, preview trong game | [Confirmed] | P2–P3 |
| FR-CNT-011 | Version pinning: học sinh đang ở màn/quest/bài/attempt/trận giữ version cũ; lượt/màn mới nhận version mới nhất; exam session và bài giao ghim version | [Confirmed] | P2 |
| FR-CNT-012 | Publish đẩy snapshot bất biến lên Valkey/CDN; content manifest theo tenant (ETag/304); câu hỏi đề và answer key không lên CDN; học sinh nhận bản mới ≤ 30 giây sau publish [Confirmed] | [Confirmed] | P2 |
| FR-CNT-013 | Activity Registry: mọi loại hoạt động học tập trong Release (`quiz`, `interactive_exercise`, `minigame`, `video_lesson`, `audio_lesson`, `simulation`) đi qua interface plugin chung; `written_response`, `speaking`, `project` ngoài Release (ADR-0072); quest/assignment/exam/challenge chỉ tham chiếu `activity_id` + version ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), ADR-0048) | [Confirmed] | P2 |
| FR-CNT-014 | Video bài học: chapter, phụ đề WebVTT vi/en, resume, câu hỏi chèn tại mốc thời gian, nhánh tương tác, hoàn thành theo khoảng đã xem thật ([VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md)) | [Confirmed] tính năng / [Confirmed] tham số | P4 |
| FR-CNT-015 | Bài nghe (`audio_lesson`): nghe hiểu có câu hỏi tự chấm ([AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md)). Bài nói (`speaking`) **Removed (ADR-0072) · Deferred (ADR-0082)**; không có AI giọng nói trong Release 1.0 (ADR-0059) | [Confirmed] nghe / Deferred (ADR-0082) nói | P4 |
| FR-CNT-016 | Mô phỏng: template Godot đóng gói, CMS chỉ tham số hóa, server kiểm chứng xác định bằng checker Go ([SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md), ADR-0052) | [Confirmed] | P4 |
| FR-CNT-017 | Công thức Toán/Lý/Hóa: soạn bằng MathLive, hiển thị MathJax 4 trên portal, SVG render sẵn trong game; game chỉ hiển thị, không nhập công thức (ADR-0055) | [Confirmed] | P2 |
| FR-CNT-018 | Ngôn ngữ nội dung (ADR-0056): nội dung game bắt buộc đủ `vi` + `en`; nội dung học tập có `content_language` + bản dịch tùy chọn; chặn submit review/publish khi thiếu bản bắt buộc (`TRANSLATION_INCOMPLETE`); theo dõi `translation_status` ([CONTENT_MODEL](../02-learning/CONTENT_MODEL.md)) | [Confirmed — ADR-0056] | P2 |
| FR-CNT-019 | Nội dung cho nhóm `g1_3` (lớp 1–3): lồng tiếng tiếng Việt bắt buộc cho đề bài và hướng dẫn quest, là điều kiện publish; khi không có tai nghe hiển thị phụ đề + icon ([CONTENT_MODEL](../02-learning/CONTENT_MODEL.md)) | [Confirmed] | P2 |
| FR-CNT-020 | Khối nội dung `worked_example` (lời giải mẫu từng bước) trong lesson/learning material, gợi ý xem lại sau khi sai ([CONTENT_MODEL](../02-learning/CONTENT_MODEL.md)) | [Confirmed] | P2 |
| FR-CNT-021 | Tìm kiếm nội dung trong CMS theo metadata (khối, môn, chủ đề, LO, độ khó, trạng thái) và full-text tiếng Việt không phân biệt dấu (`unaccent`) ([CONTENT_MODEL](../02-learning/CONTENT_MODEL.md)) | [Confirmed] | P3 |

### 2.6. Question Bank — `QB` (§4.3)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-QB-001 | 11 loại: Single Choice, Multiple Choice, True/False, Fill in the Blank, Short Answer, Numeric Answer, Matching, Ordering, Drag and Drop, Image-based, Interactive | [Confirmed] | P2 (6 loại đầu) – P3 (đủ) |
| FR-QB-002 | Thuộc tính: ID, stem, render data, options, answer key, explanation, difficulty, grade, subject, topic, LO, scoring rule, version | [Confirmed] | P2 |
| FR-QB-003 | Answer key và scoring rule không bao giờ gửi về client (ADR-0016) | [Confirmed] | P2 |
| FR-QB-004 | Random câu hỏi theo điều kiện (grade/topic/LO/difficulty/loại trừ câu đã làm gần đây) | [Confirmed] | P2 |
| FR-QB-005 | Tạo đề từ Question Bank theo Assessment Blueprint | [Confirmed] | P3 |
| FR-QB-006 | Chấm Short Answer bằng so khớp chuẩn hóa (bỏ dấu cách thừa, không phân biệt hoa thường, danh sách đáp án chấp nhận); Numeric có dung sai | [Confirmed] | P2 |
| FR-QB-007 | Nội dung câu hỏi hỗ trợ tiếng Việt có dấu, công thức toán đơn giản, hình ảnh, audio (môn Tiếng Anh) | [Confirmed] | P2 |
| FR-QB-008 | Câu hỏi tham số hóa: `render_payload.params` có ràng buộc, server sinh giá trị theo seed của attempt, lưu trong `attempt_items.presentation.params`, render stem và chấm theo giá trị đó; biểu thức chỉ qua evaluator an toàn ([QUESTION_BANK](../02-learning/QUESTION_BANK.md)) | [Confirmed] | P3 |
| FR-QB-009 | Interactive validator registry: mỗi widget câu `interactive` có validator Go thuần đăng ký trong registry, server replay `action_log` để kiểm chứng; không chạy script do người soạn viết ([QUESTION_BANK](../02-learning/QUESTION_BANK.md)) | [Confirmed] | P3 |
| FR-QB-010 | Item analysis: worker tính `p_value`, `discrimination`, `distractor_rate`, thời gian, tỉ lệ `rapid_guess`/gợi ý từ lần trả lời đầu có tính điểm (đã loại `rapid_guess`), hiển thị trong CMS, không tự sửa version đã publish ([QUESTION_BANK](../02-learning/QUESTION_BANK.md)) | [Confirmed] | P3 |
| FR-QB-011 | Import câu hỏi hàng loạt từ Excel/CSV/QTI-lite có validate vào trạng thái Draft; import Word/MathType theo [CONTENT_PRODUCTION_PLAN](../02-learning/CONTENT_PRODUCTION_PLAN.md) | [Confirmed] | P3 |
| FR-QB-013 | Phản hồi theo phương án nhiễu: `options[].feedback` (i18n) lưu phía đáp án, chỉ trả trong kết quả chấm (`ActivityResponseResult.feedback`), không có trong render payload ([QUESTION_BANK](../02-learning/QUESTION_BANK.md)) | [Confirmed] | P2 |
| FR-QB-014 | Hiệu chỉnh độ khó bán tự động: hệ thống đề xuất độ khó mới từ `p_value` khi đủ số lượt làm, người duyệt nội dung chấp nhận mới đổi (tạo version mới) ([QUESTION_BANK](../02-learning/QUESTION_BANK.md)) | [Confirmed] | P3 |
| FR-QB-015 | Chấm partial credit cấu hình được theo `scoring_rule` (`all_or_nothing` hoặc `partial` với `partial_method` theo dạng câu: `right_minus_wrong`, `proportional`, `adjacent_pairs`) ([QUESTION_BANK](../02-learning/QUESTION_BANK.md)) | [Confirmed] | P2 |

### 2.7. Activity & Attempt Engine — `EXR` (§4.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-EXR-001 | Chế độ: bài cá nhân, theo quest, theo chủ đề, nhà trường giao theo khối (ADR-0095) (`school_admin`/`academic_manager`), tự luyện, làm lại câu sai, chẩn đoán | [Confirmed] (người giao theo ADR-0072) | P2 (nhà trường giao: P3) |
| FR-EXR-002 | Lưu Attempt, Answer, Correctness, Score, Duration, Submission Time, Content Version, LO | [Confirmed] | P2 |
| FR-EXR-003 | Practice: phản hồi đúng/sai + giải thích ngay sau từng câu | [Confirmed] | P2 |
| FR-EXR-004 | Resume attempt chưa hoàn thành theo chính sách (TTL, số lần) | [Confirmed] / tham số [Confirmed] | P2 |
| FR-EXR-005 | Submit idempotent (`Idempotency-Key`), một attempt chỉ submit một lần | [Confirmed] | P2 |
| FR-EXR-006 | Thời gian tính theo server timestamp; client elapsed chỉ tham khảo | [Confirmed] | P2 |
| FR-EXR-007 | Một họ API attempt `/api/v1/learning/activity-attempts` cho mọi loại hoạt động (start, responses, progress, submit, resume, review) — ADR-0059 | [Confirmed] | P2 |
| FR-EXR-008 | Evaluation tách khỏi Attempt: chiến lược chấm chỉ là tự động; đúng một evaluation có hiệu lực; re-evaluate do hệ thống khi nội dung có lỗi (`attempt.re_evaluated`, do `content_admin`/`academic_manager` kích hoạt, có audit) tạo evaluation mới, không sửa cũ ([ACTIVITY_ENGINE §7](../02-learning/ACTIVITY_ENGINE.md#7-evaluation), ADR-0072) | [Confirmed] | P3 |
| FR-EXR-009 | **Removed (ADR-0072)** — tự luận chấm rubric, hàng đợi chấm, trả bài, chấm lại thủ công ngoài Release; ngoài version này ([docs/99-deferred](../99-deferred/README.md)) | Removed · Deferred (ADR-0082) | — |
| FR-EXR-010 | **Removed (ADR-0072)** — activity `project` (dự án nhiều bước chấm tay) ngoài Release | Removed · Deferred (ADR-0082) | — |
| FR-EXR-011 | **Removed (ADR-0072)** — AI hỗ trợ chấm bài viết/nói phụ thuộc người duyệt chấm; mở lại cần ADR mới (ADR-0053 Proposed) | Removed · Deferred (ADR-0082) | — |
| FR-EXR-012 | Gợi ý nhiều cấp: `hints[]` trong câu hỏi; `POST /api/v1/learning/activity-attempts/{attempt_id}/items/{position}/hints:next` trả gợi ý cấp tiếp theo; learning event mang `hints_used`; mastery nhân trọng số `w_hint` khi có dùng gợi ý ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)) | [Confirmed] | P2 |
| FR-EXR-013 | `retry_wrong` ưu tiên câu "anh em" (cùng LO, cùng loại/độ khó, khác `question_id`) thay vì lặp lại đúng câu đã sai ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)) | [Confirmed] | P2 |
| FR-EXR-014 | Chế độ luyện trộn chủ đề (interleaving): bộ câu luyện tập trộn nhiều LO đã học theo tỷ lệ cấu hình ([LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md)) | [Confirmed] | P3 |
| FR-EXR-015 | Content manifest mang `min_client_version` theo `activity_type`; client cũ không nhận activity có loại chưa hỗ trợ, hiển thị thông báo cần cập nhật thay vì lỗi ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)) | [Confirmed] | P2 |
| FR-EXR-016 | Attempt quá `deadline_at` chuyển `expired`; worker tự nộp (auto-submit) nếu đã có câu trả lời; phản hồi đến sau deadline + ân hạn mạng bị từ chối `ATTEMPT_EXPIRED` ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)) | [Confirmed] | P2 |

### 2.8. Exam & Practice — `EXM` (§4.5)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-EXM-001 | Hình thức: Practice Test, Mock Exam, Chapter Test, Semester Test, Subject Test, Custom Assessment, Timed Exam | [Confirmed] | P3 |
| FR-EXM-002 | Tạo đề, random đề, random câu, xáo trộn đáp án, cấu hình thời gian & thang điểm | [Confirmed] | P3 |
| FR-EXM-003 | Nộp bài, chấm điểm, hiển thị kết quả theo `result_visibility`, phân tích câu sai, đề xuất nội dung ôn tập | [Confirmed] | P3 |
| FR-EXM-004 | Phân biệt luyện tập và kiểm tra chính thức (`is_official`) | [Confirmed] | P3 |
| FR-EXM-005 | Chống nộp trùng, chống sửa điểm từ client, tự nộp khi hết `deadline_at` | [Confirmed] | P3 |
| FR-EXM-006 | Kết quả thi lưu bền vững trong PostgreSQL, không phụ thuộc Valkey | [Confirmed] | P3 |
| FR-EXM-007 | Thi chính thức có giá trị pháp lý/proctoring | [Confirmed — owner 2026-11-20] OQ-18: không thuộc Release 1.0 | Ngoài Release |
| FR-EXM-008 | Chấm lại đề (regrade) là re-evaluate do hệ thống khi câu hỏi lỗi (`RegradeRequest`, do `content_admin`/`academic_manager` kích hoạt, có lý do, audit): điểm mới, lịch sử điểm cũ truy xuất được, phát `exam.regraded`/`attempt.re_evaluated` ([EXAM_ENGINE](../02-learning/EXAM_ENGINE.md)) | [Confirmed] | P3 |
| FR-EXM-009 | Gia hạn thời gian làm bài cho học sinh cụ thể (`accommodations[]` của ExamSession, vd. +25%) do `school_admin`/`academic_manager` cấu hình ([EXAM_ENGINE](../02-learning/EXAM_ENGINE.md)) | [Confirmed] | P3 |
| FR-EXM-010 | Đề `is_official`: cân bằng độ khó giữa các biến thể đề theo `p_value`, báo cáo độ tin cậy (KR-20/alpha) sau kỳ kiểm tra ([EXAM_ENGINE](../02-learning/EXAM_ENGINE.md)) | [Confirmed] | P3 |

### 2.9. Learning Progress & Mastery — `PRG` (§4.6)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-PRG-001 | Theo dõi tiến độ, số bài hoàn thành, tỷ lệ chính xác, thời gian học, số lần làm, LO mastery, kỹ năng yếu, tiến bộ theo thời gian | [Confirmed] | P2–P3 |
| FR-PRG-002 | Mastery theo `(student, LO)` tính từ nhiều bằng chứng, không đồng nhất với điểm một lần làm bài | [Confirmed] | P2 |
| FR-PRG-003 | Mô hình mastery cấu hình được, mặc định Weighted Evidence (ADR-0017), lưu `model_version` | [Confirmed] cấu hình / [Confirmed] thuật toán | P2 |
| FR-PRG-004 | Gợi ý ôn tập dựa trên LO yếu (`/api/v1/learning/recommendations`) | [Confirmed] | P3 |
| FR-PRG-005 | Học sinh xem tiến độ dạng thân thiện theo độ tuổi (sao/huy hiệu cho lớp 1–5, biểu đồ cho 6–9) | [Confirmed] | P3 |
| FR-PRG-006 | Lọc đoán bừa: response có `response_ms` dưới ngưỡng tối thiểu của item (mặc định theo loại câu) gắn cờ `rapid_guess`, bị loại khỏi mastery và hiện trong báo cáo nhà trường là "làm quá nhanh"; mastery hiệu chỉnh hệ số đoán trúng theo loại câu ([LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)) | [Confirmed] | P2 |
| FR-PRG-007 | Bài chẩn đoán (`AttemptMode = diagnostic`) ngắn đầu năm học/đầu chủ đề để khởi tạo mastery và gợi ý điểm bắt đầu; không tính vào bảng xếp hạng, không vào báo cáo kết quả bài giao ([LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)) | [Confirmed] | P3 |
| FR-PRG-008 | Gợi ý ôn tập tự động có giải thích (LO yếu, LO tiên quyết) là con đường học chính thay cho chỉ định của giáo viên; học sinh thấy lý do gợi ý trong game (ADR-0072, [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)) | [Confirmed] | P3 |
| FR-PRG-009 | Recompute mastery: job tính lại mastery từ `learning_events` khi đổi `model_version` (chạy shadow, so sánh rồi chuyển) và cho LO bị ảnh hưởng khi re-evaluate ([LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)) | [Confirmed] | P3 |

### 2.10. Learning Events & Analytics — `ANL` (§16)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-ANL-001 | Ghi learning events (activity/question/exercise/exam/lesson/quest/learning_objective.*) với event_id, student_id, school_id, session_id, activity_id, content_id, content_version, type, timestamp, payload, idempotency key | [Confirmed] | P2 |
| FR-ANL-002 | Lưu bền vững trong `learning.learning_events` (LRS nội bộ), tương thích tư duy xAPI | [Confirmed] / thiết kế [Confirmed] | P2 |
| FR-ANL-003 | Analytics theo học sinh, khối, trường, môn, chủ đề, LO (bỏ chiều giáo viên theo ADR-0072, bỏ chiều lớp theo ADR-0095) | [Confirmed] (viết lại theo ADR-0072, ADR-0095) | P3 |
| FR-ANL-004 | Không dùng dữ liệu hành vi ngoài mục đích đã công bố | [Confirmed] | P1+ |
| FR-ANL-005 | Aggregation chạy trong worker, dashboard đọc từ read model | [Confirmed] | P3 |
| FR-ANL-006 | **Game Analytics** tách khỏi Learning Analytics và Monitoring: chỉ số engagement, retention, funnel, tiến trình, kinh tế, xã hội, chơi lành mạnh, hiệu năng theo hạng thiết bị — định nghĩa ở [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) | [Confirmed] tách / chỉ số [Confirmed] | P4 |
| FR-ANL-007 | Nguồn: domain event NATS + telemetry client tối thiểu không PII; lưu schema `analytics` của module game (ADR-0058) | [Confirmed] | P4 |
| FR-ANL-008 | Định danh giả danh `player_key`; không lưu tên, nickname, `student_id`, nội dung chat trong analytics | [Confirmed] | P4 |
| FR-ANL-009 | Dashboard Game Analytics trong workspace Game Live Operations (platform); báo cáo tổng hợp cho Tenant/School Admin, ẩn ô < 5 học sinh | [Confirmed] | P4 |
| FR-ANL-010 | Không dùng thời gian chơi làm thước đo thành công; có chỉ số chơi lành mạnh (vượt mốc nhắc nghỉ, chơi khuya) | [Confirmed] | P4 |
| FR-ANL-011 | Tenant tắt được telemetry sản phẩm; telemetry ổn định (hiệu năng, crash) vẫn thu (cơ chế kỹ thuật; cơ sở pháp lý/consent: Deferred ADR-0082) | [Confirmed] | P4 |
| FR-ANL-012 | Có trang dashboard cho trường → khối → học sinh (School Learning Dashboard), tenant, platform và Game Analytics; số liệu theo ngày/tuần/tháng | [Confirmed — người duyệt] (dashboard cho nhà trường theo ADR-0072; trường → khối theo ADR-0095) | P3–P4 |
| FR-ANL-013 | Dashboard chỉ đọc bảng tổng hợp dựng gia tăng; khoảng thời gian tối đa 2 năm, độ hạt tự chọn theo khoảng; hiển thị thời điểm cập nhật và nhãn tạm tính ([DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md)) | [Confirmed — ADR-0062] | P3 |
| FR-ANL-014 | Xuất báo cáo dashboard chạy bất đồng bộ, link tải có hạn, có audit | [Confirmed — ADR-0062] | P3 |
| FR-ANL-015 | Báo cáo tham khảo theo mức đánh giá TT27/2020 (tiểu học) và TT22/2021 (THCS) từ mastery/kết quả; ghi rõ "chỉ tham khảo", không thay đánh giá chính thức của nhà trường ([LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)) | [Confirmed] | P3 |
| FR-ANL-016 | Báo cáo learning analytics cấp tổ chức/tenant/platform chỉ dùng dữ liệu gộp, ẩn ô có < 5 học sinh (k-anonymity tối thiểu) ([LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)) | [Confirmed] | P3 |
| FR-ANL-017 | Ngân hàng câu hỏi/đề đo hiệu quả (`efficacy_instrument`) tách khỏi mọi luyện tập, không xuất hiện trong practice, quest hay thi đấu ([LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md), ADR-0080) | [Confirmed] | P3 |
| FR-ANL-018 | Bài kiểm tra độc lập trước/sau cho cụm (trường, khối) tham gia và cụm so sánh cùng khối (ADR-0095); không tạo evidence mastery, không cộng XP, không công bố điểm cho học sinh ([LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md), ADR-0080) | [Confirmed] | P3 |
| FR-ANL-019 | Ghi thỏa thuận tham gia nghiên cứu của tenant (pilot), có thu hồi và audit ([LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md), ADR-0080) | [Confirmed] | P3 |
| FR-ANL-020 | Thử nghiệm A/B cấu hình mastery và luật gợi ý theo nhánh gán ổn định, ghi `experiment_arms` vào learning event, có ngưỡng dừng sớm ([LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md), ADR-0080) | [Confirmed] | P3 |
| FR-ANL-021 | Báo cáo hiệu quả: effect size, tỉ lệ học sinh tiến bộ, tỉ lệ rời nghiên cứu, độ tin cậy dự báo của mastery so với kiểm tra độc lập ([LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md), ADR-0080) | [Confirmed] | P3 |
| FR-ANL-022 | Xuất dữ liệu nghiên cứu định danh giả, chỉ cho nhân sự platform có quyền, áp k-anonymity như FR-ANL-016 ([LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md), ADR-0080) | [Confirmed] | P3 |
| FR-ANL-023 | Nhà trường rút cụm (trường, khối) hoặc học sinh khỏi phần phân tích nghiên cứu (ADR-0095); dữ liệu đã rút bị loại khỏi báo cáo và bản xuất sau đó ([LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md), ADR-0080) | [Confirmed] | P3 |

### 2.11. Game World & Character — `GAME` (§2.3, §3.1–3.3, §13)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-GAME-001 | Thế giới 3D stylized/cartoon/low-poly, thân thiện, không bạo lực | [Confirmed] | P2 |
| FR-GAME-002 | Thế giới gồm 8 vùng đất có cốt truyện, bao trọn 7 khu chức năng bắt buộc (Main Hub, Learning Zone theo môn, Practice Zone, Exam Zone, Competition Arena, Social Zone, Quest Zone) — bảng ánh xạ ở [WORLD_DESIGN §3](../01-game-design/WORLD_DESIGN.md#3-bản-đồ-vùng-đất) | [Confirmed] khu chức năng · [Confirmed] tên vùng | P2 (Hub, vùng môn học, Practice, Quest) – P3 (Exam) – P4 (Arena) – P5 (Social) |
| FR-GAME-003 | Character controller, camera controller, animation state machine, NPC interaction | [Confirmed] | P2 |
| FR-GAME-004 | Avatar customization: skin, outfit, hair, accessories, emotes; character level; inventory | [Confirmed] | P2 |
| FR-GAME-015 | Luồng tạo avatar lần đầu (preset, dáng người, màu da/tóc, trang phục mặc định, nickname), equipment slot, avatar preset, 6 emote 3D mặc định (vẫy tay, vỗ tay, nhảy, ăn mừng, cảm ơn, xin lỗi) — [CHARACTER_SYSTEM §5.0, §7](../01-game-design/CHARACTER_SYSTEM.md) | [Confirmed] tính năng (Master Prompt mới §5.2, §13.5) / [Confirmed] chi tiết | P2 |
| FR-GAME-016 | Mạch truyện "Thắp lại Tia Sáng" gồm 7 chương gắn vùng; chương môn học mở song song, không khóa học tập theo truyện ([WORLD_DESIGN §2.2](../01-game-design/WORLD_DESIGN.md#22-mạch-truyện-chính-confirmed--người-duyệt-2026-11-15-adr-0056)) | [Confirmed] | P2–P4 |
| FR-GAME-017 | World map, cổng di chuyển, khám phá (discovery point, collectible, % hoàn thành bản đồ), mở khóa vùng theo level/quest/LO, không theo thanh toán ([WORLD_DESIGN §6–§8](../01-game-design/WORLD_DESIGN.md)) | [Confirmed] | P2–P3 |
| FR-GAME-018 | Học tập lồng trong ngữ cảnh game: mỗi khối học tập ngắn rồi xen tương tác thế giới; không biến game thành chuỗi quiz ([GAME_DESIGN](../01-game-design/GAME_DESIGN.md), [LEARNING_EXPERIENCE §2.1](../02-learning/LEARNING_EXPERIENCE.md)) | [Confirmed] nguyên tắc · [Confirmed] ngưỡng | P2 |
| FR-GAME-005 | Nickname sinh tự động/chọn từ danh sách duyệt; tên thật không hiển thị cho người chơi khác | [Confirmed] | P2 |
| FR-GAME-006 | Game client data-driven: quest, mini-game, NPC dialogue, reward, cấu hình khu vực tải từ backend; cập nhật nội dung không cần build lại game | [Confirmed] | P2 |
| FR-GAME-007 | Game session server-side (`/api/v1/game/sessions`), ghi platform, client version, end reason | [Confirmed] | P1 |
| FR-GAME-008 | Kiểm tra phiên bản client tối thiểu; buộc cập nhật khi không tương thích API | [Confirmed] | P1 |
| FR-GAME-009 | Chạy được trên máy tầm trung: renderer Mobile/Compatibility, preset Thấp/Trung bình/Cao tự phát hiện, độ phân giải động, LOD, occlusion, instancing, lightmap bake, nén texture, streaming vùng ([PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md), ADR-0057) | [Confirmed] | P2–P6 |
| FR-GAME-010 | UI theo nhóm tuổi: lớp 1–5 trực quan/ít thao tác/chữ to + hỗ trợ đọc âm thanh; lớp 6–9 nhiều thông tin hơn | [Confirmed] / audio [Confirmed] | P2 |
| FR-GAME-011 | Telemetry client (crash, FPS, load time) không chứa PII, rate limit (`/api/v1/game/telemetry`) | [Confirmed] | P2 |
| FR-GAME-012 | Nhắc nghỉ sau thời gian chơi liên tục (mặc định 45 phút), giờ truy cập do trường cấu hình; **giờ giới nghiêm cấp platform** `platform_curfew` (mặc định 21:30–06:30) cho game và chat: không đăng nhập, không tạo game session, không gửi chat, phiên đang chạy được cảnh báo rồi kết thúc; trường chỉ siết thêm; áp như nhau cho mọi học sinh ([CHILD_SAFETY §4.1](../10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078), ADR-0078) | [Confirmed] OQ-17 · giới nghiêm [Confirmed — Technical Architect 2026-11-17] | P2 |
| FR-GAME-013 | **Lưu thời gian đăng nhập game**: mỗi lần vào game ghi `game.login_events` (thời điểm vào/ra theo giờ server, nền tảng, thiết bị rút gọn, lý do kết thúc; `ip_hash`, không IP thô) và cập nhật `users.last_login_at`; vận hành tenant (Tenant/School Admin) xem lịch sử và "số ngày chưa vào game" (ADR-0031) | [Confirmed] | P1 |
| FR-GAME-014 | Game có nút "Đăng xuất" (đóng game session, thu hồi phiên, xóa token khỏi kho bảo mật — ADR-0086); đóng game kết thúc game session nhưng giữ phiên đăng nhập trên thiết bị cá nhân; thiết bị phòng máy dùng chung (`shared_device`) không lưu token và đăng xuất khi đóng; phiên không heartbeat > 45 s tự kết thúc | [Confirmed] | P1 |

### 2.12. Quest — `QST` (§3.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-QST-001 | Quest có: ID, tên, mô tả, loại, khối, môn, chủ đề, LO, điều kiện bắt đầu, các bước, điều kiện hoàn thành, phần thưởng, thời gian, phiên bản | [Confirmed] | P2 |
| FR-QST-002 | 12 loại quest: Main, Side, Daily, Weekly, Learning, Exploration, Social, Party, Challenge, Seasonal, Special Event, Boss (quest schema v2; thay 7 loại trước đây) | [Confirmed] (Master Prompt mới §7) | P2 (Party/Challenge/Boss: P3–P4) |
| FR-QST-008 | Quest hỗ trợ narrative, NPC, level requirement, learning requirement, repeatability, cooldown, failure conditions; thất bại mặc định không mất tiến độ | [Confirmed] / chi tiết [Confirmed] | P2 |
| FR-QST-003 | Quest liên kết nội dung học tập; bước `activity` tạo Attempt trong Activity & Attempt Engine | [Confirmed] | P2 |
| FR-QST-004 | Bước: talk_npc, go_to, activity, collect, minigame, party_activity | [Confirmed] | P2 |
| FR-QST-005 | Tiến độ quest lưu server, hoàn thành bước qua domain event (ADR-0019) | [Confirmed] | P2 |
| FR-QST-006 | Quest lọc theo grade_level của học sinh; không giao nội dung sai khối | [Confirmed] | P2 |
| FR-QST-007 | Shared Quest Progress theo cấu hình `shared_progress_policy`; kết quả học tập vẫn riêng từng học sinh | [Confirmed] | P4 |
| FR-QST-009 | Bước quest kiểu `interdependent` có `role`: mỗi thành viên party giữ một phần dữ kiện, cả nhóm phải phối hợp mới hoàn thành ([QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md)) | [Confirmed] | P4 |

### 2.13. Mini-game Engine — `MG` (§3.5)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-MG-001 | Loại: Multiple Choice, True/False, Fill in the Blank, Drag and Drop, Matching, Ordering, Calculation, Puzzle, Word Game, Science Simulation, Timed Challenge | [Confirmed] | P2 (6 loại) – sau |
| FR-MG-002 | Mini-game không có ngân hàng câu hỏi riêng; lấy từ Learning Engine qua QuestionSet/selection rule | [Confirmed] | P2 |
| FR-MG-003 | Cấu hình: độ khó, thời gian, số câu, điểm, cách tính kết quả, phản hồi đúng/sai, cơ chế hoàn thành, reward config | [Confirmed] | P2 |
| FR-MG-004 | Mỗi mini-game là scene Godot tái sử dụng, nhận `MiniGameConfig` + render payload | [Confirmed] | P2 |

### 2.14. Multiplayer & Presence — `MP` (§5.1, §14)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-MP-001 | Hỗ trợ Multiplayer World, Room, Lobby, Team, Match, Session | [Confirmed] | P4 |
| FR-MP-002 | Hub instance tối đa 40 người, cấp theo school/grade band; bạn bè/party ưu tiên cùng instance (ADR-0012) | [Confirmed] OQ-05 | P4 |
| FR-MP-003 | Presence/online status (Valkey), chỉ hiển thị cho bạn bè/party/club theo policy (không theo lớp — ADR-0095) | [Confirmed] / hiển thị [Confirmed] | P4 |
| FR-MP-004 | Đồng bộ vị trí qua WebSocket ≤10 Hz, interpolation, server kiểm tra tốc độ/biên (ADR-0011) | [Confirmed] OQ-06 | P4 |
| FR-MP-005 | Heartbeat, timeout, reconnect, session recovery, disconnect handling | [Confirmed] | P4 |
| FR-MP-006 | Không gửi chuyển động 3D qua REST; Valkey không làm game simulation server | [Confirmed] | P4 |

### 2.15. Party — `PTY` (Party Addendum)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-PTY-001 | Create, Join, Leave, Invite Friend, Accept, Reject, Kick, Transfer Leader, Disband, Settings, Member Status, Party Chat | [Confirmed] | P4 (gồm Party Chat văn bản tự do — channel `party` always-on, FR-CHAT-009) |
| FR-PTY-002 | Thuộc tính: Party ID, name, leader, members, max members, type, visibility, invitation policy, current activity, current game session, timestamps | [Confirmed] | P4 |
| FR-PTY-003 | Party types: Friend Party, Study Party (`study_party`, đổi tên từ Study Group), Quest Party, Competition Team, Custom Group; không có Class Group (ADR-0095) | [Confirmed] (viết lại theo ADR-0095) | P4 |
| FR-PTY-004 | Max members cấu hình được; mặc định 4, cho phép 2–8 | [Confirmed] cấu hình / [Confirmed] giá trị | P4 |
| FR-PTY-005 | Một học sinh chỉ có một active Party tại một thời điểm (Addendum §6 ghi cho MVP; giữ nguyên cho Release 1.0); chống duplicate membership (unique constraint) | [Confirmed] | P4 |
| FR-PTY-006 | Party Lobby: danh sách thành viên, avatar, online, Ready/Not Ready, leader controls, chọn activity & game mode, mời, group chat, Start Session | [Confirmed] | P4 |
| FR-PTY-007 | Start Session: xác nhận leader → kiểm tra thành viên → kiểm tra điều kiện khối/quyền → tạo Game Session/room → gán thành viên → gửi session info tới clients → đồng bộ trạng thái → theo dõi reconnect/disconnect → kết thúc và lưu kết quả | [Confirmed] | P4 |
| FR-PTY-008 | Leader disconnect: grace period 60 giây, sau đó tự chuyển leader cho thành viên tham gia sớm nhất còn online | [Confirmed] cơ chế / [Confirmed] tham số | P4 |
| FR-PTY-009 | Party private cần lời mời/quyền; lời mời hết hạn sau 5 phút; không mời người đã block/bị block | [Confirmed] / tham số [Confirmed] | P4 |
| FR-PTY-010 | Giới hạn theo school/grade/activity; mặc định chỉ cùng trường; lớp 1–3 chỉ `friend`/`quest`, cùng đúng khối, khác trường được trong cùng tenant theo ADR-0054, không liên tenant (ADR-0095) | [Confirmed] cấu hình / [Confirmed] mặc định (viết lại theo ADR-0095) | P4 |
| FR-PTY-011 | Chống spam lời mời (rate limit), audit log cho create/kick/transfer/disband | [Confirmed] | P4 |
| FR-PTY-012 | Thành viên mất kết nối: các thành viên khác tiếp tục nếu activity cho phép (`continue_on_disconnect`) | [Confirmed] | P4 |
| FR-PTY-013 | Không chia sẻ kết quả học tập cá nhân giữa thành viên; mỗi người có attempt riêng | [Confirmed] | P4 |
| FR-PTY-014 | API `/api/v1/multiplayer/parties`, `/api/v1/multiplayer/invitations`; realtime event qua `/realtime/v1` | [Confirmed] | P4 |

### 2.16. Matchmaking — `MM` (§5.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-MM-001 | Chế độ: Random, Skill-based, Grade-based, Subject-based, Topic-based, Friend, Team, Tournament | [Confirmed] | P4 |
| FR-MM-002 | Ràng buộc cứng (khối lớp, độ phù hợp nội dung, policy trường) kiểm tra **trước** rating; không ghép lớp 1 với lớp 9 | [Confirmed] | P4 |
| FR-MM-003 | Mặc định: lớp 1–5 chỉ ghép cùng khối (`grade_level`); lớp 6–9 theo `grade_band` (±1 khối khi chờ lâu); **chỉ cùng trường**. Ghép liên trường trong cùng tenant `unit` chỉ khi tenant bật; liên tenant chỉ ở sự kiện platform | [Confirmed] | P4 |
| FR-MM-004 | Vòng đời: Queue → Ticket → Match Proposal → Lobby → Ready Check → Team Assignment → Match Start → Match End → Result; hỗ trợ Reconnect | [Confirmed] | P4 |
| FR-MM-005 | Cửa sổ rating mở rộng theo thời gian chờ; timeout ghép trận 90 giây đề xuất chế độ khác/bot luyện tập | [Confirmed] | P4 |
| FR-MM-006 | Ticket chỉ trong Valkey; mất → client tạo lại (ADR-0004) | [Confirmed] | P4 |

### 2.17. Competition / Battle — `CMP` (§5.3, §5.6)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CMP-001 | Chế độ: 1v1 Knowledge Battle, Team Battle, Co-op Challenge, Timed Quiz, Subject Tournament, Grade/School/Seasonal Competition (thi đấu khối thay thi đấu lớp — ADR-0095) | [Confirmed] | P4 (Seasonal: P6+) |
| FR-CMP-002 | Battle là state machine server-authoritative: question plan do server chọn, chấm và tính điểm thời gian server-side | [Confirmed] | P4 |
| FR-CMP-003 | Kết quả trận ghi một lần (unique `match_id`), idempotent, audit | [Confirmed] | P4 |
| FR-CMP-004 | Câu trả lời trong trận ghi learning evidence với context `competition` (trọng số thấp cho mastery) | [Confirmed] | P4 |
| FR-CMP-005 | Xử lý disconnect: grace 30 giây; quá hạn → xử thua phần còn lại/forfeit theo mode | [Confirmed] | P4 |
| FR-CMP-006 | Không có bonus điểm tốc độ trong 1v1 cho lớp 1–5 ([COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)) | [Confirmed] | P4 |

### 2.18. Ranking — `RNK` (§5.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-RNK-001 | Hệ thống ranking độc lập; Glicko-2 theo `(student, mode, subject, season)` (ADR-0018) | [Confirmed] độc lập / [Confirmed] Glicko-2 | P4 |
| FR-RNK-002 | Không dùng rating để đánh giá năng lực học tập; tách Learning Score, Mastery, Game Rating, Game XP, Competition Points | [Confirmed] | P4 |
| FR-RNK-003 | Hiển thị rating cho học sinh dưới dạng tier thân thiện; **không** hiển thị số rating hay `rating_delta` cho học sinh ở mọi khối (lớp 1–9); số rating chỉ dùng nội bộ và cho vận hành ([COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)) | [Confirmed — Technical Architect 2026-11-16] | P4 |
| FR-RNK-004 | Season soft-reset rating | [Confirmed] | P4 |

### 2.19. Leaderboard — `LDB` (§5.4)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-LDB-001 | Bảng: Global, School, Grade, Subject, Topic, Game Mode, Friends, Team, Daily, Weekly, Monthly, Season | [Confirmed] | P4 (Global trong Release 1.0; quy tắc hiển thị nickname theo FR-LDB-003) |
| FR-LDB-002 | Leaderboard cache bằng Valkey sorted set, nguồn từ PostgreSQL | [Confirmed] | P4 |
| FR-LDB-003 | Leaderboard ngoài phạm vi trường chỉ hiển thị nickname, không tên trường/khối chi tiết | [Confirmed] | P4 |
| FR-LDB-004 | Trường có thể tắt leaderboard công khai cho lớp 1–3 | [Confirmed] | P4 |
| FR-LDB-005 | Mục tiêu tập thể của khối trong trường (`grade_goal`) thay bảng của lớp: hiển thị tiến độ chung, không xếp hạng cá nhân, không tên, ẩn ô đóng góp < 5 học sinh ([COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md), ADR-0095) | [Confirmed] (viết lại theo ADR-0095) | P4 |

### 2.20. Tournament — `TRN` (§5.5)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-TRN-001 | Format: Single Elimination, Double Elimination, Round Robin, Team, School, Grade Tournament (giải khối của trường thay giải lớp — ADR-0095) | [Confirmed] | P4 (Single elim, Round robin bật mặc định; Double elim, vòng bảng, Team, School, Grade xây cùng tournament engine ở P4, tắt theo template tới khi platform bật) |
| FR-TRN-002 | Tạo giải, đăng ký, chia bảng, tạo bracket, lên lịch, xác nhận người tham gia, quản lý trận, xác nhận kết quả, trao thưởng, lưu lịch sử | [Confirmed] | P4 |
| FR-TRN-003 | Bracket lưu PostgreSQL, không chỉ Valkey | [Confirmed] | P4 |

### 2.21. Social — `SOC` (§6.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-SOC-001 | Friend list, friend request, invitation; tenant `common` tắt mặc định kết bạn (ADR-0078) | [Confirmed] | P5 (cơ bản cho mời party: P4) |
| FR-SOC-002 | Mặc định chỉ kết bạn cùng trường (như nhau cho mọi khối); tenant `unit` có thể mở liên trường trong tenant | [Confirmed] | P5 |
| FR-SOC-003 | Block / Mute người dùng; block chặn chat, mời, ghép party | [Confirmed] | P5 (Block: P4 cùng party) |
| FR-SOC-004 | Team/club của học sinh theo policy ([SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)); nhóm học tập LMS (Study Group, ADR-0084) đã bỏ theo ADR-0095 | [Confirmed] (viết lại theo ADR-0095) | P5 |

### 2.22. Chat — `CHAT` (§6.1–6.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CHAT-001 | Channel: World, Area, **Match**, Party, Team, Direct Message; cấu hình bằng policy; không có channel Class/Study Group và không có kênh theo khối (ADR-0095) | [Confirmed] | P4 (Match/Party/Team theo trận), P5 (còn lại) |
| FR-CHAT-002 | Không có rule = deny; quyền vào channel theo quan hệ (cùng trường/instance/nhóm/bạn bè) và policy, như nhau cho mọi khối (ADR-0020, ADR-0095) | [Confirmed] | P5 |
| FR-CHAT-003 | Text, emoji, reactions, quick messages, group chat, history, unread count, notifications | [Confirmed] | P4 (channel gameplay), P5 (còn lại) |
| FR-CHAT-004 | **Một bộ quy tắc chat cho mọi lớp 1–9** (không điều kiện theo `grade_level`/`grade_band`): đủ 7 channel; World = cùng trường (tenant `unit` có thể mở rộng toàn tenant); DM chỉ giữa bạn bè không block nhau; mọi học sinh dùng text, emoji, quick message, reaction (ADR-0020). Tenant `common`: tắt mặc định DM và World chat; trường có thể chọn preset "chỉ quick message + emote" cho cả trường (ADR-0078) | [Confirmed] (giá trị số [Confirmed]) | P5 |
| FR-CHAT-005 | **Mọi tin nhắn phải hợp lệ**: pipeline đồng bộ phía server cho mọi `kind` — normalize → định dạng/độ dài → profanity (tiếng Việt có/không dấu, teencode, leetspeak, tiếng Anh) → PII → spam/flood → bắt nạt → allow/mask/block/hold_for_review; `block` không lưu nội dung | [Confirmed] chức năng / [Confirmed] chi tiết pipeline | P5 |
| FR-CHAT-006 | Lịch sử chat theo quyền và retention job (mặc định 180 ngày; rà soát thời hạn theo luật: Deferred ADR-0082) | [Confirmed] / thời hạn [Confirmed] | P5 |
| FR-CHAT-007 | Không có voice chat trong Release 1.0; điểm mở rộng chỉ ở mức thiết kế, không hiện thực (ADR-0051, ADR-0070) | [Confirmed] | — |
| FR-CHAT-008 | Giao tin nhắn realtime qua `/realtime/v1`, gửi qua REST hoặc WS có idempotency client message id | [Confirmed] | P4 |
| FR-CHAT-009 | **Chat gameplay luôn có** (ADR-0020 mục 6): vào trận 1v1 → channel `match`; trận đội → `team` mỗi đội + `match` chung; tạo party → `party`. Tự tạo, tự join, đủ text/emoji/quick/reaction; không tắt được bằng cài đặt tenant/trường, không áp khung giờ của channel (giờ giới nghiêm platform chặn ở mức phiên chơi); chỉ mute/block cá nhân hoặc sanction ngăn một người gửi; preset "chỉ quick message + emote" của trường thu hẹp loại tin nhưng không tắt channel (ADR-0078) | [Confirmed] | P4 |
| FR-CHAT-010 | UI chat trong lobby/trận/party: panel thu gọn được nhưng không tắt; quick message một chạm; tab Đội/Cả trận; menu Tắt tiếng/Chặn/Báo cáo trên từng tin; báo cáo từ màn hình kết quả trận | [Confirmed] tính năng / [Confirmed] UX | P4 |
| FR-CHAT-011 | **Purge chat gameplay** (ADR-0033): channel `match`, `team` theo trận, `party` (sau giải tán) bị xóa vật lý sau `gameplay_chat_retention_days` (mặc định 7, tối thiểu 3) từ `closed_at`; World/Area sau 24 h; giữ bằng chứng của report/case/sanction trước khi purge; hoãn khi case mở; không áp dụng Team cố định/club/DM; người chơi được thông báo thời hạn lưu | [Confirmed] / [Confirmed] số ngày | P4 |
| FR-CHAT-012 | Sửa tin trong 15 phút (chạy lại kiểm duyệt, lưu revision) và thu hồi trong 24 giờ (placeholder, giữ bằng chứng, vẫn báo cáo được) — ADR-0034 | [Confirmed] tính năng / [Confirmed] thời hạn | P4 |
| FR-CHAT-013 | Reply, Quote (trích từ bản đã che), Forward (cùng tenant, ≤ 5 đích, chạy lại kiểm duyệt) — ADR-0034 | [Confirmed] / [Confirmed] giới hạn | P4 |
| FR-CHAT-014 | Emoji, Sticker (pack platform, có thể mở khóa bằng phần thưởng), GIF (thư viện nội bộ đã duyệt, không dịch vụ bên thứ ba), Reactions (≤ 3/người/tin) — ADR-0034 | [Confirmed] / [Confirmed] nguồn GIF nội bộ | P4 |
| FR-CHAT-015 | Mention `@nickname` (≤ 5/tin, thông báo + đếm riêng), Pin (≤ 3/channel), Copy (bản đã che), Typing indicator — ADR-0034 | [Confirmed] / [Confirmed] giới hạn | P4 |
| FR-CHAT-016 | Trạng thái Sent/Delivered/Read theo con trỏ, Unread count + mention count, Lịch sử phân trang và tìm kiếm full-text tiếng Việt có/không dấu — ADR-0034 | [Confirmed] | P4 (tìm kiếm P5) |
| FR-CHAT-017 | Truyền tải chat qua NATS JetStream (bền vững, trước khi xác nhận Sent) + Valkey Pub/Sub (fan-out realtime), lưu PostgreSQL bởi `chat-persister` — ADR-0035 | [Confirmed] | P4 |

### 2.23. Moderation & Child Safety — `MOD` (§6.3, §17.2)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-MOD-001 | Content/profanity filtering, anti-spam, anti-bullying, PII detection | [Confirmed] | P5 |
| FR-MOD-002 | Report message, report user, block, mute | [Confirmed] | P5 |
| FR-MOD-003 | Moderation queue tập trung ở Platform Safety Team (role `safety_moderator`); mọi case gắn tenant/trường; audit mọi hành động (ADR-0072) | [Confirmed] (người xử lý theo ADR-0072) | P5 |
| FR-MOD-004 | Chat policy theo platform → tenant → school (bỏ cấp class — ADR-0095; không theo grade — thay mục "Age/Grade-based Policy" của Master Prompt §6.3 theo ADR-0020); cấp dưới chỉ siết chặt hơn cấp trên | [Confirmed] | P5 |
| FR-MOD-005 | Không ai có quyền đọc toàn bộ chat của mọi trường; truy cập chat đều audit | [Confirmed] | P5 |
| FR-MOD-006 | Sanction: warn, mute có thời hạn, restrict (chỉ quick message), escalate | [Confirmed] | P5 |
| FR-MOD-007 | **Removed (ADR-0072)** — điều khiển channel lớp của giáo viên; channel lớp chịu chat policy cấp trường do `school_admin` đặt trong giới hạn platform | Removed | — |
| FR-MOD-008 | Không giám sát chat vượt quá mục đích an toàn cần thiết | [Confirmed] | P5 |
| FR-MOD-009 | Danh mục từ nhạy cảm quản lý trong CMS: category (severity, action mặc định), term (match type, biến thể tự sinh không dấu/teencode/leetspeak), allowlist, version; không hardcode (ADR-0030) | [Confirmed] | P5 |
| FR-MOD-010 | Action theo mức: `mask` / `block` / `hold_for_review` / `block_and_alert`; phản hồi không lộ từ đã khớp | [Confirmed] cơ chế / [Confirmed] ánh xạ severity → action | P5 |
| FR-MOD-011 | Công tắc kiểm duyệt bật/tắt toàn bộ hoặc từng bước (`sensitive_terms`, `spam`, `bullying`, `async_analysis`) theo platform/tenant; mặc định bật; chỉ System Admin; lý do bắt buộc, **phê duyệt của System Admin thứ hai**, audit, banner CMS, alert. Bước **PII và phát hiện grooming luôn chạy**, không có công tắc (ADR-0078) | [Confirmed] ADR-0030, ADR-0078 | P5 |
| FR-MOD-012 | Khi tắt vẫn giữ PII, grooming, độ dài tối đa, rate limit, quyền channel, sanction, block/mute/report, moderation thủ công | [Confirmed] ADR-0030 | P5 |
| FR-MOD-013 | Từ bổ sung cấp tenant (chỉ siết chặt) và bật/tắt category cấp tenant (chỉ category có `allow_tenant_toggle`) do Platform Safety Team thực hiện theo yêu cầu của tenant, có audit; role tenant không có quyền ghi danh mục từ hay bật/tắt category (ADR-0072) | [Confirmed] | P5 |
| FR-MOD-014 | Mọi lần tắt bắt buộc có `expires_at` ≤ 24 giờ (`moderation_switch_max_duration`), tự bật lại khi hết hạn (ADR-0078) | [Confirmed — Technical Architect 2026-11-17] | P5 |
| FR-MOD-015 | Publish danh mục chạy test hồi quy (recall ≥ 95%, false positive ≤ 3%) và reload không downtime ≤ 60 s | [Confirmed] | P5 |
| FR-MOD-016 | `safety_moderator` đọc liên tenant **chỉ trong module moderation** (case, bằng chứng, tin nhắn liên quan case) qua `tenancy.Bypass(ctx, "moderation_case")`, có audit và metric; không đọc chat ngoài phạm vi case ([MODERATION](../03-social-multiplayer/MODERATION.md)) | [Confirmed] ADR-0072 / [Confirmed] chi tiết | P5 |
| FR-MOD-017 | Thông báo nhà trường về vụ việc nghiêm trọng liên quan học sinh của trường: tóm tắt, không chứa nội dung chat thô, để trường liên hệ phụ huynh; case lưu thời điểm gửi yêu cầu và thời điểm trường xác nhận đã liên hệ ([MODERATION](../03-social-multiplayer/MODERATION.md)) | [Confirmed] ADR-0072 / [Confirmed] chi tiết | P5 |
| FR-MOD-018 | Safety Team trực 24/7 (on-call) cho case mức p1 (grooming, tự hại) ([MODERATION](../03-social-multiplayer/MODERATION.md), [CHILD_SAFETY](../10-security/CHILD_SAFETY.md)) | [Confirmed] | P5 |

### 2.24. Notification — `NTF`

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-NTF-001 | Thông báo trong game: lời mời party/kết bạn, bài được giao, kết quả, phần thưởng, thông báo trường | [Confirmed] | P2 (in-game) – P5 (realtime đầy đủ) |
| FR-NTF-002 | Realtime qua WebSocket; lưu bền vững để xem lại (`/api/v1/notifications`) | [Confirmed] | P5 |
| FR-NTF-003 | Không gửi push notification ngoài game tới học sinh trong Release 1.0; thông báo chỉ hiển thị trong game (inbox, HUD). Mở push sau này cần ADR mới và chính sách giờ được phép (OQ-17) | [Confirmed] | P5 |

### 2.25. Gamification & Reward — `GAM`, `RWD` (§3.3, §7)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-GAM-001 | XP, Level, Achievement, Badge, Daily/Weekly Challenge, Season Reward, Collection | [Confirmed] | P2 (XP, Level, Badge) – P4 (Season) |
| FR-GAM-002 | XP từ hoàn thành bài học, bài tập, quest, hoạt động, hợp tác, thi đấu | [Confirmed] | P2–P4 |
| FR-GAM-003 | Reward Engine có rule + cap ngày/tuần + diminishing returns khi lặp nội dung; chống farm câu dễ | [Confirmed] | P2 |
| FR-GAM-004 | XP ledger append-only, idempotent theo `(source_type, source_id, student_id, rule_id)` | [Confirmed] | P2 |
| FR-GAM-005 | Phần thưởng game không ảnh hưởng điểm/mastery/đánh giá chính thức | [Confirmed] | P2 |
| FR-RWD-001 | Inventory, cosmetic (outfit, hair, accessory, emote) mở khóa bằng bài tập, quest, sự kiện, thành tựu, thi đấu | [Confirmed] | P2 |
| FR-RWD-002 | Không cơ chế trả tiền để có kết quả học tập tốt hơn; không loot box | [Confirmed] / loot box [Confirmed] | P2 |
| FR-RWD-003 | Reward có giá trị nghiệp vụ chỉ do server trao | [Confirmed] | P2 |
| FR-GAM-006 | Ba miền tiến trình (game, learning, competition) tách biệt, không gộp điểm; game level không phải bằng chứng học tập — [PROGRESSION_SYSTEM §2](../01-game-design/PROGRESSION_SYSTEM.md) | [Confirmed] | P2 |
| FR-GAM-007 | Catalog nguồn XP có mã; đường cong XP cấu hình được, có version, không hardcode trong Godot; danh hiệu theo khoảng level | [Confirmed] / tham số [Confirmed] | P2 |
| FR-GAM-008 | Level-up event, phần thưởng lên cấp, progress bar, lịch sử level và giao dịch XP | [Confirmed] | P2 |
| FR-GAM-009 | World progression: mở khóa khu theo level/quest/LO/achievement, điểm khám phá, vật sưu tầm, hoàn thành bản đồ; khu học tập theo khối luôn mở; không trả tiền để mở khu | [Confirmed] / chi tiết [Confirmed] | P2–P3 |
| FR-GAM-010 | Quy tắc chuyển năm học khi học sinh lên lớp: giữ level, vật phẩm và vùng đã mở; nội dung học tập và quest lọc theo khối mới; mạch truyện tiếp tục, không chơi lại phần đã hoàn thành ([PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)) | [Confirmed] | P2 |
| FR-GAM-011 | Không giảm XP cho hoạt động ôn LO tiên quyết do hệ thống gợi ý hoặc nhà trường giao (kể cả nội dung khối dưới); giảm trọng số evidence từ `timed_challenge` ([PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)) | [Confirmed] | P2 |
| FR-CHL-001 | Challenge Engine: 11 loại (time, perfect, survival, puzzle, knowledge, boss, coop, party, weekly, seasonal, tournament), 5 mức độ khó cấu hình theo grade/subject/LO/activity/hiệu suất/mode — [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) | [Confirmed] (Master Prompt mới §8) | P3 |
| FR-CHL-002 | Boss Battle nhiều phase, đóng góp cá nhân/nhóm, retry, không phạt khi sai; server-authoritative trong module game | [Confirmed] / nơi chạy [Confirmed] | P3–P4 |
| FR-CHL-003 | Mỗi câu/hoạt động trong challenge/boss là attempt học tập cá nhân; learning record riêng từng thành viên | [Confirmed] | P3 |
| FR-ECO-001 | Item catalog (cosmetic, outfit, hair, accessory, emote, pet, title, badge, frame, decoration, knowledge card), equipment slot, inventory, item history — [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | [Confirmed] | P2 |
| FR-ECO-002 | Tiền trong game "Xu" + sổ cái append-only, ví không âm, đối soát; không mua bằng tiền thật (ADR-0049) | [Confirmed] (OQ-14) | P2 |
| FR-ECO-003 | Shop chỉ cosmetic, giá Xu, giới hạn mua, vật phẩm xác định; không loot box/hộp ngẫu nhiên trả phí | [Confirmed] không loot box / Shop [Confirmed] | P2 |
| FR-ECO-004 | Collection (pet, knowledge card, cosmetic, badge, exploration, event), xử lý trùng lặp, trading tắt mặc định | [Confirmed] (Master Prompt mới §10.2) | P2–P3 |
| FR-ECO-005 | Achievement có định nghĩa, trigger, điều kiện, tiến độ, ẩn, tier, version; không dựa trên thời gian chơi | [Confirmed] | P2 |

### 2.25a. Event & Season (live-ops) — `EVT` (Master Prompt tái cấu trúc §12, §16.1)

Nguồn: [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md).

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-EVT-001 | Sự kiện live-ops: lễ hội thế giới, cuối tuần, sự kiện trường, community goal, chiến dịch học tập, boss đặc biệt, quest giới hạn thời gian; cấu hình dữ liệu, có version (ADR-0043) | [Confirmed] loại / [Confirmed] danh mục | P3 |
| FR-EVT-002 | Event Builder trong Game Operations Console: lịch, đối tượng (khối/trường/tenant), nội dung tham chiếu, phần thưởng, preview, duyệt, publish, lên lịch, tiến độ, analytics | [Confirmed] | P3 |
| FR-EVT-003 | Vòng đời Scheduled → Live → Ended → Archived, dừng khẩn cấp/tiếp tục/hủy có lý do và audit; lượt chơi đang dở không bị cắt; tiến độ đã đạt không bao giờ bị giảm | [Confirmed] | P3 |
| FR-EVT-004 | Community goal cộng dồn theo trường/tenant từ event server đã kiểm chứng, trần đóng góp mỗi học sinh/ngày, không xếp hạng cá nhân; thưởng mốc cho mọi người có đóng góp | [Confirmed] | P3 |
| FR-EVT-005 | Season track miễn phí (không track trả phí), điểm mùa từ hoạt động đa dạng có trần/ngày, thưởng cosmetic; hết mùa tự cấp thưởng chưa nhận | [Confirmed] miễn phí / [Confirmed] tham số | P4 |
| FR-EVT-006 | Tenant tổ chức sự kiện trường từ template platform, chỉ với `allowed_params` (ADR-0027, ADR-0029) | [Confirmed] | P3 |
| FR-EVT-007 | Quy tắc lành mạnh: không phạt mất chuỗi, không ép online, không khóa học tập, tôn trọng giờ truy cập và nhắc nghỉ | [Confirmed — Master Prompt §12] | P3 |
| FR-EVT-008 | Nội dung sự kiện platform song ngữ bắt buộc (ADR-0056); trang trí trong ngân sách máy tầm trung, có biến thể preset Thấp (ADR-0057) | [Confirmed] | P3 |

Acceptance criteria: AC-EVT-001…012 ([EVENT_SEASON_SYSTEM §18](../01-game-design/EVENT_SEASON_SYSTEM.md#18-acceptance-criteria)).

### 2.26. CMS — `CMS` (§8)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-CMS-001 | Vai trò tenant: `tenant_admin`, `school_admin`, `academic_manager`, `tenant_content_editor`, `tenant_content_reviewer`; vai trò platform: `system_admin` (Platform Admin), `content_admin`, `content_reviewer`, `game_designer`, `liveops`, `competition_admin`, `safety_moderator`. Không có role giáo viên và `tenant_moderator` (ADR-0072) — [ADMIN_PORTAL §2](../04-admin-portal/ADMIN_PORTAL.md#2-permission-model) | [Confirmed] gốc (sửa theo ADR-0072) / [Confirmed] bổ sung | P3 |
| FR-CMS-002 | Curriculum Management (chương trình, khối, môn, chủ đề, LO) | [Confirmed] | P3 |
| FR-CMS-003 | Content Management: lesson, material, media, question bank, question set, assessment, exercise, learning activity | [Confirmed] | P3 |
| FR-CMS-004 | Game Content: quest, mini-game config, game level, NPC dialogue, reward config, world event, game activity — **chỉ platform** (workspace Game Operations Console, `/api/v1/game-admin/*`) | [Confirmed] (platform-only theo ADR-0027) | P3 |
| FR-CMS-005 | Competition Management: matchmaking config, ranking rules, tournament/challenge template, season, competition reward — **chỉ platform**; tenant tạo giải/thử thách từ template được gán | [Confirmed] (platform-only theo ADR-0027) | P4 |
| FR-CMS-006 | Workflow, versioning, preview (Godot Web export nhúng cho nội dung game), publishing, rollback theo chính sách, audit trail | [Confirmed] / preview Godot [Confirmed] | P3 |
| FR-CMS-007 | CMS không làm thay đổi lịch sử bài đã hoàn thành | [Confirmed] | P3 |
| FR-CMS-008 | Nội dung phạm vi tenant do `tenant_content_editor` soạn, `tenant_content_reviewer` duyệt (OQ-07 đã trả lời bằng ADR-0072) | [Confirmed] | P3 |
| FR-CMS-009 | **Một Admin Portal** React (TypeScript) `src/cms/frontend/apps/portal` với 5 workspace (School & Learning Management, Learning Content Studio, Game Operations Console, Game Live Operations, Platform Administration), đăng nhập bằng form riêng của portal qua module identity (client `eduworld-portal`, ADR-0091) (ADR-0022, ADR-0027 mục 7); giao diện **Admin/Ops UI** cho vận hành game (Ant Design 5 + Ant Design Pro, theme EduWorld), **không** theo phong cách game (ADR-0047) | [Confirmed] React; [Confirmed] TypeScript/Vite | P3 |
| FR-CMS-016 | Dashboard live-ops trong CMS/Admin: CCU, trận đang chạy, hàng chờ ghép trận, đăng nhập/hàng chờ đăng nhập, tin nhắn bị chặn, báo cáo chờ xử lý, lỗi theo module, publish/rollback gần đây, trạng thái SIS sync (DESIGN_SYSTEM §4.3) | [Confirmed] | P3 |
| FR-CMS-017 | Thumbnail/turntable render tự động từ asset 3D của game cho mọi version publish, hiển thị trong CMS như dữ liệu nhận diện (ADR-0047) | [Confirmed] | P3 |
| FR-CMS-018 | **Danh mục menu/action của Admin Portal và bật/tắt trong DB** (ADR-0090): danh mục workspace → menu → action là contract `specs/portal/menu-catalog.yaml` có bản sao đọc-chỉ trong DB, menu portal sinh từ danh mục; **chỉ System Admin** bật/tắt menu/action theo mặc định platform hoặc ghi đè cho một tenant đích (lý do, `If-Match`, audit); mục bảo vệ (người dùng & phân quyền, tenant, bật/tắt menu, nhật ký toàn hệ thống, Thùng rác, case kiểm duyệt và sanction, thông báo an toàn của trường) không tắt được; backend chặn operation của menu/action đang tắt bằng `403 FEATURE_DISABLED`, portal ẩn menu/action đang tắt; chỉ áp cho Admin Portal, không áp cho game client ([ADMIN_PORTAL §2.7](../04-admin-portal/ADMIN_PORTAL.md#27-danh-mục-menuaction-và-bậttắt-adr-0090)) | [Confirmed] ADR-0090 | P1 |
| FR-CMS-010 | Vận hành của mỗi tenant đăng nhập Admin Portal (workspace School & Learning Management, Learning Content Studio) và thao tác chức năng Learning Management trong đúng một tenant | [Confirmed] ADR-0027 | P3 |
| FR-CMS-011 | Vận hành tenant không thấy và không thao tác được menu/cấu hình game; các lớp chặn: gateway nội bộ, `azp` + role platform, permission catalog, bundle (lớp MFA bỏ theo ADR-0094) | [Confirmed] ADR-0027, ADR-0094 | P3 |
| FR-CMS-012 | Menu dựng từ `GET /api/v1/me/permissions`; permission catalog trong code, role tenant không có `game.*`/`competition.config.*`/`platform.*` | [Confirmed] ADR-0027 | P1 |
| FR-CMS-013 | System Admin là người duy nhất xem dữ liệu liên tenant: dropdown tìm kiếm tenant (mặc định "Tất cả", chọn nhiều), cột Tenant, ghi luôn chọn một tenant đích | [Confirmed] ADR-0027 §6 | P3 |
| FR-CMS-014 | Xóa = soft-delete; dữ liệu đã xóa không hiển thị; Thùng rác cho System Admin (Tenant Admin nếu được cấp) | [Confirmed] ADR-0028 / [Confirmed] Tenant Admin | P2 |
| FR-CMS-015 | System Admin gán dữ liệu platform cho tenant (chỉ đọc với tenant) và clone dữ liệu cấu hình/nội dung giữa tenant (không clone dữ liệu học sinh) | [Confirmed] ADR-0029 | P2 (gán) – P3 (clone) |

### 2.27. LMS — `LMS` (§9)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-LMS-001 | Module: Assignment, Lesson Delivery, Assessment, Learning Progress, Learning Analytics, School Learning Dashboard (thay Teacher Dashboard theo ADR-0072); không còn Course, Learning Path, Enrollment, Schedule (ADR-0095) | [Confirmed] (sửa theo ADR-0072, ADR-0095) | P3 |
| FR-LMS-002 | `school_admin`/`academic_manager` giao bài tập, giao đề ôn tập (target: khối của một trường + thêm học sinh khác khối cùng trường − loại học sinh, hoặc danh sách học sinh; `subject_id` trực tiếp, không course — ADR-0095), đặt open/due, số lần làm, `ResultRelease` | [Confirmed] (người giao theo ADR-0072; target theo ADR-0095) | P3 |
| FR-LMS-003 | Nhà trường theo dõi kết quả, tiến độ, kỹ năng yếu theo LO qua School Learning Dashboard (FR-LMS-011) | [Confirmed] (người xem theo ADR-0072) | P3 |
| FR-LMS-004 | Chỉ `school_admin` tổ chức thử thách khối/giải trường từ template; nhóm học tập đã bỏ theo ADR-0095 | [Confirmed] (viết lại theo ADR-0095) | P4 |
| FR-LMS-005 | Phân quyền theo trường và môn (scope `school`, `subject` của UserRole); không có scope lớp theo phân công giảng dạy (ADR-0072) | [Confirmed] (viết lại theo ADR-0072) | P3 |
| FR-LMS-006 | Bài được giao xuất hiện trong game như quest/nhiệm vụ đặc biệt, ở tab "Nhà trường giao" | [Confirmed] | P3 |
| FR-LMS-007 | Nhà trường chỉ **chọn** quest/mini-game đã publish và được gán cho tenant để giao bài; thử thách/giải trường tạo từ template platform, chỉ điền `allowed_params` | [Confirmed] chọn, không cấu hình (ADR-0027) / [Confirmed] `allowed_params` | P2–P4 |
| FR-LMS-011 | **School Learning Dashboard** cho `school_admin`/`academic_manager`: trường → khối → học sinh (ADR-0095), tiến độ bài giao, LO yếu, cờ "làm quá nhanh", báo cáo tham khảo TT27/TT22; đọc rollup (ADR-0062), API `GET /api/v1/lms/dashboards/school` ([SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md), [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md)) | [Confirmed] ADR-0072 / [Confirmed] chỉ số | P3 |
| FR-LMS-012 | Thao tác cá nhân trên bài giao: gia hạn `due_at` (`extend`), miễn (`excuse`), nhắc học sinh chưa làm (`remind`, giới hạn `assignment_remind_limit`); mọi thao tác ghi audit ([SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md)) | [Confirmed] | P3 |
| FR-LMS-013 | Bảng kết quả bài giao (theo khối, không course — ADR-0095) theo điểm tự chấm (không sổ điểm chấm tay, không cột XP/rating) và xuất XLSX/CSV bằng job nền có watermark, chỉ dữ liệu trong quyền, audit; báo cáo tham khảo TT27/TT22 theo FR-ANL-015 ([SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md)) | [Confirmed] | P3 |

### 2.28. Audit & Operations — `AUD`, `OPS` (§15, §17, §18)

| ID | Yêu cầu | Nhãn | Phase |
|---|---|---|---|
| FR-AUD-001 | Audit log cho: đăng nhập admin, thay đổi quyền, truy cập dữ liệu học sinh chéo phạm vi, publish nội dung, kết quả thi/trận, moderation, SIS sync | [Confirmed] | P1+ |
| FR-AUD-002 | Audit log append-only, partition theo tháng, retention theo data inventory ở [DATA_PRIVACY §2](../10-security/DATA_PRIVACY.md#2-data-inventory) (rà soát thời hạn theo luật: Deferred ADR-0082) | [Confirmed] | P1 |
| FR-OPS-001 | Health, readiness, liveness endpoint cho mọi binary | [Confirmed] | P1 |
| FR-OPS-002 | Docker + Docker Compose local; environment example; migration; backup script; deployment docs | [Confirmed] | P1 |
| FR-OPS-003 | Metrics: online players, active sessions, matchmaking queue, match duration, API latency, error rate, chat rate, learning completion, exam submission, DB performance | [Confirmed] | P1–P6 |
| FR-OPS-004 | Không commit secrets; secret qua secret manager/Docker secrets | [Confirmed] | P1 |
| FR-OPS-005 | CI: lint, test, spec lint/breaking check, build Go/Godot/Web theo path | [Confirmed] | P1 |

## 3. Non-functional Requirements

Giá trị số nhất quán với [SYSTEM_ARCHITECTURE §7](../06-architecture/SYSTEM_ARCHITECTURE.md#7-non-functional-baseline). NFR-PERF-001..004, NFR-PERF-011..016, NFR-SCAL-001..002, NFR-SCAL-005, NFR-AVAIL-001..002, NFR-SEC-001, NFR-PRIV-001, NFR-OBS-001 được định nghĩa ở đó và **không lặp lại** dưới đây; bảng sau bổ sung.

| ID | Yêu cầu | Giá trị | Nhãn |
|---|---|---|---|
| NFR-PERF-005 | Thời gian từ mở game đến vào Main Hub (đã đăng nhập, cache ấm) | Theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11 (hạng Tầm trung) | [Confirmed] ADR-0057 |
| NFR-PERF-006 | Kích thước bản cài và gói nội dung theo vùng | Theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11; nội dung vùng tải theo nhu cầu | [Confirmed] ADR-0057 |
| NFR-PERF-007 | Matchmaking wait p90 | ≤ 45 giây khi queue đủ người | [Confirmed] |
| NFR-PERF-008 | Hub sync tick và băng thông | 10 Hz; băng thông theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §10 | [Confirmed] |
| NFR-PERF-017 | Thời gian truy vấn dashboard (School Learning Dashboard, tenant, platform, Game Analytics) tại API | p95 ≤ 2 giây với dữ liệu 2 năm; không truy vấn nào chạy quá 5 giây ([DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md), ADR-0062) | [Confirmed] |
| NFR-PERF-018 | Dashboard không ảnh hưởng database nghiệp vụ | Chỉ đọc bảng rollup trên read replica; job chốt ngày làm tăng p95 ghi primary < 20% | [Confirmed — ADR-0062] |
| NFR-PERF-019 | Ngân sách băng thông cho cả phòng máy (classroom budget): tổng tải đầu tiết của cả lớp, cache LAN tùy chọn theo hash, gói nội dung cài sẵn, tải trước ngoài giờ | Theo [PERFORMANCE_BUDGET §10.1](../08-game-client/PERFORMANCE_BUDGET.md#101-ngân-sách-phòng-máy-classroom-budget-confirmed) | [Confirmed] |
| NFR-SCAL-003 | Scale ngang từng microservice (api, worker) và `realtime` độc lập (ADR-0036) | — | [Confirmed] |
| NFR-SCAL-004 | Bảng lớn (learning_events, chat messages, audit logs, attempt answers) partition theo tháng | — | [Confirmed] cần kế hoạch / [Confirmed] cách làm |
| NFR-AVAIL-003 | Mất Valkey không làm mất dữ liệu học tập; realtime suy giảm có kiểm soát | — | [Confirmed] |
| NFR-AVAIL-004 | Graceful shutdown: drain WS, hoàn tất request, không mất outbox | — | [Confirmed] |
| NFR-SEC-002 | TLS 1.2+ cho mọi kết nối ngoài; mật khẩu/secret không trong log | — | [Confirmed] |
| NFR-SEC-003 | Rate limit theo user & IP tại APISIX; rate limit nghiệp vụ trong Go cho submit, invite, chat | — | [Confirmed] |
| NFR-SEC-004 | Idempotency cho thao tác quan trọng (ADR-0010) | — | [Confirmed] |
| NFR-SEC-005 | Quét lỗ hổng dependency và image trong CI; pentest trước production | — | [Confirmed] |
| NFR-SEC-006 | Tenant isolation test tự động: học sinh/nhân sự trường A không truy cập dữ liệu trường B | 100% endpoint tenant-scoped | [Confirmed] |
| NFR-PRIV-002 | Data minimization: không lưu dữ liệu học sinh không cần cho mục đích học tập | — | [Confirmed] |
| NFR-PRIV-003 | Retention job tự động (gộp/ẩn danh hóa, soft-delete — ADR-0028, ADR-0070) | Thời hạn theo [DATA_PRIVACY §2](../10-security/DATA_PRIVACY.md#2-data-inventory) và [CONFIG_DEFAULTS](CONFIG_DEFAULTS.md); rà soát theo luật: Deferred (ADR-0082) | [Confirmed] |
| NFR-PRIV-004 | **Deferred (ADR-0082)** — đánh giá pháp lý (luật bảo vệ dữ liệu cá nhân, bảo vệ trẻ em, DPIA) trước production không thuộc version này; rủi ro được chấp nhận ghi ở RISKS | — | Deferred |
| NFR-SAFE-001 | 100% tin nhắn (mọi channel, mọi `kind`) qua pipeline kiểm tra hợp lệ phía server trước khi phát; không có đường gửi nào bỏ qua bộ lọc | — | [Confirmed] |
| NFR-SAFE-002 | Pre-send filter p95 ≤ 50 ms | — | [Confirmed] |
| NFR-SAFE-004 | Bước PII và phát hiện grooming chạy trên 100% tin nhắn ở mọi channel, mọi tenant; không cấu hình, công tắc hay biến môi trường nào tắt được (ADR-0078) | 100% | [Confirmed — Technical Architect 2026-11-17] |
| NFR-SAFE-005 | Giờ giới nghiêm platform thực thi ở server: trong `platform_curfew` không có game session mới, realtime ticket mới hay tin chat nào được chấp nhận; phiên đang chạy kết thúc đúng giờ (sai lệch ≤ 1 phút) (ADR-0078) | 0 vi phạm | [Confirmed — Technical Architect 2026-11-17] |
| NFR-SAFE-003 | Không thiết kế cơ chế gây nghiện: không streak phạt, không thưởng đăng nhập liên tục mang tính ép buộc | — | [Confirmed] nguyên tắc / [Confirmed] biện pháp |
| NFR-A11Y-001 | Font tối thiểu 24 px (1080p) cho lớp 1–3; tương phản WCAG AA cho UI chữ; hỗ trợ đọc to câu hỏi | — | [Confirmed] |
| NFR-A11Y-003 | CMS/LMS/Admin đạt WCAG 2.1 AA (tương phản, bàn phím, trình đọc màn hình); kiểm tra tự động khi build token và bằng axe (ADR-0047) | — | [Confirmed] |
| NFR-MAINT-007 | Không hardcode màu/font/kích thước trong web và scene Godot; mọi giá trị giao diện từ design token, CI chặn vi phạm (ADR-0047) | — | [Confirmed] |
| NFR-A11Y-002 | Không yêu cầu phản xạ nhanh cho nội dung học tập bắt buộc; chế độ không giới hạn thời gian cho practice | — | [Confirmed] |
| NFR-I18N-001 | **Song ngữ** `vi-VN` (mặc định) + `en-US`: mọi chuỗi UI game qua file dịch, không hardcode; đổi ngôn ngữ lúc chạy không cần khởi động lại ([ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md)) | — | [Confirmed] |
| NFR-I18N-002 | Văn bản game (tên vùng/NPC/item/quest/hội thoại/sự kiện/chương truyện) bắt buộc đủ `vi` + `en` trước khi publish; nội dung học tập có `content_language`, bản dịch tùy chọn, fallback có nhãn | — | [Confirmed] game · [Confirmed] học tập |
| NFR-I18N-003 | Bố cục chịu chuỗi dài hơn 35%; font phủ đủ tiếng Việt có dấu và Latin; định dạng số/ngày/giờ theo locale | — | [Confirmed] |
| NFR-I18N-004 | Không dịch máy chat giữa người chơi; quick message và tin hệ thống hiển thị theo ngôn ngữ người xem | — | [Confirmed] |
| NFR-OBS-002 | Structured log JSON, trace OTel (lan truyền qua lời gọi facade giữa module và NATS), metrics Prometheus cho mọi binary và APISIX, nhãn `service` (binary) và `module` bắt buộc (ADR-0023, ADR-0074) | — | [Confirmed] |
| NFR-OBS-003 | Client telemetry không chứa PII | — | [Confirmed] |
| NFR-MAINT-001 | Clean Architecture trong mỗi module (domain → application → infrastructure/presentation), kiểm tra bằng linter; module khác chỉ import facade `api/` có trong `x-allowed-callers`, dùng chung `src/backend/pkg/` (ADR-0074, ADR-0038) | — | [Confirmed] stakeholder 2026-11-15 |
| NFR-MAINT-002 | Contract-first: spec public **và internal** trước code, CI chặn breaking change không có version mới (spec public); facade test theo spec internal giữa module (ADR-0074) | — | [Confirmed] |
| NFR-MAINT-003 | Code coverage unit test domain/application layer ≥ 70% mỗi module | — | [Confirmed] |
| NFR-MAINT-004 | CQRS: command thay đổi trạng thái qua aggregate + outbox; query không ghi dữ liệu (ADR-0038) | — | [Confirmed] stakeholder 2026-11-15 |
| NFR-MAINT-005 | **Mọi trường/cột kiểu chuỗi có giới hạn độ dài** ở DB (`varchar(n)`, ent `MaxLen`), API (OpenAPI/AsyncAPI `maxLength`) và JSON Schema; CI chặn thiếu giới hạn; vượt giới hạn trả `422` (ADR-0041) | 100% trường chuỗi | [Confirmed] stakeholder 2026-11-15 |
| NFR-MAINT-006 | Gọi module khác chỉ qua facade sinh từ spec internal, trong tiến trình, có deadline và kiểm `x-allowed-callers`; timeout/retry/circuit breaker chỉ cho tích hợp bên ngoài (ADR-0074, thay token `svc-*` và typed client HTTP của ADR-0040) | — | [Confirmed] stakeholder 2026-11-15 · cập nhật ADR-0074 (2026-11-17) |
| NFR-DEPL-001 | **Triển khai theo binary**: mỗi binary (`core-api`, `realtime`, `worker` theo profile) build, deploy, scale và rollback riêng; migration chạy theo từng database module; hợp đồng giữa binary (lệnh NATS, event) tương thích ngược giữa phiên bản N và N-1 (ADR-0074, thay triển khai độc lập từng service của ADR-0036) | — | [Confirmed] stakeholder 2026-11-15 |
| NFR-DEPL-002 | Mỗi module domain một database riêng (một cụm PostgreSQL); không module nào truy cập database của module khác (ADR-0037, ADR-0074) | — | [Confirmed] stakeholder 2026-11-15 |
| NFR-AVAIL-005 | (Đổi mã 2026-11-15, trước trùng NFR-AVAIL-003) Một module hoặc database module lỗi, một profile worker dừng hay một panic trong `core-api` không làm sập module khác: các chức năng không phụ thuộc vẫn hoạt động; chức năng phụ thuộc suy giảm có thông báo (ADR-0074) | Chaos test TC-SVC-010…013, 016, 017 và diễn tập khôi phục riêng một database TC-SVC-014 pass | [Confirmed] |
| NFR-COMP-001 | **Chạy được trên máy tầm trung** (yêu cầu bắt buộc): mọi tính năng Release 1.0 đạt FPS mục tiêu ở hạng Tầm trung (PC iGPU UHD 630/Vega 8 + 8 GB RAM; Android 4 GB RAM Mali-G52/Adreno 610; iPhone 11; Web WebGL 2 trên PC tầm trung) và chơi được ở hạng Tối thiểu với preset Thấp; hạng, FPS, renderer, preset, ngân sách theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) | AC-PERF-001…011 | [Confirmed] yêu cầu ADR-0057 · thông số [Confirmed] OQ-04 |
| NFR-COMP-003 | iOS 15+ (iPhone 11 / iPad thế hệ 8 trở lên là hạng Tầm trung) | Theo NFR-COMP-001 | [Confirmed] OQ-04 |
| NFR-COMP-004 | Web: Chrome/Edge bản mới bắt buộc (Firefox/Safari không được hỗ trợ trong Release 1.0: không kiểm thử, không chặn truy cập; thiếu WebGL 2 thì hiện trang thông báo — owner 2026-11-20, OQ-04), WebGL 2, renderer Compatibility, preset tối đa Trung bình; tải đầu và bộ nhớ theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §11, §14 | Theo NFR-COMP-001 | [Confirmed] OQ-04 |
| NFR-COMP-002 | Client cũ vẫn hoạt động ≥ 1 học kỳ sau khi ra API v mới | — | [Confirmed] |

## 4. User Stories chính

| ID | Là | Tôi muốn | Để | Phase |
|---|---|---|---|---|
| US-AUTH-001 | Học sinh | đăng nhập bằng username/mật khẩu trường cấp ngay trong game, không phải chọn trường (ADR-0063, ADR-0068, ADR-0069) | vào game mà không cần tạo tài khoản mới | P1 |
| US-AUTH-002 | Học sinh | mở lại game trên máy của mình mà không phải nhập lại mật khẩu khi phiên còn hạn, và có nút Đăng xuất (ADR-0086) | vào học nhanh mà tài khoản vẫn an toàn; máy dùng chung ở trường luôn hỏi mật khẩu | P1 |
| US-AUTH-003 | School Admin | khóa tài khoản học sinh đã nghỉ học | học sinh đó không vào được hệ thống | P1 |
| US-SIS-001 | School Admin | nhập danh sách học sinh (trường, khối) từ SIS (CSV hoặc API) | không phải tạo tay từng tài khoản | P1 |
| US-SIS-002 | School Admin | xem và xử lý các bản ghi xung đột khi đồng bộ | dữ liệu học sinh chính xác | P1 |
| US-GAME-001 | Học sinh lớp 2 | chọn ngoại hình nhân vật bằng hình ảnh lớn, dễ bấm | có nhân vật của riêng mình | P2 |
| US-GAME-002 | Học sinh | đi lại trong Main Hub và đến Learning Zone môn Toán | bắt đầu học theo cách của game | P2 |
| US-QST-001 | Học sinh | nhận nhiệm vụ giúp NPC giải bài toán | học kiến thức thông qua câu chuyện | P2 |
| US-QST-002 | Học sinh | thấy nhiệm vụ hằng ngày phù hợp khối của mình | có mục tiêu ngắn mỗi ngày | P2 |
| US-MG-001 | Học sinh | chơi mini-game nối từ/kéo thả với câu hỏi đúng chương trình | luyện tập vui hơn | P2 |
| US-EXR-001 | Học sinh | làm lại các câu đã sai | củng cố kỹ năng còn yếu | P2 |
| US-EXR-002 | Học sinh | tiếp tục bài đang làm dở khi bị mất mạng | không mất công làm lại | P2 |
| US-GAM-001 | Học sinh | nhận XP và trang phục mới khi hoàn thành quest | có động lực tiếp tục | P2 |
| US-EXM-001 | Học sinh lớp 9 | làm đề thi thử có giới hạn thời gian và xem phân tích lỗi sai | chuẩn bị cho kỳ thi | P3 |
| US-LMS-001 | Academic Manager | giao bộ bài tập chương 3 cho khối 4 của trường (bớt vài học sinh đã học trước), hạn thứ Sáu | học sinh luyện tập đúng trọng tâm | P3 |
| US-LMS-002 | School Admin | xem LO nào khối còn yếu trên School Learning Dashboard | giao thêm bài ôn tập và điều chỉnh kế hoạch của trường | P3 |
| US-LMS-003 | School Admin | xem trang tổng quan trường: bài giao đang mở/sắp hạn, LO cần củng cố, học sinh cần chú ý | biết ngay việc cần làm trong tuần | P3 |
| US-LMS-004 | Academic Manager | theo dõi tiến độ từng bài giao theo khối (đã nộp, chưa bắt đầu, `missed`) | nhắc hoặc gia hạn kịp thời | P3 |
| US-LMS-005 | School Admin | xem hồ sơ học tập của một học sinh trong trường (mastery theo LO, lịch sử bài, gợi ý đã nhận) | hỗ trợ đúng học sinh cần giúp | P3 |
| US-LMS-006 | Academic Manager | giao bài ôn tập cho khối/danh sách học sinh yếu một LO trong 2 thao tác từ bản đồ kỹ năng [Confirmed] | củng cố nhanh LO yếu | P3 |
| US-LMS-007 | Academic Manager | xem phân tích đề: phổ điểm, câu nhiều học sinh sai | điều chỉnh nội dung ôn tập | P3 |
| US-LMS-008 | — | **Removed (ADR-0095)** — nhóm học tập không còn; thử thách khối từ template: US-TRN-001 | — | — |
| US-LMS-009 | School Admin | xem học sinh có tự học theo gợi ý tự động và tiến bộ sau đó không [Confirmed] | đánh giá hiệu quả tự học | P3 |
| US-LMS-010 | Academic Manager | xuất báo cáo kết quả, gồm báo cáo tham khảo TT27/TT22 [Confirmed] | dùng trong họp chuyên môn của trường | P3 |
| US-LMS-011 | Academic Manager | giao quest/mini-game đã publish theo LO yếu [Confirmed] | học sinh ôn tập ngay trong game | P3 |
| US-PRG-001 | Học sinh | nhận gợi ý ôn tập có lời giải thích khi một kỹ năng còn yếu | tự học tiếp mà không cần người chỉ định | P3 |
| US-EXR-003 | Học sinh lớp 3 | bấm "Gợi ý" khi bí và nghe đề bài được đọc to | tự làm được bài mà không bỏ cuộc | P2 |
| US-CMS-001 | Content Admin | soạn câu hỏi, gửi duyệt, xuất bản phiên bản mới | nội dung được kiểm soát chất lượng | P3 |
| US-CMS-002 | Content Reviewer | xem preview câu hỏi như trong game trước khi duyệt | phát hiện lỗi hiển thị | P3 |
| US-PTY-001 | Học sinh A | tạo party và mời bạn B, C, D | cùng làm quest với nhau | P4 |
| US-PTY-002 | Thành viên party | bấm Ready và thấy trạng thái các bạn | biết khi nào cả nhóm sẵn sàng | P4 |
| US-PTY-003 | Leader | chọn hoạt động và bắt đầu phiên chơi chung | cả nhóm vào cùng instance | P4 |
| US-PTY-004 | Thành viên party | tiếp tục chơi khi một bạn bị rớt mạng | không bị gián đoạn cả nhóm | P4 |
| US-MM-001 | Học sinh lớp 5 | tìm đối thủ cùng khối để đấu Toán 1v1 | thi đấu công bằng | P4 |
| US-LDB-001 | Học sinh | xem mục tiêu tập thể tuần của khối mình trong trường (ADR-0095) | biết cả khối đang tiến tới đâu và mình góp phần | P4 |
| US-TRN-001 | School Admin | tổ chức giải đấu kiến thức cấp khối của trường từ template (ADR-0095) | tạo thi đua lành mạnh | P4 |
| US-SOC-001 | Học sinh lớp 7 | gửi lời mời kết bạn tới bạn cùng trường | chơi và học cùng nhau | P5 |
| US-CHAT-001 | Học sinh lớp 2 | gửi tin nhắn nhanh "Cố lên!" cho nhóm | giao tiếp mà không cần gõ chữ | P5 |
| US-CHAT-002 | Học sinh | báo cáo tin nhắn khiến mình khó chịu | được bảo vệ | P5 |
| US-MOD-001 | — | **Removed (ADR-0072)** — xử lý báo cáo theo lớp không còn; thay bởi US-MOD-003, US-MOD-004 | — | — |
| US-MOD-002 | School Admin | cấu hình chính sách chat của trường trong giới hạn platform (như nhau cho mọi lớp, ADR-0020) | phù hợp quy định trường | P5 |
| US-MOD-003 | Safety Moderator | xử lý báo cáo của mọi trường trong một hàng đợi, chỉ thấy tin nhắn thuộc case | phản ứng nhanh và nhất quán giữa các trường | P5 |
| US-MOD-004 | School Admin | nhận thông báo tóm tắt về vụ việc nghiêm trọng của học sinh trường mình | liên hệ phụ huynh kịp thời | P5 |

## 5. Acceptance Criteria chính

Mỗi AC phải được tự động hóa thành test (xem [TEST_STRATEGY](../12-project/TEST_STRATEGY.md)) trừ khi ghi chú "manual".

### 5.1. Authentication (Master Prompt §22)

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-AUTH-001 | Học sinh có bản ghi SIS hợp lệ đăng nhập bằng form trong game (`/auth/game/login`, ADR-0068) và nhận player bootstrap với đúng school, grade (không có class — ADR-0095) | P1 |
| AC-AUTH-002 | Access token hết hạn → client tự refresh, request được thử lại một lần, người dùng không bị đẩy ra | P1 |
| AC-AUTH-003 | Refresh token hết hạn/bị revoke → client quay về màn hình đăng nhập, không crash | P1 |
| AC-AUTH-004 | Bấm "Đăng xuất" (hoặc thoát game trên máy dùng chung) → game session kết thúc, refresh token bị thu hồi, token bị xóa khỏi kho bảo mật và RAM, kết nối WS đóng; mở lại game → form đăng nhập (ADR-0086) | P1 |
| AC-AUTH-005 | User đăng nhập Keycloak nhưng không tồn tại trong SIS/dữ liệu đồng bộ → không tạo tài khoản, trả lỗi `ACCOUNT_NOT_PROVISIONED`, ghi audit | P1 |
| AC-AUTH-006 | Học sinh bị khóa → REST trả 403 `ACCOUNT_LOCKED`, kết nối realtime bị cắt trong ≤ 60 giây | P1 |
| AC-AUTH-007 | Học sinh lên khối hoặc chuyển trường trong tenant → sau sync, bootstrap trả khối/trường mới, lịch sử học tập ghi khối/trường cũ vẫn truy xuất được (ADR-0095) | P1 |
| AC-AUTH-008 | Học sinh chuyển trường → không còn truy cập dữ liệu/channel trường cũ; dữ liệu trường cũ không lộ cho trường mới ngoài phạm vi policy | P1 |
| AC-AUTH-009 | Token có `aud` sai hoặc chữ ký sai bị từ chối cả ở APISIX và backend (test gọi thẳng backend) | P1 |
| AC-AUTH-010 | Token của học sinh tenant A mang claim `organization` của tenant B (giả lập) → backend trả `TENANT_MISMATCH`, có audit; đăng nhập lại bình thường → vào tenant A | P1 |
| AC-AUTH-011 | Đăng nhập không chọn tenant: user thuộc 1 tenant (SIS hoặc tenant chung do platform tạo) → vào tenant đó; thuộc nhiều tenant → `403 TENANT_NOT_SELECTED` kèm danh sách, chọn sau đăng nhập, không nhập lại mật khẩu; không thuộc tenant chung và không thuộc tenant SIS nào → `403 ACCOUNT_NOT_PROVISIONED`, không vào game, phiên Keycloak bị thu hồi, có audit, không tạo bản ghi trong tenant chung | P1 |
| AC-AUTH-012 | Nhân sự (ví dụ `academic_manager`) có UserRole ở 2 tenant: phiên tenant A không đọc được dữ liệu tenant B; đổi sang tenant B không phải nhập lại mật khẩu | P1 |
| AC-AUTH-013 | Học sinh chuyển sang trường thuộc tenant khác: sau SIS sync, phiên cũ bị thu hồi; đăng nhập lại → backend tự xác định tenant mới với lịch sử của chính học sinh. Chuyển trường trong cùng tenant `unit` → phiên và membership không đổi | P1 |
| AC-AUTH-014 | Máy phòng tin học (`device.cfg` `shared_device: true`): không lưu refresh token xuống kho bảo mật (chỉ RAM), không nhớ username; thoát game hoặc 30 phút không hoạt động → đăng xuất; học sinh dùng máy sau phải nhập tài khoản và mật khẩu của mình (ADR-0086 mục 2) | P1 |
| AC-AUTH-015 | School Admin/Tenant Admin/System Admin đăng nhập game → `403 GAME_ACCESS_STUDENT_ONLY`, không tạo game session/login event, có audit | P1 |
| AC-AUTH-016 | Học sinh đang chơi trên máy A đăng nhập máy B → B hiện hộp thoại xác nhận; chọn tiếp tục → A nhận thông báo "đã đăng xuất vì đăng nhập thiết bị khác" và về màn hình đăng nhập; refresh token của A không còn dùng được; chọn hủy → A không bị ảnh hưởng | P1 |
| AC-AUTH-017 | Mỗi lần vào game tạo đúng 1 `game.login_events` và cập nhật `last_login_at`; vận hành tenant xem được lịch sử, không thấy IP | P1 |
| AC-AUTH-019 | Màn đăng nhập game chỉ có username/password (+ ngôn ngữ, "Nhớ tên đăng nhập", hướng dẫn quên mật khẩu); không có bước chọn trường, ô mã trường, nút đăng nhập nhanh (ADR-0063); việc giữ phiên do hệ thống quyết định theo loại thiết bị (ADR-0086), không có ô "Ghi nhớ đăng nhập" | P1 |
| AC-AUTH-020 | Thiết bị cá nhân: thoát game rồi mở lại khi phiên còn hạn → tự vào game, không hiện form (refresh kèm `device_id`, token xoay vòng được ghi đè trong kho bảo mật); phiên hết hạn (idle 7 ngày / tối đa 30 ngày) hoặc bị thu hồi → xóa token, hiện form. Web/máy dùng chung → luôn hiện form; không có access token hay mật khẩu trên đĩa ở mọi nền tảng (ADR-0086) | P1 |
| AC-AUTH-021 | Game đang mở, mất mạng 2 phút rồi có lại → reconnect bằng refresh token, không hỏi mật khẩu; lỗi mạng khi khởi động không xóa token đã lưu (ADR-0086) | P1 |
| AC-AUTH-022 | Log APISIX, log module identity, trace và crash report không chứa mật khẩu mẫu sau khi gọi `POST /auth/game/login` (ADR-0068) | P1 |
| AC-AUTH-023 | Sai mật khẩu 5 lần → tài khoản khóa tạm; lần 6 với mật khẩu đúng vẫn nhận `401 INVALID_CREDENTIALS` cùng thông điệp; sau thời gian chờ đăng nhập được | P1 |
| AC-AUTH-024 | Thiếu `X-Tenant-Id` ở route tenant-scoped → `400 TENANT_HEADER_REQUIRED`; header khác tenant của phiên → `403 TENANT_MISMATCH` + audit (ADR-0065) | P1 |
| AC-AUTH-025 | Học sinh đăng nhập portal → bị chặn ở Keycloak; gọi API portal bằng token học sinh → `403 PORTAL_ACCESS_STAFF_ONLY` (ADR-0066) | P1 |
| AC-AUTH-026 | Tạo tài khoản với username/email đã tồn tại (khác hoa thường, kể cả tenant khác) → `409 USERNAME_TAKEN`/`EMAIL_TAKEN`; SIS gửi trùng (người khác) → issue `duplicate_identifier`; cùng một người → thêm membership (ADR-0067, ADR-0069) | P1 |
| AC-AUTH-027 | Refresh token đã xoay vòng bị dùng lại → `401 REFRESH_TOKEN_REUSED`, cả phiên game bị thu hồi, có audit + security event; refresh với `device_id` khác thiết bị gắn phiên → `401 REFRESH_TOKEN_DEVICE_MISMATCH`, không phát token (ADR-0086 mục 5) | P1 |
| AC-AUTH-028 | Đổi/đặt lại mật khẩu, khóa tài khoản/xử phạt, thu hồi quyền truy cập, dọn học sinh hoặc takeover sang thiết bị khác → phiên game của thiết bị bị thu hồi; lần mở game kế tiếp hiện form đăng nhập (`401 REFRESH_TOKEN_INVALID`) (ADR-0086 mục 5) | P1 |
| AC-AUTH-029 | Portal: tải lại trang khi phiên còn hạn → phiên được khôi phục bằng refresh token trong cookie (`token:refresh`), không đăng nhập lại; quá idle 30 phút hoặc tối đa 10 giờ → về trang `/login` của portal; phiên `platform_staff` quá 8 giờ từ lúc nhập mật khẩu → refresh bị từ chối, về `/login` (ADR-0086 mục 7, ADR-0091 mục 4, ADR-0094) | P1 |
| AC-AUTH-030 | Bộ cài Windows mặc định → `device.cfg` `shared_device: true`, đăng nhập không ghi refresh token vào DPAPI, thoát game = đăng xuất; bộ cài chọn "máy cá nhân" → lưu token và tự vào lại (ADR-0078, ADR-0086) | P1 |
| AC-AUTH-031 | Mở game có refresh token hợp lệ trên mọi nền tảng → hiện "Tiếp tục là <nickname>?" trước khi tạo game session; "Không phải tôi" → `gameLogout`, token bị xóa, hiện form S01 | P1 |
| AC-AUTH-032 | Trong giờ giới nghiêm `platform_curfew`: đăng nhập, vào lại bằng refresh token và tạo game session bị từ chối với mã lỗi giới nghiêm; phiên đang chơi nhận cảnh báo trước 10 phút rồi kết thúc, tiến độ được lưu (ADR-0078) | P2 |
| AC-AUTH-033 | Portal gửi username/email và mật khẩu trong **một** request (không mã xác thực, kể cả tài khoản vận hành platform — ADR-0094); sai mật khẩu hoặc tài khoản không tồn tại đều nhận cùng `401 INVALID_CREDENTIALS` và cùng thông điệp (ADR-0091) | P1 |
| AC-AUTH-034 | Nhân sự đăng nhập lần đầu bằng mật khẩu tạm → `403 ACCOUNT_SETUP_REQUIRED` kèm `setup_ticket`; đổi mật khẩu trên portal rồi đăng nhập lại được bằng mật khẩu mới (không có bước cài ứng dụng xác thực — ADR-0094); `setup_ticket` dùng lại hoặc quá hạn → `401 SETUP_TICKET_INVALID` (ADR-0091) | P1 |
| AC-AUTH-035 | "Quên mật khẩu" luôn trả `202` dù tài khoản có tồn tại hay không; chỉ nhân sự nhận email; link đặt lại dùng một lần và hết hạn theo cấu hình; đặt lại thành công thu hồi mọi phiên portal của người đó (ADR-0091) | P1 |
| AC-AUTH-036 | Refresh token portal chỉ nằm trong cookie `HttpOnly; Secure; SameSite=Strict` (JavaScript không đọc được), access token không có trong `localStorage`/`sessionStorage`; `token:refresh`/`logout` thiếu hoặc sai header `X-EW-CSRF` → `403 CSRF_TOKEN_INVALID` (ADR-0091) | P1 |
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
| AC-PRG-002 | Response có `response_ms` dưới ngưỡng item được gắn `rapid_guess`, không làm đổi mastery và xuất hiện là "làm quá nhanh" trên School Learning Dashboard | P2 |
| AC-EXR-007 | Render payload không chứa `options[].feedback`; sau khi chấm, kết quả trả feedback của phương án đã chọn; gọi `hints:next` lần lượt trả đúng cấp gợi ý, event ghi `hints_used` | P2 |
| AC-EXR-008 | Không tạo được activity `written_response`, `speaking`, `project` trong Release 1.0 (registry từ chối loại ngoài Release) | P2 |

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
| AC-PTY-008 | Leader mất kết nối quá grace period → leader tự chuyển, mọi thành viên nhận event `party.leader.transferred` | P4 |
| AC-PTY-009 | Mỗi thành viên có attempt và learning result riêng; không thành viên nào thấy điểm chi tiết của người khác trừ khi activity cấu hình hiển thị tổng điểm đội | P4 |
| AC-PTY-010 | Người không được mời không join được party private; join khi đã ở party khác bị từ chối `PARTY_ALREADY_IN_PARTY` | P4 |
| AC-PTY-011 | Vượt rate limit mời → 429; mọi kick/transfer/disband có audit log | P4 |
| AC-PTY-012 | Học sinh chấp nhận đồng thời hai lời mời của hai party khác nhau → chỉ một thành công, không có membership trùng | P4 |
| AC-PTY-013 | Tạo party → channel `party` có ngay (always-on), thành viên gửi được văn bản tự do; leader không tắt được chat party | P4 |

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
| AC-CHAT-001 | Tạo channel party/club theo policy; thành viên hợp lệ tham gia được; loại channel `class`/`study_group` bị từ chối (ADR-0095) | P5 |
| AC-CHAT-002 | Gửi/nhận tin nhắn realtime; unread count đúng sau reconnect | P5 |
| AC-CHAT-003 | Cùng một tin nhắn ở cùng loại channel cho cùng quyết định (`allow`/`mask`/`block`) với học sinh lớp 1–3 và lớp 6–9; tin chứa PII bị chặn như nhau | P5 |
| AC-CHAT-004 | Tin nhắn chứa số điện thoại/email/link → bị block hoặc mask theo policy | P5 |
| AC-CHAT-005 | A block B → B không gửi DM, không mời party, không thấy A trong danh sách gợi ý | P5 |
| AC-CHAT-006 | Report tạo ModerationCase trong hàng đợi Safety Team, gắn đúng tenant và trường của học sinh | P5 |
| AC-MOD-020 | Thêm term mới vào danh mục và publish → trong ≤ 60 s tin chứa term (kể cả bản không dấu/teencode) bị xử lý theo action của category, không cần deploy | P5 |
| AC-MOD-021 | System Admin tắt toàn bộ kiểm duyệt cho tenant X (có lý do, `expires_at` ≤ 24 giờ, System Admin thứ hai duyệt) → tin ở tenant X không vi phạm PII/grooming gửi được với decision `unchecked`, vẫn bị giới hạn độ dài/rate limit; tenant Y không đổi; có audit, banner, alert | P5 |
| AC-MOD-030 | Yêu cầu tắt bước PII hoặc grooming ở mọi cấp bị từ chối; khi công tắc tổng tắt, tin có SĐT/mạng xã hội vẫn bị chặn (ADR-0078) | P4 |
| AC-MOD-022 | Tenant Admin không bật/tắt được công tắc kiểm duyệt, không ghi được từ bổ sung và không bật/tắt được category (`PERMISSION_DENIED`); `safety_moderator` bật/tắt category có `allow_tenant_toggle` cho tenant X theo yêu cầu → hiệu lực ≤ 60 s chỉ ở tenant X, có audit | P5 |
| AC-MOD-026 | `safety_moderator` mở case của tenant X đọc được tin nhắn thuộc case (có audit `tenancy.Bypass` lý do `moderation_case`); gọi API chat ngoài module moderation với tenant X → `403`; role tenant không có quyền ghi danh mục từ nhạy cảm | P5 |
| AC-MOD-027 | Case nghiêm trọng gửi thông báo cho School Admin của trường: không chứa nội dung chat thô; trường xác nhận đã liên hệ phụ huynh → case lưu thời điểm xác nhận | P5 |
| AC-CHAT-007 | Không truy cập channel trái phép; học sinh/nhân sự trường A không đọc được tin nhắn trường B (kể cả qua ID đoán) | P5 |
| AC-CHAT-008 | Trận 1v1 bắt đầu → hai người chat được ngay trong channel `match`; trận đội → mỗi đội có channel `team` riêng; tạo party → channel `party` có ngay, người vào sau tự là member | P4 |
| AC-CHAT-009 | Policy tenant/trường không tắt được chat `match`/`team`/`party` (`CHAT_POLICY_ALWAYS_ON`) | P4 |
| AC-CHAT-010 | Tin trong trận đóng ngày D bị purge sau D + `gameplay_chat_retention_days`; tin đã bị báo cáo vẫn xem được trong moderation case sau purge; channel có case mở không bị purge; channel club/DM không bao giờ bị purge | P4 |
| AC-CHAT-011 | Người gửi chỉ thấy trạng thái "đã gửi" sau khi NATS JetStream ack; khi NATS lỗi tin hiện "chưa gửi được" và tự gửi lại, không tạo trùng |
| AC-CHAT-012 | Sửa tin sau 15 phút hoặc thu hồi sau 24 giờ bị từ chối; tin đã thu hồi vẫn báo cáo được và moderator thấy nội dung gốc |
| AC-CHAT-013 | Forward sang channel của tenant khác bị từ chối; GIF chỉ chọn được từ thư viện nội bộ |
| AC-CHAT-014 | Trong party 4 người, người gửi thấy ai đã nhận/đã xem; tìm "nhan vat" ra tin "nhân vật" trong nhóm mình tham gia |
| AC-CHAT-015 | Trong giờ giới nghiêm platform không gửi được tin ở mọi channel; trường chọn preset "chỉ quick message + emote" → text tự do bị từ chối ở mọi channel của trường; tenant `common` không kết bạn/DM/World chat (ADR-0078) | P4 |

### 5.6. Multi-tenancy

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-ORG-001 | Người dùng tenant A không truy cập được bất kỳ tài nguyên nào của tenant B (test tự động trên mọi endpoint tenant-scoped) | P1+ |
| AC-ORG-002 | Academic Manager có UserRole scope `subject` chỉ giao bài và xem dashboard của môn được phân quyền trong trường mình (viết lại theo ADR-0072) | P3 |
| AC-ORG-003 | School Admin chỉ quản lý phạm vi trường được phân quyền | P1 |
| AC-ORG-004 | System Admin quản trị toàn hệ thống và là người duy nhất xem dữ liệu liên tenant (dropdown tenant); mọi truy cập dữ liệu học sinh được audit | P1 |
| AC-CMS-001 | School Admin/Tenant Admin đăng nhập Admin Portal: chỉ thấy workspace B và C, không có menu Game Operations/Live Ops/Platform; gọi `/api/v1/game-admin/*` qua gateway public → không có route; gọi thẳng backend bằng token của mình → `403 PERMISSION_DENIED` + audit | P3 |
| AC-CMS-002 | Người dùng tenant gửi `tenant_ids` của tenant khác → tham số bị bỏ qua, chỉ thấy dữ liệu tenant mình | P3 |
| AC-CMS-003 | System Admin mở danh sách câu hỏi không chọn tenant → thấy mọi tenant, có cột Tenant; tạo bản ghi mà không chọn tenant đích (hoặc phạm vi platform), tức không gửi `X-Tenant-Id` → `400 TENANT_HEADER_REQUIRED` | P3 |
| AC-CMS-004 | Xóa một câu hỏi → biến mất khỏi danh sách, xuất hiện trong Thùng rác; bài đã làm vẫn xem lại được; khôi phục thành công | P2 |
| AC-CMS-005 | System Admin gán question set platform cho tenant A → nhân sự tenant A thấy (chỉ đọc), nhân sự tenant B không thấy; clone question set từ A sang B → B có bản draft riêng, không mang dữ liệu học sinh | P3 |
| AC-CNT-001 | Học sinh A đang ở màn dùng quest v3; CMS publish v4 → A vẫn thấy v3 đến khi rời màn; học sinh B vào màn sau publish thấy v4 |
| AC-CNT-002 | Rollback câu hỏi từ v5 về v4 có lý do → lượt làm bài mới dùng v4; `content_releases` ghi lại; attempt đang làm với v5 không bị đổi |
| AC-CNT-003 | Xem version cũ đã publish ở chế độ chỉ đọc và diff với version hiện hành |
| AC-CNT-004 | Upload ảnh có EXIF GPS → biến thể đã xử lý không còn EXIF; file sai magic bytes bị từ chối `MEDIA_TYPE_NOT_ALLOWED` |
| AC-CNT-005 | **Removed (ADR-0072)** — chấm tự luận theo rubric ngoài Release; re-evaluate hệ thống được kiểm bằng AC-EXR-005 và [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| AC-CNT-006 | Video có câu hỏi chèn tại 01:30 → video dừng đúng mốc; tua tới cuối không làm activity hoàn thành |
| AC-CNT-007 | Mô phỏng đòn bẩy: client báo đạt nhưng action log không đạt → server trả không đạt, ghi integrity signal |
| AC-CNT-008 | Câu hỏi có công thức hiển thị SVG trong game; học sinh trả lời bằng chọn đáp án/nhập số; LaTeX ngoài allowlist bị chặn `MATH_RENDER_FAILED` khi lưu |
| AC-ORG-005 | Bỏ điều kiện tenant trong query (lỗi lập trình giả lập) vẫn bị RLS chặn | P1 |
| AC-ORG-006 | Dữ liệu đã soft-delete không xuất hiện ở bất kỳ API đọc nào; role `<module>_app` chạy `DELETE` → lỗi quyền; CI phát hiện query thiếu `tenant_id`/`deleted_at IS NULL` | P1 |
| AC-ORG-007 | Học sinh tenant chung chỉ thấy nội dung platform được gán cho `common`; không thấy dữ liệu của tenant nào khác | P1 |

### 5.7. Gamification

| ID | Tiêu chí | Phase |
|---|---|---|
| AC-GAM-001 | Làm lại cùng bài đã hoàn thành nhiều lần → XP giảm dần rồi về 0 theo rule | P2 |
| AC-GAM-002 | Vượt cap XP ngày → không cộng thêm, học sinh được thông báo thân thiện | P2 |
| AC-GAM-003 | Replay event `activity.submitted` → XP không cộng trùng | P2 |
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
| Nhà trường cung cấp được danh sách học sinh kèm trường, khối (CSV hoặc API) | [Confirmed] | Không thể provisioning → chặn P1 pilot |
| Trường chấp nhận tài khoản local Keycloak do EduWorld cấp (Release 1.0 không brokering IdP trường, ADR-0063) | [Confirmed — owner 2026-11-20] OQ-02 | Thay đổi login flow cho từng trường |
| Danh sách thiết bị tham chiếu theo hạng (PERFORMANCE_BUDGET §2) khớp thiết bị thực tế ở trường | [Confirmed — owner 2026-11-20] OQ-04 (khảo sát pilot chỉ cập nhật danh sách máy kiểm thử) | Hạ preset mặc định/ngân sách, không loại hạng Tầm trung (ADR-0057) |
| Nội dung pilot (Toán + Tiếng Việt 1–5) do EduWorld tự biên soạn theo LO, không sao chép sách giáo khoa khi chưa có giấy phép | [Confirmed — owner 2026-11-20] OQ-08 | Chậm P2 |
| Chính sách chat mặc định được trường chấp nhận (rà soát pháp lý: Deferred ADR-0082) | [Confirmed — owner 2026-11-20] OQ-09 | Điều chỉnh policy trước P5 |

## 7. Truy vết yêu cầu → tài liệu thiết kế

PRD là danh sách yêu cầu gốc; chi tiết, tham số và AC mở rộng nằm ở tài liệu sở hữu dưới đây. Giá trị mặc định cấu hình được: [CONFIG_DEFAULTS](CONFIG_DEFAULTS.md).

| Domain (mã) | Tài liệu sở hữu |
|---|---|
| GAME (thế giới, nhân vật), QST, GAM/RWD (XP, level, phần thưởng), CHL, ECO, EVT, CMP/RNK/LDB/TRN | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md) (GDD tổng), [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md), [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md), [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md), [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md), [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md), [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md), [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md), [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) |
| CUR, CNT, QB, EXR, EXM, PRG, ANL, MG (activity) | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md), [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md), [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), [QUESTION_BANK](../02-learning/QUESTION_BANK.md), [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md), [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md), [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md), [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md), [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md), [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md), [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md), [ASSESSMENT_REGULATION_PROFILE](../02-learning/ASSESSMENT_REGULATION_PROFILE.md); ANL còn [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md), [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md). Nội dung ngoài version này: [docs/99-deferred](../99-deferred/README.md) (ADR-0082) |
| MP, PTY, MM, SOC, CHAT, MOD, NTF | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md), [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md), [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md), [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md), [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md), [MODERATION](../03-social-multiplayer/MODERATION.md) |
| CMS, LMS | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md), [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md), [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) |
| AUTH, ORG, SIS | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md), [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md), [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md), [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md), [MULTITENANCY](../06-architecture/MULTITENANCY.md) |
| Client game, MG (mini-game) | [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md), [WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md), [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md), [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) |
| Kiến trúc, API, dữ liệu | [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md), [DOMAIN_MODEL](../06-architecture/DOMAIN_MODEL.md), [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md), [API_DESIGN](../07-backend/API_DESIGN.md), [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md), [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) |
| AUD, OPS, NFR-SEC/PRIV/SAFE | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md), [CHILD_SAFETY](../10-security/CHILD_SAFETY.md), [DATA_PRIVACY](../10-security/DATA_PRIVACY.md), [ANTI_CHEAT](../10-security/ANTI_CHEAT.md), [DEPLOYMENT](../11-operations/DEPLOYMENT.md), [MONITORING](../11-operations/MONITORING.md), [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) |
| Kiểm thử, kế hoạch | [TEST_STRATEGY](../12-project/TEST_STRATEGY.md) (§6.0 ánh xạ 20 kịch bản nghiệm thu), [ROADMAP](../12-project/ROADMAP.md) |

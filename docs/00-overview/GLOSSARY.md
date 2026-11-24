# Glossary — Thuật ngữ

> Trạng thái: Draft · Phiên bản: 0.4 · Owner: Technical Writer · Cập nhật: 2026-11-24 (ADR-0101: bỏ Windows, client chỉ Web/Android/iOS) · 2026-11-21 (ADR-0095: bỏ Class, Enrollment, lớp ảo; thêm Khối, Hồ sơ học sinh hiệu lực, Mục tiêu tập thể của khối; channel còn 6 loại) · 2026-11-17 (ADR-0074: thêm Module, Facade, Binary/profile, `src/backend/pkg/modcall`, Spec internal, Facade test; Microservice, Typed client, Service-to-service token đánh dấu Superseded; tenant QA, bão đăng nhập theo `login_capacity_target`)
> Liên quan: [DOMAIN_MODEL](../06-architecture/DOMAIN_MODEL.md) · [CONFIG_DEFAULTS](CONFIG_DEFAULTS.md) · [docs/README](../README.md)

Thuật ngữ dùng thống nhất trong tài liệu, code, API và UI. Cột "Tên trong code" là tên chuẩn cho entity/field/enum. Glossary chỉ định nghĩa; quy tắc và tham số nằm ở tài liệu sở hữu (xem [CONFIG_DEFAULTS](CONFIG_DEFAULTS.md)).

## 1. Tổ chức & định danh

| Thuật ngữ | Tên trong code | Định nghĩa |
|---|---|---|
| Platform | `platform` | Toàn bộ hệ thống EduWorld do nhà vận hành quản lý |
| Tenant | `tenant`, `tenant_id` | **Đơn vị cô lập dữ liệu và đơn vị đăng nhập** (ADR-0005, ADR-0026): một trường độc lập (`tenant_type = school`), một đơn vị nhiều trường như Sở/Phòng GD&ĐT, hệ thống trường (`unit`), hoặc tenant chung (`common`). 1 Tenant = 1 Keycloak Organization. Mọi truy vấn lọc theo `tenant_id`; mỗi phiên đúng một tenant |
| Tenant chung | `common` | Tenant duy nhất chứa học sinh do platform (System Admin) tạo/import có chủ đích; có School ảo "EduWorld chung"; học sinh khai báo khối, không có lớp ảo (ADR-0095). Không tự đưa người dùng vào tenant chung — người không thuộc tenant chung và không thuộc tenant SIS nào bị từ chối `ACCOUNT_NOT_PROVISIONED` (ADR-0026, ADR-0063) |
| Tenant QA | `tenants.is_qa` | Tenant riêng chỉ chứa tài khoản kiểm thử và probe synthetic (vd. `hs.qa.probe`); chỉ System Admin đặt cờ, tenant chung không được đánh dấu; dữ liệu bị loại khỏi analytics và dashboard platform ([DASHBOARD_DATA §3.4](../07-backend/DASHBOARD_DATA.md), [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md)) |
| Đơn vị (Unit) | `tenant_type = unit` | Tenant gồm nhiều trường; thay cho khái niệm "Organization (Tổ chức)" trước đây. "Keycloak Organization" vẫn là tên tính năng của Keycloak |
| School (Trường) | `school`, `school_id` | Trường trong **danh mục trường**, thuộc đúng một tenant và một tỉnh/thành; chiều phân loại/báo cáo, **không** phải khóa cô lập (ADR-0060) |
| Danh mục platform | `catalog.*` | Dữ liệu dùng chung mọi tenant, chỉ System Admin sửa: quốc gia, tỉnh/thành, kế thừa đơn vị hành chính (ADR-0060) |
| Country (Quốc gia) | `country_code` | Mã ISO 3166-1 alpha-2 (vd. `VN`); Release 1.0 chỉ bật Việt Nam |
| Province (Tỉnh/thành) | `province_id`, `code` | Đơn vị hành chính cấp 1 thuộc một quốc gia; Việt Nam có **34 đơn vị** hiện hành (từ 01/7/2025). `code` là mã nội bộ ổn định (vd. `vn.ha-noi`), `official_code` là mã chính thức (chờ đối chiếu) |
| Kế thừa đơn vị hành chính | `province_successions` | Ánh xạ đơn vị cũ (đã sáp nhập/đổi tên) → đơn vị hiện hành, dùng để chuyển dữ liệu cũ |
| Lớp ảo theo khối | — | **Bỏ (ADR-0095)** — tenant chung chỉ có School ảo + khối của học sinh |
| Login sync | — | Đồng bộ dữ liệu học sinh từ SIS khi đăng nhập: không gọi SIS nếu dữ liệu đủ và mới; thiếu dữ liệu → tra cứu chặn ≤ 3 s; dữ liệu cũ → làm mới nền (ADR-0061) |
| Tenant membership | view `identity.tenant_memberships_v` | Tập tenant mà user thuộc về theo dữ liệu (hồ sơ học sinh hiệu lực ở trường của tenant, UserRole của nhân sự — ADR-0095), đúng một dòng mỗi `(tenant_id, user_id)`; dùng để xác định tenant khi đăng nhập |
| Tenant alias | `keycloak_org_alias` | Mã định danh tenant dạng chữ thường (vd. `th-nghiado-a`), là alias của Keycloak Organization; Release 1.0 không client nào xin scope `organization:{alias}` (game: ADR-0068; portal: ADR-0091) |
| Mã tenant (đăng nhập) | `login_code` | Mã ngắn 6 ký tự của tenant để nhập trước khi đăng nhập — **ngoài Release 1.0, mở rộng sau** (ADR-0063: không có ô "nhập mã trường" trên UI; học sinh không chọn trường) |
| Tenant mismatch | `TENANT_MISMATCH` | Lỗi khi tenant đã chọn không nằm trong membership của người đăng nhập; luôn được audit |
| Soft-delete | `deleted_at`, `deleted_by` | "Xóa" trong ứng dụng = đánh dấu đã xóa; ứng dụng không bao giờ xóa vật lý và mặc định không đọc bản ghi đã xóa (ADR-0028) |
| Ẩn danh hóa | `anonymize` | Ghi đè dữ liệu cá nhân, giữ bản ghi — cách đáp ứng nghĩa vụ xóa theo luật khi ứng dụng chỉ soft-delete; Release 1.0 không xóa vật lý (ADR-0028, ADR-0070) |
| Academic Year (Năm học) | `academic_year` | Khoảng thời gian một năm học của trường |
| Class (Lớp) | — | **Bỏ (ADR-0095)** — không còn thực thể lớp học; "lớp 1–3", "lớp 6–9" trong tài liệu là cách gọi **khối** |
| Hồ sơ học sinh hiệu lực | `org.students` (`status = active`, `deleted_at IS NULL`) | Học sinh thuộc **đúng một trường** (`school_id`) với **một khối** (`grade_level`), cập nhật mỗi năm học từ SIS; thay Enrollment, nguồn của membership tenant và `Principal.school_ids` (ADR-0095) |
| Grade level (Khối) | `grade_level` | Số nguyên 1–9; đơn vị tổ chức học sinh trong trường (giao bài, dashboard, party lớp 1–3, mục tiêu tập thể). Dữ liệu học tập ghi khối tại thời điểm sự kiện (ADR-0095) |
| Grade band | `grade_band` | Nhóm khối dùng cho UX, ghép hub/matchmaking, leaderboard: `g1_3`, `g4_5`, `g6_7`, `g8_9`. **Không** dùng cho chat — chat có một bộ quy tắc chung cho mọi khối (ADR-0020) |
| Enrollment | — | **Bỏ (ADR-0095)** — học sinh thuộc trường trực tiếp qua hồ sơ học sinh hiệu lực; mã lỗi `NO_ACTIVE_ENROLLMENT` giữ tên, nghĩa "không có trường/khối hiệu lực" |
| User type | `user_type` | `student`, `staff` (nhân sự; nhân sự platform = `staff` có `primary_tenant_id = NULL`, nhân sự trường = `staff` có `primary_tenant_id`); không có `teacher`, không có `admin` (ADR-0072, ADR-0081) |
| Keycloak User | — | Tài khoản trong Keycloak (định danh xác thực) |
| EduWorld User | `user` | Tài khoản nội bộ EduWorld |
| Account status | `user.status` | `pending`, `active`, `locked`, `deactivated`, `anonymized` (đã ẩn danh hóa theo yêu cầu xóa dữ liệu — [DATA_PRIVACY](../10-security/DATA_PRIVACY.md)) |
| External Identity | `external_identity` | Liên kết `(issuer, subject)` từ token OIDC tới `user` |
| Principal | `Principal` | Đối tượng đại diện người gọi trong một request (user, tenant của phiên, roles, permissions, scopes) |
| Permission | `Permission` | Quyền nguyên tử dạng `<nhóm>.<đối tượng>.<hành động>` (vd. `lms.assignment.write`, `game.quest.write`), **định nghĩa trong code**, có `scope_kind` `tenant`/`platform` (ADR-0066). Role tenant không bao giờ có `game.*`/`competition.config.*`/`platform.*` (ADR-0027) |
| Role | `identity.roles` | Tập permission **lưu trong DB**: role hệ thống seed (`is_system`, không sửa/xóa) hoặc role tùy biến của tenant [Confirmed]; có `role_scope_kind` (ADR-0066) |
| UserRole | `identity.user_roles` | Gán một Role cho một User trong tenant + phạm vi (`platform`/`tenant`/`school`/`subject`; không có scope lớp theo phân công giảng dạy — ADR-0072), có `valid_from/valid_to`, `granted_by`, `source` (`sis`/`manual`); thay tên `RoleAssignment`/`role_assignments` cũ (ADR-0066) |
| X-Tenant-Id | header HTTP | Header **bắt buộc** mang `tenant_id` (UUID) của phiên trên mọi request tenant-scoped; gateway kiểm tra có mặt (`400 TENANT_HEADER_REQUIRED`), `core-api` kiểm tra khớp token + membership (`403 TENANT_MISMATCH`); không bao giờ tin một mình (ADR-0065) |
| Định danh đăng nhập | `username`, `email` | Duy nhất trên SSO (Keycloak) và `identity.users`, không phân biệt hoa thường, nên không trùng trong cùng tenant; một người một tài khoản, nhiều membership (ADR-0067, ADR-0069) |
| Login ticket | `login_ticket` | Vé ngắn hạn (2 phút, một lần) trả kèm `TENANT_NOT_SELECTED` khi học sinh thuộc nhiều tenant, dùng để chọn tenant mà không nhập lại mật khẩu (ADR-0068) |
| Phiên game (giữ đăng nhập) | `game_auth_sessions`, `game_session_sso` | Phiên Keycloak của client `eduworld-game-backend` (idle 7 ngày, tối đa 30 ngày) giữ bằng refresh token xoay vòng, gắn `device_id`; còn hạn thì mở game tự vào lại không nhập mật khẩu (`POST /auth/game/token:refresh` `mode=resume`). Khác **game session** (phiên chơi một thiết bị của module game, ADR-0031/ADR-0083) ([ADR-0086](../adr/ADR-0086-giu-phien-game-bang-refresh-token.md)) |
| Kho bảo mật thiết bị | `SecureStorage` | Kho bảo mật của hệ điều hành nơi game lưu refresh token trên thiết bị cá nhân: Android Keystore, iOS Keychain; Web chỉ giữ trong RAM (ADR-0086, ADR-0101) |
| Máy dùng chung | `shared_device` | Thiết bị cấu hình `device.cfg` `shared_device: true` (phòng tin học): token chỉ trong RAM, không nhớ username, thoát game hoặc 30 phút không hoạt động = đăng xuất, mở lại phải đăng nhập (ADR-0064, ADR-0086) |
| Role scope kind | `role_scope_kind` | `tenant` (gán trong một tenant) hoặc `platform` (chỉ scope platform, đăng nhập Admin Portal `eduworld-portal`; không MFA — ADR-0094) |
| System Admin | `system_admin` | Role platform duy nhất xem/ghi dữ liệu liên tenant (dropdown tenant, audit) — ADR-0027 §6 |
| Tenant Admin | `tenant_admin` | Quản trị một tenant (trường độc lập hoặc đơn vị nhiều trường), chỉ Learning Management |
| School Admin | `school_admin` | Quản trị một trường: SIS, tài khoản, chat policy và khung giờ truy cập cấp trường, giao bài theo khối, thử thách khối/giải trường, School Learning Dashboard (ADR-0072, ADR-0095) |
| Academic Manager | `academic_manager` | Quản lý chuyên môn (scope trường/tenant, có thể theo môn): curriculum, blueprint, giao bài/đề theo khối, School Learning Dashboard, kích hoạt re-evaluate (ADR-0072, ADR-0095) |
| Safety Moderator | `safety_moderator` | Role platform của **Platform Safety Team**: xử lý moderation case và sanction của mọi tenant, đọc chat chỉ trong phạm vi case qua `tenancy.Bypass(ctx, "moderation_case")` có audit, ghi danh mục từ nhạy cảm cấp tenant (ADR-0072) |
| Nhân sự (staff) | `tenant_staff`, `platform_staff` | Người dùng Admin Portal (realm role tương ứng); học sinh không thuộc nhóm này (ADR-0066) |
| Admin Portal | `src/cms/frontend/apps/portal`, client `eduworld-portal` | Ứng dụng quản trị web duy nhất (ADR-0027 mục 7) gồm 5 workspace; trang `/login` do portal tự vẽ, đăng nhập qua module identity (client confidential `eduworld-portal`, ADR-0091), học sinh không đăng nhập được (ADR-0066); menu và workspace hiển thị theo permission và trạng thái bật/tắt trong danh mục menu/action (ADR-0090) |
| Danh mục menu/action portal | `specs/portal/menu-catalog.yaml`, `platform.portal_menu_catalog` | Contract phân cấp workspace → menu (`menu.<workspace>.<menu>`) → action (`action.<workspace>.<menu>.<action>`) của Admin Portal, có bản sao đọc-chỉ trong DB; System Admin bật/tắt menu/action qua `platform.feature_flags` (mặc định platform hoặc ghi đè theo tenant); mục `can_disable = false` không tắt được (ADR-0090) |
| Setup ticket | `setup_ticket` | Vé dùng một lần (10 phút, `portal_setup_ticket_ttl`) trả kèm `403 ACCOUNT_SETUP_REQUIRED` khi tài khoản nhân sự còn bước bắt buộc (đổi mật khẩu tạm); không chứa mật khẩu (ADR-0091) |
| Client đăng nhập game | `eduworld-game-backend` | Client Keycloak **confidential** chỉ bật Direct Access Grants; chỉ module identity giữ secret và gọi khi học sinh đăng nhập bằng form trong game (ADR-0068) |
| Workspace | `workspace` | Vùng chức năng của Admin Portal: School & Learning Management, Learning Content Studio (tenant thấy được), Game Operations Console, Game Live Operations, Platform Administration (chỉ platform). Mã mỗi workspace là chunk lazy-load, không phải lớp bảo mật |
| Learning Management workspace | workspace School & Learning Management + Learning Content Studio | Vùng cho vận hành tenant: giao bài, nội dung học tập của tenant, báo cáo, cài đặt tenant/trường. Không có cấu hình game |
| Game Operations Console (Game Studio) | workspace Game Operations Console, `/api/v1/game-admin/*` | Vùng cấu hình game và thi đấu chỉ dành cho đội platform (quest, mini-game, NPC, reward, season, template giải đấu…) |
| Thùng rác | `trash` | Màn hình/API xem và khôi phục bản ghi đã soft-delete (ADR-0028) |
| Gán cho tenant / Clone | `tenant_resource_assignments` / `tenant-clones` | System Admin cho tenant dùng dữ liệu platform (chỉ đọc) / sao chép dữ liệu cấu hình giữa tenant (ADR-0029) |
| Scope (phạm vi quyền) | `scope_type`, `scope_id` | Platform / Tenant / School / Subject (không có Class — ADR-0072, ADR-0095) |
| JIT Provisioning | — | Tạo tài khoản nội bộ khi đăng nhập lần đầu, nếu khớp bản ghi SIS và policy cho phép |
| SIS | `sis` | Student Information System — hệ thống quản lý học sinh của nhà trường/Sở |
| Reconciliation | — | Đối soát dữ liệu SIS và EduWorld, phát hiện xung đột |
| EduWorld SIS Contract | `contract_v1` | API + DTO **do EduWorld định nghĩa** (`specs/sis/eduworld-sis-contract.v1.yaml`) mà SIS của trường phải triển khai; EduWorld không nhận DTO riêng của SIS (ADR-0021) |
| SIS facade | — | Lớp do trường/nhà cung cấp SIS tự xây, chuyển dữ liệu SIS sang EduWorld SIS Contract; nằm ngoài EduWorld |
| Conformance test kit | — | Bộ test + mock server sinh từ contract, giao cho trường để tự kiểm tra facade trước khi kết nối |

## 2. Học tập

| Thuật ngữ | Tên trong code | Định nghĩa |
|---|---|---|
| Curriculum | `curriculum` | Chương trình học (vd. GDPT 2018), có phiên bản |
| Subject (Môn) | `subject` | Toán, Tiếng Việt, Tiếng Anh, Khoa học... |
| Topic (Chủ đề) | `topic` | Đơn vị kiến thức theo môn × khối, có thể lồng nhau |
| Learning Objective (LO, Mục tiêu học tập) | `learning_objective` | Năng lực/kiến thức cụ thể có thể đánh giá |
| Learning Unit / Lesson / Learning Activity | `learning_unit`, `lesson`, `learning_activity` | Cấu trúc giảng dạy |
| Question / Question Version | `question`, `question_version` | Câu hỏi và phiên bản bất biến của nó |
| Render payload | `render_payload` | Dữ liệu hiển thị câu hỏi gửi cho client, **không chứa đáp án** |
| Answer key | `answer_key` | Đáp án và quy tắc chấm — **chỉ tồn tại phía server** |
| Question Set | `question_set` | Tập câu hỏi tĩnh hoặc quy tắc chọn câu |
| Assessment Blueprint | `assessment_blueprint` | Ma trận đề: cấu trúc, số câu, thời gian, thang điểm |
| Attempt (Lượt làm bài) | `attempt` | Một lần làm bài tập/đề |
| Exercise | `exercise` | Hoạt động luyện tập, không chính thức; chạy trên Activity & Attempt Engine |
| Activity & Attempt Engine | module `learning` | Lõi thực thi hoạt động học tập: registry loại activity, vòng đời attempt, chấm, evaluation ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)) |
| Exam Session | `exam_session` | Một đợt kiểm tra/thi thử có cửa sổ thời gian |
| Learning Event | `learning_event` | Bản ghi bất biến về một hoạt động học tập (xAPI-like); `context.mode` = mode của Attempt hoặc `lesson` |
| Learning Score | — | Điểm số của một attempt |
| Activity Registry | `activity.Registry` | Cơ chế plugin trong module learning: mỗi `activity_type` trong Release (`quiz`, `interactive_exercise`, `minigame`, `video_lesson`, `audio_lesson`, `simulation`) triển khai interface chung (ADR-0048); `written_response`, `speaking`, `project` ngoài Release (ADR-0072) |
| Activity type | `activity_type` | Loại hoạt động học tập; quyết định plugin xử lý, renderer Godot và editor portal |
| Evaluation | `evaluation` | Kết quả chấm tự động của một attempt; re-evaluate do hệ thống (`attempt.re_evaluated`) tạo evaluation mới, đúng một evaluation có hiệu lực |
| Re-evaluate | `attempt.re_evaluated` | Chấm lại tự động khi nội dung có lỗi, do `content_admin`/`academic_manager` kích hoạt, có audit; không có chấm lại thủ công (ADR-0072) |
| Gợi ý (hint) | `hints[]`, `hints_used` | Gợi ý nhiều cấp trong câu hỏi, mở lần lượt bằng `…/hints:next`; số gợi ý đã dùng ghi vào learning event và giảm trọng số mastery |
| Phản hồi theo phương án | `options[].feedback` | Giải thích riêng cho từng phương án nhiễu, chỉ trả sau khi chấm, không có trong render payload |
| Đoán bừa | `rapid_guess` | Cờ cho response có `response_ms` dưới ngưỡng tối thiểu của item; loại khỏi mastery, báo nhà trường là "làm quá nhanh" |
| Bài chẩn đoán | `AttemptMode = diagnostic` | Bài ngắn đầu năm học/đầu chủ đề để khởi tạo mastery và chọn điểm bắt đầu |
| Yêu cầu cần đạt | `yccd_ref` | Mã yêu cầu cần đạt của chương trình GDPT 2018 gắn vào LO (≤ 64 ký tự), kèm `competency_ids[]` |
| School Learning Dashboard | `GET /lms/dashboards/school` | Dashboard học tập cho `school_admin`/`academic_manager` theo trường → khối → học sinh, đọc rollup (ADR-0062, ADR-0072, ADR-0095); kết quả chỉ tham khảo, không thay đánh giá chính thức |
| Rubric, hàng đợi chấm | — | Ngoài version này (ADR-0082) |
| Embedded question (câu hỏi chèn) | `embedded_questions[].cue_id` | Câu hỏi hiện tại một mốc thời gian trong video/audio, video tạm dừng |
| Simulation template | `simulation_templates` | Scene mô phỏng Godot đóng gói sẵn, chỉ tham số hóa từ CMS; kết quả kiểm chứng bằng checker Go (ADR-0052) |
| Checker (mô phỏng) | `simulation.Checker` | Mã Go tính lại kết quả mô phỏng xác định từ tham số + action log |
| MathJax 4 | — | Thư viện hiển thị công thức trên Admin Portal; dùng để render sẵn SVG cho game (ADR-0055) |
| MathLive | `<math-field>` | Trình nhập công thức trên Admin Portal (biên tập viên soạn nội dung); game không nhập công thức |
| mhchem | `\ce{}`, `\pu{}` | Cú pháp LaTeX cho công thức hóa học và đơn vị |
| Math SVG | `math/<sha256>.svg`, `math_ref` | SVG công thức đã render sẵn và sanitize, game hiển thị inline |
| Mastery (Mức thành thạo) | `objective_mastery` | Mức độ nắm vững một LO, tính từ nhiều bằng chứng (ADR-0017) |
| Content status | `content_status` | `draft`, `in_review`, `approved`, `published`, `archived`, `rejected` |
| Release pointer (con trỏ phát hành) | `current_version_id`, `content_releases` | Version mà học sinh nhận khi bắt đầu lượt mới; publish/rollback/unpublish chỉ đổi con trỏ, có lịch sử (ADR-0043) |
| Rollback | `:rollback` | Trỏ con trỏ về một version đã publish trước đó, không tạo version mới, bắt buộc lý do |
| Version pinning (ghim phiên bản) | `level_sessions.content_versions`, `source_version_id`… | Lượt chơi/học đang diễn ra giữ version lúc bắt đầu dù đã có bản mới |
| Content manifest | `content_manifests` | Danh sách item → version hiện hành của một tenant, kèm snapshot URL/sha256; client tải khi khởi động và khi vào màn mới (ETag) |
| Snapshot | `ew:<module>:cv:*`, CDN `content/…` | Bản nội dung đã dựng sẵn của một version, bất biến, dùng để tải nhanh |
| Media asset | `media_assets` | Metadata của file ảnh/âm thanh/video/3D; file nằm ở object storage + CDN, khóa `media/<sha256>/<variant>` (ADR-0044) |

## 3. Game

| Thuật ngữ | Tên trong code | Định nghĩa |
|---|---|---|
| Game Profile | `game_profile` | Hồ sơ game của học sinh: nickname, level, avatar |
| Login event (game) | `game.login_events` | Bản ghi một lần học sinh vào game: thời điểm vào/ra, nền tảng, thiết bị rút gọn, lý do kết thúc (ADR-0031) |
| Last login | `last_login_at` | Lần vào game gần nhất của học sinh; dùng cho báo cáo và worker dọn tài khoản |
| Device ID | `device_id` / header `X-Device-Id` | UUID của một bản cài game, dùng cho quy tắc một thiết bị; không phải định danh phần cứng |
| Takeover | — | Đăng nhập trên thiết bị mới khi thiết bị cũ đang chơi; sau khi học sinh xác nhận, thiết bị cũ bị đăng xuất (ADR-0031) |
| Dọn học sinh không đăng nhập | `student-inactive-cleanup` | Worker soft-delete học sinh không vào game quá số ngày cấu hình (ADR-0032) |
| Nickname | `nickname` | Tên hiển thị trong game; không phải tên thật |
| Thế giới (World) | — | "Quần đảo Tri Thức" (*The Knowledge Archipelago*): toàn bộ thế giới game ([WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md)) |
| Vùng đất (Region) | `region` | 8 vùng có cốt truyện, tên song ngữ [Confirmed — ADR-0056]: Thị trấn Trung Tâm (*EduWorld Central*), Vương quốc Số (*Math Kingdom*), Rừng Ngôn Ngữ (*Language Forest*), Đảo Khoa Học (*Science Island*), Vương quốc Lịch Sử (*History Kingdom*), Đại dương Khám Phá (*Discovery Ocean*), Núi Thử Thách (*Challenge Mountain*), Đấu trường Nhà Vô Địch (*Champion Arena*) |
| Area / Khu chức năng | `world_area`, `area_id` | Khu cụ thể bên trong vùng (vd. `main_hub`, `learning_zone.math`, `exam_zone`, `arena`); bao trọn 7 khu chức năng bắt buộc A–G của Master Prompt gốc |
| Chương truyện | `story_chapter` | Một chương của mạch truyện "Thắp lại Tia Sáng", gắn một vùng, kể bằng chuỗi quest `main`, kết thúc bằng boss |
| Tia Sáng | — | Vật phẩm cốt truyện; thắp lại Tia Sáng làm vùng đất đổi màu theo mastery |
| Discovery point / Collectible | `discovery`, `collectible` | Điểm khám phá và vật sưu tầm trong thế giới, tính vào % hoàn thành bản đồ |
| Cổng gió | `portal` | Điểm dịch chuyển giữa các vùng trên world map |
| Instance | `room` (type `hub_instance`/`private_instance`) | Bản sao của một khu vực chứa một nhóm người chơi giới hạn |
| Quest | `quest_definition`, `quest_version`, `quest_progress` | Nhiệm vụ gồm các bước, liên kết nội dung học tập |
| Mini-game | `minigame_config` | Cấu hình trò chơi nhỏ, lấy câu hỏi từ Learning Engine |
| NPC | `npc` | Nhân vật không phải người chơi |
| Game Session | `game_session` | Phiên chơi từ lúc vào game đến khi thoát |
| Realtime ticket | — | Mã dùng một lần để mở kết nối WebSocket (ADR-0008) |
| XP / Level | `xp_ledger`, `level` | Điểm kinh nghiệm game và cấp độ |
| Reward Rule / Grant | `reward_rule`, `reward_grant` | Quy tắc thưởng và lần trao thưởng |
| Cosmetic | — | Vật phẩm trang trí, không ảnh hưởng học tập |
| Progression Curve | `progression_curve` | Đường cong XP → level cấu hình, có version ([PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)) |
| Danh hiệu cấp độ | `title_code` | Danh hiệu theo khoảng level (Tân binh, Nhà thám hiểm…) |
| World progression | `world_area_unlock`, `exploration_progress` | Mở khóa khu, điểm khám phá, vật sưu tầm, hoàn thành bản đồ |
| Challenge | `challenge`, `challenge_attempt` | Thử thách PvE có luật (time, perfect, survival…), 5 mức độ khó ([CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)) |
| Boss / Encounter | `boss_version`, `boss_encounter` | Boss nhiều phase; một lượt đánh boss solo hoặc party |
| Đóng góp tối thiểu | `min_contribution` | Phần mỗi thành viên phải tự làm để được tính tham gia và nhận thưởng nhóm |
| Xu | `currency_code = xu` | Tiền trong game, kiếm bằng hoạt động, chỉ mua cosmetic; không mua bằng tiền thật (ADR-0049) |
| Sổ cái (ledger) | `xp_ledger`, `currency_ledger`, `item_ledger` | Bảng append-only ghi mọi thay đổi XP, tiền, vật phẩm |
| Collection | `collection` | Bộ sưu tập (pet, knowledge card, cosmetic, badge, exploration, event) |
| Knowledge Card | `knowledge_card` | Thẻ kiến thức nhận khi đạt mastery một LO |
| Achievement | `achievement` | Thành tựu có trigger, điều kiện, tier, có thể ẩn |

## 4. Multiplayer & thi đấu

| Thuật ngữ | Tên trong code | Định nghĩa |
|---|---|---|
| Party | `party` | Nhóm học sinh chơi cùng nhau, có leader |
| Study party (Nhóm cùng học) | `party_type = study_party` | Party tạm thời học cùng của học sinh lớp 6–9 (đổi tên từ `study_group` của module competition — ADR-0095); không phải nhóm học tập LMS (đã bỏ) |
| Party Lobby | — | Màn hình/phòng chờ của party trước khi bắt đầu hoạt động |
| Lobby (match) | `room` type `lobby` | Phòng chờ trước trận đấu (ready check) |
| Room | `room` | Không gian realtime có vòng đời (hub instance, private instance, battle, co-op, lobby) |
| Matchmaking Ticket | `matchmaking_ticket` | Yêu cầu ghép trận nằm trong hàng đợi |
| Match Proposal | — | Đề xuất ghép trận chờ người chơi xác nhận (ready check) |
| Match | `match` | Một trận thi đấu |
| Knowledge Battle | `battle_1v1` / `team_battle` | Đấu kiến thức trả lời câu hỏi |
| Friend duel | `/competition/duels` | Trận đấu giao hữu thách đấu bạn bè, không tính rating |
| Game Rating | `rating` | Chỉ số Glicko-2 (rating, RD, volatility) theo mode/subject/season |
| Competition Points | `competition_points` | Điểm tích lũy cho bảng xếp hạng mùa/giải |
| Season (Mùa) | `season` | Chu kỳ thi đấu có reset/soft-reset |
| Leaderboard | `leaderboard_definition` | Bảng xếp hạng theo phạm vi & khoảng thời gian |
| Mục tiêu tập thể của khối | `grade_goal` | Mục tiêu chung của học sinh một khối trong một trường (thay bảng/mục tiêu lớp): chỉ tiến độ chung, không xếp hạng cá nhân, không tên, ẩn ô đóng góp < 5 học sinh ([COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md), ADR-0095) |
| Tournament / Bracket | `tournament`, `bracket_node` | Giải đấu và nhánh đấu |

## 5. Social & chat

| Thuật ngữ | Tên trong code | Định nghĩa |
|---|---|---|
| Channel | `channel` | Kênh chat: `world`, `area`, `match`, `party`, `team`, `direct` (không có `class`, `study_group`, không có kênh theo khối — ADR-0095) |
| Match channel | `match` | Channel chat tự tạo cho mọi người chơi của một trận (1v1, trận đội, trận giải); đóng khi trận kết thúc |
| Chat gameplay (always-on) | — | Channel `match`, `team` theo trận, `party`: tự tạo, tự join, không tắt được bằng cài đặt tenant/trường (ADR-0020 mục 6) |
| Purge (chat gameplay) | — | Xóa vật lý chat gameplay và World/Area sau thời hạn lưu (`gameplay_chat_retention_days`), sau khi đã giữ bằng chứng moderation; ngoại lệ duy nhất của soft-delete (ADR-0033) |
| Quick Message | `quick_message` | Tin nhắn soạn sẵn, đã duyệt, chọn từ danh sách; mọi học sinh đều dùng được cùng với text tự do |
| Chat Policy | `chat_policy` | Quy tắc ai được gửi gì ở channel nào |
| Moderation Case | `moderation_case` | Hồ sơ xử lý vi phạm; do Platform Safety Team (`safety_moderator`) xử lý tập trung; vụ việc nghiêm trọng gửi thông báo tóm tắt (không có chat thô) cho nhà trường (ADR-0072) |
| Danh mục từ nhạy cảm | `sensitive_term_category`, `sensitive_term`, `sensitive_allowlist` | Danh sách từ/cụm/mẫu nhạy cảm theo nhóm và mức độ, quản lý trong CMS, có phiên bản (`catalog_version`); không gửi xuống client (ADR-0030) |
| Công tắc kiểm duyệt | `moderation_switches` | Bật/tắt toàn bộ (`enabled_all`) hoặc từng bước kiểm tra nội dung chat (`sensitive_terms`, `pii`, `spam`, `bullying`, `async_analysis`) theo platform/tenant; chỉ System Admin (ADR-0030) |
| `unchecked` | — | Quyết định ghi cho tin được gửi khi các bước kiểm tra nội dung đang tắt |
| Sanction | `sanction` | Biện pháp: cảnh báo, mute, hạn chế, khóa chat |
| Block / Mute | `block`, `mute` | Chặn hoàn toàn / tắt thông báo-hiển thị |
| Bạn bè | `friendship`, `friend_request` | Quan hệ hai chiều, mặc định trong cùng trường ([SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)) |
| Club / Guild | `club` | Nhóm bền vững (School Club, Subject Club, Competition Team, Interest Group; không có Study Group — ADR-0095), có vai trò, mục tiêu, chat bền vững; khác Party tạm thời (ADR-0050) |
| PII | — | Personally Identifiable Information — thông tin định danh cá nhân |

## 6. Kỹ thuật

| Thuật ngữ | Định nghĩa |
|---|---|
| Module | Đơn vị mã theo bounded context trong `src/backend/modules/<module>/` (ADR-0074): 5 module domain `identity`, `learning`, `game`, `competition`, `chat`, mỗi module có database riêng, cộng module `realtime` (không DB). Module khác chỉ được import facade `api/` của nó; bên trong là `internal/{domain,application,infrastructure,presentation}` và `module.go` (`Register`) |
| Facade | Interface + DTO công khai duy nhất của một module (`src/backend/modules/<module>/api/`), sinh bằng `make gen` từ `specs/openapi/modules/<module>.internal.v1.yaml`; module khác gọi **trong tiến trình** qua adapter `infrastructure` và `src/backend/pkg/modcall`, chỉ khi có tên trong `x-allowed-callers` (ADR-0074 mục 3) |
| Binary / profile | Ba binary backend (ADR-0074 mục 2): `core-api` (REST của 5 module, handler NATS lệnh realtime), `realtime` (WebSocket, ticket, presence, định tuyến), `worker` (outbox relay, consumer, job). `worker` chọn nhóm việc bằng `--profile` (`events`, `chat`, `jobs`), triển khai thành Swarm service `worker-events`, `worker-chat`, `worker-jobs` ([DEPLOYMENT §3.1](../11-operations/DEPLOYMENT.md#31-binary-và-swarm-service-backend-adr-0074)) |
| `src/backend/pkg/modcall` | Helper gọi facade trong tiến trình: span tracing, deadline (`module_call_deadline`), `TenantScope`, Principal gốc (on-behalf), `Idempotency-Key`, kiểm `x-allowed-callers`, ánh xạ lỗi sang `problem`; thay `src/backend/pkg/svcclient` (ADR-0074) |
| Microservice / Service — **Superseded** | Mô hình 6 service triển khai độc lập của ADR-0036, đã thay bằng modular monolith ([ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md)). Trong tài liệu hiện hành, "service" chỉ còn nghĩa Swarm service (đơn vị triển khai Docker Swarm) hoặc dịch vụ bên ngoài; tên cũ `<x>-service` đọc là module `<x>` |
| Platform kit (`src/backend/pkg/`) | Thư viện Go dùng chung cho mọi module: config, logging, tracing, HTTP middleware, `UnitOfWork`, outbox, NATS/Valkey, `src/backend/pkg/modcall`, auth, testkit (không có mediator — ADR-0087) |
| Clean Architecture | Tổ chức mỗi module thành các tầng Domain → Application → Infrastructure/Presentation; phụ thuộc chỉ hướng vào trong (ADR-0038) |
| Hexagonal / Ports & Adapters | Domain ở trung tâm, giao tiếp ngoài qua port (interface) và adapter; cách hiện thực ranh giới tầng trong Clean Architecture |
| CQRS | Command Query Responsibility Segregation: tách luồng ghi (write context, command) và luồng đọc (read context, query) thành package, port và persistence riêng; presentation gọi handler qua interface có kiểu, không mediator/bus (ADR-0038, ADR-0087) |
| Command | Yêu cầu thay đổi trạng thái; handler trong `internal/application/command/` nạp aggregate qua port `port/write`, ghi DB + outbox trong một `UnitOfWork`, trả ID/version |
| Query | Yêu cầu chỉ đọc; handler trong `internal/application/query/` đọc read model/projection qua port `port/read`, không ghi DB, không phát event, không dùng `UnitOfWork`, có thể dùng replica/cache |
| Write context / Read context | Hai nửa tách biệt của CQRS trong một module: write context (`application/command`, `port/write`, `infrastructure/persistence/write`, primary DB, `UnitOfWork` + outbox) và read context (`application/query`, `port/read`, `infrastructure/persistence/read`, replica/cache, chỉ đọc); ranh giới kiểm bằng `go-arch-lint`/`depguard` (ADR-0087) |
| Unit of work (`UnitOfWork`) | Transaction ent của write context (`src/backend/pkg/entx`): đặt `app.tenant_id`, gom ghi nghiệp vụ và outbox vào một commit; chỉ command handler dùng (ADR-0087) |
| Read model | Bảng/view tối ưu cho màn hình đọc, cập nhật đồng bộ hoặc qua event |
| ent | ORM Go code-first (entgo.io) dùng thay Entity Framework: schema bằng code, query type-safe, interceptor global filter `tenant_id` + `deleted_at`, hook soft-delete (ADR-0039) |
| Atlas | Công cụ versioned migration sinh từ ent schema, có `migrate lint` (ADR-0039) |
| Global query filter | Điều kiện ent tự thêm vào mọi query (`tenant_id`, `deleted_at IS NULL`); chỉ API liên tenant/Thùng rác được bỏ, có audit |
| Typed client — **Superseded** | HTTP client sinh từ OpenAPI internal để gọi service khác (ADR-0040); thay bằng facade gọi trong tiến trình (ADR-0074). Client HTTP chỉ còn ở adapter tích hợp bên ngoài (Keycloak Admin, SIS, object storage) |
| Spec internal | `specs/openapi/modules/<module>.internal.v1.yaml`: hợp đồng facade của module (operation, schema có `maxLength`, `x-allowed-callers`); không phải HTTP endpoint, không qua APISIX (ADR-0074 mục 3). Khi tách module thành service, cùng spec sinh HTTP server/client |
| Service-to-service token — **Superseded** | Token client credentials `svc-<service>` giữa service (ADR-0040), đã bỏ (ADR-0074 mục 5); chỉ còn `svc-identity` để module identity gọi Keycloak Admin API |
| Circuit breaker | Cơ chế ngừng gọi tạm thời một hệ thống bên ngoài đang lỗi (Keycloak Admin, SIS, object storage) để tránh lỗi dây chuyền; không dùng giữa module (`external_api_resilience`, ADR-0074) |
| Saga | Chuỗi bước liên module (database khác nhau) có trạng thái bền vững, mỗi bước idempotent, có thử lại/bù trừ; module identity điều phối qua facade platform (clone, cleanup, merge, anonymize, khôi phục) |
| Projection (`ref_*`) | Bản sao tối thiểu dữ liệu của module khác (tenant, trường, học sinh kèm khối) trong database của module dùng nó, cập nhật qua event (ADR-0037) |
| Eventual consistency | Dữ liệu giữa các module (database khác nhau) khớp nhau sau một độ trễ ngắn, không tức thời |
| Facade test | Test hợp đồng facade theo spec internal (`make test-contract`): bên nhận chạy ví dụ của spec qua facade thật, bên gọi test adapter với fake sinh từ cùng interface; thay contract test HTTP consumer-driven giữa service (ADR-0074) |
| Outbox | Bảng event ghi cùng transaction nghiệp vụ trong database của module, relay ra NATS JetStream bởi profile `worker-events` (ADR-0042, thay transport Valkey Streams của ADR-0019) |
| NATS JetStream | Message streaming bền vững của NATS; dùng làm đường ghi chat trước khi lưu PostgreSQL (ADR-0035) |
| Fan-out | Phát một message tới mọi node/kết nối đang giữ thành viên của channel; chat dùng Valkey Pub/Sub `ew:chat:fanout:<channel_id>` |
| Persister | Worker `chat-persister` đọc JetStream và ghi tin chat vào PostgreSQL theo lô, idempotent |
| Receipt (Sent/Delivered/Read) | Trạng thái tin nhắn tính từ con trỏ `last_delivered_seq`/`last_read_seq` của từng thành viên (ADR-0034) |
| Seq gap | Khoảng trống trong chuỗi `seq` client nhận được (do mất gói fan-out); client bù bằng `chat.channel.sync`/lịch sử |
| Idempotency Key | Khóa để một thao tác lặp lại chỉ có hiệu lực một lần |
| RLS | Row-Level Security của PostgreSQL |
| PKCE | Proof Key for Code Exchange — bảo vệ OIDC cho public client. Release 1.0 **không** client nào dùng: game (ADR-0068) và Admin Portal (ADR-0091) đều đăng nhập qua backend bằng client confidential |
| Featuregate | Middleware `featuregate` trong `src/backend/pkg/httpx` (`core-api`), chỉ áp cho token `azp = eduworld-portal`: chặn operation có `x-portal-action` thuộc action/menu đang tắt bằng `403 FEATURE_DISABLED`; chạy sau xác thực, trước `authz.Require`; không phải lớp authorization (ADR-0090) |
| CSRF double-submit | Bảo vệ CSRF cho cookie refresh token của portal: cookie `__Host-ew_portal_csrf` (JavaScript đọc được) phải trùng header `X-EW-CSRF` ở `token:refresh`/`logout`, sai → `403 CSRF_TOKEN_INVALID` (ADR-0091) |
| JWKS | JSON Web Key Set — khóa công khai để kiểm tra chữ ký JWT |
| Server-authoritative | Server là nguồn quyết định trạng thái/kết quả |
| Interest management | Chỉ gửi cập nhật của thực thể gần người chơi |
| Interpolation | Nội suy vị trí giữa các snapshot để chuyển động mượt |
| CCU | Concurrent users — số người dùng đồng thời |
| Login storm (bão đăng nhập) | Nhiều học sinh đăng nhập cùng lúc đầu giờ học; mục tiêu năng lực thiết kế là tham số `login_capacity_target` ([CONFIG_DEFAULTS](CONFIG_DEFAULTS.md), ADR-0045, ADR-0064) |
| Waiting room (hàng chờ đăng nhập) | Khi vượt ngưỡng, gateway trả `429 + Retry-After`; game hiển thị "Đang vào lớp…" và tự thử lại có jitter |
| Warm-up theo lịch học | Tăng số replica (`docker service scale` qua `ops-scheduler`) 15 phút trước khung giờ cao điểm đã biết (ADR-0024, ADR-0045) |
| Docker Swarm | Chế độ cluster có sẵn của Docker Engine dùng cho staging/production: replica, rolling update, DNS nội bộ, overlay network, secrets (ADR-0024). Hệ thống chưa dùng Kubernetes |
| Stack file | File cú pháp Compose (`deploy/swarm/<env>/*.stack.yml`) mô tả service, replica, placement, network, secret của một môi trường; deploy bằng `docker stack deploy` |
| Overlay network | Mạng ảo của Swarm nối container trên nhiều node; EduWorld tách `edge`, `app`, `data`, `admin`, có mã hóa giữa node |
| Replica / task | Một bản chạy của service trong Swarm; số replica tăng/giảm theo lịch học |
| Mã QR lớp / Thẻ QR / Mật khẩu hình ảnh | Các cách đăng nhập nhanh không cần gõ mật khẩu, qua authenticator Keycloak tùy biến (ADR-0045) — **ngoài Release 1.0, mở rộng sau** (ADR-0063) |
| SIS nguồn | SIS mà bản ghi học sinh được đồng bộ từ đó, xác định bằng khóa `(sis_connection_id, student_external_id)` (ADR-0046) |
| Object storage / CDN | Kho file S3-compatible (floci ở mọi môi trường, ADR-0071) và mạng phân phối nội dung; lưu media và snapshot nội dung bất biến (ADR-0043, ADR-0044) |
| floci | Trình giả lập AWS mã nguồn mở (MIT, image `floci/floci`, cổng 4566) dùng làm object storage S3 cho mọi môi trường: local, CI, staging và production (ADR-0044, ADR-0071; staging/production có cấu hình bắt buộc, backup và giám sát riêng) |
| xAPI / LRS | Experience API / Learning Record Store — chuẩn ghi nhận hoạt động học tập |
| ADR | Architecture Decision Record; mỗi ADR một file trong `docs/adr/` |
| CONFIG_DEFAULTS | Chỉ mục mọi tham số mặc định cấu hình được, trỏ về tài liệu sở hữu ([CONFIG_DEFAULTS](CONFIG_DEFAULTS.md)) |

## 6b. Song ngữ, i18n (ADR-0056)

| Thuật ngữ | Tên trong code | Định nghĩa |
|---|---|---|
| Song ngữ | — | Game hỗ trợ `vi-VN` (mặc định) và `en-US` ([ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md)) |
| Locale | `vi-VN`, `en-US` | Mã ngôn ngữ ở client/HTTP (`Accept-Language`); trong dữ liệu nội dung dùng khóa ngắn `vi`/`en` |
| Văn bản game | `localized_text` | Văn bản hiển thị trong game (tên, hội thoại, quest…), bắt buộc đủ `vi` + `en` |
| Văn bản học tập | `i18nText` | Văn bản nội dung học tập; bắt buộc bản gốc, bản dịch tùy chọn |
| Ngôn ngữ gốc | `content_language` | Ngôn ngữ soạn nội dung học tập (`vi`/`en`) |
| Ngôn ngữ hiển thị | `display_language` | Ngôn ngữ thực tế học sinh đã thấy trong một attempt |
| Trạng thái bản dịch | `translation_status` | `complete` / `partial` / `outdated` |
| Khóa dịch | `ui.<screen>.<element>` | Khóa chuỗi UI trong file gettext `.po` của Godot |
| Hiển thị song song | — | Tên riêng hiển thị đồng thời hai ngôn ngữ, vd "Vương quốc Số · *Math Kingdom*" |
| Pseudo-localization | `xx-PS` | Locale giả kéo dài chuỗi để phát hiện chuỗi hardcode, tràn chữ |
| XLIFF | — | Định dạng file trao đổi bản dịch dùng khi xuất/nhập trong Admin Portal |

## 7. Giao diện (ADR-0047)

| Thuật ngữ | Định nghĩa |
|---|---|
| Design token | Giá trị giao diện có tên (màu, font, cỡ chữ, khoảng cách, bo góc, tham số shader…) khai báo một lần trong `design/` (W3C Design Tokens), sinh tự động cho từng nền tảng |
| Game design system | Hệ thống giao diện của game Godot 3D: Theme Godot, `UiTokens`, token 3D (toon/outline, ánh sáng, VFX, độ hiếm), density theo grade band; nguồn `design/game-tokens/` |
| Admin/Ops UI | Hệ thống giao diện của CMS/LMS/Admin dành cho vận hành game: Ant Design 5 + Ant Design Pro với theme EduWorld (sáng/tối/compact); nguồn `design/admin-tokens/`; không theo phong cách game |
| Game Theme | Resource `Theme` của Godot sinh từ game-tokens (`src/game/resources/ui/theme/eduworld_theme.tres`), áp dụng cho mọi `Control` |
| Mã màu nghiệp vụ | Màu có ý nghĩa nghiệp vụ dùng chung giữa game và CMS: môn học/khu vực, độ hiếm, trạng thái nội dung; nguồn `design/shared-tokens/` |
| Density | Tập token ghi đè mật độ hiển thị: theo grade band ở game, compact ở CMS |
| Thumbnail / Turntable | Ảnh tĩnh / chuỗi ảnh xoay 360° render headless bằng Godot từ asset 3D của game, dùng trong CMS để nhận diện nội dung |
| Renderer parity | Yêu cầu hình ảnh tương đương giữa renderer Mobile, Forward+ và Compatibility (Web) của Godot, kiểm tra bằng ảnh so sánh trong CI |
| Storybook | Trang tài liệu và kiểm thử component web (`src/cms/frontend/packages/ops-ui`) |
| Visual regression | Kiểm thử so sánh ảnh chụp giao diện với ảnh chuẩn để phát hiện thay đổi ngoài ý muốn |

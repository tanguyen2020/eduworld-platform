# Glossary — Thuật ngữ

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: Technical Writer · Cập nhật: 2026-11-15
> Liên quan: [DOMAIN_MODEL](../06-architecture/DOMAIN_MODEL.md) · [CONFIG_DEFAULTS](CONFIG_DEFAULTS.md) · [docs/README](../README.md)

Thuật ngữ dùng thống nhất trong tài liệu, code, API và UI. Cột "Tên trong code" là tên chuẩn cho entity/field/enum. Glossary chỉ định nghĩa; quy tắc và tham số nằm ở tài liệu sở hữu (xem [CONFIG_DEFAULTS](CONFIG_DEFAULTS.md)).

## 1. Tổ chức & định danh

| Thuật ngữ | Tên trong code | Định nghĩa |
|---|---|---|
| Platform | `platform` | Toàn bộ hệ thống EduWorld do nhà vận hành quản lý |
| Tenant | `tenant`, `tenant_id` | **Đơn vị cô lập dữ liệu và đơn vị đăng nhập** (ADR-0005, ADR-0026): một trường độc lập (`tenant_type = school`), một đơn vị nhiều trường như Sở/Phòng GD&ĐT, hệ thống trường (`unit`), hoặc tenant chung (`common`). 1 Tenant = 1 Keycloak Organization. Mọi truy vấn lọc theo `tenant_id`; mỗi phiên đúng một tenant |
| Tenant chung | `common` | Tenant duy nhất chứa học sinh do platform (System Admin) tạo/import có chủ đích; có School ảo "EduWorld chung" và lớp ảo theo khối. Không tự đưa người dùng vào tenant chung — người không thuộc tenant chung và không thuộc tenant SIS nào bị từ chối `ACCOUNT_NOT_PROVISIONED` (ADR-0026, ADR-0063) |
| Đơn vị (Unit) | `tenant_type = unit` | Tenant gồm nhiều trường; thay cho khái niệm "Organization (Tổ chức)" trước đây. "Keycloak Organization" vẫn là tên tính năng của Keycloak |
| School (Trường) | `school`, `school_id` | Trường trong **danh mục trường**, thuộc đúng một tenant và một tỉnh/thành; chiều phân loại/báo cáo, **không** phải khóa cô lập (ADR-0060) |
| Danh mục platform | `catalog.*` | Dữ liệu dùng chung mọi tenant, chỉ System Admin sửa: quốc gia, tỉnh/thành, kế thừa đơn vị hành chính (ADR-0060) |
| Country (Quốc gia) | `country_code` | Mã ISO 3166-1 alpha-2 (vd. `VN`); Release 1.0 chỉ bật Việt Nam |
| Province (Tỉnh/thành) | `province_id`, `code` | Đơn vị hành chính cấp 1 thuộc một quốc gia; Việt Nam có **34 đơn vị** hiện hành (từ 01/7/2025). `code` là mã nội bộ ổn định (vd. `vn.ha-noi`), `official_code` là mã chính thức (chờ đối chiếu) |
| Kế thừa đơn vị hành chính | `province_successions` | Ánh xạ đơn vị cũ (đã sáp nhập/đổi tên) → đơn vị hiện hành, dùng để chuyển dữ liệu cũ |
| Lớp ảo theo khối | `classes.is_virtual` | Lớp "Khối n – chung" của tenant chung, để mọi học sinh luôn thuộc một trường và một lớp |
| Login sync | — | Đồng bộ dữ liệu học sinh từ SIS khi đăng nhập: không gọi SIS nếu dữ liệu đủ và mới; thiếu dữ liệu → tra cứu chặn ≤ 3 s; dữ liệu cũ → làm mới nền (ADR-0061) |
| Tenant membership | view `identity.tenant_memberships_v` | Tập tenant mà user thuộc về theo dữ liệu (enrollment, teaching assignment, UserRole), đúng một dòng mỗi `(tenant_id, user_id)`; dùng để xác định tenant khi đăng nhập |
| Tenant alias | `keycloak_org_alias` | Mã định danh tenant dạng chữ thường (vd. `th-nghiado-a`), dùng trong scope `organization:{alias}` |
| Mã tenant (đăng nhập) | `login_code` | Mã ngắn 6 ký tự của tenant để nhập trước khi đăng nhập — **ngoài Release 1.0, mở rộng sau** (ADR-0063: không có ô "nhập mã trường" trên UI; học sinh không chọn trường) |
| Tenant mismatch | `TENANT_MISMATCH` | Lỗi khi tenant đã chọn không nằm trong membership của người đăng nhập; luôn được audit |
| Soft-delete | `deleted_at`, `deleted_by` | "Xóa" trong ứng dụng = đánh dấu đã xóa; ứng dụng không bao giờ xóa vật lý và mặc định không đọc bản ghi đã xóa (ADR-0028) |
| Ẩn danh hóa | `anonymize` | Ghi đè dữ liệu cá nhân, giữ bản ghi — cách đáp ứng nghĩa vụ xóa theo luật khi ứng dụng chỉ soft-delete; Release 1.0 không xóa vật lý (ADR-0028, ADR-0070) |
| Academic Year (Năm học) | `academic_year` | Khoảng thời gian một năm học của trường |
| Class (Lớp) | `class` | Lớp học thuộc một trường, một năm học, một khối |
| Grade level (Khối lớp) | `grade_level` | Số nguyên 1–9 |
| Grade band | `grade_band` | Nhóm khối dùng cho UX, ghép hub/matchmaking, leaderboard: `g1_3`, `g4_5`, `g6_7`, `g8_9`. **Không** dùng cho chat — chat có một bộ quy tắc chung cho mọi lớp (ADR-0020) |
| Enrollment | `enrollment` | Quan hệ học sinh thuộc lớp trong khoảng thời gian |
| Teaching Assignment | `teaching_assignment` | Giáo viên được phân dạy môn/lớp |
| Keycloak User | — | Tài khoản trong Keycloak (định danh xác thực) |
| EduWorld User | `user` | Tài khoản nội bộ EduWorld |
| Account status | `user.status` | `pending`, `active`, `locked`, `deactivated`, `anonymized` (đã ẩn danh hóa theo yêu cầu xóa dữ liệu — [DATA_PRIVACY](../10-security/DATA_PRIVACY.md)) |
| External Identity | `external_identity` | Liên kết `(issuer, subject)` từ token OIDC tới `user` |
| Principal | `Principal` | Đối tượng đại diện người gọi trong một request (user, tenant của phiên, roles, permissions, scopes) |
| Permission | `Permission` | Quyền nguyên tử dạng `<nhóm>.<đối tượng>.<hành động>` (vd. `lms.assignment.write`, `game.quest.write`), **định nghĩa trong code**, có `scope_kind` `tenant`/`platform` (ADR-0066). Role tenant không bao giờ có `game.*`/`competition.config.*`/`platform.*` (ADR-0027) |
| Role | `identity.roles` | Tập permission **lưu trong DB**: role hệ thống seed (`is_system`, không sửa/xóa) hoặc role tùy biến của tenant [Proposed]; có `role_scope_kind` (ADR-0066) |
| UserRole | `identity.user_roles` | Gán một Role cho một User trong tenant + phạm vi (`platform`/`tenant`/`school`/`class`/`subject`), có `valid_from/valid_to`, `granted_by`, `source` (`sis`/`manual`); thay tên `RoleAssignment`/`role_assignments` cũ (ADR-0066) |
| X-Tenant-Id | header HTTP | Header **bắt buộc** mang `tenant_id` (UUID) của phiên trên mọi request tenant-scoped; gateway kiểm tra có mặt (`400 TENANT_HEADER_REQUIRED`), service kiểm tra khớp token + membership (`403 TENANT_MISMATCH`); không bao giờ tin một mình (ADR-0065) |
| Định danh đăng nhập | `username`, `email` | Duy nhất trên SSO (Keycloak) và `identity.users`, không phân biệt hoa thường, nên không trùng trong cùng tenant; một người một tài khoản, nhiều membership (ADR-0067, ADR-0069) |
| Login ticket | `login_ticket` | Vé ngắn hạn (2 phút, một lần) trả kèm `TENANT_NOT_SELECTED` khi học sinh thuộc nhiều tenant, dùng để chọn tenant mà không nhập lại mật khẩu (ADR-0068) |
| Role scope kind | `role_scope_kind` | `tenant` (gán trong một tenant) hoặc `platform` (chỉ scope platform, đăng nhập Admin Portal `eduworld-portal` có MFA) |
| System Admin | `system_admin` | Role platform duy nhất xem/ghi dữ liệu liên tenant (dropdown tenant, audit) — ADR-0027 §6 |
| Tenant Admin | `tenant_admin` | Quản trị một tenant (trường độc lập hoặc đơn vị nhiều trường), chỉ Learning Management |
| Admin Portal | `web/apps/portal`, client `eduworld-portal` | Ứng dụng quản trị web duy nhất (ADR-0027 mục 7) gồm 5 workspace; màn đăng nhập riêng (theme `eduworld-ops`), học sinh không đăng nhập được (ADR-0066); menu và workspace hiển thị theo permission |
| Client đăng nhập game | `eduworld-game-backend` | Client Keycloak **confidential** chỉ bật Direct Access Grants; chỉ identity-service giữ secret và gọi khi học sinh đăng nhập bằng form trong game (ADR-0068) |
| Workspace | `workspace` | Vùng chức năng của Admin Portal: School & Learning Management, Learning Content Studio (tenant thấy được), Game Operations Console, Game Live Operations, Platform Administration (chỉ platform). Mã mỗi workspace là chunk lazy-load, không phải lớp bảo mật |
| Learning Management workspace | workspace School & Learning Management + Learning Content Studio | Vùng cho vận hành tenant: giao bài, nội dung học tập của tenant, báo cáo, cài đặt tenant/trường. Không có cấu hình game |
| Game Operations Console (Game Studio) | workspace Game Operations Console, `/api/v1/game-admin/*` | Vùng cấu hình game và thi đấu chỉ dành cho đội platform (quest, mini-game, NPC, reward, season, template giải đấu…) |
| Thùng rác | `trash` | Màn hình/API xem và khôi phục bản ghi đã soft-delete (ADR-0028) |
| Gán cho tenant / Clone | `tenant_resource_assignments` / `tenant-clones` | System Admin cho tenant dùng dữ liệu platform (chỉ đọc) / sao chép dữ liệu cấu hình giữa tenant (ADR-0029) |
| Scope (phạm vi quyền) | `scope_type`, `scope_id` | Platform / Tenant / School / Class / Subject |
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
| Activity & Attempt Engine | `learning-service` | Lõi thực thi hoạt động học tập: registry loại activity, vòng đời attempt, chấm, evaluation ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)) |
| Exam Session | `exam_session` | Một đợt kiểm tra/thi thử có cửa sổ thời gian |
| Learning Event | `learning_event` | Bản ghi bất biến về một hoạt động học tập (xAPI-like); `context.mode` = mode của Attempt hoặc `lesson` |
| Learning Score | — | Điểm số của một attempt |
| Activity Registry | `activity.Registry` | Cơ chế plugin trong learning-service: mỗi `activity_type` (quiz, interactive_exercise, written_response, video_lesson, audio_lesson, speaking, simulation, minigame, project) triển khai interface chung (ADR-0048) |
| Activity type | `activity_type` | Loại hoạt động học tập; quyết định plugin xử lý, renderer Godot và editor portal |
| Evaluation | `evaluation` | Kết quả chấm của một attempt (system, giáo viên, gợi ý AI); một attempt có nhiều evaluation, đúng một có hiệu lực |
| Rubric | `rubric`, `rubric_version` | Bộ tiêu chí + mức điểm để chấm tự luận, nói, dự án; tiêu chí gắn learning objective |
| Review queue (hàng đợi chấm) | `review_tasks` | Danh sách bài chờ giáo viên chấm theo phân công lớp × môn |
| Embedded question (câu hỏi chèn) | `embedded_questions[].cue_id` | Câu hỏi hiện tại một mốc thời gian trong video/audio, video tạm dừng |
| Simulation template | `simulation_templates` | Scene mô phỏng Godot đóng gói sẵn, chỉ tham số hóa từ CMS; kết quả kiểm chứng bằng checker Go (ADR-0052) |
| Checker (mô phỏng) | `simulation.Checker` | Mã Go tính lại kết quả mô phỏng xác định từ tham số + action log |
| MathJax 4 | — | Thư viện hiển thị công thức trên Admin Portal; dùng để render sẵn SVG cho game (ADR-0055) |
| MathLive | `<math-field>` | Trình nhập công thức trên Admin Portal (giáo viên soạn, nhận xét); game không nhập công thức |
| mhchem | `\ce{}`, `\pu{}` | Cú pháp LaTeX cho công thức hóa học và đơn vị |
| Math SVG | `math/<sha256>.svg`, `math_ref` | SVG công thức đã render sẵn và sanitize, game hiển thị inline |
| Mastery (Mức thành thạo) | `objective_mastery` | Mức độ nắm vững một LO, tính từ nhiều bằng chứng (ADR-0017) |
| Content status | `content_status` | `draft`, `in_review`, `approved`, `published`, `archived`, `rejected` |
| Release pointer (con trỏ phát hành) | `current_version_id`, `content_releases` | Version mà học sinh nhận khi bắt đầu lượt mới; publish/rollback/unpublish chỉ đổi con trỏ, có lịch sử (ADR-0043) |
| Rollback | `:rollback` | Trỏ con trỏ về một version đã publish trước đó, không tạo version mới, bắt buộc lý do |
| Version pinning (ghim phiên bản) | `level_sessions.content_versions`, `source_version_id`… | Lượt chơi/học đang diễn ra giữ version lúc bắt đầu dù đã có bản mới |
| Content manifest | `content_manifests` | Danh sách item → version hiện hành của một tenant, kèm snapshot URL/sha256; client tải khi khởi động và khi vào màn mới (ETag) |
| Snapshot | `ew:<svc>:cv:*`, CDN `content/…` | Bản nội dung đã dựng sẵn của một version, bất biến, dùng để tải nhanh |
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
| Tournament / Bracket | `tournament`, `bracket_node` | Giải đấu và nhánh đấu |

## 5. Social & chat

| Thuật ngữ | Tên trong code | Định nghĩa |
|---|---|---|
| Channel | `channel` | Kênh chat: `world`, `area`, `match`, `party`, `team`, `class`, `study_group`, `direct` |
| Match channel | `match` | Channel chat tự tạo cho mọi người chơi của một trận (1v1, trận đội, trận giải); đóng khi trận kết thúc |
| Chat gameplay (always-on) | — | Channel `match`, `team` theo trận, `party`: tự tạo, tự join, không tắt được bằng cài đặt trường/lớp (ADR-0020 mục 6) |
| Purge (chat gameplay) | — | Xóa vật lý chat gameplay và World/Area sau thời hạn lưu (`gameplay_chat_retention_days`), sau khi đã giữ bằng chứng moderation; ngoại lệ duy nhất của soft-delete (ADR-0033) |
| Quick Message | `quick_message` | Tin nhắn soạn sẵn, đã duyệt, chọn từ danh sách; mọi học sinh đều dùng được cùng với text tự do |
| Chat Policy | `chat_policy` | Quy tắc ai được gửi gì ở channel nào |
| Moderation Case | `moderation_case` | Hồ sơ xử lý vi phạm |
| Danh mục từ nhạy cảm | `sensitive_term_category`, `sensitive_term`, `sensitive_allowlist` | Danh sách từ/cụm/mẫu nhạy cảm theo nhóm và mức độ, quản lý trong CMS, có phiên bản (`catalog_version`); không gửi xuống client (ADR-0030) |
| Công tắc kiểm duyệt | `moderation_switches` | Bật/tắt toàn bộ (`enabled_all`) hoặc từng bước kiểm tra nội dung chat (`sensitive_terms`, `pii`, `spam`, `bullying`, `async_analysis`) theo platform/tenant; chỉ System Admin (ADR-0030) |
| `unchecked` | — | Quyết định ghi cho tin được gửi khi các bước kiểm tra nội dung đang tắt |
| Sanction | `sanction` | Biện pháp: cảnh báo, mute, hạn chế, khóa chat |
| Block / Mute | `block`, `mute` | Chặn hoàn toàn / tắt thông báo-hiển thị |
| Bạn bè | `friendship`, `friend_request` | Quan hệ hai chiều, mặc định trong cùng trường ([SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)) |
| Club / Guild | `club` | Nhóm bền vững (Study Group, School Club, Subject Club, Competition Team, Interest Group), có vai trò, mục tiêu, chat bền vững; khác Party tạm thời (ADR-0050) |
| PII | — | Personally Identifiable Information — thông tin định danh cá nhân |

## 6. Kỹ thuật

| Thuật ngữ | Định nghĩa |
|---|---|
| Microservice / Service | Một đơn vị triển khai độc lập có database riêng. EduWorld có 6 service: `identity-service`, `learning-service`, `game-service`, `competition-service`, `chat-service`, `realtime-gateway` (ADR-0036). Thay mô hình Modular Monolith của ADR-0001 (đã superseded) |
| Platform kit (`pkg/`) | Thư viện Go dùng chung cho mọi service: config, logging, tracing, mediator CQRS, outbox, NATS/Valkey, typed client, auth, testkit |
| Clean Architecture | Tổ chức mỗi service thành các tầng Domain → Application → Infrastructure/Presentation; phụ thuộc chỉ hướng vào trong (ADR-0038) |
| Hexagonal / Ports & Adapters | Domain ở trung tâm, giao tiếp ngoài qua port (interface) và adapter; cách hiện thực ranh giới tầng trong Clean Architecture |
| CQRS | Command Query Responsibility Segregation: tách luồng ghi (command) và luồng đọc (query) (ADR-0038) |
| Command | Yêu cầu thay đổi trạng thái; đi qua aggregate, ghi DB + outbox trong một transaction, trả ID/version |
| Query | Yêu cầu chỉ đọc; đọc read model/projection, không thay đổi dữ liệu, có thể dùng replica/cache |
| Mediator | Bộ điều phối nội bộ nhận command/query và gọi handler tương ứng qua pipeline (logging, tracing, validation, authorization, idempotency, transaction) |
| Read model | Bảng/view tối ưu cho màn hình đọc, cập nhật đồng bộ hoặc qua event |
| ent | ORM Go code-first (entgo.io) dùng thay Entity Framework: schema bằng code, query type-safe, interceptor global filter `tenant_id` + `deleted_at`, hook soft-delete (ADR-0039) |
| Atlas | Công cụ versioned migration sinh từ ent schema, có `migrate lint` (ADR-0039) |
| Global query filter | Điều kiện ent tự thêm vào mọi query (`tenant_id`, `deleted_at IS NULL`); chỉ API liên tenant/Thùng rác được bỏ, có audit |
| Typed client | HTTP client type-safe sinh từ OpenAPI internal bằng oapi-codegen, dùng để gọi service khác (tương đương Refit), có timeout/retry/circuit breaker (ADR-0040) |
| Internal API | Endpoint `/internal/v1/*` chỉ dành cho service khác gọi, không qua APISIX public, đặc tả trong `specs/openapi/services/<service>.internal.v1.yaml` |
| Service-to-service token | Access token client credentials của Keycloak client `svc-<service>`, audience `svc-<callee>-api`; bên nhận kiểm tra `azp` theo allowlist từng endpoint |
| Circuit breaker | Cơ chế ngừng gọi tạm thời service đích đang lỗi để tránh lỗi dây chuyền |
| Saga | Chuỗi bước liên service có trạng thái bền vững, mỗi bước idempotent, có thử lại/bù trừ; identity-service điều phối (clone, cleanup, merge, anonymize, khôi phục) |
| Projection (`ref_*`) | Bản sao tối thiểu dữ liệu của service khác (tenant, học sinh, lớp) trong database của service dùng nó, cập nhật qua event (ADR-0037) |
| Eventual consistency | Dữ liệu giữa các service khớp nhau sau một độ trễ ngắn, không tức thời |
| Contract test (consumer-driven) | Bên gọi ghi kỳ vọng với API của bên nhận; bên nhận chạy kiểm chứng trong CI để không phá bên gọi |
| Outbox | Bảng event ghi cùng transaction nghiệp vụ trong database của service, relay ra NATS JetStream (ADR-0042, thay transport Valkey Streams của ADR-0019) |
| NATS JetStream | Message streaming bền vững của NATS; dùng làm đường ghi chat trước khi lưu PostgreSQL (ADR-0035) |
| Fan-out | Phát một message tới mọi node/kết nối đang giữ thành viên của channel; chat dùng Valkey Pub/Sub `ew:chat:fanout:<channel_id>` |
| Persister | Worker `chat-persister` đọc JetStream và ghi tin chat vào PostgreSQL theo lô, idempotent |
| Receipt (Sent/Delivered/Read) | Trạng thái tin nhắn tính từ con trỏ `last_delivered_seq`/`last_read_seq` của từng thành viên (ADR-0034) |
| Seq gap | Khoảng trống trong chuỗi `seq` client nhận được (do mất gói fan-out); client bù bằng `chat.sync`/lịch sử |
| Idempotency Key | Khóa để một thao tác lặp lại chỉ có hiệu lực một lần |
| RLS | Row-Level Security của PostgreSQL |
| PKCE | Proof Key for Code Exchange — bảo vệ OIDC cho public client; dùng cho Admin Portal (game đăng nhập qua backend, ADR-0068) |
| JWKS | JSON Web Key Set — khóa công khai để kiểm tra chữ ký JWT |
| Server-authoritative | Server là nguồn quyết định trạng thái/kết quả |
| Interest management | Chỉ gửi cập nhật của thực thể gần người chơi |
| Interpolation | Nội suy vị trí giữa các snapshot để chuyển động mượt |
| CCU | Concurrent users — số người dùng đồng thời |
| Login storm (bão đăng nhập) | Nhiều học sinh đăng nhập cùng lúc đầu giờ học; thiết kế cho 3.000 lượt/10 phút, đỉnh 50 đăng nhập mật khẩu/giây (ADR-0045) |
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
| Game Theme | Resource `Theme` của Godot sinh từ game-tokens (`godot/resources/ui/theme/eduworld_theme.tres`), áp dụng cho mọi `Control` |
| Mã màu nghiệp vụ | Màu có ý nghĩa nghiệp vụ dùng chung giữa game và CMS: môn học/khu vực, độ hiếm, trạng thái nội dung; nguồn `design/shared-tokens/` |
| Density | Tập token ghi đè mật độ hiển thị: theo grade band ở game, compact ở CMS |
| Thumbnail / Turntable | Ảnh tĩnh / chuỗi ảnh xoay 360° render headless bằng Godot từ asset 3D của game, dùng trong CMS để nhận diện nội dung |
| Renderer parity | Yêu cầu hình ảnh tương đương giữa renderer Mobile, Forward+ và Compatibility (Web) của Godot, kiểm tra bằng ảnh so sánh trong CI |
| Storybook | Trang tài liệu và kiểm thử component web (`web/packages/ops-ui`) |
| Visual regression | Kiểm thử so sánh ảnh chụp giao diện với ảnh chuẩn để phát hiện thay đổi ngoài ý muốn |

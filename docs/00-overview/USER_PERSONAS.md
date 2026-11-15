# User Personas

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: Product Manager + UX/UI Designer · Cập nhật: 2026-11-15
> Liên quan: [PRODUCT_VISION](PRODUCT_VISION.md) · [PRODUCT_REQUIREMENTS](PRODUCT_REQUIREMENTS.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [MULTITENANCY](../06-architecture/MULTITENANCY.md)

Personas dưới đây là **giả thuyết [Proposed]** dựa trên Master Prompt và hiểu biết chung về trường phổ thông Việt Nam. Cần kiểm chứng bằng phỏng vấn/khảo sát tại trường pilot trong Phase 0–1 (liên quan OQ-04, OQ-07, OQ-09, OQ-17). Tên nhân vật là hư cấu.

Cột "Vai trò Keycloak" dùng realm roles của ADR-0006; phạm vi chi tiết (school/class/subject) nằm ở `UserRole` trong backend (Role/UserRole lưu DB, permission trong code — ADR-0066). Học sinh chỉ đăng nhập game (form trong game, ADR-0068); nhân sự chỉ đăng nhập Admin Portal (màn đăng nhập riêng, không chọn tenant trước, ADR-0066/0067).

## 1. Tổng quan

| # | Persona | Grade band | Vai trò Keycloak | Kênh chính | Ưu tiên phase |
|---|---|---|---|---|---|
| P-01 | Học sinh lớp 1–3 ("Bé Na") | `g1_3` | `student` | Godot (tablet/phòng máy) | P2 |
| P-02 | Học sinh lớp 4–5 ("Minh") | `g4_5` | `student` | Godot | P2 |
| P-03 | Học sinh lớp 6–7 ("Lan") | `g6_7` | `student` | Godot (điện thoại cá nhân/PC nhà) | P2–P5 |
| P-04 | Học sinh lớp 8–9 ("Huy") | `g8_9` | `student` | Godot | P3–P5 |
| P-05 | Giáo viên chủ nhiệm ("Cô Hạnh") | — | `teacher` | CMS/LMS web | P3 |
| P-06 | Giáo viên bộ môn ("Thầy Sơn") | — | `teacher` | CMS/LMS web | P3 |
| P-07 | Quản trị trường ("Anh Tuấn") | — | `school_admin` | Admin Portal (workspace C, B) | P1 |
| P-11 | Quản trị tenant / đơn vị ("Chị Hoa") | — | `tenant_admin` | Admin Portal (workspace C, B) | P1 |
| P-12 | Game Designer / Live-ops ("Khoa") | — | `game_designer`, `liveops`, `competition_admin` | Admin Portal (workspace A Game Operations Console, E Game Live Operations) | P2–P4 |
| P-08 | Quản trị nội dung / Reviewer ("Chị Mai") | — | `content_admin`, `content_reviewer` | CMS web | P2–P3 |
| P-09 | Academic Manager ("Bà Thu") | — | `academic_manager` | CMS web, báo cáo | P3 |
| P-10 | System Admin ("Đạt") | — | `system_admin` | Admin Portal (mọi workspace, D Platform Administration), Keycloak, Grafana | P1 |

## 2. Học sinh

### P-01 — Học sinh lớp 1–3 (6–9 tuổi)

| Mục | Mô tả |
|---|---|
| Bối cảnh | Bé Na, lớp 2. Đọc còn chậm, quen chơi game đơn giản trên tablet của bố mẹ. Học EduWorld chủ yếu ở phòng máy hoặc tablet dùng chung của lớp, đôi khi ở nhà dưới sự giám sát |
| Mục tiêu | Có nhân vật dễ thương, đi khám phá, giúp NPC, nhận quà; hoàn thành bài cô giao |
| Động lực chơi | Khám phá và câu chuyện cùng Mây; sưu tầm (pet, sticker, vật phẩm); thấy vùng đất đổi màu khi mình học giỏi lên; được khen ngay khi làm đúng. Không cần xếp hạng ([GAME_DESIGN](../01-game-design/GAME_DESIGN.md)) |
| Pain points | Đọc hướng dẫn dài khó; gõ phím chậm; dễ nản khi sai nhiều; không tự đăng nhập được bằng mật khẩu phức tạp |
| Thiết bị | Tablet Android dùng chung, máy tính phòng tin học Windows; tai nghe không phải lúc nào cũng có |
| Ngữ cảnh | Phiên 15–25 phút trong tiết học hoặc ở nhà; giáo viên hướng dẫn tại lớp |
| Nhu cầu UX | Icon lớn, ít chữ, chữ ≥ 24 px, đọc to câu hỏi bằng audio, thao tác chạm/kéo thả, phản hồi bằng hình ảnh & âm thanh vui, không giới hạn thời gian ở practice, điều hướng tối đa 2 cấp |
| Nhu cầu an toàn | Chat theo quy tắc chung (ADR-0020), mọi tin được kiểm tra hợp lệ; quick message giúp giao tiếp khi chưa gõ nhanh; không thấy tên thật người khác; leaderboard công khai có thể tắt; nhắc nghỉ |
| Đăng nhập | Release 1.0: username + password ngắn do giáo viên in phát, nhập mỗi lần mở game; ô tên đăng nhập có thể điền sẵn trên thiết bị riêng; quên mật khẩu thì nhờ thầy cô đặt lại ([ADR-0063](../adr/ADR-0063-dang-nhap-release-1-0-username-password-va-quy-tac-tenant.md), [ADR-0064](../adr/ADR-0064-khong-tu-dang-nhap-khi-mo-lai-game.md), [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md)). QR lớp + PIN, thẻ QR, mật khẩu hình ảnh: mở rộng sau |
| Quyền hạn | Chơi, làm bài, xem tiến độ bản thân, tùy biến nhân vật, chat (text, emoji, quick message) theo policy chung |
| Thành công khi | Tự hoàn thành một quest học tập không cần giáo viên giúp sau 2 buổi làm quen |

### P-02 — Học sinh lớp 4–5 (9–11 tuổi)

| Mục | Mô tả |
|---|---|
| Bối cảnh | Minh, lớp 5. Đọc tốt, thích sưu tầm và thi đua với bạn cùng lớp |
| Mục tiêu | Lên level, mở khóa trang phục, đứng top bảng xếp hạng lớp, chơi cùng bạn thân |
| Động lực chơi | Lên level và danh hiệu; bộ sưu tập; daily/weekly quest; party với bạn thân; boss đầu tiên; thi đua nhẹ (top 10 lớp, không thấy số rating) |
| Pain points | Bài tập lặp lại nhàm; muốn chơi cùng bạn nhưng không biết bạn đang online ở đâu |
| Thiết bị | Phòng máy Windows, tablet/điện thoại của phụ huynh |
| Ngữ cảnh | Ở lớp và ở nhà buổi tối 20–40 phút |
| Nhu cầu UX | Mục tiêu rõ (daily quest), tiến trình trực quan (thanh XP, sao), mini-game đa dạng, party đơn giản bằng lời mời từ danh sách bạn |
| Nhu cầu an toàn | Chat theo quy tắc chung, mọi tin được kiểm tra hợp lệ; chống farm XP tự nhiên (cap) |
| Quyền hạn | Như P-01 + tạo/tham gia party, đấu 1v1 cùng khối, chat theo policy |
| Thành công khi | Có thói quen làm daily quest ≥ 3 ngày/tuần mà không do giáo viên ép |

### P-03 — Học sinh lớp 6–7 (11–13 tuổi)

| Mục | Mô tả |
|---|---|
| Bối cảnh | Lan, lớp 7. Có điện thoại riêng, dùng mạng xã hội, quan tâm đến hình ảnh bản thân trước bạn bè |
| Mục tiêu | Thể hiện cá tính qua avatar, học nhóm với bạn, thắng trận đấu kiến thức |
| Động lực chơi | Cá tính (outfit, emote, danh hiệu); club với bạn; team battle và tier mùa giải; boss party; khám phá bí mật ở Đại dương Khám Phá |
| Pain points | Giao diện "trẻ con" gây ngại; nhiều môn, khó biết nên ôn gì |
| Thiết bị | Điện thoại Android tầm trung (RAM 3–4 GB), PC ở nhà |
| Ngữ cảnh | Tự học ở nhà, nhóm học trực tuyến buổi tối |
| Nhu cầu UX | Giao diện trưởng thành hơn (grade band `g6_7`), nhiều thông tin hơn, gợi ý ôn tập theo LO yếu, club học nhóm, team battle |
| Nhu cầu an toàn | DM chỉ với bạn bè (như mọi lớp); PII filter (chặn SĐT, link mạng xã hội); anti-bullying; report/block dễ tìm |
| Quyền hạn | Như P-02 + kết bạn cùng trường, club (ADR-0050), DM theo policy, team battle |
| Thành công khi | Dùng gợi ý ôn tập và cải thiện ≥ 1 mức mastery ở 3 LO trong một tháng |

### P-04 — Học sinh lớp 8–9 (13–15 tuổi)

| Mục | Mô tả |
|---|---|
| Bối cảnh | Huy, lớp 9, chuẩn bị thi vào lớp 10. Thời gian hạn hẹp, ưu tiên hiệu quả |
| Mục tiêu | Luyện đề, thi thử có giới hạn thời gian, xem phân tích lỗi sai, thi đấu tournament môn học |
| Động lực chơi | Chinh phục thử thách khó (Expert/Legendary), tier và giải đấu, thấy tiến bộ mastery rõ ràng; muốn đi thẳng tới nơi luyện đề |
| Pain points | Không muốn mất thời gian "đi bộ" trong game để tới chỗ luyện đề; lo ngại game làm xao nhãng |
| Thiết bị | PC/laptop, điện thoại |
| Ngữ cảnh | Phiên học tập trung 30–60 phút; tối và cuối tuần |
| Nhu cầu UX | Cổng gió đi thẳng tới Tháp Thử Thách (Núi Thử Thách, [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md)); kết quả chi tiết theo chương/LO; lịch sử đề đã làm; chế độ tập trung giảm hiệu ứng |
| Nhu cầu an toàn | Như P-03; chống áp lực tiêu cực từ xếp hạng (cho phép ẩn hạng của mình) |
| Quyền hạn | Như P-03 + tournament, mock exam |
| Thành công khi | Làm ≥ 1 đề thi thử/tuần và đọc phân tích lỗi sai |

## 3. Giáo viên

### P-05 — Giáo viên chủ nhiệm

| Mục | Mô tả |
|---|---|
| Bối cảnh | Cô Hạnh, chủ nhiệm lớp 3A, dạy nhiều môn ở tiểu học; kỹ năng công nghệ cơ bản |
| Mục tiêu | Giao bài nhanh, biết em nào tụt lại, giữ lớp an toàn khi dùng game |
| Pain points | Ít thời gian; lo học sinh chơi thay vì học; không muốn đọc toàn bộ tin nhắn của học sinh nhưng phải xử lý khi có chuyện |
| Thiết bị | Laptop cá nhân, máy chiếu lớp học; đôi khi điện thoại |
| Ngữ cảnh | Soạn bài buổi tối, theo dõi trong giờ học |
| Nhu cầu UX | Giao bài trong ≤ 3 bước từ bộ đề có sẵn; dashboard lớp một màn hình (ai chưa làm, LO yếu); thông báo report trong lớp; nút mute/khóa chat lớp |
| Nhu cầu an toàn & quyền riêng tư | Chỉ thấy học sinh lớp mình; xem nội dung chat chỉ khi có report/theo policy, mọi truy cập được audit |
| Quyền hạn | `UserRole(teacher, class)`: quản lý lớp chủ nhiệm, giao bài, xem kết quả, moderation channel lớp, tạo nhóm học tập, tổ chức thử thách lớp |
| Thành công khi | Giao ≥ 1 bài/tuần và mở dashboard ≥ 1 lần/tuần |

### P-06 — Giáo viên bộ môn

| Mục | Mô tả |
|---|---|
| Bối cảnh | Thầy Sơn, dạy Toán cho 5 lớp khối 8–9 |
| Mục tiêu | Tạo đề ôn tập theo ma trận, so sánh kết quả giữa các lớp, phát hiện LO yếu theo khối |
| Pain points | Ra đề tốn thời gian; câu hỏi tự soạn khó tái sử dụng |
| Thiết bị | Laptop |
| Nhu cầu UX | Tạo đề từ Assessment Blueprint, chọn câu từ Question Bank có lọc theo LO/độ khó, soạn câu hỏi riêng (phạm vi trường, cần duyệt — OQ-07), báo cáo theo LO × lớp |
| Quyền hạn | `UserRole(teacher, class, subject)` cho các lớp được phân công; tạo nội dung `owner_scope = school` ở trạng thái Draft |
| Thành công khi | Tạo được đề thi thử 40 câu từ ngân hàng trong ≤ 10 phút |

## 4. Quản trị

### P-07 — Quản trị trường (School Admin)

| Mục | Mô tả |
|---|---|
| Bối cảnh | Anh Tuấn, phụ trách CNTT của trường THCS |
| Mục tiêu | Đồng bộ học sinh/lớp từ SIS, quản lý tài khoản, cấu hình chính sách trường |
| Pain points | Dữ liệu SIS không sạch (trùng, thiếu); học sinh chuyển lớp giữa năm; phụ huynh hỏi về an toàn |
| Thiết bị | PC văn phòng |
| Nhu cầu UX | Màn hình SyncRun và ReconciliationIssue rõ ràng; khóa/mở khóa tài khoản; cấu hình chat policy theo khối, giờ truy cập, leaderboard; xuất báo cáo |
| Quyền hạn | `UserRole(school_admin, school)`: toàn quyền Learning Management trong phạm vi một trường của tenant, không thấy tenant khác; **không** thấy menu/cấu hình game (ADR-0027); không đọc chat tự do nếu không có case |
| Thành công khi | Onboard một trường 1.000 học sinh trong ≤ 1 ngày làm việc |

### P-08 — Quản trị nội dung / Reviewer

| Mục | Mô tả |
|---|---|
| Bối cảnh | Chị Mai, biên tập viên nội dung Toán tiểu học thuộc đội EduWorld; đôi khi đóng vai reviewer |
| Mục tiêu | Soạn và xuất bản câu hỏi, question set, quest, mini-game config chất lượng, đúng chương trình |
| Pain points | Không thấy câu hỏi hiển thị thế nào trong game; sửa câu đã dùng sợ hỏng lịch sử |
| Nhu cầu UX | Editor theo loại câu hỏi, preview trong game (Godot Web export nhúng), diff giữa version, workflow review có comment, import hàng loạt |
| Quyền hạn | `content_admin`: tạo/sửa Draft, gửi review, publish (sau Approved) phạm vi platform; `content_reviewer`: Approve/Reject, không tự duyệt nội dung của mình |
| Thành công khi | Xuất bản 100 câu hỏi/tuần với tỷ lệ bị báo lỗi sau publish < 2% |

### P-09 — Academic Manager

| Mục | Mô tả |
|---|---|
| Bối cảnh | Bà Thu, quản lý chuyên môn của một tenant (tổ chuyên môn trường, hoặc Phòng GD / hệ thống trường là tenant `unit`) |
| Mục tiêu | Quản lý khung chương trình, LO, ma trận đề chuẩn; theo dõi chất lượng học tập tổng hợp |
| Pain points | Khó so sánh giữa trường; dữ liệu không theo LO |
| Nhu cầu UX | Curriculum editor, báo cáo tổng hợp theo trường/khối/LO (dữ liệu tổng hợp, không danh tính khi không cần) |
| Quyền hạn | `UserRole(academic_manager, tenant)` (tổ chuyên môn của trường hoặc đơn vị): curriculum riêng & blueprint của tenant, báo cáo tổng hợp trong tenant; không truy cập chat, không thấy cấu hình game |
| Thành công khi | Ban hành blueprint chung cho kỳ thi thử toàn tổ chức |

### P-11 — Quản trị tenant / đơn vị (Tenant Admin)

| Mục | Mô tả |
|---|---|
| Bối cảnh | Chị Hoa, chuyên viên Phòng GD quản lý 25 trường tiểu học là một tenant `unit`; hoặc hiệu phó một trường độc lập (tenant `school`) |
| Mục tiêu | Vận hành Learning Management cho cả tenant: cài đặt tenant, phân quyền School Admin/giáo viên, đồng bộ SIS, nội dung học tập của tenant, báo cáo theo trường |
| Pain points | Phải làm việc trên nhiều trường; sợ thao tác nhầm làm mất dữ liệu; muốn dùng lại bộ câu hỏi chuẩn |
| Thiết bị | PC văn phòng |
| Nhu cầu UX | Chọn trường trong đơn vị; dùng nội dung "Từ EduWorld" được gán (chỉ đọc); Thùng rác để khôi phục khi xóa nhầm [Proposed]; không có dropdown tenant (luôn một tenant) |
| Quyền hạn | `UserRole(tenant_admin, tenant)` (quản lý Role/UserRole trong tenant, ADR-0066): toàn bộ Learning Management trong tenant; **không** thấy tenant khác, menu Game Studio hay cấu hình thi đấu (ADR-0027) |
| Thành công khi | Onboard một đơn vị 25 trường trong ≤ 3 ngày làm việc |

### P-12 — Game Designer / Live-ops (đội EduWorld)

| Mục | Mô tả |
|---|---|
| Bối cảnh | Khoa, game designer của đội EduWorld |
| Mục tiêu | Thiết kế quest, mini-game, phần thưởng, sự kiện thế giới, mùa giải và template giải đấu áp dụng cho mọi tenant |
| Pain points | Cần preview trong game; thay đổi phải công bằng cho mọi trường |
| Thiết bị | Laptop, VPN + MFA |
| Nhu cầu UX | Game Operations Console (workspace A) và Game Live Operations (workspace E) trong Admin Portal, preview Godot Web, workflow duyệt lane `platform_game` |
| Quyền hạn | `game_designer`/`liveops`/`competition_admin` (role platform): chỉ dữ liệu platform (`tenant_id = NULL`); **không** xem dữ liệu học sinh/tenant |
| Thành công khi | Ra một sự kiện thế giới mới mỗi tháng không cần build lại client |

### P-10 — System Admin

| Mục | Mô tả |
|---|---|
| Bối cảnh | Đạt, kỹ sư vận hành EduWorld |
| Mục tiêu | Hệ thống ổn định, onboard tổ chức/trường mới, cấu hình Keycloak/IdP, xử lý sự cố |
| Pain points | Sự cố giờ cao điểm (sáng thứ Hai, giờ ra chơi); yêu cầu truy cập dữ liệu nhạy cảm cần kiểm soát |
| Thiết bị | Laptop, truy cập qua VPN/mạng quản trị |
| Nhu cầu UX | Admin Portal (`eduworld-portal`, MFA bắt buộc với role platform; workspace Platform Administration), Grafana dashboard, runbook, công cụ tạo Tenant/School, gán/clone dữ liệu giữa tenant, thùng rác, cấu hình SIS connection |
| Quyền hạn | `system_admin`: **người duy nhất xem dữ liệu liên tenant** (dropdown tìm kiếm tenant, mặc định "Tất cả tenant", chọn nhiều; cột Tenant; ghi luôn chọn một tenant đích), quản lý tenant, gán dữ liệu platform cho tenant và clone giữa tenant (ADR-0029), Thùng rác mọi tenant (ADR-0028), Game Studio; mọi truy cập dữ liệu tenant có audit. Truy cập dữ liệu học sinh cần lý do và được audit (break-glass) |
| Thành công khi | Onboard tổ chức mới không cần sửa code; MTTR sự cố P1 ≤ 1 giờ |

## 5. Ma trận quyền tóm tắt

Chi tiết ở [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) và [MULTITENANCY](../06-architecture/MULTITENANCY.md).

| Hành động | HS | GV chủ nhiệm | GV bộ môn | School Admin | Content Admin | Reviewer | Academic Mgr | System Admin |
|---|---|---|---|---|---|---|---|---|
| Xem dữ liệu nhiều tenant | — | — | — | — | — | — | — | ✔ (duy nhất, audit) |
| Cấu hình game / thi đấu (Game Studio) | — | — | — | ✘ | — | — | ✘ | ✔ (+ Game Designer/Live-ops/Competition Admin) |
| Gán dữ liệu cho tenant / clone giữa tenant | — | — | — | ✘ | — | — | ✘ | ✔ |
| Thùng rác | — | — | — | ✘ (Tenant Admin: tenant mình [Proposed]) | — | — | ✘ | ✔ mọi tenant |
| Chơi game, làm bài | ✔ | — | — | — | — | — | — | — |
| Xem kết quả bản thân | ✔ | — | — | — | — | — | — | — |
| Xem kết quả học sinh | — | Lớp mình | Lớp × môn được phân | Trường | — | — | Tổng hợp tenant | Theo policy + audit |
| Giao bài | — | ✔ | ✔ | ✔ | — | — | — | — |
| Soạn nội dung | — | Phạm vi trường [Proposed] | Phạm vi trường [Proposed] | Trường (trong tenant) | Platform | — | Tenant | ✔ |
| Duyệt nội dung | — | — | — | Phạm vi trường [Proposed] | — | ✔ (không tự duyệt) | Tenant | ✔ |
| Cấu hình chat policy | — | Lớp (siết chặt hơn) | — | Trường | — | — | — | Platform |
| Xử lý report chat | — | Lớp mình | Lớp dạy [Proposed] | Trường | — | — | — | Escalation |
| Đồng bộ SIS / khóa tài khoản | — | — | — | Trường | — | — | — | ✔ |
| Cấu hình Keycloak/IdP | — | — | — | — | — | — | — | ✔ (system_admin) |

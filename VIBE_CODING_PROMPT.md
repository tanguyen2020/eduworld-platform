<!--
VIBE_CODING_PROMPT.md — Lưu trữ nguyên văn yêu cầu gốc của dự án EduWorld.
Ngày lưu: 2026-11-15.
Mục đích: là nguồn yêu cầu gốc ([Confirmed]) cho PRD, ADR và toàn bộ tài liệu trong docs/.
Không chỉnh sửa nội dung bên dưới. Mọi diễn giải, giả định và quyết định kỹ thuật cụ thể
nằm ở docs/adr/ (ADR), docs/00-overview/PRODUCT_REQUIREMENTS.md
và docs/12-project/OPEN_QUESTIONS.md.
THỨ TỰ ƯU TIÊN KHI MÂU THUẪN (dành cho AI coding agent): AGENTS.md > ADR Accepted > tài liệu sở hữu trong docs/
và specs/ > file này. File này là lịch sử yêu cầu, KHÔNG phải chỉ dẫn triển khai: nhiều điểm đã được ADR thay đổi
(ví dụ: không làm MVP mà làm Release 1.0 đầy đủ, đăng nhập game qua form + backend, không voice chat, floci mọi môi trường).
Bắt đầu từ AGENTS.md. Thay đổi yêu cầu gốc phải được ghi thành phiên bản mới
của file này kèm mục CHANGELOG.
File gồm hai phần: (1) Master Vibe Coding Prompt v1.0, (2) Addendum — Party, Group & Cooperative Multiplayer.
-->

# MASTER VIBE CODING PROMPT — EDUWORLD
## 3D Multiplayer Educational Game & Learning Platform

**Phiên bản:** 1.0  
**Tên dự án:** EduWorld  
**Đối tượng:** Học sinh lớp 1–9  
**Ngôn ngữ mặc định:** Tiếng Việt  
**Game Engine:** Godot 4.x (3D)  
**Backend:** Golang  
**Identity & SSO:** Keycloak  
**API Gateway:** Apache APISIX  
**Cache / Realtime Infrastructure:** Valkey  
**SIS Integration:** External Student Information System

---

# PHẦN 1. VAI TRÒ CỦA AI CODING AGENT

Bạn là một đội ngũ phát triển sản phẩm EdTech/Game chuyên nghiệp, bao gồm các vai trò:

1. Product Manager – phân tích nghiệp vụ và định hướng sản phẩm.
2. Solution Architect – thiết kế kiến trúc tổng thể.
3. Game Director – thiết kế gameplay và trải nghiệm game 3D.
4. Senior Godot Developer – phát triển game client bằng Godot.
5. Senior Golang Developer – phát triển backend.
6. Backend Architect – thiết kế API, domain và database.
7. Identity & Security Engineer – triển khai Keycloak, SSO, bảo mật.
8. DevOps Engineer – triển khai APISIX, Valkey, PostgreSQL và môi trường vận hành.
9. EdTech Specialist – thiết kế chương trình học, bài tập, đánh giá và ôn thi.
10. Multiplayer Engineer – thiết kế realtime, matchmaking và thi đấu.
11. UX/UI Designer – thiết kế giao diện phù hợp với học sinh lớp 1–9.
12. QA Engineer – kiểm thử chức năng, tích hợp, hiệu năng và bảo mật.
13. Technical Writer – xây dựng tài liệu kỹ thuật và hướng dẫn vận hành.

Bạn phải làm việc như một đội ngũ phát triển sản phẩm thực tế, không phải chỉ tạo một bản demo giao diện.

Mọi quyết định kiến trúc phải có lý do, có tài liệu và có khả năng triển khai.

Không tự ý bỏ qua các yêu cầu nghiệp vụ quan trọng.

Không tạo ra một hệ thống quá phức tạp bằng cách chia microservices ngay từ đầu nếu chưa có lý do về tải hoặc yêu cầu triển khai.

Ưu tiên kiến trúc dễ phát triển, dễ bảo trì, dễ kiểm thử và mở rộng.

---

# PHẦN 2. TỔNG QUAN SẢN PHẨM

## 2.1. Định vị EduWorld

EduWorld là nền tảng Game-based Learning dành cho học sinh từ lớp 1 đến lớp 9.

EduWorld không phải là một game giải trí thông thường có thêm câu hỏi học tập.

EduWorld là một thế giới game 3D được xây dựng với mục tiêu:

- Thu hút học sinh tham gia học tập thông qua trải nghiệm game.
- Giúp học sinh học kiến thức theo chương trình.
- Giúp học sinh làm bài tập trực tiếp trong game.
- Hỗ trợ luyện tập, ôn tập và luyện đề trước các kỳ thi.
- Tạo môi trường thi đấu kiến thức giữa các học sinh.
- Tạo môi trường hợp tác, giao tiếp và học tập cộng đồng.
- Giúp giáo viên, nhà trường theo dõi quá trình học tập.
- Tạo động lực học tập thông qua nhiệm vụ, thành tựu, phần thưởng và thi đấu.

Định hướng trải nghiệm:

**Learn – Play – Practice – Compete – Connect**

Nguyên tắc sản phẩm:

Học sinh đến vì game, ở lại vì trải nghiệm, tiến bộ nhờ học tập và có động lực nhờ thi đấu.

## 2.2. Đối tượng sử dụng

- Học sinh lớp 1–9.
- Giáo viên.
- Quản lý nhà trường.
- Quản trị viên nội dung.
- Quản trị viên hệ thống.
- Quản trị viên nền tảng EduWorld.

Hệ thống cần hỗ trợ nhiều trường học và nhiều tổ chức.

Dữ liệu phải được phân tách theo tenant/school phù hợp.

## 2.3. Định hướng game 3D

Game được xây dựng bằng Godot 4.x với môi trường 3D.

Các yếu tố chính:

- Nhân vật 3D.
- Avatar và customization.
- Bản đồ 3D.
- Khu vực tương tác.
- NPC.
- Nhiệm vụ.
- Mini-game.
- Khu vực học tập.
- Khu vực thi đấu.
- Multiplayer.
- Giao tiếp giữa người chơi.

Không hardcode toàn bộ nội dung học tập vào các scene Godot.

Game client phải có khả năng tải nội dung và cấu hình nhiệm vụ từ backend.

Thiết kế hệ thống data-driven để có thể cập nhật nội dung mà không phải build lại toàn bộ game.

---

# PHẦN 3. GAME DESIGN — THIẾT KẾ THẾ GIỚI 3D

## 3.1. Game World

Xây dựng một thế giới 3D giáo dục có phong cách:

- Thân thiện.
- Màu sắc sinh động.
- Dễ nhận biết.
- Phù hợp học sinh tiểu học và trung học cơ sở.
- Không có yếu tố bạo lực hoặc nội dung không phù hợp.
- Có khả năng mở rộng thêm bản đồ và khu vực.

Phong cách hình ảnh đề xuất:

Stylized 3D / Cartoon 3D / Low-poly có chất lượng hình ảnh tốt.

Không yêu cầu đồ họa chân thực như game AAA.

Ưu tiên hiệu năng, khả năng chạy trên máy cấu hình phổ thông và thiết bị di động.

## 3.2. Các khu vực trong game

Thiết kế các khu vực có chức năng riêng:

### A. Main Hub

Khu vực trung tâm:

- Nơi người chơi đăng nhập vào thế giới.
- Hiển thị nhân vật.
- Hiển thị nhiệm vụ.
- Hiển thị thông báo.
- Đi đến các khu vực khác.
- Gặp gỡ bạn bè.

### B. Learning Zone

Khu học tập:

- Khu Toán học.
- Khu Tiếng Việt.
- Khu Tiếng Anh.
- Khu Khoa học.
- Các môn học khác theo chương trình được cấu hình.

### C. Practice Zone

Khu luyện tập:

- Bài tập theo chủ đề.
- Bài tập theo lớp.
- Bài tập theo mức độ.
- Luyện tập theo kỹ năng.
- Bài tập cá nhân.

### D. Exam Zone

Khu vực ôn thi:

- Luyện đề.
- Thi thử.
- Kiểm tra theo chương.
- Kiểm tra theo học kỳ.
- Đánh giá kiến thức.
- Xem kết quả và phân tích lỗi sai.

### E. Competition Arena

Khu vực thi đấu:

- Thi đấu 1v1.
- Thi đấu đội.
- Đấu kiến thức.
- Thử thách theo thời gian.
- Tournament.
- Ranking.

### F. Social Zone

Khu vực cộng đồng:

- Gặp bạn bè.
- Chat.
- Party.
- Nhóm học tập.
- Hoạt động cộng đồng được kiểm soát.

### G. Quest Zone

Khu vực nhiệm vụ:

- Nhiệm vụ học tập.
- Nhiệm vụ khám phá.
- Nhiệm vụ theo ngày.
- Nhiệm vụ theo tuần.
- Nhiệm vụ theo chủ đề kiến thức.

## 3.3. Nhân vật và customization

Xây dựng hệ thống:

- Character.
- Avatar.
- Skin.
- Outfit.
- Hair.
- Accessories.
- Emotes.
- Animation.
- Character level.
- Inventory.

Phần thưởng có thể được mở khóa thông qua:

- Hoàn thành bài tập.
- Hoàn thành nhiệm vụ.
- Tham gia sự kiện.
- Đạt thành tựu.
- Tham gia thi đấu.

Không tạo cơ chế phần thưởng khiến học sinh phải chi tiền để đạt kết quả học tập tốt hơn.

## 3.4. Quest System

Quest là một trong những thành phần cốt lõi.

Mỗi nhiệm vụ có thể bao gồm:

- ID.
- Tên.
- Mô tả.
- Loại nhiệm vụ.
- Lớp học áp dụng.
- Môn học.
- Chủ đề.
- Learning Objective.
- Điều kiện bắt đầu.
- Các bước thực hiện.
- Điều kiện hoàn thành.
- Phần thưởng.
- Thời gian.
- Phiên bản nội dung.

Hỗ trợ các loại:

- Educational Quest.
- Exploration Quest.
- Daily Quest.
- Weekly Quest.
- Challenge Quest.
- Cooperative Quest.
- Competition Quest.

Quest phải liên kết với nội dung học tập.

Ví dụ:

Học sinh nhận nhiệm vụ giúp NPC giải quyết một vấn đề. Muốn hoàn thành nhiệm vụ, học sinh phải trả lời các câu hỏi Toán học hoặc giải bài tập phù hợp với lớp của mình.

## 3.5. Mini-game Engine

Thiết kế hệ thống mini-game có thể tái sử dụng.

Các dạng mini-game:

- Multiple Choice.
- True / False.
- Fill in the Blank.
- Drag and Drop.
- Matching.
- Ordering.
- Calculation.
- Puzzle.
- Word Game.
- Science Simulation.
- Timed Challenge.

Mini-game không được tự quản lý ngân hàng câu hỏi riêng.

Nội dung phải lấy từ Learning Engine.

Mỗi mini-game cần hỗ trợ cấu hình:

- Độ khó.
- Thời gian.
- Số câu hỏi.
- Điểm số.
- Cách tính kết quả.
- Phản hồi đúng/sai.
- Cơ chế hoàn thành.
- Reward configuration.

---

# PHẦN 4. LEARNING ENGINE — HỆ THỐNG HỌC TẬP

Đây là domain quan trọng nhất của EduWorld.

Game phải phục vụ Learning Engine, không được để logic học tập phụ thuộc hoàn toàn vào gameplay.

## 4.1. Curriculum

Xây dựng cấu trúc chương trình học:

Curriculum
→ Grade
→ Subject
→ Topic
→ Learning Objective
→ Learning Unit
→ Lesson
→ Learning Activity
→ Question / Exercise / Assessment.

Phải hỗ trợ nhiều chương trình học và phiên bản chương trình học.

Không hardcode một chương trình học duy nhất.

Cho phép CMS quản lý nội dung và cấu trúc chương trình.

## 4.2. Learning Content

Các loại nội dung:

- Lesson.
- Learning Material.
- Video.
- Image.
- Audio.
- Interactive Content.
- Question.
- Exercise.
- Question Set.
- Assessment.
- Game Activity.

Nội dung phải có metadata:

- Grade.
- Subject.
- Topic.
- Learning Objective.
- Difficulty.
- Content Version.
- Status.
- Author.
- Reviewer.
- Published At.

Hỗ trợ trạng thái:

Draft → Review → Approved → Published → Archived.

Nội dung đã được sử dụng trong kết quả học tập phải có version để bảo đảm khả năng truy xuất lịch sử.

## 4.3. Question Bank

Xây dựng ngân hàng câu hỏi tập trung.

Các dạng câu hỏi:

1. Single Choice.
2. Multiple Choice.
3. True / False.
4. Fill in the Blank.
5. Short Answer.
6. Numeric Answer.
7. Matching.
8. Ordering.
9. Drag and Drop.
10. Image-based Question.
11. Interactive Question.

Mỗi câu hỏi cần có:

- Question ID.
- Nội dung.
- Dữ liệu hiển thị.
- Danh sách đáp án.
- Đáp án đúng.
- Explanation.
- Difficulty.
- Grade.
- Subject.
- Topic.
- Learning Objective.
- Scoring Rule.
- Version.

Hỗ trợ random câu hỏi theo điều kiện.

Hỗ trợ tạo đề từ Question Bank.

Không để game client quyết định đáp án đúng hoặc tự xác nhận điểm số cuối cùng.

## 4.4. Exercise Engine

Cho phép học sinh:

- Làm bài tập cá nhân.
- Làm bài tập theo nhiệm vụ.
- Làm bài tập theo chủ đề.
- Làm bài tập theo lớp.
- Làm bài tập được giáo viên giao.
- Làm bài tập tự luyện.
- Làm lại câu hỏi sai.

Hệ thống cần lưu:

- Attempt.
- Answer.
- Correctness.
- Score.
- Duration.
- Submission Time.
- Content Version.
- Learning Objective.

Hỗ trợ resume các bài tập chưa hoàn thành theo chính sách.

## 4.5. Exam & Practice Engine

Các hình thức:

- Practice Test.
- Mock Exam.
- Chapter Test.
- Semester Test.
- Subject Test.
- Custom Assessment.
- Timed Exam.

Chức năng:

- Tạo đề.
- Random đề.
- Random câu hỏi.
- Cấu hình thời gian.
- Cấu hình thang điểm.
- Nộp bài.
- Chấm điểm.
- Hiển thị kết quả.
- Phân tích câu sai.
- Đề xuất nội dung cần ôn tập.

Phân biệt rõ luyện tập và kiểm tra chính thức.

Kết quả thi phải được lưu bền vững.

Có cơ chế chống nộp bài trùng và chống sửa điểm từ client.

## 4.6. Learning Progress

Theo dõi:

- Tiến độ học tập.
- Số bài đã hoàn thành.
- Tỷ lệ chính xác.
- Thời gian học.
- Số lần làm bài.
- Learning Objective Mastery.
- Kỹ năng còn yếu.
- Tiến bộ theo thời gian.

Không đồng nhất điểm số một lần làm bài với mức độ thành thạo kiến thức.

Cần có phương pháp đánh giá mastery rõ ràng và có thể cấu hình.

---

# PHẦN 5. MULTIPLAYER & COMPETITION

## 5.1. Multiplayer

Hỗ trợ:

- Multiplayer World.
- Party.
- Room.
- Lobby.
- Friend.
- Team.
- Match.
- Session.

Người chơi có thể:

- Gặp gỡ người chơi khác.
- Tham gia nhóm.
- Mời bạn bè.
- Cùng thực hiện nhiệm vụ.
- Tham gia thi đấu.
- Cùng học tập.

## 5.2. Matchmaking

Các chế độ:

- Random Matchmaking.
- Skill-based Matchmaking.
- Grade-based Matchmaking.
- Subject-based Matchmaking.
- Topic-based Matchmaking.
- Friend Matchmaking.
- Team Matchmaking.
- Tournament Matchmaking.

Quy tắc:

Ưu tiên kiểm tra điều kiện độ tuổi/lớp học và tính phù hợp của nội dung trước khi sử dụng rating để ghép trận.

Không ghép học sinh lớp 1 với lớp 9 chỉ vì có điểm rating tương đương.

Hỗ trợ:

- Queue.
- Matchmaking Ticket.
- Match Proposal.
- Lobby.
- Ready Check.
- Team Assignment.
- Match Start.
- Match End.
- Reconnect.
- Match Result.

## 5.3. Competition Modes

- 1v1 Knowledge Battle.
- Team Battle.
- Co-op Challenge.
- Timed Quiz.
- Subject Tournament.
- Class Competition.
- School Competition.
- Seasonal Competition.

## 5.4. Ranking

Xây dựng hệ thống ranking độc lập.

Các bảng xếp hạng:

- Global.
- School.
- Class.
- Grade.
- Subject.
- Topic.
- Game Mode.
- Friends.
- Team.
- Daily.
- Weekly.
- Monthly.
- Season.

Hỗ trợ ELO hoặc Glicko-2 cho các chế độ thi đấu phù hợp.

Không sử dụng ELO để đánh giá năng lực học tập.

Phải phân biệt:

- Learning Score.
- Learning Mastery.
- Game Rating.
- Game XP.
- Competition Points.

Đây là các dữ liệu có ý nghĩa khác nhau.

## 5.5. Tournament

Hỗ trợ:

- Single Elimination.
- Double Elimination.
- Round Robin.
- Team Tournament.
- School Tournament.
- Class Tournament.

Chức năng:

- Tạo giải.
- Đăng ký.
- Chia bảng.
- Tạo bracket.
- Lên lịch.
- Xác nhận người tham gia.
- Quản lý trận đấu.
- Xác nhận kết quả.
- Trao thưởng.
- Lưu lịch sử.

## 5.6. Anti-cheat

Các nguyên tắc:

- Server-authoritative result.
- Không tin điểm số từ Godot client.
- Kiểm tra thời gian làm bài.
- Kiểm tra duplicate submission.
- Rate limiting.
- Idempotency.
- Audit log.
- Phát hiện hành vi bất thường.
- Bảo vệ API thi đấu.

---

# PHẦN 6. SOCIAL & IN-GAME CHAT

EduWorld cần có khả năng giao tiếp trực tiếp giữa học sinh.

Chat phải là một domain riêng.

## 6.1. Chat Channels

Hỗ trợ:

- World Chat.
- Area Chat.
- Party Chat.
- Team Chat.
- Class Chat.
- Study Group Chat.
- Direct Message.

Các loại channel phải được cấu hình bằng policy.

Không mặc định mở toàn bộ channel cho mọi học sinh.

## 6.2. Chat Features

- Text Messaging.
- Emoji.
- Reactions.
- Quick Messages.
- Group Chat.
- Friend List.
- Friend Request.
- Invitation.
- Message History.
- Unread Count.
- Notifications.
- Mute.
- Block.
- Report.
- Moderation.

Thiết kế API và UI để có thể mở rộng voice chat trong tương lai nhưng không triển khai voice chat mặc định trong MVP.

## 6.3. Safety & Moderation

Vì học sinh thuộc nhóm tuổi 6–15, an toàn phải được thiết kế ngay từ đầu.

Bắt buộc có:

- Content Filtering.
- Profanity Filtering.
- Anti-spam.
- Anti-bullying.
- Personal Information Detection.
- Report Message.
- Report User.
- Block User.
- Mute User.
- Moderation Queue.
- Moderation Audit.
- School-level Chat Policy.
- Age/Grade-based Policy.
- Teacher/Admin Moderation.

Các chính sách cần hỗ trợ:

- Giới hạn người có thể nhắn tin trực tiếp.
- Hạn chế chat công khai.
- Hạn chế chia sẻ thông tin cá nhân.
- Cấu hình quyền theo trường.
- Cấu hình quyền theo lớp.
- Kiểm soát lịch sử chat theo quyền và chính sách lưu trữ.
- Có quy trình xử lý báo cáo.

Không để một người dùng có quyền truy cập toàn bộ dữ liệu chat của tất cả trường.

Không sử dụng việc giám sát chat như một lý do để thu thập dữ liệu vượt quá mục đích cần thiết.

Phải có chính sách bảo vệ dữ liệu cá nhân và trẻ em phù hợp pháp luật Việt Nam.

---

# PHẦN 7. GAMIFICATION

Xây dựng hệ thống tạo động lực học tập.

Các thành phần:

- XP.
- Level.
- Achievement.
- Badge.
- Quest Reward.
- Inventory.
- Character Customization.
- Daily Challenge.
- Weekly Challenge.
- Season Reward.
- Collection.

XP có thể nhận từ:

- Hoàn thành bài học.
- Hoàn thành bài tập.
- Hoàn thành quest.
- Tham gia hoạt động.
- Hợp tác với bạn bè.
- Thi đấu.

Không khuyến khích hành vi spam bài tập hoặc làm đi làm lại câu dễ để farm XP.

Reward Engine phải có rule và giới hạn.

Không để phần thưởng game ảnh hưởng đến kết quả đánh giá học tập chính thức.

---

# PHẦN 8. CMS — CONTENT MANAGEMENT SYSTEM

CMS là hệ thống quản trị nội dung tập trung.

CMS phải hỗ trợ các vai trò:

- Platform Admin.
- Content Admin.
- Academic Manager.
- School Admin.
- Teacher.
- Content Reviewer.

Các module:

## 8.1. Curriculum Management

- Quản lý chương trình.
- Quản lý khối lớp.
- Quản lý môn học.
- Quản lý chủ đề.
- Quản lý learning objective.

## 8.2. Content Management

- Lesson Management.
- Learning Material.
- Video/Image/Audio.
- Question Bank.
- Question Set.
- Assessment.
- Exercise.
- Learning Activity.

## 8.3. Game Content Management

- Quest Management.
- Mini-game Configuration.
- Game Level.
- NPC Dialogue.
- Reward Configuration.
- World Event.
- Game Activity.

## 8.4. Competition Management

- Matchmaking Configuration.
- Ranking Rules.
- Tournament.
- Season.
- Challenge.
- Competition Reward.

## 8.5. Content Workflow

Draft → Review → Approved → Published → Archived.

Phải hỗ trợ:

- Versioning.
- Preview.
- Publishing.
- Rollback theo chính sách.
- Audit Trail.

CMS không được làm thay đổi lịch sử bài tập đã hoàn thành.

---

# PHẦN 9. LMS — LEARNING MANAGEMENT SYSTEM

LMS quản lý việc triển khai hoạt động học tập.

Các module:

- Course.
- Learning Path.
- Enrollment.
- Assignment.
- Schedule.
- Lesson Delivery.
- Assessment.
- Learning Progress.
- Learning Analytics.
- Teacher Dashboard.

Giáo viên có thể:

- Giao bài tập.
- Giao đề ôn tập.
- Theo dõi kết quả học sinh.
- Theo dõi tiến độ.
- Xem kỹ năng học sinh còn yếu.
- Tạo nhóm học tập.
- Tổ chức thử thách học tập.

Hỗ trợ phân quyền theo trường, lớp và môn học.

CMS quản lý nội dung.

LMS quản lý hoạt động học tập.

Có thể triển khai chung trong một Golang Modular Monolith ở giai đoạn đầu nhưng phải phân tách domain rõ ràng.

---

# PHẦN 10. SIS INTEGRATION & KEYCLOAK SSO

Đây là yêu cầu bắt buộc.

Học sinh sử dụng tài khoản được nhà trường cấp.

Học sinh không phải tự đăng ký tài khoản riêng trong game.

## 10.1. Authentication Architecture

Sử dụng Keycloak để:

- SSO.
- OpenID Connect.
- User Authentication.
- Identity Federation.
- Token Management.
- Role Mapping.
- Session Management.

Godot Game Client sử dụng OIDC Authorization Code Flow + PKCE.

Không sử dụng client secret trong Godot.

Không lưu mật khẩu của học sinh.

## 10.2. Login Flow

1. Học sinh mở EduWorld.
2. Godot kiểm tra trạng thái đăng nhập.
3. Nếu chưa đăng nhập, mở luồng đăng nhập Keycloak.
4. Keycloak thực hiện xác thực theo cấu hình của tổ chức/trường.
5. Backend xác minh token.
6. Backend tìm ExternalIdentity.
7. Nếu chưa có tài khoản nội bộ, thực hiện Just-in-Time Provisioning nếu được phép.
8. Backend lấy hoặc kiểm tra dữ liệu học sinh từ SIS/local synchronized records.
9. Backend ánh xạ student, school, class và enrollment.
10. Backend cấp game session/access token phù hợp.
11. Godot nhận thông tin người chơi và tải game.

## 10.3. Identity Mapping

Phải phân biệt:

- Keycloak User.
- EduWorld User.
- Student.
- Teacher.
- School.
- Class.
- Enrollment.
- External Identity.

Sử dụng issuer + subject để định danh external identity.

Không sử dụng email hoặc tên hiển thị làm khóa định danh duy nhất.

Hỗ trợ:

- Just-in-Time Provisioning.
- Automatic Provisioning.
- SIS Synchronization.
- Account Reconciliation.
- Student Transfer.
- Class Transfer.
- School Transfer.
- Account Deactivation.
- Account Lock.
- Token Expiration.

## 10.4. Keycloak Configuration

Thiết kế cấu hình:

- Realm.
- Clients.
- Roles.
- Groups.
- Identity Providers.
- Client Scopes.
- Token Claims.
- Audience.
- Session Policies.

Client đề xuất:

- eduworld-game.
- eduworld-cms.
- eduworld-admin.
- eduworld-api.

Không hardcode realm/client configuration trong mã nguồn.

Sử dụng environment/configuration để triển khai các môi trường.

Mô hình multi-school Realm phải được thiết kế linh hoạt. Quyết định một Realm chung hay nhiều Realm cần được ghi trong ADR sau khi xác định yêu cầu vận hành và khả năng của SIS/IdP.

## 10.5. SIS Connector

Tạo abstraction cho nhiều SIS Provider.

Các chức năng:

- Student Synchronization.
- School Synchronization.
- Class Synchronization.
- Teacher Synchronization.
- Enrollment Synchronization.
- Account Status Synchronization.
- Reconciliation.
- Sync Error Handling.
- Retry.
- Audit.

Không giả định một SIS cụ thể khi chưa được cung cấp API documentation.

Không để Godot gọi trực tiếp SIS API.

---

# PHẦN 11. APACHE APISIX — API GATEWAY

Sử dụng Apache APISIX làm cổng API thống nhất.

Các chức năng:

- Routing.
- TLS Termination.
- Authentication Integration.
- JWT/OIDC Validation.
- Rate Limiting.
- Request Size Limiting.
- CORS.
- Access Control.
- Logging.
- Metrics.
- Tracing.
- WebSocket Proxy.

Thiết kế route theo domain:

- /api/v1/auth/*
- /api/v1/students/*
- /api/v1/game/*
- /api/v1/learning/*
- /api/v1/exercises/*
- /api/v1/exams/*
- /api/v1/competition/*
- /api/v1/leaderboards/*
- /api/v1/chat/*
- /api/v1/social/*
- /api/v1/cms/*
- /api/v1/lms/*
- /api/v1/sis/*

Các route phải có authentication và authorization policy phù hợp.

APISIX không thay thế authorization nghiệp vụ trong Golang.

Backend phải kiểm tra tenant, school, class và quyền truy cập tài nguyên.

WebSocket cần được cấu hình riêng cho các kết nối realtime.

Không để API quản trị nội bộ hoặc SIS credentials lộ ra public gateway.

---

# PHẦN 12. GOLANG BACKEND ARCHITECTURE

## 12.1. Kiến trúc tổng thể

Ưu tiên Modular Monolith ở giai đoạn đầu.

Sử dụng:

- Clean Architecture hoặc Hexagonal Architecture.
- Domain-driven design ở mức phù hợp.
- Dependency Injection rõ ràng.
- Repository abstraction.
- Service layer.
- Event-driven integration khi cần.
- API versioning.

Không tạo microservices chỉ vì muốn có nhiều service.

Thiết kế ranh giới domain để có thể tách service sau này.

## 12.2. Các module

```text
backend/
  cmd/
    api/
    worker/
    realtime/

  internal/
    identity/
    authorization/
    organization/
    student/
    teacher/

    sis/
      provider/
      synchronization/
      mapping/
      reconciliation/

    curriculum/
    content/
    question_bank/

    learning/
    exercise/
    assessment/
    exam/
    learning_record/
    learning_analytics/

    game/
    quest/
    minigame/
    game_session/

    multiplayer/
      presence/
      room/
      lobby/
      matchmaking/
      party/
      realtime/

    competition/
      ranking/
      leaderboard/
      tournament/
      season/
      challenge/

    social/
      friend/
      team/

    chat/
      channel/
      message/
      moderation/
      notification/

    gamification/
    reward/

    cms/
    lms/

    audit/
    reporting/

  pkg/
  migrations/
  configs/
  tests/
```

Cấu trúc trên là định hướng. Có thể điều chỉnh theo implementation nhưng phải giữ được ranh giới nghiệp vụ.

## 12.3. API Design

- REST API cho các nghiệp vụ thông thường.
- WebSocket cho realtime chat và realtime game signaling/state khi phù hợp.
- gRPC cho giao tiếp nội bộ khi thực sự cần.
- OpenAPI/Swagger cho REST API.
- API versioning.
- Request validation.
- Standard error response.
- Pagination.
- Filtering.
- Idempotency cho các thao tác quan trọng.

Không expose database model trực tiếp ra API.

---

# PHẦN 13. GODOT 3D GAME CLIENT ARCHITECTURE

## 13.1. Nguyên tắc

Godot là client hiển thị và xử lý tương tác.

Backend là nơi quản lý dữ liệu và kết quả nghiệp vụ có thẩm quyền.

Godot không được tự quyết định:

- Điểm thi chính thức.
- Kết quả thi đấu cuối cùng.
- Learning Mastery.
- Reward có giá trị nghiệp vụ.
- Quyền truy cập tài nguyên.
- Tài khoản và thông tin học sinh.

## 13.2. Module đề xuất

```text
godot/
  scenes/
    bootstrap/
    auth/
    world/
    character/
    npc/
    quest/
    learning/
    practice/
    exam/
    competition/
    chat/
    social/
    ui/

  scripts/
    core/
    networking/
    authentication/
    player/
    world/
    learning/
    multiplayer/
    competition/
    chat/
    inventory/
    notifications/

  resources/
    characters/
    items/
    animations/
    materials/
    environments/
    audio/

  assets/
    models/
    textures/
    audio/
    fonts/
    icons/
```

## 13.3. 3D Character System

- Character Controller.
- Camera Controller.
- Animation State Machine.
- Character Customization.
- Character Inventory.
- Character Interaction.
- NPC Interaction.

## 13.4. World System

- Scene Management.
- World Loading.
- Area Transition.
- Spawn Points.
- Interactive Objects.
- NPC.
- Quest Trigger.
- Multiplayer Player Synchronization.

Thiết kế tối ưu hiệu năng 3D.

Hỗ trợ LOD, occlusion, texture optimization, batching và giới hạn tài nguyên theo thiết bị.

## 13.5. UI/UX

Giao diện:

- Main Menu.
- Login.
- Character Selection.
- World HUD.
- Quest Panel.
- Learning Panel.
- Exercise UI.
- Exam UI.
- Competition UI.
- Leaderboard.
- Chat Panel.
- Friend List.
- Inventory.
- Notification.
- Settings.

Thiết kế UI theo nhóm tuổi:

- Lớp 1–5: trực quan, ít thao tác, chữ dễ đọc.
- Lớp 6–9: giao diện trưởng thành hơn, nhiều thông tin hơn nhưng vẫn thân thiện.

Không hiển thị thông tin cá nhân nhạy cảm của học sinh cho người chơi khác.

---

# PHẦN 14. REALTIME & MULTIPLAYER NETWORKING

Thiết kế rõ các loại kết nối:

- REST: business operations.
- WebSocket: chat, notifications, matchmaking events và signaling.
- Realtime gameplay transport: lựa chọn theo yêu cầu gameplay và khả năng Godot.

Không mặc định gửi toàn bộ chuyển động 3D qua REST.

Thiết kế:

- Server-authoritative gameplay cho những chế độ cần tính công bằng.
- Client prediction nếu phù hợp.
- Interpolation.
- Reconnection.
- Session Recovery.
- Heartbeat.
- Timeout.
- Disconnect Handling.
- Room Lifecycle.
- State Synchronization.

Phải xác định loại transport cụ thể qua ADR trước khi triển khai multiplayer production.

Không coi Valkey là game simulation server.

---

# PHẦN 15. VALKEY & DATA PERSISTENCE

## 15.1. Valkey

Sử dụng cho:

- Cache.
- Player Presence.
- Online Status.
- Matchmaking Queue.
- Lobby State.
- Temporary Game State.
- Realtime Coordination.
- Rate Limiting.
- Session Cache.
- Leaderboard Cache.
- Pub/Sub hoặc Streams cho các luồng phù hợp.

Không sử dụng Valkey làm nguồn dữ liệu duy nhất cho:

- Learning Records.
- Exam Results.
- Match Results.
- Player Rating.
- Tournament Brackets.
- Student Information.
- Published Content.

## 15.2. PostgreSQL

Đề xuất PostgreSQL làm persistent database ban đầu.

Quản lý:

- User.
- Student.
- School.
- Class.
- Enrollment.
- External Identity Mapping.
- Learning Content.
- Question Bank.
- Exercise Attempt.
- Exam Result.
- Learning Record.
- Game Profile.
- Quest Progress.
- Match History.
- Ranking.
- Tournament.
- Chat History.
- Moderation Records.
- Audit Logs.

Phải thiết kế migration và indexing.

Có chiến lược backup và recovery.

Không tạo bảng quá lớn mà không có kế hoạch phân vùng hoặc lưu trữ phù hợp.

---

# PHẦN 16. LEARNING EVENT & ANALYTICS

Thiết kế event tracking cho toàn bộ hoạt động học tập.

Các event tham khảo:

- activity.started
- activity.completed
- question.viewed
- question.answered
- question.correct
- question.incorrect
- exercise.started
- exercise.submitted
- exam.started
- exam.submitted
- lesson.completed
- quest.completed
- learning_objective.progressed
- learning_objective.mastered

Mỗi event cần có:

- Event ID.
- Student ID.
- School ID.
- Session ID.
- Activity ID.
- Content ID.
- Content Version.
- Event Type.
- Timestamp.
- Payload.
- Idempotency Key.

Thiết kế tương thích với tư duy xAPI/Learning Record Store khi cần mở rộng, nhưng không bắt buộc triển khai full LRS trong MVP.

Analytics cần hỗ trợ:

- Học sinh.
- Giáo viên.
- Lớp học.
- Trường học.
- Môn học.
- Chủ đề.
- Learning Objective.

Không dùng dữ liệu hành vi ngoài mục đích đã công bố.

---

# PHẦN 17. SECURITY & CHILD SAFETY

Đây là yêu cầu bắt buộc.

## 17.1. Security

- HTTPS/TLS.
- Secure Token Handling.
- OIDC PKCE.
- JWT Validation.
- RBAC.
- Tenant Isolation.
- School-level Authorization.
- Input Validation.
- SQL Injection Protection.
- Rate Limiting.
- Audit Logging.
- Secret Management.
- Secure Configuration.
- Backup.
- Recovery.
- Vulnerability Scanning.

## 17.2. Child Safety

- Age-appropriate UX.
- Chat Moderation.
- Privacy Protection.
- Personal Information Protection.
- Anti-bullying.
- Reporting.
- Blocking.
- Teacher Controls.
- School Controls.
- Content Moderation.
- Safe Reward System.

Không cung cấp tính năng cho phép học sinh dễ dàng công khai thông tin cá nhân.

Không mở chat tự do không kiểm soát cho toàn bộ học sinh.

Không thiết kế cơ chế gây nghiện, ép buộc hoặc gây áp lực tiêu cực lên trẻ em.

## 17.3. Privacy

Thiết kế theo nguyên tắc:

- Data Minimization.
- Purpose Limitation.
- Access Control.
- Data Retention Policy.
- Auditability.
- Secure Data Transfer.
- Data Deletion Policy.

Cần đánh giá yêu cầu pháp lý hiện hành tại Việt Nam trước khi đưa vào production.

---

# PHẦN 18. DEVOPS & DEPLOYMENT

Thiết kế môi trường:

- Local Development.
- Development.
- Staging.
- Production.

Các thành phần:

- Godot Build Pipeline.
- Golang Backend.
- Keycloak.
- Apache APISIX.
- Valkey.
- PostgreSQL.
- Monitoring.
- Logging.
- Tracing.

Sử dụng Docker và Docker Compose cho local development.

Có thể sử dụng Kubernetes khi quy mô triển khai yêu cầu.

Không bắt buộc Kubernetes ngay từ đầu.

Tạo:

- Dockerfile.
- docker-compose.yml.
- Environment Example.
- Database Migration.
- Health Check.
- Readiness Check.
- Liveness Check.
- Backup Script.
- Deployment Documentation.

Không commit secrets vào repository.

## Monitoring

Thiết kế khả năng tích hợp:

- Prometheus.
- Grafana.
- OpenTelemetry.
- Centralized Logging.
- Error Tracking.

Các metrics quan trọng:

- Online Players.
- Active Sessions.
- Matchmaking Queue.
- Match Duration.
- API Latency.
- Error Rate.
- Chat Message Rate.
- Learning Completion.
- Exam Submission.
- Database Performance.

---

# PHẦN 19. PROJECT DOCUMENTATION

Trước khi triển khai code, hãy tạo bộ tài liệu đầy đủ.

Cấu trúc đề xuất:

```text
docs/
  00-overview/
    PRODUCT_VISION.md
    PRODUCT_REQUIREMENTS.md
    USER_PERSONAS.md
    GLOSSARY.md

  01-product/
    GAME_DESIGN.md
    GAME_WORLD_DESIGN.md
    GAMEPLAY_LOOP.md
    LEARNING_EXPERIENCE.md
    COMPETITION_DESIGN.md
    SOCIAL_CHAT_DESIGN.md
    CHILD_SAFETY.md

  02-architecture/
    SYSTEM_ARCHITECTURE.md
    ARCHITECTURE_DECISIONS.md
    DOMAIN_MODEL.md
    MULTITENANCY.md
    REALTIME_ARCHITECTURE.md
    SECURITY_ARCHITECTURE.md

  03-backend/
    BACKEND_DESIGN.md
    API_DESIGN.md
    DATABASE_DESIGN.md
    VALKEY_DESIGN.md
    EVENT_DESIGN.md

  04-identity/
    KEYCLOAK_SSO.md
    IDENTITY_MAPPING.md
    SIS_INTEGRATION.md
    ACCOUNT_PROVISIONING.md

  05-learning/
    CURRICULUM_MODEL.md
    CONTENT_MODEL.md
    QUESTION_BANK.md
    EXERCISE_ENGINE.md
    EXAM_ENGINE.md
    LEARNING_ANALYTICS.md

  06-game/
    GODOT_ARCHITECTURE.md
    3D_WORLD_DESIGN.md
    CHARACTER_SYSTEM.md
    QUEST_SYSTEM.md
    MINIGAME_ENGINE.md
    GAME_NETWORKING.md

  07-multiplayer/
    MULTIPLAYER_DESIGN.md
    MATCHMAKING.md
    LEADERBOARD.md
    RANKING.md
    TOURNAMENT.md
    CHAT_SYSTEM.md
    MODERATION.md

  08-cms-lms/
    CMS_DESIGN.md
    LMS_DESIGN.md
    TEACHER_DASHBOARD.md

  09-security/
    SECURITY.md
    DATA_PRIVACY.md
    CHILD_PROTECTION.md
    ANTI_CHEAT.md

  10-devops/
    DEPLOYMENT.md
    LOCAL_DEVELOPMENT.md
    MONITORING.md
    BACKUP_RECOVERY.md

  11-project/
    ROADMAP.md
    MILESTONES.md
    TEST_STRATEGY.md
    RISKS.md
    OPEN_QUESTIONS.md
```

Ngoài ra cần tạo:

```text
README.md
AGENTS.md
CONTRIBUTING.md
CHANGELOG.md
VIBE_CODING_PROMPT.md
```

Mỗi tài liệu phải có nội dung thực chất, không tạo file rỗng hoặc chỉ chứa tiêu đề.

---

# PHẦN 20. DEVELOPMENT ROADMAP

Không triển khai tất cả chức năng cùng một lúc.

Chia thành các giai đoạn.

## Phase 0 — Discovery & Architecture

- Phân tích yêu cầu.
- Chốt product scope.
- Xác định domain.
- Thiết kế kiến trúc.
- Thiết kế database.
- Thiết kế API.
- Thiết kế Keycloak.
- Thiết kế APISIX.
- Thiết kế SIS integration.
- Tạo tài liệu.
- Tạo ADR.

Deliverable: bộ tài liệu và architecture baseline.

## Phase 1 — Foundation

- Golang Backend.
- PostgreSQL.
- Valkey.
- Keycloak.
- APISIX.
- Godot 3D Bootstrap.
- Authentication.
- User Profile.
- School/Student Mapping.
- Basic CI/CD.

Deliverable: học sinh có thể đăng nhập và vào thế giới 3D.

## Phase 2 — Core Educational Gameplay

- 3D Main Hub.
- Character Controller.
- NPC.
- Quest System.
- Learning Engine.
- Question Bank.
- Exercise Engine.
- Mini-game Engine.
- XP/Reward.

Deliverable: học sinh có thể vào game và hoàn thành nhiệm vụ học tập.

## Phase 3 — Exam & Learning Management

- CMS.
- LMS.
- Assignment.
- Exam Engine.
- Mock Exam.
- Learning Progress.
- Teacher Dashboard.
- Learning Analytics.

Deliverable: có thể tạo bài tập, giao bài, luyện đề và theo dõi kết quả.

## Phase 4 — Multiplayer & Competition

- Multiplayer World.
- Party.
- Lobby.
- Matchmaking.
- Knowledge Battle.
- Leaderboard.
- Ranking.
- Tournament.
- Match History.

Deliverable: học sinh có thể thi đấu với nhau.

## Phase 5 — Social & Chat

- Friend System.
- Group.
- Chat Channels.
- Direct Message theo policy.
- Realtime Notifications.
- Moderation.
- Reporting.
- Teacher Controls.

Deliverable: môi trường giao tiếp an toàn trong game.

## Phase 6 — Production Readiness

- Performance Optimization.
- Security Testing.
- Load Testing.
- Anti-cheat.
- Backup/Recovery.
- Monitoring.
- Deployment.
- UAT.
- Production Release.

Mỗi phase phải có tiêu chí nghiệm thu rõ ràng.

Không đánh dấu hoàn thành nếu chỉ có giao diện giả lập.

---

# PHẦN 21. CODING RULES — QUY TẮC CHO AI AGENT

1. Luôn đọc `README.md`, `AGENTS.md` và tài liệu liên quan trước khi sửa code.
2. Không tự ý thay đổi kiến trúc đã được chốt trong ADR.
3. Nếu cần thay đổi kiến trúc, tạo ADR mới và giải thích lý do.
4. Không tạo code trùng lặp.
5. Không viết business logic trong UI.
6. Không viết business logic trong Godot scene script nếu có thể tách thành service/domain logic.
7. Không hardcode nội dung học tập.
8. Không hardcode credentials.
9. Không đưa secret vào Git.
10. Không tin dữ liệu điểm số do client gửi lên.
11. Không bỏ qua authorization.
12. Không bỏ qua tenant isolation.
13. Không lưu dữ liệu quan trọng duy nhất trong Valkey.
14. Không tạo endpoint không có authentication/authorization khi endpoint cần bảo vệ.
15. Không tạo mock implementation rồi tuyên bố production-ready.
16. Không bỏ qua error handling.
17. Không bỏ qua logging và observability.
18. Không bỏ qua test.
19. Không tự ý xóa code hoặc migration có thể ảnh hưởng dữ liệu.
20. Luôn cập nhật tài liệu khi thay đổi kiến trúc hoặc API.

## Coding Quality

Golang:

- Format bằng gofmt.
- Static analysis.
- Unit Tests.
- Integration Tests.
- Context propagation.
- Structured Logging.
- Error Wrapping.
- Graceful Shutdown.

Godot:

- Scene organization rõ ràng.
- Reusable Components.
- Resource-based Configuration.
- Signal-based Communication phù hợp.
- Tách UI và gameplay logic.
- Không để scene phụ thuộc chặt vào scene khác.
- Có kiểm thử các logic quan trọng.

API:

- OpenAPI Specification.
- Versioning.
- Validation.
- Pagination.
- Idempotency.
- Standard Error Response.

---

# PHẦN 22. TESTING & ACCEPTANCE CRITERIA

Mỗi module cần có:

- Unit Test.
- Integration Test.
- API Test.
- Security Test khi phù hợp.
- Performance Test khi cần.

Các tình huống bắt buộc phải kiểm tra:

### Authentication

- Học sinh đăng nhập qua Keycloak.
- Token hết hạn.
- Refresh token.
- Logout.
- User không tồn tại trong SIS.
- Student bị khóa.
- Student chuyển lớp.
- Student chuyển trường.

### Learning

- Tải bài học.
- Làm bài tập.
- Chấm điểm.
- Nộp bài.
- Làm lại bài.
- Lưu lịch sử.
- Không thể tự sửa kết quả.

### Competition

- Tạo match.
- Matchmaking.
- Hai người tham gia.
- Disconnect.
- Reconnect.
- Hoàn thành trận đấu.
- Tính kết quả.
- Cập nhật ranking.
- Không tính trùng kết quả.

### Chat

- Tạo channel.
- Tham gia channel.
- Gửi tin nhắn.
- Nhận tin nhắn.
- Block.
- Report.
- Moderation.
- Không truy cập channel trái phép.
- Không đọc tin nhắn của trường khác.

### Multi-tenancy

- Học sinh trường A không truy cập dữ liệu trường B.
- Giáo viên chỉ quản lý lớp được cấp quyền.
- School Admin chỉ quản lý phạm vi trường được phân quyền.
- Platform Admin có quyền quản trị toàn hệ thống theo policy.

### Performance

Xây dựng kịch bản kiểm thử tải dựa trên mục tiêu concurrent users được xác nhận trong giai đoạn Discovery.

Không tự tuyên bố hệ thống hỗ trợ hàng chục nghìn người chơi nếu chưa có benchmark.

---

# PHẦN 23. CÁCH AI AGENT PHẢI THỰC HIỆN DỰ ÁN

Thực hiện theo trình tự dưới đây.

## Step 1 — Inspect Repository

- Kiểm tra toàn bộ repository.
- Xác định công nghệ đang có.
- Kiểm tra cấu trúc.
- Tìm code hiện có.
- Không ghi đè code hiện có khi chưa hiểu mục đích.

Nếu repository mới, tạo cấu trúc ban đầu phù hợp.

## Step 2 — Requirement Analysis

Tạo:

- Product Vision.
- PRD.
- Functional Requirements.
- Non-functional Requirements.
- User Stories.
- Acceptance Criteria.
- Open Questions.

Phân loại yêu cầu:

- Confirmed.
- Proposed.
- TBD.

Không biến giả định thành yêu cầu đã xác nhận.

## Step 3 — Architecture Design

Thiết kế:

- System Architecture.
- Domain Model.
- Database Model.
- API Contract.
- Authentication Flow.
- SIS Integration.
- Realtime Architecture.
- Godot Architecture.
- Deployment Architecture.

Tạo ADR cho các quyết định kỹ thuật quan trọng.

## Step 4 — Documentation

Hoàn thành bộ tài liệu trong thư mục `docs/`.

Không tạo tài liệu sơ sài.

Tài liệu phải đủ để developer khác có thể triển khai.

## Step 5 — Foundation Implementation

Triển khai nền tảng trước:

- Golang Backend.
- PostgreSQL.
- Valkey.
- Keycloak.
- APISIX.
- Godot Bootstrap.
- Authentication.
- Basic CI/CD.

## Step 6 — Implement Feature by Feature

Mỗi feature phải thực hiện theo chu trình:

1. Đọc requirement.
2. Thiết kế solution.
3. Cập nhật API/data model nếu cần.
4. Implement backend.
5. Implement Godot client.
6. Implement CMS/LMS nếu liên quan.
7. Viết test.
8. Chạy test.
9. Cập nhật tài liệu.
10. Báo cáo kết quả.

## Step 7 — Review

Sau mỗi module:

- Code Review.
- Architecture Review.
- Security Review.
- Test Review.
- Documentation Review.

Không chuyển sang module tiếp theo nếu module hiện tại có lỗi nghiêm trọng chưa được xử lý.

---

# PHẦN 24. YÊU CẦU ĐẦU RA CỦA AI AGENT

Sau khi đọc prompt, không bắt đầu viết toàn bộ hệ thống ngay lập tức.

Trước tiên hãy:

1. Phân tích yêu cầu EduWorld.
2. Xác định những yêu cầu đã được xác nhận.
3. Xác định các giả định.
4. Xác định các câu hỏi còn thiếu.
5. Đề xuất architecture baseline.
6. Tạo sơ đồ kiến trúc bằng Mermaid.
7. Tạo domain model.
8. Tạo cấu trúc repository.
9. Tạo danh sách tài liệu cần xây dựng.
10. Đề xuất roadmap triển khai.

Sau đó bắt đầu tạo tài liệu và implementation theo từng phase.

Mỗi lần hoàn thành công việc phải báo cáo:

- Đã làm gì.
- File nào đã tạo/sửa.
- Kiến trúc hoặc quyết định nào đã thay đổi.
- Test nào đã chạy.
- Kết quả test.
- Vấn đề còn tồn tại.
- Công việc tiếp theo.

Không tuyên bố hoàn thành khi chưa có bằng chứng kiểm thử.

---

# PHẦN 25. CÁC QUYẾT ĐỊNH CHƯA ĐƯỢC TỰ Ý GIẢ ĐỊNH

Các thông tin dưới đây cần được làm rõ trong Discovery:

1. SIS cụ thể và API documentation.
2. Keycloak triển khai một Realm hay nhiều Realm.
3. Quy mô học sinh và concurrent players.
4. Godot hỗ trợ Windows, Android, iOS hay Web.
5. Game world là persistent world, multiplayer hub hay hybrid.
6. Multiplayer gameplay dùng realtime transport nào.
7. Mô hình CMS: Platform Admin, School Admin, Teacher hay thêm các cấp quản lý.
8. Chương trình học áp dụng theo bộ sách/chương trình nào.
9. Chính sách chat theo độ tuổi, trường và lớp.
10. Database và deployment topology cuối cùng.
11. Các chỉ tiêu hiệu năng và SLA.
12. Chính sách lưu trữ dữ liệu và quyền truy cập dữ liệu học sinh.

Nếu chưa có câu trả lời, hãy đề xuất phương án mặc định, ghi rõ là Proposed/TBD và tiếp tục các phần không bị phụ thuộc.

Không dừng toàn bộ dự án chỉ vì thiếu một thông tin chưa cần thiết ở giai đoạn hiện tại.

---

# PRODUCT PRINCIPLES — NGUYÊN TẮC CUỐI CÙNG

EduWorld phải đáp ứng đồng thời 5 mục tiêu:

**1. EDUCATION FIRST**

Giá trị cốt lõi là học tập, luyện tập và tiến bộ kiến thức.

**2. GAME EXPERIENCE**

Game 3D phải hấp dẫn, trực quan, có thế giới và gameplay thực sự.

**3. COMPETITION**

Học sinh có thể thi đấu, thử thách và cải thiện kỹ năng.

**4. SOCIAL CONNECTION**

Học sinh có thể kết nối, trao đổi và hợp tác trong môi trường an toàn.

**5. SCHOOL INTEGRATION**

Nhà trường có thể quản lý học sinh, chương trình học, bài tập và kết quả thông qua hệ thống tích hợp SIS, Keycloak và CMS/LMS.

Hãy xây dựng EduWorld như một sản phẩm EdTech có khả năng phát triển lâu dài, không phải một bản demo game ngắn hạn.

**Mục tiêu cuối cùng: Xây dựng một thế giới 3D nơi học sinh chủ động tham gia, vui chơi, học tập, luyện thi, thi đấu và kết nối với bạn bè mỗi ngày.**

---

# ADDENDUM — PARTY, GROUP & COOPERATIVE MULTIPLAYER

EduWorld phải hỗ trợ hệ thống tạo nhóm cho phép nhiều học sinh cùng tham gia một Party và chơi game trong cùng một nhóm.

## 1. Party Management

Hỗ trợ:

- Create Party.
- Join Party.
- Leave Party.
- Invite Friend.
- Accept Invitation.
- Reject Invitation.
- Kick Member.
- Transfer Leader.
- Disband Party.
- Party Settings.
- Party Member Status.
- Party Chat.

Mỗi Party có:

- Party ID.
- Party Name.
- Leader ID.
- Members.
- Maximum Members.
- Party Type.
- Visibility.
- Invitation Policy.
- Current Activity.
- Current Game Session.
- Created At.
- Updated At.

## 2. Party Types

- Friend Party.
- Study Group.
- Quest Party.
- Competition Team.
- Class Group.
- Custom Group.

## 3. Party Lobby

Tạo lobby riêng cho mỗi Party.

Các chức năng:

- Danh sách thành viên.
- Avatar và trạng thái online.
- Ready / Not Ready.
- Leader Controls.
- Activity Selection.
- Game Mode Selection.
- Invitation.
- Group Chat.
- Start Session.

## 4. Cooperative Gameplay

Các thành viên trong Party có thể:

- Cùng di chuyển trong game world.
- Cùng khám phá bản đồ.
- Cùng thực hiện Quest.
- Cùng giải quyết Educational Puzzle.
- Cùng tham gia Learning Challenge.
- Cùng tham gia Team Competition.
- Cùng hoàn thành nhiệm vụ có nhiều bước.

Hỗ trợ Shared Quest Progress theo cấu hình nhiệm vụ.

Không mặc định chia sẻ kết quả học tập cá nhân giữa các thành viên.

Mỗi học sinh vẫn phải có learning attempt và kết quả riêng.

## 5. Party Session

Khi Party bắt đầu hoạt động:

1. Backend xác nhận Leader.
2. Kiểm tra thành viên.
3. Kiểm tra điều kiện lớp học và quyền tham gia.
4. Tạo Game Session.
5. Gán các thành viên.
6. Gửi thông tin session cho Godot clients.
7. Đồng bộ trạng thái người chơi.
8. Theo dõi reconnect và disconnect.
9. Kết thúc session và lưu kết quả.

## 6. Group Rules

- MVP: một học sinh chỉ có một active Party tại một thời điểm.
- Một Party có configurable maximum members.
- Leader có quyền quản lý Party.
- Có cơ chế chuyển Leader.
- Có cơ chế xử lý Leader disconnect.
- Party riêng tư cần lời mời hoặc quyền tham gia.
- Có thể cấu hình giới hạn theo school, class, grade và activity.
- Không cho phép người dùng vượt qua các điều kiện bảo vệ học sinh.
- Chống gửi lời mời spam.
- Chống duplicate membership.
- Có rate limiting và audit log.

## 7. Technical Architecture

Golang modules:

- Party Service.
- Party Member Service.
- Party Invitation Service.
- Party Lobby Service.
- Cooperative Session Service.

Valkey:

- Party Presence.
- Realtime Party State.
- Lobby State.
- Invitation Cache.
- Realtime Event Distribution.

PostgreSQL:

- Party.
- Party Member.
- Party Invitation.
- Party History.
- Party Activity.
- Session Membership.

Godot:

- Party UI.
- Party Lobby.
- Friend Invitation UI.
- Member Status.
- Leader Controls.
- Group Chat.
- Cooperative Gameplay.

## 8. Acceptance Criteria

- Học sinh A có thể tạo Party.
- Học sinh A có thể mời B, C, D.
- B, C, D có thể chấp nhận lời mời.
- Các thành viên nhìn thấy nhau trong Party Lobby.
- Leader có thể chọn hoạt động.
- Tất cả thành viên đủ điều kiện có thể tham gia cùng Game Session.
- Các thành viên có thể cùng thực hiện nhiệm vụ.
- Khi một thành viên mất kết nối, những thành viên còn lại vẫn tiếp tục nếu chế độ chơi cho phép.
- Khi Leader mất kết nối, hệ thống có chính sách chuyển Leader hoặc phục hồi.
- Kết quả học tập của từng thành viên được ghi nhận riêng.
- Không có thành viên trái phép tham gia Party riêng.

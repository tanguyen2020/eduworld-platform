# ADR-0072 — Bỏ vai trò giáo viên: học sinh tự học, bỏ hoạt động chấm tay, kiểm duyệt tập trung ở platform

> **Trạng thái:** Accepted · một phần superseded by ADR-0081 · một phần superseded by ADR-0084 · **Ngày:** 2026-11-15 · **OQ:** OQ-07 · **Thay một phần:** ADR-0020, ADR-0021, ADR-0027, ADR-0048, ADR-0050, ADR-0053, ADR-0059, ADR-0062, ADR-0066 · **Tài liệu:** [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [MODERATION](../03-social-multiplayer/MODERATION.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0081](ADR-0081-user-type-student-staff-phan-biet-bang-tenant.md).** Không còn hiệu lực: giá trị `admin` của `user_type` (mục 1). **Còn hiệu lực:** toàn bộ phần còn lại; `user_type` chỉ gồm `student`, `staff`.

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0084](ADR-0084-nhom-hoc-tap-thuoc-learning-service.md):** study group (mục 2) là nhóm LMS do learning-service sở hữu, không phải một loại club theo ADR-0050.

## Bối cảnh

Baseline ban đầu (theo Master Prompt, `VIBE_CODING_PROMPT.md` §8) có vai trò **giáo viên** trên Admin Portal: giao bài cho lớp được phân công, chấm tự luận/bài nói/project theo rubric, xem Teacher Dashboard, điều khiển channel chat lớp, tạo study group, xử lý vụ việc chat. Giáo viên chưa bao giờ vào game (ADR-0031).

Khi review độ phù hợp cho game giáo dục ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md)), Technical Architect quyết định: **game và CMS/LMS không cần giáo viên giám sát**. Sản phẩm định vị là môi trường **học sinh tự học**, hệ thống tự chấm, tự gợi ý; nhà trường quản trị ở cấp trường. Quyết định này kéo theo hai câu hỏi phải chốt cùng lúc: ai chấm các hoạt động cần chấm tay, và ai xử lý vụ việc an toàn trẻ em mà trước đây giáo viên/moderator trường làm.

Quyết định này ghi đè một phần yêu cầu gốc trong `VIBE_CODING_PROMPT.md` (file lưu nguyên văn, không chỉnh sửa theo ghi chú đầu file). PRD đánh dấu các yêu cầu liên quan là "Removed (ADR-0072)".

## Phương án

1. Bỏ hẳn vai trò giáo viên (chọn).
2. Giữ giáo viên để giao bài và xem kết quả, chỉ bỏ các chức năng giám sát.
3. Giáo viên chỉ xem báo cáo.

## Quyết định

### 1. Không còn vai trò giáo viên [Confirmed]

- Không có role `teacher`, không có `user_type = teacher`. `user_type` còn `student`, `staff`, `admin`.
- Không nhập giáo viên và phân công giảng dạy từ SIS: EduWorld SIS Contract v1 bỏ thực thể `teacher`, `teaching_assignment` và trường `homeroom_teacher_external_id`; identity-service bỏ `org.teachers`, `org.teaching_assignments` và các event `teaching_assignment.changed`.
- Không còn phạm vi phân quyền `class` hay `class × subject` theo phân công giảng dạy. Phạm vi `subject` vẫn giữ cho `academic_manager`.
- Role tenant sau thay đổi: `tenant_admin`, `school_admin`, `academic_manager`, `tenant_content_editor`, `tenant_content_reviewer`. Bỏ `teacher` và `tenant_moderator`.
- Role platform sau thay đổi: `system_admin`, `content_admin`, `content_reviewer`, `game_designer`, `liveops`, `competition_admin` và **thêm `safety_moderator`** (mục 4).
- Portal vẫn chỉ dành cho nhân sự (realm role `tenant_staff`/`platform_staff`, ADR-0066); học sinh vẫn không vào được.

### 2. Học sinh tự học; nhà trường quản trị [Confirmed]

- **Con đường học chính** là tự học: learning path, gợi ý ôn tập theo mastery và đồ thị LO tiên quyết ([LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)), quest học tập trong game. Hệ thống thay vai trò "giáo viên chỉ định ôn tập" bằng gợi ý tự động có giải thích.
- **LMS giữ lại** ở cấp trường:
  - Course = lớp × môn × năm học, tạo tự động từ lớp (SIS) và môn theo khối của curriculum (không còn dựa vào phân công giảng dạy).
  - Learning path, assignment, schedule, exam theo blueprint: người tạo là `school_admin` hoặc `academic_manager`; đối tượng là lớp, nhóm học tập hoặc danh sách học sinh.
  - Thử thách lớp và giải đấu trường từ template: chỉ `school_admin`.
  - Nhóm học tập (study group, một loại club theo ADR-0050): do `school_admin`/`academic_manager` tạo, không do giáo viên.
- **Dashboard:** Teacher Dashboard được thay bằng **School Learning Dashboard** cho `school_admin`/`academic_manager`, xem theo trường → lớp → học sinh, cùng dữ liệu rollup (ADR-0062). Kết quả là tham khảo cho nhà trường, không thay đánh giá chính thức theo quy định của Bộ.
- **Trong game:** tab "Thầy cô giao" đổi thành "Nhà trường giao"; không còn phản hồi/nhận xét của giáo viên trong màn kết quả.
- **Chế độ "trong giờ học"** do giáo viên khóa Arena/Social được thay bằng **khung giờ truy cập** do `school_admin` đặt ở cấp trường (đã có).

### 3. Bỏ hoạt động cần chấm tay khỏi Release [Confirmed]

- Chỉ giữ hoạt động server tự chấm được: `quiz`, `interactive_exercise`, `minigame`, `video_lesson` (câu hỏi chèn tự chấm), `audio_lesson` (nghe hiểu, câu hỏi tự chấm), `simulation` (checker Go, ADR-0052).
- **Ngoài Release:** `written_response`, `speaking`, `project`, rubric, hàng đợi chấm, trả bài, chấm lại thủ công. Tài liệu [WRITTEN_RESPONSE_RUBRIC](../99-deferred/WRITTEN_RESPONSE_RUBRIC.md) giữ lại như thiết kế tham khảo, gắn nhãn ngoài Release. Mở lại phải có ADR mới (ví dụ chấm tự động có AI, ADR-0053).
- Loại câu hỏi `short_answer` vẫn giữ (chấm tự động theo đáp án chấp nhận + chuẩn hóa).
- `AttemptStatus` không còn `pending_review`, `returned`. Chiến lược chấm chỉ là tự động. Chấm lại chỉ còn **re-evaluate do hệ thống** khi nội dung có lỗi (`attempt.re_evaluated`), do `content_admin` hoặc `academic_manager` kích hoạt, có audit.

### 4. Kiểm duyệt tập trung ở Platform Safety Team [Confirmed]

- Thêm role platform **`safety_moderator`**: `moderation.case.*`, `moderation.sanction.*`, đọc chat trong phạm vi case, `chat.sensitive_terms.read`. Đăng nhập portal bắt buộc MFA như mọi role platform.
- `safety_moderator` được đọc liên tenant **chỉ trong module moderation** (case, bằng chứng, tin nhắn liên quan case) qua `tenancy.Bypass(ctx, "moderation_case")`, có audit và metric. Quyền đọc liên tenant đầy đủ vẫn chỉ thuộc `system_admin` (cập nhật AGENTS.md R20).
- Danh mục từ nhạy cảm cấp tenant: chỉ Safety Team ghi (theo yêu cầu của tenant); không còn quyền `chat.tenant_terms.write` cho role tenant.
- Nhà trường nhận **thông báo** về vụ việc nghiêm trọng liên quan học sinh của trường (tóm tắt, không có nội dung chat thô) để liên hệ phụ huynh; case lưu thời điểm yêu cầu và thời điểm trường xác nhận đã liên hệ.
- Channel chat lớp không còn điều khiển của giáo viên; channel lớp chịu chat policy cấp trường do `school_admin` đặt trong giới hạn platform (ADR-0020, ADR-0030). Quy tắc chat như nhau cho mọi lớp (R15) không đổi.

### 5. Mốc dự án

Mốc M3 "Teacher Loop" đổi thành **M3 School Loop**: nhà trường giao bài/đề → học sinh làm trong game → nhà trường xem kết quả và LO yếu, học sinh nhận gợi ý ôn tập tự động.

## Hệ quả

(+) Phạm vi gọn hơn đáng kể: bỏ hàng đợi chấm, rubric, ba loại activity, đồng bộ giáo viên từ SIS, điều khiển lớp. Mọi kết quả học tập do server tự chấm, khớp ADR-0016. Kiểm duyệt nhất quán giữa các trường.

(−) Mất kỹ năng sản sinh (viết, nói) trong Release; cần lộ trình chấm tự động nếu muốn đưa lại. Chất lượng gợi ý tự động trở thành yếu tố quyết định hiệu quả học tập (xem [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) PED-04, PED-05, PED-08). Safety Team platform cần nhân sự và lịch trực đủ cho mọi tenant. Phải sửa PRD, ADMIN_PORTAL, SCHOOL_LMS, ACTIVITY_ENGINE, CHAT/MODERATION, SIS contract, OpenAPI, TEST_STRATEGY, ROADMAP trong cùng đợt.

## Phương án đã loại

- **Giữ giáo viên giao bài/xem kết quả (phương án 2)** và **giáo viên chỉ xem báo cáo (phương án 3):** không chọn vì định hướng sản phẩm là học sinh tự học, nhà trường quản trị.
- **AI chấm tự động bài viết/nói trong Release:** chưa chọn; giữ ADR-0053 ở trạng thái Proposed, ngoài Release.
- **Moderator của từng trường:** không chọn; kiểm duyệt tập trung để nhất quán và giảm gánh nặng cho trường.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR; trả lời OQ-07 (không có giáo viên) | Technical Architect |
| 2026-11-15 | Một phần superseded by ADR-0081 (`user_type`) | Technical Architect |
| 2026-11-16 | Một phần superseded by ADR-0084 (study group) | Technical Architect |

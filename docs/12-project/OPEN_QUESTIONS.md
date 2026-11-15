# Open Questions

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: Product Manager + Solution Architect · Cập nhật: 2026-11-15
> Liên quan: [ADR](../adr/README.md) · [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md) · [RISKS](RISKS.md) · [ROADMAP](ROADMAP.md) · [DOCUMENTATION_AUDIT](DOCUMENTATION_AUDIT.md)

Nguyên tắc (Master Prompt §25): nếu chưa có câu trả lời, dùng **phương án mặc định [Proposed]** để tiếp tục các phần không bị chặn; ghi rõ phase nào **bắt buộc** phải có câu trả lời.

## 1. Danh sách câu hỏi

| ID | Câu hỏi | Mặc định đề xuất (Proposed) | Ảnh hưởng | Cần trước | Người trả lời | Trạng thái |
|---|---|---|---|---|---|---|
| OQ-01 | Trường nào cung cấp API theo EduWorld SIS Contract v1 và khi nào? Phương thức xác thực (OAuth2 client credentials / API key / mTLS)? Có hỗ trợ delta, webhook, student lookup không? Có môi trường test không? | **Đã chốt hướng (ADR-0021):** EduWorld sở hữu DTO và API contract (`specs/sis/`); trường tự dựng facade. Trường chưa có API dùng `csv_import` | SIS_INTEGRATION, ACCOUNT_PROVISIONING | Cuối Phase 1 (danh sách trường pilot) | Nhà trường | Partially answered |
| OQ-02 | Keycloak một realm hay nhiều realm? Trường có IdP riêng không (Google/Microsoft/SIS SSO)? Ai quản trị Keycloak? | Một realm + Organizations + brokering (ADR-0006) | KEYCLOAK_SSO, MULTITENANCY | Giữa Phase 1 | Vận hành + đối tác | Open |
| OQ-03 | Quy mô: số trường, số học sinh, CCU mục tiêu cho pilot và năm 1? Giờ cao điểm? | Pilot 5 trường/5.000 HS/1.000 CCU; năm 1: 50.000 HS/5.000 CCU | Capacity, load test, chi phí hạ tầng | Phase 4 (trước load test) | Product owner | Partially answered (10.000 CCU, ADR-0045) |
| OQ-04 | Nền tảng: Windows, Android, iOS, Web? Thiết bị phổ biến tại trường (phòng máy, tablet dùng chung)? | Release 1.0 hỗ trợ cả 4 nền tảng Windows, Android, iOS, Web (ADR-0013); Windows + Android phát triển trước, iOS/Web hoàn thiện Phase 6. Cần khảo sát để chốt phiên bản OS/trình duyệt tối thiểu và device matrix. Device Authorization Grant cho máy dùng chung [TBD] | GODOT_ARCHITECTURE, ngân sách hiệu năng, login flow | Đầu Phase 1 | Product owner + khảo sát trường | Open |
| OQ-05 | Thế giới game: persistent world, multiplayer hub hay hybrid? Số người tối đa thấy nhau trong 1 khu? | Hybrid: hub instance ≤40 người + private instance + match room (ADR-0012) | 3D_WORLD_DESIGN, REALTIME | Đầu Phase 2 | Game Director + PO | Open |
| OQ-06 | Realtime transport cho gameplay? Có chế độ nào cần vật lý thời gian thực không? | WebSocket qua Go realtime server (ADR-0011) | GAME_NETWORKING | Đầu Phase 4 | Multiplayer Engineer | Open |
| OQ-07 | Mô hình quản trị CMS: Platform Admin, Sở/Phòng GD, School Admin, Teacher, tổ bộ môn? Giáo viên có được tạo câu hỏi riêng không? | 6 vai trò như Master Prompt §8; giáo viên tạo nội dung phạm vi trường, cần reviewer cấp trường duyệt | CMS_DESIGN, RBAC | Đầu Phase 3 | PO + đối tác | Open |
| OQ-08 | Chương trình học áp dụng: GDPT 2018? Bộ sách nào (Kết nối tri thức, Chân trời sáng tạo, Cánh Diều...)? Nguồn nội dung và bản quyền? | Model hỗ trợ nhiều curriculum/version; seed GDPT 2018 khung môn Toán lớp 1–5 cho pilot | CURRICULUM_MODEL, nội dung pilot | Đầu Phase 2 | EdTech Specialist + PO | Open |
| OQ-09 | Chính sách chat? | **Đã trả lời (ADR-0020):** một bộ quy tắc chung cho mọi lớp 1–9, đủ 8 loại channel (gồm `match`), mọi tin nhắn phải qua kiểm tra hợp lệ phía server trước khi phát. Còn mở: khung giờ chat mặc định và có cho World chat toàn organization hay không | CHAT_SYSTEM, MODERATION, CHILD_SAFETY | Đầu Phase 5 | PO + trường + tư vấn pháp lý | Answered (chi tiết còn mở) |
| OQ-10 | Hạ tầng đặt ở đâu: cloud (nhà cung cấp tại VN?) hay on-prem? Dùng dịch vụ managed (PostgreSQL, Valkey, object storage) hay tự vận hành? | **Nền tảng đã chốt: Docker (Compose local, Docker Swarm staging/production), chưa dùng Kubernetes (ADR-0024).** Còn mở: cloud hay on-prem, managed hay tự vận hành | DEPLOYMENT | Đầu Phase 6 (staging từ Phase 1) | DevOps + PO | Open |
| OQ-11 | SLA và chỉ tiêu hiệu năng? Khung giờ bảo trì? | 99.5% trong giờ học; p95 REST < 200ms đọc | MONITORING, DEPLOYMENT | Phase 6 | PO | Open |
| OQ-12 | Chính sách lưu trữ dữ liệu, thời gian giữ, ai được truy cập dữ liệu học sinh, yêu cầu pháp lý (Luật Bảo vệ dữ liệu cá nhân, Nghị định 13/2023/NĐ-CP và văn bản thay thế/hướng dẫn, quy định ngành giáo dục)? Có cần đồng ý của phụ huynh? | Dữ liệu lưu tại VN; chat 180 ngày; learning record suốt thời gian học + 1 năm; xin đồng ý qua nhà trường | DATA_PRIVACY, retention jobs | Trước Phase 5 (chat) và trước Production | Tư vấn pháp lý | Open |
| OQ-13 | Có vai trò phụ huynh (parent portal) không? | Không có trong Master Prompt nên chưa đưa vào Release 1.0; model identity không chặn việc thêm vai trò `parent` sau khi có quyết định | PRD, identity | Phase 3 | PO | Open |
| OQ-14 | Có cơ chế kinh doanh/thanh toán không (B2B license theo trường, B2C)? | B2B theo trường, không có in-app purchase | Gamification, reward | Phase 2 | PO | Open |
| OQ-15 | Ngôn ngữ giao diện ngoài tiếng Việt? Hỗ trợ học sinh dân tộc thiểu số/khiếm khuyết? | vi-VN mặc định, kiến trúc i18n sẵn, en-US cho môn Tiếng Anh | UI/UX, content | Phase 2 | PO | **Answered** (ADR-0056): song ngữ vi-VN + en-US |
| OQ-16 | Nguồn asset 3D/âm thanh (tự làm, thuê ngoài, mua store)? Phong cách cụ thể? | Stylized low-poly; mua pack có license thương mại + tự làm nhân vật chính | WORLD_DESIGN, WORLD_RENDERING, ngân sách | Đầu Phase 2 | Game Director | Open |
| OQ-17 | Thời gian chơi tối đa/ngày, giờ giới nghiêm theo độ tuổi? Ai cấu hình (trường/phụ huynh)? | Nhắc nghỉ sau 45 phút; trường cấu hình giờ truy cập | CHILD_SAFETY, gamification | Phase 2 | PO + chuyên gia giáo dục | Open |
| OQ-18 | Kỳ thi chính thức có tổ chức trong EduWorld không hay chỉ luyện thi? Có yêu cầu giám sát (proctoring)? | Chỉ luyện/thi thử + kiểm tra do giáo viên giao; không làm thi chính thức có giá trị pháp lý trong Release 1.0 khi chưa có quyết định | EXAM_ENGINE, ANTI_CHEAT | Phase 3 | PO + trường | Open |
| OQ-19 | Voice chat có nằm trong Release 1.0 không? Nếu có: phạm vi (chỉ party/team hay cả class), ghi âm/kiểm duyệt giọng nói, đồng ý của phụ huynh, giới hạn thời gian | Master Prompt §6.2 chỉ yêu cầu thiết kế mở rộng → thiết kế sẵn extension point (API, UI, `ChatPolicy.voice`), **triển khai sau khi có quyết định** | CHAT_SYSTEM, MODERATION, DATA_PRIVACY, REALTIME_ARCHITECTURE | Đầu Phase 5 | PO + trường + tư vấn pháp lý | **Answered** (ADR-0070): không có voice chat trong Release 1.0 |
| OQ-20 | Khi luật bảo vệ dữ liệu cá nhân yêu cầu xóa, hoặc dữ liệu hết thời hạn lưu trữ (chat, log, learning event), có được xóa vật lý không? Ai phê duyệt? | Ứng dụng chỉ soft-delete và ẩn danh hóa (ADR-0028). Xóa vật lý/gỡ partition chỉ qua quy trình vận hành có phê duyệt, ngoài ứng dụng | DATA_PRIVACY, BACKUP_RECOVERY, DATABASE_DESIGN | Trước Phase 5 | Tư vấn pháp lý + PO | **Answered** (ADR-0070): Release 1.0 không xóa vật lý; chỉ soft-delete + ẩn danh hóa; ngoại lệ duy nhất ADR-0033 |
| OQ-21 | Object storage cho staging/production là gì: dịch vụ S3-compatible của cloud tại Việt Nam (nhà cung cấp nào) hay cụm MinIO tự vận hành? floci có được dùng ở môi trường nào ngoài local/CI không? | floci cho local/dev/CI (đã chỉ định); staging/production dùng S3-compatible thật (ADR-0044) | DEPLOYMENT, BACKUP_RECOVERY, chi phí CDN | Trước khi dựng staging (Phase 1) | DevOps + PO | **Answered** (ADR-0071): floci cho mọi môi trường, kể cả staging/production |
| OQ-22 | Chọn nhà cung cấp nhận dạng giọng nói/chấm phát âm (tiếng Việt, tiếng Anh) cho hoạt động speaking? Xử lý dữ liệu giọng nói trẻ em ở đâu? | Chưa chọn: bài speaking chấm tay bởi giáo viên; adapter nhà cung cấp sẵn sàng; dữ liệu giọng nói xử lý tại Việt Nam (OQ-12), xóa ghi âm sau 90 ngày kể từ khi chấm | AUDIO_LANGUAGE_LEARNING, DATA_PRIVACY | Trước Phase 4 | PO + EdTech + tư vấn pháp lý | **Answered** (ADR-0059): không làm AI giọng nói trong Release 1.0, giáo viên chấm tay bài nói |

## 2. Quy trình đóng câu hỏi

1. Người trả lời ghi quyết định + ngày + nguồn (biên bản họp/email).
2. Cập nhật trạng thái → `Answered`.
3. Cập nhật ADR liên quan (Proposed → Accepted, hoặc ADR mới nếu khác mặc định).
4. Đổi nhãn `[TBD]`/`[Proposed]` trong các tài liệu ảnh hưởng.
5. Ghi CHANGELOG.

## 3. Nhật ký trả lời

| Ngày | OQ | Quyết định | Nguồn |
|---|---|---|---|
| 2026-11-15 | OQ-01 (hướng tích hợp) | DTO/API do EduWorld định nghĩa; mỗi trường cung cấp API theo contract, không dùng DTO của SIS → ADR-0021 Accepted | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | OQ-09 | Chat không phân biệt theo lớp; mọi tin nhắn phải hợp lệ → ADR-0020 Accepted | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Phạm vi | Làm game hoàn chỉnh (Release 1.0 đầy đủ chức năng), không chỉ MVP | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | ADR-0022 | Frontend CMS/LMS/Admin dùng React | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Tenancy | Tenant = trường độc lập hoặc đơn vị nhiều trường; có tenant chung cho học sinh không thuộc tenant nào; mọi truy vấn lọc theo tenant_id; một tenant tại một thời điểm; System Admin xem liên tenant với dropdown lọc → ADR-0005, ADR-0026, ADR-0027 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Xóa dữ liệu | Chỉ soft-delete, không hard-delete, không đọc dữ liệu đã xóa → ADR-0028 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Dữ liệu dùng chung | System Admin gán dữ liệu cho tenant và clone giữa tenant → ADR-0029 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | CMS | Vận hành tenant chỉ thao tác Learning Management, không chạm cấu hình game → ADR-0027 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Chat | Danh mục từ nhạy cảm + công tắc tắt được toàn bộ pipeline kiểm tra → ADR-0030 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Game | Chỉ học sinh vào game, lưu thời gian login, một thiết bị/tài khoản, worker dọn học sinh không login → ADR-0031, ADR-0032 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Chat | Chat luôn có trong 1v1/đội/party; purge nhóm chat gameplay; bộ tính năng chat đầy đủ; NATS + Valkey Pub/Sub, lưu PostgreSQL → ADR-0020, ADR-0033, ADR-0034, ADR-0035 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Kiến trúc | Giữ Golang; microservices theo hệ thống lớn; DB riêng mỗi service; CQRS + Clean Architecture; ORM tương đương EF (ent); typed client tương đương Refit; mọi string có maxLength → ADR-0036…0042 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | OQ-03 (CCU) | Mục tiêu 10.000 người chơi đồng thời → ADR-0045; số trường/học sinh tổng còn mở | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | S3 | Dùng floci cho S3 (local/dev/CI) → ADR-0044; môi trường production còn mở ở OQ-21 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Công thức | MathJax 4 + MathLive; game chỉ hiển thị công thức → ADR-0055 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Audit | Giữ 6 microservice, giữ chat như nhau mọi lớp, một Admin Portal 5 workspace, chỉ bổ sung phần thiếu quan trọng → DOCUMENTATION_AUDIT §4.1 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | Nội dung | Draft/Publish, giữ phiên bản khi đang chơi, xem/rollback version, publish lên cache; media ở object storage + CDN → ADR-0043, ADR-0044 | Stakeholder (hội thoại thiết kế) |
| 2026-11-15 | OQ-15, thế giới | Game song ngữ Việt – Anh; người duyệt chốt bộ tên thế giới/chương/NPC → ADR-0056 | Stakeholder (người duyệt) |
| 2026-11-15 | Hiệu năng | Bắt buộc chạy trên máy tầm trung, bố cục web/mobile → ADR-0057 | Stakeholder (người duyệt) |
| 2026-11-15 | OQ-22 | Chưa làm AI giọng nói trong phiên bản này → ADR-0059 | Stakeholder (người duyệt) |
| 2026-11-15 | Danh mục | Danh mục quốc gia/34 tỉnh thành/trường; học sinh thuộc một trường một lớp → ADR-0060 | Stakeholder (người duyệt) |
| 2026-11-15 | SIS | Đồng bộ SIS sang game khi đăng nhập → ADR-0061 | Stakeholder (người duyệt) |
| 2026-11-15 | Dashboard | Dashboard dữ liệu lớn theo ngày, không làm treo DB → ADR-0062 | Stakeholder (người duyệt) |
| 2026-11-15 | Đăng nhập | Không chọn trường; học sinh chỉ username/password; không thuộc tenant chung/SIS thì không cho đăng nhập → ADR-0063 | Stakeholder (người duyệt) |
| 2026-11-15 | Đăng nhập | Mở lại game phải nhập lại mật khẩu, không auto-login → ADR-0064 | Stakeholder (người duyệt) |
| 2026-11-15 | Tenancy | tenant_id bắt buộc, gắn trên header X-Tenant-Id → ADR-0065 | Stakeholder (người duyệt) |
| 2026-11-15 | OQ-07, Portal | Portal có màn đăng nhập riêng, học sinh không được vào, phân quyền Role/UserRole → ADR-0066 | Stakeholder (người duyệt) |
| 2026-11-15 | Đăng nhập portal | Portal không chọn tenant, xử lý như game; username/email duy nhất (toàn hệ thống để đăng nhập không chọn tenant) → ADR-0067 | Stakeholder (người duyệt) |
| 2026-11-15 | Form đăng nhập game | Ô nhập username/password trong game, xác thực qua backend → ADR-0068 | Stakeholder (người duyệt) |
| 2026-11-15 | Định danh | Username/email không trùng trong tenant; đăng nhập SSO xác thực trước, identity kiểm tra tenant sau; một người một tài khoản SSO; không chọn tenant → ADR-0069 (làm rõ ADR-0067 mục 2) | Stakeholder (người duyệt) |
| 2026-11-15 | OQ-19 | Không làm voice chat trong version này → ADR-0070 | Stakeholder (người duyệt) |
| 2026-11-15 | OQ-20, analytics | Không xóa vật lý theo luật trong version này; không làm ngoại lệ purge dữ liệu analytics thô → ADR-0070 | Stakeholder (người duyệt) |
| 2026-11-15 | OQ-21 | Object storage staging/production lưu trữ trên floci → ADR-0071 | Stakeholder (người duyệt) |

# Architecture Decision Records (ADR)

> Trạng thái: Review · Phiên bản: 1.1 · Owner: Solution Architect · Cập nhật: 2026-11-21 (ADR-0095 bỏ khái niệm lớp; ADR-0084 Superseded) · 2026-11-20 (owner chốt toàn bộ ADR còn `Proposed`: chuyển `Accepted`, trừ ADR-0053 và ADR-0077 hoãn theo ADR-0082; ADR-0007 Superseded)
> Liên quan: [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) · [OPEN_QUESTIONS](../12-project/OPEN_QUESTIONS.md) · [DOCUMENTATION_AUDIT](../12-project/DOCUMENTATION_AUDIT.md)

ADR ghi lại các quyết định kiến trúc và sản phẩm quan trọng của EduWorld: bối cảnh, phương án, quyết định, hệ quả và lịch sử thay đổi. Mỗi ADR là một file `ADR-NNNN-<slug>.md`. Người mới nên đọc mục **Quyết định hiện hành theo chủ đề** trước, không cần đọc ADR đã bị thay.

## Quy trình

- Trạng thái: `Proposed` → `Accepted` → `Superseded by ADR-xxxx` (toàn phần) hoặc "một phần superseded" (có khối cảnh báo ghi rõ phần còn/không còn hiệu lực).
- `Proposed` là mặc định làm việc; phải được review và chuyển `Accepted` trước khi phase phụ thuộc vào nó kết thúc.
- Ngày 2026-11-20 owner dự án chốt mọi ADR còn `Proposed`: chuyển `Accepted` (ghi dòng "Owner chốt (Accepted)" ở Lịch sử thay đổi của từng ADR), trừ ADR-0053 và ADR-0077 (giữ `Proposed · hoãn`, ADR-0082) và ADR-0007 (cả phần game và portal đã bị thay bởi ADR-0068, ADR-0091 → Superseded). ADR mới viết sau ngày đó vẫn bắt đầu ở `Proposed`.
- ADR `Accepted` không đổi ý nghĩa quyết định; thay đổi = ADR mới. Điều chỉnh tham chiếu (tên thư mục, tên service) cập nhật trực tiếp và ghi vào **Lịch sử thay đổi** của ADR.
- ADR mới theo [TEMPLATE.md](TEMPLATE.md); thêm dòng vào bảng chỉ mục và nhóm chủ đề dưới đây.
- Câu hỏi chưa có quyết định nằm ở [OPEN_QUESTIONS](../12-project/OPEN_QUESTIONS.md).

## Chỉ mục

| ADR | Tiêu đề | Trạng thái | OQ |
|---|---|---|---|
| [0001](ADR-0001-go-modular-monolith-voi-3-tien-trinh-trien-khai.md) | Go Modular Monolith với 3 tiến trình triển khai | Superseded by ADR-0036 | — |
| [0002](ADR-0002-monorepo.md) | Monorepo | Accepted · cấu trúc thư mục superseded by ADR-0074 và ADR-0092 | — |
| [0003](ADR-0003-postgresql-la-nguon-du-lieu-ben-vung-duy-nhat-schema-theo-mo.md) | PostgreSQL là nguồn dữ liệu bền vững duy nhất, schema theo module | Accepted · một phần superseded by ADR-0037 và ADR-0039 | OQ-10 |
| [0004](ADR-0004-valkey-chi-cho-trang-thai-tam-thoi-cache-va-phoi-hop-realtim.md) | Valkey chỉ cho trạng thái tạm thời, cache và phối hợp realtime | Accepted | — |
| [0005](ADR-0005-mo-hinh-multi-tenancy-shared-database-tenant-id-rls.md) | Mô hình multi-tenancy: shared database + `tenant_id` + RLS | Accepted · một phần superseded by ADR-0095 | — |
| [0006](ADR-0006-keycloak-mot-realm-dung-organizations-identity-brokering.md) | Keycloak: một Realm dùng Organizations + Identity Brokering | Accepted · một phần superseded by ADR-0026 và ADR-0027 | OQ-02 |
| [0007](ADR-0007-godot-dang-nhap-bang-oidc-authorization-code-pkce-qua-system.md) | Godot đăng nhập bằng OIDC Authorization Code + PKCE qua system browser | Superseded by ADR-0068 (game) và ADR-0091 (portal) | OQ-04 |
| [0008](ADR-0008-access-token-cua-keycloak-la-credential-api-realtime-dung-ti.md) | Access token của Keycloak là credential API; realtime dùng ticket ngắn hạn | Accepted | — |
| [0009](ADR-0009-apache-apisix-lam-api-gateway-backend-van-tu-authorize.md) | Apache APISIX làm API gateway, backend vẫn tự authorize | Accepted · một phần superseded by ADR-0094 | — |
| [0010](ADR-0010-rest-openapi-3-1-contract-first-problem-details-cursor-pagin.md) | REST + OpenAPI 3.1 contract-first, Problem Details, cursor pagination, Idempotency-Key | Accepted | — |
| [0011](ADR-0011-realtime-transport-websocket-qua-go-realtime-server.md) | Realtime transport: WebSocket qua Go realtime server | Accepted | OQ-05, OQ-06 |
| [0012](ADR-0012-the-gioi-game-hybrid-hub-instance-private-instance-match-roo.md) | Thế giới game Hybrid: hub instance + private instance + match room | Accepted | OQ-05 |
| [0013](ADR-0013-nen-tang-client-release-1-0-gom-windows-android-ios-web.md) | Nền tảng client: Release 1.0 gồm Windows, Android, iOS, Web | Accepted · phần ngân sách hiệu năng superseded by ADR-0057 | OQ-04 |
| [0014](ADR-0014-godot-4-x-voi-gdscript-co-static-typing.md) | Godot 4.x với GDScript có static typing | Accepted | OQ-04 |
| [0015](ADR-0015-content-versioning-bat-bien-sau-publish.md) | Content versioning bất biến sau Publish | Accepted · cách rollback superseded by ADR-0043 | — |
| [0016](ADR-0016-server-authoritative-scoring-answer-key-khong-bao-gio-roi-se.md) | Server-authoritative scoring; answer key không bao giờ rời server | Accepted | — |
| [0017](ADR-0017-mastery-model-co-the-cau-hinh-mac-dinh-weighted-evidence.md) | Mastery model có thể cấu hình, mặc định Weighted Evidence | Accepted | OQ-08 |
| [0018](ADR-0018-glicko-2-cho-game-rating-tach-biet-learning-mastery.md) | Glicko-2 cho Game Rating, tách biệt Learning Mastery | Accepted | — |
| [0019](ADR-0019-transactional-outbox-valkey-streams-cho-domain-events.md) | Transactional Outbox + Valkey Streams cho domain events | Accepted · transport superseded by ADR-0042 | — |
| [0020](ADR-0020-chat-thong-nhat-cho-moi-lop-moi-tin-nhan-phai-qua-kiem-tra-h.md) | Chat thống nhất cho mọi lớp, mọi tin nhắn phải qua kiểm tra hợp lệ | Accepted · một phần superseded by ADR-0072 và ADR-0095 | — |
| [0021](ADR-0021-sis-integration-theo-contract-do-eduworld-so-huu.md) | SIS integration theo contract do EduWorld sở hữu | Accepted · một phần superseded by ADR-0072 và ADR-0095 | OQ-01 |
| [0022](ADR-0022-cms-lms-web-la-react-typescript-spa.md) | Admin Portal (CMS/LMS) là React + TypeScript SPA | Accepted · một phần superseded by ADR-0094 | — |
| [0023](ADR-0023-observability-bang-opentelemetry-prometheus-grafana-loki-tem.md) | Observability bằng OpenTelemetry + Prometheus + Grafana + Loki + Tempo | Accepted | — |
| [0024](ADR-0024-chay-tren-docker-compose-cho-local-docker-swarm-cho-staging.md) | Chạy trên Docker: Compose cho local, Docker Swarm cho staging/production, chưa dùng Kubernetes | Accepted | OQ-10 |
| [0025](ADR-0025-thu-vien-nen-tang-cho-go-backend.md) | Thư viện nền tảng cho Go backend | Accepted · một phần superseded by ADR-0039 và ADR-0040 | — |
| [0026](ADR-0026-dang-nhap-theo-tenant-xac-dinh-tenant-kiem-tra-cheo-mac-dinh.md) | Đăng nhập theo tenant: xác định tenant, kiểm tra chéo, mặc định tenant chung | Accepted · supersedes phần ánh xạ Organization của ADR-0006 · một phần superseded by ADR-0095 | — |
| [0027](ADR-0027-cms-tach-vung-tenant-chi-thao-tac-learning-management-cau-hi.md) | Admin Portal: một portal 5 workspace; tenant chỉ thao tác Learning Management, cấu hình game chỉ thuộc platform | Accepted · một phần superseded by ADR-0072 · một phần superseded by ADR-0094 · một phần superseded by ADR-0095 | — |
| [0028](ADR-0028-chi-soft-delete-khong-hard-delete-mac-dinh-khong-doc-du-lieu.md) | Chỉ soft-delete, không hard-delete; mặc định không đọc dữ liệu đã xóa | Accepted | OQ-20 |
| [0029](ADR-0029-system-admin-gan-du-lieu-cho-tenant-va-clone-du-lieu-giua-te.md) | System Admin gán dữ liệu cho tenant và clone dữ liệu giữa tenant | Accepted · một phần superseded by ADR-0094 | — |
| [0030](ADR-0030-chinh-sach-tu-nhay-cam-trong-chat-danh-muc-quan-ly-duoc-va-c.md) | Chính sách từ nhạy cảm trong chat: danh mục quản lý được và công tắc bật/tắt kiểm duyệt | Accepted · một phần superseded by ADR-0078 | — |
| [0031](ADR-0031-game-chi-danh-cho-hoc-sinh-luu-lich-su-dang-nhap-mot-thiet-b.md) | Game chỉ dành cho học sinh, lưu lịch sử đăng nhập, một thiết bị cho mỗi tài khoản | Accepted | — |
| [0032](ADR-0032-worker-don-tai-khoan-hoc-sinh-khong-dang-nhap-game.md) | Worker dọn tài khoản học sinh không đăng nhập game | Accepted · một phần superseded by ADR-0085 | — |
| [0033](ADR-0033-purge-nhom-chat-gameplay-ngoai-le-co-kiem-soat-cua-adr-0028.md) | Purge nhóm chat gameplay (ngoại lệ có kiểm soát của ADR-0028) | Accepted | OQ-20 |
| [0034](ADR-0034-bo-tinh-nang-chat-day-du.md) | Bộ tính năng chat đầy đủ | Accepted · một phần superseded by ADR-0072 và ADR-0095 | — |
| [0035](ADR-0035-duong-truyen-chat-nats-jetstream-valkey-pub-sub-luu-postgres.md) | Đường truyền chat: NATS JetStream + Valkey Pub/Sub, lưu PostgreSQL | Accepted | — |
| [0036](ADR-0036-microservices-theo-he-thong-lon-thay-adr-0001.md) | Microservices theo hệ thống lớn | Superseded by ADR-0074 | — |
| [0037](ADR-0037-moi-service-mot-database-postgresql-thay-mot-phan-adr-0003.md) | Mỗi service một database PostgreSQL | Accepted · một phần superseded by ADR-0074 · supersedes phần DB/schema của ADR-0003 | OQ-10 |
| [0038](ADR-0038-cqrs-clean-architecture-trong-moi-service.md) | CQRS + Clean Architecture trong mỗi service | Accepted · một phần superseded by ADR-0087 | — |
| [0039](ADR-0039-orm-ent-cho-go-tuong-duong-entity-framework-migration-bang-a.md) | ORM ent cho Go (tương đương Entity Framework), migration bằng Atlas | Accepted · supersedes phần ORM/migration của ADR-0025 | — |
| [0040](ADR-0040-goi-service-to-service-bang-typed-rest-client-sinh-tu-openap.md) | Gọi service-to-service bằng typed REST client sinh từ OpenAPI (tương đương Refit) | Accepted · một phần superseded by ADR-0074 | — |
| [0041](ADR-0041-moi-cot-truong-kieu-chuoi-phai-co-gioi-han-do-dai.md) | Mọi cột/trường kiểu chuỗi phải có giới hạn độ dài | Accepted | — |
| [0042](ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) | Domain event qua Transactional Outbox → NATS JetStream | Accepted · supersedes transport của ADR-0019 | — |
| [0043](ADR-0043-phat-hanh-noi-dung-draft-publish-con-tro-phien-ban-giu-phien.md) | Phát hành nội dung: Draft/Publish, con trỏ phiên bản, giữ phiên bản khi đang chơi, rollback, đẩy cache | Accepted · supersedes cách rollback của ADR-0015 · một phần superseded by ADR-0072 và ADR-0095 | — |
| [0044](ADR-0044-luu-tru-hinh-anh-va-media-object-storage-s3-compatible-cdn.md) | Lưu trữ hình ảnh và media: object storage S3-compatible + CDN | Accepted | OQ-10, OQ-12, OQ-21 |
| [0045](ADR-0045-dang-nhap-nhanh-o-quy-mo-10-000-ccu.md) | Đăng nhập nhanh ở quy mô 10.000 CCU | Accepted · một phần superseded by ADR-0072 | OQ-03 |
| [0046](ADR-0046-nhan-biet-sis-nguon-cua-hoc-sinh-khi-dang-nhap.md) | Nhận biết SIS nguồn của học sinh khi đăng nhập | Accepted · một phần superseded by ADR-0095 | OQ-01 |
| [0047](ADR-0047-hai-he-thong-giao-dien-game-godot-3d-va-admin-ops-cms-lms.md) | Hai hệ thống giao diện: Game (Godot 3D) và Admin/Ops (Admin Portal) | Accepted | — |
| [0048](ADR-0048-learning-activity-registry-mo-rong-loai-hoat-dong-hoc-tap-kh.md) | Learning Activity Registry: mở rộng loại hoạt động học tập không sửa lõi | Accepted | — |
| [0049](ADR-0049-so-cai-phan-thuong-va-tien-te-trong-game-reward-currency-led.md) | Sổ cái phần thưởng và tiền tệ trong game (Reward & Currency Ledger) | Accepted | OQ-14 |
| [0050](ADR-0050-party-tam-thoi-va-guild-club-ben-vung-la-hai-khai-niem-tach.md) | Party (tạm thời) và Guild/Club (bền vững) là hai khái niệm tách biệt | Accepted · một phần superseded by ADR-0084 và ADR-0095 | — |
| [0051](ADR-0051-pham-vi-voice-chat.md) | Phạm vi voice chat | Accepted (qua 0070) | OQ-19 |
| [0052](ADR-0052-thuc-thi-mo-phong-simulation-execution.md) | Thực thi mô phỏng (Simulation Execution) | Accepted | — |
| [0053](ADR-0053-cham-diem-co-ho-tro-ai-ai-assisted-grading.md) | Chấm điểm có hỗ trợ AI (AI-assisted Grading) | Proposed · chưa thuộc Release 1.0 · hoãn (ADR-0082) | OQ-12 |
| [0054](ADR-0054-tuong-tac-xa-hoi-lien-truong-cross-school-social-interaction.md) | Tương tác xã hội liên trường (Cross-school Social Interaction) | Accepted | OQ-09 |
| [0055](ADR-0055-cong-thuc-toan-vat-ly-hoa-hoc-mathjax-4-hien-thi-mathlive-nh.md) | Công thức Toán, Vật lý, Hóa học: MathJax 4 (hiển thị) + MathLive (nhập trên Admin Portal) | Accepted | — |
| [0056](ADR-0056-game-song-ngu-viet-anh.md) | Game song ngữ Việt – Anh | Accepted | OQ-15 |
| [0057](ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md) | Chạy được trên máy tầm trung: hạng thiết bị và ngân sách hiệu năng | Accepted | OQ-04 |
| [0058](ADR-0058-game-analytics-tong-hop-trong-postgresql-qua-su-kien-nats.md) | Game Analytics: tổng hợp trong PostgreSQL qua sự kiện NATS | Accepted | — |
| [0059](ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md) | Hợp nhất API attempt; chưa làm AI giọng nói trong Release 1.0 | Accepted · một phần superseded by ADR-0072 | OQ-22 |
| [0060](ADR-0060-danh-muc-quoc-gia-tinh-thanh-truong-hoc-sinh-thuoc-mot-truong-mot-lop.md) | Danh mục quốc gia, tỉnh/thành, trường; học sinh thuộc một trường một lớp | Accepted · một phần superseded by ADR-0095 | — |
| [0061](ADR-0061-dong-bo-du-lieu-hoc-sinh-tu-sis-khi-dang-nhap.md) | Đồng bộ dữ liệu học sinh từ SIS khi đăng nhập | Accepted · một phần superseded by ADR-0072 và ADR-0095 | OQ-01 |
| [0062](ADR-0062-kien-truc-du-lieu-dashboard-khong-truy-van-du-lieu-tho.md) | Kiến trúc dữ liệu dashboard: chỉ đọc bảng tổng hợp | Accepted · một phần superseded by ADR-0072 và ADR-0095 | — |
| [0063](ADR-0063-dang-nhap-release-1-0-username-password-va-quy-tac-tenant.md) | Đăng nhập Release 1.0: username/password và quy tắc tenant | Accepted · một phần superseded by ADR-0094 · một phần superseded by ADR-0095 | — |
| [0064](ADR-0064-khong-tu-dang-nhap-khi-mo-lai-game.md) | Không tự đăng nhập khi mở lại game | Accepted · một phần superseded by ADR-0086 · một phần superseded by ADR-0094 | — |
| [0065](ADR-0065-tenant-id-tren-header-moi-request.md) | tenant_id trên header mọi request, luôn kiểm tra chéo | Accepted | — |
| [0066](ADR-0066-dang-nhap-rieng-cho-portal-va-phan-quyen-role-userrole.md) | Đăng nhập riêng cho portal; phân quyền Role/UserRole | Accepted · một phần superseded by ADR-0072 · một phần superseded by ADR-0091 · một phần superseded by ADR-0094 · một phần superseded by ADR-0096 | OQ-07 |
| [0067](ADR-0067-portal-khong-chon-tenant-va-dinh-danh-dang-nhap-duy-nhat.md) | Portal không chọn tenant; username/email duy nhất | Accepted · một phần superseded by ADR-0091 · một phần superseded by ADR-0094 | — |
| [0068](ADR-0068-dang-nhap-game-bang-form-trong-game-qua-backend.md) | Đăng nhập game bằng form trong game qua backend | Accepted · một phần superseded by ADR-0091 · một phần superseded by ADR-0094 | — |
| [0069](ADR-0069-username-email-duy-nhat-theo-tenant-va-dang-nhap-khong-chon-tenant.md) | Đăng nhập: SSO xác thực trước, kiểm tra tenant sau; username/email không trùng trong tenant | Accepted | — |
| [0070](ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md) | Release 1.0: không voice chat, không xóa vật lý theo luật, không purge analytics thô | Accepted · mục 2 có ngoại lệ DROP một lần theo ADR-0095 | OQ-19, OQ-20 |
| [0071](ADR-0071-object-storage-production-dung-floci.md) | Object storage staging/production dùng floci | Accepted (rủi ro được chấp nhận) · superseded by ADR-0075 | OQ-21 |
| [0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) | Bỏ vai trò giáo viên: học sinh tự học, bỏ hoạt động chấm tay, kiểm duyệt tập trung ở platform | Accepted · một phần superseded by ADR-0081 · một phần superseded by ADR-0084 · một phần superseded by ADR-0094 · một phần superseded by ADR-0095 | OQ-07 |
| [0073](ADR-0073-phat-hanh-theo-pilot-uu-tien-giao-duc.md) | Phát hành theo pilot ưu tiên giáo dục | Accepted | OQ-08 |
| [0074](ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) | Modular monolith có ranh giới cưỡng chế thay cho 6 microservice | Accepted · supersedes ADR-0036 | OQ-10 |
| [0075](ADR-0075-object-storage-production-khong-dung-floci.md) | Object storage staging/production không dùng floci | Accepted · mục 2, 4 superseded by ADR-0093 | OQ-10, OQ-21 |
| [0076](ADR-0076-dang-nhap-nhanh-cho-lop-1-3.md) | Đăng nhập nhanh cho lớp 1–3: thẻ QR + PIN/mật khẩu hình | Rejected | OQ-03 |
| [0077](ADR-0077-mo-hinh-consent-va-vai-tro-du-lieu.md) | Mô hình consent, vai trò xử lý dữ liệu, quy định game cho người dưới 18 tuổi | Proposed · hoãn (ADR-0082) | OQ-12, OQ-17 |
| [0078](ADR-0078-mac-dinh-an-toan-cho-tre-nho.md) | Mặc định an toàn cho trẻ nhỏ | Accepted | OQ-17 |
| [0079](ADR-0079-nen-tang-ban-dau-windows-va-android.md) | Nền tảng bản đầu: Windows + Android | Rejected | OQ-04 |
| [0080](ADR-0080-do-hieu-qua-hoc-tap-la-exit-criterion-pilot.md) | Đo hiệu quả học tập là exit criterion của pilot | Accepted · một phần superseded by ADR-0095 | — |
| [0081](ADR-0081-user-type-student-staff-phan-biet-bang-tenant.md) | `user_type` chỉ gồm `student` và `staff`; phân biệt nhân sự trường/platform bằng tenant | Accepted | — |
| [0082](ADR-0082-spec-driven-development-va-noi-dung-deferred.md) | Spec-Driven Development (Spec Kit + EARS); nội dung không làm vào thư mục deferred | Accepted | OQ-12, OQ-24 |
| [0083](ADR-0083-dang-nhap-game-va-tao-game-session-tach-biet.md) | Đăng nhập game và tạo game session là hai bước riêng | Accepted | — |
| [0084](ADR-0084-nhom-hoc-tap-thuoc-learning-service.md) | Nhóm học tập (study group) thuộc learning-service | Superseded by ADR-0095 | — |
| [0085](ADR-0085-don-hoc-sinh-giu-user-keycloak.md) | Dọn học sinh không đăng nhập: giữ user Keycloak, gỡ membership | Accepted | — |
| [0086](ADR-0086-giu-phien-game-bang-refresh-token.md) | Giữ phiên game bằng refresh token: còn hạn thì không phải đăng nhập lại | Accepted · một phần superseded by ADR-0094 | — |
| [0087](ADR-0087-cqrs-tach-read-write-khong-mediator.md) | CQRS tách rõ read/write context, không dùng mediator | Accepted | — |
| [0088](ADR-0088-ci-cd-bang-github-actions.md) | CI/CD bằng GitHub Actions | Accepted | — |
| [0089](ADR-0089-chot-tang-du-lieu-rls-mixin-outbox.md) | Chốt tầng dữ liệu: RLS chỉ lọc tenant, bộ mixin ent, outbox và partition hạ tầng | Accepted | — |
| [0090](ADR-0090-danh-muc-menu-action-portal-bat-tat-trong-db.md) | Danh mục menu/action của Admin Portal và bật/tắt lưu trong database | Accepted · một phần superseded by ADR-0094 | — |
| [0091](ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) | Admin Portal đăng nhập bằng form riêng qua backend, không chuyển hướng OIDC | Accepted · một phần supersedes ADR-0066, 0067, 0068 · một phần superseded by ADR-0094 · một phần superseded by ADR-0096 | — |
| [0092](ADR-0092-to-chuc-ma-nguon-trong-src.md) | Tổ chức mã nguồn trong src/: backend dùng chung, cms/frontend, game | Accepted · một phần supersedes ADR-0074 | — |
| [0093](ADR-0093-object-storage-tu-host-seaweedfs-truoc-chuyen-s3-managed-bang-cau-hinh.md) | Object storage tự host SeaweedFS trước, chuyển S3 managed bằng cấu hình | Accepted · một phần supersedes ADR-0075 | OQ-10, OQ-21 |
| [0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md) | Bỏ MFA (TOTP) cho đăng nhập Admin Portal | Accepted (rủi ro được chấp nhận) · một phần supersedes ADR-0091, 0009, 0022, 0027, 0029, 0063, 0064, 0066, 0067, 0068, 0072, 0086, 0090 · một phần superseded by ADR-0096 | — |
| [0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | Bỏ khái niệm lớp: học sinh thuộc trường theo khối (bỏ enrollment, course, learning path, lịch, nhóm học tập; DROP một lần) | Accepted · supersedes ADR-0084 · một phần supersedes ADR-0005, 0020, 0021, 0026, 0027, 0034, 0043, 0046, 0050, 0060, 0061, 0062, 0063, 0072, 0080 · ngoại lệ mục 2 ADR-0070 | — |
| [0096](ADR-0096-chan-hoc-sinh-o-portal-khong-tinh-la-dang-nhap-sai.md) | Chặn học sinh ở Admin Portal không được tính là đăng nhập sai | Accepted · một phần supersedes ADR-0066, 0091, 0094 | — |
| [0097](ADR-0097-bo-yeu-cau-du-lieu-ca-nhan-va-dich-noi-dung-hoc-tap-khoi-portal-release-1.md) | Bỏ "Yêu cầu dữ liệu cá nhân" và "Dịch nội dung học tập" khỏi Admin Portal Release 1 | Accepted (rủi ro được chấp nhận) | — |
| [0098](ADR-0098-tailwind-css-lam-lop-tien-ich-bo-cuc-cho-admin-portal.md) | Tailwind CSS làm lớp tiện ích bố cục cho Admin Portal (bổ sung ADR-0047) | Accepted · bổ sung ADR-0047 | — |
| [0099](ADR-0099-doi-ten-config-type-levels-thanh-progression-levels.md) | Đổi tên config type `levels` thành `progression-levels` | Accepted | — |

## Quyết định hiện hành theo chủ đề

Danh sách dưới đây chỉ gồm ADR **đang có hiệu lực** (Accepted). ADR bị thay toàn phần không được liệt kê; ADR bị thay một phần được ghi kèm phần thay thế.

### Kiến trúc & triển khai

| ADR | Quyết định hiện hành |
|---|---|
| [0087](ADR-0087-cqrs-tach-read-write-khong-mediator.md) | CQRS tách read/write context (command/query, port và persistence riêng), không mediator; cross-cutting qua middleware và gọi tường minh |
| [0088](ADR-0088-ci-cd-bang-github-actions.md) | CI/CD bằng GitHub Actions (`.github/workflows/`, ghcr.io, runner macOS cho iOS, GitHub Environments) |
| [0082](ADR-0082-spec-driven-development-va-noi-dung-deferred.md) | Spec-Driven Development chuẩn Spec Kit + EARS: `.specify/` (constitution, template), feature `specs/<NNN>-<slug>/`, danh mục trong CODE_PLAN; `docs/99-deferred`, `specs/_deferred` ngoài phạm vi |
| [0074](ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) | Modular monolith: module `identity`, `learning`, `game`, `competition`, `chat`, `realtime` trong `modules/`; 3 binary `core-api`, `realtime`, `worker` (profile `events`/`chat`/`jobs`); mỗi module một database trên một cụm PostgreSQL; gọi giữa module trong tiến trình qua facade sinh từ spec internal |
| [0092](ADR-0092-to-chuc-ma-nguon-trong-src.md) | Mã nguồn trong `src/`: `src/backend` (Go module duy nhất: cmd, modules, pkg), `src/cms/frontend` (Admin Portal), `src/game` (Godot); deploy/specs/docs/scripts/design ở gốc |
| [0038](ADR-0038-cqrs-clean-architecture-trong-moi-service.md) | CQRS + Clean Architecture trong mỗi service |
| [0040](ADR-0040-goi-service-to-service-bang-typed-rest-client-sinh-tu-openap.md) | Spec internal là hợp đồng facade có kiểu, `x-allowed-callers` (transport thay bởi ADR-0074) |
| [0039](ADR-0039-orm-ent-cho-go-tuong-duong-entity-framework-migration-bang-a.md) | ORM ent + migration Atlas (thay phần ORM của 0025) |
| [0025](ADR-0025-thu-vien-nen-tang-cho-go-backend.md) | Thư viện Go nền tảng còn hiệu lực (chi, oapi-codegen, slog, nats.go…) |
| [0002](ADR-0002-monorepo.md) | Monorepo (cấu trúc thư mục theo 0092) |
| [0009](ADR-0009-apache-apisix-lam-api-gateway-backend-van-tu-authorize.md) | APISIX làm gateway; backend vẫn tự authorize; route platform chỉ nội bộ |
| [0010](ADR-0010-rest-openapi-3-1-contract-first-problem-details-cursor-pagin.md) | REST + OpenAPI 3.1 contract-first, Problem Details, cursor, Idempotency-Key |
| [0011](ADR-0011-realtime-transport-websocket-qua-go-realtime-server.md) | WebSocket qua `realtime-gateway` cho mọi realtime |
| [0042](ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) | Domain event: outbox → NATS JetStream (thay transport của 0019) |
| [0019](ADR-0019-transactional-outbox-valkey-streams-cho-domain-events.md) | Transactional outbox + idempotent consumer (phần còn hiệu lực) |
| [0024](ADR-0024-chay-tren-docker-compose-cho-local-docker-swarm-cho-staging.md) | Docker: Compose local, Swarm staging/production, chưa dùng Kubernetes |
| [0023](ADR-0023-observability-bang-opentelemetry-prometheus-grafana-loki-tem.md) | Observability: OpenTelemetry + Prometheus + Grafana + Loki + Tempo |

### Dữ liệu & tenancy

| ADR | Quyết định hiện hành |
|---|---|
| [0037](ADR-0037-moi-service-mot-database-postgresql-thay-mot-phan-adr-0003.md) | Mỗi module một database PostgreSQL, mặc định một cụm (thay phần DB của 0003; một phần thay bởi 0074) |
| [0003](ADR-0003-postgresql-la-nguon-du-lieu-ben-vung-duy-nhat-schema-theo-mo.md) | PostgreSQL là system of record, UUIDv7, partition bảng lớn (phần còn hiệu lực) |
| [0004](ADR-0004-valkey-chi-cho-trang-thai-tam-thoi-cache-va-phoi-hop-realtim.md) | Valkey chỉ cho dữ liệu tạm, cache, phối hợp realtime |
| [0005](ADR-0005-mo-hinh-multi-tenancy-shared-database-tenant-id-rls.md) | Multi-tenancy: `tenant_id` + RLS; tenant = trường / đơn vị / chung |
| [0028](ADR-0028-chi-soft-delete-khong-hard-delete-mac-dinh-khong-doc-du-lieu.md) | Chỉ soft-delete; mặc định không đọc dữ liệu đã xóa; Release 1.0 không xóa vật lý (0070) |
| [0029](ADR-0029-system-admin-gan-du-lieu-cho-tenant-va-clone-du-lieu-giua-te.md) | System Admin gán và clone dữ liệu giữa tenant |
| [0041](ADR-0041-moi-cot-truong-kieu-chuoi-phai-co-gioi-han-do-dai.md) | Mọi trường chuỗi có giới hạn độ dài |
| [0044](ADR-0044-luu-tru-hinh-anh-va-media-object-storage-s3-compatible-cdn.md) | Media trên object storage S3-compatible + CDN (mục 1 staging/production thay bởi 0075) |
| [0075](ADR-0075-object-storage-production-khong-dung-floci.md) + [0093](ADR-0093-object-storage-tu-host-seaweedfs-truoc-chuyen-s3-managed-bang-cau-hinh.md) | Local/CI dùng floci; staging/production dùng SeaweedFS tự host (3 node, replication 3 bản), chuyển S3 managed tại VN bằng cấu hình khi có ngân sách; bucket dữ liệu học sinh có RPO như dữ liệu học tập |
| [0089](ADR-0089-chot-tang-du-lieu-rls-mixin-outbox.md) | RLS chỉ lọc tenant; mixin ent theo loại bảng; scope Tenant/CrossTenant/Platform + `tenancy.Bypass` reason đóng; outbox không RLS; gỡ partition chỉ cho `infra.*` |
| [0070](ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md) | Release 1.0 không xóa vật lý (chỉ ẩn danh hóa, ngoại lệ runtime 0033); không purge dữ liệu analytics thô; ngoại lệ cấu trúc một lần: DROP bảng/cột khái niệm lớp trước lần triển khai staging/production đầu tiên (0095 mục 8) |
| [0065](ADR-0065-tenant-id-tren-header-moi-request.md) | Header `X-Tenant-Id` bắt buộc trên request tenant-scoped, luôn kiểm tra chéo với token + membership |
| [0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | Không có lớp/enrollment: học sinh thuộc đúng một trường với một khối (`org.students.school_id`, `grade_level`); membership tenant suy ra từ học sinh hiệu lực; khối tại thời điểm sự kiện ghi trên dữ liệu học tập; DROP bảng/cột khái niệm lớp kèm backup R12 |
| [0060](ADR-0060-danh-muc-quoc-gia-tinh-thanh-truong-hoc-sinh-thuoc-mot-truong-mot-lop.md) | Danh mục quốc gia / tỉnh thành (34 tỉnh VN) / trường; học sinh thuộc đúng một trường (mục 3 "một lớp" thay bởi 0095) |
| [0062](ADR-0062-kien-truc-du-lieu-dashboard-khong-truy-van-du-lieu-tho.md) | Dashboard chỉ đọc bảng rollup trên read replica; giới hạn khoảng thời gian, `statement_timeout`, cache, xuất bất đồng bộ |

### Identity & SIS

| ADR | Quyết định hiện hành |
|---|---|
| [0096](ADR-0096-chan-hoc-sinh-o-portal-khong-tinh-la-dang-nhap-sai.md) | Flow Keycloak của portal chỉ còn username → mật khẩu; module `identity` từ chối tài khoản không phải nhân sự **sau** khi Keycloak cấp token (`403 PORTAL_ACCESS_STAFF_ONLY`, thu hồi phiên) để học sinh vào nhầm portal không bị brute-force khóa khỏi game (Proposed, đã triển khai) |
| [0094](ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md) | **Admin Portal không còn MFA/TOTP** cho mọi tài khoản, kể cả role platform (owner chấp nhận rủi ro): đăng nhập chỉ username/email + mật khẩu; route platform giữ gateway nội bộ + `azp = eduworld-portal` + role platform; phiên `platform_staff` tối đa 8 giờ từ lúc nhập mật khẩu; kiểm soát bù trừ ghi trong ADR |
| [0086](ADR-0086-giu-phien-game-bang-refresh-token.md) | Refresh token lưu trong kho bảo mật thiết bị cá nhân (xoay vòng, gắn thiết bị); còn hạn thì không đăng nhập lại; máy dùng chung chỉ giữ trong RAM |
| [0085](ADR-0085-don-hoc-sinh-giu-user-keycloak.md) | Dọn học sinh: giữ user Keycloak enabled, gỡ membership, thu hồi phiên; tự khôi phục khi đăng nhập lại |
| [0083](ADR-0083-dang-nhap-game-va-tao-game-session-tach-biet.md) | `/auth/game/login` chỉ xác thực, không tạo game session; client gọi `POST /game/sessions` (409 takeover ở đây) |
| [0081](ADR-0081-user-type-student-staff-phan-biet-bang-tenant.md) | `user_type` = `student` hoặc `staff`; nhân sự platform có `primary_tenant_id = NULL` |
| [0006](ADR-0006-keycloak-mot-realm-dung-organizations-identity-brokering.md) | Keycloak một realm, IdP brokering (ánh xạ Organization theo 0026, client theo 0027) |
| [0063](ADR-0063-dang-nhap-release-1-0-username-password-va-quy-tac-tenant.md) | Đăng nhập Release 1.0 bằng username/password, không chọn trường; 1 tenant → vào thẳng, nhiều tenant → chọn sau đăng nhập, chỉ tenant chung → tenant chung, không thuộc tenant nào → từ chối |
| [0069](ADR-0069-username-email-duy-nhat-theo-tenant-va-dang-nhap-khong-chon-tenant.md) | **SSO xác thực trước, identity-service kiểm tra tenant sau**; một người một tài khoản SSO (username/email duy nhất trên Keycloak); username/email không trùng trong cùng `tenant_id` |
| [0068](ADR-0068-dang-nhap-game-bang-form-trong-game-qua-backend.md) | Game đăng nhập bằng form trong game, qua `POST /auth/game/login` của identity-service (client confidential); thay 0007 cho game |
| [0064](ADR-0064-khong-tu-dang-nhap-khi-mo-lai-game.md) | Không tự đăng nhập khi mở lại game — đã thay một phần bởi ADR-0086 (giữ phiên bằng refresh token) |
| [0066](ADR-0066-dang-nhap-rieng-cho-portal-va-phan-quyen-role-userrole.md) | Admin Portal có trang đăng nhập riêng, học sinh không vào được; phân quyền Permission → Role → UserRole |
| [0067](ADR-0067-portal-khong-chon-tenant-va-dinh-danh-dang-nhap-duy-nhat.md) | Portal đăng nhập không chọn tenant (mục 2–3 được làm rõ bởi 0069) |
| [0026](ADR-0026-dang-nhap-theo-tenant-xac-dinh-tenant-kiem-tra-cheo-mac-dinh.md) | Kiểm tra chéo tenant của phiên (mục 3–4 thay bởi 0063) |
| [0008](ADR-0008-access-token-cua-keycloak-la-credential-api-realtime-dung-ti.md) | Access token Keycloak cho REST; realtime dùng ticket ngắn hạn |
| [0045](ADR-0045-dang-nhap-nhanh-o-quy-mo-10-000-ccu.md) | Năng lực đăng nhập 10.000 CCU, hàng chờ, warm-up (mục 3 thay bởi 0064, mục 4 thay bởi 0063) |
| [0021](ADR-0021-sis-integration-theo-contract-do-eduworld-so-huu.md) | SIS integration theo contract do EduWorld sở hữu (contract 1.3.0 không còn Class/Enrollment — 0095) |
| [0046](ADR-0046-nhan-biet-sis-nguon-cua-hoc-sinh-khi-dang-nhap.md) | Nhận biết SIS nguồn của học sinh khi đăng nhập (nhánh IdP thay bởi 0063) |
| [0061](ADR-0061-dong-bo-du-lieu-hoc-sinh-tu-sis-khi-dang-nhap.md) | Đồng bộ dữ liệu học sinh từ SIS khi đăng nhập: chặn khi thiếu dữ liệu, chạy nền khi dữ liệu cũ hơn 24 giờ |
| [0031](ADR-0031-game-chi-danh-cho-hoc-sinh-luu-lich-su-dang-nhap-mot-thiet-b.md) | Game chỉ cho học sinh; lịch sử đăng nhập; một thiết bị mỗi tài khoản |
| [0032](ADR-0032-worker-don-tai-khoan-hoc-sinh-khong-dang-nhap-game.md) | Worker dọn học sinh không đăng nhập game |

### Game & nội dung

| ADR | Quyết định hiện hành |
|---|---|
| [0012](ADR-0012-the-gioi-game-hybrid-hub-instance-private-instance-match-roo.md) | Thế giới hybrid: hub instance + private instance + match room |
| [0013](ADR-0013-nen-tang-client-release-1-0-gom-windows-android-ios-web.md) | Release 1.0 trên Windows, Android, iOS, Web (ngân sách hiệu năng thay bởi 0057) |
| [0014](ADR-0014-godot-4-x-voi-gdscript-co-static-typing.md) | Godot 4.x + GDScript static typing |
| [0016](ADR-0016-server-authoritative-scoring-answer-key-khong-bao-gio-roi-se.md) | Server-authoritative scoring; answer key không rời server |
| [0018](ADR-0018-glicko-2-cho-game-rating-tach-biet-learning-mastery.md) | Glicko-2 cho Game Rating; 5 loại số liệu tách biệt |
| [0049](ADR-0049-so-cai-phan-thuong-va-tien-te-trong-game-reward-currency-led.md) | Sổ cái phần thưởng và tiền tệ "Xu" |
| [0050](ADR-0050-party-tam-thoi-va-guild-club-ben-vung-la-hai-khai-niem-tach.md) | Party tạm thời tách biệt Guild/Club bền vững |
| [0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | Party lớp 1–3 chỉ `friend`/`quest`, cùng đúng khối, khác trường được trong cùng tenant (theo 0054); không party liên tenant; party `study_group` → `study_party`; mục tiêu tập thể của khối thay bảng/mục tiêu lớp; thử thách/giải khối thay cấp lớp |
| [0054](ADR-0054-tuong-tac-xa-hoi-lien-truong-cross-school-social-interaction.md) | Quy tắc tương tác xã hội liên trường |
| [0015](ADR-0015-content-versioning-bat-bien-sau-publish.md) | Version nội dung bất biến sau publish (phần còn hiệu lực) |
| [0043](ADR-0043-phat-hanh-noi-dung-draft-publish-con-tro-phien-ban-giu-phien.md) | Draft/Publish, release pointer, giữ phiên bản khi đang chơi, rollback, cache |
| [0099](ADR-0099-doi-ten-config-type-levels-thanh-progression-levels.md) | Config type bảng cấp độ XP là `progression-levels` (không còn `levels`); gói màn chơi là `level-packages` |

### Học tập

| ADR | Quyết định hiện hành |
|---|---|
| [0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) | Chỉ hoạt động server tự chấm trong Release; `written_response`, `speaking`, `project`, rubric, hàng đợi chấm ngoài Release |
| [0048](ADR-0048-learning-activity-registry-mo-rong-loai-hoat-dong-hoc-tap-kh.md) | Learning Activity Registry |
| [0017](ADR-0017-mastery-model-co-the-cau-hinh-mac-dinh-weighted-evidence.md) | Mastery model cấu hình được (weighted evidence) |
| [0052](ADR-0052-thuc-thi-mo-phong-simulation-execution.md) | Mô phỏng chạy trên Godot, server kiểm chứng |
| [0053](ADR-0053-cham-diem-co-ho-tro-ai-ai-assisted-grading.md) | AI chỉ gợi ý chấm, giáo viên duyệt (chưa thuộc Release 1.0) |
| [0055](ADR-0055-cong-thuc-toan-vat-ly-hoa-hoc-mathjax-4-hien-thi-mathlive-nh.md) | Công thức: MathJax 4 + MathLive; game chỉ hiển thị |
| [0056](ADR-0056-game-song-ngu-viet-anh.md) | Game song ngữ Việt – Anh; bộ tên thế giới đã duyệt |
| [0059](ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md) | Một họ API attempt; không AI giọng nói trong 1.0 |

### Chat & an toàn

| ADR | Quyết định hiện hành |
|---|---|
| [0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) | Kiểm duyệt tập trung ở Platform Safety Team (`safety_moderator`); nhà trường chỉ nhận thông báo vụ việc để liên hệ phụ huynh |
| [0020](ADR-0020-chat-thong-nhat-cho-moi-lop-moi-tin-nhan-phai-qua-kiem-tra-h.md) | Chat như nhau mọi khối; mọi tin qua kiểm tra hợp lệ; chat gameplay luôn có; 6 loại channel, không có kênh lớp/khối, policy platform → tenant → school (0095) |
| [0030](ADR-0030-chinh-sach-tu-nhay-cam-trong-chat-danh-muc-quan-ly-duoc-va-c.md) | Danh mục từ nhạy cảm + công tắc kiểm duyệt (bước PII/grooming luôn chạy, tắt bước khác có hạn và hai người duyệt — 0078) |
| [0078](ADR-0078-mac-dinh-an-toan-cho-tre-nho.md) | Mặc định an toàn: PII/grooming luôn chạy, giờ giới nghiêm platform, siết tenant `common`, preset quick message, Windows mặc định máy dùng chung + màn "Tiếp tục là…" |
| [0034](ADR-0034-bo-tinh-nang-chat-day-du.md) | Bộ tính năng chat đầy đủ |
| [0035](ADR-0035-duong-truyen-chat-nats-jetstream-valkey-pub-sub-luu-postgres.md) | Chat qua NATS JetStream + Valkey Pub/Sub, lưu PostgreSQL |
| [0033](ADR-0033-purge-nhom-chat-gameplay-ngoai-le-co-kiem-soat-cua-adr-0028.md) | Purge chat gameplay (ngoại lệ duy nhất của 0028) |
| [0051](ADR-0051-pham-vi-voice-chat.md) | Không có voice chat trong Release 1.0 (xác nhận bởi [0070](ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md)) |

### Giao diện & Admin Portal

| ADR | Quyết định hiện hành |
|---|---|
| [0097](ADR-0097-bo-yeu-cau-du-lieu-ca-nhan-va-dich-noi-dung-hoc-tap-khoi-portal-release-1.md) | Release 1 không có menu Portal "Yêu cầu dữ liệu" (F016) và "Bản dịch" nội dung học tập (F025): gỡ 6 operation; yêu cầu của chủ thể dữ liệu xử lý thủ công theo DATA_PRIVACY §3.1 (rủi ro chấp nhận, RISK-57) |
| [0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | LMS không còn lớp, course, learning path, lịch, nhóm học tập; giao bài/đề theo (trường, khối) ± danh sách học sinh; School Learning Dashboard trường → khối → học sinh; menu "Học sinh theo khối" |
| [0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) | Không có vai trò giáo viên; học sinh tự học, nhà trường (`school_admin`/`academic_manager`) giao bài và xem School Learning Dashboard; role platform `safety_moderator` |
| [0027](ADR-0027-cms-tach-vung-tenant-chi-thao-tac-learning-management-cau-hi.md) | Một Admin Portal 5 workspace; tenant chỉ Learning Management; System Admin liên tenant |
| [0022](ADR-0022-cms-lms-web-la-react-typescript-spa.md) | Portal là React SPA |
| [0047](ADR-0047-hai-he-thong-giao-dien-game-godot-3d-va-admin-ops-cms-lms.md) | Hai design system: Game (Godot 3D) và Admin/Ops (Ant Design 5) |
| [0098](ADR-0098-tailwind-css-lam-lop-tien-ich-bo-cuc-cho-admin-portal.md) | Portal dùng thêm Tailwind CSS v3 (ghim phiên bản) chỉ cho bố cục/tiện ích: Preflight tắt, theme chỉ trỏ biến admin token `var(--ew-*)`, cấm arbitrary value (ESLint/stylelint/test); Ant Design vẫn là thư viện component |
| [0066](ADR-0066-dang-nhap-rieng-cho-portal-va-phan-quyen-role-userrole.md) | Đăng nhập portal riêng (màn đăng nhập theo 0091); màn quản trị Role / UserRole |
| [0091](ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) | Portal đăng nhập bằng form riêng: `identity` gọi Keycloak Direct Access Grant (client `eduworld-portal` confidential), username/email + mật khẩu (không OTP, ADR-0094); refresh token trong cookie HttpOnly; tenant qua `X-Tenant-Id` |
| [0090](ADR-0090-danh-muc-menu-action-portal-bat-tat-trong-db.md) | Danh mục menu/action portal trong `specs/portal/menu-catalog.yaml` + bản sao DB; System Admin bật/tắt qua `platform.feature_flags` (platform/tenant); backend chặn bằng `featuregate` → `403 FEATURE_DISABLED` |

### Vận hành

| ADR | Quyết định hiện hành |
|---|---|
| [0024](ADR-0024-chay-tren-docker-compose-cho-local-docker-swarm-cho-staging.md) | Docker Swarm, scale theo lịch học |
| [0023](ADR-0023-observability-bang-opentelemetry-prometheus-grafana-loki-tem.md) | Observability |
| [0045](ADR-0045-dang-nhap-nhanh-o-quy-mo-10-000-ccu.md) | Năng lực đăng nhập, warm-up theo lịch |
| [0032](ADR-0032-worker-don-tai-khoan-hoc-sinh-khong-dang-nhap-game.md) | Job dọn tài khoản |
| [0033](ADR-0033-purge-nhom-chat-gameplay-ngoai-le-co-kiem-soat-cua-adr-0028.md) | Job purge chat gameplay |
| [0057](ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md) | Hạng thiết bị tầm trung, preset chất lượng, ngân sách hiệu năng |
| [0058](ADR-0058-game-analytics-tong-hop-trong-postgresql-qua-su-kien-nats.md) | Game Analytics trong PostgreSQL qua NATS (raw không purge, 0070) |
| [0073](ADR-0073-phat-hanh-theo-pilot-uu-tien-giao-duc.md) | Phát hành theo pilot ưu tiên giáo dục rồi mở rộng theo giai đoạn, đủ 4 nền tảng (0013) |
| [0080](ADR-0080-do-hieu-qua-hoc-tap-la-exit-criterion-pilot.md) | Báo cáo hiệu quả học tập (kiểm tra độc lập trước/sau, cụm (trường, khối) so sánh — 0095) là exit criterion của pilot; hạ tầng A/B mô hình mastery và luật gợi ý (mục 5) |
| [0062](ADR-0062-kien-truc-du-lieu-dashboard-khong-truy-van-du-lieu-tho.md) | Job chốt rollup dashboard; không truy vấn dữ liệu thô cho dashboard |

### Đề xuất hoãn hoặc bị từ chối

| ADR | Đề xuất | Trạng thái |
|---|---|---|
| [0077](ADR-0077-mo-hinh-consent-va-vai-tro-du-lieu.md) | Mô hình consent và vai trò dữ liệu | Hoãn (ADR-0082): pháp lý không có trong version này |
| [0076](ADR-0076-dang-nhap-nhanh-cho-lop-1-3.md) | Thẻ QR + PIN/mật khẩu hình cho lớp 1–3 | Rejected — giữ username/password (0063) |
| [0079](ADR-0079-nen-tang-ban-dau-windows-va-android.md) | Bản đầu chỉ Windows + Android | Rejected — giữ đủ 4 nền tảng (0013) |

### Không còn hiệu lực

| ADR | Thay bởi |
|---|---|
| [0001](ADR-0001-go-modular-monolith-voi-3-tien-trinh-trien-khai.md) | [0036](ADR-0036-microservices-theo-he-thong-lon-thay-adr-0001.md) — modular monolith → microservices (về sau [0074](ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) quay lại modular monolith có ranh giới cưỡng chế) |
| [0036](ADR-0036-microservices-theo-he-thong-lon-thay-adr-0001.md) | [0074](ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) — 6 microservice → modular monolith 3 binary |
| [0007](ADR-0007-godot-dang-nhap-bang-oidc-authorization-code-pkce-qua-system.md) | [0068](ADR-0068-dang-nhap-game-bang-form-trong-game-qua-backend.md) (game: form trong game qua backend) và [0091](ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) (portal: form riêng qua backend) — bỏ luồng Authorization Code + PKCE qua trình duyệt |
| [0071](ADR-0071-object-storage-production-dung-floci.md) | [0075](ADR-0075-object-storage-production-khong-dung-floci.md) — floci production → S3 managed/Garage/SeaweedFS |
| [0084](ADR-0084-nhom-hoc-tap-thuoc-learning-service.md) | [0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) — bỏ nhóm học tập cùng khái niệm lớp |

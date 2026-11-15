# AGENTS.md — Quy tắc cho AI Coding Agent và developer

File này là hợp đồng làm việc cho mọi AI agent (và developer) khi sửa repository EduWorld. Phải đọc file này trước khi thực hiện bất kỳ thay đổi nào.

## 1. Trước khi bắt đầu

1. Đọc [README.md](README.md), file này, và [docs/README.md](docs/README.md) (quy ước tài liệu, mã định danh).
2. Đọc [chỉ mục ADR](docs/adr/README.md) (mục "Quyết định hiện hành theo chủ đề"). **Không** làm trái ADR đã có. Muốn thay đổi thì tạo ADR mới, giải thích lý do và đánh dấu ADR cũ là Superseded.
3. Đọc tài liệu của domain sắp sửa (xem bảng §6) và contract liên quan trong [specs/](specs/README.md).
4. Lấy giá trị mặc định từ [CONFIG_DEFAULTS](docs/00-overview/CONFIG_DEFAULTS.md) (trỏ về tài liệu sở hữu), không tự đặt con số.
5. Kiểm tra [OPEN_QUESTIONS.md](docs/12-project/OPEN_QUESTIONS.md). Nếu công việc phụ thuộc một OQ chưa trả lời, dùng mặc định `[Proposed]` và ghi rõ điều đó trong báo cáo.

## 2. Quy tắc bắt buộc (không ngoại lệ)

| # | Quy tắc |
|---|---|
| R1 | Không đặt business logic trong UI (Godot scene script, React component). Logic nằm trong service/domain. |
| R2 | Không hardcode nội dung học tập, quest, câu hỏi, đáp án trong client hoặc scene. |
| R3 | Không hardcode credential, realm, client id, URL môi trường. Mọi cấu hình đi qua env/config. |
| R4 | Không commit secret. File `.env` thật nằm trong `.gitignore`, chỉ commit `.env.example`. |
| R5 | Không tin bất kỳ điểm số, kết quả, thời gian, phần thưởng nào do client gửi lên (ADR-0016). |
| R6 | Không gửi answer key/scoring rule xuống client. |
| R7 | Mọi endpoint (trừ health) phải có authentication **và** authorization nghiệp vụ trong Go. APISIX không thay thế authorization. |
| R8 | Mọi truy vấn dữ liệu nghiệp vụ phải lọc theo `tenant_id` qua `TenantScope` (ADR-0005); tenant là trường độc lập, đơn vị nhiều trường hoặc tenant chung. Mỗi phiên chỉ một tenant; chỉ `system_admin` dùng `CrossTenantScope` (dropdown tenant, audit). Không có query "lấy tất cả" cho dữ liệu học sinh/chat. Không suy ra tenant từ username/email/header. |
| R9 | Không lưu dữ liệu quan trọng chỉ trong Valkey (ADR-0004). |
| R10 | Thao tác quan trọng (submit, nộp thi, kết quả trận, trao thưởng, lời mời) phải idempotent (`Idempotency-Key`) và có audit log. |
| R11 | Không sửa nội dung `Published`. Tạo version mới (ADR-0015). Attempt luôn tham chiếu `content_version_id`. |
| R12 | Không xóa migration đã chạy, không viết migration xóa dữ liệu mà chưa có ADR và kế hoạch backup. Dùng expand/contract. |
| R13 | **Clean Architecture + ranh giới service (ADR-0036, ADR-0038).** Trong service: `domain` không import gì ngoài thư viện chuẩn và `pkg/` thuần; `application` chỉ import `domain` và port; `infrastructure`/`presentation` hiện thực port. Giữa service: **không** import code của service khác (`services/<a>` không import `services/<b>`), chỉ dùng chung `pkg/`; **không** đọc/ghi database của service khác; giao tiếp chỉ qua API internal (R29) hoặc event NATS (ADR-0042). Linter (`depguard`/`go-arch-lint`) chặn vi phạm. |
| R14 | Không log PII, token, nội dung chat thô, đáp án học sinh ở mức INFO. |
| R15 | Chat dùng **một bộ quy tắc cho mọi lớp** (ADR-0020): không thêm điều kiện theo `grade_level`/`grade_band` vào chat policy hay code chat; **mọi tin nhắn** phải đi qua pipeline kiểm tra hợp lệ phía server trước khi phát, không có đường gửi nào bỏ qua pipeline; pipeline chỉ bỏ bước khi công tắc kiểm duyệt hiệu lực (do System Admin đặt, ADR-0030) đang tắt bước đó. Không mở channel mới hoặc nới policy chat mà không cập nhật [CHILD_SAFETY](docs/10-security/CHILD_SAFETY.md) và [CHAT_SYSTEM](docs/03-social-multiplayer/CHAT_SYSTEM.md). Không hardcode từ nhạy cảm trong code/test/seed (dùng danh mục CMS, fixture chỉ placeholder); không trả từ đã khớp, vị trí khớp hay nội dung danh mục về client; công tắc kiểm duyệt (ADR-0030) chỉ đọc từ cấu hình hiệu lực, không có cờ tắt nào trong code hay biến môi trường. Chat `match`/`team` theo trận/`party` là **always-on** (ADR-0020 mục 6): không thêm cờ/policy/feature flag nào tắt được chúng ở cấp tenant/trường/lớp; xóa vật lý chat chỉ qua job `gameplay-chat-purge` theo ADR-0033, không qua API ứng dụng. |
| R16 | Không hiển thị tên thật, trường, lớp của học sinh cho người chơi khác ngoài phạm vi policy cho phép. |
| R17 | Không tạo mock/stub rồi tuyên bố production-ready. Mock phải gắn nhãn rõ ràng trong code và báo cáo. |
| R18 | Không bỏ qua error handling, logging, tracing, test. |
| R19 | Không tạo cơ chế pay-to-win, loot box trả tiền, dark pattern hay áp lực gây nghiện cho trẻ. |
| R20 | Không cấp permission `game.*`, `competition.config.*`, `platform.*` cho role tenant (catalog permission trong `pkg/authz/catalog.go`, mọi service kiểm tra lúc khởi động). Mọi endpoint cấu hình game/thi đấu nằm dưới `/api/v1/game-admin/*` (chỉ gateway nội bộ, `azp = eduworld-portal` + MFA `acr` + role platform). Admin Portal là **một ứng dụng** 5 workspace; mã workspace platform tách chunk lazy-load nhưng **không** được coi là lớp bảo mật: mọi chặn quyền phải nằm ở backend (ADR-0027 mục 3, 7). Chỉ `system_admin` được đọc liên tenant (`tenant_ids`), và ghi của System Admin luôn có một `tenant_id` đích. |
| R21 | **Chỉ soft-delete (ADR-0028).** Không viết `DELETE FROM`/`TRUNCATE` trong code ứng dụng hoặc SQL tường minh; ent hook chuyển delete thành soft-delete (ADR-0039); "xóa" = set `deleted_at`/`deleted_by` qua helper `SoftDelete` + audit + event `<entity>.deleted`. Mọi query đọc phải có `deleted_at IS NULL` (trừ bảng append-only và module Thùng rác). Unique dùng partial index `WHERE deleted_at IS NULL`. Nghĩa vụ xóa theo luật = ẩn danh hóa; Release 1.0 không có xóa vật lý hay gỡ partition (ADR-0070). **Ngoại lệ duy nhất (ADR-0033):** purge chat gameplay (`chat.gameplay_messages`, channel `match`/`party`/`team` theo trận, `chat.ephemeral_messages`) trong `services/chat-service/internal/infrastructure/purge`, chạy bằng job của `chat-worker` với role DB `chat_purger`; không API/use case nào khác được xóa vật lý. |
| R22 | Gán/clone dữ liệu giữa tenant chỉ System Admin làm (ADR-0029); clone không bao giờ sao chép dữ liệu học sinh (học sinh, enrollment, attempt, kết quả, chat, mastery, XP). |
| R23 | **Game chỉ dành cho học sinh (ADR-0031).** Mọi route người chơi (`/game/*` trừ `client-config`, `/multiplayer/*`, `/competition/*` phía người chơi, realtime ticket) phải qua middleware `game_player` (`azp = eduworld-game` + `user_type = student`, nếu không `GAME_ACCESS_STUDENT_ONLY`). Không tạo cửa sau cho giáo viên/vận hành vào game; preview dùng dữ liệu mẫu, không tạo game session. Không bỏ quy tắc một thiết bị (unique game session active) và không lưu IP thô trong login events. |
| R24 | **Dọn học sinh không đăng nhập (ADR-0032) là soft-delete**, không hard-delete; số ngày lấy từ `platform.platform_settings`/tenant settings, không hardcode/env; luôn có báo trước, loại trừ, ngưỡng xác nhận và khôi phục khi đăng nhập lại. |
| R25 | **Chat qua NATS JetStream (ADR-0035):** không phát (fan-out) tin chat nào chưa được JetStream ack; không ghi tin chat trực tiếp vào PostgreSQL từ realtime (chỉ `chat-persister`); game client/CMS **không bao giờ** kết nối NATS hay Valkey. Tính năng chat mới (ADR-0034) phải: chạy lại kiểm duyệt khi tạo nội dung mới (sửa, forward), giữ bằng chứng khi thu hồi, không trả nội dung gốc đã che. |
| R26 | **CQRS (ADR-0038).** Thay đổi trạng thái chỉ qua **command handler**: nạp aggregate, gọi phương thức domain, lưu bằng ent trong transaction, ghi outbox cùng transaction, trả ID/version. **Query handler** chỉ đọc (read model/projection, có thể dùng replica/cache), không ghi DB, không phát event. Mọi handler đi qua mediator `pkg/mediator` để có logging, tracing, validation, authorization, idempotency. Không event sourcing. |
| R27 | **ent global filter (ADR-0039).** Không tạo client/query ent bỏ interceptor `tenant_id` + `deleted_at IS NULL`. Bỏ filter chỉ được làm trong API liên tenant của System Admin hoặc Thùng rác, qua hàm `pkg/tenancy.Bypass(ctx, reason)` có audit và metric. SQL tường minh ở Query side phải tự có điều kiện tenant + soft-delete và có test. Không auto-migrate khi khởi động; migration chỉ do Atlas sinh và được review. |
| R28 | **Mọi chuỗi có giới hạn độ dài (ADR-0041).** Mọi field string trong ent schema có `MaxLen(n)` → `varchar(n)`; không dùng `text` không giới hạn. Mọi `type: string` trong OpenAPI (public + internal), AsyncAPI, JSON Schema, event schema có `maxLength` (trừ `format: uuid/date/date-time`, enum, hoặc `pattern` giới hạn). Dùng bảng độ dài chuẩn trong ADR-0041. CI (Spectral, ajv, `atlas migrate lint`) chặn merge nếu thiếu. |
| R29 | **Gọi service khác chỉ qua typed client (ADR-0040).** Client sinh bằng oapi-codegen từ `specs/openapi/services/<callee>.internal.v1.yaml`; bọc trong adapter `infrastructure` sau một port của `application`. Bắt buộc dùng wrapper `pkg/svcclient` (timeout, retry chỉ cho thao tác idempotent, circuit breaker, bulkhead, token `svc-<caller>`, truyền `traceparent`/`X-Tenant-Id`/`Idempotency-Key`). Không gọi HTTP thủ công, không dùng token người dùng để gọi service khác. Endpoint internal mới phải khai báo `x-allowed-callers` và có contract test với mọi bên gọi. Phản ứng phụ (XP, quest, projection, mở channel chat) dùng event, không gọi đồng bộ. |
| R30 | **Đường đăng nhập phải nhẹ (ADR-0045, ADR-0046).** Không gọi SIS hay service khác đồng bộ trên `/auth/session` và `/game/sessions` (trừ on-demand lookup lần đầu qua IdP theo ADR-0046); dùng JWKS/Principal/tenant cache. Không đoán tenant hay SIS nguồn từ username/email. Không giới hạn đăng nhập theo IP đơn lẻ (NAT trường học). Không tự xác thực mật khẩu/PIN/QR ngoài Keycloak và `login-credentials:verify`. |
| R31 | **Phát hành nội dung (ADR-0043, ADR-0044).** Không sửa version đã publish; rollback chỉ đổi con trỏ `current_version_id` và ghi `content_releases`. Mọi lượt chơi/học phải ghim version khi bắt đầu và chỉ đọc theo version đã ghim. Không đưa câu hỏi đề, answer key hay media private lên CDN công khai. File media chỉ ở object storage (không lưu blob trong DB), khóa `media/<sha256>/…` bất biến. |
| R32 | **Hai design system (ADR-0047).** CMS/LMS/Admin dùng Admin/Ops UI (Ant Design 5 + Pro, theme sinh từ `design/admin-tokens/`); **không** mang phong cách, component hay Theme của game vào web. Game chỉ dùng Theme/token sinh từ `design/game-tokens/`; không dùng thư viện Admin UI trong Godot. Không hardcode màu/font/cỡ chữ/khoảng cách trong CSS/TSX hay `.tscn`; mọi giá trị lấy từ token. File sinh từ token (`eduworld_theme.tres`, `ui_tokens.gd`, `resources/art/generated/`, theme web) không sửa tay: sửa `design/` rồi `make design-tokens`. Mã màu nghiệp vụ (môn học, độ hiếm, trạng thái nội dung) chỉ khai báo ở `design/shared-tokens/`. |
| R33 | **Một nguồn sở hữu cho mỗi quy tắc/tham số.** Mỗi quy tắc, tham số mặc định, sơ đồ hay quyết định chỉ được định nghĩa ở **một** tài liệu sở hữu ([docs/README §3.3](docs/README.md)); tài liệu khác chỉ link. Tham số mặc định đăng ký ở [CONFIG_DEFAULTS](docs/00-overview/CONFIG_DEFAULTS.md); đổi giá trị phải sửa tài liệu sở hữu + CONFIG_DEFAULTS + code/config trong cùng MR. Khi viết code, đọc giá trị từ cấu hình (platform/tenant settings, cấu hình game có version), không hardcode. Di chuyển/gộp tài liệu bằng `scripts/docs/*.mjs` và chạy `make docs-check`. |

## 3. Quy trình cho mỗi feature

1. Đọc requirement (`FR-*`, `US-*`, `AC-*` trong [PRODUCT_REQUIREMENTS](docs/00-overview/PRODUCT_REQUIREMENTS.md)).
2. Thiết kế giải pháp. Nếu quyết định có ảnh hưởng kiến trúc thì viết ADR.
3. Xác định **service sở hữu** (bảng §6). Tính năng chạm nhiều service: thiết kế API internal/event trước, thứ tự triển khai service nhận → service gọi.
4. Cập nhật contract trước: `specs/openapi/services/<service>.v1.yaml` (public), `<service>.internal.v1.yaml` (internal), `specs/asyncapi`, `specs/schemas`, `specs/events`; chạy `make gen SVC=<service>`.
5. Ent schema (có `MaxLen`) → `make migrate-diff` (Atlas) → domain → application (command/query handler) → infrastructure (repository ent, typed client, publisher) → presentation (handler HTTP/consumer).
6. Godot client (service → state → UI).
7. CMS/LMS nếu liên quan.
8. Viết test: unit theo tầng, integration (database riêng của service, global filter), contract test với service liên quan; thêm test tenant isolation và authorization cho mọi endpoint mới (public và internal).
9. Chạy test, lint, format.
10. Cập nhật tài liệu và CHANGELOG.
11. Báo cáo theo mẫu §5.

## 4. Lệnh kiểm tra (áp dụng từ Phase 1)

| Phạm vi | Lệnh dự kiến |
|---|---|
| Go (theo service) | `make gen-check SVC=<svc>` · `make lint SVC=<svc>` (gofmt, go vet, golangci-lint, go-arch-lint tầng Clean Architecture) · `make test SVC=<svc>` · `make test-integration SVC=<svc>` (testcontainers) · `make migrate-lint SVC=<svc>` (Atlas) |
| Liên service | `make test-contract` (consumer-driven) · `make test-e2e` |
| Specs | `make specs-lint` (Spectral + rule `maxLength` bắt buộc, AsyncAPI validate, ajv JSON Schema/event schema) · `make specs-breaking` (oasdiff cho spec public và internal) |
| Godot | `make godot-test` (GdUnit4 headless) · `make godot-export-check` |
| Web | `make web-lint` (ESLint + stylelint chặn hardcode) · `make web-test` · `make storybook` · visual regression (Playwright) |
| Design system | `make design-tokens` · `make design-tokens-check` (file sinh khớp nguồn, tương phản WCAG AA) |
| Toàn bộ | `make check` |

Tên target chính thức được chốt trong [LOCAL_DEVELOPMENT](docs/11-operations/LOCAL_DEVELOPMENT.md).

## 5. Mẫu báo cáo sau mỗi lần làm việc

```markdown
### Đã làm
- ...
### File tạo/sửa
- path — mô tả
### Quyết định kiến trúc
- (ADR mới/thay đổi, hoặc "Không có")
### Test đã chạy & kết quả
- lệnh → kết quả (dán số liệu pass/fail)
### Vấn đề còn tồn tại / giả định
- ... (ghi rõ [Proposed]/[TBD], OQ liên quan)
### Việc tiếp theo
- ...
```

Không tuyên bố hoàn thành khi chưa có bằng chứng kiểm thử.

## 6. Domain → tài liệu → service

| Domain | Tài liệu sở hữu | Service (ADR-0036) · database | Thư mục Godot |
|---|---|---|---|
| Thiết kế game tổng, thế giới, cốt truyện | [GAME_DESIGN](docs/01-game-design/GAME_DESIGN.md), [WORLD_DESIGN](docs/01-game-design/WORLD_DESIGN.md) | `game-service` · `eduworld_game` | `scenes/world`, `scripts/world` |
| Nhân vật, avatar, emote | [CHARACTER_SYSTEM](docs/01-game-design/CHARACTER_SYSTEM.md) | `game-service` | `scenes/character`, `scripts/player` |
| XP, level, quest, challenge/boss, kinh tế, vật phẩm, sưu tầm, game session, Game Operations Console | [PROGRESSION_SYSTEM](docs/01-game-design/PROGRESSION_SYSTEM.md), [QUEST_SYSTEM](docs/01-game-design/QUEST_SYSTEM.md), [CHALLENGE_BOSS_SYSTEM](docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md), [INVENTORY_ECONOMY](docs/01-game-design/INVENTORY_ECONOMY.md) | `game-service` | `scenes/quest`, `scripts/inventory` |
| Thi đấu, ranking, leaderboard, tournament | [COMPETITION_SYSTEM](docs/01-game-design/COMPETITION_SYSTEM.md) | `competition-service` · `eduworld_competition` | `scenes/competition` |
| Party, room, matchmaking, multiplayer gameplay | [PARTY_SYSTEM](docs/03-social-multiplayer/PARTY_SYSTEM.md), [MATCHMAKING](docs/03-social-multiplayer/MATCHMAKING.md), [MULTIPLAYER_GAMEPLAY](docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | `competition-service` | `scripts/multiplayer` |
| Bạn bè, club/guild, chat, kiểm duyệt, thông báo, sticker/GIF | [SOCIAL_SYSTEM](docs/03-social-multiplayer/SOCIAL_SYSTEM.md), [CHAT_SYSTEM](docs/03-social-multiplayer/CHAT_SYSTEM.md), [MODERATION](docs/03-social-multiplayer/MODERATION.md) | `chat-service` · `eduworld_chat` | `scenes/chat`, `scenes/social` |
| Trải nghiệm học tập, curriculum, nội dung & phát hành, question bank | [LEARNING_EXPERIENCE](docs/02-learning/LEARNING_EXPERIENCE.md), [CURRICULUM_MODEL](docs/02-learning/CURRICULUM_MODEL.md), [CONTENT_MODEL](docs/02-learning/CONTENT_MODEL.md), [QUESTION_BANK](docs/02-learning/QUESTION_BANK.md) | `learning-service` · `eduworld_learning` | `scripts/learning` |
| Activity & attempt (quiz, tương tác, video, audio, tự luận, mô phỏng, mini-game), đề thi | [ACTIVITY_ENGINE](docs/02-learning/ACTIVITY_ENGINE.md) + tài liệu từng định dạng, [EXAM_ENGINE](docs/02-learning/EXAM_ENGINE.md), [MINIGAME_ENGINE](docs/08-game-client/MINIGAME_ENGINE.md) | `learning-service` | `scenes/learning`, `activity_renderers/` |
| Mastery, analytics, LMS, dashboard giáo viên | [LEARNING_ANALYTICS](docs/02-learning/LEARNING_ANALYTICS.md), [SCHOOL_LMS](docs/04-admin-portal/SCHOOL_LMS.md) | `learning-service` | — (web/) |
| Admin Portal (5 workspace) | [ADMIN_PORTAL](docs/04-admin-portal/ADMIN_PORTAL.md) | gọi API của các service | — (`web/apps/portal`) |
| Identity, SSO, tenant, phân quyền, audit, settings, saga, SIS, dọn học sinh | [KEYCLOAK_SSO](docs/09-identity/KEYCLOAK_SSO.md), [IDENTITY_MAPPING](docs/09-identity/IDENTITY_MAPPING.md), [ACCOUNT_PROVISIONING](docs/09-identity/ACCOUNT_PROVISIONING.md), [SIS_INTEGRATION](docs/09-identity/SIS_INTEGRATION.md), [MULTITENANCY](docs/06-architecture/MULTITENANCY.md) | `identity-service` · `eduworld_identity` | `scripts/authentication` |
| Realtime (WebSocket, presence, định tuyến lệnh) | [REALTIME_ARCHITECTURE](docs/06-architecture/REALTIME_ARCHITECTURE.md), `specs/asyncapi` | `realtime-gateway` (không DB) | `scripts/networking` |
| Giao diện game & Admin/Ops | [DESIGN_SYSTEM](docs/05-ux/DESIGN_SYSTEM.md) | — (`design/`, `web/packages/ops-ui`) | `resources/ui/theme`, `resources/art/generated`, `scenes/ui` |
| Hạ tầng dùng chung | [BACKEND_DESIGN](docs/07-backend/BACKEND_DESIGN.md), [DEPLOYMENT](docs/11-operations/DEPLOYMENT.md) | `pkg/` (platform kit) | — |

## 7. Ngôn ngữ & phong cách

- Tài liệu: tiếng Việt, thuật ngữ kỹ thuật tiếng Anh theo [GLOSSARY](docs/00-overview/GLOSSARY.md).
- Code, identifier, commit message: tiếng Anh. Chuỗi UI: tiếng Việt qua file dịch (không hardcode chuỗi trong scene).
- Go: `gofmt`, error wrapping `fmt.Errorf("...: %w", err)`, `context.Context` là tham số đầu, `log/slog`.
- GDScript: static typing bắt buộc, `snake_case` cho hàm/biến, `PascalCase` cho class, signal ở thì quá khứ (`answer_submitted`).

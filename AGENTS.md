# AGENTS.md — Quy tắc cho AI Coding Agent và developer

File này là hợp đồng làm việc cho mọi AI agent (và developer) khi sửa repository EduWorld. Phải đọc file này trước khi thực hiện bất kỳ thay đổi nào.

Dự án theo **Spec-Driven Development** (ADR-0082): file này là **constitution** (nguyên tắc bất biến); mỗi tính năng được triển khai từ một **feature spec** chuẩn GitHub Spec Kit trong `specs/<NNN>-<slug>/` (`spec.md` → `plan.md` + `data-model.md`, `contracts/`, `quickstart.md` → `tasks.md`). Điều khoản gate: [constitution](.specify/memory/constitution.md); danh mục, thứ tự và trạng thái feature: [CODE_PLAN](docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md); lệnh cho AI agent: `/speckit.specify`, `/speckit.clarify`, `/speckit.plan`, `/speckit.tasks`, `/speckit.analyze`, `/speckit.implement` (`.claude/commands/`).

## 1. Trước khi bắt đầu

0. Chọn feature trong [danh mục feature](docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) (trạng thái `Ready`, phụ thuộc đã `Done`) và đọc toàn bộ thư mục `specs/<NNN>-<slug>/` của feature đó. Không có feature spec thì không code tính năng mới — viết feature spec trước bằng `/speckit.specify` theo [template](.specify/templates/spec-template.md).
1. Đọc [README.md](README.md), file này, và [docs/README.md](docs/README.md) (quy ước tài liệu, mã định danh).
2. Đọc [chỉ mục ADR](docs/adr/README.md) (mục "Quyết định hiện hành theo chủ đề"). **Không** làm trái ADR đã có. Muốn thay đổi thì tạo ADR mới, giải thích lý do và đánh dấu ADR cũ là Superseded.
3. Đọc tài liệu của domain sắp sửa (xem bảng §6) và contract liên quan trong [specs/](specs/README.md).
4. Lấy giá trị mặc định từ [CONFIG_DEFAULTS](docs/00-overview/CONFIG_DEFAULTS.md) (trỏ về tài liệu sở hữu), không tự đặt con số.
5. Kiểm tra [OPEN_QUESTIONS.md](docs/12-project/OPEN_QUESTIONS.md). Nếu công việc phụ thuộc một OQ chưa trả lời, dùng mặc định `[Proposed]` và ghi rõ điều đó trong báo cáo.

## 2. Quy tắc bắt buộc (không ngoại lệ)

| # | Quy tắc |
|---|---|
| R1 | Không đặt business logic trong UI (Godot scene script, React component). Logic nằm trong module backend (domain/application) hoặc service của client. |
| R2 | Không hardcode nội dung học tập, quest, câu hỏi, đáp án trong client hoặc scene. |
| R3 | Không hardcode credential, realm, client id, URL môi trường. Mọi cấu hình đi qua env/config. |
| R4 | Không commit secret. File `.env` thật nằm trong `.gitignore`, chỉ commit `.env.example`. |
| R5 | Không tin bất kỳ điểm số, kết quả, thời gian, phần thưởng nào do client gửi lên (ADR-0016). |
| R6 | Không gửi answer key/scoring rule xuống client. |
| R7 | Mọi endpoint (trừ health) phải có authentication **và** authorization nghiệp vụ trong Go. APISIX không thay thế authorization. |
| R8 | Mọi truy vấn dữ liệu nghiệp vụ phải lọc theo `tenant_id` qua `TenantScope` (ADR-0005); tenant là trường độc lập, đơn vị nhiều trường hoặc tenant chung. Mỗi phiên chỉ một tenant; chỉ `system_admin` dùng `CrossTenantScope` (dropdown tenant, audit); `safety_moderator` chỉ đọc liên tenant trong module moderation theo R20. Không có query "lấy tất cả" cho dữ liệu học sinh/chat. Không suy ra tenant từ username/email/header. |
| R9 | Không lưu dữ liệu quan trọng chỉ trong Valkey (ADR-0004). |
| R10 | Thao tác quan trọng (submit, nộp thi, kết quả trận, trao thưởng, lời mời) phải idempotent (`Idempotency-Key`) và có audit log. |
| R11 | Không sửa nội dung `Published`. Tạo version mới (ADR-0015). Attempt luôn tham chiếu `content_version_id`. |
| R12 | Không xóa migration đã chạy, không viết migration xóa dữ liệu mà chưa có ADR và kế hoạch backup. Dùng expand/contract. |
| R13 | **Clean Architecture + ranh giới module (ADR-0074, ADR-0038, ADR-0087).** Backend là modular monolith: module `identity`, `learning`, `game`, `competition`, `chat`, `realtime` trong `modules/<module>/`, chạy trong 3 binary `cmd/core-api`, `cmd/realtime`, `cmd/worker` (profile `events`/`chat`/`jobs`). Trong module: `domain` không import gì ngoài thư viện chuẩn và `pkg/` thuần; `application` chỉ import `domain` và port; `infrastructure`/`presentation` hiện thực port. Giữa module: **chỉ** import `modules/<khác>/api` (facade sinh từ spec internal) và chỉ khi module gọi có trong `x-allowed-callers` của operation; không import `modules/<khác>/internal/...` (Go `internal/` + `depguard`/`go-arch-lint` chặn); **không** đọc/ghi database của module khác (mỗi module một database, role DB riêng, không `CONNECT` chéo); giao tiếp chỉ qua facade (R29) hoặc event NATS (ADR-0042). Tách module thành service riêng cần ADR mới theo tiêu chí ADR-0074 mục 7. |
| R14 | Không log PII, token, nội dung chat thô, đáp án học sinh ở mức INFO. |
| R15 | Chat dùng **một bộ quy tắc cho mọi lớp** (ADR-0020): không thêm điều kiện theo `grade_level`/`grade_band` vào chat policy hay code chat; **mọi tin nhắn** phải đi qua pipeline kiểm tra hợp lệ phía server trước khi phát, không có đường gửi nào bỏ qua pipeline; pipeline chỉ bỏ bước khi công tắc kiểm duyệt hiệu lực (do System Admin đặt, ADR-0030) đang tắt bước đó. Không mở channel mới hoặc nới policy chat mà không cập nhật [CHILD_SAFETY](docs/10-security/CHILD_SAFETY.md) và [CHAT_SYSTEM](docs/03-social-multiplayer/CHAT_SYSTEM.md). Không hardcode từ nhạy cảm trong code/test/seed (dùng danh mục CMS, fixture chỉ placeholder); không trả từ đã khớp, vị trí khớp hay nội dung danh mục về client; công tắc kiểm duyệt (ADR-0030) chỉ đọc từ cấu hình hiệu lực, không có cờ tắt nào trong code hay biến môi trường. Chat `match`/`team` theo trận/`party` là **always-on** (ADR-0020 mục 6): không thêm cờ/policy/feature flag nào tắt được chúng ở cấp tenant/trường/lớp; xóa vật lý chat chỉ qua job `gameplay-chat-purge` theo ADR-0033, không qua API ứng dụng. |
| R16 | Không hiển thị tên thật, trường, lớp của học sinh cho người chơi khác ngoài phạm vi policy cho phép. |
| R17 | Không tạo mock/stub rồi tuyên bố production-ready. Mock phải gắn nhãn rõ ràng trong code và báo cáo. |
| R18 | Không bỏ qua error handling, logging, tracing, test. |
| R19 | Không tạo cơ chế pay-to-win, loot box trả tiền, dark pattern hay áp lực gây nghiện cho trẻ. |
| R20 | Không cấp permission `game.*`, `competition.config.*`, `platform.*`, `moderation.case.*`, `moderation.sanction.*`, `chat.tenant_terms.write`, `chat.tenant_category.toggle` cho role tenant (catalog permission trong `pkg/authz/catalog.go`, mọi binary kiểm tra lúc khởi động). Mọi endpoint cấu hình game/thi đấu nằm dưới `/api/v1/game-admin/*` (chỉ gateway nội bộ, `azp = eduworld-portal` + MFA `acr` + role platform). Admin Portal là **một ứng dụng** 5 workspace; mã workspace platform tách chunk lazy-load nhưng **không** được coi là lớp bảo mật: mọi chặn quyền phải nằm ở backend (ADR-0027 mục 3, 7). Chỉ `system_admin` được đọc liên tenant đầy đủ (`tenant_ids`), và ghi của System Admin luôn có một `tenant_id` đích. `safety_moderator` (role platform, MFA) chỉ được đọc liên tenant **trong module moderation** (case, bằng chứng, tin nhắn liên quan case) qua `tenancy.Bypass(ctx, "moderation_case")` có audit và metric; không có quyền liên tenant nào khác (ADR-0072). |
| R21 | **Chỉ soft-delete (ADR-0028).** Không viết `DELETE FROM`/`TRUNCATE` trong code ứng dụng hoặc SQL tường minh; ent hook chuyển delete thành soft-delete (ADR-0039); "xóa" = set `deleted_at`/`deleted_by` qua helper `SoftDelete` + audit + event `<entity>.deleted`. Mọi query đọc phải có `deleted_at IS NULL` (trừ bảng append-only và module Thùng rác). Unique dùng partial index `WHERE deleted_at IS NULL`. Nghĩa vụ xóa theo luật = ẩn danh hóa; Release 1.0 không có xóa vật lý hay gỡ partition (ADR-0070). **Ngoại lệ duy nhất (ADR-0033):** purge chat gameplay (`chat.gameplay_messages`, channel `match`/`party`/`team` theo trận, `chat.ephemeral_messages`) trong `modules/chat/internal/infrastructure/purge`, chạy bằng job `gameplay-chat-purge` trong profile `worker-chat` (ADR-0074) với role DB `chat_purger`; không API/use case nào khác được xóa vật lý. |
| R22 | Gán/clone dữ liệu giữa tenant chỉ System Admin làm (ADR-0029); clone không bao giờ sao chép dữ liệu học sinh (học sinh, enrollment, attempt, kết quả, chat, mastery, XP). |
| R23 | **Game chỉ dành cho học sinh (ADR-0031).** Mọi route người chơi (`/game/*` trừ `client-config`, `/multiplayer/*`, `/competition/*` phía người chơi, `/social/*`, realtime ticket) phải qua middleware `game_player` (`azp = eduworld-game-backend` + `user_type = student`, nếu không `GAME_ACCESS_STUDENT_ONLY`). Không tạo cửa sau cho nhân sự nhà trường/vận hành vào game; preview dùng dữ liệu mẫu, không tạo game session. Không bỏ quy tắc một thiết bị (unique game session active) và không lưu IP thô trong login events. |
| R24 | **Dọn học sinh không đăng nhập (ADR-0032) là soft-delete**, không hard-delete; số ngày lấy từ `platform.platform_settings`/tenant settings, không hardcode/env; luôn có báo trước, loại trừ, ngưỡng xác nhận và khôi phục khi đăng nhập lại. |
| R25 | **Chat qua NATS JetStream (ADR-0035):** không phát (fan-out) tin chat nào chưa được JetStream ack; không ghi tin chat trực tiếp vào PostgreSQL từ realtime (chỉ consumer `chat-persister` trong profile `worker-chat`); game client/CMS **không bao giờ** kết nối NATS hay Valkey. Tính năng chat mới (ADR-0034) phải: chạy lại kiểm duyệt khi tạo nội dung mới (sửa, forward), giữ bằng chứng khi thu hồi, không trả nội dung gốc đã che. |
| R26 | **CQRS tách read/write context (ADR-0038, ADR-0087).** **Write context:** command handler trong `internal/application/command/` nạp aggregate qua port `port/write`, gọi phương thức domain, lưu bằng ent trong transaction (`UnitOfWork`), ghi outbox cùng transaction, trả ID/version. **Read context:** query handler trong `internal/application/query/` chỉ đọc read model/projection qua port `port/read` (được dùng replica/cache), không ghi DB, không phát event, không dùng `UnitOfWork`. **Không dùng mediator/bus chung**; presentation gọi handler qua interface có kiểu. Cross-cutting: HTTP middleware `pkg/httpx` (xác thực, tenant, idempotency, log, tracing, Problem Details); authorization gọi tường minh `authz.Require` đầu mỗi use case; validation trong DTO/handler. Không event sourcing. |
| R27 | **ent global filter (ADR-0039, ADR-0089).** Không tạo client/query ent bỏ interceptor `tenant_id` + `deleted_at IS NULL`; mỗi entity dùng đúng một mixin `pkg/entx`. Bỏ filter chỉ qua `pkg/tenancy.Bypass(ctx, reason)` có audit và metric, với `reason` thuộc danh sách đóng `tenancy.BypassReason` của ADR-0089 (thêm reason mới phải sửa ADR); linter cấm lý do khác. RLS chỉ lọc tenant; `deleted_at` do interceptor và view `<table>_live` bảo đảm. SQL tường minh ở Query side phải tự có điều kiện tenant + soft-delete và có test. Không auto-migrate khi khởi động; migration do Atlas quản lý, phần ent không biểu diễn được (RLS, trigger, GRANT/REVOKE, view `_live`, hàm `SECURITY DEFINER`, DDL partition) viết tay trong cùng thư mục và được review như migration sinh tự động. |
| R28 | **Mọi chuỗi có giới hạn độ dài (ADR-0041).** Mọi field string trong ent schema có `MaxLen(n)` → `varchar(n)`; không dùng `text` không giới hạn. Mọi `type: string` trong OpenAPI (public + internal), AsyncAPI, JSON Schema, event schema có `maxLength` (trừ `format: uuid/date/date-time`, enum, hoặc `pattern` giới hạn). Dùng bảng độ dài chuẩn trong ADR-0041. CI (Spectral, ajv, `atlas migrate lint`) chặn merge nếu thiếu. |
| R29 | **Gọi module khác chỉ qua facade có kiểu (ADR-0074, ADR-0040).** Facade sinh bằng `make gen` từ `specs/openapi/modules/<callee>.internal.v1.yaml` (kiểu DTO + interface strict-server vào `modules/<callee>/api`), gọi trong tiến trình qua adapter `infrastructure` sau một port của `application`, bằng helper `pkg/modcall` (span, deadline, `TenantScope`, Principal gốc, `Idempotency-Key`, ánh xạ lỗi qua `context`). Không HTTP nội bộ, không token `svc-*` giữa module, không gọi thẳng struct/hàm nội bộ của module khác. Operation mới phải khai báo `x-allowed-callers` và có facade test (bên gọi với fake facade, bên nhận với ví dụ trong spec). Tích hợp bên ngoài (Keycloak Admin, SIS, object storage) đi qua adapter có timeout, retry chỉ cho thao tác idempotent, circuit breaker. Phản ứng phụ (XP, quest, projection, mở channel chat) dùng event, không gọi đồng bộ. |
| R30 | **Đường đăng nhập phải nhẹ (ADR-0045, ADR-0046).** Không gọi SIS hay module khác đồng bộ trên `/auth/session` và `/game/sessions` (trừ on-demand lookup lần đầu qua IdP theo ADR-0046); dùng JWKS/Principal/tenant cache. Không đoán tenant hay SIS nguồn từ username/email. Không giới hạn đăng nhập theo IP đơn lẻ (NAT trường học). Không tự xác thực mật khẩu/PIN/QR ngoài Keycloak và `login-credentials:verify`. |
| R31 | **Phát hành nội dung (ADR-0043, ADR-0044).** Không sửa version đã publish; rollback chỉ đổi con trỏ `current_version_id` và ghi `content_releases`. Mọi lượt chơi/học phải ghim version khi bắt đầu và chỉ đọc theo version đã ghim. Không đưa câu hỏi đề, answer key hay media private lên CDN công khai. File media chỉ ở object storage (không lưu blob trong DB), khóa `media/<sha256>/…` bất biến. |
| R32 | **Hai design system (ADR-0047).** CMS/LMS/Admin dùng Admin/Ops UI (Ant Design 5 + Pro, theme sinh từ `design/admin-tokens/`); **không** mang phong cách, component hay Theme của game vào web. Game chỉ dùng Theme/token sinh từ `design/game-tokens/`; không dùng thư viện Admin UI trong Godot. Không hardcode màu/font/cỡ chữ/khoảng cách trong CSS/TSX hay `.tscn`; mọi giá trị lấy từ token. File sinh từ token (`eduworld_theme.tres`, `ui_tokens.gd`, `resources/art/generated/`, theme web) không sửa tay: sửa `design/` rồi `make design-tokens`. Mã màu nghiệp vụ (môn học, độ hiếm, trạng thái nội dung) chỉ khai báo ở `design/shared-tokens/`. |
| R33 | **Một nguồn sở hữu cho mỗi quy tắc/tham số.** Mỗi quy tắc, tham số mặc định, sơ đồ hay quyết định chỉ được định nghĩa ở **một** tài liệu sở hữu ([docs/README §3.3](docs/README.md)); tài liệu khác chỉ link. Tham số mặc định đăng ký ở [CONFIG_DEFAULTS](docs/00-overview/CONFIG_DEFAULTS.md); đổi giá trị phải sửa tài liệu sở hữu + CONFIG_DEFAULTS + code/config trong cùng MR. Khi viết code, đọc giá trị từ cấu hình (platform/tenant settings, cấu hình game có version), không hardcode. Di chuyển/gộp tài liệu bằng `scripts/docs/*.mjs` và chạy `make docs-check`. **Mã FR chỉ do PRD cấp**; tài liệu domain không có bảng FR cục bộ với mã riêng (ADR-0082). |
| R34 | **Spec-Driven Development và phạm vi (ADR-0082).** Mọi tính năng triển khai từ feature spec `specs/<NNN>-<slug>/` (chuẩn Spec Kit); `plan.md` phải qua Constitution Check; mọi tiêu chí `F<NNN>-AC-*` (EARS) phải có test pass trước khi đánh dấu `Done`. Không tự mở rộng phạm vi ngoài `spec.md`; thiếu/mâu thuẫn thì dừng và ghi "Câu hỏi mở". **Không đọc để triển khai** `docs/99-deferred/` và `specs/_deferred/` (kỹ năng viết/nói, pháp lý — không làm trong version này); không sinh code, endpoint, bảng hay màn hình cho các nội dung đó. |

## 3. Quy trình cho mỗi feature

Quy trình dưới đây đã được viết sẵn thành `tasks.md` trong từng feature spec; làm theo `tasks.md`, bước nào thiếu thì bổ sung vào `tasks.md` trước khi làm.

1. Đọc feature spec (`spec.md`: FR, tiêu chí EARS, tham số; `plan.md`: thiết kế) và requirement gốc (`FR-*`, `US-*` trong [PRODUCT_REQUIREMENTS](docs/00-overview/PRODUCT_REQUIREMENTS.md)).
2. Thiết kế giải pháp (đã có trong `plan.md`; cập nhật nếu cần). Nếu quyết định có ảnh hưởng kiến trúc thì viết ADR.
3. Xác định **module sở hữu** (bảng §6). Tính năng chạm nhiều module: thiết kế facade (spec internal)/event trước, thứ tự triển khai module nhận → module gọi.
4. Cập nhật contract trước: `specs/openapi/modules/<module>.v1.yaml` (public), `<module>.internal.v1.yaml` (facade), `specs/asyncapi`, `specs/schemas`, `specs/events`; chạy `make gen MOD=<module>`.
5. Ent schema (có `MaxLen`) → `make migrate-diff` (Atlas) → domain → application (command/query handler) → infrastructure (repository ent, typed client, publisher) → presentation (handler HTTP/consumer).
6. Godot client (client service → state → UI).
7. CMS/LMS nếu liên quan.
8. Viết test: unit theo tầng, integration (database riêng của module, global filter), facade test với module liên quan, test kiến trúc (import chéo module); thêm test tenant isolation và authorization cho mọi endpoint và operation facade mới.
9. Chạy test, lint, format.
10. Cập nhật tài liệu và CHANGELOG.
11. Báo cáo theo mẫu §5.

## 4. Lệnh kiểm tra (áp dụng từ Phase 1)

| Phạm vi | Lệnh dự kiến |
|---|---|
| Go (theo module) | `make gen-check MOD=<module>` · `make lint MOD=<module>` (gofmt, go vet, golangci-lint, go-arch-lint tầng Clean Architecture) · `make test MOD=<module>` · `make test-integration MOD=<module>` (testcontainers) · `make migrate-lint MOD=<module>` (Atlas) |
| Liên module | `make test-contract` (facade test bên gọi/bên nhận theo spec internal) · `make arch-check` (import chéo module, `x-allowed-callers`) · `make test-e2e` |
| Specs | `make specs-lint` (Spectral + rule `maxLength` bắt buộc, AsyncAPI validate, ajv JSON Schema/event schema, `check-enums`) · `node scripts/specs/check-features.mjs` (feature spec SDD) · `node scripts/specs/check-cross-features.mjs` (mỗi event một chủ sở hữu, bảng "Mới" không trùng) · `node scripts/specs/check-portal-catalog.mjs` (danh mục menu/action portal ↔ `x-portal-action`, ADR-0090) · `make specs-breaking` (oasdiff cho spec public và internal) |
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

## 6. Domain → tài liệu → module

| Domain | Tài liệu sở hữu | Module (ADR-0074) · database | Thư mục Godot |
|---|---|---|---|
| Thiết kế game tổng, thế giới, cốt truyện | [GAME_DESIGN](docs/01-game-design/GAME_DESIGN.md), [WORLD_DESIGN](docs/01-game-design/WORLD_DESIGN.md) | module `game` · `eduworld_game` | `scenes/world`, `scripts/world` |
| Nhân vật, avatar, emote | [CHARACTER_SYSTEM](docs/01-game-design/CHARACTER_SYSTEM.md) | module `game` | `scenes/character`, `scripts/player` |
| XP, level, quest, challenge/boss, kinh tế, vật phẩm, sưu tầm, game session, Game Operations Console | [PROGRESSION_SYSTEM](docs/01-game-design/PROGRESSION_SYSTEM.md), [QUEST_SYSTEM](docs/01-game-design/QUEST_SYSTEM.md), [CHALLENGE_BOSS_SYSTEM](docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md), [INVENTORY_ECONOMY](docs/01-game-design/INVENTORY_ECONOMY.md) | module `game` | `scenes/quest`, `scripts/inventory` |
| Thi đấu, ranking, leaderboard, tournament | [COMPETITION_SYSTEM](docs/01-game-design/COMPETITION_SYSTEM.md) | module `competition` · `eduworld_competition` | `scenes/competition` |
| Party, room, matchmaking, multiplayer gameplay | [PARTY_SYSTEM](docs/03-social-multiplayer/PARTY_SYSTEM.md), [MATCHMAKING](docs/03-social-multiplayer/MATCHMAKING.md), [MULTIPLAYER_GAMEPLAY](docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | module `competition` | `scripts/multiplayer` |
| Bạn bè, club/guild, chat, kiểm duyệt, thông báo, sticker/GIF | [SOCIAL_SYSTEM](docs/03-social-multiplayer/SOCIAL_SYSTEM.md), [CHAT_SYSTEM](docs/03-social-multiplayer/CHAT_SYSTEM.md), [MODERATION](docs/03-social-multiplayer/MODERATION.md) | module `chat` · `eduworld_chat` | `scenes/chat`, `scenes/social` |
| Trải nghiệm học tập, curriculum, nội dung & phát hành, question bank | [LEARNING_EXPERIENCE](docs/02-learning/LEARNING_EXPERIENCE.md), [CURRICULUM_MODEL](docs/02-learning/CURRICULUM_MODEL.md), [CONTENT_MODEL](docs/02-learning/CONTENT_MODEL.md), [QUESTION_BANK](docs/02-learning/QUESTION_BANK.md) | module `learning` · `eduworld_learning` | `scripts/learning` |
| Activity & attempt (quiz, tương tác, video, audio, tự luận, mô phỏng, mini-game), đề thi | [ACTIVITY_ENGINE](docs/02-learning/ACTIVITY_ENGINE.md) + tài liệu từng định dạng, [EXAM_ENGINE](docs/02-learning/EXAM_ENGINE.md), [MINIGAME_ENGINE](docs/08-game-client/MINIGAME_ENGINE.md) | module `learning` | `scenes/learning`, `activity_renderers/` |
| Mastery, analytics, LMS, School Learning Dashboard (dashboard trường) | [LEARNING_ANALYTICS](docs/02-learning/LEARNING_ANALYTICS.md), [SCHOOL_LMS](docs/04-admin-portal/SCHOOL_LMS.md) | module `learning` | — (web/) |
| Admin Portal (5 workspace) | [ADMIN_PORTAL](docs/04-admin-portal/ADMIN_PORTAL.md) | gọi API public của `core-api` | — (`web/apps/portal`) |
| Identity, SSO, tenant, phân quyền, audit, settings, saga, SIS, dọn học sinh | [KEYCLOAK_SSO](docs/09-identity/KEYCLOAK_SSO.md), [IDENTITY_MAPPING](docs/09-identity/IDENTITY_MAPPING.md), [ACCOUNT_PROVISIONING](docs/09-identity/ACCOUNT_PROVISIONING.md), [SIS_INTEGRATION](docs/09-identity/SIS_INTEGRATION.md), [MULTITENANCY](docs/06-architecture/MULTITENANCY.md) | module `identity` · `eduworld_identity` | `scripts/authentication` |
| Realtime (WebSocket, presence, định tuyến lệnh) | [REALTIME_ARCHITECTURE](docs/06-architecture/REALTIME_ARCHITECTURE.md), `specs/asyncapi` | module/binary `realtime` (không DB) | `scripts/networking` |
| Giao diện game & Admin/Ops | [DESIGN_SYSTEM](docs/05-ux/DESIGN_SYSTEM.md) | — (`design/`, `web/packages/ops-ui`) | `resources/ui/theme`, `resources/art/generated`, `scenes/ui` |
| Hạ tầng dùng chung | [BACKEND_DESIGN](docs/07-backend/BACKEND_DESIGN.md), [DEPLOYMENT](docs/11-operations/DEPLOYMENT.md) | `pkg/` (platform kit) | — |

## 7. Ngôn ngữ & phong cách

- Tài liệu: tiếng Việt, thuật ngữ kỹ thuật tiếng Anh theo [GLOSSARY](docs/00-overview/GLOSSARY.md).
- Code, identifier, commit message: tiếng Anh. Chuỗi UI: tiếng Việt qua file dịch (không hardcode chuỗi trong scene).
- Go: `gofmt`, error wrapping `fmt.Errorf("...: %w", err)`, `context.Context` là tham số đầu, `log/slog`.
- GDScript: static typing bắt buộc, `snake_case` cho hàm/biến, `PascalCase` cho class, signal ở thì quá khứ (`answer_submitted`).

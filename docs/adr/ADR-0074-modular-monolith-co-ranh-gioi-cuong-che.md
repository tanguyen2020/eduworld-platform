# ADR-0074 — Modular monolith có ranh giới cưỡng chế thay cho 6 microservice

> **Trạng thái:** Accepted · supersedes ADR-0036 · một phần superseded by [ADR-0092](ADR-0092-to-chuc-ma-nguon-trong-src.md) (đường dẫn mã) · một phần supersedes ADR-0037, ADR-0040 · **Ngày:** 2026-11-15 · **OQ:** OQ-10 · **Tài liệu:** [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) · [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) · [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) · [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

> [!NOTE]
> **Đường dẫn mã đổi theo ADR-0092:** Go module ở `src/backend/` (`src/backend/cmd/`, `src/backend/modules/<module>/`, `src/backend/pkg/`); Admin Portal ở `src/cms/frontend/`; game ở `src/game/`. Mọi quyết định khác của ADR này giữ nguyên.

## Bối cảnh

[ADR-0036](ADR-0036-microservices-theo-he-thong-lon-thay-adr-0001.md) chia backend thành 6 microservice Go, mỗi service một database, giao tiếp bằng typed client + outbox/NATS + saga. Bối cảnh của ADR đó chỉ có một câu ("Stakeholder quyết định chia microservices vì có nhiều hệ thống lớn"), không có phương án đã loại và không phân tích năng lực đội.

Đội dự kiến khoảng 3 Go dev và 0,5 DevOps ([ROADMAP §1](../12-project/ROADMAP.md)), chạy Docker Swarm không tự scale. Đường nóng vẫn gọi đồng bộ (`principals:resolve`, bắt đầu attempt, lấy/chấm câu hỏi thi đấu), nên lợi ích cô lập lỗi của microservice chủ yếu trên danh nghĩa (RISK-27…30, [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) K6).

Phần có hồ sơ chạy khác biệt thật sự là realtime (WebSocket sống lâu, cần drain khi deploy), chat (lưu lượng tăng đột biến, cô lập an toàn, job purge với role DB riêng) và các job nền. Technical Architect đánh giá ngày 2026-11-17 và chọn modular monolith tách theo hồ sơ chạy.

## Quyết định

### 1. Module và mã nguồn

- Một Go module ở gốc repository (`go.mod` duy nhất), giữ `pkg/` platform kit.
- Năm module domain giữ đúng bounded context của ADR-0036: `identity`, `learning`, `game`, `competition`, `chat`. Thêm module `realtime` (không có DB, không nạp module domain) cho WebSocket gateway. Room host là thư viện `pkg/roomhost` (máy trạng thái phòng, trạng thái trong Valkey có lease theo phòng), được module `competition` dùng cho trận battle/co-op (`ew.cmd.competition.*`) và module `game` dùng cho boss/challenge (`ew.cmd.game.boss.*`); cả hai chạy trong `core-api` dưới dạng handler NATS và chấm câu trả lời qua facade `learning` trong tiến trình.
- Cấu trúc mỗi module: `modules/<module>/`
  - `api/`: **facade công khai** duy nhất được module khác import (interface + DTO sinh từ spec internal, mục 3);
  - `internal/{domain,application,infrastructure,presentation}`: Clean Architecture và CQRS theo ADR-0038/ADR-0087;
  - `module.go`: hàm `Register(...)` để composition root nối module.
- Quy tắc `internal/` của Go chặn import chéo vào bên trong module ở mức compiler. `depguard`/`go-arch-lint` chặn thêm: module chỉ import `modules/<khác>/api` khi được liệt kê trong `x-allowed-callers` của operation tương ứng.

### 2. Ba binary, nhiều profile triển khai

| Binary | Nội dung | Triển khai (Swarm service) |
|---|---|---|
| `cmd/core-api` | REST public của mọi module (`/api/v1/*`), route `game-admin` cho gateway nội bộ, handler NATS request/reply cho lệnh từ realtime (kể cả room host của `competition`) | `core-api`, ≥ 2 replica |
| `cmd/realtime` | Kết thúc WebSocket `/realtime/v1`, xác thực ticket, presence, định tuyến lệnh qua NATS; không chứa logic nghiệp vụ | `realtime`, ≥ 2 replica, drain khi deploy |
| `cmd/worker` | Outbox relay, consumer event, job định kỳ; chọn nhóm việc bằng `--profile` | `worker-events` (relay + consumer domain), `worker-chat` (`chat-persister`, `chat-notifier`, `chat-moderation-async`, `chat-stats`, `gameplay-chat-purge`), `worker-jobs` (job định kỳ, có khóa leader) |

Một profile hỏng hoặc quá tải không chặn profile khác. Chat tăng đột biến chỉ cần tăng replica `worker-chat`.

### 3. Gọi giữa module: trong tiến trình, hợp đồng vẫn là spec internal

- Spec `specs/openapi/modules/<module>.internal.v1.yaml` (đổi thư mục từ `specs/openapi/services/`) **giữ vai trò hợp đồng** của facade: operation, schema có `maxLength` (R28), `x-allowed-callers`.
- `make gen` sinh từ spec internal: kiểu DTO và interface strict-server vào `modules/<module>/api/`. Module đích hiện thực interface; module gọi dùng interface đó qua adapter `infrastructure` sau một port `application` (giữ R29 về cấu trúc).
- Không có HTTP nội bộ, không token `svc-*`, không retry/circuit breaker cho gọi giữa module. Ngữ cảnh truyền qua `context.Context` bằng helper `pkg/modcall`: span tracing, deadline, `TenantScope`, Principal gốc (on-behalf), `Idempotency-Key`, ánh xạ lỗi sang `problem`.
- Phản ứng phụ (XP, quest, projection, mở channel chat) vẫn qua outbox → NATS JetStream (ADR-0042), không gọi facade.
- Khi tách một module thành service, cùng spec internal sinh HTTP server/client; người gọi chỉ đổi adapter.

### 4. Dữ liệu: mỗi module một database trên một cụm PostgreSQL

- Giữ năm database của ADR-0037 (`eduworld_identity`, `eduworld_learning`, `eduworld_game`, `eduworld_competition`, `eduworld_chat`), mặc định chung **một cụm** PostgreSQL ở mọi môi trường, qua PgBouncer. Lý do giữ database riêng thay vì một database chia schema: mỗi database đã có schema `infra` và `ref` riêng; ranh giới không truy cập chéo được cưỡng chế bằng `CONNECT`; tách module sau chỉ cần chuyển database.
- Role DB theo module (`<module>_app`, `<module>_platform`, `<module>_readonly`, `<module>_migrator`, `<module>_relay`, `<module>_partition_admin`, `chat_purger`) theo ADR-0037 và ADR-0089. Ký hiệu `<svc>` trong tài liệu cũ đọc là `<module>`.
- `core-api` và `worker` mở một pool kết nối cho mỗi database của module mình nạp. Không join, không transaction chéo database. Thao tác ghi vắt qua nhiều module vẫn dùng saga/outbox như hiện nay, nhưng các bước gọi facade trong tiến trình.
- Backup một cụm (pgBackRest), khôi phục được theo từng database.

### 5. Gateway và định danh

- APISIX public: `/api/v1/*` → `core-api`, `/realtime/v1` → `realtime`. Gateway nội bộ: `/api/v1/game-admin/*` → `core-api` (giữ R20).
- Bỏ các Keycloak client `svc-learning`, `svc-game`, `svc-competition`, `svc-chat`, `svc-realtime`. Giữ `svc-identity` chỉ để gọi Keycloak Admin API.
- Tích hợp bên ngoài (Keycloak Admin, SIS, object storage) vẫn có timeout, retry cho thao tác idempotent và circuit breaker trong adapter riêng.

### 6. Ánh xạ tên cũ

ADR cũ giữ tên service như bản ghi lịch sử; khi đọc, áp bảng sau. Tài liệu và feature spec đang hiệu lực dùng tên mới.

| Tên cũ (ADR-0036) | Tên mới | Thư mục mã |
|---|---|---|
| `identity-service` | module `identity` | `modules/identity/` |
| `learning-service` | module `learning` | `modules/learning/` |
| `game-service` | module `game` | `modules/game/` |
| `competition-service` | module `competition` | `modules/competition/` |
| `chat-service` | module `chat` | `modules/chat/` |
| `realtime-gateway` | module/binary `realtime` | `modules/realtime/`, `cmd/realtime` |
| `chat-worker`, `chat-persister` | consumer trong profile `worker-chat` | `cmd/worker --profile=chat` |
| binary `api` / `worker` của từng service | `cmd/core-api`, `cmd/worker` | `cmd/` |
| `specs/openapi/services/` | `specs/openapi/modules/` | — |
| `pkg/svcclient` | `pkg/modcall` (gọi facade trong tiến trình) | `pkg/modcall/` |
| `<svc>_app`, `<svc>_platform`… | `<module>_app`, `<module>_platform`… (tên database giữ nguyên) | — |

### 7. Tiêu chí tách module thành service

Chỉ tách khi có số đo: tải khác biệt rõ mà tăng replica profile không giải quyết được, nhịp phát hành khác, hoặc đội backend lớn hơn 2 squad. Chat là ứng viên tách đầu tiên. Mỗi lần tách cần ADR mới.

## Hệ quả

(+) Bớt 5 bộ image/pipeline/dashboard, bỏ token và contract test HTTP nội bộ, giảm RAM cho dev cục bộ. Giữ ranh giới dữ liệu (database riêng) và ranh giới mã (compiler + linter), nên tách sau rẻ. Hồ sơ chạy khác biệt (realtime, chat, job) vẫn scale riêng.

(−) Một lỗi panic không bắt trong `core-api` ảnh hưởng mọi module; giảm bằng recover middleware, nhiều replica và giới hạn tài nguyên. Phải sửa SYSTEM_ARCHITECTURE, BACKEND_DESIGN, DATABASE_DESIGN, API_DESIGN, LOCAL_DEVELOPMENT, DEPLOYMENT, AGENTS R13/R29, CODE_PLAN và các feature spec nhắc `services/<svc>`.

## Phương án đã loại

- **Giữ 6 microservice (ADR-0036):** chi phí vận hành vượt năng lực đội; cô lập lỗi chủ yếu trên danh nghĩa vì đường nóng gọi đồng bộ.
- **Hybrid (chat là service riêng từ đầu):** thêm một DB riêng cụm, pipeline và hợp đồng HTTP mà chưa có số đo cần thiết. Ranh giới an toàn của chat đã có nhờ database riêng và role `chat_purger`.
- **Một database chia schema:** đụng tên schema `infra`/`ref`, ranh giới chỉ dựa vào grant.
- **Monolith không ranh giới:** khó tách sau, vi phạm Clean Architecture.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-17 | Accepted (phương án 1 của đánh giá topology); chốt cấu trúc `modules/`, 3 binary + profile worker, database theo module trên một cụm, facade sinh từ spec internal | Technical Architect |
| 2026-11-17 | Làm rõ: room host thuộc `competition` trong `core-api` (không ở binary `realtime`); `producer` của envelope event đổi thành `<module>/<thành phần>` | Technical Architect |
| 2026-11-17 | Đường dẫn mã chuyển vào `src/` theo ADR-0092 | Technical Architect |

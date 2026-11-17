# ADR-0036 — Microservices theo hệ thống lớn

> **Trạng thái:** Superseded by [ADR-0074](ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · trước đó supersedes [ADR-0001](ADR-0001-go-modular-monolith-voi-3-tien-trinh-trien-khai.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md), [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md), [API_DESIGN](../07-backend/API_DESIGN.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Đã được thay bởi ADR-0074.** Không còn hiệu lực: 6 service build/triển khai độc lập, binary `api`/`worker` theo service. **Còn hiệu lực (chuyển sang ADR-0074):** ranh giới bounded context của 5 module domain và vai trò của realtime.

## Bối cảnh

Stakeholder quyết định chia microservices vì có nhiều hệ thống lớn (chat, CMS, game), mức chia theo hệ thống lớn, giữ Golang. Quyết định được giữ khi đối chiếu Master Prompt "Re-architecture" (yêu cầu modular monolith) — DOCUMENTATION_AUDIT C1.

## Quyết định

Backend gồm **6 service Go**, mỗi service build, triển khai, scale độc lập, có binary `api` và (nếu cần) `worker`:

| Service | Bounded context | Database |
|---|---|---|
| `identity-service` (Platform) | Identity, Tenant, Authorization/permission catalog, SIS sync & provisioning, membership Keycloak, student cleanup, platform settings, audit store, điều phối Thùng rác/gán/clone | `eduworld_identity` |
| `learning-service` (CMS/Learning/LMS) | Curriculum, Content, Question Bank, Activity Engine, Exam, Learning Record & Mastery, Learning Analytics, LMS, School Learning Dashboard; giữ answer key (ADR-0016) | `eduworld_learning` |
| `game-service` | Game profile, World/Area, Quest, Mini-game config, Progression/XP, Challenge/Boss, Inventory/Economy, Achievement/Collection, Game session, login events, Game Operations config | `eduworld_game` |
| `competition-service` (Multiplayer & Competition) | Party, Room, Matchmaking, Battle/Co-op state, Rating, Leaderboard, Tournament, Season | `eduworld_competition` |
| `chat-service` (Social & Chat) | Friend, Block, Guild/Club (Team, Study Group), Chat (mọi channel), Moderation, Sensitive terms, Notification, sticker/GIF | `eduworld_chat` |
| `realtime-gateway` | Kết thúc WebSocket `/realtime/v1`, xác thực ticket, presence, định tuyến lệnh/sự kiện | Chỉ Valkey |

- **Đồng bộ:** REST service-to-service qua typed client (ADR-0040). **Bất đồng bộ:** domain event qua NATS JetStream (ADR-0042).
- Không service nào đọc database của service khác; dữ liệu tham chiếu được **chiếu (projection)** vào DB service dùng nó qua event (ADR-0037).
- Thao tác nhiều service (clone, cleanup học sinh, khôi phục Thùng rác) dùng **saga** do identity-service điều phối, mỗi bước idempotent, có bù trừ.
- APISIX định tuyến theo prefix tới service (bảng đầy đủ trong API_DESIGN).
- **Định tuyến realtime:** gateway chỉ giữ kết nối; lệnh WS chuyển tới service sở hữu bằng **NATS request/reply** (`ew.cmd.<service>.<domain>.<action>`, header có `tenant_id`, `user_id`, `game_session_id`); input hub tần suất cao qua NATS core `ew.in.room.<room_id>`; service phát sự kiện qua **Valkey Pub/Sub** (`ew:rt:user:<user_id>`, `ew:rt:room:<room_id>`, `ew:chat:fanout:<channel_id>`), gateway subscribe và giao xuống WS.
- **Spec:** spec public gộp `specs/openapi/eduworld-api.v1.yaml` là nguồn sự thật, mỗi operation có `x-service`; spec public theo service (`specs/openapi/services/<service>.v1.yaml`) **sinh tự động** từ spec gộp; spec internal `<service>.internal.v1.yaml` viết tay.

## Hệ quả

(+) Chat, Learning, Game scale và phát hành độc lập; sự cố một service không kéo sập toàn hệ thống. (−) Cần service discovery, tracing phân tán, contract test, saga; độ phức tạp vận hành tăng (RISKS).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted, supersede ADR-0001 | Stakeholder |
| 2026-11-15 | Bổ sung định tuyến realtime qua NATS request/reply + Valkey Pub/Sub | Thiết kế realtime |
| 2026-11-15 | Nguồn sự thật spec: spec gộp, spec theo service sinh tự động | API_DESIGN |
| 2026-11-15 | Giữ microservices khi audit theo Master Prompt mới | DOCUMENTATION_AUDIT §4.1 |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |
| 2026-11-17 | Superseded by ADR-0074 (modular monolith) | Technical Architect |

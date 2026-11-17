# Research: F091 — Hub instance, presence, đồng bộ vị trí

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Module sở hữu hub instance

- **Decision**: module competition là room host của `hub_instance` (dùng `pkg/roomhost`); realtime chỉ định tuyến (`hub.*` → `ew.cmd.competition.*`, `hub.player.move` → `ew.in.room.*`) và giữ presence online/offline. module game không tham gia đường hub (chỉ phát event hồ sơ, area, mở khóa).
- **Rationale**: Tài liệu sở hữu đều thống nhất: [REALTIME_ARCHITECTURE §1, §4.1, §6](../../docs/06-architecture/REALTIME_ARCHITECTURE.md) (thay đổi v0.3), [MULTIPLAYER_GAMEPLAY §3, §4](../../docs/03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md), [VALKEY_DESIGN §3.1](../../docs/07-backend/VALKEY_DESIGN.md) (owner `competition · room`), [DATABASE_DESIGN §2.9](../../docs/07-backend/DATABASE_DESIGN.md) (`multiplayer.rooms` trong `eduworld_competition`), AGENTS §6. Party (cũng ở competition) cần đặt cả nhóm vào cùng instance — cùng service tránh gọi chéo.
- **Alternatives**: module game (theo CODE_PLAN §4 và mô tả định tuyến AsyncAPI) — mâu thuẫn bốn tài liệu sở hữu và buộc party (competition) gọi đồng bộ module game; bị loại. Cần sửa: CODE_PLAN dòng F091, AsyncAPI `info.description`, LOCAL_DEVELOPMENT §4 (biến `EW_RT_HUB_TICK_HZ`, `EW_RT_HUB_INSTANCE_CAPACITY` đặt ở realtime → chuyển sang module competition).

## R2. Dữ liệu area runtime và mở khóa

- **Decision**: module competition giữ projection `ref.world_areas` (từ event mới `content.published` (`item_type = world_area`) của module game: capacity, `max_speed_mps`, biên, spawn point, nhóm khối được vào, `requires_unlock`, status) và `ref.area_unlocks` (từ `world.area_unlocked`, đã có trong catalog EVENT_DESIGN §4.6). `JoinHubInstance` chỉ đọc projection.
- **Rationale**: Đường vào khu là đường nóng (bão kết nối đầu giờ); R29/R30 tránh gọi đồng bộ; projection theo [DATABASE_DESIGN §2.17](../../docs/07-backend/DATABASE_DESIGN.md).
- **Alternatives**: Gọi facade `game` mỗi lần join (thêm độ trễ, phụ thuộc sẵn sàng của module game, trái NFR-AVAIL-005); cấu hình tĩnh trong module competition (trái R2/R33 — dữ liệu area do Game Ops quản lý). REST `enterWorldArea`/`joinFriendInWorld` của module game giữ vai trò cũ sẽ phải gọi competition để gán instance — cần thống nhất với F071 (contracts §6).

## R3. Tín hiệu presence cho module khác

- **Decision**: Gateway publish NATS core (không JetStream) subject `ew.rt.presence.<tenant_id>` mỗi khi presence đổi, payload `presence-changed.schema.json` (`user_id`, `tenant_id`, `school_id`, `status`, `activity`, `area_id`, `hidden`, `ts`). module chat (F095) subscribe để gửi cho bạn bè; competition (F101) dùng `party.member.status_changed` riêng. Room host đẩy `area_id`/`room_id` cho gateway bằng control `presence.update` trên `ew:rt:user:<id>` (envelope F090 research R1).
- **Rationale**: Presence là ephemeral (ADR-0004), chấp nhận mất; mỗi key Valkey chỉ một owner nên module khác không đọc `ew:gateway:presence:*` ([VALKEY_DESIGN §1](../../docs/07-backend/VALKEY_DESIGN.md)).
- **Alternatives**: module chat đọc trực tiếp key presence của gateway (trái quy tắc owner key); domain event qua outbox (gateway không có DB).

## R4. Đóng gói snapshot theo gateway node

- **Decision**: Lúc join, room host lưu `Ew-Node-Id` của thành viên; mỗi tick tạo một gói cho mỗi node chứa snapshot riêng từng người xem (`target.user_ids`) và publish `ew:rt:room:<room_id>:out` với `droppable = true`; gateway chỉ giao phần của user local. Cập nhật node khi thành viên resume ở node khác (lệnh `hub.instance.join` lại với cùng `room_id`).
- **Rationale**: Giảm số message Pub/Sub theo [REALTIME_ARCHITECTURE §6, §11](../../docs/06-architecture/REALTIME_ARCHITECTURE.md).
- **Alternatives**: Một message mỗi người xem (40 × 10 Hz × số instance — vượt ngân sách Pub/Sub); một snapshot chung không lọc AOI (vượt `hub_sync_bandwidth`).

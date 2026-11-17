# Data Model: F079 — Workspace A — Game Operations Console UI (world, NPC, quest builder, reward, item, event, season)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F079 là frontend, không tạo hay sửa bảng. Cấu hình game, version, release, media render, client manifest thuộc module game (F071–F078).

Trạng thái phía client (không bền vững):

| Trạng thái | Nơi giữ | Ghi chú |
|---|---|---|
| Bản nháp đang soạn | RAM + autosave lên server (`updateGameConfigDraft`) | Không lưu browser storage; cảnh báo rời trang khi autosave lỗi |
| ETag version đang mở | RAM | Gửi `If-Match` |
| Bộ lọc danh sách | URL query | Chia sẻ link được |
| Preview token | RAM, hết hạn theo `expires_at` | Không log |

Schema dùng (chỉ đọc định nghĩa): `GameConfigType`, `GameConfigInput`, `GameConfigVersion`, `ContentStatus`, `PublishRequest`, `RollbackRequest`, `ContentRelease` trong `specs/openapi/eduworld-api.v1.yaml`; payload theo `specs/schemas/{quest,npc-dialogue,minigame-config,reward-rule,progression-curve,achievement,challenge,boss,shop-item,event,season-track}.schema.json`.

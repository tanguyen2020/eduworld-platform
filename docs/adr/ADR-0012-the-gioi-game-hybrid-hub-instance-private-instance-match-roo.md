# ADR-0012 — Thế giới game Hybrid: hub instance + private instance + match room

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-05 · **Tài liệu:** [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md), [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

OQ-05: thế giới persistent MMO, multiplayer hub hay hybrid. Học sinh cần thấy bạn bè nhưng không cần một thế giới liên tục quy mô lớn.

## Quyết định

- **Hub/Zone instance** (Main Hub, các khu học tập, Social Zone…): shard tối đa 40 người [Confirmed], cấp theo `(tenant/school, grade band)`; bạn bè/party được ưu tiên vào cùng instance.
- **Private instance:** cho party/quest co-op/boss, chỉ thành viên party.
- **Match room:** cho Knowledge Battle/Team Battle/trận giải đấu, vòng đời ngắn.
- Không có persistent MMO world liên tục; trạng thái bền vững (quest progress, inventory, vị trí cuối ở hub) lưu PostgreSQL.

## Hệ quả

(+) Kiểm soát tải và an toàn (nhóm theo trường, khối). (−) Cần logic gán instance và chuyển instance khi vào party.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-20 | Owner chốt (Accepted) | Owner |

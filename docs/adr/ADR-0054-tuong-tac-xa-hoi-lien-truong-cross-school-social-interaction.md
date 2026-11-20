# ADR-0054 — Tương tác xã hội liên trường (Cross-school Social Interaction)

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-09 · **Tài liệu:** [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md), [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md), [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Các quy tắc liên trường rải rác ở ADR-0020, ADR-0026, MATCHMAKING, PARTY_SYSTEM; cần một nguồn duy nhất (Master Prompt "Re-architecture" §13–14).

## Quyết định

| Hoạt động | Cùng trường | Khác trường, cùng tenant đơn vị | Khác tenant |
|---|---|---|---|
| Kết bạn, DM | Có | Khi tenant bật | Không |
| Party, guild/club | Có | Khi tenant bật | Không |
| World/Area chat | Có (World mặc định theo trường) | World mở rộng toàn tenant khi tenant bật | Không |
| Ghép trận 1v1/đội, chat `match` | Có (mặc định) | Khi tenant bật | Chỉ trong giải đấu/sự kiện liên tenant do platform tổ chức |
| Leaderboard | Có | Có | Chỉ bảng toàn platform, hiển thị nickname |

- Người ngoài trường chỉ thấy **nickname + avatar**, không thấy tên thật, trường, lớp.
- Quy tắc chat như nhau cho mọi lớp (ADR-0020); mọi tin vẫn qua kiểm duyệt theo ADR-0030.
- Hoạt động liên tenant chỉ tồn tại trong ngữ cảnh platform tạo (giải đấu, sự kiện); kết thúc thì kênh liên lạc đóng.

## Hệ quả

(+) Một bảng quy tắc duy nhất cho mọi tính năng xã hội. (−) Mỗi tính năng phải kiểm tra phạm vi trường/tenant nhất quán.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR, hợp nhất quy tắc rải rác | Audit theo Master Prompt mới |
| 2026-11-20 | Owner chốt (Accepted) | Owner |

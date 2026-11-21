# Research: F100 — Party: tạo/tham gia/mời/kick/chuyển leader/giải tán, một party active, giới hạn

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Đường đẩy sự kiện party tới client

- **Decision**: module competition publish mỗi message party (`party.member.joined`, `party.member.left`, `party.leader.transferred`, `party.state.snapshot`, `party.party.disbanded`, …) tới `ew:rt:user:<member_id>` của từng thành viên active (≤ 8 lần `PUBLISH`); lời mời publish tới `ew:rt:user:<invitee_id>` [Confirmed].
- **Rationale**: gateway chỉ `SUBSCRIBE` topic `ew:rt:party:<party_id>` khi module trả `subscribe` trong reply một lệnh WS ([REALTIME_ARCHITECTURE §4.1, §2.1](../../docs/06-architecture/REALTIME_ARCHITECTURE.md)); tạo/accept/join đi qua REST nên không có reply WS để subscribe, và khi bị kick không có cơ chế gỡ subscribe → người bị kick vẫn nhận sự kiện party. Fan-out theo user không cần trạng thái subscription, không lộ dữ liệu sau khi rời, chi phí nhỏ (party ≤ 8).
- **Alternatives**: (a) topic party + lệnh WS mới `party.state.subscribe` + gateway tự gỡ khi thấy `party.member.left` của chính user — đưa logic nghiệp vụ vào gateway (trái REALTIME §1); (b) chỉ dùng REST polling — không đạt ≤ 1 s (AC-PTY-003).
- **Hệ quả**: cập nhật REALTIME_ARCHITECTURE §5 và VALKEY_DESIGN §3.2 (topic `ew:rt:party:<party_id>` không dùng trong Release 1.0) — task Polish.

## R2. Kiểm duyệt tên party

- **Decision**: command `CreateParty`/`UpdatePartySettings` gọi `moderateDisplayText` của facade `chat` (trong tiến trình qua `src/backend/pkg/modcall`, deadline của request) khi `name` khác tên mặc định. Kết quả `blocked` → `422 VALIDATION_FAILED` (`errors[].field = name`, `reason = content_not_allowed`), không trả từ đã khớp (R15). Hết deadline hoặc facade trả lỗi `UNAVAILABLE` → lưu tên mặc định "Nhóm của {nickname}" (khóa dịch phía client, server lưu chuỗi tiếng Việt cắt ≤ 40 ký tự) và trả `201`.
- **Rationale**: MODERATION §1 yêu cầu tên party qua pipeline; pipeline thuộc module chat (R13 không gọi chéo DB). Fallback an toàn: không để tên chưa kiểm duyệt hiển thị, không chặn trải nghiệm khi module chat lỗi.
- **Alternatives**: (a) chỉ cho chọn tên từ danh sách sinh sẵn như nickname — trái PARTY_SYSTEM §3 (tên tự đặt [Confirmed]); (b) kiểm duyệt bất đồng bộ sau khi lưu — tên vi phạm hiển thị trong khoảng trễ.
- **Contract**: operation facade `moderateDisplayText` (`x-allowed-callers: [competition]`) — Cần thêm vào `chat.internal.v1.yaml`, F116 hiện thực.

## R3. Dữ liệu bạn bè, block, sanction, khối (trước ADR-0095: lớp)

- **Decision**: projection trong `eduworld_competition`: `ref.friendships` (từ `friend.added`/`friend.removed`), `ref.blocks` (`user.blocked`/`user.unblocked`), `ref.sanctions` (`sanction.applied`/`sanction.lifted`, chỉ loại ảnh hưởng multiplayer), `ref.enrollments` (`enrollment.changed`) bên cạnh `ref.students`, `ref.player_profiles`, `ref.tenants` đã có trong DATABASE_DESIGN §2.17. **Cập nhật ADR-0095 (2026-11-21):** không còn `ref.enrollments`; trường/khối lấy từ `ref.students` (`student.synced`, `student.placement_changed`).
- **Rationale**: policy party kiểm tra trong transaction, không gọi đồng bộ module chat/identity (R29, đường ghi nhẹ). `ref.blocks` đã được nhắc ở DATABASE_DESIGN §2.10 (duels).
- **Alternatives**: gọi module chat internal mỗi lần mời — thêm phụ thuộc đồng bộ, tăng độ trễ, không có endpoint.
- **Hệ quả**: bổ sung DATABASE_DESIGN §2.17 và consumer trong EVENT_DESIGN §3 (competition tiêu thụ `friend.*`, `sanction.*`, `enrollment.changed`) — task Polish.

## R4. Tuần tự hóa membership

- **Decision**: mọi command thay đổi membership chạy trong transaction: `SELECT … FROM multiplayer.parties WHERE party_id = $1 FOR UPDATE`, đếm thành viên active, insert/update `party_members`; unique partial index `party_members_one_active_party_uq` và `party_members_one_leader_uq` là chốt cuối; vi phạm unique → map `PARTY_ALREADY_IN_PARTY`. Sau commit mới cập nhật Valkey (script Lua của VALKEY_DESIGN §4.3) và publish.
- **Rationale**: theo DATABASE_DESIGN §3.7 và VALKEY_DESIGN §4.3 (PG là nguồn sự thật).
- **Alternatives**: khóa Valkey trước — không bền khi Valkey lỗi.

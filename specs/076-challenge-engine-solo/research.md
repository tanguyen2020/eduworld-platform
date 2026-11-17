# Research: F076 — Challenge Engine solo, 5 mức khó, room host dùng chung

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Challenge solo có cần room host không?

- **Decision**: Có, ở mức tối giản: mỗi `ChallengeAttempt` `in_progress` là một room một thành viên trong `src/backend/pkg/roomhost` để có timer hạn chót theo giờ server, đẩy `challenge.attempt.updated` qua Valkey và khôi phục khi đổi node; PostgreSQL là nguồn sự thật (`deadline_at`, điểm), job `challenge-deadline` là dự phòng.
- **Rationale**: CODE_PLAN §4 giao "room host dùng chung" cho F076; boss (F103) và battle (F105) cần cùng cơ chế (CHALLENGE_BOSS_SYSTEM §4, REALTIME_ARCHITECTURE). Dựng ở feature có ít người chơi nhất giúp kiểm thử lease/checkpoint trước khi có party.
- **Alternatives**: (a) Chỉ job quét `deadline_at` mỗi N giây — đơn giản nhưng sai số lớn, không dùng lại cho boss/battle; (b) đặt room host trong `realtime` — trái ADR-0074 (module `realtime` không có nghiệp vụ, không DB).

## R2. Ranh giới `src/backend/pkg/roomhost`

- **Decision**: Gói gồm `Host` (đăng ký room type), `Lease` (Valkey `ew:rt:room:<room_id>:lease`, SET NX PX, gia hạn), `Timer` (theo `src/backend/pkg/clock`), `CheckpointStore` (interface, service hiện thực), `Grace` (theo loại room), `Publisher` (Valkey `ew:rt:room:<room_id>:out` / `ew:rt:user:<id>`). Không biết challenge, boss hay battle.
- **Rationale**: R13 cho phép `src/backend/pkg/` dùng chung nếu không chứa nghiệp vụ; mỗi service tự hiện thực `RoomHandler` (OnTick, OnTimer, OnRestore).
- **Alternatives**: Thư viện actor bên ngoài — thêm phụ thuộc, khó kiểm soát lease với Valkey ACL theo service.

## R3. Cập nhật tiến độ từ kết quả chấm

- **Decision**: Tiêu thụ `activity.graded` (consumer `game.challenge_progress`), tra challenge attempt theo `attempt_id` (index GIN `learning_attempt_ids`), áp luật theo `challenge_type` trong domain.
- **Rationale**: Chấm điểm thuộc module learning (ADR-0016); event đã có schema; không cần gọi đồng bộ sau mỗi câu (R29). `getAttemptResult` chỉ dùng đối soát khi nghi lệch.
- **Alternatives**: Godot báo kết quả cho module game — vi phạm R5.

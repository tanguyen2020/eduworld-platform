# Data Model: F062 — Main Hub greybox, character controller, `QualityManager`, tab "Nhà trường giao"

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F062 là game client, không tạo/sửa bảng PostgreSQL. Dữ liệu bài giao chỉ giữ trong RAM (`AppState.assignments`) suốt phiên; không cache dữ liệu tenant xuống đĩa ([GODOT_ARCHITECTURE §8.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md)).

Mô hình phía client (tham khảo cho code, không phải bảng):

| Model (GDScript) | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| `MyAssignmentModel` | `MyAssignment` (OpenAPI) | `assignment_id`, `title`, `subject_id`, `content_kind`, `content_ref`, `exam_session_id`*, `open_at`, `due_at`, `status`, `attempt_id`, `attempts_left`, `result_release`*, `late_policy`* | RAM (`AppState.assignments`) |
| `PlayTimePolicyModel` | `PlayerBootstrap.policies.play_time` | `break_reminder_minutes`, `access_window` | RAM |
| `QualityProfile` | benchmark/`client-config.device_profiles` | `renderer`, `preset`, `render_scale`, `fps_cap` | `user://settings.cfg` mục `[quality]` (thiết bị, không gửi server) |

\* Trường cần thêm vào contract — xem [contracts/README.md](contracts/README.md) §6.

## 2. Vòng đời / trạng thái

`AssignmentService`: `idle → loading → ready | error` (giữ dữ liệu cũ khi `error`). Trạng thái bài (`status`) do server tính; client không suy ra.

## 3. Quy tắc kiểm tra dữ liệu

- Parse an toàn: trường lạ bỏ qua, `status`/`content_kind` lạ → hiển thị "không hỗ trợ" (F062-AC-10).
- `title` hiển thị nguyên văn, cắt bằng ellipsis + tooltip (NFR-I18N-003).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — (client không phát domain event) | | |

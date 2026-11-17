# Research: F066 — Bài nghe và mô phỏng: plugin `audio_lesson`, `simulation` + checker Go, template Godot

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Đếm lượt nghe (`max_plays`)

- **Decision**: Thêm `play_started: boolean` vào `ActivityProgressReport` và `plays_used`, `max_plays` vào `ActivityProgressState`; lưu `plays_used smallint` trong `assessment.video_progress` (bảng tiến độ media dùng chung). Một lượt = phát từ vị trí 0 hoặc phát lại sau khi tới cuối; tạm dừng/tiếp tục/tua trong lượt không tính. Server tăng `plays_used` trong cùng transaction với cập nhật tiến độ, theo `client_seq` (idempotent). Vượt lượt → `422 AUDIO_PLAY_LIMIT_REACHED`, trạng thái không đổi.
- **Rationale**: Dùng lại endpoint tiến độ mà STUDENT_GAME_UX_LEARNING §6.3 đã nêu ("`POST …/progress` (lượt nghe)"); không thêm endpoint; số lượt server ghi là nguồn sự thật cho báo cáo.
- **Alternatives**: (a) Chỉ client đếm — không có dữ liệu tin cậy, mất khi resume trên thiết bị khác. (b) Endpoint riêng `…/plays` — thêm bề mặt API không cần. (c) Cấp signed URL một lần mỗi lượt — media đã cache theo `sha256` (ADR-0044), không ngăn được phát lại cục bộ, tăng tải.

## R2. Danh mục template và checker

- **Decision**: Mỗi template có descriptor trong mã nguồn module learning `internal/domain/activity/simulation/templates/<template_code>/v<n>/descriptor.json` (`template_code`, `template_version`, `simulation_kind`, `title`, `parameter_schema`, `actions[]` {`action`, `args_schema`}, `metrics[]`, `checker_available`) và checker Go đăng ký vào `CheckerRegistry`. Job khởi động `simtemplates.Sync` upsert `content.simulation_templates` (`status = active`; descriptor bị gỡ → `retired`, không xóa). Nếu có nội dung published dùng template mà không có checker cho version đó trong khi descriptor khai báo `checker_available = true` → service không ready (như Registry, ACTIVITY_ENGINE §5).
- **Rationale**: "platform đăng ký khi phát hành template" (SIMULATION_ENGINE §7) mà không có API ghi; checker và schema tham số phát hành cùng mã nên luôn khớp; không có đường nạp code từ CMS (ADR-0052).
- **Alternatives**: (a) API `/game-admin/simulation-templates` — mở bề mặt ghi platform không có trong tài liệu. (b) Seed SQL tay — dễ lệch với checker.

## R3. Không gửi điều kiện kiểm chứng xuống client

- **Decision**: Render payload chỉ có `goals[] {goal_id, description}`; `check` (`metric`, `operator`, `value`, `tolerance`) ở lại server. `verified_outcome` trả sau run chỉ gồm metric được template đánh dấu `display: true` và kết quả đạt/không theo mục tiêu.
- **Rationale**: ACTIVITY_ENGINE §5 cấm "tham số kiểm chứng" trong render payload; với mục tiêu dạng đo (ví dụ nhiệt độ sôi) `value` chính là đáp án (R6).
- **Alternatives**: Gửi đầy đủ `check` để client tự tick mục tiêu — vi phạm R6 và AC-UX-210 ("Đạt mục tiêu" chỉ sau khi server xác nhận).

## R4. Mô hình toán dùng chung và vector kiểm thử

- **Decision**: Template `physics.lever` v1: đòn bẩy tĩnh, mô-men `Σ m·g·d` mỗi bên, `g` cố định trong descriptor, vị trí đặt vật là ô rời rạc theo `parameters` (`arm_length_m`, `slots_per_side`, `masses_kg[]`). Hành động: `place_mass {mass_index, side, slot}`, `remove_mass {mass_index}`, `release {}`. Metric: `torque_left`, `torque_right`, `torque_diff`, `balanced`. Checker và scene GDScript cùng hiện thực công thức số học xác định (không dùng physics engine Godot cho phần chấm). Vector chung đặt ở `specs/examples/simulation/physics.lever/v1/vectors.json`, đọc bởi test Go và GdUnit4.
- **Rationale**: SIMULATION_ENGINE §5 yêu cầu cùng mô hình đơn giản hóa và vector chung; đặt dưới `specs/examples` để là một nguồn duy nhất cho cả hai phía.
- **Alternatives**: Vector trong từng repo con — có thể lệch; dùng physics engine — không xác định giữa nền tảng.

## R5. Seed

- **Decision**: `seed_policy = fixed` → seed = 64 bit đầu của SHA-256(`activity_version_id`); `per_attempt` → sinh khi start; cả hai trường hợp ghi vào `assessment.attempts.seed` (cột có sẵn, DATABASE_DESIGN §2.6), không thêm cột. Run gửi seed khác → `SIMULATION_LOG_INVALID`.
- **Rationale**: Tái lập được, không cần bảng mới.
- **Alternatives**: Seed do client chọn — cho phép dò seed thuận lợi.

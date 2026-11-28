# Data Model: F136 — Nickname tự gõ qua kiểm duyệt (giữ "Gợi ý"), báo cáo và đặt lại nickname

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

| Bảng | Trạng thái | Cột (thay đổi) | Module · database |
|---|---|---|---|
| `game.game_profiles` | Có sẵn (F070) — **Thêm cột** | `nickname_key varchar(32)` NULL (chữ thường + NFC + khoảng trắng đã gộp của `nickname`; NULL khi `nickname` NULL); UNIQUE partial `(tenant_id, nickname_key) WHERE deleted_at IS NULL AND nickname_key IS NOT NULL`; index cũ `(tenant_id, nickname)` giữ nguyên | game · `eduworld_game` |
| `game.nickname_changes` | Có sẵn (F070, append-only TA) — không thêm cột | `reason` giữ `initial`/`player_change`/`reset`; `changed_by` = học sinh, nhân sự trường hoặc `NULL` (hệ thống theo sanction); thêm hàm `game.anonymize_nickname_changes(p_tenant_id uuid, p_student_id uuid, p_value varchar(32))` `SECURITY DEFINER` (viết tay, role chỉ của facade ẩn danh) — cách duy nhất UPDATE bảng này | game · `eduworld_game` |
| `chat.sanctions` | Có sẵn (F116, F121) — **Thêm giá trị** | `type` thêm `nickname_reset` (tức thời: `ends_at = starts_at`, `scope = all`) | chat · `eduworld_chat` |
| `chat.reports` | Có sẵn (F121) — không thêm cột | `context jsonb` nhận thêm khóa `reported_field` (`nickname`); bản chụp nickname (≤ 32) lưu trong `moderation_evidence` của case | chat · `eduworld_chat` |

Không có bảng mới. Bộ đếm lần thử: Valkey `ew:game:nick:attempts:<tenant_id>:<student_id>` (cửa sổ trượt `nickname_attempt_rate`, TTL bằng cửa sổ) — bảo vệ phụ, không phải dữ liệu quan trọng (R9).

Client Godot (RAM, không lưu đĩa): `NicknameEditorState` (`text`, `mode` = `chip`/`typed`, `selected_option_id`, `precheck` = `ok`/`too_short`/`too_long`/`chars`/`digits`, `server_error`, `retry_after`, `changes_left`).

## 2. Vòng đời / trạng thái

- `GameProfile.nickname`: `NULL` (mới tạo, `setup_required`) → đặt lần đầu (`initial`) → đổi (`player_change`, tính giới hạn tháng) → đặt lại (`reset`, về `NULL`, `setup_required`) → đặt lại lần đầu sau reset (không tính giới hạn) → … ; soft-delete/khôi phục cùng GameProfile (F016); ẩn danh hóa → giá trị ẩn danh F070.
- Lần gửi nickname tự gõ: `received` → `format_rejected` | `limit_reached` | `rate_limited` | `moderation_blocked` | `personal_blocked` | `unavailable` | `taken` | `saved`. Chỉ `saved` ghi DB.
- Sanction `nickname_reset`: `applied` (tức thời, không có `lifted`); consumer game xử lý một lần theo `sanction_id`.

## 3. Quy tắc kiểm tra dữ liệu

- Chuẩn hóa: NFC → trim → gộp khoảng trắng; `nickname_key` = chữ thường của kết quả. Tập ký tự: `[A-Za-z]`, chữ Latin có dấu dựng sẵn dùng trong tiếng Việt (gồm `đ`, `Đ`), `[0-9]`, khoảng trắng đơn; ≥ 1 chữ cái; ≤ `nickname_max_digits` chữ số; `nickname_min_length` ≤ độ dài (điểm mã) ≤ 32.
- `nickname` và `nickname_key`: ent `MaxLen(32)` (`varchar(32)`), OpenAPI/AsyncAPI `maxLength: 32` (R28). `reason` của `resetStudentNickname` `maxLength: 500`.
- Kiểm duyệt và dữ liệu cá nhân chạy trên chuỗi đã chuẩn hóa; quyết định chỉ `allowed`/`blocked`.
- Không lưu chuỗi bị từ chối ở đâu (DB, log, audit, Valkey); bộ đếm chỉ lưu số.

## 4. Event / outbox

| Event | Vai trò | Ghi chú |
|---|---|---|
| `game_profile.updated` | game phát (owner F070) khi đặt/đổi/đặt lại nickname | Schema owner F070; không đổi cấu trúc (`nickname` có thể `null` sau đặt lại — đã cho phép) |
| `sanction.applied` | chat phát (owner F116) loại `nickname_reset`; game tiêu thụ | Schema owner F116: thêm giá trị `nickname_reset` vào enum `sanction_type` |
| (tiêu thụ) `sanction.applied` | game tiêu thụ — consumer `game.sanction_nickname_reset` (owner F116) | Bỏ qua mọi loại khác `nickname_reset`; idempotent `processed_events` |

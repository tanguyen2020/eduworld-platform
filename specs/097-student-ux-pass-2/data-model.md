# Data Model: F097 — Student UX pass 2: màn làm bài như một trò chơi, tạo nhân vật bằng hình, marker không che nhân vật

> Feature chỉ có dữ liệu hiển thị phía client, token/icon và một cài đặt project; không bảng, không event, không dữ liệu học sinh mới, không đổi payload hoạt động.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| `AnswerBadge` | scene component | `number` (phím 1–6 / thứ tự), `palette_index` (0–5), `is_selected()` theo Button cha | RAM |
| `ProgressTrail` | scene component | `current` (1-based), `total`, `mark_of(i)` → `done`/`current`/`todo`, `uses_bar()` (> 15) | RAM (từ `renderer.progress()`) |
| `AvatarThumb` | scene component | `skin`, `hair`, `outfit` từ `AvatarConfigModel` của mẫu (`ItemVisual.appearance_color`, tint item), `is_selected()` | RAM |
| `ColorDot` | scene component | `color` (token `avatar.*`), `is_selected()` | RAM |
| `CloudyMascot` | scene component | + `compact`, `side_tail`, `mood` (`NEUTRAL`/`HAPPY`/`ENCOURAGE`) | RAM |
| `MarkerFader` | `scripts/world/marker_fader.gd` | `near_m` (ProjectSettings `eduworld/world/marker_near_fade_m`, mặc định 5,0), `HYSTERESIS_M` 0,5, `hidden[]`, `should_hide(d, d_player)` | RAM |
| Token mới | `design/game-tokens` (`tokens@0.14.0`) | `density.<band>.font.stem`, `density.<band>.answer.min_height`, `component.{answer_tile,question_card,drop_zone,feedback,progress_trail,activity_board,coach}.*` | sinh Theme + `UiTokens` |
| Icon mới | `design/icons/game/*.svg` | `verdict_correct`, `verdict_try`, `exit`, `next_light`, `arrow_{up,down,left,right}`, `rotate_{left,right}` | → `src/game/resources/ui/icons/` |
| Khóa dịch | `locale/{vi,en}.po` | mới: `ui.activity_player.coach_cheer`, `ui.activity_player.coach_cheer_hint`; đổi giá trị: `ui.feedback.correct`, `ui.feedback.incorrect` | `.po` |

## 2. Vòng đời / trạng thái

- Thẻ đáp án: thường → (chạm/phím) đã chọn (nền nhấn + ✓) → (Kiểm tra, server chấm) khóa: giữ dáng, không nhận input → câu sau.
- Mây: `NEUTRAL` (câu mới) → `HAPPY` (đúng) / `ENCOURAGE` (chưa đúng) → `NEUTRAL` (Tiếp).
- Marker: hiện → (gần camera < 5 m và gần hơn nhân vật) ẩn → (xa hơn 5,5 m hoặc xa hơn nhân vật) hiện.
- Xem trước S02: tự quay → (học sinh xoay) dừng 3 s → tự quay; "Giảm chuyển động" = đứng yên.

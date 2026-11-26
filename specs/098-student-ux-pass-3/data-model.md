# Data Model: F098 — Student UX pass 3: chạm là trả lời, ma trận kiểm tra 4 khối

> Feature chỉ có dữ liệu hiển thị phía client, token và một cài đặt project; không bảng, không event, không dữ liệu học sinh mới, không đổi payload hoạt động.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| `InstantAnswerPolicy` | `scripts/learning/instant_answer_policy.gd` | `INSTANT_TYPES` (`single_choice`, `true_false`), `BLOCKED_MODES` (`exam`, `diagnostic`, `competition`), `SETTING` `eduworld/learning/tap_confirm_bands`; `mode_allows(attempt)`, `item_allows(item)`, `applies(attempt, item)`, `double_tap(band)` | Không (hàm thuần + ProjectSettings) |
| `ResponseResultModel.correct_response` | `ActivityResponseResult.correct_response` (contract có sẵn) | `QuestionResponse` của câu (`{option_id}`, `{value}`) hoặc `{}` | RAM, không log/không đĩa |
| `QuestionRendererBase` | scene base | + `instant_answer`, `double_tap`, `grading`, tín hiệu `choice_tapped(repeated)`; `set_instant_answer()`, `set_grading()`, `show_verdict()` | RAM |
| `AnswerBadge.verdict` | scene component | `NONE`, `PENDING` (ba chấm), `CORRECT` (✓), `TRY` (✗), `ANSWER` (✓ đáp án đúng) | RAM |
| `ActivityHost` | scene | + `_instant`, `_double_tap` theo câu | RAM |
| Token mới | `design/game-tokens/component.json` (`tokens@0.15.0`) | `component.answer_tile.{correct_bg,correct_border,try_bg,try_border,tag_correct_bg,tag_try_bg,tag_pending_bg,tag_pending_border,tag_text,tag_pending_text}` | sinh Theme + `UiTokens` |
| Cài đặt client | `project.godot` | `eduworld/learning/tap_confirm_bands` = `PackedStringArray()` | ProjectSettings |
| Khóa dịch | `locale/{vi,en}.po` | `ui.question.tap_answer`, `ui.question.tap_answer_twice`, `ui.question.grading`, `ui.question.tile_correct`, `ui.question.tile_try`, `ui.question.tile_answer`, `ui.activity_player.tap_again`, `ui.activity_player.resend` | `.po` |
| `BandLayoutProbe` | `tests/fakes/band_layout_probe.gd` (TEST-ONLY) | kiểm nút/chữ/thẻ/tràn khung theo khối | — |

## 2. Vòng đời / trạng thái

- Thẻ (chạm là trả lời): thường → (chạm/phím) **đang chấm** (mọi thẻ khóa, nhãn "Đang chấm…", huy hiệu ba chấm) → (kết quả có `is_correct`) **đúng** (✓ "Đúng rồi!") / **chưa đúng** (✗ "Chưa đúng, không sao!") [+ thẻ **đáp án đúng** nếu có `correct_response`] → câu sau. Lỗi 4xx: đang chấm → thường (giữ lựa chọn, "Gửi lại"). Mất mạng: đang chấm (giữ, hàng đợi) → kết quả khi có mạng.
- Hai chạm: thường → (chạm 1) đã chọn (✓) + Mây "Chạm lần nữa…" → (chạm lại cùng thẻ / phím / nút chính) đang chấm → …; chạm thẻ khác: đã chọn thẻ khác.
- Mây: `NEUTRAL` → `HAPPY` (đúng) / `ENCOURAGE` (chưa đúng) → `NEUTRAL` (Tiếp).

# Data Model: F099 — Student UX pass 4: màu trạng thái rõ ràng, bố cục cân, trình phát media thân thiện

> Feature chỉ có dữ liệu hiển thị phía client và token; không bảng, không event, không dữ liệu học sinh mới, không đổi payload hoạt động.

## 1. Entity

| Model / resource | Nguồn | Trường | Lưu ở |
|---|---|---|---|
| `AnswerBadge.Verdict.REST` | scene component | huy hiệu số trên đĩa trung tính (`rest_accent`) | RAM |
| `QuestionRendererBase` | scene base | + `fit_height(available, tall) -> float`, `_fit_tiles()`, `_fit_rows()`, `_apply_tall(tall)`; `show_verdict()` đặt thẻ nghỉ | RAM |
| `ActivityRendererBase` | base | + `fit_layout(available, tall) -> float` (-1 = không cân), `coach_hint() -> String`, tín hiệu `coach_hint_changed()` | RAM |
| `ActivityHost` | scene | + `TopSpacer`, `BottomSpacer`, `_balance()` | RAM |
| `MediaTrack` | `scenes/ui/components/media_track.gd` (`Range`) | `watched_ms`, `chapters: PackedInt32Array`, `markers: Array[{at_ms, done}]`; tín hiệu `seek_released(value)` | RAM |
| `VttParser.voice_at(ms)` | `scripts/learning/vtt_parser.gd` | tên người nói của cue (`<v …>`) | RAM |
| Token mới | `design/game-tokens/*.json` (`tokens@0.16.0`) | `palette.{cyan,plum,indigo}.{50,700}`, `number.ratio.{150,300}`; `component.answer_tile.{bg,accent}_1…6` (giá trị mới), `component.answer_tile.{rest_bg,rest_border,rest_accent,rest_text,grow_max,grow_max_tall}`, `component.caption.{bg,border,text}`, `component.media_track.{track,watched,played,marker,marker_done,marker_text,chapter}`, `component.media_stage.bg` | sinh Theme + `UiTokens` |
| Theme mới | `godot-tres.mjs` | `AnswerTileRest`, `CaptionCard`, `CaptionLabel`, `MediaStage` | `.tres` sinh |
| Icon mới | `design/icons/game/` | `play_light`, `pause_light`, `rewind`, `captions` | `resources/ui/icons/` sinh |
| Khóa dịch | `locale/{vi,en}.po` | `ui.media.*`, `ui.video.coach_*`, `ui.audio.coach_*`, `ui.simulation.coach_*`, `ui.simulation.{goals,steps,runs}` | `.po` |
| `BandLayoutProbe.empty_bands()` | `tests/fakes/band_layout_probe.gd` (TEST-ONLY) | dải trống dọc trong `ActivityBoard` | — |

## 2. Vòng đời / trạng thái

- Thẻ sau khi chấm (có `is_correct`): thẻ chọn → đúng (✓ "Đúng rồi!") / chưa đúng (✗ "Chưa đúng, không sao!"); thẻ `correct_response` → "Đáp án đúng" (✓); mọi thẻ khác → nghỉ (`AnswerTileRest`, số giữ). `is_correct` rỗng → không đổi.
- Bố cục: hiện câu / đổi cỡ / đổi chiều → `_balance()` → `fit_layout()` → (dọc) spacer.
- Nút chính video: `Phát` (chưa phát/dừng) ⇄ `Tạm dừng` (đang phát) → `Xem lại` (hết video). Bài nghe: `Nghe` → `Tạm dừng` → `Nghe lại` (lượt mới, xin server) / mờ khi hết lượt.
- Lời Mây media: theo trạng thái trình phát (spec Clarifications), đổi qua `coach_hint_changed`.

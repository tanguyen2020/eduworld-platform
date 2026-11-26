# Implementation Plan: F099 — Student UX pass 4: màu trạng thái rõ ràng, bố cục cân, trình phát media thân thiện

**Branch**: `099-student-ux-pass-4` · **Date**: 2026-11-26 · **Spec**: [spec.md](spec.md)

## Summary

Ba phần, chỉ phía client, trên khung làm bài F097/F098:

- **Màu trạng thái (US1)**: token `tokens@0.16.0` đổi bộ màu kẹo (xanh dương, tím, hồng, xanh biển, mận, chàm — primitive mới `palette.{cyan,plum,indigo}.{50,700}`), thêm `component.answer_tile.{rest_bg,rest_border,rest_accent}` + Theme `AnswerTileRest`. `AnswerBadge.Verdict.REST` vẽ đĩa trung tính có số. `QuestionRendererBase.show_verdict()` đặt `REST` cho mọi thẻ không phải thẻ chọn/thẻ đáp án đúng khi có `is_correct`. Test scene so màu StyleBox/huy hiệu của thẻ nghỉ với màu trạng thái; test unit đo khoảng cách sắc độ trên `UiTokens`.
- **Bố cục cân (US2)**: `QuestionRendererBase.fit_height(available, tall)` (thẻ cao lên trong trần `grow_max`/`grow_max_tall`, khối thẻ tham chiếu câu bốn lựa chọn — 2 hàng ngang, 4 hàng dọc; một cột khi dọc cho câu ≤ 4 lựa chọn và đúng/sai), `QuizActivityRenderer.fit_layout()` (căn giữa khối câu hỏi trong vùng cuộn), `ActivityHost.balance()` (dọc: `Body` cao bằng nội dung, hai spacer chia phần trống trên/dưới cụm câu hỏi + hàng Mây). `BandLayoutProbe.empty_bands()` đo dải trống dọc trong `ActivityBoard`; ma trận gọi cho màn một bước.
- **Media (US3)**: thành phần `MediaTrack` (Range tự vẽ: rãnh, đã xem, đã phát, vạch chương, dấu "?"/✓, kéo/chạm = tua qua luật cũ). Video/bài nghe: nút chính icon + chữ đổi theo trạng thái, nút "Lùi 10 giây", nút lớn giữa khung video khi dừng, `CaptionCard` + tên người nói (`VttParser.caption_at`), thẻ bài nghe (loa + tên đoạn). Mô phỏng: thẻ mục tiêu ✓/○, bước đánh số, nút icon + chữ, "Lượt thử: n/m". `ActivityRendererBase.coach_hint()` + `coach_hint_changed` → bong bóng Mây của host. Overlay câu hỏi: `ActivityBoard`; `show_result` gọi `show_verdict`. Icon mới `play_light`, `pause_light`, `rewind`, `captions`.
- **Chip lọc (P3)**: `ChipScroller` thay `ScrollContainer` trần trong `UiBuild.chip_scroller` — mũi tên hai bên chỉ khi hàng tràn.
- **Bằng chứng (US4)**: `tools/lib/screen_capture.gd` thêm `activity_video`, `activity_video_cue`, `activity_audio`, `activity_simulation` (playback giả TEST-ONLY); chụp trước (mã cũ) và sau, 4 khối.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2, static typing); Node 20 (pipeline token) |
| Primary Dependencies | GdUnit4; Style Dictionary 4 + fontkit (`design/`) |
| Storage | Không |
| Testing | GdUnit4 headless (unit, scene); `pnpm test` + `checks/run-all.mjs` (`design/`); capture có renderer |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | `STUDENT_GAME_UX_LEARNING` §2.1 P-5/P-6: thanh tiến độ vẽ lại khi vị trí đổi (≤ 1 lần/khung khi phát), bố cục tính lại khi đổi cỡ/câu, không mỗi khung |
| Constraints | AGENTS R1, R5, R6, R19, R32, R33, R34 · ADR-0016, ADR-0047, ADR-0103 |
| Scale/Scope | 6 bộ màu, 1 kiểu thẻ nghỉ, 3 renderer media, 4 màn chụp mới × 4 khối × 3 cửa sổ |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec + Clarifications [Confirmed] trước code; không contract mới; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client; lời gợi ý/bố cục ở renderer, luật lượt nghe/tua/lượt thử vẫn ở server (R1) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu mới, không log nội dung |
| IV Server-authoritative | ✅ | Thẻ "Đáp án đúng" chỉ từ `correct_response`; mục tiêu mô phỏng chỉ từ kết quả đã kiểm chứng (R5, R6) |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | Lời tử tế, không đếm ngược/so sánh (R19) |
| VII Một nguồn sự thật, cấu hình | ✅ | Trần cao thẻ ở token (`DESIGN_SYSTEM` §5.1); quy tắc hiển thị ở `STUDENT_GAME_UX_LEARNING` §4.1b, §5–§8 |
| VIII UI tách logic, token | ✅ | Màu/cỡ ở token; Theme sinh, không override |
| IX Test có bằng chứng | ✅ | Mỗi F099-AC có test hoặc bằng chứng công cụ (quickstart §5) |

## Project Structure

```text
design/game-tokens/{primitive,component}.json, design/sd/formats/godot-tres.mjs, design/icons/game/{play_light,pause_light,rewind,captions}.svg, design/package.json (0.16.0), design/CHANGELOG.md
src/game/resources/ui/theme/* + resources/ui/icons/* (sinh), src/game/scripts/core/ui_tokens.gd (sinh)
src/game/scenes/ui/components/{answer_badge,media_track,chip_scroller,ui_build}.gd
src/game/scripts/learning/{question_renderer_base,activity_renderer_base,vtt_parser}.gd
src/game/scenes/learning/{activity_host.gd,ActivityHost.tscn}, questions/{single_choice,multiple_choice,true_false}_question.gd
src/game/scenes/learning/activity_renderers/{quiz,video_lesson,audio_lesson,simulation}/*
src/game/locale/{vi,en}.po, messages.pot, xx_PS.po
src/game/tools/lib/screen_capture.gd
src/game/tests/{unit/learning/test_answer_palette.gd,scene/learning/test_answer_state_colours.gd,scene/learning/test_balanced_layout.gd,scene/video/test_media_player_ux.gd,scene/test_grade_band_matrix.gd,scene/test_chip_scroller.gd,fakes/band_layout_probe.gd} + test cập nhật
```

## Phase 0: Research

- Ảnh F098: `AnswerTile3` = `palette.green.700` trùng `correct_border`; `AnswerTile2` = `palette.orange.700` trùng `try_border` → không đủ đổi độ đậm, phải đổi sắc độ.
- `QuestionRendererBase.show_verdict()` chỉ đổi thẻ đã chọn và thẻ đáp án đúng; các thẻ khác giữ kiểu kẹo `AnswerTile<n>` (nguồn nhầm lẫn).
- `QuizRenderer` = `Scroll` (dọc) > `Content` (expand) > câu hỏi (expand). Thẻ đáp án không có `custom_minimum_size`; cao đến từ lề StyleBox. `Control.update_minimum_size()` vô hiệu cache lên trên đồng bộ → đặt lại min của thẻ rồi đọc `get_combined_minimum_size()` cho cao tự nhiên ngay.
- Cửa sổ 720×1280 → logic 1280×2276: chiều cao bảng ≈ 2200 px, nội dung một bước ≈ 600 px → cần cả thẻ cao lên (3×), một cột, và cụm Mây đi liền đáp án để không dải nào > 25%.
- `VttParser` bỏ thẻ `<v Người nói>`; cần giữ tên để hiện bằng chữ.
- Media renderer tự tạo `VideoPlayback`/`AudioPlayback` khi `playback == null`; ảnh chụp thay bằng playback giả sau khi host mở (TEST-ONLY, trong `tools/`).

## Phase 1: Design & Contracts

- Không contract API/event mới (contracts/README.md). Model hiển thị: data-model.md. Quy trình duyệt: quickstart.md.

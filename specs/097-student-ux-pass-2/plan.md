# Implementation Plan: F097 — Student UX pass 2: màn làm bài như một trò chơi, tạo nhân vật bằng hình, marker không che nhân vật

**Branch**: `097-student-ux-pass-2` · **Date**: 2026-11-26 · **Spec**: [spec.md](spec.md)

## Summary

Ba phần trên pipeline token/icon của F005/F096, chỉ phía client:

- **Màn học tập (US1, US2)**: token `tokens@0.14.0` — density `font.stem`, `answer.min_height`; component `answer_tile` (6 bộ màu), `question_card`, `drop_zone`, `feedback` (nền nhạt), `progress_trail`, `activity_board`, `coach`; Theme sinh `AnswerTile1…6` (viền đáy dày, lề trái cho huy hiệu — `styleBox` thêm `borderBottom`, `padL`, `padT`), `QuestionCard`, `StemLabel`, `InstructionLabel`, `DropZone`, `ActivityBoard`, `PictureTile`, `AnswerGrid/Box/Flow`. `QuestionRendererBase` dựng thẻ câu hỏi (`_add_stem`: `QuestionCard` + `RichContentView` biến thể `StemLabel` + nút "Đọc to" phát tín hiệu `read_aloud_requested`), thẻ đáp án (`_style_tile` + `AnswerBadge`), khóa thẻ không làm xám (`_lock_tile`). Mọi renderer có câu hỏi (quiz, video, audio, simulation) gọi `wire_question()` để chuyển tín hiệu đọc to lên host; `shows_stem()` báo host đề đã vẽ → `VoiceOverPlayer.stem_on_screen` (không phụ đề lặp, F096-3). `ActivityHost.tscn` làm lại: `SkyBackdrop` + bảng `ActivityBoard`, header (Thoát có icon, tiêu đề, `ProgressTrail`, "n/N"), hàng dưới (Mây `CloudyMascot` compact + bong bóng `SpeechBubble` chứa câu động viên, `FeedbackStrip`, `HintPanel`, lỗi; nút chính có icon). `FeedbackStrip` icon đĩa `verdict_correct`/`verdict_try`, "Xem ví dụ" cạnh giải thích. Chuỗi `ui.feedback.*` lời tử tế, `ui.activity_player.coach_cheer*`.
- **S02 (US3)**: `CharacterCreation.tscn` trên nền trời; `Stage` (`QuestionCard` + `SkyBackdrop` + SubViewport trong suốt, camera gần, fov 38); bàn xoay ở mọi bước, dừng 3 s sau thao tác; mẫu = `PictureTile` + `AvatarThumb`; màu da/tóc = `ColorDot` (dùng cả ở Tủ đồ); nút xoay/‹ ›/Tiếp/Xác nhận có icon.
- **Marker (US4)**: `MarkerFader` (ZoneRoot thêm), nhóm `camera_fade` trên `WorldAnchor/Marker` và `QuestMarker3D`, `ProjectSettings eduworld/world/marker_near_fade_m = 5.0`.
- **Nhật ký (polish)**: tab `ChoiceChip` + ✓, Đóng/Bắt đầu có icon.
- **Bằng chứng**: `tools/lib/screen_capture.gd` thêm 11 màn học tập, gắn Theme theo khối (như Boot); chạy từng màn một tiến trình.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2, static typing); Node 20 (pipeline token) |
| Primary Dependencies | GdUnit4; Style Dictionary 4 + fontkit (`design/`) |
| Storage | Không (một ProjectSettings `eduworld/world/marker_near_fade_m`) |
| Testing | GdUnit4 headless (unit, scene); `pnpm test` + `checks/run-all.mjs` (`design/`); capture có renderer |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | `STUDENT_GAME_UX_LEARNING` §2.1 P-5/P-6: không hiệu ứng nền động, vẽ tĩnh `_draw()`; `MarkerFader` kiểm 10 lần/giây, không mỗi frame |
| Constraints | AGENTS R1, R2, R5, R6, R19, R32, R33, R34 · ADR-0047, ADR-0103 |
| Scale/Scope | 11 dạng câu hỏi, khung activity player, S02, marker, Nhật ký; 13 AC |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec + bảng kiểm trước code; không contract mới; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client; renderer chỉ hiển thị, host gọi service như cũ |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu mới |
| IV Server-authoritative | ✅ | Kết quả chấm chỉ từ server; không đáp án ở client (R5, R6) |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | Lời sai tử tế, không đỏ gắt/từ phạt/đếm ngược/so sánh (R19, NFR-SAFE-003) |
| VII Một nguồn sự thật, cấu hình | ✅ | Cỡ/màu ở token; khoảng cách marker ở WORLD_RENDERING + CONFIG_DEFAULTS + ProjectSettings |
| VIII UI tách logic, token | ✅ | Không override Theme trong script/scene; file sinh không sửa tay; icon qua pipeline |
| IX Test có bằng chứng | ✅ | Mỗi F097-AC có test hoặc bằng chứng công cụ (quickstart §5) |

## Project Structure

```text
design/game-tokens/{primitive,component}.json, design/game-tokens/density/*.json, design/sd/formats/{godot-tres,gdscript-tokens}.mjs
design/icons/game/{verdict_correct,verdict_try,exit,next_light,arrow_up,arrow_down,arrow_left,arrow_right,rotate_left,rotate_right}.svg
design/package.json (0.14.0), design/CHANGELOG.md
src/game/resources/ui/{theme,icons}/* (sinh), src/game/scripts/core/ui_tokens.gd (sinh)
src/game/scenes/ui/components/{answer_badge,progress_trail,avatar_thumb,color_dot,cloudy_mascot}.gd
src/game/scripts/learning/{question_renderer_base,activity_renderer_base}.gd
src/game/scenes/learning/{ActivityHost.tscn,activity_host.gd,FeedbackStrip.tscn,feedback_strip.gd,HintPanel.tscn,hint_panel.gd,voice_over_player.gd,rich_content_view.gd}
src/game/scenes/learning/questions/*.tscn + *_question.gd, src/game/scenes/learning/activity_renderers/*/*.gd
src/game/scenes/character/{CharacterCreation.tscn,character_creation_screen.gd,customization_studio.gd}
src/game/scripts/world/{marker_fader.gd,zone_root.gd}, src/game/scenes/world/WorldAnchor.tscn, src/game/scenes/quest/{QuestMarker3D,QuestJournal,AssignmentRow}.tscn, quest_journal.gd
src/game/project.godot, src/game/locale/{vi,en}.po, messages.pot, xx_PS.po
src/game/tools/lib/screen_capture.gd
src/game/tests/{scene/learning/test_activity_player_ux.gd,scene/character/test_character_creation_ux.gd,unit/world/test_marker_fader.gd} + test cập nhật
```

## Phase 0: Research

- F096 bảng kiểm, ảnh `activity_feedback-after-*`: đề hai lần (VoiceCaption + đề), ô trắng chữ nhỏ, không tiến độ hình.
- Godot 4.7: `StyleBoxFlat` viền từng cạnh (viền đáy dày = cảm giác khối kẹo), `content_margin_*` riêng (lề trái cho huy hiệu, lề trên cho ảnh ô); Button tắt (`disabled`) luôn vẽ kiểu disabled kể cả khi đang chọn → khóa thẻ bằng `mouse_filter`/`focus_mode` để giữ dáng "đã chọn".
- Hub spawn: camera sau lưng ~6 m, đá Tia Sáng của `quest.hub.welcome_sign` (0, 0, 12) cách camera ~4,4 m, nhân vật ~6 m → quy tắc "gần camera hơn 5 m và gần hơn nhân vật".

## Phase 1: Design & Contracts

- Không contract API/event mới (contracts/README.md). Model hiển thị: data-model.md. Quy trình duyệt: quickstart.md.

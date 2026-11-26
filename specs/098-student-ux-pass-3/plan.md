# Implementation Plan: F098 — Student UX pass 3: chạm là trả lời, ma trận kiểm tra 4 khối

**Branch**: `098-student-ux-pass-3` · **Date**: 2026-11-26 · **Spec**: [spec.md](spec.md)

## Summary

Hai phần, chỉ phía client, trên khung làm bài F097:

- **Chạm là trả lời (US1, US2)**: helper `InstantAnswerPolicy` (`scripts/learning/`) quyết định từ `AttemptModel` (`feedback_timing`, `mode`, `result_release`), loại câu (`single_choice`, `true_false` một mệnh đề) và cài đặt client `eduworld/learning/tap_confirm_bands`. `QuestionRendererBase` thêm tín hiệu `choice_tapped(repeated)` (chạm/phím vào thẻ của câu một bước), `set_instant_answer(on, double_tap)` (đổi dòng hướng dẫn), `set_grading(on)` (khóa thẻ + nhãn "Đang chấm…"), `show_verdict(result)` (thẻ đúng/chưa đúng + "Đáp án đúng" từ `correct_response`). `QuizActivityRenderer` chuyển tiếp và trả `supports_instant_answer() = true`. `ActivityHost` tính `_instant`/`_double_tap` ở mỗi câu, gọi `press_primary()` khi `choice_tapped` hợp lệ (đồng bộ: trạng thái thành `AWAITING_RESULT` trước khi chạm kế tiếp tới), ẩn "Kiểm tra" khi chạm là trả lời, nút "Gửi lại" sau lỗi. `ResponseResultModel` đọc `correct_response` (chỉ hiển thị). `AnswerBadge` thêm `verdict` (ba chấm / ✓ / ✗). Token `tokens@0.15.0`: `component.answer_tile.{correct_*,try_*,tag_*}`, Theme `AnswerTileCorrect`, `AnswerTileTry`, `TileTagCorrect`, `TileTagTry`, `TileTagPending`. Chuỗi `ui.question.tap_answer`, `ui.question.tap_answer_twice`, `ui.question.grading`, `ui.question.tile_correct`, `ui.question.tile_try`, `ui.question.tile_answer`, `ui.activity_player.tap_again`, `ui.activity_player.resend`.
- **Ma trận khối (US3)**: `tests/scene/test_grade_band_matrix.gd` dựng mỗi màn trong một `SubViewport` cỡ logic của cửa sổ tham chiếu, gắn Theme của khối, rồi `BandLayoutProbe` (`tests/fakes/`) kiểm nút/chữ/thẻ/tràn khung. `tools/lib/screen_capture.gd` thêm màn `activity_tap_grading`, `activity_tap_correct`, `activity_tap_incorrect`, `activity_tap_confirm`; chụp 4 khối, xem ảnh, sửa lỗi.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.7.2, static typing); Node 20 (pipeline token) |
| Primary Dependencies | GdUnit4; Style Dictionary 4 + fontkit (`design/`) |
| Storage | Không (một ProjectSettings `eduworld/learning/tap_confirm_bands`) |
| Testing | GdUnit4 headless (unit, scene); `pnpm test` + `checks/run-all.mjs` (`design/`); capture có renderer |
| Target Platform | Web (Compatibility/WebGL 2), Android, iOS (ADR-0101) |
| Project Type | game client |
| Performance Goals | `STUDENT_GAME_UX_LEARNING` §2.1 P-5/P-6: nhãn/huy hiệu vẽ tĩnh, không animation mới |
| Constraints | AGENTS R1, R5, R6, R10, R14, R19, R32, R33, R34 · ADR-0016, ADR-0047, ADR-0103 |
| Scale/Scope | 2 dạng câu chạm-là-trả-lời, 1 cài đặt client, 13 nhóm màn × 4 khối × 3 cửa sổ |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec + Clarifications [Confirmed] trước code; không contract mới; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ client; quyết định ở helper `scripts/learning/`, scene chỉ nối tín hiệu (R1) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không dữ liệu mới; `correct_response` chỉ trong RAM, không log (R14) |
| IV Server-authoritative | ✅ | Chấm ở server; client chỉ hiện `is_correct`/`correct_response` server gửi (R5, R6); Idempotency-Key như cũ (R10) |
| V Xác thực, phân quyền | ✅ | Không endpoint mới |
| VI An toàn trẻ em | ✅ | Lời tử tế, cam ấm không đỏ, không từ phạt (R19) |
| VII Một nguồn sự thật, cấu hình | ✅ | Luật thời điểm ở ACTIVITY_ENGINE (không đổi); quy tắc tương tác ở STUDENT_GAME_UX_LEARNING §4.1b; `tap_confirm_bands` ở CONFIG_DEFAULTS + ProjectSettings |
| VIII UI tách logic, token | ✅ | Màu/cỡ ở token; Theme sinh, không override |
| IX Test có bằng chứng | ✅ | Mỗi F098-AC có test hoặc bằng chứng công cụ (quickstart §5) |

## Project Structure

```text
design/game-tokens/component.json, design/sd/formats/godot-tres.mjs, design/package.json (0.15.0), design/CHANGELOG.md
src/game/resources/ui/theme/* (sinh), src/game/scripts/core/ui_tokens.gd (sinh)
src/game/scripts/learning/{instant_answer_policy,question_renderer_base,activity_renderer_base}.gd
src/game/scripts/learning/models/response_result.gd
src/game/scenes/ui/components/answer_badge.gd
src/game/scenes/learning/{activity_host.gd}, questions/{single_choice,true_false}_question.gd, activity_renderers/quiz/quiz_renderer.gd
src/game/project.godot (eduworld/learning/tap_confirm_bands)
src/game/locale/{vi,en}.po, messages.pot, xx_PS.po
src/game/tools/lib/screen_capture.gd
src/game/tests/{unit/learning/test_instant_answer_policy.gd,scene/learning/test_tap_to_answer.gd,scene/test_grade_band_matrix.gd,fakes/band_layout_probe.gd} + test cập nhật
```

## Phase 0: Research

- Hôm nay (F097): `ActivityHost.press_primary()` ở `PRESENTING` gọi `renderer.check()` → `commit()` → `response_submitted` → `_on_response_submitted` đặt `AWAITING_RESULT` **trước** `await` đầu tiên → gọi đồng bộ từ tín hiệu chạm là an toàn (chạm thứ hai thấy trạng thái đã đổi).
- `LearningService.respond` khóa `Idempotency-Key` theo `respond|attempt|position|response` và giữ khóa tới khi server trả lời; lỗi tạm thời vào hàng đợi `PendingWrites` (gửi lại cùng khóa).
- `QuizActivityRenderer.show_index` phát `answer_changed` sau khi `restore()` câu đã lưu → không thể dùng `answer_changed` làm "học sinh vừa chạm" (sẽ tự gửi khi resume) → cần tín hiệu riêng `choice_tapped`.
- `ActivityResponseResult.correct_response` (OpenAPI learning) chỉ có khi FeedbackTiming = immediate và chính sách practice cho phép; `ResponseResultModel` chưa đọc trường này.
- Stretch `canvas_items` + `expand`, cơ sở 1280×720: cửa sổ 1024×600 → logic 1280×750; 720×1280 → logic 1280×2276. Test ma trận dựng màn trong `SubViewport` cỡ logic (Theme khối gắn vào control gốc vì SubViewport không kế thừa Theme của cửa sổ).

## Phase 1: Design & Contracts

- Không contract API/event mới (contracts/README.md); client đọc thêm trường có sẵn `correct_response`. Model hiển thị: data-model.md. Quy trình duyệt: quickstart.md.

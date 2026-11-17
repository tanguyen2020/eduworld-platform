# Implementation Plan: F063 — Activity player: renderer registry, renderer 11 loại câu hỏi, gợi ý, kết quả, công thức SVG

**Branch**: `063-activity-player` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

`LearningService` (`RefCounted`) bọc họ API `/learning/activity-attempts` (ADR-0059). `ActivityHost` (khung §3 của STUDENT_GAME_UX_LEARNING: header, vùng nội dung, dải phản hồi, action bar) nhận `Attempt`, hỏi `ActivityRendererRegistry` (autoload) để nạp renderer theo `activity_type`; F063 đăng ký `quiz` và `interactive_exercise`. Renderer dạng câu hỏi dùng `QuestionRendererRegistry` → 11 scene câu hỏi, mỗi scene dùng một answer model sinh `QuestionResponse` đúng schema. `HintPanel`, `FeedbackStrip`, `MathInline` + `MathSvgCache`, `VoiceOverPlayer` dùng chung. `PendingWrites` giữ câu trả lời trong RAM khi mất mạng. `ActivityResult` hiện kết quả học tập tách phần thưởng. Hợp đồng `ActivityRendererBase` được F065/F066/F073 tái dùng.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim), static typing (ADR-0014) |
| Primary Dependencies | GdUnit4; `ApiClient`, `Clock` (F061); `QualityManager`, `AppState`, `EventBus`, `SceneRouter` (F060/F062) |
| Storage | Không có dữ liệu nghiệp vụ bền; cache bất biến `user://cache/math/<sha256>.svg`, `user://cache/media/<sha256>/` (ADR-0044, ADR-0055) |
| Testing | GdUnit4 unit + scene runner; contract test parse fixture sinh từ example OpenAPI; integration headless với compose |
| Target Platform | Windows, Android (và Web/iOS theo ADR-0013) |
| Project Type | game client |
| Performance Goals | [PERFORMANCE_BUDGET §3, §7 (Activity player), §9, §11](../../docs/08-game-client/PERFORMANCE_BUDGET.md); P-1…P-6 [STUDENT_GAME_UX_LEARNING §2.1](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) |
| Constraints | AGENTS R1, R2, R5, R6, R10, R17, R32 · ADR-0016, ADR-0043, ADR-0048, ADR-0055, ADR-0056, ADR-0059 |
| Scale/Scope | ≤ 200 item/attempt (`Attempt.items maxItems`); 2 renderer activity + 11 renderer câu hỏi |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta `hint_count`, `activity_min_client_versions` ở Phase 2 (T004) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public; Service ← Gameplay ← UI |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` mọi request; render payload chỉ trong RAM; test F063-AC-11 |
| IV Server-authoritative | ✅ | Renderer không chấm/không giữ đáp án; mọi ghi có `Idempotency-Key`; hiển thị đúng/sai chỉ từ server |
| V Xác thực, phân quyền | ✅ | Token RAM (F061); `GAME_ACCESS_STUDENT_ONLY`/404 xử lý; không URL hardcode |
| VI An toàn trẻ em | ✅ | Không hiện dữ liệu học sinh khác, không so sánh, không thông điệp trừ điểm; log không chứa câu trả lời (R14) |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên; số lượt thử/resume do server |
| VIII UI tách logic, token | ✅ | Logic ở `LearningService`/answer model; Theme sinh từ token |
| IX Test có bằng chứng | ✅ | Mỗi F063-AC có task test; mock Prism có nhãn |

## Project Structure

### Documentation (feature này)

```text
specs/063-activity-player/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
godot/
├── scenes/learning/
│   ├── ActivityHost.tscn (+ activity_host.gd)        # khung chung: header, content, FeedbackStrip, ActionBar, ExitConfirm
│   ├── activity_renderers/quiz/QuizRenderer.tscn
│   ├── activity_renderers/interactive_exercise/InteractiveExerciseRenderer.tscn
│   ├── questions/{SingleChoice,MultipleChoice,TrueFalse,FillInBlank,ShortAnswer,Numeric,Matching,Ordering,DragDrop,ImageBased,Interactive}.tscn
│   ├── HintPanel.tscn, FeedbackStrip.tscn, VoiceOverPlayer.tscn, WorkedExampleOverlay.tscn, UpdateRequired.tscn
│   ├── math/MathInline.tscn
│   └── ActivityResult.tscn (+ activity_result.gd)
├── scripts/learning/
│   ├── learning_service.gd                  # start/get/respond/hint/pause/submit/review
│   ├── activity_renderer_registry.gd        # autoload ActivityRendererRegistry
│   ├── activity_renderer_base.gd            # class_name ActivityRendererBase (hợp đồng chung)
│   ├── question_renderer_registry.gd
│   ├── question_renderer_base.gd            # class_name QuestionRendererBase
│   ├── answer_models/{single_choice,multiple_choice,true_false,fill_in_blank,short_answer,numeric,matching,ordering,drag_drop,image_based,interactive}_answer.gd
│   ├── math_svg_cache.gd
│   └── models/{attempt.gd,question_render.gd,response_result.gd,attempt_review.gd}
├── scripts/networking/pending_writes.gd     # hàng đợi ghi RAM
├── locale/{vi.po,en.po}                     # ui.activity_player.*, ui.question.*, ui.feedback.*, ui.result.*, ui.learning.no_translation
└── tests/{unit,scene,contract,integration}/learning/, tests/fixtures/learning/attempt_*.json
```

**Structure Decision**: Theo [GODOT_ARCHITECTURE §3, §6.2, §6.3](../../docs/08-game-client/GODOT_ARCHITECTURE.md); hợp đồng renderer mô phỏng `MiniGameBase` ([MINIGAME_ENGINE §4.1](../../docs/08-game-client/MINIGAME_ENGINE.md)) để F065/F066/F073 cắm vào.

## Phase 0: Research

Không có — ADR-0048/0055/0059 và ACTIVITY_ENGINE đủ. Hợp đồng renderer chốt tại đây:

```gdscript
class_name ActivityRendererBase extends Control
signal response_ready(submission: Dictionary)   # ActivityResponseSubmission (kind, position|cue_id…, response, client_elapsed_ms)
signal progress_ready(report: Dictionary)       # ActivityProgressReport (video/audio)
signal hint_requested(position: int)
signal finished_requested
signal quit_requested
func setup(attempt: AttemptModel, host: ActivityHostApi) -> void: pass
func show_result(result: ResponseResultModel) -> void: pass
func show_hint(hint: HintResultModel) -> void: pass
func teardown() -> void: pass
```

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| Service | `LearningService.start(activity_id, ctx)`, `resume(attempt_id)`, `respond(sub)`, `request_next_hint(attempt_id, position)`, `pause()`, `submit()`, `review()`; `MathSvgCache.get_texture(asset, scale, color)`; `PendingWrites` | Signal `attempt_loaded`, `response_result_received`, `hint_received`, `attempt_submitted`, `attempt_failed(problem)` |
| Gameplay | — (player toàn màn 2D; gọi `QualityManager.suspend_world`) | |
| UI | `ActivityHost`, renderer, câu hỏi, `HintPanel`, `FeedbackStrip`, `ActivityResult`, `UpdateRequired` | Chỉ gọi method service |

### Tích hợp và phụ thuộc

- F030 (attempt API), F032 (hints, recommendations), F031 (review/mastery), F027 (media/SVG CDN, signed URL), F062 (intent `activity_host`, `suspend_world`), F061 (`ApiClient`, `Clock`).
- Feature dùng lại hợp đồng: F064 (renderer câu hỏi ở chế độ exam, không phản hồi), F065, F066, F073.

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key` UUIDv7 cho `startActivityAttempt`, `submitActivityResponse`, `revealNextHint`, `submitActivityAttempt`; giữ nguyên khi retry; không tự retry ghi không có key.
- Audit ghi phía server (F030); client không log body câu trả lời.

### Quan sát

- Telemetry: `activity_update_required{activity_type}`, `math_svg_failed`, `activity_open_ms`; không PII, không nội dung câu trả lời (R14).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |

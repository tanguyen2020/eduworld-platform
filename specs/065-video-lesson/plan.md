# Implementation Plan: F065 — Bài học video: plugin `video_lesson` và renderer Godot

**Branch**: `065-video-lesson` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Backend: thêm plugin `video_lesson` vào Activity Registry của module learning (ADR-0048), hiện thực `activity.Plugin` (ACTIVITY_ENGINE §5): `ValidateConfig`, `BuildRenderPayload`, `Respond` (`cue_answer`, `branch_choice`), `Progress` (gộp khoảng xem, kiểm hợp lý, chính sách tua), `Evaluate` (completion rule), `CompletionRule`, `EmitEvidence`. Logic thuần (gộp khoảng, hợp lý, hoàn thành) ở tầng domain; lưu `assessment.video_progress`, `assessment.video_branch_choices` bằng ent + Atlas; câu chèn là item của attempt (research R1). Godot: renderer `VideoLessonRenderer` dựa trên hợp đồng `ActivityRendererBase` của F063 với `VideoStreamPlayer` (Theora), `VttParser` + `SubtitleLayer`, `CueOverlay` (dùng renderer câu hỏi F063), `VideoProgressReporter` (heartbeat), `WatchedRanges` (khóa tua).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 (module learning) / GDScript (Godot 4.x ghim, static typing) |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `pkg/entx`, `pkg/outbox`, `pkg/tenancy`, `pkg/problem` / GdUnit4, `VideoStreamPlayer` (Theora) |
| Storage | PostgreSQL `eduworld_learning` (schema `assessment`); floci S3 cho media (F027) |
| Testing | go test (unit domain/application), testcontainers (integration, global filter), contract test plugin chung (TC-ACT-001) / GdUnit4 unit + scene + integration headless |
| Target Platform | Linux container / Windows, Android (Web 480p) |
| Project Type | backend module + game client |
| Performance Goals | Heartbeat p95 theo NFR API chung; client 720p@30 tầm trung ([STUDENT_GAME_UX_LEARNING §5.5](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md)) |
| Constraints | AGENTS R5, R6, R8, R10, R13, R21, R26–R28, R31 · ADR-0016, ADR-0037–0044, ADR-0074, ADR-0048, ADR-0056, ADR-0057 |
| Scale/Scope | ≤ 30 câu chèn, ≤ 50 chương, ≤ 20 đoạn, ≤ 64 khoảng xem/attempt (`video-lesson.schema.json`) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta schema render, `media[].variant`, `watched_ranges` ở Phase 2 (T004–T005) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Plugin trong module learning; domain thuần; command `ReportProgress`/`SubmitResponse`/`SubmitAttempt` là command handler (write context); media qua port F027 cùng module |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới có `tenant_id`, `deleted_at`, RLS; ent global filter; test F065-AC-13; không hard-delete |
| IV Server-authoritative | ✅ | Hoàn thành/điểm tính ở server; payload không đáp án (TC-ACT-001); submit/response có `Idempotency-Key`, audit, outbox |
| V Xác thực, phân quyền | ✅ | Route người chơi qua `game_player` + owner; validate CMS dưới `learning_content.activity.write` (endpoint của F030) |
| VI An toàn trẻ em | ✅ | Không log nội dung câu trả lời/PII; integrity signal không tự phạt |
| VII Một nguồn sự thật, cấu hình | ✅ | `video_completion`, `attempts_per_question` theo tên; mọi chuỗi `varchar(n)`/`maxLength` |
| VIII UI tách logic, token | ✅ | Renderer chỉ phát intent; phụ đề theo Theme |
| IX Test có bằng chứng | ✅ | Mỗi F065-AC có task test; test hợp đồng plugin |

## Project Structure

### Documentation (feature này)

```text
specs/065-video-lesson/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
modules/learning/
├── ent/schema/{video_progress.go,video_branch_choice.go}           # MaxLen, mixin Base/Tenant/SoftDelete (pkg/entx)
├── migrations/<ts>_video_lesson.sql                                 # Atlas diff + RLS viết tay
├── internal/domain/activity/videolesson/
│   ├── config.go            # VideoLessonConfig + Validate (thuần)
│   ├── ranges.go            # Merge, Clamp(max 64), WatchedMs, Frontier
│   ├── plausibility.go      # AcceptRanges(prev, incoming, elapsed, maxRate)
│   ├── seek.go              # CheckSeek(policy, position, frontier, completed)
│   └── completion.go        # Evaluate(rule, ratio, cues) → Completed/Passed
├── internal/application/activityplugin/videolesson/plugin.go       # hiện thực activity.Plugin (F030), đăng ký Registry
├── internal/application/port/{video_progress_repository.go,media_variant_resolver.go,integrity_signal_writer.go}
├── internal/infrastructure/persistence/{video_progress_repo.go,video_branch_choice_repo.go,integrity_signal_repo.go}
├── cmd/api/main.go                                                  # Registry.Register(videolesson.New(...))
└── tests/{unit/videolesson,integration/videolesson,contract/plugins}/
godot/
├── scenes/learning/activity_renderers/video_lesson/VideoLessonRenderer.tscn (+ video_lesson_renderer.gd)
├── scenes/learning/video/VideoLessonPlayer.tscn                     # VideoStreamPlayer + SubtitleLayer + CueOverlay + ChapterList + SeekBar
├── scripts/learning/{vtt_parser.gd,video_progress_reporter.gd,watched_ranges.gd}
├── locale/{vi.po,en.po}                                             # ui.video.*
└── tests/{unit,scene,integration}/video/
```

**Structure Decision**: Backend theo [BACKEND_DESIGN §3](../../docs/07-backend/BACKEND_DESIGN.md) với thư mục `modules/learning/` (Bước 0 D1); plugin nằm ở application vì cần port (repository, media), quy tắc thuần ở domain. Godot theo [GODOT_ARCHITECTURE §3, §6.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md).

## Phase 0: Research

→ [research.md](research.md): R1 lưu câu chèn là item; R2 kiểm hợp lý + chính sách tua; R3 client chọn biến thể.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `videolesson.Config`, `Ranges`, `AcceptRanges`, `CheckSeek`, `Evaluate` | Không I/O; invariant ≤ 64 khoảng, `start_ms < end_ms ≤ duration_ms` |
| application | Plugin `videolesson.Plugin`; command của engine F030 gọi plugin: `StartAttempt` → `BuildRenderPayload`, `ReportProgress` → `Progress`, `SubmitResponse` → `Respond`, `SubmitAttempt` → `Evaluate`/`EmitEvidence` | Người chơi: `game_player` + owner; CMS validate: `learning_content.activity.write` |
| infrastructure | `VideoProgressRepository` (ent, `SELECT … FOR UPDATE` theo `attempt_id`), `VideoBranchChoiceRepository`, `IntegritySignalWriter`, `MediaVariantResolver` (port F027) | Ghi trong transaction của command + outbox |
| presentation | Handler có sẵn của F030: `startActivityAttempt`, `reportActivityProgress`, `submitActivityResponse`, `submitActivityAttempt`, `getActivityAttempt`; `listActivityTypes` trả `video_lesson` | Không thêm endpoint |

### Tích hợp và phụ thuộc

- F030 (Plugin/Registry, attempt tables, command/query handler, outbox, audit, Problem codes), F027 (`media_variants`, signed URL), F031 (learning event writer, evidence → mastery), F063 (renderer câu hỏi, `ActivityHost`, `PendingWrites`), F062 (`QualityManager` preset, `suspend_world`).
- Event phát: learning event `activity.started`, `video.progressed`, `video.cue_answered`, `question.answered`, `activity.completed`; domain event (qua engine F030) `activity.started`, `activity.submitted`, `activity.graded`.

### Bảo mật, tenant, idempotency, audit

- `TenantScope` cho mọi truy vấn; RLS trên bảng mới; owner check `attempt.student_id = principal.user_id`.
- Idempotency: progress theo `client_seq`; responses/submit theo `Idempotency-Key` (`infra.idempotency_records`).
- Audit `attempt.submitted` do engine F030; plugin không ghi audit riêng.

### Quan sát

- Metric `eduworld_video_progress_trimmed_total`, `eduworld_video_seek_rejected_total`, `eduworld_video_completion_total{completed}`; span `videolesson.Progress`, `videolesson.Evaluate`; log không chứa câu trả lời (R14).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |

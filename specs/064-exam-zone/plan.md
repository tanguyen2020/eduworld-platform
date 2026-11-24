# Implementation Plan: F064 — Exam Zone: thi có giờ theo server, tự nộp, resume, thông báo bài mới

**Branch**: `064-exam-zone` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

`ExamService` (`RefCounted`) bọc `/exams/*` (getExamSession, listExamSessions, startExamAttempt, saveExamAnswer, submitExamAttempt, getExamResult). Ba màn: `ExamLobby` (danh sách đề + quy chế), `ExamRunner` (đồng hồ theo `Clock`, lưới câu, đánh dấu, renderer câu hỏi F063 ở chế độ exam), `ExamResult`. `ExamCountdown` là hàm thuần (`remaining_ms(deadline_at, server_now)`, mốc cảnh báo) để unit test với đồng hồ giả. Lưu đáp án qua `PendingWrites` (RAM, gộp theo `position`). `NotificationService` (`RefCounted`) so sánh danh sách bài giao giữa các lần làm mới của `AssignmentService` (F062) để phát toast/badge — không polling định kỳ, không push.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim), static typing (ADR-0014) |
| Primary Dependencies | GdUnit4; `ApiClient`, `Clock` (F061); `AssignmentService`, `QualityManager`, HUD (F062); `QuestionRendererRegistry`, `PendingWrites` (F063) |
| Storage | Không có — đề và câu trả lời chỉ trong RAM |
| Testing | GdUnit4 unit (đồng hồ giả, `FakeApiClient`), scene runner, integration headless với compose (toxiproxy cho mất mạng) |
| Target Platform | Android, Web, iOS (ADR-0013; không Windows — ADR-0101) |
| Project Type | game client |
| Performance Goals | P-6 [STUDENT_GAME_UX_LEARNING §2.1](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md); [PERFORMANCE_BUDGET §7 (Activity player), §9](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |
| Constraints | AGENTS R5, R6, R10, R16, R17 · ADR-0016, ADR-0031, ADR-0043, ADR-0065 |
| Scale/Scope | Đề ≤ 200 câu; 1 attempt active |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta `ExamSession.navigation`, `MyAssignment.exam_session_id`, `takeover` ở Phase 2 (T004) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public; Service ← UI |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id`; không lưu đề xuống đĩa; test F064-AC-10 |
| IV Server-authoritative | ✅ | Thời gian theo `deadline_at`; tự nộp phía server; client không gửi điểm; submit có `Idempotency-Key` |
| V Xác thực, phân quyền | ✅ | Token RAM; `GAME_ACCESS_STUDENT_ONLY`/404 xử lý |
| VI An toàn trẻ em | ✅ | Không so sánh/xếp hạng; cảnh báo giờ nhẹ, không nhấp nháy; không push ngoài game |
| VII Một nguồn sự thật, cấu hình | ✅ | Không hardcode thời lượng/ân hạn; ghi nhận thiếu tham số ân hạn offline |
| VIII UI tách logic, token | ✅ | Đếm ngược/diff thông báo ở service thuần; Theme từ token |
| IX Test có bằng chứng | ✅ | Mỗi F064-AC có task test |

## Project Structure

### Documentation (feature này)

```text
specs/064-exam-zone/
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
src/game/
├── scenes/exam/
│   ├── ExamLobby.tscn (+ exam_lobby.gd)       # danh sách đề + màn quy chế
│   ├── ExamRunner.tscn (+ exam_runner.gd)     # header đồng hồ, QuestionGrid, nút Trước/Sau, Đánh dấu, Nộp, SubmitConfirm
│   ├── QuestionGrid.tscn, SubmitConfirm.tscn, ExamCountdownLabel.tscn
│   └── ExamResult.tscn (+ exam_result.gd)
├── scenes/ui/hud/NotificationBell.tscn        # badge 🔔 (slot HUD của F062)
├── scenes/world/zones/main_hub/Zone.tscn      # thêm Interactable `board.exam_sessions` (greybox)
├── scripts/learning/exam_service.gd
├── scripts/learning/exam_countdown.gd         # hàm thuần
├── scripts/learning/models/{exam_session.gd,exam_result.gd}
├── scripts/notifications/notification_service.gd
├── locale/{vi.po,en.po}                       # ui.exam.*, ui.notification.assignment_new, ui.error.EXAM_*
└── tests/{unit,scene,integration}/exam/, tests/fixtures/exam/*.json
```

**Structure Decision**: Theo [GODOT_ARCHITECTURE §3](../../docs/08-game-client/GODOT_ARCHITECTURE.md) (`scenes/exam/`, `scripts/notifications/notification_service.gd`). Logic thời gian và diff thông báo tách hàm thuần để test.

## Phase 0: Research

Không có. Quyết định ghi trong spec Clarifications: hàng đợi RAM thay vì lưu đệm đĩa (GODOT_ARCHITECTURE sở hữu hành vi client, báo lại mâu thuẫn với EXAM_ENGINE §6); thông báo dựa trên làm mới theo điểm kích hoạt.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| Service | `ExamService.load_session(id)`, `start(id)`, `save_answer(position, response)`, `submit()`, `result(attempt_id)`, `list_sessions()`; `ExamCountdown`; `NotificationService.on_assignments_updated(list)` | Signal `exam_started`, `time_warning(minutes)`, `time_up`, `exam_submitted`, `notice_added` |
| UI | `ExamLobby`, `ExamRunner`, `ExamResult`, `NotificationBell`, toast | Chỉ gọi service, đọc `AppState` |
| Gameplay | `Interactable` `board.exam_sessions` → `ui_requested("exam_lobby")` | |

### Tích hợp và phụ thuộc

- F042 (API exam), F041 (`listMyAssignments`), F063 (renderer câu hỏi chế độ exam, `PendingWrites`, `ActivityHost` cho "Luyện LO này"), F062 (intent `exam_lobby`, HUD, `AssignmentService`), F061 (`Clock`).

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key` cho `startExamAttempt`, `submitExamAttempt`; `saveExamAnswer` là PUT ghi đè (idempotent theo `position`).
- Audit nộp bài phía server (F042); client không log câu trả lời.

### Quan sát

- Telemetry: `exam_clock_skew_ms` (|offset| lớn), `exam_offline_sync{count}`; không PII, không nội dung câu trả lời.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |

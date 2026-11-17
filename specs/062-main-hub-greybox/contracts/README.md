# Contracts: F062 — Main Hub greybox, character controller, `QualityManager`, tab "Nhà trường giao"

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listMyAssignments` | `GET /api/v1/learning/my-assignments` | `specs/openapi/eduworld-api.v1.yaml` | `game_player` (học sinh, chỉ bài của mình) | Cần sửa (§6) |
| `getClientConfig` | `GET /api/v1/game/client-config` | `specs/openapi/eduworld-api.v1.yaml` | public (không `X-Tenant-Id`) | Có sẵn (`device_profiles`) |
| `postTelemetry` | `POST /api/v1/game/telemetry` | `specs/openapi/eduworld-api.v1.yaml` | `game_player` | Có sẵn (gồm `TelemetryEvent.type = break_reminder_shown`, `reminder_no`, `minutes_played` — F062-AC-24) |
| `heartbeatGameSession` | `POST /api/v1/game/sessions/{game_session_id}:heartbeat` | `specs/openapi/eduworld-api.v1.yaml` | `game_player` | Có sẵn (gọi bởi F061) |

Dữ liệu bootstrap dùng: `PlayerBootstrap.grade_band`, `PlayerBootstrap.policies.play_time.{break_reminder_minutes, access_window}` (Có sẵn, trả bởi `/auth/game/login` — F013/F061).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Client không gọi internal API | | | |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Chưa dùng realtime ở F062 (F092) | |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | | | |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | | |

## 6. Delta cần thực hiện

- **Cần sửa** `components.schemas.MyAssignment` (`specs/openapi/eduworld-api.v1.yaml` và `specs/openapi/modules/learning.v1.yaml`): thêm
  - `exam_session_id` (`Uuid`, có khi `content_kind = assessment` và bài `mode = graded` đã tạo ExamSession — [SCHOOL_LMS §4](../../../docs/04-admin-portal/SCHOOL_LMS.md));
  - `result_release` (`$ref ResultRelease`) để hiện "Kết quả sẽ có khi nhà trường công bố";
  - `late_policy` (`{allow_late: boolean, late_window_hours: integer 0..168}`, cùng shape `AssignmentCreate.late_policy`) cho nhãn quá hạn.
  Chạy `make specs-lint`, `make specs-breaking`; contract test với F041. Task T004 (thuộc delta chung với F064).

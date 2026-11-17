# Data Model: F058 — Workspace B — Learning Activity Builder (quiz, interactive_exercise, video_lesson), UI re-evaluate

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F058 là frontend, không tạo hay sửa bảng. Activity, version, release, media, regrade request thuộc module learning (F030, F026, F027, F031).

Trạng thái phía client (không bền vững):

| Trạng thái | Nơi giữ | Ghi chú |
|---|---|---|
| Bản nháp form chưa lưu | RAM (React Hook Form) | Autosave qua `updateCmsActivityDraft`; khi `412`/`409` giữ cục bộ để người dùng chép |
| ETag version đang sửa | RAM | Gửi `If-Match` mỗi lần lưu/chuyển trạng thái |
| `Idempotency-Key` thao tác đang chờ | RAM, theo mutation | Sinh một lần, giữ khi thử lại; xóa khi thành công |
| Phiên preview | RAM | Hủy khi quá `expires_at`; tạo lại khi bấm Xem trước |
| Polling `getRegradeRequest` / `getMediaAsset` | RAM | Dừng khi `completed`/`failed` hoặc `ready`/`rejected` |
| Cờ "video có lời" | RAM (form) | Không gửi server; chỉ điều kiện bắt buộc phụ đề `vi` [Proposed] |

Schema dùng (chỉ đọc định nghĩa):

| Schema | Nguồn | Dùng cho |
|---|---|---|
| `ActivityTypeInfo`, `EvaluationStrategy` | `specs/openapi/modules/learning.v1.yaml` | Bước 1, lọc chiến lược chấm |
| `ActivityDefinition`, `ActivityDefinitionInput` | như trên | Tạo/sửa activity |
| `LearningPreviewRequest`, `LearningPreviewSession`, `ActivityRender`, `QuestionRender` | như trên | Preview |
| `ContentVersion`, `ContentVersionHistory`, `PublishRequest`, `ContentRelease` | như trên | Workflow, chọn `corrected_version_id` |
| `RegradeRequestCreate`, `RegradeRequest` | như trên | Chấm lại |
| `MediaAsset`, `MediaKind`, `MediaPresignResponse` | như trên | Video, phụ đề |
| `activity-base.schema.json` (`questionSource`, `questionInteraction`, `evaluation`, `feedback`, `completionRule`, `activityCommon`) | `specs/schemas/activities/` | Form sinh từ schema |
| `quiz.schema.json`, `interactive-exercise.schema.json`, `video-lesson.schema.json` | như trên | Cấu hình theo loại |

## 3. Quy tắc kiểm tra dữ liệu

Kiểm tra sớm phía client (server vẫn là nguồn quyết định, `ACTIVITY_CONFIG_INVALID`):

- `questionSource`: `kind = questions` → 1…`maxItems` câu, mỗi câu `published` và loại ∈ `allowed_question_types`; `kind = question_set` → có `question_set_id`; `kind = selection` → có `count`, `difficulty_min ≤ difficulty_max`.
- Video: `0 ≤ start_ms < end_ms ≤ duration_ms`, chương không chồng lấn; `0 ≤ at_ms ≤ duration_ms`; `cue_id`, `chapter_id`, `segment_id`, `branch_point_id`, `option_id` duy nhất trong phạm vi; `target_segment_id` tồn tại; có segment `is_ending` khi có nhánh; đúng một phụ đề `is_default` khi có phụ đề ([VIDEO_LEARNING §3, §5–§7](../../docs/02-learning/VIDEO_LEARNING.md)).
- Chấm lại: `reason` 10–1.000 ký tự; `use_new_key` cần `corrected_version_id` là version `published` của cùng item ([ACTIVITY_ENGINE §7.3](../../docs/02-learning/ACTIVITY_ENGINE.md)).

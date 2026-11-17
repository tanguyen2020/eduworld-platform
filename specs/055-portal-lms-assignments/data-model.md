# Data Model: F055 — Workspace C — LMS: course, learning path, wizard giao bài, theo dõi nộp bài, lịch, nhóm học tập

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F055 là frontend, không tạo hay sửa bảng. Dữ liệu course, learning path, assignment, submission, schedule, study group thuộc module learning (F040, F041), exam session thuộc F042; Portal chỉ đọc/ghi qua API public.

Trạng thái phía client (không bền vững, không chứa dữ liệu nghiệp vụ được tin):

| Trạng thái | Nơi giữ | Ghi chú |
|---|---|---|
| Cache truy vấn TanStack Query | RAM | Query key luôn chứa `tenantId` (hoặc danh sách `tenant_ids` của System Admin); xóa khi đổi tenant/đăng xuất |
| Bản nháp wizard chưa gửi | `sessionStorage` khóa `ew.portal.lms.wizardDraft.<tenantId>` | Per-viewer, bọc try/catch; không chứa `Idempotency-Key` đã dùng; xóa khi publish thành công |
| `Idempotency-Key` của thao tác đang chạy | RAM của mutation | Sinh một lần mỗi thao tác, giữ khi retry |

Schema dùng (chỉ đọc định nghĩa, không đổi): `Assignment`, `AssignmentCreate`, `AssignmentUpdate`, `AssignmentTarget`, `AssignmentSubmission`, `AssignmentExtendRequest`, `ReasonRequest`, `Course`, `LearningPath`, `LearningPathInput`, `PathNode`, `ScheduleItem`, `ScheduleItemInput`, `StudyGroup`, `StudyGroupInput`, `ExamSession` trong `specs/openapi/eduworld-api.v1.yaml`.

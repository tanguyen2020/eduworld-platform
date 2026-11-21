# Data Model: F081 — Workspace C — sự kiện trường từ template

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F081 không tạo hay sửa bảng. Sự kiện trường, community goal thuộc F077 ([DATABASE_DESIGN §2.8c](../../docs/07-backend/DATABASE_DESIGN.md)); F081 chỉ thêm một read model (DTO) đọc từ các bảng đó. Read model `TenantGameSummary` mở rộng đã chuyển sang F080 (data-model §5).

### Read model `SchoolEventProgress` (query `GetSchoolEventProgress`, module game)

| Field | Nguồn | Ghi chú |
|---|---|---|
| `event_id`, `event_run_id`, `status` | `game.event_runs` (lọc `tenant_id`, `deleted_at IS NULL`) | `status` theo `EventRunStatus` |
| `participants_count` | `count(*)` `game.event_participation` theo `event_run_id`, trường trong scope | Không trả danh sách |
| `community_goal` {`goal_id`, `metric`, `target`, `total`, `milestones[]` {`pct`, `reached`}} | `game.community_goals` + `game.community_goal_progress` (`scope_id` = trường) | PostgreSQL, không Valkey (R9); `null` nếu sự kiện không có goal |
| `by_grade[]` {`grade_level`, `contribution_total`, `contributors_count`} | `SUM(amount)`, `COUNT(DISTINCT student_id)` từ `game.community_goal_contributions` `GROUP BY grade_level` (khối tại thời điểm đóng góp; lọc `tenant_id`, `goal_id`, `scope_id`) — ADR-0095 | Chỉ khối trong dải EVENT_SEASON_SYSTEM §6; bỏ dòng có `contributors_count` < `analytics_min_cell_size`; sắp theo `grade_level` |
| `data_as_of` | `community_goal_progress.recomputed_at` | |

## 2. Vòng đời / trạng thái

Không thay đổi; hiển thị theo `EventRunStatus` (`scheduled → live → ended → archived`, `suspended`, `cancelled`) của [EVENT_SEASON_SYSTEM §5](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md). Nút Sửa chỉ ở `scheduled`; Hủy ở `scheduled`/`live` (server quyết định).

## 3. Quy tắc kiểm tra dữ liệu

- Không trả `student_id`, tên, nickname, thứ hạng trong read model (AGENTS R16).
- SQL tường minh phải có `tenant_id = $tenant` và (bảng có cột) `deleted_at IS NULL`, có test (R27).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | Chỉ query, không phát event | — |

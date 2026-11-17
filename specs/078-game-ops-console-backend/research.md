# Research: F078 — Game Operations Console backend: workflow nội dung game, thumbnail 3D, catalog cho nhà trường

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Catalog quest/mini-game cho nhà trường

- **Decision**: Giữ `listGameActivities` ở module learning (`x-service: learning`, như OpenAPI hiện tại). module game là nguồn: phát `content.published`/`content.rolled_back`/`content.unpublished` (kèm metadata catalog); `tenant_data.assigned`/`unassigned` do module identity phát (F045) sau khi gọi facade `applyTenantAssignment`; module learning dựng projection theo tenant; `listAssignedGameActivities` (internal) để rebuild projection (DATABASE_DESIGN §2.17 quy tắc 3).
- **Rationale**: Bộ chọn nội dung khi giao bài (activity, question set, quest, mini-game) nằm ở module learning; giao bài cần kiểm `content_version_id` đã publish tại thời điểm ghi mà không gọi đồng bộ (R29). Không đổi contract public đã có.
- **Alternatives**: (a) Đổi `x-service` sang game và portal gọi hai module — thêm một route ngoại lệ trong `/lms/*` và LMS vẫn cần projection để kiểm khi tạo bài giao; (b) learning gọi đồng bộ game khi giao bài — vi phạm hướng dẫn R29 về phản ứng phụ và tăng phụ thuộc.

## R2. Chạy render thumbnail/turntable

- **Decision**: Worker module game nhận `content.published` (loại có asset 3D) → ghi job vào work-queue NATS `ew.job.game.render.>` (stream `GAME_RENDER_JOBS`, retention WorkQueue) → container `game-renderer` (Godot headless, script `src/game/tools/thumbnail_renderer/render_thumbnails.gd`, cùng lệnh `make render-thumbnails`) lấy job, tải asset từ object storage, render, tải kết quả lên `media/<sha256>/…` và trả kết quả qua `ew.job.game.render.result`; worker module game đăng ký `media_assets`/`media_usages` trong transaction.
- **Rationale**: Godot headless nặng và có vòng đời khác Go worker; tách container cho phép scale riêng và không cài Godot vào image `worker`. Kết quả ghi DB vẫn do module game (R13: không module nào khác ghi DB của game).
- **Alternatives**: (a) Gọi `godot --headless` trực tiếp trong worker Go — image phình, crash renderer ảnh hưởng consumer khác; (b) Render trong CI lúc build asset — không phủ "mọi version publish" khi version chỉ đổi material/tham số.

## R3. Quyền theo loại cấu hình

- **Decision**: Bảng ánh xạ `config_type → permission ghi` trong code (`internal/presentation/http/public/gameadmin_permissions.go`), kiểm tại handler và use case; đọc cho phép quyền ghi của loại, `game.content.review`, `game.content.publish`; duyệt `game.content.review`; publish/rollback/unpublish `game.content.publish`.
- **Rationale**: Khớp ADMIN_PORTAL §2.2, §3.3, §3.12; R7 yêu cầu authz nghiệp vụ trong Go.
- **Alternatives**: Một permission chung `game.content.write` — mất phân tách `game_designer`/`liveops` của ADMIN_PORTAL §2.3.

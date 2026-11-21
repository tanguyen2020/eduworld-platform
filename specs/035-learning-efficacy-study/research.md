# Research: F035 — Nghiên cứu hiệu quả học tập

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Ngăn evidence đo hiệu quả vào mastery, XP, dashboard

- **Decision**: ExamSession có `efficacy_wave_id` không phát `exam.*` ra outbox; learning event vẫn ghi với `context.efficacy_wave_id`; consumer mastery (F031) và job recompute bỏ qua event có trường này (phòng thủ hai lớp).
- **Rationale**: Mastery, rollup dashboard (F043), XP/quest (module game), thông báo đều tiêu thụ `exam.graded` ([EVENT_DESIGN §4](../../docs/07-backend/EVENT_DESIGN.md)); chặn tại nguồn không phải sửa module khác (R13, R29).
- **Alternatives**: Thêm cờ `purpose` vào `exam.graded` để mọi consumer tự lọc — bị loại vì cần sửa module game, notification và dễ sót consumer mới; AttemptMode mới `efficacy_test` — bị loại vì đổi enum chung, ảnh hưởng mọi spec (TC-LRN-017).

## R2. Thống kê trong hệ thống

- **Decision**: Domain thuần `efficacy/stats` (float64, không thư viện ngoài):
  - Gain `post − pre` (tỉ lệ đúng), normalized gain `(post − pre)/(1 − pre)` (bỏ qua khi `pre = 1`).
  - Hedges' g: `g = J·(M_i − M_c)/S_p`, `S_p = √(((n_i−1)s_i² + (n_c−1)s_c²)/(n_i+n_c−2))`, `J = 1 − 3/(4(n_i+n_c)−9)`; `SE = √((n_i+n_c)/(n_i·n_c) + g²/(2(n_i+n_c)))`.
  - Hiệu chỉnh cụm: ICC một chiều (ANOVA theo lớp), `deff = 1 + (m̄ − 1)·ICC`, `SE_adj = SE·√deff`, CI 95% = `g ± 1,96·SE_adj`.
  - Spearman ρ (hạng trung bình khi trùng), AUC = thống kê Mann–Whitney chuẩn hóa, KR-20 dùng lại `examquality.KR20` của F044.
- **Rationale**: Đủ cho §4–§5 của [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md); golden test tính tay được; mô hình đa mức chạy ngoài hệ thống trên dữ liệu xuất.
- **Alternatives**: Thư viện `gonum/stat` — được phép nếu cần, nhưng các công thức trên ngắn, tránh phụ thuộc; chạy R/Python trong worker — bị loại (thêm runtime, khó kiểm soát dữ liệu).

## R3. Mã giả danh

- **Decision**: `participant_code = base32(HMAC-SHA256(key, study_id ‖ student_id))[:16]`; `cohort_code` (ADR-0095 — thay `class_code`), `site_code` tương tự với `school_id` của cụm, `tenant_id`. Không lưu bảng ánh xạ; tra ngược chỉ khi có khóa (vận hành, audit).
- **Rationale**: Theo mẫu `player_key` ([GAME_ANALYTICS](../../docs/11-operations/GAME_ANALYTICS.md) A-4); muối `study_id` làm mã không liên kết được giữa các study ([DATA_PRIVACY §4](../../docs/10-security/DATA_PRIVACY.md)).
- **Alternatives**: UUID ngẫu nhiên lưu bảng ánh xạ — bị loại vì bảng ánh xạ là dữ liệu nhạy cảm thêm cần bảo vệ.

## R4. Phân arm ổn định

- **Decision**: `bucket = uint64(sha256(experiment_id ‖ unit_id)[:8]) mod 100`; arm = arm đầu tiên có `allocation_pct` cộng dồn > bucket; lưu `experiment_assignments` ở lần tiếp xúc đầu. Study dùng `sha256(study_id ‖ pair_no)` bit thấp nhất để chọn lớp can thiệp trong cặp.
- **Rationale**: Tái lập được, không cần trạng thái để tính lại; lưu lần đầu giữ arm khi chuyển lớp (F035-AC-22).
- **Alternatives**: `math/rand` có seed — bị loại vì phụ thuộc phiên bản thư viện.

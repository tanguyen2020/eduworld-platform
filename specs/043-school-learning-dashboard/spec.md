# Feature Specification: F043 — Rollup analytics và School Learning Dashboard API

**Feature Branch**: `043-school-learning-dashboard` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C3 · **Phụ thuộc**: F031, F041
**Input**: Nhà trường (School Admin, Academic Manager) theo dõi bài giao, LO yếu, học sinh cần hỗ trợ và hiệu quả tự học theo trường → khối → học sinh (ADR-0095: không còn lớp); Tenant Admin và System Admin xem bản tổng hợp. Mọi số liệu đọc rollup dựng gia tăng, không truy vấn dữ liệu thô (ADR-0062, ADR-0072). FR nguồn: FR-ANL-003, FR-ANL-005, FR-ANL-012, FR-ANL-013, FR-ANL-016, FR-LMS-003, FR-LMS-011, FR-PRG-001, NFR-PERF-017, NFR-PERF-018.

## Clarifications

### Session 2026-11-16

- Q: Bảng rollup đặt tên và cột theo DATABASE_DESIGN §2.7 (`progress_daily(date…)`, `class_objective_rollups`) hay DASHBOARD_DATA? → A: Theo DASHBOARD_DATA §3 và LEARNING_ANALYTICS §5 (`bucket_date`, `class_id` snapshot, `metric_version`, `is_final`, `rm_class_objective_stats`, `rollup_class_objective_weekly`); DATABASE_DESIGN cập nhật trong task Phase 2 (R33: DASHBOARD_DATA sở hữu cơ chế dashboard).
- Q: Hồ sơ học tập một học sinh (US-LMS-005) có trong feature? → A: Có, qua endpoint `GET /lms/students/{student_id}/analytics` (LEARNING_ANALYTICS §8), contract cần thêm.
- Q: Phân tích đề/bài (US-LMS-007) và xuất báo cáo thuộc feature nào? → A: F044 (báo cáo đề, export); F043 chỉ cung cấp rollup và API đọc dashboard.

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).

### Session 2026-11-21 (triển khai — quyết định [Confirmed] chờ owner)

- Q: Consumer `learning.rollup_dashboard` tách riêng hay dùng consumer `learning.progress_daily` của F031? → A: [Confirmed] Mở rộng `learning.progress_daily`: cùng event (`activity.graded`, `lesson.completed`), cùng transaction và một dòng `processed_events` cộng dồn `progress_daily` và `rollup_class/school/tenant_daily` — hai cấp không lệch nhau; upsert theo từng event (chưa gom lô `dashboard_ingest_batch`, đo lại ở TC-PERF-025).
- Q: Tỷ lệ bài giao (`assignment.*`) và trạng thái mastery (`learning_objective.*`) cộng dồn từ event? → A: [Confirmed] Không: trạng thái nộp (`missed`, `excused`) và level không cộng dồn được; job `read-model-refresh` (15 phút) và `dashboard-day-close` tính lại từ `lms.assignment_submissions`/`objective_masteries` (số nộp đến hạn trong ngày, đã nộp, đúng hạn) — idempotent.
- Q: Distinct "hôm nay" bằng HyperLogLog Valkey? → A: [Confirmed] Tính chính xác từ `progress_daily` của ngày ở mỗi lần refresh (vài nghìn dòng/trường, qua index) — không phụ thuộc Valkey (ADR-0004); HLL để lại khi đo tải cần.
- Q: Role `learning_dashboard` đọc gì ngoài rollup? → A: [Confirmed] Thêm SELECT trên nguồn trạng thái của DASHBOARD_DATA §2 (`objective_masteries`, `rm_student_recommendations`, `lms.assignments`, `lms.assignment_submissions`), projection `ref.*` (tên lớp, sĩ số, phạm vi) và metadata curriculum (`curriculum.topics`, `curriculum.learning_objectives` + `infra.tenant_resource_assignments` cho policy `content_read`); không có quyền trên `learning_events`, `mastery_history`, `assessment.*`, bảng `lms.*` khác (TC-DASH-001).
- Q: Tổng hợp liên tenant chạy bằng role nào? → A: [Confirmed] `learning_dashboard` chịu RLS một tenant nên đọc liên tenant dùng `learning_platform` trên replica (`EW_DB_LEARNING_DASHBOARD_PLATFORM_DSN`) dưới `CrossTenantScope` (`system_admin_cross_tenant` khi có `tenant_ids`, `platform_analytics` khi vắng — danh sách ADR-0089), có audit `cross_tenant.accessed`; thiếu một trong hai DSN → route dashboard trả 503, không bao giờ dùng pool nghiệp vụ.
- Q: Thiếu `from`/`to` qua HTTP? → A: [Confirmed] Contract khai báo `required` nên trả `422 VALIDATION_FAILED` trước use case; mọi lỗi khoảng khác (đảo ngược, > 730 ngày, grain mịn hơn) và thiếu khoảng ở use case trả `422 DASHBOARD_RANGE_TOO_LARGE`. `DashboardPageSize` nới `maximum` của contract (100 → 10000) để server giới hạn về `dashboard_page_size_max` thay vì từ chối (F043-AC-16).
- Q: Định nghĩa số liệu chưa chốt? → A: [Confirmed] (1) "Học sinh hoạt động" của widget trường = distinct trong 7 ngày kết thúc ở `to`; ở báo cáo tenant/platform = trung bình distinct mỗi ngày/tuần/tháng theo `grain`; khi không lọc môn dùng distinct mọi môn (`active_students_all`). (2) LO "cần củng cố" = tỷ lệ `beginning`+`developing` ≥ 50% học sinh có evidence. (3) Read model dùng `level` đã lưu của `objective_masteries` (decay chỉ áp khi học sinh đọc tiến độ của mình). (4) `recent_attempts` của hồ sơ = lịch sử bài giao (role dashboard không đọc `attempts`). (5) Snapshot mastery tuần cập nhật ở mỗi lần refresh (lần cuối trong tuần là snapshot); job `mastery-snapshot-weekly` gộp vào `read-model-refresh`. (6) `time_hist` (percentile) chưa lưu vì không endpoint F043 nào đọc percentile; báo cáo tenant/platform chưa trả `series`. (7) `grade_level = 0` = chưa rõ khối. (8) Job `dashboard-day-close` chạy mỗi 15 phút, chốt khi đã qua `dashboard_day_close_time`.
- Q: Tham số `dashboard_*` đọc từ đâu? → A: [Confirmed] Module learning chưa có reader platform/tenant settings cho các khóa này: `analytics.DefaultConfig()` giữ giá trị CONFIG_DEFAULTS, composition root truyền vào (mẫu `answer_rate`); `dashboard_attention_thresholds` theo tenant dùng mặc định platform tới khi có reader.

### Session 2026-11-21 (triển khai — [Confirmed], owner chốt 2026-11-20)

- Q: T042 `DashboardMeta.available_from` tính thế nào? → A: [Confirmed] Ngày `bucket_date` nhỏ nhất của rollup ngày trong phạm vi câu trả lời: (ADR-0095: không còn nhánh lớp — luôn `rollup_school_daily` của trường; xem Session ADR-0095) `rollup_class_daily` của lớp khi có `class_id` (dashboard lớp, dashboard trường lọc lớp), ngược lại `rollup_school_daily` của các trường của phạm vi (dashboard trường, chuỗi, hồ sơ học sinh theo trường của học sinh, tóm tắt tenant); không áp bộ lọc khối/môn; `null`/vắng khi phạm vi chưa có dòng và ở tóm tắt platform (không tính liên tenant). Chưa có bảng `available_from` theo chỉ số của DASHBOARD_DATA §5 (backfill) — khi có, thay nguồn mà không đổi contract.
- Q: T042 `replica_lag_warning`? → A: [Confirmed] Đo trong cùng transaction đọc (savepoint, không bao giờ làm hỏng câu trả lời): `pg_is_in_recovery()` và đã replay hết WAL nhận được → 0, ngược lại `now() - pg_last_xact_replay_timestamp()`; cờ = độ trễ > `dashboard_replica_lag_alert_seconds` (60 s, CONFIG_DEFAULTS, `analytics.Config.ReplicaLagAlert`); không đo được → cờ `true` (log WARN). Số liệu vẫn trả từ replica (không fallback primary, F043-AC-21); phản hồi có cờ `true` không ghi cache Valkey. Cả hai trường là tùy chọn trong contract (không phá vỡ client cũ).

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Bỏ khái niệm lớp (owner chốt 2026-11-21, [ADR-0095](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md)) — dashboard đổi gì? → A: [Confirmed] Phân cấp **trường → khối → học sinh**. Rollup ghi khối **tại thời điểm sự kiện** (`progress_daily.grade_level`, `learning_events.grade_level` của WP-L1; khối hiện tại của `ref.students` chỉ khi dòng chưa có); không ghi `class_id` nữa (cột giữ tới WP-R). Bỏ `rollup_class_*`, `rollup_class_objective_weekly`, `rm_class_objective_stats`, `rm_class_subject_summary` (không ghi, không đọc, DROP ở WP-R); thêm `rollup_grade_objective_weekly` và `rm_grade_objective_stats` (trường × khối × LO, heatmap LO của khối), `rm_school_grade_summary` thêm tỉ lệ nộp bài theo khối (khối của học sinh lúc được giao — `assignment_target_students.grade_level`). Không backfill SQL: job `read-model-refresh`/`dashboard-day-close` dựng lại (ADR-0095 chạy trước lần triển khai staging đầu tiên).
- Q: Contract? → A: [Confirmed] Xóa `getClassDashboard`; thêm `getGradeDashboard` (`GET /lms/schools/{school_id}/grades/{grade_level}/dashboard`, `x-portal-action: action.school_lms.school_dashboard.view`, `analytics.school.read` trong trường/môn của grant). `getSchoolDashboard` bỏ `class_id` (giữ `grade_level` để lọc một khối): `widgets.classes` → `widgets.grades`, `weak_objectives[].class_ids` → `grade_levels`, `students_needing_attention[].class_id` → `grade_level` (khối hiện tại), `DashboardAssignmentProgress.by_class` → `by_grade`, thêm `suppressed`; `widgets.active_students` nullable. `getSchoolAnalyticsSeries` giữ `grade_level` (chuỗi khối). Không đổi `DashboardMeta` (`available_from` = rollup ngày của trường; `replica_lag_warning` như cũ). Quyền: chỉ `analytics.school.read` (`analytics.class.read` đã gộp ở WP-I1).
- Q: Che ô nhỏ ở cấp khối? → A: [Confirmed] Mọi ô tổng hợp cấp khối có dưới `analytics_min_cell_size` (5) học sinh bị che (FR-ANL-016) với che bổ sung giữa các khối của trường (`analytics.SuppressGrades`, kích thước khối = học sinh hiệu lực của `ref.students`); khối 0 (chưa rõ) luôn bị che. Số tổng hợp của trường (accuracy, chuỗi, học sinh hoạt động, tự học, LO cần củng cố) chỉ cộng khối hiển thị; lọc một khối bị che hoặc trường không có khối nào hiển thị → `suppressed: true`, số tổng hợp `null`, chuỗi `value: null` + `suppressed`. Ô tiến độ bài giao theo khối có dưới 5 học sinh được giao cũng bị che (che bổ sung trong bài giao; tổng của bài = các ô hiển thị). Danh sách học sinh của khối và học sinh cần chú ý (dữ liệu từng học sinh trong quyền của nhân sự trường) vẫn trả. File xuất áp cùng quy tắc (F044).

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: Chỉ plan/contracts nhắc MFA cho đọc liên tenant của System Admin; đọc là `platform.cross_tenant.read` + client portal + audit, không MFA. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Pipeline rollup gia tăng và chốt ngày (Priority: P1)

Mỗi câu trả lời, bài nộp, thay đổi mastery và trạng thái bài giao được cộng dồn gần thời gian thực vào rollup theo học sinh × ngày, trường × khối, tenant (ADR-0095: không còn cấp lớp); job chốt ngày tính chỉ số không cộng dồn và đặt cờ đã chốt.

**Why this priority**: Mọi dashboard đọc rollup; không có rollup thì không có dashboard (FR-ANL-005, FR-ANL-013).

**Independent Test**: Chạy golden dataset qua consumer + job chốt ngày → số liệu rollup khớp tính tay; gửi trùng event không đếm hai lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F043-AC-01 | KHI nhận event học tập (`question.answered`, `activity.submitted`, `activity.graded`, `exam.graded`, `attempt.re_evaluated`, `assignment.*`, `learning_objective.*`), consumer rollup PHẢI upsert cộng dồn vào dòng của ngày event ở mọi cấp (học sinh, trường × khối theo `grade_level` ghi lúc event — ADR-0095, tenant × khối) với `is_final = false` | FR-ANL-005 · [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-004 |
| F043-AC-02 | KHI cùng `event_id` được giao lại, consumer PHẢI không cộng lần hai (`processed_events`) | ADR-0042 · [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-005, TC-SVC-003 |
| F043-AC-03 | KHI job chốt ngày chạy cho một tenant, HỆ THỐNG PHẢI tính chính xác chỉ số distinct (học sinh hoạt động), đặt `is_final = true`, dựng lại tuần/tháng chứa ngày đó từ bảng ngày và tăng phiên bản cache của tenant | FR-ANL-013 · [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-004 |
| F043-AC-04 | NẾU event đến muộn trong `dashboard_late_window_hours`, THÌ HỆ THỐNG PHẢI cộng vào đúng ngày và đánh dấu ngày cần chốt lại; NẾU muộn hơn cửa sổ, THÌ HỆ THỐNG PHẢI không sửa rollup và tăng metric `eduworld_rollup_late_events_total` | [DASHBOARD_DATA §5](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-005 |
| F043-AC-05 | HỆ THỐNG PHẢI tính tỷ lệ (accuracy, `rapid_guess`, nộp đúng hạn) từ tử số/mẫu số lưu riêng, loại lần trả lời `rapid_guess` khỏi accuracy và mastery, và gắn `metric_version` cho mọi dòng | FR-PRG-001 · [LEARNING_ANALYTICS §4](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-DASH-004 |
| F043-AC-06 | HỆ THỐNG PHẢI làm mới read model trạng thái hiện tại (phân bố level trường × khối × LO, trường × khối × môn kèm tỉ lệ nộp bài theo khối — ADR-0095) trong `school_dashboard_refresh` và snapshot mastery theo tuần | FR-ANL-003 · [LEARNING_ANALYTICS §5](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-LMS-003 |

---

### User Story 2 — School Learning Dashboard cấp trường và khối (Priority: P1)

`ad.truong.a` mở dashboard trường: bài giao đang mở/sắp hạn, LO cần củng cố, học sinh cần chú ý, tự học & gợi ý, thông báo an toàn; drill-down sang khối (heatmap LO của khối, học sinh của khối, cờ "làm quá nhanh") — ADR-0095.

**Why this priority**: Trả lời năm câu hỏi hằng tuần của nhà trường (US-LMS-002, US-LMS-003, US-LMS-004).

**Independent Test**: `ad.truong.a` gọi `getSchoolDashboard` và `getGradeDashboard` cho khối 3 → số liệu khớp truy vấn đối soát; khối dưới 5 học sinh bị che; `ad.truong.b` gọi với `school_id` TH Demo A → 404.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F043-AC-07 | KHI người có `analytics.school.read` gọi `getSchoolDashboard`, module learning PHẢI trả trong một lần gọi các khối: bài giao mở/sắp hạn với tỷ lệ nộp theo khối, các khối của trường, LO cần củng cố, học sinh cần chú ý, tự học & gợi ý, số thông báo an toàn chưa xác nhận, kèm `grain`, `data_as_of`, `is_partial` | FR-LMS-011 · US-LMS-003 · [SCHOOL_LMS §8.1](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F043-AC-08 | HỆ THỐNG PHẢI đưa học sinh vào danh sách "cần chú ý" theo `dashboard_attention_thresholds` (bài `missed`, LO `beginning`, lâu không vào game, tỷ lệ `rapid_guess` hiển thị là "làm quá nhanh") | US-LMS-004 · AC-PRG-002 · [SCHOOL_LMS §8.1](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F043-AC-09 | HỆ THỐNG PHẢI trả khối tự học & gợi ý gồm thời gian học trung bình/học sinh, tỷ lệ học sinh bắt đầu hoạt động gợi ý trong 7 ngày và số LO lên mức sau ôn tập gợi ý (`recommendation_follow_rate`) | US-LMS-009 · [LEARNING_ANALYTICS §4](../../docs/02-learning/LEARNING_ANALYTICS.md) | — |
| F043-AC-10 | KHI gọi `getGradeDashboard`, HỆ THỐNG PHẢI trả tiến độ bài giao của khối, phân bố mức mastery khối × LO (heatmap, ADR-0095) và danh sách học sinh của khối phân trang cursor ≤ `dashboard_page_size_max` | US-LMS-002 · [SCHOOL_LMS §8.3](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003, TC-DASH-002 |
| F043-AC-11 | HỆ THỐNG PHẢI trả Learning Score, Mastery và XP/Game Rating tách biệt, không có cột XP, rating hay xếp hạng học sinh theo điểm học tập, không chứa nội dung chat | FR-ANL-003 · ADR-0018 · [SCHOOL_LMS §9](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F043-AC-12 | KHI gọi `getSchoolAnalyticsSeries` với `metric` hợp lệ, HỆ THỐNG PHẢI trả chuỗi ≤ 120 điểm theo độ hạt server chọn | FR-ANL-012 · [DASHBOARD_DATA §8](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-003 |
| F043-AC-13 | HỆ THỐNG PHẢI giới hạn `school_admin` trong trường của UserRole và `academic_manager` trong trường/môn được gán; ngoài phạm vi trường PHẢI trả `404 NOT_FOUND`, môn ngoài phạm vi PHẢI trả `403 PERMISSION_DENIED` | AC-ORG-002 · AC-ORG-003 | TC-TEN-002, TC-TEN-003 |
| F043-AC-14 | NẾU người gọi là `safety_moderator`, học sinh, hoặc thiếu `analytics.school.read`, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` | [SCHOOL_LMS §13](../../docs/04-admin-portal/SCHOOL_LMS.md) (AC-LMS-018) · AGENTS R7 | — |
| F043-AC-15 | NẾU `school_id` (kể cả của `getGradeDashboard`) thuộc tenant khác, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND`; khóa cache PHẢI chứa `tenant_id` và không trả dữ liệu chéo tenant | AC-ORG-001 | TC-DASH-009, TC-TEN-001 |
| F043-AC-28 | HỆ THỐNG PHẢI che mọi ô tổng hợp cấp khối có dưới `analytics_min_cell_size` học sinh (che bổ sung giữa các khối của trường; ô tiến độ bài giao theo khối theo số học sinh được giao), chỉ cộng khối hiển thị vào số của trường và trả `suppressed: true` với số tổng hợp `null` khi lọc một khối bị che (ADR-0095) | FR-ANL-016 · [LEARNING_ANALYTICS §7](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-DASH-009 |

---

### User Story 3 — Quy tắc truy vấn và bảo vệ database (Priority: P1)

Dashboard luôn có khoảng thời gian giới hạn, độ hạt tự chọn, chạy trên read replica với timeout và giới hạn đồng thời, không làm chậm database nghiệp vụ.

**Why this priority**: Dữ liệu tăng theo ngày; thiếu bảo vệ sẽ làm treo database (NFR-PERF-017, NFR-PERF-018).

**Independent Test**: Gọi dashboard với 731 ngày → `422 DASHBOARD_RANGE_TOO_LARGE`; 5 truy vấn đồng thời một user → truy vấn thứ 5 `429 DASHBOARD_BUSY`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F043-AC-16 | NẾU thiếu `from`/`to`, `to < from`, khoảng > `dashboard_max_range_days` hoặc `grain` mịn hơn quy tắc, THÌ HỆ THỐNG PHẢI trả `422 DASHBOARD_RANGE_TOO_LARGE`; `page_size` lớn hơn `dashboard_page_size_max` PHẢI bị giới hạn | FR-ANL-013 | TC-DASH-002 |
| F043-AC-17 | HỆ THỐNG PHẢI chọn độ hạt ngày khi khoảng ≤ `dashboard_grain_daily_max_days`, tuần khi ≤ `dashboard_grain_weekly_max_days`, còn lại tháng, và đọc bảng `*_daily`/`*_weekly`/`*_monthly` tương ứng | FR-ANL-013 | TC-DASH-003 |
| F043-AC-18 | HỆ THỐNG PHẢI chạy truy vấn dashboard bằng role `learning_dashboard` trên read replica, chỉ đọc bảng rollup/read model, không đọc `learning_events`, `attempts`, `attempt_answers` | NFR-PERF-018 · ADR-0062 | TC-DASH-001 |
| F043-AC-19 | NẾU truy vấn vượt `dashboard_statement_timeout`, THÌ HỆ THỐNG PHẢI hủy ở database và trả `504 DASHBOARD_QUERY_TIMEOUT` không giữ kết nối; NẾU người dùng vượt `dashboard_max_concurrent_per_user` hoặc tenant vượt `dashboard_max_concurrent_per_tenant`, THÌ HỆ THỐNG PHẢI trả `429 DASHBOARD_BUSY` có `Retry-After` | NFR-PERF-017 · [DASHBOARD_DATA §7](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-006 |
| F043-AC-20 | HỆ THỐNG PHẢI cache kết quả theo khóa có `tenant_id` + phiên bản dữ liệu (TTL `dashboard_cache_ttl_today`/`dashboard_cache_ttl_closed`); sau chốt ngày PHẢI không trả cache cũ; NẾU mất Valkey, THÌ HỆ THỐNG PHẢI vẫn trả đúng số liệu từ replica | ADR-0004 · [DASHBOARD_DATA §9](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-005, TC-DASH-007 |
| F043-AC-21 | NẾU replica dừng, THÌ HỆ THỐNG PHẢI trả `503` có thông báo và không chuyển truy vấn dashboard sang primary | NFR-PERF-018 | TC-DASH-007 |
| F043-AC-22 | HỆ THỐNG PHẢI đạt p95 ≤ `dashboard_query_p95_target` và không truy vấn nào > `dashboard_statement_timeout` trên dữ liệu 2 năm của 50.000 học sinh; mọi truy vấn trong query catalog PHẢI dùng index/partition pruning, không Seq Scan trên bảng rollup ngày | NFR-PERF-017 · [DASHBOARD_DATA §15](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-010, TC-PERF-025, TC-DASH-001 |

---

### User Story 4 — Hồ sơ học tập của một học sinh (Priority: P2)

School Admin mở hồ sơ một học sinh trong trường: tổng quan môn, mastery theo topic, xu hướng 8 tuần, lịch sử bài, gợi ý hệ thống đang hiển thị; mọi lượt xem được audit.

**Why this priority**: Hỗ trợ đúng học sinh cần giúp (US-LMS-005), sau dashboard trường/khối.

**Independent Test**: `ad.truong.a` gọi `getStudentLearningProfile` cho `hs.an.3a` → có dữ liệu, audit `learning.profile_viewed`; `ad.truong.b` gọi → 404.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F043-AC-23 | KHI người có quyền trong phạm vi gọi hồ sơ học sinh, HỆ THỐNG PHẢI trả tổng quan môn, mastery theo topic/LO, xu hướng 8 tuần, lịch sử bài, gợi ý đang hiển thị và ghi audit `learning.profile_viewed` | US-LMS-005 · FR-PRG-001 · [LEARNING_ANALYTICS §9](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-LMS-003 |
| F043-AC-24 | NẾU học sinh ngoài trường/môn trong phạm vi của người gọi, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND` không lộ sự tồn tại; response PHẢI không chứa Game Rating, vị trí trong game, bạn bè, nội dung chat | [SCHOOL_LMS §8.5, §9](../../docs/04-admin-portal/SCHOOL_LMS.md) · AGENTS R16 | TC-TEN-003 |

---

### User Story 5 — Tổng hợp cấp tenant và liên tenant (Priority: P2)

Tenant Admin của đơn vị nhiều trường xem tổng hợp học tập theo trường × khối × môn; System Admin xem tổng hợp liên tenant qua dropdown tenant; ô < 5 học sinh bị ẩn.

**Why this priority**: Phục vụ quản lý cấp đơn vị và platform (FR-ANL-012), cần rollup US1.

**Independent Test**: `ad.donvi` gọi `getTenantLearningSummary` → chỉ trường của `pgd-demo`, ô nhỏ hiện "< 5"; `sa.admin` gọi `getPlatformLearningSummary` với `tenant_ids` → audit `cross_tenant.accessed`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F043-AC-25 | KHI người có `analytics.tenant.read` gọi `getTenantLearningSummary`, HỆ THỐNG PHẢI trả số liệu gộp theo trường × khối × môn trong tenant của phiên và ẩn mọi ô có số học sinh < `analytics_min_cell_size` sao cho tổng không suy ra được ô ẩn | FR-ANL-016 · FR-ANL-012 | TC-DASH-009 |
| F043-AC-26 | KHI System Admin gọi `getPlatformLearningSummary` với `tenant_ids[]` (không gửi `X-Tenant-Id`), HỆ THỐNG PHẢI dùng `CrossTenantScope`, trả `tenant_id` trên từng dòng, ẩn ô < `analytics_min_cell_size` và ghi audit `cross_tenant.accessed` | FR-ANL-016 · AC-ORG-004 · ADR-0027 | TC-TEN-004, TC-TEN-018 |
| F043-AC-27 | NẾU người dùng tenant gửi `tenant_ids`, THÌ HỆ THỐNG PHẢI bỏ qua tham số và chỉ trả dữ liệu tenant của phiên; NẾU role tenant gọi route `/admin/analytics/*`, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` | AC-CMS-002 · AGENTS R20 | TC-TEN-011 |

### Edge Cases

- NẾU học sinh lên khối/chuyển trường giữa kỳ, THÌ số liệu cũ giữ ở khối/trường cũ theo `grade_level`/`school_id` ghi lúc event (F043-AC-01, ADR-0095); danh sách học sinh và "cần chú ý" theo khối hiện tại.
- NẾU định nghĩa chỉ số đổi, THÌ tăng `metric_version`, dashboard chỉ đọc version hiện hành (F043-AC-05).
- NẾU re-evaluate đổi điểm một attempt đã chốt ngày trong cửa sổ trễ, THÌ ngày đó được chốt lại (F043-AC-04).
- NẾU replica trễ > `dashboard_replica_lag_alert_seconds`, THÌ response vẫn trả kèm `data_as_of` cũ và cảnh báo trễ.
- NẾU khoảng thời gian chứa hôm nay, THÌ `is_partial = true` (F043-AC-07).
- NẾU học sinh bị soft-delete/dọn, THÌ không xuất hiện trong danh sách chi tiết; số gộp đã chốt không đổi.
- NẾU trường không có bài giao đang mở, THÌ khối bài giao trả rỗng, không lỗi.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-ANL-003 | Analytics theo học sinh, khối, trường, môn, chủ đề, LO (ADR-0095) | [LEARNING_ANALYTICS §5, §7](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-ANL-005 | Aggregation trong worker, dashboard đọc read model | [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-012 | API dashboard trường → khối → học sinh, tenant, platform (phần học tập; ADR-0095) | [DASHBOARD_DATA §2, §14](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-013 | Chỉ đọc rollup, khoảng ≤ 2 năm, độ hạt tự chọn, `data_as_of`, `is_partial` | [DASHBOARD_DATA §8](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-016 | Ẩn ô < 5 học sinh ở báo cáo tenant/platform | [LEARNING_ANALYTICS §5](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-LMS-003 | Theo dõi kết quả, tiến độ, kỹ năng yếu theo LO | [SCHOOL_LMS §8](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-011 | School Learning Dashboard API `getSchoolDashboard` | [SCHOOL_LMS §8](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-PRG-001 | Chỉ số tiến độ (bài hoàn thành, accuracy, thời gian, LO yếu, tiến bộ theo thời gian) | [LEARNING_ANALYTICS §4](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| NFR-PERF-017 | p95 truy vấn dashboard ≤ 2 s trên dữ liệu 2 năm, không truy vấn > 5 s | [DASHBOARD_DATA §15](../../docs/07-backend/DASHBOARD_DATA.md) |
| NFR-PERF-018 | Chỉ đọc rollup trên replica, không ảnh hưởng database nghiệp vụ | [DASHBOARD_DATA §7](../../docs/07-backend/DASHBOARD_DATA.md) |

### Key Entities

- **Rollup chuỗi thời gian**: số đo cộng dồn theo phạm vi (học sinh, trường × khối × môn, trường × khối × LO, tenant × khối × môn — ADR-0095) × ngày/tuần/tháng, có tử/mẫu số, `metric_version`, cờ đã chốt, thời điểm tính.
- **Read model trạng thái hiện tại**: phân bố level mastery trường × khối × LO, trường × khối × môn (kèm tỉ lệ nộp bài theo khối), snapshot mastery tuần.
- **Query catalog**: danh sách truy vấn dashboard định nghĩa sẵn (mã truy vấn, bảng đọc, tham số phạm vi), không có truy vấn tự do.
- **Danh sách cần chú ý**: học sinh vượt ngưỡng `dashboard_attention_thresholds` kèm lý do.
- **Hồ sơ học tập học sinh**: tổng hợp đọc từ rollup cấp học sinh, mastery, gợi ý (F032).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `dashboard_max_range_days`, `dashboard_grain_daily_max_days`, `dashboard_grain_weekly_max_days` | Giới hạn khoảng, chọn độ hạt |
| `dashboard_page_size_max` | Phân trang danh sách chi tiết |
| `dashboard_statement_timeout`, `dashboard_db_pool_per_api_replica` | Bảo vệ replica |
| `dashboard_max_concurrent_per_user`, `dashboard_max_concurrent_per_tenant` | Giới hạn đồng thời |
| `dashboard_cache_ttl_today`, `dashboard_cache_ttl_closed` | Cache kết quả |
| `dashboard_day_close_time`, `dashboard_late_window_hours`, `dashboard_ingest_batch` | Pipeline rollup |
| `dashboard_rollup_retention_daily_years / weekly / monthly`, `dashboard_replica_lag_alert_seconds` | Partition, cảnh báo trễ |
| `dashboard_query_p95_target` | Mục tiêu hiệu năng |
| `school_dashboard_refresh` | Chu kỳ làm mới read model |
| `dashboard_attention_thresholds` | Danh sách cần chú ý |
| `analytics_min_cell_size` | Ẩn ô nhỏ (k-anonymity) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không đọc bảng nghiệp vụ/thô từ API dashboard | NFR-PERF-018 · ADR-0062 |
| p95 ≤ 2 s, không truy vấn > 5 s | NFR-PERF-017 |
| Mọi truy vấn có tenant + phạm vi quyền + khoảng thời gian | AGENTS R8, R27 · ADR-0005 |
| Chỉ System Admin đọc liên tenant, có audit | AGENTS R20 · ADR-0027 |
| Không trả nội dung chat, Game Rating trong dashboard học tập | AGENTS R14, R16 · ADR-0018 |
| PostgreSQL là nguồn sự thật, Valkey chỉ cache | AGENTS R9 · ADR-0004 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Golden dataset: số cộng dồn khớp tuyệt đối sau chốt ngày; học sinh hoạt động lệch ≤ 2% trước chốt ngày và khớp tuyệt đối sau chốt ngày.
- **SC-002**: Kiểm thử tải 500 nhân sự mở dashboard trong 10 phút trên dữ liệu 2 năm: p95 ≤ 2 giây, 0 truy vấn > 5 giây.
- **SC-003**: 0 truy vấn của query catalog chạm bảng nghiệp vụ/thô (kiểm bằng quyền role dashboard và EXPLAIN).
- **SC-004**: 100% endpoint mới có test tenant isolation và authorization pass (School Admin trường khác, Academic Manager môn khác, Safety Moderator, role tenant gọi route platform).
- **SC-005**: Tiến độ bài giao trên dashboard cập nhật ≤ `school_dashboard_refresh` sau khi học sinh nộp.

## Ngoài phạm vi

- Giao diện School Learning Dashboard trên portal — F056; widget thông báo an toàn lấy từ module chat (F121/F122), F043 chỉ hiển thị số đếm khi API có sẵn.
- Xuất báo cáo bất đồng bộ, bảng kết quả course, báo cáo tham khảo TT27/TT22, phân tích đề/item analysis — F044.
- Định nghĩa mastery, gợi ý tự động — F031, F032; dữ liệu bài giao — F041; Game Analytics — F080.
- Kho phân tích cột (ClickHouse) — chỉ khi vượt ngưỡng DASHBOARD_DATA §12, cần ADR mới.

## Assumptions

- `ref.login_activity` (projection từ module game) có sau khi F014 phát event đăng nhập; trước đó chỉ số "lâu không vào game" hiển thị "chưa có dữ liệu".
- Số thông báo an toàn chưa xác nhận đọc qua facade `chat` khi F122 Done [Confirmed]; trước đó trả `null`, không mock (R17).
- Read replica bắt buộc ở staging/production; local dùng primary với role `learning_dashboard` [Confirmed].
- Ngưỡng `dashboard_attention_thresholds` theo tenant [Confirmed]; các tham số dashboard khác [Confirmed].

# Feature Specification: F043 — Rollup analytics và School Learning Dashboard API

**Feature Branch**: `043-school-learning-dashboard` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C3 · **Phụ thuộc**: F031, F041
**Input**: Nhà trường (School Admin, Academic Manager) theo dõi bài giao, LO yếu, học sinh cần hỗ trợ và hiệu quả tự học theo trường → lớp → học sinh; Tenant Admin và System Admin xem bản tổng hợp. Mọi số liệu đọc rollup dựng gia tăng, không truy vấn dữ liệu thô (ADR-0062, ADR-0072). FR nguồn: FR-ANL-003, FR-ANL-005, FR-ANL-012, FR-ANL-013, FR-ANL-016, FR-LMS-003, FR-LMS-011, FR-PRG-001, NFR-PERF-017, NFR-PERF-018.

## Clarifications

### Session 2026-11-16

- Q: Bảng rollup đặt tên và cột theo DATABASE_DESIGN §2.7 (`progress_daily(date…)`, `class_objective_rollups`) hay DASHBOARD_DATA? → A: Theo DASHBOARD_DATA §3 và LEARNING_ANALYTICS §5 (`bucket_date`, `class_id` snapshot, `metric_version`, `is_final`, `rm_class_objective_stats`, `rollup_class_objective_weekly`); DATABASE_DESIGN cập nhật trong task Phase 2 (R33: DASHBOARD_DATA sở hữu cơ chế dashboard).
- Q: Hồ sơ học tập một học sinh (US-LMS-005) có trong feature? → A: Có, qua endpoint `GET /lms/students/{student_id}/analytics` (LEARNING_ANALYTICS §8), contract cần thêm.
- Q: Phân tích đề/bài (US-LMS-007) và xuất báo cáo thuộc feature nào? → A: F044 (báo cáo đề, export); F043 chỉ cung cấp rollup và API đọc dashboard.

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Pipeline rollup gia tăng và chốt ngày (Priority: P1)

Mỗi câu trả lời, bài nộp, thay đổi mastery và trạng thái bài giao được cộng dồn gần thời gian thực vào rollup theo học sinh × ngày, lớp, trường, tenant; job chốt ngày tính chỉ số không cộng dồn và đặt cờ đã chốt.

**Why this priority**: Mọi dashboard đọc rollup; không có rollup thì không có dashboard (FR-ANL-005, FR-ANL-013).

**Independent Test**: Chạy golden dataset qua consumer + job chốt ngày → số liệu rollup khớp tính tay; gửi trùng event không đếm hai lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F043-AC-01 | KHI nhận event học tập (`question.answered`, `activity.submitted`, `activity.graded`, `exam.graded`, `attempt.re_evaluated`, `assignment.*`, `learning_objective.*`), consumer rollup PHẢI upsert cộng dồn vào dòng của ngày event ở mọi cấp (học sinh, lớp theo `class_id` snapshot lúc event, trường, tenant) với `is_final = false` | FR-ANL-005 · [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-004 |
| F043-AC-02 | KHI cùng `event_id` được giao lại, consumer PHẢI không cộng lần hai (`processed_events`) | ADR-0042 · [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-005, TC-SVC-003 |
| F043-AC-03 | KHI job chốt ngày chạy cho một tenant, HỆ THỐNG PHẢI tính chính xác chỉ số distinct (học sinh hoạt động), đặt `is_final = true`, dựng lại tuần/tháng chứa ngày đó từ bảng ngày và tăng phiên bản cache của tenant | FR-ANL-013 · [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-004 |
| F043-AC-04 | NẾU event đến muộn trong `dashboard_late_window_hours`, THÌ HỆ THỐNG PHẢI cộng vào đúng ngày và đánh dấu ngày cần chốt lại; NẾU muộn hơn cửa sổ, THÌ HỆ THỐNG PHẢI không sửa rollup và tăng metric `eduworld_rollup_late_events_total` | [DASHBOARD_DATA §5](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-005 |
| F043-AC-05 | HỆ THỐNG PHẢI tính tỷ lệ (accuracy, `rapid_guess`, nộp đúng hạn) từ tử số/mẫu số lưu riêng, loại lần trả lời `rapid_guess` khỏi accuracy và mastery, và gắn `metric_version` cho mọi dòng | FR-PRG-001 · [LEARNING_ANALYTICS §4](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-DASH-004 |
| F043-AC-06 | HỆ THỐNG PHẢI làm mới read model trạng thái hiện tại (phân bố level lớp × LO, tổng hợp lớp × môn, trường × khối × môn) trong `school_dashboard_refresh` và snapshot mastery theo tuần | FR-ANL-003 · [LEARNING_ANALYTICS §5](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-LMS-003 |

---

### User Story 2 — School Learning Dashboard cấp trường và lớp (Priority: P1)

`ad.truong.a` mở dashboard trường: bài giao đang mở/sắp hạn, LO cần củng cố, học sinh cần chú ý, tự học & gợi ý, thông báo an toàn; drill-down sang lớp (heatmap LO × học sinh, cờ "làm quá nhanh").

**Why this priority**: Trả lời năm câu hỏi hằng tuần của nhà trường (US-LMS-002, US-LMS-003, US-LMS-004).

**Independent Test**: `ad.truong.a` gọi `getSchoolDashboard` và `getClassDashboard` cho 3A → số liệu khớp truy vấn đối soát; `ad.truong.b` gọi với `school_id` TH Demo A → 404.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F043-AC-07 | KHI người có `analytics.school.read` gọi `getSchoolDashboard`, module learning PHẢI trả trong một lần gọi các khối: bài giao mở/sắp hạn với tỷ lệ nộp theo lớp, LO cần củng cố, học sinh cần chú ý, tự học & gợi ý, số thông báo an toàn chưa xác nhận, kèm `grain`, `data_as_of`, `is_partial` | FR-LMS-011 · US-LMS-003 · [SCHOOL_LMS §8.1](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F043-AC-08 | HỆ THỐNG PHẢI đưa học sinh vào danh sách "cần chú ý" theo `dashboard_attention_thresholds` (bài `missed`, LO `beginning`, lâu không vào game, tỷ lệ `rapid_guess` hiển thị là "làm quá nhanh") | US-LMS-004 · AC-PRG-002 · [SCHOOL_LMS §8.1](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F043-AC-09 | HỆ THỐNG PHẢI trả khối tự học & gợi ý gồm thời gian học trung bình/học sinh, tỷ lệ học sinh bắt đầu hoạt động gợi ý trong 7 ngày và số LO lên mức sau ôn tập gợi ý (`recommendation_follow_rate`) | US-LMS-009 · [LEARNING_ANALYTICS §4](../../docs/02-learning/LEARNING_ANALYTICS.md) | — |
| F043-AC-10 | KHI gọi `getClassDashboard`, HỆ THỐNG PHẢI trả tiến độ bài giao, phân bố mức mastery lớp × LO (heatmap) và danh sách học sinh phân trang cursor ≤ `dashboard_page_size_max` | US-LMS-002 · [SCHOOL_LMS §8.3](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003, TC-DASH-002 |
| F043-AC-11 | HỆ THỐNG PHẢI trả Learning Score, Mastery và XP/Game Rating tách biệt, không có cột XP, rating hay xếp hạng học sinh theo điểm học tập, không chứa nội dung chat | FR-ANL-003 · ADR-0018 · [SCHOOL_LMS §9](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F043-AC-12 | KHI gọi `getSchoolAnalyticsSeries` với `metric` hợp lệ, HỆ THỐNG PHẢI trả chuỗi ≤ 120 điểm theo độ hạt server chọn | FR-ANL-012 · [DASHBOARD_DATA §8](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-003 |
| F043-AC-13 | HỆ THỐNG PHẢI giới hạn `school_admin` trong trường của UserRole và `academic_manager` trong trường/môn được gán; ngoài phạm vi trường PHẢI trả `404 NOT_FOUND`, môn ngoài phạm vi PHẢI trả `403 PERMISSION_DENIED` | AC-ORG-002 · AC-ORG-003 | TC-TEN-002, TC-TEN-003 |
| F043-AC-14 | NẾU người gọi là `safety_moderator`, học sinh, hoặc thiếu `analytics.school.read`, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` | [SCHOOL_LMS §13](../../docs/04-admin-portal/SCHOOL_LMS.md) (AC-LMS-018) · AGENTS R7 | — |
| F043-AC-15 | NẾU `school_id`/`class_id` thuộc tenant khác, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND`; khóa cache PHẢI chứa `tenant_id` và không trả dữ liệu chéo tenant | AC-ORG-001 | TC-DASH-009, TC-TEN-001 |

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

**Why this priority**: Hỗ trợ đúng học sinh cần giúp (US-LMS-005), sau dashboard trường/lớp.

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

- NẾU học sinh chuyển lớp giữa kỳ, THÌ số liệu cũ giữ ở lớp cũ theo `class_id` snapshot (F043-AC-01).
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
| FR-ANL-003 | Analytics theo học sinh, lớp, trường, môn, chủ đề, LO | [LEARNING_ANALYTICS §5, §7](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-ANL-005 | Aggregation trong worker, dashboard đọc read model | [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-012 | API dashboard trường → lớp → học sinh, tenant, platform (phần học tập) | [DASHBOARD_DATA §2, §14](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-013 | Chỉ đọc rollup, khoảng ≤ 2 năm, độ hạt tự chọn, `data_as_of`, `is_partial` | [DASHBOARD_DATA §8](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-016 | Ẩn ô < 5 học sinh ở báo cáo tenant/platform | [LEARNING_ANALYTICS §5](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-LMS-003 | Theo dõi kết quả, tiến độ, kỹ năng yếu theo LO | [SCHOOL_LMS §8](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-011 | School Learning Dashboard API `getSchoolDashboard` | [SCHOOL_LMS §8](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-PRG-001 | Chỉ số tiến độ (bài hoàn thành, accuracy, thời gian, LO yếu, tiến bộ theo thời gian) | [LEARNING_ANALYTICS §4](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| NFR-PERF-017 | p95 truy vấn dashboard ≤ 2 s trên dữ liệu 2 năm, không truy vấn > 5 s | [DASHBOARD_DATA §15](../../docs/07-backend/DASHBOARD_DATA.md) |
| NFR-PERF-018 | Chỉ đọc rollup trên replica, không ảnh hưởng database nghiệp vụ | [DASHBOARD_DATA §7](../../docs/07-backend/DASHBOARD_DATA.md) |

### Key Entities

- **Rollup chuỗi thời gian**: số đo cộng dồn theo phạm vi (học sinh, lớp, lớp × LO, trường × khối × môn, tenant × khối × môn) × ngày/tuần/tháng, có tử/mẫu số, `metric_version`, cờ đã chốt, thời điểm tính.
- **Read model trạng thái hiện tại**: phân bố level mastery lớp × LO, tổng hợp lớp × môn, trường × khối × môn, snapshot mastery tuần.
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
- Số thông báo an toàn chưa xác nhận đọc qua facade `chat` khi F122 Done [Proposed]; trước đó trả `null`, không mock (R17).
- Read replica bắt buộc ở staging/production; local dùng primary với role `learning_dashboard` [Proposed].
- Ngưỡng `dashboard_attention_thresholds` theo tenant [Confirmed]; các tham số dashboard khác [Proposed].

# Feature Specification: F056 — Workspace C — School Learning Dashboard và báo cáo

**Feature Branch**: `056-portal-school-dashboard` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/cms/frontend/apps/portal` (workspace C) · **Mốc**: M-C3 · **Phụ thuộc**: F043, F044, F052
**Input**: Màn School Learning Dashboard (trường → lớp → học sinh), bản đồ kỹ năng LO, hồ sơ học sinh, bảng kết quả course, phân tích đề/bài, xuất báo cáo bất đồng bộ (gồm báo cáo tham khảo TT27/TT22), tổng hợp tenant và platform. FR nguồn: FR-LMS-003, FR-LMS-011, FR-LMS-013, FR-ANL-012, FR-ANL-013, FR-ANL-014, FR-ANL-015.

## Clarifications

### Session 2026-11-16

- Q: Dashboard đọc dữ liệu từ đâu? → A: Chỉ API dashboard đọc rollup/read model (ADR-0062); Portal không tự tổng hợp từ danh sách nghiệp vụ.
- Q: Ngưỡng "Cần chú ý" (`dashboard_attention_thresholds`) áp ở đâu? → A: Server tính; Portal chỉ hiển thị `reason` trả về.
- Q: Hồ sơ học sinh và phân tích đề dùng endpoint nào? → A: Chưa có trong OpenAPI; F056 ghi "Cần thêm" (`getStudentLearningProfile`, `getExamSessionReport`, `getAssignmentItemAnalysis`) và tạo task Phase 2 cho owner F043/F044.
- Q: Biểu đồ dùng thư viện nào? → A: ECharts (heatmap LO theo [SCHOOL_LMS §8.7](../../docs/04-admin-portal/SCHOOL_LMS.md)); màu từ `admin-tokens`.

### Session 2026-11-17

- Q: Tên thư mục workspace và component ô bị ẩn? → A: Workspace C là `school-lms` (ADMIN_PORTAL §1.2, RB-619); ô < 5 dùng **một** component `SuppressedCell` (DESIGN_SYSTEM §4) do F056 tạo, F080/F081 dùng lại (thay `SuppressedValue`).

### Session 2026-11-20 (owner chốt)

- Endpoint hồ sơ học sinh và phân tích đề/bài (contracts §6, Assumptions "chờ owner F043/F044 chốt") → **Chốt:** theo đề xuất ở contracts §6; F043 làm `getStudentLearningProfile` và `SchoolDashboard.widgets.self_study`, F044 làm `getExamSessionReport`/`getAssignmentItemAnalysis` (đã có trong spec/tasks của hai feature).

### Session 2026-11-21 (triển khai — [Confirmed], owner chốt 2026-11-20)

Quyết định khi triển khai, chọn phương án bảo thủ nhất (owner đã duyệt trước mặc định đề xuất):

- **Phân tích đề/bài (F056-AC-13):** F044 chỉ thêm `getExamSessionReport`; `getAssignmentItemAnalysis` **không** có trong OpenAPI. Không thêm endpoint mới: tab Phân tích của bài `graded` đọc `Assignment.exam_session_id` → `getExamSessionReport` (phổ điểm, trung vị, tỉ lệ đạt, p-value loại `rapid_guess`, `top_misconceptions`, cảnh báo độ tin cậy/chênh biến thể). Bài không có phiên thi hiện "chưa có phân tích theo câu". Contract không cho biết học sinh nào sai từng câu, nên nút "Giao ôn tập cho học sinh sai câu này" mở wizard F055 `mode=practice` với lớp của bài (không điền danh sách học sinh). Câu hỏi mở F056-1.
- **Thử lại `DASHBOARD_BUSY` (F056-AC-04):** chỉ một lớp thử lại là apiClient: `429 DASHBOARD_BUSY` chờ `Retry-After` kẹp trong 2–5 giây, tối đa `MAX_RETRIES` = 2 lần; `504 DASHBOARD_QUERY_TIMEOUT` không lặp lại (gợi ý thu hẹp phạm vi). `useDashboardQueue` (ops-ui) chỉ giới hạn ≤ `dashboard_max_concurrent_per_user` truy vấn đồng thời mỗi trang.
- **Preset "Học kỳ này"/"Năm học này" (F056-AC-03):** API công khai chưa có ngày năm học/học kỳ của trường (bảng `academic_years` không có operation đọc). Hai preset hiển thị nhưng bị vô hiệu kèm tooltip "Chưa có lịch năm học của trường"; `DashboardRangePicker` nhận lịch qua prop khi contract có. Câu hỏi mở F056-2.
- **"Đang tính số liệu…", `available_from`, cảnh báo trễ replica (F056-AC-02, Edge Cases):** `DashboardMeta` chỉ có `grain`, `data_as_of`, `is_partial`. Portal hiển thị "Đang tính số liệu…" cho giá trị `null` khi `is_partial = true` (không bao giờ vẽ `null` thành 0); "Chưa có số liệu trước ngày …" và cảnh báo trễ chưa làm vì contract chưa có trường — Câu hỏi mở F056-3.
- **Heatmap (F056-AC-09):** `MasteryHeatmap` là bảng HTML (caption, tiêu đề hàng/cột, ô focus được bằng bàn phím, `aria-label` + tooltip giá trị chính xác, ký hiệu ○◔◑◕●) tô màu bằng token ngữ nghĩa Admin (`colorErrorBg`, `colorWarningBg`, `colorInfoBg`, `colorSuccessBg`, `colorFillSecondary`) — không thêm mã màu nghiệp vụ mới. ECharts 5.6.0 (tải theo nhu cầu, chunk riêng) dùng cho biểu đồ cột/đường (phổ điểm, xu hướng 8 tuần, phút học), luôn kèm `aria-label` liệt kê giá trị. Đã cập nhật SCHOOL_LMS §8.7.
- **Lý do "Cần chú ý" (F056-AC-05):** dùng enum contract `AttentionReason` (`missed_assignments`, `beginning_objectives`, `inactive_game`, `rapid_guess` → "Làm quá nhanh", `scheduled_cleanup`) thay cho tên trong văn bản AC (`inactive`, `low_accuracy`, `rapid_guessing`). Nhãn mức mastery theo LEARNING_EXPERIENCE §7 [Confirmed] (Mới bắt đầu, Đang tiến bộ, Thành thạo, Xuất sắc; `not_started` → "Chưa bắt đầu").
- **Route và màn:** đường dẫn lấy từ danh mục menu (ADR-0090): `/school-lms/school-dashboard` (tổng quan, `classes/:id`, `students/:id`, `reference` — mức tham khảo TT27/TT22 từ `getRegulationReferenceReport`), `/school-lms/results` (khóa học → `:courseId` → `:courseId/analysis/:assignmentId`), `/school-lms/reports` (tab "Xuất báo cáo" theo `action.school_lms.reports.export`, tab "Tổng hợp đơn vị" theo `action.school_lms.reports.view`), `/platform-admin/reports`. Thay giả định `/portal/school-lms/dashboard/*`.
- **Tên học sinh:** `StudentLearningProfile` và `ClassDashboard.students` không có họ tên; Portal lấy tên từ `listTenantClassStudents` của lớp đang xem (F052), thiếu thì hiện "Học sinh". Tên môn ở tổng hợp platform lấy từ `listCmsSubjects` (không có quyền thì hiện mã).
- **Tham số:** giá trị `dashboard_*`, `school_dashboard_refresh` phạm vi `code` mirror trong `dashboard/model/limits.ts`, test so với CONFIG_DEFAULTS (R33).
- **Mock test:** theo mẫu repo (`MockPortalBackend` + `src/test/dashboard-mock.ts`, nhãn MOCK) thay cho MSW `__tests__/msw/handlers.ts` của T010.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Trang tổng quan trường (Priority: P1) 🎯 MVP

`ad.truong.a` mở tổng quan TH Demo A tuần này và trả lời 5 câu hỏi: bài đã làm chưa, LO yếu, ai cần hỗ trợ, có tự học sau gợi ý không, có thông báo an toàn chờ xác nhận không (US-LMS-003, US-LMS-009, US-MOD-004).

**Why this priority**: Đây là màn nhà trường mở mỗi tuần; là kịch bản demo M-C3.

**Independent Test**: Với dữ liệu seed sau khi 3 học sinh làm bài, mở tổng quan, đổi khoảng thời gian, kiểm 5 widget, nhãn "Cập nhật lúc", "Tạm tính".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F056-AC-01 | KHI người dùng có `analytics.school.read` mở Tổng quan, Portal PHẢI gọi `getSchoolDashboard` với `school_id`, `from`, `to` và bộ lọc khối/lớp/môn, rồi hiển thị widget theo thứ tự 5 câu hỏi: bài giao đang mở/sắp hạn, LO cần củng cố, cần chú ý, tự học và gợi ý, thông báo an toàn | FR-LMS-011 · FR-LMS-003 · [SCHOOL_LMS §8.1](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F056-AC-02 | HỆ THỐNG PHẢI hiển thị trên mỗi widget "Cập nhật lúc HH:mm" từ `data_as_of`, nhãn độ hạt từ `grain`, nhãn "Tạm tính" khi `is_partial = true`, và "Đang tính số liệu…" thay cho số 0 khi ngày chưa chốt theo `dashboard_day_close_time` | FR-ANL-013 · AC-DASH-006 · [ADMIN_PORTAL §3.14.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-DASH-003 |
| F056-AC-03 | HỆ THỐNG PHẢI cung cấp bộ chọn khoảng thời gian với preset (Hôm nay, 7 ngày, 30 ngày, Học kỳ này, Năm học này, 12 tháng) và khoảng tùy chọn không vượt `dashboard_max_range_days`; ngày ngoài giới hạn bị vô hiệu; không có "toàn bộ thời gian"; người dùng chỉ chọn được độ hạt thô hơn quy tắc `dashboard_grain_daily_max_days`/`dashboard_grain_weekly_max_days` | FR-ANL-013 · AC-DASH-002 · AC-DASH-003 | TC-DASH-002 |
| F056-AC-04 | NẾU API trả `422 DASHBOARD_RANGE_TOO_LARGE` hoặc `504 DASHBOARD_QUERY_TIMEOUT`, THÌ widget PHẢI hiển thị gợi ý thu hẹp khoảng/phạm vi; NẾU trả `429 DASHBOARD_BUSY`, THÌ widget PHẢI tự thử lại sau 2–5 giây tối đa 2 lần; mỗi widget tải độc lập (skeleton riêng) và một trang không gửi quá `dashboard_max_concurrent_per_user` truy vấn đồng thời | FR-ANL-013 · AC-DASH-004 · AC-DASH-005 | TC-DASH-006 |
| F056-AC-05 | KHI hiển thị "Cần chú ý", Portal PHẢI liệt kê học sinh với nhãn theo `reason` (`inactive`, `low_accuracy`, `missed_assignments`, `rapid_guessing` → "làm quá nhanh") bằng ngôn ngữ trung tính, không xếp hạng học sinh theo điểm | FR-LMS-011 · AC-LMS-017 · [SCHOOL_LMS §8.8](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F056-AC-06 | KHI giá trị có `suppressed = true` hoặc ô dưới 5 học sinh ở báo cáo cấp tenant/platform, Portal PHẢI hiển thị "< 5" thay cho số bằng component dùng chung `SuppressedCell` (`src/cms/frontend/packages/ops-ui/src/dashboard/`, kèm tooltip) | FR-ANL-016 · AC-DASH-011 | TC-DASH-009 |
| F056-AC-07 | Widget an toàn PHẢI chỉ hiển thị số thông báo chưa xác nhận (`safety_notifications_unacknowledged`) và liên kết tới màn thông báo an toàn; HỆ THỐNG PHẢI không hiển thị nội dung chat ở bất kỳ màn nào của F056 | AC-LMS-018 · AGENTS R14, R16 | — |

---

### User Story 2 — Drill-down lớp, bản đồ kỹ năng, hồ sơ học sinh (Priority: P1)

School Admin đi từ trường → lớp 3B → học sinh, xem heatmap lớp × LO và học sinh × LO, rồi giao ôn tập cho nhóm yếu LO "Chia có dư" trong 2 thao tác (US-LMS-002, US-LMS-005, US-LMS-006).

**Why this priority**: Trả lời câu hỏi 2–3 ở mức hành động được; nối sang wizard giao bài (F055).

**Independent Test**: Từ tổng quan bấm lớp 3A → heatmap → ô LO → evidence; mở hồ sơ `hs.an.3a`; bấm "Giao ôn tập theo LO".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F056-AC-08 | KHI người dùng chọn một lớp, Portal PHẢI gọi `getClassDashboard` (hoặc `getSchoolDashboard` có `class_id`) giữ nguyên khoảng thời gian, môn, và hiển thị breadcrumb trường → lớp → học sinh | FR-LMS-011 · AC-LMS-010 | TC-LMS-003 |
| F056-AC-09 | HỆ THỐNG PHẢI vẽ heatmap LO bằng component `MasteryHeatmap` (ops-ui) với ký hiệu mức (○◔◑◕●) kèm màu từ token, có nhãn cho trình đọc màn hình và tooltip giá trị chính xác; không dùng màu là kênh duy nhất | FR-LMS-011 · NFR-A11Y-003 · [DESIGN_SYSTEM §4.4](../../docs/05-ux/DESIGN_SYSTEM.md) | TC-UI-006 |
| F056-AC-10 | KHI người dùng có `lms.assignment.write` bấm "Giao ôn tập theo LO" trên một LO yếu, Portal PHẢI mở wizard F055 đã điền `mode = practice`, LO, lớp và danh sách học sinh yếu LO đó, để hoàn tất trong 2 thao tác | US-LMS-006 · [SCHOOL_LMS §8.3](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F056-AC-11 | KHI người dùng mở hồ sơ học sinh, Portal PHẢI hiển thị mastery theo LO/topic, xu hướng 8 tuần, lịch sử bài, gợi ý hệ thống đang hiển thị (từ `getStudentLearningProfile`) và tab Hoạt động đăng nhập từ `listTenantStudentLoginEvents`; PHẢI không hiển thị Game Rating, vị trí trong game, bạn bè, chat, IP hay mã thiết bị | US-LMS-005 · AC-LMS-015 · [SCHOOL_LMS §8.5](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |

---

### User Story 3 — Bảng kết quả và phân tích đề/bài (Priority: P2)

Academic Manager xem bảng kết quả course Toán 3A và phân tích đề "Phép nhân": phổ điểm, câu nhiều học sinh sai (US-LMS-007).

**Why this priority**: Phục vụ họp chuyên môn; cần dữ liệu đề đã chấm (F044).

**Independent Test**: Mở course Toán 3A → tab Kết quả; mở bài graded → tab Phân tích.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F056-AC-12 | KHI người dùng mở tab Kết quả của course, Portal PHẢI hiển thị `getCourseResults` (cột = bài `graded`, giá trị `score_10` theo `score_scale`, trạng thái nộp), phân trang cursor không quá `dashboard_page_size_max`; không có cột XP, rating, điểm thi đấu hay ô nhập điểm tay | FR-LMS-013 · [SCHOOL_LMS §10](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F056-AC-13 | KHI người dùng mở tab Phân tích của bài/đề, Portal PHẢI hiển thị phổ điểm, trung vị, tỉ lệ đạt, p-value theo câu, lựa chọn sai phổ biến, tỉ lệ `rapid_guess` (đã loại khỏi p-value) và cảnh báo độ tin cậy thấp khi server trả, kèm nút "Giao ôn tập cho học sinh sai câu này" mở wizard F055 | US-LMS-007 · AC-LMS-017 · [SCHOOL_LMS §8.6](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-EXM-004 |

---

### User Story 4 — Xuất báo cáo bất đồng bộ (Priority: P2)

Academic Manager xuất kết quả bài giao (XLSX) và báo cáo tham khảo TT27/TT22 để dùng trong họp chuyên môn (US-LMS-010).

**Why this priority**: FR-ANL-014, FR-LMS-013; không chặn xem dashboard.

**Independent Test**: Bấm Xuất → thông báo "Đang chuẩn bị" → link tải xuất hiện, hết hạn đúng thời gian; mở 3 export cùng lúc.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F056-AC-14 | KHI người dùng có `report.export` bấm Xuất, Portal PHẢI gọi `createLmsReport` (`report_type`, `format`, phạm vi) với `Idempotency-Key`, hiển thị "Đang chuẩn bị báo cáo", theo dõi `getLmsReport` và chỉ hiện link khi job xong kèm thời điểm hết hạn theo `dashboard_export_link_ttl_minutes`; không tải file trực tiếp từ request xuất | FR-ANL-014 · FR-LMS-013 · AC-DASH-009 | TC-DASH-008 |
| F056-AC-15 | NẾU server trả `429 EXPORT_LIMIT_REACHED` (giới hạn `dashboard_export_max_concurrent_per_user`) hoặc link đã hết hạn (`403`), THÌ Portal PHẢI hiển thị thông điệp theo `code` và cho tạo lại khi được phép | FR-ANL-014 · AC-DASH-009 | TC-DASH-008 |
| F056-AC-16 | Ở NƠI báo cáo là tham khảo theo TT27/TT22 (`report_type = regulation_reference`) hoặc màn hiển thị mức tham khảo, Portal PHẢI hiển thị nhãn "Tham khảo — không thay đánh giá chính thức" và mức "Chưa đủ dữ liệu" khi server trả như vậy | FR-ANL-015 · AC-ARP-003 · [ASSESSMENT_REGULATION_PROFILE §5](../../docs/02-learning/ASSESSMENT_REGULATION_PROFILE.md) | — |

---

### User Story 5 — Tổng hợp tenant và platform (Priority: P3)

`ad.donvi` xem tổng hợp học tập các trường của đơn vị; `sa.admin` xem tổng hợp liên tenant (FR-ANL-012).

**Why this priority**: Cấp tổng hợp; dữ liệu ngày đã chốt.

**Independent Test**: `ad.donvi` mở Tổng hợp đơn vị, lọc trường; `sa.admin` lọc 2 tenant.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F056-AC-17 | KHI người dùng có `analytics.tenant.read` mở Tổng hợp đơn vị, Portal PHẢI gọi `getTenantLearningSummary` (lọc `school_ids`, khối, khoảng), hiển thị "< 5" cho ô ẩn và cho xuất qua `createTenantReport`/`getTenantReport` | FR-ANL-012 · FR-ANL-016 · [DASHBOARD_DATA §14](../../docs/07-backend/DASHBOARD_DATA.md) | TC-ANL-005 |
| F056-AC-18 | Ở NƠI người dùng là System Admin, Portal PHẢI hiển thị tổng hợp platform từ `getPlatformLearningSummary` với `tenant_ids[]` từ bộ lọc tenant (F057), cột Tenant ở mọi bảng, và xuất qua `createPlatformReport`/`getPlatformReport` | FR-ANL-012 · FR-CMS-013 | TC-CMS-TEN-02 |

---

### User Story 6 — Phân quyền và cô lập tenant trên dashboard (Priority: P1)

**Why this priority**: Dữ liệu học tập của học sinh; constitution điều III, V, VI.

**Independent Test**: `qlcm.truong.a`, `ad.truong.b`, `sm.antoan`, `cm.noidung` mở dashboard TH Demo A.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F056-AC-19 | HỆ THỐNG PHẢI chỉ hiện menu Tiến độ & báo cáo khi `getMyPermissions` có `analytics.school.read`/`analytics.tenant.read`; bộ lọc môn của `academic_manager` chỉ gồm môn trong scope; NẾU API trả `403` THÌ Portal PHẢI hiển thị "Ngoài phạm vi quyền" không kèm dữ liệu | FR-LMS-005 · AC-LMS-003 · AGENTS R7 | TC-TEN-002 |
| F056-AC-20 | KHI người dùng không có permission analytics (Safety Moderator, Content Admin) mở URL dashboard, Portal PHẢI hiển thị trang 403 của portal và không gọi API dashboard | AC-LMS-018 · [ADMIN_PORTAL §2.5b](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-01 |
| F056-AC-21 | HỆ THỐNG PHẢI gửi `X-Tenant-Id` = tenant phiên; KHI `school_id`/`class_id`/`student_id` thuộc tenant hoặc trường khác trả `404`, Portal PHẢI hiển thị "Không tìm thấy" không tiết lộ sự tồn tại; cache truy vấn phía client có khóa chứa tenant | AC-LMS-012 · AC-DASH-011 · AGENTS R8 | TC-DASH-009 |
| F056-AC-22 | HỆ THỐNG PHẢI giới hạn ô tìm kiếm/bộ lọc nhập tay theo `maxLength` của tham số OpenAPI | ADR-0041 | TC-TEN-024 |
| F056-AC-23 | KHI menu/action của feature (`menu.school_lms.school_dashboard`, `menu.school_lms.results`, `menu.school_lms.reports`, `menu.platform_admin.reports` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU trường chưa có dữ liệu (đầu năm), THÌ widget PHẢI hiển thị trạng thái rỗng có hướng dẫn, không lỗi.
- NẾU chỉ số mới có `available_from`, THÌ PHẢI hiển thị "Chưa có số liệu trước ngày …" (F056-AC-02).
- NẾU replica trễ và server trả dữ liệu kèm cảnh báo trễ, THÌ Portal PHẢI hiển thị cảnh báo cạnh "Cập nhật lúc".
- NẾU người dùng đổi tenant/đơn vị khi export đang chạy, THÌ Portal PHẢI bỏ theo dõi job của tenant cũ.
- NẾU lớp có > `dashboard_page_size_max` học sinh trong bảng, THÌ PHẢI phân trang cursor.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-LMS-003 | Theo dõi kết quả, tiến độ, LO yếu | [SCHOOL_LMS §8](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-011 | UI School Learning Dashboard trường → lớp → học sinh | [SCHOOL_LMS §8](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-013 | Bảng kết quả tự chấm, xuất XLSX/CSV | [SCHOOL_LMS §10](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-ANL-012 | Dashboard trường, tenant, platform | [DASHBOARD_DATA §2](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-013 | Quy tắc UI: khoảng ≤ 2 năm, độ hạt, `data_as_of`, tạm tính | [ADMIN_PORTAL §3.14.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ANL-014 | Xuất báo cáo bất đồng bộ, link có hạn | [DASHBOARD_DATA §10](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-015 | Hiển thị báo cáo tham khảo TT27/TT22 có nhãn | [ASSESSMENT_REGULATION_PROFILE](../../docs/02-learning/ASSESSMENT_REGULATION_PROFILE.md) |
| FR-ANL-016 | Ẩn ô < 5 học sinh ở cấp tổng hợp | [LEARNING_ANALYTICS](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **SchoolDashboard**: gói widget của trường/lớp kèm `grain`, `data_as_of`, `is_partial`.
- **ClassDashboard**: phân bố mastery theo LO của lớp và học sinh cần chú ý.
- **StudentLearningProfile**: mastery theo LO, xu hướng, lịch sử bài, gợi ý đang hiển thị của một học sinh.
- **CourseResults**: bảng điểm tự chấm các bài graded của course.
- **ItemAnalysis**: thống kê theo câu của một đề/bài.
- **ReportJob**: job xuất báo cáo, trạng thái và link ký có hạn.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `dashboard_max_range_days`, `dashboard_grain_daily_max_days`, `dashboard_grain_weekly_max_days` | Giới hạn bộ chọn khoảng và độ hạt |
| `dashboard_max_concurrent_per_user` | Giới hạn truy vấn song song của một trang |
| `dashboard_page_size_max` | Kích thước trang bảng chi tiết |
| `dashboard_day_close_time` | Nhãn "Đang tính số liệu…" |
| `dashboard_attention_thresholds` | Ngưỡng "Cần chú ý" (server áp) |
| `school_dashboard_refresh` | Chu kỳ làm mới số liệu tuần |
| `dashboard_export_link_ttl_minutes`, `dashboard_export_max_concurrent_per_user`, `report_link_ttl` | Hiển thị hạn link, lỗi giới hạn export |
| `score_scale` | Thang điểm hiển thị |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Trang tổng quan hiển thị đầy đủ widget trong mục tiêu p95 của `school_dashboard_refresh`/NFR | NFR-PERF-017 · NFR-PERF-018 |
| Không business logic/tổng hợp số liệu ở client | AGENTS R1 · ADR-0062 |
| Chart theo DESIGN_SYSTEM §4.4, màu từ token | AGENTS R32 · ADR-0047 |
| WCAG 2.1 AA; heatmap có ký hiệu | NFR-A11Y-003 |
| Không log PII; không hiển thị dữ liệu ngoài scope | AGENTS R14, R16 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: School Admin trả lời được 5 câu hỏi tuần từ một màn tổng quan mà không cần mở màn khác (kiểm bằng kịch bản quickstart).
- **SC-002**: Từ LO yếu tới bài ôn tập đã giao trong ≤ 2 thao tác (bấm "Giao ôn tập theo LO" → "Giao bài").
- **SC-003**: 100% widget hiển thị thời điểm dữ liệu; 0 widget hiển thị số 0 khi dữ liệu đang tính.
- **SC-004**: 0 trường hợp hiển thị dữ liệu ngoài tenant/scope trong test ma trận quyền; 0 lỗi axe serious/critical.

## Ngoài phạm vi

- Rollup, read model và API dashboard: F043; export worker, item analysis, chất lượng đề: F044.
- Wizard giao bài: F055 (F056 chỉ mở với tham số điền sẵn).
- Màn thông báo an toàn và xác nhận liên hệ phụ huynh: F122 (workspace C, phần nhận); hoạt động đăng nhập danh sách lớp, dọn học sinh: F052.
- Báo cáo hoạt động game của tenant (Game Analytics tổng hợp): F080.
- Bộ lọc tenant liên tenant: F057; shell: F051.

## Assumptions

- Thư mục `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/`, route `/portal/school-lms/dashboard/*` (ADMIN_PORTAL §1.2, F051).
- [Confirmed] Endpoint hồ sơ học sinh và phân tích đề/bài theo đề xuất ở [contracts §6](contracts/README.md) — owner chốt 2026-11-20 (F043/F044 làm, đã có trong spec/tasks của hai feature); US2 phần hồ sơ và US3 phần phân tích chỉ chạy sau delta.
- Số liệu "tự học sau gợi ý" (câu hỏi 4) cần trường bổ sung trong `SchoolDashboard` (delta [Confirmed]); trước khi có, widget ẩn kèm "Chưa có số liệu".

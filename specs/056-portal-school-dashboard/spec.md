# Feature Specification: F056 — Workspace C — School Learning Dashboard và báo cáo

**Feature Branch**: `056-portal-school-dashboard` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/cms/frontend/apps/portal` (workspace C) · **Mốc**: M-C3 · **Phụ thuộc**: F043, F044, F052
**Input**: Màn School Learning Dashboard (trường → khối → học sinh — ADR-0095), bản đồ kỹ năng LO, hồ sơ học sinh, kết quả theo bài giao, phân tích đề/bài, xuất báo cáo bất đồng bộ (gồm báo cáo tham khảo TT27/TT22), tổng hợp tenant và platform. FR nguồn: FR-LMS-003, FR-LMS-011, FR-LMS-013, FR-ANL-012, FR-ANL-013, FR-ANL-014, FR-ANL-015.

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
- **T052 (2026-11-21, sau F010 T047 / F043 T042):** "Năm học này" = năm học của trường (theo bộ lọc trường) chứa hôm nay, nếu không có thì năm `is_current` đã bắt đầu (kết thúc kẹp ở hôm nay); "Học kỳ này" = học kỳ của năm đó chứa hôm nay — `listSchoolAcademicYears` hiện trả `terms` rỗng (OQ F010-4) nên preset học kỳ vẫn vô hiệu kèm tooltip; chưa chọn trường thì cả hai vô hiệu. Portal không tự suy ra ngày. `DashboardWidget` hiện "Chưa có số liệu trước ngày DD/MM/YYYY" khi `available_from` sau `from` của câu trả lời và thẻ cảnh báo "Số liệu có thể chậm do đồng bộ trễ" khi `replica_lag_warning`; số liệu vẫn hiển thị.
- **Heatmap (F056-AC-09):** `MasteryHeatmap` là bảng HTML (caption, tiêu đề hàng/cột, ô focus được bằng bàn phím, `aria-label` + tooltip giá trị chính xác, ký hiệu ○◔◑◕●) tô màu bằng token ngữ nghĩa Admin (`colorErrorBg`, `colorWarningBg`, `colorInfoBg`, `colorSuccessBg`, `colorFillSecondary`) — không thêm mã màu nghiệp vụ mới. ECharts 5.6.0 (tải theo nhu cầu, chunk riêng) dùng cho biểu đồ cột/đường (phổ điểm, xu hướng 8 tuần, phút học), luôn kèm `aria-label` liệt kê giá trị. Đã cập nhật SCHOOL_LMS §8.7.
- **Lý do "Cần chú ý" (F056-AC-05):** dùng enum contract `AttentionReason` (`missed_assignments`, `beginning_objectives`, `inactive_game`, `rapid_guess` → "Làm quá nhanh", `scheduled_cleanup`) thay cho tên trong văn bản AC (`inactive`, `low_accuracy`, `rapid_guessing`). Nhãn mức mastery theo LEARNING_EXPERIENCE §7 [Confirmed] (Mới bắt đầu, Đang tiến bộ, Thành thạo, Xuất sắc; `not_started` → "Chưa bắt đầu").
- **Route và màn:** đường dẫn lấy từ danh mục menu (ADR-0090): `/school-lms/school-dashboard` (tổng quan, `classes/:id`, `students/:id`, `reference` — mức tham khảo TT27/TT22 từ `getRegulationReferenceReport`), `/school-lms/results` (khóa học → `:courseId` → `:courseId/analysis/:assignmentId`), `/school-lms/reports` (tab "Xuất báo cáo" theo `action.school_lms.reports.export`, tab "Tổng hợp đơn vị" theo `action.school_lms.reports.view`), `/platform-admin/reports`. Thay giả định `/portal/school-lms/dashboard/*`.
- **Tên học sinh:** `StudentLearningProfile` và `ClassDashboard.students` không có họ tên; Portal lấy tên từ `listTenantClassStudents` của lớp đang xem (F052), thiếu thì hiện "Học sinh". Tên môn ở tổng hợp platform lấy từ `listCmsSubjects` (không có quyền thì hiện mã).
- **Tham số:** giá trị `dashboard_*`, `school_dashboard_refresh` phạm vi `code` mirror trong `dashboard/model/limits.ts`, test so với CONFIG_DEFAULTS (R33).
- **Mock test:** theo mẫu repo (`MockPortalBackend` + `src/test/dashboard-mock.ts`, nhãn MOCK) thay cho MSW `__tests__/msw/handlers.ts` của T010.

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: Chỉ plan/contracts nhắc MFA cho đọc liên tenant; đọc là `platform.cross_tenant.read` + client portal + audit, không MFA. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-21 (owner chốt giao diện — [Confirmed])

- Giao diện màn làm việc → **Chốt:** hướng B "Sổ tay giáo viên" áp qua token và component token `ConfigProvider` (DESIGN_SYSTEM §4.1–§4.2): nền trang ngả ấm, thẻ viền 1 px bo 12 px, chữ nội dung 15 px (compact 13 giữ nguyên lựa chọn người dùng), header/sidebar nền thẻ. Trang tổng quan dashboard có lời chào theo buổi ("Chào buổi sáng, thầy cô"), nút chính "+ Giao bài mới" cỡ lớn (vẫn bọc `ActionGate action.school_lms.assignments.edit`), mỗi widget là thẻ KPI có viền trên 4 px theo màu ngữ nghĩa (`DashboardWidget accent`), số KPI 30 px. Dữ liệu và hành vi không đổi: mọi số vẫn từ `getSchoolDashboard`, không số liệu dựng sẵn.

### Session 2026-11-22 (ADR-0094 — [Confirmed])

- Q: Văn bản còn mô tả MFA/`MFA_REQUIRED`/step-up như hành vi hiện hành? → A: [Confirmed] Viết lại theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md), giữ nguyên mã AC: plan (platform summary). Route platform chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform; thiếu → `403 PERMISSION_DENIED` + `authz.denied`; không còn `403 MFA_REQUIRED` hay hộp đăng nhập lại (step-up). Câu trả lời Clarifications cũ nhắc MFA là lịch sử.

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Drill-down còn cấp lớp? → A: **Không.** [ADR-0095](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) §4: dashboard đi **trường → khối → học sinh**. Portal bỏ trang lớp, bộ lọc lớp và mọi liên kết lớp. Tổng quan liệt kê các khối của trường (`widgets.grades`: tỉ lệ nộp, liên kết tới trang khối), tiến độ mỗi bài theo khối (`by_grade`), khối của LO yếu (`grade_levels`) và khối của học sinh cần chú ý; ô dưới ngưỡng (`suppressed`) hiện "< 5". Trang khối `grades/:grade` gọi `getGradeDashboard` (WP-L2): heatmap LO của khối (`MasteryHeatmap`), học sinh của khối (cursor, tên từ `listTenantGradeStudents`), "Giao ôn tập theo LO" điền trường/môn/khối và học sinh yếu LO; khối dưới ngưỡng: số tổng hợp "< 5", không heatmap. Breadcrumb trường → khối → học sinh.
- Q: Heatmap LO cấp khối? → A: Có, ở trang khối (`getGradeDashboard.objective_distribution`, rollup trường × khối × môn × LO của WP-L2); `MasteryHeatmap` còn ở hồ sơ học sinh (F056-AC-09).
- Q: Tên học sinh ở hồ sơ? → A: `StudentLearningProfile` không có họ tên và danh sách học sinh của lớp đã bỏ: Portal truyền tên từ dòng "Cần chú ý" qua trạng thái điều hướng (không đưa vào URL); mở hồ sơ trực tiếp bằng URL hiện "Học sinh". Thay quyết định "Tên học sinh" của Session 2026-11-21 (triển khai).
- Q: Bảng kết quả course (F056-AC-12)? → A: Course đã bỏ (ADR-0095): màn "Kết quả bài/đề" liệt kê bài giao (`listAssignments` lọc `school_id`, `subject_id`, `grade_level`) với đối tượng, số đã nộp và điểm tự chấm trung bình (`completion.average_score`); mỗi bài mở "Phân tích" (F056-AC-13) và xuất `assignment_results`. Không còn `getCourseResults`, `course_results`. Route: `/school-lms/results` → `:assignmentId`.
- Q: "Giao ôn tập theo LO" / "Giao ôn tập cho học sinh sai câu này" điền gì? → A: `mode=practice`, `objective_id` (nếu có), `school_id`, `subject_id`, `grade` — từ bộ lọc của tổng quan hoặc từ bài giao (khối đầu tiên của bài); không `class_id`.
- Q: Báo cáo? → A: Bỏ `course_results`; `skill_map` theo trường + khối + môn (`grade_level` bắt buộc — WP-L2); `regulation_reference` theo trường + môn, lọc khối tùy chọn; mức tham khảo hiện tên học sinh của khối đang lọc (`listTenantGradeStudents`), không lọc khối thì đánh số "Học sinh 1, 2, …".
- F056-AC-22 → **Removed (ADR-0095)**: ô nhập tay duy nhất của F056 là tìm học sinh trong trang lớp, đã bỏ cùng trang lớp.
- Từ ngữ: mọi chuỗi UI dùng "khối", không "lớp" (ADR-0095 §1).

### Session 2026-11-28 (golden dataset local cho T021 — [Confirmed — owner "chốt hết"])

- Q: "Golden dataset F043" cho E2E T021 trên backend compose là gì? → A: [Confirmed — owner "chốt hết"] Golden dataset của F043 (T003) là dữ liệu tổng hợp trong test tích hợp (`dashKit`), không nạp được vào stack. Bản local là `make seed-attempts` ([LOCAL_DEVELOPMENT §5.2](../../docs/11-operations/LOCAL_DEVELOPMENT.md#52-seed-mở-rộng-media-golden-dataset-make-seed-media-make-seed-attempts)): 6 attempt qua API thật (`hs.an.3a` 4/4 + 0/1, `hs.binh.3a` 3/4 + 0/1, `hs.chi.3b` 2/4 có 1 `rapid_guess` + 1/1) trên hai LO không seed/test nào khác dùng (`MATH.G3.NUM.DIV_REMAINDER`, `MATH.G3.GEO.PERIMETER`). Giá trị golden (`DEMOA1` khối 3, 7 ngày mặc định): `active_students` 3, `accuracy` 0.643, `rapid_guess_ratio` 0.067, hai bài 3/8, heatmap như bảng của LOCAL_DEVELOPMENT §5.2 — `VERIFY=1` kiểm lại. Các số tổng quan chỉ đúng khi chưa học sinh nào khác của khối làm bài trong 7 ngày (test Godot/E2E làm bài sẽ đổi); hai hàng heatmap chỉ đổi khi chấm lại (F058 T047).
- Q: T047 (dữ liệu 2 năm)? → A: [Confirmed] Không seed local (không có biến thể nhỏ được đặc tả): thuộc đường datagen perf/staging (`make datagen`, F132).

### Session 2026-11-28 (định nghĩa "Học sinh hoạt động" — [Confirmed — owner "chốt hết"])

- Q: Cùng nhãn "Học sinh hoạt động" mà tổng quan trường và báo cáo tenant/platform ra số khác định nghĩa? → A: [Confirmed — owner "chốt hết"] Portal không tính lại; backend trả một định nghĩa (distinct trong kỳ đã chọn, [LEARNING_ANALYTICS §4](../../docs/02-learning/LEARNING_ANALYTICS.md#4-progress-metrics), [F043 Clarifications 2026-11-28](../043-school-learning-dashboard/spec.md)). Ô bị che vẫn hiện "< 5".

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Trang tổng quan trường (Priority: P1) 🎯 MVP

`ad.truong.a` mở tổng quan TH Demo A tuần này và trả lời 5 câu hỏi: bài đã làm chưa, LO yếu, ai cần hỗ trợ, có tự học sau gợi ý không, có thông báo an toàn chờ xác nhận không (US-LMS-003, US-LMS-009, US-MOD-004).

**Why this priority**: Đây là màn nhà trường mở mỗi tuần; là kịch bản demo M-C3.

**Independent Test**: Với dữ liệu seed sau khi 3 học sinh làm bài, mở tổng quan, đổi khoảng thời gian, kiểm 5 widget, nhãn "Cập nhật lúc", "Tạm tính".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F056-AC-01 | KHI người dùng có `analytics.school.read` mở Tổng quan, Portal PHẢI gọi `getSchoolDashboard` với `school_id`, `from`, `to` và bộ lọc khối/môn (không lớp — ADR-0095), rồi hiển thị widget theo thứ tự 5 câu hỏi: bài giao đang mở/sắp hạn, LO cần củng cố, cần chú ý, tự học và gợi ý, thông báo an toàn | FR-LMS-011 · FR-LMS-003 · [SCHOOL_LMS §8.1](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F056-AC-02 | HỆ THỐNG PHẢI hiển thị trên mỗi widget "Cập nhật lúc HH:mm" từ `data_as_of`, nhãn độ hạt từ `grain`, nhãn "Tạm tính" khi `is_partial = true`, và "Đang tính số liệu…" thay cho số 0 khi ngày chưa chốt theo `dashboard_day_close_time` | FR-ANL-013 · AC-DASH-006 · [ADMIN_PORTAL §3.14.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-DASH-003 |
| F056-AC-03 | HỆ THỐNG PHẢI cung cấp bộ chọn khoảng thời gian với preset (Hôm nay, 7 ngày, 30 ngày, Học kỳ này, Năm học này, 12 tháng) và khoảng tùy chọn không vượt `dashboard_max_range_days`; ngày ngoài giới hạn bị vô hiệu; không có "toàn bộ thời gian"; người dùng chỉ chọn được độ hạt thô hơn quy tắc `dashboard_grain_daily_max_days`/`dashboard_grain_weekly_max_days` | FR-ANL-013 · AC-DASH-002 · AC-DASH-003 | TC-DASH-002 |
| F056-AC-04 | NẾU API trả `422 DASHBOARD_RANGE_TOO_LARGE` hoặc `504 DASHBOARD_QUERY_TIMEOUT`, THÌ widget PHẢI hiển thị gợi ý thu hẹp khoảng/phạm vi; NẾU trả `429 DASHBOARD_BUSY`, THÌ widget PHẢI tự thử lại sau 2–5 giây tối đa 2 lần; mỗi widget tải độc lập (skeleton riêng) và một trang không gửi quá `dashboard_max_concurrent_per_user` truy vấn đồng thời | FR-ANL-013 · AC-DASH-004 · AC-DASH-005 | TC-DASH-006 |
| F056-AC-05 | KHI hiển thị "Cần chú ý", Portal PHẢI liệt kê học sinh với nhãn theo `reason` (`inactive`, `low_accuracy`, `missed_assignments`, `rapid_guessing` → "làm quá nhanh") bằng ngôn ngữ trung tính, không xếp hạng học sinh theo điểm | FR-LMS-011 · AC-LMS-017 · [SCHOOL_LMS §8.8](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-003 |
| F056-AC-06 | KHI giá trị có `suppressed = true` hoặc ô dưới 5 học sinh ở báo cáo cấp tenant/platform, Portal PHẢI hiển thị "< 5" thay cho số bằng component dùng chung `SuppressedCell` (`src/cms/frontend/packages/ops-ui/src/dashboard/`, kèm tooltip) | FR-ANL-016 · AC-DASH-011 | TC-DASH-009 |
| F056-AC-07 | Widget an toàn PHẢI chỉ hiển thị số thông báo chưa xác nhận (`safety_notifications_unacknowledged`) và liên kết tới màn thông báo an toàn; HỆ THỐNG PHẢI không hiển thị nội dung chat ở bất kỳ màn nào của F056 | AC-LMS-018 · AGENTS R14, R16 | — |

---

### User Story 2 — Drill-down khối, bản đồ kỹ năng, hồ sơ học sinh (Priority: P1)

School Admin đi từ trường → khối 3 → học sinh, xem LO khối còn yếu và mastery học sinh × LO, rồi giao ôn tập LO "Chia có dư" cho khối trong 2 thao tác (US-LMS-002, US-LMS-005, US-LMS-006; ADR-0095).

**Why this priority**: Trả lời câu hỏi 2–3 ở mức hành động được; nối sang wizard giao bài (F055).

**Independent Test**: Từ tổng quan chọn khối 3 → "LO cần củng cố" → mở hồ sơ `hs.an.3a` từ "Cần chú ý"; bấm "Giao ôn tập theo LO".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F056-AC-08 | KHI người dùng chọn một khối của trường, Portal PHẢI gọi `getSchoolDashboard` có `grade_level`, giữ nguyên khoảng thời gian và môn, và KHI mở một học sinh PHẢI hiển thị breadcrumb trường → khối → học sinh; Portal PHẢI không có trang, bộ lọc, danh sách hay liên kết lớp (ADR-0095) | FR-LMS-011 · AC-LMS-010 · ADR-0095 | TC-LMS-003 |
| F056-AC-09 | HỆ THỐNG PHẢI vẽ heatmap LO bằng component `MasteryHeatmap` (ops-ui) với ký hiệu mức (○◔◑◕●) kèm màu từ token, có nhãn cho trình đọc màn hình và tooltip giá trị chính xác; không dùng màu là kênh duy nhất | FR-LMS-011 · NFR-A11Y-003 · [DESIGN_SYSTEM §4.4](../../docs/05-ux/DESIGN_SYSTEM.md) | TC-UI-006 |
| F056-AC-10 | KHI người dùng có `lms.assignment.write` bấm "Giao ôn tập theo LO" trên một LO yếu, Portal PHẢI mở wizard F055 đã điền `mode = practice`, LO, trường, môn và khối của bộ lọc (ADR-0095: không lớp), để hoàn tất trong 2 thao tác | US-LMS-006 · [SCHOOL_LMS §8.3](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F056-AC-11 | KHI người dùng mở hồ sơ học sinh, Portal PHẢI hiển thị mastery theo LO/topic, xu hướng 8 tuần, lịch sử bài, gợi ý hệ thống đang hiển thị (từ `getStudentLearningProfile`) và tab Hoạt động đăng nhập từ `listTenantStudentLoginEvents`; PHẢI không hiển thị Game Rating, vị trí trong game, bạn bè, chat, IP hay mã thiết bị | US-LMS-005 · AC-LMS-015 · [SCHOOL_LMS §8.5](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |

---

### User Story 3 — Bảng kết quả và phân tích đề/bài (Priority: P2)

Academic Manager xem kết quả các bài giao môn Toán khối 3 và phân tích đề "Phép nhân": phổ điểm, câu nhiều học sinh sai (US-LMS-007).

**Why this priority**: Phục vụ họp chuyên môn; cần dữ liệu đề đã chấm (F044).

**Independent Test**: Mở "Kết quả bài/đề", lọc trường + Toán + khối 3; mở bài graded → Phân tích.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F056-AC-12 | KHI người dùng mở "Kết quả bài/đề", Portal PHẢI liệt kê bài giao từ `listAssignments` lọc theo trường, môn, khối (ADR-0095: không course, không lớp) với đối tượng, số đã nộp và điểm tự chấm trung bình theo `score_scale`, phân trang cursor; không có cột XP, rating, điểm thi đấu hay ô nhập điểm tay | FR-LMS-013 · [SCHOOL_LMS §10](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F056-AC-13 | KHI người dùng mở tab Phân tích của bài/đề, Portal PHẢI hiển thị phổ điểm, trung vị, tỉ lệ đạt, p-value theo câu, lựa chọn sai phổ biến, tỉ lệ `rapid_guess` (đã loại khỏi p-value) và cảnh báo độ tin cậy thấp khi server trả, kèm nút "Giao ôn tập cho học sinh sai câu này" mở wizard F055 (trường, môn, khối của bài) | US-LMS-007 · AC-LMS-017 · [SCHOOL_LMS §8.6](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-EXM-004 |

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
| F056-AC-21 | HỆ THỐNG PHẢI gửi `X-Tenant-Id` = tenant phiên; KHI `school_id`/`student_id` thuộc tenant hoặc trường khác trả `404`, Portal PHẢI hiển thị "Không tìm thấy" không tiết lộ sự tồn tại; cache truy vấn phía client có khóa chứa tenant | AC-LMS-012 · AC-DASH-011 · AGENTS R8 | TC-DASH-009 |
| F056-AC-22 | **Removed (ADR-0095)** — ô tìm học sinh của trang lớp đã bỏ cùng trang lớp. Chỉ giữ kiểm âm: dashboard PHẢI không có trang lớp hay lời gọi `/lms/classes/…` | ADR-0041 · ADR-0095 | — |
| F056-AC-23 | KHI menu/action của feature (`menu.school_lms.school_dashboard`, `menu.school_lms.results`, `menu.school_lms.reports`, `menu.platform_admin.reports` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU trường chưa có dữ liệu (đầu năm), THÌ widget PHẢI hiển thị trạng thái rỗng có hướng dẫn, không lỗi.
- NẾU chỉ số mới có `available_from`, THÌ PHẢI hiển thị "Chưa có số liệu trước ngày …" (F056-AC-02).
- NẾU replica trễ và server trả dữ liệu kèm cảnh báo trễ, THÌ Portal PHẢI hiển thị cảnh báo cạnh "Cập nhật lúc".
- NẾU người dùng đổi tenant/đơn vị khi export đang chạy, THÌ Portal PHẢI bỏ theo dõi job của tenant cũ.
- NẾU danh sách (bài giao, mức tham khảo) có > `dashboard_page_size_max` dòng, THÌ PHẢI phân trang cursor.
- NẾU server còn trả trường lớp (`classes`, `by_class`, `class_ids`, `class_id`) trước WP-L2, THÌ Portal PHẢI bỏ qua, không hiển thị (ADR-0095).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-LMS-003 | Theo dõi kết quả, tiến độ, LO yếu | [SCHOOL_LMS §8](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-011 | UI School Learning Dashboard trường → khối → học sinh (ADR-0095) | [SCHOOL_LMS §8](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-013 | Kết quả tự chấm theo bài giao (không course — ADR-0095), xuất XLSX/CSV | [SCHOOL_LMS §10](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-ANL-012 | Dashboard trường, tenant, platform | [DASHBOARD_DATA §2](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-013 | Quy tắc UI: khoảng ≤ 2 năm, độ hạt, `data_as_of`, tạm tính | [ADMIN_PORTAL §3.14.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ANL-014 | Xuất báo cáo bất đồng bộ, link có hạn | [DASHBOARD_DATA §10](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-015 | Hiển thị báo cáo tham khảo TT27/TT22 có nhãn | [ASSESSMENT_REGULATION_PROFILE](../../docs/02-learning/ASSESSMENT_REGULATION_PROFILE.md) |
| FR-ANL-016 | Ẩn ô < 5 học sinh ở cấp tổng hợp | [LEARNING_ANALYTICS](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **SchoolDashboard**: gói widget của trường (lọc khối, môn) kèm `grain`, `data_as_of`, `is_partial`. (ClassDashboard: Removed — ADR-0095.)
- **StudentLearningProfile**: mastery theo LO, xu hướng, lịch sử bài, gợi ý đang hiển thị của một học sinh.
- **Assignment** (F055): bài giao với `completion` (đã nộp, điểm trung bình) — thay CourseResults (Removed — ADR-0095).
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
- Màn thông báo an toàn và xác nhận liên hệ phụ huynh: F122 (workspace C, phần nhận); hoạt động đăng nhập, "Học sinh theo khối", dọn học sinh: F052 (danh sách trường → khối → học sinh: WP-I1).
- Báo cáo hoạt động game của tenant (Game Analytics tổng hợp): F080.
- Bộ lọc tenant liên tenant: F057; shell: F051.

## Assumptions

- Thư mục `src/cms/frontend/apps/portal/src/workspaces/school-lms/dashboard/`, route `/portal/school-lms/dashboard/*` (ADMIN_PORTAL §1.2, F051).
- [Confirmed] Endpoint hồ sơ học sinh và phân tích đề/bài theo đề xuất ở [contracts §6](contracts/README.md) — owner chốt 2026-11-20 (F043/F044 làm, đã có trong spec/tasks của hai feature); US2 phần hồ sơ và US3 phần phân tích chỉ chạy sau delta.
- Số liệu "tự học sau gợi ý" (câu hỏi 4) cần trường bổ sung trong `SchoolDashboard` (delta [Confirmed]); trước khi có, widget ẩn kèm "Chưa có số liệu".

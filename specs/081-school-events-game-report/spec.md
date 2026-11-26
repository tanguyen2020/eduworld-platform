# Feature Specification: F081 — Workspace C — sự kiện trường từ template

**Feature Branch**: `081-school-events-game-report` · **Created**: 2026-11-16 · **Status**: Done · **Module**: `src/cms/frontend/apps/portal` (workspace C `school-lms`) + module `game` (1 query đọc) · **Mốc**: M3 · **Phụ thuộc**: F052, F077
**Input**: Màn "Sự kiện của trường" trong workspace C cho Tenant Admin/School Admin (và System Admin trên tenant đích): tạo sự kiện từ template platform đã gán, chỉ điền `allowed_params`; danh sách, sửa trước khi chạy, hủy có lý do; xem tiến độ community goal theo khối (ADR-0095). Backend nhỏ ở module game: query tiến độ sự kiện trường. Màn "Hoạt động game" (báo cáo game tổng hợp cho nhà trường) đã chuyển sang F080 US5 (M4b) khi tách F081 (Clarifications Session 2026-11-17). FR nguồn: FR-EVT-006, FR-EVT-004, FR-LMS-007, FR-CMS-010, FR-CMS-013.

## Clarifications

### Session 2026-11-16

- Q: API tạo/sửa/hủy sự kiện trường đã có backend chưa? → A: Có — F077 US5 (`listEventTemplates`, `listSchoolEvents`, `createSchoolEvent`, `getSchoolEvent`, `updateSchoolEvent`, `cancelSchoolEvent`, route ngoại lệ `/lms/*` → module game). F081 chỉ thêm UI và query tiến độ.
- Q: "Xem tiến độ community goal theo lớp" (nay theo khối — Session 2026-11-21) ([ADMIN_PORTAL §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md)) gọi API nào? → A: Chưa có (`getCommunityGoal` chỉ cho học sinh). F081 thêm `getSchoolEventProgress` (module game, chỉ đọc PostgreSQL) — "Cần thêm" ([contracts](contracts/README.md) §6).
- Q: Bộ chọn quest/challenge trong form hiển thị tên lấy ở đâu khi `EventTemplate.quest_options` chỉ là UUID? → A: Delta cộng thêm `quest_option_items[]`/`challenge_option_items[]` (id + tiêu đề song ngữ + khối) vào `EventTemplate`; Portal không tự tra tên ở module khác.
- Q: `TenantGameSummary` thiếu một số chỉ số §7.2 (`active_days_per_week`, `session_duration_p50`, `chapter0_completion`, `social_play_rate`, chơi khuya, phân bố nền tảng/hạng thiết bị)? → A: Mở rộng response (cộng thêm field, đọc bảng tổng hợp của F080); không định nghĩa lại chỉ số. Đổi ở Session 2026-11-17: phần này chuyển sang F080 (F080-AC-31).
- Q: Academic Manager có xem báo cáo game không? GAME_ANALYTICS §8 ghi "không", nhưng ADMIN_PORTAL §2.2 và SECURITY_ARCHITECTURE cấp `analytics.school.read` (kèm báo cáo game) cho `academic_manager`. → A: [Confirmed] Theo permission catalog (ADMIN_PORTAL §2.2 sở hữu catalog): có `analytics.tenant.read`/`analytics.school.read` ở scope tenant/trường thì xem được; UserRole chỉ scope `subject` → `403 PERMISSION_DENIED` (đổi ở Session 2026-11-17; dữ liệu game không có chiều môn). Mâu thuẫn GAME_ANALYTICS §8 ghi báo cáo. Đổi ở Session 2026-11-17: câu trả lời này chuyển sang F080 cùng màn "Hoạt động game".
- Q: Xuất CSV báo cáo game tenant? → A: Không trong §7.2; ngoài phạm vi (chuyển sang F080 cùng màn báo cáo).

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: F081 (M3) dùng `getTenantGameSummary`, rollup, `suppression.go` của F080 (M4b) — xử lý phụ thuộc ngược mốc thế nào? → A: Technical Architect chốt tách F081 ([CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý)): sự kiện trường từ template và tiến độ giữ ở F081 (M3, chỉ cần F052, F077); toàn bộ User Story 4 cũ "Báo cáo hoạt động game của tenant/trường" (tiêu chí 13–18 cũ của F081, mở rộng `TenantGameSummary`, màn "Hoạt động game") chuyển sang F080 US5 thành F080-AC-27…32 (M4b). Mã tiêu chí 13–18 của F081 bỏ trống, không dùng lại; User Story 5 cũ đổi số thành User Story 4.

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Bỏ khái niệm lớp ([ADR-0095](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) mục 4, 5) — đối tượng sự kiện trường và bảng tiến độ đổi thế nào? → A: [Confirmed] Đối tượng chỉ gồm trường và khối (`audience.school_ids`, `grade_levels`; `class_ids` đã gỡ khỏi `SchoolEventCreate/Update` — áp 2026-11-21, WP-S); bộ chọn khối là danh sách khối 1–9 giao với `grade_levels` của template, không gọi `listTenantSchoolClasses`. `SchoolEventProgress.by_class[]` → `by_grade[] {grade_level, contribution_total, contributors_count}` sắp theo khối, chỉ khối thuộc dải EVENT_SEASON_SYSTEM §6 (khối 4–9).
- Q: Ô đóng góp theo khối có áp che ô nhỏ không? → A: [Confirmed] Có — ADR-0095 mục 4: mọi ô tổng hợp cấp khối áp FR-ANL-016; khối có < `analytics_min_cell_size` (5) học sinh đóng góp không được trả (thay quyết định cũ "bảng lớp không áp ẩn ô < 5" ở Assumptions).

### Session 2026-11-25 (triển khai)

Owner ra lệnh "chốt hết": các khoảng trống gặp khi triển khai được chốt bằng mặc định an toàn nhất, không mở rộng phạm vi (R34).

- Q: Sự kiện có nhiều trường (hoặc cả đơn vị) và community goal tính theo trường — `getSchoolEventProgress` trả tiến độ của trường nào? → A: [Confirmed] Thêm query `school_id` (tùy chọn) vào operation mới; mặc định là trường duy nhất của sự kiện (trường sở hữu hoặc trường duy nhất trong `audience.school_ids`). Mục tiêu tính theo trường mà sự kiện có nhiều trường và thiếu `school_id` → `422 VALIDATION_FAILED` trên `school_id`; trường ngoài đối tượng sự kiện, ngoài tenant hoặc ngoài scope UserRole → `404`. Số tham gia và `by_grade` tính cho trường đang xem; mục tiêu tính theo tenant thì tổng/mốc là của tenant. Response có `school_id` (null = cả đơn vị). Portal hiện bộ chọn trường (mọi trang `listTenantSchools`, lọc theo đối tượng) khi sự kiện không đúng một trường.
- Q: `by_grade` cho nhà trường có theo cờ `show_grade_contributions` của sự kiện không? → A: [Confirmed] Có — theo [EVENT_SEASON_SYSTEM §6](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) ("khi sự kiện bật `show_grade_contributions`, nhà trường và học sinh xem được đóng góp theo khối"); tắt → `by_grade` rỗng. Dải khối 4–9 và ẩn ô dùng chung `liveops.VisibleGrades` của F077 (không tạo `GradeBreakdownPolicy` trùng — R33); dải khối đăng ký ở CONFIG_DEFAULTS `community_goal_grade_breakdown_grades`.
- Q: `EventTemplate` không có `grade_levels` — bộ chọn khối lấy danh sách nào? → A: [Confirmed] Khối 1–9 (`SchoolEventCreate.audience.grade_levels`), chỉ hiện khi template cho phép `audience.grade_levels`; server kiểm cuối cùng. Không gọi `listTenantSchoolClasses` (ADR-0095).
- Q: `SchoolEvent.status` khi run `archived` hiển thị gì? → A: [Confirmed] Giữ `ended`/`cancelled` (trạng thái kết thúc trường đã biết — quyết định của F077); `suspended` nay trả đúng `suspended` (trước hiển thị `live`), lọc được bằng `status=suspended`. Enum `SchoolEventStatus` gồm cả `archived` cho khớp `EventRunStatus`. Tham số `status` của `listSchoolEvents` **giữ kiểu chuỗi** (`maxLength: 32`, mô tả trỏ `SchoolEventStatus`) thay vì `$ref` enum như delta ban đầu: đổi sang enum là breaking (`request-parameter-became-enum`, `make specs-breaking`) và chỉ được chấp nhận kèm ADR — không đáng một ADR; giá trị khác không khớp sự kiện nào (hành vi cũ). Tenant không có nút Sửa/Hủy khi `suspended` (portal); server giữ quy tắc chuyển trạng thái của F077.
- Q: `listSchoolEvents` khai báo `TenantIds` nhưng route đòi `X-Tenant-Id` — System Admin đọc nhiều tenant thế nào (F081-AC-06)? → A: [Confirmed] Route đổi sang `TenantIdHeaderOptional`: không header + `tenant_ids` → CrossTenantScope; không header, không `tenant_ids` → mọi tenant qua `tenancy.Bypass(system_admin_cross_tenant)` có audit `cross_tenant.accessed`; role platform khác → `403`. Trang keyset trong SQL. Middleware `httpx` nay đọc **mọi** giá trị `tenant_ids` lặp lại (`style: form, explode: true` của contract — trước chỉ đọc giá trị đầu). Chi tiết/sửa/hủy/tiến độ vẫn một tenant (`X-Tenant-Id` = tenant của dòng, portal giữ trong `?tenant=`).
- Q: `422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED` không có `errors[]` — portal map về trường thế nào (F081-AC-04)? → A: [Confirmed] Module game thêm `errors[] {field = tên tham số (vd. `audience.school_ids`, `quest_ids`, `ends_at`, `template_id`), code invalid}` (cộng thêm, không breaking); portal đặt lỗi lên đúng ô; lỗi từ nhạy cảm (`VALIDATION_FAILED`) chỉ hiện thông điệp chung.
- Q: Test đơn vị portal đặt ở `tests/unit/...` như plan? → A: [Confirmed] Theo quy ước hiện có của portal: `src/workspaces/school-lms/school-events/__tests__/` (Vitest + MOCK `src/test/school-events-mock.ts`, nhãn MOCK — R17). E2E Playwright với backend compose (T014, T029, T032, T034) chưa chạy được trong phiên này; tiêu chí tương ứng có test Vitest toàn portal + integration test module game (bảng AC → test trong tasks.md).

### Session 2026-11-26 (follow-up gateway — owner "chốt hết", [Confirmed])

- Q: `check-routes.mjs` báo route `portal-tenant` bắt buộc `X-Tenant-Id` trong khi `listSchoolEvents` khai `TenantIdHeaderOptional` — sửa ở gateway thế nào mà không nới các route khác? → A: [Confirmed] Route riêng `lms-school-events-list` trong `deploy/apisix/routes/routes.yaml` (gateway `both`, chỉ `GET`, đúng URI `/api/v1/lms/school-events`, `tenant_header: optional`, ưu tiên 200 > `portal-tenant`), theo cách các operation "System Admin bỏ `X-Tenant-Id`" khác dùng route chế độ tùy chọn (`tenant-optional`, `platform-internal`). `POST /lms/school-events` và mọi `/lms/school-events/{event_id}*` vẫn ở `portal-tenant` (bắt buộc); module game vẫn đòi header với mọi người gọi trừ System Admin. Test: `deploy/apisix/scripts/check-routes.test.mjs`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tạo sự kiện trường từ template (Priority: P1) 🎯 MVP

`ad.truong.a` mở "Sự kiện trường", chọn template "Tuần lễ Toán học" đã được gán, điền tên, ngày, chọn khối 3 và quest trong danh sách template, chọn huy hiệu trường; gửi tạo.

**Why this priority**: FR-EVT-006 — cách duy nhất tenant tổ chức sự kiện game (ADR-0027: không cấu hình game ngoài template).

**Independent Test**: Trên backend F077 thật: template chưa gán không hiện; tạo hợp lệ → sự kiện `scheduled`; bấm Tạo hai lần → một sự kiện; tên chứa từ nhạy cảm (fixture placeholder) → lỗi tại trường tên, không hiển thị từ khớp.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F081-AC-01 | KHI người có `lms.school_event.write` mở "Tạo sự kiện", Portal PHẢI hiển thị template từ `listEventTemplates` (chỉ template đã gán cho tenant phiên) với tên, loại, mô tả, `max_duration_days`; NẾU danh sách rỗng, THÌ PHẢI hiển thị trạng thái rỗng hướng dẫn liên hệ EduWorld, không có form tự do | FR-EVT-006 · FR-LMS-007 · [EVENT_SEASON_SYSTEM §10](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) · ADR-0029 | TC-TEN-019 |
| F081-AC-02 | KHI chọn template, Portal PHẢI chỉ dựng trường có trong `allowed_params`; quest/challenge/phần thưởng PHẢI chọn từ `quest_option_items`/`challenge_option_items`/`reward_options` của template; đối tượng PHẢI chỉ gồm trường/khối trong phạm vi UserRole (khối chọn trong `grade_levels` của template, không có lớp — ADR-0095); Portal PHẢI không có ô nhập luật chơi, cách tính điểm hay phần thưởng ngoài template | FR-EVT-006 · FR-LMS-007 · [ADMIN_PORTAL §3.6, §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-EVT-008 |
| F081-AC-03 | KHI bấm Tạo, Portal PHẢI gọi `createSchoolEvent` với `Idempotency-Key` sinh một lần cho ý định và giữ nguyên khi thử lại (mạng lỗi, bấm lặp), rồi mở chi tiết sự kiện vừa tạo; audit do backend ghi | FR-EVT-006 · AGENTS R10 | TC-EVT-008 |
| F081-AC-04 | NẾU server trả `422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED` hoặc `422 VALIDATION_FAILED`, THÌ Portal PHẢI map lỗi về đúng trường theo `errors[].field` (lỗi từ nhạy cảm chỉ hiển thị thông điệp chung, không có từ khớp); NẾU `503 UNAVAILABLE`, THÌ PHẢI giữ dữ liệu form và cho thử lại; NẾU `409 CONFLICT`, THÌ PHẢI báo đã đạt số sự kiện đồng thời tối đa của đơn vị | FR-EVT-006 · AC-EVT-008 · AGENTS R15 | TC-EVT-008 |
| F081-AC-05 | Portal PHẢI bắt buộc tên `vi`, cho `en` tùy chọn, giới hạn mọi ô theo `maxLength` của `TenantLocalizedName`/`TenantLocalizedText` có bộ đếm ký tự, chọn ngày theo giờ ICT và hiển thị thời lượng tối đa của template; kiểm tra cuối cùng do server | FR-EVT-006 · ADR-0041 · ADR-0056 | TC-TEN-024 |

---

### User Story 2 — Danh sách, sửa, hủy sự kiện trường (Priority: P1)

School Admin xem các sự kiện của trường theo trạng thái, sửa ngày/đối tượng khi chưa chạy, hủy kèm lý do; Tenant Admin xem mọi trường trong đơn vị; System Admin xem nhiều tenant.

**Why this priority**: Vòng đời tối thiểu để tổ chức sự kiện an toàn (FR-EVT-006, FR-EVT-003 phần tenant).

**Independent Test**: Sửa với ETag cũ → thông báo xung đột; hủy không lý do → bị chặn; `ad.truong.b` dán URL sự kiện của TH Demo A → "Không tìm thấy".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F081-AC-06 | KHI mở danh sách, Portal PHẢI hiển thị `listSchoolEvents` (cursor, lọc trạng thái) với tên, template, lịch ICT, đối tượng, trạng thái (Tag màu token); với System Admin PHẢI có bộ lọc tenant nhiều lựa chọn (`tenant_ids`) và cột Tenant, và khi tạo PHẢI chọn một tenant đích | FR-CMS-013 · FR-CMS-010 · [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-TEN-004 |
| F081-AC-07 | KHI sửa sự kiện trạng thái `scheduled`, Portal PHẢI gọi `updateSchoolEvent` với `If-Match`; NẾU trả `412 VERSION_MISMATCH` hoặc `409 CONFLICT`, THÌ PHẢI báo dữ liệu đã đổi và cho tải lại; TRONG KHI sự kiện `live`/`ended`/`cancelled`/`archived`, Portal PHẢI không hiển thị nút Sửa | FR-EVT-006 · ADR-0043 | — |
| F081-AC-08 | KHI hủy, Portal PHẢI bắt buộc lý do 3–1000 ký tự, xác nhận hai bước nêu "phần thưởng học sinh đã đạt được giữ nguyên", và gọi `cancelSchoolEvent` với `Idempotency-Key` | FR-EVT-006 · FR-EVT-003 · AGENTS R10 | TC-EVT-008 |
| F081-AC-09 | NẾU người dùng không có `lms.school_event.write` (vd. `academic_manager`) mở URL "Sự kiện trường", THÌ Portal PHẢI hiển thị trang 403 và không có mục menu; NẾU API trả `404 NOT_FOUND` (sự kiện trường/tenant khác) hoặc `403 TENANT_MISMATCH`, THÌ PHẢI hiển thị "Không tìm thấy" không lộ thông tin sự kiện | FR-CMS-010 · AGENTS R8 · NFR-SEC-006 | TC-TEN-003 |

---

### User Story 3 — Tiến độ sự kiện trường (Priority: P2)

Trong khi sự kiện chạy, School Admin xem số học sinh tham gia, thanh tiến độ community goal với các mốc, và tổng đóng góp theo khối — không có tên hay xếp hạng học sinh.

**Why this priority**: FR-EVT-004 — nhà trường theo dõi hiệu quả sự kiện; phụ thuộc US1–US2.

**Independent Test**: Seed đóng góp cho 3A/3B → API trả tổng khớp `community_goal_progress`; `ad.truong.b` gọi cùng `event_id` → 404; response không có `student_id`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F081-AC-10 | KHI người có `lms.school_event.write` gọi `getSchoolEventProgress`, module game PHẢI trả trạng thái run, số học sinh tham gia, tổng và mốc community goal theo phạm vi trường của sự kiện đọc từ PostgreSQL (không từ bộ đếm Valkey), tổng đóng góp và số học sinh có đóng góp theo khối (`by_grade`) chỉ cho khối thuộc dải mà [EVENT_SEASON_SYSTEM §6](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) cho phép và có ≥ `analytics_min_cell_size` học sinh đóng góp (ADR-0095), kèm `data_as_of`; response PHẢI không chứa `student_id`, tên, nickname hay thứ hạng | FR-EVT-004 · AGENTS R9, R16 | — |
| F081-AC-11 | Portal PHẢI hiển thị tiến độ dạng thanh có mốc 25/50/75/100% theo dữ liệu server, bảng khối sắp theo khối tăng dần (không cột hạng, không tô nổi khối dẫn đầu), "Cập nhật lúc", và nút làm mới thủ công | FR-EVT-004 · AGENTS R19 · [ADMIN_PORTAL §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F081-AC-12 | NẾU người gọi không có `lms.school_event.write`, sự kiện thuộc trường ngoài scope UserRole hoặc tenant khác, THÌ module game PHẢI trả `403 PERMISSION_DENIED` / `404 NOT_FOUND`; header `X-Tenant-Id` khác tenant phiên PHẢI nhận `403 TENANT_MISMATCH` | AGENTS R7, R8 · NFR-SEC-006 | TC-TEN-003 |

---

### User Story 4 — Ràng buộc chung của màn (Priority: P2)

**Why this priority**: Constitution điều V, VII, VIII cho mọi màn mới.

**Independent Test**: Lint/E2E: network log mọi POST có `Idempotency-Key`; không import component game; điện thoại chỉ xem.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F081-AC-19 | HỆ THỐNG PHẢI hiển thị lỗi theo `code` + `trace_id`, dùng Admin/Ops design system (không import component/Theme game, không hardcode màu/cỡ chữ), và trên điện thoại chỉ cho xem danh sách, chi tiết và tiến độ (form tạo/sửa hiển thị "Tác vụ này cần màn hình lớn hơn") | AGENTS R32 · ADR-0047 · [ADMIN_PORTAL §3.14](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-UI-010 |
| F081-AC-20 | KHI menu/action của feature (`menu.school_lms.school_events` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU template bị gỡ gán sau khi sự kiện đã tạo, THÌ sự kiện vẫn hiển thị và chạy theo version đã ghim; form tạo mới không còn template đó (F081-AC-01).
- NẾU sự kiện bị platform dừng khẩn cấp (`suspended`), THÌ danh sách hiển thị trạng thái "Tạm dừng bởi EduWorld", không có nút Sửa/Hủy cho tenant (delta `SchoolEvent.status`).
- NẾU học sinh đổi khối/trường sau khi đóng góp (`student.placement_changed`), THÌ đóng góp cũ vẫn tính cho khối ghi tại thời điểm đóng góp (ADR-0095 mục 2).
- NẾU sự kiện không có community goal, THÌ màn tiến độ chỉ hiển thị số tham gia.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-EVT-006 | UI tạo/sửa/hủy sự kiện trường chỉ với `allowed_params` | [EVENT_SEASON_SYSTEM §10](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) · [ADMIN_PORTAL §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-EVT-004 | Xem tiến độ community goal theo khối, không xếp hạng cá nhân (query + UI) | [EVENT_SEASON_SYSTEM §6](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) |
| FR-LMS-007 | Tenant chỉ chọn nội dung trong template đã gán, không cấu hình game | [ADMIN_PORTAL §3.6](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-010 | Màn chạy trong đúng một tenant phiên | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-013 | System Admin: lọc nhiều tenant khi đọc, một tenant đích khi ghi | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **EventTemplate (đã gán)**: mẫu sự kiện platform, `allowed_params`, danh sách quest/challenge/phần thưởng cho phép.
- **SchoolEvent**: sự kiện của tenant tạo từ template; lịch, đối tượng, trạng thái run.
- **SchoolEventProgress**: ảnh chụp tiến độ (tham gia, community goal, đóng góp theo khối) — chỉ số tổng hợp, không định danh học sinh.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `school_event_max_duration_days` | Thông điệp thời lượng tối đa (server kiểm) |
| `school_events_concurrent_per_tenant` | Thông điệp lỗi `409 CONFLICT` |
| `community_goal_progress_flush_seconds` | Gợi ý độ trễ hiển thị tiến độ |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không business logic ở UI: quyền, `allowed_params`, giới hạn, ẩn ô đều ở server | AGENTS R1 |
| Không lộ danh tính/xếp hạng học sinh trong tiến độ | AGENTS R16 · NFR-OBS-003 |
| Văn bản tenant qua kiểm tra từ nhạy cảm ở server, không trả từ khớp | AGENTS R15 |
| Admin/Ops UI, token, WCAG 2.1 AA | AGENTS R32 · NFR-A11Y-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: School Admin tạo một sự kiện trường từ template trong ≤ 5 phút trên dữ liệu seed (E2E có đo thời gian).
- **SC-002**: 100% request tạo/hủy sự kiện có `Idempotency-Key`; bấm lặp/thử lại tạo đúng một sự kiện.
- **SC-003**: 0 response tiến độ sự kiện chứa định danh học sinh (quét tự động).
- **SC-004**: 0 dữ liệu sự kiện của trường hoặc tenant ngoài phạm vi trả về trong bộ test cô lập (6 tổ hợp tài khoản demo).

## Ngoài phạm vi

- Backend vòng đời sự kiện trường, kiểm tra từ nhạy cảm, community goal: F077.
- Báo cáo hoạt động game cho nhà trường (màn "Hoạt động game" workspace C, mở rộng `TenantGameSummary`, rollup, `player_key`): F080 US5 (M4b).
- Template sự kiện và gán template cho tenant: F079 (soạn) và F057/F045 (gán). Event Builder platform, giám sát/dừng khẩn cấp: F079, F080.
- Thử thách khối/giải trường từ template (module competition): F107/F108. Hoạt động đăng nhập từng học sinh: F052.
- Thông báo trong game về sự kiện trường: không trong tài liệu nguồn — chưa làm.

## Assumptions

- [Confirmed] Route `/school-lms/school-events`, `/school-lms/school-events/:eventId` trong chunk `school-lms` (F052); menu "Giao bài & lịch → Sự kiện trường".
- [Confirmed] Bảng đóng góp theo khối áp ẩn ô < `analytics_min_cell_size` (5) theo ADR-0095 mục 4 (thay quyết định cũ không ẩn ô của bảng lớp); chỉ hiện số học sinh có đóng góp, không danh sách.
- System Admin xem tiến độ của một tenant đích mỗi lần (API có `X-Tenant-Id`).

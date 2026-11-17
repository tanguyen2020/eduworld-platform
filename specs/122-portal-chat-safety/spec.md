# Feature Specification: F122 — Portal an toàn chat: kiểm duyệt (E), danh mục và công tắc (D), chính sách chat trường (C)

**Feature Branch**: `122-portal-chat-safety` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `web/apps/portal` (workspace E, D, C) · **Mốc**: M5 · **Phụ thuộc**: F051, F120, F121 (và F116 cho API danh mục/công tắc)
**Input**: Giao diện Admin/Ops cho an toàn chat: hàng đợi case liên tenant của Platform Safety Team, thông báo nhà trường, từ bổ sung/nhóm từ của tenant (E); danh mục từ nhạy cảm platform và công tắc kiểm duyệt (D); thông báo vụ việc, chính sách chat trường, nhóm từ chỉ xem, duyệt club (C); banner "Đang tắt kiểm duyệt" mọi workspace. FR nguồn: FR-MOD-003, FR-MOD-004, FR-MOD-009, FR-MOD-011, FR-MOD-017.

## Clarifications

### Session 2026-11-16

- Q: Từ bổ sung và bật/tắt nhóm từ của tenant đặt ở workspace nào? → A: E → Kiểm duyệt, Safety Moderator/System Admin ghi (ADR-0072; [ADMIN_PORTAL §3.9](../../docs/04-admin-portal/ADMIN_PORTAL.md)). MODERATION §11.3 còn ghi D và Tenant Admin bật/tắt — mâu thuẫn, theo ADMIN_PORTAL + ADR-0072 (báo cáo).
- Q: Banner cho người dùng tenant lấy dữ liệu từ đâu? → A: Người dùng tenant không gọi được `/admin/chat/moderation-switches`; thêm `getTenantModerationStatus` (delta, owner F116/F122) trả bước đang tắt và thời hạn, không trả lý do.
- Q: Chính sách `student_club_creation` do ai làm UI? → A: F052 (cài đặt trường, F052-AC-22). F122 chỉ làm danh sách duyệt/quản lý club của trường.
- Q: Tên route/thư mục workspace? → A: Theo F051 (shell chốt): `/live-ops`, `/platform-admin`, `/school-lms` → `src/workspaces/{live-ops,platform-admin,school-lms}`.

### Session 2026-11-17

- Q: UI phê duyệt hai người cho công tắc kiểm duyệt và UI chọn preset trường (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Hàng đợi case của Platform Safety Team (Priority: P1) 🎯 MVP

`sm.antoan` mở E → Kiểm duyệt, lọc p1 mọi tenant, nhận case, xem bằng chứng (gốc, revision), ẩn tin, áp sanction, đóng case có lý do.

**Why this priority**: Không có UI thì Safety Team không xử lý được case (FR-MOD-003).

**Independent Test**: Đăng nhập `sm.antoan` → chỉ thấy module Kiểm duyệt; xử lý một case từ đầu tới cuối; network log có `Idempotency-Key`, `If-Match`, `X-Tenant-Id` = tenant case.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-01 | KHI người có `moderation.case.read` mở Hàng đợi case, Portal PHẢI hiển thị `listModerationCases` với bộ lọc tenant (mặc định "Tất cả", chọn nhiều, chỉ trong module Kiểm duyệt), mức p1–p3, trạng thái, người được gán (`me`/`unassigned`/`any`), category, cột Tenant, hạn SLA (quá hạn tô cảnh báo bằng token trạng thái) | FR-MOD-003 · [ADMIN_PORTAL §3.9, §2.5b](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CHAT-042 |
| F122-AC-02 | KHI mở case, Portal PHẢI gọi `getModerationCase` + `listModerationCaseMessages`, hiển thị panel bằng chứng (tin trong snapshot đánh dấu, nội dung gốc, revision, trạng thái thu hồi), lịch sử thao tác, thông báo trường; ô tìm kiếm chỉ tìm trong case (`q` ≤ `maxLength`) và PHẢI không có ô tìm kiếm chat tự do ở bất kỳ màn nào | FR-MOD-003 · FR-MOD-005 · [MODERATION §6.1](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-040 |
| F122-AC-03 | KHI moderator nhận case, ẩn tin, áp sanction, chuyển cấp, đóng/mở lại, Portal PHẢI gọi `assignModerationCase`/`actOnModerationCase`/`createSanction`/`revokeSanction` với `Idempotency-Key`, `If-Match` (ETag case), `X-Tenant-Id` = tenant của case, bắt buộc ghi chú/lý do (bộ đếm theo `maxLength`) và xác nhận hai bước cho `account_lock` | FR-MOD-003 · FR-MOD-006 · AGENTS R10 | TC-CHAT-007 |
| F122-AC-04 | NẾU API trả `412`, `409 MODERATION_CASE_CLOSED` hoặc `403 PERMISSION_DENIED` (thao tác cần trưởng ca), THÌ Portal PHẢI hiển thị thông điệp theo `code` kèm `trace_id`, tải lại case khi `412`, không tự thử lại thao tác ghi | FR-MOD-003 · [ADMIN_PORTAL §10.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-02 |

---

### User Story 2 — Gửi và theo dõi thông báo nhà trường (Priority: P1)

Từ case p1, moderator chọn trường, học sinh của trường đó, điền tóm tắt theo mẫu; danh sách thông báo hiển thị chờ xác nhận/quá hạn.

**Why this priority**: FR-MOD-017; đường liên hệ phụ huynh duy nhất.

**Independent Test**: Gửi thông báo cho TH Demo A → `ad.truong.a` nhận (US7); danh sách platform lọc `overdue`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-05 | KHI người có `moderation.school_notification.send` bấm "Gửi thông báo trường" trên case, Portal PHẢI cho chọn một trường trong `school_ids` của case, chỉ học sinh liên quan thuộc trường đó (≤ 20), mẫu tóm tắt (10–1000 ký tự, hướng dẫn "không chép nội dung chat"), rồi gọi `notifySchoolForCase` với `Idempotency-Key`; case tenant `common` PHẢI ẩn nút và hiện "Liên hệ trực tiếp" | FR-MOD-017 · [MODERATION §6.4](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-043 |
| F122-AC-06 | KHI mở "Thông báo nhà trường" ở E, Portal PHẢI hiển thị `listSchoolSafetyNotificationsPlatform` lọc tenant, `acknowledged`, `overdue`, cột thời điểm gửi, hạn, kết quả liên hệ | FR-MOD-017 · [ADMIN_PORTAL §3.9](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CHAT-043 |

---

### User Story 3 — Danh mục từ nhạy cảm platform (Priority: P1)

`sa.admin` thêm term vào category `insult`, thử chuỗi, xem kết quả hồi quy, publish phiên bản mới.

**Why this priority**: FR-MOD-009; không hardcode từ (R15), danh mục chỉ quản lý được qua portal.

**Independent Test**: CRUD category/term/allowlist, `testSensitiveTerms`, `publishSensitiveCatalog`; `sm.antoan` chỉ xem.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-07 | KHI System Admin có `chat.sensitive_terms.write` mở D → An toàn chat → Danh mục, Portal PHẢI cho CRUD category (`listSensitiveCategories`, `createSensitiveCategory`, `updateSensitiveCategory`, `deleteSensitiveCategory` = xóa mềm có xác nhận), term (lọc category/locale/match_type/trạng thái, biến thể ≤ 50) và allowlist, validate theo schema (`code` pattern, `term` ≤ 200, RE2 cảnh báo khi `pattern`) | FR-MOD-009 · [MODERATION §11](../../docs/03-social-multiplayer/MODERATION.md) · ADR-0030 | TC-CHAT-015 |
| F122-AC-08 | KHI bấm "Thử chuỗi", Portal PHẢI gọi `testSensitiveTerms` (text ≤ 500, tùy chọn tenant) và hiển thị quyết định, `masked_text`, danh sách category/action; KHI publish, Portal PHẢI bắt buộc `change_summary` (5–1000), gọi `publishSensitiveCatalog` và hiển thị `listSensitiveCatalogVersions` với chỉ số hồi quy | FR-MOD-009 · FR-MOD-015 · [MODERATION §11.4](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-012 |
| F122-AC-09 | TRONG KHI người dùng chỉ có `chat.sensitive_terms.read` (Safety Moderator), Portal PHẢI hiển thị danh mục chỉ đọc; Portal PHẢI không ghi nội dung term vào log/telemetry/analytics và không lưu vào `localStorage` | FR-MOD-009 · AGENTS R14, R15 | TC-CHAT-015 |

---

### User Story 4 — Công tắc kiểm duyệt (Priority: P1)

`sa.admin` đề xuất tắt bước `spam` cho TH Demo A 2 giờ để điều tra lỗi lọc nhầm, nhập lý do; `sa.admin2` phê duyệt; lịch sử ghi lại. Bước PII và grooming không có công tắc (ADR-0078).

**Why this priority**: FR-MOD-011; thao tác rủi ro cao với trẻ em, cần UI có xác nhận.

**Independent Test**: Bảng công tắc lọc 2 tenant + dòng Platform; tắt không lý do bị chặn; lịch sử hiển thị.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-10 | KHI System Admin có `chat.policy.toggle` mở Công tắc kiểm duyệt, Portal PHẢI hiển thị `listModerationSwitches` dạng bảng (dòng Platform + tenant theo bộ lọc tenant), mỗi dòng công tắc `enabled_all` và 4 bước tắt được (`sensitive_terms`, `spam`, `bullying`, `async_analysis`); PII và grooming PHẢI hiển thị "Luôn bật" không có công tắc; bước bị cấp trên tắt PHẢI hiện khóa kèm chú thích | FR-MOD-011 · [MODERATION §12.1](../../docs/03-social-multiplayer/MODERATION.md) · [ADMIN_PORTAL §3.9](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CHAT-013 |
| F122-AC-11 | KHI tắt bất kỳ bước nào, Portal PHẢI mở hộp xác nhận nêu rủi ro với học sinh 6–15 tuổi, bắt buộc `reason` (≥ 10 ký tự, bộ đếm), bắt buộc chọn `expires_at` ≤ `moderation_switch_max_duration`, rồi gọi `setModerationSwitches` (`X-Tenant-Id` = tenant đích với scope tenant); NẾU server trả `422 MODERATION_SWITCH_REASON_REQUIRED`, THÌ Portal PHẢI đánh dấu ô lý do | FR-MOD-011 · AC-MOD-016 · ADR-0030 | TC-CHAT-015 |
| F122-AC-12 | KHI mở "Lịch sử", Portal PHẢI hiển thị `listModerationSwitchHistory` (before/after, lý do, `expires_at`, actor, `source` manual/auto_expire) | FR-MOD-011 · [MODERATION §12.2](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-016 |
| F122-AC-27 | KHI yêu cầu tắt được gửi, Portal PHẢI hiển thị trạng thái "Chờ phê duyệt" (chưa hiệu lực) và danh sách yêu cầu chờ cho System Admin khác với nút Phê duyệt/Từ chối gọi `approveModerationSwitchChange`; Portal PHẢI ẩn nút Phê duyệt với người đề xuất và NẾU server trả `403 MODERATION_SWITCH_SELF_APPROVAL`, `422 MODERATION_SWITCH_EXPIRY_REQUIRED` hoặc `422 MODERATION_SWITCH_STEP_NOT_ALLOWED`, THÌ PHẢI hiển thị thông điệp theo `code` | FR-MOD-011 · FR-MOD-014 · [MODERATION §12.2](../../docs/03-social-multiplayer/MODERATION.md#122-quy-tắc-thao-tác) · ADR-0078 | TC-CHAT-045 |

---

### User Story 5 — Banner "Đang tắt kiểm duyệt" (Priority: P1)

Khi TH Demo A đang tắt `spam`, `ad.truong.a`, `sm.antoan` và `sa.admin` (khi lọc tenant đó) thấy banner đỏ trên mọi màn.

**Why this priority**: Bắt buộc theo ADR-0030 khi tắt kiểm duyệt.

**Independent Test**: Tắt `spam` tenant A (đã duyệt) → banner ở portal `ad.truong.a`, không ở `ad.truong.b`; hết `expires_at` → banner mất ≤ 60 s.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-13 | TRONG KHI tenant của phiên có bước kiểm duyệt bị tắt, Portal PHẢI hiển thị trong `BannerSlot` (F051) banner cố định "Đang tắt kiểm duyệt: <bước> — đến <thời hạn>" trên mọi màn cho Tenant Admin/School Admin (qua `getTenantModerationStatus`), cho Safety Moderator và System Admin (qua `listModerationSwitches`, theo tenant đang lọc); làm mới ≤ 60 s, mất khi bật lại hoặc hết hạn | FR-MOD-011 · AC-MOD-018 · [ADMIN_PORTAL §3.9](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CHAT-016 |
| F122-AC-14 | NẾU người dùng thuộc tenant khác, THÌ Portal PHẢI không hiển thị banner của tenant đang tắt; banner PHẢI không hiển thị lý do cho người dùng tenant | FR-MOD-011 · AGENTS R8 | TC-TEN-001 |

---

### User Story 6 — Từ bổ sung và nhóm từ của tenant (Priority: P2)

Theo yêu cầu của `ad.donvi`, `sm.antoan` thêm term bổ sung cho `pgd-demo`, ghi mã yêu cầu, thử chuỗi trong tenant và publish; bật/tắt category được phép.

**Why this priority**: FR-MOD-013 bổ trợ cho FR-MOD-009; ít dùng hơn danh mục platform.

**Independent Test**: Ghi term cho tenant đích, publish; category `allow_tenant_toggle=false` có công tắc khóa; Tenant Admin ở C chỉ xem tên/mô tả/mức.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-15 | KHI người có `chat.tenant_terms.write` chọn một tenant đích ở E → Kiểm duyệt → Từ bổ sung, Portal PHẢI cho `listTenantSensitiveTerms`/`createTenantSensitiveTerm`/`updateTenantSensitiveTerm`/`deleteTenantSensitiveTerm` (chỉ category có sẵn), thử chuỗi với `tenant_id`, và `publishTenantSensitiveCatalog` với `change_summary` bắt buộc chứa mã yêu cầu của tenant | FR-MOD-013 · FR-MOD-009 · ADR-0072 | TC-CHAT-015 |
| F122-AC-16 | KHI mở Nhóm từ của tenant, Portal PHẢI gọi `toggleTenantSensitiveCategory` (`enabled`, `reason`) chỉ cho category `allow_tenant_toggle = true`, các category khác hiện công tắc khóa kèm chú thích; NẾU server trả `403 TENANT_TOGGLE_NOT_ALLOWED`, THÌ hiển thị theo `code` | FR-MOD-013 · AC-MOD-017 | TC-CHAT-015 |
| F122-AC-17 | KHI Tenant Admin (`chat.sensitive_terms.read`) mở C → An toàn → Nhóm từ, Portal PHẢI hiển thị `listTenantSensitiveCategories` chỉ đọc (tên, mô tả, mức, trạng thái), không có nội dung term platform và không có nút ghi | FR-MOD-009 · [MODERATION §11.3](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-015 |

---

### User Story 7 — Thông báo an toàn của trường (Priority: P1)

`ad.truong.a` thấy thông báo "3A · Nghiêm trọng", đọc tóm tắt, ghi chú liên hệ và xác nhận đã liên hệ phụ huynh — trên cả điện thoại.

**Why this priority**: FR-MOD-017 phía nhà trường; SLA xác nhận 24 h với p1.

**Independent Test**: Danh sách chỉ trường mình; xác nhận lần 2 → thông điệp xung đột; giao diện mobile dạng thẻ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-18 | KHI người có `moderation.school_notification.read` mở C → An toàn → Thông báo vụ việc, Portal PHẢI hiển thị `listSchoolSafetyNotifications` (lọc trạng thái, lớp, khoảng 30 ngày) với panel chi tiết (học sinh, lớp, vai trò, loại, mức, biện pháp, tóm tắt, thời điểm yêu cầu, hạn SLA) — không có khu vực nội dung chat; badge số chờ xác nhận trên menu | FR-MOD-017 · [ADMIN_PORTAL §1.5.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CHAT-043 |
| F122-AC-19 | KHI bấm "Xác nhận đã liên hệ phụ huynh", Portal PHẢI bắt buộc chọn `contact_outcome`, ghi chú ≤ 500 (bộ đếm), gọi `acknowledgeSchoolSafetyNotification` với `Idempotency-Key`; NẾU `409 CONFLICT`, THÌ hiển thị "Thông báo đã được xác nhận" và tải lại | FR-MOD-017 · AC-MOD-027 | TC-CHAT-043 |
| F122-AC-20 | Ở NƠI màn hình < 768 px, Portal PHẢI hiển thị danh sách dạng thẻ và màn chi tiết riêng có nút xác nhận như [ADMIN_PORTAL §1.5.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | FR-MOD-017 · NFR-A11Y-003 | TC-UI-004 |

---

### User Story 8 — Chính sách chat cấp trường và lớp (Priority: P2)

`ad.truong.a` thu hẹp giờ chat trường, tắt DM, tắt channel lớp 3B; thấy `match`/`team`/`party` khóa "luôn bật".

**Why this priority**: FR-MOD-004 phía nhà trường; mặc định platform đã an toàn nên không chặn vận hành.

**Independent Test**: Lưu chính sách trường hợp lệ; thử nới lỏng → thông điệp `CHAT_POLICY_RELAX_NOT_ALLOWED`; không có điều khiển theo khối.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-21 | KHI người có `school.settings.write` mở C → Cài đặt trường → Chat policy, Portal PHẢI tải `getChatPolicy` (school hoặc class) và `getEffectiveChatPolicy`, hiển thị theo `channel_type`: bật/tắt `direct`, `world`, `class`, `study_group`; thu hẹp giờ; nâng `strictness`; slow mode; hiển thị tên thật cho bạn cùng lớp; giá trị cấp trên làm giới hạn của control | FR-MOD-004 · [MODERATION §8](../../docs/03-social-multiplayer/MODERATION.md) · [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-018 |
| F122-AC-22 | HỆ THỐNG PHẢI hiển thị `match`/`team`/`party` khóa "Luôn bật" (chỉ cho siết rate, slow mode ≤ 10 s, strictness) và không có bất kỳ control nào theo khối/lớp tuổi; KHI lưu, Portal PHẢI gửi `putChatPolicy` với `If-Match`, `reason` bắt buộc, map `CHAT_POLICY_RELAX_NOT_ALLOWED`/`CHAT_POLICY_ALWAYS_ON`/`VALIDATION_FAILED` về control tương ứng | FR-MOD-004 · ADR-0020 · AGENTS R15 | TC-CHAT-018 |
| F122-AC-28 | KHI School Admin mở Chính sách chat của trường, Portal PHẢI cho chọn `chat_policy_preset` (`standard` hoặc `quick_message_emote_only`) áp cho cả trường kèm mô tả tác động (mọi channel kể cả trận/party chỉ còn quick message + emote), không có lựa chọn theo khối/lớp, bắt buộc `reason`, gọi API ghi preset của F120 và hiển thị lỗi theo `code` | FR-ORG-007 · AC-CHAT-015 · [CHILD_SAFETY §3.5](../../docs/10-security/CHILD_SAFETY.md#35-preset-chỉ-quick-message--emote-adr-0078) · AGENTS R15 | TC-CHAT-048 |

---

### User Story 9 — Club của trường (Priority: P3)

`ad.truong.a` duyệt club học sinh đề xuất, khóa/lưu trữ club, gỡ thành viên; không xem nội dung chat club.

**Why this priority**: Chỉ cần khi trường bật `requires_approval`.

**Independent Test**: Club `pending_approval` hiển thị, duyệt → `active`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-23 | KHI người có `social.club.approve` mở C → An toàn → Club, Portal PHẢI liệt kê `listClubs` (`status`, `school_id` trường mình), cho `approveClub`, `archiveClub`, `removeClubMember` với `Idempotency-Key` và xác nhận; PHẢI không có màn xem nội dung chat club | FR-SOC-004 · [SOCIAL_SYSTEM §6.2](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | TC-SOC-008 |

---

### User Story 10 — Bảo vệ các màn an toàn chat (Priority: P1)

**Why this priority**: Constitution điều V, VI.

**Independent Test**: Ma trận tài khoản × URL; `sm.antoan` chỉ thấy Kiểm duyệt; `ad.truong.a` không có menu E/D.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F122-AC-24 | TRONG KHI người dùng là Safety Moderator, Portal PHẢI chỉ hiển thị module Kiểm duyệt của workspace E (không A, B, C, D, không Live dashboard/Game Analytics); KHI người dùng không có permission tương ứng mở URL màn an toàn chat của E/D, Portal PHẢI hiển thị trang 403 và không tải chunk workspace đó | FR-MOD-003 · ADR-0027 · AGENTS R20 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-01 |
| F122-AC-25 | NẾU API E/D trả `403 MFA_REQUIRED`, THÌ Portal PHẢI step-up MFA và quay lại URL; NẾU gateway nội bộ không kết nối được, THÌ hiển thị "Cần kết nối mạng quản trị (VPN)" | ADR-0027 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-02 |
| F122-AC-26 | HỆ THỐNG PHẢI giới hạn mọi ô nhập theo `maxLength` schema OpenAPI có bộ đếm ký tự, giữ nguyên `Idempotency-Key` khi thử lại, và chỉ dùng component/theme Admin/Ops (không component game) | ADR-0041 · ADR-0047 · AGENTS R10, R32 | TC-TEN-024 |
| F122-AC-29 | KHI menu/action của feature (`menu.live_ops.moderation_cases`, `menu.live_ops.sanctions`, `menu.live_ops.tenant_terms`, `menu.platform_admin.chat_safety`, `menu.platform_admin.moderation_switches`, `menu.school_lms.safety_notifications`, `menu.school_lms.chat_safety` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU case bị người khác nhận trong lúc đang xem, THÌ `assignModerationCase` trả `409`/`412` → Portal tải lại và báo "Case đã được nhận".
- NẾU danh sách case > 1 trang, THÌ phân trang cursor; không tải toàn bộ.
- NẾU System Admin không lọc tenant nào, THÌ banner hiển thị số tenant đang tắt và link tới bảng công tắc, không liệt kê mọi tenant (F122-AC-13).
- NẾU `testSensitiveTerms` trả match có `start/end`, THÌ chỉ tô trên chuỗi người dùng tự nhập, không hiển thị ngoài màn danh mục.
- NẾU trường không có chính sách riêng, THÌ editor hiển thị giá trị kế thừa từ tenant/platform với nhãn "kế thừa".
- NẾU màn hình điện thoại mở danh mục/công tắc, THÌ chỉ đọc kèm "Tác vụ này cần màn hình lớn hơn" ([ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md)).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-MOD-003 | UI hàng đợi case liên tenant, xử lý, sanction | [MODERATION §6](../../docs/03-social-multiplayer/MODERATION.md) · [ADMIN_PORTAL §3.9](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-MOD-004 | UI chính sách chat cấp trường/lớp | [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · [MODERATION §8](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-009 | UI danh mục từ nhạy cảm (platform, tenant, chỉ xem) | [MODERATION §11](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-011 | UI công tắc kiểm duyệt + banner | [MODERATION §12](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-017 | UI gửi/nhận/xác nhận thông báo nhà trường | [MODERATION §6.4](../../docs/03-social-multiplayer/MODERATION.md) · [ADMIN_PORTAL §1.5.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-MOD-013 | UI từ bổ sung, bật/tắt nhóm từ tenant (Safety Team) | [ADMIN_PORTAL §3.9](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-MOD-015 | Hiển thị kết quả hồi quy khi publish | [MODERATION §11.4](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-SOC-004 | UI duyệt/quản lý club của trường | [SOCIAL_SYSTEM §6.2](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Case** (hiển thị): mức, trạng thái, hạn SLA, người được gán, bằng chứng, thao tác.
- **SchoolSafetyNotification**: tóm tắt cho trường, hạn, kết quả liên hệ.
- **SensitiveTermCategory / SensitiveTerm / AllowlistEntry / CatalogVersion**: danh mục và phiên bản.
- **ModerationSwitch**: trạng thái bật/tắt theo platform/tenant, lý do, thời hạn, lịch sử.
- **ChatPolicy (trường/lớp)**: rule theo loại channel trong giới hạn cấp trên.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `school_notification_ack_sla_hours` | Hiển thị hạn xác nhận (server tính) |
| `report_sla` | Hiển thị hạn xử lý case |
| `moderation_switches` | Mặc định bật mọi bước |
| `chat_policy_cache` | Thời gian hiệu lực thay đổi policy (chú thích UI) |
| `portal_session_sso` | Phiên platform, step-up MFA |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không business logic ở UI: quyền, giới hạn, SLA do server; UI chỉ hiển thị/ẩn | AGENTS R1 |
| Workspace E/D chỉ gateway nội bộ + MFA; chunk lazy-load không phải lớp bảo mật | AGENTS R20 · ADR-0027 |
| Không hiển thị nội dung chat ngoài case; không log nội dung term/tin | AGENTS R14, R15 |
| Theme Admin/Ops từ `design/admin-tokens`, không hardcode style | AGENTS R32 · ADR-0047 |
| Accessibility WCAG 2.1 AA | NFR-A11Y-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Safety Moderator xử lý một case (nhận → bằng chứng → sanction → đóng) trong ≤ 6 thao tác chính.
- **SC-002**: 100% thao tác ghi của các màn an toàn chat có `Idempotency-Key` hoặc `If-Match` trong network log E2E.
- **SC-003**: Banner xuất hiện/biến mất ≤ 60 giây sau khi công tắc đổi trong 100% lần thử.
- **SC-004**: 0 màn hình portal hiển thị nội dung chat ngoài module Kiểm duyệt (quét E2E).

## Ngoài phạm vi

- API case/sanction/thông báo: F121; API danh mục, công tắc, từ bổ sung tenant: F116; API chính sách chat, club: F120.
- `student_club_creation`, khung giờ truy cập game, cài đặt trường khác: F052. Thời hạn chat gameplay (D): F057.
- Shell, `BannerSlot`, `TenantFilter`, `TenantTargetField`: F051. Live dashboard, Game Analytics (E): F080. Sticker/GIF (A): chưa có feature.

## Assumptions

- [Proposed] Thư mục: `web/apps/portal/src/workspaces/live-ops/moderation/`, `…/platform-admin/chat-safety/`, `…/school-lms/safety/`; banner `web/apps/portal/src/shell/banners/ModerationDisabledBanner.tsx` (F051 cung cấp slot). Tên thư mục workspace D là `platform-admin` (RB-603/RB-619 đã chốt).
- [Proposed] Danh sách học sinh để chọn trong form thông báo lấy từ `subject_user_ids` của `ModerationCaseDetail` kèm `school_id` (delta F121 `school_ids`); nếu thiếu tên, hiển thị mã học sinh.
- [Proposed] Four-eyes khi publish danh mục platform do server thực thi; UI chỉ hiển thị trạng thái chờ duyệt nếu server trả.

# Feature Specification: F120 — Channel bền vững (world, area, team, DM), club, chính sách theo cấp, tìm kiếm, retention

**Feature Branch**: `120-persistent-channels-clubs` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `chat` · **Mốc**: M5 · **Phụ thuộc**: F118 (và F095 bạn bè/block)
**Input**: Bổ sung các channel ngoài gameplay (world, area, team cố định của club, direct — không có channel lớp, nhóm học tập hay kênh theo khối, ADR-0095), club do học sinh/nhà trường tạo, chính sách chat 3 cấp `platform → tenant → school`, tìm kiếm full-text tiếng Việt, retention ẩn danh hóa cho channel bền vững; không có voice. FR nguồn: FR-CHAT-001, FR-CHAT-004, FR-CHAT-006, FR-CHAT-007, FR-CHAT-016, FR-MOD-004, FR-SOC-004.

## Clarifications

### Session 2026-11-16

- Q: Channel `study_group` lấy thành viên từ đâu? → A: Theo [ADR-0084](../../docs/adr/ADR-0084-nhom-hoc-tap-thuoc-learning-service.md): module chat tiêu thụ `study_group.created/updated/member_changed/deleted` của module learning (F040), chỉ giữ bản chiếu thành viên trong `chat.channel_members`; không gọi đồng bộ module learning, không có bảng `social.study_group_members`. **Thay bởi Session 2026-11-21 (ADR-0095): không còn channel `study_group`.**
- Q: Study group còn là một `club_type`? → A: Không (ADR-0084 mục 4; ADR-0095 bỏ hẳn nhóm học tập LMS). `ClubType` bỏ `study_group` (delta contract); SOCIAL_SYSTEM §6.1, DATABASE_DESIGN §2.11, CHAT_SYSTEM §4 cần sửa theo (báo cáo).
- Q: Club dùng `channel_type` nào? → A: [Confirmed] `team` cố định, `scope_ref = {club_id}`, `is_gameplay = false`, không purge — khớp CHAT_SYSTEM §2 ("team cố định") và CHECK `is_gameplay` của DATABASE_DESIGN §2.12.
- Q: Ai sửa chính sách chat từng cấp? → A: Platform: System Admin; tenant: `tenant.settings.write`; trường: `school.settings.write` (School Admin trường mình) — [ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md). Contract chưa có → delta. (Cấp lớp bỏ theo Session 2026-11-21, ADR-0095.)
- Q: Thời hạn lưu channel bền vững? → A: Mặc định 180 ngày theo CHAT_SYSTEM §2, FR-CHAT-006; tên tham số chưa đăng ký CONFIG_DEFAULTS → đề xuất `persistent_chat_retention_days` (Assumptions).

### Session 2026-11-17

- Q: Mã AC cục bộ của CHAT_SYSTEM và mã lỗi tắt tạo club sau đối soát? → A: Tham chiếu tiêu chí chi tiết CHAT_SYSTEM §13 dùng `AC-CHS-NNN` (RB-505), mã PRD giữ `AC-CHAT-NNN`. Trường tắt tạo club → `403 PERMISSION_DENIED`; `SOCIAL_SCOPE_VIOLATION` chỉ cho vi phạm phạm vi (RB-626). `student_club_creation` = `disabled`/`requires_approval`/`auto` (RB-511).
- Q: Ghi preset cấp trường, giới hạn `allowed_hours` theo giờ giới nghiêm, mặc định tenant `common` (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

### Session 2026-11-20 (owner chốt — OQ-09)

- Khung giờ chat và phạm vi World (OQ-09 đã trả lời) → **Chốt:** chat policy mặc định cấp platform có `allowed_hours` 06:30–21:30 ICT cho channel bền vững (`direct`, club, `world`/`area`; `class`/`study_group` bỏ theo Session 2026-11-21), giá trị đọc từ `chat_allowed_hours` (CONFIG_DEFAULTS), luôn nằm ngoài `platform_curfew`, tenant/trường chỉ thu hẹp (F120-AC-27); `match`/`team`/`party` không có khung giờ riêng. World theo trường (`world_scope = school`), chỉ tenant `unit` bật được `tenant`; không có phạm vi organization/liên tenant; tenant `common` tắt World (F120-AC-12, F120-AC-28 đã đúng).

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: Chỉ plan nhắc MFA của route platform; đọc là gateway nội bộ + role platform. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Channel `class` và `study_group` còn không? Có thay bằng kênh theo khối? → A: Không còn cả hai, và **không** tạo kênh khối (kênh khối gồm hàng trăm học sinh không quen trong trường — rủi ro an toàn trẻ em) — [ADR-0095 §5](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md), FR-CHAT-001, AC-CHAT-001. Loại channel còn lại: `world`, `area`, `match`, `party`, `team`, `direct`; F120 phụ trách `world`, `area`, `team` cố định (club) và `direct`. F120-AC-01, F120-AC-02, F120-AC-03 → **Removed (ADR-0095)**; User Story 1 viết lại thành "thành viên channel bền vững đúng và cô lập" (F120-AC-04 nhận phần "rời thì mất quyền đọc" của AC-02 cũ, áp cho club).
- Q: Module chat còn tiêu thụ `class.synced`, `class.created`, `enrollment.changed`, `study_group.*`? Có cần `student.placement_changed`? → A: Không tiêu thụ các event lớp/nhóm học tập (Deprecated trong [EVENT_DESIGN](../../docs/07-backend/EVENT_DESIGN.md), xóa cùng code — ADR-0095 §7). F120 không cần thông tin trường/khối từ event riêng: quyền World kiểm động theo projection `ref.students` (lõi chat F115 duy trì, nguồn `student.placement_changed`), thành viên club do chính module chat quản lý. F120 không thêm consumer `student.placement_changed`.
- Q: Chính sách chat còn mấy cấp? → A: 3 cấp `platform → tenant → school` (FR-MOD-004, ADR-0095 §5); bỏ `scope_type = class`, tham số `class_id` của effective policy và mọi rule/preset dưới cấp trường. R15 không đổi: không điều kiện theo khối (F120-AC-08). F120-AC-06, F120-AC-10, F120-AC-26, F120-AC-27 viết lại theo 3 cấp.
- Q: Club còn liên quan lớp/nhóm học tập? → A: Không. `ClubType` không có `study_group`; club do nhà trường lập không còn tùy chọn "thêm cả lớp" (`ClubCreate.class_id`, `add_whole_class` bỏ). Delta `ChannelType`, `ClubType`, `ClubCreate`, mô tả receipts trong `chat.v1` do WP-C1 áp trong OpenAPI bundle (contracts §6, **chờ áp**). F120-AC-17 viết lại.
- Q: Danh sách retention và `allowed_hours` của channel bền vững? → A: Chỉ còn `team` cố định (club), `direct` và `world`/`area` (ephemeral, purge ở F117). F120-AC-24 viết lại; trạng thái channel `archived` (chỉ dùng khi study group bị xóa) bỏ khỏi F120 [Proposed — WP-S, không còn nguồn dùng].

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Thành viên channel bền vững đúng và cô lập (Priority: P1) 🎯 MVP

`hs.em.4a` được nhận vào club "Nhà thám hiểm" của TH Demo C → thấy channel club, chỉ đọc tin từ lúc vào; rời club thì mất quyền đọc; nhân sự, học sinh trường khác và tenant khác không đọc được channel đó. (Không có channel lớp hay nhóm học tập — ADR-0095.)

**Why this priority**: Channel bền vững cốt lõi của M5; mọi tính năng khác (tìm kiếm, retention) cần channel có thành viên đúng.

**Independent Test**: `hs.em.4a` vào club (F120-AC-20) → `listMyChannels` có channel `team` cố định của club, `listChannelMessages` chỉ trả tin từ `joined_at`; `leaveClub` → lịch sử `404`; `ad.truong.b` (nhân sự) và `hs.dung.7a` (trường khác, tenant chưa bật liên trường) gọi `listChannelMessages` → `404`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F120-AC-01 | **Removed (ADR-0095)** — không còn thực thể lớp và channel `class` (ADR-0095 §5). Chỉ giữ kiểm âm: HỆ THỐNG PHẢI không tạo channel `class` (yêu cầu loại này → `422 VALIDATION_FAILED`) và module chat PHẢI không tiêu thụ `class.synced`/`class.created` | FR-CHAT-001 · AC-CHAT-001 · ADR-0095 | — |
| F120-AC-02 | **Removed (ADR-0095)** — không còn enrollment và channel lớp; quy tắc "rời thì mất quyền đọc lịch sử" chuyển vào F120-AC-04 cho channel club. Chỉ giữ kiểm âm: module chat PHẢI không tiêu thụ `enrollment.changed` | FR-CHAT-001 · ADR-0095 | — |
| F120-AC-03 | **Removed (ADR-0095)** — không còn nhóm học tập LMS (ADR-0084 bị thay toàn phần) và channel `study_group`. Chỉ giữ kiểm âm: HỆ THỐNG PHẢI không tạo channel `study_group` (→ `422 VALIDATION_FAILED`) và module chat PHẢI không tiêu thụ `study_group.*` | FR-SOC-004 · AC-CHAT-001 · ADR-0095 | — |
| F120-AC-04 | KHI thành viên channel bền vững thay đổi (vào/rời/bị gỡ khỏi club — F120-AC-20; mở channel `direct` — F120-AC-14), module chat PHẢI ghi `joined_at`/`left_at` đúng một lần cho mỗi thay đổi (`Idempotency-Key` cho lệnh, `processed_events` cho event `user.blocked`/`user.unblocked`), không bao giờ thêm nhân sự (`user_type = staff`) làm thành viên channel, và chỉ cho thành viên xem tin từ `joined_at`; sau `left_at` học sinh PHẢI không đọc được lịch sử channel đó (`404`) | [CHAT_SYSTEM §2, §8.3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · ADR-0072 · ADR-0095 | TC-SVC-003 |
| F120-AC-05 | NẾU người không phải thành viên (kể cả cùng tenant, nhân sự hoặc học sinh trường khác) gọi `listChannelMessages`/`sendChannelMessage` trên channel bền vững, THÌ module chat PHẢI trả `404` không lộ tồn tại; RLS PHẢI chặn khi repository bỏ điều kiện tenant | AC-CHS-008 · AC-CHS-009 · AGENTS R8 | TC-CHAT-009 |

---

### User Story 2 — Chính sách chat theo cấp platform → tenant → trường (Priority: P1)

`ad.truong.a` thu hẹp giờ chat của trường, tắt DM cho trường; thử nới rate limit vượt platform, gắn rule cho riêng một khối hoặc tắt chat `match` → bị từ chối. (Không có cấp lớp — ADR-0095.)

**Why this priority**: Không có rule = deny (FR-CHAT-002); channel bền vững chỉ chạy khi có effective policy.

**Independent Test**: `putChatPolicy` cấp `school` hợp lệ → học sinh nhắn ngoài giờ bị `CHAT_NOT_ALLOWED`; gửi rule nới lỏng / có `grade_band` / tắt `party` / `scope_type = class` → 422 đúng `code`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F120-AC-06 | KHI người có quyền đúng cấp (platform: `platform.settings.write`; tenant: `tenant.settings.write`; trường: `school.settings.write` trong trường mình) gọi `putChatPolicy` với `If-Match` và `reason` ở một trong ba cấp `platform`/`tenant`/`school` (cấp khác, kể cả `class` cũ → `422 VALIDATION_FAILED`), module chat PHẢI validate theo `chat-policy.schema.json` v5, lưu version mới, ghi audit before/after và phát `chat_policy.updated` | FR-MOD-004 · [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · [MODERATION §9](../../docs/03-social-multiplayer/MODERATION.md) · ADR-0095 | TC-CHAT-001 |
| F120-AC-07 | NẾU rule cấp dưới nới lỏng hơn cấp trên (không có `allow_override_relax`), THÌ module chat PHẢI trả `422 CHAT_POLICY_RELAX_NOT_ALLOWED` | FR-MOD-004 · AC-CHS-011 | — |
| F120-AC-08 | NẾU rule chứa `grade_band`/`grade_level` hoặc điều kiện theo khối, THÌ module chat PHẢI trả `422 VALIDATION_FAILED`; HỆ THỐNG PHẢI không có nhánh code chat nào đọc khối lớp | FR-CHAT-004 · AC-CHS-012 · AGENTS R15 | TC-CHAT-010 |
| F120-AC-09 | NẾU rule cho `match`/`team` theo trận/`party` đặt `enabled:false`, bỏ `text`/`emoji`/`quick`/`sticker`/`gif`, có `allowed_hours` hoặc `slow_mode_seconds` > 10, THÌ module chat PHẢI trả `422 CHAT_POLICY_ALWAYS_ON` | ADR-0020 · AC-CHS-020 | TC-CHAT-018 |
| F120-AC-10 | HỆ THỐNG PHẢI tính effective policy theo phép giao `platform ∩ tenant ∩ school ∩ sanction ∩ quan hệ` (không có cấp lớp — ADR-0095), không có rule ở cấp nào = deny, cache theo `chat_policy_cache` và vô hiệu khi nhận `chat_policy.updated` | FR-MOD-004 · FR-CHAT-002 · [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | — |
| F120-AC-11 | NẾU người dùng tenant gọi `putChatPolicy` cho tenant/trường khác hoặc cấp `platform`, THÌ module chat PHẢI trả `403 PERMISSION_DENIED`/`404` và audit `authz.denied` | AGENTS R7, R20 · ADR-0027 | TC-TEN-003 |
| F120-AC-26 | KHI School Admin (trường mình) đặt `chat_policy_preset` (`standard` hoặc `quick_message_emote_only`) với `reason`, module chat PHẢI lưu ở cấp trường, ghi audit, phát `chat_policy.updated`, hiệu lực trên mọi instance ≤ 60 giây qua `chat_policy_cache`; preset PHẢI áp cho cả trường, NẾU request gắn preset theo khối hoặc phạm vi nhỏ hơn trường, THÌ PHẢI trả `422 VALIDATION_FAILED` | FR-CHAT-004 · FR-ORG-007 · AC-CHAT-015 · [CHILD_SAFETY §3.5](../../docs/10-security/CHILD_SAFETY.md#35-preset-chỉ-quick-message--emote-adr-0078) · AGENTS R15 | TC-CHAT-048 |
| F120-AC-27 | NẾU rule cấp tenant/trường đặt `allowed_hours` của channel bền vững vượt ra ngoài khoảng ngoài giới nghiêm hiệu lực (ví dụ mở tới 22:00 khi `platform_curfew` bắt đầu 21:30), THÌ module chat PHẢI trả `422 CHAT_POLICY_RELAX_NOT_ALLOWED` | FR-MOD-004 · AC-CHAT-015 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 | TC-CHAT-046 |

---

### User Story 3 — World, Area và tin nhắn riêng (DM) (Priority: P2)

Học sinh trong Main Hub chat ở kênh Area của instance, kênh World của trường; nhắn riêng với bạn bè; bị block thì DM đóng băng.

**Why this priority**: Đủ các loại channel theo FR-CHAT-001/004 (6 loại, cùng channel club); sau thành viên và chính sách vì cần effective policy.

**Independent Test**: `hs.an.3a` gửi World → `hs.binh.3a` nhận, `hs.dung.7a` (tenant khác) không; DM với người chưa là bạn → `CHAT_NOT_ALLOWED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F120-AC-12 | KHI học sinh gửi tin World, module chat PHẢI dùng channel `world` theo trường (`scope_ref.school_id`) hoặc theo tenant khi policy `world_scope = tenant` (chỉ tenant `unit`); kiểm quyền động theo `ref.students`, không lưu `channel_members`; lưu ở `chat.ephemeral_messages` | FR-CHAT-004 · [CHAT_SYSTEM §2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · ADR-0054 | TC-CHAT-002 |
| F120-AC-13 | KHI học sinh gửi tin Area, module chat PHẢI chỉ chấp nhận khi `room_id` do realtime gắn vào lệnh trùng instance hiện tại của kết nối; channel `area` tạo lười theo `room_id` | FR-CHAT-001 · [CHAT_SYSTEM §4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-008 |
| F120-AC-14 | KHI gửi tin `direct` đầu tiên giữa hai học sinh là bạn bè và không block nhau, module chat PHẢI tạo channel `direct` cho cặp đã chuẩn hóa thứ tự; NẾU không là bạn bè hoặc có block, THÌ PHẢI trả `403 CHAT_NOT_ALLOWED` | FR-CHAT-004 · AC-CHS-013 | TC-CHAT-005 |
| F120-AC-15 | KHI nhận `user.blocked`, module chat PHẢI đặt channel `direct` của cặp đó `frozen` (gửi → `CHAT_NOT_ALLOWED`); người ngoài trường ở World mở rộng/club liên trường PHẢI chỉ thấy nickname và avatar | FR-CHAT-004 · AGENTS R16 · [CHAT_SYSTEM §2](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-SOC-006 |
| F120-AC-16 | HỆ THỐNG PHẢI không có endpoint, lệnh realtime hay quyền voice nào; `capabilities.voice` luôn `false` và rule `voice` luôn deny | FR-CHAT-007 · ADR-0070 | TC-AUD-004 |
| F120-AC-28 | TRONG KHI tenant là `common` và System Admin chưa bật lại (`common_tenant_social_defaults`), module chat PHẢI từ chối mở/gửi channel `direct` và `world` bằng `403 CHAT_NOT_ALLOWED`, và channel `area` PHẢI chỉ nhận `quick` + `emoji` (`403 CHAT_KIND_NOT_ALLOWED`); KHI System Admin bật lại cho một tenant `common` cụ thể với `reason`, PHẢI ghi audit và hiệu lực ≤ 60 giây; role tenant PHẢI không bật lại được (`403 PERMISSION_DENIED`) | FR-CHAT-004 · AC-CHAT-015 · [CHILD_SAFETY §3.6](../../docs/10-security/CHILD_SAFETY.md#36-tenant-common-adr-0078) · ADR-0078 | TC-CHAT-047 |

---

### User Story 4 — Club của học sinh và nhà trường (Priority: P2)

School Admin tạo CLB Toán cho trường; khi trường đặt `student_club_creation = requires_approval`, học sinh lớp 4–9 đề xuất club "Nhà thám hiểm", School Admin duyệt, club có channel chat.

**Why this priority**: FR-SOC-004 (Team/club theo policy); phụ thuộc channel bền vững.

**Independent Test**: `hs.em.4a` gọi `createClub` → `pending_approval`; mời trước khi duyệt → `CLUB_NOT_APPROVED`; `approveClub` → `active` + channel `team` cố định.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F120-AC-17 | KHI gọi `createClub` với `Idempotency-Key`, module chat PHẢI cho học sinh tạo chỉ khi chính sách trường `student_club_creation` cho phép (không → `403 PERMISSION_DENIED`), đặt `pending_approval` hoặc `active` theo chính sách, chạy tên/mô tả qua pipeline kiểm duyệt (`block` → `451 CHAT_CONTENT_BLOCKED`, không echo nội dung) và từ chối `club_type` ngoài 4 loại của SOCIAL_SYSTEM §6.1 (kể cả giá trị cũ `study_group`) bằng `422 VALIDATION_FAILED`; club do nhân sự trường tạo PHẢI không có tùy chọn thêm cả lớp (`class_id`/`add_whole_class` không được nhận → `422 VALIDATION_FAILED`) | FR-SOC-004 · [SOCIAL_SYSTEM §6.1](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) · ADR-0095 | TC-SOC-004 |
| F120-AC-18 | KHI người có `social.club.approve` (School Admin trường của club) gọi `approveClub`, module chat PHẢI chuyển `active`, tạo channel `team` cố định gắn `chat_channel_id`, phát `club.approved`; thao tác club trước khi duyệt PHẢI trả `409 CLUB_NOT_APPROVED` | FR-SOC-004 · [SOCIAL_SYSTEM §6.2](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | TC-SOC-004 |
| F120-AC-19 | NẾU club đã đạt `club_max_members` hoặc học sinh đã ở `clubs_per_student_max` club active, THÌ module chat PHẢI trả `409 CLUB_FULL`/`409 CLUB_LIMIT_REACHED` (kiểm trong transaction); mời/xin vào người khác tenant hoặc đã block PHẢI trả `403 SOCIAL_SCOPE_VIOLATION` | FR-SOC-004 · FR-SOC-002 | TC-SOC-005 |
| F120-AC-20 | KHI thành viên tham gia/rời/bị gỡ (`acceptClubJoinRequest`, `acceptClubInvitation`, `leaveClub`, `removeClubMember`), module chat PHẢI cập nhật thành viên channel club trong cùng transaction và phát `club.member_joined`/`club.member_left`; `archiveClub`/khóa club PHẢI chuyển channel `read_only` | FR-SOC-004 · [SOCIAL_SYSTEM §6.5](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | TC-SOC-008 |
| F120-AC-21 | KHI nhận event học tập/game đã kiểm chứng (`activity.graded`, `quest.completed`, `boss.encounter_completed`, `world.discovery_found`), module chat PHẢI cộng `club_goal_progress` của thành viên club active theo `metric`, idempotent theo `event_id`, không lưu điểm/mastery; đạt `target` → `club.goal_completed` | FR-SOC-004 · [SOCIAL_SYSTEM §6.3](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | TC-SOC-007 |

---

### User Story 5 — Tìm kiếm lịch sử chat (Priority: P2)

Học sinh gõ "nhan vat" tìm tin "nhân vật" trong channel club và DM của mình.

**Why this priority**: FR-CHAT-016 (tìm kiếm P5); cần channel bền vững có dữ liệu.

**Independent Test**: Tìm từ khóa không dấu; tin thu hồi/bị chặn/channel khác/tenant khác không xuất hiện.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F120-AC-22 | KHI học sinh gọi `searchMessages` (`q` theo `maxLength` schema), module chat PHẢI chỉ trả tin trong channel người gọi đang là thành viên, lọc `tenant_id`, `deleted_at IS NULL`, `recalled_at IS NULL`, `moderation_state IN ('allowed','masked','unchecked')`, khớp có/không dấu trên `body_display`, phân trang cursor | FR-CHAT-016 · AC-CHS-042 · [CHAT_SYSTEM §8.4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) | TC-CHAT-040 |
| F120-AC-23 | HỆ THỐNG PHẢI không khớp nội dung gốc bị che, không tìm trong `world`/`area`, không trả tin đã ẩn danh hóa và không có tham số tìm liên tenant/toàn hệ thống | FR-MOD-005 · AC-MOD-007 | TC-CHAT-040 |

---

### User Story 6 — Retention channel bền vững (Priority: P2)

Sau 180 ngày, nội dung tin channel club và DM được ẩn danh hóa; tin liên quan case đang mở được giữ làm bằng chứng.

**Why this priority**: FR-CHAT-006, tối thiểu hóa dữ liệu trẻ em; không chặn chức năng chat.

**Independent Test**: Seed tin `created_at` quá hạn → chạy job → `body`/`body_display`/revision bị ghi đè, metadata còn, API không trả; tin thuộc case mở được sao vào `moderation_evidence` trước.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F120-AC-24 | KHI job `persistent-chat-retention` chạy, module chat PHẢI ẩn danh hóa (ghi đè `body`, `body_display`, `message_revisions.body*`, đặt `anonymized_at`) tin `team` cố định (club)/`direct` quá `retention_days` của channel, theo lô, không `DELETE`, không gỡ partition, ghi audit tổng hợp không nội dung | FR-CHAT-006 · ADR-0028 · ADR-0070 · AGENTS R21 | TC-CHAT-024 |
| F120-AC-25 | NẾU tin quá hạn thuộc report/moderation case chưa hết hạn lưu, THÌ job PHẢI sao sang `chat.moderation_evidence` trước khi ẩn danh hóa; API người dùng PHẢI không trả tin `anonymized_at IS NOT NULL` | FR-CHAT-006 · [MODERATION §10](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-022 |

### Edge Cases

- NẾU học sinh rời club rồi được nhận lại, THÌ chỉ xem tin từ `joined_at` mới, không thấy tin trong khoảng đã rời (F120-AC-04).
- NẾU client cũ gửi `channel_type` `class`/`study_group` hoặc `scope_type = class`, THÌ trả `422 VALIDATION_FAILED`, không tạo channel/policy (ADR-0095).
- NẾU trường tắt `direct` bằng policy, THÌ channel còn đó nhưng gửi trả `CHAT_NOT_ALLOWED`, lịch sử vẫn đọc được trong retention.
- NẾU học sinh vừa unblock, THÌ channel `direct` mở lại (`open`) khi nhận `user.unblocked` và cả hai vẫn là bạn bè.
- NẾU tên club trùng trong trường, THÌ cho phép (không unique) nhưng tên phải qua kiểm duyệt (F120-AC-17).
- NẾU policy cấp trường bị đặt cho trường đã ngừng hoạt động hoặc không thuộc tenant của người gọi, THÌ trả `404`.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CHAT-001 | Channel `world`, `area`, `team` cố định (club), `direct` (gameplay: F117); không channel `class`/`study_group`/khối (ADR-0095) | [CHAT_SYSTEM §2, §4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-004 | Một bộ quy tắc cho mọi khối (lớp 1–9); World cùng trường; DM chỉ bạn bè không block | [CHAT_SYSTEM §1, §3.1](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-006 | Lịch sử theo quyền + retention ẩn danh hóa channel bền vững | [CHAT_SYSTEM §6](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-CHAT-007 | Không voice (ràng buộc) | [CHAT_SYSTEM §11](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) · ADR-0070 |
| FR-CHAT-016 | Tìm kiếm full-text có/không dấu (phần P5) | [CHAT_SYSTEM §8.4](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-MOD-004 | Chính sách 3 cấp platform → tenant → school, cấp dưới chỉ siết chặt | [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-SOC-004 | Club/team học sinh theo policy và channel club (nhóm học tập LMS đã bỏ — ADR-0095) | [SOCIAL_SYSTEM §6](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |
| FR-CHAT-002 | Deny-by-default cho channel mới (lõi ở F115) | [CHAT_SYSTEM §3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| FR-SOC-002 | Phạm vi club cùng trường, liên trường khi tenant `unit` bật | [SOCIAL_SYSTEM §7](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) |

### Key Entities

- **Channel** (bền vững): loại, phạm vi (club, trường/tenant, instance, cặp DM), trạng thái `open`/`read_only`/`frozen`, thời hạn lưu.
- **ChannelMember**: học sinh thuộc channel, thời điểm vào/rời, con trỏ đọc, mức thông báo.
- **ChatPolicy**: rule theo `channel_type` ở một cấp (platform/tenant/trường), version, người sửa, lý do.
- **Club**: nhóm bền vững, loại, trạng thái duyệt, owner, giới hạn thành viên, channel chat; **ClubMember**, **ClubJoinRequest**, **ClubInvitation**, **ClubGoal**, **ClubEvent**.
- **RetentionRun**: lần chạy ẩn danh hóa (số tin, số channel, số bằng chứng sao chép).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `chat_max_length` | Độ dài tối đa theo loại channel |
| `chat_rate_per_user` | Trần tổng tin/phút mỗi người |
| `chat_policy_cache` | TTL cache effective policy |
| `club_max_members` / `clubs_per_student_max` | Giới hạn club |
| `student_club_creation` | Chính sách tạo club của học sinh (đọc từ cài đặt trường) |
| `ephemeral_chat_retention` | Lưu World/Area (purge do F117) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Mọi tin (kể cả tên club, mô tả) qua pipeline phía server trước khi phát; không nhánh theo khối | AGENTS R15 · NFR-SAFE-001 · ADR-0020 |
| Tin chỉ phát sau JetStream ack; chỉ `chat-persister` ghi PostgreSQL | AGENTS R25 · ADR-0035 |
| Chỉ soft-delete/ẩn danh hóa; không xóa vật lý channel bền vững | AGENTS R21 · ADR-0028 · ADR-0070 |
| Thành viên channel chỉ từ club (cùng module) và quan hệ bạn bè/chặn; không đọc DB và không gọi đồng bộ module khác | AGENTS R13, R29 · ADR-0095 |
| Không log nội dung chat/PII ở INFO | AGENTS R14 |
| Tìm kiếm p95 theo [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md); pipeline p95 theo `moderation_pipeline_latency` | NFR-SAFE-002 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% học sinh vào club thấy channel club ngay sau khi tham gia và mất quyền đọc ngay sau khi rời (cùng transaction) trong bộ test thành viên.
- **SC-002**: 0 tin nhắn đọc được bởi người ngoài channel trong bộ test cô lập (học sinh, nhân sự, tenant khác).
- **SC-003**: 100% rule chứa điều kiện khối hoặc tắt chat gameplay bị từ chối.
- **SC-004**: Tìm kiếm trả kết quả đúng cho truy vấn có/không dấu trong ≥ 99% mẫu bộ test tiếng Việt.
- **SC-005**: 0 tin bền vững còn nội dung sau hạn lưu (trừ bằng chứng case), 0 dòng bị xóa vật lý.

## Ngoài phạm vi

- Channel/purge gameplay `match`/`team` theo trận/`party`, purge World/Area: F117. Tính năng sửa/thu hồi/reply/sticker…: F118. Lõi channel, JetStream, persister: F115. Pipeline + danh mục: F116.
- Báo cáo, case, sanction, thông báo trường: F121. UI portal chính sách chat trường, duyệt club: F122. UI chat Godot: F119.
- Channel lớp, nhóm học tập, kênh theo khối: không có (ADR-0095). Bạn bè/block: F095. Party từ club: F100.
- Voice chat: ngoài Release (ADR-0070).

## Assumptions

- [Confirmed] Club dùng channel `team` cố định (Clarifications); CHAT_SYSTEM §2 cần ghi rõ.
- [Confirmed] ADR-0095: không channel `class`/`study_group`/khối, policy 3 cấp, club không "thêm cả lớp"; delta `chat.v1` (`ChannelType`, `ClubType`, `ClubCreate`, mô tả receipts) do WP-C1 áp (contracts §6).
- [Confirmed] Thời hạn lưu channel bền vững đọc từ `platform.platform_settings` khóa `chat_retention.persistent_days` (mặc định 180, CHAT_SYSTEM §2); đề xuất đăng ký tên `persistent_chat_retention_days` trong CONFIG_DEFAULTS (task Polish).
- [Confirmed] `student_club_creation` đọc từ projection `ref.schools` (event `school.updated` của identity, F015); giá trị theo SOCIAL_SYSTEM `disabled`/`requires_approval`/`auto` (RB-511 đã chốt theo SOCIAL_SYSTEM).
- [Confirmed] Metric `minutes_learning` của club goal chỉ tính khi `activity.graded` có `duration_seconds` (delta, owner F031); trước đó tạo goal metric này trả `422 VALIDATION_FAILED`.
- OQ-09 (đã trả lời 2026-11-20): `chat_allowed_hours` 06:30–21:30 ICT cho channel bền vững, trường chỉ thu hẹp; `world_chat_scope` = `school` (CONFIG_DEFAULTS, CHAT_SYSTEM §3.1).

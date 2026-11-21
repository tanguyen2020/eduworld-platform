# Feature Specification: F124 — Thực thi xử phạt và vận hành Safety Team: consumer `sanction.applied` (khóa tài khoản), email thông báo vụ việc cho trường, vòng trực, break-glass

**Feature Branch**: `124-safety-enforcement-oncall` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `identity` (khóa tài khoản theo sanction, email nhân sự) + module `chat` (vòng trực p1, break-glass) · **Mốc**: M5 · **Phụ thuộc**: F015, F121 (dùng lại chuỗi khóa/thu hồi của F016)
**Input**: Hoàn tất các phần F121 để ngoài phạm vi: (1) module identity tiêu thụ `sanction.applied`/`sanction.lifted` để khóa/mở khóa tài khoản và thu hồi phiên khi sanction `account_lock`; (2) email (không nội dung) báo nhà trường có thông báo vụ việc mới và nhắc lại; (3) vòng trực on-call 24/7 cho case p1 (phân công tự động, gọi ca chính → ca dự phòng → trưởng ca → System Admin); (4) break-glass của System Admin (lý do, thời hạn, phê duyệt hai người, audit). FR nguồn: FR-MOD-006, FR-AUTH-010, FR-MOD-017, FR-MOD-018, FR-MOD-005, FR-AUD-001.

## Clarifications

### Session 2026-11-16

- Q: Tài khoản đang bị School Admin khóa (nghỉ học) lại nhận `account_lock` từ Safety Team, sau đó trường mở khóa — tài khoản có mở không? → A: Không. Mỗi khóa có **nguồn** riêng; tài khoản chỉ `active` khi không còn khóa hiệu lực nào. Khóa nguồn kiểm duyệt chỉ gỡ qua `revokeSanction`/hết hạn sanction (F121), không gỡ qua `unlockTenantUser`/`unlockUser` ([MODERATION §7](../../docs/03-social-multiplayer/MODERATION.md): khóa quản trị của trường "không phải sanction kiểm duyệt").
- Q: Ai gửi email cho nhà trường? → A: module identity (nơi duy nhất giữ email nhân sự — F121 Clarifications) tiêu thụ event của module chat; bảng giao email không lưu địa chỉ, chỉ `recipient_user_id`, địa chỉ đọc lúc gửi (R14, NFR-PRIV-002).
- Q: "Chưa có người nhận" của case p1 khi đã phân công theo vòng trực? → A: Case được gán tự động (`assigned_via = rotation`) vẫn ở `open` cho tới khi người trực bấm nhận (`assignModerationCase` → `in_review`, F121). "Chưa nhận" = còn `open`.
- Q: Ai duyệt break-glass? → A: [MODERATION §6.1](../../docs/03-social-multiplayer/MODERATION.md) "phê duyệt hai người": một System Admin yêu cầu, một System Admin **khác** duyệt; permission platform-only mới `platform.break_glass.request`, `platform.break_glass.approve` [Confirmed] (R20: tiền tố `platform.*` không bao giờ cấp cho role tenant).
- Q: Lịch trực lưu ở đâu? → A: module chat (module moderation), dữ liệu platform không theo tenant; thời điểm ca, người trực (user_id nhân sự platform), vai trò `primary`/`secondary` [Confirmed]. Trưởng ca (`moderation.case.lead`, F121) quản lý lịch.

### Session 2026-11-17

- Q: Xác nhận sau đối soát (RB-621, RB-510)? → A: Khóa nhiều nguồn lưu ở `identity.account_locks` ([DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)); F016 dùng chung bảng này thay một `status_reason`. Metric bypass là `eduworld_tenant_filter_bypass_total{module,reason}`.

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F124-AC-16, AC-21: bỏ điều kiện "token thiếu MFA" và mã `403 MFA_REQUIRED` của API break-glass; `safety_moderator`, role tenant vẫn `403 PERMISSION_DENIED`. T033, T042, plan, quickstart, contracts nhắc MFA là lịch sử. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Sanction `account_lock` khóa tài khoản và thu hồi phiên (Priority: P1) 🎯 MVP

`sm.truongca` áp `account_lock` cho học sinh `hs.binh.3a` (case p1 grooming). Trong 1 phút học sinh mất phiên game/WS, không đăng nhập lại được; khi trưởng ca thu hồi sanction, học sinh đăng nhập lại được — trừ khi trường cũng đang khóa.

**Why this priority**: Sanction mạnh nhất của Safety Team không có hiệu lực nếu module identity không thực thi (F121 chỉ phát event).

**Independent Test**: Phát `sanction.applied` (`account_lock`) → REST `403 ACCOUNT_LOCKED` ≤ 60 s, Keycloak `enabled=false` + logout ≤ 1 phút; `sanction.lifted` → `user.access_restored`; khi có thêm khóa của trường → vẫn `locked`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F124-AC-01 | KHI nhận `sanction.applied` có `sanction_type = account_lock` cho một học sinh, module identity PHẢI tạo khóa nguồn `moderation_sanction` (tham chiếu `sanction_id`, `case_id`), đặt trạng thái tài khoản `locked`, ghi audit `user.locked` (actor hệ thống, `reason_code = moderation_sanction`) và phát `user.status_changed` + `user.access_revoked` trong cùng transaction để chạy chuỗi thu hồi của F016 (REST ≤ 60 s, realtime, Keycloak disable + logout ≤ 1 phút) | FR-MOD-006 · FR-AUTH-010 · AC-AUTH-006 · [ACCOUNT_PROVISIONING §6](../../docs/09-identity/ACCOUNT_PROVISIONING.md) · [MODERATION §7](../../docs/03-social-multiplayer/MODERATION.md) | TC-AUTH-006 |
| F124-AC-02 | KHI nhận `sanction.lifted` của một sanction `account_lock` (thu hồi hoặc hết `ends_at`), module identity PHẢI giải phóng đúng khóa nguồn đó; NẾU không còn khóa hiệu lực nào khác, THÌ PHẢI chuyển `active` và phát `user.access_restored`; ngược lại PHẢI giữ `locked` | FR-MOD-006 · FR-AUTH-010 | TC-AUTH-006 |
| F124-AC-03 | KHI cùng event được giao nhiều lần hoặc `sanction.lifted` đến trước `sanction.applied` của cùng `sanction_id`, module identity PHẢI xử lý đúng một lần (`processed_events`) và không khóa lại tài khoản bằng sanction đã gỡ | FR-AUTH-010 · ADR-0042 | TC-SVC-003 |
| F124-AC-04 | NẾU `sanction_type` khác `account_lock`, người bị áp không phải học sinh, hoặc `tenant_id` của envelope không khớp membership của học sinh, THÌ module identity PHẢI không đổi trạng thái tài khoản; trường hợp lệch tenant PHẢI ghi audit `security.tenant_mismatch` và chuyển event vào DLQ | AGENTS R8 · NFR-SEC-006 | TC-TEN-001 |
| F124-AC-05 | NẾU người có `tenant.user.lock` gọi `unlockTenantUser`, hoặc System Admin gọi `unlockUser`, khi tài khoản còn khóa nguồn `moderation_sanction`, THÌ module identity PHẢI chỉ gỡ khóa nguồn của người gọi (nếu có), giữ `locked` và trả `409 ACCOUNT_LOCK_HELD_BY_SAFETY` | FR-AUTH-010 · [MODERATION §7](../../docs/03-social-multiplayer/MODERATION.md) | — |
| F124-AC-06 | HỆ THỐNG PHẢI cho học sinh bị khóa chỉ nhận `403 ACCOUNT_LOCKED` trung tính; nhân sự trường trong scope xem được nguồn khóa (`moderation_sanction`) nhưng không thấy `case_id`, lý do nội bộ hay nội dung case | FR-AUTH-010 · AGENTS R14, R16 · [ACCOUNT_PROVISIONING §6](../../docs/09-identity/ACCOUNT_PROVISIONING.md) | — |

---

### User Story 2 — Email báo nhà trường có thông báo vụ việc (Priority: P1)

Safety Team gửi thông báo vụ việc cho TH Demo A; `ad.truong.a` nhận email "Có thông báo an toàn mới cần xác nhận" kèm link portal, không có tên học sinh hay nội dung; quá hạn chưa xác nhận thì nhận email nhắc.

**Why this priority**: FR-MOD-017 — trường là đầu mối liên hệ phụ huynh (OQ-13); thông báo chỉ trong portal dễ bị bỏ lỡ.

**Independent Test**: Phát `moderation.school_notified` cho TH Demo A → mailpit có đúng 1 email/School Admin của trường; `ad.truong.b` không nhận; nội dung email không chứa họ tên/mã học sinh, mã case, loại vụ việc.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F124-AC-07 | KHI nhận `moderation.school_notified`, module identity PHẢI tạo một lượt gửi email mẫu `school_safety_notification` cho mỗi nhân sự có UserRole `school_admin` còn hiệu lực ở `school_id` của event (không có thì `tenant_admin` của tenant); email PHẢI chỉ gồm tiêu đề cố định, tên trường, hạn xác nhận và link tới màn "An toàn → Thông báo vụ việc" | FR-MOD-017 · [MODERATION §6.4](../../docs/03-social-multiplayer/MODERATION.md) · AC-MOD-027 | TC-CHAT-043 |
| F124-AC-08 | KHI job nhắc lại của F121 phát `moderation.school_notification_reminded`, module identity PHẢI gửi email mẫu `school_safety_reminder` (lần nhắc thứ n) tới cùng nhóm người nhận | FR-MOD-017 · AC-MOD-027 | TC-CHAT-043 |
| F124-AC-09 | HỆ THỐNG PHẢI không đưa vào email, log INFO, audit hay bảng giao email bất kỳ họ tên, mã học sinh, mã case, `category_code`, tóm tắt vụ việc hay nội dung chat; bảng giao email PHẢI chỉ lưu `recipient_user_id`, địa chỉ email đọc lúc gửi | AGENTS R14 · FR-MOD-005 · NFR-PRIV-002 | TC-CHAT-009 |
| F124-AC-10 | HỆ THỐNG PHẢI gửi đúng một email cho mỗi (`event_id`, người nhận, mẫu); NẾU SMTP lỗi, THÌ PHẢI thử lại có backoff tới số lần tối đa rồi đánh dấu `failed`, tăng `eduworld_email_deliveries_total{template,status}` và bắn alert `EmailDeliveryFailing`, không chặn consumer | ADR-0042 · AGENTS R10, R18 | TC-SVC-003 |
| F124-AC-11 | NẾU người nhận không còn UserRole hiệu lực, bị khóa/soft-delete, hoặc thuộc tenant/trường khác event, THÌ module identity PHẢI bỏ qua (`skipped`) và không gửi | AGENTS R8 · NFR-SEC-006 | TC-TEN-003 |

---

### User Story 3 — Vòng trực on-call 24/7 cho case p1 (Priority: P1)

Trưởng ca lên lịch tuần: `sm.antoan` ca chính, `sm.truongca` ca dự phòng. Case p1 lúc 23:00 tự gán cho ca chính và gửi email gọi; 15 phút chưa nhận → gọi ca dự phòng + alert SMS; tiếp tục chưa nhận → trưởng ca; quá hạn phản hồi → System Admin.

**Why this priority**: FR-MOD-018, AC-MOD-028; F121 chỉ có alert và nhận thủ công.

**Independent Test**: Đồng hồ giả: tạo ca, tạo case p1 → `assigned_via = rotation`, `moderation.oncall_paged{level=primary}`; tiến 1×, 2× `p1_unclaimed_alert_minutes` → `secondary`, `lead`; nhận case → không gọi tiếp.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F124-AC-12 | KHI người có `moderation.case.lead` gọi `createModerationOncallShift` (`moderator_user_id`, `oncall_role` primary/secondary, `starts_at` < `ends_at`, `note` ≤ 500) với `Idempotency-Key`, module chat PHẢI kiểm người trực là nhân sự platform có `moderation.case.act` (qua `resolvePrincipal`), từ chối ca chồng lấn cùng vai trò bằng `409 ONCALL_SHIFT_OVERLAP`, lưu ca và ghi audit `moderation.oncall_shift.created`; `deleteModerationOncallShift` PHẢI soft-delete và audit | FR-MOD-018 · [CHILD_SAFETY §11](../../docs/10-security/CHILD_SAFETY.md) · AGENTS R10, R21 | TC-CHAT-007 |
| F124-AC-13 | KHI case `p1` được tạo và có ca `primary` hiện hành, module chat PHẢI đặt `assignee_moderator_id` = người trực, `assigned_via = rotation` (case giữ `open`), phát `moderation.case_assigned` và `moderation.oncall_paged` (`level = primary`); module identity PHẢI gửi email mẫu `moderation_oncall_page` chỉ chứa mức ưu tiên và link case, không chứa tenant, học sinh hay nội dung | FR-MOD-018 · [EVENT_DESIGN §4](../../docs/07-backend/EVENT_DESIGN.md) · AGENTS R14 | TC-CHAT-042 |
| F124-AC-14 | TRONG KHI case `p1` còn `open`, job `oncall-escalator` PHẢI gọi lần lượt ca `secondary` khi quá `p1_unclaimed_alert_minutes`, trưởng ca (`level = lead`) khi quá gấp đôi ngưỡng đó, và System Admin (`level = system_admin`) khi quá hạn phản hồi đầu tiên theo `report_sla`; mỗi mức gọi đúng một lần, ghi `moderation_actions` (`oncall_page`) | FR-MOD-018 · [MODERATION §6.3](../../docs/03-social-multiplayer/MODERATION.md) · AC-MOD-028 | — |
| F124-AC-15 | NẾU không có ca `primary` (hoặc `secondary`) hiện hành, THÌ module chat PHẢI gọi thẳng mức kế tiếp còn người, đặt gauge `eduworld_moderation_oncall_coverage{oncall_role}` = 0 và alert `ModerationOncallGap` PHẢI bắn; alert `ModerationP1Unclaimed` (F121) PHẢI định tuyến tới receiver `safety-oncall` của Alertmanager | FR-MOD-018 · [MONITORING §5](../../docs/11-operations/MONITORING.md) | — |
| F124-AC-16 | NẾU người không có `moderation.case.lead` gọi tạo/xóa ca, THÌ module chat PHẢI trả `403 PERMISSION_DENIED`; `listModerationOncallShifts` PHẢI yêu cầu `moderation.case.read`; role tenant gọi `/admin/moderation/oncall-shifts*` PHẢI không có route ở gateway public và nhận `403` khi gọi thẳng | AGENTS R7, R20 · ADR-0072 | TC-CMS-WS-02 |

---

### User Story 4 — Break-glass của System Admin (Priority: P2)

Có yêu cầu pháp lý về tin nhắn DM của một học sinh không thuộc case nào. `sa.admin` tạo yêu cầu (lý do, phạm vi, khoảng thời gian, thời hạn); `sa.admin2` duyệt; `sa.admin` đọc tin trong phạm vi tới khi hết hạn; mọi lượt đọc có audit.

**Why this priority**: [MODERATION §6.1](../../docs/03-social-multiplayer/MODERATION.md), [CHILD_SAFETY §7](../../docs/10-security/CHILD_SAFETY.md) — đường duy nhất đọc chat ngoài case; hiếm dùng nhưng phải có kiểm soát.

**Independent Test**: Yêu cầu → tự duyệt `403`; người thứ hai duyệt → đọc được tin trong phạm vi, ngoài phạm vi `403`; quá `expires_at` → `403 BREAK_GLASS_NOT_ACTIVE`; audit đủ các lượt.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F124-AC-17 | KHI người có `platform.break_glass.request` gọi `createBreakGlassRequest` (`X-Tenant-Id` đích, `scope_type` channel/user, `scope_id`, `window_from` < `window_to`, `duration_minutes`, `reason` 20–1000 ký tự) với `Idempotency-Key`, module chat PHẢI lưu yêu cầu `pending`, ghi audit `break_glass.requested` và phát `break_glass.requested` để module identity email mọi System Admin khác có `platform.break_glass.approve` (không nội dung lý do) | FR-MOD-005 · FR-AUD-001 · [MODERATION §6.1, §9](../../docs/03-social-multiplayer/MODERATION.md) | TC-TEN-004 |
| F124-AC-18 | KHI người có `platform.break_glass.approve` **khác** người yêu cầu gọi `approveBreakGlassRequest`/`rejectBreakGlassRequest` (`decision_note` ≤ 1000, `If-Match`, `Idempotency-Key`), module chat PHẢI chuyển `approved` (đặt `expires_at`) hoặc `rejected` và audit; người yêu cầu tự duyệt PHẢI nhận `403 PERMISSION_DENIED`; yêu cầu không còn `pending` PHẢI trả `409 BREAK_GLASS_NOT_PENDING` | FR-MOD-005 · [CHILD_SAFETY §7](../../docs/10-security/CHILD_SAFETY.md) | — |
| F124-AC-19 | TRONG KHI grant `approved` và chưa hết hạn, module chat PHẢI cho đúng người yêu cầu gọi `listBreakGlassMessages` chỉ trong phạm vi (tin của channel, hoặc tin do người dùng gửi, trong tenant đích và khoảng `window_from`–`window_to`), đọc qua `tenancy.Bypass(ctx, "break_glass")`, ghi audit `break_glass.messages_viewed` mỗi lượt và tăng `eduworld_tenant_filter_bypass_total{reason="break_glass"}`; ngoài phạm vi, người khác, grant hết hạn/thu hồi PHẢI trả `403 BREAK_GLASS_NOT_ACTIVE` | FR-MOD-005 · AGENTS R27 · ADR-0039 | TC-TEN-023 |
| F124-AC-20 | KHI quá `expires_at` hoặc System Admin gọi `revokeBreakGlassGrant` (`reason` ≤ 1000), module chat PHẢI kết thúc grant ngay (`expired`/`revoked`), audit; `listBreakGlassGrants` PHẢI cho mọi người có `platform.break_glass.approve` xem toàn bộ yêu cầu (người, lý do, phạm vi, số tin đã xem) để hậu kiểm | FR-AUD-001 · FR-MOD-005 | — |
| F124-AC-21 | NẾU `safety_moderator`, role tenant hoặc token thiếu MFA gọi API break-glass, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED`/`403 MFA_REQUIRED` (route chỉ ở gateway nội bộ); NẾU chuỗi vượt `maxLength` hoặc `duration_minutes`/khoảng thời gian vượt giới hạn, THÌ PHẢI trả `422 VALIDATION_FAILED` | AGENTS R20, R28 · ADR-0027 | TC-TEN-024 |
| F124-AC-22 | HỆ THỐNG PHẢI không chép nội dung tin vào audit, event hay log của break-glass; tin gameplay đã purge (ADR-0033) PHẢI không được khôi phục để phục vụ break-glass | FR-MOD-008 · AGENTS R14, R21 · ADR-0033 | TC-CHAT-022 |

### Edge Cases

- NẾU `account_lock` có `ends_at` NULL ("theo vụ"), THÌ khóa giữ tới khi `revokeSanction` (F124-AC-02).
- NẾU profile `worker-chat` ngừng nên không phát `sanction.lifted` khi hết hạn, THÌ tài khoản giữ khóa lâu hơn (hướng an toàn); F121 phát bù khi chạy lại.
- NẾU học sinh bị `account_lock` đang trong bài kiểm tra, THÌ attempt kết thúc theo chuỗi thu hồi F016 (không xử lý riêng ở F124).
- NẾU School Admin vừa là người nhận email vừa bị khóa, THÌ bỏ qua người đó (F124-AC-11); không còn người nhận nào → metric `skipped` + alert `SchoolSafetyNotificationNoRecipient`.
- NẾU case p1 thuộc tenant `common`, THÌ vòng trực vẫn áp (không có email trường — F121-AC-21).
- NẾU người trực rời ca giữa chừng (ca bị xóa), THÌ case đang gán `rotation` còn `open` được job gọi tiếp mức kế tiếp.
- NẾU grant break-glass hết hạn giữa một trang kết quả, THÌ trang kế tiếp trả `403 BREAK_GLASS_NOT_ACTIVE`.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-MOD-006 | Thực thi sanction `account_lock` ở module identity | [MODERATION §7](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-AUTH-010 | Khóa nguồn kiểm duyệt → `user.access_revoked`; mở khóa theo nguồn | [ACCOUNT_PROVISIONING §6](../../docs/09-identity/ACCOUNT_PROVISIONING.md) |
| FR-MOD-017 | Email báo có thông báo vụ việc và nhắc lại (không nội dung) | [MODERATION §6.4](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-018 | Lịch trực, phân công tự động, chuỗi gọi on-call p1 | [MODERATION §6.3](../../docs/03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY §11](../../docs/10-security/CHILD_SAFETY.md) |
| FR-MOD-005 | Break-glass là đường duy nhất đọc chat ngoài case, có audit | [MODERATION §6.1, §9](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-AUD-001 | Audit khóa theo sanction, lịch trực, break-glass | [MODERATION §9](../../docs/03-social-multiplayer/MODERATION.md) |

### Key Entities

- **AccountLock**: một khóa tài khoản theo nguồn (quản trị trường, quản trị platform, SIS, sanction kiểm duyệt), tham chiếu nguồn, thời điểm tạo/giải phóng.
- **EmailDelivery**: một lượt gửi email cho nhân sự theo mẫu, nguồn event, trạng thái, số lần thử.
- **OncallShift**: ca trực Safety Team (người trực, vai trò chính/dự phòng, thời gian).
- **BreakGlassGrant**: yêu cầu/quyền truy cập khẩn cấp (người yêu cầu, người duyệt, phạm vi, khoảng thời gian tin, hạn, trạng thái).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `p1_unclaimed_alert_minutes` | Mốc gọi ca dự phòng (1×) và trưởng ca (2×) |
| `report_sla` | Mốc gọi System Admin (quá hạn phản hồi đầu tiên p1) |
| `school_notification_ack_sla_hours` | Hạn xác nhận hiển thị trong email |
| `principal_cache`, `principal_cache_ttl` | Độ trễ thu hồi quyền REST khi khóa |
| `idempotency_record_ttl` | Khóa idempotency của API ghi |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Khóa/mở khóa qua event, không gọi đồng bộ chat ↔ identity | AGENTS R29 · ADR-0042 |
| Bypass `break_glass` chỉ trong handler break-glass của System Admin, audit + metric | AGENTS R20, R27 · ADR-0039 |
| `/admin/moderation/oncall-shifts*`, `/admin/break-glass/*` chỉ gateway nội bộ, `azp = eduworld-portal` + MFA | AGENTS R20 · ADR-0027 |
| Không PII/nội dung case trong email, log INFO, audit | AGENTS R14 · NFR-PRIV-002 |
| Thao tác ghi có `Idempotency-Key` + audit | AGENTS R10 · NFR-SEC-004 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% sanction `account_lock` làm tài khoản mất quyền truy cập REST ≤ 60 giây và phiên Keycloak ≤ 1 phút trong bộ test.
- **SC-002**: 0 lần mở khóa ngoài ý muốn khi còn khóa nguồn khác (ma trận nguồn khóa × thao tác mở).
- **SC-003**: ≥ 99% email thông báo vụ việc đến hộp thư (mailpit/staging) ≤ 5 phút sau khi gửi thông báo; 0 email chứa thông tin học sinh.
- **SC-004**: Trong diễn tập hằng quý, 100% case p1 có người trực được gọi ngay khi tạo và mức kế tiếp được gọi đúng mốc.
- **SC-005**: 100% lượt đọc break-glass có bản ghi audit; 0 lượt đọc khi grant không `approved` hoặc đã hết hạn.

## Ngoài phạm vi

- Tạo/thu hồi sanction, case, thông báo trường, job nhắc lại và alert `ModerationP1Unclaimed`: F121 (F124 chỉ thêm event `moderation.school_notification_reminded`, `moderation.oncall_paged`, cột `assigned_via` và job gọi trực).
- Cơ chế lock/unlock, Keycloak disable/logout, đóng WS, kết thúc game session: F016, F090.
- Giao diện lịch trực và break-glass trên portal: chưa có feature — đề xuất bổ sung vào F122 (workspace E/D).
- Định tuyến Alertmanager chung, kênh SMS: F131 (F124 thêm route `safety-oncall`).
- Liên hệ phụ huynh tenant `common`, quy trình pháp lý/consent: deferred — ADR-0082 (OQ-13).

## Assumptions

- [Confirmed] Permission mới platform-only `platform.break_glass.request`, `platform.break_glass.approve` (gán `system_admin`); cập nhật ADMIN_PORTAL §2.2 và `src/backend/pkg/authz/catalog.go`.
- [Confirmed] Tham số mới cần đăng ký CONFIG_DEFAULTS (R33): `break_glass_max_duration_hours` = 4, `break_glass_max_window_days` = 30, `oncall_shift_max_hours` = 168, `email_delivery_max_attempts` = 8 (backoff lũy thừa tới 1 giờ). Code đọc từ `platform.platform_settings`, không hardcode.
- [Confirmed] Mã lỗi mới: `ACCOUNT_LOCK_HELD_BY_SAFETY` (409), `ONCALL_SHIFT_OVERLAP` (409), `BREAK_GLASS_NOT_PENDING` (409), `BREAK_GLASS_NOT_ACTIVE` (403) — thêm API_DESIGN §4.1 và `ProblemCode`.
- [Confirmed] Không có School Admin → gửi Tenant Admin (MODERATION §6.4 chỉ nêu School Admin).
- Seed local [Confirmed]: `sm.truongca` (F121), `sa.admin2` (system_admin thứ hai, duyệt break-glass).
- Email gửi qua SMTP cấu hình bằng env/secret (local: mailpit, [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).

# Feature Specification: F121 — Moderation: báo cáo, sanction, hàng đợi Safety Team liên tenant, thông báo vụ việc cho trường

**Feature Branch**: `121-moderation-safety-team` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `chat` (module moderation) · **Mốc**: M5 · **Phụ thuộc**: F116
**Input**: Học sinh báo cáo tin/người dùng → case gắn tenant/trường; Platform Safety Team (`safety_moderator`, MFA) xử lý hàng đợi liên tenant **chỉ trong module moderation** qua `tenancy.Bypass(ctx, "moderation_case")`; sanction có thời hạn; thông báo tóm tắt (không chat thô) cho nhà trường để liên hệ phụ huynh; trực 24/7 cho case p1. FR nguồn: FR-MOD-002, FR-MOD-003, FR-MOD-005, FR-MOD-006, FR-MOD-008, FR-MOD-016, FR-MOD-017, FR-MOD-018.

## Clarifications

### Session 2026-11-16

- Q: Enum trạng thái case, loại sanction, lý do báo cáo khác nhau giữa OpenAPI và MODERATION? → A: Theo tài liệu sở hữu [MODERATION §5–§7](../../docs/03-social-multiplayer/MODERATION.md): case `open`/`in_review`/`escalated`/`resolved`/`reopened`; sanction `warning`/`mute`/`restrict_text`/`restrict_dm`/`restrict_multiplayer`/`restrict_competition`/`account_lock`; reason `rude_language`/`bullying`/`asked_personal_info`/`spam`/`inappropriate_content`/`cheating`/`other`. OpenAPI sửa ở Phase 2 (delta "Cần sửa").
- Q: Ai đọc nội dung case? → A: Người được gán (`assignee_moderator_id`) và trưởng ca; `system_admin` có `moderation.*` (ADMIN_PORTAL §2.3). Trưởng ca biểu diễn bằng permission đề xuất `moderation.case.lead` [Proposed] (catalog chưa có — báo cáo).
- Q: Phân công theo vòng trực tự động? → A: Release này: nhận thủ công (`assignModerationCase` tự gán) và trưởng ca gán cho người khác (`assigned_via = manual`); vòng trực tự động [Proposed] làm sau, không chặn FR-MOD-018 (alert on-call vẫn chạy).
- Q: Email báo "có thông báo mới" cho School Admin? → A: Email không chứa tên/nội dung; module chat không giữ email nhân sự nên module identity gửi khi tiêu thụ `moderation.school_notified` (ngoài phạm vi, báo cáo). Feature này lưu và phát event.
- Q: Ngữ cảnh snapshot báo cáo ±10 hay ±20 tin? → A: Theo MODERATION §5 (sở hữu): ±10 tin trong channel lúc tạo report; moderator xem thêm ±200 tin ngữ cảnh khi channel còn (MODERATION §6.1). DATABASE_DESIGN ghi ±20 — mâu thuẫn, báo cáo.

### Session 2026-11-17

- Q: Tên metric bypass và mã AC CHAT_SYSTEM sau đối soát? → A: Metric `eduworld_tenant_filter_bypass_total{module,reason}` (RB-510); tiêu chí chi tiết CHAT_SYSTEM §13 tham chiếu bằng `AC-CHS-NNN` (RB-505); `sender_nickname` `maxLength` 32 (RB-302).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Học sinh báo cáo, hệ thống tạo case (Priority: P1) 🎯 MVP

`hs.binh.3a` báo cáo tin "cho mình số điện thoại nhà bạn" của một bạn trong party; nhận lời cảm ơn; case p1 xuất hiện trong hàng đợi Safety Team gắn TH Demo A.

**Why this priority**: Không có report → không có case → mọi chức năng moderation khác vô nghĩa.

**Independent Test**: `createReport` → `listModerationCases` (`sm.antoan`) có case đúng `tenant_id`/`school_ids`, `severity`, `sla_due_at`; báo cáo lần 2 cùng target trong 24 h tăng `report_count`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F121-AC-01 | KHI học sinh gọi `createReport` (`target_type` message/user, `reason`, `note` ≤ `maxLength`, `context` match/party/channel) với `Idempotency-Key`, module chat PHẢI lưu report kèm snapshot ±10 tin của channel đó (gồm nội dung gốc và mọi revision nếu tin đã sửa/thu hồi), trả `201 {report_id, status: open}` và thông điệp cảm ơn theo `Accept-Language`, không tiết lộ danh tính người báo cáo | FR-MOD-002 · [MODERATION §5](../../docs/03-social-multiplayer/MODERATION.md) · AC-CHS-033 | TC-CHAT-006 |
| F121-AC-02 | KHI tạo report, module chat PHẢI gộp vào case `open`/`in_review` cùng target trong 24 giờ (tăng `report_count`) hoặc tạo case mới gắn `tenant_id` + `school_ids` của học sinh liên quan, `severity` theo [MODERATION §6.2](../../docs/03-social-multiplayer/MODERATION.md) và `sla_due_at` theo `report_sla`, ghi outbox `report.created` | FR-MOD-003 · AC-CHS-007 · AC-MOD-005 | TC-CHAT-006 |
| F121-AC-03 | KHI nhận `chat.message_held` (`hold_for_review`/`block_and_alert`) hoặc tín hiệu `async_analysis`/tự hại từ F116, module chat PHẢI tạo case với `source` tương ứng (`filter_alert`/`async_analysis`/`self_harm_signal`), idempotent theo `event_id` | FR-MOD-003 · [MODERATION §3–§4](../../docs/03-social-multiplayer/MODERATION.md) | TC-SVC-003 |
| F121-AC-04 | KHI case mới có `severity = p1`, module chat PHẢI áp ngay hạn chế tạm thời người bị báo cáo (sanction hệ thống `mute` phạm vi mọi channel, chặn lời mời/kết bạn mới, `reason_code = case_hold`) và gỡ khi case kết thúc nếu không chuyển thành sanction | FR-MOD-018 · [MODERATION §6.3](../../docs/03-social-multiplayer/MODERATION.md) · AC-MOD-028 | TC-CHAT-043 |
| F121-AC-05 | NẾU tin gameplay được báo cáo khi channel chưa bị purge hoặc tin đã thu hồi trong cửa sổ báo cáo, THÌ module chat PHẢI tạo case bình thường; NẾU target không tồn tại/ngoài channel người báo cáo là thành viên/khác tenant, THÌ PHẢI trả `404` | [MODERATION §5](../../docs/03-social-multiplayer/MODERATION.md) · AC-MOD-024 · AGENTS R8 | TC-CHAT-030 |

---

### User Story 2 — Hàng đợi liên tenant và đọc case qua Bypass (Priority: P1)

`sm.antoan` mở hàng đợi, lọc p1 của 2 tenant, nhận case, xem bằng chứng và tin ngữ cảnh; mỗi lần xem có audit.

**Why this priority**: FR-MOD-003, FR-MOD-016 — kiểm duyệt tập trung là quyết định ADR-0072.

**Independent Test**: `sm.antoan` liệt kê case 2 tenant, mở tin của case → audit + metric bypass; cùng token gọi `getSchoolDashboard` → 403.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F121-AC-06 | KHI người có `moderation.case.read` (Safety Moderator, System Admin; token `eduworld-portal` + MFA, gateway nội bộ) gọi `listModerationCases` không có `X-Tenant-Id`, module chat PHẢI đọc liên tenant qua `tenancy.Bypass(ctx, "moderation_case")`, lọc `tenant_ids`, `status`, `severity`, `assignee`, `category_code`, phân trang cursor, ghi audit và tăng `eduworld_tenant_filter_bypass_total{reason="moderation_case"}` | FR-MOD-016 · ADR-0072 · [ADMIN_PORTAL §2.5b](../../docs/04-admin-portal/ADMIN_PORTAL.md) · AC-MOD-026 | TC-CHAT-042 |
| F121-AC-07 | KHI người được gán hoặc trưởng ca gọi `getModerationCase`/`listModerationCaseMessages`, module chat PHẢI trả snapshot bằng chứng + ±200 tin ngữ cảnh của channel liên quan (nội dung gốc, revision, trạng thái), tìm `q` chỉ trong phạm vi đó, ghi audit `moderation.case.viewed`/`moderation.case.messages_viewed`/`moderation.case.searched` kèm `tenant_id` đích; NẾU không phải người được gán/trưởng ca, THÌ PHẢI trả `403 PERMISSION_DENIED` | FR-MOD-016 · FR-MOD-005 · [MODERATION §6.1](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-040 |
| F121-AC-08 | KHI gọi `assignModerationCase` với `Idempotency-Key` và `X-Tenant-Id` = tenant của case, module chat PHẢI đặt `assignee_moderator_id` (tự nhận, hoặc trưởng ca gán người khác), chuyển `open → in_review`, phát `moderation.case_assigned`; case đã `resolved` PHẢI trả `409 MODERATION_CASE_CLOSED` | FR-MOD-003 · [MODERATION §6.2](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-007 |
| F121-AC-09 | NẾU token `safety_moderator` gọi API ngoài module moderation (LMS, nội dung, hồ sơ học sinh, login events, `searchMessages`), THÌ module đích PHẢI trả `403 PERMISSION_DENIED`; NẾU role tenant gọi `/admin/moderation/*`, THÌ gateway public PHẢI không có route và backend trả `403` | FR-MOD-016 · AC-MOD-006 · AGENTS R20 | TC-CHAT-042 |
| F121-AC-10 | NẾU token thiếu MFA (`acr`) hoặc `azp ≠ eduworld-portal`, THÌ module chat PHẢI trả `403 MFA_REQUIRED`/`403 PERMISSION_DENIED` và audit `authz.denied` | FR-MOD-003 · ADR-0027 | TC-CMS-WS-02 |

---

### User Story 3 — Xử lý case và sanction (Priority: P1)

Moderator gỡ tin vi phạm, áp `restrict_text` 7 ngày, đóng case; học sinh bị hạn chế thấy lý do phù hợp lứa tuổi; người báo cáo nhận "Báo cáo của em đã được xử lý".

**Why this priority**: FR-MOD-006; kết thúc vòng xử lý vụ việc.

**Independent Test**: `actOnModerationCase` (`remove_message`, `restrict`) → tin thành placeholder realtime, học sinh không gửi text được; `revokeSanction` → gửi lại được.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F121-AC-11 | KHI người có `moderation.case.act` gọi `actOnModerationCase` (`warn`/`mute`/`restrict`/`remove_message`/`escalate`/`dismiss`/`close`/`reopen`, `note` 3–1000 ký tự) với `If-Match`, `Idempotency-Key`, `X-Tenant-Id` = tenant của case, module chat PHẢI chuyển trạng thái theo [MODERATION §6.2](../../docs/03-social-multiplayer/MODERATION.md), ghi `moderation_actions` (append-only) + audit, trả `ModerationCaseDetail`; sai version → `412`, case đóng → `409 MODERATION_CASE_CLOSED` | FR-MOD-003 · FR-MOD-006 | TC-CHAT-007 |
| F121-AC-12 | KHI `remove_message`, module chat PHẢI đặt `moderation_state = removed` + soft-delete tin, giữ nội dung gốc làm bằng chứng, phát `chat.message_removed` (realtime `chat.message.removed`); API người dùng PHẢI chỉ trả placeholder | FR-MOD-006 · ADR-0034 · AGENTS R25 | TC-CHAT-007 |
| F121-AC-13 | KHI `createSanction` (`sanction_type` theo MODERATION §7, `duration_minutes` trong giới hạn schema, `reason` 3–1000, `case_id`), module chat PHẢI lưu sanction, cập nhật cache pipeline, phát `sanction.applied` (realtime `chat.sanction.applied` chỉ tới người bị áp, kèm `reason_code` và `ends_at`); `restrict_multiplayer` > 7 ngày và `account_lock` PHẢI yêu cầu trưởng ca (`moderation.case.lead`) hoặc System Admin, ngược lại `403 PERMISSION_DENIED` | FR-MOD-006 · [MODERATION §7](../../docs/03-social-multiplayer/MODERATION.md) · AC-MOD-009 | TC-CHAT-007 |
| F121-AC-14 | TRONG KHI sanction `mute`/`restrict_text`/`restrict_dm` còn hiệu lực, module chat PHẢI từ chối gửi tương ứng (`CHAT_MUTED`/`CHAT_KIND_NOT_ALLOWED`/`CHAT_NOT_ALLOWED`), kể cả khi công tắc kiểm duyệt tắt và ở chat gameplay always-on | FR-MOD-006 · AC-CHS-023 · AC-MOD-015 | TC-CHAT-014 |
| F121-AC-15 | KHI `revokeSanction` với `reason` hoặc sanction hết `ends_at`, module chat PHẢI ghi `revoked_at`/kết thúc, phát `sanction.lifted`, xóa cache trong ≤ 60 s | FR-MOD-006 · [MODERATION §7](../../docs/03-social-multiplayer/MODERATION.md) | — |
| F121-AC-16 | KHI case chuyển `resolved`, module chat PHẢI gửi thông báo trong game `moderation_notice` cho người báo cáo "Báo cáo của em đã được xử lý" không nêu chi tiết hình phạt | FR-MOD-002 · [SOCIAL_SYSTEM §8](../../docs/03-social-multiplayer/SOCIAL_SYSTEM.md) | — |

---

### User Story 4 — Thông báo vụ việc cho nhà trường (Priority: P1)

Case p1 liên quan học sinh TH Demo A: moderator gửi tóm tắt cho trường; `ad.truong.a` xác nhận "đã liên hệ phụ huynh"; quá 24 h chưa xác nhận → nhắc lại.

**Why this priority**: FR-MOD-017; nhà trường là đầu mối phụ huynh (OQ-13).

**Independent Test**: `notifySchoolForCase` → `listSchoolSafetyNotifications` của `ad.truong.a` có tóm tắt, không có nội dung chat; `ad.truong.b` không thấy; xác nhận lần 2 → 409.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F121-AC-17 | KHI người có `moderation.school_notification.send` gọi `notifySchoolForCase` (`school_id`, `student_ids` ≤ 20 đều thuộc trường đó và liên quan case, `summary` 10–1000) với `Idempotency-Key`, module chat PHẢI lưu thông báo, đặt `school_notified_at` (lần đầu) trên case, `ack_due_at` theo `school_notification_ack_sla_hours`, phát `moderation.school_notified`; học sinh trường khác trong `student_ids` → `422 VALIDATION_FAILED` | FR-MOD-017 · [MODERATION §6.4](../../docs/03-social-multiplayer/MODERATION.md) · AC-MOD-027 | TC-CHAT-043 |
| F121-AC-18 | KHI `school_admin` gọi `listSchoolSafetyNotifications`, module chat PHẢI chỉ trả thông báo của trường trong scope UserRole (Tenant Admin: mọi trường của tenant), gồm mã case, học sinh (tên, lớp, vai trò), loại vụ việc, mức, biện pháp, tóm tắt, hạn — không có trường nội dung tin nhắn | FR-MOD-017 · AC-MOD-006 · AGENTS R14 | TC-CHAT-043 |
| F121-AC-19 | KHI gọi `acknowledgeSchoolSafetyNotification` (`contact_outcome`, `note` ≤ 500) với `Idempotency-Key`, module chat PHẢI lưu `school_ack_at`, `acknowledged_by`, `contact_outcome` trên thông báo và case, phát `moderation.school_acknowledged`; xác nhận lần hai → `409 CONFLICT`; thông báo trường khác → `404` | FR-MOD-017 · [MODERATION §6.4](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-043 |
| F121-AC-20 | KHI quá `ack_due_at` chưa xác nhận, job `school-notification-reminder` PHẢI nhắc lại trường (thông báo portal) và báo người được gán; quá hạn lần hai PHẢI báo trưởng ca; `listSchoolSafetyNotificationsPlatform` PHẢI lọc được `overdue`, `acknowledged` | FR-MOD-017 · AC-MOD-027 | TC-CHAT-043 |
| F121-AC-21 | NẾU case thuộc tenant `common`, THÌ module chat PHẢI không tạo thông báo trường (`422 VALIDATION_FAILED`), đặt `contact_path = platform_direct` và giữ hạn chế tạm thời theo F121-AC-04 | [MODERATION §6.4](../../docs/03-social-multiplayer/MODERATION.md) · OQ-13 | — |

---

### User Story 5 — Trực 24/7 cho case p1 và leo thang SLA (Priority: P2)

Case p1 tạo lúc 23:00 → nếu 15 phút chưa ai nhận, on-call nhận cảnh báo; case quá hạn phản hồi tự leo thang.

**Why this priority**: FR-MOD-018; cần hàng đợi (US2) trước.

**Independent Test**: Đồng hồ giả: case p1 không gán sau `p1_unclaimed_alert_minutes` → alert `ModerationP1Unclaimed` bắn; case p2 quá hạn phản hồi → `escalated`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F121-AC-22 | HỆ THỐNG PHẢI xuất gauge `eduworld_moderation_cases_open{severity,assigned}` và alert khi có case p1 chưa gán quá `p1_unclaimed_alert_minutes` | FR-MOD-018 · [MODERATION §6.3](../../docs/03-social-multiplayer/MODERATION.md) · AC-MOD-028 | — |
| F121-AC-23 | KHI case quá hạn phản hồi đầu tiên theo `report_sla`, job `moderation-sla-escalator` PHẢI chuyển `escalated`, ghi action hệ thống và báo trưởng ca; không áp sanction dài hạn tự động | FR-MOD-018 · [MODERATION §6.3](../../docs/03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY §9](../../docs/10-security/CHILD_SAFETY.md) | — |
| F121-AC-24 | KHI case có category tự hại, `getModerationCase` PHẢI kèm hướng dẫn hỗ trợ (mã `support_guide = child_protection_111`) cho moderator; học sinh liên quan không bị phạt vì tín hiệu tự hại | FR-MOD-018 · [CHILD_SAFETY §8](../../docs/10-security/CHILD_SAFETY.md) | — |

---

### User Story 6 — Không giám sát vượt mục đích an toàn (Priority: P2)

**Why this priority**: FR-MOD-005, FR-MOD-008 — ràng buộc bất biến của ADR-0072.

**Independent Test**: Quét OpenAPI không có endpoint duyệt/tìm toàn bộ chat; audit không chứa nội dung tin.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F121-AC-25 | HỆ THỐNG PHẢI không có endpoint nào đọc/tìm tin nhắn ngoài ngữ cảnh case hoặc channel người gọi là thành viên; audit/log/event moderation PHẢI chỉ tham chiếu `message_id`/`case_id`, không sao chép nội dung | FR-MOD-005 · FR-MOD-008 · AC-MOD-007 · [MODERATION §9](../../docs/03-social-multiplayer/MODERATION.md) | TC-CHAT-009 |
| F121-AC-26 | KHI moderator mở case sau khi channel gameplay đã purge, module chat PHẢI chỉ trả `moderation_evidence` đã lưu; snapshot case PHẢI được giữ tới khi case đóng cộng thời hạn lưu case và không bị job purge/retention xóa | FR-MOD-008 · ADR-0033 · AC-MOD-024 | TC-CHAT-022 |

### Edge Cases

- NẾU cùng học sinh báo cáo cùng target nhiều lần trong 24 h, THÌ chỉ tăng `report_count` một lần cho người đó (trọng số), không phạt người báo cáo (F121-AC-02).
- NẾU nhiều report bị bác bỏ từ cùng người, THÌ giảm trọng số report sau, không sanction người báo cáo [Proposed].
- NẾU case liên trường, THÌ một case; mỗi trường nhận thông báo riêng chỉ chứa học sinh của trường đó (F121-AC-17).
- NẾU sanction trùng loại đang hiệu lực, THÌ tạo sanction mới với `ends_at` mới; sanction cũ giữ lịch sử.
- NẾU assignee rời ca, THÌ trưởng ca gán lại; case không mất bằng chứng.
- NẾU học sinh bị `account_lock`, THÌ module identity khóa qua event `sanction.applied` (F016); module chat không gọi đồng bộ.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-MOD-002 | Report message/user (API, snapshot, phản hồi); block/mute ở F095 | [MODERATION §5](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-003 | Hàng đợi tập trung Safety Team, case gắn tenant/trường, audit | [MODERATION §6](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-005 | Không ai đọc toàn bộ chat; truy cập đều audit | [MODERATION §6.1, §9](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-006 | Sanction: warning, mute, restrict, escalate | [MODERATION §7](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-008 | Tối thiểu hóa: chỉ dữ liệu cần cho an toàn | [MODERATION §10](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-016 | Đọc liên tenant chỉ trong module moderation qua Bypass | [MODERATION §6.1](../../docs/03-social-multiplayer/MODERATION.md) · ADR-0072 |
| FR-MOD-017 | Thông báo nhà trường, lưu thời điểm gửi/xác nhận | [MODERATION §6.4](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-018 | Trực 24/7 cho p1 (alert, leo thang) | [MODERATION §6.3](../../docs/03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY §9, §11](../../docs/10-security/CHILD_SAFETY.md) |

### Key Entities

- **Report**: báo cáo của học sinh (đối tượng, lý do, ghi chú, ngữ cảnh trận/party, snapshot).
- **ModerationCase**: vụ việc (tenant, trường liên quan, nguồn, mức ưu tiên, trạng thái, người được gán, hạn SLA, thời điểm thông báo/xác nhận trường, đường liên hệ).
- **ModerationAction**: lịch sử thao tác bất biến trên case.
- **Sanction**: biện pháp với học sinh (loại, phạm vi, thời hạn, người áp, gỡ).
- **ModerationEvidence**: bản sao bằng chứng bất biến.
- **SchoolSafetyNotification**: thông báo tóm tắt cho một trường, hạn xác nhận, kết quả liên hệ.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `report_sla` | `sla_due_at`, leo thang |
| `p1_unclaimed_alert_minutes` | Alert on-call p1 |
| `school_notification_ack_sla_hours` | `ack_due_at`, nhắc lại |
| `risk_signal_thresholds` | Ngưỡng tạo case tự động (F116 áp, F121 nhận case) |
| `auto_mute_spam` | Mute tự động (nguồn sanction hệ thống) |
| `gameplay_chat_retention_days` | Cửa sổ báo cáo tin gameplay |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Bypass chỉ trong handler moderation, có audit + metric | AGENTS R27 · ADR-0072 |
| Endpoint `/admin/moderation/*` chỉ gateway nội bộ, `azp = eduworld-portal` + MFA | AGENTS R20 · ADR-0027 |
| Thao tác ghi có `Idempotency-Key` + audit | AGENTS R10 |
| Không log nội dung chat/PII ở INFO; audit không chép nội dung | AGENTS R14 |
| Sanction thực thi ở pipeline kể cả chat always-on | AGENTS R15 · ADR-0020 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% report tạo case trong hàng đợi đúng tenant/trường trong ≤ 5 giây.
- **SC-002**: 100% lượt xem nội dung case có bản ghi audit với tenant đích; 0 truy cập chat ngoài case trong bộ test.
- **SC-003**: Case p1 chưa gán kích hoạt cảnh báo on-call đúng hạn trong 100% lần diễn tập.
- **SC-004**: 0 thông báo trường chứa nội dung tin nhắn hoặc học sinh trường khác.

## Ngoài phạm vi

- Pipeline đồng bộ, danh mục từ, công tắc, phân tích bất đồng bộ (tạo tín hiệu): F116. Block/mute cá nhân: F095. Báo cáo từ UI game: F119.
- Giao diện hàng đợi, thông báo trường, từ bổ sung tenant: F122. Từ bổ sung và bật/tắt nhóm từ của tenant (API): F116.
- Khóa tài khoản thực tế khi `account_lock` (consumer `sanction.applied`) và email báo thông báo mới cho School Admin: module identity (F016/F015) — báo cáo để bổ sung.
- Vòng trực tự động, break-glass của System Admin: chưa có feature (đề xuất bổ sung).

## Assumptions

- [Proposed] Ánh xạ reason → mức: `asked_personal_info` → p1; `bullying`, `inappropriate_content` → p2; `rude_language`, `spam`, `cheating`, `other` → p3 (MODERATION §6.2 chỉ quy định loại vụ việc).
- [Proposed] Permission `moderation.case.lead` cho trưởng ca; cần thêm vào catalog ADMIN_PORTAL §2.2 và `pkg/authz/catalog.go`.
- [Proposed] Thời hạn giữ snapshot = case đóng + 1 năm (MODERATION §10), chưa đăng ký CONFIG_DEFAULTS.
- Hold p1 dùng sanction hệ thống (`issued_by_kind = system`) để tái sử dụng cơ chế thực thi.

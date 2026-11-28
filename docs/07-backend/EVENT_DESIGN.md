# Event Design

> Trạng thái: Draft · Phiên bản: 0.6 · Owner: Backend Architect + EdTech Specialist · Cập nhật: 2026-11-28 (§3: dữ liệu do migration tạo phải ghi event vào outbox trong migration — F010, tenant `common`) · 2026-11-21 (ADR-0095 WP-I1: identity phát `student.placement_changed`, ngừng và xóa schema `class.created|updated|synced`, `enrollment.changed`; learning/competition bỏ nhánh nhận hai event đó; `sis.reconciliation_issue_raised.kind`, `user_role.changed.scope_type` bỏ giá trị lớp) · 2026-11-21 (ADR-0095 WP-L1: xóa `study_group.*`, `schedule_item.deleted`, `assignment.created.v1` cùng code F040/F041; producer F041 phát `assignment.created.v2`; `assignment.cancelled`, `exam.opened`, `exam.closed` bỏ `course_id` tùy chọn) · 2026-11-21 (ADR-0095 WP-S: event mới `student.placement_changed.v1` — owner F010; `assignment.created.v2` giao theo khối — owner F041; `enrollment.changed`, `class.*`, `study_group.*`, `schedule_item.deleted` đánh dấu Deprecated, xóa cùng code ở WP-I1/WP-L1) · 2026-11-20 (owner chốt OQ-17: `game_session.ended`/`student.game_logged_out` thêm `end_reason = daily_limit`; module identity tiêu thụ `game_session.ended` cho projection thời gian chơi/ngày) · 2026-11-17 (payload schema event M-C0/M-C1 — identity, game, `audit.recorded`; `user.provisioned` thêm `source = platform_common`, `platform_settings.changed` thêm scope `school`/`feature_flag` và `curfew_window`, `end_reason = curfew`) · 2026-11-17 (domain event `lesson.completed` — owner F031, consumer XP F074; nhắc nghỉ không phải domain event mà là telemetry client `break_reminder_shown`) · 2026-11-17 (Áp ADR-0074: relay và consumer chạy ở profile `worker-events`/`worker-chat`, event là tích hợp giữa module; subject `ew.evt.<module>.*` không đổi) · 2026-11-17 (Áp ADR-0089: retention outbox/processed events/dead letters bằng gỡ partition, khóa partition `event_occurred_at`) · 2026-11-16 (event `study_group.*` — ADR-0084; identity thu hồi phiên game giữ bằng refresh token — ADR-0086; event học tập thống nhất `activity.*`)
> Liên quan: [ADR-0042](../adr/ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) · [ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · [ADR-0019](../adr/ADR-0019-transactional-outbox-valkey-streams-cho-domain-events.md) · [VALKEY_DESIGN](VALKEY_DESIGN.md) · [DATABASE_DESIGN](DATABASE_DESIGN.md) · [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [JSON Schema learning event](../../specs/schemas/learning-event.schema.json) · [DATA_PRIVACY](../10-security/DATA_PRIVACY.md)

> [!IMPORTANT]
> **Bỏ khái niệm lớp — [ADR-0095](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) (owner chốt 2026-11-21).** Event bị bỏ: `class.created`, `class.updated`, `class.synced`, `enrollment.changed`, `study_group.*`, `schedule_item.deleted` (và type outbox `course.updated`, `learning_path.*`, `schedule_item.created|updated`). Event mới `student.placement_changed.v1` (identity, owner F010) thay `enrollment.changed`. Payload đổi không tương thích (`assignment.created` targets `grade|students`, `report.completed.report_type`, `party.created.visibility`, `chat_policy.updated.scope_type`…) dùng file `.v2` + `schema_version: 2`, consumer nhận cả hai trong đợt chuyển, producer chuyển sau; bỏ trường tùy chọn sửa tại chỗ v1. Dòng catalog bên dưới chỉ đổi cùng MR với schema + code (`check-cross-features`). Nội dung bên dưới còn nhắc lớp/enrollment/course/learning path/lịch/nhóm học tập sẽ được viết lại ở WP-S (event mới), WP-L1, WP-C1, WP-I1 ([CODE_PLAN §4.2](../12-project/CODE_PLAN_CMS_LMS_FIRST.md#42-kế-hoạch-bỏ-khái-niệm-lớp-adr-0095)); khi mâu thuẫn, ADR-0095 và đoạn này thắng.

> **Thay đổi v0.3:** domain event giữa các module đi qua transactional outbox trong database của từng module → NATS JetStream (stream `EVT_<MODULE>`, subject `ew.evt.<module>.<event_type>`), thay Valkey Streams (ADR-0042). Cột "Producer" trong catalog ghi tên domain con; domain con thuộc module nào theo [DOMAIN_MODEL §1](../06-architecture/DOMAIN_MODEL.md).
>
> **Thay đổi v0.6 (ADR-0074):** backend là modular monolith, nhưng mỗi module vẫn có database và transaction riêng nên domain event vẫn là cơ chế tích hợp giữa module (không gọi thẳng handler của module khác). Outbox relay và consumer domain event chạy ở profile `worker-events`, consumer của `CHAT_MESSAGES` ở `worker-chat`, job ở `worker-jobs` (binary `src/backend/cmd/worker`); `core-api` chỉ ghi outbox trong transaction. Stream, subject và tên consumer không đổi.

## 1. Hai loại event

| Loại | Mục đích | Lưu bền vững | Vận chuyển |
|---|---|---|---|
| **Domain event** | Tích hợp giữa module (cộng XP, cập nhật quest, projection `ref`, audit) | `infra.outbox_events` trong database của module phát (30 ngày) | Outbox relay (profile `worker-events`) → NATS JetStream `ew.evt.<module>.<event_type>` (stream `EVT_<MODULE>`) |
| **Learning event** | Bằng chứng học tập (LRS nội bộ, xAPI-like) cho mastery, analytics, báo cáo | `learning.learning_events` trong `eduworld_learning` (partition tháng, giữ theo retention của DATA_PRIVACY) | Ghi trực tiếp trong transaction của use case **và** đồng thời phát domain event `learning.event_recorded` (tùy chọn, cho consumer realtime) |

Learning event là **dữ liệu**; domain event là **tín hiệu**. Không suy ra lịch sử học tập từ stream. Learning event phát sinh ở module khác (quest, trận đấu) được module learning ghi khi tiêu thụ domain event tương ứng (`quest.completed`, `match.completed`).

## 2. Envelope chuẩn

Áp dụng cho cả hai loại (learning event có thêm trường bắt buộc §5.1).

```json
{
  "event_id": "0193a5f4-6c1e-7b2a-9d1f-3c4e5a6b7c8d",
  "event_type": "activity.submitted",
  "schema_version": 1,
  "occurred_at": "2026-11-15T07:30:12.345Z",
  "recorded_at": "2026-11-15T07:30:12.360Z",
  "producer": "learning/activity",
  "tenant_id": "0192f…",
  "actor": { "type": "student", "id": "0192e…" },
  "subject": { "type": "attempt", "id": "0193a…" },
  "session_id": "0193a…",
  "idempotency_key": "attempt:0193a…:submit",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "payload": { }
}
```

| Trường | Kiểu | Bắt buộc | Ghi chú |
|---|---|---|---|
| `event_id` | UUIDv7 | ✔ | Sinh tại producer; sắp xếp theo thời gian |
| `event_type` | string (≤ 64) | ✔ | `<entity>.<past_tense_verb>` |
| `schema_version` | int | ✔ | Tăng khi payload thay đổi không tương thích |
| `occurred_at` | timestamp UTC | ✔ | Thời điểm nghiệp vụ (server time) |
| `recorded_at` | timestamp UTC | ✔ | Thời điểm ghi |
| `producer` | string (≤ 64) | ✔ | `<module>/<thành phần>`, vd. `learning/exercise` (pattern trong [envelope.schema.json](../../specs/events/envelope.schema.json); đổi từ `<x>-service/…` theo ADR-0074 trước khi có code, envelope vẫn là v1) |
| `tenant_id` | UUID | ✔ (trừ event platform, khi đó `null`) | Tenant (ADR-0005); consumer phải dùng làm `TenantScope`. `school_id` (trường bên trong tenant) nếu cần nằm trong `payload` |
| `actor` | object | ✔ | `student`, `staff` (nhân sự trường hoặc platform, ADR-0081), `system`, `service`. Learning event chỉ dùng `student`, `system`, `staff` (§5.1) |
| `subject` | object | ✔ | Tài nguyên chính của event |
| `session_id` | UUID | — | `game_session_id` hoặc phiên CMS |
| `idempotency_key` | string (≤ 128) | ✔ | Khóa nghiệp vụ tất định — giống nhau khi sự việc lặp lại |
| `trace_id` | string (≤ 64) | — | W3C trace id |
| `payload` | object | ✔ | Theo catalog §4; **chỉ ID và giá trị cần thiết**, không PII |

## 3. Outbox → NATS JetStream (ADR-0042)

```mermaid
sequenceDiagram
  participant UC as Command handler module A (core-api hoặc worker)
  participant PGA as Database module A
  participant RL as Outbox relay module A (worker-events)
  participant JS as NATS JetStream (stream EVT_A)
  participant C as Consumer module B (worker-events)
  participant PGB as Database module B
  UC->>PGA: BEGIN, ghi nghiệp vụ, INSERT infra.outbox_events, COMMIT
  loop pg_notify hoặc mỗi 200 ms
    RL->>PGA: SELECT … WHERE published_at IS NULL ORDER BY id LIMIT 500 FOR UPDATE SKIP LOCKED
    RL->>JS: Publish ew.evt.<module>.<event_type> (Nats-Msg-Id = event_id)
    JS-->>RL: ack (đã lưu, khử trùng lặp 2')
    RL->>PGA: UPDATE published_at = now()
  end
  C->>JS: Fetch (durable consumer <module_b>.<handler>)
  C->>PGB: BEGIN, INSERT infra.processed_events(consumer, event_id, event_occurred_at) ON CONFLICT DO NOTHING
  alt mới
    C->>PGB: gọi command handler (UnitOfWork), COMMIT
  else đã xử lý
    C->>PGB: ROLLBACK (bỏ qua)
  end
  C->>JS: Ack
```

Bảng `infra.outbox_events` và `infra.processed_events` có trong **database của mỗi module**; relay của module A đọc outbox bằng role `<module>_relay`, consumer của module B ghi `processed_events` trong database của B, nên module A và B không bao giờ chung transaction dù chạy cùng tiến trình `worker-events`; DDL chuẩn ở [DATABASE_DESIGN §3.9](DATABASE_DESIGN.md#39-outbox--processed-events).

`pg_notify('ew_outbox')` sau commit để relay phản ứng nhanh; polling 200 ms làm dự phòng.

**Dữ liệu do migration tạo [Confirmed — owner "chốt hết" 2026-11-28].** Dòng mà module khác chiếu vào `ref.*` được tạo bằng use case có phát event. Dòng buộc phải seed bằng migration (tenant `common`, School ảo "EduWorld chung", danh mục tỉnh — F010) thì chính migration đó, hoặc một migration expand-only sau nó, PHẢI ghi event tương ứng vào `infra.outbox_events` trong cùng transaction: envelope §2 với `actor.type = system`, `producer = <module>/migration`; payload đọc từ dòng hiện tại như use case phát; `event_id` UUIDv7 theo thời điểm migration với bộ đếm theo thứ tự phụ thuộc (relay phát theo `event_id`: tenant trước trường); chặn trùng bằng `NOT EXISTS` cùng loại event + subject. Relay và consumer xử lý như mọi event (idempotent, `source_version`). Hiện thực: `20270203000001_f010_system_rows_events` của module identity ([F010 Clarifications 2026-11-28](../../specs/010-tenant-org-structure/spec.md#session-2026-11-28-dữ-liệu-do-migration-tạo-chưa-phát-event--confirmed--owner-chốt-hết)).

### 3.1. Stream, subject và consumer

| Stream | Subjects | Producer | Lưu | Ghi chú |
|---|---|---|---|---|
| `EVT_IDENTITY` | `ew.evt.identity.>` | module identity | 7 ngày, file, 3 replicas | Tenant, user, SIS, membership, cleanup, settings |
| `EVT_LEARNING` | `ew.evt.learning.>` | module learning | 7 ngày | Content, activity/attempt, exam, mastery, LMS |
| `EVT_GAME` | `ew.evt.game.>` | module game | 7 ngày | Game session, login, quest, XP, reward, Game Studio publish |
| `EVT_COMPETITION` | `ew.evt.competition.>` | module competition | 7 ngày | Party, match, rating, tournament |
| `EVT_CHAT` | `ew.evt.chat.>` | module chat | 7 ngày | Domain event chat/moderation (không chứa nội dung tin) |
| `EVT_REALTIME` | `ew.evt.realtime.>` | binary `realtime` (gateway, F090) | 7 ngày | Chỉ `audit.recorded` của gateway (vd. `realtime.rate_limit_closed` khi đóng 4429): gateway không có database/outbox nên publish trực tiếp (`Nats-Msg-Id` = `event_id`, at-most-once), tạo stream khi khởi động; consumer `identity.audit_sink` tiêu thụ như mọi `EVT_*` |
| `CHAT_MESSAGES` | `ew.chat.*.msg.*`, `ew.chat.*.evt.*`, `ew.chat.*.cursor.*` | module chat | 72 h | Đường ghi tin nhắn (ADR-0035, §4.10) |
| `DLQ_<MODULE>` (`DLQ_IDENTITY`, `DLQ_LEARNING`, `DLQ_GAME`, `DLQ_COMPETITION`, `DLQ_CHAT`) | `ew.dlq.<module>.<consumer>` | consumer của module nhận | 30 ngày | Event và message chat xử lý lỗi quá số lần (theo module nhận; module chat dùng chung `DLQ_CHAT` cho consumer của `CHAT_MESSAGES`) |
| `IDENTITY_SIS_JOBS` | `ew.job.identity.sis.>` | module identity (`core-api`: upload CSV, webhook SIS; `worker-jobs`: lịch) | Work-queue (retention `WorkQueue`) | Hàng đợi job SIS sync, thay `ew:sis:jobs` cũ |

Stream và durable consumer được tạo idempotent khi binary `worker` khởi động profile chạy consumer đó (`worker-events` cho `EVT_*`, `worker-chat` cho `CHAT_MESSAGES`; `CreateOrUpdateStream`/`CreateOrUpdateConsumer`), cấu hình trong code đăng ký ở `module.go` + env. Consumer đặt tên `<module_nhận>.<handler>` (vd. `game.quest_on_activity_submitted`) và dùng `FilterSubjects` chỉ lấy event cần.

### 3.2. Bản đồ consumer giữa module

Mọi consumer dưới đây chạy trong profile `worker-events` (consumer của chat trên stream `CHAT_MESSAGES` chạy ở `worker-chat`, §4.10); cột là module nhận.

| Event (producer) | identity | learning | game | competition | chat |
|---|---|---|---|---|---|
| `tenant.*`, `school.*`, `student.synced` (identity; không còn event lớp — ADR-0095) | — | projection `ref.*` | projection `ref.*` | projection `ref.*` | projection `ref.*` |
| `student.placement_changed` (identity, ADR-0095) | — | projection `ref.students` (`school_id`, `grade_level`, hiệu lực); thêm học sinh mới vào bài giao khối còn mở (F041) | projection `ref.students`; `grade_level` đổi kích hoạt quy tắc chuyển năm học ([PROGRESSION_SYSTEM §5.1](../01-game-design/PROGRESSION_SYSTEM.md#51-chuyển-năm-học-lên-lớp-confirmed)) | projection `ref.students` (quy tắc khối của party, ghép trận) | projection `ref.students` |
| `school.updated` (identity; gồm cài đặt trường `settings_subset`) | — | projection `ref.schools` | projection `ref.schools` | projection `ref.schools` (giờ truy cập, bảng xếp hạng bị tắt) | projection `ref.schools`, chính sách chat cấp trường |
| `tenant_data.assigned`, `tenant_data.unassigned` (identity, ADR-0029) | — | xóa cache pool câu hỏi; projection `ref.game_activities` khi `resource_type` là `quest`/`minigame_config` (catalog giao bài) | — (module game nhận gán qua facade `applyTenantAssignment` trước khi identity phát) | — | — |
| `user_role.changed`, `role.updated`, `tenant_membership.changed` (identity) | Keycloak membership + realm role `tenant_staff`/`platform_staff` | invalidate Principal cache | invalidate | invalidate | invalidate |
| `user.access_revoked`, `student.cleaned_up`, `student.restored` (identity) | thu hồi `game_auth_sessions` + phiên Keycloak (`access_revoked`/`cleanup`, ADR-0086) — xử lý nội bộ của `keycloak-provisioner` | `ref.students` | đóng game session, bỏ challenge đang chơi (`student_unavailable`, F076), `ref.students` | rời party, `ref.students` | rời channel, `ref.students` |
| `activity.submitted`, `activity.graded`, `exam.graded`, `attempt.re_evaluated`, `learning_objective.*` (learning) | — | mastery (nội bộ) | quest progress, XP/reward; tiến độ challenge solo từ `activity.graded` mode `challenge` (consumer `game.challenge_progress`, F076); `learning_objective.mastered` → XP nguồn `mastery.objective_mastered` (một lần mỗi học sinh × LO, [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)) | — | — |
| `lesson.completed` (learning, F031) | — | `progress_daily.lessons_completed` (nội bộ, consumer `learning.progress_daily`) | XP nguồn `lesson.completed` (consumer `game.reward_engine`, [PROGRESSION_SYSTEM §3.1](../01-game-design/PROGRESSION_SYSTEM.md#31-catalog-nguồn-xp)) | — | — |
| `assignment.published`, `assignment.results_released` (learning) | — | — | — | — | thông báo trong game (consumer `chat.notify_assignment`; người nhận qua facade learning `listAssignmentRecipients`, `assignment.published` chỉ khi `status = open`; "Thông báo trường" chỉ gồm bài giao và kết quả bài giao) |
| `party.invitation_sent` (competition), `level.reached`, `achievement.unlocked` (game), `friend_request.sent`, `report.resolved` (chat) | — | — | — | — | thông báo trong game (consumer `chat.notify_party_invitation`, `chat.notify_progression`, `chat.notify_friend_request`, `chat.notify_moderation` — F123) |
| `club.invitation_sent`, `club.join_requested`, `club.approved` (chat) | — | — | — | — | thông báo club trong game (consumer `chat.notify_club`, delta F120/F123 cho F111): `club_invitation` cho người được mời, `club_join_request` cho chủ club và officer là học sinh, `club_approved` cho chủ club học sinh khi nhân sự trường duyệt (club tạo `active` ngay không báo); deep link `open_club` |
| `sanction.applied`, `sanction.lifted`, `moderation.school_notified`, `moderation.school_notification_reminded`, `moderation.oncall_paged`, `break_glass.requested` (chat) | consumer `identity.sanction_enforcer`: `sanction.applied` loại `account_lock` → tạo `account_locks` nguồn `moderation_sanction`, `user.status_changed` + `user.access_revoked` (thu hồi `game_auth_sessions` + phiên Keycloak); `sanction.lifted` → giải phóng đúng khóa đó (active lại khi không còn khóa nào); consumer `identity.staff_mailer`: thông báo/nhắc trường → email `school_admin` (fallback `tenant_admin`), `moderation.oncall_paged` (event platform) → email người trực/trưởng ca/System Admin, `break_glass.requested` (event platform) → email System Admin duyệt; hàng đợi `identity.email_deliveries`, job `email-sender` (F124) | — | — | — | — |
| `chat_policy.updated`, `chat_policy.common_tenant_social_updated` (chat, delta F112-7) | consumer `identity.chat_direct_policy`: bản chiếu `direct_message_enabled` theo cấp (platform → `identity.chat_direct_policy_platform`; tenant/school/common → `identity.chat_direct_policies`, chỉ thay bằng version mới hơn) để điền `PlayerBootstrap.policies.chat.direct_message_enabled` không gọi module chat trên đường đăng nhập (R30) | — | — | — | vô hiệu cache effective policy (`chat_policy.updated`) |
| `student.game_logged_in/out` (game) | `users.last_login_at` | `ref.login_activity` (dashboard) | — | — | — |
| `activity.*`, `exam.graded`, `attempt.re_evaluated`, `assignment.*`, `learning_objective.*` (learning — nội bộ) | — | consumer `learning.rollup_dashboard`: upsert `progress_daily` và `rollup_class/school/tenant_daily` ([DASHBOARD_DATA §4](DASHBOARD_DATA.md#4-pipeline-gia-tăng)) | — | — | — |
| `game_profile.updated` (game) | — | — | — | `ref.player_profiles` | `ref.player_profiles` |
| `level.reached` (game) | — | — | — | `ref.player_profiles.level` (consumer `competition.projection_game`, chỉ tăng — `PartyMember.level`, delta F102) | `ref.player_profiles.level` + `title` (consumer `chat.projection_game`, level chỉ tăng — `PlayerCard.level`/`title`, delta F095) |
| `game_session.ended` (game) | projection thời gian chơi trong ngày ICT của học sinh (`duration_ms`) để từ chối đăng nhập game khi đạt `daily_play_limit_minutes` của trường ([CHILD_SAFETY §4.2](../10-security/CHILD_SAFETY.md#42-cưỡng-chế-giới-hạn-chơingày-oq-17), F013-AC-33) | — | — | — | — |
| `game_session.replaced` (game) | revoke Keycloak session (`sid`) + đánh dấu `game_auth_sessions` của thiết bị cũ thu hồi (`takeover`, ADR-0086) — refresh token lưu trên thiết bị cũ không dùng lại được | — | — | đánh dấu disconnect trong grace | — |
| `quest.completed` (game) | — | learning event (evidence) | — | — | club goal `quests_completed` (consumer `chat.club_goals_game`, F120) |
| `activity.graded` (learning), `world.discovery_found` (game) | — | — | — | — | club goal `activities_completed` / `discoveries_found` (consumer `chat.club_goals_learning` / `chat.club_goals_game`, mỗi event một lần, không lưu điểm — F120) |
| `match.started`, `match.completed`, `party.created/disbanded`, `party.member_*` (competition) | — | evidence `competition` (mastery trọng số thấp) | XP/reward thi đấu, co-op quest | — | mở/đóng channel `match`/`team`/`party`, `ref.parties` — channel tạo bằng consumer sau khi competition commit (outbox), không cùng transaction vì hai database khác nhau |
| `party.activity_started`, `match.started` (competition) | — | — | — | — | "Vừa chơi cùng" `social.recent_coplayers` của `listFriendSuggestions` (consumer `chat.recent_coplayers`, delta F095 cho F111: thành viên hoạt động party → `party`/`coop`, người trong trận → `match`, trận > 8 người chỉ cùng đội; không bao giờ cặp khác tenant) |
| `audit.recorded` (mọi module) | ghi `audit.audit_logs` | — | — | — | — |
| Sự kiện chơi: `game_session.*`, `world.*`, `story.*`, `avatar.created`, `quest.*`, `challenge.*`, `boss.*`, `xp.awarded`, `level.reached`, `currency.*`, `shop.item_purchased`, `achievement.unlocked`, `collection.completed` (game); `party.created`, `party.activity_started`, `match.completed`, `duel.*` (competition); `chat.daily_stats` (chat); `activity.submitted` (learning, chỉ đếm) | — | — | **analytics consumer** (durable `analytics`, ghi `analytics.events_raw` — [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md), ADR-0058) | — | — |
| `user.blocked`, `user.unblocked`, `friend.added`, `friend.removed` (chat) | — | — | — | projection `ref.blocks`, `ref.friendships` (consumer F091: chặn mời party, ghép trận, duel; ưu tiên bạn bè trong hub) | `user.blocked`/`user.unblocked`: consumer `chat.membership` đóng băng / mở lại channel `direct` của cặp (F120) |
| `content.published`, `content.rolled_back`, `content.unpublished` (game — cấu hình Game Studio: `quest`, `minigame_config`, `world_area`, `item`, `reward_rule`…) | — | projection `ref.game_activities` (`quest`/`minigame_config`, catalog giao bài); projection `ref.minigame_config_versions` (`content.published` `item_type = minigame_config`, khối `minigame_config` — snapshot bất biến cho plugin `minigame`, consumer `learning.minigame_config_projection`, F073) | snapshot/manifest (nội bộ) | projection `ref.world_areas` (`world_area`, hub) | — (sticker pack/GIF thuộc module chat, không cần event chéo) |

Consumer rollup dashboard (`learning.rollup_dashboard`, `game.analytics`) gom lô ≤ 1 s/500 event, upsert idempotent theo `processed_events`, chạy trên primary; job chốt ngày và xử lý event đến muộn ≤ 48 giờ theo [DASHBOARD_DATA §4–5](DASHBOARD_DATA.md#4-pipeline-gia-tăng) (ADR-0062).

Realtime push tới client **không** là consumer JetStream: module xử lý event (consumer trong `worker-events`) hoặc command (trong `core-api`) tự `PUBLISH` Valkey `ew:rt:user:<id>` / `ew:rt:room:<id>` / `ew:chat:fanout:<channel>`; binary `realtime` subscribe và giao xuống WS ([REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md)).

## 4. Catalog domain events

Producer/consumer ghi theo domain con (package trong `src/backend/modules/<module>/internal/domain`); domain con → module: identity/tenancy/organization/sis/audit → **module identity**; content/curriculum/activity/exam/learning_record/learning_analytics/lms → **module learning**; game_session/quest/minigame/gamification/reward → **module game**; party/multiplayer/competition/ranking/leaderboard/tournament/season → **module competition**; friend/chat/moderation/notification → **module chat**; RT = đẩy realtime qua Valkey Pub/Sub tới binary `realtime`. Bản đồ consumer giữa module: §3.2.

**Feature sở hữu schema (R33, ADR-0082).** Mỗi event có đúng một module phát (cột Producer) và, trong module đó, đúng một feature sở hữu: feature hiện thực use case phát event, ghi event trong mục "Event / outbox" của `specs/<NNN>-*/data-model.md` và tạo `specs/events/payloads/<module>/<event_type>.v<N>.schema.json`. Feature khác cùng phát (vd. thêm trường tương thích) ghi "owner F<NNN>" trên dòng event; feature tiêu thụ chỉ tham chiếu schema, không tạo. Event chung `audit.recorded` có một schema ở `payloads/common/` (owner F003). Cột Producer có thể ghi mã feature (vd. `quest (F103)`) và khi đó phải khớp feature sở hữu. Kiểm tra: `node scripts/specs/check-cross-features.mjs` (`--list` in bảng module:event → feature).

Ký hiệu consumer: GAM = gamification, QST = quest, LR = learning_record, LA = learning_analytics, LMS, RT = realtime push, AUD = audit, NTF = notification, RNK = ranking, LDB = leaderboard, MOD = moderation, ANA = game analytics consumer (module game, [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md)). Mọi event chơi liệt kê ở §3.2 dòng "Sự kiện chơi" đều có ANA là consumer dù cột Consumers bên dưới không ghi lại.

### 4.1. Identity, Tenancy, Organization & SIS

| Event | Producer | Consumers | Payload chính |
|---|---|---|---|
| `user.provisioned` | identity | AUD, GAM (tạo GameProfile) | `user_id`, `user_type`, `source` (jit/sis/manual/platform_common — học sinh tenant chung do System Admin tạo, F010-AC-24; self_registration — học sinh tự đăng ký trong game, F135, ADR-0104) |
| `user.activated` | identity | AUD, GAM | `user_id`, `external_identity_id` — lần đăng nhập đầu chuyển `pending → active` ([ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md)) |
| `user.access_revoked` | identity | RT (đóng WS, close code 4403), AUD | `user_id`, `reason` (locked/deactivated/transferred/password_changed — đặt lại mật khẩu chỉ thu hồi phiên, không tắt user, [ACCOUNT_PROVISIONING §6d](../09-identity/ACCOUNT_PROVISIONING.md#6d-đặt-lại-mật-khẩu-học-sinh)) |
| `user.access_restored` | identity | AUD, NTF | `user_id`, `reason` — mở khóa; Keycloak `enabled=true` |
| `user.status_changed` | identity | AUD, MOD | `user_id`, `from`, `to` |
| `user.email_changed` | identity | identity (keycloak_access: email Keycloak, `emailVerified = false`), AUD | `user_id`, `email_verified` (`false`) — không chứa địa chỉ email (OQ F012-1) |
| `user_role.changed` | identity | AUD, cache invalidation (Principal, menu portal), tenancy (đồng bộ membership Keycloak Organization và realm role `tenant_staff`/`platform_staff`, ADR-0066) | `user_role_id`, `user_id`, `role_id`, `role_code`, `tenant_id` (null nếu platform), `scope_type` (`platform`/`tenant`/`school`/`subject` — không có `class`, ADR-0095), `scope_id`, `source` (`manual`), `op` (granted/revoked/expired/updated), `actor_id` — thay `role_assignment.changed` |
| `role.updated` | identity | AUD, cache invalidation (Principal của mọi user có UserRole với role này) | `role_id`, `code`, `tenant_id` (null = dùng chung), `role_scope_kind`, `op` (created/updated/deleted), `permissions_added[]`, `permissions_removed[]`, `actor_id` (ADR-0066) |
| `tenant.created` / `tenant.updated` | tenancy | identity (cache alias, membership), Keycloak sync (tạo/cập nhật Organization), AUD | `tenant_id`, `tenant_type`, `status`, `changed_fields[]` |
| `tenant_data.assigned` / `tenant_data.unassigned` | tenancy (F045 — phát sau khi module sở hữu tài nguyên ghi gán qua facade, kể cả tài nguyên của module game; `tenant_data.assigned` + audit chỉ cho tenant được gán mới — cờ `created` của `applyTenantAssignments`; tenant đã có assignment vẫn nằm trong kết quả nhưng không phát lại, F045 Clarifications 2026-11-28) | content/lms (pool câu hỏi, catalog giao bài `ref.game_activities`), AUD | `resource_type`, `resource_id`, `tenant_id`, `assignment_id` (ADR-0029) |
| `tenant_data.cloned` | tenancy (worker clone) | AUD (ghi cả tenant nguồn và đích), NTF (System Admin) | `job_id`, `source_tenant_id`, `target_tenant_id`, `resource_type`, `count`, `status` |
| `tenant_membership.changed` | tenancy | identity (cache, Keycloak Organization member add/remove, thu hồi session) | `user_id`, `tenant_id`, `op` (added/removed), `source` |
| `<entity>.deleted` / `<entity>.restored` | module sở hữu entity | AUD, cache invalidation, consumer liên quan (vd. `class.deleted` → chat ẩn channel lớp) | `<entity>_id`, `tenant_id`, `deleted_by`/`restored_by`, `reason` — soft-delete/khôi phục (ADR-0028) |
| `student.provisioning_requested` | sis | identity (worker tạo user Keycloak qua admin API) | `student_id`, `tenant_id`, `school_id`, `sis_connection_id` |
| `student.synced` | sis | AUD, cache invalidation, game (quy tắc chuyển năm học khi đổi khối) | `student_id`, `school_id`, `action` (create/update/deactivate), `sync_run_id` |
| `school.synced` | sis | AUD, cache invalidation | `school_id`, `sync_run_id`, `changed_fields[]` |
| `student.placement_changed` | organization (F010; mọi nguồn đổi trường/khối/hiệu lực của học sinh: SIS F011/F017, tenant chung, saga F045, chuyển trường/lên khối, vòng đời F016 — ADR-0095 mục 2, 7) | identity (cache Principal), learning (`ref.students`, thêm học sinh mới vào bài giao khối còn mở), game (quy tắc chuyển năm học khi `grade_level` đổi), competition (`ref.students`), chat (`ref.students`), AUD | `student_id`, `tenant_id`, `school_id`, `grade_level`, `status` (`active`/`inactive`), `previous_tenant_id?`, `previous_school_id?`, `previous_grade_level?`, `previous_status?`, `changed_fields[]`, `academic_year_id?`, `effective_from?`, `source` (`sis`/`manual`/`platform_common`/`platform_saga`/`lifecycle`), `sync_run_id?`, `source_version` — schema [`student.placement_changed.v1`](../../specs/events/payloads/identity/student.placement_changed.v1.schema.json); producer: identity (WP-I1, 2026-11-21) |
| `student.transferred` | organization | chat, party (rời party), LMS, tenancy (đổi membership nếu khác tenant), AUD | `student_id`, `from_tenant_id`, `to_tenant_id`, `from_school_id`, `to_school_id` |
| `school.updated` | organization (F010) | mọi module (projection `ref.schools` gồm `settings_subset`), AUD | `school_id`, `tenant_id`, `status`, `name?`, `settings_subset` (chỉ khóa module nhận cần: giờ truy cập, `daily_play_limit_minutes` cho module game — F015-AC-30, `enabled_subject_codes` (môn trường bật, vắng = mọi môn bật) cho module game — F015 T054/F071-AC-03, bảng xếp hạng bị tắt, `allow_cross_school_listing`, chính sách chat cấp trường), `changed_fields[]`, `source_version` — đổi cài đặt trường (F015) phát cùng event và schema |
| `province.updated` | identity (danh mục platform, ADR-0060; envelope `tenant_id = null`) | learning, game (projection `ref.provinces`), identity (`ref.schools` tên tỉnh phi chuẩn hóa) | `province_id`, `code`, `country_code`, `official_code`, `name {vi,en}`, `status`, `effective_to`, `successor_code`, `op` (`created`/`updated`/`deactivated`/`succession_recorded`) |
| `student.cleanup_scheduled` | worker `student-inactive-cleanup` | NTF (Tenant Admin/School Admin: danh sách sắp bị dọn), AUD | `tenant_id`, `student_ids[]` (lô), `scheduled_cleanup_at`, `cutoff_days` (ADR-0032) |
| `student.cleaned_up` | worker `student-inactive-cleanup` | identity (gỡ Keycloak Organization, thu hồi session; **giữ** user Keycloak enabled để đăng nhập lại khôi phục — [ACCOUNT_PROVISIONING §6b](../09-identity/ACCOUNT_PROVISIONING.md#6b-dọn-học-sinh-không-đăng-nhập-game-adr-0032)), party (rời party), RT (đóng WS), LA, AUD | `student_id`, `tenant_id`, `last_login_at`, `cutoff_days`, `run_id` — soft-delete, không xóa dữ liệu |
| `student.restored` | identity (JIT khi học sinh đăng nhập lại) | identity (thêm lại membership), LA, AUD | `student_id`, `tenant_id`, `cleaned_up_at`, `restored_via` (`login`/`admin`) |
| `user.anonymized` | identity (F016, saga `user_anonymize`) | mọi module (ẩn danh hóa projection `ref.*`, learning events theo §9), AUD | `user_id`, `tenant_id`, `user_type`, `trigger` (`retention`; `data_request` chỉ còn ở saga ghi trước [ADR-0097](../adr/ADR-0097-bo-yeu-cau-du-lieu-ca-nhan-va-dich-noi-dung-hoc-tap-khoi-portal-release-1.md)), `data_request_id?`, `anonymized_at` — không chứa PII cũ |
| `platform_settings.changed` | tenancy | API/worker (invalidate cache), game/chat (giới nghiêm), AUD | `key` (`student_cleanup`/`chat_retention`/`platform_curfew`/`curfew_window`… hoặc khóa flag), `scope` (`platform`/`tenant`/`school`/`feature_flag`), `tenant_id?`, `school_id?`, `flag_key?`, `before`, `after`, `curfew_window?` (ADR-0078), `reason`, `actor_id` (F015) |
| `audit.recorded` | mọi module (outbox) | module identity (ghi `audit.audit_logs`) | `actor_id`, `actor_type`, `action`, `target_type`, `target_id`, `result`, `reason?`, `metadata` — không PII/nội dung chat (§3.2) |
| `sis.sync_completed` | sis | AUD, NTF (admin) | `sync_run_id`, `stats`, `issues_count` |
| `student.sis_link_created` | identity (provisioning/JIT/reconciliation) | AUD, LA | `user_id`, `tenant_id`, `school_id`, `sis_connection_id`, `student_external_id`, `via` (`provisioning`/`jit_login_key`/`jit_lookup`/`transfer`) (ADR-0046) |
| `student.sis_link_superseded` | identity (reconciliation chuyển trường giữa hai SIS) | AUD, LA | `user_id`, `old_sis_connection_id`, `old_student_external_id`, `new_sis_connection_id`, `new_student_external_id`, `cross_tenant` (ADR-0046) |
| `sis.reconciliation_issue_raised` | sis | NTF (school admin) | `issue_id`, `kind` (không còn `missing_class`, `class_school_mismatch`, `orphan_enrollment`, `multi_school_enrollment`, `grade_mismatch` — SIS contract 1.3.0, ADR-0095) |

**Mở rộng sau (ADR-0063):** `login_credential.issued` / `login_credential.revoked` (đăng nhập nhanh PIN/thẻ QR/mật khẩu hình ảnh, ADR-0045) không phát trong Release 1.0. Payload JSON Schema của event identity: [`specs/events/payloads/identity/`](../../specs/events/payloads/identity/) — F010, F011, F012, F013, F015 và `student.cleaned_up`, `student.restored` của F016 đã có (2026-11-17); `audit.recorded` ở [`specs/events/payloads/common/`](../../specs/events/payloads/common/) (owner F003).

### 4.2. Content & Curriculum

| Event | Producer | Consumers | Payload chính |
|---|---|---|---|
| `content.version_submitted` | learning / game (module sở hữu item; game: workflow Game Studio) | NTF (reviewer) | `content_type`, `content_id`, `version_id` |
| `content.version_approved` / `content.version_rejected` | như trên | NTF, AUD | như trên + `reviewer_id` |
| `content.published` | learning / game / chat (module sở hữu item) | snapshot worker cập nhật con trỏ cache `ew:<module>:current:*`, manifest builder (game), LMS (bài `follow_latest`), QB pool cache, AUD | `item_type`, `item_id`, `from_version_id`, `to_version_id`, `to_version`, `release_id`, `owner_scope`, `tenant_id?`, `scheduled`, `published_by` (ADR-0043). Với module learning: `item_type` theo enum `LearningContentType`, thêm `activity_type?`, `grade_levels[]`, `subject_id?`, `objective_ids[]` — schema [`content.published.v1`](../../specs/events/payloads/learning/content.published.v1.schema.json). Với module game (F071): `item_type` `world_area`/`story_chapter`/`npc_dialogue`/`level`, `item_id` là mã hoặc UUID, nội dung platform (`tenant_id` null), `world_area` mang phần runtime cho projection `ref.world_areas` của competition — schema [`game/content.published.v1`](../../specs/events/payloads/game/content.published.v1.schema.json); F078: thêm `config_id`, khối `catalog` (quest/mini-game: tiêu đề song ngữ, `grade_levels`, `subject_id`, `objective_ids`, `estimated_minutes`, `thumbnail_url`) và `content_refs[]` (activity/question set/lesson được tham chiếu, OQ F054-3); `scheduled = true` khi job `scheduled-publish` áp dụng lịch hẹn |
| `content.rolled_back` | như trên | như trên; game (F078): projection `ref.minigame_config_pointers` của learning (consumer `learning.minigame_config_projection`) | như `content.published` + `reason` (bắt buộc); game: kèm khối `world_area`/`minigame_config`/`catalog` của version đích — schema [`game/content.rolled_back.v1`](../../specs/events/payloads/game/content.rolled_back.v1.schema.json); chat (sticker pack/GIF, owner F082): `to_version_id`, `from_version?`, `reason` — schema [`chat/content.rolled_back.v1`](../../specs/events/payloads/chat/content.rolled_back.v1.schema.json) |
| `content.unpublished` | như trên | như trên (xóa con trỏ cache, gỡ khỏi manifest) | `item_type`, `item_id`, `from_version_id`, `release_id`, `reason`; chat (sticker pack/GIF, owner F082): `from_version`, `reason` — schema [`chat/content.unpublished.v1`](../../specs/events/payloads/chat/content.unpublished.v1.schema.json) |
| `content.publish_scheduled` | như trên | worker hẹn giờ, AUD | `item_type`, `item_id`, `version_id`, `publish_at` |
| `content.manifest_updated` | game (manifest builder) | realtime → WS `content.manifest.updated` tới học sinh online của tenant (chỉ báo, áp dụng ở màn sau) | `tenant_id`, `manifest_version`, `etag`, `changed_items[]` (≤ 200) |
| `content.version_archived` | learning / game (module sở hữu item) | cache, LMS | như `content.published` (không đổi con trỏ) |
| `media.uploaded` | learning / game / chat | media worker | `asset_id`, `tenant_id?`, `kind`, `quarantine_key`, `size_bytes` (ADR-0044) |
| `media.processed` | media worker | module sở hữu (cập nhật `media_assets.status=ready`, `variants`), NTF (người upload) | `asset_id`, `sha256`, `variants[]` |
| `media.rejected` | media worker | module sở hữu, NTF | `asset_id`, `reason` (`virus`/`type_mismatch`/`too_large`/`transcode_failed`) |
| `curriculum.version_published` | curriculum | content, LA | `curriculum_version_id` |

### 4.3. Assessment (exercise / exam)

| Event | Producer | Consumers | Payload chính |
|---|---|---|---|
| `activity.started` | activity engine | LA, LMS | `attempt_id`, `student_id`, `activity_type?`, `mode`, `context_type?`, `context_id?`, `content_version_id`, `recommendation_id?` (khi attempt bắt đầu từ gợi ý tự động, LEARNING_ANALYTICS §6) |
| `activity.abandoned` | activity engine | LA, LMS | `attempt_id`, `student_id`, `mode`, `reason` (`expired`/`user`/`superseded`) ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)) |
| `exam.opened` / `exam.closed` | exam | NTF, RT, LMS, AUD | `exam_session_id`, `school_id`, `window_start`, `window_end`, `status` — cửa sổ thi chuyển `open`/`closed` ([EXAM_ENGINE](../02-learning/EXAM_ENGINE.md)) — schema [`exam.opened.v1`](../../specs/events/payloads/learning/exam.opened.v1.schema.json); — schema [`exam.closed.v1`](../../specs/events/payloads/learning/exam.closed.v1.schema.json) |
| `exam.started` | exam | LA, AUD | `attempt_id`, `exam_session_id`, `deadline_at` — schema [`exam.started.v1`](../../specs/events/payloads/learning/exam.started.v1.schema.json) |
| `exam.submitted` | exam | LMS, AUD, worker `learning.exam_grade` | `attempt_id`, `exam_session_id`, `submit_reason` (`manual`/`auto_timeout`/`window_closed`) — schema [`exam.submitted.v1`](../../specs/events/payloads/learning/exam.submitted.v1.schema.json) |
| `exam.graded` | exam | **LR** (evidence mastery lấy từ đây, không từ `exam.submitted`), LMS, NTF, GAM | `attempt_id`, `evaluation_id`, `exam_session_id`, `score`, `max_score`, `is_official`, `result_release` — schema [`exam.graded.v1`](../../specs/events/payloads/learning/exam.graded.v1.schema.json) |
| `exam.results_released` | exam | NTF, RT, LMS | `exam_session_id`, `released_by?` (`manual`), `released_at` ([EXAM_ENGINE §8](../02-learning/EXAM_ENGINE.md#8-chấm--công-bố-kết-quả)) — schema [`exam.results_released.v1`](../../specs/events/payloads/learning/exam.results_released.v1.schema.json) |
| `exam.regraded` | exam | LR (event bù trừ evidence, không xóa event cũ), LMS, NTF | `exam_session_id`, `regrade_request_id`, `question_version_id`, `action`, `affected_attempt_ids[]` — `RegradeRequest` là lệnh **re-evaluate do hệ thống chạy** (không chấm tay), do `content_admin`/`academic_manager` tạo qua `POST /cms/learning/regrade-requests` (`createRegradeRequest`, permission `learning_content.re_evaluate`); mỗi attempt bị ảnh hưởng phát `attempt.re_evaluated` với `trigger.re_evaluation_id` = `regrade_request_id` ([EXAM_ENGINE §9](../02-learning/EXAM_ENGINE.md#9-chấm-lại-regrade)) — schema [`exam.regraded.v1`](../../specs/events/payloads/learning/exam.regraded.v1.schema.json) |
| `activity.submitted` | activity engine (ADR-0048) | GAM, QST, LR, LMS, RT | `attempt_id`, `student_id`, `activity_id?`, `activity_type` (`ActivityType`), `content_version_id`, `mode` (`AttemptMode`), `context_type?`, `context_id?`, `assignment_id?`, `status` (`graded` khi chấm xong lúc nộp / `submitted` khi chấm hoặc công bố trì hoãn), `score?`, `max_score?`, `correct_count?`, `item_count`, `hints_used`, `rapid_guess_count`, `duration_ms`, `submit_reason`, `objective_ids[]` — schema [`activity.submitted.v1`](../../specs/events/payloads/learning/activity.submitted.v1.schema.json) ([ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)) |
| `activity.graded` | activity engine | LR, LMS, NTF, GAM | `attempt_id`, `student_id`, `evaluation_id`, `activity_type`, `mode`, `source` (luôn `system` — mọi hoạt động trong Release do server tự chấm, ADR-0072), `strategies[]`, `score`, `max_score`, `passed?`, `result_release`, `results_visible_at?`, `objective_ids[]`, `content_version_id?`, `first_try_correct_count?`, `recommendation_id?`, `content_grade_levels?` (delta F074: Reward Engine đếm giảm dần theo content version, XP theo câu đúng lần đầu, miễn giảm ôn LO tiên quyết), `minigame?` (delta F073: `minigame_config_id`, `minigame_config_version_id`, `minigame_type`, `game_score`, `completion_met`, `context_type`, `context_id` — chỉ attempt qua plugin `minigame`; mastery không dùng `game_score`; Reward Engine bỏ `activity.graded` mode `minigame`, thưởng qua `minigame.completed`) — schema [`activity.graded.v1`](../../specs/events/payloads/learning/activity.graded.v1.schema.json) |
| `attempt.re_evaluated` | activity engine (F031 — re-evaluate; chấm lại đề ở exam phát cùng schema) | LR (bút toán điều chỉnh evidence), LMS (cập nhật `AssignmentSubmission`), NTF | `attempt_id`, `previous_evaluation_id`, `new_evaluation_id`, `previous_score`, `new_score`, `question_version_ids[]`, `reason`, `trigger {role_code: content_admin \| academic_manager, re_evaluation_id}` — hệ thống chấm lại khi nội dung có lỗi, có audit; envelope `actor.type = staff` (người kích hoạt) hoặc `system` (ADR-0072 mục 3) |
| `simulation.run_verified` / `simulation.run_rejected` | activity engine | LR, AUD (rejected → integrity) | `run_id`, `attempt_id`, `template_code`, `goals_met` |
| `assignment.created` | lms | AUD (consumer LMS read model của F040 đã gỡ — ADR-0095) | **v2 (ADR-0095, producer F041 phát từ WP-L1, envelope `schema_version: 2`)**: `assignment_id`, `school_id`, `subject_id` (bắt buộc, thay `course_id`), `created_by`, `created_by_role` (`school_admin`/`academic_manager`/`system_admin`), `content_type`, `content_id`, `pinned_version_id?`, `follow_latest`, `mode`, `status`, `targets[] {target_type: grade | students, grade_level?, student_count, included_student_count?, excluded_student_count?}`, `open_at`, `due_at`, `max_attempts`, `feedback_timing`, `result_release`, `objective_ids[]` — schema [`assignment.created.v2`](../../specs/events/payloads/learning/assignment.created.v2.schema.json); v1 (target `class`/`study_group`, `course_id`) đã xóa vì không còn producer và consumer trong repo |
| `assignment.published` | lms | NTF, RT (tab "Nhà trường giao" trong game) | Như `assignment.created` với `status` = `scheduled`/`open` ([SCHOOL_LMS §4.1](../04-admin-portal/SCHOOL_LMS.md#41-tạo-và-giao-bài)) |
| `assignment.results_released` | lms | NTF, RT | `assignment_id`, `school_id`, `released_at`, `released_by?` (khi `result_release = manual`), `trigger` (`after_close`/`manual`) — schema [`assignment.results_released.v1`](../../specs/events/payloads/learning/assignment.results_released.v1.schema.json) |
| `assignment.cancelled` | lms (F041, `cancelAssignment`) | LMS lịch (F040 soft-delete mục `assignment_due`), NTF | `assignment_id`, `school_id`, `course_id?`, `exam_session_id?`, `cancelled_at`, `cancelled_by`, `reason` — schema [`assignment.cancelled.v1`](../../specs/events/payloads/learning/assignment.cancelled.v1.schema.json) |
| `assignment.completed` | lms | GAM, LMS read model | `assignment_id`, `school_id`, `student_id`, `attempt_id`, `late` — schema [`assignment.completed.v1`](../../specs/events/payloads/learning/assignment.completed.v1.schema.json) |
| `assignment.submission_updated` | lms | LMS read model, dashboard rollup | `assignment_id`, `student_id`, `status` (`not_started`/`in_progress`/`submitted`/`late`/`missed`/`excused`), `attempts_used`, `best_attempt_id?`, `score?`, `max_score?`, `rapid_guess_count` — schema [`assignment.submission_updated.v1`](../../specs/events/payloads/learning/assignment.submission_updated.v1.schema.json) |
| `assignment.closed` | lms (worker đóng bài khi qua `due_at` + cửa sổ nộp muộn) | NTF, LMS read model, AUD | `assignment_id`, `school_id`, `closed_at`, `missed_count`, `status` (trạng thái sau khi đóng) — schema [`assignment.closed.v1`](../../specs/events/payloads/learning/assignment.closed.v1.schema.json) |
| `assignment.reminded` | lms (F041, `RemindAssignment` — số lần nhắc có giới hạn) | NTF (thông báo trong game tới học sinh chưa nộp), AUD | `assignment_id`, `school_id`, `student_ids[]` (≤ 2000), `reminder_no`, `due_at`, `reminded_by` — schema [`assignment.reminded.v1`](../../specs/events/payloads/learning/assignment.reminded.v1.schema.json) |
| `report.completed` | learning_analytics (F044, job `report-jobs`) | NTF portal (người yêu cầu), AUD | `report_id`, `scope_level`, `report_type`, `format`, `status` (`succeeded`/`failed`, cùng enum `ReportJob.status`), `requested_by`, `row_count?` — không chứa link tải hay nội dung báo cáo [Confirmed] — schema [`report.completed.v1`](../../specs/events/payloads/learning/report.completed.v1.schema.json) |

ExamSession có `efficacy_wave_id` (F035, [LEARNING_EFFICACY_PLAN §3.2](../02-learning/LEARNING_EFFICACY_PLAN.md)) **không phát** `exam.*`, `activity.*` ra outbox (chặn tại nguồn, ADR-0080 mục 6): bài được chấm ngay trong transaction nộp bài, chỉ ghi learning event (có `context.efficacy_wave_id`); vì vậy không evidence mastery, không XP/quest, không thông báo, không vào dashboard.

Chấm tay, trả bài và bài nói ngoài version này (ADR-0082): không có event. Payload JSON Schema của event learning: [`specs/events/payloads/learning/`](../../specs/events/payloads/learning/).

### 4.4. Learning record

| Event | Producer | Consumers | Payload chính |
|---|---|---|---|
| `learning_objective.progressed` | learning_record | GAM (achievement), RT, LA | `objective_id`, `from`, `to` (value), `level` |
| `learning_objective.mastered` | learning_record | GAM (XP `mastery.objective_mastered`), NTF, LMS | `objective_id`, `value`, `evidence_count` |
| `lesson.completed` | learning_record (F031) | GAM (XP nguồn `lesson.completed`, [PROGRESSION_SYSTEM §3.1](../01-game-design/PROGRESSION_SYSTEM.md#31-catalog-nguồn-xp) — F074), LA (`progress_daily.lessons_completed`, nội bộ) | `lesson_session_id`, `lesson_id`, `lesson_version_id` (ghim khi bắt đầu, R31), `lesson_version`, `student_id`, `subject_id?`, `grade_levels[]`, `completion_rule` (`all_required_activities`/`any_activity`), `mode` (`practice`/`assignment`), `assignment_id?`, `recommendation_id?`, `required_activity_count`, `completed_activity_count`, `attempt_ids[]`, `objective_ids[]`, `started_at`, `completed_at`, `duration_ms` — phát cùng learning event `lesson.completed` (§5.2) khi học sinh hoàn thành mọi activity bắt buộc của bài; đúng một lần mỗi `(student_id, lesson_version_id)` (`idempotency_key` = `lesson:<student_id>:<lesson_version_id>:completed`) — schema [`lesson.completed.v1`](../../specs/events/payloads/learning/lesson.completed.v1.schema.json) |
| `learning.recommendation_generated` | learning_analytics | RT, LMS | `student_id`, `objective_ids[]`, `content_refs[]` |

### 4.5. Game

| Event | Producer | Consumers | Payload chính |
|---|---|---|---|
| `game_session.started` / `game_session.ended` | game_session | LA (thời gian học), AUD; `game_session.ended` thêm module identity (projection thời gian chơi/ngày, F013-AC-33) | `game_session_id`, `platform`, `client_version`, `duration_ms`, `end_reason` (`logout`/`replaced_by_other_device`/`timeout`/`account_locked`/`access_revoked`/`cleanup`/`client_crash`/`curfew`/`daily_limit` — `curfew`: ADR-0078; `daily_limit`: job đóng phiên khi học sinh đạt giới hạn chơi/ngày của trường, [CHILD_SAFETY §4.2](../10-security/CHILD_SAFETY.md#42-cưỡng-chế-giới-hạn-chơingày-oq-17), F014-AC-27; ánh xạ từ `user.access_revoked.reason`: `account_locked` → `account_locked`, `cleanup` → `cleanup`, `access_revoked`/`admin`/`password_changed` → `access_revoked`) |
| `student.game_logged_in` | game_session | LA (hoạt động, lần đăng nhập cuối), AUD | `student_id`, `tenant_id`, `game_session_id`, `login_event_id`, `platform`, `login_at` — không có IP/device_id thô (ADR-0031) |
| `student.game_logged_out` | game_session | LA, AUD | `student_id`, `game_session_id`, `logout_at`, `end_reason` |
| `game_session.replaced` | game_session | RT (gửi `session.session.replaced` tới thiết bị cũ, close 4409), identity (revoke Keycloak session theo `sid`), multiplayer (đánh dấu disconnect, giữ chỗ trong grace), AUD | `old_game_session_id`, `new_game_session_id`, `student_id`, `new_platform`, `old_keycloak_sid`, `old_device_id` (identity thu hồi phiên Keycloak + phiên game đã lưu của thiết bị cũ — ADR-0086) |
| `quest.accepted` | quest | LA | `quest_progress_id`, `quest_id`, `quest_version_id`, `quest_type`, `start_trigger`, `period_key?`, `party_id?` — [schema](../../specs/events/payloads/game/quest.accepted.v1.schema.json) |
| `quest.step_completed` | quest | RT | `quest_progress_id`, `quest_id`, `quest_version_id`, `step_id`, `step_type`, `attempt_id?` — [schema](../../specs/events/payloads/game/quest.step_completed.v1.schema.json) |
| `quest.completed` | quest | GAM, LR, RT, NTF, ANA | `quest_progress_id`, `quest_id`, `quest_version_id`, `quest_type`, `objective_ids[]`, `completion_no`, `rewards[]` (ý định), `party_id?`, `story_chapter_code?`, `area_id?` — [schema](../../specs/events/payloads/game/quest.completed.v1.schema.json) |
| `quest.abandoned` | quest (`:abandon` hoặc hết hạn/thất bại theo QUEST_SYSTEM) | ANA | `quest_progress_id`, `quest_id`, `quest_version_id`, `quest_type`, `reason` (`player`/`expired`/`failed`), `step_id?`, `failure_kind?` — [schema](../../specs/events/payloads/game/quest.abandoned.v1.schema.json) |
| `quest.party_completed` | quest (F103 — phiên quest chung của party) | competition (kết thúc party activity, F101), GAM, ANA | `party_quest_session_id`, `party_activity_id`, `party_id`, `quest_version_id`, `outcome` (`completed`/`failed`/`abandoned`), `member_ids[]` — kết quả từng học sinh vẫn phát `quest.completed` riêng |
| `world.area_entered` | world | ANA | `area_id`, `from_area_id`, `instance_kind` — gộp tối đa 1 event/area/phút/người chơi |
| `minigame.completed` | minigame (F073) | GAM (Reward Engine `game.reward_engine_game`, áp `reward_rule_id` của cấu hình khi `rewards_eligible`), ANA | `play_id`, `student_id`, `minigame_config_id`, `minigame_config_version_id`, `minigame_type`, `attempt_id`, `context_type` (`free_play`/`quest_step`/`assignment`), `context_id?`, `learning_score`, `max_score`, `game_score`, `correct_count?`, `item_count?`, `completion_met`, `rewards_eligible` (= `completion_met` ∧ lượt trong ngày ICT ≤ `limits.max_plays_per_day`), `plays_today`, `reward_rule_id?`, `grade_levels[]`, `subject_id?`, `completed_at` — module game (consumer `game.minigame_on_activity_graded`) phát đúng một lần cho mỗi lượt khi nhận `activity.graded` có khối `minigame`; bước quest `minigame` hoàn thành từ `activity.graded` (`completion_met`), không từ event này; module learning **không** tiêu thụ (evidence đã có từ attempt) — schema [`minigame.completed.v1`](../../specs/events/payloads/game/minigame.completed.v1.schema.json) ([MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md)) |
| `item.granted` | game (F075) | RT, AUD, chat F118 (`ref.sticker_unlocks`) | `entry_id`, `student_id`, `item_id` (mã), `item_kind`, `quantity`, `quantity_total`, `reason_code`, `source_type`, `source_id`, `grant_id?`, `rule_version_id?`, `granted_at` — schema [`item.granted.v1`](../../specs/events/payloads/game/item.granted.v1.schema.json) |
| `event.scheduled` / `event.started` / `event.suspended` / `event.resumed` / `event.ended` / `event.cancelled` | event (live-ops, [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md)) | content manifest, RT (Event Center), NTF (trong game), QST, ANA, AUD | `event_id`, `event_run_id`, `event_version_id`, `event_kind`, `owner_scope`, `status`, `audience` (scope, tenant_ids, school_ids, grade_levels — không lớp), `starts_at`, `ends_at`, `claim_until`, `reason?` (lý do thao tác vận hành; null khi scheduler), `changed_at` — sự kiện platform: envelope `tenant_id` null (F077) — schema `event.<action>.v1` (vd. [schema](../../specs/events/payloads/game/event.started.v1.schema.json)) |
| `event.participation_started` | event | ANA, achievement-evaluator (trigger), world unlock facts | `event_id`, `event_run_id`, `event_kind`, `student_id`, `school_id`, `joined_at` — [schema](../../specs/events/payloads/game/event.participation_started.v1.schema.json) |
| `event.milestone_claimed` | event | achievement-evaluator (trigger), ANA | `event_id`, `event_run_id`, `milestone_id`, `student_id`, `auto_granted`, `claimed_at` — phần thưởng đã cấp trong cùng transaction qua Reward Engine (`reward.granted`) — [schema](../../specs/events/payloads/game/event.milestone_claimed.v1.schema.json) |
| `community_goal.milestone_reached` / `community_goal.completed` | event | RT, world (đổi trang trí), ANA — thưởng người có đóng góp cấp trong cùng transaction (`reward.granted` `trigger_kind = community_goal`) | `goal_id`, `event_run_id`, `event_id`, `goal_code`, `scope`, `scope_id`, `milestone_pct` (reached), `total`, `target`, `world_change_code?` — không danh tính học sinh; không là trigger thành tựu — [schema](../../specs/events/payloads/game/community_goal.milestone_reached.v1.schema.json), [schema](../../specs/events/payloads/game/community_goal.completed.v1.schema.json) |
| `season_track.tier_reached` | season_track | achievement-evaluator (trigger), RT, ANA | `season_id`, `season_track_id`, `student_id`, `tier`, `points`, `reached_at` — [schema](../../specs/events/payloads/game/season_track.tier_reached.v1.schema.json) |
| `season_track.reward_claimed` | season_track | ANA, AUD | `season_id`, `season_track_id`, `student_id`, `tier`, `auto_granted` (bool), `claimed_at` — [schema](../../specs/events/payloads/game/season_track.reward_claimed.v1.schema.json) |

**Nhắc nghỉ không phải domain event.** Lời nhắc nghỉ do game client hiển thị (F062-AC-21, ngưỡng `break_reminder` của [CHILD_SAFETY](../10-security/CHILD_SAFETY.md)); server không biết thời điểm nhắc nên không có producer outbox. Client ghi nhận bằng telemetry `break_reminder_shown` qua `POST /game/telemetry` ([GAME_ANALYTICS §3.2](../11-operations/GAME_ANALYTICS.md#32-nguồn-client-telemetry-post-apiv1gametelemetry), F062 gửi, F080 tổng hợp chỉ số chơi lành mạnh); không thêm vào catalog này.

### 4.6. Gamification

| Event | Producer | Consumers | Payload chính |
|---|---|---|---|
| `xp.awarded` | game (F074, Reward Engine) | RT, LDB (`competition.leaderboard_xp` → bảng `friends_xp_weekly`, bỏ bút toán bù/0 XP), ANA | `xp_entry_id`, `student_id`, `amount`, `reason_code`, `source_type`, `source_code`, `source_id`, `content_version_id?`, `rule_id?`, `rule_version?`, `capped` (bool), `xp_total`, `level`, `awarded_at` — mỗi bút toán `xp_ledger` (kể cả 0 XP bị cap), cùng transaction; schema [`xp.awarded.v1`](../../specs/events/payloads/game/xp.awarded.v1.schema.json) |
| `level.reached` | game (F074) | `game.unlock_on_events` (mở vùng theo level), RT, NTF, achievement (F075), ANA, competition (`competition.projection_game`: `ref.player_profiles.level`, delta F102), chat (`chat.projection_game`: `ref.player_profiles.level` + `title` của PlayerCard, delta F095) | `student_id`, `from_level`, `to_level`, `curve_id`, `curve_version`, `title_code?`, `title?` (LocalizedName của danh hiệu level, delta F095), `trigger_xp_entry_id?` (NULL khi job `curve-rollout`), `reached_at` — một event mỗi cấp, không phát tại `max_level` ([PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)); schema [`level.reached.v1`](../../specs/events/payloads/game/level.reached.v1.schema.json) |
| `achievement.progressed` | game (F075, achievement-evaluator) | RT | `achievement_id`, `code`, `version`, `student_id`, `progress`, `target?`, `tier?`, `hidden`, `progressed_at` — schema [`achievement.progressed.v1`](../../specs/events/payloads/game/achievement.progressed.v1.schema.json) |
| `achievement.unlocked` | game (F075, achievement-evaluator) | RT, NTF (F123), `game.unlock_on_events` (F071) | `achievement_id`, `code`, `student_id`, `tier`, `category`, `hidden`, `grant_id?`, `unlocked_at` — một lần mỗi (student, achievement, tier); schema [`achievement.unlocked.v1`](../../specs/events/payloads/game/achievement.unlocked.v1.schema.json) |
| `avatar.created` | game | achievement, LA (funnel) | `student_id`, `preset_code?` |
| `avatar.updated` | game (F070) | ANA, RT (realtime cập nhật ngoại hình trong phòng) | `student_id`, `avatar_config_hash` |
| `game_profile.updated` | game (F070) | competition, chat (projection `ref.player_profiles`) | `student_id`, `tenant_id`, `nickname`, `avatar_config_hash`, `avatar_config` (JSON ngoại hình đã chuẩn hóa, ≤ 4 KB — competition hiển thị trong hub, F091), `avatar_thumb_key?`, `level`, `source_version` |
| `world.area_unlocked` | game | RT, NTF, QST, competition (projection `ref.area_unlocks`) | `student_id`, `area_id`, `rule_version`, `unlocked_by`, `unlocked_at` — một lần/(học sinh, area); schema [`world.area_unlocked.v1`](../../specs/events/payloads/game/world.area_unlocked.v1.schema.json) (F071) |
| `world.discovery_found` | game | QST, achievement, reward | `student_id`, `area_id`, `interactable_id`, `kind` (discovery/collectible), `collection_id?`, `level_session_id?`, `completion_pct` — schema [`world.discovery_found.v1`](../../specs/events/payloads/game/world.discovery_found.v1.schema.json) (F071) |
| `world.area_completed` | game | achievement, reward | `student_id`, `area_id`, `completion_pct`, `completed_at` — lần đầu đạt 100 % (F071) |
| `story.chapter_completed` | game | achievement, reward, RT, LA | `student_id`, `chapter_code`, `chapter_version_id`, `region_code` ([WORLD_DESIGN §2.2](../01-game-design/WORLD_DESIGN.md)) — phát một lần/chương/học sinh |
| `challenge.started` | game | LA, ANA | `challenge_attempt_id`, `challenge_id`, `challenge_version_id`, `challenge_type`, `student_id`, `difficulty`, `period_key`, `deadline_at?`, `party_id?` (null ở F076) — [schema](../../specs/events/payloads/game/challenge.started.v1.schema.json) |
| `challenge.completed` | game | reward (`game.reward_engine_game`), achievement, LA, ANA | `challenge_attempt_id`, `challenge_id`, `challenge_version_id`, `status` (succeeded/failed/abandoned), `end_reason`, `score`, `difficulty`, `reward_multiplier`, `first_clear`, `objective_ids[]`, `activities_completed` — đúng một lần mỗi attempt ([CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md), [schema](../../specs/events/payloads/game/challenge.completed.v1.schema.json)) |
| `boss.encounter_started` | game (F103) | RT, LA | `encounter_id`, `boss_version_id`, `mode`, `party_activity_id`, `member_ids[]`, `deadline_at` |
| `boss.phase_completed` | game (F103) | RT, ANA | `encounter_id`, `phase_id`, `phase_index`, `retry_no`, `energy_remaining` |
| `boss.encounter_completed` | game (F103) | reward (`game.reward_engine_game`: `defeat_rewards`/`first_defeat_rewards` cho người `eligible`, idempotent theo `(encounter_id, student_id)`), QST (`game.quest_on_boss_completed`: bước `boss` cho người `eligible`), competition (`competition.party_activity_end`, theo `party_activity_id`), achievement, LA | `encounter_id`, `result` (defeated/ended), `outcome`, `party_activity_id`, `phases_cleared`, `contributions[]` (`student_id`, `eligible`, `first_defeat`), `defeat_rewards`, `first_defeat_rewards` |
| `currency.credited` / `currency.debited` | game (F075) | RT, AUD, ANA | `currency_code`, `delta`, `balance_after`, `reason_code`, `source_id` ([INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md), ADR-0049) |
| `shop.item_purchased` | game (F075) | RT, achievement-evaluator, AUD | `purchase_id`, `student_id`, `shop_item_id`, `shop_item_version_id`, `item_id`, `item_kind`, `price`, `quantity`, `currency_code`, `balance_after`, `currency_entry_id`, `purchased_at` — schema [`shop.item_purchased.v1`](../../specs/events/payloads/game/shop.item_purchased.v1.schema.json) |
| `collection.item_added` / `collection.completed` | game (F075) | RT, achievement-evaluator (`collection.completed`) | `collection_id`, `code`, `student_id`, `item_id`/`collection_type`, `owned_count`, `total_count`, `grant_id?` — phần thưởng hoàn thành cấp một lần cùng transaction; schema [`collection.item_added.v1`](../../specs/events/payloads/game/collection.item_added.v1.schema.json), [`collection.completed.v1`](../../specs/events/payloads/game/collection.completed.v1.schema.json) |
| `economy.adjusted` | game (F075) | AUD | `adjustment_id`, `student_id`, `kind`, `delta`, `entry_ids[]`, `reason`, `ticket_ref?`, `created_by`, `approved_by`, `applied_at` — schema [`economy.adjusted.v1`](../../specs/events/payloads/game/economy.adjusted.v1.schema.json) |
| `reward.granted` | game (F074, Reward Engine) | RT, AUD | `grant_id`, `student_id`, `trigger_kind` (`event_rule`, `level_up`; F075: `achievement`, `collection`, `knowledge_card`), mục `items[]` có `duplicate_xu` khi vật phẩm trùng được quy đổi, `items[]`, `xp`, `level?`, `curve_id?`, `source_event_id`, `granted_at` — mỗi `reward_grants` mới (idempotent theo `idempotency_key`); schema [`reward.granted.v1`](../../specs/events/payloads/game/reward.granted.v1.schema.json) |

### 4.7. Multiplayer & Party

| Event | Producer | Consumers | Payload chính |
|---|---|---|---|
| `party.created` | party | AUD, chat (tạo channel party) | `party_id`, `party_type`, `leader_id`, `max_members` |
| `party.settings_updated` | party | RT, AUD | `party_id`, `changed_fields[]` |
| `party.invitation_sent` / `_accepted` / `_rejected` / `_expired` / `_cancelled` | party | RT, NTF, AUD | `invitation_id`, `inviter_id`, `invitee_id` |
| `party.member_joined` / `party.member_left` / `party.member_kicked` | party | RT, chat, AUD | `party_id`, `student_id`, `reason` |
| `party.leader_transferred` | party | RT, chat (vai trò leader trong channel `party` — consumer `chat.gameplay_channels`, F118), game (co-op), AUD | `party_id`, `from`, `to`, `reason` (`manual`/`leader_disconnected`/`leader_left`) |
| `party.activity_selected` | party | RT | `party_id`, `activity_type` (gồm `boss`, [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md)), `activity_ref` |
| `party.activity_started` | party | RT, QST, AUD, chat (`chat.recent_coplayers` — "Vừa chơi cùng", delta F095) | `party_id`, `activity_ref`, `room_id`, `game_session_ref` |
| `party.activity_ended` | party | RT, AUD | `party_id`, `outcome` (completed/failed/abandoned/aborted) |
| `party.activity_completed` | party | GAM (trigger reward rule co-op), QST, LA | `party_activity_id`, `party_id`, `activity_type`, `activity_ref`, `member_ids[]` — phát **thêm** khi `party.activity_ended` có `outcome=completed`; kết quả học tập vẫn ghi riêng từng học sinh |
| `party.disbanded` | party | RT, chat (đóng channel), AUD | `party_id`, `reason` |

Ready/Not ready là trạng thái tạm thời của lobby (Valkey), **không** phải domain event: chỉ đẩy qua WS (`party.member.ready_changed`), không ghi outbox.

### 4.7.1. Quy tắc đặt tên: domain event ↔ WebSocket message

| Loại | Dạng | Ví dụ | Nơi định nghĩa |
|---|---|---|---|
| Domain event (outbox → NATS JetStream) | `<entity>.<past_verb>` — **2 phần**, động từ ghép dùng `_` | `party.member_joined`, `chat.message_held` | Tài liệu này |
| WS server → client | `<domain>.<entity>.<past_verb>` — **3 phần** | `party.member.joined`, `chat.message.created` | [AsyncAPI](../../specs/asyncapi/eduworld-realtime.v1.yaml) |
| WS client → server (lệnh) | `<domain>.<entity>.<verb>` — 3 phần, động từ hiện tại | `party.member.ready_set`, `chat.message.send` | AsyncAPI |

Module sở hữu dịch domain event/kết quả command thành WS message theo bảng ánh xạ (code `internal/presentation/realtime/eventmap.go` của từng module) rồi `PUBLISH` Valkey; `realtime` chỉ giao xuống client. Tài liệu domain khi nói về **message gửi tới client** dùng tên 3 phần; khi nói về **outbox/consumer** dùng tên 2 phần.

| Domain event | WS message (server → client) | Ghi chú |
|---|---|---|
| `party.member_joined` / `party.member_left` / `party.member_kicked` | `party.member.joined` / `party.member.left` (kèm `reason=kicked`) | |
| `party.leader_transferred` | `party.leader.transferred` | |
| `party.invitation_sent` | `party.invitation.received` (chỉ tới invitee) | |
| `party.invitation_accepted` / `_rejected` / `_expired` / `_cancelled` | `party.invitation.updated` (`status`) | |
| `party.settings_updated` | `party.state.snapshot` | Gửi lại snapshot đầy đủ |
| `party.activity_selected` | `party.activity.selected` | |
| `party.activity_started` / `party.activity_ended` | `party.session.started` / `party.session.ended` | |
| `party.disbanded` | `party.party.disbanded` | |
| `chat.message_created` (persister phát qua outbox **sau** khi commit PG; chỉ cho audit/analytics) | `chat.message.created` | Đường giao tin tới client **không** chờ domain event: module chat publish NATS JetStream → ack → `PUBLISH ew:chat:fanout:<channel_id>` (ADR-0035, §4.10) |
| `chat.message_edited` / `chat.message_recalled` | `chat.message.edited` / `chat.message.recalled` | Như trên: fan-out ngay sau JetStream ack |
| `chat.message_pinned` / `chat.message_unpinned` | `chat.pin.updated` | |
| `chat.message_held` | `chat.message.held` (chỉ người gửi) | |
| `chat.message_removed` | `chat.message.removed` | |
| `sanction.applied` | `chat.sanction.applied` (chỉ người bị áp dụng) | |
| — (trạng thái Valkey của matchmaker) | `matchmaking.ticket.updated`, `matchmaking.proposal.created` / `.updated` / `.cancelled` | Matchmaking queue là dữ liệu tạm (ADR-0004), không qua outbox |
| `match.created` | `matchmaking.match.assigned` (tới từng participant) | |
| `match.completed` | `battle.match.ended` | Từ state machine của room; `match.completed` là bản ghi bền vững |

### 4.8. Competition

| Event | Producer | Consumers | Payload chính |
|---|---|---|---|
| `match.created` | competition | AUD | `match_id`, `mode`, `participants[]` |
| `match.started` | competition | LA, chat (mở channel `match`/`team`; `chat.recent_coplayers` — "Vừa chơi cùng", delta F095) | `match_id`, `room_id`, `mode`, `matchmaking_type`, `ranked`, `is_bot`, `subject_id`, `grade_level`, `question_count`, `question_plan_version`, `participants[] {student_id, tenant_id, school_id, team_no}` (owner F105) |
| `match.completed` | competition | RNK, LDB, GAM, LR, NTF, chat (đóng channel) | `match_id`, `mode`, `matchmaking_type`, `ranked`, `rated`, `is_bot`, `subject_id`, `grade_level`, `status`, `end_reason`, `winner_team`, `tournament_id`, `tournament_node_id`, `results[] {student_id, school_id, team_no, outcome, score, correct_count}` (owner F105; một lần/trận) |
| `match.aborted` | competition | AUD, chat (đóng channel) | `match_id`, `mode`, `reason` (`server_failure`/`plan_failure`), `participants[]` (owner F105; không kết quả, không rating) |
| `rating.updated` | ranking (F106) | LDB, RT | `match_id`, `student_id`, `school_id`, `mode`, `subject_id`, `season_id`, `rating`, `rd`, `games_played`, `provisional`, `tier_code`, `rule_version_id` — số rating chỉ cho consumer server; client nhận `competition.tier.updated` (chỉ tier) |
| `competition_points.awarded` | ranking (F106) | LDB, RT, ANA | `student_id`, `school_id`, `season_id`, `source_type` (`match`/`tournament`), `source_id`, `mode`, `subject_id`, `points`, `balance_after`, `capped`, `day_key`, `rule_version_id` |
| `leaderboard.period_closed` | leaderboard (F106, job đóng kỳ) | NTF (trong game), RT, AUD | `leaderboard_id`, `code`, `metric`, `scope_type`, `scope_id`, `school_id`, `grade_level`, `period_key`, `members`, `top[]` (≤ 100: `student_id`, `rank`, `value`), `grade_goal {current, target, reached}` (mục tiêu của khối: `top` rỗng) |
| `competition_config.published` | competition (F108, platform event, cùng transaction với `content_releases`) | competition (consumer `competition.config_apply` + job `competition-config-apply`: đồng bộ `seasons`, `leaderboard_definitions` khi version có hiệu lực; matchmaking/ranking đọc version hiệu lực theo dòng thời gian release, trận/giải đã bắt đầu giữ version ghim — F104/F106/F107), learning (`content.external_usages`, `ref_type = competition` — chờ phía learning như F078 T040), AUD | `config_type`, `config_id`, `code`, `config_version_id` (null khi unpublish), `version`, `previous_version_id`, `action` (`publish`/`rollback`/`unpublish`), `effective_from` (publish ≥ lúc publish; rollback/unpublish = lúc thao tác), `release_id`, `released_by`, `reason`, `content_refs[]` — schema `specs/events/payloads/competition/competition_config.published.v1` |
| `tournament.match_scheduled` / `tournament.completed` | tournament (F107) | NTF, RT, GAM | `match_scheduled`: `tournament_id`, `node_id`, `school_id`, `scheduled_at`, `notice_minutes` (10/1, mỗi mốc một lần), `entry_ids[2]`, `student_ids[]`; `completed`: `tournament_id`, `challenge_id`, `school_id`, `format`, `completed_at`, `results[] {entry_id, final_rank, student_ids[]}` — chỉ id, không tên (R16) |
| `tournament.rewarded` | tournament (F107) | module game (cấp vật phẩm/badge/title theo `reward_ref`), AUD | `tournament_id`, `entry_id`, `rule_id`, `final_rank`, `student_ids[]`, `reward_ref {reward_rule_code?, item_code?, badge_code?, title_code?}`, `points` — một lần mỗi `(tournament_id, entry_id, rule_id)`; consumer `competition.tournament_match` của chính module tiêu thụ `match.completed`/`match.aborted` để đẩy bracket |
| `season.started` | season (F106, platform event) | module game (`game.season_projection` — projection `ref.seasons` theo `source_version`, mở season track, F077), LDB | `season_id`, `code`, `name`, `theme_code`, `starts_at`, `ends_at`, `previous_season_id`, `source_version` |
| `season.ended` | season | GAM (season reward theo tier), module game (đóng season track, tự cấp thưởng chưa nhận — [EVENT_SEASON_SYSTEM §7](../01-game-design/EVENT_SEASON_SYSTEM.md)), LDB | `season_id`, `code`, `ended_at`, `results_count`, `source_version` (F106, platform event — sau khi `season_results` đã ghi) |
| `duel.sent` / `duel.accepted` / `duel.declined` / `duel.expired` | competition (friend duel) | RT, NTF (trong game), AUD | `duel_id`, `challenger_id`, `opponent_id`, `subject_id`, `match_id?` — unranked, không phát `rating.updated` ([MATCHMAKING §8](../03-social-multiplayer/MATCHMAKING.md)) |

### 4.9. Social & Chat

| Event | Producer | Consumers | Payload chính |
|---|---|---|---|
| `friend_request.sent` / `friend.added` / `friend.removed` | friend | RT, NTF, competition (`friend.*` → projection `ref.friendships`: hub, party) | `requester_id`, `addressee_id` |
| `user.blocked` / `user.unblocked` | friend | chat, competition (party, matchmaking, duel), AUD | `blocker_id`, `blocked_id` |
| `user.muted` / `user.unmuted` | friend | chat (ẩn tin), RT | `muter_id`, `muted_id` |
| `club.created` / `club.approved` / `club.archived` | club ([SOCIAL_SYSTEM §6](../03-social-multiplayer/SOCIAL_SYSTEM.md)) | NTF, AUD (channel club tạo/khóa trong cùng transaction của lệnh club, F120) | `club_id`, `tenant_id`, `club_type`, `owner_id`, `school_id?`, `status`/`chat_channel_id`/`actor_id` theo event — schema `specs/events/payloads/chat/club.*.v1` |
| `club.member_joined` / `club.member_left` | club | RT, AUD (thành viên channel club cập nhật cùng transaction, F120) | `club_id`, `tenant_id`, `user_id`, `role`, `chat_channel_id?`, `source`/`reason` |
| `club.goal_completed` | club | reward (huy hiệu club), NTF | `club_id`, `goal_id`, `metric`, `target` |
| `club.invitation_sent` | club | NTF (`chat.notify_club` → `club_invitation`), AUD | `invitation_id`, `club_id`, `tenant_id`, `inviter_id`, `invitee_id`, `expires_at`, `occurred_at` — chỉ khi tạo lời mời mới (delta F120 cho F111); schema [`club.invitation_sent.v1`](../../specs/events/payloads/chat/club.invitation_sent.v1.schema.json) |
| `club.join_requested` | club | NTF (`chat.notify_club` → `club_join_request` cho chủ club + officer), AUD | `request_id`, `club_id`, `tenant_id`, `requester_id`, `occurred_at` — chỉ khi tạo yêu cầu mới (delta F120 cho F111); schema [`club.join_requested.v1`](../../specs/events/payloads/chat/club.join_requested.v1.schema.json) |
| `chat.message_created` | chat-persister (sau khi commit PG; nguồn: NATS `ew.chat.*.msg.*`) | AUD/LA (đếm, không nội dung) | `message_id`, `channel_id`, `seq`, `sender_id`, `kind` — **không** chứa nội dung text. Các consumer cần nội dung (moderation async, notifier) đọc trực tiếp NATS JetStream (§4.10) |
| `chat.message_edited` / `chat.message_recalled` / `chat.message_forwarded` | chat-persister | AUD, MOD (sửa/forward → tín hiệu async) | `message_id`, `channel_id`, `revision?`, `actor_id`, `target_channel_ids?` — không nội dung |
| `chat.message_pinned` / `chat.message_unpinned` | chat-persister | AUD | `channel_id`, `message_id`, `actor_id` |
| `chat.daily_stats` | chat (job `gameplay-chat-purge`: phần cộng thêm của rollup `gameplay_chat_daily_stats` trước khi xóa, F117) | ANA | `tenant_id`, `stat_date`, `channel_type`, `channels`, `messages`, `masked`, `blocked` (tin held/removed đã lưu), `reported` — **không** nội dung, không theo người, không theo trường/khối (R14, R16) |
| `chat.message_held` | chat | MOD | `message_id`, `channel_id`, `reasons[]` (`category_codes` của danh mục, `pii`, `bullying`), `catalog_version` |
| `chat.message_removed` | moderation | RT, AUD | `message_id`, `channel_id`, `by` |
| `report.created` | moderation | MOD queue, NTF (`safety_moderator`) | `report_id`, `target_type`, `target_id`, `reason` |
| `report.resolved` | moderation (F121, khi case `resolved`) | NTF (F123: thông báo `moderation_notice` "Báo cáo của em đã được xử lý" cho người báo cáo) | `report_id`, `case_id`, `reporter_id`, `kind` (`moderation_notice`), `message_code` — **không** chi tiết hình phạt, không danh tính người bị báo cáo |
| `moderation.case_assigned` | moderation | NTF (`safety_moderator` được gán), AUD | `case_id`, `tenant_id`, `severity`, `assignee_moderator_id`, `assigned_via` (`rotation`/`manual`) ([MODERATION](../03-social-multiplayer/MODERATION.md)) |
| `moderation.school_notified` | moderation | NTF (`school_admin` của trường học sinh), identity (gửi email SMTP), AUD | `case_id`, `tenant_id`, `school_id`, `severity`, `category_code`, `school_notified_at` — tóm tắt, **không** chứa nội dung chat thô; tenant `common` không phát (Safety Team liên hệ trực tiếp) |
| `moderation.school_acknowledged` | moderation | NTF (`assignee_moderator_id`), AUD | `case_id`, `tenant_id`, `school_id`, `school_ack_at`, `acknowledged_by`, `contact_outcome` (`parent_contacted`/`unable_to_contact`/`not_needed`) |
| `moderation.school_notification_reminded` | moderation (F124, job nhắc lại) | identity (email nhắc `school_admin`), AUD | `notification_id`, `case_id`, `tenant_id`, `school_id`, `reminder_no`, `ack_due_at`, `reminded_at` — không nội dung chat |
| `moderation.oncall_paged` | moderation (F124, vòng trực p1) | identity (email gọi người trực), AUD | `case_id`, `tenant_id`, `severity`, `level` (`primary`/`secondary`/`lead`/`system_admin`), `moderator_user_id?`, `paged_at` — **event platform** (envelope `tenant_id` null: người nhận là nhân sự platform; `tenant_id` trong payload chỉ để audit) |
| `break_glass.requested` | moderation (F124) | identity (email người phê duyệt), AUD | `grant_id`, `tenant_id`, `requested_by`, `requested_at` — **event platform** (envelope `tenant_id` null); không lý do, phạm vi hay nội dung |
| `sanction.applied` / `sanction.lifted` | moderation | chat, RT, identity (khóa/giải phóng tài khoản theo nguồn `moderation_sanction`, thu hồi phiên), competition (projection `ref.sanctions`: chặn party/ghép trận), AUD | `sanction_id`, `user_id`, `tenant_id`, `case_id`, `sanction_type` (enum theo [MODERATION](../03-social-multiplayer/MODERATION.md)), `until`, `source_version`; `applied` thêm `scope`, `reason_code`, `issued_by_kind`; `lifted` thêm `lift_reason` (`revoked`/`expired`/`case_resolved`), `lifted_at` — không lý do nội bộ |
| `sensitive_term.created` / `sensitive_term.updated` / `sensitive_term.deleted` | moderation (danh mục) | AUD | `term_id`, `category_code`, `scope`, `tenant_id?`, `actor_id` — **không** chứa nội dung term |
| `sensitive_catalog.published` | moderation (danh mục) | worker (phát `ew:chat:sensitive:version`), AUD | `scope`, `tenant_id?`, `catalog_version`, `term_count`, `checksum` |
| `chat_policy.moderation_switch_changed` | moderation (công tắc) | RT/API (invalidate cache), AUD, MON (metric `policy_disabled`), NTF (thông báo Tenant Admin của tenant bị ảnh hưởng) | `scope`, `tenant_id?`, `before`, `after`, `reason`, `expires_at?`, `actor_id`, `source` (`manual`/`auto_expire`) |
| `moderation_switch.change_rejected` | moderation (công tắc, F122) | AUD, MON | `request_id`, `scope`, `tenant_id?`, `enabled_all`, `steps`, `proposed_by`, `rejected_by`, `reason`, `rejected_at` — yêu cầu tắt bị System Admin thứ hai từ chối, giá trị hiệu lực không đổi; schema [`moderation_switch.change_rejected.v1`](../../specs/events/payloads/chat/moderation_switch.change_rejected.v1.schema.json) |
| `chat.channel_opened` | chat (consumer `chat.gameplay_channels`, F117) | RT (auto-join thành viên) | `channel_id`, `channel_type` (`match`/`party`/`team`), `tenant_id`, `scope_ref`, `member_ids[]`, `opened_at` — channel gameplay tự tạo/đổi membership khi vào trận/tạo party (ADR-0020 §6) |
| `chat.channel_closed` | chat (consumer `chat.gameplay_channels` khi `match.completed`/`match.aborted`/`party.disbanded`, đặt `purge_after` trong cùng transaction — F117) | RT, AUD | `channel_id`, `channel_type`, `tenant_id`, `scope_ref`, `closed_at`, `purge_after` (ADR-0033) |
| `chat.gameplay_purged` | worker `gameplay-chat-purge` | AUD (tổng hợp), MON | `run_id`, `tenant_id` (null = dòng cấp lượt chạy), `kind` (`gameplay`/`ephemeral`), `status` (`succeeded`/`partial`/`failed`/`blocked`), `channels`, `messages`, `partitions_dropped`, `deferred_open_case`, `evidence_copied`, `started_at`, `finished_at` — **không** nội dung |
| `chat_policy.tenant_category_toggled` | moderation | RT/API (invalidate), AUD | `tenant_id`, `category_code`, `enabled`, `reason`, `actor_id` |
| `chat_policy.updated` | chat (F120, chính sách chat theo cấp) | chat (vô hiệu cache effective policy `chat_policy_cache`), identity (bản chiếu `direct_message_enabled`, delta F112-7), RT, AUD | `policy_id`, `scope_type` (`platform`/`tenant`/`school` — không có `class`, ADR-0095), `scope_id?`, `tenant_id?`, `policy_version`, `changed_fields[]`, `preset?`, `direct_message_enabled?` (cấp này có để `direct` bật không — thêm 2026-11-28, F112-7), `actor_id`, `occurred_at` — không nội dung tin; schema [`chat_policy.updated.v1`](../../specs/events/payloads/chat/chat_policy.updated.v1.schema.json) |
| `chat_policy.common_tenant_social_updated` | chat (F120, `putCommonTenantSocialOverride`, delta F112-7) | identity (bản chiếu `direct_message_enabled` của tenant `common`), AUD | `tenant_id`, `direct_enabled`, `world_enabled`, `area_free_text_enabled`, `version`, `actor_id`, `occurred_at` — không lý do (lý do ở audit); schema [`chat_policy.common_tenant_social_updated.v1`](../../specs/events/payloads/chat/chat_policy.common_tenant_social_updated.v1.schema.json) |

### 4.10. Chat transport trên NATS JetStream (ADR-0035) — khác outbox

Tin nhắn chat **không** đi qua outbox vì cần độ trễ thấp: module chat publish thẳng vào stream `CHAT_MESSAGES` làm đường ghi bền vững. Outbox (→ `EVT_*`, ADR-0042) dùng cho mọi domain event khác, kể cả `chat.message_*` dành cho audit/analytics.

| Subject | Nội dung | Publisher | Consumer (durable) |
|---|---|---|---|
| `ew.chat.<tenant_id>.msg.<channel_id>` | Tin mới (envelope đầy đủ, nội dung **đã kiểm duyệt** + bản gốc khi `mask`) | module chat (handler `ew.cmd.chat.message.send` hoặc REST fallback) | `persister`, `notifier`, `moderation`, `stats` (profile `worker-chat`) |
| `ew.chat.<tenant_id>.evt.<channel_id>` | `edit` (revision mới), `recall`, `reaction_set/unset`, `pin/unpin`, `forward` (tạo tin ở channel đích = msg riêng), `remove` (moderator) | module chat | `persister`, `moderation`, `stats` |
| `ew.chat.<tenant_id>.cursor.<channel_id>` | Con trỏ gộp `{user_id, delivered_seq, read_seq}` | module chat (nhận receipt từ gateway qua `ew.cmd.chat.receipt.*`) | `persister` |
| `ew.dlq.chat.<consumer>` (stream `DLQ_CHAT`) | Message lỗi quá `max_deliver` | consumer | Vận hành (alert) |

Envelope chat:

```json
{
  "v": 1,
  "kind": "msg",
  "message_id": "0193a6b2-...",
  "tenant_id": "0192f0...",
  "channel_id": "0193a1...",
  "channel_type": "party",
  "seq": 1842,
  "revision": 1,
  "sender_id": "0192ab...",
  "client_message_id": "6f1c...",
  "msg_kind": "text",
  "body_display": "Mình làm câu 3 rồi nè",
  "body_original_ref": null,
  "reply_to_message_id": null,
  "quote": null,
  "forwarded_from": null,
  "mentions": [],
  "sticker_id": null,
  "gif_id": null,
  "moderation": { "decision": "allow", "catalog_version": 57, "steps_skipped": [] },
  "created_at": "2026-11-15T08:30:12.345Z",
  "trace_id": "4bf92f..."
}
```

- Header NATS `Nats-Msg-Id = message_id` (khử trùng lặp trong `duplicate_window` 2'); persister `ON CONFLICT (message_id) DO NOTHING`.
- Sự kiện `evt` mang `revision` tăng dần; persister bỏ qua bản có `revision` ≤ revision hiện tại (đến muộn).
- Nội dung gốc khi `mask` không nằm trong `body_display`; nếu cần lưu (bằng chứng), module chat gửi trong trường `body_original` chỉ đọc được bởi consumer `persister`/`moderation` (stream không mở cho consumer khác) [Confirmed].
- Stream `CHAT_MESSAGES`: file storage, 3 replicas, `max_age` 72 h, `duplicate_window` 2'. Consumer `ack_wait` 30 s, `max_deliver` 5.
- Domain event `chat.message_*` ở §4.9 do persister phát **sau** khi commit PG (qua outbox), dành cho audit/analytics không cần nội dung.

## 5. Learning events (Master Prompt §16)

### 5.1. Trường bổ sung bắt buộc

| Trường | Ghi chú |
|---|---|
| `student_id` | = `actor.id` khi actor là học sinh |
| `tenant_id` | Tenant của học sinh tại thời điểm xảy ra (khóa cô lập) |
| `school_id` | Trường (bên trong tenant) của học sinh tại thời điểm xảy ra |
| `session_id` | `game_session_id` |
| `activity_id` | Learning activity / quest step / match / assignment ref |
| `content_id`, `content_version_id` | Nội dung cụ thể (question version, question set version, lesson version…) |
| `actor` | `type` ∈ `student` (học sinh tự làm), `system` (server/worker sinh: chấm, mastery), `staff` (nhân sự kích hoạt, vd. re-evaluate) |
| `context` | `{ mode, assignment_id?, exam_session_id?, quest_version_id?, minigame_version_id?, challenge_attempt_id?, match_id?, party_id?, is_official?, efficacy_wave_id?, experiment_arms? }` (F035: `efficacy_wave_id` trên event của attempt ExamSession đo hiệu quả — mọi consumer mastery, progress, gợi ý, rollup và recompute bỏ qua; `experiment_arms` = danh sách `<experiment_code>:<arm>` ≤ 10 của học sinh trong thử nghiệm `shadow`/`live`, không bao giờ trả về client); `mode` theo enum chuẩn `AttemptMode` (`practice`, `assignment`, `quest`, `minigame`, `challenge`, `exam`, `competition`, `retry_wrong`, `diagnostic` — [enums.schema.json](../../specs/schemas/common/enums.schema.json)). Event không gắn attempt (`lesson.completed`, `quest.completed`) dùng mode của ngữ cảnh (`practice` khi tự học, `assignment` khi nhà trường giao, `quest`) |
| `objective_ids[]` | Learning objectives liên quan |

### 5.2. Catalog learning event

| `event_type` | Khi nào | Payload chính |
|---|---|---|
| `activity.started` | Bắt đầu learning activity bất kỳ | `source_kind` (`learning_activity`/`lesson`/`question_set`/`assessment`/`quest_step`/`challenge`), `activity_type?` (`ActivityType`), `attempt_id?` |
| `activity.submitted` | Nộp attempt của learning activity | `attempt_id`, `activity_type`, `status` (`submitted`/`graded`), `score?`, `correct_count?`, `item_count?`, `hints_used`, `duration_ms`, `submit_reason` |
| `activity.graded` | Server chấm xong attempt (đồng bộ hoặc trì hoãn); `actor.type = system` | `attempt_id`, `evaluation_id`, `activity_type?`, `score`, `passed?`, `correct_count?`, `item_count?`, `result_release?` |
| `activity.completed` | Hoàn thành activity | `source_kind`, `duration_ms`, `completion`, `score?` |
| `video.progressed` | `watched_ratio` vượt mốc 25/50/75/100% (không phát mỗi heartbeat) | `attempt_id`, `watched_ratio`, `chapter_id?` ([VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md)) |
| `video.cue_answered` | Trả lời câu hỏi chèn trong video/audio | `attempt_id`, `cue_id`, `question_version_id`, `is_correct`, `hints_used?`, `rapid_guess?` |
| `simulation.goal_met` | Checker xác nhận đạt một mục tiêu mô phỏng | `attempt_id`, `run_id?`, `goal_id` |
| `lesson.completed` | Hoàn thành bài học | `duration_ms`, `progress` (0..1) |
| `question.viewed` | Server gửi render payload câu hỏi cho học sinh | `attempt_id`, `position` (≥ 1), `question_type` |
| `question.answered` | Server chấm một câu trả lời | `attempt_id`, `position` (≥ 1), `question_version_id`, `question_type` (`QuestionType`), `is_correct`, `is_scored`, `score`, `response` (theo [`question.schema.json#/$defs/response`](../../specs/schemas/question.schema.json)), `response_ms` (server), `attempt_no`, `hints_used` (số cấp gợi ý đã mở trước lần trả lời), `rapid_guess` ([ACTIVITY_ENGINE §11.1](../02-learning/ACTIVITY_ENGINE.md#111-cờ-rapid_guess--trả-lời-quá-nhanh-để-kịp-đọc-đề-confirmed)), `error_tags[]` |
| `question.correct` / `question.incorrect` | Dẫn xuất từ `question.answered` (để tương thích catalog §16; không ghi trùng bản ghi — xem §5.4) | — |
| `exam.started` / `exam.submitted` | Như domain event | `attempt_id`, `exam_session_id`, `exam_type`, `is_official` |
| `exam.graded` | Server chấm xong bài thi; evidence mastery của đề lấy từ event này; `actor.type = system` | `attempt_id`, `evaluation_id`, `exam_session_id`, `score`, `is_official`, `result_release` |
| `attempt.re_evaluated` | Hệ thống chấm lại do nội dung lỗi (event bù trừ, không xóa event cũ); `actor.type = staff` hoặc `system` | `attempt_id`, `previous_evaluation_id`, `new_evaluation_id`, `previous_score`, `new_score`, `question_version_ids[]`, `reason`, `trigger.role_code` (`content_admin`/`academic_manager`) |
| `quest.completed` | Hoàn thành quest có gắn LO | `quest_version_id` |
| `learning_objective.progressed` | Mastery thay đổi | `from`, `to`, `level`, `model_version` |
| `learning_objective.mastered` | Đạt ngưỡng mastered; `actor.type = system` | `objective_id`, `previous_level`, `new_level` (`MasteryLevel`: `not_started`/`beginning`/`developing`/`proficient`/`mastered`), `value`, `evidence_count`, `distinct_days`, `model_version` |

### 5.3. Ánh xạ xAPI (định hướng tương thích; full LRS endpoint chỉ làm khi có quyết định — Master Prompt §16 không bắt buộc)

| EduWorld `event_type` | xAPI verb | xAPI object type | Result |
|---|---|---|---|
| `activity.started` | `http://adlnet.gov/expapi/verbs/launched` | Activity (`learning_activity`) | — |
| `activity.completed` | `http://adlnet.gov/expapi/verbs/completed` | Activity | `completion: true`, `duration` |
| `lesson.completed` | `http://adlnet.gov/expapi/verbs/completed` | Activity (`lesson`) | `duration` |
| `question.viewed` | `http://id.tincanapi.com/verb/viewed` | `cmi.interaction` | — |
| `question.answered` | `http://adlnet.gov/expapi/verbs/answered` | `cmi.interaction` (`interactionType` theo `QuestionType`, bảng dưới) | `success`, `score.scaled`, `response` (đã ẩn danh hóa nếu xuất); extension `hints_used`, `rapid_guess` |
| `activity.submitted` / `exam.submitted` | `http://adlnet.gov/expapi/verbs/completed` | Assessment | `score.raw/max/scaled`, `duration` |
| `activity.graded` / `exam.graded` | `http://adlnet.gov/expapi/verbs/scored` | Assessment | `score.raw/max/scaled`, `success` = `passed` |
| `attempt.re_evaluated` | `http://adlnet.gov/expapi/verbs/scored` | Assessment | Statement mới với `score` mới; context extension `previous_evaluation_id` (statement cũ giữ nguyên) |
| `exam.started` | `http://adlnet.gov/expapi/verbs/attempted` | Assessment | — |
| `quest.completed` | `http://adlnet.gov/expapi/verbs/completed` | Activity (`quest`) | — |
| `learning_objective.mastered` | `http://adlnet.gov/expapi/verbs/mastered` | Activity (`learning_objective`) | `score.scaled` = mastery value |
| `learning_objective.progressed` | `http://adlnet.gov/expapi/verbs/progressed` | Activity (`learning_objective`) | `score.scaled` |

Ánh xạ `QuestionType` → xAPI `interactionType`:

| `QuestionType` | `interactionType` |
|---|---|
| `single_choice`, `multiple_choice` | `choice` |
| `true_false` | `true-false` |
| `fill_in_blank` | `fill-in` |
| `short_answer` | `fill-in` |
| `numeric` | `numeric` |
| `matching` | `matching` |
| `ordering` | `sequencing` |
| `drag_drop`, `image_based` | `other` (kèm extension `question_type`) |
| `interactive` | `other` |

`context.mode` (`AttemptMode`) xuất thành context extension `mode`; `diagnostic` thêm `contextActivities.category` "diagnostic" để phân biệt bài chẩn đoán với bài có điểm.

Actor xAPI dùng `account { homePage: "https://eduworld.vn", name: <pseudonymous student id> }` — **không** dùng email/tên. Statement có `actor.type = staff` (re-evaluate) ghi người kích hoạt trong `authority`, actor vẫn là học sinh. Context: `registration` = `attempt_id`; `contextActivities.parent` = assignment/quest/match; `extensions` chứa `tenant_id`, `school_id`, `content_version_id`. Export xAPI là chức năng tương lai (adapter trong `learning_analytics`).

### 5.4. Quy tắc ghi

1. Learning event ghi trong **cùng transaction** với thay đổi nghiệp vụ (attempt answer, submit).
2. `idempotency_key` tất định, unique trong bảng: vd. `qa:<attempt_id>:<position>:<answer_seq>`, `sub:<attempt_id>`, `lo:<student_id>:<objective_id>:<source_event_id>`. Ghi lặp → `ON CONFLICT DO NOTHING`.
3. `question.correct`/`question.incorrect` là **view** trên `question.answered` (`is_correct`), không phải bản ghi riêng — tránh trùng lặp số liệu.
4. Event từ client (vd. `lesson.completed` cho nội dung video) được **server xác nhận**: client gửi REST, server kiểm tra hợp lý (thời lượng tối thiểu, trạng thái lesson) rồi mới ghi; client không ghi trực tiếp learning event.
5. Learning event sinh trong match/competition có `context.mode = "competition"`; trọng số mastery thấp hơn (ADR-0017).
6. Bản ghi `attempt_answers` (DB) và payload `question.answered` dùng cùng tên trường `response_ms`, `hints_used`, `rapid_guess` ([ACTIVITY_ENGINE §6.4](../02-learning/ACTIVITY_ENGINE.md#64-dữ-liệu-lưu-trữ-confirmed)).
7. Chấm lại không sửa event cũ: `attempt.re_evaluated` là bút toán bù trừ; learning_record tính lại LO bị ảnh hưởng ([LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)). `question.answered` có `rapid_guess = true` hoặc `is_scored = false` không phải evidence mastery; `hints_used` giảm trọng số evidence theo `w_hint`.

## 6. Versioning schema

- JSON Schema cho từng `event_type` × `schema_version` đặt trong `specs/schemas/` (learning event: [learning-event.schema.json](../../specs/schemas/learning-event.schema.json); domain event: payload `specs/events/payloads/<module>/<event_type>.v<n>.schema.json` — [danh sách](../../specs/events/README.md), bổ sung dần theo module). Enum học tập dùng chung (`QuestionType`, `ActivityType`, `AttemptMode`, `AttemptStatus`, `FeedbackTiming`, `ResultRelease`, `LearningContentType`) chỉ định nghĩa ở [`specs/schemas/common/enums.schema.json`](../../specs/schemas/common/enums.schema.json); CI chạy `node scripts/specs/check-enums.mjs`.
- **Thay đổi tương thích** (thêm trường optional): giữ `schema_version`.
- **Thay đổi không tương thích**: tăng `schema_version`; producer phát song song v cũ và v mới trong ≥ 1 release; consumer xử lý theo `schema_version`.
- CI: unit test producer validate envelope theo schema; contract test consumer với fixture.

## 7. Ordering, retry, DLQ

| Vấn đề | Quy tắc |
|---|---|
| Ordering | Chỉ bảo đảm **theo `partition_key`** khi consumer xử lý tuần tự trong một stream. Consumer cần thứ tự (vd. ranking theo student) dùng `occurred_at` + version check (optimistic) thay vì giả định thứ tự toàn cục |
| At-least-once | Consumer bắt buộc idempotent (`infra.processed_events` trong database của module nhận) |
| Retry | Lỗi tạm thời: `Nak` với delay theo `NumDelivered`: 1′, 5′, 30′ (`BackOff` của consumer); `ack_wait` 60 s |
| DLQ | `NumDelivered > 5` hoặc lỗi không phục hồi (schema invalid) → publish `ew.dlq.<module>.<consumer>` (stream `DLQ_<MODULE>`) + bản ghi `infra.dead_letters` (database của module nhận) + `Term` message gốc; cảnh báo vận hành |
| Replay | Trong 7 ngày: tạo consumer tạm với `DeliverByStartTime`; xa hơn: `src/backend/cmd/worker replay --module=<module phát> --from <time>` đọc `infra.outbox_events` của module phát (30 ngày) và publish lại (khử trùng nhờ `Nats-Msg-Id` + `processed_events`) |
| Poison message | Không chặn consumer; luôn chuyển DLQ |

## 8. Retention

| Dữ liệu | Thời gian | Nhãn |
|---|---|---|
| NATS JetStream `EVT_*` | 7 ngày (`max_age`), file storage, 3 replicas | [Confirmed] |
| NATS JetStream `CHAT_MESSAGES` | 72 giờ | [Confirmed] |
| NATS JetStream `DLQ_<MODULE>` | 30 ngày | [Confirmed] |
| `infra.outbox_events` | `outbox_retention` (30 ngày) sau `published_at` hiển thị cho replay; partition theo ngày, job `infra-partition-maintenance` (role `<module>_partition_admin`) gỡ partition hết hạn và từ chối gỡ partition còn dòng chưa publish; ứng dụng không `DELETE` (ngoại lệ có giới hạn của ADR-0070 theo ADR-0089 §5, [DATABASE_DESIGN §3.9](DATABASE_DESIGN.md#39-outbox--processed-events)) | [Confirmed] |
| `infra.processed_events` | `outbox_retention` (30 ngày); partition theo ngày trên `event_occurred_at` của envelope (event giao lại luôn vào cùng partition), gỡ partition như outbox (ADR-0089 §4–5) | [Confirmed] |
| `infra.dead_letters` | 90 ngày; partition theo ngày, job `infra-partition-maintenance` gỡ partition hết hạn và từ chối gỡ partition còn dead letter chưa xử lý (`resolved_at IS NULL`) (ADR-0089 §5) | [Confirmed] |
| `learning.learning_events` | Suốt thời gian học tại trường + 1 năm; sau đó ẩn danh hóa/tổng hợp | [Confirmed] |
| Event chat (`chat.*`) | Theo retention chat (`persistent_chat_retention_days`, 180 ngày) | [Confirmed — owner 2026-11-20] OQ-09 |
| Dữ liệu chat gameplay (channel `match`/`party`/`team` theo trận, tin ephemeral) | Purge vật lý sau `gameplay_chat_retention_days` (mặc định 7, tối thiểu 3) / 24 h — ngoại lệ ADR-0033; outbox event chỉ chứa ID nên không cần purge | [Confirmed] |

Partition `learning.learning_events` theo tháng (`occurred_at`). Hết hạn: ứng dụng chỉ ẩn danh hóa; partition giữ lại; Release 1.0 không detach, archive hay gỡ partition (ADR-0028, ADR-0070).

## 9. Privacy

- Payload chỉ chứa ID nội bộ (UUIDv7), không tên, email, ngày sinh, nội dung chat, câu trả lời văn bản của học sinh trong **domain event** (consumer cần chi tiết thì gọi facade của module sở hữu với scope, ADR-0074).
- Learning event lưu `response` của câu trả lời trong `learning.learning_events.payload` chỉ khi cần cho phân tích lỗi sai; câu trả lời dạng văn bản tự do (short answer) được giới hạn 500 ký tự và chạy PII detector trước khi lưu [Confirmed].
- Mọi consumer xử lý event với `TenantScope` = `tenant_id` của event; event platform (`tenant_id = null`) chỉ consumer platform xử lý.
- Analytics chỉ dùng event cho mục đích đã công bố (Master Prompt §16); dashboard tổng hợp vượt tenant (chỉ System Admin) áp dụng ngưỡng k-anonymity (MULTITENANCY §7).
- Yêu cầu xóa dữ liệu (DATA_PRIVACY) được thực thi bằng job ẩn danh hóa `actor.id`/`student_id` trong learning events (thay bằng pseudonym không đảo ngược), giữ số liệu tổng hợp.

## 10. Observability

| Metric | Ý nghĩa |
|---|---|
| `eduworld_outbox_unpublished_count{module}`, `eduworld_outbox_lag_seconds{module}` | Relay chậm/kẹt (cảnh báo > 60 s) |
| `eduworld_events_consumed_total{module,consumer,event_type,result}` | Thông lượng, lỗi |
| `eduworld_consumer_pending{stream,consumer}` | Số message chờ trên JetStream (từ `nats` exporter) |
| `eduworld_events_consumer_lag_seconds{consumer}` | `now - occurred_at` của event vừa xử lý |
| `eduworld_events_dlq_total{consumer,event_type}` | Cảnh báo > 0 |
| `eduworld_learning_events_written_total{event_type}` | Theo dõi Learning Completion / Exam Submission (MONITORING) |

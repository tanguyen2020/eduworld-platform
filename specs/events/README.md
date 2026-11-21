# EduWorld Event Specs

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: Backend Architect · Cập nhật: 2026-11-21 (ADR-0095: `identity/student.placement_changed.v1` — owner F010, thay `enrollment.changed`; `learning/assignment.created.v2` giao theo khối — owner F041; schema `class.*`, `enrollment.changed`, `study_group.*`, `schedule_item.deleted` Deprecated, xóa cùng code) · 2026-11-21 (WP-L1: xóa `study_group.*`, `schedule_item.deleted`, `assignment.created.v1`) · 2026-11-17 (payload schema M-C0/M-C1: `common/audit.recorded`, event identity của F010/F011/F013/F015/F016, event game của F014) · 2026-11-15
> Liên quan: [EVENT_DESIGN](../../docs/07-backend/EVENT_DESIGN.md) · [ADR-0042](../../docs/adr/ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) · [ADR-0035](../../docs/adr/ADR-0035-duong-truyen-chat-nats-jetstream-valkey-pub-sub-luu-postgres.md)

Hợp đồng máy đọc cho **domain event giữa các service**. Catalog nghiệp vụ (event nào, ai phát, ai nhận) nằm ở [EVENT_DESIGN §4](../../docs/07-backend/EVENT_DESIGN.md#4-catalog-domain-events). Thư mục này giữ schema để producer/consumer validate trong contract test.

## File

| File | Nội dung |
|---|---|
| `envelope.schema.json` | Envelope chuẩn của mọi domain event (EVENT_DESIGN §2): `event_id` (UUIDv7, cũng là `Nats-Msg-Id`), `event_type`, `schema_version`, `tenant_id`, `actor`, `subject`, `idempotency_key`, `payload` |
| [`../schemas/learning-event.schema.json`](../schemas/learning-event.schema.json) | Learning event (xAPI-like), mở rộng envelope với trường học tập bắt buộc |
| `chat-envelope.v1.schema.json` | Envelope tin chat trên NATS JetStream `CHAT_MESSAGES` (`ew.chat.<tenant_id>.msg.<channel_id>`, ADR-0035, EVENT_DESIGN §4.10) — **không** phải domain event outbox; owner F115 |

Payload đã có schema:

| Service | File (`payloads/<service>/<event_type>.v<schema_version>.schema.json`) |
|---|---|
| common | `audit.recorded.v1` (mọi module phát, owner F003) |
| identity | `user_role.changed.v1`, `role.updated.v1`, `province.updated.v1`, `class.synced.v1` (Deprecated — ADR-0095); F010: `tenant.created.v1`, `tenant.updated.v1`, `school.updated.v1`, `class.created.v1`, `class.updated.v1`, `enrollment.changed.v1` (ba schema này Deprecated — ADR-0095), `student.placement_changed.v1` (ADR-0095, thay `enrollment.changed`), `student.transferred.v1`, `tenant_membership.changed.v1`, `user.provisioned.v1`; F011: `school.synced.v1`, `student.synced.v1`, `student.provisioning_requested.v1`, `student.sis_link_created.v1`, `student.sis_link_superseded.v1`, `user.status_changed.v1`, `user.access_revoked.v1`, `sis.sync_completed.v1`, `sis.reconciliation_issue_raised.v1`; F013: `user.activated.v1`; F015: `platform_settings.changed.v1`; F016: `student.cleaned_up.v1`, `student.restored.v1` |
| chat | F095: `friend_request.sent.v1`, `friend.added.v1`, `friend.removed.v1`, `user.blocked.v1`, `user.unblocked.v1`, `user.muted.v1`, `user.unmuted.v1`; F115: `chat.message_created.v1` (không nội dung) |
| competition | F100/F101: `party.*`; F104: `match.created.v1`, `duel.sent.v1`, `duel.accepted.v1`, `duel.declined.v1`, `duel.expired.v1` |
| game | F014: `game_session.started.v1`, `game_session.ended.v1`, `game_session.replaced.v1`, `student.game_logged_in.v1`, `student.game_logged_out.v1` |
| learning | `content.published.v1`, `activity.submitted.v1`, `activity.graded.v1`, `assignment.created.v2` (F041, ADR-0095: target `grade`/`students`, `subject_id`, không `course_id`; dùng chung cho `assignment.published`; v1 đã xóa), `lesson.completed.v1` (F031), `curriculum.version_published.v1` (F021), `media.uploaded.v1`, `media.processed.v1`, `media.rejected.v1` (F027), `learning.recommendation_generated.v1` (F032), `assignment.results_released.v1`, `assignment.completed.v1`, `assignment.closed.v1`, `assignment.submission_updated.v1`, `assignment.reminded.v1`, `assignment.cancelled.v1` (F041) |

**Feature sở hữu:** mỗi file `payloads/<module>/<event_type>.v<N>.schema.json` do đúng một feature tạo — feature hiện thực use case phát event trong module producer (quy tắc ở [EVENT_DESIGN §4](../../docs/07-backend/EVENT_DESIGN.md#4-catalog-domain-events)); feature khác cùng phát hoặc tiêu thụ chỉ tham chiếu ("owner F<NNN>"), không tạo file và đề xuất delta tương thích qua owner. Tên file phải là một `event_type` có trong catalog. Bảng module:event → feature: `node scripts/specs/check-cross-features.mjs --list`.

Event do mọi module phát (`audit.recorded`, EVENT_DESIGN §4.1) có một schema duy nhất ở `payloads/common/audit.recorded.v1.schema.json` (owner F003), không đặt theo module.

Schema payload theo từng `event_type` khác được thêm dần khi service triển khai (Phase 1+). Schema mới có `examples` (một payload hợp lệ) làm mẫu fixture cho contract test consumer. Mọi `string` phải có `maxLength` (ADR-0041); enum học tập (`ActivityType`, `AttemptMode`, `FeedbackTiming`, `ResultRelease`, `LearningContentType`) `$ref` tới [`../schemas/common/enums.schema.json`](../schemas/common/enums.schema.json).

## Stream và subject (NATS JetStream)

| Stream | Subject | Producer | Lưu giữ | Ghi chú |
|---|---|---|---|---|
| `EVT_IDENTITY` | `ew.evt.identity.<event_type>` | module identity | 7 ngày | Tenant, user, SIS, membership, cleanup, settings, `audit.recorded` được tiêu thụ tại đây |
| `EVT_LEARNING` | `ew.evt.learning.<event_type>` | module learning | 7 ngày | Content, activity/attempt, exam, mastery, LMS |
| `EVT_GAME` | `ew.evt.game.<event_type>` | module game | 7 ngày | Game session, login, quest, XP, reward, Game Studio publish |
| `EVT_COMPETITION` | `ew.evt.competition.<event_type>` | module competition | 7 ngày | Party, match, rating, tournament |
| `EVT_CHAT` | `ew.evt.chat.<event_type>` | module chat | 7 ngày | Chat/moderation (không chứa nội dung tin) |
| `EVT_REALTIME` | `ew.evt.realtime.<event_type>` | binary `realtime` (gateway, F090) | 7 ngày | Chỉ `audit.recorded` (vd. `realtime.rate_limit_closed` khi đóng 4429), payload schema chung [`payloads/common/audit.recorded.v1.schema.json`](payloads/common/audit.recorded.v1.schema.json); gateway không có database/outbox nên publish trực tiếp (`Nats-Msg-Id` = `event_id`, at-most-once — chấp nhận cho audit vi phạm rate limit); gateway tạo stream khi khởi động |
| `CHAT_MESSAGES` | `ew.chat.<tenant_id>.msg.<channel_id>`, `ew.chat.<tenant_id>.evt.<channel_id>`, `ew.chat.<tenant_id>.cursor.<channel_id>` | module chat | 72 giờ | Đường ghi tin nhắn (ADR-0035) — không qua outbox |
| `DLQ_<SVC>` | `ew.dlq.<module>.<consumer>` | consumer của service nhận | 30 ngày | Event lỗi quá số lần |
| `IDENTITY_SIS_JOBS` | `ew.job.identity.sis.>` | module identity | Work-queue | Job đồng bộ SIS |

Subject **không** thuộc JetStream (NATS core, không lưu):

| Subject | Dùng cho |
|---|---|
| `ew.cmd.<module>.<type>` | Request/reply: `realtime` chuyển lệnh WebSocket tới handler của module sở hữu trong `core-api` (ADR-0074, bảng `x-routing` của AsyncAPI); header `Ew-Tenant-Id`, `Ew-User-Id`, `Ew-Game-Session-Id`, `Ew-Device-Id`, `Ew-Conn-Id`, `Ew-Node-Id`, `traceparent` do gateway gắn (`src/backend/pkg/realtime`); chỉ user NATS `realtime` được publish |
| `ew.cmd.game.session.heartbeat` | Gateway → module game mỗi `ws_heartbeat` khi kết nối mở, payload `{game_session_id}`; reply lỗi `CONFLICT`/`NOT_FOUND` → gateway đóng 4409 (F090-AC-12) |
| `ew.in.room.<room_id>` | Input tần suất cao (di chuyển hub, input co-op/battle không cần ack), fire-and-forget |

**Audit tập trung:** mọi service phát `audit.recorded` (trên stream của chính service đó); module `identity` tiêu thụ và ghi `audit.audit_logs`.

## Quy tắc

1. Producer ghi outbox **cùng transaction** với thay đổi nghiệp vụ; relay publish với `Nats-Msg-Id = event_id`.
2. Consumer durable đặt tên `<service_nhận>.<handler>`, xử lý at-least-once, idempotent qua `infra.processed_events`.
3. Thay đổi payload không tương thích → tăng `schema_version`, consumer hỗ trợ song song version cũ trong thời gian chuyển tiếp.
4. Validate envelope trong contract test: `ajv` draft 2020-12 strict (xem [specs/README](../README.md#kiểm-tra)).

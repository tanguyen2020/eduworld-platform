# EduWorld Event Specs

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: Backend Architect · Cập nhật: 2026-11-17 (payload schema M-C0/M-C1: `common/audit.recorded`, event identity của F010/F011/F013/F015/F016, event game của F014) · 2026-11-15
> Liên quan: [EVENT_DESIGN](../../docs/07-backend/EVENT_DESIGN.md) · [ADR-0042](../../docs/adr/ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) · [ADR-0035](../../docs/adr/ADR-0035-duong-truyen-chat-nats-jetstream-valkey-pub-sub-luu-postgres.md)

Hợp đồng máy đọc cho **domain event giữa các service**. Catalog nghiệp vụ (event nào, ai phát, ai nhận) nằm ở [EVENT_DESIGN §4](../../docs/07-backend/EVENT_DESIGN.md#4-catalog-domain-events). Thư mục này giữ schema để producer/consumer validate trong contract test.

## File

| File | Nội dung |
|---|---|
| `envelope.schema.json` | Envelope chuẩn của mọi domain event (EVENT_DESIGN §2): `event_id` (UUIDv7, cũng là `Nats-Msg-Id`), `event_type`, `schema_version`, `tenant_id`, `actor`, `subject`, `idempotency_key`, `payload` |
| [`../schemas/learning-event.schema.json`](../schemas/learning-event.schema.json) | Learning event (xAPI-like), mở rộng envelope với trường học tập bắt buộc |

Payload đã có schema:

| Service | File (`payloads/<service>/<event_type>.v<schema_version>.schema.json`) |
|---|---|
| common | `audit.recorded.v1` (mọi module phát, owner F003) |
| identity | `user_role.changed.v1`, `role.updated.v1`, `province.updated.v1`, `class.synced.v1`; F010: `tenant.created.v1`, `tenant.updated.v1`, `school.updated.v1`, `class.created.v1`, `class.updated.v1`, `enrollment.changed.v1`, `student.transferred.v1`, `tenant_membership.changed.v1`, `user.provisioned.v1`; F011: `school.synced.v1`, `student.synced.v1`, `student.provisioning_requested.v1`, `student.sis_link_created.v1`, `student.sis_link_superseded.v1`, `user.status_changed.v1`, `user.access_revoked.v1`, `sis.sync_completed.v1`, `sis.reconciliation_issue_raised.v1`; F013: `user.activated.v1`; F015: `platform_settings.changed.v1`; F016: `student.cleaned_up.v1`, `student.restored.v1` |
| game | F014: `game_session.started.v1`, `game_session.ended.v1`, `game_session.replaced.v1`, `student.game_logged_in.v1`, `student.game_logged_out.v1` |
| learning | `content.published.v1`, `activity.submitted.v1`, `activity.graded.v1`, `assignment.created.v1` (dùng chung cho `assignment.published`), `study_group.created.v1`, `study_group.updated.v1`, `study_group.member_changed.v1`, `study_group.deleted.v1` (ADR-0084), `lesson.completed.v1` (F031), `curriculum.version_published.v1` (F021), `media.uploaded.v1`, `media.processed.v1`, `media.rejected.v1` (F027), `learning.recommendation_generated.v1` (F032), `schedule_item.deleted.v1` (F040) |

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
| `CHAT_MESSAGES` | `ew.chat.<tenant_id>.msg.<channel_id>`, `ew.chat.<tenant_id>.evt.<channel_id>`, `ew.chat.<tenant_id>.cursor.<channel_id>` | module chat | 72 giờ | Đường ghi tin nhắn (ADR-0035) — không qua outbox |
| `DLQ_<SVC>` | `ew.dlq.<module>.<consumer>` | consumer của service nhận | 30 ngày | Event lỗi quá số lần |
| `IDENTITY_SIS_JOBS` | `ew.job.identity.sis.>` | module identity | Work-queue | Job đồng bộ SIS |

Subject **không** thuộc JetStream (NATS core, không lưu):

| Subject | Dùng cho |
|---|---|
| `ew.cmd.<service>.<domain>.<action>` | Request/reply: `realtime` chuyển lệnh WebSocket tới module sở hữu (ADR-0036) |
| `ew.in.room.<room_id>` | Input tần suất cao (di chuyển hub, input co-op/battle không cần ack), fire-and-forget |

**Audit tập trung:** mọi service phát `audit.recorded` (trên stream của chính service đó); module `identity` tiêu thụ và ghi `audit.audit_logs`.

## Quy tắc

1. Producer ghi outbox **cùng transaction** với thay đổi nghiệp vụ; relay publish với `Nats-Msg-Id = event_id`.
2. Consumer durable đặt tên `<service_nhận>.<handler>`, xử lý at-least-once, idempotent qua `infra.processed_events`.
3. Thay đổi payload không tương thích → tăng `schema_version`, consumer hỗ trợ song song version cũ trong thời gian chuyển tiếp.
4. Validate envelope trong contract test: `ajv` draft 2020-12 strict (xem [specs/README](../README.md#kiểm-tra)).

# EduWorld Event Specs

> Trạng thái: Draft · Phiên bản: 0.1 · Owner: Backend Architect · Cập nhật: 2026-11-15
> Liên quan: [EVENT_DESIGN](../../docs/07-backend/EVENT_DESIGN.md) · [ADR-0042](../../docs/adr/ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) · [ADR-0035](../../docs/adr/ADR-0035-duong-truyen-chat-nats-jetstream-valkey-pub-sub-luu-postgres.md)

Hợp đồng máy đọc cho **domain event giữa các service**. Catalog nghiệp vụ (event nào, ai phát, ai nhận) nằm ở [EVENT_DESIGN §4](../../docs/07-backend/EVENT_DESIGN.md#4-catalog-domain-events). Thư mục này giữ schema để producer/consumer validate trong contract test.

## File

| File | Nội dung |
|---|---|
| `envelope.schema.json` | Envelope chuẩn của mọi domain event (EVENT_DESIGN §2): `event_id` (UUIDv7, cũng là `Nats-Msg-Id`), `event_type`, `schema_version`, `tenant_id`, `actor`, `subject`, `idempotency_key`, `payload` |
| [`../schemas/learning-event.schema.json`](../schemas/learning-event.schema.json) | Learning event (xAPI-like), mở rộng envelope với trường học tập bắt buộc |

Đã có: `payloads/identity/user_role.changed.v1`, `role.updated.v1`, `province.updated.v1`, `class.synced.v1` (SPEC-3). Schema payload theo từng `event_type` khác được thêm dần dưới `payloads/<service>/<event_type>.v<schema_version>.schema.json` khi service triển khai (Phase 1+). Mọi `string` phải có `maxLength` (ADR-0041).

## Stream và subject (NATS JetStream)

| Stream | Subject | Producer | Lưu giữ | Ghi chú |
|---|---|---|---|---|
| `EVT_IDENTITY` | `ew.evt.identity.<event_type>` | identity-service | 7 ngày | Tenant, user, SIS, membership, cleanup, settings, `audit.recorded` được tiêu thụ tại đây |
| `EVT_LEARNING` | `ew.evt.learning.<event_type>` | learning-service | 7 ngày | Content, exercise, exam, mastery, LMS |
| `EVT_GAME` | `ew.evt.game.<event_type>` | game-service | 7 ngày | Game session, login, quest, XP, reward, Game Studio publish |
| `EVT_COMPETITION` | `ew.evt.competition.<event_type>` | competition-service | 7 ngày | Party, match, rating, tournament |
| `EVT_CHAT` | `ew.evt.chat.<event_type>` | chat-service | 7 ngày | Chat/moderation (không chứa nội dung tin) |
| `CHAT_MESSAGES` | `ew.chat.<tenant_id>.msg.<channel_id>`, `ew.chat.<tenant_id>.evt.<channel_id>`, `ew.chat.<tenant_id>.cursor.<channel_id>` | chat-service | 72 giờ | Đường ghi tin nhắn (ADR-0035) — không qua outbox |
| `DLQ_<SVC>` | `ew.dlq.<svc>.<consumer>` | consumer của service nhận | 30 ngày | Event lỗi quá số lần |
| `IDENTITY_SIS_JOBS` | `ew.job.identity.sis.>` | identity-service | Work-queue | Job đồng bộ SIS |

Subject **không** thuộc JetStream (NATS core, không lưu):

| Subject | Dùng cho |
|---|---|
| `ew.cmd.<service>.<domain>.<action>` | Request/reply: `realtime-gateway` chuyển lệnh WebSocket tới service sở hữu (ADR-0036) |
| `ew.in.room.<room_id>` | Input tần suất cao (di chuyển hub, input co-op/battle không cần ack), fire-and-forget |

**Audit tập trung:** mọi service phát `audit.recorded` (trên stream của chính service đó); `identity-service` tiêu thụ và ghi `audit.audit_logs`.

## Quy tắc

1. Producer ghi outbox **cùng transaction** với thay đổi nghiệp vụ; relay publish với `Nats-Msg-Id = event_id`.
2. Consumer durable đặt tên `<service_nhận>.<handler>`, xử lý at-least-once, idempotent qua `infra.processed_events`.
3. Thay đổi payload không tương thích → tăng `schema_version`, consumer hỗ trợ song song version cũ trong thời gian chuyển tiếp.
4. Validate envelope trong contract test: `ajv` draft 2020-12 strict (xem [specs/README](../README.md#kiểm-tra)).

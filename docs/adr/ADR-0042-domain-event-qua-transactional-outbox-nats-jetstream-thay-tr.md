# ADR-0042 — Domain event qua Transactional Outbox → NATS JetStream

> **Trạng thái:** Accepted · supersedes transport của [ADR-0019](ADR-0019-transactional-outbox-valkey-streams-cho-domain-events.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Microservices (ADR-0036) cần message bus bền vững giữa service; NATS đã có cho chat (ADR-0035).

## Quyết định

- Mỗi service ghi event vào bảng `outbox_events` trong **cùng transaction** với thay đổi nghiệp vụ (trong DB của service đó); relay publish lên NATS JetStream subject `ew.evt.<service>.<event_type>` với `Nats-Msg-Id = event_id`.
- Stream theo service (`EVT_IDENTITY`, `EVT_LEARNING`, `EVT_GAME`, `EVT_COMPETITION`, `EVT_CHAT`), giữ 7 ngày; DLQ `DLQ_<SVC>`; outbox giữ 30 ngày để replay. Work-queue riêng khi cần (vd `IDENTITY_SIS_JOBS`).
- Consumer là durable consumer của service nhận, at-least-once, idempotent nhờ `processed_events` trong DB của service nhận; lỗi lặp lại → DLQ + alert.
- Valkey **không** dùng Streams cho domain event.
- Envelope event theo EVENT_DESIGN (`event_id`, `event_type`, `tenant_id`, `schema_version`, `trace_id`…), schema trong `specs/events/`.

## Hệ quả

(+) Một message bus bền vững cho toàn hệ thống. (−) NATS là thành phần hạ tầng quan trọng, cần HA và giám sát.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted, supersede transport của ADR-0019 | Hệ quả ADR-0035/0036 |

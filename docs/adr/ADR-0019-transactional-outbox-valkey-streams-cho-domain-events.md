# ADR-0019 — Transactional Outbox + Valkey Streams cho domain events

> **Trạng thái:** Accepted · transport superseded by [ADR-0042](ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay.** Không còn hiệu lực: publish lên **Valkey Streams** (thay bằng NATS JetStream, ADR-0042). **Còn hiệu lực:** transactional outbox ghi cùng transaction nghiệp vụ, consumer at-least-once + idempotent bằng `processed_events`, learning events lưu bền vững trong PostgreSQL.

## Bối cảnh

Phản ứng chéo domain (cộng XP, cập nhật quest, mastery) cần đáng tin mà không dùng distributed transaction.

## Quyết định

- Ghi event vào bảng `outbox_events` trong **cùng transaction** với thay đổi nghiệp vụ; relay đọc outbox (`FOR UPDATE SKIP LOCKED`) và publish (transport hiện hành: NATS JetStream, ADR-0042).
- Consumer xử lý at-least-once, idempotent nhờ bảng `processed_events(consumer, event_id)`.
- Learning events (xAPI-like) lưu bền vững trong database của learning-service (LRS nội bộ), không chỉ nằm trên stream.
- *(Gốc, không còn hiệu lực)* Publish lên Valkey Stream `ew:events:<domain>`, giữ 7 ngày.

## Hệ quả

(+) Không mất event khi service lỗi giữa chừng. (−) Thêm relay và bảng outbox trong mỗi database.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Valkey Streams) | Phase 0 |
| 2026-11-15 | Transport đổi sang NATS JetStream | ADR-0042 |
| 2026-11-20 | Owner chốt (Accepted) | Owner |

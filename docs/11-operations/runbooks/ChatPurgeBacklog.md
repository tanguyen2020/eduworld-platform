# ChatPurgeBacklog

> Runbook của alert `ChatPurgeBacklog` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Gameplay chat purge is more than 2 days behind or failing_
- Điều kiện (MONITORING §5): `eduworld_chat_purge_backlog_oldest_seconds` > 2 ngày hoặc `eduworld_chat_purge_failures_total` tăng

## Tác động

Purge chat gameplay chậm > 2 ngày hoặc lỗi (ADR-0033).

## Chẩn đoán

1. Job `gameplay-chat-purge`, case mở đang hoãn purge

## Xử lý

1. Sửa job (role `chat_purger`); không xóa tay

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [CHAT_SYSTEM](../../03-social-multiplayer/CHAT_SYSTEM.md)
- undefined
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

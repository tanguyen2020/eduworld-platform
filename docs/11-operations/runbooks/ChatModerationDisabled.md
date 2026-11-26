# ChatModerationDisabled

> Runbook của alert `ChatModerationDisabled` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Moderation step <step> switched off (<tenant>)_
- Điều kiện (MONITORING §5): `eduworld_chat_policy_disabled == 1` cho bất kỳ tenant/bước nào

## Tác động

Một bước kiểm duyệt chat đang tắt (đã được hai System Admin duyệt, hết hạn tối đa 24 h).

## Chẩn đoán

1. `listModerationSwitches` — lý do, `expires_at`

## Xử lý

1. Xác nhận với System Admin; theo dõi bật lại tự động

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [CHAT_SYSTEM](../../03-social-multiplayer/CHAT_SYSTEM.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

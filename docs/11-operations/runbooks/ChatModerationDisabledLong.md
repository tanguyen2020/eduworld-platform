# ChatModerationDisabledLong

> Runbook của alert `ChatModerationDisabledLong` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Moderation step <step> off for more than its allowed time (<tenant>)_
- Điều kiện (MONITORING §5): `eduworld_chat_policy_disabled == 1` liên tục > 24 h, hoặc bước `pii` tắt > 4 h

## Tác động

Bước kiểm duyệt tắt quá lâu (> 24 h, hoặc `pii` > 4 h).

## Chẩn đoán

1. `listModerationSwitchHistory`

## Xử lý

1. Báo quản lý vận hành và Tenant Admin; đề xuất bật lại

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Safety Team lead.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [CHILD_SAFETY](../../10-security/CHILD_SAFETY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

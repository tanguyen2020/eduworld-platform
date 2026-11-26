# ModerationOncallGap

> Runbook của alert `ModerationOncallGap` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _No live on-call shift of role <oncall_role> covers now_
- Điều kiện (MONITORING §5): `min by (oncall_role) (eduworld_moderation_oncall_coverage) == 0` trong 5 phút (không có ca trực hiện hành của vai trò, F124)

## Tác động

Không có ca trực hiện hành cho một vai trò: case p1 sẽ gọi thẳng mức kế tiếp.

## Chẩn đoán

1. `listModerationOncallShifts?from=<now>&to=<now+1d>` — vai trò không có ca hiện hành (gauge `eduworld_moderation_oncall_coverage{oncall_role}` = 0)

## Xử lý

1. Trưởng ca tạo ca bù (`createModerationOncallShift`, người trực có `moderation.case.act`)
2. Ghi biên bản thiếu ca vào `docs/11-operations/alert-drills/`

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Trưởng ca Safety Team.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

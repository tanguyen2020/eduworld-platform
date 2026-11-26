# ModerationP1Unclaimed

> Runbook của alert `ModerationP1Unclaimed` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _A p1 moderation case has no assignee for more than 15 minutes (p1_unclaimed_alert_minutes)_
- Điều kiện (MONITORING §5): `max(eduworld_moderation_case_unclaimed_age_seconds{severity="p1"}) > 900` (case p1 chưa gán quá `p1_unclaimed_alert_minutes`, F121)

## Tác động

Case p1 (grooming, tự hại) chưa có người nhận > 15 phút (`p1_unclaimed_alert_minutes`).

## Chẩn đoán

1. Mở hàng đợi Safety Team: `listModerationCases?severity=p1&assignee=unassigned` (portal workspace E, F122)
2. Vòng trực (F124): case p1 mới đã gán `rotation` cho ca `primary`; job `oncall-escalator` gọi tiếp `secondary` (15 phút), trưởng ca (30 phút), System Admin (1 giờ) — kiểm `oncall_page_level` trên case

## Xử lý

1. Người trực nhận case cũ nhất (`assignModerationCase`); case p1 đã có hạn chế tạm thời người bị báo cáo (`case_hold`, F121-AC-04)
2. Case tự hại có `support_guide = child_protection_111`: làm theo CHILD_SAFETY §8–§9; gửi thông báo trường (`notifySchoolForCase`)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Quá 1 giờ: `moderation-sla-escalator` đã chuyển `escalated` — trưởng ca nhận; ghi biên bản vào `docs/11-operations/alert-drills/`.

## Liên quan

- [MODERATION](../../03-social-multiplayer/MODERATION.md)
- [CHILD_SAFETY](../../10-security/CHILD_SAFETY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

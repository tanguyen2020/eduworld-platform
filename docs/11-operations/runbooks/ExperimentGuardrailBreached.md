# ExperimentGuardrailBreached

> Runbook của alert `ExperimentGuardrailBreached` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/learning.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _An A/B learning experiment was stopped by its guardrail_
- Điều kiện (MONITORING §5): Một thí nghiệm A/B bị guardrail dừng (`increase(eduworld_experiment_guardrail_breaches_total[1d]) > 0`, F035)

## Tác động

Một thí nghiệm A/B bị dừng: nhánh làm giảm chỉ số chính; học sinh đã về nhánh đối chứng.

## Chẩn đoán

1. `getLearningExperimentResults`

## Xử lý

1. Đội EdTech xem kết quả và quyết định

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [LEARNING_EFFICACY_PLAN](../../02-learning/LEARNING_EFFICACY_PLAN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

# SyntheticProbeFailing

> Runbook của alert `SyntheticProbeFailing` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/synthetic.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Synthetic probe <probe> failed two cycles in a row_
- Điều kiện (MONITORING §5): Probe synthetic (`eduworld_synthetic_probe_success`, chu kỳ 5 phút) không thành công 2 chu kỳ liên tiếp (không mẫu thành công trong 11 phút; lượt `rate_limited` không tính)

## Tác động

Luồng chính của học sinh (client-config hoặc đăng nhập → tạo phiên → đăng xuất) hỏng trước khi trường báo.

## Chẩn đoán

1. Probe nào lỗi (`eduworld_synthetic_probe_success{probe}`), log service `synthetic-probe`
2. APISIX route, Keycloak, `core-api`

## Xử lý

1. Xử lý theo alert gốc đi kèm (`ServiceDown`, `LoginLatencyHigh`…)
2. Tenant QA bị khóa/dọn? kiểm tra `hs.qa.probe` (TEST_STRATEGY §4)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [TEST_STRATEGY](../../12-project/TEST_STRATEGY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

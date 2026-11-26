# SISSyncFailed

> Runbook của alert `SISSyncFailed` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Two SIS syncs in a row failed for <connection>_
- Điều kiện (MONITORING §5): 2 lần sync liên tiếp thất bại cho một connection

## Tác động

Hai lần sync SIS liên tiếp thất bại cho một connection.

## Chẩn đoán

1. `listSisSyncRuns` — mã lỗi của run
2. `eduworld_sis_contract_unknown_fields_total` (SIS đổi hợp đồng?)

## Xử lý

1. Sửa cấu hình/credential (secret `identity_sis_*`) hoặc chờ SIS của trường
2. Báo School Admin

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [SIS_INTEGRATION](../../09-identity/SIS_INTEGRATION.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

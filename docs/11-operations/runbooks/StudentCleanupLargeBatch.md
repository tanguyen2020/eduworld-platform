# StudentCleanupLargeBatch

> Runbook của alert `StudentCleanupLargeBatch` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _A student cleanup run waits for approval (more than 10% of a tenant)_
- Điều kiện (MONITORING §5): Một lần chạy dọn > 10% học sinh của tenant (`status = awaiting_approval`) — `increase(eduworld_student_cleanup_large_batch_total[1h]) > 0`

## Tác động

Một lần dọn học sinh không đăng nhập vượt 10% tenant: đang chờ duyệt.

## Chẩn đoán

1. Danh sách ứng viên (portal, System Admin)

## Xử lý

1. System Admin xem và xác nhận/hủy (ADR-0032)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [ACCOUNT_PROVISIONING](../../09-identity/ACCOUNT_PROVISIONING.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

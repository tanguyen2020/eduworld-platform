# StudentCleanupJobFailed

> Runbook của alert `StudentCleanupJobFailed` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _student-inactive-cleanup failed or has not succeeded for 48 hours_
- Điều kiện (MONITORING §5): Job lỗi hoặc không chạy thành công > 48 h

## Tác động

Học sinh không đăng nhập không được dọn đúng lịch (ADR-0032).

## Chẩn đoán

1. `eduworld_job_runs_total{job="student-inactive-cleanup"}`
2. Log `worker-jobs`

## Xử lý

1. Sửa lỗi và chạy lại job

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [ACCOUNT_PROVISIONING](../../09-identity/ACCOUNT_PROVISIONING.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

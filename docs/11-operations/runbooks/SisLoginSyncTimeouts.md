# SisLoginSyncTimeouts

> Runbook của alert `SisLoginSyncTimeouts` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _More than 5% of blocking login syncs time out_
- Điều kiện (MONITORING §5): `eduworld_sis_login_sync_total{branch="blocking",result="timeout"}` > 5% nhánh B trong 10 phút ([SIS_INTEGRATION §4.5](../09-identity/SIS_INTEGRATION.md))

## Tác động

Đăng nhập nhánh chặn (B) chờ SIS quá lâu.

## Chẩn đoán

1. SIS của trường nào chậm (`eduworld_sis_login_sync_duration_seconds`)
2. Circuit breaker theo connection

## Xử lý

1. Báo trường; breaker sẽ chuyển sang nhánh không chặn

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [SIS_INTEGRATION](../../09-identity/SIS_INTEGRATION.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

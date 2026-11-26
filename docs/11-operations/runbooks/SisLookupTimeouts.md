# SisLookupTimeouts

> Runbook của alert `SisLookupTimeouts` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _More than 20% of SIS lookups of <connection> time out_
- Điều kiện (MONITORING §5): `eduworld_sis_lookup_total{result="timeout"}` > 20% trong 15 phút theo connection

## Tác động

Nhận biết SIS nguồn khi đăng nhập lần đầu bị timeout.

## Chẩn đoán

1. `eduworld_sis_lookup_total` theo connection
2. Facade SIS và mạng tới SIS

## Xử lý

1. Báo trường; kiểm tra cấu hình connection

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [SIS_INTEGRATION](../../09-identity/SIS_INTEGRATION.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

# AuditTenantMismatch

> Runbook của alert `AuditTenantMismatch` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Audit events name a school of another tenant_
- Điều kiện (MONITORING §5): `increase(eduworld_audit_records_total{result="tenant_mismatch"}[1h])` > 0

## Tác động

Event audit nêu trường của tenant khác: dấu hiệu lỗi cô lập tenant ở module nguồn.

## Chẩn đoán

1. DLQ_IDENTITY và module nguồn (`service` của metric)
2. Event gốc (không xuất dữ liệu học sinh ra ngoài)

## Xử lý

1. Tạo ticket ưu tiên cao cho module nguồn; không replay event lỗi

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Báo Security lead (nghi rò rỉ liên tenant).

## Liên quan

- [MULTITENANCY](../../06-architecture/MULTITENANCY.md)
- [SECURITY_ARCHITECTURE](../../10-security/SECURITY_ARCHITECTURE.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

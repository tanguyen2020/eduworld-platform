# TenantFilterBypassSpike

> Runbook của alert `TenantFilterBypassSpike` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Tenant filter bypasses of module <module> above 5× the baseline_
- Điều kiện (MONITORING §5): `eduworld_tenant_filter_bypass_total` tăng > 5× baseline 1 h

## Tác động

Nghi lạm dụng quyền đọc liên tenant.

## Chẩn đoán

1. `eduworld_tenant_filter_bypass_total` theo `module`, `reason`
2. Audit `tenancy.Bypass` (ai, lý do)

## Xử lý

1. Xác minh với System Admin/Safety Team; thu hồi quyền nếu không hợp lệ

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Báo Security lead.

## Liên quan

- [MULTITENANCY](../../06-architecture/MULTITENANCY.md)
- [SECURITY_ARCHITECTURE](../../10-security/SECURITY_ARCHITECTURE.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

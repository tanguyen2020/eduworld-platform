# GameRefreshReuseSpike

> Runbook của alert `GameRefreshReuseSpike` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Refresh token reuse or device mismatch spike_
- Điều kiện (MONITORING §5): `rate(eduworld_auth_game_refresh_total{result=~"reused|device_mismatch"}[15m])` > 5× baseline hoặc > 20/giờ ở một tenant

## Tác động

Dấu hiệu token bị đánh cắp hoặc client refresh song song (lỗi bản client).

## Chẩn đoán

1. Audit theo `device_id` (`identity.login_events`), không xem dữ liệu cá nhân ngoài phạm vi điều tra
2. Phiên bản client mới phát hành? (single-flight refresh)

## Xử lý

1. Lỗi client: báo đội Godot, cân nhắc dừng rollout
2. Nghi lộ token: thu hồi phiên theo quy trình bảo mật (SECURITY_ARCHITECTURE)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Nghi tấn công: báo Security lead ngay.

## Liên quan

- [SECURITY_ARCHITECTURE](../../10-security/SECURITY_ARCHITECTURE.md)
- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

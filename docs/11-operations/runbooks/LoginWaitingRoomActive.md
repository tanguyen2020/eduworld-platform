# LoginWaitingRoomActive

> Runbook của alert `LoginWaitingRoomActive` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _The login waiting room has queued students for 10 minutes_
- Điều kiện (MONITORING §5): `eduworld_auth_waiting_room_queued` > 0 liên tục 10 phút

## Tác động

Học sinh phải chờ trong hàng đợi đăng nhập > 10 phút.

## Chẩn đoán

1. `eduworld_auth_waiting_room_queued`, tốc độ đăng nhập
2. Năng lực Keycloak

## Xử lý

1. Scale Keycloak/`core-api`; xem có cần nâng ngưỡng hàng chờ

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [KEYCLOAK_SSO](../../09-identity/KEYCLOAK_SSO.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)

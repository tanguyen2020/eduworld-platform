# Tenant QA của staging/production (F131 T039)

> Liên quan: [TEST_STRATEGY §4](../../../docs/12-project/TEST_STRATEGY.md) · [MONITORING §5](../../../docs/11-operations/MONITORING.md#5-alert-rules--confirmed) (`SyntheticProbeFailing`) · [ACCOUNT_PROVISIONING](../../../docs/09-identity/ACCOUNT_PROVISIONING.md) · [DATABASE_DESIGN](../../../docs/07-backend/DATABASE_DESIGN.md) (`tenants.is_qa`)

Probe synthetic (`synthetic-probe` của `obs.stack.yml`, `tests/synthetic/k6`) đăng nhập bằng học sinh `hs.qa.probe` của **tenant QA** `qa-probe` (`is_qa = true`). Tenant này chỉ chứa tài khoản synthetic, không học sinh thật, không nội dung học tập riêng; số liệu của nó bị loại khỏi analytics và dashboard nhà trường/liên tenant (anti-join `ref.tenants.is_qa`). Không có tệp dữ liệu nào ở đây: mật khẩu và danh sách thật là dữ liệu vận hành, không commit (AGENTS R4).

## Tạo (một lần mỗi môi trường, System Admin)

1. `createTenant` code `qa-probe`, `tenant_type = school`, Keycloak Organization alias `qa-probe`; đặt `is_qa = true` (chỉ System Admin; hiện `createTenant` chưa nhận `is_qa` — [Proposed] bổ sung tham số ở module identity, hoặc cập nhật bằng job vận hành có audit).
2. Tạo trường QA và import CSV một học sinh qua pipeline import (F011, `SisRun`): username `hs.qa.probe`, khối bất kỳ, không có thông tin cá nhân thật.
3. Đặt mật khẩu mạnh sinh ngẫu nhiên → lưu vào SOPS `deploy/secrets/<env>/ops.sops.yaml` khóa `ops_probe_password` → `make secrets-sync ENV=<env>`; `EW_PROBE_USERNAME`, `EW_PROBE_DEVICE_ID` (UUID cố định của "thiết bị" probe) trong `deploy/swarm/<env>/obs.env`.
4. Loại trừ khỏi job dọn học sinh không đăng nhập (`platform.cleanup_exclusions`, ADR-0032) — probe đăng nhập mỗi 5 phút nên không bao giờ đến hạn, loại trừ là lớp bảo vệ thứ hai.

## Giới hạn của probe (F131-AC-20)

- Chỉ `getClientConfig` và chuỗi `gameLogin` → `createGameSession` → `endGameSession` → `gameLogout`; không bao giờ bắt đầu attempt hay ghi dữ liệu học tập.
- `429 LOGIN_QUEUE_BUSY`/`RATE_LIMITED`: ghi `result="rate_limited"`, không thử lại trong lượt; lượt sau chờ đủ `Retry-After` (trạng thái trong textfile).
- Cùng một `device_id` mọi lượt, không `takeover` (không làm tăng `GameTakeoverSpike`).

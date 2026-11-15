# ADR-0009 — Apache APISIX làm API gateway, backend vẫn tự authorize

> **Trạng thái:** Proposed · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [API_DESIGN](../07-backend/API_DESIGN.md), [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Cần một cổng API thống nhất cho TLS, routing, rate limit, quan sát; nhưng authorization nghiệp vụ phải nằm ở backend.

## Quyết định

- APISIX (≥3.x) đảm nhiệm TLS termination, routing theo prefix tới từng service (ADR-0036), xác thực JWT (`openid-connect` bearer-only hoặc `jwt-auth` với JWKS), rate limit (`limit-req`, `limit-count` dùng Valkey), giới hạn kích thước body, CORS, `prometheus`, `opentelemetry`, access log, WebSocket proxy cho `/realtime/*`.
- **Quy ước route:**
  - `/api/v1/admin/*`, `/api/v1/sis/*`, `/api/v1/game-admin/*`: API quản trị cho người dùng platform (Admin Portal, ADR-0027) — **chỉ trên APISIX nội bộ/VPN**, yêu cầu MFA.
  - `/internal/*`: machine-to-machine (SIS webhook, Keycloak backchannel logout) — không qua gateway public.
  - `/internal/v1/*` giữa các service (ADR-0040) không đi qua APISIX.
  - Keycloak admin console chỉ trong mạng quản trị.
- APISIX **không** thay thế authorization nghiệp vụ: tenant/school/class/ownership/permission luôn kiểm tra trong Go.
- Local dev: APISIX standalone (`apisix.yaml`); staging/production: etcd + cấu hình khai báo trong Git.

## Hệ quả

(+) Một điểm vào, chính sách biên nhất quán. (−) Hai lớp kiểm tra token (gateway + service) cần đồng bộ cấu hình issuer/audience.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | Quy ước route nội bộ (`/api/v1/admin|sis`, `/internal/*`) | Vòng đồng bộ tài liệu |
| 2026-11-15 | Thêm `/game-admin/*` nội bộ + MFA; routing theo service | ADR-0027, ADR-0036 |

# Contracts: F131 — Monitoring, alerting, dashboard vận hành, runbook

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getClientConfig` | `GET /api/v1/game/client-config` | `specs/openapi/eduworld-api.v1.yaml` | Không xác thực (probe `client_config`) | Có sẵn |
| `gameLogin` | `POST /api/v1/auth/game/login` | `specs/openapi/eduworld-api.v1.yaml` | Học sinh QA (probe `student_login`) | Có sẵn |
| `createGameSession` | `POST /api/v1/game/sessions` | `specs/openapi/eduworld-api.v1.yaml` | `game_player` (học sinh QA) | Có sẵn |
| `gameLogout` | `POST /api/v1/auth/game/logout` | `specs/openapi/eduworld-api.v1.yaml` | Học sinh QA (dọn phiên sau probe) | Có sẵn |
| — | `/metrics` trên cổng quản trị mọi binary | Không thuộc OpenAPI (DEPLOYMENT §14) | Chỉ mạng nội bộ | Không áp dụng |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không gọi internal API | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không phát/tiêu thụ; chỉ scrape `prometheus-nats-exporter` | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 6. Delta cần thực hiện

- Không có delta contract trong `specs/`. Không probe portal (đăng nhập portal cần MFA tương tác); sức khỏe portal theo dõi qua SLO APISIX.
- Hợp đồng ngoài: định dạng rule Prometheus, cấu hình Alertmanager, JSON dashboard Grafana, SDK Sentry (GlitchTip).

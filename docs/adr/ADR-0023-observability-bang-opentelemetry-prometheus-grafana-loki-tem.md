# ADR-0023 — Observability bằng OpenTelemetry + Prometheus + Grafana + Loki + Tempo

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [MONITORING](../11-operations/MONITORING.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

6 service, gateway, NATS và game client cần quan sát xuyên suốt (log, metric, trace).

## Quyết định

- Go dùng `log/slog` (JSON) + OpenTelemetry SDK (trace + metrics), xuất qua OTel Collector tới **Prometheus** (metrics), **Tempo** (trace), **Loki** (log), hiển thị bằng **Grafana**. Trace lan truyền qua HTTP và NATS.
- APISIX bật plugin `prometheus` và `opentelemetry`.
- Error tracking Sentry-compatible (GlitchTip/Sentry tự host) [Confirmed].
- Godot gửi crash/perf telemetry tối giản, không chứa PII, qua `/api/v1/game/telemetry` có rate limit.
- Tên metric dùng tiền tố `eduworld_*`.

## Hệ quả

(+) Một ngăn xếp mã nguồn mở, tự host được tại Việt Nam. (−) Cần vận hành thêm các thành phần quan sát.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | Thống nhất tiền tố metric `eduworld_*` | Vòng đồng bộ tài liệu |
| 2026-11-20 | Owner chốt (Accepted) | Owner |

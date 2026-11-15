# ADR-0001 — Go Modular Monolith với 3 tiến trình triển khai

> **Trạng thái:** Superseded by [ADR-0036](ADR-0036-microservices-theo-he-thong-lon-thay-adr-0001.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Đã được thay bởi ADR-0036.** Toàn bộ quyết định "một Go module, 3 binary" **không còn hiệu lực**: backend là 6 microservice Go, mỗi service có database riêng (ADR-0036, ADR-0037). Nguyên tắc **còn giữ**: ranh giới domain rõ, không import nội bộ của domain khác, kiểm tra ranh giới bằng linter (nay áp dụng bên trong từng service theo ADR-0038). Nội dung dưới đây giữ để truy vết.

## Bối cảnh

Master Prompt gốc yêu cầu không chia microservices sớm nhưng phải giữ ranh giới domain để tách sau. Hệ thống có hai dạng tải khác nhau: request/response (REST) và kết nối lâu dài (WebSocket), cộng thêm job nền (SIS sync, outbox, matchmaking).

## Quyết định (gốc, không còn hiệu lực)

Một Go module duy nhất `backend/`, build ra 3 binary dùng chung code domain:

| Binary | Vai trò | Scale |
|---|---|---|
| `cmd/api` | REST API cho mọi domain | Stateless, scale ngang |
| `cmd/realtime` | WebSocket gateway: chat, notification, presence, party/lobby, matchmaking events, hub sync, battle room | Stateful theo kết nối, scale ngang, room có "owner node" |
| `cmd/worker` | Outbox relay, SIS sync, matchmaker loop, leaderboard/analytics aggregation, moderation async, scheduled jobs | Scale theo consumer group; job singleton dùng Valkey lock |

Mỗi module trong `internal/<module>` có cấu trúc hexagonal (`domain/`, `app/`, `adapters/`, `ports/`). Module chỉ gọi module khác qua interface công khai `internal/<module>/api.go` hoặc qua domain event; cấm import `domain`/`adapters` của module khác (kiểm tra bằng `depguard`/`go-arch-lint`).

## Hệ quả (gốc)

(+) Một codebase, một pipeline, transaction cục bộ dễ dàng; tách service sau chỉ cần thay adapter in-process bằng gRPC/HTTP. (−) Cần kỷ luật ranh giới; lỗi panic có thể ảnh hưởng nhiều domain trong cùng tiến trình.

## Phương án đã loại (gốc)

Microservices ngay từ đầu (chi phí vận hành, distributed transaction); một binary duy nhất (WebSocket và REST cạnh tranh tài nguyên).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) | Phase 0 |
| 2026-11-15 | Superseded bởi ADR-0036: chuyển sang 6 microservice | Stakeholder chọn microservices theo hệ thống lớn |

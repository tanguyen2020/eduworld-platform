# ADR-0058 — Game Analytics: tổng hợp trong PostgreSQL qua sự kiện NATS

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) · [Chỉ mục ADR](README.md)

> [!NOTE]
> **[ADR-0070](ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md)** (2026-11-15): không có ngoại lệ purge cho `analytics.events_raw`; hết hạn chỉ ẩn danh hóa, partition giữ lại; theo dõi dung lượng theo ngưỡng chuyển kho của ADR này.

## Bối cảnh

Cần phân tích hành vi chơi (DAU/WAU/MAU, retention, funnel quest, tham gia sự kiện) tách khỏi Learning Analytics và giám sát kỹ thuật (Master Prompt tái cấu trúc §23). Hạ tầng hiện chạy Docker, ưu tiên ít thành phần (ADR-0024).

## Phương án

1. Kho phân tích riêng (ClickHouse) ngay từ đầu.
2. **Bảng tổng hợp trong PostgreSQL, nạp từ domain event NATS** (đề xuất).
3. Dịch vụ analytics bên thứ ba (loại: dữ liệu trẻ em ra ngoài, tracking).

## Quyết định

- Nguồn dữ liệu: **domain event phía server** đã có trên NATS (`session`, `quest`, `challenge`, `progression`, `shop`, `party`, `match`, `event`…) và một tập **telemetry client tối thiểu** (vào/ra màn, FPS/hiệu năng, lỗi) gửi qua `/game/telemetry` không chứa PII.
- **Consumer analytics trong game-service** ghi vào schema `analytics` của `eduworld_game`: bảng sự kiện thô rút gọn (giữ 90 ngày) và bảng tổng hợp theo ngày/tuần/tenant/khối (giữ 2 năm) [Confirmed].
- Chỉ số định nghĩa một nơi (GAME_ANALYTICS); dashboard trong workspace Game Live Operations, truy vấn qua API admin của game-service.
- **Quyền xem [Confirmed]:** platform (live-ops, System Admin) xem toàn bộ; Tenant Admin xem số liệu tổng hợp của tenant mình; không có màn xem hành vi chi tiết từng học sinh ngoài mục đích hỗ trợ/an toàn có audit.
- Ngưỡng chuyển sang kho riêng (ClickHouse): khi bảng sự kiện thô > 500 triệu dòng hoặc truy vấn dashboard p95 > 5 giây → ADR mới.

## Hệ quả

(+) Không thêm hạ tầng; dữ liệu ở trong nước, trong hệ thống. (−) PostgreSQL chịu thêm tải ghi; cần partition theo ngày và job rollup.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Bổ sung theo audit |
| 2026-11-20 | Owner chốt (Accepted) | Owner |

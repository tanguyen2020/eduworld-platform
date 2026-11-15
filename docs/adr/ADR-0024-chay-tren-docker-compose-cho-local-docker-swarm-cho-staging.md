# ADR-0024 — Chạy trên Docker: Compose cho local, Docker Swarm cho staging/production, chưa dùng Kubernetes

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-10 · **Tài liệu:** [DEPLOYMENT](../11-operations/DEPLOYMENT.md), [LOCAL_DEVELOPMENT](../11-operations/LOCAL_DEVELOPMENT.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder xác nhận hệ thống chạy trên Docker, chưa dùng Kubernetes.

## Quyết định

1. **Local/dev:** Docker Compose (toàn bộ hạ tầng + 6 service, hoặc chạy hybrid một service bằng Go native).
2. **Staging/production:** Docker Engine ở **chế độ Swarm** trên VM (on-prem hoặc cloud tại Việt Nam theo OQ-10/OQ-12) [Proposed]. Mỗi môi trường mô tả bằng **stack file** (`deploy/swarm/<env>/*.stack.yml`, cú pháp Compose) trong Git:
   - Service có `replicas`, `update_config` (rolling update `start-first`, `failure_action: rollback`), `rollback_config`, `healthcheck`, `resources.limits`, `placement` theo nhãn node (`role=app|data|edge|realtime`).
   - **Overlay network** tách lớp: `edge` (APISIX), `app` (service, NATS), `data` (PostgreSQL, Valkey, object storage), `admin` (Keycloak admin, công cụ vận hành).
   - Service discovery bằng DNS nội bộ của Swarm, cân bằng tải qua VIP; APISIX trỏ upstream theo tên service.
   - Docker secrets/configs cho credential và cấu hình; không đưa secret vào image hay Git.
   - Ít nhất 3 manager node.
3. **Thành phần có trạng thái** (PostgreSQL, NATS JetStream, Valkey, Keycloak, object storage) ghim vào node cố định (placement + volume) với HA ở tầng ứng dụng (PostgreSQL primary/replica + Patroni hoặc tương đương, NATS 3 node, Valkey replica + Sentinel, Keycloak 3 node); có thể thay bằng dịch vụ managed [Proposed].
4. **Không autoscale theo tải (không KEDA/HPA):** chuẩn bị đủ năng lực cho giờ cao điểm; job lịch chạy `docker service scale` **theo lịch học**; alert khi CPU/độ trễ vượt ngưỡng để scale thủ công/bán tự động [Proposed].
5. **CI/CD:** build image theo service, deploy `docker stack deploy` theo service (tag bất biến); migrate Atlas chạy như one-off job trước khi cập nhật service; rollback bằng `docker service rollback`.
6. **Không dùng Kubernetes trong Release 1.0;** giữ khả năng chuyển sau (container bất biến, cấu hình qua env, `/livez` `/readyz`, không phụ thuộc tính năng riêng của Swarm trong code). Chuyển sang Kubernetes cần ADR mới.

## Hệ quả

(+) Vận hành đơn giản, cùng định dạng Compose từ local đến production. (−) Không autoscale theo tải thực, cần kế hoạch năng lực; HA thành phần có trạng thái tự quản lý.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Compose local, Kubernetes tùy chọn) | Phase 0 |
| 2026-11-15 | Accepted: Docker Swarm, chưa dùng Kubernetes | Stakeholder |

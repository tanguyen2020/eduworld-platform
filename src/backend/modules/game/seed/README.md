# Seed — module game

Dữ liệu giả cho local ([LOCAL_DEVELOPMENT §5](../../../../../docs/11-operations/LOCAL_DEVELOPMENT.md)). F001 chưa có seed; feature sở hữu bảng thêm seed ở đây. Không seed vào schema `ref` (projection nạp qua event).

## economy (F075)

DEV SEED: `economy/items.v1.json` (10 vật phẩm cosmetic, `tradable` luôn false), `shop-items.v1.json` (3 mặt hàng giá Xu; `wings_paper_01` chỉ gán `th-demo-a` qua `assignments.v1.json`), `collections.v1.json` (bộ knowledge card ẩn tới món đầu — `objective_id` là placeholder DEV, gắn LO thật khi có curriculum), `achievements.v1.json` (thành tựu đếm quest bậc đồng/bạc và một thành tựu ẩn). Publish qua `PublishGameContent` trong `cmd/seed`; tiền `xu` (cap 300/ngày) do migration `f075_economy` tạo.

## liveops (F077)

DEV SEED: `liveops/events.v1.json` (lễ hội `world_festival` platform khối 3–5, điểm sự kiện, 2 mốc, community goal phạm vi trường — lịch 2027 nên run ở trạng thái `scheduled`; ở local sửa `start_at`/`end_at` cách hiện tại vài phút để xem chuyển trạng thái), `templates.v1.json` (template "Tuần lễ Toán học" cho sự kiện trường, gán `th-demo-a` qua `assignments.v1.json`), `season-tracks.v1.json` (season track 30 cấp × 100 điểm, trần 100 điểm/ngày, của mùa `pre-season` do module competition seed — mở khi nhận `season.started`). Publish qua `PublishGameContent` trong `cmd/seed`.

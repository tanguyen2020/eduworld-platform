# Seed — module game

Dữ liệu giả cho local ([LOCAL_DEVELOPMENT §5](../../../../../docs/11-operations/LOCAL_DEVELOPMENT.md)). F001 chưa có seed; feature sở hữu bảng thêm seed ở đây. Không seed vào schema `ref` (projection nạp qua event).

## economy (F075)

DEV SEED: `economy/items.v1.json` (10 vật phẩm cosmetic, `tradable` luôn false), `shop-items.v1.json` (3 mặt hàng giá Xu; `wings_paper_01` chỉ gán `th-demo-a` qua `assignments.v1.json`), `collections.v1.json` (bộ knowledge card ẩn tới món đầu — `objective_id` là placeholder DEV, gắn LO thật khi có curriculum), `achievements.v1.json` (thành tựu đếm quest bậc đồng/bạc và một thành tựu ẩn). Publish qua `PublishGameContent` trong `cmd/seed`; tiền `xu` (cap 300/ngày) do migration `f075_economy` tạo.

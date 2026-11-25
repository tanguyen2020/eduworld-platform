# Contracts: F068 — Art pass 2: Chibi anime cartoon 3D cho Main Hub và nhân vật

> Feature chỉ đổi đồ họa phía client; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. `SparkTreeModel` nhận giá trị của enum `MasteryLevel` đã có (`learning.v1.yaml`) nhưng lượt này **không gọi** API nào (Câu hỏi mở F068-1: chưa có operation tổng mastery của khối trong trường).

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có. Emote của người chơi khác vẫn theo `hub.player.emoted` (F092); clip và biểu cảm chọn phía client từ `ItemVisual`.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ của client: `KidSkeleton` ↔ Skeleton3D của `AvatarRig.tscn` ↔ track clip ↔ BONES của mesh skinned (data-model.md §1); token `art.palette.*` (slot) ↔ UV mesh ↔ uniform `palette[64]`.

## 6. Delta cần thực hiện

- Không có delta contract. Delta token: `tokens@0.10.0` (`design/CHANGELOG.md`).

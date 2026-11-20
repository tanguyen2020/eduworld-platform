# ADR-0052 — Thực thi mô phỏng (Simulation Execution)

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Master Prompt "Re-architecture" §15.1 yêu cầu mô phỏng khoa học, vật lý, hình học 3D, phòng thí nghiệm ảo.

## Phương án

1. Chạy mô phỏng trên server, stream kết quả (tốn tài nguyên, độ trễ).
2. **Chạy trên client Godot, server kiểm chứng kết quả.**
3. Nhúng mô phỏng web bên thứ ba (khó kiểm soát nội dung, tracking).

## Quyết định (đề xuất phương án 2)

- Mô phỏng là **scene Godot đóng gói sẵn** (trong game build hoặc gói PCK có ký số trên CDN), **tham số hóa bằng dữ liệu** từ portal (activity type `simulation`, ADR-0048). Portal **không** nạp script/code tùy ý.
- Client ghi **tham số đầu vào + chuỗi hành động**; server kiểm chứng bằng **checker xác định** viết bằng Go cho từng loại mô phỏng (dùng cùng mô hình toán đơn giản hóa với scene), hoặc chỉ chấm câu hỏi gắn kèm khi không kiểm chứng được.
- Kết quả chỉ làm bằng chứng học tập khi qua kiểm chứng; không cấp thưởng theo kết quả client tự báo.
- Mô phỏng phải chạy được trên thiết bị chuẩn (ADR-0013) và Web export.

## Hệ quả

(+) Tương tác mượt, không tốn tài nguyên server, vẫn chống gian lận. (−) Mỗi loại mô phỏng cần checker Go và bộ vector kiểm thử chung với scene.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Audit theo Master Prompt mới |
| 2026-11-20 | Owner chốt (Accepted) | Owner |

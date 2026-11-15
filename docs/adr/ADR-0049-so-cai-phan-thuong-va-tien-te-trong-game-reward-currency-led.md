# ADR-0049 — Sổ cái phần thưởng và tiền tệ trong game (Reward & Currency Ledger)

> **Trạng thái:** Proposed · **Ngày:** 2026-11-15 · **OQ:** OQ-14 · **Tài liệu:** [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md), [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Kinh tế game phải kiểm toán được; cấp thưởng và thay đổi tiền tệ do server quyết định, idempotent (Master Prompt "Re-architecture" §11). Loại tiền tệ cụ thể chờ OQ-14.

## Phương án

1. Lưu số dư trực tiếp và cập nhật tại chỗ.
2. **Sổ cái append-only + số dư tính/snapshot.**
3. Double-entry kế toán đầy đủ (loại: quá nặng khi không có tiền thật).

## Quyết định (đề xuất phương án 2)

- Mọi thay đổi XP, tiền tệ, vật phẩm đi qua **ledger append-only** trong game-service (`xp_ledger`, `currency_ledger`, `item_ledger`: `entry_id`, `tenant_id`, `student_id`, `delta`, `reason_code`, `source_type`, `source_id`, `rule_id`, `idempotency_key` unique, `created_at`). Số dư cache trong `wallets`/`inventory_items` cập nhật cùng transaction, đối soát định kỳ.
- **Một loại tiền mềm "Xu"** kiếm bằng hoạt động trong game, dùng mua vật phẩm trang trí trong Shop. **Không** tiền thật, **không** loot box, **không** phần thưởng ngẫu nhiên trả phí. Loại tiền thứ hai chỉ khi OQ-14 trả lời.
- Không số dư âm; mua hàng kiểm tra số dư trong transaction có khóa hoặc `version`.
- Ledger không sửa/xóa (ADR-0028); sai sót sửa bằng bút toán bù có lý do và người duyệt (four-eyes).
- Phần thưởng không ảnh hưởng điểm học tập hay mastery (ADR-0018).

## Hệ quả

(+) Truy vết mọi phần thưởng, chống cộng trùng, khôi phục được số dư. (−) Ledger tăng nhanh, cần partition theo tháng.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Audit theo Master Prompt mới |

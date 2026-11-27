# Contracts: F125 — Student UX pass 7: phóng to giao diện trên điện thoại và nút đóng đỏ cho mọi cửa sổ

> Feature chỉ đổi hiển thị phía client (hệ số canvas, Theme, khung cửa sổ); không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Đóng cửa sổ (kể cả Nhóm) không gọi server.

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ client: token ↔ Theme/`UiTokens` (`make design-tokens-check`: generated, tương phản AA, semver `0.21.0`); `BandLayoutProbe.SIZES` ↔ `CompactPolicy` (`test_compact_policy.gd::test_matrix_sizes_follow_the_policy`).

## 6. Delta cần thực hiện

- Không có delta contract API. Delta design: `tokens@0.21.0` (MINOR — chỉ thêm token `size.control.60`, `size.layout.540`, `component.compact.logical_height`, `density.compact.{button.min_height,font.small}` và 4 Theme sinh `eduworld_theme_<band>_compact.tres`).

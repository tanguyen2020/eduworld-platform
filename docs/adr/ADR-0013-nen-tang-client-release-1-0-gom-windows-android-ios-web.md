# ADR-0013 — Nền tảng client: Release 1.0 gồm Windows, Android, iOS, Web

> **Trạng thái:** Accepted · phần ngân sách hiệu năng superseded by [ADR-0057](ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md) · Superseded một phần bởi [ADR-0101](ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) (bỏ Windows) · **Ngày:** 2026-11-15 · **OQ:** OQ-04 · **Tài liệu:** [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md), [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md), [WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0057](ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md).** Không còn hiệu lực: ngân sách hiệu năng (thiết bị chuẩn, FPS, triangles, draw call) và "Web preset đồ họa thấp" — nay theo hạng thiết bị tầm trung trong [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md). **Còn hiệu lực:** 4 nền tảng, thứ tự phát triển, một codebase GDScript.

> [!WARNING]
> **Superseded một phần bởi [ADR-0101](ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) (bỏ Windows, 2026-11-24).** Không còn hiệu lực: Windows 10+ trong danh sách nền tảng, thứ tự "Windows + Android từ Phase 1", ghi chú ADR-0078 mục 5 về Windows bên dưới. **Còn hiệu lực:** Release 1.0 trên **Web, Android, iOS** (Web: WebGL 2, Compatibility, single-threaded), Android từ Phase 1, Web trong CI từ Phase 2, iOS từ Phase 4, một codebase GDScript, khác biệt nền tảng nằm sau interface.

> [!NOTE]
> Bổ sung bởi ADR-0078 mục 5: trên Windows, game client mặc định coi là máy dùng chung (không lưu refresh token) trừ khi bộ cài chọn "máy cá nhân". ADR-0079 (thu hẹp nền tảng) bị Rejected; giữ đủ 4 nền tảng.

## Bối cảnh

Stakeholder yêu cầu game hoàn chỉnh (không chỉ MVP). Thiết bị ở trường đa dạng: phòng máy Windows, tablet Android, iPad, trình duyệt.

## Quyết định

- Release 1.0 hỗ trợ **4 nền tảng**: Windows 10+, Android 9+, iOS 15+, Web (WebGL 2, renderer Compatibility, export single-threaded để không phụ thuộc COOP/COEP).
- Thứ tự phát triển: Windows + Android từ Phase 1; build Web trong CI từ Phase 2 để đo tải/FPS; pipeline iOS (ký, TestFlight) từ Phase 4; Phase 6 tối ưu và phát hành cả 4.
- Hạng thiết bị, FPS, renderer, preset và ngân sách hiệu năng: theo ADR-0057 và [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) (thay ngân sách ban đầu của ADR này).
- Một codebase GDScript (ADR-0014); khác biệt nền tảng (đăng nhập, secure storage) nằm sau interface.

## Hệ quả

(+) Phủ mọi loại thiết bị ở trường. (−) Chi phí test thiết bị và ký ứng dụng iOS.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Windows + Android trước, Web/iOS sau) | Phase 0 |
| 2026-11-15 | Release 1.0 gồm 4 nền tảng | Stakeholder: game hoàn chỉnh |
| 2026-11-15 | Phần ngân sách hiệu năng (Android 3–4 GB, 30 FPS, ≤ 150k triangles, ≤ 100 draw call; Web preset thấp) chuyển sang ADR-0057/PERFORMANCE_BUDGET theo yêu cầu "chạy trên máy tầm trung" | Stakeholder (người duyệt) |
| 2026-11-17 | Ghi chú bổ sung từ ADR-0078 mục 5 (không đổi ý nghĩa quyết định) | Technical Architect |
| 2026-11-20 | Owner chốt (Accepted) | Owner |
| 2026-11-24 | Superseded một phần bởi ADR-0101: bỏ Windows, client chỉ Web, Android, iOS | Owner dự án |

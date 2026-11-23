# ADR-0099 — Đổi tên config type `levels` thành `progression-levels`

> **Trạng thái:** Accepted · **Ngày:** 2026-11-23 · **OQ:** — · **Tài liệu:** [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [API_DESIGN](../07-backend/API_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Hợp đồng `/api/v1/game-admin/{config_type}` có giá trị `levels` trong enum `GameConfigType` cho bảng level sinh từ đường cong XP. Tên này trùng nghĩa với gói level/khu chơi `game.level_definitions` (config type `level-packages`, F071), gây nhầm giữa "cấp độ XP" và "màn chơi". Backlog đối chiếu RB-308 và Clarifications Session 2026-11-17 của F074 (Technical Architect) đã chốt đổi tên, nhưng chưa có ADR nên `make specs-breaking` (danh sách ngoại lệ chỉ nhận thay đổi phá vỡ có ADR, ADR-0095 mục 7) chặn việc gỡ giá trị cũ. Các route `/game-admin/*` chưa được hiện thực (F078), chưa client nào gửi `levels`.

## Quyết định

Enum `GameConfigType` dùng `progression-levels` thay cho `levels` ở mọi path `/game-admin/{config_type}` và `POST /game-admin/preview/sessions` (spec gộp và `game.v1.yaml`), cùng các ánh xạ `x-portal` (module `game`, action `action.game_ops.progression.*`, permission `game.progression.write`). Giá trị `levels` bị gỡ ngay, không giữ alias: chưa có route hiện thực hay dữ liệu dùng nó. Tên cột của bảng là `game.progression_levels.xp_required` (PROGRESSION_SYSTEM §6). Gói level/khu chơi giữ config type `level-packages`.

## Hệ quả

(+) Một tên một nghĩa: `progression-levels` (bảng cấp độ XP) và `level-packages` (gói màn chơi) không còn trùng. (+) F078 hiện thực workflow `/game-admin/*` trên tên mới, không phải giữ alias.
(−) oasdiff báo `request-parameter-enum-value-removed`/`request-property-enum-value-removed` cho các operation `/game-admin/*`: chấp nhận qua `specs/breaking-allowlist.yaml` dẫn ADR này.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-23 | Tạo ADR (F074 T039, RB-308) | F074 |

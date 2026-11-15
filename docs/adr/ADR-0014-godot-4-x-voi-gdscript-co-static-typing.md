# ADR-0014 — Godot 4.x với GDScript có static typing

> **Trạng thái:** Proposed · **Ngày:** 2026-11-15 · **OQ:** OQ-04 · **Tài liệu:** [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Cần một ngôn ngữ chạy được trên mọi export target (Windows, Android, iOS, Web).

## Quyết định

- GDScript với **static typing bắt buộc** (`warnings/untyped_declaration = error`).
- Test bằng GdUnit4.
- Logic thuần (API client, state store, quest client logic) tách khỏi scene thành class `RefCounted` để test được.

## Hệ quả

(+) Hỗ trợ đủ export target (C# chưa ổn định trên Web/mobile ở mọi bản 4.x), vòng lặp phát triển nhanh. (−) Hiệu năng tính toán nặng thấp hơn C#/C++; phần nặng nằm ở server.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |

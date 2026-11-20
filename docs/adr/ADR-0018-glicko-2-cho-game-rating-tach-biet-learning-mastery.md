# ADR-0018 — Glicko-2 cho Game Rating, tách biệt Learning Mastery

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md), [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Không được dùng điểm thi đấu để đánh giá năng lực học tập, và không được gộp các loại điểm khác nhau.

## Quyết định

- Rating thi đấu dùng **Glicko-2** theo `(student, mode, subject, season)`; chỉ dùng cho ghép trận và xếp hạng thi đấu.
- Rating **không** hiển thị cho nhà trường như chỉ số năng lực, **không** ảnh hưởng mastery/điểm.
- Năm loại số liệu tách biệt: **Learning Score**, **Learning Mastery**, **Game Rating**, **Game XP/Level**, **Competition Points**.

## Hệ quả

(+) Ghép trận công bằng mà không lẫn với đánh giá học tập. (−) Người dùng cần UI giải thích rõ các loại điểm.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |
| 2026-11-20 | Owner chốt (Accepted) | Owner |

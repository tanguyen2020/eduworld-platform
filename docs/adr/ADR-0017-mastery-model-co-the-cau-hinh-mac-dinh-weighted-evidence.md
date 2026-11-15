# ADR-0017 — Mastery model có thể cấu hình, mặc định Weighted Evidence

> **Trạng thái:** Proposed · **Ngày:** 2026-11-15 · **OQ:** OQ-08 · **Tài liệu:** [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Điểm một lần làm bài không phản ánh mức nắm vững kiến thức; cần mô hình mastery có thể thay đổi khi có dữ liệu thật.

## Quyết định

- Mastery tính theo `(student, learning_objective)`, giá trị 0..1, cập nhật bằng EWMA có trọng số:
  - theo ngữ cảnh: exam 1.0, assignment 0.9, practice 0.7, retry_wrong 0.7, quest 0.6, minigame 0.6, competition 0.4;
  - theo độ khó; decay theo thời gian.
  - Tham số chi tiết và ví dụ tính trong LEARNING_ANALYTICS.
- Mức: `not_started`, `beginning` (< 0.4), `developing` (< 0.6), `proficient` (< 0.8), `mastered` (≥ 0.8 và ≥ 5 evidence trên ≥ 2 ngày khác nhau).
- Thuật toán là strategy cắm được (`MasteryModel` interface), tham số lưu trong cấu hình có phiên bản; có thể thay bằng BKT/IRT sau khi có dữ liệu.
- Điểm một lần làm bài **không** đồng nhất với mastery.

## Hệ quả

(+) Đánh giá năng lực ổn định hơn điểm đơn lẻ. (−) Tham số cần hiệu chỉnh bằng dữ liệu thật.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | Thêm trọng số `minigame`, `retry_wrong` | LEARNING_ANALYTICS |

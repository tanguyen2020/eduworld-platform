# ADR-0016 — Server-authoritative scoring; answer key không bao giờ rời server

> **Trạng thái:** Proposed · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [QUESTION_BANK](../02-learning/QUESTION_BANK.md), [ANTI_CHEAT](../10-security/ANTI_CHEAT.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Client (Godot) không đáng tin; điểm, phần thưởng và kết quả chính thức không được để client quyết định.

## Quyết định

- API giao câu hỏi/hoạt động cho client chỉ trả về **render payload** (không có đáp án, không có scoring rule).
- Client gửi câu trả lời thô; server chấm, lưu, trả về đúng/sai + giải thích theo chính sách của chế độ (luyện tập: ngay lập tức; kiểm tra: sau khi nộp/sau khi đóng đề).
- Điểm, XP, phần thưởng, mastery, rating đều do server tính.
- Thời gian làm bài dùng timestamp server (`started_at`, `deadline_at`).

## Hệ quả

(+) Chống gian lận và lộ đáp án. (−) Phản hồi đúng/sai cần một vòng gọi server.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |

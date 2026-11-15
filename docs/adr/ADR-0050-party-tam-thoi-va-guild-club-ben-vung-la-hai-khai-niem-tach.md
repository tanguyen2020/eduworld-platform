# ADR-0050 — Party (tạm thời) và Guild/Club (bền vững) là hai khái niệm tách biệt

> **Trạng thái:** Proposed · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md), [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Master Prompt "Re-architecture" §13.2–13.3 yêu cầu party tạm thời và guild/club bền vững.

## Quyết định (đề xuất)

- **Party** (competition-service): nhóm tạm thời để chơi cùng, 2–8 người (mặc định 4), tự giải tán; chat `party` bị purge (ADR-0033).
- **Guild/Club** (chat-service, domain Social): nhóm **bền vững** có loại (Study Group, School Club, Subject Club, Competition Team, Interest Group), vai trò (owner/officer/member), yêu cầu tham gia, mục tiêu và hoạt động nhóm, thành tích nhóm, channel chat bền vững (không purge), sự kiện câu lạc bộ.
- **Phạm vi:** guild/club chỉ trong **một tenant**; tạo bởi giáo viên hoặc học sinh nhưng cần giáo viên/tenant duyệt [Proposed]; tối đa 50 thành viên (cấu hình). Party có thể tạo từ thành viên một club.
- Team cố định và Study Group hiện có là các loại guild/club (không thêm khái niệm thứ ba).

## Hệ quả

(+) Rõ vòng đời và lưu trữ của từng loại nhóm. (−) Domain guild/club cần đặc tả chi tiết (lượt sau).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Audit theo Master Prompt mới |

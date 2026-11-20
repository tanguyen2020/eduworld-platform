# ADR-0050 — Party (tạm thời) và Guild/Club (bền vững) là hai khái niệm tách biệt

> **Trạng thái:** Proposed · một phần superseded by ADR-0084 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md), [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0084](ADR-0084-nhom-hoc-tap-thuoc-learning-service.md):** study group không còn là một loại club; study group là nhóm LMS của learning-service. **Còn hiệu lực:** party tạm thời tách biệt club bền vững; club do học sinh tạo thuộc chat-service.

## Bối cảnh

Master Prompt "Re-architecture" §13.2–13.3 yêu cầu party tạm thời và guild/club bền vững.

## Quyết định (đề xuất)

- **Party** (competition-service): nhóm tạm thời để chơi cùng, 2–8 người (mặc định 4), tự giải tán; chat `party` bị purge (ADR-0033).
- **Guild/Club** (chat-service, domain Social): nhóm **bền vững** có loại (Study Group, School Club, Subject Club, Competition Team, Interest Group), vai trò (owner/officer/member), yêu cầu tham gia, mục tiêu và hoạt động nhóm, thành tích nhóm, channel chat bền vững (không purge), sự kiện câu lạc bộ.
- **Phạm vi:** guild/club chỉ trong **một tenant**; tạo bởi nhà trường (`school_admin`/`academic_manager`) hoặc học sinh; club do học sinh tạo cần nhà trường duyệt [Confirmed]; tối đa 50 thành viên (cấu hình). Party có thể tạo từ thành viên một club.
- Team cố định và Study Group hiện có là các loại guild/club (không thêm khái niệm thứ ba).

## Hệ quả

(+) Rõ vòng đời và lưu trữ của từng loại nhóm. (−) Domain guild/club cần đặc tả chi tiết (lượt sau).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Audit theo Master Prompt mới |
| 2026-11-15 | Điều chỉnh theo ADR-0072 (bỏ vai trò giáo viên, bỏ hoạt động chấm tay khỏi Release) | Technical Architect |
| 2026-11-16 | Một phần superseded by ADR-0084 (study group) | Technical Architect |

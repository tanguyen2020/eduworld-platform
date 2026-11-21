# ADR-0084 — Nhóm học tập (study group) thuộc learning-service

> **Trạng thái:** Superseded by ADR-0095 · **Ngày:** 2026-11-16 · **OQ:** — · **Thay một phần:** ADR-0050 (study group là một loại club), ADR-0072 (mục 2, câu nhắc ADR-0050) · **Tài liệu:** [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) · [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Đã được thay toàn phần bởi [ADR-0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md)** (2026-11-21): LMS không còn nhóm học tập (study group); bảng `lms.study_groups`, `lms.study_group_members`, event `study_group.*` và channel chat `study_group` bị bỏ. **Còn hiệu lực:** không — club do học sinh tạo vẫn theo ADR-0050.

## Bối cảnh

ADR-0050 và DATABASE_DESIGN coi study group là một loại club (`social.clubs`) của chat-service. LMS cần study group từ mốc M-C3 làm đối tượng giao bài (`target_type = study_group`), trong khi chat-service chỉ có từ M4a; spec OpenAPI đã đặt `/lms/schools/{id}/study-groups` ở learning-service. Feature F040 bị chặn vì mâu thuẫn này.

## Quyết định

1. **Study group là nhóm LMS do learning-service sở hữu** (bảng `lms.study_groups`, `lms.study_group_members`; tenant + soft-delete). Người tạo/sửa: `school_admin`, `academic_manager` (ADR-0072). Dùng làm đối tượng giao bài và lọc báo cáo.
2. learning-service phát event `study_group.created`, `study_group.updated`, `study_group.member_changed`, `study_group.deleted` qua outbox.
3. Khi có channel bền vững (M5), chat-service tạo/cập nhật channel `study_group` bằng cách tiêu thụ các event trên; chat-service không sở hữu dữ liệu thành viên study group, chỉ giữ bản chiếu cho quyền vào channel.
4. **Club do học sinh tạo** (social club) vẫn thuộc chat-service theo ADR-0050; study group không còn là một loại club.

## Hệ quả

(+) LMS có study group từ M-C3, đúng thứ tự CMS trước; quyền giao bài kiểm tra trong cùng service. (−) Sửa DATABASE_DESIGN (chuyển bảng sang `eduworld_learning`), SOCIAL_SYSTEM, CHAT_SYSTEM, EVENT_DESIGN và feature F040.

## Phương án đã loại

chat-service sở hữu, learning-service làm facade: F040 phải chờ chat-service (M4a), phá thứ tự CMS trước.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-16 | Tạo ADR | Technical Architect |
| 2026-11-21 | Superseded by ADR-0095 (bỏ khái niệm lớp, nhóm học tập) | Owner |

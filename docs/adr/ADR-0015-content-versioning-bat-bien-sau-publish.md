# ADR-0015 — Content versioning bất biến sau Publish

> **Trạng thái:** Proposed · cách rollback superseded by [ADR-0043](ADR-0043-phat-hanh-noi-dung-draft-publish-con-tro-phien-ban-giu-phien.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay.** Không còn hiệu lực: "rollback = publish lại một version cũ thành version mới" → nay rollback **trỏ release pointer về version cũ**, không tạo version mới (ADR-0043). **Còn hiệu lực:** version đã publish bất biến; mọi attempt/learning event tham chiếu version.

## Bối cảnh

Nội dung được sửa liên tục nhưng lịch sử học tập phải truy vết đúng nội dung học sinh đã làm.

## Quyết định

- Mỗi đối tượng nội dung (Question, QuestionSet, Lesson, Activity, Assessment, Quest, MiniGameConfig, NPC Dialogue…) có bản ghi định danh (`*_id`) và nhiều phiên bản (`*_versions`).
- Version ở trạng thái `published` **bất biến**; sửa = tạo version mới (`draft → in_review → approved → published`).
- Mọi attempt/answer/learning event tham chiếu `content_version_id`.
- Archive không xóa.
- Phát hành, rollback, giữ phiên bản khi đang chơi và đẩy cache: theo ADR-0043.

## Hệ quả

(+) Lịch sử học tập không bị thay đổi khi sửa nội dung. (−) Nhiều version cùng tồn tại, cần quản lý snapshot.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | Cách rollback thay bởi release pointer | ADR-0043 |

# ADR-0043 — Phát hành nội dung: Draft/Publish, con trỏ phiên bản, giữ phiên bản khi đang chơi, rollback, đẩy cache

> **Trạng thái:** Accepted · supersedes cách rollback của [ADR-0015](ADR-0015-content-versioning-bat-bien-sau-publish.md) · một phần superseded by ADR-0072 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md), [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md).** Không còn hiệu lực: chủ thể "giáo viên" chọn "Luôn dùng bản mới nhất" cho bài giao — nay là người tạo bài giao của nhà trường (`school_admin`/`academic_manager`). **Còn hiệu lực:** toàn bộ mô hình phát hành.

## Bối cảnh

Stakeholder yêu cầu: nội dung sửa liên tục nên cần Draft/Publish; publish xong học sinh mới thấy bản mới; học sinh đang ở version cũ vẫn thấy nội dung cũ, vào màn mới mới thấy bản mới nhất; xem được từng version đã publish; rollback dùng lại version cũ; publish xong đưa lên cache.

## Quyết định

1. **Hai lớp: Version và Release.**
   - **Version** (`*_versions`): bất biến sau khi publish; Draft sửa tự do; sửa nội dung đã publish = tạo Draft mới từ version đó.
   - **Release pointer** (`content_items.current_version_id`): version học sinh nhận khi **bắt đầu** lượt mới. Mỗi lần đổi con trỏ ghi `content_releases` (`item_id`, `from_version`, `to_version`, `action ∈ {publish, rollback, unpublish}`, `reason`, `released_by`, `released_at`), có audit.
   - Áp dụng cho mọi nội dung có version: lesson, question, question set, activity, rubric, assessment blueprint, learning path (learning-service); quest, mini-game config, NPC dialogue, level package, challenge, boss, reward rule, item, progression curve (game-service); sticker/GIF pack (chat-service).
2. **Workflow:** `draft → in_review → approved → published`; publish ngay hoặc **hẹn giờ** (`publish_at`) [Confirmed]; unpublish = gỡ con trỏ, không xóa version.
3. **Lịch sử version:** Admin Portal liệt kê mọi version, xem chỉ đọc, so sánh hai version (diff), preview trong game.
4. **Rollback = trỏ con trỏ về một version đã publish trước đó** (không sao chép, không tạo version mới); bắt buộc lý do, quyền riêng, audit; có thể roll forward. Học sinh đang chơi không bị ngắt.
5. **Giữ phiên bản khi đang chơi (version pinning):**
   - Khi học sinh **bắt đầu** đơn vị chơi/học (level, quest, mini-game, bài học, attempt, challenge, trận đấu), server ghi version vào đơn vị đó (`quest_progress.quest_version_id`, `attempts.source_version_id`, `level_sessions.content_versions`, `matches.question_plan_version`…). Mọi lần tải nội dung trong đơn vị dùng version đã ghi; kết thúc thì lượt sau nhận bản mới nhất.
   - Quest nhiều ngày giữ version đến khi hoàn thành hoặc bỏ [Confirmed].
   - **Đề kiểm tra:** ExamSession ghi version blueprint lúc tạo; cả lớp cùng đề.
   - **Bài giao:** mặc định giữ version lúc giao; giáo viên có thể chọn "Luôn dùng bản mới nhất" [Confirmed].
   - **Game client:** khi khởi động và **khi vào màn mới**, tải **content manifest** của tenant (item → version hiện hành, `manifest_version`, ETag); trong một màn không đổi nội dung; cache cục bộ theo `(item_id, version)`.
6. **Đẩy cache khi publish:**
   - Bước 1 (trước khi đổi con trỏ): dựng **snapshot** bất biến của version (render payload không có answer key, cấu hình, gói level, danh sách media), ghi Valkey `ew:<svc>:cv:<item_type>:<item_id>:<version>` (TTL trượt 7 ngày [Confirmed]) và CDN cho gói nội dung game không nhạy cảm (đường dẫn có version/hash, `Cache-Control: immutable`).
   - Bước 2: đổi con trỏ trong transaction + outbox `content.published` / `content.rolled_back`.
   - Bước 3: consumer cập nhật con trỏ cache `ew:<svc>:current:<tenant_id>:<item_type>:<item_id>` và manifest của các tenant liên quan (sở hữu hoặc được gán, ADR-0029), phát `content.manifest_updated` (áp dụng ở lượt sau).
   - Từ bấm Publish đến khi học sinh mới nhận được ≤ 30 giây [Confirmed].
   - **Câu hỏi đề kiểm tra và mọi answer key không lên CDN**; chỉ phục vụ qua API có kiểm quyền.
7. **Sửa đồng thời:** Draft dùng optimistic concurrency (`version`, ADR-0039).

## Hệ quả

(+) Biên tập liên tục không gián đoạn học sinh; rollback tức thì; tải nhanh nhờ snapshot bất biến. (−) Nhiều version cùng lúc trong các lượt đang chạy, phải giữ snapshot mọi version còn tham chiếu; thêm worker dựng snapshot/manifest.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted, supersede cách rollback của ADR-0015 | Stakeholder |
| 2026-11-15 | Mở rộng phạm vi sang activity, rubric, challenge, boss, progression curve | ADR-0048, PROGRESSION_SYSTEM |
| 2026-11-15 | Một phần superseded by ADR-0072 (bỏ vai trò giáo viên) | Technical Architect |

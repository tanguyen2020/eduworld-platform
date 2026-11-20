# ADR-0056 — Game song ngữ Việt – Anh

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-15 · **Tài liệu:** [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder (người duyệt sản phẩm) xác nhận: **game dùng song ngữ**, đồng thời duyệt bộ tên thế giới, chương truyện và NPC đang đề xuất trong WORLD_DESIGN. Trước đây chỉ có vi-VN mặc định và i18n "sẵn sàng" (OQ-15).

## Quyết định

1. **Ngôn ngữ hỗ trợ trong Release 1.0 [Confirmed]:** tiếng Việt (`vi-VN`, mặc định) và tiếng Anh (`en-US`).
2. **Phạm vi bắt buộc song ngữ [Confirmed]:** toàn bộ chuỗi UI game, HUD, menu, thông báo hệ thống, thông điệp lỗi hiển thị cho học sinh, tutorial, hội thoại NPC, văn bản quest/challenge/boss, tên và mô tả vùng đất/khu vực/NPC/vật phẩm/thành tựu/danh hiệu/sự kiện, chương truyện.
3. **Nội dung học tập [Confirmed]:** mỗi version nội dung có `content_language` (`vi` hoặc `en`). Môn Tiếng Anh soạn bằng tiếng Anh, hướng dẫn có thể kèm tiếng Việt. Các môn khác soạn tiếng Việt; bản dịch tiếng Anh là **tùy chọn** (cho trường song ngữ). Khi học sinh chọn tiếng Anh mà nội dung chưa có bản dịch thì hiển thị ngôn ngữ gốc kèm nhãn "Chưa có bản tiếng Anh". Attempt ghi lại ngôn ngữ đã hiển thị.
4. **Chọn ngôn ngữ:** tenant đặt ngôn ngữ mặc định; học sinh đổi trong Cài đặt (lưu theo hồ sơ, áp dụng ngay không cần khởi động lại). Có chế độ **hiển thị song song** cho tên riêng và từ vựng chủ chốt (ví dụ "Vương quốc Số · Math Kingdom") để hỗ trợ học tiếng Anh [Confirmed].
5. **Kỹ thuật:**
   - Godot: chuỗi UI dùng `TranslationServer` với file gettext `.po` (`godot/locale/vi.po`, `en.po`), mọi `Label`/`Button` dùng khóa dịch, không hardcode chuỗi.
   - Dữ liệu từ backend: trường hiển thị dùng kiểu `i18nText` (`{"vi": "...", "en": "..."}`) trong JSON Schema/OpenAPI; API nhận `Accept-Language`, trả cả hai ngôn ngữ cho dữ liệu cache dài (manifest, snapshot nội dung game) để đổi ngôn ngữ không cần tải lại.
   - Admin Portal: biên tập hai ngôn ngữ cạnh nhau; kiểm tra **đủ bản dịch** trước khi publish (bắt buộc `vi` và `en` cho nội dung thuộc phạm vi mục 2; tùy chọn cho nội dung học tập). Giao diện portal mặc định `vi`, có `en` [Confirmed].
   - Font hỗ trợ đầy đủ tiếng Việt có dấu và Latin; bố cục chịu được chuỗi dài hơn 35% so với bản ngắn nhất.
   - Đọc to (TTS) cho lớp 1–3 hỗ trợ cả hai ngôn ngữ nếu dùng [Confirmed].
6. **Không dịch máy chat** giữa người chơi (bảo vệ riêng tư, tránh sai lệch).
7. **Bộ tên thế giới [Confirmed]:** 8 vùng đất, 7 chương truyện "Thắp lại Tia Sáng", NPC chính theo WORLD_DESIGN, mỗi tên có cặp Việt – Anh.

## Hệ quả

(+) Hỗ trợ trường song ngữ và mục tiêu học tiếng Anh. Tên thế giới đã chốt nên đội art/nội dung bắt đầu sản xuất được. (−) Chi phí dịch và kiểm duyệt bản dịch cho mọi nội dung game; CMS phải chặn publish khi thiếu bản dịch bắt buộc; kiểm thử UI ở cả hai ngôn ngữ.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR; duyệt bộ tên thế giới | Stakeholder (người duyệt) |

# ADR-0055 — Công thức Toán, Vật lý, Hóa học: MathJax 4 (hiển thị) + MathLive (nhập trên Admin Portal)

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [QUESTION_BANK](../02-learning/QUESTION_BANK.md), [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder chỉ định công thức dùng **MathJax 4 + MathLive**; **trên game chỉ hiển thị**, không nhập công thức. MathJax/MathLive chạy trên trình duyệt; Godot native không có JavaScript/DOM.

## Quyết định

1. **Lưu trữ:** công thức là **LaTeX** (≤ 2.000 ký tự, ADR-0041), hóa học dùng **mhchem** (`\ce{...}`, `\pu{...}`); rich text đánh dấu `\( ... \)` (inline) và `\[ ... \]` (block).
2. **Nhập chỉ ở Admin Portal:** MathLive (`<math-field>`, bàn phím ảo toán/hóa) trong mọi editor có công thức.
3. **Hiển thị trên portal:** MathJax 4 (extension `mhchem`) cho preview, danh sách, School Learning Dashboard, xem bài làm.
4. **Render SVG lúc soạn, không có worker thường trực:** khi lưu/publish, portal dùng MathJax 4 trên trình duyệt render mỗi công thức thành **SVG** + **speech text**, gửi kèm nội dung. learning-service **kiểm tra allowlist LaTeX và sanitize SVG** (chỉ phần tử/thuộc tính an toàn, ≤ 256 KB, độ sâu lồng ≤ 20), lưu `math/<sha256>.svg` trên object storage/CDN; thiếu/không hợp lệ → `422 MATH_RENDER_FAILED`. Import hàng loạt đi qua màn Import của portal; CLI dùng cùng MathJax 4 trong CI là [Confirmed].
5. **Game chỉ hiển thị:** render payload chứa tham chiếu SVG + speech text; Godot nạp SVG (`Image.load_svg_from_string`, scale theo DPI), chèn vào `RichTextLabel`, màu theo Theme game, cache theo hash. **Không có bàn phím toán trong game.**
6. **Câu trả lời của học sinh không phải công thức:** chọn đáp án (đáp án có thể là công thức), đúng/sai, nhập số (có dung sai), điền khuyết số/từ, ghép cặp, sắp xếp, kéo thả. Học sinh tự gõ biểu thức toán không có trong Release 1.0 [Confirmed].
7. **An toàn:** LaTeX không tin cậy; tắt `\href`, `\url`, `\html…`, `\class`, `\style`, `\require`, `\unicode` tùy ý; extension trong allowlist (`ams`, `mhchem`, `color` giới hạn); SVG luôn sanitize phía server.
8. **Trợ năng:** portal dùng đọc công thức của MathJax 4; game dùng speech text sinh sẵn.
9. **Phiên bản:** ghim MathJax 4.x và MathLive trong `package.json` của portal; nâng cấp chạy lại visual regression bộ công thức mẫu; nội dung đã publish không render lại (SVG bất biến theo version, ADR-0043).

## Hệ quả

(+) Không thêm service/worker; công thức nhất quán giữa portal và game; game nhẹ. (−) SVG phụ thuộc bước render trên trình duyệt người soạn nên server phải kiểm tra chặt; học sinh không gõ được biểu thức trong game.

## Phương án đã loại

Math worker Node.js phía server (không cần khi game chỉ hiển thị); bàn phím toán trong Godot; engine công thức tự viết trong Godot; WebView trong game native; KaTeX.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted: MathJax 4 + MathLive, math worker + bàn phím toán Godot | Stakeholder |
| 2026-11-15 | Game chỉ hiển thị: bỏ math worker và bàn phím toán, render SVG trên trình duyệt | Stakeholder |
| 2026-11-15 | Thống nhất giới hạn SVG 256 KB, độ sâu lồng 20 | Đồng bộ spec |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |

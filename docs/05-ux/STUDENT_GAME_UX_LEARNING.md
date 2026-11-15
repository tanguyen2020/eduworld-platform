# Student Game UX — Màn học tập

> Trạng thái: Draft · Phiên bản: 0.1 · Owner: UX/UI Designer + EdTech Specialist · Cập nhật: 2026-11-15
> Liên quan: [STUDENT_GAME_UX](STUDENT_GAME_UX.md) (quy ước chung, màn game) · [DESIGN_SYSTEM §5](DESIGN_SYSTEM.md#5-game-design-system-godot-3d) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) · [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md) · [WRITTEN_RESPONSE_RUBRIC](../02-learning/WRITTEN_RESPONSE_RUBRIC.md) · [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md) · [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) · [ADR-0055](../adr/ADR-0055-cong-thuc-toan-vat-ly-hoa-hoc-mathjax-4-hien-thi-mathlive-nh.md) · [ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md) · [ADR-0057](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md) · [ADR-0059](../adr/ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md)

**Mục lục:** 1. Mục đích và phạm vi · 2. Quy ước áp dụng · 3. Learning activity player (khung chung) · 4. Exercise / question player · 5. Video player · 6. Ghi âm bài nói · 7. Tự luận và dự án · 8. Mô phỏng · 9. Bài kiểm tra có giờ · 10. Kết quả học tập · 11. Khóa dịch đề xuất · 12. Acceptance criteria

## 1. Mục đích và phạm vi

Đặc tả **hành động được** (không chỉ mô tả hình ảnh) cho 8 màn học tập học sinh dùng trong game, mỗi màn có **bố cục Web/PC** và **bố cục Mobile**. Tài liệu chỉ quy định UX; luật chấm, số lượt thử, cỡ chữ đề bài, quy tắc hoàn thành… do tài liệu nghiệp vụ sở hữu và được link tới, không chép lại giá trị.

Ngoài phạm vi: các màn game (Hub, world map, quest journal, inventory, party, chat, arena…) — xem [STUDENT_GAME_UX](STUDENT_GAME_UX.md); giao diện Admin Portal — xem [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md).

## 2. Quy ước áp dụng

Quy ước chung (nền tảng, hướng màn hình, vùng chạm, phím tắt, safe area, mật độ) do [STUDENT_GAME_UX](STUDENT_GAME_UX.md) sở hữu. Tóm tắt điểm dùng trong tài liệu này:

| Hạng mục | Quy ước |
|---|---|
| Hướng màn hình | Ngang trên mọi nền tảng |
| Web/PC | ≥ 1280×720, tham chiếu 1920×1080; chuột + bàn phím + phím tắt |
| Mobile | Điện thoại ngang ≥ 6" (tham chiếu 2400×1080) và tablet ngang (tham chiếu 2160×1620); cảm ứng; tôn trọng safe area (tai thỏ, thanh điều hướng). Game trên trình duyệt di động không hỗ trợ trong Release 1.0 |
| Vùng chạm | ≥ 48 dp; lớp 1–3 ≥ 64 dp |
| Cỡ chữ đề bài | Theo [LEARNING_EXPERIENCE §8](../02-learning/LEARNING_EXPERIENCE.md#8-ux-học-tập-theo-grade-band) |
| Song ngữ | Mọi chuỗi UI qua khóa `ui.<screen>.<element>` (gettext, [ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md)); nội dung học tập hiển thị theo `content_language` của version; thiếu bản dịch khi học sinh chọn tiếng Anh → hiển thị bản gốc + nhãn `ui.learning.no_translation` ("Chưa có bản tiếng Anh" / "English version not available") |
| API | Một họ attempt duy nhất ([ADR-0059](../adr/ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md)): start `POST /api/v1/learning/activities/{activity_id}/attempts`; sau đó `/api/v1/learning/activity-attempts/{attempt_id}` + `/responses`, `/review`, `/progress`, `/written-response`, `/recordings`, `/submissions`, `/simulation-runs`, `:submit`. Bài kiểm tra có giờ dùng API của [EXAM_ENGINE §11](../02-learning/EXAM_ENGINE.md#11-api-tóm-tắt) |
| Hiệu năng | Ngân sách theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) và [ADR-0057](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md); quy tắc riêng cho màn học tập ở §2.1 |

### 2.1. Quy tắc hiệu năng chung cho màn học tập (máy tầm trung)

| # | Quy tắc |
|---|---|
| P-1 | Khi mở bất kỳ màn học tập toàn màn (§3–§10), thế giới 3D phía sau **dừng cập nhật vật lý/AI/animation không cần thiết** và render ở độ phân giải thấp (hoặc chụp một khung làm nền mờ tĩnh) để dành GPU/CPU cho UI và video. Người chơi khác trong instance vẫn được đồng bộ ở mức tối thiểu (không render) |
| P-2 | Công thức hiển thị bằng **SVG render sẵn** cache theo hash (`user://cache/media/<sha256>`); rasterize một lần theo DPI hiện tại, không rasterize lại mỗi frame ([ADR-0055](../adr/ADR-0055-cong-thuc-toan-vat-ly-hoa-hoc-mathjax-4-hien-thi-mathlive-nh.md)) |
| P-3 | Video Ogg Theora chọn độ phân giải theo preset chất lượng: Thấp/Web 480p, Trung bình/Cao 720p ([VIDEO_LEARNING §2](../02-learning/VIDEO_LEARNING.md#2-định-dạng-và-phát)) |
| P-4 | Preload media của câu hỏi kế tiếp trong lúc học sinh làm câu hiện tại; không tải cả bài một lúc trên mobile |
| P-5 | Hiệu ứng phản hồi (hạt sáng, NPC) dùng sprite/AnimatedSprite 2D, không thêm ánh sáng 3D; tổng thời lượng ≤ 1,5 s và tắt được |
| P-6 | Không quá một panel Control lớn được vẽ lại mỗi frame; đồng hồ chỉ cập nhật 1 lần/giây |

## 3. Learning activity player (khung chung)

**Mục đích.** Khung bao quanh mọi `activity_type` ([ACTIVITY_ENGINE §12](../02-learning/ACTIVITY_ENGINE.md#12-renderer-và-editor-registry)); renderer của từng loại vẽ vào vùng nội dung. Đảm bảo mọi hoạt động có cùng cách thoát, tạm dừng, xem tiến độ và nhận phản hồi.

**Người dùng & khối.** Mọi học sinh lớp 1–9.

**Điểm vào / ra.** Vào từ: bước quest do NPC giao, phase boss/challenge, mini-game trong khu học tập, Practice Zone, "Ôn câu sai", bài giáo viên giao (quest journal / Bưu tá Nhím). Ra: hoàn thành → màn Kết quả (§10) hoặc quay lại cảnh game với phản hồi NPC; thoát giữa chừng → về cảnh game, attempt `paused` (resume theo [ACTIVITY_ENGINE §6.3](../02-learning/ACTIVITY_ENGINE.md#63-resume)).

### 3.1. Bố cục Web/PC (1920×1080)

```text
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ [◀ Thoát]  Giúp Ông Số sửa cầu · Toán lớp 3        ●●●○○ 3/5      ⏳ 04:12   [⏸] [⚙] │ header 72px
├──────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│                      VÙNG NỘI DUNG (renderer theo activity_type)                     │
│                      rộng tối đa 1280px, căn giữa                                    │
│                                                                                      │
│                                                                                      │
├────────────┬─────────────────────────────────────────────────────────────────────────┤
│ [NPC avatar]│ "Gần đúng rồi! Thử nhìn lại phép nhân nhé."   [💡 Gợi ý] [🔊 Đọc to]    │ dải phản hồi
├────────────┴─────────────────────────────────────────────────────────────────────────┤
│                                             [Bỏ qua*]          [ Kiểm tra / Tiếp ▶ ] │ action bar 88px
└──────────────────────────────────────────────────────────────────────────────────────┘
* chỉ hiện khi mode cho phép bỏ qua
```

### 3.2. Bố cục Mobile (điện thoại ngang 2400×1080)

```text
┌──────────────────────────────────────────────────────────────────────┐
│[◀] Sửa cầu · Toán 3          ●●●○○ 3/5    ⏳4:12   [⏸]              │ header 56dp (thu gọn tên)
├──────────────────────────────────────────────────────────┬───────────┤
│                                                          │ [💡]      │
│          VÙNG NỘI DUNG                                   │ [🔊]      │ cột công cụ phải
│          (cuộn dọc nếu dài)                              │           │ (ngón cái phải)
│                                                          │ [Kiểm tra]│ nút chính 64dp
│                                                          │           │
├──────────────────────────────────────────────────────────┴───────────┤
│ 🙂 "Gần đúng rồi! Thử nhìn lại phép nhân nhé."   (tự ẩn sau 4 s)      │ phản hồi trượt lên
└──────────────────────────────────────────────────────────────────────┘
```

Tablet ngang: dùng bố cục Web/PC thu tỉ lệ (vùng nội dung rộng tối đa 85% màn), nút chính ở dưới phải. Bàn phím ảo mở lên → vùng nội dung tự cuộn để ô nhập không bị che.

### 3.3. Thành phần

| Thành phần | Mô tả |
|---|---|
| Header | Nút thoát, tên hoạt động (song ngữ theo nội dung game), chỉ báo tiến độ (chấm/thanh theo khối), đồng hồ (dạng theo [LEARNING_EXPERIENCE §8](../02-learning/LEARNING_EXPERIENCE.md#8-ux-học-tập-theo-grade-band)), tạm dừng, cài đặt nhanh (âm lượng, đọc to, cỡ chữ) |
| Vùng nội dung | Scene renderer đăng ký trong registry; nhận render payload, phát tín hiệu `response_ready` |
| Dải phản hồi | NPC + lời nhắn theo [LEARNING_EXPERIENCE §4.1](../02-learning/LEARNING_EXPERIENCE.md#41-cấu-trúc-phản-hồi); nút gợi ý nhiều cấp, đọc to |
| Action bar | Nút chính một hành động duy nhất tại một thời điểm (Kiểm tra → Tiếp → Hoàn thành) |
| Hộp xác nhận thoát | "Em muốn tạm dừng? Tiến độ được giữ lại." — Tiếp tục / Tạm dừng |

### 3.4. Dữ liệu & API

| Hành động | API |
|---|---|
| Bắt đầu / resume | `POST /learning/activities/{activity_id}/attempts` (Idempotency-Key; gọi lại khi đã có attempt `in_progress` → trả attempt cũ) |
| Trạng thái, thời gian còn lại | `GET /learning/activity-attempts/{attempt_id}` (`server_time`, `deadline_at`) |
| Gửi phản hồi | `POST /learning/activity-attempts/{attempt_id}/responses` |
| Tiến độ (video, mô phỏng) | `POST /learning/activity-attempts/{attempt_id}/progress` |
| Nộp | `POST /learning/activity-attempts/{attempt_id}:submit` (Idempotency-Key) |
| Xem lại sau nộp | `GET /learning/activity-attempts/{attempt_id}/review` |

### 3.5. Trạng thái

| Trạng thái | Hiển thị |
|---|---|
| Đang tải | Khung giữ nguyên header, vùng nội dung skeleton + NPC "Đang chuẩn bị…"; > 8 s hiện nút Thử lại |
| Lỗi tải nội dung | `ui.activity_player.error_load` + Thử lại / Quay lại; không mất tiến độ |
| Mất mạng | Banner vàng "Mất kết nối — câu trả lời sẽ được gửi khi có mạng"; nút Kiểm tra chuyển sang chờ; resume tự động khi có mạng (trong resume window) |
| Hết giờ (mode có deadline) | Khóa nhập, thông báo "Hết giờ rồi! Bài đã được nộp." → Kết quả |
| Chờ giáo viên chấm | (`pending_review`) Thông báo "Bài đã gửi cho thầy/cô" + phần thưởng hoàn thành (nếu có) → quay lại game |
| Bị trả lại (`returned`) | Mở lại đúng chỗ, hiển thị nhận xét giáo viên ở dải phản hồi |
| Hoạt động chưa hỗ trợ trên client | (renderer không có) "Cần cập nhật game để làm hoạt động này" + nút cập nhật |
| Thành công | Hiệu ứng ngắn → §10 hoặc quay lại cảnh game |

### 3.6. Tương tác & phản hồi

- Phím tắt PC: `Enter` = nút chính, `Esc` = mở hộp thoát, `H` = gợi ý, `R` = đọc to, `1–6` = chọn đáp án (§4), `Tab` duyệt phần tử.
- Mobile: nút chính luôn ở vùng ngón cái phải; vuốt xuống trên header không đóng màn (tránh thoát nhầm).
- Phản hồi đúng/sai, số lượt thử, gợi ý theo [LEARNING_EXPERIENCE §4](../02-learning/LEARNING_EXPERIENCE.md#4-phản-hồi-đúngsai-thân-thiện) và [ACTIVITY_ENGINE §8](../02-learning/ACTIVITY_ENGINE.md#8-phản-hồi-trong-game). Không bao giờ hiện đáp án trước khi nộp với `exam` hoặc `assignment` có `after_due`.

### 3.7. Khác biệt grade band

| | `g1_3` | `g4_5` | `g6_7`, `g8_9` |
|---|---|---|---|
| Tiến độ | Chấm tròn to + nhân vật nhỏ chạy theo | Thanh + số câu | Số câu "3/10" |
| Đồng hồ | Ẩn hoặc "cát chảy" | Thanh thời gian | Phút / phút:giây |
| Đọc to | Mặc định bật | Tùy chọn | Tùy chọn |
| Nút | ≥ 64 dp, có icon + chữ | ≥ 48 dp | ≥ 48 dp |

### 3.8. Trợ năng & an toàn

- Đọc to đề và lựa chọn (audio ghi sẵn hoặc TTS) cho lớp 1–3; điều chỉnh cỡ chữ +20%; chế độ tương phản cao; không dùng chỉ màu để báo đúng/sai (kèm icon và chữ).
- Không hiển thị tên thật hay điểm của học sinh khác trong màn học tập.

## 4. Exercise / question player

**Mục đích.** Renderer cho câu hỏi từ Question Bank (quiz, bài tập tương tác, question set, ôn câu sai, câu chèn video, câu trong boss/challenge). Template chung cho 11 dạng ([QUESTION_BANK](../02-learning/QUESTION_BANK.md)), skin theo grade band.

**Điểm vào / ra.** Bên trong khung §3 (hoặc khung trận/boss); ra khi trả lời xong câu cuối.

### 4.1. Bố cục Web/PC — câu chọn đáp án (single/multiple choice, đúng/sai, câu hình ảnh)

```text
┌──────────────────────────── vùng nội dung (≤ 1280px) ───────────────────────────────┐
│  Câu 3                                                         [🔊]                   │
│  Bạn Nam có 3 túi, mỗi túi 6 quả cam. Nam có tất cả bao nhiêu quả?                   │
│  ┌──────────── hình minh họa (tùy chọn, ≤ 40% chiều cao) ─────────────┐               │
│  └─────────────────────────────────────────────────────────────────────┘               │
│  ┌──────────────────────┐  ┌──────────────────────┐                                   │
│  │ (1)  9 quả           │  │ (2)  18 quả          │   lưới 2×2 (≤ 4 lựa chọn)         │
│  └──────────────────────┘  └──────────────────────┘   hoặc 1 cột (lựa chọn dài,      │
│  ┌──────────────────────┐  ┌──────────────────────┐    có công thức)                  │
│  │ (3)  12 quả          │  │ (4)  3 + 6           │                                   │
│  └──────────────────────┘  └──────────────────────┘                                   │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### 4.2. Bố cục Mobile — câu chọn đáp án

```text
┌────────────── vùng nội dung ──────────────────┬───────────┐
│ Câu 3                                   [🔊]  │ [💡]      │
│ Bạn Nam có 3 túi, mỗi túi 6 quả cam…          │ [🔊]      │
│ [hình nhỏ – chạm để phóng to]                 │           │
│ ┌──────────────┐ ┌──────────────┐             │           │
│ │  9 quả       │ │  18 quả      │  thẻ ≥ 64dp │ [Kiểm tra]│
│ └──────────────┘ └──────────────┘             │           │
│ ┌──────────────┐ ┌──────────────┐             │           │
│ │  12 quả      │ │  3 + 6       │             │           │
│ └──────────────┘ └──────────────┘             │           │
└───────────────────────────────────────────────┴───────────┘
```

Đề dài vượt nửa chiều cao: đề thu gọn còn 3 dòng + "Xem đầy đủ" mở overlay; lựa chọn luôn hiển thị không cần cuộn.

### 4.3. Bố cục theo nhóm dạng câu hỏi

| Nhóm | Web/PC | Mobile | Nhập liệu |
|---|---|---|---|
| Chọn một / nhiều, đúng/sai, câu hình ảnh (chọn trên ảnh) | §4.1; nhiều đáp án có checkbox + nhãn "Chọn tất cả đáp án đúng"; đúng/sai là 2 nút lớn | §4.2; đúng/sai 2 nút chiếm toàn chiều rộng | Chuột/phím `1–6`; chạm |
| Điền khuyết (từ/số) | Câu văn với ô trống inline; ô đang chọn được tô viền | Câu văn ở trên, ô nhập ở dưới cùng + bàn phím; điền lần lượt từng ô | Bàn phím; mobile dùng bàn phím hệ thống (chữ) hoặc bàn phím số tùy biến (số) |
| Nhập số | Ô nhập lớn + đơn vị bên phải; bàn phím số tùy biến hiện ở dưới ô (có `,` thập phân, `−`, xóa) | Bàn phím số tùy biến chiếm nửa phải màn, đề bên trái | Bàn phím số tùy biến (không mở bàn phím hệ thống) |
| Ghép cặp | Hai cột thẻ; kéo thẻ trái sang phải hoặc chạm trái rồi chạm phải; nối bằng đường màu | Hai cột hẹp; chỉ chạm-chạm (kéo dễ nhầm trên màn nhỏ) | Chuột kéo / chạm |
| Sắp xếp | Danh sách thẻ dọc có tay nắm kéo; phím `↑ ↓` di chuyển thẻ đang chọn | Thẻ dọc; giữ để kéo; nút ▲▼ trên thẻ cho lớp 1–3 | Kéo / nút |
| Kéo thả (vào vùng/nhãn ảnh) | Ảnh hoặc sơ đồ ở giữa, khay thẻ ở dưới; vùng thả sáng lên khi kéo gần | Ảnh bên trái (phóng to được), khay thẻ cột phải | Kéo; mobile có "chạm thẻ → chạm vùng" thay thế |
| Tương tác (widget riêng) | Widget đầy đủ trong vùng nội dung | Widget cùng logic, bố cục ngang tối giản | Theo widget |

### 4.4. Công thức

- Đề, lựa chọn, gợi ý, giải thích có công thức → SVG inline trong `RichTextLabel`, cao theo dòng, tô màu theo Theme game ([ADR-0055](../adr/ADR-0055-cong-thuc-toan-vat-ly-hoa-hoc-mathjax-4-hien-thi-mathlive-nh.md)); công thức block căn giữa, cho chạm để phóng to trên mobile.
- Học sinh **không nhập** công thức: dạng trả lời dùng được ở [QUESTION_BANK §3.6.1](../02-learning/QUESTION_BANK.md#361-câu-hỏi-có-công-thức-adr-0055).
- Speech text của công thức dùng cho đọc to.

### 4.5. Dữ liệu & API, trạng thái

- Mỗi lần "Kiểm tra" → `POST …/responses` `{position, response, client_elapsed_ms}`; phản hồi theo mode. Lượt thử lại theo [LEARNING_EXPERIENCE §4.2](../02-learning/LEARNING_EXPERIENCE.md#42-số-lượt-thử-proposed).
- Trạng thái riêng: chưa chọn đáp án → nút chính mờ + gợi ý "Chọn một đáp án"; đã khóa câu (hết lượt) → hiển thị đáp án đúng + giải thích (khi mode cho phép); gửi lỗi → giữ lựa chọn, nút "Gửi lại".

### 4.6. Khác biệt grade band

`g1_3`: tối đa 3–4 lựa chọn có hình, ưu tiên chạm/kéo, không gõ chữ; `g4_5`: thêm điền khuyết, sắp xếp, nhập số với bàn phím số; `g6_9`: đủ 11 dạng, bàn phím đầy đủ ([LEARNING_EXPERIENCE §8](../02-learning/LEARNING_EXPERIENCE.md#8-ux-học-tập-theo-grade-band)).

## 5. Video player

**Mục đích.** Phát bài học video có chương, phụ đề song ngữ, câu hỏi chèn; tính hoàn thành theo phần đã xem thật ([VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md)).

**Điểm vào / ra.** Vào từ bước quest "xem bài", bài giao, thư viện khu học tập. Ra khi hoàn thành (§10) hoặc thoát (resume vị trí đã xem).

### 5.1. Bố cục Web/PC

```text
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ [◀ Thoát]  Phân số là gì? · Toán lớp 4                         Đã xem 62%   [⚙]     │
├────────────────────────────────────────────────────────────────┬─────────────────────┤
│                                                                │ Chương              │
│                     KHUNG VIDEO 16:9 (1280×720)                │ ✓ 1. Chia bánh      │
│                                                                │ ▶ 2. Tử và mẫu      │
│        ┌───────── phụ đề (2 dòng, nền mờ) ─────────┐           │ 🔒 3. So sánh       │
│        │ Mẫu số cho biết chia thành mấy phần bằng nhau│         │ (chưa xem: khóa tua)│
│        └──────────────────────────────────────────────┘        │                     │
├────────────────────────────────────────────────────────────────┤ Câu hỏi: ◆ ◆ ◇     │
│ [▶/⏸] [⟲10s]  ━━━━━━━━━●━━━━━━━━━━◆━━━━━━━  04:12 / 07:30     │                     │
│ thanh: phần đã xem tô đậm, ◆ = câu chèn          [CC vi|en|Tắt] [🔊] [⛶]             │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### 5.2. Bố cục Mobile

```text
┌──────────────────────────────────────────────────────────────────────┐
│[◀] Phân số là gì?                                   62%   [≡ Chương] │ header tự ẩn khi phát
│                                                                      │
│                 VIDEO TOÀN CHIỀU NGANG (letterbox)                    │
│          phụ đề 2 dòng                                               │
│ [▶] [⟲10s] ━━━━━━●━━━━◆━━━━  4:12/7:30    [CC] [🔊]                    │ điều khiển tự ẩn sau 3 s
└──────────────────────────────────────────────────────────────────────┘
```

Danh sách chương mở dạng ngăn kéo từ phải. Tablet: như Web/PC, cột chương thu hẹp.

### 5.3. Câu hỏi chèn

Tới mốc `at_ms`: video tạm dừng, khung video mờ, câu hỏi hiện ở overlay giữa màn (dùng renderer §4); `required` → không có nút bỏ qua; sau khi trả lời → phản hồi ngắn → tiếp tục phát (hoặc tua lại đoạn liên quan nếu sai và được cấu hình) theo [VIDEO_LEARNING §5](../02-learning/VIDEO_LEARNING.md#5-câu-hỏi-chèn-embedded-question). Mobile: overlay chiếm toàn màn, nút chính ở phải.

### 5.4. Dữ liệu & API, trạng thái

| Hành động | API |
|---|---|
| Bắt đầu | `POST /learning/activities/{activity_id}/attempts` → URL video, chương, danh sách cue (không có đáp án), phụ đề |
| Heartbeat tiến độ (10 s) | `POST …/progress` |
| Trả lời câu chèn | `POST …/responses` `{cue_id, response}` |
| Hoàn thành | `POST …:submit` |

| Trạng thái | Hiển thị |
|---|---|
| Video chưa sẵn sàng (`VIDEO_NOT_READY`) | "Video đang được chuẩn bị, em quay lại sau nhé" |
| Đệm/mạng chậm | Vòng tải trên khung; tự hạ 720p → 480p sau 2 lần đệm > 3 s |
| Tua vượt phần đã xem | Thanh không cho kéo qua vùng chưa xem; toast `ui.video.seek_locked` |
| Còn câu chèn bắt buộc khi hết video | Tự quay lại câu chưa trả lời |
| Hoàn thành | "Hoàn thành!" + điểm câu chèn → §10 |

### 5.5. Song ngữ, trợ năng, hiệu năng

- Phụ đề WebVTT `vi`/`en`; nút CC chọn vi/en/tắt; mặc định theo ngôn ngữ giao diện; môn Tiếng Anh mặc định phụ đề `en`, cho bật song song 2 dòng vi+en [Proposed].
- Phụ đề vẽ bằng Label theo Theme (cỡ ≥ cỡ chữ cơ sở của khối, nền mờ).
- Hiệu năng: P-1, P-3; dừng render thế giới 3D khi phát video; giải mã Theora trên thiết bị tầm trung ở 720p@30, 480p cho Web và preset Thấp.

## 6. Ghi âm bài nói

**Mục đích.** Học sinh ghi âm bài đọc to, nhắc lại, trả lời bằng lời; **không có nhận dạng giọng nói hay chấm phát âm tự động** trong Release 1.0 ([ADR-0059](../adr/ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md)); giáo viên nghe và chấm tay theo rubric.

**Điểm vào / ra.** Vào từ hoạt động `speaking` (thường ở Rừng Ngôn Ngữ hoặc bài giao). Ra sau khi gửi → trạng thái chờ giáo viên chấm.

### 6.1. Bố cục Web/PC

```text
┌──────────────────────────── vùng nội dung ───────────────────────────────────────────┐
│  Đọc to đoạn văn sau:                                                [🔊 Nghe mẫu]   │
│  ┌──────────────────────────────────────────────────────────────────────────────┐    │
│  │ "My name is Lan. I live in Ha Noi. I like reading books."                    │    │
│  └──────────────────────────────────────────────────────────────────────────────┘    │
│                                                                                      │
│                     ( ● )  Giữ hoặc bấm để ghi   00:12 / 00:30                       │
│                     ▁▃▅▇▅▃▁  sóng âm khi đang ghi                                     │
│                                                                                      │
│  Bản ghi của em:  [▶ Nghe lại]   Lần ghi: 2/3   [↺ Ghi lại]                           │
└──────────────────────────────────────────────────────────────────────────────────────┘
                                                         action bar: [ Gửi cho thầy/cô ▶ ]
```

### 6.2. Bố cục Mobile

```text
┌──────────────────────────────────────────────────┬───────────────┐
│ Đọc to đoạn văn sau:              [🔊 Nghe mẫu]  │               │
│ "My name is Lan. I live in Ha Noi…"              │    ( ● )      │ nút ghi 96dp
│ (cuộn nếu dài)                                   │   00:12/00:30 │
│ Bản ghi: [▶] Lần 2/3 [↺]                         │ [Gửi ▶]       │
└──────────────────────────────────────────────────┴───────────────┘
```

### 6.3. Luồng và trạng thái

| Trạng thái | Hiển thị / hành vi |
|---|---|
| Chưa có quyền micro | Màn giải thích thân thiện "Để ghi giọng đọc, game cần dùng micro" → xin quyền hệ điều hành; từ chối → hướng dẫn bật lại trong cài đặt hoặc lựa chọn thay thế (nếu giáo viên cho phép) |
| Sẵn sàng | Nút ghi; nghe mẫu (nếu `repeat_after`) |
| Đang ghi | Biểu tượng đỏ nhấp nháy + chữ "Đang ghi âm" (luôn hiển thị), đếm thời gian, tự dừng ở `max_record_seconds` ([AUDIO_LANGUAGE_LEARNING §3.2](../02-learning/AUDIO_LANGUAGE_LEARNING.md#33-ghi-âm-trên-godot)) |
| Đã ghi | Nghe lại, Ghi lại (tối đa `max_retakes`, hết lượt → nút mờ), Gửi |
| Đang tải lên | Thanh tiến độ; mất mạng → giữ bản ghi local, tự thử lại; không cho thoát mà mất bản ghi khi chưa xác nhận |
| Đã gửi, chờ chấm | "Thầy/cô sẽ nghe và nhận xét" + phần thưởng hoàn thành (nếu quest cho phép) |
| Có kết quả | Thông báo qua Bưu tá Nhím; mở §10 xem nhận xét + rubric |

API: `POST /learning/activities/{id}/attempts` → `POST …/recordings:presign` → upload PUT → `POST …/recordings/{recording_id}:complete` → `POST …:submit`. Không có `speech-feedback`.

Khác biệt khối: lớp 1–3 dùng "chạm để ghi, chạm để dừng" (không giữ), nút 96 dp, đọc to đề mặc định; lớp 6–9 có thể "giữ để ghi". Song ngữ: chỉ dẫn và nhãn theo ngôn ngữ UI, văn bản cần đọc theo `content_language`. An toàn: chỉ ghi khi học sinh bấm; không ghi nền; tệp lưu bucket riêng, không công khai ([AUDIO_LANGUAGE_LEARNING §4](../02-learning/AUDIO_LANGUAGE_LEARNING.md#4-bảo-vệ-dữ-liệu-giọng-nói-trẻ-em)). Hiệu năng: ghi Ogg Vorbis mono 16 kHz; tắt nhạc nền game khi ghi.

## 7. Tự luận và dự án

**Mục đích.** Soạn bài trả lời ngắn/dài/tự luận/lời giải toán bằng văn bản và số, nộp ảnh/tệp khi được bật; xem nhận xét và rubric của giáo viên ([WRITTEN_RESPONSE_RUBRIC](../02-learning/WRITTEN_RESPONSE_RUBRIC.md)). Bài dự án có các bước và nhóm.

### 7.1. Bố cục Web/PC — soạn bài

```text
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ [◀] Viết đoạn văn tả con vật em yêu · Tiếng Việt 4            Đã lưu 10:42 ✓         │
├──────────────────────────────┬───────────────────────────────────────────────────────┤
│ Đề bài (cuộn)                 │ [B] [•] ↵      Số từ: 86 / 80–150                     │
│ Yêu cầu: …                    │ ┌───────────────────────────────────────────────────┐ │
│ Tiêu chí chấm (rubric rút gọn)│ │ Nhà em có một chú mèo tên Mướp…                    │ │
│ • Mở bài đúng ý              │ │                                                     │ │
│ • Tả đặc điểm (2 ý)           │ │                                                     │ │
│ • Câu văn đúng chính tả       │ └───────────────────────────────────────────────────┘ │
│                               │ Đính kèm (nếu được bật): [📷 Ảnh bài làm] [📎 Tệp]    │
├──────────────────────────────┴───────────────────────────────────────────────────────┤
│                                                      [Lưu nháp]   [ Nộp bài ▶ ]      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### 7.2. Bố cục Mobile — soạn bài

```text
┌──────────────────────────────────────────────────────────────────────┐
│[◀] Tả con vật          [📄 Đề bài]   86 từ (80–150)   Đã lưu ✓  [Nộp]│
│┌────────────────────────────────────────────────────────────────────┐│
││ Nhà em có một chú mèo tên Mướp…                                    ││ ô soạn toàn màn
│└────────────────────────────────────────────────────────────────────┘│
│ [B][•][↵]  [📷]                                                       │ thanh công cụ trên bàn phím
└──────────────────────────────────────────────────────────────────────┘
```

Đề bài và rubric mở dạng ngăn kéo trên (nút "Đề bài"). Bàn phím hệ thống chiếm nửa dưới; ô soạn tự co.

### 7.3. Xem kết quả có nhận xét

Web/PC: hai cột — bài làm (có highlight đoạn giáo viên bình luận) | bảng rubric (tiêu chí, mức đạt, điểm) + nhận xét chung (có thể chứa công thức SVG). Mobile: tab "Bài làm" / "Nhận xét" / "Rubric". Trạng thái `returned`: nút "Sửa và nộp lại".

### 7.4. Dự án nhóm

Bảng bước (milestone) dạng dòng thời gian: tên bước, hạn, loại sản phẩm, trạng thái nhóm; phần "Đóng góp của em" (≤ 1.000 ký tự) luôn riêng từng học sinh. Mobile: danh sách bước dọc, chạm để mở bước.

### 7.5. Dữ liệu & API, trạng thái

| Hành động | API |
|---|---|
| Lưu nháp tự động (15 s) và thủ công | `PUT …/written-response` |
| Tải ảnh/tệp | `POST …/submissions:presign` → PUT → xác nhận |
| Nộp | `POST …:submit` → `pending_review` |
| Xem nhận xét | `GET …/review` |

Trạng thái riêng: dưới số từ tối thiểu → nút Nộp mờ + "Cần thêm N từ"; vượt tối đa → ngăn gõ thêm; tệp bị từ chối (`SUBMISSION_FILE_REJECTED`) → "Tệp không hợp lệ, em thử ảnh khác"; nội dung có thông tin cá nhân/từ nhạy cảm bị cảnh báo theo [WRITTEN_RESPONSE_RUBRIC §7](../02-learning/WRITTEN_RESPONSE_RUBRIC.md#7-kiểm-duyệt-nội-dung-bài-viết). Khác biệt khối: lớp 1–2 không có tự luận (dùng `speaking`); lớp 3–5 trình soạn tối giản không định dạng. Hiệu năng: lưu nháp chạy nền, không chặn gõ; mobile không vẽ lại thế giới 3D khi bàn phím mở.

## 8. Mô phỏng

**Mục đích.** Khung UI quanh scene mô phỏng (thí nghiệm khoa học, vật lý, hình học 3D, phòng thí nghiệm ảo) với bảng điều khiển tham số, mục tiêu và ghi kết quả ([SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md)).

### 8.1. Bố cục Web/PC

```text
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ [◀] Phòng thí nghiệm: Nước sôi ở bao nhiêu độ? · Khoa học 5      Mục tiêu 1/2  [↺]   │
├─────────────────────────────────────────────────────────────┬────────────────────────┤
│                                                             │ Mục tiêu               │
│                 SCENE MÔ PHỎNG 3D/2D                        │ ☐ Đun nước tới khi sôi │
│                 (template PCK đã ký)                        │ ☐ Ghi nhiệt độ         │
│                                                             │ Điều khiển             │
│                                                             │ Lửa: [──●────] Vừa     │
│                                                             │ [▶ Bắt đầu] [⏸] [⏩x2] │
│                                                             │ Quan sát               │
│                                                             │ Nhiệt độ: 87 °C        │
├─────────────────────────────────────────────────────────────┴────────────────────────┤
│  Câu hỏi kết luận (renderer §4) mở sau khi đạt mục tiêu                [ Tiếp ▶ ]    │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### 8.2. Bố cục Mobile

Scene chiếm toàn màn; bảng điều khiển là panel trượt từ phải (mở/đóng bằng nút ⚙ lớn), mục tiêu hiển thị dạng chip ở góc trên trái; điều khiển thanh trượt ≥ 64 dp; cử chỉ xoay/zoom camera bằng hai ngón (hình học 3D) với nút "Về góc nhìn mặc định".

### 8.3. Dữ liệu & API, trạng thái

`POST /learning/activities/{id}/attempts` (tham số, seed, mục tiêu) → học sinh thao tác (client ghi action log) → `POST …/simulation-runs` → hiển thị **kết quả đã kiểm chứng** từ server ("Đạt mục tiêu" chỉ hiện sau khi server xác nhận) → câu hỏi kết luận `POST …/responses` → `:submit`. Trạng thái: nạp template (thanh tiến độ, ≤ 3 s khi đã cache); log không hợp lệ (`SIMULATION_LOG_INVALID`) → "Thí nghiệm chưa ghi đúng, em làm lại nhé" + nút làm lại; thiết bị không đủ (Web/preset Thấp) → dùng bản giản lược của template.

Hiệu năng: scene mô phỏng thay thế render thế giới (không chạy song song); tuân thủ ngân sách template ([SIMULATION_ENGINE §6](../02-learning/SIMULATION_ENGINE.md#6-hiệu-năng)); tốc độ ×2 tắt trên preset Thấp nếu FPS < 30.

## 9. Bài kiểm tra có giờ

**Mục đích.** Làm đề kiểm tra/thi thử có đồng hồ theo server, danh sách câu, đánh dấu câu, nộp và tự nộp khi hết giờ ([EXAM_ENGINE](../02-learning/EXAM_ENGINE.md), [LEARNING_EXPERIENCE §9](../02-learning/LEARNING_EXPERIENCE.md#9-exam-experience)). Không có phản hồi đúng/sai khi đang làm.

### 9.1. Màn quy chế (trước khi bắt đầu)

Số câu, thời gian, có quay lại câu trước không, khi nào xem kết quả, "Thời gian vẫn chạy nếu mất mạng"; nút "Bắt đầu" (gọi `POST /api/v1/exams/sessions/{id}/attempts`).

### 9.2. Bố cục Web/PC

```text
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ Kiểm tra giữa kỳ · Toán 7                      ⏱ 23:41 còn lại            [Nộp bài]   │
├──────────────────┬───────────────────────────────────────────────────────────────────┤
│ Danh sách câu    │ Câu 7 / 20                                       [🚩 Đánh dấu]    │
│ ① ② ③ ④ ⑤        │                                                                   │
│ ⑥ ⑦ ⑧ ⑨ ⑩        │   (renderer §4, không phản hồi đúng/sai)                          │
│ …                │                                                                   │
│ ● đã làm         │                                                                   │
│ ○ chưa làm       │                                                                   │
│ 🚩 đánh dấu      │                                     [◀ Câu trước]  [Câu sau ▶]    │
└──────────────────┴───────────────────────────────────────────────────────────────────┘
```

### 9.3. Bố cục Mobile

Header: đồng hồ + nút "Danh sách" (mở lưới câu dạng ngăn kéo) + Nộp; câu hỏi toàn màn; nút ◀ ▶ ở hai cạnh dưới; đánh dấu trong menu ⋯.

### 9.4. Trạng thái

| Trạng thái | Hiển thị |
|---|---|
| Còn 5 phút / 1 phút | Đồng hồ chuyển màu cảnh báo nhẹ + toast một lần (không nhấp nháy liên tục) |
| Mất mạng | Banner "Đang mất kết nối — câu trả lời được lưu tạm trên máy"; thời gian vẫn chạy; đồng bộ khi có mạng ([EXAM_ENGINE §6](../02-learning/EXAM_ENGINE.md#6-làm-bài--deadline-server)) |
| Nộp bài | Hộp xác nhận liệt kê câu chưa làm/đánh dấu; `POST /api/v1/exams/attempts/{id}:submit` (Idempotency-Key) |
| Hết giờ | Khóa nhập, "Hết giờ — bài đã được nộp tự động" → màn chờ kết quả |
| Kết quả chưa công bố | "Thầy/cô sẽ công bố kết quả sau" |

Lưu câu trả lời: `PUT /api/v1/exams/attempts/{id}/answers/{position}` mỗi lần đổi đáp án. Đồng hồ tính từ `deadline_at` và `server_time`. Khác biệt khối: lớp 1–3 không có bài kiểm tra có giờ dạng số giây; đồng hồ "cát chảy". Hiệu năng: không hiệu ứng, không NPC, chỉ khung + renderer.

## 10. Kết quả học tập

**Mục đích.** Tổng kết sau hoạt động: kết quả học tập, LO cần ôn, gợi ý tiếp theo, mức mastery theo cách hiển thị của khối, và **phần thưởng game tách bạch với điểm** (5 loại số liệu không trộn lẫn, [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)).

### 10.1. Bố cục Web/PC

```text
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                         🎉 Hoàn thành! Cầu đã được sửa xong                          │
├───────────────────────────────────────┬──────────────────────────────────────────────┤
│ KẾT QUẢ HỌC TẬP                       │ PHẦN THƯỞNG TRONG GAME                        │
│ Đúng 7/8 câu                          │ +40 XP   [thanh XP ━━━━●━━ Lv 6]              │
│ Mức "Phép nhân trong bảng 6":          │ +10 Xu                                        │
│   🌳 Đang tiến bộ → Thành thạo         │ 🏅 Huy hiệu "Thợ sửa cầu"                     │
│ Cần ôn: Nhân với 7 (1 câu sai)         │                                              │
│ [Xem lại câu sai]                      │                                              │
├───────────────────────────────────────┴──────────────────────────────────────────────┤
│ Mây gợi ý: "Vì lần trước em còn nhầm phép nhân 7" → [Luyện 3 câu nhân 7]  [Về quest ▶]│
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### 10.2. Bố cục Mobile

Hai thẻ xếp ngang có thể vuốt (Kết quả học tập | Phần thưởng); gợi ý của Mây ở dải dưới; nút "Về quest" bên phải.

### 10.3. Quy tắc nội dung

- Cách hiển thị mastery và điểm theo [LEARNING_EXPERIENCE §7](../02-learning/LEARNING_EXPERIENCE.md#7-hiển-thị-tiến-bộ-cho-học-sinh) (lớp 1–3: sao/hình ảnh, không thang 10).
- Phần thưởng chỉ lấy từ sự kiện server đã xác nhận (`progression.xp.updated`, phần thưởng quest); XP không hiển thị như điểm học tập.
- Gợi ý tiếp theo theo thứ tự ưu tiên [LEARNING_EXPERIENCE §6.2](../02-learning/LEARNING_EXPERIENCE.md#62-recommendation-release-10-rule-based).
- Bài kiểm tra: kết quả chỉ hiện khi được công bố; không có XP theo điểm.
- Bài chờ chấm (`pending_review`): thẻ học tập ghi "Đang chờ thầy/cô chấm".
- Không so sánh với bạn khác trên màn này.

Hiệu năng: hiệu ứng chúc mừng ≤ 1,5 s, tắt được; thế giới 3D vẫn ở trạng thái P-1 cho đến khi rời màn.

## 11. Khóa dịch đề xuất

Khóa theo mẫu `ui.<screen>.<element>`; giá trị vi/en trong `godot/locale/vi.po` và `en.po` ([ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md)). Danh sách khởi đầu:

| Khóa | vi | en |
|---|---|---|
| `ui.activity_player.exit` | Thoát | Exit |
| `ui.activity_player.pause_confirm` | Em muốn tạm dừng? Tiến độ được giữ lại. | Take a break? Your progress is saved. |
| `ui.activity_player.check` / `.next` / `.finish` | Kiểm tra / Tiếp / Hoàn thành | Check / Next / Finish |
| `ui.activity_player.hint` / `.read_aloud` | Gợi ý / Đọc to | Hint / Read aloud |
| `ui.activity_player.error_load` | Chưa tải được bài, em thử lại nhé. | Couldn't load the activity. Please try again. |
| `ui.activity_player.offline` | Mất kết nối — câu trả lời sẽ được gửi khi có mạng. | Offline — your answers will be sent when you're back online. |
| `ui.activity_player.time_up` | Hết giờ rồi! Bài đã được nộp. | Time's up! Your work has been submitted. |
| `ui.activity_player.pending_review` | Bài đã gửi cho thầy/cô. | Sent to your teacher. |
| `ui.activity_player.update_required` | Cần cập nhật game để làm hoạt động này. | Please update the game to play this activity. |
| `ui.question.select_answer` | Chọn một đáp án | Choose an answer |
| `ui.question.select_all` | Chọn tất cả đáp án đúng | Choose all correct answers |
| `ui.learning.no_translation` | Chưa có bản tiếng Anh | English version not available |
| `ui.video.seek_locked` | Em xem hết phần này trước nhé. | Watch this part first. |
| `ui.video.not_ready` | Video đang được chuẩn bị, em quay lại sau nhé. | The video is still being prepared. Please come back later. |
| `ui.video.subtitles` | Phụ đề | Subtitles |
| `ui.recording.mic_permission` | Để ghi giọng đọc, game cần dùng micro. | The game needs your microphone to record your voice. |
| `ui.recording.recording` | Đang ghi âm | Recording |
| `ui.recording.retake` / `.play` / `.send` | Ghi lại / Nghe lại / Gửi cho thầy/cô | Re-record / Play / Send to teacher |
| `ui.written.saved_at` | Đã lưu lúc {time} | Saved at {time} |
| `ui.written.need_words` | Cần thêm {n} từ | {n} more words needed |
| `ui.written.submit` / `.resubmit` | Nộp bài / Sửa và nộp lại | Submit / Edit and resubmit |
| `ui.simulation.goal_reached` | Đạt mục tiêu! | Goal reached! |
| `ui.simulation.log_invalid` | Thí nghiệm chưa ghi đúng, em làm lại nhé. | The experiment wasn't recorded correctly. Let's try again. |
| `ui.exam.rules_start` | Bắt đầu làm bài | Start test |
| `ui.exam.flag` | Đánh dấu | Flag |
| `ui.exam.time_left` | còn lại | left |
| `ui.exam.auto_submitted` | Hết giờ — bài đã được nộp tự động. | Time's up — your test was submitted automatically. |
| `ui.exam.results_pending` | Thầy/cô sẽ công bố kết quả sau. | Your teacher will release the results later. |
| `ui.result.learning` / `.rewards` | Kết quả học tập / Phần thưởng trong game | Learning results / Game rewards |
| `ui.result.review_wrong` | Xem lại câu sai | Review mistakes |
| `ui.result.back_to_quest` | Về nhiệm vụ | Back to quest |

## 12. Acceptance criteria

| ID | Tiêu chí |
|---|---|
| AC-UX-201 | Mọi `activity_type` mở trong cùng khung §3: có nút thoát, tạm dừng, tiến độ, nút chính duy nhất; thoát giữa chừng giữ tiến độ và resume đúng câu (trong resume window) |
| AC-UX-202 | Trên điện thoại ngang 2400×1080, mọi màn §3–§10 dùng được hoàn toàn bằng cảm ứng; vùng chạm ≥ 48 dp (lớp 1–3 ≥ 64 dp); không phần tử nào nằm trong vùng không an toàn (safe area) |
| AC-UX-203 | Trên PC, mọi thao tác chính dùng được bằng bàn phím (`Enter`, `Esc`, `1–6`, `H`, `R`, `Tab`) |
| AC-UX-204 | 11 dạng câu hỏi hiển thị và trả lời được trên cả Web/PC và Mobile theo §4.3; ghép cặp và kéo thả có phương án chạm-chạm trên mobile |
| AC-UX-205 | Công thức hiển thị bằng SVG đúng màu Theme ở cả hai ngôn ngữ; không có ô nhập công thức trong game |
| AC-UX-206 | Đổi ngôn ngữ vi ↔ en trong Cài đặt cập nhật toàn bộ chuỗi UI của các màn này ngay, không khởi động lại; nội dung chưa có bản tiếng Anh hiển thị bản gốc kèm nhãn `ui.learning.no_translation` |
| AC-UX-207 | Video: không tua được qua phần chưa xem lần đầu; câu chèn bắt buộc tạm dừng video; phụ đề chọn vi/en/tắt; tự hạ 480p khi đệm lặp lại |
| AC-UX-208 | Ghi âm: chỉ ghi khi học sinh bấm, luôn có chỉ báo "Đang ghi âm"; tối đa `max_retakes` lần ghi lại; không có phản hồi phát âm tự động; sau khi gửi hiển thị trạng thái chờ giáo viên chấm |
| AC-UX-209 | Tự luận: lưu nháp tự động mỗi 15 giây, mở trên thiết bị khác thấy bản nháp mới nhất; không nộp được khi dưới số từ tối thiểu |
| AC-UX-210 | Mô phỏng: "Đạt mục tiêu" chỉ hiển thị sau khi server kiểm chứng `simulation-runs` |
| AC-UX-211 | Bài kiểm tra: đồng hồ khớp `deadline_at` của server (sai lệch ≤ 1 s sau khi bù `server_time`); hết giờ tự khóa và nộp; không hiện đúng/sai khi đang làm |
| AC-UX-212 | Màn Kết quả tách hai vùng "Kết quả học tập" và "Phần thưởng trong game"; mastery hiển thị theo khối ([LEARNING_EXPERIENCE §7](../02-learning/LEARNING_EXPERIENCE.md#7-hiển-thị-tiến-bộ-cho-học-sinh)) |
| AC-UX-213 | Trên thiết bị tầm trung tham chiếu ([ADR-0057](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md)), các màn §3–§10 giữ ≥ 30 FPS trên mobile/web và ≥ 60 FPS trên PC (trừ khi phát video: ổn định theo FPS video); mở activity player áp dụng P-1 |
| AC-UX-214 | Mất mạng giữa chừng ở mọi màn không làm mất câu trả lời/bản ghi/bản nháp đã nhập; giao diện báo rõ trạng thái và tự đồng bộ khi có mạng |

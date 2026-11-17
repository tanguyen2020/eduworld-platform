# Quickstart: F119 — Giao diện chat trong Godot: panel, quick message, tab, tắt tiếng, báo cáo

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F092, F102, F109, F115–F118; F121 cho báo cáo thật (trước đó Prism mock gắn nhãn MOCK, chỉ build debug).
- Lệnh: `make up`, `make seed`, `make up-app`; mở `src/game/` bằng Godot 4.x, chạy `res://scenes/bootstrap/bootstrap.tscn` hai instance với `--device-id` khác nhau ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (TH Demo A), `hs.dung.7a`; `sm.antoan` để xem báo cáo trong hàng đợi (F121).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | An tạo party, mời Bình; mở lobby | Panel chat có ngay; thu gọn → tin mới hiện badge; không có nút tắt chat | F119-AC-01 |
| 2 | Team battle 2v2 | Tab "Đội" và "Cả trận", badge riêng | F119-AC-02 |
| 3 | Câu hỏi hiển thị trong trận (mobile) | Panel thu thành dải 2 dòng, không che đáp án | F119-AC-03 |
| 4 | Ngắt mạng An, gửi tin, nối lại | Đồng hồ → "Đang kết nối lại chat…" → đã gửi đúng một lần | F119-AC-04, F119-AC-05 |
| 5 | Kết thúc trận, gửi tin; mở lịch sử trận | Ô nhập khóa; "Tin nhắn trong trận được giữ N ngày" (N từ server) | F119-AC-06, F119-AC-07 |
| 6 | An (vi) chạm "Chúc may mắn!", Bình đặt `en-US` | Bình thấy bản tiếng Anh | F119-AC-08 |
| 7 | An gửi tin có SĐT; gửi dồn | Thông điệp `ui.chat.feedback.pii_blocked`, không lộ chuỗi khớp; "Bạn gửi hơi nhanh…" | F119-AC-09, F119-AC-10 |
| 8 | Bình bị mute (auto-mute F116) | Ô nhập khóa kèm giờ mở lại, vẫn đọc tin | F119-AC-11 |
| 9 | Bình nhấn giữ tin của An → Báo cáo → `rude_language` | ≤ 2 thao tác tới hộp báo cáo; `createReport` có `match_id`; `sm.antoan` thấy case | F119-AC-12, F119-AC-13 |
| 10 | Màn kết quả trận → "Báo cáo người chơi", "Chặn" | Gọi API, tin của người bị chặn ẩn ngay | F119-AC-14, F119-AC-15 |
| 11 | Mở picker: pack khóa, GIF theo chủ đề | Ổ khóa + gợi ý; không có ô tìm Internet; ≤ 3 ảnh động phát | F119-AC-16, F119-AC-17 |
| 12 | Thả cảm xúc, trả lời, sao chép tin đã che, thu hồi tin của mình | Số đếm cập nhật; clipboard chỉ bản đã che; thu hồi hỏi xác nhận; nhãn/placeholder đúng | F119-AC-18, F119-AC-19, F119-AC-20 |
| 13 | Gửi 500 tin vào channel trên thiết bị tầm trung | FPS trong ngân sách; ≤ `chat_recent_buffer` tin trong RAM | F119-AC-21 |
| 14 | Chạy pseudo-locale `xx-PS`; `make godot-test` lint theme | Không tràn chữ, không chuỗi/màu hardcode | F119-AC-22 |
| 15 | Quét mã `src/game/` | Không kết nối NATS/Valkey, không logic kiểm duyệt | F119-AC-23 |

## 3. Lệnh test tự động

```bash
make godot-test            # GdUnit4 headless: tests/unit/chat, tests/scene/chat
make godot-export-check
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4, ảnh chụp panel ở `vi`/`en`/`xx-PS` (PC + mobile), số liệu FPS profiler thiết bị lab, video thao tác báo cáo ≤ 2 chạm; ghi rõ kịch bản nào chạy trên Prism mock.

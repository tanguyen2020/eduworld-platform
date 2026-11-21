# Quickstart: F118 — Bộ tính năng nhắn tin đầy đủ (sửa/thu hồi, reply/forward, mention, receipts) và sticker/GIF

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F115, F116, F117 (party/trận có channel); F075 cho kịch bản pack khóa (trước đó kiểm fail-closed).
- Lệnh: `make up`, `make seed` (1 sticker pack mẫu mặc định), `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=chat` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (party 3 người, An leader), `hs.dung.7a` (tenant `pgd-demo`), `ad.truong.a`, `sa.admin` (Game Studio, `game.chat_assets.write`), `cm.noidung` (người duyệt GIF).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | An gửi tin, sửa sau 1 phút | Bình nhận `chat.message.edited` `revision = 2`; `message_revisions` có bản cũ | F118-AC-01 |
| 2 | An sửa sau `chat_edit_window`; Bình sửa tin của An; sửa với revision cũ | `CHAT_EDIT_WINDOW_EXPIRED` ×2; `412` | F118-AC-02 |
| 3 | An sửa sang nội dung chứa PII | `CHAT_CONTENT_BLOCKED`, bản cũ giữ | F118-AC-03 |
| 4 | An ghim rồi thu hồi một tin | Mọi người thấy placeholder, pin tự bỏ; DB còn `body` | F118-AC-04, F118-AC-06 |
| 5 | Bình reply + quote tin đã che của An; sau đó An thu hồi | Excerpt chỉ có `***`; sau thu hồi hiển thị "Tin nhắn gốc không còn" | F118-AC-05, F118-AC-07, F118-AC-08 |
| 6 | An forward tin sang channel trận và channel của `hs.dung.7a` (tenant khác) | Đích hợp lệ có tin mới `forwarded_from`; đích khác tenant `CHAT_FORWARD_NOT_ALLOWED`; gọi lại cùng key → cùng kết quả | F118-AC-09, F118-AC-10, F118-AC-11 |
| 7 | Gửi sticker pack mặc định; pack khóa chưa mở; `gif_id` lạ | Thành công; `CHAT_ASSET_NOT_AVAILABLE` ×2 | F118-AC-12, F118-AC-13, F118-AC-14 |
| 8 | Bình thả 4 reaction lên một tin | 3 thành công, lần 4 bị từ chối; số đếm đồng bộ | F118-AC-16 |
| 9 | `sa.admin` upload GIF 3 MB; upload hợp lệ rồi tự duyệt; `cm.noidung` duyệt + publish; `ad.truong.a` gọi `createGameConfig_chat` | Từ chối kích thước; từ chối tự duyệt; publish OK; `403` | F118-AC-17 |
| 10 | An gửi tin 6 mention; tin mention Bình + một học sinh không thuộc party | `CHAT_MENTION_LIMIT`; chỉ Bình nhận `chat.mention.created` (nickname, không username) | F118-AC-18, F118-AC-19 |
| 11 | Chi (không leader) ghim tin party; An ghim tin thứ 4 | `403`; `CHAT_PIN_LIMIT_REACHED` | F118-AC-20 |
| 12 | Bình gõ phím liên tục 10 s rồi dừng | Typing tối đa 1 lần/3 s, tự tắt sau 5 s, không có dòng DB | F118-AC-21 |
| 13 | Bình nhận, Chi đọc; An gọi `getMessageReceipts` | Từng người đúng trạng thái; channel club > 8 thành viên (F120) chỉ số đếm | F118-AC-22, F118-AC-23 |
| 14 | Chi đọc trên thiết bị 2 | Thiết bị 1 nhận `chat.unread.updated`, mention reset | F118-AC-25 |
| 15 | `hs.dung.7a` gọi `editMessage`/`getMessageReceipts` tin của party; token `ad.truong.a` gọi `forwardMessage` | `404`; `403 GAME_ACCESS_STUDENT_ONLY` | F118-AC-26 |

## 3. Lệnh test tự động

```bash
make test MOD=chat
make test-integration MOD=chat
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test, trace_id kịch bản 1 và 6, truy vấn chứng minh nội dung gốc còn sau thu hồi (kịch bản 4), log kiểm tệp GIF (kịch bản 9).

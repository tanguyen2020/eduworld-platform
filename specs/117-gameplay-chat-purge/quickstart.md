# Quickstart: F117 — Chat gameplay luôn có (`match`/`team`/`party`) và purge chat gameplay

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F115, F116, F101 (party lobby), F105 (battle); F121 cho kịch bản bằng chứng (trước đó kiểm chế độ fail-closed).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=chat` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (`th-demo-a`), `hs.dung.7a` (`pgd-demo`), `ad.truong.a` (School Admin), `sm.antoan`.
- Đồng hồ giả lập cho job: `go -C src/backend run ./cmd/worker --profile=chat --job=gameplay-chat-purge --once --now=<RFC3339>` (chỉ build tag `dev`).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | An tạo party (F100), mời Bình; Chi vào sau | `party.state.snapshot.chat_channel_id` có ngay; cả ba chat được; Chi đọc từ lúc vào | F117-AC-01, F117-AC-04 |
| 2 | Phát lại `party.created` + `party.member_joined` hai lần (nats-box) | Không channel/thành viên trùng | F117-AC-03 |
| 3 | Team battle 2v2 (An+Bình vs Chi+bot học sinh) | 1 `match` + 2 `team`; Chi gọi lịch sử `team` của An → `404` | F117-AC-02 |
| 4 | An đổi thiết bị giữa trận (takeover F014), gửi `chat.channel.sync` | Vẫn member, nhận tin bỏ lỡ | F117-AC-06 |
| 5 | `ad.truong.a` đặt rule cấp trường `party.enabled=false` (fixture repo/API F120) | `422 CHAT_POLICY_ALWAYS_ON`; chat party vẫn chạy | F117-AC-08, F117-AC-09 |
| 6 | Áp sanction `mute` cho Bình (F116 auto-mute) | Bình vẫn nhận tin, gửi → `CHAT_MUTED` | F117-AC-11 |
| 7 | An chặn Chi (`blockUser`) | An/Chi không thấy tin của nhau, cả hai vẫn trong trận | F117-AC-12 |
| 8 | Bình đọc trên thiết bị 1 | Thiết bị 2 nhận `chat.unread.updated`; `listMyChannels` đúng `unread_count` | F117-AC-13, F117-AC-14 |
| 9 | Kết thúc trận (`match.completed`) rồi gửi tin | `CHAT_CHANNEL_CLOSED`; `purge_after = closed_at + N` | F117-AC-05, F117-AC-15 |
| 10 | Chưa có F121: chạy purge với `--now = purge_after + 1h` | Không xóa; run `blocked`; metric `purge_blocked` = 1 | F117-AC-19 |
| 11 | Có F121: An báo cáo một tin trận ngày D+2; chạy purge D+N−1 rồi D+N | D+N−1 còn; D+N xóa channel, API `404`; `sm.antoan` thấy tin trong bằng chứng | F117-AC-16, F117-AC-17, F117-AC-23 |
| 12 | Channel có case `open` quá hạn | Hoãn, ghi lý do; đóng case → purge lần sau | F117-AC-18 |
| 13 | Đếm dòng channel club (`team` cố định)/`direct` trước/sau purge; thử `DELETE` bằng role `chat_app` và `chat_purger` trên `chat.messages` | Không đổi; lỗi quyền | F117-AC-20 |
| 14 | Tin `world`/`area` > `ephemeral_chat_retention` | Partition ngày bị drop sau khi sao bằng chứng | F117-AC-21 |
| 15 | Xem `chat.gameplay_purge_runs`, audit, metric | Có số liệu, không nội dung | F117-AC-22 |
| 16 | Trường TH Demo A bật preset; tạo party An + Bình; An gửi `text` rồi `quick` | Channel `party` tự tạo; `text` → `CHAT_KIND_NOT_ALLOWED`; `quick` gửi được | F117-AC-24 |

## 3. Lệnh test tự động

```bash
make test MOD=chat
make test-integration MOD=chat     # role chat_purger thật, đồng hồ giả lập
make test-contract                         # consumer chat ↔ competition (party.*, match.*)
go -C src/backend test ./modules/chat/tests/static/... # DELETE chỉ trong internal/infrastructure/purge; không cờ tắt always-on
```

## 4. Bằng chứng cần nộp

- Kết quả test, bảng đếm dòng trước/sau purge, bản ghi `gameplay_purge_runs`, trace_id kịch bản 1 (event → channel → tin đầu tiên), ảnh bằng chứng trong case (kịch bản 11).
